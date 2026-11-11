// Package systemdunit provides best-effort systemd unit compatibility tweaks.
//
// 背景：面板/Agent 重启或升级时，systemd 默认 KillMode=control-group 会把整个
// cgroup 里的进程一起杀掉——包括核心（xray/sing-box）与代理（caddy/nginx）
// 这些数据面子进程，导致每次升级都断流。
//
// 新安装脚本生成的 unit 已带 KillMode=process；老版本安装的 unit 没有，
// 这里在重启前补一个 drop-in 做兼容（失败不阻断，最坏退回旧行为）。
package systemdunit

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// EnsureKillModeProcess 为 systemd 单元写入 `KillMode=process` drop-in。
// unit 可传 "corefusion" 或 "corefusion.service"。
func EnsureKillModeProcess(unit string) {
	unit = strings.TrimSpace(unit)
	if unit == "" {
		return
	}
	if !strings.HasSuffix(unit, ".service") {
		unit += ".service"
	}
	dir := filepath.Join("/etc/systemd/system", unit+".d")
	path := filepath.Join(dir, "10-killmode.conf")
	const content = "[Service]\n# 只杀主进程：核心/代理子进程在管理面重启期间保持运行\nKillMode=process\n"

	if cur, err := os.ReadFile(path); err == nil && strings.Contains(string(cur), "KillMode=process") {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("[systemdunit] mkdir %s failed: %v", dir, err)
		return
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		log.Printf("[systemdunit] write %s failed: %v", path, err)
		return
	}
	if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
		log.Printf("[systemdunit] daemon-reload failed: %v", err)
		return
	}
	log.Printf("[systemdunit] ensured KillMode=process for %s", unit)
}
