package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	agentpb "corefusion/internal/runtime/agent/proto"
	"corefusion/internal/util/safepath"
)

// handleReadLogFile reads the last N lines from a log file on the agent node.
// In stdout mode, it first tries to read from the LogBroker's in-memory snapshot
// (which captures real-time stdout output). If the broker has no data, it falls
// back to reading the stdout log file (if managed=true and writer is active).
func (a *AgentApp) handleReadLogFile(req *agentpb.ReadLogFileRequest) *agentpb.ReadLogFileResponse {
	logType := req.LogType
	lastLines := int(req.LastLines)
	if lastLines <= 0 {
		lastLines = 500
	}

	// 增量读（tail -F）：offset > 0 表示从该字节偏移读取新增内容。
	// 只推进到最后一个完整行（\n），半行留待下次——面板侧无需再做差分。
	if req.Offset > 0 {
		return a.readLogIncremental(req, lastLines)
	}

	// Extract client-log-package filter parameters.
	// When filter_emails is set, only lines containing one of the emails are returned.
	// When start_time/end_time are set, lines outside the range are excluded.
	filterEmails := req.GetFilterEmails()
	hasFilter := len(filterEmails) > 0 || req.StartTime > 0 || req.EndTime > 0
	var filterStart, filterEnd time.Time
	if req.StartTime > 0 {
		filterStart = time.Unix(req.StartTime, 0)
	}
	if req.EndTime > 0 {
		filterEnd = time.Unix(req.EndTime, 0)
	}

	// If a specific log_path is provided, read that file directly (used by
	// rotation policies that point to a specific file like access.log).
	// We still check stdout mode via logType for the broker fallback.
	if req.LogPath != "" {
		logPath := a.resolveRelPath(req.LogPath)

		// In stdout mode, try LogBroker snapshot first (real-time data)
		mode := a.getLogMode(logType)
		if mode == "stdout" && a.logMgr != nil {
			lines, err := a.logMgr.GetProxyStdoutLog(context.Background(), logType, lastLines)
			if err == nil && len(lines) > 0 {
				realLines := []string{}
				for _, l := range lines {
					if !strings.Contains(l, "[未运行") && !strings.Contains(l, "未产生标准输出") {
						realLines = append(realLines, l)
					}
				}
				if len(realLines) > 0 {
					return &agentpb.ReadLogFileResponse{
						Success: true,
						Lines:   agentFilterLines(realLines, filterEmails, filterStart, filterEnd, hasFilter),
						LogPath: fmt.Sprintf("stdout://%s", logType),
					}
				}
			}
		}

		// Read the specified file directly
		f, err := os.Open(logPath)
		if err != nil {
			return &agentpb.ReadLogFileResponse{
				Success: false,
				Error:   fmt.Sprintf("open log file: %v", err),
				LogPath: logPath,
			}
		}
		defer f.Close()

		lines, err := readLastLinesFromFile(f, lastLines)
		if err != nil {
			return &agentpb.ReadLogFileResponse{
				Success: false,
				Error:   fmt.Sprintf("read log file: %v", err),
				LogPath: logPath,
			}
		}

		// tail 模式也回传当前文件大小 → 面板从锚点开始增量轮询
		nextOffset := int64(0)
		if fi, err := f.Stat(); err == nil {
			nextOffset = fi.Size()
		}
		return &agentpb.ReadLogFileResponse{
			Success:    true,
			Lines:      agentFilterLines(lines, filterEmails, filterStart, filterEnd, hasFilter),
			LogPath:    logPath,
			NextOffset: nextOffset,
		}
	}

	// No log_path specified — resolve by log type (original behavior)
	// In stdout mode, try LogBroker snapshot first (real-time data)
	mode := a.getLogMode(logType)
	if mode == "stdout" && a.logMgr != nil {
		lines, err := a.logMgr.GetProxyStdoutLog(context.Background(), logType, lastLines)
		if err == nil && len(lines) > 0 {
			// Filter out placeholder messages
			realLines := []string{}
			for _, l := range lines {
				if !strings.Contains(l, "[未运行") && !strings.Contains(l, "未产生标准输出") {
					realLines = append(realLines, l)
				}
			}
			if len(realLines) > 0 {
				return &agentpb.ReadLogFileResponse{
					Success: true,
					Lines:   agentFilterLines(realLines, filterEmails, filterStart, filterEnd, hasFilter),
					LogPath: fmt.Sprintf("stdout://%s", logType),
				}
			}
		}
	}

	// Fall back to file reading
	logPath := a.resolveLogFilePath(logType)
	if logPath == "" {
		return &agentpb.ReadLogFileResponse{
			Success: false,
			Error:   fmt.Sprintf("unknown log type: %s", logType),
		}
	}

	f, err := os.Open(logPath)
	if err != nil {
		// stdout capture 文件不存在（writer 未启用）时，回退读核心进程的 tee 文件
		//（data/logs/<type>.log — 核心进程 stderr/stdout 的落地文件）。
		altPath := filepath.Join(a.dataDir, "logs", logType+".log")
		if f2, err2 := os.Open(altPath); err2 == nil {
			f = f2
			logPath = altPath
		} else {
			return &agentpb.ReadLogFileResponse{
				Success: false,
				Error:   fmt.Sprintf("open log file: %v", err),
				LogPath: logPath,
			}
		}
	}
	defer f.Close()

	lines, err := readLastLinesFromFile(f, lastLines)
	if err != nil {
		return &agentpb.ReadLogFileResponse{
			Success: false,
			Error:   fmt.Sprintf("read log file: %v", err),
			LogPath: logPath,
		}
	}

	// tail 模式也回传当前文件大小 → 面板从锚点开始增量轮询
	nextOffset := int64(0)
	if fi, err := f.Stat(); err == nil {
		nextOffset = fi.Size()
	}
	return &agentpb.ReadLogFileResponse{
		Success:    true,
		Lines:      agentFilterLines(lines, filterEmails, filterStart, filterEnd, hasFilter),
		LogPath:    logPath,
		NextOffset: nextOffset,
	}
}

// handleRotateLog triggers log rotation for a specific log type on the agent node.
// req.LogPath (a rotation policy's concrete file path) takes precedence over the
// per-type default resolution.
//
// Rotation semantics must match the writer process:
//   - nginx: rename + recreate, then "nginx -s reopen" so nginx reopens its FDs
//   - everything else (caddy/xray/singbox): the writer holds an
//     O_APPEND handle and has NO reopen signal (Caddy v2 dropped SIGUSR1) — a
//     plain rename would leave it writing into the rotated archive while the
//     fresh file stays empty forever. Use copy-truncate instead: archive a copy,
//     then truncate the original in place; the O_APPEND handle stays valid.
func (a *AgentApp) handleRotateLog(req *agentpb.RotateLogRequest) *agentpb.RotateLogResponse {
	logPath := req.GetLogPath()
	if logPath != "" {
		logPath = a.resolveRelPath(logPath)
		if logPath == "" {
			return &agentpb.RotateLogResponse{
				Success: false,
				Error:   "invalid log path",
			}
		}
	} else {
		logPath = a.resolveLogFilePath(req.LogType)
	}
	if logPath == "" {
		return &agentpb.RotateLogResponse{
			Success: false,
			Error:   fmt.Sprintf("unknown log type: %s", req.LogType),
		}
	}

	info, err := os.Stat(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &agentpb.RotateLogResponse{
				Success:      true,
				RotatedFiles: 0,
				CleanedFiles: 0,
				FreedBytes:   0,
			}
		}
		return &agentpb.RotateLogResponse{
			Success: false,
			Error:   fmt.Sprintf("stat log file: %v", err),
		}
	}

	if info.Size() == 0 {
		return &agentpb.RotateLogResponse{
			Success:      true,
			RotatedFiles: 0,
			CleanedFiles: 0,
			FreedBytes:   0,
		}
	}

	// Rotate with date suffix (append counter on same-day duplicates)
	dateSuffix := time.Now().Format("20060102")
	rotatedPath := fmt.Sprintf("%s.%s", logPath, dateSuffix)
	if _, err := os.Stat(rotatedPath); err == nil {
		for i := 2; i < 100; i++ {
			rotatedPath = fmt.Sprintf("%s.%s.%d", logPath, dateSuffix, i)
			if _, err := os.Stat(rotatedPath); os.IsNotExist(err) {
				break
			}
		}
	}

	if req.LogType == "nginx" {
		// nginx supports a reopen signal: rename + recreate, then reopen FDs.
		if err := os.Rename(logPath, rotatedPath); err != nil {
			return &agentpb.RotateLogResponse{
				Success: false,
				Error:   fmt.Sprintf("rename log file: %v", err),
			}
		}
		newFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY, info.Mode())
		if err != nil {
			_ = os.Rename(rotatedPath, logPath)
			return &agentpb.RotateLogResponse{
				Success: false,
				Error:   fmt.Sprintf("create new log file: %v", err),
			}
		}
		newFile.Close()

		if nginxPath, err := exec.LookPath("nginx"); err == nil {
			if out, err := exec.Command(nginxPath, "-s", "reopen").CombinedOutput(); err != nil {
				log.Printf("[Agent] nginx -s reopen failed: %v, output: %s", err, string(out))
			}
		} else {
			log.Printf("[Agent] nginx not found in PATH, skipping reopen signal")
		}
	} else {
		// No reopen support → copy-truncate: archive a copy, truncate in place.
		// The writer's O_APPEND handle keeps working on the truncated file.
		src, err := os.Open(logPath)
		if err != nil {
			return &agentpb.RotateLogResponse{
				Success: false,
				Error:   fmt.Sprintf("open log file: %v", err),
			}
		}
		dst, err := os.OpenFile(rotatedPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
		if err != nil {
			src.Close()
			return &agentpb.RotateLogResponse{
				Success: false,
				Error:   fmt.Sprintf("create rotated file: %v", err),
			}
		}
		if _, err := io.Copy(dst, src); err != nil {
			src.Close()
			dst.Close()
			return &agentpb.RotateLogResponse{
				Success: false,
				Error:   fmt.Sprintf("copy log file: %v", err),
			}
		}
		src.Close()
		dst.Close()

		if err := os.Truncate(logPath, 0); err != nil {
			return &agentpb.RotateLogResponse{
				Success: false,
				Error:   fmt.Sprintf("truncate log file: %v", err),
			}
		}
	}

	return &agentpb.RotateLogResponse{
		Success:      true,
		RotatedFiles: 1,
		CleanedFiles: 0,
		FreedBytes:   info.Size(),
	}
}

// handlePackageLogs collects logs within a time range and packages them into a gzip file.
func (a *AgentApp) handlePackageLogs(req *agentpb.PackageLogsRequest) *agentpb.PackageLogsResponse {
	logPath := a.resolveLogFilePath(req.LogType)
	if logPath == "" {
		return &agentpb.PackageLogsResponse{
			Success: false,
			Error:   fmt.Sprintf("unknown log type: %s", req.LogType),
		}
	}

	startTime := time.Unix(req.StartTime, 0)
	endTime := time.Unix(req.EndTime, 0)

	// Collect log files (current + rotated)
	var logFiles []string
	if _, err := os.Stat(logPath); err == nil {
		logFiles = append(logFiles, logPath)
	}

	if req.IncludeRotated {
		dir := filepath.Dir(logPath)
		base := filepath.Base(logPath)
		entries, err := os.ReadDir(dir)
		if err == nil {
			for _, entry := range entries {
				name := entry.Name()
				if len(name) > len(base) && name[:len(base)] == base && name != base {
					logFiles = append(logFiles, filepath.Join(dir, name))
				}
			}
		}
	}

	// Create package directory
	pkgDir := filepath.Join(a.dataDir, "logs", "packages")
	os.MkdirAll(pkgDir, 0750)

	pkgPath := filepath.Join(pkgDir, fmt.Sprintf("pkg_%d.tar.gz", time.Now().Unix()))
	pkgFile, err := os.Create(pkgPath)
	if err != nil {
		return &agentpb.PackageLogsResponse{
			Success: false,
			Error:   fmt.Sprintf("create package file: %v", err),
		}
	}
	defer pkgFile.Close()

	gw := gzip.NewWriter(pkgFile)
	defer gw.Close()

	writer := io.Writer(gw)
	if req.Desensitize {
		writer = newDesensitizingWriter(gw)
	}

	totalLines := 0
	for _, lf := range logFiles {
		f, err := os.Open(lf)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			// Try to parse timestamp from line and filter by time range
			if isInTimeRange(line, startTime, endTime) {
				fmt.Fprintln(writer, line)
				totalLines++
			}
		}
		f.Close()
	}

	info, _ := os.Stat(pkgPath)

	log.Printf("[Agent] packaged %d lines from %s (%d bytes)", totalLines, req.LogType, info.Size())

	return &agentpb.PackageLogsResponse{
		Success:       true,
		PackagePath:   pkgPath,
		PackageSize:   info.Size(),
		DownloadToken: fmt.Sprintf("pkg_%d", time.Now().Unix()),
	}
}

// readLogIncremental serves the offset-based incremental read (tail -F) for the
// panel's SSE log streams. next_offset always lands after the last COMPLETE
// newline, so no line is ever split or lost across polls. If the file was
// truncated/rotated (size < offset), it falls back to a fresh tail and flags
// truncated=true so the panel can re-anchor.
func (a *AgentApp) readLogIncremental(req *agentpb.ReadLogFileRequest, lastLines int) *agentpb.ReadLogFileResponse {
	logPath := req.GetLogPath()
	if logPath != "" {
		logPath = a.resolveRelPath(logPath)
		if logPath == "" {
			return &agentpb.ReadLogFileResponse{Success: false, Error: "invalid log path"}
		}
	} else {
		logPath = a.resolveLogFilePath(req.LogType)
	}
	if logPath == "" {
		return &agentpb.ReadLogFileResponse{Success: false, Error: fmt.Sprintf("unknown log type: %s", req.LogType)}
	}

	resp := &agentpb.ReadLogFileResponse{LogPath: logPath}

	info, err := os.Stat(logPath)
	if err != nil {
		// 文件消失（轮转/清理）→ 让面板重新 tail 锚定
		resp.Success = true
		resp.Truncated = true
		return resp
	}

	// 截断/轮转：文件比记录的 offset 还小 → 重新 tail 锚定
	if info.Size() < req.Offset {
		f, err := os.Open(logPath)
		if err != nil {
			resp.Success = false
			resp.Error = fmt.Sprintf("open log file: %v", err)
			return resp
		}
		defer f.Close()
		lines, err := readLastLinesFromFile(f, lastLines)
		if err != nil {
			resp.Success = false
			resp.Error = fmt.Sprintf("read log file: %v", err)
			return resp
		}
		resp.Success = true
		resp.Lines = lines
		resp.NextOffset = info.Size()
		resp.Truncated = true
		return resp
	}

	f, err := os.Open(logPath)
	if err != nil {
		resp.Success = false
		resp.Error = fmt.Sprintf("open log file: %v", err)
		return resp
	}
	defer f.Close()

	if _, err := f.Seek(req.Offset, io.SeekStart); err != nil {
		resp.Success = false
		resp.Error = fmt.Sprintf("seek log file: %v", err)
		return resp
	}
	data, err := io.ReadAll(io.LimitReader(f, maxIncrementalReadBytes))
	if err != nil {
		resp.Success = false
		resp.Error = fmt.Sprintf("read log file: %v", err)
		return resp
	}

	// 只消费到最后一个完整 \n；半行留在文件里，下次 offset 不越过它
	nextOffset := req.Offset
	lines := []string{}
	if cut := bytes.LastIndexByte(data, '\n'); cut >= 0 {
		nextOffset = req.Offset + int64(cut) + 1
		body := strings.TrimSuffix(string(data[:cut+1]), "\n")
		if body != "" {
			lines = strings.Split(body, "\n")
		}
	}

	filterEmails := req.GetFilterEmails()
	hasFilter := len(filterEmails) > 0 || req.StartTime > 0 || req.EndTime > 0
	var filterStart, filterEnd time.Time
	if req.StartTime > 0 {
		filterStart = time.Unix(req.StartTime, 0)
	}
	if req.EndTime > 0 {
		filterEnd = time.Unix(req.EndTime, 0)
	}

	resp.Success = true
	resp.Lines = agentFilterLines(lines, filterEmails, filterStart, filterEnd, hasFilter)
	resp.NextOffset = nextOffset
	return resp
}

// maxIncrementalReadBytes caps one incremental read (16 MB) — a stream left
// open for a long time on a hot file will be served in bounded chunks.
const maxIncrementalReadBytes = 16 << 20

// resolveLogFilePath resolves the log file path for a given log type.
// In stdout mode, the log is persisted to logs/{type}-stdout.log by LogWriter.
// In file mode, the log is written to the process's configured log file path.
func (a *AgentApp) resolveLogFilePath(logType string) string {
	logDir := filepath.Join(a.dataDir, "logs")

	// Determine log output mode from settings
	mode := a.getLogMode(logType)

	// In stdout mode, LogWriter persists captured stdout to {type}-stdout.log
	if mode == "stdout" {
		return filepath.Join(logDir, logType+"-stdout.log")
	}

	// File mode (or off): return the process's default log file path
	switch logType {
	case "xray":
		// ⚠ 用户活动行（accepted + email:）只写入 **access** 日志，error 日志里
		//   只有警告 → 打包/查看用户日志必须读 access 日志（真机验证踩到：
		//   xray-access.log 12 万行 email 记录、xray.log 0 行）。优先用运行时
		//   配置里声明的 access 路径，兜底默认名。
		accessPath, _ := a.readXrayLogPaths()
		if accessPath != "" {
			return accessPath
		}
		return filepath.Join(logDir, "xray-access.log")
	case "singbox":
		return filepath.Join(logDir, "singbox.log")
	case "nginx":
		// Read actual error log path from nginx settings
		ctx := context.Background()
		if v, err := a.settingRepo.GetByKey(ctx, "proxy.nginx_settings"); err == nil && v != "" {
			var ns struct {
				ErrorLogPath string `json:"error_log_path"`
			}
			if json.Unmarshal([]byte(v), &ns) == nil && ns.ErrorLogPath != "" {
				return a.resolveRelPath(ns.ErrorLogPath)
			}
		}
		return filepath.Join(logDir, "nginx-error.log")
	case "caddy":
		// Caddy logs are per-domain; return the first enabled site's log path
		ctx := context.Background()
		if a.caddySiteRepo != nil {
			sites, err := a.caddySiteRepo.List(ctx, 1)
			if err == nil {
				for _, site := range sites {
					if site.Template == "l4" || site.Template == "sni" {
						continue
					}
					if !site.LogEnabled {
						continue
					}
					logPath := site.LogPath
					if logPath == "" {
						logPath = fmt.Sprintf("logs/%s.log", sanitizeDomainForLog(site.Domain))
					}
					return a.resolveRelPath(logPath)
				}
			}
		}
		// 兜底：无启日志站点时读 Caddy 全局日志（与 Caddyfile 生成的
		// "output file logs/caddy_access.log" 同名，下划线——此前误用横线
		// caddy-access.log 导致"日志文件不存在"）。
		return filepath.Join(logDir, "caddy_access.log")
	default:
		return ""
	}
}

// getLogMode reads the log output mode for a given log type from local settings.
// Core types (xray/singbox) use "core.log_output_config".
// Proxy types (nginx/caddy) use "proxy.log_output_config".
// Returns "file" as default if not configured.
func (a *AgentApp) getLogMode(logType string) string {
	if a.settingRepo == nil {
		return "file"
	}

	ctx := context.Background()
	isCore := false
	for _, ct := range []string{"xray", "singbox"} {
		if logType == ct {
			isCore = true
			break
		}
	}

	if isCore {
		v, err := a.settingRepo.GetByKey(ctx, "core.log_output_config")
		if err != nil || v == "" {
			// No saved config — derive from actual runtime config
			return a.deriveCoreLogMode(logType)
		}
		var cfg struct {
			Xray struct {
				Mode string `json:"mode"`
			} `json:"xray"`
			Singbox struct {
				Mode string `json:"mode"`
			} `json:"singbox"`
		}
		if json.Unmarshal([]byte(v), &cfg) != nil {
			return "file"
		}
		// For xray and singbox, override with mode derived from actual runtime config
		switch logType {
		case "xray":
			derived := a.deriveCoreLogMode("xray")
			if derived != "" {
				return derived
			}
			return cfg.Xray.Mode
		case "singbox":
			derived := a.deriveCoreLogMode("singbox")
			if derived != "" {
				return derived
			}
			return cfg.Singbox.Mode
		}
		return "file"
	}

	// Proxy types
	v, err := a.settingRepo.GetByKey(ctx, "proxy.log_output_config")
	if err != nil || v == "" {
		return "file"
	}
	var cfg struct {
		Nginx struct {
			Mode string `json:"mode"`
		} `json:"nginx"`
		Caddy struct {
			Mode string `json:"mode"`
		} `json:"caddy"`
	}
	if json.Unmarshal([]byte(v), &cfg) != nil {
		return "file"
	}
	switch logType {
	case "nginx":
		return cfg.Nginx.Mode
	case "caddy":
		return cfg.Caddy.Mode
	}
	return "file"
}

// deriveCoreLogMode infers the log output mode for xray/singbox by reading the
// actual runtime config file. This keeps the Agent's mode in sync with changes
// made in the protocol center without needing to save the log output config.
// Returns "" if the mode cannot be determined (caller should fall back to saved config).
func (a *AgentApp) deriveCoreLogMode(coreType string) string {
	switch coreType {
	case "xray":
		// Read xray.jsonc and infer mode from log section
		configPath := filepath.Join(a.dataDir, "runtime", "xray.jsonc")
		data, err := os.ReadFile(configPath)
		if err != nil {
			return ""
		}
		cleaned := stripJSONCComments(string(data))
		var cfg struct {
			Log struct {
				Access   string `json:"access"`
				Error    string `json:"error"`
				LogLevel string `json:"loglevel"`
			} `json:"log"`
		}
		if json.Unmarshal([]byte(cleaned), &cfg) != nil {
			return ""
		}
		// off mode
		if cfg.Log.LogLevel == "none" {
			return "off"
		}
		if cfg.Log.Access == "none" && cfg.Log.Error == "none" {
			return "off"
		}
		// stdout mode — empty string means stdout in xray, /dev/stderr is LogBroker capture
		if cfg.Log.Access == "" || cfg.Log.Error == "" || cfg.Log.Error == "/dev/stderr" {
			return "stdout"
		}
		// file mode — has actual file paths
		return "file"

	case "singbox":
		// Read singbox config and infer mode from log section
		configPath := filepath.Join(a.dataDir, "runtime", "singbox.json")
		data, err := os.ReadFile(configPath)
		if err != nil {
			// Try .jsonc extension
			configPath = filepath.Join(a.dataDir, "runtime", "singbox.jsonc")
			data, err = os.ReadFile(configPath)
			if err != nil {
				return ""
			}
		}
		cleaned := stripJSONCComments(string(data))
		var cfg struct {
			Log struct {
				Level  string `json:"level"`
				Output string `json:"output"`
			} `json:"log"`
		}
		if json.Unmarshal([]byte(cleaned), &cfg) != nil {
			return ""
		}
		if cfg.Log.Level == "silent" {
			return "off"
		}
		if cfg.Log.Output == "stderr" || cfg.Log.Output == "" {
			return "stdout"
		}
		return "file"
	}
	return ""
}

// readLastLinesFromFile reads the last N lines from an open file.
// 反向分块读取，收集时用 append（最后 reverse）——禁止逐行 prepend（O(n²)，
// 5 万行 = 上十亿次元素拷贝，会把请求卡到超时）。
func readLastLinesFromFile(f *os.File, n int) ([]string, error) {
	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}
	fileSize := stat.Size()
	if fileSize == 0 {
		return []string{}, nil
	}

	bufSize := int64(64 * 1024)
	rev := make([]string, 0, n) // 逆文件序收集（最后 reverse）
	var offset int64 = fileSize
	var leftover []byte

	for len(rev) < n+1 && offset > 0 {
		readSize := bufSize
		if offset < readSize {
			readSize = offset
		}
		offset -= readSize

		chunk := make([]byte, readSize)
		if _, err := f.ReadAt(chunk, offset); err != nil && err != io.EOF {
			return nil, err
		}

		data := append(chunk, leftover...)
		for i := len(data) - 1; i >= 0; i-- {
			if data[i] == '\n' {
				line := string(data[i+1:])
				if len(line) > 0 && line[len(line)-1] == '\r' {
					line = line[:len(line)-1]
				}
				if line != "" {
					rev = append(rev, line)
				}
				if len(rev) >= n {
					break
				}
				data = data[:i]
			}
		}
		leftover = data
		if len(rev) >= n {
			break
		}
	}

	if len(leftover) > 0 && len(rev) < n {
		line := string(leftover)
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
		if line != "" {
			rev = append(rev, line)
		}
	}

	// rev 是逆文件序 → 反转成文件序，并只保留最后 n 行
	if len(rev) > n {
		rev = rev[:n]
	}
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	return rev, nil
}

// isInTimeRange checks if a log line's timestamp falls within the given range.
// Lines without a parseable timestamp are always included.
// ⚠ 必须用 ParseInLocation（本地时区）：日志行写的是节点本地时间，按 UTC 解析会把
// 时间平移时区差（+08 → 全部"落在未来"）→ 过滤后一行不剩（真机验证踩到）。
func isInTimeRange(line string, start, end time.Time) bool {
	// Try to parse common timestamp formats from the beginning of the line
	// Format: 2026/07/08 16:00:00 ...
	if len(line) >= 19 {
		t, err := time.ParseInLocation("2006/01/02 15:04:05", line[:19], time.Local)
		if err == nil {
			return !t.Before(start) && !t.After(end)
		}
	}
	// If we can't parse the timestamp, include the line (conservative approach)
	return true
}

// agentFilterLines filters log lines by email identifiers and/or time range.
// This is used for client log packaging: the panel sends the user's inbound email
// identifiers so the agent can pre-filter on the remote node, avoiding the need to
// transfer the entire log file over the network.
// If hasFilter is false (no filter_emails, no time range), lines are returned as-is.
// A nil input returns nil (safe for protobuf nil slices).
func agentFilterLines(lines []string, filterEmails []string, start, end time.Time, hasFilter bool) []string {
	if !hasFilter || len(lines) == 0 {
		return lines
	}

	// Build email lookup set for fast matching
	emailSet := make(map[string]bool, len(filterEmails))
	for _, e := range filterEmails {
		emailSet[e] = true
	}

	hasTimeFilter := !start.IsZero() || !end.IsZero()
	filtered := make([]string, 0, len(lines))

	for _, line := range lines {
		// Filter by email: only keep lines containing at least one of the filter emails
		if len(emailSet) > 0 {
			matched := false
			for email := range emailSet {
				if strings.Contains(line, email) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		// Filter by time range
		if hasTimeFilter && !isInTimeRange(line, start, end) {
			continue
		}

		filtered = append(filtered, line)
	}

	return filtered
}

// ============================================================
// Log desensitization
// ============================================================

var (
	desensitizeIPv4   = regexp.MustCompile(`\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}`)
	desensitizeUUID   = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	desensitizeEmail  = regexp.MustCompile(`[\w.-]+@[\w.-]+\.\w+`)
	desensitizePasswd = regexp.MustCompile(`(?i)password["\s:=]+["']?[^"'\s,}]+`)
	desensitizeToken  = regexp.MustCompile(`(?i)token["\s:=]+["']?[^"'\s,}]+`)
)

// desensitizingWriter wraps an io.Writer and applies regex substitutions to each line.
type desensitizingWriter struct {
	w       *bufio.Writer
	scanner *bufio.Scanner
	buf     []byte
}

func newDesensitizingWriter(w io.Writer) *desensitizingWriter {
	return &desensitizingWriter{
		w: bufio.NewWriter(w),
	}
}

func (dw *desensitizingWriter) Write(p []byte) (int, error) {
	dw.buf = append(dw.buf, p...)
	// Process complete lines
	for {
		idx := -1
		for i, b := range dw.buf {
			if b == '\n' {
				idx = i
				break
			}
		}
		if idx < 0 {
			break
		}
		line := string(dw.buf[:idx])
		dw.buf = dw.buf[idx+1:]
		desensitizedLine := desensitizeLine(line)
		dw.w.WriteString(desensitizedLine)
		dw.w.WriteByte('\n')
	}
	return len(p), nil
}

func (dw *desensitizingWriter) Close() error {
	// Flush remaining buffer
	if len(dw.buf) > 0 {
		dw.w.WriteString(desensitizeLine(string(dw.buf)))
		dw.buf = nil
	}
	return dw.w.Flush()
}

func desensitizeLine(line string) string {
	line = desensitizeIPv4.ReplaceAllString(line, "***.***.***.***")
	line = desensitizeUUID.ReplaceAllString(line, "********-****-****-****-************")
	line = desensitizeEmail.ReplaceAllString(line, "***@***.***")
	line = desensitizePasswd.ReplaceAllString(line, "password: \"***\"")
	line = desensitizeToken.ReplaceAllString(line, "token: \"***\"")
	return line
}

// handleDiscoverLogs discovers all log files on the agent node.
// It returns a list of discovered log files similar to the panel's DiscoverLogs.
func (a *AgentApp) handleDiscoverLogs(ctx context.Context) *agentpb.DiscoverLogsResponse {
	logDir := filepath.Join(a.dataDir, "logs")
	var logs []*agentpb.DiscoveredLogInfo

	// Discover core process logs
	for _, coreType := range []string{"xray", "singbox"} {
		mode := a.getLogMode(coreType)
		switch mode {
		case "stdout":
			// stdout mode: LogWriter captures stdout and manages the capture file
			// (including rotation), so we don't discover it as a separate log.
			continue
		case "off":
			continue
		default: // "file"
			if coreType == "xray" {
				// Read actual access/error paths from the runtime config file
				accessPath, errorPath := a.readXrayLogPaths()
				if errorPath != "" {
					logs = append(logs, &agentpb.DiscoveredLogInfo{
						SourceType: coreType,
						Domain:     "error",
						LogPath:    errorPath,
					})
				}
				if accessPath != "" {
					logs = append(logs, &agentpb.DiscoveredLogInfo{
						SourceType: coreType,
						Domain:     "access",
						LogPath:    accessPath,
					})
				}
			} else {
				logs = append(logs, &agentpb.DiscoveredLogInfo{
					SourceType: coreType,
					Domain:     mode,
					LogPath:    filepath.Join(logDir, coreType+".log"),
				})
			}
		}
	}

	// Discover nginx logs — read actual log paths from proxy.nginx_settings
	nginxMode := "file"
	if v, err := a.settingRepo.GetByKey(ctx, "proxy.log_output_config"); err == nil && v != "" {
		var cfg struct {
			Nginx struct {
				Mode string `json:"mode"`
			} `json:"nginx"`
		}
		if json.Unmarshal([]byte(v), &cfg) == nil && cfg.Nginx.Mode != "" {
			nginxMode = cfg.Nginx.Mode
		}
	}
	// In stdout mode, nginx writes to stdout which is captured by LogWriter.
	// LogWriter manages the stdout capture file and its rotation, so we don't
	// discover it as a separate log to avoid duplicate rotation.
	if nginxMode == "stdout" {
		// skip
	} else if nginxMode != "off" {
		// Read nginx settings to get actual log paths
		type nginxLogSettings struct {
			ErrorLog      bool   `json:"error_log"`
			ErrorLogPath  string `json:"error_log_path"`
			AccessLog     bool   `json:"access_log"`
			AccessLogPath string `json:"access_log_path"`
			StreamLog     bool   `json:"stream_log"`
			StreamLogPath string `json:"stream_log_path"`
		}
		var ns nginxLogSettings
		if v, err := a.settingRepo.GetByKey(ctx, "proxy.nginx_settings"); err == nil && v != "" {
			json.Unmarshal([]byte(v), &ns)
		}
		// Use defaults if settings are empty
		if ns.ErrorLogPath == "" {
			ns.ErrorLogPath = "logs/error.log"
			ns.ErrorLog = true
		}
		if ns.AccessLogPath == "" {
			ns.AccessLogPath = "logs/access.log"
			ns.AccessLog = true
		}

		if ns.ErrorLog && ns.ErrorLogPath != "" {
			logs = append(logs, &agentpb.DiscoveredLogInfo{
				SourceType: "nginx",
				Domain:     "error",
				LogPath:    a.resolveRelPath(ns.ErrorLogPath),
			})
		}
		if ns.AccessLog && ns.AccessLogPath != "" {
			logs = append(logs, &agentpb.DiscoveredLogInfo{
				SourceType: "nginx",
				Domain:     "access",
				LogPath:    a.resolveRelPath(ns.AccessLogPath),
			})
		}
		if ns.StreamLog && ns.StreamLogPath != "" {
			logs = append(logs, &agentpb.DiscoveredLogInfo{
				SourceType: "nginx",
				Domain:     "stream",
				LogPath:    a.resolveRelPath(ns.StreamLogPath),
			})
		}
	}

	// Discover caddy logs — read actual log paths from caddy_sites table
	caddyMode := "file"
	if v, err := a.settingRepo.GetByKey(ctx, "proxy.log_output_config"); err == nil && v != "" {
		var cfg struct {
			Caddy struct {
				Mode string `json:"mode"`
			} `json:"caddy"`
		}
		if json.Unmarshal([]byte(v), &cfg) == nil && cfg.Caddy.Mode != "" {
			caddyMode = cfg.Caddy.Mode
		}
	}
	// In stdout mode, caddy writes to stdout which is captured by LogWriter.
	// LogWriter manages the stdout capture file and its rotation, so we don't
	// discover it as a separate log to avoid duplicate rotation.
	if caddyMode == "stdout" {
		// skip
	} else if caddyMode != "off" {
		// 全局统一 access 日志（与 Caddyfile 生成的 "output file logs/caddy_access.log"
		// 同名，面板本地的 discoverCaddyLogs 也始终包含它）——站点表为空时兜底。
		logs = append(logs, &agentpb.DiscoveredLogInfo{
			SourceType: "caddy",
			Domain:     "access",
			LogPath:    a.resolveRelPath("logs/caddy_access.log"),
			LogFormat:  "json",
		})
		// Per-site custom log paths (sites that override the global logger)
		if a.caddySiteRepo != nil {
			sites, err := a.caddySiteRepo.List(ctx, 1)
			if err == nil {
				globalPath := a.resolveRelPath("logs/caddy_access.log")
				for _, site := range sites {
					// Skip L4 and SNI sites — they don't have HTTP-level logs
					if site.Template == "l4" || site.Template == "sni" {
						continue
					}
					if !site.LogEnabled || site.LogPath == "" {
						continue
					}
					logPath := a.resolveRelPath(site.LogPath)
					if logPath == globalPath {
						continue // 已在全局条目里
					}
					logs = append(logs, &agentpb.DiscoveredLogInfo{
						SourceType: "caddy",
						Domain:     site.Domain,
						LogPath:    logPath,
						LogFormat:  site.LogFormat,
					})
				}
			}
		}
	}

	// Cross-check with local policies
	if a.logMgr != nil {
		policies, _ := a.logMgr.ListPolicies(ctx)
		policyPaths := make(map[string]bool)
		for _, p := range policies {
			policyPaths[p.LogPath] = true
		}
		for _, l := range logs {
			if policyPaths[l.LogPath] {
				l.HasPolicy = true
			}
		}
	}

	return &agentpb.DiscoverLogsResponse{
		Success: true,
		Logs:    logs,
	}
}

// resolveRelPath resolves a relative path (e.g. "logs/error.log") to an absolute
// path under the agent's data directory. Absolute paths are returned as-is.
func (a *AgentApp) resolveRelPath(p string) string {
	// ⚠ 日志路径也来自面板，而 Agent 以 root 运行：`../../etc/shadow` 会被读回面板 ✗
	// （安全审计 v5 H3）。这里拒绝上跳 —— 返回空串让后续 ReadFile 失败（fail closed）。
	// 合法场景不受影响：绝对路径（如 nginx 自己的 /var/log/...）与数据目录下的相对路径都保留。
	if err := safepath.RejectTraversal("log path", p); err != nil {
		log.Printf("[Agent] resolveRelPath: %v", err)
		return ""
	}
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(a.dataDir, p)
}

// readXrayLogPaths reads the access and error log paths from the xray runtime
// config file (dataDir/runtime/xray.jsonc). Returns resolved absolute paths.
// If the config file doesn't exist or has no log section, falls back to defaults.
func (a *AgentApp) readXrayLogPaths() (accessPath, errorPath string) {
	// Defaults
	accessPath = filepath.Join(a.dataDir, "logs", "xray-access.log")
	errorPath = filepath.Join(a.dataDir, "logs", "xray.log")

	configPath := filepath.Join(a.dataDir, "runtime", "xray.jsonc")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return
	}

	// xray.jsonc may contain // comments — strip them before parsing.
	cleaned := stripJSONCComments(string(data))
	var cfg struct {
		Log struct {
			Access string `json:"access"`
			Error  string `json:"error"`
		} `json:"log"`
	}
	if json.Unmarshal([]byte(cleaned), &cfg) != nil {
		return
	}

	if cfg.Log.Access != "" && cfg.Log.Access != "none" {
		accessPath = a.resolveRelPath(cfg.Log.Access)
	} else {
		accessPath = "" // access log disabled
	}
	if cfg.Log.Error != "" && cfg.Log.Error != "none" {
		errorPath = a.resolveRelPath(cfg.Log.Error)
	} else {
		errorPath = "" // error log disabled
	}
	return
}

// stripJSONCComments removes // line comments and /* block comments */ from
// JSONC content so it can be parsed as standard JSON.
func stripJSONCComments(s string) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		if i+1 < len(s) && s[i] == '/' && s[i+1] == '/' {
			// Skip to end of line
			for i < len(s) && s[i] != '\n' {
				i++
			}
		} else if i+1 < len(s) && s[i] == '/' && s[i+1] == '*' {
			// Skip to */
			i += 2
			for i+1 < len(s) && !(s[i] == '*' && s[i+1] == '/') {
				i++
			}
			i += 2
		} else if s[i] == '"' {
			// Copy string literal (including escaped quotes)
			b.WriteByte(s[i])
			i++
			for i < len(s) && s[i] != '"' {
				if s[i] == '\\' && i+1 < len(s) {
					b.WriteByte(s[i])
					b.WriteByte(s[i+1])
					i += 2
				} else {
					b.WriteByte(s[i])
					i++
				}
			}
			if i < len(s) {
				b.WriteByte(s[i])
				i++
			}
		} else {
			b.WriteByte(s[i])
			i++
		}
	}
	return b.String()
}

// sanitizeDomainForLog converts a domain to a safe log file name.
func sanitizeDomainForLog(domain string) string {
	if domain == "" || domain == "_" || domain == ":80" || domain == ":443" {
		return "default"
	}
	if strings.HasPrefix(domain, ":") {
		return "port" + domain[1:]
	}
	return domain
}

// handleGetWriterStatus returns the status of all LogWriters on this agent node.
func (a *AgentApp) handleGetWriterStatus(ctx context.Context) map[string]interface{} {
	if a.logMgr == nil {
		return map[string]interface{}{
			"success": false,
			"error":   "log manager not initialized",
		}
	}
	statuses := a.logMgr.GetWriterStatus()
	writers := make([]map[string]interface{}, 0, len(statuses))
	for _, st := range statuses {
		writers = append(writers, map[string]interface{}{
			"proxy_type":  st.ProxyType,
			"mode":        st.Mode,
			"managed":     st.Managed,
			"active":      st.Active,
			"paused":      st.Paused,
			"file_path":   st.FilePath,
			"log_dir":     st.LogDir,
			"retain_days": st.RetainDays,
			"compress":    st.Compress,
			"max_size_mb": st.MaxSizeMB,
		})
	}
	return map[string]interface{}{
		"success": true,
		"writers": writers,
	}
}
