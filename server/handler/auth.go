package handler

import (
	"time"

	"taskpanel/middleware"
	"taskpanel/pkg/response"
	"taskpanel/service"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	svc         *service.AuthService
	loginLimiter gin.HandlerFunc
}

func NewAuthHandler() *AuthHandler {
	return &AuthHandler{
		svc:          service.NewAuthService(),
		loginLimiter: middleware.RateLimit(5, time.Minute),
	}
}

// CheckInit GET /auth/check-init
func (h *AuthHandler) CheckInit(c *gin.Context) {
	response.Success(c, gin.H{"need_init": h.svc.NeedInit()})
}

// Init POST /auth/init
func (h *AuthHandler) Init(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	user, err := h.svc.InitAdmin(req.Username, req.Password)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Created(c, gin.H{"message": "初始化成功", "user": gin.H{
		"id": user.ID, "username": user.Username,
	}})
}

// Login POST /auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	ip := middleware.ResolveClientIP(c)
	result, err := h.svc.Login(req.Username, req.Password, ip)
	if err != nil {
		switch err {
		case service.ErrAccountLocked:
			response.TooManyRequests(c, err.Error())
		case service.ErrAccountDisabled:
			response.Forbidden(c, err.Error())
		case service.ErrUserNotFound, service.ErrInvalidPassword:
			response.Unauthorized(c, "用户名或密码错误")
		default:
			response.InternalError(c, err.Error())
		}
		return
	}

	response.Success(c, gin.H{
		"message":     "登录成功",
		"access_token": result.Token,
		"expires_at":  result.ExpiresAt.Format(time.RFC3339),
		"username":    result.Username,
	})
}

// Logout POST /auth/logout
func (h *AuthHandler) Logout(c *gin.Context) {
	jti, _ := c.Get("jti")
	if jti != nil {
		middleware.BlockToken(jti.(string))
	}
	response.Success(c, gin.H{"message": "已退出登录"})
}

// GetUser GET /auth/user
func (h *AuthHandler) GetUser(c *gin.Context) {
	username, _ := c.Get("username")
	response.Success(c, gin.H{"username": username})
}

func (h *AuthHandler) RegisterRoutes(r *gin.RouterGroup) {
	auth := r.Group("/auth")
	{
		auth.GET("/check-init", h.CheckInit)
		auth.POST("/init", h.Init)
		auth.POST("/login", h.loginLimiter, h.Login)
		auth.POST("/logout", middleware.JWTAuth(), h.Logout)
		auth.GET("/user", middleware.JWTAuth(), h.GetUser)
	}
}
