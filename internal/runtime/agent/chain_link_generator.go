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

// generateEntryOutboundsTyped generates chain proxy outbounds and routing rules
// for an entry server based on its chain_links. Each active link becomes an xray outbound
// pointing to the exit target (cluster server or external node).
// Returns typed results; use GenerateEntryOutbounds / GenerateEntryBalancers for the interface methods.
//
// Multi-exit support (see design doc §6.5 / §6.6): links are grouped by
// EntryInboundID. Within a group:
//   - a single exit  → one field rule (inboundTag → chain-* outboundTag)
//   - multiple exits → a balancer (prefix selector "chain-<in>-") + observatory,
//     and the field rule points to balancerTag instead of a single outbound.
func (g *ChainLinkGenerator) generateEntryOutboundsTyped(ctx context.Context, serverID int64) ([]ChainEntryOutbound, []ChainRoutingRule, []ChainBalancer, *ChainObservatory, error) {
	links, err := g.linkRepo.ListByEntry(ctx, serverID, 0)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("list chain links: %w", err)
	}
	if len(links) == 0 {
		return nil, nil, nil, nil, nil
	}

	// 入口节点（用于决策点 ①：出口地址内网/公网智能选择，见设计文档 §6.1.1）
	entry, err := g.serverRepo.GetByID(ctx, serverID)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("entry server %d not found: %w", serverID, err)
	}

	var outbounds []ChainEntryOutbound
	var routingRules []ChainRoutingRule
	var balancers []ChainBalancer
	var observatory *ChainObservatory

	// 按 EntryInboundID 分组（同一入口可关联多个出口，见 §6.5）
	groups := make(map[int64][]entity.ChainLink)
	order := make([]int64, 0)
	for _, link := range links {
		if !link.Enabled {
			continue
		}
		if _, ok := groups[link.EntryInboundID]; !ok {
			order = append(order, link.EntryInboundID)
		}
		groups[link.EntryInboundID] = append(groups[link.EntryInboundID], link)
	}

	for _, inID := range order {
		groupLinks := groups[inID]
		inboundTag := fmt.Sprintf("in-%d", inID)

		// 先为每个出口生成 outbound（无论单/多出口，全部产出，balancer 靠 selector 前缀圈定）
		for _, link := range groupLinks {
			var outbound ChainEntryOutbound
			var berr error
			if link.IsExitExternal() {
				outbound, berr = g.buildExternalOutbound(ctx, link)
			} else if link.IsExitServer() {
				outbound, berr = g.buildServerOutbound(ctx, link, entry)
			} else {
				log.Printf("[ChainLink] link %d has no valid exit target (server_id=0, external_id=0)", link.ID)
				continue
			}
			if berr != nil {
				log.Printf("[ChainLink] link %d: %v", link.ID, berr)
				continue
			}
			outbounds = append(outbounds, outbound)
		}

		if len(groupLinks) == 1 {
			// 单出口：直接 field 规则 inboundTag → chain-* outboundTag（见 §6.4）
			link := groupLinks[0]
			tag := link.ExitTag
			if tag == "" {
				tag = g.exitTagFor(link)
			}
			routingRules = append(routingRules, ChainRoutingRule{
				Type:        "field",
				InboundTag:  []string{inboundTag},
				OutboundTag: tag,
				RuleTag:     fmt.Sprintf("exit-%d-single", inID),
			})
			continue
		}

		// 多出口：负载均衡模式（见 §6.6）—— 多选自动判定为负载均衡
		balTag := fmt.Sprintf("bal-chain-%d", inID)
		prefix := fmt.Sprintf("chain-%d-", inID) // selector 前缀，自动圈定本入口全部出口
		balancers = append(balancers, ChainBalancer{
			Tag:         balTag,
			Selector:    []string{prefix},
			FallbackTag: "direct",
			Strategy:    map[string]interface{}{"type": "leastPing"},
		})
		routingRules = append(routingRules, ChainRoutingRule{
			Type:        "field",
			InboundTag:  []string{inboundTag},
			BalancerTag: balTag,
			RuleTag:     fmt.Sprintf("exit-%d-lb", inID),
		})
		// 仅当该入口存在负载均衡时才注入 observatory，subjectSelector 用前缀隔离（§6.6.5）
		if observatory == nil {
			observatory = &ChainObservatory{
				SubjectSelector:   []string{prefix},
				ProbeURL:          "https://www.google.com/generate_204",
				ProbeInterval:     "10s",
				EnableConcurrency: true,
			}
		} else {
			// 多个负载均衡入口时，把所有前缀合并进同一个 observatory 的 subjectSelector
			observatory.SubjectSelector = append(observatory.SubjectSelector, prefix)
		}
	}

	return outbounds, routingRules, balancers, observatory, nil
}

// exitTagFor returns the canonical chain-* outbound tag for a link (§6.1).
func (g *ChainLinkGenerator) exitTagFor(link entity.ChainLink) string {
	if link.IsExitExternal() {
		return fmt.Sprintf("chain-%d-ext%d", link.EntryInboundID, link.ExitExternalID)
	}
	return fmt.Sprintf("chain-%d-%d", link.EntryInboundID, link.ExitServerID)
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
// entry is the entry (source) server, used to pick the optimal exit address
// (internal IP when both nodes share a private network, otherwise public IP).
func (g *ChainLinkGenerator) buildServerOutbound(ctx context.Context, link entity.ChainLink, entry *entity.NodeServer) (ChainEntryOutbound, error) {
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

	// tag 统一以 chain- 开头：chain-<入口inboundID>-<出口serverID>（见设计文档 §6.1 / §6.1.1）
	tag := link.ExitTag
	if tag == "" {
		tag = fmt.Sprintf("chain-%d-%d", link.EntryInboundID, link.ExitServerID)
	}

	// 决策点 ①：出口地址内网/公网智能选择（见设计文档 §6.1.1 resolveExitAddress）
	address := resolveExitAddress(entry, exitServer)

	return ChainEntryOutbound{
		Tag:      tag,
		Protocol: protocol,
		Settings: buildLinkOutboundSettings(protocol, address, port, relayUUID),
		StreamSettings: map[string]interface{}{
			"network":  transport,
			"security": security,
		},
	}, nil
}

// resolveExitAddress selects the best address for an exit server:
//   - if both entry and exit are in the same private network (same region and
//     the exit exposes an InternalIP), use InternalIP to save public bandwidth;
//   - otherwise use PublicIP, falling back to Hostname.
// External nodes carry their own address (from ProxyJSON) and do not use this.
func resolveExitAddress(entry, exit *entity.NodeServer) string {
	if exit.InternalIP != "" && samePrivateNetwork(entry, exit) {
		return exit.InternalIP
	}
	if exit.PublicIP != "" {
		return exit.PublicIP
	}
	return exit.Hostname
}

// samePrivateNetwork reports whether two servers are co-located such that the
// entry can reach the exit via its internal IP. We approximate this by matching
// the region field; extend with subnet/VPC awareness if needed.
func samePrivateNetwork(entry, exit *entity.NodeServer) bool {
	return entry.Region != "" && entry.Region == exit.Region
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

	// 外部节点出口 tag 同样以 chain- 开头，保持全局一致：chain-<入口inboundID>-ext<外部ID>（§6.1.1）
	tag := link.ExitTag
	if tag == "" {
		tag = fmt.Sprintf("chain-%d-ext%d", link.EntryInboundID, link.ExitExternalID)
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
// Balancers and observatory (multi-exit load balancing, see design doc §6.6) are
// returned separately via GenerateEntryBalancers.
func (g *ChainLinkGenerator) GenerateEntryOutbounds(ctx context.Context, serverID int64) ([]interface{}, []interface{}, error) {
	outbounds, rules, _, _, err := g.generateEntryOutboundsTyped(ctx, serverID)
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

// GenerateEntryBalancers implements coreconfig.ChainLinkProvider.
// It returns the routing.balancers[] entries and the root observatory config
// for load-balanced (multi-exit) entries. Returns nil slices when no entry uses
// load balancing. See design doc §6.5 / §6.6.
func (g *ChainLinkGenerator) GenerateEntryBalancers(ctx context.Context, serverID int64) (balancers []interface{}, observatory map[string]interface{}) {
	_, _, bals, obs, err := g.generateEntryOutboundsTyped(ctx, serverID)
	if err != nil || len(bals) == 0 {
		return nil, nil
	}
	bal := make([]interface{}, 0, len(bals))
	for _, b := range bals {
		data, _ := json.Marshal(b)
		var m map[string]interface{}
		json.Unmarshal(data, &m)
		bal = append(bal, m)
	}
	if obs != nil {
		data, _ := json.Marshal(obs)
		var m map[string]interface{}
		json.Unmarshal(data, &m)
		observatory = m
	}
	return bal, observatory
}
