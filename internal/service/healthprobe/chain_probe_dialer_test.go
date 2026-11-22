package healthprobe

import (
	"fmt"
	"testing"
)

func chainTestExitProxy() map[string]interface{} {
	return map[string]interface{}{
		"name":   "office",
		"type":   "vmess",
		"server": "exit.example.com",
		"port":   float64(443),
		"uuid":   "exit-uuid",
	}
}

func chainTestTargets() []ChainTarget {
	return []ChainTarget{
		{
			Protocol: "vless",
			Host:     "cdn-a.example.com",
			Port:     443,
			Proxy: map[string]interface{}{
				"type": "vless", "server": "cdn-a.example.com", "port": float64(443), "uuid": "a",
			},
		},
		{
			Protocol: "vless",
			Host:     "104.16.0.1",
			Port:     443,
			Proxy: map[string]interface{}{
				"type": "vless", "server": "104.16.0.1", "port": float64(443), "uuid": "a",
			},
		},
	}
}

// 链式探测必须把目标出站挂在出口出站后面（xray: sockopt.dialerProxy），
// 并且每个目标有自己的 socks 入口 + 路由规则。只从出口直连 probe_url 是错的：
// 探针入站永远收不到连接，探测结果与"链路是否可用"无关。
func TestBuildChainProbeXrayConfigWiresDialerProxy(t *testing.T) {
	targets := chainTestTargets()
	conf, err := buildChainProbeConfig("xray", chainTestExitProxy(), targets, []int{0, 1}, []int{20001, 20002})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	inbounds, _ := conf["inbounds"].([]interface{})
	if len(inbounds) != 2 {
		t.Fatalf("want 2 socks inbounds, got %d", len(inbounds))
	}
	if got := inbounds[0].(map[string]interface{})["port"]; got != 20001 {
		t.Fatalf("inbound port = %v, want 20001", got)
	}

	outbounds, _ := conf["outbounds"].([]interface{})
	if len(outbounds) != 3 {
		t.Fatalf("want 2 targets + 1 exit, got %d", len(outbounds))
	}
	for i := 0; i < 2; i++ {
		ob, _ := outbounds[i].(map[string]interface{})
		if ob["tag"] != fmt.Sprintf("probe-target-%d", i) {
			t.Fatalf("outbound %d tag = %v", i, ob["tag"])
		}
		ss, _ := ob["streamSettings"].(map[string]interface{})
		sockopt, _ := ss["sockopt"].(map[string]interface{})
		if sockopt["dialerProxy"] != chainProbeExitTag {
			t.Fatalf("target %d not chained to exit: %#v", i, ss)
		}
	}
	exitOB, _ := outbounds[2].(map[string]interface{})
	if exitOB["tag"] != chainProbeExitTag {
		t.Fatalf("exit outbound tag = %v", exitOB["tag"])
	}

	routing, _ := conf["routing"].(map[string]interface{})
	rules, _ := routing["rules"].([]interface{})
	if len(rules) != 2 {
		t.Fatalf("want 2 routing rules, got %d", len(rules))
	}
	rule0, _ := rules[0].(map[string]interface{})
	if rule0["outboundTag"] != "probe-target-0" {
		t.Fatalf("rule0 = %#v", rule0)
	}
}

// sing-box 走 detour：目标出站的底层连接经出口拨号。
func TestBuildChainProbeSingboxConfigUsesDetour(t *testing.T) {
	conf, err := buildChainProbeConfig("singbox", chainTestExitProxy(), chainTestTargets(), []int{0, 1}, []int{20011, 20012})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	outs, _ := conf["outbounds"].([]interface{})
	if len(outs) != 3 {
		t.Fatalf("want 3 outbounds, got %d", len(outs))
	}
	for i := 0; i < 2; i++ {
		ob, _ := outs[i].(map[string]interface{})
		if ob["detour"] != chainProbeExitTag {
			t.Fatalf("target %d detour = %v, want %s", i, ob["detour"], chainProbeExitTag)
		}
	}
	exitOB, _ := outs[2].(map[string]interface{})
	if _, has := exitOB["detour"]; has {
		t.Fatalf("exit outbound must not have detour: %#v", exitOB)
	}
	route, _ := conf["route"].(map[string]interface{})
	rules, _ := route["rules"].([]interface{})
	if len(rules) != 2 {
		t.Fatalf("want 2 route rules, got %d", len(rules))
	}
	if route["final"] != chainProbeExitTag {
		t.Fatalf("route.final = %v", route["final"])
	}
}

// 链接转换器产出的 alpn 是 []string，出站转换层按 []interface{} 读；
// 不做归一化会静默丢掉 ALPN（gRPC over TLS 必须带 h2）和 gRPC serviceName。
func TestBuildChainProbeXrayConfigKeepsALPN(t *testing.T) {
	targets := []ChainTarget{{
		Protocol: "vless",
		Host:     "cdn.example.com",
		Port:     443,
		Proxy: map[string]interface{}{
			"type": "vless", "server": "cdn.example.com", "port": float64(443), "uuid": "a",
			"tls": true, "servername": "real.example.com", "network": "grpc",
			"alpn":      []string{"h2", "http/1.1"},
			"grpc-opts": map[string]interface{}{"grpc-service-name": "svc"},
		},
	}}
	conf, err := buildChainProbeConfig("xray", chainTestExitProxy(), targets, []int{0}, []int{20031})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	ob := conf["outbounds"].([]interface{})[0].(map[string]interface{})
	ss := ob["streamSettings"].(map[string]interface{})
	ts, _ := ss["tlsSettings"].(map[string]interface{})
	if ts == nil || ts["serverName"] != "real.example.com" {
		t.Fatalf("tlsSettings.serverName = %v", ss["tlsSettings"])
	}
	alpn, _ := ts["alpn"].([]interface{})
	if len(alpn) != 2 || alpn[0] != "h2" {
		t.Fatalf("alpn lost: %#v", ts["alpn"])
	}
	grpc, _ := ss["grpcSettings"].(map[string]interface{})
	if grpc["serviceName"] != "svc" {
		t.Fatalf("grpc serviceName lost: %#v", grpc)
	}
}

// 核心不会的协议必须直接报错，而不是生成坏出站后整轮静默失败。
func TestBuildChainProbeConfigRejectsUnsupportedProtocol(t *testing.T) {
	targets := []ChainTarget{{
		Protocol: "hysteria2",
		Host:     "hy2.example.com",
		Proxy:    map[string]interface{}{"type": "hysteria2", "server": "hy2.example.com", "port": float64(443)},
	}}
	if _, err := buildChainProbeConfig("xray", chainTestExitProxy(), targets, []int{0}, []int{20021}); err == nil {
		t.Fatal("xray 不支持 hysteria2 出站，必须报错")
	}
	if _, err := buildChainProbeConfig("unknown-core", chainTestExitProxy(), chainTestTargets(), []int{0}, []int{20021}); err == nil {
		t.Fatal("未知拨号核心必须报错")
	}
}

// mihomo 用 listeners[].proxy + 目标 proxy 的 dialer-proxy 组成链。
func TestBuildChainProbeMihomoConfigUsesDialerProxy(t *testing.T) {
	conf, err := buildChainProbeConfig("mihomo", chainTestExitProxy(), chainTestTargets(), []int{0, 1}, []int{20041, 20042})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	listeners, _ := conf["listeners"].([]interface{})
	if len(listeners) != 2 {
		t.Fatalf("want 2 listeners, got %d", len(listeners))
	}
	proxies, _ := conf["proxies"].([]interface{})
	if len(proxies) != 3 {
		t.Fatalf("want 2 targets + 1 exit, got %d", len(proxies))
	}
	for i := 0; i < 2; i++ {
		pm := proxies[i].(map[string]interface{})
		if pm["name"] != fmt.Sprintf("probe-target-%d", i) || pm["dialer-proxy"] != chainProbeExitTag {
			t.Fatalf("target proxy %d not chained: %#v", i, pm)
		}
		l := listeners[i].(map[string]interface{})
		if l["proxy"] != pm["name"] {
			t.Fatalf("listener %d proxy = %v", i, l["proxy"])
		}
	}
	exit := proxies[2].(map[string]interface{})
	if exit["name"] != chainProbeExitTag {
		t.Fatalf("exit proxy name = %v", exit["name"])
	}
}
