package inbounduser

import "testing"

// 改过 tag 的入站：精确匹配失败时按客户前缀兜底，且能取到该账号的真实 email。
func TestFindEntryFallsBackToStaleTag(t *testing.T) {
	configJSON := `{
		"protocol": "vmess",
		"settings": {
			"clients": [
				{"email": "4wo1rz@xray.com", "id": "seed"},
				{"email": "user@example.com_vmess28307", "id": "u1"}
			]
		}
	}`
	entry := FindEntry(configJSON, "user@example.com", "inbound-vmess-36832")
	if entry == nil || entry["id"] != "u1" {
		t.Fatalf("应兜底命中旧后缀账号, got %v", entry)
	}
	if got := ResolveEmail(configJSON, "user@example.com", "inbound-vmess-36832"); got != "user@example.com_vmess28307" {
		t.Fatalf("ResolveEmail = %q", got)
	}
}

func TestFindEntryPrefersExact(t *testing.T) {
	configJSON := `{"settings":{"clients":[
		{"email":"user@example.com_vmess28307","id":"old"},
		{"email":"user@example.com_vmess36832","id":"new"}
	]}}`
	entry := FindEntry(configJSON, "user@example.com", "inbound-vmess-36832")
	if entry == nil || entry["id"] != "new" {
		t.Fatalf("应优先精确匹配, got %v", entry)
	}
}

// 没有该客户账号时返回 nil / 空串：绝不借用别人的凭据。
func TestFindEntryDoesNotLeakOthers(t *testing.T) {
	configJSON := `{"settings":{"clients":[{"email":"other@x.com_inbound-1"},{"email":"third@y.com"}]}}`
	if entry := FindEntry(configJSON, "user@example.com", "inbound-1"); entry != nil {
		t.Fatalf("不应命中别人的账号: %v", entry)
	}
	if got := ResolveEmail(configJSON, "user@example.com", "inbound-1"); got != "" {
		t.Fatalf("ResolveEmail 应返回空串, got %q", got)
	}
}

func TestFindEntryPrefixBoundary(t *testing.T) {
	configJSON := `{"settings":{"clients":[{"email":"user@example.com.evil_inbound-1"}]}}`
	if entry := FindEntry(configJSON, "user@example.com", "inbound-1"); entry != nil {
		t.Fatalf("相似前缀不应命中: %v", entry)
	}
}

// sing-box 形态：顶层 users[].name。
func TestFindEntrySingboxUsers(t *testing.T) {
	configJSON := `{"users":[{"name":"user@example.com_vmess28307","uuid":"u1"}]}`
	entry := FindEntry(configJSON, "user@example.com", "inbound-vmess-36832")
	if entry == nil || entry["uuid"] != "u1" {
		t.Fatalf("应命中 sing-box 账号, got %v", entry)
	}
	if got := EntryEmail(entry); got != "user@example.com_vmess28307" {
		t.Fatalf("EntryEmail = %q", got)
	}
}

func TestFindEntryPlainEmailAndInvalidJSON(t *testing.T) {
	if entry := FindEntry(`{"settings":{"clients":[{"email":"user@example.com"}]}}`, "user@example.com", "inbound-1"); entry == nil {
		t.Fatal("裸 email 应命中")
	}
	if entry := FindEntry("not json", "user@example.com", "inbound-1"); entry != nil {
		t.Fatal("非法 JSON 应返回 nil")
	}
	if entry := FindEntry(`{"settings":{"clients":[]}}`, "", "inbound-1"); entry != nil {
		t.Fatal("空客户邮箱应返回 nil")
	}
}

func TestEmail(t *testing.T) {
	cases := map[string]string{
		Email("a@b.com", "inbound-vless-443"):    "a@b.com_vless443",
		Email("a@b.com", "inbound-hysteria-123"): "a@b.com_hysteria123",
		Email("a@b.com", "inbound-443"):          "a@b.com_inbound-443",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("Email = %q, want %q", got, want)
		}
	}
}
