package handler

// 青龙(whyour/qinglong)OpenAPI 兼容层。
// 按 qinglong.online 文档实现 /open 前缀接口:认证 GET /open/auth/token、
// 应用管理 /open/app、资源接口 /open/crontab|env|log|system|config|script|dependence|subscription。
// 响应统一青龙格式 {code:200, data:...}。

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"taskpanel/config"
	"taskpanel/middleware"
	"taskpanel/model"
	"taskpanel/pkg/pathutil"
	"taskpanel/service"

	"github.com/gin-gonic/gin"
)

// ---- 响应包装(青龙格式) ----

func qlSuccess(c *gin.Context, data interface{}) {
	c.JSON(200, gin.H{"code": 200, "data": data})
}

func qlFail(c *gin.Context, httpCode, code int, msg string) {
	c.JSON(httpCode, gin.H{"code": code, "message": msg})
}

// ---- scope 校验(兼容青龙模块权限与本项目原生 scope) ----

var qlScopeAliases = map[string][]string{
	"crontab:read":       {"crontab:read", "crontab", "tasks:read"},
	"crontab:write":      {"crontab:write", "crontab", "tasks:run"},
	"env:read":           {"env:read", "env", "envs:read"},
	"env:write":          {"env:write", "env"},
	"log:read":           {"log:read", "log", "logs:read"},
	"system:read":        {"system:read", "system"},
	"config:read":        {"config:read", "config"},
	"config:write":       {"config:write", "config"},
	"script:read":        {"script:read", "script"},
	"script:write":       {"script:write", "script"},
	"dependence:read":    {"dependence:read", "dependence"},
	"dependence:write":   {"dependence:write", "dependence"},
	"subscription:read":  {"subscription:read", "subscription"},
	"subscription:write": {"subscription:write", "subscription"},
}

func qlScope(scope string) gin.HandlerFunc {
	allowed := qlScopeAliases[scope]
	return func(c *gin.Context) {
		v, ok := c.Get("open_claims")
		claims, ok2 := v.(*middleware.Claims)
		if !ok || !ok2 {
			qlFail(c, 401, 401, "缺少令牌信息")
			c.Abort()
			return
		}
		for _, s := range claims.Scopes {
			for _, a := range allowed {
				if s == a {
					c.Next()
					return
				}
			}
		}
		qlFail(c, 403, 403, "无权限:需要 scope "+scope)
		c.Abort()
	}
}

// ---- 认证:GET/POST /open/auth/token ----

func qlAuthToken(c *gin.Context) {
	clientID := c.Query("client_id")
	clientSecret := c.Query("client_secret")
	if clientID == "" {
		var req struct {
			ClientID     string `json:"client_id"`
			ClientSecret string `json:"client_secret"`
		}
		_ = c.ShouldBindJSON(&req)
		clientID, clientSecret = req.ClientID, req.ClientSecret
	}
	if clientID == "" || clientSecret == "" {
		qlFail(c, 400, 400, "缺少 client_id 或 client_secret")
		return
	}
	token, expiresAt, err := service.NewOpenAPIService().Token(clientID, clientSecret)
	if err != nil {
		qlFail(c, 401, 401, err.Error())
		return
	}
	qlSuccess(c, gin.H{
		"token":      token,
		"token_type": "Bearer",
		"expiration": expiresAt.Unix(),
	})
}

// ---- 应用管理 /open/app(JWT) ----

func qlAppList(c *gin.Context) {
	qlSuccess(c, service.NewOpenAPIService().List())
}

func qlAppCreate(c *gin.Context) {
	var req struct {
		Name   string   `json:"name" binding:"required"`
		Scopes []string `json:"scopes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		qlFail(c, 400, 400, "请求参数错误")
		return
	}
	app, secret, err := service.NewOpenAPIService().Create(req.Name, req.Scopes)
	if err != nil {
		qlFail(c, 400, 400, err.Error())
		return
	}
	recordAudit(c, model.AuditActionOpenAppCreate, app.Name, "")
	qlSuccess(c, gin.H{
		"id": app.ID, "name": app.Name, "client_id": app.ClientID,
		"client_secret": secret, "enabled": app.Enabled, "created_at": app.CreatedAt,
	})
}

func qlAppUpdate(c *gin.Context) {
	var req struct {
		ID     uint     `json:"id" binding:"required"`
		Name   *string  `json:"name"`
		Scopes []string `json:"scopes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ID == 0 {
		qlFail(c, 400, 400, "请求参数错误(id 必填)")
		return
	}
	var scopes []string
	if req.Scopes != nil {
		scopes = req.Scopes
	}
	if err := service.NewOpenAPIService().Update(req.ID, req.Name, scopes); err != nil {
		qlFail(c, 400, 400, err.Error())
		return
	}
	recordAudit(c, model.AuditActionOpenAppUpdate, "", "")
	qlSuccess(c, gin.H{"message": "已更新"})
}

func qlAppBatchDelete(c *gin.Context) {
	var ids []uint
	if err := c.ShouldBindJSON(&ids); err != nil || len(ids) == 0 {
		qlFail(c, 400, 400, "请求体应为应用ID数组")
		return
	}
	for _, id := range ids {
		_ = service.NewOpenAPIService().Delete(id)
	}
	recordAudit(c, model.AuditActionOpenAppDelete, fmt.Sprintf("apps:%v", ids), "")
	qlSuccess(c, gin.H{"message": "已删除"})
}

func qlAppResetSecret(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	secret, err := service.NewOpenAPIService().ResetSecret(uint(id))
	if err != nil {
		qlFail(c, 400, 400, err.Error())
		return
	}
	recordAudit(c, model.AuditActionOpenAppReset, "", "")
	qlSuccess(c, gin.H{"client_secret": secret})
}

// ---- /open/crontab(定时任务) ----

func unixMS(t *time.Time) interface{} {
	if t == nil {
		return nil
	}
	return t.UnixMilli()
}

func unixT(t time.Time) int64 { return t.UnixMilli() }

func qlRunStatus(s string) interface{} {
	switch s {
	case "success":
		return "success"
	case "failed":
		return "failure"
	case "aborted":
		return "timeout"
	default:
		return nil
	}
}

func qlCronDict(t model.Task) map[string]interface{} {
	return map[string]interface{}{
		"id":                  t.ID,
		"name":                t.Name,
		"command":             t.Command,
		"schedule":            t.CronExpression,
		"labels":              []string(t.Tags),
		"isDisabled":          !t.Enabled,
		"last_execution_time": unixMS(t.LastRunAt),
		"last_run_time":       unixMS(t.LastRunAt),
		"last_result":         qlRunStatus(t.LastRunStatus),
		"created":             unixT(t.CreatedAt),
		"updated":             unixT(t.UpdatedAt),
	}
}

func qlCronList(c *gin.Context) {
	keyword := c.Query("searchValue")
	tasks := service.NewTaskService().List(keyword, "", "")
	out := make([]map[string]interface{}, len(tasks))
	for i, t := range tasks {
		out[i] = qlCronDict(t)
	}
	qlSuccess(c, out)
}

func qlCronCreate(c *gin.Context) {
	var req struct {
		Name     string   `json:"name"`
		Command  string   `json:"command"`
		Schedule string   `json:"schedule"`
		Labels   []string `json:"labels"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" || req.Command == "" || req.Schedule == "" {
		qlFail(c, 400, 400, "name/command/schedule 必填")
		return
	}
	task, err := service.NewTaskService().Create(service.CreateInput{
		Name: req.Name, Command: req.Command, CronExpression: req.Schedule,
		Enabled: false, Tags: req.Labels,
	})
	if err != nil {
		qlFail(c, 400, 400, err.Error())
		return
	}
	recordAudit(c, model.AuditActionTaskCreate, fmt.Sprintf("task:%d", task.ID), task.Name)
	qlSuccess(c, qlCronDict(*task))
}

func qlCronUpdate(c *gin.Context) {
	var req struct {
		ID         uint      `json:"id" binding:"required"`
		Name       *string   `json:"name"`
		Command    *string   `json:"command"`
		Schedule   *string   `json:"schedule"`
		Labels     *[]string `json:"labels"`
		IsDisabled *bool     `json:"isDisabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ID == 0 {
		qlFail(c, 400, 400, "id 必填")
		return
	}
	var enabled *bool
	if req.IsDisabled != nil {
		v := !*req.IsDisabled
		enabled = &v
	}
	task, err := service.NewTaskService().Update(req.ID, service.UpdateInput{
		Name: req.Name, Command: req.Command, CronExpression: req.Schedule,
		Enabled: enabled, Tags: req.Labels,
	})
	if err != nil {
		qlFail(c, 400, 400, err.Error())
		return
	}
	recordAudit(c, model.AuditActionTaskUpdate, fmt.Sprintf("task:%d", task.ID), task.Name)
	qlSuccess(c, qlCronDict(*task))
}

func qlCronBatchDelete(c *gin.Context) {
	var ids []uint
	if err := c.ShouldBindJSON(&ids); err != nil || len(ids) == 0 {
		qlFail(c, 400, 400, "请求体应为任务ID数组")
		return
	}
	res := service.NewTaskService().BatchDelete(ids)
	recordAudit(c, model.AuditActionTaskDelete, fmt.Sprintf("tasks:%v", ids), "")
	qlSuccess(c, gin.H{"message": "已删除", "deleted": res.OK})
}

func qlCronRun(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if err := service.NewTaskService().Run(uint(id)); err != nil {
		qlFail(c, 400, 400, err.Error())
		return
	}
	recordAudit(c, model.AuditActionTaskRun, fmt.Sprintf("task:%d", id), "")
	qlSuccess(c, gin.H{"message": "任务已启动"})
}

func qlCronStop(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if err := service.NewTaskService().Stop(uint(id)); err != nil {
		qlFail(c, 400, 400, err.Error())
		return
	}
	recordAudit(c, model.AuditActionTaskStop, fmt.Sprintf("task:%d", id), "")
	qlSuccess(c, gin.H{"message": "任务已停止"})
}

func qlCronEnable(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if _, err := service.NewTaskService().SetEnabled(uint(id), true); err != nil {
		qlFail(c, 400, 400, err.Error())
		return
	}
	qlSuccess(c, gin.H{"message": "已启用"})
}

func qlCronDisable(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if _, err := service.NewTaskService().SetEnabled(uint(id), false); err != nil {
		qlFail(c, 400, 400, err.Error())
		return
	}
	qlSuccess(c, gin.H{"message": "已禁用"})
}

// ---- /open/env(环境变量) ----

func qlEnvDict(e model.EnvVar) map[string]interface{} {
	status := 1
	if e.Enabled {
		status = 0
	}
	return map[string]interface{}{
		"id":        e.ID,
		"name":      e.Name,
		"value":     e.Value,
		"remarks":   e.Remark,
		"status":    status,
		"created":   unixT(e.CreatedAt),
		"updated":   unixT(e.UpdatedAt),
		"timestamp": unixT(e.CreatedAt),
	}
}

func qlEnvList(c *gin.Context) {
	search := c.Query("searchValue")
	list := service.NewEnvService().ListPlain(search, "")
	out := make([]map[string]interface{}, len(list))
	for i, m := range list {
		out[i] = qlEnvDictFromMap(m)
	}
	qlSuccess(c, out)
}

func qlEnvDictFromMap(m map[string]interface{}) map[string]interface{} {
	id, _ := m["id"].(uint)
	name, _ := m["name"].(string)
	value, _ := m["value"].(string)
	remark, _ := m["remark"].(string)
	enabled, _ := m["enabled"].(bool)
	createdAt, _ := m["created_at"].(time.Time)
	status := 1
	if enabled {
		status = 0
	}
	return map[string]interface{}{
		"id": id, "name": name, "value": value, "remarks": remark,
		"status": status, "created": unixT(createdAt), "updated": unixT(createdAt),
		"timestamp": unixT(createdAt),
	}
}

func qlEnvCreate(c *gin.Context) {
	var items []struct {
		Name    string `json:"name"`
		Value   string `json:"value"`
		Remarks string `json:"remarks"`
	}
	if err := c.ShouldBindJSON(&items); err != nil || len(items) == 0 {
		qlFail(c, 400, 400, "请求体应为环境变量数组")
		return
	}
	var created []map[string]interface{}
	for _, it := range items {
		env, err := service.NewEnvService().Create(it.Name, it.Value, "", it.Remarks, true)
		if err != nil {
			qlFail(c, 400, 400, err.Error())
			return
		}
		recordAudit(c, model.AuditActionEnvCreate, fmt.Sprintf("env:%d", env.ID), env.Name)
		created = append(created, qlEnvDict(*env))
	}
	qlSuccess(c, created)
}

func qlEnvUpdate(c *gin.Context) {
	var req struct {
		ID      uint   `json:"id" binding:"required"`
		Name    string `json:"name"`
		Value   string `json:"value"`
		Remarks string `json:"remarks"`
		Status  *int   `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ID == 0 {
		qlFail(c, 400, 400, "id 必填")
		return
	}
	var enabled *bool
	if req.Status != nil {
		v := *req.Status == 0
		enabled = &v
	}
	env, err := service.NewEnvService().Update(req.ID, req.Name, req.Value, "", req.Remarks, enabled)
	if err != nil {
		qlFail(c, 400, 400, err.Error())
		return
	}
	recordAudit(c, model.AuditActionEnvUpdate, fmt.Sprintf("env:%d", env.ID), env.Name)
	qlSuccess(c, qlEnvDict(*env))
}

func qlEnvBatchDelete(c *gin.Context) {
	var ids []uint
	if err := c.ShouldBindJSON(&ids); err != nil || len(ids) == 0 {
		qlFail(c, 400, 400, "请求体应为环境变量ID数组")
		return
	}
	n, err := service.NewEnvService().BatchDelete(ids)
	if err != nil {
		qlFail(c, 500, 500, err.Error())
		return
	}
	recordAudit(c, model.AuditActionEnvBatchDel, fmt.Sprintf("envs:%v", ids), "")
	qlSuccess(c, gin.H{"message": "已删除", "deleted": n})
}

func qlEnvStatus(c *gin.Context) {
	var req struct {
		ID     uint `json:"id" binding:"required"`
		Status *int `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ID == 0 || req.Status == nil {
		qlFail(c, 400, 400, "id 与 status 必填")
		return
	}
	v := *req.Status == 0
	env, err := service.NewEnvService().Update(req.ID, "", "", "", "", &v)
	if err != nil {
		qlFail(c, 400, 400, err.Error())
		return
	}
	recordAudit(c, model.AuditActionEnvUpdate, fmt.Sprintf("env:%d", env.ID), env.Name)
	qlSuccess(c, qlEnvDict(*env))
}

// ---- /open/log(日志) ----

func qlLogList(c *gin.Context) {
	qlSuccess(c, logFileList())
}

func logFileList() []map[string]string {
	dir := config.C.Data.LogDir
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []map[string]string{}
	}
	var files []map[string]string
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		files = append(files, map[string]string{"title": e.Name(), "value": filepath.Join(dir, e.Name())})
	}
	sort.Slice(files, func(i, j int) bool { return files[i]["title"] < files[j]["title"] })
	return files
}

func qlLogDetail(c *gin.Context) {
	file := c.Query("file")
	if file == "" {
		qlFail(c, 400, 400, "缺少 file 参数")
		return
	}
	full, err := pathutil.SafeJoin(config.C.Data.LogDir, filepath.Base(file), true)
	if err != nil || full == "" {
		qlFail(c, 403, 403, "日志文件访问受限")
		return
	}
	b, err := os.ReadFile(full)
	if err != nil {
		qlFail(c, 404, 404, "日志文件不存在")
		return
	}
	qlSuccess(c, gin.H{"content": string(b)})
}

func qlLogFile(c *gin.Context) {
	file := c.Param("file")
	full, err := pathutil.SafeJoin(config.C.Data.LogDir, filepath.Base(file), true)
	if err != nil || full == "" {
		qlFail(c, 403, 403, "日志文件访问受限")
		return
	}
	b, err := os.ReadFile(full)
	if err != nil {
		qlFail(c, 404, 404, "日志文件不存在")
		return
	}
	c.Data(200, "text/plain; charset=utf-8", b)
}

// ---- /open/system(系统信息) ----

func qlSystemInfo(c *gin.Context) {
	qlSuccess(c, gin.H{
		"isInitialized": !service.NewAuthService().NeedInit(),
		"version":       Version,
		"publishTime":   time.Now().UnixMilli(),
		"branch":        "main",
		"changeLog":     "task-panel 青龙兼容版",
	})
}

func qlSystemConfig(c *gin.Context) {
	qlSuccess(c, service.NewSystemConfigService().GetConfig())
}

// ---- /open/config(配置) ----

func qlConfigSample(c *gin.Context) {
	qlSuccess(c, []map[string]string{
		{"title": "config.sh", "value": "#!/bin/bash\nexport TZ=Asia/Shanghai\n"},
	})
}

func qlConfigList(c *gin.Context) {
	cfg := service.NewSystemConfigService().GetConfig()
	var out []map[string]interface{}
	for k, v := range cfg {
		out = append(out, map[string]interface{}{"title": k, "value": fmt.Sprint(v)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i]["title"].(string) < out[j]["title"].(string) })
	qlSuccess(c, out)
}

func qlConfigDetail(c *gin.Context) {
	path := c.Query("path")
	cfg := service.NewSystemConfigService().GetConfig()
	if v, ok := cfg[path]; ok {
		qlSuccess(c, gin.H{"content": fmt.Sprint(v)})
		return
	}
	qlFail(c, 404, 404, "配置不存在: "+path)
}

// ---- /open/script(脚本) ----

func qlScriptList(c *gin.Context) {
	qlSuccess(c, scriptFileList())
}

func scriptFileList() []map[string]string {
	root := config.C.Data.ScriptsDir
	var out []map[string]string
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		if rel == "" || strings.HasPrefix(filepath.Base(p), ".") {
			return nil
		}
		out = append(out, map[string]string{"title": rel, "value": rel})
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i]["title"] < out[j]["title"] })
	return out
}

func qlScriptDetail(c *gin.Context) {
	path := c.Query("path")
	file := c.Query("file")
	rel := path
	if file != "" {
		rel = filepath.Join(path, file)
	}
	if rel == "" {
		qlFail(c, 400, 400, "缺少 path/file 参数")
		return
	}
	full, err := pathutil.SafeJoin(config.C.Data.ScriptsDir, rel, true)
	if err != nil || full == "" {
		qlFail(c, 403, 403, "脚本访问受限")
		return
	}
	b, err := os.ReadFile(full)
	if err != nil {
		qlFail(c, 404, 404, "脚本不存在")
		return
	}
	qlSuccess(c, gin.H{"content": string(b)})
}

// ---- /open/dependence 与 /open/subscription(本项目无对应能力,返回空) ----

func qlDepList(c *gin.Context)  { qlSuccess(c, []interface{}{}) }
func qlSubList(c *gin.Context)  { qlSuccess(c, []interface{}{}) }

// ---- 路由注册 ----

// RegisterQinglongCompat 注册青龙兼容路由(/open 前缀,独立于 /api/v1)。
func RegisterQinglongCompat(engine *gin.Engine) {
	engine.GET("/open/auth/token", qlAuthToken)
	engine.POST("/open/auth/token", qlAuthToken)

	apps := engine.Group("/open/app", middleware.JWTAuth())
	{
		apps.GET("", qlAppList)
		apps.POST("", qlAppCreate)
		apps.PUT("", qlAppUpdate)
		apps.DELETE("", qlAppBatchDelete)
		apps.PUT("/:id/reset-secret", qlAppResetSecret)
	}

	open := engine.Group("/open", middleware.OpenAuth())
	{
		open.GET("/crontab", qlScope("crontab:read"), qlCronList)
		open.POST("/crontab", qlScope("crontab:write"), qlCronCreate)
		open.PUT("/crontab", qlScope("crontab:write"), qlCronUpdate)
		open.DELETE("/crontab", qlScope("crontab:write"), qlCronBatchDelete)
		open.PUT("/crontab/:id/run", qlScope("crontab:write"), qlCronRun)
		open.PUT("/crontab/:id/stop", qlScope("crontab:write"), qlCronStop)
		open.PUT("/crontab/:id/enable", qlScope("crontab:write"), qlCronEnable)
		open.PUT("/crontab/:id/disable", qlScope("crontab:write"), qlCronDisable)

		open.GET("/env", qlScope("env:read"), qlEnvList)
		open.POST("/env", qlScope("env:write"), qlEnvCreate)
		open.PUT("/env", qlScope("env:write"), qlEnvUpdate)
		open.DELETE("/env", qlScope("env:write"), qlEnvBatchDelete)
		open.PUT("/env/status", qlScope("env:write"), qlEnvStatus)

		open.GET("/log", qlScope("log:read"), qlLogList)
		open.GET("/log/detail", qlScope("log:read"), qlLogDetail)
		open.GET("/log/:file", qlScope("log:read"), qlLogFile)

		open.GET("/system", qlScope("system:read"), qlSystemInfo)
		open.GET("/system/config", qlScope("system:read"), qlSystemConfig)

		open.GET("/config/sample", qlScope("config:read"), qlConfigSample)
		open.GET("/config/file", qlScope("config:read"), qlConfigList)
		open.GET("/config/detail", qlScope("config:read"), qlConfigDetail)

		open.GET("/script", qlScope("script:read"), qlScriptList)
		open.GET("/script/detail", qlScope("script:read"), qlScriptDetail)

		open.GET("/dependence", qlScope("dependence:read"), qlDepList)
		open.GET("/subscription", qlScope("subscription:read"), qlSubList)
	}
}
