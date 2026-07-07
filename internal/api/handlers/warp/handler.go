package warp

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/service/warp"
)

// Handler handles WARP HTTP requests.
// All endpoints are scoped to a specific node server via the :id path parameter.
type Handler struct {
	svc *warp.Service
}

// NewHandler creates a new WARP handler.
func NewHandler(svc *warp.Service) *Handler {
	return &Handler{svc: svc}
}

// List handles GET /api/v1/node-servers/:id/warp
// Returns all WARP configs for the specified node server.
func (h *Handler) List(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}
	items, err := h.svc.ListByServer(serverID)
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"items": items})
}

// Status handles GET /api/v1/node-servers/:id/warp/status?core_type=xray
// Returns the active WARP registration status for the specified node server.
// If core_type is provided, returns the config matching that core type;
// otherwise returns the latest active config for the server.
func (h *Handler) Status(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}
	coreType := c.Query("core_type")
	status, err := h.svc.GetStatus(serverID, coreType)
	if err != nil {
		response.InternalError(c, "STATUS_FAILED", err.Error())
		return
	}
	response.OK(c, status)
}

// Register handles POST /api/v1/node-servers/:id/warp/register
// Registers a new WARP config for the specified node server.
// Body: { "core_type": "xray", "remark": "" }
func (h *Handler) Register(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}
	var req struct {
		CoreType string `json:"core_type"`
		Remark   string `json:"remark"`
	}
	// Allow empty body — core_type defaults to ""
	c.ShouldBindJSON(&req)
	status, err := h.svc.Register(serverID, req.CoreType)
	if err != nil {
		response.InternalError(c, "REGISTER_FAILED", err.Error())
		return
	}
	response.OK(c, status)
}

// GenerateConfig handles POST /api/v1/node-servers/:id/warp/generate-config
// Generates a WireGuard config file from the stored WARP credentials.
// Body: { "core_type": "xray" }
func (h *Handler) GenerateConfig(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}
	var req struct {
		CoreType string `json:"core_type"`
	}
	c.ShouldBindJSON(&req)
	config, err := h.svc.GenerateConfig(serverID, req.CoreType)
	if err != nil {
		response.InternalError(c, "GENERATE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"config": config})
}

// GetConfig handles GET /api/v1/node-servers/:id/warp/config?core_type=xray
// Returns the stored WireGuard config for the specified node server.
func (h *Handler) GetConfig(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}
	coreType := c.Query("core_type")
	config, _ := h.svc.GetConfig(serverID, coreType)
	response.OK(c, gin.H{"config": config})
}

// Delete handles DELETE /api/v1/node-servers/:id/warp/:warpId
// Deletes a specific WARP config by its ID.
func (h *Handler) Delete(c *gin.Context) {
	warpID, err := strconv.ParseInt(c.Param("warpId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid warp config ID")
		return
	}
	if err := h.svc.Reset(warpID); err != nil {
		response.InternalError(c, "RESET_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "deleted"})
}
