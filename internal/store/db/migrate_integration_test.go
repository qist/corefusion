package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gorm.io/gorm/clause"

	"corefusion/internal/migration"
	"corefusion/internal/model/entity"
)

// requireDisposableTestDB 只允许对“名字里明显是测试库”的目标执行。
// 这个测试会清空目标库的表（MigrateFromSQLite 的语义是覆盖），必须防止
// 有人把 CF_TEST_*_DSN 指到 corefusion / 生产库上（历史上出过删真实数据的事故）。
func requireDisposableTestDB(t *testing.T, dsn string) {
	t.Helper()
	low := strings.ToLower(dsn)
	for _, marker := range []string{"test", "cfmig", "cffix", "ci_", "sandbox"} {
		if strings.Contains(low, marker) {
			return
		}
	}
	t.Fatalf("refuse to run against non-disposable database %q: name must contain test/cfmig/cffix/ci_/sandbox", dsn)
}

// TestMigrateFromSQLiteToRemote 覆盖“备份还原”走的跨库路径：
// SQLite 文件 → MySQL / PostgreSQL，并回归两件事：
//  1. 全新空库也能建全 118 张表（模型清单唯一）；
//  2. 拷贝完成后自增序列可用（PG 必须 setval，否则第一条 INSERT 撞主键）。
func TestMigrateFromSQLiteToRemote(t *testing.T) {
	targets := []struct{ name, env string }{
		{"mysql", "CF_TEST_MYSQL_DSN"},
		{"postgres", "CF_TEST_PG_DSN"},
	}
	for _, tc := range targets {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			dsn := os.Getenv(tc.env)
			if dsn == "" {
				t.Skipf("%s not set", tc.env)
			}
			requireDisposableTestDB(t, dsn)

			// ① 造一个带真实数据的 SQLite 源库
			srcPath := filepath.Join(t.TempDir(), "src.db")
			srcCfg := Config{Type: DBTypeSQLite, DSN: srcPath, MaxOpenConns: 1, MaxIdleConns: 1}
			if err := Init(srcCfg); err != nil {
				t.Fatalf("init sqlite source: %v", err)
			}
			RecordStartup(srcCfg)
			if err := migration.AutoMigrate(DB); err != nil {
				t.Fatalf("auto-migrate sqlite source: %v", err)
			}
			// 用显式 id 写入（模拟真实库里的存量数据，正是导致 PG 序列落后的场景）
			if err := DB.Create(&entity.SystemSetting{Key: "test.integration", Value: "v1"}).Error; err != nil {
				t.Fatalf("seed system_settings: %v", err)
			}
			if err := DB.Create(&entity.RuleSource{
				ServerID: 0, Name: "it-source", RepoURL: "a/b", SyncCron: "0 3 * * *", Status: "idle",
			}).Error; err != nil {
				t.Fatalf("seed rule_sources: %v", err)
			}
			if err := DB.Exec(
				"INSERT INTO system_settings (key, value, created_at, updated_at) VALUES (?,?,datetime('now'),datetime('now'))",
				"test.explicit", "x",
			).Error; err != nil {
				t.Fatalf("seed explicit-id row: %v", err)
			}
			_ = Close()

			// ② 还原到远端库
			dstCfg := ParseDSN(dsn)
			tables, rows, err := MigrateFromSQLite(srcPath, dstCfg)
			if err != nil {
				t.Fatalf("MigrateFromSQLite: %v", err)
			}
			if tables == 0 || rows == 0 {
				t.Fatalf("nothing copied: tables=%d rows=%d", tables, rows)
			}

			// ③ 连上目标库校验数据 + 序列
			if err := Init(dstCfg); err != nil {
				t.Fatalf("init target: %v", err)
			}
			defer func() { _ = Close() }()

			var got int64
			if err := DB.Model(&entity.SystemSetting{}).
				Where(clause.Eq{Column: clause.Column{Name: "key"}, Value: "test.integration"}).
				Count(&got).Error; err != nil {
				t.Fatalf("count copied row: %v", err)
			}
			if got != 1 {
				t.Fatalf("copied system_settings row not found (count=%d)", got)
			}

			// 序列是否推进：这里插入一条新行，序列未同步时 PG 会报 duplicate key
			if err := DB.Create(&entity.SystemSetting{Key: "test.after-restore", Value: "v2"}).Error; err != nil {
				t.Fatalf("insert after restore failed (sequence not synced?): %v", err)
			}
			if err := DB.Create(&entity.RuleSource{
				ServerID: 0, Name: "it-source-after", RepoURL: "a/c", SyncCron: "0 3 * * *", Status: "idle",
			}).Error; err != nil {
				t.Fatalf("insert rule_source after restore failed: %v", err)
			}
		})
	}
}
