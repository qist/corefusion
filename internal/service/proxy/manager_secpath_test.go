package proxy

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestDownloadAssetRejectsUnknownType 锁定安全审计 v6 H-3 的第一道防线：
// `type` 来自请求体，非 nginx/caddy 一律拒绝，且不发起任何下载。
func TestDownloadAssetRejectsUnknownType(t *testing.T) {
	m := &Manager{dataDir: t.TempDir(), binDir: filepath.Join(t.TempDir(), "bin")}
	if _, err := m.DownloadAsset(context.Background(), "http://127.0.0.1:1/x", "../../../../etc/cron.d/pwn"); err == nil {
		t.Fatal("unknown proxy type must be rejected")
	}
}

// TestExtractFromTarGzRejectsLuaTraversal 锁定 H-3 的第二道防线：
// tar 条目 `nginx/lua/../../..` 不能写出 dataDir/proxy/lua。
func TestExtractFromTarGzRejectsLuaTraversal(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	escapeName := "cf-pwn-" + filepath.Base(t.TempDir())
	payload := []byte("pwn")
	hdr := &tar.Header{
		Name:     "nginx/lua/../../../../tmp/" + escapeName,
		Mode:     0644,
		Size:     int64(len(payload)),
		Typeflag: tar.TypeReg,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(payload); err != nil {
		t.Fatal(err)
	}
	// 一个合法条目：应正常落到 proxy/lua/resty/init.lua
	okPayload := []byte("-- ok")
	if err := tw.WriteHeader(&tar.Header{
		Name:     "nginx/lua/resty/init.lua",
		Mode:     0644,
		Size:     int64(len(okPayload)),
		Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(okPayload); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}

	dataDir := t.TempDir()
	m := &Manager{dataDir: dataDir, binDir: filepath.Join(dataDir, "bin")}
	if err := os.MkdirAll(m.binDir, 0755); err != nil {
		t.Fatal(err)
	}
	_, _ = m.extractFromTarGz(bytes.NewReader(buf.Bytes()), "nginx", filepath.Join(m.binDir, "nginx-linux-amd64"))

	if _, err := os.Stat("/tmp/" + escapeName); err == nil {
		t.Fatalf("tar entry escaped to /tmp/%s", escapeName)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "proxy", "lua", "resty", "init.lua")); err != nil {
		t.Fatalf("legitimate lua entry should be extracted: %v", err)
	}
}
