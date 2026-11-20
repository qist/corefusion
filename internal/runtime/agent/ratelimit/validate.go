package ratelimit

import (
	"fmt"
	"regexp"
	"strings"
)

// ifaceNamePattern 限制网卡/IFB 设备名：Linux 接口名最多 15 字符（IFNAMSIZ-1），
// 只允许字母、数字与 `._:@-`，且不能以 '-' 开头（避免被 tc 当成选项）。
// 安全审计 v7 L-1：iface 会进入 `tc ... dev <iface>` 的 argv，虽然不经过 shell，
// 但含空格/分号的值仍可能被工具解析成额外参数。
var ifaceNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:@-]{0,14}$`)

// ValidateIfaceName 校验网卡/IFB 设备名；空字符串表示自动探测，允许。
func ValidateIfaceName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	if !ifaceNamePattern.MatchString(name) {
		return fmt.Errorf("非法网卡名 %q：只允许字母、数字与 . _ : @ -（最多 15 字符）", name)
	}
	return nil
}
