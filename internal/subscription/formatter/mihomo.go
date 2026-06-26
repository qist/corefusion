package formatter

import (
	"encoding/json"
	"fmt"
	"strings"

	"corefusion/internal/model/entity"
)

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

// FormatMihomo generates a complete Mihomo YAML config.
func FormatMihomo(nodes []Node, config *MihomoConfig) ([]byte, error) {
	var yaml strings.Builder

	// General section
	yaml.WriteString("mode: rule\n")
	yaml.WriteString("log-level: info\n")
	if config != nil && config.General != nil {
		if mode, ok := config.General["mode"].(string); ok && mode != "" {
			yaml.WriteString(fmt.Sprintf("mode: %s\n", mode))
		}
		if logLevel, ok := config.General["log_level"].(string); ok && logLevel != "" {
			yaml.WriteString(fmt.Sprintf("log-level: %s\n", logLevel))
		}
		if allowLan, ok := config.General["allow_lan"].(bool); ok {
			yaml.WriteString(fmt.Sprintf("allow-lan: %v\n", allowLan))
		}
	}
	yaml.WriteString("\n")

	// DNS section
	if config != nil && config.DNS != nil {
		yaml.WriteString("dns:\n")
		if enable, ok := config.DNS["enable"].(bool); ok {
			yaml.WriteString(fmt.Sprintf("  enable: %v\n", enable))
		}
		if nameserver, ok := config.DNS["nameserver"].([]interface{}); ok && len(nameserver) > 0 {
			yaml.WriteString("  nameserver:\n")
			for _, ns := range nameserver {
				yaml.WriteString(fmt.Sprintf("    - %v\n", ns))
			}
		}
		if fallback, ok := config.DNS["fallback"].([]interface{}); ok && len(fallback) > 0 {
			yaml.WriteString("  fallback:\n")
			for _, fb := range fallback {
				yaml.WriteString(fmt.Sprintf("    - %v\n", fb))
			}
		}
		yaml.WriteString("\n")
	}

	// Listeners section (default not enabled)
	if config != nil && len(config.Listeners) > 0 {
		yaml.WriteString("listeners:\n")
		for _, listener := range config.Listeners {
			yaml.WriteString("  - name: " + getString(listener, "name", "mixed") + "\n")
			yaml.WriteString("    type: " + getString(listener, "type", "mixed") + "\n")
			yaml.WriteString(fmt.Sprintf("    port: %d\n", getInt(listener, "port", 7890)))
			if enabled, ok := listener["enabled"].(bool); ok {
				yaml.WriteString(fmt.Sprintf("    enabled: %v\n", enabled))
			}
		}
		yaml.WriteString("\n")
	}

	// Proxy groups
	if config != nil && len(config.ProxyGroups) > 0 {
		yaml.WriteString("proxy-groups:\n")
		for _, group := range config.ProxyGroups {
			yaml.WriteString("  - name: " + getString(group, "name", "PROXY") + "\n")
			yaml.WriteString("    type: " + getString(group, "type", "select") + "\n")
			if proxies, ok := group["proxies"].([]interface{}); ok && len(proxies) > 0 {
				yaml.WriteString("    proxies:\n")
				for _, p := range proxies {
					yaml.WriteString(fmt.Sprintf("      - %v\n", p))
				}
			}
		}
		yaml.WriteString("\n")
	}

	// Proxies (from nodes)
	yaml.WriteString("proxies:\n")
	for _, node := range nodes {
		yaml.WriteString("  - name: " + node.Name + "\n")
		yaml.WriteString("    type: " + node.Protocol + "\n")
		yaml.WriteString("    server: " + node.Host + "\n")
		yaml.WriteString(fmt.Sprintf("    port: %d\n", node.Port))
		if node.UUID != "" {
			yaml.WriteString("    uuid: " + node.UUID + "\n")
		}
		if node.Password != "" {
			yaml.WriteString("    password: " + node.Password + "\n")
		}
		if node.Protocol == "vless" && node.Flow != "" {
			yaml.WriteString("    flow: " + node.Flow + "\n")
		}
		if node.Protocol == "vless" && node.Encryption != "" && node.Encryption != "none" {
			yaml.WriteString("    encryption: " + node.Encryption + "\n")
		}
		if node.TLS {
			yaml.WriteString("    tls: true\n")
			if node.ServerName != "" {
				yaml.WriteString("    servername: " + node.ServerName + "\n")
			}
		}
		if node.Transport != "" && node.Transport != "raw" && node.Transport != "tcp" {
			yaml.WriteString("    network: " + node.Transport + "\n")
		}
	}
	yaml.WriteString("\n")

	// Proxy providers
	if config != nil && len(config.ProxyProviders) > 0 {
		yaml.WriteString("proxy-providers:\n")
		for name, provider := range config.ProxyProviders {
			if p, ok := provider.(map[string]interface{}); ok {
				yaml.WriteString("  " + name + ":\n")
				yaml.WriteString("    type: " + getString(p, "type", "http") + "\n")
				if url, ok := p["url"].(string); ok && url != "" {
					yaml.WriteString("    url: \"" + url + "\"\n")
				}
				if interval, ok := p["interval"].(float64); ok {
					yaml.WriteString(fmt.Sprintf("    interval: %d\n", int(interval)))
				}
			}
		}
		yaml.WriteString("\n")
	}

	// Rule providers
	if config != nil && len(config.RuleProviders) > 0 {
		yaml.WriteString("rule-providers:\n")
		for name, provider := range config.RuleProviders {
			if p, ok := provider.(map[string]interface{}); ok {
				yaml.WriteString("  " + name + ":\n")
				yaml.WriteString("    type: " + getString(p, "type", "http") + "\n")
				yaml.WriteString("    behavior: " + getString(p, "behavior", "domain") + "\n")
				if url, ok := p["url"].(string); ok && url != "" {
					yaml.WriteString("    url: \"" + url + "\"\n")
				}
				if interval, ok := p["interval"].(float64); ok {
					yaml.WriteString(fmt.Sprintf("    interval: %d\n", int(interval)))
				}
			}
		}
		yaml.WriteString("\n")
	}

	// Rules
	if config != nil && len(config.Rules) > 0 {
		yaml.WriteString("rules:\n")
		for _, rule := range config.Rules {
			line := "  - "
			if rule.RuleProvider != "" {
				// RULE-SET style rule
				line += "RULE-SET," + rule.RuleProvider + "," + rule.OutboundTag
			} else {
				line += rule.RuleType + "," + rule.MatchValue + "," + rule.OutboundTag
			}
			if rule.RuleAction != "" {
				line += "," + rule.RuleAction
			}
			yaml.WriteString(line + "\n")
		}
	}

	return []byte(yaml.String()), nil
}

func getString(m map[string]interface{}, key, def string) string {
	if m == nil {
		return def
	}
	if v, ok := m[key].(string); ok && v != "" {
		return v
	}
	return def
}

func getInt(m map[string]interface{}, key string, def int) int {
	if m == nil {
		return def
	}
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	return def
}

// ParseMihomoConfig parses JSON strings into MihomoConfig.
func ParseMihomoConfig(listenersJSON, proxyGroupsJSON, ruleProvidersJSON, proxyProvidersJSON, dnsJSON, generalJSON string) *MihomoConfig {
	config := &MihomoConfig{}

	if listenersJSON != "" {
		json.Unmarshal([]byte(listenersJSON), &config.Listeners)
	}
	if proxyGroupsJSON != "" {
		json.Unmarshal([]byte(proxyGroupsJSON), &config.ProxyGroups)
	}
	if ruleProvidersJSON != "" {
		json.Unmarshal([]byte(ruleProvidersJSON), &config.RuleProviders)
	}
	if proxyProvidersJSON != "" {
		json.Unmarshal([]byte(proxyProvidersJSON), &config.ProxyProviders)
	}
	if dnsJSON != "" {
		json.Unmarshal([]byte(dnsJSON), &config.DNS)
	}
	if generalJSON != "" {
		json.Unmarshal([]byte(generalJSON), &config.General)
	}

	return config
}

// ConvertRulesToEntries converts database rules to RuleEntry format.
func ConvertRulesToEntries(rules []entity.SubscriptionRule) []RuleEntry {
	var entries []RuleEntry
	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		entries = append(entries, RuleEntry{
			RuleType:     r.RuleType,
			MatchValue:   r.MatchValue,
			OutboundTag:  r.OutboundTag,
			RuleProvider: r.RuleProvider,
			RuleAction:   r.RuleAction,
		})
	}
	return entries
}
