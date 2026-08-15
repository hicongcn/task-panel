package handler

import (
	"fmt"
	"time"

	"taskpanel/middleware"
	"taskpanel/model"
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
	service.NewAuditService().Record(req.Username, model.AuditActionInitAdmin, "auth", "初始化管理员", middleware.ResolveClientIP(c))
	response.Created(c, gin.H{"message": "初始化成功", "user": gin.H{
		"id": user.ID, "username": user.Username,
	}})
}

// Login POST /auth/login {username, password, totp_code?}
func (h *AuthHandler) Login(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
		TOTPCode string `json:"totp_code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	ip := middleware.ResolveClientIP(c)
	result, err := h.svc.Login(req.Username, req.Password, ip, req.TOTPCode)
	if err != nil {
		service.NewAuditService().Record(req.Username, model.AuditActionLoginFailed, "auth", "", ip)
		switch err {
		case service.ErrAccountLocked:
			service.GetNotifyService().NotifyEvent("登录异常告警",
				fmt.Sprintf("账号 %q 因多次失败已被锁定(IP: %s)", req.Username, ip))
			response.TooManyRequests(c, err.Error())
		case service.ErrAccountDisabled:
			response.Forbidden(c, err.Error())
		case service.ErrInvalidTOTP:
			response.Unauthorized(c, "动态验证码错误")
		case service.ErrUserNotFound, service.ErrInvalidPassword:
			response.Unauthorized(c, "用户名或密码错误")
		default:
			response.InternalError(c, err.Error())
		}
		return
	}
	service.NewAuditService().Record(result.Username, model.AuditActionLoginSuccess, "auth", "", ip)

	response.Success(c, gin.H{
		"message":      "登录成功",
		"access_token": result.Token,
		"expires_at":   result.ExpiresAt.Format(time.RFC3339),
		"username":     result.Username,
	})
}

// Logout POST /auth/logout
func (h *AuthHandler) Logout(c *gin.Context) {
	jti, _ := c.Get("jti")
	if jti != nil {
		middleware.BlockToken(jti.(string))
	}
	recordAudit(c, model.AuditActionLogout, "auth", "")
	response.Success(c, gin.H{"message": "已退出登录"})
}

// GetUser GET /auth/user
func (h *AuthHandler) GetUser(c *gin.Context) {
	username, _ := c.Get("username")
	response.Success(c, gin.H{"username": username})
}

// TOTPSetup GET /auth/totp/setup 生成绑定用的密钥与 otpauth URI。
func (h *AuthHandler) TOTPSetup(c *gin.Context) {
	username, _ := c.Get("username")
	secret, uri, err := h.svc.TOTPSetup(username.(string))
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, gin.H{"data": gin.H{"secret": secret, "uri": uri}})
}

// TOTPEnable POST /auth/totp/enable {secret, code} 验证后启用 2FA。
func (h *AuthHandler) TOTPEnable(c *gin.Context) {
	username, _ := c.Get("username")
	var req struct {
		Secret string `json:"secret" binding:"required"`
		Code   string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	if err := h.svc.TOTPEnable(username.(string), req.Secret, req.Code); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	recordAudit(c, model.AuditActionTOTPEnable, "auth", "")
	response.Success(c, gin.H{"message": "已启用双重认证"})
}

// TOTPDisable POST /auth/totp/disable {password} 校验密码后关闭 2FA。
func (h *AuthHandler) TOTPDisable(c *gin.Context) {
	username, _ := c.Get("username")
	var req struct {
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	if err := h.svc.TOTPDisable(username.(string), req.Password); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	recordAudit(c, model.AuditActionTOTPDisable, "auth", "")
	response.Success(c, gin.H{"message": "已关闭双重认证"})
}

// ChangePassword POST /auth/password {old_password, new_password} 修改密码并吊销当前令牌。
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	username, _ := c.Get("username")
	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	if err := h.svc.ChangePassword(username.(string), req.OldPassword, req.NewPassword); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	// 吊销当前令牌,强制重新登录
	if jti, ok := c.Get("jti"); ok {
		middleware.BlockToken(jti.(string))
	}
	recordAudit(c, model.AuditActionChangePassword, "auth", "")
	response.Success(c, gin.H{"message": "密码已修改,请重新登录"})
}

// TOTPStatus GET /auth/totp/status 返回是否已启用 2FA。
func (h *AuthHandler) TOTPStatus(c *gin.Context) {
	username, _ := c.Get("username")
	response.Success(c, gin.H{"data": gin.H{"enabled": h.svc.TOTPStatus(username.(string))}})
}

func (h *AuthHandler) RegisterRoutes(r *gin.RouterGroup) {
	auth := r.Group("/auth")
	{
		auth.GET("/check-init", h.CheckInit)
		auth.POST("/init", h.Init)
		auth.POST("/login", h.loginLimiter, h.Login)
		auth.POST("/logout", middleware.JWTAuth(), h.Logout)
		auth.GET("/user", middleware.JWTAuth(), h.GetUser)
		auth.POST("/password", middleware.JWTAuth(), h.ChangePassword)
		totpGroup := auth.Group("/totp", middleware.JWTAuth())
		{
			totpGroup.GET("/setup", h.TOTPSetup)
			totpGroup.POST("/enable", h.TOTPEnable)
			totpGroup.POST("/disable", h.TOTPDisable)
			totpGroup.GET("/status", h.TOTPStatus)
		}
	}
}
