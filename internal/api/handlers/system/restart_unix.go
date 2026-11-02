//go:build !windows

package system

import (
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// restartProcess restarts the current process (Unix).
//
// systemd 托管（cgroup 能解析出 .service 段）时：直接以**非零码退出**，交给
// 单元的 Restart=on-failure（install.sh 单元模板均为此配置，RestartSec=3）
// 用新二进制拉起。这是 systemd 的原生重启通道。
//
// ⚠ 不再用 syscall.Exec 原地替换：容器环境实测（2026-11-02，116Rocky9 容器）
// Exec 后新镜像会立刻退出且 journal 无任何痕迹（"面板 restart 有时不换进程"
// 的老问题同源），表现为更新后单元 inactive、面板失联。
//
// 非 systemd（nohup / 容器无 systemd）：re-exec 自身后退出（父进程退出前
// 稍作等待，让子进程先完成 exec，避免被 cgroup 清理连带杀掉）。
//
// 参数按数组传递，不经 shell，无注入面（安全审计 V4-S5 / v5 §3）。
func restartProcess() {
	unit := systemdUnitName()
	if unit != "" {
		log.Printf("[system] restart: exiting(1); systemd will restart %s", unit)
		os.Exit(1)
	}
	executable, err := os.Executable()
	if err != nil {
		os.Exit(0)
	}
	argv := append([]string{executable}, os.Args[1:]...)
	if err := syscall.Exec(executable, argv, os.Environ()); err != nil {
		// exec 失败的兜底：退回 spawn，至少尝试拉起新进程。
		cmd := exec.Command(executable, os.Args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Start()
		os.Exit(0)
	}
}

// systemdUnitName 从 cgroup 解析当前进程所属的 systemd 单元名（如 corefusion.service）。
// docker 容器（面板进程不受 systemd 管理时）cgroup 段以容器 id 或 .scope 结尾，
// 不会误匹配；找不到 .service 段则返回空。
func systemdUnitName() string {
	data, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		idx := strings.LastIndex(line, "/")
		if idx < 0 {
			continue
		}
		name := strings.TrimSpace(line[idx+1:])
		if strings.HasSuffix(name, ".service") {
			return name
		}
	}
	return ""
}
