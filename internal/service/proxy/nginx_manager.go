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

// NginxManager handles Nginx-specific site management and config generation.
type NginxManager struct {
	dataDir      string
	configDir    string
	siteRepo     *repository.NginxSiteRepository
	settingStore SettingStore
	serverID     int64
	isLocal      bool
}

// SetServerID sets the server ID for per-node site filtering.
func (m *NginxManager) SetServerID(id int64) { m.serverID = id }

// SetIsLocal sets whether this manager is operating on the local node.
func (m *NginxManager) SetIsLocal(v bool) { m.isLocal = v }

// NewNginxManager creates a new NginxManager.
func NewNginxManager(dataDir string, siteRepo *repository.NginxSiteRepository, settingStore SettingStore) *NginxManager {
	absDir := dataDir
	if !filepath.IsAbs(dataDir) {
		if abs, err := filepath.Abs(dataDir); err == nil {
			absDir = abs
		}
	}
	return &NginxManager{
		dataDir:      absDir,
		configDir:    filepath.Join(absDir, "proxy"),
		siteRepo:     siteRepo,
		settingStore: settingStore,
	}
}

// --- Site CRUD ---

func (m *NginxManager) ListSites(ctx context.Context) ([]entity.NginxSite, error) {
	return m.siteRepo.List(ctx, m.serverID)
}

func (m *NginxManager) ListEnabledSites(ctx context.Context) ([]entity.NginxSite, error) {
	return m.siteRepo.ListEnabled(ctx, m.serverID)
}

func (m *NginxManager) CreateSite(ctx context.Context, site *entity.NginxSite) error {
	if site.ServerID == 0 {
		site.ServerID = m.serverID
	}
	return m.siteRepo.Create(ctx, site)
}

func (m *NginxManager) GetSite(ctx context.Context, id int64) (*entity.NginxSite, error) {
	return m.siteRepo.GetByID(ctx, id)
}

func (m *NginxManager) UpdateSite(ctx context.Context, site *entity.NginxSite) error {
	return m.siteRepo.Update(ctx, site)
}

func (m *NginxManager) DeleteSite(ctx context.Context, id int64) error {
	return m.siteRepo.Delete(ctx, id)
}

// --- SNI Management ---

func (m *NginxManager) ToggleSNIEnabled(ctx context.Context, enabled bool) (int, error) {
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

func (m *NginxManager) DeleteAllSNI(ctx context.Context) (int, error) {
	return m.deleteByTemplate(ctx, "sni")
}

func (m *NginxManager) DeleteAllL4(ctx context.Context) (int, error) {
	return m.deleteByTemplate(ctx, "l4")
}

func (m *NginxManager) deleteByTemplate(ctx context.Context, template string) (int, error) {
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

// BatchUpdatePorts updates HTTPS site listen ports and proxy_protocol.
func (m *NginxManager) BatchUpdatePorts(ctx context.Context, port string, enable bool) (int, error) {
	sites, err := m.siteRepo.List(ctx, m.serverID)
	if err != nil {
		return 0, err
	}
	defaultHttpsPort := m.getDefaultHttpsPort(ctx)
	count := 0
	for _, site := range sites {
		if site.Template == "l4" || site.Template == "sni" {
			continue
		}
		if site.CertPath == "" {
			continue
		}
		if enable {
			site.ListenPort = port
			site.ProxyProtocol = true
		} else {
			site.ListenPort = defaultHttpsPort
			site.ProxyProtocol = false
		}
		if err := m.siteRepo.Update(ctx, &site); err != nil {
			continue
		}
		count++
	}
	return count, nil
}

func (m *NginxManager) getDefaultHttpsPort(ctx context.Context) string {
	if m.settingStore == nil {
		return "443"
	}
	v, err := m.settingStore.GetByKey(ctx, "proxy.nginx_settings")
	if err != nil || v == "" {
		return "443"
	}
	var s NginxSettings
	if json.Unmarshal([]byte(v), &s) == nil && s.DefaultHttpsPort != "" {
		return s.DefaultHttpsPort
	}
	return "443"
}

// --- Config Generation ---

// SaveAndApply generates nginx config and writes to disk.
func (m *NginxManager) SaveAndApply(ctx context.Context) (string, error) {
	config, err := m.GenerateConfig(ctx)
	if err != nil {
		return "", err
	}
	path := filepath.Join(m.configDir, "nginx.conf")
	os.MkdirAll(m.configDir, 0755)
	if err := os.WriteFile(path, []byte(config), 0644); err != nil {
		return "", err
	}
	return config, nil
}

// DefaultNginxConf returns a simple default nginx.conf.
func (m *NginxManager) DefaultNginxConf() string {
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

// --- NginxSettings ---

// GetNginxSettings reads Nginx settings from database (uses the manager's serverID).
func (m *NginxManager) GetNginxSettings(ctx context.Context) (*NginxSettings, error) {
	return m.GetNginxSettingsForServer(ctx, m.serverID)
}

// GetNginxSettingsForServer reads Nginx settings from database for a specific server.
func (m *NginxManager) GetNginxSettingsForServer(ctx context.Context, serverID int64) (*NginxSettings, error) {
	settings := DefaultNginxSettings()
	if m.settingStore == nil {
		return settings, nil
	}

	// Try per-node key first
	key := perNodeNginxSettingsKey(serverID)
	v, err := m.settingStore.GetByKey(ctx, key)
	if err != nil || v == "" {
		// Fallback to global key for backward compatibility
		if serverID > 1 {
			v, err = m.settingStore.GetByKey(ctx, "proxy.nginx_settings")
			if err != nil || v == "" {
				return settings, nil
			}
		} else {
			return settings, nil
		}
	}
	var saved NginxSettings
	if err := json.Unmarshal([]byte(v), &saved); err != nil {
		return settings, nil
	}
	mergeNginxSettings(settings, &saved)
	return settings, nil
}

// SaveNginxSettings saves Nginx settings to database (uses the manager's serverID).
func (m *NginxManager) SaveNginxSettings(ctx context.Context, settings *NginxSettings) error {
	if m.settingStore == nil {
		return nil
	}
	data, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	key := perNodeNginxSettingsKey(m.serverID)
	return m.settingStore.Set(ctx, key, string(data))
}

// panelAddr returns the panel address (host:port) for reverse_proxy directives.
// It reads panel.listen_port from settings, defaulting to 8080.
func (m *NginxManager) panelAddr(ctx context.Context) string {
	port := "8080"
	if m.settingStore != nil {
		if v, err := m.settingStore.GetByKey(ctx, "panel.listen_port"); err == nil && v != "" {
			port = v
		}
	}
	return "127.0.0.1:" + port
}

// resolvePath resolves a relative cert path against dataDir.
func (m *NginxManager) resolvePath(path string) string {
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

// nginxUpstreamKey converts a domain/name to a safe upstream block name.
func nginxUpstreamKey(name string) string {
	s := strings.ReplaceAll(name, ".", "_")
	s = strings.ReplaceAll(s, ":", "_")
	s = strings.ReplaceAll(s, "*", "wildcard")
	s = strings.ReplaceAll(s, "-", "_")
	if s == "" {
		s = "default"
	}
	return s
}

// --- WAF Management ---

// wafBaseDir returns the WAF root directory: dataDir/proxy/lua/waf
func (m *NginxManager) wafBaseDir() string {
	return filepath.Join(m.dataDir, "proxy", "lua", "waf")
}

// wafRuleDir returns the WAF rule-config directory.
func (m *NginxManager) wafRuleDir() string {
	return filepath.Join(m.wafBaseDir(), "rule-config")
}

// WAFRuleFile describes a rule file in rule-config/.
type WAFRuleFile struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// ListWAFRules lists all global rule files in rule-config/.
func (m *NginxManager) ListWAFRules() []WAFRuleFile {
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
		// Only list .rule files
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
func (m *NginxManager) ReadWAFRule(name string) (string, error) {
	// Prevent path traversal
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
func (m *NginxManager) SaveWAFRule(name, content string) error {
	if strings.Contains(name, "..") || strings.Contains(name, "/") {
		return fmt.Errorf("invalid rule name")
	}
	path := filepath.Join(m.wafRuleDir(), name)
	os.MkdirAll(filepath.Dir(path), 0755)
	return os.WriteFile(path, []byte(content), 0644)
}

// ReadWAFConfig reads the WAF config.lua file.
func (m *NginxManager) ReadWAFConfig() (string, error) {
	path := filepath.Join(m.wafBaseDir(), "config.lua")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// SaveWAFConfig writes the WAF config.lua file.
// Automatically corrects log_dir and rule_dir to relative paths so the config
// works on both panel and remote Agents regardless of installation directory.
func (m *NginxManager) SaveWAFConfig(content string) error {
	path := filepath.Join(m.wafBaseDir(), "config.lua")
	os.MkdirAll(filepath.Dir(path), 0755)
	// Patch paths to relative (in case user pasted original /apps/nginx/ paths)
	content = strings.Replace(content,
		"config_log_dir = \"/apps/nginx/log/\"",
		"config_log_dir = \"logs\"", 1)
	content = strings.Replace(content,
		"config_rule_dir = \"/apps/nginx/conf/waf/rule-config\"",
		"config_rule_dir = \"proxy/lua/waf/rule-config\"", 1)
	return os.WriteFile(path, []byte(content), 0644)
}

// ReadWAFDomainConfig reads the domain.json file.
func (m *NginxManager) ReadWAFDomainConfig() (string, error) {
	path := filepath.Join(m.wafRuleDir(), "domain.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// SaveWAFDomainConfig writes the domain.json file.
func (m *NginxManager) SaveWAFDomainConfig(content string) error {
	path := filepath.Join(m.wafRuleDir(), "domain.json")
	os.MkdirAll(filepath.Dir(path), 0755)
	return os.WriteFile(path, []byte(content), 0644)
}

// ListWAFDomainRuleDirs lists domain directories under rule-config/domains/.
func (m *NginxManager) ListWAFDomainRuleDirs() []string {
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
// domain is the directory name under rule-config/domains/, ruleName is the file (e.g. "url.rule").
func (m *NginxManager) ReadWAFDomainRule(domain, ruleName string) (string, error) {
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
func (m *NginxManager) SaveWAFDomainRule(domain, ruleName, content string) error {
	if strings.Contains(domain, "..") || strings.Contains(ruleName, "..") {
		return fmt.Errorf("invalid path")
	}
	path := filepath.Join(m.wafRuleDir(), "domains", domain, ruleName)
	os.MkdirAll(filepath.Dir(path), 0755)
	return os.WriteFile(path, []byte(content), 0644)
}

// CollectWAFFiles reads all WAF files from disk and returns them as a slice of
// (relativePath, content) pairs for syncing to remote Agent nodes.
func (m *NginxManager) CollectWAFFiles() []WAFFileEntry {
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
		entries = append(entries, WAFFileEntry{Path: filepath.ToSlash(rel), Content: data})
		return nil
	})
	return entries
}

// WAFFileEntry is a (path, content) pair for WAF file syncing.
type WAFFileEntry struct {
	Path    string
	Content []byte
}

// EnableWAFDomain enables WAF for a domain by adding it to domain.json and
// copying global rule files to domains/<domain>/ directory for independent editing.
func (m *NginxManager) EnableWAFDomain(domain string) error {
	if strings.Contains(domain, "..") || domain == "" {
		return fmt.Errorf("invalid domain")
	}
	// 1. Copy global rule files to domains/<domain>/
	domainRuleDir := filepath.Join(m.wafRuleDir(), "domains", domain)
	os.MkdirAll(domainRuleDir, 0755)
ruleFiles := []string{"args.rule", "blackip.rule", "cookie.rule", "post.rule",
"url.rule", "useragent.rule", "whiteip.rule", "whiteurl.rule",
"whiteua.rule", "referer.rule", "fileext.rule"}
	for _, rf := range ruleFiles {
		src := filepath.Join(m.wafRuleDir(), rf)
		dst := filepath.Join(domainRuleDir, rf)
		data, err := os.ReadFile(src)
		if err != nil {
			continue // skip missing files
		}
		os.WriteFile(dst, data, 0644)
	}
	// 2. Add entry to domain.json
	return m.updateDomainJSON(domain, map[string]string{
		"rule_dir": fmt.Sprintf("domains/%s", domain),
	})
}

// DisableWAFDomain disables WAF for a domain by removing it from domain.json.
// The domain's rule directory is kept on disk for potential future re-enable.
func (m *NginxManager) DisableWAFDomain(domain string) error {
	if strings.Contains(domain, "..") || domain == "" {
		return fmt.Errorf("invalid domain")
	}
	return m.removeFromDomainJSON(domain)
}

// IsWAFDomainEnabled checks if a domain has WAF enabled in domain.json.
// Returns true only if the domain exists in domain.json AND waf_enable is not "off".
// If waf_enable is "off", the switch should show as off (WAF disabled for this domain).
func (m *NginxManager) IsWAFDomainEnabled(domain string) bool {
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
	// Check waf_enable: default to enabled if not set
	if cfgMap, ok := cfg.(map[string]interface{}); ok {
		if wafEnable, ok := cfgMap["waf_enable"].(string); ok && wafEnable == "off" {
			return false
		}
	}
	return true
}

// updateDomainJSON adds or updates a domain entry in domain.json.
func (m *NginxManager) updateDomainJSON(domain string, config map[string]string) error {
	path := filepath.Join(m.wafRuleDir(), "domain.json")
	data, err := os.ReadFile(path)
	var obj map[string]interface{}
	if err == nil {
		json.Unmarshal(data, &obj)
	}
	if obj == nil {
		obj = map[string]interface{}{}
	}
	// Preserve _comment
	cfg := map[string]interface{}{}
	for k, v := range config {
		cfg[k] = v
	}
	obj[domain] = cfg
	out, err := json.MarshalIndent(obj, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0644)
}

// removeFromDomainJSON removes a domain entry from domain.json.
func (m *NginxManager) removeFromDomainJSON(domain string) error {
	path := filepath.Join(m.wafRuleDir(), "domain.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil // no file = nothing to remove
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil
	}
	delete(obj, domain)
	out, err := json.MarshalIndent(obj, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0644)
}

// GenerateDefaultConfig returns a minimal nginx.conf for when no sites exist.
func (m *NginxManager) GenerateDefaultConfig() string {
	var sb strings.Builder
	sb.WriteString("worker_processes auto;\n")
	sb.WriteString("events {\n")
	sb.WriteString("    worker_connections 1024;\n")
	sb.WriteString("}\n")
	sb.WriteString("http {\n")
	sb.WriteString("    server {\n")
	sb.WriteString("        listen 80;\n")
	sb.WriteString("        location / {\n")
	sb.WriteString("            return 200 \"CoreFusion Proxy - OK\";\n")
	sb.WriteString("        }\n")
	sb.WriteString("    }\n")
	sb.WriteString("}\n")
	return sb.String()
}
