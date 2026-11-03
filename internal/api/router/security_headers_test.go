package router

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
)

// 安全审计 v6 L-5：管理端 token 存 localStorage，CSP 是出现 XSS 后阻止
// "内联脚本 / 事件属性 / 未知来源脚本" 直接执行的最后一道闸。
func TestAdminContentSecurityPolicy(t *testing.T) {
	csp := adminContentSecurityPolicy()
	if !strings.Contains(csp, "script-src 'self' 'unsafe-eval'") {
		t.Errorf("缺少 script-src 'self' 'unsafe-eval'：%s", csp)
	}
	// 'unsafe-inline' 一旦出现，内联事件属性（XSS 常见载荷）就会重新可用
	if strings.Contains(csp, "'unsafe-inline'") {
		t.Errorf("不得包含 'unsafe-inline'：%s", csp)
	}
	for _, want := range []string{"worker-src 'self' blob:", "object-src 'none'", "base-uri 'self'", "frame-ancestors 'self'"} {
		if !strings.Contains(csp, want) {
			t.Errorf("CSP 缺少 %q：%s", want, csp)
		}
	}
}

// 配了 webPath 时 index.html 会注入一段内联脚本 —— 必须进哈希白名单，
// 否则管理端在子路径部署下会整站白屏。
func TestAdminContentSecurityPolicyAllowsBasePathInlineScript(t *testing.T) {
	inline := `window.__CF_BASE_PATH__="/admin/";`
	csp := adminContentSecurityPolicy(inline)

	sum := sha256.Sum256([]byte(inline))
	want := "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
	if !strings.Contains(csp, want) {
		t.Errorf("内联 base-path 脚本未进白名单 %q：%s", want, csp)
	}
	// 空脚本不产生哈希（避免无意义 token）
	if got := adminContentSecurityPolicy(""); strings.Contains(got, "'sha256-") {
		t.Errorf("空内联脚本不应生成哈希：%s", got)
	}
}
