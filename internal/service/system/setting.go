package system

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"corefusion/internal/model/entity"
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
	RegistrationEnabled bool   `json:"registration_enabled"`
	TLSEnabled          bool   `json:"tls_enabled"`
	TLSCertID           int64  `json:"tls_cert_id"`
	TLSCertPath         string `json:"tls_cert_path"`
	TLSKeyPath          string `json:"tls_key_path"`
	MTLSEnabled         bool   `json:"mtls_enabled"`
	MTLSCAPath          string `json:"mtls_ca_path"`
	WebPath             string `json:"web_path"`
	Timezone            string `json:"timezone"`
	DataDir             string `json:"data_dir"`
}

// GitHubDomain represents a single GitHub acceleration domain.
type GitHubDomain struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	Enabled bool   `json:"enabled"`
}

// GitHubConfig contains GitHub acceleration configuration.
type GitHubConfig struct {
	Enabled bool           `json:"enabled"`
	Domains []GitHubDomain `json:"domains"`
}

// DefaultGitHubConfig returns default GitHub acceleration configuration.
func DefaultGitHubConfig() *GitHubConfig {
	return &GitHubConfig{
		Enabled: false,
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

// DefaultPanelConfig returns default panel configuration.
func DefaultPanelConfig() *PanelConfig {
	return &PanelConfig{
		ListenAddr:       "0.0.0.0",
		ListenPort:       "8080",
		SubscriptionPort: "0",
		TLSEnabled:       false,
		WebPath:          "/",
		Timezone:         "Asia/Shanghai",
		DataDir:          "data",
	}
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
	setIfExist(settings, "panel.web_path", &config.WebPath)
	setIfExist(settings, "panel.timezone", &config.Timezone)
	setIfExist(settings, "panel.data_dir", &config.DataDir)

	return config, nil
}

// UpdatePanelConfig saves panel configuration to database.
func (s *SettingService) UpdatePanelConfig(ctx context.Context, config *PanelConfig) error {
	settings := map[string]string{
		"panel.listen_addr":          config.ListenAddr,
		"panel.listen_port":          config.ListenPort,
		"panel.subscription_port":    config.SubscriptionPort,
		"panel.registration_enabled": fmt.Sprintf("%v", config.RegistrationEnabled),
		"panel.tls_enabled":          fmt.Sprintf("%v", config.TLSEnabled),
		"panel.tls_cert_id":   strconv.FormatInt(config.TLSCertID, 10),
		"panel.tls_cert_path": config.TLSCertPath,
		"panel.tls_key_path":  config.TLSKeyPath,
		"panel.mtls_enabled":  fmt.Sprintf("%v", config.MTLSEnabled),
		"panel.mtls_ca_path":  config.MTLSCAPath,
		"panel.web_path":      config.WebPath,
		"panel.timezone":      config.Timezone,
		"panel.data_dir":      config.DataDir,
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
		"notify.telegram_enabled":  fmt.Sprintf("%v", config.TelegramEnabled),
		"notify.telegram_bot_token": config.TelegramBotToken,
		"notify.telegram_chat_id":  config.TelegramChatID,
		"notify.smtp_enabled":      fmt.Sprintf("%v", config.SMTPEnabled),
		"notify.smtp_host":         config.SMTPHost,
		"notify.smtp_port":         config.SMTPPort,
		"notify.smtp_user":         config.SMTPUser,
		"notify.smtp_pass":         config.SMTPPass,
		"notify.smtp_from":         config.SMTPFrom,
		"notify.smtp_tls":          fmt.Sprintf("%v", config.SMTPTLS),
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

// Set saves a setting value by key (implements proxy.SettingStore).
func (s *SettingService) Set(ctx context.Context, key, value string) error {
	return s.repo.Set(ctx, key, value)
}

func setIfExist(m map[string]string, key string, target *string) {
	if v, ok := m[key]; ok && v != "" {
		*target = v
	}
}
