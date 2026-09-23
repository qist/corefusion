package certificate

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func anyJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// 成功返回 ✓（字段名与 Google 官方一致 ✓：keyId / b64MacKey ✓）
func TestParseGoogleEABSuccess(t *testing.T) {
	r, err := parseGoogleEABResponse([]byte(`{"keyId":"kid-g-1","b64MacKey":"mac-g-1"}`))
	if err != nil {
		t.Fatal(err)
	}
	if r.KeyID != "kid-g-1" || r.B64MacKey != "mac-g-1" {
		t.Fatalf("解析结果不对：%+v", r)
	}
}

// 权限不足 ✓ → 必须提示"要授 roles/publicca.externalAccountKeyCreator" ✓
func TestParseGoogleEABPermissionDenied(t *testing.T) {
	body := []byte(`{"error":{"code":403,"message":"The caller does not have permission","status":"PERMISSION_DENIED"}}`)
	_, err := parseGoogleEABResponse(body)
	if err == nil {
		t.Fatal("必须报错 ✗")
	}
	if !strings.Contains(err.Error(), "PERMISSION_DENIED") || !strings.Contains(err.Error(), "externalAccountKeyCreator") {
		t.Fatalf("应提示需要的角色 ✓，实际：%v", err)
	}
}

// API 未启用 ✓
func TestParseGoogleEABServiceDisabled(t *testing.T) {
	body := []byte(`{"error":{"code":403,"message":"Public CA API has not been used","status":"SERVICE_DISABLED"}}`)
	_, err := parseGoogleEABResponse(body)
	if err == nil || !strings.Contains(err.Error(), "publicca.googleapis.com") {
		t.Fatalf("应提示要先启用 API ✓，实际：%v", err)
	}
}

func TestParseGoogleEABMalformed(t *testing.T) {
	if _, err := parseGoogleEABResponse([]byte("<html>503</html>")); err == nil {
		t.Fatal("非 JSON 必须报错 ✗")
	}
}

// 项目 ID 必填 ✓（不联网就该报错 ✓）
func TestFetchGoogleEABRequiresProject(t *testing.T) {
	svc := setupEABSvc(t)
	if _, err := svc.FetchGoogleEAB(context.Background(), "  ", `{}`, "fake-token"); err == nil {
		t.Fatal("缺少 project id 必须报错 ✗")
	}
}

// EnsureGoogleEAB：已有可用 EAB → 直接用 ✓，不重复申请 ✓
func TestEnsureGoogleEABReturnsExisting(t *testing.T) {
	svc := setupEABSvc(t)
	ctx := context.Background()
	if _, err := svc.Create(ctx, EABConfigInput{
		Name: "g", CAProvider: "google", KeyID: "k", MACKey: "m",
	}); err != nil {
		t.Fatal(err)
	}
	cfg, fresh, err := svc.EnsureGoogleEAB(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil || cfg.KeyID != "k" {
		t.Fatalf("应返回已有一份 ✓：%+v", cfg)
	}
	if fresh {
		t.Fatalf("已有可用 EAB 时不该重新申请 ✗")
	}
}

// EnsureGoogleEAB：完全没配 → 返回空 ✓（调用方给提示 ✓，不 panic ✓）
func TestEnsureGoogleEABNoConfig(t *testing.T) {
	svc := setupEABSvc(t)
	cfg, fresh, err := svc.EnsureGoogleEAB(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if cfg != nil || fresh {
		t.Fatalf("没配就该返回空 ✓：%+v %v", cfg, fresh)
	}
}

// 服务账号 JSON 是凭据 ✗ → 永不外泄 ✓（只回 has_google_sa ✓），但库里要留着 ✓
func TestEABConfigNeverLeaksGoogleSA(t *testing.T) {
	svc := setupEABSvc(t)
	ctx := context.Background()
	secret := `{"type":"service_account","private_key":"SUPER-SECRET-KEY"}`
	c, err := svc.Create(ctx, EABConfigInput{
		Name: "g2", CAProvider: "google", KeyID: "k2", MACKey: "m2",
		GoogleProjectID: "proj-1", GoogleSAJSON: secret,
	})
	if err != nil {
		t.Fatal(err)
	}
	list, err := svc.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(anyJSON(list), "SUPER-SECRET-KEY") {
		t.Fatalf("服务账号私钥泄漏到接口响应 ✗：%s", anyJSON(list))
	}
	if !strings.Contains(anyJSON(list), `"has_google_sa":true`) {
		t.Fatalf("应给前端 has_google_sa 标记 ✓：%s", anyJSON(list))
	}
	got, err := svc.GetByID(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.GoogleSAJSON != secret {
		t.Fatalf("DB 里应保留服务账号（供自动再申请 ✓）")
	}
}
