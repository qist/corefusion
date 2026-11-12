package proxy

import (
	"strings"
	"testing"

	"corefusion/internal/model/entity"
)

// 反代模板必须透传 X-Forwarded-Host：订阅 provider URL 按「本次请求实际使用的域名」
// 选端点（多域名场景），缺这个头时面板只能回落到第一个启用端点，表现为
// 「换了订阅域名，provider 里还是旧域名」。
func TestForwardProxyTemplatesIncludeXForwardedHost(t *testing.T) {
	cm := &CaddyManager{}
	for _, tpl := range []string{"websocket", "grpc", "xhttp", "httpupgrade", "panel_forward", "subscription_forward", "portal_forward", ""} {
		got := cm.proxyHeaders(&entity.CaddySite{Template: tpl})
		if !strings.Contains(got, "header_up X-Forwarded-Host {host}") {
			t.Errorf("caddy template=%q 缺少 X-Forwarded-Host：\n%s", tpl, got)
		}
	}

	var sb strings.Builder
	writeNginxProxyHeaders(&sb)
	out := sb.String()
	for _, want := range []string{
		"proxy_set_header Host $host;",
		"proxy_set_header X-Forwarded-Host $host;",
		"proxy_set_header X-Forwarded-Proto $scheme;",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("nginx 反代头缺少 %q：\n%s", want, out)
		}
	}
}
