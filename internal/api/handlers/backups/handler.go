package backups

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"corefusion/internal/api/response"
	"corefusion/internal/model/entity"
	backupSvc "corefusion/internal/service/backup"
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

// ListConfigBackups handles GET /api/v1/backups/configs.
func (h *Handler) ListConfigBackups(c *gin.Context) {
	backupDir := filepath.Join(h.dataDir, "backup", "configs")
	files, _ := filepath.Glob(filepath.Join(backupDir, "*.tar.gz"))

	items := make([]gin.H, 0, len(files))
	for _, f := range files {
		info, err := os.Stat(f)
		if err == nil {
			items = append(items, gin.H{
				"filename":  filepath.Base(f),
				"size":      info.Size(),
				"created_at": info.ModTime(),
			})
		}
	}

	response.OK(c, items)
}

// CreateConfigBackup handles POST /api/v1/backups/configs.
func (h *Handler) CreateConfigBackup(c *gin.Context) {
	backupDir := filepath.Join(h.dataDir, "backup", "configs")
	os.MkdirAll(backupDir, 0750)

	filename := fmt.Sprintf("config-backup-%s.tar.gz", time.Now().Format("20060102-150405"))
	backupPath := filepath.Join(backupDir, filename)

	// TODO: implement actual config backup (tar configs directory)
	// For now, create an empty file as placeholder
	if err := os.WriteFile(backupPath, []byte{}, 0640); err != nil {
		response.InternalError(c, "BACKUP_FAILED", err.Error())
		return
	}

	response.Created(c, gin.H{
		"filename":  filename,
		"path":      backupPath,
		"created_at": time.Now(),
	})
}

// RestoreConfigBackup handles POST /api/v1/backups/configs/restore.
func (h *Handler) RestoreConfigBackup(c *gin.Context) {
	var req struct {
		Filename string `json:"filename" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	// TODO: implement actual config restore
	response.OK(c, gin.H{"message": "config restore initiated", "filename": req.Filename})
}

// ListDBBackups handles GET /api/v1/backups/database.
func (h *Handler) ListDBBackups(c *gin.Context) {
	backupDir := filepath.Join(h.dataDir, "backup", "database")
	files, _ := filepath.Glob(filepath.Join(backupDir, "*.db"))

	items := make([]gin.H, 0, len(files))
	for _, f := range files {
		info, err := os.Stat(f)
		if err == nil {
			items = append(items, gin.H{
				"filename":  filepath.Base(f),
				"size":      info.Size(),
				"created_at": info.ModTime(),
			})
		}
	}

	response.OK(c, items)
}

// CreateDBBackup handles POST /api/v1/backups/database.
func (h *Handler) CreateDBBackup(c *gin.Context) {
	backupDir := filepath.Join(h.dataDir, "backup", "database")
	os.MkdirAll(backupDir, 0750)

	filename := fmt.Sprintf("db-backup-%s.db", time.Now().Format("20060102-150405"))
	backupPath := filepath.Join(backupDir, filename)

	// Use SQLite VACUUM INTO for consistent backup
	sqlDB, err := h.db.DB()
	if err != nil {
		response.InternalError(c, "BACKUP_FAILED", err.Error())
		return
	}

	_, err = sqlDB.Exec(fmt.Sprintf("VACUUM INTO '%s'", backupPath))
	if err != nil {
		response.InternalError(c, "BACKUP_FAILED", err.Error())
		return
	}

	info, _ := os.Stat(backupPath)
	response.Created(c, gin.H{
		"filename":  filename,
		"path":      backupPath,
		"size":      info.Size(),
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

	// TODO: implement actual DB restore (requires restart)
	response.OK(c, gin.H{"message": "database restore requires restart", "filename": req.Filename})
}

// ListRetentionPolicies handles GET /api/v1/backups/retention-policies.
func (h *Handler) ListRetentionPolicies(c *gin.Context) {
	// TODO: implement
	response.OK(c, []interface{}{})
}

// CreateRetentionPolicy handles POST /api/v1/backups/retention-policies.
func (h *Handler) CreateRetentionPolicy(c *gin.Context) {
	// TODO: implement
	response.OK(c, gin.H{"code": "NOT_IMPLEMENTED"})
}

// UpdateRetentionPolicy handles PUT /api/v1/backups/retention-policies/:id.
func (h *Handler) UpdateRetentionPolicy(c *gin.Context) {
	// TODO: implement
	response.OK(c, gin.H{"code": "NOT_IMPLEMENTED"})
}

// DeleteRetentionPolicy handles DELETE /api/v1/backups/retention-policies/:id.
func (h *Handler) DeleteRetentionPolicy(c *gin.Context) {
	// TODO: implement
	response.OK(c, gin.H{"code": "NOT_IMPLEMENTED"})
}

// ListCleanupJobs handles GET /api/v1/backups/cleanup-jobs.
func (h *Handler) ListCleanupJobs(c *gin.Context) {
	// TODO: implement
	response.Paginated(c, []interface{}{}, 0, 20, 0)
}

// CreateCleanupJob handles POST /api/v1/backups/cleanup-jobs.
func (h *Handler) CreateCleanupJob(c *gin.Context) {
	// TODO: implement
	response.OK(c, gin.H{"code": "NOT_IMPLEMENTED"})
}

// GetSnapshotValidation handles GET /api/v1/backups/snapshots/:id/validation.
func (h *Handler) GetSnapshotValidation(c *gin.Context) {
	// TODO: implement
	response.OK(c, gin.H{"code": "NOT_IMPLEMENTED"})
}

// ValidateSnapshot handles POST /api/v1/backups/snapshots/:id/validate.
func (h *Handler) ValidateSnapshot(c *gin.Context) {
	// TODO: implement
	response.OK(c, gin.H{"code": "NOT_IMPLEMENTED"})
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
	if err := h.svc.RestoreBackup(c.Request.Context(), id); err != nil {
		response.InternalError(c, "RESTORE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "restore completed, restarting..."})

	// Auto restart after restore
	go func() {
		time.Sleep(500 * time.Millisecond)
		restartProcess()
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

	if err := h.svc.RestoreFromFile(c.Request.Context(), file); err != nil {
		response.InternalError(c, "RESTORE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "restore completed, restart recommended"})
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
