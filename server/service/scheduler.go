package service

import (
	"fmt"
	"log"
	"sync"
	"time"

	"taskpanel/database"
	"taskpanel/model"
	"taskpanel/pkg/cronutil"

	"github.com/robfig/cron/v3"
)

// Scheduler 管理所有任务的 cron 调度。进程内单实例。
type Scheduler struct {
	mu      sync.Mutex
	cron    *cron.Cron
	entries map[uint]cron.EntryID // taskID -> cron entry
}

var defaultScheduler = newScheduler()

func newScheduler() *Scheduler {
	c := cron.New(cron.WithParser(cronutil.Parser()), cron.WithChain())
	c.Start()
	return &Scheduler{cron: c, entries: make(map[uint]cron.EntryID)}
}

// GetScheduler 返回全局调度器。
func GetScheduler() *Scheduler { return defaultScheduler }

// Reload 清空所有调度并重新从数据库加载(备份恢复后调用)。
func (s *Scheduler) Reload() error {
	s.mu.Lock()
	for id := range s.entries {
		s.cron.Remove(s.entries[id])
	}
	s.entries = make(map[uint]cron.EntryID)
	s.mu.Unlock()
	return s.LoadEnabled()
}

// LoadEnabled 启动时加载所有已启用任务并注册。
func (s *Scheduler) LoadEnabled() error {
	var tasks []model.Task
	if err := database.DB.Where("enabled = ?", true).Find(&tasks).Error; err != nil {
		return err
	}
	for i := range tasks {
		t := tasks[i]
		if err := s.Add(&t); err != nil {
			log.Printf("warn: 注册任务 %d 失败: %v", t.ID, err)
		}
	}
	return nil
}

// Add 注册或更新某任务的调度。
func (s *Scheduler) Add(task *model.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 先移除旧的
	if id, ok := s.entries[task.ID]; ok {
		s.cron.Remove(id)
		delete(s.entries, task.ID)
	}

	if !task.Enabled {
		return nil
	}
	if err := cronutil.Validate(task.CronExpression); err != nil {
		return err
	}

	id, err := s.cron.AddFunc(task.CronExpression, func() {
		s.trigger(task.ID, false)
	})
	if err != nil {
		return err
	}
	s.entries[task.ID] = id
	return nil
}

// Remove 移除某任务的调度。
func (s *Scheduler) Remove(taskID uint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id, ok := s.entries[taskID]; ok {
		s.cron.Remove(id)
		delete(s.entries, taskID)
	}
}

// RunNow 手动触发一次执行。返回错误表示已在运行或入队失败。
func (s *Scheduler) RunNow(taskID uint) error {
	return s.trigger(taskID, true)
}

// trigger 触发执行。immediate=true 表示手动触发。
// MVP 采用同步语义:同一任务串行执行,已在运行则拒绝。
// 通过 Executor.Acquire 原子占位,避免"并发触发 + 异步启动"窗口内重复执行。
func (s *Scheduler) trigger(taskID uint, immediate bool) error {
	// 加载最新任务
	var task model.Task
	if err := database.DB.First(&task, taskID).Error; err != nil {
		return fmt.Errorf("任务不存在: %w", err)
	}

	exec := GetExecutor()
	// 原子占位:已在运行(含启动窗口)则拒绝
	if !exec.Acquire(taskID) {
		return fmt.Errorf("任务正在运行中")
	}

	// 占位成功后异步执行,避免阻塞调度器/caller
	go exec.Run(&task)
	_ = immediate
	return nil
}

// Stop 停止调度器并等待在跑任务结束(最多等 5 秒)。
func (s *Scheduler) Stop() {
	stopCtx := s.cron.Stop()
	select {
	case <-stopCtx.Done():
	case <-time.After(5 * time.Second):
		log.Println("warn: 等待调度器停止超时")
	}
}
