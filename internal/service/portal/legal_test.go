package portal

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderMarkdownHTML(t *testing.T) {
	md := "# 标题\n\n正文 **加粗** 与 `code`，以及 [链接](https://example.com)。\n\n- 项目一\n- 项目二\n\n## 二级\n\n> 引用\n"
	out := renderMarkdownHTML(md)
	for _, want := range []string{
		"<h1>标题</h1>",
		"<strong>加粗</strong>",
		"<code>code</code>",
		`<a href="https://example.com" target="_blank" rel="noopener">链接</a>`,
		"<ul>",
		"<li>项目一</li>",
		"<h2>二级</h2>",
		"<blockquote>",
		"<p>引用</p>",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("renderMarkdownHTML 缺少 %q\n实际：%s", want, out)
		}
	}
}

func TestRenderMarkdownHTMLEscapesRawHTML(t *testing.T) {
	out := renderMarkdownHTML("正常 <script>alert(1)</script> 文本")
	if strings.Contains(out, "<script>") {
		t.Fatalf("原始 HTML 应被转义，实际：%s", out)
	}
	if !strings.Contains(out, "&lt;script&gt;") {
		t.Fatalf("应转义为实体，实际：%s", out)
	}
}

func TestPickLocale(t *testing.T) {
	available := []string{"en", "zh-CN"}

	// 显式 lang 优先
	if got := pickLocale("en", "zh-CN", available, "zh-CN"); got != "en" {
		t.Errorf("显式 lang 应优先，得到 %q", got)
	}
	// 无显式 lang → Accept-Language（en-US → en）
	if got := pickLocale("", "en-US,en;q=0.9", available, "zh-CN"); got != "en" {
		t.Errorf("Accept-Language 前缀匹配失败，得到 %q", got)
	}
	// 都不匹配 → 默认语言
	if got := pickLocale("", "fr-FR", available, "zh-CN"); got != "zh-CN" {
		t.Errorf("应回退默认语言，得到 %q", got)
	}
	// 可用集合只有一个时，总是返回它
	if got := pickLocale("fr", "", []string{"en"}, "zh-CN"); got != "en" {
		t.Errorf("只有一个可用语言时应返回它，得到 %q", got)
	}
}

func TestParseAcceptLanguage(t *testing.T) {
	got := parseAcceptLanguage("en-US,en;q=0.9, zh-CN;q=0.8,*")
	want := []string{"en-US", "en", "zh-CN"}
	if len(got) != len(want) {
		t.Fatalf("长度不符：%v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("第 %d 项应为 %q，得到 %q", i, want[i], got[i])
		}
	}
}

func TestParseLegalContent(t *testing.T) {
	// 新格式：按语言 map
	if m := parseLegalContent(json.RawMessage(`{"zh-CN":"中","en":"EN"}`)); m["zh-CN"] != "中" || m["en"] != "EN" {
		t.Errorf("新格式解析失败：%v", m)
	}
	// 旧格式：单语言纯字符串 → 归一到默认语言
	if m := parseLegalContent(json.RawMessage(`"旧内容"`)); m[defaultLangCode] != "旧内容" {
		t.Errorf("旧格式应归一默认语言，得到 %v", m)
	}
	// 空值
	if m := parseLegalContent(json.RawMessage(`""`)); m != nil {
		t.Errorf("空字符串应为 nil，得到 %v", m)
	}
	if m := parseLegalContent(nil); m != nil {
		t.Errorf("空输入应为 nil，得到 %v", m)
	}
}
