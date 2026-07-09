package main

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	agentpb "corefusion/internal/runtime/agent/proto"
)

// handleReadLogFile reads the last N lines from a log file on the agent node.
func (a *AgentApp) handleReadLogFile(req *agentpb.ReadLogFileRequest) *agentpb.ReadLogFileResponse {
	logPath := a.resolveLogFilePath(req.LogType)
	if logPath == "" {
		return &agentpb.ReadLogFileResponse{
			Success: false,
			Error:   fmt.Sprintf("unknown log type: %s", req.LogType),
		}
	}

	lastLines := int(req.LastLines)
	if lastLines <= 0 {
		lastLines = 500
	}

	f, err := os.Open(logPath)
	if err != nil {
		return &agentpb.ReadLogFileResponse{
			Success:  false,
			Error:    fmt.Sprintf("open log file: %v", err),
			LogPath:  logPath,
		}
	}
	defer f.Close()

	lines, err := readLastLinesFromFile(f, lastLines)
	if err != nil {
		return &agentpb.ReadLogFileResponse{
			Success:  false,
			Error:    fmt.Sprintf("read log file: %v", err),
			LogPath:  logPath,
		}
	}

	return &agentpb.ReadLogFileResponse{
		Success:  true,
		Lines:    lines,
		LogPath:  logPath,
	}
}

// handleRotateLog triggers log rotation for a specific log type on the agent node.
func (a *AgentApp) handleRotateLog(req *agentpb.RotateLogRequest) *agentpb.RotateLogResponse {
	logPath := a.resolveLogFilePath(req.LogType)
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
				Success:       true,
				RotatedFiles:  0,
				CleanedFiles:  0,
				FreedBytes:    0,
			}
		}
		return &agentpb.RotateLogResponse{
			Success: false,
			Error:   fmt.Sprintf("stat log file: %v", err),
		}
	}

	if info.Size() == 0 {
		return &agentpb.RotateLogResponse{
			Success:       true,
			RotatedFiles:  0,
			CleanedFiles:  0,
			FreedBytes:    0,
		}
	}

	// Rotate: rename with date suffix
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

	return &agentpb.RotateLogResponse{
		Success:       true,
		RotatedFiles:  1,
		CleanedFiles:  0,
		FreedBytes:    info.Size(),
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
		Success:      true,
		PackagePath:  pkgPath,
		PackageSize:  info.Size(),
		DownloadToken: fmt.Sprintf("pkg_%d", time.Now().Unix()),
	}
}

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
		return filepath.Join(logDir, "xray.log")
	case "singbox":
		return filepath.Join(logDir, "singbox.log")
	case "hysteria2":
		return filepath.Join(logDir, "hysteria2.log")
	case "tuic":
		return filepath.Join(logDir, "tuic.log")
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
		return filepath.Join(logDir, "caddy-access.log")
	default:
		return ""
	}
}

// getLogMode reads the log output mode for a given log type from local settings.
// Core types (xray/singbox/hysteria2/tuic) use "core.log_output_config".
// Proxy types (nginx/caddy) use "proxy.log_output_config".
// Returns "file" as default if not configured.
func (a *AgentApp) getLogMode(logType string) string {
	if a.settingRepo == nil {
		return "file"
	}

	ctx := context.Background()
	isCore := false
	for _, ct := range []string{"xray", "singbox", "hysteria2", "tuic"} {
		if logType == ct {
			isCore = true
			break
		}
	}

	if isCore {
		v, err := a.settingRepo.GetByKey(ctx, "core.log_output_config")
		if err != nil || v == "" {
			return "file"
		}
		var cfg struct {
			Xray      struct{ Mode string `json:"mode"` } `json:"xray"`
			Singbox   struct{ Mode string `json:"mode"` } `json:"singbox"`
			Hysteria2 struct{ Mode string `json:"mode"` } `json:"hysteria2"`
			Tuic      struct{ Mode string `json:"mode"` } `json:"tuic"`
		}
		if json.Unmarshal([]byte(v), &cfg) != nil {
			return "file"
		}
		switch logType {
		case "xray":
			return cfg.Xray.Mode
		case "singbox":
			return cfg.Singbox.Mode
		case "hysteria2":
			return cfg.Hysteria2.Mode
		case "tuic":
			return cfg.Tuic.Mode
		}
		return "file"
	}

	// Proxy types
	v, err := a.settingRepo.GetByKey(ctx, "proxy.log_output_config")
	if err != nil || v == "" {
		return "file"
	}
	var cfg struct {
		Nginx struct{ Mode string `json:"mode"` } `json:"nginx"`
		Caddy struct{ Mode string `json:"mode"` } `json:"caddy"`
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

// readLastLinesFromFile reads the last N lines from an open file.
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
	var lines []string
	var offset int64 = fileSize
	var leftover []byte

	for len(lines) < n+1 && offset > 0 {
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
					lines = append([]string{line}, lines...)
				}
				if len(lines) >= n {
					break
				}
				data = data[:i]
			}
		}
		leftover = data
		if len(lines) >= n {
			break
		}
	}

	if len(leftover) > 0 && len(lines) < n {
		line := string(leftover)
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
		if line != "" {
			lines = append([]string{line}, lines...)
		}
	}

	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines, nil
}

// isInTimeRange checks if a log line's timestamp falls within the given range.
// Lines without a parseable timestamp are always included.
func isInTimeRange(line string, start, end time.Time) bool {
	// Try to parse common timestamp formats from the beginning of the line
	// Format: 2026/07/08 16:00:00 ...
	if len(line) >= 19 {
		t, err := time.Parse("2006/01/02 15:04:05", line[:19])
		if err == nil {
			return !t.Before(start) && !t.After(end)
		}
	}
	// If we can't parse the timestamp, include the line (conservative approach)
	return true
}

// ============================================================
// Log desensitization
// ============================================================

var (
	desensitizeIPv4    = regexp.MustCompile(`\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}`)
	desensitizeUUID    = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	desensitizeEmail   = regexp.MustCompile(`[\w.-]+@[\w.-]+\.\w+`)
	desensitizePasswd  = regexp.MustCompile(`(?i)password["\s:=]+["']?[^"'\s,}]+`)
	desensitizeToken   = regexp.MustCompile(`(?i)token["\s:=]+["']?[^"'\s,}]+`)
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
	for _, coreType := range []string{"xray", "singbox", "hysteria2", "tuic"} {
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
			Nginx struct{ Mode string `json:"mode"` } `json:"nginx"`
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
			Caddy struct{ Mode string `json:"mode"` } `json:"caddy"`
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
		// Read caddy site configs from database for actual log paths
		if a.caddySiteRepo != nil {
			sites, err := a.caddySiteRepo.List(ctx, 1)
			if err == nil {
				for _, site := range sites {
					// Skip L4 and SNI sites — they don't have HTTP-level logs
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
					logs = append(logs, &agentpb.DiscoveredLogInfo{
						SourceType: "caddy",
						Domain:     site.Domain,
						LogPath:    a.resolveRelPath(logPath),
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
