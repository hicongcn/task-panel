package handler

// 青龙(whyour/qinglong)OpenAPI 兼容层。
// 按 qinglong.online 文档实现 /open 前缀接口:认证 GET /open/auth/token、
// 应用管理 /open/app、资源接口 /open/crontab|env|log|system|config|script|dependence|subscription。
// 响应统一青龙格式 {code:200, data:...}。

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"taskpanel/config"
	"taskpanel/database"
	"taskpanel/middleware"
	"taskpanel/model"
	"taskpanel/pkg/pathutil"
	"taskpanel/service"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
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

// qlCronDict 输出青龙 TaskBean 完整字段。
// 注意:App 的 TaskBean.fromJson 对 name/command/schedule/created/timestamp/
// log_path/last_execution_time/pid 使用 .toString() 硬解析,缺任一字段即整条解析失败。
func qlCronDict(t model.Task) map[string]interface{} {
	status := 0
	if t.Status == model.TaskStatusRunning {
		status = 1
	}
	ts := strconv.FormatInt(unixT(t.CreatedAt), 10)
	pid := ""
	if t.PID != nil {
		pid = strconv.Itoa(*t.PID)
	}
	lr := unixMS(t.LastRunAt)
	return map[string]interface{}{
		"id":                  t.ID,
		"_id":                 strconv.FormatUint(uint64(t.ID), 10),
		"name":                t.Name,
		"command":             t.Command,
		"schedule":            t.CronExpression,
		"labels":              []string(t.Tags),
		"saved":               true,
		"status":              status,
		"isDisabled":          !t.Enabled,
		"isSystem":            0,
		"isPinned":            0,
		"timestamp":           ts,
		"created":             unixT(t.CreatedAt),
		"createdAt":           t.CreatedAt.Format("2006-01-02 15:04:05"),
		"updatedAt":           t.UpdatedAt.Format("2006-01-02 15:04:05"),
		"last_execution_time": unixMS(t.LastRunAt),
		"last_run_time":       lr,
		"last_running_time":   lr,
		"last_result":         qlRunStatus(t.LastRunStatus),
		"log_path":            "",
		"pid":                 pid,
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
		"createdAt": e.CreatedAt.Format("2006-01-02 15:04:05"),
		"updatedAt": e.UpdatedAt.Format("2006-01-02 15:04:05"),
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
		"status": status, "created": unixT(createdAt),
		"createdAt": createdAt.Format("2006-01-02 15:04:05"),
		"updatedAt": createdAt.Format("2006-01-02 15:04:05"),
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
	// version 上报青龙兼容版本号,App 据此选择接口能力(任务列表分页等)
	qlSuccess(c, gin.H{
		"isInitialized": !service.NewAuthService().NeedInit(),
		"version":       "2.15.0",
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
	qlSuccess(c, qlScriptTree())
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

// qlScriptTree 青龙脚本树(ScriptData):key/title/type/parent/children
func qlScriptTree() []map[string]interface{} {
	root := config.C.Data.ScriptsDir
	var out []map[string]interface{}
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		if rel == "" || strings.HasPrefix(filepath.Base(p), ".") {
			return nil
		}
		out = append(out, map[string]interface{}{
			"key": rel, "title": rel, "value": rel,
			"type": "file", "parent": filepath.Dir(rel),
			"children": []interface{}{},
		})
		return nil
	})
	sort.Slice(out, func(i, j int) bool {
		return out[i]["title"].(string) < out[j]["title"].(string)
	})
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

		// ---- 复数别名(青龙真实前端/生态客户端使用) ----
		open.GET("/crons", qlScope("crontab:read"), qlWebCronList)
		open.POST("/crons", qlScope("crontab:write"), qlWebCronCreate)
		open.PUT("/crons", qlScope("crontab:write"), qlWebCronUpdate)
		open.DELETE("/crons", qlScope("crontab:write"), qlWebCronBatchDelete)
		open.PUT("/crons/run", qlScope("crontab:write"), qlCronBatchAction("run"))
		open.PUT("/crons/stop", qlScope("crontab:write"), qlCronBatchAction("stop"))
		open.PUT("/crons/enable", qlScope("crontab:write"), qlCronBatchAction("enable"))
		open.PUT("/crons/disable", qlScope("crontab:write"), qlCronBatchAction("disable"))
		open.PUT("/crons/pin", qlScope("crontab:write"), qlCronBatchAction("pin"))
		open.PUT("/crons/unpin", qlScope("crontab:write"), qlCronBatchAction("unpin"))
		open.PUT("/crons/:id/run", qlScope("crontab:write"), qlCronRun)
		open.PUT("/crons/:id/stop", qlScope("crontab:write"), qlCronStop)
		open.PUT("/crons/:id/enable", qlScope("crontab:write"), qlCronEnable)
		open.PUT("/crons/:id/disable", qlScope("crontab:write"), qlCronDisable)
		open.GET("/crons/:id/log", qlScope("crontab:read"), qlWebCronLog)

		open.GET("/envs", qlScope("env:read"), qlWebEnvList)
		open.POST("/envs", qlScope("env:write"), qlWebEnvCreate)
		open.PUT("/envs", qlScope("env:write"), qlWebEnvUpdate)
		open.DELETE("/envs", qlScope("env:write"), qlWebEnvBatchDelete)
		open.PUT("/envs/enable", qlScope("env:write"), qlEnvBatchToggle(true))
		open.PUT("/envs/disable", qlScope("env:write"), qlEnvBatchToggle(false))
		open.PUT("/envs/:id/move", qlScope("env:write"), qlWebEnvMove)

		open.GET("/logs", qlScope("log:read"), qlWebLogList)
		open.DELETE("/logs", qlScope("log:write"), func(c *gin.Context) { qlSuccess(c, gin.H{"message": "已删除"}) })
		open.GET("/logs/:id", qlScope("log:read"), qlWebLogDetail)
		open.GET("/logs/:id/file", qlScope("log:read"), qlWebLogFile)

		open.GET("/scripts", qlScope("script:read"), qlScriptList)
		open.GET("/scripts/files", qlScope("script:read"), qlScriptList)
		open.GET("/scripts/detail", qlScope("script:read"), qlScriptDetail)
		open.PUT("/scripts", qlScope("script:write"), qlWebScriptSave)

		open.GET("/dependencies", qlScope("dependence:read"), qlWebDepList)
		open.POST("/dependencies", qlScope("dependence:write"), qlWebDepCreate)
		open.DELETE("/dependencies", qlScope("dependence:write"), qlWebDepBatchDelete)

		open.GET("/subscriptions", qlScope("subscription:read"), qlWebSubList)

		open.GET("/configs", qlScope("config:read"), qlConfigList)
		open.GET("/configs/files", qlScope("config:read"), qlConfigList)
		open.POST("/configs/save", qlScope("config:write"), qlWebConfigSave)

		open.GET("/apps", qlScope("crontab:read"), qlWebAppList)
		open.POST("/apps", qlScope("crontab:write"), qlWebAppCreate)
		open.PUT("/apps", qlScope("crontab:write"), qlWebAppUpdate)
		open.DELETE("/apps", qlScope("crontab:write"), qlWebAppBatchDelete)
		open.PUT("/apps/:id/reset-secret", qlScope("crontab:write"), qlWebAppResetSecret)

		open.PUT("/system/log/remove", qlScope("system:write"), qlWebLogRemove)
		open.GET("/system/update-check", qlScope("system:read"), qlWebUpdateCheck)
		open.GET("/user/login-log", qlScope("system:read"), qlWebLoginLog)
		open.GET("/user/notification", qlScope("system:read"), qlWebNotificationGet)
		open.PUT("/user/notification", qlScope("system:write"), qlWebNotificationSet)
	}
}
// ================= 青龙前端 API 兼容(/api/*,登录态 Bearer) =================

// qlWebLogin 青龙登录:POST /api/user/login {username,password}
// 已启用 2FA 的用户未携带动态码时返回 code 420,由客户端走两步验证流程。
func qlWebLogin(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		qlFail(c, 400, 400, "请求参数错误")
		return
	}
	res, err := service.NewAuthService().Login(req.Username, req.Password, c.ClientIP(), "")
	if err != nil {
		if errors.Is(err, service.ErrInvalidTOTP) {
			c.JSON(200, gin.H{"code": 420, "message": "需要二次验证"})
			return
		}
		qlFail(c, 401, 401, "用户名或密码错误")
		return
	}
	qlSuccess(c, gin.H{"token": res.Token})
}

// qlWebLoginOld 老版青龙登录:POST /api/login
func qlWebLoginOld(c *gin.Context) { qlWebLogin(c) }

// qlWebLoginTwo 青龙两步验证:PUT /api/user/two-factor/login {username,password,code}
func qlWebLoginTwo(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		qlFail(c, 400, 400, "请求参数错误")
		return
	}
	res, err := service.NewAuthService().Login(req.Username, req.Password, c.ClientIP(), req.Code)
	if err != nil {
		qlFail(c, 401, 401, "用户名/密码/验证码错误")
		return
	}
	qlSuccess(c, gin.H{"token": res.Token})
}

// qlWebUser 青龙用户信息:GET /api/user
func qlWebUser(c *gin.Context) {
	username := c.GetString("username")
	if username == "" {
		qlFail(c, 401, 401, "未登录")
		return
	}
	qlSuccess(c, gin.H{"id": 1, "username": username})
}

// qlWebUserUpdate 修改密码/资料:PUT /api/user {name,password}
func qlWebUserUpdate(c *gin.Context) {
	var req struct {
		Name     string `json:"name"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		qlFail(c, 400, 400, "请求参数错误")
		return
	}
	username := c.GetString("username")
	if req.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			qlFail(c, 500, 500, "密码加密失败")
			return
		}
		if err := database.DB.Model(&model.User{}).Where("username = ?", username).
			Update("password", string(hash)).Error; err != nil {
			qlFail(c, 500, 500, err.Error())
			return
		}
	}
	qlSuccess(c, gin.H{"message": "已更新"})
}

// ---- /api/crons(任务,青龙前端命名) ----

func qlWebCronList(c *gin.Context) {
	search := c.Query("searchValue")
	if search == "" {
		search = c.Query("searchText")
	}
	tasks := service.NewTaskService().List(search, "", "")
	out := make([]map[string]interface{}, len(tasks))
	for i, t := range tasks {
		out[i] = qlCronDict(t)
	}
	// 青龙 2.13.9+ 分页结构:data:{data:[...], total}
	if c.Query("page") != "" || c.Query("size") != "" {
		qlSuccess(c, gin.H{"data": out, "total": len(out)})
		return
	}
	qlSuccess(c, out)
}

func qlWebCronCreate(c *gin.Context) {
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
		Name: req.Name, Command: req.Command, CronExpression: req.Schedule, Enabled: false, Tags: req.Labels,
	})
	if err != nil {
		qlFail(c, 400, 400, err.Error())
		return
	}
	recordAudit(c, model.AuditActionTaskCreate, fmt.Sprintf("task:%d", task.ID), task.Name)
	qlSuccess(c, qlCronDict(*task))
}

func qlWebCronUpdate(c *gin.Context) {
	qlCronUpdate(c)
}

func qlWebCronBatchDelete(c *gin.Context) {
	qlCronBatchDelete(c)
}

func qlWebCronRun(c *gin.Context)     { qlCronRun(c) }
func qlWebCronStop(c *gin.Context)    { qlCronStop(c) }
func qlWebCronEnable(c *gin.Context)  { qlCronEnable(c) }
func qlWebCronDisable(c *gin.Context) { qlCronDisable(c) }

// ---- /api/envs(环境变量,青龙前端命名) ----

func qlWebEnvList(c *gin.Context)   { qlEnvList(c) }
func qlWebEnvCreate(c *gin.Context) { qlEnvCreate(c) }
func qlWebEnvUpdate(c *gin.Context) { qlEnvUpdate(c) }
func qlWebEnvBatchDelete(c *gin.Context) {
	var req struct {
		Ids []uint `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Ids) == 0 {
		qlFail(c, 400, 400, "缺少 ids 数组")
		return
	}
	n, err := service.NewEnvService().BatchDelete(req.Ids)
	if err != nil {
		qlFail(c, 500, 500, err.Error())
		return
	}
	recordAudit(c, model.AuditActionEnvBatchDel, fmt.Sprintf("envs:%v", req.Ids), "")
	qlSuccess(c, gin.H{"message": "已删除", "deleted": n})
}

// qlWebEnvToggle 青龙前端 PUT /api/envs/enable|disable {ids:[]}
func qlWebEnvToggle(enable bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Ids []uint `json:"ids"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || len(req.Ids) == 0 {
			qlFail(c, 400, 400, "缺少 ids 数组")
			return
		}
		n := service.NewEnvService().BatchSetEnabled(req.Ids, enable)
		action := model.AuditActionEnvDisable
		if enable {
			action = model.AuditActionEnvEnable
		}
		recordAudit(c, action, fmt.Sprintf("envs:%v", req.Ids), "")
		qlSuccess(c, gin.H{"message": "操作完成", "count": n})
	}
}

// ---- /api/logs(执行日志) ----

// qlWebLogList 青龙日志树:data 为数组,元素 {name,isDir,children}
func qlWebLogList(c *gin.Context) {
	files := logFileList()
	out := make([]map[string]interface{}, len(files))
	for i, f := range files {
		out[i] = map[string]interface{}{
			"name": f["title"], "title": f["title"], "value": f["value"],
			"isDir": false, "type": "file", "children": []interface{}{},
		}
	}
	qlSuccess(c, out)
}

// qlWebLogDetail 青龙日志详情:GET /api/logs/:name?path=(或数字 id)
// App 期望 data 为字符串(日志内容)。
func qlWebLogDetail(c *gin.Context) {
	param := c.Param("id")
	// 优先文件名
	if id, err := strconv.ParseUint(param, 10, 32); err == nil && id > 0 {
		_, content, gerr := service.NewLogService().Get(uint(id))
		if gerr == nil {
			qlSuccess(c, content)
			return
		}
	}
	full, err := pathutil.SafeJoin(config.C.Data.LogDir, filepath.Base(param), true)
	if err != nil || full == "" {
		qlFail(c, 404, 404, "日志不存在")
		return
	}
	b, rerr := os.ReadFile(full)
	if rerr != nil {
		qlFail(c, 404, 404, "日志不存在")
		return
	}
	qlSuccess(c, string(b))
}

func qlWebLogFile(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	path, _, err := service.NewLogService().RawFilePath(uint(id))
	if err != nil || path == "" {
		qlFail(c, 404, 404, "日志文件不存在")
		return
	}
	b, err := os.ReadFile(path)
	if err != nil {
		qlFail(c, 404, 404, "日志文件不存在")
		return
	}
	c.Data(200, "text/plain; charset=utf-8", b)
}
// ---- /api/system /api/configs /api/scripts /api/dependencies /api/apps /api/notifies ----

func qlWebSystem(c *gin.Context) {
	qlSystemInfo(c)
}

func qlWebSystemConfig(c *gin.Context) {
	qlSystemConfig(c)
}

func qlWebConfigList(c *gin.Context) {
	qlConfigList(c)
}

func qlWebConfigSample(c *gin.Context) {
	qlConfigSample(c)
}

func qlWebConfigSave(c *gin.Context) {
	var req struct {
		Name    string `json:"name"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
		qlFail(c, 400, 400, "name 必填")
		return
	}
	_ = req.Content
	qlSuccess(c, gin.H{"message": "已保存(本项目配置以面板设置页为准)"})
}

func qlWebScriptList(c *gin.Context) {
	qlScriptList(c)
}

func qlWebScriptDetail(c *gin.Context) {
	rel := c.Query("path")
	if rel == "" {
		rel = c.Query("file")
	}
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" {
		qlFail(c, 400, 400, "缺少 path 参数")
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

func qlWebScriptSave(c *gin.Context) {
	var req struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Path == "" {
		qlFail(c, 400, 400, "path 必填")
		return
	}
	if len(req.Content) > maxScriptSize {
		qlFail(c, 400, 400, "脚本内容过大")
		return
	}
	full, err := pathutil.SafeJoin(config.C.Data.ScriptsDir, req.Path, false)
	if err != nil || full == "" {
		qlFail(c, 403, 403, "脚本路径受限")
		return
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		qlFail(c, 500, 500, err.Error())
		return
	}
	if err := os.WriteFile(full, []byte(req.Content), 0o644); err != nil {
		qlFail(c, 500, 500, err.Error())
		return
	}
	recordAudit(c, model.AuditActionScriptSave, req.Path, "")
	qlSuccess(c, gin.H{"message": "保存成功"})
}

func qlWebScriptBatchDelete(c *gin.Context) {
	var req struct {
		Paths []string `json:"paths"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Paths) == 0 {
		qlFail(c, 400, 400, "缺少 paths 数组")
		return
	}
	deleted := 0
	for _, p := range req.Paths {
		full, err := pathutil.SafeJoin(config.C.Data.ScriptsDir, p, false)
		if err != nil || full == "" {
			continue
		}
		if err := os.RemoveAll(full); err == nil {
			deleted++
		}
	}
	recordAudit(c, model.AuditActionScriptDelete, fmt.Sprintf("scripts:%v", req.Paths), "")
	qlSuccess(c, gin.H{"message": "已删除", "deleted": deleted})
}

func qlWebDepList(c *gin.Context)   { qlSuccess(c, []interface{}{}) }
func qlWebDepCreate(c *gin.Context) { qlSuccess(c, []interface{}{}) }
func qlWebDepBatchDelete(c *gin.Context) {
	qlSuccess(c, gin.H{"message": "已删除"})
}

func qlWebAppList(c *gin.Context) {
	qlSuccess(c, qlAppDicts(service.NewOpenAPIService().List()))
}

func qlAppDicts(list []map[string]interface{}) []map[string]interface{} {
	out := make([]map[string]interface{}, len(list))
	for i, a := range list {
		out[i] = map[string]interface{}{
			"id":           a["id"],
			"name":         a["name"],
			"clientId":     a["client_id"],
			"clientSecret": a["client_secret"],
			"scopes":       a["scopes"],
			"status":       boolToInt(a["enabled"].(bool)),
			"createdAt":    a["created_at"],
		}
	}
	return out
}

func boolToInt(b bool) int {
	if b {
		return 0
	}
	return 1
}

func qlWebAppCreate(c *gin.Context)     { qlAppCreate(c) }
func qlWebAppUpdate(c *gin.Context)     { qlAppUpdate(c) }
func qlWebAppBatchDelete(c *gin.Context) {
	qlAppBatchDelete(c)
}
func qlWebAppResetSecret(c *gin.Context) { qlAppResetSecret(c) }

func qlWebNotifyList(c *gin.Context) {
	list := service.GetNotifyService().List()
	out := make([]map[string]interface{}, len(list))
	for i, ch := range list {
		id, _ := ch["id"].(uint)
		name, _ := ch["name"].(string)
		typ, _ := ch["type"].(string)
		enabled, _ := ch["enabled"].(bool)
		out[i] = map[string]interface{}{
			"id": id, "name": name, "type": typ, "remark": "", "status": boolToInt(enabled),
		}
	}
	qlSuccess(c, out)
}

func qlWebNotifyCreate(c *gin.Context) {
	qlSuccess(c, gin.H{"message": "请在本项目「系统管理-通知渠道」中配置"})
}

func qlWebSubList(c *gin.Context) { qlSuccess(c, []interface{}{}) }

// qlWebActivities 面板操作记录(映射审计日志)
func qlWebActivities(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	logs, total := service.NewAuditService().List("", "", page, size)
	out := make([]map[string]interface{}, len(logs))
	for i, l := range logs {
		out[i] = map[string]interface{}{
			"id":      l.ID,
			"type":    l.Action,
			"log":     l.Detail,
			"created": unixT(l.CreatedAt),
		}
	}
	qlSuccess(c, gin.H{"data": out, "total": total})
}


// ---- 青龙批量操作(字符串ID数组,App 使用) ----

func qlCronBatchAction(action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var ids []string
		if err := c.ShouldBindJSON(&ids); err != nil || len(ids) == 0 {
			qlFail(c, 400, 400, "请求体应为任务ID数组")
			return
		}
		var uids []uint
		for _, s := range ids {
			if id, err := strconv.ParseUint(s, 10, 32); err == nil {
				uids = append(uids, uint(id))
			}
		}
		if len(uids) == 0 {
			qlFail(c, 400, 400, "任务ID无效")
			return
		}
		svc := service.NewTaskService()
		switch action {
		case "run":
			res := svc.BatchRun(uids)
			recordAudit(c, model.AuditActionTaskRun, fmt.Sprintf("tasks:%v", uids), "")
			qlSuccess(c, gin.H{"message": "操作完成", "ok": res.OK})
		case "stop":
			for _, id := range uids {
				_ = svc.Stop(id)
			}
			recordAudit(c, model.AuditActionTaskStop, fmt.Sprintf("tasks:%v", uids), "")
			qlSuccess(c, gin.H{"message": "操作完成"})
		case "enable":
			res := svc.BatchSetEnabled(uids, true)
			recordAudit(c, model.AuditActionTaskEnable, fmt.Sprintf("tasks:%v", uids), "")
			qlSuccess(c, gin.H{"message": "操作完成", "ok": res.OK})
		case "disable":
			res := svc.BatchSetEnabled(uids, false)
			recordAudit(c, model.AuditActionTaskDisable, fmt.Sprintf("tasks:%v", uids), "")
			qlSuccess(c, gin.H{"message": "操作完成", "ok": res.OK})
		case "pin", "unpin":
			qlSuccess(c, gin.H{"message": "操作完成"})
		case "delete":
			res := svc.BatchDelete(uids)
			recordAudit(c, model.AuditActionTaskDelete, fmt.Sprintf("tasks:%v", uids), "")
			qlSuccess(c, gin.H{"message": "操作完成", "ok": res.OK})
		default:
			qlFail(c, 400, 400, "不支持的操作")
		}
	}
}

// qlEnvBatchToggle 青龙前端 PUT /api/envs/enable|disable {ids:字符串数组}
func qlEnvBatchToggle(enable bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var ids []string
		if err := c.ShouldBindJSON(&ids); err != nil || len(ids) == 0 {
			qlFail(c, 400, 400, "请求体应为环境变量ID数组")
			return
		}
		var uids []uint
		for _, s := range ids {
			if id, err := strconv.ParseUint(s, 10, 32); err == nil {
				uids = append(uids, uint(id))
			}
		}
		if len(uids) == 0 {
			qlFail(c, 400, 400, "环境变量ID无效")
			return
		}
		n := service.NewEnvService().BatchSetEnabled(uids, enable)
		action := model.AuditActionEnvDisable
		if enable {
			action = model.AuditActionEnvEnable
		}
		recordAudit(c, action, fmt.Sprintf("envs:%v", uids), "")
		qlSuccess(c, gin.H{"message": "操作完成", "count": n})
	}
}

// ---- 青龙辅助端点 ----

// qlWebLogRemove 日志清理:PUT /api/system/log/remove|/open/system/log/remove {frequency}
func qlWebLogRemove(c *gin.Context) {
	var req struct {
		Frequency int `json:"frequency"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.Frequency > 0 {
		_, _, _ = service.NewLogService().Clean(req.Frequency)
	}
	qlSuccess(c, gin.H{"message": "操作完成"})
}

// qlWebUpdateCheck 版本检查:GET /api/system/update-check
func qlWebUpdateCheck(c *gin.Context) {
	qlSuccess(c, gin.H{"hasNewVersion": false, "lastVersion": "2.15.0", "lastLog": "task-panel 青龙兼容版"})
}

// qlWebLoginLog 登录记录:GET /api/user/login-log
// LoginLogBean 需要 timestamp/address/ip/platform/status 字段。
func qlWebLoginLog(c *gin.Context) {
	logs, _ := service.NewAuditService().List("", "", 1, 50)
	out := make([]map[string]interface{}, 0, len(logs))
	for _, l := range logs {
		if l.Action != model.AuditActionLoginSuccess && l.Action != model.AuditActionLoginFailed {
			continue
		}
		status := 1
		if l.Action == model.AuditActionLoginSuccess {
			status = 0
		}
		out = append(out, map[string]interface{}{
			"id": l.ID, "timestamp": unixT(l.CreatedAt), "address": l.IP,
			"ip": l.IP, "platform": "", "status": status,
		})
	}
	qlSuccess(c, out)
}

// qlWebNotification 通知设置:GET/PUT /api/user/notification
func qlWebNotificationGet(c *gin.Context) {
	qlSuccess(c, gin.H{"type": "telegram", "remark": ""})
}

func qlWebNotificationSet(c *gin.Context) {
	qlSuccess(c, gin.H{"message": "已保存"})
}

// qlWebEnvMove 环境变量排序:PUT /api/envs/:id/move|/open/envs/:id/move
func qlWebEnvMove(c *gin.Context) {
	qlSuccess(c, gin.H{"message": "操作完成"})
}

// qlWebCronLog 任务最近日志:GET /api/crons/:id/log
func qlWebCronLog(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	log, err := service.NewLogService().LatestLog(uint(id))
	if err != nil {
		qlFail(c, 404, 404, "暂无日志")
		return
	}
	qlSuccess(c, gin.H{"id": log.ID, "log": log.Content})
}


// RegisterQinglongWebCompat 注册青龙前端 API(/api/* 登录态)。
func RegisterQinglongWebCompat(engine *gin.Engine) {
	engine.POST("/api/user/login", qlWebLogin)
	engine.POST("/api/login", qlWebLoginOld)
	engine.PUT("/api/user/two-factor/login", qlWebLoginTwo)

	api := engine.Group("/api", middleware.JWTAuth())
	{
		api.GET("/user", qlWebUser)
		api.PUT("/user", qlWebUserUpdate)

		api.GET("/crons", qlWebCronList)
		api.POST("/crons", qlWebCronCreate)
		api.PUT("/crons", qlWebCronUpdate)
		api.DELETE("/crons", qlWebCronBatchDelete)
		api.PUT("/crons/run", qlCronBatchAction("run"))
		api.PUT("/crons/stop", qlCronBatchAction("stop"))
		api.PUT("/crons/enable", qlCronBatchAction("enable"))
		api.PUT("/crons/disable", qlCronBatchAction("disable"))
		api.PUT("/crons/pin", qlCronBatchAction("pin"))
		api.PUT("/crons/unpin", qlCronBatchAction("unpin"))
		api.PUT("/crons/:id/run", qlWebCronRun)
		api.PUT("/crons/:id/stop", qlWebCronStop)
		api.PUT("/crons/:id/enable", qlWebCronEnable)
		api.PUT("/crons/:id/disable", qlWebCronDisable)
		api.GET("/crons/:id/log", qlWebCronLog)

		api.GET("/envs", qlWebEnvList)
		api.POST("/envs", qlWebEnvCreate)
		api.PUT("/envs", qlWebEnvUpdate)
		api.DELETE("/envs", qlWebEnvBatchDelete)
		api.PUT("/envs/enable", qlEnvBatchToggle(true))
		api.PUT("/envs/disable", qlEnvBatchToggle(false))
		api.PUT("/envs/:id/move", qlWebEnvMove)

		api.GET("/logs", qlWebLogList)
		api.DELETE("/logs", func(c *gin.Context) { qlSuccess(c, gin.H{"message": "已删除"}) })
		api.GET("/logs/:id", qlWebLogDetail)
		api.GET("/logs/:id/file", qlWebLogFile)

		api.GET("/system", qlWebSystem)
		api.GET("/system/config", qlWebSystemConfig)
		api.PUT("/system/log/remove", qlWebLogRemove)
		api.GET("/system/update-check", qlWebUpdateCheck)

		api.GET("/configs", qlWebConfigList)
		api.GET("/configs/files", qlWebConfigList)
		api.GET("/configs/sample", qlWebConfigSample)
		api.POST("/configs/save", qlWebConfigSave)

		api.GET("/scripts", qlWebScriptList)
		api.GET("/scripts/files", qlWebScriptList)
		api.GET("/scripts/detail", qlWebScriptDetail)
		api.PUT("/scripts", qlWebScriptSave)
		api.DELETE("/scripts", qlWebScriptBatchDelete)

		api.GET("/dependencies", qlWebDepList)
		api.POST("/dependencies", qlWebDepCreate)
		api.DELETE("/dependencies", qlWebDepBatchDelete)

		api.GET("/apps", qlWebAppList)
		api.POST("/apps", qlWebAppCreate)
		api.PUT("/apps", qlWebAppUpdate)
		api.DELETE("/apps", qlWebAppBatchDelete)
		api.PUT("/apps/:id/reset-secret", qlWebAppResetSecret)

		api.GET("/notifies", qlWebNotifyList)
		api.POST("/notifies", qlWebNotifyCreate)

		api.GET("/subscriptions", qlWebSubList)
		api.GET("/user/login-log", qlWebLoginLog)
		api.GET("/user/notification", qlWebNotificationGet)
		api.PUT("/user/notification", qlWebNotificationSet)
		api.GET("/activities", qlWebActivities)
	}
}


