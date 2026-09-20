package subscriptions

import "testing"

// 历史数据只有扁平字段（sing-box 编辑器旧行为），必须能被归一成订阅端读取的
// _accessDomain 对象，否则订阅里拿不到域名/端口 —— 客户端访问不了。
func TestEnsureAccessDomainFromFlatFields(t *testing.T) {
	cfg := map[string]interface{}{
		"type":                "vless",
		"tag":                 "sb-flat",
		"listen_port":         float64(12345),
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
