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

type EnvHandler struct {
	svc *service.EnvService
}

func NewEnvHandler() *EnvHandler {
	return &EnvHandler{svc: service.NewEnvService()}
}

func (h *EnvHandler) List(c *gin.Context) {
	// 明文返回(面板为单管理员自用环境,用户要求直接显示值)
	data := h.svc.ListPlain(c.Query("keyword"), c.Query("group"))
	response.Success(c, gin.H{"data": data, "total": len(data)})
}

func (h *EnvHandler) Groups(c *gin.Context) {
	response.Success(c, gin.H{"data": h.svc.Groups()})
}

func (h *EnvHandler) Create(c *gin.Context) {
	var req struct {
		Name    string `json:"name" binding:"required"`
		Value   string `json:"value" binding:"required"`
		Group   string `json:"group"`
		Remark  string `json:"remark"`
		Enabled *bool  `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	env, err := h.svc.Create(req.Name, req.Value, req.Group, req.Remark, enabled)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	recordAudit(c, model.AuditActionEnvCreate, fmt.Sprintf("env:%d", env.ID), env.Name)
	response.Created(c, gin.H{"data": service.GinEnvDict(*env, true)})
}

func (h *EnvHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var req struct {
		Name    string `json:"name"`
		Value   string `json:"value"`
		Group   string `json:"group"`
		Remark  string `json:"remark"`
		Enabled *bool  `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	env, err := h.svc.Update(uint(id), req.Name, req.Value, req.Group, req.Remark, req.Enabled)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	recordAudit(c, model.AuditActionEnvUpdate, fmt.Sprintf("env:%d", id), env.Name)
	response.Success(c, gin.H{"data": service.GinEnvDict(*env, true)})
}

func (h *EnvHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if err := h.svc.Delete(uint(id)); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	recordAudit(c, model.AuditActionEnvDelete, fmt.Sprintf("env:%d", id), "")
	response.Success(c, gin.H{"message": "删除成功"})
}

func (h *EnvHandler) BatchDelete(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	n, err := h.svc.BatchDelete(req.IDs)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	recordAudit(c, model.AuditActionEnvBatchDel, fmt.Sprintf("envs:%v", req.IDs), "")
	response.Success(c, gin.H{"message": "已删除", "count": n})
}

// BatchEnable POST /envs/batch/enable
func (h *EnvHandler) BatchEnable(c *gin.Context) { h.batchToggle(c, true) }

// BatchDisable POST /envs/batch/disable
func (h *EnvHandler) BatchDisable(c *gin.Context) { h.batchToggle(c, false) }

func (h *EnvHandler) batchToggle(c *gin.Context, enable bool) {
	ids, ok := batchIDs(c)
	if !ok {
		response.BadRequest(c, "请求参数错误")
		return
	}
	n := h.svc.BatchSetEnabled(ids, enable)
	action := model.AuditActionEnvDisable
	if enable {
		action = model.AuditActionEnvEnable
	}
	recordAudit(c, action, fmt.Sprintf("envs:%v", ids), "")
	response.Success(c, gin.H{"message": "批量操作完成", "count": n})
}

// Reorder PUT /envs/reorder {ids: [...]} 拖拽排序后保存顺序。
func (h *EnvHandler) Reorder(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 {
		response.BadRequest(c, "请求参数错误")
		return
	}
	if err := h.svc.Reorder(req.IDs); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, gin.H{"message": "排序已保存"})
}

func (h *EnvHandler) RegisterRoutes(r *gin.RouterGroup) {
	envs := r.Group("/envs", middleware.JWTAuth())
	{
		envs.GET("", h.List)
		envs.GET("/groups", h.Groups)
		envs.POST("", h.Create)
		envs.PUT("/:id", h.Update)
		envs.DELETE("/:id", h.Delete)
		envs.DELETE("/batch", h.BatchDelete)
		envs.POST("/batch/enable", h.BatchEnable)
		envs.POST("/batch/disable", h.BatchDisable)
		envs.PUT("/reorder", h.Reorder)
	}
}
