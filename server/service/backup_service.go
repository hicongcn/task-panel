package service

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"taskpanel/config"
	"taskpanel/database"
	"taskpanel/model"
	"taskpanel/pkg/backup"

	"github.com/robfig/cron/v3"
)

// BackupService 提供加密备份的创建、恢复、列表、删除与定时备份。
type BackupService struct {
	cronMU sync.Mutex
	cron   *cron.Cron
}

var defaultBackupService = &BackupService{}

func GetBackupService() *BackupService { return defaultBackupService }

// BackupInfo 备份文件信息。
type BackupInfo struct {
	Name      string    `json:"name"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
}

// backupDir 返回备份目录。
func (s *BackupService) backupDir() string {
	return config.C.Backup.Dir
}

// backupKey 返回 32 字节加密密钥。
func (s *BackupService) backupKey() []byte {
	return config.BackupKey(config.C.Data.Dir)
}

// Create 手动创建加密备份。
func (s *BackupService) Create() (*BackupInfo, error) {
	dir := s.backupDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("创建备份目录失败: %w", err)
	}

	name := fmt.Sprintf("taskpanel-backup-%s.backup", time.Now().Format("20060102-150405"))
	dest := filepath.Join(dir, name)

	sources := map[string]string{
		"taskpanel.db": config.C.Database.Path,
		"scripts":      config.C.Data.ScriptsDir,
	}

	if err := backup.CreateBackup(dest, s.backupKey(), sources); err != nil {
		return nil, fmt.Errorf("备份失败: %w", err)
	}

	fi, err := os.Stat(dest)
	if err != nil {
		return nil, err
	}
	return &BackupInfo{
		Name: name, Size: fi.Size(), CreatedAt: fi.ModTime(),
	}, nil
}

// List 返回备份文件列表(按时间降序)。
func (s *BackupService) List() []BackupInfo {
	dir := s.backupDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []BackupInfo
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".backup") {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, BackupInfo{
			Name: e.Name(), Size: fi.Size(), CreatedAt: fi.ModTime(),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out
}

// Delete 删除指定备份文件。
func (s *BackupService) Delete(name string) error {
	path := filepath.Join(s.backupDir(), name)
	if !strings.HasSuffix(name, ".backup") {
		return fmt.Errorf("非法文件名")
	}
	// 防止路径穿越
	if strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return fmt.Errorf("非法文件名")
	}
	return os.Remove(path)
}

// DownloadPath 返回备份文件的绝对路径(供 handler 直接 ServeFile)。
func (s *BackupService) DownloadPath(name string) (string, error) {
	if !strings.HasSuffix(name, ".backup") || strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return "", fmt.Errorf("非法文件名")
	}
	p := filepath.Join(s.backupDir(), name)
	if _, err := os.Stat(p); err != nil {
		return "", fmt.Errorf("文件不存在")
	}
	return p, nil
}

// Restore 从备份文件恢复。先自动备份当前状态,再停调度→替换→重载。
func (s *BackupService) Restore(filePath string) error {
	// 1. 解密解包到临时目录
	tmpDir, err := os.MkdirTemp("", "tp-restore-*")
	if err != nil {
		return fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := backup.ExtractBackup(filePath, s.backupKey(), tmpDir); err != nil {
		return fmt.Errorf("解密失败(密钥不匹配或文件损坏): %w", err)
	}

	// 2. 校验:临时目录必须有 taskpanel.db
	dbFile := filepath.Join(tmpDir, "taskpanel.db")
	if _, err := os.Stat(dbFile); err != nil {
		return fmt.Errorf("备份文件缺少数据库文件")
	}

	log.Println("备份恢复:开始恢复流程")

	// 3. 自动备份当前状态(安全网)
	info, _ := s.Create()
	if info != nil {
		log.Printf("备份恢复:已自动备份当前状态 -> %s", info.Name)
	}

	// 4. 停止调度器 + 执行器
	GetScheduler().Stop()
	GetExecutor().StopAll()
	log.Println("备份恢复:调度器与执行器已停止")

	// 5. 关闭数据库连接
	if sqlDB, err := database.DB.DB(); err == nil {
		sqlDB.Close()
		log.Println("备份恢复:数据库连接已关闭")
	}

	// 6. 替换数据库文件
	if err := copyFile(dbFile, config.C.Database.Path); err != nil {
		return fmt.Errorf("替换数据库失败: %w", err)
	}

	// 7. 替换脚本目录
	scriptsDir := config.C.Data.ScriptsDir
	_ = os.RemoveAll(scriptsDir)
	if err := copyDir(filepath.Join(tmpDir, "scripts"), scriptsDir); err != nil {
		return fmt.Errorf("恢复脚本目录失败: %w", err)
	}

	log.Println("备份恢复:文件已替换")

	// 8. 重新打开数据库
	if err := database.Init(config.C.Database.Path); err != nil {
		return fmt.Errorf("重新打开数据库失败: %w", err)
	}
	log.Println("备份恢复:数据库已重新打开")

	// 9. 重新加载调度器
	if err := GetScheduler().Reload(); err != nil {
		log.Printf("warn: 重新加载调度器失败: %v", err)
	}
	log.Println("备份恢复:完成")
	return nil
}

// ---- 定时备份 ----

// setting keys
const (
	settingBackupSchedule = "backup_schedule_enabled"
	settingBackupCron     = "backup_schedule_cron"
	settingBackupKeep     = "backup_keep"
)

// InitScheduledBackup 启动时根据当前 setting 注册定时备份。
func (s *BackupService) InitScheduledBackup() {
	s.cronMU.Lock()
	defer s.cronMU.Unlock()

	s.cron = cron.New(cron.WithParser(cron.NewParser(cron.Minute|cron.Hour|cron.Dom|cron.Month|cron.Dow)))
	s.cron.Start()
	s.updateSchedule()
}

// UpdateScheduledBackup 在 setting 变更后调用,重新注册定时任务。
func (s *BackupService) UpdateScheduledBackup() {
	s.cronMU.Lock()
	defer s.cronMU.Unlock()
	// 停掉旧 cron,重新创建
	if s.cron != nil {
		s.cron.Stop()
	}
	s.cron = cron.New(cron.WithParser(cron.NewParser(cron.Minute|cron.Hour|cron.Dom|cron.Month|cron.Dow)))
	s.cron.Start()
	s.updateSchedule()
}

func (s *BackupService) updateSchedule() {
	enabled := getSetting(settingBackupSchedule, "false") == "true"
	if !enabled {
		return
	}
	cronExpr := getSetting(settingBackupCron, "0 3 * * *")
	keep, _ := strconv.Atoi(getSetting(settingBackupKeep, "10"))
	if keep < 1 {
		keep = 1
	}
	if _, err := s.cron.AddFunc(cronExpr, func() {
		s.runScheduledBackup(keep)
	}); err != nil {
		log.Printf("warn: 注册定时备份失败: %v", err)
	} else {
		log.Printf("定时备份已注册: %s (保留 %d 份)", cronExpr, keep)
	}
}

func (s *BackupService) runScheduledBackup(keep int) {
	info, err := s.Create()
	if err != nil {
		log.Printf("定时备份失败: %v", err)
		return
	}
	log.Printf("定时备份完成: %s", info.Name)

	// 清理旧备份
	all := s.List()
	if len(all) > keep {
		for _, b := range all[keep:] {
			_ = s.Delete(b.Name)
			log.Printf("定时备份:清理旧备份 %s", b.Name)
		}
	}
}

// ---- 工具 ----

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		srcPath := filepath.Join(src, e.Name())
		dstPath := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func getSetting(key, defaultVal string) string {
	var s model.Setting
	if err := database.DB.Where("key = ?", key).First(&s).Error; err != nil {
		return defaultVal
	}
	return s.Value
}

func setSetting(key, value string) error {
	return database.DB.Save(&model.Setting{Key: key, Value: value}).Error
}

// SettingService 提供设置读写(供 handler 使用)。
type SettingService struct{}

func NewSettingService() *SettingService { return &SettingService{} }

func (s *SettingService) Get(key, defaultVal string) string {
	return getSetting(key, defaultVal)
}

func (s *SettingService) Set(key, value string) error {
	return setSetting(key, value)
}

func (s *SettingService) GetAll() map[string]string {
	var rows []model.Setting
	database.DB.Find(&rows)
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		out[r.Key] = r.Value
	}
	return out
}