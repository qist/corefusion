package certificate

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	sqlitedrv "github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

func setupEmailSvc(t *testing.T) *ACMEEmailService {
	t.Helper()
	db, err := gorm.Open(sqlitedrv.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&entity.ACMEAccountConfig{}); err != nil {
		t.Fatal(err)
	}
	return NewACMEEmailService(repository.NewACMEAccountConfigRepository(db))
}

// 核心：CA + EAB 必须**存得住** ✓（用户要求"做个配置，不然每次都填写" ✗）
func TestACMEEmailServiceStoresCAAndEAB(t *testing.T) {
	svc := setupEmailSvc(t)
	ctx := context.Background()

	created, err := svc.Create(ctx, ACMEAccountInput{
		Email:      "test@example.com",
		CAProvider: "google",
		EABKeyID:   "kid-123",
		EABMACKey:  "mac-abc",
		IsDefault:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.CAProvider != "google" || created.EABKeyID != "kid-123" || created.EABMACKey != "mac-abc" {
		t.Fatalf("CA/EAB 未保存：%+v", created)
	}

	// 重新读出来（模拟下次签发 ✓）
	got, err := svc.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.EABKeyID != "kid-123" || got.EABMACKey != "mac-abc" || got.CAProvider != "google" {
		t.Fatalf("读取后 CA/EAB 不一致：%+v", got)
	}
}

// 密钥**永不外泄** ✓：JSON 里不能出现 MAC，只能有 has_eab ✓
func TestACMEEmailServiceNeverLeaksMACKey(t *testing.T) {
	svc := setupEmailSvc(t)
	ctx := context.Background()
	if _, err := svc.Create(ctx, ACMEAccountInput{
		Email: "a@b.com", CAProvider: "zerossl", EABKeyID: "kid", EABMACKey: "SECRET-MAC",
	}); err != nil {
		t.Fatal(err)
	}

	list, err := svc.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(list)
	if strings.Contains(string(raw), "SECRET-MAC") {
		t.Fatalf("MAC Key 泄漏到接口响应里 ✗：%s", raw)
	}
	if !strings.Contains(string(raw), `"has_eab":true`) {
		t.Fatalf("应给前端 has_eab 标记 ✓：%s", raw)
	}
}

// 更新时 MAC 留空 = **保持原值** ✓（前端拿不到密钥，不该因为没重填就被清掉 ✗）
func TestACMEEmailServiceUpdateKeepsMACWhenEmpty(t *testing.T) {
	svc := setupEmailSvc(t)
	ctx := context.Background()
	c, err := svc.Create(ctx, ACMEAccountInput{
		Email: "a@b.com", CAProvider: "google", EABKeyID: "kid-1", EABMACKey: "mac-1",
	})
	if err != nil {
		t.Fatal(err)
	}

	// 改邮箱 + KID，MAC 不填 ✓
	upd, err := svc.Update(ctx, c.ID, ACMEAccountInput{
		Email: "a2@b.com", CAProvider: "google", EABKeyID: "kid-2", EABMACKey: "",
	})
	if err != nil {
		t.Fatal(err)
	}
	if upd.EABMACKey != "mac-1" {
		t.Fatalf("留空时应保留原 MAC ✓，实际 %q", upd.EABMACKey)
	}
	if upd.EABKeyID != "kid-2" || upd.Email != "a2@b.com" {
		t.Fatalf("其它字段应更新：%+v", upd)
	}

	// 明确给了新 MAC → 覆盖 ✓
	upd2, err := svc.Update(ctx, c.ID, ACMEAccountInput{
		Email: "a2@b.com", CAProvider: "google", EABKeyID: "kid-2", EABMACKey: "mac-2",
	})
	if err != nil {
		t.Fatal(err)
	}
	if upd2.EABMACKey != "mac-2" {
		t.Fatalf("显式给 MAC 应覆盖 ✓，实际 %q", upd2.EABMACKey)
	}
}

// CA 留空 → 默认 lets_encrypt ✓（保证库里不为空 ✓）
func TestACMEEmailServiceDefaultCA(t *testing.T) {
	svc := setupEmailSvc(t)
	c, err := svc.Create(context.Background(), ACMEAccountInput{Email: "x@y.com"})
	if err != nil {
		t.Fatal(err)
	}
	if c.CAProvider != "lets_encrypt" {
		t.Fatalf("CA 留空应默认 lets_encrypt ✓，实际 %q", c.CAProvider)
	}
}

// 默认账号切换仍然正常 ✓（回归 ✓）
func TestACMEEmailServiceDefaultSwitch(t *testing.T) {
	svc := setupEmailSvc(t)
	ctx := context.Background()
	a, _ := svc.Create(ctx, ACMEAccountInput{Email: "a@b.com", IsDefault: true})
	b, _ := svc.Create(ctx, ACMEAccountInput{Email: "b@b.com", IsDefault: true})
	_, _ = a, b
	def, err := svc.GetDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if def.Email != "b@b.com" {
		t.Fatalf("默认账号应是最后设置的那个 ✓，实际 %q", def.Email)
	}
}
