package service

import (
	"fmt"
	"strconv"
	"time"
)

// 面板级配置的 Setting 键。
const (
	settingPanelTitle = "panel_title"
	settingPanelLogo  = "panel_logo"
	settingLogClean   = "log_clean_days"
)

// SystemConfigService 面板级配置(标题/图标/日志清理)。
type SystemConfigService struct{}

func NewSystemConfigService() *SystemConfigService { return &SystemConfigService{} }

// GetConfig 返回面板配置。
func (s *SystemConfigService) GetConfig() map[string]interface{} {
	return map[string]interface{}{
		"panel_title":        getSetting(settingPanelTitle, "Task Panel"),
		"panel_logo":         getSetting(settingPanelLogo, ""),
		"log_clean_days":     settingInt(settingLogClean, 0),
		"notify_tpl_success": getSetting(settingTplSuccess, ""),
		"notify_tpl_failed":  getSetting(settingTplFailed, ""),
		"notify_tpl_aborted": getSetting(settingTplAborted, ""),
		"event_alerts":       getSetting(settingEventAlerts, "true") == "true",
	}
}

// UpdateConfig 保存面板配置,并同步日志清理调度。
func (s *SystemConfigService) UpdateConfig(values map[string]interface{}) error {
	for k, v := range values {
		switch k {
		case "panel_title", "panel_logo":
			if s, ok := v.(string); ok && len(s) <= 64 {
				if err := setSetting(k, s); err != nil {
					return err
				}
			}
		case "notify_tpl_success", "notify_tpl_failed", "notify_tpl_aborted":
			if s, ok := v.(string); ok && len(s) <= 500 {
				if err := setSetting(k, s); err != nil {
					return err
				}
			}
		case "event_alerts":
			if b, ok := v.(bool); ok {
				if err := setSetting(k, strconv.FormatBool(b)); err != nil {
					return err
				}
			}
		case "log_clean_days":
			n := 0
			switch t := v.(type) {
			case float64:
				n = int(t)
			case string:
				n, _ = strconv.Atoi(t)
			}
			if n < 0 {
				n = 0
			}
			if err := setSetting(k, strconv.Itoa(n)); err != nil {
				return err
			}
		}
	}
	return nil
}

// InitLogCleanSchedule 启动日志自动清理:启动时清一次,之后每 24 小时检查。
func (s *SystemConfigService) InitLogCleanSchedule() {
	go func() {
		for {
			if days := settingInt(settingLogClean, 0); days > 0 {
				if records, files, err := NewLogService().Clean(days); err == nil && (records > 0 || files > 0) {
					fmt.Printf("日志自动清理: 删除 %d 条记录、%d 个文件(%d 天前)\n", records, files, days)
				}
			}
			time.Sleep(24 * time.Hour)
		}
	}()
}

func settingInt(key string, def int) int {
	n, err := strconv.Atoi(getSetting(key, ""))
	if err != nil {
		return def
	}
	return n
}
