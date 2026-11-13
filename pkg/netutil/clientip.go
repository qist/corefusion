package netutil

import (
	"net/netip"
	"strings"
)

// cgnat 是运营商级 NAT 段（100.64.0.0/10）。它不是公网可路由地址，
// 也不该被当成"用户设备 IP"做限速/阻断的键。
var cgnat = netip.MustParsePrefix("100.64.0.0/10")

// IsPublicClientIP 判断一个 IP 是否是"可直接用于限速/设备统计的公网客户端地址"。
//
// 背景：核心上报的连接源 IP 在多种部署下并不是真实客户端地址 ——
//   - 入站前面挂了 nginx/caddy 且未开 PROXY protocol → 127.0.0.1；
//   - 容器/内网转发 → 私网地址；
//   - IPv6 链路本地/ULA。
//
// 这些地址既不能用来打 nft 标记，也不能算设备数（一个 127.0.0.1 会把所有
// localhost 连接算成同一台设备）。统一在这里判定，Agent / 面板共用同一口径。
func IsPublicClientIP(raw string) bool {
	v := strings.TrimSpace(raw)
	v = strings.TrimPrefix(v, "[")
	v = strings.TrimSuffix(v, "]")
	if v == "" {
		return false
	}
	addr, err := netip.ParseAddr(v)
	if err != nil {
		return false
	}
	addr = addr.Unmap()
	if !addr.IsValid() ||
		addr.IsUnspecified() ||
		addr.IsLoopback() ||
		addr.IsPrivate() || // RFC1918 + fc00::/7
		addr.IsLinkLocalUnicast() ||
		addr.IsLinkLocalMulticast() ||
		addr.IsMulticast() {
		return false
	}
	if addr.Is4() && cgnat.Contains(addr) {
		return false
	}
	return true
}
