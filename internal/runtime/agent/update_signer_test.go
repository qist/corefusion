package agent

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	secretcrypto "corefusion/internal/security/crypto"
)

func newTestSigner(t *testing.T) *AgentUpdateSigner {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return NewAgentUpdateSigner(priv)
}

// 安全审计 v7 L-6：签名必须能把「可信哈希」与「可信下载地址」解耦。
func TestAgentUpdateSignatureRoundTrip(t *testing.T) {
	signer := newTestSigner(t)
	checksum := strings.Repeat("ab", 32)
	sig := signer.SignChecksum(checksum)
	if sig == "" {
		t.Fatal("签名不应为空")
	}
	pub := signer.PublicKeyPEM()
	if len(pub) == 0 {
		t.Fatal("公钥 PEM 不应为空")
	}
	// 带 sha256: 前缀也应通过（面板/Agent 两边都会规范化）
	if err := VerifyAgentUpdateSignature(pub, "sha256:"+checksum, sig); err != nil {
		t.Fatalf("正确签名应通过：%v", err)
	}
	// 哈希被替换 → 必须失败
	if err := VerifyAgentUpdateSignature(pub, strings.Repeat("cd", 32), sig); err == nil {
		t.Fatal("哈希不匹配必须拒绝")
	}
	// 未签名 → 必须失败
	if err := VerifyAgentUpdateSignature(pub, checksum, ""); err == nil {
		t.Fatal("未签名更新必须拒绝")
	}
	// 换一把公钥 → 必须失败
	if err := VerifyAgentUpdateSignature(newTestSigner(t).PublicKeyPEM(), checksum, sig); err == nil {
		t.Fatal("公钥不匹配必须拒绝")
	}
}

// memSignerStore 是 UpdateSigningKeyStore 的内存实现。
type memSignerStore struct {
	vals map[string]string
}

func (m *memSignerStore) GetByKey(_ context.Context, key string) (string, error) {
	return m.vals[key], nil
}

func (m *memSignerStore) Set(_ context.Context, key, value string) error {
	if m.vals == nil {
		m.vals = map[string]string{}
	}
	m.vals[key] = value
	return nil
}

func TestLoadOrCreateAgentUpdateSignerPersists(t *testing.T) {
	dir := t.TempDir()
	store := &memSignerStore{}
	first, err := LoadOrCreateAgentUpdateSigner(context.Background(), dir, store)
	if err != nil {
		t.Fatalf("first load: %v", err)
	}
	// 私钥必须加密存库（备份只含数据库 → 密钥随备份迁移）
	saved := store.vals[UpdateSigningKeySettingKey]
	if !secretcrypto.IsEncrypted(saved) {
		t.Fatalf("数据库里的私钥未加密：%q", saved)
	}
	path := filepath.Join(dir, "agent", updateSigningKeyFile)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("key file missing: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("私钥文件权限 = %v, want 0600", info.Mode().Perm())
	}
	// 模拟重启（文件删掉，只留数据库）：必须还是同一把密钥
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	second, err := LoadOrCreateAgentUpdateSigner(context.Background(), dir, store)
	if err != nil {
		t.Fatalf("second load: %v", err)
	}
	checksum := strings.Repeat("ef", 32)
	if err := VerifyAgentUpdateSignature(second.PublicKeyPEM(), checksum, first.SignChecksum(checksum)); err != nil {
		t.Fatalf("重启后仍应使用同一把密钥：%v", err)
	}
}

// 旧版本只有文件：启动后应迁移进数据库，且密钥不变。
func TestLoadOrCreateAgentUpdateSignerMigratesLegacyFile(t *testing.T) {
	dir := t.TempDir()
	store := &memSignerStore{}
	// 先用文件模式生成
	fileOnly, err := LoadOrCreateAgentUpdateSigner(context.Background(), dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	checksum := strings.Repeat("12", 32)
	// 再用带 store 的模式加载（模拟升级后启动）
	migrated, err := LoadOrCreateAgentUpdateSigner(context.Background(), dir, store)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if !secretcrypto.IsEncrypted(store.vals[UpdateSigningKeySettingKey]) {
		t.Fatal("迁移后数据库应保存加密私钥")
	}
	if err := VerifyAgentUpdateSignature(migrated.PublicKeyPEM(), checksum, fileOnly.SignChecksum(checksum)); err != nil {
		t.Fatalf("迁移不应更换密钥：%v", err)
	}
}

func TestLoadOrCreateAgentUpdateSignerRejectsCorruptKey(t *testing.T) {
	// 数据库里是损坏的密文/明文 seed
	store := &memSignerStore{vals: map[string]string{UpdateSigningKeySettingKey: "not-a-valid-ciphertext"}}
	if _, err := LoadOrCreateAgentUpdateSigner(context.Background(), t.TempDir(), store); err == nil {
		t.Fatal("损坏的数据库私钥必须报错（不能静默生成新密钥，否则已发布节点的公钥会失效）")
	}

	// 旧版本文件损坏同样必须报错
	dir := t.TempDir()
	path := filepath.Join(dir, "agent", updateSigningKeyFile)
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not-base64"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrCreateAgentUpdateSigner(context.Background(), dir, nil); err == nil {
		t.Fatal("损坏的私钥文件必须报错")
	}
}

func TestNilSignerIsSafe(t *testing.T) {
	var s *AgentUpdateSigner
	if got := s.SignChecksum("abc"); got != "" {
		t.Errorf("nil signer 应返回空签名，得到 %q", got)
	}
	if got := s.PublicKeyPEM(); got != nil {
		t.Errorf("nil signer 应返回 nil 公钥")
	}
}
