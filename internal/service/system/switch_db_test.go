package system

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"corefusion/internal/store/db"

	"github.com/qist/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openSQLiteForTest(t *testing.T, path string) *gorm.DB {
	t.Helper()
	g, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	return g
}

func chdirTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	return dir
}

func TestUpsertEnvContent(t *testing.T) {
	got := upsertEnvContent("A=1\n# c\nCF_DB_DSN=old\n", "CF_DB_DSN", "new")
	if !strings.Contains(got, "CF_DB_DSN=new") || strings.Contains(got, "old") {
		t.Errorf("应替换旧值：%q", got)
	}
	if !strings.Contains(got, "A=1") || !strings.Contains(got, "# c") {
		t.Errorf("其它行与注释必须保留：%q", got)
	}
	got2 := upsertEnvContent("A=1\n", "CF_DB_DSN", "new")
	if !strings.Contains(got2, "CF_DB_DSN=new") {
		t.Errorf("不存在时应追加：%q", got2)
	}
}

// 写 .env 必须"读回校验"：写不进去要报错，且**不能**把原内容弄丢（回滚）。
func TestSaveEnvFileVerified(t *testing.T) {
	chdirTemp(t)

	if err := os.WriteFile(".env", []byte("A=1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := saveEnvFileVerified("CF_DB_DSN", "mysql://u:p@tcp(1.2.3.4:3306)/x"); err != nil {
		t.Fatalf("正常写入应成功：%v", err)
	}
	data, _ := os.ReadFile(".env")
	if !strings.Contains(string(data), "CF_DB_DSN=mysql://u:p@tcp(1.2.3.4:3306)/x") || !strings.Contains(string(data), "A=1") {
		t.Errorf("应写入且保留其它内容：%q", data)
	}

	// 失败路径：把 .env 换成目录 → 写入必然失败 → 必须返回错误（而不是静默成功 ✗）
	dir := t.TempDir()
	chdirSecond := func() { _ = os.Chdir(dir) }
	chdirSecond()
	if err := os.Mkdir(".env", 0750); err != nil {
		t.Fatal(err)
	}
	if err := saveEnvFileVerified("CF_DB_DSN", "x"); err == nil {
		t.Error("写不进去时必须报错（不能谎报成功）")
	}
}

// 切换成功：数据真的迁移过去了，且 `.env` 才被更新（其它键保留）。
func TestSwitchDatabasePersistsEnvAfterMigration(t *testing.T) {
	dir := chdirTemp(t)

	if err := os.WriteFile(".env", []byte("JWT_SECRET=keepme\nCF_DB_DSN=data/sqlite/app.db\n"), 0600); err != nil {
		t.Fatal(err)
	}

	srcPath := filepath.Join(dir, "src.db")
	tgtPath := filepath.Join(dir, "tgt.db")

	// 目标库预建测试表（AutoMigrate 只认识实体表）
	pre := openSQLiteForTest(t, tgtPath)
	if err := pre.Exec("CREATE TABLE switch_rows (id INTEGER PRIMARY KEY, note TEXT)").Error; err != nil {
		t.Fatal(err)
	}
	if sqlDB, err := pre.DB(); err == nil {
		sqlDB.Close()
	}

	// 用 db.Init 把全局源库指到临时 sqlite
	if err := db.Init(db.Config{Type: db.DBTypeSQLite, DSN: srcPath}); err != nil {
		t.Fatalf("db.Init: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if db.DB == nil {
		t.Fatal("源库未初始化")
	}
	if err := db.DB.Exec("CREATE TABLE switch_rows (id INTEGER PRIMARY KEY, note TEXT)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.DB.Exec("INSERT INTO switch_rows (id, note) VALUES (1,'a'),(2,'b')").Error; err != nil {
		t.Fatal(err)
	}

	svc := &SettingService{}
	tables, rows, err := svc.SwitchDatabase(context.Background(), &DBConfig{Type: "sqlite", DSN: tgtPath})
	if err != nil {
		t.Fatalf("切换应成功：%v", err)
	}
	if tables < 1 || rows < 2 {
		t.Errorf("应至少迁移 1 表 2 行，实际 tables=%d rows=%d", tables, rows)
	}

	// 数据真的在新库里
	after := openSQLiteForTest(t, tgtPath)
	var cnt int64
	if err := after.Raw("SELECT COUNT(*) FROM switch_rows").Scan(&cnt).Error; err != nil {
		t.Fatalf("读回新库失败：%v", err)
	}
	if cnt != 2 {
		t.Errorf("新库应有 2 行，实际 %d", cnt)
	}

	// .env：DSN 已指向新库，且**其它键保留**（不能把 JWT_SECRET 弄丢 ✗）
	env := loadEnvFile()
	if env["CF_DB_DSN"] != tgtPath {
		t.Errorf(".env 的 CF_DB_DSN 应为 %q，实际 %q", tgtPath, env["CF_DB_DSN"])
	}
	if env["JWT_SECRET"] != "keepme" {
		t.Errorf("其它键必须保留：%v", env)
	}
}

// ⚠ 核心保证：迁移失败时 `.env` **一个字都不能改** ✗（否则重启指向空库/不可用库 ✗）。
func TestSwitchDatabaseKeepsEnvWhenMigrationFails(t *testing.T) {
	dir := chdirTemp(t)

	const oldEnv = "JWT_SECRET=keepme\nCF_DB_DSN=data/sqlite/app.db\n"
	if err := os.WriteFile(".env", []byte(oldEnv), 0600); err != nil {
		t.Fatal(err)
	}

	srcPath := filepath.Join(dir, "src.db")
	roPath := filepath.Join(dir, "ro.db")

	// 只读目标库（sqlite 强制，root 也绕不过）
	seed := openSQLiteForTest(t, roPath)
	if err := seed.Exec("CREATE TABLE placeholder (id INTEGER)").Error; err != nil {
		t.Fatal(err)
	}
	if sqlDB, err := seed.DB(); err == nil {
		sqlDB.Close()
	}
	if _, err := gorm.Open(sqlite.Open("file:"+roPath+"?mode=ro"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	}); err != nil {
		t.Skipf("驱动不支持只读 URI DSN，跳过：%v", err)
	}

	if err := db.Init(db.Config{Type: db.DBTypeSQLite, DSN: srcPath}); err != nil {
		t.Fatalf("db.Init: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	svc := &SettingService{}
	_, _, err := svc.SwitchDatabase(context.Background(), &DBConfig{
		Type: "sqlite",
		DSN:  "file:" + roPath + "?mode=ro",
	})
	if err == nil {
		t.Fatal("目标库不可写时切换必须失败")
	}

	// .env 必须原封不动
	got, rerr := os.ReadFile(".env")
	if rerr != nil {
		t.Fatalf("读 .env: %v", rerr)
	}
	if string(got) != oldEnv {
		t.Errorf(".env 在切换失败时不得被改动\n got=%q\nwant=%q", got, oldEnv)
	}
}

// 读回校验的真正价值：**能发现"写进去了但不是我们要的值"** ——
// 例如值里带换行会把 env 文件拆成两行（等于往 .env 注入了一个键 ✗）。
// 此时必须报错、并把 .env 回滚成原样 ✓。
func TestSaveEnvFileVerifiedRejectsInjectingValue(t *testing.T) {
	chdirTemp(t)
	const old = "A=1\nCF_DB_DSN=old\n"
	if err := os.WriteFile(".env", []byte(old), 0600); err != nil {
		t.Fatal(err)
	}

	err := saveEnvFileVerified("CF_DB_DSN", "new\nINJECTED=1")
	if err == nil {
		t.Fatal("带换行的值会破坏 env 格式（等于注入键），必须报错")
	}
	got, _ := os.ReadFile(".env")
	if string(got) != old {
		t.Errorf(".env 应回滚成原样\n got=%q\nwant=%q", got, old)
	}
}
