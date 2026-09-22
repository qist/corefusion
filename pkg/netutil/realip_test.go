package netutil

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// 安全审计 v5 S3：`GetRealIP` 是限流键与审计日志的来源，必须满足：
//   - 未配置可信代理时**完全忽略** XFF（只能用直连地址）；
//   - 配置了可信代理、但 XFF 里没有任何合法 IP 时，**回退到直连地址**（不能把任意文本当 IP 返回 ✗）。
func TestGetRealIPOnlyTrustsForwardedHeadersFromConfiguredProxies(t *testing.T) {
	defer SetTrustedProxies(nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req.RemoteAddr = "203.0.113.9:34567"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")

	// ① 未配置可信代理 → 忽略 XFF（否则任何人都能靠换 XFF 绕过登录限流）
	SetTrustedProxies(nil)
	if got := GetRealIP(req); got != "203.0.113.9" {
		t.Errorf("未配置可信代理时必须用直连地址，得到 %q", got)
	}

	// ② 直连方是可信代理 → 采信 XFF
	SetTrustedProxies([]string{"203.0.113.9"})
	if got := GetRealIP(req); got != "1.2.3.4" {
		t.Errorf("可信代理的 XFF 应被采信，得到 %q", got)
	}
}

func TestGetRealIPDoesNotReturnUnvalidatedForwardedText(t *testing.T) {
	defer SetTrustedProxies(nil)
	SetTrustedProxies([]string{"203.0.113.9"})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req.RemoteAddr = "203.0.113.9:34567"
	// 全是"看着像地址但不是 IP"的伪造文本（旧实现在兜底分支会把第一个非空字符串**原样返回** ✗，
	// 该值会流进限流键、审计日志，甚至被拼进 nft 规则的参数）
	req.Header.Set("X-Forwarded-For", "not-an-ip; rm -rf /, 999.999.999.999")

	if got := GetRealIP(req); got != "203.0.113.9" {
		t.Errorf("非法 XFF 必须回退到直连地址，得到 %q（旧实现会原样返回伪造串）", got)
	}
}

func TestTrustedProxyStringsRoundTrip(t *testing.T) {
	defer SetTrustedProxies(nil)
	SetTrustedProxies([]string{"10.0.0.0/8", "::1"})
	got := TrustedProxyStrings()
	if len(got) != 2 || got[0] != "10.0.0.0/8" || got[1] != "::1" {
		t.Errorf("原始列表必须可回读（gin 引擎要用同一份配置），得到 %v", got)
	}
	SetTrustedProxies(nil)
	if got := TrustedProxyStrings(); len(got) != 0 {
		t.Errorf("清空后应为空，得到 %v", got)
	}
}
