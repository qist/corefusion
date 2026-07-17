package netutil

import (
	"net"
	"net/http"
	"strings"
)

// CDN header 优先级（从高到低）：
// 1. CF-Connecting-IP     — Cloudflare 专用，最可信
// 2. X-Real-IP            — Nginx/Caddy 反代设置
// 3. X-Forwarded-For      — 标准代理头，取第一个
// 4. True-Client-IP       — Akamai / Cloudflare Enterprise
// 5. X-Client-IP          — 部分负载均衡器
var cdnHeaders = []string{
	"CF-Connecting-IP",
	"True-Client-IP",
	"X-Real-IP",
	"X-Client-IP",
}

// GetRealIP 从 HTTP 请求中获取客户端真实 IP。
//
// 优先级：
//  1. CDN 专用头（CF-Connecting-IP, True-Client-IP 等）
//  2. X-Forwarded-For 第一个非内网 IP
//  3. http.Request.RemoteAddr（TCP 直连 IP）
//
// 如果 CoreFusion 在 CDN/反代后面，c.ClientIP() 返回的是 CDN 节点 IP，
// 本函数解析 CDN 头获取支付平台的真实出口 IP。
func GetRealIP(r *http.Request) string {
	// ① 检查 CDN 专用头（按优先级）
	for _, header := range cdnHeaders {
		ip := strings.TrimSpace(r.Header.Get(header))
		if ip != "" && isValidPublicIP(ip) {
			return ip
		}
	}

	// ② 检查 X-Forwarded-For（可能包含多级代理链）
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// X-Forwarded-For: client, proxy1, proxy2
		parts := strings.Split(xff, ",")
		for i := len(parts) - 1; i >= 0; i-- {
			ip := strings.TrimSpace(parts[i])
			if ip != "" && isValidPublicIP(ip) {
				return ip
			}
		}
		// 如果全是内网 IP，取第一个（至少有个地址）
		if len(parts) > 0 {
			first := strings.TrimSpace(parts[0])
			if first != "" {
				return first
			}
		}
	}

	// ③ 回退到 TCP 连接地址
	if r.RemoteAddr != "" {
		// RemoteAddr 格式: IP:Port
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err == nil {
			return host
		}
		return r.RemoteAddr
	}

	return ""
}

// isValidPublicIP 检查 IP 字符串是否为有效的公网 IP（排除内网/保留地址，支持 IPv4 和 IPv6）。
func isValidPublicIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	return isValidPublicIPAddr(ip)
}

// isValidPublicIPAddr 检查 net.IP 是否为有效的公网 IP（排除内网/保留地址，支持 IPv4 和 IPv6）。
func isValidPublicIPAddr(ip net.IP) bool {
	if ip == nil {
		return false
	}
	// 通用排除：回环、链路本地、未指定地址（同时覆盖 IPv4 和 IPv6）
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return false
	}
	// IPv4 私有/保留网段检查（参考 IANA Special-Purpose Address Registry）
	if ip4 := ip.To4(); ip4 != nil {
		switch {
		case ip4[0] == 0: // 0.0.0.0/8 (本网络)
			return false
		case ip4[0] == 10: // 10.0.0.0/8 (私有 A)
			return false
		case ip4[0] == 127: // 127.0.0.0/8 (回环)
			return false
		case ip4[0] == 169 && ip4[1] == 254: // 169.254.0.0/16 (link-local)
			return false
		case ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31: // 172.16.0.0/12 (私有 B)
			return false
		case ip4[0] == 192 && ip4[1] == 0 && ip4[2] == 0: // 192.0.0.0/24 (IETF 协议分配)
			return false
		case ip4[0] == 192 && ip4[1] == 0 && ip4[2] == 2: // 192.0.2.0/24 (TEST-NET-1)
			return false
		case ip4[0] == 192 && ip4[1] == 168: // 192.168.0.0/16 (私有 C)
			return false
		case ip4[0] == 198 && ip4[1] >= 18 && ip4[1] <= 19: // 198.18.0.0/15 (基准测试)
			return false
		case ip4[0] == 198 && ip4[1] == 51 && ip4[2] == 100: // 198.51.100.0/24 (TEST-NET-2)
			return false
		case ip4[0] == 203 && ip4[1] == 0 && ip4[2] == 113: // 203.0.113.0/24 (TEST-NET-3)
			return false
		case ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127: // 100.64.0.0/10 (CGNAT)
			return false
		case ip4[0] >= 224: // 224.0.0.0/4 (multicast) + 240.0.0.0/4 (reserved)
			return false
		}
		return true
	}
	// IPv6 私有/保留地址检查
	// 注意：::ffff:0:0/96 (IPv4-mapped IPv6) 会被 ip.To4() 归一化为 IPv4，
	//       因此已由上面的 IPv4 分支处理（如 ::ffff:127.0.0.1 会被 ip4[0]==127 拦截）。
	// 以下仅处理纯 IPv6 地址（ip.To4() 返回 nil）。
	ip6 := ip.To16()
	if ip6 != nil {
		// ::1 — 回环（已由 IsLoopback 覆盖）
		// fc00::/7 — Unique Local Addresses (ULA)，IPv6 私有地址
		if (ip6[0] & 0xfe) == 0xfc {
			return false
		}
		// fe80::/10 — Link-Local（已由 IsLinkLocalUnicast 覆盖，但显式再检查一次更安全）
		if ip6[0] == 0xfe && (ip6[1]&0xc0) == 0x80 {
			return false
		}
		// ff00::/8 — Multicast
		if ip6[0] == 0xff {
			return false
		}
		// 2001:db8::/32 — Documentation/example addresses (RFC 3849)
		if ip6[0] == 0x20 && ip6[1] == 0x01 && ip6[2] == 0x0d && ip6[3] == 0xb8 {
			return false
		}
		// 64:ff9b::/96 — NAT64 well-known prefix (RFC 6052)
		if ip6[0] == 0x00 && ip6[1] == 0x64 && ip6[2] == 0xff && ip6[3] == 0x9b {
			return false
		}
		// 100::/64 — Discard prefix (RFC 6666)
		if ip6[0] == 0x01 && ip6[1] == 0x00 && ip6[2] == 0 && ip6[3] == 0 &&
			ip6[4] == 0 && ip6[5] == 0 && ip6[6] == 0 && ip6[7] == 0 {
			return false
		}
	}
	return true
}

// IsPrivateIP 检查 IP 字符串是否为内网/保留地址。
// 用于 SSRF 防护（域名验证时禁止访问内网）。
func IsPrivateIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return true // 无法解析视为不安全
	}
	return !isValidPublicIPAddr(ip)
}

// IsReservedIP 检查 IP 是否为云元数据端点等保留地址（支持 IPv4 和 IPv6）。
// 169.254.169.254 是云厂商 metadata 端点，必须禁止访问。
// IPv6 fd00:ec2::254 是 AWS IMDS IPv6 端点。
func IsReservedIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return true
	}
	// IPv4 云元数据端点
	if ip.Equal(net.ParseIP("169.254.169.254")) {
		return true
	}
	// IPv6 云元数据端点 (AWS IMDS IPv6)
	if ip.Equal(net.ParseIP("fd00:ec2::254")) {
		return true
	}
	// IPv6 ULA 私有地址 fc00::/7
	ip6 := ip.To16()
	if ip6 != nil && ip.To4() == nil {
		if (ip6[0] & 0xfe) == 0xfc {
			return true
		}
	}
	return false
}
