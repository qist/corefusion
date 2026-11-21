package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveAgentCAPath(t *testing.T) {
	t.Run("新路径存在时优先使用", func(t *testing.T) {
		dataDir := filepath.Join(t.TempDir(), "data")
		newPath := filepath.Join(dataDir, "certs", "ca-cert.pem")
		if err := os.MkdirAll(filepath.Dir(newPath), 0750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(newPath, []byte("new"), 0600); err != nil {
			t.Fatal(err)
		}
		if got := resolveAgentCAPath(dataDir); got != newPath {
			t.Fatalf("resolveAgentCAPath() = %q, want %q", got, newPath)
		}
	})

	t.Run("新路径缺失时回退旧安装脚本路径", func(t *testing.T) {
		installDir := t.TempDir()
		dataDir := filepath.Join(installDir, "data")
		legacyPath := filepath.Join(installDir, "certs", "ca-cert.pem")
		if err := os.MkdirAll(filepath.Dir(legacyPath), 0750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(legacyPath, []byte("legacy"), 0600); err != nil {
			t.Fatal(err)
		}
		if got := resolveAgentCAPath(dataDir); got != legacyPath {
			t.Fatalf("resolveAgentCAPath() = %q, want legacy %q", got, legacyPath)
		}
	})

	t.Run("两处都不存在时返回标准路径", func(t *testing.T) {
		dataDir := filepath.Join(t.TempDir(), "data")
		want := filepath.Join(dataDir, "certs", "ca-cert.pem")
		if got := resolveAgentCAPath(dataDir); got != want {
			t.Fatalf("resolveAgentCAPath() = %q, want %q", got, want)
		}
	})
}

func TestExtractSubIDFromEmail(t *testing.T) {
	cases := []struct {
		email string
		want  int64
	}{
		{"sub_12@example.com", 12},
		{"12@example.com", 12},
		{"12@example.com_vless12345", 12},
		// 回归：数字前缀不是订阅 ID，否则直连账号会被错误归户到别人的订阅。
		{"4wei@163.com", 0},
		{"4wei@163.com_vless12345", 0},
		{"sub_4wei@163.com", 0},
		{"user@example.com", 0},
		{"sub_abc@example.com", 0},
		{"", 0},
	}
	for _, tc := range cases {
		if got := extractSubIDFromEmail(tc.email); got != tc.want {
			t.Errorf("extractSubIDFromEmail(%q) = %d, want %d", tc.email, got, tc.want)
		}
	}
}
