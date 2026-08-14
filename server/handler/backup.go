package handler

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"taskpanel/config"
	"taskpanel/middleware"
	"taskpanel/model"
	"taskpanel/pkg/response"
	"taskpanel/service"

	"github.com/gin-gonic/gin"
)

type BackupHandler struct {
	svc     *service.BackupService
	setting *service.SettingService
}

func NewBackupHandler() *BackupHandler {
	return &BackupHandler{svc: service.GetBackupService(), setting: service.NewSettingService()}
}

// Create POST /backups
func (h *BackupHandler) Create(c *gin.Context) {
	info, err := h.svc.Create()
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	recordAudit(c, model.AuditActionBackupCreate, info.Name, "")
	response.Created(c, gin.H{"data": info})
}

// List GET /backups
func (h *BackupHandler) List(c *gin.Context) {
	response.Success(c, gin.H{"data": h.svc.List()})
}

// Download GET /backups/:name/download
func (h *BackupHandler) Download(c *gin.Context) {
	name := c.Param("name")
	path, err := h.svc.DownloadPath(name)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	c.FileAttachment(path, name)
}

// Delete DELETE /backups/:name
func (h *BackupHandler) Delete(c *gin.Context) {
	name := c.Param("name")
	if err := h.svc.Delete(name); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	recordAudit(c, model.AuditActionBackupDelete, name, "")
	response.Success(c, gin.H{"message": "已删除"})
}

// Restore POST /backups/restore (multipart/form-data: file)
func (h *BackupHandler) Restore(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "请选择备份文件")
		return
	}
	// 手动保存到备份目录(gin 的 SaveUploadedFile 会 chmod 父目录,系统 TMPDIR 不允许)。
	src, err := file.Open()
	if err != nil {
		response.BadRequest(c, "读取上传文件失败")
		return
	}
	defer src.Close()

	tmp := filepath.Join(config.C.Backup.Dir, fmt.Sprintf(".restore-%d.tmp", time.Now().UnixNano()))
	out, err := os.Create(tmp)
	if err != nil {
		response.InternalError(c, "保存上传文件失败")
		return
	}
	if _, err := io.Copy(out, src); err != nil {
		out.Close()
		_ = os.Remove(tmp)
		response.InternalError(c, "保存上传文件失败")
		return
	}
	out.Close()
	defer os.Remove(tmp)

	if err := h.svc.Restore(tmp); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	recordAudit(c, model.AuditActionBackupRestore, file.Filename, "")
	response.Success(c, gin.H{"message": "恢复成功"})
}

// RestoreByName POST /backups/:name/restore 用备份目录内已有文件恢复。
func (h *BackupHandler) RestoreByName(c *gin.Context) {
	name := c.Param("name")
	path, err := h.svc.DownloadPath(name)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	if err := h.svc.Restore(path); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	recordAudit(c, model.AuditActionBackupRestore, name, "")
	response.Success(c, gin.H{"message": "恢复成功"})
}

// GetSettings GET /backups/settings
func (h *BackupHandler) GetSettings(c *gin.Context) {
	response.Success(c, gin.H{"data": h.setting.GetAll()})
}

// UpdateSettings PUT /backups/settings
func (h *BackupHandler) UpdateSettings(c *gin.Context) {
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	changed := false
	for k, v := range req {
		if k == "backup_schedule_enabled" || k == "backup_schedule_cron" || k == "backup_keep" {
			if err := h.setting.Set(k, v); err != nil {
				response.InternalError(c, err.Error())
				return
			}
			changed = true
		}
	}
	if changed {
		service.GetBackupService().UpdateScheduledBackup()
	}
	recordAudit(c, model.AuditActionBackupSetting, "backup_settings", "")
	response.Success(c, gin.H{"message": "已保存"})
}

func (h *BackupHandler) RegisterRoutes(r *gin.RouterGroup) {
	g := r.Group("/backups", middleware.JWTAuth())
	g.POST("", h.Create)
	g.GET("", h.List)
	g.GET("/settings", h.GetSettings)
	g.PUT("/settings", h.UpdateSettings)
	g.GET("/:name/download", h.Download)
	g.DELETE("/:name", h.Delete)
	g.POST("/restore", h.Restore)
	g.POST("/:name/restore", h.RestoreByName)
}