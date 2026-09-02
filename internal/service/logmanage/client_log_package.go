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
	"strings"
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
//   4. 按时间范围过滤，只保留该用户的日志行，脱敏后打包为 gzip
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
	// ReadLogFileOnServerFiltered reads log lines from a remote node with server-side filtering.
	// The agent pre-filters by filterEmails and time range before transmission,
	// so the returned lines are already user-specific — no need for panel-side isUserLogLine.
	ReadLogFileOnServerFiltered(ctx context.Context, serverID int64, logType string, lastLines int, filterEmails []string, startTime, endTime int64) ([]string, string, error)
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

// HasEntryNodes checks if a client has any associated entry nodes for log packaging.
func (s *Service) HasEntryNodes(ctx context.Context, clientID int64) bool {
	if s.clientLogDeps == nil {
		return false
	}
	client, err := s.clientLogDeps.ClientRepo.GetByID(ctx, clientID)
	if err != nil {
		return false
	}
	nodes, err := s.getClientEntryNodes(ctx, client)
	if err != nil {
		return false
	}
	return len(nodes) > 0
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

	// Build set of inbound emails for filtering: {email}_{tagSuffix}
	inboundEmails := buildInboundEmails(client.Email, nodes)

	// Process asynchronously
	go s.processPackageTask(clientID, pkgID, client.Email, inboundEmails, nodes, hours)

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
// Expired records are excluded from the results.
func (s *Service) ListClientPackages(ctx context.Context, clientID int64, limit, offset int) ([]entity.ClientLogPackage, int64, error) {
	if s.clientLogDeps == nil {
		return nil, 0, fmt.Errorf("client log packaging not configured")
	}
	return s.clientLogDeps.PkgRepo.ListActiveByClientID(ctx, clientID, limit, offset)
}

// ListAllPackages returns all log packages for admin view.
func (s *Service) ListAllPackages(ctx context.Context, limit, offset int) ([]entity.ClientLogPackage, int64, error) {
	if s.clientLogDeps == nil {
		return nil, 0, fmt.Errorf("client log packaging not configured")
	}
	return s.clientLogDeps.PkgRepo.ListAll(ctx, limit, offset)
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

// DeletePackage deletes a log package record and its file for a client.
func (s *Service) DeletePackage(ctx context.Context, clientID int64, pkgID string) error {
	if s.clientLogDeps == nil {
		return fmt.Errorf("client log packaging not configured")
	}
	pkg, err := s.clientLogDeps.PkgRepo.GetByID(ctx, pkgID)
	if err != nil {
		return fmt.Errorf("package not found")
	}
	if pkg.ClientID != clientID {
		return fmt.Errorf("package not found")
	}
	// Remove file if exists
	if pkg.FilePath != "" {
		os.Remove(pkg.FilePath)
	}
	return s.clientLogDeps.PkgRepo.Delete(ctx, pkgID)
}

// CleanupExpiredPackages removes expired package files and records.
func (s *Service) CleanupExpiredPackages(ctx context.Context) error {
	if s.clientLogDeps == nil {
		return nil
	}
	pkgs, err := s.clientLogDeps.PkgRepo.ListExpired(ctx, 500)
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

// buildInboundEmails generates all possible inbound email identifiers for a client.
// Xray uses {email}_{tagSuffix} format, e.g. "user@test.com_vless29660".
// These are used to filter log lines — only lines containing one of these emails are included.
func buildInboundEmails(clientEmail string, nodes []entryNodeInfo) map[string]bool {
	emails := make(map[string]bool)
	for _, node := range nodes {
		ie := generateInboundEmail(clientEmail, node.Tag)
		emails[ie] = true
	}
	return emails
}

// generateInboundEmail creates a unique email per inbound.
// Mirrors the logic in client/service.go generateInboundEmail.
func generateInboundEmail(baseEmail, tag string) string {
	parts := strings.Split(tag, "-")
	if len(parts) >= 3 {
		return fmt.Sprintf("%s_%s%s", baseEmail, parts[1], parts[len(parts)-1])
	}
	return fmt.Sprintf("%s_%s", baseEmail, tag)
}

// isUserLogLine checks if a log line belongs to the requesting user.
// It looks for any of the user's inboundEmails in the line.
// Lines without any email: field (e.g. API calls, system messages) are excluded.
func isUserLogLine(line string, inboundEmails map[string]bool) bool {
	for email := range inboundEmails {
		if strings.Contains(line, email) {
			return true
		}
	}
	return false
}

// processPackageTask runs in a goroutine to collect, filter, desensitize, and package logs.
//
// For local nodes: read all lines, then filter by inboundEmails + time range on the panel side.
// For remote nodes: send filterEmails + time range to the agent so it pre-filters on the node,
// returning only the user's log lines — this avoids transferring entire log files over the network.
func (s *Service) processPackageTask(clientID int64, pkgID string, clientEmail string, inboundEmails map[string]bool, nodes []entryNodeInfo, hours int) {
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
	dw := newPanelDesensitizingWriter(gw, clientEmail)

	totalLines := 0

	// Collect logs per unique (ServerID, LogType) to avoid reading the same log file multiple times.
	// Multiple inbound tags on the same node/core are handled by the inboundEmails filter.
	readNodes := make(map[string]bool) // "ServerID:LogType" → already read

	// Build filterEmails slice for remote node calls
	filterEmails := make([]string, 0, len(inboundEmails))
	for e := range inboundEmails {
		filterEmails = append(filterEmails, e)
	}

	for _, node := range nodes {
		readKey := fmt.Sprintf("%d:%s", node.ServerID, node.LogType)
		if readNodes[readKey] {
			continue // already read this node's logs
		}
		readNodes[readKey] = true

		isLocalNode := node.ServerID <= 1
		lines, err := s.collectNodeLogs(ctx, node, startTime, now, filterEmails)
		if err != nil {
			log.Printf("logmanage: collect logs from server %d (%s): %v", node.ServerID, node.LogType, err)
			continue
		}

		// Write node header
		fmt.Fprintf(dw, "=== Node (server %d, %s) ===\n", node.ServerID, node.LogType)

		for _, line := range lines {
			// For local nodes: we read all lines and need to filter on the panel side.
			// For remote nodes: the agent already filtered by filterEmails + time range,
			// so we skip the isUserLogLine check but still apply time range as a safety net.
			if isLocalNode {
				if !isLogInTimeRange(line, startTime, now) {
					continue
				}
				if !isUserLogLine(line, inboundEmails) {
					continue
				}
			}
			fmt.Fprintln(dw, line)
			totalLines++
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
// For remote nodes, filterEmails and the time range are sent to the agent so it
// pre-filters on the node side, returning only matching lines.
func (s *Service) collectNodeLogs(ctx context.Context, node entryNodeInfo, startTime, endTime time.Time, filterEmails []string) ([]string, error) {
	// Local node (serverID == 1)
	if node.ServerID <= 1 {
		return s.collectLocalNodeLogs(ctx, node.LogType, startTime, endTime)
	}

	// Remote node: use server-side filtering to avoid transferring entire log files
	if s.clientLogDeps.NodePackager == nil {
		return nil, fmt.Errorf("node packager not configured")
	}
	lines, _, err := s.clientLogDeps.NodePackager.ReadLogFileOnServerFiltered(
		ctx, node.ServerID, node.LogType, ClientPackageReadLines,
		filterEmails,
		startTime.Unix(),
		endTime.Unix(),
	)
	if err != nil {
		return nil, err
	}
	// Safety: remote node may return nil lines when no data matches
	if lines == nil {
		lines = []string{}
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
// Flow: Client -> PlanTemplate -> NodeGroups -> NodeGroupMembers -> InboundConfigs -> (ServerID, CoreType, Tag)
func (s *Service) getClientEntryNodes(ctx context.Context, client *entity.Client) ([]entryNodeInfo, error) {
	deps := s.clientLogDeps

	// Get accessible nodes for the client's plan template
	accessResult, err := deps.PlanSvc.GetAccessibleNodes(ctx, client.PlanID)
	if err != nil {
		return nil, fmt.Errorf("get accessible nodes: %w", err)
	}

	// Deduplicate by (ServerID, Tag) — each unique tag produces different inboundEmail
	seen := make(map[string]bool)
	var nodes []entryNodeInfo

	for _, inbound := range accessResult.InboundConfigs {
		key := fmt.Sprintf("%d:%s", inbound.ServerID, inbound.Tag)
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
// Log desensitization (panel-side, for client log packages)
// ============================================================
//
// 设计原则：
//   - 用户自己的邮箱保留（处理争议用）
//   - 去掉 email 后面的 inbound tag 后缀（如 _vless29660）
//   - 源 IP 脱敏
//   - 源端口脱敏
//   - inbound/routing 标签脱敏（[inbound-vless-29660 -> warp-13hc] → [inbound -> outbound]）
//   - UUID、其他用户邮箱、密码、token 脱敏

var (
	panelDesensitizeIPv4   = regexp.MustCompile(`\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}`)
	panelDesensitizeUUID   = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	panelDesensitizeEmail  = regexp.MustCompile(`[\w.-]+@[\w.-]+\.\w+`)
	panelDesensitizePasswd = regexp.MustCompile(`(?i)password["\s:=]+["']?[^"'\s,}]+`)
	panelDesensitizeToken  = regexp.MustCompile(`(?i)token["\s:=]+["']?[^"'\s,}]+`)

	// Match source port after IP: e.g. ":37859" in "from ***.***.***.***:37859"
	panelDesensitizeSrcPort = regexp.MustCompile(`(:\d{1,5})\s`)

	// Match inbound/routing tags: [inbound-vless-29660 -> warp-13hc]
	// Captures the entire bracket content for replacement
	panelDesensitizeRoutingTag = regexp.MustCompile(`\[inbound[^\]]*->[^\]]*\]`)

	// Match email tag suffix: _vless29660, _vmess12345 etc at end of line or after email
	panelDesensitizeEmailTagSuffix = regexp.MustCompile(`_[a-z]+\d+`)
)

// panelDesensitizingWriter wraps a *gzip.Writer and applies regex substitutions to each line.
type panelDesensitizingWriter struct {
	w           *gzip.Writer
	buf         []byte
	clientEmail string // preserved as-is in desensitized output
}

func newPanelDesensitizingWriter(w *gzip.Writer, clientEmail string) *panelDesensitizingWriter {
	return &panelDesensitizingWriter{w: w, clientEmail: clientEmail}
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
		desensitized := panelDesensitizeLine(line, dw.clientEmail)
		dw.w.Write([]byte(desensitized + "\n"))
	}
	return len(p), nil
}

func (dw *panelDesensitizingWriter) Close() error {
	if len(dw.buf) > 0 {
		desensitized := panelDesensitizeLine(string(dw.buf), dw.clientEmail)
		dw.w.Write([]byte(desensitized))
		dw.buf = nil
	}
	return dw.w.Close()
}

// panelDesensitizeLine applies desensitization rules to a single log line.
//
// Example input:
//
//	2026/07/10 18:49:08.353166 from 172.18.160.1:37859 accepted tcp:www.google.com:443 [inbound-vless-29660 -> warp-13hc] email: newclient@test.com_vless29660
//
// Example output (clientEmail = "newclient@test.com"):
//
//	2026/07/10 18:49:08.353166 from ***.***.***.***:***** accepted tcp:www.google.com:443 [inbound -> outbound] email: newclient@test.com
func panelDesensitizeLine(line string, clientEmail string) string {
	// 1. Desensitize source IP
	line = panelDesensitizeIPv4.ReplaceAllString(line, "***.***.***.***")

	// 2. Desensitize source port (IP:port pattern already masked IP, now mask port)
	// After IP masking: "from ***.***.***.***:37859 " → "from ***.***.***.***:***** "
	line = panelDesensitizeSrcPort.ReplaceAllStringFunc(line, func(match string) string {
		return ":***** "
	})

	// 3. Desensitize inbound/routing tags: [inbound-vless-29660 -> warp-13hc] → [inbound -> outbound]
	line = panelDesensitizeRoutingTag.ReplaceAllString(line, "[inbound -> outbound]")

	// 4. Handle email field: keep client's own email, remove tag suffix, mask other emails
	// The email in Xray logs is like "user@test.com_vless29660"
	// We want to keep the base email but strip the _tagSuffix
	if clientEmail != "" {
		// First, strip the _tagSuffix from the client's own inbound email occurrences
		// e.g. "newclient@test.com_vless29660" → "newclient@test.com"
		line = panelDesensitizeEmailTagSuffix.ReplaceAllStringFunc(line, func(match string) string {
			// Check if this suffix follows the client email
			idx := strings.Index(line, clientEmail+match)
			if idx >= 0 {
				return "" // remove suffix, keep just the email
			}
			return match // keep other suffixes for now
		})
	}

	// 5. Mask other users' emails (not the client's own)
	// We need to mask any email that isn't the client's own
	line = panelDesensitizeEmail.ReplaceAllStringFunc(line, func(match string) string {
		if match == clientEmail {
			return match // keep own email
		}
		return "***@***.***"
	})

	// 6. Desensitize UUIDs
	line = panelDesensitizeUUID.ReplaceAllString(line, "********-****-****-****-************")

	// 7. Desensitize passwords and tokens
	line = panelDesensitizePasswd.ReplaceAllString(line, "password: \"***\"")
	line = panelDesensitizeToken.ReplaceAllString(line, "token: \"***\"")

	return line
}
