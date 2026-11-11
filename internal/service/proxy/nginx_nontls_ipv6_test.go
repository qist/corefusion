package proxy

import (
	"context"
	"strings"
	"testing"

	"corefusion/internal/model/entity"
)

// 生产回归：non-tls.invalid 是系统内部站点，默认**不继承**全局 IPv6；
// 只有站点显式 enable_ipv6 时才生成，且必须复用站点自己的端口（[::1]:8444）。
//
// 若误用全局 IPv6 地址里的端口（[::1]:443），会与 TLS 站点的
// `listen [::1]:443 ssl proxy_protocol` 冲突：
//   - nginx: "protocol options redefined for [::1]:443"
//   - nginx: "bind() to [::1]:443 failed (98: Address already in use)"
func TestNginxConfigNonTLSIPv6UsesSitePort(t *testing.T) {
	sites := []entity.NginxSite{
		{
			ServerID: 1, Domain: "tls.example.com", ListenPort: "127.0.0.1:443",
			CertPath: "certs/example.com/cert.pem", KeyPath: "certs/example.com/key.pem",
			ProxyProtocol: true, Template: "http", Path: "/", Upstream: "http://127.0.0.1:8080",
			Enabled: true,
		},
		{
			ServerID: 1, Domain: "non-tls.invalid", Template: "return_code",
			Path: "/", ListenPort: "127.0.0.1:8444", ProxyProtocol: true,
			BackendPath: "403", Enabled: true, EnableIpv6: true,
		},
	}
	m := newNginxTestManager(t, sites,
		`{"ipv6": true, "ipv6_listen_addr": "::1", "default_https_port": "127.0.0.1:443", "stream_sni_enabled": true, "stream_nontls_target": "127.0.0.1:8444"}`)
	cfg, err := m.GenerateConfigForServer(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(cfg, "listen [::1]:8444 proxy_protocol;") {
		t.Fatalf("non-tls 站点显式开启 IPv6 后缺少 [::1]:8444 监听：\n%s", cfg)
	}
	// 同一个 IPv6 端口不能既被 TLS server 以 ssl 监听、又被 non-tls server 以明文监听。
	if strings.Contains(cfg, "listen [::1]:443 proxy_protocol;") {
		t.Fatalf("生成了与 TLS 端口冲突的 [::1]:443 proxy_protocol：\n%s", cfg)
	}
}

// 全局监听拆成 http/https × v4/v6 四项：站点未显式指定 listen_port 时各用各的地址，
// 不会出现 HTTP 站点复用 HTTPS 的 IPv6 端口（反之亦然）。
func TestNginxConfigGlobalListenSplit(t *testing.T) {
	sites := []entity.NginxSite{
		{
			ServerID: 1, Domain: "tls.example.com", CertPath: "cert.pem", KeyPath: "key.pem",
			Enabled: true, Path: "/", Upstream: "127.0.0.1:8080",
		},
		{
			ServerID: 1, Domain: "plain.example.com",
			Enabled: true, Path: "/", Upstream: "127.0.0.1:8080",
		},
	}
	m := newNginxTestManager(t, sites,
		`{"ipv6":true,"http_listen":"127.0.0.1:8080","https_listen":"127.0.0.1:8443","http_listen_v6":"[::1]:8080","https_listen_v6":"[::1]:8443"}`)
	cfg, err := m.GenerateConfigForServer(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"listen 127.0.0.1:8443 ssl;",
		"listen [::1]:8443 ssl;",
		"listen 127.0.0.1:8080;",
		"listen [::1]:8080;",
	} {
		if !strings.Contains(cfg, want) {
			t.Fatalf("缺少 %q：\n%s", want, cfg)
		}
	}
}

// 系统内部站点（*.invalid）未显式开启 IPv6 时，即使全局 IPv6 打开也不生成 v6 监听。
func TestNginxConfigInternalSiteSkipsGlobalIPv6(t *testing.T) {
	sites := []entity.NginxSite{
		{
			ServerID: 1, Domain: "non-tls.invalid", Template: "return_code",
			Path: "/", ListenPort: "127.0.0.1:8444", ProxyProtocol: true,
			BackendPath: "403", Enabled: true,
		},
	}
	m := newNginxTestManager(t, sites,
		`{"ipv6": true, "http_listen_v6": "[::1]:80", "https_listen_v6": "[::1]:443"}`)
	cfg, err := m.GenerateConfigForServer(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(cfg, "listen [::1]:8444") {
		t.Fatalf("系统站点未开 enable_ipv6 却生成了 IPv6 监听：\n%s", cfg)
	}
}
