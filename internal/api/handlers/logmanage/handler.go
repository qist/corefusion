package logmanage

import (
	"io"
	"strconv"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/model/entity"
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

// --- Policy CRUD ---

// ListPolicies returns all rotation policies.
func (h *Handler) ListPolicies(c *gin.Context) {
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
func (h *Handler) Rotate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
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
	created, err := h.svc.SyncPolicies(c.Request.Context())
	if err != nil {
		response.InternalError(c, "SYNC_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"created": created})
}

// --- Status ---

// GetStatus returns log management system status.
func (h *Handler) GetStatus(c *gin.Context) {
	status, err := h.svc.GetStatus(c.Request.Context())
	if err != nil {
		response.InternalError(c, "STATUS_FAILED", err.Error())
		return
	}
	response.OK(c, status)
}

// --- Log Viewer ---

// GetLog returns the last N lines of a policy's log file.
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

	logLines, err := h.svc.TailLog(c.Request.Context(), policy.LogPath, lines)
	if err != nil {
		response.InternalError(c, "READ_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"lines": logLines, "path": policy.LogPath})
}

// StreamLog streams new log lines via SSE.
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

	ch := make(chan string, 64)
	go h.svc.TailLogStream(c.Request.Context(), policy.LogPath, lines, ch)

	c.Stream(func(w io.Writer) bool {
		line, ok := <-ch
		if !ok {
			return false
		}
		c.SSEvent("log", line)
		return true
	})
}
