package certificates

import (
	"reflect"
	"testing"
)

// 核心回归：IP 证书模式下前端只发 domains=[] + ips=[IPv4, IPv6]，
// 后端以前**只读 domains** → 空列表 → "at least one domain required"，
// IP 证书 100% 签不出来（生产实测 2026-11-05）。
func TestMergeSANNamesIPCerts(t *testing.T) {
	cases := []struct {
		name    string
		domains []string
		ips     []string
		want    []string
	}{
		{
			name: "IP 证书：IPv4 + IPv6 同时签发",
			ips:  []string{"203.0.113.10", "2001:db8::1"},
			want: []string{"203.0.113.10", "2001:db8::1"},
		},
		{
			name:    "域名证书不受影响",
			domains: []string{"example.com", "*.example.org"},
			want:    []string{"example.com", "*.example.org"},
		},
		{
			name:    "混合：域名在前、IP 在后",
			domains: []string{"example.com"},
			ips:     []string{"203.0.113.10"},
			want:    []string{"example.com", "203.0.113.10"},
		},
		{
			name:    "去重 + 去空白 + 大小写归一",
			domains: []string{" Example.com ", "", "example.com"},
			ips:     []string{"EXAMPLE.COM", "203.0.113.10"},
			want:    []string{"Example.com", "203.0.113.10"},
		},
		{
			name: "两者都空",
			want: nil,
		},
	}

	for _, c := range cases {
		got := mergeSANNames(c.domains, c.ips)
		if len(got) == 0 && len(c.want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Fatalf("%s ✗：mergeSANNames(%v, %v) = %v，应为 %v ✓",
				c.name, c.domains, c.ips, got, c.want)
		}
	}
}
