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

	// 失败重试策略
	// taskTimeout 用于判定"推进中但迟迟没有结果"的任务已卡死：
	// Agent 侧下载超时为 10 分钟，这里留出替换/重启的余量。
	taskTimeout  time.Duration
	maxAttempts  int             // 含首次推送在内的最大尝试次数
	retryBackoff []time.Duration // 第 n 次失败后的等待时间
	monitorEvery time.Duration   // 卡死看护 + 重试调度周期
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
	// Attempts 是本次更新已推送的次数（1 = 首次推送）。
	Attempts int `json:"attempts"`
	// NextRetryAt 非空表示失败后已排入重试队列。
	NextRetryAt *time.Time `json:"next_retry_at,omitempty"`
}

// inFlight 表示任务已经推给 Agent、正在等待结果。
func (t *UpdateTask) inFlight() bool {
	switch t.Status {
	case UpdateStatusPending, UpdateStatusDownloading, UpdateStatusVerifying,
		UpdateStatusReplacing, UpdateStatusRestarting:
		return true
	}
	return false
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
		taskTimeout:      12 * time.Minute,
		maxAttempts:      3,
		retryBackoff:     []time.Duration{30 * time.Second, 2 * time.Minute},
		monitorEvery:     30 * time.Second,
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
//
// 管理动作会把重试计数重置为第 1 次尝试。
func (u *AgentUpdater) UpdateServer(ctx context.Context, serverID int64) error {
	return u.push(ctx, serverID, 1)
}

// push 把 UpdateAgent 指令推给指定节点，并登记一个带 CommandID 的结果等待者。
// attempt 为本次是第几次尝试（1 = 首次）。
func (u *AgentUpdater) push(ctx context.Context, serverID int64, attempt int) error {
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

	// Check if already updating。只有"确实还在推进且未超时"的任务才阻塞新推送：
	// 以前只要状态还停在 downloading 就永久拒绝，导致失败/卡死的任务把重试也堵死。
	if existing, ok := u.pendingUpdates[serverID]; ok && existing.inFlight() {
		age := time.Since(existing.StartedAt)
		if age < u.taskTimeout {
			u.mu.Unlock()
			return fmt.Errorf("server %d already has an update in progress (status=%s, %v elapsed)", serverID, existing.Status, age.Truncate(time.Second))
		}
		log.Printf("[AgentUpdater] server %d: previous update task stalled for %v, superseding it",
			serverID, age.Truncate(time.Second))
	}

	// Check if server is online
	if !u.nodeMgr.IsOnline(serverID) {
		u.mu.Unlock()
		return fmt.Errorf("server %d is offline, cannot push update", serverID)
	}

	// Skip if already on target version。
	// 若这里还有残留的失败任务（例如上一次确认丢失、重试时发现节点其实已到位），
	// 直接收尾为 done，避免 UI 上一直挂着一条"失败"记录。
	if server.AgentVersion == u.currentVersion {
		if existing, ok := u.pendingUpdates[serverID]; ok {
			existing.Status = UpdateStatusDone
			existing.Error = ""
			existing.NextRetryAt = nil
			now := time.Now()
			existing.CompletedAt = &now
		}
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
		Attempts:    attempt,
	}
	u.pendingUpdates[serverID] = task

	// Send update command via PanelCommand stream.
	// 必须带 CommandId：Agent 的失败回执靠 TaskId 与面板的等待者对应，
	// 以前不带 CommandId，Agent 回了 TASK_FAILED 也会被面板按空 ID 丢弃。
	cmdID := generateCommandID()
	resultCh := u.nodeMgr.RegisterTaskResultWaiter(cmdID)
	cmd := &agentpb.PanelCommand{
		CommandId: cmdID,
		Command: &agentpb.PanelCommand_UpdateAgent{
			UpdateAgent: &agentpb.UpdateAgentCommand{
				Version:     version,
				DownloadUrl: downloadURL,
				Checksum:    checksum,
			},
		},
	}

	if err := u.nodeMgr.SendCommand(ctx, serverID, cmd); err != nil {
		u.nodeMgr.UnregisterTaskResultWaiter(cmdID)
		task.Status = UpdateStatusFailed
		task.Error = err.Error()
		now := time.Now()
		task.CompletedAt = &now
		u.scheduleRetryLocked(task)
		u.mu.Unlock()
		return fmt.Errorf("send update command to server %d: %w", serverID, err)
	}

	task.Status = UpdateStatusDownloading
	osName, arch := u.nodeMgr.GetServerPlatform(serverID)
	log.Printf("[AgentUpdater] update pushed to server %d (attempt %d/%d): %s → %s (platform=%s-%s)",
		serverID, attempt, u.maxAttempts, task.FromVersion, task.ToVersion, osName, arch)

	u.mu.Unlock()

	// 等 Agent 的失败回执（成功回执通常收不到：Agent 替换二进制后立刻重启进程）。
	go u.watchUpdateResult(resultCh, cmdID, serverID, task)

	return nil
}

// watchUpdateResult 等待 Agent 对本条 UpdateAgent 指令的回执。
// 只处理失败回执：成功回执通常收不到（Agent 替换二进制后立刻重启进程），
// 成功由重连/心跳上报的新版本号确认（OnAgentReconnect）。
func (u *AgentUpdater) watchUpdateResult(resultCh chan *agentpb.TaskResult, cmdID string, serverID int64, task *UpdateTask) {
	timer := time.NewTimer(u.taskTimeout)
	defer timer.Stop()
	defer u.nodeMgr.UnregisterTaskResultWaiter(cmdID)

	select {
	case res := <-resultCh:
		if res == nil || res.Status != agentpb.TaskStatus_TASK_FAILED {
			return // 成功回执同样交给 OnAgentReconnect 确认
		}
		log.Printf("[AgentUpdater] server %d reported update failure: %s", serverID, res.Error)
		u.failTask(serverID, task, "agent: "+res.Error)
	case <-timer.C:
		// 超时交给 monitorPendingUpdates 统一收割（避免两处写同一任务）
	}
}

// failTask 把指定任务标记为失败并排入重试队列。
// 只有 pendingUpdates 里仍是同一个任务对象时才生效（防止覆盖后续新推送）。
func (u *AgentUpdater) failTask(serverID int64, task *UpdateTask, reason string) {
	u.mu.Lock()
	defer u.mu.Unlock()

	cur, ok := u.pendingUpdates[serverID]
	if !ok || cur != task || !cur.inFlight() {
		return
	}
	now := time.Now()
	cur.Status = UpdateStatusFailed
	cur.Error = reason
	cur.CompletedAt = &now
	u.scheduleRetryLocked(cur)
}

// scheduleRetryLocked 按当前尝试次数决定"稍后重试"还是"放弃"。调用方必须持有 u.mu。
func (u *AgentUpdater) scheduleRetryLocked(task *UpdateTask) {
	if task.Attempts >= u.maxAttempts {
		task.NextRetryAt = nil
		log.Printf("[AgentUpdater] server %d: giving up after %d attempt(s), last error: %s",
			task.ServerID, task.Attempts, task.Error)
		return
	}
	at := time.Now().Add(u.retryDelay(task.Attempts))
	task.NextRetryAt = &at
	log.Printf("[AgentUpdater] server %d: retry %d/%d scheduled in %v (last error: %s)",
		task.ServerID, task.Attempts+1, u.maxAttempts, u.retryDelay(task.Attempts), task.Error)
}

// retryDelay 返回第 attempt 次失败后的等待时间（attempt 从 1 开始）。
func (u *AgentUpdater) retryDelay(attempt int) time.Duration {
	if len(u.retryBackoff) == 0 {
		return 30 * time.Second
	}
	idx := attempt - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(u.retryBackoff) {
		idx = len(u.retryBackoff) - 1
	}
	return u.retryBackoff[idx]
}

// monitorPendingUpdates 是更新任务的看护循环：
//  1. 推进中但超过 taskTimeout 仍无结果的任务判定为卡死 → failed + 排重试；
//  2. 到点的失败任务重新推送（最多 maxAttempts 次）；
//  3. 收尾校验：节点实际版本已等于目标版本的任务直接判成功（回执可能丢失）。
func (u *AgentUpdater) monitorPendingUpdates(ctx context.Context) {
	type retryJob struct {
		serverID int64
		attempt  int
	}

	u.mu.Lock()
	now := time.Now()
	var toRetry []retryJob
	var toVerify []*UpdateTask
	for _, t := range u.pendingUpdates {
		switch {
		case t.inFlight() && now.Sub(t.StartedAt) > u.taskTimeout:
			stalled := t.Status
			t.Status = UpdateStatusFailed
			t.Error = fmt.Sprintf("no result from agent within %v (task stalled in %s)", u.taskTimeout, stalled)
			completed := now
			t.CompletedAt = &completed
			toVerify = append(toVerify, t)
			u.scheduleRetryLocked(t)
		case t.Status == UpdateStatusFailed && t.NextRetryAt != nil && !now.Before(*t.NextRetryAt):
			t.NextRetryAt = nil
			toRetry = append(toRetry, retryJob{serverID: t.ServerID, attempt: t.Attempts + 1})
		}
	}
	u.mu.Unlock()

	// 收尾校验：版本已到位但没等到确认（心跳/重连与 TaskResult 都可能错开）。
	for _, t := range toVerify {
		server, err := u.serverRepo.GetByID(ctx, t.ServerID)
		if err != nil || server.AgentVersion != t.ToVersion {
			continue
		}
		u.mu.Lock()
		if cur, ok := u.pendingUpdates[t.ServerID]; ok && cur == t {
			cur.Status = UpdateStatusDone
			cur.Error = ""
			cur.NextRetryAt = nil
		}
		u.mu.Unlock()
		log.Printf("[AgentUpdater] server %d already reports %s, marking update done", t.ServerID, t.ToVersion)
	}

	for _, job := range toRetry {
		if err := u.push(ctx, job.serverID, job.attempt); err != nil {
			log.Printf("[AgentUpdater] server %d retry %d/%d failed to dispatch: %v",
				job.serverID, job.attempt, u.maxAttempts, err)
			u.settleDispatchFailure(ctx, job.serverID, job.attempt)
		}
	}
}

// settleDispatchFailure 处理"重试没能派发出去"的情况：
// 节点实际已跑到目标版本（确认丢失）→ 直接判成功；
// 其它原因（离线/平台无二进制等）→ 记一次尝试，退避后再试，用满 maxAttempts 后放弃。
func (u *AgentUpdater) settleDispatchFailure(ctx context.Context, serverID int64, attempt int) {
	u.mu.Lock()
	task, ok := u.pendingUpdates[serverID]
	u.mu.Unlock()
	if !ok {
		return
	}

	if server, err := u.serverRepo.GetByID(ctx, serverID); err == nil && server.AgentVersion == task.ToVersion {
		u.mu.Lock()
		if cur, ok := u.pendingUpdates[serverID]; ok && cur == task {
			cur.Status = UpdateStatusDone
			cur.Error = ""
			cur.NextRetryAt = nil
			now := time.Now()
			cur.CompletedAt = &now
		}
		u.mu.Unlock()
		log.Printf("[AgentUpdater] server %d already reports %s, marking update done", serverID, task.ToVersion)
		return
	}

	u.mu.Lock()
	defer u.mu.Unlock()
	if cur, ok := u.pendingUpdates[serverID]; ok && cur == task && cur.Status == UpdateStatusFailed {
		cur.Attempts = attempt
		u.scheduleRetryLocked(cur)
	}
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
	targeted := 0

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

		targeted++
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
		return fmt.Errorf("%d/%d updates failed", len(errs), targeted)
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
		task.Error = "" // 之前失败/不匹配留下的错误信息必须清掉，否则 UI 上"成功但带错误"
		task.NextRetryAt = nil
		now := time.Now()
		task.CompletedAt = &now
		log.Printf("[AgentUpdater] server %d updated successfully: %s → %s",
			serverID, task.FromVersion, task.ToVersion)
	} else {
		task.Status = UpdateStatusFailed
		task.Error = fmt.Sprintf("version mismatch: expected %s, got %s", task.ToVersion, reportedVersion)
		now := time.Now()
		task.CompletedAt = &now
		u.scheduleRetryLocked(task)
		log.Printf("[AgentUpdater] server %d update verification failed: expected %s, got %s",
			serverID, task.ToVersion, reportedVersion)
	}
}

// RunLoop 是更新任务的后台维护循环，务必在面板启动时拉起：
//   - monitorEvery 周期：收割卡死任务 + 到点重试（失败自动重试，不需要人工再点）；
//   - interval 周期：把仍落后于目标版本的在线节点再推一遍（长周期兜底）。
//
// 注意：以前的 RunLoop 只在启动时 currentVersion 非空才运行，而版本是管理员
// 点推送时才设置的，所以它开机即退出；它也从没被调用过。
func (u *AgentUpdater) RunLoop(ctx context.Context) {
	monitor := time.NewTicker(u.monitorEvery)
	defer monitor.Stop()
	reconcile := time.NewTicker(u.interval)
	defer reconcile.Stop()

	log.Printf("[AgentUpdater] maintenance loop started (monitor=%v, reconcile=%v, timeout=%v, maxAttempts=%d)",
		u.monitorEvery, u.interval, u.taskTimeout, u.maxAttempts)

	for {
		select {
		case <-ctx.Done():
			log.Println("[AgentUpdater] stopped")
			return
		case <-monitor.C:
			u.monitorPendingUpdates(ctx)
			// 已结束的任务保留 1 小时供排查，超时清理，避免列表越积越多
			u.CleanupCompleted(time.Hour)
		case <-reconcile.C:
			if u.GetCurrentVersion() == "" {
				continue
			}
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
