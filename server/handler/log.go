package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"taskpanel/config"
	"taskpanel/middleware"
	"taskpanel/pkg/dlticket"
	"taskpanel/pkg/response"
	"taskpanel/service"

	"github.com/gin-gonic/gin"
)

type LogHandler struct {
	svc *service.LogService
}

func NewLogHandler() *LogHandler {
	return &LogHandler{svc: service.NewLogService()}
}

// List GET /logs?task_id=&page=&page_size=
func (h *LogHandler) List(c *gin.Context) {
	taskID, _ := strconv.ParseUint(c.Query("task_id"), 10, 32)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	logs, total := h.svc.List(uint(taskID), page, pageSize)
	response.Success(c, gin.H{"data": logs, "total": total})
}

// Detail GET /logs/:id
func (h *LogHandler) Detail(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	log, content, err := h.svc.Get(uint(id))
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.Success(c, gin.H{"data": log, "content": content})
}

// Latest GET /tasks/:id/latest-log
func (h *LogHandler) Latest(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	log, err := h.svc.LatestLog(uint(id))
	if err != nil || log == nil {
		response.Success(c, gin.H{"data": nil})
		return
	}
	response.Success(c, gin.H{"data": log})
}

// LiveTicket GET /tasks/:id/live-ticket  (需登录)
// 签发短期 SSE 票据。浏览器 EventSource 无法携带 Authorization 头,若直接把 JWT 放
// query,会泄漏到服务器访问日志与浏览器历史,故改用短期 HMAC 票据(与日志下载一致)。
func (h *LogHandler) LiveTicket(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	username, _ := c.Get("username")
	ticket, expiresAt, err := dlticket.Issue(config.C.JWT.Secret, liveResource(id), username.(string), liveTicketTTL)
	if err != nil {
		response.InternalError(c, "签发实时日志票据失败")
		return
	}
	response.Success(c, gin.H{
		"ticket":     ticket,
		"expires_at": expiresAt.Format(time.RFC3339),
	})
}

// LiveLogs GET /tasks/:id/live-logs?ticket=<hmac ticket>  (SSE)
// 订阅当前运行中任务的实时日志;任务未运行或已结束时返回 done 事件。
// 票据经 LiveTicket 接口签发,短时有效且绑定任务 ID,避免长时 JWT 暴露。
func (h *LogHandler) LiveLogs(c *gin.Context) {
	ticket := strings.TrimSpace(c.Query("ticket"))
	if ticket == "" {
		writeSSE(c, "error", "unauthorized")
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if _, err := dlticket.Verify(config.C.JWT.Secret, ticket, liveResource(id)); err != nil {
		writeSSE(c, "error", "unauthorized")
		return
	}

	log, err := h.svc.LatestRunningLog(uint(id))
	if err != nil || log == nil {
		writeSSE(c, "done", "not_running")
		return
	}

	broker := service.GetLogBroker().Get(log.ID)
	if broker == nil {
		writeSSE(c, "done", "finished")
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	history, ch := broker.Subscribe()
	defer broker.Unsubscribe(ch)

	// 先回放历史
	for _, line := range history {
		if line == "\x00DONE" {
			continue
		}
		writeSSE(c, "data", line)
	}
	c.Writer.Flush()

	// 实时转发
	ctx := c.Request.Context()
	for {
		select {
		case line, ok := <-ch:
			if !ok {
				writeSSE(c, "done", "closed")
				c.Writer.Flush()
				return
			}
			if line == "\x00DONE" {
				writeSSE(c, "done", "finished")
				c.Writer.Flush()
				return
			}
			writeSSE(c, "data", line)
			c.Writer.Flush()
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Minute):
			writeSSE(c, "done", "timeout")
			c.Writer.Flush()
			return
		}
	}
}

// RawTicket GET /logs/:id/raw-ticket  (需登录)
// 签发短期下载票据,返回可直接用于浏览器下载的 URL。
func (h *LogHandler) RawTicket(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	full, _, err := h.svc.RawFilePath(uint(id))
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	username, _ := c.Get("username")
	ticket, expiresAt, err := dlticket.Issue(config.C.JWT.Secret, logResource(id), username.(string), dlticket.DefaultTTL)
	if err != nil {
		response.InternalError(c, "签发下载票据失败")
		return
	}
	response.Success(c, gin.H{
		"url":        fmt.Sprintf("/api/v1/logs/%d/raw?ticket=%s", id, ticket),
		"expires_at": expiresAt.Format(time.RFC3339),
	})
	_ = full
}

// DownloadRaw GET /logs/:id/raw?ticket=  (仅认票据)
func (h *LogHandler) DownloadRaw(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	ticket := strings.TrimSpace(c.Query("ticket"))
	if ticket == "" {
		response.Unauthorized(c, "缺少下载票据")
		return
	}
	if _, err := dlticket.Verify(config.C.JWT.Secret, ticket, logResource(id)); err != nil {
		response.Unauthorized(c, "下载票据无效或已过期")
		return
	}
	full, baseName, err := h.svc.RawFilePath(uint(id))
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", baseName))
	c.Header("Cache-Control", "no-store")
	http.ServeFile(c.Writer, c.Request, full)
}

func (h *LogHandler) RegisterRoutes(r *gin.RouterGroup) {
	// raw 下载只认票据(浏览器下载无法带 Authorization),单独注册在鉴权之外。
	r.GET("/logs/:id/raw", h.DownloadRaw)

	logs := r.Group("/logs", middleware.JWTAuth())
	{
		logs.GET("", h.List)
		logs.GET("/:id", h.Detail)
		logs.GET("/:id/raw-ticket", h.RawTicket)
	}

	// SSE 实时流:EventSource 无法带 Authorization,先经鉴权接口换取短期票据再订阅。
	tasks := r.Group("/tasks")
	{
		tasks.GET("/:id/latest-log", middleware.JWTAuth(), h.Latest)
		tasks.GET("/:id/live-ticket", middleware.JWTAuth(), h.LiveTicket)
		tasks.GET("/:id/live-logs", h.LiveLogs)
	}
}

func logResource(id uint64) string {
	return fmt.Sprintf("log:%d", id)
}

func liveResource(id uint64) string {
	return fmt.Sprintf("live:%d", id)
}

// liveTicketTTL 是实时日志票据有效期(秒)。SSE 单连接最长 5 分钟,票据需覆盖
// 建立连接 + 前端有限次重连的窗口。
const liveTicketTTL = 10 * time.Minute

// writeSSE 写一条 SSE 事件。
func writeSSE(c *gin.Context, event, data string) {
	fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event, data)
}
