package subscriptions

import "testing"

// 「链接用哪个端口」必须与界面上选择的端口模式一致。
// 界面（xray / sing-box 一致）：
//   direct  → 端口模式：使用配置端口(域名里配的) / 使用入站端口
//   cf_domain / cf_ip → 没有端口模式选项，用 CDN 条目的端口
//   custom  → 端口模式：使用自定义端口 / 使用入站端口
func TestResolveHostPortHonorsPortMode(t *testing.T) {
	const listenPort = 20001
	domains := []shareDomainEntry{
		{Type: "direct", Host: "direct.example.com", Port: 443, Ports: []int{8443}},
		{Type: "cf_domain", Host: "cdn.example.com", Port: 2096, Ports: []int{2096}},
		{Type: "cf_ip", Host: "1.2.3.4", Port: 2053, Ports: []int{2053}},
	}

	cases := []struct {
		name     string
		ad       map[string]interface{}
		wantHost string
		wantPort int
	}{
		{
			name:     "direct + 使用配置端口 → 域名配置的端口",
			ad:       map[string]interface{}{"type": "direct", "directDomain": "direct.example.com", "directPortMode": "config"},
			wantHost: "direct.example.com",
			wantPort: 8443,
		},
		{
			name:     "direct + 使用入站端口 → 入站监听端口",
			ad:       map[string]interface{}{"type": "direct", "directDomain": "direct.example.com", "directPortMode": "inbound"},
			wantHost: "direct.example.com",
			wantPort: listenPort,
		},
		{
			// 界面上 cf_* 没有端口模式选项；就算 direct 下选过 inbound 也不该串过来
			name:     "cf_domain（残留 inbound 模式）→ 仍用 CDN 条目端口",
			ad:       map[string]interface{}{"type": "cf_domain", "cfDomain": "cdn.example.com", "directPortMode": "inbound"},
			wantHost: "cdn.example.com",
			wantPort: 2096,
		},
		{
			name:     "cf_ip（残留 inbound 模式）→ 仍用 CDN 条目端口",
			ad:       map[string]interface{}{"type": "cf_ip", "cfIP": "1.2.3.4", "directPortMode": "inbound"},
			wantHost: "1.2.3.4",
			wantPort: 2053,
		},
		{
			name:     "custom + 使用自定义端口",
			ad:       map[string]interface{}{"type": "custom", "customDomain": "www.example.com", "customPortMode": "custom", "customPort": float64(9633)},
			wantHost: "www.example.com",
			wantPort: 9633,
		},
		{
			// 修复点：此前完全忽略 customPortMode，选了「使用入站端口」也不生效
			name:     "custom + 使用入站端口 → 入站监听端口",
			ad:       map[string]interface{}{"type": "custom", "customDomain": "www.example.com", "customPortMode": "inbound", "customPort": float64(9633)},
			wantHost: "www.example.com",
			wantPort: listenPort,
		},
	}

	for _, tc := range cases {
		tc.ad["enabled"] = true // resolveHostPort 先判断 enabled，未启用时回退监听地址
		host, port := resolveHostPort(map[string]interface{}{"_accessDomain": tc.ad}, "0.0.0.0", listenPort, domains)
		if host != tc.wantHost || port != tc.wantPort {
			t.Errorf("%s: got %s:%d, want %s:%d", tc.name, host, port, tc.wantHost, tc.wantPort)
		}
	}
}
