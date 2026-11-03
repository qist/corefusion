package proxy

import "testing"

// TestPickAutoStartProxy 锁定开机自启的唯一代理选择规则：
// 运行中的优先 > 已启用站点数多者 > 平局按顺序（Caddy 在前）。
// 背景：caddy/nginx 共用 80/443，绝不能因为「二进制先装/遍历顺序在前」就把
// 没配置的那个拉起来，导致真正配置的代理起不来、整站代理失联。
func TestPickAutoStartProxy(t *testing.T) {
	caddy := ProxyCaddy
	nginx := ProxyNginx

	cases := []struct {
		name string
		in   []autoStartCandidate
		want ProxyType
		ok   bool
	}{
		{name: "无候选", in: nil, ok: false},
		{name: "只有一个（caddy）", in: []autoStartCandidate{{proxyType: caddy}}, want: caddy, ok: true},
		{name: "只有一个（nginx）", in: []autoStartCandidate{{proxyType: nginx, sites: 3}}, want: nginx, ok: true},
		{
			name: "都没站点：平局按顺序（caddy）",
			in:   []autoStartCandidate{{proxyType: caddy}, {proxyType: nginx}},
			want: caddy, ok: true,
		},
		{
			name: "caddy 空配置、nginx 有站点 → 选 nginx（不能抢端口）",
			in:   []autoStartCandidate{{proxyType: caddy}, {proxyType: nginx, sites: 6}},
			want: nginx, ok: true,
		},
		{
			name: "caddy 有站点、nginx 空配置 → 选 caddy",
			in:   []autoStartCandidate{{proxyType: caddy, sites: 2}, {proxyType: nginx}},
			want: caddy, ok: true,
		},
		{
			name: "都已经在运行 → 保持已运行的 nginx，不再启动 caddy",
			in:   []autoStartCandidate{{proxyType: caddy}, {proxyType: nginx, running: true}},
			want: nginx, ok: true,
		},
		{
			name: "运行中的优先于站点数",
			in:   []autoStartCandidate{{proxyType: caddy, sites: 99}, {proxyType: nginx, sites: 1, running: true}},
			want: nginx, ok: true,
		},
	}

	for _, tc := range cases {
		got, ok := pickAutoStartProxy(tc.in)
		if ok != tc.ok {
			t.Fatalf("%s: ok=%v want %v", tc.name, ok, tc.ok)
		}
		if ok && got.proxyType != tc.want {
			t.Fatalf("%s: picked %s want %s", tc.name, got.proxyType, tc.want)
		}
	}
}
