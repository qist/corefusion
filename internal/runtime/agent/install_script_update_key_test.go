package agent

import (
	"strings"
	"testing"
)

const testUpdatePubKeyPEM = `-----BEGIN PUBLIC KEY-----
MCowBQYDK2VwAyEA1234567890abcdefghijklmnopqrstuvwxyzABCDEFG=
-----END PUBLIC KEY-----
`

// 安全审计 v7 L-6：安装脚本必须把面板更新公钥写进节点 config.yaml，
// 否则 Agent 只能退回「仅校验 SHA-256」。
func TestInstallScriptWritesUpdatePublicKey(t *testing.T) {
	script, err := NewAgentInstallScript("panel.example.com", 8080).
		WithUpdatePublicKey([]byte(testUpdatePubKeyPEM)).
		Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.Contains(script, "UPDATE_PUBKEY_PEM=") {
		t.Fatal("脚本缺少 UPDATE_PUBKEY_PEM 变量")
	}
	if !strings.Contains(script, `printf '\nupdate:\n  public_key: |\n'`) {
		t.Fatal("脚本未写入 update.public_key 配置块")
	}
	// 块标量缩进（4 空格）
	if !strings.Contains(script, `sed 's/^/    /'`) {
		t.Fatal("公钥未按 YAML 块标量缩进写入")
	}
}

func TestInstallScriptOmitsUpdateBlockWithoutKey(t *testing.T) {
	script, err := NewAgentInstallScript("panel.example.com", 8080).Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if strings.Contains(script, "update:\n  public_key: |") {
		t.Fatal("没有公钥时不应写 update 配置块")
	}
}
