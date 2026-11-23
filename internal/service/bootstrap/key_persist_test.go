package bootstrap

import (
	"context"
	"os"
	"strings"
	"testing"

	seccrypto "corefusion/internal/security/crypto"
)

// 向导第 3 步展示/下载的密钥必须与 Complete 落盘的一致。
//
// 以前的实现里 `d.AutoGenerateSecrets` 默认 true，Complete 会再生成一把新密钥：
// 用户在配置清单里下载的 .env.secret 与最终 .env.secret 不同，按下载文件备份后
// 还原时 CA 私钥/DB DSN/第三方密钥全部解不开。
func TestCompleteKeepsDraftSecretKey(t *testing.T) {
	chdir(t)
	dataDir := t.TempDir()
	d := NewDraft(dataDir)
	d.DataDir = dataDir
	d.ListenPort = "18081"
	d.AdminUsername = "cf_key_test"
	d.AdminPassword = "StrongPass1!"
	d.ProxyType = ProxyNone
	d.SubscriptionPort = "0"
	d.PortalPort = "0"

	want := strings.TrimSpace(d.SecretEncryptionKey)
	if len(want) != 64 {
		t.Fatalf("NewDraft key = %q (len %d), want 64 hex", want, len(want))
	}

	if err := Complete(context.Background(), dataDir, d); err != nil {
		t.Fatalf("complete: %v", err)
	}

	got := strings.TrimSpace(seccrypto.ReadSecretEnvValue(seccrypto.SecretEncryptionKeyName))
	if got != want {
		t.Fatalf(".env.secret 密钥 = %q, 与向导展示的 %q 不一致（下载的备份将无法解密）", got, want)
	}
	if b, err := os.ReadFile(".env.secret"); err != nil || !strings.Contains(string(b), "CF_SECRET_ENCRYPTION_KEY="+want) {
		t.Fatalf(".env.secret 内容异常: %v %q", err, string(b))
	}
}
