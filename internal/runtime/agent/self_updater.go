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

	// Step 1: Download
	tmpPath, err := u.downloadFile(downloadURL)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	log.Printf("[SelfUpdater] downloaded to %s", tmpPath)

	// Step 2: Verify checksum
	if checksum != "" {
		if err := u.verifyChecksum(tmpPath, checksum); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("checksum verification failed: %w", err)
		}
		log.Printf("[SelfUpdater] checksum verified")
	} else {
		log.Printf("[SelfUpdater] skipping checksum verification (no checksum provided)")
	}

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
	tmpPath := filepath.Join(os.TempDir(), "corefusion-agent-new")

	// Choose HTTP client based on URL source
	var client *http.Client
	if netutil.IsGitHubURL(downloadURL) {
		// Read GitHub acceleration settings from local DB
		accelDomain := u.getGithubAccelDomain()
		proxyURL := u.getGithubProxyURL()
		client = netutil.GitHubHTTPClientWithProxy(10*time.Minute, accelDomain, proxyURL)
		// Apply acceleration domain rewrite
		downloadURL = netutil.AccelerateGitHubURL(downloadURL, accelDomain)
		log.Printf("[SelfUpdater] using GitHub client (accel=%s, proxy=%s)", accelDomain, proxyURL)
	} else {
		client = netutil.NewHTTPClient(10 * time.Minute)
	}

	resp, err := client.Get(downloadURL)
	if err != nil {
		return "", fmt.Errorf("HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	// Write to temp file
	out, err := os.Create(tmpPath)
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}

	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		os.Remove(tmpPath)
		return "", fmt.Errorf("write temp file: %w", err)
	}
	out.Close()

	return tmpPath, nil
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
// - If a .bak file exists and its modification time is less than 30 seconds ago,
//   it likely means the new binary crashed immediately after startup.
//   In this case, roll back to the .bak binary and restart.
// - If a .bak file exists but is older than 30 seconds, the update was successful.
//   Delete the .bak file.
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
