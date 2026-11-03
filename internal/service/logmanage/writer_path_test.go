package logmanage

import (
	"path/filepath"
	"testing"
)

// TestResolveFilePathRejectsEscape 锁定安全审计 v6 M-2：
// capture_path/file_path 必须落在 dataDir 内（旧实现绝对路径直通 → 可 O_APPEND/轮转删除任意文件）。
func TestResolveFilePathRejectsEscape(t *testing.T) {
	dir := t.TempDir()

	w := &LogWriter{dataDir: dir, filePath: "../../etc/cron.d/pwn"}
	if got := w.resolveFilePath(); got != "" {
		t.Fatalf("traversal capture path must be rejected, got %q", got)
	}

	w = &LogWriter{dataDir: dir, filePath: "/etc/cron.d/pwn"}
	if got := w.resolveFilePath(); got != "" {
		t.Fatalf("absolute capture path outside dataDir must be rejected, got %q", got)
	}

	w = &LogWriter{dataDir: dir, filePath: "logs/nginx-stdout.log"}
	if got := w.resolveFilePath(); got != filepath.Join(dir, "logs", "nginx-stdout.log") {
		t.Fatalf("legitimate relative path should resolve under dataDir, got %q", got)
	}
}
