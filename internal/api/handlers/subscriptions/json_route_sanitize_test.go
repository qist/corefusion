package subscriptions

import (
	"encoding/json"
	"reflect"
	"testing"

	"corefusion/internal/model/entity"
)

func parseJSONMap(t *testing.T, data []byte) map[string]interface{} {
	t.Helper()
	var out map[string]interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("output should be valid JSON: %v\n%s", err, data)
	}
	return out
}

// sing-box：final / 规则 / rule_set.download_detour 指向不存在出站时必须清掉，
// 否则核心启动即 FATAL（default outbound not found / rule-set not found）。
func TestSanitizeSingboxRouteDropsDanglingRefs(t *testing.T) {
	in := []byte(`{
		"outbounds": [
			{"type":"direct","tag":"direct"},
			{"type":"socks","tag":"node-a","server":"127.0.0.1","server_port":1080}
		],
		"dns": {
			"final": "missing-dns",
			"servers": [
				{"type":"udp","tag":"d","server":"8.8.8.8","detour":"proxy"}
			]
		},
		"route": {
			"final": "proxy",
			"default_domain_resolver": "dns-x",
			"rule_set": [
				{"type":"remote","tag":"cn","url":"https://example.com/cn.srs","format":"binary","download_detour":"proxy"},
				{"type":"remote","tag":"ok","url":"https://example.com/ok.srs","format":"binary","download_detour":"node-a","http_client":"hc"},
				{"type":"local","tag":"localcn","path":"rules/local.json"},
				{"type":"remote","tag":"bad","url":"https://example.com/bad.srs","format":"yaml"},
				{"type":"inline","tag":"inlinecn","rules":[{"domain_suffix":["cn"]}]}
			],
			"rules": [
				{"domain_suffix":["a.com"],"outbound":"direct"},
				{"domain_suffix":["b.com"],"outbound":"proxy"},
				{"rule_set":["missing"],"outbound":"direct"},
				{"rule_set":["cn"],"outbound":"node-a"},
				{"rule_set":["localcn"],"outbound":"direct"},
				{"action":"reject"},
				{"domain_suffix":["c.com"],"action":"reject","outbound":"proxy"},
				123
			]
		}
	}`)

	out := parseJSONMap(t, sanitizeJSONRoute(in, "route", "singbox"))
	route := out["route"].(map[string]interface{})

	if _, has := route["final"]; has {
		t.Fatalf("dangling route.final should be removed: %v", route)
	}
	if _, has := route["default_domain_resolver"]; has {
		t.Fatalf("dangling default_domain_resolver should be removed: %v", route)
	}
	dns := out["dns"].(map[string]interface{})
	if _, has := dns["final"]; has {
		t.Fatalf("dangling dns.final should be removed: %v", dns)
	}
	if _, has := dns["servers"].([]interface{})[0].(map[string]interface{})["detour"]; has {
		t.Fatalf("dangling dns server detour should be removed: %v", dns["servers"])
	}
	sets := route["rule_set"].([]interface{})
	if len(sets) != 3 {
		t.Fatalf("local / invalid rule_set should be dropped (want remote cn + remote ok + inline), got %v", sets)
	}
	if sets[2].(map[string]interface{})["tag"] != "inlinecn" {
		t.Fatalf("inline rule_set should be kept: %v", sets[2])
	}
	if _, has := sets[0].(map[string]interface{})["download_detour"]; has {
		t.Fatalf("dangling download_detour should be removed: %v", sets[0])
	}
	okSet := sets[1].(map[string]interface{})
	if got := okSet["download_detour"]; got != "node-a" {
		t.Fatalf("valid download_detour should be kept, got %v", got)
	}
	if _, has := okSet["http_client"]; has {
		t.Fatalf("http_client not delivered with subscription should be removed: %v", okSet)
	}
	rules := route["rules"].([]interface{})
	if len(rules) != 5 {
		t.Fatalf("want 5 kept rules (dangling outbound rewritten to default), got %d: %v", len(rules), rules)
	}
	first := rules[0].(map[string]interface{})
	if first["outbound"] != "direct" {
		t.Fatalf("valid rule dropped: %v", first)
	}
	if got := rules[1].(map[string]interface{})["outbound"]; got != "direct" {
		t.Fatalf("dangling outbound should fall back to default outbound, got %v", got)
	}

	// 没有悬空引用时必须原样返回（不要无谓重排 JSON）。
	clean := []byte(`{"outbounds":[{"type":"direct","tag":"direct"}],"route":{"rules":[{"domain_suffix":["a.com"],"outbound":"direct"}]}}`)
	if got := sanitizeJSONRoute(clean, "route", "singbox"); string(got) != string(clean) {
		t.Fatalf("clean config should be untouched, got %s", got)
	}
}

// 没有 route 段时 sing-box 的 dns 也要清理：否则 detour/final 悬空同样启动 FATAL。
func TestSanitizeSingboxDNSWithoutRoute(t *testing.T) {
	in := []byte(`{
		"outbounds":[{"type":"direct","tag":"direct"}],
		"dns":{"final":"missing","servers":[{"type":"udp","tag":"d","server":"8.8.8.8","detour":"proxy"}]}
	}`)
	out := parseJSONMap(t, sanitizeJSONRoute(in, "route", "singbox"))
	dns := out["dns"].(map[string]interface{})
	if _, has := dns["final"]; has {
		t.Fatalf("dangling dns.final should be removed: %v", dns)
	}
	if _, has := dns["servers"].([]interface{})[0].(map[string]interface{})["detour"]; has {
		t.Fatalf("dangling dns detour should be removed: %v", dns["servers"])
	}
}

// xray：悬空 outboundTag、无有效匹配字段的规则、空 balancer 都要清理，
// 否则 xray 启动 FATAL 或命中规则时静默丢连接。
func TestSanitizeXrayRouteDropsDanglingRefs(t *testing.T) {
	in := []byte(`{
		"outbounds": [
			{"protocol":"freedom","tag":"direct"},
			{"protocol":"blackhole","tag":"block"}
		],
		"routing": {
			"balancers": [
				{"tag":"bal","selector":["direct","proxy"],"fallbackTag":"proxy"},
				{"tag":"dead","selector":["proxy"]},
				{"tag":"nosel"}
			],
			"rules": [
				{"type":"field","domain":["geosite:cn"],"outboundTag":"direct"},
				{"type":"field","domain":["geosite:google"],"outboundTag":"proxy"},
				{"type":"field","outboundTag":"direct"},
				{"type":"field","domain":["a.com"],"balancerTag":"dead"},
				{"type":"field","domain":["b.com"],"balancerTag":"bal"},
				{"type":"field","domain":["ads.com"],"outboundTag":"blocked"},
				{"type":"field","domain":["c.com"],"balancerTag":"nosel"},
				{"type":"field","domain_suffix":["cn"],"outboundTag":"direct"},
				123
			]
		}
	}`)

	out := parseJSONMap(t, sanitizeJSONRoute(in, "routing", "xray"))
	routing := out["routing"].(map[string]interface{})

	balancers := routing["balancers"].([]interface{})
	if len(balancers) != 1 {
		t.Fatalf("balancer with no existing outbound should be dropped, got %v", balancers)
	}
	bal := balancers[0].(map[string]interface{})
	sel := bal["selector"].([]interface{})
	if !reflect.DeepEqual(sel, []interface{}{"direct"}) {
		t.Fatalf("dangling selector members should be pruned, got %v", sel)
	}
	if _, has := bal["fallbackTag"]; has {
		t.Fatalf("dangling fallbackTag should be removed: %v", bal)
	}

	rules := routing["rules"].([]interface{})
	if len(rules) != 6 {
		t.Fatalf("want 6 kept rules (dangling outbound/balancer rewritten), got %d: %v", len(rules), rules)
	}
	if got := rules[0].(map[string]interface{})["outboundTag"]; got != "direct" {
		t.Fatalf("valid rule dropped: %v", rules[0])
	}
	if got := rules[1].(map[string]interface{})["outboundTag"]; got != "direct" {
		t.Fatalf("dangling outboundTag should fall back to default outbound, got %v", rules[1])
	}
	if got := rules[2].(map[string]interface{})["outboundTag"]; got != "direct" {
		t.Fatalf("rule of dropped balancer should fall back to default outbound, got %v", rules[2])
	}
	if got := rules[3].(map[string]interface{})["balancerTag"]; got != "bal" {
		t.Fatalf("valid balancer rule dropped: %v", rules[3])
	}
	if got := rules[4].(map[string]interface{})["outboundTag"]; got != "block" {
		t.Fatalf("blocked alias should map to block, got %v", rules[4])
	}
	if got := rules[5].(map[string]interface{})["outboundTag"]; got != "direct" {
		t.Fatalf("rule of dropped no-selector balancer should fall back to default, got %v", rules[5])
	}
}

// 历史 `route.rule-set`（连字符）必须迁移成 route.rule_set 并删掉旧键：
// sing-box 严格解码，unknown field "rule-set" 会让整份订阅拒载。
func TestInjectRuleSetsRemovesLegacyHyphenKey(t *testing.T) {
	cfg := &entity.MihomoConfig{ConfigJSON: `{"route":{"rule-set":[
		{"type":"remote","tag":"cn","format":"binary","url":"https://example.com/cn.srs"}
	]}}`}
	base := []byte(`{"outbounds":[{"type":"direct","tag":"direct"}]}`)
	data := mergeFormatSections(base, cfg, "route")
	out := parseJSONMap(t, injectRuleSets(data, cfg))
	route := out["route"].(map[string]interface{})

	if _, has := route["rule-set"]; has {
		t.Fatalf("legacy rule-set key must be removed: %v", route)
	}
	if _, has := route["rule_set"]; !has {
		t.Fatalf("rule_set should be written under route: %v", route)
	}
}

// injectProxyGroup（sing-box）：生成 tag=proxy 的 selector，规则/final 里的 proxy
// 必须保留而不是被 sanitize 回落成默认节点。
func TestInjectProxyGroupSingbox(t *testing.T) {
	in := []byte(`{
		"outbounds": [
			{"type":"socks","tag":"node-a","server":"127.0.0.1","server_port":1080},
			{"type":"socks","tag":"node-b","server":"127.0.0.2","server_port":1080},
			{"type":"direct","tag":"direct"},
			{"type":"block","tag":"block"}
		],
		"route": {
			"final": "proxy",
			"rules": [
				{"domain_suffix":["a.com"],"outbound":"proxy"},
				{"domain_suffix":["b.com"],"outbound":"PROXY"}
			]
		}
	}`)

	out := parseJSONMap(t, sanitizeJSONRoute(injectProxyGroup(in, "singbox"), "route", "singbox"))

	var selector map[string]interface{}
	for _, item := range out["outbounds"].([]interface{}) {
		m := item.(map[string]interface{})
		if m["tag"] == proxyGroupTag {
			selector = m
		}
	}
	if selector == nil {
		t.Fatalf("selector %q should be injected: %v", proxyGroupTag, out["outbounds"])
	}
	if selector["type"] != "selector" {
		t.Fatalf("proxy group should be a selector: %v", selector)
	}
	if got := selector["outbounds"]; !reflect.DeepEqual(got, []interface{}{"node-a", "node-b"}) {
		t.Fatalf("selector members should be nodes only, got %v", got)
	}
	if selector["default"] != "node-a" {
		t.Fatalf("selector default should be the first node, got %v", selector["default"])
	}

	route := out["route"].(map[string]interface{})
	if route["final"] != proxyGroupTag {
		t.Fatalf("route.final proxy should be kept once group exists, got %v", route["final"])
	}
	rules := route["rules"].([]interface{})
	if got := rules[0].(map[string]interface{})["outbound"]; got != proxyGroupTag {
		t.Fatalf("rule referencing proxy should stay on the proxy group, got %v", got)
	}
	if got := rules[1].(map[string]interface{})["outbound"]; got != proxyGroupTag {
		t.Fatalf("uppercase PROXY should resolve to the proxy group, got %v", got)
	}
}

// injectProxyGroup（xray）：没有 selector 出站，用 random balancer 承载 proxy 组，
// 规则里的 outboundTag=proxy 改写成 balancerTag=proxy。
func TestInjectProxyGroupXray(t *testing.T) {
	in := []byte(`{
		"outbounds": [
			{"protocol":"vless","tag":"node-a"},
			{"protocol":"vless","tag":"node-b"},
			{"protocol":"freedom","tag":"direct"},
			{"protocol":"blackhole","tag":"block"}
		],
		"routing": {
			"rules": [
				{"type":"field","domain":["b.com"],"outboundTag":"proxy"},
				{"type":"field","domain":["c.com"],"outboundTag":"REJECT"},
				{"type":"field","domain":["d.com"],"outboundTag":"direct"}
			]
		}
	}`)

	out := parseJSONMap(t, sanitizeJSONRoute(injectProxyGroup(in, "xray"), "routing", "xray"))
	routing := out["routing"].(map[string]interface{})

	var bal map[string]interface{}
	for _, item := range routing["balancers"].([]interface{}) {
		m := item.(map[string]interface{})
		if m["tag"] == proxyGroupTag {
			bal = m
		}
	}
	if bal == nil {
		t.Fatalf("balancer %q should be injected: %v", proxyGroupTag, routing["balancers"])
	}
	if got := bal["selector"]; !reflect.DeepEqual(got, []interface{}{"node-a", "node-b"}) {
		t.Fatalf("balancer selector should be nodes only, got %v", got)
	}
	if st, _ := bal["strategy"].(map[string]interface{}); st["type"] != "random" {
		t.Fatalf("balancer should use random strategy: %v", bal["strategy"])
	}

	rules := routing["rules"].([]interface{})
	first := rules[0].(map[string]interface{})
	if first["balancerTag"] != proxyGroupTag {
		t.Fatalf("proxy rule should be rewritten to balancerTag: %v", first)
	}
	if _, has := first["outboundTag"]; has {
		t.Fatalf("outboundTag should be removed after rewrite: %v", first)
	}
	if got := rules[1].(map[string]interface{})["outboundTag"]; got != "block" {
		t.Fatalf("REJECT alias should map to block, got %v", got)
	}
}

// 节点自己就叫 proxy 时不能生成同名组：duplicate tag 会让客户端整份拒载。
func TestInjectProxyGroupSkipsOnTagCollision(t *testing.T) {
	in := []byte(`{"outbounds":[{"type":"socks","tag":"proxy"},{"type":"direct","tag":"direct"}],"route":{"rules":[]}}`)
	if got := injectProxyGroup(in, "singbox"); string(got) != string(in) {
		t.Fatalf("colliding node tag should leave config untouched, got %s", got)
	}
}

// config_json 里历史遗留的 Mihomo 形状 dns 不能合并进 sing-box / xray 输出。
func TestMergeFormatSectionsSkipsMihomoDNS(t *testing.T) {
	cfg := &entity.MihomoConfig{
		Name: "legacy",
		ConfigJSON: `{
			"dns": {"enable":true,"enhanced-mode":"fake-ip","fake-ip-range":"198.18.0.1/16","nameserver":["8.8.8.8"]},
			"route": {"rules":[{"domain_suffix":["cn"],"outbound":"direct"}]}
		}`,
	}
	base := []byte(`{"outbounds":[{"type":"direct","tag":"direct"}]}`)
	out := parseJSONMap(t, mergeFormatSections(base, cfg, "log", "dns", "route", "experimental"))

	if _, has := out["dns"]; has {
		t.Fatalf("Mihomo-shaped dns must not be merged into JSON format: %v", out["dns"])
	}
	if _, has := out["route"]; !has {
		t.Fatalf("other sections should still be merged: %s", out)
	}
}
