package agent

import (
	"testing"
)

// 安全审计 v7 L-9：host 会拼进 bash 脚本的双引号变量，必须只允许主机名/IP 字符。
func TestInstallScriptRejectsShellInjectionHost(t *testing.T) {
	malicious := []string{
		`evil"; touch /tmp/pwn; echo "`,
		`evil$(id)`,
		`evil host`,
		"evil\nhost",
		`evil'`,
	}
	for _, host := range malicious {
		if _, err := NewAgentInstallScript(host, 8080).Generate(); err == nil {
			t.Errorf("host %q 未被拒绝", host)
		}
	}
}

func TestInstallScriptAcceptsNormalHosts(t *testing.T) {
	hosts := []string{"panel.example.com", "127.0.0.1", "2001:db8::1", "cdn-sub.example.org"}
	for _, host := range hosts {
		if err := validateInstallHost("面板地址", host); err != nil {
			t.Errorf("validateInstallHost(%q) = %v, want nil", host, err)
		}
	}
	if _, err := NewAgentInstallScript("panel.example.com", 8080).Generate(); err != nil {
		t.Fatalf("正常 host 生成脚本失败: %v", err)
	}
	// IPv6 字面量允许带方括号
	if err := validateInstallHost("面板地址", "[2001:db8::1]"); err != nil {
		t.Errorf("IPv6 bracket host 被拒绝: %v", err)
	}
}
