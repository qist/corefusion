package netutil

import "testing"

func TestIPWhitelistAllows(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		ip       string
		expected bool
	}{
		{"未配置 → 放行", "", "203.0.113.9", true},
		{"仅空白 → 放行", "  ,\n ", "203.0.113.9", true},
		{"仅注释 → 放行", "# office\n", "203.0.113.9", true},
		{"单 IP 命中", "203.0.113.9", "203.0.113.9", true},
		{"单 IP 未命中", "203.0.113.9", "203.0.113.10", false},
		{"auto 后缀命中", "203.0.113.9:auto", "203.0.113.9", true},
		{"auto 后缀未命中", "203.0.113.9:auto", "203.0.113.10", false},
		{"混合列表命中第二个", "10.0.0.1,203.0.113.9:auto", "203.0.113.9", true},
		{"CIDR 命中", "203.0.113.0/24", "203.0.113.200", true},
		{"CIDR 未命中", "203.0.113.0/24", "203.0.114.1", false},
		{"IPv6 命中", "2001:db8::1", "2001:db8::1", true},
		{"IPv6 auto 后缀命中", "2001:db8::1:auto", "2001:db8::1", true},
		{"换行/分号分隔", "10.0.0.1;\n203.0.113.9", "203.0.113.9", true},
		{"来源 IP 为空 → 拒绝", "203.0.113.9", "", false},
		{"非法条目被忽略", "not-an-ip,203.0.113.9", "203.0.113.9", true},
		{"全为非法条目 → 拒绝（已配置但无法匹配）", "not-an-ip", "203.0.113.9", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IPWhitelistAllows(tc.raw, tc.ip); got != tc.expected {
				t.Fatalf("IPWhitelistAllows(%q, %q) = %v, want %v", tc.raw, tc.ip, got, tc.expected)
			}
		})
	}
}
