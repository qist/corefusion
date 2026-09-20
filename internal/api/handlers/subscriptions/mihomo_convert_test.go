package subscriptions

import (
	"testing"

	"corefusion/internal/subscription/formatter"
)

// mihomo 与 sing-box 的代理类型名不同，必须归一 —— 否则外部订阅里的 ss / socks5 节点
// 在 sing-box 订阅里会被当成未知协议**整条丢掉**（filterRenderedNodesForFormat 按协议名过滤）。
func TestMihomoProxyMapToNodeNormalizesType(t *testing.T) {
	cases := map[string]string{
		"ss": "shadowsocks", "ssr": "shadowsocksr",
		"socks5": "socks", "socks5h": "socks",
		"https": "http", "hy2": "hysteria2", "any-tls": "anytls",
		"vless": "vless", "anytls": "anytls",
	}
	for in, want := range cases {
		node := mihomoProxyMapToNode(map[string]interface{}{
			"type": in, "name": "n", "server": "1.2.3.4", "port": float64(443),
		})
		if node == nil {
			t.Fatalf("%s: 转换返回 nil", in)
		}
		if node.Protocol != want {
			t.Errorf("%s 归一后 = %s, want %s", in, node.Protocol, want)
		}
	}
}

// snell 的凭据字段是 psk（不是 password）；mihomo 的 trojan / hysteria / hysteria2 /
// tuic / anytls 选项里**没有 tls 字段**（协议本身恒为 TLS），映射时要补 tls，
// 否则 sni / 证书校验在后续渲染里会全部丢掉。
func TestMihomoProxyMapToNodeCredentialsAndTLS(t *testing.T) {
	snell := mihomoProxyMapToNode(map[string]interface{}{
		"type": "snell", "name": "s", "server": "1.2.3.4", "port": float64(6153), "psk": "PSK", "version": float64(4),
	})
	if snell.Password != "PSK" {
		t.Errorf("snell 的 psk 未映射成凭据: %q", snell.Password)
	}
	if snell.Extra["version"] != float64(4) {
		t.Errorf("snell version 未保留: %v", snell.Extra["version"])
	}
	for _, typ := range []string{"trojan", "hysteria", "hysteria2", "tuic", "anytls"} {
		n := mihomoProxyMapToNode(map[string]interface{}{
			"type": typ, "name": "n", "server": "1.2.3.4", "port": float64(443), "sni": "s.example.com",
		})
		if !n.TLS {
			t.Errorf("%s 应视为恒 TLS", typ)
		}
	}
}

// 这些节点混进 sing-box 订阅会让客户端拒绝加载**整份**订阅，必须整条剔除。
func TestSingboxNodeDropReason(t *testing.T) {
	if r := singboxNodeDropReason(&formatter.Node{Protocol: "vless", Transport: "xhttp"}); r == "" {
		t.Error("xhttp 传输应被剔除")
	}
	// hysteria v1 的 up_mbps/down_mbps 是必填（缺了报 "missing upload speed"）
	if r := singboxNodeDropReason(&formatter.Node{Protocol: "hysteria", Transport: "tcp"}); r == "" {
		t.Error("hysteria v1 缺上下行带宽应被剔除")
	}
	if r := singboxNodeDropReason(&formatter.Node{
		Protocol: "hysteria", Transport: "tcp",
		Extra: map[string]interface{}{"up": "100 Mbps"},
	}); r != "" {
		t.Errorf("有带宽不应被剔除: %s", r)
	}
	if r := singboxNodeDropReason(&formatter.Node{Protocol: "vless", Transport: "ws"}); r != "" {
		t.Errorf("正常节点不应被剔除: %s", r)
	}
}
