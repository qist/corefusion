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
	val, err := h.db.WithContext(c.Request.Context()).Raw(
		"SELECT value FROM system_settings WHERE key = ?", realityTargetsKey,
	).Rows()
	if err != nil || !val.Next() {
		if val != nil {
			val.Close()
		}
		c.Data(200, "application/json", []byte(defaultRealityTargets))
		return
	}
	var v string
	val.Scan(&v)
	val.Close()
	if v == "" {
		c.Data(200, "application/json", []byte(defaultRealityTargets))
		return
	}
	c.Data(200, "application/json", []byte(v))
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
	h.db.WithContext(c.Request.Context()).Exec(
		"INSERT INTO system_settings (key, value, created_at, updated_at) VALUES (?, ?, datetime('now'), datetime('now')) ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at",
		realityTargetsKey, string(body),
	)
	response.OK(c, arr)
}

// ResetRealityTargets handles POST /api/v1/system/reality-targets/reset.
func (h *Handler) ResetRealityTargets(c *gin.Context) {
	h.db.WithContext(c.Request.Context()).Exec(
		"DELETE FROM system_settings WHERE key = ?", realityTargetsKey,
	)
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
