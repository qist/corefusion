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

// NodeLister provides the list of all server IDs (for push-to-all).
type NodeLister interface {
	ListAllServerIDs(ctx context.Context) ([]int64, error)
}

// SourceFileMapping defines a single rule file and its full download URL.
// If URL is empty, the file is downloaded from the base URL + "/" + Name.
type SourceFileMapping struct {
	Name string `json:"name"` // e.g. "args.rule"
	URL  string `json:"url"`  // full URL; empty = use base URL + "/" + Name
}

// GitHubSourceConfig holds the source URL and sync schedule for WAF rules.
// URL is the base URL for downloading rule files.
// FileMappings is a list of {name, url} pairs. If a mapping's URL is empty,
// the file is downloaded from URL + "/" + Name. If URL is non-empty, it is
// used as the full download address (no concatenation).
// For backward compatibility, if FileMappings is empty, defaultRuleFiles are used.
type GitHubSourceConfig struct {
	URL           string              `json:"url"`
	FileMappings  []SourceFileMapping `json:"file_mappings"`
	Files         string              `json:"files"`        // deprecated: comma-separated list, kept for backward compat
	SyncCron      string              `json:"sync_cron"`
	Enabled       bool                `json:"enabled"`
}

// Service manages WAF rules stored in the database.
type Service struct {
	ruleRepo   *repository.WafRuleRepository
	cfgRepo    *repository.WafConfigRepository
	domainRepo *repository.WafDomainConfigRepository
	syncer     NodeSyncer
	settingSvc SettingStore
	nodeLister NodeLister
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

// SetNodeLister sets the node lister for push-to-all operations.
func (s *Service) SetNodeLister(lister NodeLister) {
	s.nodeLister = lister
}

// MigrateNodeRulesMergeMode fixes legacy node-scoped WAF rules whose merge_mode
// was left as "replace" (the old frontend never sent merge_mode, so the backend
// defaulted to "replace"). A "replace" node rule silently drops the global rule
// of the same name, which is almost never what the user intends — node rules are
// meant to SUPPLEMENT the global rule. This idempotent migration flips any node
// rule that is "replace" (or empty) to "append" so global rules are preserved.
// It returns the number of rows migrated.
func (s *Service) MigrateNodeRulesMergeMode(ctx context.Context) (int64, error) {
	return s.ruleRepo.MigrateNodeReplaceToAppend(ctx)
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
	content = normalizeRuleContent(content)
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
	return s.ruleRepo.Delete(ctx, name, 0, "")
}

// --- WAF Rules (Node-level, merged with global) ---

// MergedRule represents a rule merged from global + node scope.
// It includes the source indicator so the UI can show whether a rule
// comes from global, node, or merged (append).
type MergedRule struct {
	Name         string `json:"name"`
	MergeMode    string `json:"merge_mode"`            // append | replace (from node rule, or global if no node rule)
	Content      string `json:"content"`               // merged content (what actually lands on disk)
	GlobalContent string `json:"global_content,omitempty"` // global part (only when source=merged)
	NodeContent  string `json:"node_content,omitempty"`    // node-only part (only when source=merged)
	Hash         string `json:"hash"`                    // hash of merged content
	Source       string `json:"source"`                  // "global" | "node" | "merged"
	Enabled      bool   `json:"enabled"`
	UpdatedAt    string `json:"updated_at"`              // latest update time
}

// ListNodeRules returns merged rules (global + node override) for a specific server.
// Each rule shows the effective content after merge, with a source field indicating:
//   - "global": rule only exists in global scope
//   - "node": rule only exists in node scope (no global counterpart)
//   - "merged": rule exists in both, node appended to global
func (s *Service) ListNodeRules(ctx context.Context, serverID int64) ([]MergedRule, error) {
	rules, err := s.ruleRepo.ListByServer(ctx, serverID)
	if err != nil {
		return nil, err
	}
	merged := s.mergeRulesWithMeta(ctx, rules, serverID)
	result := make([]MergedRule, 0, len(merged))
	for name, mr := range merged {
		result = append(result, MergedRule{
			Name:      name,
			MergeMode: mr.mergeMode,
			Content:   mr.content,
			Hash:      hashContent(mr.content),
			Source:    mr.source,
			Enabled:   true,
			UpdatedAt: mr.updatedAt,
		})
	}
	return result, nil
}

// GetNodeRule returns a merged rule by name for a specific server.
// Falls back to global if no node-level rule exists.
func (s *Service) GetNodeRule(ctx context.Context, name string, serverID int64) (*MergedRule, error) {
	// Try node-level first
	nodeRule, nodeErr := s.ruleRepo.GetNodeByName(ctx, name, serverID)
	globalRule, globalErr := s.ruleRepo.GetGlobalByName(ctx, name)

	if nodeErr != nil && globalErr != nil {
		return nil, fmt.Errorf("rule %s not found", name)
	}

	mr := &MergedRule{Name: name}

	if nodeErr != nil {
		// Only global exists
		mr.Content = globalRule.Content
		mr.MergeMode = globalRule.MergeMode
		mr.Source = "global"
		mr.Hash = hashContent(globalRule.Content)
		mr.UpdatedAt = globalRule.UpdatedAt.Format(time.RFC3339)
		return mr, nil
	}

	if globalErr != nil {
		// Only node exists
		mr.Content = nodeRule.Content
		mr.MergeMode = nodeRule.MergeMode
		mr.Source = "node"
		mr.Hash = hashContent(nodeRule.Content)
		mr.UpdatedAt = nodeRule.UpdatedAt.Format(time.RFC3339)
		return mr, nil
	}

	// Both exist: merge
	mergeMode := nodeRule.MergeMode
	if mergeMode == "" {
		mergeMode = "append"
	}
	mr.MergeMode = mergeMode
	mr.Source = "merged"
	mr.GlobalContent = normalizeRuleContent(globalRule.Content)
	mr.NodeContent = normalizeRuleContent(nodeRule.Content)

	switch mergeMode {
	case "append":
		mr.Content = joinRuleContent(globalRule.Content, nodeRule.Content)
	default:
		mr.Content = normalizeRuleContent(nodeRule.Content)
	}
	mr.Hash = hashContent(mr.Content)

	// Use the later of the two timestamps
	if nodeRule.UpdatedAt.After(globalRule.UpdatedAt) {
		mr.UpdatedAt = nodeRule.UpdatedAt.Format(time.RFC3339)
	} else {
		mr.UpdatedAt = globalRule.UpdatedAt.Format(time.RFC3339)
	}

	return mr, nil
}

// SaveNodeRule saves a node-scoped WAF rule.
// mergeMode: "append" or "replace". Default is "append" so that a node rule
// supplements (rather than silently replaces) the global rule — a node editing
// a rule inherits the global content plus its own additions.
func (s *Service) SaveNodeRule(ctx context.Context, name string, serverID int64, content, mergeMode string) error {
	if mergeMode == "" {
		mergeMode = "append"
	}
	content = normalizeRuleContent(content)
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
	return s.ruleRepo.Delete(ctx, name, serverID, "")
}

// --- WAF Rules (Domain-level, merged with global+node) ---

// ListDomainRules returns all domain-scoped rules for a server+domain.
func (s *Service) ListDomainRules(ctx context.Context, serverID int64, domain string) ([]entity.WafRule, error) {
	return s.ruleRepo.ListDomainRules(ctx, serverID, domain)
}

// DeleteDomainRule removes a domain-scoped WAF rule.
func (s *Service) DeleteDomainRule(ctx context.Context, name string, serverID int64, domain string) error {
	return s.ruleRepo.Delete(ctx, name, serverID, domain)
}

// mergeDomainRules merges global+node rules with domain-level rules.
// Takes the global+node merged result and applies domain-level overrides:
//   - append: domain content appended to merged result
//   - replace: domain content replaces merged result entirely
// Returns the final merged content for each rule file.
func (s *Service) mergeDomainRules(ctx context.Context, baseMerged map[string]string, serverID int64, domain string) map[string]string {
	result := make(map[string]string, len(baseMerged))
	// Copy base merged
	for k, v := range baseMerged {
		result[k] = v
	}

	// Apply domain overrides
	domainRules, err := s.ruleRepo.ListDomainRules(ctx, serverID, domain)
	if err != nil {
		return result // no domain rules, return base
	}

	for _, dr := range domainRules {
		if !dr.Enabled {
			continue
		}
		base, hasBase := result[dr.Name]
		mode := dr.MergeMode
		if mode == "" {
			mode = "append"
		}
		if !hasBase || dr.Content == "" {
			// No base or empty domain content — use domain content as-is
			result[dr.Name] = dr.Content
			continue
		}
		switch mode {
		case "append":
			result[dr.Name] = joinRuleContent(base, dr.Content)
		default: // replace
			result[dr.Name] = normalizeRuleContent(dr.Content)
		}
	}

	return result
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

// GetDomainJSONContent returns the domain.json content for a server.
// This builds the JSON from all domain configs in the database.
func (s *Service) GetDomainJSONContent(ctx context.Context, serverID int64) (string, error) {
	return s.buildDomainJSON(ctx, serverID)
}

// SaveDomainConfigsBulk replaces all domain configs for a server from a JSON object.
// The JSON format is: {"domain1": {...config...}, "domain2": {...config...}}
func (s *Service) SaveDomainConfigsBulk(ctx context.Context, serverID int64, content string) error {
	// Parse the JSON content
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return fmt.Errorf("invalid domain JSON: %w", err)
	}

	// Delete all existing domain configs for this server
	if err := s.domainRepo.DeleteByServer(ctx, serverID); err != nil {
		return fmt.Errorf("failed to clear domain configs: %w", err)
	}

	// Insert each domain config
	for domain, cfgRaw := range raw {
		if domain == "_comment" {
			continue
		}
		cfg := &entity.WafDomainConfig{
			ServerID: serverID,
			Domain:   domain,
			Config:   string(cfgRaw),
		}
		if err := s.domainRepo.Upsert(ctx, cfg); err != nil {
			log.Printf("[WAF] failed to upsert domain config for %s: %v", domain, err)
			continue
		}
	}

	return nil
}

// DeleteDomainConfig removes a domain WAF config.
func (s *Service) DeleteDomainConfig(ctx context.Context, serverID int64, domain string) error {
	return s.domainRepo.DeleteByServerAndDomain(ctx, serverID, domain)
}

// --- GitHub Sync ---

// githubRawBase is the default base URL for rule files in the nginxguard repo.
// Used only when the user has not configured a custom source URL.
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
// These are NOT downloaded from GitHub — syncing them would overwrite
// user-entered data with repo example/placeholder content.
var userDefinedRules = []githubRuleDefault{
	{Name: "blackip.rule", MergeMode: "append"},
	{Name: "whiteip.rule", MergeMode: "append"},
	{Name: "whiteurl.rule", MergeMode: "append"},
	{Name: "cdnip.rule", MergeMode: "replace"},
}

// isUserDefinedRule returns true if the file name is a user-defined rule
// that should never be overwritten by GitHub sync.
func isUserDefinedRule(name string) bool {
	for _, r := range userDefinedRules {
		if r.Name == name {
			return true
		}
	}
	return false
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

// SyncFromGitHub downloads rule files from the configured source.
// Reads the user-saved source config (URL, file mappings) from settings.
// Falls back to the default GitHub source only if no config has been saved.
// Only syncs global-scope rules. Node-level rules are preserved.
func (s *Service) SyncFromGitHub(ctx context.Context) error {
	// Try to read user-saved config
	source, _ := s.GetGitHubSource(ctx)
	if source != nil && (source.URL != "" || len(source.FileMappings) > 0) {
		return s.SyncFromGitHubWithSource(ctx, source.URL, source.Files, source.FileMappings)
	}
	// No user config: use defaults
	return s.SyncFromGitHubWithSource(ctx, githubRawBase, "", nil)
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
// After writing root-level rules, syncs the merged content to every domain rule
// directory so that domain-specific rule files stay in sync with global updates.
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

	// Sync merged rules to domain-specific directories.
	// For each domain with WAF enabled and a custom rule_dir, merge global+node
	// rules with domain-level rules (from DB) and write the complete result to
	// the domain directory. This ensures:
	//   1. Domain has a complete rule set (no missing checks → no bypassed attacks)
	//   2. Domain-level user customizations are preserved (stored in DB, not overwritten)
	//   3. WAF engine reads from domain directory directly (no fallback needed)
	if domainConfigs, err := s.domainRepo.ListByServer(ctx, serverID); err == nil {
		for _, dc := range domainConfigs {
			var cfg map[string]interface{}
			if json.Unmarshal([]byte(dc.Config), &cfg) != nil {
				continue
			}
			wafEnable, _ := cfg["waf_enable"].(string)
			if wafEnable != "on" {
				continue
			}
			ruleDir, _ := cfg["rule_dir"].(string)
			if ruleDir == "" {
				continue // No custom rule_dir → domain uses global rules directly
			}
			// Three-layer merge: global+node (already in `merged`) + domain rules
			domainMerged := s.mergeDomainRules(ctx, merged, serverID, dc.Domain)
			domainDir := filepath.Join(s.wafRulesDir(), ruleDir)
			os.MkdirAll(domainDir, 0755)
			for name, content := range domainMerged {
				filePath := filepath.Join(domainDir, name)
				if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
					log.Printf("[WAF] failed to sync rule %s to domain %s: %v", name, dc.Domain, err)
				}
			}
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
	// config.lua must go to proxy/lua/waf/config.lua — the location Nginx's Lua
	// engine actually loads via `require 'config'`. Writing it to waf-rules/ would
	// create a dead file that nobody reads.
	nginxCfg, err := s.GetConfig(ctx, serverID, "nginx")
	if err == nil && nginxCfg != "" {
		nginxConfigPath := filepath.Join(s.dataDir, "proxy", "lua", "waf", "config.lua")
		if err := os.WriteFile(nginxConfigPath, []byte(nginxCfg), 0644); err != nil {
			log.Printf("[WAF] failed to write config.lua: %v", err)
		}
		// Remove the old dead config.lua from waf-rules/ if it exists (legacy)
		deadPath := filepath.Join(ruleDir, "config.lua")
		if _, err := os.Stat(deadPath); err == nil {
			os.Remove(deadPath)
			log.Printf("[WAF] removed dead config.lua from waf-rules/")
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

// mergedRuleEntry holds the merged content and metadata for a rule.
type mergedRuleEntry struct {
	content   string
	mergeMode string
	source    string // "global" | "node" | "merged"
	updatedAt string
}

// mergeRules merges global and node-level rules into a single map.
// Each rule carries its own merge_mode:
//   - "append":   node content is appended to global content (global + "\n" + node)
//   - "replace":  node content replaces global content entirely
//
// Rules with no global counterpart are included as-is.
// Returns map[ruleName]content.
func (s *Service) mergeRules(ctx context.Context, rules []entity.WafRule, serverID int64) map[string]string {
	merged := s.mergeRulesWithMeta(ctx, rules, serverID)
	result := make(map[string]string, len(merged))
	for name, mr := range merged {
		result[name] = mr.content
	}
	return result
}

// mergeRulesWithMeta merges global and node-level rules with source metadata.
// Returns map[ruleName]mergedRuleEntry with content, mergeMode, source, and updatedAt.
func (s *Service) mergeRulesWithMeta(ctx context.Context, rules []entity.WafRule, serverID int64) map[string]mergedRuleEntry {
	type ruleEntry struct {
		content   string
		mergeMode string
		updatedAt time.Time
	}
	globalRules := make(map[string]ruleEntry)
	nodeRules := make(map[string]ruleEntry)

	for _, r := range rules {
		entry := ruleEntry{content: r.Content, mergeMode: r.MergeMode, updatedAt: r.UpdatedAt}
		if entry.mergeMode == "" {
			entry.mergeMode = "replace"
		}
		if r.RuleScope == "global" {
			globalRules[r.Name] = entry
		} else if r.RuleScope == "node" && r.ServerID == serverID {
			nodeRules[r.Name] = entry
		}
	}

	merged := make(map[string]mergedRuleEntry)

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
		var latestTS time.Time
		if hasGlobal && hasNode {
			if gEntry.updatedAt.After(nEntry.updatedAt) {
				latestTS = gEntry.updatedAt
			} else {
				latestTS = nEntry.updatedAt
			}
		} else if hasGlobal {
			latestTS = gEntry.updatedAt
		} else if hasNode {
			latestTS = nEntry.updatedAt
		}

		if !hasNode || nEntry.content == "" {
			// No node override, use global as-is
			if hasGlobal {
				merged[name] = mergedRuleEntry{
					content:   gEntry.content,
					mergeMode: gEntry.mergeMode,
					source:    "global",
					updatedAt: latestTS.Format(time.RFC3339),
				}
			}
			continue
		}

		if !hasGlobal {
			// Node-only rule
			merged[name] = mergedRuleEntry{
				content:   nEntry.content,
				mergeMode: nEntry.mergeMode,
				source:    "node",
				updatedAt: latestTS.Format(time.RFC3339),
			}
			continue
		}

		// Both exist: merge based on the node rule's merge_mode
		var content string
		switch nEntry.mergeMode {
		case "append":
			content = joinRuleContent(gEntry.content, nEntry.content)
		default: // "replace"
			content = normalizeRuleContent(nEntry.content)
		}
		merged[name] = mergedRuleEntry{
			content:   content,
			mergeMode: nEntry.mergeMode,
			source:    "merged",
			updatedAt: latestTS.Format(time.RFC3339),
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
// 5. Domain-specific .rule files (domains/{domain}/*.rule)
//    — built from DB, not disk: for each domain with waf_enable=on,
//      push the merged rule content so remote Agents get their own copy.
func (s *Service) CollectAllFiles(ctx context.Context, serverID int64) ([]*WAFFile, error) {
	return s.CollectAllFilesForProxyTypes(ctx, serverID, []string{"nginx", "caddy"})
}

// CollectAllFilesForProxyTypes collects WAF files but only includes
// config files (config.lua for nginx, config.json for caddy) for proxies
// that are actually installed on the target node. Rule files, domain.json,
// and domain-specific rules are always included since they are shared.
func (s *Service) CollectAllFilesForProxyTypes(ctx context.Context, serverID int64, proxyTypes []string) ([]*WAFFile, error) {
	installed := make(map[string]bool)
	for _, pt := range proxyTypes {
		installed[pt] = true
	}

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

	// 5. Domain-specific .rule files — three-layer merged (global+node+domain).
	// For each domain that has WAF enabled AND has a custom rule_dir,
	// merge global+node rules with domain-level rules (from DB) and push
	// the complete result to domains/{rule_dir}/{filename}.
	// This ensures remote nodes get complete domain rule sets.
	if domainConfigs, err := s.domainRepo.ListByServer(ctx, serverID); err == nil {
		for _, dc := range domainConfigs {
			var cfg map[string]interface{}
			if json.Unmarshal([]byte(dc.Config), &cfg) != nil {
				continue
			}
			wafEnable, _ := cfg["waf_enable"].(string)
			if wafEnable != "on" {
				continue
			}
			ruleDir, _ := cfg["rule_dir"].(string)
			if ruleDir == "" {
				continue // No custom rule_dir → domain uses global rules
			}
			// Three-layer merge: global+node (already in `merged`) + domain rules
			domainMerged := s.mergeDomainRules(ctx, merged, serverID, dc.Domain)
			for name, content := range domainMerged {
				relPath := filepath.Join(ruleDir, name)
				files = append(files, &WAFFile{
					Path:    relPath,
					Content: []byte(content),
					Hash:    hashContent(content),
				})
			}
		}
	}

	// 2. Caddy config (config.json) — only if caddy is installed on the node
	if installed["caddy"] {
		caddyCfg, err := s.GetConfig(ctx, serverID, "caddy")
		if err == nil && caddyCfg != "" {
			files = append(files, &WAFFile{
				Path:    "config.json",
				Content: []byte(caddyCfg),
				Hash:    hashContent(caddyCfg),
			})
		}
	}

	// 3. Nginx config (config.lua) — only if nginx is installed on the node
	if installed["nginx"] {
		nginxCfg, err := s.GetConfig(ctx, serverID, "nginx")
		if err == nil && nginxCfg != "" {
			files = append(files, &WAFFile{
				Path:    "config.lua",
				Content: []byte(nginxCfg),
				Hash:    hashContent(nginxCfg),
			})
		}
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

// SyncToAllNodesFromDB pushes all WAF files to all known nodes.
// Uses the NodeLister to get all server IDs, then pushes to each.
// Local node (ID=1) is skipped (already landed to disk).
func (s *Service) SyncToAllNodesFromDB(ctx context.Context) error {
	if s.nodeLister == nil {
		return nil // no lister configured, skip
	}
	nodeIDs, err := s.nodeLister.ListAllServerIDs(ctx)
	if err != nil {
		return fmt.Errorf("list server IDs: %w", err)
	}
	var lastErr error
	for _, id := range nodeIDs {
		if id == 1 {
			continue // skip local node, already landed
		}
		if err := s.SyncToNode(ctx, id); err != nil {
			log.Printf("[WAF] push to server %d failed: %v", id, err)
			lastErr = err
		} else {
			log.Printf("[WAF] pushed rules to server %d", id)
		}
	}
	return lastErr
}

// GetGitHubSource returns the configured GitHub source URL and sync schedule.
func (s *Service) GetGitHubSource(ctx context.Context) (*GitHubSourceConfig, error) {
	if s.settingSvc == nil {
		return &GitHubSourceConfig{}, nil
	}
	val, err := s.settingSvc.GetByKey(ctx, "waf.github_source")
	if err != nil || val == "" {
		return &GitHubSourceConfig{}, nil
	}
	var cfg GitHubSourceConfig
	if err := json.Unmarshal([]byte(val), &cfg); err != nil {
		return &GitHubSourceConfig{}, nil
	}
	return &cfg, nil
}

// SaveGitHubSource saves the source URL, file mappings, and sync schedule.
func (s *Service) SaveGitHubSource(ctx context.Context, cfg *GitHubSourceConfig) error {
	if s.settingSvc == nil {
		return fmt.Errorf("setting store not configured")
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return s.settingSvc.Set(ctx, "waf.github_source", string(data))
}

// SyncFromGitHubWithSource downloads rule files from a configurable source.
// sourceURL is the base URL (used when a file mapping has no explicit URL).
// files is a legacy comma-separated list (used when fileMappings is empty).
// fileMappings is the preferred format: each entry has Name + optional full URL.
// If a mapping's URL is non-empty, it is used directly (no concatenation).
// If a mapping's URL is empty, the file is downloaded from sourceURL + "/" + Name.
func (s *Service) SyncFromGitHubWithSource(ctx context.Context, sourceURL, files string, fileMappings []SourceFileMapping) error {
	client := s.githubHTTPClient(ctx, 60*time.Second)

	if sourceURL == "" {
		// Try reading user-saved config as fallback
		source, _ := s.GetGitHubSource(ctx)
		if source != nil && source.URL != "" {
			sourceURL = source.URL
			// Also pick up file mappings if the caller didn't provide any
			if len(fileMappings) == 0 && len(source.FileMappings) > 0 {
				fileMappings = source.FileMappings
			}
		} else {
			sourceURL = githubRawBase
		}
	}

	// Build merge-mode lookup from known rules
	mergeMap := make(map[string]string)
	for _, rd := range defaultRuleFiles {
		mergeMap[rd.Name] = rd.MergeMode
	}
	for _, rd := range userDefinedRules {
		mergeMap[rd.Name] = rd.MergeMode
	}

	type syncItem struct {
		name string
		url  string
		mm   string
	}
	var items []syncItem

	if len(fileMappings) > 0 {
		// Use file mappings (preferred path) — skip user-defined rules
		// so GitHub sync never overwrites user-entered data.
		for _, fm := range fileMappings {
			name := strings.TrimSpace(fm.Name)
			if name == "" || isUserDefinedRule(name) {
				continue
			}
			mm := mergeMap[name]
			if mm == "" {
				mm = "replace"
			}
			items = append(items, syncItem{name: name, url: strings.TrimSpace(fm.URL), mm: mm})
		}
	} else if files != "" {
		// Legacy: comma-separated file names, URL = sourceURL + "/" + name
		for _, name := range strings.Split(files, ",") {
			name = strings.TrimSpace(name)
			if name == "" || isUserDefinedRule(name) {
				continue
			}
			mm := mergeMap[name]
			if mm == "" {
				mm = "replace"
			}
			items = append(items, syncItem{name: name, url: "", mm: mm})
		}
	} else {
		// Use defaults
		for _, rd := range defaultRuleFiles {
			items = append(items, syncItem{name: rd.Name, url: "", mm: rd.MergeMode})
		}
	}

	for _, item := range items {
		// Determine the full download URL
		rawURL := item.url
		if rawURL == "" {
			// Fall back to base URL + "/" + filename
			rawURL = strings.TrimSuffix(sourceURL, "/") + "/" + item.name
		}
		// else: rawURL is the full address provided by the user — use as-is

		req, _ := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("[WAF] failed to download %s from %s: %v", item.name, rawURL, err)
			continue
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			log.Printf("[WAF] failed to read %s: %v", item.name, err)
			continue
		}

		content := string(body)
		rule := &entity.WafRule{
			Name:      item.name,
			RuleScope: "global",
			ServerID:  0,
			MergeMode: item.mm,
			Content:   content,
			Hash:      hashContent(content),
			Enabled:   true,
		}
		if err := s.ruleRepo.Upsert(ctx, rule); err != nil {
			log.Printf("[WAF] failed to upsert rule %s: %v", item.name, err)
			continue
		}
		log.Printf("[WAF] synced rule %s from %s (%d bytes, merge=%s)", item.name, rawURL, len(body), item.mm)
	}

	return nil
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

// normalizeRuleContent trims surrounding whitespace and collapses runs of blank
// lines into a single newline, so stored/merged rule files stay clean.
func normalizeRuleContent(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines))
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if t == "" {
			if len(out) > 0 && out[len(out)-1] != "" {
				out = append(out, "")
			}
			continue
		}
		out = append(out, t)
	}
	// Drop a single trailing blank line left by the collapse above.
	if len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return strings.Join(out, "\n")
}

// joinRuleContent merges global + node rule text with a single newline separator,
// trimming both sides and ignoring empty halves. This avoids doubled blank lines
// when the global text already ends (or the node text starts) with a newline.
//
// In append mode the node content may contain a copy of the global rules plus
// a few extra lines. To avoid duplicating the global block, we deduplicate by
// line: any line in the node content that already appears in the global content
// is dropped, keeping only the genuinely new lines.
func joinRuleContent(global, node string) string {
	g := normalizeRuleContent(global)
	n := normalizeRuleContent(node)
	switch {
	case g == "" && n == "":
		return ""
	case g == "":
		return n
	case n == "":
		return g
	}

	// Build a set of lines already present in the global block.
	globalLines := strings.Split(g, "\n")
	seen := make(map[string]bool, len(globalLines))
	for _, ln := range globalLines {
		seen[ln] = true
	}

	// Keep only node lines that are NOT in the global block.
	// Empty lines are also dropped so they don't sneak through
	// the dedup check (an empty line in the node content that
	// happens between global-copy and new lines would survive
	// because the global block, after normalizeRuleContent, has
	// no trailing empty lines).
	nodeLines := strings.Split(n, "\n")
	var extra []string
	for _, ln := range nodeLines {
		if ln == "" {
			continue
		}
		if !seen[ln] {
			extra = append(extra, ln)
		}
	}

	if len(extra) == 0 {
		return g // All node lines were duplicates of global
	}
	return g + "\n" + strings.Join(extra, "\n")
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

// GetDomainRuleFile reads a domain-scoped rule from the DB.
// Returns the domain-specific content only. If the domain doesn't have
// this rule in the DB, returns empty string.
func (s *Service) GetDomainRuleFile(ctx context.Context, domain, name string, serverID int64) (string, error) {
	rule, err := s.ruleRepo.GetDomainByName(ctx, name, serverID, domain)
	if err != nil {
		return "", err
	}
	return rule.Content, nil
}

// GetDomainRuleWithGlobal returns the global rule content (read-only reference),
// the domain-specific rule content (editable, from DB), and the domain rule's
// merge_mode ("append" or "replace").
// If the domain doesn't have its own rule in the DB, domainContent is empty
// and mergeMode defaults to "append".
func (s *Service) GetDomainRuleWithGlobal(ctx context.Context, domain, name string, serverID int64) (globalContent, domainContent, mergeMode string, err error) {
	// Get global rule from DB
	globalRule, gErr := s.ruleRepo.GetGlobalByName(ctx, name)
	if gErr == nil {
		globalContent = globalRule.Content
	}

	// Get domain-specific rule from DB
	domainRule, dErr := s.ruleRepo.GetDomainByName(ctx, name, serverID, domain)
	if dErr == nil {
		domainContent = domainRule.Content
		mergeMode = domainRule.MergeMode
	}
	if mergeMode == "" {
		mergeMode = "append"
	}

	return globalContent, domainContent, mergeMode, nil
}

// SaveDomainRuleFileWithMode saves a domain-scoped rule to the DB with a merge_mode.
// merge_mode controls how the domain rule combines with the global+node merged result:
//   - "append":  domain content appended to global+node (supplements existing rules)
//   - "replace": domain content replaces global+node entirely (independent rule set)
func (s *Service) SaveDomainRuleFileWithMode(ctx context.Context, domain, name, content, mergeMode string, serverID int64) error {
	if mergeMode == "" {
		mergeMode = "append"
	}
	rule := &entity.WafRule{
		Name:      name,
		RuleScope: "domain",
		ServerID:  serverID,
		Domain:    domain,
		MergeMode: mergeMode,
		Content:   content,
		Hash:      hashContent(content),
		Enabled:   true,
		UpdatedAt: time.Now(),
	}
	return s.ruleRepo.Upsert(ctx, rule)
}

// InitDomainRules creates an empty directory for a domain's WAF rules.
// Domain rules are stored in the DB, not on disk — the directory is only
// needed for LandRulesForServer to write merged results to.
func (s *Service) InitDomainRules(ctx context.Context, domain string) error {
	domainDir := filepath.Join(s.wafRulesDir(), "domains", domain)
	return os.MkdirAll(domainDir, 0755)
}
