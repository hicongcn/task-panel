package service

import (
	"fmt"
	"time"

	"taskpanel/database"
	"taskpanel/model"
	"taskpanel/pkg/cronutil"
)

// TaskService 负责任务 CRUD 与调度联动。
type TaskService struct{}

func NewTaskService() *TaskService { return &TaskService{} }

// List 列出任务,支持关键字与状态过滤。
func (s *TaskService) List(keyword, status string) []model.Task {
	q := database.DB.Model(&model.Task{})
	if keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where("name LIKE ?", like)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var tasks []model.Task
	q.Order("updated_at DESC").Find(&tasks)
	return tasks
}

// Get 按 ID 取任务。
func (s *TaskService) Get(id uint) (*model.Task, error) {
	var task model.Task
	if err := database.DB.First(&task, id).Error; err != nil {
		return nil, fmt.Errorf("任务不存在")
	}
	return &task, nil
}

// CreateInput 创建任务入参。
type CreateInput struct {
	Name           string
	Command        string
	CronExpression string
	Enabled        bool
	TimeoutSeconds int
	MaxRetries     int
	RetryInterval  int
}

// Create 新建任务。enabled=true 时同步注册调度。
func (s *TaskService) Create(in CreateInput) (*model.Task, error) {
	if err := validateTaskInput(in); err != nil {
		return nil, err
	}
	status := model.TaskStatusIdle
	runStatus := model.RunStatusNone
	task := &model.Task{
		Name: in.Name, Command: in.Command, CronExpression: in.CronExpression,
		Enabled: in.Enabled, TimeoutSeconds: in.TimeoutSeconds,
		MaxRetries: in.MaxRetries, RetryInterval: in.RetryInterval,
		Status: status, LastRunStatus: runStatus,
	}
	if err := database.DB.Create(task).Error; err != nil {
		return nil, fmt.Errorf("创建失败: %w", err)
	}
	if task.Enabled {
		if err := GetScheduler().Add(task); err != nil {
			// 注册失败回滚 enabled,但保留任务
			database.DB.Model(task).Update("enabled", false)
			task.Enabled = false
		}
	}
	return task, nil
}

// UpdateInput 更新入参(指针字段为 nil 表示不修改)。
type UpdateInput struct {
	Name           *string
	Command        *string
	CronExpression *string
	Enabled        *bool
	TimeoutSeconds *int
	MaxRetries     *int
	RetryInterval  *int
}

func (s *TaskService) Update(id uint, in UpdateInput) (*model.Task, error) {
	task, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	// 先用临时对象做完整性校验
	tmp := CreateInput{
		Name: task.Name, Command: task.Command, CronExpression: task.CronExpression,
		TimeoutSeconds: task.TimeoutSeconds, MaxRetries: task.MaxRetries, RetryInterval: task.RetryInterval,
	}
	if in.Name != nil {
		tmp.Name = *in.Name
	}
	if in.Command != nil {
		tmp.Command = *in.Command
	}
	if in.CronExpression != nil {
		tmp.CronExpression = *in.CronExpression
	}
	if in.TimeoutSeconds != nil {
		tmp.TimeoutSeconds = *in.TimeoutSeconds
	}
	if in.MaxRetries != nil {
		tmp.MaxRetries = *in.MaxRetries
	}
	if in.RetryInterval != nil {
		tmp.RetryInterval = *in.RetryInterval
	}
	if err := validateTaskInput(tmp); err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}
	if in.Name != nil {
		updates["name"] = *in.Name
	}
	if in.Command != nil {
		updates["command"] = *in.Command
	}
	if in.CronExpression != nil {
		updates["cron_expression"] = *in.CronExpression
	}
	if in.TimeoutSeconds != nil {
		updates["timeout_seconds"] = *in.TimeoutSeconds
	}
	if in.MaxRetries != nil {
		updates["max_retries"] = *in.MaxRetries
	}
	if in.RetryInterval != nil {
		updates["retry_interval"] = *in.RetryInterval
	}
	enabledChanged := false
	if in.Enabled != nil {
		updates["enabled"] = *in.Enabled
		enabledChanged = true
	}
	if len(updates) > 0 {
		if err := database.DB.Model(task).Updates(updates).Error; err != nil {
			return nil, fmt.Errorf("更新失败: %w", err)
		}
	}
	if err := database.DB.First(task, id).Error; err != nil {
		return nil, fmt.Errorf("更新后读取任务失败: %w", err)
	}

	// 重新注册调度(enabled 变化或 cron 变化都重建)
	if enabledChanged || in.CronExpression != nil {
		_ = GetScheduler().Add(task)
	}
	return task, nil
}

// Delete 删除任务并移除调度。
func (s *TaskService) Delete(id uint) error {
	GetScheduler().Remove(id)
	return database.DB.Delete(&model.Task{}, id).Error
}

// SetEnabled 启用/禁用并联动调度。
func (s *TaskService) SetEnabled(id uint, enabled bool) (*model.Task, error) {
	task, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	task.Enabled = enabled
	if err := database.DB.Model(task).Update("enabled", enabled).Error; err != nil {
		return nil, err
	}
	if enabled {
		if err := GetScheduler().Add(task); err != nil {
			return nil, err
		}
	} else {
		GetScheduler().Remove(id)
	}
	return task, nil
}

// Run 手动触发任务。
func (s *TaskService) Run(id uint) error {
	task, err := s.Get(id)
	if err != nil {
		return err
	}
	if task.Status == model.TaskStatusRunning {
		return fmt.Errorf("任务正在运行中")
	}
	return GetScheduler().RunNow(id)
}

// Stop 停止运行中的任务。
func (s *TaskService) Stop(id uint) error {
	task, err := s.Get(id)
	if err != nil {
		return err
	}
	if !GetExecutor().ManualStop(id) {
		return fmt.Errorf("任务未在运行")
	}
	// 立即把状态标记为 idle(执行器完成块会再结算一次最终状态)
	now := time.Now()
	database.DB.Model(task).Updates(map[string]interface{}{
		"status": model.TaskStatusIdle,
		"last_run_status": model.RunStatusAborted,
		"last_run_at": &now,
	})
	return nil
}

// validateTaskInput 校验任务输入合法性。
func validateTaskInput(in CreateInput) error {
	if in.Name == "" {
		return fmt.Errorf("任务名称不能为空")
	}
	if in.Command == "" {
		return fmt.Errorf("命令不能为空")
	}
	// command 必须能解析(脚本须存在且位于脚本目录内)
	if _, err := ParseCommand(in.Command); err != nil {
		return fmt.Errorf("命令校验失败: %w", err)
	}
	if in.CronExpression == "" {
		return fmt.Errorf("Cron 表达式不能为空")
	}
	if err := cronutil.Validate(in.CronExpression); err != nil {
		return err
	}
	if in.TimeoutSeconds < 0 {
		return fmt.Errorf("超时秒数不能为负")
	}
	if in.MaxRetries < 0 {
		return fmt.Errorf("重试次数不能为负")
	}
	if in.RetryInterval < 0 {
		return fmt.Errorf("重试间隔不能为负")
	}
	return nil
}
