package db

import (
	"path/filepath"
	"testing"
)

// 要求："数据库连接也走 env" —— 证明 `CF_DB_DSN`（以及类型/连接池）确实驱动**真实连接**，
// 而不是只有一份没人用的配置解析 ✓。
func TestConfigFromEnvDrivesRealConnection(t *testing.T) {
	dir := t.TempDir()
	appDB := filepath.Join(dir, "app.db")
	defaultDB := filepath.Join(dir, "default.db")

	// ① 裸路径 → 识别为 sqlite，并且真能连上、真能写（= 连接确实来自 env）
	cfg := ConfigFromEnv(map[string]string{"CF_DB_DSN": appDB}, defaultDB)
	if cfg.Type != DBTypeSQLite {
		t.Fatalf("裸路径应识别为 sqlite，实际 %q", cfg.Type)
	}
	if cfg.DSN != appDB {
		t.Fatalf("DSN 应取 env 值：got=%q want=%q", cfg.DSN, appDB)
	}
	gdb, cleanup, err := OpenDBPublic(cfg)
	if err != nil {
		t.Fatalf("用 env 指定的 DSN 连接失败：%v", err)
	}
	defer cleanup()
	if err := gdb.Exec("CREATE TABLE env_probe (id INTEGER)").Error; err != nil {
		t.Errorf("应能在 env 指定的库里建表（证明用的就是它）：%v", err)
	}

	// ② 带 scheme 的类型识别（mysql:// 前缀要去掉，postgres:// 要保留给 pgx）
	cases := []struct{ dsn, wantType, wantDSN string }{
		{"mysql://u:p@tcp(127.0.0.1:3306)/db", DBTypeMySQL, "u:p@tcp(127.0.0.1:3306)/db"},
		{"postgres://u:p@127.0.0.1:5432/db?sslmode=disable", DBTypePostgres, "postgres://u:p@127.0.0.1:5432/db?sslmode=disable"},
		{"sqlite:///opt/app.db", DBTypeSQLite, "/opt/app.db"},
	}
	for _, c := range cases {
		got := ConfigFromEnv(map[string]string{"CF_DB_DSN": c.dsn}, defaultDB)
		if got.Type != c.wantType || got.DSN != c.wantDSN {
			t.Errorf("CF_DB_DSN=%q → type=%q dsn=%q，期望 type=%q dsn=%q",
				c.dsn, got.Type, got.DSN, c.wantType, c.wantDSN)
		}
	}

	// ③ 连接池参数也走 env（切换数据库时面板正是把这些写进 .env 的）
	pool := ConfigFromEnv(map[string]string{
		"CF_DB_DSN":            "mysql://u:p@tcp(127.0.0.1:3306)/db",
		"CF_DB_MAX_OPEN_CONNS": "33",
		"CF_DB_MAX_IDLE_CONNS": "7",
	}, defaultDB)
	if pool.MaxOpenConns != 33 || pool.MaxIdleConns != 7 {
		t.Errorf("连接池参数应来自 env：open=%d idle=%d", pool.MaxOpenConns, pool.MaxIdleConns)
	}

	// ④ 没设 CF_DB_DSN → 回退默认 sqlite 路径（与老部署兼容）
	fallback := ConfigFromEnv(map[string]string{}, defaultDB)
	if fallback.Type != DBTypeSQLite || fallback.DSN != defaultDB {
		t.Errorf("未设置时应回退默认 sqlite：type=%q dsn=%q", fallback.Type, fallback.DSN)
	}

	// ⑤ CF_DB_TYPE 可以覆盖自动识别
	override := ConfigFromEnv(map[string]string{
		"CF_DB_DSN":  "u:p@tcp(127.0.0.1:3306)/db",
		"CF_DB_TYPE": DBTypeMySQL,
	}, defaultDB)
	if override.Type != DBTypeMySQL {
		t.Errorf("CF_DB_TYPE 应覆盖识别结果，实际 %q", override.Type)
	}
}
