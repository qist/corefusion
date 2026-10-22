package system

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	"corefusion/pkg/netutil"
)

// PanelGitHubRepo 是面板（本仓库）的 GitHub 仓库，用于拉取版本列表与发布产物。
const PanelGitHubRepo = "qist/corefusion"

// 面板自更新任务的动作与状态。
const (
	PanelUpdateActionUpdate   = "update"
	PanelUpdateActionRollback = "rollback"

	PanelUpdateStatusPending = "pending"
	PanelUpdateStatusRunning = "running"
	PanelUpdateStatusSuccess = "success"
	PanelUpdateStatusFailed  = "failed"

	// panelUpdateMaxBinaryBytes 限制单个产物的大小，防止异常响应把磁盘写满。
	panelUpdateMaxBinaryBytes = 300 << 20
	// 面板二进制不应小于该值，否则基本可以断定下载到的是错误页面/HTML。
	panelUpdateMinBinaryBytes = 1 << 20
)

// 面板自更新的可预期错误，handler 据此映射成 409 / 422。
var (
	ErrPanelUpdateInProgress     = errors.New("another panel update job is running")
	ErrPanelRollbackUnavailable  = errors.New("no rollback backup available")
	ErrPanelUpdateUnsupported    = errors.New("panel self-update is only supported on linux")
	ErrPanelUpdateChecksumAbsent = errors.New("release asset has no sha256 checksum")
)

// PanelReleaseAsset 是发布产物中的单个文件。
type PanelReleaseAsset struct {
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	DownloadURL string `json:"download_url"`
}

// PanelRelease 是面板版本列表中的一项（仅保留自更新需要的字段）。
type PanelRelease struct {
	Version     string `json:"version"`
	Name        string `json:"name"`
	Prerelease  bool   `json:"prerelease"`
	PublishedAt string `json:"published_at"`
	Notes       string `json:"notes"`
	// Asset 是匹配当前平台（linux/<arch>）的产物；为空表示该版本不含当前平台产物。
	Asset *PanelReleaseAsset `json:"asset,omitempty"`
	// ChecksumSHA256 来自发布资产的 digest（GitHub 新版 API 提供），可能为空。
	ChecksumSHA256 string `json:"checksum_sha256,omitempty"`
	// ChecksumURL 是同名 .sha256 旁挂文件的地址（发布工作流一定会生成）。
	ChecksumURL string `json:"checksum_url,omitempty"`
}

// PanelRollbackInfo 描述本机是否具备回滚能力。
type PanelRollbackInfo struct {
	Available  bool   `json:"available"`
	Version    string `json:"version,omitempty"`
	BinaryPath string `json:"binary_path,omitempty"`
	CreatedAt  string `json:"created_at,omitempty"`
}

// PanelReleaseList 是 GET /system/releases 的响应体。
type PanelReleaseList struct {
	CurrentVersion  string            `json:"current_version"`
	Platform        string            `json:"platform"`
	AssetName       string            `json:"asset_name"`
	LatestVersion   string            `json:"latest_version,omitempty"`
	UpdateAvailable bool              `json:"update_available"`
	Releases        []PanelRelease    `json:"releases"`
	Rollback        PanelRollbackInfo `json:"rollback"`
}

// PanelUpdateService 负责面板自身的版本查询、自更新与回滚。
//
// 设计要点：
//   - 与 Agent 自更新保持一致的安全基线：必须能拿到 sha256 才允许替换二进制；
//   - 替换前把当前二进制备份成 <exe>.bak，回滚直接用它；
//   - 下载/校验/替换在后台任务里执行，HTTP 请求立即返回 202。
type PanelUpdateService struct {
	db         *gorm.DB
	settingSvc *SettingService
	dataDir    string
	version    string
	repo       string

	mu        sync.Mutex // 串行化「启动任务」与「校验单任务在跑」
	restartFn func()

	// 供测试替换
	executableFn func() (string, error)
	httpClientFn func(ctx context.Context) *http.Client
}

// NewPanelUpdateService 创建面板自更新服务。
func NewPanelUpdateService(db *gorm.DB, settingSvc *SettingService, dataDir, version string) *PanelUpdateService {
	return &PanelUpdateService{
		db:           db,
		settingSvc:   settingSvc,
		dataDir:      dataDir,
		version:      version,
		repo:         PanelGitHubRepo,
		restartFn:    restartCurrentProcess,
		executableFn: os.Executable,
	}
}

// SetRestartFunc 覆盖重启实现（默认 re-exec 当前进程）。
func (s *PanelUpdateService) SetRestartFunc(fn func()) {
	if fn != nil {
		s.restartFn = fn
	}
}

// ListReleases 拉取 GitHub 上的面板版本列表，并标注当前平台可用产物。
func (s *PanelUpdateService) ListReleases(ctx context.Context, limit int) (*PanelReleaseList, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	assetName := panelAssetName(runtime.GOARCH)

	url := fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=%d", s.repo, limit)
	var raw []githubRelease
	if err := s.getJSON(ctx, url, &raw); err != nil {
		return nil, err
	}

	out := &PanelReleaseList{
		CurrentVersion: s.version,
		Platform:       runtime.GOOS + "/" + runtime.GOARCH,
		AssetName:      assetName,
		Releases:       make([]PanelRelease, 0, len(raw)),
	}
	for _, r := range raw {
		if r.Draft {
			continue
		}
		item := PanelRelease{
			Version:     r.TagName,
			Name:        r.Name,
			Prerelease:  r.Prerelease,
			PublishedAt: r.PublishedAt,
			Notes:       r.Body,
		}
		for _, a := range r.Assets {
			switch a.Name {
			case assetName:
				size := a.Size
				item.Asset = &PanelReleaseAsset{
					Name:        a.Name,
					Size:        size,
					DownloadURL: a.BrowserDownloadURL,
				}
				item.ChecksumSHA256 = normalizeSHA256(a.Digest)
			case assetName + ".sha256":
				item.ChecksumURL = a.BrowserDownloadURL
			}
		}
		if out.LatestVersion == "" {
			out.LatestVersion = r.TagName
		}
		out.Releases = append(out.Releases, item)
	}
	out.UpdateAvailable = out.LatestVersion != "" && out.LatestVersion != s.version
	out.Rollback = s.RollbackInfo()
	return out, nil
}

// RollbackInfo 读取本机备份二进制信息（<exe>.bak 与 <exe>.bak.json）。
func (s *PanelUpdateService) RollbackInfo() PanelRollbackInfo {
	exe, err := s.currentBinaryPath()
	if err != nil {
		return PanelRollbackInfo{}
	}
	bak := exe + ".bak"
	info, err := os.Stat(bak)
	if err != nil || info.IsDir() {
		return PanelRollbackInfo{}
	}
	out := PanelRollbackInfo{
		Available:  true,
		BinaryPath: bak,
		CreatedAt:  info.ModTime().Format(time.RFC3339),
	}
	if meta, err := readBackupMeta(bak + ".json"); err == nil {
		out.Version = meta.Version
		if meta.CreatedAt != "" {
			out.CreatedAt = meta.CreatedAt
		}
	}
	return out
}

// StartUpdate 创建并异步执行一次自更新任务。
// version 为空时安装最新版本（含预发布，保证 alpha 也能被显式安装）。
func (s *PanelUpdateService) StartUpdate(ctx context.Context, version string) (*entity.PanelUpdateJob, error) {
	if runtime.GOOS != "linux" {
		return nil, ErrPanelUpdateUnsupported
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureNoRunningJob(); err != nil {
		return nil, err
	}
	job := &entity.PanelUpdateJob{
		Action:      PanelUpdateActionUpdate,
		Status:      PanelUpdateStatusPending,
		FromVersion: s.version,
		ToVersion:   strings.TrimSpace(version),
	}
	if err := s.db.Create(job).Error; err != nil {
		return nil, fmt.Errorf("create update job: %w", err)
	}
	go s.runUpdate(job.ID, job.ToVersion)
	return job, nil
}

// StartRollback 创建并异步执行一次回滚任务（用 <exe>.bak 还原）。
func (s *PanelUpdateService) StartRollback(ctx context.Context) (*entity.PanelUpdateJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureNoRunningJob(); err != nil {
		return nil, err
	}
	info := s.RollbackInfo()
	if !info.Available {
		return nil, ErrPanelRollbackUnavailable
	}
	job := &entity.PanelUpdateJob{
		Action:      PanelUpdateActionRollback,
		Status:      PanelUpdateStatusPending,
		FromVersion: s.version,
		ToVersion:   info.Version,
		BackupPath:  info.BinaryPath,
	}
	if err := s.db.Create(job).Error; err != nil {
		return nil, fmt.Errorf("create rollback job: %w", err)
	}
	go s.runRollback(job.ID)
	return job, nil
}

// ListJobs 分页列出历史任务（新→旧）。
func (s *PanelUpdateService) ListJobs(ctx context.Context, limit, offset int) ([]entity.PanelUpdateJob, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	var total int64
	if err := s.db.Model(&entity.PanelUpdateJob{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []entity.PanelUpdateJob
	if err := s.db.Order("id DESC").Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	if items == nil {
		items = []entity.PanelUpdateJob{}
	}
	return items, total, nil
}

// --- 后台任务 ---

func (s *PanelUpdateService) runUpdate(jobID int64, version string) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	job, err := s.beginJob(jobID)
	if err != nil {
		log.Printf("[panel-update] job %d: %v", jobID, err)
		return
	}

	fail := func(phase string, err error) {
		log.Printf("[panel-update] job %d failed at %s: %v", jobID, phase, err)
		s.finishJob(jobID, PanelUpdateStatusFailed, phase, err.Error())
	}

	// 1. 解析目标版本与产物
	s.setPhase(jobID, "resolving")
	release, err := s.resolveRelease(ctx, version)
	if err != nil {
		fail("resolving", err)
		return
	}
	if release.Asset == nil {
		fail("resolving", fmt.Errorf("version %s has no asset for %s/%s", release.Version, runtime.GOOS, runtime.GOARCH))
		return
	}
	job.ToVersion = release.Version
	job.DownloadURL = release.Asset.DownloadURL
	s.db.Model(&entity.PanelUpdateJob{}).Where("id = ?", jobID).Updates(map[string]any{
		"to_version":   release.Version,
		"download_url": release.Asset.DownloadURL,
	})

	// 2. 下载
	s.setPhase(jobID, "downloading")
	tmpPath, err := s.downloadAsset(ctx, release)
	if err != nil {
		fail("downloading", err)
		return
	}
	defer os.Remove(tmpPath)

	// 3. 校验（没有 sha256 就拒绝更新，绝不替换无法校验的二进制）
	s.setPhase(jobID, "verifying")
	expected, err := s.resolveChecksum(ctx, release)
	if err != nil {
		fail("verifying", err)
		return
	}
	s.db.Model(&entity.PanelUpdateJob{}).Where("id = ?", jobID).Update("checksum", expected)
	if err := verifyFileSHA256(tmpPath, expected); err != nil {
		fail("verifying", err)
		return
	}
	if err := verifyELF(tmpPath); err != nil {
		fail("verifying", err)
		return
	}

	// 4. 备份 + 原子替换
	s.setPhase(jobID, "applying")
	bakPath, err := s.applyBinary(tmpPath, release.Version, job.FromVersion, jobID)
	if err != nil {
		fail("applying", err)
		return
	}
	s.db.Model(&entity.PanelUpdateJob{}).Where("id = ?", jobID).Update("backup_path", bakPath)

	// 5. 重启（进程会退出，finished_at 尽量先落库）
	s.finishJob(jobID, PanelUpdateStatusSuccess, "restarting", "")
	s.scheduleRestart()
}

func (s *PanelUpdateService) runRollback(jobID int64) {
	job, err := s.beginJob(jobID)
	if err != nil {
		log.Printf("[panel-rollback] job %d: %v", jobID, err)
		return
	}
	fail := func(phase string, err error) {
		log.Printf("[panel-rollback] job %d failed at %s: %v", jobID, phase, err)
		s.finishJob(jobID, PanelUpdateStatusFailed, phase, err.Error())
	}

	s.setPhase(jobID, "applying")
	bakPath := job.BackupPath
	if bakPath == "" {
		bakPath = s.backupPath()
	}
	if info, err := os.Stat(bakPath); err != nil || info.IsDir() {
		fail("applying", ErrPanelRollbackUnavailable)
		return
	}
	if err := verifyELF(bakPath); err != nil {
		fail("applying", fmt.Errorf("backup binary is not a valid ELF: %w", err))
		return
	}
	if err := s.replaceWith(bakPath); err != nil {
		fail("applying", err)
		return
	}
	s.finishJob(jobID, PanelUpdateStatusSuccess, "restarting", "")
	s.scheduleRestart()
}

func (s *PanelUpdateService) beginJob(jobID int64) (*entity.PanelUpdateJob, error) {
	now := time.Now()
	if err := s.db.Model(&entity.PanelUpdateJob{}).Where("id = ?", jobID).Updates(map[string]any{
		"status":     PanelUpdateStatusRunning,
		"started_at": now,
	}).Error; err != nil {
		return nil, fmt.Errorf("mark job running: %w", err)
	}
	var job entity.PanelUpdateJob
	if err := s.db.First(&job, jobID).Error; err != nil {
		return nil, fmt.Errorf("load job: %w", err)
	}
	return &job, nil
}

func (s *PanelUpdateService) setPhase(jobID int64, phase string) {
	s.db.Model(&entity.PanelUpdateJob{}).Where("id = ?", jobID).Update("phase", phase)
}

func (s *PanelUpdateService) finishJob(jobID int64, status, phase, errMsg string) {
	now := time.Now()
	s.db.Model(&entity.PanelUpdateJob{}).Where("id = ?", jobID).Updates(map[string]any{
		"status":        status,
		"phase":         phase,
		"error_message": errMsg,
		"finished_at":   now,
	})
}

func (s *PanelUpdateService) scheduleRestart() {
	go func() {
		time.Sleep(500 * time.Millisecond)
		if s.restartFn != nil {
			s.restartFn()
		}
	}()
}

func (s *PanelUpdateService) ensureNoRunningJob() error {
	var count int64
	// 进程重启/崩溃可能留下 pending|running 的僵尸任务；超过 30 分钟视为失效，不再阻塞新任务。
	cutoff := time.Now().Add(-30 * time.Minute)
	if err := s.db.Model(&entity.PanelUpdateJob{}).
		Where("status IN ? AND updated_at > ?", []string{PanelUpdateStatusPending, PanelUpdateStatusRunning}, cutoff).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return ErrPanelUpdateInProgress
	}
	return nil
}

// --- 版本查询 ---

type githubRelease struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	Body        string `json:"body"`
	Draft       bool   `json:"draft"`
	Prerelease  bool   `json:"prerelease"`
	PublishedAt string `json:"published_at"`
	Assets      []struct {
		Name               string `json:"name"`
		Size               int64  `json:"size"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Digest             string `json:"digest"`
	} `json:"assets"`
}

// resolveRelease 按版本号取发布信息；version 为空时取最新（含预发布）。
func (s *PanelUpdateService) resolveRelease(ctx context.Context, version string) (*PanelRelease, error) {
	version = strings.TrimSpace(version)
	var url string
	if version == "" {
		url = fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=1", s.repo)
		var list []githubRelease
		if err := s.getJSON(ctx, url, &list); err != nil {
			return nil, err
		}
		if len(list) == 0 {
			return nil, errors.New("repository has no published release")
		}
		return s.convertRelease(list[0]), nil
	}
	url = fmt.Sprintf("https://api.github.com/repos/%s/releases/tags/%s", s.repo, version)
	var r githubRelease
	if err := s.getJSON(ctx, url, &r); err != nil {
		return nil, err
	}
	if r.Draft {
		return nil, fmt.Errorf("release %s is a draft", version)
	}
	return s.convertRelease(r), nil
}

func (s *PanelUpdateService) convertRelease(r githubRelease) *PanelRelease {
	assetName := panelAssetName(runtime.GOARCH)
	out := &PanelRelease{
		Version:     r.TagName,
		Name:        r.Name,
		Prerelease:  r.Prerelease,
		PublishedAt: r.PublishedAt,
		Notes:       r.Body,
	}
	for _, a := range r.Assets {
		switch a.Name {
		case assetName:
			out.Asset = &PanelReleaseAsset{Name: a.Name, Size: a.Size, DownloadURL: a.BrowserDownloadURL}
			out.ChecksumSHA256 = normalizeSHA256(a.Digest)
		case assetName + ".sha256":
			out.ChecksumURL = a.BrowserDownloadURL
		}
	}
	return out
}

// panelAssetName 返回当前架构对应的面板产物名（发布工作流产物命名）。
func panelAssetName(goarch string) string {
	return "corefusion-linux-" + goarch
}

// --- 网络与校验 ---

func (s *PanelUpdateService) getJSON(ctx context.Context, url string, dst any) error {
	client := s.client(ctx)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("github api: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("github api %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(dst); err != nil {
		return fmt.Errorf("decode github response: %w", err)
	}
	return nil
}

func (s *PanelUpdateService) downloadAsset(ctx context.Context, release *PanelRelease) (string, error) {
	tmp, err := os.CreateTemp(s.dataDir, "panel-update-*")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := func(err error) (string, error) {
		tmp.Close()
		os.Remove(tmpPath)
		return "", err
	}
	if err := tmp.Chmod(0o600); err != nil {
		return cleanup(fmt.Errorf("chmod temp file: %w", err))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, release.Asset.DownloadURL, nil)
	if err != nil {
		return cleanup(err)
	}
	resp, err := s.client(ctx).Do(req)
	if err != nil {
		return cleanup(fmt.Errorf("download %s: %w", release.Asset.DownloadURL, err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return cleanup(fmt.Errorf("download %s: HTTP %d", release.Asset.DownloadURL, resp.StatusCode))
	}
	n, err := io.Copy(tmp, io.LimitReader(resp.Body, panelUpdateMaxBinaryBytes))
	if err != nil {
		return cleanup(fmt.Errorf("write temp file: %w", err))
	}
	if n < panelUpdateMinBinaryBytes {
		return cleanup(fmt.Errorf("downloaded file is too small (%d bytes), likely not a binary", n))
	}
	if err := tmp.Sync(); err != nil {
		return cleanup(fmt.Errorf("sync temp file: %w", err))
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("close temp file: %w", err)
	}
	return tmpPath, nil
}

// resolveChecksum 取期望的 sha256：优先资产 digest，其次同名 .sha256 文件。
func (s *PanelUpdateService) resolveChecksum(ctx context.Context, release *PanelRelease) (string, error) {
	if c := normalizeSHA256(release.ChecksumSHA256); c != "" {
		return c, nil
	}
	if release.ChecksumURL == "" {
		return "", ErrPanelUpdateChecksumAbsent
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, release.ChecksumURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := s.client(ctx).Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch checksum: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch checksum: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "", fmt.Errorf("read checksum: %w", err)
	}
	for _, field := range strings.Fields(string(body)) {
		if c := normalizeSHA256(field); c != "" {
			return c, nil
		}
	}
	return "", ErrPanelUpdateChecksumAbsent
}

func (s *PanelUpdateService) client(ctx context.Context) *http.Client {
	if s.httpClientFn != nil {
		return s.httpClientFn(ctx)
	}
	var accel, proxy string
	if s.settingSvc != nil {
		accel = s.settingSvc.GetActiveGitHubDomain(ctx)
		proxy = s.settingSvc.GetGitHubProxyURL(ctx)
	}
	return netutil.GitHubHTTPClientWithProxy(10*time.Minute, accel, proxy)
}

// --- 文件操作 ---

func (s *PanelUpdateService) currentBinaryPath() (string, error) {
	fn := s.executableFn
	if fn == nil {
		fn = os.Executable
	}
	exe, err := fn()
	if err != nil {
		return "", fmt.Errorf("get executable path: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return exe, nil
}

func (s *PanelUpdateService) backupPath() string {
	exe, err := s.currentBinaryPath()
	if err != nil {
		return ""
	}
	return exe + ".bak"
}

// applyBinary 备份当前二进制，并用 newBinary 原子替换它。
// 返回备份文件路径。
func (s *PanelUpdateService) applyBinary(newBinary, newVersion, fromVersion string, jobID int64) (string, error) {
	exe, err := s.currentBinaryPath()
	if err != nil {
		return "", err
	}
	bak := exe + ".bak"
	if err := copyFile(exe, bak, 0o755); err != nil {
		return "", fmt.Errorf("backup current binary: %w", err)
	}
	meta := panelBackupMeta{
		Version:    fromVersion,
		CreatedAt:  time.Now().Format(time.RFC3339),
		JobID:      jobID,
		ReplacedBy: newVersion,
	}
	if data, err := json.MarshalIndent(meta, "", "  "); err == nil {
		_ = os.WriteFile(bak+".json", data, 0o600)
	}
	if err := s.replaceWith(newBinary); err != nil {
		return "", err
	}
	return bak, nil
}

// replaceWith 把 src 的内容原子地写到当前可执行文件路径上。
// 临时文件必须建在可执行文件同目录，保证 rename 不跨文件系统。
func (s *PanelUpdateService) replaceWith(src string) error {
	exe, err := s.currentBinaryPath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(exe)
	tmp, err := os.CreateTemp(dir, ".panel-update-*")
	if err != nil {
		return fmt.Errorf("在 %s 创建临时文件失败（面板进程需要对该目录有写权限）: %w", dir, err)
	}
	tmpPath := tmp.Name()
	removeTmp := true
	defer func() {
		if removeTmp {
			os.Remove(tmpPath)
		}
	}()

	in, err := os.Open(src)
	if err != nil {
		tmp.Close()
		return fmt.Errorf("open new binary: %w", err)
	}
	defer in.Close()
	if _, err := io.Copy(tmp, io.LimitReader(in, panelUpdateMaxBinaryBytes)); err != nil {
		tmp.Close()
		return fmt.Errorf("copy new binary: %w", err)
	}
	if err := tmp.Chmod(0o755); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod new binary: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync new binary: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close new binary: %w", err)
	}
	if err := os.Rename(tmpPath, exe); err != nil {
		return fmt.Errorf("replace %s: %w", exe, err)
	}
	removeTmp = false
	return nil
}

type panelBackupMeta struct {
	Version    string `json:"version"`
	CreatedAt  string `json:"created_at"`
	JobID      int64  `json:"job_id"`
	ReplacedBy string `json:"replaced_by"`
}

func readBackupMeta(path string) (panelBackupMeta, error) {
	var meta panelBackupMeta
	data, err := os.ReadFile(path)
	if err != nil {
		return meta, err
	}
	err = json.Unmarshal(data, &meta)
	return meta, err
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, io.LimitReader(in, panelUpdateMaxBinaryBytes)); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Chmod(tmp, mode); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}

func verifyFileSHA256(path, expected string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("read file: %w", err)
	}
	actual := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expected, actual)
	}
	return nil
}

// verifyELF 粗校验产物确实是 Linux ELF 可执行文件（防止把 HTML 错误页装上）。
func verifyELF(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	var magic [4]byte
	if _, err := io.ReadFull(f, magic[:]); err != nil {
		return fmt.Errorf("read magic: %w", err)
	}
	if magic[0] != 0x7f || magic[1] != 'E' || magic[2] != 'L' || magic[3] != 'F' {
		return errors.New("downloaded file is not an ELF executable")
	}
	return nil
}

func normalizeSHA256(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(strings.ToLower(s), "sha256:")
	s = strings.TrimSpace(s)
	if len(s) != 64 {
		return ""
	}
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return ""
		}
	}
	return s
}

// restartCurrentProcess 默认重启实现：re-exec 自身后退出。
func restartCurrentProcess() {
	exe, err := os.Executable()
	if err != nil {
		os.Exit(0)
	}
	cmd := exec.Command(exe, os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Start()
	os.Exit(0)
}
