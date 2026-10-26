package proxy

import (
	"strings"
	"testing"

	"corefusion/internal/model/entity"
)

// TestNginxConfigProxyProtocolRealIP 锁定 unix socket + PROXY protocol 的取址链路：
//   - 全局生成 $pp_addr（PROXY 头 → $remote_addr 兜底）；
//   - $clientRealIp 在 XFF 为空时回退 $pp_addr —— 早期被 400/405 拒绝的请求
//     realip 还没执行，只有 $proxy_protocol_addr 有值（否则日志记成 "unix:"）；
//   - unix listen 声明 proxy_protocol，且 set_real_ip_from unix:（AF_UNIX 的 peer
//     不是 IP，写 127.0.0.1 匹配不上，realip 不会改写）。
func TestNginxConfigProxyProtocolRealIP(t *testing.T) {
	unixSite := func(pp bool) []entity.NginxSite {
		return []entity.NginxSite{{
			ServerID: 2, Domain: "unix.example.com", ListenSocket: "/dev/shm/nginx.sock",
			ProxyProtocol: pp, Enabled: true, Path: "/", Upstream: "127.0.0.1:8080",
		}}
	}

	cfg := generateFor(t, newNginxTestManager(t, unixSite(true), `{}`))
	for _, want := range []string{
		"map $proxy_protocol_addr $pp_addr {",
		"default $proxy_protocol_addr;",
		`        "" $pp_addr;`,
		"default $pp_addr;",
		"listen unix:/dev/shm/nginx.sock proxy_protocol;",
		"set_real_ip_from unix:;",
		"real_ip_header proxy_protocol;",
	} {
		if !strings.Contains(cfg, want) {
			t.Fatalf("缺少 %q：\n%s", want, cfg)
		}
	}

	// 未开 PROXY protocol 的 unix 站点：不能声明 proxy_protocol，
	// 也不该信任 unix:（没有 PROXY 头可读，realip 无事可做）。
	cfg = generateFor(t, newNginxTestManager(t, unixSite(false), `{}`))
	if strings.Contains(cfg, "listen unix:/dev/shm/nginx.sock proxy_protocol;") {
		t.Fatalf("未开 PROXY protocol 时 unix listen 不应带 proxy_protocol：\n%s", cfg)
	}
	if strings.Contains(cfg, "set_real_ip_from unix:;") {
		t.Fatalf("未开 PROXY protocol 时不应生成 set_real_ip_from unix:：\n%s", cfg)
	}
	// map 是全局兜底，无论是否启用 PROXY protocol 都应存在。
	if !strings.Contains(cfg, "map $proxy_protocol_addr $pp_addr {") {
		t.Fatalf("应始终生成 $pp_addr map：\n%s", cfg)
	}
}
