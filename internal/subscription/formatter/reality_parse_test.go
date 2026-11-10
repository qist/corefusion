package formatter

import (
	"encoding/json"
	"testing"
)

// 链接里的 pqv（ML-DSA-65 校验公钥）必须解析进 reality-opts，否则 xray_json /
// sing-box 订阅丢掉 mldsa65Verify；服务端开了 mldsa65Seed 时客户端校验会失败。
func TestParseVlessURIKeepsMldsa65Verify(t *testing.T) {
	uri := "vless://11111111-1111-1111-1111-111111111111@a.example.com:443" +
		"?encryption=none&security=reality&pbk=PUBKEY&sid=abcd&pqv=PQVERIFY&sni=www.example.com" +
		"&fp=chrome&type=grpc&serviceName=svc#n"
	node := ParseURIToNode(uri)
	if node == nil {
		t.Fatal("解析失败")
	}
	ro, _ := node.Extra["reality-opts"].(map[string]interface{})
	if ro == nil {
		t.Fatalf("缺 reality-opts: %#v", node.Extra)
	}
	if ro["public-key"] != "PUBKEY" || ro["mldsa65-verify"] != "PQVERIFY" || ro["short-id"] != "abcd" {
		t.Fatalf("reality-opts 不完整: %#v", ro)
	}

	raw, err := (&XrayFormatter{}).Format([]Node{*node})
	if err != nil {
		t.Fatalf("Format: %v", err)
	}
	var doc struct {
		Outbounds []map[string]interface{} `json:"outbounds"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil || len(doc.Outbounds) == 0 {
		t.Fatalf("出站 JSON 非法: %v", err)
	}
	ss, _ := doc.Outbounds[0]["streamSettings"].(map[string]interface{})
	if ss["security"] != "reality" {
		t.Fatalf("security = %v", ss["security"])
	}
	rs, _ := ss["realitySettings"].(map[string]interface{})
	if rs["mldsa65Verify"] != "PQVERIFY" || rs["password"] != "PUBKEY" || rs["shortId"] != "abcd" {
		t.Fatalf("realitySettings 不完整: %#v", rs)
	}
}

// trojan 的 REALITY 链接此前完全没解析：security=reality / pbk / sid 全丢，
// 订阅里退化成普通 TLS。这里做端到端断言。
func TestParseTrojanURIKeepsReality(t *testing.T) {
	uri := "trojan://pass@a.example.com:443?security=reality&pbk=PUBKEY&sid=abcd&pqv=PQVERIFY" +
		"&sni=www.example.com&fp=chrome&type=grpc&serviceName=svc#n"
	node := ParseURIToNode(uri)
	if node == nil {
		t.Fatal("解析失败")
	}
	ro, _ := node.Extra["reality-opts"].(map[string]interface{})
	if ro == nil || ro["public-key"] != "PUBKEY" || ro["mldsa65-verify"] != "PQVERIFY" {
		t.Fatalf("trojan 未解析 REALITY: %#v", node.Extra)
	}
	if node.ServerName != "www.example.com" {
		t.Fatalf("SNI = %q", node.ServerName)
	}

	raw, err := (&XrayFormatter{}).Format([]Node{*node})
	if err != nil {
		t.Fatalf("Format: %v", err)
	}
	var doc struct {
		Outbounds []map[string]interface{} `json:"outbounds"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil || len(doc.Outbounds) == 0 {
		t.Fatalf("出站 JSON 非法: %v", err)
	}
	ss, _ := doc.Outbounds[0]["streamSettings"].(map[string]interface{})
	if ss["security"] != "reality" {
		t.Fatalf("trojan security = %v, want reality", ss["security"])
	}
	rs, _ := ss["realitySettings"].(map[string]interface{})
	if rs["mldsa65Verify"] != "PQVERIFY" {
		t.Fatalf("trojan realitySettings 缺 mldsa65Verify: %#v", rs)
	}
}
