package coreconfig

import (
	"encoding/json"
	"os"
	"testing"
)

// TestConvertChainOutboundsToSingbox_Hysteria2AndReality 覆盖链式改造新增的两条路径：
//  1. 出口是**已有的 hysteria2 入站**（只有 sing-box 入口能用 —— xray 没有 hy2 出站）
//  2. 出口是**已有的 reality 入站**（复用真实入站时必须把 publicKey/shortId/指纹带过去）
//
// 转换结果会被 dump 到 /tmp/chain_converted.json，供 `sing-box check` 做真实核心校验。
func TestConvertChainOutboundsToSingbox_Hysteria2AndReality(t *testing.T) {
	in := []interface{}{
		map[string]interface{}{
			"tag":      "chain-11-22",
			"protocol": "hysteria2",
			"settings": map[string]interface{}{
				"servers": []interface{}{
					map[string]interface{}{
						"address": "1.2.3.4", "port": float64(31443), "password": "pw1",
						"obfs": map[string]interface{}{"type": "salamander", "password": "obfspw"},
					},
				},
			},
			"streamSettings": map[string]interface{}{
				"network": "tcp", "security": "reality",
				"realitySettings": map[string]interface{}{
					"serverName": "www.bing.com", "publicKey": "REPLACE_PUBKEY", "shortId": "ab12", "fingerprint": "chrome",
				},
			},
		},
		// ③ 由**真实 sing-box hy2 入站**（type=hysteria2 + users[].password + obfs + tls{enabled,server_name,alpn}）
		// 派生出来的中间形状 —— agent 侧安全层探测必须是 TLS（hy2 没有 transport，曾经因此漏掉 tls ✗）
		map[string]interface{}{
			"tag":      "chain-118-1",
			"protocol": "hysteria2",
			"settings": map[string]interface{}{
				"servers": []interface{}{
					map[string]interface{}{
						"address": "exit.example.com", "port": float64(47180), "password": "m3VBCBsUpHwGlOXc",
						"obfs": map[string]interface{}{"type": "salamander", "password": "MEcFyeCK2bKnohdNoVBqiwHkgNYGOGQ0"},
					},
				},
			},
			"streamSettings": map[string]interface{}{
				"network": "tcp", "security": "tls",
				// agent 的 applyExitInboundTLS 把 SNI 写在 tlsSettings（老路径写顶层）
				"tlsSettings": map[string]interface{}{"serverName": "example.com"},
				"alpn":        []interface{}{"h3"},
			},
		},
		map[string]interface{}{
			"tag":      "chain-11-23",
			"protocol": "vless",
			"settings": map[string]interface{}{
				"vnext": []interface{}{
					map[string]interface{}{
						"address": "1.2.3.5", "port": float64(443),
						"users": []interface{}{map[string]interface{}{"id": "6b1a1c2d-0000-4000-8000-000000000001", "encryption": "none"}},
					},
				},
			},
			"streamSettings": map[string]interface{}{
				"network": "tcp", "security": "reality",
				"realitySettings": map[string]interface{}{
					"serverName": "www.bing.com", "publicKey": "REPLACE_PUBKEY", "shortId": "ab12",
				},
			},
		},
	}

	out := convertChainOutboundsToSingbox(in)
	if len(out) != 3 {
		t.Fatalf("期望 3 条出站，得到 %d", len(out))
	}

	byTag := map[string]map[string]interface{}{}
	for _, o := range out {
		if m, ok := o.(map[string]interface{}); ok {
			if tg, _ := m["tag"].(string); tg != "" {
				byTag[tg] = m
			}
		}
	}
	find := func(tag string) map[string]interface{} {
		m, ok := byTag[tag]
		if !ok {
			t.Fatalf("找不到出站 %s（实际：%v）", tag, byTag)
		}
		return m
	}

	hy2 := find("chain-11-22")
	if hy2["type"] != "hysteria2" || hy2["server"] != "1.2.3.4" || hy2["password"] != "pw1" {
		t.Errorf("hy2 出站字段不对: %v", hy2)
	}
	tls, _ := hy2["tls"].(map[string]interface{})
	if tls == nil || tls["enabled"] != true {
		t.Fatalf("hy2 必须带 tls（实测缺了核心会 FATAL TLS required）: %v", hy2)
	}
	if tls["server_name"] != "www.bing.com" {
		t.Errorf("hy2 tls.server_name 丢失: %v", tls)
	}
	reality, _ := tls["reality"].(map[string]interface{})
	if reality == nil || reality["public_key"] == nil {
		t.Errorf("hy2 reality 参数丢失: %v", tls)
	}
	// reality 客户端**必须**有 uTLS（实测 uTLS is required by reality client）
	if utls, _ := tls["utls"].(map[string]interface{}); utls == nil || utls["enabled"] != true {
		t.Errorf("reality 必须带 utls: %v", tls)
	}
	if obfs, _ := hy2["obfs"].(map[string]interface{}); obfs == nil || obfs["password"] != "obfspw" {
		t.Errorf("hy2 obfs 丢失: %v", hy2)
	}

	vl := find("chain-11-23")
	vTls, _ := vl["tls"].(map[string]interface{})
	if vTls == nil || vTls["server_name"] != "www.bing.com" {
		t.Errorf("vless 的 tls 丢失: %v", vl)
	}
	if _, ok := vTls["reality"].(map[string]interface{}); !ok {
		t.Errorf("vless 的 reality 丢失: %v", vTls)
	}
	if u, _ := vTls["utls"].(map[string]interface{}); u == nil || u["enabled"] != true {
		t.Errorf("vless reality 缺少 utls: %v", vTls)
	}

	// ③ 真实 hy2 入站派生：必须带 tls + SNI + alpn（缺 tls → 核心 `FATAL TLS required`）
	hy2b := find("chain-118-1")
	if hy2b["type"] != "hysteria2" || hy2b["server_port"] != float64(47180) {
		t.Errorf("hy2(真实入站)字段不对: %v", hy2b)
	}
	bTls, _ := hy2b["tls"].(map[string]interface{})
	if bTls == nil || bTls["enabled"] != true {
		t.Fatalf("hy2(真实入站)必须带 tls: %v", hy2b)
	}
	if bTls["server_name"] != "example.com" {
		t.Errorf("hy2(真实入站) SNI 丢失（tlsSettings.serverName 没被读到）: %v", bTls)
	}
	if alpn, _ := bTls["alpn"].([]interface{}); len(alpn) != 1 || alpn[0] != "h3" {
		t.Errorf("hy2(真实入站) alpn 丢失: %v", bTls)
	}

	// dump 给 `sing-box check` 做真实核心校验
	dump := map[string]interface{}{
		"log":       map[string]interface{}{"level": "warn"},
		"outbounds": out,
	}
	b, _ := json.MarshalIndent(dump, "", "  ")
	if err := os.WriteFile("/tmp/chain_converted.json", b, 0o644); err != nil {
		t.Logf("dump 失败（不影响断言）: %v", err)
	}
}

// TestConvertChainOutboundsToSingbox_GrpcServiceName 回归：gRPC 出口的 service_name
// 必须从 streamSettings.grpcSettings 带进 sing-box 出站的 transport.service_name ——
// 漏了它核心不报错，只是永远连不上（真机：sing-box 出口 gRPC）。
func TestConvertChainOutboundsToSingbox_GrpcServiceName(t *testing.T) {
	in := []interface{}{
		map[string]interface{}{
			"tag": "chain-grpc", "protocol": "vless",
			"settings": map[string]interface{}{"vnext": []interface{}{
				map[string]interface{}{"address": "1.2.3.4", "port": float64(443),
					"users": []interface{}{map[string]interface{}{"id": "11111111-1111-1111-1111-111111111111", "encryption": "none"}}},
			}},
			"streamSettings": map[string]interface{}{
				"network": "grpc", "security": "tls",
				"grpcSettings": map[string]interface{}{"serviceName": "ov2zy5xlnlmi"},
			},
		},
	}
	out := convertChainOutboundsToSingbox(in)
	if len(out) != 1 {
		t.Fatalf("期望 1 条出站，得到 %d", len(out))
	}
	m, _ := out[0].(map[string]interface{})
	tr, _ := m["transport"].(map[string]interface{})
	if tr == nil || tr["type"] != "grpc" || tr["service_name"] != "ov2zy5xlnlmi" {
		t.Fatalf("gRPC service_name 未带进 transport: %#v", m)
	}
}

// TestConvertChainOutboundsToSingbox_TransportNames 传输名映射：
// xray 的 `raw`（= TCP 的新叫法）**不能**透传成 sing-box 的 transport.type ✗ ——
// 透传会让核心拒绝启动：`FATAL ... unknown transport type: raw`（真机 E2E 撞到过）。
// TCP 在 sing-box 里就是"没有 transport"字段；ws 这类才写 transport。
func TestConvertChainOutboundsToSingbox_TransportNames(t *testing.T) {
	in := []interface{}{
		map[string]interface{}{
			"tag": "t-raw", "protocol": "vless",
			"settings": map[string]interface{}{"vnext": []interface{}{
				map[string]interface{}{"address": "1.2.3.4", "port": float64(443),
					"users": []interface{}{map[string]interface{}{"id": "11111111-1111-1111-1111-111111111111", "encryption": "none"}}},
			}},
			"streamSettings": map[string]interface{}{"network": "raw", "security": "none"},
		},
		map[string]interface{}{
			"tag": "t-tcp", "protocol": "vless",
			"settings": map[string]interface{}{"vnext": []interface{}{
				map[string]interface{}{"address": "1.2.3.5", "port": float64(443),
					"users": []interface{}{map[string]interface{}{"id": "11111111-1111-1111-1111-111111111111", "encryption": "none"}}},
			}},
			"streamSettings": map[string]interface{}{"network": "tcp", "security": "none"},
		},
		map[string]interface{}{
			"tag": "t-ws", "protocol": "vless",
			"settings": map[string]interface{}{"vnext": []interface{}{
				map[string]interface{}{"address": "1.2.3.6", "port": float64(443),
					"users": []interface{}{map[string]interface{}{"id": "11111111-1111-1111-1111-111111111111", "encryption": "none"}}},
			}},
			"streamSettings": map[string]interface{}{"network": "ws", "security": "none", "path": "/ws"},
		},
	}
	out := convertChainOutboundsToSingbox(in)
	if len(out) != 3 {
		t.Fatalf("期望 3 条出站，得到 %d", len(out))
	}
	for _, o := range out {
		m, _ := o.(map[string]interface{})
		tag, _ := m["tag"].(string)
		tr, has := m["transport"]
		switch tag {
		case "t-raw", "t-tcp":
			if has {
				t.Errorf("%s: TCP/raw 不应有 transport（sing-box 无 raw 类型），得到 %#v", tag, tr)
			}
		case "t-ws":
			tm, _ := tr.(map[string]interface{})
			if !has || tm == nil || tm["type"] != "ws" || tm["path"] != "/ws" {
				t.Errorf("t-ws: transport 应为 {type:ws,path:/ws}，得到 %#v", tr)
			}
		}
	}
}

// TestConvertChainOutboundsToSingbox_VlessFlow 回归：vless 出站的 flow（xtls-rprx-vision）
// 必须从 xray 形态的用户带进 sing-box 出站的顶层 flow 字段。
//
// 真机事故：面板（SingboxUsers / xray vless 入站）默认给 vless 用户写 flow=xtls-rprx-vision，
// 出口按 vision 握手；链式出站丢了 flow 后握手对不上 → 永远连不上，而 `sing-box check`
// 与 `xray run -test` **都不报错**（flow 是可选项）—— 典型的静默失效。
func TestConvertChainOutboundsToSingbox_VlessFlow(t *testing.T) {
	in := []interface{}{
		map[string]interface{}{
			"tag": "chain-flow", "protocol": "vless",
			"settings": map[string]interface{}{"vnext": []interface{}{
				map[string]interface{}{"address": "1.2.3.4", "port": float64(443),
					"users": []interface{}{map[string]interface{}{
						"id": "11111111-1111-1111-1111-111111111111", "encryption": "none", "flow": "xtls-rprx-vision",
					}}},
			}},
			"streamSettings": map[string]interface{}{"network": "tcp", "security": "tls"},
		},
	}
	out := convertChainOutboundsToSingbox(in)
	if len(out) != 1 {
		t.Fatalf("期望 1 条出站，得到 %d", len(out))
	}
	m, _ := out[0].(map[string]interface{})
	if m["flow"] != "xtls-rprx-vision" {
		t.Fatalf("vless flow 未带进 sing-box 出站（出口按 vision 握手会连不上）: %#v", m)
	}
}
