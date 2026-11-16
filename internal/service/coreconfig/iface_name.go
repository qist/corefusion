package coreconfig

import (
	"fmt"
	"hash/fnv"
	"strings"
)

// maxIfaceNameLen 是 Linux 网卡名的上限：IFNAMSIZ(16) 含结尾 NUL，实际名字最长 15 字节。
//
// 超长时核心直接起不来（sing-box 1.15 实测）：
//
//	FATAL start service: start outbound/wireguard[wg-test]: open tun: create ifreq: invalid argument
//
// 面板历史版本给 WireGuard 端点默认生成 `WireGuard-<6位随机>`（16 字符），
// 与「使用系统网络集成」一起用时必然踩这个限制。
const maxIfaceNameLen = 15

// NormalizeIfaceName 把用户填写的网卡名收敛成 Linux 可用的接口名：
//   - 只保留字母、数字、`.`、`_`、`-`，其余（中文/空格/斜杠等）一律换成 `-`；
//   - 去掉首尾的 `.` / `-`，最长 15 字节；
//   - 为空、仍是保留名（lo）或超长 → 回退到 fallback（通常是端点 tag）；
//   - fallback 也不可用时用 `wg` + 6 位哈希，保证结果确定且唯一。
//
// 返回值永远是合法名字，调用方可以直接写进配置。
func NormalizeIfaceName(raw, fallback string) string {
	if v := ifaceCharsOnly(raw); v != "" && len(v) <= maxIfaceNameLen && !reservedIfaceName(v) {
		return v
	}
	if v := ifaceCharsOnly(fallback); v != "" && !reservedIfaceName(v) {
		if len(v) > maxIfaceNameLen {
			v = strings.Trim(v[:maxIfaceNameLen], "-.")
		}
		if v != "" {
			return v
		}
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(raw + "|" + fallback))
	return fmt.Sprintf("wg%06x", h.Sum32()&0xffffff)
}

// ifaceCharsOnly 只保留内核接受的字符，并把连续非法字符压成一个 `-`。
func ifaceCharsOnly(s string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.TrimSpace(s) {
		ok := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			r == '_' || r == '.' || r == '-'
		if !ok {
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
			continue
		}
		b.WriteRune(r)
		prevDash = r == '-'
	}
	return strings.Trim(b.String(), "-.")
}

// reservedIfaceName 是无论如何都不能当 VPN 网卡名的名字。
func reservedIfaceName(s string) bool {
	switch s {
	case "", ".", "..", "lo":
		return true
	}
	return false
}
