package service

import (
	"os"
	"path/filepath"
	"time"

	"taskpanel/config"
	"taskpanel/database"
	"taskpanel/model"
	"taskpanel/pkg/pathutil"
)

// MigrateService 批量导入/导出任务/脚本/环境变量(JSON)。
type MigrateService struct{}

func NewMigrateService() *MigrateService { return &MigrateService{} }

// TaskExport 导出的任务。
type TaskExport struct {
	Name           string   `json:"name"`
	Command        string   `json:"command"`
	CronExpression string   `json:"cron_expression"`
	Enabled        bool     `json:"enabled"`
	TimeoutSeconds int      `json:"timeout_seconds"`
	MaxRetries     int      `json:"max_retries"`
	RetryInterval  int      `json:"retry_interval"`
	Tags           []string `json:"tags"`
}

// ScriptExport 导出的脚本。
type ScriptExport struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// EnvExport 导出的环境变量(含明文值,注意妥善保管)。
type EnvExport struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	Group   string `json:"group"`
	Remark  string `json:"remark"`
	Enabled bool   `json:"enabled"`
}

// ExportData 导出数据结构(导入时同样按此结构解析)。
type ExportData struct {
	Version    string         `json:"version"`
	ExportedAt time.Time      `json:"exported_at"`
	Tasks      []TaskExport   `json:"tasks"`
	Scripts    []ScriptExport `json:"scripts"`
	Envs       []EnvExport    `json:"envs"`
}

// ImportResult 导入结果统计。
type ImportResult struct {
	TasksOK        int `json:"tasks_ok"`
	TasksSkipped   int `json:"tasks_skipped"`
	ScriptsOK      int `json:"scripts_ok"`
	ScriptsSkipped int `json:"scripts_skipped"`
	EnvsOK         int `json:"envs_ok"`
	EnvsSkipped    int `json:"envs_skipped"`
}

// Export 导出全部任务/脚本/环境变量。
func (s *MigrateService) Export() (*ExportData, error) {
	data := &ExportData{Version: "task-panel-migrate/v1", ExportedAt: time.Now()}

	var tasks []model.Task
	database.DB.Order("id ASC").Find(&tasks)
	for _, t := range tasks {
		data.Tasks = append(data.Tasks, TaskExport{
			Name: t.Name, Command: t.Command, CronExpression: t.CronExpression,
			Enabled: t.Enabled, TimeoutSeconds: t.TimeoutSeconds,
			MaxRetries: t.MaxRetries, RetryInterval: t.RetryInterval,
			Tags: t.Tags,
		})
	}

	base := config.C.Data.ScriptsDir
	_ = filepath.Walk(base, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(base, p)
		if err != nil {
			return nil
		}
		content, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		data.Scripts = append(data.Scripts, ScriptExport{
			Path: filepath.ToSlash(rel), Content: string(content),
		})
		return nil
	})

	var envs []model.EnvVar
	database.DB.Order("id ASC").Find(&envs)
	for _, e := range envs {
		data.Envs = append(data.Envs, EnvExport{
			Name: e.Name, Value: e.Value, Group: e.Group, Remark: e.Remark, Enabled: e.Enabled,
		})
	}
	return data, nil
}

// Import 导入任务/脚本/环境变量。
// 顺序:先脚本(任务命令校验要求脚本已存在)→ 任务 → 环境变量。
// 任务与环境变量同名跳过;脚本同名覆盖(仅做路径安全校验,信任导入来源)。
func (s *MigrateService) Import(data *ExportData) (*ImportResult, error) {
	res := &ImportResult{}

	base := config.C.Data.ScriptsDir
	for _, sc := range data.Scripts {
		if err := pathutil.ValidateRelativePath(sc.Path); err != nil {
			res.ScriptsSkipped++
			continue
		}
		full, err := pathutil.SafeJoin(base, sc.Path, false)
		if err != nil {
			res.ScriptsSkipped++
			continue
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			res.ScriptsSkipped++
			continue
		}
		if err := os.WriteFile(full, []byte(sc.Content), 0o644); err != nil {
			res.ScriptsSkipped++
			continue
		}
		res.ScriptsOK++
	}

	for _, t := range data.Tasks {
		var count int64
		database.DB.Model(&model.Task{}).Where("name = ?", t.Name).Count(&count)
		if count > 0 {
			res.TasksSkipped++
			continue
		}
		if _, err := s.createTask(t); err != nil {
			res.TasksSkipped++
			continue
		}
		res.TasksOK++
	}

	for _, e := range data.Envs {
		var count int64
		database.DB.Model(&model.EnvVar{}).Where("name = ?", e.Name).Count(&count)
		if count > 0 {
			res.EnvsSkipped++
			continue
		}
		if err := database.DB.Create(&model.EnvVar{
			Name: e.Name, Value: e.Value, Group: e.Group, Remark: e.Remark, Enabled: e.Enabled,
		}).Error; err != nil {
			res.EnvsSkipped++
			continue
		}
		res.EnvsOK++
	}
	return res, nil
}

func (s *MigrateService) createTask(t TaskExport) (*model.Task, error) {
	return NewTaskService().Create(CreateInput{
		Name: t.Name, Command: t.Command, CronExpression: t.CronExpression,
		Enabled: t.Enabled, TimeoutSeconds: t.TimeoutSeconds,
		MaxRetries: t.MaxRetries, RetryInterval: t.RetryInterval,
		Tags: t.Tags,
	})
}
