package client

import (
	"testing"
)

// 入站 tag 改过（inbound-vmess-36833 → inbound-vmess-36832）时，存量账号后缀仍是旧 tag。
// 全量同步账号必须认出「该客户已有账号」，否则会给同一客户再加一条 email → 重复节点。
func TestInboundEmailTakenForClientStaleTagSuffix(t *testing.T) {
	configJSON := `{
		"protocol": "vmess",
		"settings": {
			"clients": [
				{"email": "seed@xray.local", "id": "seed"},
				{"email": "user@example.com_vmess28307", "id": "11111111-1111-1111-1111-111111111111"}
			]
		}
	}`
	if !inboundEmailTakenForClient(configJSON, "user@example.com", "inbound-vmess-36832") {
		t.Fatal("旧 tag 后缀的存量账号应被识别为已存在（避免重复添加）")
	}
}

// 正常路径：后缀与当前 tag 一致。
func TestInboundEmailTakenForClientExactSuffix(t *testing.T) {
	configJSON := `{"settings":{"clients":[{"email":"user@example.com_inbound-51878"}]}}`
	if !inboundEmailTakenForClient(configJSON, "user@example.com", "inbound-51878") {
		t.Fatal("精确后缀应命中")
	}
	if inboundEmailTakenForClient(configJSON, "other@x.com", "inbound-51878") {
		t.Fatal("其它客户不应命中")
	}
}

// 裸 email（无后缀）也算已存在。
func TestInboundEmailTakenForClientPlainEmail(t *testing.T) {
	configJSON := `{"settings":{"clients":[{"email":"user@example.com"}]}}`
	if !inboundEmailTakenForClient(configJSON, "user@example.com", "inbound-1") {
		t.Fatal("裸 email 应命中")
	}
}

// sing-box 形态：顶层 users[].name。
func TestInboundEmailTakenForClientSingboxName(t *testing.T) {
	configJSON := `{"type":"vmess","users":[{"name":"user@example.com_vmess28307","uuid":"u"}]}`
	if !inboundEmailTakenForClient(configJSON, "user@example.com", "inbound-vmess-36832") {
		t.Fatal("sing-box 顶层 users[].name 应命中")
	}
}

// 前缀必须有下划线边界：相似邮箱不能被当成同一客户。
func TestInboundEmailTakenForClientPrefixBoundary(t *testing.T) {
	configJSON := `{"settings":{"clients":[{"email":"user@example.com.evil_vmess1"}]}}`
	if inboundEmailTakenForClient(configJSON, "user@example.com", "inbound-vmess-1") {
		t.Fatal("相似前缀不应命中")
	}
}

// 配置解析失败按「不存在」处理（与 userExistsInConfig 一致）。
func TestInboundEmailTakenForClientInvalidJSON(t *testing.T) {
	if inboundEmailTakenForClient("not json", "user@example.com", "inbound-1") {
		t.Fatal("非法 JSON 应返回 false")
	}
}
