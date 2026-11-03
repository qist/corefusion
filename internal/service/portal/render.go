package portal

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strings"
)

// 门户页面的脚本来源策略（安全审计 v6 L-6）。
//
// 背景：模板 HTML/JS 由管理员自撰，等价于后台特权内容 —— CSP 不可能也不应该拦住
// 管理员自己写的脚本（更何况自定义模块 JS 走 new Function 沙箱）。本项要挡的是
// **非管理员内容**升级成脚本执行：用户可提交内容一旦漏出 HTML 注入，外部脚本或
// <img onerror> 这类事件属性都会把访客会话带走。
//
// 做法：从渲染产物里**静态写出**的 <script> 标签推导白名单 ——
//   - 外部脚本只放行它的源（scheme://host），运行时注入的第三方域一律拒绝；
//   - 内联脚本用 sha256 哈希放行（**不启用 'unsafe-inline'**），事件属性、动态插入的
//     <script> 都执行不了。
//
// 'unsafe-eval' 是硬需求：Vue 运行时模板编译器与门户的自定义模块 JS 都用 new Function。
var (
	scriptBlockRe   = regexp.MustCompile(`(?is)<script\b([^>]*)>(.*?)</script\s*>`)
	scriptSrcAttrRe = regexp.MustCompile(`(?i)\bsrc\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))`)
)

// maxScriptSrcTokens 限制 script-src 的长度（模板异常/被写入海量脚本时不至于撑爆响应头）。
const maxScriptSrcTokens = 64

// ScriptSrcCSP 依据渲染产物生成 script-src 指令值（不含 "script-src " 前缀）。
func ScriptSrcCSP(pageHTML string) string {
	tokens := []string{"'self'", "'unsafe-eval'"}
	seen := map[string]bool{"'self'": true, "'unsafe-eval'": true}
	add := func(tok string) {
		if tok == "" || seen[tok] || len(tokens) >= maxScriptSrcTokens {
			return
		}
		seen[tok] = true
		tokens = append(tokens, tok)
	}

	for _, m := range scriptBlockRe.FindAllStringSubmatch(pageHTML, -1) {
		attrs, body := m[1], m[2]
		if sm := scriptSrcAttrRe.FindStringSubmatch(attrs); sm != nil {
			src := sm[1]
			if src == "" {
				src = sm[2]
			}
			if src == "" {
				src = sm[3]
			}
			if origin := scriptOrigin(src); origin != "" {
				add(origin)
			} else if strings.HasPrefix(strings.TrimSpace(src), "/") {
				add("'self'")
			}
			continue
		}
		if strings.TrimSpace(body) == "" {
			continue
		}
		sum := sha256.Sum256([]byte(body))
		add("'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'")
	}
	return strings.Join(tokens, " ")
}

// scriptOrigin 把脚本 URL 归一成 CSP 源表达式；非 http(s)（data:/blob:/javascript:）
// 一律返回空字符串 = 不放行。
func scriptOrigin(src string) string {
	src = strings.TrimSpace(src)
	if src == "" {
		return ""
	}
	if strings.HasPrefix(src, "//") {
		src = "https:" + src
	}
	u, err := url.Parse(src)
	if err != nil || u.Host == "" {
		return ""
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return ""
	}
	return scheme + "://" + u.Host
}

// BaseSecurityHeaders 返回门户/合规页面的基础安全响应头（不含 script-src）。
//
// 只设不影响自定义的几条：
//   - object-src 'none'：禁掉 object/embed 这类老式注入载体；
//   - base-uri 'self'：防止注入 base 改写所有相对链接（钓鱼/劫持资源）；
//   - frame-ancestors 'self'：防点击劫持；
//   - nosniff + Referrer-Policy：阻止内容嗅探、少泄漏来源。
func BaseSecurityHeaders() map[string]string {
	return map[string]string{
		"Content-Security-Policy": "object-src 'none'; base-uri 'self'; frame-ancestors 'self'",
		"X-Content-Type-Options":  "nosniff",
		"Referrer-Policy":         "same-origin",
	}
}

// SecurityHeaders 返回门户页面的安全响应头；scriptSrc 为空时退化为基础头
// （例如门户模板尚未渲染出来 / 无脚本）。
func SecurityHeaders(scriptSrc string) map[string]string {
	h := BaseSecurityHeaders()
	if strings.TrimSpace(scriptSrc) != "" {
		h["Content-Security-Policy"] = "script-src " + scriptSrc + "; " + h["Content-Security-Policy"]
	}
	return h
}

// SecurityHeaders 返回当前渲染缓存对应的门户安全响应头（script-src 与缓存里的
// 渲染产物一致；模板更新会随缓存失效一起重算）。
func (s *Service) SecurityHeaders() map[string]string {
	if s == nil || s.cache == nil {
		return BaseSecurityHeaders()
	}
	return SecurityHeaders(s.cache.CSP())
}

func renderPortalHTML(htmlContent, cssContent, jsContent, configStr string, cfg RenderConfig) string {
	// configStr is intentionally unused: the structured config reaches the
	// page through the runtime config endpoint (with an initial snapshot
	// injected below for fast first paint).
	_ = configStr

	siteName := cfg.SiteName
	if siteName == "" {
		siteName = "CoreFusion"
	}

	// Scalar placeholder substitution (used by the default template and by
	// legacy code-mode templates)
	scalars := map[string]string{
		"{{.PortalName}}":          html.EscapeString(siteName),
		"{{.LogoURL}}":             cfg.LogoURL,
		"{{.RegistrationEnabled}}": fmt.Sprintf("%t", cfg.RegistrationEnabled),
		"{{.PrimaryColor}}":        cfg.PrimaryColor,
		"{{.BackgroundColor}}":     cfg.BackgroundColor,
		"{{.CardBackgroundColor}}": cfg.CardBackgroundColor,
		"{{.TextColor}}":           cfg.TextColor,
		"{{.AccentColor}}":         cfg.AccentColor,
		"{{.FontFamily}}":          cfg.FontFamily,
	}
	apply := func(s string) string {
		for k, v := range scalars {
			s = strings.ReplaceAll(s, k, v)
		}
		return s
	}
	htmlContent = apply(htmlContent)
	cssContent = apply(cssContent)
	jsContent = apply(jsContent)

	// Inject the v-cloak rule into <head> so the uncompiled Vue template is hidden
	// from the very first paint. Without this, the rule lives only in the CSS that
	// renderPortalHTML appends *after* the #app markup, so the browser can paint the
	// raw "{{ siteName }}" / v-* markup for a frame before Vue mounts (a flash of
	// HTML). This is independent of the DB-stored css_content, so it works even when
	// a previously-seeded template lacks the rule.
	const criticalCloakCSS = "<style>[v-cloak]{display:none}</style>"
	if idx := strings.Index(htmlContent, "<head>"); idx >= 0 {
		insertAt := idx + len("<head>")
		htmlContent = htmlContent[:insertAt] + "\n" + criticalCloakCSS + htmlContent[insertAt:]
	} else {
		htmlContent = criticalCloakCSS + htmlContent
	}

	// Initial config snapshot for instant first paint. The runtime refetches
	// fresh config on boot, so this snapshot never needs to be up to the second.
	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		cfgJSON = []byte("{}")
	}

	var b strings.Builder
	b.Grow(len(htmlContent) + len(cssContent) + len(jsContent) + len(cfgJSON) + 128)
	b.WriteString(htmlContent)
	b.WriteString("\n<script>window.PORTAL_CONFIG = ")
	b.Write(cfgJSON)
	b.WriteString(";</script>\n<style>\n")
	b.WriteString(cssContent)
	b.WriteString("\n</style>\n<script>\n")
	b.WriteString(jsContent)
	b.WriteString("\n</script>")
	return b.String()
}
