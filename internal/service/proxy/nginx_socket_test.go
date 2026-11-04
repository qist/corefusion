package proxy

import (
	"strings"
	"testing"

	"corefusion/internal/model/entity"
)

// 只挂 unix socket 的站点（典型：REALITY 回落 server /dev/shm/nginx.sock）：
// listen_port 为空表示"只监听这个 socket"，生成器不能再按默认端口补 listen 443/80 ——
// 否则会凭空多出一个 HTTPS 监听（还可能带 default_server），抢走真实站点的流量。
func TestNginxConfigSocketOnlySiteDoesNotListenOn443(t *testing.T) {
	sites := []entity.NginxSite{{
		ServerID: 2, Domain: "unix-nginx-0.invalid", Template: "static", Path: "/",
		ListenSocket:  "/dev/shm/nginx.sock",
		ListenPort:    "",
		ProxyProtocol: true,
		Upstream:      "/usr/share/nginx/html",
		CertPath:      "certs/example.com/cert.pem",
		KeyPath:       "certs/example.com/key.pem",
		Enabled:       true,
	}}
	cfg := generateFor(t, newNginxTestManager(t, sites, `{}`))
	if !strings.Contains(cfg, "listen unix:/dev/shm/nginx.sock proxy_protocol;") {
		t.Fatalf("缺少 unix listen：\n%s", cfg)
	}
	if strings.Contains(cfg, "listen 443") || strings.Contains(cfg, "listen 80") {
		t.Fatalf("socket-only 站点不应再监听 443/80：\n%s", cfg)
	}
	// socket-only 只跳过 listen，不能连证书一起跳过 —— REALITY target 必须能完成 TLS 握手。
	if !strings.Contains(cfg, "ssl_certificate ../certs/example.com/cert.pem;") {
		t.Fatalf("socket-only 站点丢失证书：\n%s", cfg)
	}
	if !strings.Contains(cfg, "try_files $uri $uri/ =404;") {
		t.Fatalf("static 模板内容丢失：\n%s", cfg)
	}
}

// websocket 回源的 URI 语义必须交给 nginx 原生规则（upstream 末尾有没有 /），
// 不能按 backend_path 强行补 /：迁移来的 xray ws 入站需要完整路径 /efnxuvlqws，
// 补 / 会把它剥成 / → 上游 404（实测 TROJANWS / VMESSWS 全挂）。
func TestNginxConfigWebsocketKeepsUpstreamURI(t *testing.T) {
	sites := []entity.NginxSite{{
		ServerID: 2, Domain: "w.example.com", Template: "websocket", Path: "/efnxuvlqws",
		Upstream: "http://127.0.0.1:7778", ListenPort: "443", Enabled: true,
	}}
	cfg := generateFor(t, newNginxTestManager(t, sites, `{}`))
	if !strings.Contains(cfg, "proxy_pass http://127.0.0.1:7778;") {
		t.Fatalf("websocket 回源应原样透传完整 URI：\n%s", cfg)
	}
	if strings.Contains(cfg, "proxy_pass http://127.0.0.1:7778/;") {
		t.Fatalf("不应给 websocket 回源补尾斜杠（会剥掉 location 前缀）：\n%s", cfg)
	}
}
