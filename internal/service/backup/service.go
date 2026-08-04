package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/db"
	"corefusion/internal/store/repository"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jackc/pgx/v5"
	"github.com/jlaffaye/ftp"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// Service handles backup business logic.
type Service struct {
	repo    *repository.BackupRepository
	dataDir string
	db      *gorm.DB
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
		// sqlite: consistent VACUUM copy (handles WAL correctly)
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

// RestoreBackup restores from a backup file.
func (s *Service) RestoreBackup(ctx context.Context, id int64) error {
	rec, err := s.repo.GetRecordByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.extractTarGz(rec.FilePath, s.dataDir); err != nil {
		return err
	}

	// After extracting, import the database dump that was baked into the archive,
	// using the current environment DB configuration as the authority.
	dbType, dsn := db.StartupConfig()
	switch dbType {
	case db.DBTypeMySQL, db.DBTypePostgres:
		if dsn == "" {
			return fmt.Errorf("数据库 DSN 未配置（环境变量 CF_DB_DSN），无法还原 %s 备份", dbTypeLabel(dbType))
		}
		if err := s.importDumpsFromArchive(ctx, dbType, dsn); err != nil {
			return err
		}
	default:
		// sqlite: drop stale WAL/SHM so the restored snapshot is authoritative.
		os.Remove(filepath.Join(s.dataDir, "sqlite", "app.db-wal"))
		os.Remove(filepath.Join(s.dataDir, "sqlite", "app.db-shm"))
		// Any db_dump/*.sql in the archive belongs to a different DB type; discard it.
		os.RemoveAll(filepath.Join(s.dataDir, "db_dump"))
	}
	return nil
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

// RestoreFromFile restores from an uploaded file.
func (s *Service) RestoreFromFile(ctx context.Context, reader io.Reader) error {
	// Save to temp file
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

	return s.extractTarGz(tmpPath, s.dataDir)
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
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(dest)) {
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
		}
	}
	return nil
}

// --- Database dump (pure Go, no external tools) ---

// DumpDatabase produces a consistent dump of the configured database into destPath.
//   - sqlite:   VACUUM INTO (offline-consistent copy of the live db)
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
		// WAL mode: merge all committed transactions from the -wal file back
		// into the main db file and truncate the WAL before taking the copy.
		// This guarantees the dumped snapshot is fully flushed and keeps the
		// WAL from growing unbounded. If the db is busy, checkpoint is skipped
		// (non-fatal): VACUUM INTO still reads committed WAL data on its own.
		if _, err := sqlDB.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
			log.Printf("[Backup] sqlite wal_checkpoint(TRUNCATE) skipped: %v", err)
		}
		safe := strings.ReplaceAll(destPath, "'", "''")
		if _, err := sqlDB.Exec("VACUUM INTO '" + safe + "'"); err != nil {
			return fmt.Errorf("vacuum into failed: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unsupported database type for dump: %s", dbType)
	}
}

// RestoreDatabase restores the configured database from a dump file.
//   - sqlite:   copies the .db VACUUM copy over the live db file (caller restarts)
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

// restoreMySQL imports a .sql dump using a connection with multiStatements enabled.
func restoreMySQL(ctx context.Context, dsn, content string) error {
	connDSN := dsn
	if !strings.HasPrefix(dsn, "mysql://") {
		connDSN = "mysql://" + dsn
	}
	if strings.Contains(connDSN, "?") {
		connDSN += "&multiStatements=true"
	} else {
		connDSN += "?multiStatements=true"
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
	if err := gdb.WithContext(ctx).Exec(content).Error; err != nil {
		return fmt.Errorf("execute mysql dump: %w", err)
	}
	return nil
}

// restorePostgres imports a .sql dump via pgx (supports multiple statements).
func restorePostgres(ctx context.Context, dsn, content string) error {
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connect postgres for restore: %w", err)
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, content); err != nil {
		return fmt.Errorf("execute postgres dump: %w", err)
	}
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

	connDSN := dsn
	if !strings.HasPrefix(dsn, "mysql://") {
		connDSN = "mysql://" + dsn
	}
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

	if _, err := fmt.Fprintf(out, "-- MySQL dump (pure Go)\n-- Database: %s\nSET FOREIGN_KEY_CHECKS=0;\n\n", dbName); err != nil {
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
			if _, err := fmt.Fprintf(out, "INSERT INTO %s (%s) VALUES (%s);\n", quoteMySQLIdent(tbl), colList, strings.Join(parts, ",")); err != nil {
				rows.Close()
				return err
			}
		}
		rows.Close()
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
		var def string
		if err := conn.QueryRow(ctx, "SELECT pg_get_tabledef($1::text)", tbl).Scan(&def); err != nil {
			return fmt.Errorf("table def %s: %w", tbl, err)
		}
		defStr := strings.TrimSpace(def)
		defStr = strings.TrimSuffix(defStr, ";")
		if _, err := fmt.Fprintf(out, "DROP TABLE IF EXISTS %s CASCADE;\n%s;\n\n", quotePgIdent(tbl), defStr); err != nil {
			return err
		}

		// Data as INSERT statements so restore needs no COPY/psql (pure Go).
		rows, err := conn.Query(ctx, "SELECT * FROM "+quotePgIdent(tbl))
		if err != nil {
			return fmt.Errorf("select %s: %w", tbl, err)
		}
		cols := rows.FieldDescriptions()
		colNames := make([]string, len(cols))
		for i, c := range cols {
			colNames[i] = quotePgIdent(c.Name)
		}
		for rows.Next() {
			raw := rows.RawValues()
			parts := make([]string, len(raw))
			for i, rv := range raw {
				parts[i] = pgLiteral(rv, cols[i].DataTypeOID)
			}
			if _, err := fmt.Fprintf(out, "INSERT INTO %s (%s) VALUES (%s);\n", quotePgIdent(tbl), strings.Join(colNames, ","), strings.Join(parts, ",")); err != nil {
				rows.Close()
				return err
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
		if _, err := fmt.Fprintln(out); err != nil {
			return err
		}
	}

	return nil
}

// pgLiteral formats a raw column value using PostgreSQL text COPY escaping.
// oid is the PostgreSQL data type OID; numeric/boolean types can be emitted verbatim.
func pgLiteral(raw []byte, oid uint32) string {
	if raw == nil {
		return "\\N"
	}
	switch oid {
	case 16: // bool
		if string(raw) == "t" || string(raw) == "1" || string(raw) == "true" {
			return "t"
		}
		return "f"
	case 21, 23, 20, 26, 700, 701, 1700: // int2, int4, int8, oid, float4, float8, numeric
		return string(raw)
	default:
		return escapePgText(string(raw))
	}
}

func escapePgText(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString("\\\\")
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case '\t':
			b.WriteString("\\t")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// mysqlLiteral formats a Go value as a MySQL literal for INSERT statements.
func mysqlLiteral(v interface{}) string {
	if v == nil {
		return "NULL"
	}
	switch val := v.(type) {
	case []byte:
		return "X'" + hexEncode(val) + "'"
	case string:
		return "'" + strings.ReplaceAll(strings.ReplaceAll(val, "\\", "\\\\"), "'", "''") + "'"
	case int, int32, int64, float32, float64:
		return fmt.Sprintf("%v", val)
	case bool:
		if val {
			return "1"
		}
		return "0"
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
	svc     *Service
	stopCh  chan struct{}
	running bool
}

// NewBackupScheduler creates a new backup scheduler.
func NewBackupScheduler(svc *Service) *BackupScheduler {
	return &BackupScheduler{
		svc:    svc,
		stopCh: make(chan struct{}),
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
	ticker := time.NewTicker(1 * time.Minute)
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
