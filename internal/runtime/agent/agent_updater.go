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
// 多平台支持：
//   - 面板 data/binaries/active/corefusion-agent/ 下可存放多个平台的二进制
//   - 命名规则：corefusion-agent-{os}-{arch}（如 corefusion-agent-linux-amd64）
//   - 推送更新时根据目标节点上报的 os/arch 选择对应二进制
//
// 本地服务器不需要更新 Agent（它就是面板本身）。
type AgentUpdater struct {
	mu           sync.RWMutex
	serverRepo   *repository.NodeServerRepository
	nodeMgr      *NodeServerManager
	dataDir      string // 面板数据目录，用于扫描本地 Agent 二进制
	panelBaseURL string // 面板基础 URL（如 http://127.0.0.1:8080），用于构建下载链接

	// 当前可用的 Agent 版本（所有平台共享同一 VERSION 文件）
	currentVersion string

	// 按平台存储的二进制信息，key = "{os}-{arch}"（如 "linux-amd64"）
	platformBinaries map[string]*AgentBinaryInfo

	// 手动指定的下载信息（通过 WithVersion 设置，用于 GitHub Release 等外部 URL 场景）
	// 当此字段非空时，所有平台使用同一个 URL（由外部托管平台处理 os/arch 路由）
	manualDownloadURL string
	manualChecksum    string

	// 更新任务跟踪
	pendingUpdates map[int64]*UpdateTask // server_id → task

	// 配置
	interval       time.Duration // 检查间隔
	maxConcurrency int           // 最大并发更新数
}

// AgentBinaryInfo 记录单个平台的 Agent 二进制信息。
type AgentBinaryInfo struct {
	OS          string // e.g. "linux", "darwin", "windows"
	Arch        string // e.g. "amd64", "arm64"
	Checksum    string // SHA-256 校验和
	DownloadURL string // 下载 URL（完整或相对路径）
	BinaryPath  string // 本地文件绝对路径
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
		serverRepo:       serverRepo,
		nodeMgr:          nodeMgr,
		platformBinaries: make(map[string]*AgentBinaryInfo),
		pendingUpdates:   make(map[int64]*UpdateTask),
		interval:         6 * time.Hour,
		maxConcurrency:   5,
	}
}

// WithVersion sets the target Agent version for updates (manual/GitHub Release mode).
// When set, all servers use the same download URL — the external hosting platform
// (e.g. GitHub Releases) is expected to serve the correct binary per os/arch.
func (u *AgentUpdater) WithVersion(version, downloadURL, checksum string) *AgentUpdater {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.currentVersion = version
	u.manualDownloadURL = downloadURL
	u.manualChecksum = checksum
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

// WithPanelBaseURL sets the panel base URL (e.g. http://127.0.0.1:8080)
// so that scanLocalAgentBinaries can return full download URLs.
func (u *AgentUpdater) WithPanelBaseURL(url string) *AgentUpdater {
	u.panelBaseURL = strings.TrimRight(url, "/")
	return u
}

// GetCurrentVersion returns the current target Agent version.
func (u *AgentUpdater) GetCurrentVersion() string {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.currentVersion
}

// GetPlatformBinaries returns all scanned platform binaries info.
func (u *AgentUpdater) GetPlatformBinaries() map[string]*AgentBinaryInfo {
	u.mu.RLock()
	defer u.mu.RUnlock()
	out := make(map[string]*AgentBinaryInfo, len(u.platformBinaries))
	for k, v := range u.platformBinaries {
		out[k] = v
	}
	return out
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

// resolveBinaryForServer 为指定服务器选择对应的二进制信息。
// 优先使用手动指定的 URL（WithVersion），否则按节点 os/arch 从本地扫描结果中选择。
func (u *AgentUpdater) resolveBinaryForServer(serverID int64) (version, downloadURL, checksum string, err error) {
	version = u.currentVersion
	log.Printf("[AgentUpdater][DEBUG] resolveBinary server=%d currentVersion=%q dataDir=%q platformKeys=%v", serverID, u.currentVersion, u.dataDir, keysOf(u.platformBinaries))
	if version == "" {
		return "", "", "", fmt.Errorf("no target version set")
	}

	// 手动指定模式（GitHub Release 等外部 URL）
	if u.manualDownloadURL != "" {
		return version, u.manualDownloadURL, u.manualChecksum, nil
	}

	// 本地扫描模式：按节点平台选择
	osName, arch := u.nodeMgr.GetServerPlatform(serverID)
	platformKey := osName + "-" + arch

	info, ok := u.platformBinaries[platformKey]
	if !ok {
		// 尝试 fallback：如果只有通用名 corefusion-agent，归为 linux-amd64
		if platformKey == "linux-amd64" {
			if info, ok = u.platformBinaries["generic"]; ok {
				return version, info.DownloadURL, info.Checksum, nil
			}
		}
		available := make([]string, 0, len(u.platformBinaries))
		for k := range u.platformBinaries {
			available = append(available, k)
		}
		return "", "", "", fmt.Errorf("no agent binary for platform %s (available: %v)", platformKey, available)
	}

	return version, info.DownloadURL, info.Checksum, nil
}

// UpdateServer triggers an Agent update on a specific server.
// The update command is sent via the PanelCommand stream.
// If no target version/URL is set, it auto-scans the panel's local Agent binary.
func (u *AgentUpdater) UpdateServer(ctx context.Context, serverID int64) error {
	u.mu.Lock()

	// Auto-scan local binaries if no version is configured
	if u.currentVersion == "" {
		if u.dataDir == "" {
			u.mu.Unlock()
			return fmt.Errorf("no target version set and no dataDir for auto-scan")
		}
		if err := u.scanLocalAgentBinaries(); err != nil {
			u.mu.Unlock()
			return fmt.Errorf("no target version and cannot scan local binary: %w", err)
		}
		log.Printf("[AgentUpdater] auto-scanned local binaries: version=%s, platforms=%d", u.currentVersion, len(u.platformBinaries))
	}

	if u.currentVersion == "" {
		u.mu.Unlock()
		return fmt.Errorf("no target version set")
	}

	// Check if server is local (no update needed)
	server, err := u.serverRepo.GetByID(ctx, serverID)
	if err != nil {
		u.mu.Unlock()
		return fmt.Errorf("server not found: %w", err)
	}
	if server.IsLocal {
		u.mu.Unlock()
		return fmt.Errorf("local server does not need Agent update")
	}

	// Check if already updating
	if existing, ok := u.pendingUpdates[serverID]; ok {
		if existing.Status == UpdateStatusDownloading ||
			existing.Status == UpdateStatusVerifying ||
			existing.Status == UpdateStatusReplacing ||
			existing.Status == UpdateStatusRestarting {
			u.mu.Unlock()
			return fmt.Errorf("server %d already has an update in progress (status=%s)", serverID, existing.Status)
		}
	}

	// Check if server is online
	if !u.nodeMgr.IsOnline(serverID) {
		u.mu.Unlock()
		return fmt.Errorf("server %d is offline, cannot push update", serverID)
	}

	// Skip if already on target version
	if server.AgentVersion == u.currentVersion {
		u.mu.Unlock()
		return fmt.Errorf("server %d already on version %s", serverID, u.currentVersion)
	}

	// Resolve the correct binary for this server's platform
	version, downloadURL, checksum, err := u.resolveBinaryForServer(serverID)
	if err != nil {
		u.mu.Unlock()
		return fmt.Errorf("server %d: %w", serverID, err)
	}

	// Create update task
	task := &UpdateTask{
		ServerID:    serverID,
		FromVersion: server.AgentVersion,
		ToVersion:   version,
		Status:      UpdateStatusPending,
		StartedAt:   time.Now(),
	}
	u.pendingUpdates[serverID] = task

	// Send update command via PanelCommand stream
	cmd := &agentpb.PanelCommand{
		Command: &agentpb.PanelCommand_UpdateAgent{
			UpdateAgent: &agentpb.UpdateAgentCommand{
				Version:     version,
				DownloadUrl: downloadURL,
				Checksum:    checksum,
			},
		},
	}

	if err := u.nodeMgr.SendCommand(ctx, serverID, cmd); err != nil {
		task.Status = UpdateStatusFailed
		task.Error = err.Error()
		now := time.Now()
		task.CompletedAt = &now
		u.mu.Unlock()
		return fmt.Errorf("send update command to server %d: %w", serverID, err)
	}

	task.Status = UpdateStatusDownloading
	osName, arch := u.nodeMgr.GetServerPlatform(serverID)
	log.Printf("[AgentUpdater] update pushed to server %d: %s → %s (platform=%s-%s)",
		serverID, task.FromVersion, task.ToVersion, osName, arch)

	u.mu.Unlock()
	return nil
}

// UpdateAllServers triggers Agent updates on all eligible remote servers.
// Updates are sent concurrently up to maxConcurrency.
// If no target version is set, auto-scans the panel's local Agent binary.
func (u *AgentUpdater) UpdateAllServers(ctx context.Context) error {
	// Auto-scan local binaries if no version is configured
	if u.currentVersion == "" && u.dataDir != "" {
		u.mu.Lock()
		if err := u.scanLocalAgentBinaries(); err != nil {
			u.mu.Unlock()
			return fmt.Errorf("no target version and cannot scan local binary: %w", err)
		}
		log.Printf("[AgentUpdater] auto-scanned local binaries: version=%s, platforms=%d", u.currentVersion, len(u.platformBinaries))
		u.mu.Unlock()
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

// scanLocalAgentBinaries scans the panel's local Agent binary directory
// and populates u.platformBinaries with info for each platform found.
// It also reads the VERSION file to set u.currentVersion.
//
// Supported binary naming:
//   - corefusion-agent-{os}-{arch}  (e.g. corefusion-agent-linux-amd64, corefusion-agent-darwin-arm64)
//   - corefusion-agent              (generic/fallback, treated as linux-amd64)
func (u *AgentUpdater) scanLocalAgentBinaries() error {
	agentDir := filepath.Join(u.dataDir, "binaries", "active", "corefusion-agent")

	// Read VERSION file
	versionFile := filepath.Join(agentDir, "VERSION")
	data, err := os.ReadFile(versionFile)
	if err != nil {
		return fmt.Errorf("read VERSION file: %w", err)
	}
	version := strings.TrimSpace(string(data))
	if version == "" {
		return fmt.Errorf("VERSION file is empty")
	}
	u.currentVersion = version

	// Scan directory for binary files
	entries, err := os.ReadDir(agentDir)
	if err != nil {
		return fmt.Errorf("read agent directory: %w", err)
	}

	u.platformBinaries = make(map[string]*AgentBinaryInfo)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()

		// Skip VERSION and non-binary files
		if name == "VERSION" || strings.HasSuffix(name, ".bak") || strings.HasSuffix(name, ".sha256") {
			continue
		}

		// Must start with "corefusion-agent"
		if !strings.HasPrefix(name, "corefusion-agent") {
			continue
		}

		binaryPath := filepath.Join(agentDir, name)

		// Parse os-arch from filename
		var osName, arch, platformKey string
		suffix := strings.TrimPrefix(name, "corefusion-agent-")
		if suffix != name {
			// Has os-arch suffix: e.g. "linux-amd64"
			parts := strings.SplitN(suffix, "-", 2)
			if len(parts) == 2 {
				osName = parts[0]
				arch = parts[1]
			} else {
				// Unexpected format, skip
				continue
			}
			platformKey = osName + "-" + arch
		} else {
			// Generic name "corefusion-agent" — treat as linux-amd64 fallback
			osName = "linux"
			arch = "amd64"
			platformKey = "generic"
		}

		// Compute SHA-256 checksum
		checksum, err := computeFileChecksum(binaryPath)
		if err != nil {
			log.Printf("[AgentUpdater] warning: failed to compute checksum for %s: %v", name, err)
			continue
		}

		// Build download URL
		downloadURL := fmt.Sprintf("/api/v1/agent/download?os=%s&arch=%s", osName, arch)
		if u.panelBaseURL != "" {
			downloadURL = u.panelBaseURL + downloadURL
		}

		info := &AgentBinaryInfo{
			OS:          osName,
			Arch:        arch,
			Checksum:    checksum,
			DownloadURL: downloadURL,
			BinaryPath:  binaryPath,
		}
		u.platformBinaries[platformKey] = info

		log.Printf("[AgentUpdater] scanned binary: platform=%s-%s, file=%s, checksum=%s...",
			osName, arch, name, checksum[:12])
	}

	// If we have a generic binary, also register it as linux-amd64
	if generic, ok := u.platformBinaries["generic"]; ok {
		generic.OS = "linux"
		generic.Arch = "amd64"
		// Also register as linux-amd64 if no platform-specific one exists
		if _, exists := u.platformBinaries["linux-amd64"]; !exists {
			linuxAmd64Info := *generic
			u.platformBinaries["linux-amd64"] = &linuxAmd64Info
		}
	}

	if len(u.platformBinaries) == 0 {
		return fmt.Errorf("no agent binaries found in %s", agentDir)
	}

	log.Printf("[AgentUpdater] scan complete: version=%s, platforms=%d", version, len(u.platformBinaries))
	return nil
}

// computeFileChecksum computes the SHA-256 checksum of a file.
func computeFileChecksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// keysOf returns the keys of a map as a slice (for debug logging).
func keysOf(m map[string]*AgentBinaryInfo) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
