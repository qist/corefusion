package proxy

import (
	"strings"
	"testing"

	"corefusion/internal/model/entity"
)

// 生产形态回归：公网 443 归 stream，HTTP 在 127.0.0.1:443，QUIC 在公网 443，
// 客户端 IPv6 走 [::1]:443（server_config 补），并带非 TLS 分流。
func TestNginxConfigProductionShape(t *testing.T) {
	sites := []entity.NginxSite{
		{
			ServerID:   2,
			Domain:     "site1.example.com, site1.example.org",
			ListenPort: "127.0.0.1:443", QuicListen: "443",
			CertPath: "certs/example.com/cert.pem", KeyPath: "certs/example.com/key.pem",
			EnableQuic: true, EnableHttp2: true, ProxyProtocol: true,
			Template: "http", Path: "/dns-query", Upstream: "https://127.0.0.1:8443",
			ServerConfig:   "listen [::1]:443 ssl proxy_protocol;\nlisten [::1]:443 quic;\nadd_header X-Frame-Options SAMEORIGIN always;",
			LocationConfig: "proxy_set_header X-Real-IP $proxy_protocol_addr;",
			Enabled:        true,
		},
		{
			ServerID:   2,
			Domain:     "site1.example.com, site1.example.org",
			ListenPort: "127.0.0.1:443", QuicListen: "443",
			CertPath: "certs/example.com/cert.pem", KeyPath: "certs/example.com/key.pem",
			EnableQuic: true, EnableHttp2: true, ProxyProtocol: true,
			Template: "grpc", Path: "/dzfg9lm24p0x", Upstream: "unix:/dev/shm/TROJANGRPC.sock",
			LocationConfig: "if ($content_type !~ \"application/grpc\") { return 404; }",
			Enabled:        true,
		},
		{
			ServerID: 2, Domain: "a.example.com", Template: "sni", Path: "443",
			SNITarget: "127.0.0.1:59026", Enabled: true,
		},
		{
			ServerID: 2, Domain: "fallback", Template: "sni", Path: "443",
			SNITarget: "127.0.0.1:59026", Enabled: true,
		},
		{
			ServerID: 2, Domain: "non-tls.invalid", Template: "return_code",
			Path: "/", ListenPort: "127.0.0.1:8444", ProxyProtocol: true,
			BackendPath: "403", Enabled: true,
		},
	}
	cfg := generateFor(t, newNginxTestManager(t, sites,
		`{"stream_sni_enabled": true, "stream_nontls_target": "127.0.0.1:8444"}`))

	for _, want := range []string{
		"server_name site1.example.com site1.example.org;",
		"listen 127.0.0.1:443 ssl proxy_protocol;",
		"listen 443 quic;",
		"listen [::1]:443 ssl proxy_protocol;",
		"listen [::1]:443 quic;",
		"proxy_pass https://127.0.0.1:8443;",
		"grpc_pass grpc://backend_unix__dev_shm_TROJANGRPC_sock;",
		`if ($content_type !~ "application/grpc") { return 404; }`,
		"listen 127.0.0.1:8444 proxy_protocol;",
		"return 403;",
		"map $ssl_preread_protocol $sni_backend_guarded {",
		"proxy_pass $sni_backend_guarded;",
	} {
		if !strings.Contains(cfg, want) {
			t.Fatalf("缺少 %q：\n%s", want, cfg)
		}
	}
}
