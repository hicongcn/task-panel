// Package totp 实现 RFC 6238 的 TOTP(基于时间的一次性密码)。
// 仅用标准库(crypto/hmac + sha1),无第三方依赖。
package totp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// GenerateSecret 生成 20 字节随机密钥,返回 base32(去填充)。
func GenerateSecret() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return strings.TrimRight(base32.StdEncoding.EncodeToString(b), "="), nil
}

// Validate 校验用户输入的 6 位 TOTP 码,容忍 ±1 个时间窗口的时钟偏移。
func Validate(secret, code string) bool {
	code = strings.TrimSpace(code)
	if len(code) != 6 {
		return false
	}
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(secret))
	if err != nil {
		return false
	}
	now := time.Now().Unix() / 30
	for i := int64(-1); i <= 1; i++ {
		if hotp(key, uint64(now+i)) == code {
			return true
		}
	}
	return false
}

// ProvisioningURI 生成 otpauth:// URI(前端据此渲染二维码)。
func ProvisioningURI(username, secret string) string {
	issuer := "task-panel"
	label := url.PathEscape(issuer + ":" + username)
	return fmt.Sprintf("otpauth://totp/%s?secret=%s&issuer=%s&digits=6&period=30",
		label, url.QueryEscape(secret), url.QueryEscape(issuer))
}

// CurrentCode 返回当前时刻的 6 位 TOTP 码(测试与运维工具使用)。
func CurrentCode(secret string) string {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(secret))
	if err != nil {
		return ""
	}
	return hotp(key, uint64(time.Now().Unix()/30))
}

// hotp 计算 HTOP(RFC 4226)6 位码。
func hotp(key []byte, counter uint64) string {
	var msg [8]byte
	for i := 0; i < 8; i++ {
		msg[7-i] = byte(counter >> (8 * i))
	}
	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write(msg[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	code := (uint32(sum[offset])&0x7f)<<24 |
		uint32(sum[offset+1])<<16 |
		uint32(sum[offset+2])<<8 |
		uint32(sum[offset+3])
	return fmt.Sprintf("%06d", code%1000000)
}
