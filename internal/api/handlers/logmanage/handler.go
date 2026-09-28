package logmanage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/model/entity"
	agentpb "corefusion/internal/runtime/agent/proto"
	"corefusion/internal/service/logmanage"
)

// Handler handles log management HTTP requests.
type Handler struct {
	svc *logmanage.Service
}

// NewHandler creates a new log management handler.
func NewHandler(svc *logmanage.Service) *Handler {
	return &Handler{svc: svc}
}

// getNginxSettingsForSync returns the raw nginx settings JSON for syncing to remote agents.
func (h *Handler) getNginxSettingsForSync(ctx context.Context) string {
	return h.svc.GetNginxSettingsRaw(ctx)
}

// --- Policy CRUD ---

// ListPolicies returns all rotation policies.
func (h *Handler) ListPolicies(c *gin.Context) {
	serverID, _ := strconv.ParseInt(c.DefaultQuery("server_id", "1"), 10, 64)
	if serverID == 0 {
		serverID = 1
	}

	if serverID > 1 {
		// Remote node: query agent's policies via gRPC
		if nodeLogMgr == nil {
			response.InternalError(c, "NOT_CONFIGURED", "node log manager not configured")
			return
		}
		policies, err := h.svc.ListPoliciesForServer(c.Request.Context(), serverID)
		if err != nil {
			response.InternalError(c, "LIST_FAILED", err.Error())
			return
		}
		if policies == nil {
			policies = []entity.LogRotationPolicy{}
		}
		response.OK(c, policies)
		return
	}

	policies, err := h.svc.ListPolicies(c.Request.Context())
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	if policies == nil {
		policies = []entity.LogRotationPolicy{}
	}
	response.OK(c, policies)
}

// GetPolicy returns a policy by ID.
func (h *Handler) GetPolicy(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	policy, err := h.svc.GetPolicy(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "NOT_FOUND", "policy not found")
		return
	}
	response.OK(c, policy)
}

// CreatePolicy creates a new rotation policy.
func (h *Handler) CreatePolicy(c *gin.Context) {
	var req struct {
		Name       string `json:"name" binding:"required"`
		SourceType string `json:"source_type" binding:"required"`
		LogPath    string `json:"log_path" binding:"required"`
		ServerID   int64  `json:"server_id"`
		Enabled    *bool  `json:"enabled"`
		RotateCron string `json:"rotate_cron"`
		RetainDays int    `json:"retain_days"`
		MaxSize    int64  `json:"max_size"`
		Compress   *bool  `json:"compress"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	compress := false
	if req.Compress != nil {
		compress = *req.Compress
	}

	policy := &entity.LogRotationPolicy{
		Name:       req.Name,
		SourceType: req.SourceType,
		LogPath:    req.LogPath,
		ServerID:   req.ServerID,
		Enabled:    enabled,
		RotateCron: req.RotateCron,
		RetainDays: req.RetainDays,
		MaxSize:    req.MaxSize,
		Compress:   compress,
	}
	if err := h.svc.CreatePolicy(c.Request.Context(), policy); err != nil {
		response.InternalError(c, "CREATE_FAILED", err.Error())
		return
	}
	response.Created(c, policy)
}

// UpdatePolicy updates an existing rotation policy.
func (h *Handler) UpdatePolicy(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	existing, err := h.svc.GetPolicy(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "NOT_FOUND", "policy not found")
		return
	}

	var req struct {
		Name       *string `json:"name"`
		SourceType *string `json:"source_type"`
		LogPath    *string `json:"log_path"`
		Enabled    *bool   `json:"enabled"`
		RotateCron *string `json:"rotate_cron"`
		RetainDays *int    `json:"retain_days"`
		MaxSize    *int64  `json:"max_size"`
		Compress   *bool   `json:"compress"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.SourceType != nil {
		existing.SourceType = *req.SourceType
	}
	if req.LogPath != nil {
		existing.LogPath = *req.LogPath
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if req.RotateCron != nil {
		existing.RotateCron = *req.RotateCron
	}
	if req.RetainDays != nil {
		existing.RetainDays = *req.RetainDays
	}
	if req.MaxSize != nil {
		existing.MaxSize = *req.MaxSize
	}
	if req.Compress != nil {
		existing.Compress = *req.Compress
	}

	if err := h.svc.UpdatePolicy(c.Request.Context(), existing); err != nil {
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}
	response.OK(c, existing)
}

// DeletePolicy deletes a rotation policy.
func (h *Handler) DeletePolicy(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	if err := h.svc.DeletePolicy(c.Request.Context(), id); err != nil {
		response.InternalError(c, "DELETE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "deleted"})
}

// --- Manual Rotation ---

// Rotate triggers manual rotation for a specific policy.
// For remote node policies (ServerID > 1), rotates via gRPC on the agent.
func (h *Handler) Rotate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	policy, err := h.svc.GetPolicy(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "NOT_FOUND", "policy not found")
		return
	}

	// Remote node: rotate via gRPC
	if policy.ServerID > 1 {
		if nodeLogMgr == nil {
			response.InternalError(c, "NOT_CONFIGURED", "node log manager not configured")
			return
		}
		rotated, cleaned, freed, err := nodeLogMgr.RotateLogOnServer(c.Request.Context(), policy.ServerID, policy.SourceType)
		if err != nil {
			response.InternalError(c, "ROTATE_FAILED", err.Error())
			return
		}
		response.OK(c, gin.H{
			"rotated_files": rotated,
			"cleaned_files": cleaned,
			"freed_bytes":   freed,
		})
		return
	}

	// Local node: rotate locally
	job, err := h.svc.Rotate(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "ROTATE_FAILED", err.Error())
		return
	}
	response.OK(c, job)
}

// --- Job History ---

// ListPolicyJobs returns job history for a specific policy.
func (h *Handler) ListPolicyJobs(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	jobs, total, err := h.svc.ListJobsByPolicy(c.Request.Context(), id, limit, offset)
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	if jobs == nil {
		jobs = []entity.LogRotationJob{}
	}
	response.Paginated(c, jobs, total, limit, offset)
}

// --- Discovery ---

// DiscoverLogs discovers all configured log files.
func (h *Handler) DiscoverLogs(c *gin.Context) {
	serverID, _ := strconv.ParseInt(c.DefaultQuery("server_id", "1"), 10, 64)
	if serverID == 0 {
		serverID = 1
	}

	if serverID > 1 {
		// Remote node: discover via gRPC
		if nodeLogMgr == nil {
			response.InternalError(c, "NOT_CONFIGURED", "node log manager not configured")
			return
		}
		logs, err := nodeLogMgr.DiscoverLogsOnServer(c.Request.Context(), serverID)
		if err != nil {
			response.InternalError(c, "DISCOVER_FAILED", err.Error())
			return
		}
		// Convert proto logs to DiscoveredLog format
		var result []logmanage.DiscoveredLog
		for _, l := range logs {
			result = append(result, logmanage.DiscoveredLog{
				SourceType: l.SourceType,
				Domain:     l.Domain,
				LogPath:    l.LogPath,
				LogFormat:  l.LogFormat,
				HasPolicy:  l.HasPolicy,
			})
		}
		if result == nil {
			result = []logmanage.DiscoveredLog{}
		}
		response.OK(c, result)
		return
	}

	logs, err := h.svc.DiscoverLogs(c.Request.Context())
	if err != nil {
		response.InternalError(c, "DISCOVER_FAILED", err.Error())
		return
	}
	if logs == nil {
		logs = []logmanage.DiscoveredLog{}
	}
	response.OK(c, logs)
}

// SyncPolicies creates policies for discovered logs.
func (h *Handler) SyncPolicies(c *gin.Context) {
	serverID, _ := strconv.ParseInt(c.DefaultQuery("server_id", "1"), 10, 64)
	if serverID == 0 {
		serverID = 1
	}
	created, err := h.svc.SyncPoliciesForServer(c.Request.Context(), serverID)
	if err != nil {
		response.InternalError(c, "SYNC_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"created": created})
}

// --- Status ---

// GetStatus returns log management system status.
func (h *Handler) GetStatus(c *gin.Context) {
	serverID, _ := strconv.ParseInt(c.DefaultQuery("server_id", "1"), 10, 64)
	if serverID == 0 {
		serverID = 1
	}
	status, err := h.svc.GetStatusForServer(c.Request.Context(), serverID)
	if err != nil {
		response.InternalError(c, "STATUS_FAILED", err.Error())
		return
	}
	response.OK(c, status)
}

// --- Log Output Config ---

// GetLogOutput returns the log output configuration.
func (h *Handler) GetLogOutput(c *gin.Context) {
	serverID, _ := strconv.ParseInt(c.DefaultQuery("server_id", "1"), 10, 64)
	if serverID == 0 {
		serverID = 1
	}
	cfg, err := h.svc.GetLogOutputForServer(c.Request.Context(), serverID)
	if err != nil {
		response.InternalError(c, "FETCH_FAILED", err.Error())
		return
	}
	response.OK(c, cfg)
}

// SaveLogOutput saves the log output configuration.
func (h *Handler) SaveLogOutput(c *gin.Context) {
	serverID, _ := strconv.ParseInt(c.DefaultQuery("server_id", "1"), 10, 64)
	if serverID == 0 {
		serverID = 1
	}
	var cfg logmanage.LogOutputConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if err := h.svc.SaveLogOutputForServer(c.Request.Context(), serverID, &cfg); err != nil {
		response.InternalError(c, "SAVE_FAILED", err.Error())
		return
	}
	// Sync to remote node if needed
	if serverID > 1 && nodeLogMgrExt != nil {
		data, _ := json.Marshal(cfg)
		nginxSettings := h.getNginxSettingsForSync(c.Request.Context())
		_ = nodeLogMgrExt.SyncLogConfigToServer(c.Request.Context(), serverID, "", string(data), nginxSettings)

		// Regenerate and sync proxy config with new log mode, then reload/restart
		if proxyLogModeApplier != nil {
			if cfg.Nginx.Mode != "" {
				proxyLogModeApplier(c.Request.Context(), serverID, "nginx", cfg.Nginx.Mode)
			}
			if cfg.Caddy.Mode != "" {
				proxyLogModeApplier(c.Request.Context(), serverID, "caddy", cfg.Caddy.Mode)
			}
		}
	}
	response.OK(c, cfg)
}

// GetCoreLogOutput returns the core log output configuration.
func (h *Handler) GetCoreLogOutput(c *gin.Context) {
	serverID, _ := strconv.ParseInt(c.DefaultQuery("server_id", "1"), 10, 64)
	if serverID == 0 {
		serverID = 1
	}
	cfg, err := h.svc.GetCoreLogOutputForServer(c.Request.Context(), serverID)
	if err != nil {
		response.InternalError(c, "FETCH_FAILED", err.Error())
		return
	}
	response.OK(c, cfg)
}

// SaveCoreLogOutput saves the core log output configuration.
func (h *Handler) SaveCoreLogOutput(c *gin.Context) {
	serverID, _ := strconv.ParseInt(c.DefaultQuery("server_id", "1"), 10, 64)
	if serverID == 0 {
		serverID = 1
	}
	var cfg logmanage.CoreLogOutputConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if err := h.svc.SaveCoreLogOutputForServer(c.Request.Context(), serverID, &cfg); err != nil {
		response.InternalError(c, "SAVE_FAILED", err.Error())
		return
	}
	// Sync to remote node if needed
	if serverID > 1 && nodeLogMgrExt != nil {
		data, _ := json.Marshal(cfg)
		nginxSettings := h.getNginxSettingsForSync(c.Request.Context())
		_ = nodeLogMgrExt.SyncLogConfigToServer(c.Request.Context(), serverID, string(data), "", nginxSettings)
	}
	response.OK(c, cfg)
}

// isValidLogType checks if the given type is a supported log source type.
func isValidLogType(t string) bool {
	for _, valid := range logmanage.AllLogTypes() {
		if t == valid {
			return true
		}
	}
	return false
}

// TakeoverStdout enables stdout capture for a process (proxy or core).
func (h *Handler) TakeoverStdout(c *gin.Context) {
	proxyType := c.Param("type")
	if !isValidLogType(proxyType) {
		response.BadRequest(c, "INVALID_TYPE", "unsupported type")
		return
	}
	if err := h.svc.TakeoverStdout(c.Request.Context(), proxyType); err != nil {
		response.InternalError(c, "TAKEOVER_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "stdout capture enabled", "type": proxyType})
}

// ReleaseStdout disables stdout capture for a process (proxy or core).
func (h *Handler) ReleaseStdout(c *gin.Context) {
	proxyType := c.Param("type")
	if !isValidLogType(proxyType) {
		response.BadRequest(c, "INVALID_TYPE", "unsupported type")
		return
	}
	if err := h.svc.ReleaseStdout(c.Request.Context(), proxyType); err != nil {
		response.InternalError(c, "RELEASE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "stdout capture disabled", "type": proxyType})
}

// --- Log Writer Management ---

// GetWriterStatus returns the status of all log writers.
func (h *Handler) GetWriterStatus(c *gin.Context) {
	serverID, _ := strconv.ParseInt(c.Query("server_id"), 10, 64)
	if serverID <= 0 {
		serverID = 1
	}

	// Local node (serverId=1): use local logmanage service
	if serverID == 1 {
		statuses := h.svc.GetWriterStatus()
		response.OK(c, statuses)
		return
	}

	// Remote node: use NodeLogManager
	if nodeLogMgr == nil {
		response.InternalError(c, "NOT_CONFIGURED", "node log manager not configured")
		return
	}

	writers, err := nodeLogMgr.GetWriterStatusOnServer(c.Request.Context(), serverID)
	if err != nil {
		response.InternalError(c, "FETCH_FAILED", err.Error())
		return
	}
	response.OK(c, writers)
}

// StartWriter starts the log writer for a process type (proxy or core).
func (h *Handler) StartWriter(c *gin.Context) {
	proxyType := c.Param("type")
	if !isValidLogType(proxyType) {
		response.BadRequest(c, "INVALID_TYPE", "unsupported type")
		return
	}
	var req struct {
		Mode string `json:"mode" binding:"required"` // "stdout" or "file"
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if req.Mode != "stdout" && req.Mode != "file" {
		response.BadRequest(c, "INVALID_MODE", "mode must be stdout or file")
		return
	}
	serverID, _ := strconv.ParseInt(c.DefaultQuery("server_id", "1"), 10, 64)
	if serverID <= 0 {
		serverID = 1
	}
	if serverID == 1 {
		if err := h.svc.StartWriter(proxyType, req.Mode); err != nil {
			response.InternalError(c, "START_FAILED", err.Error())
			return
		}
	} else {
		if nodeLogMgr == nil {
			response.InternalError(c, "NOT_CONFIGURED", "node log manager not configured")
			return
		}
		if err := nodeLogMgr.StartWriterOnServer(c.Request.Context(), serverID, proxyType, req.Mode); err != nil {
			response.InternalError(c, "START_FAILED", err.Error())
			return
		}
	}
	response.OK(c, gin.H{"message": "writer started", "type": proxyType, "mode": req.Mode})
}

// StopWriter stops the log writer for a process type (proxy or core).
func (h *Handler) StopWriter(c *gin.Context) {
	proxyType := c.Param("type")
	if !isValidLogType(proxyType) {
		response.BadRequest(c, "INVALID_TYPE", "unsupported type")
		return
	}
	serverID, _ := strconv.ParseInt(c.DefaultQuery("server_id", "1"), 10, 64)
	if serverID <= 0 {
		serverID = 1
	}
	if serverID == 1 {
		if err := h.svc.StopWriter(proxyType); err != nil {
			response.InternalError(c, "STOP_FAILED", err.Error())
			return
		}
	} else {
		if nodeLogMgr == nil {
			response.InternalError(c, "NOT_CONFIGURED", "node log manager not configured")
			return
		}
		if err := nodeLogMgr.StopWriterOnServer(c.Request.Context(), serverID, proxyType); err != nil {
			response.InternalError(c, "STOP_FAILED", err.Error())
			return
		}
	}
	response.OK(c, gin.H{"message": "writer stopped", "type": proxyType})
}

// UpdateWriterConfig updates the writer configuration.
func (h *Handler) UpdateWriterConfig(c *gin.Context) {
	proxyType := c.Param("type")
	if !isValidLogType(proxyType) {
		response.BadRequest(c, "INVALID_TYPE", "unsupported type")
		return
	}
	var cfg logmanage.LogWriterConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	serverID, _ := strconv.ParseInt(c.DefaultQuery("server_id", "1"), 10, 64)
	if serverID <= 0 {
		serverID = 1
	}
	if serverID == 1 {
		if err := h.svc.UpdateWriterConfig(proxyType, cfg); err != nil {
			response.InternalError(c, "UPDATE_FAILED", err.Error())
			return
		}
	} else {
		if nodeLogMgr == nil {
			response.InternalError(c, "NOT_CONFIGURED", "node log manager not configured")
			return
		}
		if err := nodeLogMgr.UpdateWriterConfigOnServer(c.Request.Context(), serverID, proxyType, cfg.Mode, int32(cfg.RetainDays), cfg.Compress, int32(cfg.MaxSizeMB)); err != nil {
			response.InternalError(c, "UPDATE_FAILED", err.Error())
			return
		}
	}
	response.OK(c, gin.H{"message": "config updated", "type": proxyType})
}

// ListWriterFiles returns the rotated files for a writer.
func (h *Handler) ListWriterFiles(c *gin.Context) {
	proxyType := c.Param("type")
	if !isValidLogType(proxyType) {
		response.BadRequest(c, "INVALID_TYPE", "unsupported type")
		return
	}
	files := h.svc.ListWriterFiles(proxyType)
	if files == nil {
		files = []logmanage.LogFileInfo{}
	}
	response.OK(c, files)
}

// ProxyStdoutLog returns the last N lines from a process's stdout broker.
func (h *Handler) ProxyStdoutLog(c *gin.Context) {
	proxyType := c.Param("type")
	if !isValidLogType(proxyType) {
		response.BadRequest(c, "INVALID_TYPE", "unsupported type")
		return
	}
	serverID, _ := strconv.ParseInt(c.DefaultQuery("server_id", "1"), 10, 64)
	if serverID <= 0 {
		serverID = 1
	}
	lines, _ := strconv.Atoi(c.DefaultQuery("lines", "500"))

	// Local node (serverId=1): use local logmanage service
	if serverID == 1 {
		logLines, err := h.svc.GetProxyStdoutLog(c.Request.Context(), proxyType, lines)
		if err != nil {
			response.InternalError(c, "READ_FAILED", err.Error())
			return
		}
		response.OK(c, gin.H{"lines": logLines, "type": proxyType, "server_id": serverID})
		return
	}

	// Remote node: use NodeLogManager
	if nodeLogMgr == nil {
		response.InternalError(c, "NOT_CONFIGURED", "node log manager not configured")
		return
	}

	logLines, _, err := nodeLogMgr.ReadLogFileOnServer(c.Request.Context(), serverID, proxyType, lines)
	if err != nil {
		response.InternalError(c, "READ_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"lines": logLines, "type": proxyType, "server_id": serverID})
}

// findNewLines returns lines in current that appeared after the previous batch.
// It finds the longest suffix of previous that matches a prefix of current,
// then returns the remaining lines in current. If no overlap is found (file
// was rotated or too many new lines), returns all current lines.
func findNewLines(previous, current []string) []string {
	if len(previous) == 0 {
		return current
	}
	if len(current) == 0 {
		return nil
	}
	maxOverlap := len(previous)
	if len(current) < maxOverlap {
		maxOverlap = len(current)
	}
	for overlap := maxOverlap; overlap > 0; overlap-- {
		match := true
		for i := 0; i < overlap; i++ {
			if previous[len(previous)-overlap+i] != current[i] {
				match = false
				break
			}
		}
		if match {
			return current[overlap:]
		}
	}
	return current
}

// ProxyStdoutStream streams a process's stdout via SSE.
func (h *Handler) ProxyStdoutStream(c *gin.Context) {
	proxyType := c.Param("type")
	if !isValidLogType(proxyType) {
		response.BadRequest(c, "INVALID_TYPE", "unsupported type")
		return
	}
	serverID, _ := strconv.ParseInt(c.DefaultQuery("server_id", "1"), 10, 64)
	if serverID <= 0 {
		serverID = 1
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// Send initial comment to confirm connection
	c.Writer.WriteString(": connected\n\n")
	c.Writer.Flush()

	// Local node (serverId=1): use local logmanage service
	if serverID == 1 {
		ch := make(chan string, 64)
		go h.svc.StreamProxyStdout(c.Request.Context(), proxyType, ch)

		c.Stream(func(w io.Writer) bool {
			line, ok := <-ch
			if !ok {
				return false
			}
			c.SSEvent("log", line)
			return true
		})
		return
	}

	// Remote node: use NodeLogManager to stream logs
	if nodeLogMgr == nil {
		c.Writer.WriteString(": error: node log manager not configured\n\n")
		c.Writer.Flush()
		return
	}

	// For remote nodes, poll via gRPC every 2 seconds (same as other handlers)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	// Send initial snapshot first
	var prevLines []string
	initLines, _, err := nodeLogMgr.ReadLogFileOnServer(ctx, serverID, proxyType, 200)
	if err == nil && len(initLines) > 0 {
		for _, line := range initLines {
			c.SSEvent("log", line)
		}
		c.Writer.Flush()
		prevLines = initLines
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			lines, _, err := nodeLogMgr.ReadLogFileOnServer(ctx, serverID, proxyType, 200)
			if err != nil {
				continue
			}
			newLines := findNewLines(prevLines, lines)
			if len(newLines) > 0 {
				for _, line := range newLines {
					c.SSEvent("log", line)
				}
				c.Writer.Flush()
			}
			prevLines = lines
		}
	}
}

// --- Log Viewer ---

// GetLog returns the last N lines of a policy's log file.
// For remote node policies (ServerID > 1), reads via gRPC from the agent.
func (h *Handler) GetLog(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	policy, err := h.svc.GetPolicy(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "NOT_FOUND", "policy not found")
		return
	}

	lines, _ := strconv.Atoi(c.DefaultQuery("lines", "500"))
	if lines <= 0 {
		lines = 500
	}

	// Remote node: read log via gRPC from the agent
	if policy.ServerID > 1 {
		if nodeLogMgr == nil {
			response.InternalError(c, "NOT_CONFIGURED", "node log manager not configured")
			return
		}
		logLines, logPath, err := nodeLogMgr.ReadLogFileOnServerWithPath(c.Request.Context(), policy.ServerID, policy.SourceType, policy.LogPath, lines)
		if err != nil {
			// Return friendly message instead of 500 error
			resolvedPath := policy.LogPath
			if resolvedPath == "" {
				resolvedPath = policy.SourceType
			}
			response.OK(c, gin.H{
				"lines": []string{
					fmt.Sprintf("[日志文件不存在] %s", resolvedPath),
					fmt.Sprintf("[提示] %s", err.Error()),
				},
				"path":   resolvedPath,
				"exists": false,
			})
			return
		}
		response.OK(c, gin.H{"lines": logLines, "path": logPath, "exists": len(logLines) > 0})
		return
	}

	// Local node: read from local file system
	logLines, err := h.svc.TailLogWithType(c.Request.Context(), policy.LogPath, policy.SourceType, lines)
	if err != nil {
		response.InternalError(c, "READ_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"lines": logLines, "path": policy.LogPath, "exists": fileExists(policy.LogPath)})
}

// StreamLog streams new log lines via SSE.
// For remote node policies (ServerID > 1), polls the agent via gRPC.
func (h *Handler) StreamLog(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	policy, err := h.svc.GetPolicy(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "NOT_FOUND", "policy not found")
		return
	}

	lines, _ := strconv.Atoi(c.DefaultQuery("lines", "200"))
	if lines <= 0 {
		lines = 200
	}

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// Remote node: poll via gRPC
	if policy.ServerID > 1 {
		if nodeLogMgr == nil {
			c.Writer.WriteString("event: log\ndata: [节点日志管理器未配置]\n\n")
			c.Writer.Flush()
			return
		}

		// Send initial snapshot
		logLines, _, err := nodeLogMgr.ReadLogFileOnServerWithPath(c.Request.Context(), policy.ServerID, policy.SourceType, policy.LogPath, lines)
		if err != nil {
			// Send friendly error as log content instead of HTTP 500
			resolvedPath := policy.LogPath
			if resolvedPath == "" {
				resolvedPath = policy.SourceType
			}
			c.SSEvent("log", fmt.Sprintf("[日志文件不存在] %s", resolvedPath))
			c.SSEvent("log", fmt.Sprintf("[提示] %s", err.Error()))
			c.Writer.Flush()
			return
		}
		if len(logLines) > 0 {
			for _, line := range logLines {
				c.SSEvent("log", line)
			}
			c.Writer.Flush()
		}

		// Poll for new lines every 2 seconds
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		prevLines := logLines
		ctx := c.Request.Context()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				newRead, _, err := nodeLogMgr.ReadLogFileOnServerWithPath(ctx, policy.ServerID, policy.SourceType, policy.LogPath, lines)
				if err != nil || len(newRead) == 0 {
					continue
				}
				diff := findNewLines(prevLines, newRead)
				if len(diff) > 0 {
					for _, line := range diff {
						c.SSEvent("log", line)
					}
					c.Writer.Flush()
				}
				prevLines = newRead
			}
		}
	}

	// Local node: stream from file or broker
	ch := make(chan string, 64)
	go h.svc.TailLogStreamWithType(c.Request.Context(), policy.LogPath, policy.SourceType, lines, ch)

	c.Stream(func(w io.Writer) bool {
		line, ok := <-ch
		if !ok {
			return false
		}
		c.SSEvent("log", line)
		return true
	})
}

// --- Node-based Log Management (Centralized) ---

// fileExists checks if a log file exists on the local filesystem.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// NodeLogManager provides log management for remote nodes.
type NodeLogManager interface {
	ReadLogFileOnServer(ctx context.Context, serverID int64, logType string, lastLines int) ([]string, string, error)
	ReadLogFileOnServerWithPath(ctx context.Context, serverID int64, logType, logPath string, lastLines int) ([]string, string, error)
	RotateLogOnServer(ctx context.Context, serverID int64, logType string) (int32, int32, int64, error)
	DiscoverLogsOnServer(ctx context.Context, serverID int64) ([]*agentpb.DiscoveredLogInfo, error)
	SyncLogConfigToServer(ctx context.Context, serverID int64, coreLogConfig, proxyLogConfig, nginxSettings string) error
	GetWriterStatusOnServer(ctx context.Context, serverID int64) ([]map[string]interface{}, error)
	StartWriterOnServer(ctx context.Context, serverID int64, proxyType, mode string) error
	StopWriterOnServer(ctx context.Context, serverID int64, proxyType string) error
	UpdateWriterConfigOnServer(ctx context.Context, serverID int64, proxyType, mode string, retainDays int32, compress bool, maxSizeMB int32) error
}

// nodeLogMgr is set by the router to enable remote node log management.
var nodeLogMgr NodeLogManager

// nodeLogMgrExt is the same as nodeLogMgr but typed for sync config usage.
// Set alongside nodeLogMgr in SetNodeLogManager.
var nodeLogMgrExt NodeLogManager

// SetNodeLogManager sets the node log manager for centralized log management.
func SetNodeLogManager(m NodeLogManager) {
	nodeLogMgr = m
	nodeLogMgrExt = m
}

// ProxyLogModeApplier is called after log mode is changed for a remote node.
// It should regenerate proxy config with the new log mode, sync it to the
// remote agent, and reload/restart the proxy accordingly.
type ProxyLogModeApplier func(ctx context.Context, serverID int64, proxyType string, newMode string)

// proxyLogModeApplier is set by the router to enable proxy config resync
// after log mode change on remote nodes.
var proxyLogModeApplier ProxyLogModeApplier

// SetProxyLogModeApplier sets the callback for regenerating and syncing
// proxy config after a log mode change on remote nodes.
func SetProxyLogModeApplier(fn ProxyLogModeApplier) {
	proxyLogModeApplier = fn
}

// ListNodeLogs lists available log types for a node.
func (h *Handler) ListNodeLogs(c *gin.Context) {
	types := logmanage.AllLogTypes()
	response.OK(c, gin.H{"log_types": types})
}

// GetNodeLog returns the last N lines of a log from a specific node.
// Supports optional log_path query param to read a specific file.
func (h *Handler) GetNodeLog(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("serverId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_SERVER_ID", "invalid server id")
		return
	}
	logType := c.Param("type")
	if !isValidLogType(logType) {
		response.BadRequest(c, "INVALID_TYPE", "unsupported log type")
		return
	}
	lines, _ := strconv.Atoi(c.DefaultQuery("lines", "500"))
	if lines <= 0 {
		lines = 500
	}
	logPath := c.Query("log_path")

	// Local node (serverId=1)
	if serverID == 1 {
		// If log_path is specified, read from that file
		if logPath != "" {
			logLines, err := h.svc.TailLogWithType(c.Request.Context(), logPath, logType, lines)
			if err != nil {
				response.InternalError(c, "READ_FAILED", err.Error())
				return
			}
			response.OK(c, gin.H{"lines": logLines, "type": logType, "server_id": serverID, "log_path": logPath})
			return
		}
		// No log_path: read from stdout broker
		logLines, err := h.svc.GetProxyStdoutLog(c.Request.Context(), logType, lines)
		if err != nil {
			response.InternalError(c, "READ_FAILED", err.Error())
			return
		}
		response.OK(c, gin.H{"lines": logLines, "type": logType, "server_id": serverID})
		return
	}

	// Remote node: use NodeLogManager
	if nodeLogMgr == nil {
		response.InternalError(c, "NOT_CONFIGURED", "node log manager not configured")
		return
	}
	var logLines []string
	var resolvedPath string
	if logPath != "" {
		logLines, resolvedPath, err = nodeLogMgr.ReadLogFileOnServerWithPath(c.Request.Context(), serverID, logType, logPath, lines)
	} else {
		logLines, resolvedPath, err = nodeLogMgr.ReadLogFileOnServer(c.Request.Context(), serverID, logType, lines)
	}
	if err != nil {
		resolvedPath := logPath
		if resolvedPath == "" {
			resolvedPath = logType
		}
		response.OK(c, gin.H{
			"lines": []string{
				fmt.Sprintf("[日志文件不存在] %s", resolvedPath),
				fmt.Sprintf("[提示] %s", err.Error()),
			},
			"type": logType, "server_id": serverID, "log_path": resolvedPath, "exists": false,
		})
		return
	}
	response.OK(c, gin.H{"lines": logLines, "type": logType, "server_id": serverID, "log_path": resolvedPath, "exists": len(logLines) > 0})
}

// StreamNodeLog streams log lines from a specific node via SSE.
// Supports optional log_path query param to stream a specific file.
func (h *Handler) StreamNodeLog(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("serverId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_SERVER_ID", "invalid server id")
		return
	}
	logType := c.Param("type")
	if !isValidLogType(logType) {
		response.BadRequest(c, "INVALID_TYPE", "unsupported log type")
		return
	}
	logPath := c.Query("log_path")

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	c.Writer.WriteString(": connected\n\n")
	c.Writer.Flush()

	// Local node: use local logmanage service
	if serverID == 1 {
		// If log_path is specified, stream from file
		if logPath != "" {
			ch := make(chan string, 64)
			go h.svc.TailLogStreamWithType(c.Request.Context(), logPath, logType, 200, ch)
			c.Stream(func(w io.Writer) bool {
				line, ok := <-ch
				if !ok {
					return false
				}
				c.SSEvent("log", line)
				return true
			})
			return
		}
		// No log_path: stream from stdout broker
		ch := make(chan string, 64)
		go h.svc.StreamProxyStdout(c.Request.Context(), logType, ch)
		c.Stream(func(w io.Writer) bool {
			line, ok := <-ch
			if !ok {
				return false
			}
			c.SSEvent("log", line)
			return true
		})
		return
	}

	// Remote node: poll via gRPC every 2 seconds
	if nodeLogMgr == nil {
		c.SSEvent("log", "[节点日志管理器未配置]")
		c.Writer.Flush()
		return
	}

	// First, send existing lines
	var logLines []string
	if logPath != "" {
		logLines, _, err = nodeLogMgr.ReadLogFileOnServerWithPath(c.Request.Context(), serverID, logType, logPath, 500)
	} else {
		logLines, _, err = nodeLogMgr.ReadLogFileOnServer(c.Request.Context(), serverID, logType, 500)
	}
	if err == nil && len(logLines) > 0 {
		for _, line := range logLines {
			c.SSEvent("log", line)
		}
		c.Writer.Flush()
	}

	// Then poll for new lines every 2 seconds
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	prevLines := logLines
	ctx := c.Request.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			var lines []string
			if logPath != "" {
				lines, _, err = nodeLogMgr.ReadLogFileOnServerWithPath(ctx, serverID, logType, logPath, 500)
			} else {
				lines, _, err = nodeLogMgr.ReadLogFileOnServer(ctx, serverID, logType, 500)
			}
			if err != nil || len(lines) == 0 {
				continue
			}
			// Send only new lines using diff comparison
			diff := findNewLines(prevLines, lines)
			if len(diff) > 0 {
				for _, line := range diff {
					c.SSEvent("log", line)
				}
				c.Writer.Flush()
			}
			prevLines = lines
		}
	}
}

// RotateNodeLog triggers log rotation on a specific node.
func (h *Handler) RotateNodeLog(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("serverId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_SERVER_ID", "invalid server id")
		return
	}
	logType := c.Param("type")
	if !isValidLogType(logType) {
		response.BadRequest(c, "INVALID_TYPE", "unsupported log type")
		return
	}

	// Local node: use local logmanage service (find policy by source_type)
	if serverID == 1 {
		policies, err := h.svc.ListPolicies(c.Request.Context())
		if err != nil {
			response.InternalError(c, "LIST_FAILED", err.Error())
			return
		}
		for _, p := range policies {
			if p.SourceType == logType {
				job, err := h.svc.Rotate(c.Request.Context(), p.ID)
				if err != nil {
					response.InternalError(c, "ROTATE_FAILED", err.Error())
					return
				}
				response.OK(c, job)
				return
			}
		}
		response.NotFound(c, "POLICY_NOT_FOUND", "no rotation policy found for "+logType)
		return
	}

	// Remote node
	if nodeLogMgr == nil {
		response.InternalError(c, "NOT_CONFIGURED", "node log manager not configured")
		return
	}
	rotated, cleaned, freed, err := nodeLogMgr.RotateLogOnServer(c.Request.Context(), serverID, logType)
	if err != nil {
		response.InternalError(c, "ROTATE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{
		"rotated_files": rotated,
		"cleaned_files": cleaned,
		"freed_bytes":   freed,
	})
}
