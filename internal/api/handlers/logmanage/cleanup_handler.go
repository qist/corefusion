package logmanage

import (
	"context"
	"strconv"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/model/entity"
	trafficSvc "corefusion/internal/service/traffic"
)

// SettingStore is the interface for reading/writing system settings.
type SettingStore interface {
	GetByPrefix(ctx context.Context, prefix string) (map[string]string, error)
	SetByKey(ctx context.Context, key, value string) error
}

// CleanupHandler handles database log cleanup rule management.
// It is registered alongside the main logmanage handler.
type CleanupHandler struct {
	cleanupSvc  CleanupService
	settingStore SettingStore
}

// CleanupService is the interface for the log cleanup service.
type CleanupService interface {
	ListRules() ([]entity.LogCleanupRule, error)
	GetRule(id int64) (*entity.LogCleanupRule, error)
	UpdateRule(rule *entity.LogCleanupRule) error
	CleanupNow(ctx context.Context) ([]trafficSvc.CleanupResult, int64, error)
	CleanupTableByName(ctx context.Context, tableName string) (int64, error)
}

// NewCleanupHandler creates a new cleanup handler.
func NewCleanupHandler(svc CleanupService) *CleanupHandler {
	return &CleanupHandler{cleanupSvc: svc}
}

// WithSettingStore injects a setting store for scheduler interval config.
func (h *CleanupHandler) WithSettingStore(s SettingStore) *CleanupHandler {
	h.settingStore = s
	return h
}

// ListCleanupRules returns all log cleanup rules.
func (h *CleanupHandler) ListCleanupRules(c *gin.Context) {
	rules, err := h.cleanupSvc.ListRules()
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	if rules == nil {
		rules = []entity.LogCleanupRule{}
	}
	response.OK(c, rules)
}

// UpdateCleanupRule updates a cleanup rule's retain days, enabled status, etc.
func (h *CleanupHandler) UpdateCleanupRule(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	existing, err := h.cleanupSvc.GetRule(id)
	if err != nil {
		response.NotFound(c, "NOT_FOUND", "cleanup rule not found")
		return
	}

	var req struct {
		RetainDays  *int    `json:"retain_days"`
		Enabled     *bool   `json:"enabled"`
		CleanupCron *string `json:"cleanup_cron"`
		DisplayName *string `json:"display_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	if req.RetainDays != nil {
		existing.RetainDays = *req.RetainDays
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if req.CleanupCron != nil {
		existing.CleanupCron = *req.CleanupCron
	}
	if req.DisplayName != nil {
		existing.DisplayName = *req.DisplayName
	}

	if err := h.cleanupSvc.UpdateRule(existing); err != nil {
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}
	response.OK(c, existing)
}

// CleanupAll manually triggers cleanup for all enabled rules.
func (h *CleanupHandler) CleanupAll(c *gin.Context) {
	results, total, err := h.cleanupSvc.CleanupNow(c.Request.Context())
	if err != nil {
		response.InternalError(c, "CLEANUP_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"results": results, "total_deleted": total})
}

// CleanupTable manually triggers cleanup for a specific table.
func (h *CleanupHandler) CleanupTable(c *gin.Context) {
	tableName := c.Param("table")
	if tableName == "" {
		response.BadRequest(c, "INVALID_TABLE", "table name required")
		return
	}
	deleted, err := h.cleanupSvc.CleanupTableByName(c.Request.Context(), tableName)
	if err != nil {
		response.InternalError(c, "CLEANUP_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"table": tableName, "deleted": deleted})
}

// ──────────── Scheduler Interval Config ────────────

// defaultSchedulerIntervals maps setting keys to default values (in minutes).
var defaultSchedulerIntervals = map[string]int{
	"scheduler.log_cleanup_interval":           360,  // 6h
	"scheduler.client_log_cleanup_interval":     10,  // 10m
	"scheduler.waf_log_cleanup_interval":        1440, // 24h
	"scheduler.waf_log_retain_days":             30,
	"scheduler.backup_check_interval":           1,
	"scheduler.callback_retry_interval":         1,
	"scheduler.processing_recovery_interval":    1,
	"scheduler.order_cleanup_interval":          5,
	"scheduler.trial_scan_interval":              60,  // 1h
	"scheduler.downgrade_scan_interval":         5,
	"scheduler.cfip_check_interval":              5,
}

// schedulerLabels maps setting keys to human-readable labels.
var schedulerLabels = map[string]string{
	"scheduler.log_cleanup_interval":           "DB Log Cleanup Check Interval",
	"scheduler.client_log_cleanup_interval":     "Client Log Package Cleanup Interval",
	"scheduler.waf_log_cleanup_interval":        "WAF Log File Cleanup Interval",
	"scheduler.waf_log_retain_days":             "WAF Log Retain Days",
	"scheduler.backup_check_interval":           "Backup Scheduler Check Interval",
	"scheduler.callback_retry_interval":         "Callback Retry Interval",
	"scheduler.processing_recovery_interval":    "Processing Recovery Interval",
	"scheduler.order_cleanup_interval":          "Expired Order Cleanup Interval",
	"scheduler.trial_scan_interval":              "Trial Expiry Scan Interval",
	"scheduler.downgrade_scan_interval":         "Downgrade Scan Interval",
	"scheduler.cfip_check_interval":              "CFIP Source Fetch Interval",
}

// GetSchedulerConfig handles GET /log-manage/scheduler-config.
// Returns all scheduler interval settings with their current values and defaults.
func (h *CleanupHandler) GetSchedulerConfig(c *gin.Context) {
	if h.settingStore == nil {
		response.OK(c, gin.H{"items": []interface{}{}})
		return
	}
	settings, err := h.settingStore.GetByPrefix(c.Request.Context(), "scheduler.")
	if err != nil {
		response.InternalError(c, "GET_FAILED", err.Error())
		return
	}

	type SchedulerItem struct {
		Key      string `json:"key"`
		Label    string `json:"label"`
		Value    int    `json:"value"`
		Default  int    `json:"default"`
	}

	items := make([]SchedulerItem, 0, len(defaultSchedulerIntervals))
	for key, def := range defaultSchedulerIntervals {
		val := def
		if v, ok := settings[key]; ok && v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				val = n
			}
		}
		items = append(items, SchedulerItem{
			Key:     key,
			Label:   schedulerLabels[key],
			Value:   val,
			Default: def,
		})
	}
	response.OK(c, gin.H{"items": items})
}

// UpdateSchedulerConfig handles PUT /log-manage/scheduler-config.
// Updates one or more scheduler interval settings. Changes take effect on next restart.
func (h *CleanupHandler) UpdateSchedulerConfig(c *gin.Context) {
	if h.settingStore == nil {
		response.BadRequest(c, "NOT_CONFIGURED", "setting store not available")
		return
	}
	var body struct {
		Items []struct {
			Key   string `json:"key" binding:"required"`
			Value int    `json:"value"`
		} `json:"items" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	for _, item := range body.Items {
		// Validate key is a known scheduler key
		if _, ok := defaultSchedulerIntervals[item.Key]; !ok {
			response.BadRequest(c, "INVALID_KEY", "unknown scheduler key: "+item.Key)
			return
		}
		if item.Value <= 0 {
			response.BadRequest(c, "INVALID_VALUE", "value must be positive for key: "+item.Key)
			return
		}
		if err := h.settingStore.SetByKey(c.Request.Context(), item.Key, strconv.Itoa(item.Value)); err != nil {
			response.InternalError(c, "SET_FAILED", err.Error())
			return
		}
	}
	response.OK(c, gin.H{"message": "scheduler config updated, restart required to apply new intervals"})
}
