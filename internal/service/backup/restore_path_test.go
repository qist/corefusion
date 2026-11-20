package backup

import "testing"

// 安全审计 v7 M-5：恢复只允许数据库文件，不能覆盖配置/证书/二进制。
func TestIsRestorableBackupPath(t *testing.T) {
	allowed := []string{
		"sqlite/app.db",
		"./sqlite/app.db",
		"db_dump/corefusion.sql",
		"db_dump/app.SQL",
	}
	for _, p := range allowed {
		if !isRestorableBackupPath(p) {
			t.Errorf("isRestorableBackupPath(%q) = false, want true", p)
		}
	}

	denied := []string{
		"proxy/nginx.conf",
		"certs/example.com/key.pem",
		"binaries/active/xray/xray-linux-amd64",
		"db_dump/../../etc/cron.d/x.sql",
		"db_dump/sub/a.sql",
		"sqlite/other.db",
		"app.db",
		"data/sqlite/app.db",
		"db_dump/x.sql.bak",
	}
	for _, p := range denied {
		if isRestorableBackupPath(p) {
			t.Errorf("isRestorableBackupPath(%q) = true, want false", p)
		}
	}
}
