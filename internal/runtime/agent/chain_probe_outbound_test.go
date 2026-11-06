package agent

import "testing"

// 探活拨号复用链式代理的出站转换：mihomo proxy map 必须能还原出
// 地址/端口/凭据/传输层（含 reality），否则探测结果与线上可用性不一致。
func TestBuildXrayOutboundFromMihomoProxy(t *testing.T) {
	pm := map[string]interface{}{
		"type":       "vless",
		"server":     "node.example.com",
		"port":       float64(443),
		"uuid":       "11111111-2222-3333-4444-555555555555",
		"flow":       "xtls-rprx-vision",
		"tls":        true,
		"servername": "cdn.example.com",
		"network":    "tcp",
		"reality-opts": map[string]interface{}{
			"public-key": "PUBKEY",
			"short-id":   "abcd",
		},
		"client-fingerprint": "chrome",
	}

	ob, err := BuildXrayOutboundFromMihomoProxy(pm)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if ob["protocol"] != "vless" {
		t.Fatalf("protocol = %v", ob["protocol"])
	}
	settings, _ := ob["settings"].(map[string]interface{})
	vnext, _ := settings["vnext"].([]map[string]interface{})
	if len(vnext) != 1 || vnext[0]["address"] != "node.example.com" || vnext[0]["port"] != 443 {
		t.Fatalf("vnext = %#v", vnext)
	}
	users, _ := vnext[0]["users"].([]map[string]interface{})
	if len(users) != 1 || users[0]["id"] != pm["uuid"] || users[0]["flow"] != "xtls-rprx-vision" {
		t.Fatalf("users = %#v", users)
	}

	ss, _ := ob["streamSettings"].(map[string]interface{})
	re, _ := ss["realitySettings"].(map[string]interface{})
	if ss["security"] != "reality" || re["publicKey"] != "PUBKEY" || re["shortId"] != "abcd" {
		t.Fatalf("streamSettings = %#v", ss)
	}
}

// 缺地址/协议时明确报错，避免生成空出站后静默探测失败。
func TestBuildXrayOutboundFromMihomoProxyRejectsIncomplete(t *testing.T) {
	if _, err := BuildXrayOutboundFromMihomoProxy(map[string]interface{}{"server": "a.example.com"}); err == nil {
		t.Fatal("missing type must error")
	}
	if _, err := BuildXrayOutboundFromMihomoProxy(map[string]interface{}{"type": "vless"}); err == nil {
		t.Fatal("missing server must error")
	}
}
