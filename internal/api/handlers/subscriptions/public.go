package subscriptions

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"corefusion/internal/model/entity"
	"corefusion/internal/service/plan"
	"corefusion/internal/service/traffic"
	"corefusion/internal/store/repository"
	"corefusion/internal/subscription/formatter"
	"corefusion/internal/subscription/mihomoconv"
	"corefusion/internal/subscription/sharelink"
	"corefusion/pkg/netutil"
)

// RateLimitPusher pushes IP association to rate limit agents.
// Implemented by ratelimitsvc.Service.
type RateLimitPusher interface {
	PushIPAssociation(ctx context.Context, serverID int64, ip string, subID int64, maxDownMbps, maxUpMbps, connLimit, deviceLimit int32) error
}

// PublicHandler handles public subscription requests.
type PublicHandler struct {
	subRepo           *repository.SubscriptionRepository
	inboundRepo       *repository.InboundConfigRepository
	shareDomainRepo   *repository.ShareDomainConfigRepository
	cfIPEntryRepo     *repository.CFIPEntryRepository
	clientRepo        *repository.ClientRepository
	planSvc           *plan.Service
	extNodeRepo       *repository.ExternalNodeRepository
	mihomoRepo        *repository.MihomoConfigRepository
	mihomoRuleRepo    *repository.MihomoRuleRepository
	systemSettingRepo *repository.SystemSettingRepository
	accessLog         *repository.AccessLogRepository
	rateLimitPusher   RateLimitPusher
	nodeServerRepo    *repository.NodeServerRepository
	networkTracker    *traffic.SubscriptionIPTracker
	// Chain proxy repos for filtering relay_only nodes from subscription
	// and rendering external node servers (design doc §8.1 / §8.2)
	chainExitCfgRepo *repository.ChainExitConfigRepository
	extServerRepo    *repository.ExternalNodeServerRepository
}

// SetNetworkTracker injects the nftables IP tracker so subscription access records can
// feed the network-layer traffic accounting bridge.
func (h *PublicHandler) SetNetworkTracker(t *traffic.SubscriptionIPTracker) {
	h.networkTracker = t
}

// NewPublicHandler creates a new public subscription handler.
func NewPublicHandler(
	subRepo *repository.SubscriptionRepository,
	inboundRepo *repository.InboundConfigRepository,
	shareDomainRepo *repository.ShareDomainConfigRepository,
	cfIPEntryRepo *repository.CFIPEntryRepository,
	clientRepo *repository.ClientRepository,
	planSvc *plan.Service,
	extNodeRepo *repository.ExternalNodeRepository,
	mihomoRepo *repository.MihomoConfigRepository,
	mihomoRuleRepo *repository.MihomoRuleRepository,
	systemSettingRepo *repository.SystemSettingRepository,
	accessLog *repository.AccessLogRepository,
) *PublicHandler {
	return &PublicHandler{
		subRepo:           subRepo,
		inboundRepo:       inboundRepo,
		shareDomainRepo:   shareDomainRepo,
		cfIPEntryRepo:     cfIPEntryRepo,
		clientRepo:        clientRepo,
		planSvc:           planSvc,
		extNodeRepo:       extNodeRepo,
		mihomoRepo:        mihomoRepo,
		mihomoRuleRepo:    mihomoRuleRepo,
		systemSettingRepo: systemSettingRepo,
		accessLog:         accessLog,
	}
}

// SetRateLimitPusher injects the rate limit pusher for IP association on subscription access.
func (h *PublicHandler) SetRateLimitPusher(pusher RateLimitPusher, nodeServerRepo *repository.NodeServerRepository) {
	h.rateLimitPusher = pusher
	h.nodeServerRepo = nodeServerRepo
}

// SetChainProxyRepos injects chain exit config and external node server repos
// for filtering relay_only nodes from subscription output (§8.1) and
// rendering external node servers as virtual entries (§8.2).
func (h *PublicHandler) SetChainProxyRepos(
	chainExitCfgRepo *repository.ChainExitConfigRepository,
	extServerRepo *repository.ExternalNodeServerRepository,
) {
	h.chainExitCfgRepo = chainExitCfgRepo
	h.extServerRepo = extServerRepo
}

// shareableProtocols are protocols that generate share links.
// shareableProtocols 是「可渲染成订阅节点」的协议全集。
// 注意：这里只判断「有没有能力渲染」，具体某订阅格式是否支持该协议由
// formatSupportedProtocols 决定（例如 anytls 只能进 sing-box/mihomo 格式，
// 不能进 xray_json）。sing-box 专属协议此前被这个 map 直接挡掉，永远不会出现在订阅里。
var shareableProtocols = map[string]bool{
	"vless":           true,
	"vmess":           true,
	"trojan":          true,
	"shadowsocks":     true,
	"shadowsocks2022": true,
	"hysteria":        true,
	"hysteria2":       true,
	"tuic":            true,
	"socks":           true,
	"http":            true,
	"mixed":           true,
	// sing-box 专属（仅 singbox_json / mihomo_yaml 格式支持）
	"anytls": true,
	"naive":  true,
	"snell":  true,
}

// shareDomainEntry holds a share domain with its ports.
type shareDomainEntry struct {
	Type  string
	Host  string
	Port  int
	Ports []int
}

type renderedSubscriptionNode struct {
	MemberID        int64
	InboundConfigID int64
	ExternalNodeID  int64
	Link            string
	Node            *formatter.Node
	ProxyMap        map[string]interface{}
}

type billingTemplateConfig struct {
	UsedTemplate      string
	RemainingTemplate string
	ResetTemplate     string
	ExpiryTemplate    string
	BillingNodeIDs    []int64 // NodeGroupMember IDs from subscription
}

// ServeSubscription handles GET /sub/:token.
func (h *PublicHandler) ServeSubscription(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "MISSING_TOKEN", "message": "token is required"})
		return
	}

	ctx := c.Request.Context()

	sub, err := h.subRepo.GetByToken(ctx, token)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "SUBSCRIPTION_NOT_FOUND", "message": "subscription not found"})
		return
	}

	// Set Subscription-Userinfo header immediately so it's available
	// even on error responses (suspended/expired/inactive).
	// This is critical for compatibility with subscription tools
	// (Clash, V2RayN, Shadowrocket, Surge, Quantumult X, etc.)
	// that display traffic info regardless of subscription status.
	c.Header("Subscription-Userinfo", formatSubscriptionInfo(sub))
	c.Header("Profile-Update-Interval", "6")

	// Build Profile-Title and Profile-Web-Page-Url headers.
	// These are supported by Clash Verge Rev, V2RayN, and other modern
	// subscription tools to display a custom profile name and link.
	profileTitle := "CoreFusion"
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	// Profile-Web-Page-Url: use system setting if configured,
	// otherwise auto-detect from request host (strip port).
	if h.systemSettingRepo != nil {
		if portalURL, err := h.systemSettingRepo.GetByKey(ctx, "panel.profile_web_page_url"); err == nil && portalURL != "" {
			c.Header("Profile-Web-Page-Url", portalURL)
		}
	}
	if c.GetHeader("Profile-Web-Page-Url") == "" {
		portalHost := c.GetHeader("X-Forwarded-Host")
		if portalHost == "" {
			portalHost = c.Request.Host
		}
		if portalHost != "" {
			if host, _, err := net.SplitHostPort(portalHost); err == nil {
				portalHost = host
			}
			c.Header("Profile-Web-Page-Url", fmt.Sprintf("%s://%s/portal", scheme, portalHost))
		}
	}

	// Detect client language from Accept-Language header
	lang := detectLanguage(c.GetHeader("Accept-Language"))

	// Load client info once (used for status check, profile title, and plan nodes)
	var clientEmail string
	var tmpl *entity.PlanTemplate
	var members []entity.NodeGroupMember
	if sub.ClientID > 0 && h.clientRepo != nil {
		if cl, err := h.clientRepo.GetByID(ctx, sub.ClientID); err == nil {
			clientEmail = cl.Email
			if clientEmail != "" {
				profileTitle = clientEmail
			}

			// Check client status first (client may be suspended while subscription is still active)
			switch cl.Status {
			case "suspended":
				c.Header("Profile-Title", profileTitle)
				c.Header("Content-Type", "text/plain; charset=utf-8")
				c.String(http.StatusForbidden, t(lang, "suspended"))
				return
			case "expired":
				c.Header("Profile-Title", profileTitle)
				c.Header("Content-Type", "text/plain; charset=utf-8")
				c.String(http.StatusForbidden, t(lang, "expired"))
				return
			}

			if cl.PlanID > 0 && h.planSvc != nil {
				tmpl, _ = h.planSvc.GetTemplateByID(ctx, cl.PlanID)
				if rawMembers, err := h.planSvc.GetTemplateNodeMembers(ctx, cl.PlanID); err == nil {
					for _, m := range rawMembers {
						if m.Enabled {
							members = append(members, m)
						}
					}
				}
			}
		}
	}

	c.Header("Profile-Title", profileTitle)

	if sub.Status != "active" {
		c.Header("Content-Type", "text/plain; charset=utf-8")
		c.String(http.StatusForbidden, t(lang, "inactive"))
		return
	}

	shareDomains := h.loadShareDomains(ctx)

	// Format output - query param ?format= overrides subscription default
	formatStr := c.Query("format")
	if formatStr == "" {
		formatStr = sub.Format
	}
	if formatStr == "" {
		formatStr = "base64_uri"
	}

	billingTemplates := h.loadBillingTemplateConfig(ctx, tmpl)
	billingTemplates.BillingNodeIDs = sub.BillingNodeIDs
	// Fallback: if subscription doesn't specify billing node IDs,
	// use the template's billing node IDs.
	if len(billingTemplates.BillingNodeIDs) == 0 && tmpl != nil {
		billingTemplates.BillingNodeIDs = []int64(tmpl.BillingNodeIDs)
	}
	renderedNodes := h.collectRenderedNodes(ctx, shareDomains, members, clientEmail, billingTemplates)
	// 各订阅格式支持的协议不同：不支持的协议不要写进订阅（客户端会解析失败/忽略）
	renderedNodes = filterRenderedNodesForFormat(renderedNodes, formatStr)
	links := renderedNodeLinks(renderedNodes)
	nodes := renderedFormatterNodes(renderedNodes)
	billingNodes := buildBillingInfoEntries(sub, billingTemplates, renderedNodes)
	billingLinks := renderedNodeLinks(billingNodes)
	billingFormatterNodes := renderedFormatterNodes(billingNodes)

	var data []byte
	contentType := "text/plain"

	// Determine mihomo config: ?config= param > ?platform= match > subscription linked config
	var mihomoConfig *entity.MihomoConfig
	var mihomoRules []entity.MihomoRule

	configName := c.Query("config")
	platformName := c.Query("platform")
	if configName != "" && h.mihomoRepo != nil {
		configs, _ := h.mihomoRepo.List(ctx)
		for _, cfg := range configs {
			if cfg.Name == configName && cfg.Enabled {
				mihomoConfig = &cfg
				break
			}
		}
	} else if platformName != "" && h.mihomoRepo != nil {
		// Try to find a config matching the requested platform
		configs, _ := h.mihomoRepo.List(ctx)
		for _, cfg := range configs {
			if cfg.Enabled && cfg.Platform == platformName && cfg.Format == "mihomo_yaml" {
				mihomoConfig = &cfg
				break
			}
		}
		// Fallback: if no exact platform match, try universal (empty platform)
		if mihomoConfig == nil {
			for _, cfg := range configs {
				if cfg.Enabled && cfg.Platform == "" && cfg.Format == "mihomo_yaml" {
					mihomoConfig = &cfg
					break
				}
			}
		}
	} else if sub.MihomoConfigID != nil && h.mihomoRepo != nil {
		mihomoConfig, _ = h.mihomoRepo.GetByID(ctx, *sub.MihomoConfigID)
	}

	if mihomoConfig != nil && h.mihomoRuleRepo != nil {
		mihomoRules, _ = h.mihomoRuleRepo.ListByConfigID(ctx, mihomoConfig.ID)
	}

	// Build subscription info header (for browser display)
	infoHeader := buildSubscriptionInfoHeader(sub, billingTemplates)

	// Build the Subscription-Userinfo body comment line.
	// This is prepended to raw_uri and base64_uri responses so that tools
	// which only parse the response body (not HTTP headers) can still
	// display traffic usage info.
	userinfoLine := "Subscription-Userinfo: " + formatSubscriptionInfo(sub)

	switch formatter.Format(formatStr) {
	case formatter.FormatBase64Aggregate:
		joined := strings.Join(append(billingLinks, links...), "\n")
		// Prepend Subscription-Userinfo as plain-text line before base64 content.
		// Pattern: "Subscription-Userinfo: ...\n\n<base64>"
		// Supported by v2rayN and tools following the V2board convention.
		data = []byte(userinfoLine + "\n\n" + base64.StdEncoding.EncodeToString([]byte(joined)))
	case formatter.FormatRawURI:
		allLinks := append(billingLinks, links...)
		var headerLines []string
		// Add Subscription-Userinfo as body comment for tools that only read from body
		headerLines = append(headerLines, "# "+userinfoLine)
		if infoHeader != "" {
			headerLines = append(headerLines, infoHeader)
		}
		headerStr := strings.Join(headerLines, "\n") + "\n"
		data = []byte(headerStr + strings.Join(allLinks, "\n"))
	case formatter.FormatMihomoYAML:
		allItems := append(billingNodes, renderedNodes...)
		proxyMaps := buildRenderedMihomoProxyMaps(allItems)
		if mihomoConfig != nil {
			data = h.generateMihomoYAML(mihomoConfig, mihomoRules, proxyMaps)
		} else {
			data = generatePlainMihomoYAML(proxyMaps)
		}
		contentType = "text/yaml"
	case formatter.FormatSingBoxJSON:
		nodes = append(billingFormatterNodes, nodes...)
		f := formatter.GetFormatter(formatter.FormatSingBoxJSON)
		data, _ = f.Format(nodes)
		contentType = f.ContentType()
	case formatter.FormatXrayJSON:
		// 与 sing-box 一致：直接用**配置派生的节点**，不要从链接反解。
		// 链接是**有损中间态**：raw 会被写成 tcp（用户明确要求 xray 的 JSON 保持 raw）、
		// kcp 的 mtu/tti/header/seed（finalmask）、tag 里的空格/中文（会变成 %20 转义）
		// 都会丢或变形 —— 反解出来的节点既对不上弹窗里的片段，也对不上服务端配置。
		nodes = append(billingFormatterNodes, nodes...)
		f := formatter.GetFormatter(formatter.FormatXrayJSON)
		data, _ = f.Format(nodes)
		contentType = f.ContentType()
	default:
		joined := strings.Join(append(billingLinks, links...), "\n")
		data = []byte(userinfoLine + "\n\n" + base64.StdEncoding.EncodeToString([]byte(joined)))
	}

	// Record access
	if h.accessLog != nil {
		ip := netutil.GetRealIP(c.Request)
		ua := c.Request.UserAgent()
		realIP := c.GetHeader("X-Real-IP")
		cdnIP := c.GetHeader("CF-Connecting-IP")
		cdnProvider := detectCDNProvider(c)
		go func() {
			h.recordAccess(sub.ID, ip, ua, realIP, cdnIP, cdnProvider)
		}()

		// Feed the network-layer (nftables) accounting bridge: record this
		// subscription's source IP. Only subscriptions NOT already counted by
		// xray are actually billed by the bridge.
		if h.networkTracker != nil {
			h.networkTracker.RecordAccess(sub.ID, sub.ClientID, ip)
		}

		// Push IP association to rate limit agents (P0 trigger source).
		// When a user accesses their subscription URL, we know their real IP
		// and which subscription they belong to — push this to every node
		// that has rate limiting enabled, so the agent can apply nft+tc rules.
		if h.rateLimitPusher != nil && sub.ClientID > 0 {
			clientIP := netutil.GetRealIP(c.Request)
			go h.pushRateLimitAssociation(clientIP, sub, tmpl)
		}
	}

	c.Header("Content-Type", contentType+"; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=\"subscription.txt\"")
	c.Data(http.StatusOK, contentType+"; charset=utf-8", data)
}

// collectShareLinks generates share link URIs using the sharelink package.
func (h *PublicHandler) collectShareLinks(
	ctx context.Context,
	shareDomains []shareDomainEntry,
	allowedInboundIDs map[int64]bool,
	clientEmail string,
) []string {
	var links []string

	for _, coreType := range []string{"xray", "singbox", "hysteria2", "tuic"} {
		configs, err := h.inboundRepo.ListAll(ctx, coreType)
		if err != nil {
			continue
		}
		for _, cfg := range configs {
			if cfg.Port == 0 || !shareableProtocols[cfg.Protocol] {
				continue
			}
			// Filter by plan nodes if client has a plan
			if allowedInboundIDs != nil && !allowedInboundIDs[cfg.ID] {
				continue
			}
			var config map[string]interface{}
			if err := json.Unmarshal([]byte(cfg.ConfigJSON), &config); err != nil {
				continue
			}
			// sing-box 入站归一成 xray 形态（否则用户凭证找不到，该节点会被整体跳过）
			if view, skip := subscriptionView(cfg.Tag, config, clientEmail); skip {
				continue
			} else {
				config = view
			}
			// Only nodes with _accessDomain enabled
			ad, ok := config["_accessDomain"].(map[string]interface{})
			if !ok || !ad["enabled"].(bool) {
				continue
			}

			host, port := h.shareAddress(ctx, cfg, config, shareDomains)

			// Find the specific client's credentials for this subscription
			settings := getMap(config, "settings")
			if clientEmail != "" {
				if userClient := findClientByEmail(settings, clientEmail, cfg.Tag); userClient != nil {
					// Create a settings map with only this client
					settings = map[string]interface{}{
						"clients":    []interface{}{userClient},
						"encryption": getString(settings, "encryption", "none"),
					}
				} else {
					// Client has no account on this node → skip
					continue
				}
			} else {
				// No client email → skip nodes requiring authentication
				continue
			}
			// ss/ss2022：链接需要 method，多用户 ss2022 的密码必须是 `服务端密钥:用户密钥`。
			// 上面只挑了 clients/encryption，会把 2022-blake3 写成默认 aes-256-gcm、且密码只有
			// 用户密钥 → 订阅链接（以及由链接派生的 mihomo 片段）连不上。
			settings = linkSettingsForUser(config, settings, cfg.Protocol, getString(firstClient(settings), "password", ""))

			nodeName := cfg.Remark
			if nodeName == "" {
				nodeName = cfg.Tag
			}
			node := sharelink.Node{
				Tag:            nodeName,
				Protocol:       cfg.Protocol,
				Host:           host,
				Port:           port,
				Settings:       settings,
				StreamSettings: getMap(config, "streamSettings"),
				FinalMask:      finalMaskWithPortHop(config),
			}

			link := sharelink.Generate(node)
			if link != "" {
				links = append(links, link)
			}
		}
	}
	return links
}

func renderedNodeLinks(items []renderedSubscriptionNode) []string {
	links := make([]string, 0, len(items))
	for _, item := range items {
		if item.Link != "" {
			links = append(links, item.Link)
		}
	}
	return links
}

func renderedFormatterNodes(items []renderedSubscriptionNode) []formatter.Node {
	nodes := make([]formatter.Node, 0, len(items))
	for _, item := range items {
		if item.Node != nil {
			nodes = append(nodes, *item.Node)
		}
	}
	return nodes
}

func (h *PublicHandler) collectRenderedNodes(ctx context.Context, shareDomains []shareDomainEntry, members []entity.NodeGroupMember, clientEmail string, templates billingTemplateConfig) []renderedSubscriptionNode {
	var items []renderedSubscriptionNode

	// Preload node servers to filter out non-primary primary_backup nodes.
	// Only the active primary should appear in subscription output.
	// Both "backup" and "backup_recovering" nodes must NOT be returned —
	// they become visible only after failover promotes them to primary.
	backupServerIDs := make(map[int64]bool)
	// Also build mirror node mapping: primaryServerID → []mirrorNodeServer
	// This is used for virtual expansion of mirror nodes in subscription output.
	mirrorMap := make(map[int64][]entity.NodeServer)
	// Build set of relay_only server IDs — pure exit nodes must not appear
	// in subscription output (design doc §8.1).
	relayOnlyServerIDs := make(map[int64]bool)
	if h.chainExitCfgRepo != nil {
		if exitCfgs, err := h.chainExitCfgRepo.ListAll(ctx); err == nil {
			for _, ec := range exitCfgs {
				if ec.Enabled && ec.RelayOnly {
					relayOnlyServerIDs[ec.ServerID] = true
				}
			}
		}
	}
	if h.nodeServerRepo != nil {
		if servers, err := h.nodeServerRepo.ListAll(ctx); err == nil {
			for _, s := range servers {
				if s.NodeType == entity.NodeTypePrimaryBackup &&
					(s.FailoverRole == entity.FailoverRoleBackup ||
						s.FailoverRole == entity.FailoverRoleBackupRecovering) {
					backupServerIDs[s.ID] = true
				}
				if s.NodeType == entity.NodeTypeMirror && s.PartnerServerID > 0 && s.Enabled {
					mirrorMap[s.PartnerServerID] = append(mirrorMap[s.PartnerServerID], s)
				}
			}
		}
	}

	inboundMemberIDs := make(map[int64][]int64)
	externalMemberIDs := make(map[int64][]int64)
	if len(members) > 0 {
		for _, member := range members {
			if member.InboundConfigID > 0 {
				inboundMemberIDs[member.InboundConfigID] = append(inboundMemberIDs[member.InboundConfigID], member.ID)
			}
			if member.ExternalNodeID > 0 {
				externalMemberIDs[member.ExternalNodeID] = append(externalMemberIDs[member.ExternalNodeID], member.ID)
			}
		}
	}

	// Build a set of billing-selected inbound config IDs and external node IDs.
	// These nodes should be rendered even if they're not in the plan.
	billingMemberSet := make(map[int64]bool)
	for _, id := range templates.BillingNodeIDs {
		billingMemberSet[id] = true
	}
	billingInboundSet := make(map[int64]bool)
	billingExternalSet := make(map[int64]bool)
	for _, m := range members {
		if billingMemberSet[m.ID] {
			if m.InboundConfigID > 0 {
				billingInboundSet[m.InboundConfigID] = true
			}
			if m.ExternalNodeID > 0 {
				billingExternalSet[m.ExternalNodeID] = true
			}
		}
	}

	for _, coreType := range []string{"xray", "singbox", "hysteria2", "tuic"} {
		configs, err := h.inboundRepo.ListAll(ctx, coreType)
		if err != nil {
			continue
		}
		for _, cfg := range configs {
			if cfg.Port == 0 || !shareableProtocols[cfg.Protocol] {
				continue
			}
			// Skip inbound configs on primary_backup backup nodes.
			// Only the active primary should serve traffic; the backup
			// becomes visible after failover promotes it to primary.
			if backupServerIDs[cfg.ServerID] {
				continue
			}
			// Skip pure exit nodes (relay_only=true) — they must not
			// appear in subscription output (design doc §8.1).
			if relayOnlyServerIDs[cfg.ServerID] {
				continue
			}
			memberIDs := inboundMemberIDs[cfg.ID]
			isBillingSelected := billingInboundSet[cfg.ID]
			if len(members) > 0 && len(memberIDs) == 0 && !isBillingSelected {
				continue
			}
			if len(memberIDs) == 0 {
				memberIDs = []int64{0}
			}

			var config map[string]interface{}
			if err := json.Unmarshal([]byte(cfg.ConfigJSON), &config); err != nil {
				continue
			}
			// sing-box 入站归一成 xray 形态（否则用户凭证找不到，该节点会被整体跳过）
			if view, skip := subscriptionView(cfg.Tag, config, clientEmail); skip {
				continue
			} else {
				config = view
			}
			ad, ok := config["_accessDomain"].(map[string]interface{})
			if !ok {
				continue
			}
			enabled, _ := ad["enabled"].(bool)
			if !enabled {
				continue
			}

			host, port := h.shareAddress(ctx, cfg, config, shareDomains)
			settings := getMap(config, "settings")

			linkSettings := settings
			credExtra := map[string]interface{}{}
			var uuid, password, flow string
			if clientEmail != "" {
				userClient := findClientByEmail(settings, clientEmail, cfg.Tag)
				if userClient == nil {
					// Client has no account on this node → skip (security: never use other users' credentials)
					continue
				}
				linkSettings = map[string]interface{}{
					"clients":    []interface{}{userClient},
					"encryption": getString(settings, "encryption", "none"),
				}
				uuid, _ = userClient["id"].(string)
				password, _ = userClient["password"].(string)
				flow, _ = userClient["flow"].(string)
				if cfg.Protocol == "hysteria" {
					if auth, ok := userClient["auth"].(string); ok && auth != "" {
						password = auth
					}
				}
				if cfg.Protocol == "snell" {
					// snell 的 psk 在入站**顶层**（不是每个用户一条），客户端出站需要它
					if psk := getString(config, "psk", ""); psk != "" {
						password = psk
					}
				}
			} else {
				// No client email → skip nodes requiring authentication (security)
				continue
			}
			// 凭据字段按协议取（hysteria→auth、http/socks→accounts 的 user/pass、snell→顶层 psk）：
			// 取错字段的节点"看得到、认证过不了"。这里先把用户记录里的凭据规整好，
			// 再算 SS-2022 的多用户密码（`服务端密钥:用户密钥`）。
			applyClientCredentials(cfg.Protocol, firstClient(linkSettings), config, &password, credExtra)
			effectivePassword := ss2022ClientPassword(config, cfg.Protocol, password)

			nodeName := cfg.Remark
			if nodeName == "" {
				nodeName = cfg.Tag
			}
			shareNode := sharelink.Node{
				Tag:            nodeName,
				Protocol:       cfg.Protocol,
				Host:           host,
				Port:           port,
				Settings:       linkSettingsForUser(config, linkSettings, cfg.Protocol, password),
				StreamSettings: getMap(config, "streamSettings"),
				FinalMask:      finalMaskWithPortHop(config),
			}
			link := sharelink.Generate(shareNode)

			ss := getMap(config, "streamSettings")
			protocol := cfg.Protocol
			if protocol == "hysteria" {
				protocol = "hysteria2"
			}
			transport := normalizeNodeTransport(getString(ss, "network", "raw"))
			node := &formatter.Node{
				Name:       nodeName,
				CoreType:   cfg.CoreType,
				Protocol:   protocol,
				Host:       host,
				Port:       port,
				Transport:  transport,
				TLS:        getString(ss, "security", "none") != "none",
				ServerName: getTLSServerName(ss),
				UUID:       uuid,
				Password:   effectivePassword,
				Flow:       flow,
				Encryption: nodeEncryption(cfg.Protocol, getMap(config, "settings")),
				Extra:      buildExtraWith(config, ss, cfg.Protocol, credExtra),
			}
			for _, memberID := range memberIDs {
				item := renderedSubscriptionNode{
					MemberID:        memberID,
					InboundConfigID: cfg.ID,
					Link:            link,
				}
				nodeCopy := *node
				item.Node = &nodeCopy
				if item.Link == "" && item.Node == nil {
					continue
				}
				items = append(items, item)
			}

			// Virtual expansion: if this inbound belongs to a primary server
			// that has mirror nodes, generate an additional subscription
			// entry for each mirror. The mirror uses the same protocol/port/
			// credentials as the primary, but with its own host address.
			if mirrors, ok := mirrorMap[cfg.ServerID]; ok {
				for _, mirror := range mirrors {
					mirrorHost := mirror.PublicIP
					if mirrorHost == "" {
						mirrorHost = mirror.Hostname
					}
					if mirrorHost == "" {
						continue
					}

					// Deep-copy the config and replace _accessDomain
					mirrorConfig := deepCopyMap(config)
					replaceAccessDomainForMirror(mirrorConfig, mirror)
					mirrorHostResolved, mirrorPort, resolved := resolveAccessDomain(mirrorConfig, cfg.Port, shareDomains)
					if !resolved {
						// 镜像的 _accessDomain 没被改写成功（类型不认识等）→ 直接用镜像自己的地址
						mirrorHostResolved, mirrorPort = mirrorHost, cfg.Port
					}

					mirrorNodeName := fmt.Sprintf("%s (镜像 %s)", nodeName, mirror.Name)
					if mirror.Name == "" {
						mirrorNodeName = fmt.Sprintf("%s (镜像 %d)", nodeName, mirror.ID)
					}

					mirrorShareNode := sharelink.Node{
						Tag:            mirrorNodeName,
						Protocol:       cfg.Protocol,
						Host:           mirrorHostResolved,
						Port:           mirrorPort,
						Settings:       linkSettingsForUser(mirrorConfig, linkSettings, cfg.Protocol, password),
						StreamSettings: getMap(mirrorConfig, "streamSettings"),
						FinalMask:      getFinalMask(mirrorConfig),
					}
					mirrorLink := sharelink.Generate(mirrorShareNode)

					mirrorNode := &formatter.Node{
						Name:       mirrorNodeName,
						CoreType:   cfg.CoreType,
						Protocol:   protocol,
						Host:       mirrorHostResolved,
						Port:       mirrorPort,
						Transport:  transport,
						TLS:        node.TLS,
						ServerName: node.ServerName,
						UUID:       uuid,
						Password:   effectivePassword,
						Flow:       flow,
						Encryption: node.Encryption,
						Extra:      node.Extra,
					}

					for _, memberID := range memberIDs {
						item := renderedSubscriptionNode{
							MemberID:        memberID,
							InboundConfigID: cfg.ID,
							Link:            mirrorLink,
						}
						nodeCopy := *mirrorNode
						item.Node = &nodeCopy
						if item.Link == "" && item.Node == nil {
							continue
						}
						items = append(items, item)
					}
				}
			}
		}
	}

	if h.extNodeRepo == nil {
		return items
	}
	allExternalNodes, err := h.extNodeRepo.ListAll(ctx)
	if err != nil {
		return items
	}
	for _, extNode := range allExternalNodes {
		if !extNode.Enabled {
			continue
		}
		memberIDs := externalMemberIDs[extNode.ID]
		isBillingSelected := billingExternalSet[extNode.ID]
		if len(members) > 0 && len(memberIDs) == 0 && !isBillingSelected {
			continue
		}
		if len(memberIDs) == 0 {
			memberIDs = []int64{0}
		}

		var pm map[string]interface{}
		if json.Unmarshal([]byte(extNode.ProxyJSON), &pm) != nil {
			continue
		}
		node := mihomoProxyMapToNode(pm)
		if node != nil && extNode.Name != "" {
			node.Name = extNode.Name
		}
		link := formatter.MihomoToURI(pm)
		if link != "" {
			link = renameShareLink(link, extNode.Name)
		}
		for _, memberID := range memberIDs {
			item := renderedSubscriptionNode{
				MemberID:       memberID,
				ExternalNodeID: extNode.ID,
				Link:           link,
			}
			if node != nil {
				nodeCopy := *node
				item.Node = &nodeCopy
			}
			item.ProxyMap = cloneProxyMap(pm)
			if item.ProxyMap != nil && extNode.Name != "" {
				item.ProxyMap["name"] = extNode.Name
			}
			if item.Link == "" && item.Node == nil {
				continue
			}
			items = append(items, item)
		}
	}

	// Render external node servers registered as virtual entries (§8.2).
	// These are external_node_servers records — external nodes that the
	// admin has explicitly registered as "virtual entry" nodes for
	// subscription display. Unlike regular external_nodes (which appear
	// via node group membership), external_node_servers are always shown
	// when enabled and the subscription has no plan restriction on them.
	if h.extServerRepo != nil {
		extServers, err := h.extServerRepo.ListAll(ctx)
		if err == nil {
			for _, es := range extServers {
				if !es.Enabled {
					continue
				}
				// Get the underlying external node for proxy JSON
				extNode, err := h.extNodeRepo.GetByID(ctx, es.ExternalNodeID)
				if err != nil || extNode == nil || !extNode.Enabled {
					continue
				}

				var pm map[string]interface{}
				if json.Unmarshal([]byte(extNode.ProxyJSON), &pm) != nil {
					continue
				}

				node := mihomoProxyMapToNode(pm)
				if node != nil && es.Name != "" {
					node.Name = es.Name
				}
				link := formatter.MihomoToURI(pm)
				if link != "" {
					link = renameShareLink(link, es.Name)
				}

				item := renderedSubscriptionNode{
					ExternalNodeID: extNode.ID,
					Link:           link,
				}
				if node != nil {
					nodeCopy := *node
					item.Node = &nodeCopy
				}
				item.ProxyMap = cloneProxyMap(pm)
				if item.ProxyMap != nil && es.Name != "" {
					item.ProxyMap["name"] = es.Name
				}
				if item.Link == "" && item.Node == nil {
					continue
				}
				items = append(items, item)
			}
		}
	}

	return items
}

func buildBillingInfoEntries(sub *entity.Subscription, templates billingTemplateConfig, items []renderedSubscriptionNode) []renderedSubscriptionNode {
	if len(templates.BillingNodeIDs) == 0 {
		return nil
	}
	labels := buildBillingInfoLabels(sub, templates)
	if len(labels) == 0 {
		return nil
	}
	// Build source map keyed by MemberID.
	sources := make(map[int64]renderedSubscriptionNode)
	for _, item := range items {
		if item.Link == "" && item.Node == nil {
			continue
		}
		if item.MemberID > 0 {
			if _, exists := sources[item.MemberID]; !exists {
				sources[item.MemberID] = item
			}
		}
	}
	if len(sources) == 0 {
		return nil
	}

	result := make([]renderedSubscriptionNode, 0, len(labels))
	for idx, label := range labels {
		memberID := templates.BillingNodeIDs[idx%len(templates.BillingNodeIDs)]
		source, ok := sources[memberID]
		if !ok {
			continue
		}
		cloned := renderedSubscriptionNode{}
		if source.Link != "" {
			cloned.Link = renameShareLink(source.Link, label)
		}
		if source.Node != nil {
			nodeCopy := *source.Node
			nodeCopy.Name = label
			cloned.Node = &nodeCopy
		} else if cloned.Link != "" {
			cloned.Node = formatter.ParseURIToNode(cloned.Link)
		}
		if source.ProxyMap != nil {
			cloned.ProxyMap = cloneProxyMap(source.ProxyMap)
			if cloned.ProxyMap != nil {
				cloned.ProxyMap["name"] = label
			}
		}
		if cloned.Link == "" && cloned.Node == nil {
			continue
		}
		result = append(result, cloned)
	}
	return result
}

func renameShareLink(link, newName string) string {
	if link == "" || newName == "" {
		return link
	}
	if strings.HasPrefix(link, "vmess://") {
		payload := strings.TrimPrefix(link, "vmess://")
		raw, err := decodeVmessPayload(payload)
		if err == nil {
			var obj map[string]interface{}
			if json.Unmarshal(raw, &obj) == nil {
				obj["ps"] = newName
				if data, err := json.Marshal(obj); err == nil {
					return "vmess://" + base64.StdEncoding.EncodeToString(data)
				}
			}
		}
		return link
	}
	if idx := strings.LastIndex(link, "#"); idx >= 0 {
		return link[:idx+1] + url.PathEscape(newName)
	}
	return link + "#" + url.PathEscape(newName)
}

func decodeVmessPayload(payload string) ([]byte, error) {
	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	var lastErr error
	for _, encoding := range encodings {
		raw, err := encoding.DecodeString(payload)
		if err == nil {
			return raw, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

// collectExternalNodesAndLinks returns formatter.Nodes and share URIs from external nodes.
func (h *PublicHandler) collectExternalNodesAndLinks(ctx context.Context, allowedIDs map[int64]bool) ([]formatter.Node, []string) {
	var nodes []formatter.Node
	var links []string
	all, err := h.extNodeRepo.ListAll(ctx)
	if err != nil {
		return nodes, links
	}
	for _, n := range all {
		if !n.Enabled || !allowedIDs[n.ID] {
			continue
		}
		var pm map[string]interface{}
		if json.Unmarshal([]byte(n.ProxyJSON), &pm) != nil {
			continue
		}
		// Use mihomo proxy map directly for Node (all fields preserved)
		node := mihomoProxyMapToNode(pm)
		if node != nil {
			if n.Name != "" {
				node.Name = n.Name
			}
			nodes = append(nodes, *node)
		}
		// Generate complete URI from mihomo proxy map
		uri := formatter.MihomoToURI(pm)
		if uri != "" {
			// Replace fragment with external node name
			if idx := strings.LastIndex(uri, "#"); idx >= 0 {
				uri = uri[:idx+1] + url.PathEscape(n.Name)
			}
			links = append(links, uri)
		}
	}
	return nodes, links
}

// collectFormatterNodes collects nodes for the formatter (used by singbox/xray/mihomo formats).
func (h *PublicHandler) collectFormatterNodes(ctx context.Context, shareDomains []shareDomainEntry, allowedInboundIDs map[int64]bool, clientEmail string) []formatter.Node {
	var nodes []formatter.Node

	for _, coreType := range []string{"xray", "singbox", "hysteria2", "tuic"} {
		configs, err := h.inboundRepo.ListAll(ctx, coreType)
		if err != nil {
			continue
		}
		for _, cfg := range configs {
			if cfg.Port == 0 || !shareableProtocols[cfg.Protocol] {
				continue
			}
			// Filter by plan nodes if client has a plan
			if allowedInboundIDs != nil && !allowedInboundIDs[cfg.ID] {
				continue
			}
			var config map[string]interface{}
			if err := json.Unmarshal([]byte(cfg.ConfigJSON), &config); err != nil {
				continue
			}
			// sing-box 入站归一成 xray 形态（否则用户凭证找不到，该节点会被整体跳过）
			if view, skip := subscriptionView(cfg.Tag, config, clientEmail); skip {
				continue
			} else {
				config = view
			}
			ad, ok := config["_accessDomain"].(map[string]interface{})
			if !ok || !ad["enabled"].(bool) {
				continue
			}

			host, port := h.shareAddress(ctx, cfg, config, shareDomains)
			settings := getMap(config, "settings")
			credExtra := map[string]interface{}{}
			var uuid, password, flow string

			// Find the specific client's credentials for this subscription
			var credClient map[string]interface{}
			if clientEmail != "" {
				userClient := findClientByEmail(settings, clientEmail, cfg.Tag)
				if userClient == nil {
					// Client has no account on this node → skip (security: never use other users' credentials)
					continue
				}
				uuid, _ = userClient["id"].(string)
				password, _ = userClient["password"].(string)
				flow, _ = userClient["flow"].(string)
				// Hysteria uses "auth" field instead of "password"
				if cfg.Protocol == "hysteria" {
					if auth, ok := userClient["auth"].(string); ok && auth != "" {
						password = auth
					}
				}
				if cfg.Protocol == "snell" {
					// snell 的 psk 在入站**顶层**（不是每个用户一条），客户端出站需要它
					if psk := getString(config, "psk", ""); psk != "" {
						password = psk
					}
				}
				credClient = userClient
			} else {
				// No client email → skip nodes requiring authentication (security)
				continue
			}
			applyClientCredentials(cfg.Protocol, credClient, config, &password, credExtra)
			effectivePassword := ss2022ClientPassword(config, cfg.Protocol, password)

			ss := getMap(config, "streamSettings")
			nodeName := cfg.Remark
			if nodeName == "" {
				nodeName = cfg.Tag
			}
			protocol := cfg.Protocol
			if protocol == "hysteria" {
				protocol = "hysteria2"
			}
			extra := buildExtra(config, ss, cfg.Protocol)
			transport := normalizeNodeTransport(getString(ss, "network", "raw"))
			node := formatter.Node{
				Name:       nodeName,
				CoreType:   cfg.CoreType,
				Protocol:   protocol,
				Host:       host,
				Port:       port,
				Transport:  transport,
				TLS:        getString(ss, "security", "none") != "none",
				ServerName: getTLSServerName(ss),
				UUID:       uuid,
				Password:   effectivePassword,
				Flow:       flow,
				Encryption: nodeEncryption(cfg.Protocol, getMap(config, "settings")),
				Extra:      mergeExtraMaps(extra, credExtra),
			}
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// generateMihomoYAML generates a complete mihomo YAML config with proxies and rules.
func (h *PublicHandler) generateMihomoYAML(config *entity.MihomoConfig, rules []entity.MihomoRule, proxyMaps []map[string]interface{}) []byte {
	var yaml strings.Builder

	// Parse config_json
	var cfg map[string]interface{}
	json.Unmarshal([]byte(config.ConfigJSON), &cfg)

	// Build node names list for proxy groups injection
	nodeNames := getMihomoProxyNames(proxyMaps)

	// Output all top-level fields except proxies and rules (those are injected)
	skipKeys := map[string]bool{"proxies": true, "rules": true}

	for _, key := range getSortedKeys(cfg) {
		if skipKeys[key] {
			continue
		}
		value := cfg[key]
		// Skip empty or disabled fields
		if isEmptyOrDisabled(key, value) {
			continue
		}
		// For proxy-groups, inject node names into groups that have _hasExternalNodes
		if key == "proxy-groups" {
			writeProxyGroups(&yaml, value, nodeNames)
			continue
		}
		// For DNS, clean up corrupted nameserver-policy
		if key == "dns" {
			value = cleanDNSConfig(value)
		}
		writeYAMLField(&yaml, key, value, 0)
	}

	yaml.WriteString(buildMihomoProxiesYAML(proxyMaps))

	// Inject rules from database
	if len(rules) > 0 {
		yaml.WriteString("\nrules:\n")
		for _, rule := range rules {
			if rule.Enabled {
				yaml.WriteString("  - " + rule.RuleString + "\n")
			}
		}
	}

	return []byte(yaml.String())
}

func generatePlainMihomoYAML(proxyMaps []map[string]interface{}) []byte {
	return []byte(buildMihomoProxiesYAML(proxyMaps))
}

func buildMihomoProxiesYAML(proxyMaps []map[string]interface{}) string {
	var yaml strings.Builder
	yaml.WriteString("proxies:\n")
	for _, proxyMap := range proxyMaps {
		writeMihomoProxyMap(&yaml, proxyMap)
	}
	return yaml.String()
}

func getMihomoProxyNames(proxyMaps []map[string]interface{}) []string {
	names := make([]string, 0, len(proxyMaps))
	for _, proxyMap := range proxyMaps {
		name, _ := proxyMap["name"].(string)
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

// writeProxyGroups writes proxy-groups with node injection for groups that have _hasExternalNodes.
func writeProxyGroups(yaml *strings.Builder, value interface{}, nodeNames []string) {
	groups, ok := value.([]interface{})
	if !ok || len(groups) == 0 {
		return
	}
	yaml.WriteString("proxy-groups:\n")
	for _, g := range groups {
		group, ok := g.(map[string]interface{})
		if !ok {
			continue
		}
		yaml.WriteString("  - name: " + formatYAMLString(getString(group, "name", "")) + "\n")
		yaml.WriteString("    type: " + getString(group, "type", "select") + "\n")

		// Build proxies list: existing proxies + node names (if _hasExternalNodes)
		proxies := getStringSlice(group, "proxies")
		hasExternal, _ := group["_hasExternalNodes"].(bool)
		if hasExternal && len(nodeNames) > 0 {
			proxies = append(proxies, nodeNames...)
		}

		if len(proxies) > 0 {
			yaml.WriteString("    proxies:\n")
			for _, p := range proxies {
				yaml.WriteString("      - " + formatYAMLString(p) + "\n")
			}
		}

		// Optional fields
		if url := getString(group, "url", ""); url != "" {
			yaml.WriteString("    url: " + formatYAMLString(url) + "\n")
		}
		if interval := getInt(group, "interval", 0); interval > 0 {
			yaml.WriteString(fmt.Sprintf("    interval: %d\n", interval))
		}
		if lazy, ok := group["lazy"].(bool); ok && lazy {
			yaml.WriteString("    lazy: true\n")
		}
		if timeout := getInt(group, "timeout", 0); timeout > 0 {
			yaml.WriteString(fmt.Sprintf("    timeout: %d\n", timeout))
		}
		if maxFailed := getInt(group, "max-failed-times", 0); maxFailed > 0 {
			yaml.WriteString(fmt.Sprintf("    max-failed-times: %d\n", maxFailed))
		}
		if filter := getString(group, "filter", ""); filter != "" {
			yaml.WriteString("    filter: " + formatYAMLString(filter) + "\n")
		}
		if excludeFilter := getString(group, "exclude-filter", ""); excludeFilter != "" {
			yaml.WriteString("    exclude-filter: " + formatYAMLString(excludeFilter) + "\n")
		}
	}
}

func writeMihomoProxyMap(yaml *strings.Builder, proxy map[string]interface{}) {
	if len(proxy) == 0 {
		return
	}
	keys := getOrderedMihomoProxyKeys(proxy)
	if len(keys) == 0 {
		return
	}
	firstKey := keys[0]
	if value, ok := proxy[firstKey].(string); ok && value != "" {
		yaml.WriteString("  - " + formatYAMLString(firstKey) + ": " + formatYAMLString(value) + "\n")
	} else {
		yaml.WriteString("  -\n")
		writeYAMLField(yaml, firstKey, proxy[firstKey], 2)
	}
	for _, key := range keys[1:] {
		writeYAMLField(yaml, key, proxy[key], 2)
	}
}

func getOrderedMihomoProxyKeys(proxy map[string]interface{}) []string {
	preferred := []string{
		"name", "type", "server", "port", "uuid", "password", "cipher", "flow", "tls",
		"servername", "sni", "udp", "network", "alterId", "encryption", "plugin",
		"plugin-opts", "ws-opts", "grpc-opts", "http-opts", "h2-opts", "httpupgrade-opts",
		"xhttp-opts", "reality-opts", "ech-opts", "skip-cert-verify", "client-fingerprint",
		"fingerprint", "alpn",
	}
	seen := make(map[string]bool, len(proxy))
	keys := make([]string, 0, len(proxy))
	for _, key := range preferred {
		if _, ok := proxy[key]; ok {
			keys = append(keys, key)
			seen[key] = true
		}
	}
	for _, key := range getSortedKeys(proxy) {
		if !seen[key] {
			keys = append(keys, key)
		}
	}
	return keys
}

// isEmptyOrDisabled checks if a config field should be skipped in output.
// Only skips truly empty values, NOT false/0 which are valid config choices.
func isEmptyOrDisabled(key string, value interface{}) bool {
	if value == nil {
		return true
	}
	switch v := value.(type) {
	case string:
		return v == ""
	case []interface{}:
		return len(v) == 0
	case map[string]interface{}:
		return len(v) == 0
	}
	// bool, int, float64 are always valid (false/0 are explicit choices)
	return false
}

// getSortedKeys returns sorted keys from a map for consistent output.
func getSortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// Simple sort
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}

// writeYAMLField writes a single YAML field.
func writeYAMLField(yaml *strings.Builder, key string, value interface{}, indent int) {
	// Skip empty keys
	if key == "" {
		return
	}
	prefix := strings.Repeat("  ", indent)
	quotedKey := formatYAMLString(key)

	switch v := value.(type) {
	case map[string]interface{}:
		if len(v) == 0 {
			return
		}
		yaml.WriteString(prefix + quotedKey + ":\n")
		for _, k := range getSortedKeys(v) {
			writeYAMLField(yaml, k, v[k], indent+1)
		}
	case map[string]string:
		if len(v) == 0 {
			return
		}
		yaml.WriteString(prefix + quotedKey + ":\n")
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		for _, k := range keys {
			writeYAMLField(yaml, k, v[k], indent+1)
		}
	case map[string][]string:
		if len(v) == 0 {
			return
		}
		yaml.WriteString(prefix + quotedKey + ":\n")
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		for _, k := range keys {
			writeYAMLField(yaml, k, v[k], indent+1)
		}
	case []interface{}:
		if len(v) == 0 {
			return
		}
		yaml.WriteString(prefix + quotedKey + ":\n")
		for _, item := range v {
			switch i := item.(type) {
			case map[string]interface{}:
				yaml.WriteString(prefix + "  -\n")
				for _, k := range getSortedKeys(i) {
					writeYAMLField(yaml, k, i[k], indent+2)
				}
			default:
				// Always quote string values in arrays for safety
				if s, ok := i.(string); ok {
					yaml.WriteString(prefix + "  - " + formatYAMLValueQuoted(s) + "\n")
				} else {
					yaml.WriteString(prefix + "  - " + formatYAMLValue(i) + "\n")
				}
			}
		}
	case []string:
		if len(v) == 0 {
			return
		}
		yaml.WriteString(prefix + quotedKey + ":\n")
		for _, item := range v {
			yaml.WriteString(prefix + "  - " + formatYAMLValueQuoted(item) + "\n")
		}
	case string:
		if v != "" {
			yaml.WriteString(prefix + quotedKey + ": " + formatYAMLString(v) + "\n")
		}
	case bool:
		yaml.WriteString(prefix + quotedKey + ": " + fmt.Sprintf("%v", v) + "\n")
	case float64:
		yaml.WriteString(prefix + quotedKey + ": " + fmt.Sprintf("%v", v) + "\n")
	case int:
		yaml.WriteString(prefix + quotedKey + ": " + fmt.Sprintf("%d", v) + "\n")
	case nil:
		// Skip nil values
	}
}

func formatYAMLValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		return formatYAMLString(val)
	case bool:
		return fmt.Sprintf("%v", val)
	case float64:
		return fmt.Sprintf("%v", val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

func formatYAMLString(s string) string {
	if s == "" {
		return "\"\""
	}
	if s == "true" || s == "false" || s == "null" || s == "~" {
		return "\"" + s + "\""
	}
	// Quote if contains special YAML characters or could be ambiguous
	// 控制字符（\n \t \r …）也必须进引号并用 YAML 转义写法，否则会把值折行/写坏：
	// 名称类字段虽然已过 SanitizeNodeName，但密码、SNI、路径等值同样会走这里。
	if strings.ContainsAny(s, ":#{}[]&*!|>'\"@`,?+\\") || strings.ContainsAny(s, "\n\r\t\v\f") ||
		strings.HasPrefix(s, " ") || strings.HasSuffix(s, " ") ||
		strings.HasPrefix(s, "-") || strings.HasPrefix(s, "+") || strings.HasPrefix(s, ".") || strings.HasPrefix(s, "@") {
		escaped := strings.ReplaceAll(s, "\\", "\\\\")
		escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
		escaped = strings.ReplaceAll(escaped, "\n", "\\n")
		escaped = strings.ReplaceAll(escaped, "\r", "\\r")
		escaped = strings.ReplaceAll(escaped, "\t", "\\t")
		return "\"" + escaped + "\""
	}
	if _, err := strconv.ParseFloat(s, 64); err == nil {
		return "\"" + s + "\""
	}
	return s
}

// formatYAMLValueQuoted always wraps string in double quotes
func formatYAMLValueQuoted(s string) string {
	escaped := strings.ReplaceAll(s, "\\", "\\\\")
	escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
	return "\"" + escaped + "\""
}

// cleanDNSConfig removes corrupted nameserver-policy from DNS config
// and converts list-form nameserver-policy (imported from YAML) into a proper mapping.
func cleanDNSConfig(dns interface{}) interface{} {
	d, ok := dns.(map[string]interface{})
	if !ok {
		return dns
	}
	// Ensure all fake-ip-filter entries are quoted strings
	if filter, ok := d["fake-ip-filter"].([]interface{}); ok {
		quoted := make([]interface{}, len(filter))
		for i, f := range filter {
			quoted[i] = fmt.Sprintf("%v", f)
		}
		d["fake-ip-filter"] = quoted
	}
	// Handle nameserver-policy stored as a list of single-key maps (YAML import issue).
	// mihomo requires nameserver-policy to be a mapping, not a sequence.
	if nspList, ok := d["nameserver-policy"].([]interface{}); ok {
		merged := make(map[string]interface{})
		for _, item := range nspList {
			if m, ok := item.(map[string]interface{}); ok {
				for k, v := range m {
					if strings.Contains(k, "\n") {
						continue
					}
					merged[k] = v
				}
			}
		}
		if len(merged) > 0 {
			d["nameserver-policy"] = merged
		} else {
			delete(d, "nameserver-policy")
		}
	} else if nsp, ok := d["nameserver-policy"].(map[string]interface{}); ok {
		// Keep all keys as-is, only remove truly corrupted ones
		cleaned := make(map[string]interface{})
		for k, v := range nsp {
			if strings.Contains(k, "\n") {
				continue
			}
			cleaned[k] = v
		}
		d["nameserver-policy"] = cleaned
	}
	return d
}

// --- helpers ---

func (h *PublicHandler) loadShareDomains(ctx context.Context) []shareDomainEntry {
	var entries []shareDomainEntry

	// Load direct and cf_domain from share_domain_configs (NOT cf_ip, that comes from cf_ip_entries)
	if h.shareDomainRepo != nil {
		domains, err := h.shareDomainRepo.List(ctx, "")
		if err == nil {
			for _, d := range domains {
				if !d.Enabled || d.Value == "" {
					continue
				}
				// Skip cf_ip type - those come from cf_ip_entries table
				if d.Type == "cf_ip" {
					continue
				}
				entry := shareDomainEntry{Type: d.Type, Host: d.Value, Port: d.Port}
				if d.Ports != "" {
					json.Unmarshal([]byte(d.Ports), &entry.Ports)
				}
				if len(entry.Ports) == 0 {
					entry.Ports = []int{entry.Port}
				}
				entries = append(entries, entry)
			}
		}
	}

	// Load cf_ip from cf_ip_entries (separate table)
	if h.cfIPEntryRepo != nil {
		ips, err := h.cfIPEntryRepo.List(ctx, 0)
		if err == nil {
			for _, ip := range ips {
				if ip.IP == "" {
					continue
				}
				entry := shareDomainEntry{Type: "cf_ip", Host: ip.IP, Port: 443}
				if ip.Ports != "" {
					json.Unmarshal([]byte(ip.Ports), &entry.Ports)
				}
				if len(entry.Ports) == 0 {
					entry.Ports = []int{443}
				}
				entries = append(entries, entry)
			}
		}
	}

	return entries
}

func resolveHostPort(config map[string]interface{}, listenHost string, listenPort int, shareDomains []shareDomainEntry) (string, int) {
	if host, port, ok := resolveAccessDomain(config, listenPort, shareDomains); ok {
		return host, port
	}
	return fallbackHost(listenHost), listenPort
}

// resolveAccessDomain 解析「用户在接入域名里**明确选择**的域名/IP」。
//
// ok=false 的三种情况：未启用 / 选中的类型下面那个下拉**没选**（值为空）/ 类型不认识。
// 此时调用方必须回退到节点自身的地址（见 PublicHandler.shareAddress），
// **绝不能再从面板级分享域名列表里猜一个** —— 此前用 pickByType / rand 随机取，
// 同一个入站每次渲染出来的 server 与 port 都不一样，而且与界面上下拉的选择完全无关
// （用户报的 "server 不对 / 下拉对应的没有"）。
func resolveAccessDomain(config map[string]interface{}, listenPort int, shareDomains []shareDomainEntry) (string, int, bool) {
	ad, ok := config["_accessDomain"].(map[string]interface{})
	if !ok {
		return "", 0, false
	}
	if enabled, _ := ad["enabled"].(bool); !enabled {
		return "", 0, false
	}

	adType, _ := ad["type"].(string)
	portMode, _ := ad["directPortMode"].(string)

	switch adType {
	case "direct":
		domain, _ := ad["directDomain"].(string)
		if domain == "" {
			return "", 0, false
		}
		return domain, resolvePort(ad, portMode, listenPort, shareDomains, "direct", domain), true
	case "cf_domain":
		// cf_domain / cf_ip 用的是 CDN 条目自身配置的端口（界面上这两种类型
		// 没有「端口模式」选项），因此显式传空 portMode —— 否则 direct 类型下
		// 残留的 "inbound" 选择会串到 CDN 域名上，链接端口与界面预览不一致。
		domain, _ := ad["cfDomain"].(string)
		if domain == "" {
			return "", 0, false
		}
		return domain, resolvePort(ad, "", listenPort, shareDomains, "cf_domain", domain), true
	case "cf_ip":
		ip, _ := ad["cfIP"].(string)
		if ip == "" {
			return "", 0, false
		}
		return ip, resolvePort(ad, "", listenPort, shareDomains, "cf_ip", ip), true
	case "custom":
		domain, _ := ad["customDomain"].(string)
		if domain == "" {
			return "", 0, false
		}
		// custom 类型自己的「端口模式」：inbound = 用入站监听端口（与界面一致），
		// 否则用自定义端口（默认 443）。此前完全没有读 customPortMode，
		// 导致界面上选了「使用入站端口」也不生效。
		if mode, _ := ad["customPortMode"].(string); mode == "inbound" {
			return domain, listenPort, true
		}
		customPort := 443
		if p, ok := ad["customPort"].(float64); ok && p > 0 {
			customPort = int(p)
		}
		return domain, customPort, true
	}
	return "", 0, false
}

// shareAddress 计算「发给客户端的地址」：
//  1. 接入域名里**已明确选择**的域名/IP（resolveAccessDomain）；
//  2. 否则退回节点自身的公网 / 内网 IP（未启用接入域名的入站本来就走这条）；
//  3. 都没有才用监听地址兜底。
//
// 关键：第 2 步是必需的 —— 启用了接入域名但下拉没选时，以前会从面板级分享域名列表里
// 随机抓一个（server 与界面选择无关、每次渲染还不同）；现在统一退到节点地址，
// 结果稳定且至少是能连的地址。
func (h *PublicHandler) shareAddress(ctx context.Context, cfg entity.InboundConfig, config map[string]interface{}, shareDomains []shareDomainEntry) (string, int) {
	if host, port, ok := resolveAccessDomain(config, cfg.Port, shareDomains); ok {
		return host, port
	}
	if h.nodeServerRepo != nil {
		if node, err := h.nodeServerRepo.GetByID(ctx, cfg.ServerID); err == nil && node != nil {
			if node.PublicIP != "" {
				return node.PublicIP, cfg.Port
			}
			if node.InternalIP != "" {
				return node.InternalIP, cfg.Port
			}
		}
	}
	return fallbackHost(cfg.Listen), cfg.Port
}

func resolvePort(ad map[string]interface{}, portMode string, listenPort int, shareDomains []shareDomainEntry, adType, domain string) int {
	if portMode == "inbound" {
		return listenPort
	}
	for _, d := range shareDomains {
		if d.Host == domain {
			if len(d.Ports) > 0 {
				return d.Ports[rand.Intn(len(d.Ports))]
			}
			return d.Port
		}
	}
	return 443
}

func fallbackHost(listenHost string) string {
	if listenHost == "" || listenHost == "0.0.0.0" || listenHost == "::" {
		return "127.0.0.1"
	}
	return listenHost
}

func getMap(m map[string]interface{}, key string) map[string]interface{} {
	if m == nil {
		return nil
	}
	v, ok := m[key].(map[string]interface{})
	if !ok {
		return nil
	}
	return v
}

func getString(m map[string]interface{}, key, def string) string {
	if m == nil {
		return def
	}
	v, ok := m[key].(string)
	if !ok || v == "" {
		return def
	}
	return v
}

func getInt(m map[string]interface{}, key string, def int) int {
	if m == nil {
		return def
	}
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return def
}

func getBool(m map[string]interface{}, key string) bool {
	if m == nil {
		return false
	}
	v, _ := m[key].(bool)
	return v
}

func getBoolDef(m map[string]interface{}, key string, def bool) bool {
	if m == nil {
		return def
	}
	v, ok := m[key].(bool)
	if !ok {
		return def
	}
	return v
}

func getStringSlice(m map[string]interface{}, key string) []string {
	if m == nil {
		return nil
	}
	// 同时支持 []string（面板内部构造的视图）与 []interface{}（JSON 反序列化的 xray 配置），
	// 只认后者会让前者静默丢空（REALITY 链接缺 sni/sid、alpn 丢失等）。
	switch v := m[key].(type) {
	case []string:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if item != "" {
				out = append(out, item)
			}
		}
		return out
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// findClientByEmail finds a specific client entry in the inbound config by matching the email pattern.
// The email pattern is: {client_email}_{tag_prefix}
func findClientByEmail(settings map[string]interface{}, clientEmail, tag string) map[string]interface{} {
	if settings == nil {
		return nil
	}
	// Generate the expected inbound email (same logic as service/client/inbound_manager.go)
	inboundEmail := generateInboundEmail(clientEmail, tag)

	for _, key := range []string{"clients", "users"} {
		arr, ok := settings[key].([]interface{})
		if !ok {
			continue
		}
		for _, u := range arr {
			user, ok := u.(map[string]interface{})
			if !ok {
				continue
			}
			if user["email"] == inboundEmail {
				return user
			}
		}
	}
	return nil
}

// generateInboundEmail creates a unique email per inbound (mirrors service/client/inbound_manager.go).
func generateInboundEmail(baseEmail, tag string) string {
	parts := strings.Split(tag, "-")
	if len(parts) >= 3 {
		return fmt.Sprintf("%s_%s%s", baseEmail, parts[1], parts[len(parts)-1])
	}
	return fmt.Sprintf("%s_%s", baseEmail, tag)
}

func getSettingsClients(config map[string]interface{}) []map[string]interface{} {
	settings := getMap(config, "settings")
	if settings == nil {
		return nil
	}
	clients, ok := settings["clients"].([]interface{})
	if !ok {
		return nil
	}
	var result []map[string]interface{}
	for _, c := range clients {
		if m, ok := c.(map[string]interface{}); ok {
			result = append(result, m)
		}
	}
	return result
}

func convertLinksToMihomoProxyMaps(links []string) []map[string]interface{} {
	if len(links) == 0 {
		return nil
	}
	proxyMaps, err := mihomoconv.ConvertsV2Ray([]byte(strings.Join(links, "\n")))
	if err != nil || len(proxyMaps) == 0 {
		return nil
	}
	result := make([]map[string]interface{}, 0, len(proxyMaps))
	for idx, pm := range proxyMaps {
		if len(pm) == 0 {
			continue
		}
		if idx < len(links) {
			normalizeSubscriptionMihomoProxyMap(links[idx], pm)
		}
		result = append(result, pm)
	}
	return result
}

func buildRenderedMihomoProxyMaps(items []renderedSubscriptionNode) []map[string]interface{} {
	if len(items) == 0 {
		return nil
	}
	result := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		switch {
		case item.ProxyMap != nil:
			pm := cloneProxyMap(item.ProxyMap)
			if len(pm) == 0 {
				continue
			}
			normalizeSubscriptionMihomoProxyMap(item.Link, pm)
			result = append(result, pm)
		case item.Link != "":
			converted := convertLinksToMihomoProxyMaps([]string{item.Link})
			if len(converted) == 0 {
				continue
			}
			result = append(result, converted[0])
		}
	}
	return dedupeMihomoProxyNames(result)
}

// dedupeMihomoProxyNames 保证代理名（mihomo 的 `name`，即"标签"）唯一。
//
// mihomo 对重名是**整份拒绝**（实测 1.10：`proxy trojan-worker is the duplicate name` →
// `configuration file xxx test failed`），不是跳过该条 —— 于是用户导入订阅后"很多节点不显示"
// 甚至一个都没有。重名来源很常见：内部入站重名、外部节点与订阅节点重名、
// 同一节点既直连又走镜像/多来源。
//
// 命名口径与 sing-box / xray 的出站 tag 完全一致（formatter.uniqueNodeTag）：
// 重名时追加 " 2"、" 3"…，这样三种格式里同一个节点名字相同，用户对照时不会困惑。
func dedupeMihomoProxyNames(proxyMaps []map[string]interface{}) []map[string]interface{} {
	used := make(map[string]int, len(proxyMaps))
	for _, pm := range proxyMaps {
		if pm == nil {
			continue
		}
		name, _ := pm["name"].(string)
		pm["name"] = uniqueMihomoName(name, used)
	}
	return proxyMaps
}

// uniqueMihomoName 与 formatter.uniqueNodeTag 同语义（那边未导出，故在此重复一份）。
// 注意：生成出来的新名字也要登记，否则"已有节点本身就叫 X 2"时会与新生成的名字再撞一次。
func uniqueMihomoName(name string, used map[string]int) string {
	// 与 sing-box / xray 的 tag 用同一套清洗（中文保留，控制字符/超长处理掉）
	name = formatter.SanitizeNodeName(name)
	if used[name] == 0 {
		used[name] = 1
		return name
	}
	for n := used[name] + 1; ; n++ {
		candidate := fmt.Sprintf("%s %d", name, n)
		if used[candidate] == 0 {
			used[name] = n
			used[candidate] = 1
			return candidate
		}
	}
}

func parseSubscriptionLinksToFormatterNodes(links []string) []formatter.Node {
	if len(links) == 0 {
		return nil
	}
	nodes := make([]formatter.Node, 0, len(links))
	for _, link := range links {
		node := formatter.ParseURIToNode(link)
		if node == nil {
			continue
		}
		nodes = append(nodes, *node)
	}
	return nodes
}

func normalizeSubscriptionMihomoProxyMap(link string, proxy map[string]interface{}) {
	if len(proxy) == 0 {
		return
	}
	trimEmptyALPN(proxy)

	// mihomo 的 network 取值里**没有** "tcp"/"raw"（裸 TCP 就该省略这个字段）：
	// 外部节点带过来的 ProxyMap 可能写着 `network: raw`/`network: tcp`，
	// 这里直接删掉，避免 YAML 里出现客户端不认识的值。
	if v, ok := proxy["network"].(string); ok {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "tcp", "raw":
			delete(proxy, "network")
		}
	}

	// mihomo **没有** `network: httpupgrade`：它的 vless/vmess 运行时 switch 只认 "ws"，
	// HTTP Upgrade 是 ws 上的开关（ws-opts.v2ray-http-upgrade，见 mihomo docs/config.yaml 与
	// adapter/outbound/vless.go 的 V2rayHttpUpgrade）。写 network: httpupgrade 会被当未知值
	// → 客户端静默回退成裸 TCP → 连不上（e2e 矩阵实测：mihomo 报 connect error，xray/sing-box 正常）。
	// 这里统一改写（链接转换 / 外部节点 ProxyMap 两条路都会经过），并保证 ws-opts 存在。
	if v, ok := proxy["network"].(string); ok && strings.EqualFold(strings.TrimSpace(v), "httpupgrade") {
		proxy["network"] = "ws"
		wsOpts, _ := proxy["ws-opts"].(map[string]interface{})
		if wsOpts == nil {
			wsOpts = map[string]interface{}{}
			proxy["ws-opts"] = wsOpts
		}
		wsOpts["v2ray-http-upgrade"] = true
	}

	proxyType, _ := proxy["type"].(string)
	switch proxyType {
	case "vmess":
		normalizeVMessSubscriptionProxyMap(link, proxy)
	}
}

func cloneProxyMap(src map[string]interface{}) map[string]interface{} {
	if len(src) == 0 {
		return nil
	}
	raw, err := json.Marshal(src)
	if err != nil {
		return nil
	}
	var dst map[string]interface{}
	if err := json.Unmarshal(raw, &dst); err != nil {
		return nil
	}
	return dst
}

// deepCopyMap creates a deep copy of a map[string]interface{} via JSON round-trip.
// Used for cloning inbound config before modifying _accessDomain for mirror nodes.
func deepCopyMap(src map[string]interface{}) map[string]interface{} {
	return cloneProxyMap(src)
}

// replaceAccessDomainForMirror replaces the host/IP in _accessDomain with the
// mirror node's own address. The mirror node uses the same protocol/port/
// credentials as the primary, but users connect to the mirror's IP/domain.
func replaceAccessDomainForMirror(config map[string]interface{}, mirror entity.NodeServer) {
	ad, ok := config["_accessDomain"].(map[string]interface{})
	if !ok {
		ad = map[string]interface{}{}
		config["_accessDomain"] = ad
	}

	// Determine the mirror's address
	mirrorHost := mirror.PublicIP
	if mirrorHost == "" {
		mirrorHost = mirror.Hostname
	}
	if mirrorHost == "" {
		return
	}

	adType, _ := ad["type"].(string)
	switch adType {
	case "direct":
		ad["directDomain"] = mirrorHost
	case "cf_domain":
		// For CDN-type, use the mirror's IP (may point to different CDN edge)
		ad["cfIP"] = mirrorHost
	case "cf_ip":
		ad["cfIP"] = mirrorHost
	case "custom":
		ad["customDomain"] = mirrorHost
	}
}

func normalizeVMessSubscriptionProxyMap(link string, proxy map[string]interface{}) {
	// Mihomo's converter adds xudp for vmess, but this field is not part of the
	// original Clash-style proxy object we want to preserve for subscription output.
	delete(proxy, "xudp")

	if !strings.HasPrefix(link, "vmess://") {
		return
	}
	payload := strings.TrimPrefix(link, "vmess://")
	raw, err := decodeVmessPayload(payload)
	if err != nil {
		return
	}

	var values map[string]interface{}
	if err := json.Unmarshal(raw, &values); err != nil {
		return
	}

	network, _ := values["net"].(string)
	network = strings.ToLower(network)
	if network != "ws" && network != "httpupgrade" {
		return
	}

	path, _ := values["path"].(string)
	if path == "" {
		return
	}

	wsOpts, _ := proxy["ws-opts"].(map[string]interface{})
	if wsOpts == nil {
		wsOpts = make(map[string]interface{})
		proxy["ws-opts"] = wsOpts
	}
	wsOpts["path"] = path
}

func trimEmptyALPN(proxy map[string]interface{}) {
	alpn, ok := proxy["alpn"]
	if !ok {
		return
	}

	switch values := alpn.(type) {
	case []string:
		for _, value := range values {
			if strings.TrimSpace(value) != "" {
				return
			}
		}
		delete(proxy, "alpn")
	case []interface{}:
		for _, value := range values {
			if s, ok := value.(string); !ok || strings.TrimSpace(s) != "" {
				return
			}
		}
		delete(proxy, "alpn")
	}
}

// mihomoProxyMapToNode converts a mihomo proxy map to formatter.Node.
// Returns nil for unsupported transports (kcp, mkcp, quic, etc.).
func mihomoProxyMapToNode(p map[string]interface{}) *formatter.Node {
	name, _ := p["name"].(string)
	proto, _ := p["type"].(string)
	proto = normalizeMihomoProtocol(proto)
	server, _ := p["server"].(string)

	// Filter unsupported transports for mihomo
	transport := "tcp"
	if v, ok := p["network"].(string); ok && v != "" {
		transport = v
	}
	unsupportedTransports := map[string]bool{"kcp": true, "mkcp": true, "quic": true}
	if unsupportedTransports[transport] {
		return nil
	}
	port := 0
	if v, ok := p["port"].(string); ok {
		fmt.Sscanf(v, "%d", &port)
	} else if v, ok := p["port"].(float64); ok {
		port = int(v)
	}
	uuid, _ := p["uuid"].(string)
	password, _ := p["password"].(string)
	// snell 的凭据字段是 psk（不是 password），不取就永远认不上
	if password == "" {
		password, _ = p["psk"].(string)
	}
	flow, _ := p["flow"].(string)
	encryption, _ := p["encryption"].(string)

	tls := false
	if v, ok := p["tls"].(bool); ok {
		tls = v
	}
	// mihomo 的这几个类型选项里**没有 tls 字段**（协议本身就恒为 TLS），
	// 不补 tls=true 的话 sni/证书校验等参数在后续渲染里会全部丢掉。
	switch proto {
	case "trojan", "hysteria", "hysteria2", "tuic", "anytls":
		tls = true
	}
	sni := ""
	if v, ok := p["servername"].(string); ok && v != "" {
		sni = v
	} else if v, ok := p["sni"].(string); ok && v != "" {
		sni = v
	}

	// Build Extra from all mihomo proxy fields
	extra := make(map[string]interface{})
	copyKeys := []string{
		"alterId", "cipher", "alpn", "fingerprint", "client-fingerprint",
		"skip-cert-verify", "tfo", "mptcp", "routing-mark", "udp", "xudp",
		"reality-opts", "ech-opts", "support-x25519mlkem768",
		"ws-opts", "ws-headers", "grpc-opts", "xhttp-opts", "h2-opts", "http-opts",
		"obfs", "obfs-password", "up", "down", "ports", "hop-interval",
		"bbr-profile", "max-idle-timeout", "keep-alive-period",
		"disable-path-mtu-discovery", "max-incoming-streams",
		"initial-stream-receive-window", "max-stream-receive-window",
		"initial-connection-receive-window", "max-connection-receive-window",
		"realm-opts",
		"packet-addr", "packet-encoding",
		"plugin", "plugin-opts", "udp-over-tcp", "udp-over-tcp-version",
		// TUIC specific
		"token", "congestion-controller", "disable-sni", "udp-relay-mode",
		// Hysteria v1 specific
		"auth_str", "protocol", "auth-str",
		// Socks5/HTTP specific
		"username",
		// SS specific
		"obfs-min-packet-size", "obfs-max-packet-size",
		// Snell specific（psk 是凭据、version 出站必填）
		"psk", "version", "reuse",
		// AnyTLS specific
		"idle-session-check-interval", "idle-session-timeout", "min-idle-session",
		"disable-reuse", "client-metadata",
		// SSH specific
		"private-key", "private-key-passphrase", "host-key", "host-key-algorithms",
		// Hysteria2 / TUIC 扩展
		"cwnd", "udp-mtu", "handshake-timeout", "reduce-rtt", "heartbeat-interval",
		"max-open-streams", "udp-over-stream", "udp-over-stream-version",
		"max-udp-relay-packet-size", "max-datagram-frame-size",
		"recv-window", "recv-window-conn", "fast-open",
		// 安全包装（mihomo / sing-box 都认）
		"shadow-tls-opts", "restls-opts", "jls-opts", "tlsmirror-opts",
	}
	for _, k := range copyKeys {
		if v, ok := p[k]; ok {
			extra[k] = v
		}
	}
	if len(extra) == 0 {
		extra = nil
	}

	return &formatter.Node{
		Name:       name,
		Protocol:   proto,
		Host:       server,
		Port:       port,
		Transport:  transport,
		TLS:        tls,
		ServerName: sni,
		UUID:       uuid,
		Password:   password,
		Flow:       flow,
		Encryption: encryption,
		Extra:      extra,
	}
}

// normalizeMihomoProtocol 把 mihomo 的代理类型名归一成面板 / 其它核心通用的名字。
// mihomo 与 sing-box 的类型名并不一致（ss / shadowsocks、socks5 / socks、hy2 / hysteria2），
// 原样透传会让外部订阅里的节点在 sing-box 订阅里被当成未知协议整条丢掉。
func normalizeMihomoProtocol(t string) string {
	switch strings.ToLower(t) {
	case "ss":
		return "shadowsocks"
	case "ssr":
		return "shadowsocksr"
	case "socks5", "socks5h":
		return "socks"
	case "https":
		return "http"
	case "hy2":
		return "hysteria2"
	case "any-tls":
		return "anytls"
	}
	return t
}

func getTLSServerName(ss map[string]interface{}) string {
	if ss == nil {
		return ""
	}
	// Try tlsSettings.serverName
	if tls := getMap(ss, "tlsSettings"); tls != nil {
		if sni, ok := tls["serverName"].(string); ok && sni != "" {
			return sni
		}
	}
	// Try realitySettings.serverNames[0]
	if reality := getMap(ss, "realitySettings"); reality != nil {
		if sniList, ok := reality["serverNames"].([]interface{}); ok && len(sniList) > 0 {
			if s, ok := sniList[0].(string); ok {
				return s
			}
		}
	}
	return ""
}

// buildExtra extracts all mihomo-relevant fields from xray config into Extra map.
// Only non-empty values are stored. Keys use mihomo YAML field names.
// normalizeNodeTransport **只做命名归一，绝不降级**：
//   - 空 → tcp（老数据没写 network）；
//   - mkcp → kcp（面板内部/编辑器用 "mkcp"，所有客户端格式里的名字都是 "kcp"）；
//   - 其余原样保留 —— 特别是 **raw 保持 raw**（xray 25+ 的正式名；用户明确要求
//     xray 的 JSON 里不要换成 tcp）。各格式自己决定怎么呈现：
//     xray 写 raw + rawSettings；mihomo / sing-box 的 network/transport 里没有
//     raw/tcp 这个取值，裸 TCP 就不写这个字段（由各 formatter 负责省略）。
//
// **绝不能**在这里把"表达不了的传输"（kcp / quic / splithttp …）改成 tcp：
// 那会漏出一个"传输参数全丢、但看起来能用"的节点（客户端按裸 TCP 连 → 连不上）。
// 该由各格式自己的支持性判断整条剔除：
// singboxNodeDropReason / mihomoNodeDropReason / filterRenderedNodesForFormat。
func normalizeNodeTransport(network string) string {
	switch strings.ToLower(strings.TrimSpace(network)) {
	case "":
		return "tcp"
	case "mkcp":
		return "kcp"
	default:
		return strings.ToLower(strings.TrimSpace(network))
	}
}

func buildExtra(config, ss map[string]interface{}, protocol string) map[string]interface{} {
	extra := make(map[string]interface{})

	// AlterID (vmess)
	if settings := getMap(config, "settings"); settings != nil {
		for _, key := range []string{"clients", "users"} {
			if arr, ok := settings[key].([]interface{}); ok && len(arr) > 0 {
				if c, ok := arr[0].(map[string]interface{}); ok {
					if aid, ok := c["alterId"].(float64); ok && aid > 0 {
						extra["alterId"] = int(aid)
					}
					// VMess cipher
					if cipher, ok := c["security"].(string); ok && cipher != "" {
						extra["cipher"] = cipher
					}
				}
			}
		}
	}

	// TLS settings
	security := getString(ss, "security", "none")
	if security == "tls" {
		tls := getMap(ss, "tlsSettings")
		if tls != nil {
			if alpn := getStringSlice(tls, "alpn"); len(alpn) > 0 {
				extra["alpn"] = alpn
			}
			if fp, ok := tls["fingerprint"].(string); ok && fp != "" {
				extra["client-fingerprint"] = fp
			}
			extra["skip-cert-verify"] = getBool(tls, "allowInsecure")
		}
	} else if security == "reality" {
		// 统一取参（兼容 publicKey/shortId 扁平写法与 xray 25+ 的 settings[0].password 写法）。
		// 以前只认扁平字段，面板编辑器存的是 settings[] 形态 → REALITY 参数**全丢**，
		// sing-box 片段退化成 tls.enabled=true、xray 片段退化成 security=tls（普通 TLS 握手），
		// 客户端永远连不上（用户报的 "json 完全生成错误"）。
		if reality := getMap(ss, "realitySettings"); reality != nil {
			rp := sharelink.RealityClientParams(ss)
			ro := make(map[string]interface{})
			if rp.PublicKey != "" {
				ro["public-key"] = rp.PublicKey
			}
			if rp.ShortID != "" {
				ro["short-id"] = rp.ShortID
			}
			if rp.SpiderX != "" {
				ro["spider-x"] = rp.SpiderX
			}
			// 抗量子校验：mihomo / sing-box 都没有这个字段（xray 有），
			// 由 XrayFormatter 单独取用，不影响其它格式。
			if rp.MLDSA65Verify != "" {
				ro["mldsa65-verify"] = rp.MLDSA65Verify
			}
			if len(ro) > 0 {
				extra["reality-opts"] = ro
				// 供 sing-box 格式剔除判定：xray ≥26.9.8 的 REALITY 服务端要求 ClientHello 带
				// X25519MLKEM768 key share，而 sing-box（含官方构建）的 uTLS chrome 指纹不发它、
				// 也无法配置绕过（上游 issue SagerNet/sing-box#4520）→ 发给 sing-box 用户就是死节点。
				// 这里按**配置形状**区分服务端核心：sing-box 入站是 `type` 形状，xray 是 `protocol` 形状。
				if _, isSingboxInbound := config["type"].(string); !isSingboxInbound {
					extra["_xray-reality"] = true
				}
			}
			if rp.Fingerprint != "" {
				extra["client-fingerprint"] = rp.Fingerprint
			}
		}
	}

	// Transport sub-options (only mihomo-supported transports)
	network := getString(ss, "network", "raw")
	supportedTransports := map[string]bool{"tcp": true, "raw": true, "ws": true, "grpc": true, "xhttp": true, "h2": true, "http": true, "httpupgrade": true}
	if !supportedTransports[network] {
		// Unsupported transport (kcp, mkcp, quic, etc.) - skip transport sub-options
		network = ""
	}
	// TCP with HTTP header disguise → treat as http transport for mihomo
	if network == "tcp" || network == "raw" {
		rawSettings := getMap(ss, "rawSettings")
		if rawSettings == nil {
			rawSettings = getMap(ss, "tcpSettings")
		}
		if header := getMap(rawSettings, "header"); header != nil && getString(header, "type", "") == "http" {
			network = "http"
			if opts := extractHTTPDisguiseOpts(header); len(opts) > 0 {
				extra["http-opts"] = opts
			}
		}
		network = "tcp"
	}

	// mKCP：xray 表达得了（mihomo 表达不了，会在片段层被整条剔除）。
	// XrayFormatter 从 kcp-settings 读 mtu/tti/... 并原样带上 finalmask（Xray 25+ 的
	// mkcp-legacy header/seed 就在 finalmask.udp 里）——缺了这段，xray 片段就会变成
	// 一个参数全丢的 network: tcp 节点。
	if rawNetwork := strings.ToLower(getString(ss, "network", "raw")); rawNetwork == "kcp" || rawNetwork == "mkcp" {
		if kcp := getMap(ss, "kcpSettings"); kcp != nil {
			ks := map[string]interface{}{}
			for _, key := range []string{"mtu", "tti", "uplinkCapacity", "downlinkCapacity", "congestion", "readBufferSize", "writeBufferSize"} {
				if v, ok := kcp[key]; ok {
					ks[key] = v
				}
			}
			if len(ks) > 0 {
				extra["kcp-settings"] = ks
			}
		}
		if fm := getMap(ss, "finalmask"); fm != nil {
			extra["finalmask"] = fm
		}
	}

	switch network {
	case "ws":
		ws := getMap(ss, "wsSettings")
		if ws != nil {
			opts := make(map[string]interface{})
			if path, ok := ws["path"].(string); ok && path != "" {
				opts["path"] = path
				// Detect early data in path (?ed=2560), set max-early-data to 4096
				if idx := strings.Index(path, "?ed="); idx >= 0 {
					edStr := path[idx+4:]
					if ampIdx := strings.Index(edStr, "&"); ampIdx >= 0 {
						edStr = edStr[:ampIdx]
					}
					if _, err := strconv.Atoi(edStr); err == nil {
						opts["max-early-data"] = 4096
						opts["early-data-header-name"] = "Sec-WebSocket-Protocol"
					}
				}
			}
			if headers := getMap(ws, "headers"); headers != nil {
				opts["headers"] = headers
			}
			if ed, ok := ws["maxEarlyData"].(float64); ok && ed > 0 {
				if _, exists := opts["max-early-data"]; !exists {
					opts["max-early-data"] = int(ed)
				}
			}
			if edhp, ok := ws["earlyDataHeaderName"].(string); ok && edhp != "" {
				opts["early-data-header-name"] = edhp
			}
			if len(opts) > 0 {
				extra["ws-opts"] = opts
			}
		}
		// VLESS ws-headers (top-level in streamSettings)
		if wsHeaders := getMap(ss, "wsHeaders"); wsHeaders != nil {
			extra["ws-headers"] = wsHeaders
		}
	case "grpc":
		grpc := getMap(ss, "grpcSettings")
		if grpc != nil {
			opts := make(map[string]interface{})
			if sn, ok := grpc["serviceName"].(string); ok && sn != "" {
				opts["grpc-service-name"] = sn
			}
			if mm, ok := grpc["multiMode"].(bool); ok && mm {
				// grpc-mode not supported by mihomo
			}
			if len(opts) > 0 {
				extra["grpc-opts"] = opts
			}
		}
	case "xhttp":
		xhttp := getMap(ss, "xhttpSettings")
		if xhttp != nil {
			opts := make(map[string]interface{})
			if mode, ok := xhttp["mode"].(string); ok && mode != "" {
				opts["mode"] = mode
			}
			if path, ok := xhttp["path"].(string); ok && path != "" {
				opts["path"] = path
			}
			if host, ok := xhttp["host"].(string); ok && host != "" {
				opts["host"] = host
			}
			if headers := getMap(xhttp, "headers"); headers != nil {
				opts["headers"] = headers
			}
			if len(opts) > 0 {
				extra["xhttp-opts"] = opts
			}
		}
	case "httpupgrade":
		hu := getMap(ss, "httpupgradeSettings")
		if hu != nil {
			opts := make(map[string]interface{})
			if path, ok := hu["path"].(string); ok && path != "" {
				opts["path"] = path
				// Detect early data in path (?ed=2560)
				if idx := strings.Index(path, "?ed="); idx >= 0 {
					edStr := path[idx+4:]
					if ampIdx := strings.Index(edStr, "&"); ampIdx >= 0 {
						edStr = edStr[:ampIdx]
					}
					if _, err := strconv.Atoi(edStr); err == nil {
						opts["v2ray-http-upgrade-fast-open"] = true
					}
				}
			}
			if host, ok := hu["host"].(string); ok && host != "" {
				opts["host"] = host
			}
			if len(opts) > 0 {
				extra["httpupgrade-opts"] = opts
			}
		}
	case "h2", "http":
		h2 := getMap(ss, "h2Settings")
		if h2 == nil {
			h2 = getMap(ss, "httpSettings")
		}
		if h2 != nil {
			opts := make(map[string]interface{})
			if path, ok := h2["path"].(string); ok && path != "" {
				opts["path"] = path
			}
			if host, ok := h2["host"].([]interface{}); ok && len(host) > 0 {
				var hosts []string
				for _, h := range host {
					if s, ok := h.(string); ok {
						hosts = append(hosts, s)
					}
				}
				if len(hosts) > 0 {
					opts["host"] = hosts
				}
			}
			if len(opts) > 0 {
				if network == "h2" {
					extra["h2-opts"] = opts
				} else {
					extra["http-opts"] = opts
				}
			}
		}
	}

	// Hysteria2：三种订阅格式（Mihomo / xray / sing-box）都从 Extra 取参数，
	// 而 hy2 的参数在两种核心里放的位置完全不同：
	//   sing-box 放在**顶层**（obfs{type,password} / up_mbps / down_mbps / bbr_profile）；
	//   xray 放在 streamSettings.hysteriaSettings + finalmask（obfs / up / down / quicParams）。
	// 这里统一归一到 mihomo 风格的扁平键（obfs / obfs-password / up / down / ports /
	// hop-interval / bbr-profile / QUIC 窗口…）。缺了这段，服务端开了混淆、指定了 BBR
	// 或开了动态端口时，订阅里拿到的就是"能连但参数不全"的裸节点（分享链接却正常）。
	if protocol == "hysteria" || protocol == "hysteria2" {
		hy := getMap(ss, "hysteriaSettings")
		fm := finalMaskWithPortHop(config) // 已把 _portHop 合成到 quicParams.udpHop
		var qp map[string]interface{}
		if fm != nil {
			extra["finalmask"] = fm
			qp = getMap(fm, "quicParams")
		}

		obfsType := getString(hy, "obfs", "")
		obfsPassword := getString(hy, "obfs_password", "")
		if obfsPassword == "" {
			obfsPassword = getString(hy, "obfsPassword", "")
		}
		if fm != nil {
			if udp, ok := fm["udp"].([]interface{}); ok {
				for _, item := range udp {
					entry, ok := item.(map[string]interface{})
					if !ok || entry["type"] != "salamander" {
						continue
					}
					if obfsType == "" {
						obfsType = "salamander"
					}
					if obfsPassword == "" {
						if st := getMap(entry, "settings"); st != nil {
							obfsPassword = getString(st, "password", "")
						}
					}
				}
			}
		}
		if obfsType != "" {
			extra["obfs"] = obfsType
		}
		if obfsPassword != "" {
			extra["obfs-password"] = obfsPassword
		}

		// 带宽：xray 写 hysteriaSettings.up/down，sing-box 写 up_mbps/down_mbps，
		// brutal 拥塞时又只写在 finalmask.quicParams.brutalUp/Down。
		if up := hysteriaBandwidth(hy, qp, "up", "up_mbps", "brutalUp"); up != "" {
			extra["up"] = up
		}
		if down := hysteriaBandwidth(hy, qp, "down", "down_mbps", "brutalDown"); down != "" {
			extra["down"] = down
		}

		// 动态端口 / 跳跃间隔（xray: hopPorts/hopInterval；finalmask: quicParams.udpHop）
		ports := getString(hy, "hopPorts", "")
		interval := valueToText(hy["hopInterval"])
		if qp != nil {
			if hop := getMap(qp, "udpHop"); hop != nil {
				if ports == "" {
					ports = getString(hop, "ports", "")
				}
				if interval == "" {
					interval = valueToText(hop["interval"])
				}
			}
		}
		if ports != "" {
			extra["ports"] = ports
		}
		if interval != "" {
			extra["hop-interval"] = interval
		}

		// BBR 拥塞控制：xray 在 finalmask.quicParams.bbrProfile，sing-box 在顶层 bbr_profile
		if bbr := getString(hy, "bbr_profile", ""); bbr != "" {
			extra["bbr-profile"] = bbr
		} else if qp != nil {
			if bbr := getString(qp, "bbrProfile", ""); bbr != "" && bbr != "standard" {
				extra["bbr-profile"] = bbr
			}
		}

		// QUIC 窗口等：有值就下发（mihomo / sing-box 的字段名在这些键上是一致的）
		if qp != nil {
			for _, pair := range [][2]string{
				{"initStreamReceiveWindow", "initial-stream-receive-window"},
				{"maxStreamReceiveWindow", "max-stream-receive-window"},
				{"initConnectionReceiveWindow", "initial-connection-receive-window"},
				{"maxConnectionReceiveWindow", "max-connection-receive-window"},
				{"maxIdleTimeout", "max-idle-timeout"},
				{"keepAlivePeriod", "keep-alive-period"},
				{"maxIncomingStreams", "max-incoming-streams"},
			} {
				if v := getInt(qp, pair[0], 0); v > 0 {
					extra[pair[1]] = v
				}
			}
			if v, ok := qp["disablePathMTUDiscovery"].(bool); ok && v {
				extra["disable-path-mtu-discovery"] = true
			}
		}
	}

	// 部分 sing-box 专属协议把参数放在**顶层**（不走 transport / tls），这里按协议透传：
	// 少了它，订阅里的该节点要么缺必填字段（snell 的 version），要么丢可选参数。
	switch protocol {
	case "snell":
		if v := getInt(config, "version", 0); v > 0 {
			extra["version"] = v
		}
	case "tuic":
		if cc := getString(config, "congestion_control", ""); cc != "" {
			extra["congestion-controller"] = cc
		}
		if m := getString(config, "udp_relay_mode", ""); m != "" {
			extra["udp-relay-mode"] = m
		}
		if z, _ := config["zero_rtt_handshake"].(bool); z {
			extra["zero-rtt-handshake"] = true
		}
	case "anytls":
		if v := getString(config, "idle_session_check_interval", ""); v != "" {
			extra["idle-session-check-interval"] = v
		}
		if v := getString(config, "idle_session_timeout", ""); v != "" {
			extra["idle-session-timeout"] = v
		}
		if v := getInt(config, "min_idle_session", 0); v > 0 {
			extra["min-idle-session"] = v
		}
	}

	// Sockopt
	if sockopt := getMap(ss, "sockopt"); sockopt != nil {
		if tfo, ok := sockopt["tcpFastOpen"].(bool); ok && tfo {
			extra["tfo"] = true
		}
		if mark, ok := sockopt["mark"].(float64); ok && mark > 0 {
			extra["routing-mark"] = int(mark)
		}
	}

	// UDP (always true for subscription nodes)
	extra["udp"] = true

	if len(extra) == 0 {
		return nil
	}
	return extra
}

// hysteriaBandwidth 依次从 hysteriaSettings 的两个键、再从 finalmask.quicParams
// 的 brutal 键里取带宽，统一成 "200 Mbps" 文本；都没有返回空串。
func hysteriaBandwidth(hy, qp map[string]interface{}, directKey, mbpsKey, brutalKey string) string {
	for _, key := range []string{directKey, mbpsKey} {
		if v, ok := hy[key]; ok && v != nil {
			if s := bandwidthText(v); s != "" {
				return s
			}
		}
	}
	// finalmask 里的 brutalUp/brutalDown 只在 brutal 拥塞下才有效：用在 BBR 上会让
	// sing-box 客户端反过来请求 brutal，与服务端设定的 bbr_profile 冲突
	//（mihomoconv 也是这个口径）。
	if qp != nil && isBrutalCongestion(qp) {
		if v, ok := qp[brutalKey]; ok && v != nil {
			return bandwidthText(v)
		}
	}
	return ""
}

// isBrutalCongestion 判断 quicParams.congestion 是否为 brutal / force-brutal。
func isBrutalCongestion(qp map[string]interface{}) bool {
	cc := strings.ToLower(getString(qp, "congestion", ""))
	return cc == "brutal" || cc == "force-brutal"
}

// bandwidthText 把带宽值统一成 mihomo / sing-box 都认的 "200 Mbps" 文本。
func bandwidthText(v interface{}) string {
	switch t := v.(type) {
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return ""
		}
		if _, err := strconv.ParseFloat(s, 64); err == nil {
			return s + " Mbps"
		}
		return s
	case float64:
		if t <= 0 {
			return ""
		}
		return strconv.FormatFloat(t, 'f', -1, 64) + " Mbps"
	case int:
		if t <= 0 {
			return ""
		}
		return strconv.Itoa(t) + " Mbps"
	}
	return ""
}

// valueToText 把标量值转成文本（nil / 空串返回空），用于 hopInterval 这类可能是
// 数字也可能是字符串的字段。
func valueToText(v interface{}) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	}
	return ""
}

func extractHTTPDisguiseOpts(header map[string]interface{}) map[string]interface{} {
	request := getMap(header, "request")
	if request == nil {
		return nil
	}

	opts := make(map[string]interface{})
	if paths := normalizeStringList(request["path"]); len(paths) > 0 {
		opts["path"] = paths
	}
	if headers := getMap(request, "headers"); headers != nil {
		normalizedHeaders := make(map[string]interface{})
		for key, value := range headers {
			if items := normalizeStringList(value); len(items) > 0 {
				normalizedHeaders[key] = items
			}
		}
		if len(normalizedHeaders) > 0 {
			opts["headers"] = normalizedHeaders
		}
	}
	if len(opts) == 0 {
		return nil
	}
	return opts
}

func normalizeStringList(value interface{}) []string {
	switch v := value.(type) {
	case string:
		if v == "" {
			return nil
		}
		return []string{v}
	case []string:
		items := make([]string, 0, len(v))
		for _, item := range v {
			if item != "" {
				items = append(items, item)
			}
		}
		return items
	case []interface{}:
		items := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				items = append(items, s)
			}
		}
		return items
	default:
		return nil
	}
}

func getFinalMask(config map[string]interface{}) map[string]interface{} {
	// Try top-level finalmask
	if fm := getMap(config, "finalmask"); fm != nil {
		return fm
	}
	// Try streamSettings.finalmask
	if ss := getMap(config, "streamSettings"); ss != nil {
		return getMap(ss, "finalmask")
	}
	return nil
}

func (h *PublicHandler) recordAccess(subID int64, ip, ua, realIP, cdnIP, cdnProvider string) {
	if h.accessLog == nil {
		return
	}
	logEntry := &entity.SubscriptionAccessLog{
		SubscriptionID: subID,
		IP:             ip,
		UserAgent:      ua,
		RealIP:         realIP,
		CDNIP:          cdnIP,
		CDNProvider:    cdnProvider,
		StatusCode:     200,
		HitResult:      "hit",
	}
	if err := h.accessLog.Create(context.Background(), logEntry); err != nil {
		log.Printf("sub: failed to record access: %v", err)
	}
}

// pushRateLimitAssociation pushes the user's IP to all nodes that have rate limiting enabled.
// This is the P0 trigger source: when a user accesses their subscription URL,
// we know their real IP and can associate it with their subscription's speed limit,
// concurrent connection limit, and device limit.
func (h *PublicHandler) pushRateLimitAssociation(clientIP string, sub *entity.Subscription, tmpl *entity.PlanTemplate) {
	if h.rateLimitPusher == nil || h.nodeServerRepo == nil || clientIP == "" {
		return
	}

	ctx := context.Background()

	// Resolve the client's limits (speed, concurrent connections, device count)
	var maxDownMbps, maxUpMbps, connLimit, deviceLimit int32
	if cl, err := h.clientRepo.GetByID(ctx, sub.ClientID); err == nil {
		// Client-level override takes priority
		if cl.SpeedLimitMbps > 0 {
			maxDownMbps = int32(cl.SpeedLimitMbps)
		}
		if cl.SpeedLimitUpMbps > 0 {
			maxUpMbps = int32(cl.SpeedLimitUpMbps)
		}
		if cl.ConcurrentLimit > 0 {
			connLimit = int32(cl.ConcurrentLimit)
		}
		if cl.DeviceLimit > 0 {
			deviceLimit = int32(cl.DeviceLimit)
		}
		// If client has no override, inherit from plan template
		if maxDownMbps == 0 && tmpl != nil && tmpl.SpeedLimitMbps > 0 {
			maxDownMbps = int32(tmpl.SpeedLimitMbps)
		}
		if maxUpMbps == 0 && tmpl != nil && tmpl.SpeedLimitUpMbps > 0 {
			maxUpMbps = int32(tmpl.SpeedLimitUpMbps)
		}
		if connLimit == 0 && tmpl != nil && tmpl.ConcurrentLimit > 0 {
			connLimit = int32(tmpl.ConcurrentLimit)
		}
		if deviceLimit == 0 && tmpl != nil && tmpl.DeviceLimit > 0 {
			deviceLimit = int32(tmpl.DeviceLimit)
		}
	}

	// Find all servers and push association to those with rate limiting enabled
	servers, err := h.nodeServerRepo.ListEnabled(ctx)
	if err != nil {
		return
	}
	for _, server := range servers {
		if server.ID <= 0 {
			continue
		}
		// Push to this server — the rateLimitSvc will check if rate limiting
		// is enabled for this server before actually sending the command
		if err := h.rateLimitPusher.PushIPAssociation(ctx, server.ID, clientIP, sub.ID, maxDownMbps, maxUpMbps, connLimit, deviceLimit); err != nil {
			log.Printf("sub: rate limit push failed for server %d: %v", server.ID, err)
		}
	}
}

// detectCDNProvider checks CDN-specific headers to identify the CDN provider.
func detectCDNProvider(c *gin.Context) string {
	if c.GetHeader("CF-Connecting-IP") != "" {
		return "cloudflare"
	}
	if c.GetHeader("True-Client-IP") != "" {
		return "akamai"
	}
	if c.GetHeader("X-Real-IP") != "" {
		return "reverse_proxy"
	}
	return ""
}

// buildSubscriptionInfoHeader builds a human-readable subscription info block.
func buildSubscriptionInfoHeader(sub *entity.Subscription, templates billingTemplateConfig) string {
	if len(templates.BillingNodeIDs) == 0 {
		return ""
	}
	var lines []string
	for _, label := range buildBillingInfoLabels(sub, templates) {
		lines = append(lines, "# "+label)
	}
	return strings.Join(lines, "\n")
}

func buildBillingInfoLabels(sub *entity.Subscription, templates billingTemplateConfig) []string {
	return []string{
		renderBillingTemplate(templates.UsedTemplate, sub),
		renderBillingTemplate(templates.RemainingTemplate, sub),
		renderBillingTemplate(templates.ResetTemplate, sub),
		renderBillingTemplate(templates.ExpiryTemplate, sub),
	}
}

func renderBillingTemplate(template string, sub *entity.Subscription) string {
	if template == "" {
		return ""
	}
	variables := buildBillingTemplateVariables(sub)
	replacements := make([]string, 0, len(variables)*2)
	for _, key := range billingTemplateVariables {
		replacements = append(replacements, "{"+key+"}", variables[key])
	}
	replacer := strings.NewReplacer(replacements...)
	return replacer.Replace(template)
}

var billingTemplateVariables = []string{
	"remaining_human",
	"remaining_bytes",
	"remaining_gb",
	"used_human",
	"used_bytes",
	"used_gb",
	"quota_human",
	"quota_bytes",
	"quota_gb",
	"reset_human",
	"reset_days",
	"reset_at",
	"reset_date",
	"expires_human",
	"expires_at",
	"expires_date",
	"expires_unix",
	"quota_state",
}

func buildBillingTemplateVariables(sub *entity.Subscription) map[string]string {
	remain := sub.QuotaBytes - sub.UsedBytesCache
	if remain < 0 {
		remain = 0
	}
	noResetStr := "No Reset"
	todayStr := "Today"
	daysFmt := "%d Days"
	unlimitedStr := "Unlimited"
	// Determine the effective reset time: use NextResetAt if set, otherwise
	// fall back to ExpiresAt — when the subscription expires and renews,
	// the quota is reset at that point.
	effectiveResetAt := sub.NextResetAt
	if (effectiveResetAt == nil || effectiveResetAt.IsZero()) && sub.ExpiresAt != nil && !sub.ExpiresAt.IsZero() {
		effectiveResetAt = sub.ExpiresAt
	}

	resetHuman := noResetStr
	resetDays := "0"
	resetAt := ""
	resetDate := ""
	if effectiveResetAt != nil && !effectiveResetAt.IsZero() {
		resetAt = effectiveResetAt.Format(time.RFC3339)
		resetDate = effectiveResetAt.Format("2006-01-02")
		remaining := time.Until(*effectiveResetAt)
		if remaining <= 0 {
			resetHuman = todayStr
		} else {
			days := int((remaining + 24*time.Hour - 1) / (24 * time.Hour))
			if days < 1 {
				days = 1
			}
			resetDays = strconv.Itoa(days)
			resetHuman = fmt.Sprintf(daysFmt, days)
		}
	}

	expiresHuman := unlimitedStr
	expiresAt := ""
	expiresDate := ""
	expiresUnix := "0"
	if sub.ExpiresAt != nil && !sub.ExpiresAt.IsZero() {
		expiresAt = sub.ExpiresAt.Format(time.RFC3339)
		expiresDate = sub.ExpiresAt.Format("2006-01-02")
		expiresUnix = strconv.FormatInt(sub.ExpiresAt.Unix(), 10)
		expiresHuman = expiresDate
	}

	remainingHuman := unlimitedStr
	quotaHuman := unlimitedStr
	if sub.QuotaBytes > 0 {
		remainingHuman = humanizeBytesGB(remain)
		quotaHuman = humanizeBytesGB(sub.QuotaBytes)
	}

	return map[string]string{
		"remaining_human": remainingHuman,
		"remaining_bytes": strconv.FormatInt(remain, 10),
		"remaining_gb":    fmt.Sprintf("%.2f", float64(remain)/1073741824),
		"used_human":      humanizeBytesGB(sub.UsedBytesCache),
		"used_bytes":      strconv.FormatInt(sub.UsedBytesCache, 10),
		"used_gb":         fmt.Sprintf("%.2f", float64(sub.UsedBytesCache)/1073741824),
		"quota_human":     quotaHuman,
		"quota_bytes":     strconv.FormatInt(sub.QuotaBytes, 10),
		"quota_gb":        fmt.Sprintf("%.2f", float64(sub.QuotaBytes)/1073741824),
		"reset_human":     resetHuman,
		"reset_days":      resetDays,
		"reset_at":        resetAt,
		"reset_date":      resetDate,
		"expires_human":   expiresHuman,
		"expires_at":      expiresAt,
		"expires_date":    expiresDate,
		"expires_unix":    expiresUnix,
		"quota_state":     sub.QuotaState,
	}
}

func defaultBillingTemplateConfig() billingTemplateConfig {
	return billingTemplateConfig{
		UsedTemplate:      "Used: {used_human}",
		RemainingTemplate: "Remaining: {remaining_human}",
		ResetTemplate:     "Reset In: {reset_human}",
		ExpiryTemplate:    "Expires: {expires_human}",
	}
}

func (h *PublicHandler) loadBillingTemplateConfig(ctx context.Context, tmpl *entity.PlanTemplate) billingTemplateConfig {
	config := defaultBillingTemplateConfig()
	if h.systemSettingRepo != nil {
		settings, err := h.systemSettingRepo.GetByPrefix(ctx, "subscription.billing_template.")
		if err == nil {
			if v := strings.TrimSpace(settings["subscription.billing_template.default_used"]); v != "" {
				config.UsedTemplate = v
			}
			if v := strings.TrimSpace(settings["subscription.billing_template.default_remaining"]); v != "" {
				config.RemainingTemplate = v
			}
			if v := strings.TrimSpace(settings["subscription.billing_template.default_reset"]); v != "" {
				config.ResetTemplate = v
			}
			if v := strings.TrimSpace(settings["subscription.billing_template.default_expiry"]); v != "" {
				config.ExpiryTemplate = v
			}
		}
	}
	if tmpl != nil {
		if v := strings.TrimSpace(tmpl.BillingUsedTemplate); v != "" {
			config.UsedTemplate = v
		}
		if v := strings.TrimSpace(tmpl.BillingRemainingTemplate); v != "" {
			config.RemainingTemplate = v
		}
		if v := strings.TrimSpace(tmpl.BillingResetTemplate); v != "" {
			config.ResetTemplate = v
		}
		if v := strings.TrimSpace(tmpl.BillingExpiryTemplate); v != "" {
			config.ExpiryTemplate = v
		}
	}
	return config
}

func humanizeBytesGB(v int64) string {
	return fmt.Sprintf("%.2f GB", float64(v)/1073741824)
}

// buildInfoNode creates a REJECT node with usage info as name (shows in clash proxy list).
// Format: ↑:1.21GB,↓:17.64GB,TOT:512GB,EXPIRES:2026-09-02
func buildInfoNode(sub *entity.Subscription, lang string) *formatter.Node {
	usedGB := float64(sub.UsedBytesCache) / 1073741824
	totalGB := float64(sub.QuotaBytes) / 1073741824
	var expiresStr string
	if sub.ExpiresAt != nil && !sub.ExpiresAt.IsZero() {
		expiresStr = sub.ExpiresAt.Format("2006-01-02")
	}
	remainGB := totalGB - usedGB
	if remainGB < 0 {
		remainGB = 0
	}
	name := fmt.Sprintf("↑:%.2fGB,↓:0.00GB,TOT:%.0fGB,EXPIRES:%s,REMAIN:%.0fGB", usedGB, totalGB, expiresStr, remainGB)
	return &formatter.Node{
		Name:     name,
		Protocol: "reject",
		Host:     "127.0.0.1",
		Port:     0,
	}
}

func formatSubscriptionInfo(sub *entity.Subscription) string {
	upload := sub.UploadBytesCache
	download := sub.DownloadBytesCache
	total := sub.QuotaBytes
	expires := int64(0)
	if sub.ExpiresAt != nil && !sub.ExpiresAt.IsZero() {
		expires = sub.ExpiresAt.Unix()
	}
	return fmt.Sprintf("upload=%d; download=%d; total=%d; expire=%d", upload, download, total, expires)
}

// --- i18n ---

var messages = map[string]map[string]string{
	"zh": {
		"suspended": "订阅已暂停 - 流量已用完，请联系客服续费",
		"expired":   "订阅已到期 - 请续费后使用",
		"inactive":  "订阅未激活",
	},
	"en": {
		"suspended": "Subscription suspended - Traffic exhausted, please contact support",
		"expired":   "Subscription expired - Please renew",
		"inactive":  "Subscription inactive",
	},
	"ja": {
		"suspended": "サブスペンション停止 - トラフィック使い切り、サポートにお問い合わせください",
		"expired":   "サブスペンション期限切れ - 更新してください",
		"inactive":  "サブスペンション未アクティブ",
	},
	"ko": {
		"suspended": "구독 일시 중지 - 트래픽 소진, 고객센터에 문의하세요",
		"expired":   "구독 만료 - 갱신해 주세요",
		"inactive":  "구독 비활성",
	},
	"ru": {
		"suspended": "Подписка приостановлена - Трафик исчерпан, обратитесь в поддержку",
		"expired":   "Подписка истекла - Продлите подписку",
		"inactive":  "Подписка не активна",
	},
}

// detectLanguage detects language from Accept-Language header.
func detectLanguage(acceptLang string) string {
	if acceptLang == "" {
		return "en"
	}
	// Check for zh, ja, ko, ru prefix
	for _, lang := range []string{"zh", "ja", "ko", "ru"} {
		if len(acceptLang) >= 2 && acceptLang[:2] == lang {
			return lang
		}
	}
	return "en"
}

// t returns translated message for the given language and key.
func t(lang, key string) string {
	if msgs, ok := messages[lang]; ok {
		if msg, ok := msgs[key]; ok {
			return msg
		}
	}
	// Fallback to English
	if msgs, ok := messages["en"]; ok {
		if msg, ok := msgs[key]; ok {
			return msg
		}
	}
	return key
}

// ClientVisibleNode represents a node visible to a client, with its share link.
type ClientVisibleNode struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Protocol   string `json:"protocol"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Transport  string `json:"transport"`
	TLS        bool   `json:"tls"`
	Region     string `json:"region,omitempty"`
	GroupName  string `json:"group_name,omitempty"`
	Link       string `json:"link,omitempty"`
	InboundID  int64  `json:"inbound_id,omitempty"`
	ExternalID int64  `json:"external_id,omitempty"`
}

// ListClientVisibleNodes returns all nodes visible to the given client,
// based on the client's plan template and node group membership.
// This reuses the same logic as ServeSubscription but returns structured data.
func (h *PublicHandler) ListClientVisibleNodes(ctx context.Context, clientID int64) ([]ClientVisibleNode, error) {
	if h.clientRepo == nil || clientID == 0 {
		return nil, nil
	}

	client, err := h.clientRepo.GetByID(ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("client not found: %w", err)
	}

	// Security: only return node connection details for clients with an active subscription.
	// Suspended/expired/no-plan clients get an empty list — they can still see node names
	// via the /plans API, but not the connection links.
	if client.Status != "active" || client.PlanID == 0 {
		return []ClientVisibleNode{}, nil
	}

	clientEmail := client.Email

	var members []entity.NodeGroupMember
	if client.PlanID > 0 && h.planSvc != nil {
		if rawMembers, err := h.planSvc.GetTemplateNodeMembers(ctx, client.PlanID); err == nil {
			for _, m := range rawMembers {
				if m.Enabled {
					members = append(members, m)
				}
			}
		}
	}

	shareDomains := h.loadShareDomains(ctx)
	var tmpl *entity.PlanTemplate
	if client.PlanID > 0 && h.planSvc != nil {
		tmpl, _ = h.planSvc.GetTemplateByID(ctx, client.PlanID)
	}
	billingTemplates := h.loadBillingTemplateConfig(ctx, tmpl)

	rendered := h.collectRenderedNodes(ctx, shareDomains, members, clientEmail, billingTemplates)

	nodes := make([]ClientVisibleNode, 0, len(rendered))
	for _, r := range rendered {
		n := ClientVisibleNode{
			ID:         r.MemberID,
			InboundID:  r.InboundConfigID,
			ExternalID: r.ExternalNodeID,
			Link:       r.Link,
		}
		if r.Node != nil {
			n.Name = r.Node.Name
			n.Protocol = r.Node.Protocol
			n.Host = r.Node.Host
			n.Port = r.Node.Port
			n.Transport = r.Node.Transport
			n.TLS = r.Node.TLS
		}
		nodes = append(nodes, n)
	}
	return nodes, nil
}
