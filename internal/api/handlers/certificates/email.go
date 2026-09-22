package certificates

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	certService "corefusion/internal/service/certificate"
)

// EmailHandler handles ACME email/account HTTP requests.
type EmailHandler struct {
	svc *certService.ACMEEmailService
}

// NewEmailHandler creates a new email handler.
func NewEmailHandler(svc *certService.ACMEEmailService) *EmailHandler {
	return &EmailHandler{svc: svc}
}

// List handles GET /api/v1/acme-accounts.
//
// 注意返回里**不含** EAB 的 MAC Key ✗（密钥不外泄 ✓），只给 `has_eab` 标记 ✓。
func (h *EmailHandler) List(c *gin.Context) {
	configs, err := h.svc.List(c.Request.Context())
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	response.OK(c, configs)
}

// Create handles POST /api/v1/acme-accounts.
func (h *EmailHandler) Create(c *gin.Context) {
	var req struct {
		Email     string `json:"email" binding:"required"`
		IsDefault bool   `json:"is_default"`
		// CA + EAB：ZeroSSL / Google Public CA 必填 ✓，存下来后签发时自动带上 ✓
		CAProvider string `json:"ca_provider"`
		EABKeyID   string `json:"eab_key_id"`
		EABMACKey  string `json:"eab_mac_key"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	cfg, err := h.svc.Create(c.Request.Context(), certService.ACMEAccountInput{
		Email:      req.Email,
		IsDefault:  req.IsDefault,
		CAProvider: req.CAProvider,
		EABKeyID:   req.EABKeyID,
		EABMACKey:  req.EABMACKey,
	})
	if err != nil {
		response.InternalError(c, "CREATE_FAILED", err.Error())
		return
	}
	response.Created(c, cfg)
}

// Update handles PUT /api/v1/acme-accounts/:id.
func (h *EmailHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	var req struct {
		Email      string `json:"email" binding:"required"`
		IsDefault  bool   `json:"is_default"`
		CAProvider string `json:"ca_provider"`
		EABKeyID   string `json:"eab_key_id"`
		EABMACKey  string `json:"eab_mac_key"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	cfg, err := h.svc.Update(c.Request.Context(), id, certService.ACMEAccountInput{
		Email:      req.Email,
		IsDefault:  req.IsDefault,
		CAProvider: req.CAProvider,
		EABKeyID:   req.EABKeyID,
		EABMACKey:  req.EABMACKey,
	})
	if err != nil {
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}
	response.OK(c, cfg)
}

// Delete handles DELETE /api/v1/acme-accounts/:id.
func (h *EmailHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		response.InternalError(c, "DELETE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "deleted"})
}

// GetDefault handles GET /api/v1/acme-accounts/default.
func (h *EmailHandler) GetDefault(c *gin.Context) {
	cfg, err := h.svc.GetDefault(c.Request.Context())
	if err != nil {
		response.NotFound(c, "NOT_FOUND", "no default account configured")
		return
	}
	response.OK(c, cfg)
}
