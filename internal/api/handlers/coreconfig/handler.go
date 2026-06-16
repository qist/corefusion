package coreconfig

import (
	"encoding/json"
	"io"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/service/coreconfig"
)

// Handler handles core config HTTP requests.
type Handler struct {
	svc *coreconfig.Service
}

// NewHandler creates a new core config handler.
func NewHandler(svc *coreconfig.Service) *Handler {
	return &Handler{svc: svc}
}

// List handles GET /api/v1/protocols.
// Returns all core types with their section list.
func (h *Handler) List(c *gin.Context) {
	result, err := h.svc.List(c.Request.Context())
	if err != nil {
		response.InternalError(c, "LIST_PROTOCOLS_FAILED", err.Error())
		return
	}
	response.OK(c, result)
}

// Get handles GET /api/v1/protocols/:coreType.
// Returns all sections merged into one JSON object.
func (h *Handler) Get(c *gin.Context) {
	coreType := c.Param("coreType")
	if !isValidCoreType(coreType) {
		response.BadRequest(c, "INVALID_CORE_TYPE", "unsupported core type: "+coreType)
		return
	}

	full, err := h.svc.GetFull(c.Request.Context(), coreType)
	if err != nil {
		response.NotFound(c, "PROTOCOL_NOT_FOUND", err.Error())
		return
	}
	response.OK(c, full)
}

// Save handles PUT /api/v1/protocols/:coreType.
// Replaces all sections from a merged JSON object.
func (h *Handler) Save(c *gin.Context) {
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

	if err := h.svc.SaveFull(c.Request.Context(), coreType, body); err != nil {
		response.InternalError(c, "SAVE_PROTOCOL_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "saved"})
}

// GetSection handles GET /api/v1/protocols/:coreType/section/:section.
func (h *Handler) GetSection(c *gin.Context) {
	coreType := c.Param("coreType")
	section := c.Param("section")
	if !isValidCoreType(coreType) {
		response.BadRequest(c, "INVALID_CORE_TYPE", "unsupported core type: "+coreType)
		return
	}

	val, err := h.svc.GetSection(c.Request.Context(), coreType, section)
	if err != nil {
		response.NotFound(c, "SECTION_NOT_FOUND", err.Error())
		return
	}
	response.OK(c, val)
}

// SaveSection handles PUT /api/v1/protocols/:coreType/section/:section.
func (h *Handler) SaveSection(c *gin.Context) {
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

	if err := h.svc.SaveSection(c.Request.Context(), coreType, section, body); err != nil {
		response.InternalError(c, "SAVE_SECTION_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "saved"})
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

	if err := h.svc.Validate(body.ConfigJSON); err != nil {
		response.BadRequest(c, "VALIDATION_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"valid": true})
}

func isValidCoreType(ct string) bool {
	for _, t := range coreconfig.SupportedCoreTypes {
		if t == ct {
			return true
		}
	}
	return false
}
