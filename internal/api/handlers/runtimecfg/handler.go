// Package runtimecfg exposes the runtime config workflow HTTP API
// (snapshots / dry-run / apply jobs / rollback / disable-proxy).
package runtimecfg

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/model/dto"
	"corefusion/internal/service/runtimeapply"
)

// Handler handles runtime config workflow requests.
type Handler struct {
	svc *runtimeapply.Service
}

// NewHandler creates a runtime config handler.
func NewHandler(svc *runtimeapply.Service) *Handler {
	return &Handler{svc: svc}
}

type reasonBody struct {
	Reason string `json:"reason"`
}

type rollbackBody struct {
	SnapshotID int64  `json:"snapshot_id" binding:"required"`
	Reason     string `json:"reason"`
}

// Overview handles GET /runtime/overview.
func (h *Handler) Overview(c *gin.Context) {
	out, err := h.svc.Overview(c.Request.Context())
	if err != nil {
		response.InternalError(c, "QUERY_FAILED", err.Error())
		return
	}
	response.OK(c, out)
}

// ListSnapshots handles GET /runtime/snapshots.
func (h *Handler) ListSnapshots(c *gin.Context) {
	var q dto.ListQuery
	_ = c.ShouldBindQuery(&q)
	snaps, total, err := h.svc.ListSnapshots(c.Request.Context(), q.Limit, q.Offset)
	if err != nil {
		response.InternalError(c, "QUERY_FAILED", err.Error())
		return
	}
	response.Paginated(c, snaps, total, q.Limit, q.Offset)
}

// CreateSnapshot handles POST /runtime/snapshots.
func (h *Handler) CreateSnapshot(c *gin.Context) {
	var body reasonBody
	_ = c.ShouldBindJSON(&body)
	snap, err := h.svc.CreateSnapshot(c.Request.Context(), "manual", "api", body.Reason)
	if err != nil {
		response.InternalError(c, "SNAPSHOT_FAILED", err.Error())
		return
	}
	snap.ConfigJSON = "" // do not echo full payload
	response.Created(c, snap)
}

// GetSnapshot handles GET /runtime/snapshots/:id.
func (h *Handler) GetSnapshot(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	snap, err := h.svc.GetSnapshot(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "SNAPSHOT_NOT_FOUND", err.Error())
		return
	}
	response.OK(c, snap)
}

// DeleteSnapshot handles DELETE /runtime/snapshots/:id.
func (h *Handler) DeleteSnapshot(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.svc.DeleteSnapshot(c.Request.Context(), id); err != nil {
		response.InternalError(c, "DELETE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"success": true})
}

// DryRun handles POST /runtime/dry-run.
func (h *Handler) DryRun(c *gin.Context) {
	res, err := h.svc.DryRun(c.Request.Context())
	if err != nil {
		response.InternalError(c, "DRY_RUN_FAILED", err.Error())
		return
	}
	response.OK(c, res)
}

// Apply handles POST /runtime/apply.
func (h *Handler) Apply(c *gin.Context) {
	var body reasonBody
	_ = c.ShouldBindJSON(&body)
	job, err := h.svc.Apply(c.Request.Context(), "api", body.Reason)
	if err != nil && job == nil {
		response.InternalError(c, "APPLY_FAILED", err.Error())
		return
	}
	response.OK(c, job)
}

// ListJobs handles GET /runtime/apply-jobs.
func (h *Handler) ListJobs(c *gin.Context) {
	var q dto.ListQuery
	_ = c.ShouldBindQuery(&q)
	jobs, total, err := h.svc.ListJobs(c.Request.Context(), q.Limit, q.Offset)
	if err != nil {
		response.InternalError(c, "QUERY_FAILED", err.Error())
		return
	}
	response.Paginated(c, jobs, total, q.Limit, q.Offset)
}

// GetJob handles GET /runtime/apply-jobs/:id.
func (h *Handler) GetJob(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	detail, err := h.svc.GetJob(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "JOB_NOT_FOUND", err.Error())
		return
	}
	response.OK(c, detail)
}

// RetryJob handles POST /runtime/apply-jobs/:id/retry.
func (h *Handler) RetryJob(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var body reasonBody
	_ = c.ShouldBindJSON(&body)
	job, err := h.svc.RetryJob(c.Request.Context(), id, body.Reason)
	if err != nil && job == nil {
		response.BadRequest(c, "RETRY_FAILED", err.Error())
		return
	}
	response.OK(c, job)
}

// CancelJob handles POST /runtime/apply-jobs/:id/cancel.
func (h *Handler) CancelJob(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var body reasonBody
	_ = c.ShouldBindJSON(&body)
	if err := h.svc.CancelJob(c.Request.Context(), id, body.Reason); err != nil {
		response.BadRequest(c, "CANCEL_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"success": true})
}

// Rollback handles POST /runtime/rollback.
func (h *Handler) Rollback(c *gin.Context) {
	var body rollbackBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	job, err := h.svc.Rollback(c.Request.Context(), body.SnapshotID, "api", body.Reason)
	if err != nil && job == nil {
		response.BadRequest(c, "ROLLBACK_FAILED", err.Error())
		return
	}
	response.OK(c, job)
}

// DisableProxy handles POST /runtime/disable-proxy.
func (h *Handler) DisableProxy(c *gin.Context) {
	var body reasonBody
	_ = c.ShouldBindJSON(&body)
	job, err := h.svc.DisableProxy(c.Request.Context(), "api", body.Reason)
	if err != nil && job == nil {
		response.InternalError(c, "DISABLE_FAILED", err.Error())
		return
	}
	response.OK(c, job)
}
