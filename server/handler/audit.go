package handler

import (
	"strconv"

	"taskpanel/middleware"
	"taskpanel/pkg/response"
	"taskpanel/service"

	"github.com/gin-gonic/gin"
)

type AuditHandler struct {
	svc *service.AuditService
}

func NewAuditHandler() *AuditHandler { return &AuditHandler{svc: service.NewAuditService()} }

// List GET /audit-logs?username=&action=&page=&page_size=
func (h *AuditHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	logs, total := h.svc.List(c.Query("username"), c.Query("action"), page, pageSize)
	response.Success(c, gin.H{"data": logs, "total": total})
}

func (h *AuditHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.Group("/audit-logs", middleware.JWTAuth()).GET("", h.List)
}

// recordAudit 从当前上下文取用户名与 IP,记录一条审计日志(供各 handler 复用)。
func recordAudit(c *gin.Context, action, resource, detail string) {
	username, _ := c.Get("username")
	user := ""
	if username != nil {
		user = username.(string)
	}
	service.NewAuditService().Record(user, action, resource, detail, middleware.ResolveClientIP(c))
}
