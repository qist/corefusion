package system

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"corefusion/internal/api/response"
	"corefusion/internal/model/entity"
	"corefusion/internal/service/system"
)

// Handler handles system HTTP requests.
type Handler struct {
	db         *gorm.DB
	settingSvc *system.SettingService
	statsSvc   *system.StatsService
	dataDir    string
}

// NewHandler creates a new system handler.
func NewHandler(db *gorm.DB, settingSvc *system.SettingService, dataDir string, version string) *Handler {
	return &Handler{
		db:         db,
		settingSvc: settingSvc,
		statsSvc:   system.NewStatsService(version),
		dataDir:    dataDir,
	}
}

// GetSystemStats handles GET /api/v1/system/stats.
func (h *Handler) GetSystemStats(c *gin.Context) {
	stats := h.statsSvc.GetStats()
	response.OK(c, stats)
}

// --- Panel Config ---

// GetPanelConfig handles GET /api/v1/system/panel-config.
func (h *Handler) GetPanelConfig(c *gin.Context) {
	config, err := h.settingSvc.GetPanelConfig(c.Request.Context())
	if err != nil {
		response.InternalError(c, "GET_FAILED", err.Error())
		return
	}
	response.OK(c, config)
}

// UpdatePanelConfig handles PUT /api/v1/system/panel-config.
func (h *Handler) UpdatePanelConfig(c *gin.Context) {
	var config system.PanelConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if err := h.settingSvc.UpdatePanelConfig(c.Request.Context(), &config); err != nil {
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "panel config updated, restart required for listen_addr/port/tls changes"})
}

// --- Notification Config ---

// GetNotificationConfig handles GET /api/v1/system/notification-config.
func (h *Handler) GetNotificationConfig(c *gin.Context) {
	config, err := h.settingSvc.GetNotificationConfig(c.Request.Context())
	if err != nil {
		response.InternalError(c, "GET_FAILED", err.Error())
		return
	}
	if config.SMTPPass != "" {
		config.SMTPPass = "********"
	}
	response.OK(c, config)
}

// UpdateNotificationConfig handles PUT /api/v1/system/notification-config.
func (h *Handler) UpdateNotificationConfig(c *gin.Context) {
	var config system.NotificationConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if config.SMTPPass == "********" {
		existing, _ := h.settingSvc.GetNotificationConfig(c.Request.Context())
		if existing != nil {
			config.SMTPPass = existing.SMTPPass
		}
	}
	if err := h.settingSvc.UpdateNotificationConfig(c.Request.Context(), &config); err != nil {
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "notification config updated"})
}

// TestSMTP handles POST /api/v1/system/test-smtp.
func (h *Handler) TestSMTP(c *gin.Context) {
	var config system.NotificationConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if err := h.settingSvc.TestSMTP(c.Request.Context(), &config); err != nil {
		response.InternalError(c, "SMTP_TEST_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "test email sent"})
}

// TestTelegram handles POST /api/v1/system/test-telegram.
func (h *Handler) TestTelegram(c *gin.Context) {
	var config system.NotificationConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if err := h.settingSvc.TestTelegram(c.Request.Context(), &config); err != nil {
		response.InternalError(c, "TELEGRAM_TEST_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "test message sent"})
}

// --- GitHub Config ---

// GetGitHubConfig handles GET /api/v1/system/github-config.
func (h *Handler) GetGitHubConfig(c *gin.Context) {
	config, err := h.settingSvc.GetGitHubConfig(c.Request.Context())
	if err != nil {
		response.InternalError(c, "GET_FAILED", err.Error())
		return
	}
	response.OK(c, config)
}

// UpdateGitHubConfig handles PUT /api/v1/system/github-config.
func (h *Handler) UpdateGitHubConfig(c *gin.Context) {
	var config system.GitHubConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if err := h.settingSvc.UpdateGitHubConfig(c.Request.Context(), &config); err != nil {
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "github config updated"})
}

func (h *Handler) GetSubscriptionBillingTemplates(c *gin.Context) {
	config, err := h.settingSvc.GetSubscriptionBillingTemplateConfig(c.Request.Context())
	if err != nil {
		response.InternalError(c, "GET_FAILED", err.Error())
		return
	}
	response.OK(c, config)
}

func (h *Handler) UpdateSubscriptionBillingTemplates(c *gin.Context) {
	var config system.SubscriptionBillingTemplateConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if err := h.settingSvc.UpdateSubscriptionBillingTemplateConfig(c.Request.Context(), &config); err != nil {
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "subscription billing templates updated"})
}

// --- Notification Templates ---

// ListTemplates handles GET /api/v1/system/notification-templates.
func (h *Handler) ListTemplates(c *gin.Context) {
	templates, err := h.settingSvc.ListTemplates(c.Request.Context())
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	response.OK(c, templates)
}

// CreateTemplate handles POST /api/v1/system/notification-templates.
func (h *Handler) CreateTemplate(c *gin.Context) {
	var tmpl entity.NotificationTemplate
	if err := c.ShouldBindJSON(&tmpl); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if err := h.settingSvc.CreateTemplate(c.Request.Context(), &tmpl); err != nil {
		response.InternalError(c, "CREATE_FAILED", err.Error())
		return
	}
	response.Created(c, tmpl)
}

// UpdateTemplate handles PUT /api/v1/system/notification-templates/:id.
func (h *Handler) UpdateTemplate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	var tmpl entity.NotificationTemplate
	if err := c.ShouldBindJSON(&tmpl); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	tmpl.ID = id
	if err := h.settingSvc.UpdateTemplate(c.Request.Context(), &tmpl); err != nil {
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}
	response.OK(c, tmpl)
}

// DeleteTemplate handles DELETE /api/v1/system/notification-templates/:id.
func (h *Handler) DeleteTemplate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	if err := h.settingSvc.DeleteTemplate(c.Request.Context(), id); err != nil {
		response.InternalError(c, "DELETE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "deleted"})
}

// --- mTLS ---

// UploadMTLSCA handles POST /api/v1/system/upload-mtls-ca.
func (h *Handler) UploadMTLSCA(c *gin.Context) {
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		response.BadRequest(c, "INVALID_REQUEST", "file is required")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		response.InternalError(c, "READ_FAILED", err.Error())
		return
	}

	path, err := h.settingSvc.SaveMTLSCAFile(c.Request.Context(), h.dataDir, data)
	if err != nil {
		response.InternalError(c, "SAVE_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"path": path})
}

// MigrateData handles POST /api/v1/system/migrate-data.
func (h *Handler) MigrateData(c *gin.Context) {
	var req struct {
		NewDir string `json:"new_dir" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	// Save new data dir setting
	if err := h.settingSvc.UpdateDataDir(c.Request.Context(), req.NewDir); err != nil {
		response.InternalError(c, "SAVE_FAILED", err.Error())
		return
	}

	// Copy data to new directory
	if err := h.settingSvc.MigrateDataToDir(h.dataDir, req.NewDir); err != nil {
		response.InternalError(c, "MIGRATE_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{
		"message": "data migrated, restart required",
		"new_dir": req.NewDir,
	})
}

// Restart handles POST /api/v1/system/restart.
func (h *Handler) Restart(c *gin.Context) {
	response.OK(c, gin.H{"message": "restarting..."})

	go func() {
		time.Sleep(500 * time.Millisecond)
		restartProcess()
	}()
}

// --- Generic Settings ---

// GetSettings handles GET /api/v1/system/settings?prefix=xxx
// Returns all system settings matching the given key prefix.
func (h *Handler) GetSettings(c *gin.Context) {
	prefix := c.Query("prefix")
	if prefix == "" {
		response.BadRequest(c, "MISSING_PREFIX", "prefix query parameter is required")
		return
	}
	settings, err := h.settingSvc.GetByPrefix(c.Request.Context(), prefix)
	if err != nil {
		response.InternalError(c, "GET_FAILED", err.Error())
		return
	}
	items := make([]gin.H, 0, len(settings))
	for k, v := range settings {
		items = append(items, gin.H{"key": k, "value": v})
	}
	response.OK(c, gin.H{"items": items})
}

// BatchUpdateSettings handles PUT /api/v1/system/settings/batch
// Upserts multiple system settings at once.
func (h *Handler) BatchUpdateSettings(c *gin.Context) {
	var body struct {
		Items []struct {
			Key   string `json:"key" binding:"required"`
			Value string `json:"value"`
		} `json:"items" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	for _, item := range body.Items {
		if err := h.settingSvc.SetByKey(c.Request.Context(), item.Key, item.Value); err != nil {
			response.InternalError(c, "SET_FAILED", err.Error())
			return
		}
	}
	response.OK(c, gin.H{"message": "settings updated"})
}

// --- Existing stubs ---

func (h *Handler) GetBootstrapState(c *gin.Context) {
	var count int64
	h.db.Raw("SELECT COUNT(*) FROM users").Scan(&count)
	response.OK(c, gin.H{"initialized": count > 0})
}

func (h *Handler) FinishBootstrap(c *gin.Context) {
	response.OK(c, gin.H{"message": "bootstrap completed"})
}

func (h *Handler) ListReleases(c *gin.Context) {
	response.OK(c, []interface{}{})
}

func (h *Handler) SystemUpdate(c *gin.Context) {
	response.OK(c, gin.H{"code": "NOT_IMPLEMENTED", "message": "system update not yet implemented"})
}

func (h *Handler) SystemRollback(c *gin.Context) {
	response.OK(c, gin.H{"code": "NOT_IMPLEMENTED", "message": "system rollback not yet implemented"})
}

func (h *Handler) ListUpdateJobs(c *gin.Context) {
	response.OK(c, []interface{}{})
}

const realityTargetsKey = "reality.targets"

var defaultRealityTargets = `[
  {"target":"www.amazon.com:443","sni":"www.amazon.com"},
  {"target":"aws.amazon.com:443","sni":"aws.amazon.com"},
  {"target":"www.oracle.com:443","sni":"www.oracle.com"},
  {"target":"www.nvidia.com:443","sni":"www.nvidia.com"},
  {"target":"www.amd.com:443","sni":"www.amd.com"},
  {"target":"www.intel.com:443","sni":"www.intel.com"},
  {"target":"www.sony.com:443","sni":"www.sony.com"},
  {"target":"www.microsoft.com:443","sni":"www.microsoft.com"},
  {"target":"www.google.com:443","sni":"www.google.com"},
  {"target":"dl.google.com:443","sni":"dl.google.com"},
  {"target":"www.apple.com:443","sni":"www.apple.com"},
  {"target":"www.cloudflare.com:443","sni":"www.cloudflare.com"}
]`

// GetRealityTargets handles GET /api/v1/system/reality-targets.
func (h *Handler) GetRealityTargets(c *gin.Context) {
	var s entity.SystemSetting
	err := h.db.WithContext(c.Request.Context()).Where("`key` = ?", realityTargetsKey).First(&s).Error
	if err != nil || s.Value == "" {
		c.Data(200, "application/json", []byte(defaultRealityTargets))
		return
	}
	c.Data(200, "application/json", []byte(s.Value))
}

// UpdateRealityTargets handles PUT /api/v1/system/reality-targets.
func (h *Handler) UpdateRealityTargets(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		response.BadRequest(c, "INVALID_BODY", "failed to read body")
		return
	}
	// Validate JSON
	var arr []interface{}
	if err := json.Unmarshal(body, &arr); err != nil {
		response.BadRequest(c, "INVALID_JSON", "must be a JSON array")
		return
	}
	h.settingSvc.Set(c.Request.Context(), realityTargetsKey, string(body))
	response.OK(c, arr)
}

// ResetRealityTargets handles POST /api/v1/system/reality-targets/reset.
func (h *Handler) ResetRealityTargets(c *gin.Context) {
	h.settingSvc.Delete(c.Request.Context(), realityTargetsKey)
	c.Data(200, "application/json", []byte(defaultRealityTargets))
}

// GetConfigFile handles GET /api/v1/system/config-file/:coreType.
// Reads the actual config file from disk (runtime/xray.jsonc etc.)
func (h *Handler) GetConfigFile(c *gin.Context) {
	coreType := c.Param("coreType")
	if coreType == "" {
		coreType = "xray"
	}
	configPath := filepath.Join(h.dataDir, "runtime", coreType+".jsonc")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			response.NotFound(c, "CONFIG_NOT_FOUND", "config file not found: "+configPath)
			return
		}
		response.InternalError(c, "READ_FAILED", err.Error())
		return
	}
	c.Data(200, "application/json", data)
}

// --- Database Config ---

// GetDBConfig handles GET /api/v1/system/db-config.
func (h *Handler) GetDBConfig(c *gin.Context) {
	config, err := h.settingSvc.GetDBConfig(c.Request.Context())
	if err != nil {
		response.InternalError(c, "GET_FAILED", err.Error())
		return
	}
	response.OK(c, config)
}

// UpdateDBConfig handles PUT /api/v1/system/db-config.
func (h *Handler) UpdateDBConfig(c *gin.Context) {
	var config system.DBConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if err := h.settingSvc.UpdateDBConfig(c.Request.Context(), &config); err != nil {
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "database config updated, restart required to apply changes"})
}

// TestDBConnection handles POST /api/v1/system/db-config/test.
func (h *Handler) TestDBConnection(c *gin.Context) {
	var config system.DBConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if err := h.settingSvc.TestDBConnection(c.Request.Context(), &config); err != nil {
		response.InternalError(c, "CONNECTION_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "database connection successful"})
}

// MigrateDBData handles POST /api/v1/system/db-config/migrate.
func (h *Handler) MigrateDBData(c *gin.Context) {
	var config system.DBConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	tables, rows, err := h.settingSvc.MigrateDBData(c.Request.Context(), &config)
	if err != nil {
		response.InternalError(c, "MIGRATE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{
		"message":       "data migration completed, restart required to apply changes",
		"tables_copied": tables,
		"rows_copied":   rows,
	})
}

// --- Encryption Key Config ---

// GetEncryptionKeyStatus handles GET /api/v1/system/encryption-key-status.
func (h *Handler) GetEncryptionKeyStatus(c *gin.Context) {
	configured := h.settingSvc.IsEncryptionKeyConfigured()
	response.OK(c, gin.H{"configured": configured})
}

// GenerateEncryptionKey handles POST /api/v1/system/generate-encryption-key.
func (h *Handler) GenerateEncryptionKey(c *gin.Context) {
	if h.settingSvc.IsEncryptionKeyConfigured() {
		response.BadRequest(c, "ALREADY_CONFIGURED", "encryption key is already configured")
		return
	}
	key, err := h.settingSvc.GenerateAndSaveEncryptionKey()
	if err != nil {
		response.InternalError(c, "GENERATE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"success": true, "message": "encryption key generated and saved to .env", "key_preview": key[:8] + "****"})
}

// --- JWT Secret Config ---

// GetJWTSecretStatus handles GET /api/v1/system/jwt-secret-status.
func (h *Handler) GetJWTSecretStatus(c *gin.Context) {
	configured, preview := h.settingSvc.JWTSecretStatus()
	response.OK(c, gin.H{"configured": configured, "preview": preview})
}

// RegenerateJWTSecret handles POST /api/v1/system/regenerate-jwt-secret.
func (h *Handler) RegenerateJWTSecret(c *gin.Context) {
	secret, err := h.settingSvc.RegenerateJWTSecret()
	if err != nil {
		response.InternalError(c, "GENERATE_FAILED", err.Error())
		return
	}
	preview := secret[:8] + "****"
	response.OK(c, gin.H{"success": true, "message": "JWT secret regenerated and saved to .env. All users will need to re-login.", "preview": preview})
}

// --- Env Config (SAFE_PATHS) ---

// GetEnvConfig handles GET /api/v1/system/env-config.
// Returns .env file values for SAFE_PATHS and other env-only settings.
func (h *Handler) GetEnvConfig(c *gin.Context) {
	safePaths := h.settingSvc.GetSafePaths()
	response.OK(c, gin.H{
		"safe_paths": safePaths,
	})
}

// UpdateEnvConfig handles PUT /api/v1/system/env-config.
// Updates SAFE_PATHS in .env file. Restart required.
func (h *Handler) UpdateEnvConfig(c *gin.Context) {
	var req struct {
		SafePaths string `json:"safe_paths"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if req.SafePaths == "" {
		response.BadRequest(c, "INVALID_REQUEST", "safe_paths is required")
		return
	}
	if err := h.settingSvc.UpdateSafePaths(req.SafePaths); err != nil {
		response.InternalError(c, "SAVE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "env config updated, restart required"})
}
