package certificate

import (
	"strings"
	"testing"
)

// 成功响应 ✓（字段名与 acme.sh 解析的完全一致 ✓：eab_kid / eab_hmac_key ✓）
func TestParseZeroSSLEABSuccess(t *testing.T) {
	body := []byte(`{"success":true,"eab_kid":"kid-abc","eab_hmac_key":"hmac-xyz"}`)
	r, err := parseZeroSSLEABResponse(body)
	if err != nil {
		t.Fatal(err)
	}
	if r.EABKID != "kid-abc" || r.EABHMAC != "hmac-xyz" {
		t.Fatalf("解析结果不对：%+v", r)
	}
}

// 失败响应 ✓（实测形状：{"success":false,"error":{"code":2900,"type":"missing_email"}} ✓）
func TestParseZeroSSLEABError(t *testing.T) {
	body := []byte(`{"success":false,"error":{"code":2900,"type":"missing_email","info":null}}`)
	_, err := parseZeroSSLEABResponse(body)
	if err == nil {
		t.Fatal("失败响应必须报错 ✗")
	}
	if !strings.Contains(err.Error(), "missing_email") || !strings.Contains(err.Error(), "2900") {
		t.Fatalf("错误信息应带上 type/code 便于排查 ✓，实际：%v", err)
	}
}

func TestParseZeroSSLEABMalformed(t *testing.T) {
	if _, err := parseZeroSSLEABResponse([]byte("<html>502</html>")); err == nil {
		t.Fatal("非 JSON 必须报错 ✗")
	}
}

// success=true 但缺字段 → 也必须报错 ✓（否则会存一份空 EAB 进去 ✗）
func TestParseZeroSSLEABIncomplete(t *testing.T) {
	if _, err := parseZeroSSLEABResponse([]byte(`{"success":true,"eab_kid":"k"}`)); err == nil {
		t.Fatal("缺 eab_hmac_key 必须报错 ✗")
	}
}
