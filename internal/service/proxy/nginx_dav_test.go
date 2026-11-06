package proxy

import (
	"strings"
	"testing"

	"corefusion/internal/model/entity"
)

// WebDAV 目录不能带 try_files：RFC4918 的 PUT/MKCOL/DELETE 会在 precontent 阶段
// 被 try_files 判成 404，再叠加 server 级 error_page 404=200 就变成
// “返回 200 但文件没落盘”，客户端随后还会 MOVE 覆盖掉原文件。
func TestNginxConfigDAVLocationHasNoTryFiles(t *testing.T) {
	sites := []entity.NginxSite{
		{
			ServerID: 2, Domain: "dav.example.com", Template: "static", Path: "/dav",
			LocationConfig: "root /apps/nginx/html;\n" +
				"autoindex on;\n" +
				"dav_methods PUT DELETE MKCOL COPY MOVE;\n" +
				"dav_ext_methods PROPFIND OPTIONS;\n" +
				"create_full_put_path on;\n" +
				"dav_access user:rw group:r all:r;",
			Enabled: true,
		},
	}
	cfg := generateFor(t, newNginxTestManager(t, sites, `{}`))

	start := strings.Index(cfg, "location /dav {")
	if start < 0 {
		t.Fatalf("缺少 /dav 段：\n%s", cfg)
	}
	end := strings.Index(cfg[start:], "\n        }")
	if end < 0 {
		t.Fatalf("无法定位 /dav 段结尾：\n%s", cfg)
	}
	block := cfg[start : start+end]

	if strings.Contains(block, "try_files") {
		t.Fatalf("DAV 段不应出现 try_files：\n%s", block)
	}
	if strings.Contains(block, "index index.html") {
		t.Fatalf("DAV 段不应注入默认 index：\n%s", block)
	}
	if !strings.Contains(block, "dav_methods PUT DELETE MKCOL COPY MOVE;") {
		t.Fatalf("dav_methods 丢失：\n%s", block)
	}
	// root 由用户配置决定，不能被生成器改写
	if !strings.Contains(block, "root /apps/nginx/html;") {
		t.Fatalf("用户配置的 root 被改写：\n%s", block)
	}
}

// 普通静态目录只补 index，不补 try_files：
// try_files 是 SPA（前端路由）才需要的东西，本地静态/下载/上传目录都不需要。
func TestNginxConfigPlainStaticHasNoTryFiles(t *testing.T) {
	sites := []entity.NginxSite{
		{ServerID: 2, Domain: "files.example.com", Template: "static", Path: "/dl",
			Upstream: "/srv/downloads", Enabled: true},
	}
	cfg := generateFor(t, newNginxTestManager(t, sites, `{}`))
	if strings.Contains(cfg, "try_files") {
		t.Fatalf("普通静态目录不应默认生成 try_files：\n%s", cfg)
	}
	if !strings.Contains(cfg, "index index.html index.htm;") {
		t.Fatalf("静态目录应保留默认 index：\n%s", cfg)
	}
	if !strings.Contains(cfg, "root /srv/downloads;") {
		t.Fatalf("静态目录 root 丢失：\n%s", cfg)
	}
}

// 用户自己写了 index / try_files 时（例如 SPA 的 fallback），生成器必须原样保留。
func TestNginxConfigStaticRespectsCustomIndexAndTryFiles(t *testing.T) {
	sites := []entity.NginxSite{
		{
			ServerID: 2, Domain: "spa.example.com", Template: "static", Path: "/",
			LocationConfig: "root /usr/share/nginx/html;\n" +
				"index index.php index.html;\n" +
				"try_files $uri $uri/ /index.html;",
			Enabled: true,
		},
	}
	cfg := generateFor(t, newNginxTestManager(t, sites, `{}`))

	if strings.Contains(cfg, "index index.html index.htm;") {
		t.Fatalf("不应再补默认 index：\n%s", cfg)
	}
	if strings.Contains(cfg, "try_files $uri $uri/ =404;") {
		t.Fatalf("不应再补默认 try_files：\n%s", cfg)
	}
	if strings.Count(cfg, "try_files $uri $uri/ /index.html;") != 1 {
		t.Fatalf("用户自定义 try_files 应原样保留一次：\n%s", cfg)
	}
}
