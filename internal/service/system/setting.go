package system

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"corefusion/internal/constants"
	"corefusion/internal/model/entity"
	"corefusion/internal/store/db"
	"corefusion/internal/store/repository"
)

// SettingService handles system settings business logic.
type SettingService struct {
	repo     *repository.SystemSettingRepository
	tmplRepo *repository.NotificationTemplateRepository
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

// DefaultPanelConfig returns default panel configuration.
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
		"panel.agent_grpc_port":       config.AgentGRPCPort,
		"panel.web_path":             config.WebPath,
		"panel.timezone":             config.Timezone,
		"panel.data_dir":             config.DataDir,
		"panel.trusted_proxies":      config.TrustedProxies,
		"panel.profile_web_page_url":  config.ProfileWebPageUrl,
		"panel.mode":                 config.Mode,
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
func (s *SettingService) GetNotificationConfig(ctx context.Context) (*NotificationConfig, error) {
	settings, err := s.repo.GetByPrefix(ctx, "notify.")
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
		if err := s.repo.Set(ctx, key, value); err != nil {
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
	Type          string `json:"type"`             // sqlite | mysql | postgres
	DSN           string `json:"dsn"`              // Connection string (passwords masked on GET)
	MaxOpenConns  int    `json:"max_open_conns"`   // Pool: max open connections (MySQL/PostgreSQL)
	MaxIdleConns  int    `json:"max_idle_conns"`   // Pool: max idle connections (MySQL/PostgreSQL)
	CurrentType   string `json:"current_type"`     // Read-only: type currently in use
	EffectiveDSN  string `json:"effective_dsn"`    // Read-only: DSN currently in use (masked)
	RestartNeeded bool   `json:"restart_needed"`   // Read-only: true if config changed since startup
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
	dsnForEnv := s.resolveDSN(config.DSN)
	if config.Type == "mysql" && !strings.HasPrefix(dsnForEnv, "mysql://") {
		dsnForEnv = "mysql://" + dsnForEnv
	} else if config.Type == "postgres" && !strings.HasPrefix(dsnForEnv, "postgres://") && !strings.HasPrefix(dsnForEnv, "postgresql://") {
		dsnForEnv = "postgres://" + dsnForEnv
	}

	// Write CF_DB_DSN to .env
	if err := saveEnvFile("CF_DB_DSN", dsnForEnv); err != nil {
		return fmt.Errorf("save .env: %w", err)
	}

	// Write pool settings to .env
	if config.MaxOpenConns > 0 {
		saveEnvFile("CF_DB_MAX_OPEN_CONNS", strconv.Itoa(config.MaxOpenConns))
	}
	if config.MaxIdleConns > 0 {
		saveEnvFile("CF_DB_MAX_IDLE_CONNS", strconv.Itoa(config.MaxIdleConns))
	}

	return nil
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

	return os.WriteFile(".env", []byte(strings.Join(lines, "\n")+"\n"), 0644)
}

// GetByKey retrieves a setting value by key (implements proxy.SettingStore).
func (s *SettingService) GetByKey(ctx context.Context, key string) (string, error) {
	return s.repo.GetByKey(ctx, key)
}

// GetByPrefix retrieves all settings with a given key prefix.
func (s *SettingService) GetByPrefix(ctx context.Context, prefix string) (map[string]string, error) {
	return s.repo.GetByPrefix(ctx, prefix)
}

// Set saves a setting value by key (implements proxy.SettingStore).
func (s *SettingService) Set(ctx context.Context, key, value string) error {
	return s.repo.Set(ctx, key, value)
}

// SetByKey saves a setting value by key (implements portal.SettingStore).
func (s *SettingService) SetByKey(ctx context.Context, key, value string) error {
	return s.repo.Set(ctx, key, value)
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
