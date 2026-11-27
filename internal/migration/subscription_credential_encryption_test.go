package migration

import (
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	"corefusion/internal/security/crypto"
)

func rawCredential(t *testing.T, db *gorm.DB, table, column string, id int64) string {
	t.Helper()
	var val string
	if err := db.Table(table).Select(column).Where("id = ?", id).Row().Scan(&val); err != nil {
		t.Fatalf("读取 %s.%s 失败：%v", table, column, err)
	}
	return val
}

// 存量明文的订阅 token / 轮换历史 token 必须在启动迁移后变成 enc: 密文，
// 且幂等（重复执行不二次加密）。
func TestMigrateSubscriptionCredentialEncryption(t *testing.T) {
	db := newMigrationTestDB(t,
		&entity.Subscription{},
		&entity.SubscriptionRotation{},
	)
	now := time.Now()

	if err := db.Exec(
		`INSERT INTO subscriptions (client_id, format, token, token_version, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		1, "base64_uri", "plain-sub-token", 1, "active", now, now,
	).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(
		`INSERT INTO subscription_rotations (subscription_id, old_token, new_token, rotated_by, reason, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		1, "plain-old-token", "plain-new-token", 1, "", now,
	).Error; err != nil {
		t.Fatal(err)
	}

	migrateSubscriptionCredentialEncryption(db)

	checks := []struct {
		table string
		col   string
		want  string
	}{
		{"subscriptions", "token", "plain-sub-token"},
		{"subscription_rotations", "old_token", "plain-old-token"},
		{"subscription_rotations", "new_token", "plain-new-token"},
	}
	stored := map[string]string{}
	for _, c := range checks {
		var id int64
		if err := db.Table(c.table).Select("id").Order("id ASC").Limit(1).Row().Scan(&id); err != nil {
			t.Fatal(err)
		}
		val := rawCredential(t, db, c.table, c.col, id)
		if !strings.HasPrefix(val, "enc:") {
			t.Fatalf("%s.%s 未加密：%q", c.table, c.col, val)
		}
		plain, err := crypto.DecryptSecret(val)
		if err != nil {
			t.Fatalf("%s.%s 解密失败：%v", c.table, c.col, err)
		}
		if plain != c.want {
			t.Fatalf("%s.%s 解密结果 = %q, want %q", c.table, c.col, plain, c.want)
		}
		stored[c.table+"."+c.col] = val
	}

	// 幂等：再跑一次，密文不得变化。
	migrateSubscriptionCredentialEncryption(db)
	for _, c := range checks {
		var id int64
		if err := db.Table(c.table).Select("id").Order("id ASC").Limit(1).Row().Scan(&id); err != nil {
			t.Fatal(err)
		}
		if got := rawCredential(t, db, c.table, c.col, id); got != stored[c.table+"."+c.col] {
			t.Fatalf("%s.%s 被二次加密：%q -> %q", c.table, c.col, stored[c.table+"."+c.col], got)
		}
	}
}

// 短链 code：存量明文/旧确定性密文都要变成「code_hmac 指纹 + 随机密文」，且幂等。
func TestMigrateShortLinkCodeFingerprint(t *testing.T) {
	db := newMigrationTestDB(t, &entity.SubscriptionShortLink{})
	now := time.Now()
	legacyEnc, err := crypto.EncryptSecretDeterministic("legacy-short-code")
	if err != nil {
		t.Fatal(err)
	}

	insert := func(code string) {
		t.Helper()
		if err := db.Exec(
			`INSERT INTO subscription_short_links
			 (subscription_id, code, format, enabled, providers, lock_params, token_version, follow_token_rotation, access_count, created_by, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			1, code, "base64_uri", true, false, false, 1, false, 0, 0, now, now,
		).Error; err != nil {
			t.Fatal(err)
		}
	}
	insert("plain-short-code")
	insert(legacyEnc)

	migrateShortLinkCodeFingerprint(db)

	type row struct {
		ID       int64
		Code     string
		CodeHMAC string
	}
	var rows []row
	if err := db.Table("subscription_short_links").Select("id, code, code_hmac").Order("id ASC").Scan(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	first := map[int64]string{}
	for _, r := range rows {
		if !strings.HasPrefix(r.Code, "enc:") {
			t.Fatalf("code#%d 未加密：%q", r.ID, r.Code)
		}
		if len(r.CodeHMAC) != 64 {
			t.Fatalf("code#%d 指纹长度 = %d", r.ID, len(r.CodeHMAC))
		}
		plain, err := crypto.DecryptSecret(r.Code)
		if err != nil {
			t.Fatal(err)
		}
		fp, err := crypto.FingerprintSecret(plain)
		if err != nil {
			t.Fatal(err)
		}
		if fp != r.CodeHMAC {
			t.Fatalf("code#%d 指纹与明文不匹配", r.ID)
		}
		first[r.ID] = r.Code
	}

	// 幂等：code_hmac 已存在 → 不再重加密。
	migrateShortLinkCodeFingerprint(db)
	if err := db.Table("subscription_short_links").Select("id, code").Order("id ASC").Scan(&rows).Error; err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.Code != first[r.ID] {
			t.Fatalf("code#%d 被二次加密：%q -> %q", r.ID, first[r.ID], r.Code)
		}
	}
}

// 外部订阅源 URL：存量明文 → 随机 AES-256-GCM，按 ID 读取不变；幂等。
func TestMigrateExternalNodeSourceURLEncryption(t *testing.T) {
	db := newMigrationTestDB(t, &entity.ExternalNodeSource{})
	now := time.Now()
	const srcURL = "https://upstream.example.com/sub/0123456789abcdef"
	if err := db.Exec(
		`INSERT INTO external_node_sources (name, source_type, source_url, enabled, sort_order, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"upstream", "mihomo", srcURL, true, 0, now, now,
	).Error; err != nil {
		t.Fatal(err)
	}

	migrateExternalNodeSourceURLEncryption(db)

	var id int64
	if err := db.Table("external_node_sources").Select("id").Order("id ASC").Limit(1).Row().Scan(&id); err != nil {
		t.Fatal(err)
	}
	stored := rawCredential(t, db, "external_node_sources", "source_url", id)
	if !strings.HasPrefix(stored, "enc:") || strings.Contains(stored, "0123456789abcdef") {
		t.Fatalf("source_url 未加密：%q", stored)
	}
	plain, err := crypto.DecryptSecret(stored)
	if err != nil {
		t.Fatal(err)
	}
	if plain != srcURL {
		t.Fatalf("解密结果 = %q, want %q", plain, srcURL)
	}

	migrateExternalNodeSourceURLEncryption(db)
	if again := rawCredential(t, db, "external_node_sources", "source_url", id); again != stored {
		t.Fatalf("source_url 被二次加密：%q -> %q", stored, again)
	}
}
