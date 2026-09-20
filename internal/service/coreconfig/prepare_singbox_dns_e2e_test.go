package coreconfig

import (
	"context"
	"encoding/json"
	"os"
	"testing"
)

// 整条生成链路的回归测试：把“旧格式 DNS + 新增顶层段落 + 新路由规则字段”喂给
// PrepareFullConfig，断言产物满足 sing-box 1.14+ 的严格解码要求。
//
// 期望值均由真实二进制 `sing-box check` 验证得到（例如 dns.servers[].address、
// 规则级 strategy/domain_resolver、顶层 dns.fakeip/hosts/domain_matcher 都会 FATAL）。
//
// 需要人工用二进制复核时：
//
//	CF_DUMP_TEST_CONFIG=/tmp/prepared.json go test ./internal/service/coreconfig/ -run TestPrepareFullConfig
//	sing-box check -c /tmp/prepared.json
func TestPrepareFullConfigSingboxLegacyDNSIsMigrated(t *testing.T) {
	full := map[string]interface{}{
		"log": map[string]interface{}{"level": "info", "timestamp": true},
		"dns": map[string]interface{}{
			"domain_matcher":    "adguard", // 已移除 -> 必须删除
			"independent_cache": true,      // 1.14 弃用 -> 必须删除
			"fakeip":            map[string]interface{}{"enabled": true, "inet4_range": "198.18.0.0/15"},
			"hosts":             map[string]interface{}{"example.com": "1.2.3.4"},
			"servers": []interface{}{
				map[string]interface{}{
					"type": "https", "tag": "dns-remote",
					"address": "https://1.1.1.1/dns-query", "strategy": "prefer_ipv4", "disable_cache": true,
				},
				map[string]interface{}{"type": "predefined", "tag": "pre", "address": "8.8.8.8"},
			},
			"rules": []interface{}{
				map[string]interface{}{"query_type": []interface{}{"A", "AAAA"}, "server": "fakeip"},
			},
			"final": "dns-remote",
		},
		"ntp": map[string]interface{}{
			"enabled": true, "server": "ntp.aliyun.com", "server_port": 123, "interval": "30m", "detour": "direct",
		},
		"certificate":           map[string]interface{}{"store": "system"},
		"certificate_providers": []interface{}{},
		"http_clients":          []interface{}{},
		"network_namespaces":    []interface{}{},
		"services":              []interface{}{},
		"inbounds": []interface{}{
			map[string]interface{}{
				"type": "mixed", "tag": "in-1", "listen": "0.0.0.0", "listen_port": 1080,
				"sniff": true, "tcp_fast_open": true, "udp_timeout": "5m",
			},
		},
		"outbounds": []interface{}{
			map[string]interface{}{"type": "direct", "tag": "direct"},
			map[string]interface{}{"type": "block", "tag": "block"},
		},
		"route": map[string]interface{}{
			"final": "direct", "auto_detect_interface": true,
			"rules": []interface{}{
				map[string]interface{}{
					"ip_version": 4, "auth_user": []interface{}{"u1"}, "invert": true,
					"port": []interface{}{53, 443}, "port_range": []interface{}{"1000:2000"},
					"process_path": []interface{}{"/usr/bin/curl"},
					// 非法字段：必须被清理（规则级 strategy/domain_resolver 都不存在）
					"strategy":        "prefer_ipv4",
					"domain_resolver": "dns-remote",
					"outbound":        "block",
				},
				map[string]interface{}{
					"type": "logical", "mode": "or",
					"rules":    []interface{}{map[string]interface{}{"domain_suffix": []interface{}{".example.com"}}},
					"outbound": "direct",
				},
			},
		},
		"experimental": map[string]interface{}{
			"v2ray_api": map[string]interface{}{
				"listen": "127.0.0.1:10086",
				"stats":  map[string]interface{}{"enabled": true, "inbounds": []interface{}{"in-1"}},
			},
		},
	}

	out, err := PrepareFullConfig(PrepareFullConfigOptions{
		Ctx:        context.Background(),
		ServerID:   1,
		CoreType:   "singbox",
		FullConfig: full,
	})
	if err != nil {
		t.Fatalf("PrepareFullConfig: %v", err)
	}
	if dump := os.Getenv("CF_DUMP_TEST_CONFIG"); dump != "" {
		if err := os.WriteFile(dump, out, 0o644); err != nil {
			t.Fatalf("dump: %v", err)
		}
		t.Logf("已写出 %s", dump)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("产物不是合法 JSON: %v", err)
	}

	// --- DNS ---
	dns, _ := got["dns"].(map[string]interface{})
	if dns == nil {
		t.Fatal("dns 段丢失")
	}
	for _, gone := range []string{"domain_matcher", "independent_cache", "fakeip", "hosts"} {
		if _, ok := dns[gone]; ok {
			t.Errorf("dns.%s 未被清理（sing-box 会 FATAL）", gone)
		}
	}
	if st, _ := dns["strategy"].(string); st != "prefer_ipv4" {
		t.Errorf("server 级 strategy 未提升到顶层: %q", st)
	}
	servers, _ := dns["servers"].([]interface{})
	if len(servers) != 3 {
		t.Fatalf("servers 数量 = %d, want 3: %#v", len(servers), servers)
	}
	s0, _ := servers[0].(map[string]interface{})
	if _, ok := s0["address"]; ok {
		t.Error("server.address 未清理")
	}
	if srv, _ := s0["server"].(string); srv != "1.1.1.1" {
		t.Errorf("URL 形式 address 未拆出 server: %#v", s0["server"])
	}
	if p, _ := s0["path"].(string); p != "/dns-query" {
		t.Errorf("未拆出 path: %#v", s0["path"])
	}
	if _, ok := s0["strategy"]; ok {
		t.Error("server 级 strategy 未清理")
	}
	s1, _ := servers[1].(map[string]interface{})
	if typ, _ := s1["type"].(string); typ != "hosts" {
		t.Errorf("predefined 未改名为 hosts: %#v", typ)
	}
	if _, ok := s1["server"]; ok {
		t.Error("hosts 类型残留 server 字段（hosts 只接受 path/predefined）")
	}
	if pre, ok := s1["predefined"].(map[string]interface{}); !ok || pre["example.com"] != "1.2.3.4" {
		t.Errorf("顶层 hosts 记录未合并进 hosts server: %#v", s1["predefined"])
	}
	s2, _ := servers[2].(map[string]interface{})
	if typ, _ := s2["type"].(string); typ != "fakeip" {
		t.Errorf("顶层 fakeip 未转成 server: %#v", typ)
	}
	if _, ok := s2["detour"]; ok {
		t.Error("fakeip 类型残留 detour（fakeip 无拨号字段）")
	}

	// --- route ---
	route, _ := got["route"].(map[string]interface{})
	if route == nil {
		t.Fatal("route 段丢失")
	}
	if dr, _ := route["default_domain_resolver"].(string); dr != "dns-remote" {
		t.Errorf("未自动补 route.default_domain_resolver: %#v", route["default_domain_resolver"])
	}
	rules, _ := route["rules"].([]interface{})
	var userRule map[string]interface{}
	for _, raw := range rules {
		rm, _ := raw.(map[string]interface{})
		if _, ok := rm["ip_version"]; ok {
			userRule = rm
		}
	}
	if userRule == nil {
		t.Fatal("未找到用户规则")
	}
	for _, gone := range []string{"strategy", "domain_resolver"} {
		if _, ok := userRule[gone]; ok {
			t.Errorf("route rule 残留 %s（未知字段，会 FATAL）", gone)
		}
	}
	if ports, ok := userRule["port"].([]interface{}); !ok || len(ports) != 2 {
		t.Errorf("port 应为数字数组: %#v", userRule["port"])
	}
	if _, ok := userRule["process_path"]; !ok {
		t.Error("process_path 丢失")
	}

	// --- 新增顶层段落 ---
	if _, ok := got["certificate"]; !ok {
		t.Error("certificate 段丢失")
	}
	for _, empty := range []string{"services", "certificate_providers", "http_clients", "network_namespaces"} {
		if _, ok := got[empty]; ok {
			t.Errorf("空段落 %s 应被 CleanEmpty 移除", empty)
		}
	}
	// --- 入站 legacy 字段 ---
	inbounds, _ := got["inbounds"].([]interface{})
	ib0, _ := inbounds[0].(map[string]interface{})
	if _, ok := ib0["sniff"]; ok {
		t.Error("入站 legacy sniff 未清理（1.13 起移除，会 FATAL）")
	}
	if _, ok := ib0["domain_strategy"]; ok {
		t.Error("入站 legacy domain_strategy 未清理")
	}
	if v, _ := ib0["tcp_fast_open"].(bool); !v {
		t.Error("入站通用监听字段 tcp_fast_open 丢失")
	}
}
