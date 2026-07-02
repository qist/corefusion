package binaries

import (
	"context"
	"net"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	binariesSvc "corefusion/internal/service/binaries"
)

// SettingStore provides access to system settings.
type SettingStore interface {
	GetByKey(ctx context.Context, key string) (string, error)
}

// Handler handles binary management HTTP requests.
// NOTE: Binary download/install logic lives in binaries.Installer and is
// exposed via the node-server API (handlers/nodeserver). This handler only
// provides local-only operations: key generation, binary listing, etc.
type Handler struct {
	resolver *binariesSvc.Resolver
	dataDir  string
}

// NewHandler creates a new binary handler.
func NewHandler(resolver *binariesSvc.Resolver, dataDir string, settingSvc SettingStore) *Handler {
	absDir := dataDir
	if !filepath.IsAbs(dataDir) {
		if abs, err := filepath.Abs(dataDir); err == nil {
			absDir = abs
		}
	}
	return &Handler{resolver: resolver, dataDir: absDir}
}

// List handles GET /api/v1/binaries.
func (h *Handler) List(c *gin.Context) {
	binaries := h.resolver.ListInstalled()
	response.OK(c, binaries)
}

// GenerateEchKeys handles POST /api/v1/binaries/ech-generate.
func (h *Handler) GenerateEchKeys(c *gin.Context) {
	var req struct {
		ServerName string `json:"server_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ServerName == "" {
		response.BadRequest(c, "INVALID_PARAMS", "server_name is required")
		return
	}

	binaryPath, _, err := h.resolver.Resolve("xray")
	if err != nil {
		response.NotFound(c, "BINARY_NOT_FOUND", err.Error())
		return
	}

	cmd := exec.Command(binaryPath, "tls", "ech", "--serverName", req.ServerName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		response.InternalError(c, "ECH_GENERATE_FAILED", strings.TrimSpace(string(out)))
		return
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 4 {
		response.InternalError(c, "ECH_GENERATE_FAILED", "invalid ech output")
		return
	}

	result := map[string]string{
		"echConfigList": strings.TrimSpace(lines[1]),
		"echServerKeys": strings.TrimSpace(lines[3]),
	}
	response.OK(c, result)
}

// GenerateX25519Key handles POST /api/v1/binaries/x25519-key.
func (h *Handler) GenerateX25519Key(c *gin.Context) {
	binaryPath, _, err := h.resolver.Resolve("xray")
	if err != nil {
		response.NotFound(c, "BINARY_NOT_FOUND", err.Error())
		return
	}

	cmd := exec.Command(binaryPath, "x25519")
	out, err := cmd.Output()
	if err != nil {
		response.InternalError(c, "X25519_FAILED", err.Error())
		return
	}

	result := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		switch {
		case strings.Contains(key, "PrivateKey"):
			result["privateKey"] = val
		case strings.Contains(key, "PublicKey") || strings.Contains(key, "Password"):
			result["password"] = val
		}
	}
	response.OK(c, result)
}

// GenerateMldsa65Key handles POST /api/v1/binaries/mldsa65-key.
func (h *Handler) GenerateMldsa65Key(c *gin.Context) {
	binaryPath, _, err := h.resolver.Resolve("xray")
	if err != nil {
		response.NotFound(c, "BINARY_NOT_FOUND", err.Error())
		return
	}

	cmd := exec.Command(binaryPath, "mldsa65")
	out, err := cmd.Output()
	if err != nil {
		response.InternalError(c, "MLDSA65_FAILED", err.Error())
		return
	}

	result := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		switch key {
		case "Seed":
			result["seed"] = val
		case "Verify":
			result["verify"] = val
		}
	}
	response.OK(c, result)
}

// GenerateWgKey handles POST /api/v1/binaries/wg-key.
func (h *Handler) GenerateWgKey(c *gin.Context) {
	coreType := c.Query("core_type")
	if coreType == "" {
		coreType = "xray"
	}
	binaryPath, _, err := h.resolver.Resolve(coreType)
	if err != nil {
		response.NotFound(c, "BINARY_NOT_FOUND", err.Error())
		return
	}

	cmd := exec.Command(binaryPath, "wg")
	out, err := cmd.Output()
	if err != nil {
		response.InternalError(c, "WG_KEY_FAILED", err.Error())
		return
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	result := map[string]string{}
	for _, line := range lines {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			if key == "PrivateKey" {
				result["privateKey"] = val
			} else if strings.Contains(key, "PublicKey") {
				result["publicKey"] = val
			}
		}
	}
	response.OK(c, result)
}

// GenerateVlessEnc handles POST /api/v1/binaries/vlessenc.
func (h *Handler) GenerateVlessEnc(c *gin.Context) {
	binaryPath, _, err := h.resolver.Resolve("xray")
	if err != nil {
		response.NotFound(c, "BINARY_NOT_FOUND", err.Error())
		return
	}

	cmd := exec.Command(binaryPath, "vlessenc")
	out, err := cmd.CombinedOutput()
	if err != nil {
		response.InternalError(c, "VLESSENC_FAILED", strings.TrimSpace(string(out)))
		return
	}

	auths := parseVlessEncAuths(string(out))
	response.OK(c, gin.H{"auths": auths})
}

// parseVlessEncAuths parses the output of `xray vlessenc` command.
func parseVlessEncAuths(output string) []map[string]string {
	lines := strings.Split(output, "\n")
	var auths []map[string]string
	var current map[string]string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Authentication:") {
			if current != nil {
				auths = append(auths, current)
			}
			label := strings.TrimSpace(strings.TrimPrefix(line, "Authentication:"))
			current = map[string]string{
				"id":    vlessEncAuthID(label),
				"label": label,
			}
		} else if strings.HasPrefix(line, `"decryption"`) || strings.HasPrefix(line, `"encryption"`) {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 && current != nil {
				key := strings.Trim(parts[0], `" `)
				val := strings.TrimSpace(parts[1])
				val = strings.TrimSuffix(val, ",")
				val = strings.Trim(val, `" `)
				current[key] = val
			}
		}
	}

	if current != nil {
		auths = append(auths, current)
	}

	return auths
}

// vlessEncAuthID extracts a short ID from the auth label.
func vlessEncAuthID(label string) string {
	normalized := strings.NewReplacer("-", "", "_", "", " ", "").Replace(strings.ToLower(label))
	switch {
	case strings.Contains(normalized, "mlkem768"):
		return "mlkem768"
	case strings.Contains(normalized, "x25519"):
		return "x25519"
	default:
		return normalized
	}
}

// ListInterfaces handles GET /api/v1/binaries/interfaces.
func (h *Handler) ListInterfaces(c *gin.Context) {
	ifaces, err := net.Interfaces()
	if err != nil {
		response.InternalError(c, "INTERFACES_FAILED", err.Error())
		return
	}
	names := []string{}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		names = append(names, iface.Name)
	}
	response.OK(c, names)
}

// NOTE: Download, Install, Update, Rollback, ListReleases, downloadAndExtract,
// extractZip, filterAssets, githubAccelDomain, githubRepo, buildDownloadURL,
// getServiceID, ProcessManager, SetProcessManager have been REMOVED.
// All binary install/download/list-releases operations now go through
// the nodeserver handler which delegates to binaries.Installer (with proxy support).
