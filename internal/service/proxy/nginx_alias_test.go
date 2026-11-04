package proxy

import (
	"strings"
	"testing"

	"corefusion/internal/model/entity"
)

// location_config 里已有 alias 时，static 模板不能再补 root：
// alias 是前缀替换，root 是路径拼接，两者同时出现会改变实际映射目录。
func TestNginxConfigStaticAliasNotOverriddenByRoot(t *testing.T) {
	sites := []entity.NginxSite{
		{
			ServerID: 2, Domain: "a.example.com", Template: "static", Path: "/664c",
			Upstream:       "",
			LocationConfig: "alias /apps/nginx/html/664c;\nauth_basic \"x\";\nauth_basic_user_file /apps/nginx/key/passwd;",
			Enabled:        true,
		},
	}
	cfg := generateFor(t, newNginxTestManager(t, sites, `{}`))
	if !strings.Contains(cfg, "alias /apps/nginx/html/664c;") {
		t.Fatalf("alias 丢失：\n%s", cfg)
	}
	if strings.Contains(cfg, "root /var/www;") {
		t.Fatalf("不应补默认 root /var/www：\n%s", cfg)
	}
	if !strings.Contains(cfg, "auth_basic_user_file /apps/nginx/key/passwd;") {
		t.Fatalf("授权文件路径丢失：\n%s", cfg)
	}
}
