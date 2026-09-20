package client

import (
	"encoding/json"
	"testing"

	"corefusion/internal/model/entity"
)

// sing-box 入站的用户字段与 xray 不同（顶层 users[]、字段 name/uuid/password），
// 字段映射错了会导致下发后无法认证，这里逐协议锁住。
func TestSingboxUserEntryFields(t *testing.T) {
	cases := []struct {
		protocol string
		wantKeys []string
		notKeys  []string
	}{
		{"vless", []string{"name", "uuid", "flow"}, []string{"password"}},
		{"vmess", []string{"name", "uuid"}, []string{"password"}},
		{"trojan", []string{"name", "password"}, []string{"uuid"}},
		{"hysteria2", []string{"name", "password"}, []string{"uuid"}},
		{"tuic", []string{"name", "uuid", "password"}, nil},
		{"shadowsocks", []string{"name", "password"}, []string{"uuid"}},
		{"anytls", []string{"name", "password"}, []string{"uuid"}},
		{"snell", []string{"name", "psk"}, []string{"uuid"}},
		{"naive", []string{"name", "username", "password"}, nil},
		{"mixed", []string{"name", "username", "password"}, nil},
	}

	for _, tc := range cases {
		u := singboxUserEntry(tc.protocol, "user@example.com", "uuid-1", "pass-1", "xtls-rprx-vision", "aes-128-gcm")
		if u["name"] != "user@example.com" {
			t.Errorf("%s: name 应为订阅邮箱, got %v", tc.protocol, u["name"])
		}
		for _, k := range tc.wantKeys {
			if _, ok := u[k]; !ok {
				t.Errorf("%s: 缺少字段 %s (%v)", tc.protocol, k, u)
			}
		}
		for _, k := range tc.notKeys {
			if _, ok := u[k]; ok {
				t.Errorf("%s: 不应包含字段 %s (%v)", tc.protocol, k, u)
			}
		}
		if _, ok := u["_paused"]; !ok {
			t.Errorf("%s: 缺少 _paused 标记（面板用它过滤暂停用户）", tc.protocol)
		}
	}
}

// vless 无 flow 时不应写出空的 flow 字段（sing-box 对空串会报错）
func TestSingboxUserEntrySkipsEmptyFlow(t *testing.T) {
	u := singboxUserEntry("vless", "a@b.c", "uuid-1", "", "", "")
	if _, ok := u["flow"]; ok {
		t.Error("flow 为空时不应写入")
	}
	if _, ok := u["password"]; ok {
		t.Error("vless 不应写入 password")
	}
}

// 入站归属核心的判断：core_type 优先，兼容「无 settings 但有顶层 users」的历史数据。
func TestIsSingboxInbound(t *testing.T) {
	cases := []struct {
		name     string
		coreType string
		config   string
		want     bool
	}{
		{"core_type=singbox", "singbox", `{"type":"vless","users":[]}`, true},
		{"core_type=xray", "xray", `{"settings":{"clients":[]}}`, false},
		{"无 core_type，顶层 users（sing-box 形态）", "", `{"type":"trojan","users":[]}`, true},
		{"无 core_type，有 settings（xray 形态）", "", `{"settings":{"clients":[]}}`, false},
		{"空配置", "", `{}`, false},
		{"非法 JSON", "", `not-json`, false},
	}
	for _, tc := range cases {
		ib := &entity.InboundConfig{CoreType: tc.coreType, ConfigJSON: tc.config}
		if got := isSingboxInbound(ib); got != tc.want {
			t.Errorf("%s: isSingboxInbound = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// sing-box 的用户标识是 name，xray 是 email —— 查找/删除/暂停都必须兼容两者。
func TestUserContainersAndLookup(t *testing.T) {
	singboxConfig := map[string]interface{}{
		"type": "vless",
		"users": []interface{}{
			map[string]interface{}{"name": "sub@example.com", "uuid": "u1"},
		},
	}
	if findUserEntry(singboxConfig, "sub@example.com") == nil {
		t.Error("sing-box：按 name 查找用户失败")
	}
	if !setUserPausedInConfig(singboxConfig, "sub@example.com", true) {
		t.Error("sing-box：设置 _paused 失败")
	}
	if u := findUserEntry(singboxConfig, "sub@example.com"); u["_paused"] != true {
		t.Errorf("sing-box：_paused 未生效: %v", u)
	}
	if !deleteUserFromConfig(singboxConfig, "sub@example.com") {
		t.Error("sing-box：删除用户失败")
	}
	if users := singboxConfig["users"].([]interface{}); len(users) != 0 {
		t.Errorf("sing-box：删除后仍有用户 %v", users)
	}

	xrayConfig := map[string]interface{}{
		"settings": map[string]interface{}{
			"clients": []interface{}{
				map[string]interface{}{"email": "x@example.com", "id": "uuid"},
			},
		},
	}
	if findUserEntry(xrayConfig, "x@example.com") == nil {
		t.Error("xray：按 email 查找用户失败")
	}
	if !deleteUserFromConfig(xrayConfig, "x@example.com") {
		t.Error("xray：删除用户失败")
	}
	settings := xrayConfig["settings"].(map[string]interface{})
	if clients := settings["clients"].([]interface{}); len(clients) != 0 {
		t.Errorf("xray：删除后仍有用户 %v", clients)
	}
}

// 解析 sing-box 入站的协议类型：优先 type 字段，其次从 tag 推断。
func TestSingboxProtocolOf(t *testing.T) {
	cfg := map[string]interface{}{"type": "hysteria2"}
	if got := singboxProtocolOf(cfg, "whatever"); got != "hysteria2" {
		t.Errorf("singboxProtocolOf = %s, want hysteria2", got)
	}
	// 无 type 时回退 tag 推断
	if got := singboxProtocolOf(map[string]interface{}{}, "inbound-trojan-12345"); got != "trojan" {
		t.Errorf("singboxProtocolOf(tag) = %s, want trojan", got)
	}
}

// 生成的用户 JSON 必须能被解析（避免写入非法结构）
func TestSingboxUserEntryMarshalable(t *testing.T) {
	cfg := map[string]interface{}{"type": "tuic", "users": []interface{}{}}
	u := singboxUserEntry("tuic", "a@b.c", "uuid", "pass", "", "")
	cfg["users"] = append(cfg["users"].([]interface{}), u)
	if _, err := json.Marshal(cfg); err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
}
