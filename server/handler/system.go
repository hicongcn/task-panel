package handler

import (
	"runtime"
	"time"

	"taskpanel/middleware"
	"taskpanel/model"
	"taskpanel/pkg/response"
	"taskpanel/service"

	"github.com/gin-gonic/gin"
)

const Version = "1.0.0"

type SystemHandler struct{}

func NewSystemHandler() *SystemHandler { return &SystemHandler{} }

// Health GET /health
func (h *SystemHandler) Health(c *gin.Context) {
	c.JSON(200, gin.H{"status": "ok"})
}

// Version GET /system/version
func (h *SystemHandler) Version(c *gin.Context) {
	response.Success(c, gin.H{"version": Version, "go_version": runtime.Version(),
		"build_time": time.Now().Format("2006-01-02")})
}

// Stats GET /system/stats 系统监控快照(后台采样缓存,零阻塞)。
func (h *SystemHandler) Stats(c *gin.Context) {
	response.Success(c, gin.H{"data": service.GetSysMonitor().Stats()})
}

// PanelInfo GET /system/panel 公开的面板信息(标题/图标,登录页展示用)。
func (h *SystemHandler) PanelInfo(c *gin.Context) {
	cfg := service.NewSystemConfigService().GetConfig()
	response.Success(c, gin.H{"data": gin.H{
		"panel_title": cfg["panel_title"], "panel_logo": cfg["panel_logo"],
	}})
}

// Config GET /system/config 面板配置(标题/图标/日志清理)。
func (h *SystemHandler) Config(c *gin.Context) {
	response.Success(c, gin.H{"data": service.NewSystemConfigService().GetConfig()})
}

// UpdateConfig PUT /system/config 保存面板配置。
func (h *SystemHandler) UpdateConfig(c *gin.Context) {
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	if err := service.NewSystemConfigService().UpdateConfig(req); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	recordAudit(c, model.AuditActionSystemConfig, "system", "")
	response.Success(c, gin.H{"message": "已保存"})
}

func (h *SystemHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/health", h.Health)
	r.GET("/system/panel", h.PanelInfo) // 公开:登录页展示标题/图标
	sys := r.Group("/system", middleware.JWTAuth())
	{
		sys.GET("/version", h.Version)
		sys.GET("/stats", h.Stats)
		sys.GET("/config", h.Config)
		sys.PUT("/config", h.UpdateConfig)
	}
}
