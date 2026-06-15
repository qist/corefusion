package georule

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/service/georule"
)

// Handler handles geo rule HTTP requests.
type Handler struct {
	svc       *georule.Service
	scheduler *georule.Scheduler
}

// NewHandler creates a new geo rule handler.
func NewHandler(svc *georule.Service, scheduler *georule.Scheduler) *Handler {
	return &Handler{svc: svc, scheduler: scheduler}
}

// --- Rule Sources ---

// ListSources handles GET /api/v1/geo-rules/sources.
func (h *Handler) ListSources(c *gin.Context) {
	sources, err := h.svc.ListSources(c.Request.Context())
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	response.OK(c, sources)
}

// CreateSource handles POST /api/v1/geo-rules/sources.
func (h *Handler) CreateSource(c *gin.Context) {
	var req struct {
		Name     string `json:"name" binding:"required"`
		RepoURL  string `json:"repo_url" binding:"required"`
		SyncCron string `json:"sync_cron"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	src, err := h.svc.CreateSource(c.Request.Context(), req.Name, req.RepoURL, req.SyncCron)
	if err != nil {
		response.InternalError(c, "CREATE_FAILED", err.Error())
		return
	}
	if h.scheduler != nil {
		h.scheduler.RefreshJob(src.ID, src.Enabled, src.SyncCron)
	}
	response.Created(c, src)
}

// UpdateSource handles PUT /api/v1/geo-rules/sources/:id.
func (h *Handler) UpdateSource(c *gin.Context) {
	var req struct {
		Name     string `json:"name" binding:"required"`
		RepoURL  string `json:"repo_url" binding:"required"`
		SyncCron string `json:"sync_cron"`
		Enabled  *bool  `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	id := parseIntParam(c, "id")
	src, err := h.svc.UpdateSource(c.Request.Context(), id, req.Name, req.RepoURL, req.SyncCron, enabled)
	if err != nil {
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}
	if h.scheduler != nil {
		h.scheduler.RefreshJob(src.ID, src.Enabled, src.SyncCron)
	}
	response.OK(c, src)
}

// DeleteSource handles DELETE /api/v1/geo-rules/sources/:id.
func (h *Handler) DeleteSource(c *gin.Context) {
	id := parseIntParam(c, "id")
	if h.scheduler != nil {
		h.scheduler.RefreshJob(id, false, "")
	}
	if err := h.svc.DeleteSource(c.Request.Context(), id); err != nil {
		response.InternalError(c, "DELETE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "deleted"})
}

// ToggleSchedule handles PUT /api/v1/geo-rules/sources/:id/schedule.
func (h *Handler) ToggleSchedule(c *gin.Context) {
	id := parseIntParam(c, "id")
	var req struct {
		Enabled  bool   `json:"enabled"`
		SyncCron string `json:"sync_cron"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	src, err := h.svc.UpdateSourceSchedule(c.Request.Context(), id, req.Enabled, req.SyncCron)
	if err != nil {
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}
	if h.scheduler != nil {
		h.scheduler.RefreshJob(src.ID, src.Enabled, src.SyncCron)
	}
	response.OK(c, src)
}

// FetchGitHubAssets handles GET /api/v1/geo-rules/github-assets?repo=...
// Proxies GitHub API calls with acceleration for frontend asset preview.
func (h *Handler) FetchGitHubAssets(c *gin.Context) {
	repo := c.Query("repo")
	if repo == "" {
		response.BadRequest(c, "INVALID_REQUEST", "repo is required")
		return
	}
	assets, tag, err := h.svc.FetchGitHubReleaseAssets(c.Request.Context(), repo)
	if err != nil {
		response.InternalError(c, "FETCH_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"assets": assets, "tag": tag})
}

// FetchReleaseAssets handles GET /api/v1/geo-rules/sources/:id/assets.
func (h *Handler) FetchReleaseAssets(c *gin.Context) {
	id := parseIntParam(c, "id")
	assets, tag, err := h.svc.FetchReleaseAssets(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "FETCH_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"assets": assets, "tag": tag})
}

// SelectFiles handles POST /api/v1/geo-rules/sources/:id/select.
func (h *Handler) SelectFiles(c *gin.Context) {
	id := parseIntParam(c, "id")
	var req struct {
		Files []string `json:"files" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if err := h.svc.SelectFiles(c.Request.Context(), id, req.Files); err != nil {
		response.InternalError(c, "SELECT_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "selected"})
}

// SyncSource handles POST /api/v1/geo-rules/sources/:id/sync.
func (h *Handler) SyncSource(c *gin.Context) {
	id := parseIntParam(c, "id")
	key := h.svc.StartSyncSource(id)
	response.OK(c, gin.H{"job_key": key, "status": "running"})
}

// SyncAll handles POST /api/v1/geo-rules/sync-all.
func (h *Handler) SyncAll(c *gin.Context) {
	key := h.svc.StartSyncAll()
	response.OK(c, gin.H{"job_key": key, "status": "running"})
}

// SyncStatus handles GET /api/v1/geo-rules/sync-status/:key.
func (h *Handler) SyncStatus(c *gin.Context) {
	key := c.Param("key")
	job := h.svc.GetSyncJobStatus(key)
	if job == nil {
		response.NotFound(c, "NOT_FOUND", "job not found")
		return
	}
	response.OK(c, job)
}

// --- Rule Files ---

// ListFiles handles GET /api/v1/geo-rules/files.
func (h *Handler) ListFiles(c *gin.Context) {
	files, err := h.svc.ListFiles(c.Request.Context())
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	response.OK(c, files)
}

// Download handles POST /api/v1/geo-rules/files/:id/download.
func (h *Handler) Download(c *gin.Context) {
	id := parseIntParam(c, "id")
	rule, err := h.svc.Download(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "DOWNLOAD_FAILED", err.Error())
		return
	}
	response.OK(c, rule)
}

// DeleteFile handles DELETE /api/v1/geo-rules/files/:id.
func (h *Handler) DeleteFile(c *gin.Context) {
	id := parseIntParam(c, "id")
	if err := h.svc.DeleteFile(c.Request.Context(), id); err != nil {
		response.InternalError(c, "DELETE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "deleted"})
}

// SetAutoSync handles PUT /api/v1/geo-rules/files/:id/auto-sync.
func (h *Handler) SetAutoSync(c *gin.Context) {
	id := parseIntParam(c, "id")
	var req struct {
		AutoSync bool `json:"auto_sync"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if err := h.svc.SetAutoSync(c.Request.Context(), id, req.AutoSync); err != nil {
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"auto_sync": req.AutoSync})
}

func parseIntParam(c *gin.Context, name string) int64 {
	var id int64
	fmt.Sscanf(c.Param(name), "%d", &id)
	return id
}
