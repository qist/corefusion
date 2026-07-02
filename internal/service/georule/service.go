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

// SettingStore provides access to system settings.
type SettingStore interface {
	GetByKey(ctx context.Context, key string) (string, error)
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
type Service struct {
	sourceRepo *repository.RuleSourceRepository
	fileRepo   *repository.GeoRuleRepository
	dataDir    string
	settingSvc SettingStore
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

// httpClientWithAccel returns an HTTP client with GitHub acceleration applied.
func (s *Service) httpClientWithAccel(ctx context.Context) *http.Client {
	return netutil.GitHubHTTPClient(120*time.Second, s.githubAccelDomain(ctx))
}

// SeedDefaults creates the default Loyalsoldier source if not exists.
func (s *Service) SeedDefaults(ctx context.Context, serverID int64) error {
	if serverID == 0 {
		serverID = 1
	}
	sources, err := s.sourceRepo.ListByServer(ctx, serverID)
	if err != nil {
		return err
	}
	if len(sources) > 0 {
		return nil // already seeded
	}

	os.MkdirAll(filepath.Join(s.dataDir, "rules"), 0750)

	src := &entity.RuleSource{
		ServerID: serverID,
		Name:    "Loyalsoldier v2ray-rules-dat",
		RepoURL: "Loyalsoldier/v2ray-rules-dat",
		Enabled: false,
		SyncCron: "0 3 * * *",
		Status:  "idle",
	}
	if err := s.sourceRepo.Create(ctx, src); err != nil {
		return err
	}

	// Fetch release and add default files
	go func() {
		time.Sleep(2 * time.Second) // Wait for server to be ready
		ctx := context.Background()
		assets, tag, err := s.FetchReleaseAssets(ctx, src.ID)
		if err != nil {
			log.Printf("seed: failed to fetch assets: %v", err)
			return
		}
		defaultFiles := []string{"geoip.dat", "geosite.dat"}
		assetMap := make(map[string]ReleaseAsset)
		for _, a := range assets {
			assetMap[a.Name] = a
		}
		for _, name := range defaultFiles {
			if asset, ok := assetMap[name]; ok {
				rule := &entity.GeoRuleFile{
					ServerID:    serverID,
					SourceID:    src.ID,
					Name:        name,
					FileName:    name,
					DownloadURL: asset.DownloadURL,
					FileSize:    asset.Size,
					Version:     tag,
					AutoSync:    true,
					Status:      "available",
				}
				if err := s.fileRepo.Create(nil, rule); err != nil {
					log.Printf("seed: failed to create rule %s: %v", name, err)
				} else {
					log.Printf("seed: added default rule %s", name)
				}
			}
		}
		src.LastSyncTag = tag
		s.sourceRepo.Update(nil, src)
	}()

	return nil
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
	if serverID == 0 {
		serverID = 1
	}
	src := &entity.RuleSource{
		ServerID: serverID,
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

func (s *Service) DeleteFile(ctx context.Context, fileID int64) error {
	rule, err := s.fileRepo.GetByID(ctx, fileID)
	if err != nil {
		return err
	}
	os.Remove(filepath.Join(s.dataDir, "rules", rule.FileName))
	return s.fileRepo.Delete(ctx, fileID)
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
