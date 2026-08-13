package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"taskpanel/config"
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
		CORS: config.CORSConfig{Origins: []string{"http://localhost:5173"}},
	}
	for _, d := range []string{config.C.Data.Dir, config.C.Data.ScriptsDir, config.C.Data.LogDir} {
		_ = os.MkdirAll(d, 0o755)
	}

	if err := database.Init(config.C.Database.Path); err != nil {
		log.Fatal(err)
	}

	testEngine = gin.New()
	testEngine.Use(gin.Logger(), gin.Recovery())
	router.Setup(testEngine)

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
		if _, err := svc.Login("admin", "wrong-password", ip); err == nil {
			t.Fatalf("attempt %d should fail", i+1)
		}
	}
	// 第 6 次应返回锁定
	if _, err := svc.Login("admin", "wrong-password", ip); err != service.ErrAccountLocked {
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
