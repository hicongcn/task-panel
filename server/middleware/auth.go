// Package middleware 提供认证、CORS、限流、IP 解析等 HTTP 中间件。
package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	"taskpanel/config"
	"taskpanel/database"
	"taskpanel/model"
	"taskpanel/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// Claims 是 JWT 载荷。
type Claims struct {
	Username  string   `json:"username"`
	TokenType string   `json:"token_type"` // access / open
	Scopes    []string `json:"scopes,omitempty"` // 仅 open 令牌携带
	jwt.RegisteredClaims
}

// GenerateToken 签发访问令牌,返回 token 字符串与到期时间。
func GenerateToken(username string, ttl time.Duration) (string, time.Time, error) {
	return generateToken(username, "access", nil, ttl)
}

// GenerateOpenToken 签发 Open API 令牌(带权限范围)。
func GenerateOpenToken(appName string, scopes []string, ttl time.Duration) (string, time.Time, error) {
	return generateToken(appName, "open", scopes, ttl)
}

func generateToken(username, tokenType string, scopes []string, ttl time.Duration) (string, time.Time, error) {
	expiresAt := time.Now().Add(ttl)
	claims := Claims{
		Username:  username,
		TokenType: tokenType,
		Scopes:    scopes,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        newJTI(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	str, err := token.SignedString([]byte(config.C.JWT.Secret))
	if err != nil {
		return "", time.Time{}, err
	}
	return str, expiresAt, nil
}

// ParseToken 解析并校验令牌,返回载荷。
func ParseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		return []byte(config.C.JWT.Secret), nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, jwt.ErrSignatureInvalid
}

// IsTokenBlocked 判断 jti 是否已被吊销(登出/改密)。
func IsTokenBlocked(jti string) bool {
	if jti == "" {
		return false
	}
	var count int64
	database.DB.Model(&model.TokenBlock{}).Where("jti = ?", jti).Count(&count)
	return count > 0
}

// BlockToken 将 jti 加入黑名单,24 小时后过期可清理。
func BlockToken(jti string) {
	if jti == "" {
		return
	}
	database.DB.Create(&model.TokenBlock{
		JTI:       jti,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	})
}

// JWTAuth 校验 Authorization: Bearer <token>,并把 username/jti 注入上下文。
func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := ExtractBearerToken(c.GetHeader("Authorization"))
		if tokenStr == "" {
			response.Unauthorized(c, "缺少授权令牌")
			c.Abort()
			return
		}

		claims, err := ParseToken(tokenStr)
		if err != nil {
			response.Unauthorized(c, "令牌无效或已过期")
			c.Abort()
			return
		}
		if claims.TokenType != "access" {
			response.Unauthorized(c, "令牌类型错误")
			c.Abort()
			return
		}
		if IsTokenBlocked(claims.ID) {
			response.Unauthorized(c, "令牌已被吊销")
			c.Abort()
			return
		}

		c.Set("username", claims.Username)
		c.Set("jti", claims.ID)
		c.Next()
	}
}

// ExtractBearerToken 从 Authorization 头提取 Bearer token。
func ExtractBearerToken(header string) string {
	if !strings.HasPrefix(header, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
}

func newJTI() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}
