package xraycontrol

import "testing"

// iptables 的端口段必须用冒号；中划线会被当成服务名解析失败
// （真机报错：Port "20000-30000" does not resolve to anything）。
func TestIptablesPortSpec(t *testing.T) {
	cases := map[string]string{
		"443":         "443",
		"20000-30000": "20000:30000",
		"1000-2000":   "1000:2000",
	}
	for in, want := range cases {
		if got := iptablesPortSpec(in); got != want {
			t.Errorf("iptablesPortSpec(%q) = %q, want %q", in, got, want)
		}
	}
}
