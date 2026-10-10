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
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	agentpb "corefusion/internal/runtime/agent/proto"
	"corefusion/internal/store/repository"
)

// Syncer triggers an immediate config sync (implemented by configsync.Service).
type Syncer interface {
	SyncNow(ctx context.Context) error
	SyncServers(ctx context.Context, serverIDs []int64) error
	LastSync() time.Time
}

// ConfigWriter writes runtime config files (implemented by process.Manager).
type ConfigWriter interface {
	WriteRuntimeConfig(coreType string, data []byte) error
}

// ServerLister enumerates the server (node) IDs that runtime config is synced to.
// Satisfied by NodeServerRepository.
type ServerLister interface {
	ListAllServerIDs(ctx context.Context) ([]int64, error)
	ListAll(ctx context.Context) ([]entity.NodeServer, error)
}

// VerifyNodeResult is a single node's live verification state, used by the
// runtime UI to show the multi-node verification surface before/after an apply.
type VerifyNodeResult struct {
	ServerID    int64             `json:"server_id"`
	Name        string            `json:"name"`
	Hostname    string            `json:"hostname"`
	Region      string            `json:"region"`
	IsLocal     bool              `json:"is_local"`
	AgentStatus string            `json:"agent_status"`
	Online      bool              `json:"online"`
	VerifyOk    bool              `json:"verify_ok"`
	Reason      string            `json:"reason"`
	Cores       []VerifyCoreState `json:"cores"`
}

// VerifyCoreState is the live state of one core on a node.
type VerifyCoreState struct {
	CoreType string `json:"core_type"`
	State    string `json:"state"`
	Crashed  bool   `json:"crashed"`
}

// VerifyNodes returns the live verification state of every node that runtime
// config is synced to. It does not create a job — it is a read-only preview of
// the same per-node checks run after an apply, so the UI can surface all nodes.
// VerifyNodes returns the live verification state of every known node.
func (s *Service) VerifyNodes(ctx context.Context) ([]VerifyNodeResult, error) {
	return s.VerifySelected(ctx, nil)
}

// VerifySelected verifies only the given nodes. When serverIDs is empty (or
// nil) every known node is verified. Unknown IDs are ignored.
func (s *Service) VerifySelected(ctx context.Context, serverIDs []int64) ([]VerifyNodeResult, error) {
	ids, err := s.serverLister.ListAllServerIDs(ctx)
	if err != nil || len(ids) == 0 {
		ids = []int64{1} // always include the local panel node
	}
	idSet := map[int64]struct{}{1: {}}
	for _, id := range ids {
		idSet[id] = struct{}{}
	}

	servers := make([]int64, 0, len(idSet))
	for id := range idSet {
		servers = append(servers, id)
	}

	// Apply the node filter when the caller selected specific nodes.
	if len(serverIDs) > 0 {
		pick := make([]int64, 0, len(serverIDs))
		for _, id := range serverIDs {
			if _, ok := idSet[id]; ok {
				pick = append(pick, id)
			}
		}
		servers = pick
	}
	sort.Slice(servers, func(i, j int) bool { return servers[i] < servers[j] })

	infoMap := map[int64]entity.NodeServer{}
	if s.serverLister != nil {
		if all, e := s.serverLister.ListAll(ctx); e == nil {
			for _, ns := range all {
				infoMap[ns.ID] = ns
			}
		}
	}

	vctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results := make([]VerifyNodeResult, 0, len(servers))
	for _, id := range servers {
		ns := infoMap[id]
		online := false
		if s.statusProvider != nil {
			online = s.statusProvider.IsOnlineForServer(id)
		}
		ok, reason, states := s.verifyNode(vctx, id)
		cores := make([]VerifyCoreState, 0, len(states))
		for ct, st := range states {
			cores = append(cores, VerifyCoreState{CoreType: ct, State: st, Crashed: st == "crashed"})
		}
		sort.Slice(cores, func(i, j int) bool { return cores[i].CoreType < cores[j].CoreType })
		results = append(results, VerifyNodeResult{
			ServerID:    id,
			Name:        ns.Name,
			Hostname:    ns.Hostname,
			Region:      ns.Region,
			IsLocal:     ns.IsLocal,
			AgentStatus: ns.AgentStatus,
			Online:      online,
			VerifyOk:    ok,
			Reason:      reason,
			Cores:       cores,
		})
	}
	return results, nil
}

// NodeStatusProvider reports a node's live runtime status, used for
// control-plane-orchestrated post-apply verification. Satisfied by
// agent.NodeServerManager (GetStatusForServer / IsOnlineForServer).
type NodeStatusProvider interface {
	GetStatusForServer(ctx context.Context, serverID int64) (*agentpb.GetStatusResponse, error)
	IsOnlineForServer(serverID int64) bool
}

// coreTypes are the runtime config files managed under <dataDir>/runtime/.
var coreTypes = []string{"xray", "singbox"}

// Service implements the runtime config workflow.
type Service struct {
	db             *gorm.DB
	dataDir        string
	syncer         Syncer
	writer         ConfigWriter // optional; falls back to direct file writes
	serverLister   ServerLister
	statusProvider NodeStatusProvider
}

// NewService creates a runtime apply service. serverLister and statusProvider
// enable multi-node post-apply verification; pass nil to disable verification.
func NewService(db *gorm.DB, dataDir string, syncer Syncer, writer ConfigWriter, serverLister ServerLister, statusProvider NodeStatusProvider) *Service {
	return &Service{db: db, dataDir: dataDir, syncer: syncer, writer: writer, serverLister: serverLister, statusProvider: statusProvider}
}

func (s *Service) runtimePath(coreType string) string {
	return filepath.Join(s.dataDir, "runtime", coreType+".jsonc")
}

// ---------------------------------------------------------------------------
// Snapshots
// ---------------------------------------------------------------------------

// CreateSnapshot captures current runtime config files into a snapshot row.
// serverIDs records the intended target nodes (empty = all nodes).
func (s *Service) CreateSnapshot(ctx context.Context, snapshotType, triggerSource, reason string, serverIDs []int64) (*entity.RuntimeSnapshot, error) {
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

	idsJSON, _ := json.Marshal(serverIDs)
	snap := &entity.RuntimeSnapshot{
		SnapshotType:        snapshotType,
		TriggerSource:       triggerSource,
		Reason:              reason,
		TargetServerIDsJSON: string(idsJSON),
		ConfigJSON:          string(cfgJSON),
		FilePathsJSON:       string(pathsJSON),
		ChecksumSHA256:      hex.EncodeToString(h.Sum(nil)),
		CreatedAt:           time.Now(),
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
func (s *Service) DryRun(ctx context.Context, serverIDs []int64) (*DryRunResult, error) {
	var inbounds []entity.InboundConfig
	q := s.db.WithContext(ctx).Where("enabled = ?", true)
	if len(serverIDs) > 0 {
		q = q.Where("server_id IN ?", serverIDs)
	}
	if err := q.Find(&inbounds).Error; err != nil {
		return nil, err
	}
	// 直查库：config_json 是密文，先解密再做 JSON 校验（否则密文会被误报为非法 JSON）
	repository.DecryptInboundConfigs(inbounds)

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

func (s *Service) newJob(ctx context.Context, operationKind, triggerSource, reason string, snapshotID *int64, serverIDs []int64) (*entity.RuntimeApplyJob, error) {
	now := time.Now()
	idsJSON, _ := json.Marshal(serverIDs)
	job := &entity.RuntimeApplyJob{
		TriggerSource:       triggerSource,
		Reason:              reason,
		OperationKind:       operationKind,
		Status:              "running",
		Phase:               "start",
		SnapshotID:          snapshotID,
		TargetServerIDsJSON: string(idsJSON),
		TargetCount:         1, // corrected by runVerification once nodes are enumerated
		StartedAt:           &now,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := s.db.WithContext(ctx).Create(job).Error; err != nil {
		return nil, err
	}
	return job, nil
}

// runVerification performs control-plane-orchestrated, multi-node post-apply
// verification. A single node can only report its own runtime state, so the
// control plane enumerates every synced node, queries each one's live status
// via the agent, and aggregates the results into the job. No single node is
// expected to verify the whole topology by itself.
func (s *Service) runVerification(ctx context.Context, job *entity.RuntimeApplyJob, serverIDs []int64) {
	if s.statusProvider == nil || s.serverLister == nil {
		job.VerifyStatus = "skipped"
		job.UpdatedAt = time.Now()
		_ = s.db.WithContext(ctx).Save(job).Error
		s.addEvent(ctx, job.ID, "warn", "verify_skipped", "verify", "node status provider not configured")
		return
	}

	vctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ids, err := s.serverLister.ListAllServerIDs(vctx)
	if err != nil || len(ids) == 0 {
		// Always include the local panel node (id 1).
		ids = []int64{1}
	}
	idSet := map[int64]struct{}{1: {}}
	for _, id := range ids {
		idSet[id] = struct{}{}
	}
	// Only verify the explicitly selected nodes (if any).
	if len(serverIDs) > 0 {
		picked := map[int64]struct{}{}
		for _, id := range serverIDs {
			if _, ok := idSet[id]; ok {
				picked[id] = struct{}{}
			}
		}
		idSet = picked
	}
	servers := make([]int64, 0, len(idSet))
	for id := range idSet {
		servers = append(servers, id)
	}

	type nodeResult struct {
		id     int64
		ok     bool
		reason string
		states map[string]string
	}
	results := make([]nodeResult, len(servers))
	var wg sync.WaitGroup
	for i, id := range servers {
		wg.Add(1)
		go func(i int, id int64) {
			defer wg.Done()
			ok, reason, states := s.verifyNode(vctx, id)
			results[i] = nodeResult{id: id, ok: ok, reason: reason, states: states}
		}(i, id)
	}
	wg.Wait()

	okCount, failCount := 0, 0
	now := time.Now()
	for _, r := range results {
		status := "verified_ok"
		if !r.ok {
			status = "verified_failed"
			failCount++
		} else {
			okCount++
		}
		statesJSON, _ := json.Marshal(r.states)
		_ = s.db.WithContext(ctx).Create(&entity.RuntimeApplyJobTarget{
			JobID:        job.ID,
			TargetType:   "server",
			TargetID:     r.id,
			ApplyMode:    job.OperationKind,
			Status:       status,
			Phase:        "verify",
			ErrorMessage: r.reason,
			StartedAt:    job.StartedAt,
			FinishedAt:   timePtr(now),
			CreatedAt:    now,
			UpdatedAt:    now,
		}).Error
		s.addEvent(ctx, job.ID, levelForVerify(r.ok), "verify_node", "verify",
			fmt.Sprintf("server %d: %s", r.id, r.reason))
		_ = s.db.WithContext(ctx).Model(&entity.RuntimeApplyJobEvent{}).
			Where("job_id = ? AND event_type = ? AND message = ?", job.ID, "verify_node",
				fmt.Sprintf("server %d: %s", r.id, r.reason)).
			Update("detail_json", string(statesJSON)).Error
	}

	job.TargetCount = len(servers)
	job.SuccessCount = okCount
	job.FailedCount = failCount
	switch {
	case failCount == 0:
		job.VerifyStatus = "ok"
	case okCount == 0:
		job.VerifyStatus = "failed"
	default:
		job.VerifyStatus = "partial"
	}
	job.UpdatedAt = now
	_ = s.db.WithContext(ctx).Save(job).Error
	s.addEvent(ctx, job.ID, verifyLevel(job.VerifyStatus), "verify_finished", "verify",
		fmt.Sprintf("verified %d node(s): %d ok, %d failed", len(servers), okCount, failCount))
}

// verifyNode checks a single node's live runtime state via the agent. It returns
// whether the node's cores are healthy and a human-readable reason. It never
// verifies anything beyond this node — aggregation is the control plane's job.
func (s *Service) verifyNode(ctx context.Context, serverID int64) (ok bool, reason string, states map[string]string) {
	states = map[string]string{}
	if !s.statusProvider.IsOnlineForServer(serverID) {
		return false, "节点离线", states
	}
	resp, err := s.statusProvider.GetStatusForServer(ctx, serverID)
	if err != nil {
		return false, "状态查询失败: " + err.Error(), states
	}
	crashed := []string{}
	for _, c := range resp.GetCores() {
		states[c.GetCoreType()] = c.GetState()
		if c.GetState() == "crashed" {
			crashed = append(crashed, c.GetCoreType())
		}
	}
	if len(crashed) > 0 {
		return false, "存在崩溃的 core: " + strings.Join(crashed, ","), states
	}
	if len(resp.GetCores()) == 0 {
		return true, "节点在线（无运行中的 core）", states
	}
	return true, "节点在线，core 运行正常", states
}

func timePtr(t time.Time) *time.Time { return &t }

func levelForVerify(ok bool) string {
	if ok {
		return "info"
	}
	return "error"
}

func verifyLevel(status string) string {
	switch status {
	case "ok":
		return "info"
	case "skipped":
		return "warn"
	default:
		return "error"
	}
}

// Apply snapshots current config, then triggers an immediate sync (DB -> runtime files)
// for the selected nodes. An empty serverIDs syncs every known node.
func (s *Service) Apply(ctx context.Context, triggerSource, reason string, serverIDs []int64) (*entity.RuntimeApplyJob, error) {
	var snapID *int64
	if snap, err := s.CreateSnapshot(ctx, "pre_apply", triggerSource, reason, nil); err == nil {
		snapID = &snap.ID
	}

	job, err := s.newJob(ctx, "apply", triggerSource, reason, snapID, serverIDs)
	if err != nil {
		return nil, err
	}
	s.addEvent(ctx, job.ID, "info", "apply_started", "sync", "triggering immediate config sync")

	syncErr := s.syncer.SyncServers(ctx, serverIDs)
	s.finishJob(ctx, job, syncErr)
	if syncErr == nil {
		s.runVerification(ctx, job, serverIDs)
	}
	return job, syncErr
}

// Rollback restores runtime config files from the given snapshot.
// Note: the DB remains the source of truth — the periodic configsync will
// regenerate files from DB on its next tick, so rollback is intended as an
// emergency measure while the underlying DB config is being corrected.
func (s *Service) Rollback(ctx context.Context, snapshotID int64, triggerSource, reason string, serverIDs []int64) (*entity.RuntimeApplyJob, error) {
	var snap entity.RuntimeSnapshot
	if err := s.db.WithContext(ctx).First(&snap, snapshotID).Error; err != nil {
		return nil, fmt.Errorf("snapshot %d not found", snapshotID)
	}
	var configs map[string]string
	if err := json.Unmarshal([]byte(snap.ConfigJSON), &configs); err != nil {
		return nil, fmt.Errorf("snapshot %d has invalid payload: %w", snapshotID, err)
	}

	// Capture current state before overwriting.
	if cur, err := s.CreateSnapshot(ctx, "pre_rollback", triggerSource, fmt.Sprintf("before rollback to snapshot %d", snapshotID), nil); err == nil {
		_ = cur
	}

	job, err := s.newJob(ctx, "rollback", triggerSource, reason, &snap.ID, serverIDs)
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

	s.finishJob(ctx, job, firstErr)
	if firstErr == nil {
		s.runVerification(ctx, job, serverIDs)
	}
	return job, firstErr
}

// DisableProxy is an emergency stop: writes empty-inbound configs for the
// JSON-based cores (xray/singbox) whose runtime files exist. Recover via /runtime/apply.
func (s *Service) DisableProxy(ctx context.Context, triggerSource, reason string, serverIDs []int64) (*entity.RuntimeApplyJob, error) {
	if _, err := s.CreateSnapshot(ctx, "pre_disable", triggerSource, reason, nil); err != nil {
		return nil, fmt.Errorf("snapshot before disable failed: %w", err)
	}

	job, err := s.newJob(ctx, "disable_proxy", triggerSource, reason, nil, serverIDs)
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

	s.finishJob(ctx, job, firstErr)
	if firstErr == nil {
		s.runVerification(ctx, job, serverIDs)
	}
	return job, firstErr
}

// RetryJob re-executes a failed apply/rollback-style job as a new sync, scoped
// to the same target nodes the original job was intended for.
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
	var targetIDs []int64
	if job.TargetServerIDsJSON != "" {
		_ = json.Unmarshal([]byte(job.TargetServerIDsJSON), &targetIDs)
	}
	return s.Apply(ctx, "retry", reason, targetIDs)
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
	Job     entity.RuntimeApplyJob         `json:"job"`
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
		"snapshot_count": snapCount,
		"job_count":      jobCount,
		"jobs_by_status": statusMap,
		"last_sync":      s.syncer.LastSync(),
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
