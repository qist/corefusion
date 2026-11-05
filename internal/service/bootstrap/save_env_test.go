package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	seccrypto "corefusion/internal/security/crypto"
)

// 初始化生成的 .env 不能包含加密主密钥：它只应写进 .env.secret。
// 否则下次启动会被当作"存量部署"搬家，并留下一个含密钥的 .env.bak。
func TestSaveEnvKeepsSecretOutOfDotEnv(t *testing.T) {
	dir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	d := NewDraft(dir)
	d.SecretEncryptionKey = strings.Repeat("a", 64)
	if err := SaveEnv(dir, d); err != nil {
		t.Fatalf("SaveEnv: %v", err)
	}

	env, err := os.ReadFile(".env")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(env), seccrypto.SecretEncryptionKeyName) {
		t.Fatalf(".env must not contain %s:\n%s", seccrypto.SecretEncryptionKeyName, env)
	}
	sec, err := os.ReadFile(seccrypto.SecretEnvFile)
	if err != nil {
		t.Fatalf("read %s: %v", seccrypto.SecretEnvFile, err)
	}
	if !strings.Contains(string(sec), d.SecretEncryptionKey) {
		t.Fatalf("%s missing key:\n%s", seccrypto.SecretEnvFile, sec)
	}
	if fi, err := os.Stat(".env"); err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf(".env perm = %v (err=%v), want 0600", fi.Mode().Perm(), err)
	}

	// 启动时的存量搬家必须是 no-op：不能再生成含密钥的 .env.bak。
	seccrypto.RelocateKeyToSecretFile(seccrypto.SecretEncryptionKeyName)
	if m, _ := filepath.Glob(".env.bak-*"); len(m) != 0 {
		t.Fatalf("relocation should be a no-op after init, got backups %v", m)
	}

	// 下载/展示用的 .env 文本同样不能带密钥（只允许注释里提到键名）。
	if strings.Contains(EnvContent(dir, d), seccrypto.SecretEncryptionKeyName+"=") {
		t.Fatalf("EnvContent must not contain %s=", seccrypto.SecretEncryptionKeyName)
	}
}

// 老版本 .env 里残留密钥时，初始化保存应把它迁移到 .env.secret 并从 .env 删除。
func TestSaveEnvMovesLegacyKeyOutOfDotEnv(t *testing.T) {
	dir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	legacy := "SAFE_PATHS=" + dir + "\n" + seccrypto.SecretEncryptionKeyName + "=legacy-key\n"
	if err := os.WriteFile(".env", []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	d := NewDraft(dir)
	d.SecretEncryptionKey = "new-key"
	if err := SaveEnv(dir, d); err != nil {
		t.Fatalf("SaveEnv: %v", err)
	}

	env, _ := os.ReadFile(".env")
	if strings.Contains(string(env), seccrypto.SecretEncryptionKeyName) {
		t.Fatalf(".env still contains legacy key:\n%s", env)
	}
	if got := seccrypto.ReadSecretEnvValue(seccrypto.SecretEncryptionKeyName); got != "new-key" {
		t.Fatalf(".env.secret = %q, want new-key", got)
	}
}
