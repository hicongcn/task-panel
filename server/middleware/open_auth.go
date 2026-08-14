package middleware

import (
	"net/http"
	"strings"

	"taskpanel/pkg/response"

	"github.com/gin-gonic/gin"
)

// OpenAuth 校验 Open API 令牌(Authorization: Bearer <token>)。
// 仅接受 token_type = open 的令牌;解析失败/过期/类型不符一律 401。
func OpenAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "缺少 Bearer 令牌"})
			return
		}
		claims, err := ParseToken(strings.TrimPrefix(auth, "Bearer "))
		if err != nil || claims.TokenType != "open" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "令牌无效或已过期"})
			return
		}
		c.Set("open_claims", claims)
		c.Next()
	}
}

// RequireScope 校验 Open API 令牌是否包含指定 scope,否则 403。
func RequireScope(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		v, ok := c.Get("open_claims")
		claims, ok2 := v.(*Claims)
		if !ok || !ok2 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "缺少令牌信息"})
			return
		}
		for _, s := range claims.Scopes {
			if s == scope {
				c.Next()
				return
			}
		}
		response.Forbidden(c, "无权限:需要 scope "+scope)
	}
}
