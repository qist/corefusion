package metrics

import (
	"testing"
	"time"

	"corefusion/internal/model/entity"
)

// 告警窗口必须随有效期收缩：固定的 30 天会让 6–7 天的 IP 证书
// 刚签发就进「即将过期」。
func TestCertExpiryWarnWindow(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name string
		cert entity.Certificate
		want time.Duration
	}{
		{
			name: "7 天短寿命 IP 证书 → 有效期 1/3",
			cert: entity.Certificate{CreatedAt: now, ExpiresAt: now.Add(7 * 24 * time.Hour)},
			want: 7 * 24 * time.Hour / 3,
		},
		{
			name: "90 天域名证书 → 仍按 30 天",
			cert: entity.Certificate{CreatedAt: now, ExpiresAt: now.Add(90 * 24 * time.Hour)},
			want: 30 * 24 * time.Hour,
		},
		{
			name: "1 年证书 → 上限 30 天",
			cert: entity.Certificate{CreatedAt: now, ExpiresAt: now.Add(365 * 24 * time.Hour)},
			want: 30 * 24 * time.Hour,
		},
		{
			name: "极短证书 → 下限 1 天",
			cert: entity.Certificate{CreatedAt: now, ExpiresAt: now.Add(36 * time.Hour)},
			want: 24 * time.Hour,
		},
		{
			name: "续期后按 LastRenewedAt 重新计算",
			cert: entity.Certificate{
				CreatedAt:     now.Add(-90 * 24 * time.Hour),
				LastRenewedAt: durationPtr(now),
				ExpiresAt:     now.Add(7 * 24 * time.Hour),
			},
			want: 7 * 24 * time.Hour / 3,
		},
		{
			name: "缺创建时间 → 回退 30 天",
			cert: entity.Certificate{ExpiresAt: now.Add(6 * 24 * time.Hour)},
			want: 30 * 24 * time.Hour,
		},
	}
	for _, tc := range cases {
		got := certExpiryWarnWindow(tc.cert)
		if got != tc.want {
			t.Errorf("%s: window = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func durationPtr(t time.Time) *time.Time { return &t }
