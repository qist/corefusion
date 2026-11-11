package proxy

import (
	"context"
	"strings"
	"testing"

	"corefusion/internal/model/entity"
)

// 生产回归：non-tls.invalid 站点（listen 127.0.0.1:8444）在全局 IPv6 开启时，
// IPv6 监听必须复用**站点自己的端口**（[::1]:8444）。
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
			BackendPath: "403", Enabled: true,
		},
	}
	m := newNginxTestManager(t, sites,
		`{"ipv6": true, "ipv6_listen_addr": "::1", "default_https_port": "127.0.0.1:443", "stream_sni_enabled": true, "stream_nontls_target": "127.0.0.1:8444"}`)
	cfg, err := m.GenerateConfigForServer(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(cfg, "listen [::1]:8444 proxy_protocol;") {
		t.Fatalf("non-tls 站点缺少 [::1]:8444 监听：\n%s", cfg)
	}
	// 同一个 IPv6 端口不能既被 TLS server 以 ssl 监听、又被 non-tls server 以明文监听。
	if strings.Contains(cfg, "listen [::1]:443 proxy_protocol;") {
		t.Fatalf("生成了与 TLS 端口冲突的 [::1]:443 proxy_protocol：\n%s", cfg)
	}
}
