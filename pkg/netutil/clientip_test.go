package netutil

import "testing"

func TestIsPublicClientIP(t *testing.T) {
	cases := []struct {
		ip   string
		want bool
	}{
		{"1.2.3.4", true},
		{"8.8.8.8", true},
		{"2001:4860:4860::8888", true},
		{"::ffff:1.2.3.4", true}, // IPv4-mapped 公网地址
		{"", false},
		{"not-an-ip", false},
		{"0.0.0.0", false},
		{"127.0.0.1", false},
		{"10.1.2.3", false},
		{"172.16.5.5", false},
		{"172.31.255.254", false},
		{"192.168.1.10", false},
		{"169.254.1.1", false},
		{"100.64.0.1", false}, // CGNAT
		{"::1", false},
		{"fe80::1", false},
		{"fd00::1", false},
		{"224.0.0.1", false},
	}
	for _, tc := range cases {
		if got := IsPublicClientIP(tc.ip); got != tc.want {
			t.Errorf("IsPublicClientIP(%q) = %v, want %v", tc.ip, got, tc.want)
		}
	}
}

// 在线用户展示：私网/CGNAT 也要保留（内网客户端连入时不能整条消失）；
// 但回环/链路本地/多播仍然排除（无法归属到具体设备）。
func TestIsUsableClientIP(t *testing.T) {
	cases := map[string]bool{
		"8.8.8.8":        true,
		"192.168.55.254": true,
		"10.0.0.5":       true,
		"172.16.1.2":     true,
		"100.64.0.9":     true, // CGNAT
		"fd00::1":        true, // ULA
		"127.0.0.1":      false,
		"::1":            false,
		"0.0.0.0":        false,
		"169.254.1.1":    false,
		"fe80::1":        false,
		"224.0.0.1":      false,
		"":               false,
		"not-an-ip":      false,
	}
	for in, want := range cases {
		if got := IsUsableClientIP(in); got != want {
			t.Errorf("IsUsableClientIP(%q) = %v, want %v", in, got, want)
		}
	}
	// 原有口径不能被放宽
	if IsPublicClientIP("192.168.55.254") {
		t.Error("IsPublicClientIP 仍应拒绝私网地址")
	}
}
