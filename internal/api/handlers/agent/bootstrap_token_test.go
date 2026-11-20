package agent

import (
	"net/http/httptest"
	"testing"
)

// 安全审计 v7 L-5：引导令牌优先走请求头，URL query 仅兼容旧命令（并需标记出来记告警）。
func TestBootstrapTokenFromRequest(t *testing.T) {
	cases := []struct {
		name      string
		url       string
		header    map[string]string
		wantToken string
		wantInURL bool
	}{
		{
			name:      "header wins",
			url:       "/api/v1/agent/install?id=1&token=from-url",
			header:    map[string]string{"X-Bootstrap-Token": " from-header "},
			wantToken: "from-header",
			wantInURL: false,
		},
		{
			name:      "bearer token",
			url:       "/api/v1/agent/install?id=1",
			header:    map[string]string{"Authorization": "Bearer abc123"},
			wantToken: "abc123",
			wantInURL: false,
		},
		{
			name:      "legacy query",
			url:       "/api/v1/agent/install?id=1&token=legacy",
			wantToken: "legacy",
			wantInURL: true,
		},
		{
			name:      "missing",
			url:       "/api/v1/agent/install?id=1",
			wantToken: "",
			wantInURL: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", tc.url, nil)
			for k, v := range tc.header {
				r.Header.Set(k, v)
			}
			token, inURL := bootstrapTokenFromRequest(r)
			if token != tc.wantToken {
				t.Errorf("token = %q, want %q", token, tc.wantToken)
			}
			if inURL != tc.wantInURL {
				t.Errorf("tokenInURL = %v, want %v", inURL, tc.wantInURL)
			}
		})
	}
}
