package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/constants"
	"corefusion/internal/runtime/agent"
	"corefusion/internal/service/binaries"
	"corefusion/pkg/netutil"
)

// Handler handles Agent binary download and install script endpoints.
// These are public endpoints (no auth) — security is provided by the
// Bootstrap Token which is embedded in the install command.
type Handler struct {
	dataDir    string
	tokenMgr   *agent.BootstrapTokenManager
	settingSvc binaries.SettingStore // for GitHub acceleration and panel settings
}

// NewHandler creates a new Agent handler.
func NewHandler(dataDir string, tokenMgr *agent.BootstrapTokenManager) *Handler {
	return &Handler{
		dataDir:  dataDir,
		tokenMgr: tokenMgr,
	}
}

// WithSettingStore sets the system setting store for GitHub acceleration.
func (h *Handler) WithSettingStore(s binaries.SettingStore) *Handler {
	h.settingSvc = s
	return h
}

// DownloadAgent serves the Agent binary from the panel's local directory.
// This is primarily for development/testing. In production, the install
// script falls back to GitHub Releases when this endpoint returns 404.
//
// GET /api/v1/agent/download?os=linux&arch=amd64
func (h *Handler) DownloadAgent(c *gin.Context) {
	osName := c.DefaultQuery("os", "linux")
	arch := c.DefaultQuery("arch", "amd64")

	if osName != "linux" || (arch != "amd64" && arch != "arm64") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported os/arch"})
		return
	}

	agentDir := filepath.Join(h.dataDir, "binaries", "active", "corefusion-agent")
	candidates := []string{
		filepath.Join(agentDir, fmt.Sprintf("corefusion-agent-%s-%s", osName, arch)),
		filepath.Join(agentDir, "corefusion-agent"),
	}

	for _, p := range candidates {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			c.File(p)
			return
		}
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "agent binary not found on panel"})
}

// GetInstallScript generates and returns the Agent install script.
// The Bootstrap Token is verified (but not consumed) to ensure the
// request is legitimate. The token is consumed later when the Agent
// actually boots and calls POST /api/v1/agent/bootstrap.
//
// GET /api/v1/agent/install?id=<server_id>&token=<bootstrap_token>
func (h *Handler) GetInstallScript(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Query("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid server id"})
		return
	}

	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token required"})
		return
	}

	if h.tokenMgr == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token manager not initialized"})
		return
	}

	// Verify token without consuming it
	if !h.tokenMgr.VerifyToken(c.Request.Context(), serverID, token) {
		c.JSON(http.StatusForbidden, gin.H{"error": "invalid or expired token"})
		return
	}

	// Determine panel address.
	// Behind a reverse proxy (Caddy/Nginx), c.Request.TLS is always nil,
	// so we check X-Forwarded-Proto first, then query param, then TLS.
	panelHost := c.Query("host")
	if panelHost == "" {
		// X-Forwarded-Host (set by proxy) takes priority
		if fh := c.GetHeader("X-Forwarded-Host"); fh != "" {
			panelHost = stripPort(fh)
		} else {
			panelHost = stripPort(c.Request.Host)
		}
	}

	// Detect TLS: X-Forwarded-Proto > query param > c.Request.TLS
	panelTLS := false
	if fp := c.GetHeader("X-Forwarded-Proto"); fp == "https" {
		panelTLS = true
	} else if c.Query("tls") == "true" {
		panelTLS = true
	} else if c.Request.TLS != nil {
		panelTLS = true
	}

	scheme := "http"
	if panelTLS {
		scheme = "https"
	}

	// Determine panel HTTP port
	// Priority: query param > X-Forwarded-Port > port in Host header > TLS 443 > 8080
	panelPort := 8080
	if p := c.Query("port"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			panelPort = v
		}
	} else if fp := c.GetHeader("X-Forwarded-Port"); fp != "" {
		if v, err := strconv.Atoi(fp); err == nil {
			panelPort = v
		}
	} else if _, portStr, err := net.SplitHostPort(c.Request.Host); err == nil {
		if v, err := strconv.Atoi(portStr); err == nil {
			panelPort = v
		}
	} else if panelTLS {
		panelPort = 443
	}

	// Determine gRPC port: query param > system setting > default 18443
	grpcPort := 0
	if g := c.Query("grpc_port"); g != "" {
		if v, err := strconv.Atoi(g); err == nil {
			grpcPort = v
		}
	}
	if grpcPort == 0 && h.settingSvc != nil {
		if v, err := h.settingSvc.GetByKey(c.Request.Context(), "panel.agent_grpc_port"); err == nil && v != "" {
			if p, err := strconv.Atoi(v); err == nil && p > 0 {
				grpcPort = p
			}
		}
	}
	if grpcPort == 0 {
		grpcPort = constants.DefaultAgentGRPCPort
	}
	script := agent.NewAgentInstallScript(panelHost, panelPort).
		WithScheme(scheme).
		WithGRPCPort(grpcPort).
		WithTLS(panelTLS).
		WithBootstrapToken(token).
		// 面板 CA 自动随脚本下发 → Agent 引导阶段即可校验面板身份（无需任何人工填写）
		WithCACertPEM(h.tokenMgr.CACertPEM()).
		WithServerID(serverID).
		WithGitHubURL("https://github.com/qist/corefusion/releases/download")

	content, err := script.Generate()
	if err != nil {
		response.InternalError(c, "SCRIPT_GEN_FAILED", err.Error())
		return
	}

	c.Data(http.StatusOK, "text/x-shellscript", []byte(content))
}

// stripPort removes the port from a host:port string.
func stripPort(hostPort string) string {
	if idx := strings.LastIndex(hostPort, ":"); idx > 0 {
		return hostPort[:idx]
	}
	return hostPort
}

// ============================================================
// Agent Version & Releases API
// ============================================================

// AgentGitHubRepo 是 Agent 与面板共用的发布仓库。
// 产物命名见 .github/workflows/release.yml：corefusion-agent-{os}-{arch}（附同名 .sha256）。
const AgentGitHubRepo = agent.AgentGitHubRepo

// agentReleaseAsset 是版本列表中的单个 Agent 产物。
type agentReleaseAsset struct {
	Name        string `json:"name"`
	OS          string `json:"os,omitempty"`
	Arch        string `json:"arch,omitempty"`
	Size        int64  `json:"size"`
	DownloadURL string `json:"download_url"`
	ChecksumURL string `json:"checksum_url,omitempty"`
	Checksum    string `json:"checksum,omitempty"`
}

// agentRelease 是 Agent 版本列表中的一项。
type agentRelease struct {
	Version     string              `json:"version"`
	Name        string              `json:"name"`
	Prerelease  bool                `json:"prerelease"`
	PublishedAt string              `json:"published_at"`
	Notes       string              `json:"notes"`
	Assets      []agentReleaseAsset `json:"assets"`
}

// GetAgentVersion returns the panel's locally stored Agent binary version info.
// GET /api/v1/agent/version
func (h *Handler) GetAgentVersion(c *gin.Context) {
	agentDir := filepath.Join(h.dataDir, "binaries", "active", "corefusion-agent")

	// Try platform-specific binary first, then generic name
	candidates := []string{
		filepath.Join(agentDir, fmt.Sprintf("corefusion-agent-%s-%s", runtime.GOOS, runtime.GOARCH)),
		filepath.Join(agentDir, "corefusion-agent"),
	}

	var agentPath string
	var info os.FileInfo
	for _, p := range candidates {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			agentPath = p
			info = fi
			break
		}
	}

	if agentPath == "" {
		response.NotFound(c, "AGENT_BINARY_NOT_FOUND", "agent binary not found on panel")
		return
	}

	checksum, _ := computeSHA256(agentPath)
	version := h.detectAgentVersion(agentDir)

	response.OK(c, gin.H{
		"version":      version,
		"checksum":     checksum,
		"binary_size":  info.Size(),
		"binary_path":  agentPath,
		"built_at":     info.ModTime().Format(time.RFC3339),
		"download_url": fmt.Sprintf("/api/v1/agent/download?os=linux&arch=%s", runtime.GOARCH),
	})
}

// GetAgentReleases fetches available Agent releases from GitHub.
// GET /api/v1/agent/releases?limit=20
func (h *Handler) GetAgentReleases(c *gin.Context) {
	limit := 20
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}

	// Read GitHub acceleration settings
	accelDomain := h.githubAccelDomain(c)
	proxyURL := h.githubProxyURL(c)

	apiURL := netutil.AccelerateGitHubURL(
		fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=%d", AgentGitHubRepo, limit),
		accelDomain,
	)
	client := netutil.GitHubHTTPClientWithProxy(30*time.Second, accelDomain, proxyURL)

	req, _ := http.NewRequestWithContext(c.Request.Context(), "GET", apiURL, nil)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		response.InternalError(c, "GITHUB_API_FAILED", err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		// Repository has no releases yet — return empty list
		response.OK(c, gin.H{
			"releases":       []interface{}{},
			"latest_version": "",
			"repo":           AgentGitHubRepo,
			"source":         "github",
		})
		return
	}
	if resp.StatusCode != 200 {
		response.InternalError(c, "GITHUB_API_ERROR", fmt.Sprintf("GitHub API returned %d", resp.StatusCode))
		return
	}

	var ghReleases []binaries.GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&ghReleases); err != nil {
		response.InternalError(c, "PARSE_FAILED", err.Error())
		return
	}

	// 只保留含 Agent 产物的版本；每个产物带上 os/arch 与 sha256（digest 或 .sha256 地址）。
	result := make([]agentRelease, 0, len(ghReleases))
	latest := ""
	for _, r := range ghReleases {
		if r.Draft {
			continue
		}
		item := agentRelease{
			Version:     r.TagName,
			Name:        r.Name,
			Prerelease:  r.Prerelease,
			PublishedAt: r.PublishedAt,
			Notes:       r.Body,
		}
		checksumURLs := make(map[string]string, len(r.Assets))
		for _, a := range r.Assets {
			if strings.HasSuffix(a.Name, ".sha256") {
				checksumURLs[strings.TrimSuffix(a.Name, ".sha256")] = a.BrowserDownloadURL
			}
		}
		for _, a := range r.Assets {
			if !strings.HasPrefix(a.Name, "corefusion-agent-") || strings.HasSuffix(a.Name, ".sha256") {
				continue
			}
			osName, arch, ok := parseAgentAssetName(a.Name)
			if !ok {
				continue
			}
			item.Assets = append(item.Assets, agentReleaseAsset{
				Name:        a.Name,
				OS:          osName,
				Arch:        arch,
				Size:        a.Size,
				DownloadURL: a.BrowserDownloadURL,
				ChecksumURL: checksumURLs[a.Name],
				Checksum:    normalizeSHA256(a.Digest),
			})
		}
		if latest == "" {
			latest = r.TagName
		}
		if len(item.Assets) > 0 {
			result = append(result, item)
		}
	}

	response.OK(c, gin.H{
		"releases":       result,
		"latest_version": latest,
		"repo":           AgentGitHubRepo,
		"source":         "github",
	})
}

// parseAgentAssetName 从 corefusion-agent-{os}-{arch} 解析出平台。
func parseAgentAssetName(name string) (osName, arch string, ok bool) {
	rest, found := strings.CutPrefix(name, "corefusion-agent-")
	if !found {
		return "", "", false
	}
	idx := strings.LastIndex(rest, "-")
	if idx <= 0 || idx == len(rest)-1 {
		return "", "", false
	}
	return rest[:idx], rest[idx+1:], true
}

// normalizeSHA256 归一化 "sha256:xxx" / 纯 hash 形式的校验和。
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

// FetchAgentChecksum attempts to download a .sha256 checksum file for the
// given download URL. Returns "" if not available.
func (h *Handler) FetchAgentChecksum(downloadURL string) string {
	checksumURL := downloadURL + ".sha256"
	client := netutil.NewHTTPClient(30 * time.Second)
	resp, err := client.Get(checksumURL)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return ""
	}
	data, _ := io.ReadAll(resp.Body)
	parts := strings.Fields(string(data))
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

// detectAgentVersion reads the VERSION file from the agent binary directory.
func (h *Handler) detectAgentVersion(agentDir string) string {
	versionFile := filepath.Join(agentDir, "VERSION")
	data, err := os.ReadFile(versionFile)
	if err == nil {
		v := strings.TrimSpace(string(data))
		if v != "" {
			return v
		}
	}
	return "unknown"
}

// githubAccelDomain reads the GitHub acceleration domain from system settings.
func (h *Handler) githubAccelDomain(c *gin.Context) string {
	if h.settingSvc == nil {
		return ""
	}
	val, err := h.settingSvc.GetByKey(c.Request.Context(), "github.acceleration")
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

// githubProxyURL reads the GitHub HTTP/SOCKS proxy URL from system settings.
func (h *Handler) githubProxyURL(c *gin.Context) string {
	if h.settingSvc == nil {
		return ""
	}
	val, err := h.settingSvc.GetByKey(c.Request.Context(), "github.acceleration")
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

// computeSHA256 calculates the SHA-256 hash of a file.
func computeSHA256(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}
