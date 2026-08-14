// Package cli 提供运维子命令(同一二进制):
//
//	taskpanel account-reset --user admin [--password 新密码] [--disable-2fa]
//	taskpanel log-clean --days 7
//	taskpanel task-trigger --id 1 | --name 任务名
//
// 用于 2FA 找回、密码重置、日志清理与手动触发等运维场景。
package cli

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"taskpanel/config"
	"taskpanel/database"
	"taskpanel/model"
	"taskpanel/service"

	"golang.org/x/crypto/bcrypt"
)

// Run 执行 CLI 子命令;返回错误时由 main 打印并退出。
func Run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}

	// 加载配置并初始化数据库
	cfgPath := os.Getenv("CONFIG_PATH")
	if cfgPath == "" {
		cfgPath = "config.yaml"
	}
	if _, err := config.Load(cfgPath); err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}
	if err := database.Init(config.C.Database.Path); err != nil {
		return fmt.Errorf("初始化数据库失败: %w", err)
	}

	switch args[0] {
	case "account-reset":
		return cmdAccountReset(args[1:])
	case "log-clean":
		return cmdLogClean(args[1:])
	case "task-trigger":
		return cmdTaskTrigger(args[1:])
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		return fmt.Errorf("未知命令: %s", args[0])
	}
}

func printUsage() {
	fmt.Print(`task-panel 运维工具

用法:
  taskpanel account-reset --user admin [--password 新密码] [--disable-2fa]
  taskpanel log-clean --days 7
  taskpanel task-trigger --id 1
  taskpanel task-trigger --name 任务名
  taskpanel help
`)
}

// cmdAccountReset 重置用户密码 / 关闭 2FA(2FA 找回的兜底通道)。
func cmdAccountReset(args []string) error {
	fs := flag.NewFlagSet("account-reset", flag.ExitOnError)
	user := fs.String("user", "admin", "用户名")
	password := fs.String("password", "", "新密码(留空则不修改)")
	disable2fa := fs.Bool("disable-2fa", false, "关闭该用户的 2FA")
	_ = fs.Parse(args)

	if *password == "" && !*disable2fa {
		return fmt.Errorf("至少指定 --password 或 --disable-2fa 之一")
	}

	var u model.User
	if err := database.DB.Where("username = ?", *user).First(&u).Error; err != nil {
		return fmt.Errorf("用户 %q 不存在", *user)
	}

	if *password != "" {
		if len(*password) < 6 || len(*password) > 128 {
			return fmt.Errorf("密码长度需 6-128 位")
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(*password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		if err := database.DB.Model(&u).Update("password", string(hash)).Error; err != nil {
			return err
		}
		fmt.Printf("✅ 已重置用户 %q 的密码\n", *user)
	}

	if *disable2fa {
		if err := database.DB.Model(&u).Update("totp_secret", "").Error; err != nil {
			return err
		}
		fmt.Printf("✅ 已关闭用户 %q 的 2FA\n", *user)
	}
	return nil
}

// cmdLogClean 清理 days 天前的执行日志(数据库记录 + 日志文件)。
func cmdLogClean(args []string) error {
	fs := flag.NewFlagSet("log-clean", flag.ExitOnError)
	days := fs.Int("days", 7, "清理多少天前的日志")
	_ = fs.Parse(args)

	if *days < 1 {
		return fmt.Errorf("--days 需为正整数")
	}
	cutoff := time.Now().AddDate(0, 0, -*days)

	// 找出过期记录并删除对应日志文件
	var logs []model.TaskLog
	database.DB.Where("started_at < ?", cutoff).Find(&logs)
	deletedFiles := 0
	for _, l := range logs {
		if l.LogPath != "" {
			_ = os.Remove(l.LogPath)
			deletedFiles++
		}
	}

	res := database.DB.Where("started_at < ?", cutoff).Delete(&model.TaskLog{})
	if res.Error != nil {
		return res.Error
	}
	fmt.Printf("✅ 已清理 %d 条日志记录,删除 %d 个日志文件(早于 %s)\n",
		res.RowsAffected, deletedFiles, cutoff.Format("2006-01-02"))
	return nil
}

// cmdTaskTrigger 手动触发任务。
func cmdTaskTrigger(args []string) error {
	fs := flag.NewFlagSet("task-trigger", flag.ExitOnError)
	id := fs.Uint("id", 0, "任务 ID")
	name := fs.String("name", "", "任务名称")
	_ = fs.Parse(args)

	var task model.Task
	if *id > 0 {
		if err := database.DB.First(&task, *id).Error; err != nil {
			return fmt.Errorf("任务 %d 不存在", *id)
		}
	} else if *name != "" {
		if err := database.DB.Where("name = ?", strings.TrimSpace(*name)).First(&task).Error; err != nil {
			return fmt.Errorf("任务 %q 不存在", *name)
		}
	} else {
		return fmt.Errorf("请指定 --id 或 --name")
	}

	if err := service.GetScheduler().RunNow(task.ID); err != nil {
		return fmt.Errorf("触发失败: %w", err)
	}
	fmt.Printf("✅ 已触发任务 %q (id=%d)\n", task.Name, task.ID)
	return nil
}
