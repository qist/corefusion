package proxy

import (
	"strings"
	"testing"

	"corefusion/internal/model/entity"
)

// ACME HTTP-01 只访问 80：带证书的 site block（443）里不应生成挑战路由。
func TestCaddyACMEOnlyOnPort80(t *testing.T) {
	tlsOnly := []entity.CaddySite{
		{
			ServerID: 2, Domain: "secure.example.com", Template: "http",
			Path: "/", Upstream: "127.0.0.1:10001",
			CertPath: "cert.pem", KeyPath: "key.pem", Enabled: true,
		},
	}
	cfg := generateCaddyFor(t, newCaddyTestManager(t, tlsOnly, nil))
	if strings.Contains(cfg, "@acme") {
		t.Fatalf("仅监听 443 的站点不应生成 ACME 挑战路由：\n%s", cfg)
	}

	// 用显式 :80 的站点（Caddy 裸域名默认走 443，不会监听 80）。
	withHTTP := append(tlsOnly, entity.CaddySite{
		ServerID: 2, Domain: "plain.example.com:80", Template: "http",
		Path: "/", Upstream: "127.0.0.1:10002", Enabled: true,
	})
	cfg = generateCaddyFor(t, newCaddyTestManager(t, withHTTP, nil))
	if !strings.Contains(cfg, "@acme") {
		t.Fatalf("监听 80 的站点应生成 ACME 挑战路由：\n%s", cfg)
	}
	// Caddy 指令排序里 redir 在 handle 之前：直接写 redir 会把 ACME 挑战也 302 掉
	// （adapt 后 acme 路由会被丢弃），所以跳转必须放进 handle 兜底块。
	if strings.Contains(cfg, "\n\tredir https://") {
		t.Fatalf("redirect 块不应直接写 redir（会先于 ACME handle 执行）：\n%s", cfg)
	}
	if !strings.Contains(cfg, "\thandle {\n\t\tredir https://") {
		t.Fatalf("redirect 块应使用 handle 兜底块包住 redir：\n%s", cfg)
	}
}

// caddy 非 grpc 回源 TLS 也必须显式写 transport/http/tls_server_name，
// 否则会用 to 里的 127.0.0.1 当 SNI，上游按 SNI 选证书时拿不到正确的证书。
func TestCaddyUpstreamTLSUsesConfiguredSNI(t *testing.T) {
	sites := []entity.CaddySite{
		{
			ServerID: 2, Domain: "a.example.com", Template: "http",
			Path: "/", Upstream: "127.0.0.1:10001",
			UpstreamTLS: true, UpstreamSNI: "internal.example.net", Enabled: true,
		},
		{
			ServerID: 2, Domain: "b.example.com", Template: "http",
			Path: "/", Upstream: "127.0.0.1:10002",
			UpstreamTLS: true, // 未配置 → 回退站点域名
			Enabled:     true,
		},
	}
	cfg := generateCaddyFor(t, newCaddyTestManager(t, sites, nil))

	for _, want := range []string{
		"to https://127.0.0.1:10001",
		"transport http {",
		"tls_server_name internal.example.net",
		"tls_insecure_skip_verify",
		"tls_server_name b.example.com",
	} {
		if !strings.Contains(cfg, want) {
			t.Fatalf("缺少 %q：\n%s", want, cfg)
		}
	}
}
