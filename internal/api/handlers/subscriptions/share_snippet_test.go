package subscriptions

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"corefusion/internal/model/entity"
	"corefusion/internal/subscription/formatter"
	"corefusion/internal/subscription/sharelink"
)

func TestNormalizeShareFormat(t *testing.T) {
	cases := map[string]string{
		"": "uri", "uri": "uri", "base64_uri": "uri",
		"mihomo": "mihomo", "mihomo_yaml": "mihomo", "clash": "mihomo", "yaml": "mihomo",
		"singbox": "singbox", "sing-box": "singbox", "singbox_json": "singbox",
		"xray": "xray", "xray_json": "xray",
		" Mihomo ": "mihomo",
	}
	for in, want := range cases {
		if got := normalizeShareFormat(in); got != want {
			t.Errorf("normalizeShareFormat(%q) = %q, want %q", in, got, want)
		}
	}
}

// 片段支持矩阵必须与订阅同一份 —— 否则会出现"订阅里没有、弹窗里能粘"的两套口径。
func TestSnippetSupportMatrixMatchesSubscriptions(t *testing.T) {
	// 实测本机 xray（26.7.28）没有 tuic 出站：写了整份配置都会被拒载
	if snippetSupportMatrix("xray")["tuic"] {
		t.Error("xray 不应支持 tuic（实测 unknown config id: tuic）")
	}
	for fmt, protos := range map[string][]string{
		"mihomo":  {"snell", "ssh", "shadowsocks", "mixed", "hysteria2"},
		"xray":    {"shadowsocks", "socks", "http", "mixed", "hysteria2"},
		"singbox": {"snell", "ssh", "shadowsocks", "mixed", "hysteria2"},
	} {
		m := snippetSupportMatrix(fmt)
		for _, p := range protos {
			if !m[p] {
				t.Errorf("%s 应支持 %s", fmt, p)
			}
		}
	}
	// naive：sing-box 默认构建不带 naive **出站**，混进订阅会被拒载 → 三种格式都不给
	for _, f := range []string{"mihomo", "singbox", "xray"} {
		if snippetSupportMatrix(f)["naive"] {
			t.Errorf("%s 不应支持 naive", f)
		}
	}
}

// 无标准 URI 的协议（snell/ssh）在拿不到链接时必须能由节点直出片段。
func TestMihomoSnippetFromNode(t *testing.T) {
	snell := &formatter.Node{
		Name: "sn1", Protocol: "snell", Host: "1.2.3.4", Port: 443,
		Password: "PSK123", Extra: map[string]interface{}{"version": 4},
	}
	out := mihomoProxySnippetFromNode(snell)
	if !strings.Contains(out, "type: snell") || !strings.Contains(out, "psk: PSK123") {
		t.Errorf("snell 片段应为 type: snell + psk（不是 password）:\n%s", out)
	}

	// mihomo 不支持的协议不给片段（避免生成无效配置）
	if got := mihomoProxySnippetFromNode(&formatter.Node{Protocol: "naive", Host: "1.2.3.4", Port: 443}); got != "" {
		t.Errorf("naive 不应产出 mihomo 片段, got %q", got)
	}
	if got := mihomoProxySnippetFromNode(nil); got != "" {
		t.Errorf("nil 节点应返回空, got %q", got)
	}
}

// sing-box 表达不了的传输（xhttp）必须与订阅一样被判为"不支持"，
// 否则用户拿到的片段会让客户端拒绝加载整份配置。
func TestSingboxSnippetDropsUnsupportedTransport(t *testing.T) {
	node := &formatter.Node{
		Name: "x1", Protocol: "vless", Host: "1.2.3.4", Port: 443,
		Transport: "xhttp", UUID: "11111111-1111-1111-1111-111111111111",
	}
	if _, ok := renderShareSnippet("singbox", "vless://11111111-1111-1111-1111-111111111111@1.2.3.4:443", node); ok {
		t.Error("xhttp 传输不应产出 sing-box 片段")
	}
	// 同一节点在 xray 侧是合法的
	if _, ok := renderShareSnippet("xray", "", node); !ok {
		t.Error("xhttp 在 xray 侧应可渲染")
	}
}

// 逐用户返回多条候选标识：前端选择用户的键与后端 label 口径未必一致（xray=email，
// sing-box=name/username），只给一个 label 会匹配不上 → 片段为空。
func TestInboundUserKeysCoversIdentifiers(t *testing.T) {
	u := inboundUser{label: "user1", record: map[string]interface{}{
		"name": "user1", "username": "user1@x", "email": "user1@x", "id": "uuid-1",
	}}
	keys := inboundUserKeys(u)
	joined := strings.Join(keys, ",")
	for _, want := range []string{"user1", "user1@x", "uuid-1"} {
		if !strings.Contains(joined, want) {
			t.Errorf("候选标识缺 %s: %v", want, keys)
		}
	}
	if keys[0] != strings.ToLower("user1") {
		t.Errorf("首个候选应为 label, got %v", keys)
	}
}

// http/socks 入站的用户存在 settings.accounts[]（{user,pass}）而不是 clients[]，
// 不认这个形状时这类入站"一个用户都没有" → 全部无法生成片段。
func TestInboundUserRecordsReadsAccounts(t *testing.T) {
	config := map[string]interface{}{
		"settings": map[string]interface{}{
			"auth": "password",
			"accounts": []interface{}{
				map[string]interface{}{"user": "u1", "pass": "p1"},
				map[string]interface{}{"user": "u2", "pass": "p2"},
			},
		},
	}
	users := inboundUserRecords(config)
	if len(users) != 2 {
		t.Fatalf("应识别 2 个账号, got %d", len(users))
	}
	if users[0].label != "u1" {
		t.Errorf("label = %q, want u1", users[0].label)
	}
	// 归一成 client 形状，下游（链接生成/节点构造）只认 username/password
	if users[0].record["password"] != "p1" || users[0].record["username"] != "u1" {
		t.Errorf("账号未归一: %v", users[0].record)
	}
}

// ss 的加密方式必须带进链接：只挑 clients/encryption 两个键时 method 会退回默认的
// aes-256-gcm，而面板实际用的是 2022-blake3-*，生成出**连不上**的链接。
func TestShareViewKeepsCipherMethod(t *testing.T) {
	config := map[string]interface{}{
		"protocol": "shadowsocks",
		"settings": map[string]interface{}{
			"method":   "2022-blake3-aes-128-gcm",
			"network":  "tcp",
			"password": "server-key",
			"clients": []interface{}{
				map[string]interface{}{"email": "a@x.com", "password": "user-key"},
				map[string]interface{}{"email": "b@x.com", "password": "other-key"},
			},
		},
	}
	users := inboundUserRecords(config)
	if len(users) != 2 {
		t.Fatalf("用户数 = %d, want 2", len(users))
	}
	view := shareViewForUser("ss-tag", config, users[0])
	settings := getMap(view, "settings")
	if got := getString(settings, "method", ""); got != "2022-blake3-aes-128-gcm" {
		t.Errorf("method = %q（应保留原加密方式）", got)
	}
	// 只带该用户自己的凭证
	clients, _ := settings["clients"].([]interface{})
	if len(clients) != 1 {
		t.Fatalf("clients 应收敛成 1 条, got %d", len(clients))
	}
	if email := getString(firstClient(settings), "email", ""); email != "a@x.com" {
		t.Errorf("client email = %q, want a@x.com", email)
	}
}

// 单用户入站（凭证写在 settings.password，没有 clients[]）此前"一个用户都没有" →
// 四种格式全部 0 条，前端显示"不支持该协议"（mihomo/sing-box/xray 其实都直接支持它）。
func TestInboundUserRecordsSynthesizesSingleUser(t *testing.T) {
	config := map[string]interface{}{
		"protocol": "shadowsocks",
		"settings": map[string]interface{}{
			"method":   "aes-256-gcm",
			"password": "server-pass",
			"network":  "tcp",
		},
	}
	users := inboundUserRecords(config)
	if len(users) != 1 {
		t.Fatalf("单用户 ss 应合成 1 条用户记录, got %d", len(users))
	}
	// label 必须与前端 XrayInbounds.getUsers() 的虚拟用户一致，否则弹窗选中后匹配不上
	if users[0].label != "default" {
		t.Errorf("label = %q, want default", users[0].label)
	}

	view := shareViewForUser("inbound-shadowsocks-1", config, users[0])
	link := sharelink.Generate(sharelink.Node{
		Tag: "inbound-shadowsocks-1", Protocol: "shadowsocks", Host: "1.2.3.4", Port: 12345,
		Settings:       getMap(view, "settings"),
		StreamSettings: getMap(view, "streamSettings"),
	})
	if !strings.HasPrefix(link, "ss://") {
		t.Fatalf("应生成 ss:// 链接, got %q", link)
	}
	want := base64.StdEncoding.EncodeToString([]byte("aes-256-gcm:server-pass"))
	if !strings.Contains(link, want) {
		t.Errorf("链接未带服务端凭据/加密方式（客户端会连不上）: %q", link)
	}
	// mihomo 片段同样必须能生成（用户报的"YAML 显示不支持该协议"）
	if snippet := mihomoProxySnippet(link); !strings.Contains(snippet, "type: ss") {
		t.Errorf("mihomo 片段应含 type: ss:\n%s", snippet)
	}
	if snippet := mihomoProxySnippetFromNode(nodeForSingleUserShare(config, view, users[0])); snippet == "" {
		t.Error("节点直出（兜底路径）也应能生成 mihomo 片段")
	}

	// 无认证的 socks 也要能分享（settings.auth 缺省即 noauth）
	socks := inboundUserRecords(map[string]interface{}{
		"protocol": "socks",
		"settings": map[string]interface{}{"auth": "noauth"},
	})
	if len(socks) != 1 {
		t.Errorf("noauth socks 应合成 1 条用户记录, got %d", len(socks))
	}
	// 有认证但没账号（坏配置）→ 不合成，避免生成无效链接
	if got := inboundUserRecords(map[string]interface{}{
		"protocol": "socks",
		"settings": map[string]interface{}{"auth": "password"},
	}); len(got) != 0 {
		t.Errorf("auth=password 且无账号时不应合成, got %d", len(got))
	}
}

// nodeForSingleUserShare 模拟 InboundShareLinks 里 formatterNodeForShare 的取参（供兜底路径断言）。
func nodeForSingleUserShare(config, view map[string]interface{}, u inboundUser) *formatter.Node {
	settings := getMap(view, "settings")
	return &formatter.Node{
		Name:       "inbound-shadowsocks-1",
		Protocol:   "shadowsocks",
		Host:       "1.2.3.4",
		Port:       12345,
		Password:   getString(firstClient(settings), "password", ""),
		Encryption: getString(settings, "method", ""),
	}
}

// mihomo 对**重名代理是整份拒绝**（不是跳过该条）：
//
//	proxy trojan-worker is the duplicate name → configuration file xxx test failed
//
// 于是用户导入订阅后"很多节点不显示"。命名口径要与 sing-box/xray 的出站 tag 一致。
func TestMihomoProxyMapsDedupeNames(t *testing.T) {
	items := []renderedSubscriptionNode{
		{ProxyMap: map[string]interface{}{"name": "trojan-worker", "type": "trojan", "server": "1.1.1.1", "port": 443}},
		{ProxyMap: map[string]interface{}{"name": "trojan-worker", "type": "trojan", "server": "2.2.2.2", "port": 443}},
		{ProxyMap: map[string]interface{}{"name": "", "type": "ss", "server": "3.3.3.3", "port": 443}},
		{ProxyMap: map[string]interface{}{"name": "trojan-worker 2", "type": "trojan", "server": "5.5.5.5", "port": 443}},
	}
	got := buildRenderedMihomoProxyMaps(items)
	names := getMihomoProxyNames(got)
	want := []string{"trojan-worker", "trojan-worker 2", "node", "trojan-worker 2 2"}
	if len(names) != len(want) {
		t.Fatalf("代理数 = %d, want %d (%v)", len(names), len(want), names)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("第 %d 个代理名 = %q, want %q", i, names[i], want[i])
		}
	}
	// 产出必须是合法 YAML 形状：名字带空格也要原样写出
	if yaml := buildMihomoProxiesYAML(got); !strings.Contains(yaml, "- name: trojan-worker 2") {
		t.Errorf("YAML 未含去重后的名字:\n%s", yaml)
	}
}

// 裸 TCP（raw/tcp）在 mihomo 里**不写 network**：mihomo 的 network 取值只有
// ws/http/h2/grpc/httpupgrade/xhttp（裸 TCP 就是省略该字段），写 raw/tcp 都是
// 客户端不认识的值。两条路（链接转换 / 外部节点 ProxyMap）都要处理。
func TestMihomoOutputOmitsBareTCPTransport(t *testing.T) {
	// 路径①：分享链接（vmess 的传输在 base64 JSON 的 net 字段里）
	body, _ := json.Marshal(map[string]interface{}{
		"v": "2", "ps": "n", "add": "1.2.3.4", "port": 443,
		"id": "11111111-1111-1111-1111-111111111111", "aid": 0, "net": "raw",
	})
	snippet := mihomoProxySnippet("vmess://" + base64.StdEncoding.EncodeToString(body))
	if strings.Contains(snippet, "network:") {
		t.Errorf("裸 TCP 的 mihomo 片段不应出现 network 字段:\n%s", snippet)
	}
	if !strings.Contains(snippet, "type: vmess") {
		t.Errorf("片段本体丢了:\n%s", snippet)
	}

	// 路径②：外部节点直接给的 ProxyMap（不经过链接）
	maps := buildRenderedMihomoProxyMaps([]renderedSubscriptionNode{
		{ProxyMap: map[string]interface{}{
			"name": "ext-node", "type": "trojan", "server": "1.1.1.1", "port": 443,
			"password": "pw", "network": "raw",
		}},
	})
	if len(maps) != 1 {
		t.Fatalf("应保留 1 个代理, got %d", len(maps))
	}
	if got, ok := maps[0]["network"]; ok {
		t.Errorf("外部节点的 network = %v, want 删掉该字段", got)
	}
	// 其它传输不受影响
	maps = buildRenderedMihomoProxyMaps([]renderedSubscriptionNode{
		{ProxyMap: map[string]interface{}{
			"name": "ws-node", "type": "trojan", "server": "1.1.1.1", "port": 443,
			"password": "pw", "network": "ws",
		}},
	})
	if got := maps[0]["network"]; got != "ws" {
		t.Errorf("ws 不应被改写, got %v", got)
	}
}

// mKCP（面板里存 "mkcp"）这类传输**不能**被悄悄改成 tcp：
//   - sing-box / mihomo 表达不了 → 必须整条剔除（原来被改成 tcp，于是漏出一个
//     "参数全丢、看起来能用"的节点，用户拿到的配置连不上）；
//   - xray 表达得了 → 片段必须带上 network: kcp + kcpSettings + finalmask。
func TestKcpTransportIsNotDegradedToTCP(t *testing.T) {
	cfg := &entity.InboundConfig{Tag: "kcp-in", Protocol: "vless", Listen: "0.0.0.0", Port: 54461}
	config := map[string]interface{}{
		"streamSettings": map[string]interface{}{
			"network":  "mkcp",
			"security": "none",
			"kcpSettings": map[string]interface{}{
				"mtu": float64(1350), "tti": float64(50), "congestion": true,
			},
			"finalmask": map[string]interface{}{
				"udp": []interface{}{map[string]interface{}{
					"type":     "mkcp-legacy",
					"settings": map[string]interface{}{"header": "wireguard", "value": ""},
				}},
			},
		},
		"settings": map[string]interface{}{
			"clients": []interface{}{map[string]interface{}{"email": "a@x", "id": "11111111-1111-1111-1111-111111111111"}},
		},
	}
	view := shareViewForUser("kcp-in", config, inboundUser{label: "a@x", record: map[string]interface{}{
		"email": "a@x", "id": "11111111-1111-1111-1111-111111111111",
	}})
	node := formatterNodeForShare("kcp-in", cfg, view, firstClient(getMap(view, "settings")), "1.2.3.4", 54461)
	if node.Transport != "kcp" {
		t.Fatalf("transport = %q, want kcp（mkcp 只做命名归一，不能降级成 tcp）", node.Transport)
	}

	// sing-box / mihomo 都不支持 mKCP → 不产出片段，并给出原因
	if _, ok := renderShareSnippet("singbox", "", node); ok {
		t.Error("sing-box 不应产出 kcp 片段")
	}
	if _, ok := renderShareSnippet("mihomo", "vless://11111111-1111-1111-1111-111111111111@1.2.3.4:54461?type=kcp", node); ok {
		t.Error("mihomo 不应产出 kcp 片段")
	}
	if got := shareUnsupportedReason("mihomo", node); got != "transport=kcp" {
		t.Errorf("reason = %q, want transport=kcp", got)
	}
	if got := shareUnsupportedReason("singbox", node); got != "transport=kcp" {
		t.Errorf("singbox reason = %q, want transport=kcp", got)
	}

	// xray 支持 → 片段必须带全参数
	text, ok := renderShareSnippet("xray", "", node)
	if !ok {
		t.Fatal("xray 应能产出 kcp 片段")
	}
	var ob map[string]interface{}
	if err := json.Unmarshal([]byte(text), &ob); err != nil {
		t.Fatalf("片段不是合法 JSON: %v", err)
	}
	ss, _ := ob["streamSettings"].(map[string]interface{})
	if ss["network"] != "kcp" {
		t.Errorf("network = %v, want kcp", ss["network"])
	}
	if _, has := ss["kcpSettings"]; !has {
		t.Errorf("缺 kcpSettings（mtu/tti 等参数丢了）: %v", ss)
	}
	if _, has := ss["finalmask"]; !has {
		t.Errorf("缺 finalmask（mkcp-legacy 的 header/seed 要在这里）: %v", ss)
	}
}

// JSON 片段应是"单个出站对象"，用户可直接粘进自己的 outbounds 数组。
func TestOutboundJSONSnippetShape(t *testing.T) {
	node := &formatter.Node{Name: "v1", Protocol: "vless", Host: "1.2.3.4", Port: 443,
		UUID: "11111111-1111-1111-1111-111111111111", TLS: true}
	for _, f := range []formatter.Format{formatter.FormatSingBoxJSON, formatter.FormatXrayJSON} {
		text, ok := outboundJSONSnippet(f, node)
		if !ok {
			t.Fatalf("%s 渲染失败", f)
		}
		var ob map[string]interface{}
		if err := json.Unmarshal([]byte(text), &ob); err != nil {
			t.Fatalf("%s 片段不是合法 JSON: %v", f, err)
		}
		if _, err := json.Marshal(ob["outbounds"]); err == nil && ob["outbounds"] != nil {
			t.Errorf("%s 片段应是单个出站，而不是整份配置", f)
		}
		if ob["tag"] == nil {
			t.Errorf("%s 片段缺 tag", f)
		}
	}
}
