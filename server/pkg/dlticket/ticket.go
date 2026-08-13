// Package dlticket 提供短期、绑定资源的下载票据。
//
// 背景:浏览器原生下载无法携带 Authorization 头,而文件下载接口又不应该裸奔。
// 方案:先走带鉴权的接口换取一张票据,再用带票据的 URL 下载。
// 票据是 HMAC-SHA256 签名(无状态、可水平扩展),绑定资源标识并限时有效。
package dlticket

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// DefaultTTL 是票据默认有效期。
const DefaultTTL = 2 * time.Minute

type payload struct {
	Resource string `json:"res"`
	User     string `json:"user"`
	Expires  int64  `json:"exp"` // 纳秒时间戳,保证过期判定精度
}

// Issue 为 resource 签发一张票据,返回票据字符串与过期时间。
func Issue(secret, resource, user string, ttl time.Duration) (string, time.Time, error) {
	if strings.TrimSpace(secret) == "" {
		return "", time.Time{}, fmt.Errorf("签发密钥不能为空")
	}
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	expires := time.Now().Add(ttl)
	raw, err := json.Marshal(payload{
		Resource: resource,
		User:     user,
		Expires:  expires.UnixNano(),
	})
	if err != nil {
		return "", time.Time{}, err
	}

	body := base64.RawURLEncoding.EncodeToString(raw)
	sig := sign(secret, body)
	return body + "." + sig, expires, nil
}

// Verify 校验票据是否有效:签名正确、未过期、资源标识匹配。
func Verify(secret, ticket, resource string) (string, error) {
	if strings.TrimSpace(secret) == "" || ticket == "" {
		return "", fmt.Errorf("无效票据")
	}

	parts := strings.Split(ticket, ".")
	if len(parts) != 2 {
		return "", fmt.Errorf("无效票据")
	}
	body, sig := parts[0], parts[1]

	if !hmac.Equal([]byte(sign(secret, body)), []byte(sig)) {
		return "", fmt.Errorf("票据签名无效")
	}

	raw, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return "", fmt.Errorf("票据内容损坏")
	}

	var p payload
	if err := json.Unmarshal(raw, &p); err != nil {
		return "", fmt.Errorf("票据内容损坏")
	}

	if time.Now().UnixNano() > p.Expires {
		return "", fmt.Errorf("票据已过期")
	}
	if p.Resource != resource {
		return "", fmt.Errorf("票据与目标资源不匹配")
	}
	return p.User, nil
}

func sign(secret, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
