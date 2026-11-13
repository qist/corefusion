package subscriptions

import (
	"encoding/json"
	"os"
	"testing"

	"corefusion/internal/model/entity"

	"gopkg.in/yaml.v3"
)

// 订阅格式编辑器里「能编辑、能载入」的字段必须在生成 YAML 时原样输出。
// 历史 bug：这些字段只在 UI 侧存在（mihomo 的 dns.proxy-server-nameserver /
// direct-nameserver / fallback-filter、顶层 interface-name / routing-mark /
// skip-auth-prefixes / lan-*-ips、tun.inet6-route-exclude-address、组级
// hidden/disable-udp/include-all/interface-name/routing-mark 等），
// 保存时被丢弃 → 重新打开开关又变回关闭。
func TestGenerateMihomoYAMLPreservesEditorFields(t *testing.T) {
	cfg := map[string]interface{}{
		"mixed-port":          7890,
		"allow-lan":           true,
		"interface-name":      "en0",
		"routing-mark":        6667,
		"skip-auth-prefixes":  []interface{}{"127.0.0.1/8"},
		"lan-allowed-ips":     []interface{}{"192.168.1.0/24"},
		"lan-disallowed-ips":  []interface{}{"192.168.1.5/32"},
		"unified-delay":       true,
		"proxy-groups": []interface{}{
			map[string]interface{}{
				"name": "PROXY", "type": "url-test",
				"proxies":       []interface{}{"DIRECT"},
				"url":           "https://www.gstatic.com/generate_204",
				"interval":      300,
				"hidden":        true,
				"disable-udp":   true,
				"exclude-type":  "Shadowsocks|Http",
				"include-all":   true,
				"interface-name": "en0",
				"routing-mark":  7777,
				// 面板内部字段不能下发给客户端
				"_hasExternalNodes": true,
				"_healthCheck":      true,
			},
		},
		"tun": map[string]interface{}{
			"enable": true,
			"inet6-route-exclude-address": []interface{}{"fc00::/7", "fe80::/10"},
		},
		"dns": map[string]interface{}{
			"enable":     true,
			"nameserver": []interface{}{"https://doh.pub/dns-query"},
			"proxy-server-nameserver": []interface{}{
				"https://dns.google/dns-query", "8.8.8.8",
			},
			"direct-nameserver": []interface{}{"223.5.5.5"},
			"fallback-filter": map[string]interface{}{
				"geoip":      true,
				"geoip-code": "CN",
				"ipcidr":     []interface{}{"240.0.0.0/4"},
				"domain":     []interface{}{"+.google.com"},
			},
		},
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}

	out := (&PublicHandler{}).generateMihomoYAML(&entity.MihomoConfig{ConfigJSON: string(raw)}, nil, nil, nil)

	// 落文件给 `mihomo -t` 用；未设置环境变量时只做断言
	if p := os.Getenv("CF_SUB_YAML_OUT"); p != "" {
		if err := os.WriteFile(p, out, 0o644); err != nil {
			t.Fatalf("write yaml: %v", err)
		}
	}

	var doc map[string]interface{}
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("generated yaml is invalid: %v\n%s", err, out)
	}

	strs := func(v interface{}) []string {
		list, _ := v.([]interface{})
		got := make([]string, 0, len(list))
		for _, x := range list {
			if s, ok := x.(string); ok {
				got = append(got, s)
			}
		}
		return got
	}

	if got := doc["interface-name"]; got != "en0" {
		t.Errorf("interface-name = %v, want en0", got)
	}
	if got := doc["routing-mark"]; got != 6667 {
		t.Errorf("routing-mark = %v, want 6667", got)
	}
	if got := strs(doc["skip-auth-prefixes"]); len(got) != 1 || got[0] != "127.0.0.1/8" {
		t.Errorf("skip-auth-prefixes = %v", got)
	}
	if got := strs(doc["lan-allowed-ips"]); len(got) != 1 || got[0] != "192.168.1.0/24" {
		t.Errorf("lan-allowed-ips = %v", got)
	}
	if got := strs(doc["lan-disallowed-ips"]); len(got) != 1 || got[0] != "192.168.1.5/32" {
		t.Errorf("lan-disallowed-ips = %v", got)
	}

	tun, _ := doc["tun"].(map[string]interface{})
	if got := strs(tun["inet6-route-exclude-address"]); len(got) != 2 || got[0] != "fc00::/7" {
		t.Errorf("tun.inet6-route-exclude-address = %v", got)
	}

	dns, _ := doc["dns"].(map[string]interface{})
	if dns == nil {
		t.Fatalf("dns section missing:\n%s", out)
	}
	if got := strs(dns["proxy-server-nameserver"]); len(got) != 2 || got[1] != "8.8.8.8" {
		t.Errorf("dns.proxy-server-nameserver = %v", got)
	}
	if got := strs(dns["direct-nameserver"]); len(got) != 1 || got[0] != "223.5.5.5" {
		t.Errorf("dns.direct-nameserver = %v", got)
	}
	ff, _ := dns["fallback-filter"].(map[string]interface{})
	if ff == nil {
		t.Fatalf("dns.fallback-filter missing:\n%s", out)
	}
	if ff["geoip-code"] != "CN" {
		t.Errorf("fallback-filter.geoip-code = %v", ff["geoip-code"])
	}
	if got := strs(ff["ipcidr"]); len(got) != 1 || got[0] != "240.0.0.0/4" {
		t.Errorf("fallback-filter.ipcidr = %v", got)
	}
	if got := strs(ff["domain"]); len(got) != 1 || got[0] != "+.google.com" {
		t.Errorf("fallback-filter.domain = %v", got)
	}

	groups, _ := doc["proxy-groups"].([]interface{})
	if len(groups) != 1 {
		t.Fatalf("proxy-groups = %v", doc["proxy-groups"])
	}
	group, _ := groups[0].(map[string]interface{})
	for _, key := range []string{"hidden", "disable-udp", "include-all"} {
		if group[key] != true {
			t.Errorf("group.%s = %v, want true", key, group[key])
		}
	}
	if group["exclude-type"] != "Shadowsocks|Http" {
		t.Errorf("group.exclude-type = %v", group["exclude-type"])
	}
	if group["interface-name"] != "en0" {
		t.Errorf("group.interface-name = %v", group["interface-name"])
	}
	if group["routing-mark"] != 7777 {
		t.Errorf("group.routing-mark = %v", group["routing-mark"])
	}
	// 面板内部字段必须剥离
	if _, ok := group["_hasExternalNodes"]; ok {
		t.Errorf("_hasExternalNodes leaked into client config")
	}
	if _, ok := group["_healthCheck"]; ok {
		t.Errorf("_healthCheck leaked into client config")
	}
}
