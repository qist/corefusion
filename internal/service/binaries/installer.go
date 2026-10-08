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
	TagName    string        `json:"tag_name"`
	Prerelease bool          `json:"prerelease"`
	Assets     []GitHubAsset `json:"assets"`
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

// ProcessController provides process restart during binary updates.
type ProcessController interface {
	IsRunning(serviceID int64) bool
	Restart(ctx context.Context, serviceID int64, coreType string) error
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

// Install downloads and installs a core binary on the local machine.
// It uses runtime.GOOS/runtime.GOARCH for the local platform.
func (i *Installer) Install(ctx context.Context, coreType, version, downloadURL, assetName string) (*BinaryInfo, error) {
	return i.InstallForPlatform(ctx, coreType, version, downloadURL, assetName, runtime.GOOS, runtime.GOARCH)
}

// InstallForPlatform downloads and installs a core binary for the specified OS/arch.
// If downloadURL is empty, it constructs the URL from the core type and version.
// If version is empty, it fetches the latest release.
func (i *Installer) InstallForPlatform(ctx context.Context, coreType, version, downloadURL, assetName, osName, archName string) (*BinaryInfo, error) {
	repoURL := githubRepo(coreType)
	if repoURL == "" {
		return nil, fmt.Errorf("unsupported core type: %s", coreType)
	}

	// If version is empty, fetch latest.
	//
	// 直接取**最新发布**（GitHub /releases 按创建时间倒序，releases[0] 即最新）。
	//
	// ⚠ 不要改成「优先正式版」：XTLS/Xray-core 近年所有 release 都标 prerelease
	//（实测 v26.6~v26.9 全是），回退正式版会选到远古版本（实测选中 26.3.27），
	// 而面板生成的运行配置是 Xray 25+ 口径（finalmask.udp / mkcp-legacy），
	// 旧版一起来就 unknown config id: mkcp-legacy 崩溃循环 —— 线上踩过 ✗。
	// 风险排序：装预发布最多不稳，装旧正式版是直接起不来。
	if version == "" {
		releases, err := i.ListReleasesGitHubForPlatform(ctx, coreType, osName, archName)
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
		downloadURL = buildDownloadURL(coreType, repoURL, version, osName, archName)
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

	// Find the downloaded binary in temp.
	// The binary name inside the archive uses the actual platform naming.
	srcName := binaryNameInArchive(coreType, osName, archName)
	srcPath := filepath.Join(tmpDir, srcName)
	if _, err := os.Stat(srcPath); err != nil {
		return nil, fmt.Errorf("downloaded binary not found: %w", err)
	}

	// Check if process is running before replacing
	serviceID := getServiceID(coreType)
	wasRunning := false
	if i.procCtl != nil && serviceID > 0 {
		wasRunning = i.procCtl.IsRunning(serviceID)
	}

	// Move from temp to final location (on Linux, replacing a running binary
	// is safe — the running process keeps using the old inode via open fd)
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

	// Restart the process to pick up the new binary (only if it was running)
	if wasRunning && i.procCtl != nil && serviceID > 0 {
		i.procCtl.Restart(ctx, serviceID, coreType)
	}

	return &BinaryInfo{
		CoreType:   coreType,
		Version:    version,
		BinaryPath: destDir,
		Installed:  true,
	}, nil
}

// ListReleasesGitHub fetches available releases from GitHub for the local platform.
func (i *Installer) ListReleasesGitHub(ctx context.Context, coreType string) ([]GitHubRelease, error) {
	return i.ListReleasesGitHubForPlatform(ctx, coreType, runtime.GOOS, runtime.GOARCH)
}

// ListReleasesGitHubForPlatform fetches available releases from GitHub for a core type,
// filtering assets by the specified OS/arch.
func (i *Installer) ListReleasesGitHubForPlatform(ctx context.Context, coreType, osName, archName string) ([]GitHubRelease, error) {
	repoURL := githubRepo(coreType)
	if repoURL == "" {
		return nil, fmt.Errorf("unsupported core type: %s", coreType)
	}

	accelDomain := i.githubAccelDomain(ctx)
	proxyURL := i.githubProxyURL(ctx)
	apiURL := netutil.AccelerateGitHubURL(
		fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=20", repoURL),
		accelDomain,
	)
	client := netutil.GitHubHTTPClientWithProxy(30*time.Second, accelDomain, proxyURL)
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
	// GitHub API 响应加上限：避免异常/恶意响应把内存撑爆（正常 JSON 远小于此）
	if err := json.NewDecoder(io.LimitReader(resp.Body, netutil.MaxJSONResponseBytes)).Decode(&releases); err != nil {
		return nil, err
	}

	// Filter assets by core type and target platform
	for i := range releases {
		var filtered []GitHubAsset
		for _, a := range releases[i].Assets {
			if filterAsset(coreType, a.Name, osName, archName) {
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

// githubProxyURL reads the optional HTTP/SOCKS proxy URL from the GitHub
// acceleration settings. Returns "" when unset.
func (i *Installer) githubProxyURL(ctx context.Context) string {
	if i.settingSvc == nil {
		return ""
	}
	val, err := i.settingSvc.GetByKey(ctx, "github.acceleration")
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

func (i *Installer) downloadAndExtract(ctx context.Context, rawURL, destDir, coreType string) error {
	accelDomain := i.githubAccelDomain(ctx)
	proxyURL := i.githubProxyURL(ctx)
	accelURL := netutil.AccelerateGitHubURL(rawURL, accelDomain)
	client := netutil.GitHubHTTPClientWithProxy(300*time.Second, accelDomain, proxyURL)
	req, _ := http.NewRequestWithContext(ctx, "GET", accelURL, nil)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	// 核心二进制会整块读进内存做格式校验 → 必须有上限（正常 Xray/sing-box 约 30~80 MB，
	// 这里给到 1 GiB 的宽松上限，只为防住异常/恶意响应把它撑爆 ✗）。
	bodyBytes, err := netutil.ReadLimited(resp.Body, netutil.MaxBinaryBytes)
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
		if err := os.WriteFile(destPath, bodyBytes, 0755); err != nil {
			return err
		}
		if err := validateBinary(destPath, coreType); err != nil {
			os.Remove(destPath)
			return fmt.Errorf("downloaded file is not a valid binary: %w", err)
		}
		return nil
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
		if matchCoreBinary(name, coreType) && !strings.HasSuffix(name, "/") {
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
			if err := validateBinary(destPath, coreType); err != nil {
				os.Remove(destPath)
				return fmt.Errorf("extracted file is not a valid binary: %w", err)
			}
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
		if matchCoreBinary(name, coreType) {
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
			if err := validateBinary(destPath, coreType); err != nil {
				os.Remove(destPath)
				return fmt.Errorf("extracted file is not a valid binary: %w", err)
			}
			return nil
		}
	}

	return fmt.Errorf("binary not found in zip archive")
}

// githubRepo maps core type to GitHub repo path.
//
// 注意：singbox 指向本组织的自建打包仓库 qist/singbox-binaries。
// 官方 SagerNet/sing-box 的 release 一律不含 with_v2ray_api 这个 build tag，
// 配置里一旦出现 experimental.v2ray_api 就会启动失败：
//
//	create v2ray-server: v2ray api is not included in this build, rebuild with -tags with_v2ray_api
//
// 而 v2ray_api 是面板按订阅用户统计 sing-box 流量的唯一可用通道
// （clash_api 的 /connections 不暴露 user 字段，无法按用户聚合）。
// 自建仓库的产物命名与官方完全一致（sing-box-<ver>-<os>-<arch>.tar.gz / .zip），
// 因此 buildDownloadURL() 与资产过滤逻辑无需任何其他改动。
func githubRepo(coreType string) string {
	repos := map[string]string{
		"xray":      "XTLS/Xray-core",
		"singbox":   "qist/singbox-binaries",
		"hysteria2": "apernet/hysteria",
		"tuic":      "Itsusinn/tuic",
	}
	return repos[coreType]
}

// buildDownloadURL constructs the correct download URL for each core type.
//
// 各上游（及本组织自建的 singbox-binaries）资产命名并不统一，下面按实测结果逐核心处理，
// 所有差异都用真实 release 资产名 + HTTP 200/404 对照验证过：
//   - xray:      Xray-<os>-<arch>.zip，macOS 用 "macos"（不是 runtime.GOOS 的 "darwin"）
//   - singbox:   sing-box-<ver>-<os>-<arch>.tar.gz；Windows 是 .zip
//   - hysteria2: hysteria-<os>-<arch>；Windows 需 .exe 后缀
//   - tuic:      tuic-server-<arch>-<os>；Windows 需 .exe 后缀
//
// 注意 macOS 命名：xray 用 macos，其余核心（含 sing-box）用 darwin。
func buildDownloadURL(coreType, repoURL, version, osName, archName string) string {
	// Windows 可执行文件后缀（上游只有 hysteria/tuic 直接发裸可执行文件）
	windowsExe := ""
	if osName == "windows" {
		windowsExe = ".exe"
	}
	// Xray 的 macOS 资产名用 macos
	xrayOSName := osName
	if osName == "darwin" {
		xrayOSName = "macos"
	}

	switch coreType {
	case "xray":
		archPart := "64"
		switch archName {
		case "arm64":
			archPart = "arm64-v8a"
		case "arm":
			archPart = "arm32-v7a"
		case "386":
			archPart = "32"
		}
		asset := fmt.Sprintf("Xray-%s-%s.zip", xrayOSName, archPart)
		return fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repoURL, version, asset)

	case "singbox":
		arch := archName
		if arch == "arm" {
			arch = "armv7"
		}
		// 官方与自建仓库：Windows 为 .zip，其余平台为 .tar.gz
		ext := "tar.gz"
		if osName == "windows" {
			ext = "zip"
		}
		asset := fmt.Sprintf("sing-box-%s-%s-%s.%s", strings.TrimPrefix(version, "v"), osName, arch, ext)
		return fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repoURL, version, asset)

	case "hysteria2":
		asset := fmt.Sprintf("hysteria-%s-%s%s", osName, archName, windowsExe)
		return fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repoURL, version, asset)

	case "tuic":
		archPart := "x86_64"
		switch archName {
		case "arm64":
			archPart = "aarch64"
		case "arm":
			archPart = "armv7"
		case "386":
			archPart = "i686"
		}
		asset := fmt.Sprintf("tuic-server-%s-%s%s", archPart, osName, windowsExe)
		return fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repoURL, version, asset)
	}
	return ""
}

// matchCoreBinary checks if an archive entry name corresponds to the
// given core type. It examines only the base name (not the full path)
// to avoid false matches like "sing-box-1.10.0/LICENSE" matching "sing-box".
//
// 匹配大小写不敏感，并剥离 Windows 的 .exe 后缀：
//   - Linux/macOS/BSD: sing-box            → 命中
//   - Windows:         sing-box.exe        → 命中（旧实现中 "sing-box.exe" 既不等于
//     "sing-box" 也不以 "sing-box-" 开头，导致 Windows 包解压后找不到二进制）
//   - Windows xray:    Xray-windows-64.exe → 命中（旧实现 Contains 大小写敏感，漏判）
func matchCoreBinary(name, coreType string) bool {
	// Extract the base name (last path component) to avoid matching
	// directory prefixes like "sing-box-1.10.0-linux-amd64/LICENSE"
	base := filepath.Base(name)

	// Exclude common non-binary files by extension
	lower := strings.ToLower(base)
	if strings.HasSuffix(lower, ".txt") || strings.HasSuffix(lower, ".md") ||
		strings.HasSuffix(lower, ".sha256") || strings.HasSuffix(lower, ".sig") ||
		strings.HasSuffix(lower, ".dgst") || strings.HasSuffix(lower, ".sbom") ||
		strings.HasSuffix(lower, ".pem") || strings.HasSuffix(lower, ".apk") ||
		strings.HasSuffix(lower, ".deb") || strings.HasSuffix(lower, ".rpm") ||
		strings.HasSuffix(lower, ".zst") {
		return false
	}
	// Exclude known non-binary file names
	switch lower {
	case "license", "license.txt", "license.md", "readme", "readme.md", "changelog", "changelog.md":
		return false
	}

	// 统一比较基准：小写 + 去 .exe（.exe 不在上面的排除列表里）
	stem := strings.TrimSuffix(lower, ".exe")

	switch coreType {
	case "singbox":
		// Match "sing-box" or "singbox" as the base name itself
		// (e.g. "sing-box" inside the tar.gz, not "sing-box-1.10.0/LICENSE")
		return stem == "sing-box" || stem == "singbox" ||
			strings.HasPrefix(stem, "sing-box-") || strings.HasPrefix(stem, "singbox-")
	case "hysteria2":
		return stem == "hysteria" || strings.HasPrefix(stem, "hysteria-") || strings.HasPrefix(stem, "hysteria_")
	case "tuic":
		return strings.HasPrefix(stem, "tuic")
	default:
		return strings.Contains(stem, strings.ToLower(coreType))
	}
}

// validateBinary checks that a file looks like a real executable binary
// (ELF on Linux, Mach-O on macOS, PE on Windows) rather than a text file
// that was mistakenly extracted (e.g. LICENSE, README).
func validateBinary(path, coreType string) error {
	// On Windows, skip magic check — just verify it's a reasonable size
	if runtime.GOOS == "windows" {
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if info.Size() < 1024 {
			return fmt.Errorf("file too small (%d bytes), likely not a binary", info.Size())
		}
		return nil
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	magic := make([]byte, 4)
	n, err := f.Read(magic)
	if err != nil || n < 4 {
		return fmt.Errorf("cannot read file header: %w", err)
	}

	// ELF binary: 0x7f 'E' 'L' 'F'
	if magic[0] == 0x7f && magic[1] == 'E' && magic[2] == 'L' && magic[3] == 'F' {
		return nil
	}
	// Mach-O (macOS): 0xFE 0xED 0xFA 0xCE or 0xCF 0xFA 0xED 0xFE
	if (magic[0] == 0xFE && magic[1] == 0xED && magic[2] == 0xFA && (magic[3] == 0xCE || magic[3] == 0xCF)) ||
		(magic[0] == 0xCF && magic[1] == 0xFA && magic[2] == 0xED && magic[3] == 0xFE) ||
		(magic[0] == 0xCA && magic[1] == 0xFE && magic[2] == 0xBA && (magic[3] == 0xBE || magic[3] == 0xBF)) {
		return nil
	}
	// PE (Windows): 'M' 'Z'
	if magic[0] == 'M' && magic[1] == 'Z' {
		return nil
	}

	// Not a known binary format
	return fmt.Errorf("file does not have a valid executable magic number (got: %x %x %x %x)",
		magic[0], magic[1], magic[2], magic[3])
}

// filterAsset returns true if the asset name is relevant for the core type
// and matches the target OS/arch.
func filterAsset(coreType, name, osName, archName string) bool {
	lower := strings.ToLower(name)
	// Exclude non-binary files
	if strings.HasSuffix(lower, ".txt") || strings.HasSuffix(lower, ".md") ||
		strings.HasSuffix(lower, ".sha256") || strings.HasSuffix(lower, ".sig") ||
		strings.HasSuffix(lower, ".dgst") || strings.HasSuffix(lower, ".sbom") ||
		strings.HasSuffix(lower, ".pem") || strings.HasSuffix(lower, ".apk") ||
		strings.HasSuffix(lower, ".deb") || strings.HasSuffix(lower, ".rpm") ||
		strings.HasSuffix(lower, ".pkg.tar.zst") ||
		lower == "license" || lower == "readme.md" {
		return false
	}
	if coreType == "tuic" && !strings.Contains(lower, "tuic-server") {
		return false
	}
	// Check OS match
	if !assetMatchesOS(lower, osName) {
		return false
	}
	// Check arch match
	if !assetMatchesArch(lower, archName) {
		return false
	}
	return true
}

// assetMatchesOS checks if the asset name contains the target OS.
func assetMatchesOS(lowerName, osName string) bool {
	osAliases := map[string][]string{
		"linux":   {"linux"},
		"darwin":  {"darwin", "macos", "mac"},
		"windows": {"windows", "win"},
		"freebsd": {"freebsd"},
	}
	aliases, ok := osAliases[osName]
	if !ok {
		aliases = []string{osName}
	}
	for _, a := range aliases {
		if strings.Contains(lowerName, a) {
			return true
		}
	}
	return false
}

// assetMatchesArch checks if the asset name contains the target architecture.
// Different projects use different naming conventions for the same arch.
func assetMatchesArch(lowerName, archName string) bool {
	// Try exact arch name as a segment (delimited by _ or - or .)
	if strings.Contains(lowerName, "_"+archName) ||
		strings.Contains(lowerName, "-"+archName) ||
		strings.Contains(lowerName, "."+archName) ||
		strings.HasSuffix(lowerName, archName) {
		return true
	}
	archAliases := map[string][]string{
		"amd64":    {"x86_64", "x86-64"},
		"arm64":    {"aarch64", "arm64-v8a", "armv8"},
		"arm":      {"armv7", "arm32-v7a", "armhf", "armv7hl"},
		"386":      {"i386", "i686", "x86"},
		"mips":     {"mips32"},
		"mipsle":   {"mips32le", "mipsle"},
		"mips64":   {"mips64"},
		"mips64le": {"mips64le"},
	}
	aliases, ok := archAliases[archName]
	if !ok {
		return false
	}
	for _, a := range aliases {
		if strings.Contains(lowerName, a) {
			return true
		}
	}
	// Special case: amd64 can match "64" but NOT "arm64"
	if archName == "amd64" {
		if strings.Contains(lowerName, "_64") || strings.Contains(lowerName, "-64") {
			if !strings.Contains(lowerName, "arm64") {
				return true
			}
		}
	}
	// Special case: 386 can match "32" but NOT "arm32"
	if archName == "386" {
		if strings.Contains(lowerName, "_32") || strings.Contains(lowerName, "-32") || strings.Contains(lowerName, "32.") || strings.HasSuffix(lowerName, "32") {
			if !strings.Contains(lowerName, "arm32") {
				return true
			}
		}
	}
	return false
}

// binaryNameInArchive returns the expected binary name inside a downloaded archive.
func binaryNameInArchive(coreType, osName, archName string) string {
	name := coreType + "-" + osName + "-" + archName
	if osName == "windows" {
		name += ".exe"
	}
	return name
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

// ListReleases fetches available releases from GitHub for the local platform.
func (i *Installer) ListReleases(ctx context.Context, coreType string) ([]ReleaseInfo, error) {
	return i.ListReleasesForPlatform(ctx, coreType, runtime.GOOS, runtime.GOARCH)
}

// ListReleasesForPlatform fetches available releases from GitHub,
// filtering assets by the specified OS/arch.
func (i *Installer) ListReleasesForPlatform(ctx context.Context, coreType, osName, archName string) ([]ReleaseInfo, error) {
	repoURL := githubRepo(coreType)
	if repoURL == "" {
		return nil, fmt.Errorf("unsupported core type: %s", coreType)
	}

	accelDomain := i.githubAccelDomain(ctx)
	proxyURL := i.githubProxyURL(ctx)
	apiURL := netutil.AccelerateGitHubURL(
		fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=20", repoURL),
		accelDomain,
	)
	client := netutil.GitHubHTTPClientWithProxy(30*time.Second, accelDomain, proxyURL)
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
	if err := json.NewDecoder(io.LimitReader(resp.Body, netutil.MaxJSONResponseBytes)).Decode(&ghReleases); err != nil {
		return nil, err
	}

	var result []ReleaseInfo
	for _, r := range ghReleases {
		// Only include releases that have at least one matching asset
		hasMatch := false
		for _, a := range r.Assets {
			if filterAsset(coreType, a.Name, osName, archName) {
				hasMatch = true
				break
			}
		}
		if hasMatch {
			result = append(result, ReleaseInfo{
				Version:    r.TagName,
				Prerelease: r.Prerelease,
			})
		}
	}
	return result, nil
}
