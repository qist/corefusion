package backup

import (
	"context"
	"strings"
	"testing"

	"corefusion/internal/util/safepath"
)

// TestCreateBackupRejectsPathTraversalName 锁定安全审计 v6 M-1：
// 备份名必须是纯名字，不能借 `..` 把 tar.gz 写到任意目录。
func TestCreateBackupRejectsPathTraversalName(t *testing.T) {
	s := &Service{}
	_, err := s.CreateBackup(context.Background(), "../../../root/.ssh/authorized_keys", "manual")
	if err == nil {
		t.Fatal("path traversal backup name must be rejected")
	}
	if !strings.Contains(err.Error(), "backup name") {
		t.Fatalf("error should mention backup name, got %v", err)
	}
}

// 正常名字（含中文/下划线/点）不能被误伤（只测名字校验，不进入 DB 流程）。
func TestBackupNameValidationAllowsPlainName(t *testing.T) {
	if err := safepath.CheckName("backup name", "daily_备份-2026.11.03"); err != nil {
		t.Fatalf("plain name must not be rejected: %v", err)
	}
}
