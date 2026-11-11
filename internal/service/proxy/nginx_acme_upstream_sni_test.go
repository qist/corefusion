package proxy

import (
	"strings"
	"testing"

	"corefusion/internal/model/entity"
)

// nginxServerBlocks 按 `server {` 切出各 server 块，便于断言“某个监听上到底有没有挑战”。
func nginxServerBlocks(cfg string) []string {
	parts := strings.Split(cfg, "    server {")
	if len(parts) <= 1 {
		return nil
	}
	return parts[1:]
}

func acmeBlockOnPort(cfg, listen string) (withACME, total int) {
	for _, blk := range nginxServerBlocks(cfg) {
		if !strings.Contains(blk, "listen "+listen) {
			continue
		}
		total++
		if strings.Contains(blk, "/.well-known/acme-challenge/") {
			withACME++
		}
	}
	return
}

// ACME HTTP-01 只会访问 80：443（TLS）server 块里生成 challenge location 是死代码。
// 同时兜底 :80 server 必须带挑战（只有 TLS 站点时它占着 80）。
func TestNginxACMEChallengeOnlyOnPort80(t *testing.T) {
	tlsOnly := []entity.NginxSite{
		{
			ServerID: 2, Domain: "secure.example.com", Template: "http",
			Path: "/", Upstream: "127.0.0.1:10001",
			CertPath: "cert.pem", KeyPath: "key.pem", Enabled: true,
		},
	}
	cfg := generateFor(t, newNginxTestManager(t, tlsOnly, `{}`))
	if strings.Contains(cfg, "/.well-known/acme-challenge/") {
		t.Fatalf("仅监听 443 的站点不应生成 ACME 挑战 location：\n%s", cfg)
	}

	// 再加一个监听 80 的站点：redirect 块与该 HTTP 块都要能提供挑战，
	// 且跳转必须写在 location / 里（server 级 return 会先于 location 执行，会吃掉挑战请求）。
	withHTTP := append(tlsOnly, entity.NginxSite{
		ServerID: 2, Domain: "plain.example.com", Template: "http",
		Path: "/", Upstream: "127.0.0.1:10002", ListenPort: "80", Enabled: true,
	})
	cfg = generateFor(t, newNginxTestManager(t, withHTTP, `{}`))
	if withACME, total := acmeBlockOnPort(cfg, "80"); total == 0 || withACME != total {
		t.Fatalf("监听 80 的 server 都应带 ACME 挑战：\n%s", cfg)
	}
	if withACME, _ := acmeBlockOnPort(cfg, "443"); withACME != 0 {
		t.Fatalf("443 server 块不应生成 ACME 挑战 location：\n%s", cfg)
	}
	if !strings.Contains(cfg, "location / {\n            return 301 ") {
		t.Fatalf("跳转必须写在 location / 里，否则 server 级 return 会先执行、吃掉 ACME 挑战：\n%s", cfg)
	}
	if strings.Contains(cfg, "\n        return 301 https://") {
		t.Fatalf("不应再生成 server 级 return（会跳过 ACME location）：\n%s", cfg)
	}
}

// 回源 TLS 的 SNI 必须可配置：上游证书域名可能和站点域名完全不同。
func TestNginxUpstreamTLSUsesConfiguredSNI(t *testing.T) {
	sites := []entity.NginxSite{
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
	cfg := generateFor(t, newNginxTestManager(t, sites, `{}`))

	if !strings.Contains(cfg, "proxy_ssl_name internal.example.net;") {
		t.Fatalf("回源 SNI 未使用站点配置的 upstream_sni：\n%s", cfg)
	}
	if !strings.Contains(cfg, "proxy_ssl_name b.example.com;") {
		t.Fatalf("未配置 upstream_sni 时应回退到站点域名：\n%s", cfg)
	}
}
