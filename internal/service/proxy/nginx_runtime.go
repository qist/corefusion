package proxy

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// detectRunningNginx 探测**正在运行**的 nginx master 进程，返回它的
// 可执行文件路径、配置文件路径（`-c` 指定）与工作目录。
//
// ⚠ 为什么必须探测而不是写死 ✗：面板托管的 nginx 配置文件在 dataDir/proxy/nginx.conf ✓，
// 但节点上完全可能是**外部 nginx**（运维/系统自带，例如
// `/apps/nginx/sbin/nginx -c /etc/nginx/nginx.conf` ✓）。
// 这时若仍拿面板托管的 proxy/nginx.conf 去测/重载：
//   - 该文件在外部部署里根本不存在 → `nginx: [emerg] open() ".../proxy/nginx.conf" failed` ✗，
//     control_proxy 任务直接失败 ✓（真机踩到过：/apps 与 /etc/nginx 之争 ✓）；
//   - 即便碰巧存在，它也不是线上生效的那份配置 ✗，重载毫无意义 ✓。
//
// 实现：nginx master 会把进程标题改写成
//
//	`<progname>: master process <argv0> -c <conf>`
//
// 所以直接解析 /proc/<pid>/cmdline ✓；相对路径按 /proc/<pid>/cwd 解析 ✓。
// 找不到（nginx 没跑、非 Linux）时返回空串，由调用方按托管路径兜底 ✓。
func detectRunningNginx() (binPath, confPath, cwd string) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return "", "", ""
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid := e.Name()
		if _, err := strconv.Atoi(pid); err != nil {
			continue // 只处理纯数字 PID 目录 ✓
		}
		raw, err := os.ReadFile(filepath.Join("/proc", pid, "cmdline"))
		if err != nil || len(raw) == 0 {
			continue
		}
		cmdline := strings.ReplaceAll(strings.TrimRight(string(raw), "\x00"), "\x00", " ")
		procCwd := ""
		if link, err := os.Readlink(filepath.Join("/proc", pid, "cwd")); err == nil {
			procCwd = link
		}
		if bin, conf, ok := parseNginxMasterCmdline(cmdline, procCwd); ok {
			return bin, conf, procCwd
		}
	}
	return "", "", ""
}

// parseNginxMasterCmdline 从单条进程 cmdline 里解析 nginx master 的
// 二进制路径与 `-c` 指定的配置文件路径（相对路径按 cwd 解析 ✓）。
// 不是 nginx master 时返回 ok=false ✓。
func parseNginxMasterCmdline(cmdline, cwd string) (binPath, confPath string, ok bool) {
	if !strings.Contains(cmdline, "master process") {
		return "", "", false // worker 及其它进程不含该标记 ✓
	}
	fields := strings.Fields(cmdline)
	// 标题形如 "nginx: master process <argv0> ..."，"process" 后一个 token 才是原始 argv[0] ✓
	argStart := -1
	for i, f := range fields {
		if f == "process" {
			argStart = i + 1
			break
		}
	}
	if argStart < 0 || argStart >= len(fields) {
		return "", "", false
	}
	binTok := fields[argStart]
	// 二进制名必须含 nginx（避免同名巧合；托管二进制名为 nginx-linux-amd64 ✓）
	if !strings.Contains(strings.ToLower(filepath.Base(binTok)), "nginx") {
		return "", "", false
	}
	binPath = resolveProcPath(binTok, cwd)
	for i := argStart; i < len(fields)-1; i++ {
		if fields[i] == "-c" {
			confPath = resolveProcPath(fields[i+1], cwd)
			break
		}
	}
	return binPath, confPath, true
}

// resolveProcPath 把进程启动参数里的路径解析成绝对路径（相对路径按进程 cwd 解析 ✓）。
func resolveProcPath(p, cwd string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if filepath.IsAbs(p) {
		return p
	}
	if cwd == "" {
		return p
	}
	return filepath.Join(cwd, p)
}

// nginxCompiledConfPath 解析 `nginx -V` 输出里的编译期默认配置路径 `--conf-path=` ✓。
//
// ⚠ nginx 把 `-V` 的信息写到 **stderr** ✓，故用 CombinedOutput ✓。
// 用于运行中 nginx 未显式带 `-c`（用编译默认配置启动）时的兜底 ✓。
func nginxCompiledConfPath(binPath string) string {
	if strings.TrimSpace(binPath) == "" {
		return ""
	}
	out, _ := exec.Command(binPath, "-V").CombinedOutput()
	if len(out) == 0 {
		return ""
	}
	for _, f := range strings.Fields(string(out)) {
		if strings.HasPrefix(f, "--conf-path=") {
			return strings.TrimSpace(strings.TrimPrefix(f, "--conf-path="))
		}
	}
	return ""
}
