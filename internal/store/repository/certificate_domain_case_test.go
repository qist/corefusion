package repository

import (
	"context"
	"testing"

	"github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

func newCertTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("内存 SQLite 不可用（跳过）：%v", err)
	}
	if err := db.AutoMigrate(&entity.Certificate{}); err != nil {
		t.Skipf("迁移失败（跳过）：%v", err)
	}
	return db
}

// 重新签发要覆盖旧证书：域名大小写不同也必须是同一条记录，
// 否则会多出一条重复证书（真机语义：DNS 名称大小写不敏感）。
func TestGetByDomainAndKeyTypeIgnoresDomainCase(t *testing.T) {
	db := newCertTestDB(t)
	repo := NewCertificateRepository(db)
	ctx := context.Background()

	row := &entity.Certificate{
		Name: "Example.com", Domain: "Example.com", Issuer: "google",
		ProviderType: "acme", KeyType: "ecc", KeyBits: 384,
		CertPath: "certs/Example.com/cert.pem", KeyPath: "certs/Example.com/key.pem",
		Status: "active",
	}
	if err := repo.Create(ctx, row); err != nil {
		t.Fatal(err)
	}

	got, err := repo.GetByDomainAndKeyType(ctx, "example.com", "ecc")
	if err != nil {
		t.Fatalf("大小写不同应命中同一条记录，得到错误：%v", err)
	}
	if got.ID != row.ID {
		t.Fatalf("命中了别的记录：id=%d，应为 %d", got.ID, row.ID)
	}

	rows, err := repo.ListByDomain(ctx, "EXAMPLE.COM")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ID != row.ID {
		t.Fatalf("ListByDomain 大小写不敏感失败：%+v", rows)
	}

	// 不同密钥类型仍是两条（ECC + RSA 双证书）
	rsa := &entity.Certificate{
		Name: "example.com", Domain: "example.com", Issuer: "google",
		ProviderType: "acme", KeyType: "rsa", KeyBits: 2048,
		CertPath: "certs/example.com/rsa/cert.pem", KeyPath: "certs/example.com/rsa/key.pem",
		Status: "active",
	}
	if err := repo.Create(ctx, rsa); err != nil {
		t.Fatal(err)
	}
	if got, err := repo.GetByDomainAndKeyType(ctx, "example.com", "rsa"); err != nil || got.ID != rsa.ID {
		t.Fatalf("RSA 应命中自己的记录：got=%v err=%v", got, err)
	}
	if rows, _ := repo.ListByDomain(ctx, "example.com"); len(rows) != 2 {
		t.Fatalf("同域 ECC+RSA 应有两条，实际 %d", len(rows))
	}
}
