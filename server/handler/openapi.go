package handler

import (
	"fmt"
	"strconv"

	"taskpanel/middleware"
	"taskpanel/model"
	"taskpanel/pkg/response"
	"taskpanel/service"

	"github.com/gin-gonic/gin"
)

type OpenAPIHandler struct {
	svc *service.OpenAPIService
}

func NewOpenAPIHandler() *OpenAPIHandler { return &OpenAPIHandler{svc: service.NewOpenAPIService()} }

// ---- 应用管理(需登录) ----

// List GET /open/apps
func (h *OpenAPIHandler) List(c *gin.Context) {
	response.Success(c, gin.H{"data": h.svc.List(), "scopes": service.ValidScopes()})
}

// Create POST /open/apps {name, scopes}
func (h *OpenAPIHandler) Create(c *gin.Context) {
	var req struct {
		Name   string   `json:"name" binding:"required"`
		Scopes []string `json:"scopes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	app, secret, err := h.svc.Create(req.Name, req.Scopes)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	recordAudit(c, model.AuditActionOpenAppCreate, app.Name, "")
	response.Created(c, gin.H{"data": gin.H{
		"id": app.ID, "name": app.Name, "client_id": app.ClientID,
		"client_secret": secret, "enabled": app.Enabled, "created_at": app.CreatedAt,
		"secret_warning": "client_secret 仅显示一次,请立即保存",
	}})
}

// Update PUT /open/apps/:id {name?, scopes?}
func (h *OpenAPIHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	var name *string
	var scopes []string
	scopesSet := false
	if v, ok := req["name"].(string); ok {
		name = &v
	}
	if v, ok := req["scopes"].([]interface{}); ok {
		scopesSet = true
		for _, item := range v {
			if s, ok := item.(string); ok {
				scopes = append(scopes, s)
			}
		}
	}
	if !scopesSet {
		scopes = nil
	}
	if err := h.svc.Update(uint(id), name, scopes); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	recordAudit(c, model.AuditActionOpenAppUpdate, "", "")
	response.Success(c, gin.H{"message": "已更新"})
}

// Delete DELETE /open/apps/:id
func (h *OpenAPIHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if err := h.svc.Delete(uint(id)); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	recordAudit(c, model.AuditActionOpenAppDelete, "", "")
	response.Success(c, gin.H{"message": "已删除"})
}

// ResetSecret PUT /open/apps/:id/reset-secret
func (h *OpenAPIHandler) ResetSecret(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	secret, err := h.svc.ResetSecret(uint(id))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	recordAudit(c, model.AuditActionOpenAppReset, "", "")
	response.Success(c, gin.H{"data": gin.H{
		"client_secret": secret, "secret_warning": "client_secret 仅显示一次,请立即保存",
	}})
}

// ---- 认证(公开) ----

// AuthToken POST /open/auth/token {client_id, client_secret}
func (h *OpenAPIHandler) AuthToken(c *gin.Context) {
	var req struct {
		ClientID     string `json:"client_id" binding:"required"`
		ClientSecret string `json:"client_secret" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	token, expiresAt, err := h.svc.Token(req.ClientID, req.ClientSecret)
	if err != nil {
		recordAudit(c, model.AuditActionOpenAuthFail, "auth", req.ClientID)
		service.GetNotifyService().NotifyEvent("OpenAPI 认证失败",
			fmt.Sprintf("应用 %q 尝试换取令牌失败: %s", req.ClientID, err.Error()))
		response.Unauthorized(c, err.Error())
		return
	}
	response.Success(c, gin.H{"data": gin.H{
		"token": token, "token_type": "bearer", "expires_at": expiresAt,
	}})
}

// ---- 开放接口(OpenAuth + scope) ----

func (h *OpenAPIHandler) Tasks(c *gin.Context) {
	response.Success(c, gin.H{"data": service.NewTaskService().List("", "", "")})
}

func (h *OpenAPIHandler) RunTask(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if err := service.NewTaskService().Run(uint(id)); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"message": "任务已启动"})
}

func (h *OpenAPIHandler) Logs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	logs, total := service.NewLogService().List(0, page, pageSize)
	response.Success(c, gin.H{"data": logs, "total": total})
}

func (h *OpenAPIHandler) Envs(c *gin.Context) {
	response.Success(c, gin.H{"data": service.NewEnvService().List("", "")})
}

func (h *OpenAPIHandler) RegisterRoutes(r *gin.RouterGroup) {
	// 应用管理(需登录)
	apps := r.Group("/open/apps", middleware.JWTAuth())
	{
		apps.GET("", h.List)
		apps.POST("", h.Create)
		apps.PUT("/:id", h.Update)
		apps.DELETE("/:id", h.Delete)
		apps.PUT("/:id/reset-secret", h.ResetSecret)
	}

	// 令牌(公开)
	r.POST("/open/auth/token", h.AuthToken)

	// 开放接口(OpenAuth + scope)
	open := r.Group("/open", middleware.OpenAuth())
	{
		open.GET("/tasks", middleware.RequireScope(model.ScopeTasksRead), h.Tasks)
		open.POST("/tasks/:id/run", middleware.RequireScope(model.ScopeTasksRun), h.RunTask)
		open.GET("/logs", middleware.RequireScope(model.ScopeLogsRead), h.Logs)
		open.GET("/envs", middleware.RequireScope(model.ScopeEnvsRead), h.Envs)
	}
}
