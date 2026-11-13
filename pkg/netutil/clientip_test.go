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
