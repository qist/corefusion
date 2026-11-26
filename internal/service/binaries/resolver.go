package binaries

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// BinaryInfo contains information about an installed binary.
type BinaryInfo struct {
	CoreType   string `json:"core_type"`
	Version    string `json:"version"`
	BinaryPath string `json:"binary_path"`
	Installed  bool   `json:"installed"`
}

// cachedVersion 按二进制文件的修改时间/大小缓存版本号，
// 避免每次生成配置都 exec 一次核心。
type cachedVersion struct {
	modTime time.Time
	size    int64
	version string
}

// Resolver resolves binary paths and versions for core types.
type Resolver struct {
	dataDir string

	mu          sync.Mutex
	versionMemo map[string]cachedVersion
}

// NewResolver creates a new binary resolver.
func NewResolver(dataDir string) *Resolver {
	// Convert to absolute path so it works from any directory
	absDir := dataDir
	if !filepath.IsAbs(dataDir) {
		if abs, err := filepath.Abs(dataDir); err == nil {
			absDir = abs
		}
	}
	return &Resolver{dataDir: absDir, versionMemo: make(map[string]cachedVersion)}
}

// Resolve returns the binary path and version for a given core type.
func (r *Resolver) Resolve(coreType string) (binaryPath, version string, err error) {
	binaryPath = r.binaryPath(coreType)

	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		return "", "", fmt.Errorf("binary not installed for core type %s", coreType)
	}

	version = r.detectVersion(binaryPath)
	return binaryPath, version, nil
}

// Version 返回已安装核心的版本（未安装或探测失败返回空串）。
// 二进制文件未变化时复用缓存，避免频繁 exec。
func (r *Resolver) Version(coreType string) string {
	path := r.binaryPath(coreType)
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	r.mu.Lock()
	if c, ok := r.versionMemo[coreType]; ok && c.modTime.Equal(info.ModTime()) && c.size == info.Size() {
		r.mu.Unlock()
		return c.version
	}
	r.mu.Unlock()

	version := r.detectVersion(path)
	if version == "unknown" {
		version = ""
	}
	r.mu.Lock()
	r.versionMemo[coreType] = cachedVersion{modTime: info.ModTime(), size: info.Size(), version: version}
	r.mu.Unlock()
	return version
}

// ListInstalled returns all installed core binaries with their versions.
// hysteria2/tuic 独立核心已下线（协议由 sing-box 承载），不再出现在安装列表。
// mihomo 只能手工放进 binaries/active/mihomo，但探测/链路拨号支持它：
// 这里必须一并枚举，否则节点装了 mihomo 也会被当成"没有可用核心"。
func (r *Resolver) ListInstalled() []BinaryInfo {
	coreTypes := []string{"xray", "singbox", "mihomo"}
	var result []BinaryInfo

	for _, ct := range coreTypes {
		info := BinaryInfo{
			CoreType:   ct,
			BinaryPath: r.binaryPath(ct),
		}

		if _, err := os.Stat(info.BinaryPath); err == nil {
			info.Installed = true
			info.Version = r.detectVersion(info.BinaryPath)
		}

		result = append(result, info)
	}
	return result
}

// binaryPath returns the expected binary path for a core type.
func (r *Resolver) binaryPath(coreType string) string {
	name := coreType + "-" + runtime.GOOS + "-" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(r.dataDir, "binaries", "active", coreType, name)
}

// detectVersion tries to get the version from a binary by running it with version flags.
func (r *Resolver) detectVersion(binaryPath string) string {
	flags := []string{"version", "-v", "--version", "-version"}
	// mihomo 不认识 `version` 子命令：它会把参数当成配置来源直接**启动代理**，
	// cmd.Output() 就会一直等进程退出（实测卡死 10 分钟）。mihomo 只认 `-v`。
	if strings.HasPrefix(strings.ToLower(filepath.Base(binaryPath)), "mihomo") {
		flags = []string{"-v"}
	}
	for _, flag := range flags {
		// 通用兜底：任何核心都可能把未知参数当成"启动"，必须限时，不能阻塞调用方。
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		out, err := exec.CommandContext(ctx, binaryPath, flag).Output()
		timedOut := ctx.Err() == context.DeadlineExceeded
		cancel()
		if timedOut {
			continue
		}
		if err == nil && len(out) > 0 {
			// Return first line only
			version := string(out)
			for i, c := range version {
				if c == '\n' || c == '\r' {
					version = version[:i]
					break
				}
			}
			return version
		}
	}
	return "unknown"
}
