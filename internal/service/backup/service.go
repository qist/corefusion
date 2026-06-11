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

	if err := s.createTarGz(s.dataDir, filePath, []string{"backup", "rules_tmp", "cache"}); err != nil {
		return nil, fmt.Errorf("create backup: %w", err)
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
func (s *Service) UploadToS3(ctx context.Context, backupID int64, config S3Config) error {
	rec, err := s.repo.GetRecordByID(ctx, backupID)
	if err != nil {
		return err
	}

	f, err := os.Open(rec.FilePath)
	if err != nil {
		return err
	}
	defer f.Close()

	// Simple S3 PUT request
	url := fmt.Sprintf("%s/%s/%s", config.Endpoint, config.Bucket, filepath.Base(rec.FilePath))
	req, err := http.NewRequestWithContext(ctx, "PUT", url, f)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/gzip")

	// Add S3 auth headers (simplified - in production use AWS SDK)
	if config.AccessKey != "" && config.SecretKey != "" {
		// For now, just set basic auth or S3 signature
		// In production, implement AWS Signature V4
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("S3 upload failed: HTTP %d", resp.StatusCode)
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

	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip excluded directories
		rel, _ := filepath.Rel(source, path)
		for _, excl := range excludeDirs {
			if rel == excl || strings.HasPrefix(rel, excl+"/") {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// Skip the dest file itself
		if path == dest {
			return nil
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = rel

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(tw, file)
		return err
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
