package georule

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
	"corefusion/pkg/netutil"
)

// NodeOperator handles remote node operations for geo rule files.
type NodeOperator interface {
	DownloadGeoRule(ctx context.Context, serverID int64, fileName, downloadURL, accelDomain, proxyURL string) (success bool, fileSize int64, errMsg string, err error)
	DeleteGeoRule(ctx context.Context, serverID int64, fileName string) (success bool, errMsg string, err error)
	IsLocalServer(serverID int64) bool
}

// SettingStore provides access to system settings.
type SettingStore interface {
	GetByKey(ctx context.Context, key string) (string, error)
}

// NodeLister provides the list of all server IDs (for push-to-all).
type NodeLister interface {
	ListAllServerIDs(ctx context.Context) ([]int64, error)
}

// GitHubRelease represents a GitHub release with assets.
type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		Size               int64  `json:"size"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// ReleaseAsset is a downloadable file from a release.
type ReleaseAsset struct {
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	DownloadURL string `json:"download_url"`
}

// Service handles geo rule file business logic.
// Geo rules are global (server_id=0) — all nodes share the same rule files.
// The panel downloads files to data/rules/, then pushes them to all nodes
// via the DownloadGeoRule command (Agent downloads from panel's HTTP endpoint).
type Service struct {
	sourceRepo   *repository.RuleSourceRepository
	fileRepo     *repository.GeoRuleRepository
	dataDir      string
	settingSvc   SettingStore
	nodeOp       NodeOperator
	nodeLister NodeLister
}

// NewService creates a new geo rule service.
func NewService(sourceRepo *repository.RuleSourceRepository, fileRepo *repository.GeoRuleRepository, dataDir string, settingSvc SettingStore) *Service {
	return &Service{
		sourceRepo: sourceRepo,
		fileRepo:   fileRepo,
		dataDir:    dataDir,
		settingSvc: settingSvc,
	}
}

// SetNodeOperator sets the node operator for remote node downloads.
func (s *Service) SetNodeOperator(op NodeOperator) {
	s.nodeOp = op
}

// SetNodeLister sets the node lister for push-to-all operations.
func (s *Service) SetNodeLister(lister NodeLister) {
	s.nodeLister = lister
}

// githubAccelDomain reads the active GitHub acceleration domain from settings.
func (s *Service) githubAccelDomain(ctx context.Context) string {
	if s.settingSvc == nil {
		return ""
	}
	val, err := s.settingSvc.GetByKey(ctx, "github.acceleration")
	if err != nil || val == "" {
		return ""
	}
	var cfg struct {
		Enabled bool `json:"enabled"`
		Domains []struct {
			URL     string `json:"url"`
			Enabled bool   `json:"enabled"`
		} `json:"domains"`
	}
	if json.Unmarshal([]byte(val), &cfg) != nil || !cfg.Enabled {
		return ""
	}
	for _, d := range cfg.Domains {
		if d.Enabled && d.URL != "" {
			return d.URL
		}
	}
	return ""
}

// githubProxyURL reads the HTTP proxy URL from GitHub acceleration settings.
func (s *Service) githubProxyURL(ctx context.Context) string {
	if s.settingSvc == nil {
		return ""
	}
	val, err := s.settingSvc.GetByKey(ctx, "github.acceleration")
	if err != nil || val == "" {
		return ""
	}
	var cfg struct {
		ProxyURL string `json:"proxy_url"`
	}
	if json.Unmarshal([]byte(val), &cfg) != nil {
		return ""
	}
	return cfg.ProxyURL
}

// httpClientWithAccel returns an HTTP client with GitHub acceleration applied.
func (s *Service) httpClientWithAccel(ctx context.Context) *http.Client {
	return netutil.GitHubHTTPClient(120*time.Second, s.githubAccelDomain(ctx))
}

// SeedDefaults creates the default Loyalsoldier source if not exists.
// Geo rules are global (server_id=0), shared by all nodes.
// Default files geoip.dat and geosite.dat are pre-selected with direct download
// URLs so they are available immediately without depending on the GitHub API.
func (s *Service) SeedDefaults(ctx context.Context, serverID int64) error {
	// serverID is ignored — geo rules are always global (server_id=0)
	sources, err := s.sourceRepo.ListByServer(ctx, 0)
	if err != nil {
		return err
	}
	if len(sources) > 0 {
		return nil // already seeded
	}

	os.MkdirAll(filepath.Join(s.dataDir, "rules"), 0750)

	src := &entity.RuleSource{
		ServerID: 0, // global
		Name:     "Loyalsoldier v2ray-rules-dat",
		RepoURL:  "Loyalsoldier/v2ray-rules-dat",
		Enabled:  false,
		SyncCron: "0 3 * * *",
		Status:   "idle",
	}
	if err := s.sourceRepo.Create(ctx, src); err != nil {
		return err
	}

	// Pre-select default files with direct download URLs.
	// This ensures geoip.dat and geosite.dat are available in the UI and
	// can be synced immediately without depending on the GitHub API.
	defaultFiles := []struct {
		name        string
		downloadURL string
	}{
		{"geoip.dat", "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat"},
		{"geosite.dat", "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat"},
	}
	for _, df := range defaultFiles {
		rule := &entity.GeoRuleFile{
			ServerID:    0, // global
			SourceID:    src.ID,
			Name:        df.name,
			FileName:    df.name,
			DownloadURL: df.downloadURL,
			AutoSync:    true,
			Status:      "available",
		}
		if err := s.fileRepo.Create(ctx, rule); err != nil {
			log.Printf("seed: failed to create rule %s: %v", df.name, err)
		} else {
			log.Printf("seed: added default rule %s", df.name)
		}
	}

	// Asynchronously fetch the latest release tag to update version and
	// precise download URLs from GitHub. This is optional — the direct URLs
	// above are sufficient for downloading.
	go func() {
		time.Sleep(2 * time.Second) // Wait for server to be ready
		ctx := context.Background()
		assets, tag, err := s.FetchReleaseAssets(ctx, src.ID)
		if err != nil {
			log.Printf("seed: failed to fetch assets (using direct URLs as fallback): %v", err)
			return
		}
		assetMap := make(map[string]ReleaseAsset)
		for _, a := range assets {
			assetMap[a.Name] = a
		}
		// Update file records with precise download URLs and version from GitHub
		files, _ := s.fileRepo.ListBySource(ctx, src.ID)
		for _, f := range files {
			if asset, ok := assetMap[f.FileName]; ok {
				f.DownloadURL = asset.DownloadURL
				f.FileSize = asset.Size
				f.Version = tag
				s.fileRepo.Update(ctx, &f)
			}
		}
		src.LastSyncTag = tag
		s.sourceRepo.Update(nil, src)
	}()

	return nil
}

// EnsureDefaultsAndSync ensures default geo rule sources and files exist for the
// given server, then triggers an async sync to download them. This is intended
// to be called after a core binary is installed so that geoip.dat/geosite.dat
// are available for the core to use. Geo rules are global, so the download
// happens once on the panel and is then pushed to all nodes.
func (s *Service) EnsureDefaultsAndSync(serverID int64) {
	// Ensure default source and files exist (idempotent) — global
	if err := s.SeedDefaults(context.Background(), 0); err != nil {
		log.Printf("EnsureDefaultsAndSync: SeedDefaults failed: %v", err)
	}
	// Trigger async sync to download the files to panel's data/rules/
	s.StartSyncAll()
	// After download, push to all nodes (including the one that triggered this)
	go func() {
		time.Sleep(5 * time.Second) // give download time to complete
		if err := s.SyncToAllNodes(context.Background()); err != nil {
			log.Printf("EnsureDefaultsAndSync: push to all nodes failed: %v", err)
		}
	}()
}

// --- Rule Source CRUD ---

func (s *Service) ListSources(ctx context.Context) ([]entity.RuleSource, error) {
	return s.sourceRepo.List(ctx)
}

// ListSourcesByServer lists rule sources for a specific node server.
func (s *Service) ListSourcesByServer(ctx context.Context, serverID int64) ([]entity.RuleSource, error) {
	return s.sourceRepo.ListByServer(ctx, serverID)
}

func (s *Service) CreateSource(ctx context.Context, serverID int64, name, repoURL, syncCron string) (*entity.RuleSource, error) {
	// serverID ignored — geo rules are always global
	src := &entity.RuleSource{
		ServerID: 0, // global
		Name:    name,
		RepoURL: repoURL,
		Enabled: false,
		SyncCron: syncCron,
		Status:  "idle",
	}
	if err := s.sourceRepo.Create(ctx, src); err != nil {
		return nil, err
	}

	// If it's a direct URL (starts with http), auto-create a rule file
	if strings.HasPrefix(repoURL, "http://") || strings.HasPrefix(repoURL, "https://") {
		fileName := filepath.Base(repoURL)
		if fileName == "" || fileName == "." || fileName == "/" {
			fileName = name
		}
		rule := &entity.GeoRuleFile{
			ServerID:    serverID,
			SourceID:    src.ID,
			Name:        fileName,
			FileName:    fileName,
			DownloadURL: repoURL,
			AutoSync:    true,
			Status:      "available",
		}
		s.fileRepo.Create(ctx, rule)
	}

	return src, nil
}

func (s *Service) UpdateSource(ctx context.Context, id int64, name, repoURL, syncCron string, enabled bool) (*entity.RuleSource, error) {
	src, err := s.sourceRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	src.Name = name
	src.RepoURL = repoURL
	src.SyncCron = syncCron
	src.Enabled = enabled
	if err := s.sourceRepo.Update(ctx, src); err != nil {
		return nil, err
	}
	return src, nil
}

func (s *Service) UpdateSourceSchedule(ctx context.Context, id int64, enabled bool, syncCron string) (*entity.RuleSource, error) {
	src, err := s.sourceRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	src.Enabled = enabled
	src.SyncCron = syncCron
	if err := s.sourceRepo.Update(ctx, src); err != nil {
		return nil, err
	}
	return src, nil
}

func (s *Service) DeleteSource(ctx context.Context, id int64) error {
	// Delete files from disk first
	files, _ := s.fileRepo.ListBySource(ctx, id)
	for _, f := range files {
		os.Remove(filepath.Join(s.dataDir, "rules", f.FileName))
	}
	s.fileRepo.DeleteBySource(ctx, id)
	return s.sourceRepo.Delete(ctx, id)
}

// --- Fetch Release Assets from GitHub ---

// FetchGitHubReleaseAssets fetches the latest release assets for any GitHub repo URL.
// Unlike FetchReleaseAssets, this does not require a saved source.
func (s *Service) FetchGitHubReleaseAssets(ctx context.Context, repoURL string) ([]ReleaseAsset, string, error) {
	apiURL := netutil.AccelerateGitHubURL(
		fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repoURL),
		s.githubAccelDomain(ctx),
	)
	req, _ := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := s.httpClientWithAccel(ctx).Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("fetch releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, "", fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var release GitHubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return nil, "", fmt.Errorf("parse response: %w", err)
	}

	var assets []ReleaseAsset
	for _, a := range release.Assets {
		assets = append(assets, ReleaseAsset{
			Name:        a.Name,
			Size:        a.Size,
			DownloadURL: a.BrowserDownloadURL,
		})
	}
	return assets, release.TagName, nil
}

func (s *Service) FetchReleaseAssets(ctx context.Context, sourceID int64) ([]ReleaseAsset, string, error) {
	src, err := s.sourceRepo.GetByID(ctx, sourceID)
	if err != nil {
		return nil, "", err
	}

	apiURL := netutil.AccelerateGitHubURL(
		fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", src.RepoURL),
		s.githubAccelDomain(ctx),
	)
	req, _ := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := s.httpClientWithAccel(ctx).Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("fetch releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, "", fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var release GitHubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return nil, "", fmt.Errorf("parse response: %w", err)
	}

	var assets []ReleaseAsset
	for _, a := range release.Assets {
		assets = append(assets, ReleaseAsset{
			Name:        a.Name,
			Size:        a.Size,
			DownloadURL: a.BrowserDownloadURL,
		})
	}
	return assets, release.TagName, nil
}

// --- Select Files to Sync ---

func (s *Service) SelectFiles(ctx context.Context, sourceID int64, fileNames []string) error {
	src, err := s.sourceRepo.GetByID(ctx, sourceID)
	if err != nil {
		return err
	}

	// Fetch latest release to get download URLs
	assets, tag, err := s.FetchReleaseAssets(ctx, sourceID)
	if err != nil {
		return err
	}

	assetMap := make(map[string]ReleaseAsset)
	for _, a := range assets {
		assetMap[a.Name] = a
	}

	for _, name := range fileNames {
		asset, ok := assetMap[name]
		if !ok {
			continue
		}

		// Check if already exists
		existing, _ := s.fileRepo.GetBySourceAndName(ctx, sourceID, name)
		if existing != nil {
			existing.DownloadURL = asset.DownloadURL
			existing.FileSize = asset.Size
			existing.Version = tag
			s.fileRepo.Update(ctx, existing)
			continue
		}

		rule := &entity.GeoRuleFile{
			ServerID:    src.ServerID,
			SourceID:    sourceID,
			Name:        name,
			FileName:    name,
			DownloadURL: asset.DownloadURL,
			FileSize:    asset.Size,
			Version:     tag,
			AutoSync:    true,
			Status:      "available",
		}
		s.fileRepo.Create(ctx, rule)
	}

	// Update source last sync tag
	src.LastSyncTag = tag
	s.sourceRepo.Update(ctx, src)

	return nil
}

// --- Download ---

func (s *Service) ListFiles(ctx context.Context) ([]entity.GeoRuleFile, error) {
	return s.fileRepo.List(ctx)
}

// ListFilesByServer lists geo rule files for a specific node server.
func (s *Service) ListFilesByServer(ctx context.Context, serverID int64) ([]entity.GeoRuleFile, error) {
	return s.fileRepo.ListByServer(ctx, serverID)
}

func (s *Service) ListFilesBySource(ctx context.Context, sourceID int64) ([]entity.GeoRuleFile, error) {
	return s.fileRepo.ListBySource(ctx, sourceID)
}

func (s *Service) Download(ctx context.Context, fileID int64) (*entity.GeoRuleFile, error) {
	rule, err := s.fileRepo.GetByID(ctx, fileID)
	if err != nil {
		return nil, err
	}
	// Geo rules are global — always download to panel's local data/rules/
	return s.downloadLocal(ctx, rule)
}

// downloadLocal downloads the file to the panel's local data/rules/ directory.
func (s *Service) downloadLocal(ctx context.Context, rule *entity.GeoRuleFile) (*entity.GeoRuleFile, error) {
	rulesDir := filepath.Join(s.dataDir, "rules")
	tmpDir := filepath.Join(s.dataDir, "rules_tmp")
	os.MkdirAll(rulesDir, 0750)
	os.MkdirAll(tmpDir, 0750)

	filePath := filepath.Join(rulesDir, rule.FileName)
	tmpPath := filepath.Join(tmpDir, rule.FileName)

	rule.Status = "downloading"
	s.fileRepo.Update(ctx, rule)

	downloadURL := netutil.AccelerateGitHubURL(rule.DownloadURL, s.githubAccelDomain(ctx))
	req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		rule.Status = "error"
		s.fileRepo.Update(ctx, rule)
		return nil, err
	}

	resp, err := s.httpClientWithAccel(ctx).Do(req)
	if err != nil {
		rule.Status = "error"
		s.fileRepo.Update(ctx, rule)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		rule.Status = "error"
		s.fileRepo.Update(ctx, rule)
		return nil, fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	// Download to temp directory first
	f, err := os.Create(tmpPath)
	if err != nil {
		rule.Status = "error"
		s.fileRepo.Update(ctx, rule)
		return nil, err
	}

	size, err := io.Copy(f, resp.Body)
	f.Close()
	if err != nil {
		os.Remove(tmpPath)
		rule.Status = "error"
		s.fileRepo.Update(ctx, rule)
		return nil, err
	}

	// Atomic move from temp to final location
	if err := os.Rename(tmpPath, filePath); err != nil {
		os.Remove(tmpPath)
		rule.Status = "error"
		s.fileRepo.Update(ctx, rule)
		return nil, err
	}

	rule.FileSize = size
	rule.LastUpdated = time.Now()
	rule.Status = "available"
	s.fileRepo.Update(ctx, rule)

	return rule, nil
}

// downloadRemote is no longer used — geo rules are always downloaded to the
// panel's local directory first, then pushed to remote nodes via SyncToNode.
// Kept for backward compatibility but just delegates to downloadLocal.
func (s *Service) downloadRemote(ctx context.Context, rule *entity.GeoRuleFile) (*entity.GeoRuleFile, error) {
	return s.downloadLocal(ctx, rule)
}

// DeleteFile deletes a geo rule file from the panel's local disk and database,
// then propagates the deletion to all remote nodes asynchronously.
func (s *Service) DeleteFile(ctx context.Context, fileID int64) error {
	rule, err := s.fileRepo.GetByID(ctx, fileID)
	if err != nil {
		return err
	}
	fileName := rule.FileName
	os.Remove(filepath.Join(s.dataDir, "rules", fileName))
	if err := s.fileRepo.Delete(ctx, fileID); err != nil {
		return err
	}

	// Propagate deletion to all remote nodes asynchronously
	go func() {
		if err := s.DeleteFromAllNodes(context.Background(), fileName); err != nil {
			log.Printf("[georule] propagate deletion of %s to all nodes failed: %v", fileName, err)
		}
	}()

	return nil
}

// DeleteFromAllNodes sends DeleteGeoRule commands to all remote nodes.
// The local node's file is already deleted by the caller.
func (s *Service) DeleteFromAllNodes(ctx context.Context, fileName string) error {
	if s.nodeOp == nil || s.nodeLister == nil {
		return nil
	}
	nodeIDs, err := s.nodeLister.ListAllServerIDs(ctx)
	if err != nil {
		return fmt.Errorf("list server IDs: %w", err)
	}
	var lastErr error
	for _, id := range nodeIDs {
		if s.nodeOp.IsLocalServer(id) {
			continue // local node file already deleted
		}
		success, errMsg, err := s.nodeOp.DeleteGeoRule(ctx, id, fileName)
		if err != nil {
			log.Printf("[georule] delete file %s on server %d failed: %v", fileName, id, err)
			lastErr = err
			continue
		}
		if !success && errMsg != "LOCAL" {
			log.Printf("[georule] delete file %s on server %d failed: %s", fileName, id, errMsg)
			continue
		}
		log.Printf("[georule] deleted file %s on server %d", fileName, id)
	}
	return lastErr
}

func (s *Service) SetAutoSync(ctx context.Context, fileID int64, autoSync bool) error {
	rule, err := s.fileRepo.GetByID(ctx, fileID)
	if err != nil {
		return err
	}
	rule.AutoSync = autoSync
	return s.fileRepo.Update(ctx, rule)
}

// --- Sync ---

// SyncResult contains the result of a sync operation.
type SyncResult struct {
	SourceID   int64  `json:"source_id"`
	Status     string `json:"status"`
	Tag        string `json:"tag"`
	Downloaded int    `json:"downloaded"`
	Total      int    `json:"total"`
	Error      string `json:"error,omitempty"`
}

// isDirectURL checks if a repo URL is a direct file URL (not a GitHub repo).
func isDirectURL(repoURL string) bool {
	return strings.HasPrefix(repoURL, "http://") || strings.HasPrefix(repoURL, "https://")
}

// SyncSource syncs a single source: fetches latest release and downloads auto-sync files.
func (s *Service) SyncSource(ctx context.Context, sourceID int64) (*SyncResult, error) {
	src, err := s.sourceRepo.GetByID(ctx, sourceID)
	if err != nil {
		return nil, err
	}

	src.Status = "syncing"
	s.sourceRepo.Update(ctx, src)

	files, _ := s.fileRepo.ListBySource(ctx, sourceID)
	downloaded := 0
	tag := ""

	if isDirectURL(src.RepoURL) {
		// Direct URL mode: just download files directly
		for _, f := range files {
			if !f.AutoSync {
				continue
			}
			if _, err := s.Download(ctx, f.ID); err != nil {
				log.Printf("sync: download %s failed: %v", f.Name, err)
				continue
			}
			downloaded++
		}
		tag = time.Now().Format("200601021504")
	} else {
		// GitHub repo mode: fetch releases and update URLs
		assets, releaseTag, err := s.FetchReleaseAssets(ctx, sourceID)
		if err != nil {
			src.Status = "error"
			s.sourceRepo.Update(ctx, src)
			return nil, err
		}
		tag = releaseTag

		assetMap := make(map[string]ReleaseAsset)
		for _, a := range assets {
			assetMap[a.Name] = a
		}
		for _, f := range files {
			if asset, ok := assetMap[f.FileName]; ok {
				f.DownloadURL = asset.DownloadURL
				f.Version = tag
				s.fileRepo.Update(ctx, &f)
			}
		}
		for _, f := range files {
			if !f.AutoSync {
				continue
			}
			if _, err := s.Download(ctx, f.ID); err != nil {
				log.Printf("sync: download %s failed: %v", f.Name, err)
				continue
			}
			downloaded++
		}
	}

	src.LastSyncAt = time.Now()
	src.LastSyncTag = tag
	src.Status = "idle"
	s.sourceRepo.Update(ctx, src)

	return &SyncResult{
		SourceID:   sourceID,
		Status:     "done",
		Tag:        tag,
		Downloaded: downloaded,
		Total:      len(files),
	}, nil
}

// SyncAll syncs all sources (manual sync ignores enabled flag).
func (s *Service) SyncAll(ctx context.Context) ([]SyncResult, error) {
	sources, err := s.sourceRepo.List(ctx)
	if err != nil {
		return nil, err
	}

	var results []SyncResult
	for _, src := range sources {
		result, err := s.SyncSource(ctx, src.ID)
		if err != nil {
			results = append(results, SyncResult{
				SourceID: src.ID,
				Status:   "error",
				Error:    err.Error(),
			})
			continue
		}
		results = append(results, *result)
	}
	return results, nil
}

// SyncAllByServer syncs all sources for a specific node server.
func (s *Service) SyncAllByServer(ctx context.Context, serverID int64) ([]SyncResult, error) {
	sources, err := s.sourceRepo.ListByServer(ctx, serverID)
	if err != nil {
		return nil, err
	}

	var results []SyncResult
	for _, src := range sources {
		result, err := s.SyncSource(ctx, src.ID)
		if err != nil {
			results = append(results, SyncResult{
				SourceID: src.ID,
				Status:   "error",
				Error:    err.Error(),
			})
			continue
		}
		results = append(results, *result)
	}
	return results, nil
}

// SyncJobStatus tracks an async sync job.
type SyncJobStatus struct {
	Status     string       `json:"status"` // running, done, error
	Results    []SyncResult `json:"results,omitempty"`
	Error      string       `json:"error,omitempty"`
	StartedAt  time.Time    `json:"started_at"`
	FinishedAt time.Time    `json:"finished_at,omitempty"`
}

var (
	syncMu   sync.Mutex
	syncJobs = make(map[string]*SyncJobStatus)
)

// StartSyncSource starts an async sync for a source.
func (s *Service) StartSyncSource(sourceID int64) string {
	key := fmt.Sprintf("source-%d", sourceID)
	syncMu.Lock()
	syncJobs[key] = &SyncJobStatus{Status: "running", StartedAt: time.Now()}
	syncMu.Unlock()

	go func() {
		result, err := s.SyncSource(context.Background(), sourceID)
		syncMu.Lock()
		defer syncMu.Unlock()
		job := syncJobs[key]
		if err != nil {
			job.Status = "error"
			job.Error = err.Error()
		} else {
			job.Status = "done"
			job.Results = []SyncResult{*result}
		}
		job.FinishedAt = time.Now()
	}()

	return key
}

// StartSyncAll starts an async sync for all enabled sources.
func (s *Service) StartSyncAll() string {
	key := "sync-all"
	syncMu.Lock()
	syncJobs[key] = &SyncJobStatus{Status: "running", StartedAt: time.Now()}
	syncMu.Unlock()

	go func() {
		results, err := s.SyncAll(context.Background())
		syncMu.Lock()
		job := syncJobs[key]
		if err != nil {
			job.Status = "error"
			job.Error = err.Error()
		} else {
			job.Status = "done"
			job.Results = results
		}
		job.FinishedAt = time.Now()
		syncMu.Unlock()

		// After sync completes, push all files to remote nodes
		if err == nil {
			go func() {
				time.Sleep(1 * time.Second)
				if err := s.SyncToAllNodes(context.Background()); err != nil {
					log.Printf("[georule] auto-push to all nodes after sync failed: %v", err)
				}
			}()
		}
	}()

	return key
}

// StartSyncAllByServer starts an async sync for all sources of a specific node server.
func (s *Service) StartSyncAllByServer(serverID int64) string {
	key := fmt.Sprintf("sync-all-%d", serverID)
	syncMu.Lock()
	syncJobs[key] = &SyncJobStatus{Status: "running", StartedAt: time.Now()}
	syncMu.Unlock()

	go func() {
		results, err := s.SyncAllByServer(context.Background(), serverID)
		syncMu.Lock()
		defer syncMu.Unlock()
		job := syncJobs[key]
		if err != nil {
			job.Status = "error"
			job.Error = err.Error()
		} else {
			job.Status = "done"
			job.Results = results
		}
		job.FinishedAt = time.Now()
	}()

	return key
}

// GetSyncJobStatus returns the status of an async sync job.
func (s *Service) GetSyncJobStatus(key string) *SyncJobStatus {
	syncMu.Lock()
	defer syncMu.Unlock()
	job, ok := syncJobs[key]
	if !ok {
		return nil
	}
	// Clean up finished jobs older than 5 minutes
	if job.Status != "running" && time.Since(job.FinishedAt) > 5*time.Minute {
		delete(syncJobs, key)
		return nil
	}
	return job
}

// --- Push to Nodes ---

// SyncToNode pushes all geo rule files to a specific node.
// For local nodes, files are already on disk (downloaded by the panel).
// For remote nodes, sends DownloadGeoRule commands with **relative** download
// URLs (e.g. "/api/v1/geo-rules/files/3/download-file"). The Agent resolves
// the relative URL using the panel host/port from its own config — same
// pattern as AgentUpdater for Agent binary updates. This avoids the need
// to configure panel.external_url.
func (s *Service) SyncToNode(ctx context.Context, serverID int64) error {
	if s.nodeOp == nil {
		return nil
	}
	// Local node already has the files in data/rules/
	if s.nodeOp.IsLocalServer(serverID) {
		return nil
	}

	// Get all global geo rule files
	files, err := s.fileRepo.ListByServer(ctx, 0)
	if err != nil {
		return err
	}

	for _, f := range files {
		// Skip files that are not yet downloaded or in error state
		if f.Status != "available" {
			continue
		}
		// Build a **relative** download URL — the Agent resolves it using
		// the panel host/port from its config (same as UpdateAgent).
		downloadURL := fmt.Sprintf("/api/v1/geo-rules/files/%d/download-file", f.ID)

		success, _, errMsg, err := s.nodeOp.DownloadGeoRule(ctx, serverID, f.FileName, downloadURL, "", "")
		if err != nil {
			log.Printf("[georule] push file %s to server %d failed: %v", f.FileName, serverID, err)
			continue
		}
		if !success {
			log.Printf("[georule] push file %s to server %d failed: %s", f.FileName, serverID, errMsg)
			continue
		}
		log.Printf("[georule] pushed file %s to server %d", f.FileName, serverID)
	}
	return nil
}

// SyncToAllNodes pushes all geo rule files to all known nodes.
// Skips the local node (files already on disk). This is called
// automatically after sync completes, or manually from the UI.
func (s *Service) SyncToAllNodes(ctx context.Context) error {
	if s.nodeLister == nil {
		return nil
	}
	nodeIDs, err := s.nodeLister.ListAllServerIDs(ctx)
	if err != nil {
		return fmt.Errorf("list server IDs: %w", err)
	}
	var lastErr error
	for _, id := range nodeIDs {
		if s.nodeOp != nil && s.nodeOp.IsLocalServer(id) {
			continue // local node already has files
		}
		if err := s.SyncToNode(ctx, id); err != nil {
			log.Printf("[georule] push to server %d failed: %v", id, err)
			lastErr = err
		}
	}
	return lastErr
}

// ServeFileContent returns the file content from disk for HTTP download by Agents.
// Called by the HTTP handler when an Agent requests a geo rule file.
func (s *Service) ServeFileContent(fileID int64) (filePath string, fileName string, err error) {
	rule, err := s.fileRepo.GetByID(context.Background(), fileID)
	if err != nil {
		return "", "", err
	}
	filePath = filepath.Join(s.dataDir, "rules", rule.FileName)
	if _, err := os.Stat(filePath); err != nil {
		return "", "", fmt.Errorf("file %s not found on disk", rule.FileName)
	}
	return filePath, rule.FileName, nil
}
