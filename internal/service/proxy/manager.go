package proxy

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"corefusion/internal/runtime/healthcheck"
	"corefusion/internal/runtime/process"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
	"corefusion/pkg/netutil"
)

// ProxyGuardianConfig holds persisted guardian settings for a proxy.
type ProxyGuardianConfig struct {
	AutoStart   bool
	AutoRestart bool
	MaxRestarts int
}

// ProxyType represents the type of reverse proxy.
type ProxyType string

const (
	ProxyNginx ProxyType = "nginx"
	ProxyCaddy ProxyType = "caddy"
)

// ProxyInfo contains information about a proxy installation.
type ProxyInfo struct {
	Type       ProxyType `json:"type"`
	Name       string    `json:"name"`
	Installed  bool      `json:"installed"`
	Version    string    `json:"version"`
	Path       string    `json:"path"`
	ConfigPath string    `json:"config_path"`
	Status     string    `json:"status"`
	Managed    bool      `json:"managed"`
}

// SettingStore provides access to system settings.
type SettingStore interface {
	GetByKey(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
}

// CrashLogInserter inserts crash log records. Implemented by repository.CrashLogRepository.
type CrashLogInserter interface {
	InsertCrashLog(ctx context.Context, proxyType string, attempt, maxRestarts int)
}

// SetCrashLogRepo sets the crash log repository for recording proxy crash events.
func (m *Manager) SetCrashLogRepo(repo CrashLogInserter) {
	m.crashLogRepo = repo
}

// Manager handles proxy program management.
type Manager struct {
	dataDir      string
	binDir       string
	configDir    string
	siteRepo     *repository.ProxySiteRepository
	settingStore SettingStore
	crashLogRepo CrashLogInserter
	serverID     int64
	isLocal      bool
	// Config generators (set by router to use new separated managers)
	caddyConfigGen func(ctx context.Context) (string, error)
	nginxConfigGen func(ctx context.Context) (string, error)
	// onProxyInstalled is called after a proxy binary is installed/updated.
	// Used to re-sync WAF configs from DB so user customizations survive
	// the tar.gz overwrite during nginx reinstall.
	onProxyInstalled func(ctx context.Context, proxyType string)
	// Stdout log broker for real-time log viewing
	logBrokers map[ProxyType]*LogBroker
	// Process guardian for auto-restart on crash (shared healthcheck)
	guardians map[ProxyType]*healthcheck.Guardian
	// Tracked PIDs for precise process killing (avoids pkill -f killing wrong processes)
	processPIDs map[ProxyType]int
	// mu protects logBrokers and guardians from concurrent access
	// (StartProxy can be called from Guardian's crash-recovery goroutine)
	mu sync.Mutex
}

// SetServerID sets the server ID for per-node site filtering.
func (m *Manager) SetServerID(id int64) { m.serverID = id }

// SetIsLocal sets whether this manager is operating on the local node.
func (m *Manager) SetIsLocal(v bool) { m.isLocal = v }

// SetCaddyConfigGen sets the Caddy config generator function.
func (m *Manager) SetCaddyConfigGen(fn func(ctx context.Context) (string, error)) {
	m.caddyConfigGen = fn
}

// SetNginxConfigGen sets the Nginx config generator function.
func (m *Manager) SetNginxConfigGen(fn func(ctx context.Context) (string, error)) {
	m.nginxConfigGen = fn
}

// SetOnProxyInstalled sets a callback invoked after a proxy binary install.
// The router wires this to WAF service LandRulesForServer so that user's
// custom WAF config (stored in DB) overwrites the tar.gz default config.lua.
func (m *Manager) SetOnProxyInstalled(fn func(ctx context.Context, proxyType string)) {
	m.onProxyInstalled = fn
}

// getNginxAccessLogPath returns the nginx access log path from settings.
func (m *Manager) getNginxAccessLogPath() string {
	if m.settingStore == nil {
		return filepath.Join(m.dataDir, "logs", "access.log")
	}
	v, err := m.settingStore.GetByKey(context.Background(), "proxy.nginx_settings")
	if err != nil || v == "" {
		return filepath.Join(m.dataDir, "logs", "access.log")
	}
	var settings struct {
		AccessLog     bool   `json:"access_log"`
		AccessLogPath string `json:"access_log_path"`
	}
	if json.Unmarshal([]byte(v), &settings) == nil && settings.AccessLog && settings.AccessLogPath != "" {
		if filepath.IsAbs(settings.AccessLogPath) {
			return settings.AccessLogPath
		}
		return filepath.Join(m.dataDir, settings.AccessLogPath)
	}
	return filepath.Join(m.dataDir, "logs", "access.log")
}

// GetLogBroker returns the log broker for a proxy type, or nil if not running.
func (m *Manager) GetLogBroker(proxyType ProxyType) *LogBroker {
m.mu.Lock()
defer m.mu.Unlock()
if m.logBrokers == nil {
return nil
}
return m.logBrokers[proxyType]
}

// githubAccelDomain reads the active GitHub acceleration domain from settings.
func (m *Manager) githubAccelDomain() string {
	if m.settingStore == nil {
		return ""
	}
	val, err := m.settingStore.GetByKey(context.Background(), "github.acceleration")
	if err != nil || val == "" {
		return ""
	}
	var cfg struct {
		Enabled bool `json:"enabled"`
		Domains []struct {
			URL     string `json:"url"`
			Enabled bool   `json:"enabled"`
		} `json:"domains"`
	}
	if json.Unmarshal([]byte(val), &cfg) != nil || !cfg.Enabled {
		return ""
	}
	for _, d := range cfg.Domains {
		if d.Enabled && d.URL != "" {
			return d.URL
		}
	}
	return ""
}

// githubProxyURL reads the optional HTTP/SOCKS proxy URL from the GitHub
// acceleration settings. Returns "" when unset.
func (m *Manager) githubProxyURL() string {
	if m.settingStore == nil {
		return ""
	}
	val, err := m.settingStore.GetByKey(context.Background(), "github.acceleration")
	if err != nil || val == "" {
		return ""
	}
	var cfg struct {
		ProxyURL string `json:"proxy_url"`
	}
	if json.Unmarshal([]byte(val), &cfg) != nil {
		return ""
	}
	return cfg.ProxyURL
}

// githubHTTPClient creates an HTTP client with GitHub acceleration and proxy.
func (m *Manager) githubHTTPClient(timeout time.Duration) *http.Client {
	return netutil.GitHubHTTPClientWithProxy(timeout, m.githubAccelDomain(), m.githubProxyURL())
}

// lookupNginxUser returns the UID and GID of the nginx user.
func lookupNginxUser() (int, int, error) {
	u, err := user.Lookup("nginx")
	if err != nil {
		return 0, 0, err
	}
	uid := 0
	gid := 0
	fmt.Sscanf(u.Uid, "%d", &uid)
	fmt.Sscanf(u.Gid, "%d", &gid)
	return uid, gid, nil
}

// NewManager creates a new proxy manager.
func NewManager(dataDir string, siteRepo *repository.ProxySiteRepository, settingStore SettingStore) *Manager {
	absDir := dataDir
	if !filepath.IsAbs(dataDir) {
		if abs, err := filepath.Abs(dataDir); err == nil {
			absDir = abs
		}
	}
	return &Manager{
		dataDir:      absDir,
		binDir:       filepath.Join(absDir, "binaries", "proxy"),
		configDir:    filepath.Join(absDir, "proxy"),
		siteRepo:     siteRepo,
		settingStore: settingStore,
		logBrokers:   make(map[ProxyType]*LogBroker),
		guardians:    make(map[ProxyType]*healthcheck.Guardian),
		processPIDs:  make(map[ProxyType]int),
	}
}

// proxyProcessName returns the binary base name used for precise process
// detection. For managed binaries this is the platform-specific name (e.g.
// "caddy-linux-amd64"), which avoids false positives from pgrep -f matching
// unrelated processes that merely contain "caddy" in their path. For
// system-installed binaries it returns the short name (e.g. "caddy").
func (m *Manager) proxyProcessName(proxyType ProxyType) string {
	name := string(proxyType)
	path, _, _ := m.detectBinary(name)
	if path != "" {
		return filepath.Base(path)
	}
	return name
}

// DetectProxy checks if a proxy is installed and returns its info.
func (m *Manager) DetectProxy(proxyType ProxyType) *ProxyInfo {
	info := &ProxyInfo{
		Type: proxyType,
		Name: string(proxyType),
	}

	switch proxyType {
	case ProxyNginx:
		info.Name = "Nginx"
		info.ConfigPath = m.toRelative(filepath.Join(m.configDir, "nginx.conf"))
		path, version, managed := m.detectBinary("nginx")
		info.Path = m.toRelative(path)
		info.Version = version
		info.Installed = path != ""
		info.Managed = managed
		info.Status = m.getProcessStatus(m.proxyProcessName(ProxyNginx))
	case ProxyCaddy:
		info.Name = "Caddy"
		info.ConfigPath = m.toRelative(filepath.Join(m.configDir, "Caddyfile"))
		path, version, managed := m.detectBinary("caddy")
		info.Path = m.toRelative(path)
		info.Version = version
		info.Installed = path != ""
		info.Managed = managed
		info.Status = m.getProcessStatus(m.proxyProcessName(ProxyCaddy))
	}

	return info
}

// toRelative converts an absolute path to relative from dataDir.
func (m *Manager) toRelative(path string) string {
	if path == "" {
		return path
	}
	if rel, err := filepath.Rel(m.dataDir, path); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return path
}

// detectBinary finds a binary, checking managed dir first, then system paths.
func (m *Manager) detectBinary(name string) (path, version string, managed bool) {
	managedPath := filepath.Join(m.binDir, m.binaryName(name))
	if _, err := os.Stat(managedPath); err == nil {
		return managedPath, m.getVersion(managedPath), true
	}

	for _, p := range m.systemPaths(name) {
		if _, err := os.Stat(p); err == nil {
			return p, m.getVersion(p), false
		}
	}

	if p, err := exec.LookPath(name); err == nil {
		return p, m.getVersion(p), false
	}

	return "", "", false
}

func (m *Manager) binaryName(name string) string {
	if runtime.GOOS == "windows" {
		return name + "-" + runtime.GOOS + "-" + runtime.GOARCH + ".exe"
	}
	return name + "-" + runtime.GOOS + "-" + runtime.GOARCH
}

func (m *Manager) systemPaths(name string) []string {
	switch runtime.GOOS {
	case "linux":
		return []string{
			"/usr/sbin/" + name, "/usr/bin/" + name,
			"/usr/local/bin/" + name, "/usr/local/sbin/" + name,
		}
	case "darwin":
		return []string{"/usr/local/bin/" + name, "/opt/homebrew/bin/" + name}
	case "windows":
		exe := name + ".exe"
		return []string{
			filepath.Join(os.Getenv("ProgramFiles"), name, exe),
			filepath.Join(os.Getenv("ProgramFiles(x86)"), name, exe),
		}
	}
	return nil
}

func (m *Manager) getVersion(path string) string {
	for _, flag := range []string{"version", "-v", "--version"} {
		cmd := exec.Command(path, flag)
		// Use separate stdout/stderr buffers because nginx writes its version
		// to stderr (not stdout), while caddy writes to stdout but may emit
		// JSON-formatted warning logs on stderr when HOME/XDG_CONFIG_HOME are
		// unset (common in systemd service environments).
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		if err != nil {
			continue
		}
		// Combine stdout and stderr, then let parseProxyVersion find the
		// version line among any JSON log noise.
		raw := stdout.String()
		if strings.TrimSpace(raw) == "" {
			raw = stderr.String()
		} else if strings.TrimSpace(stderr.String()) != "" {
			// Prepend stderr in case stdout only has log output (unlikely
			// but keeps the search robust).
			raw = raw + "\n" + stderr.String()
		}
		v := parseProxyVersion(raw)
		if v != "" {
			return v
		}
	}
	return "unknown"
}

// parseProxyVersion extracts a clean version string from proxy version output.
//
// nginx -v stderr:  "nginx version: nginx/1.24.0"
// caddy version:    "v2.11.4 h1:XKxkMTgNSizEvKG6QHue6cAsFOteU2qA61w2tKkCWi0="
//
// In systemd service environments, caddy may also emit JSON log warnings on
// stderr (zerolog format), e.g.:
//   {"level":"warn","ts":...,"msg":"unable to determine directory ..."}
// These lines must be skipped to find the actual version line.
func parseProxyVersion(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	// Scan lines, skip JSON log lines and empty lines, find the version line.
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Skip JSON log lines (caddy zerolog output starts with '{')
		if line[0] == '{' {
			continue
		}

		// nginx: "nginx version: nginx/1.24.0" → "1.24.0"
		if strings.HasPrefix(line, "nginx version:") {
			rest := strings.TrimSpace(strings.TrimPrefix(line, "nginx version:"))
			if idx := strings.Index(rest, "/"); idx >= 0 {
				return rest[idx+1:]
			}
			return rest
		}

		// caddy: "v2.11.4 h1:hash..." → "v2.11.4" (take first token)
		if idx := strings.Index(line, " "); idx > 0 {
			return line[:idx]
		}

		// Single-token line — return as-is with length guard.
		if len(line) > 40 {
			line = line[:40]
		}
		return line
	}
	return ""
}

func (m *Manager) getProcessStatus(name string) string {
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command("tasklist", "/FI", "IMAGENAME eq "+name+".exe", "/NH")
		out, err := cmd.Output()
		if err == nil && strings.Contains(string(out), name+".exe") {
			return "running"
		}
		return "stopped"
	default:
		cmd := exec.Command("pgrep", "-f", name)
		out, err := cmd.Output()
		if err != nil {
			return "stopped"
		}
		for _, pidStr := range strings.Fields(strings.TrimSpace(string(out))) {
			statPath := fmt.Sprintf("/proc/%s/stat", pidStr)
			data, err := os.ReadFile(statPath)
			if err != nil {
				continue
			}
			str := string(data)
			if idx := strings.LastIndex(str, ")"); idx >= 0 && idx+2 < len(str) {
				state := str[idx+2 : idx+3]
				if state != "Z" {
					return "running"
				}
			}
		}
		return "stopped"
	}
}

// GenerateConfig assembles the full Caddyfile from all enabled caddy sites.
func (m *Manager) GenerateConfig(ctx context.Context) (string, error) {
	sites, err := m.siteRepo.ListEnabledByType(ctx, m.serverID, "caddy")
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	var httpSites []entity.ProxySite
	var l4Sites []entity.ProxySite
	sniByPort := make(map[string][]entity.ProxySite) // port -> SNI sites

	// Separate L4/SNI sites from HTTP sites
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

	// Check if any SNI/L4 site has proxy_protocol enabled
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

	// Global options block (includes layer4 if needed)
	sb.WriteString("{\n")
	sb.WriteString("\tauto_https off\n")
	if needPP {
		sb.WriteString("\n\tservers {\n")
		sb.WriteString("\t\tlistener_wrappers {\n")
		sb.WriteString("\t\t\tproxy_protocol {\n")
		sb.WriteString("\t\t\t\tallow 127.0.0.0/8\n")
		sb.WriteString("\t\t\t\tallow 10.0.0.0/8\n")
		sb.WriteString("\t\t\t\tallow 172.16.0.0/12\n")
		sb.WriteString("\t\t\t\tallow 192.168.0.0/16\n")
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
	// Log configuration - check log output mode (per-node)
	caddyLogMode := "file"
	if m.settingStore != nil {
		logKey := "proxy.log_output_config"
		if m.serverID > 1 {
			logKey = fmt.Sprintf("%s.%d", logKey, m.serverID)
		}
		v, err := m.settingStore.GetByKey(ctx, logKey)
		if err != nil || v == "" {
			// Fall back to global key
			v, err = m.settingStore.GetByKey(ctx, "proxy.log_output_config")
		}
		if err == nil && v != "" {
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
	// Global named logger: all sites share a single access log file.
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
	if hasL4 {
		sb.WriteString("\n\tlayer4 {\n")
		// L4 sites - each is its own block
		for _, site := range l4Sites {
			block := m.generateL4Block(&site)
			for _, line := range strings.Split(strings.TrimRight(block, "\n"), "\n") {
				sb.WriteString("\t\t" + line + "\n")
			}
		}
		// SNI sites - grouped by port, matchers first then fallback at end
		for port, sniSites := range sniByPort {
			localPort := sniSites[0].LocalPort
			if localPort == "" {
				localPort = "8443"
			}
			sb.WriteString(fmt.Sprintf("\t\t:%s {\n", port))
			// Collect fallback rules and matcher rules separately
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
				// "fallback" domain = catch-all rule (no SNI matcher)
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
			// Fallback route at the end - no SNI matcher, catches all unmatched
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
	domainSites := make(map[string][]entity.ProxySite)
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

		// TLS from first site that has it
		for _, site := range siteList {
			if site.CertPath != "" && site.KeyPath != "" {
				sb.WriteString(fmt.Sprintf("\ttls %s %s\n", stripDataDirPrefix(site.CertPath, m.dataDir), stripDataDirPrefix(site.KeyPath, m.dataDir)))
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

		// Path-based routing
		for _, site := range siteList {
			// Static file server: root + file_server instead of reverse_proxy
			if site.Template == "static" {
				rootDir := strings.TrimSpace(site.Upstream)
				if rootDir == "" {
					rootDir = "/var/www"
				}
				sb.WriteString(fmt.Sprintf("\troot * %s\n", rootDir))
				sb.WriteString("\tfile_server\n")
				if site.CustomConfig != "" {
					for _, line := range strings.Split(strings.TrimSpace(site.CustomConfig), "\n") {
						sb.WriteString("\t" + strings.TrimSpace(line) + "\n")
					}
				}
				continue
			}
			if site.Path == "" || site.Path == "/" {
				sb.WriteString(fmt.Sprintf("\treverse_proxy %s {\n", site.Upstream))
				sb.WriteString(m.proxyHeaders(&site))
				sb.WriteString("\t}\n")
			} else {
				sb.WriteString(fmt.Sprintf("\t@%s {\n", pathToMatcher(site.Path)))
				sb.WriteString(fmt.Sprintf("\t\tpath %s*\n", site.Path))
				sb.WriteString("\t}\n")
				sb.WriteString(fmt.Sprintf("\treverse_proxy @%s %s {\n", pathToMatcher(site.Path), site.Upstream))
				sb.WriteString(m.proxyHeaders(&site))
				sb.WriteString("\t}\n")
			}
			// Site-level custom directives (outside reverse_proxy)
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

// stripDataDirPrefix removes the dataDir base name prefix from a path.
// e.g., dataDir="/opt/corefusion/data", path="data/certs/x.pem" -> "certs/x.pem"
// e.g., dataDir="/opt/corefusion/data", path="/opt/corefusion/data/certs/x.pem" -> "certs/x.pem"
func stripDataDirPrefix(path, dataDir string) string {
	if path == "" {
		return path
	}
	// Handle absolute paths: strip dataDir prefix if the path is inside dataDir
	if filepath.IsAbs(path) {
		if rel, err := filepath.Rel(dataDir, path); err == nil && !strings.HasPrefix(rel, "..") {
			return rel
		}
		return path
	}
	// Handle relative paths: strip "data/" prefix if it matches dataDir's base name
	base := filepath.Base(dataDir)
	if strings.HasPrefix(path, base+"/") {
		return strings.TrimPrefix(path, base+"/")
	}
	return path
}

// resolvePath resolves a relative cert path against dataDir.
// Absolute paths are returned as-is for backward compatibility.
// Relative paths are resolved against dataDir.
func (m *Manager) resolvePath(path string) string {
	if path == "" {
		return path
	}
	// Absolute path - return as-is
	if filepath.IsAbs(path) {
		return path
	}
	// Strip "data/" prefix if it matches dataDir's base name
	// e.g., dataDir="/opt/corefusion/data", path="data/certs/x.pem" -> "certs/x.pem"
	base := filepath.Base(m.dataDir)
	if strings.HasPrefix(path, base+"/") {
		path = strings.TrimPrefix(path, base+"/")
	}
	return filepath.Join(m.dataDir, path)
}

func (m *Manager) proxyHeaders(site *entity.ProxySite) string {
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
	default:
		sb.WriteString("\t\theader_up X-Real-IP {remote_host}\n")
		sb.WriteString("\t\theader_up X-Forwarded-For {remote_host}\n")
		sb.WriteString("\t\theader_up X-Forwarded-Proto {scheme}\n")
	}
	// Note: proxy_protocol is only for layer4 blocks, not reverse_proxy
	// HTTP sites don't support proxy_protocol in Caddy reverse_proxy directive
	return sb.String()
}

// generateL4Block generates a layer4 block for L4 sites.
func (m *Manager) generateL4Block(site *entity.ProxySite) string {
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

func pathToMatcher(path string) string {
	s := strings.TrimPrefix(path, "/")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "-", "_")
	return s
}

// ListSites returns all proxy sites from the database.
func (m *Manager) ListSites(ctx context.Context) ([]entity.ProxySite, error) {
	return m.siteRepo.List(ctx, m.serverID)
}

// ListSitesByType returns proxy sites filtered by proxy type.
func (m *Manager) ListSitesByType(ctx context.Context, proxyType string) ([]entity.ProxySite, error) {
	return m.siteRepo.ListByType(ctx, m.serverID, proxyType)
}

// CreateSite creates a new proxy site in the database.
func (m *Manager) CreateSite(ctx context.Context, site *entity.ProxySite) error {
	if (site.Template == "portal_forward" || site.Template == "subscription_forward") && !m.isLocal {
		return fmt.Errorf("%s sites can only be created on the panel node", site.Template)
	}
	if site.ServerID == 0 {
		site.ServerID = m.serverID
	}
	return m.siteRepo.Create(ctx, site)
}

// GetSite returns a proxy site by ID.
func (m *Manager) GetSite(ctx context.Context, id int64) (*entity.ProxySite, error) {
	return m.siteRepo.GetByID(ctx, id)
}

// UpdateSite updates a proxy site in the database.
func (m *Manager) UpdateSite(ctx context.Context, site *entity.ProxySite) error {
	return m.siteRepo.Update(ctx, site)
}

// DeleteSite deletes a proxy site from the database.
func (m *Manager) DeleteSite(ctx context.Context, id int64) error {
	return m.siteRepo.Delete(ctx, id)
}

// SaveAndApply saves config to file and returns it.
func (m *Manager) SaveAndApply(ctx context.Context) (string, error) {
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


// InstallProxy installs a proxy using system package manager or download.
func (m *Manager) InstallProxy(ctx context.Context, proxyType ProxyType) error {
	switch proxyType {
	case ProxyNginx:
		return m.installNginx(ctx)
	case ProxyCaddy:
		return m.installCaddy(ctx)
	}
	return fmt.Errorf("unsupported proxy type: %s", proxyType)
}

// DownloadAndInstallProxy downloads a specific proxy binary version via the given URL,
// installs it, and restarts the proxy if it was running.
func (m *Manager) DownloadAndInstallProxy(ctx context.Context, proxyType ProxyType, downloadURL string) error {
	// Check if proxy is currently running
	wasRunning := m.DetectProxy(proxyType).Status == "running"

	// Download the specific version
	if _, err := m.DownloadAsset(ctx, downloadURL, string(proxyType)); err != nil {
		return fmt.Errorf("download %s: %w", proxyType, err)
	}

	// If proxy was running, restart it to pick up the new binary
	if wasRunning {
		log.Printf("proxy %s: was running, restarting after version update", proxyType)
		if err := m.RestartProxy(ctx, proxyType); err != nil {
			return fmt.Errorf("restart %s after update: %w", proxyType, err)
		}
	}

	return nil
}

func (m *Manager) installNginx(ctx context.Context) error {
	releases, err := m.ListNginxReleases("qist/nginx-binaries")
	if err != nil {
		return fmt.Errorf("list nginx releases: %w", err)
	}
	if len(releases) == 0 || len(releases[0].Assets) == 0 {
		return fmt.Errorf("no matching nginx release found for %s/%s", detectNginxDistro(), runtime.GOARCH)
	}

	asset := releases[0].Assets[0]
	if _, err = m.DownloadAsset(ctx, asset.BrowserDownloadURL, "nginx"); err != nil {
		return fmt.Errorf("download nginx: %w", err)
	}

	// Generate config from DB (database = source of truth).
	// On Panel mode, nginxConfigGen generates from DB.
	// On Agent mode, config is synced by panel via RPC — skip generation.
	if m.nginxConfigGen != nil {
		config, err := m.nginxConfigGen(ctx)
		if err != nil {
			return fmt.Errorf("generate nginx config: %w", err)
		}
		if err := m.WriteConfigFile(ProxyNginx, config); err != nil {
			return fmt.Errorf("write nginx config: %w", err)
		}
	}

	return nil
}

func (m *Manager) installCaddy(ctx context.Context) error {
	releases, err := m.ListReleases("qist/caddy-binaries")
	if err != nil {
		return fmt.Errorf("list releases: %w", err)
	}
	if len(releases) == 0 || len(releases[0].Assets) == 0 {
		return fmt.Errorf("no matching release found")
	}

	asset := releases[0].Assets[0]
	if _, err = m.DownloadAsset(ctx, asset.BrowserDownloadURL, "caddy"); err != nil {
		return fmt.Errorf("download caddy: %w", err)
	}

	// Generate config from DB (database = source of truth).
	// On Panel mode, caddyConfigGen generates from DB.
	// On Agent mode, config is synced by panel via RPC — skip generation.
	if m.caddyConfigGen != nil {
		config, err := m.caddyConfigGen(ctx)
		if err != nil {
			return fmt.Errorf("generate caddy config: %w", err)
		}
		if err := m.WriteConfigFile(ProxyCaddy, config); err != nil {
			return fmt.Errorf("write caddy config: %w", err)
		}
	}

	return nil
}

// checkNginxUser verifies the nginx system user exists, creating it if missing.
func checkNginxUser() error {
	_, err := user.Lookup("nginx")
	if err == nil {
		return nil // user exists
	}
	// Create nginx system user
	cmd := exec.Command("useradd", "-r", "-s", "/sbin/nologin", "nginx")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("nginx user not found and creation failed: %s (run: useradd -r -s /sbin/nologin nginx)", strings.TrimSpace(string(out)))
	}
	log.Println("created nginx system user")
	return nil
}

// ensureNginxDependencies checks and creates required Nginx files/directories.
func (m *Manager) ensureNginxDependencies() error {
	// Create required directories
	dirs := []string{
		filepath.Join(m.dataDir, "logs"),
		filepath.Join(m.dataDir, "runtime"),
		filepath.Join(m.dataDir, "proxy"),
		filepath.Join(m.dataDir, "proxy", "lua"),
		filepath.Join(m.dataDir, "temp"),
		filepath.Join(m.dataDir, "temp", "client_body_temp"),
		filepath.Join(m.dataDir, "temp", "proxy_temp"),
		filepath.Join(m.dataDir, "temp", "fastcgi_temp"),
		filepath.Join(m.dataDir, "temp", "uwsgi_temp"),
		filepath.Join(m.dataDir, "temp", "scgi_temp"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create dir %s: %w", dir, err)
		}
	}
	// Ensure proxy directory is accessible by nginx worker (0755, not 0750)
	// nginx.conf and lua files live here; worker process (user nginx) must be
	// able to read them. Without this, access_by_lua_file fails with Permission denied.
	proxyDir := filepath.Join(m.dataDir, "proxy")
	os.Chmod(proxyDir, 0755)

	// Set temp directory, lua directory, and logs directory ownership to nginx user
	if nginxUID, nginxGID, err := lookupNginxUser(); err == nil {
		// Chown logs directory to nginx user so WAF (running as nginx) can create
		// daily log files via io.open(). Don't chown existing log files — xray/caddy
		// logs are written by root and should stay root-owned.
		logsDir := filepath.Join(m.dataDir, "logs")
		os.MkdirAll(logsDir, 0775)
		os.Chown(logsDir, nginxUID, nginxGID)
		// Chown temp directory tree
		tempDir := filepath.Join(m.dataDir, "temp")
		filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
			if err == nil {
				if chownErr := os.Chown(path, nginxUID, nginxGID); chownErr != nil {
					log.Printf("proxy nginx: failed to chown %s to %d:%d: %v", path, nginxUID, nginxGID, chownErr)
				}
			}
			return nil
		})
		// Chown lua directory tree so nginx worker can read lua/waf files
		luaDir := filepath.Join(proxyDir, "lua")
		filepath.Walk(luaDir, func(path string, info os.FileInfo, err error) error {
			if err == nil {
				if chownErr := os.Chown(path, nginxUID, nginxGID); chownErr != nil {
					log.Printf("proxy nginx: failed to chown %s to %d:%d: %v", path, nginxUID, nginxGID, chownErr)
				}
			}
			return nil
		})
	}

	// Ensure mime.types exists in proxy config directory (same as nginx.conf)
	mimePath := filepath.Join(proxyDir, "mime.types")
	if _, err := os.Stat(mimePath); os.IsNotExist(err) {
		if err := os.WriteFile(mimePath, []byte(defaultMimeTypes), 0644); err != nil {
			return fmt.Errorf("create mime.types: %w", err)
		}
		log.Println("created default mime.types")
	}

	return nil
}

const defaultMimeTypes = `types {
    text/html                             html htm shtml;
    text/css                              css;
    text/xml                              xml;
    image/gif                             gif;
    image/jpeg                            jpeg jpg;
    application/javascript                js;
    application/atom+xml                  atom;
    application/rss+xml                   rss;
    text/plain                            txt;
    application/json                      json;
    application/octet-stream              bin exe dll;
    image/png                             png;
    image/svg+xml                         svg svgz;
    image/webp                            webp;
    application/x-font-ttf               ttf;
    application/font-woff                 woff;
    application/font-woff2                woff2;
}
`

// StartProxy starts a proxy service.
func (m *Manager) StartProxy(ctx context.Context, proxyType ProxyType) error {
	info := m.DetectProxy(proxyType)
	if !info.Installed {
		return fmt.Errorf("%s is not installed", proxyType)
	}

	// Prevent nginx and caddy from running simultaneously — they share ports 80/443.
	var otherType ProxyType
	switch proxyType {
	case ProxyNginx:
		otherType = ProxyCaddy
	case ProxyCaddy:
		otherType = ProxyNginx
	}
	otherInfo := m.DetectProxy(otherType)
	if otherInfo.Status == "running" {
		return fmt.Errorf("%s is running, please stop it first (ports 80/443 conflict)", otherType)
	}

	// Check if already running
	if info.Status == "running" {
		// If guardian already exists, just skip
		m.mu.Lock()
		_, hasGuardian := m.guardians[proxyType]
		m.mu.Unlock()
		if hasGuardian {
			log.Printf("proxy %s: already running with guardian, skipping start", proxyType)
			return nil
		}

		// Process is running but no guardian (e.g. panel restarted while proxy kept running).
		// Determine current log mode to decide how to handle stdout capture.
		logMode := "file"
		if m.settingStore != nil {
			if v, err := m.settingStore.GetByKey(context.Background(), "proxy.log_output_config"); err == nil && v != "" {
				var cfg struct {
					Nginx struct {
						Mode string `json:"mode"`
					} `json:"nginx"`
					Caddy struct {
						Mode string `json:"mode"`
					} `json:"caddy"`
				}
				if json.Unmarshal([]byte(v), &cfg) == nil {
					switch proxyType {
					case ProxyNginx:
						logMode = cfg.Nginx.Mode
					case ProxyCaddy:
						logMode = cfg.Caddy.Mode
					}
				}
			}
		}

		pid := m.findProxyPID(m.proxyProcessName(proxyType))
		if pid > 0 {
			if logMode == "stdout" {
				// Stdout mode: the old process's stdout pipe is dead (panel restarted).
				// We must kill and restart the proxy to re-attach the LogBroker pipe.
				log.Printf("proxy %s: already running (pid %d) in stdout mode, restarting to re-attach stdout pipe", proxyType, pid)
				m.stopExisting(proxyType)
				time.Sleep(1 * time.Second)
				// Fall through to normal startup below
			} else {
				// File mode: just attach guardian and create a file-tailing LogBroker.
				log.Printf("proxy %s: already running (pid %d), attaching guardian", proxyType, pid)
				m.attachGuardian(proxyType, pid)
				// Create LogBroker for file mode (tail access log)
				broker := NewLogBroker()
				if proxyType == ProxyNginx {
					logPath := m.getNginxAccessLogPath()
					if logPath != "" {
						broker.TailFile(logPath)
					}
				}
				m.mu.Lock()
				m.logBrokers[proxyType] = broker
				m.mu.Unlock()
				return nil
			}
		} else {
			log.Printf("proxy %s: already running, skipping start", proxyType)
			return nil
		}
	}

	// Ensure nginx dependencies before starting
	if proxyType == ProxyNginx {
		if err := checkNginxUser(); err != nil {
			return err
		}
		if err := m.ensureNginxDependencies(); err != nil {
			return fmt.Errorf("nginx dependencies: %w", err)
		}
	}

	// Clear manually stopped flag when user explicitly starts
	m.ClearManuallyStopped(ctx, proxyType)
	m.stopExisting(proxyType)

	// Generate config from database and write to file (database = source of truth).
	// On the Panel (local node), config generators are wired and generate from DB.
	// On the Agent (remote node), config generators are NOT set — the config file
	// was already synced by the panel via SyncProxyConfig RPC. In that case, only
	// write a default config if no config file exists yet (first run).
	bgCtx := context.Background()
	switch proxyType {
	case ProxyNginx:
		if m.nginxConfigGen != nil {
			// Panel mode: generate config from DB (database = source of truth)
			config, err := m.nginxConfigGen(bgCtx)
			if err != nil {
				return fmt.Errorf("generate nginx config: %w", err)
			}
			m.WriteConfigFile(ProxyNginx, config)
		} else {
			// Agent mode: config file is synced by panel via RPC.
			// If it doesn't exist, the panel hasn't synced yet — refuse to start.
			configPath := filepath.Join(m.dataDir, m.GetConfigPath(ProxyNginx))
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				return fmt.Errorf("nginx config not yet synced by panel, cannot start")
			}
		}
	default:
		if m.caddyConfigGen != nil {
			// Panel mode: generate config from DB (database = source of truth)
			config, err := m.caddyConfigGen(bgCtx)
			if err != nil {
				return fmt.Errorf("generate caddy config: %w", err)
			}
			m.WriteConfigFile(ProxyCaddy, config)
		} else {
			// Agent mode: config file is synced by panel via RPC.
			// If it doesn't exist, the panel hasn't synced yet — refuse to start.
			configPath := filepath.Join(m.dataDir, m.GetConfigPath(ProxyCaddy))
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				return fmt.Errorf("caddy config not yet synced by panel, cannot start")
			}
		}
	}

	configPath := m.GetConfigPath(proxyType)

	var args []string
	switch proxyType {
	case ProxyNginx:
		args = []string{"-c", configPath}
	case ProxyCaddy:
		args = []string{"run", "--config", configPath, "--adapter", "caddyguardfile"}
	}

	// Use relative binary path from dataDir
	binPath := info.Path
	if strings.HasPrefix(binPath, m.dataDir) {
		binPath, _ = filepath.Rel(m.dataDir, binPath)
	}
	cmd := exec.Command(binPath, args...)
	cmd.Dir = m.dataDir

// Create log broker for real-time viewing
broker := NewLogBroker()
m.mu.Lock()
m.logBrokers[proxyType] = broker
m.mu.Unlock()

	// Check log output mode
	logMode := "file"
	if m.settingStore != nil {
		if v, err := m.settingStore.GetByKey(context.Background(), "proxy.log_output_config"); err == nil && v != "" {
			var cfg struct {
				Nginx struct {
					Mode string `json:"mode"`
				} `json:"nginx"`
				Caddy struct {
					Mode string `json:"mode"`
				} `json:"caddy"`
			}
			if json.Unmarshal([]byte(v), &cfg) == nil {
				switch proxyType {
				case ProxyNginx:
					logMode = cfg.Nginx.Mode
				case ProxyCaddy:
					logMode = cfg.Caddy.Mode
				}
			}
		}
	}

	if logMode == "stdout" {
		// Stdout mode: capture via pipe
		broker.AttachCmd(cmd)
	} else {
		// File mode: tail the access log file
		if proxyType == ProxyNginx {
			logPath := m.getNginxAccessLogPath()
			if logPath != "" {
				broker.TailFile(logPath)
			}
		}
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start: %w", err)
	}

	go func() { cmd.Wait() }()

	time.Sleep(500 * time.Millisecond)
	if cmd.Process != nil && runtime.GOOS != "windows" {
		if err := cmd.Process.Signal(syscall.Signal(0)); err != nil {
			return fmt.Errorf("process exited immediately")
		}
	}

	// Start guardian for crash detection and auto-restart
	proxyTypeStr := string(proxyType)
	guardian := healthcheck.New(
		healthcheck.DefaultConfig(proxyTypeStr, 0),
		func() bool {
			info := m.DetectProxy(proxyType)
			return info.Status == "running"
		},
		func(ctx context.Context) error {
			return m.StartProxy(ctx, proxyType)
		},
	)
	// Apply persisted guardian settings from DB
	_, autoRestart, maxRestarts := m.GetProxyGuardianSettings(context.Background(), proxyType)
	guardian.SetMaxRestarts(maxRestarts)
	guardian.SetAutoRestart(autoRestart)
	// Register crash callback for logging
	guardian.OnCrash(func(name string, attempt, maxRestarts int) {
		log.Printf("proxy %s: crash detected (attempt %d/%d)", name, attempt, maxRestarts)
		if m.crashLogRepo != nil {
			m.crashLogRepo.InsertCrashLog(context.Background(), name, attempt, maxRestarts)
		}
	})
m.mu.Lock()
m.guardians[proxyType] = guardian
m.processPIDs[proxyType] = cmd.Process.Pid
m.mu.Unlock()
guardian.Start(cmd.Process.Pid)
	log.Printf("proxy %s: guardian started (pid %d)", proxyType, cmd.Process.Pid)

	return nil
}

func (m *Manager) stopExisting(proxyType ProxyType) {
	procName := m.proxyProcessName(proxyType)
	switch runtime.GOOS {
	case "windows":
		exec.Command("taskkill", "/IM", procName+".exe", "/F").Run()
	default:
		// First try graceful stop via signal
		exec.Command("pkill", "-f", procName).Run()
		time.Sleep(1 * time.Second)
		// Force kill any remaining processes
		exec.Command("pkill", "-9", "-f", procName).Run()
		time.Sleep(500 * time.Millisecond)
	}
	// Wait for process to fully exit
	time.Sleep(1 * time.Second)
}

// StopProxy stops a proxy service and marks it as manually stopped.
func (m *Manager) StopProxy(ctx context.Context, proxyType ProxyType) error {
// Stop guardian to prevent auto-restart
m.mu.Lock()
if g, ok := m.guardians[proxyType]; ok {
g.Stop()
delete(m.guardians, proxyType)
}
m.mu.Unlock()

	if m.settingStore != nil {
		m.settingStore.Set(ctx, "proxy."+string(proxyType)+".manually_stopped", "true")
	}
	return m.killProxyProcess(proxyType)
}

// StopProxyForShutdown stops a proxy without marking as manually stopped.
// Used when the panel exits so proxy can auto-start on next panel start.
func (m *Manager) StopProxyForShutdown(ctx context.Context, proxyType ProxyType) error {
// Stop guardian
m.mu.Lock()
if g, ok := m.guardians[proxyType]; ok {
g.Stop()
delete(m.guardians, proxyType)
}
m.mu.Unlock()
return m.killProxyProcess(proxyType)
}

// killProxyProcess stops the proxy process. If a tracked PID is available
// (from StartProxy), it uses a precise PID-based kill to avoid accidentally
// killing other processes whose command line contains the proxy type name.
// Falls back to pkill -f if no PID is tracked.
func (m *Manager) killProxyProcess(proxyType ProxyType) error {
	// Try precise PID kill first
	m.mu.Lock()
	pid, hasPID := m.processPIDs[proxyType]
	if hasPID {
		delete(m.processPIDs, proxyType)
	}
	m.mu.Unlock()

	if hasPID && pid > 0 {
		if isProcessAlive(pid) {
			log.Printf("proxy %s: killing process by PID %d", proxyType, pid)
			syscall.Kill(pid, syscall.SIGTERM)
			// Wait for graceful exit (up to 5 seconds)
			deadline := time.Now().Add(5 * time.Second)
			for isProcessAlive(pid) && time.Now().Before(deadline) {
				time.Sleep(100 * time.Millisecond)
			}
			if isProcessAlive(pid) {
				log.Printf("proxy %s: force killing PID %d", proxyType, pid)
				syscall.Kill(pid, syscall.SIGKILL)
			}
			return nil
		}
		log.Printf("proxy %s: tracked PID %d is no longer alive, falling back to pkill", proxyType, pid)
	}

	// Fallback: pkill using precise binary name
	procName := m.proxyProcessName(proxyType)
	switch runtime.GOOS {
	case "windows":
		return exec.Command("taskkill", "/IM", procName+".exe", "/F").Run()
	default:
		return exec.Command("pkill", "-f", procName).Run()
	}
}

// isProcessAlive checks if a process with the given PID is still running.
func isProcessAlive(pid int) bool {
	if runtime.GOOS == "windows" {
		return true // simplified: always fallback on Windows
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 doesn't kill the process, just checks if it exists
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

// ClearManuallyStopped clears the manually stopped flag.
func (m *Manager) ClearManuallyStopped(ctx context.Context, proxyType ProxyType) {
	if m.settingStore != nil {
		m.settingStore.Set(ctx, "proxy."+string(proxyType)+".manually_stopped", "false")
	}
	// Guardian will be recreated on next start
}

// GetGuardianStatus returns the guardian status for a proxy type.
func (m *Manager) GetGuardianStatus(proxyType ProxyType) map[string]interface{} {
	// Read persisted settings so they are available even when guardian is not running
	autoStart, autoRestart, maxRestarts := m.GetProxyGuardianSettings(context.Background(), proxyType)

	m.mu.Lock()
	defer m.mu.Unlock()
	if g, ok := m.guardians[proxyType]; ok {
		status := g.Status()
		status["auto_start"] = autoStart
		status["auto_restart"] = autoRestart
		status["max_restarts"] = maxRestarts
		// Add CPU and memory stats if process is running
		if pid, ok := status["pid"]; ok {
			if pidInt, ok := pid.(int); ok && pidInt > 0 {
				cpu, mem := process.GetProcessStats(pidInt)
				status["cpu"] = cpu
				status["mem"] = mem
			}
		}
		return status
	}
	return map[string]interface{}{
		"proxy_type":       string(proxyType),
		"state":            "no_guardian",
		"current_restarts": 0,
		"max_restarts":     maxRestarts,
		"auto_start":       autoStart,
		"auto_restart":     autoRestart,
	}
}

// GetGuardian returns the guardian for a proxy type.
func (m *Manager) GetGuardian(proxyType ProxyType) (*healthcheck.Guardian, bool) {
m.mu.Lock()
defer m.mu.Unlock()
g, ok := m.guardians[proxyType]
return g, ok
}

// GetProxyGuardianSettings reads guardian settings (auto_start, auto_restart,
// max_restarts) from the settings DB. Returns defaults if not set.
func (m *Manager) GetProxyGuardianSettings(ctx context.Context, proxyType ProxyType) (autoStart, autoRestart bool, maxRestarts int) {
	autoStart = true // default: auto-start enabled
	autoRestart = false
	maxRestarts = 5

	if m.settingStore == nil {
		return
	}
	// auto_start: default true unless explicitly set to "false"
	if v, err := m.settingStore.GetByKey(ctx, "proxy."+string(proxyType)+".auto_start"); err == nil {
		autoStart = v != "false"
	} else {
		// Fall back to manually_stopped (inverse) for backward compat
		autoStart = !m.IsManuallyStopped(ctx, proxyType)
	}
	if v, err := m.settingStore.GetByKey(ctx, "proxy."+string(proxyType)+".auto_restart"); err == nil {
		autoRestart = v == "true"
	}
	if v, err := m.settingStore.GetByKey(ctx, "proxy."+string(proxyType)+".max_restarts"); err == nil {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxRestarts = n
		}
	}
	return
}

// SetProxyGuardianSettings persists guardian settings to the settings DB.
func (m *Manager) SetProxyGuardianSettings(ctx context.Context, proxyType ProxyType, autoStart, autoRestart bool, maxRestarts int) error {
	if m.settingStore == nil {
		return nil
	}
	if err := m.settingStore.Set(ctx, "proxy."+string(proxyType)+".auto_start", fmt.Sprintf("%v", autoStart)); err != nil {
		return err
	}
	if err := m.settingStore.Set(ctx, "proxy."+string(proxyType)+".auto_restart", fmt.Sprintf("%v", autoRestart)); err != nil {
		return err
	}
	if err := m.settingStore.Set(ctx, "proxy."+string(proxyType)+".max_restarts", strconv.Itoa(maxRestarts)); err != nil {
		return err
	}
	// Also update manually_stopped for backward compat with AutoStartManaged
	if !autoStart {
		m.settingStore.Set(ctx, "proxy."+string(proxyType)+".manually_stopped", "true")
	} else {
		m.settingStore.Set(ctx, "proxy."+string(proxyType)+".manually_stopped", "false")
	}
	return nil
}

// GetSetting reads a setting value from the store.
func (m *Manager) GetSetting(ctx context.Context, key string) (string, error) {
	if m.settingStore == nil {
		return "", nil
	}
	return m.settingStore.GetByKey(ctx, key)
}

// panelAddr returns the panel address (host:port) for reverse_proxy directives.
// It reads panel.listen_port from settings, defaulting to 8080.
func (m *Manager) panelAddr(ctx context.Context) string {
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
func (m *Manager) portalAddr(ctx context.Context) string {
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

// SetSetting writes a setting value to the store.
func (m *Manager) SetSetting(ctx context.Context, key, value string) error {
	if m.settingStore == nil {
		return nil
	}
	return m.settingStore.Set(ctx, key, value)
}

// IsManuallyStopped checks if a proxy was manually stopped.
func (m *Manager) IsManuallyStopped(ctx context.Context, proxyType ProxyType) bool {
	if m.settingStore == nil {
		return false
	}
	v, err := m.settingStore.GetByKey(ctx, "proxy."+string(proxyType)+".manually_stopped")
	return err == nil && v == "true"
}

// RestartProxy restarts the proxy process asynchronously.
// It stops the Guardian first (to prevent interference), then runs stop + start
// in a goroutine. This avoids blocking the caller for 3+ seconds and aligns
// with process.Manager.Restart() which is also async.
func (m *Manager) RestartProxy(ctx context.Context, proxyType ProxyType) error {
	// Stop guardian to prevent auto-restart during restart sequence
	m.mu.Lock()
	if g, ok := m.guardians[proxyType]; ok {
		g.Stop()
		delete(m.guardians, proxyType)
	}
	m.mu.Unlock()

	// Clear manually_stopped flag so the new process can be guarded
	if m.settingStore != nil {
		m.settingStore.Set(ctx, "proxy."+string(proxyType)+".manually_stopped", "false")
	}

	// Run stop + start asynchronously
	go func() {
		log.Printf("proxy %s: async restart begin", proxyType)
		m.killProxyProcess(proxyType)
		time.Sleep(time.Second)
		if err := m.StartProxy(context.Background(), proxyType); err != nil {
			log.Printf("proxy %s: async restart failed: %v", proxyType, err)
		} else {
			log.Printf("proxy %s: async restart completed", proxyType)
		}
	}()

	return nil
}

func (m *Manager) GetConfigPath(proxyType ProxyType) string {
	// Return relative path (resolved from cmd.Dir = dataDir)
	switch proxyType {
	case ProxyNginx:
		return "proxy/nginx.conf"
	case ProxyCaddy:
		return "proxy/Caddyfile"
	}
	return ""
}

// findProxyPID finds the PID of a running proxy process by name.
func (m *Manager) findProxyPID(name string) int {
	cmd := exec.Command("pgrep", "-f", name)
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	for _, pidStr := range strings.Fields(strings.TrimSpace(string(out))) {
		pid, err := strconv.Atoi(pidStr)
		if err != nil || pid <= 0 {
			continue
		}
		// Skip zombie processes
		statPath := fmt.Sprintf("/proc/%d/stat", pid)
		data, err := os.ReadFile(statPath)
		if err != nil {
			continue
		}
		str := string(data)
		if idx := strings.LastIndex(str, ")"); idx >= 0 && idx+2 < len(str) {
			state := str[idx+2 : idx+3]
			if state != "Z" {
				return pid
			}
		}
	}
	return 0
}

// attachGuardian creates a guardian for an already-running proxy process.
func (m *Manager) attachGuardian(proxyType ProxyType, pid int) {
	proxyTypeStr := string(proxyType)
	guardian := healthcheck.New(
		healthcheck.DefaultConfig(proxyTypeStr, 0),
		func() bool {
			info := m.DetectProxy(proxyType)
			return info.Status == "running"
		},
		func(ctx context.Context) error {
			return m.StartProxy(ctx, proxyType)
		},
	)
	_, autoRestart, maxRestarts := m.GetProxyGuardianSettings(context.Background(), proxyType)
	guardian.SetMaxRestarts(maxRestarts)
	guardian.SetAutoRestart(autoRestart)
	guardian.OnCrash(func(name string, attempt, maxRestarts int) {
		log.Printf("proxy %s: crash detected (attempt %d/%d)", name, attempt, maxRestarts)
		if m.crashLogRepo != nil {
			m.crashLogRepo.InsertCrashLog(context.Background(), name, attempt, maxRestarts)
		}
	})
	m.mu.Lock()
	m.guardians[proxyType] = guardian
	m.processPIDs[proxyType] = pid
	m.mu.Unlock()
	guardian.Start(pid)
	// Override startedAt with the real process start time from /proc/<pid>/stat
	// so that uptime is accurate even when the panel restarts while proxy keeps running.
	if startTime := process.GetProcessStartTime(pid); !startTime.IsZero() {
		guardian.SetStartedAt(startTime)
	}
	log.Printf("proxy %s: guardian attached to existing process (pid %d)", proxyType, pid)
}

// AutoStartManaged syncs config from DB and starts managed proxies.
// Skips proxies that have auto_start disabled.
func (m *Manager) AutoStartManaged() {
	time.Sleep(2 * time.Second)
	for _, proxyType := range []ProxyType{ProxyCaddy, ProxyNginx} {
		info := m.DetectProxy(proxyType)
		if info.Installed && info.Managed {
			autoStart, _, _ := m.GetProxyGuardianSettings(context.Background(), proxyType)
			if !autoStart {
				log.Printf("proxy %s: auto_start disabled, skipping", proxyType)
				continue
			}
			log.Printf("proxy %s: managed, syncing config and starting", proxyType)
			if err := m.StartProxy(context.Background(), proxyType); err != nil {
				log.Printf("auto-start %s failed: %v", proxyType, err)
			} else {
				log.Printf("auto-start %s succeeded", proxyType)
			}
		}
	}
}

// --- GitHub Release ---

type GitHubRelease struct {
	TagName    string        `json:"tag_name"`
	Prerelease bool          `json:"prerelease"`
	Assets     []GitHubAsset `json:"assets"`
}

type GitHubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// ListReleases fetches caddy releases from GitHub, filtering by the local platform.
func (m *Manager) ListReleases(repo string) ([]GitHubRelease, error) {
	return m.ListReleasesForPlatform(repo, runtime.GOOS, runtime.GOARCH)
}

// ListReleasesForPlatform fetches caddy releases from GitHub,
// filtering assets by the specified OS/arch.
func (m *Manager) ListReleasesForPlatform(repo, osName, archName string) ([]GitHubRelease, error) {
	apiURL := netutil.AccelerateGitHubURL(
		fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=10", repo),
		m.githubAccelDomain(),
	)
	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := m.githubHTTPClient(30 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var releases []GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, err
	}

	for i := range releases {
		var filtered []GitHubAsset
		for _, a := range releases[i].Assets {
			if m.isMatchingBinary(a.Name, osName, archName) {
				filtered = append(filtered, a)
			}
		}
		releases[i].Assets = filtered
	}

	return releases, nil
}

func (m *Manager) isMatchingBinary(name, osName, archName string) bool {
	lower := strings.ToLower(name)
	if strings.HasSuffix(lower, ".sig") || strings.HasSuffix(lower, ".pem") ||
		strings.HasSuffix(lower, ".sbom") || strings.HasSuffix(lower, ".txt") ||
		strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".dgst") {
		return false
	}
	// Check OS
	if !proxyAssetMatchesOS(lower, osName) {
		return false
	}
	// Check arch
	if !proxyAssetMatchesArch(lower, archName) {
		return false
	}
	ext := filepath.Ext(lower)
	return ext == "" || ext == ".exe" || strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".zip")
}

// proxyAssetMatchesOS checks if the asset name contains the target OS.
func proxyAssetMatchesOS(lowerName, osName string) bool {
	osAliases := map[string][]string{
		"linux":   {"linux"},
		"darwin":  {"darwin", "macos", "mac"},
		"windows": {"windows", "win"},
		"freebsd": {"freebsd"},
	}
	aliases, ok := osAliases[osName]
	if !ok {
		aliases = []string{osName}
	}
	for _, a := range aliases {
		if strings.Contains(lowerName, a) {
			return true
		}
	}
	return false
}

// proxyAssetMatchesArch checks if the asset name contains the target architecture.
// Handles naming variants: amd64/x86_64/64, arm64/aarch64, arm/armv7/armhf, etc.
func proxyAssetMatchesArch(lowerName, archName string) bool {
	// Try exact arch name first
	if strings.Contains(lowerName, "_"+archName) ||
		strings.Contains(lowerName, "-"+archName) ||
		strings.Contains(lowerName, "."+archName) ||
		strings.HasSuffix(lowerName, archName) {
		return true
	}
	archAliases := map[string][]string{
		"amd64": {"x86_64", "x86-64"},
		"arm64": {"aarch64", "arm64-v8a", "armv8"},
		"arm":   {"armv7", "armhf", "armv7hl"},
		"386":   {"i386", "i686", "x86"},
	}
	aliases, ok := archAliases[archName]
	if !ok {
		return false
	}
	for _, a := range aliases {
		if strings.Contains(lowerName, a) {
			return true
		}
	}
	// Special case: amd64 can match "64" but NOT "arm64" or "386"
	// Check that "64" appears as a distinct segment (not part of arm64/386)
	if archName == "amd64" {
		// Look for patterns like _64, -64, or .64
		if strings.Contains(lowerName, "_64") || strings.Contains(lowerName, "-64") {
			// Make sure it's not arm64 or arm32-v7a etc
			if !strings.Contains(lowerName, "arm64") {
				return true
			}
		}
	}
	// Special case: 386 can match "32"
	if archName == "386" {
		if strings.Contains(lowerName, "_32") || strings.Contains(lowerName, "-32") || strings.Contains(lowerName, "32.") || strings.HasSuffix(lowerName, "32") {
			if !strings.Contains(lowerName, "arm32") {
				return true
			}
		}
	}
	return false
}

// parseOSReleaseID extracts the ID value from /etc/os-release content.
func parseOSReleaseID(content string) string {
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "ID=") {
			id := strings.TrimPrefix(line, "ID=")
			id = strings.Trim(id, "\"")
			return strings.ToLower(id)
		}
	}
	return ""
}

// detectNginxDistro returns the distro identifier matching nginx binary naming convention.
// The qist/nginx-binaries repo uses glibc/musl libc as the distro tag:
//   - glibc  — all glibc-based Linux (debian, ubuntu, RHEL, centos, rocky, almalinux, fedora, etc.)
//     Built on CentOS 7 (glibc 2.17), compatible with any glibc distro.
//   - musl   — Alpine / OpenWrt
//   - macos  — macOS
//   - windows — Windows
func detectNginxDistro() string {
	switch runtime.GOOS {
	case "darwin":
		return "macos"
	case "windows":
		return "windows"
	case "linux":
		// Read /etc/os-release to determine libc variant
		data, err := os.ReadFile("/etc/os-release")
		if err != nil {
			return "glibc" // fallback: most common
		}
		id := parseOSReleaseID(string(data))
		switch id {
		case "alpine", "openwrt":
			return "musl"
		default:
			// All other distros (debian, ubuntu, centos, rhel, rocky, almalinux, fedora, etc.)
			// are glibc-based and share the same glibc binary.
			return "glibc"
		}
	}
	return "glibc"
}

// isMatchingNginxBinary checks if an asset name matches the target platform for nginx binaries.
// nginx binary naming: nginx_{version}_{glibc|musl|macos|windows}_{arch}.tar.gz
// distroHint (from Agent heartbeat) is a raw distro ID like "ubuntu", "alpine", "redhat";
// it is normalized to glibc/musl by detectNginxDistroForOS.
func (m *Manager) isMatchingNginxBinary(name, osName, archName, distroHint string) bool {
	lower := strings.ToLower(name)
	if strings.HasSuffix(lower, ".sig") || strings.HasSuffix(lower, ".pem") ||
		strings.HasSuffix(lower, ".sbom") || strings.HasSuffix(lower, ".txt") ||
		strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".dgst") {
		return false
	}
	distro := detectNginxDistroForOS(osName, distroHint)
	if !strings.Contains(lower, distro) {
		return false
	}
	// Check arch with aliases
	if !proxyAssetMatchesArch(lower, archName) {
		return false
	}
	return strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".zip")
}

// detectNginxDistroForOS returns the distro identifier for nginx binary naming.
// For non-linux OS, returns the OS name directly (macos, windows).
// For linux, normalizes the distroHint (raw distro ID from Agent heartbeat, e.g.
// "ubuntu", "debian", "alpine", "redhat") to glibc or musl.
// If distroHint is empty, reads local /etc/os-release as fallback.
func detectNginxDistroForOS(osName, distroHint string) string {
	switch osName {
	case "darwin":
		return "macos"
	case "windows":
		return "windows"
	case "linux":
		// If Agent reported a distro, normalize to glibc/musl
		if distroHint != "" {
			switch distroHint {
			case "alpine", "openwrt":
				return "musl"
			default:
				// All other Linux distros (debian, ubuntu, redhat, centos, rocky, etc.)
				// are glibc-based and share the same glibc binary.
				return "glibc"
			}
		}
		// Fallback: read local /etc/os-release
		data, err := os.ReadFile("/etc/os-release")
		if err != nil {
			return "glibc" // fallback: most common
		}
		id := parseOSReleaseID(string(data))
		switch id {
		case "alpine", "openwrt":
			return "musl"
		default:
			return "glibc"
		}
	}
	return "glibc"
}

// ListNginxReleases fetches nginx releases from qist/nginx-binaries with local platform filtering.
func (m *Manager) ListNginxReleases(repo string) ([]GitHubRelease, error) {
	return m.ListNginxReleasesForPlatform(repo, runtime.GOOS, runtime.GOARCH, "")
}

// ListNginxReleasesForPlatform fetches nginx releases, filtering by the specified OS/arch/distro.
func (m *Manager) ListNginxReleasesForPlatform(repo, osName, archName, distro string) ([]GitHubRelease, error) {
	apiURL := netutil.AccelerateGitHubURL(
		fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=10", repo),
		m.githubAccelDomain(),
	)
	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := m.githubHTTPClient(30 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var releases []GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, err
	}

	for i := range releases {
		var filtered []GitHubAsset
		for _, a := range releases[i].Assets {
			if m.isMatchingNginxBinary(a.Name, osName, archName, distro) {
				filtered = append(filtered, a)
			}
		}
		releases[i].Assets = filtered
	}

	return releases, nil
}

func (m *Manager) DownloadAsset(ctx context.Context, downloadURL, name string) (string, error) {
	os.MkdirAll(m.binDir, 0750)
	binaryName := m.binaryName(name)
	destPath := filepath.Join(m.binDir, binaryName)

	accelURL := netutil.AccelerateGitHubURL(downloadURL, m.githubAccelDomain())
	req, _ := http.NewRequestWithContext(ctx, "GET", accelURL, nil)
	client := m.githubHTTPClient(300 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("download returned %d", resp.StatusCode)
	}

	if strings.HasSuffix(strings.ToLower(downloadURL), ".tar.gz") {
		return m.extractFromTarGz(resp.Body, name, destPath)
	}

	tmpPath := destPath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return "", err
	}
	f.Close()

	if err := os.Rename(tmpPath, destPath); err != nil {
		os.Remove(tmpPath)
		return "", err
	}
	os.Chmod(destPath, 0755)
	return destPath, nil
}

func (m *Manager) extractFromTarGz(reader io.Reader, name, destPath string) (string, error) {
	gz, err := gzip.NewReader(reader)
	if err != nil {
		return "", err
	}
	defer gz.Close()

	var binaryFound bool
	tr := tar.NewReader(gz)

	// For nginx: track whether lua/conf directories were extracted
	var luaExtracted bool

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if header.Typeflag == tar.TypeDir {
			continue
		}

		baseName := filepath.Base(header.Name)

		// Extract binary
		if baseName == name || baseName == name+".exe" || strings.HasPrefix(baseName, name+"-") {
			tmpPath := destPath + ".tmp"
			f, err := os.Create(tmpPath)
			if err != nil {
				return "", err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				os.Remove(tmpPath)
				return "", err
			}
			f.Close()
			if err := os.Rename(tmpPath, destPath); err != nil {
				os.Remove(tmpPath)
				return "", err
			}
			os.Chmod(destPath, 0755)
			binaryFound = true
		}

		// Extract nginx lua/ directory to dataDir/proxy/lua/
		if name == "nginx" && strings.Contains(header.Name, "/lua/") {
			// Determine the sub-path after "nginx/lua/"
			idx := strings.Index(header.Name, "/lua/")
			if idx >= 0 {
				subPath := header.Name[idx+len("/lua/"):]
				if subPath != "" {
					outPath := filepath.Join(m.dataDir, "proxy", "lua", subPath)
					if err := os.MkdirAll(filepath.Dir(outPath), 0755); err == nil {
						if f, err := os.Create(outPath); err == nil {
							io.Copy(f, tr)
							f.Close()
							luaExtracted = true
						}
					}
				}
			}
		}

		// Extract mime.types for nginx (to proxy config dir)
		if name == "nginx" && baseName == "mime.types" {
			mimePath := filepath.Join(m.dataDir, "proxy", "mime.types")
			if err := os.MkdirAll(filepath.Dir(mimePath), 0750); err == nil {
				if f, err := os.Create(mimePath); err == nil {
					io.Copy(f, tr)
					f.Close()
					log.Printf("extracted mime.types to proxy/")
				}
			}
		}

	}

	// Patch WAF config.lua with correct paths after lua extraction
	if name == "nginx" && luaExtracted {
		m.patchWAFConfig()
		// Remove the old rule-config directory from proxy/lua/waf/ —
		// rules are now unified under proxy/waf-rules/ (pushed by WAF syncer).
		// The nginx tar.gz still includes rule-config, so delete it after extraction.
		oldRuleDir := filepath.Join(m.dataDir, "proxy", "lua", "waf", "rule-config")
		if _, err := os.Stat(oldRuleDir); err == nil {
			os.RemoveAll(oldRuleDir)
			log.Printf("removed obsolete proxy/lua/waf/rule-config/ directory")
		}
		// Re-sync WAF config from DB so user customizations (cc_rate, etc.)
		// survive the tar.gz overwrite. patchWAFConfig only fixes paths;
		// this call restores the user's actual WAF settings from database.
		if m.onProxyInstalled != nil {
			m.onProxyInstalled(context.Background(), "nginx")
			log.Printf("re-synced WAF config from DB after nginx install")
		}
		// Chown lua directory to nginx user so worker process can read files
		luaDir := filepath.Join(m.dataDir, "proxy", "lua")
		if nginxUID, nginxGID, err := lookupNginxUser(); err == nil {
			filepath.Walk(luaDir, func(path string, info os.FileInfo, err error) error {
				if err == nil {
					os.Chown(path, nginxUID, nginxGID)
				}
				return nil
			})
			log.Printf("chown proxy/lua/ to nginx user (%d:%d)", nginxUID, nginxGID)
		}
		// Ensure proxy dir is accessible (0755, not 0750)
		os.Chmod(filepath.Join(m.dataDir, "proxy"), 0755)
		// Ensure logs dir is owned by nginx so WAF can create log files
		logsDir := filepath.Join(m.dataDir, "logs")
		os.MkdirAll(logsDir, 0775)
		if nginxUID, nginxGID, err := lookupNginxUser(); err == nil {
			os.Chown(logsDir, nginxUID, nginxGID)
		}
	}

	if !binaryFound {
		return "", fmt.Errorf("binary %q not found in archive", name)
	}
	return destPath, nil
}

// ReadConfigFile reads the config file from disk.
func (m *Manager) ReadConfigFile(proxyType ProxyType) (string, error) {
	relPath := m.GetConfigPath(proxyType)
	if relPath == "" {
		return "", fmt.Errorf("unknown proxy type")
	}
	fullPath := filepath.Join(m.dataDir, relPath)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", nil
	}
	return string(data), nil
}

// WriteConfigFile writes config to disk file only.
func (m *Manager) WriteConfigFile(proxyType ProxyType, config string) error {
	relPath := m.GetConfigPath(proxyType)
	if relPath == "" {
		return fmt.Errorf("unknown proxy type")
	}
	fullPath := filepath.Join(m.dataDir, relPath)
	os.MkdirAll(filepath.Dir(fullPath), 0750)
	return os.WriteFile(fullPath, []byte(config), 0644)
}

// ValidateConfig validates a proxy config file content.
func (m *Manager) ValidateConfig(proxyType ProxyType, config string) error {
	// Write to temp file and validate
	tmpDir := filepath.Join(m.dataDir, "tmp")
	os.MkdirAll(tmpDir, 0750)
	tmpFile := filepath.Join(tmpDir, string(proxyType)+"_validate.conf")
	os.WriteFile(tmpFile, []byte(config), 0644)
	defer os.Remove(tmpFile)

	info := m.DetectProxy(proxyType)
	if !info.Installed {
		return nil // Can't validate without binary
	}

	var cmd *exec.Cmd
	switch proxyType {
	case ProxyNginx:
		cmd = exec.Command(info.Path, "-t", "-c", tmpFile)
	case ProxyCaddy:
		cmd = exec.Command(info.Path, "validate", "--config", tmpFile, "--adapter", "caddyguardfile")
	default:
		return nil
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

// ReloadProxy sends reload signal to running proxy.
func (m *Manager) ReloadProxy(ctx context.Context, proxyType ProxyType) error {
	switch proxyType {
	case ProxyNginx:
		// Test config first, then reload
		info := m.DetectProxy(proxyType)
		if !info.Installed {
			// Not installed — silently skip. This happens when WAF changes
			// trigger reload for both nginx and caddy but only one is installed.
			log.Printf("proxy %s: not installed, skipping reload", proxyType)
			return nil
		}
		if info.Status != "running" {
			// Not running — config is on disk, will take effect on next start.
			log.Printf("proxy %s: not running (state=%s), skipping reload", proxyType, info.Status)
			return nil
		}
		binPath := info.Path
		if strings.HasPrefix(binPath, m.dataDir) {
			binPath, _ = filepath.Rel(m.dataDir, binPath)
		}
		configPath := m.GetConfigPath(proxyType)
		// Test config
		testCmd := exec.Command(binPath, "-t", "-c", configPath)
		testCmd.Dir = m.dataDir
		if out, err := testCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("nginx config test failed: %s", strings.TrimSpace(string(out)))
		}
		// Reload with config file
		reloadCmd := exec.Command(binPath, "-c", configPath, "-s", "reload")
		reloadCmd.Dir = m.dataDir
		return reloadCmd.Run()
	case ProxyCaddy:
		// Caddy supports in-process hot config reload via "caddy reload" command.
		info := m.DetectProxy(proxyType)
		if !info.Installed {
			// Not installed — silently skip.
			log.Printf("proxy %s: not installed, skipping reload", proxyType)
			return nil
		}
		if info.Status != "running" {
			// Not running — config is on disk, will be picked up on next start.
			log.Printf("proxy %s: not running (state=%s), skipping reload", proxyType, info.Status)
			return nil
		}
		binPath := info.Path
		if strings.HasPrefix(binPath, m.dataDir) {
			binPath, _ = filepath.Rel(m.dataDir, binPath)
		}
		configPath := m.GetConfigPath(proxyType)
		reloadCmd := exec.Command(binPath, "reload", "--config", configPath, "--adapter", "caddyguardfile")
		reloadCmd.Dir = m.dataDir
		if out, err := reloadCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("caddy reload failed: %s", strings.TrimSpace(string(out)))
		}
		return nil
	}
	return fmt.Errorf("unsupported proxy type")
}


// patchWAFConfig rewrites the WAF config.lua with paths matching this installation.
// Uses relative paths (relative to the nginx working directory = dataDir) so the
// same config.lua works on both the panel and remote Agents without modification.
// - "logs" resolves to dataDir/logs (CWD = dataDir when nginx starts)
// - "proxy/waf-rules" resolves to dataDir/proxy/waf-rules (unified rule dir)
//
// Note: config.lua lives at proxy/lua/waf/config.lua (part of the Lua source
// tree), but config_rule_dir inside it points to the unified rule directory.
func (m *Manager) patchWAFConfig() {
	configPath := filepath.Join(m.dataDir, "proxy", "lua", "waf", "config.lua")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return
	}
	content := string(data)
	// Replace log dir with relative path (works on panel AND agent)
	content = strings.Replace(content,
		"config_log_dir = \"/apps/nginx/log/\"",
		"config_log_dir = \"logs\"",
		1)
	// Replace rule dir with unified relative path
	content = strings.Replace(content,
		"config_rule_dir = \"/apps/nginx/conf/waf/rule-config\"",
		"config_rule_dir = \"proxy/waf-rules\"",
		1)
	// Also handle previously-patched configs that used the old path
	content = strings.Replace(content,
		"config_rule_dir = \"proxy/lua/waf/rule-config\"",
		"config_rule_dir = \"proxy/waf-rules\"",
		1)
	if err := os.WriteFile(configPath, []byte(content), 0644); err == nil {
		log.Printf("patched WAF config.lua with relative paths (log_dir=logs, rule_dir=proxy/waf-rules)")
	}
	// Remove the old rule-config directory — rules are now unified under
	// proxy/waf-rules/. The nginx tar.gz still includes rule-config, so
	// delete it here as a safety net (also catches upgrades from old versions).
	oldRuleDir := filepath.Join(m.dataDir, "proxy", "lua", "waf", "rule-config")
	if _, err := os.Stat(oldRuleDir); err == nil {
		os.RemoveAll(oldRuleDir)
		log.Printf("removed obsolete proxy/lua/waf/rule-config/ directory (patchWAFConfig)")
	}
}

func run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// SNISettings contains SNI routing configuration.
type SNISettings struct {
	Enabled            bool     `json:"enabled"`
	ListenPort         string   `json:"listen_port"`
	LocalPort          string   `json:"local_port"`
	ProxyProtocolRules []string `json:"proxy_protocol_rules"`
}

// GetSNISettings reads SNI settings from database.
func (m *Manager) GetSNISettings(ctx context.Context) (*SNISettings, error) {
	settings := &SNISettings{
		Enabled:    false,
		ListenPort: "443",
		LocalPort:  "8443",
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
	return settings, nil
}

// SaveSNISettings saves SNI settings to database.
func (m *Manager) SaveSNISettings(ctx context.Context, settings *SNISettings) error {
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
	return m.settingStore.Set(ctx, "proxy.sni_local_port", settings.LocalPort)
}

// BatchUpdatePorts updates HTTPS site listen ports and domain suffixes.
// Caddy: adds/removes :port suffix on domain (e.g. example.com -> example.com:8443).
// Nginx: only changes listen_port, domain stays unchanged.
// Only affects sites with TLS (cert_path set) — port 80 HTTP sites are untouched.
func (m *Manager) BatchUpdatePorts(ctx context.Context, port string, enable bool) (int, error) {
	sites, err := m.siteRepo.List(ctx, m.serverID)
	if err != nil {
		return 0, err
	}
	// Read default HTTPS port from nginx settings
	defaultHttpsPort := "443"
	if m.settingStore != nil {
		if v, err := m.settingStore.GetByKey(ctx, "proxy.nginx_settings"); err == nil && v != "" {
			var s NginxSettings
			if json.Unmarshal([]byte(v), &s) == nil && s.DefaultHttpsPort != "" {
				defaultHttpsPort = s.DefaultHttpsPort
			}
		}
	}

	count := 0
	for _, site := range sites {
		// Skip L4/SNI sites
		if site.Template == "l4" || site.Template == "sni" {
			continue
		}
		// Only migrate HTTPS sites (have TLS cert), skip plain HTTP
		if site.CertPath == "" {
			continue
		}

		if site.ProxyType == "nginx" {
			// Nginx: change listen_port and proxy_protocol
			if enable {
				site.ListenPort = port
				site.ProxyProtocol = true
			} else {
				site.ListenPort = defaultHttpsPort
				site.ProxyProtocol = false
			}
		} else {
			// Caddy: add/remove :port suffix on domain
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
		}

		if err := m.siteRepo.Update(ctx, &site); err != nil {
			continue
		}
		count++
	}
	return count, nil
}

// DeleteAllSNI deletes all SNI template sites from the database.
func (m *Manager) DeleteAllSNI(ctx context.Context) (int, error) {
	return m.DeleteByTemplate(ctx, "sni")
}

// ToggleSNIEnabled enables or disables all SNI template sites without deleting them.
// When disabled, rules stay in DB but won't be used in config generation.
func (m *Manager) ToggleSNIEnabled(ctx context.Context, proxyType string, enabled bool) (int, error) {
	sites, err := m.siteRepo.List(ctx, m.serverID)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, site := range sites {
		if site.Template == "sni" && (proxyType == "" || site.ProxyType == proxyType) {
			site.Enabled = enabled
			if err := m.siteRepo.Update(ctx, &site); err != nil {
				continue
			}
			count++
		}
	}
	return count, nil
}

// DeleteAllL4 deletes all L4 template sites from the database.
func (m *Manager) DeleteAllL4(ctx context.Context) (int, error) {
	return m.DeleteByTemplate(ctx, "l4")
}

// DeleteByTemplate deletes all sites with the given template.
func (m *Manager) DeleteByTemplate(ctx context.Context, template string) (int, error) {
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

// GetL4Enabled reads L4 enabled state from database.
func (m *Manager) GetL4Enabled(ctx context.Context) bool {
	if m.settingStore == nil {
		return false
	}
	v, err := m.settingStore.GetByKey(ctx, "proxy.l4_enabled")
	return err == nil && v == "true"
}

// SetL4Enabled saves L4 enabled state to database.
func (m *Manager) SetL4Enabled(ctx context.Context, enabled bool) error {
	if m.settingStore == nil {
		return nil
	}
	v := "false"
	if enabled {
		v = "true"
	}
	return m.settingStore.Set(ctx, "proxy.l4_enabled", v)
}
