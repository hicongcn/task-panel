package middleware

import (
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// 可配置的可信代理网段。默认只信任回环地址 —— 不默认信任整个私网段,
// 避免局域网直连时攻击者用 X-Forwarded-For 伪造来源 IP 绕过白名单/限流。
// 部署在反向代理(nginx 等)后时,由管理员显式配置代理网段。
var trustedProxyCIDRs = defaultTrustedProxyCIDRs()

func defaultTrustedProxyCIDRs() []*net.IPNet {
	var nets []*net.IPNet
	for _, cidr := range []string{"127.0.0.1/32", "::1/128"} {
		_, network, err := net.ParseCIDR(cidr)
		if err == nil {
			nets = append(nets, network)
		}
	}
	return nets
}

// SetTrustedProxyCIDRs 覆盖可信代理网段配置。
func SetTrustedProxyCIDRs(cidrs []string) error {
	var nets []*net.IPNet
	for _, cidr := range cidrs {
		_, network, err := net.ParseCIDR(strings.TrimSpace(cidr))
		if err != nil {
			return err
		}
		nets = append(nets, network)
	}
	if len(nets) == 0 {
		nets = defaultTrustedProxyCIDRs()
	}
	trustedProxyCIDRs = nets
	return nil
}

// ResolveClientIP 解析客户端真实 IP:
//   - 仅当直连对端(RemoteAddr)属于可信代理网段时,才信任转发头;
//   - 转发头优先取 X-Forwarded-For 中最右侧的非可信代理地址;
//   - 其余情况一律用直连对端 IP,防止伪造。
func ResolveClientIP(c *gin.Context) string {
	return ResolveClientIPFromRequest(c.Request)
}

// ResolveClientIPFromRequest 供非 Gin 场景复用。
func ResolveClientIPFromRequest(r *http.Request) string {
	remoteIP := normalizeIPString(r.RemoteAddr)

	if isTrustedProxy(remoteIP) {
		if forwarded := extractForwardedIP(r.Header); forwarded != "" {
			return forwarded
		}
	}
	if remoteIP != "" {
		return remoteIP
	}
	return ""
}

// extractForwardedIP 从 X-Forwarded-For(或 X-Real-IP)中提取客户端 IP。
// XFF 可能为逗号分隔的链,取最右侧的非可信代理地址。
func extractForwardedIP(header http.Header) string {
	if xff := strings.TrimSpace(header.Get("X-Forwarded-For")); xff != "" {
		parts := strings.Split(xff, ",")
		for i := len(parts) - 1; i >= 0; i-- {
			ip := normalizeIPString(parts[i])
			if ip == "" {
				continue
			}
			parsed := net.ParseIP(ip)
			if parsed != nil && isTrustedProxy(parsed.String()) {
				continue
			}
			return ip
		}
	}
	if real := normalizeIPString(header.Get("X-Real-IP")); real != "" {
		return real
	}
	return ""
}

func isTrustedProxy(ip string) bool {
	if ip == "" {
		return false
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, network := range trustedProxyCIDRs {
		if network.Contains(parsed) {
			return true
		}
	}
	return false
}

func normalizeIPString(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	}
	value = strings.Trim(value, "[]")
	ip := net.ParseIP(value)
	if ip == nil {
		return ""
	}
	return ip.String()
}
