package netutil

import (
	"fmt"
	"net"
	"strings"
)

// 用户级 IP 白名单（clients.ip_whitelist）解析、校验与匹配。
//
// 口径（2026-10-20 确认）：
//   - 只作用于**订阅拉取**（/sub/:token 与 8081 的 /:token）；门户登录、OAuth、
//     其它 client-auth API 一律不拦 —— 否则用户填错 IP 会把自己锁在门外。
//   - 白名单是**用户级**（按 client_id），不是面板全局。
//   - 未配置白名单、或该客户没有绑定订阅地址时，必须放行。
//
// 存储格式是历史遗留的自由文本：逗号/分号/空白/换行分隔，条目可以是
//   - 单个 IPv4 / IPv6
//   - CIDR（如 203.0.113.0/24）
//   - 带 ":auto" 后缀的自动添加项（AutoAddIP 写入，如 1.2.3.4:auto）
//   - `#` 开头的整行注释
const ipWhitelistAutoSuffix = ":auto"

// ParseIPWhitelist 把白名单原文切成条目（去空、去注释）。
// 先按行切分并整行丢弃注释，再按逗号/分号/空白切条目 —— 否则 "# office"
// 会被空格拆成 "#" + "office"，后者变成非法条目。
func ParseIPWhitelist(raw string) []string {
	entries := make([]string, 0, 8)
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		for _, f := range strings.FieldsFunc(line, func(r rune) bool {
			return r == ',' || r == ';' || r == ' ' || r == '\t'
		}) {
			if f = strings.TrimSpace(f); f != "" {
				entries = append(entries, f)
			}
		}
	}
	return entries
}

// NormalizeWhitelistEntry 去掉自动添加后缀，返回可被 net 包解析的 IP/CIDR。
// 先按原文解析，失败再尝试去后缀 —— 避免把 IPv6 地址里的冒号误当后缀。
func NormalizeWhitelistEntry(entry string) string {
	if entry == "" {
		return ""
	}
	if strings.Contains(entry, "/") {
		if _, _, err := net.ParseCIDR(entry); err == nil {
			return entry
		}
		if trimmed := strings.TrimSuffix(entry, ipWhitelistAutoSuffix); trimmed != entry {
			if _, _, err := net.ParseCIDR(trimmed); err == nil {
				return trimmed
			}
		}
		return ""
	}
	if net.ParseIP(entry) != nil {
		return entry
	}
	if trimmed := strings.TrimSuffix(entry, ipWhitelistAutoSuffix); trimmed != entry {
		if net.ParseIP(trimmed) != nil {
			return trimmed
		}
	}
	return ""
}

// ValidateIPWhitelist 校验用户提交的白名单文本：空文本合法（= 不限制），
// 每个条目必须是 IP / CIDR（可带 ":auto"），否则返回错误。
// 返回规范化后的文本（用逗号连接），便于落库前统一格式。
func ValidateIPWhitelist(raw string) (string, error) {
	entries := ParseIPWhitelist(raw)
	if len(entries) == 0 {
		return "", nil
	}
	normalized := make([]string, 0, len(entries))
	seen := make(map[string]bool, len(entries))
	for _, entry := range entries {
		norm := NormalizeWhitelistEntry(entry)
		if norm == "" {
			return "", fmt.Errorf("invalid IP whitelist entry: %q", entry)
		}
		key := norm
		if seen[key] {
			continue
		}
		seen[key] = true
		normalized = append(normalized, norm)
	}
	return strings.Join(normalized, ","), nil
}

// IPWhitelistAllows 判断来源 IP 是否被白名单放行。
// raw 为空（未配置）→ 放行；配置了白名单但来源 IP 无法解析 → 拒绝
// （既然启用了白名单，就必须能判定来源）。
func IPWhitelistAllows(raw, clientIP string) bool {
	entries := ParseIPWhitelist(raw)
	if len(entries) == 0 {
		return true
	}
	ip := net.ParseIP(strings.TrimSpace(clientIP))
	if ip == nil {
		return false
	}
	for _, entry := range entries {
		norm := NormalizeWhitelistEntry(entry)
		if norm == "" {
			continue
		}
		if strings.Contains(norm, "/") {
			if _, ipnet, err := net.ParseCIDR(norm); err == nil && ipnet.Contains(ip) {
				return true
			}
			continue
		}
		if allowed := net.ParseIP(norm); allowed != nil && allowed.Equal(ip) {
			return true
		}
	}
	return false
}
