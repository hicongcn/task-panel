// Package database 负责 SQLite 初始化、自动迁移与连接池管理。
package database

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"taskpanel/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB 是全局数据库连接。
var DB *gorm.DB

// newGormLogger 构造 GORM 日志器:Warn 级别,忽略 ErrRecordNotFound 噪音
// (如登录锁查询"无记录"是正常分支,不应作为错误刷屏)。
func newGormLogger() logger.Interface {
	return logger.New(log.New(os.Stdout, "\r\n", log.LstdFlags), logger.Config{
		SlowThreshold:             time.Second,
		LogLevel:                  logger.Warn,
		IgnoreRecordNotFoundError: true,
		Colorful:                  false,
	})
}

// Init 打开(必要时创建)SQLite 数据库并执行自动迁移。
func Init(path string) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger: newGormLogger(),
	})
	if err != nil {
		return err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxOpenConns(1)               // SQLite 单写者,限制并发连接避免锁竞争
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if err := db.AutoMigrate(
		&model.User{},
		&model.Task{},
		&model.EnvVar{},
		&model.TaskLog{},
		&model.TokenBlock{},
		&model.LoginAttempt{},
		&model.AuditLog{},
	); err != nil {
		return err
	}

	DB = db
	log.Printf("database ready: %s", path)
	return nil
}
