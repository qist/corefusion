package binaries

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/model/dto"
	binariesSvc "corefusion/internal/service/binaries"
	"corefusion/pkg/netutil"
)

// SettingStore provides access to system settings.
type SettingStore interface {
	GetByKey(ctx context.Context, key string) (string, error)
}

// GitHubRelease represents a GitHub release.
type GitHubRelease struct {
	TagName    string `json:"tag_name"`
	Prerelease bool   `json:"prerelease"`
	Assets     []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// ProcessManager provides process start/stop capabilities.
type ProcessManager interface {
	Stop(ctx context.Context, serviceID int64) error
	Start(ctx context.Context, serviceID int64, coreType string) error
}

// Handler handles binary management HTTP requests.
type Handler struct {
	resolver   *binariesSvc.Resolver
	dataDir    string
	settingSvc SettingStore
	processMgr ProcessManager
}

// NewHandler creates a new binary handler.
func NewHandler(resolver *binariesSvc.Resolver, dataDir string, settingSvc SettingStore) *Handler {
	return &Handler{resolver: resolver, dataDir: dataDir, settingSvc: settingSvc}
}

// SetProcessManager sets the process manager for stop/start during updates.
func (h *Handler) SetProcessManager(pm ProcessManager) {
	h.processMgr = pm
}

// githubAccelDomain reads the active GitHub acceleration domain from settings.
func (h *Handler) githubAccelDomain(ctx context.Context) string {
	if h.settingSvc == nil {
		return ""
	}
	val, err := h.settingSvc.GetByKey(ctx, "github.acceleration")
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

// List handles GET /api/v1/binaries.
func (h *Handler) List(c *gin.Context) {
	binaries := h.resolver.ListInstalled()
	response.OK(c, binaries)
}

// ListReleases handles GET /api/v1/binaries/releases.
func (h *Handler) ListReleases(c *gin.Context) {
	coreType := c.Query("core_type")
	if coreType == "" {
		response.BadRequest(c, "INVALID_REQUEST", "core_type is required")
		return
	}

	// Map core type to GitHub repo
	repoURL := h.githubRepo(coreType)
	if repoURL == "" {
		response.BadRequest(c, "INVALID_CORE_TYPE", "unsupported core type: "+coreType)
		return
	}

	// Fetch releases from GitHub API
	apiURL := netutil.AccelerateGitHubURL(
		fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=20", repoURL),
		h.githubAccelDomain(c.Request.Context()),
	)
	client := netutil.GitHubHTTPClient(30*time.Second, h.githubAccelDomain(c.Request.Context()))
	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		response.InternalError(c, "FETCH_FAILED", err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		response.InternalError(c, "FETCH_FAILED", fmt.Sprintf("GitHub API returned %d", resp.StatusCode))
		return
	}

	body, _ := io.ReadAll(resp.Body)
	var releases []GitHubRelease
	if err := json.Unmarshal(body, &releases); err != nil {
		response.InternalError(c, "PARSE_FAILED", err.Error())
		return
	}

	var items []gin.H
	for _, r := range releases {
		filtered := filterAssets(coreType, r.Assets)
		if len(filtered) == 0 {
			continue
		}
		items = append(items, gin.H{
			"version":    r.TagName,
			"prerelease": r.Prerelease,
			"assets":     filtered,
		})
	}

	var q dto.ListQuery
	c.ShouldBindQuery(&q)
	response.Paginated(c, items, int64(len(items)), q.Limit, q.Offset)
}

// Download handles POST /api/v1/binaries/download.
func (h *Handler) Download(c *gin.Context) {
	var req struct {
		CoreType string `json:"core_type" binding:"required"`
		Version  string `json:"version" binding:"required"`
		Asset    string `json:"asset"` // optional: specific asset name
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	repoURL := h.githubRepo(req.CoreType)
	if repoURL == "" {
		response.BadRequest(c, "INVALID_CORE_TYPE", "unsupported core type: "+req.CoreType)
		return
	}

	var downloadURL string
	if req.Asset != "" {
		downloadURL = fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repoURL, req.Version, req.Asset)
	} else {
		downloadURL = h.buildDownloadURL(req.CoreType, repoURL, req.Version)
	}
	if downloadURL == "" {
		response.BadRequest(c, "INVALID_CORE_TYPE", "cannot determine download URL for: "+req.CoreType)
		return
	}

	// Download to temp directory first
	tmpDir := filepath.Join(h.dataDir, "binaries", "tmp")
	os.MkdirAll(tmpDir, 0750)
	if err := h.downloadAndExtract(downloadURL, tmpDir, req.CoreType); err != nil {
		response.InternalError(c, "DOWNLOAD_FAILED", err.Error())
		return
	}

	// Find the downloaded binary in temp
	srcName := req.CoreType + "-" + runtime.GOOS + "-" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		srcName += ".exe"
	}
	srcPath := filepath.Join(tmpDir, srcName)
	if _, err := os.Stat(srcPath); err != nil {
		response.InternalError(c, "DOWNLOAD_FAILED", "downloaded binary not found")
		return
	}

	// Stop process before replacing
	serviceID := h.getServiceID(req.CoreType)
	if h.processMgr != nil && serviceID > 0 {
		h.processMgr.Stop(c.Request.Context(), serviceID)
		time.Sleep(500 * time.Millisecond)
	}

	// Move from temp to final location
	destDir := filepath.Join(h.dataDir, "binaries", "active", req.CoreType)
	os.MkdirAll(destDir, 0750)
	destPath := filepath.Join(destDir, srcName)
	os.Remove(destPath)
	if err := os.Rename(srcPath, destPath); err != nil {
		// Try copy if rename fails (cross-device)
		data, _ := os.ReadFile(srcPath)
		os.WriteFile(destPath, data, 0755)
		os.Remove(srcPath)
	}
	os.Chmod(destPath, 0755)

	// Start process with new binary
	if h.processMgr != nil && serviceID > 0 {
		h.processMgr.Start(c.Request.Context(), serviceID, req.CoreType)
	}

	response.OK(c, gin.H{
		"core_type": req.CoreType,
		"version":   req.Version,
		"path":      destDir,
	})
}

// Install handles POST /api/v1/binaries/install.
func (h *Handler) Install(c *gin.Context) {
	// Same as download for now
	h.Download(c)
}

// Update handles POST /api/v1/binaries/update.
func (h *Handler) Update(c *gin.Context) {
	h.Download(c)
}

// Rollback handles POST /api/v1/binaries/rollback.
func (h *Handler) Rollback(c *gin.Context) {
	response.OK(c, gin.H{"code": "NOT_IMPLEMENTED", "message": "binary rollback not yet implemented"})
}

// getServiceID returns the service ID for a core type.
func (h *Handler) getServiceID(coreType string) int64 {
	ids := map[string]int64{"xray": 1, "singbox": 2, "hysteria2": 3, "tuic": 4}
	return ids[coreType]
}

// githubRepo maps core type to GitHub repo path.
func (h *Handler) githubRepo(coreType string) string {
	repos := map[string]string{
		"xray":       "XTLS/Xray-core",
		"singbox":    "SagerNet/sing-box",
		"hysteria2":  "apernet/hysteria",
		"tuic":       "Itsusinn/tuic",
	}
	return repos[coreType]
}

// buildDownloadURL constructs the correct download URL for each core type.
func (h *Handler) buildDownloadURL(coreType, repoURL, version string) string {
	osName := runtime.GOOS // linux, darwin, windows
	arch := runtime.GOARCH // amd64, arm64

	switch coreType {
	case "xray":
		// Xray: Xray-linux-64.zip, Xray-linux-arm64-v8a.zip
		archName := "64"
		if arch == "arm64" {
			archName = "arm64-v8a"
		}
		asset := fmt.Sprintf("Xray-%s-%s.zip", osName, archName)
		return fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repoURL, version, asset)

	case "singbox":
		// sing-box: sing-box-{version}-linux-amd64.tar.gz
		if arch == "amd64" {
			arch = "amd64"
		}
		asset := fmt.Sprintf("sing-box-%s-%s-%s.tar.gz", strings.TrimPrefix(version, "v"), osName, arch)
		return fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repoURL, version, asset)

	case "hysteria2":
		// Hysteria: hysteria-linux-amd64
		asset := fmt.Sprintf("hysteria-%s-%s", osName, arch)
		return fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repoURL, version, asset)

	case "tuic":
		// TUIC: tuic-server-linux-x86_64
		archName := "x86_64"
		if arch == "arm64" {
			archName = "aarch64"
		}
		asset := fmt.Sprintf("tuic-server-%s-%s", osName, archName)
		return fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repoURL, version, asset)
	}
	return ""
}

// filterAssets filters release assets to only include relevant binaries.
func filterAssets(coreType string, assets []struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}) []struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
} {
	var filtered []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	}
	for _, a := range assets {
		name := a.Name
		// Skip non-binary files
		if strings.HasSuffix(name, ".txt") || strings.HasSuffix(name, ".md") ||
			strings.HasSuffix(name, ".sha256") || strings.HasSuffix(name, ".sig") ||
			name == "LICENSE" || name == "README.md" {
			continue
		}
		// For TUIC, only show server binaries
		if coreType == "tuic" && !strings.Contains(name, "tuic-server") {
			continue
		}
		filtered = append(filtered, a)
	}
	return filtered
}

// downloadAndExtract downloads a tar.gz binary and extracts it.
func (h *Handler) downloadAndExtract(rawURL, destDir, coreType string) error {
	accelURL := netutil.AccelerateGitHubURL(rawURL, h.githubAccelDomain(context.Background()))
	client := netutil.GitHubHTTPClient(300*time.Second, h.githubAccelDomain(context.Background()))
	resp, err := client.Get(accelURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	// Read entire response body into memory for format detection
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	// Try zip extraction first (Xray uses .zip)
	if len(bodyBytes) >= 2 && bodyBytes[0] == 'P' && bodyBytes[1] == 'K' {
		return h.extractZip(bodyBytes, destDir, coreType)
	}

	// Try tar.gz extraction
	gz, err := gzip.NewReader(bytes.NewReader(bodyBytes))
	if err != nil {
		// Not gzip either, save directly as binary
		binaryName := coreType + "-" + runtime.GOOS + "-" + runtime.GOARCH
		if runtime.GOOS == "windows" {
			binaryName += ".exe"
		}
		destPath := filepath.Join(destDir, binaryName)
		if err := os.WriteFile(destPath, bodyBytes, 0755); err != nil {
			return err
		}
		return nil
	}

	// Extract from tar.gz
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Look for the binary
		name := header.Name
		if strings.Contains(name, coreType) && !strings.HasSuffix(name, "/") {
			binaryName := coreType + "-" + runtime.GOOS + "-" + runtime.GOARCH
			if runtime.GOOS == "windows" {
				binaryName += ".exe"
			}
			destPath := filepath.Join(destDir, binaryName)
			f, err := os.Create(destPath)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
			os.Chmod(destPath, 0755)
			return nil
		}
	}

	return fmt.Errorf("binary not found in archive")
}

// extractZip extracts a binary from a zip archive.
func (h *Handler) extractZip(data []byte, destDir, coreType string) error {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}

	binaryName := coreType + "-" + runtime.GOOS + "-" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}

	for _, f := range r.File {
		// Skip directories
		if f.FileInfo().IsDir() {
			continue
		}
		// Look for the binary (match by coreType name or Xray/xray)
		name := filepath.Base(f.Name)
		lowerName := strings.ToLower(name)
		if lowerName == coreType || lowerName == coreType+".exe" || name == "xray" || name == "xray.exe" {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			defer rc.Close()

			destPath := filepath.Join(destDir, binaryName)
			outFile, err := os.Create(destPath)
			if err != nil {
				return err
			}
			defer outFile.Close()

			if _, err := io.Copy(outFile, rc); err != nil {
				return err
			}
			os.Chmod(destPath, 0755)
			return nil
		}
	}

	return fmt.Errorf("binary not found in zip archive")
}
