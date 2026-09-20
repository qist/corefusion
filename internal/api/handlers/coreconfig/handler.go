package coreconfig

import (
	"context"
	"encoding/json"
	"io"
	"strconv"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/service/coreconfig"
)

// ProcessManager provides process restart capabilities.
type ProcessManager interface {
	Restart(ctx context.Context, serviceID int64, coreType string) error
	IsRunning(serviceID int64) bool
}

// Handler handles core config HTTP requests.
type Handler struct {
	svc        *coreconfig.Service
	processMgr ProcessManager
}

// NewHandler creates a new core config handler.
func NewHandler(svc *coreconfig.Service) *Handler {
	return &Handler{svc: svc}
}

// SetProcessManager sets the process manager for auto-restart.
func (h *Handler) SetProcessManager(pm ProcessManager) {
	h.processMgr = pm
}

// parseServerID extracts the server ID from the URL path parameter.
// Falls back to 1 (local node) for legacy callers that don't include :id.
func parseServerID(c *gin.Context) int64 {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return 1
	}
	return id
}

// List handles GET /api/v1/node-servers/:id/protocols.
// Returns all core types with their section list for the specified server.
func (h *Handler) List(c *gin.Context) {
	serverID := parseServerID(c)
	result, err := h.svc.List(c.Request.Context(), serverID)
	if err != nil {
		response.InternalError(c, "LIST_PROTOCOLS_FAILED", err.Error())
		return
	}
	response.OK(c, result)
}

// Get handles GET /api/v1/protocols/:coreType.
// Returns all sections merged into one JSON object.
func (h *Handler) Get(c *gin.Context) {
	serverID := parseServerID(c)
	coreType := c.Param("coreType")
	if !isValidCoreType(coreType) {
		response.BadRequest(c, "INVALID_CORE_TYPE", "unsupported core type: "+coreType)
		return
	}

	full, err := h.svc.GetFull(c.Request.Context(), serverID, coreType)
	if err != nil {
		response.NotFound(c, "PROTOCOL_NOT_FOUND", err.Error())
		return
	}
	response.OK(c, full)
}

// Save handles PUT /api/v1/protocols/:coreType.
// Replaces all sections from a merged JSON object.
func (h *Handler) Save(c *gin.Context) {
	serverID := parseServerID(c)
	coreType := c.Param("coreType")
	if !isValidCoreType(coreType) {
		response.BadRequest(c, "INVALID_CORE_TYPE", "unsupported core type: "+coreType)
		return
	}

	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		response.BadRequest(c, "INVALID_REQUEST", "read body failed")
		return
	}

	stripped := coreconfig.StripJSONC(raw)
	var body map[string]interface{}
	if err := json.Unmarshal(stripped, &body); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	if err := h.svc.SaveFull(c.Request.Context(), serverID, coreType, body); err != nil {
		response.InternalError(c, "SAVE_PROTOCOL_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "saved"})
}

// GetSection handles GET /api/v1/protocols/:coreType/section/:section.
func (h *Handler) GetSection(c *gin.Context) {
	serverID := parseServerID(c)
	coreType := c.Param("coreType")
	section := c.Param("section")
	if !isValidCoreType(coreType) {
		response.BadRequest(c, "INVALID_CORE_TYPE", "unsupported core type: "+coreType)
		return
	}

	val, err := h.svc.GetSection(c.Request.Context(), serverID, coreType, section)
	if err != nil {
		response.NotFound(c, "SECTION_NOT_FOUND", err.Error())
		return
	}
	response.OK(c, val)
}

// SaveSection handles PUT /api/v1/protocols/:coreType/section/:section.
func (h *Handler) SaveSection(c *gin.Context) {
	serverID := parseServerID(c)
	coreType := c.Param("coreType")
	section := c.Param("section")
	if !isValidCoreType(coreType) {
		response.BadRequest(c, "INVALID_CORE_TYPE", "unsupported core type: "+coreType)
		return
	}

	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		response.BadRequest(c, "INVALID_REQUEST", "read body failed")
		return
	}

	// Strip JSONC comments, then validate as JSON
	stripped := coreconfig.StripJSONC(raw)
	var body json.RawMessage
	if err := json.Unmarshal(stripped, &body); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	if err := h.svc.SaveSection(c.Request.Context(), serverID, coreType, section, body); err != nil {
		response.InternalError(c, "SAVE_SECTION_FAILED", err.Error())
		return
	}

	// Xray routing/inbounds/outbounds are applied separately via runtime APIs.
	// Skipping restart here avoids forcing a stopped xray process to restart
	// when the UI only intends to save config changes to the database.
	serviceID := h.getServiceID(coreType)
	if h.processMgr != nil && serviceID > 0 && shouldAutoRestartSection(coreType, section) && h.processMgr.IsRunning(serviceID) {
		h.processMgr.Restart(c.Request.Context(), serviceID, coreType)
	}

	response.OK(c, gin.H{"message": "saved"})
}

// getServiceID returns the service ID for a core type.
func (h *Handler) getServiceID(coreType string) int64 {
	ids := map[string]int64{"xray": 1, "singbox": 2, "hysteria2": 3, "tuic": 4}
	return ids[coreType]
}

func shouldAutoRestartSection(coreType, section string) bool {
	if coreType == "xray" {
		switch section {
		case "routing", "inbounds", "outbounds":
			return false
		}
	}
	return true
}

// Validate handles POST /api/v1/protocols/:coreType/validate.
func (h *Handler) Validate(c *gin.Context) {
	coreType := c.Param("coreType")
	if !isValidCoreType(coreType) {
		response.BadRequest(c, "INVALID_CORE_TYPE", "unsupported core type: "+coreType)
		return
	}

	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		response.BadRequest(c, "INVALID_REQUEST", "read body failed")
		return
	}

	stripped := coreconfig.StripJSONC(raw)
	var body struct {
		ConfigJSON string `json:"config_json"`
	}
	if err := json.Unmarshal(stripped, &body); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	res, err := h.svc.Validate(coreType, body.ConfigJSON)
	if err != nil {
		response.BadRequest(c, "VALIDATION_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{
		"valid":   true,
		"checked": res.Checked,
		"version": res.Version,
		"output":  res.Output,
	})
}

func isValidCoreType(ct string) bool {
	for _, t := range coreconfig.SupportedCoreTypes {
		if t == ct {
			return true
		}
	}
	return false
}
