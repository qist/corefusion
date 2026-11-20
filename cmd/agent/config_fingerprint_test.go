package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 安全审计 v7 L-5：面板证书指纹允许 fingerprint（带冒号）与 sha256sum（纯 hex）两种写法。
func TestNormalizeCertFingerprint(t *testing.T) {
	raw := "AA:BB:CC:dd:ee:ff"
	if got := normalizeCertFingerprint("  " + raw + " "); got != "aabbccddeeff" {
		t.Fatalf("normalizeCertFingerprint(%q) = %q", raw, got)
	}
	if got := normalizeCertFingerprint(""); got != "" {
		t.Fatalf("空指纹应归一化为空，得到 %q", got)
	}
	if got := normalizeCertFingerprint("aabb cc dd"); got != "aabbccdd" {
		t.Fatalf("空格应被去掉，得到 %q", got)
	}
}

// 指纹写错（长度不是 64 位 hex）时配置加载应直接报错，而不是静默忽略后走 TOFU。
func TestLoadConfigRejectsBadFingerprint(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	cfg := "panel:\n  host: 127.0.0.1\n  port: 8080\n  server_id: 1\nbootstrap:\n  token: abc\n  panel_cert_fingerprint: \"not-a-fingerprint\"\n"
	if err := os.WriteFile(path, []byte(cfg), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadConfig(path); err == nil {
		t.Fatal("非法指纹应导致配置加载失败")
	} else if !strings.Contains(err.Error(), "panel_cert_fingerprint") {
		t.Fatalf("错误信息应指出字段：%v", err)
	}

	// 合法指纹（带冒号）应被接受并归一化
	good := strings.Repeat("AB", 32)
	withColons := ""
	for i := 0; i < len(good); i += 2 {
		if withColons != "" {
			withColons += ":"
		}
		withColons += good[i : i+2]
	}
	cfg = "panel:\n  host: 127.0.0.1\n  port: 8080\n  server_id: 1\nbootstrap:\n  token: abc\n  panel_cert_fingerprint: \"" + withColons + "\"\n"
	if err := os.WriteFile(path, []byte(cfg), 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadConfig(path)
	if err != nil {
		t.Fatalf("合法指纹不应报错：%v", err)
	}
	if loaded.Bootstrap.PanelCertFingerprint != strings.ToLower(strings.ReplaceAll(withColons, ":", "")) {
		t.Fatalf("指纹未归一化：%q", loaded.Bootstrap.PanelCertFingerprint)
	}
}

// 安装脚本写入的 update.public_key 是 YAML 块标量，加载后应保留完整 PEM。
func TestLoadConfigParsesUpdatePublicKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	cfg := "panel:\n  host: 127.0.0.1\n  port: 8080\n  server_id: 1\n" +
		"bootstrap:\n  token: abc\n" +
		"update:\n  public_key: |\n    -----BEGIN PUBLIC KEY-----\n    MCowBQYDK2VwAyEA\n    -----END PUBLIC KEY-----\n"
	if err := os.WriteFile(path, []byte(cfg), 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadConfig(path)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	key := loaded.Update.PublicKey
	for _, want := range []string{"BEGIN PUBLIC KEY", "MCowBQYDK2VwAyEA", "END PUBLIC KEY"} {
		if !strings.Contains(key, want) {
			t.Fatalf("update.public_key 缺少 %q：%q", want, key)
		}
	}
}
