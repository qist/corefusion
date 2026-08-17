package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// CaddyManager handles Caddy-specific site management and config generation.
type CaddyManager struct {
dataDir      string
configDir    string
siteRepo     *repository.CaddySiteRepository
settingStore SettingStore
serverID     int64
isLocal      bool
}

// wafBaseDir returns the CaddyGuard WAF root directory: dataDir/proxy/caddyguard
func (m *CaddyManager) wafBaseDir() string {
	return filepath.Join(m.dataDir, "proxy", "caddyguard")
}

// wafRuleDir returns the CaddyGuard rule-config directory.
func (m *CaddyManager) wafRuleDir() string {
	return filepath.Join(m.wafBaseDir(), "rule-config")
}

// SetServerID sets the server ID for per-node site filtering.
func (m *CaddyManager) SetServerID(id int64) { m.serverID = id }

// SetIsLocal sets whether this manager is operating on the local node.
func (m *CaddyManager) SetIsLocal(v bool) { m.isLocal = v }

// NewCaddyManager creates a new CaddyManager.
func NewCaddyManager(dataDir string, siteRepo *repository.CaddySiteRepository, settingStore SettingStore) *CaddyManager {
	absDir := dataDir
	if !filepath.IsAbs(dataDir) {
		if abs, err := filepath.Abs(dataDir); err == nil {
			absDir = abs
		}
	}
	return &CaddyManager{
		dataDir:      absDir,
		configDir:    filepath.Join(absDir, "proxy"),
		siteRepo:     siteRepo,
		settingStore: settingStore,
	}
}

// --- Site CRUD ---

func (m *CaddyManager) ListSites(ctx context.Context) ([]entity.CaddySite, error) {
	return m.siteRepo.List(ctx, m.serverID)
}

func (m *CaddyManager) ListEnabledSites(ctx context.Context) ([]entity.CaddySite, error) {
	return m.siteRepo.ListEnabled(ctx, m.serverID)
}

func (m *CaddyManager) CreateSite(ctx context.Context, site *entity.CaddySite) error {
	if site.ServerID == 0 {
		site.ServerID = m.serverID
	}
	return m.siteRepo.Create(ctx, site)
}

func (m *CaddyManager) GetSite(ctx context.Context, id int64) (*entity.CaddySite, error) {
	return m.siteRepo.GetByID(ctx, id)
}

func (m *CaddyManager) UpdateSite(ctx context.Context, site *entity.CaddySite) error {
	return m.siteRepo.Update(ctx, site)
}

func (m *CaddyManager) DeleteSite(ctx context.Context, id int64) error {
	return m.siteRepo.Delete(ctx, id)
}

// --- Config Generation ---

// GenerateConfig assembles the full Caddyfile from all enabled sites.
func (m *CaddyManager) GenerateConfig(ctx context.Context) (string, error) {
	return m.GenerateConfigForServer(ctx, m.serverID)
}

// GenerateConfigForServer assembles the full Caddyfile from all enabled sites for a specific server.
func (m *CaddyManager) GenerateConfigForServer(ctx context.Context, serverID int64) (string, error) {
	sites, err := m.siteRepo.ListEnabled(ctx, serverID)
	if err != nil {
		return "", err
	}
	// Read SNI settings for proxy_protocol rules
	sniSettings, _ := m.GetSNISettingsForServer(ctx, serverID)

	var sb strings.Builder
	var httpSites []entity.CaddySite
	var l4Sites []entity.CaddySite
	sniByPort := make(map[string][]entity.CaddySite)

	for _, site := range sites {
		if site.Template == "sni" {
			port := site.Path
			if port == "" {
				port = "443"
			}
			sniByPort[port] = append(sniByPort[port], site)
		} else if site.Template == "l4" {
			l4Sites = append(l4Sites, site)
		} else {
			httpSites = append(httpSites, site)
		}
	}

	hasL4 := len(l4Sites) > 0 || len(sniByPort) > 0

	// Check proxy_protocol for SNI/L4
	needPP := false
	for _, site := range l4Sites {
		if site.ProxyProtocol {
			needPP = true
			break
		}
	}
	if !needPP {
		for _, sites := range sniByPort {
			for _, site := range sites {
				if site.ProxyProtocol {
					needPP = true
					break
				}
			}
			if needPP {
				break
			}
		}
	}

	// Global options
	sb.WriteString("{\n")
	sb.WriteString("\tauto_https off\n")

	// CaddyGuard WAF global config — when enabled, all sites automatically get WAF.
	// The caddyguardfile adapter injects the Guard handler into every HTTP server.
	sb.WriteString("\tcaddyguard {\n")
	sb.WriteString(fmt.Sprintf("\t\trule_dir %s\n", m.wafRuleDir()))
	sb.WriteString("\t}\n")
	if needPP {
		sb.WriteString("\n\tservers {\n")
		sb.WriteString("\t\tlistener_wrappers {\n")
		sb.WriteString("\t\t\tproxy_protocol {\n")
		for _, rule := range sniSettings.ProxyProtocolRules {
			sb.WriteString(fmt.Sprintf("\t\t\t\tallow %s\n", rule))
		}
		sb.WriteString("\t\t\t\tfallback_policy ignore\n")
		sb.WriteString("\t\t\t}\n")
		sb.WriteString("\t\t\ttls\n")
		sb.WriteString("\t\t}\n")
		sb.WriteString("\t}\n")
	}
	if hasL4 {
		sb.WriteString("\n\tlayer4 {\n")
		for _, site := range l4Sites {
			block := m.generateL4Block(&site)
			for _, line := range strings.Split(strings.TrimRight(block, "\n"), "\n") {
				sb.WriteString("\t\t" + line + "\n")
			}
		}
		for port, sniSites := range sniByPort {
			localPort := sniSites[0].LocalPort
			if localPort == "" {
				localPort = "8443"
			}
			sb.WriteString(fmt.Sprintf("\t\t:%s {\n", port))
			var fallbackTarget string
			var fallbackPP bool
			for _, site := range sniSites {
				domain := site.Domain
				target := site.SNITarget
				if target == "" {
					target = site.Upstream
				}
				if target == "" {
					target = "127.0.0.1:443"
				}
				if domain == "fallback" || domain == "" {
					fallbackTarget = target
					fallbackPP = site.ProxyProtocol
					continue
				}
				matcher := strings.ReplaceAll(domain, ".", "_")
				matcher = strings.ReplaceAll(matcher, "*", "wildcard")
				sb.WriteString(fmt.Sprintf("\t\t\t# SNI: %s -> %s\n", domain, target))
				sb.WriteString(fmt.Sprintf("\t\t\t@%s tls sni %s\n", matcher, domain))
sb.WriteString(fmt.Sprintf("\t\t\troute @%s {\n", matcher))
				if site.ProxyProtocol {
					sb.WriteString(fmt.Sprintf("\t\t\t\tproxy %s {\n", target))
					sb.WriteString("\t\t\t\t\tproxy_protocol v2\n")
					sb.WriteString("\t\t\t\t}\n")
				} else {
					sb.WriteString(fmt.Sprintf("\t\t\t\tproxy %s\n", target))
				}
				sb.WriteString("\t\t\t}\n")
			}
			if fallbackTarget == "" {
				fallbackTarget = fmt.Sprintf("127.0.0.1:%s", localPort)
			}
sb.WriteString(fmt.Sprintf("\t\t\t# fallback -> %s\n", fallbackTarget))
			sb.WriteString("\t\t\troute {\n")
			if fallbackPP {
				sb.WriteString(fmt.Sprintf("\t\t\t\tproxy %s {\n", fallbackTarget))
				sb.WriteString("\t\t\t\t\tproxy_protocol v2\n")
				sb.WriteString("\t\t\t\t}\n")
			} else {
				sb.WriteString(fmt.Sprintf("\t\t\t\tproxy %s\n", fallbackTarget))
			}
			sb.WriteString("\t\t\t}\n")
			sb.WriteString("\t\t}\n")
		}
		sb.WriteString("\t}\n")
	}
	sb.WriteString("}\n\n")

	if len(httpSites) == 0 && !hasL4 {
		sb.WriteString(":80 {\n")
		if m.isLocal {
			sb.WriteString("\t# ACME HTTP-01 challenge support\n")
			sb.WriteString("\t@acme path /.well-known/acme-challenge/*\n")
			sb.WriteString("\thandle @acme {\n")
sb.WriteString(fmt.Sprintf("\t\treverse_proxy %s\n", m.panelAddr(ctx)))
		sb.WriteString("\t}\n")
		}
		sb.WriteString("\trespond \"CoreFusion\" 200\n")
		sb.WriteString("}\n")
		return sb.String(), nil
	}

	// Group HTTP sites by domain
	domainSites := make(map[string][]entity.CaddySite)
	for _, site := range httpSites {
		domainSites[site.Domain] = append(domainSites[site.Domain], site)
	}

	for domain, siteList := range domainSites {
		sb.WriteString(fmt.Sprintf("# %s\n", domain))
		sb.WriteString(fmt.Sprintf("%s {\n", domain))

		for _, site := range siteList {
			if site.CertPath != "" && site.KeyPath != "" {
				sb.WriteString(fmt.Sprintf("\ttls %s %s\n", m.resolvePath(site.CertPath), m.resolvePath(site.KeyPath)))
				break
			}
		}

		// ACME HTTP-01 challenge support (local node only)
		if m.isLocal {
			sb.WriteString("\t# ACME HTTP-01 challenge support\n")
			sb.WriteString("\t@acme path /.well-known/acme-challenge/*\n")
			sb.WriteString("\thandle @acme {\n")
sb.WriteString(fmt.Sprintf("\t\treverse_proxy %s\n", m.panelAddr(ctx)))
		sb.WriteString("\t}\n")
		}

		// Per-site log block
		// Check log output mode (per-node)
		caddyLogMode := m.getCaddyLogModeForServer(ctx, serverID)
		if caddyLogMode != "off" {
			for _, site := range siteList {
				if site.LogEnabled {
					sb.WriteString("\tlog {\n")
					switch caddyLogMode {
					case "stdout":
						sb.WriteString("\t\toutput stdout\n")
					default: // "file"
						logPath := site.LogPath
						if logPath == "" {
							logPath = fmt.Sprintf("logs/%s.log", sanitizeLogName(domain))
						}
						sb.WriteString(fmt.Sprintf("\t\toutput file %s\n", logPath))
					}
					if site.LogFormat == "json" {
						sb.WriteString("\t\tformat json\n")
					}
					sb.WriteString("\t}\n")
					break
				}
			}
		}

	for _, site := range siteList {
		// Static file server: root + file_server instead of reverse_proxy
			if site.Template == "static" {
				rootDir := strings.TrimSpace(site.Upstream)
				if rootDir == "" {
					rootDir = "/var/www"
				}
				sb.WriteString(fmt.Sprintf("\troot * %s\n", rootDir))
				sb.WriteString("\tfile_server\n")
				// Site-level custom config
				if site.CustomConfig != "" {
					for _, line := range strings.Split(strings.TrimSpace(site.CustomConfig), "\n") {
						sb.WriteString("\t" + strings.TrimSpace(line) + "\n")
					}
				}
				continue
			}
			// Build reverse_proxy block
			backendPath := site.BackendPath
			if backendPath == "" {
				backendPath = "/"
			}
			useHandlePath := false
			if site.Path == "" || site.Path == "/" {
				sb.WriteString("\treverse_proxy {\n")
			} else if backendPath == "/" {
				// Backend expects root path: use handle_path to strip the location prefix
				sb.WriteString(fmt.Sprintf("\thandle_path %s/* {\n", strings.TrimSuffix(site.Path, "/")))
				sb.WriteString("\t\treverse_proxy {\n")
				useHandlePath = true
			} else {
				sb.WriteString(fmt.Sprintf("\t@%s {\n", pathToMatcher(site.Path)))
				sb.WriteString(fmt.Sprintf("\t\tpath %s*\n", site.Path))
				sb.WriteString("\t}\n")
				sb.WriteString(fmt.Sprintf("\treverse_proxy @%s {\n", pathToMatcher(site.Path)))
			}
			// Multiple upstream targets
			for _, up := range strings.Split(site.Upstream, ",") {
				up = strings.TrimSpace(up)
				if up != "" {
					sb.WriteString(fmt.Sprintf("\t\tto %s\n", up))
				}
			}
			// Load balancing policy
			if site.LbPolicy != "" {
				sb.WriteString(fmt.Sprintf("\t\tlb_policy %s\n", site.LbPolicy))
			}
			sb.WriteString(m.proxyHeaders(&site))
			// Proxy-level custom config (inside reverse_proxy)
			if site.ProxyConfig != "" {
				for _, line := range strings.Split(strings.TrimSpace(site.ProxyConfig), "\n") {
					sb.WriteString("\t\t" + strings.TrimSpace(line) + "\n")
				}
			}
			if useHandlePath {
				sb.WriteString("\t\t}\n") // close reverse_proxy
				sb.WriteString("\t}\n")   // close handle_path
			} else {
				sb.WriteString("\t}\n") // close reverse_proxy
			}
			// Site-level custom config (outside reverse_proxy)
			if site.CustomConfig != "" {
				for _, line := range strings.Split(strings.TrimSpace(site.CustomConfig), "\n") {
					sb.WriteString("\t" + strings.TrimSpace(line) + "\n")
				}
			}
		}
		sb.WriteString("}\n\n")
	}

	return sb.String(), nil
}

// panelAddr returns the panel address (host:port) for reverse_proxy directives.
// It reads panel.listen_port from settings, defaulting to 8080.
func (m *CaddyManager) panelAddr(ctx context.Context) string {
	port := "8080"
	if m.settingStore != nil {
		if v, err := m.settingStore.GetByKey(ctx, "panel.listen_port"); err == nil && v != "" {
			port = v
		}
	}
	return "127.0.0.1:" + port
}

func (m *CaddyManager) resolvePath(path string) string {
	if path == "" {
		return path
	}
	// Return relative path so config is portable across local and remote nodes.
	// Caddy runs with cmd.Dir = dataDir, so relative paths resolve correctly.
	if filepath.IsAbs(path) {
		if rel, err := filepath.Rel(m.dataDir, path); err == nil && !strings.HasPrefix(rel, "..") {
			return rel
		}
		return path
	}
	base := filepath.Base(m.dataDir)
	if strings.HasPrefix(path, base+"/") {
		return strings.TrimPrefix(path, base+"/")
	}
	return path
}

func (m *CaddyManager) proxyHeaders(site *entity.CaddySite) string {
	var sb strings.Builder
	switch site.Template {
	case "static":
		// no proxy headers for static file server
	case "websocket":
		sb.WriteString("\t\theader_up Upgrade {>Upgrade}\n")
		sb.WriteString("\t\theader_up Connection {>Connection}\n")
		sb.WriteString("\t\theader_up Host {hostport}\n")
		sb.WriteString("\t\theader_up X-Real-IP {remote_host}\n")
		sb.WriteString("\t\theader_up X-Forwarded-For {remote_host}\n")
		sb.WriteString("\t\theader_up X-Forwarded-Proto {scheme}\n")
	case "grpc":
		sb.WriteString("\t\ttransport http {\n")
		sb.WriteString("\t\t\tversions h2c\n")
		sb.WriteString("\t\t}\n")
		sb.WriteString("\t\theader_up X-Real-IP {remote_host}\n")
		sb.WriteString("\t\theader_up X-Forwarded-For {remote_host}\n")
	case "xhttp", "httpupgrade":
		sb.WriteString("\t\theader_up Upgrade {>Upgrade}\n")
		sb.WriteString("\t\theader_up Connection {>Connection}\n")
		sb.WriteString("\t\theader_up Host {hostport}\n")
		sb.WriteString("\t\theader_up X-Real-IP {remote_host}\n")
		sb.WriteString("\t\theader_up X-Forwarded-For {remote_host}\n")
		sb.WriteString("\t\theader_up X-Forwarded-Proto {scheme}\n")
	default:
		sb.WriteString("\t\theader_up X-Real-IP {remote_host}\n")
		sb.WriteString("\t\theader_up X-Forwarded-For {remote_host}\n")
		sb.WriteString("\t\theader_up X-Forwarded-Proto {scheme}\n")
	}
	return sb.String()
}

func (m *CaddyManager) generateL4Block(site *entity.CaddySite) string {
	var sb strings.Builder
	port := site.Path
	if port == "" {
		port = "443"
	}
	port = site.Path
	if port == "" {
		port = "443"
	}
	upstream := site.Upstream
	if upstream == "" {
		upstream = m.panelAddr(context.Background())
	}
	proto := site.L4Protocol
	if proto == "" {
		proto = "tcp"
	}
	sb.WriteString(fmt.Sprintf("# L4 %s forward on :%s\n", proto, port))
	sb.WriteString(fmt.Sprintf(":%s {\n", port))
	sb.WriteString("\troute {\n")
	if site.ProxyProtocol {
		sb.WriteString(fmt.Sprintf("\t\tproxy %s {\n", upstream))
		sb.WriteString("\t\t\tproxy_protocol v2\n")
		sb.WriteString("\t\t}\n")
	} else {
		sb.WriteString(fmt.Sprintf("\t\tproxy %s\n", upstream))
	}
	sb.WriteString("\t}\n")
	sb.WriteString("}\n")
	return sb.String()
}

// SaveAndApply generates config and writes to disk.
func (m *CaddyManager) SaveAndApply(ctx context.Context) (string, error) {
	config, err := m.GenerateConfig(ctx)
	if err != nil {
		return "", err
	}
	path := filepath.Join(m.configDir, "Caddyfile")
	os.MkdirAll(m.configDir, 0750)
	if err := os.WriteFile(path, []byte(config), 0644); err != nil {
		return "", err
	}
	return config, nil
}

// --- SNI Management ---

func (m *CaddyManager) ToggleSNIEnabled(ctx context.Context, enabled bool) (int, error) {
	sites, err := m.siteRepo.List(ctx, m.serverID)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, site := range sites {
		if site.Template == "sni" {
			site.Enabled = enabled
			if err := m.siteRepo.Update(ctx, &site); err != nil {
				continue
			}
			count++
		}
	}
	return count, nil
}

func (m *CaddyManager) DeleteAllSNI(ctx context.Context) (int, error) {
	return m.deleteByTemplate(ctx, "sni")
}

func (m *CaddyManager) DeleteAllL4(ctx context.Context) (int, error) {
	return m.deleteByTemplate(ctx, "l4")
}

func (m *CaddyManager) deleteByTemplate(ctx context.Context, template string) (int, error) {
	sites, err := m.siteRepo.List(ctx, m.serverID)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, site := range sites {
		if site.Template == template {
			if err := m.siteRepo.Delete(ctx, site.ID); err != nil {
				continue
			}
			count++
		}
	}
	return count, nil
}

// BatchUpdatePorts updates HTTPS site domains to add/remove port suffix.
func (m *CaddyManager) BatchUpdatePorts(ctx context.Context, port string, enable bool) (int, error) {
	sites, err := m.siteRepo.List(ctx, m.serverID)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, site := range sites {
		if site.Template == "l4" || site.Template == "sni" {
			continue
		}
		if site.CertPath == "" {
			continue
		}
		domain := site.Domain
		if enable {
			if strings.Contains(domain, ":") {
				continue
			}
			site.Domain = domain + ":" + port
		} else {
			if idx := strings.LastIndex(domain, ":"); idx > 0 {
				site.Domain = domain[:idx]
			} else {
				continue
			}
		}
		if err := m.siteRepo.Update(ctx, &site); err != nil {
			continue
		}
		count++
	}
	return count, nil
}

// perNodeSNISettingsKey returns the settings key for a server ID.
// Local node (serverID=1) uses the legacy global key for backward compatibility.
// Remote nodes use a per-node key.
func perNodeSNISettingsKey(baseKey string, serverID int64) string {
	if serverID <= 1 {
		return baseKey
	}
	return fmt.Sprintf("%s.%d", baseKey, serverID)
}

// GetSNISettings reads SNI settings from database (uses the manager's serverID).
func (m *CaddyManager) GetSNISettings(ctx context.Context) (*SNISettings, error) {
	return m.GetSNISettingsForServer(ctx, m.serverID)
}

// GetSNISettingsForServer reads SNI settings from database for a specific server.
func (m *CaddyManager) GetSNISettingsForServer(ctx context.Context, serverID int64) (*SNISettings, error) {
	settings := &SNISettings{
		Enabled:            false,
		ListenPort:         "443",
		LocalPort:          "8443",
		ProxyProtocolRules: []string{"127.0.0.0/8", "::1/128"},
	}
	if m.settingStore == nil {
		return settings, nil
	}

	// Try per-node keys first
	enabledKey := perNodeSNISettingsKey("proxy.sni_enabled", serverID)
	if v, err := m.settingStore.GetByKey(ctx, enabledKey); err == nil && v == "true" {
		settings.Enabled = true
	} else if serverID > 1 {
		// Fallback to global key for backward compatibility
		if v, err := m.settingStore.GetByKey(ctx, "proxy.sni_enabled"); err == nil && v == "true" {
			settings.Enabled = true
		}
	}

	listenPortKey := perNodeSNISettingsKey("proxy.sni_listen_port", serverID)
	if v, err := m.settingStore.GetByKey(ctx, listenPortKey); err == nil && v != "" {
		settings.ListenPort = v
	} else if serverID > 1 {
		if v, err := m.settingStore.GetByKey(ctx, "proxy.sni_listen_port"); err == nil && v != "" {
			settings.ListenPort = v
		}
	}

	localPortKey := perNodeSNISettingsKey("proxy.sni_local_port", serverID)
	if v, err := m.settingStore.GetByKey(ctx, localPortKey); err == nil && v != "" {
		settings.LocalPort = v
	} else if serverID > 1 {
		if v, err := m.settingStore.GetByKey(ctx, "proxy.sni_local_port"); err == nil && v != "" {
			settings.LocalPort = v
		}
	}

	rulesKey := perNodeSNISettingsKey("proxy.proxy_protocol_rules", serverID)
	if v, err := m.settingStore.GetByKey(ctx, rulesKey); err == nil && v != "" {
		var rules []string
		if json.Unmarshal([]byte(v), &rules) == nil && len(rules) > 0 {
			settings.ProxyProtocolRules = rules
		}
	} else if serverID > 1 {
		if v, err := m.settingStore.GetByKey(ctx, "proxy.proxy_protocol_rules"); err == nil && v != "" {
			var rules []string
			if json.Unmarshal([]byte(v), &rules) == nil && len(rules) > 0 {
				settings.ProxyProtocolRules = rules
			}
		}
	}
	return settings, nil
}

// SaveSNISettings saves SNI settings to database (uses the manager's serverID).
func (m *CaddyManager) SaveSNISettings(ctx context.Context, settings *SNISettings) error {
	if m.settingStore == nil {
		return nil
	}
	enabled := "false"
	if settings.Enabled {
		enabled = "true"
	}

	enabledKey := perNodeSNISettingsKey("proxy.sni_enabled", m.serverID)
	if err := m.settingStore.Set(ctx, enabledKey, enabled); err != nil {
		return err
	}
	if settings.ListenPort == "" {
		settings.ListenPort = "443"
	}
	if settings.LocalPort == "" {
		settings.LocalPort = "8443"
	}

	listenPortKey := perNodeSNISettingsKey("proxy.sni_listen_port", m.serverID)
	if err := m.settingStore.Set(ctx, listenPortKey, settings.ListenPort); err != nil {
		return err
	}

	localPortKey := perNodeSNISettingsKey("proxy.sni_local_port", m.serverID)
	if err := m.settingStore.Set(ctx, localPortKey, settings.LocalPort); err != nil {
		return err
	}

	// Save proxy_protocol rules
	if len(settings.ProxyProtocolRules) > 0 {
		data, _ := json.Marshal(settings.ProxyProtocolRules)
		rulesKey := perNodeSNISettingsKey("proxy.proxy_protocol_rules", m.serverID)
		return m.settingStore.Set(ctx, rulesKey, string(data))
	}
	return nil
}

// sanitizeLogName converts a domain to a clean log file name.
// :80 -> default, _ -> default, www.example.com -> www.example.com
func sanitizeLogName(domain string) string {
	if domain == "" || domain == "_" || domain == ":80" || domain == ":443" {
		return "default"
	}
	// Remove leading colon-port for port-only domains like ":8080"
	if strings.HasPrefix(domain, ":") {
		return "port" + domain[1:]
	}
	return domain
}

// getCaddyLogModeForServer reads the caddy log output mode for a specific server.
// Uses per-node key for remote nodes, global key for local node.
func (m *CaddyManager) getCaddyLogModeForServer(ctx context.Context, serverID int64) string {
	caddyLogMode := "file"
	if m.settingStore == nil {
		return caddyLogMode
	}
	key := perNodeSNISettingsKey("proxy.log_output_config", serverID)
	v, err := m.settingStore.GetByKey(ctx, key)
	if err != nil || v == "" {
		// Fall back to global key
		v, err = m.settingStore.GetByKey(ctx, "proxy.log_output_config")
		if err != nil || v == "" {
			return caddyLogMode
		}
	}
	var cfg struct {
		Caddy struct {
			Mode string `json:"mode"`
		} `json:"caddy"`
	}
	if json.Unmarshal([]byte(v), &cfg) == nil && cfg.Caddy.Mode != "" {
		caddyLogMode = cfg.Caddy.Mode
	}
	return caddyLogMode
}

// BatchUpdateLogEnabled updates the log_enabled flag for all HTTP sites of a server.
// When log output mode is "off", all sites should have log_enabled=false.
// When log output mode is "file" or "stdout", all sites should have log_enabled=true.
func (m *CaddyManager) BatchUpdateLogEnabled(ctx context.Context, enabled bool) (int, error) {
	sites, err := m.siteRepo.List(ctx, m.serverID)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, site := range sites {
		if site.Template == "sni" || site.Template == "l4" {
			continue
		}
		if site.LogEnabled == enabled {
			continue
		}
		site.LogEnabled = enabled
		if err := m.siteRepo.Update(ctx, &site); err != nil {
			continue
		}
		count++
	}
	return count, nil
}

// --- WAF Management (CaddyGuard) ---
// CaddyGuard uses JSON config (config.json) and .rule files, same format as
// NginxGuard. The WAF root is dataDir/proxy/caddyguard/rule-config/.

// ReadWAFConfig reads the CaddyGuard config.json file.
func (m *CaddyManager) ReadWAFConfig() (string, error) {
	path := filepath.Join(m.wafRuleDir(), "config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// SaveWAFConfig writes the CaddyGuard config.json file.
func (m *CaddyManager) SaveWAFConfig(content string) error {
	path := filepath.Join(m.wafRuleDir(), "config.json")
	os.MkdirAll(filepath.Dir(path), 0755)
	return os.WriteFile(path, []byte(content), 0644)
}

// ReadWAFDomainConfig reads the domain.json file.
func (m *CaddyManager) ReadWAFDomainConfig() (string, error) {
	path := filepath.Join(m.wafRuleDir(), "domain.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// SaveWAFDomainConfig writes the domain.json file.
func (m *CaddyManager) SaveWAFDomainConfig(content string) error {
	path := filepath.Join(m.wafRuleDir(), "domain.json")
	os.MkdirAll(filepath.Dir(path), 0755)
	return os.WriteFile(path, []byte(content), 0644)
}

// ListWAFRules lists all global .rule files in rule-config/.
func (m *CaddyManager) ListWAFRules() []WAFRuleFile {
	dir := m.wafRuleDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var files []WAFRuleFile
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".rule") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, WAFRuleFile{Name: name, Size: info.Size()})
	}
	return files
}

// ReadWAFRule reads a global rule file by name (e.g. "url.rule").
func (m *CaddyManager) ReadWAFRule(name string) (string, error) {
	if strings.Contains(name, "..") || strings.Contains(name, "/") {
		return "", fmt.Errorf("invalid rule name")
	}
	path := filepath.Join(m.wafRuleDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// SaveWAFRule writes a global rule file by name.
func (m *CaddyManager) SaveWAFRule(name, content string) error {
	if strings.Contains(name, "..") || strings.Contains(name, "/") {
		return fmt.Errorf("invalid rule name")
	}
	path := filepath.Join(m.wafRuleDir(), name)
	os.MkdirAll(filepath.Dir(path), 0755)
	return os.WriteFile(path, []byte(content), 0644)
}

// ListWAFDomainRuleDirs lists domain directories under rule-config/domains/.
func (m *CaddyManager) ListWAFDomainRuleDirs() []string {
	dir := filepath.Join(m.wafRuleDir(), "domains")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names
}

// ReadWAFDomainRule reads a per-domain rule file.
func (m *CaddyManager) ReadWAFDomainRule(domain, ruleName string) (string, error) {
	if strings.Contains(domain, "..") || strings.Contains(ruleName, "..") {
		return "", fmt.Errorf("invalid path")
	}
	path := filepath.Join(m.wafRuleDir(), "domains", domain, ruleName)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// SaveWAFDomainRule writes a per-domain rule file.
func (m *CaddyManager) SaveWAFDomainRule(domain, ruleName, content string) error {
	if strings.Contains(domain, "..") || strings.Contains(ruleName, "..") {
		return fmt.Errorf("invalid path")
	}
	path := filepath.Join(m.wafRuleDir(), "domains", domain, ruleName)
	os.MkdirAll(filepath.Dir(path), 0755)
	return os.WriteFile(path, []byte(content), 0644)
}

// CollectWAFFiles reads all CaddyGuard WAF files from disk and returns them
// as (relativePath, content) pairs for syncing to remote Agent nodes.
// Paths are prefixed with "caddyguard/" so Agent writes to proxy/caddyguard/.
func (m *CaddyManager) CollectWAFFiles() []WAFFileEntry {
	wafDir := m.wafBaseDir()
	var entries []WAFFileEntry
	filepath.Walk(wafDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(wafDir, path)
		if err != nil {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		entries = append(entries, WAFFileEntry{
			Path:    "caddyguard/" + filepath.ToSlash(rel),
			Content: data,
		})
		return nil
	})
	return entries
}

// IsWAFDomainEnabled checks if a domain has WAF enabled in domain.json.
func (m *CaddyManager) IsWAFDomainEnabled(domain string) bool {
	data, err := os.ReadFile(filepath.Join(m.wafRuleDir(), "domain.json"))
	if err != nil {
		return false
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return false
	}
	cfg, ok := obj[domain]
	if !ok {
		return false
	}
	if cfgMap, ok := cfg.(map[string]interface{}); ok {
		if wafEnable, ok := cfgMap["waf_enable"].(string); ok && wafEnable == "off" {
			return false
		}
	}
	return true
}

// EnableWAFDomain enables WAF for a domain by adding it to domain.json and
// creating a per-domain rule directory with default rules.
func (m *CaddyManager) EnableWAFDomain(domain string) error {
	if strings.Contains(domain, "..") || domain == "" {
		return fmt.Errorf("invalid domain")
	}
	domainDir := filepath.Join(m.wafRuleDir(), "domains", domain)
	os.MkdirAll(domainDir, 0755)
	globalDir := m.wafRuleDir()
	entries, _ := os.ReadDir(globalDir)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".rule") {
			continue
		}
		src := filepath.Join(globalDir, e.Name())
		dst := filepath.Join(domainDir, e.Name())
		data, err := os.ReadFile(src)
		if err != nil {
			continue
		}
		os.WriteFile(dst, data, 0644)
	}
	return m.updateDomainJSON(domain, map[string]string{
		"rule_dir": fmt.Sprintf("domains/%s", domain),
	})
}

// DisableWAFDomain disables WAF for a domain by setting waf_enable off in domain.json.
func (m *CaddyManager) DisableWAFDomain(domain string) error {
	if strings.Contains(domain, "..") || domain == "" {
		return fmt.Errorf("invalid domain")
	}
	return m.updateDomainJSON(domain, map[string]string{
		"waf_enable": "off",
	})
}

// updateDomainJSON updates a domain's config in domain.json (merge keys).
func (m *CaddyManager) updateDomainJSON(domain string, updates map[string]string) error {
	path := filepath.Join(m.wafRuleDir(), "domain.json")
	os.MkdirAll(filepath.Dir(path), 0755)
	data, err := os.ReadFile(path)
	var obj map[string]map[string]interface{}
	if err != nil {
		obj = make(map[string]map[string]interface{})
	} else {
		json.Unmarshal(data, &obj)
		if obj == nil {
			obj = make(map[string]map[string]interface{})
		}
	}
	cfg, ok := obj[domain]
	if !ok {
		cfg = make(map[string]interface{})
	}
	for k, v := range updates {
		cfg[k] = v
	}
	obj[domain] = cfg
	out, _ := json.MarshalIndent(obj, "", "  ")
	return os.WriteFile(path, out, 0644)
}

// removeFromDomainJSON removes a domain from domain.json.
func (m *CaddyManager) removeFromDomainJSON(domain string) error {
	path := filepath.Join(m.wafRuleDir(), "domain.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var obj map[string]map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	delete(obj, domain)
	out, _ := json.MarshalIndent(obj, "", "  ")
	return os.WriteFile(path, out, 0644)
}
