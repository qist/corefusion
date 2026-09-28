package georule

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/service/georule"
)

// Handler handles geo rule HTTP requests.
// Geo rules are global (not per-node), so most endpoints don't require a server ID.
type Handler struct {
	svc       *georule.Service
	scheduler *georule.Scheduler
}

// NewHandler creates a new geo rule handler.
func NewHandler(svc *georule.Service, scheduler *georule.Scheduler) *Handler {
	return &Handler{svc: svc, scheduler: scheduler}
}

// --- Rule Sources (global, no server ID needed) ---

// ListSources handles GET /api/v1/geo-rules/sources.
func (h *Handler) ListSources(c *gin.Context) {
	sources, err := h.svc.ListSourcesByServer(c.Request.Context(), 0)
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
	src, err := h.svc.CreateSource(c.Request.Context(), 0, req.Name, req.RepoURL, req.SyncCron)
	if err != nil {
		response.InternalError(c, "CREATE_FAILED", err.Error())
		return
	}
	if h.scheduler != nil {
		h.scheduler.RefreshJob(src.ID, src.Enabled, src.SyncCron)
	}
	response.Created(c, src)
}

// UpdateSource handles PUT /api/v1/geo-rules/sources/:sourceId.
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

// DeleteSource handles DELETE /api/v1/geo-rules/sources/:sourceId.
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

// ToggleSchedule handles PUT /api/v1/geo-rules/sources/:sourceId/schedule.
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

// FetchReleaseAssets handles GET /api/v1/geo-rules/sources/:sourceId/assets.
func (h *Handler) FetchReleaseAssets(c *gin.Context) {
	id := parseIntParam(c, "sourceId")
	assets, tag, err := h.svc.FetchReleaseAssets(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "FETCH_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"assets": assets, "tag": tag})
}

// SelectFiles handles POST /api/v1/geo-rules/sources/:sourceId/select.
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

// AddFile handles POST /api/v1/geo-rules/sources/:sourceId/files.
// 手动向源添加一个任意 URL 的规则文件（典型：sing-box .srs 分支 raw URL），
// 添加后异步下载到面板 data/rules/ 并推送所有节点。
func (h *Handler) AddFile(c *gin.Context) {
	id := parseIntParam(c, "sourceId")
	var req struct {
		Name string `json:"name" binding:"required"`
		URL  string `json:"url" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	rule, err := h.svc.AddFile(c.Request.Context(), id, req.Name, req.URL)
	if err != nil {
		response.InternalError(c, "ADD_FILE_FAILED", err.Error())
		return
	}
	response.OK(c, rule)
}

// SyncSource handles POST /api/v1/geo-rules/sources/:sourceId/sync.
func (h *Handler) SyncSource(c *gin.Context) {
	id := parseIntParam(c, "sourceId")
	key := h.svc.StartSyncSource(id)
	response.OK(c, gin.H{"job_key": key, "status": "running"})
}

// SyncAll handles POST /api/v1/geo-rules/sync-all.
// Syncs all sources globally, then auto-pushes to all nodes.
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

// --- Rule Files (global) ---

// ListFiles handles GET /api/v1/geo-rules/files.
func (h *Handler) ListFiles(c *gin.Context) {
	files, err := h.svc.ListFilesByServer(c.Request.Context(), 0)
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	response.OK(c, files)
}

// Download handles POST /api/v1/geo-rules/files/:fileId/download.
// Triggers a download from GitHub to the panel's local data/rules/ directory.
func (h *Handler) Download(c *gin.Context) {
	id := parseIntParam(c, "fileId")
	rule, err := h.svc.Download(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "DOWNLOAD_FAILED", err.Error())
		return
	}
	// After download, auto-push this file to all nodes
	go h.svc.SyncToAllNodes(c.Request.Context())
	response.OK(c, rule)
}

// DeleteFile handles DELETE /api/v1/geo-rules/files/:fileId.
func (h *Handler) DeleteFile(c *gin.Context) {
	id := parseIntParam(c, "fileId")
	if err := h.svc.DeleteFile(c.Request.Context(), id); err != nil {
		response.InternalError(c, "DELETE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "deleted"})
}

// SetAutoSync handles PUT /api/v1/geo-rules/files/:fileId/auto-sync.
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

// DownloadFile handles GET /api/v1/geo-rules/files/:fileId/download-file.
// Serves the actual file content from the panel's data/rules/ directory.
// This endpoint is used by remote Agents to download geo rule files from the panel.
func (h *Handler) DownloadFile(c *gin.Context) {
	id := parseIntParam(c, "fileId")
	filePath, fileName, err := h.svc.ServeFileContent(id)
	if err != nil {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", fileName))
	c.File(filePath)
}

// PushToAllNodes handles POST /api/v1/geo-rules/push-all.
// Pushes all geo rule files to all online nodes.
func (h *Handler) PushToAllNodes(c *gin.Context) {
	if err := h.svc.SyncToAllNodes(c.Request.Context()); err != nil {
		response.InternalError(c, "PUSH_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "pushed"})
}

// PushToNode handles POST /api/v1/node-servers/:id/geo-rules/push.
// Pushes all geo rule files to a specific node.
func (h *Handler) PushToNode(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		serverID = 0
	}
	if err := h.svc.SyncToNode(c.Request.Context(), serverID); err != nil {
		response.InternalError(c, "PUSH_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "pushed"})
}

// SeedDefaults handles POST /api/v1/geo-rules/seed.
// Creates default rule sources and files if none exist.
func (h *Handler) SeedDefaults(c *gin.Context) {
	if err := h.svc.SeedDefaults(c.Request.Context(), 0); err != nil {
		response.InternalError(c, "SEED_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "seeded"})
}

// Status handles GET /api/v1/geo-rules/status.
// Returns which geo files exist and are usable.
func (h *Handler) Status(c *gin.Context) {
	files, err := h.svc.ListFilesByServer(c.Request.Context(), 0)
	if err != nil {
		response.InternalError(c, "STATUS_FAILED", err.Error())
		return
	}
	result := map[string]bool{
		"geoip.dat":   false,
		"geosite.dat": false,
	}
	for _, f := range files {
		if _, ok := result[f.FileName]; ok {
			result[f.FileName] = true
		}
	}
	response.OK(c, result)
}

func parseIntParam(c *gin.Context, name string) int64 {
	var id int64
	fmt.Sscanf(c.Param(name), "%d", &id)
	return id
}
