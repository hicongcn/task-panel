package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"taskpanel/config"
	"taskpanel/database"
	"taskpanel/model"
	"taskpanel/pkg/pathutil"

	"gorm.io/gorm"
)

// LogService 负责任务日志查询与定位。
type LogService struct{}

func NewLogService() *LogService { return &LogService{} }

// List 分页返回任务日志,可按 task_id 过滤。
func (s *LogService) List(taskID uint, page, pageSize int) ([]model.TaskLog, int64) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	where := database.DB.Model(&model.TaskLog{})
	if taskID > 0 {
		where = where.Where("task_id = ?", taskID)
	}

	var total int64
	where.Session(&gorm.Session{}).Count(&total)

	var logs []model.TaskLog
	where.Session(&gorm.Session{}).Order("started_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs)
	return logs, total
}

// Get 返回单条日志记录(含内容)。
func (s *LogService) Get(id uint) (*model.TaskLog, string, error) {
	var log model.TaskLog
	if err := database.DB.First(&log, id).Error; err != nil {
		return nil, "", fmt.Errorf("日志不存在")
	}
	content := log.Content
	if strings.TrimSpace(content) == "" && log.LogPath != "" {
		// 从落盘文件读取
		full, err := safeLogPath(log.LogPath)
		if err == nil {
			if data, rerr := os.ReadFile(full); rerr == nil {
				content = string(data)
			}
		}
	}
	return &log, content, nil
}

// LatestRunningLog 返回某任务当前正在运行的日志记录(无则 nil)。
func (s *LogService) LatestRunningLog(taskID uint) (*model.TaskLog, error) {
	var log model.TaskLog
	if err := database.DB.Where("task_id = ? AND status = ?", taskID, model.LogStatusRunning).
		Order("started_at DESC").First(&log).Error; err != nil {
		return nil, nil
	}
	return &log, nil
}

// LatestLog 返回某任务最近一条日志(无则 nil)。
func (s *LogService) LatestLog(taskID uint) (*model.TaskLog, error) {
	var log model.TaskLog
	if err := database.DB.Where("task_id = ?", taskID).
		Order("started_at DESC").First(&log).Error; err != nil {
		return nil, nil
	}
	return &log, nil
}

// RawFilePath 校验并返回日志原始文件绝对路径。供下载票据使用。
func (s *LogService) RawFilePath(id uint) (string, string, error) {
	var log model.TaskLog
	if err := database.DB.First(&log, id).Error; err != nil {
		return "", "", fmt.Errorf("日志不存在")
	}
	if strings.TrimSpace(log.LogPath) == "" {
		return "", "", fmt.Errorf("该日志没有独立文件(内容仅存于数据库)")
	}
	full, err := safeLogPath(log.LogPath)
	if err != nil {
		return "", "", err
	}
	if info, err := os.Stat(full); err != nil || info.IsDir() {
		return "", "", fmt.Errorf("原始日志文件不存在或已被清理")
	}
	return full, filepath.Base(full), nil
}

// safeLogPath 把日志目录内的相对/绝对路径解析并校验不越界。
func safeLogPath(logPath string) (string, error) {
	logDir := config.C.Data.LogDir
	return pathutil.ResolveWithinBase(logDir, logPath, false)
}
