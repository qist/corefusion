package ratelimit

import "testing"

// 安全审计 v7 L-1：网卡名进 tc argv 前必须只允许安全字符。
func TestValidateIfaceName(t *testing.T) {
	good := []string{"", "eth0", "ens18", "wg0", "br-abc123", "ifb0", "veth1.2", "eth0@if5", "bond0:1"}
	for _, name := range good {
		if err := ValidateIfaceName(name); err != nil {
			t.Errorf("ValidateIfaceName(%q) = %v, want nil", name, err)
		}
	}

	bad := []string{
		"eth0 ; flush ruleset",
		"eth0\neth1",
		"-eth0",
		"eth 0",
		"$(id)",
		"eth0 dev",
		"a1234567890123456", // 超过 15 字符
	}
	for _, name := range bad {
		if err := ValidateIfaceName(name); err == nil {
			t.Errorf("ValidateIfaceName(%q) = nil, want error", name)
		}
	}
}
