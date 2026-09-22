package db

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/qist/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// 要求："数据库迁移要保证能写进去" —— 旧实现里**所有写入失败只打 warning 就继续** ✗
// （清表失败 / 批量插入失败 / 逐行插入失败），迁移会"成功返回"却丢数据 ✗（静默数据丢失）。
// 现在：写前预检 + 写入失败致命 + 拷后行数核对。

func openTestSQLite(t *testing.T, path string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	return db
}

func tableExists(t *testing.T, db *gorm.DB, name string) bool {
	t.Helper()
	var n int64
	if err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", name).Scan(&n).Error; err != nil {
		t.Fatalf("check table %s: %v", name, err)
	}
	return n > 0
}

// 预检：可写库通过（且不残留探针表）；只读库必须被判为不可写。
func TestVerifyTargetWritable(t *testing.T) {
	dir := t.TempDir()

	// ① 可写
	db := openTestSQLite(t, filepath.Join(dir, "ok.db"))
	if err := verifyTargetWritable(db); err != nil {
		t.Errorf("可写库应通过预检：%v", err)
	}
	if tableExists(t, db, migrateProbeTable) {
		t.Error("探针表必须被清理掉（不能留在业务库里）")
	}

	// ② 只读（file:...?mode=ro 由 sqlite 强制，root 也绕不过）
	roPath := filepath.Join(dir, "ro.db")
	seed := openTestSQLite(t, roPath)
	if err := seed.Exec("CREATE TABLE t (id INTEGER)").Error; err != nil {
		t.Fatal(err)
	}
	if sqlDB, err := seed.DB(); err == nil {
		sqlDB.Close()
	}
	roDB, err := gorm.Open(sqlite.Open("file:"+roPath+"?mode=ro"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Skipf("驱动不支持只读 URI DSN，跳过：%v", err)
	}
	if err := verifyTargetWritable(roDB); err == nil {
		t.Error("只读目标库必须预检失败（否则会静默丢数据）")
	}
}

// 迁移成功路径：数据真的写进去了（行数核对 + 目标可读回）+ 探针表不残留。
func TestMigrateToVerifiesWrites(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "src.db")
	tgtPath := filepath.Join(dir, "tgt.db")

	// 源：一张测试表 + 两行数据（表名不用 _cf_ 前缀，避免被跳过）
	src := openTestSQLite(t, srcPath)
	if err := src.Exec("CREATE TABLE migrate_probe_rows (id INTEGER PRIMARY KEY, note TEXT)").Error; err != nil {
		t.Fatal(err)
	}
	if err := src.Exec("INSERT INTO migrate_probe_rows (id, note) VALUES (1,'a'),(2,'b')").Error; err != nil {
		t.Fatal(err)
	}

	// 目标：预建同名表（AutoMigrate 只认识实体表，测试表要自己建；否则拷贝会因缺表而失败）
	tgt := openTestSQLite(t, tgtPath)
	if err := tgt.Exec("CREATE TABLE migrate_probe_rows (id INTEGER PRIMARY KEY, note TEXT)").Error; err != nil {
		t.Fatal(err)
	}
	if sqlDB, err := tgt.DB(); err == nil {
		sqlDB.Close()
	}

	// MigrateTo 用全局 DB 作源 + 全局 startType 决定方言
	prevDB, prevType := DB, startType
	DB, startType = src, DBTypeSQLite
	defer func() { DB, startType = prevDB, prevType }()

	tables, rows, err := MigrateTo(Config{Type: DBTypeSQLite, DSN: tgtPath})
	if err != nil {
		t.Fatalf("迁移应成功：%v", err)
	}
	if tables < 1 || rows < 2 {
		t.Errorf("应至少拷 1 张表 2 行，实际 tables=%d rows=%d", tables, rows)
	}

	// 目标里数据确实在（这是"能写进去"的端到端判据）
	after := openTestSQLite(t, tgtPath)
	var cnt int64
	if err := after.Raw("SELECT COUNT(*) FROM migrate_probe_rows").Scan(&cnt).Error; err != nil {
		t.Fatalf("读回目标失败：%v", err)
	}
	if cnt != 2 {
		t.Errorf("目标表应有 2 行，实际 %d", cnt)
	}
	if tableExists(t, after, migrateProbeTable) {
		t.Error("探针表不应残留在目标库里")
	}
}

// 目标不可写时，整体迁移必须**失败返回**（旧实现会把它当 warning 吞掉 ✗）。
func TestMigrateToFailsWhenTargetNotWritable(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "src.db")
	roPath := filepath.Join(dir, "ro.db")

	src := openTestSQLite(t, srcPath)
	if err := src.Exec("CREATE TABLE migrate_probe_rows (id INTEGER PRIMARY KEY)").Error; err != nil {
		t.Fatal(err)
	}
	if err := src.Exec("INSERT INTO migrate_probe_rows (id) VALUES (1)").Error; err != nil {
		t.Fatal(err)
	}

	seed := openTestSQLite(t, roPath)
	if err := seed.Exec("CREATE TABLE placeholder (id INTEGER)").Error; err != nil {
		t.Fatal(err)
	}
	if sqlDB, err := seed.DB(); err == nil {
		sqlDB.Close()
	}
	// 用只读 URI 打开：AutoMigrate/预检都会失败
	if _, err := gorm.Open(sqlite.Open("file:"+roPath+"?mode=ro"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	}); err != nil {
		t.Skipf("驱动不支持只读 URI DSN，跳过：%v", err)
	}

	prevDB, prevType := DB, startType
	DB, startType = src, DBTypeSQLite
	defer func() { DB, startType = prevDB, prevType }()

	_, _, err := MigrateTo(Config{Type: DBTypeSQLite, DSN: "file:" + roPath + "?mode=ro"})
	if err == nil {
		t.Fatal("目标库不可写时迁移必须失败（旧实现只打 warning 就'成功' ✗）")
	}
	msg := err.Error()
	if !strings.Contains(msg, "writable") && !strings.Contains(msg, "auto-migrate") &&
		!strings.Contains(msg, "readonly") && !strings.Contains(msg, "read-only") &&
		!strings.Contains(msg, "attempt to write") {
		t.Errorf("错误信息应指明写不进去，实际：%v", err)
	}
}

// 写入过程中失败（例：目标表有约束让某一行写不进去）必须**整体失败**，
// 而不是像旧实现那样只记 warning 继续、最后报"成功" ✗（静默丢数据）。
func TestMigrateToFailsOnRowInsertError(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "src.db")
	tgtPath := filepath.Join(dir, "tgt.db")

	src := openTestSQLite(t, srcPath)
	if err := src.Exec("CREATE TABLE migrate_probe_rows (id INTEGER PRIMARY KEY, note TEXT)").Error; err != nil {
		t.Fatal(err)
	}
	if err := src.Exec("INSERT INTO migrate_probe_rows (id, note) VALUES (1,'ok'),(2,'boom')").Error; err != nil {
		t.Fatal(err)
	}

	// 目标表带 CHECK：note='boom' 的行写不进去
	tgt := openTestSQLite(t, tgtPath)
	if err := tgt.Exec("CREATE TABLE migrate_probe_rows (id INTEGER PRIMARY KEY, note TEXT CHECK (note <> 'boom'))").Error; err != nil {
		t.Fatal(err)
	}
	if sqlDB, err := tgt.DB(); err == nil {
		sqlDB.Close()
	}

	prevDB, prevType := DB, startType
	DB, startType = src, DBTypeSQLite
	defer func() { DB, startType = prevDB, prevType }()

	_, _, err := MigrateTo(Config{Type: DBTypeSQLite, DSN: tgtPath})
	if err == nil {
		t.Fatal("有行写不进去时迁移必须失败（旧实现只 warning → 谎报成功 ✗）")
	}
	if !strings.Contains(err.Error(), "insert into migrate_probe_rows failed") {
		t.Errorf("错误应指明是哪张表写失败：%v", err)
	}
}
