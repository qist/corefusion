package subscriptions

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
	"corefusion/internal/subscription/formatter"
	"corefusion/internal/subscription/sharelink"
)

// PublicHandler handles public subscription requests.
type PublicHandler struct {
	subRepo         *repository.SubscriptionRepository
	inboundRepo     *repository.InboundConfigRepository
	shareDomainRepo *repository.ShareDomainConfigRepository
	cfIPEntryRepo   *repository.CFIPEntryRepository
	clientRepo      *repository.ClientRepository
	planNodeRepo    *repository.PlanNodeRepository
	mihomoRepo      *repository.MihomoConfigRepository
	mihomoRuleRepo  *repository.MihomoRuleRepository
	accessLog       *repository.AccessLogRepository
}

// NewPublicHandler creates a new public subscription handler.
func NewPublicHandler(
	subRepo *repository.SubscriptionRepository,
	inboundRepo *repository.InboundConfigRepository,
	shareDomainRepo *repository.ShareDomainConfigRepository,
	cfIPEntryRepo *repository.CFIPEntryRepository,
	clientRepo *repository.ClientRepository,
	planNodeRepo *repository.PlanNodeRepository,
	mihomoRepo *repository.MihomoConfigRepository,
	mihomoRuleRepo *repository.MihomoRuleRepository,
	accessLog *repository.AccessLogRepository,
) *PublicHandler {
	return &PublicHandler{
		subRepo:         subRepo,
		inboundRepo:     inboundRepo,
		shareDomainRepo: shareDomainRepo,
		cfIPEntryRepo:   cfIPEntryRepo,
		clientRepo:      clientRepo,
		planNodeRepo:    planNodeRepo,
		mihomoRepo:      mihomoRepo,
		mihomoRuleRepo:  mihomoRuleRepo,
		accessLog:       accessLog,
	}
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

	// Detect client language from Accept-Language header
	lang := detectLanguage(c.GetHeader("Accept-Language"))

	// Check client status first (client may be suspended while subscription is still active)
	if sub.ClientID > 0 && h.clientRepo != nil {
		if cl, err := h.clientRepo.GetByID(ctx, sub.ClientID); err == nil {
			switch cl.Status {
			case "suspended":
				c.Header("Content-Type", "text/plain; charset=utf-8")
				c.String(http.StatusForbidden, t(lang, "suspended"))
				return
			case "expired":
				c.Header("Content-Type", "text/plain; charset=utf-8")
				c.String(http.StatusForbidden, t(lang, "expired"))
				return
			}
		}
	}

	if sub.Status != "active" {
		c.Header("Content-Type", "text/plain; charset=utf-8")
		c.String(http.StatusForbidden, t(lang, "inactive"))
		return
	}

	shareDomains := h.loadShareDomains(ctx)

	// Get allowed inbound IDs from client's plan and client email
	var allowedInboundIDs map[int64]bool
	var clientEmail string
	if sub.ClientID > 0 && h.clientRepo != nil {
		if cl, err := h.clientRepo.GetByID(ctx, sub.ClientID); err == nil {
			clientEmail = cl.Email
			if cl.PlanID > 0 && h.planNodeRepo != nil {
				if nodes, err := h.planNodeRepo.ListByPlanID(ctx, cl.PlanID); err == nil {
					allowedInboundIDs = make(map[int64]bool)
					for _, n := range nodes {
						if n.Enabled {
							allowedInboundIDs[n.InboundConfigID] = true
						}
					}
				}
			}
		}
	}

	links := h.collectShareLinks(ctx, shareDomains, allowedInboundIDs, clientEmail)

	// Format output - query param ?format= overrides subscription default
	formatStr := c.Query("format")
	if formatStr == "" {
		formatStr = sub.Format
	}
	if formatStr == "" {
		formatStr = "base64_uri"
	}

	var data []byte
	contentType := "text/plain"

	// Determine mihomo config: ?config= param > subscription linked config
	var mihomoConfig *entity.MihomoConfig
	var mihomoRules []entity.MihomoRule

	configName := c.Query("config")
	if configName != "" && h.mihomoRepo != nil {
		// Find config by name
		configs, _ := h.mihomoRepo.List(ctx)
		for _, cfg := range configs {
			if cfg.Name == configName && cfg.Enabled {
				mihomoConfig = &cfg
				break
			}
		}
	} else if sub.MihomoConfigID != nil && h.mihomoRepo != nil {
		// Use subscription's linked config
		mihomoConfig, _ = h.mihomoRepo.GetByID(ctx, *sub.MihomoConfigID)
	}

	if mihomoConfig != nil && h.mihomoRuleRepo != nil {
		mihomoRules, _ = h.mihomoRuleRepo.ListByConfigID(ctx, mihomoConfig.ID)
	}

	switch formatter.Format(formatStr) {
	case formatter.FormatBase64Aggregate:
		joined := strings.Join(links, "\n")
		data = []byte(base64.StdEncoding.EncodeToString([]byte(joined)))
	case formatter.FormatRawURI:
		data = []byte(strings.Join(links, "\n"))
	case formatter.FormatMihomoYAML:
		if mihomoConfig != nil {
			// Use full mihomo config with rules, DNS, proxy-groups, etc.
			data = h.generateMihomoYAML(mihomoConfig, mihomoRules, links)
		} else {
			f := formatter.GetFormatter(formatter.FormatMihomoYAML)
			nodes := h.collectFormatterNodes(ctx, shareDomains, allowedInboundIDs, clientEmail)
			data, _ = f.Format(nodes)
		}
		contentType = "text/yaml"
	case formatter.FormatSingBoxJSON:
		f := formatter.GetFormatter(formatter.FormatSingBoxJSON)
		nodes := h.collectFormatterNodes(ctx, shareDomains, allowedInboundIDs, clientEmail)
		data, _ = f.Format(nodes)
		contentType = f.ContentType()
	case formatter.FormatXrayJSON:
		f := formatter.GetFormatter(formatter.FormatXrayJSON)
		nodes := h.collectFormatterNodes(ctx, shareDomains, allowedInboundIDs, clientEmail)
		data, _ = f.Format(nodes)
		contentType = f.ContentType()
	default:
		joined := strings.Join(links, "\n")
		data = []byte(base64.StdEncoding.EncodeToString([]byte(joined)))
	}

	// Record access
	if h.accessLog != nil {
		ip := c.ClientIP()
		ua := c.Request.UserAgent()
		realIP := c.GetHeader("X-Real-IP")
		cdnIP := c.GetHeader("CF-Connecting-IP")
		cdnProvider := ""
		if cdnIP != "" {
			cdnProvider = "cloudflare"
		}
		go func() {
			h.recordAccess(sub.ID, ip, ua, realIP, cdnIP, cdnProvider)
		}()
	}

	c.Header("Content-Type", contentType+"; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=\"subscription.txt\"")
	c.Header("Profile-Update-Interval", "6")
	c.Header("Subscription-Userinfo", formatSubscriptionInfo(sub))
	c.Data(http.StatusOK, contentType+"; charset=utf-8", data)
}

// collectShareLinks generates share link URIs using the sharelink package.
func (h *PublicHandler) collectShareLinks(ctx context.Context, shareDomains []shareDomainEntry, allowedInboundIDs map[int64]bool, clientEmail string) []string {
	var links []string

	for _, coreType := range []string{"xray", "singbox", "hysteria2", "tuic"} {
		configs, err := h.inboundRepo.List(ctx, coreType)
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
				}
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

// collectFormatterNodes collects nodes for the formatter (used by singbox/xray/mihomo formats).
func (h *PublicHandler) collectFormatterNodes(ctx context.Context, shareDomains []shareDomainEntry, allowedInboundIDs map[int64]bool, clientEmail string) []formatter.Node {
	var nodes []formatter.Node

	for _, coreType := range []string{"xray", "singbox", "hysteria2", "tuic"} {
		configs, err := h.inboundRepo.List(ctx, coreType)
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
				if userClient := findClientByEmail(settings, clientEmail, cfg.Tag); userClient != nil {
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
				}
			} else {
				// Fallback to first client if no email specified
				clients := getSettingsClients(config)
				if len(clients) > 0 {
					c := clients[0]
					uuid, _ = c["id"].(string)
					password, _ = c["password"].(string)
					flow, _ = c["flow"].(string)
					method, _ = c["method"].(string)
					if cfg.Protocol == "hysteria" {
						if auth, ok := c["auth"].(string); ok && auth != "" {
							password = auth
						}
					}
				}
			}

			ss := getMap(config, "streamSettings")
			nodeName := cfg.Remark
			if nodeName == "" {
				nodeName = cfg.Tag
			}
			node := formatter.Node{
				Name:       nodeName,
				Protocol:   cfg.Protocol,
				Host:       host,
				Port:       port,
				Transport:  getString(ss, "network", "raw"),
				TLS:        getString(ss, "security", "none") != "none",
				ServerName: getTLSServerName(ss),
				UUID:       uuid,
				Password:   password,
				Flow:       flow,
				Encryption: method,
			}
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// generateMihomoYAML generates a complete mihomo YAML config with proxies and rules.
func (h *PublicHandler) generateMihomoYAML(config *entity.MihomoConfig, rules []entity.MihomoRule, links []string) []byte {
	var yaml strings.Builder

	// Parse config_json
	var cfg map[string]interface{}
	json.Unmarshal([]byte(config.ConfigJSON), &cfg)

	// Output all top-level fields except proxies and rules (those are injected)
	skipKeys := map[string]bool{"proxies": true, "rules": true}

	for _, key := range getSortedKeys(cfg) {
		if skipKeys[key] {
			continue
		}
		value := cfg[key]
		writeYAMLField(&yaml, key, value, 0)
	}

	// Inject proxies from links
	yaml.WriteString("\nproxies:\n")
	for _, link := range links {
		yaml.WriteString("  - " + link + "\n")
	}

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
	prefix := strings.Repeat("  ", indent)

	switch v := value.(type) {
	case map[string]interface{}:
		yaml.WriteString(prefix + key + ":\n")
		for _, k := range getSortedKeys(v) {
			writeYAMLField(yaml, k, v[k], indent+1)
		}
	case []interface{}:
		yaml.WriteString(prefix + key + ":\n")
		for _, item := range v {
			switch i := item.(type) {
			case map[string]interface{}:
				yaml.WriteString(prefix + "  -\n")
				for _, k := range getSortedKeys(i) {
					writeYAMLField(yaml, k, i[k], indent+2)
				}
			default:
				yaml.WriteString(prefix + "  - " + formatYAMLValue(i) + "\n")
			}
		}
	case string:
		if v != "" {
			yaml.WriteString(prefix + key + ": " + formatYAMLString(v) + "\n")
		}
	case bool:
		yaml.WriteString(prefix + key + ": " + fmt.Sprintf("%v", v) + "\n")
	case float64:
		yaml.WriteString(prefix + key + ": " + fmt.Sprintf("%v", v) + "\n")
	case int:
		yaml.WriteString(prefix + key + ": " + fmt.Sprintf("%d", v) + "\n")
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
	// Characters that need quoting in YAML
	if s == "" {
		return "\"\""
	}
	if s == "true" || s == "false" || s == "null" || s == "~" {
		return "\"" + s + "\""
	}
	if strings.ContainsAny(s, ":#{}[]&*!|>'\"@`") || strings.HasPrefix(s, " ") || strings.HasSuffix(s, " ") {
		return "\"" + strings.ReplaceAll(s, "\"", "\\\"") + "\""
	}
	// Check if it looks like a number or boolean
	if _, err := strconv.ParseFloat(s, 64); err == nil {
		return "\"" + s + "\""
	}
	return s
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

func formatSubscriptionInfo(sub *entity.Subscription) string {
	upload := sub.UsedBytesCache
	total := sub.QuotaBytes
	expires := int64(0)
	if sub.ExpiresAt != nil && !sub.ExpiresAt.IsZero() {
		expires = sub.ExpiresAt.Unix()
	}
	return fmt.Sprintf("upload=%d; download=0; total=%d; expires=%d", upload, total, expires)
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
