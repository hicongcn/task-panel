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
	"taskpanel/pkg/validator"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidUsername = errors.New("用户名需 1-32 位,支持中文、字母、数字和下划线")
	ErrPasswordTooShort = errors.New("密码长度需 6-128 位")
	ErrAlreadyInit      = errors.New("系统已初始化")
	ErrUserNotFound     = errors.New("用户不存在")
	ErrInvalidPassword  = errors.New("用户名或密码错误")
	ErrAccountLocked    = errors.New("账号已锁定,请稍后再试")
	ErrAccountDisabled  = errors.New("账号已被禁用")
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

// Login 校验账密并签发令牌。失败时按 IP 记录失败次数并锁定。
func (s *AuthService) Login(username, password, ip string) (*LoginResult, error) {
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

func recordFailedLogin(ip, username string) int {
	return RecordFailedLogin(ip, username)
}

func clearLoginAttempts(ip, username string) {
	ClearLoginAttempts(ip, username)
}
