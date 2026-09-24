package system

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"corefusion/internal/constants"
	"corefusion/internal/model/entity"
	"corefusion/internal/security/crypto"
	"corefusion/internal/store/db"
	"corefusion/internal/store/repository"
)

// SettingService handles system settings business logic.
type SettingService struct {
	repo     *repository.SystemSettingRepository
	tmplRepo *repository.NotificationTemplateRepository
}

// sensitiveSettingKeys 是需要 AES-256-GCM 加密落库的敏感配置键。
// 与充值卡密钥 / TOTP secret 共用同一套 crypto（CF_SECRET_ENCRYPTION_KEY）。
// 命中这些键的值在写库前 EncryptSecret、读出后 DecryptSecret；
// 其它键（域名、开关、端口等）保持明文。
var sensitiveSettingKeys = map[string]bool{
	"notify.telegram_bot_token":  true,
	"notify.smtp_pass":           true,
	"oauth.github_client_secret": true,
	"oauth.google_client_secret": true,
}

// isSensitiveSetting 判断键是否需要在写库时加密。
func isSensitiveSetting(key string) bool { return sensitiveSettingKeys[key] }

// encryptSensitiveValue 写库前加密敏感值（空值 / 已加密值原样返回，防重复加密）。
func encryptSensitiveValue(key, value string) (string, error) {
	if !isSensitiveSetting(key) || value == "" || crypto.IsEncrypted(value) {
		return value, nil
	}
	return crypto.EncryptSecret(value)
}

// decryptSensitiveValue 读出后解密敏感值（对存量明文向后兼容：非 enc: 前缀原样返回）。
func decryptSensitiveValue(key, value string) string {
	if !isSensitiveSetting(key) || value == "" || !crypto.IsEncrypted(value) {
		return value
	}
	plain, err := crypto.DecryptSecret(value)
	if err != nil {
		return value // 解密失败保留原值，避免破坏存量数据
	}
	return plain
}

// EncryptExistingSecrets 把存量明文敏感配置加密落库（幂等，启动时调用）。
// 只在"历史上明文存储"的键上生效：已加密 / 空值跳过。
func (s *SettingService) EncryptExistingSecrets(ctx context.Context) error {
	for key := range sensitiveSettingKeys {
		v, err := s.repo.GetByKey(ctx, key)
		if err != nil || v == "" || crypto.IsEncrypted(v) {
			continue
		}
		enc, err := crypto.EncryptSecret(v)
		if err != nil {
			return fmt.Errorf("encrypt %s: %w", key, err)
		}
		if err := s.repo.Set(ctx, key, enc); err != nil {
			return fmt.Errorf("save %s: %w", key, err)
		}
	}
	return nil
}

// NewSettingService creates a new setting service.
func NewSettingService(repo *repository.SystemSettingRepository, tmplRepo *repository.NotificationTemplateRepository) *SettingService {
	return &SettingService{repo: repo, tmplRepo: tmplRepo}
}

// PanelConfig contains all panel configuration fields.
type PanelConfig struct {
	ListenAddr          string `json:"listen_addr"`
	ListenPort          string `json:"listen_port"`
	SubscriptionPort    string `json:"subscription_port"`
	PortalPort          string `json:"portal_port"`
	RegistrationEnabled bool   `json:"registration_enabled"`
	TLSEnabled          bool   `json:"tls_enabled"`
	TLSCertID           int64  `json:"tls_cert_id"`
	TLSCertPath         string `json:"tls_cert_path"`
	TLSKeyPath          string `json:"tls_key_path"`
	MTLSEnabled         bool   `json:"mtls_enabled"`
	MTLSCAPath          string `json:"mtls_ca_path"`
	AgentGRPCPort       string `json:"agent_grpc_port"` // Agent gRPC 监听端口 (默认 18443)
	WebPath             string `json:"web_path"`
	Timezone            string `json:"timezone"`
	DataDir             string `json:"data_dir"`
	TrustedProxies      string `json:"trusted_proxies"`
	ProfileWebPageUrl   string `json:"profile_web_page_url"`
	Mode                string `json:"mode"` // "full" (default) or "management_only"
	// PasskeyRPIDs 是 Passkey（WebAuthn）允许的 RP ID 白名单，逗号分隔。
	// WebAuthn 的 RP ID 必须是可注册域名（IP 不行，localhost 例外），且凭证绑定 RP ID。
	// 命中规则：请求 Host == 条目 或为其子域（host == rp || HasSuffix(host, "."+rp)）。
	// 登录（未认证）阶段只认白名单已有条目，防止伪造 Host 拿数据；密码登录永远兜底。
	PasskeyRPIDs string `json:"passkey_rp_ids"`
	// ExportMaxRows 是**单次数据导出（CSV）的行数上限** ✓。
	// 原来这个上限在各导出 handler 里**各写各的** ✗（审计导出 5 万、充值卡导出 10 万），
	// 想调大小只能改代码重编译；现在做成设置项，设置页可直接改 ✓。
	// 非法/越界的值会被 NormalizeExportMaxRows 夹回合法范围 ✓，0 表示用默认值 ✓。
	ExportMaxRows int `json:"export_max_rows"`
}

// GitHubDomain represents a single GitHub acceleration domain.
type GitHubDomain struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	Enabled bool   `json:"enabled"`
}

// GitHubConfig contains GitHub acceleration configuration.
type GitHubConfig struct {
	Enabled  bool           `json:"enabled"`
	Domains  []GitHubDomain `json:"domains"`
	ProxyURL string         `json:"proxy_url"` // optional HTTP/SOCKS proxy URL (e.g. http://127.0.0.1:7890)
}

// DefaultGitHubConfig returns default GitHub acceleration configuration.
func DefaultGitHubConfig() *GitHubConfig {
	return &GitHubConfig{
		Enabled:  false,
		ProxyURL: "",
		Domains: []GitHubDomain{
			{Name: "gh-proxy.com", URL: "https://gh-proxy.com", Enabled: true},
			{Name: "geekertao", URL: "https://ghfile.geekertao.top", Enabled: false},
			{Name: "llkk.cc", URL: "https://gh.llkk.cc", Enabled: false},
			{Name: "ghfast.top", URL: "https://ghfast.top", Enabled: false},
		},
	}
}

// NotificationConfig contains notification configuration fields.
type NotificationConfig struct {
	TelegramEnabled  bool   `json:"telegram_enabled"`
	TelegramBotToken string `json:"telegram_bot_token"`
	TelegramChatID   string `json:"telegram_chat_id"`
	SMTPEnabled      bool   `json:"smtp_enabled"`
	SMTPHost         string `json:"smtp_host"`
	SMTPPort         string `json:"smtp_port"`
	SMTPUser         string `json:"smtp_user"`
	SMTPPass         string `json:"smtp_pass"`
	SMTPFrom         string `json:"smtp_from"`
	SMTPTLS          bool   `json:"smtp_tls"`
}

type SubscriptionBillingTemplateConfig struct {
	DefaultUsedTemplate      string   `json:"default_used_template"`
	DefaultRemainingTemplate string   `json:"default_remaining_template"`
	DefaultResetTemplate     string   `json:"default_reset_template"`
	DefaultExpiryTemplate    string   `json:"default_expiry_template"`
	Variables                []string `json:"variables,omitempty"`
}

// SchedulerConfig contains configurable intervals for background scheduled tasks.
// All intervals are in minutes. A value of 0 means "use default".
type SchedulerConfig struct {
	LogCleanupInterval         int `json:"log_cleanup_interval"`         // DB log cleanup check interval (default: 360 = 6h)
	ClientLogCleanupInterval   int `json:"client_log_cleanup_interval"`  // Expired client log package cleanup interval (default: 10 = 10m)
	WAFLogCleanupInterval      int `json:"waf_log_cleanup_interval"`     // WAF log file cleanup interval (default: 1440 = 24h)
	WAFLogRetainDays           int `json:"waf_log_retain_days"`          // WAF log retain days (default: 30)
	BackupCheckInterval        int `json:"backup_check_interval"`        // Backup scheduler check interval (default: 1 = 1m)
	CallbackRetryInterval      int `json:"callback_retry_interval"`      // Charge callback retry interval (default: 1 = 1m)
	ProcessingRecoveryInterval int `json:"processing_recovery_interval"` // Stuck processing recovery interval (default: 1 = 1m)
	OrderCleanupInterval       int `json:"order_cleanup_interval"`       // Expired order cleanup interval (default: 5 = 5m)
	TrialScanInterval          int `json:"trial_scan_interval"`          // Trial expiry scan interval (default: 60 = 1h)
	DowngradeScanInterval      int `json:"downgrade_scan_interval"`      // Pending downgrade scan interval (default: 5 = 5m)
	CFIPCheckInterval          int `json:"cfip_check_interval"`          // CFIP source fetch check interval (default: 5 = 5m)
}

// DefaultSchedulerConfig returns default scheduler configuration.
func DefaultSchedulerConfig() *SchedulerConfig {
	return &SchedulerConfig{
		LogCleanupInterval:         360,
		ClientLogCleanupInterval:   10,
		WAFLogCleanupInterval:      1440,
		WAFLogRetainDays:           30,
		BackupCheckInterval:        1,
		CallbackRetryInterval:      1,
		ProcessingRecoveryInterval: 1,
		OrderCleanupInterval:       5,
		TrialScanInterval:          60,
		DowngradeScanInterval:      5,
		CFIPCheckInterval:          5,
	}
}

// GetSchedulerConfig retrieves scheduler configuration from database.
func (s *SettingService) GetSchedulerConfig(ctx context.Context) (*SchedulerConfig, error) {
	settings, err := s.repo.GetByPrefix(ctx, "scheduler.")
	if err != nil {
		return nil, err
	}
	config := DefaultSchedulerConfig()
	setIntIfExist(settings, "scheduler.log_cleanup_interval", &config.LogCleanupInterval)
	setIntIfExist(settings, "scheduler.client_log_cleanup_interval", &config.ClientLogCleanupInterval)
	setIntIfExist(settings, "scheduler.waf_log_cleanup_interval", &config.WAFLogCleanupInterval)
	setIntIfExist(settings, "scheduler.waf_log_retain_days", &config.WAFLogRetainDays)
	setIntIfExist(settings, "scheduler.backup_check_interval", &config.BackupCheckInterval)
	setIntIfExist(settings, "scheduler.callback_retry_interval", &config.CallbackRetryInterval)
	setIntIfExist(settings, "scheduler.processing_recovery_interval", &config.ProcessingRecoveryInterval)
	setIntIfExist(settings, "scheduler.order_cleanup_interval", &config.OrderCleanupInterval)
	setIntIfExist(settings, "scheduler.trial_scan_interval", &config.TrialScanInterval)
	setIntIfExist(settings, "scheduler.downgrade_scan_interval", &config.DowngradeScanInterval)
	setIntIfExist(settings, "scheduler.cfip_check_interval", &config.CFIPCheckInterval)
	return config, nil
}

// UpdateSchedulerConfig saves scheduler configuration to database.
func (s *SettingService) UpdateSchedulerConfig(ctx context.Context, config *SchedulerConfig) error {
	settings := map[string]string{
		"scheduler.log_cleanup_interval":         strconv.Itoa(config.LogCleanupInterval),
		"scheduler.client_log_cleanup_interval":  strconv.Itoa(config.ClientLogCleanupInterval),
		"scheduler.waf_log_cleanup_interval":     strconv.Itoa(config.WAFLogCleanupInterval),
		"scheduler.waf_log_retain_days":          strconv.Itoa(config.WAFLogRetainDays),
		"scheduler.backup_check_interval":        strconv.Itoa(config.BackupCheckInterval),
		"scheduler.callback_retry_interval":      strconv.Itoa(config.CallbackRetryInterval),
		"scheduler.processing_recovery_interval": strconv.Itoa(config.ProcessingRecoveryInterval),
		"scheduler.order_cleanup_interval":       strconv.Itoa(config.OrderCleanupInterval),
		"scheduler.trial_scan_interval":          strconv.Itoa(config.TrialScanInterval),
		"scheduler.downgrade_scan_interval":      strconv.Itoa(config.DowngradeScanInterval),
		"scheduler.cfip_check_interval":          strconv.Itoa(config.CFIPCheckInterval),
	}
	for key, value := range settings {
		if err := s.repo.Set(ctx, key, value); err != nil {
			return err
		}
	}
	return nil
}

// DefaultPanelConfig returns default panel configuration.
// 导出上限的默认值与合法范围 ✓。上限存在是为了**防止一次导出把面板拖垮** ✗：
// 全表导出会一次性把几万行读进内存并生成响应体，行数失控时面板会 OOM。
const (
	DefaultExportMaxRows = 50000
	MinExportMaxRows     = 1000
	MaxExportMaxRows     = 2000000
)

// NormalizeExportMaxRows 把任意输入夹到合法范围 ✓：<=0 视为"未设置"→ 用默认值 ✓。
func NormalizeExportMaxRows(n int) int {
	if n <= 0 {
		return DefaultExportMaxRows
	}
	if n < MinExportMaxRows {
		return MinExportMaxRows
	}
	if n > MaxExportMaxRows {
		return MaxExportMaxRows
	}
	return n
}

func DefaultPanelConfig() *PanelConfig {
	return &PanelConfig{
		ListenAddr:       "0.0.0.0",
		ListenPort:       "8080",
		SubscriptionPort: "0",
		PortalPort:       "0",
		TLSEnabled:       false,
		AgentGRPCPort:    strconv.Itoa(constants.DefaultAgentGRPCPort),
		WebPath:          "/",
		Timezone:         "Asia/Shanghai",
		DataDir:          "data",
		ExportMaxRows:    DefaultExportMaxRows,
	}
}

func DefaultSubscriptionBillingTemplateConfig() *SubscriptionBillingTemplateConfig {
	config := &SubscriptionBillingTemplateConfig{
		DefaultUsedTemplate:      "Used: {used_human}",
		DefaultRemainingTemplate: "Remaining: {remaining_human}",
		DefaultResetTemplate:     "Reset In: {reset_human}",
		DefaultExpiryTemplate:    "Expires: {expires_human}",
		Variables: []string{
			"{remaining_human}",
			"{remaining_bytes}",
			"{remaining_gb}",
			"{used_human}",
			"{used_bytes}",
			"{used_gb}",
			"{quota_human}",
			"{quota_bytes}",
			"{quota_gb}",
			"{reset_human}",
			"{reset_days}",
			"{reset_at}",
			"{reset_date}",
			"{expires_human}",
			"{expires_at}",
			"{expires_date}",
			"{expires_unix}",
			"{quota_state}",
		},
	}
	return config
}

// GetPanelConfig retrieves panel configuration from database.
func (s *SettingService) GetPanelConfig(ctx context.Context) (*PanelConfig, error) {
	settings, err := s.repo.GetByPrefix(ctx, "panel.")
	if err != nil {
		return nil, err
	}

	config := DefaultPanelConfig()
	setIfExist(settings, "panel.listen_addr", &config.ListenAddr)
	setIfExist(settings, "panel.listen_port", &config.ListenPort)
	setIfExist(settings, "panel.subscription_port", &config.SubscriptionPort)
	setIfExist(settings, "panel.portal_port", &config.PortalPort)
	if v, ok := settings["panel.registration_enabled"]; ok {
		config.RegistrationEnabled = v == "true"
	}
	if v, ok := settings["panel.tls_enabled"]; ok {
		config.TLSEnabled = v == "true"
	}
	if v, ok := settings["panel.tls_cert_id"]; ok && v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			config.TLSCertID = id
		}
	}
	setIfExist(settings, "panel.tls_cert_path", &config.TLSCertPath)
	setIfExist(settings, "panel.tls_key_path", &config.TLSKeyPath)
	if v, ok := settings["panel.mtls_enabled"]; ok {
		config.MTLSEnabled = v == "true"
	}
	setIfExist(settings, "panel.mtls_ca_path", &config.MTLSCAPath)
	setIfExist(settings, "panel.agent_grpc_port", &config.AgentGRPCPort)
	setIfExist(settings, "panel.web_path", &config.WebPath)
	setIfExist(settings, "panel.timezone", &config.Timezone)
	setIfExist(settings, "panel.data_dir", &config.DataDir)
	setIfExist(settings, "panel.trusted_proxies", &config.TrustedProxies)
	setIfExist(settings, "panel.profile_web_page_url", &config.ProfileWebPageUrl)
	setIfExist(settings, "panel.mode", &config.Mode)
	setIfExist(settings, "panel.passkey_rp_ids", &config.PasskeyRPIDs)
	if v, ok := settings["panel.export_max_rows"]; ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			config.ExportMaxRows = NormalizeExportMaxRows(n)
		}
	}

	return config, nil
}

// UpdatePanelConfig saves panel configuration to database.
func (s *SettingService) UpdatePanelConfig(ctx context.Context, config *PanelConfig) error {
	settings := map[string]string{
		"panel.listen_addr":          config.ListenAddr,
		"panel.listen_port":          config.ListenPort,
		"panel.subscription_port":    config.SubscriptionPort,
		"panel.portal_port":          config.PortalPort,
		"panel.tls_enabled":          fmt.Sprintf("%v", config.TLSEnabled),
		"panel.tls_cert_id":          strconv.FormatInt(config.TLSCertID, 10),
		"panel.tls_cert_path":        config.TLSCertPath,
		"panel.tls_key_path":         config.TLSKeyPath,
		"panel.mtls_enabled":         fmt.Sprintf("%v", config.MTLSEnabled),
		"panel.mtls_ca_path":         config.MTLSCAPath,
		"panel.agent_grpc_port":      config.AgentGRPCPort,
		"panel.web_path":             config.WebPath,
		"panel.timezone":             config.Timezone,
		"panel.data_dir":             config.DataDir,
		"panel.trusted_proxies":      config.TrustedProxies,
		"panel.profile_web_page_url": config.ProfileWebPageUrl,
		"panel.mode":                 config.Mode,
		"panel.passkey_rp_ids":       config.PasskeyRPIDs,
		// 导出上限：越界/非法一律先夹到合法范围再落库 ✓
		//（各导出 handler 拿到的值随时可用，不用再各自校验 ✓）
		"panel.export_max_rows": strconv.Itoa(NormalizeExportMaxRows(config.ExportMaxRows)),
	}

	// Read old data_dir BEFORE updating database
	oldDataDir := "data"
	if old, err := s.repo.GetByKey(ctx, "panel.data_dir"); err == nil && old != "" {
		oldDataDir = old
	}

	// Update all settings in database
	for key, value := range settings {
		if err := s.repo.Set(ctx, key, value); err != nil {
			return err
		}
	}

	// Update .env file with data directory
	if config.DataDir != "" {
		saveEnvFile("SAFE_PATHS", config.DataDir)
	}

	// If data directory changed, migrate data
	if config.DataDir != "" && oldDataDir != config.DataDir {
		if err := migrateDataDir(oldDataDir, config.DataDir); err != nil {
			return fmt.Errorf("migrate data: %w", err)
		}
	}

	return nil
}

// migrateDataDir copies data from old directory to new directory.
func migrateDataDir(oldDir, newDir string) error {
	// Resolve relative paths
	if !filepath.IsAbs(oldDir) {
		if abs, err := filepath.Abs(oldDir); err == nil {
			oldDir = abs
		}
	}
	if !filepath.IsAbs(newDir) {
		if abs, err := filepath.Abs(newDir); err == nil {
			newDir = abs
		}
	}

	// If same directory, nothing to do
	if oldDir == newDir {
		return nil
	}

	// Check old directory exists
	if _, err := os.Stat(oldDir); os.IsNotExist(err) {
		// Old dir doesn't exist, just create new one
		return os.MkdirAll(newDir, 0750)
	}

	// Create new directory
	if err := os.MkdirAll(newDir, 0750); err != nil {
		return fmt.Errorf("create new dir: %w", err)
	}

	// Copy all contents from old to new
	return copyDir(oldDir, newDir)
}

// copyDir recursively copies a directory.
func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			os.MkdirAll(dstPath, 0750)
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return err
			}
			os.MkdirAll(filepath.Dir(dstPath), 0750)
			if err := os.WriteFile(dstPath, data, 0600); err != nil {
				return err
			}
		}
	}
	return nil
}

// GetNotificationConfig retrieves notification configuration from database.
// 注意：必须走 s.GetByPrefix（内含敏感键解密），否则 SMTP 密码/Telegram token
// 会以密文返回到 handler，导致 ******** 保留逻辑把密文原样回写（密文永不变）。
func (s *SettingService) GetNotificationConfig(ctx context.Context) (*NotificationConfig, error) {
	settings, err := s.GetByPrefix(ctx, "notify.")
	if err != nil {
		return nil, err
	}

	config := &NotificationConfig{
		SMTPPort: "465",
		SMTPTLS:  true,
	}
	if v, ok := settings["notify.telegram_enabled"]; ok {
		config.TelegramEnabled = v == "true"
	}
	setIfExist(settings, "notify.telegram_bot_token", &config.TelegramBotToken)
	setIfExist(settings, "notify.telegram_chat_id", &config.TelegramChatID)
	if v, ok := settings["notify.smtp_enabled"]; ok {
		config.SMTPEnabled = v == "true"
	}
	setIfExist(settings, "notify.smtp_host", &config.SMTPHost)
	setIfExist(settings, "notify.smtp_port", &config.SMTPPort)
	setIfExist(settings, "notify.smtp_user", &config.SMTPUser)
	setIfExist(settings, "notify.smtp_pass", &config.SMTPPass)
	setIfExist(settings, "notify.smtp_from", &config.SMTPFrom)
	if v, ok := settings["notify.smtp_tls"]; ok {
		config.SMTPTLS = v == "true"
	}

	return config, nil
}

// UpdateNotificationConfig saves notification configuration to database.
func (s *SettingService) UpdateNotificationConfig(ctx context.Context, config *NotificationConfig) error {
	settings := map[string]string{
		"notify.telegram_enabled":   fmt.Sprintf("%v", config.TelegramEnabled),
		"notify.telegram_bot_token": config.TelegramBotToken,
		"notify.telegram_chat_id":   config.TelegramChatID,
		"notify.smtp_enabled":       fmt.Sprintf("%v", config.SMTPEnabled),
		"notify.smtp_host":          config.SMTPHost,
		"notify.smtp_port":          config.SMTPPort,
		"notify.smtp_user":          config.SMTPUser,
		"notify.smtp_pass":          config.SMTPPass,
		"notify.smtp_from":          config.SMTPFrom,
		"notify.smtp_tls":           fmt.Sprintf("%v", config.SMTPTLS),
	}

	for key, value := range settings {
		if err := s.Set(ctx, key, value); err != nil {
			return err
		}
	}
	return nil
}

// GetGitHubConfig retrieves GitHub acceleration configuration from database.
func (s *SettingService) GetGitHubConfig(ctx context.Context) (*GitHubConfig, error) {
	val, err := s.repo.GetByKey(ctx, "github.acceleration")
	if err != nil || val == "" {
		return DefaultGitHubConfig(), nil
	}

	config := DefaultGitHubConfig()
	if err := json.Unmarshal([]byte(val), config); err != nil {
		return DefaultGitHubConfig(), nil
	}
	return config, nil
}

// UpdateGitHubConfig saves GitHub acceleration configuration to database.
func (s *SettingService) UpdateGitHubConfig(ctx context.Context, config *GitHubConfig) error {
	data, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshal github config: %w", err)
	}
	return s.repo.Set(ctx, "github.acceleration", string(data))
}

func (s *SettingService) GetSubscriptionBillingTemplateConfig(ctx context.Context) (*SubscriptionBillingTemplateConfig, error) {
	config := DefaultSubscriptionBillingTemplateConfig()
	settings, err := s.repo.GetByPrefix(ctx, "subscription.billing_template.")
	if err != nil {
		return nil, err
	}
	setIfExist(settings, "subscription.billing_template.default_used", &config.DefaultUsedTemplate)
	setIfExist(settings, "subscription.billing_template.default_remaining", &config.DefaultRemainingTemplate)
	setIfExist(settings, "subscription.billing_template.default_reset", &config.DefaultResetTemplate)
	setIfExist(settings, "subscription.billing_template.default_expiry", &config.DefaultExpiryTemplate)
	return config, nil
}

func (s *SettingService) UpdateSubscriptionBillingTemplateConfig(ctx context.Context, config *SubscriptionBillingTemplateConfig) error {
	settings := map[string]string{
		"subscription.billing_template.default_used":      config.DefaultUsedTemplate,
		"subscription.billing_template.default_remaining": config.DefaultRemainingTemplate,
		"subscription.billing_template.default_reset":     config.DefaultResetTemplate,
		"subscription.billing_template.default_expiry":    config.DefaultExpiryTemplate,
	}
	for key, value := range settings {
		if err := s.repo.Set(ctx, key, value); err != nil {
			return err
		}
	}
	return nil
}

// GetActiveGitHubDomain returns the first enabled acceleration domain URL, or empty string.
func (s *SettingService) GetActiveGitHubDomain(ctx context.Context) string {
	config, err := s.GetGitHubConfig(ctx)
	if err != nil || !config.Enabled {
		return ""
	}
	for _, d := range config.Domains {
		if d.Enabled {
			return d.URL
		}
	}
	return ""
}

// GetGitHubProxyURL returns the configured HTTP/SOCKS proxy URL for GitHub
// requests, or empty string if none is set.
func (s *SettingService) GetGitHubProxyURL(ctx context.Context) string {
	config, err := s.GetGitHubConfig(ctx)
	if err != nil {
		return ""
	}
	return config.ProxyURL
}

// SaveMTLSCAFile saves an uploaded CA certificate file.
func (s *SettingService) SaveMTLSCAFile(ctx context.Context, dataDir string, content []byte) (string, error) {
	dir := filepath.Join(dataDir, "certs", "mtls")
	os.MkdirAll(dir, 0750)

	path := filepath.Join(dir, "ca.pem")
	if err := os.WriteFile(path, content, 0600); err != nil {
		return "", fmt.Errorf("write CA file: %w", err)
	}

	// Update setting
	if err := s.repo.Set(ctx, "panel.mtls_ca_path", path); err != nil {
		return "", err
	}

	return path, nil
}

// TestSMTP tests SMTP connection by sending a test email.
func (s *SettingService) TestSMTP(ctx context.Context, config *NotificationConfig) error {
	addr := config.SMTPHost + ":" + config.SMTPPort
	auth := smtp.PlainAuth("", config.SMTPUser, config.SMTPPass, config.SMTPHost)
	msg := []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: CoreFusion SMTP Test\r\n\r\nThis is a test email from CoreFusion.", config.SMTPFrom, config.SMTPFrom))
	return smtp.SendMail(addr, auth, config.SMTPFrom, []string{config.SMTPFrom}, msg)
}

// TestTelegram tests Telegram bot by sending a test message.
func (s *SettingService) TestTelegram(ctx context.Context, config *NotificationConfig) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", config.TelegramBotToken)
	payload := fmt.Sprintf(`{"chat_id":"%s","text":"CoreFusion Telegram Test"}`, config.TelegramChatID)

	client := &http.Client{}
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("telegram API returned %d", resp.StatusCode)
	}
	return nil
}

// --- Notification Templates ---

func (s *SettingService) ListTemplates(ctx context.Context) ([]entity.NotificationTemplate, error) {
	return s.tmplRepo.List(ctx)
}

func (s *SettingService) CreateTemplate(ctx context.Context, tmpl *entity.NotificationTemplate) error {
	return s.tmplRepo.Create(ctx, tmpl)
}

func (s *SettingService) UpdateTemplate(ctx context.Context, tmpl *entity.NotificationTemplate) error {
	return s.tmplRepo.Update(ctx, tmpl)
}

func (s *SettingService) DeleteTemplate(ctx context.Context, id int64) error {
	return s.tmplRepo.Delete(ctx, id)
}

// UpdateDataDir updates the data directory setting in DB and .env file.
func (s *SettingService) UpdateDataDir(ctx context.Context, dataDir string) error {
	if err := s.repo.Set(ctx, "panel.data_dir", dataDir); err != nil {
		return err
	}
	// Also update .env file
	return saveEnvFile("SAFE_PATHS", dataDir)
}

// MigrateDataToDir copies data from old directory to new directory.
func (s *SettingService) MigrateDataToDir(oldDir, newDir string) error {
	return migrateDataDir(oldDir, newDir)
}

// --- Database Config ---
// All database configuration is stored in the .env file only (like SAFE_PATHS).
// This avoids the chicken-and-egg problem: you need DB config to connect to DB.

// DBConfig contains database connection configuration.
// Read from .env file only. Changes take effect on next panel restart.
type DBConfig struct {
	Type          string `json:"type"`           // sqlite | mysql | postgres
	DSN           string `json:"dsn"`            // Connection string (passwords masked on GET)
	MaxOpenConns  int    `json:"max_open_conns"` // Pool: max open connections (MySQL/PostgreSQL)
	MaxIdleConns  int    `json:"max_idle_conns"` // Pool: max idle connections (MySQL/PostgreSQL)
	CurrentType   string `json:"current_type"`   // Read-only: type currently in use
	EffectiveDSN  string `json:"effective_dsn"`  // Read-only: DSN currently in use (masked)
	RestartNeeded bool   `json:"restart_needed"` // Read-only: true if config changed since startup
}

// GetDBConfig reads database configuration from the .env file.
// DSN passwords are masked for display; the real value is preserved in .env.
func (s *SettingService) GetDBConfig(ctx context.Context) (*DBConfig, error) {
	env := loadEnvFile()

	config := &DBConfig{
		Type:         "sqlite",
		MaxOpenConns: 100,
		MaxIdleConns: 10,
	}

	// Parse CF_DB_DSN
	dsn := env["CF_DB_DSN"]
	if dsn != "" {
		switch {
		case strings.HasPrefix(dsn, "mysql://"):
			config.Type = "mysql"
			config.DSN = maskDSN(dsn)
		case strings.HasPrefix(dsn, "postgres://"), strings.HasPrefix(dsn, "postgresql://"):
			config.Type = "postgres"
			config.DSN = maskDSN(dsn)
		case strings.HasPrefix(dsn, "sqlite://"):
			config.Type = "sqlite"
			config.DSN = strings.TrimPrefix(dsn, "sqlite://")
		default:
			config.Type = "sqlite"
			config.DSN = dsn // bare path
		}
	} else {
		// Default: derive from SAFE_PATHS
		safePaths := env["SAFE_PATHS"]
		if safePaths == "" {
			safePaths = "data"
		}
		config.DSN = filepath.Join(safePaths, "sqlite", "app.db")
	}

	// CF_DB_TYPE override
	if dbType := env["CF_DB_TYPE"]; dbType != "" {
		config.Type = dbType
	}

	// Pool settings
	if v := env["CF_DB_MAX_OPEN_CONNS"]; v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			config.MaxOpenConns = n
		}
	}
	if v := env["CF_DB_MAX_IDLE_CONNS"]; v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			config.MaxIdleConns = n
		}
	}

	// Current running config (set at startup)
	startType, startDSN := db.StartupConfig()
	config.CurrentType = startType
	config.EffectiveDSN = maskDSN(startDSN)

	// Check if restart needed (env changed since startup)
	// Compare the raw .env DSN with the startup DSN.
	// Since startDSN may have the scheme prefix stripped by ConfigFromEnv,
	// we normalize both for comparison.
	currentEnvDSN := env["CF_DB_DSN"]
	needsRestart := false
	if currentEnvDSN != "" && startDSN != "" {
		// Strip scheme prefix from env DSN for fair comparison
		envDSNNormalized := strings.TrimPrefix(currentEnvDSN, "mysql://")
		envDSNNormalized = strings.TrimPrefix(envDSNNormalized, "postgres://")
		envDSNNormalized = strings.TrimPrefix(envDSNNormalized, "postgresql://")
		envDSNNormalized = strings.TrimPrefix(envDSNNormalized, "sqlite://")
		needsRestart = (envDSNNormalized != startDSN)
	} else if currentEnvDSN != "" && startDSN == "" {
		// Env has config but startup used default SQLite
		needsRestart = true
	}
	config.RestartNeeded = needsRestart

	return config, nil
}

// UpdateDBConfig saves database configuration to the .env file.
// All fields are written to .env — nothing is stored in system_settings.
// Changes take effect on next panel restart.
func (s *SettingService) UpdateDBConfig(ctx context.Context, config *DBConfig) error {
	// Validate type
	switch config.Type {
	case "sqlite", "mysql", "postgres":
		// ok
	default:
		return fmt.Errorf("unsupported database type: %s", config.Type)
	}

	// Resolve DSN: if masked (unchanged by user), keep the existing .env value
	dsnForEnv := s.envDSNFor(config)

	// ⚠ 写 .env 之前**先确认连得上** ✗：否则一次手误（端口写错/密码写错）就会把面板
	// 指到一个连不上的库，重启即失联 ✗。连不上就不动 .env ✓。
	probe := s.targetConfig(config)
	probe.MaxOpenConns, probe.MaxIdleConns = 1, 1
	if err := db.TestConnection(probe); err != nil {
		return fmt.Errorf("database is not reachable, .env left unchanged: %w", err)
	}

	// Write CF_DB_DSN to .env（读回校验：写不进去就回滚旧内容 ✓）
	if err := saveEnvFileVerified("CF_DB_DSN", dsnForEnv); err != nil {
		return fmt.Errorf("save .env: %w", err)
	}

	// Write pool settings to .env（同样读回校验，不再忽略错误 ✗）
	if config.MaxOpenConns > 0 {
		if err := saveEnvFileVerified("CF_DB_MAX_OPEN_CONNS", strconv.Itoa(config.MaxOpenConns)); err != nil {
			return fmt.Errorf("save pool settings: %w", err)
		}
	}
	if config.MaxIdleConns > 0 {
		if err := saveEnvFileVerified("CF_DB_MAX_IDLE_CONNS", strconv.Itoa(config.MaxIdleConns)); err != nil {
			return fmt.Errorf("save pool settings: %w", err)
		}
	}

	return nil
}

// upsertEnvContent 在 .env 内容里 upsert 一个键（保持其它行原样；纯函数，便于测试）。
func upsertEnvContent(content, key, value string) string {
	lines := []string{}
	found := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 && strings.TrimSpace(parts[0]) == key {
				lines = append(lines, key+"="+value)
				found = true
				continue
			}
		}
		lines = append(lines, line)
	}
	if !found {
		lines = append(lines, key+"="+value)
	}
	return strings.Join(lines, "\n") + "\n"
}

// saveEnvFileVerified 写 .env 并**读回校验**，失败即回滚（"保证写成功"）。
//
// 为什么不能只信 WriteFile 的返回值：写到一半、磁盘满、被截断、写入的是别的路径……
// 都可能"返回成功但内容不对" ✗。而 .env 是**重启后生效的唯一依据** —— 一旦写坏：
//   - 写丢了 CF_DB_DSN → 重启后回落到默认 SQLite（看起来"数据全没了"✗）；
//   - 写丢了 JWT_SECRET → 所有人被登出 ✗。
//
// 所以：先备份旧内容 → 写 → 读回比对 → 不一致就回滚旧内容并报错 ✓。
func saveEnvFileVerified(key, value string) error {
	oldContent, readErr := os.ReadFile(".env")
	hadOld := readErr == nil

	if err := saveEnvFile(key, value); err != nil {
		return fmt.Errorf("write .env: %w", err)
	}

	// 读回校验（用与启动同一套解析逻辑，避免"写法/读法"不一致 ✗）
	if got := loadEnvFile()[key]; got != value {
		if hadOld {
			if rerr := os.WriteFile(".env", oldContent, 0600); rerr != nil {
				log.Printf("[setting] CRITICAL: .env verify failed and rollback failed: %v", rerr)
			} else {
				log.Printf("[setting] .env verify failed for %s, rolled back to previous content", key)
			}
		}
		return fmt.Errorf("verify .env: %s not persisted (want %q, got %q)", key, value, got)
	}
	return nil
}

// envDSNFor 返回要写进 `CF_DB_DSN` 的值（补齐类型前缀；掩码密码回退到已有值）。
func (s *SettingService) envDSNFor(config *DBConfig) string {
	dsn := s.resolveDSN(config.DSN)
	if config.Type == "mysql" && !strings.HasPrefix(dsn, "mysql://") {
		dsn = "mysql://" + dsn
	} else if config.Type == "postgres" && !strings.HasPrefix(dsn, "postgres://") && !strings.HasPrefix(dsn, "postgresql://") {
		dsn = "postgres://" + dsn
	}
	return dsn
}

// targetConfig 把前端传来的 DB 配置解析成 db.Config（含类型覆盖与连接池参数）。
func (s *SettingService) targetConfig(config *DBConfig) db.Config {
	dstCfg := db.ParseDSN(s.resolveDSN(config.DSN))
	if config.Type != "" && dstCfg.Type == "sqlite" && config.Type != "sqlite" {
		dstCfg.Type = config.Type
	}
	dstCfg.MaxOpenConns = config.MaxOpenConns
	dstCfg.MaxIdleConns = config.MaxIdleConns
	return dstCfg
}

// SwitchDatabase 把"切换到新数据库"做成一个**有序、可验证**的动作（"保证能写成功"）：
//
//	① 先确认**连得上**目标库；
//	② 再真的把数据写进去（db.MigrateTo 内部已有：写前预检 / 写入失败即失败 / 拷后行数核对）；
//	③ **最后**才写 `.env`（读回校验，失败回滚）—— 只有这一步成功，"切换"才算成功；
//	④ 再连一次目标库确认它确实可用。
//
// ⚠ 顺序是关键 ✗：旧流程是"保存配置（写 .env）"与"迁移数据"两个独立按钮，用户很容易
// **只保存不迁移**，或者迁移失败时 `.env` 已经指向了新库 → 重启后面板连上**空库**、
// 看起来"数据全丢了" ✗。现在只要迁移没成功，`.env` 一个字都不会改 ✓（面板继续用旧库 ✓）。
func (s *SettingService) SwitchDatabase(ctx context.Context, config *DBConfig) (tablesCopied int, rowsCopied int64, err error) {
	switch config.Type {
	case "sqlite", "mysql", "postgres":
		// ok
	default:
		return 0, 0, fmt.Errorf("unsupported database type: %s", config.Type)
	}

	dstCfg := s.targetConfig(config)
	dsnForEnv := s.envDSNFor(config)

	// ① 连得上才算候选
	dstCfg.MaxOpenConns, dstCfg.MaxIdleConns = 1, 1
	if err := db.TestConnection(dstCfg); err != nil {
		return 0, 0, fmt.Errorf("target database is not reachable: %w", err)
	}
	dstCfg = s.targetConfig(config) // 恢复真实连接池参数

	// ② 数据真的写进去（内部保证：预检 + 写入失败即失败 + 拷后行数核对）
	tablesCopied, rowsCopied, err = db.MigrateTo(dstCfg)
	if err != nil {
		return 0, 0, fmt.Errorf("migrate data: %w", err)
	}

	// ③ 最后才写 .env（读回校验 + 回滚）
	if err := saveEnvFileVerified("CF_DB_DSN", dsnForEnv); err != nil {
		return tablesCopied, rowsCopied, fmt.Errorf("persist CF_DB_DSN: %w", err)
	}
	if config.MaxOpenConns > 0 {
		if err := saveEnvFileVerified("CF_DB_MAX_OPEN_CONNS", strconv.Itoa(config.MaxOpenConns)); err != nil {
			return tablesCopied, rowsCopied, fmt.Errorf("persist CF_DB_MAX_OPEN_CONNS: %w", err)
		}
	}
	if config.MaxIdleConns > 0 {
		if err := saveEnvFileVerified("CF_DB_MAX_IDLE_CONNS", strconv.Itoa(config.MaxIdleConns)); err != nil {
			return tablesCopied, rowsCopied, fmt.Errorf("persist CF_DB_MAX_IDLE_CONNS: %w", err)
		}
	}

	// ④ 再连一次，确认新库确实可用（写进去之后仍然连得上）
	check := s.targetConfig(config)
	check.MaxOpenConns, check.MaxIdleConns = 1, 1
	if err := db.TestConnection(check); err != nil {
		return tablesCopied, rowsCopied, fmt.Errorf("target database unusable after switch: %w", err)
	}

	log.Printf("[setting] database switched: %d tables / %d rows copied; .env updated (restart to apply)",
		tablesCopied, rowsCopied)
	return tablesCopied, rowsCopied, nil
}

// TestDBConnection tests connectivity to a database using the given config.
// Does NOT use or modify the running database connection.
// If the DSN contains masked passwords (********), reads the real DSN from .env.
func (s *SettingService) TestDBConnection(ctx context.Context, config *DBConfig) error {
	dsn := s.resolveDSN(config.DSN)
	dbCfg := db.ParseDSN(dsn)
	// If type was explicitly set and DSN has no scheme, use the explicit type
	if config.Type != "" && dbCfg.Type == "sqlite" && config.Type != "sqlite" {
		dbCfg.Type = config.Type
	}
	dbCfg.MaxOpenConns = 1
	dbCfg.MaxIdleConns = 1
	return db.TestConnection(dbCfg)
}

// MigrateDBData copies all data from the current (running) database to the specified target database.
// The target database is specified by the given config.
// Before copying data, AutoMigrate is run on the target to create all tables.
// Warning: this will DELETE all existing data in target tables before inserting source data.
func (s *SettingService) MigrateDBData(ctx context.Context, dstConfig *DBConfig) (tablesCopied int, rowsCopied int64, err error) {
	dsn := s.resolveDSN(dstConfig.DSN)
	dstCfg := db.ParseDSN(dsn)
	if dstConfig.Type != "" && dstCfg.Type == "sqlite" && dstConfig.Type != "sqlite" {
		dstCfg.Type = dstConfig.Type
	}
	// Apply pool settings for target
	dstCfg.MaxOpenConns = dstConfig.MaxOpenConns
	dstCfg.MaxIdleConns = dstConfig.MaxIdleConns
	return db.MigrateTo(dstCfg)
}

// resolveDSN returns the real DSN. If the input contains masked passwords (********),
// it falls back to the actual DSN from .env (meaning the user didn't change the field).
func (s *SettingService) resolveDSN(dsn string) string {
	if strings.Contains(dsn, "********") {
		env := loadEnvFile()
		if envDSN := env["CF_DB_DSN"]; envDSN != "" {
			return envDSN
		}
	}
	// If DSN has no scheme prefix but type is mysql/postgres, the env may have the prefix
	// Try reading from .env to get the full DSN with scheme
	if !strings.HasPrefix(dsn, "mysql://") && !strings.HasPrefix(dsn, "postgres://") && strings.Contains(dsn, "@tcp(") {
		env := loadEnvFile()
		if envDSN := env["CF_DB_DSN"]; envDSN != "" {
			return envDSN
		}
	}
	return dsn
}

// maskDSN masks passwords in a DSN string for display.
func maskDSN(dsn string) string {
	if dsn == "" {
		return ""
	}
	// Bare path (SQLite) — no masking needed
	if !strings.Contains(dsn, "://") && !strings.Contains(dsn, "@") {
		return dsn
	}
	// Mask password in postgres://user:password@host/db
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		atIdx := strings.LastIndex(dsn, "@")
		schemeEnd := strings.Index(dsn, "://") + 3
		if atIdx > schemeEnd {
			userPart := dsn[schemeEnd:atIdx]
			if colonIdx := strings.Index(userPart, ":"); colonIdx >= 0 {
				return dsn[:schemeEnd+colonIdx+1] + "********" + dsn[atIdx:]
			}
		}
	}
	// Mask password in mysql://user:password@tcp(host:port)/db
	// Also handles bare format: user:password@tcp(host:port)/db (without mysql:// prefix)
	if strings.HasPrefix(dsn, "mysql://") {
		rest := strings.TrimPrefix(dsn, "mysql://")
		atIdx := strings.Index(rest, "@")
		if atIdx >= 0 {
			userPart := rest[:atIdx]
			if colonIdx := strings.Index(userPart, ":"); colonIdx >= 0 {
				return "mysql://" + userPart[:colonIdx+1] + "********" + rest[atIdx:]
			}
		}
	} else if strings.Contains(dsn, "@tcp(") {
		// Bare MySQL DSN without mysql:// prefix: user:password@tcp(host:port)/db
		atIdx := strings.Index(dsn, "@tcp(")
		if atIdx >= 0 {
			userPart := dsn[:atIdx]
			if colonIdx := strings.Index(userPart, ":"); colonIdx >= 0 {
				return userPart[:colonIdx+1] + "********" + dsn[atIdx:]
			}
		}
	}
	return dsn
}

// loadEnvFile reads the .env file and returns key-value pairs.
func loadEnvFile() map[string]string {
	env := make(map[string]string)
	data, err := os.ReadFile(".env")
	if err != nil {
		return env
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			env[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return env
}

// saveEnvFile updates a key in the .env file.
func saveEnvFile(key, value string) error {
	lines := []string{}
	found := false

	if data, err := os.ReadFile(".env"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
				parts := strings.SplitN(trimmed, "=", 2)
				if len(parts) == 2 && strings.TrimSpace(parts[0]) == key {
					lines = append(lines, key+"="+value)
					found = true
					continue
				}
			}
			lines = append(lines, line)
		}
	}

	if !found {
		lines = append(lines, key+"="+value)
	}

	// ⚠ .env 里可能有 **数据库密码**（CF_DB_DSN）/ JWT_SECRET 等 → 权限必须 0600 ✓
	// （安全审计 v5 P-4）。WriteFile 对**已存在**的文件不改权限 ✗ → 显式 Chmod 一次。
	envPath := ".env"
	if err := os.WriteFile(envPath, []byte(strings.Join(lines, "\n")+"\n"), 0600); err != nil {
		return err
	}
	if err := os.Chmod(envPath, 0600); err != nil {
		log.Printf("[setting] warning: chmod .env failed: %v", err)
	}
	return nil
}

// GetByKey retrieves a setting value by key (implements proxy.SettingStore).
func (s *SettingService) GetByKey(ctx context.Context, key string) (string, error) {
	v, err := s.repo.GetByKey(ctx, key)
	if err != nil {
		return "", err
	}
	return decryptSensitiveValue(key, v), nil
}

// GetByPrefix retrieves all settings with a given key prefix.
func (s *SettingService) GetByPrefix(ctx context.Context, prefix string) (map[string]string, error) {
	values, err := s.repo.GetByPrefix(ctx, prefix)
	if err != nil {
		return nil, err
	}
	for k, v := range values {
		values[k] = decryptSensitiveValue(k, v)
	}
	return values, nil
}

// Set saves a setting value by key (implements proxy.SettingStore).
// 敏感键在写库前自动加密。
func (s *SettingService) Set(ctx context.Context, key, value string) error {
	enc, err := encryptSensitiveValue(key, value)
	if err != nil {
		return err
	}
	return s.repo.Set(ctx, key, enc)
}

// SetByKey saves a setting value by key (implements portal.SettingStore).
func (s *SettingService) SetByKey(ctx context.Context, key, value string) error {
	return s.Set(ctx, key, value)
}

// Delete deletes a setting by key.
func (s *SettingService) Delete(ctx context.Context, key string) error {
	return s.repo.Delete(ctx, key)
}

func setIfExist(m map[string]string, key string, target *string) {
	if v, ok := m[key]; ok && v != "" {
		*target = v
	}
}

func setIntIfExist(m map[string]string, key string, target *int) {
	if v, ok := m[key]; ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			*target = n
		}
	}
}

// --- Encryption Key Config ---

// IsEncryptionKeyConfigured checks if CF_SECRET_ENCRYPTION_KEY is set in .env.
func (s *SettingService) IsEncryptionKeyConfigured() bool {
	env := loadEnvFile()
	key := env["CF_SECRET_ENCRYPTION_KEY"]
	return key != "" && key != "0000000000000000000000000000000000000000000000000000000000000000"
}

// GenerateAndSaveEncryptionKey generates a random 32-byte key and saves it to .env.
// Returns the generated key (for preview only) or error.
// WARNING: Once generated, do not change this key, or all encrypted data (charge cards, charge site secrets, etc.) will be lost!
func (s *SettingService) GenerateAndSaveEncryptionKey() (string, error) {
	// Generate random 32 bytes
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	key := hex.EncodeToString(b)

	// Save to .env file
	if err := saveEnvFile("CF_SECRET_ENCRYPTION_KEY", key); err != nil {
		return "", err
	}

	return key, nil
}

// --- JWT Secret Config ---

// JWTSecretStatus returns whether JWT_SECRET is configured and a masked preview.
func (s *SettingService) JWTSecretStatus() (configured bool, preview string) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		env := loadEnvFile()
		secret = env["JWT_SECRET"]
	}
	if secret == "" {
		return false, ""
	}
	if len(secret) <= 8 {
		return true, secret[:len(secret)/2] + "****"
	}
	return true, secret[:8] + "****"
}

// RegenerateJWTSecret generates a new random JWT secret and saves it to .env.
// Also updates the running process environment so the new secret takes effect
// immediately without requiring a restart.
func (s *SettingService) RegenerateJWTSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	secret := hex.EncodeToString(b)
	if err := saveEnvFile("JWT_SECRET", secret); err != nil {
		return "", err
	}
	os.Setenv("JWT_SECRET", secret)
	return secret, nil
}

// --- Env Config (SAFE_PATHS) ---

// GetSafePaths reads SAFE_PATHS from .env file. Returns "data" if not set.
func (s *SettingService) GetSafePaths() string {
	env := loadEnvFile()
	sp := env["SAFE_PATHS"]
	if sp == "" {
		sp = "data"
	}
	return sp
}

// UpdateSafePaths writes SAFE_PATHS to .env file. Restart required to take effect.
func (s *SettingService) UpdateSafePaths(safePaths string) error {
	return saveEnvFile("SAFE_PATHS", safePaths)
}
