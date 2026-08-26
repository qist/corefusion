package logmanage

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"corefusion/internal/model/entity"
)

// Rotate executes log rotation for a single policy.
// If MaxSize is configured (> 0) and the file size hasn't reached it,
// rotation is skipped (returns a success job with RotatedFiles=0).
func (s *Service) Rotate(ctx context.Context, policyID int64) (*entity.LogRotationJob, error) {
	policy, err := s.policyRepo.GetByID(ctx, policyID)
	if err != nil {
		return nil, fmt.Errorf("policy not found: %w", err)
	}

	job := &entity.LogRotationJob{
		PolicyID:  policy.ID,
		Status:    "success",
		StartedAt: time.Now(),
	}

	// Rotate the log file
	rotated, rotateErr := s.rotateFile(policy)
	job.RotatedFiles = rotated
	if rotateErr != nil {
		job.Status = "failed"
		job.ErrorMessage = rotateErr.Error()
		job.FinishedAt = time.Now()
		_ = s.jobRepo.Create(ctx, job)
		return job, rotateErr
	}

	// Clean expired log files (always runs, even if rotation was skipped)
	cleaned, freed, cleanErr := s.cleanExpired(policy)
	job.CleanedFiles = cleaned
	job.FreedBytes = freed
	if cleanErr != nil {
		log.Printf("logmanage: cleanup warning for %s: %v", policy.LogPath, cleanErr)
	}

	job.FinishedAt = time.Now()
	if err := s.jobRepo.Create(ctx, job); err != nil {
		log.Printf("logmanage: failed to save job record: %v", err)
	}

	if rotated > 0 {
		log.Printf("logmanage: rotated %s - %d files rotated, %d files cleaned, %d bytes freed",
			policy.LogPath, job.RotatedFiles, job.CleanedFiles, job.FreedBytes)
	} else {
		log.Printf("logmanage: skipped rotation for %s (file too small or not exists), cleaned %d expired files",
			policy.LogPath, job.CleanedFiles)
	}
	return job, nil
}

// RotateAll executes rotation for all enabled policies.
func (s *Service) RotateAll(ctx context.Context) error {
	policies, err := s.policyRepo.ListEnabled(ctx)
	if err != nil {
		return err
	}

	for _, policy := range policies {
		if _, err := s.Rotate(ctx, policy.ID); err != nil {
			log.Printf("logmanage: failed to rotate policy %d (%s): %v", policy.ID, policy.Name, err)
		}
	}
	return nil
}

// rotateFile performs the actual log file rotation.
// Returns (0, nil) without error if rotation is skipped (file too small or not exists).
func (s *Service) rotateFile(policy *entity.LogRotationPolicy) (int, error) {
	logPath := policy.LogPath

	// Check if the log file exists
	info, err := os.Stat(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil // nothing to rotate
		}
		return 0, fmt.Errorf("stat %s: %w", logPath, err)
	}

	// Skip if file is empty
	if info.Size() == 0 {
		return 0, nil
	}

	// If MaxSize is configured, only rotate when file size reaches the threshold.
	if policy.MaxSize > 0 && info.Size() < policy.MaxSize {
		log.Printf("logmanage: skip rotation for %s (size=%d, threshold=%d)",
			logPath, info.Size(), policy.MaxSize)
		return 0, nil
	}

	// When MaxSize is 0 (unlimited), skip files smaller than 1KB to avoid
	// rotating tiny/empty-ish files that haven't accumulated meaningful content.
	if policy.MaxSize == 0 && info.Size() < 1024 {
		return 0, nil
	}

	// Generate rotated filename with date suffix
	dateSuffix := time.Now().Format("20060102")
	rotatedPath := fmt.Sprintf("%s.%s", logPath, dateSuffix)

	// If rotated file already exists, append a counter
	if _, err := os.Stat(rotatedPath); err == nil {
		for i := 2; i < 100; i++ {
			rotatedPath = fmt.Sprintf("%s.%s.%d", logPath, dateSuffix, i)
			if _, err := os.Stat(rotatedPath); os.IsNotExist(err) {
				break
			}
		}
	}

	// Rename the current log file
	if err := os.Rename(logPath, rotatedPath); err != nil {
		return 0, fmt.Errorf("rename %s -> %s: %w", logPath, rotatedPath, err)
	}

	// Create a new empty log file with same permissions
	newFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY, info.Mode())
	if err != nil {
		// Try to restore the original file
		_ = os.Rename(rotatedPath, logPath)
		return 0, fmt.Errorf("create new %s: %w", logPath, err)
	}
	newFile.Close()

	// For nginx, send reopen signal
	if policy.SourceType == "nginx" {
		s.signalNginxReopen()
	}

	// Compress if configured
	if policy.Compress {
		if err := gzipFile(rotatedPath); err != nil {
			log.Printf("logmanage: compress warning for %s: %v", rotatedPath, err)
		}
	}

	return 1, nil
}

// cleanExpired removes rotated log files older than RetainDays.
func (s *Service) cleanExpired(policy *entity.LogRotationPolicy) (int, int64, error) {
	if policy.RetainDays <= 0 {
		return 0, 0, nil
	}

	dir := filepath.Dir(policy.LogPath)
	base := filepath.Base(policy.LogPath)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, 0, fmt.Errorf("read dir %s: %w", dir, err)
	}

	cutoff := time.Now().AddDate(0, 0, -policy.RetainDays)
	cleaned := 0
	var freed int64

	for _, entry := range entries {
		name := entry.Name()

		// Match rotated files: basename.YYYYMMDD or basename.YYYYMMDD.gz
		if !strings.HasPrefix(name, base+".") {
			continue
		}

		// Parse the date from the filename
		dateStr := strings.TrimPrefix(name, base+".")
		dateStr = strings.TrimSuffix(dateStr, ".gz")
		dateStr = strings.TrimSuffix(dateStr, ".1")
		dateStr = strings.TrimSuffix(dateStr, ".2")
		dateStr = strings.TrimSuffix(dateStr, ".3")
		dateStr = strings.TrimSuffix(dateStr, ".4")
		dateStr = strings.TrimSuffix(dateStr, ".5")
		dateStr = strings.TrimSuffix(dateStr, ".6")
		dateStr = strings.TrimSuffix(dateStr, ".7")
		dateStr = strings.TrimSuffix(dateStr, ".8")
		dateStr = strings.TrimSuffix(dateStr, ".9")

		fileDate, err := time.Parse("20060102", dateStr)
		if err != nil {
			continue // not a rotated file we recognize
		}

		if fileDate.Before(cutoff) {
		 fullPath := filepath.Join(dir, name)
		 info, err := entry.Info()
		 if err == nil {
			 freed += info.Size()
		 }
		 if err := os.Remove(fullPath); err != nil {
			 log.Printf("logmanage: failed to delete %s: %v", fullPath, err)
			 continue
		 }
		 cleaned++
		}
	}

	return cleaned, freed, nil
}

// signalNginxReopen sends the reopen signal to nginx.
func (s *Service) signalNginxReopen() {
	// Try to find nginx binary
	nginxPath, err := exec.LookPath("nginx")
	if err != nil {
		log.Printf("logmanage: nginx not found in PATH, skipping reopen signal")
		return
	}

	cmd := exec.Command(nginxPath, "-s", "reopen")
	if output, err := cmd.CombinedOutput(); err != nil {
		log.Printf("logmanage: nginx -s reopen failed: %v, output: %s", err, string(output))
	}
}

// gzipFile compresses a file using gzip and replaces the original.
func gzipFile(path string) error {
	src, err := os.Open(path)
	if err != nil {
		return err
	}
	defer src.Close()

	dstPath := path + ".gz"
	dst, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	gw := gzip.NewWriter(dst)
	if _, err := io.Copy(gw, src); err != nil {
		gw.Close()
		os.Remove(dstPath)
		return err
	}
	if err := gw.Close(); err != nil {
		os.Remove(dstPath)
		return err
	}

	// Remove original after successful compression
	return os.Remove(path)
}

// RotateExpiredLogs is called by the scheduler for a specific policy.
func (s *Service) RotateExpiredLogs(ctx context.Context, policyID int64) {
	if _, err := s.Rotate(ctx, policyID); err != nil {
		log.Printf("logmanage: scheduled rotation failed for policy %d: %v", policyID, err)
	}
}

// parseRotatedDate extracts the date from a rotated log filename.
// Supports: basename.20060102, basename.20060102.gz, basename.20060102.N
func parseRotatedDate(base, name string) (time.Time, bool) {
	if !strings.HasPrefix(name, base+".") {
		return time.Time{}, false
	}

	suffix := strings.TrimPrefix(name, base+".")
	// Remove .gz extension
	suffix = strings.TrimSuffix(suffix, ".gz")

	// The date part should be the first 8 characters
	if len(suffix) < 8 {
		return time.Time{}, false
	}

	dateStr := suffix[:8]
	t, err := time.Parse("20060102", dateStr)
	if err != nil {
		return time.Time{}, false
	}

	return t, true
}
