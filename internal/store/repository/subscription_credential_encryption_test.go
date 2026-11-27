package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

func newSubscriptionCredentialTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("内存 SQLite 不可用（跳过）：%v", err)
	}
	if err := db.AutoMigrate(
		&entity.Subscription{},
		&entity.SubscriptionShortLink{},
		&entity.SubscriptionRotation{},
		&entity.ExternalNodeSource{},
	); err != nil {
		t.Skipf("迁移失败（跳过）：%v", err)
	}
	return db
}

func rawColumn(t *testing.T, db *gorm.DB, table, column string, id int64) string {
	t.Helper()
	var val string
	if err := db.Table(table).Select(column).Where("id = ?", id).Row().Scan(&val); err != nil {
		t.Fatalf("读取 %s.%s 失败：%v", table, column, err)
	}
	return val
}

// 订阅 token 必须密文落库，但仓储返回给上层的仍是明文；存量明文按 token 查询时
// 要能命中并顺手加密回写。
func TestSubscriptionTokenEncryptedAtRest(t *testing.T) {
	db := newSubscriptionCredentialTestDB(t)
	repo := NewSubscriptionRepository(db)
	ctx := context.Background()

	const token = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	sub := &entity.Subscription{
		ClientID: 1, Format: "base64_uri", Token: token,
		TokenVersion: 1, Status: "active",
	}
	if err := repo.Create(ctx, sub); err != nil {
		t.Fatal(err)
	}
	if sub.Token != token {
		t.Fatalf("Create 后内存 token 被改写：%q", sub.Token)
	}
	stored := rawColumn(t, db, "subscriptions", "token", sub.ID)
	if !strings.HasPrefix(stored, "enc:") || strings.Contains(stored, token) {
		t.Fatalf("token 未加密落库：%q", stored)
	}

	got, err := repo.GetByToken(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	if got.Token != token {
		t.Fatalf("GetByToken 未解密：%q", got.Token)
	}

	// 存量明文行：直接绕过 hook 写入，再按明文 token 查询应命中并加密回写。
	const legacyToken = "legacy-plaintext-token"
	now := time.Now()
	if err := db.Table("subscriptions").Create(map[string]interface{}{
		"client_id": 2, "format": "base64_uri", "token": legacyToken,
		"token_version": 1, "status": "active",
		"created_at": now, "updated_at": now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	var legacyID int64
	if err := db.Table("subscriptions").Select("id").Where("token = ?", legacyToken).Row().Scan(&legacyID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetByToken(ctx, legacyToken); err != nil {
		t.Fatal(err)
	}
	if upgraded := rawColumn(t, db, "subscriptions", "token", legacyID); !strings.HasPrefix(upgraded, "enc:") {
		t.Fatalf("存量明文 token 未被加密回写：%q", upgraded)
	}
}

// 短链 code 同样必须密文落库，且 GetByCode / CodeExists 仍按明文工作。
func TestShortLinkCodeEncryptedAtRest(t *testing.T) {
	db := newSubscriptionCredentialTestDB(t)
	repo := NewSubscriptionShortLinkRepository(db)
	ctx := context.Background()

	const code = "abcdefghjk0123456789"
	link := &entity.SubscriptionShortLink{
		SubscriptionID: 1, Code: code, Format: "base64_uri", Enabled: true,
	}
	if err := repo.Create(ctx, link); err != nil {
		t.Fatal(err)
	}
	if link.Code != code {
		t.Fatalf("Create 后内存 code 被改写：%q", link.Code)
	}
	stored := rawColumn(t, db, "subscription_short_links", "code", link.ID)
	if !strings.HasPrefix(stored, "enc:") || strings.Contains(stored, code) {
		t.Fatalf("code 未加密落库：%q", stored)
	}
	hmac := rawColumn(t, db, "subscription_short_links", "code_hmac", link.ID)
	if len(hmac) != 64 || strings.Contains(hmac, code) {
		t.Fatalf("code_hmac 不是 64 位指纹：%q", hmac)
	}

	got, err := repo.GetByCode(ctx, code)
	if err != nil {
		t.Fatal(err)
	}
	if got.Code != code {
		t.Fatalf("GetByCode 未解密：%q", got.Code)
	}
	exists, err := repo.CodeExists(ctx, code)
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("CodeExists 未命中短码指纹")
	}

	// Code 用随机 nonce：同一个 code 再保存一次，密文会变，但指纹不变。
	if err := repo.Update(ctx, got); err != nil {
		t.Fatal(err)
	}
	if again := rawColumn(t, db, "subscription_short_links", "code", link.ID); again == stored {
		t.Fatal("Code 密文未使用随机 nonce（保存前后相同）")
	}
	if again := rawColumn(t, db, "subscription_short_links", "code_hmac", link.ID); again != hmac {
		t.Fatalf("同一个 code 的指纹变化：%q -> %q", hmac, again)
	}
}

// 轮换历史里的旧/新 token 也不能明文落库，读出来要是明文。
func TestSubscriptionRotationTokensEncryptedAtRest(t *testing.T) {
	db := newSubscriptionCredentialTestDB(t)
	repo := NewRotationRepository(db)
	ctx := context.Background()

	rot := &entity.SubscriptionRotation{
		SubscriptionID: 1, OldToken: "old-plain-token", NewToken: "new-plain-token", RotatedBy: 1,
	}
	if err := repo.Create(ctx, rot); err != nil {
		t.Fatal(err)
	}
	for _, col := range []string{"old_token", "new_token"} {
		if stored := rawColumn(t, db, "subscription_rotations", col, rot.ID); !strings.HasPrefix(stored, "enc:") {
			t.Fatalf("%s 未加密落库：%q", col, stored)
		}
	}
	list, _, err := repo.ListBySubscriptionID(ctx, 1, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].OldToken != "old-plain-token" || list[0].NewToken != "new-plain-token" {
		t.Fatalf("轮换历史未解密：%+v", list)
	}
}

// 外部订阅源 URL 可能带上游 token：必须随机 AES-256-GCM 落库，按 ID 读取时回明文。
func TestExternalNodeSourceURLEncryptedAtRest(t *testing.T) {
	db := newSubscriptionCredentialTestDB(t)
	repo := NewExternalNodeSourceRepository(db)
	ctx := context.Background()

	const srcURL = "https://upstream.example.com/sub/0123456789abcdef0123456789abcdef"
	src := &entity.ExternalNodeSource{
		Name: "upstream", SourceType: "mihomo", SourceURL: srcURL, Enabled: true,
	}
	if err := repo.Create(ctx, src); err != nil {
		t.Fatal(err)
	}
	if src.SourceURL != srcURL {
		t.Fatalf("Create 后内存 SourceURL 被改写：%q", src.SourceURL)
	}
	stored := rawColumn(t, db, "external_node_sources", "source_url", src.ID)
	if !strings.HasPrefix(stored, "enc:") || strings.Contains(stored, "0123456789abcdef") {
		t.Fatalf("source_url 未加密落库：%q", stored)
	}

	got, err := repo.GetByID(ctx, src.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.SourceURL != srcURL {
		t.Fatalf("GetByID 未解密：%q", got.SourceURL)
	}
	list, err := repo.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].SourceURL != srcURL {
		t.Fatalf("List 未解密：%+v", list)
	}
}
