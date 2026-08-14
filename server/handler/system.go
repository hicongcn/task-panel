package handler

import (
	"runtime"
	"time"

	"taskpanel/middleware"
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

func (h *SystemHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/health", h.Health)
	sys := r.Group("/system", middleware.JWTAuth())
	{
		sys.GET("/version", h.Version)
		sys.GET("/stats", h.Stats)
	}
}
