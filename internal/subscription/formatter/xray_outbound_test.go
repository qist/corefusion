package formatter

import (
	"encoding/json"
	"testing"
)

// renderFirstXrayOutbound 渲染单个节点并取出 xray 出站。
func renderFirstXrayOutbound(t *testing.T, node Node) map[string]interface{} {
	t.Helper()
	data, err := (&XrayFormatter{}).Format([]Node{node})
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Outbounds []map[string]interface{} `json:"outbounds"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Outbounds) == 0 {
		t.Fatal("no outbounds")
	}
	return cfg.Outbounds[0]
}

// Xray 25+ 要求 VLESS 的每个 user 显式带 encryption（至少 "none"）：
// 缺了会报 `VLESS users: please add/set "encryption":"none" for every user`，
// 让**整份**客户端配置加载失败（实测 11/11 份订阅都因此挂掉）。
func TestXrayFormatterVlessPerUserEncryption(t *testing.T) {
	for _, enc := range []string{"", "none", "mlkem768x25519plus.native.0rtt.abc"} {
		ob := renderFirstXrayOutbound(t, Node{
			Name: "v", Protocol: "vless", Host: "1.2.3.4", Port: 443,
			UUID: "11111111-1111-1111-1111-111111111111", Encryption: enc,
		})
		settings, _ := ob["settings"].(map[string]interface{})
		vnext, _ := settings["vnext"].([]interface{})
		if len(vnext) == 0 {
			t.Fatal("vnext 缺失")
		}
		entry, _ := vnext[0].(map[string]interface{})
		users, _ := entry["users"].([]interface{})
		if len(users) == 0 {
			t.Fatalf("users 缺失: %#v", entry["users"])
		}
		user, _ := users[0].(map[string]interface{})
		want := enc
		if want == "" {
			want = "none"
		}
		if user["encryption"] != want {
			t.Errorf("encryption=%q 时 user.encryption = %v, want %q", enc, user["encryption"], want)
		}
	}
}

// 客户端 REALITY 用 serverName（单数）；serverNames 是服务端字段，
// Xray 25+ 会直接报 `non-empty "serverNames", please use "serverName" instead`。
func TestXrayFormatterRealityUsesServerName(t *testing.T) {
	ob := renderFirstXrayOutbound(t, Node{
		Name: "r", Protocol: "vless", Host: "1.2.3.4", Port: 443,
		UUID: "11111111-1111-1111-1111-111111111111", ServerName: "www.apple.com",
		Extra: map[string]interface{}{
			"reality-opts":       map[string]interface{}{"public-key": "PBK", "short-id": "SID"},
			"client-fingerprint": "chrome",
		},
	})
	ss, _ := ob["streamSettings"].(map[string]interface{})
	rs, _ := ss["realitySettings"].(map[string]interface{})
	if rs["serverName"] != "www.apple.com" {
		t.Errorf("reality 应写 serverName: %v", rs["serverName"])
	}
	if _, has := rs["serverNames"]; has {
		t.Errorf("reality 不能写 serverNames（Xray 25+ 直接拒载）: %v", rs["serverNames"])
	}
	if rs["publicKey"] != "PBK" || rs["shortId"] != "SID" || rs["fingerprint"] != "chrome" {
		t.Errorf("reality 参数不全: %v", rs)
	}
}

// mkcp 的 header/seed 已被 Xray 25+ 移除（`feature mkcp header & seed has been removed`），
// 要写成 finalmask.udp 的 mkcp-legacy 条目（与服务端运行配置一致）。
func TestXrayFormatterKCPMigratesToFinalMask(t *testing.T) {
	ob := renderFirstXrayOutbound(t, Node{
		Name: "k", Protocol: "vless", Host: "1.2.3.4", Port: 54461,
		UUID: "11111111-1111-1111-1111-111111111111", Encryption: "none", Transport: "kcp",
		Extra: map[string]interface{}{
			"kcp-settings": map[string]interface{}{
				"mtu": 1350, "tti": 50, "headerType": "wireguard", "seed": "SwOzlkxeZpKaCxWysYucYQ==",
			},
		},
	})
	ss, _ := ob["streamSettings"].(map[string]interface{})
	kcp, _ := ss["kcpSettings"].(map[string]interface{})
	if _, has := kcp["header"]; has {
		t.Errorf("kcpSettings 不能带 header（Xray 25+ 已移除）: %v", kcp["header"])
	}
	if _, has := kcp["seed"]; has {
		t.Errorf("kcpSettings 不能带 seed（Xray 25+ 已移除）: %v", kcp["seed"])
	}
	if kcp["mtu"] != float64(1350) {
		t.Errorf("kcpSettings.mtu 应保留: %v", kcp["mtu"])
	}
	fm, _ := ss["finalmask"].(map[string]interface{})
	udp, _ := fm["udp"].([]interface{})
	var gotHeader, gotSeed bool
	for _, item := range udp {
		m, _ := item.(map[string]interface{})
		if m["type"] != "mkcp-legacy" {
			continue
		}
		st, _ := m["settings"].(map[string]interface{})
		if st["header"] == "wireguard" {
			gotHeader = true
		}
		if st["value"] == "SwOzlkxeZpKaCxWysYucYQ==" {
			gotSeed = true
		}
	}
	if !gotHeader || !gotSeed {
		t.Fatalf("mkcp-legacy 未正确迁移: %s", mustJSON(t, fm))
	}
}

func mustJSON(t *testing.T, v interface{}) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
