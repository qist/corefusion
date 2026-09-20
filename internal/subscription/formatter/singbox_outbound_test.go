package formatter

import (
	"encoding/json"
	"testing"
)

// renderFirstOutbound 渲染单个节点并取出第一个出站。
func renderFirstOutbound(t *testing.T, node Node) map[string]interface{} {
	t.Helper()
	data, err := (&SingBoxFormatter{}).Format([]Node{node})
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Outbounds []map[string]interface{} `json:"outbounds"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Outbounds) == 0 {
		t.Fatal("no outbounds")
	}
	return cfg.Outbounds[0]
}

// REALITY 必须写成 tls.reality（public_key / short_id）并强制 uTLS：
// 少了它客户端不会报错，而是把 REALITY 当普通 TLS 握手 → 永远连不上。
// 注意 node.TLS 可能是 false（外部节点从 mihomo 映射转换时丢了 tls），
// REALITY 本身隐含 TLS，必须照样写出 tls。
func TestSingBoxFormatterReality(t *testing.T) {
	ob := renderFirstOutbound(t, Node{
		Name: "r", Protocol: "vless", Host: "1.2.3.4", Port: 443,
		UUID: "11111111-1111-1111-1111-111111111111", ServerName: "nuvexo.com",
		Extra: map[string]interface{}{
			"reality-opts": map[string]interface{}{"public-key": "PBK", "short-id": "SID"},
		},
	})
	tls, _ := ob["tls"].(map[string]interface{})
	if tls == nil {
		t.Fatal("reality 节点必须带 tls")
	}
	reality, _ := tls["reality"].(map[string]interface{})
	if reality["public_key"] != "PBK" || reality["short_id"] != "SID" {
		t.Fatalf("reality 字段缺失: %v", tls["reality"])
	}
	utls, _ := tls["utls"].(map[string]interface{})
	if utls == nil || utls["enabled"] != true {
		t.Fatalf("reality 必须强制 uTLS（sing-box 会报 uTLS is required by reality client）: %v", tls["utls"])
	}
}

// tuic / hysteria2 / anytls 在 sing-box 里强制 TLS，外部节点转换时常丢 tls: true。
func TestSingBoxFormatterForcesTLS(t *testing.T) {
	for _, proto := range []string{"tuic", "hysteria2", "anytls"} {
		ob := renderFirstOutbound(t, Node{Name: proto, Protocol: proto, Host: "1.2.3.4", Port: 443})
		tls, _ := ob["tls"].(map[string]interface{})
		if tls == nil || tls["enabled"] != true {
			t.Errorf("%s 必须补 tls.enabled=true: %v", proto, ob["tls"])
		}
	}
}

// sing-box 没有 mixed 出站类型（实测 unknown outbound type: mixed）→ 映射成 http。
func TestSingBoxFormatterMixedToHTTP(t *testing.T) {
	ob := renderFirstOutbound(t, Node{Name: "m", Protocol: "mixed", Host: "1.2.3.4", Port: 1080, Password: "pw"})
	if ob["type"] != "http" {
		t.Fatalf("mixed 应映射为 http 出站, got %v", ob["type"])
	}
}

// snell 出站的凭据字段是 psk（不是 password），且必须有 version。
func TestSingBoxFormatterSnell(t *testing.T) {
	ob := renderFirstOutbound(t, Node{
		Name: "s", Protocol: "snell", Host: "1.2.3.4", Port: 6153, Password: "PSK",
		Extra: map[string]interface{}{"version": 4},
	})
	if ob["psk"] != "PSK" {
		t.Fatalf("snell 需要 psk, got %v", ob["psk"])
	}
	if _, has := ob["password"]; has {
		t.Fatalf("snell 不能带 password（未知字段会 FATAL）: %v", ob["password"])
	}
	if ob["version"] != float64(4) {
		t.Fatalf("snell 需要 version（缺了报 snell: missing version）, got %v", ob["version"])
	}
}

// 端口跳跃节点 port=0 → server_port 取区间首端口（不能留 0）。
func TestSingBoxFormatterPortFallback(t *testing.T) {
	ob := renderFirstOutbound(t, Node{
		Name: "h", Protocol: "hysteria2", Host: "1.2.3.4", Port: 0, Password: "p", TLS: true,
		Extra: map[string]interface{}{"ports": "10000-19999"},
	})
	if ob["server_port"] != float64(10000) {
		t.Fatalf("server_port 应取区间首端口, got %v", ob["server_port"])
	}
	ports, _ := ob["server_ports"].([]interface{})
	if len(ports) != 1 || ports[0] != "10000:19999" {
		t.Fatalf("server_ports 缺失: %v", ob["server_ports"])
	}
}

// 外部节点的 alpn 经 JSON 反序列化是 []interface{}、指纹写在 client-fingerprint
// → 之前 .([]string) / .(string) 断言会静默丢字段。
func TestSingBoxFormatterTolerantALPNAndFingerprint(t *testing.T) {
	ob := renderFirstOutbound(t, Node{
		Name: "x", Protocol: "vless", Host: "1.2.3.4", Port: 443, TLS: true,
		UUID: "11111111-1111-1111-1111-111111111111",
		Extra: map[string]interface{}{
			"alpn":               []interface{}{"h2", "http/1.1"},
			"client-fingerprint": "chrome",
		},
	})
	tls, _ := ob["tls"].(map[string]interface{})
	alpn, _ := tls["alpn"].([]interface{})
	if len(alpn) != 2 || alpn[0] != "h2" {
		t.Fatalf("alpn 丢失: %v", tls["alpn"])
	}
	utls, _ := tls["utls"].(map[string]interface{})
	if utls["fingerprint"] != "chrome" {
		t.Fatalf("client-fingerprint 未映射成 utls.fingerprint: %v", tls["utls"])
	}
}
