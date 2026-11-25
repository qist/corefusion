package proxy

import (
	"strings"
	"testing"

	"corefusion/internal/model/entity"
)

// Caddy 的 PROXY protocol 版本按站点生效：
//   - 显式 v1 / v2 / off 优先；
//   - 旧数据没有版本字段时维持 ProxyProtocol=true → v2、false → off。
func TestCaddyProxyProtocolVersion(t *testing.T) {
	sites := []entity.CaddySite{
		{ServerID: 2, Domain: "v1.example.com", Template: "sni", Path: "443", SNITarget: "127.0.0.1:9001", ProxyProtocolVersion: "v1", Enabled: true},
		{ServerID: 2, Domain: "v2.example.com", Template: "sni", Path: "443", SNITarget: "127.0.0.1:9002", ProxyProtocolVersion: "v2", Enabled: true},
		{ServerID: 2, Domain: "off.example.com", Template: "sni", Path: "443", SNITarget: "127.0.0.1:9003", ProxyProtocol: true, ProxyProtocolVersion: "off", Enabled: true},
		{ServerID: 2, Domain: "legacy.example.com", Template: "sni", Path: "443", SNITarget: "127.0.0.1:9004", ProxyProtocol: true, Enabled: true},
	}
	cfg := generateCaddyFor(t, newCaddyTestManager(t, sites, nil))

	if n := strings.Count(cfg, "proxy_protocol v1"); n != 1 {
		t.Fatalf("显式 v1 应只出现一次，实际 %d 次：\n%s", n, cfg)
	}
	if n := strings.Count(cfg, "proxy_protocol v2"); n != 2 {
		t.Fatalf("显式 v2 + 旧布尔值应各出现一次，实际 %d 次：\n%s", n, cfg)
	}
	// 显式 off 优先于旧布尔值：不能生成带 PP 的块。
	if !strings.Contains(cfg, "proxy 127.0.0.1:9003\n") {
		t.Fatalf("显式 off 的站点应生成单行 proxy（不发 PP）：\n%s", cfg)
	}
	if strings.Contains(cfg, "proxy 127.0.0.1:9003 {") {
		t.Fatalf("显式 off 的站点不应展开 proxy 块：\n%s", cfg)
	}
}

// L4 转发同样支持按站点选版本。
func TestCaddyL4ProxyProtocolVersion(t *testing.T) {
	sites := []entity.CaddySite{
		{ServerID: 2, Domain: "", Template: "l4", Path: "8443", Upstream: "127.0.0.1:9005", ProxyProtocolVersion: "v1", Enabled: true},
	}
	cfg := generateCaddyFor(t, newCaddyTestManager(t, sites, nil))

	if !strings.Contains(cfg, "proxy_protocol v1") {
		t.Fatalf("L4 站点未写入 proxy_protocol v1：\n%s", cfg)
	}
	if strings.Contains(cfg, "proxy_protocol v2") {
		t.Fatalf("L4 站点不应再写死 v2：\n%s", cfg)
	}
}

// 全部显式关闭时不应再挂 listener_wrappers（避免给 HTTP 服务端多余包装）。
func TestCaddyProxyProtocolAllOffSkipsWrappers(t *testing.T) {
	sites := []entity.CaddySite{
		{ServerID: 2, Domain: "off.example.com", Template: "sni", Path: "443", SNITarget: "127.0.0.1:9003", ProxyProtocol: true, ProxyProtocolVersion: "off", Enabled: true},
	}
	cfg := generateCaddyFor(t, newCaddyTestManager(t, sites, nil))

	if strings.Contains(cfg, "proxy_protocol") {
		t.Fatalf("全部关闭时不应生成任何 proxy_protocol：\n%s", cfg)
	}
	if strings.Contains(cfg, "listener_wrappers") {
		t.Fatalf("全部关闭时不应生成 listener_wrappers：\n%s", cfg)
	}
}

// fallback 规则也按自己的版本发送。
func TestCaddyFallbackProxyProtocolVersion(t *testing.T) {
	sites := []entity.CaddySite{
		{ServerID: 2, Domain: "fallback", Template: "sni", Path: "443", SNITarget: "127.0.0.1:8443", LocalPort: "8443", ProxyProtocolVersion: "v1", Enabled: true},
		{ServerID: 2, Domain: "a.example.com", Template: "sni", Path: "443", SNITarget: "127.0.0.1:9001", ProxyProtocolVersion: "off", Enabled: true},
	}
	cfg := generateCaddyFor(t, newCaddyTestManager(t, sites, nil))

	idx := strings.Index(cfg, "# fallback -> 127.0.0.1:8443")
	if idx < 0 {
		t.Fatalf("缺少 fallback 规则：\n%s", cfg)
	}
	tail := cfg[idx:]
	if !strings.Contains(tail, "proxy_protocol v1") {
		t.Fatalf("fallback 规则应使用 v1：\n%s", cfg)
	}
}
