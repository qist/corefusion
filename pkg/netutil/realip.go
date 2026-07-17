package netutil

import (
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
)

// ============================================================
// Trusted Proxy Configuration
// ============================================================

// trustedProxyConfig 是全局可信代理配置，通过 SetTrustedProxies 设置。
// 只有来自这些 IP/CIDR 的请求才会被解析 CDN/代理头。
var (
	trustedProxyMu  sync.RWMutex
	trustedProxyCfg *TrustedProxyConfig
)

// TrustedProxyConfig 保存预解析的可信代理 IP/CIDR 列表。
type TrustedProxyConfig struct {
	cidrs []*net.IPNet
	ips   []net.IP
}

// NewTrustedProxyConfig 解析 IP/CIDR 字符串列表为可信代理配置。
// 支持 "1.2.3.4"、"10.0.0.0/8"、"::1"、"fd00::/8" 等格式。
func NewTrustedProxyConfig(proxies []string) *TrustedProxyConfig {
	cfg := &TrustedProxyConfig{}
	for _, p := range proxies {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// 尝试 CIDR
		if strings.Contains(p, "/") {
			_, cidr, err := net.ParseCIDR(p)
			if err != nil {
				log.Printf("[netutil] invalid trusted proxy CIDR %q: %v", p, err)
				continue
			}
			cfg.cidrs = append(cfg.cidrs, cidr)
		} else {
			// 单个 IP
			ip := net.ParseIP(p)
			if ip == nil {
				log.Printf("[netutil] invalid trusted proxy IP %q", p)
				continue
			}
			cfg.ips = append(cfg.ips, ip)
		}
	}
	return cfg
}

// IsTrusted 检查 IP 是否在可信代理列表中。
func (c *TrustedProxyConfig) IsTrusted(ipStr string) bool {
	if c == nil {
		return false
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	for _, c := range c.cidrs {
		if c.Contains(ip) {
			return true
		}
	}
	for _, trustedIP := range c.ips {
		if trustedIP.Equal(ip) {
			return true
		}
	}
	return false
}

// SetTrustedProxies 设置全局可信代理列表。
// 应在程序启动时调用（如从配置文件/环境变量读取后）。
// 设置后，GetRealIP 只有在 RemoteAddr 来自这些 IP 时才解析 CDN 头。
func SetTrustedProxies(proxies []string) {
	trustedProxyMu.Lock()
	defer trustedProxyMu.Unlock()
	trustedProxyCfg = NewTrustedProxyConfig(proxies)
	log.Printf("[netutil] trusted proxies configured: %d entries", len(proxies))
}

// GetTrustedProxyConfig 返回当前全局可信代理配置（只读）。
func GetTrustedProxyConfig() *TrustedProxyConfig {
	trustedProxyMu.RLock()
	defer trustedProxyMu.RUnlock()
	return trustedProxyCfg
}

// ============================================================
// Cloudflare IP Ranges
// ============================================================

// CloudflareIPv4Ranges 返回 Cloudflare 的 IPv4 段。
// 来源：https://www.cloudflare.com/ips-v4/
var cloudflareIPv4Ranges = []string{
	"173.245.48.0/20",
	"103.21.244.0/22",
	"103.22.200.0/22",
	"103.31.4.0/22",
	"141.101.64.0/18",
	"108.162.192.0/18",
	"190.93.240.0/20",
	"188.114.96.0/20",
	"197.234.240.0/22",
	"198.41.128.0/17",
	"162.158.0.0/15",
	"104.16.0.0/13",
	"104.24.0.0/14",
	"172.64.0.0/13",
	"131.0.72.0/22",
}

// CloudflareIPv6Ranges 返回 Cloudflare 的 IPv6 段。
// 来源：https://www.cloudflare.com/ips-v6/
var cloudflareIPv6Ranges = []string{
	"2400:cb00::/32",
	"2606:4700::/32",
	"2803:f800::/32",
	"2606:4700::/32",
	"2803:f800::/32",
	"2a06:98c0::/29",
	"2c0f:f248::/32",
}

// CloudflareProxyRanges 返回 Cloudflare 所有 IP 段（IPv4 + IPv6）。
// 可直接传给 SetTrustedProxies。
func CloudflareProxyRanges() []string {
	result := make([]string, 0, len(cloudflareIPv4Ranges)+len(cloudflareIPv6Ranges))
	result = append(result, cloudflareIPv4Ranges...)
	result = append(result, cloudflareIPv6Ranges...)
	return result
}

// ============================================================
// Real IP Extraction
// ============================================================

// CDN header 优先级（从高到低）：
// 1. CF-Connecting-IP     — Cloudflare 专用，最可信
// 2. True-Client-IP       — Akamai / Cloudflare Enterprise
// 3. X-Real-IP            — Nginx/Caddy 反代设置
// 4. X-Client-IP          — 部分负载均衡器
var cdnHeaders = []string{
	"CF-Connecting-IP",
	"True-Client-IP",
	"X-Real-IP",
	"X-Client-IP",
}

// GetRealIP 从 HTTP 请求中获取客户端真实 IP（安全版本）。
//
// 安全策略：
//   - 首先获取 TCP 直连 IP（RemoteAddr）
//   - 如果配置了全局可信代理（SetTrustedProxies），检查直连 IP 是否可信
//   - 只有来自可信代理的请求，才解析 CDN/代理头（CF-Connecting-IP 等）
//   - 如果没有配置可信代理，或直连 IP 不在可信列表中，直接返回 RemoteAddr
//
// 这样可以防止攻击者直连服务器伪造 CF-Connecting-IP 头。
func GetRealIP(r *http.Request) string {
	// ① 获取 TCP 直连 IP
	directIP := remoteAddrToIP(r)
	if directIP == "" {
		return ""
	}

	// ② 检查是否来自可信代理
	cfg := GetTrustedProxyConfig()
	if cfg == nil || !cfg.IsTrusted(directIP) {
		// 不可信来源：直接返回 TCP 连接 IP，忽略所有头
		return directIP
	}

	// ③ 来自可信代理：解析 CDN 专用头（按优先级）
	for _, header := range cdnHeaders {
		ip := strings.TrimSpace(r.Header.Get(header))
		if ip != "" && isValidPublicIP(ip) {
			return ip
		}
	}

	// ④ 解析 X-Forwarded-For（可能包含多级代理链）
	// 从右向左取第一个公网 IP（跳过可信代理自身的 IP）
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		parts := strings.Split(xff, ",")
		for i := len(parts) - 1; i >= 0; i-- {
			ip := strings.TrimSpace(parts[i])
			if ip == "" {
				continue
			}
			// 跳过可信代理自身的 IP
			if cfg.IsTrusted(ip) {
				continue
			}
			if isValidPublicIP(ip) {
				return ip
			}
		}
		// 如果全是内网/可信 IP，取第一个（至少有个地址）
		for i := 0; i < len(parts); i++ {
			ip := strings.TrimSpace(parts[i])
			if ip != "" {
				return ip
			}
		}
	}

	// ⑤ 可信代理但头缺失/无效：回退到直连 IP
	return directIP
}

// remoteAddrToIP 从 http.Request.RemoteAddr 提取 IP 地址。
func remoteAddrToIP(r *http.Request) string {
	if r.RemoteAddr == "" {
		return ""
	}
	// RemoteAddr 格式: IP:Port 或 [IPv6]:Port
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

// ============================================================
// IP Validation (IPv4 + IPv6)
// ============================================================

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
