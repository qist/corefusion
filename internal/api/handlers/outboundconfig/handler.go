package outboundconfig

import (
	"encoding/json"
	"strconv"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/service/outboundconfig"
)

// Handler handles outbound config HTTP requests.
type Handler struct {
	svc *outboundconfig.Service
}

// NewHandler creates a new outbound config handler.
func NewHandler(svc *outboundconfig.Service) *Handler {
	return &Handler{svc: svc}
}

// List handles GET /api/v1/outbound-configs.
func (h *Handler) List(c *gin.Context) {
	coreType := c.Query("core_type")
	configs, err := h.svc.List(c.Request.Context(), coreType)
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	response.OK(c, configs)
}

// Get handles GET /api/v1/outbound-configs/:id.
func (h *Handler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	cfg, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "NOT_FOUND", err.Error())
		return
	}
	response.OK(c, cfg)
}

// Create handles POST /api/v1/outbound-configs.
func (h *Handler) Create(c *gin.Context) {
	var req struct {
		CoreType   string          `json:"core_type"`
		Remark     string          `json:"remark"`
		Protocol   string          `json:"protocol"`
		Tag        string          `json:"tag"`
		ConfigJSON json.RawMessage `json:"config_json"`
		Enabled    bool            `json:"enabled"`
		IsDefault  bool            `json:"is_default"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	cfg, err := h.svc.Create(c.Request.Context(), req.CoreType, req.Remark, req.Protocol, req.Tag, string(req.ConfigJSON), req.Enabled, req.IsDefault)
	if err != nil {
		response.InternalError(c, "CREATE_FAILED", err.Error())
		return
	}
	response.OK(c, cfg)
}

// Update handles PUT /api/v1/outbound-configs/:id.
func (h *Handler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	var req struct {
		Remark     string          `json:"remark"`
		Protocol   string          `json:"protocol"`
		Tag        string          `json:"tag"`
		ConfigJSON json.RawMessage `json:"config_json"`
		Enabled    bool            `json:"enabled"`
		IsDefault  bool            `json:"is_default"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	cfg, err := h.svc.Update(c.Request.Context(), id, req.Remark, req.Protocol, req.Tag, string(req.ConfigJSON), req.Enabled, req.IsDefault)
	if err != nil {
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}
	response.OK(c, cfg)
}

// Delete handles DELETE /api/v1/outbound-configs/:id.
func (h *Handler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		response.InternalError(c, "DELETE_FAILED", err.Error())
		return
	}
	response.OK(c, nil)
}
