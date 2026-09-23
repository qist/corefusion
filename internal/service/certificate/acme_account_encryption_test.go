package certificate

import (
	"context"
	"testing"

	"github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

const svcEABMAC = "S30IILacK_mOa6pdcByyIIRxKL_KbSZelD3UsxAeaXge1DExample"

// 从 service 视角盯住两条真实链路 ✓：
//  1. Create 返回的 cfg 会**立刻**被拿去签发 ✓ → MAC 必须是明文 ✗（否则 CA 只会回"EAB 无效" ✗）；
//  2. 编辑时前端拿不到密钥 ✓ → "留空 = 保持原值"之后仍必须是明文 ✓、has_eab 判断不能失效 ✓。
func TestACMEEmailServiceReturnsPlainMAC(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("内存 SQLite 不可用（跳过）：%v", err)
	}
	if err := db.AutoMigrate(&entity.ACMEAccountConfig{}); err != nil {
		t.Skipf("迁移失败（跳过）：%v", err)
	}
	svc := NewACMEEmailService(repository.NewACMEAccountConfigRepository(db))
	ctx := context.Background()

	cfg, err := svc.Create(ctx, ACMEAccountInput{
		Email: "test@example.com", CAProvider: "zerossl",
		EABKeyID: "koQayFUN4XpLF-8udnQjSQ", EABMACKey: svcEABMAC,
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ID == 0 {
		t.Error("Create 返回的 cfg.ID=0，前端拿不到 id")
	}
	if cfg.EABMACKey != svcEABMAC {
		t.Errorf("Create 返回的 MAC 不是明文：%q", cfg.EABMACKey)
	}

	// 编辑：不回填密钥
	if _, err := svc.Update(ctx, cfg.ID, ACMEAccountInput{
		Email: "changed@example.com", CAProvider: "zerossl", EABKeyID: "koQayFUN4XpLF-8udnQjSQ",
	}); err != nil {
		t.Fatal(err)
	}
	after, err := svc.GetByID(ctx, cfg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.EABMACKey != svcEABMAC {
		t.Errorf("留空更新后 MAC 丢失或成了密文：%q", after.EABMACKey)
	}
	if after.Email != "changed@example.com" {
		t.Errorf("普通字段没更新：%q", after.Email)
	}

	// has_eab 靠"明文非空"判断 ✓ 若 repository 不解密就永远为 true ✗
	list, err := svc.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || !list[0].HasEAB {
		t.Errorf("has_eab 判断失效：%+v", list)
	}
}
