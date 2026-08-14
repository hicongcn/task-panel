// Package router 注册全部 HTTP 路由。
package router

import (
	"taskpanel/handler"
	"taskpanel/middleware"

	"github.com/gin-gonic/gin"
)

func Setup(engine *gin.Engine) {
	engine.Use(middleware.SecurityHeaders())
	engine.Use(middleware.CORS())

	v1 := engine.Group("/api/v1")
	v1.Use(middleware.MaxBodySize(20 << 20)) // 限制 API 请求体 20MB,防内存耗尽

	handler.NewAuthHandler().RegisterRoutes(v1)
	handler.NewTaskHandler().RegisterRoutes(v1)
	handler.NewEnvHandler().RegisterRoutes(v1)
	handler.NewScriptHandler().RegisterRoutes(v1)
	handler.NewLogHandler().RegisterRoutes(v1)
	handler.NewSystemHandler().RegisterRoutes(v1)
	handler.NewAuditHandler().RegisterRoutes(v1)
	handler.NewNotifyHandler().RegisterRoutes(v1)

	// 版本(公开)
	engine.GET("/api/v1/version", func(c *gin.Context) {
		c.JSON(200, gin.H{"version": handler.Version, "api_version": "v1"})
	})

	engine.GET("/robots.txt", func(c *gin.Context) {
		c.Data(200, "text/plain; charset=utf-8", []byte("User-agent: *\nDisallow: /\n"))
	})
}
