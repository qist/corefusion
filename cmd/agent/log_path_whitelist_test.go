package main

import (
	"path/filepath"
	"testing"
)

// 安全审计 v7 M-7：日志路径除「像日志文件」外，还必须落在白名单目录。
func TestResolveRelPathWhitelist(t *testing.T) {
	dataDir := "/opt/corefusion-agent/data"
	a := &AgentApp{dataDir: dataDir}

	allowed := map[string]string{
		"logs/error.log":            filepath.Join(dataDir, "logs/error.log"),
		"/var/log/nginx/access.log": "/var/log/nginx/access.log",
		"/www/wwwlogs/site.log":     "/www/wwwlogs/site.log",
		"/var/log/xray/error.log.1": "/var/log/xray/error.log.1",
	}
	for in, want := range allowed {
		if got := a.resolveRelPath(in); got != want {
			t.Errorf("resolveRelPath(%q) = %q, want %q", in, got, want)
		}
	}

	denied := []string{
		"/etc/passwd.log",
		"/root/.ssh/authorized_keys.log",
		"/var/log/../../etc/shadow.log",
		"/etc/shadow",
		"../../etc/shadow.log",
		"/tmp/x.log",
	}
	for _, in := range denied {
		if got := a.resolveRelPath(in); got != "" {
			t.Errorf("resolveRelPath(%q) = %q, want 拒绝", in, got)
		}
	}
}
