package passkey

import "testing"

// TestResolveRPID 覆盖设计核心的 RP ID 白名单命中规则：
//   - host == rp 精确命中；
//   - host 是 rp 的子域（多一层、多两层）命中；
//   - 取最短匹配（最通用的可注册域）→ 多子域共用一把 passkey；
//   - 伪子域（evil-example.com）不命中 —— 必须有 "." 边界；
//   - IP 字面量不命中任何域名条目；
//   - 空白名单 / 空 host 均不命中。
func TestResolveRPID(t *testing.T) {
	rpids := []string{"example.com", "panel.example.com", "localhost", "example.co.uk"}

	cases := []struct {
		name string
		host string
		want string
	}{
		{"精确命中主域", "example.com", "example.com"},
		{"精确命中白名单子域", "panel.example.com", "panel.example.com"},
		{"paneltest 不是 panel.example.com 子域", "paneltest.example.com", "example.com"},
		{"一层子域命中主域", "www.example.com", "example.com"},
		{"两层子域命中主域", "a.b.example.com", "example.com"},
		{"面板子域带端口", "panel.example.com:8443", "panel.example.com"},
		{"IP 字面量不命中", "192.168.1.10", ""},
		{"远程 IP 字面量不命中", "8.8.8.8", ""},
		{"localhost 带端口", "localhost:8080", "localhost"},
		{"伪子域（无点边界）不命中", "evil-example.com", ""},
		{"伪子域带前缀不命中", "notexample.com", ""},
		{"eTLD+1 写法（多段后缀）", "shop.example.co.uk", "example.co.uk"},
		{"FQDN 结尾点归一化", "panel.example.com.", "panel.example.com"},
		{"大小写归一化", "PANEL.EXAMPLE.COM", "panel.example.com"},
		{"空 host 不命中", "", ""},
		{"端口但 host 为 IPv6 loopback 不命中域名条目", "[::1]:8080", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveRPIDFromList(rpids, tc.host)
			if got != tc.want {
				t.Fatalf("resolveRPIDFromList(%v, %q) = %q, want %q", rpids, tc.host, got, tc.want)
			}
		})
	}

	if got := resolveRPIDFromList(nil, "example.com"); got != "" {
		t.Fatalf("空白名单应返回空串，got %q", got)
	}

	// 白名单只有子域条目时，其更深的子域也能命中该条目
	if got := resolveRPIDFromList([]string{"panel.example.com"}, "sub.panel.example.com"); got != "panel.example.com" {
		t.Fatalf("子域应命中白名单子域条目，got %q", got)
	}
}

// TestNormalizeHost 验证端口剥离与边界字符处理。
func TestNormalizeHost(t *testing.T) {
	cases := []struct{ in, want string }{
		{"panel.example.com", "panel.example.com"},
		{"panel.example.com:8443", "panel.example.com"},
		{":8080", ""},
		{"192.168.1.1:8080", "192.168.1.1"},
		{"[::1]:8080", "::1"},
		{"::1", "::1"},
		{"localhost.", "localhost"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := normalizeHost(tc.in); got != tc.want {
			t.Fatalf("normalizeHost(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestIsSecureContext 覆盖 secure context 判定（WebAuthn 硬前提）。
func TestIsSecureContext(t *testing.T) {
	if !IsSecureContext("https", "panel.example.com") {
		t.Fatal("https 必须是安全上下文")
	}
	if !IsSecureContext("http", "localhost") {
		t.Fatal("http://localhost 是安全上下文（浏览器特例）")
	}
	if IsSecureContext("http", "192.168.1.10") {
		t.Fatal("http + 私网 IP 不是安全上下文")
	}
	if IsSecureContext("http", "panel.example.com") {
		t.Fatal("http + 公网域名不是安全上下文")
	}
}
