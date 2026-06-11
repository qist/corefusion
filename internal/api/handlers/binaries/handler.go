package binaries

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/model/dto"
	binariesSvc "corefusion/internal/service/binaries"
)

// GitHubRelease represents a GitHub release.
type GitHubRelease struct {
	TagName    string `json:"tag_name"`
	Prerelease bool   `json:"prerelease"`
	Assets     []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// Handler handles binary management HTTP requests.
type Handler struct {
	resolver *binariesSvc.Resolver
	dataDir  string
}

// NewHandler creates a new binary handler.
func NewHandler(resolver *binariesSvc.Resolver, dataDir string) *Handler {
	return &Handler{resolver: resolver, dataDir: dataDir}
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
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=20", repoURL)
	client := &http.Client{}
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

	// Determine asset name
	osName := runtime.GOOS
	archName := runtime.GOARCH
	if archName == "amd64" {
		archName = "64"
	} else if archName == "arm64" {
		archName = "arm64-v8a"
	}

	assetName := fmt.Sprintf("%s-%s-%s", req.CoreType, osName, archName)
	downloadURL := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repoURL, req.Version, assetName)

	// Download and extract
	destDir := filepath.Join(h.dataDir, "binaries", "active", req.CoreType)
	os.MkdirAll(destDir, 0750)

	if err := h.downloadAndExtract(downloadURL, destDir, req.CoreType); err != nil {
		response.InternalError(c, "DOWNLOAD_FAILED", err.Error())
		return
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
func (h *Handler) downloadAndExtract(url, destDir, coreType string) error {
	client := &http.Client{}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	// Try tar.gz extraction
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		// Not a gzip, save directly
		binaryName := coreType
		if runtime.GOOS == "windows" {
			binaryName += ".exe"
		}
		destPath := filepath.Join(destDir, binaryName)
		f, err := os.Create(destPath)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := io.Copy(f, resp.Body); err != nil {
			return err
		}
		os.Chmod(destPath, 0755)
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
			binaryName := coreType
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
