package mihomo

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/model/entity"
	"corefusion/internal/service/mihomo"
)

// Handler handles mihomo config HTTP requests.
type Handler struct {
	svc *mihomo.Service
}

func NewHandler(svc *mihomo.Service) *Handler {
	return &Handler{svc: svc}
}

// List handles GET /api/v1/mihomo/configs.
func (h *Handler) List(c *gin.Context) {
	configs, err := h.svc.List(c.Request.Context())
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	response.OK(c, configs)
}

// GetByID handles GET /api/v1/mihomo/configs/:id.
func (h *Handler) GetByID(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid config id")
		return
	}
	config, data, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "NOT_FOUND", "config not found")
		return
	}
	response.OK(c, gin.H{"config": config, "data": data})
}

// Create handles POST /api/v1/mihomo/configs.
func (h *Handler) Create(c *gin.Context) {
	var input mihomo.CreateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	config, err := h.svc.Create(c.Request.Context(), input)
	if err != nil {
		response.InternalError(c, "CREATE_FAILED", err.Error())
		return
	}
	response.Created(c, config)
}

// Update handles PUT /api/v1/mihomo/configs/:id.
func (h *Handler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid config id")
		return
	}

	var input struct {
		Name        *string           `json:"name"`
		ClientType  *string           `json:"client_type"`
		Format      *string           `json:"format"`
		Description *string           `json:"description"`
		Platform    *string           `json:"platform"`
		Enabled     *bool             `json:"enabled"`
		ConfigJSON  *string           `json:"config_json"`
		PlanIDs     *entity.Int64List `json:"plan_ids"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	config, err := h.svc.GetConfigByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "NOT_FOUND", "config not found")
		return
	}

	if input.Name != nil {
		config.Name = *input.Name
	}
	if input.ClientType != nil {
		config.ClientType = *input.ClientType
	}
	if input.Format != nil {
		config.Format = *input.Format
	}
	if input.Description != nil {
		config.Description = *input.Description
	}
	if input.Enabled != nil {
		config.Enabled = *input.Enabled
	}
	if input.ConfigJSON != nil {
		config.ConfigJSON = *input.ConfigJSON
	}
	if input.Platform != nil {
		config.Platform = *input.Platform
	}
	if input.PlanIDs != nil {
		config.PlanIDs = *input.PlanIDs
	}

	if err := h.svc.UpdateConfig(c.Request.Context(), config); err != nil {
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}

	response.OK(c, config)
}

// Delete handles DELETE /api/v1/mihomo/configs/:id.
func (h *Handler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid config id")
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		response.InternalError(c, "DELETE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "config deleted"})
}

// --- Proxy Group handlers ---

// ListProxyGroups handles GET /api/v1/mihomo/configs/:id/proxy-groups.
func (h *Handler) ListProxyGroups(c *gin.Context) {
	configID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid config id")
		return
	}
	groups, err := h.svc.ListProxyGroups(c.Request.Context(), configID)
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	response.OK(c, groups)
}

// CreateProxyGroup handles POST /api/v1/mihomo/configs/:id/proxy-groups.
func (h *Handler) CreateProxyGroup(c *gin.Context) {
	configID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid config id")
		return
	}
	var input mihomo.CreateProxyGroupInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	group, err := h.svc.CreateProxyGroup(c.Request.Context(), configID, input)
	if err != nil {
		response.InternalError(c, "CREATE_FAILED", err.Error())
		return
	}
	response.Created(c, group)
}

// DeleteProxyGroup handles DELETE /api/v1/mihomo/proxy-groups/:id.
func (h *Handler) DeleteProxyGroup(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	if err := h.svc.DeleteProxyGroup(c.Request.Context(), id); err != nil {
		response.InternalError(c, "DELETE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "proxy group deleted"})
}

// --- Rule handlers ---

// ListRules handles GET /api/v1/mihomo/configs/:id/rules.
func (h *Handler) ListRules(c *gin.Context) {
	configID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid config id")
		return
	}
	rules, err := h.svc.ListRules(c.Request.Context(), configID)
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	response.OK(c, rules)
}

// CreateRule handles POST /api/v1/mihomo/configs/:id/rules.
func (h *Handler) CreateRule(c *gin.Context) {
	configID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid config id")
		return
	}
	var input mihomo.CreateRuleInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	rule, err := h.svc.CreateRule(c.Request.Context(), configID, input)
	if err != nil {
		response.InternalError(c, "CREATE_FAILED", err.Error())
		return
	}
	response.Created(c, rule)
}

// DeleteRule handles DELETE /api/v1/mihomo/rules/:id.
func (h *Handler) DeleteRule(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	if err := h.svc.DeleteRule(c.Request.Context(), id); err != nil {
		response.InternalError(c, "DELETE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "rule deleted"})
}

// --- Rule Provider handlers ---

// ListRuleProviders handles GET /api/v1/mihomo/configs/:id/rule-providers.
func (h *Handler) ListRuleProviders(c *gin.Context) {
	configID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid config id")
		return
	}
	providers, err := h.svc.ListRuleProviders(c.Request.Context(), configID)
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	response.OK(c, providers)
}

// CreateRuleProvider handles POST /api/v1/mihomo/configs/:id/rule-providers.
func (h *Handler) CreateRuleProvider(c *gin.Context) {
	configID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid config id")
		return
	}
	var input mihomo.CreateRuleProviderInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	provider, err := h.svc.CreateRuleProvider(c.Request.Context(), configID, input)
	if err != nil {
		response.InternalError(c, "CREATE_FAILED", err.Error())
		return
	}
	response.Created(c, provider)
}

// DeleteRuleProvider handles DELETE /api/v1/mihomo/rule-providers/:id.
func (h *Handler) DeleteRuleProvider(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	if err := h.svc.DeleteRuleProvider(c.Request.Context(), id); err != nil {
		response.InternalError(c, "DELETE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "rule provider deleted"})
}

// GenerateYAML handles GET /api/v1/mihomo/configs/:id/yaml.
func (h *Handler) GenerateYAML(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid config id")
		return
	}
	yaml, err := h.svc.GenerateYAML(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "GENERATE_FAILED", err.Error())
		return
	}
	c.Data(200, "text/yaml; charset=utf-8", []byte(yaml))
}
