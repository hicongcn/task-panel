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

type TaskHandler struct {
	svc *service.TaskService
}

func NewTaskHandler() *TaskHandler {
	return &TaskHandler{svc: service.NewTaskService()}
}

func (h *TaskHandler) List(c *gin.Context) {
	tasks := h.svc.List(c.Query("keyword"), c.Query("status"))
	response.Success(c, gin.H{"data": tasks, "total": len(tasks)})
}

func (h *TaskHandler) Get(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	task, err := h.svc.Get(uint(id))
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	cronDesc := service.DescribeCron(task.CronExpression)
	response.Success(c, gin.H{"data": task, "cron_desc": cronDesc})
}

func (h *TaskHandler) Create(c *gin.Context) {
	var req struct {
		Name           string `json:"name" binding:"required"`
		Command        string `json:"command" binding:"required"`
		CronExpression string `json:"cron_expression" binding:"required"`
		Enabled        bool   `json:"enabled"`
		TimeoutSeconds int    `json:"timeout_seconds"`
		MaxRetries     int    `json:"max_retries"`
		RetryInterval  int    `json:"retry_interval"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	task, err := h.svc.Create(service.CreateInput{
		Name: req.Name, Command: req.Command, CronExpression: req.CronExpression,
		Enabled: req.Enabled, TimeoutSeconds: req.TimeoutSeconds,
		MaxRetries: req.MaxRetries, RetryInterval: req.RetryInterval,
	})
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	recordAudit(c, model.AuditActionTaskCreate, fmt.Sprintf("task:%d", task.ID), task.Name)
	response.Created(c, gin.H{"data": task})
}

func (h *TaskHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	in := service.UpdateInput{}
	if v, ok := req["name"].(string); ok {
		in.Name = &v
	}
	if v, ok := req["command"].(string); ok {
		in.Command = &v
	}
	if v, ok := req["cron_expression"].(string); ok {
		in.CronExpression = &v
	}
	if v, ok := req["enabled"].(bool); ok {
		in.Enabled = &v
	}
	if v, ok := req["timeout_seconds"]; ok {
		if n := toInt(v); n != nil {
			in.TimeoutSeconds = n
		}
	}
	if v, ok := req["max_retries"]; ok {
		if n := toInt(v); n != nil {
			in.MaxRetries = n
		}
	}
	if v, ok := req["retry_interval"]; ok {
		if n := toInt(v); n != nil {
			in.RetryInterval = n
		}
	}
	task, err := h.svc.Update(uint(id), in)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	recordAudit(c, model.AuditActionTaskUpdate, fmt.Sprintf("task:%d", task.ID), task.Name)
	response.Success(c, gin.H{"data": task})
}

func (h *TaskHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if err := h.svc.Delete(uint(id)); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	recordAudit(c, model.AuditActionTaskDelete, fmt.Sprintf("task:%d", id), "")
	response.Success(c, gin.H{"message": "删除成功"})
}

func (h *TaskHandler) Enable(c *gin.Context) {
	h.toggle(c, true)
}

func (h *TaskHandler) Disable(c *gin.Context) {
	h.toggle(c, false)
}

func (h *TaskHandler) toggle(c *gin.Context, enable bool) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	task, err := h.svc.SetEnabled(uint(id), enable)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	action := model.AuditActionTaskDisable
	if enable {
		action = model.AuditActionTaskEnable
	}
	recordAudit(c, action, fmt.Sprintf("task:%d", id), task.Name)
	response.Success(c, gin.H{"message": "已更新", "data": task})
}

func (h *TaskHandler) Run(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if err := h.svc.Run(uint(id)); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	recordAudit(c, model.AuditActionTaskRun, fmt.Sprintf("task:%d", id), "")
	response.Success(c, gin.H{"message": "任务已启动"})
}

func (h *TaskHandler) Stop(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if err := h.svc.Stop(uint(id)); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	recordAudit(c, model.AuditActionTaskStop, fmt.Sprintf("task:%d", id), "")
	response.Success(c, gin.H{"message": "任务已停止"})
}

// CronDescribe POST /tasks/cron-describe 校验并返回可读描述。
func (h *TaskHandler) CronDescribe(c *gin.Context) {
	var req struct {
		Expression string `json:"expression" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	desc, err := service.ValidateAndDescribe(req.Expression)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"data": desc})
}

func (h *TaskHandler) RegisterRoutes(r *gin.RouterGroup) {
	tasks := r.Group("/tasks", middleware.JWTAuth())
	{
		tasks.GET("", h.List)
		tasks.GET("/:id", h.Get)
		tasks.POST("", h.Create)
		tasks.PUT("/:id", h.Update)
		tasks.DELETE("/:id", h.Delete)
		tasks.PUT("/:id/enable", h.Enable)
		tasks.PUT("/:id/disable", h.Disable)
		tasks.PUT("/:id/run", h.Run)
		tasks.PUT("/:id/stop", h.Stop)
		tasks.POST("/cron-describe", h.CronDescribe)
	}
}

// toInt 把 JSON 数值(interface{})安全转成 *int。
func toInt(v interface{}) *int {
	switch n := v.(type) {
	case float64:
		i := int(n)
		return &i
	case int:
		i := n
		return &i
	}
	return nil
}
