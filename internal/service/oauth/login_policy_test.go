package oauth

import (
	"testing"
	"time"

	"corefusion/internal/model/entity"
)

func pastTime() *time.Time   { t := time.Now().Add(-24 * time.Hour); return &t }
func futureTime() *time.Time { t := time.Now().Add(24 * time.Hour); return &t }

// 到期账号必须能通过 OAuth 登录去续费；只有真正的封禁才拒绝。
func TestCanLoginWithOAuth(t *testing.T) {
	cases := []struct {
		name   string
		client *entity.Client
		want   bool
	}{
		{"active", &entity.Client{Status: "active"}, true},
		{"status=expired", &entity.Client{Status: "expired"}, true},
		{"suspended+auto_expired", &entity.Client{Status: "suspended", SuspendReason: "auto_expired"}, true},
		{"suspended+admin", &entity.Client{Status: "suspended", SuspendReason: "admin"}, false},
		{"suspended+auto_quota", &entity.Client{Status: "suspended", SuspendReason: "auto_quota"}, false},
		{"suspended 无原因 + 已过期（历史遗留）", &entity.Client{Status: "suspended", ExpiresAt: pastTime()}, true},
		{"suspended 无原因 + 未过期", &entity.Client{Status: "suspended", ExpiresAt: futureTime()}, false},
		{"suspended 无原因 + 无到期时间", &entity.Client{Status: "suspended"}, false},
		{"空状态", &entity.Client{}, false},
		{"nil", nil, false},
	}
	for _, tc := range cases {
		if got := canLoginWithOAuth(tc.client); got != tc.want {
			t.Errorf("%s: canLoginWithOAuth = %v, want %v", tc.name, got, tc.want)
		}
	}
}
