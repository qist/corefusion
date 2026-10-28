package proxy

import (
	"strings"
	"testing"
)

const sampleNginxConf = `
# 现网形态：一个 server 多域名 + 直接上游 + 204 探测 + echo
http {
    upstream php {
        server 127.0.0.1:9000;
    }

    server {
        listen 443 ssl http2;
        server_name 6gzw5k.tycng.com *.ugigc.dpdns.org *.swsixtwo.top;
        ssl_certificate /etc/nginx/certs/tycng.pem;
        ssl_certificate_key /etc/nginx/certs/tycng.key;

        location / {
            proxy_pass http://php;
            proxy_set_header Host $host;
        }

        location = /generate_204 {
            return 204;
        }
    }

    server {
        listen 80;
        server_name probe.example.com;

        location / {
            default_type text/plain;
            return 200 "$clientRealIp\n";
        }
    }

    server {
        listen 443 ssl;
        server_name go.example.com;
        return 301 https://target.example.com$request_uri;
    }
}
`

func TestParseNginxSitesRealWorld(t *testing.T) {
	res := ParseNginxSites(sampleNginxConf)
	if len(res.Sites) != 4 {
		t.Fatalf("期望解析出 4 个站点（proxy/204/echo/redirect），实际 %d: %+v", len(res.Sites), res.Sites)
	}

	byKey := map[string]struct {
		template string
		upstream string
		backend  string
		port     string
	}{}
	for _, s := range res.Sites {
		byKey[s.Domain+"|"+s.Path] = struct {
			template string
			upstream string
			backend  string
			port     string
		}{s.Template, s.Upstream, s.BackendPath, s.ListenPort}
	}

	proxyKey := "6gzw5k.tycng.com, *.ugigc.dpdns.org, *.swsixtwo.top|/"
	got, ok := byKey[proxyKey]
	if !ok {
		t.Fatalf("多域名站点未合并，keys=%v", keysOf(byKey))
	}
	if got.template != "proxy" || got.upstream != "http://127.0.0.1:9000" || got.port != "443" {
		t.Fatalf("proxy 站点解析错误: %+v", got)
	}

	probe, ok := byKey["6gzw5k.tycng.com, *.ugigc.dpdns.org, *.swsixtwo.top|/generate_204"]
	if !ok || probe.template != "return_code" || probe.backend != "204" {
		t.Fatalf("204 探测 location 解析错误: %+v", probe)
	}

	echo, ok := byKey["probe.example.com|/"]
	if !ok || echo.template != "echo_ip" {
		t.Fatalf("echo 探测页解析错误: %+v", echo)
	}

	redirect, ok := byKey["go.example.com|/"]
	if !ok || redirect.template != "redirect" || redirect.backend != "https://target.example.com" {
		t.Fatalf("整站跳转解析错误: %+v", redirect)
	}

	// 证书/开关
	for _, s := range res.Sites {
		if s.Domain == "go.example.com" {
			continue
		}
		if strings.Contains(s.Domain, "6gzw5k") {
			if s.CertPath != "/etc/nginx/certs/tycng.pem" || s.KeyPath != "/etc/nginx/certs/tycng.key" {
				t.Fatalf("证书路径未解析: %+v", s)
			}
			if !s.EnableHttp2 {
				t.Fatalf("listen ... http2 应启用 http2")
			}
		}
	}
}

func TestParseNginxSitesSkipsCommentsAndDefaults(t *testing.T) {
	conf := `
server {
    listen [::]:8080;
    server_name a.example.com; # 行尾注释
    location /api/ {
        proxy_pass https://10.0.0.5:8443;
    }
}
`
	res := ParseNginxSites(conf)
	if len(res.Sites) != 1 {
		t.Fatalf("期望 1 个站点，实际 %d (%v)", len(res.Sites), res.Sites)
	}
	s := res.Sites[0]
	if s.ListenPort != "8080" || s.Path != "/api/" || s.Upstream != "https://10.0.0.5:8443" {
		t.Fatalf("解析错误: %+v", s)
	}
	if !s.EnableIpv6 {
		t.Fatalf("listen [::] 应启用 ipv6")
	}
}

func TestParseNginxSitesEmpty(t *testing.T) {
	res := ParseNginxSites("")
	if len(res.Sites) != 0 || len(res.Warnings) == 0 {
		t.Fatalf("空配置应无站点且有 warning: %+v", res)
	}
}

func TestParseNginxSitesInvalidReturnCodeFallsBack(t *testing.T) {
	conf := `
server {
    listen 80;
    server_name rc.example.com;
    location = /health {
        return abc;
    }
}
`
	res := ParseNginxSites(conf)
	if len(res.Sites) != 0 {
		t.Fatalf("非法 return 不应生成站点: %+v", res.Sites)
	}
}

func keysOf(m map[string]struct {
	template string
	upstream string
	backend  string
	port     string
}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// TestParseNginxSitesSkipsVariableProxyPass 含变量的 proxy_pass 依赖 map/set，
// 无法随站点一起迁移 → 跳过并给 warning（否则导入后 nginx 起不来）。
func TestParseNginxSitesSkipsVariableProxyPass(t *testing.T) {
	conf := `
server {
    listen 443 ssl;
    server_name v.example.com;
    set $website backend.internal;
    location / {
        proxy_pass https://$website;
    }
    location /ok/ {
        proxy_pass http://127.0.0.1:8080;
    }
}
`
	res := ParseNginxSites(conf)
	if len(res.Sites) != 1 || res.Sites[0].Path != "/ok/" {
		t.Fatalf("只应导入不含变量的 location: %+v", res.Sites)
	}
	found := false
	for _, w := range res.Warnings {
		if strings.Contains(w, "含变量") {
			found = true
		}
	}
	if !found {
		t.Fatalf("应给出含变量的 warning: %v", res.Warnings)
	}
}

// TestParseNginxSitesSkipsUndefinedUpstream 引用未定义 upstream 组的裸名字
// （http://vnStat）必须跳过，否则生成 proxy_pass 后 nginx 启动会 DNS 解析失败。
func TestParseNginxSitesSkipsUndefinedUpstream(t *testing.T) {
	conf := `
server {
    listen 80;
    server_name u.example.com;
    location /bad/ { proxy_pass http://vnStat; }
    location /ok/  { proxy_pass http://127.0.0.1:8080; }
    location /host/ { proxy_pass http://backend.internal; }
}
`
	res := ParseNginxSites(conf)
	if len(res.Sites) != 2 {
		t.Fatalf("应只导入 IP 与带点域名两种，实际 %d: %+v", len(res.Sites), res.Sites)
	}
	hasWarn := false
	for _, w := range res.Warnings {
		if strings.Contains(w, "未定义的 upstream") {
			hasWarn = true
		}
	}
	if !hasWarn {
		t.Fatalf("应提示未定义 upstream: %v", res.Warnings)
	}
}
