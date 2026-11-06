package proxy

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// EnsureRuntimePermissions 修正托管代理（nginx/caddy）运行期需要的目录与文件权限。
//
// 背景（真机踩过）：数据目录由面板以 root 创建为 0750，而 nginx worker 以 nginx
// 用户运行 —— 于是：
//   - WAF 的 access.lua 读不到 → 每条请求报 "failed to load external Lua file"，
//     WAF 静默失效；
//   - WAF 规则目录/日志目录同样不可读不可写；
//   - 迁移进来的 conf.d 自定义配置也读不到。
//
// 这类问题不该让部署者手工 chmod（升级/换机就会复现），所以面板启动和每次启动
// 代理前都自动校正一次。只放开"遍历/读取"权限，不碰数据库等敏感文件的内容权限。
func EnsureRuntimePermissions(dataDir string) error {
	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		return nil
	}
	var firstErr error
	record := func(err error) {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	// 1) 数据根目录：nginx 只需要 "可遍历"（o+x），不需要列出内容。
	if err := addModeBits(dataDir, 0, 0o001); err != nil {
		record(err)
	}

	proxyDir := filepath.Join(dataDir, "proxy")
	if err := os.MkdirAll(proxyDir, 0o755); err != nil {
		return fmt.Errorf("create proxy dir: %w", err)
	}
	// 已有安装可能是 0750（历史 MkdirAll 参数），显式补 g+x/o+x，否则 nginx 连 proxy 都进不去。
	record(addModeBits(proxyDir, 0, 0o005))
	// 2) 代理运行时目录（lua / waf-rules / conf.d / mime.types 等）：目录可进入、文件可读。
	for _, sub := range []string{proxyDir, filepath.Join(proxyDir, "conf.d")} {
		if err := os.MkdirAll(sub, 0o755); err != nil {
			record(err)
			continue
		}
	}
	for _, root := range []string{
		filepath.Join(proxyDir, "lua"),
		filepath.Join(proxyDir, "waf-rules"),
		filepath.Join(proxyDir, "conf.d"),
	} {
		record(walkAddReadPerms(root))
	}

	// 3) 日志目录：WAF 在 worker（nginx 用户）里写日志，必须可创建/追加。
	logsDir := filepath.Join(dataDir, "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		record(err)
	} else if uid, gid, err := lookupNginxUser(); err == nil {
		if err := os.Chown(logsDir, uid, gid); err != nil {
			record(err)
		}
		record(os.Chmod(logsDir, 0o775))
		// 已存在的 WAF 日志文件可能是 root 建的，一并交给 nginx 追加。
		if entries, err := os.ReadDir(logsDir); err == nil {
			for _, e := range entries {
				if e.IsDir() || !strings.HasSuffix(e.Name(), "_waf.log") {
					continue
				}
				p := filepath.Join(logsDir, e.Name())
				record(os.Chown(p, uid, gid))
				record(addModeBits(p, 0o220, 0))
			}
		}
	} else {
		// 没有 nginx 用户（例如未安装代理）：保持目录对同组可写即可。
		record(os.Chmod(logsDir, 0o775))
	}

	if firstErr != nil {
		log.Printf("[proxy] ensure runtime permissions on %s: %v", dataDir, firstErr)
	}
	return firstErr
}

// walkAddReadPerms 给目录补 o+x（可进入）、给文件补 o+r（可读）。
// 只加位、不减位，避免影响管理员已有的更严格或更宽松设置。
func walkAddReadPerms(root string) error {
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return addModeBits(path, 0, 0o005)
		}
		return addModeBits(path, 0o444, 0)
	})
}

// addModeBits 给文件/目录补权限位；mode 为当前权限（传 0 表示自行读取）。
func addModeBits(path string, addFile, addOther uint32) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	mode := info.Mode().Perm()
	add := os.FileMode(0)
	if info.IsDir() {
		add |= os.FileMode(addOther)
	} else {
		add |= os.FileMode(addFile)
	}
	// 目录同时要保留"可进入"（o+x 与 g+x 由 addOther 控制），文件补读位。
	if add == 0 || mode&add == add {
		return nil
	}
	return os.Chmod(path, mode|add)
}
