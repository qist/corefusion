package bootstrap

import (
	"strings"
	"testing"
)

// IP 证书模式：SAN 里的 IPv4 + IPv6 都要出现在访问地址里（IPv6 带方括号）。
func TestAccessURLsIPCertShowsBothFamilies(t *testing.T) {
	d := &Draft{
		ListenAddr:  "0.0.0.0",
		ListenPort:  "10111",
		TLSMode:     "acme",
		TLSCertPath: "/tmp/cert.pem",
		TLSKeyPath:  "/tmp/key.pem",
		TLSDomains:  []string{"203.0.113.10", "2001:db8::1"},
	}
	got := d.accessURLs("", d.ListenPort, true, false, "")
	want := []string{"https://203.0.113.10:10111", "https://[2001:db8::1]:10111"}
	if len(got) != len(want) {
		t.Fatalf("accessURLs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("accessURLs[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// 通配监听地址不能出现在访问链接里；没有域名也没有 IP SAN 时只回退 loopback
// （未签发证书时面板只监听 127.0.0.1，给公网 IP 会连不上）。
func TestAccessHostsNeverExposeWildcard(t *testing.T) {
	d := &Draft{ListenAddr: "0.0.0.0", TLSMode: "acme", TLSCertPath: "/tmp/cert.pem", TLSKeyPath: "/tmp/key.pem"}
	for _, host := range d.accessHosts("") {
		if isWildcardHost(host) {
			t.Fatalf("wildcard host leaked into access hosts: %q", host)
		}
	}
	got := d.accessURLs("", "10111", true, false, "")
	want := []string{"https://127.0.0.1:10111"}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("accessURLs = %v, want %v", got, want)
	}
}

// 门户关闭时配置清单不应再给门户访问地址；同端口订阅要带 /sub。
func TestSummaryAccessPortalHiddenAndSubPath(t *testing.T) {
	dir := t.TempDir()
	d := NewDraft(dir)
	d.ListenAddr = "0.0.0.0"
	d.ListenPort = "10111"
	d.SubscriptionPort = "10111" // 与面板同端口 → 挂在 /sub
	d.PortalPort = "10111"
	d.PortalEnabled = false
	d.TLSMode = "acme"
	d.TLSCertPath = "/tmp/cert.pem"
	d.TLSKeyPath = "/tmp/key.pem"
	d.TLSDomains = []string{"203.0.113.10"}

	sum := Summary(dir, d)
	access, ok := sum["access"].(map[string][]string)
	if !ok {
		t.Fatalf("summary access type = %T", sum["access"])
	}
	if len(access["portal"]) != 0 {
		t.Fatalf("portal access should be empty when disabled, got %v", access["portal"])
	}
	if len(access["panel"]) != 1 || access["panel"][0] != "https://203.0.113.10:10111" {
		t.Fatalf("panel access = %v", access["panel"])
	}
	if len(access["subscription"]) != 1 || !strings.HasSuffix(access["subscription"][0], "/sub") {
		t.Fatalf("subscription access = %v, want .../sub", access["subscription"])
	}
}
