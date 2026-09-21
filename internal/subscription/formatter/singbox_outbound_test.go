package formatter

import (
	"encoding/json"
	"fmt"
	"strings"
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

// 裸 TCP 不许出现 transport（sing-box 没有 tcp/raw 这个 type，写了直接 FATAL）；
// 但**裸 TCP + HTTP 伪装**必须写成 transport.type=http —— 以前整段跳过，伪装参数
// 被丢掉，客户端按裸 TCP 连 → 连不上（服务端在等 HTTP 请求头）。
func TestSingBoxTransportForBareTCPAndHTTPDisguise(t *testing.T) {
	// 裸 TCP：不写 transport
	bare := renderFirstOutbound(t, Node{
		Name: "v", Protocol: "vless", Host: "1.2.3.4", Port: 443,
		UUID: "11111111-1111-1111-1111-111111111111", Transport: "raw",
	})
	if tr, ok := bare["transport"]; ok {
		t.Errorf("裸 TCP 不该有 transport: %v", tr)
	}

	// HTTP 伪装：必须映射成 transport.type=http（带 path / host / headers）
	disguised := renderFirstOutbound(t, Node{
		Name: "v", Protocol: "vless", Host: "1.2.3.4", Port: 443,
		UUID: "11111111-1111-1111-1111-111111111111", Transport: "raw",
		Extra: map[string]interface{}{
			"http-opts": map[string]interface{}{
				"path":    []string{"/"},
				"headers": map[string]interface{}{"Host": []string{"www.baidu.com"}},
			},
		},
	})
	tr, _ := disguised["transport"].(map[string]interface{})
	if tr == nil || tr["type"] != "http" {
		t.Fatalf("HTTP 伪装应写成 transport.type=http, got %v", disguised["transport"])
	}
	if tr["path"] != "/" {
		t.Errorf("path = %v, want /", tr["path"])
	}
	if got := fmt.Sprint(tr["host"]); got != "[www.baidu.com]" {
		t.Errorf("host = %v, want [www.baidu.com]", tr["host"])
	}
	if _, ok := tr["headers"]; !ok {
		t.Errorf("headers 应保留: %v", tr)
	}

	// 真传输照旧
	ws := renderFirstOutbound(t, Node{
		Name: "w", Protocol: "vless", Host: "1.2.3.4", Port: 443,
		UUID: "11111111-1111-1111-1111-111111111111", Transport: "ws",
		Extra: map[string]interface{}{"ws-opts": map[string]interface{}{"path": "/ws"}},
	})
	tr, _ = ws["transport"].(map[string]interface{})
	if tr == nil || tr["type"] != "ws" || tr["path"] != "/ws" {
		t.Errorf("ws transport 不对: %v", ws["transport"])
	}
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

// hysteria v1 与 v2 的出站字段口径不同：v1 的 obfs 是**字符串**、凭据叫 auth_str、
// up_mbps/down_mbps 必填；v2 的 obfs 才是 {type,password} 对象（写错会报
// "cannot unmarshal object into Go struct field Hysteria.obfs"）。
func TestSingBoxFormatterHysteria1(t *testing.T) {
	ob := renderFirstOutbound(t, Node{
		Name: "hy1", Protocol: "hysteria", Host: "1.2.3.4", Port: 36712,
		Extra: map[string]interface{}{
			"auth-str": "AUTH", "obfs": "obfs1", "up": "100 Mbps", "down": "200 Mbps",
		},
	})
	if ob["auth_str"] != "AUTH" {
		t.Errorf("v1 凭据字段应为 auth_str: %v", ob["auth_str"])
	}
	if ob["obfs"] != "obfs1" {
		t.Errorf("v1 的 obfs 必须是字符串: %#v", ob["obfs"])
	}
	if ob["up_mbps"] != float64(100) || ob["down_mbps"] != float64(200) {
		t.Errorf("v1 带宽: up=%v down=%v", ob["up_mbps"], ob["down_mbps"])
	}
}

// anytls 有官方 URI 规范；无法用 URI 表达的协议必须返回空 —— 返回
// `xxx://host:port#name` 这种没有凭据的链接会被客户端导入成永远连不上的坏节点。
func TestFormatURIAnyTLSAndUnknown(t *testing.T) {
	uri := formatURI(Node{
		Name: "n", Protocol: "anytls", Host: "1.2.3.4", Port: 8443, Password: "P@ss word",
		ServerName: "a.example.com", Extra: map[string]interface{}{"skip-cert-verify": true},
	})
	for _, want := range []string{"anytls://P%40ss%20word@1.2.3.4:8443/?", "sni=a.example.com", "insecure=1"} {
		if !strings.Contains(uri, want) {
			t.Errorf("anytls 链接缺 %q: %s", want, uri)
		}
	}
	for _, proto := range []string{"wireguard", "ssr", "snell", "ssh", "mieru"} {
		if got := formatURI(Node{Protocol: proto, Host: "1.2.3.4", Port: 51820}); got != "" {
			t.Errorf("%s 没有标准 URI，应返回空: %s", proto, got)
		}
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
