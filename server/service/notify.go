package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"net/url"
	"strings"
	"time"

	"taskpanel/database"
	"taskpanel/model"
)

// NotifyService 负责通知渠道 CRUD 与消息发送。
type NotifyService struct{}

func NewNotifyService() *NotifyService { return &NotifyService{} }

var defaultNotifyService = &NotifyService{}

// GetNotifyService 返回全局单例。
func GetNotifyService() *NotifyService { return defaultNotifyService }

// maskedPassword 表示脱敏后的密码占位,更新时视为"未修改"。
const maskedPassword = "******"

// ---- 各渠道配置结构 ----

type webhookConfig struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
}

type telegramConfig struct {
	BotToken string `json:"bot_token"`
	ChatID   string `json:"chat_id"`
}

type barkConfig struct {
	Server    string `json:"server"`
	DeviceKey string `json:"device_key"`
}

type emailConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	From     string `json:"from"`
	To       string `json:"to"`
}

// ---- CRUD ----

// List 返回全部渠道,config 解析为 object 返回。
func (s *NotifyService) List() []map[string]interface{} {
	var chs []model.NotifyChannel
	database.DB.Order("id ASC").Find(&chs)
	out := make([]map[string]interface{}, len(chs))
	for i, ch := range chs {
		out[i] = channelDict(ch)
	}
	return out
}

// Create 新建渠道。config 为类型对应的配置对象。
func (s *NotifyService) Create(name, typ string, enabled bool, config map[string]interface{}) (*model.NotifyChannel, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("渠道名称不能为空")
	}
	if err := validateNotifyConfig(typ, config); err != nil {
		return nil, err
	}
	cfgBytes, _ := json.Marshal(config)
	ch := &model.NotifyChannel{Name: name, Type: typ, Enabled: enabled, Config: string(cfgBytes)}
	if err := database.DB.Create(ch).Error; err != nil {
		return nil, fmt.Errorf("创建失败: %w", err)
	}
	return ch, nil
}

// Update 更新渠道。
func (s *NotifyService) Update(id uint, name, typ string, enabled *bool, config map[string]interface{}) (*model.NotifyChannel, error) {
	var ch model.NotifyChannel
	if err := database.DB.First(&ch, id).Error; err != nil {
		return nil, fmt.Errorf("渠道不存在")
	}
	if name != "" {
		ch.Name = name
	}
	if typ != "" {
		ch.Type = typ
	}
	if enabled != nil {
		ch.Enabled = *enabled
	}
	if config != nil {
		// email 密码为掩码时保留原密码(未修改)。
		if ch.Type == model.NotifyTypeEmail {
			if pw, ok := config["password"].(string); ok && pw == maskedPassword {
				var orig map[string]interface{}
				_ = json.Unmarshal([]byte(ch.Config), &orig)
				if opw, ok := orig["password"].(string); ok {
					config["password"] = opw
				}
			}
		}
		if err := validateNotifyConfig(ch.Type, config); err != nil {
			return nil, err
		}
		cfgBytes, _ := json.Marshal(config)
		ch.Config = string(cfgBytes)
	}
	if err := database.DB.Save(&ch).Error; err != nil {
		return nil, fmt.Errorf("更新失败: %w", err)
	}
	return &ch, nil
}

// Delete 删除渠道。
func (s *NotifyService) Delete(id uint) error {
	return database.DB.Delete(&model.NotifyChannel{}, id).Error
}

// Toggle 启用/禁用渠道。
func (s *NotifyService) Toggle(id uint, enabled bool) (*model.NotifyChannel, error) {
	var ch model.NotifyChannel
	if err := database.DB.First(&ch, id).Error; err != nil {
		return nil, fmt.Errorf("渠道不存在")
	}
	ch.Enabled = enabled
	if err := database.DB.Save(&ch).Error; err != nil {
		return nil, err
	}
	return &ch, nil
}

// Test 测试发送一条消息到指定渠道(不落库)。
func (s *NotifyService) Test(typ string, config map[string]interface{}) error {
	if err := validateNotifyConfig(typ, config); err != nil {
		return err
	}
	cfgBytes, _ := json.Marshal(config)
	ch := model.NotifyChannel{Name: "test", Type: typ, Config: string(cfgBytes)}
	return s.sendOne(ch, "task-panel 测试通知", "这是一条测试消息")
}

// ---- 发送 ----

// Notify 向所有启用渠道异步发送消息(best-effort,失败仅记日志)。
func (s *NotifyService) Notify(title, content string) {
	var chs []model.NotifyChannel
	database.DB.Where("enabled = ?", true).Find(&chs)
	for _, ch := range chs {
		ch := ch
		go s.sendOne(ch, title, content)
	}
}

// NotifyTaskResult 发送任务执行结果通知。
func (s *NotifyService) NotifyTaskResult(taskName, status string, duration float64) {
	title, content := buildTaskResultMessage(taskName, status, duration)
	s.Notify(title, content)
}

func (s *NotifyService) sendOne(ch model.NotifyChannel, title, content string) error {
	var err error
	switch ch.Type {
	case model.NotifyTypeWebhook:
		err = sendWebhook(ch.Config, title, content)
	case model.NotifyTypeTelegram:
		err = sendTelegram(ch.Config, title, content)
	case model.NotifyTypeBark:
		err = sendBark(ch.Config, title, content)
	case model.NotifyTypeEmail:
		err = sendEmail(ch.Config, title, content)
	default:
		err = fmt.Errorf("未知渠道类型: %s", ch.Type)
	}
	if err != nil {
		log.Printf("通知发送失败 [%s/%s]: %v", ch.Name, ch.Type, err)
	}
	return err
}

// ---- 各渠道 sender ----

func httpClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Second}
}

func sendWebhook(cfgJSON, title, content string) error {
	var cfg webhookConfig
	if err := json.Unmarshal([]byte(cfgJSON), &cfg); err != nil {
		return err
	}
	if cfg.URL == "" {
		return fmt.Errorf("webhook url 不能为空")
	}
	method := strings.ToUpper(cfg.Method)
	if method == "" {
		method = http.MethodPost
	}
	if method != http.MethodGet && method != http.MethodPost && method != http.MethodPut {
		return fmt.Errorf("webhook method 仅支持 GET/POST/PUT")
	}
	payload, _ := json.Marshal(map[string]string{"title": title, "content": content})
	req, err := http.NewRequest(method, cfg.URL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}
	resp, err := httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook 返回 %d", resp.StatusCode)
	}
	return nil
}

func sendTelegram(cfgJSON, title, content string) error {
	var cfg telegramConfig
	if err := json.Unmarshal([]byte(cfgJSON), &cfg); err != nil {
		return err
	}
	if cfg.BotToken == "" || cfg.ChatID == "" {
		return fmt.Errorf("telegram bot_token/chat_id 不能为空")
	}
	text := title
	if content != "" {
		text += "\n" + content
	}
	payload, _ := json.Marshal(map[string]string{"chat_id": cfg.ChatID, "text": text})
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", cfg.BotToken)
	resp, err := httpClient().Post(apiURL, "application/json", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("telegram 返回 %d", resp.StatusCode)
	}
	return nil
}

func sendBark(cfgJSON, title, content string) error {
	var cfg barkConfig
	if err := json.Unmarshal([]byte(cfgJSON), &cfg); err != nil {
		return err
	}
	if cfg.DeviceKey == "" {
		return fmt.Errorf("bark device_key 不能为空")
	}
	server := strings.TrimRight(cfg.Server, "/")
	if server == "" {
		server = "https://api.day.app"
	}
	fullURL := fmt.Sprintf("%s/%s/%s/%s", server,
		url.PathEscape(cfg.DeviceKey), url.PathEscape(title), url.PathEscape(content))
	resp, err := httpClient().Get(fullURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("bark 返回 %d", resp.StatusCode)
	}
	return nil
}

func sendEmail(cfgJSON, title, content string) error {
	var cfg emailConfig
	if err := json.Unmarshal([]byte(cfgJSON), &cfg); err != nil {
		return err
	}
	if cfg.Host == "" || cfg.From == "" || cfg.To == "" {
		return fmt.Errorf("email host/from/to 不能为空")
	}
	if cfg.Port == 0 {
		cfg.Port = 587
	}
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	var auth smtp.Auth
	if cfg.Username != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
	}
	msg := buildEmailMessage(cfg.From, cfg.To, title, content)
	return smtp.SendMail(addr, auth, cfg.From, []string{cfg.To}, []byte(msg))
}

func buildEmailMessage(from, to, subject, body string) string {
	var b strings.Builder
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + to + "\r\n")
	b.WriteString("Subject: " + subject + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("\r\n")
	b.WriteString(body)
	return b.String()
}

// ---- 校验与消息构建 ----

func validateNotifyConfig(typ string, config map[string]interface{}) error {
	cfgBytes, _ := json.Marshal(config)
	switch typ {
	case model.NotifyTypeWebhook:
		var c webhookConfig
		_ = json.Unmarshal(cfgBytes, &c)
		if c.URL == "" {
			return fmt.Errorf("webhook 需配置 url")
		}
		if !strings.HasPrefix(c.URL, "http://") && !strings.HasPrefix(c.URL, "https://") {
			return fmt.Errorf("webhook url 必须以 http(s):// 开头")
		}
	case model.NotifyTypeTelegram:
		var c telegramConfig
		_ = json.Unmarshal(cfgBytes, &c)
		if c.BotToken == "" || c.ChatID == "" {
			return fmt.Errorf("telegram 需配置 bot_token 和 chat_id")
		}
	case model.NotifyTypeBark:
		var c barkConfig
		_ = json.Unmarshal(cfgBytes, &c)
		if c.DeviceKey == "" {
			return fmt.Errorf("bark 需配置 device_key")
		}
	case model.NotifyTypeEmail:
		var c emailConfig
		_ = json.Unmarshal(cfgBytes, &c)
		if c.Host == "" || c.From == "" || c.To == "" {
			return fmt.Errorf("email 需配置 host/from/to")
		}
	default:
		return fmt.Errorf("不支持的渠道类型: %s", typ)
	}
	return nil
}

func buildTaskResultMessage(taskName, status string, duration float64) (string, string) {
	statusText := map[string]string{
		model.RunStatusSuccess: "成功",
		model.RunStatusFailed:  "失败",
		model.RunStatusAborted: "终止",
	}[status]
	if statusText == "" {
		statusText = status
	}
	title := fmt.Sprintf("任务执行%s", statusText)
	content := fmt.Sprintf("任务「%s」执行%s,耗时 %.2f 秒", taskName, statusText, duration)
	return title, content
}

func channelDict(ch model.NotifyChannel) map[string]interface{} {
	var cfg map[string]interface{}
	if ch.Config != "" {
		_ = json.Unmarshal([]byte(ch.Config), &cfg)
	}
	if cfg == nil {
		cfg = map[string]interface{}{}
	}
	// email 密码脱敏,避免明文出现在响应里。
	if ch.Type == model.NotifyTypeEmail {
		if pw, ok := cfg["password"].(string); ok && pw != "" {
			cfg["password"] = maskedPassword
		}
	}
	return map[string]interface{}{
		"id":         ch.ID,
		"name":       ch.Name,
		"type":       ch.Type,
		"enabled":    ch.Enabled,
		"config":     cfg,
		"created_at": ch.CreatedAt,
	}
}
