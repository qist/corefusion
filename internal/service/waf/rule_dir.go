package waf

import (
	"net"
	"strings"
)

// 站点域名组（多域名站点）在 domain.json 里的 key 可能是
//
//	"www.example.com, www.example.org, www.example.net, www.example.io"
//
// key 允许逗号/空格（WAF 引擎按 host 拆分匹配），但 **rule_dir 是文件系统路径**：
// 逗号、空格、通配符 `*` 会让路径非法或需要转义（Lua/CaddyGuard 都是直接拼接读文件），
// 而且整串域名很容易超过单个目录名 255 字节的上限。
//
// 所以 rule_dir 统一按「域名组里第一个域名」归一：稳定、可读、唯一，
// 且与单域名站点的历史目录（domains/<domain>）完全兼容。
const ruleDirPrefix = "domains/"

// DomainRuleDirFor 返回站点域名组对应的规则目录（相对 waf-rules/）。
//
//	"a.example.com, b.example.org, ..." → "domains/a.example.com"
func DomainRuleDirFor(domain string) string {
	return ruleDirPrefix + sanitizeDomainDirName(firstDomainOfGroup(domain))
}

// NormalizeRuleDir 归一已有的 rule_dir 值：
//   - 空串：原样返回（不覆盖，沿用全局规则）；
//   - 绝对路径 / 非 domains/ 开头的自定义目录：原样返回（属于管理员自建目录，安全校验另做）；
//   - domains/<单个安全名字>：原样返回（保持用户/历史目录不变）；
//   - domains/<多域名或含空格、逗号、通配符等非法字符>：改成 domains/<首个域名>。
//
// 这样既修掉多域名站点产生的非法目录，又不会把管理员自定义的目录改掉。
func NormalizeRuleDir(domain, ruleDir string) string {
	dir := strings.TrimSpace(ruleDir)
	if dir == "" {
		return ""
	}
	if !strings.HasPrefix(strings.ToLower(dir), ruleDirPrefix) {
		return dir
	}
	name := strings.TrimSpace(dir[len(ruleDirPrefix):])
	if name != "" && isSafeDomainDirName(name) {
		return ruleDirPrefix + name
	}
	return DomainRuleDirFor(domain)
}

// firstDomainOfGroup 取逗号分隔域名组里的第一个名字，并去掉端口 / 末尾点。
func firstDomainOfGroup(group string) string {
	for _, part := range strings.Split(group, ",") {
		d := strings.TrimSpace(part)
		if d == "" {
			continue
		}
		// [2001:db8::1]:443 / [2001:db8::1]
		if strings.HasPrefix(d, "[") {
			if i := strings.LastIndex(d, "]"); i > 0 {
				d = d[1:i]
			}
		} else if host, _, err := net.SplitHostPort(d); err == nil {
			d = host
		}
		return strings.TrimSuffix(strings.TrimSpace(d), ".")
	}
	return ""
}

// isSafeDomainDirName 判断单个目录名是否合法：只允许小写字母/数字/点/横线/下划线，
// 不含逗号、空格、通配符等需要转义的字符。
func isSafeDomainDirName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}

// sanitizeDomainDirName 把域名转成安全的单段目录名；空/非法时回落到 "default"。
func sanitizeDomainDirName(domain string) string {
	d := strings.ToLower(strings.TrimSpace(domain))
	// 通配符域名：*.example.com → _wildcard.example.com（`*` 不能出现在路径里）
	if strings.HasPrefix(d, "*.") {
		d = "_wildcard." + d[2:]
	}
	var b strings.Builder
	for _, r := range d {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	// 只裁掉首尾的 `.`/`-`：保留 `_wildcard.` 前缀（否则会和真实域名 wildcard.example.com 撞目录）。
	name := strings.Trim(b.String(), ".-")
	// 单段文件名上限通常是 255 字节；这里留足空间给 `<name>/<rule>` 的余量。
	if len(name) > 128 {
		name = strings.TrimRight(name[:128], ".-")
	}
	if name == "" || name == "_" {
		return "default"
	}
	return name
}
