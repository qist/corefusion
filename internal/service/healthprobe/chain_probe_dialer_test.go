package healthprobe

import (
	"encoding/json"
	"fmt"
	"strings"
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
	conf, err := buildChainProbeConfig("xray", chainTestExitProxy(), targets, []int{0, 1}, []int{20001, 20002}, "")
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
	conf, err := buildChainProbeConfig("singbox", chainTestExitProxy(), chainTestTargets(), []int{0, 1}, []int{20011, 20012}, "")
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
	conf, err := buildChainProbeConfig("xray", chainTestExitProxy(), targets, []int{0}, []int{20031}, "")
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
	if _, err := buildChainProbeConfig("xray", chainTestExitProxy(), targets, []int{0}, []int{20021}, ""); err == nil {
		t.Fatal("xray 不支持 hysteria2 出站，必须报错")
	}
	if _, err := buildChainProbeConfig("unknown-core", chainTestExitProxy(), chainTestTargets(), []int{0}, []int{20021}, ""); err == nil {
		t.Fatal("未知拨号核心必须报错")
	}
}

// mihomo 用 listeners[].proxy + 目标 proxy 的 dialer-proxy 组成链。
func TestBuildChainProbeMihomoConfigUsesDialerProxy(t *testing.T) {
	conf, err := buildChainProbeConfig("mihomo", chainTestExitProxy(), chainTestTargets(), []int{0, 1}, []int{20041, 20042}, "")
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

// 模板模式（exitProxy 为空）：目标出站自己就是客户端配置，不能再挂
// dialerProxy/detour/dialer-proxy，否则等于又绕一跳。
func TestBuildChainProbeConfigTemplateMode(t *testing.T) {
	targets := chainTestTargets()
	valid := []int{0, 1}
	ports := []int{19001, 19002}
	for _, dialer := range []string{"xray", "singbox", "mihomo"} {
		cfg, err := buildChainProbeConfig(dialer, nil, targets, valid, ports, "")
		if err != nil {
			t.Fatalf("%s build: %v", dialer, err)
		}
		raw, err := json.Marshal(cfg)
		if err != nil {
			t.Fatalf("%s marshal: %v", dialer, err)
		}
		s := string(raw)
		for _, bad := range []string{"dialerProxy", `"detour"`, "dialer-proxy"} {
			if strings.Contains(s, bad) {
				t.Fatalf("%s 模板模式不应挂出口（发现 %s）: %s", dialer, bad, s)
			}
		}
		if dialer == "xray" {
			obs, _ := cfg["outbounds"].([]interface{})
			if len(obs) != 2 {
				t.Fatalf("xray 模板模式 want 2 outbounds, got %d", len(obs))
			}
		}
		if dialer == "mihomo" {
			proxies, _ := cfg["proxies"].([]interface{})
			if len(proxies) != 2 {
				t.Fatalf("mihomo 模板模式 want 2 proxies, got %d", len(proxies))
			}
		}
	}
}

func chainFindByField(list []interface{}, field, want string) map[string]interface{} {
	for _, item := range list {
		m, _ := item.(map[string]interface{})
		if m == nil {
			continue
		}
		if v, _ := m[field].(string); v == want {
			return m
		}
	}
	return nil
}

// 三个核心都要能把「出口网卡」写进临时配置；有出口时只绑出口出站。
func TestChainProbeInterfaceBinding(t *testing.T) {
	ports := func(n int) []int {
		out := make([]int, n)
		for i := range out {
			out[i] = 21000 + i
		}
		return out
	}

	// xray: sockopt.interface
	conf, err := buildChainProbeConfig("xray", chainTestExitProxy(), chainTestTargets(), []int{0, 1}, ports(2), "eth0")
	if err != nil {
		t.Fatalf("xray build: %v", err)
	}
	outs, _ := conf["outbounds"].([]interface{})
	exit := chainFindByField(outs, "tag", chainProbeExitTag)
	if exit == nil {
		t.Fatal("xray 缺少出口出站")
	}
	ss, _ := exit["streamSettings"].(map[string]interface{})
	so, _ := ss["sockopt"].(map[string]interface{})
	if so == nil || so["interface"] != "eth0" {
		t.Fatalf("xray 出口未绑定网卡：%+v", exit)
	}
	if target := chainFindByField(outs, "tag", "probe-target-0"); target != nil {
		tss, _ := target["streamSettings"].(map[string]interface{})
		tso, _ := tss["sockopt"].(map[string]interface{})
		if tso != nil && tso["interface"] != nil {
			t.Fatalf("有出口时目标出站不应绑网卡：%+v", tso)
		}
	}

	// sing-box: bind_interface
	conf, err = buildChainProbeConfig("singbox", chainTestExitProxy(), chainTestTargets(), []int{0, 1}, ports(2), "eth0")
	if err != nil {
		t.Fatalf("singbox build: %v", err)
	}
	outs, _ = conf["outbounds"].([]interface{})
	if exit := chainFindByField(outs, "tag", chainProbeExitTag); exit == nil || exit["bind_interface"] != "eth0" {
		t.Fatalf("sing-box 出口未绑定网卡：%+v", exit)
	}

	// mihomo: interface-name
	conf, err = buildChainProbeConfig("mihomo", chainTestExitProxy(), chainTestTargets(), []int{0, 1}, ports(2), "eth0")
	if err != nil {
		t.Fatalf("mihomo build: %v", err)
	}
	proxies, _ := conf["proxies"].([]interface{})
	if exit := chainFindByField(proxies, "name", chainProbeExitTag); exit == nil || exit["interface-name"] != "eth0" {
		t.Fatalf("mihomo 出口未绑定网卡：%+v", exit)
	}
}

// 留空 = 不绑定，走系统默认路由。
func TestChainProbeInterfaceEmpty(t *testing.T) {
	for _, dialer := range []string{"xray", "singbox", "mihomo"} {
		conf, err := buildChainProbeConfig(dialer, chainTestExitProxy(), chainTestTargets(), []int{0}, []int{21100}, "")
		if err != nil {
			t.Fatalf("%s build: %v", dialer, err)
		}
		raw, _ := json.Marshal(conf)
		for _, bad := range []string{`"interface"`, `"bind_interface"`, `"interface-name"`} {
			if strings.Contains(string(raw), bad) {
				t.Fatalf("%s 留空时不应出现 %s：%s", dialer, bad, raw)
			}
		}
	}
}

// 模板模式（无出口）时网卡绑到目标出站上。
func TestChainProbeInterfaceTemplateMode(t *testing.T) {
	conf, err := buildChainProbeConfig("xray", nil, chainTestTargets(), []int{0}, []int{21200}, "eth1")
	if err != nil {
		t.Fatalf("xray template build: %v", err)
	}
	outs, _ := conf["outbounds"].([]interface{})
	target := chainFindByField(outs, "tag", "probe-target-0")
	if target == nil {
		t.Fatal("缺少目标出站")
	}
	ss, _ := target["streamSettings"].(map[string]interface{})
	so, _ := ss["sockopt"].(map[string]interface{})
	if so == nil || so["interface"] != "eth1" {
		t.Fatalf("模板模式目标出站未绑定网卡：%+v", target)
	}
}

func TestValidEgressInterface(t *testing.T) {
	for _, ok := range []string{"", "eth0", "ppp0", "br-lan", "wg0.1", "enp3s0f0"} {
		if !ValidEgressInterface(ok) {
			t.Fatalf("%q 应合法", ok)
		}
	}
	for _, bad := range []string{"eth 0", "eth/0", "网卡", "a\nb", "this-name-is-way-too-long-for-a-nic"} {
		if ValidEgressInterface(bad) {
			t.Fatalf("%q 应非法", bad)
		}
	}
}
