package binaries

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// TestNormalizeDigest 锁定安全审计 v6 M-10 的校验和解析：
// GitHub API 返回 "sha256:<hex>"，非 sha256 / 长度不对一律视为「没有校验和」。
func TestNormalizeDigest(t *testing.T) {
	const hex64 = "b49089ec95ac3ce5df084bc5fb0a97b9cb3b05f903920f5e9af31d1231b99746"

	cases := []struct {
		in   string
		want string
	}{
		{"sha256:" + hex64, hex64},
		{"SHA256:" + hex64, hex64},
		{" " + hex64 + " ", hex64},
		{"sha512:" + hex64, ""},
		{"sha256:deadbeef", ""},
		{"", ""},
		{"sha256:zzzzZzZzZzZzZzZzZzZzZzZzZzZzZzZzZzZzZzZzZzZzZzZzZzZzZzZzZzZzZzZz", ""},
	}
	for _, c := range cases {
		if got := normalizeDigest(c.in); got != c.want {
			t.Errorf("normalizeDigest(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestFindReleaseDigest 按下载 URL / 资产名匹配 digest；匹配不到返回空（调用方决定拒绝还是告警）。
func TestFindReleaseDigest(t *testing.T) {
	const d1 = "1111111111111111111111111111111111111111111111111111111111111111"
	const d2 = "2222222222222222222222222222222222222222222222222222222222222222"
	releases := []GitHubRelease{{
		TagName: "v26.10.31",
		Assets: []GitHubAsset{
			{Name: "Xray-linux-64.zip", BrowserDownloadURL: "https://github.com/XTLS/Xray-core/releases/download/v26.10.31/Xray-linux-64.zip", Digest: "sha256:" + d1},
			{Name: "Xray-linux-arm64-v8a.zip", BrowserDownloadURL: "https://github.com/XTLS/Xray-core/releases/download/v26.10.31/Xray-linux-arm64-v8a.zip", Digest: "sha256:" + d2},
		},
	}}

	if got := findReleaseDigest(releases, "v26.10.31", "https://github.com/XTLS/Xray-core/releases/download/v26.10.31/Xray-linux-64.zip", ""); got != d1 {
		t.Fatalf("by exact URL: got %q want %q", got, d1)
	}
	if got := findReleaseDigest(releases, "v26.10.31", "https://mirror.example.com/mirror/Xray-linux-arm64-v8a.zip", ""); got != d2 {
		t.Fatalf("by base name: got %q want %q", got, d2)
	}
	if got := findReleaseDigest(releases, "v26.10.31", "https://mirror.example.com/unknown.zip", ""); got != "" {
		t.Fatalf("unknown asset should have no digest, got %q", got)
	}
	if got := findReleaseDigest(nil, "v26.10.31", "https://github.com/XTLS/Xray-core/releases/download/v26.10.31/Xray-linux-64.zip", ""); got != "" {
		t.Fatalf("nil releases should have no digest, got %q", got)
	}
}

// TestDownloadAndExtractRejectsChecksumMismatch 锁定 M-10 的拒绝路径：
// 整包 SHA-256 不匹配时必须直接失败，且不落盘。
func TestDownloadAndExtractRejectsChecksumMismatch(t *testing.T) {
	payload := []byte("this-is-not-a-real-archive")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	inst := NewInstaller(t.TempDir(), NewResolver(t.TempDir()), nil)
	destDir := t.TempDir()

	// 故意给一个错误的 64 位 hex 校验和
	wrong := "0000000000000000000000000000000000000000000000000000000000000000"
	err := inst.downloadAndExtract(context.Background(), srv.URL, destDir, "xray", wrong)
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected checksum mismatch, got %v", err)
	}
	entries, _ := os.ReadDir(destDir)
	if len(entries) != 0 {
		t.Fatalf("no file should be written on checksum mismatch, found %d", len(entries))
	}

	// 正确的校验和 → 不应再报 checksum 错误（后续解包失败属预期）
	sum := sha256.Sum256(payload)
	ok := hex.EncodeToString(sum[:])
	err = inst.downloadAndExtract(context.Background(), srv.URL, destDir, "xray", ok)
	if err != nil && strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("valid checksum should not be rejected: %v", err)
	}
}
