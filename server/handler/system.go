package handler

import (
	"runtime"
	"time"

	"taskpanel/middleware"
	"taskpanel/pkg/response"

	"github.com/gin-gonic/gin"
)

const Version = "0.1.0"

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

func (h *SystemHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/health", h.Health)
	sys := r.Group("/system", middleware.JWTAuth())
	{
		sys.GET("/version", h.Version)
	}
}
