package portal

import (
	"crypto/sha256"
	"encoding/base64"
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

// 门户页面的安全响应头：基础头永远在，script-src 由渲染产物推导（安全审计 v6 L-6）。
func TestPortalSecurityHeaders(t *testing.T) {
	h := BaseSecurityHeaders()
	csp := h["Content-Security-Policy"]
	for _, want := range []string{"object-src 'none'", "base-uri 'self'", "frame-ancestors 'self'"} {
		if !strings.Contains(csp, want) {
			t.Errorf("CSP 缺少 %q：%s", want, csp)
		}
	}
	// 基础头（合规文档等页面）不带 script-src：那些页面没有脚本，收紧也无意义
	for _, bad := range []string{"script-src", "default-src", "style-src"} {
		if strings.Contains(csp, bad) {
			t.Errorf("基础 CSP 不应包含 %q：%s", bad, csp)
		}
	}
	if h["X-Content-Type-Options"] != "nosniff" {
		t.Errorf("应有 nosniff，实际 %q", h["X-Content-Type-Options"])
	}
	if h["Referrer-Policy"] == "" {
		t.Error("应设置 Referrer-Policy")
	}

	// 带 script-src 的版本：指令必须**前置**（不覆盖基础头里的 object-src 等）
	withScript := SecurityHeaders("'self' 'unsafe-eval'")["Content-Security-Policy"]
	if !strings.HasPrefix(withScript, "script-src 'self' 'unsafe-eval'; ") {
		t.Errorf("script-src 应前置，实际 %q", withScript)
	}
	for _, want := range []string{"object-src 'none'", "base-uri 'self'", "frame-ancestors 'self'"} {
		if !strings.Contains(withScript, want) {
			t.Errorf("带 script-src 的 CSP 丢了基础指令 %q：%s", want, withScript)
		}
	}

	// 每次调用返回独立 map（调用方改动不污染其它请求）
	h["Content-Security-Policy"] = "mutated"
	if BaseSecurityHeaders()["Content-Security-Policy"] == "mutated" {
		t.Error("SecurityHeaders 每次应返回新的 map")
	}
}

// ScriptSrcCSP：只放行模板里**静态写出**的脚本源，内联脚本走 sha256，
// 绝不启用 'unsafe-inline'（否则 <img onerror> 这类事件属性注入照样执行）。
func TestScriptSrcCSP(t *testing.T) {
	page := `<html><head>` +
		`<script src="https://cdn.jsdelivr.net/npm/vue@3/dist/vue.global.prod.js"></script>` +
		`<script src="/static/qrcode.min.js"></script>` +
		`<script>window.PORTAL_CONFIG = {"a":1};</script>` +
		`<script id="empty"></script>` +
		`<script src="data:text/javascript,alert(1)"></script>` +
		`</head><body><img src=x onerror="alert(1)"></body></html>`

	csp := ScriptSrcCSP(page)
	for _, want := range []string{"'self'", "'unsafe-eval'", "https://cdn.jsdelivr.net"} {
		if !strings.Contains(csp, want) {
			t.Errorf("script-src 缺少 %q：%s", want, csp)
		}
	}
	// 内联脚本：sha256(内容) 白名单
	inline := `window.PORTAL_CONFIG = {"a":1};`
	sum := sha256.Sum256([]byte(inline))
	if want := "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"; !strings.Contains(csp, want) {
		t.Errorf("内联脚本缺少哈希白名单 %q：%s", want, csp)
	}
	// 事件属性 / 动态注入靠这条兜住
	if strings.Contains(csp, "'unsafe-inline'") {
		t.Errorf("script-src 不得包含 'unsafe-inline'：%s", csp)
	}
	// data: 脚本一律不放行
	if strings.Contains(csp, "data:") {
		t.Errorf("script-src 不应放行 data:：%s", csp)
	}
	// 空内联脚本不产生无意义哈希
	if strings.Count(csp, "'sha256-") != 1 {
		t.Errorf("只应为 1 个内联脚本生成哈希：%s", csp)
	}

	// 默认模板（真实渲染产物）必须能通过：否则门户整站脚本会被 CSP 拦掉
	rendered := renderPortalHTML(defaultPortalHTML, defaultPortalCSS, defaultPortalJS, "", MergeConfig(nil, false))
	defCSP := ScriptSrcCSP(rendered)
	if !strings.Contains(defCSP, "https://cdn.jsdelivr.net") {
		t.Errorf("默认模板的 CDN 源未被放行：%s", defCSP)
	}
	if !strings.Contains(defCSP, "'sha256-") {
		t.Errorf("默认模板的内联脚本未获得哈希白名单：%s", defCSP)
	}
}
