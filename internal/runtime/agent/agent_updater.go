package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	agentpb "corefusion/internal/runtime/agent/proto"
	"corefusion/internal/store/repository"
	"corefusion/pkg/netutil"
)

// AgentGitHubRepo 是面板/Agent 二进制的发布仓库。
// 面板自更新与 Agent 推送共用同一份 Release（产物名见 .github/workflows/release.yml）。
const AgentGitHubRepo = "qist/corefusion"

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

	// 手动指定的下载信息（通过 WithVersion 设置，用于任意外部 URL 场景）
	// 当此字段非空时，所有平台使用同一个 URL（由外部托管平台处理 os/arch 路由）
	manualDownloadURL string
	manualChecksum    string

	// useGitHub 为 true 时，按目标节点的 os/arch 从 GitHub Release 解析产物
	// （corefusion-agent-{os}-{arch}）及其 sha256，无需面板本地存二进制。
	useGitHub bool

	// settingRepo 用于读取 GitHub 加速域名/代理（与面板自更新同一套设置）。
	settingRepo *repository.SystemSettingRepository

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

// UpdateTask tracks the update progress of a single server.
type UpdateTask struct {
	ServerID    int64      `json:"server_id"`
	FromVersion string     `json:"from_version"`
	ToVersion   string     `json:"to_version"`
	Status      string     `json:"status"` // pending / downloading / verifying / replacing / restarting / done / failed
	Error       string     `json:"error"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
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
	u.useGitHub = false
	return u
}

// WithGitHubVersion 设定目标版本，推送时按每个节点的 os/arch 从 GitHub Release
// 解析 corefusion-agent-{os}-{arch} 与其 sha256（面板无需本地存 Agent 二进制）。
func (u *AgentUpdater) WithGitHubVersion(version string) *AgentUpdater {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.currentVersion = strings.TrimSpace(version)
	u.manualDownloadURL = ""
	u.manualChecksum = ""
	u.useGitHub = true
	return u
}

// WithLocalBinary 使用面板 data/binaries/active/corefusion-agent 下的本地二进制
// （版本由该目录的 VERSION 文件决定）。
func (u *AgentUpdater) WithLocalBinary() *AgentUpdater {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.currentVersion = ""
	u.manualDownloadURL = ""
	u.manualChecksum = ""
	u.useGitHub = false
	return u
}

// WithSettingRepo 注入系统设置仓库，用于 GitHub 加速域名 / 代理。
func (u *AgentUpdater) WithSettingRepo(repo *repository.SystemSettingRepository) *AgentUpdater {
	u.settingRepo = repo
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
//   - WithGitHubVersion：按节点 os/arch 从 GitHub Release 解析产物与 sha256；
//   - WithVersion（URL 非空）：直接使用给定 URL；
//   - WithLocalBinary：按节点 os/arch 从面板本地扫描结果中选择。
func (u *AgentUpdater) resolveBinaryForServer(ctx context.Context, serverID int64) (version, downloadURL, checksum string, err error) {
	version = u.currentVersion

	if u.useGitHub {
		if version == "" {
			return "", "", "", fmt.Errorf("no target version set")
		}
		osName, arch := u.nodeMgr.GetServerPlatform(serverID)
		downloadURL, checksum, err = u.resolveGitHubAsset(ctx, version, osName, arch)
		if err != nil {
			return "", "", "", err
		}
		return version, downloadURL, checksum, nil
	}

	if u.manualDownloadURL != "" {
		return version, u.manualDownloadURL, u.manualChecksum, nil
	}

	if version == "" {
		return "", "", "", fmt.Errorf("no target version set")
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

// --- GitHub Release 解析 ---

type agentGitHubAsset struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Digest             string `json:"digest"`
}

type agentGitHubRelease struct {
	TagName     string             `json:"tag_name"`
	Prerelease  bool               `json:"prerelease"`
	PublishedAt string             `json:"published_at"`
	Body        string             `json:"body"`
	Assets      []agentGitHubAsset `json:"assets"`
}

// resolveGitHubAsset 在指定 Release 中按节点平台取 Agent 产物与 sha256。
// 校验和优先用 GitHub API 的 asset digest，其次取同名 .sha256 旁挂文件；
// 两者都没有则拒绝推送（与 Agent 自更新“无校验不更新”的基线一致）。
func (u *AgentUpdater) resolveGitHubAsset(ctx context.Context, version, osName, arch string) (downloadURL, checksum string, err error) {
	assetName := fmt.Sprintf("corefusion-agent-%s-%s", osName, arch)
	release, err := u.fetchAgentRelease(ctx, version)
	if err != nil {
		return "", "", err
	}

	var checksumURL string
	for _, a := range release.Assets {
		switch a.Name {
		case assetName:
			downloadURL = a.BrowserDownloadURL
			checksum = normalizeChecksum(a.Digest)
		case assetName + ".sha256":
			checksumURL = a.BrowserDownloadURL
		}
	}
	if downloadURL == "" {
		return "", "", fmt.Errorf("release %s has no asset %s", version, assetName)
	}
	if checksum == "" && checksumURL != "" {
		if checksum, err = u.fetchChecksumFile(ctx, checksumURL); err != nil {
			return "", "", err
		}
	}
	if checksum == "" {
		return "", "", fmt.Errorf("release %s asset %s has no sha256 checksum", version, assetName)
	}
	return downloadURL, checksum, nil
}

func (u *AgentUpdater) fetchAgentRelease(ctx context.Context, version string) (*agentGitHubRelease, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/tags/%s",
		AgentGitHubRepo, url.PathEscape(version))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := u.gitHubClient(ctx).Do(req)
	if err != nil {
		return nil, fmt.Errorf("github api: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("github api %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var release agentGitHubRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&release); err != nil {
		return nil, fmt.Errorf("decode github response: %w", err)
	}
	return &release, nil
}

func (u *AgentUpdater) fetchChecksumFile(ctx context.Context, checksumURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checksumURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := u.gitHubClient(ctx).Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch checksum: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch checksum: HTTP %d", resp.StatusCode)
	}
	body, err := netutil.ReadLimited(resp.Body, netutil.MaxChecksumBytes)
	if err != nil {
		return "", fmt.Errorf("read checksum: %w", err)
	}
	for _, field := range strings.Fields(string(body)) {
		if c := normalizeChecksum(field); c != "" {
			return c, nil
		}
	}
	return "", fmt.Errorf("checksum file has no sha256: %s", checksumURL)
}

// gitHubClient 读取面板的 GitHub 加速域名/代理设置，构造 GitHub 专用客户端。
func (u *AgentUpdater) gitHubClient(ctx context.Context) *http.Client {
	accel, proxy := u.gitHubSettings(ctx)
	return netutil.GitHubHTTPClientWithProxy(30*time.Second, accel, proxy)
}

func (u *AgentUpdater) gitHubSettings(ctx context.Context) (accel, proxy string) {
	if u.settingRepo == nil {
		return "", ""
	}
	val, err := u.settingRepo.GetByKey(ctx, "github.acceleration")
	if err != nil || val == "" {
		return "", ""
	}
	var cfg struct {
		Enabled bool `json:"enabled"`
		Domains []struct {
			URL     string `json:"url"`
			Enabled bool   `json:"enabled"`
		} `json:"domains"`
		ProxyURL string `json:"proxy_url"`
	}
	if json.Unmarshal([]byte(val), &cfg) != nil {
		return "", ""
	}
	proxy = cfg.ProxyURL
	if !cfg.Enabled {
		return "", proxy
	}
	for _, d := range cfg.Domains {
		if d.Enabled && d.URL != "" {
			return d.URL, proxy
		}
	}
	return "", proxy
}

// UpdateServer triggers an Agent update on a specific server.
// The update command is sent via the PanelCommand stream.
// If no target version/URL is set, it auto-scans the panel's local Agent binary.
func (u *AgentUpdater) UpdateServer(ctx context.Context, serverID int64) error {
	u.mu.Lock()

	switch {
	case u.useGitHub:
		if u.currentVersion == "" {
			u.mu.Unlock()
			return fmt.Errorf("no target version set")
		}
	case u.manualDownloadURL == "":
		// 面板本地二进制模式：每次推送重新扫描，保证用的是面板上最新的 Agent 二进制
		// （以前只在 currentVersion 为空时扫描，换二进制后必须重启面板才生效）。
		if u.dataDir == "" {
			u.mu.Unlock()
			return fmt.Errorf("no target version set and no dataDir for auto-scan")
		}
		if err := u.scanLocalAgentBinaries(); err != nil {
			u.mu.Unlock()
			return fmt.Errorf("cannot scan local agent binary: %w", err)
		}
		log.Printf("[AgentUpdater] scanned local binaries: version=%s, platforms=%d", u.currentVersion, len(u.platformBinaries))
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
	version, downloadURL, checksum, err := u.resolveBinaryForServer(ctx, serverID)
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
// GitHub 模式必须显式给出目标版本；本地二进制模式每次都会重新扫描，
// 面板上没有可用二进制时直接报错（由前端引导改走 GitHub 版本）。
func (u *AgentUpdater) UpdateAllServers(ctx context.Context) error {
	u.mu.Lock()
	if u.currentVersion == "" && !u.useGitHub {
		if u.dataDir == "" {
			u.mu.Unlock()
			return fmt.Errorf("面板本地未配置 Agent 二进制，请从 GitHub 选择版本推送")
		}
		if err := u.scanLocalAgentBinaries(); err != nil {
			u.mu.Unlock()
			return fmt.Errorf("面板本地未配置可用的 Agent 二进制（%v），请从 GitHub 选择版本推送", err)
		}
		log.Printf("[AgentUpdater] scanned local binaries: version=%s, platforms=%d", u.currentVersion, len(u.platformBinaries))
	}
	if u.currentVersion == "" {
		u.mu.Unlock()
		return fmt.Errorf("no target version set")
	}
	u.mu.Unlock()

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
