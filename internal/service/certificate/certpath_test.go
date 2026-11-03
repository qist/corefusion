package certificate

import "testing"

// TestValidateCertFilePath 锁定安全审计 v6 H-5：
// 证书路径来自 DB/请求，`..` 必须被拒（允许管理员配置绝对路径）。
func TestValidateCertFilePath(t *testing.T) {
	ok := []string{"", "certs/a/cert.pem", "/etc/nginx/certs/a.pem"}
	for _, p := range ok {
		if err := validateCertFilePath("cert_path", p); err != nil {
			t.Errorf("legitimate path %q rejected: %v", p, err)
		}
	}
	bad := []string{"../../etc/shadow", "certs/../../../etc/shadow", "/etc/../etc/shadow"}
	for _, p := range bad {
		if err := validateCertFilePath("cert_path", p); err == nil {
			t.Errorf("traversal path %q must be rejected", p)
		}
	}
}
