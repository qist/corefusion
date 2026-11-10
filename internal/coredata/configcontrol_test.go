package coredata

import "testing"

// MASQUE 出站必须能被进程内 Build（面板出站/路由热下发走这条路）。
// 依赖更新前 xray-core 没有 proxy/masque，Build 会报 unknown protocol。
func TestBuildOutboundConfigMasque(t *testing.T) {
	raw := []byte(`{
		"tag": "warp-masque-1",
		"protocol": "masque",
		"settings": {"address": "203.0.113.1", "port": 443, "remoteDNS": ["1.1.1.1"]},
		"streamSettings": {
			"network": "masque",
			"security": "tls",
			"masqueSettings": {"path": "/"},
			"tlsSettings": {"serverName": "cloudflareaccess.com", "alpn": ["h3"]}
		}
	}`)

	outbound, tag, err := BuildOutboundConfig(raw)
	if err != nil {
		t.Fatalf("BuildOutboundConfig(masque) error: %v", err)
	}
	if tag != "warp-masque-1" {
		t.Fatalf("tag = %q, want warp-masque-1", tag)
	}
	if outbound == nil || outbound.GetTag() != "warp-masque-1" {
		t.Fatalf("unexpected outbound config: %+v", outbound)
	}
	if outbound.ProxySettings == nil {
		t.Fatal("masque outbound has no proxy settings")
	}
}

// 普通出站仍然可用（回归）。
func TestBuildOutboundConfigFreedom(t *testing.T) {
	outbound, tag, err := BuildOutboundConfig([]byte(`{"tag":"direct","protocol":"freedom","settings":{"domainStrategy":"UseIP"}}`))
	if err != nil {
		t.Fatalf("BuildOutboundConfig(freedom) error: %v", err)
	}
	if tag != "direct" || outbound.GetTag() != "direct" {
		t.Fatalf("tag = %q / %q, want direct", tag, outbound.GetTag())
	}
}

// 全量替换路由时必须自动补 "inboundTag:[api] → api" 规则，否则后续热下发全废。
func TestBuildRoutingConfigAddsImplicitAPIRule(t *testing.T) {
	built, err := BuildRoutingConfig([]byte(`{"rules":[{"type":"field","ip":["1.1.1.1"],"outboundTag":"direct"}]}`))
	if err != nil {
		t.Fatalf("BuildRoutingConfig error: %v", err)
	}
	if len(built.Rule) < 2 {
		t.Fatalf("rules = %d, want >= 2 (implicit api + user rule)", len(built.Rule))
	}
	if got := built.Rule[0].GetTag(); got != "api" {
		t.Fatalf("first rule outboundTag = %q, want api", got)
	}
}

// 已有 api 规则时不要重复插入。
func TestBuildRoutingConfigKeepsExistingAPIRule(t *testing.T) {
	built, err := BuildRoutingConfig([]byte(`{"rules":[
		{"type":"field","inboundTag":["api"],"outboundTag":"api"},
		{"type":"field","ip":["1.1.1.1"],"outboundTag":"direct"}
	]}`))
	if err != nil {
		t.Fatalf("BuildRoutingConfig error: %v", err)
	}
	apiRules := 0
	for _, r := range built.Rule {
		if r.GetTag() == "api" {
			apiRules++
		}
	}
	if apiRules != 1 {
		t.Fatalf("api rules = %d, want 1", apiRules)
	}
}
