//go:build windows

package process

// cleanupStaleUnixSockets 在 Windows 上无 unix socket 残留问题，空实现。
func cleanupStaleUnixSockets(string) int { return 0 }

// ensureUnixSocketPerms 在 Windows 上没有文件型 unix socket，空实现。
func ensureUnixSocketPerms(string) {}
