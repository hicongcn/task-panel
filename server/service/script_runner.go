package service

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync/atomic"
	"time"

	"taskpanel/pkg/pathutil"
)

// ScriptRunResult 一次脚本调试运行的同步结果。
type ScriptRunResult struct {
	Output    string
	ExitCode  int
	Duration  float64
	TimedOut  bool
}

// RunScriptDebug 以调试方式同步运行一个已解析的命令计划,收集输出。
// 用于脚本编辑器"运行代码/运行脚本"。timeout<=0 时默认 60 秒。
func RunScriptDebug(plan *CommandPlan, env map[string]string, timeout time.Duration) ScriptRunResult {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	start := time.Now()

	binary, args := plan.CommandParts()
	cmd := exec.Command(binary, args...)
	cmd.Dir = plan.WorkDir
	cmd.Env = buildDebugEnv(env)
	setProcessGroup(cmd)

	// 合并 stdout/stderr 到一个管道,按行扫描
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		scanner := bufio.NewScanner(pr)
		scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
		for scanner.Scan() {
			buf.WriteString(scanner.Text())
			buf.WriteByte('\n')
		}
		close(done)
	}()

	startErr := cmd.Start()
	if startErr != nil {
		pw.Close()
		<-done
		return ScriptRunResult{
			Output: fmt.Sprintf("启动失败: %v\n", startErr),
			ExitCode: 1, Duration: time.Since(start).Seconds(),
		}
	}

	killed := atomic.Bool{}
	timer := time.AfterFunc(timeout, func() {
		killed.Store(true)
		killProcessGroup(cmd.Process)
	})

	err := cmd.Wait()
	pw.Close()
	<-done
	timer.Stop()

	exitCode := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		} else {
			exitCode = 1
		}
	}
	return ScriptRunResult{
		Output:   buf.String(),
		ExitCode: exitCode,
		Duration: time.Since(start).Seconds(),
		TimedOut: killed.Load(),
	}
}

// buildDebugEnv 构造调试运行环境:安全基线 + 用户环境(过滤危险变量)。
func buildDebugEnv(env map[string]string) []string {
	out := []string{
		"PATH=" + safeEnvPath(),
		"HOME=" + safeEnvHome(),
		"TZ=" + currentTZ(),
	}
	for k, v := range env {
		if isDangerousEnvName(k) {
			continue
		}
		out = append(out, k+"="+v)
	}
	return out
}

// ResolveScriptPath 校验脚本相对路径在脚本目录内并返回绝对路径(mustExist)。
// 供 handler 复用,统一穿越防护入口。
func ResolveScriptPath(scriptsDir, relPath string, mustExist bool) (string, error) {
	return pathutil.SafeJoin(scriptsDir, relPath, mustExist)
}

func safeEnvPath() string {
	if v := os.Getenv("PATH"); v != "" {
		return v
	}
	return "/usr/local/bin:/usr/bin:/bin"
}

func safeEnvHome() string {
	if v := os.Getenv("HOME"); v != "" {
		return v
	}
	return "/tmp"
}
