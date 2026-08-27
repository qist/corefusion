package coreconfig

import (
	"context"
	"encoding/json"
	"log"
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

	// 5. Ensure stats section exists
	if _, exists := fullConfig["stats"]; !exists {
		fullConfig["stats"] = map[string]interface{}{}
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
				fullConfig["routing"] = routing
			}
		}

		// 8c. If this server is a chain exit, add relay-in inbound.
		exitInbound, err := opts.ChainLinkProvider.GenerateExitInbound(opts.Ctx, opts.ServerID)
		if err != nil {
			log.Printf("[PrepareFullConfig] chain exit inbound failed for server %d: %v", opts.ServerID, err)
		} else if exitInbound != nil {
			existingInbounds, _ := fullConfig["inbounds"].([]interface{})
			fullConfig["inbounds"] = append(existingInbounds, exitInbound)
		}

		// 8d. If this server is relay_only, filter out user inbounds
		// (keep only relay-in and api inbounds).
		if opts.ChainLinkProvider.IsRelayOnlyServer(opts.Ctx, opts.ServerID) {
			filterRelayOnlyInbounds(fullConfig)
		}
	}

	// 9. Ensure default outbounds (direct, blocked) exist — safety net so every
	// node always has freedom+blackhole even if missing from DB.
	EnsureDefaultOutbounds(fullConfig, opts.CoreType)

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
// (blackhole). If they are missing, they are appended. This guarantees that
// every node — local or remote — always has these two essential outbounds,
// even if the outbound_configs table doesn't have them for that server.
func EnsureDefaultOutbounds(fullConfig map[string]interface{}, coreType string) {
	if coreType != "xray" {
		return
	}
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
