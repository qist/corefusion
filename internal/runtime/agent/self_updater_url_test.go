package agent

import "testing"

// 安全审计 v7 L-6：更新包只能来自面板自身或 GitHub Release 域名。
func TestValidateUpdateURL(t *testing.T) {
	u := NewSelfUpdater(t.TempDir(), nil)
	u.SetAllowedHosts("panel.example.com", "203.0.113.10", "")

	allowed := []string{
		"https://panel.example.com/api/v1/agent/download?os=linux&arch=amd64",
		"http://203.0.113.10:8080/api/v1/agent/download?os=linux&arch=amd64",
		"https://github.com/qist/corefusion/releases/download/v1/corefusion-agent-linux-amd64",
		"https://objects.githubusercontent.com/abc/def",
		"http://127.0.0.1:12345/agent", // 本地测试
	}
	for _, raw := range allowed {
		if err := u.validateUpdateURL(raw); err != nil {
			t.Errorf("validateUpdateURL(%q) = %v, want nil", raw, err)
		}
	}

	denied := []string{
		"https://evil.example.net/corefusion-agent",
		"https://panel.example.com.evil.net/agent",
		"ftp://panel.example.com/agent",
		"http://github.com/qist/corefusion/releases/download/v1/x", // GitHub 必须 https
		"file:///etc/passwd",
		"panel.example.com/agent", // 无 scheme
		"",
	}
	for _, raw := range denied {
		if err := u.validateUpdateURL(raw); err == nil {
			t.Errorf("validateUpdateURL(%q) = nil, want error", raw)
		}
	}
}
