package coreconfig

import (
	"context"
	"encoding/json"
	"log"
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

// ChainLinkProvider provides chain proxy outbounds, routing rules, and relay
// inbound for a server. Implemented by the ChainLinkGenerator in the agent
// package. When set, PrepareFullConfig merges chain link outbounds and
// routing rules into the config, and adds a relay-in inbound if the server
// is configured as a chain exit.
type ChainLinkProvider interface {
	// GenerateEntryOutbounds returns chain proxy outbounds and routing rules
	// for the entry side of this server.
	GenerateEntryOutbounds(ctx context.Context, serverID int64) (outbounds []interface{}, routingRules []interface{}, err error)
	// GenerateEntryBalancers returns routing.balancers[] entries and the root
	// observatory config for load-balanced (multi-exit) entries. Returns nil
	// slices when no entry uses load balancing. See design doc §6.5 / §6.6.
	GenerateEntryBalancers(ctx context.Context, serverID int64) (balancers []interface{}, observatory map[string]interface{})
	// GenerateExitInbound returns the relay-in inbound config map for the
	// exit side of this server, or nil if not an exit.
	GenerateExitInbound(ctx context.Context, serverID int64) (map[string]interface{}, error)
	// IsRelayOnlyServer returns true if the server is a pure exit (no user inbounds).
	IsRelayOnlyServer(ctx context.Context, serverID int64) bool
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

	// 8b. If ChainLinkProvider is set, merge chain link outbounds and routing rules.
	if opts.ChainLinkProvider != nil {
		chainOutbounds, chainRules, err := opts.ChainLinkProvider.GenerateEntryOutbounds(opts.Ctx, opts.ServerID)
		if err != nil {
			log.Printf("[PrepareFullConfig] chain link outbounds failed for server %d: %v", opts.ServerID, err)
		} else {
			// For sing-box: convert xray-format chain outbounds/rules to sing-box format
			if opts.CoreType == "singbox" {
				chainOutbounds = convertChainOutboundsToSingbox(chainOutbounds)
				chainRules = convertChainRulesToSingbox(chainRules)
			}
			if len(chainOutbounds) > 0 {
				existingObs, _ := fullConfig["outbounds"].([]interface{})
				fullConfig["outbounds"] = mergeChainOutbounds(existingObs, chainOutbounds)
			}
			if len(chainRules) > 0 {
				routing, _ := fullConfig["routing"].(map[string]interface{})
				if routing == nil {
					routing = map[string]interface{}{}
				}
				existingRules, _ := routing["rules"].([]interface{})
				routing["rules"] = mergeChainRoutingRules(existingRules, chainRules)

				// 多出口负载均衡（设计文档 §6.6）：合并 balancers 与根 observatory
				// sing-box 不使用 balancers/observatory（用 selector/urltest outbound 代替）
				if opts.CoreType != "singbox" {
					chainBalancers, chainObservatory := opts.ChainLinkProvider.GenerateEntryBalancers(opts.Ctx, opts.ServerID)
					if len(chainBalancers) > 0 {
						existingBal, _ := routing["balancers"].([]interface{})
						routing["balancers"] = append(existingBal, chainBalancers...)
					}
					if chainObservatory != nil {
						// §6.6.5: 当 Sampling > 0 时使用 burstObservatory（特征弱，适合入口节点）
						// 否则使用标准 observatory
						if sampling, ok := chainObservatory["sampling"]; ok && sampling != nil {
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
				fullConfig["routing"] = routing
			}
		}

		// 8c. If this server is a chain exit, add relay-in inbound.
		exitInbound, err := opts.ChainLinkProvider.GenerateExitInbound(opts.Ctx, opts.ServerID)
		if err != nil {
			log.Printf("[PrepareFullConfig] chain exit inbound failed for server %d: %v", opts.ServerID, err)
		} else if exitInbound != nil {
			// For sing-box: convert xray-format relay-in inbound to sing-box format
			if opts.CoreType == "singbox" {
				exitInbound = convertRelayInboundToSingbox(exitInbound)
			}
			existingInbounds, _ := fullConfig["inbounds"].([]interface{})
			fullConfig["inbounds"] = append(existingInbounds, exitInbound)
		}

		// 8d. If this server is relay_only, filter out user inbounds
		// (keep only relay-in and api inbounds).
		if opts.ChainLinkProvider.IsRelayOnlyServer(opts.Ctx, opts.ServerID) {
			filterRelayOnlyInbounds(fullConfig)
		}
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
	data, err := json.MarshalIndent(fullConfig, "", "  ")
	if err != nil {
		return nil, err
	}
	return data, nil
}

// EnsureDefaultOutbounds checks that the standard default outbounds exist in
// the final config. For xray, these are "direct" (freedom) and "blocked"
// (blackhole). For singbox, these are "direct", "block", and "dns-out".
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
			"tag":     "direct",
		})
		log.Printf("[PrepareFullConfig] added missing 'direct' (freedom) outbound")
	}
	if !hasBlocked {
		outbounds = append(outbounds, map[string]interface{}{
			"protocol": "blackhole",
			"tag":     "blocked",
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
func CleanInboundFields(inbound map[string]interface{}, tag, listen, protocol string, port int) {
	// Ensure required fields are set from the record
	if tag != "" {
		inbound["tag"] = tag
	}
	if listen != "" {
		inbound["listen"] = listen
	}
	if port > 0 {
		inbound["port"] = port
	}
	if protocol != "" {
		inbound["protocol"] = protocol
	}
	// Remove internal fields
	delete(inbound, "_remark")
	delete(inbound, "_enabled")
	delete(inbound, "_listenType")
	delete(inbound, "_socketPath")
	delete(inbound, "_socketPerm")
	delete(inbound, "_wgPublicKey")
	delete(inbound, "_accessDomain")
	delete(inbound, "_portHop")
	// Clean nested _ fields from users/clients/accounts and filter paused users
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

// filterRelayOnlyInbounds removes user inbounds from the config, keeping only
// relay-in and api inbounds. This is used for pure exit nodes (relay_only=true)
// that should not accept user connections.
func filterRelayOnlyInbounds(fullConfig map[string]interface{}) {
	inbounds, ok := fullConfig["inbounds"].([]interface{})
	if !ok {
		return
	}
	filtered := make([]interface{}, 0, len(inbounds))
	for _, ib := range inbounds {
		ibMap, ok := ib.(map[string]interface{})
		if !ok {
			filtered = append(filtered, ib)
			continue
		}
		tag, _ := ibMap["tag"].(string)
		// Keep only relay-in and api inbounds for pure exit nodes
		if tag == "relay-in" || tag == "api" {
			filtered = append(filtered, ib)
		}
	}
	fullConfig["inbounds"] = filtered
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

// EnsureSingboxV2RayAPI ensures the experimental section exists for traffic
// statistics. It prefers v2ray_api (gRPC stats, same protocol as xray) when
// the binary supports it, falling back to clash_api otherwise.
//
// Since sing-box 1.15+ default builds don't include the with_v2ray_api tag,
// we use clash_api as the traffic stats backend. The Panel's trafficsync
// service can query clash_api's /connections endpoint for per-user traffic.
//
// https://sing-box.sagernet.org/configuration/experimental/v2ray-api/
// https://sing-box.sagernet.org/configuration/experimental/clash-api/
func EnsureSingboxV2RayAPI(fullConfig map[string]interface{}) {
	exp, ok := fullConfig["experimental"].(map[string]interface{})
	if !ok {
		exp = map[string]interface{}{}
		fullConfig["experimental"] = exp
	}

	// If v2ray_api is already present (user explicitly configured it),
	// respect that choice and ensure stats is enabled.
	if v2api, ok := exp["v2ray_api"].(map[string]interface{}); ok {
		if _, ok := v2api["listen"].(string); !ok {
			v2api["listen"] = "127.0.0.1:10086"
		}
		stats, ok := v2api["stats"].(map[string]interface{})
		if !ok {
			stats = map[string]interface{}{}
			v2api["stats"] = stats
		}
		stats["enabled"] = true
		return
	}

	// Otherwise, ensure clash_api exists for traffic monitoring.
	// clash_api provides connection stats via RESTful API.
	clashAPI, ok := exp["clash_api"].(map[string]interface{})
	if !ok {
		clashAPI = map[string]interface{}{}
		exp["clash_api"] = clashAPI
	}
	if _, ok := clashAPI["external_controller"].(string); !ok {
		clashAPI["external_controller"] = "127.0.0.1:10086"
	}
}

// PopulateSingboxStatsTargets populates the v2ray_api.stats.inbounds/outbounds
// lists with all inbound and outbound tags so that traffic counters are created
// for every proxy endpoint. This mirrors the xray policy "statsInboundUplink/
// statsInboundDownlink" mechanism — sing-box uses explicit tag lists instead.
//
// If the user already specified inbounds/outbounds in stats, we merge (append
// missing tags) rather than replace.
func PopulateSingboxStatsTargets(fullConfig map[string]interface{}) {
	// Collect all inbound tags
	var inboundTags []string
	if inbounds, ok := fullConfig["inbounds"].([]interface{}); ok {
		for _, ib := range inbounds {
			if m, ok := ib.(map[string]interface{}); ok {
				if tag, _ := m["tag"].(string); tag != "" {
					// Skip internal inbounds that don't need per-tag stats
					switch m["type"] {
					case "tun", "redirect", "tuic":
						continue
					}
					inboundTags = append(inboundTags, tag)
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

	// Merge inbound tags
	if len(inboundTags) > 0 {
		existing := toStringSlice(stats["inbounds"])
		merged := mergeUnique(existing, inboundTags)
		if len(merged) > 0 {
			stats["inbounds"] = merged
		}
	}
	// Merge outbound tags
	if len(outboundTags) > 0 {
		existing := toStringSlice(stats["outbounds"])
		merged := mergeUnique(existing, outboundTags)
		if len(merged) > 0 {
			stats["outbounds"] = merged
		}
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
			if network != "" && network != "tcp" {
				transport := map[string]interface{}{"type": network}
				if path, ok := ss["path"].(string); ok {
					transport["path"] = path
				}
				if sn, ok := ss["serviceName"].(string); ok {
					transport["service_name"] = sn
				}
				sb["transport"] = transport
			}
			if security == "tls" {
				sb["tls"] = map[string]interface{}{"enabled": true}
				if sni, ok := ss["serverName"].(string); ok && sni != "" {
					sb["tls"].(map[string]interface{})["server_name"] = sni
				}
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
// For balancer rules (balancerTag set), sing-box uses selector/urltest outbounds
// instead of balancers. These rules are skipped (handled by selector outbounds).
func convertChainRulesToSingbox(rules []interface{}) []interface{} {
	result := make([]interface{}, 0, len(rules))
	for _, r := range rules {
		m, ok := r.(map[string]interface{})
		if !ok {
			continue
		}
		// Skip balancer rules — sing-box doesn't use balancers
		if balTag, ok := m["balancerTag"].(string); ok && balTag != "" {
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
		if outTag, ok := m["outboundTag"].(string); ok && outTag != "" {
			sb["outbound"] = outTag
		}
		if len(sb) > 0 {
			result = append(result, sb)
		}
	}
	return result
}

// convertRelayInboundToSingbox converts xray-format relay-in inbound to sing-box format.
//
// xray:  {"tag":"relay-in","protocol":"vless","listen":"0.0.0.0","port":443,
//	  "settings":{"clients":[{"id":"uuid","email":"chain-relay"}]}}
// sing-box: {"tag":"relay-in","type":"vless","listen":"0.0.0.0","listen_port":443,
//	  "users":[{"name":"chain-relay","uuid":"uuid"}]}
func convertRelayInboundToSingbox(inbound map[string]interface{}) map[string]interface{} {
	protocol, _ := inbound["protocol"].(string)
	tag, _ := inbound["tag"].(string)
	listen, _ := inbound["listen"].(string)
	port := inbound["port"]

	sb := map[string]interface{}{
		"tag":    tag,
		"type":   protocol,
		"listen": listen,
	}
	if port != nil {
		sb["listen_port"] = port
	}

	// Convert settings.clients to users[]
	if settings, ok := inbound["settings"].(map[string]interface{}); ok {
		if clients, ok := settings["clients"].([]interface{}); ok {
			users := make([]interface{}, 0, len(clients))
			for _, c := range clients {
				cm, ok := c.(map[string]interface{})
				if !ok {
					continue
				}
				user := map[string]interface{}{}
				if email, ok := cm["email"].(string); ok && email != "" {
					user["name"] = email
				}
				if id, ok := cm["id"].(string); ok && id != "" {
					user["uuid"] = id
				}
				if pw, ok := cm["password"].(string); ok && pw != "" {
					user["password"] = pw
				}
				users = append(users, user)
			}
			if len(users) > 0 {
				sb["users"] = users
			}
		}
	}

	// Convert streamSettings if present (for TLS/transport)
	if ss, ok := inbound["streamSettings"].(map[string]interface{}); ok {
		security, _ := ss["security"].(string)
		if security == "tls" {
			tls := map[string]interface{}{"enabled": true}
			if sni, ok := ss["serverName"].(string); ok && sni != "" {
				tls["server_name"] = sni
			}
			sb["tls"] = tls
		}
		network, _ := ss["network"].(string)
		if network != "" && network != "tcp" {
			transport := map[string]interface{}{"type": network}
			if path, ok := ss["path"].(string); ok {
				transport["path"] = path
			}
			if sn, ok := ss["serviceName"].(string); ok {
				transport["service_name"] = sn
			}
			sb["transport"] = transport
		}
	}

	return sb
}
