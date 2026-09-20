package coreconfig

import (
	"context"
	"encoding/json"
	"testing"
)

// ---------------------------------------------------------------------------
// stubs
// ---------------------------------------------------------------------------

type stubEndpointProvider struct {
	raw []json.RawMessage
}

func (s *stubEndpointProvider) GetEnabledEndpoints(ctx context.Context, serverID int64, coreType string) ([]json.RawMessage, error) {
	return s.raw, nil
}

type stubChainLinkProvider struct {
	entryOutbounds []interface{}
	entryRules     []interface{}
	entryBalancers []interface{}
	exitInbound    map[string]interface{}
	relayOnly      bool
}

func (s *stubChainLinkProvider) GenerateEntryOutbounds(ctx context.Context, serverID int64) ([]interface{}, []interface{}, error) {
	return s.entryOutbounds, s.entryRules, nil
}

func (s *stubChainLinkProvider) GenerateEntryBalancers(ctx context.Context, serverID int64) ([]interface{}, map[string]interface{}) {
	return s.entryBalancers, nil
}

func (s *stubChainLinkProvider) GenerateExitInbound(ctx context.Context, serverID int64) (map[string]interface{}, error) {
	return s.exitInbound, nil
}

func (s *stubChainLinkProvider) IsRelayOnlyServer(ctx context.Context, serverID int64) bool {
	return s.relayOnly
}

// ---------------------------------------------------------------------------
// route 段名：sing-box 必须写 route，不能是 routing
// ---------------------------------------------------------------------------

// sing-box 的链式代理路由规则必须落在顶层 route 段。
// 此前实现无条件写 "routing"，而 sing-box 的 JSON 解码是 DisallowUnknownFields，
// 顶层出现未知键会让核心直接 FATAL。
func TestPrepareFullConfigSingboxChainRulesUseRouteSection(t *testing.T) {
	full := map[string]interface{}{
		"route":    map[string]interface{}{"rules": []interface{}{}},
		"inbounds": []interface{}{map[string]interface{}{"type": "vless", "tag": "in-1"}},
	}
	chain := &stubChainLinkProvider{
		entryOutbounds: []interface{}{map[string]interface{}{
			"tag":      "chain-1-2",
			"protocol": "vless",
			"settings": map[string]interface{}{
				"vnext": []interface{}{map[string]interface{}{
					"address": "1.2.3.4",
					"port":    float64(443),
					"users":   []interface{}{map[string]interface{}{"id": "uuid-1"}},
				}},
			},
			"streamSettings": map[string]interface{}{"network": "tcp"},
		}},
		entryRules: []interface{}{map[string]interface{}{
			"type":        "field",
			"inboundTag":  []interface{}{"in-1"},
			"outboundTag": "chain-1-2",
		}},
	}

	if _, err := PrepareFullConfig(PrepareFullConfigOptions{
		Ctx:               context.Background(),
		ServerID:          1,
		CoreType:          "singbox",
		FullConfig:        full,
		ChainLinkProvider: chain,
	}); err != nil {
		t.Fatalf("PrepareFullConfig: %v", err)
	}

	if _, exists := full["routing"]; exists {
		t.Fatal("sing-box 配置里出现了顶层 routing 键（应为 route）——会导致核心 FATAL")
	}

	route, _ := full["route"].(map[string]interface{})
	if route == nil {
		t.Fatal("缺少 route 段")
	}
	rules, _ := route["rules"].([]interface{})
	var found map[string]interface{}
	for _, raw := range rules {
		rm, _ := raw.(map[string]interface{})
		if rm["outbound"] == "chain-1-2" {
			found = rm
			break
		}
	}
	if found == nil {
		t.Fatalf("route.rules 中没有链式代理规则: %#v", rules)
	}
	if _, ok := found["inbound_tag"]; ok {
		t.Error("规则仍带 xray 风格的 inbound_tag")
	}
	inbound, ok := found["inbound"].([]string)
	if !ok || len(inbound) != 1 || inbound[0] != "in-1" {
		t.Errorf("inbound 字段不正确: %#v", found["inbound"])
	}

	// 出口 outbound 应被转成 sing-box 格式（type/server/server_port/uuid）
	outbounds, _ := full["outbounds"].([]interface{})
	var chainOb map[string]interface{}
	for _, raw := range outbounds {
		m, _ := raw.(map[string]interface{})
		if m["tag"] == "chain-1-2" {
			chainOb = m
			break
		}
	}
	if chainOb == nil {
		t.Fatal("缺少链式代理 outbound")
	}
	if chainOb["type"] != "vless" || chainOb["server"] != "1.2.3.4" {
		t.Errorf("outbound 未转换为 sing-box 格式: %#v", chainOb)
	}
	if _, ok := chainOb["protocol"]; ok {
		t.Error("outbound 仍带 xray 风格的 protocol 字段")
	}
}

// 多出口：xray 的 routing.balancers[] 要转成 sing-box 的 urltest outbound，
// 并且 balancerTag 规则必须被改写成指向该 urltest —— 之前这类规则被直接丢弃，
// 入口入站没有任何规则命中，流量会落到 route.final(direct)，等于多出口链路静默失效。
func TestPrepareFullConfigSingboxBalancerBecomesURLTest(t *testing.T) {
	chainOutbound := func(exitID string) interface{} {
		return map[string]interface{}{
			"tag":      "chain-1-" + exitID,
			"protocol": "vless",
			"settings": map[string]interface{}{
				"vnext": []interface{}{map[string]interface{}{
					"address": "1.2.3." + exitID,
					"port":    float64(443),
					"users":   []interface{}{map[string]interface{}{"id": "uuid-" + exitID}},
				}},
			},
			"streamSettings": map[string]interface{}{"network": "tcp"},
		}
	}

	full := map[string]interface{}{
		"route":    map[string]interface{}{"rules": []interface{}{}},
		"inbounds": []interface{}{map[string]interface{}{"type": "vless", "tag": "in-1"}},
	}
	chain := &stubChainLinkProvider{
		entryOutbounds: []interface{}{chainOutbound("2"), chainOutbound("3")},
		entryRules: []interface{}{map[string]interface{}{
			"type":        "field",
			"inboundTag":  []interface{}{"in-1"},
			"balancerTag": "bal-chain-1",
		}},
		entryBalancers: []interface{}{map[string]interface{}{
			"tag":      "bal-chain-1",
			"selector": []interface{}{"chain-1-"},
		}},
	}

	if _, err := PrepareFullConfig(PrepareFullConfigOptions{
		Ctx: context.Background(), ServerID: 1, CoreType: "singbox",
		FullConfig: full, ChainLinkProvider: chain,
	}); err != nil {
		t.Fatalf("PrepareFullConfig: %v", err)
	}

	// 1) 生成 urltest outbound，成员为两个出口
	outbounds, _ := full["outbounds"].([]interface{})
	var urltest map[string]interface{}
	for _, raw := range outbounds {
		m, _ := raw.(map[string]interface{})
		if m["type"] == "urltest" {
			urltest = m
		}
	}
	if urltest == nil {
		t.Fatalf("未生成 urltest outbound: %#v", outbounds)
	}
	if urltest["tag"] != "bal-chain-1" {
		t.Errorf("urltest tag = %v，期望 bal-chain-1", urltest["tag"])
	}
	members, _ := urltest["outbounds"].([]string)
	if len(members) != 2 {
		t.Fatalf("urltest 成员数 = %d，期望 2: %#v", len(members), members)
	}

	// 2) 路由规则必须指向该 urltest（不能被丢弃）
	route, _ := full["route"].(map[string]interface{})
	rules, _ := route["rules"].([]interface{})
	found := false
	for _, raw := range rules {
		rm, _ := raw.(map[string]interface{})
		if rm["outbound"] == "bal-chain-1" {
			found = true
			in, _ := rm["inbound"].([]string)
			if len(in) != 1 || in[0] != "in-1" {
				t.Errorf("规则 inbound 不正确: %#v", rm["inbound"])
			}
		}
	}
	if !found {
		t.Fatal("balancer 规则被丢弃 —— 入口流量会直连泄漏（route.final）")
	}
}

// 单出口（无 balancer）时不应生成 urltest
func TestPrepareFullConfigSingboxSingleExitHasNoURLTest(t *testing.T) {
	full := map[string]interface{}{
		"route":    map[string]interface{}{"rules": []interface{}{}},
		"inbounds": []interface{}{map[string]interface{}{"type": "vless", "tag": "in-1"}},
	}
	chain := &stubChainLinkProvider{
		entryOutbounds: []interface{}{map[string]interface{}{
			"tag": "chain-1-2", "protocol": "vless",
			"settings": map[string]interface{}{"vnext": []interface{}{map[string]interface{}{
				"address": "1.2.3.4", "port": float64(443),
				"users": []interface{}{map[string]interface{}{"id": "uuid-1"}},
			}}},
			"streamSettings": map[string]interface{}{"network": "tcp"},
		}},
		entryRules: []interface{}{map[string]interface{}{
			"type": "field", "inboundTag": []interface{}{"in-1"}, "outboundTag": "chain-1-2",
		}},
	}
	if _, err := PrepareFullConfig(PrepareFullConfigOptions{
		Ctx: context.Background(), ServerID: 1, CoreType: "singbox",
		FullConfig: full, ChainLinkProvider: chain,
	}); err != nil {
		t.Fatalf("PrepareFullConfig: %v", err)
	}
	outbounds, _ := full["outbounds"].([]interface{})
	for _, raw := range outbounds {
		if m, _ := raw.(map[string]interface{}); m["type"] == "urltest" {
			t.Errorf("单出口不应生成 urltest: %#v", m)
		}
	}
}

// xray 仍然必须用 routing 段（不能被这次改动带偏）
func TestPrepareFullConfigXrayChainRulesUseRoutingSection(t *testing.T) {
	full := map[string]interface{}{
		"routing":  map[string]interface{}{"rules": []interface{}{}},
		"inbounds": []interface{}{},
	}
	chain := &stubChainLinkProvider{
		entryRules: []interface{}{map[string]interface{}{
			"type":        "field",
			"inboundTag":  []interface{}{"in-1"},
			"outboundTag": "chain-1-2",
		}},
	}
	if _, err := PrepareFullConfig(PrepareFullConfigOptions{
		Ctx: context.Background(), ServerID: 1, CoreType: "xray",
		FullConfig: full, ChainLinkProvider: chain,
	}); err != nil {
		t.Fatalf("PrepareFullConfig: %v", err)
	}
	if _, exists := full["route"]; exists {
		t.Error("xray 配置不应出现 route 段")
	}
	routing, _ := full["routing"].(map[string]interface{})
	rules, _ := routing["rules"].([]interface{})
	if len(rules) == 0 {
		t.Fatal("routing.rules 为空，链式代理规则丢失")
	}
}

// ---------------------------------------------------------------------------
// endpoints 合并
// ---------------------------------------------------------------------------

// sing-box 的 endpoint_configs（WireGuard/Tailscale/...）必须合并进顶层 endpoints[]，
// 且按 tag 去重（sing-box 要求 endpoints 与 outbounds 之间 tag 不重复）。
func TestPrepareFullConfigSingboxMergesEndpoints(t *testing.T) {
	full := map[string]interface{}{
		"endpoints": []interface{}{
			map[string]interface{}{"type": "wireguard", "tag": "wg-dup", "private_key": "existing"},
		},
	}
	ep := &stubEndpointProvider{raw: []json.RawMessage{
		json.RawMessage(`{"type":"wireguard","tag":"wg-dup","private_key":"should-be-skipped"}`),
		json.RawMessage(`{"type":"tailscale","tag":"ts-1"}`),
	}}

	if _, err := PrepareFullConfig(PrepareFullConfigOptions{
		Ctx: context.Background(), ServerID: 1, CoreType: "singbox",
		FullConfig: full, EndpointProvider: ep,
	}); err != nil {
		t.Fatalf("PrepareFullConfig: %v", err)
	}

	endpoints, _ := full["endpoints"].([]interface{})
	if len(endpoints) != 2 {
		t.Fatalf("endpoints 数量应为 2（原 1 个 + 新增 1 个，重复 tag 跳过），实际 %d: %#v", len(endpoints), endpoints)
	}
	if first, _ := endpoints[0].(map[string]interface{}); first["private_key"] != "existing" {
		t.Errorf("同名 tag 不应被覆盖: %#v", first)
	}
	second, _ := endpoints[1].(map[string]interface{})
	if second["tag"] != "ts-1" || second["type"] != "tailscale" {
		t.Errorf("新增端点不正确: %#v", second)
	}
}

// xray 不应合并 endpoints（无此概念）
func TestPrepareFullConfigXrayIgnoresEndpoints(t *testing.T) {
	full := map[string]interface{}{"inbounds": []interface{}{}}
	ep := &stubEndpointProvider{raw: []json.RawMessage{
		json.RawMessage(`{"type":"wireguard","tag":"wg-1"}`),
	}}
	if _, err := PrepareFullConfig(PrepareFullConfigOptions{
		Ctx: context.Background(), ServerID: 1, CoreType: "xray",
		FullConfig: full, EndpointProvider: ep,
	}); err != nil {
		t.Fatalf("PrepareFullConfig: %v", err)
	}
	if _, exists := full["endpoints"]; exists {
		t.Error("xray 配置不应出现 endpoints")
	}
}

// ---------------------------------------------------------------------------
// v2ray_api 默认启用 / clash_api 默认不配置
// ---------------------------------------------------------------------------

func TestEnsureSingboxV2RayAPIEnablesStatsWithoutCreatingClashAPI(t *testing.T) {
	full := map[string]interface{}{}
	EnsureSingboxV2RayAPI(full)

	exp, _ := full["experimental"].(map[string]interface{})
	if exp == nil {
		t.Fatal("缺少 experimental 段")
	}
	if _, exists := exp["clash_api"]; exists {
		t.Error("不应自动创建 clash_api（默认不配置）")
	}
	v2api, _ := exp["v2ray_api"].(map[string]interface{})
	if v2api == nil {
		t.Fatal("缺少 v2ray_api 段")
	}
	if v2api["listen"] != defaultSingboxV2RayAPIListen {
		t.Errorf("v2ray_api.listen = %v，期望 %s", v2api["listen"], defaultSingboxV2RayAPIListen)
	}
	stats, _ := v2api["stats"].(map[string]interface{})
	if stats == nil || stats["enabled"] != true {
		t.Errorf("stats.enabled 未置为 true: %#v", stats)
	}
}

func TestEnsureSingboxV2RayAPIAvoidsClashPortConflict(t *testing.T) {
	full := map[string]interface{}{
		"experimental": map[string]interface{}{
			"clash_api": map[string]interface{}{"external_controller": "127.0.0.1:10086"},
		},
	}
	EnsureSingboxV2RayAPI(full)
	exp, _ := full["experimental"].(map[string]interface{})
	clash, _ := exp["clash_api"].(map[string]interface{})
	if clash["external_controller"] == "127.0.0.1:10086" {
		t.Error("clash_api 与 v2ray_api 端口冲突未避让")
	}
}

// ---------------------------------------------------------------------------
// stats 白名单：必须补入 users，否则按用户计费恒为 0
// ---------------------------------------------------------------------------

func TestPopulateSingboxStatsTargetsIncludesUsers(t *testing.T) {
	full := map[string]interface{}{
		"inbounds": []interface{}{
			map[string]interface{}{
				"type": "vless", "tag": "in-1",
				"users": []interface{}{map[string]interface{}{"name": "a@test"}},
			},
			map[string]interface{}{
				"type": "mixed", "tag": "in-2",
				"users": []interface{}{map[string]interface{}{"username": "bob", "password": "p"}},
			},
		},
		"outbounds": []interface{}{
			map[string]interface{}{"type": "vless", "tag": "proxy-1"},
			map[string]interface{}{"type": "direct", "tag": "direct"},
			map[string]interface{}{"type": "selector", "tag": "auto"},
		},
	}
	EnsureSingboxV2RayAPI(full)
	PopulateSingboxStatsTargets(full)

	exp, _ := full["experimental"].(map[string]interface{})
	v2api, _ := exp["v2ray_api"].(map[string]interface{})
	stats, _ := v2api["stats"].(map[string]interface{})

	users := toStringSlice(stats["users"])
	if len(users) != 2 || users[0] != "a@test" || users[1] != "bob" {
		t.Fatalf("stats.users 应包含 2 个用户（name + username），实际 %#v", users)
	}
	inbounds := toStringSlice(stats["inbounds"])
	if len(inbounds) != 2 {
		t.Errorf("stats.inbounds 应为 2 个，实际 %#v", inbounds)
	}
	outbounds := toStringSlice(stats["outbounds"])
	if len(outbounds) != 1 || outbounds[0] != "proxy-1" {
		t.Errorf("stats.outbounds 应只含代理出站，实际 %#v", outbounds)
	}
}

// 已有白名单应保留并追加，不覆盖
func TestPopulateSingboxStatsTargetsMergesExistingLists(t *testing.T) {
	full := map[string]interface{}{
		"inbounds": []interface{}{map[string]interface{}{"type": "vless", "tag": "in-1"}},
		"experimental": map[string]interface{}{
			"v2ray_api": map[string]interface{}{
				"listen": "127.0.0.1:10086",
				"stats": map[string]interface{}{
					"enabled": true,
					"users":   []interface{}{"manual@test"},
				},
			},
		},
	}
	PopulateSingboxStatsTargets(full)
	exp, _ := full["experimental"].(map[string]interface{})
	v2api, _ := exp["v2ray_api"].(map[string]interface{})
	stats, _ := v2api["stats"].(map[string]interface{})
	users := toStringSlice(stats["users"])
	found := false
	for _, u := range users {
		if u == "manual@test" {
			found = true
		}
	}
	if !found {
		t.Errorf("已有 stats.users 被覆盖: %#v", users)
	}
}

// ---------------------------------------------------------------------------
// 历史字段名归一化
// ---------------------------------------------------------------------------

func TestMigrateSingboxLegacyFieldsRenamesRouteRuleKeys(t *testing.T) {
	full := map[string]interface{}{
		"inbounds": []interface{}{},
		"route": map[string]interface{}{
			"rules": []interface{}{
				map[string]interface{}{
					"inbound_tag": []interface{}{"in-1"},
					"process":     []interface{}{"curl"},
					"outbound":    "direct",
				},
				map[string]interface{}{
					// 已经是正确字段名时不应被改动
					"inbound": []interface{}{"in-2"},
				},
			},
		},
	}
	MigrateSingboxLegacyFields(full)

	route, _ := full["route"].(map[string]interface{})
	rules, _ := route["rules"].([]interface{})
	if len(rules) != 2 {
		t.Fatalf("规则数量变化: %d", len(rules))
	}
	first, _ := rules[0].(map[string]interface{})
	if _, ok := first["inbound_tag"]; ok {
		t.Error("inbound_tag 未被清理（sing-box 会因未知字段 FATAL）")
	}
	if _, ok := first["inbound"]; !ok {
		t.Error("inbound 未生成")
	}
	if _, ok := first["process"]; ok {
		t.Error("process 未被清理")
	}
	if _, ok := first["process_name"]; !ok {
		t.Error("process_name 未生成")
	}
	second, _ := rules[1].(map[string]interface{})
	if in, ok := second["inbound"].([]interface{}); !ok || len(in) != 1 || in[0] != "in-2" {
		t.Errorf("已有 inbound 字段被破坏: %#v", second["inbound"])
	}
}

// 历史 DNS 配置（sing-box 1.12 之前）必须被归一，否则核心启动即 FATAL：
// server.address / server 级 strategy / server 级 disable_cache / type=predefined
// / 顶层 dns.fakeip / 顶层 dns.hosts / dns.domain_matcher / dns.independent_cache
func TestMigrateSingboxLegacyDNSNormalizesLegacyFormat(t *testing.T) {
	full := map[string]interface{}{
		"inbounds": []interface{}{},
		"dns": map[string]interface{}{
			"strategy":          "",
			"domain_matcher":    "adguard",
			"independent_cache": true,
			"fakeip": map[string]interface{}{
				"enabled":     true,
				"inet4_range": "198.18.0.0/15",
			},
			"hosts": map[string]interface{}{
				"example.com": "1.2.3.4",
			},
			"servers": []interface{}{
				map[string]interface{}{
					"type":          "https",
					"tag":           "dns-remote",
					"address":       "https://1.1.1.1/dns-query",
					"strategy":      "prefer_ipv4",
					"disable_cache": true,
				},
				map[string]interface{}{
					"type":    "predefined",
					"tag":     "pre",
					"address": "8.8.8.8",
				},
			},
		},
	}

	MigrateSingboxLegacyFields(full)

	dns, _ := full["dns"].(map[string]interface{})
	if _, ok := dns["domain_matcher"]; ok {
		t.Error("domain_matcher 未清理（sing-box 会因未知字段 FATAL）")
	}
	if _, ok := dns["independent_cache"]; ok {
		t.Error("independent_cache 未清理（1.14 起已弃用）")
	}
	if _, ok := dns["fakeip"]; ok {
		t.Error("顶层 fakeip 未清理（1.14 起移除，会 FATAL）")
	}
	if _, ok := dns["hosts"]; ok {
		t.Error("顶层 hosts 未清理（1.14 起移除，会 FATAL）")
	}
	if st, _ := dns["strategy"].(string); st != "prefer_ipv4" {
		t.Errorf("server 级 strategy 未提升为顶层 strategy: %q", st)
	}

	servers, _ := dns["servers"].([]interface{})
	if len(servers) != 3 { // 2 个原 server + fakeip；顶层 hosts 合并进已有的 hosts server
		t.Fatalf("servers 数量 = %d, want 3: %#v", len(servers), servers)
	}

	first, _ := servers[0].(map[string]interface{})
	if _, ok := first["address"]; ok {
		t.Error("address 未清理")
	}
	if srv, _ := first["server"].(string); srv != "1.1.1.1" {
		t.Errorf("URL 形式 address 未拆出 server: %#v", first["server"])
	}
	if port, _ := first["server_port"].(int); port != 0 {
		// URL 未带端口时不应产生 server_port
		if _, ok := first["server_port"]; ok {
			t.Errorf("不应生成 server_port: %#v", first["server_port"])
		}
	}
	if p, _ := first["path"].(string); p != "/dns-query" {
		t.Errorf("URL 形式 address 未拆出 path: %#v", first["path"])
	}
	if _, ok := first["strategy"]; ok {
		t.Error("server 级 strategy 未清理")
	}
	if _, ok := first["disable_cache"]; ok {
		t.Error("server 级 disable_cache 未清理")
	}

	second, _ := servers[1].(map[string]interface{})
	if typ, _ := second["type"].(string); typ != "hosts" {
		t.Errorf("predefined 未改名为 hosts: %#v", second["type"])
	}
	// hosts 类型不接受 server/server_port（HostsDNSServerOptions 只有 path/predefined），
	// 迁移时必须按类型裁剪，否则 sing-box 严格解码会 FATAL。
	if _, ok := second["server"]; ok {
		t.Errorf("hosts 类型残留 server 字段（会 FATAL）: %#v", second["server"])
	}

	// 顶层 hosts 记录必须合并进已有 hosts server（不能丢数据）
	if pre, ok := second["predefined"].(map[string]interface{}); !ok || pre["example.com"] != "1.2.3.4" {
		t.Errorf("顶层 hosts 记录未合并进 hosts server: %#v", second["predefined"])
	}

	// fakeip 必须追加在末尾（不能成为默认 server，否则 sing-box 报错）
	fakeip, _ := servers[2].(map[string]interface{})
	if typ, _ := fakeip["type"].(string); typ != "fakeip" {
		t.Errorf("顶层 fakeip 未转成 server: %#v", servers[2])
	}
	if r, _ := fakeip["inet4_range"].(string); r != "198.18.0.0/15" {
		t.Errorf("fakeip 范围未保留: %#v", fakeip["inet4_range"])
	}
}

// 已是新格式的 DNS 配置不应被改动
func TestMigrateSingboxLegacyDNSKeepsNewFormat(t *testing.T) {
	full := map[string]interface{}{
		"inbounds": []interface{}{},
		"dns": map[string]interface{}{
			"servers": []interface{}{
				map[string]interface{}{
					"type":        "udp",
					"tag":         "d1",
					"server":      "8.8.8.8",
					"server_port": 53,
					"detour":      "direct",
				},
			},
			"final": "d1",
		},
	}
	MigrateSingboxLegacyFields(full)

	dns, _ := full["dns"].(map[string]interface{})
	servers, _ := dns["servers"].([]interface{})
	if len(servers) != 1 {
		t.Fatalf("servers 数量被改动: %d", len(servers))
	}
	s, _ := servers[0].(map[string]interface{})
	if s["server"] != "8.8.8.8" || s["detour"] != "direct" {
		t.Errorf("新格式字段被破坏: %#v", s)
	}
	if port, _ := s["server_port"].(int); port != 53 {
		t.Errorf("server_port 被破坏: %#v", s["server_port"])
	}
	if _, ok := dns["servers"]; !ok {
		t.Error("servers 丢失")
	}
}
