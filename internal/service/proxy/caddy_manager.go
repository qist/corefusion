package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
	// nodeServerRepo is used to detect mirror nodes and resolve their primary
	// server ID for config generation. When set, GenerateConfigForServer checks
	// if the target is a mirror and uses the primary's server_id to query sites.
	nodeServerRepo *repository.NodeServerRepository
}

// wafBaseDir returns the unified WAF rules directory: dataDir/proxy/waf-rules
func (m *CaddyManager) wafBaseDir() string {
	return filepath.Join(m.dataDir, "proxy", "waf-rules")
}

// wafRuleDir returns the WAF rules directory (same as wafBaseDir for compatibility).
func (m *CaddyManager) wafRuleDir() string {
	return m.wafBaseDir()
}

// SetServerID sets the server ID for per-node site filtering.
func (m *CaddyManager) SetServerID(id int64) { m.serverID = id }

// SetIsLocal sets whether this manager is operating on the local node.
func (m *CaddyManager) SetIsLocal(v bool) { m.isLocal = v }

// SetNodeServerRepo sets the node server repository for mirror node detection.
// When set, GenerateConfigForServer will check if the target is a mirror node
// and use its primary's server_id to query caddy_sites, SNI settings, and WAF.
func (m *CaddyManager) SetNodeServerRepo(repo *repository.NodeServerRepository) {
	m.nodeServerRepo = repo
}

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
	if (site.Template == "portal_forward" || site.Template == "subscription_forward") && !m.isLocal {
		return fmt.Errorf("%s sites can only be created on the panel node", site.Template)
	}
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

// resolveSourceServerID checks if the given serverID is a mirror node.
// If it is, returns the primary server's ID and true.
// Otherwise, returns the original serverID and false.
func (m *CaddyManager) resolveSourceServerID(ctx context.Context, serverID int64) (int64, bool) {
	if m.nodeServerRepo == nil {
		return serverID, false
	}
	server, err := m.nodeServerRepo.GetByID(ctx, serverID)
	if err != nil || server == nil {
		return serverID, false
	}
	if server.NodeType == entity.NodeTypeMirror && server.PartnerServerID > 0 {
		return server.PartnerServerID, true
	}
	return serverID, false
}

// GenerateConfigForServer assembles the full Caddyfile from all enabled sites for a specific server.
// If the target server is a mirror node, sites and settings are fetched from
// the primary server's data (partner_server_id). This ensures mirrors get the
// same Caddyfile as the primary without needing separate DB records.
func (m *CaddyManager) GenerateConfigForServer(ctx context.Context, serverID int64) (string, error) {
	// Determine the source server ID for site/SNI/WAF queries.
	// For mirror nodes, use the primary's server_id.
	sourceServerID, isMirror := m.resolveSourceServerID(ctx, serverID)
	if isMirror {
		log.Printf("[CaddyManager] server %d is mirror, using source server %d for sites", serverID, sourceServerID)
	}

	sites, err := m.siteRepo.ListEnabled(ctx, sourceServerID)
	if err != nil {
		return "", err
	}
	// Read SNI settings for proxy_protocol rules from the primary
	sniSettings, _ := m.GetSNISettingsForServer(ctx, sourceServerID)

	var sb strings.Builder
	var httpSites []entity.CaddySite
	var l4Sites []entity.CaddySite
	var realitySites []entity.CaddySite
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
		} else if site.Template == "reality" {
			realitySites = append(realitySites, site)
		} else {
			// portal_forward: only valid on the panel node (isLocal). Remote
			// nodes have no panel process, so skip generating it for them.
			if site.Template == "portal_forward" {
				if !m.isLocal {
					continue
				}
				site.Upstream = m.portalAddr(ctx)
				// If portal has a dedicated port, no path needed (whole domain forwards).
				// If portal shares the panel port, use /p path.
				if site.Path == "" {
					portalPort := ""
					if m.settingStore != nil {
						if v, err := m.settingStore.GetByKey(ctx, "panel.portal_port"); err == nil {
							portalPort = v
						}
					}
					if portalPort == "" || portalPort == "0" {
						site.Path = "/p"
					}
				} else if site.Template == "subscription_forward" {
					// subscription_forward: only valid on the panel node (isLocal).
					// Subscription URLs are served by the panel process on its
					// main port under the /sub path.
					if !m.isLocal {
						continue
					}
					site.Upstream = m.panelAddr(ctx)
					if site.Path == "" {
						site.Path = "/sub"
					}
				}
			}
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

	// CaddyGuard WAF global config — only generated when WAF is enabled for this node.
	// The caddyguardfile adapter injects the Guard handler into every HTTP server.
	// Use relative path so it works on both panel and remote Agents.
	wafEnabled := m.isWAFEnabledForServer(ctx, sourceServerID)
	if wafEnabled {
		sb.WriteString("\tcaddyguard {\n")
		sb.WriteString("\t\trule_dir proxy/waf-rules\n")
		sb.WriteString("\t}\n")
	}
	// When layer4 SNI is active, listener_wrappers (proxy_protocol + tls)
	// are needed for the backend HTTPS server to receive real client IP.
	// We no longer force protocols h1: the real fix for the empty response
	// bug is ensuring non-TLS sites use :80 so layer4 owns :443 exclusively.
	// Keep h1 h2 (no h3) to avoid UDP 443 conflict with layer4.
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
		sb.WriteString("\t\tprotocols h1 h2\n")
		sb.WriteString("\t}\n")
	} else if hasL4 {
		sb.WriteString("\n\tservers {\n")
		sb.WriteString("\t\tprotocols h1 h2\n")
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

	// Global named logger: all sites share a single access log file.
	// Sites reference this logger by name ("log default") instead of each
	// creating their own file. This is the recommended Caddy pattern.
	caddyLogMode := m.getCaddyLogModeForServer(ctx, serverID)
	switch caddyLogMode {
	case "off":
		// no global logger
	case "stdout":
		sb.WriteString("\n\tlog default {\n")
		sb.WriteString("\t\toutput stdout\n")
		sb.WriteString("\t\tformat json\n")
		sb.WriteString("\t}\n")
	default: // "file"
		sb.WriteString("\n\tlog default {\n")
		sb.WriteString("\t\toutput file logs/caddy_access.log\n")
		sb.WriteString("\t\tformat json\n")
		sb.WriteString("\t}\n")
	}

	// Reality: add servers block with listener_wrappers (proxy_protocol + tls) and protocols h1 h2
	if len(realitySites) > 0 {
		for _, rs := range realitySites {
			port := strings.TrimPrefix(rs.Domain, ":")
			if port == "" {
				port = "9443"
			}
			sb.WriteString(fmt.Sprintf("\tservers 127.0.0.1:%s {\n", port))
			sb.WriteString("\t\tlistener_wrappers {\n")
			sb.WriteString("\t\t\tproxy_protocol {\n")
			sb.WriteString("\t\t\t\tallow 127.0.0.1/32\n")
			sb.WriteString("\t\t\t}\n")
			sb.WriteString("\t\t\ttls\n")
			sb.WriteString("\t\t}\n")
			sb.WriteString("\t\tprotocols h1 h2\n")
			sb.WriteString("\t}\n")
		}
	}
	sb.WriteString("}\n\n")

	// Reality: generate independent server blocks (bind 127.0.0.1, tls, no domain required)
	for _, rs := range realitySites {
		port := strings.TrimPrefix(rs.Domain, ":")
		if port == "" {
			port = "9443"
		}
		sb.WriteString(fmt.Sprintf("# Reality :%s\n", port))
		sb.WriteString(fmt.Sprintf(":%s {\n", port))
		sb.WriteString("\tbind 127.0.0.1\n")
		if rs.CertPath != "" && rs.KeyPath != "" {
			sb.WriteString(fmt.Sprintf("\ttls %s %s\n", m.resolvePath(rs.CertPath), m.resolvePath(rs.KeyPath)))
		}
		if rs.CustomConfig != "" {
			for _, line := range strings.Split(strings.TrimSpace(rs.CustomConfig), "\n") {
				sb.WriteString("\t" + strings.TrimSpace(line) + "\n")
			}
		}
		sb.WriteString("}\n\n")
	}

	if len(httpSites) == 0 && !hasL4 {
		sb.WriteString(":80 {\n")
		// 远程节点：签发由**节点上的 Agent 在本机**完成 ✓ → 挑战记录在 Agent 进程内存里，
		// 面板并不知道 ✗ → 必须直接服务**节点本地的挑战目录** ✓（不能 reverse_proxy 回面板 ✗）。
		if !m.isLocal {
			sb.WriteString("\t# ACME HTTP-01 challenge support (node-local, served from disk)\n")
			sb.WriteString("\t@acme path /.well-known/acme-challenge/*\n")
			sb.WriteString("\thandle @acme {\n")
			sb.WriteString("\t\troot * /var/lib/corefusion-agent/acme-challenge\n")
			sb.WriteString("\t\tfile_server\n")
			sb.WriteString("\t}\n")
		}
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
		// Check if this domain has TLS certificates
		hasCert := false
		for _, site := range siteList {
			if site.CertPath != "" && site.KeyPath != "" {
				hasCert = true
				break
			}
		}
		// When layer4 is active, non-TLS sites must use :80 to avoid
		// conflicting with layer4's :443 listener (SO_REUSEPORT causes
		// ~50% of connections to bypass layer4 and return empty responses).
		// Skip domains that already specify a port (e.g. ":80", "0.0.0.0:8080").
		siteAddr := domain
		if hasL4 && !hasCert && !strings.HasPrefix(domain, ":") {
			siteAddr = domain + ":80"
		}
		sb.WriteString(fmt.Sprintf("# %s\n", domain))
		sb.WriteString(fmt.Sprintf("%s {\n", siteAddr))

		for _, site := range siteList {
			if site.CertPath != "" && site.KeyPath != "" {
				sb.WriteString(fmt.Sprintf("\ttls %s %s\n", m.resolvePath(site.CertPath), m.resolvePath(site.KeyPath)))
				break
			}
		}

		// ACME HTTP-01 challenge support
		//
		// 分两种 ✗：本机（面板）→ 挑战**代理回面板** ✓（面板驱动签发 ✓）；
		// 远程节点 → **服务节点本地挑战目录** ✓（签发由节点上的 Agent 做 ✓，面板没有挑战记录 ✗）。
		// 远程节点：签发由**节点上的 Agent 在本机**完成 ✓ → 挑战记录在 Agent 进程内存里，
		// 面板并不知道 ✗ → 必须直接服务**节点本地的挑战目录** ✓（不能 reverse_proxy 回面板 ✗）。
		if !m.isLocal {
			sb.WriteString("\t# ACME HTTP-01 challenge support (node-local, served from disk)\n")
			sb.WriteString("\t@acme path /.well-known/acme-challenge/*\n")
			sb.WriteString("\thandle @acme {\n")
			sb.WriteString("\t\troot * /var/lib/corefusion-agent/acme-challenge\n")
			sb.WriteString("\t\tfile_server\n")
			sb.WriteString("\t}\n")
		}
		if m.isLocal {
			sb.WriteString("\t# ACME HTTP-01 challenge support\n")
			sb.WriteString("\t@acme path /.well-known/acme-challenge/*\n")
			sb.WriteString("\thandle @acme {\n")
			sb.WriteString(fmt.Sprintf("\t\treverse_proxy %s\n", m.panelAddr(ctx)))
			sb.WriteString("\t}\n")
		}

		// Per-site log configuration:
		// - LogEnabled=true + LogPath=""  → use global "default" logger (unified file)
		// - LogEnabled=true + LogPath!=""  → custom per-site log file (overrides global)
		// - LogEnabled=false              → no logging for this site
		if caddyLogMode != "off" {
			siteHasLog := false
			for _, site := range siteList {
				if site.LogEnabled {
					siteHasLog = true
					if site.LogPath != "" {
						// Custom per-site log file
						sb.WriteString("\tlog {\n")
						switch caddyLogMode {
						case "stdout":
							sb.WriteString("\t\toutput stdout\n")
						default:
							sb.WriteString(fmt.Sprintf("\t\toutput file %s\n", site.LogPath))
						}
						if site.LogFormat == "json" {
							sb.WriteString("\t\tformat json\n")
						}
						sb.WriteString("\t}\n")
					} else {
						// Reference the global named logger
						sb.WriteString("\tlog default\n")
					}
					break
				}
			}
			_ = siteHasLog
		}

		for _, site := range siteList {
			// Per-site WAF switch: when WafEnabled is explicitly false,
			// inject `caddyguard waf_enable off` into the site's route block.
			wafBypass := false
			if site.WafEnabled != nil && !*site.WafEnabled {
				wafBypass = true
			}
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
			useRouteWrap := false  // wraps in route { ... } for WAF bypass
			useHandlePath := false // uses handle_path to strip prefix
			if site.Path == "" || site.Path == "/" {
				if wafBypass {
					sb.WriteString("\troute {\n")
					sb.WriteString("\t\tcaddyguard waf_enable off\n")
					sb.WriteString("\t\treverse_proxy {\n")
					useRouteWrap = true
				} else {
					sb.WriteString("\treverse_proxy {\n")
				}
			} else if backendPath == "/" {
				// Backend expects root path: use handle_path to strip the location prefix
				if wafBypass {
					sb.WriteString(fmt.Sprintf("\t@%s {\n", pathToMatcher(site.Path)))
					sb.WriteString(fmt.Sprintf("\t\tpath %s*\n", site.Path))
					sb.WriteString("\t}\n")
					sb.WriteString(fmt.Sprintf("\troute @%s {\n", pathToMatcher(site.Path)))
					sb.WriteString("\t\tcaddyguard waf_enable off\n")
					sb.WriteString(fmt.Sprintf("\t\thandle_path %s/* {\n", strings.TrimSuffix(site.Path, "/")))
					sb.WriteString("\t\t\treverse_proxy {\n")
					useRouteWrap = true
				} else {
					sb.WriteString(fmt.Sprintf("\thandle_path %s/* {\n", strings.TrimSuffix(site.Path, "/")))
					sb.WriteString("\t\treverse_proxy {\n")
				}
				useHandlePath = true
			} else {
				if wafBypass {
					sb.WriteString(fmt.Sprintf("\t@%s {\n", pathToMatcher(site.Path)))
					sb.WriteString(fmt.Sprintf("\t\tpath %s*\n", site.Path))
					sb.WriteString("\t}\n")
					sb.WriteString(fmt.Sprintf("\troute @%s {\n", pathToMatcher(site.Path)))
					sb.WriteString("\t\tcaddyguard waf_enable off\n")
					sb.WriteString(fmt.Sprintf("\t\treverse_proxy @%s {\n", pathToMatcher(site.Path)))
					useRouteWrap = true
				} else {
					sb.WriteString(fmt.Sprintf("\t@%s {\n", pathToMatcher(site.Path)))
					sb.WriteString(fmt.Sprintf("\t\tpath %s*\n", site.Path))
					sb.WriteString("\t}\n")
					sb.WriteString(fmt.Sprintf("\treverse_proxy @%s {\n", pathToMatcher(site.Path)))
				}
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
			if useRouteWrap {
				sb.WriteString("\t}\n") // close route block (WAF bypass wrapper)
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

// portalAddr returns the portal address (host:port) for portal_forward template.
// Reads panel.portal_port from settings; falls back to panel.listen_port.
func (m *CaddyManager) portalAddr(ctx context.Context) string {
	port := ""
	if m.settingStore != nil {
		if v, err := m.settingStore.GetByKey(ctx, "panel.portal_port"); err == nil && v != "" {
			port = v
		}
	}
	if port == "" || port == "0" {
		return m.panelAddr(ctx)
	}
	return "127.0.0.1:" + port
}

// isWAFEnabledForServer checks if WAF is enabled for a specific server.
// Reads the per-node WAF switch from settings (proxy.waf_enabled.<serverID>).
// When WAF is disabled, no caddyguard block is generated in the Caddyfile.
func (m *CaddyManager) isWAFEnabledForServer(ctx context.Context, serverID int64) bool {
	if m.settingStore == nil {
		return false
	}
	key := "proxy.waf_enabled"
	if serverID > 1 {
		key = fmt.Sprintf("proxy.waf_enabled.%d", serverID)
	}
	v, err := m.settingStore.GetByKey(ctx, key)
	if err != nil || v == "" {
		return false
	}
	return v == "true"
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

	// Try per-node keys first.
	// IMPORTANT: if the per-node key exists (even if value is "false"), use it.
	// Only fall back to the global key when the per-node key does NOT exist.
	// This prevents a remote node from inheriting the local node's "true" when
	// the remote node has explicitly disabled SNI.
	enabledKey := perNodeSNISettingsKey("proxy.sni_enabled", serverID)
	if v, err := m.settingStore.GetByKey(ctx, enabledKey); err == nil {
		// per-node key exists — use its value directly
		settings.Enabled = v == "true"
	} else if serverID > 1 {
		// per-node key doesn't exist — fall back to global key
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

// SanitizeLogName converts a domain to a clean log file name.
// :80 -> default, _ -> default, www.example.com -> www.example.com
func SanitizeLogName(domain string) string {
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

// --- WAF Management moved to service/waf/ ---
// All WAF rule management (ReadWAFConfig, SaveWAFConfig, ListWAFRules, etc.)
// has been moved to internal/service/waf/service.go.
// The manager only retains wafBaseDir/wafRuleDir for config generation path references.

// StripPort removes the port from a domain string (e.g. "example.com:8443" -> "example.com").
// CaddyGuard's getDomain() strips port from r.Host before matching against domain.json,
// so domain.json keys must not contain ports.
func StripPort(domain string) string {
	if idx := strings.LastIndex(domain, ":"); idx > 0 {
		return domain[:idx]
	}
	return domain
}
