package subscriptions

import (
	"context"
	"testing"

	"corefusion/internal/model/entity"
)

// 接入域名里**没选**具体域名/IP 时，绝不能从面板级分享域名列表里随机取一个：
// 曾用 pickByType/rand 随机选，同一个入站每次渲染的 server 都不同，而且与界面上下拉的
// 选择完全无关（用户报的 "server 不对 / 下拉对应的没有"）。
func TestResolveAccessDomainRequiresExplicitSelection(t *testing.T) {
	const listenPort = 20001
	domains := []shareDomainEntry{
		{Type: "direct", Host: "direct.example.com", Port: 443, Ports: []int{443}},
		{Type: "cf_domain", Host: "cdn.example.com", Port: 2096, Ports: []int{2096}},
		{Type: "cf_ip", Host: "1.2.3.4", Port: 2053, Ports: []int{2053}},
	}

	empty := []map[string]interface{}{
		{"type": "direct", "directDomain": ""},
		{"type": "cf_domain", "cfDomain": ""},
		{"type": "cf_ip", "cfIP": ""},
		{"type": "custom", "customDomain": ""},
		{"type": "direct"}, // 字段缺失
		{"type": "unknown-type"},
	}
	for _, ad := range empty {
		ad["enabled"] = true
		cfg := map[string]interface{}{"_accessDomain": ad}
		host, port, ok := resolveAccessDomain(cfg, listenPort, domains)
		if ok || host != "" || port != 0 {
			t.Errorf("ad=%v 应判为未选择, got %q:%d ok=%v", ad, host, port, ok)
		}
		// resolveHostPort 必须回退监听地址/入站端口（而不是面板域名）
		if h, p := resolveHostPort(cfg, "0.0.0.0", listenPort, domains); h != "127.0.0.1" || p != listenPort {
			t.Errorf("ad=%v 应回退 127.0.0.1:%d, got %s:%d", ad, listenPort, h, p)
		}
	}

	// 没有 _accessDomain / 未启用 → 同样不取面板域名
	if _, _, ok := resolveAccessDomain(map[string]interface{}{}, listenPort, domains); ok {
		t.Error("缺少 _accessDomain 时不应解析出地址")
	}
	if _, _, ok := resolveAccessDomain(map[string]interface{}{"_accessDomain": map[string]interface{}{
		"enabled": false, "type": "direct", "directDomain": "direct.example.com",
	}}, listenPort, domains); ok {
		t.Error("未启用接入域名时不应解析出地址")
	}

	// 明确选了值的仍然照常解析
	if host, port, ok := resolveAccessDomain(map[string]interface{}{"_accessDomain": map[string]interface{}{
		"enabled": true, "type": "direct", "directDomain": "direct.example.com", "directPortMode": "inbound",
	}}, listenPort, domains); !ok || host != "direct.example.com" || port != listenPort {
		t.Errorf("选中的域名应正常解析, got %s:%d ok=%v", host, port, ok)
	}
}

// 分享地址的兜底顺序：已选的接入域名 → 节点公网/内网 IP → 监听地址。
// 没有 nodeServerRepo（等价于节点没登记 IP）时直到底，而不是拿面板域名顶替。
func TestShareAddressFallsBackToListenAddress(t *testing.T) {
	h := &PublicHandler{}
	cfg := entity.InboundConfig{Listen: "0.0.0.0", Port: 20001}
	host, port := h.shareAddress(context.Background(), cfg, map[string]interface{}{
		"_accessDomain": map[string]interface{}{"enabled": true, "type": "direct", "directDomain": ""},
	}, []shareDomainEntry{{Type: "direct", Host: "direct.example.com", Port: 443, Ports: []int{443}}})
	if host != "127.0.0.1" || port != 20001 {
		t.Errorf("got %s:%d, want 127.0.0.1:20001", host, port)
	}
}

// 「链接用哪个端口」必须与界面上选择的端口模式一致。
// 界面（xray / sing-box 一致）：
//
//	direct  → 端口模式：使用配置端口(域名里配的) / 使用入站端口
//	cf_domain / cf_ip → 没有端口模式选项，用 CDN 条目的端口
//	custom  → 端口模式：使用自定义端口 / 使用入站端口
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

// 节点自身对外地址的优先顺序：**公网 IP → 主机名 → 内网 IP**。
//
// 回归点：此前 shareAddress 只取「公网 IP → 内网 IP」，**跳过了主机名** ✗ ——
// 节点只填了 hostname（如 192.168.2.50）、public_ip/internal_ip 留空时，
// 会一路落到监听地址兜底（0.0.0.0 → 127.0.0.1），下发给客户端的 server 变成
// 客户端本机地址（真机现象：test-agent-50 的入站 server 恒为 127.0.0.1 ✗）。
func TestPreferNodeAddressPriority(t *testing.T) {
	cases := []struct {
		name string
		node entity.NodeServer
		want string
	}{
		{"三者都有 → 公网 IP", entity.NodeServer{PublicIP: "1.2.3.4", Hostname: "h.example.com", InternalIP: "10.0.0.1"}, "1.2.3.4"},
		{"无公网 → 主机名", entity.NodeServer{Hostname: "192.168.2.50", InternalIP: "10.0.0.1"}, "192.168.2.50"},
		{"只有内网 → 内网 IP", entity.NodeServer{InternalIP: "10.0.0.1"}, "10.0.0.1"},
		{"全空 → 空串", entity.NodeServer{}, ""},
	}
	for _, tc := range cases {
		if got := preferNodeAddress(tc.node); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

// 端到端（无 repo 之外的依赖）：没有 nodeServerRepo 时 shareAddress 仍回退监听地址；
// 有节点主机名时，主机名必须优先于监听地址兜底 —— 这条以前会得到 127.0.0.1。
func TestShareAddressUsesHostnameBeforeListenFallback(t *testing.T) {
	// 直接验证优先链本身（repo 为具体类型，不便注入；preferNodeAddress 是同一逻辑的纯函数）
	node := entity.NodeServer{Hostname: "192.168.2.50"}
	if got := preferNodeAddress(node); got != "192.168.2.50" {
		t.Fatalf("主机名应被采用，got %q", got)
	}
	// 监听兜底仍只对 0.0.0.0/:: 生效
	if got := fallbackHost("0.0.0.0"); got != "127.0.0.1" {
		t.Fatalf("0.0.0.0 应兜底 127.0.0.1，got %q", got)
	}
}
