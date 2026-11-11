package proxy

import (
	"strings"
	"testing"

	"corefusion/internal/model/entity"
)

// streamListens 提取生成配置里所有 `listen ...` 行（去缩进），便于断言。
func streamListens(cfg string) []string {
	var out []string
	for _, line := range strings.Split(cfg, "\n") {
		if strings.Contains(line, "listen ") {
			out = append(out, strings.TrimSpace(line))
		}
	}
	return out
}

func hasListen(listens []string, want string) bool {
	for _, l := range listens {
		if l == want {
			return true
		}
	}
	return false
}

// SNI 站点勾选 IPv6 并填写站点级地址时，必须用**站点地址**生成 v6 监听。
// 回归：旧实现只用全局 stream_ipv6_listen_addr，站点里填的地址完全不生效
// （界面「关联 IPv6 监听」看起来开了、配置里却是全局地址）。
func TestNginxSNIIPv6UsesSiteAddress(t *testing.T) {
	site := entity.NginxSite{
		ServerID: 2, Domain: "a.example.com", Template: "sni",
		Path: "203.0.113.10:443", SNITarget: "127.0.0.1:443",
		Enabled: true, EnableIpv6: true, IPv6ListenAddr: "[2001:db8::5]:443",
	}
	cfg := generateFor(t, newNginxTestManager(t, []entity.NginxSite{site},
		`{"stream_sni_enabled":true,"stream_ipv6":false,"ipv6":false,"stream_ipv6_listen_addr":"[2001:db8::1]:443"}`))
	listens := streamListens(cfg)

	if !hasListen(listens, "listen [2001:db8::5]:443 reuseport fastopen=4096 so_keepalive=on backlog=4096;") {
		t.Fatalf("SNI 站点级 IPv6 地址未生效，listens=%v", listens)
	}
	if strings.Contains(cfg, "[2001:db8::1]:443") {
		t.Fatalf("站点显式指定地址时不应再套用全局地址：%v", listens)
	}
}

// 勾选 IPv6 但地址留空 → 用全局 stream_ipv6_listen_addr。
func TestNginxSNIIPv6FallsBackToGlobal(t *testing.T) {
	site := entity.NginxSite{
		ServerID: 2, Domain: "a.example.com", Template: "sni",
		Path: "203.0.113.10:443", SNITarget: "127.0.0.1:443",
		Enabled: true, EnableIpv6: true,
	}
	cfg := generateFor(t, newNginxTestManager(t, []entity.NginxSite{site},
		`{"stream_sni_enabled":true,"stream_ipv6":false,"ipv6":false,"stream_ipv6_listen_addr":"[2001:db8::1]:443"}`))
	if !hasListen(streamListens(cfg), "listen [2001:db8::1]:443 reuseport fastopen=4096 so_keepalive=on backlog=4096;") {
		t.Fatalf("留空时应回退全局 stream IPv6 地址：%v", streamListens(cfg))
	}
}

// 误把 IPv4 地址填进 IPv6 输入框（生产实际出现过 203.0.113.10:443）时，
// 不能生成非法监听，应回退全局值。
func TestNginxSNIIPv6IgnoresInvalidIPv4(t *testing.T) {
	site := entity.NginxSite{
		ServerID: 2, Domain: "a.example.com", Template: "sni",
		Path: "203.0.113.10:443", SNITarget: "127.0.0.1:443",
		Enabled: true, EnableIpv6: true, IPv6ListenAddr: "203.0.113.10:443",
	}
	cfg := generateFor(t, newNginxTestManager(t, []entity.NginxSite{site},
		`{"stream_sni_enabled":true,"stream_ipv6":false,"ipv6":false,"stream_ipv6_listen_addr":"[2001:db8::1]:443"}`))
	listens := streamListens(cfg)

	if hasListen(listens, "listen [203.0.113.10]:443 reuseport fastopen=4096 so_keepalive=on backlog=4096;") ||
		strings.Contains(cfg, "listen [203.0.113.10]") {
		t.Fatalf("IPv4 被误当成 IPv6 监听：%v", listens)
	}
	if !hasListen(listens, "listen [2001:db8::1]:443 reuseport fastopen=4096 so_keepalive=on backlog=4096;") {
		t.Fatalf("非法站点地址应回退全局值：%v", listens)
	}
}

// L4 站点同样要支持站点级 IPv6 地址。
func TestNginxL4IPv6UsesSiteAddress(t *testing.T) {
	site := entity.NginxSite{
		ServerID: 2, Domain: "l4.example.com", Template: "l4", L4Protocol: "tcp",
		Path: "9443", Upstream: "127.0.0.1:9443",
		Enabled: true, EnableIpv6: true, IPv6ListenAddr: "[2001:db8::6]",
	}
	cfg := generateFor(t, newNginxTestManager(t, []entity.NginxSite{site},
		`{"stream_ipv6":false,"ipv6":false,"stream_ipv6_listen_addr":"[2001:db8::1]:443"}`))
	if !hasListen(streamListens(cfg), "listen [2001:db8::6]:9443 ipv6only=on so_keepalive=on;") {
		t.Fatalf("L4 站点级 IPv6 地址未生效：%v", streamListens(cfg))
	}
}
