package main

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"taskpanel/cli"
	"taskpanel/config"
	"taskpanel/database"
	"taskpanel/router"
	"taskpanel/service"
	"taskpanel/webembed"

	"github.com/gin-gonic/gin"
)

func main() {
	// 运维子命令(如 account-reset / log-clean / task-trigger)
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "account-reset", "log-clean", "task-trigger", "help", "-h", "--help":
			if err := cli.Run(os.Args[1:]); err != nil {
				fmt.Fprintln(os.Stderr, "错误:", err)
				os.Exit(1)
			}
			return
		}
	}

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
	// 无任何配置文件:返回空,由 config.Load 使用内置默认配置(单文件发布模式)
	return ""
}

// setupStaticFrontend 让 Go 二进制直接托管前端静态资源(无需 nginx)。
// 优先级:显式配置/自动探测的外部目录 > 内嵌前端(webembed,单文件分发)。
func setupStaticFrontend(engine *gin.Engine, webDir string) {
	if strings.TrimSpace(webDir) == "" {
		webDir = autoDetectWebDir()
	}
	if abs := resolveWebDir(webDir); abs != "" {
		mountFrontend(engine, abs)
		return
	}
	mountEmbeddedFrontend(engine)
}

// resolveWebDir 解析并校验外部前端目录(index.html 必须存在)。
func resolveWebDir(webDir string) string {
	if strings.TrimSpace(webDir) == "" {
		return ""
	}
	abs, err := filepath.Abs(webDir)
	if err != nil {
		return ""
	}
	if _, err := os.Stat(filepath.Join(abs, "index.html")); err != nil {
		return ""
	}
	return abs
}

func mountFrontend(engine *gin.Engine, abs string) {
	index := filepath.Join(abs, "index.html")
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

// mountEmbeddedFrontend 挂载内嵌的 web/dist(单文件发布,无需外部目录)。
// 注意:
//  1. 标准库 FileServer 会把 /index.html 请求 301 到 ./,
//     因此根路径用 FileServer 直接服务(自动 index.html),SPA 深路径由 NoRoute 手动回退;
//  2. gin 的 StaticFS 预检用 fs.Open(*filepath),而 *filepath 带前导斜杠,
//     embed.FS 拒绝以 / 开头的路径,必须用 trimFS 去掉前导斜杠(否则所有资源 404→回退 index.html 白屏)。
func mountEmbeddedFrontend(engine *gin.Engine) {
	sub, err := fs.Sub(webembed.Dist, "dist")
	if err != nil {
		log.Printf("warn: 内嵌前端不可用: %v", err)
		return
	}
	indexBytes, err := fs.ReadFile(sub, "index.html")
	if err != nil {
		log.Printf("warn: 内嵌前端缺少 index.html(占位模式,忽略)")
		return
	}
	fsys := http.FS(sub)
	engine.GET("/", gin.WrapH(http.FileServer(fsys)))
	// assets 作为 StaticFS 的根:gin 预检/FileServer 打开的是相对 assets 的路径
	// (如 index-xxx.js),若直接传 dist 根,会因路径层级不匹配全部 404→回退 index.html 白屏。
	if assetsSub, err := fs.Sub(sub, "assets"); err == nil {
		engine.StaticFS("/assets", trimFS{http.FS(assetsSub)})
	}
	engine.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(404, gin.H{"code": 404, "message": "route not found"})
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexBytes)
	})
	log.Printf("前端静态资源已内嵌挂载(webembed)")
}

// trimFS 兼容 gin StaticFS:其预检传入的 *filepath 参数以 / 开头,
// embed.FS 不接收前导斜杠路径,这里统一去掉。
type trimFS struct {
	inner http.FileSystem
}

func (t trimFS) Open(name string) (http.File, error) {
	return t.inner.Open(strings.TrimPrefix(name, "/"))
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
