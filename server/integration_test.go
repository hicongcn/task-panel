package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"taskpanel/config"
	"taskpanel/pkg/totp"
	"taskpanel/database"
	"taskpanel/router"
	"taskpanel/service"

	"github.com/gin-gonic/gin"
)

var testEngine *gin.Engine

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)

	dir, err := os.MkdirTemp("", "taskpanel-it-*")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

	config.C = &config.Config{
		Server:   config.ServerConfig{Port: 5700, Mode: "debug"},
		Database: config.DatabaseConfig{Path: filepath.Join(dir, "test.db")},
		JWT:      config.JWTConfig{Secret: "integration-test-secret", TokenExpireH: 72},
		Data: config.DataConfig{
			Dir:        dir,
			ScriptsDir: filepath.Join(dir, "scripts"),
			LogDir:     filepath.Join(dir, "logs"),
		},
		Backup: config.BackupConfig{Dir: filepath.Join(dir, "backups")},
		CORS:   config.CORSConfig{Origins: []string{"http://localhost:5173"}},
	}
	for _, d := range []string{config.C.Data.Dir, config.C.Data.ScriptsDir, config.C.Data.LogDir, config.C.Backup.Dir} {
		_ = os.MkdirAll(d, 0o755)
	}

	if err := database.Init(config.C.Database.Path); err != nil {
		log.Fatal(err)
	}

	testEngine = gin.New()
	testEngine.Use(gin.Logger(), gin.Recovery())
	router.Setup(testEngine)

	// 与 main.go 一致:启动监控采样(集成测试断言 stats 用)
	service.GetSysMonitor().Start()

	code := m.Run()
	os.Exit(code)
}

type apiResp struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func doRequest(t *testing.T, method, path, token string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	testEngine.ServeHTTP(w, req)
	return w
}

func decodeResp(t *testing.T, w *httptest.ResponseRecorder) apiResp {
	t.Helper()
	var r apiResp
	if err := json.Unmarshal(w.Body.Bytes(), &r); err != nil {
		t.Fatalf("decode response %q: %v", w.Body.String(), err)
	}
	return r
}

// innerData 取业务数据:统一响应为 {code,message,data},而多数 handler 又在 data
// 内包了一层 {"data": 业务对象, ...}。这里统一取到最内层业务对象(无嵌套则原样返回)。
func innerData(t *testing.T, w *httptest.ResponseRecorder) json.RawMessage {
	t.Helper()
	r := decodeResp(t, w)
	var inner struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(r.Data, &inner); err == nil && len(inner.Data) > 0 {
		return inner.Data
	}
	return r.Data
}

func assertCode(t *testing.T, w *httptest.ResponseRecorder, wantHTTP int, wantCode int, msg string) {
	t.Helper()
	if w.Code != wantHTTP {
		t.Fatalf("%s: http status = %d, want %d (body=%s)", msg, w.Code, wantHTTP, w.Body.String())
	}
	r := decodeResp(t, w)
	if r.Code != wantCode {
		t.Fatalf("%s: code = %d, want %d (msg=%s)", msg, r.Code, wantCode, r.Message)
	}
}

func waitFor(t *testing.T, timeout time.Duration, cond func() bool, desc string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for: %s", desc)
}

// ---------- 认证流程 ----------

func TestAuthFlow(t *testing.T) {
	// 1. 未初始化
	w := doRequest(t, "GET", "/api/v1/auth/check-init", "", nil)
	assertCode(t, w, 200, 0, "check-init")
	var ci struct {
		NeedInit bool `json:"need_init"`
	}
	if err := json.Unmarshal(decodeResp(t, w).Data, &ci); err != nil || !ci.NeedInit {
		t.Fatalf("expected need_init=true, got %+v err=%v", ci, err)
	}

	// 2. 初始化管理员
	w = doRequest(t, "POST", "/api/v1/auth/init", "", map[string]string{
		"username": "admin", "password": "Admin@12345",
	})
	assertCode(t, w, 201, 0, "init")

	// 3. 重复初始化被拒绝
	w = doRequest(t, "POST", "/api/v1/auth/init", "", map[string]string{
		"username": "admin2", "password": "Admin@12345",
	})
	assertCode(t, w, 400, 400, "re-init rejected")

	// 4. 登录
	w = doRequest(t, "POST", "/api/v1/auth/login", "", map[string]string{
		"username": "admin", "password": "Admin@12345",
	})
	assertCode(t, w, 200, 0, "login")
	var lr struct {
		AccessToken string `json:"access_token"`
		Username    string `json:"username"`
	}
	if err := json.Unmarshal(decodeResp(t, w).Data, &lr); err != nil || lr.AccessToken == "" || lr.Username != "admin" {
		t.Fatalf("login result invalid: %+v err=%v", lr, err)
	}
	token := lr.AccessToken

	// 5. 错误密码
	w = doRequest(t, "POST", "/api/v1/auth/login", "", map[string]string{
		"username": "admin", "password": "wrong-password",
	})
	assertCode(t, w, 401, 401, "wrong password")

	// 6. 携带 token 访问受保护接口
	w = doRequest(t, "GET", "/api/v1/auth/user", token, nil)
	assertCode(t, w, 200, 0, "auth/user")
	var u struct {
		Username string `json:"username"`
	}
	if err := json.Unmarshal(decodeResp(t, w).Data, &u); err != nil || u.Username != "admin" {
		t.Fatalf("user = %+v err=%v", u, err)
	}

	// 7. 无 token 访问受保护接口 → 401
	w = doRequest(t, "GET", "/api/v1/tasks", "", nil)
	assertCode(t, w, 401, 401, "no token")

	// 8. 登出后 token 被吊销
	w = doRequest(t, "POST", "/api/v1/auth/logout", token, nil)
	assertCode(t, w, 200, 0, "logout")
	w = doRequest(t, "GET", "/api/v1/auth/user", token, nil)
	assertCode(t, w, 401, 401, "token revoked after logout")
}

func TestLoginLockoutService(t *testing.T) {
	svc := service.NewAuthService()
	// 用唯一 IP 避免与其它测试相互影响
	ip := fmt.Sprintf("10.%d.%d.%d", time.Now().UnixNano()%200+1, time.Now().UnixNano()%200+1, time.Now().UnixNano()%200+1)

	// 连续 5 次失败
	for i := 0; i < 5; i++ {
		if _, err := svc.Login("admin", "wrong-password", ip, ""); err == nil {
			t.Fatalf("attempt %d should fail", i+1)
		}
	}
	// 第 6 次应返回锁定
	if _, err := svc.Login("admin", "wrong-password", ip, ""); err != service.ErrAccountLocked {
		t.Fatalf("expected ErrAccountLocked, got %v", err)
	}
}

// ---------- 任务 CRUD 与调度 ----------

func TestTaskCRUDAndRun(t *testing.T) {
	token := getToken(t)

	// 先写一个脚本
	w := doRequest(t, "PUT", "/api/v1/scripts/content", token, map[string]string{
		"path":    "hello.sh",
		"content": "#!/bin/bash\necho hello-from-task\n",
	})
	assertCode(t, w, 200, 0, "create script")

	// 创建任务
	w = doRequest(t, "POST", "/api/v1/tasks", token, map[string]interface{}{
		"name": "测试任务", "command": "hello.sh", "cron_expression": "0 3 * * *",
		"enabled": false, "timeout_seconds": 30,
	})
	assertCode(t, w, 201, 0, "create task")
	var created struct {
		ID   uint   `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(innerData(t, w), &created); err != nil || created.ID == 0 {
		t.Fatalf("create task invalid: %+v err=%v body=%s", created, err, w.Body.String())
	}
	taskID := created.ID

	// 非法 cron 被拒绝
	w = doRequest(t, "POST", "/api/v1/tasks", token, map[string]interface{}{
		"name": "bad", "command": "hello.sh", "cron_expression": "not-a-cron",
	})
	assertCode(t, w, 400, 400, "invalid cron rejected")

	// 非法命令(不存在的脚本)被拒绝
	w = doRequest(t, "POST", "/api/v1/tasks", token, map[string]interface{}{
		"name": "bad", "command": "missing.sh", "cron_expression": "0 3 * * *",
	})
	assertCode(t, w, 400, 400, "missing script rejected")

	// 命令穿越防护
	w = doRequest(t, "POST", "/api/v1/tasks", token, map[string]interface{}{
		"name": "bad", "command": "../outside.sh", "cron_expression": "0 3 * * *",
	})
	assertCode(t, w, 400, 400, "command traversal rejected")

	// 列表
	w = doRequest(t, "GET", "/api/v1/tasks?keyword=测试", token, nil)
	assertCode(t, w, 200, 0, "list tasks")
	var list []map[string]interface{}
	if err := json.Unmarshal(innerData(t, w), &list); err != nil || len(list) != 1 {
		t.Fatalf("expected 1 task, got %d err=%v", len(list), err)
	}

	// 更新
	w = doRequest(t, "PUT", fmt.Sprintf("/api/v1/tasks/%d", taskID), token, map[string]interface{}{
		"name": "测试任务-改",
	})
	assertCode(t, w, 200, 0, "update task")

	// cron 描述
	w = doRequest(t, "POST", "/api/v1/tasks/cron-describe", token, map[string]string{
		"expression": "0 3 * * *",
	})
	assertCode(t, w, 200, 0, "cron describe")

	// 手动运行
	w = doRequest(t, "PUT", fmt.Sprintf("/api/v1/tasks/%d/run", taskID), token, nil)
	assertCode(t, w, 200, 0, "run task")

	// 等待执行完成(以 last_run_status 从初始值变化为标志,避免把"初始 idle"误判为完成)
	waitFor(t, 10*time.Second, func() bool {
		w := doRequest(t, "GET", fmt.Sprintf("/api/v1/tasks/%d", taskID), token, nil)
		var tr struct {
			Status        string `json:"status"`
			LastRunStatus string `json:"last_run_status"`
		}
		_ = json.Unmarshal(innerData(t, w), &tr)
		return tr.Status == "idle" && tr.LastRunStatus != "none"
	}, "task finished")

	// 校验任务状态
	w = doRequest(t, "GET", fmt.Sprintf("/api/v1/tasks/%d", taskID), token, nil)
	var tr struct {
		LastRunStatus string `json:"last_run_status"`
	}
	_ = json.Unmarshal(innerData(t, w), &tr)
	if tr.LastRunStatus != "success" {
		t.Fatalf("expected last_run_status=success, got %q", tr.LastRunStatus)
	}

	// 日志列表
	w = doRequest(t, "GET", fmt.Sprintf("/api/v1/logs?task_id=%d", taskID), token, nil)
	assertCode(t, w, 200, 0, "list logs")
	var logs []struct {
		ID     uint   `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(innerData(t, w), &logs); err != nil || len(logs) == 0 {
		t.Fatalf("no logs: %+v err=%v", logs, err)
	}
	if logs[0].Status != "success" {
		t.Fatalf("log status = %q, want success", logs[0].Status)
	}

	// 日志详情含输出
	w = doRequest(t, "GET", fmt.Sprintf("/api/v1/logs/%d", logs[0].ID), token, nil)
	assertCode(t, w, 200, 0, "log detail")
	var detail struct {
		Content string `json:"content"`
	}
	_ = json.Unmarshal(decodeResp(t, w).Data, &detail)
	if !strings.Contains(detail.Content, "hello-from-task") {
		t.Fatalf("log content missing script output: %q", detail.Content)
	}

	// 下载票据 → 原文下载
	w = doRequest(t, "GET", fmt.Sprintf("/api/v1/logs/%d/raw-ticket", logs[0].ID), token, nil)
	assertCode(t, w, 200, 0, "raw ticket")
	var ticket struct {
		URL string `json:"url"`
	}
	_ = json.Unmarshal(decodeResp(t, w).Data, &ticket)
	if ticket.URL == "" {
		t.Fatal("empty download url")
	}
	w = doRequest(t, "GET", ticket.URL, "", nil)
	if w.Code != 200 {
		t.Fatalf("raw download status = %d, body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "hello-from-task") {
		t.Fatalf("raw download content wrong: %q", w.Body.String())
	}

	// 删除任务
	w = doRequest(t, "DELETE", fmt.Sprintf("/api/v1/tasks/%d", taskID), token, nil)
	assertCode(t, w, 200, 0, "delete task")

	// 清理脚本
	_ = doRequest(t, "DELETE", "/api/v1/scripts?path=hello.sh", token, nil)
}

func TestTaskStop(t *testing.T) {
	token := getToken(t)

	_ = doRequest(t, "PUT", "/api/v1/scripts/content", token, map[string]string{
		"path":    "sleepy.sh",
		"content": "#!/bin/bash\nsleep 30\n",
	})
	w := doRequest(t, "POST", "/api/v1/tasks", token, map[string]interface{}{
		"name": "长任务", "command": "sleepy.sh", "cron_expression": "0 3 * * *",
		"enabled": false, "timeout_seconds": 120,
	})
	assertCode(t, w, 201, 0, "create long task")
	var created struct {
		ID uint `json:"id"`
	}
	_ = json.Unmarshal(innerData(t, w), &created)
	taskID := created.ID
	if taskID == 0 {
		t.Fatalf("task id = 0, body=%s", w.Body.String())
	}

	// 启动
	w = doRequest(t, "PUT", fmt.Sprintf("/api/v1/tasks/%d/run", taskID), token, nil)
	assertCode(t, w, 200, 0, "run long task")

	// 等待进入 running
	waitFor(t, 5*time.Second, func() bool {
		w := doRequest(t, "GET", fmt.Sprintf("/api/v1/tasks/%d", taskID), token, nil)
		var tr struct {
			Status string `json:"status"`
		}
		_ = json.Unmarshal(innerData(t, w), &tr)
		return tr.Status == "running"
	}, "task running")

	// 停止
	w = doRequest(t, "PUT", fmt.Sprintf("/api/v1/tasks/%d/stop", taskID), token, nil)
	assertCode(t, w, 200, 0, "stop task")

	// 等待结束并校验 aborted
	waitFor(t, 10*time.Second, func() bool {
		w := doRequest(t, "GET", fmt.Sprintf("/api/v1/tasks/%d", taskID), token, nil)
		var tr struct {
			Status string `json:"status"`
		}
		_ = json.Unmarshal(innerData(t, w), &tr)
		return tr.Status == "idle"
	}, "task stopped")

	w = doRequest(t, "GET", fmt.Sprintf("/api/v1/tasks/%d", taskID), token, nil)
	var tr struct {
		LastRunStatus string `json:"last_run_status"`
	}
	_ = json.Unmarshal(innerData(t, w), &tr)
	if tr.LastRunStatus != "aborted" {
		t.Fatalf("expected aborted, got %q", tr.LastRunStatus)
	}

	_ = doRequest(t, "DELETE", fmt.Sprintf("/api/v1/tasks/%d", taskID), token, nil)
	_ = doRequest(t, "DELETE", "/api/v1/scripts?path=sleepy.sh", token, nil)
}

// ---------- 环境变量 ----------

func TestEnvCRUD(t *testing.T) {
	token := getToken(t)

	w := doRequest(t, "POST", "/api/v1/envs", token, map[string]interface{}{
		"name": "MY_SECRET", "value": "s3cr3t-value", "group": "prod", "remark": "备注", "enabled": true,
	})
	assertCode(t, w, 201, 0, "create env")
	var created struct {
		ID   uint   `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(innerData(t, w), &created); err != nil || created.ID == 0 {
		t.Fatalf("env create invalid: %+v err=%v body=%s", created, err, w.Body.String())
	}

	// 非法变量名
	w = doRequest(t, "POST", "/api/v1/envs", token, map[string]interface{}{
		"name": "1BAD_NAME", "value": "x",
	})
	assertCode(t, w, 400, 400, "invalid env name rejected")

	// 列表(值脱敏,不返回原值)
	w = doRequest(t, "GET", "/api/v1/envs", token, nil)
	assertCode(t, w, 200, 0, "list envs")
	body := w.Body.String()
	if strings.Contains(body, "s3cr3t-value") {
		t.Fatal("env list leaked raw value")
	}
	if !strings.Contains(body, "value_masked") {
		t.Fatal("env list missing value_masked")
	}

	// 更新
	w = doRequest(t, "PUT", fmt.Sprintf("/api/v1/envs/%d", created.ID), token, map[string]interface{}{
		"name": "MY_SECRET", "value": "new-value", "group": "prod", "remark": "更新", "enabled": false,
	})
	assertCode(t, w, 200, 0, "update env")

	// 删除
	w = doRequest(t, "DELETE", fmt.Sprintf("/api/v1/envs/%d", created.ID), token, nil)
	assertCode(t, w, 200, 0, "delete env")
}

// ---------- 脚本管理 ----------

func TestScriptManagementAndTraversal(t *testing.T) {
	token := getToken(t)

	// 保存文件
	w := doRequest(t, "PUT", "/api/v1/scripts/content", token, map[string]string{
		"path": "dir/hello.py", "content": "print('hi')\n",
	})
	assertCode(t, w, 200, 0, "save script")

	// 读取内容
	w = doRequest(t, "GET", "/api/v1/scripts/content?path=dir/hello.py", token, nil)
	assertCode(t, w, 200, 0, "read script")
	var content struct {
		Content string `json:"content"`
	}
	_ = json.Unmarshal(innerData(t, w), &content)
	if content.Content != "print('hi')\n" {
		t.Fatalf("content = %q", content.Content)
	}

	// 文件树
	w = doRequest(t, "GET", "/api/v1/scripts/tree", token, nil)
	assertCode(t, w, 200, 0, "script tree")

	// 重命名
	w = doRequest(t, "PUT", "/api/v1/scripts/rename", token, map[string]string{
		"old_path": "dir/hello.py", "new_name": "world.py",
	})
	assertCode(t, w, 200, 0, "rename script")

	// 重命名后原路径不存在,新路径可读
	w = doRequest(t, "GET", "/api/v1/scripts/content?path=dir/world.py", token, nil)
	assertCode(t, w, 200, 0, "renamed file readable")

	// 路径穿越防护
	w = doRequest(t, "GET", "/api/v1/scripts/content?path=../../etc/passwd", token, nil)
	assertCode(t, w, 400, 400, "traversal read rejected")
	w = doRequest(t, "PUT", "/api/v1/scripts/content", token, map[string]string{
		"path": "../escape.py", "content": "x",
	})
	assertCode(t, w, 400, 400, "traversal save rejected")
	w = doRequest(t, "GET", "/api/v1/scripts/content?path=/etc/passwd", token, nil)
	assertCode(t, w, 400, 400, "absolute path rejected")

	// 删除
	w = doRequest(t, "DELETE", "/api/v1/scripts?path=dir/world.py", token, nil)
	assertCode(t, w, 200, 0, "delete script")
}

// ---------- 实时日志(SSE)票据 ----------

func TestLiveTicket(t *testing.T) {
	token := getToken(t)

	// 未鉴权获取票据 → 401
	w := doRequest(t, "GET", "/api/v1/tasks/1/live-ticket", "", nil)
	assertCode(t, w, 401, 401, "live-ticket no auth")

	// 鉴权获取票据 → 200 且返回票据
	w = doRequest(t, "GET", "/api/v1/tasks/1/live-ticket", token, nil)
	assertCode(t, w, 200, 0, "live-ticket")
	var tic struct {
		Ticket string `json:"ticket"`
	}
	if err := json.Unmarshal(decodeResp(t, w).Data, &tic); err != nil || tic.Ticket == "" {
		t.Fatalf("live-ticket invalid: %+v err=%v body=%s", tic, err, w.Body.String())
	}

	// 无票据访问 SSE → unauthorized(SSE 文本,HTTP 200)
	w = doRequest(t, "GET", "/api/v1/tasks/1/live-logs", "", nil)
	if !strings.Contains(w.Body.String(), "unauthorized") {
		t.Fatalf("expected unauthorized, got %q", w.Body.String())
	}

	// 坏票据 → unauthorized
	w = doRequest(t, "GET", "/api/v1/tasks/1/live-logs?ticket=bad.ticket", "", nil)
	if !strings.Contains(w.Body.String(), "unauthorized") {
		t.Fatalf("expected unauthorized for bad ticket, got %q", w.Body.String())
	}
}

// ---------- 审计日志 ----------

func TestAuditLogs(t *testing.T) {
	token := getToken(t) // 触发 login_success 审计

	// 保存脚本 + 创建任务,触发 script_save / task_create 审计
	w := doRequest(t, "PUT", "/api/v1/scripts/content", token, map[string]string{
		"path": "audit_test.sh", "content": "#!/bin/bash\necho hi\n",
	})
	assertCode(t, w, 200, 0, "save script")
	w = doRequest(t, "POST", "/api/v1/tasks", token, map[string]interface{}{
		"name": "审计测试", "command": "audit_test.sh", "cron_expression": "0 3 * * *", "enabled": false,
	})
	assertCode(t, w, 201, 0, "create task")

	// 查询审计日志
	w = doRequest(t, "GET", "/api/v1/audit-logs?page=1&page_size=50", token, nil)
	assertCode(t, w, 200, 0, "list audit logs")
	var list struct {
		Data  []map[string]interface{} `json:"data"`
		Total int64                    `json:"total"`
	}
	if err := json.Unmarshal(decodeResp(t, w).Data, &list); err != nil {
		t.Fatalf("decode audit list: %v", err)
	}
	if list.Total == 0 {
		t.Fatal("expected audit logs")
	}

	// 校验包含 task_create,并取出任务 id 用于清理
	var taskID uint
	foundCreate := false
	for _, l := range list.Data {
		if l["action"] == "task_create" {
			foundCreate = true
			if r, ok := l["resource"].(string); ok {
				fmt.Sscanf(r, "task:%d", &taskID)
			}
		}
	}
	if !foundCreate {
		t.Fatalf("expected task_create audit log, got %+v", list.Data)
	}

	// 按动作过滤
	w = doRequest(t, "GET", "/api/v1/audit-logs?action=script_save", token, nil)
	assertCode(t, w, 200, 0, "filter by action")
	var filtered struct {
		Data []map[string]interface{} `json:"data"`
	}
	_ = json.Unmarshal(decodeResp(t, w).Data, &filtered)
	if len(filtered.Data) == 0 {
		t.Fatal("expected script_save audit log")
	}

	// 清理
	if taskID > 0 {
		_ = doRequest(t, "DELETE", fmt.Sprintf("/api/v1/tasks/%d", taskID), token, nil)
	}
	_ = doRequest(t, "DELETE", "/api/v1/scripts?path=audit_test.sh", token, nil)
}

// ---------- 通知渠道 ----------

func TestNotifyChannels(t *testing.T) {
	token := getToken(t)

	// 创建 webhook 渠道
	w := doRequest(t, "POST", "/api/v1/notify-channels", token, map[string]interface{}{
		"name": "测试 webhook", "type": "webhook", "enabled": false,
		"config": map[string]interface{}{"url": "https://example.com/hook"},
	})
	assertCode(t, w, 201, 0, "create notify channel")
	var created struct {
		Data map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(decodeResp(t, w).Data, &created); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	id := uint(created.Data["id"].(float64))

	// 列表
	w = doRequest(t, "GET", "/api/v1/notify-channels", token, nil)
	assertCode(t, w, 200, 0, "list notify channels")

	// toggle
	w = doRequest(t, "PUT", fmt.Sprintf("/api/v1/notify-channels/%d/toggle", id), token, map[string]interface{}{"enabled": true})
	assertCode(t, w, 200, 0, "toggle notify channel")

	// 非法类型应被拒绝
	w = doRequest(t, "POST", "/api/v1/notify-channels", token, map[string]interface{}{
		"name": "bad", "type": "foo", "config": map[string]interface{}{},
	})
	assertCode(t, w, 400, 400, "invalid type rejected")

	// 删除
	w = doRequest(t, "DELETE", fmt.Sprintf("/api/v1/notify-channels/%d", id), token, nil)
	assertCode(t, w, 200, 0, "delete notify channel")
}

// ---------- 任务结果通知(端到端) ----------

// TestTaskFailedNotify 命令解析失败(脚本不存在)也应发送失败通知。
func TestTaskFailedNotify(t *testing.T) {
	token := getToken(t)

	var mu sync.Mutex
	var received []map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var m map[string]string
		_ = json.NewDecoder(r.Body).Decode(&m)
		mu.Lock()
		received = append(received, m)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	w := doRequest(t, "POST", "/api/v1/notify-channels", token, map[string]interface{}{
		"name": "失败通知", "type": "webhook", "enabled": true,
		"config": map[string]interface{}{"url": srv.URL},
	})
	assertCode(t, w, 201, 0, "create webhook")
	var ch struct {
		Data map[string]interface{} `json:"data"`
	}
	_ = json.Unmarshal(decodeResp(t, w).Data, &ch)
	channelID := uint(ch.Data["id"].(float64))

	// 先建脚本再建任务,随后删除脚本 → 运行触发"命令解析失败"路径
	w = doRequest(t, "PUT", "/api/v1/scripts/content", token, map[string]string{
		"path": "temp_fail.sh", "content": "#!/bin/bash\necho hi\n",
	})
	assertCode(t, w, 200, 0, "save script")
	w = doRequest(t, "POST", "/api/v1/tasks", token, map[string]interface{}{
		"name": "失败任务", "command": "temp_fail.sh", "cron_expression": "0 3 * * *", "enabled": false,
	})
	assertCode(t, w, 201, 0, "create task")
	var created struct {
		Data map[string]interface{} `json:"data"`
	}
	_ = json.Unmarshal(decodeResp(t, w).Data, &created)
	taskID := uint(created.Data["id"].(float64))

	// 删除脚本后运行 → 解析失败路径
	_ = doRequest(t, "DELETE", "/api/v1/scripts?path=temp_fail.sh", token, nil)
	w = doRequest(t, "PUT", fmt.Sprintf("/api/v1/tasks/%d/run", taskID), token, nil)
	assertCode(t, w, 200, 0, "run task")

	waitFor(t, 15*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(received) > 0
	}, "failed notification")

	mu.Lock()
	got := received
	mu.Unlock()
	if len(got) == 0 {
		t.Fatal("未收到失败通知")
	}
	if got[0]["title"] != "任务执行失败" {
		t.Fatalf("通知标题应为失败: %+v", got[0])
	}

	_ = doRequest(t, "DELETE", fmt.Sprintf("/api/v1/tasks/%d", taskID), token, nil)
	_ = doRequest(t, "DELETE", fmt.Sprintf("/api/v1/notify-channels/%d", channelID), token, nil)
}

func TestTaskResultNotify(t *testing.T) {
	token := getToken(t)

	// 本地 webhook 接收端
	var mu sync.Mutex
	var received []map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var m map[string]string
		_ = json.NewDecoder(r.Body).Decode(&m)
		mu.Lock()
		received = append(received, m)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// 创建并启用 webhook 渠道
	w := doRequest(t, "POST", "/api/v1/notify-channels", token, map[string]interface{}{
		"name": "e2e webhook", "type": "webhook", "enabled": true,
		"config": map[string]interface{}{"url": srv.URL},
	})
	assertCode(t, w, 201, 0, "create webhook")
	var ch struct {
		Data map[string]interface{} `json:"data"`
	}
	_ = json.Unmarshal(decodeResp(t, w).Data, &ch)
	channelID := uint(ch.Data["id"].(float64))

	// 保存脚本 + 创建任务
	w = doRequest(t, "PUT", "/api/v1/scripts/content", token, map[string]string{
		"path": "notify_test.sh", "content": "#!/bin/bash\necho ok\n",
	})
	assertCode(t, w, 200, 0, "save script")
	w = doRequest(t, "POST", "/api/v1/tasks", token, map[string]interface{}{
		"name": "通知测试", "command": "notify_test.sh", "cron_expression": "0 3 * * *", "enabled": false,
	})
	assertCode(t, w, 201, 0, "create task")
	var created struct {
		Data map[string]interface{} `json:"data"`
	}
	_ = json.Unmarshal(decodeResp(t, w).Data, &created)
	taskID := uint(created.Data["id"].(float64))

	// 运行任务
	w = doRequest(t, "PUT", fmt.Sprintf("/api/v1/tasks/%d/run", taskID), token, nil)
	assertCode(t, w, 200, 0, "run task")

	// 等待通知到达
	waitFor(t, 15*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(received) > 0
	}, "task result notification")

	mu.Lock()
	got := received
	mu.Unlock()
	if len(got) == 0 {
		t.Fatal("未收到任务结果通知")
	}
	if got[0]["title"] != "任务执行成功" {
		t.Fatalf("通知标题不符: %+v", got[0])
	}

	// 清理
	_ = doRequest(t, "DELETE", fmt.Sprintf("/api/v1/tasks/%d", taskID), token, nil)
	_ = doRequest(t, "DELETE", "/api/v1/scripts?path=notify_test.sh", token, nil)
	_ = doRequest(t, "DELETE", fmt.Sprintf("/api/v1/notify-channels/%d", channelID), token, nil)
}

// doUpload 构造 multipart/form-data 上传请求。
func doUpload(t *testing.T, path, token, field, filename string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile(field, filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	testEngine.ServeHTTP(w, req)
	return w
}

// ---------- 备份与恢复 ----------

func TestBackupRestore(t *testing.T) {
	token := getToken(t)

	// 1. 备份前状态:创建脚本 before_backup.sh
	w := doRequest(t, "PUT", "/api/v1/scripts/content", token, map[string]string{
		"path": "before_backup.sh", "content": "#!/bin/bash\necho before\n",
	})
	assertCode(t, w, 200, 0, "save script before backup")

	// 2. 创建备份
	w = doRequest(t, "POST", "/api/v1/backups", token, nil)
	assertCode(t, w, 201, 0, "create backup")
	var created struct {
		Data map[string]interface{} `json:"data"`
	}
	_ = json.Unmarshal(decodeResp(t, w).Data, &created)
	backupName := created.Data["name"].(string)
	if backupName == "" {
		t.Fatal("备份名称为空")
	}

	// 3. 读备份文件内容(用于稍后上传恢复)
	backupPath := filepath.Join(config.C.Backup.Dir, backupName)
	backupBytes, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("读备份文件失败: %v", err)
	}
	if len(backupBytes) < 20 {
		t.Fatal("备份文件异常偏小")
	}

	// 4. 修改数据:新增 after_backup.sh
	w = doRequest(t, "PUT", "/api/v1/scripts/content", token, map[string]string{
		"path": "after_backup.sh", "content": "#!/bin/bash\necho after\n",
	})
	assertCode(t, w, 200, 0, "save script after backup")

	// 5. 上传恢复
	w = doUpload(t, "/api/v1/backups/restore", token, "file", backupName, backupBytes)
	assertCode(t, w, 200, 0, "restore backup")

	// 6. 验证:脚本树回到备份时状态(before 存在,after 不存在)
	w = doRequest(t, "GET", "/api/v1/scripts/tree", token, nil)
	assertCode(t, w, 200, 0, "list scripts after restore")
	body := w.Body.String()
	if !strings.Contains(body, "before_backup.sh") {
		t.Fatalf("恢复后应包含 before_backup.sh, got %s", body)
	}
	if strings.Contains(body, "after_backup.sh") {
		t.Fatalf("恢复后不应包含 after_backup.sh, got %s", body)
	}

	// 清理备份文件
	_ = os.Remove(backupPath)
}

// ---------- IP 白名单 ----------

func TestIPWhitelist(t *testing.T) {
	defer func() { config.C.Security.IPWhitelist = nil }()

	// httptest 请求默认 RemoteAddr 为 192.0.2.1:1234,可信代理默认仅回环,XFF 无效。
	check := func(want int, desc string) {
		t.Helper()
		w := doRequest(t, "GET", "/api/v1/health", "", nil)
		if w.Code != want {
			t.Fatalf("%s: http = %d, want %d (body=%s)", desc, w.Code, want, w.Body.String())
		}
	}

	config.C.Security.IPWhitelist = []string{"192.0.2.0/24"}
	check(200, "CIDR 内放行")

	config.C.Security.IPWhitelist = []string{"127.0.0.0/8"}
	check(403, "CIDR 外拒绝")

	config.C.Security.IPWhitelist = []string{"192.0.2.1"}
	check(200, "单个 IP 放行")

	config.C.Security.IPWhitelist = []string{"10.0.0.0/8"}
	check(403, "单个 IP 外拒绝")

	config.C.Security.IPWhitelist = nil
	check(200, "空列表全部放行")
}

// ---------- 依赖管理 ----------

func TestDeps(t *testing.T) {
	token := getToken(t)

	// 非法包名(命令注入尝试)应被拒绝
	w := doRequest(t, "POST", "/api/v1/deps/python/install", token, map[string]string{
		"package": "pkg;rm -rf /",
	})
	assertCode(t, w, 400, 400, "注入字符拒绝")

	// 空包名应被拒绝
	w = doRequest(t, "POST", "/api/v1/deps/node/uninstall", token, map[string]string{
		"package": " ",
	})
	assertCode(t, w, 400, 400, "空包名拒绝")

	// 环境无 pip3 时应返回友好错误(有 pip3 则列表应 200)
	hasPip := func() bool {
		_, err := exec.LookPath("pip3")
		return err == nil
	}()
	w = doRequest(t, "GET", "/api/v1/deps/python", token, nil)
	if hasPip {
		assertCode(t, w, 200, 0, "pip3 列表")
	} else {
		assertCode(t, w, 400, 400, "无 pip3 友好错误")
	}

	// npm 同理
	hasNpm := func() bool {
		_, err := exec.LookPath("npm")
		return err == nil
	}()
	w = doRequest(t, "GET", "/api/v1/deps/node", token, nil)
	if hasNpm {
		assertCode(t, w, 200, 0, "npm 列表")
	} else {
		assertCode(t, w, 400, 400, "无 npm 友好错误")
	}
}

// ---------- 系统监控 ----------

func TestSystemStats(t *testing.T) {
	token := getToken(t)
	w := doRequest(t, "GET", "/api/v1/system/stats", token, nil)
	assertCode(t, w, 200, 0, "system stats")
	var st struct {
		Data struct {
			MemTotal   uint64  `json:"mem_total"`
			DiskTotal  uint64  `json:"disk_total"`
			Hostname   string  `json:"hostname"`
			Uptime     uint64  `json:"uptime_seconds"`
		} `json:"data"`
	}
	if err := json.Unmarshal(decodeResp(t, w).Data, &st); err != nil {
		t.Fatalf("decode stats: %v", err)
	}
	if st.Data.MemTotal == 0 {
		t.Fatal("mem_total 应为正数")
	}
	if st.Data.DiskTotal == 0 {
		t.Fatal("disk_total 应为正数")
	}
	if st.Data.Hostname == "" {
		t.Fatal("hostname 不应为空")
	}
	// 未登录应 401
	w = doRequest(t, "GET", "/api/v1/system/stats", "", nil)
	if w.Code != 401 {
		t.Fatalf("未登录应 401, got %d", w.Code)
	}
}

// ---------- Open API ----------

func TestOpenAPI(t *testing.T) {
	token := getToken(t)

	// 创建应用
	w := doRequest(t, "POST", "/api/v1/open/apps", token, map[string]interface{}{
		"name": "测试应用", "scopes": []string{"tasks:read", "tasks:run"},
	})
	assertCode(t, w, 201, 0, "create app")
	var created struct {
		Data struct {
			ID           uint   `json:"id"`
			ClientID     string `json:"client_id"`
			ClientSecret string `json:"client_secret"`
		} `json:"data"`
	}
	_ = json.Unmarshal(decodeResp(t, w).Data, &created)
	if created.Data.ClientID == "" || created.Data.ClientSecret == "" {
		t.Fatal("client_id/secret 不应为空")
	}

	// 保留字拒绝
	w = doRequest(t, "POST", "/api/v1/open/apps", token, map[string]interface{}{
		"name": "system", "scopes": []string{},
	})
	assertCode(t, w, 400, 400, "保留字拒绝")

	// 非法 scope 拒绝
	w = doRequest(t, "POST", "/api/v1/open/apps", token, map[string]interface{}{
		"name": "x", "scopes": []string{"bad:scope"},
	})
	assertCode(t, w, 400, 400, "非法 scope 拒绝")

	// 列表:应包含解析后的 scopes 数组(回归:曾因 json:"-" 导致前端权限范围为空)
	w = doRequest(t, "GET", "/api/v1/open/apps", token, nil)
	assertCode(t, w, 200, 0, "list apps")
	var appList struct {
		Data []struct {
			ID     uint     `json:"id"`
			Scopes []string `json:"scopes"`
		} `json:"data"`
	}
	_ = json.Unmarshal(decodeResp(t, w).Data, &appList)
	foundApp := false
	for _, a := range appList.Data {
		if a.ID == created.Data.ID {
			foundApp = true
			if len(a.Scopes) != 2 || a.Scopes[0] != "tasks:read" {
				t.Fatalf("列表应含 scopes 数组: %+v", a.Scopes)
			}
		}
	}
	if !foundApp {
		t.Fatal("列表未包含刚创建的应用")
	}

	// 错误密钥换 token → 401
	w = doRequest(t, "POST", "/api/v1/open/auth/token", "", map[string]string{
		"client_id": created.Data.ClientID, "client_secret": "wrong-secret",
	})
	assertCode(t, w, 401, 401, "错误 secret 拒绝")

	// 正确换 token
	w = doRequest(t, "POST", "/api/v1/open/auth/token", "", map[string]string{
		"client_id": created.Data.ClientID, "client_secret": created.Data.ClientSecret,
	})
	assertCode(t, w, 200, 0, "换取 token")
	var tok struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	_ = json.Unmarshal(decodeResp(t, w).Data, &tok)
	if tok.Data.Token == "" {
		t.Fatal("token 为空")
	}
	// 带 scope 的接口放行(tasks:read);doRequest 会自动加 "Bearer " 前缀
	openAuth := tok.Data.Token
	w = doRequest(t, "GET", "/api/v1/open/tasks", openAuth, nil)
	assertCode(t, w, 200, 0, "open tasks 放行")

	// 无对应 scope 的接口拒绝(logs:read 未授予)→ 403
	w = doRequest(t, "GET", "/api/v1/open/logs", openAuth, nil)
	if w.Code != 403 {
		t.Fatalf("未授权 scope 应 403, got %d (body=%s)", w.Code, w.Body.String())
	}

	// 无 token → 401
	w = doRequest(t, "GET", "/api/v1/open/tasks", "", nil)
	assertCode(t, w, 401, 401, "无 token 拒绝")

	// 重置密钥后旧 secret 失效
	w = doRequest(t, "PUT", fmt.Sprintf("/api/v1/open/apps/%d/reset-secret", created.Data.ID), token, nil)
	assertCode(t, w, 200, 0, "reset secret")
	var reset struct {
		Data struct {
			ClientSecret string `json:"client_secret"`
		} `json:"data"`
	}
	_ = json.Unmarshal(decodeResp(t, w).Data, &reset)
	if reset.Data.ClientSecret == "" || reset.Data.ClientSecret == created.Data.ClientSecret {
		t.Fatal("新 secret 应不同")
	}
	w = doRequest(t, "POST", "/api/v1/open/auth/token", "", map[string]string{
		"client_id": created.Data.ClientID, "client_secret": created.Data.ClientSecret,
	})
	assertCode(t, w, 401, 401, "旧 secret 失效")

	// 清理
	_ = doRequest(t, "DELETE", fmt.Sprintf("/api/v1/open/apps/%d", created.Data.ID), token, nil)
}

// ---------- 2FA / TOTP ----------

func TestTOTP(t *testing.T) {
	token := getToken(t)

	// 初始状态:未启用
	w := doRequest(t, "GET", "/api/v1/auth/totp/status", token, nil)
	assertCode(t, w, 200, 0, "totp status")
	var st struct {
		Data struct {
			Enabled bool `json:"enabled"`
		} `json:"data"`
	}
	_ = json.Unmarshal(decodeResp(t, w).Data, &st)
	if st.Data.Enabled {
		t.Fatal("初始不应启用 2FA")
	}

	// 生成绑定密钥
	w = doRequest(t, "GET", "/api/v1/auth/totp/setup", token, nil)
	assertCode(t, w, 200, 0, "totp setup")
	var setup struct {
		Data struct {
			Secret string `json:"secret"`
			URI    string `json:"uri"`
		} `json:"data"`
	}
	_ = json.Unmarshal(decodeResp(t, w).Data, &setup)
	if setup.Data.Secret == "" || !strings.Contains(setup.Data.URI, "otpauth://") {
		t.Fatal("setup 返回非法")
	}

	// 错误码启用 → 400
	w = doRequest(t, "POST", "/api/v1/auth/totp/enable", token, map[string]string{
		"secret": setup.Data.Secret, "code": "000000",
	})
	assertCode(t, w, 400, 400, "错误码启用拒绝")

	// 正确码启用 → 200
	code := totp.CurrentCode(setup.Data.Secret)
	w = doRequest(t, "POST", "/api/v1/auth/totp/enable", token, map[string]string{
		"secret": setup.Data.Secret, "code": code,
	})
	assertCode(t, w, 200, 0, "启用 2FA")

	// 现在登录必须带 TOTP(用唯一 IP,避免共享 IP 限流/锁定计数)
	uniqueIP := fmt.Sprintf("10.9.%d.%d", time.Now().UnixNano()%200+1, time.Now().UnixNano()%200+1)
	loginBody := map[string]string{"username": "admin", "password": "Admin@12345"}
	w = doRequestIP(t, "POST", "/api/v1/auth/login", "", uniqueIP, loginBody)
	assertCode(t, w, 401, 401, "不带 TOTP 拒绝")
	w = doRequestIP(t, "POST", "/api/v1/auth/login", "", uniqueIP, map[string]string{
		"username": "admin", "password": "Admin@12345", "totp_code": "000000",
	})
	assertCode(t, w, 401, 401, "错误 TOTP 拒绝")
	w = doRequestIP(t, "POST", "/api/v1/auth/login", "", uniqueIP, map[string]string{
		"username": "admin", "password": "Admin@12345", "totp_code": totp.CurrentCode(setup.Data.Secret),
	})
	assertCode(t, w, 200, 0, "正确 TOTP 登录")

	// 解除(需当前密码)
	w = doRequest(t, "POST", "/api/v1/auth/totp/disable", token, map[string]string{
		"password": "wrong-password",
	})
	assertCode(t, w, 400, 400, "错误密码解除拒绝")
	w = doRequest(t, "POST", "/api/v1/auth/totp/disable", token, map[string]string{
		"password": "Admin@12345",
	})
	assertCode(t, w, 200, 0, "解除 2FA")

	// 解除后登录不再需要 TOTP
	w = doRequest(t, "POST", "/api/v1/auth/login", "", loginBody)
	assertCode(t, w, 200, 0, "解除后正常登录")
}

// doRequestIP 与 doRequest 相同,但可指定来源 IP(避免触发共享 IP 的登录限流/锁定)。
func doRequestIP(t *testing.T, method, path, token, ip string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.RemoteAddr = ip + ":12345"
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	testEngine.ServeHTTP(w, req)
	return w
}

// ---------- 任务标签与批量操作 ----------

func TestTaskTags(t *testing.T) {
	token := getToken(t)

	// 准备脚本
	w := doRequest(t, "PUT", "/api/v1/scripts/content", token, map[string]string{
		"path": "tag_test.sh", "content": "#!/bin/bash\necho ok\n",
	})
	assertCode(t, w, 200, 0, "save script")

	// 创建带标签任务
	mk := func(name string, tags []string) uint {
		t.Helper()
		w := doRequest(t, "POST", "/api/v1/tasks", token, map[string]interface{}{
			"name": name, "command": "tag_test.sh", "cron_expression": "0 3 * * *",
			"enabled": false, "tags": tags,
		})
		assertCode(t, w, 201, 0, "create task "+name)
		var c struct {
			Data struct {
				ID   uint     `json:"id"`
				Tags []string `json:"tags"`
			} `json:"data"`
		}
		_ = json.Unmarshal(decodeResp(t, w).Data, &c)
		if len(c.Data.Tags) != len(tags) {
			t.Fatalf("tags 未保存: %+v", c.Data.Tags)
		}
		return c.Data.ID
	}
	idA := mk("标签任务A", []string{"备份", "日常"})
	idB := mk("标签任务B", []string{"爬虫"})

	// 按标签过滤
	enc := url.QueryEscape("备份")
	w = doRequest(t, "GET", "/api/v1/tasks?tag="+enc, token, nil)
	assertCode(t, w, 200, 0, "filter by tag")
	var list struct {
		Data []struct {
			ID   uint     `json:"id"`
			Tags []string `json:"tags"`
		} `json:"data"`
	}
	_ = json.Unmarshal(decodeResp(t, w).Data, &list)
	if len(list.Data) != 1 || list.Data[0].ID != idA {
		t.Fatalf("按标签过滤结果不符: %+v", list.Data)
	}

	// 标签统计
	w = doRequest(t, "GET", "/api/v1/tasks/tags", token, nil)
	assertCode(t, w, 200, 0, "tag stats")
	var stats struct {
		Data map[string]int `json:"data"`
	}
	_ = json.Unmarshal(decodeResp(t, w).Data, &stats)
	if stats.Data["备份"] != 1 || stats.Data["爬虫"] != 1 || stats.Data["日常"] != 1 {
		t.Fatalf("标签统计不符: %+v", stats.Data)
	}

	// 批量禁用/启用/运行/删除
	ids := []uint{idA, idB}
	for _, path := range []string{"disable", "enable", "run", "delete"} {
		w = doRequest(t, "POST", "/api/v1/tasks/batch/"+path, token, map[string]interface{}{"ids": ids})
		assertCode(t, w, 200, 0, "batch "+path)
	}

	// 批量删除后列表应为空
	w = doRequest(t, "GET", "/api/v1/tasks", token, nil)
	_ = json.Unmarshal(decodeResp(t, w).Data, &list)
	for _, tk := range list.Data {
		if tk.ID == idA || tk.ID == idB {
			t.Fatal("批量删除未生效")
		}
	}

	// 清理脚本
	_ = doRequest(t, "DELETE", "/api/v1/scripts?path=tag_test.sh", token, nil)
}

// ---------- 批量导入/导出 ----------

func TestMigrate(t *testing.T) {
	token := getToken(t)

	// 准备数据:脚本 + 任务 + 环境变量
	w := doRequest(t, "PUT", "/api/v1/scripts/content", token, map[string]string{
		"path": "mig_test.sh", "content": "#!/bin/bash\necho migrate\n",
	})
	assertCode(t, w, 200, 0, "save script")
	w = doRequest(t, "POST", "/api/v1/tasks", token, map[string]interface{}{
		"name": "迁移任务", "command": "mig_test.sh", "cron_expression": "0 3 * * *",
		"enabled": false, "tags": []string{"迁移"},
	})
	assertCode(t, w, 201, 0, "create task")
	w = doRequest(t, "POST", "/api/v1/envs", token, map[string]interface{}{
		"name": "MIG_KEY", "value": "mig-value", "group": "迁移", "enabled": true,
	})
	assertCode(t, w, 201, 0, "create env")

	// 导出
	w = doRequest(t, "GET", "/api/v1/migrate/export", token, nil)
	assertCode(t, w, 200, 0, "export")
	var exp struct {
		Data struct {
			Tasks   []map[string]interface{} `json:"tasks"`
			Scripts []map[string]interface{} `json:"scripts"`
			Envs    []map[string]interface{} `json:"envs"`
		} `json:"data"`
	}
	if err := json.Unmarshal(decodeResp(t, w).Data, &exp); err != nil {
		t.Fatalf("decode export: %v", err)
	}
	if len(exp.Data.Tasks) == 0 || len(exp.Data.Scripts) == 0 || len(exp.Data.Envs) == 0 {
		t.Fatalf("导出内容不全: tasks=%d scripts=%d envs=%d",
			len(exp.Data.Tasks), len(exp.Data.Scripts), len(exp.Data.Envs))
	}
	hasMigValue := false
	for _, e := range exp.Data.Envs {
		if e["value"] == "mig-value" {
			hasMigValue = true
		}
	}
	if !hasMigValue {
		t.Fatal("导出应包含环境变量明文值")
	}

	// 清理数据(模拟迁移到空环境)
	_ = doRequest(t, "DELETE", "/api/v1/scripts?path=mig_test.sh", token, nil)
	w = doRequest(t, "GET", "/api/v1/tasks?keyword=迁移任务", token, nil)
	assertCode(t, w, 200, 0, "list task to delete")
	var list struct {
		Data []struct {
			ID uint `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(decodeResp(t, w).Data, &list)
	if len(list.Data) > 0 {
		_ = doRequest(t, "DELETE", fmt.Sprintf("/api/v1/tasks/%d", list.Data[0].ID), token, nil)
	}
	w = doRequest(t, "GET", "/api/v1/envs", token, nil)
	var envList struct {
		Data []struct {
			ID uint `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(decodeResp(t, w).Data, &envList)
	for _, e := range envList.Data {
		_ = doRequest(t, "DELETE", fmt.Sprintf("/api/v1/envs/%d", e.ID), token, nil)
	}

	// 导入
	w = doRequest(t, "POST", "/api/v1/migrate/import", token, map[string]interface{}{
		"tasks": exp.Data.Tasks, "scripts": exp.Data.Scripts, "envs": exp.Data.Envs,
	})
	assertCode(t, w, 200, 0, "import")
	var ir struct {
		Data struct {
			TasksOK   int `json:"tasks_ok"`
			ScriptsOK int `json:"scripts_ok"`
			EnvsOK    int `json:"envs_ok"`
		} `json:"data"`
	}
	_ = json.Unmarshal(decodeResp(t, w).Data, &ir)
	if ir.Data.TasksOK < 1 || ir.Data.ScriptsOK < 1 || ir.Data.EnvsOK < 1 {
		t.Fatalf("导入结果不符: %+v", ir.Data)
	}

	// 验证恢复
	w = doRequest(t, "GET", "/api/v1/scripts/tree", token, nil)
	if !strings.Contains(w.Body.String(), "mig_test.sh") {
		t.Fatal("脚本未恢复")
	}
	w = doRequest(t, "GET", "/api/v1/tasks", token, nil)
	body := w.Body.String()
	if !strings.Contains(body, "迁移任务") || !strings.Contains(body, "迁移") {
		t.Fatal("任务或标签未恢复")
	}

	// 清理导入的数据
	_ = doRequest(t, "DELETE", "/api/v1/scripts?path=mig_test.sh", token, nil)
	w = doRequest(t, "GET", "/api/v1/tasks?keyword=迁移任务", token, nil)
	_ = json.Unmarshal(decodeResp(t, w).Data, &list)
	if len(list.Data) > 0 {
		_ = doRequest(t, "DELETE", fmt.Sprintf("/api/v1/tasks/%d", list.Data[0].ID), token, nil)
	}
}

// ---------- 助手 ----------

var (
	tokenMu  sync.Mutex
	sharedTok string
)

// getToken 返回一个已登录 token(缓存复用,避免触发登录限流)。
func getToken(t *testing.T) string {
	t.Helper()
	tokenMu.Lock()
	defer tokenMu.Unlock()
	if sharedTok != "" {
		return sharedTok
	}
	sharedTok = mustLogin(t)
	return sharedTok
}

func mustLogin(t *testing.T) string {
	t.Helper()
	w := doRequest(t, "GET", "/api/v1/auth/check-init", "", nil)
	var ci struct {
		NeedInit bool `json:"need_init"`
	}
	_ = json.Unmarshal(decodeResp(t, w).Data, &ci)
	if ci.NeedInit {
		w = doRequest(t, "POST", "/api/v1/auth/init", "", map[string]string{
			"username": "admin", "password": "Admin@12345",
		})
		if w.Code != 201 {
			t.Fatalf("init failed: %s", w.Body.String())
		}
	}
	w = doRequest(t, "POST", "/api/v1/auth/login", "", map[string]string{
		"username": "admin", "password": "Admin@12345",
	})
	if w.Code != 200 {
		t.Fatalf("login failed: %s", w.Body.String())
	}
	var lr struct {
		AccessToken string `json:"access_token"`
	}
	_ = json.Unmarshal(decodeResp(t, w).Data, &lr)
	if lr.AccessToken == "" {
		t.Fatal("empty token")
	}
	return lr.AccessToken
}
