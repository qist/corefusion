package proxy

import (
	"strings"
	"testing"

	"corefusion/internal/model/entity"
)

func TestParseCaddyfileSites_Comprehensive(t *testing.T) {
	content := `
{
	email admin@example.com
}

example.com, www.example.com {
	tls /etc/ssl/example.pem /etc/ssl/example.key
	encode gzip
	header {
		X-Custom "1"
	}

	reverse_proxy 127.0.0.1:9000 {
		header_up Host {host}
		transport http {
			tls
			tls_server_name backend.local
		}
	}

	handle_path /api/* {
		reverse_proxy 127.0.0.1:9001
	}

	handle /ws/* {
		reverse_proxy 127.0.0.1:9002
	}

	redir /old https://new.example.com permanent
}

code.example.com {
	respond 204
}

static.example.com {
	root * /var/www/html
	file_server
}

echo.example.com {
	respond "{remote_host}" 200
}

old.example.com {
	redir https://target.example.com{uri}
}
`
	res := ParseCaddyfileSites(content)
	if len(res.Warnings) > 0 {
		t.Fatalf("不应有告警: %v", res.Warnings)
	}
	sites := res.Sites
	if len(sites) != 8 {
		t.Fatalf("期望 8 条站点，得到 %d: %+v", len(sites), sites)
	}
	byPath := map[string]entity.CaddySite{}
	for _, s := range sites {
		byPath[s.Domain+"|"+s.Path] = s
	}

	main := byPath["example.com, www.example.com|/"]
	if main.Template != "http" || main.Upstream != "127.0.0.1:9000" || !main.UpstreamTLS {
		t.Errorf("主站点错误: %+v", main)
	}
	if main.CertPath != "/etc/ssl/example.pem" || main.KeyPath != "/etc/ssl/example.key" {
		t.Errorf("证书路径错误: %+v", main)
	}
	if main.RedirectHTTPS == nil || !*main.RedirectHTTPS {
		t.Errorf("RedirectHTTPS 应为 true: %+v", main)
	}

	api := byPath["example.com, www.example.com|/api"]
	if api.Template != "http" || api.Upstream != "127.0.0.1:9001" || api.BackendPath != "/" {
		t.Errorf("handle_path 应去前缀 (BackendPath=/): %+v", api)
	}
	ws := byPath["example.com, www.example.com|/ws"]
	if ws.BackendPath != "/ws" {
		t.Errorf("handle 应保路径 (BackendPath=/ws): %+v", ws)
	}

	rd := byPath["example.com, www.example.com|/old"]
	if rd == (entity.CaddySite{}) {
		t.Fatalf("/old 路径的 redir 未导入")
	}
	if rd.Template != "redirect" || rd.BackendPath != "https://new.example.com" {
		t.Errorf("redir 目标错误: %+v", rd)
	}

	codeSite := byPath["code.example.com|/"]
	if codeSite.Template != "return_code" || codeSite.BackendPath != "204" {
		t.Errorf("respond 204 应识别为 return_code: %+v", codeSite)
	}

	st := byPath["static.example.com|/"]
	if st.Template != "static" || st.Upstream != "/var/www/html" {
		t.Errorf("static 站点错误: %+v", st)
	}

	echo := byPath["echo.example.com|/"]
	if echo.Template != "echo_ip" {
		t.Errorf("respond {remote_host} 应识别为 echo_ip: %+v", echo)
	}
}

func TestParseCaddyfileSites_NamedMatcherAndGlobal(t *testing.T) {
	content := `
@app {
	path /app/*
}

app.example.com {
	reverse_proxy @app 127.0.0.1:7001
	reverse_proxy 127.0.0.1:7000
}

:8443 {
	proxy 127.0.0.1:9000
}

plain.example.com:8443 {
	reverse_proxy 127.0.0.1:8000
}

http://plain-tls.example.com {
	reverse_proxy 127.0.0.1:8001
}
`
	res := ParseCaddyfileSites(content)
	joined := strings.Join(res.Warnings, "\n")
	if !strings.Contains(joined, "8443") {
		t.Errorf("非 80/443 端口应产生告警: %v", res.Warnings)
	}
	var l4Warn bool
	for _, w := range res.Warnings {
		if strings.Contains(w, "无主机名") {
			l4Warn = true
		}
	}
	if !l4Warn {
		t.Errorf(":8443 块应给出 L4 告警: %v", res.Warnings)
	}
	if len(res.Sites) != 4 {
		t.Fatalf("期望 4 条站点，得到 %d: %+v", len(res.Sites), res.Sites)
	}
	for _, s := range res.Sites {
		if s.Domain == "app.example.com" && s.Path == "/app" && s.Upstream != "127.0.0.1:7001" {
			t.Errorf("命名匹配器站点错误: %+v", s)
		}
		if s.Domain == "app.example.com" && s.Path == "/" && s.Upstream != "127.0.0.1:7000" {
			t.Errorf("站点级 reverse_proxy 错误: %+v", s)
		}
		if s.Domain == "http://plain-tls.example.com" {
			t.Errorf("http:// 前缀应被剥掉: %+v", s)
		}
		if s.Domain == "plain-tls.example.com" && (*s.RedirectHTTPS) {
			t.Errorf("http:// 站点不应强制跳 https: %+v", s)
		}
	}
}

func TestParseCaddyfileSites_Empty(t *testing.T) {
	res := ParseCaddyfileSites("   \n# comment only\n")
	if len(res.Sites) != 0 {
		t.Errorf("空配置不应有站点: %+v", res.Sites)
	}
	if len(res.Warnings) == 0 {
		t.Errorf("空配置应有告警")
	}
}

func TestParseCaddyfileSites_UnsupportedRespond(t *testing.T) {
	content := `
x.example.com {
	respond "hello world" 200
}
`
	res := ParseCaddyfileSites(content)
	if len(res.Sites) != 0 {
		t.Errorf("任意文本 respond 应跳过: %+v", res.Sites)
	}
}

func TestParseCaddyfileSites_Dedupe(t *testing.T) {
	content := `
d.example.com {
	reverse_proxy 127.0.0.1:1
	handle /a/* {
		reverse_proxy 127.0.0.1:2
	}
	handle /a/* {
		reverse_proxy 127.0.0.1:3
	}
}
`
	res := ParseCaddyfileSites(content)
	if len(res.Sites) != 2 {
		t.Fatalf("重复路径应去重为 2 条，得到 %d", len(res.Sites))
	}
	if len(res.Warnings) == 0 {
		t.Errorf("去重应产生告警")
	}
}
