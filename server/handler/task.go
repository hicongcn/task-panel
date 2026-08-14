package handler

import (
	"fmt"
	"strings"
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
	tasks := h.svc.List(c.Query("keyword"), c.Query("status"), c.Query("tag"))
	response.Success(c, gin.H{"data": tasks, "total": len(tasks)})
}

// Tags GET /tasks/tags 返回所有标签及计数。
func (h *TaskHandler) Tags(c *gin.Context) {
	response.Success(c, gin.H{"data": h.svc.TagStats()})
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
		Name           string   `json:"name" binding:"required"`
		Command        string   `json:"command" binding:"required"`
		CronExpression string   `json:"cron_expression" binding:"required"`
		Enabled        bool     `json:"enabled"`
		TimeoutSeconds int      `json:"timeout_seconds"`
		MaxRetries     int      `json:"max_retries"`
		RetryInterval  int      `json:"retry_interval"`
		Tags           []string `json:"tags"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	task, err := h.svc.Create(service.CreateInput{
		Name: req.Name, Command: req.Command, CronExpression: req.CronExpression,
		Enabled: req.Enabled, TimeoutSeconds: req.TimeoutSeconds,
		MaxRetries: req.MaxRetries, RetryInterval: req.RetryInterval,
		Tags: sanitizeTags(req.Tags),
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
	if v, ok := req["tags"].([]interface{}); ok {
		tags := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				tags = append(tags, s)
			}
		}
		tags = sanitizeTags(tags)
		in.Tags = &tags
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

// batchIDs 解析批量操作请求体 {ids: [...]}。
func batchIDs(c *gin.Context) ([]uint, bool) {
	var req struct {
		IDs []uint `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 {
		return nil, false
	}
	return req.IDs, true
}

// BatchEnable POST /tasks/batch/enable
func (h *TaskHandler) BatchEnable(c *gin.Context) {
	h.batchToggle(c, true)
}

// BatchDisable POST /tasks/batch/disable
func (h *TaskHandler) BatchDisable(c *gin.Context) {
	h.batchToggle(c, false)
}

func (h *TaskHandler) batchToggle(c *gin.Context, enable bool) {
	ids, ok := batchIDs(c)
	if !ok {
		response.BadRequest(c, "请求参数错误")
		return
	}
	res := h.svc.BatchSetEnabled(ids, enable)
	action := model.AuditActionTaskDisable
	if enable {
		action = model.AuditActionTaskEnable
	}
	recordAudit(c, action, fmt.Sprintf("tasks:%v", ids), "")
	response.Success(c, gin.H{"message": "批量操作完成", "data": res})
}

// BatchRun POST /tasks/batch/run
func (h *TaskHandler) BatchRun(c *gin.Context) {
	ids, ok := batchIDs(c)
	if !ok {
		response.BadRequest(c, "请求参数错误")
		return
	}
	res := h.svc.BatchRun(ids)
	recordAudit(c, model.AuditActionTaskRun, fmt.Sprintf("tasks:%v", ids), "")
	response.Success(c, gin.H{"message": "批量触发完成", "data": res})
}

// BatchDelete POST /tasks/batch/delete
func (h *TaskHandler) BatchDelete(c *gin.Context) {
	ids, ok := batchIDs(c)
	if !ok {
		response.BadRequest(c, "请求参数错误")
		return
	}
	res := h.svc.BatchDelete(ids)
	recordAudit(c, model.AuditActionTaskDelete, fmt.Sprintf("tasks:%v", ids), "")
	response.Success(c, gin.H{"message": "批量删除完成", "data": res})
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
		tasks.GET("/tags", h.Tags)
		tasks.GET("/:id", h.Get)
		tasks.POST("", h.Create)
		tasks.PUT("/:id", h.Update)
		tasks.DELETE("/:id", h.Delete)
		tasks.PUT("/:id/enable", h.Enable)
		tasks.PUT("/:id/disable", h.Disable)
		tasks.PUT("/:id/run", h.Run)
		tasks.PUT("/:id/stop", h.Stop)
		tasks.POST("/cron-describe", h.CronDescribe)
		tasks.POST("/batch/enable", h.BatchEnable)
		tasks.POST("/batch/disable", h.BatchDisable)
		tasks.POST("/batch/run", h.BatchRun)
		tasks.POST("/batch/delete", h.BatchDelete)
	}
}

// sanitizeTags 清洗标签:去空白、去重、限制长度与数量。
func sanitizeTags(tags []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" || len(t) > 32 || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	if len(out) > 20 {
		out = out[:20]
	}
	return out
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
