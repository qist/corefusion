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

// 容错：只粘"最小字段集"时自动补 type / token_uri ✓（少两行不该报错 ✗）
func TestNormalizeServiceAccountJSONMinimal(t *testing.T) {
	raw := `{"client_email":"a@b.iam.gserviceaccount.com","private_key":"-----BEGIN PRIVATE KEY-----\nx\n-----END PRIVATE KEY-----\n"}`
	out, err := normalizeServiceAccountJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"type":"service_account"`) || !strings.Contains(out, "oauth2.googleapis.com/token") {
		t.Fatalf("应自动补 type/token_uri ✓：%s", out)
	}
}

// 粘成 API Key ✓ → 必须说清"这是 API Key，没用"，并给出替代路径 ✓
func TestNormalizeServiceAccountJSONRejectsAPIKey(t *testing.T) {
	_, err := normalizeServiceAccountJSON("AIzaSyD-abcdefghijklmnop")
	if err == nil {
		t.Fatal("API Key 必须被拒绝 ✗")
	}
	if !strings.Contains(err.Error(), "API Key") || !strings.Contains(err.Error(), "EAB-KID") {
		t.Fatalf("提示应讲清它是 API Key 并给出替代方案 ✓：%v", err)
	}
}

// 完整 JSON 但缺 client_email/private_key ✓ → 提示要的是密钥文件 ✓
func TestNormalizeServiceAccountJSONMissingFields(t *testing.T) {
	_, err := normalizeServiceAccountJSON(`{"type":"service_account","project_id":"p"}`)
	if err == nil || !strings.Contains(err.Error(), "client_email") {
		t.Fatalf("应指出缺 client_email / private_key ✓：%v", err)
	}
}

// 非 JSON（比如只粘了一行 / 粘了文件路径 ✓）→ 明确提示要"整个文件内容" ✓
func TestNormalizeServiceAccountJSONNotJSON(t *testing.T) {
	_, err := normalizeServiceAccountJSON("/root/corefusion-eab.json")
	if err == nil || !strings.Contains(err.Error(), "完整内容") {
		t.Fatalf("应提示要给整个文件内容：%v", err)
	}
}

// 空 ✓
func TestNormalizeServiceAccountJSONEmpty(t *testing.T) {
	if _, err := normalizeServiceAccountJSON("   "); err == nil {
		t.Fatal("空必须报错 ✗")
	}
}

// 申请失败时也要**把服务账号存下来** ✓（否则用户填的白丢 ✗，且修好 IAM 后无法"自动再申请" ✗）
func TestSaveGoogleAuthOnFailurePath(t *testing.T) {
	svc := setupEABSvc(t)
	ctx := context.Background()

	// ① 有同名 Google 行 → 就地补上项目 ID / 服务账号 ✓，且不动已有 KID/MAC ✓
	existing, err := svc.Create(ctx, EABConfigInput{
		Name: "Google Public CA - proj-x", CAProvider: "google", KeyID: "k-old", MACKey: "m-old",
	})
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := svc.saveGoogleAuth(ctx, "proj-x", `{"type":"service_account","client_email":"a@b.iam"}`)
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil || cfg.GoogleProjectID != "proj-x" || cfg.GoogleSAJSON == "" {
		t.Fatalf("应存下项目 ID 与服务账号 ✓：%+v", cfg)
	}
	if cfg.ID != existing.ID || cfg.KeyID != "k-old" || cfg.MACKey != "m-old" {
		t.Fatalf("不该动已有 KID/MAC ✓：%+v", cfg)
	}

	// ② 只有一份"没名字对上、但已存过服务账号"的 Google 行 → 也认它 ✓
	svc2 := setupEABSvc(t)
	if _, err := svc2.Create(ctx, EABConfigInput{
		Name: "随便起的名字", CAProvider: "google", KeyID: "k2", MACKey: "m2", GoogleSAJSON: `{"a":1}`,
	}); err != nil {
		t.Fatal(err)
	}
	got, err := svc2.saveGoogleAuth(ctx, "proj-y", "")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.GoogleProjectID != "proj-y" {
		t.Fatalf("应更新那份已存过服务账号的配置 ✓：%+v", got)
	}

	// ③ 完全没有 → 新建一份 ✓
	svc3 := setupEABSvc(t)
	made, err := svc3.saveGoogleAuth(ctx, "proj-z", `{"type":"service_account"}`)
	if err != nil {
		t.Fatal(err)
	}
	if made == nil || made.CAProvider != "google" || made.GoogleProjectID != "proj-z" {
		t.Fatalf("应新建一份 Google 配置 ✓：%+v", made)
	}
	if made.Name != "Google Public CA - proj-z" {
		t.Fatalf("名称约定不符：%s", made.Name)
	}

	// ④ 两者都空 → 不做事 ✓（返回 nil,nil ✓）
	if c, err := svc3.saveGoogleAuth(ctx, "", ""); err != nil || c != nil {
		t.Fatalf("空输入应直接返回 ✓：%+v %v", c, err)
	}
}

// 保存路径（Create/Update 的入参）必须能存下 GCP 两字段 ✓ —— 这正是用户"填了却丢了"的根因 ✗
func TestEABConfigInputCarriesGoogleAuth(t *testing.T) {
	svc := setupEABSvc(t)
	ctx := context.Background()
	c, err := svc.Create(ctx, EABConfigInput{
		Name: "g-save", CAProvider: "google", KeyID: "k", MACKey: "m",
		GoogleProjectID: "proj-1", GoogleSAJSON: `{"type":"service_account"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.GoogleProjectID != "proj-1" || c.GoogleSAJSON == "" || !c.HasGoogleSA {
		t.Fatalf("Create 应存下 GCP 授权 ✓：%+v", c)
	}
	upd, err := svc.Update(ctx, c.ID, EABConfigInput{
		Name: "g-save", CAProvider: "google", KeyID: "k", MACKey: "",
		GoogleProjectID: "proj-2",
	})
	if err != nil {
		t.Fatal(err)
	}
	if upd.GoogleProjectID != "proj-2" {
		t.Fatalf("Update 应更新项目 ID ✓：%+v", upd)
	}
	if upd.GoogleSAJSON == "" {
		t.Fatalf("Update 留空服务账号时应保持原值 ✓：%+v", upd)
	}
}
