package service

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestValidatePkg(t *testing.T) {
	valid := []string{"requests", "numpy==1.26.0", "pandas>=2.0", "numpy>1.0", "uvicorn[standard]", "eslint", "axios"}
	for _, p := range valid {
		if err := validatePkg(p); err != nil {
			t.Errorf("%q 应通过,却报错: %v", p, err)
		}
	}
	invalid := []string{"", " ", "a b", "pkg;rm -rf /", "pkg$(id)", "`ls`", "pkg|cat", "pkg\n", "pkg\t", "pkg\x00x"}
	for _, p := range invalid {
		if err := validatePkg(p); err == nil {
			t.Errorf("%q 应被拒绝", p)
		}
	}
}

func TestExtractJSONValue(t *testing.T) {
	// 前置警告 + JSON(模拟 macOS pip 输出)
	out := `WARNING: The directory '/Users/x/Library/Caches/pip' or its parent directory is not owned...
[{"name": "requests", "version": "2.34.2"}]
`
	raw, err := extractJSONValue(out)
	if err != nil {
		t.Fatalf("extractJSONValue: %v", err)
	}
	var pkgs []PkgInfo
	if err := json.Unmarshal(raw, &pkgs); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(pkgs) != 1 || pkgs[0].Name != "requests" {
		t.Fatalf("解析结果不符: %+v", pkgs)
	}

	// npm 风格对象(前置 notice 文本)
	out2 := `npm notice something
{"dependencies":{"axios":{"version":"1.6.0"}}}
`
	raw, err = extractJSONValue(out2)
	if err != nil {
		t.Fatalf("extractJSONValue obj: %v", err)
	}
	var res map[string]interface{}
	if err := json.Unmarshal(raw, &res); err != nil {
		t.Fatalf("unmarshal obj: %v", err)
	}
	deps := res["dependencies"].(map[string]interface{})
	axios := deps["axios"].(map[string]interface{})
	if axios["version"] != "1.6.0" {
		t.Fatalf("对象解析结果不符: %+v", res)
	}

	// 无 JSON → 报错
	if _, err := extractJSONValue("全是警告,没有 JSON"); err == nil {
		t.Fatal("应报错")
	}

	// 损坏 JSON → 报错
	if _, err := extractJSONValue(`{"broken"`); err == nil {
		t.Fatal("损坏 JSON 应报错")
	}
}

func TestRunCmd(t *testing.T) {
	out, err := runCmd(t.Context(), 5*time.Second, "echo", "hello", "world")
	if err != nil {
		t.Fatalf("runCmd: %v", err)
	}
	if !strings.Contains(out, "hello world") {
		t.Fatalf("输出不符: %q", out)
	}

	// 超时
	start := time.Now()
	_, err = runCmd(t.Context(), 100*time.Millisecond, "sleep", "5")
	if err == nil {
		t.Fatal("应超时报错")
	}
	if time.Since(start) > 3*time.Second {
		t.Fatal("超时未生效")
	}
}
