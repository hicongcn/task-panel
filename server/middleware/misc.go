package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"taskpanel/config"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// RateLimiter 基于内存的每 IP 限流器。
type RateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	limit    int
	window   time.Duration
}

type visitor struct {
	count    int
	lastSeen time.Time
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{visitors: make(map[string]*visitor), limit: limit, window: window}
	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) cleanup() {
	for {
		time.Sleep(rl.window)
		rl.mu.Lock()
		for ip, v := range rl.visitors {
			if time.Since(v.lastSeen) > rl.window {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// Allow 返回该 key 是否仍在配额内。
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	v, ok := rl.visitors[key]
	if !ok {
		rl.visitors[key] = &visitor{count: 1, lastSeen: time.Now()}
		return true
	}
	if time.Since(v.lastSeen) > rl.window {
		v.count = 1
		v.lastSeen = time.Now()
		return true
	}
	v.count++
	v.lastSeen = time.Now()
	return v.count <= rl.limit
}

// RateLimit 返回按 IP 限流的中间件。
func RateLimit(limit int, window time.Duration) gin.HandlerFunc {
	limiter := NewRateLimiter(limit, window)
	return func(c *gin.Context) {
		if !limiter.Allow(ResolveClientIP(c)) {
			c.JSON(http.StatusTooManyRequests, gin.H{"code": 429, "message": "请求过于频繁,请稍后再试"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// CORS 按配置白名单放行 origin,默认拒绝 null / 空 origin。
func CORS() gin.HandlerFunc {
	allowed := []string{"http://localhost:5173", "http://localhost:5700"}
	if config.C != nil && len(config.C.CORS.Origins) > 0 {
		allowed = config.C.CORS.Origins
	}
	allowedSet := make(map[string]bool, len(allowed))
	for _, o := range allowed {
		allowedSet[o] = true
	}

	return cors.New(cors.Config{
		AllowOriginWithContextFunc: func(c *gin.Context, origin string) bool {
			if origin == "" || origin == "null" {
				return false
			}
			return allowedSet[origin]
		},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "Accept"},
		ExposeHeaders:    []string{"Content-Disposition", "Content-Length"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	})
}

// SecurityHeaders 设置通用安全响应头。
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		c.Next()
	}
}

// IPWhitelist 校验客户端真实 IP 是否在允许列表内。
// 列表为空时放行(默认不启用);支持单个 IP 或 CIDR(如 192.168.1.0/24)。
// 运行时读取 config.C,便于测试动态调整;IP 解析复用 ResolveClientIP
// (仅信任可信代理网段转发头,防伪造)。
func IPWhitelist() gin.HandlerFunc {
	return func(c *gin.Context) {
		if config.C == nil || len(config.C.Security.IPWhitelist) == 0 {
			c.Next()
			return
		}
		nets := parseIPWhitelist(config.C.Security.IPWhitelist)
		if len(nets) == 0 {
			c.Next()
			return
		}
		clientIP := net.ParseIP(ResolveClientIP(c))
		if clientIP == nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": 403, "message": "无法解析客户端 IP"})
			return
		}
		for _, n := range nets {
			if n.Contains(clientIP) {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": 403, "message": "IP 不在白名单内"})
	}
}

// parseIPWhitelist 解析配置项为网段列表(单个 IP 视为 /32 或 /128)。
func parseIPWhitelist(items []string) []*net.IPNet {
	var nets []*net.IPNet
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if strings.Contains(item, "/") {
			if _, n, err := net.ParseCIDR(item); err == nil {
				nets = append(nets, n)
			}
			continue
		}
		if ip := net.ParseIP(item); ip != nil {
			if ip.To4() != nil {
				if _, n, err := net.ParseCIDR(item + "/32"); err == nil {
					nets = append(nets, n)
				}
			} else {
				if _, n, err := net.ParseCIDR(item + "/128"); err == nil {
					nets = append(nets, n)
				}
			}
		}
	}
	return nets
}

// MaxBodySize 限制请求体大小,超出时读取会失败(ShouldBindJSON 报错 → 400)。
// 防止超大 JSON(如脚本内容)导致的内存耗尽 DoS。
func MaxBodySize(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}

// RequireAuth 仅供未来扩展角色体系时占位,当前等价于 JWTAuth。
func RequireAuth() gin.HandlerFunc {
	return JWTAuth()
}
