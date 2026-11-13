package proxy

import (
	"testing"

	"corefusion/internal/model/entity"
)

// sing-box REALITY 的 SNI 也要能派生 443 分流规则（TLS 在核心内完成，
// nginx/caddy 只能按 ClientHello 的 SNI 转 TCP）。它的服务名是单个
// tls.server_name，且核心不支持 PROXY 协议 → 派生规则必须 ProxyProtocol=false。
func TestSNIRulesFromInboundSingboxReality(t *testing.T) {
	cfg := entity.InboundConfig{
		Enabled:  true,
		CoreType: "singbox",
		Protocol: "vless",
		Listen:   "0.0.0.0",
		Port:     31161,
		ConfigJSON: `{
			"type": "vless",
			"listen": "0.0.0.0",
			"listen_port": 31161,
			"tls": {
				"enabled": true,
				"server_name": "www.apple.com",
				"reality": {
					"enabled": true,
					"handshake": {"server": "www.apple.com", "server_port": 443},
					"private_key": "k",
					"short_id": ["ab"]
				}
			},
			"users": [{"name": "a@b.c", "uuid": "11111111-1111-4111-8111-111111111111"}]
		}`,
	}
	rules := SNIRulesFromInbound(cfg)
	if len(rules) != 1 {
		t.Fatalf("应派生 1 条规则，实际 %d: %+v", len(rules), rules)
	}
	r := rules[0]
	if r.Domain != "www.apple.com" {
		t.Errorf("domain = %q", r.Domain)
	}
	if r.Target != "127.0.0.1:31161" {
		t.Errorf("target = %q, want 127.0.0.1:31161", r.Target)
	}
	if r.ProxyProtocol {
		t.Error("sing-box 不支持 PROXY 协议，派生规则必须是 proxy_protocol=false")
	}
	if !r.FromSingbox {
		t.Error("规则应标记 FromSingbox（nginx 侧据此在 proxy_protocol on 时跳过）")
	}
}

// unix / 抽象 socket 监听：目标必须是 unix: 写法（nginx 才能连）。
func TestSNIRulesFromInboundSingboxRealitySocket(t *testing.T) {
	for name, listen := range map[string]string{
		"unix":     "/dev/shm/SB.sock,0666",
		"abstract": "@sb-reality",
	} {
		cfg := entity.InboundConfig{
			Enabled: true, CoreType: "singbox", Protocol: "trojan", Listen: listen, Port: 10000,
			ConfigJSON: `{"type":"trojan","tls":{"enabled":true,"server_name":"t.example.com",
				"reality":{"enabled":true}}}`,
		}
		rules := SNIRulesFromInbound(cfg)
		if len(rules) != 1 {
			t.Fatalf("%s: 应派生 1 条规则，实际 %+v", name, rules)
		}
		want := "unix:/dev/shm/SB.sock"
		if listen[0] == '@' {
			want = "unix:@sb-reality"
		}
		if rules[0].Target != want {
			t.Errorf("%s: target = %q, want %q", name, rules[0].Target, want)
		}
	}
}

// 非 REALITY / 未启 TLS / 无 server_name / 协议不支持 → 不派生。
func TestSNIRulesFromInboundSingboxRealitySkips(t *testing.T) {
	cases := map[string]string{
		"tls disabled": `{"type":"vless","tls":{"enabled":false,"server_name":"a.com",
			"reality":{"enabled":true}}}`,
		"reality disabled": `{"type":"vless","tls":{"enabled":true,"server_name":"a.com",
			"reality":{"enabled":false}}}`,
		"no server name": `{"type":"vless","tls":{"enabled":true,"reality":{"enabled":true}}}`,
		"hysteria2 has no reality": `{"type":"hysteria2","tls":{"enabled":true,"server_name":"a.com",
			"reality":{"enabled":true}}}`,
	}
	for name, raw := range cases {
		cfg := entity.InboundConfig{Enabled: true, CoreType: "singbox", Listen: "0.0.0.0", Port: 10001, ConfigJSON: raw}
		if rules := SNIRulesFromInbound(cfg); len(rules) != 0 {
			t.Errorf("%s: 不应派生，实际 %+v", name, rules)
		}
	}
}

// 显式站点优先：用户手工给同域名配了别的 target 时，派生规则不能覆盖。
func TestMergeDerivedNginxSNISitesKeepsExplicit(t *testing.T) {
	existing := []entity.NginxSite{{
		ServerID: 1, Domain: "www.apple.com", Template: "sni", SNITarget: "unix:/dev/shm/CUSTOM.sock", Enabled: true,
	}}
	rules := []derivedSNIRule{{
		Domain: "www.apple.com", Target: "127.0.0.1:31161", FromSingbox: true,
	}, {
		Domain: "www.amd.com", Target: "127.0.0.1:31162", FromSingbox: true,
	}}
	out := mergeDerivedNginxSNISites(1, existing, rules)
	if len(out) != 2 {
		t.Fatalf("应保留显式站点并追加 1 条派生规则，实际 %d", len(out))
	}
	if out[0].SNITarget != "unix:/dev/shm/CUSTOM.sock" {
		t.Errorf("显式站点被覆盖: %+v", out[0])
	}
	if out[1].Domain != "www.amd.com" || out[1].ProxyProtocol {
		t.Errorf("派生站点不正确: %+v", out[1])
	}
}

// nginx 的 proxy_protocol 是按 listen 组写死的：开了 PROXY 头时，
// sing-box 的派生规则必须整组丢弃（否则把流量导到必然握手失败的后端），
// xray 的规则不受影响。
func TestFilterDerivedSNIRulesByProxyProtocol(t *testing.T) {
	rules := []derivedSNIRule{
		{Domain: "x.example.com", Target: "unix:/dev/shm/X.sock", ProxyProtocol: true},
		{Domain: "sb.example.com", Target: "127.0.0.1:31161", FromSingbox: true},
	}
	out, skipped := filterDerivedSNIRules(rules, true)
	if skipped != 1 || len(out) != 1 || out[0].Domain != "x.example.com" {
		t.Fatalf("proxy_protocol on 时应只丢 sing-box 规则: out=%+v skipped=%d", out, skipped)
	}
	out, skipped = filterDerivedSNIRules(rules, false)
	if skipped != 0 || len(out) != 2 {
		t.Fatalf("proxy_protocol off 时应全部保留: out=%+v skipped=%d", out, skipped)
	}
}
