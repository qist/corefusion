//go:build !windows

package process

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, listens ...string) string {
	t.Helper()
	dir := t.TempDir()
	cfg := map[string]any{
		"inbounds": func() []map[string]any {
			out := make([]map[string]any, 0, len(listens))
			for _, l := range listens {
				out = append(out, map[string]any{"listen": l, "port": 443, "protocol": "trojan"})
			}
			return out
		}(),
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

// 崩溃残留的 socket 文件必须被清理，否则新核心 bind 会 EADDRINUSE。
func TestCleanupStaleUnixSocketsRemovesDeadSocket(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "dead.sock")

	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	// 模拟 SIGKILL：关闭 listener 但不 unlink 文件。
	ln.(*net.UnixListener).SetUnlinkOnClose(false)
	_ = ln.Close()
	if _, err := os.Stat(sock); err != nil {
		t.Fatalf("precondition: socket file should remain: %v", err)
	}

	cfg := writeConfig(t, sock+",0666")
	if n := cleanupStaleUnixSockets(cfg); n != 1 {
		t.Fatalf("expected 1 removed, got %d", n)
	}
	if _, err := os.Stat(sock); !os.IsNotExist(err) {
		t.Fatalf("socket file should be removed, got err=%v", err)
	}
	if _, err := os.Stat(sock + ".lock"); !os.IsNotExist(err) {
		t.Fatalf("lock file should be removed, got err=%v", err)
	}
}

// 有进程在 listen 的 socket 绝不能删。
func TestCleanupStaleUnixSocketsKeepsLiveSocket(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "live.sock")

	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	cfg := writeConfig(t, sock)
	if n := cleanupStaleUnixSockets(cfg); n != 0 {
		t.Fatalf("expected 0 removed, got %d", n)
	}
	if _, err := os.Stat(sock); err != nil {
		t.Fatalf("live socket must remain: %v", err)
	}
}

// abstract socket（@name）不落盘，不应参与清理；普通文件也不能误删。
func TestCleanupStaleUnixSocketsIgnoresNonSocketPaths(t *testing.T) {
	dir := t.TempDir()
	regular := filepath.Join(dir, "not-a-socket.txt")
	if err := os.WriteFile(regular, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}

	cfg := writeConfig(t, "@vless-grpc", regular, "127.0.0.1", "")
	if n := cleanupStaleUnixSockets(cfg); n != 0 {
		t.Fatalf("expected 0 removed, got %d", n)
	}
	if _, err := os.Stat(regular); err != nil {
		t.Fatalf("regular file must remain: %v", err)
	}
}

func TestUnixSocketPath(t *testing.T) {
	cases := map[string]string{
		"/dev/shm/x.sock,0666": "/dev/shm/x.sock",
		"unix:/dev/shm/x.sock": "/dev/shm/x.sock",
		"@vless-grpc":          "",
		"127.0.0.1":            "",
		"0.0.0.0":              "",
		"":                     "",
		"  /dev/shm/y.sock  ":  "/dev/shm/y.sock",
	}
	for in, want := range cases {
		if got := unixSocketPath(in); got != want {
			t.Errorf("unixSocketPath(%q) = %q, want %q", in, got, want)
		}
	}
}
