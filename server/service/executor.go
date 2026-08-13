package service

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"taskpanel/config"
	"taskpanel/database"
	"taskpanel/model"
)

// Executor 负责把一个任务计划跑起来,管理超时/重试/停止/日志。
type Executor struct {
	mu        sync.Mutex
	running   map[uint]*runHandle // taskID -> 当前运行句柄
}

type runHandle struct {
	taskID  uint
	cancel  context.CancelFunc
	process *os.Process
	mu      sync.Mutex
	stopped bool
}

var defaultExecutor = &Executor{running: make(map[uint]*runHandle)}

// GetExecutor 返回全局执行器单例。
func GetExecutor() *Executor { return defaultExecutor }

// ManualStop 标记任务为手动停止(在 kill 之前调用,保证完成块结算时识别为 aborted)。
func (e *Executor) ManualStop(taskID uint) bool {
	e.mu.Lock()
	h, ok := e.running[taskID]
	e.mu.Unlock()
	if !ok {
		return false
	}
	// 在锁内拷贝 cancel/process,避免与 registerProcess 的并发写产生数据竞争;
	// 拷贝后在锁外执行 kill,不持有锁做系统调用。
	h.mu.Lock()
	h.stopped = true
	cancel := h.cancel
	process := h.process
	h.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if process != nil {
		killProcessGroup(process)
	}
	return true
}

// Acquire 原子地检查并占位一个任务的执行槽位:同一任务同时只允许一个执行实例。
// 成功返回 true;若任务已在运行(含正在启动的窗口)返回 false。
func (e *Executor) Acquire(taskID uint) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.running[taskID]; ok {
		return false
	}
	e.running[taskID] = &runHandle{taskID: taskID}
	return true
}

// isStopped 返回任务是否已被手动标记停止。
func (e *Executor) isStopped(taskID uint) bool {
	h := e.getHandle(taskID)
	if h == nil {
		return false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.stopped
}

// Run 同步执行一个任务直到结束(含重试)。调用方须先 Acquire 占位,本函数负责释放。
func (e *Executor) Run(task *model.Task) {
	defer e.clearRunning(task.ID)

	plan, err := ParseCommand(task.Command)
	if err != nil {
		e.finishTaskWithError(task, err)
		return
	}

	logPath := buildLogPath(task.ID)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		e.finishTaskWithError(task, fmt.Errorf("打开日志文件失败: %w", err))
		return
	}

	now := time.Now()
	taskLog := &model.TaskLog{
		TaskID: task.ID, TaskName: task.Name,
		Status: model.LogStatusRunning, LogPath: logPath, StartedAt: now,
	}
	if err := database.DB.Create(taskLog).Error; err != nil {
		logFile.Close()
		e.finishTaskWithError(task, fmt.Errorf("创建日志记录失败: %w", err))
		return
	}

	broker := GetLogBroker().GetOrCreate(taskLog.ID)
	writeOutput := func(line string) {
		broker.Append(line)
		_, _ = logFile.WriteString(line + "\n")
	}

	startTime := time.Now()
	writeOutput(fmt.Sprintf("=== 开始执行 [%s] %s ===", startTime.Format("2006-01-02 15:04:05"), task.Name))

	// 标记任务为运行中
	database.DB.Model(task).Updates(map[string]interface{}{
		"status": model.TaskStatusRunning,
		"last_run_at": &now,
	})

	exitCode, success := e.runWithRetries(task, plan, writeOutput)

	logFile.Close()
	endedAt := time.Now()
	duration := endedAt.Sub(startTime).Seconds()

	// 判定最终状态:手动停止 -> aborted
	finalRunStatus := model.RunStatusFailed
	finalLogStatus := model.LogStatusFailed
	if success {
		finalRunStatus = model.RunStatusSuccess
		finalLogStatus = model.LogStatusSuccess
	}
	if e.isStopped(task.ID) {
		finalRunStatus = model.RunStatusAborted
		finalLogStatus = model.LogStatusAborted
	}

	writeOutput(fmt.Sprintf("=== 执行结束 [%s] 耗时 %.2f 秒 退出码 %d 状态 %s ===",
		endedAt.Format("2006-01-02 15:04:05"), duration, exitCode, finalRunStatus))

	// 收尾:更新任务 + 日志记录,关闭 SSE 广播
	broker.Done()
	GetLogBroker().Remove(taskLog.ID)

	database.DB.Model(taskLog).Updates(map[string]interface{}{
		"status":   finalLogStatus,
		"ended_at": &endedAt,
		"duration": duration,
	})
	database.DB.Model(task).Updates(map[string]interface{}{
		"status": model.TaskStatusIdle,
		"last_run_status": finalRunStatus,
		"last_run_duration": duration,
	})
}

// runWithRetries 执行重试循环,返回最终退出码与是否成功。
// 重试间隔与每次启动前都会检查手动停止标志,保证停止操作立即生效。
func (e *Executor) runWithRetries(task *model.Task, plan *CommandPlan, writeOutput func(string)) (int, bool) {
	maxRetries := task.MaxRetries
	if maxRetries < 0 {
		maxRetries = 0
	}
	exitCode := 1
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			writeOutput(fmt.Sprintf("[第 %d 次重试]", attempt))
			if task.RetryInterval > 0 {
				if !e.sleepInterruptible(task.ID, task.RetryInterval) {
					writeOutput("[手动停止,取消重试]")
					break
				}
			}
		}
		if e.isStopped(task.ID) {
			writeOutput("[手动停止,取消执行]")
			break
		}
		code, err := e.executeOnce(task, plan, writeOutput)
		exitCode = code
		if err == nil && code == 0 {
			return code, true
		}
	}
	return exitCode, false
}

// sleepInterruptible 分段睡眠,期间检测手动停止;返回 false 表示已被停止。
func (e *Executor) sleepInterruptible(taskID uint, seconds int) bool {
	if seconds <= 0 {
		seconds = 1
	}
	deadline := time.Now().Add(time.Duration(seconds) * time.Second)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for time.Now().Before(deadline) {
		if e.isStopped(taskID) {
			return false
		}
		<-ticker.C
	}
	return !e.isStopped(taskID)
}

// executeOnce 执行一次命令,处理超时与手动停止。
func (e *Executor) executeOnce(task *model.Task, plan *CommandPlan, writeOutput func(string)) (int, error) {
	timeout := time.Duration(task.TimeoutSeconds) * time.Second
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	binary, args := plan.CommandParts()
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = plan.WorkDir
	cmd.Env = buildExecEnv(task)
	setProcessGroup(cmd)

	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	if err := cmd.Start(); err != nil {
		pw.Close()
		return 1, fmt.Errorf("启动失败: %w", err)
	}

	// 注册进程与取消函数供停止使用
	e.registerProcess(task.ID, cmd.Process, cancel)

	// 超时定时器:任务提前结束时 Stop,避免定时器空转。
	var timer *time.Timer
	if timeout > 0 {
		timer = time.AfterFunc(timeout, cancel)
		defer timer.Stop()
	}

	done := make(chan struct{})
	go func() {
		scanner := bufio.NewScanner(pr)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			writeOutput(scanner.Text())
		}
		close(done)
	}()

	err := cmd.Wait()
	pw.Close()
	<-done

	if ctx.Err() == context.DeadlineExceeded {
		writeOutput("[执行超时,已终止]")
		return 1, fmt.Errorf("执行超时")
	}

	exitCode := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		} else {
			exitCode = 1
		}
	}
	return exitCode, err
}

func (e *Executor) registerProcess(taskID uint, p *os.Process, ctx context.CancelFunc) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if h, ok := e.running[taskID]; ok {
		h.mu.Lock()
		h.process = p
		h.cancel = ctx
		stopped := h.stopped
		h.mu.Unlock()
		if stopped {
			// 注册前已被手动停止(启动窗口内的停止),立即终止本次执行。
			ctx()
			killProcessGroup(p)
		}
	}
}

// getHandle 返回某任务当前运行句柄(可能为 nil)。
func (e *Executor) getHandle(taskID uint) *runHandle {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.running[taskID]
}

func (e *Executor) clearRunning(taskID uint) {
	e.mu.Lock()
	delete(e.running, taskID)
	e.mu.Unlock()
}

func (e *Executor) finishTaskWithError(task *model.Task, err error) {
	now := time.Now()
	database.DB.Model(task).Updates(map[string]interface{}{
		"status": model.TaskStatusIdle,
		"last_run_status": model.RunStatusFailed,
		"last_run_at": &now,
		"last_run_duration": 0.0,
	})
	taskLog := &model.TaskLog{
		TaskID: task.ID, TaskName: task.Name,
		Status: model.LogStatusFailed,
		Content: fmt.Sprintf("=== 执行失败 ===\n%s\n", err.Error()),
		StartedAt: now, EndedAt: &now,
	}
	database.DB.Create(taskLog)
}

// buildExecEnv 构造子进程环境:面板安全基线变量 + 已启用环境变量。
func buildExecEnv(task *model.Task) []string {
	env := []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
		"TZ=" + currentTZ(),
	}
	for k, v := range NewEnvService().BuildTaskEnv() {
		if isDangerousEnvName(k) {
			continue
		}
		env = append(env, k+"="+v)
	}
	// 预留脚本回调能力:v0.1 注入面板基础信息,令牌留待 v2 启用
	env = append(env, "PANEL_API_BASE=http://127.0.0.1:"+strconv.Itoa(config.C.Server.Port)+"/api/v1")
	return env
}

// currentTZ 返回面板时区,默认 Asia/Shanghai。
func currentTZ() string {
	tz := os.Getenv("TZ")
	if tz == "" {
		return "Asia/Shanghai"
	}
	return tz
}

// isDangerousEnvName 过滤会被动态链接器/解释器滥用的高危变量。
func isDangerousEnvName(name string) bool {
	switch name {
	case "LD_PRELOAD", "LD_LIBRARY_PATH", "DYLD_INSERT_LIBRARIES", "DYLD_LIBRARY_PATH":
		return true
	}
	return false
}

// buildLogPath 生成任务日志文件路径。
func buildLogPath(taskID uint) string {
	ts := time.Now().Format("20060102_150405")
	name := fmt.Sprintf("task_%d_%s_%d.log", taskID, ts, time.Now().UnixNano())
	return filepath.Join(config.C.Data.LogDir, name)
}
