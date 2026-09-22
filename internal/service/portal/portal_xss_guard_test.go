package portal

import (
	"strings"
	"testing"
)

// 安全审计 v5 M7：门户用 v-html 渲染 markdown，而 marked 默认**放行原始 HTML** ✗ ——
// 工单消息是**用户可提交**的内容（客户 ↔ 客服双向）→ 客户写 script/img-onerror 就能在
// 客服或管理员的浏览器里执行（跨角色存储型 XSS）。修法：用户内容先转义再走 markdown。
//
// 门户模板是 Go 原始字符串，所以这些断言直接盯模板内容（改坏了立刻会被抓到）。
func TestPortalTicketMessageUsesUntrustedRenderer(t *testing.T) {
	// 模板分三块常量：HTML 标记在 defaultPortalHTML，JS（渲染器）在 defaultPortalJS
	tpl := defaultPortalHTML
	js := defaultPortalJS

	if !strings.Contains(js, "var renderMdUntrusted = function(md)") {
		t.Fatal("门户模板缺少 renderMdUntrusted（用户内容渲染器）")
	}
	if !strings.Contains(js, "var escapeHtmlForMd = function(s)") {
		t.Fatal("门户模板缺少 escapeHtmlForMd（HTML 转义器）")
	}

	// 转义器必须真的把 5 个危险字符转掉，否则等于没转义
	for _, pair := range []string{
		"/&/g, '&amp;'", "/</g, '&lt;'", "/>/g, '&gt;'", "/\"/g, '&quot;'", "/'/g, '&#39;'",
	} {
		if !strings.Contains(js, pair) {
			t.Errorf("转义器缺少规则 %s（转义不完整等于没转义）", pair)
		}
	}

	// 顺序不能反：必须**先转义再交给 marked**；反过来就没有任何保护了
	if !strings.Contains(js, "var safe = escapeHtmlForMd(md)") ||
		!strings.Contains(js, "marked.parse(safe") {
		t.Error("必须先 escapeHtmlForMd 再 marked.parse（顺序反了等于没防护）")
	}

	// 工单消息必须用不可信渲染器
	if !strings.Contains(tpl, "v-html=\"renderMdUntrusted(msg.content)\"") {
		t.Error("工单消息必须用 renderMdUntrusted 渲染（内容来自用户）")
	}
	if strings.Contains(tpl, "v-html=\"renderMd(msg.content)\"") {
		t.Error("工单消息不得直接用 renderMd（marked 放行原始 HTML → 跨角色存储型 XSS）")
	}

	// 管理员自撰内容（公告/文章）保持原样：它们等价于后台特权内容，收紧会打断自定义
	if !strings.Contains(tpl, "v-html=\"renderMd(articleDetail.content)\"") {
		t.Error("文章仍应使用 renderMd（管理员自撰，属于特权内容）")
	}

	// 工具集里也要导出，便于自定义模块对用户内容使用它
	if !strings.Contains(js, "renderMdUntrusted: renderMdUntrusted") {
		t.Error("应在导出的工具集里带上 renderMdUntrusted")
	}
}

// 门户页面的安全响应头：只设不会打断自定义 HTML/JS/CDN 的那几条。
func TestPortalSecurityHeaders(t *testing.T) {
	h := SecurityHeaders()
	csp := h["Content-Security-Policy"]
	for _, want := range []string{"object-src 'none'", "base-uri 'self'", "frame-ancestors 'self'"} {
		if !strings.Contains(csp, want) {
			t.Errorf("CSP 缺少 %q：%s", want, csp)
		}
	}
	// 门户允许管理员自定义 HTML/JS 且使用 CDN → 不能收紧脚本源（否则直接打断自定义功能）
	for _, bad := range []string{"script-src", "default-src", "style-src"} {
		if strings.Contains(csp, bad) {
			t.Errorf("CSP 不应包含 %q（会打断门户自定义脚本/样式与 CDN）：%s", bad, csp)
		}
	}
	if h["X-Content-Type-Options"] != "nosniff" {
		t.Errorf("应有 nosniff，实际 %q", h["X-Content-Type-Options"])
	}
	if h["Referrer-Policy"] == "" {
		t.Error("应设置 Referrer-Policy")
	}

	// 每次调用返回独立 map（调用方改动不污染其它请求）
	h["Content-Security-Policy"] = "mutated"
	if SecurityHeaders()["Content-Security-Policy"] == "mutated" {
		t.Error("SecurityHeaders 每次应返回新的 map")
	}
}
