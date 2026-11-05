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
	// 迁移前会备份 .env（便于人工回滚），但备份里不能留明文密钥
	matches, _ := filepath.Glob(filepath.Join(dir, ".env.bak-*"))
	if len(matches) != 1 {
		t.Fatalf("expected one .env backup, got %v", matches)
	}
	b, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if strings.Contains(string(b), testKey) {
		t.Fatalf("backup must not contain plaintext key:\n%s", b)
	}
	if !strings.Contains(string(b), "CF_SECRET_ENCRYPTION_KEY=#redacted") {
		t.Fatalf("backup should mark the key as redacted:\n%s", b)
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

// TestEnsureSecretKeyAt 锁定节点侧行为（Agent 没有 .env）：
// 首次生成 32 字节随机密钥写入 <dir>/.env.secret（0600），之后复用不再变。
func TestEnsureSecretKeyAt(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CF_SECRET_ENCRYPTION_KEY", "")
	os.Unsetenv("CF_SECRET_ENCRYPTION_KEY")

	first, err := EnsureSecretKeyAt(dir, "CF_SECRET_ENCRYPTION_KEY")
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 64 {
		t.Fatalf("key should be 64 hex chars, got %d", len(first))
	}
	path := filepath.Join(dir, SecretEnvFile)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("secret file not created: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Fatalf("permission = %o, want 600", perm)
	}

	second, err := EnsureSecretKeyAt(dir, "CF_SECRET_ENCRYPTION_KEY")
	if err != nil {
		t.Fatal(err)
	}
	if second != first {
		t.Fatalf("key must be reused across restarts, got %q vs %q", second, first)
	}
}

// TestEnsureSecretKeyAtPrefersEnv 运维显式注入的环境变量优先，不覆盖文件。
func TestEnsureSecretKeyAtPrefersEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CF_SECRET_ENCRYPTION_KEY", testKey)

	got, err := EnsureSecretKeyAt(dir, "CF_SECRET_ENCRYPTION_KEY")
	if err != nil {
		t.Fatal(err)
	}
	if got != testKey {
		t.Fatalf("env should win, got %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, SecretEnvFile)); !os.IsNotExist(err) {
		t.Fatalf("no file should be created when env provides the key (err=%v)", err)
	}
}

// TestReencryptEnvFileLegacy 锁定"历史主机名密钥 → 当前主密钥"的迁移：
// 老密文必须能被解出来并用新密钥重写，明文值不能变；重复执行幂等。
func TestReencryptEnvFileLegacy(t *testing.T) {
	inTempDir(t)

	// 1) 模拟老部署：没有 env 密钥 → 用 legacy hostname 派生密钥加密
	os.Unsetenv("CF_SECRET_ENCRYPTION_KEY")
	defaultKey = nil
	const dsn = "postgres://user:p%40ss@127.0.0.1:5432/corefusion"
	legacyEnc, err := EncryptSecret(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(".env", []byte("SAFE_PATHS=data\nCF_DB_DSN="+legacyEnc+"\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// 2) 切换到新的随机主密钥（模拟升级后 .env.secret 就绪）
	t.Setenv("CF_SECRET_ENCRYPTION_KEY", testKey)
	defaultKey = nil
	if CanDecryptWithPrimary(legacyEnc) {
		t.Fatal("legacy ciphertext must not decrypt with the new key before migration")
	}

	// 3) 迁移
	n, err := ReencryptEnvFile(".env")
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 rewritten value, got %d", n)
	}

	// 4) 迁移后：主密钥可解、明文不变、其它键保留
	migrated := ReadEnvFileValue(".env", "CF_DB_DSN")
	if !CanDecryptWithPrimary(migrated) {
		t.Fatal("migrated value must be decryptable with the primary key")
	}
	if migrated == legacyEnc {
		t.Fatal("ciphertext should change after re-encryption")
	}
	plain, err := DecryptSecret(migrated)
	if err != nil || plain != dsn {
		t.Fatalf("plaintext changed: %q err=%v", plain, err)
	}
	if ReadEnvFileValue(".env", "SAFE_PATHS") != "data" {
		t.Fatal("unrelated keys must be preserved")
	}

	// 5) 幂等：再跑一次不重写
	if n2, err := ReencryptEnvFile(".env"); err != nil || n2 != 0 {
		t.Fatalf("second run should be a no-op, got n=%d err=%v", n2, err)
	}
}

// TestCanDecryptWithPrimaryRejectsPlainAndGarbage 只对 enc: 密文返回 true。
func TestCanDecryptWithPrimaryRejectsPlainAndGarbage(t *testing.T) {
	t.Setenv("CF_SECRET_ENCRYPTION_KEY", testKey)
	defaultKey = nil
	if CanDecryptWithPrimary("plain-value") {
		t.Fatal("plain value must not be treated as primary-encrypted")
	}
	if CanDecryptWithPrimary("enc:not-base64:%%%") {
		t.Fatal("garbage must not be treated as primary-encrypted")
	}
	enc, err := EncryptSecret("hello")
	if err != nil {
		t.Fatal(err)
	}
	if !CanDecryptWithPrimary(enc) {
		t.Fatal("freshly encrypted value must decrypt with primary key")
	}
}
