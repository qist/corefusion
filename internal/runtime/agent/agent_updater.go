package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	agentpb "corefusion/internal/runtime/agent/proto"
	"corefusion/internal/store/repository"
)

// AgentUpdater manages automatic Agent binary updates across remote servers.
//
// 更新流程：
//  1. 管理员上传新版本 Agent 二进制到面板（或指定下载 URL）
//  2. AgentUpdater 通过 PanelCommand 流推送 UpdateAgent 指令到各远程节点
//  3. Agent 收到指令后下载新版本、校验哈希、替换二进制并重启
//  4. Agent 重连后上报新版本号，面板确认更新成功
//
// 本地服务器不需要更新 Agent（它就是面板本身）。
type AgentUpdater struct {
	mu         sync.RWMutex
	serverRepo *repository.NodeServerRepository
	nodeMgr    *NodeServerManager
	dataDir    string // 面板数据目录，用于扫描本地 Agent 二进制

	// 当前可用的 Agent 版本
	currentVersion string
	downloadURL    string // 新版本下载基础 URL
	checksum       string // SHA-256 校验和

	// 更新任务跟踪
	pendingUpdates map[int64]*UpdateTask // server_id → task

	// 配置
	interval       time.Duration // 检查间隔
	maxConcurrency int           // 最大并发更新数
}

// UpdateTask 跟踪单个服务器的 Agent 更新进度。
type UpdateTask struct {
	ServerID    int64
	FromVersion string
	ToVersion   string
	Status      string // pending / downloading / verifying / replacing / restarting / done / failed
	Error       string
	StartedAt   time.Time
	CompletedAt *time.Time
}

// UpdateTaskStatus constants.
const (
	UpdateStatusPending     = "pending"
	UpdateStatusDownloading = "downloading"
	UpdateStatusVerifying   = "verifying"
	UpdateStatusReplacing   = "replacing"
	UpdateStatusRestarting  = "restarting"
	UpdateStatusDone        = "done"
	UpdateStatusFailed      = "failed"
)

// NewAgentUpdater creates a new AgentUpdater.
func NewAgentUpdater(serverRepo *repository.NodeServerRepository, nodeMgr *NodeServerManager) *AgentUpdater {
	return &AgentUpdater{
		serverRepo:     serverRepo,
		nodeMgr:        nodeMgr,
		pendingUpdates: make(map[int64]*UpdateTask),
		interval:       6 * time.Hour,
		maxConcurrency: 5,
	}
}

// WithVersion sets the target Agent version for updates.
func (u *AgentUpdater) WithVersion(version, downloadURL, checksum string) *AgentUpdater {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.currentVersion = version
	u.downloadURL = downloadURL
	u.checksum = checksum
	return u
}

// WithInterval sets the check interval.
func (u *AgentUpdater) WithInterval(d time.Duration) *AgentUpdater {
	u.interval = d
	return u
}

// WithMaxConcurrency sets the maximum number of concurrent updates.
func (u *AgentUpdater) WithMaxConcurrency(n int) *AgentUpdater {
	u.maxConcurrency = n
	return u
}

// WithDataDir sets the panel data directory for local binary scanning.
func (u *AgentUpdater) WithDataDir(dir string) *AgentUpdater {
	u.dataDir = dir
	return u
}

// GetCurrentVersion returns the current target Agent version.
func (u *AgentUpdater) GetCurrentVersion() string {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.currentVersion
}

// GetPendingUpdates returns all pending update tasks.
func (u *AgentUpdater) GetPendingUpdates() []UpdateTask {
	u.mu.RLock()
	defer u.mu.RUnlock()
	tasks := make([]UpdateTask, 0, len(u.pendingUpdates))
	for _, t := range u.pendingUpdates {
		tasks = append(tasks, *t)
	}
	return tasks
}

// UpdateServer triggers an Agent update on a specific server.
// The update command is sent via the PanelCommand stream.
// If no target version/URL is set, it auto-scans the panel's local Agent binary.
func (u *AgentUpdater) UpdateServer(ctx context.Context, serverID int64) error {
	u.mu.Lock()

	// Auto-scan local binary if no version/URL is configured
	if u.currentVersion == "" || u.downloadURL == "" {
		if u.dataDir == "" {
			u.mu.Unlock()
			return fmt.Errorf("no target version set and no dataDir for auto-scan")
		}
		version, checksum, downloadURL, err := u.scanLocalAgentBinary()
		if err != nil {
			u.mu.Unlock()
			return fmt.Errorf("no target version and cannot scan local binary: %w", err)
		}
		u.currentVersion = version
		u.downloadURL = downloadURL
		u.checksum = checksum
		log.Printf("[AgentUpdater] auto-scanned local binary: version=%s", version)
	}

	if u.currentVersion == "" {
		u.mu.Unlock()
		return fmt.Errorf("no target version set")
	}
	defer u.mu.Unlock()

	// Check if server is local (no update needed)
	server, err := u.serverRepo.GetByID(ctx, serverID)
	if err != nil {
		return fmt.Errorf("server not found: %w", err)
	}
	if server.IsLocal {
		return fmt.Errorf("local server does not need Agent update")
	}

	// Check if already updating
	if existing, ok := u.pendingUpdates[serverID]; ok {
		if existing.Status == UpdateStatusDownloading ||
			existing.Status == UpdateStatusVerifying ||
			existing.Status == UpdateStatusReplacing ||
			existing.Status == UpdateStatusRestarting {
			return fmt.Errorf("server %d already has an update in progress (status=%s)", serverID, existing.Status)
		}
	}

	// Check if server is online
	if !u.nodeMgr.IsOnline(serverID) {
		return fmt.Errorf("server %d is offline, cannot push update", serverID)
	}

	// Skip if already on target version
	if server.AgentVersion == u.currentVersion {
		return fmt.Errorf("server %d already on version %s", serverID, u.currentVersion)
	}

	// Create update task
	task := &UpdateTask{
		ServerID:    serverID,
		FromVersion: server.AgentVersion,
		ToVersion:   u.currentVersion,
		Status:      UpdateStatusPending,
		StartedAt:   time.Now(),
	}
	u.pendingUpdates[serverID] = task

	// Send update command via PanelCommand stream
	cmd := &agentpb.PanelCommand{
		Command: &agentpb.PanelCommand_UpdateAgent{
			UpdateAgent: &agentpb.UpdateAgentCommand{
				Version:     u.currentVersion,
				DownloadUrl: u.downloadURL,
				Checksum:    u.checksum,
			},
		},
	}

	if err := u.nodeMgr.SendCommand(ctx, serverID, cmd); err != nil {
		task.Status = UpdateStatusFailed
		task.Error = err.Error()
		now := time.Now()
		task.CompletedAt = &now
		return fmt.Errorf("send update command to server %d: %w", serverID, err)
	}

	task.Status = UpdateStatusDownloading
	log.Printf("[AgentUpdater] update pushed to server %d: %s → %s",
		serverID, task.FromVersion, task.ToVersion)

	return nil
}

// UpdateAllServers triggers Agent updates on all eligible remote servers.
// Updates are sent concurrently up to maxConcurrency.
// If no target version is set, auto-scans the panel's local Agent binary.
func (u *AgentUpdater) UpdateAllServers(ctx context.Context) error {
	// Auto-scan local binary if no version is configured
	if u.currentVersion == "" && u.dataDir != "" {
		version, checksum, downloadURL, err := u.scanLocalAgentBinary()
		if err != nil {
			return fmt.Errorf("no target version and cannot scan local binary: %w", err)
		}
		u.mu.Lock()
		u.currentVersion = version
		u.downloadURL = downloadURL
		u.checksum = checksum
		u.mu.Unlock()
		log.Printf("[AgentUpdater] auto-scanned local binary: version=%s", version)
	}

	if u.currentVersion == "" {
		return fmt.Errorf("no target version set")
	}

	servers, err := u.serverRepo.ListAll(ctx)
	if err != nil {
		return fmt.Errorf("list servers: %w", err)
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, u.maxConcurrency)
	errCh := make(chan error, len(servers))

	for _, server := range servers {
		if server.IsLocal {
			continue
		}
		if server.AgentVersion == u.currentVersion {
			continue // Already up to date
		}
		if !u.nodeMgr.IsOnline(server.ID) {
			log.Printf("[AgentUpdater] server %d (%s) offline, skipping", server.ID, server.Name)
			continue
		}

		wg.Add(1)
		go func(srvID int64, srvName string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if err := u.UpdateServer(ctx, srvID); err != nil {
				log.Printf("[AgentUpdater] failed to update server %d (%s): %v", srvID, srvName, err)
				errCh <- fmt.Errorf("server %d: %w", srvID, err)
			}
		}(server.ID, server.Name)
	}

	wg.Wait()
	close(errCh)

	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("%d/%d updates failed", len(errs), len(servers))
	}

	log.Printf("[AgentUpdater] all eligible servers updated to %s", u.currentVersion)
	return nil
}

// OnAgentReconnect is called when an Agent reconnects after update.
// It verifies the new version and marks the update task as done.
func (u *AgentUpdater) OnAgentReconnect(serverID int64, reportedVersion string) {
	u.mu.Lock()
	defer u.mu.Unlock()

	task, ok := u.pendingUpdates[serverID]
	if !ok {
		return
	}

	if reportedVersion == task.ToVersion {
		task.Status = UpdateStatusDone
		now := time.Now()
		task.CompletedAt = &now
		log.Printf("[AgentUpdater] server %d updated successfully: %s → %s",
			serverID, task.FromVersion, task.ToVersion)
	} else {
		task.Status = UpdateStatusFailed
		task.Error = fmt.Sprintf("version mismatch: expected %s, got %s", task.ToVersion, reportedVersion)
		now := time.Now()
		task.CompletedAt = &now
		log.Printf("[AgentUpdater] server %d update verification failed: expected %s, got %s",
			serverID, task.ToVersion, reportedVersion)
	}
}

// RunLoop starts a periodic check loop that auto-updates outdated Agents.
// Blocks until ctx is cancelled.
func (u *AgentUpdater) RunLoop(ctx context.Context) {
	if u.currentVersion == "" {
		log.Println("[AgentUpdater] no target version set, auto-update disabled")
		return
	}

	ticker := time.NewTicker(u.interval)
	defer ticker.Stop()

	log.Printf("[AgentUpdater] auto-update loop started, interval=%v, target=%s", u.interval, u.currentVersion)

	for {
		select {
		case <-ctx.Done():
			log.Println("[AgentUpdater] stopped")
			return
		case <-ticker.C:
			if err := u.autoUpdateOutdated(ctx); err != nil {
				log.Printf("[AgentUpdater] auto-update check error: %v", err)
			}
		}
	}
}

// autoUpdateOutdated finds and updates all servers running outdated Agent versions.
func (u *AgentUpdater) autoUpdateOutdated(ctx context.Context) error {
	servers, err := u.serverRepo.ListAll(ctx)
	if err != nil {
		return err
	}

	outdated := 0
	for _, server := range servers {
		if server.IsLocal {
			continue
		}
		if server.AgentVersion != "" && server.AgentVersion != u.currentVersion {
			if u.nodeMgr.IsOnline(server.ID) {
				outdated++
				if err := u.UpdateServer(ctx, server.ID); err != nil {
					log.Printf("[AgentUpdater] auto-update server %d: %v", server.ID, err)
				}
			}
		}
	}

	if outdated > 0 {
		log.Printf("[AgentUpdater] found %d outdated server(s), update pushed", outdated)
	}

	return nil
}

// CleanupCompleted removes completed/failed tasks older than the given duration.
func (u *AgentUpdater) CleanupCompleted(maxAge time.Duration) {
	u.mu.Lock()
	defer u.mu.Unlock()

	now := time.Now()
	for id, task := range u.pendingUpdates {
		if task.CompletedAt != nil && now.Sub(*task.CompletedAt) > maxAge {
			delete(u.pendingUpdates, id)
		}
	}
}

// scanLocalAgentBinary scans the panel's local Agent binary directory and
// returns version, checksum, and download URL.
func (u *AgentUpdater) scanLocalAgentBinary() (version, checksum, downloadURL string, err error) {
	agentDir := filepath.Join(u.dataDir, "binaries", "active", "corefusion-agent")

	// Read VERSION file
	versionFile := filepath.Join(agentDir, "VERSION")
	data, err := os.ReadFile(versionFile)
	if err != nil {
		return "", "", "", fmt.Errorf("read VERSION file: %w", err)
	}
	version = strings.TrimSpace(string(data))
	if version == "" {
		return "", "", "", fmt.Errorf("VERSION file is empty")
	}

	// Find the binary file
	candidates := []string{
		filepath.Join(agentDir, "corefusion-agent-linux-amd64"),
		filepath.Join(agentDir, "corefusion-agent-linux-arm64"),
		filepath.Join(agentDir, "corefusion-agent"),
	}
	var binaryPath string
	for _, p := range candidates {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			binaryPath = p
			break
		}
	}
	if binaryPath == "" {
		return "", "", "", fmt.Errorf("agent binary not found in %s", agentDir)
	}

	// Compute SHA-256
	f, err := os.Open(binaryPath)
	if err != nil {
		return "", "", "", fmt.Errorf("open binary: %w", err)
	}
	defer f.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", "", "", fmt.Errorf("compute checksum: %w", err)
	}
	checksum = hex.EncodeToString(hasher.Sum(nil))

	// Build download URL (relative path — panel will resolve to full URL)
	downloadURL = "/api/v1/agent/download?os=linux&arch=amd64"

	log.Printf("[AgentUpdater] scanned local binary: version=%s, checksum=%s...", version, checksum[:12])
	return version, checksum, downloadURL, nil
}
