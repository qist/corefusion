package clientauth

import (
	"encoding/json"
	"testing"
)

// mkCallbackBody 复刻充值站点的回调 body ✓ ——
// 字段与书写顺序无关 ✓：Go 的 map 序列化按 key 字典序 ✓，所以 sign 会落在中间 ✓。
func mkCallbackBody() map[string]any {
	return map[string]any{
		"trade_no":      "CF202609231248324aaeeec3",
		"site_trade_no": "MOCK123456",
		"status":        "paid", // 不参与签名 ✓
		"amount":        int64(1),
		"currency":      "CNY",
		"timestamp":     int64(1758600000),
		"nonce":         "0123456789abcdef0123456789abcdef",
		"paid_currency": "CNY",
		"paid_amount":   int64(1),
		"method":        "alipay",
		"exchange_rate": 1.0,
		"paid_at":       "2026-09-23T12:48:32Z",
	}
}

// TestStripTopLevelSignMember 是这次真机排查的核心回归 ✓：
//
// 充值站点按官方对接文档做：先对「**不含 sign** 的 body 字节」算 body_hash ✓，
// 再把 sign 追加进 body 发送 ✓。面板必须能把 sign 成员原样摘掉 ✓，
// 还原出**逐字节相同**的 body ✓，否则 body_hash 永远对不上 ✗ → 回调恒 400 INVALID_SIGNATURE ✗。
//
// ⚠ 这里曾有一个真实 bug ✗：剥 sign 时从 `InputOffset()-1` 往回找分隔逗号 ✗，
// 而 `json.Decoder.InputOffset()` 返回的是"上一个 token **结束之后**"的偏移 ✓
// （正好落在分隔逗号上 ✓）→ 多退一位 → 把**前一个成员一起删掉** ✗。
// 该 bug 就是被本测试的第一条断言抓出来的 ✓。
func TestStripTopLevelSignMember(t *testing.T) {
	body := mkCallbackBody()
	withoutSign, err := json.Marshal(body) // 站点用来算 body_hash 的字节 ✓
	if err != nil {
		t.Fatal(err)
	}
	body["sign"] = "deadbeef" // 实际发出的 body 多了 sign ✓
	withSign, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}

	got, ok := stripTopLevelSignMember(withSign)
	if !ok {
		t.Fatalf("没找到 sign 成员 ✗ body=%s", withSign)
	}
	if string(got) != string(withoutSign) {
		t.Fatalf("还原字节不一致 ✗\n  got=%s\n want=%s", got, withoutSign)
	}
	var check map[string]any
	if err := json.Unmarshal(got, &check); err != nil {
		t.Fatalf("还原结果不是合法 JSON ✗ %v：%s", err, got)
	}
	if _, exists := check["sign"]; exists {
		t.Fatal("sign 没被摘掉 ✗")
	}
	if len(check) != len(body)-1 {
		t.Fatalf("成员数不对 ✗ got=%d want=%d：%s", len(check), len(body)-1, got)
	}
}

// TestStripTopLevelSignMember_SignFirst 覆盖 sign 是**第一个成员**的情况 ✓：
// 此时必须保留 `{`、吃掉"后置逗号"✓；若把前后两个逗号都吃掉 ✗，前后成员会被粘在一起 ✗。
func TestStripTopLevelSignMember_SignFirst(t *testing.T) {
	const (
		withoutSign = `{"amount":1,"currency":"CNY"}`
		withSign    = `{"sign":"deadbeef","amount":1,"currency":"CNY"}`
	)
	got, ok := stripTopLevelSignMember([]byte(withSign))
	if !ok {
		t.Fatal("没找到 sign 成员 ✗")
	}
	if string(got) != withoutSign {
		t.Fatalf("还原字节不一致 ✗\n  got=%s\n want=%s", got, withoutSign)
	}
}

// TestStripTopLevelSignMember_SignInMiddle 覆盖 sign 夹在中间的情况 ✓（键序由客户端决定 ✓）。
func TestStripTopLevelSignMember_SignInMiddle(t *testing.T) {
	const (
		withoutSign = `{"trade_no":"CF1","amount":1,"currency":"CNY"}`
		withSign    = `{"trade_no":"CF1","sign":"deadbeef","amount":1,"currency":"CNY"}`
	)
	got, ok := stripTopLevelSignMember([]byte(withSign))
	if !ok {
		t.Fatal("没找到 sign 成员 ✗")
	}
	if string(got) != withoutSign {
		t.Fatalf("还原字节不一致 ✗\n  got=%s\n want=%s", got, withoutSign)
	}
}

// TestStripTopLevelSignMember_NoSign 没有 sign 成员时应返回 ok=false ✓（调用方会退回旧口径 ✓）。
func TestStripTopLevelSignMember_NoSign(t *testing.T) {
	if _, ok := stripTopLevelSignMember([]byte(`{"a":1}`)); ok {
		t.Fatal("没有 sign 时不应返回 ok=true ✗")
	}
	if _, ok := stripTopLevelSignMember([]byte(`not json`)); ok {
		t.Fatal("非 JSON 不应返回 ok=true ✗")
	}
}

// TestCallbackBodyHashCandidates 确认两个候选各自对应什么 ✓：
// stripped = 「加 sign 之前」的字节 ✓（官方文档口径 ✓）、full = 收到的完整字节 ✓（旧口径 ✓）。
func TestCallbackBodyHashCandidates(t *testing.T) {
	body := mkCallbackBody()
	withoutSign, _ := json.Marshal(body)
	body["sign"] = "deadbeef"
	withSign, _ := json.Marshal(body)

	stripped, full := callbackBodyHashCandidates(withSign)
	if stripped != sha256Hex(withoutSign) {
		t.Fatalf("stripped 哈希不对 ✗ got=%s want=%s", stripped, sha256Hex(withoutSign))
	}
	if full != sha256Hex(withSign) {
		t.Fatalf("full 哈希不对 ✗ got=%s want=%s", full, sha256Hex(withSign))
	}
	if stripped == full {
		t.Fatal("两个候选不应相同 ✗")
	}
}

// TestVerifyChargeCallback_DocStyle 端到端复刻"站点按文档签名"→ 面板必须验签通过 ✓。
// 这正是修复前**必然失败**的场景 ✓（旧实现只对完整 body 取哈希 ✗ → 循环依赖 ✗）。
func TestVerifyChargeCallback_DocStyle(t *testing.T) {
	const secret = "cs_test_secret"
	svc := &CommerceService{}

	body := mkCallbackBody()
	withoutSign, _ := json.Marshal(body)
	bodyHash := sha256Hex(withoutSign)

	input := ChargeCallbackInput{
		TradeNo:     body["trade_no"].(string),
		SiteTradeNo: body["site_trade_no"].(string),
		Status:      "paid",
		Amount:      body["amount"].(int64),
		Currency:    body["currency"].(string),
		Timestamp:   body["timestamp"].(int64),
		Nonce:       body["nonce"].(string),
	}
	// 站点侧签名 ✓（8 字段字典序 ✓）—— 用服务自己的签名函数复刻同一套载荷 ✓
	input.Sign = svc.signChargePayload(map[string]any{
		"amount":        input.Amount,
		"body_hash":     bodyHash,
		"currency":      input.Currency,
		"nonce":         input.Nonce,
		"site_trade_no": input.SiteTradeNo,
		"timestamp":     input.Timestamp,
		"trade_no":      input.TradeNo,
		"version":       "v1",
	}, secret)

	body["sign"] = input.Sign
	withSign, _ := json.Marshal(body)
	stripped, _ := callbackBodyHashCandidates(withSign)

	if !svc.verifyChargeCallback(input, stripped, secret) {
		t.Fatal("按文档签名的回调应该验签通过 ✗（stripped 候选）")
	}
	if svc.verifyChargeCallback(input, sha256Hex(withSign), "wrong-secret") {
		t.Fatal("错误密钥不应通过 ✗")
	}
	// 篡改 body 后（金额变了）旧哈希必须失效 ✓
	tampered := mkCallbackBody()
	tampered["sign"] = input.Sign
	tampered["amount"] = int64(999)
	tamperedBytes, _ := json.Marshal(tampered)
	strippedTampered, _ := callbackBodyHashCandidates(tamperedBytes)
	if svc.verifyChargeCallback(input, strippedTampered, secret) {
		t.Fatal("body 被篡改后不应验签通过 ✗")
	}
}
