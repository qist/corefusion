package agent

import (
	"strings"
	"testing"
)

// 安全审计 v7 L-6：配置了公钥后，未签名/签名不符的更新必须在下载前就被拒绝。
func TestPerformUpdateRefusesUnsignedWhenPublicKeyConfigured(t *testing.T) {
	signer := newTestSigner(t)
	checksum := strings.Repeat("ab", 32)

	u := NewSelfUpdater(t.TempDir(), nil)
	u.SetUpdatePublicKey(signer.PublicKeyPEM())

	// 未签名
	err := u.PerformUpdate("v1", "http://127.0.0.1:1/agent", checksum, "")
	if err == nil || !strings.Contains(err.Error(), "signature") {
		t.Fatalf("未签名更新应被拒绝，got err=%v", err)
	}
	// 签名不符（用另一把密钥签）
	err = u.PerformUpdate("v1", "http://127.0.0.1:1/agent", checksum, newTestSigner(t).SignChecksum(checksum))
	if err == nil || !strings.Contains(err.Error(), "signature") {
		t.Fatalf("签名不符应被拒绝，got err=%v", err)
	}
}

// 没有公钥（历史手工安装）时保持旧行为：只校验 SHA-256，不因缺签名而拒绝。
func TestPerformUpdateLegacyWithoutPublicKey(t *testing.T) {
	u := NewSelfUpdater(t.TempDir(), nil)
	err := u.PerformUpdate("v1", "http://127.0.0.1:1/agent", strings.Repeat("ab", 32), "")
	if err == nil {
		t.Fatal("不可达地址应失败")
	}
	if strings.Contains(err.Error(), "signature") {
		t.Fatalf("未配置公钥时不应要求签名：%v", err)
	}
	if !strings.Contains(err.Error(), "download") {
		t.Fatalf("应走到下载阶段，got %v", err)
	}
}
