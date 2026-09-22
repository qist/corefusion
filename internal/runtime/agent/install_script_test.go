package agent

import (
	"strings"
	"testing"
)

// 安全审计 v5 M6：安装脚本必须**自动**把面板 CA 交给 Agent（引导阶段才能校验面板身份），
// 并且把含 bootstrap token 的 config.yaml 收紧到 0600 —— 全程不需要人工填写/操作。
func TestInstallScriptShipsPanelCAAndProtectsConfig(t *testing.T) {
	caPEM := "-----BEGIN CERTIFICATE-----\nMIIBfakefakefake\n-----END CERTIFICATE-----"
	script, err := NewAgentInstallScript("panel.example.com", 8080).
		WithScheme("https").
		WithTLS(true).
		WithBootstrapToken("bootstrap-token-xyz").
		WithServerID(7).
		WithCACertPEM([]byte(caPEM)).
		Generate()
	if err != nil {
		t.Fatalf("生成安装脚本失败：%v", err)
	}

	if !strings.Contains(script, caPEM) {
		t.Error("安装脚本里必须带上面板 CA 证书（否则 Agent 引导阶段无法校验面板身份）")
	}
	if !strings.Contains(script, `"$INSTALL_DIR/certs/ca-cert.pem"`) {
		t.Error("CA 证书要落到 Agent 的 certs/ca-cert.pem（正是引导与心跳读取的路径）")
	}
	if !strings.Contains(script, `chmod 600 "$CONFIG_FILE"`) {
		t.Error("config.yaml 里有 bootstrap token，必须 chmod 600")
	}
	if !strings.Contains(script, "bootstrap-token-xyz") {
		t.Error("脚本应包含 bootstrap token（由面板按需生成，属正常流程）")
	}
}

// 没提供 CA（理论上走不到，防御性）时脚本仍要能生成，只是不带 CA 段落。
func TestInstallScriptWithoutCAStillGenerates(t *testing.T) {
	script, err := NewAgentInstallScript("panel.example.com", 8080).
		WithBootstrapToken("t").
		WithServerID(1).
		Generate()
	if err != nil {
		t.Fatalf("生成失败：%v", err)
	}
	if strings.Contains(script, "BEGIN CERTIFICATE") {
		t.Error("没提供 CA 时不应凭空出现证书内容")
	}
	if !strings.Contains(script, `chmod 600 "$CONFIG_FILE"`) {
		t.Error("config 权限收紧与 CA 无关，必须始终生效")
	}
}
