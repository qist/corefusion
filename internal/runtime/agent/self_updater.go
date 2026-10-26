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

	"corefusion/internal/constants"
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
	// 替换前先落"更新中"标记：下次启动据此按**版本号**判定成功/回滚，
	// 而不是靠 .bak 的 mtime 猜（那套启发式会在连续更新时误判，把新二进制回滚掉）。
	if err := u.writeUpdateMarker(version); err != nil {
		return fmt.Errorf("write update marker: %w", err)
	}
	if err := u.replaceBinary(tmpPath); err != nil {
		u.clearUpdateMarker()
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
	selfPath, err := SelfBinaryPath()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
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
	// 必须用"安装路径"而不是 os.Executable()：replaceBinary 刚把旧二进制 rename 成
	// .bak，/proc/self/exe 会跟着指向 .bak —— 直接拿它 re-exec 就是把**旧版本**再跑一遍
	// （真机：节点 18 每次更新后版本回退，还堆出 .bak.bak.bak）。
	selfPath, err := SelfBinaryPath()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
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

// CheckCrashRecovery 判定"上一次自更新到底成不成功"，并在新二进制没起来时回滚。
//
// 判定依据（重要）：
//  1. 有 .update-pending 标记 → 按**版本号**判定。运行中的版本 == 标记里的目标版本
//     → 更新成功，清理标记与 .bak；否则回滚。
//     ⚠ 不能再用 ".bak 的 mtime 小于 30 秒" 这套启发式：rename 不改变 mtime，
//     而连续两次更新时 .bak 也可能是刚下载的新文件 → 会把刚装好的二进制回滚回去，
//     节点版本永远上不去（真机：节点 18 在 alpha.3/alpha.4 之间来回回滚）。
//  2. 没有标记（老版本 Agent 更新的，或人工替换）→ 保留原来的 mtime 启发式。
func (u *SelfUpdater) CheckCrashRecovery() {
	selfPath, err := SelfBinaryPath()
	if err != nil {
		log.Printf("[SelfUpdater] crash recovery: cannot get executable path: %v", err)
		return
	}

	bakPath := selfPath + ".bak"
	marker, markerFound := readUpdateMarker(selfPath)

	switch decideRecovery(marker, markerFound, constants.Version, fileExists(bakPath)) {
	case recoveryCleanup:
		log.Printf("[SelfUpdater] update to %s confirmed, cleaning up marker and backup", marker.ToVersion)
		u.clearUpdateMarker()
		os.Remove(bakPath)
		return
	case recoveryRollback:
		log.Printf("[SelfUpdater] running version %q does not match pending target %q, rolling back",
			constants.Version, marker.ToVersion)
		u.rollbackTo(bakPath, selfPath)
		return
	}

	// 没有标记：沿用旧的 mtime 判据
	info, err := os.Stat(bakPath)
	if err != nil {
		return // No .bak file — nothing to recover
	}
	if time.Since(info.ModTime()) >= 30*time.Second {
		log.Println("[SelfUpdater] previous .bak backup is old, removing (update was successful)")
		os.Remove(bakPath)
		return
	}
	log.Println("[SelfUpdater] crash recovery detected, rolling back to previous version")
	u.rollbackTo(bakPath, selfPath)
}

// rollbackTo 用 .bak 覆盖当前二进制并重启；成功后本进程退出（不返回）。
func (u *SelfUpdater) rollbackTo(bakPath, selfPath string) {
	if err := os.Rename(bakPath, selfPath); err != nil {
		log.Printf("[SelfUpdater] crash recovery: failed to restore backup: %v", err)
		u.clearUpdateMarker()
		return
	}
	os.Chmod(selfPath, 0755)
	u.clearUpdateMarker()

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

// SelfBinaryPath 返回"安装路径"上的可执行文件路径（解析符号链接后去掉 .bak 后缀）。
//
// ⚠ 不能直接用 os.Executable()：自更新会把旧二进制 rename 成 <path>.bak，
// 而 /proc/self/exe 会跟着改名一起走。于是：
//   - replaceBinary 把新二进制写到 .bak 上（下次再 rename 成 .bak.bak，无限套娃）；
//   - re-exec 重新拉起的是 .bak 里的**旧版本**（节点永远升不上去）。
//
// 真实事故：节点 18（容器内无 systemd，走 re-exec）在 alpha.3/alpha.4 之间反复回滚。
//
// 因此这里统一归一到安装路径：带 .bak 后缀时退回去掉后缀的路径（该路径存在时）。
func SelfBinaryPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, rerr := filepath.EvalSymlinks(exe); rerr == nil {
		exe = resolved
	}
	if base, stripped := stripBackupSuffix(exe); stripped && fileExists(base) {
		return base, nil
	}
	return exe, nil
}

// stripBackupSuffix 去掉路径末尾连续的 ".bak"（.bak / .bak.bak / .bak.bak.bak…）。
func stripBackupSuffix(path string) (string, bool) {
	base := path
	for strings.HasSuffix(base, ".bak") {
		base = strings.TrimSuffix(base, ".bak")
	}
	return base, base != path
}

// updateMarkerPath 是"自更新进行中"的标记文件（与二进制同目录）。
func updateMarkerPath(selfPath string) string { return selfPath + ".update-pending" }

// updateMarker 记录一次自更新的来源/目标版本。
type updateMarker struct {
	FromVersion string    `json:"from_version"`
	ToVersion   string    `json:"to_version"`
	At          time.Time `json:"at"`
}

// writeUpdateMarker 在替换二进制之前写下目标版本，供下次启动校验。
func (u *SelfUpdater) writeUpdateMarker(version string) error {
	selfPath, err := SelfBinaryPath()
	if err != nil {
		return err
	}
	marker := updateMarker{FromVersion: constants.Version, ToVersion: strings.TrimSpace(version), At: time.Now()}
	data, err := json.Marshal(marker)
	if err != nil {
		return err
	}
	return os.WriteFile(updateMarkerPath(selfPath), data, 0600)
}

// clearUpdateMarker 删除标记文件（不存在时静默）。
func (u *SelfUpdater) clearUpdateMarker() {
	selfPath, err := SelfBinaryPath()
	if err != nil {
		return
	}
	if err := os.Remove(updateMarkerPath(selfPath)); err != nil && !os.IsNotExist(err) {
		log.Printf("[SelfUpdater] failed to remove update marker: %v", err)
	}
}

// readUpdateMarker 读取标记文件；不存在或损坏时返回 (nil,false)。
func readUpdateMarker(selfPath string) (*updateMarker, bool) {
	data, err := os.ReadFile(updateMarkerPath(selfPath))
	if err != nil {
		return nil, false
	}
	var marker updateMarker
	if err := json.Unmarshal(data, &marker); err != nil {
		return nil, false
	}
	return &marker, true
}

// recoveryAction 是启动时的自更新收尾动作。
type recoveryAction int

const (
	recoveryNone     recoveryAction = iota // 与本次启动无关，交给 mtime 兜底逻辑
	recoveryCleanup                        // 目标版本已在运行 → 清理
	recoveryRollback                       // 目标版本没起来 → 回滚
)

// decideRecovery 是 CheckCrashRecovery 的纯决策逻辑（便于测试）：
func decideRecovery(marker *updateMarker, markerFound bool, runningVersion string, bakExists bool) recoveryAction {
	if !markerFound || marker == nil {
		return recoveryNone
	}
	target := strings.TrimSpace(marker.ToVersion)
	if target != "" && target == strings.TrimSpace(runningVersion) {
		return recoveryCleanup
	}
	if !bakExists {
		// 没有备份可回滚：只能认账，清掉标记避免反复尝试
		return recoveryCleanup
	}
	return recoveryRollback
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
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
