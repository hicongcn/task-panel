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

type NotifyHandler struct {
	svc *service.NotifyService
}

func NewNotifyHandler() *NotifyHandler { return &NotifyHandler{svc: service.NewNotifyService()} }

// List GET /notify-channels
func (h *NotifyHandler) List(c *gin.Context) {
	response.Success(c, gin.H{"data": h.svc.List()})
}

// Create POST /notify-channels
func (h *NotifyHandler) Create(c *gin.Context) {
	var req struct {
		Name    string                 `json:"name" binding:"required"`
		Type    string                 `json:"type" binding:"required"`
		Enabled bool                   `json:"enabled"`
		Config  map[string]interface{} `json:"config"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	ch, err := h.svc.Create(req.Name, req.Type, req.Enabled, req.Config)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	recordAudit(c, model.AuditActionNotifyCreate, fmt.Sprintf("notify:%d", ch.ID), ch.Name)
	response.Created(c, gin.H{"data": ch})
}

// Update PUT /notify-channels/:id
func (h *NotifyHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	var name, typ string
	var enabled *bool
	var config map[string]interface{}
	if v, ok := req["name"].(string); ok {
		name = v
	}
	if v, ok := req["type"].(string); ok {
		typ = v
	}
	if v, ok := req["enabled"].(bool); ok {
		enabled = &v
	}
	if v, ok := req["config"].(map[string]interface{}); ok {
		config = v
	}
	ch, err := h.svc.Update(uint(id), name, typ, enabled, config)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	recordAudit(c, model.AuditActionNotifyUpdate, fmt.Sprintf("notify:%d", id), ch.Name)
	response.Success(c, gin.H{"data": ch})
}

// Delete DELETE /notify-channels/:id
func (h *NotifyHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if err := h.svc.Delete(uint(id)); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	recordAudit(c, model.AuditActionNotifyDelete, fmt.Sprintf("notify:%d", id), "")
	response.Success(c, gin.H{"message": "删除成功"})
}

// Toggle PUT /notify-channels/:id/toggle
func (h *NotifyHandler) Toggle(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	ch, err := h.svc.Toggle(uint(id), req.Enabled)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	recordAudit(c, model.AuditActionNotifyToggle, fmt.Sprintf("notify:%d", id), ch.Name)
	response.Success(c, gin.H{"data": ch})
}

// Test POST /notify-channels/test 同步发送测试消息。
func (h *NotifyHandler) Test(c *gin.Context) {
	var req struct {
		Type   string                 `json:"type" binding:"required"`
		Config map[string]interface{} `json:"config"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	if err := h.svc.Test(req.Type, req.Config); err != nil {
		response.BadRequest(c, "发送失败: "+err.Error())
		return
	}
	response.Success(c, gin.H{"message": "发送成功"})
}

func (h *NotifyHandler) RegisterRoutes(r *gin.RouterGroup) {
	g := r.Group("/notify-channels", middleware.JWTAuth())
	g.GET("", h.List)
	g.POST("", h.Create)
	g.POST("/test", h.Test)
	g.PUT("/:id", h.Update)
	g.DELETE("/:id", h.Delete)
	g.PUT("/:id/toggle", h.Toggle)
}
