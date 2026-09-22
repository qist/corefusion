package subscriptions

import (
	"strings"
	"testing"
)

// 审计 v5 P-6：Profile-Title 以前直接回显订阅者邮箱 ✗（会显示在客户端、被代理/CDN 记录 →
// PII 散播；且非 ASCII 或 CRLF 还会造成非法响应头/头注入）。这里锁住脱敏行为。
func TestMaskEmailForHeader(t *testing.T) {
	cases := []struct{ in, want string }{
		{"alice@example.com", "a****@example.com"},
		{"a@b.cn", "a@b.cn"},
		{"", ""},
		{"   bob@x.io  ", "b**@x.io"},
		{"noatsign", "n*******"},
		// 头注入/非法字符必须被挡掉（CR/LF/制表/非 ASCII 一律成 *）
		{"a\r\nX-Evil: 1@ex.com", "a***********@ex.com"},
		{"用户@例子.com", "**@**.com"},
	}
	for _, c := range cases {
		if got := maskEmailForHeader(c.in); got != c.want {
			t.Errorf("maskEmailForHeader(%q) = %q，期望 %q", c.in, got, c.want)
		}
	}

	// 关键性质：结果里不得出现原始邮箱、不得含控制字符（可安全写进 HTTP 头）
	for _, in := range []string{"alice@example.com", "a\r\nX: 1@ex.com", "用户@例子.com"} {
		got := maskEmailForHeader(in)
		if strings.Contains(got, in) {
			t.Errorf("脱敏结果仍是原文：%q", got)
		}
		for _, r := range got {
			if r < 0x21 || r > 0x7e {
				t.Errorf("结果含非法头字符 %q：%q", r, got)
			}
		}
	}

	// 域名保留（客户端里还能认出是哪个订阅），本地部分只留首字符
	got := maskEmailForHeader("longlocalpart@sub.example.com")
	if !strings.HasSuffix(got, "@sub.example.com") {
		t.Errorf("域名应保留：%q", got)
	}
	if !strings.HasPrefix(got, "l") || strings.Contains(got, "onglocalpart") {
		t.Errorf("本地部分应只留首字符：%q", got)
	}
}
