package crypto

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// inTempDir 让测试在独立目录里跑（迁移函数使用相对路径 .env / .env.secret）。
func inTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	return dir
}

const testKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// TestSaveSecretEnvValuePermissions 独立密钥文件必须是 0600（含只读收紧场景）。
func TestSaveSecretEnvValuePermissions(t *testing.T) {
	dir := inTempDir(t)
	if err := SaveSecretEnvValue("CF_SECRET_ENCRYPTION_KEY", testKey); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, SecretEnvFile))
	if err != nil {
		t.Fatalf("secret file not created: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Fatalf("secret file permission = %o, want 600", perm)
	}
	if got := ReadSecretEnvValue("CF_SECRET_ENCRYPTION_KEY"); got != testKey {
		t.Fatalf("round-trip mismatch: %q", got)
	}
}

// TestRelocateKeyToSecretFile 锁定"老部署密钥搬家"的核心保证：
// 迁移后 .env 不再含密钥、.env.secret 值与原值**完全一致**（否则历史密文全废）。
func TestRelocateKeyToSecretFile(t *testing.T) {
	dir := inTempDir(t)
	envContent := "SAFE_PATHS=data\nCF_DB_DSN=enc:abc:def\nCF_SECRET_ENCRYPTION_KEY=" + testKey + "\nJWT_SECRET=xyz\n"
	if err := os.WriteFile(".env", []byte(envContent), 0600); err != nil {
		t.Fatal(err)
	}

	RelocateKeyToSecretFile("CF_SECRET_ENCRYPTION_KEY")

	if got := ReadSecretEnvValue("CF_SECRET_ENCRYPTION_KEY"); got != testKey {
		t.Fatalf("secret file key = %q, want original %q (old ciphertext would be lost)", got, testKey)
	}
	envData, err := os.ReadFile(".env")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(envData), "CF_SECRET_ENCRYPTION_KEY") {
		t.Fatalf(".env still contains the key:\n%s", envData)
	}
	// 其它配置必须原样保留
	for _, want := range []string{"SAFE_PATHS=data", "CF_DB_DSN=enc:abc:def", "JWT_SECRET=xyz"} {
		if !strings.Contains(string(envData), want) {
			t.Fatalf(".env lost %q:\n%s", want, envData)
		}
	}
	// 迁移前会备份 .env（便于人工回滚）
	matches, _ := filepath.Glob(filepath.Join(dir, ".env.bak-*"))
	if len(matches) != 1 {
		t.Fatalf("expected one .env backup, got %v", matches)
	}
	if b, err := os.ReadFile(matches[0]); err != nil || !strings.Contains(string(b), "CF_SECRET_ENCRYPTION_KEY="+testKey) {
		t.Fatalf("backup should still contain original key: %v", err)
	}
}

// TestRelocateIdempotent 重复执行（每次启动都会跑）必须安全且结果不变。
func TestRelocateIdempotent(t *testing.T) {
	inTempDir(t)
	if err := os.WriteFile(".env", []byte("SAFE_PATHS=data\nCF_SECRET_ENCRYPTION_KEY="+testKey+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	RelocateKeyToSecretFile("CF_SECRET_ENCRYPTION_KEY")
	RelocateKeyToSecretFile("CF_SECRET_ENCRYPTION_KEY")
	RelocateKeyToSecretFile("CF_SECRET_ENCRYPTION_KEY")

	if got := ReadSecretEnvValue("CF_SECRET_ENCRYPTION_KEY"); got != testKey {
		t.Fatalf("key changed after repeated relocation: %q", got)
	}
}

// TestRelocateDoesNotOverwriteDifferingKey 两边值不同时以 .env 为准、不覆盖独立文件，
// 避免"悄悄换掉正在使用的密钥"导致密文解不开。
func TestRelocateDoesNotOverwriteDifferingKey(t *testing.T) {
	inTempDir(t)
	const otherKey = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	if err := os.WriteFile(".env", []byte("CF_SECRET_ENCRYPTION_KEY="+testKey+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := SaveSecretEnvValue("CF_SECRET_ENCRYPTION_KEY", otherKey); err != nil {
		t.Fatal(err)
	}

	RelocateKeyToSecretFile("CF_SECRET_ENCRYPTION_KEY")

	if got := ReadSecretEnvValue("CF_SECRET_ENCRYPTION_KEY"); got != otherKey {
		t.Fatalf("secret file must not be overwritten, got %q", got)
	}
	if got := ReadEnvFileValue(".env", "CF_SECRET_ENCRYPTION_KEY"); got != testKey {
		t.Fatalf(".env value must be kept as source of truth, got %q", got)
	}
}

// TestRelocateNoopWhenKeyAbsent 新部署（.env 里没有密钥）时迁移什么都不做。
func TestRelocateNoopWhenKeyAbsent(t *testing.T) {
	inTempDir(t)
	if err := os.WriteFile(".env", []byte("SAFE_PATHS=data\n"), 0600); err != nil {
		t.Fatal(err)
	}
	RelocateKeyToSecretFile("CF_SECRET_ENCRYPTION_KEY")
	if _, err := os.Stat(SecretEnvFile); !os.IsNotExist(err) {
		t.Fatalf("secret file should not be created when key absent (err=%v)", err)
	}
}

// TestParseEnvContent 忽略注释/空行，保留值里的 = 号。
func TestParseEnvContent(t *testing.T) {
	got := ParseEnvContent("# comment\n\nA=1\nB=x=y\n  C = 3 \n")
	if got["A"] != "1" || got["B"] != "x=y" || got["C"] != "3" {
		t.Fatalf("unexpected parse result: %v", got)
	}
}
