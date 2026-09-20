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

// xray 出站的 settings 不能是空对象 —— 之前只处理 vless/vmess/trojan/hysteria2，
// ss / socks / http / mixed 一律写成 `settings: {}`，客户端会以
// `"servers" should have one and only one member` 拒绝加载**整份**配置。
func TestXrayFormatterSettingsForServersProtocols(t *testing.T) {
	for _, c := range []struct {
		node     Node
		wantProt string
	}{
		{Node{Name: "ss1", Protocol: "shadowsocks", Host: "1.2.3.4", Port: 443,
			Password: "pw", Encryption: "2022-blake3-aes-128-gcm"}, "shadowsocks"},
		{Node{Name: "s1", Protocol: "socks", Host: "1.2.3.4", Port: 1080, Password: "pw"}, "socks"},
		{Node{Name: "m1", Protocol: "mixed", Host: "1.2.3.4", Port: 1080}, "socks"},
		{Node{Name: "h1", Protocol: "http", Host: "1.2.3.4", Port: 8080}, "http"},
	} {
		ob := renderFirstXrayOutbound(t, c.node)
		if ob["protocol"] != c.wantProt {
			t.Errorf("%s: protocol = %v, want %s", c.node.Name, ob["protocol"], c.wantProt)
		}
		settings, _ := ob["settings"].(map[string]interface{})
		servers, _ := settings["servers"].([]interface{})
		if len(servers) != 1 {
			t.Fatalf("%s: settings.servers 应恰好 1 条（空 settings 会让 xray 拒载整份配置）: %#v",
				c.node.Name, ob["settings"])
		}
		server, _ := servers[0].(map[string]interface{})
		if server["address"] != "1.2.3.4" || server["port"] == nil {
			t.Errorf("%s: servers[0] 缺 address/port: %v", c.node.Name, server)
		}
		if c.node.Protocol == "shadowsocks" {
			// 字段名是 method（不是 mihomo 的 cipher；写错报 unknown cipher method）
			if server["method"] != "2022-blake3-aes-128-gcm" || server["password"] != "pw" {
				t.Errorf("ss: servers[0] 应带 method/password: %v", server)
			}
		}
	}
}

// 面板早期生成 shadowsocks:// 链接；两个解析器（xray 订阅 / mihomo 订阅）都必须认这个别名，
// 否则整批 ss 节点会被静默丢出订阅。
func TestParseURIToNodeAcceptsShadowsocksAlias(t *testing.T) {
	for _, uri := range []string{
		"ss://YWVzLTI1Ni1nY206cHc=@1.2.3.4:443#t",
		"shadowsocks://YWVzLTI1Ni1nY206cHc=@1.2.3.4:443#t",
		"shadowsocks2022://YWVzLTI1Ni1nY206cHc=@1.2.3.4:443#t",
	} {
		node := ParseURIToNode(uri)
		if node == nil {
			t.Fatalf("解析失败: %s", uri)
		}
		if node.Password != "pw" || node.Host != "1.2.3.4" {
			t.Errorf("%s: node = %+v", uri, node)
		}
		if node.Protocol != "shadowsocks" && node.Protocol != "ss" {
			t.Errorf("%s: protocol = %s", uri, node.Protocol)
		}
	}
}
