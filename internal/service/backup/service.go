package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jlaffaye/ftp"
)

// Service handles backup business logic.
type Service struct {
	repo    *repository.BackupRepository
	dataDir string
}

// NewService creates a new backup service.
func NewService(repo *repository.BackupRepository, dataDir string) *Service {
	return &Service{repo: repo, dataDir: dataDir}
}

// CreateBackup creates a backup of the data directory.
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
	if err := s.createTarGz(s.dataDir, tmpPath, []string{"backup", "rules_tmp", "cache", "logs"}); err != nil {
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

// RestoreBackup restores from a backup file.
func (s *Service) RestoreBackup(ctx context.Context, id int64) error {
	rec, err := s.repo.GetRecordByID(ctx, id)
	if err != nil {
		return err
	}
	return s.extractTarGz(rec.FilePath, s.dataDir)
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

// createTarGz creates a tar.gz archive of the source directory.
func (s *Service) createTarGz(source, dest string, excludeDirs []string) error {
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	// Normalize paths for comparison
	absSource, _ := filepath.Abs(source)
	absDest, _ := filepath.Abs(dest)

	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get absolute path for comparison
		absPath, _ := filepath.Abs(path)

		// Skip the destination file itself
		if absPath == absDest {
			return nil
		}

		// Skip excluded directories
		rel, _ := filepath.Rel(absSource, absPath)
		for _, excl := range excludeDirs {
			if rel == excl || strings.HasPrefix(rel, excl+"/") {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// Skip symlinks to avoid issues
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return nil // Skip files that can't be read
		}
		header.Name = rel

		if err := tw.WriteHeader(header); err != nil {
			return nil // Skip files that can't be written
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return nil // Skip files that can't be opened
		}
		defer file.Close()

		_, err = io.Copy(tw, file)
		if err != nil {
			return nil // Skip files that can't be copied
		}
		return nil
	})
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
