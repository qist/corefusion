package subscriptions

import (
	"testing"

	"corefusion/internal/model/entity"
)

// 历史数据只有扁平字段（sing-box 编辑器旧行为），必须能被归一成订阅端读取的
// _accessDomain 对象，否则订阅里拿不到域名/端口 —— 客户端访问不了。
func TestEnsureAccessDomainFromFlatFields(t *testing.T) {
	cfg := map[string]interface{}{
		"type":                 "vless",
		"tag":                  "sb-flat",
		"listen_port":          float64(12345),
		"_accessDomainEnabled": true,
		"_accessDomainType":    "direct",
		"_directDomain":        "sub.test.com",
		"_directPortMode":      "inbound",
		"_cfDomain":            "",
		"_cfIP":                "",
		"_customDomain":        "www.x.com",
		"_customPortMode":      "custom",
		"_accessCustomPort":    float64(9633),
	}
	ensureAccessDomain(cfg)

	ad, ok := cfg["_accessDomain"].(map[string]interface{})
	if !ok {
		t.Fatalf("未生成 _accessDomain 对象: %#v", cfg["_accessDomain"])
	}
	if ad["enabled"] != true {
		t.Errorf("enabled = %v, want true", ad["enabled"])
	}
	if ad["directDomain"] != "sub.test.com" {
		t.Errorf("directDomain = %v", ad["directDomain"])
	}
	if ad["directPortMode"] != "inbound" {
		t.Errorf("directPortMode = %v", ad["directPortMode"])
	}
	if ad["customPort"] != 9633 {
		t.Errorf("customPort = %v", ad["customPort"])
	}
}

// 已经是对象形态时不应被覆盖。
func TestEnsureAccessDomainKeepsExistingObject(t *testing.T) {
	orig := map[string]interface{}{"enabled": true, "type": "direct", "directDomain": "keep.me"}
	cfg := map[string]interface{}{
		"_accessDomainEnabled": true,
		"_directDomain":        "ignored.me",
		"_accessDomain":        orig,
	}
	ensureAccessDomain(cfg)
	ad := cfg["_accessDomain"].(map[string]interface{})
	if ad["directDomain"] != "keep.me" {
		t.Errorf("已存在的 _accessDomain 被覆盖: %#v", ad)
	}
}

// 完全没配接入域名时不应凭空造一个（避免未配置的节点被当成可分享）。
func TestEnsureAccessDomainNoopWhenAbsent(t *testing.T) {
	cfg := map[string]interface{}{"type": "vless", "tag": "no-ad"}
	ensureAccessDomain(cfg)
	if _, ok := cfg["_accessDomain"]; ok {
		t.Error("未配置接入域名时不应生成 _accessDomain")
	}
}

// 外部节点启用访问域名后，订阅里的 server/port 必须换成所选域名/IP，
// 但 SNI/Host 保持导入时的值（CDN 回源依赖这一点）。
func TestApplyExternalNodeAccessDomainOverridesServerPort(t *testing.T) {
	node := &entity.ExternalNode{Port: 20001}
	node.SetAccessDomain(map[string]interface{}{
		"enabled":        true,
		"type":           "direct",
		"directDomain":   "cdn.example.com",
		"directPortMode": "inbound",
	})
	pm := map[string]interface{}{
		"server": "1.2.3.4",
		"port":   float64(20001),
		"sni":    "origin.example.com",
	}
	applyExternalNodeAccessDomain(pm, node, nil)

	if pm["server"] != "cdn.example.com" {
		t.Errorf("server = %v, want cdn.example.com", pm["server"])
	}
	if pm["port"] != float64(20001) {
		t.Errorf("port = %v, want 20001（使用节点端口，且必须是 float64）", pm["port"])
	}
	if pm["sni"] != "origin.example.com" {
		t.Errorf("sni 不应被改写: %v", pm["sni"])
	}
}

// 未启用 / 启用但没选具体域名时，必须保持导入时的地址（默认导入那个）。
func TestApplyExternalNodeAccessDomainKeepsImportedAddress(t *testing.T) {
	cases := []*entity.ExternalNode{
		{Port: 20001}, // 未配置
		func() *entity.ExternalNode {
			n := &entity.ExternalNode{Port: 20001}
			n.SetAccessDomain(map[string]interface{}{"enabled": false, "type": "custom", "customDomain": "x.com"})
			return n
		}(),
		func() *entity.ExternalNode {
			n := &entity.ExternalNode{Port: 20001}
			n.SetAccessDomain(map[string]interface{}{"enabled": true, "type": "direct", "directDomain": ""})
			return n
		}(),
	}
	for i, node := range cases {
		pm := map[string]interface{}{"server": "1.2.3.4", "port": float64(20001)}
		applyExternalNodeAccessDomain(pm, node, nil)
		if pm["server"] != "1.2.3.4" || pm["port"] != float64(20001) {
			t.Errorf("case %d: 未启用/未选择时应保持导入地址, got %v:%v", i, pm["server"], pm["port"])
		}
	}
}

// 外部节点选 CF 优选 IP / CF 优选域名但没指定具体值时，从对应池随机取一个 + 该条目端口。
func TestApplyExternalNodeAccessDomainRandomFromCFPool(t *testing.T) {
	pool := []shareDomainEntry{
		{Type: "cf_ip", Host: "104.16.1.1", Port: 443, Ports: []int{443, 2082}},
		{Type: "cf_domain", Host: "cdn.example.com", Port: 2096, Ports: []int{2096}},
	}
	for _, tc := range []struct {
		adType   string
		wantHost string
		wantPort map[int]bool
	}{
		{"cf_ip", "104.16.1.1", map[int]bool{443: true, 2082: true}},
		{"cf_domain", "cdn.example.com", map[int]bool{2096: true}},
	} {
		node := &entity.ExternalNode{Port: 20001}
		node.SetAccessDomain(map[string]interface{}{"enabled": true, "type": tc.adType})
		pm := map[string]interface{}{"server": "1.2.3.4", "port": float64(20001), "sni": "origin.example.com"}
		applyExternalNodeAccessDomain(pm, node, pool)
		if pm["server"] != tc.wantHost {
			t.Errorf("%s: server = %v, want %s", tc.adType, pm["server"], tc.wantHost)
		}
		port, _ := pm["port"].(float64)
		if !tc.wantPort[int(port)] {
			t.Errorf("%s: port = %v, want one of %v", tc.adType, pm["port"], tc.wantPort)
		}
		if pm["sni"] != "origin.example.com" {
			t.Errorf("%s: sni 不应被改写: %v", tc.adType, pm["sni"])
		}
	}
}

// 端口为 0（hy2 端口跳跃）时只换地址，不把 port 改成 0。
func TestApplyExternalNodeAccessDomainPortHopKeepsZeroPort(t *testing.T) {
	node := &entity.ExternalNode{Port: 0}
	node.SetAccessDomain(map[string]interface{}{
		"enabled": true, "type": "custom", "customDomain": "edge.example.com", "customPortMode": "inbound",
	})
	pm := map[string]interface{}{"server": "1.2.3.4", "port": float64(0), "ports": "40000-50000"}
	applyExternalNodeAccessDomain(pm, node, nil)
	if pm["server"] != "edge.example.com" {
		t.Errorf("server = %v", pm["server"])
	}
	if pm["port"] != float64(0) {
		t.Errorf("port 不应被改写: %v", pm["port"])
	}
}
