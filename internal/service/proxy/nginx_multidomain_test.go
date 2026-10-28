package proxy

import (
	"strings"
	"testing"

	"corefusion/internal/model/entity"
)

// TestSplitDomains 锁定多域名解析规则：分隔符、大小写、去重、去 scheme/路径。
func TestSplitDomains(t *testing.T) {
	got := SplitDomains(" A.example.com,\nhttps://b.example.com/path; a.example.com \t c.example.com. ")
	want := []string{"a.example.com", "b.example.com", "c.example.com"}
	if len(got) != len(want) {
		t.Fatalf("SplitDomains = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("SplitDomains[%d] = %q, want %q (all=%v)", i, got[i], want[i], got)
		}
	}
	if k := DomainGroupKey(got); k != "a.example.com,b.example.com,c.example.com" {
		t.Fatalf("DomainGroupKey = %q", k)
	}
	if n := NormalizeDomainField(" b.com , a.com "); n != "b.com, a.com" {
		t.Fatalf("NormalizeDomainField = %q", n)
	}
}

// TestNginxConfigMultiDomain server_name 必须一次写出整组域名（现网同一 server 块绑 4 个域名）。
func TestNginxConfigMultiDomain(t *testing.T) {
	sites := []entity.NginxSite{
		{ServerID: 2, Domain: "a.example.com, b.example.com\nc.example.com", ListenPort: "80", Enabled: true, Path: "/", Upstream: "127.0.0.1:8080", Template: "http"},
	}
	cfg := generateFor(t, newNginxTestManager(t, sites, `{}`))
	if !strings.Contains(cfg, "server_name a.example.com b.example.com c.example.com;") {
		t.Fatalf("多域名未生成到同一个 server_name：\n%s", cfg)
	}
	if n := strings.Count(cfg, "server_name a.example.com"); n != 1 {
		t.Fatalf("同一域名组应只有一个 server 块，实际 server_name 出现 %d 次：\n%s", n, cfg)
	}
}

// TestNginxConfigMultiDomainGrouping 书写顺序不同的同一组域名必须合并到一个 server 块。
func TestNginxConfigMultiDomainGrouping(t *testing.T) {
	sites := []entity.NginxSite{
		{ServerID: 2, Domain: "a.example.com,b.example.com", ListenPort: "80", Enabled: true, Path: "/x", Upstream: "127.0.0.1:8080"},
		{ServerID: 2, Domain: "b.example.com, a.example.com", ListenPort: "80", Enabled: true, Path: "/y", Upstream: "127.0.0.1:8080"},
	}
	cfg := generateFor(t, newNginxTestManager(t, sites, `{}`))
	if n := strings.Count(cfg, "server_name a.example.com b.example.com;"); n != 1 {
		t.Fatalf("同一域名组应合并为一个 server 块，实际 %d 个：\n%s", n, cfg)
	}
	for _, loc := range []string{"location /x", "location /y"} {
		if !strings.Contains(cfg, loc) {
			t.Fatalf("合并后缺少 %s：\n%s", loc, cfg)
		}
	}
}

// TestNginxConfigEchoAndRedirectTemplates 新增模板：回显客户端 IP / 301 跳转。
func TestNginxConfigEchoAndRedirectTemplates(t *testing.T) {
	sites := []entity.NginxSite{
		{ServerID: 2, Domain: "echo.example.com", ListenPort: "80", Enabled: true, Path: "/", Template: "echo_ip"},
		{ServerID: 2, Domain: "go.example.com", ListenPort: "80", Enabled: true, Path: "/", Template: "redirect", BackendPath: "https://target.example.com"},
	}
	cfg := generateFor(t, newNginxTestManager(t, sites, `{}`))
	if !strings.Contains(cfg, `return 200 "$clientRealIp\n";`) {
		t.Fatalf("echo_ip 模板未生成回显：\n%s", cfg)
	}
	if !strings.Contains(cfg, "return 301 https://target.example.com$request_uri;") {
		t.Fatalf("redirect 模板未生成跳转：\n%s", cfg)
	}
}

// TestNginxConfigDirectUpstream 支持现网那种“上游是命名 upstream/带 scheme 的 URL”写法：
// 直接 proxy_pass http://php，而不是生成非法的 upstream backend_http_php { server http://php; }。
func TestNginxConfigDirectUpstream(t *testing.T) {
	sites := []entity.NginxSite{
		{ServerID: 2, Domain: "u.example.com", ListenPort: "80", Enabled: true, Path: "/", Upstream: "http://php"},
	}
	cfg := generateFor(t, newNginxTestManager(t, sites, `{}`))
	if strings.Contains(cfg, "upstream backend_http") {
		t.Fatalf("带 scheme 的上游不应生成 upstream 块：\n%s", cfg)
	}
	if !strings.Contains(cfg, "proxy_pass http://php;") {
		t.Fatalf("带 scheme 的上游应直接 proxy_pass：\n%s", cfg)
	}
}

// TestNginxConfigReturnCodeTemplate 探测页模板：直接返回状态码（默认 204，非法回退 204）。
func TestNginxConfigReturnCodeTemplate(t *testing.T) {
	sites := []entity.NginxSite{
		{ServerID: 2, Domain: "probe.example.com", ListenPort: "80", Enabled: true, Path: "/", Template: "return_code", BackendPath: "204"},
		{ServerID: 2, Domain: "bad.example.com", ListenPort: "80", Enabled: true, Path: "/", Template: "return_code", BackendPath: "9999"},
	}
	cfg := generateFor(t, newNginxTestManager(t, sites, `{}`))
	if !strings.Contains(cfg, "return 204;") {
		t.Fatalf("return_code 未生成 return 204：\n%s", cfg)
	}
	if strings.Contains(cfg, "return 9999;") {
		t.Fatalf("非法状态码应回退 204：\n%s", cfg)
	}
}
