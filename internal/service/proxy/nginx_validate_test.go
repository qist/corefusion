package proxy

import (
	"strings"
	"testing"

	"corefusion/internal/model/entity"
)

// 安全审计 v7 M-6：多行自定义片段是正常用法，只有花括号逃逸/标量换行才应被拒绝。
func TestValidateNginxSettingsAllowsMultilineCustomConfig(t *testing.T) {
	s := &NginxSettings{
		HTTPListen: "80",
		CustomConfig: `client_body_temp_path /tmp/nginx-client;
proxy_cache_path /tmp/cache levels=1:2 keys_zone=zone1:10m;
map $http_upgrade $connection_upgrade {
    default upgrade;
    ''      close;
}`,
	}
	if err := ValidateNginxSettings(s); err != nil {
		t.Fatalf("合法的多行 custom_config 不应被拒绝：%v", err)
	}
}

func TestValidateNginxSettingsRejectsInjection(t *testing.T) {
	cases := []struct {
		name string
		s    *NginxSettings
	}{
		{"custom_config 逃逸作用域", &NginxSettings{CustomConfig: "}\nload_module /tmp/evil.so;"}},
		{"custom_config 缺右括号", &NginxSettings{CustomConfig: "map $a $b {\n default 1;"}},
		{"日志路径换行", &NginxSettings{ErrorLogPath: "/var/log/nginx/error.log;\nload_module /tmp/evil.so"}},
		{"标量带分号", &NginxSettings{StreamDefaultTarget: "127.0.0.1:443; include /tmp/x"}},
		{"监听地址带花括号", &NginxSettings{HTTPListen: "80 {"}},
	}
	for _, tc := range cases {
		if err := ValidateNginxSettings(tc.s); err == nil {
			t.Errorf("%s：应被拒绝", tc.name)
		}
	}
}

func TestValidateNginxSiteAllowsNormalConfig(t *testing.T) {
	site := &entity.NginxSite{
		Domain:   "example.com",
		Upstream: "http://127.0.0.1:8080",
		Path:     "/",
		ServerConfig: `client_max_body_size 64m;
add_header X-Frame-Options SAMEORIGIN always;`,
		LocationConfig: `proxy_set_header Host $host;
proxy_set_header X-Real-IP $remote_addr;
if ($request_method = OPTIONS) {
    return 204;
}`,
	}
	if err := ValidateNginxSite(site); err != nil {
		t.Fatalf("正常站点配置不应被拒绝：%v", err)
	}
}

func TestValidateNginxSiteRejectsInjection(t *testing.T) {
	cases := []struct {
		name string
		site *entity.NginxSite
	}{
		{"path 换行", &entity.NginxSite{Path: "/\nroot /etc;"}},
		{"upstream 带分号", &entity.NginxSite{Upstream: "http://127.0.0.1:8080; root /etc;"}},
		{"location_config 逃逸", &entity.NginxSite{LocationConfig: "}\nserver { listen 9999; }"}},
		{"server_config 缺括号", &entity.NginxSite{ServerConfig: "location /x {"}},
	}
	for _, tc := range cases {
		if err := ValidateNginxSite(tc.site); err == nil {
			t.Errorf("%s：应被拒绝", tc.name)
		}
	}
}

// 多行片段里的引号内花括号不应影响配平判断。
func TestValidateNginxSnippetQuotedBraces(t *testing.T) {
	if err := validateNginxSnippet("x", `add_header X-Note "a { b }";`); err != nil {
		t.Errorf("引号内的花括号不应被计入：%v", err)
	}
	if err := validateNginxSnippet("x", `add_header X-Note "a { b;`); err != nil {
		t.Errorf("引号未闭合时不应因花括号报错：%v", err)
	}
	if err := validateNginxSnippet("x", strings.Repeat("if ($a) { ", 3)+"}"+strings.Repeat(" }", 2)); err != nil {
		t.Errorf("配平的多层块应允许：%v", err)
	}
}

// 安全审计 v7 M-6：日志路径必须落在白名单目录（相对路径视为面板数据目录内）。
func TestValidateNginxLogPaths(t *testing.T) {
	good := []string{
		"", "logs/error.log", "logs/nginx/access.log",
		"/var/log/nginx/error.log", "/www/wwwlogs/site.log",
		"/dev/null", "/dev/stdout",
	}
	for _, p := range good {
		s := &NginxSettings{ErrorLogPath: p, AccessLogPath: p, StreamLogPath: p}
		if err := ValidateNginxSettings(s); err != nil {
			t.Errorf("日志路径 %q 应允许：%v", p, err)
		}
	}

	bad := []string{
		"/etc/cron.d/evil", "/root/.ssh/authorized_keys",
		"../outside.log", "logs/../../etc/x.log", "/tmp/x.log",
	}
	for _, p := range bad {
		s := &NginxSettings{ErrorLogPath: p}
		if err := ValidateNginxSettings(s); err == nil {
			t.Errorf("日志路径 %q 应被拒绝", p)
		}
	}
}
