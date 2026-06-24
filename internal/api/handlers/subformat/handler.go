package subformat

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/service/subformat"
)

// Handler handles subscription format HTTP requests.
type Handler struct {
	svc *subformat.Service
}

// NewHandler creates a new subscription format handler.
func NewHandler(svc *subformat.Service) *Handler {
	return &Handler{svc: svc}
}

// List handles GET /api/v1/subscription-formats.
func (h *Handler) List(c *gin.Context) {
	formats, err := h.svc.List(c.Request.Context())
	if err != nil {
		response.InternalError(c, "LIST_FORMATS_FAILED", err.Error())
		return
	}
	response.OK(c, formats)
}

// GetByID handles GET /api/v1/subscription-formats/:id.
func (h *Handler) GetByID(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid format id")
		return
	}

	format, rules, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "FORMAT_NOT_FOUND", "format not found")
		return
	}

	response.OK(c, gin.H{
		"format": format,
		"rules":  rules,
	})
}

// Create handles POST /api/v1/subscription-formats.
func (h *Handler) Create(c *gin.Context) {
	var input subformat.CreateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	format, err := h.svc.Create(c.Request.Context(), input)
	if err != nil {
		response.InternalError(c, "CREATE_FORMAT_FAILED", err.Error())
		return
	}

	response.Created(c, format)
}

// Update handles PUT /api/v1/subscription-formats/:id.
func (h *Handler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid format id")
		return
	}

	var input subformat.UpdateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	format, err := h.svc.Update(c.Request.Context(), id, input)
	if err != nil {
		response.InternalError(c, "UPDATE_FORMAT_FAILED", err.Error())
		return
	}

	response.OK(c, format)
}

// Delete handles DELETE /api/v1/subscription-formats/:id.
func (h *Handler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid format id")
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		response.InternalError(c, "DELETE_FORMAT_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"message": "format deleted"})
}

// --- Rule handlers ---

// ListRules handles GET /api/v1/subscription-formats/:id/rules.
func (h *Handler) ListRules(c *gin.Context) {
	formatID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid format id")
		return
	}

	rules, err := h.svc.ListRules(c.Request.Context(), formatID)
	if err != nil {
		response.InternalError(c, "LIST_RULES_FAILED", err.Error())
		return
	}

	response.OK(c, rules)
}

// CreateRule handles POST /api/v1/subscription-formats/:id/rules.
func (h *Handler) CreateRule(c *gin.Context) {
	formatID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid format id")
		return
	}

	var input subformat.CreateRuleInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	rule, err := h.svc.CreateRule(c.Request.Context(), formatID, input)
	if err != nil {
		response.InternalError(c, "CREATE_RULE_FAILED", err.Error())
		return
	}

	response.Created(c, rule)
}

// UpdateRule handles PUT /api/v1/subscription-formats/:id/rules/:rid.
func (h *Handler) UpdateRule(c *gin.Context) {
	rid, err := strconv.ParseInt(c.Param("rid"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid rule id")
		return
	}

	var input subformat.UpdateRuleInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	rule, err := h.svc.UpdateRule(c.Request.Context(), rid, input)
	if err != nil {
		response.InternalError(c, "UPDATE_RULE_FAILED", err.Error())
		return
	}

	response.OK(c, rule)
}

// DeleteRule handles DELETE /api/v1/subscription-formats/:id/rules/:rid.
func (h *Handler) DeleteRule(c *gin.Context) {
	rid, err := strconv.ParseInt(c.Param("rid"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid rule id")
		return
	}

	if err := h.svc.DeleteRule(c.Request.Context(), rid); err != nil {
		response.InternalError(c, "DELETE_RULE_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"message": "rule deleted"})
}

// SortRules handles POST /api/v1/subscription-formats/:id/rules/sort.
func (h *Handler) SortRules(c *gin.Context) {
	var input subformat.SortRulesInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	if err := h.svc.SortRules(c.Request.Context(), input); err != nil {
		response.InternalError(c, "SORT_RULES_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"message": "rules sorted"})
}
