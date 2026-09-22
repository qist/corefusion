package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"corefusion/internal/store/repository"
	"corefusion/pkg/netutil"
)

// SelfUpdater handles the Agent's self-update process.
//
// The update flow is:
//  1. Download the new binary to a temp file
//  2. Verify SHA-256 checksum
//  3. Backup the current binary to .bak
//  4. Replace the current binary with the new one
//  5. Restart the Agent process (systemd or re-exec)
//
// If the new binary crashes on startup, CheckCrashRecovery() detects this
// on the next boot and rolls back to the .bak binary.
type SelfUpdater struct {
	dataDir     string
	settingRepo *repository.SystemSettingRepository
}

// NewSelfUpdater creates a new SelfUpdater.
func NewSelfUpdater(dataDir string, settingRepo *repository.SystemSettingRepository) *SelfUpdater {
	return &SelfUpdater{
		dataDir:     dataDir,
		settingRepo: settingRepo,
	}
}

// PerformUpdate executes the full update flow:
// download → verify → backup → replace → restart
//
// On success, the Agent process is restarted and this function does not return
// (the process exits). On failure, the old binary remains in place and the
// error is returned.
func (u *SelfUpdater) PerformUpdate(version, downloadURL, checksum string) error {
	log.Printf("[SelfUpdater] starting update: version=%s, url=%s", version, downloadURL)

	// Step 0: 解析期望校验和 —— **全自动**，不需要任何人手工填写 ✗
	//   ① 面板下发的 checksum 优先（面板扫描二进制时已自动算出 SHA-256）；
	//   ② 没带就自动去取 `downloadURL + ".sha256"`（与面板侧 fetchAgentChecksum 同一约定）；
	//   ③ 两者都拿不到 → **拒绝更新**（宁可保持当前版本，也不装一个无法校验的二进制）。
	expected, err := u.resolveChecksum(downloadURL, checksum)
	if err != nil {
		return fmt.Errorf("checksum unavailable, refusing update: %w", err)
	}

	// Step 1: Download
	tmpPath, err := u.downloadFile(downloadURL)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer os.Remove(tmpPath)
	log.Printf("[SelfUpdater] downloaded to %s", tmpPath)

	// Step 2: Verify checksum（现在一定校验 ✓）
	if err := u.verifyChecksum(tmpPath, expected); err != nil {
		return fmt.Errorf("checksum verification failed: %w", err)
	}
	log.Printf("[SelfUpdater] checksum verified (%s…)", expected[:12])

	// Step 3 + 4: Backup and replace
	if err := u.replaceBinary(tmpPath); err != nil {
		return fmt.Errorf("replace binary failed: %w", err)
	}
	log.Printf("[SelfUpdater] binary replaced successfully")

	// Step 5: Restart (this does not return on success)
	if err := u.restart(); err != nil {
		return fmt.Errorf("restart failed: %w", err)
	}

	return nil
}

// downloadFile downloads the binary from the given URL to a temp file.
// If the URL is a GitHub URL, GitHub acceleration domain and proxy settings
// are automatically applied from the Agent's local settings.
func (u *SelfUpdater) downloadFile(downloadURL string) (string, error) {
	// ⚠ 不再用固定的 /tmp/corefusion-agent-new ✗：那是全世界可写的目录 + 可预测文件名，
	// 本机其他用户可以抢先占位/符号链接攻击。改为在 Agent 自己的数据目录里创建**随机名**临时文件
	// （目录本身 0750、由 Agent 独占），并立刻收紧到 0600 ✓。
	tmpFile, err := os.CreateTemp(u.dataDir, "agent-update-*")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	if err := tmpFile.Chmod(0600); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return "", fmt.Errorf("chmod temp file: %w", err)
	}

	// Choose HTTP client based on URL source（与取校验和共用同一套选择逻辑）
	client := u.clientForURL(downloadURL)
	if netutil.IsGitHubURL(downloadURL) {
		// Apply acceleration domain rewrite
		downloadURL = netutil.AccelerateGitHubURL(downloadURL, u.getGithubAccelDomain())
	}

	resp, err := client.Get(downloadURL)
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return "", fmt.Errorf("HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		tmpFile.Close()
		os.Remove(tmpPath)
		return "", fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return "", fmt.Errorf("write temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("close temp file: %w", err)
	}

	return tmpPath, nil
}

// resolveChecksum 解析出"期望的 SHA-256"，全程自动、不需要人工填：
//  1. 面板下发的 checksum（面板扫描二进制时自动算的）；
//  2. 否则自动取 `downloadURL + ".sha256"`；
//  3. 都没有 → 返回错误（调用方据此拒绝更新）。
func (u *SelfUpdater) resolveChecksum(downloadURL, provided string) (string, error) {
	if c := normalizeChecksum(provided); c != "" {
		return c, nil
	}

	sidecar := downloadURL + ".sha256"
	client := u.clientForURL(sidecar)
	resp, err := client.Get(sidecar)
	if err != nil {
		return "", fmt.Errorf("fetch %s: %w", sidecar, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch %s: HTTP %d", sidecar, resp.StatusCode)
	}
	body, err := netutil.ReadLimited(resp.Body, netutil.MaxChecksumBytes)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", sidecar, err)
	}
	c := parseChecksumFile(body)
	if c == "" {
		return "", fmt.Errorf("%s does not contain a SHA-256 checksum", sidecar)
	}
	log.Printf("[SelfUpdater] checksum auto-resolved from %s", sidecar)
	return c, nil
}

// clientForURL 按 URL 来源选客户端（GitHub 走加速/代理设置），下载与取校验和共用。
func (u *SelfUpdater) clientForURL(rawURL string) *http.Client {
	if netutil.IsGitHubURL(rawURL) {
		accelDomain := u.getGithubAccelDomain()
		proxyURL := u.getGithubProxyURL()
		log.Printf("[SelfUpdater] using GitHub client (accel=%s, proxy=%s)", accelDomain, proxyURL)
		return netutil.GitHubHTTPClientWithProxy(10*time.Minute, accelDomain, proxyURL)
	}
	return netutil.NewHTTPClient(10 * time.Minute)
}

// normalizeChecksum 归一化校验和字符串（去 `sha256:` 前缀/空白/大小写）。
func normalizeChecksum(s string) string {
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

// parseChecksumFile 从 `.sha256` 文件内容里取出校验和。
// 兼容 `sha256sum` 的输出格式（`<hash>  <filename>`）以及只写 hash 的形式。
func parseChecksumFile(data []byte) string {
	for _, field := range strings.Fields(string(data)) {
		if c := normalizeChecksum(field); c != "" {
			return c
		}
	}
	return ""
}

// verifyChecksum verifies the SHA-256 checksum of the downloaded file.
// The expectedChecksum may optionally have a "sha256:" prefix.
func (u *SelfUpdater) verifyChecksum(filePath, expectedChecksum string) error {
	// Strip optional "sha256:" prefix
	expected := strings.TrimPrefix(expectedChecksum, "sha256:")
	expected = strings.TrimSpace(expected)

	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	actual := hex.EncodeToString(hasher.Sum(nil))
	if actual != expected {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expected, actual)
	}

	return nil
}

// replaceBinary backs up the current binary and replaces it with the new one.
//
// The backup is stored as <binary>.bak. On Linux, replacing a running binary
// is safe — the running process keeps using the old inode via the open file
// descriptor.
func (u *SelfUpdater) replaceBinary(newBinaryPath string) error {
	selfPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}

	// Resolve symlinks to get the real path
	selfPath, err = filepath.EvalSymlinks(selfPath)
	if err != nil {
		return fmt.Errorf("resolve symlink: %w", err)
	}

	bakPath := selfPath + ".bak"

	// Remove old backup if it exists
	os.Remove(bakPath)

	// Backup current binary
	if err := os.Rename(selfPath, bakPath); err != nil {
		return fmt.Errorf("backup current binary: %w", err)
	}

	// Move new binary into place
	if err := os.Rename(newBinaryPath, selfPath); err != nil {
		// Restore backup on failure
		os.Rename(bakPath, selfPath)
		return fmt.Errorf("move new binary: %w", err)
	}

	// Ensure executable permission
	if err := os.Chmod(selfPath, 0755); err != nil {
		return fmt.Errorf("chmod: %w", err)
	}

	return nil
}

// restart restarts the Agent process.
//
// Strategy 1 (preferred): If running under systemd, use `systemctl restart`.
// The current process is killed by systemd and a new one is started.
//
// Strategy 2 (fallback): Re-exec — start a new process with the same args,
// then exit the current process. The new process runs in a new session.
func (u *SelfUpdater) restart() error {
	if u.isSystemdManaged() {
		log.Println("[SelfUpdater] restarting via systemctl")
		cmd := exec.Command("systemctl", "restart", "corefusion-agent")
		if err := cmd.Run(); err != nil {
			log.Printf("[SelfUpdater] systemctl restart failed: %v, falling back to re-exec", err)
		} else {
			// systemctl restart kills the current process; if we're still alive,
			// it means systemd sent the signal and we should exit.
			// Give systemd a moment to handle the restart.
			time.Sleep(3 * time.Second)
			os.Exit(0)
		}
	}

	// Fallback: re-exec
	log.Println("[SelfUpdater] restarting via re-exec")
	selfPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}
	selfPath, err = filepath.EvalSymlinks(selfPath)
	if err != nil {
		return fmt.Errorf("resolve symlink: %w", err)
	}

	cmd := exec.Command(selfPath, os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("re-exec failed: %w", err)
	}

	log.Printf("[SelfUpdater] new process started (pid=%d), exiting current process", cmd.Process.Pid)
	os.Exit(0)
	return nil
}

// isSystemdManaged checks if the Agent is running under systemd.
func (u *SelfUpdater) isSystemdManaged() bool {
	// Check 1: systemd service file exists
	if _, err := os.Stat("/etc/systemd/system/corefusion-agent.service"); err == nil {
		return true
	}
	// Check 2: INVOCATION_ID environment variable (set by systemd)
	if os.Getenv("INVOCATION_ID") != "" {
		return true
	}
	return false
}

// CheckCrashRecovery checks if the Agent crashed after a recent update and
// rolls back to the previous version if so.
//
// Detection logic:
//   - If a .bak file exists and its modification time is less than 30 seconds ago,
//     it likely means the new binary crashed immediately after startup.
//     In this case, roll back to the .bak binary and restart.
//   - If a .bak file exists but is older than 30 seconds, the update was successful.
//     Delete the .bak file.
func (u *SelfUpdater) CheckCrashRecovery() {
	selfPath, err := os.Executable()
	if err != nil {
		log.Printf("[SelfUpdater] crash recovery: cannot get executable path: %v", err)
		return
	}
	selfPath, err = filepath.EvalSymlinks(selfPath)
	if err != nil {
		log.Printf("[SelfUpdater] crash recovery: cannot resolve symlink: %v", err)
		return
	}

	bakPath := selfPath + ".bak"

	info, err := os.Stat(bakPath)
	if err != nil {
		// No .bak file — nothing to recover
		return
	}

	// Check if the backup is recent (within 30 seconds)
	if time.Since(info.ModTime()) < 30*time.Second {
		// Likely crashed after update — roll back
		log.Println("[SelfUpdater] crash recovery detected, rolling back to previous version")

		// Replace current binary with backup
		if err := os.Rename(bakPath, selfPath); err != nil {
			log.Printf("[SelfUpdater] crash recovery: failed to restore backup: %v", err)
			return
		}
		os.Chmod(selfPath, 0755)

		// Restart with the rolled-back binary
		cmd := exec.Command(selfPath, os.Args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		if err := cmd.Start(); err != nil {
			log.Printf("[SelfUpdater] crash recovery: failed to restart: %v", err)
			return
		}
		log.Printf("[SelfUpdater] rolled back and restarted (pid=%d)", cmd.Process.Pid)
		os.Exit(1)
	}

	// Update was successful (survived > 30s) — clean up backup
	log.Println("[SelfUpdater] previous .bak backup is old, removing (update was successful)")
	os.Remove(bakPath)
}

// getGithubAccelDomain reads the GitHub acceleration domain from the Agent's
// local settings database. Returns "" if not configured or disabled.
func (u *SelfUpdater) getGithubAccelDomain() string {
	if u.settingRepo == nil {
		return ""
	}
	val, err := u.settingRepo.GetByKey(context.Background(), "github.acceleration")
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

// getGithubProxyURL reads the GitHub HTTP/SOCKS proxy URL from the Agent's
// local settings database. Returns "" if not configured.
func (u *SelfUpdater) getGithubProxyURL() string {
	if u.settingRepo == nil {
		return ""
	}
	val, err := u.settingRepo.GetByKey(context.Background(), "github.acceleration")
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
