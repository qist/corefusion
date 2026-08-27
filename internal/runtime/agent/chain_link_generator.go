package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// ChainLinkGenerator generates core configuration snippets for chain proxy
// nodes based on chain_links records. It replaces the legacy ChainConfigGenerator
// which relied on node_type=chain_entry/chain_exit and the chain_config JSON column.
//
// The new model decouples chain proxy roles from node types:
// - Any server with chain_links records is an "entry" (its outbound goes to the exit).
// - Any server with chain_exit_configs is an "exit" (it has a relay-in inbound).
// - A server can be both entry and exit simultaneously.
type ChainLinkGenerator struct {
	linkRepo     *repository.ChainLinkRepository
	exitCfgRepo  *repository.ChainExitConfigRepository
	serverRepo   *repository.NodeServerRepository
	extNodeRepo  *repository.ExternalNodeRepository
}

// NewChainLinkGenerator creates a new ChainLinkGenerator.
func NewChainLinkGenerator(
	linkRepo *repository.ChainLinkRepository,
	exitCfgRepo *repository.ChainExitConfigRepository,
	serverRepo *repository.NodeServerRepository,
	extNodeRepo *repository.ExternalNodeRepository,
) *ChainLinkGenerator {
	return &ChainLinkGenerator{
		linkRepo:    linkRepo,
		exitCfgRepo: exitCfgRepo,
		serverRepo:  serverRepo,
		extNodeRepo: extNodeRepo,
	}
}

// generateEntryOutboundsTyped generates outbound configs for an entry node
// based on its chain_links. Each active link becomes an xray outbound
// pointing to the exit target (cluster server or external node).
// Returns typed results; use GenerateEntryOutbounds for the interface method.
func (g *ChainLinkGenerator) generateEntryOutboundsTyped(ctx context.Context, serverID int64) ([]ChainEntryOutbound, []ChainRoutingRule, error) {
	links, err := g.linkRepo.ListByEntry(ctx, serverID, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("list chain links: %w", err)
	}
	if len(links) == 0 {
		return nil, nil, nil
	}

	var outbounds []ChainEntryOutbound
	var routingRules []ChainRoutingRule

	for _, link := range links {
		if !link.Enabled {
			continue
		}

		var outbound ChainEntryOutbound
		var err error

		if link.IsExitExternal() {
			outbound, err = g.buildExternalOutbound(ctx, link)
		} else if link.IsExitServer() {
			outbound, err = g.buildServerOutbound(ctx, link)
		} else {
			log.Printf("[ChainLink] link %d has no valid exit target (server_id=0, external_id=0)", link.ID)
			continue
		}

		if err != nil {
			log.Printf("[ChainLink] link %d: %v", link.ID, err)
			continue
		}

		outbounds = append(outbounds, outbound)
		routingRules = append(routingRules, ChainRoutingRule{
			Type:        "field",
			OutboundTag: outbound.Tag,
		})
	}

	return outbounds, routingRules, nil
}

// GenerateExitInbound generates the relay inbound for an exit server.
// Returns nil if the server has no chain_exit_configs or it's disabled.
func (g *ChainLinkGenerator) GenerateExitInbound(ctx context.Context, serverID int64) (map[string]interface{}, error) {
	exitCfg, err := g.exitCfgRepo.GetByServerID(ctx, serverID)
	if err != nil || exitCfg == nil {
		return nil, nil
	}
	if !exitCfg.Enabled || exitCfg.RelayPort == 0 {
		return nil, nil
	}

	inbound := map[string]interface{}{
		"tag":      "relay-in",
		"listen":   "0.0.0.0",
		"port":     exitCfg.RelayPort,
		"protocol": exitCfg.RelayProtocol,
		"settings": map[string]interface{}{
			"clients": []map[string]interface{}{
				{
					"id":    exitCfg.RelayUUID,
					"email": "chain-relay",
				},
			},
		},
	}

	// If relay_only, the server should not have user inbounds.
	// The caller (PrepareFullConfig) is responsible for filtering them out.
	return inbound, nil
}

// IsRelayOnlyServer returns true if the server is configured as a pure exit
// (relay_only=true in chain_exit_configs).
func (g *ChainLinkGenerator) IsRelayOnlyServer(ctx context.Context, serverID int64) bool {
	exitCfg, err := g.exitCfgRepo.GetByServerID(ctx, serverID)
	if err != nil || exitCfg == nil {
		return false
	}
	return exitCfg.Enabled && exitCfg.RelayOnly
}

// HasChainLinks returns true if the server has any active chain_links (is an entry).
func (g *ChainLinkGenerator) HasChainLinks(ctx context.Context, serverID int64) bool {
	links, err := g.linkRepo.ListByEntry(ctx, serverID, 0)
	return err == nil && len(links) > 0
}

// buildServerOutbound builds an outbound pointing to a cluster exit server.
func (g *ChainLinkGenerator) buildServerOutbound(ctx context.Context, link entity.ChainLink) (ChainEntryOutbound, error) {
	exitServer, err := g.serverRepo.GetByID(ctx, link.ExitServerID)
	if err != nil {
		return ChainEntryOutbound{}, fmt.Errorf("exit server %d not found: %w", link.ExitServerID, err)
	}

	protocol := "vless"
	transport := "tcp"
	security := "none"
	relayUUID := "chain-relay-uuid"
	port := 443

	exitCfg, err := g.exitCfgRepo.GetByServerID(ctx, link.ExitServerID)
	if err == nil && exitCfg != nil {
		protocol = exitCfg.RelayProtocol
		transport = exitCfg.RelayTransport
		security = exitCfg.RelaySecurity
		relayUUID = exitCfg.RelayUUID
		if exitCfg.RelayPort > 0 {
			port = exitCfg.RelayPort
		}
	}

	tag := link.ExitTag
	if tag == "" {
		tag = fmt.Sprintf("to-exit-%d", link.ExitServerID)
	}

	return ChainEntryOutbound{
		Tag:      tag,
		Protocol: protocol,
		Settings: buildLinkOutboundSettings(protocol, exitServer.Hostname, port, relayUUID),
		StreamSettings: map[string]interface{}{
			"network":  transport,
			"security": security,
		},
	}, nil
}

// buildExternalOutbound builds an outbound pointing to an external node.
func (g *ChainLinkGenerator) buildExternalOutbound(ctx context.Context, link entity.ChainLink) (ChainEntryOutbound, error) {
	extNode, err := g.extNodeRepo.GetByID(ctx, link.ExitExternalID)
	if err != nil {
		return ChainEntryOutbound{}, fmt.Errorf("external node %d not found: %w", link.ExitExternalID, err)
	}
	if !extNode.Enabled {
		return ChainEntryOutbound{}, fmt.Errorf("external node %d is disabled", link.ExitExternalID)
	}

	var pm map[string]interface{}
	if json.Unmarshal([]byte(extNode.ProxyJSON), &pm) != nil {
		return ChainEntryOutbound{}, fmt.Errorf("failed to parse proxy_json for external node %d", link.ExitExternalID)
	}

	tag := link.ExitTag
	if tag == "" {
		tag = fmt.Sprintf("to-ext-%d", link.ExitExternalID)
	}

	protocol := extNode.Protocol
	if protocol == "" {
		if v, ok := pm["type"].(string); ok {
			protocol = v
		}
	}

	host := extNode.Host
	if host == "" {
		if v, ok := pm["server"].(string); ok {
			host = v
		}
	}

	port := extNode.Port
	if port == 0 {
		if v, ok := pm["port"].(float64); ok {
			port = int(v)
		}
	}

	// For external nodes, the ProxyJSON already contains full credentials (uuid, password, etc.)
	// We extract the user info from the proxy map instead of using a relay UUID.
	users := extractUsersFromProxyMap(pm, protocol)

	return ChainEntryOutbound{
		Tag:      tag,
		Protocol: protocol,
		Settings: buildExternalOutboundSettings(protocol, host, port, users),
		StreamSettings: buildStreamFromProxyMap(pm),
	}, nil
}

// buildLinkOutboundSettings builds xray outbound settings for a cluster exit.
func buildLinkOutboundSettings(protocol, address string, port int, relayUUID string) map[string]interface{} {
	switch protocol {
	case "vless", "vmess":
		return map[string]interface{}{
			"vnext": []map[string]interface{}{
				{
					"address": address,
					"port":    port,
					"users": []map[string]interface{}{
						{
							"id":         relayUUID,
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
					"address":  address,
					"port":     port,
					"password": relayUUID,
				},
			},
		}
	case "shadowsocks":
		return map[string]interface{}{
			"servers": []map[string]interface{}{
				{
					"address":  address,
					"port":     port,
					"method":   "chacha20-ietf-poly1305",
					"password": relayUUID,
				},
			},
		}
	default:
		return map[string]interface{}{
			"vnext": []map[string]interface{}{
				{
					"address": address,
					"port":    port,
					"users": []map[string]interface{}{
						{"id": relayUUID, "email": "chain-relay"},
					},
				},
			},
		}
	}
}

// buildExternalOutboundSettings builds xray outbound settings from external node data.
func buildExternalOutboundSettings(protocol, address string, port int, users []map[string]interface{}) map[string]interface{} {
	switch protocol {
	case "vless", "vmess":
		if len(users) == 0 {
			users = []map[string]interface{}{
				{"id": "", "email": "chain-relay", "encryption": "none"},
			}
		}
		return map[string]interface{}{
			"vnext": []map[string]interface{}{
				{
					"address": address,
					"port":    port,
					"users":   users,
				},
			},
		}
	case "trojan":
		password := ""
		if len(users) > 0 {
			if p, ok := users[0]["password"].(string); ok {
				password = p
			}
		}
		return map[string]interface{}{
			"servers": []map[string]interface{}{
				{
					"address":  address,
					"port":     port,
					"password": password,
				},
			},
		}
	case "shadowsocks":
		method := "chacha20-ietf-poly1305"
		password := ""
		if len(users) > 0 {
			if m, ok := users[0]["method"].(string); ok && m != "" {
				method = m
			}
			if p, ok := users[0]["password"].(string); ok {
				password = p
			}
		}
		return map[string]interface{}{
			"servers": []map[string]interface{}{
				{
					"address":  address,
					"port":     port,
					"method":   method,
					"password": password,
				},
			},
		}
	default:
		return map[string]interface{}{
			"vnext": []map[string]interface{}{
				{
					"address": address,
					"port":    port,
					"users":   users,
				},
			},
		}
	}
}

// extractUsersFromProxyMap extracts user credentials from a mihomo proxy map.
func extractUsersFromProxyMap(pm map[string]interface{}, protocol string) []map[string]interface{} {
	switch protocol {
	case "vless":
		uuid, _ := pm["uuid"].(string)
		flow, _ := pm["flow"].(string)
		user := map[string]interface{}{
			"id":         uuid,
			"email":      "chain-relay",
			"encryption": "none",
		}
		if flow != "" {
			user["flow"] = flow
		}
		return []map[string]interface{}{user}
	case "vmess":
		uuid, _ := pm["uuid"].(string)
		alterId := 0
		if a, ok := pm["alterId"].(float64); ok {
			alterId = int(a)
		}
		return []map[string]interface{}{
			{
				"id":       uuid,
				"email":    "chain-relay",
				"alterId":  alterId,
				"security": "auto",
			},
		}
	case "trojan":
		password, _ := pm["password"].(string)
		return []map[string]interface{}{
			{
				"password": password,
			},
		}
	case "shadowsocks":
		password, _ := pm["password"].(string)
		method, _ := pm["cipher"].(string)
		if method == "" {
			method = "chacha20-ietf-poly1305"
		}
		return []map[string]interface{}{
			{
				"method":   method,
				"password": password,
			},
		}
	default:
		return nil
	}
}

// buildStreamFromProxyMap builds xray streamSettings from a mihomo proxy map.
func buildStreamFromProxyMap(pm map[string]interface{}) map[string]interface{} {
	transport, _ := pm["network"].(string)
	if transport == "" {
		transport = "tcp"
	}
	ss := map[string]interface{}{
		"network": transport,
	}

	tls, _ := pm["tls"].(bool)
	if tls {
		ss["security"] = "tls"
		if sni, ok := pm["servername"].(string); ok && sni != "" {
			ss["serverName"] = sni
		} else if sni, ok := pm["sni"].(string); ok && sni != "" {
			ss["serverName"] = sni
		}
		if alpn, ok := pm["alpn"].([]interface{}); ok && len(alpn) > 0 {
			ss["alpn"] = alpn
		}
		if fp, ok := pm["client-fingerprint"].(string); ok && fp != "" {
			ss["fingerprint"] = fp
		}
	}

	// WS settings
	if transport == "ws" {
		wsOpts := map[string]interface{}{}
		if path, ok := pm["ws-opts"].(map[string]interface{}); ok {
			if p, ok := path["path"].(string); ok {
				wsOpts["path"] = p
			}
			if h, ok := path["headers"].(map[string]interface{}); ok {
				wsOpts["headers"] = h
			}
		}
		ss["wsSettings"] = wsOpts
	}

	// gRPC settings
	if transport == "grpc" {
		grpcOpts := map[string]interface{}{}
		if serviceName, ok := pm["grpc-opts"].(map[string]interface{}); ok {
			if sn, ok := serviceName["grpc-service-name"].(string); ok {
				grpcOpts["serviceName"] = sn
			}
		}
		ss["grpcSettings"] = grpcOpts
	}

	return ss
}

// MergeChainLinkOutbounds merges chain link outbounds into existing outbounds.
// Removes any existing outbounds with the same tags first (idempotent).
func MergeChainLinkOutbounds(existingOutbounds []interface{}, chainOutbounds []ChainEntryOutbound) []interface{} {
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
			continue
		}
		data, _ := json.Marshal(cob)
		var m map[string]interface{}
		json.Unmarshal(data, &m)
		filtered = append(filtered, m)
	}

	return filtered
}

// MergeChainLinkRoutingRules merges chain link routing rules into existing rules.
// Chain rules go first (higher priority).
func MergeChainLinkRoutingRules(existingRules []interface{}, chainRules []ChainRoutingRule) []interface{} {
	result := make([]interface{}, 0, len(existingRules)+len(chainRules))
	for _, cr := range chainRules {
		data, _ := json.Marshal(cr)
		var m map[string]interface{}
		json.Unmarshal(data, &m)
		result = append(result, m)
	}
	result = append(result, existingRules...)
	return result
}

// ── ChainLinkProvider interface adapter ──
// The following methods satisfy the coreconfig.ChainLinkProvider interface.
// They wrap the typed methods above, converting to []interface{}.

// GenerateEntryOutbounds implements coreconfig.ChainLinkProvider.
// It wraps generateEntryOutboundsTyped, converting the typed results to []interface{}.
func (g *ChainLinkGenerator) GenerateEntryOutbounds(ctx context.Context, serverID int64) ([]interface{}, []interface{}, error) {
	outbounds, rules, err := g.generateEntryOutboundsTyped(ctx, serverID)
	if err != nil || (len(outbounds) == 0 && len(rules) == 0) {
		return nil, nil, err
	}
	obs := make([]interface{}, 0, len(outbounds))
	for _, ob := range outbounds {
		data, _ := json.Marshal(ob)
		var m map[string]interface{}
		json.Unmarshal(data, &m)
		obs = append(obs, m)
	}
	rls := make([]interface{}, 0, len(rules))
	for _, r := range rules {
		data, _ := json.Marshal(r)
		var m map[string]interface{}
		json.Unmarshal(data, &m)
		rls = append(rls, m)
	}
	return obs, rls, nil
}
