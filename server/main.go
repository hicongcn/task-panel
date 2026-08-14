package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"taskpanel/config"
	"taskpanel/database"
	"taskpanel/router"
	"taskpanel/service"

	"github.com/gin-gonic/gin"
)

func main() {
	cfgPath := resolveConfigPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	if err := database.Init(cfg.Database.Path); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}

	// 启动调度器并加载已启用任务
	if err := service.GetScheduler().LoadEnabled(); err != nil {
		log.Printf("warn: 加载已启用任务失败: %v", err)
	}
	defer service.GetScheduler().Stop()

	// 启动定时备份调度(根据运行期设置)
	service.GetBackupService().InitScheduledBackup()

	// 启动系统监控采样
	service.GetSysMonitor().Start()

	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()
	engine.Use(ginAccessLogger(), gin.Recovery())
	router.Setup(engine)
	setupStaticFrontend(engine, cfg.Server.WebDir)

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	srv := &http.Server{Addr: addr, Handler: engine}

	go func() {
		log.Printf("task-panel 已启动: http://0.0.0.0:%d", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	log.Println("正在关闭...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
	log.Println("已退出")
}

// ginAccessLogger 输出访问日志,并脱敏 query 中的敏感参数(token/ticket),
// 避免 JWT 或下载/实时日志票据被写入日志文件。
func ginAccessLogger() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		path := param.Path
		if param.Request != nil && param.Request.URL != nil {
			if u, err := url.ParseRequestURI(param.Request.URL.RequestURI()); err == nil {
				q := u.Query()
				dirty := false
				for _, k := range []string{"token", "ticket"} {
					if q.Has(k) {
						q.Set(k, "REDACTED")
						dirty = true
					}
				}
				if dirty {
					u.RawQuery = q.Encode()
					path = u.RequestURI()
				}
			}
		}
		return fmt.Sprintf("[GIN] %v | %3d | %13v | %15s | %-7s %q\n%s",
			param.TimeStamp.Format("2006/01/02 - 15:04:05"),
			param.StatusCode,
			param.Latency,
			param.ClientIP,
			param.Method,
			path,
			param.ErrorMessage,
		)
	})
}

// resolveConfigPath 按优先级查找 config.yaml:环境变量 > 可执行目录 > 当前目录。
func resolveConfigPath() string {
	if v := os.Getenv("CONFIG_PATH"); v != "" {
		return v
	}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		for _, name := range []string{"config.yaml", "../config.yaml"} {
			p := filepath.Join(exeDir, name)
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	if _, err := os.Stat("config.yaml"); err == nil {
		return "config.yaml"
	}
	// 兜底:用 exe 目录下的 config.yaml(即使不存在,Load 会报错给出明确信息)
	return "config.yaml"
}

// setupStaticFrontend 让 Go 二进制直接托管前端静态资源(无需 nginx)。
func setupStaticFrontend(engine *gin.Engine, webDir string) {
	if strings.TrimSpace(webDir) == "" {
		webDir = autoDetectWebDir()
		if webDir == "" {
			return
		}
	}
	abs, err := filepath.Abs(webDir)
	if err != nil {
		return
	}
	index := filepath.Join(abs, "index.html")
	if _, err := os.Stat(index); err != nil {
		return
	}
	engine.StaticFile("/", index)
	for _, sub := range []string{"assets"} {
		d := filepath.Join(abs, sub)
		if info, err := os.Stat(d); err == nil && info.IsDir() {
			engine.Static("/"+sub, d)
		}
	}
	engine.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(404, gin.H{"code": 404, "message": "route not found"})
			return
		}
		c.File(index)
	})
	log.Printf("前端静态目录已挂载: %s", abs)
}

func autoDetectWebDir() string {
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		for _, d := range []string{filepath.Join(exeDir, "web"), filepath.Join(exeDir, "dist")} {
			if _, err := os.Stat(filepath.Join(d, "index.html")); err == nil {
				return d
			}
		}
	}
	return ""
}
