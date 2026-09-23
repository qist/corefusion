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
	// certRepo 用于把同域名的**多张**证书一起挂到站点上 ✓（ECC + RSA 双证书 ✓）。
	// 允许为 nil ✓（未注入时只用站点自配的那组 ✓）。
	certRepo *repository.CertificateRepository
	settingStore SettingStore
	serverID     int64
	isLocal      bool
	// nodeServerRepo is used to detect mirror nodes and resolve their primary
	// server ID for config generation. When set, GenerateConfigForServer checks
	// if the target is a mirror and uses the primary's server_id to query sites.
	nodeServerRepo *repository.NodeServerRepository
}

// SetServerID sets the server ID for per-node site filtering.
func (m *NginxManager) SetServerID(id int64) { m.serverID = id }

// SetIsLocal sets whether this manager is operating on the local node.
func (m *NginxManager) SetIsLocal(v bool) { m.isLocal = v }

// SetCertificateRepo 注入证书仓储 ✓（用于生成"双证书"配置 ✓）。
func (m *NginxManager) SetCertificateRepo(repo *repository.CertificateRepository) {
	m.certRepo = repo
}

// SetNodeServerRepo sets the node server repository for mirror node detection.
// When set, GenerateConfigForServer will check if the target is a mirror node
// and use its primary's server_id to query nginx_sites and settings.
func (m *NginxManager) SetNodeServerRepo(repo *repository.NodeServerRepository) {
	m.nodeServerRepo = repo
}

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
	if (site.Template == "portal_forward" || site.Template == "subscription_forward") && !m.isLocal {
		return fmt.Errorf("%s sites can only be created on the panel node", site.Template)
	}
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

func (m *NginxManager) BatchUpdatePorts(ctx context.Context, port string, enable bool) (int, error) {
	sites, err := m.siteRepo.List(ctx, m.serverID)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, site := range sites {
		// l4/sni/**reality** 这些模板本来就不监听 443 ✓（Reality 固定用 9443 之类 ✓），
		// 一起改会把站点直接搞坏 ✗（用户报的 bug ✓）
		if site.Template == "l4" || site.Template == "sni" || site.Template == "reality" {
			continue
		}
		if site.CertPath == "" {
			continue
		}
		if enable {
			// **只迁移当前监听公网 443 的站点** ✓：
			// 原来只看"有证书" ✗ → Reality（有证书却监听 9443 ✓）也被改成 8443 ✗；自定义端口（2053 等）同样不该动 ✓
			if site.ListenPort != publicHTTPSPort {
				continue
			}
			site.ListenPort = port
			site.ProxyProtocol = true
		} else {
			// 回滚只动**当初被迁移的那批**（现在正监听迁移端口 ✓），别的站点一律不碰 ✓
			if site.ListenPort != port {
				continue
			}
			site.ListenPort = publicHTTPSPort
			site.ProxyProtocol = false
		}
		if err := m.siteRepo.Update(ctx, &site); err != nil {
			continue
		}
		count++
	}
	return count, nil
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

	// Check unified WAF master switch (proxy.waf_enabled)
	// This overrides the per-settings WAFEnabled field.
	wafKey := "proxy.waf_enabled"
	if serverID > 1 {
		wafKey = fmt.Sprintf("proxy.waf_enabled.%d", serverID)
	}
	wafVal, err := m.settingStore.GetByKey(ctx, wafKey)
	if err == nil && wafVal != "" {
		settings.WAFEnabled = wafVal == "true"
	}

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

// portalAddr returns the portal address (host:port) for portal_forward template.
// Reads panel.portal_port from settings; falls back to panel.listen_port.
func (m *NginxManager) portalAddr(ctx context.Context) string {
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

// --- WAF Management moved to service/waf/ ---
// All WAF rule management has been moved to internal/service/waf/service.go.
// The manager only retains path helpers for config generation.

// wafBaseDir returns the unified WAF rules directory: dataDir/proxy/waf-rules
func (m *NginxManager) wafBaseDir() string {
	return filepath.Join(m.dataDir, "proxy", "waf-rules")
}

// wafRuleDir returns the WAF rules directory (same as wafBaseDir for compatibility).
func (m *NginxManager) wafRuleDir() string {
	return m.wafBaseDir()
}

// wafConfigPath returns the path to config.lua.
// config.lua lives with the NginxGuard Lua source at proxy/lua/waf/config.lua
// because the Lua engine loads it via lua_package_path. The config_rule_dir
// inside config.lua points to the unified proxy/waf-rules/ directory.
func (m *NginxManager) wafConfigPath() string {
	return filepath.Join(m.dataDir, "proxy", "lua", "waf", "config.lua")
}
