// Package model 定义 GORM 数据模型。
package model

import "time"

// User 管理员账号。
type User struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"size:64;uniqueIndex" json:"username"`
	Password     string    `gorm:"size:255" json:"-"`
	Enabled      bool      `gorm:"default:true" json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Task 定时任务。
type Task struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	Name            string     `gorm:"size:128;index" json:"name"`
	Command         string     `gorm:"size:1024" json:"command"`
	CronExpression  string     `gorm:"size:128" json:"cron_expression"`
	Enabled         bool       `json:"enabled"`
	TimeoutSeconds  int        `json:"timeout_seconds"`  // 0 表示不限制
	MaxRetries      int        `json:"max_retries"`
	RetryInterval   int        `json:"retry_interval"` // 秒
	Status          string     `gorm:"size:16;index" json:"status"` // idle/running
	LastRunAt       *time.Time `json:"last_run_at"`
	LastRunStatus   string     `gorm:"size:16" json:"last_run_status"` // success/failed/aborted/none
	LastRunDuration float64    `json:"last_run_duration"`
	PID             *int       `json:"pid"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// TaskStatus 常量。
const (
	TaskStatusIdle    = "idle"
	TaskStatusRunning = "running"
)

// TaskRunStatus 常量(任务最近一次执行结果)。
const (
	RunStatusNone    = "none"
	RunStatusSuccess = "success"
	RunStatusFailed  = "failed"
	RunStatusAborted = "aborted"
)

// EnvVar 环境变量。
type EnvVar struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:128;index" json:"name"`
	Value     string    `gorm:"size:8192" json:"-"`
	Group     string    `gorm:"column:env_group;size:64;index" json:"group"`
	Remark    string    `gorm:"size:255" json:"remark"`
	Enabled   bool      `json:"enabled"`
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TaskLog 任务执行日志。
type TaskLog struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	TaskID    uint       `gorm:"index" json:"task_id"`
	TaskName  string     `gorm:"size:128" json:"task_name"`
	Status    string     `gorm:"size:16;index" json:"status"`
	Content   string     `gorm:"type:text" json:"-"`
	LogPath   string     `gorm:"size:255" json:"-"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at"`
	Duration  float64    `json:"duration"`
}

// TaskLogStatus 常量。
const (
	LogStatusRunning = "running"
	LogStatusSuccess = "success"
	LogStatusFailed  = "failed"
	LogStatusAborted = "aborted"
)

// TokenBlock 已吊销令牌黑名单(jti 级别)。
type TokenBlock struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	JTI       string    `gorm:"size:64;uniqueIndex" json:"jti"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// LoginAttempt 登录失败计数(用于锁定)。
type LoginAttempt struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	IP        string     `gorm:"size:64;index:idx_ip_user" json:"ip"`
	Username  string     `gorm:"size:64;index:idx_ip_user" json:"username"`
	Count     int        `json:"count"`
	LockedAt  *time.Time `json:"locked_at"`
	ExpiresAt time.Time  `json:"expires_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}
