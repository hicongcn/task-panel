package service

import (
	"strings"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidateNotifyConfig(t *testing.T) {
	cases := []struct {
		typ string
		cfg map[string]interface{}
		ok  bool
	}{
		{"webhook", map[string]interface{}{"url": "https://x.com/hook"}, true},
		{"webhook", map[string]interface{}{"url": "ftp://x.com"}, false},
		{"webhook", map[string]interface{}{}, false},
		{"telegram", map[string]interface{}{"bot_token": "t", "chat_id": "c"}, true},
		{"telegram", map[string]interface{}{"bot_token": "t"}, false},
		{"bark", map[string]interface{}{"device_key": "k"}, true},
		{"bark", map[string]interface{}{}, false},
		{"email", map[string]interface{}{"host": "h", "from": "f", "to": "t"}, true},
		{"email", map[string]interface{}{"host": "h"}, false},
		{"unknown", map[string]interface{}{}, false},
	}
	for _, c := range cases {
		err := validateNotifyConfig(c.typ, c.cfg)
		if c.ok && err != nil {
			t.Errorf("%s 应通过,却报错: %v", c.typ, err)
		}
		if !c.ok && err == nil {
			t.Errorf("%s 应失败,却通过", c.typ)
		}
	}
}

func TestSendWebhook(t *testing.T) {
	var got map[string]string
	done := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusOK)
		done <- struct{}{}
	}))
	defer srv.Close()

	cfgBytes, _ := json.Marshal(map[string]interface{}{"url": srv.URL})
	if err := sendWebhook(string(cfgBytes), "标题", "内容"); err != nil {
		t.Fatalf("sendWebhook: %v", err)
	}
	<-done
	if got["title"] != "标题" || got["content"] != "内容" {
		t.Fatalf("payload 不符: %+v", got)
	}
}

func TestSendBark(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfgBytes, _ := json.Marshal(map[string]interface{}{"server": srv.URL, "device_key": "mykey"})
	if err := sendBark(string(cfgBytes), "hello", "world"); err != nil {
		t.Fatalf("sendBark: %v", err)
	}
	if path != "/mykey/hello/world" {
		t.Fatalf("bark path 不符: %s", path)
	}
}

// TestDefaultTemplate 默认模板渲染(buildTaskResultMessage 依赖 DB 设置,改测纯函数)。
func TestDefaultTemplate(t *testing.T) {
	title, content := renderNotifyTemplate(defaultNotifyTemplate("success"), "每日备份", "成功", 12.5)
	if title != "任务执行成功" {
		t.Fatalf("title 不符: %s", title)
	}
	if content == "" || !strings.Contains(content, "每日备份") {
		t.Fatalf("content 不符: %s", content)
	}
}

func TestRenderNotifyTemplate(t *testing.T) {
	title, content := renderNotifyTemplate("自定义成功\n任务 {task_name} 好了,耗时 {duration}s", "备份", "成功", 3.5)
	if title != "自定义成功" || content != "任务 备份 好了,耗时 3.50s" {
		t.Fatalf("渲染不符: %q / %q", title, content)
	}
	// 空模板回退默认
	title, content = renderNotifyTemplate("", "任务A", "成功", 1.0)
	if title != "任务执行成功" || content == "" {
		t.Fatalf("默认模板不符: %q / %q", title, content)
	}
	// 无换行:内容为空
	title, content = renderNotifyTemplate("只有标题", "x", "成功", 1)
	if title != "只有标题" || content != "" {
		t.Fatalf("无换行模板不符: %q / %q", title, content)
	}
}
