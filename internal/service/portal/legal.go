package portal

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"sort"
	"strings"
)

// legalDoc 描述一份合规文档（隐私政策 / 服务条款）。
type legalDoc struct {
	key      string // "privacy" / "terms"
	titleKey string // 内置语言 key（legal.privacy / legal.terms）
	fallback string // 内置语言缺失时的兜底标题
}

var legalDocs = map[string]legalDoc{
	"privacy": {key: "privacy", titleKey: "legal.privacy", fallback: "隐私政策"},
	"terms":   {key: "terms", titleKey: "legal.terms", fallback: "服务条款"},
}

// RenderLegalPage 渲染一份独立的合规文档页面（服务端 Markdown → HTML）。
//
// doc ∈ {"privacy","terms"}；lang 为 ?lang= 显式语言；acceptLanguage 为请求头；
// backURL 为「返回门户」链接（主引擎 "/p"、独立门户引擎 "/"）。
// 返回 (HTML, HTTP 状态码)：文档不存在 / 无内容时返回 404。
func (s *Service) RenderLegalPage(ctx context.Context, doc, lang, acceptLanguage, backURL string) (string, int) {
	meta, ok := legalDocs[doc]
	if !ok {
		return "", http.StatusNotFound
	}
	cfg, err := s.GetPublicConfig(ctx)
	if err != nil {
		return "", http.StatusInternalServerError
	}
	content := map[string]string(nil)
	switch doc {
	case "privacy":
		content = cfg.PrivacyPolicyContent
	case "terms":
		content = cfg.TermsOfServiceContent
	}
	// 去掉空值，得到可用语言集合
	available := make([]string, 0, len(content))
	for code, body := range content {
		if strings.TrimSpace(body) != "" {
			available = append(available, code)
		}
	}
	if len(available) == 0 {
		return "", http.StatusNotFound
	}
	sort.Strings(available)

	defaultLocale := defaultLocaleFromLang(cfg)
	locale := pickLocale(lang, acceptLanguage, available, defaultLocale)

	title := meta.fallback
	if s, ok := matchBuiltinLangPack(locale)[meta.titleKey].(string); ok && s != "" {
		title = s
	}

	body := renderMarkdownHTML(content[locale])

	var b strings.Builder
	b.WriteString("<!DOCTYPE html>\n<html lang=\"" + html.EscapeString(locale) + "\">\n<head>\n")
	b.WriteString("<meta charset=\"UTF-8\">\n")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">\n")
	b.WriteString("<title>" + html.EscapeString(title) + " · " + html.EscapeString(cfg.SiteName) + "</title>\n")
	b.WriteString("<style>" + legalPageCSS + "</style>\n</head>\n<body>\n")
	b.WriteString("<main class=\"legal-wrap\">\n")
	b.WriteString("<header class=\"legal-head\">\n")
	b.WriteString("<a class=\"legal-back\" href=\"" + html.EscapeString(backURL) + "\">← " + html.EscapeString(cfg.SiteName) + "</a>\n")
	if len(available) > 1 {
		b.WriteString("<nav class=\"legal-langs\">")
		for _, code := range available {
			cls := "legal-lang"
			if code == locale {
				cls += " active"
			}
			b.WriteString("<a class=\"" + cls + "\" href=\"?lang=" + html.EscapeString(code) + "\">" + html.EscapeString(code) + "</a>")
		}
		b.WriteString("</nav>\n")
	}
	b.WriteString("</header>\n")
	b.WriteString("<article class=\"legal-body md\">\n" + body + "\n</article>\n")
	b.WriteString("</main>\n</body>\n</html>\n")
	return b.String(), http.StatusOK
}

// defaultLocaleFromLang 从门户语言包里取默认语言码，取不到则用内置默认（zh-CN）。
func defaultLocaleFromLang(cfg RenderConfig) string {
	var pack struct {
		Default string `json:"default"`
	}
	if len(cfg.Lang) > 0 && json.Unmarshal(cfg.Lang, &pack) == nil && pack.Default != "" {
		return pack.Default
	}
	return defaultLangCode
}

// pickLocale 依次用 显式 lang → Accept-Language → 默认语言 → 首个可用语言 匹配可用语言集合。
func pickLocale(lang, acceptLanguage string, available []string, defaultLocale string) string {
	try := func(code string) (string, bool) {
		code = strings.TrimSpace(code)
		if code == "" {
			return "", false
		}
		if m, ok := matchLocale(code, available); ok {
			return m, true
		}
		return "", false
	}
	if m, ok := try(lang); ok {
		return m
	}
	for _, code := range parseAcceptLanguage(acceptLanguage) {
		if m, ok := try(code); ok {
			return m
		}
	}
	if m, ok := try(defaultLocale); ok {
		return m
	}
	return available[0]
}

// matchLocale 在 available 中精确匹配，其次按主语言前缀匹配（en-US → en）。
func matchLocale(code string, available []string) (string, bool) {
	for _, c := range available {
		if strings.EqualFold(c, code) {
			return c, true
		}
	}
	prefix := strings.ToLower(strings.Split(code, "-")[0])
	for _, c := range available {
		if strings.ToLower(strings.Split(c, "-")[0]) == prefix {
			return c, true
		}
	}
	return "", false
}

// parseAcceptLanguage 解析 Accept-Language 头，按优先级返回语言码列表。
func parseAcceptLanguage(header string) []string {
	if strings.TrimSpace(header) == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(header, ",") {
		tag := strings.TrimSpace(strings.SplitN(part, ";", 2)[0])
		if tag != "" && tag != "*" {
			out = append(out, tag)
		}
	}
	return out
}

// ---- 极简 Markdown → HTML（服务端渲染；独立页面不依赖 JS，便于爬虫/验证抓取）----

var (
	mdHeadingRe = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
	mdULRe      = regexp.MustCompile(`^[-*+]\s+(.*)$`)
	mdOLRe      = regexp.MustCompile(`^\d+\.\s+(.*)$`)
	mdCodeRe    = regexp.MustCompile("`([^`]+)`")
	mdLinkRe    = regexp.MustCompile(`\[([^\]]+)\]\(([^)\s]+)\)`)
	mdBoldRe    = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	mdItalicRe  = regexp.MustCompile(`\*([^*]+)\*`)
)

func renderMarkdownHTML(md string) string {
	lines := strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n")
	var b strings.Builder
	inUL, inOL, inCode, inQuote := false, false, false, false
	var para []string

	flushPara := func() {
		if len(para) > 0 {
			b.WriteString("<p>" + renderInlineMD(strings.Join(para, " ")) + "</p>\n")
			para = nil
		}
	}
	closeLists := func() {
		if inUL {
			b.WriteString("</ul>\n")
			inUL = false
		}
		if inOL {
			b.WriteString("</ol>\n")
			inOL = false
		}
	}
	closeQuote := func() {
		if inQuote {
			b.WriteString("</blockquote>\n")
			inQuote = false
		}
	}

	for _, raw := range lines {
		line := strings.TrimRight(raw, " \t")
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			flushPara()
			closeLists()
			closeQuote()
			if inCode {
				b.WriteString("</code></pre>\n")
				inCode = false
			} else {
				b.WriteString("<pre><code>")
				inCode = true
			}
			continue
		}
		if inCode {
			b.WriteString(html.EscapeString(raw) + "\n")
			continue
		}
		t := strings.TrimSpace(line)
		if t == "" {
			flushPara()
			closeLists()
			closeQuote()
			continue
		}
		if t == "---" || t == "***" || t == "___" {
			flushPara()
			closeLists()
			closeQuote()
			b.WriteString("<hr>\n")
			continue
		}
		if m := mdHeadingRe.FindStringSubmatch(t); m != nil {
			flushPara()
			closeLists()
			closeQuote()
			lvl := len(m[1])
			fmt.Fprintf(&b, "<h%d>%s</h%d>\n", lvl, renderInlineMD(m[2]), lvl)
			continue
		}
		if strings.HasPrefix(t, "> ") {
			flushPara()
			closeLists()
			if !inQuote {
				b.WriteString("<blockquote>\n")
				inQuote = true
			}
			b.WriteString("<p>" + renderInlineMD(strings.TrimPrefix(t, "> ")) + "</p>\n")
			continue
		}
		if m := mdULRe.FindStringSubmatch(t); m != nil {
			flushPara()
			closeQuote()
			if inOL {
				b.WriteString("</ol>\n")
				inOL = false
			}
			if !inUL {
				b.WriteString("<ul>\n")
				inUL = true
			}
			b.WriteString("<li>" + renderInlineMD(m[1]) + "</li>\n")
			continue
		}
		if m := mdOLRe.FindStringSubmatch(t); m != nil {
			flushPara()
			closeQuote()
			if inUL {
				b.WriteString("</ul>\n")
				inUL = false
			}
			if !inOL {
				b.WriteString("<ol>\n")
				inOL = true
			}
			b.WriteString("<li>" + renderInlineMD(m[1]) + "</li>\n")
			continue
		}
		closeLists()
		closeQuote()
		para = append(para, t)
	}
	if inCode {
		b.WriteString("</code></pre>\n")
	}
	flushPara()
	closeLists()
	closeQuote()
	return b.String()
}

// renderInlineMD 先做 HTML 转义，再应用行内 Markdown（加粗/斜体/行内代码/链接）。
func renderInlineMD(s string) string {
	s = html.EscapeString(s)
	s = mdCodeRe.ReplaceAllString(s, "<code>$1</code>")
	s = mdLinkRe.ReplaceAllString(s, `<a href="$2" target="_blank" rel="noopener">$1</a>`)
	s = mdBoldRe.ReplaceAllString(s, "<strong>$1</strong>")
	s = mdItalicRe.ReplaceAllString(s, "<em>$1</em>")
	return s
}

// legalPageCSS 独立合规页面的内置样式（自包含，无外部依赖）。
const legalPageCSS = `*{box-sizing:border-box}
body{margin:0;background:#f7f8fc;color:#111827;font-family:system-ui,-apple-system,"Segoe UI",Roboto,"Helvetica Neue",Arial,"PingFang SC","Microsoft YaHei",sans-serif;line-height:1.75}
a{color:#4f46e5}
.legal-wrap{max-width:820px;margin:0 auto;padding:32px 20px 64px}
.legal-head{display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap;margin-bottom:20px}
.legal-back{font-size:14px;text-decoration:none;color:#4b5563}
.legal-back:hover{color:#4f46e5}
.legal-langs{display:flex;gap:6px}
.legal-lang{font-size:12px;padding:2px 10px;border:1px solid #e5e7eb;border-radius:999px;text-decoration:none;color:#4b5563;background:#fff}
.legal-lang.active{border-color:#4f46e5;color:#4f46e5;background:rgba(79,70,229,.08)}
.legal-body{background:#fff;border:1px solid #eef0f4;border-radius:16px;padding:32px 36px;box-shadow:0 1px 2px rgba(17,24,39,.04),0 12px 32px -20px rgba(17,24,39,.16)}
.legal-body h1{font-size:26px;margin:0 0 16px;letter-spacing:-.01em}
.legal-body h2{font-size:18px;margin:26px 0 10px}
.legal-body h3{font-size:16px;margin:20px 0 8px}
.legal-body p{margin:10px 0;color:#374151}
.legal-body ul,.legal-body ol{padding-left:22px;margin:10px 0;color:#374151}
.legal-body li{margin:4px 0}
.legal-body blockquote{margin:12px 0;padding:6px 14px;border-left:3px solid #4f46e5;background:#f5f6ff;border-radius:0 8px 8px 0;color:#4b5563}
.legal-body code{background:rgba(17,24,39,.06);padding:2px 6px;border-radius:4px;font-size:13px;font-family:"SF Mono","Fira Code",Consolas,monospace}
.legal-body pre{background:#0f172a;color:#e5e9f0;padding:14px 16px;border-radius:10px;overflow:auto}
.legal-body pre code{background:none;color:inherit;padding:0}
.legal-body hr{border:none;border-top:1px solid #eef0f4;margin:22px 0}
@media (max-width:600px){.legal-body{padding:22px 18px}}`
