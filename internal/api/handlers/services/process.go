package services

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strconv"
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
			"auto_start":        guardianAutoStart[svc.ID],
			"auto_restart":      guardianAutoRestart[svc.ID],
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

// ensureConfig ensures a config file exists for a service.
func (h *ProcessHandler) ensureConfig(coreType string, configJSON string) string {
	configDir := filepath.Join(h.dataDir, "runtime")
	os.MkdirAll(configDir, 0750)
	configPath := filepath.Join(configDir, coreType+".jsonc")
	if _, err := os.Stat(configPath); os.IsNotExist(err) && configJSON != "" {
		os.WriteFile(configPath, []byte(configJSON), 0640)
	}
	return configPath
}
