package certificate

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

func newChannelTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("内存 SQLite 不可用（跳过）：%v", err)
	}
	if err := db.AutoMigrate(&entity.DNSChannel{}); err != nil {
		t.Skipf("迁移失败（跳过）：%v", err)
	}
	return db
}

func newChannelSvc(t *testing.T) (*DNSChannelService, *repository.DNSChannelRepository) {
	t.Helper()
	repo := repository.NewDNSChannelRepository(newChannelTestDB(t))
	return NewDNSChannelService(repo), repo
}

func preview(s string) string {
	if len(s) <= 24 {
		return s
	}
	return s[:24] + "…"
}

// DNS 渠道凭据（含谷歌 GCP service account JSON）必须**加密落库**，
// 且与充值卡共用同一把密钥（internal/security/crypto ← CF_SECRET_ENCRYPTION_KEY）。
func TestDNSChannelCredentialsEncryptedAtRest(t *testing.T) {
	svc, repo := newChannelSvc(t)
	ctx := context.Background()

	const token = "y3whwemmJU78D-IhZPekX-DWaIcbJ22fifPe3g7S"
	gcpJSON := `{
  "type": "service_account",
  "project_id": "corefusion-test",
  "private_key_id": "abc123",
  "private_key": "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkq\n-----END PRIVATE KEY-----\n"
}`
	want := map[string]string{
		"GCE_PROJECT":              "corefusion-test",
		"GCE_SERVICE_ACCOUNT_FILE": gcpJSON,
		"CF_Token":                 token,
	}

	ch, err := svc.Create(ctx, "gcp", "google", want)
	if err != nil {
		t.Fatal(err)
	}

	// ① 落库必须是 enc: 前缀的 AES-256-GCM 密文（与充值卡同一实现 ✓）
	if !strings.HasPrefix(ch.EnvVars, "enc:") {
		t.Fatalf("凭据必须以加密格式落库，实际 = %q", preview(ch.EnvVars))
	}

	// ② 库里绝不能再出现明文：token、GCP 私钥、连"裸 base64(整个 JSON)"都不行
	rawJSON, _ := json.Marshal(want)
	for _, plain := range []string{
		token,
		"BEGIN PRIVATE KEY",
		"corefusion-test",
		base64.StdEncoding.EncodeToString(rawJSON),
	} {
		if strings.Contains(ch.EnvVars, plain) {
			t.Errorf("密文中仍能看到明文片段 %q", plain)
		}
	}

	// ③ 读回来必须逐键一致（多行 GCP JSON 也要原样还原 ✓）
	got, err := svc.GetEnvVars(ctx, ch.ID)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("键 %s 还原不一致：want %q, got %q", k, v, got[k])
		}
	}

	// ④ 列表接口只回掩码，且不能泄露明文 ✓
	list, err := svc.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("期望 1 条渠道，实际 %d", len(list))
	}
	masked, _ := list[0]["env_vars"].(map[string]string)
	if masked["CF_Token"] == token || strings.Contains(masked["CF_Token"], token) {
		t.Errorf("列表回显泄露了明文凭据：%q", masked["CF_Token"])
	}
	if !strings.Contains(masked["GCE_SERVICE_ACCOUNT_FILE"], "***") {
		t.Errorf("GCP JSON 也必须被掩码，实际 %q", masked["GCE_SERVICE_ACCOUNT_FILE"])
	}

	// ⑤ 转发给签发流程（ResolveDNSChannel 走的就是 GetEnvVars ✓）同样解得出明文 ✓
	row, err := repo.GetByID(ctx, ch.ID)
	if err != nil {
		t.Fatal(err)
	}
	vars, err := svc.GetEnvVarsRaw(row.EnvVars)
	if err != nil {
		t.Fatal(err)
	}
	if vars["CF_Token"] != token {
		t.Errorf("GetEnvVarsRaw 未正确解密：%q", vars["CF_Token"])
	}
}

// 升级前的历史数据是**裸 base64(JSON)**（无 enc: 前缀）→ 必须不用迁移就能继续读 ✓。
func TestDNSChannelDecodeLegacyPlainBase64(t *testing.T) {
	svc, repo := newChannelSvc(t)
	ctx := context.Background()

	legacy := map[string]string{"CF_Token": "legacy-token-abcdefgh", "CF_Account_ID": "acct-legacy"}
	raw, _ := json.Marshal(legacy)
	ch := &entity.DNSChannel{
		Name:     "legacy",
		Provider: "cloudflare",
		EnvVars:  base64.StdEncoding.EncodeToString(raw), // 旧存法 ✓
		Status:   "active",
	}
	if err := repo.Create(ctx, ch); err != nil {
		t.Fatal(err)
	}

	got, err := svc.GetEnvVars(ctx, ch.ID)
	if err != nil {
		t.Fatalf("旧格式数据必须保持可读（无需迁移）✗：%v", err)
	}
	if got["CF_Token"] != "legacy-token-abcdefgh" {
		t.Errorf("旧数据解出错误：%q", got["CF_Token"])
	}
}

// 前端编辑渠道时**不回填**凭据（列表只回掩码 ✓）→ 空提交不得清空已配好的凭据 ✗。
func TestDNSChannelUpdateKeepsCredentialsWhenEmpty(t *testing.T) {
	svc, repo := newChannelSvc(t)
	ctx := context.Background()

	ch, err := svc.Create(ctx, "cf", "cloudflare", map[string]string{
		"CF_Token":      "tok-original-123456",
		"CF_Account_ID": "acct-999",
	})
	if err != nil {
		t.Fatal(err)
	}

	// 只改名字（凭据框留空）→ 凭据必须原样保留
	if _, err := svc.Update(ctx, ch.ID, "cf-renamed", "cloudflare", map[string]string{}); err != nil {
		t.Fatal(err)
	}
	got, err := svc.GetEnvVars(ctx, ch.ID)
	if err != nil {
		t.Fatalf("改名后凭据被清空了（回归 ✗）：%v", err)
	}
	if got["CF_Token"] != "tok-original-123456" || got["CF_Account_ID"] != "acct-999" {
		t.Errorf("空提交应保留旧凭据，实际 %+v", got)
	}

	// 只轮换一个键 → 另一个键保留旧值 ✓
	if _, err := svc.Update(ctx, ch.ID, "cf-renamed", "cloudflare", map[string]string{"CF_Token": "tok-rotated-654321"}); err != nil {
		t.Fatal(err)
	}
	got2, err := svc.GetEnvVars(ctx, ch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got2["CF_Token"] != "tok-rotated-654321" {
		t.Errorf("新值未生效：%q", got2["CF_Token"])
	}
	if got2["CF_Account_ID"] != "acct-999" {
		t.Errorf("留空的键必须保留旧值，实际 %q", got2["CF_Account_ID"])
	}

	// Update 之后落库的也必须是加密格式 ✓
	row, err := repo.GetByID(ctx, ch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(row.EnvVars, "enc:") {
		t.Errorf("Update 后未加密落库：%q", preview(row.EnvVars))
	}
}
