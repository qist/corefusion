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

// FetchZeroSSL handles POST /api/v1/eab-configs/fetch-zerossl.
//
// 用**邮箱**向 ZeroSSL 自动领取一对 EAB 并存成配置 ✓ ——
// 和 acme.sh 的机制完全一样 ✓（同一个接口 ✓），之后 ZeroSSL 也能"按 CA 自动关联" ✓ 零手工签发 ✓。
//
// ⚠ 等价于用该邮箱在 ZeroSSL 取/建账号 ✓（acme.sh 默认行为 ✓），请用你自己的邮箱 ✓。
func (h *EABHandler) FetchZeroSSL(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	cfg, err := h.svc.FetchZeroSSLEAB(c.Request.Context(), req.Email)
	if err != nil {
		response.BadRequest(c, "ZEROSSL_EAB_FAILED", err.Error())
		return
	}
	response.Created(c, cfg)
}

// FetchGoogle handles POST /api/v1/eab-configs/fetch-google.
//
// 用 **GCP 授权**创建一对 Google Public CA 的 EAB 并存下来 ✓ ——
// 这是"Google 自动申请 EAB"的正规做法 ✓：Google **不像 ZeroSSL 能用邮箱换** ✗，
// **必须先有 GCP 授权** ✓（服务账号 JSON ✓ 或临时 access token ✓）。
//
// 前置条件（详见 docs/cert-google-eab.md ✓）：
//  1. 启用 Public CA API：`gcloud services enable publicca.googleapis.com` ✓
//  2. 授权角色：`roles/publicca.externalAccountKeyCreator` ✓
//
// ⚠ 记得官方规则 ✓：EAB **7 天内必须用掉 + 只能注册一个账号** ✗；
// 但注册出的 ACME 账号**不过期** ✓ → 续期不需要 EAB ✓。
func (h *EABHandler) FetchGoogle(c *gin.Context) {
	var req struct {
		ProjectID          string `json:"project_id" binding:"required"`
		ServiceAccountJSON string `json:"service_account_json"`
		AccessToken        string `json:"access_token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if req.ServiceAccountJSON == "" && req.AccessToken == "" {
		response.BadRequest(c, "GOOGLE_AUTH_REQUIRED",
			"provide either service_account_json (recommended) or access_token")
		return
	}
	cfg, err := h.svc.FetchGoogleEAB(c.Request.Context(), req.ProjectID, req.ServiceAccountJSON, req.AccessToken)
	if err != nil {
		response.BadRequest(c, "GOOGLE_EAB_FAILED", err.Error())
		return
	}
	response.Created(c, cfg)
}
