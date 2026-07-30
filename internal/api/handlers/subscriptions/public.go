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
	"corefusion/pkg/netutil"
	"corefusion/internal/store/repository"
	"corefusion/internal/subscription/formatter"
	"corefusion/internal/subscription/mihomoconv"
	"corefusion/internal/subscription/sharelink"
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

// shareableProtocols are protocols that generate share links.
var shareableProtocols = map[string]bool{
	"vless":           true,
	"vmess":           true,
	"trojan":          true,
	"shadowsocks":     true,
	"shadowsocks2022": true,
	"hysteria":        true,
	"hysteria2":       true,
	"tuic":            true,
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
		allLinks := append(billingLinks, links...)
		xrayNodes := parseSubscriptionLinksToFormatterNodes(allLinks)
		f := formatter.GetFormatter(formatter.FormatXrayJSON)
		data, _ = f.Format(xrayNodes)
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
			// Only nodes with _accessDomain enabled
			ad, ok := config["_accessDomain"].(map[string]interface{})
			if !ok || !ad["enabled"].(bool) {
				continue
			}

			host, port := resolveHostPort(config, cfg.Listen, cfg.Port, shareDomains)

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
				FinalMask:      getFinalMask(config),
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
			ad, ok := config["_accessDomain"].(map[string]interface{})
			if !ok {
				continue
			}
			enabled, _ := ad["enabled"].(bool)
			if !enabled {
				continue
			}

			host, port := resolveHostPort(config, cfg.Listen, cfg.Port, shareDomains)
			settings := getMap(config, "settings")

			linkSettings := settings
			var uuid, password, flow, method string
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
				method, _ = userClient["method"].(string)
				if cfg.Protocol == "hysteria" {
					if auth, ok := userClient["auth"].(string); ok && auth != "" {
						password = auth
					}
				}
			} else {
				// No client email → skip nodes requiring authentication (security)
				continue
			}

			nodeName := cfg.Remark
			if nodeName == "" {
				nodeName = cfg.Tag
			}
			shareNode := sharelink.Node{
				Tag:            nodeName,
				Protocol:       cfg.Protocol,
				Host:           host,
				Port:           port,
				Settings:       linkSettings,
				StreamSettings: getMap(config, "streamSettings"),
				FinalMask:      getFinalMask(config),
			}
			link := sharelink.Generate(shareNode)

			ss := getMap(config, "streamSettings")
			protocol := cfg.Protocol
			if protocol == "hysteria" {
				protocol = "hysteria2"
			}
			transport := getString(ss, "network", "raw")
			supportedTransports := map[string]bool{"tcp": true, "raw": true, "ws": true, "grpc": true, "xhttp": true, "h2": true, "http": true, "httpupgrade": true}
			if !supportedTransports[transport] {
				transport = "tcp"
			}
			if transport == "raw" {
				transport = "tcp"
			}
			node := &formatter.Node{
				Name:       nodeName,
				Protocol:   protocol,
				Host:       host,
				Port:       port,
				Transport:  transport,
				TLS:        getString(ss, "security", "none") != "none",
				ServerName: getTLSServerName(ss),
				UUID:       uuid,
				Password:   password,
				Flow:       flow,
				Encryption: method,
				Extra:      buildExtra(config, ss, cfg.Protocol),
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
			ad, ok := config["_accessDomain"].(map[string]interface{})
			if !ok || !ad["enabled"].(bool) {
				continue
			}

			host, port := resolveHostPort(config, cfg.Listen, cfg.Port, shareDomains)
			settings := getMap(config, "settings")
			var uuid, password, flow, method string

			// Find the specific client's credentials for this subscription
			if clientEmail != "" {
				userClient := findClientByEmail(settings, clientEmail, cfg.Tag)
				if userClient == nil {
					// Client has no account on this node → skip (security: never use other users' credentials)
					continue
				}
				uuid, _ = userClient["id"].(string)
				password, _ = userClient["password"].(string)
				flow, _ = userClient["flow"].(string)
				method, _ = userClient["method"].(string)
				// Hysteria uses "auth" field instead of "password"
				if cfg.Protocol == "hysteria" {
					if auth, ok := userClient["auth"].(string); ok && auth != "" {
						password = auth
					}
				}
			} else {
				// No client email → skip nodes requiring authentication (security)
				continue
			}

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
			transport := getString(ss, "network", "raw")
			// Filter unsupported transports for mihomo
			supportedTransports := map[string]bool{"tcp": true, "raw": true, "ws": true, "grpc": true, "xhttp": true, "h2": true, "http": true, "httpupgrade": true}
			if !supportedTransports[transport] {
				transport = "tcp"
			}
			if transport == "raw" {
				transport = "tcp"
			}
			node := formatter.Node{
				Name:       nodeName,
				Protocol:   protocol,
				Host:       host,
				Port:       port,
				Transport:  transport,
				TLS:        getString(ss, "security", "none") != "none",
				ServerName: getTLSServerName(ss),
				UUID:       uuid,
				Password:   password,
				Flow:       flow,
				Encryption: method,
				Extra:      extra,
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
	if strings.ContainsAny(s, ":#{}[]&*!|>'\"@`,?+\\") || strings.HasPrefix(s, " ") || strings.HasSuffix(s, " ") || strings.HasPrefix(s, "-") || strings.HasPrefix(s, "+") || strings.HasPrefix(s, ".") || strings.HasPrefix(s, "@") {
		escaped := strings.ReplaceAll(s, "\\", "\\\\")
		escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
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
	ad, ok := config["_accessDomain"].(map[string]interface{})
	if !ok {
		return fallbackHost(listenHost), listenPort
	}
	enabled, _ := ad["enabled"].(bool)
	if !enabled {
		return fallbackHost(listenHost), listenPort
	}

	adType, _ := ad["type"].(string)
	portMode, _ := ad["directPortMode"].(string)

	switch adType {
	case "direct":
		if domain, _ := ad["directDomain"].(string); domain != "" {
			return domain, resolvePort(ad, portMode, listenPort, shareDomains, "direct", domain)
		}
		if d := pickByType(shareDomains, "direct"); d != nil {
			return d.Host, resolvePortFromEntry(d, portMode, listenPort)
		}
	case "cf_domain":
		if domain, _ := ad["cfDomain"].(string); domain != "" {
			return domain, resolvePort(ad, portMode, listenPort, shareDomains, "cf_domain", domain)
		}
		if d := pickByType(shareDomains, "cf_domain"); d != nil {
			return d.Host, resolvePortFromEntry(d, portMode, listenPort)
		}
	case "cf_ip":
		if ip, _ := ad["cfIP"].(string); ip != "" {
			return ip, resolvePort(ad, portMode, listenPort, shareDomains, "cf_ip", ip)
		}
		if d := pickByType(shareDomains, "cf_ip"); d != nil {
			return d.Host, resolvePortFromEntry(d, portMode, listenPort)
		}
	case "custom":
		if domain, _ := ad["customDomain"].(string); domain != "" {
			customPort := 443
			if p, ok := ad["customPort"].(float64); ok && p > 0 {
				customPort = int(p)
			}
			return domain, customPort
		}
	}

	if len(shareDomains) > 0 {
		d := shareDomains[rand.Intn(len(shareDomains))]
		port := d.Ports[rand.Intn(len(d.Ports))]
		return d.Host, port
	}
	return fallbackHost(listenHost), listenPort
}

func pickByType(domains []shareDomainEntry, domainType string) *shareDomainEntry {
	var filtered []shareDomainEntry
	for _, d := range domains {
		if d.Type == domainType {
			filtered = append(filtered, d)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	d := filtered[rand.Intn(len(filtered))]
	return &d
}

func resolvePortFromEntry(d *shareDomainEntry, portMode string, listenPort int) int {
	if portMode == "inbound" {
		return listenPort
	}
	if len(d.Ports) > 0 {
		return d.Ports[rand.Intn(len(d.Ports))]
	}
	return d.Port
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
	v, ok := m[key].([]interface{})
	if !ok {
		return nil
	}
	var result []string
	for _, item := range v {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
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
	return result
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
	flow, _ := p["flow"].(string)
	encryption, _ := p["encryption"].(string)

	tls := false
	if v, ok := p["tls"].(bool); ok {
		tls = v
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
		reality := getMap(ss, "realitySettings")
		if reality != nil {
			ro := make(map[string]interface{})
			if pk, ok := reality["publicKey"].(string); ok && pk != "" {
				ro["public-key"] = pk
			}
			if sid, ok := reality["shortId"].(string); ok && sid != "" {
				ro["short-id"] = sid
			}
			if len(ro) > 0 {
				extra["reality-opts"] = ro
			}
			if fp, ok := reality["fingerprint"].(string); ok && fp != "" {
				extra["client-fingerprint"] = fp
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
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Protocol    string `json:"protocol"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Transport   string `json:"transport"`
	TLS         bool   `json:"tls"`
	Region      string `json:"region,omitempty"`
	GroupName   string `json:"group_name,omitempty"`
	Link        string `json:"link,omitempty"`
	InboundID   int64  `json:"inbound_id,omitempty"`
	ExternalID  int64  `json:"external_id,omitempty"`
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
