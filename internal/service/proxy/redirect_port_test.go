package proxy

import (
	"strings"
	"testing"

	"corefusion/internal/model/entity"
)

// HTTP→HTTPS 跳转默认不能把内部监听端口（8443）写进 Location：
// 客户端只走 443，带 8443 既暴露内部端口、CDN 后面还会连不上。
func TestNginxRedirectDropsInternalPortByDefault(t *testing.T) {
	sites := []entity.NginxSite{
		{
			ServerID: 2, Domain: "sub.example.com", Template: "http",
			Path: "/", Upstream: "127.0.0.1:8443", ListenPort: "127.0.0.1:8443",
			CertPath: "cert.pem", KeyPath: "key.pem", Enabled: true,
		},
		{
			ServerID: 2, Domain: "plain.example.com", Template: "http",
			Path: "/", Upstream: "127.0.0.1:10002", ListenPort: "80", Enabled: true,
		},
	}
	cfg := generateFor(t, newNginxTestManager(t, sites, `{}`))

	if !strings.Contains(cfg, "return 301 https://sub.example.com$request_uri;") {
		t.Fatalf("默认应跳到不带端口的 https://sub.example.com：\n%s", cfg)
	}
	if strings.Contains(cfg, "sub.example.com:8443") || strings.Contains(cfg, ":8443$request_uri") {
		t.Fatalf("跳转不应暴露内部端口 8443：\n%s", cfg)
	}
}

// 站点显式打开「保留端口」时才写端口。
func TestNginxRedirectKeepPortOptIn(t *testing.T) {
	sites := []entity.NginxSite{
		{
			ServerID: 2, Domain: "sub.example.com", Template: "http",
			Path: "/", Upstream: "127.0.0.1:8443", ListenPort: "127.0.0.1:8443",
			CertPath: "cert.pem", KeyPath: "key.pem", Enabled: true,
			RedirectKeepPort: true,
		},
		{
			ServerID: 2, Domain: "plain.example.com", Template: "http",
			Path: "/", Upstream: "127.0.0.1:10002", ListenPort: "80", Enabled: true,
		},
	}
	cfg := generateFor(t, newNginxTestManager(t, sites, `{}`))

	if !strings.Contains(cfg, "return 301 https://sub.example.com:8443$request_uri;") {
		t.Fatalf("显式保留端口时应写 :8443：\n%s", cfg)
	}
}

// caddy 站点地址里带端口（sub.example.com:8443）时，跳转默认同样要剥掉端口。
func TestCaddyRedirectDropsInternalPortByDefault(t *testing.T) {
	sites := []entity.CaddySite{
		{
			ServerID: 2, Domain: "sub.example.com:8443", Template: "http",
			Path: "/", Upstream: "127.0.0.1:8443",
			CertPath: "cert.pem", KeyPath: "key.pem", Enabled: true,
		},
		{
			ServerID: 2, Domain: "plain.example.com:80", Template: "http",
			Path: "/", Upstream: "127.0.0.1:10002", Enabled: true,
		},
	}
	cfg := generateCaddyFor(t, newCaddyTestManager(t, sites, nil))

	if !strings.Contains(cfg, "redir https://sub.example.com{uri} 302") {
		t.Fatalf("默认应跳到不带端口的 https://sub.example.com：\n%s", cfg)
	}
	if strings.Contains(cfg, "sub.example.com:8443{uri}") {
		t.Fatalf("跳转不应暴露内部端口 8443：\n%s", cfg)
	}
}

// caddy 站点显式打开「保留端口」时保留原地址端口。
func TestCaddyRedirectKeepPortOptIn(t *testing.T) {
	sites := []entity.CaddySite{
		{
			ServerID: 2, Domain: "sub.example.com:8443", Template: "http",
			Path: "/", Upstream: "127.0.0.1:8443",
			CertPath: "cert.pem", KeyPath: "key.pem", Enabled: true,
			RedirectKeepPort: true,
		},
		{
			ServerID: 2, Domain: "plain.example.com:80", Template: "http",
			Path: "/", Upstream: "127.0.0.1:10002", Enabled: true,
		},
	}
	cfg := generateCaddyFor(t, newCaddyTestManager(t, sites, nil))

	if !strings.Contains(cfg, "redir https://sub.example.com:8443{uri} 302") {
		t.Fatalf("显式保留端口时应写 :8443：\n%s", cfg)
	}
}
