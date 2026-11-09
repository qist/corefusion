package proxy

import (
	"context"
	"strings"
	"testing"

	"corefusion/internal/model/entity"
)

// 编辑已有 SNI 站点并修改监听地址时，预览必须按 ID 替换旧行；
// 否则旧的 443 与新的 8445 会同时出现，看起来像"生成两份配置"。
func TestNginxPreviewSiteReplacesByID(t *testing.T) {
	sites := []entity.NginxSite{
		{ServerID: 2, Domain: "a.example.com", Template: "sni", Path: "443", SNITarget: "127.0.0.1:9001", Enabled: true},
	}
	m := newNginxTestManager(t, sites, `{"stream_sni_enabled":true}`)
	stored, err := m.siteRepo.List(context.Background(), 2)
	if err != nil || len(stored) != 1 {
		t.Fatalf("读取测试站点失败：%v %+v", err, stored)
	}

	draft := stored[0]
	draft.Path = "127.0.0.1:8445"
	draft.ListenPort = "127.0.0.1:8445"
	cfg, err := m.PreviewSiteConfig(context.Background(), 2, draft)
	if err != nil {
		t.Fatalf("预览失败：%v", err)
	}
	if strings.Contains(cfg, "SNI routing on 443") {
		t.Fatalf("编辑监听地址后不应保留旧 443 分组：\n%s", cfg)
	}
	if n := strings.Count(cfg, "SNI routing on "); n != 1 {
		t.Fatalf("预览应只有一份 SNI 配置，实际 %d 份：\n%s", n, cfg)
	}
	if !strings.Contains(cfg, "SNI routing on 127.0.0.1:8445") {
		t.Fatalf("预览缺少新的监听地址：\n%s", cfg)
	}
}

// 同一监听地址的等价写法（"" / "443" / "0.0.0.0:443"）必须归到同一个 SNI 组，
// 否则编辑一条规则就会多生成一个 listen 443，nginx 直接起不来。
func TestNginxSNIEquivalentListenAddressesCollapseToOneGroup(t *testing.T) {
	sites := []entity.NginxSite{
		{ServerID: 2, Domain: "a.example.com", Template: "sni", Path: "", ListenPort: "443", SNITarget: "127.0.0.1:9001", Enabled: true},
		{ServerID: 2, Domain: "b.example.com", Template: "sni", Path: "443", SNITarget: "127.0.0.1:9002", Enabled: true},
		{ServerID: 2, Domain: "fallback", Template: "sni", Path: "0.0.0.0:443", SNITarget: "127.0.0.1:8443", Enabled: true},
	}
	cfg := generateFor(t, newNginxTestManager(t, sites,
		`{"stream_sni_enabled":true,"stream_ipv6":true}`))

	if n := strings.Count(cfg, "    listen 443 reuseport"); n != 1 {
		t.Fatalf("监听 443 应只生成一次，实际 %d 次：\n%s", n, cfg)
	}
	if n := strings.Count(cfg, "    listen [::]:443 reuseport"); n != 1 {
		t.Fatalf("IPv6 443 应只生成一次，实际 %d 次：\n%s", n, cfg)
	}
	if n := strings.Count(cfg, "# SNI routing on 443"); n != 1 {
		t.Fatalf("SNI 443 分组应只有一个，实际 %d 个：\n%s", n, cfg)
	}
}

// 历史数据里同时存在通配 443 和 127.0.0.1:443 时，只保留一个 listen，
// 并把指定 IP 组里的 SNI 域名并入通配组（否则 nginx duplicate listen）。
func TestNginxSNIWildcardAndSpecificListenMerge(t *testing.T) {
	sites := []entity.NginxSite{
		{ServerID: 2, Domain: "a.example.com", Template: "sni", Path: "443", SNITarget: "127.0.0.1:9001", Enabled: true},
		{ServerID: 2, Domain: "b.example.com", Template: "sni", Path: "127.0.0.1:443", SNITarget: "127.0.0.1:9002", Enabled: true},
	}
	cfg := generateFor(t, newNginxTestManager(t, sites, `{"stream_sni_enabled":true}`))

	if n := strings.Count(cfg, "    listen 443 reuseport"); n != 1 {
		t.Fatalf("通配与指定 IP 冲突时应只保留一个 443 监听，实际 %d 个：\n%s", n, cfg)
	}
	if strings.Contains(cfg, "listen 127.0.0.1:443") {
		t.Fatalf("不应再生成 127.0.0.1:443 监听：\n%s", cfg)
	}
	for _, domain := range []string{"a.example.com sni_", "b.example.com sni_"} {
		if !strings.Contains(cfg, domain) {
			t.Fatalf("合并后 SNI 域名 %q 不应丢失：\n%s", domain, cfg)
		}
	}
}

// SNI 规则自身开启 IPv6 时，即使全局 stream_ipv6 关闭也要生成 [::] 监听。
func TestNginxSNIPerSiteIPv6(t *testing.T) {
	sites := []entity.NginxSite{
		{ServerID: 2, Domain: "a.example.com", Template: "sni", Path: "127.0.0.1:443",
			SNITarget: "127.0.0.1:9001", Enabled: true, EnableIpv6: true},
	}
	cfg := generateFor(t, newNginxTestManager(t, sites, `{"stream_sni_enabled":true}`))

	for _, want := range []string{
		"    listen 127.0.0.1:443 reuseport",
		"    listen [::]:443 reuseport",
	} {
		if !strings.Contains(cfg, want) {
			t.Fatalf("缺少 %q：\n%s", want, cfg)
		}
	}
}

// 全局 SNI 默认监听地址未配置时回落到 443；规则本身不写监听地址也生效。
func TestNginxSNIListenAddrDefault(t *testing.T) {
	sites := []entity.NginxSite{
		{ServerID: 2, Domain: "a.example.com", Template: "sni", SNITarget: "127.0.0.1:9001", Enabled: true},
	}
	cfg := generateFor(t, newNginxTestManager(t, sites, `{"stream_sni_enabled":true}`))
	if !strings.Contains(cfg, "    listen 443 reuseport") {
		t.Fatalf("默认监听地址应为 443：\n%s", cfg)
	}
}

// 多个指定 IPv4 监听同一端口时，[::]:端口 通配监听只能出现一次，
// 否则 nginx 报 duplicate listen options。
func TestNginxSNIMultiIPv4SingleIPv6Wildcard(t *testing.T) {
	sites := []entity.NginxSite{
		{ServerID: 2, Domain: "a.example.com", Template: "sni", Path: "10.0.0.1:443", SNITarget: "127.0.0.1:9001", Enabled: true},
		{ServerID: 2, Domain: "b.example.com", Template: "sni", Path: "10.0.0.2:443", SNITarget: "127.0.0.1:9002", Enabled: true},
	}
	cfg := generateFor(t, newNginxTestManager(t, sites, `{"stream_sni_enabled":true,"stream_ipv6":true}`))

	if n := strings.Count(cfg, "    listen [::]:443 reuseport"); n != 1 {
		t.Fatalf("同端口 [::]:443 应只生成一次，实际 %d 次：\n%s", n, cfg)
	}
	for _, want := range []string{"    listen 10.0.0.1:443 reuseport", "    listen 10.0.0.2:443 reuseport"} {
		if !strings.Contains(cfg, want) {
			t.Fatalf("缺少 %q：\n%s", want, cfg)
		}
	}
}

// HTTP/HTTPS 侧的 IPv6 监听也要支持指定地址，而不是固定 [::]。
func TestNginxHTTPIPv6ListenAddr(t *testing.T) {
	sites := []entity.NginxSite{
		{ServerID: 2, Domain: "v6.example.com", ListenPort: "443", CertPath: "cert.pem", KeyPath: "key.pem",
			Enabled: true, Path: "/", Upstream: "127.0.0.1:8080"},
		{ServerID: 2, Domain: "v6.example.com", ListenPort: "80",
			Enabled: true, Path: "/", Upstream: "127.0.0.1:8080"},
	}
	cfg := generateFor(t, newNginxTestManager(t, sites, `{"ipv6":true,"ipv6_listen_addr":"2001:db8::1"}`))
	for _, want := range []string{
		"        listen [2001:db8::1]:443 ssl;",
		"        listen [2001:db8::1]:80;",
	} {
		if !strings.Contains(cfg, want) {
			t.Fatalf("缺少 %q：\n%s", want, cfg)
		}
	}
	if strings.Contains(cfg, "listen [::]:443") {
		t.Fatalf("指定地址后不应再生成通配 [::]:443：\n%s", cfg)
	}
}

// stream（SNI）侧的 IPv6 监听地址同样可指定。
func TestNginxStreamIPv6ListenAddr(t *testing.T) {
	sites := []entity.NginxSite{
		{ServerID: 2, Domain: "a.example.com", Template: "sni", Path: "443", SNITarget: "127.0.0.1:9001", Enabled: true},
	}
	cfg := generateFor(t, newNginxTestManager(t, sites, `{"stream_sni_enabled":true,"stream_ipv6":true,"stream_ipv6_listen_addr":"::1"}`))
	if !strings.Contains(cfg, "    listen [::1]:443 reuseport") {
		t.Fatalf("stream 应监听指定 IPv6 地址 [::1]:443：\n%s", cfg)
	}
}

// 全局 IPv6 监听地址带端口时：HTTPS/QUIC 用该端口，HTTP 仍用自身端口（否则撞端口）。
func TestNginxGlobalIPv6ListenAddrWithPort(t *testing.T) {
	sites := []entity.NginxSite{
		{ServerID: 2, Domain: "v6.example.com", ListenPort: "443", CertPath: "cert.pem", KeyPath: "key.pem",
			Enabled: true, Path: "/", Upstream: "127.0.0.1:8080"},
		{ServerID: 2, Domain: "v6.example.com", ListenPort: "80",
			Enabled: true, Path: "/", Upstream: "127.0.0.1:8080"},
	}
	cfg := generateFor(t, newNginxTestManager(t, sites, `{"ipv6":true,"ipv6_listen_addr":"[2001:db8::1]:8443"}`))
	for _, want := range []string{
		"        listen [2001:db8::1]:8443 ssl;",
		"        listen [2001:db8::1]:80;",
	} {
		if !strings.Contains(cfg, want) {
			t.Fatalf("缺少 %q：\n%s", want, cfg)
		}
	}
	if strings.Contains(cfg, "listen [2001:db8::1]:8443;") {
		t.Fatalf("HTTP 侧不应复用 HTTPS 的端口：\n%s", cfg)
	}
}

// stream（SNI）IPv6 监听地址带端口时使用指定端口。
func TestNginxStreamIPv6ListenAddrWithPort(t *testing.T) {
	sites := []entity.NginxSite{
		{ServerID: 2, Domain: "a.example.com", Template: "sni", Path: "443", SNITarget: "127.0.0.1:9001", Enabled: true},
	}
	cfg := generateFor(t, newNginxTestManager(t, sites, `{"stream_sni_enabled":true,"stream_ipv6":true,"stream_ipv6_listen_addr":"[2001:db8::1]:8443"}`))
	if !strings.Contains(cfg, "    listen [2001:db8::1]:8443 reuseport") {
		t.Fatalf("stream 应监听 [2001:db8::1]:8443：\n%s", cfg)
	}
	if strings.Contains(cfg, "    listen [2001:db8::1]:443 reuseport") {
		t.Fatalf("指定端口后不应再生成 443：\n%s", cfg)
	}
}

// 非法/空地址回退到 [::]，避免生成非法 listen。
func TestNginxIPv6ListenAddrFallback(t *testing.T) {
	if got := ipv6ListenHost(""); got != "::" {
		t.Fatalf("空值应回退 ::，实际 %q", got)
	}
	if got := ipv6ListenHost("0.0.0.0"); got != "::" {
		t.Fatalf("IPv4 应回退 ::，实际 %q", got)
	}
	if got := ipv6ListenHost("[::1]:443"); got != "::1" {
		t.Fatalf("应取主机部分 ::1，实际 %q", got)
	}
	if got := ipv6Listen("::1", "443"); got != "[::1]:443" {
		t.Fatalf("ipv6Listen 生成错误：%q", got)
	}
	// port 传入完整的 IPv4 监听地址时必须只取端口，否则会变成 [::1]:127.0.0.1:443。
	if got := ipv6Listen("::1", "127.0.0.1:443"); got != "[::1]:443" {
		t.Fatalf("port 含 IPv4 地址时应只取端口，实际 %q", got)
	}
}

// 站点级 IPv6 监听地址优先于全局；带端口时使用站点指定的端口。
func TestNginxSiteIPv6ListenAddrOverride(t *testing.T) {
	sites := []entity.NginxSite{
		{ServerID: 2, Domain: "v6.example.com", ListenPort: "127.0.0.1:443", CertPath: "cert.pem", KeyPath: "key.pem",
			Enabled: true, Path: "/", Upstream: "127.0.0.1:8080", EnableIpv6: true, IPv6ListenAddr: "[2001:db8::1]:8443"},
	}
	cfg := generateFor(t, newNginxTestManager(t, sites, `{"ipv6":true,"ipv6_listen_addr":"::1"}`))
	if !strings.Contains(cfg, "        listen [2001:db8::1]:8443 ssl;") {
		t.Fatalf("站点级 IPv6 监听地址未生效：\n%s", cfg)
	}
	if strings.Contains(cfg, "listen [::1]:443 ssl") {
		t.Fatalf("不应再使用全局 IPv6 地址：\n%s", cfg)
	}
}

// 站点级 IPv6 只写主机时，端口沿用站点监听的端口。
func TestNginxSiteIPv6ListenAddrHostOnly(t *testing.T) {
	sites := []entity.NginxSite{
		{ServerID: 2, Domain: "v6.example.com", ListenPort: "127.0.0.1:443", CertPath: "cert.pem", KeyPath: "key.pem",
			Enabled: true, Path: "/", Upstream: "127.0.0.1:8080", EnableIpv6: true, IPv6ListenAddr: "2001:db8::2"},
	}
	cfg := generateFor(t, newNginxTestManager(t, sites, `{"ipv6":true}`))
	if !strings.Contains(cfg, "        listen [2001:db8::2]:443 ssl;") {
		t.Fatalf("站点级 IPv6 主机未生效：\n%s", cfg)
	}
}

// IPv4 监听地址带 IP、IPv6 指定地址时，生成的 IPv6 listen 只能有端口。
func TestNginxHTTPIPv6WithSpecificIPv4Listen(t *testing.T) {
	sites := []entity.NginxSite{
		{ServerID: 2, Domain: "v6.example.com", ListenPort: "127.0.0.1:443", CertPath: "cert.pem", KeyPath: "key.pem",
			Enabled: true, Path: "/", Upstream: "127.0.0.1:8080"},
	}
	cfg := generateFor(t, newNginxTestManager(t, sites, `{"ipv6":true,"ipv6_listen_addr":"[::1]:443"}`))
	for _, want := range []string{
		"        listen 127.0.0.1:443 ssl;",
		"        listen [::1]:443 ssl;",
	} {
		if !strings.Contains(cfg, want) {
			t.Fatalf("缺少 %q：\n%s", want, cfg)
		}
	}
	if strings.Contains(cfg, "[::1]:127.0.0.1:443") {
		t.Fatalf("IPv6 监听地址不应拼接 IPv4 监听串：\n%s", cfg)
	}
}

// 生产同款配置回归：默认 HTTPS 端口与 IPv6 监听地址都带端口，SNI 监听绑定具体 IPv4。
func TestNginxProdIPv6ListenRegression(t *testing.T) {
	sites := []entity.NginxSite{
		{ServerID: 2, Domain: "app.example.com", ListenPort: "127.0.0.1:443", CertPath: "cert.pem", KeyPath: "key.pem",
			Enabled: true, Path: "/", Upstream: "127.0.0.1:8080"},
		{ServerID: 2, Domain: "sni.example.com", Template: "sni", Path: "23.254.233.185:443", SNITarget: "127.0.0.1:9001", Enabled: true},
	}
	cfg := generateFor(t, newNginxTestManager(t, sites,
		`{"ipv6":true,"ipv6_listen_addr":"[::1]:443","stream_sni_enabled":true,"stream_ipv6":true,"stream_ipv6_listen_addr":"[2001:db8::1]:443","sni_listen_addr":"23.254.233.185:443"}`))
	for _, want := range []string{
		"        listen [::1]:443 ssl;",
		"    listen [2001:db8::1]:443 reuseport",
	} {
		if !strings.Contains(cfg, want) {
			t.Fatalf("缺少 %q：\n%s", want, cfg)
		}
	}
	for _, bad := range []string{"[::1]:127.0.0.1:443", "[2001:db8::1]:23.254.233.185:443"} {
		if strings.Contains(cfg, bad) {
			t.Fatalf("生成了非法监听 %q：\n%s", bad, cfg)
		}
	}
}
