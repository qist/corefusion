//go:build !windows

package process

import (
	"encoding/json"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// cleanupStaleUnixSockets 在启动核心前清理配置里声明、但已经没有进程持有的 unix socket。
//
// 背景：xray 正常退出时 *net.UnixListener 会自动删除 socket 文件；但进程被 SIGKILL、
// OOM 或崩溃时文件会残留。下次启动 bind 同一个路径会直接 EADDRINUSE，核心起不来，
// 表现为「面板显示启动失败 / 节点永远离线」，而配置本身完全正确。
//
// 判定「可安全删除」需要同时满足：
//  1. dial 该 socket 失败（没有进程在 listen）；
//  2. 能拿到 xray 自己的 <path>.lock 文件锁（xray 用 flock 做单实例互斥，
//     进程死亡后内核自动释放，所以能拿到锁就说明没有存活实例）。
//
// 只处理配置里 inbounds[].listen 明确声明、且是文件路径（"/..."）的项；
// abstract socket（"@..."）不落盘，忽略。
func cleanupStaleUnixSockets(configPath string) int {
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return 0
	}
	var cfg struct {
		Inbounds []struct {
			Listen string `json:"listen"`
		} `json:"inbounds"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return 0
	}

	removed := 0
	seen := make(map[string]bool)
	for _, ib := range cfg.Inbounds {
		path := unixSocketPath(ib.Listen)
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		if !isStaleUnixSocket(path) {
			continue
		}
		if err := os.Remove(path); err != nil {
			if !os.IsNotExist(err) {
				log.Printf("process: failed to remove stale unix socket %s: %v", path, err)
			}
			continue
		}
		// xray 的 lock 文件与 socket 同生命周期；socket 能删说明实例已死，lock 也一并清掉。
		if err := os.Remove(path + ".lock"); err == nil {
			log.Printf("process: removed stale unix socket %s (and lock)", path)
		} else {
			log.Printf("process: removed stale unix socket %s", path)
		}
		removed++
	}
	return removed
}

// unixSocketPath 从 xray 的 listen 值里提取文件路径。
// 支持 "/dev/shm/x.sock"、"/dev/shm/x.sock,0666"、"unix:/dev/shm/x.sock"。
// 返回空表示不是文件型 unix socket。
func unixSocketPath(listen string) string {
	s := strings.TrimSpace(listen)
	if s == "" {
		return ""
	}
	s = strings.TrimPrefix(s, "unix:")
	if i := strings.Index(s, ","); i >= 0 {
		s = s[:i]
	}
	if !strings.HasPrefix(s, "/") {
		return ""
	}
	return s
}

func isStaleUnixSocket(path string) bool {
	fi, err := os.Stat(path)
	if err != nil || fi.Mode()&os.ModeSocket == 0 {
		return false
	}

	// 有进程在 listen 时 dial 会成功，绝不能删。
	if c, err := net.DialTimeout("unix", path, 200*time.Millisecond); err == nil {
		_ = c.Close()
		return false
	}

	// xray 用 <path>.lock + flock 保证单实例；拿不到锁说明还有存活进程。
	lockPath := path + ".lock"
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return false
	}
	defer f.Close()
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		return false
	}
	_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
	return true
}

// ensureUnixSocketPerms 让非 root 的代理进程（nginx worker）能连上核心创建的 unix socket。
//
// 背景（生产实测）：3x-ui 的 xray 用 listen "/dev/shm/x.sock,0666" 显式放宽 socket 权限，
// 迁移到 CoreFusion 后 listen 只剩路径（vanilla xray 不认 ,mode 后缀），xray 按 umask
// 建成 0755 → nginx worker（nginx 用户）connect() 报 EACCES，客户端看到的是
// "An existing connection was forcibly closed by the remote host"，
// 而服务端日志里是 "connect() to unix:... failed (13: Permission denied)"。
//
// 核心启动是异步的（socket 在 cmd.Start 之后才创建），所以轮询等待出现后统一放开权限。
// 只处理配置里 inbounds[].listen 明确声明的文件型 socket。
func ensureUnixSocketPerms(configPath string) {
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return
	}
	var cfg struct {
		Inbounds []struct {
			Listen string `json:"listen"`
		} `json:"inbounds"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return
	}
	paths := make([]string, 0, len(cfg.Inbounds))
	for _, ib := range cfg.Inbounds {
		if p := unixSocketPath(ib.Listen); p != "" {
			paths = append(paths, p)
		}
	}
	if len(paths) == 0 {
		return
	}

	const wantMode = os.FileMode(0o666)
	deadline := time.Now().Add(10 * time.Second)
	for {
		pending := 0
		for _, p := range paths {
			fi, statErr := os.Stat(p)
			if statErr != nil || fi.Mode()&os.ModeSocket == 0 {
				pending++
				continue
			}
			if fi.Mode().Perm()&wantMode == wantMode {
				continue
			}
			// 与 3x-ui 的 listen "...,0666" 保持一致：socket 只需要读写位，不要 exec 位。
			if err := os.Chmod(p, wantMode); err != nil {
				log.Printf("process: chmod unix socket %s: %v", p, err)
			}
		}
		if pending == 0 {
			return
		}
		if time.Now().After(deadline) {
			log.Printf("process: %d unix socket(s) not ready after 10s, permissions not fixed: %v", pending, paths)
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}
