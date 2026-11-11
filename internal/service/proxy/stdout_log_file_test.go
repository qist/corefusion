package proxy

import (
	"os"
	"path/filepath"
	"testing"
)

// 旧版 nginx stdout 模式是 logs/stdout_access.log -> /dev/stdout 软链。
// 面板/Agent 重启后软链会解析到新进程自己的 stdout：既读不到 nginx 日志，
// 旧 nginx 也还在往已断开的管道写。这里锁定“必须是真实文件”。
func TestEnsureRegularLogFileReplacesStdoutSymlink(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stdout_access.log")
	if err := os.Symlink("/dev/stdout", path); err != nil {
		t.Fatal(err)
	}

	ensureRegularLogFile(path)

	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("legacy /dev/stdout symlink was not replaced")
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("expected regular file, got mode %v", info.Mode())
	}
}

// 已是普通文件时应保留原内容（O_APPEND 追加，不能截断）。
func TestEnsureRegularLogFileKeepsExistingContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stdout_access.log")
	if err := os.WriteFile(path, []byte("existing line\n"), 0o640); err != nil {
		t.Fatal(err)
	}

	ensureRegularLogFile(path)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "existing line\n" {
		t.Fatalf("existing content was altered: %q", data)
	}
}

func TestNormalizeProxyLogModeRetiresStdout(t *testing.T) {
	cases := map[string]string{
		"stdout":  "file",
		"file":    "file",
		"":        "file",
		"unknown": "file",
		"off":     "off",
	}
	for in, want := range cases {
		if got := normalizeProxyLogMode(in); got != want {
			t.Errorf("normalizeProxyLogMode(%q) = %q, want %q", in, got, want)
		}
	}
}

// 运行中的代理是否仍使用旧版 stdout 管道/软链：需要重启一次迁移到文件。
func TestIsLegacyStdoutPipeProxy(t *testing.T) {
	dir := t.TempDir()
	m := &Manager{
		dataDir:   dir,
		configDir: filepath.Join(dir, "proxy"),
	}
	if err := os.MkdirAll(filepath.Join(dir, "logs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(m.configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// nginx：软链 = 旧版，需要重启；真实文件 = 已迁移，可接管。
	if err := os.Symlink("/dev/stdout", m.getNginxStdoutAccessLogPath()); err != nil {
		t.Fatal(err)
	}
	if !m.isLegacyStdoutPipeProxy(ProxyNginx) {
		t.Fatal("nginx symlink should be detected as legacy stdout mode")
	}
	if err := os.Remove(m.getNginxStdoutAccessLogPath()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(m.getNginxStdoutAccessLogPath(), nil, 0o640); err != nil {
		t.Fatal(err)
	}
	if m.isLegacyStdoutPipeProxy(ProxyNginx) {
		t.Fatal("regular log file must be adoptable")
	}

	// caddy：旧 Caddyfile 里是 output stdout，新配置是 output file。
	caddyfile := filepath.Join(m.configDir, "Caddyfile")
	if err := os.WriteFile(caddyfile, []byte("log default {\n\toutput stdout\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !m.isLegacyStdoutPipeProxy(ProxyCaddy) {
		t.Fatal("caddy output stdout should be detected as legacy stdout mode")
	}
	if err := os.WriteFile(caddyfile, []byte("log default {\n\toutput file logs/caddy_access.log\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if m.isLegacyStdoutPipeProxy(ProxyCaddy) {
		t.Fatal("caddy output file must be adoptable")
	}
}
