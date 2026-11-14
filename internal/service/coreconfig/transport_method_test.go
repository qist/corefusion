package coreconfig

import "testing"

func TestApplyTransportMethodAlias(t *testing.T) {
	cfg := map[string]interface{}{
		"inbounds": []interface{}{
			map[string]interface{}{
				"protocol": "vless",
				"streamSettings": map[string]interface{}{
					"network": "xhttp",
				},
			},
			map[string]interface{}{
				"protocol": "masque",
				"streamSettings": map[string]interface{}{
					"network": "masque",
					"method":  "masque", // 已有则不覆盖
				},
			},
			map[string]interface{}{
				"protocol": "vless",
				"streamSettings": map[string]interface{}{
					// 新格式只写 method：要反补 network 给旧核心
					"method": "websocket",
				},
			},
			map[string]interface{}{"protocol": "tun"}, // 无 streamSettings
		},
		"outbounds": []interface{}{
			map[string]interface{}{
				"protocol": "vless",
				"streamSettings": map[string]interface{}{
					"network": "ws",
				},
			},
		},
	}
	ApplyTransportMethodAlias(cfg)

	in := cfg["inbounds"].([]interface{})
	ss0 := in[0].(map[string]interface{})["streamSettings"].(map[string]interface{})
	if ss0["method"] != "xhttp" || ss0["network"] != "xhttp" {
		t.Errorf("inbound alias wrong: %#v", ss0)
	}
	ss1 := in[1].(map[string]interface{})["streamSettings"].(map[string]interface{})
	if ss1["method"] != "masque" {
		t.Errorf("existing method overwritten: %#v", ss1)
	}
	// 只写 method 的新格式：method 保持新枚举名，network 反补成内部短名。
	ss2 := in[2].(map[string]interface{})["streamSettings"].(map[string]interface{})
	if ss2["method"] != "websocket" || ss2["network"] != "ws" {
		t.Errorf("method-only alias wrong: %#v", ss2)
	}
	out := cfg["outbounds"].([]interface{})
	ss3 := out[0].(map[string]interface{})["streamSettings"].(map[string]interface{})
	if ss3["method"] != "websocket" || ss3["network"] != "ws" {
		t.Errorf("outbound alias wrong: %#v", ss3)
	}
}
