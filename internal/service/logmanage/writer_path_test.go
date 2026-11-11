package logmanage

import (
	"path/filepath"
	"testing"
)

// TestResolveLogPathRejectsEscape 锁定安全审计 v6 M-2：
// 日志路径必须拒绝 ".." 穿越（旧实现直接 Join → 可 O_APPEND/轮转删除任意文件）。
//
// LogWriter（stdout 落盘）已下线，统一改由 resolveLogPath 解析日志路径；
// 绝对路径仍然允许（/var/log/nginx/access.log 这类系统日志需要被发现与轮转）。
func TestResolveLogPathRejectsEscape(t *testing.T) {
	dir := t.TempDir()
	s := &Service{dataDir: dir}

	if got := s.resolveLogPath("../../etc/cron.d/pwn"); got != "" {
		t.Fatalf("traversal log path must be rejected, got %q", got)
	}

	if got := s.resolveLogPath("logs/nginx-stdout.log"); got != filepath.Join(dir, "logs", "nginx-stdout.log") {
		t.Fatalf("legitimate relative path should resolve under dataDir, got %q", got)
	}

	// 绝对路径（系统日志）原样返回，供发现/查看/轮转使用。
	if got := s.resolveLogPath("/var/log/nginx/access.log"); got != "/var/log/nginx/access.log" {
		t.Fatalf("absolute system log path should be kept, got %q", got)
	}
}
