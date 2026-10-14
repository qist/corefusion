package system

import (
	"testing"

	"corefusion/internal/model/entity"
)

// TestCertExpiryDaysFor IP 证书按 3 天提醒、域名证书按 7 天提醒。
// 背景：ACME 的 IP 证书是短时证书（约 6 天寿命），用统一的 7 天窗口会从
// "还剩 6 天"就天天提醒 —— 用户要求 IP 改到剩 3 天才提示，与自动续期同口径。
func TestCertExpiryDaysFor(t *testing.T) {
	cases := []struct {
		name   string
		cert   entity.Certificate
		expect int
	}{
		{"单项 IP", entity.Certificate{Domain: "203.0.113.48"}, certExpiryIPAheadDays},
		{"domains 数组含 IP", entity.Certificate{Domain: "x", Domains: `["203.0.113.48"]`}, certExpiryIPAheadDays},
		{"域名 + IP 混合 → 按短时证书", entity.Certificate{Domain: "a.com", Domains: `["a.com","203.0.113.48"]`}, certExpiryIPAheadDays},
		{"纯域名", entity.Certificate{Domain: "a.com"}, certExpiryAheadDays},
		{"通配符域名", entity.Certificate{Domain: "*.example.com"}, certExpiryAheadDays},
	}
	for _, c := range cases {
		if got := certExpiryDaysFor(c.cert); got != c.expect {
			t.Errorf("%s: certExpiryDaysFor=%d, want %d", c.name, got, c.expect)
		}
	}
}

// TestCertExpiryScanWindowDays 数据库扫描窗口必须取两类阈值的较大值（7 天），
// 否则只按 IP 的 3 天窗口查会漏掉 7 天内的域名证书。
func TestCertExpiryScanWindowDays(t *testing.T) {
	if got := certExpiryScanWindowDays(); got != certExpiryAheadDays {
		t.Fatalf("scan window=%d, want %d (max of IP/domain)", got, certExpiryAheadDays)
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
