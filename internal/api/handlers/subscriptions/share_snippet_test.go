package subscriptions

import (
	"encoding/json"
	"strings"
	"testing"

	"corefusion/internal/subscription/formatter"
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
