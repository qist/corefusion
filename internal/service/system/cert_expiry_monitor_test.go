package system

import (
	"testing"
	"time"

	"corefusion/internal/model/entity"
)

// TestCertExpiryWindowFor IP 证书与面板「异常告警」同口径（有效期 1/3），
// 域名证书仍按 7 天提醒。
// 背景：IP 证书约 6 天寿命、自动续期阈值 3 天；邮件若用固定 3 天窗口，
// 正常续期期间每天都会命中并发邮件 —— 面板按有效期比例计算，不会这样。
func TestCertExpiryWindowFor(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name   string
		cert   entity.Certificate
		expect time.Duration
	}{
		{
			name:   "单项 IP（7 天寿命）→ 有效期 1/3",
			cert:   entity.Certificate{Domain: "203.0.113.48", CreatedAt: now, ExpiresAt: now.Add(7 * 24 * time.Hour)},
			expect: 7 * 24 * time.Hour / 3,
		},
		{
			name:   "domains 数组含 IP（6 天寿命）→ 2 天",
			cert:   entity.Certificate{Domain: "x", Domains: `["203.0.113.48"]`, CreatedAt: now, ExpiresAt: now.Add(6 * 24 * time.Hour)},
			expect: 2 * 24 * time.Hour,
		},
		{
			name:   "域名 + IP 混合 → 按 IP 比例",
			cert:   entity.Certificate{Domain: "a.com", Domains: `["a.com","203.0.113.48"]`, CreatedAt: now, ExpiresAt: now.Add(7 * 24 * time.Hour)},
			expect: 7 * 24 * time.Hour / 3,
		},
		{
			name:   "纯域名 → 7 天",
			cert:   entity.Certificate{Domain: "a.com", CreatedAt: now, ExpiresAt: now.Add(90 * 24 * time.Hour)},
			expect: 7 * 24 * time.Hour,
		},
		{
			name:   "通配符域名 → 7 天",
			cert:   entity.Certificate{Domain: "*.example.com", CreatedAt: now, ExpiresAt: now.Add(90 * 24 * time.Hour)},
			expect: 7 * 24 * time.Hour,
		},
	}
	for _, c := range cases {
		if got := certExpiryWindowFor(c.cert); got != c.expect {
			t.Errorf("%s: certExpiryWindowFor=%v, want %v", c.name, got, c.expect)
		}
	}
}

// TestCertExpiryScanWindowDays 数据库扫描窗口必须覆盖面板告警上限（30 天），
// 再由每张证书的实际窗口过滤。
func TestCertExpiryScanWindowDays(t *testing.T) {
	if got := certExpiryScanWindowDays(); got != certExpiryMaxAheadDays {
		t.Fatalf("scan window=%d, want %d (panel max)", got, certExpiryMaxAheadDays)
	}
}

// TestCertIdentifierList domains 优先、退化 domain、去空。
func TestCertIdentifierList(t *testing.T) {
	got := certIdentifierList(`[" a.com ", "", "b.com"]`, "fallback.com")
	if len(got) != 2 || got[0] != "a.com" || got[1] != "b.com" {
		t.Fatalf("certIdentifierList=%v", got)
	}
	got = certIdentifierList("", "only.com")
	if len(got) != 1 || got[0] != "only.com" {
		t.Fatalf("fallback 失败: %v", got)
	}
}
