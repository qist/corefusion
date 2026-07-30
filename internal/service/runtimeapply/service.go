// Package runtimeapply implements the runtime config workflow:
// snapshot / dry-run / apply (job tracked) / rollback / emergency disable.
//
// It works on top of the existing configsync pipeline: "apply" triggers an
// immediate DB->runtime-file sync; "snapshot" captures the current runtime
// config files; "rollback" restores files from a snapshot.
package runtimeapply

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// Syncer triggers an immediate config sync (implemented by configsync.Service).
type Syncer interface {
	SyncNow(ctx context.Context) error
	LastSync() time.Time
}

// ConfigWriter writes runtime config files (implemented by process.Manager).
type ConfigWriter interface {
	WriteRuntimeConfig(coreType string, data []byte) error
}

// coreTypes are the runtime config files managed under <dataDir>/runtime/.
var coreTypes = []string{"xray", "singbox", "hysteria2", "tuic"}

// Service implements the runtime config workflow.
type Service struct {
	db      *gorm.DB
	dataDir string
	syncer  Syncer
	writer  ConfigWriter // optional; falls back to direct file writes
}

// NewService creates a runtime apply service.
func NewService(db *gorm.DB, dataDir string, syncer Syncer, writer ConfigWriter) *Service {
	return &Service{db: db, dataDir: dataDir, syncer: syncer, writer: writer}
}

func (s *Service) runtimePath(coreType string) string {
	return filepath.Join(s.dataDir, "runtime", coreType+".jsonc")
}

// ---------------------------------------------------------------------------
// Snapshots
// ---------------------------------------------------------------------------

// CreateSnapshot captures current runtime config files into a snapshot row.
func (s *Service) CreateSnapshot(ctx context.Context, snapshotType, triggerSource, reason string) (*entity.RuntimeSnapshot, error) {
	configs := map[string]string{}
	var paths []string
	for _, ct := range coreTypes {
		p := s.runtimePath(ct)
		data, err := os.ReadFile(p)
		if err != nil {
			continue // file absent for unused cores
		}
		configs[ct] = string(data)
		paths = append(paths, p)
	}
	if len(configs) == 0 {
		return nil, fmt.Errorf("no runtime config files found under %s", filepath.Join(s.dataDir, "runtime"))
	}

	cfgJSON, _ := json.Marshal(configs)
	pathsJSON, _ := json.Marshal(paths)

	// Deterministic checksum: hash core types in sorted order.
	keys := make([]string, 0, len(configs))
	for k := range configs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	h := sha256.New()
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte(configs[k]))
	}

	snap := &entity.RuntimeSnapshot{
		SnapshotType:   snapshotType,
		TriggerSource:  triggerSource,
		Reason:         reason,
		ConfigJSON:     string(cfgJSON),
		FilePathsJSON:  string(pathsJSON),
		ChecksumSHA256: hex.EncodeToString(h.Sum(nil)),
		CreatedAt:      time.Now(),
	}
	if err := s.db.WithContext(ctx).Create(snap).Error; err != nil {
		return nil, err
	}
	return snap, nil
}

// ListSnapshots returns snapshots (newest first) without the config payload.
func (s *Service) ListSnapshots(ctx context.Context, limit, offset int) ([]entity.RuntimeSnapshot, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var total int64
	if err := s.db.WithContext(ctx).Model(&entity.RuntimeSnapshot{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var snaps []entity.RuntimeSnapshot
	err := s.db.WithContext(ctx).
		Select("id", "snapshot_type", "trigger_source", "reason", "file_paths_json", "checksum_sha256", "created_at").
		Order("id DESC").Limit(limit).Offset(offset).Find(&snaps).Error
	return snaps, total, err
}

// GetSnapshot returns a snapshot by id, including the full config payload.
func (s *Service) GetSnapshot(ctx context.Context, id int64) (*entity.RuntimeSnapshot, error) {
	var snap entity.RuntimeSnapshot
	if err := s.db.WithContext(ctx).First(&snap, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &snap, nil
}

// DeleteSnapshot removes a snapshot by id.
func (s *Service) DeleteSnapshot(ctx context.Context, id int64) error {
	return s.db.WithContext(ctx).Delete(&entity.RuntimeSnapshot{}, "id = ?", id).Error
}

// ---------------------------------------------------------------------------
// Dry run
// ---------------------------------------------------------------------------

// DryRunResult reports validation findings without touching anything.
type DryRunResult struct {
	OK            bool     `json:"ok"`
	AffectedCores []string `json:"affected_cores"`
	PortConflicts []string `json:"port_conflicts"`
	InvalidJSON   []string `json:"invalid_json"`
}

// DryRun validates enabled inbound configs: duplicate listen ports per server
// and malformed config JSON.
func (s *Service) DryRun(ctx context.Context) (*DryRunResult, error) {
	var inbounds []entity.InboundConfig
	if err := s.db.WithContext(ctx).Where("enabled = ?", true).Find(&inbounds).Error; err != nil {
		return nil, err
	}

	res := &DryRunResult{OK: true}
	coreSet := map[string]bool{}
	portOwner := map[string]string{} // "serverID:listen:port" -> tag

	for _, ib := range inbounds {
		coreSet[ib.CoreType] = true
		if ib.Port > 0 {
			key := fmt.Sprintf("%d:%s:%d", ib.ServerID, ib.Listen, ib.Port)
			if owner, dup := portOwner[key]; dup {
				res.PortConflicts = append(res.PortConflicts,
					fmt.Sprintf("server %d port %d used by both %q and %q", ib.ServerID, ib.Port, owner, ib.Tag))
			} else {
				portOwner[key] = ib.Tag
			}
		}
		if ib.ConfigJSON != "" && !json.Valid([]byte(ib.ConfigJSON)) {
			res.InvalidJSON = append(res.InvalidJSON, fmt.Sprintf("inbound %d (%s)", ib.ID, ib.Tag))
		}
	}
	for ct := range coreSet {
		res.AffectedCores = append(res.AffectedCores, ct)
	}
	sort.Strings(res.AffectedCores)
	if len(res.PortConflicts) > 0 || len(res.InvalidJSON) > 0 {
		res.OK = false
	}
	return res, nil
}

// ---------------------------------------------------------------------------
// Jobs
// ---------------------------------------------------------------------------

func (s *Service) addEvent(ctx context.Context, jobID int64, level, eventType, phase, message string) {
	_ = s.db.WithContext(ctx).Create(&entity.RuntimeApplyJobEvent{
		JobID:      jobID,
		EventLevel: level,
		EventType:  eventType,
		Phase:      phase,
		Message:    message,
		CreatedAt:  time.Now(),
	}).Error
}

func (s *Service) finishJob(ctx context.Context, job *entity.RuntimeApplyJob, err error) {
	now := time.Now()
	job.FinishedAt = &now
	job.UpdatedAt = now
	if err != nil {
		job.Status = "failed"
		job.FailedCount = job.TargetCount
		job.ErrorMessage = err.Error()
		s.addEvent(ctx, job.ID, "error", job.OperationKind+"_failed", "finish", err.Error())
	} else {
		job.Status = "success"
		job.SuccessCount = job.TargetCount
		s.addEvent(ctx, job.ID, "info", job.OperationKind+"_finished", "finish", "completed successfully")
	}
	_ = s.db.WithContext(ctx).Save(job).Error
}

func (s *Service) newJob(ctx context.Context, operationKind, triggerSource, reason string, snapshotID *int64) (*entity.RuntimeApplyJob, error) {
	now := time.Now()
	job := &entity.RuntimeApplyJob{
		TriggerSource: triggerSource,
		Reason:        reason,
		OperationKind: operationKind,
		Status:        "running",
		Phase:         "start",
		SnapshotID:    snapshotID,
		TargetCount:   1,
		StartedAt:     &now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.db.WithContext(ctx).Create(job).Error; err != nil {
		return nil, err
	}
	_ = s.db.WithContext(ctx).Create(&entity.RuntimeApplyJobTarget{
		JobID:      job.ID,
		TargetType: "node",
		TargetID:   1,
		ApplyMode:  "restart",
		Status:     "running",
		StartedAt:  &now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}).Error
	return job, nil
}

func (s *Service) finishJobTarget(ctx context.Context, jobID int64, failed bool, errMsg string) {
	now := time.Now()
	status := "success"
	if failed {
		status = "failed"
	}
	_ = s.db.WithContext(ctx).Model(&entity.RuntimeApplyJobTarget{}).
		Where("job_id = ?", jobID).
		Updates(map[string]interface{}{
			"status": status, "error_message": errMsg,
			"finished_at": now, "updated_at": now,
		}).Error
}

// Apply snapshots current config, then triggers an immediate sync (DB -> runtime files).
func (s *Service) Apply(ctx context.Context, triggerSource, reason string) (*entity.RuntimeApplyJob, error) {
	var snapID *int64
	if snap, err := s.CreateSnapshot(ctx, "pre_apply", triggerSource, reason); err == nil {
		snapID = &snap.ID
	}

	job, err := s.newJob(ctx, "apply", triggerSource, reason, snapID)
	if err != nil {
		return nil, err
	}
	s.addEvent(ctx, job.ID, "info", "apply_started", "sync", "triggering immediate config sync")

	syncErr := s.syncer.SyncNow(ctx)
	s.finishJobTarget(ctx, job.ID, syncErr != nil, errString(syncErr))
	s.finishJob(ctx, job, syncErr)
	return job, syncErr
}

// Rollback restores runtime config files from the given snapshot.
// Note: the DB remains the source of truth — the periodic configsync will
// regenerate files from DB on its next tick, so rollback is intended as an
// emergency measure while the underlying DB config is being corrected.
func (s *Service) Rollback(ctx context.Context, snapshotID int64, triggerSource, reason string) (*entity.RuntimeApplyJob, error) {
	var snap entity.RuntimeSnapshot
	if err := s.db.WithContext(ctx).First(&snap, snapshotID).Error; err != nil {
		return nil, fmt.Errorf("snapshot %d not found", snapshotID)
	}
	var configs map[string]string
	if err := json.Unmarshal([]byte(snap.ConfigJSON), &configs); err != nil {
		return nil, fmt.Errorf("snapshot %d has invalid payload: %w", snapshotID, err)
	}

	// Capture current state before overwriting.
	if cur, err := s.CreateSnapshot(ctx, "pre_rollback", triggerSource, fmt.Sprintf("before rollback to snapshot %d", snapshotID)); err == nil {
		_ = cur
	}

	job, err := s.newJob(ctx, "rollback", triggerSource, reason, &snap.ID)
	if err != nil {
		return nil, err
	}

	var firstErr error
	for ct, content := range configs {
		if werr := s.writeConfig(ct, []byte(content)); werr != nil {
			s.addEvent(ctx, job.ID, "error", "rollback_write_failed", ct, werr.Error())
			if firstErr == nil {
				firstErr = werr
			}
			continue
		}
		s.addEvent(ctx, job.ID, "info", "rollback_written", ct, fmt.Sprintf("restored %s config (%d bytes)", ct, len(content)))
	}
	s.addEvent(ctx, job.ID, "warn", "rollback_notice", "finish",
		"runtime files restored from snapshot; periodic configsync may regenerate them from DB — fix DB config or pause sync if needed")

	s.finishJobTarget(ctx, job.ID, firstErr != nil, errString(firstErr))
	s.finishJob(ctx, job, firstErr)
	return job, firstErr
}

// DisableProxy is an emergency stop: writes empty-inbound configs for the
// JSON-based cores (xray/singbox) whose runtime files exist. Recover via /runtime/apply.
func (s *Service) DisableProxy(ctx context.Context, triggerSource, reason string) (*entity.RuntimeApplyJob, error) {
	if _, err := s.CreateSnapshot(ctx, "pre_disable", triggerSource, reason); err != nil {
		return nil, fmt.Errorf("snapshot before disable failed: %w", err)
	}

	job, err := s.newJob(ctx, "disable_proxy", triggerSource, reason, nil)
	if err != nil {
		return nil, err
	}

	empty := map[string][]byte{
		"xray":    []byte(`{"inbounds":[],"outbounds":[{"protocol":"freedom","tag":"direct"}]}`),
		"singbox": []byte(`{"inbounds":[],"outbounds":[{"type":"direct","tag":"direct"}]}`),
	}
	var firstErr error
	for ct, content := range empty {
		if _, statErr := os.Stat(s.runtimePath(ct)); statErr != nil {
			continue // core not in use
		}
		if werr := s.writeConfig(ct, content); werr != nil {
			s.addEvent(ctx, job.ID, "error", "disable_write_failed", ct, werr.Error())
			if firstErr == nil {
				firstErr = werr
			}
			continue
		}
		s.addEvent(ctx, job.ID, "info", "proxy_disabled", ct, "empty inbound config written")
	}
	s.addEvent(ctx, job.ID, "warn", "disable_notice", "finish", "proxy disabled; use POST /runtime/apply to restore")

	s.finishJobTarget(ctx, job.ID, firstErr != nil, errString(firstErr))
	s.finishJob(ctx, job, firstErr)
	return job, firstErr
}

// RetryJob re-executes a failed apply/rollback-style job as a new sync.
func (s *Service) RetryJob(ctx context.Context, jobID int64, reason string) (*entity.RuntimeApplyJob, error) {
	var job entity.RuntimeApplyJob
	if err := s.db.WithContext(ctx).First(&job, jobID).Error; err != nil {
		return nil, fmt.Errorf("job %d not found", jobID)
	}
	if job.Status != "failed" {
		return nil, fmt.Errorf("only failed jobs can be retried (current status: %s)", job.Status)
	}
	if reason == "" {
		reason = fmt.Sprintf("retry of job %d", jobID)
	}
	return s.Apply(ctx, "retry", reason)
}

// CancelJob marks a pending/running job as cancelled.
func (s *Service) CancelJob(ctx context.Context, jobID int64, reason string) error {
	var job entity.RuntimeApplyJob
	if err := s.db.WithContext(ctx).First(&job, jobID).Error; err != nil {
		return fmt.Errorf("job %d not found", jobID)
	}
	if job.Status != "pending" && job.Status != "running" {
		return fmt.Errorf("job %d is %s and cannot be cancelled", jobID, job.Status)
	}
	now := time.Now()
	return s.db.WithContext(ctx).Model(&job).Updates(map[string]interface{}{
		"status": "cancelled", "error_message": reason,
		"finished_at": now, "updated_at": now,
	}).Error
}

// ListJobs returns apply jobs (newest first).
func (s *Service) ListJobs(ctx context.Context, limit, offset int) ([]entity.RuntimeApplyJob, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var total int64
	if err := s.db.WithContext(ctx).Model(&entity.RuntimeApplyJob{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var jobs []entity.RuntimeApplyJob
	err := s.db.WithContext(ctx).Order("id DESC").Limit(limit).Offset(offset).Find(&jobs).Error
	return jobs, total, err
}

// JobDetail bundles a job with its targets and events.
type JobDetail struct {
	Job     entity.RuntimeApplyJob        `json:"job"`
	Targets []entity.RuntimeApplyJobTarget `json:"targets"`
	Events  []entity.RuntimeApplyJobEvent  `json:"events"`
}

// GetJob returns a job with targets and events.
func (s *Service) GetJob(ctx context.Context, jobID int64) (*JobDetail, error) {
	var job entity.RuntimeApplyJob
	if err := s.db.WithContext(ctx).First(&job, jobID).Error; err != nil {
		return nil, fmt.Errorf("job %d not found", jobID)
	}
	d := &JobDetail{Job: job}
	_ = s.db.WithContext(ctx).Where("job_id = ?", jobID).Find(&d.Targets).Error
	_ = s.db.WithContext(ctx).Where("job_id = ?", jobID).Order("id ASC").Find(&d.Events).Error
	return d, nil
}

// Overview summarizes the runtime workflow state.
func (s *Service) Overview(ctx context.Context) (map[string]interface{}, error) {
	var snapCount, jobCount int64
	s.db.WithContext(ctx).Model(&entity.RuntimeSnapshot{}).Count(&snapCount)
	s.db.WithContext(ctx).Model(&entity.RuntimeApplyJob{}).Count(&jobCount)

	var jobStats []struct {
		Status string
		Cnt    int64
	}
	s.db.WithContext(ctx).Model(&entity.RuntimeApplyJob{}).
		Select("status, COUNT(*) AS cnt").Group("status").Scan(&jobStats)
	statusMap := map[string]int64{}
	for _, st := range jobStats {
		statusMap[st.Status] = st.Cnt
	}

	var lastSnap entity.RuntimeSnapshot
	hasSnap := s.db.WithContext(ctx).
		Select("id", "snapshot_type", "trigger_source", "checksum_sha256", "created_at").
		Order("id DESC").First(&lastSnap).Error == nil

	var lastJob entity.RuntimeApplyJob
	hasJob := s.db.WithContext(ctx).Order("id DESC").First(&lastJob).Error == nil

	out := map[string]interface{}{
		"snapshot_count":  snapCount,
		"job_count":       jobCount,
		"jobs_by_status":  statusMap,
		"last_sync":       s.syncer.LastSync(),
	}
	if hasSnap {
		out["last_snapshot"] = lastSnap
	}
	if hasJob {
		out["last_job"] = lastJob
	}
	return out, nil
}

func (s *Service) writeConfig(coreType string, data []byte) error {
	if s.writer != nil {
		return s.writer.WriteRuntimeConfig(coreType, data)
	}
	dir := filepath.Join(s.dataDir, "runtime")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(s.runtimePath(coreType), data, 0640)
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
