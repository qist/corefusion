package subscriptions

import (
	"strings"
	"testing"

	"corefusion/internal/subscription/formatter"
)

// 分享弹窗的用户列表由**后端**给出（`/share-links` 的 users[]，index 指回 items），
// 前端只用它渲染选择框 —— 以前前端自己枚举用户再按 email/name 兜底匹配，
// 两套口径必然漂移（匹配不上就是空片段）。
//
// 同时：**暂停用户在运行中的核里已经不存在**（coreconfig/prepare.go 会把 _paused 的用户
// 从下发配置里删掉），必须由后端过滤掉，否则分享/二维码里会出现"看着能用其实不能用"的条目。
func TestInboundUserRecordsSkipsPausedUsers(t *testing.T) {
	config := map[string]interface{}{
		"protocol": "vless",
		"settings": map[string]interface{}{
			"clients": []interface{}{
				map[string]interface{}{"email": "a@x.com", "id": "11111111-1111-1111-1111-111111111111"},
				map[string]interface{}{"email": "paused@x.com", "id": "22222222-2222-2222-2222-222222222222", "_paused": true},
				map[string]interface{}{"email": "b@x.com", "id": "33333333-3333-3333-3333-333333333333", "_paused": false},
			},
		},
	}
	users := inboundUserRecords(config)
	if len(users) != 2 {
		t.Fatalf("应跳过暂停用户（1 个），got %d: %v", len(users), users)
	}
	if users[0].label != "a@x.com" || users[1].label != "b@x.com" {
		t.Errorf("剩余用户顺序/标签不对: %v %v", users[0].label, users[1].label)
	}

	// 全部暂停 → 没有可用用户（前端会提示"没有可分享的用户"，而不是"协议不支持"）
	allPaused := map[string]interface{}{
		"protocol": "vless",
		"settings": map[string]interface{}{
			"clients": []interface{}{
				map[string]interface{}{"email": "a@x.com", "_paused": true},
			},
		},
	}
	if got := inboundUserRecords(allPaused); len(got) != 0 {
		t.Errorf("全部暂停时不应有用户, got %d", len(got))
	}
}

// xray 26 起对"明文 vless / trojan + 公网地址"是**整份拒载**（实测 26.9.9）：
//
//	vless without TLS or other encryption is prohibited unless the server address is a private IP or domain
//	trojan without TLS is prohibited unless the server address is a private IP...
//
// 注意：报错文案说"or domain"，但**域名同样被拒**（只能连 IP 段/私有地址）。
// 一个这样的节点会让整份配置加载失败 → 必须整条剔除并说明原因，而不是输出"看着能用"的节点。
func TestXrayNodeDropReasonPlainVlessTrojan(t *testing.T) {
	node := func(proto, host string, tls bool, extra map[string]interface{}) *formatter.Node {
		if extra == nil {
			extra = map[string]interface{}{}
		}
		return &formatter.Node{Name: "n", Protocol: proto, Host: host, Port: 443, TLS: tls, Extra: extra}
	}

	cases := []struct {
		name string
		node *formatter.Node
		want string
	}{
		{"vless 明文 + 公网 IP", node("vless", "1.2.3.4", false, nil), "plain-vless"},
		{"vless 明文 + 域名（xray 同样拒）", node("vless", "www.example.com", false, nil), "plain-vless"},
		{"vless 明文 + 回环", node("vless", "127.0.0.1", false, nil), ""},
		{"vless 明文 + 私有网段", node("vless", "192.168.1.10", false, nil), ""},
		{"vless + TLS", node("vless", "1.2.3.4", true, nil), ""},
		{"vless + REALITY", node("vless", "1.2.3.4", false, map[string]interface{}{
			"reality-opts": map[string]interface{}{"public-key": "PBK"},
		}), ""},
		{"vless + VLESS 加密（mlkem）", &formatter.Node{
			Name: "n", Protocol: "vless", Host: "1.2.3.4", Port: 443,
			Encryption: "mlkem768x25519plus.native.0rtt.abc",
		}, ""},
		{"trojan 明文 + 公网 IP", node("trojan", "1.2.3.4", false, nil), "plain-trojan"},
		{"trojan + TLS", node("trojan", "1.2.3.4", true, nil), ""},
		{"vmess 明文 + 公网 IP（不受限制）", node("vmess", "1.2.3.4", false, nil), ""},
		{"shadowsocks 明文 + 公网 IP（不受限制）", node("shadowsocks", "1.2.3.4", false, nil), ""},
	}
	for _, tc := range cases {
		if got := xrayNodeDropReason(tc.node); got != tc.want {
			t.Errorf("%s: xrayNodeDropReason = %q, want %q", tc.name, got, tc.want)
		}
		if got := shareUnsupportedReason("xray", tc.node); got != tc.want {
			t.Errorf("%s: shareUnsupportedReason(xray) = %q, want %q", tc.name, got, tc.want)
		}
	}

	// 弹窗片段：明文 vless + 公网地址 → 不生成（否则用户拿去用会整份拒载）
	link := "vless://11111111-1111-1111-1111-111111111111@1.2.3.4:443?encryption=none&security=none#n"
	if text, ok := renderShareSnippet("xray", link, node("vless", "1.2.3.4", false, nil)); ok || text != "" {
		t.Errorf("明文 vless + 公网地址不该生成 xray 片段, got ok=%v text=%q", ok, text)
	}
	// 私网地址照旧能生成
	if _, ok := renderShareSnippet("xray", link, node("vless", "127.0.0.1", false, nil)); !ok {
		t.Error("私网/回环地址的 vless 应仍能生成 xray 片段")
	}
}

// SS-2022 **多用户**的客户端密码必须是 `服务端密钥:用户密钥`（两段 base64 冒号拼接）。
// 只给用户密钥时：xray 服务端**静默丢弃**连接（ss 认证失败不回错），客户端表现为"连不上"，
// 面板自建的多用户 ss2022 入站（如 id=70/71）全都中招。
// 实测对照：同一个入站同一个用户，冒号形式立即可通；只给用户密钥 / 只给服务端密钥 /
// 把两段密钥按原始字节拼接再 base64 都不通。
func TestSS2022MultiUserClientPassword(t *testing.T) {
	const srv = "qrE42mwHhaa6aa/JPISHIlk+7YP/aIxks/pUHQxwIig="
	const usr = "kLOV9ua9wx9q0i4o7a3oF/Pl4W5vS2HyfR5RgqR9KNo="
	cases := []struct {
		name, method, server, user, want string
	}{
		{"多用户 → 服务端:用户", "2022-blake3-aes-256-gcm", srv, usr, srv + ":" + usr},
		{"单用户（用户密码就是服务端密钥）", "2022-blake3-aes-256-gcm", srv, srv, srv},
		{"客户端没记密码 → 服务端密钥", "2022-blake3-aes-256-gcm", srv, "", srv},
		{"老式 ss 原样", "aes-256-gcm", "", usr, usr},
	}
	for _, tc := range cases {
		if got := ss2022UserPassword(tc.method, tc.server, tc.user); got != tc.want {
			t.Errorf("%s: ss2022UserPassword = %q, want %q", tc.name, got, tc.want)
		}
	}

	// xray 形态：method 与用户密码在 settings，服务端密钥在 settings.password
	xrayView := map[string]interface{}{"settings": map[string]interface{}{
		"method": "2022-blake3-aes-256-gcm", "password": srv,
		"clients": []interface{}{map[string]interface{}{"password": usr}},
	}}
	if got := ss2022ClientPassword(xrayView, "shadowsocks", usr); got != srv+":"+usr {
		t.Errorf("xray 形态 = %q, want %q", got, srv+":"+usr)
	}
	// sing-box 形态：服务端密钥在**顶层** password（settings 里只有 method）
	sbView := map[string]interface{}{
		"password": srv,
		"settings": map[string]interface{}{"method": "2022-blake3-aes-256-gcm"},
	}
	if got := ss2022ClientPassword(sbView, "shadowsocks", usr); got != srv+":"+usr {
		t.Errorf("sing-box 形态 = %q, want %q", got, srv+":"+usr)
	}
	// 非 ss 协议不受影响
	if got := ss2022ClientPassword(map[string]interface{}{"settings": map[string]interface{}{}}, "vless", "pw"); got != "pw" {
		t.Errorf("非 ss 协议不该改密码, got %q", got)
	}
}

// sing-box 入站的 shadowsocks：**method 必须同时写进 `settings.method`**。
// 链接层（sharelink）从视图顶层取 `method`，而节点层（nodeEncryption → xray/sing-box/mihomo
// 片段与订阅）读的是 `settings.method` —— 只写顶层会让 sing-box 入站派生的产物**丢 method**：
// xray 报 `failed to build outbound config`、sing-box 报 `unknown method:`，客户端直接拒载（e2e 实测）。
func TestSingboxSSViewCarriesMethodIntoSettings(t *testing.T) {
	config := map[string]interface{}{
		"type": "shadowsocks", "tag": "sb-ss", "listen": "0.0.0.0", "listen_port": float64(31019),
		"method": "aes-256-gcm", "password": "server-key",
		"users": []interface{}{map[string]interface{}{"name": "a@x.com", "password": "user-key"}},
	}
	view := singboxViewForUser(config, map[string]interface{}{"name": "a@x.com", "password": "user-key"}, "a@x.com")
	if view["method"] != "aes-256-gcm" {
		t.Errorf("视图顶层 method = %v, want aes-256-gcm（链接层要用）", view["method"])
	}
	settings, _ := view["settings"].(map[string]interface{})
	if settings == nil {
		t.Fatal("视图缺 settings")
	}
	if settings["method"] != "aes-256-gcm" {
		t.Errorf("settings.method = %v, want aes-256-gcm（节点层要用，缺了产物就没 method）", settings["method"])
	}
	if got := nodeEncryption("shadowsocks", settings); got != "aes-256-gcm" {
		t.Errorf("nodeEncryption = %q, want aes-256-gcm", got)
	}
}

// 二维码弹窗的"Mihomo 配置"必须与 mihomo 订阅**同源**（同一条 mihomoconv 链路），
// 否则又会出现"弹窗里能粘、订阅里参数不全"的两套口径。
func TestMihomoProxySnippetUsesSubscriptionPipeline(t *testing.T) {
	link := "hysteria2://pw@1.2.3.4:443?obfs=salamander&obfs-password=ob&bbr-profile=conservative" +
		"&mport=10000-19999&security=tls&sni=www.example.com#hy2"
	snippet := mihomoProxySnippet(link)
	if snippet == "" {
		t.Fatal("hy2 链接应能生成片段")
	}
	if !strings.HasPrefix(snippet, "proxies:\n") {
		t.Fatalf("片段应以 proxies: 开头（可直接粘进配置）: %q", snippet)
	}
	for _, want := range []string{
		"type: hysteria2", "server: 1.2.3.4",
		"obfs: salamander", "obfs-password: ob",
		"bbr-profile: conservative", "ports: 10000-19999",
	} {
		if !strings.Contains(snippet, want) {
			t.Errorf("片段缺 %q:\n%s", want, snippet)
		}
	}

	// 与订阅链路的结果逐字段一致（同一份转换输出）
	converted := convertLinksToMihomoProxyMaps([]string{link})
	if len(converted) != 1 {
		t.Fatalf("转换应得到 1 个代理, got %d", len(converted))
	}
	if _, ok := converted[0]["obfs-password"]; !ok {
		t.Error("订阅链路应保留 obfs-password")
	}

	// 没有链接（无标准 URI 的协议）→ 空片段，前端显示占位提示
	if got := mihomoProxySnippet(""); got != "" {
		t.Errorf("空链接应返回空片段, got %q", got)
	}
}
