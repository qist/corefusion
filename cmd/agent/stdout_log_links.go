package main

import (
	"log"
	"os"
	"path/filepath"
)

// legacyStdoutLogLinks 是标准输出模式遗留的日志软链名。
// 旧版 Agent 在 SyncProxyConfig 时会创建 -> /dev/stdout | /dev/stderr 的软链，
// 让 nginx 的 access_log/error_log 写到进程 stdout。标准输出模式已下线
// （日志模式只剩 file / off，一律写真实文件），这些软链不再需要。
var legacyStdoutLogLinks = []string{
	"stdout_access.log",
	"stdout_error.log",
	"stdout_stream.log",
}

// cleanupLegacyStdoutLogLinks 删除遗留的 stdout 日志软链（只删软链）。
//
// 如果同名路径是普通文件（说明有人在写真实日志），保持不动。
// 删除后若仍有旧配置引用该路径，nginx 会自己创建普通文件，日志反而能被面板 tail 到。
func cleanupLegacyStdoutLogLinks(logsDir string) {
	for _, name := range legacyStdoutLogLinks {
		p := filepath.Join(logsDir, name)
		fi, err := os.Lstat(p)
		if err != nil || fi.Mode()&os.ModeSymlink == 0 {
			continue
		}
		if err := os.Remove(p); err != nil {
			log.Printf("[Agent] remove legacy stdout log link %s: %v", p, err)
			continue
		}
		log.Printf("[Agent] removed legacy stdout log link %s", p)
	}
}
