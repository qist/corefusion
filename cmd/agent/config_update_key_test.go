package main

import (
	"os"
	"strings"
	"testing"
)

const testUpdateKeyPEM = `-----BEGIN PUBLIC KEY-----
MCowBQYDK2VwAyEA0UyJUJa4YyWwHhrRKEC7ofsiCs6a1cEPvtse72ztO7Q=
-----END PUBLIC KEY-----`

// 老节点 config.yaml 没有 update 段：升级后必须能补上，且原有内容/注释保持不变。
func TestWriteUpdatePublicKeyAppendsMissingBlock(t *testing.T) {
	body := `panel:
  host: "panel.example.com"
  port: 8080
  server_id: 7

bootstrap:
  token: "abc"   # 一次性令牌
agent:
  log_level: info
`
	path := writeAgentConfig(t, body)
	if err := WriteUpdatePublicKey(path, testUpdateKeyPEM); err != nil {
		t.Fatalf("write: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	got := string(raw)
	if !strings.Contains(got, "update:\n  public_key: |\n") {
		t.Fatalf("缺少 update 配置块：\n%s", got)
	}
	if !strings.Contains(got, "    -----BEGIN PUBLIC KEY-----") {
		t.Fatalf("PEM 未缩进写入：\n%s", got)
	}
	if !strings.Contains(got, `token: "abc"   # 一次性令牌`) {
		t.Fatal("原有注释/字段不应被改动")
	}
	if strings.Count(got, "public_key:") != 1 {
		t.Fatalf("public_key 应只出现一次：\n%s", got)
	}

	// 文件里已有 update 段且其它字段存在：只替换该段，不产生重复
	body2 := `panel:
  host: "h"
update:
  public_key: |
    -----BEGIN PUBLIC KEY-----
    OLD
    -----END PUBLIC KEY-----
agent:
  log_level: debug
`
	path2 := writeAgentConfig(t, body2)
	if err := WriteUpdatePublicKey(path2, testUpdateKeyPEM); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	raw2, _ := os.ReadFile(path2)
	got2 := string(raw2)
	if strings.Contains(got2, "OLD") {
		t.Fatal("旧公钥应被替换")
	}
	if strings.Count(got2, "update:") != 1 {
		t.Fatalf("update 段应只有一个：\n%s", got2)
	}
	if !strings.Contains(got2, "log_level: debug") {
		t.Fatal("update 段之后的字段不应丢失")
	}
}

// 配置里有 bootstrap token：回写后权限必须仍是 0600。
func TestWriteUpdatePublicKeyKeeps0600(t *testing.T) {
	path := writeAgentConfig(t, "panel:\n  host: h\n")
	if err := WriteUpdatePublicKey(path, testUpdateKeyPEM); err != nil {
		t.Fatalf("write: %v", err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o, want 600", fi.Mode().Perm())
	}
}

func TestWriteUpdatePublicKeyRejectsEmpty(t *testing.T) {
	path := writeAgentConfig(t, "panel:\n  host: h\n")
	if err := WriteUpdatePublicKey(path, "   "); err == nil {
		t.Fatal("空公钥应被拒绝")
	}
}
