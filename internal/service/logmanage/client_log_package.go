package logmanage

import (
	"bufio"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"corefusion/internal/model/entity"
	"corefusion/internal/service/plan"
	"corefusion/internal/store/repository"
)

// ============================================================
// Client Log Packaging — 订阅用户日志查阅
// ============================================================
//
// 流程：
//   1. 客户端用户请求打包日志（最多24小时）
//   2. 系统根据 Client -> Plan -> PlanNodes -> InboundConfigs -> ServerIDs
//      找到用户关联的所有入口节点
//   3. 对每个节点读取对应核心进程的日志（本地直读 / 远程 gRPC）
//   4. 按时间范围过滤，脱敏后打包为 gzip
//   5. 存储到 data/logs/packages/ 下，记录到 client_log_packages 表
//   6. 客户端轮询状态，就绪后下载
//   7. 打包文件1小时后过期自动清理

const (
	// MaxClientLogHours 订阅用户最多查询24小时的日志
	MaxClientLogHours = 24
	// ClientPackageExpiry 打包文件过期时间
	ClientPackageExpiry = 1 * time.Hour
	// ClientPackageReadLines 从每个节点读取的最大行数
	ClientPackageReadLines = 50000
)

// NodeLogPackager provides log reading for remote nodes.
type NodeLogPackager interface {
	ReadLogFileOnServer(ctx context.Context, serverID int64, logType string, lastLines int) ([]string, string, error)
}

// ClientLogDeps holds dependencies for client log packaging.
type ClientLogDeps struct {
	PkgRepo      *repository.ClientLogPackageRepository
	ClientRepo   *repository.ClientRepository
	PlanSvc      *plan.Service
	InboundRepo  *repository.InboundConfigRepository
	NodePackager NodeLogPackager
}

// SetClientLogDeps injects dependencies for client log packaging.
func (s *Service) SetClientLogDeps(deps *ClientLogDeps) {
	s.clientLogDeps = deps
}

// InitClientLogDeps is a convenience method that creates and sets ClientLogDeps
// from individual dependencies. This avoids the caller needing to reference
// the ClientLogDeps type directly (useful when the package alias is shadowed
// by a variable of the same name).
func (s *Service) InitClientLogDeps(
	pkgRepo *repository.ClientLogPackageRepository,
	clientRepo *repository.ClientRepository,
	planSvc *plan.Service,
	inboundRepo *repository.InboundConfigRepository,
	nodePackager NodeLogPackager,
) {
	s.clientLogDeps = &ClientLogDeps{
		PkgRepo:      pkgRepo,
		ClientRepo:   clientRepo,
		PlanSvc:      planSvc,
		InboundRepo:  inboundRepo,
		NodePackager: nodePackager,
	}
}

// entryNodeInfo represents an entry node associated with a client.
type entryNodeInfo struct {
	ServerID int64
	LogType  string // xray / singbox / hysteria2 / tuic
	Tag      string
}

// CreatePackageTask creates an async log packaging task for a client.
// hours must be between 1 and 24.
func (s *Service) CreatePackageTask(ctx context.Context, clientID int64, hours int) (*entity.ClientLogPackage, error) {
	if s.clientLogDeps == nil {
		return nil, fmt.Errorf("client log packaging not configured")
	}
	if hours <= 0 || hours > MaxClientLogHours {
		hours = MaxClientLogHours
	}

	// Validate client exists and is active
	client, err := s.clientLogDeps.ClientRepo.GetByID(ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("client not found: %w", err)
	}
	if client.Status != "active" {
		return nil, fmt.Errorf("client is not active")
	}

	// Resolve entry nodes
	nodes, err := s.getClientEntryNodes(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("resolve entry nodes: %w", err)
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no entry nodes associated with this account")
	}

	// Generate package ID
	pkgID := generatePackageID()

	// Create initial record
	subID := int64(0)
	if client.SubscriptionID != nil {
		subID = *client.SubscriptionID
	}

	pkg := &entity.ClientLogPackage{
		ID:             pkgID,
		ClientID:       clientID,
		SubscriptionID: subID,
		LogType:        "mixed",
		Hours:          hours,
		Status:         "processing",
		ExpiresAt:      time.Now().Add(ClientPackageExpiry),
	}

	if err := s.clientLogDeps.PkgRepo.Create(ctx, pkg); err != nil {
		return nil, fmt.Errorf("create package record: %w", err)
	}

	// Process asynchronously
	go s.processPackageTask(clientID, pkgID, nodes, hours)

	log.Printf("logmanage: created log package %s for client %d (%d nodes, %dh)", pkgID, clientID, len(nodes), hours)
	return pkg, nil
}

// GetPackageStatus returns the status of a log package.
func (s *Service) GetPackageStatus(ctx context.Context, clientID int64, pkgID string) (*entity.ClientLogPackage, error) {
	if s.clientLogDeps == nil {
		return nil, fmt.Errorf("client log packaging not configured")
	}
	pkg, err := s.clientLogDeps.PkgRepo.GetByID(ctx, pkgID)
	if err != nil {
		return nil, err
	}
	if pkg.ClientID != clientID {
		return nil, fmt.Errorf("package not found")
	}
	// Check expiry
	if pkg.Status == "ready" && time.Now().After(pkg.ExpiresAt) {
		pkg.Status = "expired"
	}
	return pkg, nil
}

// ListClientPackages returns log packages for a client.
func (s *Service) ListClientPackages(ctx context.Context, clientID int64, limit, offset int) ([]entity.ClientLogPackage, int64, error) {
	if s.clientLogDeps == nil {
		return nil, 0, fmt.Errorf("client log packaging not configured")
	}
	return s.clientLogDeps.PkgRepo.ListByClientID(ctx, clientID, limit, offset)
}

// DownloadPackage returns the file path for a ready package.
// The caller is responsible for serving the file and cleaning up.
func (s *Service) DownloadPackage(ctx context.Context, clientID int64, pkgID string) (string, int64, error) {
	if s.clientLogDeps == nil {
		return "", 0, fmt.Errorf("client log packaging not configured")
	}
	pkg, err := s.clientLogDeps.PkgRepo.GetByID(ctx, pkgID)
	if err != nil {
		return "", 0, err
	}
	if pkg.ClientID != clientID {
		return "", 0, fmt.Errorf("package not found")
	}
	if pkg.Status != "ready" {
		return "", 0, fmt.Errorf("package is not ready (status: %s)", pkg.Status)
	}
	if time.Now().After(pkg.ExpiresAt) {
		// Mark as expired
		pkg.Status = "expired"
		s.clientLogDeps.PkgRepo.Update(ctx, pkg)
		return "", 0, fmt.Errorf("package has expired")
	}
	return pkg.FilePath, pkg.FileSize, nil
}

// CleanupExpiredPackages removes expired package files and records.
func (s *Service) CleanupExpiredPackages(ctx context.Context) error {
	if s.clientLogDeps == nil {
		return nil
	}
	pkgs, err := s.clientLogDeps.PkgRepo.ListExpired(ctx)
	if err != nil {
		return err
	}
	for _, pkg := range pkgs {
		if pkg.FilePath != "" {
			os.Remove(pkg.FilePath)
		}
		pkg.Status = "expired"
		s.clientLogDeps.PkgRepo.Update(ctx, &pkg)
		log.Printf("logmanage: expired package %s cleaned up", pkg.ID)
	}
	return nil
}

// clientLogCleanupLoop periodically cleans up expired client log packages.
func (s *Service) clientLogCleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		ctx := context.Background()
		if err := s.CleanupExpiredPackages(ctx); err != nil {
			log.Printf("logmanage: cleanup expired packages: %v", err)
		}
	}
}

// ============================================================
// Internal implementation
// ============================================================

// processPackageTask runs in a goroutine to collect, filter, desensitize, and package logs.
func (s *Service) processPackageTask(clientID int64, pkgID string, nodes []entryNodeInfo, hours int) {
	ctx := context.Background()

	pkg, err := s.clientLogDeps.PkgRepo.GetByID(ctx, pkgID)
	if err != nil {
		log.Printf("logmanage: package %s not found: %v", pkgID, err)
		return
	}

	defer func() {
		if r := recover(); r != nil {
			pkg.Status = "failed"
			pkg.Error = fmt.Sprintf("internal error: %v", r)
			s.clientLogDeps.PkgRepo.Update(ctx, pkg)
			log.Printf("logmanage: package %s panicked: %v", pkgID, r)
		}
	}()

	now := time.Now()
	startTime := now.Add(-time.Duration(hours) * time.Hour)

	// Create package directory
	pkgDir := filepath.Join(s.dataDir, "logs", "packages")
	if err := os.MkdirAll(pkgDir, 0750); err != nil {
		pkg.Status = "failed"
		pkg.Error = fmt.Sprintf("create package dir: %v", err)
		s.clientLogDeps.PkgRepo.Update(ctx, pkg)
		return
	}

	pkgPath := filepath.Join(pkgDir, fmt.Sprintf("client_%d_%s.log.gz", clientID, pkgID))
	pkgFile, err := os.Create(pkgPath)
	if err != nil {
		pkg.Status = "failed"
		pkg.Error = fmt.Sprintf("create package file: %v", err)
		s.clientLogDeps.PkgRepo.Update(ctx, pkg)
		return
	}

	gw := gzip.NewWriter(pkgFile)
	dw := newPanelDesensitizingWriter(gw)

	totalLines := 0
	// Note: dw.Close() will close gw (gzip.Writer) internally
	// Collect logs from each entry node
	for _, node := range nodes {
		lines, err := s.collectNodeLogs(ctx, node, startTime, now)
		if err != nil {
			log.Printf("logmanage: collect logs from server %d (%s): %v", node.ServerID, node.LogType, err)
			continue
		}

		// Write node header
		fmt.Fprintf(dw, "=== Server %d | %s | inbound: %s ===\n", node.ServerID, node.LogType, node.Tag)

		for _, line := range lines {
			if isLogInTimeRange(line, startTime, now) {
				fmt.Fprintln(dw, line)
				totalLines++
			}
		}
		fmt.Fprintln(dw, "")
	}

	dw.Close() // closes gzip.Writer internally
	pkgFile.Close()

	// Get file info
	info, err := os.Stat(pkgPath)
	if err != nil {
		pkg.Status = "failed"
		pkg.Error = fmt.Sprintf("stat package file: %v", err)
		s.clientLogDeps.PkgRepo.Update(ctx, pkg)
		os.Remove(pkgPath)
		return
	}

	pkg.Status = "ready"
	pkg.FilePath = pkgPath
	pkg.FileSize = info.Size()
	pkg.Error = ""
	s.clientLogDeps.PkgRepo.Update(ctx, pkg)

	log.Printf("logmanage: package %s ready (%d lines, %d bytes)", pkgID, totalLines, info.Size())
}

// collectNodeLogs reads log lines from a node (local or remote).
func (s *Service) collectNodeLogs(ctx context.Context, node entryNodeInfo, startTime, endTime time.Time) ([]string, error) {
	// Local node (serverID == 1)
	if node.ServerID <= 1 {
		return s.collectLocalNodeLogs(ctx, node.LogType, startTime, endTime)
	}

	// Remote node
	if s.clientLogDeps.NodePackager == nil {
		return nil, fmt.Errorf("node packager not configured")
	}
	lines, _, err := s.clientLogDeps.NodePackager.ReadLogFileOnServer(ctx, node.ServerID, node.LogType, ClientPackageReadLines)
	if err != nil {
		return nil, err
	}
	return lines, nil
}

// collectLocalNodeLogs reads log lines from the local filesystem.
func (s *Service) collectLocalNodeLogs(ctx context.Context, logType string, startTime, endTime time.Time) ([]string, error) {
	// Determine log file path based on log mode
	logMode := s.GetCoreLogMode(ctx, logType)

	var logPath string
	switch logMode {
	case "stdout":
		logPath = s.resolveLogPath(fmt.Sprintf("logs/%s-stdout.log", logType))
	case "off":
		return nil, fmt.Errorf("logging is off for %s", logType)
	default:
		logPath = s.resolveLogPath(fmt.Sprintf("logs/%s.log", logType))
	}

	// Also check for rotated files
	var allLines []string

	// Read current log file
	if lines, err := readLinesFromLocalFile(logPath, ClientPackageReadLines); err == nil {
		allLines = append(allLines, lines...)
	}

	// Read rotated files (e.g., xray.log.20260108)
	dir := filepath.Dir(logPath)
	base := filepath.Base(logPath)
	if entries, err := os.ReadDir(dir); err == nil {
		for _, entry := range entries {
			name := entry.Name()
			if len(name) > len(base) && name[:len(base)] == base && name != base {
				rotatedPath := filepath.Join(dir, name)
				if lines, err := readLinesFromLocalFile(rotatedPath, ClientPackageReadLines); err == nil {
					allLines = append(allLines, lines...)
				}
			}
		}
	}

	if len(allLines) == 0 {
		return nil, fmt.Errorf("no log file found for %s", logType)
	}
	return allLines, nil
}

// getClientEntryNodes resolves all entry nodes associated with a client.
// Flow: Client -> PlanTemplate -> NodeGroups -> NodeGroupMembers -> InboundConfigs -> (ServerID, CoreType)
func (s *Service) getClientEntryNodes(ctx context.Context, client *entity.Client) ([]entryNodeInfo, error) {
	deps := s.clientLogDeps

	// Get accessible nodes for the client's plan template
	accessResult, err := deps.PlanSvc.GetAccessibleNodes(ctx, client.PlanID)
	if err != nil {
		return nil, fmt.Errorf("get accessible nodes: %w", err)
	}

	// Deduplicate by (ServerID, LogType)
	seen := make(map[string]bool)
	var nodes []entryNodeInfo

	for _, inbound := range accessResult.InboundConfigs {
		key := fmt.Sprintf("%d:%s", inbound.ServerID, inbound.CoreType)
		if seen[key] {
			continue
		}
		seen[key] = true

		nodes = append(nodes, entryNodeInfo{
			ServerID: inbound.ServerID,
			LogType:  inbound.CoreType,
			Tag:      inbound.Tag,
		})
	}

	return nodes, nil
}

// ============================================================
// Utility functions
// ============================================================

func generatePackageID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// readLinesFromLocalFile reads up to maxLines from a local file.
func readLinesFromLocalFile(path string, maxLines int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // 1MB max line
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		// Keep only the last maxLines to avoid memory issues
		if len(lines) > maxLines {
			lines = lines[len(lines)-maxLines:]
		}
	}
	return lines, scanner.Err()
}

// isLogInTimeRange checks if a log line's timestamp falls within the given range.
// Lines without a parseable timestamp are included.
func isLogInTimeRange(line string, start, end time.Time) bool {
	// Try common timestamp formats from the beginning of the line
	if len(line) >= 19 {
		// Format: 2026/07/08 16:00:00 ...
		if t, err := time.Parse("2006/01/02 15:04:05", line[:19]); err == nil {
			return !t.Before(start) && !t.After(end)
		}
	}
	if len(line) >= 25 {
		// Format: 2026-07-08T16:00:00Z ...
		if t, err := time.Parse("2006-01-02T15:04:05Z", line[:20]); err == nil {
			return !t.Before(start) && !t.After(end)
		}
	}
	// If we can't parse the timestamp, include the line
	return true
}

// ============================================================
// Log desensitization (panel-side, mirrors agent's desensitizingWriter)
// ============================================================

var (
	panelDesensitizeIPv4   = regexp.MustCompile(`\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}`)
	panelDesensitizeUUID   = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	panelDesensitizeEmail  = regexp.MustCompile(`[\w.-]+@[\w.-]+\.\w+`)
	panelDesensitizePasswd = regexp.MustCompile(`(?i)password["\s:=]+["']?[^"'\s,}]+`)
	panelDesensitizeToken  = regexp.MustCompile(`(?i)token["\s:=]+["']?[^"'\s,}]+`)
)

// panelDesensitizingWriter wraps a *gzip.Writer and applies regex substitutions to each line.
type panelDesensitizingWriter struct {
	w   *gzip.Writer
	buf []byte
}

func newPanelDesensitizingWriter(w *gzip.Writer) *panelDesensitizingWriter {
	return &panelDesensitizingWriter{w: w}
}

func (dw *panelDesensitizingWriter) Write(p []byte) (int, error) {
	dw.buf = append(dw.buf, p...)
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
		desensitized := panelDesensitizeLine(line)
		dw.w.Write([]byte(desensitized + "\n"))
	}
	return len(p), nil
}

func (dw *panelDesensitizingWriter) Close() error {
	if len(dw.buf) > 0 {
		desensitized := panelDesensitizeLine(string(dw.buf))
		dw.w.Write([]byte(desensitized))
		dw.buf = nil
	}
	return dw.w.Close()
}

func panelDesensitizeLine(line string) string {
	line = panelDesensitizeIPv4.ReplaceAllString(line, "***.***.***.***")
	line = panelDesensitizeUUID.ReplaceAllString(line, "********-****-****-****-************")
	line = panelDesensitizeEmail.ReplaceAllString(line, "***@***.***")
	line = panelDesensitizePasswd.ReplaceAllString(line, "password: \"***\"")
	line = panelDesensitizeToken.ReplaceAllString(line, "token: \"***\"")
	return line
}
