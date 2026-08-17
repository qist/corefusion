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

// --- WAF Config ---

// GetConfig returns the WAF config for a server and proxy type.
// Falls back to global (server_id=0) if per-node config not found.
func (s *Service) GetConfig(ctx context.Context, serverID int64, proxyType string) (string, error) {
	cfg, err := s.cfgRepo.Get(ctx, serverID, proxyType)
	if err != nil {
		// Fall back to global default
		cfg, err = s.cfgRepo.Get(ctx, 0, proxyType)
		if err != nil {
			return "", err
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
// 2. Creates empty user-defined rules (blackip, whiteip, whiteurl)
func (s *Service) EnsureInitialized(ctx context.Context) error {
	count, err := s.ruleRepo.CountGlobal(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil // already initialized
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
