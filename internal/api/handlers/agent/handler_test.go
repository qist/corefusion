package agent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type fakeSettingStore map[string]string

func (f fakeSettingStore) GetByKey(_ context.Context, key string) (string, error) {
	if v, ok := f[key]; ok {
		return v, nil
	}
	return "", errors.New("not found")
}

func TestPanelLocalHTTP(t *testing.T) {
	tests := []struct {
		name     string
		settings fakeSettingStore
		wantPort int
		wantTLS  bool
	}{
		{
			name:     "默认面板监听",
			settings: fakeSettingStore{},
			wantPort: 8080,
			wantTLS:  false,
		},
		{
			name: "自定义端口",
			settings: fakeSettingStore{
				"panel.listen_port": "9080",
			},
			wantPort: 9080,
			wantTLS:  false,
		},
		{
			name: "面板自身启用 TLS",
			settings: fakeSettingStore{
				"panel.listen_port": "8443",
				"panel.tls_enabled": "true",
			},
			wantPort: 8443,
			wantTLS:  true,
		},
		{
			name: "非法端口回退默认值",
			settings: fakeSettingStore{
				"panel.listen_port": "not-a-port",
			},
			wantPort: 8080,
			wantTLS:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Handler{settingSvc: tt.settings}
			port, useTLS := h.panelLocalHTTP(context.Background())
			if port != tt.wantPort || useTLS != tt.wantTLS {
				t.Fatalf("panelLocalHTTP() = (%d, %v), want (%d, %v)", port, useTLS, tt.wantPort, tt.wantTLS)
			}
		})
	}
}

func TestNormalizeAgentHost(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"panel.example.com", "panel.example.com"},
		{" Panel.Example.COM. ", "panel.example.com"},
		{"203.0.113.7", "203.0.113.7"},
		{"[2001:db8::1]", "2001:db8::1"},
		{"localhost", "localhost"},
		{"", ""},
		{"bad host", ""},
		{"panel.example.com; rm -rf /", ""},
		{"-bad.example.com", ""},
		{"bad-.example.com", ""},
		{"a..b", ""},
	}
	for _, tt := range tests {
		if got := normalizeAgentHost(tt.in); got != tt.want {
			t.Errorf("normalizeAgentHost(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestStripPortHandlesIPv6(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"panel.example.com:443", "panel.example.com"},
		{"panel.example.com", "panel.example.com"},
		{"[2001:db8::1]:18443", "2001:db8::1"},
		{"[2001:db8::1]", "2001:db8::1"},
	}
	for _, tt := range tests {
		if got := stripPort(tt.in); got != tt.want {
			t.Errorf("stripPort(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestResolveAgentGRPCHost(t *testing.T) {
	reachable := func(context.Context, string, int) bool { return true }
	unreachable := func(context.Context, string, int) bool { return false }
	detectIP := func(context.Context, time.Duration) (string, string, error) {
		return "203.0.113.10", "", nil
	}
	detectFail := func(context.Context, time.Duration) (string, string, error) {
		return "", "", errors.New("no network")
	}

	tests := []struct {
		name      string
		panelHost string
		probe     func(context.Context, string, int) bool
		detect    func(context.Context, time.Duration) (string, string, error)
		want      string
	}{
		{
			name:      "IP 字面量直接使用",
			panelHost: "203.0.113.7",
			probe:     unreachable,
			detect:    detectFail,
			want:      "203.0.113.7",
		},
		{
			name:      "域名可直接提供 gRPC（直连或四层 CDN）",
			panelHost: "panel.example.com",
			probe:     reachable,
			detect:    detectFail,
			want:      "panel.example.com",
		},
		{
			name:      "普通 CDN 不转发 gRPC，改用源站公网 IP",
			panelHost: "cdn.example.com",
			probe:     unreachable,
			detect:    detectIP,
			want:      "203.0.113.10",
		},
		{
			name:      "探测失败时保持原地址",
			panelHost: "cdn.example.com",
			probe:     unreachable,
			detect:    detectFail,
			want:      "cdn.example.com",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveAgentGRPCHost(context.Background(), tt.panelHost, 18443, tt.probe, tt.detect)
			if got != tt.want {
				t.Fatalf("resolveAgentGRPCHost() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetAgentOriginReturnsDetectedOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{
		detectPublicIP: func(context.Context, time.Duration) (string, string, error) {
			return "203.0.113.10", "", nil
		},
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/agent/origin?grpc_port=18443", nil)
	c.Request.Host = "cdn.example.com"
	h.GetAgentOrigin(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
	var body struct {
		Code string `json:"code"`
		Data struct {
			GRPCHost string `json:"grpc_host"`
			GRPCPort int    `json:"grpc_port"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if body.Data.GRPCHost != "203.0.113.10" || body.Data.GRPCPort != 18443 {
		t.Fatalf("unexpected origin response: %+v", body)
	}
}
