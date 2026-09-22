package system

import (
	"os"
	"strings"
	"testing"
)

// .env 里可能有数据库密码（CF_DB_DSN）与 JWT_SECRET → 权限必须 0600；
// 且**已存在**的文件也要被收紧（os.WriteFile 对已存在文件不改权限 ✗）。
func TestSaveEnvFileUsesAndEnforces0600(t *testing.T) {
	dir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)

	if err := saveEnvFile("CF_DB_DSN", "mysql://u:p@tcp(127.0.0.1:3306)/x"); err != nil {
		t.Fatalf("saveEnvFile: %v", err)
	}
	info, err := os.Stat(".env")
	if err != nil {
		t.Fatalf(".env 未创建：%v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("新建 .env 权限应为 0600，实际 %v", info.Mode().Perm())
	}
	data, _ := os.ReadFile(".env")
	if !strings.Contains(string(data), "CF_DB_DSN=mysql://u:p@tcp(127.0.0.1:3306)/x") {
		t.Errorf("DSN 未写入：%q", data)
	}

	// 老部署：0644 → 再次保存必须收紧
	if err := os.Chmod(".env", 0644); err != nil {
		t.Fatal(err)
	}
	if err := saveEnvFile("CF_DB_DSN", "data/sqlite/app.db"); err != nil {
		t.Fatalf("saveEnvFile: %v", err)
	}
	if info, _ = os.Stat(".env"); info.Mode().Perm() != 0600 {
		t.Errorf("已存在的 .env 应收紧到 0600，实际 %v", info.Mode().Perm())
	}
	data, _ = os.ReadFile(".env")
	if strings.Count(string(data), "CF_DB_DSN=") != 1 {
		t.Errorf("同一 key 只应出现一次：%q", data)
	}
}
