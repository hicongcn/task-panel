package service

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"taskpanel/database"
	"taskpanel/model"
	"taskpanel/pkg/validator"
)

// EnvService 负责环境变量 CRUD 与任务执行时的环境注入。
type EnvService struct{}

func NewEnvService() *EnvService { return &EnvService{} }

// MaskValue 把变量值脱敏后返回。
func MaskValue(value string) string {
	if value == "" {
		return ""
	}
	r := []rune(value)
	n := utf8.RuneCountInString(value)
	if n <= 2 {
		return strings.Repeat("*", n)
	}
	return string(r[0]) + strings.Repeat("*", n-2) + string(r[n-1])
}

// List 返回全部环境变量,值已脱敏。
func (s *EnvService) List(keyword, group string) []map[string]interface{} {
	q := database.DB.Model(&model.EnvVar{})
	if keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where("name LIKE ? OR remark LIKE ?", like, like)
	}
	if group != "" {
		q = q.Where("env_group = ?", group)
	}
	var envs []model.EnvVar
	q.Order("sort_order DESC, created_at ASC").Find(&envs)

	out := make([]map[string]interface{}, len(envs))
	for i, e := range envs {
		out[i] = ginEnvDict(e, true)
	}
	return out
}

// Groups 返回所有出现过的分组。
func (s *EnvService) Groups() []string {
	var groups []string
	database.DB.Model(&model.EnvVar{}).
		Where("env_group != ''").Distinct("env_group").Order("env_group ASC").
		Pluck("env_group", &groups)
	return groups
}

// Create 新建环境变量。
func (s *EnvService) Create(name, value, group, remark string, enabled bool) (*model.EnvVar, error) {
	if !validator.ValidateEnvName(name) {
		return nil, fmt.Errorf("变量名只能由字母、数字和下划线组成,且不能以数字开头")
	}
	env := &model.EnvVar{
		Name: name, Value: value, Group: group, Remark: remark, Enabled: enabled,
	}
	if err := database.DB.Create(env).Error; err != nil {
		return nil, fmt.Errorf("创建失败: %w", err)
	}
	return env, nil
}

// Update 更新环境变量。value 为空表示不修改原值。
func (s *EnvService) Update(id uint, name, value, group, remark string, enabled *bool) (*model.EnvVar, error) {
	var env model.EnvVar
	if err := database.DB.First(&env, id).Error; err != nil {
		return nil, fmt.Errorf("环境变量不存在")
	}
	if name != "" && name != env.Name && !validator.ValidateEnvName(name) {
		return nil, fmt.Errorf("变量名不合法")
	}
	updates := map[string]interface{}{
		"env_group": group,
		"remark":    remark,
	}
	if name != "" {
		updates["name"] = name
	}
	if value != "" {
		updates["value"] = value
	}
	if enabled != nil {
		updates["enabled"] = *enabled
	}
	if err := database.DB.Model(&env).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("更新失败: %w", err)
	}
	if err := database.DB.First(&env, id).Error; err != nil {
		return nil, fmt.Errorf("更新后读取失败: %w", err)
	}
	return &env, nil
}

// Delete 删除环境变量。
func (s *EnvService) Delete(id uint) error {
	return database.DB.Delete(&model.EnvVar{}, id).Error
}

// BatchDelete 批量删除。
func (s *EnvService) BatchDelete(ids []uint) (int64, error) {
	r := database.DB.Where("id IN ?", ids).Delete(&model.EnvVar{})
	return r.RowsAffected, r.Error
}

// BuildTaskEnv 返回所有已启用环境变量,供任务执行注入子进程。
func (s *EnvService) BuildTaskEnv() map[string]string {
	var envs []model.EnvVar
	database.DB.Where("enabled = ?", true).Order("sort_order DESC, created_at ASC").Find(&envs)
	out := make(map[string]string, len(envs))
	for _, e := range envs {
		out[e.Name] = e.Value
	}
	return out
}

func ginEnvDict(e model.EnvVar, masked bool) map[string]interface{} {
	d := map[string]interface{}{
		"id":         e.ID,
		"name":       e.Name,
		"group":      e.Group,
		"remark":     e.Remark,
		"enabled":    e.Enabled,
		"sort_order": e.SortOrder,
		"created_at": e.CreatedAt,
	}
	if masked {
		d["value_masked"] = MaskValue(e.Value)
	} else {
		d["value"] = e.Value
	}
	return d
}

// GinEnvDict 导出版,供 handler 复用。
func GinEnvDict(e model.EnvVar, masked bool) map[string]interface{} {
	return ginEnvDict(e, masked)
}
