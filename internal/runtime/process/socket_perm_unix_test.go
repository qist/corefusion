//go:build !windows

package process

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
)

// 核心（xray）默认按 umask 把 unix socket 建成 0755，非 root 的 nginx worker 连不上，
// 客户端看到的是"连接被重置"。启动后必须自动放开到 0666，否则节点全废。
func TestEnsureUnixSocketPermsOpensSocketForProxyWorker(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "vless.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	defer ln.Close()
	if err := os.Chmod(sockPath, 0o755); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	cfgPath := filepath.Join(dir, "xray.jsonc")
	cfg := map[string]interface{}{
		"inbounds": []interface{}{
			map[string]interface{}{"listen": sockPath, "port": 58367},
			// 抽象 socket 不落盘，不应影响判定
			map[string]interface{}{"listen": "@vless-grpc", "port": 22554},
			// TCP 监听没有路径
			map[string]interface{}{"listen": "0.0.0.0", "port": 11995},
		},
	}
	data, _ := json.Marshal(cfg)
	if err := os.WriteFile(cfgPath, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	ensureUnixSocketPerms(cfgPath)

	fi, err := os.Stat(sockPath)
	if err != nil {
		t.Fatalf("stat socket: %v", err)
	}
	if fi.Mode().Perm()&0o666 != 0o666 {
		t.Fatalf("socket mode = %o, want writable by others", fi.Mode().Perm())
	}
}

// 配置里没有 socket 或文件不存在时不能卡住/不能改别的文件。
func TestEnsureUnixSocketPermsNoSocketIsNoop(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "xray.jsonc")
	other := filepath.Join(dir, "plain.txt")
	if err := os.WriteFile(other, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(map[string]interface{}{
		"inbounds": []interface{}{map[string]interface{}{"listen": "0.0.0.0", "port": 443}},
	})
	if err := os.WriteFile(cfgPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	ensureUnixSocketPerms(cfgPath)
	fi, _ := os.Stat(other)
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("unrelated file must stay 0600, got %o", fi.Mode().Perm())
	}
}
