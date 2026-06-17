package binaries

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// BinaryInfo contains information about an installed binary.
type BinaryInfo struct {
	CoreType   string `json:"core_type"`
	Version    string `json:"version"`
	BinaryPath string `json:"binary_path"`
	Installed  bool   `json:"installed"`
}

// Resolver resolves binary paths and versions for core types.
type Resolver struct {
	dataDir string
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
	return &Resolver{dataDir: absDir}
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

// ListInstalled returns all installed core binaries with their versions.
func (r *Resolver) ListInstalled() []BinaryInfo {
	coreTypes := []string{"xray", "singbox", "hysteria2", "tuic"}
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
	// Try common version flags
	for _, flag := range []string{"version", "-v", "--version", "-version"} {
		cmd := exec.Command(binaryPath, flag)
		out, err := cmd.Output()
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
