package main

import "testing"

// 安全审计 v7 M-7：面板下发的日志路径只能是日志形态，不能读 /etc/shadow 之类。
func TestIsLogFileName(t *testing.T) {
	good := []string{
		"/var/log/nginx/access.log",
		"/var/log/nginx/error.log.1",
		"logs/xray-access.log",
		"/opt/corefusion/data/logs/caddy.log",
		"data/logs/run.out",
		"logs/xray.err",
	}
	for _, p := range good {
		if !isLogFileName(p) {
			t.Errorf("isLogFileName(%q) = false, want true", p)
		}
	}
	bad := []string{
		"/etc/shadow", "/etc/passwd", "/root/.ssh/authorized_keys",
		"data/config.json", "logs/", "", "..", "/etc/logrotate.d/x",
	}
	for _, p := range bad {
		if isLogFileName(p) {
			t.Errorf("isLogFileName(%q) = true, want false", p)
		}
	}
}
