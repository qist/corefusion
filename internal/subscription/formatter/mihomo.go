package formatter

import ()

// MihomoConfig holds the full Mihomo configuration sections.
type MihomoConfig struct {
	Listeners      []map[string]interface{} `json:"listeners"`
	ProxyGroups    []map[string]interface{} `json:"proxy_groups"`
	RuleProviders  map[string]interface{}   `json:"rule_providers"`
	ProxyProviders map[string]interface{}   `json:"proxy_providers"`
	DNS            map[string]interface{}   `json:"dns"`
	General        map[string]interface{}   `json:"general"`
	Rules          []RuleEntry              `json:"rules"`
}

// RuleEntry represents a single rule.
type RuleEntry struct {
	RuleType     string `json:"rule_type"`
	MatchValue   string `json:"match_value"`
	OutboundTag  string `json:"outbound_tag"`
	RuleProvider string `json:"rule_provider"`
	RuleAction   string `json:"rule_action"`
}
