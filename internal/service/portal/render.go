package portal

import (
	"encoding/json"
	"fmt"
	"html"
	"strings"
)

// renderPortalHTML assembles the portal page shell from the template parts.
//
// Only lightweight scalar placeholders (site name, colors, font, registration
// flag) are substituted here. The structured config (navigation, modules,
// lang, regions) is NOT baked into the page — the runtime fetches it live from
// /api/v1/public/portal-config, so builder changes render straight from the
// database. The assembled shell is static for a given template version, which
// is what allows the render cache to serve it without any re-rendering work.
// SecurityHeaders 返回门户页面的**安全响应头**（自动应用，不需要任何人工配置）。
//
// 只设不影响自定义的几条：
//   - object-src 'none'：禁掉 object/embed 这类老式注入载体；
//   - base-uri 'self'：防止注入 base 改写所有相对链接（钓鱼/劫持资源）；
//   - frame-ancestors 'self'：防点击劫持；
//   - nosniff + Referrer-Policy：阻止内容嗅探、少泄漏来源。
//
// **不设** default-src / script-src：门户允许管理员自定义 HTML/JS 且用了 CDN，
// 收紧脚本源会直接打断自定义功能（那属于后台特权内容，不在本项防护范围内）。
func SecurityHeaders() map[string]string {
	return map[string]string{
		"Content-Security-Policy": "object-src 'none'; base-uri 'self'; frame-ancestors 'self'",
		"X-Content-Type-Options":  "nosniff",
		"Referrer-Policy":         "same-origin",
	}
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
