package agent

import "testing"

// copyExitInboundTransport 的回归：出口入站的传输细节必须搬到入口侧出站的 streamSettings。
// 真机事故：入口是 sing-box、出口是 sing-box 的 gRPC 入站时，service_name 丢失 →
// 生成的出站 `transport:{type:grpc}` 没有 service_name → 永远连不上（核心不报错）。
func TestCopyExitInboundTransportFromSingbox(t *testing.T) {
	// sing-box 形状：顶层 transport
	cfg := map[string]interface{}{
		"type": "vless",
		"transport": map[string]interface{}{
			"type":         "grpc",
			"service_name": "ov2zy5xlnlmi",
		},
	}
	ss := map[string]interface{}{"network": "grpc", "security": "tls"}
	copyExitInboundTransport(ss, cfg, "grpc")
	g, _ := ss["grpcSettings"].(map[string]interface{})
	if g == nil || g["serviceName"] != "ov2zy5xlnlmi" {
		t.Fatalf("gRPC service_name 未搬运: %#v", ss)
	}
}

func TestCopyExitInboundTransportWsFromSingbox(t *testing.T) {
	cfg := map[string]interface{}{
		"type": "vless",
		"transport": map[string]interface{}{
			"type": "ws",
			"path": "/vless-ws",
			"headers": map[string]interface{}{
				"Host": "cdn.example.com",
			},
		},
	}
	ss := map[string]interface{}{"network": "ws", "security": "none"}
	copyExitInboundTransport(ss, cfg, "ws")
	w, _ := ss["wsSettings"].(map[string]interface{})
	if w == nil || w["path"] != "/vless-ws" {
		t.Fatalf("ws path 未搬运: %#v", ss)
	}
	hdrs, _ := w["headers"].(map[string]interface{})
	if hdrs == nil || hdrs["Host"] != "cdn.example.com" {
		t.Fatalf("ws Host 未搬运: %#v", ss)
	}
}

func TestCopyExitInboundTransportFromXray(t *testing.T) {
	// xray 形状：streamSettings.grpcSettings 原样搬
	cfg := map[string]interface{}{
		"protocol": "vless",
		"streamSettings": map[string]interface{}{
			"network": "grpc",
			"grpcSettings": map[string]interface{}{
				"serviceName": "abc123",
			},
		},
	}
	ss := map[string]interface{}{"network": "grpc", "security": "none"}
	copyExitInboundTransport(ss, cfg, "grpc")
	g, _ := ss["grpcSettings"].(map[string]interface{})
	if g == nil || g["serviceName"] != "abc123" {
		t.Fatalf("xray gRPC serviceName 未搬运: %#v", ss)
	}
}

// 裸 TCP（raw/tcp/空）没有传输细节，不应写任何 *Settings。
func TestCopyExitInboundTransportBareTCP(t *testing.T) {
	cfg := map[string]interface{}{"streamSettings": map[string]interface{}{"network": "raw"}}
	ss := map[string]interface{}{"network": "raw", "security": "none"}
	copyExitInboundTransport(ss, cfg, "raw")
	if len(ss) != 2 {
		t.Fatalf("裸 TCP 不应新增字段: %#v", ss)
	}
}

// vless 的 flow 必须从出口入站带进入口侧出站（面板默认 vision）——
// 丢了 flow 时出口按 vision 握手，出站不带 flow 会静默连不上（核心都不报错）。
func TestBuildLinkOutboundSettingsCarriesVlessFlow(t *testing.T) {
	settings := buildLinkOutboundSettingsWithCred("vless", "1.2.3.4", 443,
		"11111111-1111-1111-1111-111111111111", "", "xtls-rprx-vision")
	vnext, _ := settings["vnext"].([]map[string]interface{})
	if len(vnext) != 1 {
		t.Fatalf("vnext 结构不对: %#v", settings)
	}
	users, _ := vnext[0]["users"].([]map[string]interface{})
	if len(users) != 1 || users[0]["flow"] != "xtls-rprx-vision" {
		t.Fatalf("vless flow 未带进用户: %#v", settings)
	}

	// vmess 不认 flow，不应写入该字段。
	vm := buildLinkOutboundSettingsWithCred("vmess", "1.2.3.4", 443, "uuid", "", "xtls-rprx-vision")
	vv, _ := vm["vnext"].([]map[string]interface{})
	vu, _ := vv[0]["users"].([]map[string]interface{})
	if _, ok := vu[0]["flow"]; ok {
		t.Fatalf("vmess 用户不应带 flow: %#v", vm)
	}
}
