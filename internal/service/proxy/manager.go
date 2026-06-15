package proxy

import (
	"archive/tar"
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
	"strings"
	"syscall"
	"time"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
	"corefusion/pkg/netutil"
)

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

// Manager handles proxy program management.
type Manager struct {
	dataDir      string
	binDir       string
	configDir    string
	siteRepo     *repository.ProxySiteRepository
	settingStore SettingStore
	// Config generators (set by router to use new separated managers)
	caddyConfigGen func(ctx context.Context) (string, error)
	nginxConfigGen func(ctx context.Context) (string, error)
	// Stdout log broker for real-time log viewing
	logBrokers map[ProxyType]*LogBroker
	// Process guardian for auto-restart on crash
	guardians map[ProxyType]*ProxyGuardian
}

// SetCaddyConfigGen sets the Caddy config generator function.
func (m *Manager) SetCaddyConfigGen(fn func(ctx context.Context) (string, error)) {
	m.caddyConfigGen = fn
}

// SetNginxConfigGen sets the Nginx config generator function.
func (m *Manager) SetNginxConfigGen(fn func(ctx context.Context) (string, error)) {
	m.nginxConfigGen = fn
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

// githubHTTPClient creates an HTTP client with GitHub acceleration.
func (m *Manager) githubHTTPClient(timeout time.Duration) *http.Client {
	return netutil.GitHubHTTPClient(timeout, m.githubAccelDomain())
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
		guardians:    make(map[ProxyType]*ProxyGuardian),
	}
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
		info.Status = m.getProcessStatus("nginx")
	case ProxyCaddy:
		info.Name = "Caddy"
		info.ConfigPath = m.toRelative(filepath.Join(m.configDir, "Caddyfile"))
		path, version, managed := m.detectBinary("caddy")
		info.Path = m.toRelative(path)
		info.Version = version
		info.Installed = path != ""
		info.Managed = managed
		info.Status = m.getProcessStatus("caddy")
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
		out, err := cmd.CombinedOutput()
		if err == nil && len(out) > 0 {
			v := strings.TrimSpace(string(out))
			if idx := strings.Index(v, "\n"); idx > 0 {
				v = v[:idx]
			}
			v = strings.TrimPrefix(v, "nginx/")
			v = strings.TrimPrefix(v, "caddy ")
			if len(v) > 40 {
				v = v[:40]
			}
			return v
		}
	}
	return "unknown"
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
	sites, err := m.siteRepo.ListEnabledByType(ctx, "caddy")
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
		sb.WriteString("\t}\n")
	}
	// Log configuration - check log output mode
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
	logOutput := "stdout"
	logPath := ""
	if m.settingStore != nil {
		if v, err := m.settingStore.GetByKey(ctx, "proxy.log_output"); err == nil && v != "" {
			logOutput = v
		}
		if v, err := m.settingStore.GetByKey(ctx, "proxy.log_path"); err == nil && v != "" {
			logPath = v
		}
	}
	switch caddyLogMode {
	case "off":
		// no global log block
	case "stdout":
		sb.WriteString("\n\tlog {\n")
		sb.WriteString("\t\toutput stdout\n")
		sb.WriteString("\t}\n")
	default: // "file"
		sb.WriteString("\n\tlog {\n")
		if logOutput == "file" && logPath != "" {
			sb.WriteString(fmt.Sprintf("\t\toutput file %s\n", logPath))
		} else {
			sb.WriteString("\t\toutput stdout\n")
		}
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
				sb.WriteString(fmt.Sprintf("\t\t\t\tproxy %s\n", target))
				if site.ProxyProtocol {
					sb.WriteString("\t\t\t\tproxy_protocol\n")
				}
				sb.WriteString("\t\t\t}\n")
			}
			// Fallback route at the end - no SNI matcher, catches all unmatched
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
		sb.WriteString(":80 {\n\trespond \"CoreFusion\" 200\n}\n")
		return sb.String(), nil
	}

	// Group HTTP sites by domain
	domainSites := make(map[string][]entity.ProxySite)
	for _, site := range httpSites {
		domainSites[site.Domain] = append(domainSites[site.Domain], site)
	}

	for domain, siteList := range domainSites {
		sb.WriteString(fmt.Sprintf("# %s\n", domain))
		sb.WriteString(fmt.Sprintf("%s {\n", domain))

		// TLS from first site that has it
		for _, site := range siteList {
			if site.CertPath != "" && site.KeyPath != "" {
				sb.WriteString(fmt.Sprintf("\ttls %s %s\n", stripDataDirPrefix(site.CertPath, m.dataDir), stripDataDirPrefix(site.KeyPath, m.dataDir)))
				break
			}
		}

		// Path-based routing
		for _, site := range siteList {
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
func stripDataDirPrefix(path, dataDir string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
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

func pathToMatcher(path string) string {
	s := strings.TrimPrefix(path, "/")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "-", "_")
	return s
}

// ListSites returns all proxy sites from the database.
func (m *Manager) ListSites(ctx context.Context) ([]entity.ProxySite, error) {
	return m.siteRepo.List(ctx)
}

// ListSitesByType returns proxy sites filtered by proxy type.
func (m *Manager) ListSitesByType(ctx context.Context, proxyType string) ([]entity.ProxySite, error) {
	return m.siteRepo.ListByType(ctx, proxyType)
}

// CreateSite creates a new proxy site in the database.
func (m *Manager) CreateSite(ctx context.Context, site *entity.ProxySite) error {
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

// SaveAndApplyNginxDefault writes a default nginx.conf to disk.
func (m *Manager) SaveAndApplyNginxDefault(ctx context.Context) error {
	path := filepath.Join(m.configDir, "nginx.conf")
	os.MkdirAll(m.configDir, 0750)
	return os.WriteFile(path, []byte(m.defaultNginxConf()), 0644)
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

	// Save default nginx config
	m.SaveAndApplyNginxDefault(ctx)

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

	// Save default config to DB
	m.SaveAndApply(ctx)

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
	// Set temp directory ownership to nginx user
	if nginxUID, nginxGID, err := lookupNginxUser(); err == nil {
		tempDir := filepath.Join(m.dataDir, "temp")
		filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
			if err == nil {
				os.Chown(path, nginxUID, nginxGID)
			}
			return nil
		})
	}

	// Ensure mime.types exists in proxy config directory (same as nginx.conf)
	proxyDir := filepath.Join(m.dataDir, "proxy")
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

	// Generate config from database and write to file (database = source of truth)
	bgCtx := context.Background()
	switch proxyType {
	case ProxyNginx:
		if m.nginxConfigGen != nil {
			config, err := m.nginxConfigGen(bgCtx)
			if err != nil {
				return fmt.Errorf("generate nginx config: %w", err)
			}
			m.WriteConfigFile(ProxyNginx, config)
		} else if err := m.SaveAndApplyNginxDefault(bgCtx); err != nil {
			return fmt.Errorf("generate nginx config: %w", err)
		}
	default:
		if m.caddyConfigGen != nil {
			config, err := m.caddyConfigGen(bgCtx)
			if err != nil {
				return fmt.Errorf("generate caddy config: %w", err)
			}
			m.WriteConfigFile(ProxyCaddy, config)
		} else if _, err := m.SaveAndApply(bgCtx); err != nil {
			return fmt.Errorf("generate config: %w", err)
		}
	}

	configPath := m.GetConfigPath(proxyType)

	var args []string
	switch proxyType {
	case ProxyNginx:
		args = []string{"-c", configPath}
	case ProxyCaddy:
		args = []string{"run", "--config", configPath, "--adapter", "caddyfile"}
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
	m.logBrokers[proxyType] = broker

	// Check log output mode
	logMode := "file"
	if m.settingStore != nil {
		if v, err := m.settingStore.GetByKey(context.Background(), "proxy.log_output_config"); err == nil && v != "" {
			var cfg struct {
				Nginx struct{ Mode string `json:"mode"` } `json:"nginx"`
				Caddy struct{ Mode string `json:"mode"` } `json:"caddy"`
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
	guardian := NewProxyGuardian(proxyType, m)
	m.guardians[proxyType] = guardian
	guardian.StartMonitoring(cmd.Process.Pid)
	log.Printf("proxy %s: guardian started (pid %d)", proxyType, cmd.Process.Pid)

	return nil
}

func (m *Manager) stopExisting(proxyType ProxyType) {
	switch runtime.GOOS {
	case "windows":
		exec.Command("taskkill", "/IM", string(proxyType)+".exe", "/F").Run()
	default:
		exec.Command("pkill", "-f", string(proxyType)).Run()
	}
	time.Sleep(500 * time.Millisecond)
}

// StopProxy stops a proxy service and marks it as manually stopped.
func (m *Manager) StopProxy(ctx context.Context, proxyType ProxyType) error {
	// Stop guardian to prevent auto-restart
	if g, ok := m.guardians[proxyType]; ok {
		g.Stop()
		delete(m.guardians, proxyType)
	}

	if m.settingStore != nil {
		m.settingStore.Set(ctx, "proxy."+string(proxyType)+".manually_stopped", "true")
	}
	switch runtime.GOOS {
	case "windows":
		return exec.CommandContext(ctx, "taskkill", "/IM", string(proxyType)+".exe", "/F").Run()
	default:
		return exec.CommandContext(ctx, "pkill", "-f", string(proxyType)).Run()
	}
}

// ClearManuallyStopped clears the manually stopped flag.
func (m *Manager) ClearManuallyStopped(ctx context.Context, proxyType ProxyType) {
	if m.settingStore != nil {
		m.settingStore.Set(ctx, "proxy."+string(proxyType)+".manually_stopped", "false")
	}
	// Also clear guardian's manually stopped flag
	if g, ok := m.guardians[proxyType]; ok {
		g.ClearStopped()
	}
}

// GetGuardianStatus returns the guardian status for a proxy type.
func (m *Manager) GetGuardianStatus(proxyType ProxyType) map[string]interface{} {
	if g, ok := m.guardians[proxyType]; ok {
		return g.Status()
	}
	return map[string]interface{}{
		"proxy_type":       string(proxyType),
		"state":            "no_guardian",
		"current_restarts": 0,
		"max_restarts":     5,
	}
}

// GetGuardian returns the guardian for a proxy type.
func (m *Manager) GetGuardian(proxyType ProxyType) (*ProxyGuardian, bool) {
	g, ok := m.guardians[proxyType]
	return g, ok
}

// GetSetting reads a setting value from the store.
func (m *Manager) GetSetting(ctx context.Context, key string) (string, error) {
	if m.settingStore == nil {
		return "", nil
	}
	return m.settingStore.GetByKey(ctx, key)
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

func (m *Manager) RestartProxy(ctx context.Context, proxyType ProxyType) error {
	m.StopProxy(ctx, proxyType)
	time.Sleep(time.Second)
	return m.StartProxy(ctx, proxyType)
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

// AutoStartManaged syncs config from DB and starts managed proxies.
// Skips proxies that were manually stopped by the user.
func (m *Manager) AutoStartManaged() {
	time.Sleep(2 * time.Second)
	for _, proxyType := range []ProxyType{ProxyCaddy, ProxyNginx} {
		info := m.DetectProxy(proxyType)
		if info.Installed && info.Managed {
			if m.IsManuallyStopped(context.Background(), proxyType) {
				log.Printf("proxy %s: manually stopped, skipping auto-start", proxyType)
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

func (m *Manager) ListReleases(repo string) ([]GitHubRelease, error) {
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

	osName := runtime.GOOS
	archName := runtime.GOARCH
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
		strings.HasSuffix(lower, ".md") {
		return false
	}
	if !strings.Contains(lower, osName) || !strings.Contains(lower, archName) {
		return false
	}
	ext := filepath.Ext(lower)
	return ext == "" || ext == ".exe" || strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".zip")
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

// detectNginxDistro returns the distro name matching nginx binary naming convention.
// Supported: alpine (alpine/openwrt), debian, ubuntu, redhat (centos/rhel/rocky/almalinux/oracle/fedora), macos, windows.
func detectNginxDistro() string {
	switch runtime.GOOS {
	case "darwin":
		return "macos"
	case "windows":
		return "windows"
	case "linux":
		// Read /etc/os-release to determine distro
		data, err := os.ReadFile("/etc/os-release")
		if err != nil {
			return "debian" // fallback to most common
		}
		id := parseOSReleaseID(string(data))
		switch id {
		case "alpine", "openwrt":
			// alpine and openwrt both use musl libc, share the same binary
			return "alpine"
		case "ubuntu":
			return "ubuntu"
		case "centos", "rhel", "rocky", "almalinux", "ol", "fedora":
			// ol = Oracle Linux; all map to redhat binary
			return "redhat"
		default:
			return "debian" // debian and derivatives
		}
	}
	return "debian"
}

// isMatchingNginxBinary checks if an asset name matches the current platform for nginx binaries.
// nginx binary naming: nginx_{version}_{distro}_{arch}.tar.gz
func (m *Manager) isMatchingNginxBinary(name string) bool {
	lower := strings.ToLower(name)
	if strings.HasSuffix(lower, ".sig") || strings.HasSuffix(lower, ".pem") ||
		strings.HasSuffix(lower, ".sbom") || strings.HasSuffix(lower, ".txt") ||
		strings.HasSuffix(lower, ".md") {
		return false
	}
	distro := detectNginxDistro()
	arch := runtime.GOARCH
	if !strings.Contains(lower, distro) || !strings.Contains(lower, arch) {
		return false
	}
	return strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".zip")
}

// ListNginxReleases fetches nginx releases from qist/nginx-binaries with platform filtering.
func (m *Manager) ListNginxReleases(repo string) ([]GitHubRelease, error) {
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
			if m.isMatchingNginxBinary(a.Name) {
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
		cmd = exec.Command(info.Path, "validate", "--config", tmpFile, "--adapter", "caddyfile")
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
			return fmt.Errorf("nginx not installed")
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
		// Caddy doesn't have a simple reload signal, need to restart
		info := m.DetectProxy(proxyType)
		if !info.Installed {
			return fmt.Errorf("caddy not installed")
		}
		configPath := m.GetConfigPath(proxyType)
		m.stopExisting(proxyType)
		time.Sleep(500 * time.Millisecond)
		binPath := info.Path
		if strings.HasPrefix(binPath, m.dataDir) {
			binPath, _ = filepath.Rel(m.dataDir, binPath)
		}
		cmd := exec.Command(binPath, "run", "--config", configPath, "--adapter", "caddyfile")
		cmd.Dir = m.dataDir
		if err := cmd.Start(); err != nil {
			return err
		}
		go func() { cmd.Wait() }()
		return nil
	}
	return fmt.Errorf("unsupported proxy type")
}

// defaultCaddyfile returns a simple default Caddyfile template.
func (m *Manager) defaultCaddyfile() string {
	return `{
	auto_https off
}

:80 {
	respond "CoreFusion Proxy - OK" 200
}
`
}

// defaultNginxConf returns a simple default nginx.conf template.
func (m *Manager) defaultNginxConf() string {
	return `worker_processes auto;
events {
    worker_connections 1024;
}
http {
    server {
        listen 80;
        location / {
            return 200 "CoreFusion Proxy - OK";
        }
    }
}
`
}

func run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// SNISettings contains SNI routing configuration.
type SNISettings struct {
	Enabled       bool     `json:"enabled"`
	ListenPort    string   `json:"listen_port"`
	LocalPort     string   `json:"local_port"`
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
	sites, err := m.siteRepo.List(ctx)
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
	sites, err := m.siteRepo.List(ctx)
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
	sites, err := m.siteRepo.List(ctx)
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
