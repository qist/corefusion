package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"corefusion/internal/api/middleware"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/db"
	"corefusion/internal/store/repository"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jackc/pgx/v5"
	"github.com/jlaffaye/ftp"
	"github.com/qist/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// RestoreStatus tracks the state of an async restore operation.
type RestoreStatus struct {
	ID        int64     `json:"id"`
	Status    string    `json:"status"` // pending, downloading, restoring, done, error
	Message   string    `json:"message"`
	StartedAt time.Time `json:"started_at"`
	DoneAt    time.Time `json:"done_at,omitempty"`
}

// Service handles backup business logic.
type Service struct {
	repo          *repository.BackupRepository
	dataDir       string
	db            *gorm.DB
	restoreMu     sync.Mutex
	restoreStatus *RestoreStatus
	nextRestoreID int64
}

// NewService creates a new backup service.
func NewService(repo *repository.BackupRepository, dataDir string, database *gorm.DB) *Service {
	return &Service{repo: repo, dataDir: dataDir, db: database}
}

// CreateBackup creates a database-only backup.
//
// Only the database is backed up — everything else can be reconstructed on
// startup from the database:
//   - CA roots (ca_roots table) → EnsureCA materializes PEM to disk
//   - Domain certs (certificates table cert_content/key_content) → SyncCertsToDisk
//   - Core binaries (xray/singbox/…) → auto-downloaded from GitHub Releases
//   - Rules (GeoIP/geosite) → auto-downloaded from configured URLs
//   - Runtime configs → regenerated from database core_configs
//
// This keeps the backup small (~DB size only) and avoids "cannot overwrite
// running binary" issues during restore.
func (s *Service) CreateBackup(ctx context.Context, name, backupType string) (*entity.BackupRecord, error) {
	if name == "" {
		name = fmt.Sprintf("backup_%s", time.Now().Format("20060102_150405"))
	}

	backupDir := filepath.Join(s.dataDir, "backup")
	os.MkdirAll(backupDir, 0750)

	fileName := name + ".tar.gz"
	filePath := filepath.Join(backupDir, fileName)

	// Create backup in a temporary file first, then rename to avoid including it in the archive
	tmpPath := filePath + ".tmp"

	// Dump the database — this is the only content in the backup archive.
	dbType, dsn := db.StartupConfig()
	var dumpPath, archiveName string
	if dbType != "" && dbType != db.DBTypeSQLite {
		dumpPath = tmpPath + ".sqldump"
		if err := s.DumpDatabase(ctx, dbType, dsn, dumpPath); err != nil {
			os.Remove(dumpPath)
			return nil, fmt.Errorf("dump database for backup: %w", err)
		}
		dbName := dbNameFromDSN(dbType, dsn)
		archiveName = "db_dump/" + dbName + ".sql"
	} else {
		// sqlite: Online Backup (hot, page-by-page, with VACUUM INTO fallback)
		dumpPath = tmpPath + ".vaccumdb"
		if err := s.DumpDatabase(ctx, dbType, dsn, dumpPath); err != nil {
			os.Remove(dumpPath)
			return nil, fmt.Errorf("dump database for backup: %w", err)
		}
		archiveName = "sqlite/app.db"
	}
	defer os.Remove(dumpPath)

	// Create a tar.gz containing ONLY the database dump.
	if err := s.createDumpOnlyTarGz(tmpPath, dumpPath, archiveName); err != nil {
		os.Remove(tmpPath)
		return nil, fmt.Errorf("create backup: %w", err)
	}

	// Rename temp file to final path
	if err := os.Rename(tmpPath, filePath); err != nil {
		os.Remove(tmpPath)
		return nil, fmt.Errorf("move backup file: %w", err)
	}

	stat, _ := os.Stat(filePath)
	rec := &entity.BackupRecord{
		Name:     name,
		Type:     backupType,
		FilePath: filePath,
		FileSize: stat.Size(),
		Status:   "completed",
	}

	if err := s.repo.CreateRecord(ctx, rec); err != nil {
		return nil, err
	}

	return rec, nil
}

// createDumpOnlyTarGz creates a tar.gz archive containing only the database
// dump file. This replaces the old createTarGz approach that walked the entire
// data directory, keeping backups minimal (database-only).
func (s *Service) createDumpOnlyTarGz(dest, dumpPath, archiveName string) error {
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	return s.addFileToTar(tw, dumpPath, archiveName)
}

// ListBackups returns all backup records.
func (s *Service) ListBackups(ctx context.Context) ([]entity.BackupRecord, error) {
	return s.repo.ListRecords(ctx)
}

// DeleteBackup deletes a backup record and file.
func (s *Service) DeleteBackup(ctx context.Context, id int64) error {
	rec, err := s.repo.GetRecordByID(ctx, id)
	if err != nil {
		return err
	}
	os.Remove(rec.FilePath)
	return s.repo.DeleteRecord(ctx, id)
}

// DeleteLocalFile removes the local backup file but keeps the database record,
// marking it as remote_only. This is used after a successful remote upload when
// the user wants to free local disk space but still see the backup in the list
// and be able to restore it from remote.
func (s *Service) DeleteLocalFile(ctx context.Context, id int64) error {
	rec, err := s.repo.GetRecordByID(ctx, id)
	if err != nil {
		return err
	}
	if rec.FilePath != "" {
		os.Remove(rec.FilePath)
	}
	return s.repo.MarkRemoteOnly(ctx, id)
}

// EnforceRetention removes backups older than retainDays days.
// retainDays <= 0 disables cleanup (nothing is removed). It prunes both the
// database-tracked full backups (backup_records) and the standalone
// database/config dump files kept under dataDir/backup.
func (s *Service) EnforceRetention(ctx context.Context, retainDays int) (removed int, err error) {
	if retainDays <= 0 {
		return 0, nil
	}
	cutoff := time.Now().AddDate(0, 0, -retainDays)

	recs, err := s.repo.ListRecords(ctx)
	if err != nil {
		return 0, fmt.Errorf("list backups for retention: %w", err)
	}
	for i := range recs {
		if recs[i].CreatedAt.IsZero() || !recs[i].CreatedAt.Before(cutoff) {
			continue
		}
		if derr := s.DeleteBackup(ctx, recs[i].ID); derr != nil {
			log.Printf("[BackupRetention] failed to remove backup %d: %v", recs[i].ID, derr)
			continue
		}
		removed++
	}

	// Also prune standalone database/config dump files by modification time.
	removed += s.pruneDir(filepath.Join(s.dataDir, "backup", "database"), []string{".db", ".sql"}, cutoff)
	removed += s.pruneDir(filepath.Join(s.dataDir, "backup", "configs"), []string{".tar.gz"}, cutoff)

	return removed, nil
}

// pruneDir removes files in dir whose extension is in exts and whose
// modification time is older than cutoff. Returns the number of files removed.
func (s *Service) pruneDir(dir string, exts []string, cutoff time.Time) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	removed := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		matched := false
		for _, ext := range exts {
			if strings.HasSuffix(e.Name(), ext) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if !info.ModTime().Before(cutoff) {
			continue
		}
		if err := os.Remove(filepath.Join(dir, e.Name())); err == nil {
			removed++
		}
	}
	return removed
}

// RestoreBackup restores from a backup file (legacy sync version, use RestoreBackupAsync for non-blocking).
func (s *Service) RestoreBackup(ctx context.Context, id int64) error {
	rec, err := s.repo.GetRecordByID(ctx, id)
	if err != nil {
		return err
	}
	return s.restoreFromArchive(ctx, rec.FilePath)
}

// importDumpsFromArchive imports any db_dump/*.sql files extracted from a full backup.
func (s *Service) importDumpsFromArchive(ctx context.Context, dbType, dsn string) error {
	matches, _ := filepath.Glob(filepath.Join(s.dataDir, "db_dump", "*.sql"))
	for _, m := range matches {
		content, err := os.ReadFile(m)
		if err != nil {
			return fmt.Errorf("read db dump %s: %w", m, err)
		}
		switch dbType {
		case db.DBTypeMySQL:
			if err := restoreMySQL(ctx, dsn, string(content)); err != nil {
				return err
			}
		case db.DBTypePostgres:
			if err := restorePostgres(ctx, dsn, string(content)); err != nil {
				return err
			}
		}
		os.Remove(m)
	}
	os.RemoveAll(filepath.Join(s.dataDir, "db_dump"))
	return nil
}

// RestoreFromFile restores from an uploaded file (upload-restore).
// The file must be a .tar.gz archive containing either sqlite/app.db or
// db_dump/*.sql, produced by CreateBackup.
func (s *Service) RestoreFromFile(ctx context.Context, reader io.Reader) error {
	tmpPath := filepath.Join(s.dataDir, "backup", "restore_tmp.tar.gz")
	os.MkdirAll(filepath.Dir(tmpPath), 0750)
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, reader); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return err
	}
	f.Close()
	defer os.Remove(tmpPath)

	return s.restoreFromArchive(ctx, tmpPath)
}

// restoreFromArchive extracts a tar.gz archive and imports the database dump
// it contains, using the current environment DB configuration. This is the
// shared logic used by RestoreBackup (local), RestoreFromFile (upload), and
// RemoteRestore (FTP/S3/WebDAV pull).
func (s *Service) restoreFromArchive(ctx context.Context, archivePath string) error {
	if err := s.extractTarGz(archivePath, s.dataDir); err != nil {
		return err
	}

	dbType, dsn := db.StartupConfig()
	switch dbType {
	case db.DBTypeMySQL, db.DBTypePostgres:
		if dsn == "" {
			return fmt.Errorf("数据库 DSN 未配置（环境变量 CF_DB_DSN），无法还原 %s 备份", dbTypeLabel(dbType))
		}
		// First try importing db_dump/*.sql (same-db-type backup)
		importErr := s.importDumpsFromArchive(ctx, dbType, dsn)
		if importErr != nil {
			// Direct import failed — this might be a cross-db restore
			// (e.g. MySQL dump on PostgreSQL). Try the cross-db fallback:
			// load the SQL dump into a temporary SQLite database, then
			// migrate from SQLite to the target DB.
			log.Printf("[Backup] direct SQL import failed, trying cross-db migration: %v", importErr)
			if crossErr := s.crossDBDumpRestore(ctx, dbType, dsn); crossErr != nil {
				// Cross-db also failed — return the original error
				return importErr
			}
			importErr = nil
		}
		if importErr != nil {
			return importErr
		}
		// If no SQL dumps were found, check for a SQLite app.db in the archive.
		// This is a cross-database restore: the backup was created on SQLite,
		// but the panel is now running on MySQL/PostgreSQL. We need to open
		// the extracted SQLite file and migrate its data into the target DB.
		sqlitePath := filepath.Join(s.dataDir, "sqlite", "app.db")
		if info, err := os.Stat(sqlitePath); err == nil && info.Size() > 0 {
			log.Printf("[Backup] cross-db restore: migrating SQLite data into %s...", dbType)
			if err := s.migrateSQLiteToTarget(ctx, sqlitePath, dbType, dsn); err != nil {
				return fmt.Errorf("cross-db migrate sqlite to %s: %w", dbType, err)
			}
			log.Printf("[Backup] cross-db restore: SQLite data migrated to %s successfully", dbType)
		}
	default:
		os.Remove(filepath.Join(s.dataDir, "sqlite", "app.db-wal"))
		os.Remove(filepath.Join(s.dataDir, "sqlite", "app.db-shm"))
		os.RemoveAll(filepath.Join(s.dataDir, "db_dump"))
	}
	return nil
}

// crossDBDumpRestore handles cross-database SQL dump restore by loading the
// dump into a temporary SQLite database and then migrating to the target DB.
// This is used when the SQL dump format doesn't match the target DB type
// (e.g. MySQL dump restored on PostgreSQL).
func (s *Service) crossDBDumpRestore(ctx context.Context, targetDBType, targetDSN string) error {
	matches, _ := filepath.Glob(filepath.Join(s.dataDir, "db_dump", "*.sql"))
	if len(matches) == 0 {
		return fmt.Errorf("no SQL dump files found for cross-db restore")
	}

	// Create a temporary SQLite database
	tmpSQLite := filepath.Join(s.dataDir, "db_dump", "_cross_db_tmp.sqlite")
	os.Remove(tmpSQLite)
	defer os.Remove(tmpSQLite)

	log.Printf("[Backup] cross-db: creating temporary SQLite database for SQL import...")
	srcCfg := db.Config{
		Type: db.DBTypeSQLite,
		DSN:  tmpSQLite,
	}
	srcDB, srcCleanup, err := db.OpenDBPublic(srcCfg)
	if err != nil {
		return fmt.Errorf("create temp sqlite: %w", err)
	}
	defer srcCleanup()

	// Run AutoMigrate to create all tables in the temporary SQLite database
	if err := db.AutoMigrateOnDB(srcDB); err != nil {
		return fmt.Errorf("auto-migrate temp sqlite: %w", err)
	}

	// Execute INSERT statements from each SQL dump file on the SQLite database
	for _, m := range matches {
		content, err := os.ReadFile(m)
		if err != nil {
			return fmt.Errorf("read db dump %s: %w", m, err)
		}

		// Parse and execute only INSERT statements (skip MySQL/PG-specific DDL)
		stmts := splitSQLStatements(string(content))
		insertCount := 0
		for _, stmt := range stmts {
			trimmed := strings.TrimSpace(stmt)
			if trimmed == "" || strings.HasPrefix(trimmed, "--") {
				continue
			}
			upper := strings.ToUpper(trimmed)
			if !strings.HasPrefix(upper, "INSERT") {
				continue
			}
			if err := srcDB.Exec(trimmed).Error; err != nil {
				log.Printf("[Backup] cross-db: skip statement (sqlite error): %v", err)
				continue
			}
			insertCount++
		}
		log.Printf("[Backup] cross-db: imported %d INSERT statements from %s into temp SQLite", insertCount, filepath.Base(m))
		os.Remove(m)
	}

	// Close the source SQLite connection so data is flushed to disk
	// before MigrateFromSQLite opens a new connection to read it.
	srcCleanup()
	srcCleanup = func() {} // prevent double-close in deferred call

	// Debug: check temp SQLite file
	if info, err := os.Stat(tmpSQLite); err == nil {
		log.Printf("[Backup] cross-db: temp SQLite file size: %d bytes", info.Size())
	} else {
		log.Printf("[Backup] cross-db: WARNING temp SQLite file not found: %v", err)
	}

	// Now migrate from the temporary SQLite database to the target DB
	log.Printf("[Backup] cross-db: migrating from temp SQLite to %s...", targetDBType)
	dstCfg := db.Config{
		Type: targetDBType,
		DSN:  targetDSN,
	}
	tables, rows, err := db.MigrateFromSQLite(tmpSQLite, dstCfg)
	if err != nil {
		return fmt.Errorf("migrate temp sqlite to %s: %w", targetDBType, err)
	}
	log.Printf("[Backup] cross-db: migrated %d tables, %d rows from temp SQLite to %s", tables, rows, targetDBType)

	// Clean up db_dump directory (temp SQLite file is removed by defer)
	os.RemoveAll(filepath.Join(s.dataDir, "db_dump"))
	return nil
}

// migrateSQLiteToTarget opens a SQLite database file and migrates all its
// data into the target MySQL/PostgreSQL database using db.MigrateFromSQLite.
func (s *Service) migrateSQLiteToTarget(ctx context.Context, sqlitePath, targetDBType, targetDSN string) error {
	// Construct target Config directly from the type and DSN.
	// Note: StartupConfig() returns the already-stripped DSN (without the
	// "mysql://" or "postgres://" prefix), so we must NOT use ParseDSN here
	// (it would fail to detect the type and default to SQLite).
	dstCfg := db.Config{
		Type: targetDBType,
		DSN:  targetDSN,
	}

	// Run the migration from SQLite to target DB
	tables, rows, err := db.MigrateFromSQLite(sqlitePath, dstCfg)
	if err != nil {
		return err
	}
	log.Printf("[Backup] cross-db migrate: copied %d tables, %d rows from SQLite to %s", tables, rows, targetDBType)
	return nil
}

// RemoteRestore downloads a backup file from remote storage and restores it.
// The backup file is downloaded to a temporary location, restored, then deleted.
// This supports the "backup to remote only, restore from remote" workflow.
func (s *Service) RemoteRestore(ctx context.Context, storageID int64, filename string) error {
	storage, err := s.GetStorage(ctx, storageID)
	if err != nil {
		return fmt.Errorf("get storage config: %w", err)
	}

	tmpPath := filepath.Join(s.dataDir, "backup", "remote_restore_tmp.tar.gz")
	os.MkdirAll(filepath.Dir(tmpPath), 0750)
	out, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer os.Remove(tmpPath)

	log.Printf("[Backup] downloading %s from %s storage...", filename, storage.Type)
	switch storage.Type {
	case "s3":
		var cfg S3Config
		if err := ParseStorageConfig(storage.Config, &cfg); err != nil {
			out.Close()
			return err
		}
		if err := s.downloadFromS3(ctx, cfg, filename, out); err != nil {
			out.Close()
			return err
		}
	case "ftp":
		var cfg FTPConfig
		if err := ParseStorageConfig(storage.Config, &cfg); err != nil {
			out.Close()
			return err
		}
		if err := s.downloadFromFTP(ctx, cfg, filename, out); err != nil {
			out.Close()
			return err
		}
	case "webdav":
		var cfg WebDAVConfig
		if err := ParseStorageConfig(storage.Config, &cfg); err != nil {
			out.Close()
			return err
		}
		if err := s.downloadFromWebDAV(ctx, cfg, filename, out); err != nil {
			out.Close()
			return err
		}
	default:
		out.Close()
		return fmt.Errorf("unsupported storage type: %s", storage.Type)
	}
	out.Close()

	info, _ := os.Stat(tmpPath)
	log.Printf("[Backup] downloaded %s (%d bytes), restoring...", filename, info.Size())
	return s.restoreFromArchive(ctx, tmpPath)
}

// GetRestoreStatus returns the current async restore status, or nil if none.
func (s *Service) GetRestoreStatus() *RestoreStatus {
	s.restoreMu.Lock()
	defer s.restoreMu.Unlock()
	return s.restoreStatus
}

// startAsyncRestore launches a goroutine that downloads (if needed) and restores.
// It returns immediately with a RestoreStatus. During restore, maintenance mode
// is auto-enabled to block all other API traffic.
func (s *Service) startAsyncRestore(downloadFn func() (string, error), restoreFn func(ctx context.Context, archivePath string) error) *RestoreStatus {
	s.restoreMu.Lock()
	s.nextRestoreID++
	st := &RestoreStatus{
		ID:        s.nextRestoreID,
		Status:    "pending",
		StartedAt: time.Now(),
	}
	s.restoreStatus = st
	s.restoreMu.Unlock()

	// Enable maintenance mode for the entire process
	middleware.SetMaintenance(true, "System restore in progress. Please try again later.", "restore")
	log.Printf("[Backup] maintenance mode enabled for restore #%d", st.ID)

	go func() {
		// Always disable maintenance when done (success or error)
		defer func() {
			middleware.SetMaintenance(false, "", "")
			log.Printf("[Backup] maintenance mode disabled after restore #%d", st.ID)
		}()

		var archivePath string
		if downloadFn != nil {
			s.setRestoreStatus(st.ID, "downloading", "downloading from remote storage...")
			p, err := downloadFn()
			if err != nil {
				s.setRestoreStatus(st.ID, "error", err.Error())
				return
			}
			archivePath = p
		}

		s.setRestoreStatus(st.ID, "restoring", "restoring database from archive...")
		if err := restoreFn(context.Background(), archivePath); err != nil {
			s.setRestoreStatus(st.ID, "error", err.Error())
			return
		}

		s.setRestoreStatus(st.ID, "done", "restore completed successfully")
	}()

	return st
}

func (s *Service) setRestoreStatus(id int64, status, msg string) {
	s.restoreMu.Lock()
	defer s.restoreMu.Unlock()
	if s.restoreStatus != nil && s.restoreStatus.ID == id {
		s.restoreStatus.Status = status
		s.restoreStatus.Message = msg
		if status == "done" || status == "error" {
			s.restoreStatus.DoneAt = time.Now()
		}
	}
}

// RemoteRestoreAsync downloads a backup from remote storage and restores it asynchronously.
func (s *Service) RemoteRestoreAsync(ctx context.Context, storageID int64, filename string) (*RestoreStatus, error) {
	storage, err := s.GetStorage(ctx, storageID)
	if err != nil {
		return nil, fmt.Errorf("get storage config: %w", err)
	}

	// Use a detached context for the async download — the HTTP request
	// context (ctx) will be canceled as soon as the handler returns, but
	// the download runs in a background goroutine.
	dlCtx := context.Background()

	downloadFn := func() (string, error) {
		tmpPath := filepath.Join(s.dataDir, "backup", "remote_restore_tmp.tar.gz")
		os.MkdirAll(filepath.Dir(tmpPath), 0750)
		out, err := os.Create(tmpPath)
		if err != nil {
			return "", err
		}
		defer out.Close()

		log.Printf("[Backup] downloading %s from %s storage...", filename, storage.Type)
		switch storage.Type {
		case "s3":
			var cfg S3Config
			if err := ParseStorageConfig(storage.Config, &cfg); err != nil {
				return "", err
			}
			if err := s.downloadFromS3(dlCtx, cfg, filename, out); err != nil {
				return "", err
			}
		case "ftp":
			var cfg FTPConfig
			if err := ParseStorageConfig(storage.Config, &cfg); err != nil {
				return "", err
			}
			if err := s.downloadFromFTP(dlCtx, cfg, filename, out); err != nil {
				return "", err
			}
		case "webdav":
			var cfg WebDAVConfig
			if err := ParseStorageConfig(storage.Config, &cfg); err != nil {
				return "", err
			}
			if err := s.downloadFromWebDAV(dlCtx, cfg, filename, out); err != nil {
				return "", err
			}
		default:
			return "", fmt.Errorf("unsupported storage type: %s", storage.Type)
		}

		info, _ := os.Stat(tmpPath)
		log.Printf("[Backup] downloaded %s (%d bytes)", filename, info.Size())
		return tmpPath, nil
	}

	restoreFn := func(ctx context.Context, archivePath string) error {
		if archivePath != "" {
			defer os.Remove(archivePath)
		}
		return s.restoreFromArchive(ctx, archivePath)
	}

	return s.startAsyncRestore(downloadFn, restoreFn), nil
}

// RestoreBackupAsync restores from a local backup by ID asynchronously.
func (s *Service) RestoreBackupAsync(ctx context.Context, id int64) (*RestoreStatus, error) {
	rec, err := s.repo.GetRecordByID(ctx, id)
	if err != nil {
		return nil, err
	}
	archivePath := rec.FilePath

	restoreFn := func(ctx context.Context, _ string) error {
		return s.restoreFromArchive(ctx, archivePath)
	}

	return s.startAsyncRestore(nil, restoreFn), nil
}

// RestoreFromFileAsync restores from an uploaded file asynchronously.
func (s *Service) RestoreFromFileAsync(ctx context.Context, reader io.Reader) (*RestoreStatus, error) {
	tmpPath := filepath.Join(s.dataDir, "backup", "restore_tmp.tar.gz")
	os.MkdirAll(filepath.Dir(tmpPath), 0750)
	f, err := os.Create(tmpPath)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(f, reader); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return nil, err
	}
	f.Close()

	restoreFn := func(ctx context.Context, _ string) error {
		defer os.Remove(tmpPath)
		return s.restoreFromArchive(ctx, tmpPath)
	}

	return s.startAsyncRestore(nil, restoreFn), nil
}

// ListRemoteFiles lists backup files available on remote storage.
func (s *Service) ListRemoteFiles(ctx context.Context, storageID int64) ([]RemoteFile, error) {
	storage, err := s.GetStorage(ctx, storageID)
	if err != nil {
		return nil, fmt.Errorf("get storage config: %w", err)
	}
	switch storage.Type {
	case "s3":
		var cfg S3Config
		if err := ParseStorageConfig(storage.Config, &cfg); err != nil {
			return nil, err
		}
		return s.listS3Files(ctx, cfg)
	case "ftp":
		var cfg FTPConfig
		if err := ParseStorageConfig(storage.Config, &cfg); err != nil {
			return nil, err
		}
		return s.listFTPFiles(ctx, cfg)
	case "webdav":
		var cfg WebDAVConfig
		if err := ParseStorageConfig(storage.Config, &cfg); err != nil {
			return nil, err
		}
		return s.listWebDAVFiles(ctx, cfg)
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", storage.Type)
	}
}

// RemoteFile represents a file on remote storage.
type RemoteFile struct {
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	Modified string `json:"modified"`
}

// --- S3 download/list ---

func (s *Service) downloadFromS3(ctx context.Context, cfg S3Config, key string, dest *os.File) error {
	var opts []func(*config.LoadOptions) error
	if cfg.AccessKey != "" && cfg.SecretKey != "" {
		opts = append(opts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		))
	}
	if cfg.Region != "" {
		opts = append(opts, config.WithRegion(cfg.Region))
	}
	awsCfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return fmt.Errorf("load AWS config: %w", err)
	}
	s3Opts := []func(*s3.Options){}
	if cfg.Endpoint != "" {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true
		})
	}
	s3Client := s3.NewFromConfig(awsCfg, s3Opts...)

	result, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("S3 download: %w", err)
	}
	defer result.Body.Close()
	if _, err := io.Copy(dest, result.Body); err != nil {
		return fmt.Errorf("S3 download copy: %w", err)
	}
	return nil
}

func (s *Service) listS3Files(ctx context.Context, cfg S3Config) ([]RemoteFile, error) {
	var opts []func(*config.LoadOptions) error
	if cfg.AccessKey != "" && cfg.SecretKey != "" {
		opts = append(opts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		))
	}
	if cfg.Region != "" {
		opts = append(opts, config.WithRegion(cfg.Region))
	}
	awsCfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}
	s3Opts := []func(*s3.Options){}
	if cfg.Endpoint != "" {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true
		})
	}
	s3Client := s3.NewFromConfig(awsCfg, s3Opts...)

	result, err := s3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(cfg.Bucket),
	})
	if err != nil {
		return nil, fmt.Errorf("S3 list: %w", err)
	}
	var files []RemoteFile
	for _, obj := range result.Contents {
		name := aws.ToString(obj.Key)
		if strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".sql") || strings.HasSuffix(name, ".db") {
			files = append(files, RemoteFile{
				Name:     name,
				Size:     aws.ToInt64(obj.Size),
				Modified: obj.LastModified.Format(time.RFC3339),
			})
		}
	}
	return files, nil
}

// --- FTP download/list ---

func (s *Service) downloadFromFTP(ctx context.Context, cfg FTPConfig, filename string, dest *os.File) error {
	port := cfg.Port
	if port == 0 {
		port = 21
	}
	addr := fmt.Sprintf("%s:%d", cfg.Host, port)
	conn, err := ftp.Dial(addr, ftp.DialWithTimeout(30*time.Second))
	if err != nil {
		return fmt.Errorf("FTP connect: %w", err)
	}
	defer conn.Quit()
	if err := conn.Login(cfg.Username, cfg.Password); err != nil {
		return fmt.Errorf("FTP login: %w", err)
	}
	remotePath := cfg.Path
	if remotePath == "" {
		remotePath = "/"
	}
	fullPath := strings.TrimRight(remotePath, "/") + "/" + filename
	resp, err := conn.Retr(fullPath)
	if err != nil {
		return fmt.Errorf("FTP download: %w", err)
	}
	defer resp.Close()
	if _, err := io.Copy(dest, resp); err != nil {
		return fmt.Errorf("FTP download copy: %w", err)
	}
	return nil
}

func (s *Service) listFTPFiles(ctx context.Context, cfg FTPConfig) ([]RemoteFile, error) {
	port := cfg.Port
	if port == 0 {
		port = 21
	}
	addr := fmt.Sprintf("%s:%d", cfg.Host, port)
	conn, err := ftp.Dial(addr, ftp.DialWithTimeout(10*time.Second))
	if err != nil {
		return nil, fmt.Errorf("FTP connect: %w", err)
	}
	defer conn.Quit()
	if err := conn.Login(cfg.Username, cfg.Password); err != nil {
		return nil, fmt.Errorf("FTP login: %w", err)
	}
	remotePath := cfg.Path
	if remotePath == "" {
		remotePath = "/"
	}
	entries, err := conn.List(remotePath)
	if err != nil {
		return nil, fmt.Errorf("FTP list: %w", err)
	}
	var files []RemoteFile
	for _, e := range entries {
		if strings.HasSuffix(e.Name, ".tar.gz") || strings.HasSuffix(e.Name, ".sql") || strings.HasSuffix(e.Name, ".db") {
			files = append(files, RemoteFile{
				Name:     e.Name,
				Size:     int64(e.Size),
				Modified: e.Time.Format(time.RFC3339),
			})
		}
	}
	return files, nil
}

// --- WebDAV download/list ---

func (s *Service) downloadFromWebDAV(ctx context.Context, cfg WebDAVConfig, filename string, dest *os.File) error {
	webdavURL := strings.TrimRight(cfg.URL, "/") + "/" + filename
	req, err := http.NewRequestWithContext(ctx, "GET", webdavURL, nil)
	if err != nil {
		return err
	}
	if cfg.Username != "" {
		req.SetBasicAuth(cfg.Username, cfg.Password)
	}
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("WebDAV download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("WebDAV download failed: HTTP %d", resp.StatusCode)
	}
	if _, err := io.Copy(dest, resp.Body); err != nil {
		return fmt.Errorf("WebDAV download copy: %w", err)
	}
	return nil
}

func (s *Service) listWebDAVFiles(ctx context.Context, cfg WebDAVConfig) ([]RemoteFile, error) {
	webdavURL := strings.TrimRight(cfg.URL, "/") + "/"
	req, err := http.NewRequestWithContext(ctx, "PROPFIND", webdavURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Depth", "1")
	req.Header.Set("Content-Type", "application/xml")
	if cfg.Username != "" {
		req.SetBasicAuth(cfg.Username, cfg.Password)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("WebDAV PROPFIND: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("WebDAV PROPFIND failed: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	// Parse href entries from the WebDAV XML response
	var files []RemoteFile
	hrefRe := regexp.MustCompile(`<[^:]*:href>([^<]+)</[^:]*:href>`)
	matches := hrefRe.FindAllStringSubmatch(string(body), -1)
	seen := map[string]bool{}
	for _, m := range matches {
		href := m[1]
		name := filepath.Base(href)
		// Unescape WebDAV URL encoding
		name, _ = url.QueryUnescape(name)
		if name == "" || name == "/" || seen[name] {
			continue
		}
		seen[name] = true
		if strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".sql") || strings.HasSuffix(name, ".db") {
			files = append(files, RemoteFile{Name: name})
		}
	}
	return files, nil
}

// GetBackupFile returns the file path for download.
func (s *Service) GetBackupFile(ctx context.Context, id int64) (string, string, error) {
	rec, err := s.repo.GetRecordByID(ctx, id)
	if err != nil {
		return "", "", err
	}
	return rec.FilePath, rec.Name + ".tar.gz", nil
}

// --- Schedule ---

func (s *Service) ListSchedules(ctx context.Context) ([]entity.BackupSchedule, error) {
	return s.repo.ListSchedules(ctx)
}

func (s *Service) CreateSchedule(ctx context.Context, sched *entity.BackupSchedule) error {
	return s.repo.CreateSchedule(ctx, sched)
}

func (s *Service) UpdateSchedule(ctx context.Context, sched *entity.BackupSchedule) error {
	return s.repo.UpdateSchedule(ctx, sched)
}

func (s *Service) DeleteSchedule(ctx context.Context, id int64) error {
	return s.repo.DeleteSchedule(ctx, id)
}

// --- Storage ---

func (s *Service) ListStorages(ctx context.Context) ([]entity.BackupStorage, error) {
	return s.repo.ListStorages(ctx)
}

func (s *Service) CreateStorage(ctx context.Context, storage *entity.BackupStorage) error {
	return s.repo.CreateStorage(ctx, storage)
}

func (s *Service) UpdateStorage(ctx context.Context, storage *entity.BackupStorage) error {
	return s.repo.UpdateStorage(ctx, storage)
}

func (s *Service) DeleteStorage(ctx context.Context, id int64) error {
	return s.repo.DeleteStorage(ctx, id)
}

// UploadToS3 uploads a backup to S3-compatible storage.
func (s *Service) UploadToS3(ctx context.Context, backupID int64, cfg S3Config) error {
	rec, err := s.repo.GetRecordByID(ctx, backupID)
	if err != nil {
		return err
	}

	// Open the backup file
	f, err := os.Open(rec.FilePath)
	if err != nil {
		return fmt.Errorf("open backup file: %w", err)
	}
	defer f.Close()

	// Get file info for content length
	fileInfo, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat backup file: %w", err)
	}

	// Build S3 client options
	var opts []func(*config.LoadOptions) error

	// Set credentials if provided
	if cfg.AccessKey != "" && cfg.SecretKey != "" {
		opts = append(opts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		))
	}

	// Set region if provided
	if cfg.Region != "" {
		opts = append(opts, config.WithRegion(cfg.Region))
	}

	// Load AWS config
	awsCfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return fmt.Errorf("load AWS config: %w", err)
	}

	// Create S3 client options
	s3Opts := []func(*s3.Options){}

	// Set custom endpoint if provided (for S3-compatible services like MinIO)
	if cfg.Endpoint != "" {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true
		})
	}

	// Create S3 client
	s3Client := s3.NewFromConfig(awsCfg, s3Opts...)

	// Build the object key
	key := filepath.Base(rec.FilePath)

	// Upload the file
	_, err = s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(cfg.Bucket),
		Key:           aws.String(key),
		Body:          f,
		ContentLength: aws.Int64(fileInfo.Size()),
		ContentType:   aws.String("application/gzip"),
	})
	if err != nil {
		return fmt.Errorf("S3 upload: %w", err)
	}

	return nil
}

// UploadToWebDAV uploads a backup to WebDAV storage.
func (s *Service) UploadToWebDAV(ctx context.Context, backupID int64, config WebDAVConfig) error {
	rec, err := s.repo.GetRecordByID(ctx, backupID)
	if err != nil {
		return err
	}

	f, err := os.Open(rec.FilePath)
	if err != nil {
		return err
	}
	defer f.Close()

	url := strings.TrimRight(config.URL, "/") + "/" + filepath.Base(rec.FilePath)
	req, err := http.NewRequestWithContext(ctx, "PUT", url, f)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/gzip")
	if config.Username != "" {
		req.SetBasicAuth(config.Username, config.Password)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("WebDAV upload failed: HTTP %d", resp.StatusCode)
	}
	return nil
}

// S3Config contains S3 storage configuration.
type S3Config struct {
	Endpoint  string `json:"endpoint"`
	Bucket    string `json:"bucket"`
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
	Region    string `json:"region"`
}

// WebDAVConfig contains WebDAV storage configuration.
type WebDAVConfig struct {
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// FTPConfig contains FTP storage configuration.
type FTPConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	Path     string `json:"path"` // Remote directory path
}

// UploadToFTP uploads a backup to FTP storage.
func (s *Service) UploadToFTP(ctx context.Context, backupID int64, config FTPConfig) error {
	rec, err := s.repo.GetRecordByID(ctx, backupID)
	if err != nil {
		return err
	}

	// Set default port if not specified
	port := config.Port
	if port == 0 {
		port = 21
	}

	// Connect to FTP server
	addr := fmt.Sprintf("%s:%d", config.Host, port)
	conn, err := ftp.Dial(addr, ftp.DialWithTimeout(10*time.Second))
	if err != nil {
		return fmt.Errorf("FTP connect: %w", err)
	}
	defer conn.Quit()

	// Login
	if err := conn.Login(config.Username, config.Password); err != nil {
		return fmt.Errorf("FTP login: %w", err)
	}

	// Set default path if not specified
	remotePath := config.Path
	if remotePath == "" {
		remotePath = "/"
	}

	// Create remote directory if it doesn't exist
	if remotePath != "/" {
		if err := conn.MakeDir(remotePath); err != nil {
			// Ignore error if directory already exists
		}
	}

	// Open the backup file
	f, err := os.Open(rec.FilePath)
	if err != nil {
		return fmt.Errorf("open backup file: %w", err)
	}
	defer f.Close()

	// Build remote file path
	remoteFile := strings.TrimRight(remotePath, "/") + "/" + filepath.Base(rec.FilePath)

	// Upload file
	if err := conn.Stor(remoteFile, f); err != nil {
		return fmt.Errorf("FTP upload: %w", err)
	}

	return nil
}

// ParseStorageConfig parses storage config JSON.
func ParseStorageConfig(configJSON string, target interface{}) error {
	if configJSON == "" {
		return nil
	}
	return json.Unmarshal([]byte(configJSON), target)
}

// (createTarGz and extraTarEntry removed — backups are now database-only via
// createDumpOnlyTarGz, so walking the data directory is no longer needed.)

// addFileToTar writes a single file into the tar writer with the given archive path.
func (s *Service) addFileToTar(tw *tar.Writer, path, rel string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	header.Name = filepath.ToSlash(rel)
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	_, err = io.Copy(tw, file)
	return err
}

// extractTarGz extracts a tar.gz archive to the destination directory.
func (s *Service) extractTarGz(archive, dest string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(dest, header.Name)
		cleanDest := filepath.Clean(dest)
		cleanTarget := filepath.Clean(target)

		// Security: use filepath.Rel to detect path traversal attempts.
		// HasPrefix alone is insufficient — "/data/backup-evil" would pass
		// a HasPrefix("/data/backup") check but escapes the destination.
		rel, err := filepath.Rel(cleanDest, cleanTarget)
		if err != nil || strings.HasPrefix(rel, "..") || rel == ".." {
			continue // skip path traversal
		}

		switch header.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(target, 0750)
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(target), 0750)
			f, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		case tar.TypeSymlink, tar.TypeLink:
			// Security: explicitly skip symlinks and hardlinks to prevent
			// arbitrary file writes/reads via crafted archives.
			continue
		}
	}
	return nil
}

// --- Database dump (pure Go, no external tools) ---

// DumpDatabase produces a consistent dump of the configured database into destPath.
//   - sqlite:   Online Backup API (sqlite3_backup_init/step/finish) — hot backup
//     that copies the database page-by-page without blocking writes for long.
//     Falls back to VACUUM INTO if the Online Backup API is unavailable.
//   - mysql:    SQL dump (DROP/CREATE + INSERT) via the existing gorm connection
//   - postgres: SQL dump (DROP + CREATE + INSERT) via pgx
//
// This deliberately avoids calling mysqldump/pg_dump, which may not be installed.
func (s *Service) DumpDatabase(ctx context.Context, dbType, dsn, destPath string) error {
	switch dbType {
	case db.DBTypeMySQL:
		return dumpMySQL(ctx, dsn, destPath)
	case db.DBTypePostgres:
		return dumpPostgres(ctx, dsn, destPath)
	case db.DBTypeSQLite, "":
		return s.dumpSQLite(ctx, destPath)
	default:
		return fmt.Errorf("unsupported database type for dump: %s", dbType)
	}
}

// dumpSQLite creates a consistent copy of the live SQLite database using the
// Online Backup API (sqlite3_backup_init/step/finish). This is superior to
// VACUUM INTO for large databases because:
//
//   - It copies page-by-page in small chunks (default 100 pages per step), so
//     write operations are only briefly blocked during each step rather than
//     for the entire backup duration.
//   - It provides progress information via PageCount() and Remaining().
//   - It produces a byte-for-byte identical copy of the source database schema
//     and data (VACUUM INTO also produces a consistent copy but defrags it,
//     which changes the page layout).
//
// The source database must be in WAL mode for the backup to be truly "hot"
// (concurrent writes continue during backup). This is already ensured by
// db.Init() which sets PRAGMA journal_mode=WAL at startup.
//
// If the Online Backup API is not available (e.g. driver mismatch), it falls
// back to VACUUM INTO automatically.
func (s *Service) dumpSQLite(ctx context.Context, destPath string) error {
	if s.db == nil {
		return fmt.Errorf("database connection not available for sqlite dump")
	}
	sqlDB, err := s.db.DB()
	if err != nil {
		return fmt.Errorf("get sql.DB: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0750); err != nil {
		return err
	}

	// WAL checkpoint: merge committed WAL data into the main db file so the
	// backup snapshot is fully flushed. Non-fatal if it fails (the Online
	// Backup API reads committed WAL pages on its own).
	if _, err := sqlDB.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		log.Printf("[Backup] sqlite wal_checkpoint(TRUNCATE) skipped: %v", err)
	}

	// Try the Online Backup API first. It requires a raw *sql.Conn to access
	// the underlying driver.Conn (which must implement sqlite.BackupConn).
	if err := s.dumpSQLiteOnlineBackup(ctx, sqlDB, destPath); err == nil {
		return nil
	} else {
		log.Printf("[Backup] online backup failed, falling back to VACUUM INTO: %v", err)
	}

	// Fallback: VACUUM INTO (atomic, consistent, but blocks writes for the
	// entire duration — acceptable for small-to-medium databases).
	safe := strings.ReplaceAll(destPath, "'", "''")
	if _, err := sqlDB.Exec("VACUUM INTO '" + safe + "'"); err != nil {
		return fmt.Errorf("vacuum into failed: %w", err)
	}
	return nil
}

// dumpSQLiteOnlineBackup uses the SQLite Online Backup API to copy the live
// database to destPath page-by-page. This is the preferred method for large
// databases because it minimizes write blocking time.
//
// The backup is performed in steps of backupPageSize pages (default 100).
// Between steps, other writers can proceed, so the database remains usable
// throughout the backup. Progress is logged at 10% intervals.
//
// The destPath must be an absolute or relative file path. It is converted to
// a SQLite URI (file:...) for the backup API.
//
// The function returns nil on success, or an error if the Online Backup API
// is unavailable or the backup fails. The caller is expected to fall back to
// VACUUM INTO on error.
func (s *Service) dumpSQLiteOnlineBackup(ctx context.Context, sqlDB *sql.DB, destPath string) error {
	// Clean up any stale destination file so the backup starts fresh.
	os.Remove(destPath)
	os.Remove(destPath + "-wal")
	os.Remove(destPath + "-shm")

	// Convert destPath to a SQLite URI. The Online Backup API expects a URI
	// that can be opened by sqlite3_open (e.g. "file:/path/to/backup.db").
	// Use the absolute path to avoid ambiguity.
	absPath, err := filepath.Abs(destPath)
	if err != nil {
		absPath = destPath
	}
	dstURI := "file:" + absPath

	// Get a dedicated connection from the pool. This connection will be used
	// to initiate the backup. We must use a raw *sql.Conn (not *sql.DB)
	// because we need access to the underlying driver.Conn to call the
	// sqlite3_backup_* C functions.
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return fmt.Errorf("get connection for online backup: %w", err)
	}
	defer conn.Close()

	// Create the backup object. This calls sqlite3_backup_init, which opens
	// the destination database and sets up the backup. The source is the
	// database on the current connection.
	backup, err := sqlite.NewBackup(conn, dstURI)
	if err != nil {
		return fmt.Errorf("init online backup: %w", err)
	}

	// Drive the backup in small steps. Each step copies up to backupPageSize
	// pages (default 100, ~400KB at 4KB/page). Between steps, the database
	// lock is released, allowing concurrent writers to proceed.
	const backupPageSize = int32(100)

	// Note: PageCount() returns 0 until the first Step() has been called
	// (this is how the sqlite3_backup_* C API works). So we capture the total
	// page count after the first step and use it for progress logging.
	var lastProgressPct int = -1
	var totalPages int

	for {
		// Check if context is cancelled (backup may take a long time on
		// large databases).
		select {
		case <-ctx.Done():
			backup.Finish()
			os.Remove(destPath)
			return ctx.Err()
		default:
		}

		more, err := backup.Step(backupPageSize)
		if err != nil {
			backup.Finish()
			os.Remove(destPath)
			return fmt.Errorf("online backup step failed: %w", err)
		}

		// Capture total pages after the first step (PageCount returns 0
		// before any step has been executed).
		if totalPages == 0 {
			totalPages = backup.PageCount()
			if totalPages > 0 {
				log.Printf("[Backup] online backup started: %d pages (~%d MB)",
					totalPages, totalPages*4/1024)
			}
		}

		// Log progress at 10% intervals.
		if totalPages > 0 {
			remaining := backup.Remaining()
			done := totalPages - remaining
			pct := done * 100 / totalPages
			pct = pct / 10 * 10 // round down to nearest 10%
			if pct != lastProgressPct && pct < 100 {
				log.Printf("[Backup] online backup progress: %d%% (%d/%d pages)",
					pct, done, totalPages)
				lastProgressPct = pct
			}
		}

		if !more {
			// All pages copied.
			break
		}
	}

	// Finish the backup (calls sqlite3_backup_finish and closes the
	// destination connection). The destination file is now a complete,
	// consistent snapshot of the source database.
	if err := backup.Finish(); err != nil {
		os.Remove(destPath)
		return fmt.Errorf("online backup finish failed: %w", err)
	}

	// Verify the backup file exists and has non-zero size.
	info, err := os.Stat(destPath)
	if err != nil {
		return fmt.Errorf("online backup file not found after finish: %w", err)
	}
	if info.Size() == 0 {
		os.Remove(destPath)
		return fmt.Errorf("online backup file is empty")
	}

	log.Printf("[Backup] online backup completed: %s (%d bytes)", destPath, info.Size())
	return nil
}

// RestoreDatabase restores the configured database from a dump file.
//   - sqlite:   copies the .db backup copy over the live db file (caller restarts)
//   - mysql:    executes the .sql dump with multiStatements enabled
//   - postgres: executes the .sql dump via pgx (simple query protocol, multi-statement)
//
// Returns needRestart=true when the process must be restarted (sqlite only).
func (s *Service) RestoreDatabase(ctx context.Context, dbType, dsn, dumpPath string) (needRestart bool, err error) {
	content, err := os.ReadFile(dumpPath)
	if err != nil {
		return false, fmt.Errorf("read dump: %w", err)
	}
	switch dbType {
	case db.DBTypeMySQL:
		if dsn == "" {
			return false, fmt.Errorf("数据库 DSN 未配置（环境变量 CF_DB_DSN），无法还原 MySQL 备份")
		}
		if err := restoreMySQL(ctx, dsn, string(content)); err != nil {
			return false, err
		}
		return false, nil
	case db.DBTypePostgres:
		if dsn == "" {
			return false, fmt.Errorf("数据库 DSN 未配置（环境变量 CF_DB_DSN），无法还原 PostgreSQL 备份")
		}
		if err := restorePostgres(ctx, dsn, string(content)); err != nil {
			return false, err
		}
		return false, nil
	case db.DBTypeSQLite, "":
		dst := filepath.Join(s.dataDir, "sqlite", "app.db")
		if err := copyFile(dumpPath, dst); err != nil {
			return false, fmt.Errorf("copy sqlite dump: %w", err)
		}
		// Drop stale WAL/SHM so the restored snapshot is authoritative on next open.
		os.Remove(filepath.Join(s.dataDir, "sqlite", "app.db-wal"))
		os.Remove(filepath.Join(s.dataDir, "sqlite", "app.db-shm"))
		return true, nil
	default:
		return false, fmt.Errorf("unsupported database type for restore: %s", dbType)
	}
}

// resolveRestoreType decides which database a dump file should be restored into,
// validating it against the current environment DB configuration (read from the
// .env / CF_DB_* variables at startup). A .sql dump is restored using the env DB
// type (mysql vs postgres cannot be told from the file alone); a .db dump is SQLite
// only. This prevents silently importing a dump into the wrong database.
func ResolveRestoreType(dumpPath, envType string) (string, error) {
	if strings.HasSuffix(dumpPath, ".sql") {
		if envType != db.DBTypeMySQL && envType != db.DBTypePostgres {
			return "", fmt.Errorf("当前运行数据库为 %s，无法直接还原 SQL 备份（该备份来自 MySQL/PostgreSQL）。如需更换数据库类型，请先在同一类型的数据库中还原此备份，再使用系统设置中的「数据库迁移」功能将数据迁移到当前数据库", dbTypeLabel(envType))
		}
		return envType, nil
	}
	// .db dump is a SQLite snapshot.
	if envType != "" && envType != db.DBTypeSQLite {
		return "", fmt.Errorf("当前运行数据库为 %s，无法直接还原 SQLite 备份。如需更换数据库类型，请先在 SQLite 数据库中还原此备份，再使用系统设置中的「数据库迁移」功能将数据迁移到当前数据库", dbTypeLabel(envType))
	}
	return db.DBTypeSQLite, nil
}

func dbTypeLabel(t string) string {
	switch t {
	case db.DBTypeMySQL:
		return "MySQL"
	case db.DBTypePostgres:
		return "PostgreSQL"
	case db.DBTypeSQLite, "":
		return "SQLite"
	}
	return t
}

// restoreMySQL imports a .sql dump by splitting it into individual statements
// and executing them in batches, similar to restorePostgres.
func restoreMySQL(ctx context.Context, dsn, content string) error {
	// GORM mysql driver expects raw DSN without "mysql://" prefix.
	connDSN := strings.TrimPrefix(dsn, "mysql://")
	if strings.Contains(connDSN, "?") {
		connDSN += "&multiStatements=true&maxAllowedPacket=268435456"
	} else {
		connDSN += "?multiStatements=true&maxAllowedPacket=268435456"
	}
	gdb, err := gorm.Open(mysql.Open(connDSN), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("connect mysql for restore: %w", err)
	}
	defer func() {
		if c, e := gdb.DB(); e == nil {
			_ = c.Close()
		}
	}()

	stmts := splitSQLStatements(content)
	log.Printf("[Backup] restoring mysql dump: %d statements, %d bytes", len(stmts), len(content))

	// Batch statements to avoid "packet for query is too large" errors.
	const batchSize = 500
	for start := 0; start < len(stmts); start += batchSize {
		end := start + batchSize
		if end > len(stmts) {
			end = len(stmts)
		}
		var batch strings.Builder
		for _, s := range stmts[start:end] {
			s = strings.TrimSpace(s)
			if s != "" {
				batch.WriteString(s)
				batch.WriteByte('\n')
			}
		}
		if batch.Len() == 0 {
			continue
		}
		if err := gdb.WithContext(ctx).Exec(batch.String()).Error; err != nil {
			// Fall back to one-by-one for better error reporting
			for j, s := range stmts[start:end] {
				s = strings.TrimSpace(s)
				if s == "" {
					continue
				}
				if err := gdb.WithContext(ctx).Exec(s).Error; err != nil {
					preview := s
					if len(preview) > 200 {
						preview = preview[:200] + "..."
					}
					return fmt.Errorf("execute mysql dump (statement %d/%d): %w\nSQL: %s", start+j+1, len(stmts), err, preview)
				}
			}
		}
		if start%(batchSize*20) == 0 {
			log.Printf("[Backup] mysql restore progress: %d/%d statements", start, len(stmts))
		}
	}
	log.Printf("[Backup] mysql restore progress: %d/%d statements (done)", len(stmts), len(stmts))
	return nil
}

// restorePostgres imports a .sql dump by splitting it into individual
// statements and executing each via pgx simple query protocol.
//
// The dump is split on semicolons that end a statement (respecting string
// literals and comments). This avoids message-size issues with very large
// dumps (100MB+) that fail when sent as a single query string.
//
// After importing the data, all SERIAL/identity sequences are resynced so that
// subsequent INSERTs don't hit duplicate-key errors.
func restorePostgres(ctx context.Context, dsn, content string) error {
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connect postgres for restore: %w", err)
	}
	defer conn.Close(ctx)

	stmts := splitSQLStatements(content)
	log.Printf("[Backup] restoring postgres dump: %d statements, %d bytes", len(stmts), len(content))

	// Batch statements into chunks and send each chunk as a single multi-statement
	// query. This is ~50x faster than executing one statement at a time because it
	// avoids per-statement network round-trips. 500 statements per batch is a good
	// balance between throughput and memory.
	const batchSize = 500
	for start := 0; start < len(stmts); start += batchSize {
		end := start + batchSize
		if end > len(stmts) {
			end = len(stmts)
		}
		var batch strings.Builder
		for _, s := range stmts[start:end] {
			s = strings.TrimSpace(s)
			if s != "" {
				batch.WriteString(s)
				batch.WriteByte('\n')
			}
		}
		if batch.Len() == 0 {
			continue
		}
		if _, err := conn.Exec(ctx, batch.String()); err != nil {
			// On batch failure, fall back to one-by-one for better error reporting
			for j, s := range stmts[start:end] {
				s = strings.TrimSpace(s)
				if s == "" {
					continue
				}
				if _, err := conn.Exec(ctx, s); err != nil {
					preview := s
					if len(preview) > 200 {
						preview = preview[:200] + "..."
					}
					return fmt.Errorf("execute postgres dump (statement %d/%d): %w\nSQL: %s", start+j+1, len(stmts), err, preview)
				}
			}
		}
		if start%(batchSize*20) == 0 {
			log.Printf("[Backup] restore progress: %d/%d statements", start, len(stmts))
		}
	}
	log.Printf("[Backup] restore progress: %d/%d statements (done)", len(stmts), len(stmts))

	if err := syncPostgresSequences(ctx, conn); err != nil {
		// Non-fatal: log but don't fail the entire restore. The data is
		// already imported; only future auto-generated IDs might conflict.
		log.Printf("[Backup] WARNING: sync postgres sequences: %v", err)
	}
	return nil
}

// splitSQLStatements splits a SQL dump into individual statements.
// It respects single-quoted strings, double-quoted identifiers, line comments
// (--), and block comments (/* */). A statement ends at a semicolon that is
// not inside a string or comment.
func splitSQLStatements(sql string) []string {
	var stmts []string
	var buf strings.Builder
	inSingle := false   // inside '...'
	inDouble := false   // inside "..."
	inLineComment := false
	inBlockComment := false

	for i := 0; i < len(sql); i++ {
		c := sql[i]

		if inLineComment {
			buf.WriteByte(c)
			if c == '\n' {
				inLineComment = false
			}
			continue
		}
		if inBlockComment {
			buf.WriteByte(c)
			if c == '*' && i+1 < len(sql) && sql[i+1] == '/' {
				buf.WriteByte(sql[i+1])
				i++
				inBlockComment = false
			}
			continue
		}
		if inSingle {
			buf.WriteByte(c)
			if c == '\'' {
				// Check for escaped single quote ''
				if i+1 < len(sql) && sql[i+1] == '\'' {
					buf.WriteByte(sql[i+1])
					i++
				} else {
					inSingle = false
				}
			}
			continue
		}
		if inDouble {
			buf.WriteByte(c)
			if c == '"' {
				inDouble = false
			}
			continue
		}

		// Not inside any string or comment
		if c == '-' && i+1 < len(sql) && sql[i+1] == '-' {
			inLineComment = true
			buf.WriteByte(c)
			buf.WriteByte(sql[i+1])
			i++
			continue
		}
		if c == '/' && i+1 < len(sql) && sql[i+1] == '*' {
			inBlockComment = true
			buf.WriteByte(c)
			buf.WriteByte(sql[i+1])
			i++
			continue
		}
		if c == '\'' {
			inSingle = true
			buf.WriteByte(c)
			continue
		}
		if c == '"' {
			inDouble = true
			buf.WriteByte(c)
			continue
		}
		if c == ';' {
			buf.WriteByte(c)
			stmts = append(stmts, buf.String())
			buf.Reset()
			continue
		}
		buf.WriteByte(c)
	}
	if buf.Len() > 0 {
		stmts = append(stmts, buf.String())
	}
	return stmts
}

// syncPostgresSequences resets every SERIAL/identity sequence to
// MAX(column)+1 so that nextval() returns a value higher than any existing row.
func syncPostgresSequences(ctx context.Context, conn *pgx.Conn) error {
	// Find all tables+columns that own a sequence (SERIAL or GENERATED ... AS IDENTITY).
	rows, err := conn.Query(ctx, `
		SELECT c.relname  AS table_name,
		       a.attname  AS column_name,
		       pg_get_serial_sequence(c.relname, a.attname) AS seq_name
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum > 0 AND NOT a.attisdropped
		WHERE n.nspname = 'public'
		  AND c.relkind = 'r'
		  AND pg_get_serial_sequence(c.relname, a.attname) IS NOT NULL
	`)
	if err != nil {
		return fmt.Errorf("query sequences: %w", err)
	}
	defer rows.Close()

	type seqInfo struct {
		table, column, seq string
	}
	var seqs []seqInfo
	for rows.Next() {
		var si seqInfo
		if err := rows.Scan(&si.table, &si.column, &si.seq); err != nil {
			return fmt.Errorf("scan sequence info: %w", err)
		}
		seqs = append(seqs, si)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, si := range seqs {
		// setval(seq, MAX(column)+1, false) → next nextval() returns MAX+1
		sql := fmt.Sprintf(
			`SELECT setval('%s', COALESCE((SELECT MAX(%s) FROM %s), 0) + 1, false)`,
			si.seq, quotePgIdent(si.column), quotePgIdent(si.table),
		)
		if _, err := conn.Exec(ctx, sql); err != nil {
			log.Printf("[Backup] skip sequence %s on %s.%s: %v", si.seq, si.table, si.column, err)
		}
	}
	log.Printf("[Backup] synced %d postgres sequences after restore", len(seqs))
	return nil
}

// copyFile copies src to dst, creating parent dirs as needed.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0750); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

// dumpMySQL writes a SQL dump using a gorm (MySQL) connection built from the DSN.
func dumpMySQL(ctx context.Context, dsn, destPath string) error {
	_, _, _, _, _, dbName, err := parseMySQLDSN(dsn)
	if err != nil {
		return err
	}

	// GORM mysql driver expects the raw Go MySQL DSN
	// (user:pass@tcp(host:port)/db?params) WITHOUT a "mysql://" prefix.
	// The StartupConfig already strips the prefix via ParseDSN, so dsn
	// here should be the raw DSN. Strip any residual prefix just in case.
	connDSN := strings.TrimPrefix(dsn, "mysql://")
	gdb, err := gorm.Open(mysql.Open(connDSN), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("connect mysql for dump: %w", err)
	}
	defer func() {
		if c, e := gdb.DB(); e == nil {
			_ = c.Close()
		}
	}()

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	var tables []string
	if err := gdb.WithContext(ctx).Raw("SELECT table_name FROM information_schema.tables WHERE table_schema = ? ORDER BY table_name", dbName).Scan(&tables).Error; err != nil {
		return fmt.Errorf("list mysql tables: %w", err)
	}

	if _, err := fmt.Fprintf(out, "-- MySQL dump (pure Go)\n-- Database: %s\nSET FOREIGN_KEY_CHECKS=0;\nSET sql_mode='';\n\n", dbName); err != nil {
		return err
	}

	for _, tbl := range tables {
		// SHOW CREATE TABLE -> (Table, Create Table)
		rows, err := gdb.WithContext(ctx).Raw("SHOW CREATE TABLE " + quoteMySQLIdent(tbl)).Rows()
		if err != nil {
			return fmt.Errorf("show create table %s: %w", tbl, err)
		}
		var create string
		for rows.Next() {
			var t, c string
			if err := rows.Scan(&t, &c); err != nil {
				rows.Close()
				return err
			}
			create = c
		}
		rows.Close()
		if create == "" {
			return fmt.Errorf("empty create table for %s", tbl)
		}
		if _, err := fmt.Fprintf(out, "DROP TABLE IF EXISTS %s;\n%s;\n\n", quoteMySQLIdent(tbl), create); err != nil {
			return err
		}

		var cols []string
		if err := gdb.WithContext(ctx).Raw("SELECT COLUMN_NAME FROM information_schema.columns WHERE table_schema = ? AND table_name = ? ORDER BY ORDINAL_POSITION", dbName, tbl).Scan(&cols).Error; err != nil {
			return fmt.Errorf("columns for %s: %w", tbl, err)
		}
		colList := ""
		for i, c := range cols {
			if i > 0 {
				colList += ","
			}
			colList += quoteMySQLIdent(c)
		}

		rows, err = gdb.WithContext(ctx).Raw("SELECT * FROM " + quoteMySQLIdent(tbl)).Rows()
		if err != nil {
			return fmt.Errorf("select from %s: %w", tbl, err)
		}

		// Use extended INSERT (like mysqldump --extended-insert): one INSERT
		// statement covers up to extBatch rows. This dramatically reduces dump
		// file size and speeds up restore (fewer statements to parse/execute).
		const extBatch = 1000
		rowBuf := make([]string, 0, extBatch)
		flush := func() error {
			if len(rowBuf) == 0 {
				return nil
			}
			_, err := fmt.Fprintf(out, "INSERT INTO %s (%s) VALUES %s;\n",
				quoteMySQLIdent(tbl), colList, strings.Join(rowBuf, ","))
			rowBuf = rowBuf[:0]
			return err
		}

		for rows.Next() {
			vals, err := rows.Columns()
			if err != nil {
				rows.Close()
				return err
			}
			scan := make([]interface{}, len(vals))
			ptrs := make([]interface{}, len(vals))
			for i := range scan {
				ptrs[i] = &scan[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				rows.Close()
				return err
			}
			parts := make([]string, len(vals))
			for i, v := range scan {
				parts[i] = mysqlLiteral(v)
			}
			rowBuf = append(rowBuf, "("+strings.Join(parts, ",")+")")
			if len(rowBuf) >= extBatch {
				if err := flush(); err != nil {
					rows.Close()
					return err
				}
			}
		}
		rows.Close()
		if err := flush(); err != nil { // flush remaining rows
			return err
		}
		if _, err := fmt.Fprintln(out); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprint(out, "SET FOREIGN_KEY_CHECKS=1;\n"); err != nil {
		return err
	}
	return nil
}

// dumpPostgres writes a SQL dump using pgx COPY ... TO STDOUT.
func dumpPostgres(ctx context.Context, dsn, destPath string) error {
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connect postgres for dump: %w", err)
	}
	defer conn.Close(ctx)

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := fmt.Fprintf(out, "-- PostgreSQL dump (pure Go)\n-- DSN: %s\n\n", maskDSN(dsn)); err != nil {
		return err
	}

	var tables []string
	if err := conn.QueryRow(ctx, "SELECT string_agg(table_name,'\n' ORDER BY table_name) FROM information_schema.tables WHERE table_schema='public' AND table_type='BASE TABLE'").Scan(&tables); err != nil && err.Error() != "no rows in result set" {
		// QueryRow with string_agg returns single row (possibly NULL); treat NULL as empty
	}
	if len(tables) == 0 {
		// retry via slice scan in case string_agg returned NULL
		rows, rerr := conn.Query(ctx, "SELECT table_name FROM information_schema.tables WHERE table_schema='public' AND table_type='BASE TABLE' ORDER BY table_name")
		if rerr != nil {
			return fmt.Errorf("list pg tables: %w", rerr)
		}
		for rows.Next() {
			var t string
			if err := rows.Scan(&t); err != nil {
				rows.Close()
				return err
			}
			tables = append(tables, t)
		}
		rows.Close()
	}

	for _, tbl := range tables {
		// Use TRUNCATE instead of DROP+CREATE. The table schema is already
		// created by AutoMigrate on startup, so we only need to clear existing
		// data before inserting. This avoids needing pg_get_tabledef (which is
		// not a built-in PostgreSQL function).
		if _, err := fmt.Fprintf(out, "TRUNCATE TABLE %s CASCADE;\n", quotePgIdent(tbl)); err != nil {
			return err
		}

		// Data as INSERT statements so restore needs no COPY/psql (pure Go).
		// Use extended INSERT (multi-row VALUES) like mysqldump to reduce dump
		// size and speed up restore.
		rows, err := conn.Query(ctx, "SELECT * FROM "+quotePgIdent(tbl))
		if err != nil {
			return fmt.Errorf("select %s: %w", tbl, err)
		}
		cols := rows.FieldDescriptions()
		colNames := make([]string, len(cols))
		for i, c := range cols {
			colNames[i] = quotePgIdent(c.Name)
		}
		colList := strings.Join(colNames, ",")

		const extBatch = 1000
		rowBuf := make([]string, 0, extBatch)
		flush := func() error {
			if len(rowBuf) == 0 {
				return nil
			}
			_, err := fmt.Fprintf(out, "INSERT INTO %s (%s) VALUES %s;\n",
				quotePgIdent(tbl), colList, strings.Join(rowBuf, ","))
			rowBuf = rowBuf[:0]
			return err
		}

		for rows.Next() {
			vals := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				rows.Close()
				return fmt.Errorf("scan %s: %w", tbl, err)
			}
			parts := make([]string, len(vals))
			for i, v := range vals {
				parts[i] = pgValueLiteral(v)
			}
			rowBuf = append(rowBuf, "("+strings.Join(parts, ",")+")")
			if len(rowBuf) >= extBatch {
				if err := flush(); err != nil {
					rows.Close()
					return err
				}
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
		if err := flush(); err != nil { // flush remaining rows
			return err
		}
		if _, err := fmt.Fprintln(out); err != nil {
			return err
		}
	}

	return nil
}

// pgValueLiteral formats a Go value (as returned by pgx Scan) as a PostgreSQL
// SQL literal suitable for an INSERT statement.
func pgValueLiteral(v any) string {
	if v == nil {
		return "NULL"
	}
	switch val := v.(type) {
	case bool:
		if val {
			return "true"
		}
		return "false"
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", val)
	case float32, float64:
		return fmt.Sprintf("%v", val)
	case string:
		return "'" + strings.ReplaceAll(val, "'", "''") + "'"
	case []byte:
		if len(val) == 0 {
			return "NULL"
		}
		// pgx may return []byte for text/varchar columns. Treat as string
		// for cross-db compatibility (E'\\x...' hex format corrupts text
		// when imported into SQLite during cross-db restore).
		s := string(val)
		return "'" + strings.ReplaceAll(s, "'", "''") + "'"
	case time.Time:
		return "'" + val.Format("2006-01-02 15:04:05.999999999-07") + "'"
	case net.IP:
		return "'" + val.String() + "'"
	default:
		// Fallback: use JSON marshalling and treat as text
		b, _ := json.Marshal(val)
		return "'" + strings.ReplaceAll(string(b), "'", "''") + "'"
	}
}

// mysqlLiteral formats a Go value as a MySQL literal for INSERT statements.
func mysqlLiteral(v interface{}) string {
	if v == nil {
		return "NULL"
	}
	switch val := v.(type) {
	case []byte:
		// MySQL driver returns []byte for varchar/text/longtext/blob columns.
		// Treat as string — this is safe for text columns and produces
		// correct cross-db compatible SQL (X'hex' would corrupt text data
		// when imported into SQLite/PostgreSQL during cross-db restore).
		s := string(val)
		return "'" + strings.ReplaceAll(strings.ReplaceAll(s, "\\", "\\\\"), "'", "''") + "'"
	case string:
		return "'" + strings.ReplaceAll(strings.ReplaceAll(val, "\\", "\\\\"), "'", "''") + "'"
	case int, int32, int64, uint, uint32, uint64:
		return fmt.Sprintf("%d", val)
	case float32, float64:
		return fmt.Sprintf("%v", val)
	case bool:
		if val {
			return "1"
		}
		return "0"
	case time.Time:
		return "'" + val.Format("2006-01-02 15:04:05.999999999") + "'"
	case net.IP:
		return "'" + val.String() + "'"
	default:
		return "'" + strings.ReplaceAll(strings.ReplaceAll(fmt.Sprintf("%v", val), "\\", "\\\\"), "'", "''") + "'"
	}
}

func hexEncode(b []byte) string {
	const hexd = "0123456789ABCDEF"
	out := make([]byte, len(b)*2)
	for i, c := range b {
		out[i*2] = hexd[c>>4]
		out[i*2+1] = hexd[c&0x0F]
	}
	return string(out)
}

func quoteMySQLIdent(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

func quotePgIdent(name string) string {
	return "\"" + strings.ReplaceAll(name, "\"", "\"\"") + "\""
}

// parseMySQLDSN parses a Go MySQL driver DSN (user:pass@tcp(host:port)/db?params)
// into its components. Returns dbName as the last value.
func parseMySQLDSN(dsn string) (user, password, host, port, socket, dbName string, err error) {
	at := strings.LastIndex(dsn, "@")
	if at < 0 {
		return "", "", "", "", "", "", fmt.Errorf("invalid mysql dsn (missing '@'): %s", dsn)
	}
	cred := dsn[:at]
	rest := dsn[at+1:]

	if i := strings.Index(cred, ":"); i >= 0 {
		user = cred[:i]
		password = cred[i+1:]
	} else {
		user = cred
	}

	pOpen := strings.Index(rest, "(")
	pClose := strings.Index(rest, ")")
	if pOpen < 0 || pClose < 0 || pClose < pOpen {
		return "", "", "", "", "", "", fmt.Errorf("unsupported mysql dsn protocol: %s", rest)
	}
	addr := rest[pOpen+1 : pClose]
	after := rest[pClose+1:]
	slash := strings.Index(after, "/")
	if slash < 0 {
		return "", "", "", "", "", "", fmt.Errorf("mysql dsn missing database name: %s", dsn)
	}
	dbAndParams := after[slash+1:]
	if q := strings.Index(dbAndParams, "?"); q >= 0 {
		dbName = dbAndParams[:q]
	} else {
		dbName = dbAndParams
	}

	if strings.HasPrefix(addr, "/") {
		socket = addr
	} else if h, p, e := splitHostPort(addr); e == nil {
		host = h
		port = p
	} else {
		host = addr
	}
	return user, password, host, port, socket, dbName, nil
}

func splitHostPort(addr string) (string, string, error) {
	colon := strings.LastIndex(addr, ":")
	if colon < 0 {
		return "", "", fmt.Errorf("no port")
	}
	return addr[:colon], addr[colon+1:], nil
}

// dbNameFromDSN returns a safe filename component for the dump.
func dbNameFromDSN(dbType, dsn string) string {
	switch dbType {
	case db.DBTypeMySQL:
		_, _, _, _, _, dbName, _ := parseMySQLDSN(dsn)
		if dbName == "" {
			return "mysql"
		}
		return dbName
	case db.DBTypePostgres:
		u := dsn
		if i := strings.Index(u, "?"); i >= 0 {
			u = u[:i]
		}
		if i := strings.LastIndex(u, "/"); i >= 0 {
			u = u[i+1:]
		}
		if u == "" {
			return "postgres"
		}
		return u
	}
	return "database"
}

func maskDSN(dsn string) string {
	if i := strings.Index(dsn, "@"); i >= 0 {
		return dsn[:i] + "@***"
	}
	return dsn
}

// --- Scheduler ---

// BackupScheduler handles scheduled backup execution.
type BackupScheduler struct {
	svc      *Service
	stopCh   chan struct{}
	running  bool
	interval time.Duration
}

// NewBackupScheduler creates a new backup scheduler with the given check interval.
func NewBackupScheduler(svc *Service, checkInterval time.Duration) *BackupScheduler {
	if checkInterval <= 0 {
		checkInterval = 1 * time.Minute
	}
	return &BackupScheduler{
		svc:      svc,
		stopCh:   make(chan struct{}),
		interval: checkInterval,
	}
}

// Start starts the backup scheduler.
func (s *BackupScheduler) Start() {
	if s.running {
		return
	}
	s.running = true
	go s.run()
	log.Printf("[BackupScheduler] Started")
}

// Stop stops the backup scheduler.
func (s *BackupScheduler) Stop() {
	if !s.running {
		return
	}
	close(s.stopCh)
	s.running = false
	log.Printf("[BackupScheduler] Stopped")
}

func (s *BackupScheduler) run() {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.checkAndRun()
		}
	}
}

func (s *BackupScheduler) checkAndRun() {
	ctx := context.Background()
	schedules, err := s.svc.ListSchedules(ctx)
	if err != nil {
		log.Printf("[BackupScheduler] List schedules error: %v", err)
		return
	}

	now := time.Now()
	for _, sched := range schedules {
		if !sched.Enabled {
			continue
		}

		// Simple cron check: run if last run was more than 24 hours ago
		// In production, use a proper cron library
		if !sched.LastRunAt.IsZero() && now.Sub(sched.LastRunAt) < 24*time.Hour {
			continue
		}

		// Run the backup
		log.Printf("[BackupScheduler] Running scheduled backup: %s", sched.Name)
		s.runScheduledBackup(ctx, &sched)
	}
}

func (s *BackupScheduler) runScheduledBackup(ctx context.Context, sched *entity.BackupSchedule) {
	// Create backup
	name := fmt.Sprintf("scheduled_%s_%s", sched.Name, time.Now().Format("20060102_150405"))
	rec, err := s.svc.CreateBackup(ctx, name, "scheduled")
	if err != nil {
		log.Printf("[BackupScheduler] Create backup error: %v", err)
		return
	}

	// Upload to remote storage if configured
	if sched.StorageID != nil && *sched.StorageID > 0 {
		storage, err := s.svc.GetStorage(ctx, *sched.StorageID)
		if err != nil {
			log.Printf("[BackupScheduler] Get storage error: %v", err)
			return
		}

		if err := s.uploadToStorage(ctx, rec.ID, storage); err != nil {
			log.Printf("[BackupScheduler] Upload to %s error: %v", storage.Type, err)
			return
		}
		log.Printf("[BackupScheduler] Uploaded to %s storage: %s", storage.Type, storage.Name)

		// Optionally delete the local backup after successful upload
		if sched.DeleteLocalAfterUpload {
			if err := s.svc.DeleteLocalFile(ctx, rec.ID); err != nil {
				log.Printf("[BackupScheduler] Delete local backup error: %v", err)
			} else {
				log.Printf("[BackupScheduler] Local backup deleted after upload (remote_only): %s", name)
			}
		}
	}

	// Update last run time
	sched.LastRunAt = time.Now()
	if err := s.svc.UpdateSchedule(ctx, sched); err != nil {
		log.Printf("[BackupScheduler] Update schedule error: %v", err)
	}

	// Enforce retention policy: remove backups older than the schedule's RetainDays.
	if sched.RetainDays > 0 {
		if removed, rerr := s.svc.EnforceRetention(ctx, sched.RetainDays); rerr != nil {
			log.Printf("[BackupScheduler] Retention error: %v", rerr)
		} else if removed > 0 {
			log.Printf("[BackupScheduler] Retention: removed %d old backup(s) (retain_days=%d)", removed, sched.RetainDays)
		}
	}

	log.Printf("[BackupScheduler] Scheduled backup completed: %s", name)
}

func (s *BackupScheduler) uploadToStorage(ctx context.Context, backupID int64, storage *entity.BackupStorage) error {
	switch storage.Type {
	case "s3":
		var cfg S3Config
		if err := ParseStorageConfig(storage.Config, &cfg); err != nil {
			return err
		}
		return s.svc.UploadToS3(ctx, backupID, cfg)
	case "webdav":
		var cfg WebDAVConfig
		if err := ParseStorageConfig(storage.Config, &cfg); err != nil {
			return err
		}
		return s.svc.UploadToWebDAV(ctx, backupID, cfg)
	case "ftp":
		var cfg FTPConfig
		if err := ParseStorageConfig(storage.Config, &cfg); err != nil {
			return err
		}
		return s.svc.UploadToFTP(ctx, backupID, cfg)
	default:
		return fmt.Errorf("unsupported storage type: %s", storage.Type)
	}
}

// GetStorage returns a storage by ID.
func (s *Service) GetStorage(ctx context.Context, id int64) (*entity.BackupStorage, error) {
	return s.repo.GetStorageByID(ctx, id)
}
