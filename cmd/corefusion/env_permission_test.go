package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 审计 v5 P-4：`.env` 里可能含 JWT_SECRET / CF_SECRET_ENCRYPTION_KEY，写它时权限必须是 0600；
// 且**已存在的**文件也要被改成 0600（os.WriteFile 对已存在文件不改权限 ✗）。
func TestSaveEnvValueUses0600AndFixesExistingFile(t *testing.T) {
	dir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)

	// ① 新建：0600 + 内容正确
	saveEnvValue("JWT_SECRET", "secret-1")
	info, err := os.Stat(".env")
	if err != nil {
		t.Fatalf(".env 未创建：%v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf(".env 权限应为 0600，实际 %v", info.Mode().Perm())
	}
	data, _ := os.ReadFile(".env")
	if !strings.Contains(string(data), "JWT_SECRET=secret-1") {
		t.Errorf(".env 内容不对：%q", data)
	}

	// ② 已存在且权限是 0644（老部署的情况）→ 写入后必须被收紧到 0600
	if err := os.Chmod(".env", 0644); err != nil {
		t.Fatal(err)
	}
	saveEnvValue("JWT_SECRET", "secret-2")
	info, err = os.Stat(".env")
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("已存在的 .env 也应收紧到 0600，实际 %v", info.Mode().Perm())
	}
	data, _ = os.ReadFile(".env")
	if !strings.Contains(string(data), "JWT_SECRET=secret-2") {
		t.Errorf("值应被更新：%q", data)
	}
	// 不应把同一个 key 写两遍
	if strings.Count(string(data), "JWT_SECRET=") != 1 {
		t.Errorf("同一 key 只应出现一次：%q", data)
	}
	_ = filepath.Join(dir, ".env")
}
