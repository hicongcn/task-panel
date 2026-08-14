package service

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"taskpanel/database"
	"taskpanel/middleware"
	"taskpanel/model"
)

// OpenAPIService 开放平台应用管理与令牌签发(参考青龙 OpenAPI 结构)。
type OpenAPIService struct{}

func NewOpenAPIService() *OpenAPIService { return &OpenAPIService{} }

// openTokenTTL Open API 令牌有效期(默认 1 年)。
const openTokenTTL = 365 * 24 * time.Hour

// 保留字:不允许作为应用名(与青龙一致)。
const reservedAppName = "system"

// ValidScopes 返回可用的 scope 及中文说明。
func ValidScopes() map[string]string {
	return map[string]string{
		model.ScopeTasksRead: "查看任务",
		model.ScopeTasksRun:  "触发任务运行",
		model.ScopeLogsRead:  "查看执行日志",
		model.ScopeEnvsRead:  "查看环境变量",
	}
}

// randomAlnum 生成 n 位字母数字随机串(供 client_id 使用)。
func randomAlnum(n int) string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	for i := range b {
		b[i] = chars[int(b[i])%len(chars)]
	}
	return string(b)
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

func scopesToJSON(scopes []string) string {
	if scopes == nil {
		scopes = []string{}
	}
	b, _ := json.Marshal(scopes)
	return string(b)
}

func scopesFromJSON(s string) []string {
	var out []string
	_ = json.Unmarshal([]byte(s), &out)
	return out
}

// Create 创建应用,返回应用与明文 client_secret(仅此一次)。
func (s *OpenAPIService) Create(name string, scopes []string) (*model.OpenApp, string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, "", fmt.Errorf("应用名称不能为空")
	}
	if name == reservedAppName {
		return nil, "", fmt.Errorf("应用名称不能使用保留字 %q", reservedAppName)
	}
	if err := validateScopes(scopes); err != nil {
		return nil, "", err
	}
	var count int64
	database.DB.Model(&model.OpenApp{}).Where("name = ?", name).Count(&count)
	if count > 0 {
		return nil, "", fmt.Errorf("应用名称已存在")
	}

	secret := randomAlnum(32)
	app := &model.OpenApp{
		Name:         name,
		ClientID:     randomAlnum(4) + "-" + randomAlnum(8),
		ClientSecret: secret,
		Scopes:       scopesToJSON(scopes),
		Enabled:      true,
	}
	if err := database.DB.Create(app).Error; err != nil {
		return nil, "", fmt.Errorf("创建失败: %w", err)
	}
	return app, secret, nil
}

// List 返回应用列表(不含 secret)。
// List 返回应用列表(含解析后的 scopes 数组,不含 secret)。
func (s *OpenAPIService) List() []map[string]interface{} {
	var apps []model.OpenApp
	database.DB.Order("id ASC").Find(&apps)
	out := make([]map[string]interface{}, len(apps))
	for i, app := range apps {
		out[i] = appDict(app)
	}
	return out
}

// appDict 构造应用对外结构(含 secret 供点击复制,单管理员自用面板)。
func appDict(app model.OpenApp) map[string]interface{} {
	return map[string]interface{}{
		"id":            app.ID,
		"name":          app.Name,
		"client_id":     app.ClientID,
		"client_secret": app.ClientSecret,
		"scopes":        scopesFromJSON(app.Scopes),
		"enabled":       app.Enabled,
		"created_at":    app.CreatedAt,
		"updated_at":    app.UpdatedAt,
	}
}

// Update 更新应用名称与权限范围。
func (s *OpenAPIService) Update(id uint, name *string, scopes []string) error {
	var app model.OpenApp
	if err := database.DB.First(&app, id).Error; err != nil {
		return fmt.Errorf("应用不存在")
	}
	if name != nil {
		n := strings.TrimSpace(*name)
		if n == "" {
			return fmt.Errorf("应用名称不能为空")
		}
		if n == reservedAppName {
			return fmt.Errorf("应用名称不能使用保留字 %q", reservedAppName)
		}
		if n != app.Name {
			var count int64
			database.DB.Model(&model.OpenApp{}).Where("name = ?", n).Count(&count)
			if count > 0 {
				return fmt.Errorf("应用名称已存在")
			}
		}
		app.Name = n
	}
	if scopes != nil {
		if err := validateScopes(scopes); err != nil {
			return err
		}
		app.Scopes = scopesToJSON(scopes)
	}
	return database.DB.Save(&app).Error
}

// Delete 删除应用。
func (s *OpenAPIService) Delete(id uint) error {
	return database.DB.Delete(&model.OpenApp{}, id).Error
}

// ResetSecret 重置应用密钥,返回新 secret(仅此一次)。
func (s *OpenAPIService) ResetSecret(id uint) (string, error) {
	var app model.OpenApp
	if err := database.DB.First(&app, id).Error; err != nil {
		return "", fmt.Errorf("应用不存在")
	}
	secret := randomHex(32)
	if err := database.DB.Model(&app).Update("client_secret", secret).Error; err != nil {
		return "", err
	}
	return secret, nil
}

// Token 用 client_id + client_secret 换取 Open API 令牌。
func (s *OpenAPIService) Token(clientID, clientSecret string) (string, time.Time, error) {
	var app model.OpenApp
	if err := database.DB.Where("client_id = ?", clientID).First(&app).Error; err != nil {
		return "", time.Time{}, fmt.Errorf("应用不存在")
	}
	if !app.Enabled {
		return "", time.Time{}, fmt.Errorf("应用已禁用")
	}
	if app.ClientSecret != clientSecret {
		return "", time.Time{}, fmt.Errorf("client_secret 不正确")
	}
	return middleware.GenerateOpenToken(app.Name, scopesFromJSON(app.Scopes), openTokenTTL)
}

func validateScopes(scopes []string) error {
	valid := ValidScopes()
	for _, sc := range scopes {
		if _, ok := valid[sc]; !ok {
			return fmt.Errorf("非法 scope: %s", sc)
		}
	}
	return nil
}
