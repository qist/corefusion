package proxy

import (
	"strings"
	"testing"

	"corefusion/internal/model/entity"
)

// 公网 443 被 stream 占用时：HTTP 监听 127.0.0.1:443，QUIC 仍监听公网 443。
func TestNginxConfigQuicListenSeparateFromHTTP(t *testing.T) {
	sites := []entity.NginxSite{
		{
			ServerID: 2, Domain: "a.example.com", ListenPort: "127.0.0.1:443",
			QuicListen: "443", CertPath: "certs/a/cert.pem", KeyPath: "certs/a/key.pem",
			EnableQuic: true, EnableHttp2: true, ProxyProtocol: true, Enabled: true,
			Template: "http", Path: "/", Upstream: "127.0.0.1:8080",
		},
	}
	cfg := generateFor(t, newNginxTestManager(t, sites, `{}`))
	for _, want := range []string{
		"listen 443 quic;",
		"listen 127.0.0.1:443 ssl proxy_protocol;",
	} {
		if !strings.Contains(cfg, want) {
			t.Fatalf("缺少 %q：\n%s", want, cfg)
		}
	}
	if strings.Contains(cfg, "listen 127.0.0.1:443 quic") {
		t.Fatalf("QUIC 不应绑到回环地址：\n%s", cfg)
	}
}

// 非 TLS 流量必须与未知 SNI 分开：生成 guard map 并让 stream server 用它。
func TestNginxConfigStreamNonTLSTarget(t *testing.T) {
	sites := []entity.NginxSite{
		{ServerID: 2, Domain: "a.example.com", Template: "sni", Path: "443", SNITarget: "127.0.0.1:9001", Enabled: true},
		{ServerID: 2, Domain: "fallback", Template: "sni", Path: "443", SNITarget: "127.0.0.1:59026", Enabled: true},
	}
	cfg := generateFor(t, newNginxTestManager(t, sites,
		`{"stream_sni_enabled": true, "stream_nontls_target": "127.0.0.1:8444"}`))
	for _, want := range []string{
		"upstream direct_http {",
		"server 127.0.0.1:8444;",
		"map $ssl_preread_protocol $sni_backend_guarded {",
		`""      direct_http;`,
		"proxy_pass $sni_backend_guarded;",
	} {
		if !strings.Contains(cfg, want) {
			t.Fatalf("缺少 %q：\n%s", want, cfg)
		}
	}
}

// 未配置非 TLS 目标时保持旧行为（直接 proxy_pass $sni_backend）。
func TestNginxConfigStreamNonTLSTargetDisabled(t *testing.T) {
	sites := []entity.NginxSite{
		{ServerID: 2, Domain: "a.example.com", Template: "sni", Path: "443", SNITarget: "127.0.0.1:9001", Enabled: true},
	}
	cfg := generateFor(t, newNginxTestManager(t, sites, `{"stream_sni_enabled": true}`))
	if strings.Contains(cfg, "sni_backend_guarded") {
		t.Fatalf("未启用时不应生成 guard map：\n%s", cfg)
	}
	if !strings.Contains(cfg, "proxy_pass $sni_backend;") {
		t.Fatalf("应保持 proxy_pass $sni_backend：\n%s", cfg)
	}
}
