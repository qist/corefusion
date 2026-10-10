package clientauth

import (
	"context"
	"testing"
)

// fakeSettingStore 是 SettingStore 的内存实现，只用于解析充值链路的门户基址。
type fakeSettingStore struct{ m map[string]string }

func (f fakeSettingStore) GetByKey(_ context.Context, key string) (string, error) {
	return f.m[key], nil
}

// TestPortalBaseURL 锁定充值 return_url / notify_url 的**域名**来源：
// 优先 oauth.callback_domain（门户回调域名），绝不把面板地址下发给支付平台。
func TestPortalBaseURL(t *testing.T) {
	cases := []struct {
		name     string
		settings map[string]string
		want     string
	}{
		{
			name:     "门户回调域名（现网 p.swsixtwo.top）",
			settings: map[string]string{"oauth.callback_domain": "p.swsixtwo.top"},
			want:     "https://p.swsixtwo.top",
		},
		{
			name:     "自带 scheme 且带尾斜杠 → 归一化",
			settings: map[string]string{"oauth.callback_domain": "http://p.example.com/"},
			want:     "http://p.example.com",
		},
		{
			name:     "未配置门户域名 → 回退 panel.external_url",
			settings: map[string]string{"panel.external_url": "https://panel.example.com"},
			want:     "https://panel.example.com",
		},
		{
			name:     "都未配置 → localhost 兜底",
			settings: map[string]string{},
			want:     "http://localhost:8080",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &CommerceService{settingSvc: fakeSettingStore{m: tc.settings}}
			if got := s.portalBaseURL(context.Background()); got != tc.want {
				t.Fatalf("portalBaseURL() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestPortalReturnURL 锁定"充值 return_url 的路径"规则：
// 与 proxy portal_forward 一致 —— 有独立门户端口 → 根路径 ""，否则 "/p"。
func TestPortalReturnURL(t *testing.T) {
	cases := []struct {
		name     string
		settings map[string]string
		want     string
	}{
		{
			name: "门户回调域名 + 独立门户端口 → 根路径（现网 p.swsixtwo.top）",
			settings: map[string]string{
				"oauth.callback_domain": "p.swsixtwo.top",
				"panel.portal_port":     "8082",
			},
			want: "https://p.swsixtwo.top",
		},
		{
			name: "门户回调域名 + 无独立端口 → /p",
			settings: map[string]string{
				"oauth.callback_domain": "p.example.com",
				"panel.portal_port":     "0",
			},
			want: "https://p.example.com/p",
		},
		{
			name: "未配置门户域名 → 回退 panel.external_url + /p",
			settings: map[string]string{
				"panel.external_url": "https://panel.example.com",
				"panel.portal_port":  "0",
			},
			want: "https://panel.example.com/p",
		},
		{
			name:     "都未配置 → localhost 兜底",
			settings: map[string]string{},
			want:     "http://localhost:8080/p",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &CommerceService{settingSvc: fakeSettingStore{m: tc.settings}}
			if got := s.portalReturnURL(context.Background()); got != tc.want {
				t.Fatalf("portalReturnURL() = %q, want %q", got, tc.want)
			}
		})
	}
}
