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

	"corefusion/pkg/netutil"
)

// GitHubRelease represents a GitHub release.
type GitHubRelease struct {
	TagName    string         `json:"tag_name"`
	Prerelease bool           `json:"prerelease"`
	Assets     []GitHubAsset  `json:"assets"`
}

// GitHubAsset represents a release asset.
type GitHubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// SettingStore provides access to system settings for GitHub acceleration.
type SettingStore interface {
	GetByKey(ctx context.Context, key string) (string, error)
}

// ProcessController provides process stop/start during binary updates.
type ProcessController interface {
	Stop(ctx context.Context, serviceID int64) error
	Start(ctx context.Context, serviceID int64, coreType string) error
}

// Installer handles downloading and installing core binaries (xray/singbox/hysteria2/tuic).
// The logic is extracted from the HTTP handler so it can be reused by both
// the panel's REST API and the Agent's gRPC service.
type Installer struct {
	dataDir    string
	resolver   *Resolver
	settingSvc SettingStore
	procCtl    ProcessController
}

// NewInstaller creates a new binary installer.
func NewInstaller(dataDir string, resolver *Resolver, settingSvc SettingStore) *Installer {
	absDir := dataDir
	if !filepath.IsAbs(dataDir) {
		if abs, err := filepath.Abs(dataDir); err == nil {
			absDir = abs
		}
	}
	return &Installer{dataDir: absDir, resolver: resolver, settingSvc: settingSvc}
}

// SetProcessController sets the process controller for stop/start during updates.
func (i *Installer) SetProcessController(pc ProcessController) {
	i.procCtl = pc
}

// Install downloads and installs a core binary.
// If downloadURL is empty, it constructs the URL from the core type and version.
// If version is empty, it fetches the latest release.
func (i *Installer) Install(ctx context.Context, coreType, version, downloadURL, assetName string) (*BinaryInfo, error) {
	repoURL := githubRepo(coreType)
	if repoURL == "" {
		return nil, fmt.Errorf("unsupported core type: %s", coreType)
	}

	// If version is empty, fetch latest
	if version == "" {
		releases, err := i.ListReleasesGitHub(ctx, coreType)
		if err != nil {
			return nil, fmt.Errorf("list releases: %w", err)
		}
		if len(releases) == 0 {
			return nil, fmt.Errorf("no releases found for %s", coreType)
		}
		version = releases[0].TagName
	}

	// Build download URL if not provided
	if downloadURL == "" {
		downloadURL = buildDownloadURL(coreType, repoURL, version)
		if downloadURL == "" && assetName != "" {
			downloadURL = fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repoURL, version, assetName)
		}
		if downloadURL == "" {
			return nil, fmt.Errorf("cannot determine download URL for: %s", coreType)
		}
	}

	// Download to temp directory first
	tmpDir := filepath.Join(i.dataDir, "binaries", "tmp")
	os.MkdirAll(tmpDir, 0750)
	if err := i.downloadAndExtract(ctx, downloadURL, tmpDir, coreType); err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}

	// Find the downloaded binary in temp
	srcName := coreType + "-" + runtime.GOOS + "-" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		srcName += ".exe"
	}
	srcPath := filepath.Join(tmpDir, srcName)
	if _, err := os.Stat(srcPath); err != nil {
		return nil, fmt.Errorf("downloaded binary not found: %w", err)
	}

	// Stop process before replacing
	serviceID := getServiceID(coreType)
	if i.procCtl != nil && serviceID > 0 {
		i.procCtl.Stop(ctx, serviceID)
		time.Sleep(500 * time.Millisecond)
	}

	// Move from temp to final location
	destDir := filepath.Join(i.dataDir, "binaries", "active", coreType)
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
	if i.procCtl != nil && serviceID > 0 {
		i.procCtl.Start(ctx, serviceID, coreType)
	}

	return &BinaryInfo{
		CoreType:   coreType,
		Version:    version,
		BinaryPath: destDir,
		Installed:  true,
	}, nil
}

// ListReleasesGitHub fetches available releases from GitHub for a core type (returns full GitHubRelease).
// Used by HTTP handler.
func (i *Installer) ListReleasesGitHub(ctx context.Context, coreType string) ([]GitHubRelease, error) {
	repoURL := githubRepo(coreType)
	if repoURL == "" {
		return nil, fmt.Errorf("unsupported core type: %s", coreType)
	}

	accelDomain := i.githubAccelDomain(ctx)
	apiURL := netutil.AccelerateGitHubURL(
		fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=20", repoURL),
		accelDomain,
	)
	client := netutil.GitHubHTTPClient(30*time.Second, accelDomain)
	req, _ := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var releases []GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, err
	}

	// Filter assets
	for i := range releases {
		var filtered []GitHubAsset
		for _, a := range releases[i].Assets {
			if filterAsset(coreType, a.Name) {
				filtered = append(filtered, a)
			}
		}
		releases[i].Assets = filtered
	}

	return releases, nil
}

// ListInstalled returns all installed core binaries.
func (i *Installer) ListInstalled() []BinaryInfo {
	return i.resolver.ListInstalled()
}

func (i *Installer) githubAccelDomain(ctx context.Context) string {
	if i.settingSvc == nil {
		return ""
	}
	val, err := i.settingSvc.GetByKey(ctx, "github.acceleration")
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

func (i *Installer) downloadAndExtract(ctx context.Context, rawURL, destDir, coreType string) error {
	accelURL := netutil.AccelerateGitHubURL(rawURL, i.githubAccelDomain(ctx))
	client := netutil.GitHubHTTPClient(300*time.Second, i.githubAccelDomain(ctx))
	req, _ := http.NewRequestWithContext(ctx, "GET", accelURL, nil)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	// Try zip extraction first (Xray uses .zip)
	if len(bodyBytes) >= 2 && bodyBytes[0] == 'P' && bodyBytes[1] == 'K' {
		return extractZip(bodyBytes, destDir, coreType)
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
		return os.WriteFile(destPath, bodyBytes, 0755)
	}

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

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

func extractZip(data []byte, destDir, coreType string) error {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}

	binaryName := coreType + "-" + runtime.GOOS + "-" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}

	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
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

// githubRepo maps core type to GitHub repo path.
func githubRepo(coreType string) string {
	repos := map[string]string{
		"xray":      "XTLS/Xray-core",
		"singbox":   "SagerNet/sing-box",
		"hysteria2": "apernet/hysteria",
		"tuic":      "Itsusinn/tuic",
	}
	return repos[coreType]
}

// buildDownloadURL constructs the correct download URL for each core type.
func buildDownloadURL(coreType, repoURL, version string) string {
	osName := runtime.GOOS
	arch := runtime.GOARCH

	switch coreType {
	case "xray":
		archName := "64"
		if arch == "arm64" {
			archName = "arm64-v8a"
		}
		asset := fmt.Sprintf("Xray-%s-%s.zip", osName, archName)
		return fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repoURL, version, asset)

	case "singbox":
		if arch == "amd64" {
			arch = "amd64"
		}
		asset := fmt.Sprintf("sing-box-%s-%s-%s.tar.gz", strings.TrimPrefix(version, "v"), osName, arch)
		return fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repoURL, version, asset)

	case "hysteria2":
		asset := fmt.Sprintf("hysteria-%s-%s", osName, arch)
		return fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repoURL, version, asset)

	case "tuic":
		archName := "x86_64"
		if arch == "arm64" {
			archName = "aarch64"
		}
		asset := fmt.Sprintf("tuic-server-%s-%s", osName, archName)
		return fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repoURL, version, asset)
	}
	return ""
}

// filterAsset returns true if the asset name is relevant for the core type.
func filterAsset(coreType, name string) bool {
	if strings.HasSuffix(name, ".txt") || strings.HasSuffix(name, ".md") ||
		strings.HasSuffix(name, ".sha256") || strings.HasSuffix(name, ".sig") ||
		name == "LICENSE" || name == "README.md" {
		return false
	}
	if coreType == "tuic" && !strings.Contains(name, "tuic-server") {
		return false
	}
	return true
}

// getServiceID returns the service ID for a core type.
func getServiceID(coreType string) int64 {
	ids := map[string]int64{"xray": 1, "singbox": 2, "hysteria2": 3, "tuic": 4}
	return ids[coreType]
}

// ReleaseInfo contains GitHub release information.
type ReleaseInfo struct {
	Version    string `json:"version"`
	Prerelease bool   `json:"prerelease"`
}

// ListReleases fetches available releases from GitHub for a core type.
func (i *Installer) ListReleases(ctx context.Context, coreType string) ([]ReleaseInfo, error) {
	repoURL := githubRepo(coreType)
	if repoURL == "" {
		return nil, fmt.Errorf("unsupported core type: %s", coreType)
	}

	accelDomain := i.githubAccelDomain(ctx)
	apiURL := netutil.AccelerateGitHubURL(
		fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=20", repoURL),
		accelDomain,
	)
	client := netutil.GitHubHTTPClient(30*time.Second, accelDomain)
	req, _ := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var ghReleases []GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&ghReleases); err != nil {
		return nil, err
	}

	var result []ReleaseInfo
	for _, r := range ghReleases {
		result = append(result, ReleaseInfo{
			Version:    r.TagName,
			Prerelease: r.Prerelease,
		})
	}
	return result, nil
}
