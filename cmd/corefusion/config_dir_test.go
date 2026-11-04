package main

import (
	"os"
	"path/filepath"
	"testing"
)

// .env / .env.secret 是相对路径：配置目录必须锚定到部署目录，
// 否则在 /root、/tmp 等目录跑 CLI 会静默生成另一个密钥文件。
func TestResolveConfigDir(t *testing.T) {
	tmp := t.TempDir()
	cwd := filepath.Join(tmp, "cwd")
	exe := filepath.Join(tmp, "deploy")
	for _, d := range []string{cwd, exe} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// 1) cwd 有 .env → 用 cwd（systemd WorkingDirectory 的常规情况）
	if err := os.WriteFile(filepath.Join(cwd, ".env"), []byte("A=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := resolveConfigDir(cwd, exe); got != cwd {
		t.Fatalf("cwd with .env: got %q, want %q", got, cwd)
	}
	if err := os.Remove(filepath.Join(cwd, ".env")); err != nil {
		t.Fatal(err)
	}

	// 2) cwd 没有 .env，部署目录有 → 用部署目录
	if err := os.WriteFile(filepath.Join(exe, ".env"), []byte("A=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := resolveConfigDir(cwd, exe); got != exe {
		t.Fatalf("deploy dir with .env: got %q, want %q", got, exe)
	}

	// 3) 只有部署目录的 .env.secret 也算部署目录（密钥不会被重新生成）
	if err := os.Remove(filepath.Join(exe, ".env")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(exe, ".env.secret"), []byte("K=v\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := resolveConfigDir(cwd, exe); got != exe {
		t.Fatalf("deploy dir with .env.secret: got %q, want %q", got, exe)
	}

	// 4) 两边都没有（首次部署）→ 留在 cwd
	if err := os.Remove(filepath.Join(exe, ".env.secret")); err != nil {
		t.Fatal(err)
	}
	if got := resolveConfigDir(cwd, exe); got != cwd {
		t.Fatalf("fresh install: got %q, want %q", got, cwd)
	}

	// 5) cwd 与部署目录相同 → 原样返回
	if got := resolveConfigDir(exe, exe); got != exe {
		t.Fatalf("same dir: got %q, want %q", got, exe)
	}
}
