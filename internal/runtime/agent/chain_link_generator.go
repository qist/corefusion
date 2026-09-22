package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"

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
	linkRepo    *repository.ChainLinkRepository
	exitCfgRepo *repository.ChainExitConfigRepository
	serverRepo  *repository.NodeServerRepository
	extNodeRepo *repository.ExternalNodeRepository
	// inboundRepo：用于"链式出口 = 出口节点已有的入站"这条路径（读取该入站的真实
	// 协议/端口/传输/安全层/凭据，构造入口侧出站）。由 router 注入，可为 nil（老路径不受影响）。
	inboundRepo *repository.InboundConfigRepository
}

// SetInboundRepo 注入入站配置仓库（链式出口支持"选现有入站"必需）。
func (g *ChainLinkGenerator) SetInboundRepo(repo *repository.InboundConfigRepository) {
	g.inboundRepo = repo
}

// —— 链式出口的**协议白名单（按入口核心分开）** ——
//
// 实测依据（`xray run -test` / `sing-box check`）：
//   - xray **没有** hysteria2 出站 → `unknown config id: hysteria2` ✗
//   - sing-box 支持 hysteria2 出站 ✓，但**必须带 tls**（缺了 `FATAL TLS required`），
//     配 reality 时还必须带 uTLS（缺了 `FATAL uTLS is required by reality client`）
//
// 所以"出口入站能用什么协议"由**入口核心**决定：入口是 xray 就不能选 hy2 出口。
// 其余协议（tuic / anytls / snell / naive / socks / http / wireguard …）两边都还没实现，一律不允许
// （选了会生成核心不认的出站 → 节点起不来）。
var chainCoreProtocols = map[string][]string{
	"xray":    {"vless", "vmess", "trojan", "shadowsocks"},
	"singbox": {"vless", "vmess", "trojan", "shadowsocks", "hysteria2"},
}

func normalizeChainCore(coreType string) string {
	c := strings.ToLower(strings.TrimSpace(coreType))
	if c == "sing-box" || c == "sing_box" {
		c = "singbox"
	}
	if c == "" {
		c = "xray"
	}
	return c
}

// ChainSupportedProtocols 返回**指定入口核心**能作为链式出口的协议列表。
func ChainSupportedProtocols(coreType string) []string {
	list, ok := chainCoreProtocols[normalizeChainCore(coreType)]
	if !ok {
		return []string{}
	}
	return append([]string(nil), list...)
}

// ChainSupportedProtocol 判断某协议能否作为**该入口核心**的链式出口。
func ChainSupportedProtocol(coreType, protocol string) bool {
	for _, p := range ChainSupportedProtocols(coreType) {
		if strings.EqualFold(p, protocol) {
			return true
		}
	}
	return false
}

// ChainAllSupportedProtocols 返回所有核心的并集（"出口侧能提供哪些协议"，供前端提示）。
func ChainAllSupportedProtocols() []string {
	seen := map[string]bool{}
	var all []string
	for _, core := range []string{"xray", "singbox"} {
		for _, p := range chainCoreProtocols[core] {
			if !seen[p] {
				seen[p] = true
				all = append(all, p)
			}
		}
	}
	return all
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
//   - multiple exits with same priority → loadbalance (balancer + observatory)
//   - multiple exits with different priorities → failover (多条独立 field 规则, 按 priority 排序)
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

		// 判定多出口策略：failover vs loadbalance（见 §6.5）
		// 启发式：如果所有 link 的 Priority 相同 → loadbalance；否则 → failover
		strategy := "loadbalance"
		firstPriority := groupLinks[0].Priority
		for _, l := range groupLinks[1:] {
			if l.Priority != firstPriority {
				strategy = "failover"
				break
			}
		}

		if strategy == "failover" {
			// failover 模式（§6.5.3）：按 Priority 升序生成多条独立 field 规则
			// xray 首规则命中即转发，主出口排最前；主 down 时规则不生效，自动落下一优先级
			sortedLinks := make([]entity.ChainLink, len(groupLinks))
			copy(sortedLinks, groupLinks)
			sort.SliceStable(sortedLinks, func(i, j int) bool {
				if sortedLinks[i].Priority != sortedLinks[j].Priority {
					return sortedLinks[i].Priority < sortedLinks[j].Priority
				}
				return sortedLinks[i].SortOrder < sortedLinks[j].SortOrder
			})
			for _, link := range sortedLinks {
				tag := link.ExitTag
				if tag == "" {
					tag = g.exitTagFor(link)
				}
				routingRules = append(routingRules, ChainRoutingRule{
					Type:        "field",
					InboundTag:  []string{inboundTag},
					OutboundTag: tag,
					RuleTag:     fmt.Sprintf("exit-%d-pri%d", inID, link.Priority),
				})
			}
			continue
		}

		// loadbalance 模式（§6.6）—— 等权多出口，balancer + observatory
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
		// 使用 burstObservatory 配置参数，适合入口节点隐蔽性（§6.6.5）
		// 依据 xray 官方文档：burstObservatory 使用嵌套 pingConfig 结构
		if observatory == nil {
			observatory = &ChainObservatory{
				SubjectSelector: []string{prefix},
				PingConfig: &ChainPingConfig{
					// burstObservatory 参数（§6.6.5）：interval=1m, sampling=10
					// 探测周期 10 分钟，特征弱，适合入口节点
					Destination: "https://connectivitycheck.gstatic.com/generate_204",
					Interval:    "1m",
					Sampling:    10,
					Timeout:     "5s",
					HttpMethod:  "HEAD",
				},
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

	client := map[string]interface{}{
		"id":    exitCfg.RelayUUID,
		"email": "chain-relay",
	}
	// xray vless inbound requires "decryption":"none" at both the settings and
	// each client level, otherwise xray refuses to start.
	if exitCfg.RelayProtocol == "vless" {
		client["decryption"] = "none"
	}

	settings := map[string]interface{}{
		"clients": []map[string]interface{}{client},
	}
	if exitCfg.RelayProtocol == "vless" {
		settings["decryption"] = "none"
	}

	inbound := map[string]interface{}{
		"tag":      "relay-in",
		"listen":   "0.0.0.0",
		"port":     exitCfg.RelayPort,
		"protocol": exitCfg.RelayProtocol,
		"settings": settings,
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

// buildOutboundFromExitInbound 用**出口节点上已有的入站**构造入口侧的链式出站。
//
// 与 buildServerOutbound 的旧路径（读 chain_exit_configs 自动建的 relay-in）不同，
// 这条路径**不改动出口节点任何配置**：端口/协议/传输/安全层/凭据全部从该入站自身读取。
// 支持面板自建的两种入站形状：
//
//	xray    : {protocol, port, settings{clients[], method}, streamSettings{network, security, tlsSettings, realitySettings}}
//	sing-box: {type, listen_port, users[]/{method,password}, transport{type}, tls{enabled, server_name, reality{...}}}
//
// 返回 ok=false 表示该入站的协议不在链式支持名单里（调用方应报错提示，而不是生成坏出站）。
func (g *ChainLinkGenerator) buildOutboundFromExitInbound(ctx context.Context, link entity.ChainLink, entry, exitServer *entity.NodeServer) (ChainEntryOutbound, bool, error) {
	if g.inboundRepo == nil {
		return ChainEntryOutbound{}, false, fmt.Errorf("inbound repository not wired")
	}
	ib, err := g.inboundRepo.GetByID(ctx, link.ExitInboundID)
	if err != nil || ib == nil {
		return ChainEntryOutbound{}, false, fmt.Errorf("exit inbound %d not found: %v", link.ExitInboundID, err)
	}
	if ib.ServerID != exitServer.ID {
		return ChainEntryOutbound{}, false, fmt.Errorf("exit inbound %d does not belong to server %d", ib.ID, exitServer.ID)
	}

	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(ib.ConfigJSON), &cfg); err != nil {
		return ChainEntryOutbound{}, false, fmt.Errorf("parse exit inbound config: %w", err)
	}

	protocol := strings.ToLower(strings.TrimSpace(ib.Protocol))
	if protocol == "" {
		protocol = firstString(cfg, "protocol", "type")
	}
	entryCore := g.entryCoreType(ctx, link.EntryInboundID)
	// 按**入口核心**判断（hysteria2 只有 sing-box 入口能用 —— xray 没有 hy2 出站，实测
	// `xray run -test` 报 `unknown config id: hysteria2`）
	if !ChainSupportedProtocol(entryCore, protocol) {
		return ChainEntryOutbound{}, false, nil
	}

	// 端口：xray 用 port，sing-box 用 listen_port；再兜底 DB 列
	port := int(firstNumber(cfg, "port", "listen_port"))
	if port == 0 {
		port = ib.Port
	}
	if port == 0 {
		return ChainEntryOutbound{}, false, fmt.Errorf("exit inbound %d has no usable port", ib.ID)
	}

	// 传输 / 安全层（两种形状）
	transport := "tcp"
	security := "none"
	if ss, ok := cfg["streamSettings"].(map[string]interface{}); ok { // xray
		if v := firstString(ss, "network"); v != "" {
			transport = v
		}
		if v := firstString(ss, "security"); v != "" {
			security = v
		}
	} else if tr, ok := cfg["transport"].(map[string]interface{}); ok { // sing-box
		if v := firstString(tr, "type"); v != "" {
			transport = v
		}
	}
	// ⚠ sing-box 的 TLS/REALITY 判在**入站顶层 `tls`**，与有没有 `transport` 无关。
	// 这段原来被塞在上面那个 transport 分支里 → **hy2 这类没有 transport 的协议整段被跳过** ✗
	// → 生成的出站不带 tls → `FATAL TLS required`（起不来）。实测入站 120 复现。
	if tlsMap, ok := cfg["tls"].(map[string]interface{}); ok {
		if rm, ok := tlsMap["reality"].(map[string]interface{}); ok {
			if b, _ := rm["enabled"].(bool); b {
				security = "reality"
			}
		}
		if security == "none" {
			if b, _ := tlsMap["enabled"].(bool); b {
				security = "tls"
			}
		}
	}
	// hysteria2 是"永远 TLS"的协议：即使入站没写 tls（理论不该发生）出站也必须带 tls，
	// 否则核心直接 FATAL。这里是最后一道兜底。
	if security == "none" && strings.EqualFold(protocol, "hysteria2") {
		security = "tls"
	}

	// 凭据：从该入站自己的用户里取第一条（vless/vmess→id，trojan/ss→password）
	cred := ""
	method := ""
	if settings, ok := cfg["settings"].(map[string]interface{}); ok { // xray
		if clients, ok := settings["clients"].([]interface{}); ok && len(clients) > 0 {
			if c0, ok := clients[0].(map[string]interface{}); ok {
				cred = firstString(c0, "id", "password")
			}
		}
		method = firstString(settings, "method")
	}
	if cred == "" { // sing-box
		if users, ok := cfg["users"].([]interface{}); ok && len(users) > 0 {
			if u0, ok := users[0].(map[string]interface{}); ok {
				cred = firstString(u0, "uuid", "password")
			}
		}
		if method == "" {
			method = firstString(cfg, "method")
		}
	}
	if cred == "" {
		return ChainEntryOutbound{}, false, fmt.Errorf("exit inbound %d has no usable credential (add at least one user)", ib.ID)
	}

	// 出站的 streamSettings：传输 + 安全层 + TLS/REALITY 细节（老路径只写 network/security，
	// 用真实入站时必须把 TLS 参数带过去，否则 reality/tls 出口会连不上）
	streamSettings := map[string]interface{}{
		"network":  transport,
		"security": security,
	}
	if security == "tls" || security == "reality" {
		applyExitInboundTLS(streamSettings, cfg, security)
		// alpn 额外带一份到 streamSettings **顶层**（sing-box 转换器从那里读；
		// hy2 常见是 h3，漏了会导致握手协商不上）。
		if tlsMap, ok := cfg["tls"].(map[string]interface{}); ok {
			if alpn, ok := tlsMap["alpn"].([]interface{}); ok && len(alpn) > 0 {
				streamSettings["alpn"] = alpn
			}
		}
		if ts, ok := cfg["tlsSettings"].(map[string]interface{}); ok {
			if alpn, ok := ts["alpn"].([]interface{}); ok && len(alpn) > 0 {
				streamSettings["alpn"] = alpn
			}
		}
	}

	// hysteria2：中间形状直接带上 sing-box 需要的东西（obfs），tls 由 streamSettings 走通用逻辑，
	// 最终由 coreconfig.convertChainOutboundsToSingbox 转成 sing-box 的 hysteria2 出站。
	if strings.EqualFold(protocol, "hysteria2") {
		srv := map[string]interface{}{
			"address":  resolveExitAddress(entry, exitServer),
			"port":     port,
			"password": cred,
		}
		if obfs, ok := cfg["obfs"].(map[string]interface{}); ok && len(obfs) > 0 {
			srv["obfs"] = obfs
		}
		hy2Tag := link.ExitTag
		if hy2Tag == "" {
			hy2Tag = fmt.Sprintf("chain-%d-%d", link.EntryInboundID, link.ExitServerID)
		}
		return ChainEntryOutbound{
			Tag:            hy2Tag,
			Protocol:       protocol,
			Settings:       map[string]interface{}{"servers": []map[string]interface{}{srv}},
			StreamSettings: streamSettings,
		}, true, nil
	}

	tag := link.ExitTag
	if tag == "" {
		tag = fmt.Sprintf("chain-%d-%d", link.EntryInboundID, link.ExitServerID)
	}

	return ChainEntryOutbound{
		Tag:            tag,
		Protocol:       protocol,
		Settings:       buildLinkOutboundSettingsWithCred(protocol, resolveExitAddress(entry, exitServer), port, cred, method),
		StreamSettings: streamSettings,
	}, true, nil
}

// applyExitInboundTLS 把出口入站的 TLS/REALITY 参数搬到入口侧出站（两种形状都支持）。
func applyExitInboundTLS(streamSettings map[string]interface{}, cfg map[string]interface{}, security string) {
	if ss, ok := cfg["streamSettings"].(map[string]interface{}); ok { // xray
		if security == "reality" {
			if r, ok := ss["realitySettings"].(map[string]interface{}); ok {
				rs := map[string]interface{}{}
				if v := firstString(r, "serverName", "serverNames"); v != "" {
					rs["serverName"] = v
				}
				if v := firstString(r, "publicKey", "password"); v != "" {
					rs["publicKey"] = v
				}
				if v := firstString(r, "shortId"); v != "" {
					rs["shortId"] = v
				}
				if v := firstString(r, "fingerprint"); v != "" {
					rs["fingerprint"] = v
				}
				if settings, ok := r["settings"].([]interface{}); ok && len(settings) > 0 {
					if s0, ok := settings[0].(map[string]interface{}); ok {
						if v := firstString(s0, "password"); v != "" {
							rs["publicKey"] = v
						}
						if v := firstString(s0, "fingerprint"); v != "" {
							rs["fingerprint"] = v
						}
					}
				}
				streamSettings["realitySettings"] = rs
				return
			}
		}
		if ts, ok := ss["tlsSettings"].(map[string]interface{}); ok {
			tl := map[string]interface{}{}
			if v := firstString(ts, "serverName"); v != "" {
				tl["serverName"] = v
			}
			if fp := firstString(ts, "fingerprint"); fp != "" {
				tl["fingerprint"] = fp
			}
			streamSettings["tlsSettings"] = tl
			return
		}
	}
	// sing-box 形状
	if tlsMap, ok := cfg["tls"].(map[string]interface{}); ok {
		tl := map[string]interface{}{}
		sni := firstString(tlsMap, "server_name")
		if sni == "" {
			if rm, ok := tlsMap["reality"].(map[string]interface{}); ok {
				if hs, ok := rm["handshake"].(map[string]interface{}); ok {
					sni = firstString(hs, "server")
				}
			}
		}
		if sni != "" {
			tl["serverName"] = sni
		}
		if security == "reality" {
			if rm, ok := tlsMap["reality"].(map[string]interface{}); ok {
				rs := map[string]interface{}{}
				if v := firstString(rm, "public_key"); v != "" {
					rs["publicKey"] = v
				}
				if v := firstString(rm, "short_id"); v != "" {
					rs["shortId"] = v
				}
				if v := firstString(tlsMap, "utls"); v == "" {
					// utls 是对象；这里只关心 fingerprint
				}
				streamSettings["realitySettings"] = rs
			}
		}
		streamSettings["tlsSettings"] = tl
	}
}

// firstString 从 map 里取第一个非空字符串字段。
func firstString(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		switch v := m[k].(type) {
		case string:
			if v != "" {
				return v
			}
		case []interface{}:
			if len(v) > 0 {
				if s, ok := v[0].(string); ok && s != "" {
					return s
				}
			}
		}
	}
	return ""
}

// firstNumber 从 map 里取第一个非零数字字段（JSON 解出来是 float64）。
func firstNumber(m map[string]interface{}, keys ...string) float64 {
	for _, k := range keys {
		switch v := m[k].(type) {
		case float64:
			if v != 0 {
				return v
			}
		case int:
			if v != 0 {
				return float64(v)
			}
		}
	}
	return 0
}

// buildServerOutbound builds an outbound pointing to a cluster exit server.
// entry is the entry (source) server, used to pick the optimal exit address
// (internal IP when both nodes share a private network, otherwise public IP).
func (g *ChainLinkGenerator) buildServerOutbound(ctx context.Context, link entity.ChainLink, entry *entity.NodeServer) (ChainEntryOutbound, error) {
	exitServer, err := g.serverRepo.GetByID(ctx, link.ExitServerID)
	if err != nil {
		return ChainEntryOutbound{}, fmt.Errorf("exit server %d not found: %w", link.ExitServerID, err)
	}

	// 新路径（推荐）：链式出口 = 出口节点上**已有的入站**，链式这边不创建任何入站。
	if link.UseExitInbound() {
		ob, ok, oerr := g.buildOutboundFromExitInbound(ctx, link, entry, exitServer)
		if oerr != nil {
			return ChainEntryOutbound{}, oerr
		}
		if !ok {
			return ChainEntryOutbound{}, fmt.Errorf(
				"exit inbound %d protocol is not chain-capable for this entry core "+
					"(xray supports vless/vmess/trojan/shadowsocks; hysteria2 仅 sing-box 入口可用)",
				link.ExitInboundID)
		}
		return ob, nil
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
//
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
// entryCoreType 取"入口入站自己"的核心类型 —— 它决定入口侧出站必须是 xray 还是 sing-box 形状。
// （entity.NodeServer 上没有 core_type 字段；同一台机器可以同时跑两个核心的入站。）
func (g *ChainLinkGenerator) entryCoreType(ctx context.Context, entryInboundID int64) string {
	if g.inboundRepo == nil || entryInboundID == 0 {
		return "xray"
	}
	if ib, err := g.inboundRepo.GetByID(ctx, entryInboundID); err == nil && ib != nil && ib.CoreType != "" {
		return ib.CoreType
	}
	return "xray"
}

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

	// 按**入口核心**校验协议（与集群出口同一套规则，`ChainSupportedProtocols`）。
	// ⚠ 这里此前**完全没有校验** ✗：外部节点若是 hysteria2 / tuic / anytls 之类，
	//    buildExternalOutboundSettings 会落到 default 分支生成一个**空 settings** 的出站，
	//    结果入口节点核心直接起不来（配置"看着没问题"）。改成显式报错，日志里能直接看到原因。
	if !ChainSupportedProtocol(g.entryCoreType(ctx, link.EntryInboundID), protocol) {
		entryCore := g.entryCoreType(ctx, link.EntryInboundID)
		return ChainEntryOutbound{}, fmt.Errorf(
			"external node %d protocol %q is not chain-capable for entry core %q (supported: %s)",
			link.ExitExternalID, protocol, entryCore,
			strings.Join(ChainSupportedProtocols(entryCore), "/"))
	}

	// For external nodes, the ProxyJSON already contains full credentials (uuid, password, etc.)
	// We extract the user info from the proxy map instead of using a relay UUID.
	users := extractUsersFromProxyMap(pm, protocol)

	settings := buildExternalOutboundSettings(protocol, host, port, users)
	// hysteria2 的 obfs（salamander）在 sing-box 里是**出站顶层**字段，但链式的中间形状放在
	// settings.servers[0].obfs（转换器从那里取）→ 这里从外部节点的 ProxyJSON 注入。
	if (protocol == "hysteria2" || protocol == "hysteria") && settings != nil {
		if obfs, ok := pm["obfs"].(map[string]interface{}); ok && len(obfs) > 0 {
			if servers, ok := settings["servers"].([]map[string]interface{}); ok && len(servers) > 0 {
				servers[0]["obfs"] = obfs
			}
		}
	}

	return ChainEntryOutbound{
		Tag:            tag,
		Protocol:       protocol,
		Settings:       settings,
		StreamSettings: buildStreamFromProxyMap(pm),
	}, nil
}

// buildLinkOutboundSettings builds xray outbound settings for a cluster exit.
// buildLinkOutboundSettings 兼容旧调用：把 relayUUID 当凭据。
func buildLinkOutboundSettings(protocol, address string, port int, relayUUID string) map[string]interface{} {
	return buildLinkOutboundSettingsWithCred(protocol, address, port, relayUUID, "")
}

// buildLinkOutboundSettingsWithCred 与上面相同，但可指定 shadowsocks 的 method
// （用"出口已有入站"时必须用它自己的 method —— 旧实现写死 chacha20-ietf-poly1305，
// 遇到 2022-blake3-* 之类会构造出连不上的出站）。
func buildLinkOutboundSettingsWithCred(protocol, address string, port int, cred, method string) map[string]interface{} {
	switch protocol {
	case "vless", "vmess":
		return map[string]interface{}{
			"vnext": []map[string]interface{}{
				{
					"address": address,
					"port":    port,
					"users": []map[string]interface{}{
						{
							"id":         cred,
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
					"password": cred,
				},
			},
		}
	case "shadowsocks":
		if method == "" {
			method = "chacha20-ietf-poly1305"
		}
		return map[string]interface{}{
			"servers": []map[string]interface{}{
				{
					"address":  address,
					"port":     port,
					"method":   method,
					"password": cred,
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
						{"id": cred, "email": "chain-relay"},
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
	case "hysteria2", "hysteria":
		// 中间形状：settings.servers[0] = {address, port, password}（obfs 由 buildExternalOutbound 注入），
		// 最终由 coreconfig.convertChainOutboundsToSingbox 转成 sing-box 的 hy2 出站。
		// 注意 xray 没有 hy2 出站 → 这条路只在入口核心是 sing-box 时可达（调用方已按核心校验）。
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
	case "hysteria2", "hysteria":
		// 外部 hy2 节点：ProxyJSON 里就是 password（部分来源写作 auth / auth_str）
		password := firstString(pm, "password", "auth", "auth_str")
		return []map[string]interface{}{{"password": password}}
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
