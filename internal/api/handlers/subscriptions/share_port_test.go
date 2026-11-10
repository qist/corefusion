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
