package proxy

import (
	"strings"
	"testing"

	"corefusion/internal/model/entity"
)

// template=raw 的站点必须原样写入、且不生成 upstream 块（迁移存量复杂站点的逃生口）。
func TestNginxConfigRawServerPassthrough(t *testing.T) {
	raw := `server {
    listen 127.0.0.1:443 ssl proxy_protocol;
    server_name legacy.example.com;
    ssl_certificate /apps/nginx/sslkey/legacy/fullchain.crt;
    location /grpc {
        if ($content_type !~ "application/grpc") { return 404; }
        grpc_pass grpc://legacy-grpc;
    }
}`
	sites := []entity.NginxSite{
		{ServerID: 2, Domain: "legacy.example.com", Template: "raw", RawServer: raw, Enabled: true},
	}
	cfg := generateFor(t, newNginxTestManager(t, sites, `{}`))
	for _, want := range []string{
		"# raw server: legacy.example.com",
		"listen 127.0.0.1:443 ssl proxy_protocol;",
		`if ($content_type !~ "application/grpc") { return 404; }`,
		"grpc_pass grpc://legacy-grpc;",
		"ssl_certificate /apps/nginx/sslkey/legacy/fullchain.crt;",
	} {
		if !strings.Contains(cfg, want) {
			t.Fatalf("raw server 内容缺失 %q：\n%s", want, cfg)
		}
	}
	if strings.Contains(cfg, "upstream backend_") {
		t.Fatalf("raw 站点不应生成 upstream 块：\n%s", cfg)
	}
}

// settings.raw_stream 非空时必须整段接管 stream 块，不再生成 SNI/L4 自动分流。
func TestNginxConfigRawStreamOverridesSNI(t *testing.T) {
	sites := []entity.NginxSite{
		{ServerID: 2, Domain: "a.example.com", Template: "sni", Path: "443", SNITarget: "127.0.0.1:9001", Enabled: true},
	}
	rawStream := `map $ssl_preread_protocol $stream_map_guarded {
    ""      direct_http;
    default $stream_map;
}
upstream direct_http { server 127.0.0.1:8444; }
server {
    listen 127.0.0.1:443;
    proxy_pass $stream_map_guarded;
    ssl_preread on;
}`
	cfg := generateFor(t, newNginxTestManager(t, sites, `{"raw_stream": `+quoteJSON(rawStream)+`}`))
	if !strings.Contains(cfg, "map $ssl_preread_protocol $stream_map_guarded") {
		t.Fatalf("raw_stream 未写入：\n%s", cfg)
	}
	if strings.Contains(cfg, "map $ssl_preread_server_name") {
		t.Fatalf("raw_stream 生效时不应再生成 SNI map：\n%s", cfg)
	}
}

func quoteJSON(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
