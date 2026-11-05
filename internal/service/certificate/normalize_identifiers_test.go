package certificate

import (
	"reflect"
	"testing"
)

// 重新签发要覆盖旧证书：标识符必须先归一，否则 `Example.com` 与 `example.com`
// 会变成两条记录、两个落盘目录。
func TestNormalizeIdentifiers(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "域名转小写并保持顺序（主域名仍在第一位）",
			in:   []string{"Example.com", "WWW.Example.com"},
			want: []string{"example.com", "www.example.com"},
		},
		{
			name: "IP 证书：IPv4 + IPv6 都保留",
			in:   []string{"203.0.113.10", "2001:DB8::1"},
			want: []string{"203.0.113.10", "2001:db8::1"},
		},
		{
			name: "去空白、去空项、去重（大小写视为同一个）",
			in:   []string{"  Example.com ", "", "example.com", "EXAMPLE.COM"},
			want: []string{"example.com"},
		},
		{
			name: "混合 SAN",
			in:   []string{"Example.com", "203.0.113.10", "2001:db8::1"},
			want: []string{"example.com", "203.0.113.10", "2001:db8::1"},
		},
		{
			name: "空输入",
			in:   nil,
			want: nil,
		},
	}

	for _, c := range cases {
		got := normalizeIdentifiers(c.in)
		if len(got) == 0 && len(c.want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Fatalf("%s：normalizeIdentifiers(%v) = %v，应为 %v", c.name, c.in, got, c.want)
		}
	}
}
