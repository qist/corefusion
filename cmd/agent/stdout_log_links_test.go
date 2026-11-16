package main

import (
	"os"
	"path/filepath"
	"testing"
)

// 标准输出模式已下线：Agent 不再创建 stdout_*.log 软链，并清理历史遗留的软链；
// 但同名普通文件（真实日志）必须保留。
func TestCleanupLegacyStdoutLogLinks(t *testing.T) {
	dir := t.TempDir()

	// 遗留软链：应删除
	linkPath := filepath.Join(dir, "stdout_access.log")
	if err := os.Symlink("/dev/stdout", linkPath); err != nil {
		t.Fatal(err)
	}
	// 同名普通文件：应保留
	realPath := filepath.Join(dir, "stdout_error.log")
	if err := os.WriteFile(realPath, []byte("real log"), 0o640); err != nil {
		t.Fatal(err)
	}

	cleanupLegacyStdoutLogLinks(dir)

	if _, err := os.Lstat(linkPath); !os.IsNotExist(err) {
		t.Fatalf("stdout_access.log symlink should be removed, err=%v", err)
	}
	if data, err := os.ReadFile(realPath); err != nil || string(data) != "real log" {
		t.Fatalf("regular log file should be kept, data=%q err=%v", data, err)
	}
}

// 目录不存在时不应 panic / 报错。
func TestCleanupLegacyStdoutLogLinksMissingDir(t *testing.T) {
	cleanupLegacyStdoutLogLinks(filepath.Join(t.TempDir(), "nope"))
}
