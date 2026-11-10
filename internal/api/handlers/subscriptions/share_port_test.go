package subscriptions

import (
	"context"
	"testing"

	"corefusion/internal/model/entity"
)

// 被 SNI 前置代理承载的入站（unix/abstract 监听或 acceptProxyProtocol）对外必须是
// 前置端口（默认 443）；拿它自己的 listen 端口生成链接，客户端连上去只会被重置。
func TestSharePortForProxiedInbounds(t *testing.T) {
	h := &PublicHandler{}
	ctx := context.Background()

	cases := []struct {
		name   string
		cfg    entity.InboundConfig
		config map[string]interface{}
		want   int
	}{
		{
			name: "acceptProxyProtocol on public listen",
			cfg:  entity.InboundConfig{Port: 59026, Listen: "0.0.0.0"},
			config: map[string]interface{}{"streamSettings": map[string]interface{}{
				"sockopt": map[string]interface{}{"acceptProxyProtocol": true},
			}},
			want: 443,
		},
		{
			name:   "unix socket listen",
			cfg:    entity.InboundConfig{Port: 22090, Listen: "/dev/shm/VLESSGRPC.sock,0666"},
			config: map[string]interface{}{},
			want:   443,
		},
		{
			name:   "abstract socket listen",
			cfg:    entity.InboundConfig{Port: 22554, Listen: "@vless-grpc"},
			config: map[string]interface{}{},
			want:   443,
		},
		{
			name:   "plain public inbound keeps its own port",
			cfg:    entity.InboundConfig{Port: 11995, Listen: "0.0.0.0"},
			config: map[string]interface{}{"streamSettings": map[string]interface{}{"sockopt": map[string]interface{}{}}},
			want:   11995,
		},
	}
	for _, c := range cases {
		if got := h.sharePort(ctx, c.cfg, c.config); got != c.want {
			t.Errorf("%s: sharePort = %d, want %d", c.name, got, c.want)
		}
	}
}

// 访问域名只负责换地址：端口必须由承载方式决定。
//   - SNI 前置承载 → 前置端口（池条目写 8443 也不能用）；
//   - UDP（hy2/tuic）/ 直连 TCP → 自己的监听端口；
//   - direct / custom 显式端口 → 保留（nginx 8443 反代）。
func TestShareAddressPortFollowsTransport(t *testing.T) {
	h := &PublicHandler{}
	ctx := context.Background()

	// 池里故意配一条非 443 的 CDN 条目（模拟"坏条目"）。
	pools := []shareDomainEntry{
		{Type: "cf_domain", Host: "cdn.example.com", Port: 8443, Ports: []int{8443}},
		{Type: "direct", Host: "direct.example.com", Port: 8443, Ports: []int{8443}},
	}

	cfDomain := func(domain string) map[string]interface{} {
		return map[string]interface{}{"_accessDomain": map[string]interface{}{
			"enabled": true, "type": "cf_domain", "cfDomain": domain,
		}}
	}

	cases := []struct {
		name     string
		cfg      entity.InboundConfig
		config   map[string]interface{}
		wantHost string
		wantPort int
	}{
		{
			// nginx 按 SNI 分流到 unix socket：池条目端口不可用，必须 443。
			name:     "SNI backed inbound ignores pool port",
			cfg:      entity.InboundConfig{Port: 51878, Listen: "/dev/shm/VLESSGRPCREALITY.sock,0666"},
			config:   cfDomain("cdn.example.com"),
			wantHost: "cdn.example.com",
			wantPort: 443,
		},
		{
			// 直连 TCP 入站（11995）没配访问域名：用节点地址 + 自己的监听端口。
			name:     "plain tcp inbound without access domain keeps listen port",
			cfg:      entity.InboundConfig{Port: 11995, Listen: "0.0.0.0"},
			config:   map[string]interface{}{},
			wantHost: "127.0.0.1",
			wantPort: 11995,
		},
		{
			// nginx 在 443 做 HTTP 反代（非 SNI 分流）的入站：对外端口是 CDN 条目的 443，
			// 不能退回它自己的 7779（外部没监听/没放行）。
			name:     "cdn proxied tcp inbound keeps pool port",
			cfg:      entity.InboundConfig{Port: 7779, Listen: "0.0.0.0"},
			config:   cfDomain("cf.877774.xyz"),
			wantHost: "cf.877774.xyz",
			wantPort: 443,
		},
		{
			// hy2 是 UDP：ssl_preread 不分流，端口只能是自己的 4443。
			name:     "hysteria2 keeps listen port",
			cfg:      entity.InboundConfig{Port: 4443, Listen: "0.0.0.0", Protocol: "hysteria2"},
			config:   cfDomain("cdn.example.com"),
			wantHost: "cdn.example.com",
			wantPort: 4443,
		},
		{
			name:     "tuic keeps listen port",
			cfg:      entity.InboundConfig{Port: 50000, Listen: "0.0.0.0", Protocol: "tuic"},
			config:   cfDomain("cdn.example.com"),
			wantHost: "cdn.example.com",
			wantPort: 50000,
		},
		{
			// direct 是用户显式选择，端口模式 config → 用池里配的端口。
			name: "direct explicit port wins",
			cfg:  entity.InboundConfig{Port: 11995, Listen: "0.0.0.0"},
			config: map[string]interface{}{"_accessDomain": map[string]interface{}{
				"enabled": true, "type": "direct",
				"directDomain": "direct.example.com", "directPortMode": "config",
			}},
			wantHost: "direct.example.com",
			wantPort: 8443,
		},
		{
			// direct + 跟随入站端口 → listenPort。
			name: "direct follows inbound port",
			cfg:  entity.InboundConfig{Port: 11995, Listen: "0.0.0.0"},
			config: map[string]interface{}{"_accessDomain": map[string]interface{}{
				"enabled": true, "type": "direct",
				"directDomain": "direct.example.com", "directPortMode": "inbound",
			}},
			wantHost: "direct.example.com",
			wantPort: 11995,
		},
	}

	for _, c := range cases {
		host, port := h.shareAddress(ctx, c.cfg, c.config, nil, pools)
		if host != c.wantHost || port != c.wantPort {
			t.Errorf("%s: got %s:%d, want %s:%d", c.name, host, port, c.wantHost, c.wantPort)
		}
	}
}
