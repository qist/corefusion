package backups

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"corefusion/internal/api/middleware"
	"corefusion/internal/api/response"
	"corefusion/internal/model/entity"
	backupSvc "corefusion/internal/service/backup"
	"corefusion/internal/store/db"
)

// Handler handles backup HTTP requests.
type Handler struct {
	db      *gorm.DB
	dataDir string
	svc     *backupSvc.Service
}

// NewHandler creates a new backup handler.
func NewHandler(db *gorm.DB, dataDir string, svc *backupSvc.Service) *Handler {
	return &Handler{db: db, dataDir: dataDir, svc: svc}
}

// ListDBBackups handles GET /api/v1/backups/database.
func (h *Handler) ListDBBackups(c *gin.Context) {
	backupDir := filepath.Join(h.dataDir, "backup", "database")
	files, _ := filepath.Glob(filepath.Join(backupDir, "*.db"))
	filesSQL, _ := filepath.Glob(filepath.Join(backupDir, "*.sql"))
	files = append(files, filesSQL...)

	items := make([]gin.H, 0, len(files))
	for _, f := range files {
		info, err := os.Stat(f)
		if err == nil {
			items = append(items, gin.H{
				"filename":   filepath.Base(f),
				"size":       info.Size(),
				"created_at": info.ModTime(),
			})
		}
	}

	response.OK(c, items)
}

// CreateDBBackup handles POST /api/v1/backups/database.
func (h *Handler) CreateDBBackup(c *gin.Context) {
	ctx := c.Request.Context()
	dbType, dsn := db.StartupConfig()

	backupDir := filepath.Join(h.dataDir, "backup", "database")
	os.MkdirAll(backupDir, 0750)

	// SQLite uses a .db VACUUM copy; MySQL/PostgreSQL use a pure-Go .sql dump.
	ext := ".db"
	if dbType == db.DBTypeMySQL || dbType == db.DBTypePostgres {
		ext = ".sql"
	}
	filename := fmt.Sprintf("db-backup-%s%s", time.Now().Format("20060102-150405"), ext)
	backupPath := filepath.Join(backupDir, filename)

	if err := h.svc.DumpDatabase(ctx, dbType, dsn, backupPath); err != nil {
		response.InternalError(c, "BACKUP_FAILED", err.Error())
		return
	}

	info, _ := os.Stat(backupPath)
	response.Created(c, gin.H{
		"filename":   filename,
		"path":       backupPath,
		"size":       info.Size(),
		"created_at": time.Now(),
	})
}

// RestoreDBBackup handles POST /api/v1/backups/database/restore.
func (h *Handler) RestoreDBBackup(c *gin.Context) {
	var req struct {
		Filename string `json:"filename" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	// 安全审计 L-1：filename 用户可控，必须是纯文件名且限定备份扩展名，
	// 防 `../` 穿越把任意文件当作恢复源读入执行。
	if req.Filename != filepath.Base(req.Filename) {
		response.BadRequest(c, "INVALID_FILENAME", "filename must be a plain file name")
		return
	}
	switch filepath.Ext(req.Filename) {
	case ".db", ".sql":
	default:
		response.BadRequest(c, "INVALID_FILENAME", "filename must be a .db or .sql backup")
		return
	}

	backupPath := filepath.Join(h.dataDir, "backup", "database", req.Filename)
	if _, err := os.Stat(backupPath); err != nil {
		response.NotFound(c, "BACKUP_NOT_FOUND", "backup file not found: "+req.Filename)
		return
	}

	dbType, dsn := db.StartupConfig()
	// Judge the environment DB configuration so the dump is restored into the
	// correct database (mysql vs postgres from a .sql file, sqlite from a .db file).
	restoreType, err := backupSvc.ResolveRestoreType(backupPath, dbType)
	if err != nil {
		response.BadRequest(c, "INVALID_BACKUP", err.Error())
		return
	}
	needRestart, err := h.svc.RestoreDatabase(c.Request.Context(), restoreType, dsn, backupPath)
	if err != nil {
		response.InternalError(c, "RESTORE_FAILED", err.Error())
		return
	}

	// For sqlite the db file was swapped; restart to reload it.
	if needRestart {
		go restartProcess()
	}

	response.OK(c, gin.H{"message": "database restored", "filename": req.Filename, "restart_required": needRestart})
}

// CreateCleanupJob handles POST /api/v1/backups/cleanup-jobs.
// Triggers the retention policy manually: removes backups older than retain_days
// (defaults to 30 when not provided or <= 0).
func (h *Handler) CreateCleanupJob(c *gin.Context) {
	var req struct {
		RetainDays int `json:"retain_days"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if req.RetainDays <= 0 {
		req.RetainDays = 30
	}

	removed, err := h.svc.EnforceRetention(c.Request.Context(), req.RetainDays)
	if err != nil {
		response.InternalError(c, "CLEANUP_FAILED", err.Error())
		return
	}
	response.Created(c, gin.H{
		"removed":     removed,
		"retain_days": req.RetainDays,
		"message":     fmt.Sprintf("已清理 %d 个超过 %d 天的备份", removed, req.RetainDays),
	})
}

// --- Full Backup ---

// CreateFullBackup handles POST /api/v1/backups/create.
func (h *Handler) CreateFullBackup(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	c.ShouldBindJSON(&req)

	rec, err := h.svc.CreateBackup(c.Request.Context(), req.Name, "manual")
	if err != nil {
		response.InternalError(c, "BACKUP_FAILED", err.Error())
		return
	}
	response.Created(c, rec)
}

// ListFullBackups handles GET /api/v1/backups/list.
func (h *Handler) ListFullBackups(c *gin.Context) {
	backups, err := h.svc.ListBackups(c.Request.Context())
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	response.OK(c, backups)
}

// DownloadBackup handles GET /api/v1/backups/:id/download.
func (h *Handler) DownloadBackup(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	filePath, fileName, err := h.svc.GetBackupFile(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "NOT_FOUND", "backup not found")
		return
	}
	c.FileAttachment(filePath, fileName)
}

// RestoreBackup handles POST /api/v1/backups/:id/restore.
func (h *Handler) RestoreBackup(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	st, err := h.svc.RestoreBackupAsync(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "RESTORE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "restore started", "restore_id": st.ID, "status": st.Status})

	// Auto restart after restore completes (checked via status polling)
	go func() {
		for i := 0; i < 3600; i++ {
			time.Sleep(2 * time.Second)
			cur := h.svc.GetRestoreStatus()
			if cur == nil || cur.ID != st.ID {
				continue
			}
			if cur.Status == "done" {
				time.Sleep(500 * time.Millisecond)
				restartProcess()
				return
			}
			if cur.Status == "error" {
				return
			}
		}
	}()
}

// DeleteBackup handles DELETE /api/v1/backups/:id.
func (h *Handler) DeleteBackup(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	if err := h.svc.DeleteBackup(c.Request.Context(), id); err != nil {
		response.InternalError(c, "DELETE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "deleted"})
}

// UploadRestore handles POST /api/v1/backups/upload-restore.
func (h *Handler) UploadRestore(c *gin.Context) {
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		response.BadRequest(c, "INVALID_REQUEST", "file is required")
		return
	}
	defer file.Close()

	st, err := h.svc.RestoreFromFileAsync(c.Request.Context(), file)
	if err != nil {
		response.InternalError(c, "RESTORE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "restore started", "restore_id": st.ID, "status": st.Status})

	go func() {
		for i := 0; i < 3600; i++ {
			time.Sleep(2 * time.Second)
			cur := h.svc.GetRestoreStatus()
			if cur == nil || cur.ID != st.ID {
				continue
			}
			if cur.Status == "done" {
				time.Sleep(500 * time.Millisecond)
				restartProcess()
				return
			}
			if cur.Status == "error" {
				return
			}
		}
	}()
}

// --- Schedules ---

// ListSchedules handles GET /api/v1/backups/schedules.
func (h *Handler) ListSchedules(c *gin.Context) {
	schedules, err := h.svc.ListSchedules(c.Request.Context())
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	response.OK(c, schedules)
}

// CreateSchedule handles POST /api/v1/backups/schedules.
func (h *Handler) CreateSchedule(c *gin.Context) {
	var sched entity.BackupSchedule
	if err := c.ShouldBindJSON(&sched); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if err := h.svc.CreateSchedule(c.Request.Context(), &sched); err != nil {
		response.InternalError(c, "CREATE_FAILED", err.Error())
		return
	}
	response.Created(c, sched)
}

// UpdateSchedule handles PUT /api/v1/backups/schedules/:id.
func (h *Handler) UpdateSchedule(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	var sched entity.BackupSchedule
	if err := c.ShouldBindJSON(&sched); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	sched.ID = id
	if err := h.svc.UpdateSchedule(c.Request.Context(), &sched); err != nil {
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}
	response.OK(c, sched)
}

// DeleteSchedule handles DELETE /api/v1/backups/schedules/:id.
func (h *Handler) DeleteSchedule(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	if err := h.svc.DeleteSchedule(c.Request.Context(), id); err != nil {
		response.InternalError(c, "DELETE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "deleted"})
}

// --- Storages ---

// ListStorages handles GET /api/v1/backups/storages.
func (h *Handler) ListStorages(c *gin.Context) {
	storages, err := h.svc.ListStorages(c.Request.Context())
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	response.OK(c, storages)
}

// CreateStorage handles POST /api/v1/backups/storages.
func (h *Handler) CreateStorage(c *gin.Context) {
	var storage entity.BackupStorage
	if err := c.ShouldBindJSON(&storage); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if err := h.svc.CreateStorage(c.Request.Context(), &storage); err != nil {
		response.InternalError(c, "CREATE_FAILED", err.Error())
		return
	}
	response.Created(c, storage)
}

// UpdateStorage handles PUT /api/v1/backups/storages/:id.
func (h *Handler) UpdateStorage(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	var storage entity.BackupStorage
	if err := c.ShouldBindJSON(&storage); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	storage.ID = id
	if err := h.svc.UpdateStorage(c.Request.Context(), &storage); err != nil {
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}
	response.OK(c, storage)
}

// DeleteStorage handles DELETE /api/v1/backups/storages/:id.
func (h *Handler) DeleteStorage(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	if err := h.svc.DeleteStorage(c.Request.Context(), id); err != nil {
		response.InternalError(c, "DELETE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "deleted"})
}

// UploadToFTP handles POST /api/v1/backups/:id/upload-ftp.
func (h *Handler) UploadToFTP(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}

	var req struct {
		backupSvc.FTPConfig
		DeleteLocal bool `json:"delete_local"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	if err := h.svc.UploadToFTP(c.Request.Context(), id, req.FTPConfig); err != nil {
		response.InternalError(c, "UPLOAD_FAILED", err.Error())
		return
	}

	if req.DeleteLocal {
		if err := h.svc.DeleteLocalFile(c.Request.Context(), id); err != nil {
			response.OK(c, gin.H{"message": "uploaded to FTP successfully (local cleanup failed)", "local_deleted": false})
			return
		}
		response.OK(c, gin.H{"message": "uploaded to FTP successfully, local backup cleaned", "local_deleted": true})
		return
	}

	response.OK(c, gin.H{"message": "uploaded to FTP successfully", "local_deleted": false})
}

// UploadToS3 handles POST /api/v1/backups/:id/upload-s3.
func (h *Handler) UploadToS3(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}

	var req struct {
		backupSvc.S3Config
		DeleteLocal bool `json:"delete_local"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	if err := h.svc.UploadToS3(c.Request.Context(), id, req.S3Config); err != nil {
		response.InternalError(c, "UPLOAD_FAILED", err.Error())
		return
	}

	if req.DeleteLocal {
		if err := h.svc.DeleteLocalFile(c.Request.Context(), id); err != nil {
			response.OK(c, gin.H{"message": "uploaded to S3 successfully (local cleanup failed)", "local_deleted": false})
			return
		}
		response.OK(c, gin.H{"message": "uploaded to S3 successfully, local backup cleaned", "local_deleted": true})
		return
	}

	response.OK(c, gin.H{"message": "uploaded to S3 successfully", "local_deleted": false})
}

// UploadToWebDAV handles POST /api/v1/backups/:id/upload-webdav.
func (h *Handler) UploadToWebDAV(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}

	var req struct {
		backupSvc.WebDAVConfig
		DeleteLocal bool `json:"delete_local"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	if err := h.svc.UploadToWebDAV(c.Request.Context(), id, req.WebDAVConfig); err != nil {
		response.InternalError(c, "UPLOAD_FAILED", err.Error())
		return
	}

	if req.DeleteLocal {
		if err := h.svc.DeleteLocalFile(c.Request.Context(), id); err != nil {
			response.OK(c, gin.H{"message": "uploaded to WebDAV successfully (local cleanup failed)", "local_deleted": false})
			return
		}
		response.OK(c, gin.H{"message": "uploaded to WebDAV successfully, local backup cleaned", "local_deleted": true})
		return
	}

	response.OK(c, gin.H{"message": "uploaded to WebDAV successfully", "local_deleted": false})
}

// ListRemoteFiles handles GET /api/v1/backups/storages/:id/files.
// Lists backup files available on remote storage.
func (h *Handler) ListRemoteFiles(c *gin.Context) {
	storageID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid storage id")
		return
	}

	files, err := h.svc.ListRemoteFiles(c.Request.Context(), storageID)
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	response.OK(c, files)
}

// RemoteRestore handles POST /api/v1/backups/remote-restore.
// Downloads a backup from remote storage and restores it asynchronously.
func (h *Handler) RemoteRestore(c *gin.Context) {
	var req struct {
		StorageID int64  `json:"storage_id" binding:"required"`
		Filename  string `json:"filename" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	st, err := h.svc.RemoteRestoreAsync(c.Request.Context(), req.StorageID, req.Filename)
	if err != nil {
		response.InternalError(c, "RESTORE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "restore started", "restore_id": st.ID, "status": st.Status})

	go func() {
		for i := 0; i < 3600; i++ {
			time.Sleep(2 * time.Second)
			cur := h.svc.GetRestoreStatus()
			if cur == nil || cur.ID != st.ID {
				continue
			}
			if cur.Status == "done" {
				time.Sleep(500 * time.Millisecond)
				restartProcess()
				return
			}
			if cur.Status == "error" {
				return
			}
		}
	}()
}

// GetRestoreStatus handles GET /api/v1/backups/restore-status.
// Returns the current async restore status + maintenance info.
func (h *Handler) GetRestoreStatus(c *gin.Context) {
	st := h.svc.GetRestoreStatus()
	maintActive, maintMsg, maintSource := middleware.GetMaintenanceInfo()
	if st == nil {
		response.OK(c, gin.H{
			"status":              "idle",
			"maintenance":         maintActive,
			"maintenance_message": maintMsg,
			"maintenance_source":  maintSource,
		})
		return
	}
	response.OK(c, gin.H{
		"id":                  st.ID,
		"status":              st.Status,
		"message":             st.Message,
		"started_at":          st.StartedAt,
		"finished_at":         st.DoneAt,
		"maintenance":         maintActive,
		"maintenance_message": maintMsg,
		"maintenance_source":  maintSource,
	})
}
