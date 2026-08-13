// Package validator 提供用户名/密码等输入校验工具。
package validator

import (
	"strings"
	"unicode/utf8"
)

// ValidateUsername 校验用户名:1-32 个字符,支持中文、字母、数字和下划线。
func ValidateUsername(username string) bool {
	name := strings.TrimSpace(username)
	if name == "" || utf8.RuneCountInString(name) > 32 {
		return false
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
		case r >= 0x4e00 && r <= 0x9fff: // 常用 CJK 统一表意文字区
		default:
			return false
		}
	}
	return true
}

// ValidatePassword 校验密码:6-128 个字符。
func ValidatePassword(password string) bool {
	return utf8.RuneCountInString(password) >= 6 && utf8.RuneCountInString(password) <= 128
}

// SanitizeString 去除字符串首尾空白。
func SanitizeString(value string) string {
	return strings.TrimSpace(value)
}

// ValidateEnvName 校验环境变量名:须为合法 shell 变量名([A-Za-z_][A-Za-z0-9_]*),
// 避免把非法名字注入到子进程环境导致解释器异常。
func ValidateEnvName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r == '_':
		case r >= '0' && r <= '9':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}
