package coreconfig

import (
	"encoding/json"
	"testing"
)

func mustJSON(t *testing.T, s string) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return m
}

func outboundTags(m map[string]interface{}) []string {
	var tags []string
	obs, _ := m["outbounds"].([]interface{})
	for _, item := range obs {
		if ob, ok := item.(map[string]interface{}); ok {
			if tag, _ := ob["tag"].(string); tag != "" {
				tags = append(tags, tag)
			}
		}
	}
	return tags
}

func hasTag(tags []string, want string) bool {
	for _, t := range tags {
		if t == want {
			return true
		}
	}
	return false
}

// 禁用的出站必须从运行配置里剔除，内部标记 `_disabled` 不能外泄
// （sing-box 是 strict decode，多一个字段就 FATAL）。
func TestStripDisabledSingboxOutbounds(t *testing.T) {
	cfg := mustJSON(t, `{
	  "outbounds": [
	    {"type": "selector", "tag": "proxy", "outbounds": ["nodeA", "nodeB"]},
	    {"type": "vless", "tag": "nodeA", "server": "a.example.com", "server_port": 443},
	    {"type": "vless", "tag": "nodeB", "server": "b.example.com", "server_port": 443, "_disabled": true},
	    {"type": "direct", "tag": "direct"}
	  ],
	  "route": {"final": "proxy"}
	}`)
	if n := StripDisabledSingboxOutbounds(cfg); n != 1 {
		t.Fatalf("stripped = %d, want 1", n)
	}
	tags := outboundTags(cfg)
	if hasTag(tags, "nodeB") {
		t.Fatalf("禁用出站仍在配置里: %v", tags)
	}
	if !hasTag(tags, "nodeA") || !hasTag(tags, "proxy") {
		t.Fatalf("启用的出站被误删: %v", tags)
	}
	// 分组里不能再引用被禁用的 tag，否则启动 FATAL
	for _, item := range cfg["outbounds"].([]interface{}) {
		ob := item.(map[string]interface{})
		if _, ok := ob["_disabled"]; ok {
			t.Fatalf("_disabled 泄漏到运行配置: %v", ob)
		}
		if ob["tag"] == "proxy" {
			members := ob["outbounds"].([]interface{})
			if len(members) != 1 || members[0] != "nodeA" {
				t.Fatalf("分组成员未清理: %v", members)
			}
		}
	}
}

// 分组被清空要整组剔除，并级联把 route.final 回退到 direct。
func TestStripDisabledSingboxOutboundsCascadesGroup(t *testing.T) {
	cfg := mustJSON(t, `{
	  "outbounds": [
	    {"type": "selector", "tag": "proxy", "outbounds": ["nodeA"]},
	    {"type": "vless", "tag": "nodeA", "server": "a.example.com", "server_port": 443, "_disabled": true},
	    {"type": "direct", "tag": "direct"},
	    {"type": "block", "tag": "block"}
	  ],
	  "route": {
	    "rules": [
	      {"outbound": "proxy", "domain": ["a.com"]},
	      {"outbound": "direct", "domain": ["b.com"]}
	    ],
	    "final": "proxy"
	  }
	}`)
	if n := StripDisabledSingboxOutbounds(cfg); n != 2 {
		t.Fatalf("stripped = %d, want 2 (node + emptied group)", n)
	}
	tags := outboundTags(cfg)
	if hasTag(tags, "proxy") || hasTag(tags, "nodeA") {
		t.Fatalf("空分组/禁用出站未剔除: %v", tags)
	}
	route := cfg["route"].(map[string]interface{})
	if route["final"] != "direct" {
		t.Fatalf("final 应回退到 direct，实际 %v", route["final"])
	}
	rules := route["rules"].([]interface{})
	if len(rules) != 1 {
		t.Fatalf("引用被剔除出站的规则应移除，剩 %d 条", len(rules))
	}
	if rules[0].(map[string]interface{})["outbound"] != "direct" {
		t.Fatalf("误删了有效规则: %v", rules[0])
	}
}

// detour 与 dns.servers[].detour 指向被禁用出站时要删掉该字段。
func TestStripDisabledSingboxOutboundsDetour(t *testing.T) {
	cfg := mustJSON(t, `{
	  "outbounds": [
	    {"type": "vless", "tag": "nodeA", "server": "a.example.com", "server_port": 443, "detour": "hop", "_disabled": true},
	    {"type": "socks", "tag": "hop", "server": "127.0.0.1", "server_port": 1080, "detour": "nodeA"},
	    {"type": "direct", "tag": "direct"}
	  ],
	  "dns": {"servers": [{"tag": "remote", "type": "udp", "server": "1.1.1.1", "detour": "nodeA"}]},
	  "route": {"final": "direct"}
	}`)
	StripDisabledSingboxOutbounds(cfg)
	for _, item := range cfg["outbounds"].([]interface{}) {
		ob := item.(map[string]interface{})
		if ob["tag"] == "hop" {
			if _, ok := ob["detour"]; ok {
				t.Fatalf("detour 应删除: %v", ob)
			}
		}
	}
	server := cfg["dns"].(map[string]interface{})["servers"].([]interface{})[0].(map[string]interface{})
	if _, ok := server["detour"]; ok {
		t.Fatalf("dns detour 应删除: %v", server)
	}
}

// 没有禁用项时只清理内部标记，不动其它字段。
func TestStripDisabledSingboxOutboundsNoop(t *testing.T) {
	cfg := mustJSON(t, `{"outbounds": [
	  {"type": "vless", "tag": "nodeA", "server": "a.example.com", "server_port": 443},
	  {"type": "direct", "tag": "direct"}
	]}`)
	if n := StripDisabledSingboxOutbounds(cfg); n != 0 {
		t.Fatalf("stripped = %d, want 0", n)
	}
	if len(outboundTags(cfg)) != 2 {
		t.Fatalf("不应改动出站: %v", outboundTags(cfg))
	}
}
