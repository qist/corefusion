package coreconfig

import "testing"

func TestSupportsTransportMethod(t *testing.T) {
	cases := map[string]bool{
		"":         false,
		"unknown":  false,
		"v26.6.27": false,
		"Xray 26.6.22 (Xray, Penetrates Everything.) b99c3e5": false,
		"v26.7.10": false, // 26.7.10 < 26.7.11
		"v26.7.11": true,
		"Xray 26.7.11 (Xray, Penetrates Everything.)": true,
		"v26.9.30": true,
		"v27.1.0":  true,
		"v1.8.24":  false,
	}
	for v, want := range cases {
		if got := SupportsTransportMethod(v); got != want {
			t.Fatalf("SupportsTransportMethod(%q) = %v, want %v", v, got, want)
		}
	}
}

func newTransportTestConfig() map[string]interface{} {
	ss := func(network, method string) map[string]interface{} {
		m := map[string]interface{}{}
		if network != "" {
			m["network"] = network
		}
		if method != "" {
			m["method"] = method
		}
		return m
	}
	return map[string]interface{}{
		"inbounds": []interface{}{
			map[string]interface{}{"protocol": "vless", "streamSettings": ss("xhttp", "")},
			map[string]interface{}{"protocol": "masque", "streamSettings": ss("", "masque")},
			map[string]interface{}{"protocol": "vless", "streamSettings": ss("", "websocket")},
			map[string]interface{}{"protocol": "tun"}, // 无 streamSettings
		},
		"outbounds": []interface{}{
			map[string]interface{}{"protocol": "vless", "streamSettings": ss("ws", "")},
			map[string]interface{}{"protocol": "vless", "streamSettings": ss("mkcp", "mkcp")},
		},
	}
}

func streamOf(t *testing.T, cfg map[string]interface{}, section string, index int) map[string]interface{} {
	t.Helper()
	items, ok := cfg[section].([]interface{})
	if !ok || len(items) <= index {
		t.Fatalf("%s[%d] missing", section, index)
	}
	item, _ := items[index].(map[string]interface{})
	ss, _ := item["streamSettings"].(map[string]interface{})
	if ss == nil {
		t.Fatalf("%s[%d] has no streamSettings", section, index)
	}
	return ss
}

// 新核心：只写 method（新枚举名），不再保留 network。
func TestApplyTransportMethodAliasNewCore(t *testing.T) {
	cfg := newTransportTestConfig()
	ApplyTransportMethodAlias(cfg, "Xray 26.7.11 (Xray, Penetrates Everything.)")

	if ss := streamOf(t, cfg, "inbounds", 0); ss["method"] != "xhttp" || ss["network"] != nil {
		t.Errorf("inbound[0] = %#v", ss)
	}
	if ss := streamOf(t, cfg, "inbounds", 1); ss["method"] != "masque" || ss["network"] != nil {
		t.Errorf("inbound[1] = %#v", ss)
	}
	// method: websocket → method 保持全称，network 被删除
	if ss := streamOf(t, cfg, "inbounds", 2); ss["method"] != "websocket" || ss["network"] != nil {
		t.Errorf("inbound[2] = %#v", ss)
	}
	if ss := streamOf(t, cfg, "outbounds", 0); ss["method"] != "websocket" || ss["network"] != nil {
		t.Errorf("outbound[0] = %#v", ss)
	}
	if ss := streamOf(t, cfg, "outbounds", 1); ss["method"] != "mkcp" || ss["network"] != nil {
		t.Errorf("outbound[1] = %#v", ss)
	}
}

// 旧核心/未知版本：只写 network（短名），method 必须删掉——
// 旧核心会静默忽略 method，若只留 method 会退化成裸 TCP。
func TestApplyTransportMethodAliasOldCore(t *testing.T) {
	cfg := newTransportTestConfig()
	ApplyTransportMethodAlias(cfg, "")

	if ss := streamOf(t, cfg, "inbounds", 0); ss["network"] != "xhttp" || ss["method"] != nil {
		t.Errorf("inbound[0] = %#v", ss)
	}
	if ss := streamOf(t, cfg, "inbounds", 1); ss["network"] != "masque" || ss["method"] != nil {
		t.Errorf("inbound[1] = %#v", ss)
	}
	// 只有 method: websocket 的配置要反补成 network: ws
	if ss := streamOf(t, cfg, "inbounds", 2); ss["network"] != "ws" || ss["method"] != nil {
		t.Errorf("inbound[2] = %#v", ss)
	}
	if ss := streamOf(t, cfg, "outbounds", 0); ss["network"] != "ws" || ss["method"] != nil {
		t.Errorf("outbound[0] = %#v", ss)
	}
	if ss := streamOf(t, cfg, "outbounds", 1); ss["network"] != "mkcp" || ss["method"] != nil {
		t.Errorf("outbound[1] = %#v", ss)
	}
}

func TestApplyTransportMethodAliasKeepsUnknownTransport(t *testing.T) {
	cfg := map[string]interface{}{
		"inbounds": []interface{}{
			map[string]interface{}{
				"protocol":       "vless",
				"streamSettings": map[string]interface{}{"network": "hysteria"},
			},
		},
	}
	ApplyTransportMethodAlias(cfg, "v26.9.30")
	if ss := streamOf(t, cfg, "inbounds", 0); ss["method"] != "hysteria" {
		t.Errorf("hysteria should be preserved: %#v", ss)
	}
}
