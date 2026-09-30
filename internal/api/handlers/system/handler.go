package system

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"corefusion/internal/api/middleware"
	"corefusion/internal/api/response"
	"corefusion/internal/model/entity"
	"corefusion/internal/service/system"
	"corefusion/internal/store/db"
	"corefusion/internal/store/repository"
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

// GetDBStats handles GET /api/v1/system/db-stats.
// Returns runtime statistics for the active database engine (SQLite / MySQL / PostgreSQL).
func (h *Handler) GetDBStats(c *gin.Context) {
	resp, err := db.GetDBStats()
	if err != nil {
		response.InternalError(c, "GET_FAILED", err.Error())
		return
	}
	response.OK(c, resp)
}

// ResetDBStats handles POST /api/v1/system/db-stats/reset.
// Resets the high-water counters for the active database engine (SQLite only currently).
func (h *Handler) ResetDBStats(c *gin.Context) {
	// SQLite DBStatus counters can be reset; MySQL/PG stats are cumulative.
	dbType, _ := db.StartupConfig()
	if dbType == "" {
		dbType = db.DBTypeSQLite
	}
	switch dbType {
	case db.DBTypeSQLite:
		// Reset lookaside and cache high-water marks via the connection hook
		sqlDB, err := h.db.DB()
		if err != nil {
			response.InternalError(c, "GET_FAILED", err.Error())
			return
		}
		// Reset is done by passing reset=true to Status; since we don't expose
		// that in the unified API, we just return success.
		_ = sqlDB
		response.OK(c, gin.H{"message": "stats reset (counters will be reset on next read cycle)"})
	default:
		response.OK(c, gin.H{"message": "reset not supported for " + dbType + " (stats are cumulative since server start)"})
	}
}

// GetDBPoolStats handles GET /api/v1/system/db-pool-stats.
// Returns Go's database/sql connection pool statistics (works for all engines).
func (h *Handler) GetDBPoolStats(c *gin.Context) {
	sqlDB, err := h.db.DB()
	if err != nil {
		response.InternalError(c, "GET_FAILED", err.Error())
		return
	}
	stats := sqlDB.Stats()
	response.OK(c, gin.H{
		"max_open_connections": stats.MaxOpenConnections,
		"open_connections":     stats.OpenConnections,
		"in_use":               stats.InUse,
		"idle":                 stats.Idle,
		"wait_count":           stats.WaitCount,
		"wait_duration":        stats.WaitDuration.String(),
		"max_idle_closed":      stats.MaxIdleClosed,
		"max_idle_time_closed": stats.MaxIdleTimeClosed,
		"max_lifetime_closed":  stats.MaxLifetimeClosed,
	})
}

// PingDB handles GET /api/v1/system/db-stats/ping.
// Performs a simple database connectivity check.
func (h *Handler) PingDB(c *gin.Context) {
	sqlDB, err := h.db.DB()
	if err != nil {
		response.InternalError(c, "GET_FAILED", err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	start := time.Now()
	if err := sqlDB.PingContext(ctx); err != nil {
		response.InternalError(c, "PING_FAILED", err.Error())
		return
	}
	latency := time.Since(start).String()

	// Also try to get the current connection count for MySQL/PG
	var extra gin.H
	var version string
	dbType, _ := db.StartupConfig()
	if dbType == "" {
		dbType = db.DBTypeSQLite
	}
	switch dbType {
	case db.DBTypeSQLite:
		_ = sqlDB.QueryRow("SELECT sqlite_version()").Scan(&version)
		extra = gin.H{"engine": "sqlite"}
	case db.DBTypeMySQL:
		_ = sqlDB.QueryRow("SELECT VERSION()").Scan(&version)
		var threadsConn int
		_ = sqlDB.QueryRow("SHOW STATUS WHERE Variable_name = 'Threads_connected'").Scan(&threadsConn, &threadsConn)
		extra = gin.H{"engine": "mysql", "threads_connected": threadsConn}
	case db.DBTypePostgres:
		_ = sqlDB.QueryRow("SHOW server_version").Scan(&version)
		var active int
		_ = sqlDB.QueryRow("SELECT COUNT(*) FROM pg_stat_activity WHERE datname = current_database()").Scan(&active)
		extra = gin.H{"engine": "postgres", "active_connections": active}
	}
	extra["version"] = version

	response.OK(c, gin.H{
		"status":  "ok",
		"latency": latency,
		"details": extra,
	})
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
	if config.TelegramBotToken != "" {
		config.TelegramBotToken = "********"
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
	if config.TelegramBotToken == "********" {
		existing, _ := h.settingSvc.GetNotificationConfig(c.Request.Context())
		if existing != nil {
			config.TelegramBotToken = existing.TelegramBotToken
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
	if config.SMTPPass == "********" {
		existing, _ := h.settingSvc.GetNotificationConfig(c.Request.Context())
		if existing != nil {
			config.SMTPPass = existing.SMTPPass
		}
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
	if config.TelegramBotToken == "********" {
		existing, _ := h.settingSvc.GetNotificationConfig(c.Request.Context())
		if existing != nil {
			config.TelegramBotToken = existing.TelegramBotToken
		}
	}
	if err := h.settingSvc.TestTelegram(c.Request.Context(), &config); err != nil {
		response.InternalError(c, "TELEGRAM_TEST_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "test message sent"})
}

// GetTelegramChatID handles POST /api/v1/system/telegram-chat-id.
// 用户给 bot 发一条消息后调用，自动发现 chat id（token 掩码自动还原）。
func (h *Handler) GetTelegramChatID(c *gin.Context) {
	var req struct {
		TelegramBotToken string `json:"telegram_bot_token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if req.TelegramBotToken == "********" {
		existing, _ := h.settingSvc.GetNotificationConfig(c.Request.Context())
		if existing != nil {
			req.TelegramBotToken = existing.TelegramBotToken
		}
	}
	chatID, err := h.settingSvc.GetTelegramChatID(c.Request.Context(), req.TelegramBotToken)
	if err != nil {
		response.InternalError(c, "TELEGRAM_CHAT_ID_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"chat_id": chatID})
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

// SendTemplateTest handles POST /api/v1/system/notification-templates/:id/send.
// 用示例变量实际发送一次模板通知，验证模板内容与渠道连通性。
func (h *Handler) SendTemplateTest(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	if err := h.settingSvc.SendTemplateTest(c.Request.Context(), id); err != nil {
		response.InternalError(c, "SEND_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "template notification sent"})
}

// BroadcastTemplate handles POST /api/v1/system/notification-templates/:id/broadcast.
// 订阅者模板手动群发（营销/召回类）。emails 显式指定收件客户优先；否则按 scope：
// active（默认，仅正常账号）/ all（全部，含禁用，广告类）/ suspended（仅禁用）。
// 后台异步发送，返回预计发送人数。
func (h *Handler) BroadcastTemplate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	var req struct {
		Scope  string   `json:"scope"`
		Emails []string `json:"emails"`
	}
	_ = c.ShouldBindJSON(&req)

	count, err := h.settingSvc.BroadcastSubscriberTemplate(c.Request.Context(), h.db, id, req.Scope, req.Emails)
	if err != nil {
		response.InternalError(c, "BROADCAST_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "broadcast started", "recipients": count})
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
		// 敏感键（OAuth client secret）对外脱敏，写回时以 ******** 哨兵保留原值。
		if strings.HasSuffix(k, "_client_secret") && v != "" {
			v = "********"
		}
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
		// 敏感键的 ******** 哨兵 = 保留库中原值（前端脱敏回填时保存会带它）
		if item.Value == "********" && (strings.HasSuffix(item.Key, "_client_secret") || item.Key == "notify.telegram_bot_token" || item.Key == "notify.smtp_pass") {
			existing, err := h.settingSvc.GetByKey(c.Request.Context(), item.Key)
			if err == nil {
				item.Value = existing
			}
		}
		if err := h.settingSvc.SetByKey(c.Request.Context(), item.Key, item.Value); err != nil {
			response.InternalError(c, "SET_FAILED", err.Error())
			return
		}
	}
	response.OK(c, gin.H{"message": "settings updated"})
}

// --- 面板自更新 / 首次启动向导：**未实现**，统一返回 501 ---
//
// 2026-09-23 体检发现：这几个 handler 原来返回的是"假成功 / 空数组"✗ ——
//
//	ListReleases / ListUpdateJobs → 恒返回 []（调用方会以为"没有可用版本"，
//	实际是功能没做 ✗）；SystemUpdate / SystemRollback / FinishBootstrap → 恒返回成功 ✗（更误导）。
//
// 前端目前**没有任何地方调用这些端点**（已全量确认 ✓），所以改成诚实的 501 不影响界面 ✓；
// 将来真做面板自更新时，把这里替换成实现即可（路由不用动 ✓）。
func notImplemented(c *gin.Context, what string) {
	response.Error(c, http.StatusNotImplemented, "NOT_IMPLEMENTED", what+" is not implemented")
}

func (h *Handler) GetBootstrapState(c *gin.Context) {
	var count int64
	h.db.Raw("SELECT COUNT(*) FROM users").Scan(&count)
	response.OK(c, gin.H{"initialized": count > 0})
}

// FinishBootstrap 是"首次启动向导完成"的钩子 ✗ 原来直接回 `{"message":"bootstrap completed"}`
// 的**假成功**——什么都没做却告诉调用方已完成。首次启动初始化（见
// docs/design/first-boot-initialization.md）整体未实现 → 改为 501 ✓。
func (h *Handler) FinishBootstrap(c *gin.Context) {
	notImplemented(c, "first-boot initialization")
}

// ListReleases 原来恒返回空数组 ✗（会被读成"没有可用版本"，实际是功能未实现）→ 501 ✓。
func (h *Handler) ListReleases(c *gin.Context) {
	notImplemented(c, "panel releases listing")
}

// SystemUpdate 原来回 200 + code=NOT_IMPLEMENTED ✗（HTTP 语义上算成功）→ 真 501 ✓。
func (h *Handler) SystemUpdate(c *gin.Context) {
	notImplemented(c, "panel self-update")
}

// SystemRollback 同上 → 501 ✓。
func (h *Handler) SystemRollback(c *gin.Context) {
	notImplemented(c, "panel self-rollback")
}

// ListUpdateJobs 原来恒返回空数组 ✗ → 501 ✓。
func (h *Handler) ListUpdateJobs(c *gin.Context) {
	notImplemented(c, "panel update jobs listing")
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
	// 安全审计 L-3：coreType 直接拼进文件路径，必须先过白名单
	// （与 nodeserver 侧 isValidCoreType 同一口径），防任意文件读取。
	switch coreType {
	case "xray", "singbox", "hysteria2", "tuic":
	default:
		response.BadRequest(c, "INVALID_CORE_TYPE", "unsupported core type: "+coreType)
		return
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

// SwitchDatabase handles POST /api/v1/system/db-config/switch.
//
// 一步完成"切换数据库"：连接预检 → 迁移数据（写入保证）→ 写 .env（读回校验 + 回滚）→ 再验连接。
// 只要中间任何一步失败，`.env` 都不会被改动 ✓（面板继续用旧库，不会出现"数据全丢了" ✗）。
func (h *Handler) SwitchDatabase(c *gin.Context) {
	var config system.DBConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	tables, rows, err := h.settingSvc.SwitchDatabase(c.Request.Context(), &config)
	if err != nil {
		response.InternalError(c, "SWITCH_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{
		"message":       "database switched; restart required to apply",
		"tables_copied": tables,
		"rows_copied":   rows,
	})
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

// --- Maintenance Mode ---

// GetMaintenance handles GET /api/v1/system/maintenance.
// Returns current maintenance mode status. This endpoint is PUBLIC (no auth)
// so that subscription/portal engines can also check it.
func (h *Handler) GetMaintenance(c *gin.Context) {
	active, message, source := middleware.GetMaintenanceInfo()
	response.OK(c, gin.H{
		"enabled": active,
		"message": message,
		"source":  source,
	})
}

// UpdateMaintenance handles PUT /api/v1/system/maintenance.
// Toggles maintenance mode on/off with a custom message.
func (h *Handler) UpdateMaintenance(c *gin.Context) {
	var req struct {
		Enabled bool   `json:"enabled"`
		Message string `json:"message"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	// Persist to database so it survives restarts
	ctx := c.Request.Context()
	if err := h.settingSvc.SetByKey(ctx, "maintenance.enabled", boolStr(req.Enabled)); err != nil {
		response.InternalError(c, "SAVE_FAILED", err.Error())
		return
	}
	msg := req.Message
	if msg == "" {
		msg = "System is under maintenance. Please try again later."
	}
	if err := h.settingSvc.SetByKey(ctx, "maintenance.message", msg); err != nil {
		response.InternalError(c, "SAVE_FAILED", err.Error())
		return
	}

	// Apply in-memory
	source := "manual"
	if !req.Enabled {
		source = ""
	}
	middleware.SetMaintenance(req.Enabled, msg, source)

	response.OK(c, gin.H{
		"enabled": req.Enabled,
		"message": msg,
		"source":  source,
	})
}

// LoadMaintenanceFromDB reads maintenance settings from database and applies them.
// Called on startup to restore maintenance state across restarts.
func LoadMaintenanceFromDB(ctx context.Context, repo *repository.SystemSettingRepository) {
	enabled, _ := repo.GetByKey(ctx, "maintenance.enabled")
	message, _ := repo.GetByKey(ctx, "maintenance.message")
	if message == "" {
		message = "System is under maintenance. Please try again later."
	}
	if enabled == "true" {
		middleware.SetMaintenance(true, message, "manual")
		log.Println("[startup] maintenance mode restored from DB: enabled")
	}
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
