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

// PrepareFullConfigOptions holds the parameters for PrepareFullConfig.
type PrepareFullConfigOptions struct {
	// Ctx is the context for database queries.
	Ctx context.Context
	// ServerID is the target server ID for loading inbounds/outbounds.
	ServerID int64
	// CoreType is the proxy core type (xray, singbox, etc.).
	CoreType string
	// FullConfig is the raw merged config from core_configs table.
	// This map is modified in place.
	FullConfig map[string]interface{}
	// InboundProvider optionally provides enabled inbound configs.
	InboundProvider InboundConfigProvider
	// OutboundProvider optionally provides enabled outbound configs.
	OutboundProvider OutboundConfigProvider
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
	if opts.OutboundProvider != nil {
		enabledOutbounds, err := opts.OutboundProvider.GetEnabledOutbounds(opts.Ctx, opts.ServerID, opts.CoreType)
		if err != nil {
			log.Printf("[PrepareFullConfig] failed to load outbounds for server %d: %v", opts.ServerID, err)
		} else if len(enabledOutbounds) > 0 {
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

	// 9. Marshal to JSON
	data, err := json.MarshalIndent(fullConfig, "", "  ")
	if err != nil {
		return nil, err
	}
	return data, nil
}

// CleanEmpty removes nil values, empty strings, empty arrays, and empty maps
// recursively. This matches the cleanup logic previously in router.go.
func CleanEmpty(m map[string]interface{}) {
	for k, v := range m {
		if v == nil {
			delete(m, k)
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
