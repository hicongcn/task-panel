package service

import (
	"taskpanel/database"
	"taskpanel/model"

	"gorm.io/gorm"
)

// AuditService 负责审计日志的记录与查询。
type AuditService struct{}

func NewAuditService() *AuditService { return &AuditService{} }

// Record 记录一条审计日志。同步写入,保证审计不丢(单条写入开销极小)。
func (s *AuditService) Record(username, action, resource, detail, ip string) {
	entry := &model.AuditLog{
		Username: username, Action: action, Resource: resource,
		Detail: detail, IP: ip,
	}
	database.DB.Create(entry)
}

// List 分页返回审计日志,支持按用户名/动作过滤。
func (s *AuditService) List(username, action string, page, pageSize int) ([]model.AuditLog, int64) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	where := database.DB.Model(&model.AuditLog{})
	if username != "" {
		where = where.Where("username = ?", username)
	}
	if action != "" {
		where = where.Where("action = ?", action)
	}

	var total int64
	where.Session(&gorm.Session{}).Count(&total)

	var logs []model.AuditLog
	where.Session(&gorm.Session{}).Order("id DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs)
	return logs, total
}
