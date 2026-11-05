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
	out := cfg["outbounds"].([]interface{})
	ss2 := out[0].(map[string]interface{})["streamSettings"].(map[string]interface{})
	if ss2["method"] != "ws" {
		t.Errorf("outbound alias wrong: %#v", ss2)
	}
}
