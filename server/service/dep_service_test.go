package service

import (
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
