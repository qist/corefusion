package coreconfig

import (
	"encoding/json"
	"testing"
)

// vision 只能配裸 TCP/raw：gRPC 上的 vision 会让客户端硬失败
// （sing-box: `initialize vision: not a valid supported TLS connection`），
// 或服务端客户端 flow 不一致（`flow mismatch`）。渲染运行配置时必须清掉。
func TestCleanInboundFieldsStripsVisionOnGRPC(t *testing.T) {
	inbound := map[string]interface{}{
		"protocol": "vless",
		"streamSettings": map[string]interface{}{
			"network": "grpc",
		},
		"settings": map[string]interface{}{
			"clients": []interface{}{
				map[string]interface{}{"id": "u1", "flow": "xtls-rprx-vision", "email": "a@x"},
				map[string]interface{}{"id": "u2", "email": "b@x"},
			},
		},
	}
	CleanInboundFields(inbound, "inbound-vless-36409", "0.0.0.0", "vless", 36409, "xray")

	clients := inbound["settings"].(map[string]interface{})["clients"].([]interface{})
	if _, has := clients[0].(map[string]interface{})["flow"]; has {
		t.Fatalf("gRPC 入站的 vision flow 应被清除：%+v", clients[0])
	}
	if got := len(clients); got != 2 {
		t.Fatalf("用户数不应变化，实际 %d", got)
	}
}

// raw / tcp（含未写 network 的默认裸 TCP）必须保留 vision。
func TestCleanInboundFieldsKeepsVisionOnRaw(t *testing.T) {
	for _, tc := range []struct {
		name string
		ss   map[string]interface{}
	}{
		{"raw", map[string]interface{}{"network": "raw"}},
		{"tcp", map[string]interface{}{"network": "tcp"}},
		{"default", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inbound := map[string]interface{}{
				"protocol": "vless",
				"settings": map[string]interface{}{
					"clients": []interface{}{
						map[string]interface{}{"id": "u1", "flow": "xtls-rprx-vision"},
					},
				},
			}
			if tc.ss != nil {
				inbound["streamSettings"] = tc.ss
			}
			CleanInboundFields(inbound, "ib", "0.0.0.0", "vless", 443, "xray")
			clients := inbound["settings"].(map[string]interface{})["clients"].([]interface{})
			if flow, _ := clients[0].(map[string]interface{})["flow"].(string); flow != "xtls-rprx-vision" {
				t.Fatalf("%s 传输应保留 vision，实际 flow=%q", tc.name, flow)
			}
		})
	}
}

// 路由规则的启用/停用开关对 xray 与 sing-box 同口径：
// 停用的规则从运行配置中剔除，面板内部字段 enabled 不能下发给核心
// （sing-box 严格解码，未知字段会整份拒载）。
func TestFilterDisabledRoutingRulesBothCores(t *testing.T) {
	full := map[string]interface{}{
		"routing": map[string]interface{}{
			"rules": []interface{}{
				map[string]interface{}{"domain": []interface{}{"keep.com"}, "outboundTag": "direct", "enabled": true},
				map[string]interface{}{"domain": []interface{}{"drop.com"}, "outboundTag": "blocked", "enabled": false},
			},
		},
		"route": map[string]interface{}{
			"rules": []interface{}{
				map[string]interface{}{"domain_suffix": []interface{}{".keep.com"}, "outbound": "direct", "enabled": true},
				map[string]interface{}{"domain_suffix": []interface{}{".drop.com"}, "outbound": "block", "enabled": false},
			},
		},
	}
	FilterDisabledRoutingRules(full)

	for _, key := range []string{"routing", "route"} {
		section := full[key].(map[string]interface{})
		rules := section["rules"].([]interface{})
		if len(rules) != 1 {
			t.Fatalf("%s: 停用规则应被剔除，剩余 %d 条", key, len(rules))
		}
		if _, has := rules[0].(map[string]interface{})["enabled"]; has {
			t.Fatalf("%s: enabled 字段必须从输出剥离", key)
		}
	}
}

// 热下发路径绕过 PrepareFullConfig：单独下发的 routing 段也必须先清洗，
// 否则停用规则仍会被应用到运行中的 xray。
func TestSanitizeRoutingSection(t *testing.T) {
	raw := []byte(`{"rules":[
		{"domain":["keep.com"],"outboundTag":"direct","enabled":true},
		{"domain":["drop.com"],"outboundTag":"blocked","enabled":false}
	]}`)
	out, err := SanitizeRoutingSection(raw)
	if err != nil {
		t.Fatalf("SanitizeRoutingSection: %v", err)
	}
	var section map[string]interface{}
	if err := json.Unmarshal(out, &section); err != nil {
		t.Fatalf("unmarshal sanitized: %v", err)
	}
	rules := section["rules"].([]interface{})
	if len(rules) != 1 {
		t.Fatalf("停用规则应被剔除，剩余 %d 条", len(rules))
	}
	if _, has := rules[0].(map[string]interface{})["enabled"]; has {
		t.Fatal("enabled 字段必须从热下发 payload 剥离")
	}
}

// sing-box：transport.type 非 tcp 时同样清除。
func TestCleanInboundFieldsStripsVisionOnSingboxGRPC(t *testing.T) {
	inbound := map[string]interface{}{
		"type": "vless",
		"transport": map[string]interface{}{
			"type":         "grpc",
			"service_name": "svc",
		},
		"users": []interface{}{
			map[string]interface{}{"name": "u1", "uuid": "x", "flow": "xtls-rprx-vision"},
		},
	}
	CleanInboundFields(inbound, "sb-36409", "0.0.0.0", "vless", 36409, "singbox")
	users := inbound["users"].([]interface{})
	if _, has := users[0].(map[string]interface{})["flow"]; has {
		t.Fatalf("sing-box gRPC 入站的 flow 应被清除：%+v", users[0])
	}

	// 裸 TCP（没有 transport）保留。
	inbound2 := map[string]interface{}{
		"type": "vless",
		"users": []interface{}{
			map[string]interface{}{"name": "u1", "uuid": "x", "flow": "xtls-rprx-vision"},
		},
	}
	CleanInboundFields(inbound2, "sb-tcp", "0.0.0.0", "vless", 443, "singbox")
	if flow, _ := inbound2["users"].([]interface{})[0].(map[string]interface{})["flow"].(string); flow != "xtls-rprx-vision" {
		t.Fatalf("sing-box 裸 TCP 应保留 vision，实际 flow=%q", flow)
	}
}
