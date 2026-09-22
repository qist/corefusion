package certificates

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	certService "corefusion/internal/service/certificate"
)

// EABHandler handles EAB config HTTP requests.
//
// EAB（外部账号绑定）做成**独立配置** ✓：和"ACME 邮箱""DNS 通道"一样在单独的地方管理 ✓，
// 签发时只选一下关联即可 ✓（不用每次手填 EAB-KID / EAB-HMAC-Key ✗）。
//
// ⚠ 列表/详情里**不含** HMAC-Key ✗（密钥不外泄 ✓），只有 `has_mac_key` 标记 ✓。
type EABHandler struct {
	svc *certService.EABConfigService
}

func NewEABHandler(svc *certService.EABConfigService) *EABHandler {
	return &EABHandler{svc: svc}
}

// List handles GET /api/v1/eab-configs.
func (h *EABHandler) List(c *gin.Context) {
	list, err := h.svc.List(c.Request.Context())
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	response.OK(c, list)
}

// Create handles POST /api/v1/eab-configs.
func (h *EABHandler) Create(c *gin.Context) {
	var req struct {
		Name       string `json:"name" binding:"required"`
		CAProvider string `json:"ca_provider"`
		KeyID      string `json:"eab_key_id"`
		MACKey     string `json:"eab_mac_key"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	cfg, err := h.svc.Create(c.Request.Context(), certService.EABConfigInput{
		Name: req.Name, CAProvider: req.CAProvider, KeyID: req.KeyID, MACKey: req.MACKey,
	})
	if err != nil {
		response.BadRequest(c, "CREATE_FAILED", err.Error())
		return
	}
	response.Created(c, cfg)
}

// Update handles PUT /api/v1/eab-configs/:id.
func (h *EABHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	var req struct {
		Name       string `json:"name"`
		CAProvider string `json:"ca_provider"`
		KeyID      string `json:"eab_key_id"`
		MACKey     string `json:"eab_mac_key"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	cfg, err := h.svc.Update(c.Request.Context(), id, certService.EABConfigInput{
		Name: req.Name, CAProvider: req.CAProvider, KeyID: req.KeyID, MACKey: req.MACKey,
	})
	if err != nil {
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}
	response.OK(c, cfg)
}

// Delete handles DELETE /api/v1/eab-configs/:id.
func (h *EABHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		response.InternalError(c, "DELETE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"deleted": true})
}
