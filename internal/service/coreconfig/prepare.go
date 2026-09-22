package coreconfig

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"strings"
)

// InboundConfigProvider loads enabled inbound configs for a server.
// Implemented by inboundconfig.Service.
type InboundConfigProvider interface {
	GetEnabledInbounds(ctx context.Context, serverID int64, coreType string) ([]json.RawMessage, error)
}

// OutboundConfigProvider loads enabled outbound configs for a server.
// Implemented by outboundconfig.Service.
type OutboundConfigProvider interface {
	GetEnabledOutbounds(ctx context.Context, serverID int64, coreType string) ([]map[string]interface{}, error)
}

// EndpointConfigProvider loads enabled endpoint configs for a server.
// Implemented by endpointconfig.Service.
//
// sing-box 的 endpoints 是与 inbounds/outbounds 并列的顶层段
// （WireGuard / Tailscale / OpenConnect / OpenVPN），xray 没有对应概念，
// 因此只在 CoreType == "singbox" 时合并。
type EndpointConfigProvider interface {
	GetEnabledEndpoints(ctx context.Context, serverID int64, coreType string) ([]json.RawMessage, error)
}

// ChainLinkProvider provides chain proxy outbounds, routing rules, and relay
// inbound for a server. Implemented by the ChainLinkGenerator in the agent
// package. When set, PrepareFullConfig merges chain link outbounds and
// routing rules into the config, and adds a relay-in inbound if the server
// is configured as a chain exit.
type ChainLinkProvider interface {
	// GenerateEntryOutbounds returns chain proxy outbounds and routing rules
	// for the entry side of this server.
	// coreType 用于过滤：只输出"入口入站属于该核心"的链（一台机器可同时跑 xray 与 sing-box，
	// 而 hysteria2 之类只有 sing-box 有出站 —— 混在一起会让 xray 拒绝启动）。
	GenerateEntryOutbounds(ctx context.Context, serverID int64, coreType string) (outbounds []interface{}, routingRules []interface{}, err error)
	// GenerateEntryBalancers returns routing.balancers[] entries and the root
	// observatory config for load-balanced (multi-exit) entries. Returns nil
	// slices when no entry uses load balancing. See design doc §6.5 / §6.6.
	GenerateEntryBalancers(ctx context.Context, serverID int64, coreType string) (balancers []interface{}, observatory map[string]interface{})
}

// PrepareFullConfigOptions holds the parameters for PrepareFullConfig.
type PrepareFullConfigOptions struct {
	// Ctx is the context for database queries.
	Ctx context.Context
	// ServerID is the target server ID for loading inbounds/outbounds.
	// For mirror nodes, this should be the primary (partner) server ID
	// so that inbounds and core_configs (routing, dns, etc.) come from
	// the primary's data.
	ServerID int64
	// OutboundServerID overrides the server ID used for loading outbounds.
	// For mirror nodes, this should be the mirror's own server ID so that
	// outbounds (e.g. chain relay, custom routing) come from the mirror's
	// own outbound_configs table, not the primary's.
	// If 0, defaults to ServerID.
	OutboundServerID int64
	// CoreType is the proxy core type (xray, singbox, etc.).
	CoreType string
	// FullConfig is the raw merged config from core_configs table.
	// This map is modified in place.
	FullConfig map[string]interface{}
	// InboundProvider optionally provides enabled inbound configs.
	InboundProvider InboundConfigProvider
	// OutboundProvider optionally provides enabled outbound configs.
	OutboundProvider OutboundConfigProvider
	// EndpointProvider optionally provides enabled endpoint configs.
	// Only used when CoreType == "singbox" (top-level endpoints[]).
	EndpointProvider EndpointConfigProvider
	// LogMode controls the log section in the generated config.
	// Values: "file" (default), "stdout", "off".
	// If empty, defaults to "file".
	LogMode string
	// RateLimitEnabled indicates whether rate limiting is enabled for this node.
	// When true, EnsureOnlineStatsPolicy will add statsUserOnline to the policy section.
	RateLimitEnabled bool
	// ChainLinkProvider optionally provides chain proxy outbounds, routing
	// rules, and relay-in inbound. When set, chain link outbounds and routing
	// rules are merged into the config, and a relay-in inbound is added if
	// the server is configured as a chain exit.
	ChainLinkProvider ChainLinkProvider
}

// PrepareFullConfig applies all standard config preparation steps to produce
// a final, runtime-ready configuration. This is the single source of truth
// for config generation logic, used by both local and remote nodes.
//
// Steps applied (in order):
//  1. Clean empty/nil fields recursively
//  2. Strip _disabled sections (for all core types)
//  3. For xray: filter disabled routing rules and strip "enabled" fields
//  4. For xray: remove outbounds from core_configs (replaced by outbound_configs table)
//  5. Ensure stats section exists
//  6. For xray: apply API inbound/routing auto-add logic
//  7. Merge enabled inbounds from inbound_configs table
//  8. Merge enabled outbounds from outbound_configs table
//  9. Marshal to JSON bytes
func PrepareFullConfig(opts PrepareFullConfigOptions) ([]byte, error) {
	fullConfig := opts.FullConfig

	// 1. Clean empty/nil fields recursively
	CleanEmpty(fullConfig)

	// 2. Strip _disabled sections (applies to all core types)
	StripDisabledSections(fullConfig)

	// 3. For xray: filter disabled routing rules and strip "enabled" fields
	if opts.CoreType == "xray" {
		FilterDisabledRoutingRules(fullConfig)
	}

	// 4. For xray: remove outbounds from core_configs (replaced by outbound_configs table)
	if opts.CoreType == "xray" {
		delete(fullConfig, "outbounds")
	}

	// 5. Ensure stats section exists (xray only — sing-box uses experimental.v2ray_api.stats)
	if opts.CoreType == "xray" {
		if _, exists := fullConfig["stats"]; !exists {
			fullConfig["stats"] = map[string]interface{}{}
		}
	}

	// 6. For xray: apply API inbound/routing auto-add logic
	if opts.CoreType == "xray" {
		ApplyAPIInboundRouting(fullConfig)
	}

	// 7. Merge enabled inbounds from inbound_configs table
	if opts.InboundProvider != nil {
		enabledInbounds, err := opts.InboundProvider.GetEnabledInbounds(opts.Ctx, opts.ServerID, opts.CoreType)
		if err != nil {
			log.Printf("[PrepareFullConfig] failed to load inbounds for server %d: %v", opts.ServerID, err)
		} else if len(enabledInbounds) > 0 {
			existingInbounds, _ := fullConfig["inbounds"].([]interface{})
			for _, ib := range enabledInbounds {
				var inbound map[string]interface{}
				if json.Unmarshal(ib, &inbound) == nil {
					existingInbounds = append(existingInbounds, inbound)
				}
			}
			fullConfig["inbounds"] = existingInbounds
		}
	}

	// 8. Merge enabled outbounds from outbound_configs table
	// For mirror nodes, OutboundServerID is the mirror's own server ID,
	// so outbounds come from the mirror's own DB, not the primary's.
	outboundServerID := opts.OutboundServerID
	if outboundServerID == 0 {
		outboundServerID = opts.ServerID
	}
	if opts.OutboundProvider != nil {
		enabledOutbounds, err := opts.OutboundProvider.GetEnabledOutbounds(opts.Ctx, outboundServerID, opts.CoreType)
		if err != nil {
			log.Printf("[PrepareFullConfig] failed to load outbounds for server %d: %v", opts.ServerID, err)
		}
		if len(enabledOutbounds) > 0 {
			if opts.CoreType == "xray" {
				// For xray: replace outbounds entirely (database is source of truth)
				fullConfig["outbounds"] = enabledOutbounds
			} else {
				// For other cores: merge into existing
				existingOutbounds, _ := fullConfig["outbounds"].([]interface{})
				for _, ob := range enabledOutbounds {
					existingOutbounds = append(existingOutbounds, ob)
				}
				fullConfig["outbounds"] = existingOutbounds
			}
		}
	}

	// 8a. Merge enabled endpoints from endpoint_configs table (sing-box only).
	// sing-box 的 endpoints 是与 inbounds/outbounds 并列的顶层段。此前这些配置
	// 只写进 endpoint_configs 表，从未合并进运行时配置 —— 保存后重启核心依然看不到
	// 端点，引用 endpoint tag 的路由 / DNS detour / 出站会报 "outbound not found"。
	if opts.CoreType == "singbox" && opts.EndpointProvider != nil {
		endpoints, err := opts.EndpointProvider.GetEnabledEndpoints(opts.Ctx, opts.ServerID, opts.CoreType)
		if err != nil {
			log.Printf("[PrepareFullConfig] failed to load endpoints for server %d: %v", opts.ServerID, err)
		} else if len(endpoints) > 0 {
			existingEndpoints, _ := fullConfig["endpoints"].([]interface{})
			// 按 tag 去重：sing-box 要求 endpoints 与 outbounds 之间的 tag 也不能重复
			seen := make(map[string]bool, len(existingEndpoints))
			for _, raw := range existingEndpoints {
				if m, ok := raw.(map[string]interface{}); ok {
					if tag, _ := m["tag"].(string); tag != "" {
						seen[tag] = true
					}
				}
			}
			for _, raw := range endpoints {
				var ep map[string]interface{}
				if json.Unmarshal(raw, &ep) != nil {
					continue
				}
				tag, _ := ep["tag"].(string)
				if tag != "" && seen[tag] {
					continue
				}
				existingEndpoints = append(existingEndpoints, ep)
				if tag != "" {
					seen[tag] = true
				}
			}
			fullConfig["endpoints"] = existingEndpoints
		}
	}

	// 8b. If ChainLinkProvider is set, merge chain link outbounds and routing rules.
	if opts.ChainLinkProvider != nil {
		chainOutbounds, chainRules, err := opts.ChainLinkProvider.GenerateEntryOutbounds(opts.Ctx, opts.ServerID, opts.CoreType)
		if err != nil {
			log.Printf("[PrepareFullConfig] chain link outbounds failed for server %d: %v", opts.ServerID, err)
		} else {
			// For sing-box: convert xray-format chain outbounds/rules to sing-box format
			var sbBalancerOutbounds []interface{}
			if opts.CoreType == "singbox" {
				chainOutbounds = convertChainOutboundsToSingbox(chainOutbounds)

				// 多出口负载均衡（设计文档 §6.6 / §11.1）：xray 用 routing.balancers[]，
				// sing-box 没有 balancer，改用 urltest outbound 承担同样的职责。
				// 必须在这里处理：balancer 规则原来被直接丢弃，而 balancers 又不生成任何
				// outbound，结果是入口入站没有任何规则命中 → 流量落到 route.final(direct)
				// 直连，多出口链路静默失效（等于流量泄漏）。
				balancerTagMap := map[string]string{}
				if chainBalancers, _ := opts.ChainLinkProvider.GenerateEntryBalancers(opts.Ctx, opts.ServerID, opts.CoreType); len(chainBalancers) > 0 {
					sbBalancerOutbounds, balancerTagMap = convertChainBalancersToSingbox(chainBalancers, chainOutbounds)
				}
				chainRules = convertChainRulesToSingbox(chainRules, balancerTagMap)
			}
			if len(chainOutbounds) > 0 || len(sbBalancerOutbounds) > 0 {
				existingObs, _ := fullConfig["outbounds"].([]interface{})
				merged := mergeChainOutbounds(existingObs, chainOutbounds)
				merged = mergeChainOutbounds(merged, sbBalancerOutbounds)
				fullConfig["outbounds"] = merged
			}
			if len(chainRules) > 0 {
				// 路由段名随核心不同：xray 是 routing，sing-box 是 route。
				// 此前无条件写 "routing"，会让 sing-box 顶层多出未知键，
				// 而 sing-box 的 JSON 解码是 DisallowUnknownFields —— 配置直接 FATAL。
				routeKey := "routing"
				if opts.CoreType == "singbox" {
					routeKey = "route"
				}
				routing, _ := fullConfig[routeKey].(map[string]interface{})
				if routing == nil {
					routing = map[string]interface{}{}
				}
				existingRules, _ := routing["rules"].([]interface{})
				routing["rules"] = mergeChainRoutingRules(existingRules, chainRules)

				// 多出口负载均衡（设计文档 §6.6）：合并 balancers 与根 observatory
				// sing-box 不使用 balancers/observatory（用 selector/urltest outbound 代替）
				if opts.CoreType != "singbox" {
					chainBalancers, chainObservatory := opts.ChainLinkProvider.GenerateEntryBalancers(opts.Ctx, opts.ServerID, opts.CoreType)
					if len(chainBalancers) > 0 {
						existingBal, _ := routing["balancers"].([]interface{})
						routing["balancers"] = append(existingBal, chainBalancers...)
					}
					if chainObservatory != nil {
						// §6.6.5: 当存在 pingConfig 时使用 burstObservatory（特征弱，适合入口节点）
						// 否则使用标准 observatory
						// 依据 xray 官方文档：burstObservatory 结构为 {subjectSelector, pingConfig:{...}}
						if _, hasPingConfig := chainObservatory["pingConfig"]; hasPingConfig {
							if _, ok := fullConfig["burstObservatory"]; !ok {
								fullConfig["burstObservatory"] = chainObservatory
							}
						} else {
							if _, ok := fullConfig["observatory"]; !ok {
								fullConfig["observatory"] = chainObservatory
							}
						}
					}
				}
				fullConfig[routeKey] = routing
			}
		}

	}

	// 8e. xray：leastPing/leastLoad 的 balancer 没有 observatory 时核心拒绝启动，
	// 这里兜底补一个（面板的 observatory 段落是可选的，用户很容易漏配）。
	if opts.CoreType != "singbox" && EnsureBalancerObservatory(fullConfig) {
		log.Printf("[PrepareFullConfig] server %d: injected observatory for leastPing/leastLoad balancer(s)", opts.ServerID)
	}

	// 9. Ensure default outbounds exist — safety net so every
	// node always has direct/block even if missing from DB.
	EnsureDefaultOutbounds(fullConfig, opts.CoreType)

	// 9b. For singbox: migrate legacy inbound fields to route rule actions
	// (sniff, sniff_override_destination, domain_strategy were removed in 1.13.0)
	if opts.CoreType == "singbox" {
		MigrateSingboxLegacyFields(fullConfig)
	}

	// 9c. For singbox: ensure V2Ray API stats is enabled and populate targets
	if opts.CoreType == "singbox" {
		EnsureSingboxV2RayAPI(fullConfig)
		PopulateSingboxStatsTargets(fullConfig)
	}

	// 10. Apply log mode to the config's log section
	ApplyLogMode(fullConfig, opts.CoreType, opts.LogMode)

	// 11. Always ensure online-stats policy for xray so the Panel can show
	// real-time online subscribers (decoupled from rate limiting).
	EnsureOnlineStatsPolicy(fullConfig, opts.CoreType)

	// 12. Marshal to JSON
	// 兜底：leastLoad / leastPing 策略依赖根 observatory，缺了它 xray **启动就失败**
	// （实测 `failed to create server > core: not all dependencies are resolved`）。
	// 这两个策略在面板路由编辑器里可选，而 observatory 是 xray 配置的**顶层**键（不在 routing 内），
	// 编辑器产出不到 → 这里按 balancer 的 selector 统一补。
	if opts.CoreType != "singbox" {
		ensureObservatoryForLeastLoad(fullConfig)
	}

	data, err := json.MarshalIndent(fullConfig, "", "  ")
	if err != nil {
		return nil, err
	}
	return data, nil
}

// EnsureBalancerObservatory 兜底：xray 里凡是用 leastPing / leastLoad 策略的 balancer，
// **必须**存在 observatory（标准或 burst），否则核心启动直接失败：
//
//	failed to build routing configuration > balancer strategy leastLoad requires observatory
//
// 面板的 balancer 编辑器（XrayRouting.vue）可以只写策略不写 observatory，而 observatory 是**独立的
// 顶层段落**（XrayBurstObs.vue，可选）→ 用户很容易只配一半，配置"看着没问题、节点起不来" ✗。
// 这里在生成末尾兜底补一个，subjectSelector 直接取这些 balancer 的 selector（xray 的 selector 是
// 前缀匹配，语义一致）。已有 observatory/burstObservatory 时**不覆盖**（尊重用户配置 ✓）。
//
// 返回是否补了（供调用方打日志）。
func EnsureBalancerObservatory(fullConfig map[string]interface{}) bool {
	routing, _ := fullConfig["routing"].(map[string]interface{})
	if routing == nil {
		return false
	}
	balancers, _ := routing["balancers"].([]interface{})
	if len(balancers) == 0 {
		return false
	}
	needObservatory := false
	var selectors []interface{}
	seen := map[string]bool{}
	for _, bal := range balancers {
		bm, ok := bal.(map[string]interface{})
		if !ok {
			continue
		}
		strategyType := ""
		switch st := bm["strategy"].(type) {
		case string:
			strategyType = st
		case map[string]interface{}:
			if v, ok := st["type"].(string); ok {
				strategyType = v
			}
		}
		if strategyType != "leastPing" && strategyType != "leastLoad" {
			continue
		}
		needObservatory = true
		if sel, ok := bm["selector"].([]interface{}); ok {
			for _, tag := range sel {
				if s, ok := tag.(string); ok && s != "" && !seen[s] {
					seen[s] = true
					selectors = append(selectors, s)
				}
			}
		}
	}
	if !needObservatory {
		return false
	}
	if _, ok := fullConfig["observatory"]; ok {
		return false
	}
	if _, ok := fullConfig["burstObservatory"]; ok {
		return false
	}
	observatory := map[string]interface{}{
		"subjectSelector":   selectors,
		"probeUrl":          "https://www.google.com/generate_204",
		"probeInterval":     "10s",
		"enableConcurrency": true,
	}
	fullConfig["observatory"] = observatory
	return true
}

// EnsureDefaultOutbounds checks that the standard default outbounds exist in
// the final config. For xray, these are "direct" (freedom) and "blocked"
// (blackhole). For singbox, these are "direct", "block", and "dns-out".
// ensureObservatoryForLeastLoad 为使用 leastLoad / leastPing 策略的 balancer 补一个根 observatory。
//
// 为什么必须有：xray 的 leastLoad/leastPing 策略要读 observatory 的健康度数据，缺了它核心
// **启动即失败**（实测报错 `failed to create server > core: not all dependencies are resolved`）。
// 面板路由编辑器允许选这两种策略，但 observatory 是 xray 配置的顶层键、不在 routing 里，
// 编辑器产出不到 → 就在这里兜底：把这些 balancer 的 selector 并进 observatory.subjectSelector。
// 若已存在 observatory / burstObservatory（例如链式负载均衡注入的那份）**只合并不覆盖**，
// 避免把已有的探测配置冲掉。
func ensureObservatoryForLeastLoad(fullConfig map[string]interface{}) {
	routing, _ := fullConfig["routing"].(map[string]interface{})
	if routing == nil {
		return
	}
	balancers, _ := routing["balancers"].([]interface{})
	var selectors []string
	seen := map[string]bool{}
	for _, b := range balancers {
		bm, ok := b.(map[string]interface{})
		if !ok {
			continue
		}
		strategy, _ := bm["strategy"].(map[string]interface{})
		typ, _ := strategy["type"].(string)
		if !strings.EqualFold(typ, "leastLoad") && !strings.EqualFold(typ, "leastPing") {
			continue
		}
		sels, _ := bm["selector"].([]interface{})
		for _, s := range sels {
			str, ok := s.(string)
			if ok && str != "" && !seen[str] {
				seen[str] = true
				selectors = append(selectors, str)
			}
		}
	}
	if len(selectors) == 0 {
		return
	}
	// 已有 observatory / burstObservatory → 只合并 subjectSelector
	for _, key := range []string{"observatory", "burstObservatory"} {
		if obs, ok := fullConfig[key].(map[string]interface{}); ok {
			existing, _ := obs["subjectSelector"].([]interface{})
			for _, s := range selectors {
				has := false
				for _, e := range existing {
					if es, _ := e.(string); es == s {
						has = true
						break
					}
				}
				if !has {
					existing = append(existing, s)
				}
			}
			obs["subjectSelector"] = existing
			return
		}
	}
	selAny := make([]interface{}, 0, len(selectors))
	for _, v := range selectors {
		selAny = append(selAny, v)
	}
	fullConfig["observatory"] = map[string]interface{}{
		"subjectSelector":   selAny,
		"probeURL":          "https://connectivitycheck.gstatic.com/generate_204",
		"probeInterval":     "30s",
		"enableConcurrency": true,
	}
}

// If they are missing, they are appended. This guarantees that
// every node — local or remote — always has these essential outbounds,
// even if the outbound_configs table doesn't have them for that server.

func EnsureDefaultOutbounds(fullConfig map[string]interface{}, coreType string) {
	switch coreType {
	case "xray":
		ensureXrayDefaultOutbounds(fullConfig)
	case "singbox":
		ensureSingboxDefaultOutbounds(fullConfig)
	}
}

// ensureXrayDefaultOutbounds adds missing direct/blocked outbounds for xray.
func ensureXrayDefaultOutbounds(fullConfig map[string]interface{}) {
	// Normalize outbounds to []interface{} regardless of underlying type.
	// GetEnabledOutbounds returns []map[string]interface{}, which does NOT
	// match []interface{} type assertion — so we must convert explicitly.
	var outbounds []interface{}
	switch raw := fullConfig["outbounds"].(type) {
	case []interface{}:
		outbounds = raw
	case []map[string]interface{}:
		for _, m := range raw {
			outbounds = append(outbounds, m)
		}
	default:
		outbounds = []interface{}{}
	}
	hasDirect := false
	hasBlocked := false
	for _, ob := range outbounds {
		if m, ok := ob.(map[string]interface{}); ok {
			tag, _ := m["tag"].(string)
			if tag == "direct" {
				hasDirect = true
			}
			if tag == "blocked" {
				hasBlocked = true
			}
		}
	}
	if !hasDirect {
		outbounds = append(outbounds, map[string]interface{}{
			"protocol": "freedom",
			"tag":      "direct",
		})
		log.Printf("[PrepareFullConfig] added missing 'direct' (freedom) outbound")
	}
	if !hasBlocked {
		outbounds = append(outbounds, map[string]interface{}{
			"protocol": "blackhole",
			"tag":      "blocked",
		})
		log.Printf("[PrepareFullConfig] added missing 'blocked' (blackhole) outbound")
	}
	fullConfig["outbounds"] = outbounds
}

// CleanEmpty removes nil values, empty strings, empty arrays, and empty maps
// recursively. This matches the cleanup logic previously in router.go.
func CleanEmpty(m map[string]interface{}) {
	for k, v := range m {
		if v == nil {
			delete(m, k)
			continue
		}
		// Preserve the "log" section as-is — empty string values have
		// semantic meaning in xray (access/error: "" = stdout,
		// "none" = disabled). Cleaning them would alter behavior.
		if k == "log" {
			continue
		}
		switch val := v.(type) {
		case string:
			if val == "" {
				delete(m, k)
			}
		case []interface{}:
			if len(val) == 0 {
				delete(m, k)
			}
		case map[string]interface{}:
			CleanEmpty(val)
			if len(val) == 0 {
				delete(m, k)
			}
		}
	}
}

// CleanInboundFields removes all internal (underscore-prefixed) fields from an
// inbound config map, ensures required fields are set from the DB record, and
// filters paused users from settings. This is the shared cleaning logic used by
// both the Panel's inboundconfig.Service and the Agent's localInboundProvider
// to ensure consistent, clean output — no _accessDomain, _portHop, etc. in the
// final runtime config.
//
// coreType determines which fields are injected from the DB record:
//   - xray:    "port" and "protocol" (xray uses these in inbound config)
//   - singbox: "listen_port" and "type" (sing-box uses these; config_json
//     already contains them from buildInbound, but we ensure consistency)
func CleanInboundFields(inbound map[string]interface{}, tag, listen, protocol string, port int, coreType string) {
	// Ensure required fields are set from the record
	if tag != "" {
		inbound["tag"] = tag
	}
	if listen != "" {
		inbound["listen"] = listen
	}
	if coreType == "singbox" {
		// sing-box uses "listen_port" (not "port") and "type" (not "protocol")
		if port > 0 {
			inbound["listen_port"] = port
		}
		// "type" should already be in config_json from buildInbound; ensure it
		if protocol != "" {
			if _, exists := inbound["type"]; !exists {
				inbound["type"] = protocol
			}
		}
		// Remove xray-specific fields that may have been stored in config_json
		delete(inbound, "port")
		delete(inbound, "protocol")
	} else {
		// xray: inject "port" and "protocol" from DB record
		if port > 0 {
			inbound["port"] = port
		}
		if protocol != "" {
			inbound["protocol"] = protocol
		}
	}
	// Remove ALL internal (underscore-prefixed) fields recursively from top level
	for k := range inbound {
		if strings.HasPrefix(k, "_") {
			delete(inbound, k)
		}
	}
	// Clean nested _ fields from users/clients/accounts and filter paused users
	// For sing-box: naive/mixed/http/socks use auth.User which only has
	// username+password — remove the "name" field that frontend may have stored.
	// sing-box uses DisallowUnknownFields so "name" causes FATAL on these protocols.
	authUserProtocols := map[string]bool{"naive": true, "mixed": true, "http": true, "socks": true}
	_, isAuthUserProto := authUserProtocols[protocol]
	containers := []map[string]interface{}{inbound}
	if settings, ok := inbound["settings"].(map[string]interface{}); ok {
		containers = append(containers, settings)
	}
	for _, c := range containers {
		for _, key := range []string{"users", "clients", "accounts"} {
			if items, ok := c[key].([]interface{}); ok {
				var filtered []interface{}
				for _, item := range items {
					if m, ok := item.(map[string]interface{}); ok {
						// Skip paused users
						if paused, _ := m["_paused"].(bool); paused {
							continue
						}
						delete(m, "_quota")
						delete(m, "_expiry")
						delete(m, "_usedUp")
						delete(m, "_usedDown")
						delete(m, "_paused")
						// sing-box auth.User protocols: remove "name" field
						if coreType == "singbox" && isAuthUserProto {
							delete(m, "name")
						}
						filtered = append(filtered, m)
					}
				}
				c[key] = filtered
			}
		}
	}
}

// StripDisabledSections removes top-level sections that have "_disabled": true.
func StripDisabledSections(fullConfig map[string]interface{}) {
	for k, v := range fullConfig {
		if v == nil {
			delete(fullConfig, k)
			continue
		}
		if m, ok := v.(map[string]interface{}); ok {
			if d, e := m["_disabled"]; e && d == true {
				delete(fullConfig, k)
			}
		}
	}
}

// FilterDisabledRoutingRules removes disabled routing rules and strips
// the "enabled" field from all rules. Only applies to xray configs.
func FilterDisabledRoutingRules(fullConfig map[string]interface{}) {
	routing, ok := fullConfig["routing"].(map[string]interface{})
	if !ok {
		return
	}
	rules, ok := routing["rules"].([]interface{})
	if !ok {
		return
	}
	var filtered []interface{}
	for _, r := range rules {
		rm, ok := r.(map[string]interface{})
		if !ok {
			filtered = append(filtered, r)
			continue
		}
		// Skip disabled rules
		if en, exists := rm["enabled"]; exists && en == false {
			continue
		}
		// Strip "enabled" field from output
		delete(rm, "enabled")
		filtered = append(filtered, rm)
	}
	routing["rules"] = filtered
}

// ApplyAPIInboundRouting adds API inbound (dokodemo-door) and routing rule
// if api.listen is not set. This is the standard xray API setup logic.
func ApplyAPIInboundRouting(fullConfig map[string]interface{}) {
	apiRaw, ok := fullConfig["api"]
	if !ok {
		return
	}
	apiMap, ok := apiRaw.(map[string]interface{})
	if !ok {
		return
	}

	// Ensure necessary services
	apiMap["services"] = []string{"HandlerService", "LoggerService", "StatsService", "RoutingService"}

	listen, _ := apiMap["listen"].(string)
	tag, _ := apiMap["tag"].(string)
	if tag == "" {
		tag = "api"
	}

	if listen != "" {
		return // listen is set, use it directly
	}

	// Remove listen field (let xray use default)
	delete(apiMap, "listen")

	// Add API inbound (dokodemo-door) if not exists
	inbounds, _ := fullConfig["inbounds"].([]interface{})
	hasAPIInbound := false
	for _, ib := range inbounds {
		if ibMap, ok := ib.(map[string]interface{}); ok {
			if ibMap["tag"] == tag {
				hasAPIInbound = true
				break
			}
		}
	}
	if !hasAPIInbound {
		apiInbound := map[string]interface{}{
			"tag":      tag,
			"listen":   "127.0.0.1",
			"port":     10085,
			"protocol": "dokodemo-door",
			"settings": map[string]interface{}{"address": "127.0.0.1"},
		}
		fullConfig["inbounds"] = append(inbounds, apiInbound)
	}

	// Add routing rule for API if not exists
	routing, _ := fullConfig["routing"].(map[string]interface{})
	if routing == nil {
		routing = map[string]interface{}{"domainStrategy": "IPIfNonMatch"}
	}
	rules, _ := routing["rules"].([]interface{})
	hasAPIRule := false
	for _, r := range rules {
		if rm, ok := r.(map[string]interface{}); ok {
			if tags, ok := rm["inboundTag"].([]interface{}); ok {
				for _, t := range tags {
					if t == tag {
						hasAPIRule = true
						break
					}
				}
			}
		}
	}
	if !hasAPIRule {
		apiRule := map[string]interface{}{
			"type":        "field",
			"inboundTag":  []string{tag},
			"outboundTag": tag,
		}
		routing["rules"] = append([]interface{}{apiRule}, rules...)
		fullConfig["routing"] = routing
	}
}

// ApplyLogMode sets the log section in the config based on the specified log mode.
// mode values: "file" (default), "stdout", "off".
//
// For xray:
//   - file: preserves the user's log config from core_configs as-is (access path,
//     error path, loglevel, dnsLog, etc. are all kept). This is the default and
//     means "let the process write to the files specified in the config".
//   - stdout: overrides access to "none" and error to "/dev/stderr" so the
//     process outputs to stderr for LogBroker capture. Preserves user's loglevel.
//   - off: sets everything to "none" to disable logging entirely.
//
// For singbox:
//   - file: preserves the user's log config as-is.
//   - stdout: overrides output to "stderr". Preserves user's level.
//   - off: sets level to "silent".
//
// For hysteria2/tuic: no config-level log changes (handled by process.Manager).
func ApplyLogMode(fullConfig map[string]interface{}, coreType string, mode string) {
	if mode == "" {
		mode = "file"
	}

	switch coreType {
	case "xray":
		switch mode {
		case "stdout":
			// Route logs through stderr for LogBroker capture, but only override
			// values that are file paths. If the user already set access/error to
			// "" (stdout) or "none" (disabled) or "/dev/stderr", preserve their
			// choice — the mode was derived from their config.
			log, _ := fullConfig["log"].(map[string]interface{})
			if log == nil {
				log = map[string]interface{}{}
			}
			access, _ := log["access"].(string)
			if access != "" && access != "none" && access != "/dev/stderr" {
				log["access"] = "none"
			}
			errorVal, _ := log["error"].(string)
			if errorVal != "" && errorVal != "none" && errorVal != "/dev/stderr" {
				log["error"] = "/dev/stderr"
			}
			if log["loglevel"] == nil || log["loglevel"] == "" {
				log["loglevel"] = "warning"
			}
			fullConfig["log"] = log
		case "off":
			fullConfig["log"] = map[string]interface{}{
				"access":   "none",
				"error":    "none",
				"loglevel": "none",
			}
		default: // "file" — preserve user's log config from core_configs as-is
			// Do nothing. The log section from core_configs already has the
			// user's desired access/error/loglevel settings.
		}

	case "singbox":
		switch mode {
		case "stdout":
			log, _ := fullConfig["log"].(map[string]interface{})
			if log == nil {
				log = map[string]interface{}{}
			}
			log["output"] = "stderr"
			if log["level"] == nil || log["level"] == "" {
				log["level"] = "info"
			}
			fullConfig["log"] = log
		case "off":
			fullConfig["log"] = map[string]interface{}{
				"level": "silent",
			}
		default: // "file" — preserve user's log config from core_configs as-is
			// Do nothing.
		}

		// hysteria2 and tuic don't have config-level log settings.
		// Their log output is controlled by process.Manager (file/stdout/off).
	}
}

// EnsureOnlineStatsPolicy adds statsUserOnline to the xray policy section
// when rate limiting is enabled. This allows the Agent to query per-user
// online IPs via the xray Stats API, which is needed for the P1 trigger
// source (online-stats polling).
//
// This only affects xray configs. For other core types, it's a no-op.
// statsUserOnline is a relatively new xray-core feature; if the running
// core doesn't support it, the Agent will get an Unimplemented error and
// fall back to P0 (Panel push) as the sole IP source.
func EnsureOnlineStatsPolicy(fullConfig map[string]interface{}, coreType string) {
	if coreType != "xray" {
		return
	}

	policy, ok := fullConfig["policy"].(map[string]interface{})
	if !ok {
		return
	}
	levels, ok := policy["levels"].(map[string]interface{})
	if !ok {
		return
	}
	level0, ok := levels["0"].(map[string]interface{})
	if !ok {
		level0 = map[string]interface{}{}
		levels["0"] = level0
	}
	// Only add when rate limiting is enabled; nodes without rate limiting
	// keep the default (no online tracking) to avoid the extra memory overhead.
	level0["statsUserOnline"] = true
}

// mergeChainOutbounds merges chain link outbounds into existing outbounds.
// Removes any existing outbounds with the same tags first (idempotent).
func mergeChainOutbounds(existing []interface{}, chainOutbounds []interface{}) []interface{} {
	existingTags := make(map[string]bool)
	filtered := make([]interface{}, 0, len(existing)+len(chainOutbounds))
	for _, ob := range existing {
		if obMap, ok := ob.(map[string]interface{}); ok {
			if tag, ok := obMap["tag"].(string); ok {
				existingTags[tag] = true
			}
		}
		filtered = append(filtered, ob)
	}
	for _, cob := range chainOutbounds {
		if obMap, ok := cob.(map[string]interface{}); ok {
			if tag, ok := obMap["tag"].(string); ok && existingTags[tag] {
				continue
			}
		}
		filtered = append(filtered, cob)
	}
	return filtered
}

// mergeChainRoutingRules merges chain link routing rules into existing rules.
// Chain rules go first (higher priority).
func mergeChainRoutingRules(existing []interface{}, chainRules []interface{}) []interface{} {
	result := make([]interface{}, 0, len(existing)+len(chainRules))
	result = append(result, chainRules...)
	result = append(result, existing...)
	return result
}

// ensureSingboxDefaultOutbounds adds missing direct/block outbounds for sing-box.
// Note: sing-box 1.13.0 removed "dns" outbound type — DNS hijacking is now
// done via route rule action "hijack-dns" (handled by MigrateSingboxLegacyFields).
func ensureSingboxDefaultOutbounds(fullConfig map[string]interface{}) {
	// Normalize outbounds to []interface{}
	var outbounds []interface{}
	switch raw := fullConfig["outbounds"].(type) {
	case []interface{}:
		outbounds = raw
	case []map[string]interface{}:
		for _, m := range raw {
			outbounds = append(outbounds, m)
		}
	default:
		outbounds = []interface{}{}
	}

	hasDirect := false
	hasBlock := false
	for _, ob := range outbounds {
		if m, ok := ob.(map[string]interface{}); ok {
			tag, _ := m["tag"].(string)
			switch tag {
			case "direct":
				hasDirect = true
			case "block":
				hasBlock = true
			}
		}
	}
	if !hasDirect {
		outbounds = append(outbounds, map[string]interface{}{
			"type": "direct",
			"tag":  "direct",
		})
	}
	if !hasBlock {
		outbounds = append(outbounds, map[string]interface{}{
			"type": "block",
			"tag":  "block",
		})
	}
	fullConfig["outbounds"] = outbounds
}

// parseMbpsStr parses a bandwidth string like "100 Mbps" or "50" into an int (Mbps).
func parseMbpsStr(s string) int {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.TrimSuffix(s, "mbps")
	s = strings.TrimSuffix(s, "mb")
	s = strings.TrimSpace(s)
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			break
		}
	}
	return n
}

// normalizeSingboxRouteRuleFields 把前端/xray 习惯的字段名改写为 sing-box 的字段名：
//
//	inbound_tag → inbound
//	process     → process_name
//
// 这两组字段名在 xray 里是 inboundTag/process，前端曾直接落库为下划线形式；
// sing-box 解码时是 DisallowUnknownFields，带未知字段的规则会让核心起不来。
// 只在目标字段不存在时搬运，避免覆盖用户已正确配置的值。
func normalizeSingboxRouteRuleFields(rules []interface{}) {
	for _, raw := range rules {
		rule, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if v, exists := rule["inbound_tag"]; exists {
			if _, has := rule["inbound"]; !has {
				rule["inbound"] = v
			}
			delete(rule, "inbound_tag")
		}
		if v, exists := rule["process"]; exists {
			if _, has := rule["process_name"]; !has {
				rule["process_name"] = v
			}
			delete(rule, "process")
		}
		// 规则上的 domain_resolver 不是合法字段（拨号字段才有），
		// 但 action=resolve 时对应的字段是 server —— 有该动作时做等价改写。
		if v, exists := rule["domain_resolver"]; exists {
			action, _ := rule["action"].(string)
			if action == "resolve" {
				if _, has := rule["server"]; !has {
					rule["server"] = v
				}
			} else {
				log.Printf("[coreconfig] route rule: 删除非法字段 domain_resolver（仅拨号字段支持，resolve 动作用 server）")
			}
			delete(rule, "domain_resolver")
		}
		// 规则上的 strategy 只对 action=resolve 合法，其它情况是未知字段（FATAL）。
		if _, exists := rule["strategy"]; exists {
			action, _ := rule["action"].(string)
			if action != "resolve" {
				log.Printf("[coreconfig] route rule: 删除非法字段 strategy（仅 action=resolve 支持）")
				delete(rule, "strategy")
			}
		}
	}
}

// MigrateSingboxLegacyFields removes deprecated inbound fields (sniff,
// sniff_override_destination, sniff_timeout, domain_strategy) that were
// removed in sing-box 1.13.0, and adds equivalent route rule actions.
// https://sing-box.sagernet.org/migration/#migrate-legacy-inbound-fields-to-rule-actions
func MigrateSingboxLegacyFields(fullConfig map[string]interface{}) {
	route, ok := fullConfig["route"].(map[string]interface{})
	if !ok {
		route = map[string]interface{}{}
		fullConfig["route"] = route
	}
	rules, _ := route["rules"].([]interface{})

	// 归一化 route 规则的字段名：前端历史数据写的是 inbound_tag / process，
	// 而 sing-box 的字段是 inbound / process_name —— 未知字段会导致 FATAL。
	normalizeSingboxRouteRuleFields(rules)

	// 归一化 DNS 段：sing-box 1.12～1.14 先后移除了 server.address、server 级
	// strategy/disable_cache、顶层 fakeip/hosts、domain_matcher 等，历史数据必须
	// 转换，否则核心启动即 FATAL。
	migrateSingboxLegacyDNS(fullConfig)

	// Check if any inbound has legacy sniff fields
	needsSniffRule := false

	inbounds, _ := fullConfig["inbounds"].([]interface{})
	for _, raw := range inbounds {
		ib, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if sniff, ok := ib["sniff"].(bool); ok && sniff {
			needsSniffRule = true
			delete(ib, "sniff")
		}
		delete(ib, "sniff_override_destination")
		delete(ib, "sniff_timeout")
		delete(ib, "domain_strategy")

		// Migrate Hysteria2 up/down → up_mbps/down_mbps (sing-box 1.14+)
		if ibType, _ := ib["type"].(string); ibType == "hysteria2" {
			if up, ok := ib["up"].(string); ok {
				ib["up_mbps"] = parseMbpsStr(up)
				delete(ib, "up")
			}
			if down, ok := ib["down"].(string); ok {
				ib["down_mbps"] = parseMbpsStr(down)
				delete(ib, "down")
			}
		}

		// NetworkList: sing-box requires ["tcp","udp"] JSON array or single "tcp"/"udp" string.
		// Frontend stores as comma-separated string (e.g. "tcp,udp") — convert to array.
		// sing-box UnmarshalJSON rejects "tcp,udp" as a single unknown network name.
		if net, ok := ib["network"].(string); ok && net != "" {
			parts := strings.Split(net, ",")
			valid := true
			var nets []string
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "tcp" && p != "udp" {
					valid = false
					break
				}
				nets = append(nets, p)
			}
			if valid && len(nets) == 1 {
				ib["network"] = nets[0]
			} else if valid && len(nets) > 1 {
				ib["network"] = nets
			}
		}

		// Clean transport fields: remove fields not valid for the transport type.
		// sing-box is strict — unknown fields cause FATAL errors.
		//   http: host, path, method, headers
		//   ws: path, headers, max_early_data, early_data_header_name
		//   quic: (no extra fields)
		//   grpc: service_name, idle_timeout, permit_without_stream, ping_timeout
		//   httpupgrade: host, path, headers
		if transport, ok := ib["transport"].(map[string]interface{}); ok {
			tpType, _ := transport["type"].(string)
			validFields := map[string]map[string]bool{
				"http":        {"type": true, "host": true, "path": true, "method": true, "headers": true},
				"ws":          {"type": true, "path": true, "headers": true, "max_early_data": true, "early_data_header_name": true},
				"quic":        {"type": true},
				"grpc":        {"type": true, "service_name": true, "idle_timeout": true, "permit_without_stream": true, "ping_timeout": true},
				"httpupgrade": {"type": true, "host": true, "path": true, "headers": true},
			}
			if allowed, ok := validFields[tpType]; ok {
				for k := range transport {
					if !allowed[k] {
						delete(transport, k)
					}
				}
			}
		}

		// Clean inbound TLS: remove "utls" field (outbound-only in sing-box).
		// InboundTLSOptions does not have a UTLS field — including it causes FATAL.
		if tls, ok := ib["tls"].(map[string]interface{}); ok {
			delete(tls, "utls")
		}
	}

	// Migrate outbounds: Hysteria2 up/down → up_mbps/down_mbps
	// Also remove deprecated "dns" outbound (removed in 1.13.0, use hijack-dns rule action)
	outbounds, _ := fullConfig["outbounds"].([]interface{})
	var filteredOutbounds []interface{}
	for _, raw := range outbounds {
		ob, ok := raw.(map[string]interface{})
		if !ok {
			filteredOutbounds = append(filteredOutbounds, raw)
			continue
		}
		if obType, _ := ob["type"].(string); obType == "dns" {
			// Skip dns outbound — it's replaced by hijack-dns route action
			continue
		} else if obType == "hysteria2" {
			if up, ok := ob["up"].(string); ok {
				ob["up_mbps"] = parseMbpsStr(up)
				delete(ob, "up")
			}
			if down, ok := ob["down"].(string); ok {
				ob["down_mbps"] = parseMbpsStr(down)
				delete(ob, "down")
			}
		}
		filteredOutbounds = append(filteredOutbounds, ob)
	}
	fullConfig["outbounds"] = filteredOutbounds

	// Also remove from listen fields at top level (if present)
	delete(fullConfig, "sniff")
	delete(fullConfig, "sniff_override_destination")
	delete(fullConfig, "sniff_timeout")
	delete(fullConfig, "domain_strategy")

	if !needsSniffRule {
		return
	}

	// Check existing rules for sniff/hijack-dns actions
	hasSniffAction := false
	hasDNSHijack := false
	for _, r := range rules {
		if rm, ok := r.(map[string]interface{}); ok {
			if a, _ := rm["action"].(string); a == "sniff" {
				hasSniffAction = true
			}
			if a, _ := rm["action"].(string); a == "hijack-dns" {
				hasDNSHijack = true
			}
		}
	}

	// Prepend sniff rule and DNS hijack rule at the beginning of route rules
	var newRules []interface{}
	if !hasSniffAction {
		newRules = append(newRules, map[string]interface{}{
			"action": "sniff",
		})
	}
	if !hasDNSHijack {
		newRules = append(newRules, map[string]interface{}{
			"protocol": []interface{}{"dns"},
			"action":   "hijack-dns",
		})
	}

	// Prepend new rules to existing rules
	newRules = append(newRules, rules...)
	route["rules"] = newRules
}

// defaultSingboxV2RayAPIListen 是 sing-box v2ray_api 的默认监听地址。
const defaultSingboxV2RayAPIListen = "127.0.0.1:10086"

// migrateSingboxLegacyDNS 把历史 DNS 配置归一为 sing-box 1.14+ 的格式。
//
// 背景（均以真实二进制 `sing-box check` 验证过）：
//   - dns.servers[].address        —— 1.12 起移除，未知字段直接 FATAL
//   - dns.servers[].strategy       —— server 级已移除（FATAL），提升到顶层 dns.strategy
//   - dns.servers[].disable_cache  —— server 级已移除（FATAL），丢弃
//   - dns.servers[].type ""/"legacy" —— 旧格式已移除（FATAL），按 udp 处理
//   - dns.servers[].type "predefined" —— 已改名 hosts
//   - dns.fakeip（顶层）           —— 1.14 移除（FATAL），转成 type=fakeip 的 server
//   - dns.hosts（顶层）            —— 1.14 移除（FATAL），转成 type=hosts 的 server
//   - dns.domain_matcher           —— 未知字段（FATAL），删除
//   - dns.independent_cache        —— 1.14 起弃用（会打印 ERROR），删除
func migrateSingboxLegacyDNS(fullConfig map[string]interface{}) {
	dns, ok := fullConfig["dns"].(map[string]interface{})
	if !ok || dns == nil {
		return
	}

	servers, _ := dns["servers"].([]interface{})
	legacyStrategy := ""
	hasFakeIP := false

	for _, raw := range servers {
		s, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		// address → server；URL 形式（https://1.1.1.1/dns-query、tls://1.1.1.1:853）拆解
		if _, hasServer := s["server"]; !hasServer {
			if addr, _ := s["address"].(string); addr != "" {
				if scheme, host, port, path, isURL := parseLegacyDNSAddress(addr); isURL {
					switch scheme {
					case "https", "h3", "tls", "tcp", "udp", "quic":
						s["type"] = scheme
					}
					if host != "" {
						s["server"] = host
					}
					if port > 0 {
						s["server_port"] = port
					}
					if path != "" {
						s["path"] = path
					}
				} else {
					s["server"] = addr
				}
			}
		}
		delete(s, "address")

		if st, _ := s["strategy"].(string); st != "" && legacyStrategy == "" {
			legacyStrategy = st
		}
		delete(s, "strategy")
		delete(s, "disable_cache")

		typ, _ := s["type"].(string)
		switch typ {
		case "predefined":
			s["type"] = "hosts"
			typ = "hosts"
		case "", "legacy":
			// 旧格式（无 type / legacy）已移除：能拿到 server 就按 udp，否则丢弃
			if _, hasServer := s["server"]; hasServer {
				s["type"] = "udp"
				typ = "udp"
			}
		}
		switch typ {
		case "fakeip":
			hasFakeIP = true
		}

		// 按类型裁剪不支持的字段（如 hosts 带 server、fakeip 带 detour 都会 FATAL）
		sanitizeSingboxDNSServer(s)
	}

	// 顶层 fakeip → type=fakeip 的 server（追加在末尾，避免成为默认 server；
	// sing-box 明确禁止 fakeip 作为默认 server）
	if fp, ok := dns["fakeip"].(map[string]interface{}); ok && fp != nil {
		if enabled, hasFlag := fp["enabled"].(bool); (!hasFlag || enabled) && !hasFakeIP {
			srv := map[string]interface{}{"type": "fakeip", "tag": "fakeip"}
			if v, _ := fp["inet4_range"].(string); v != "" {
				srv["inet4_range"] = v
			} else {
				srv["inet4_range"] = "198.18.0.0/15"
			}
			if v, _ := fp["inet6_range"].(string); v != "" {
				srv["inet6_range"] = v
			}
			servers = append(servers, srv)
		}
		delete(dns, "fakeip")
	}

	// 顶层 hosts → 合并进已有 type=hosts 的 server（避免丢弃用户记录），否则新建一个
	if hs, ok := dns["hosts"].(map[string]interface{}); ok && hs != nil && len(hs) > 0 {
		merged := false
		for _, raw := range servers {
			s, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			if typ, _ := s["type"].(string); typ != "hosts" {
				continue
			}
			pre, _ := s["predefined"].(map[string]interface{})
			if pre == nil {
				pre = map[string]interface{}{}
			}
			for domain, ip := range hs {
				if _, exists := pre[domain]; !exists {
					pre[domain] = ip
				}
			}
			s["predefined"] = pre
			merged = true
			break
		}
		if !merged {
			servers = append(servers, map[string]interface{}{
				"type":       "hosts",
				"tag":        "hosts",
				"predefined": hs,
			})
		}
		delete(dns, "hosts")
	}

	if len(servers) > 0 {
		// fakeip 不能作为默认 server（sing-box: "default server cannot be fakeip"）：
		// 若它排在首位且 dns.final 未指定，移到末尾。
		if first, _ := servers[0].(map[string]interface{}); first != nil {
			if typ, _ := first["type"].(string); typ == "fakeip" {
				if fin, _ := dns["final"].(string); fin == "" {
					servers = append(servers[1:], servers[0])
				}
			}
		}
		dns["servers"] = servers
	}
	if legacyStrategy != "" {
		if cur, _ := dns["strategy"].(string); cur == "" {
			dns["strategy"] = legacyStrategy
		}
	}
	delete(dns, "domain_matcher")
	delete(dns, "independent_cache")

	// sing-box 1.12+ 要求显式声明“用哪个 DNS server 解析拨号字段”
	// （route.default_domain_resolver / domain_resolver）。缺失时，只要配置同时存在
	// fakeip/hosts 与常规 server，就会命中
	// "missing route.default_domain_resolver or domain_resolver in dial fields"
	// 并在 1.14+ 直接 FATAL（1.15 构建实测）。这里在缺失时自动补一个可用 tag。
	if len(servers) == 0 {
		return
	}
	route, _ := fullConfig["route"].(map[string]interface{})
	if route == nil {
		route = map[string]interface{}{}
		fullConfig["route"] = route
	}
	if cur, _ := route["default_domain_resolver"].(string); cur != "" {
		return
	}
	if tag := pickDefaultDomainResolver(dns, servers); tag != "" {
		route["default_domain_resolver"] = tag
	}
}

// pickDefaultDomainResolver 选一个适合做 route.default_domain_resolver 的 DNS server tag：
// 优先 dns.final 指向的 server，其次第一个可解析的 server（跳过 fakeip）。
func pickDefaultDomainResolver(dns map[string]interface{}, servers []interface{}) string {
	tagOf := func(raw interface{}) (string, string) {
		s, ok := raw.(map[string]interface{})
		if !ok {
			return "", ""
		}
		tag, _ := s["tag"].(string)
		typ, _ := s["type"].(string)
		return tag, typ
	}
	if final, _ := dns["final"].(string); final != "" {
		for _, raw := range servers {
			if tag, typ := tagOf(raw); tag == final && typ != "fakeip" {
				return tag
			}
		}
	}
	for _, raw := range servers {
		if tag, typ := tagOf(raw); tag != "" && typ != "fakeip" {
			return tag
		}
	}
	return ""
}

// singboxDNSServerDialFields 是 DialerOptions 允许的字段（远程/本地类 server 可用）
var singboxDNSServerDialFields = []string{
	"detour", "bind_interface", "inet4_bind_address", "inet6_bind_address",
	"bind_address_no_port", "protect_path", "routing_mark", "reuse_addr", "netns",
	"connect_timeout", "tcp_fast_open", "tcp_multi_path", "disable_tcp_keep_alive",
	"tcp_keep_alive", "tcp_keep_alive_interval", "udp_fragment", "domain_resolver",
	"network_strategy",
}

func withDialFields(base ...string) map[string]bool {
	out := map[string]bool{"type": true, "tag": true}
	for _, f := range base {
		out[f] = true
	}
	for _, f := range singboxDNSServerDialFields {
		out[f] = true
	}
	return out
}

// singboxDNSServerAllowedFields 按 server 类型给出允许字段白名单。
// sing-box 对每个类型都用严格解码，多写一个字段（如给 hosts 写 server、给 fakeip
// 写 detour）都会 FATAL，因此必须按类型裁剪。
func singboxDNSServerAllowedFields(typ string) map[string]bool {
	switch typ {
	case "udp", "tcp":
		return withDialFields("server", "server_port")
	case "tls":
		return withDialFields("server", "server_port", "tls")
	case "https", "h3":
		return withDialFields("server", "server_port", "path", "method", "headers", "tls")
	case "quic":
		return withDialFields("server", "server_port", "tls")
	case "local":
		return withDialFields("prefer_go", "neighbor_domain")
	case "dhcp", "mdns":
		return withDialFields("interface", "prefer_go", "neighbor_domain")
	case "hosts":
		// HostsDNSServerOptions: 只有 path / predefined（无拨号字段）
		return map[string]bool{"type": true, "tag": true, "path": true, "predefined": true}
	case "fakeip":
		// FakeIPDNSServerOptions: 只有 inet4_range / inet6_range
		return map[string]bool{"type": true, "tag": true, "inet4_range": true, "inet6_range": true}
	case "tailscale", "openconnect", "openvpn":
		return map[string]bool{
			"type": true, "tag": true, "endpoint": true,
			"accept_default_resolvers": true, "accept_search_domain": true,
		}
	case "resolved":
		return map[string]bool{
			"type": true, "tag": true, "service": true, "accept_default_resolvers": true,
		}
	default:
		return nil // 未知类型：交给 sing-box 报错，不做裁剪
	}
}

// sanitizeSingboxDNSServer 删除该 server 类型不支持的字段（避免 FATAL）。
func sanitizeSingboxDNSServer(s map[string]interface{}) {
	typ, _ := s["type"].(string)
	allowed := singboxDNSServerAllowedFields(typ)
	if allowed == nil {
		return
	}
	for k := range s {
		if !allowed[k] {
			log.Printf("[coreconfig] DNS server %q (type=%s): 删除该类型不支持的字段 %q", s["tag"], typ, k)
			delete(s, k)
		}
	}
}

// parseLegacyDNSAddress 解析旧版 DNS address 字段：
// "8.8.8.8" 返回 isURL=false；"https://1.1.1.1/dns-query" 返回拆解结果。
func parseLegacyDNSAddress(addr string) (scheme, host string, port int, path string, isURL bool) {
	idx := strings.Index(addr, "://")
	if idx <= 0 {
		return "", "", 0, "", false
	}
	scheme = strings.ToLower(addr[:idx])
	rest := addr[idx+3:]
	if i := strings.Index(rest, "/"); i >= 0 {
		path = rest[i:]
		rest = rest[:i]
	}
	host = rest
	if i := strings.LastIndex(rest, ":"); i > 0 {
		if p, err := strconv.Atoi(rest[i+1:]); err == nil {
			host = rest[:i]
			port = p
		}
	}
	return scheme, host, port, path, true
}

// EnsureSingboxV2RayAPI 确保 experimental.v2ray_api 存在并启用流量统计。
//
// 面板统一使用 v2ray_api 作为 sing-box 的统计通道，理由：
//   - 它与 xray 的 StatsService 协议同名，可直接复用现有 gRPC 客户端与解析逻辑
//     （trafficsync → coredata.XrayClient.RawStats → ParseStatsResponse）；
//   - 提供 user>>>name>>>traffic>>>uplink/downlink 计数，且支持 reset_ 原子取增量；
//   - clash_api 的 /connections 不暴露 user 字段，无法按订阅用户出账。
//
// 两个前置条件（否则统计拿不到数据）：
//  1. 节点二进制必须带 with_v2ray_api —— 官方 release 不含该 tag，缺失时核心启动即
//     FATAL（create v2ray-server: v2ray api is not included in this build），
//     因此节点要用 qist/singbox-binaries 的自建产物；
//  2. stats.users / stats.inbounds 是白名单（见 PopulateSingboxStatsTargets），
//     没列进去的标签/用户不会产生计数器。
//
// 本函数不再自动创建 clash_api（默认不配置）。若用户显式配置了 clash_api 且与
// v2ray_api 抢同一端口，则自动为其换到备用端口，避免两个监听互相冲突。
//
// https://sing-box.sagernet.org/configuration/experimental/v2ray-api/
func EnsureSingboxV2RayAPI(fullConfig map[string]interface{}) {
	exp, ok := fullConfig["experimental"].(map[string]interface{})
	if !ok || exp == nil {
		exp = map[string]interface{}{}
		fullConfig["experimental"] = exp
	}

	v2api, ok := exp["v2ray_api"].(map[string]interface{})
	if !ok || v2api == nil {
		v2api = map[string]interface{}{}
		exp["v2ray_api"] = v2api
	}
	listen, _ := v2api["listen"].(string)
	if listen == "" {
		listen = defaultSingboxV2RayAPIListen
		v2api["listen"] = listen
	}
	stats, ok := v2api["stats"].(map[string]interface{})
	if !ok || stats == nil {
		stats = map[string]interface{}{}
		v2api["stats"] = stats
	}
	stats["enabled"] = true

	// clash_api 是可选的（默认不配置）。仅在用户显式配置且端口与 v2ray_api 冲突时避让。
	if clashAPI, ok := exp["clash_api"].(map[string]interface{}); ok && clashAPI != nil {
		if ctrl, _ := clashAPI["external_controller"].(string); ctrl != "" && ctrl == listen {
			clashAPI["external_controller"] = "127.0.0.1:9090"
			log.Printf("[PrepareFullConfig] clash_api 与 v2ray_api 端口冲突(%s)，已改为 127.0.0.1:9090", listen)
		}
	}
}

// PopulateSingboxStatsTargets 自动填充 v2ray_api.stats 的白名单。
//
// 这是 sing-box 与 xray 最大的差异点：xray 用 policy.levels.0.statsUserUplink/
// statsUserDownlink 统一开启 per-user 统计，而 sing-box 的 stats 是**显式白名单** ——
// 只有当 inbound tag / outbound tag / 用户名出现在 stats.inbounds / stats.outbounds /
// stats.users 中，才会创建对应计数器（见 sing-box experimental/v2rayapi/stats.go 的
// RoutedConnection：未命中白名单直接 return，不计数）。
//
// 因此这里必须把配置中所有用户补进 stats.users，否则面板拿不到 user>>> 计数，
// sing-box 节点的按用户计费会恒为 0（v2ray_api 白装了）。
// 同时补齐 inbounds/outbounds 维度：入站全部纳入（含 tun/tproxy 等网关场景），
// 出站排除 direct/block/dns/selector/urltest 这类非代理出站。
//
// 合并策略：保留用户已有配置并追加缺失项，不覆盖已有值。
func PopulateSingboxStatsTargets(fullConfig map[string]interface{}) {
	var inboundTags []string
	var userNames []string
	if inbounds, ok := fullConfig["inbounds"].([]interface{}); ok {
		for _, ib := range inbounds {
			m, ok := ib.(map[string]interface{})
			if !ok {
				continue
			}
			if tag, _ := m["tag"].(string); tag != "" {
				inboundTags = append(inboundTags, tag)
			}
			// 用户名字段：vless/vmess/trojan/ss/hysteria2/tuic/anytls/snell 等用 users[].name，
			// socks/http/mixed 用 users[].username。
			users, _ := m["users"].([]interface{})
			for _, u := range users {
				um, ok := u.(map[string]interface{})
				if !ok {
					continue
				}
				if n, _ := um["name"].(string); n != "" {
					userNames = append(userNames, n)
				}
				if n, _ := um["username"].(string); n != "" {
					userNames = append(userNames, n)
				}
			}
		}
	}

	// Collect all outbound tags (only proxy outbounds, not direct/block/dns)
	var outboundTags []string
	if outbounds, ok := fullConfig["outbounds"].([]interface{}); ok {
		for _, ob := range outbounds {
			if m, ok := ob.(map[string]interface{}); ok {
				tag, _ := m["tag"].(string)
				if tag == "" {
					continue
				}
				switch m["type"] {
				case "direct", "block", "dns", "selector", "urltest":
					continue
				}
				outboundTags = append(outboundTags, tag)
			}
		}
	}

	// Merge into stats section
	exp, ok := fullConfig["experimental"].(map[string]interface{})
	if !ok {
		return
	}
	v2api, ok := exp["v2ray_api"].(map[string]interface{})
	if !ok {
		return
	}
	stats, ok := v2api["stats"].(map[string]interface{})
	if !ok {
		return
	}

	mergeStatsList(stats, "inbounds", inboundTags)
	mergeStatsList(stats, "outbounds", outboundTags)
	mergeStatsList(stats, "users", userNames)
}

// mergeStatsList 把 targets 合并进 stats[key]（保留已有值并追加缺失项）。
func mergeStatsList(stats map[string]interface{}, key string, targets []string) {
	if len(targets) == 0 {
		return
	}
	merged := mergeUnique(toStringSlice(stats[key]), targets)
	if len(merged) > 0 {
		stats[key] = merged
	}
}

// toStringSlice converts an interface{} to []string.
func toStringSlice(v interface{}) []string {
	switch s := v.(type) {
	case []string:
		return s
	case []interface{}:
		out := make([]string, 0, len(s))
		for _, item := range s {
			if str, ok := item.(string); ok {
				out = append(out, str)
			}
		}
		return out
	default:
		return nil
	}
}

// mergeUnique merges two string slices, skipping duplicates.
func mergeUnique(a, b []string) []string {
	seen := make(map[string]bool, len(a)+len(b))
	result := make([]string, 0, len(a)+len(b))
	for _, s := range a {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	for _, s := range b {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// ---------------------------------------------------------------------------
// ChainLink sing-box format converters
// ---------------------------------------------------------------------------

// convertChainOutboundsToSingbox converts xray-format chain outbounds
// (generated by ChainLinkGenerator) to sing-box format.
//
// xray format:
//
//	{"tag":"chain-1-2", "protocol":"vless",
//	 "settings":{"vnext":[{"address":"1.2.3.4","port":443,
//	 "users":[{"id":"uuid","email":"chain-relay","encryption":"none"}]}]},
//	 "streamSettings":{"network":"tcp","security":"none"}}
//
// sing-box format:
//
//	{"tag":"chain-1-2", "type":"vless",
//	 "server":"1.2.3.4", "server_port":443,
//	 "uuid":"uuid"}
//
// isZeroPort 判断端口值是否为 0/缺失（这类值不能写进 sing-box 的 server_port：
// 写 0 能过 `check`，但运行期连不上 ✗）。
func isZeroPort(v interface{}) bool {
	switch n := v.(type) {
	case int:
		return n == 0
	case int64:
		return n == 0
	case float64:
		return n == 0
	case nil:
		return true
	}
	return false
}

func convertChainOutboundsToSingbox(outbounds []interface{}) []interface{} {
	result := make([]interface{}, 0, len(outbounds))
	for _, ob := range outbounds {
		m, ok := ob.(map[string]interface{})
		if !ok {
			result = append(result, ob) // pass through if not a map
			continue
		}
		protocol, _ := m["protocol"].(string)
		tag, _ := m["tag"].(string)
		settings, _ := m["settings"].(map[string]interface{})

		sb := map[string]interface{}{
			"tag":  tag,
			"type": protocol,
		}

		// Extract server/port/uuid/password from xray settings
		switch protocol {
		case "vless", "vmess":
			if vnext, ok := settings["vnext"].([]interface{}); ok && len(vnext) > 0 {
				if first, ok := vnext[0].(map[string]interface{}); ok {
					sb["server"] = first["address"]
					sb["server_port"] = first["port"]
					if users, ok := first["users"].([]interface{}); ok && len(users) > 0 {
						if user, ok := users[0].(map[string]interface{}); ok {
							sb["uuid"] = user["id"]
						}
					}
				}
			}
		case "trojan":
			if servers, ok := settings["servers"].([]interface{}); ok && len(servers) > 0 {
				if first, ok := servers[0].(map[string]interface{}); ok {
					sb["server"] = first["address"]
					sb["server_port"] = first["port"]
					sb["password"] = first["password"]
				}
			}
		case "shadowsocks":
			if servers, ok := settings["servers"].([]interface{}); ok && len(servers) > 0 {
				if first, ok := servers[0].(map[string]interface{}); ok {
					sb["server"] = first["address"]
					sb["server_port"] = first["port"]
					sb["password"] = first["password"]
					sb["method"] = first["method"]
				}
			}
		case "hysteria2":
			// 链式中间形状：settings.servers[0] = {address, port, password, obfs?, serverPorts?}
			// （由 agent.buildOutboundFromExitInbound 针对"出口已有 hy2 入站"生成；
			//   外部 hy2 节点则由 agent.buildExternalOutbound 生成）
			if servers, ok := settings["servers"].([]interface{}); ok && len(servers) > 0 {
				if first, ok := servers[0].(map[string]interface{}); ok {
					sb["server"] = first["address"]
					// 端口：`serverPorts`（端口跳跃）优先 —— 此时**不要**再写 server_port ✗：
					// mihomo 的 port 常为 0，写 `server_port: 0` 不会被 `sing-box check` 拦下
					// （core 不校验端口范围 ✗），但运行期连不上 —— 正是"静默失效"✗。
					if ports, ok := first["serverPorts"].([]interface{}); ok && len(ports) > 0 {
						sb["server_ports"] = ports
					} else if v, ok := first["port"]; ok && !isZeroPort(v) {
						sb["server_port"] = v
					}
					sb["password"] = first["password"]
					if obfs, ok := first["obfs"].(map[string]interface{}); ok && len(obfs) > 0 {
						sb["obfs"] = obfs
					}
				}
			}
		default:
			// Fall back: try vnext format
			if vnext, ok := settings["vnext"].([]interface{}); ok && len(vnext) > 0 {
				if first, ok := vnext[0].(map[string]interface{}); ok {
					sb["server"] = first["address"]
					sb["server_port"] = first["port"]
					if users, ok := first["users"].([]interface{}); ok && len(users) > 0 {
						if user, ok := users[0].(map[string]interface{}); ok {
							sb["uuid"] = user["id"]
						}
					}
				}
			}
		}

		// Convert streamSettings to transport + tls
		if ss, ok := m["streamSettings"].(map[string]interface{}); ok {
			network, _ := ss["network"].(string)
			security, _ := ss["security"].(string)
			// ⚠ 传输名**不能直接透传** ✗：`raw` 是 xray 对 TCP 的新叫法，sing-box 里没有这个类型，
			// 透传会导致核心拒绝启动：
			//   FATAL decode config at runtime/singbox.jsonc: outbounds[N].transport: unknown transport type: raw
			// （真机 E2E 撞到：面板给 xray 入站默认写 network=raw，而 sing-box 入口链到这种出口就会炸 ✗）
			// → TCP 在 sing-box 里就是"没有 transport"，不写这个字段；其余按 sing-box 的类型名映射。
			switch network {
			case "", "tcp", "raw":
				// 不写 transport（即 TCP）
			case "splithttp", "xhttp":
				// xray 的 xhttp 在 sing-box 里没有对应传输 ✗ —— 不生成非法字段，交由上层核心能力矩阵拦截
			default:
				transport := map[string]interface{}{"type": network}
				// ⚠ 传输细节在 xray 里是**嵌套**的 ✗，顶层通常没有：
				//   ws          → wsSettings.path / wsSettings.headers.Host
				//   grpc        → grpcSettings.serviceName
				//   httpupgrade → httpupgradeSettings.path
				// 旧实现只读顶层 `ss["path"]` / `ss["serviceName"]` ✗ → ws 的 path、gRPC 的
				// serviceName 会被**静默丢掉** ✗：gRPC 对不上服务名、ws 落到默认路径 → 连不上，
				// 而 `sing-box check` 照样通过（这两个字段都可选）✗。真机撞到 ✓。
				// → 先读嵌套、再回落到顶层（顶层是少数派生路径写的，保持兼容）。
				path := ""
				if ws, ok := ss["wsSettings"].(map[string]interface{}); ok {
					if v, ok := ws["path"].(string); ok {
						path = v
					}
				}
				if path == "" {
					if hu, ok := ss["httpupgradeSettings"].(map[string]interface{}); ok {
						if v, ok := hu["path"].(string); ok {
							path = v
						}
					}
				}
				if path == "" {
					if v, ok := ss["path"].(string); ok {
						path = v
					}
				}
				if path != "" {
					transport["path"] = path
				}
				sn := ""
				if grpc, ok := ss["grpcSettings"].(map[string]interface{}); ok {
					if v, ok := grpc["serviceName"].(string); ok {
						sn = v
					}
				}
				if sn == "" {
					if v, ok := ss["serviceName"].(string); ok {
						sn = v
					}
				}
				if sn != "" {
					transport["service_name"] = sn
				}
				// ws 的 Host：xray 放在 wsSettings.headers.Host。
				// ⚠ sing-box 的 ws transport **没有 `host` 字段** ✗（实测：写顶层 host →
				//   `FATAL decode config: outbounds[N].transport.host: json: unknown field "host"`，
				//   节点直接起不来）→ 必须写进 `headers.Host` ✓（已用真实核心验证）。
				if ws, ok := ss["wsSettings"].(map[string]interface{}); ok {
					if hdrs, ok := ws["headers"].(map[string]interface{}); ok {
						if h, ok := hdrs["Host"].(string); ok && h != "" {
							transport["headers"] = map[string]interface{}{"Host": h}
						}
					}
				}
				sb["transport"] = transport
			}
			if security == "tls" || security == "reality" {
				tls := map[string]interface{}{"enabled": true}
				// SNI：reality 场景 xray 把它放在 realitySettings.serverName（顶层 serverName 常为空），
				// 所以先取 realitySettings 再回落到顶层，否则收不到正确 SNI（核心不会报错，
				// 只会拿服务器地址当 SNI → 连不上真实站点）。
				// SNI 可能出现在三处：streamSettings.serverName（老路径）/ tlsSettings.serverName
				// （agent 从真实入站派生时写的）/ realitySettings.serverName（reality 入站）。
				var sni string
				ts, _ := ss["tlsSettings"].(map[string]interface{})
				if v, ok := ss["serverName"].(string); ok && v != "" {
					sni = v
				}
				if sni == "" && ts != nil {
					if v, ok := ts["serverName"].(string); ok && v != "" {
						sni = v
					}
				}
				if r, ok := ss["realitySettings"].(map[string]interface{}); ok {
					if v, ok := r["serverName"].(string); ok && v != "" {
						sni = v
					}
				}
				if sni != "" {
					tls["server_name"] = sni
				}
				// 证书校验：外部节点可能开了 skip-cert-verify（自签证书），
				// 中间形状写在顶层或 tlsSettings（agent 派生时两边都可能）。
				if v, ok := ss["allowInsecure"].(bool); ok && v {
					tls["insecure"] = true
				} else if ts != nil {
					if v, ok := ts["allowInsecure"].(bool); ok && v {
						tls["insecure"] = true
					}
				}
				fp := ""
				if v, ok := ss["fingerprint"].(string); ok {
					fp = v
				}
				if fp == "" && ts != nil {
					if v, ok := ts["fingerprint"].(string); ok {
						fp = v
					}
				}
				if fp != "" {
					tls["utls"] = map[string]interface{}{"enabled": true, "fingerprint": fp}
				}
				// alpn：顶层优先，其次 tlsSettings（agent 派生时写在顶层，这里都兼容）
				if alpn, ok := ss["alpn"].([]interface{}); ok && len(alpn) > 0 {
					tls["alpn"] = alpn
				} else if ts != nil {
					if alpn, ok := ts["alpn"].([]interface{}); ok && len(alpn) > 0 {
						tls["alpn"] = alpn
					}
				}
				if security == "reality" {
					reality := map[string]interface{}{"enabled": true}
					if r, ok := ss["realitySettings"].(map[string]interface{}); ok {
						if v, ok := r["publicKey"].(string); ok && v != "" {
							reality["public_key"] = v
						}
						if v, ok := r["shortId"].(string); ok && v != "" {
							reality["short_id"] = v
						}
						if v, ok := r["fingerprint"].(string); ok && v != "" {
							tls["utls"] = map[string]interface{}{"enabled": true, "fingerprint": v}
						}
					}
					tls["reality"] = reality
					// reality 客户端**必须**有 uTLS（实测：uTLS is required by reality client）
					if _, ok := tls["utls"]; !ok {
						tls["utls"] = map[string]interface{}{"enabled": true, "fingerprint": "chrome"}
					}
				}
				sb["tls"] = tls
			}
		}

		result = append(result, sb)
	}
	return result
}

// convertChainRulesToSingbox converts xray-format routing rules to sing-box format.
//
// xray:  {"type":"field", "inboundTag":["in-1"], "outboundTag":"chain-1-2"}
// sing-box: {"inbound":["in-1"], "outbound":"chain-1-2"}
//
// balancerTag 规则（多出口）在 sing-box 里改指向 urltest outbound：
// balancerTagMap 由 convertChainBalancersToSingbox 生成（balancerTag → urltest 标签）。
// 若某个 balancer 没有解析出任何成员（map 里没有它），该规则会被丢弃并打日志 ——
// 丢弃会导致入口流量落到 route.final，因此这种情况需要在日志里可见。
func convertChainRulesToSingbox(rules []interface{}, balancerTagMap map[string]string) []interface{} {
	result := make([]interface{}, 0, len(rules))
	for _, r := range rules {
		m, ok := r.(map[string]interface{})
		if !ok {
			continue
		}
		sb := map[string]interface{}{}
		if inTag, ok := m["inboundTag"].([]interface{}); ok {
			tags := make([]string, 0, len(inTag))
			for _, t := range inTag {
				if s, ok := t.(string); ok {
					tags = append(tags, s)
				}
			}
			if len(tags) > 0 {
				sb["inbound"] = tags
			}
		}
		// 出口：优先 outboundTag；多出口时用 balancerTag 映射出的 urltest 标签
		outTag, _ := m["outboundTag"].(string)
		if outTag == "" {
			if balTag, _ := m["balancerTag"].(string); balTag != "" {
				mapped, exists := balancerTagMap[balTag]
				if !exists {
					log.Printf("[PrepareFullConfig] sing-box: balancer %q 没有可用出口成员，规则被跳过（入口流量将落到 route.final）", balTag)
					continue
				}
				outTag = mapped
			}
		}
		if outTag != "" {
			sb["outbound"] = outTag
		}
		if len(sb) > 0 {
			result = append(result, sb)
		}
	}
	return result
}

// convertChainBalancersToSingbox 把 xray 的 routing.balancers[] 转成 sing-box 的
// urltest outbound（设计文档 §11.1：Balancer → outbounds[].type:"selector"/"urltest"）。
//
// xray 的 balancer 用 selector（子串/正则匹配出站 tag）选定成员，而 sing-box 的
// urltest 需要显式 outbounds 列表，因此这里用 selector 去匹配本次生成的链路出站 tag 集合。
//
// 返回：urltest outbounds + balancerTag → urltest tag 的映射（供路由规则改写）。
func convertChainBalancersToSingbox(balancers []interface{}, chainOutbounds []interface{}) ([]interface{}, map[string]string) {
	outbounds := make([]interface{}, 0, len(balancers))
	tagMap := make(map[string]string, len(balancers))

	// 本次入口生成的链路出站 tag 集合（成员只能从这里面选）
	chainTags := make([]string, 0, len(chainOutbounds))
	for _, raw := range chainOutbounds {
		m, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if tag, _ := m["tag"].(string); tag != "" {
			chainTags = append(chainTags, tag)
		}
	}

	for _, raw := range balancers {
		m, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		tag, _ := m["tag"].(string)
		if tag == "" {
			continue
		}
		var patterns []string
		if sel, ok := m["selector"].([]interface{}); ok {
			for _, s := range sel {
				if str, ok := s.(string); ok && str != "" {
					patterns = append(patterns, str)
				}
			}
		}
		members := make([]string, 0, len(chainTags))
		seen := map[string]bool{}
		for _, ct := range chainTags {
			if seen[ct] {
				continue
			}
			for _, p := range patterns {
				if strings.Contains(ct, p) {
					members = append(members, ct)
					seen[ct] = true
					break
				}
			}
		}
		if len(members) == 0 {
			log.Printf("[PrepareFullConfig] sing-box: balancer %q 的 selector %v 未匹配到任何链路出站", tag, patterns)
			continue
		}
		outbounds = append(outbounds, map[string]interface{}{
			"type":         "urltest",
			"tag":          tag,
			"outbounds":    members,
			"url":          "https://www.gstatic.com/generate_204",
			"interval":     "3m",
			"tolerance":    50,
			"idle_timeout": "30m",
		})
		tagMap[tag] = tag
	}
	return outbounds, tagMap
}
