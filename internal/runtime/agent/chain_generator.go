package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// ChainConfigGenerator generates core configuration snippets for chain proxy nodes.
// It produces the outbound/routing additions that chain_entry nodes need to
// forward traffic to chain_exit nodes, and the relay inbound for chain_exit nodes.
type ChainConfigGenerator struct {
	serverRepo *repository.NodeServerRepository
}

// NewChainConfigGenerator creates a new ChainConfigGenerator.
func NewChainConfigGenerator(serverRepo *repository.NodeServerRepository) *ChainConfigGenerator {
	return &ChainConfigGenerator{serverRepo: serverRepo}
}

// ChainEntryOutbound represents an outbound config entry pointing to a chain_exit.
type ChainEntryOutbound struct {
	Tag           string                 `json:"tag"`
	Protocol      string                 `json:"protocol"`
	Settings      map[string]interface{} `json:"settings"`
	StreamSettings map[string]interface{} `json:"streamSettings,omitempty"`
}

// ChainRoutingRule represents a routing rule for chain proxy traffic.
type ChainRoutingRule struct {
	Type        string   `json:"type"`
	InboundTag  []string `json:"inboundTag,omitempty"`
	OutboundTag string   `json:"outboundTag,omitempty"`
	BalancerTag string   `json:"balancerTag,omitempty"`
	RuleTag     string   `json:"ruleTag,omitempty"`
}

// ChainBalancer represents a routing balancer object for load-balanced exits.
// Corresponds to xray routing.balancers[] (see design doc §6.6).
type ChainBalancer struct {
	Tag         string                 `json:"tag"`
	Selector    []string               `json:"selector"`
	FallbackTag string                 `json:"fallbackTag,omitempty"`
	Strategy    map[string]interface{} `json:"strategy,omitempty"`
}

// ChainObservatory holds the observatory config injected at the root of a
// load-balanced entry node (see design doc §6.6.5). subjectSelector uses the
// "chain-<inboundID>-" prefix so only this entry's exits are probed.
// 支持两种形态：
//   1. 标准 observatory（PingConfig==nil）→ {subjectSelector, probeUrl, probeInterval, enableConcurrency}
//   2. burstObservatory（PingConfig!=nil）→ {subjectSelector, pingConfig:{destination, interval, sampling, timeout, httpMethod}}
// 依据 xray 官方文档：https://xtls.github.io/config/observatory.html
type ChainObservatory struct {
	SubjectSelector   []string         `json:"subjectSelector"`
	ProbeURL          string           `json:"probeUrl,omitempty"`
	ProbeInterval     string           `json:"probeInterval,omitempty"`
	EnableConcurrency bool             `json:"enableConcurrency,omitempty"`
	PingConfig        *ChainPingConfig `json:"pingConfig,omitempty"`
}

// ChainPingConfig 是 burstObservatory 的嵌套 pingConfig 结构。
// xray burstObservatory 使用 pingConfig 包含探测参数，而非扁平字段。
type ChainPingConfig struct {
	Destination  string `json:"destination,omitempty"`
	Connectivity string `json:"connectivity,omitempty"`
	Interval     string `json:"interval,omitempty"`
	Sampling     int    `json:"sampling,omitempty"`
	Timeout      string `json:"timeout,omitempty"`
	HttpMethod   string `json:"httpMethod,omitempty"`
}

// GenerateChainEntryOutbounds generates outbound configs for a chain_entry node.
// Each exit in the chain_config becomes an outbound pointing to the chain_exit server.
func (g *ChainConfigGenerator) GenerateChainEntryOutbounds(ctx context.Context, server *entity.NodeServer) ([]ChainEntryOutbound, []ChainRoutingRule, error) {
	if server.NodeType != "chain_entry" {
		return nil, nil, fmt.Errorf("server %d is not a chain_entry", server.ID)
	}

	cfg, err := ParseChainConfig(server.ChainConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("parse chain_config: %w", err)
	}
	if cfg == nil || len(cfg.Exits) == 0 {
		return nil, nil, fmt.Errorf("chain_entry %d has no exits configured", server.ID)
	}

	var outbounds []ChainEntryOutbound
	var routingRules []ChainRoutingRule

	for _, exit := range cfg.Exits {
		exitServer, err := g.serverRepo.GetByID(ctx, exit.ExitServerID)
		if err != nil {
			log.Printf("[ChainGen] chain_entry %d: exit server %d not found: %v", server.ID, exit.ExitServerID, err)
			continue
		}

		// Build outbound settings based on protocol
		outbound := ChainEntryOutbound{
			Tag:      exit.OutboundTag,
			Protocol: exit.Protocol,
			Settings: buildOutboundSettings(exit, exitServer),
			StreamSettings: buildStreamSettings(exit),
		}
		outbounds = append(outbounds, outbound)

		// Routing rule: user inbound traffic → this exit outbound
		routingRules = append(routingRules, ChainRoutingRule{
			Type:        "field",
			OutboundTag: exit.OutboundTag,
		})
	}

	return outbounds, routingRules, nil
}

// MergeChainOutbounds merges chain proxy outbounds into an existing config.
// This is called when generating the full config for a chain_entry node.
func MergeChainOutbounds(existingOutbounds []interface{}, chainOutbounds []ChainEntryOutbound) []interface{} {
	// Remove any existing outbounds with the same tags (idempotent merge)
	existingTags := make(map[string]bool)
	filtered := make([]interface{}, 0, len(existingOutbounds))
	for _, ob := range existingOutbounds {
		if obMap, ok := ob.(map[string]interface{}); ok {
			if tag, ok := obMap["tag"].(string); ok {
				existingTags[tag] = true
			}
		}
		filtered = append(filtered, ob)
	}

	for _, cob := range chainOutbounds {
		if existingTags[cob.Tag] {
			continue // Don't duplicate
		}
		// Convert to map[string]interface{}
		data, _ := json.Marshal(cob)
		var m map[string]interface{}
		json.Unmarshal(data, &m)
		filtered = append(filtered, m)
	}

	return filtered
}

// MergeChainRoutingRules merges chain proxy routing rules into existing routing rules.
func MergeChainRoutingRules(existingRules []interface{}, chainRules []ChainRoutingRule) []interface{} {
	result := make([]interface{}, 0, len(existingRules)+len(chainRules))

	// Chain rules go first (higher priority)
	for _, cr := range chainRules {
		data, _ := json.Marshal(cr)
		var m map[string]interface{}
		json.Unmarshal(data, &m)
		result = append(result, m)
	}

	result = append(result, existingRules...)
	return result
}

// buildOutboundSettings builds the settings object for a chain exit outbound.
func buildOutboundSettings(exit ChainExitSpec, exitServer *entity.NodeServer) map[string]interface{} {
	switch exit.Protocol {
	case "vless", "vmess":
		return map[string]interface{}{
			"vnext": []map[string]interface{}{
				{
					"address": exitServer.Hostname,
					"port":    getExitPort(exitServer),
					"users": []map[string]interface{}{
						{
							"id":         "chain-relay-uuid",
							"email":      "chain-relay",
							"encryption": "none",
						},
					},
				},
			},
		}
	case "trojan":
		return map[string]interface{}{
			"servers": []map[string]interface{}{
				{
					"address":  exitServer.Hostname,
					"port":     getExitPort(exitServer),
					"password": "chain-relay-uuid",
				},
			},
		}
	case "shadowsocks":
		return map[string]interface{}{
			"servers": []map[string]interface{}{
				{
					"address":  exitServer.Hostname,
					"port":     getExitPort(exitServer),
					"method":   "chacha20-ietf-poly1305",
					"password": "chain-relay-uuid",
				},
			},
		}
	default:
		return map[string]interface{}{
			"vnext": []map[string]interface{}{
				{
					"address": exitServer.Hostname,
					"port":    getExitPort(exitServer),
					"users": []map[string]interface{}{
						{"id": "chain-relay-uuid", "email": "chain-relay"},
					},
				},
			},
		}
	}
}

// buildStreamSettings builds the stream settings for a chain exit outbound.
func buildStreamSettings(exit ChainExitSpec) map[string]interface{} {
	ss := map[string]interface{}{
		"network": exit.Transport,
	}
	if exit.Security != "" {
		ss["security"] = exit.Security
	}
	return ss
}

// getExitPort returns the relay port from the exit server's chain_config,
// or a default of 443 if not configured.
func getExitPort(exitServer *entity.NodeServer) int {
	cfg, err := ParseChainConfig(exitServer.ChainConfig)
	if err != nil || cfg == nil {
		return 443
	}
	if cfg.RelayPort > 0 {
		return cfg.RelayPort
	}
	return 443
}
