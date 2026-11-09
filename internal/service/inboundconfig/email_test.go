package inboundconfig

import (
	"encoding/json"
	"strings"
	"testing"
)

func clientsOf(t *testing.T, cfg string) []map[string]interface{} {
	t.Helper()
	var root map[string]interface{}
	if err := json.Unmarshal([]byte(cfg), &root); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	settings, _ := root["settings"].(map[string]interface{})
	items, _ := settings["clients"].([]interface{})
	out := make([]map[string]interface{}, 0, len(items))
	for _, it := range items {
		if m, ok := it.(map[string]interface{}); ok {
			out = append(out, m)
		}
	}
	return out
}

// xray-ui 迁移来的客户端没有 email → 必须补上，否则 xray 流量统计不到。
func TestEnsureClientEmailsFillsMissing(t *testing.T) {
	cfg := `{"settings":{"clients":[{"id":"11111111-1111-1111-1111-111111111111"},{"id":"22222222-2222-2222-2222-222222222222"}],"decryption":"none"}}`
	out := ensureClientEmails("xray", "inbound-vless-20296", cfg)
	cs := clientsOf(t, out)
	if len(cs) != 2 {
		t.Fatalf("客户端数量变了：%s", out)
	}
	for i, c := range cs {
		email, _ := c["email"].(string)
		if !strings.HasPrefix(email, "u") || !strings.HasSuffix(email, "_vless20296") {
			t.Fatalf("client[%d] email 未按 {base}_{tag后缀} 生成：%q", i, email)
		}
		if !strings.Contains(email, "@cf.local_") {
			t.Fatalf("client[%d] email 形状不对：%q", i, email)
		}
	}
	if cs[0]["email"] == cs[1]["email"] {
		t.Fatalf("不同凭据不能生成同一个 email：%v", cs[0]["email"])
	}
}

// 已有 email 的客户端必须保持原样（不擅自改写存量邮箱）。
func TestEnsureClientEmailsKeepsExisting(t *testing.T) {
	cfg := `{"settings":{"clients":[{"id":"11111111-1111-1111-1111-111111111111","email":"real@example.com_vless20296"}]}}`
	if out := ensureClientEmails("xray", "inbound-vless-20296", cfg); out != cfg {
		t.Fatalf("已有 email 不应被改写：%s", out)
	}
}

// 重复执行必须幂等（保存/迁移重跑不能一直变 email）。
func TestEnsureClientEmailsIdempotent(t *testing.T) {
	cfg := `{"settings":{"clients":[{"password":"p1"}]}}`
	first := ensureClientEmails("xray", "inbound-trojan-443", cfg)
	second := ensureClientEmails("xray", "inbound-trojan-443", first)
	if first != second {
		t.Fatalf("重复执行结果不一致：\n1: %s\n2: %s", first, second)
	}
}

// 同一入站里凭据重复（复制配置）时必须生成不同 email，否则统计会合并。
func TestEnsureClientEmailsDeduplicates(t *testing.T) {
	cfg := `{"settings":{"clients":[{"id":"same-id"},{"id":"same-id"}]}}`
	cs := clientsOf(t, ensureClientEmails("xray", "inbound-vless-1", cfg))
	if len(cs) != 2 || cs[0]["email"] == cs[1]["email"] {
		t.Fatalf("重复凭据应生成不同 email：%v", cs)
	}
}

// sing-box 的统计标识是 users[].name，不走这条 xray email 逻辑。
func TestEnsureClientEmailsSkipsOtherCores(t *testing.T) {
	cfg := `{"inbounds":[{"type":"vless","users":[{"uuid":"x"}]}]}`
	if out := ensureClientEmails("sing-box", "inbound-vless-1", cfg); out != cfg {
		t.Fatalf("sing-box 不应被改写：%s", out)
	}
}

// tag 形如 inbound-vless-20296 → vless20296；短 tag 原样。
func TestInboundEmailSuffix(t *testing.T) {
	cases := map[string]string{
		"inbound-vless-20296": "vless20296",
		"inbound-trojan-443":  "trojan443",
		"inbound-443":         "inbound-443",
		"":                    "",
	}
	for tag, want := range cases {
		if got := inboundEmailSuffix(tag); got != want {
			t.Errorf("inboundEmailSuffix(%q) = %q, want %q", tag, got, want)
		}
	}
}
