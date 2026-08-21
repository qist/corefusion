package waf

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
	"corefusion/pkg/netutil"
)

// WAFFile is a single file to sync to a remote node.
type WAFFile struct {
	Path    string // relative to waf-rules/ dir, e.g. "args.rule", "domains/xxx/url.rule"
	Content []byte
	Hash    string // SHA-256 of content; empty content = empty hash
}

// NodeSyncer abstracts pushing WAF files to nodes (local or remote).
type NodeSyncer interface {
	SyncWAFFiles(ctx context.Context, serverID int64, files []*WAFFile) error
}

// SettingStore provides access to system settings.
type SettingStore interface {
	GetByKey(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
}

// Service manages WAF rules stored in the database.
type Service struct {
	ruleRepo   *repository.WafRuleRepository
	cfgRepo    *repository.WafConfigRepository
	domainRepo *repository.WafDomainConfigRepository
	syncer     NodeSyncer
	settingSvc SettingStore
	dataDir    string
}

// NewService creates a new WAF service.
func NewService(
	ruleRepo *repository.WafRuleRepository,
	cfgRepo *repository.WafConfigRepository,
	domainRepo *repository.WafDomainConfigRepository,
	dataDir string,
	settingSvc SettingStore,
) *Service {
	return &Service{
		ruleRepo:   ruleRepo,
		cfgRepo:     cfgRepo,
		domainRepo:  domainRepo,
		dataDir:     dataDir,
		settingSvc:  settingSvc,
	}
}

// SetSyncer sets the node syncer for pushing WAF files to remote nodes.
func (s *Service) SetSyncer(syncer NodeSyncer) {
	s.syncer = syncer
}

// wafRulesDir returns the shared WAF rules directory: dataDir/proxy/waf-rules
func (s *Service) wafRulesDir() string {
	return filepath.Join(s.dataDir, "proxy", "waf-rules")
}

// --- WAF Rules (Global) ---

// ListRules returns all WAF rules from the database.
func (s *Service) ListRules(ctx context.Context) ([]entity.WafRule, error) {
	return s.ruleRepo.List(ctx)
}

// ListGlobalRules returns all global-scope WAF rules.
func (s *Service) ListGlobalRules(ctx context.Context) ([]entity.WafRule, error) {
	return s.ruleRepo.ListGlobal(ctx)
}

// GetRule returns the content of a global WAF rule by name.
func (s *Service) GetRule(ctx context.Context, name string) (*entity.WafRule, error) {
	return s.ruleRepo.GetGlobalByName(ctx, name)
}

// SaveRule saves a global WAF rule to the database and computes its hash.
// mergeMode: "append" or "replace" (default: "replace")
func (s *Service) SaveRule(ctx context.Context, name, content, mergeMode string) error {
	if mergeMode == "" {
		mergeMode = "replace"
	}
	rule := &entity.WafRule{
		Name:      name,
		RuleScope: "global",
		ServerID:  0,
		MergeMode: mergeMode,
		Content:   content,
		Hash:      hashContent(content),
		Enabled:   true,
	}
	return s.ruleRepo.Upsert(ctx, rule)
}

// DeleteRule removes a global WAF rule from the database.
func (s *Service) DeleteRule(ctx context.Context, name string) error {
	return s.ruleRepo.Delete(ctx, name, 0)
}

// --- WAF Rules (Node-level) ---

// ListNodeRules returns all node-scoped rules for a specific server.
func (s *Service) ListNodeRules(ctx context.Context, serverID int64) ([]entity.WafRule, error) {
	return s.ruleRepo.ListNodeRules(ctx, serverID)
}

// GetNodeRule returns a node-scoped rule by name and server_id.
func (s *Service) GetNodeRule(ctx context.Context, name string, serverID int64) (*entity.WafRule, error) {
	return s.ruleRepo.GetNodeByName(ctx, name, serverID)
}

// SaveNodeRule saves a node-scoped WAF rule.
// mergeMode: "append" or "replace" (default: "replace")
func (s *Service) SaveNodeRule(ctx context.Context, name string, serverID int64, content, mergeMode string) error {
	if mergeMode == "" {
		mergeMode = "replace"
	}
	rule := &entity.WafRule{
		Name:      name,
		RuleScope: "node",
		ServerID:  serverID,
		MergeMode: mergeMode,
		Content:   content,
		Hash:      hashContent(content),
		Enabled:   true,
	}
	return s.ruleRepo.Upsert(ctx, rule)
}

// DeleteNodeRule removes a node-scoped WAF rule (falls back to global).
func (s *Service) DeleteNodeRule(ctx context.Context, name string, serverID int64) error {
	return s.ruleRepo.Delete(ctx, name, serverID)
}

// --- WAF Master Switch ---

// IsWAFEnabled checks if WAF is enabled for a server.
// Reads the per-node WAF switch from settings.
// When disabled, no WAF config is generated in Caddyfile/nginx.conf.
func (s *Service) IsWAFEnabled(ctx context.Context, serverID int64) bool {
	if s.settingSvc == nil {
		return false
	}
	key := "proxy.waf_enabled"
	if serverID > 1 {
		key = fmt.Sprintf("proxy.waf_enabled.%d", serverID)
	}
	v, err := s.settingSvc.GetByKey(ctx, key)
	if err != nil || v == "" {
		return false
	}
	return v == "true"
}

// SetWAFEnabled enables or disables WAF for a server.
// When enabled for the first time, seeds default config.json (Caddy)
// and config.lua (Nginx) into the database if they don't exist yet.
// When disabled, Caddyfile will not include caddyguard block,
// and nginx.conf will not include WAF Lua directives.
func (s *Service) SetWAFEnabled(ctx context.Context, serverID int64, enabled bool) error {
	if s.settingSvc == nil {
		return fmt.Errorf("setting store not configured")
	}
	key := "proxy.waf_enabled"
	if serverID > 1 {
		key = fmt.Sprintf("proxy.waf_enabled.%d", serverID)
	}
	val := "false"
	if enabled {
		val = "true"
	}
	if err := s.settingSvc.Set(ctx, key, val); err != nil {
		return err
	}
	// When enabling, seed default WAF configs if they don't exist yet
	if enabled {
		if err := s.ensureDefaultConfig(ctx, serverID, "caddy"); err != nil {
			log.Printf("[WAF] failed to seed default caddy config for server %d: %v", serverID, err)
		}
		if err := s.ensureDefaultConfig(ctx, serverID, "nginx"); err != nil {
			log.Printf("[WAF] failed to seed default nginx config for server %d: %v", serverID, err)
		}
		// Land all rules and configs to local disk (for local node)
		if err := s.LandRulesForServer(ctx, serverID); err != nil {
			log.Printf("[WAF] failed to land rules for server %d: %v", serverID, err)
		}
	}
	return nil
}

// ensureDefaultConfig checks if a WAF config exists for the given server and proxy type.
// If not, creates one with the default content.
func (s *Service) ensureDefaultConfig(ctx context.Context, serverID int64, proxyType string) error {
	_, err := s.cfgRepo.Get(ctx, serverID, proxyType)
	if err == nil {
		// Config already exists, check if global fallback exists too
		return nil
	}
	// Check if global default exists
	_, err = s.cfgRepo.Get(ctx, 0, proxyType)
	if err == nil {
		return nil // global default exists
	}
	// Neither per-node nor global exists: seed global default
	defaultContent := DefaultCaddyWAFConfig
	if proxyType == "nginx" {
		defaultContent = DefaultNginxWAFConfig
	}
	cfg := &entity.WafConfig{
		ServerID:  0,
		ProxyType: proxyType,
		Content:   defaultContent,
		Hash:      hashContent(defaultContent),
	}
	return s.cfgRepo.Upsert(ctx, cfg)
}

// --- WAF Config ---

// GetConfig returns the WAF config for a server and proxy type.
// Falls back to global (server_id=0) if per-node config not found.
// If neither exists, returns the built-in default.
func (s *Service) GetConfig(ctx context.Context, serverID int64, proxyType string) (string, error) {
	cfg, err := s.cfgRepo.Get(ctx, serverID, proxyType)
	if err != nil {
		// Fall back to global default
		cfg, err = s.cfgRepo.Get(ctx, 0, proxyType)
		if err != nil {
			// Neither per-node nor global: return built-in default
			if proxyType == "nginx" {
				return DefaultNginxWAFConfig, nil
			}
			return DefaultCaddyWAFConfig, nil
		}
	}
	return cfg.Content, nil
}

// SaveConfig saves the WAF config for a server and proxy type.
func (s *Service) SaveConfig(ctx context.Context, serverID int64, proxyType, content string) error {
	cfg := &entity.WafConfig{
		ServerID:  serverID,
		ProxyType: proxyType,
		Content:   content,
		Hash:      hashContent(content),
	}
	return s.cfgRepo.Upsert(ctx, cfg)
}

// --- WAF Domain Config ---

// ListDomainConfigs returns all domain WAF configs for a server.
func (s *Service) ListDomainConfigs(ctx context.Context, serverID int64) ([]entity.WafDomainConfig, error) {
	return s.domainRepo.ListByServer(ctx, serverID)
}

// GetDomainConfig returns the WAF config for a specific domain on a server.
func (s *Service) GetDomainConfig(ctx context.Context, serverID int64, domain string) (string, error) {
	cfg, err := s.domainRepo.GetByDomain(ctx, serverID, domain)
	if err != nil {
		return "", err
	}
	return cfg.Config, nil
}

// SaveDomainConfig saves the WAF config for a specific domain on a server.
func (s *Service) SaveDomainConfig(ctx context.Context, serverID int64, domain, config string) error {
	cfg := &entity.WafDomainConfig{
		ServerID: serverID,
		Domain:   domain,
		Config:   config,
	}
	return s.domainRepo.Upsert(ctx, cfg)
}

// DeleteDomainConfig removes a domain WAF config.
func (s *Service) DeleteDomainConfig(ctx context.Context, serverID int64, domain string) error {
	return s.domainRepo.DeleteByServerAndDomain(ctx, serverID, domain)
}

// --- GitHub Sync ---

// githubRawBase is the base URL for rule files in the nginxguard repo.
const githubRawBase = "https://raw.githubusercontent.com/qist/nginxguard/main/rule-config"

// githubRuleDefault defines a default rule file from GitHub with its merge mode.
type githubRuleDefault struct {
	Name      string // e.g. "args.rule"
	MergeMode string // "append" or "replace"
}

// defaultRuleFiles are the rule files downloaded from qist/nginxguard.
// These include engine rules (regex/pattern) and the collected whiteua rule.
// Only blackip, whiteip, whiteurl are NOT synced from GitHub —
// they are created as empty rules for users to fill in.
var defaultRuleFiles = []githubRuleDefault{
	{Name: "args.rule", MergeMode: "replace"},
	{Name: "cookie.rule", MergeMode: "replace"},
	{Name: "fileext.rule", MergeMode: "replace"},
	{Name: "post.rule", MergeMode: "replace"},
	{Name: "referer.rule", MergeMode: "replace"},
	{Name: "url.rule", MergeMode: "replace"},
	{Name: "useragent.rule", MergeMode: "replace"},
	{Name: "whiteua.rule", MergeMode: "append"},
}

// userDefinedRules are created as empty rules for users to fill in.
// These are NOT downloaded from GitHub.
var userDefinedRules = []githubRuleDefault{
	{Name: "blackip.rule", MergeMode: "append"},
	{Name: "whiteip.rule", MergeMode: "append"},
	{Name: "whiteurl.rule", MergeMode: "append"},
	{Name: "cdnip.rule", MergeMode: "append"},
}

// githubAccelDomain reads the active GitHub acceleration domain from settings.
func (s *Service) githubAccelDomain(ctx context.Context) string {
	if s.settingSvc == nil {
		return ""
	}
	val, err := s.settingSvc.GetByKey(ctx, "github.acceleration")
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

// githubProxyURL reads the HTTP proxy URL from GitHub acceleration settings.
func (s *Service) githubProxyURL(ctx context.Context) string {
	if s.settingSvc == nil {
		return ""
	}
	val, err := s.settingSvc.GetByKey(ctx, "github.acceleration")
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
func (s *Service) githubHTTPClient(ctx context.Context, timeout time.Duration) *http.Client {
	return netutil.GitHubHTTPClientWithProxy(timeout, s.githubAccelDomain(ctx), s.githubProxyURL(ctx))
}

// SyncFromGitHub downloads rule files from qist/nginxguard and stores them in the database.
// Only syncs global-scope rules. Node-level rules are preserved.
func (s *Service) SyncFromGitHub(ctx context.Context) error {
	client := s.githubHTTPClient(ctx, 60*time.Second)

	for _, rd := range defaultRuleFiles {
		rawURL := fmt.Sprintf("%s/%s", githubRawBase, rd.Name)

		req, _ := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("[WAF] failed to download %s: %v", rd.Name, err)
			continue
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			log.Printf("[WAF] failed to read %s: %v", rd.Name, err)
			continue
		}

		content := string(body)
		rule := &entity.WafRule{
			Name:      rd.Name,
			RuleScope: "global",
			ServerID:  0,
			MergeMode: rd.MergeMode,
			Content:   content,
			Hash:      hashContent(content),
			Enabled:   true,
		}
		if err := s.ruleRepo.Upsert(ctx, rule); err != nil {
			log.Printf("[WAF] failed to upsert rule %s: %v", rd.Name, err)
			continue
		}
		log.Printf("[WAF] synced rule %s from GitHub (%d bytes, merge=%s)", rd.Name, len(body), rd.MergeMode)
	}

	return nil
}

// --- Initialization ---

// EnsureInitialized checks if the waf_rules table has global rules and initializes if not.
// 1. Downloads engine rules + whiteua from GitHub
// 2. Creates empty user-defined rules (blackip, whiteip, whiteurl, cdnip)
// 3. Ensures all user-defined rules exist (for upgrades that add new rules)
func (s *Service) EnsureInitialized(ctx context.Context) error {
	count, err := s.ruleRepo.CountGlobal(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		// Already initialized — but check for missing user-defined rules (e.g. cdnip.rule added later)
		for _, ud := range userDefinedRules {
			_, err := s.ruleRepo.GetGlobalByName(ctx, ud.Name)
			if err != nil {
				// Rule doesn't exist, create it
				rule := &entity.WafRule{
					Name:      ud.Name,
					RuleScope: "global",
					ServerID:  0,
					MergeMode: ud.MergeMode,
					Content:   "",
					Hash:      "",
					Enabled:   true,
				}
				if err := s.ruleRepo.Upsert(ctx, rule); err != nil {
					log.Printf("[WAF] failed to create missing user-defined rule %s: %v", ud.Name, err)
					continue
				}
				log.Printf("[WAF] created missing user-defined rule %s", ud.Name)
			}
		}
		return nil
	}
	log.Printf("[WAF] waf_rules table has no global rules, initializing...")

	// 1. Sync engine rules from GitHub
	if err := s.SyncFromGitHub(ctx); err != nil {
		log.Printf("[WAF] GitHub sync failed (will continue): %v", err)
	}

	// 2. Create empty user-defined rules
	for _, ud := range userDefinedRules {
		rule := &entity.WafRule{
			Name:      ud.Name,
			RuleScope: "global",
			ServerID:  0,
			MergeMode: ud.MergeMode,
			Content:   "",
			Hash:      "",
			Enabled:   true,
		}
		if err := s.ruleRepo.Upsert(ctx, rule); err != nil {
			log.Printf("[WAF] failed to create user-defined rule %s: %v", ud.Name, err)
			continue
		}
		log.Printf("[WAF] created empty user-defined rule %s", ud.Name)
	}

	return nil
}

// --- Local Disk Landing ---

// LandAllRules writes all rules from DB to the local disk (proxy/waf-rules/).
// Used for panel local node startup. Merges global + node(1) rules.
func (s *Service) LandAllRules(ctx context.Context) error {
	return s.LandRulesForServer(ctx, 1) // local server is always ID=1
}

// LandRulesForServer writes merged rules for a specific server to local disk.
// Also writes config.json (Caddy WAF) and config.lua (Nginx WAF) to the same directory.
func (s *Service) LandRulesForServer(ctx context.Context, serverID int64) error {
	rules, err := s.ruleRepo.ListByServer(ctx, serverID)
	if err != nil {
		return err
	}
	merged := s.mergeRules(ctx, rules, serverID)

	ruleDir := s.wafRulesDir()
	os.MkdirAll(ruleDir, 0755)

	for name, content := range merged {
		path := filepath.Join(ruleDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			log.Printf("[WAF] failed to write rule %s: %v", name, err)
			continue
		}
	}

	// Write config.json (Caddy WAF config)
	caddyCfg, err := s.GetConfig(ctx, serverID, "caddy")
	if err == nil && caddyCfg != "" {
		path := filepath.Join(ruleDir, "config.json")
		if err := os.WriteFile(path, []byte(caddyCfg), 0644); err != nil {
			log.Printf("[WAF] failed to write config.json: %v", err)
		}
	}

	// Write config.lua (Nginx WAF config)
	nginxCfg, err := s.GetConfig(ctx, serverID, "nginx")
	if err == nil && nginxCfg != "" {
		path := filepath.Join(ruleDir, "config.lua")
		if err := os.WriteFile(path, []byte(nginxCfg), 0644); err != nil {
			log.Printf("[WAF] failed to write config.lua: %v", err)
		}
	}

	// Write domain.json (domain-level WAF config)
	domainJSON, err := s.buildDomainJSON(ctx, serverID)
	if err == nil {
		path := filepath.Join(ruleDir, "domain.json")
		if err := os.WriteFile(path, []byte(domainJSON), 0644); err != nil {
			log.Printf("[WAF] failed to write domain.json: %v", err)
		}
	}

	return nil
}

// --- Sync to Nodes ---

// mergeRules merges global and node-level rules into a single map.
// Each rule carries its own merge_mode:
//   - "append":   node content is appended to global content (global + "\n" + node)
//   - "replace":  node content replaces global content entirely
//
// Rules with no global counterpart are included as-is.
// Returns map[ruleName]content.
func (s *Service) mergeRules(ctx context.Context, rules []entity.WafRule, serverID int64) map[string]string {
	type ruleEntry struct {
		content   string
		mergeMode string
	}
	globalRules := make(map[string]ruleEntry)
	nodeRules := make(map[string]ruleEntry)

	for _, r := range rules {
		entry := ruleEntry{content: r.Content, mergeMode: r.MergeMode}
		if entry.mergeMode == "" {
			entry.mergeMode = "replace"
		}
		if r.RuleScope == "global" {
			globalRules[r.Name] = entry
		} else if r.RuleScope == "node" && r.ServerID == serverID {
			nodeRules[r.Name] = entry
		}
	}

	merged := make(map[string]string)

	// Process all unique rule names
	names := make(map[string]bool)
	for name := range globalRules {
		names[name] = true
	}
	for name := range nodeRules {
		names[name] = true
	}

	for name := range names {
		gEntry, hasGlobal := globalRules[name]
		nEntry, hasNode := nodeRules[name]

		if !hasNode || nEntry.content == "" {
			// No node override, use global as-is
			if hasGlobal {
				merged[name] = gEntry.content
			}
			continue
		}

		if !hasGlobal {
			// Node-only rule
			merged[name] = nEntry.content
			continue
		}

		// Both exist: merge based on the node rule's merge_mode
		switch nEntry.mergeMode {
		case "append":
			if gEntry.content != "" {
				merged[name] = gEntry.content + "\n" + nEntry.content
			} else {
				merged[name] = nEntry.content
			}
		default: // "replace"
			merged[name] = nEntry.content
		}
	}

	return merged
}

// buildDomainJSON assembles the domain.json content from waf_domain_configs for a server.
func (s *Service) buildDomainJSON(ctx context.Context, serverID int64) (string, error) {
	configs, err := s.domainRepo.ListByServer(ctx, serverID)
	if err != nil {
		return "", err
	}
	if len(configs) == 0 {
		return "{}", nil
	}

	// Build a map of domain → config object
	result := make(map[string]json.RawMessage)
	for _, c := range configs {
		if c.Config == "" {
			continue
		}
		// Config is stored as JSON string, use it directly
		result[c.Domain] = json.RawMessage(c.Config)
	}

	data, err := json.MarshalIndent(result, "", "    ")
	if err != nil {
		return "{}", err
	}
	return string(data), nil
}

// CollectAllFiles collects all WAF files from DB for syncing to a node.
// Includes:
// 1. Merged global+node .rule files
// 2. config.json (Caddy WAF config, per-node)
// 3. config.lua (Nginx WAF config, per-node)
// 4. domain.json (domain-level WAF config, per-node)
func (s *Service) CollectAllFiles(ctx context.Context, serverID int64) ([]*WAFFile, error) {
	var files []*WAFFile

	// 1. Collect and merge global + node rules
	rules, err := s.ruleRepo.ListByServer(ctx, serverID)
	if err != nil {
		return nil, err
	}
	merged := s.mergeRules(ctx, rules, serverID)

	for name, content := range merged {
		h := hashContent(content)
		files = append(files, &WAFFile{
			Path:    name,
			Content: []byte(content),
			Hash:    h,
		})
	}

	// 2. Caddy config (config.json) — synced to proxy/waf-rules/config.json
	caddyCfg, err := s.GetConfig(ctx, serverID, "caddy")
	if err == nil && caddyCfg != "" {
		files = append(files, &WAFFile{
			Path:    "config.json",
			Content: []byte(caddyCfg),
			Hash:    hashContent(caddyCfg),
		})
	}

	// 3. Nginx config (config.lua) — synced to proxy/waf-rules/config.lua
	nginxCfg, err := s.GetConfig(ctx, serverID, "nginx")
	if err == nil && nginxCfg != "" {
		files = append(files, &WAFFile{
			Path:    "config.lua",
			Content: []byte(nginxCfg),
			Hash:    hashContent(nginxCfg),
		})
	}

	// 4. domain.json — per-node domain WAF config, built from waf_domain_configs
	domainJSON, err := s.buildDomainJSON(ctx, serverID)
	if err != nil {
		log.Printf("[WAF] failed to build domain.json for server %d: %v", serverID, err)
		domainJSON = "{}"
	}
	files = append(files, &WAFFile{
		Path:    "domain.json",
		Content: []byte(domainJSON),
		Hash:    hashContent(domainJSON),
	})

	return files, nil
}

// SyncToNode pushes all WAF files from DB to a specific node.
func (s *Service) SyncToNode(ctx context.Context, serverID int64) error {
	if s.syncer == nil {
		return fmt.Errorf("node syncer not configured")
	}
	files, err := s.CollectAllFiles(ctx, serverID)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return nil
	}
	return s.syncer.SyncWAFFiles(ctx, serverID, files)
}

// SyncToAllNodes pushes all WAF files to all known nodes.
// nodeIDs is the list of all server IDs (including local=1).
func (s *Service) SyncToAllNodes(ctx context.Context, nodeIDs []int64) {
	for _, id := range nodeIDs {
		if err := s.SyncToNode(ctx, id); err != nil {
			log.Printf("[WAF] sync to server %d failed: %v", id, err)
		} else {
			log.Printf("[WAF] synced rules to server %d", id)
		}
	}
}

// --- Default WAF Configs ---

// DefaultCaddyWAFConfig is the default config.json for CaddyGuard (Caddy WAF).
const DefaultCaddyWAFConfig = `{
    "waf_enable": "on",
    "trust_proxy_headers": "on",
    "log_dir": "logs",
    "rule_dir": "proxy/waf-rules",
    "white_url_check": "on",
    "white_ip_check": "on",
    "white_ua_check": "on",
    "black_ip_check": "on",
    "url_check": "on",
    "url_args_check": "on",
    "user_agent_check": "on",
    "cookie_check": "on",
    "cc_check": "on",
    "cc_rate": "150/60",
    "cc_block_ttl": 600,
    "post_check": "on",
    "referer_check": "off",
    "file_upload_check": "on",
    "bodyless": "on",
    "multipart_streaming_check": "off",
    "upload_filename_scan_limit": 0,
    "post_body_scan_limit": 2097152,
    "waf_output": "html",
    "waf_redirect_url": "https://www.caddyguard.com"
}`

// DefaultNginxWAFConfig is the default config.lua for NginxGuard (Nginx WAF).
const DefaultNginxWAFConfig = `--NginxGuard config file,enable = "on",disable = "off"

--waf status
config_waf_enable = "on"
--trust proxy forwarded headers (X-Forwarded-For, X-Real-IP, CF-Connecting-IP)
config_trust_proxy_headers = "on"
--log dir
config_log_dir = "logs"
--rule setting
config_rule_dir = "proxy/waf-rules"
--enable/disable white url
config_white_url_check = "on"
--enable/disable white ip
config_white_ip_check = "on"
--enable/disable white ua (search engine bots skip UA blacklist only, other checks still apply)
config_white_ua_check = "on"
--enable/disable block ip
config_black_ip_check = "on"
--enable/disable url filtering
config_url_check = "on"
--enalbe/disable url args filtering
config_url_args_check = "on"
--enable/disable user agent filtering
config_user_agent_check = "on"
--enable/disable cookie deny filtering
config_cookie_check = "on"
--enable/disable cc filtering
config_cc_check = "on"
--cc rate the xxx of xxx seconds
config_cc_rate = "150/60"
--cc block ttl (seconds), auto-ban IP when CC triggered, 0=disable auto-ban
config_cc_block_ttl = 600
--enable/disable post filtering
config_post_check = "on"
--enable/disable referer filtering
config_referer_check = "off"
--enable/disable file upload extension filtering
config_file_upload_check = "on"
--enable/disable bodyless method skip (GET/HEAD/OPTIONS skip body/post/file_upload checks)
config_bodyless = "on"
--enable/disable multipart temp-file body streaming scan (post.rule)
config_multipart_streaming_check = "off"
--multipart upload filename scan limit in bytes
config_upload_filename_scan_limit = 0
--inspection limit in bytes for large temp-file bodies (JSON/XML/plain text, etc.)
config_post_body_scan_limit = 2097152
--config waf output redirect/html
config_waf_output = "html"
--if config_waf_output ,setting url
config_waf_redirect_url = "https://www.waf.com"
config_output_html=[[
<html xmlns="http://www.w3.org/1999/xhtml"><head>
<meta http-equiv="Content-Type" content="text/html; charset=utf-8">
<title>NginxGuard防火墙</title>
<style>
p {
\tline-height:20px;
}
ul{ list-style-type:none;}
li{ list-style-type:none;}
</style>
</head>
<body style=" padding:0; margin:0; font:14px/1.5 Microsoft Yahei, 宋体,sans-serif; color:#555;">
 <div style="margin: 0 auto; width:1000px; padding-top:70px; overflow:hidden;">
  <div style="width:600px; float:left;">
    <div style=" height:40px; line-height:40px; color:#fff; font-size:16px; overflow:hidden; background:#6bb3f6; padding-left:20px;">  </div>
    <div style="border:1px dashed #cdcece; border-top:none; font-size:14px; background:#fff; color:#555; line-height:24px; height:220px; padding:20px 20px 0 20px; overflow-y:auto;background:#f3f7f9;">
      <p style=" margin-top:0px; margin-bottom:0px; margin-left:0px; margin-right:0px; -qt-block-indent:0; text-indent:0px;"><span style=" font-weight:600; color:#fc4f03;">您的请求带有不合法参数，已被网站管理员设置拦截！</span></p>
<p style=" margin-top:0px; margin-bottom:0px; margin-left:0px; margin-right:0px; -qt-block-indent:0; text-indent:0px;">可能原因：您提交的内容包含危险的攻击请求</p>
<p style=" margin-top:12px; margin-bottom:12px; margin-left:0px; margin-right:0px; -qt-block-indent:1; text-indent:0px;">欢迎大神进行安全测试：</p>
<ul style="margin-top: 0px; margin-bottom: 0px; margin-left: 0px; margin-right: 0px; -qt-list-indent: 1;"><li style=" margin-top:12px; margin-bottom:0px; margin-left:0px; margin-right:0px; -qt-block-indent:0; text-indent:0px;">如有安全漏洞请发邮件:admin@waf.com；</li>
</ul>
    </div>
  </div>
</div>
</body></html>
]]
`

// --- Helpers ---

// hashContent returns the SHA-256 hex hash of content.
// Empty content returns empty string (so empty rules are never "synced" by hash check).
func hashContent(content string) string {
	if content == "" {
		return ""
	}
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

// StripPort removes the port from a domain string.
func StripPort(domain string) string {
	if idx := strings.LastIndex(domain, ":"); idx > 0 {
		return domain[:idx]
	}
	return domain
}

// --- Domain Rule Files ---

// ListDomainRuleDirs returns a list of domain names that have rule directories on disk.
func (s *Service) ListDomainRuleDirs() ([]string, error) {
	domainsDir := filepath.Join(s.wafRulesDir(), "domains")
	entries, err := os.ReadDir(domainsDir)
	if err != nil {
		return []string{}, nil // directory doesn't exist yet
	}
	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name())
		}
	}
	return dirs, nil
}

// GetDomainRuleFile reads a rule file for a specific domain.
// name is the rule file name, e.g. "args.rule".
func (s *Service) GetDomainRuleFile(domain, name string) (string, error) {
	path := filepath.Join(s.wafRulesDir(), "domains", domain, name)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// SaveDomainRuleFile writes a rule file for a specific domain.
// Creates the domain directory if it doesn't exist.
func (s *Service) SaveDomainRuleFile(domain, name, content string) error {
	domainDir := filepath.Join(s.wafRulesDir(), "domains", domain)
	if err := os.MkdirAll(domainDir, 0755); err != nil {
		return err
	}
	path := filepath.Join(domainDir, name)
	return os.WriteFile(path, []byte(content), 0644)
}

// InitDomainRules copies global rule files to a domain's rule directory.
// If the domain directory already has a rule file, it is preserved (not overwritten).
// Empty-content rules (blackip, whiteip, whiteurl, cdnip) are copied as empty files.
func (s *Service) InitDomainRules(ctx context.Context, domain string) error {
	domainDir := filepath.Join(s.wafRulesDir(), "domains", domain)
	if err := os.MkdirAll(domainDir, 0755); err != nil {
		return err
	}

	// Get all global rules
	rules, err := s.ruleRepo.ListGlobal(ctx)
	if err != nil {
		return err
	}

	for _, rule := range rules {
		destPath := filepath.Join(domainDir, rule.Name)
		// Don't overwrite if the domain already has this file
		if _, err := os.Stat(destPath); err == nil {
			continue
		}
		// Copy global rule content to domain directory
		if err := os.WriteFile(destPath, []byte(rule.Content), 0644); err != nil {
			log.Printf("[WAF] failed to copy rule %s to domain %s: %v", rule.Name, domain, err)
			continue
		}
	}
	return nil
}
