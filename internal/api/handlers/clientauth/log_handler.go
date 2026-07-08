package clientauth

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/service/logmanage"
)

// LogHandler handles client log package requests.
type LogHandler struct {
	logSvc *logmanage.Service
}

// NewLogHandler creates a new client log handler.
func NewLogHandler(logSvc *logmanage.Service) *LogHandler {
	return &LogHandler{logSvc: logSvc}
}

// CreateLogPackage handles POST /api/v1/client-auth/logs/package.
// Creates an async log packaging task for the authenticated client.
func (h *LogHandler) CreateLogPackage(c *gin.Context) {
	clientID := getClientID(c)
	if clientID == 0 {
		response.Unauthorized(c, "UNAUTHORIZED", "client not authenticated")
		return
	}

	var req struct {
		Hours int `json:"hours"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		// Allow empty body — default to max hours
		req.Hours = 0
	}

	// Default and cap to 24 hours
	if req.Hours <= 0 || req.Hours > logmanage.MaxClientLogHours {
		req.Hours = logmanage.MaxClientLogHours
	}

	pkg, err := h.logSvc.CreatePackageTask(c.Request.Context(), clientID, req.Hours)
	if err != nil {
		response.BadRequest(c, "PACKAGE_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{
		"id":         pkg.ID,
		"status":     pkg.Status,
		"hours":      pkg.Hours,
		"expires_at": pkg.ExpiresAt,
		"created_at": pkg.CreatedAt,
	})
}

// GetLogPackage handles GET /api/v1/client-auth/logs/packages/:id.
// Returns the status of a log packaging task.
func (h *LogHandler) GetLogPackage(c *gin.Context) {
	clientID := getClientID(c)
	if clientID == 0 {
		response.Unauthorized(c, "UNAUTHORIZED", "client not authenticated")
		return
	}

	pkgID := c.Param("id")
	if pkgID == "" {
		response.BadRequest(c, "INVALID_ID", "package id required")
		return
	}

	pkg, err := h.logSvc.GetPackageStatus(c.Request.Context(), clientID, pkgID)
	if err != nil {
		response.NotFound(c, "PACKAGE_NOT_FOUND", err.Error())
		return
	}

	response.OK(c, gin.H{
		"id":             pkg.ID,
		"status":         pkg.Status,
		"hours":          pkg.Hours,
		"file_size":      pkg.FileSize,
		"error":          pkg.Error,
		"expires_at":     pkg.ExpiresAt,
		"created_at":     pkg.CreatedAt,
		"updated_at":     pkg.UpdatedAt,
	})
}

// ListLogPackages handles GET /api/v1/client-auth/logs/packages.
// Returns a list of log packaging tasks for the authenticated client.
func (h *LogHandler) ListLogPackages(c *gin.Context) {
	clientID := getClientID(c)
	if clientID == 0 {
		response.Unauthorized(c, "UNAUTHORIZED", "client not authenticated")
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	pkgs, total, err := h.logSvc.ListClientPackages(c.Request.Context(), clientID, limit, offset)
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}

	// Transform to lightweight response
	var items []gin.H
	for _, pkg := range pkgs {
		items = append(items, gin.H{
			"id":         pkg.ID,
			"status":     pkg.Status,
			"hours":      pkg.Hours,
			"file_size":  pkg.FileSize,
			"error":      pkg.Error,
			"expires_at": pkg.ExpiresAt,
			"created_at": pkg.CreatedAt,
		})
	}
	if items == nil {
		items = []gin.H{}
	}

	response.Paginated(c, items, total, limit, offset)
}

// DownloadLogPackage handles GET /api/v1/client-auth/logs/packages/:id/download.
// Downloads a ready log package file.
func (h *LogHandler) DownloadLogPackage(c *gin.Context) {
	clientID := getClientID(c)
	if clientID == 0 {
		response.Unauthorized(c, "UNAUTHORIZED", "client not authenticated")
		return
	}

	pkgID := c.Param("id")
	if pkgID == "" {
		response.BadRequest(c, "INVALID_ID", "package id required")
		return
	}

	filePath, fileSize, err := h.logSvc.DownloadPackage(c.Request.Context(), clientID, pkgID)
	if err != nil {
		response.BadRequest(c, "DOWNLOAD_FAILED", err.Error())
		return
	}

	// Check file exists
	if _, err := os.Stat(filePath); err != nil {
		response.NotFound(c, "FILE_NOT_FOUND", "package file not found on disk")
		return
	}

	filename := filepath.Base(filePath)
	c.Header("Content-Type", "application/gzip")
	c.Header("Content-Disposition", "attachment; filename=\""+filename+"\"")
	c.Header("Content-Length", strconv.FormatInt(fileSize, 10))
	c.File(filePath)
}

// LogPackageHealthCheck handles GET /api/v1/client-auth/logs/types.
// Returns available log types and max hours for the client.
func (h *LogHandler) LogPackageHealthCheck(c *gin.Context) {
	clientID := getClientID(c)
	if clientID == 0 {
		response.Unauthorized(c, "UNAUTHORIZED", "client not authenticated")
		return
	}

	response.OK(c, gin.H{
		"max_hours":  logmanage.MaxClientLogHours,
		"log_types":  logmanage.AllLogTypes(),
		"expiry_min": 60,
	})
}