package main

import (
	"os"
	"path/filepath"
	"testing"
)

// 证书路径以 data 目录为基准存相对路径时，必须能在 dataDir 下找到；
// 历史"相对 cwd"写法也要继续可用。
func TestResolveDataPath(t *testing.T) {
	dataDir := t.TempDir()
	cwd := t.TempDir()

	if err := os.MkdirAll(filepath.Join(dataDir, "certs", "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "certs", "a", "cert.pem"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(cwd, "data", "certs", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cwd, "data", "certs", "b", "cert.pem"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(cwd); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)

	if got := resolveDataPath(dataDir, "certs/a/cert.pem"); got != filepath.Join(dataDir, "certs/a/cert.pem") {
		t.Fatalf("data-relative path = %q", got)
	}
	if got := resolveDataPath(dataDir, "data/certs/b/cert.pem"); got != filepath.Join("data", "certs/b/cert.pem") {
		t.Fatalf("cwd-relative legacy path = %q", got)
	}
	if got := resolveDataPath(dataDir, "certs/missing/cert.pem"); got != filepath.Join(dataDir, "certs/missing/cert.pem") {
		t.Fatalf("missing path should still resolve under dataDir, got %q", got)
	}
	if got := resolveDataPath(dataDir, "/abs/cert.pem"); got != "/abs/cert.pem" {
		t.Fatalf("absolute path changed: %q", got)
	}
	if got := resolveDataPath(dataDir, ""); got != "" {
		t.Fatalf("empty path changed: %q", got)
	}
}
