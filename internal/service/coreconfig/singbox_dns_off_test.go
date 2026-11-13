package coreconfig

import "testing"

// DNS 关闭（面板默认）时必须清掉所有 DNS server 引用，否则 sing-box 启动直接
// FATAL：`initialize outbound[0]: default domain resolver not found: dns-remote`。
func TestMigrateSingboxLegacyDNSStripsReferencesWhenDisabled(t *testing.T) {
	for name, dns := range map[string]interface{}{
		"no dns section": nil,
		"empty dns":      map[string]interface{}{},
		"no servers":     map[string]interface{}{"rules": []interface{}{}},
	} {
		full := map[string]interface{}{
			"route": map[string]interface{}{
				"default_domain_resolver": "dns-remote",
				"rules": []interface{}{
					map[string]interface{}{"action": "resolve", "server": "dns-remote"},
				},
			},
			"outbounds": []interface{}{
				map[string]interface{}{"type": "direct", "tag": "direct", "domain_resolver": "dns-remote"},
			},
		}
		if dns != nil {
			full["dns"] = dns
		}
		migrateSingboxLegacyDNS(full)

		route := full["route"].(map[string]interface{})
		if _, ok := route["default_domain_resolver"]; ok {
			t.Errorf("%s: route.default_domain_resolver 未清理", name)
		}
		rule := route["rules"].([]interface{})[0].(map[string]interface{})
		if _, ok := rule["server"]; ok {
			t.Errorf("%s: resolve 规则的 server 未清理", name)
		}
		ob := full["outbounds"].([]interface{})[0].(map[string]interface{})
		if _, ok := ob["domain_resolver"]; ok {
			t.Errorf("%s: outbound.domain_resolver 未清理", name)
		}
	}
}

// DNS 开启时不能误删引用（原有行为保持不变）。
func TestMigrateSingboxLegacyDNSKeepsReferencesWhenEnabled(t *testing.T) {
	full := map[string]interface{}{
		"dns": map[string]interface{}{
			"servers": []interface{}{
				map[string]interface{}{"type": "udp", "server": "1.1.1.1", "tag": "dns-remote"},
			},
			"final": "dns-remote",
		},
		"route": map[string]interface{}{
			"default_domain_resolver": "dns-remote",
			"rules":                   []interface{}{},
		},
	}
	migrateSingboxLegacyDNS(full)
	route := full["route"].(map[string]interface{})
	if got, _ := route["default_domain_resolver"].(string); got != "dns-remote" {
		t.Errorf("DNS 开启时不应删除默认解析器，实际 %q", got)
	}
}
