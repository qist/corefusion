package services

import (
	"context"
	"log"
	"strconv"
	"sync"
	"time"

	"corefusion/internal/api/response"
	"corefusion/internal/model/entity"
	"corefusion/internal/runtime/process"
	"corefusion/internal/store/repository"

	"github.com/gin-gonic/gin"
)

// NodeOperator defines the interface for controlling cores on remote nodes.
type NodeOperator interface {
	IsLocalServer(ctx context.Context, serverID int64) bool
	ControlCore(ctx context.Context, serverID int64, coreType, action string) error
	GetRemoteState(ctx context.Context, serverID int64) (coreStates map[string]*CoreStateInfo, binaries []*BinaryInfoItem, err error)
	// PushCoreGuardians 立即把面板上的守护设置下发给远端 Agent（见 UpdateGuardian）。
	PushCoreGuardians(ctx context.Context, serverID int64) error
}

// CoreStateInfo represents a core's state on a remote node.
type CoreStateInfo struct {
	State         string
	PID           int32
	UptimeSeconds int64
	CpuUsage      float64
	MemUsageBytes int64
}

// BinaryInfoItem represents an installed binary on a remote node.
type BinaryInfoItem struct {
	CoreType   string
	Version    string
	BinaryPath string
	Installed  bool
}

// ProcessHandler handles service process management requests.
type ProcessHandler struct {
	processMgr   *process.Manager
	svcRepo      *repository.ServiceRepository
	crashRepo    *repository.CrashLogRepository
	dataDir      string
	nodeOperator NodeOperator
	panelMode    func() string // 返回 "full" / "management_only"（实时读，nil 视为 full）
	// nodeServerRepo 供看板统计「核心服务」状态时枚举所有节点。
	nodeServerRepo *repository.NodeServerRepository
}

// NewProcessHandler creates a new process handler.
func NewProcessHandler(processMgr *process.Manager, svcRepo *repository.ServiceRepository, crashRepo *repository.CrashLogRepository, dataDir string) *ProcessHandler {
	return &ProcessHandler{
		processMgr: processMgr,
		svcRepo:    svcRepo,
		crashRepo:  crashRepo,
		dataDir:    dataDir,
	}
}

// SetNodeOperator sets the node operator for remote core control.
func (h *ProcessHandler) SetNodeOperator(op NodeOperator) {
	h.nodeOperator = op
}

// SetNodeServerRepo 注入节点服务器仓储（看板统计核心服务状态时枚举节点用）。
func (h *ProcessHandler) SetNodeServerRepo(repo *repository.NodeServerRepository) {
	h.nodeServerRepo = repo
}

// serverCoreCounts 是单台节点的核心服务统计结果。
type serverCoreCounts struct {
	total   int64
	running int64
	stopped int64
	crashed int64
	unknown int64
}

// CoreAbnormalService 是一条「非运行中」核心的明细，供看板 tooltip / 告警使用。
type CoreAbnormalService struct {
	ServerID   int64  `json:"server_id"`
	ServerName string `json:"server_name"`
	CoreType   string `json:"core_type"`
	State      string `json:"state"` // crashed / stopped / unknown
}

type serverCoreResult struct {
	counts   serverCoreCounts
	abnormal []CoreAbnormalService
}

// CoreServiceStatusCounts 统计所有节点上「已安装核心」的实时状态。
//
// 口径：
//   - 未安装的核心不计入总数 —— 只跑一个核心的节点，另一个不该显示成「停止」；
//   - 本机节点读进程表 + 二进制解析器，远端节点读 Agent 心跳缓存里的核心状态
//     与已安装二进制列表（与「节点详情 → 核心进程」页同源）；
//   - 取不到节点状态（Agent 未连接 / 尚未上报）→ 计为「未上报」；
//   - state=crashed → 计为「异常」，其余非 running → 「已停止」。
func (h *ProcessHandler) CoreServiceStatusCounts(ctx context.Context) (total, running, stopped, crashed, unknown int64) {
	if h.nodeServerRepo == nil || h.svcRepo == nil || h.processMgr == nil {
		return
	}
	servers, err := h.nodeServerRepo.ListAll(ctx)
	if err != nil {
		return
	}

	// 并发取每台节点的状态：远端读 Agent 缓存（冷启动可能要等一次 stream），
	// 串行会让看板接口在节点多时等上几十秒。
	results := make([]serverCoreResult, len(servers))
	var wg sync.WaitGroup
	for i, srv := range servers {
		svcs, _, lerr := h.svcRepo.List(ctx, srv.ID, "", 1000, 0)
		if lerr != nil || len(svcs) == 0 {
			continue
		}
		wg.Add(1)
		go func(idx int, srv entity.NodeServer, svcs []entity.InboundService) {
			defer wg.Done()
			counts, abnormal := h.collectServerCores(ctx, srv, svcs)
			results[idx] = serverCoreResult{counts: counts, abnormal: abnormal}
		}(i, srv, svcs)
	}
	wg.Wait()

	for _, r := range results {
		total += r.counts.total
		running += r.counts.running
		stopped += r.counts.stopped
		crashed += r.counts.crashed
		unknown += r.counts.unknown
	}
	return
}

// CoreServiceAbnormalServices 返回所有「已安装但未运行」的核心明细（节点名 + 核心 + 状态）。
// 看板「异常 N」悬浮时据此列出具体节点，避免只给一个数字。
func (h *ProcessHandler) CoreServiceAbnormalServices(ctx context.Context) []gin.H {
	// 不依赖 processMgr：远端节点走 Agent 缓存即可（单元测试也用它验证明细）。
	if h.nodeServerRepo == nil || h.svcRepo == nil {
		return nil
	}
	servers, err := h.nodeServerRepo.ListAll(ctx)
	if err != nil {
		return nil
	}
	results := make([]serverCoreResult, len(servers))
	var wg sync.WaitGroup
	for i, srv := range servers {
		svcs, _, lerr := h.svcRepo.List(ctx, srv.ID, "", 1000, 0)
		if lerr != nil || len(svcs) == 0 {
			continue
		}
		wg.Add(1)
		go func(idx int, srv entity.NodeServer, svcs []entity.InboundService) {
			defer wg.Done()
			counts, abnormal := h.collectServerCores(ctx, srv, svcs)
			results[idx] = serverCoreResult{counts: counts, abnormal: abnormal}
		}(i, srv, svcs)
	}
	wg.Wait()

	out := make([]gin.H, 0)
	for _, r := range results {
		for _, a := range r.abnormal {
			out = append(out, gin.H{
				"server_id":   a.ServerID,
				"server_name": a.ServerName,
				"core_type":   a.CoreType,
				"state":       a.State,
			})
		}
	}
	return out
}

// collectServerCores 统计单台节点上的核心状态，并收集「非运行中」的明细（调用方负责并发）。
func (h *ProcessHandler) collectServerCores(ctx context.Context, srv entity.NodeServer, svcs []entity.InboundService) (out serverCoreCounts, abnormal []CoreAbnormalService) {
	name := srv.Name
	if name == "" {
		name = srv.Hostname
	}
	addAbnormal := func(coreType, state string) {
		if state == "" {
			state = "stopped"
		}
		abnormal = append(abnormal, CoreAbnormalService{
			ServerID: srv.ID, ServerName: name, CoreType: coreType, State: state,
		})
	}
	count := func(coreType, state string) {
		switch state {
		case "running":
			out.running++
		case "crashed":
			out.crashed++
			addAbnormal(coreType, state)
		default:
			out.stopped++
			addAbnormal(coreType, state)
		}
	}

	if h.nodeOperator != nil && !h.nodeOperator.IsLocalServer(ctx, srv.ID) {
		states, bins, rerr := h.nodeOperator.GetRemoteState(ctx, srv.ID)
		// Agent 未连接 / 尚未上报核心状态：无法判断，统一计为「未上报」。
		if rerr != nil || len(states) == 0 {
			out.total = int64(len(svcs))
			out.unknown = int64(len(svcs))
			for _, svc := range svcs {
				addAbnormal(svc.CoreType, "unknown")
			}
			return
		}
		// 二进制清单可能还没拉到（冷启动）：先按「已安装」计，避免把真实运行的
		// 核心误报成未上报；拿到清单后未安装的核心会自动从总数里消失。
		installed := make(map[string]bool, len(bins))
		for _, b := range bins {
			if b != nil {
				installed[b.CoreType] = b.Installed
			}
		}
		for _, svc := range svcs {
			if bins != nil && !installed[svc.CoreType] {
				continue // 明确未安装：不显示、不计入总数
			}
			out.total++
			state := ""
			if cs, ok := states[svc.CoreType]; ok && cs != nil {
				state = cs.State
			}
			count(svc.CoreType, state)
		}
		return
	}

	// 本机节点：进程表 + 本地二进制解析器
	localBySvc := make(map[int64]process.ProcessInfo)
	for _, p := range h.processMgr.ListAll() {
		localBySvc[p.ServiceID] = p
	}
	for _, svc := range svcs {
		if _, _, berr := h.processMgr.ResolveBinary(svc.CoreType); berr != nil {
			continue // 未安装：不显示、不计入总数
		}
		out.total++
		state := ""
		if p, ok := localBySvc[svc.ID]; ok {
			state = p.State
		}
		count(svc.CoreType, state)
	}
	return
}

// SetPanelModeFunc 注入面板模式读取函数（"full" / "management_only"）。
// 纯管理模式下本地核心的 start/restart 会被拦截（设计 §4.4：面板不运行核心进程）。
func (h *ProcessHandler) SetPanelModeFunc(fn func() string) {
	h.panelMode = fn
}

// localManagementOnly 判断该服务是否属于本地节点、且面板处于纯管理模式。
func (h *ProcessHandler) localManagementOnly(ctx context.Context, serverID int64) bool {
	if h.panelMode == nil || h.panelMode() != "management_only" {
		return false
	}
	if h.nodeOperator == nil {
		return serverID == 1
	}
	return h.nodeOperator.IsLocalServer(ctx, serverID)
}

// Start handles POST /api/v1/node-servers/:id/services/:serviceId/start.
func (h *ProcessHandler) Start(c *gin.Context) {
	serviceID, err := strconv.ParseInt(c.Param("serviceId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid service id")
		return
	}

	svc, err := h.svcRepo.GetByID(c.Request.Context(), serviceID)
	if err != nil {
		response.NotFound(c, "NOT_FOUND", "service not found")
		return
	}

	// Route to remote node via NodeOperator if not local
	if h.nodeOperator != nil && !h.nodeOperator.IsLocalServer(c.Request.Context(), svc.ServerID) {
		// Remote start: synchronous with timeout (should be fast)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := h.nodeOperator.ControlCore(ctx, svc.ServerID, svc.CoreType, "start"); err != nil {
			response.InternalError(c, "START_FAILED", err.Error())
			return
		}
		response.OK(c, gin.H{"service_id": serviceID, "status": "running"})
		return
	}

	// 纯管理模式：本地核心进程由面板托管启动被禁止（远程节点不受影响）
	if h.localManagementOnly(c.Request.Context(), svc.ServerID) {
		response.Forbidden(c, "PANEL_MANAGEMENT_ONLY", "panel is in management-only mode, local core processes are disabled")
		return
	}

	if err := h.processMgr.Start(c.Request.Context(), serviceID, svc.CoreType); err != nil {
		response.InternalError(c, "START_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"service_id": serviceID, "status": "running"})
}

// Stop handles POST /api/v1/node-servers/:id/services/:serviceId/stop.
func (h *ProcessHandler) Stop(c *gin.Context) {
	serviceID, err := strconv.ParseInt(c.Param("serviceId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid service id")
		return
	}

	svc, err := h.svcRepo.GetByID(c.Request.Context(), serviceID)
	if err != nil {
		response.NotFound(c, "NOT_FOUND", "service not found")
		return
	}

	// Route to remote node via NodeOperator if not local
	if h.nodeOperator != nil && !h.nodeOperator.IsLocalServer(c.Request.Context(), svc.ServerID) {
		// Remote stop: async — process shutdown can take 10+ seconds
		// The Agent's process.Manager.Stop is non-blocking, but the gRPC
		// round-trip still waits for the RPC completion. Fire and forget.
		go func(serverID int64, coreType string) {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			if err := h.nodeOperator.ControlCore(ctx, serverID, coreType, "stop"); err != nil {
				log.Printf("[process] async stop %s on server %d failed: %v", coreType, serverID, err)
			} else {
				log.Printf("[process] async stop %s on server %d completed", coreType, serverID)
			}
		}(svc.ServerID, svc.CoreType)
		response.OK(c, gin.H{"service_id": serviceID, "status": "stopping", "async": true})
		return
	}

	if err := h.processMgr.Stop(c.Request.Context(), serviceID); err != nil {
		response.InternalError(c, "STOP_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"service_id": serviceID, "status": "stopped"})
}

// Restart handles POST /api/v1/node-servers/:id/services/:serviceId/restart.
func (h *ProcessHandler) Restart(c *gin.Context) {
	serviceID, err := strconv.ParseInt(c.Param("serviceId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid service id")
		return
	}

	svc, err := h.svcRepo.GetByID(c.Request.Context(), serviceID)
	if err != nil {
		response.NotFound(c, "NOT_FOUND", "service not found")
		return
	}

	// Route to remote node via NodeOperator if not local
	if h.nodeOperator != nil && !h.nodeOperator.IsLocalServer(c.Request.Context(), svc.ServerID) {
		// Remote restart: async — stop+sleep+start can take 10+ seconds
		go func(serverID int64, coreType string) {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			if err := h.nodeOperator.ControlCore(ctx, serverID, coreType, "restart"); err != nil {
				log.Printf("[process] async restart %s on server %d failed: %v", coreType, serverID, err)
			} else {
				log.Printf("[process] async restart %s on server %d completed", coreType, serverID)
			}
		}(svc.ServerID, svc.CoreType)
		response.OK(c, gin.H{"service_id": serviceID, "status": "restarting", "async": true})
		return
	}

	// 纯管理模式：本地核心进程由面板托管重启被禁止（远程节点不受影响）
	if h.localManagementOnly(c.Request.Context(), svc.ServerID) {
		response.Forbidden(c, "PANEL_MANAGEMENT_ONLY", "panel is in management-only mode, local core processes are disabled")
		return
	}

	if err := h.processMgr.Restart(c.Request.Context(), serviceID, svc.CoreType); err != nil {
		response.InternalError(c, "RESTART_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"service_id": serviceID, "status": "running"})
}

// GetProcessStatus handles GET /api/v1/node-servers/:id/services/:serviceId/process-status.
func (h *ProcessHandler) GetProcessStatus(c *gin.Context) {
	serviceID, err := strconv.ParseInt(c.Param("serviceId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid service id")
		return
	}

	state, pid, startedAt, lastErr := h.processMgr.GetStatus(serviceID)
	uptime := h.processMgr.GetUptime(serviceID)
	result := gin.H{
		"service_id": serviceID,
		"state":      state,
		"pid":        pid,
		"started_at": startedAt,
		"uptime":     uptime,
	}
	if lastErr != nil {
		result["last_error"] = lastErr.Error()
	}

	response.OK(c, result)
}

// ListAllProcessStatuses handles GET /api/v1/node-servers/:id/services/process-status.
func (h *ProcessHandler) ListAllProcessStatuses(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		serverID = 1
	}

	isRemote := h.nodeOperator != nil && !h.nodeOperator.IsLocalServer(c.Request.Context(), serverID)

	// For remote nodes, use cached state from Agent instead of local processMgr
	var remoteCoreStates map[string]*CoreStateInfo
	var remoteBinaries []*BinaryInfoItem
	if isRemote {
		remoteCoreStates, remoteBinaries, _ = h.nodeOperator.GetRemoteState(c.Request.Context(), serverID)
	}

	processes := h.processMgr.ListAll()

	// Build a map for quick lookup
	statusMap := make(map[int64]process.ProcessInfo)
	for _, p := range processes {
		statusMap[p.ServiceID] = p
	}

	// Get services for this node server
	services, _, err := h.svcRepo.List(c.Request.Context(), serverID, "", 1000, 0)
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}

	// Ensure all supported core types have a service record.
	// Local node gets this via process.Manager.SeedCores at startup,
	// but remote nodes never get seeded. We auto-create missing records
	// here so both local and remote show all core types consistently.
	supportedCores := []struct {
		Name     string
		CoreType string
	}{
		{"Xray", "xray"},
		{"sing-box", "singbox"},
	}
	existingCoreTypes := make(map[string]bool)
	for _, svc := range services {
		existingCoreTypes[svc.CoreType] = true
	}
	for _, sc := range supportedCores {
		if !existingCoreTypes[sc.CoreType] {
			newSvc := &entity.InboundService{
				ServerID:   serverID,
				Name:       sc.Name,
				CoreType:   sc.CoreType,
				Protocol:   "-",
				ListenHost: "0.0.0.0",
				ListenPort: 0,
			}
			if err := h.svcRepo.Create(c.Request.Context(), newSvc); err != nil {
				log.Printf("[process] failed to seed service %s for server %d: %v", sc.CoreType, serverID, err)
			} else {
				log.Printf("[process] seeded service %s for server %d (id=%d)", sc.CoreType, serverID, newSvc.ID)
				services = append(services, *newSvc)
			}
		}
	}

	// Load all guardians (local only)
	guardians, _ := h.processMgr.ListGuardians(c.Request.Context())
	guardianAutoStart := make(map[int64]bool)
	guardianAutoRestart := make(map[int64]bool)
	guardianMaxRestarts := make(map[int64]int)
	for _, g := range guardians {
		guardianAutoStart[g.ServiceID] = g.AutoStart
		guardianAutoRestart[g.ServiceID] = g.AutoRestart
		guardianMaxRestarts[g.ServiceID] = g.MaxRestartCount
	}

	// Build binary lookup map from remote state or local resolver
	type binaryInfo struct {
		installed bool
		version   string
	}
	binaryMap := make(map[string]*binaryInfo)
	if isRemote && remoteBinaries != nil {
		for _, b := range remoteBinaries {
			binaryMap[b.CoreType] = &binaryInfo{installed: b.Installed, version: b.Version}
		}
	} else {
		// Local: resolve from processMgr
		for _, svc := range services {
			if _, ok := binaryMap[svc.CoreType]; !ok {
				if _, ver, err := h.processMgr.ResolveBinary(svc.CoreType); err == nil {
					binaryMap[svc.CoreType] = &binaryInfo{installed: true, version: ver}
				}
			}
		}
	}

	// Build core state lookup from remote state or local processMgr
	type coreStateInfo struct {
		state  string
		pid    int
		uptime int64
		cpu    float64
		mem    int64
	}
	coreStateMap := make(map[string]*coreStateInfo)
	if isRemote && remoteCoreStates != nil {
		for coreType, cs := range remoteCoreStates {
			coreStateMap[coreType] = &coreStateInfo{
				state:  cs.State,
				pid:    int(cs.PID),
				uptime: cs.UptimeSeconds,
				cpu:    cs.CpuUsage,
				mem:    cs.MemUsageBytes,
			}
		}
	}

	// Merge service info with process status
	var result []gin.H
	for _, svc := range services {
		bi := binaryMap[svc.CoreType]
		binaryInstalled := bi != nil && bi.installed
		binaryVersion := ""
		if bi != nil {
			binaryVersion = bi.version
		}

		// 二进制未安装时核心不可能开机自启/自动重启：对外一律返回关闭，
		// 否则界面会显示「已开启」但实际永远不会拉起（数据库里的历史开关值保持不变，
		// 装好二进制后仍按原设置生效）。
		autoStart := guardianAutoStart[svc.ID]
		autoRestart := guardianAutoRestart[svc.ID]
		if !binaryInstalled {
			autoStart = false
			autoRestart = false
		}

		item := gin.H{
			"service_id":        svc.ID,
			"name":              svc.Name,
			"core_type":         svc.CoreType,
			"protocol":          svc.Protocol,
			"listen_host":       svc.ListenHost,
			"listen_port":       svc.ListenPort,
			"state":             "stopped",
			"pid":               0,
			"uptime":            0,
			"cpu":               0.0,
			"mem":               int64(0),
			"binary_installed":  binaryInstalled,
			"version":           binaryVersion,
			"auto_start":        autoStart,
			"auto_restart":      autoRestart,
			"max_restart_count": guardianMaxRestarts[svc.ID],
		}

		if isRemote {
			// Use remote cached state
			if cs, ok := coreStateMap[svc.CoreType]; ok {
				item["state"] = cs.state
				item["pid"] = cs.pid
				item["uptime"] = cs.uptime
				item["cpu"] = cs.cpu
				item["mem"] = cs.mem
			}
		} else {
			// Use local process manager state
			if p, ok := statusMap[svc.ID]; ok {
				item["state"] = p.State
				item["pid"] = p.PID
				item["uptime"] = p.Uptime
				item["cpu"] = p.CPU
				item["mem"] = p.Mem
				item["started_at"] = p.StartedAt
				if p.LastError != "" {
					item["last_error"] = p.LastError
				}
			}
		}

		result = append(result, item)
	}

	response.OK(c, result)
}

// UpdateGuardian handles PUT /api/v1/node-servers/:id/services/:serviceId/guardian.
func (h *ProcessHandler) UpdateGuardian(c *gin.Context) {
	serviceID, err := strconv.ParseInt(c.Param("serviceId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid service id")
		return
	}

	var req struct {
		AutoStart   *bool `json:"auto_start"`
		AutoRestart *bool `json:"auto_restart"`
		MaxRestarts int   `json:"max_restart_count"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	// Get current config
	g, err := h.processMgr.GetGuardian(c.Request.Context(), serviceID)
	if err != nil {
		// Create default
		g = nil
	}

	autoStart := false
	autoRestart := false
	maxRestarts := 5
	if g != nil {
		autoStart = g.AutoStart
		autoRestart = g.AutoRestart
		maxRestarts = g.MaxRestartCount
	}
	if req.AutoStart != nil {
		autoStart = *req.AutoStart
	}
	if req.AutoRestart != nil {
		autoRestart = *req.AutoRestart
	}
	if req.MaxRestarts > 0 {
		maxRestarts = req.MaxRestarts
	}

	if err := h.processMgr.UpdateGuardian(c.Request.Context(), serviceID, autoStart, autoRestart, maxRestarts); err != nil {
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}

	// 守护设置只存在面板库，Agent 侧**只随 SyncData 更新**。远端节点必须立刻补推，
	// 否则会出现「面板显示已开启、kill 掉却不拉起」，要等下一次完整同步（主备
	// standby-sync / 镜像同步 / Agent 重连）才生效。
	// 异步执行：完整载荷构建+下发较慢，不能拖住这个 UI 请求；本地节点无需下发。
	if h.nodeOperator != nil {
		if sid, perr := strconv.ParseInt(c.Param("id"), 10, 64); perr == nil &&
			!h.nodeOperator.IsLocalServer(c.Request.Context(), sid) {
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
				defer cancel()
				if err := h.nodeOperator.PushCoreGuardians(ctx, sid); err != nil {
					log.Printf("[ProcessHandler] push core guardians to server %d failed: %v", sid, err)
				}
			}()
		}
	}

	response.OK(c, gin.H{
		"service_id":        serviceID,
		"auto_start":        autoStart,
		"auto_restart":      autoRestart,
		"max_restart_count": maxRestarts,
	})
}

// GetCrashLogs handles GET /api/v1/node-servers/:id/services/:serviceId/crash-logs.
func (h *ProcessHandler) GetCrashLogs(c *gin.Context) {
	serviceID, err := strconv.ParseInt(c.Param("serviceId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid service id")
		return
	}

	logs, err := h.crashRepo.ListByServiceID(c.Request.Context(), serviceID, 50)
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}

	response.OK(c, logs)
}

// ClearCrashLogs handles DELETE /api/v1/node-servers/:id/services/:serviceId/crash-logs.
func (h *ProcessHandler) ClearCrashLogs(c *gin.Context) {
	serviceID, err := strconv.ParseInt(c.Param("serviceId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid service id")
		return
	}

	if err := h.crashRepo.DeleteByServiceID(c.Request.Context(), serviceID); err != nil {
		response.InternalError(c, "DELETE_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"message": "crash logs cleared"})
}

// GetCrashLog handles GET /api/v1/node-servers/:id/services/:serviceId/crash-logs/:logId.
func (h *ProcessHandler) GetCrashLog(c *gin.Context) {
	response.NotFound(c, "NOT_FOUND", "crash log not found")
}
