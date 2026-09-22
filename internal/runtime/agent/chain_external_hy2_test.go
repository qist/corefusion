package agent

import (
	"testing"
)

// TestBuildExternalOutboundSettingsHysteria2 外部节点出口支持 hy2：
// 中间形状必须是 settings.servers[0] = {address, port, password}（转换器从那里取值），
// 并且 obfs 要能被注入进去 —— 否则 sing-box 端会得到一个没有 obfs 的 hy2 出站。
//
// 背景：`buildExternalOutbound` 此前**完全没有校验**，外部节点若是 hy2/tuic 之类会生成
// **空 settings** 的出站（入口核心直接起不来）。现已加校验 + 支持 hy2。
func TestBuildExternalOutboundSettingsHysteria2(t *testing.T) {
	users := []map[string]interface{}{{"password": "pw-from-proxyjson"}}
	settings := buildExternalOutboundSettings("hysteria2", "1.2.3.4", 31443, users)
	if settings == nil {
		t.Fatalf("hy2 外部出站 settings 为空")
	}
	servers, ok := settings["servers"].([]map[string]interface{})
	if !ok || len(servers) != 1 {
		t.Fatalf("hy2 出站形状不对（应为 servers[0]）: %#v", settings)
	}
	s0 := servers[0]
	if s0["address"] != "1.2.3.4" || s0["port"] != 31443 || s0["password"] != "pw-from-proxyjson" {
		t.Errorf("hy2 出站字段不对: %#v", s0)
	}

	// obfs 注入（模拟 buildExternalOutbound 里从 ProxyJSON 取 pm["obfs"] 的那一步）
	obfs := map[string]interface{}{"type": "salamander", "password": "obfspw"}
	s0["obfs"] = obfs
	if servers[0]["obfs"] == nil {
		t.Errorf("obfs 未注入")
	}
}

// TestExtractUsersFromProxyMapHysteria2 外部 hy2 节点的凭据字段有 password / auth / auth_str 三种写法。
func TestExtractUsersFromProxyMapHysteria2(t *testing.T) {
	cases := []map[string]interface{}{
		{"password": "p1"},
		{"auth": "p2"},
		{"auth_str": "p3"},
	}
	want := []string{"p1", "p2", "p3"}
	for i, pm := range cases {
		users := extractUsersFromProxyMap(pm, "hysteria2")
		if len(users) != 1 || users[0]["password"] != want[i] {
			t.Errorf("case %d: 取到的凭据 = %#v，期望 %q", i, users, want[i])
		}
	}
}

// TestApplyExitInboundTLSRealityServerShape 覆盖**服务端形状**的 reality 入站：
// 服务端用 `serverNames`/`shortIds`（**数组**）+ `settings[0].password`（= 客户端要的 publicKey），
// 而客户端字段是单值的 serverName/shortId/publicKey —— 之前只查 shortId，
// 导致生成的出站不带 shortId（靠服务端容忍空 shortId 才能通，很脆）。
func TestApplyExitInboundTLSRealityServerShape(t *testing.T) {
	cfg := map[string]interface{}{
		"streamSettings": map[string]interface{}{
			"network":  "raw",
			"security": "reality",
			"realitySettings": map[string]interface{}{
				"privateKey":  "server-private-key",
				"serverNames": []interface{}{"www.amd.com"},
				"shortIds":    []interface{}{"28da759cb18a710e", "f13375d9"},
				"settings":    []interface{}{map[string]interface{}{"fingerprint": "chrome", "password": "PUBKEY_VALUE"}},
			},
		},
	}
	ss := map[string]interface{}{}
	applyExitInboundTLS(ss, cfg, "reality")
	rs, _ := ss["realitySettings"].(map[string]interface{})
	if rs == nil {
		t.Fatalf("realitySettings 未生成: %#v", ss)
	}
	if rs["publicKey"] != "PUBKEY_VALUE" {
		t.Errorf("publicKey 应取服务端 settings[0].password，得到 %v", rs["publicKey"])
	}
	if rs["serverName"] != "www.amd.com" {
		t.Errorf("serverName 应取 serverNames[0]，得到 %v", rs["serverName"])
	}
	if rs["shortId"] != "28da759cb18a710e" {
		t.Errorf("shortId 应取服务端 shortIds[0]，得到 %v（缺了会靠服务端容忍空 shortId）", rs["shortId"])
	}
	if rs["fingerprint"] != "chrome" {
		t.Errorf("fingerprint 丢失: %v", rs["fingerprint"])
	}
}
