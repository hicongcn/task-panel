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

// AuditLog 审计日志(记录关键操作留痕)。
type AuditLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Username  string    `gorm:"size:64;index" json:"username"`
	Action    string    `gorm:"size:32;index" json:"action"`
	Resource  string    `gorm:"size:255" json:"resource"`
	Detail    string    `gorm:"type:text" json:"detail"`
	IP        string    `gorm:"size:64" json:"ip"`
	CreatedAt time.Time `json:"created_at"`
}

// AuditAction 常量。
const (
	AuditActionInitAdmin   = "init_admin"
	AuditActionLoginSuccess = "login_success"
	AuditActionLoginFailed  = "login_failed"
	AuditActionLogout       = "logout"

	AuditActionTaskCreate  = "task_create"
	AuditActionTaskUpdate  = "task_update"
	AuditActionTaskDelete  = "task_delete"
	AuditActionTaskRun     = "task_run"
	AuditActionTaskStop    = "task_stop"
	AuditActionTaskEnable  = "task_enable"
	AuditActionTaskDisable = "task_disable"

	AuditActionScriptSave   = "script_save"
	AuditActionScriptCreate = "script_create_dir"
	AuditActionScriptDelete = "script_delete"
	AuditActionScriptRename = "script_rename"
	AuditActionScriptUpload = "script_upload"
	AuditActionScriptRun    = "script_run"
	AuditActionScriptCode   = "script_run_code"

	AuditActionEnvCreate     = "env_create"
	AuditActionEnvUpdate     = "env_update"
	AuditActionEnvDelete     = "env_delete"
	AuditActionEnvBatchDel   = "env_batch_delete"

	AuditActionNotifyCreate = "notify_create"
	AuditActionNotifyUpdate = "notify_update"
	AuditActionNotifyDelete = "notify_delete"
	AuditActionNotifyToggle = "notify_toggle"

	AuditActionBackupCreate  = "backup_create"
	AuditActionBackupDelete  = "backup_delete"
	AuditActionBackupRestore = "backup_restore"
	AuditActionBackupSetting = "backup_setting"

	AuditActionDepInstall   = "dep_install"
	AuditActionDepUninstall = "dep_uninstall"

	AuditActionOpenAppCreate  = "open_app_create"
	AuditActionOpenAppUpdate  = "open_app_update"
	AuditActionOpenAppDelete  = "open_app_delete"
	AuditActionOpenAppReset   = "open_app_reset"
	AuditActionOpenAuthFail   = "open_auth_fail"
)

// OpenAPI 开放接口的 scope 常量。
const (
	ScopeTasksRead = "tasks:read"
	ScopeTasksRun  = "tasks:run"
	ScopeLogsRead  = "logs:read"
	ScopeEnvsRead  = "envs:read"
)

// OpenApp 开放平台应用(参考青龙 OpenAPI:client_id/secret + scopes)。
type OpenApp struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Name         string    `gorm:"size:64;uniqueIndex" json:"name"`
	ClientID     string    `gorm:"size:64;uniqueIndex" json:"client_id"`
	ClientSecret string    `gorm:"size:128" json:"-"` // 仅创建/重置时返回一次
	Scopes       string    `gorm:"type:text" json:"-"` // JSON 数组
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// NotifyChannel 通知渠道(webhook / telegram / bark / email)。
// Config 存 JSON 文本,字段因类型而异,由 service 层解析。
type NotifyChannel struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:128" json:"name"`
	Type      string    `gorm:"size:32;index" json:"type"`
	Enabled   bool      `json:"enabled"`
	Config    string    `gorm:"type:text" json:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NotifyType 常量。
const (
	NotifyTypeWebhook  = "webhook"
	NotifyTypeTelegram = "telegram"
	NotifyTypeBark     = "bark"
	NotifyTypeEmail    = "email"
)

// Setting 运行期配置(键值对),用于通知/定时备份等需要动态开关的场景。
type Setting struct {
	Key   string `gorm:"primaryKey;size:64" json:"key"`
	Value string `gorm:"type:text" json:"value"`
}
