package proxy

import (
	"encoding/json"
	"testing"

	"corefusion/internal/model/entity"
)

func realityInbound(t *testing.T, listen string, port int, serverNames []string, proxyProtocol bool) entity.InboundConfig {
	t.Helper()
	sockopt := map[string]interface{}{}
	if proxyProtocol {
		sockopt["acceptProxyProtocol"] = true
	}
	cfg := map[string]interface{}{
		"protocol": "vless",
		"streamSettings": map[string]interface{}{
			"network":  "grpc",
			"security": "reality",
			"sockopt":  sockopt,
			"realitySettings": map[string]interface{}{
				"serverNames": serverNames,
				"shortIds":    []interface{}{"", "abcd"},
			},
		},
	}
	raw, _ := json.Marshal(cfg)
	return entity.InboundConfig{Listen: listen, Port: port, Enabled: true, ConfigJSON: string(raw)}
}

// REALITY 入站的 serverNames 必须全部派生为 SNI 规则：客户端随机挑一个，
// 少一条就会落到 fallback（表现为节点时通时不通）。
func TestSNIRulesFromInboundDerivesAllServerNames(t *testing.T) {
	cfg := realityInbound(t, "/dev/shm/VLESS_GRPC_WAI.sock,0666", 38877,
		[]string{"www.a.com", "", "b.com", "www.a.com"}, true)

	rules := SNIRulesFromInbound(cfg)
	if len(rules) != 2 {
		t.Fatalf("rules = %d, want 2（空串和重复要去掉）: %#v", len(rules), rules)
	}
	if rules[0].Domain != "www.a.com" || rules[1].Domain != "b.com" {
		t.Fatalf("domains = %v", []string{rules[0].Domain, rules[1].Domain})
	}
	for _, r := range rules {
		if r.Target != "unix:/dev/shm/VLESS_GRPC_WAI.sock" {
			t.Errorf("%s target = %q", r.Domain, r.Target)
		}
		if !r.ProxyProtocol {
			t.Errorf("%s 应带 PROXY 协议头", r.Domain)
		}
	}
}

// 公网监听 + acceptProxyProtocol：后端是 127.0.0.1:port（nginx 在本机回源）。
func TestSNIRulesFromInboundProxyProtocolOnTCP(t *testing.T) {
	cfg := realityInbound(t, "0.0.0.0", 51878, []string{"aiml.apple.com"}, true)
	rules := SNIRulesFromInbound(cfg)
	if len(rules) != 1 {
		t.Fatalf("rules = %#v", rules)
	}
	if rules[0].Target != "127.0.0.1:51878" {
		t.Errorf("target = %q, want 127.0.0.1:51878", rules[0].Target)
	}
	if !rules[0].ProxyProtocol {
		t.Error("应带 PROXY 协议头")
	}
}

// 直连入站（不要求 PROXY 头）不需要 nginx 分流，不能抢走。
func TestSNIRulesFromInboundSkipsDirectInbound(t *testing.T) {
	cfg := realityInbound(t, "0.0.0.0", 50017, []string{"www.a.com"}, false)
	if rules := SNIRulesFromInbound(cfg); len(rules) != 0 {
		t.Fatalf("直连入站不应派生 SNI 规则: %#v", rules)
	}
}

// 非 REALITY（普通 TLS）由用户站点决定，不派生。
func TestSNIRulesFromInboundSkipsNonReality(t *testing.T) {
	cfg := realityInbound(t, "/dev/shm/x.sock", 11995, []string{"www.a.com"}, true)
	cfg.ConfigJSON = `{"streamSettings":{"network":"raw","security":"tls","tlsSettings":{"serverName":"www.a.com"}}}`
	if rules := SNIRulesFromInbound(cfg); len(rules) != 0 {
		t.Fatalf("非 REALITY 不应派生: %#v", rules)
	}
}

// 用户在「Nginx 代理」里显式配过的同域名规则优先，不能被派生覆盖。
func TestMergeDerivedSNISitesExplicitWins(t *testing.T) {
	existing := []entity.NginxSite{
		{ID: 9, Domain: "www.a.com", Template: "sni", SNITarget: "127.0.0.1:9999", LocalPort: "8443"},
	}
	rules := []derivedSNIRule{
		{Domain: "www.a.com", Target: "unix:/dev/shm/x.sock"},
		{Domain: "new.com", Target: "unix:/dev/shm/x.sock", ProxyProtocol: true},
	}
	got := mergeDerivedNginxSNISites(1, existing, rules)
	if len(got) != 2 {
		t.Fatalf("sites = %d, want 2", len(got))
	}
	if got[0].SNITarget != "127.0.0.1:9999" || got[0].LocalPort != "8443" {
		t.Errorf("显式站点被覆盖: %#v", got[0])
	}
	if got[1].Domain != "new.com" || got[1].Template != "sni" || got[1].SNITarget != "unix:/dev/shm/x.sock" {
		t.Errorf("派生站点不正确: %#v", got[1])
	}
	if !got[1].ProxyProtocol || got[1].L4Protocol != "tcp" {
		t.Errorf("派生站点缺 PROXY/L4: %#v", got[1])
	}
}

// nginx 的 unix socket 写法是 unix:/path，Caddy 必须是 unix//path（unix/ 前缀）。
// 用错会让 Caddy 把 "unix" 当主机名，配置能加载但流量到不了后端。
func TestCaddyProxyTargetUnixSyntax(t *testing.T) {
	cases := map[string]string{
		"unix:/dev/shm/x.sock": "unix//dev/shm/x.sock",
		"unix:@abstract":       "unix/@abstract",
		"127.0.0.1:8443":       "127.0.0.1:8443",
		"example.com:443":      "example.com:443",
	}
	for in, want := range cases {
		if got := caddyProxyTarget(in); got != want {
			t.Errorf("caddyProxyTarget(%q) = %q, want %q", in, got, want)
		}
	}
}
