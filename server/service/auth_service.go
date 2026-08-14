// Package service 实现核心业务逻辑,与 HTTP 层解耦。
package service

import (
	"errors"
	"strings"
	"time"

	"taskpanel/config"
	"taskpanel/database"
	"taskpanel/middleware"
	"taskpanel/model"
	"taskpanel/pkg/totp"
	"taskpanel/pkg/validator"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidUsername  = errors.New("用户名需 1-32 位,支持中文、字母、数字和下划线")
	ErrPasswordTooShort = errors.New("密码长度需 6-128 位")
	ErrAlreadyInit      = errors.New("系统已初始化")
	ErrUserNotFound     = errors.New("用户不存在")
	ErrInvalidPassword  = errors.New("用户名或密码错误")
	ErrAccountLocked    = errors.New("账号已锁定,请稍后再试")
	ErrAccountDisabled  = errors.New("账号已被禁用")
	ErrInvalidTOTP      = errors.New("动态验证码错误")
)

const (
	maxLoginAttempts = 5
	lockDuration     = 15 * time.Minute
)

// AuthService 封装管理员初始化与登录鉴权。
type AuthService struct{}

func NewAuthService() *AuthService { return &AuthService{} }

// NeedInit 判断是否需要初始化管理员(无任何用户时为 true)。
func (s *AuthService) NeedInit() bool {
	var count int64
	database.DB.Model(&model.User{}).Count(&count)
	return count == 0
}

// InitAdmin 创建首个管理员账号。
func (s *AuthService) InitAdmin(username, password string) (*model.User, error) {
	if !s.NeedInit() {
		return nil, ErrAlreadyInit
	}
	if !validator.ValidateUsername(username) {
		return nil, ErrInvalidUsername
	}
	if !validator.ValidatePassword(password) {
		return nil, ErrPasswordTooShort
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &model.User{Username: strings.TrimSpace(username), Password: string(hash)}
	if err := database.DB.Create(user).Error; err != nil {
		return nil, err
	}
	return user, nil
}

// LoginResult 登录成功后返回。
type LoginResult struct {
	Token      string
	ExpiresAt  time.Time
	Username   string
}

// Login 校验账密并签发令牌。已启用 2FA 的用户必须提供正确的 TOTP 码。
func (s *AuthService) Login(username, password, ip, totpCode string) (*LoginResult, error) {
	if locked, _ := checkLoginLock(ip, username); locked {
		return nil, ErrAccountLocked
	}

	var user model.User
	if err := database.DB.Where("username = ?", validator.SanitizeString(username)).First(&user).Error; err != nil {
		recordFailedLogin(ip, username)
		return nil, ErrUserNotFound
	}
	if !user.Enabled {
		return nil, ErrAccountDisabled
	}
	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)) != nil {
		recordFailedLogin(ip, username)
		return nil, ErrInvalidPassword
	}

	// 已启用 2FA 时必须校验 TOTP 码,错误同样计入失败锁定。
	if user.TOTPSecret != "" && !totp.Validate(user.TOTPSecret, totpCode) {
		recordFailedLogin(ip, username)
		return nil, ErrInvalidTOTP
	}

	clearLoginAttempts(ip, username)

	expireH := config.C.JWT.TokenExpireH
	if expireH <= 0 {
		expireH = 72
	}
	token, expiresAt, err := middleware.GenerateToken(user.Username, time.Duration(expireH)*time.Hour)
	if err != nil {
		return nil, err
	}
	return &LoginResult{Token: token, ExpiresAt: expiresAt, Username: user.Username}, nil
}

func checkLoginLock(ip, username string) (bool, time.Duration) {
	return CheckLoginLock(ip, username)
}

// ---- TOTP / 2FA ----

// TOTPSetup 生成新的 TOTP 密钥与 otpauth URI(未保存,需 Enable 时回传确认)。
func (s *AuthService) TOTPSetup(username string) (string, string, error) {
	secret, err := totp.GenerateSecret()
	if err != nil {
		return "", "", err
	}
	return secret, totp.ProvisioningURI(username, secret), nil
}

// TOTPEnable 用用户提交的 secret 校验一次性码,通过后启用 2FA。
func (s *AuthService) TOTPEnable(username, secret, code string) error {
	if !totp.Validate(secret, code) {
		return ErrInvalidTOTP
	}
	return database.DB.Model(&model.User{}).Where("username = ?", username).
		Update("totp_secret", secret).Error
}

// TOTPDisable 校验当前密码后关闭 2FA。
func (s *AuthService) TOTPDisable(username, password string) error {
	var user model.User
	if err := database.DB.Where("username = ?", username).First(&user).Error; err != nil {
		return ErrUserNotFound
	}
	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)) != nil {
		return ErrInvalidPassword
	}
	return database.DB.Model(&user).Update("totp_secret", "").Error
}

// TOTPStatus 返回当前用户是否已启用 2FA。
func (s *AuthService) TOTPStatus(username string) bool {
	var user model.User
	if err := database.DB.Where("username = ?", username).First(&user).Error; err != nil {
		return false
	}
	return user.TOTPSecret != ""
}

func recordFailedLogin(ip, username string) int {
	return RecordFailedLogin(ip, username)
}

func clearLoginAttempts(ip, username string) {
	ClearLoginAttempts(ip, username)
}
