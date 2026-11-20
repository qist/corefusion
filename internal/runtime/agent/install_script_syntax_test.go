package agent

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// 生成的安装脚本会直接在节点上以 root 执行：语法错误会导致安装失败，
// 且线上很难排查。这里用 bash -n 做一次静态语法校验。
func TestGeneratedInstallScriptIsValidBash(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	script, err := NewAgentInstallScript("panel.example.com", 8080).
		WithScheme("https").
		WithTLS(true).
		WithBootstrapToken("tok").
		WithServerID(11).
		WithAgentVersion("v0.13.0").
		WithCACertPEM([]byte("-----BEGIN CERTIFICATE-----\nfake\n-----END CERTIFICATE-----")).
		WithUpdatePublicKey([]byte("-----BEGIN PUBLIC KEY-----\nfake\n-----END PUBLIC KEY-----")).
		Generate()
	if err != nil {
		t.Fatalf("生成安装脚本失败：%v", err)
	}
	path := filepath.Join(t.TempDir(), "install.sh")
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatalf("写入脚本失败：%v", err)
	}
	if out, err := exec.Command("bash", "-n", path).CombinedOutput(); err != nil {
		t.Fatalf("安装脚本语法错误：%v\n%s", err, out)
	}
}
