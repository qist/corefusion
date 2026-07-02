package georule

import (
	"fmt"
	"strconv"

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

// ListSources handles GET /api/v1/node-servers/:id/geo-rules/sources.
func (h *Handler) ListSources(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		serverID = 1
	}
	sources, err := h.svc.ListSourcesByServer(c.Request.Context(), serverID)
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	response.OK(c, sources)
}

// CreateSource handles POST /api/v1/node-servers/:id/geo-rules/sources.
func (h *Handler) CreateSource(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		serverID = 1
	}
	var req struct {
		Name     string `json:"name" binding:"required"`
		RepoURL  string `json:"repo_url" binding:"required"`
		SyncCron string `json:"sync_cron"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	src, err := h.svc.CreateSource(c.Request.Context(), serverID, req.Name, req.RepoURL, req.SyncCron)
	if err != nil {
		response.InternalError(c, "CREATE_FAILED", err.Error())
		return
	}
	if h.scheduler != nil {
		h.scheduler.RefreshJob(src.ID, src.Enabled, src.SyncCron)
	}
	response.Created(c, src)
}

// UpdateSource handles PUT /api/v1/node-servers/:id/geo-rules/sources/:sourceId.
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
	id := parseIntParam(c, "sourceId")
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

// DeleteSource handles DELETE /api/v1/node-servers/:id/geo-rules/sources/:sourceId.
func (h *Handler) DeleteSource(c *gin.Context) {
	id := parseIntParam(c, "sourceId")
	if h.scheduler != nil {
		h.scheduler.RefreshJob(id, false, "")
	}
	if err := h.svc.DeleteSource(c.Request.Context(), id); err != nil {
		response.InternalError(c, "DELETE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "deleted"})
}

// ToggleSchedule handles PUT /api/v1/node-servers/:id/geo-rules/sources/:sourceId/schedule.
func (h *Handler) ToggleSchedule(c *gin.Context) {
	id := parseIntParam(c, "sourceId")
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

// FetchReleaseAssets handles GET /api/v1/node-servers/:id/geo-rules/sources/:sourceId/assets.
func (h *Handler) FetchReleaseAssets(c *gin.Context) {
	id := parseIntParam(c, "sourceId")
	assets, tag, err := h.svc.FetchReleaseAssets(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "FETCH_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"assets": assets, "tag": tag})
}

// SelectFiles handles POST /api/v1/node-servers/:id/geo-rules/sources/:sourceId/select.
func (h *Handler) SelectFiles(c *gin.Context) {
	id := parseIntParam(c, "sourceId")
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

// SyncSource handles POST /api/v1/node-servers/:id/geo-rules/sources/:sourceId/sync.
func (h *Handler) SyncSource(c *gin.Context) {
	id := parseIntParam(c, "sourceId")
	key := h.svc.StartSyncSource(id)
	response.OK(c, gin.H{"job_key": key, "status": "running"})
}

// SyncAll handles POST /api/v1/node-servers/:id/geo-rules/sync-all.
func (h *Handler) SyncAll(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		serverID = 1
	}
	key := h.svc.StartSyncAllByServer(serverID)
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

// ListFiles handles GET /api/v1/node-servers/:id/geo-rules/files.
func (h *Handler) ListFiles(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		serverID = 1
	}
	files, err := h.svc.ListFilesByServer(c.Request.Context(), serverID)
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	response.OK(c, files)
}

// Download handles POST /api/v1/node-servers/:id/geo-rules/files/:fileId/download.
func (h *Handler) Download(c *gin.Context) {
	id := parseIntParam(c, "fileId")
	rule, err := h.svc.Download(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "DOWNLOAD_FAILED", err.Error())
		return
	}
	response.OK(c, rule)
}

// DeleteFile handles DELETE /api/v1/node-servers/:id/geo-rules/files/:fileId.
func (h *Handler) DeleteFile(c *gin.Context) {
	id := parseIntParam(c, "fileId")
	if err := h.svc.DeleteFile(c.Request.Context(), id); err != nil {
		response.InternalError(c, "DELETE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "deleted"})
}

// SetAutoSync handles PUT /api/v1/node-servers/:id/geo-rules/files/:fileId/auto-sync.
func (h *Handler) SetAutoSync(c *gin.Context) {
	id := parseIntParam(c, "fileId")
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

// Status handles GET /api/v1/node-servers/:id/geo-rules/status.
// Returns which geo files exist and are usable.
func (h *Handler) Status(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		serverID = 1
	}
	files, err := h.svc.ListFilesByServer(c.Request.Context(), serverID)
	if err != nil {
		response.InternalError(c, "STATUS_FAILED", err.Error())
		return
	}
	result := map[string]bool{
		"geoip.dat":    false,
		"geosite.dat":  false,
	}
	for _, f := range files {
		if _, ok := result[f.FileName]; ok {
			result[f.FileName] = true
		}
	}
	response.OK(c, result)
}
