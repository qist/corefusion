package services

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/runtime/process"
	"corefusion/internal/store/repository"
)

// ProcessHandler handles service process management requests.
type ProcessHandler struct {
	processMgr *process.Manager
	svcRepo    *repository.ServiceRepository
	crashRepo  *repository.CrashLogRepository
	dataDir    string
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

// Start handles POST /api/v1/services/:id/start.
func (h *ProcessHandler) Start(c *gin.Context) {
	serviceID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid service id")
		return
	}

	svc, err := h.svcRepo.GetByID(c.Request.Context(), serviceID)
	if err != nil {
		response.NotFound(c, "NOT_FOUND", "service not found")
		return
	}

	if err := h.processMgr.Start(c.Request.Context(), serviceID, svc.CoreType); err != nil {
		response.InternalError(c, "START_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"service_id": serviceID, "status": "running"})
}

// Stop handles POST /api/v1/services/:id/stop.
func (h *ProcessHandler) Stop(c *gin.Context) {
	serviceID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid service id")
		return
	}

	if err := h.processMgr.Stop(c.Request.Context(), serviceID); err != nil {
		response.InternalError(c, "STOP_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"service_id": serviceID, "status": "stopped"})
}

// Restart handles POST /api/v1/services/:id/restart.
func (h *ProcessHandler) Restart(c *gin.Context) {
	serviceID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid service id")
		return
	}

	svc, err := h.svcRepo.GetByID(c.Request.Context(), serviceID)
	if err != nil {
		response.NotFound(c, "NOT_FOUND", "service not found")
		return
	}

	if err := h.processMgr.Restart(c.Request.Context(), serviceID, svc.CoreType); err != nil {
		response.InternalError(c, "RESTART_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"service_id": serviceID, "status": "running"})
}

// GetProcessStatus handles GET /api/v1/services/:id/process-status.
func (h *ProcessHandler) GetProcessStatus(c *gin.Context) {
	serviceID, err := strconv.ParseInt(c.Param("id"), 10, 64)
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

// ListAllProcessStatuses handles GET /api/v1/services/process-status.
func (h *ProcessHandler) ListAllProcessStatuses(c *gin.Context) {
	processes := h.processMgr.ListAll()

	// Build a map for quick lookup
	statusMap := make(map[int64]process.ProcessInfo)
	for _, p := range processes {
		statusMap[p.ServiceID] = p
	}

	// Get all services
	services, _, err := h.svcRepo.List(c.Request.Context(), "", 1000, 0)
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}

	// Load all guardians
	guardians, _ := h.processMgr.ListGuardians(c.Request.Context())
	guardianMap := make(map[int64]bool)
	guardianAutoStart := make(map[int64]bool)
	guardianAutoRestart := make(map[int64]bool)
	guardianMaxRestarts := make(map[int64]int)
	for _, g := range guardians {
		guardianMap[g.ServiceID] = true
		guardianAutoStart[g.ServiceID] = g.AutoStart
		guardianAutoRestart[g.ServiceID] = g.AutoRestart
		guardianMaxRestarts[g.ServiceID] = g.MaxRestartCount
	}

	// Merge service info with process status
	var result []gin.H
	for _, svc := range services {
		// Check binary installation
		binaryInstalled := false
		binaryVersion := ""
		if _, ver, err := h.processMgr.ResolveBinary(svc.CoreType); err == nil {
			binaryInstalled = true
			binaryVersion = ver
		}

		item := gin.H{
			"service_id":         svc.ID,
			"name":               svc.Name,
			"core_type":          svc.CoreType,
			"protocol":           svc.Protocol,
			"listen_host":        svc.ListenHost,
			"listen_port":        svc.ListenPort,
			"state":              "stopped",
			"pid":                0,
			"uptime":             0,
			"cpu":                0.0,
			"mem":                0,
			"binary_installed":   binaryInstalled,
			"version":            binaryVersion,
			"auto_start":         guardianAutoStart[svc.ID],
			"auto_restart":       guardianAutoRestart[svc.ID],
			"max_restart_count":  guardianMaxRestarts[svc.ID],
		}

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

		result = append(result, item)
	}

	response.OK(c, result)
}

// UpdateGuardian handles PUT /api/v1/services/:id/guardian.
func (h *ProcessHandler) UpdateGuardian(c *gin.Context) {
	serviceID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid service id")
		return
	}

	var req struct {
		AutoStart     *bool `json:"auto_start"`
		AutoRestart   *bool `json:"auto_restart"`
		MaxRestarts   int   `json:"max_restart_count"`
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

// GetCrashLogs handles GET /api/v1/services/:id/crash-logs.
func (h *ProcessHandler) GetCrashLogs(c *gin.Context) {
	serviceID, err := strconv.ParseInt(c.Param("id"), 10, 64)
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

// ClearCrashLogs handles DELETE /api/v1/services/:id/crash-logs.
func (h *ProcessHandler) ClearCrashLogs(c *gin.Context) {
	serviceID, err := strconv.ParseInt(c.Param("id"), 10, 64)
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

// GetCrashLog handles GET /api/v1/services/:id/crash-logs/:logId.
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
