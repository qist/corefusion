package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
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

	// detectPublicIP / probeGRPC 可在测试里替换；生产环境分别走本机公网 IP
	// 探测与真实 TCP 探测。
	detectPublicIP func(context.Context, time.Duration) (string, string, error)
	probeGRPC      func(context.Context, string, int) bool

	grpcHostMu    sync.Mutex
	grpcHostCache map[string]grpcHostCacheEntry

	// publicOriginHost cache for the unauthenticated origin-discovery endpoint.
	originMu          sync.Mutex
	originHost        string
	originExpiresAt   time.Time
	originLastAttempt time.Time
}

type grpcHostCacheEntry struct {
	host      string
	expiresAt time.Time
}

const (
	agentGRPCHostCacheTTL = 10 * time.Minute
	agentPublicIPTimeout  = 3 * time.Second
	agentGRPCProbeTimeout = 1500 * time.Millisecond
)

// NewHandler creates a new Agent handler.
func NewHandler(dataDir string, tokenMgr *agent.BootstrapTokenManager) *Handler {
	return &Handler{
		dataDir:        dataDir,
		tokenMgr:       tokenMgr,
		detectPublicIP: netutil.DetectPublicIPDirect,
		probeGRPC:      probeAgentGRPC,
		grpcHostCache:  make(map[string]grpcHostCacheEntry),
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
	panelHost := stripPort(c.Query("host"))
	if panelHost == "" {
		// X-Forwarded-Host (set by proxy) takes priority
		if fh := c.GetHeader("X-Forwarded-Host"); fh != "" {
			// Multiple proxies may append a comma-separated list; the first
			// entry is the externally visible host.
			if i := strings.IndexByte(fh, ','); i >= 0 {
				fh = fh[:i]
			}
			panelHost = stripPort(fh)
		} else {
			panelHost = stripPort(c.Request.Host)
		}
	}
	// panelHost is embedded into the generated shell script. Validate it even
	// though the bootstrap token is required: Host/X-Forwarded-Host are
	// attacker-controlled on a proxied public endpoint.
	panelHost = normalizeAgentHost(panelHost)
	if panelHost == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid panel host"})
		return
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
		if v, err := strconv.Atoi(p); err == nil && v > 0 && v <= 65535 {
			panelPort = v
		}
	} else if fp := c.GetHeader("X-Forwarded-Port"); fp != "" {
		if v, err := strconv.Atoi(fp); err == nil && v > 0 && v <= 65535 {
			panelPort = v
		}
	} else if _, portStr, err := net.SplitHostPort(c.Request.Host); err == nil {
		if v, err := strconv.Atoi(portStr); err == nil && v > 0 && v <= 65535 {
			panelPort = v
		}
	} else if panelTLS {
		panelPort = 443
	}

	// Determine gRPC port:
	// query param > panel-wide setting > node's agent_port > default 18443
	grpcPort := 0
	if g := c.Query("grpc_port"); g != "" {
		if v, err := strconv.Atoi(g); err == nil && v > 0 && v <= 65535 {
			grpcPort = v
		}
	}
	if grpcPort == 0 && h.settingSvc != nil {
		if v, err := h.settingSvc.GetByKey(c.Request.Context(), "panel.agent_grpc_port"); err == nil && v != "" {
			if p, err := strconv.Atoi(v); err == nil && p > 0 && p <= 65535 {
				grpcPort = p
			}
		}
	}
	if grpcPort == 0 {
		if p, err := h.tokenMgr.AgentPort(c.Request.Context(), serverID); err == nil && p > 0 && p <= 65535 {
			grpcPort = p
		}
	}
	if grpcPort == 0 {
		grpcPort = constants.DefaultAgentGRPCPort
	}

	// Resolve the address used for gRPC. panelHost is also the HTTP(S) address
	// used for bootstrap/download and may be a CDN domain. A normal CDN does
	// not proxy the gRPC port, so if panelHost:<grpcPort> is not reachable,
	// fall back to this panel's public IP (the origin). This is what removes
	// the need to edit every agent's YAML by hand.
	grpcHost := ""
	if q := c.Query("grpc_host"); q != "" {
		grpcHost = normalizeAgentHost(q)
		if grpcHost == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid grpc_host"})
			return
		}
	} else {
		grpcHost = h.agentGRPCHost(c.Request.Context(), panelHost, grpcPort)
	}
	if grpcHost == "" {
		grpcHost = panelHost
	}
	if grpcHost != panelHost {
		log.Printf("[AgentInstall] %s:%d is not usable for gRPC (CDN?), using panel origin %s:%d",
			panelHost, grpcPort, grpcHost, grpcPort)
	}

	// Prefer the panel's direct listener for bootstrap: it is the strictest
	// path (the Agent verifies the panel with the panel CA). The config also
	// carries the public CDN endpoint as a fallback for nodes that cannot reach
	// the origin's HTTP port; the Agent verifies the issued Agent certificate
	// against the panel CA, so a TLS-terminating proxy cannot substitute one.
	//
	// If panelHost already serves gRPC directly (LAN/direct deployment), keep
	// it; otherwise use the detected origin IP.
	bootstrapHost := panelHost
	if grpcHost != panelHost {
		bootstrapHost = grpcHost
	}
	bootstrapPort, bootstrapTLS := h.panelLocalHTTP(c.Request.Context())

	script := agent.NewAgentInstallScript(panelHost, panelPort).
		WithScheme(scheme).
		WithBootstrapEndpoint(bootstrapHost, bootstrapPort, bootstrapTLS).
		WithGRPCHost(grpcHost).
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

// GetAgentOrigin lets an already-installed Agent discover the panel's gRPC
// origin when its config only contains panel.host (for example an older node
// installed before grpc_host existed, where panel.host is the CDN domain).
// The response is intentionally public: it only reveals the panel's public
// address, and the subsequent gRPC connection is still authenticated with the
// panel CA. It must not be cached by a CDN.
//
// GET /api/v1/agent/origin?grpc_port=18443
func (h *Handler) GetAgentOrigin(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	grpcPort := constants.DefaultAgentGRPCPort
	if v, err := strconv.Atoi(c.Query("grpc_port")); err == nil && v > 0 && v <= 65535 {
		grpcPort = v
	}
	host := h.publicOriginHost(c.Request.Context())
	if host == "" {
		response.Error(c, http.StatusServiceUnavailable, "ORIGIN_DETECTION_FAILED",
			"panel origin address is not available")
		return
	}
	response.OK(c, gin.H{"grpc_host": host, "grpc_port": grpcPort})
}

// publicOriginHost returns the panel's own public IP. Unlike the install
// endpoint it deliberately ignores the request Host: this endpoint is public
// and unauthenticated, so probing an attacker-supplied host would turn it into
// an SSRF/scanning primitive. The result is cached briefly to avoid repeated
// external IP lookups.
func (h *Handler) publicOriginHost(ctx context.Context) string {
	now := time.Now()
	h.originMu.Lock()
	if h.originHost != "" && now.Before(h.originExpiresAt) {
		host := h.originHost
		h.originMu.Unlock()
		return host
	}
	// Also throttle failed lookups: this endpoint is unauthenticated, and the
	// public-IP probe makes outbound connections.
	if !h.originLastAttempt.IsZero() && now.Sub(h.originLastAttempt) < 10*time.Second {
		h.originMu.Unlock()
		return ""
	}
	h.originLastAttempt = now
	h.originMu.Unlock()

	if h.detectPublicIP == nil {
		return ""
	}
	// Probe IPv4 and IPv6 sequentially; each gets agentPublicIPTimeout, so the
	// outer budget must be larger than a single probe.
	detectCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	ipv4, ipv6, err := h.detectPublicIP(detectCtx, agentPublicIPTimeout)
	if err != nil {
		return ""
	}
	host := strings.TrimSpace(ipv4)
	if host == "" {
		host = strings.TrimSpace(ipv6)
	}
	if net.ParseIP(host) == nil {
		return ""
	}
	h.originMu.Lock()
	h.originHost = host
	h.originExpiresAt = time.Now().Add(agentGRPCHostCacheTTL)
	h.originMu.Unlock()
	return host
}

// panelLocalHTTP returns the panel's own HTTP listen port and TLS flag.
// These are deliberately read from the panel settings instead of the incoming
// request: behind a CDN/reverse proxy the request describes the public 443
// endpoint, while the Agent's first bootstrap attempt targets the direct
// listener whose certificate it can verify with the panel CA.
func (h *Handler) panelLocalHTTP(ctx context.Context) (int, bool) {
	const defaultPort = 8080
	port := defaultPort
	tlsEnabled := false
	if h.settingSvc == nil {
		return port, tlsEnabled
	}
	if v, err := h.settingSvc.GetByKey(ctx, "panel.listen_port"); err == nil {
		if p, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && p > 0 && p <= 65535 {
			port = p
		}
	}
	if v, err := h.settingSvc.GetByKey(ctx, "panel.tls_enabled"); err == nil {
		tlsEnabled = strings.EqualFold(strings.TrimSpace(v), "true")
	}
	return port, tlsEnabled
}

// agentGRPCHost returns the host the Agent should use for gRPC. The result is
// cached because reachability/public-IP detection involves network I/O and is
// requested every time the admin opens a deploy command.
func (h *Handler) agentGRPCHost(ctx context.Context, panelHost string, grpcPort int) string {
	key := net.JoinHostPort(panelHost, strconv.Itoa(grpcPort))

	h.grpcHostMu.Lock()
	if e, ok := h.grpcHostCache[key]; ok && time.Now().Before(e.expiresAt) {
		host := e.host
		h.grpcHostMu.Unlock()
		return host
	}
	h.grpcHostMu.Unlock()

	host := resolveAgentGRPCHost(ctx, panelHost, grpcPort, h.probeGRPC, h.detectPublicIP)
	if host == "" {
		host = panelHost
	}

	h.grpcHostMu.Lock()
	if h.grpcHostCache == nil {
		h.grpcHostCache = make(map[string]grpcHostCacheEntry)
	}
	h.grpcHostCache[key] = grpcHostCacheEntry{
		host:      host,
		expiresAt: time.Now().Add(agentGRPCHostCacheTTL),
	}
	h.grpcHostMu.Unlock()
	return host
}

// resolveAgentGRPCHost keeps panelHost when the gRPC port is reachable through
// it (direct deployment, LAN, or a CDN that supports TCP forwarding). If it is
// not reachable — the usual case with a normal HTTP CDN — it returns the
// panel's detected public IP instead.
func resolveAgentGRPCHost(
	ctx context.Context,
	panelHost string,
	grpcPort int,
	probe func(context.Context, string, int) bool,
	detect func(context.Context, time.Duration) (string, string, error),
) string {
	if panelHost == "" {
		return ""
	}
	if ip := net.ParseIP(strings.Trim(panelHost, "[]")); ip != nil {
		return ip.String()
	}
	if probe != nil && probe(ctx, panelHost, grpcPort) {
		return panelHost
	}
	if detect != nil {
		detectCtx, cancel := context.WithTimeout(ctx, agentPublicIPTimeout)
		defer cancel()
		ipv4, ipv6, err := detect(detectCtx, agentPublicIPTimeout)
		if err == nil {
			if ipv4 != "" && net.ParseIP(ipv4) != nil {
				return ipv4
			}
			if ipv6 != "" && net.ParseIP(ipv6) != nil {
				return ipv6
			}
		}
	}
	return panelHost
}

// probeAgentGRPC checks whether the panel's gRPC port is reachable through the
// given host. A normal CDN only proxies HTTP(S), so this fails and the install
// script switches to the origin IP.
func probeAgentGRPC(ctx context.Context, host string, port int) bool {
	d := net.Dialer{Timeout: agentGRPCProbeTimeout}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// normalizeAgentHost validates a host before it is embedded into the generated
// shell script. Only IP literals and DNS names are accepted, which prevents a
// crafted Host header / query parameter from injecting shell commands.
func normalizeAgentHost(raw string) string {
	host := strings.Trim(strings.TrimSpace(raw), "[]")
	if ip := net.ParseIP(host); ip != nil {
		return ip.String()
	}
	host = strings.TrimSuffix(host, ".")
	if host == "" || len(host) > 253 {
		return ""
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return ""
		}
		for _, r := range label {
			switch {
			case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-':
			default:
				return ""
			}
		}
	}
	return strings.ToLower(host)
}

// stripPort removes the port from a host:port string.
func stripPort(hostPort string) string {
	hostPort = strings.TrimSpace(hostPort)
	if host, _, err := net.SplitHostPort(hostPort); err == nil {
		return host
	}
	return strings.Trim(hostPort, "[]")
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
