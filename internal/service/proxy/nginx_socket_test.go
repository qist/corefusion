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
	// 站点带证书时必须 `listen unix:... ssl proxy_protocol`：REALITY 回落目标要靠这个
	// 完成 TLS 握手，缺 ssl 就是明文 HTTP（已认证的 REALITY 连接也会失败）。
	if !strings.Contains(cfg, "listen unix:/dev/shm/nginx.sock ssl proxy_protocol;") {
		t.Fatalf("缺少带 ssl 的 unix listen：\n%s", cfg)
	}
	if strings.Contains(cfg, "listen 443") || strings.Contains(cfg, "listen 80") {
		t.Fatalf("socket-only 站点不应再监听 443/80：\n%s", cfg)
	}
	// socket-only 只跳过 listen，不能连证书一起跳过 —— REALITY target 必须能完成 TLS 握手。
	if !strings.Contains(cfg, "ssl_certificate ../certs/example.com/cert.pem;") {
		t.Fatalf("socket-only 站点丢失证书：\n%s", cfg)
	}
	if !strings.Contains(cfg, "index index.html index.htm;") {
		t.Fatalf("static 模板内容丢失：\n%s", cfg)
	}
}

// websocket 回源的 URI 语义必须交给 nginx 原生规则（upstream 末尾有没有 /），
// 不能按 backend_path 强行补 /：迁移来的 xray ws 入站需要完整路径 /efnxuvlqws，
// 补 / 会把它剥成 / → 上游 404（实测 TROJANWS / VMESSWS 全挂）。
// Reality 模板必须支持 unix socket 监听（Xray REALITY 的 dest 直接指向它）：
//
//	listen unix:/dev/shm/nginx.sock ssl proxy_protocol;
//
// 且不能再生成 127.0.0.1:<端口> 的 TCP 监听。
func TestNginxRealityUnixSocketListen(t *testing.T) {
	sites := []entity.NginxSite{{
		ServerID: 2, Domain: "127.0.0.1", Template: "reality", Path: "/",
		ListenSocket: "/dev/shm/nginx.sock", ListenPort: "",
		CertPath: "certs/example.com/cert.pem", KeyPath: "certs/example.com/key.pem",
		Enabled: true,
	}}
	cfg := generateFor(t, newNginxTestManager(t, sites, `{}`))
	if !strings.Contains(cfg, "listen unix:/dev/shm/nginx.sock ssl proxy_protocol;") {
		t.Fatalf("Reality 模板缺少 unix listen：\n%s", cfg)
	}
	if strings.Contains(cfg, "listen 127.0.0.1:9443") {
		t.Fatalf("使用 unix socket 时不应再监听 TCP 端口：\n%s", cfg)
	}
	if !strings.Contains(cfg, "set_real_ip_from unix:;") {
		t.Fatalf("unix socket 必须信任 unix: 源，否则拿不到真实 IP：\n%s", cfg)
	}
	if !strings.Contains(cfg, "ssl_certificate ../certs/example.com/cert.pem;") {
		t.Fatalf("Reality 模板必须挂证书：\n%s", cfg)
	}
	if !strings.Contains(cfg, "http2 on;") {
		t.Fatalf("Reality 模板必须强制 HTTP/2：\n%s", cfg)
	}
}

// Reality 默认仍是 127.0.0.1:<端口>，且支持 "unix:" 前缀写法。
func TestNginxRealityPortAndUnixPrefix(t *testing.T) {
	sites := []entity.NginxSite{{
		ServerID: 2, Domain: "127.0.0.1", Template: "reality", Path: "/",
		ListenPort: "9443", CertPath: "certs/example.com/cert.pem", KeyPath: "certs/example.com/key.pem",
		Enabled: true,
	}, {
		ServerID: 2, Domain: "127.0.0.2", Template: "reality", Path: "/",
		ListenSocket: "unix:/dev/shm/reality2.sock", ListenPort: "9444",
		CertPath: "certs/example.com/cert.pem", KeyPath: "certs/example.com/key.pem", Enabled: true,
	}}
	cfg := generateFor(t, newNginxTestManager(t, sites, `{}`))
	if !strings.Contains(cfg, "listen 127.0.0.1:9443 ssl proxy_protocol;") {
		t.Fatalf("Reality 默认应监听 127.0.0.1:9443：\n%s", cfg)
	}
	if !strings.Contains(cfg, "listen unix:/dev/shm/reality2.sock ssl proxy_protocol;") {
		t.Fatalf("unix: 前缀写法应被归一：\n%s", cfg)
	}
	if strings.Contains(cfg, "listen unix:unix:") {
		t.Fatalf("unix: 前缀不应重复：\n%s", cfg)
	}
}

// unix 模式下可额外开一个本地 TCP 端口给 sing-box：
// sing-box 的 REALITY handshake 只支持 host:port 且不发 PROXY protocol，
// 因此这条 listen 必须不带 proxy_protocol，而 unix 那条仍然要带。
func TestNginxRealityUnixPlusLocalTCPPort(t *testing.T) {
	sites := []entity.NginxSite{{
		ServerID: 2, Domain: "127.0.0.1", Template: "reality", Path: "/",
		ListenSocket: "/dev/shm/nginx.sock", SingboxListenPort: "9443",
		CertPath: "certs/example.com/cert.pem", KeyPath: "certs/example.com/key.pem",
		ProxyProtocol: true, Enabled: true,
	}}
	cfg := generateFor(t, newNginxTestManager(t, sites, `{}`))
	if !strings.Contains(cfg, "listen unix:/dev/shm/nginx.sock ssl proxy_protocol;") {
		t.Fatalf("缺少 unix listen：\n%s", cfg)
	}
	if !strings.Contains(cfg, "        listen 127.0.0.1:9443 ssl;\n") {
		t.Fatalf("缺少给 sing-box 的本地 TCP listen（不带 proxy_protocol）：\n%s", cfg)
	}
	if strings.Contains(cfg, "listen 127.0.0.1:9443 ssl proxy_protocol;") {
		t.Fatalf("TCP listen 不能带 proxy_protocol（sing-box 不发送）：\n%s", cfg)
	}
}

// TCP 端口模式同样可以额外给 sing-box 一个不带 PP 的端口：
// Xray 用 listen_port（带 PP），sing-box 用 singbox_listen_port。
func TestNginxRealityTCPPlusSingboxPort(t *testing.T) {
	sites := []entity.NginxSite{{
		ServerID: 2, Domain: "127.0.0.1", Template: "reality", Path: "/",
		ListenPort: "9443", SingboxListenPort: "9444",
		CertPath: "certs/example.com/cert.pem", KeyPath: "certs/example.com/key.pem",
		ProxyProtocol: true, Enabled: true,
	}}
	cfg := generateFor(t, newNginxTestManager(t, sites, `{}`))
	if !strings.Contains(cfg, "        listen 127.0.0.1:9443 ssl proxy_protocol;\n") {
		t.Fatalf("Xray 入口应带 proxy_protocol：\n%s", cfg)
	}
	if !strings.Contains(cfg, "        listen 127.0.0.1:9444 ssl;\n") {
		t.Fatalf("sing-box 入口不应带 proxy_protocol：\n%s", cfg)
	}
	if strings.Contains(cfg, "listen 127.0.0.1:9444 ssl proxy_protocol;") {
		t.Fatalf("sing-box 端口不能带 proxy_protocol：\n%s", cfg)
	}
}

// Reality 端口写法带地址时只取端口，避免生成非法 listen。
func TestNginxRealityListenPortNormalized(t *testing.T) {
	sites := []entity.NginxSite{{
		ServerID: 2, Domain: "127.0.0.1", Template: "reality", Path: "/",
		ListenPort: "127.0.0.1:9444",
		CertPath:   "certs/example.com/cert.pem", KeyPath: "certs/example.com/key.pem", Enabled: true,
	}}
	cfg := generateFor(t, newNginxTestManager(t, sites, `{}`))
	if !strings.Contains(cfg, "        listen 127.0.0.1:9444 ssl proxy_protocol;\n") {
		t.Fatalf("listen_port 带地址时应只取端口：\n%s", cfg)
	}
}

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
