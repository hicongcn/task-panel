package service

import (
	"errors"
	"time"

	"taskpanel/database"
	"taskpanel/model"

	"gorm.io/gorm"
)

// CheckLoginLock 返回某 IP+用户名 是否处于锁定中及剩余时间。
func CheckLoginLock(ip, username string) (bool, time.Duration) {
	var attempt model.LoginAttempt
	err := database.DB.Where("ip = ? AND username = ?", ip, username).Take(&attempt).Error
	if err != nil {
		// 无记录或查询失败都视为未锁定(查询失败极罕见,保守放行)。
		return false, 0
	}
	if attempt.Count >= maxLoginAttempts && attempt.LockedAt != nil {
		remaining := attempt.ExpiresAt.Sub(time.Now())
		if remaining > 0 {
			return true, remaining
		}
	}
	return false, 0
}

// RecordFailedLogin 记录一次失败登录,达到阈值后锁定并阶梯延长锁定时长。
func RecordFailedLogin(ip, username string) int {
	var attempt model.LoginAttempt
	err := database.DB.Where("ip = ? AND username = ?", ip, username).Take(&attempt).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		a := model.LoginAttempt{
			IP: ip, Username: username, Count: 1, ExpiresAt: time.Now().Add(lockDuration),
		}
		database.DB.Create(&a)
		return 1
	}
	if err != nil {
		return 0
	}
	attempt.Count++
	if attempt.Count >= maxLoginAttempts {
		now := time.Now()
		attempt.LockedAt = &now
		multiplier := attempt.Count - maxLoginAttempts + 1
		if multiplier < 1 {
			multiplier = 1
		}
		attempt.ExpiresAt = now.Add(time.Duration(multiplier) * lockDuration)
	}
	database.DB.Save(&attempt)
	return attempt.Count
}

// ClearLoginAttempts 清除某 IP+用户名 的失败计数。
func ClearLoginAttempts(ip, username string) {
	database.DB.Where("ip = ? AND username = ?", ip, username).Delete(&model.LoginAttempt{})
}
