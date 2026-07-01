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
}

// SetServerID sets the server ID for per-node site filtering.
func (m *CaddyManager) SetServerID(id int64) { m.serverID = id }

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
	sites, err := m.siteRepo.ListEnabled(ctx, m.serverID)
	if err != nil {
		return "", err
	}
	compatMode := proxySandboxCompatMode(m.dataDir)

	// Read SNI settings for proxy_protocol rules
	sniSettings, _ := m.GetSNISettings(ctx)

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
			port = caddyCompatPort(port, true, compatMode)
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
	if compatMode {
		sb.WriteString("\thttp_port 29088\n")
		sb.WriteString("\thttps_port 29443\n")
	}
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
				sb.WriteString(fmt.Sprintf("\t\t\t\tproxy %s\n", target))
				if site.ProxyProtocol {
					sb.WriteString("\t\t\t\tproxy_protocol\n")
				}
				sb.WriteString("\t\t\t}\n")
			}
			if fallbackTarget == "" {
				fallbackTarget = fmt.Sprintf("127.0.0.1:%s", localPort)
			}
			sb.WriteString(fmt.Sprintf("\t\t\t# fallback -> %s\n", fallbackTarget))
			sb.WriteString("\t\t\troute {\n")
			sb.WriteString(fmt.Sprintf("\t\t\t\tproxy %s\n", fallbackTarget))
			if fallbackPP {
				sb.WriteString("\t\t\t\tproxy_protocol\n")
			}
			sb.WriteString("\t\t\t}\n")
			sb.WriteString("\t\t}\n")
		}
		sb.WriteString("\t}\n")
	}
	sb.WriteString("}\n\n")

	if len(httpSites) == 0 && !hasL4 {
		addr := ":80"
		if compatMode {
			addr = ":29088"
		}
		sb.WriteString(addr + " {\n")
		sb.WriteString("\t# ACME HTTP-01 challenge support\n")
		sb.WriteString("\t@acme path /.well-known/acme-challenge/*\n")
		sb.WriteString("\thandle @acme {\n")
		sb.WriteString("\t\treverse_proxy 127.0.0.1:8080\n")
		sb.WriteString("\t}\n")
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
		siteAddr := caddyCompatAddress(domain, compatMode)
		sb.WriteString(fmt.Sprintf("# %s\n", domain))
		sb.WriteString(fmt.Sprintf("%s {\n", siteAddr))

		for _, site := range siteList {
			if site.CertPath != "" && site.KeyPath != "" {
				sb.WriteString(fmt.Sprintf("\ttls %s %s\n", m.resolvePath(site.CertPath), m.resolvePath(site.KeyPath)))
				break
			}
		}

		// ACME HTTP-01 challenge support
		sb.WriteString("\t# ACME HTTP-01 challenge support\n")
		sb.WriteString("\t@acme path /.well-known/acme-challenge/*\n")
		sb.WriteString("\thandle @acme {\n")
		sb.WriteString("\t\treverse_proxy 127.0.0.1:8080\n")
		sb.WriteString("\t}\n")

		// Per-site log block (from first site that has it)
		// Check global log output mode
		caddyLogMode := "file"
		if m.settingStore != nil {
			if v, err := m.settingStore.GetByKey(ctx, "proxy.log_output_config"); err == nil && v != "" {
				var cfg struct {
					Caddy struct {
						Mode string `json:"mode"`
					} `json:"caddy"`
				}
				if json.Unmarshal([]byte(v), &cfg) == nil && cfg.Caddy.Mode != "" {
					caddyLogMode = cfg.Caddy.Mode
				}
			}
		}
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
			// Build reverse_proxy block
			if site.Path == "" || site.Path == "/" {
				sb.WriteString("\treverse_proxy {\n")
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
			sb.WriteString("\t}\n")
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

func (m *CaddyManager) resolvePath(path string) string {
	if path == "" {
		return path
	}
	if filepath.IsAbs(path) {
		return path
	}
	base := filepath.Base(m.dataDir)
	if strings.HasPrefix(path, base+"/") {
		path = strings.TrimPrefix(path, base+"/")
	}
	return filepath.Join(m.dataDir, path)
}

func (m *CaddyManager) proxyHeaders(site *entity.CaddySite) string {
	var sb strings.Builder
	switch site.Template {
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
	port = caddyCompatPort(port, true, proxySandboxCompatMode(m.dataDir))
	upstream := site.Upstream
	if upstream == "" {
		upstream = "127.0.0.1:8080"
	}
	proto := site.L4Protocol
	if proto == "" {
		proto = "tcp"
	}
	sb.WriteString(fmt.Sprintf("# L4 %s forward on :%s\n", proto, port))
	sb.WriteString(fmt.Sprintf(":%s {\n", port))
	sb.WriteString("\troute {\n")
	sb.WriteString(fmt.Sprintf("\t\tproxy %s\n", upstream))
	if site.ProxyProtocol {
		sb.WriteString("\t\tproxy_protocol\n")
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

// GetSNISettings reads SNI settings from database.
func (m *CaddyManager) GetSNISettings(ctx context.Context) (*SNISettings, error) {
	settings := &SNISettings{
		Enabled:            false,
		ListenPort:         "443",
		LocalPort:          "8443",
		ProxyProtocolRules: []string{"127.0.0.0/8", "::1/128"},
	}
	if m.settingStore == nil {
		return settings, nil
	}
	if v, err := m.settingStore.GetByKey(ctx, "proxy.sni_enabled"); err == nil && v == "true" {
		settings.Enabled = true
	}
	if v, err := m.settingStore.GetByKey(ctx, "proxy.sni_listen_port"); err == nil && v != "" {
		settings.ListenPort = v
	}
	if v, err := m.settingStore.GetByKey(ctx, "proxy.sni_local_port"); err == nil && v != "" {
		settings.LocalPort = v
	}
	if v, err := m.settingStore.GetByKey(ctx, "proxy.proxy_protocol_rules"); err == nil && v != "" {
		var rules []string
		if json.Unmarshal([]byte(v), &rules) == nil && len(rules) > 0 {
			settings.ProxyProtocolRules = rules
		}
	}
	return settings, nil
}

// SaveSNISettings saves SNI settings to database.
func (m *CaddyManager) SaveSNISettings(ctx context.Context, settings *SNISettings) error {
	if m.settingStore == nil {
		return nil
	}
	enabled := "false"
	if settings.Enabled {
		enabled = "true"
	}
	if err := m.settingStore.Set(ctx, "proxy.sni_enabled", enabled); err != nil {
		return err
	}
	if settings.ListenPort == "" {
		settings.ListenPort = "443"
	}
	if settings.LocalPort == "" {
		settings.LocalPort = "8443"
	}
	if err := m.settingStore.Set(ctx, "proxy.sni_listen_port", settings.ListenPort); err != nil {
		return err
	}
	if err := m.settingStore.Set(ctx, "proxy.sni_local_port", settings.LocalPort); err != nil {
		return err
	}
	// Save proxy_protocol rules
	if len(settings.ProxyProtocolRules) > 0 {
		data, _ := json.Marshal(settings.ProxyProtocolRules)
		return m.settingStore.Set(ctx, "proxy.proxy_protocol_rules", string(data))
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
