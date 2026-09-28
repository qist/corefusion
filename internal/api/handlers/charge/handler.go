package charge

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"corefusion/internal/api/handlers/exportutil"
	"corefusion/internal/api/response"
	"corefusion/internal/model/entity"
	"corefusion/internal/security/crypto"
	"corefusion/internal/store/repository"
	"corefusion/pkg/netutil"
)

// PlanRefunder handles plan-related refund operations.
// Implemented by clientauth.CommerceService to avoid circular imports.
type PlanRefunder interface {
	RefundChargeOrder(ctx context.Context, chargeOrderID int64, remark string) error
}

// Handler handles admin charge management HTTP requests.
type Handler struct {
	chargeSiteRepo  *repository.ChargeSiteRepository
	chargeOrderRepo *repository.ChargeOrderRepository
	chargeCardRepo  *repository.ChargeCardRepository
	balanceRepo     *repository.ClientBalanceRepository
	balanceTxRepo   *repository.BalanceTransactionRepository
	clientRepo      *repository.ClientRepository
	userRepo        *repository.UserRepository // v3.3: 用于查询代理员信息
	planRefunder    PlanRefunder               // 退款时回退套餐时间 + 移除节点账号
	db              *gorm.DB
}

// NewHandler creates a new charge admin handler.
func NewHandler(
	chargeSiteRepo *repository.ChargeSiteRepository,
	chargeOrderRepo *repository.ChargeOrderRepository,
	chargeCardRepo *repository.ChargeCardRepository,
	balanceRepo *repository.ClientBalanceRepository,
	balanceTxRepo *repository.BalanceTransactionRepository,
	clientRepo *repository.ClientRepository,
	userRepo *repository.UserRepository, // v3.3: 用于查询代理员信息
	db *gorm.DB,
) *Handler {
	return &Handler{
		chargeSiteRepo:  chargeSiteRepo,
		chargeOrderRepo: chargeOrderRepo,
		chargeCardRepo:  chargeCardRepo,
		balanceRepo:     balanceRepo,
		balanceTxRepo:   balanceTxRepo,
		clientRepo:      clientRepo,
		userRepo:        userRepo,
		db:              db,
	}
}

// SetPlanRefunder sets the plan refunder (called from router initialization).
// 退款时自动回退套餐到期时间 + 从节点移除用户凭证
func (h *Handler) SetPlanRefunder(rf PlanRefunder) {
	h.planRefunder = rf
}

// ============================================================
// Charge Sites CRUD
// ============================================================

// ListSites handles GET /api/v1/charge-sites.
func (h *Handler) ListSites(c *gin.Context) {
	sites, err := h.chargeSiteRepo.ListAll(c.Request.Context())
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	for i := range sites {
		h.maskSiteSecret(&sites[i])
	}
	response.OK(c, sites)
}

// GetSite handles GET /api/v1/charge-sites/:id.
func (h *Handler) GetSite(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	site, err := h.chargeSiteRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "NOT_FOUND", "charge site not found")
		return
	}
	h.maskSiteSecret(site)
	response.OK(c, site)
}

// CreateSite handles POST /api/v1/charge-sites.
func (h *Handler) CreateSite(c *gin.Context) {
	var site entity.ChargeSite
	if err := c.ShouldBindJSON(&site); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	// v3.3: 如果管理员未提供 callback_secret，自动生成强随机密钥
	plaintextSecret := site.CallbackSecret
	if plaintextSecret == "" {
		plaintextSecret = generateCallbackSecret()
	}
	encrypted, err := crypto.EncryptSecret(plaintextSecret)
	if err != nil {
		response.InternalError(c, "ENCRYPT_FAILED", err.Error())
		return
	}
	site.EncryptedSecret = encrypted
	site.SecretVersion = 1
	site.CallbackSecret = "" // 不存明文
	// Extract domain from site URL
	site.Domain = extractDomain(site.SiteURL)
	if err := h.chargeSiteRepo.Create(c.Request.Context(), &site); err != nil {
		response.InternalError(c, "CREATE_FAILED", err.Error())
		return
	}

	// v3.2: 同步写入 charge_site_secrets 表
	secretRecord := &entity.ChargeSiteSecret{
		SiteID:          site.ID,
		Version:         1,
		EncryptedSecret: encrypted,
		Status:          entity.SecretStatusActive,
		CreatedAt:       time.Now(),
	}
	h.db.WithContext(c.Request.Context()).Create(secretRecord)

	h.maskSiteSecret(&site)
	// v3.3: 创建时返回明文密钥（仅此一次），供管理员复制给支付平台
	response.Created(c, gin.H{
		"site":              site,
		"plaintext_secret":  plaintextSecret,
		"secret_shown_once": true,
		"secret_message":    "请立即复制此密钥到支付平台配置，此密钥仅显示一次",
	})
}

// UpdateSite handles PUT /api/v1/charge-sites/:id.
func (h *Handler) UpdateSite(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	site, err := h.chargeSiteRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "NOT_FOUND", "charge site not found")
		return
	}

	// Read incoming update into a temporary struct to detect secret changes
	var incoming struct {
		entity.ChargeSite
		CallbackSecret string `json:"callback_secret"`
	}
	if err := c.ShouldBindJSON(&incoming); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	// Copy fields from incoming to site
	site.Name = incoming.Name
	site.SiteURL = incoming.SiteURL
	site.APIKey = incoming.APIKey
	site.CallbackWhitelist = incoming.CallbackWhitelist
	site.SupportedMethods = incoming.SupportedMethods
	site.Currency = incoming.Currency
	site.IconURL = incoming.IconURL
	site.Enabled = incoming.Enabled
	site.SortOrder = incoming.SortOrder
	site.ConfigJSON = incoming.ConfigJSON
	site.Domain = extractDomain(site.SiteURL)

	// Handle callback_secret: only update if a new plaintext value is provided
	if incoming.CallbackSecret != "" {
		encrypted, err := crypto.EncryptSecret(incoming.CallbackSecret)
		if err != nil {
			response.InternalError(c, "ENCRYPT_FAILED", err.Error())
			return
		}
		site.EncryptedSecret = encrypted
		site.CallbackSecret = "" // 不存明文
	}

	if err := h.chargeSiteRepo.Update(c.Request.Context(), site); err != nil {
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}
	h.maskSiteSecret(site)
	response.OK(c, site)
}

// DeleteSite handles DELETE /api/v1/charge-sites/:id.
func (h *Handler) DeleteSite(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	if err := h.chargeSiteRepo.Delete(c.Request.Context(), id); err != nil {
		response.InternalError(c, "DELETE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "deleted"})
}

// ============================================================
// Domain Verification & Integration Doc
// ============================================================

// GenerateVerifyFile handles POST /api/v1/charge-sites/:id/generate-verify-file.
// Generates a verify token and content for domain ownership verification.
func (h *Handler) GenerateVerifyFile(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	site, err := h.chargeSiteRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "NOT_FOUND", "charge site not found")
		return
	}

	// Generate token: WW_verify_<16 hex chars>
	tokenBytes := make([]byte, 8)
	rand.Read(tokenBytes)
	site.VerifyToken = "WW_verify_" + hex.EncodeToString(tokenBytes)

	// Generate content: 32 random digits
	contentBytes := make([]byte, 16)
	rand.Read(contentBytes)
	site.VerifyContent = hex.EncodeToString(contentBytes)

	site.Verified = false
	if err := h.chargeSiteRepo.Update(c.Request.Context(), site); err != nil {
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}

	// Extract domain for DNS TXT record name
	txtDomain := extractDomain(site.SiteURL)

	response.OK(c, gin.H{
		"verify_token":         site.VerifyToken,
		"verify_content":       site.VerifyContent,
		"verify_file_url":      fmt.Sprintf("%s/%s.txt", strings.TrimRight(site.SiteURL, "/"), site.VerifyToken),
		"dns_txt_record_name":  "_corefusion_verify." + txtDomain,
		"dns_txt_record_value": site.VerifyContent,
		"instructions":         "请将验证文件放置在站点根目录（如 https://pay.example.com/" + site.VerifyToken + ".txt），或添加 DNS TXT 记录后点击验证",
	})
}

// VerifyDomainHTTP handles POST /api/v1/charge-sites/:id/verify-domain-http.
// Checks HTTP accessibility of the verify file.
func (h *Handler) VerifyDomainHTTP(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	site, err := h.chargeSiteRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "NOT_FOUND", "charge site not found")
		return
	}
	if site.VerifyToken == "" || site.VerifyContent == "" {
		response.BadRequest(c, "NOT_GENERATED", "请先生成验证文件")
		return
	}

	verifyURL := fmt.Sprintf("%s/%s.txt", strings.TrimRight(site.SiteURL, "/"), site.VerifyToken)

	// SSRF 防护：解析 URL 域名的 IP 地址，禁止访问内网/保留地址
	parsedURL, err := url.Parse(verifyURL)
	if err != nil {
		response.BadRequest(c, "INVALID_URL", fmt.Sprintf("invalid verify URL: %v", err))
		return
	}
	ips, err := net.LookupIP(parsedURL.Hostname())
	if err != nil {
		response.BadRequest(c, "DNS_FAILED", fmt.Sprintf("DNS 解析失败: %v", err))
		return
	}
	for _, ip := range ips {
		if netutil.IsPrivateIP(ip.String()) || netutil.IsReservedIP(ip.String()) {
			response.BadRequest(c, "SSRF_BLOCKED", fmt.Sprintf("域名解析到内网/保留地址: %s", ip))
			return
		}
	}

	// 上面的 DNS 预检只是第一道；真正的防线在**拨号层**（安全审计 v5 S5）：
	// 旧实现用普通 client，会**跟随 302** → "公网域名跳转到 169.254.169.254" 可绕过预检；
	// 而且先 LookupIP 再请求存在 DNS 重绑定 TOCTOU。安全客户端在每次建连时校验真实 IP，
	// 且不跟随重定向。
	httpClient := netutil.NewSafeHTTPClient(10*time.Second, false)
	resp, err := httpClient.Get(verifyURL)
	if err != nil {
		response.BadRequest(c, "VERIFY_FAILED", fmt.Sprintf("无法访问验证文件: %v", err))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		response.BadRequest(c, "VERIFY_FAILED", fmt.Sprintf("验证文件返回 HTTP %d", resp.StatusCode))
		return
	}
	body, _ := io.ReadAll(resp.Body)
	bodyStr := strings.TrimSpace(string(body))
	if bodyStr != site.VerifyContent {
		response.BadRequest(c, "CONTENT_MISMATCH", "验证文件内容不匹配")
		return
	}

	now := time.Now()
	expire := now.AddDate(0, 6, 0) // 180 days
	site.Verified = true
	site.VerifiedMethod = "http"
	site.VerifiedAt = &now
	site.VerifyExpireAt = &expire
	site.Domain = extractDomain(site.SiteURL)
	h.chargeSiteRepo.Update(c.Request.Context(), site)
	response.OK(c, gin.H{"verified": true, "method": "http", "verified_at": now, "verify_expire_at": expire})
}

// VerifyDomainDNS handles POST /api/v1/charge-sites/:id/verify-domain-dns.
// Checks DNS TXT record for domain ownership verification.
func (h *Handler) VerifyDomainDNS(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	site, err := h.chargeSiteRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "NOT_FOUND", "charge site not found")
		return
	}
	if site.VerifyToken == "" || site.VerifyContent == "" {
		response.BadRequest(c, "NOT_GENERATED", "请先生成验证文件")
		return
	}

	// Extract domain from site URL
	domain := extractDomain(site.SiteURL)
	dnsName := "_corefusion_verify." + domain

	txts, err := net.LookupTXT(dnsName)
	if err != nil {
		response.BadRequest(c, "DNS_LOOKUP_FAILED", fmt.Sprintf("DNS TXT 查询失败: %v", err))
		return
	}
	for _, txt := range txts {
		if strings.TrimSpace(txt) == site.VerifyContent {
			now := time.Now()
			expire := now.AddDate(0, 6, 0) // 180 days
			site.Verified = true
			site.VerifiedMethod = "dns_txt"
			site.VerifiedAt = &now
			site.VerifyExpireAt = &expire
			site.Domain = domain
			h.chargeSiteRepo.Update(c.Request.Context(), site)
			response.OK(c, gin.H{"verified": true, "method": "dns_txt", "verified_at": now, "verify_expire_at": expire})
			return
		}
	}
	response.BadRequest(c, "CONTENT_MISMATCH", "DNS TXT 记录内容不匹配")
}

// GetSiteIntegrationDoc handles GET /api/v1/charge-sites/:id/integration-doc.
// Returns the integration documentation for this specific site.
func (h *Handler) GetSiteIntegrationDoc(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	site, err := h.chargeSiteRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "NOT_FOUND", "charge site not found")
		return
	}

	callbackURL := fmt.Sprintf("/api/v1/charge/callback/%d", site.ID)

	doc := gin.H{
		"site_id":             site.ID,
		"site_name":           site.Name,
		"site_url":            site.SiteURL,
		"callback_url":        callbackURL,
		"callback_secret_set": site.EncryptedSecret != "",
		"verified":            site.Verified,
		"verified_method":     site.VerifiedMethod,
		"verified_at":         site.VerifiedAt,
		"verify_expire_at":    site.VerifyExpireAt,
		"domain":              site.Domain,
		"verify_token":        site.VerifyToken,
		"currency":            site.Currency,
		"supported_methods":   site.SupportedMethods,
		"signature_algorithm": "HMAC-SHA256",
		"signature_fields":    []string{"amount", "body_hash", "currency", "nonce", "site_trade_no", "timestamp", "trade_no", "version"},
		"signature_version":   "v1",
		"body_hash_algorithm": "SHA-256(raw_body) hex-encoded",
		"body_hash_note":      "body_hash and version are auto-included by CoreFusion server; do not send them in the request body but include them in signature payload",
		"timestamp_tolerance": 300,
		"nonce_required":      true,
		"amount_unit":         "cents (分)",
		"ip_whitelist":        site.CallbackWhitelist,
		"response_format":     `{"success": true, "message": "ok"}`,
		"example_callback": gin.H{
			"trade_no":      "CF20250717120000-xxx",
			"site_trade_no": "SITE_12345",
			"status":        "paid",
			"amount":        10000,
			"currency":      "CNY",
			"timestamp":     1721210400,
			"nonce":         "a1b2c3d4e5f6",
			"paid_currency": "CNY",
			"paid_amount":   10000,
			"method":        "alipay",
			"sign":          "<HMAC-SHA256 hex>",
		},
	}

	response.OK(c, doc)
}

// ============================================================
// Charge Orders Management
// ============================================================

// ListOrders handles GET /api/v1/charge-orders.
func (h *Handler) ListOrders(c *gin.Context) {
	status := c.Query("status")
	limit, offset := getPagination(c)
	var items []entity.ChargeOrder
	var total int64
	q := h.db.WithContext(c.Request.Context()).Model(&entity.ChargeOrder{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	q.Count(&total)
	if err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	// Enrich with client email
	type orderVO struct {
		entity.ChargeOrder
		ClientEmail    string `json:"client_email"`
		ClientNickname string `json:"client_nickname"`
	}
	result := make([]orderVO, 0, len(items))
	for _, o := range items {
		vo := orderVO{ChargeOrder: o}
		if cli, err := h.clientRepo.GetByID(c.Request.Context(), o.ClientID); err == nil {
			vo.ClientEmail = cli.Email
			vo.ClientNickname = cli.Nickname
		}
		result = append(result, vo)
	}
	response.Paginated(c, result, total, limit, offset)
}

// RefundOrderInput is the input for refunding a charge order.
type RefundOrderInput struct {
	Remark string `json:"remark"`
}

// RefundOrder handles POST /api/v1/charge-orders/:id/refund.
func (h *Handler) RefundOrder(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	order, err := h.chargeOrderRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "NOT_FOUND", "charge order not found")
		return
	}
	if order.Status != entity.ChargeStatusPaid {
		response.BadRequest(c, "INVALID_STATUS", "only paid orders can be refunded")
		return
	}

	var input RefundOrderInput
	c.ShouldBindJSON(&input)

	remark := input.Remark
	if remark == "" {
		remark = "管理员退款"
	}

	// 如果有 PlanRefunder（CommerceService），使用完整退款逻辑：
	// 退余额 + 回退套餐到期时间 + 从节点移除用户凭证
	if h.planRefunder != nil {
		if err := h.planRefunder.RefundChargeOrder(c.Request.Context(), id, remark); err != nil {
			response.InternalError(c, "REFUND_FAILED", err.Error())
			return
		}
		// 重新查询更新后的订单返回
		order, _ = h.chargeOrderRepo.GetByID(c.Request.Context(), id)
		response.OK(c, order)
		return
	}

	// 兼容模式：无 PlanRefunder 时仅退余额（旧逻辑）
	order.Status = entity.ChargeStatusRefunded
	order.Remark = remark
	if err := h.chargeOrderRepo.Update(c.Request.Context(), order); err != nil {
		response.InternalError(c, "REFUND_FAILED", err.Error())
		return
	}

	// Refund to client balance
	if order.Amount > 0 {
		bal, err := h.balanceRepo.AddBalance(c.Request.Context(), order.ClientID, order.Amount)
		if err == nil {
			txRecord := &entity.BalanceTransaction{
				ClientID:             order.ClientID,
				Type:                 entity.BalanceTxTypeRefund,
				Amount:               order.Amount,
				BalanceBefore:        bal.Balance - order.Amount,
				BalanceAfter:         bal.Balance,
				RelatedChargeOrderID: &order.ID,
				Remark:               remark,
			}
			h.balanceTxRepo.Create(c.Request.Context(), txRecord)
		}
	}

	response.OK(c, order)
}

// GetOrder handles GET /api/v1/charge-orders/:id.
func (h *Handler) GetOrder(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	order, err := h.chargeOrderRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "NOT_FOUND", "charge order not found")
		return
	}
	response.OK(c, order)
}

// ============================================================
// Charge Cards Management
// ============================================================

// ListCards handles GET /api/v1/charge-cards.
func (h *Handler) ListCards(c *gin.Context) {
	status := c.Query("status")
	cardType := c.Query("card_type")
	// v3.3: 支持按代理员筛选
	var agentID *int64
	if agentIDStr := c.Query("agent_id"); agentIDStr != "" {
		if id, err := strconv.ParseInt(agentIDStr, 10, 64); err == nil {
			agentID = &id
		}
	}
	limit, offset := getPagination(c)
	cards, total, err := h.chargeCardRepo.ListAll(c.Request.Context(), status, cardType, agentID, limit, offset)
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	response.Paginated(c, cards, total, limit, offset)
}

// CreateCardInput is the input for creating a single charge card.
// v3.3: 移除 max_uses — 充值卡一次性使用
type CreateCardInput struct {
	CardType       string     `json:"card_type" binding:"required"`
	Value          int64      `json:"value"`           // 金额(最小单位)或 plan_variant_id
	Currency       string     `json:"currency"`        // 货币类型
	PlanVariantID  *int64     `json:"plan_variant_id"` // 套餐型时绑定的规格ID
	ExpiresAt      *time.Time `json:"expires_at"`
	BatchID        string     `json:"batch_id"`
	Remark         string     `json:"remark"`
	CardCodeFormat string     `json:"card_code_format"`
	AgentID        *int64     `json:"agent_id"` // 代理员/推广员管理员ID，为空表示官方
}

// CreateCard handles POST /api/v1/charge-cards.
func (h *Handler) CreateCard(c *gin.Context) {
	var input CreateCardInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	// 套餐型/流量包型必须有有效的 plan_variant_id
	if (input.CardType == entity.ChargeCardTypePlan || input.CardType == entity.ChargeCardTypeTrafficPack) && input.PlanVariantID == nil {
		response.BadRequest(c, "INVALID_VALUE", "plan/traffic_pack card requires a valid plan_variant_id")
		return
	}
	cardValue := input.Value
	if (input.CardType == entity.ChargeCardTypePlan || input.CardType == entity.ChargeCardTypeTrafficPack) && input.PlanVariantID != nil {
		cardValue = *input.PlanVariantID
	}
	// v3.3: 关联创建者管理员
	adminID := c.GetInt64("user_id")
	adminName := c.GetString("username")
	// v3.3: 关联代理员/推广员
	var agentName string
	if input.AgentID != nil && *input.AgentID > 0 {
		if user, err := h.userRepo.GetByID(c.Request.Context(), *input.AgentID); err == nil {
			agentName = user.Username
		}
	}
	card := entity.ChargeCard{
		CardCode:           GenerateCardCode(input.CardCodeFormat),
		CardType:           input.CardType,
		Value:              cardValue,
		Currency:           input.Currency,
		Status:             entity.ChargeCardStatusActive,
		BatchID:            input.BatchID,
		Remark:             input.Remark,
		ExpiresAt:          input.ExpiresAt,
		CreatedByAdminID:   adminID,
		CreatedByAdminName: adminName,
		AgentID:            input.AgentID,
		AgentName:          agentName,
	}
	if err := h.chargeCardRepo.Create(c.Request.Context(), &card); err != nil {
		response.InternalError(c, "CREATE_FAILED", err.Error())
		return
	}
	// v3.3: 返回明文卡号（仅此一次），列表中会脱敏
	response.Created(c, gin.H{
		"card":                card,
		"plaintext_card_code": card.CardCode,
		"secret_shown_once":   true,
	})
}

// BatchCreateCardsInput is the input for batch creating charge cards.
// v3.3: 移除 max_uses — 充值卡一次性使用
type BatchCreateCardsInput struct {
	CardType       string  `json:"card_type" binding:"required"`
	Value          int64   `json:"value"`           // 金额(最小单位)或 plan_variant_id
	Currency       string  `json:"currency"`        // 货币类型
	PlanVariantID  *int64  `json:"plan_variant_id"` // 套餐型时绑定的规格ID
	Count          int     `json:"count" binding:"required,min=1,max=1000"`
	ExpiresAt      *string `json:"expires_at"`
	BatchID        string  `json:"batch_id"`
	Remark         string  `json:"remark"`
	CardCodeFormat string  `json:"card_code_format"`
	AgentID        *int64  `json:"agent_id"` // 代理员/推广员管理员ID，为空表示官方
}

// BatchCreateCards handles POST /api/v1/charge-cards/batch.
func (h *Handler) BatchCreateCards(c *gin.Context) {
	var input BatchCreateCardsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	// 套餐型/流量包型必须提供 plan_variant_id
	if (input.CardType == entity.ChargeCardTypePlan || input.CardType == entity.ChargeCardTypeTrafficPack) && input.PlanVariantID == nil {
		response.BadRequest(c, "MISSING_VARIANT", "plan/traffic_pack card requires plan_variant_id")
		return
	}
	// 金额型必须提供 value
	if input.CardType == entity.ChargeCardTypeBalance && input.Value <= 0 {
		response.BadRequest(c, "INVALID_VALUE", "balance card requires a positive value")
		return
	}
	if input.BatchID == "" {
		input.BatchID = generateBatchID()
	}
	// 确定实际 value：套餐型/流量包型用 plan_variant_id
	cardValue := input.Value
	if (input.CardType == entity.ChargeCardTypePlan || input.CardType == entity.ChargeCardTypeTrafficPack) && input.PlanVariantID != nil {
		cardValue = *input.PlanVariantID
	}

	// v3.3: 关联创建者管理员
	adminID := c.GetInt64("user_id")
	adminName := c.GetString("username")
	// v3.3: 关联代理员/推广员
	var agentName string
	if input.AgentID != nil && *input.AgentID > 0 {
		if user, err := h.userRepo.GetByID(c.Request.Context(), *input.AgentID); err == nil {
			agentName = user.Username
		}
	}

	cards := make([]entity.ChargeCard, input.Count)
	for i := 0; i < input.Count; i++ {
		cards[i] = entity.ChargeCard{
			CardCode:           GenerateCardCode(input.CardCodeFormat),
			CardType:           input.CardType,
			Value:              cardValue,
			Currency:           input.Currency,
			Status:             entity.ChargeCardStatusActive,
			BatchID:            input.BatchID,
			Remark:             input.Remark,
			CreatedByAdminID:   adminID,
			CreatedByAdminName: adminName,
			AgentID:            input.AgentID,
			AgentName:          agentName,
		}
		if input.ExpiresAt != nil {
			t, err := time.Parse(time.RFC3339, *input.ExpiresAt)
			if err == nil {
				cards[i].ExpiresAt = &t
			}
		}
	}

	if err := h.chargeCardRepo.CreateBatch(c.Request.Context(), cards); err != nil {
		response.InternalError(c, "BATCH_CREATE_FAILED", err.Error())
		return
	}
	// v3.3: 批量生成返回明文卡号（仅此一次），列表中会脱敏
	response.Created(c, gin.H{"batch_id": input.BatchID, "count": input.Count, "cards": cards})
}

// DeleteCard handles DELETE /api/v1/charge-cards/:id.
func (h *Handler) DeleteCard(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	if err := h.chargeCardRepo.Delete(c.Request.Context(), id); err != nil {
		response.InternalError(c, "DELETE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "deleted"})
}

// GetCardUsages handles GET /api/v1/charge-cards/:id/usages.
// v3.3: 查看充值卡的使用记录（谁用了、何时用、哪个订单）
func (h *Handler) GetCardUsages(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}

	type usageVO struct {
		entity.ChargeOrder
		ClientEmail    string `json:"client_email"`
		ClientNickname string `json:"client_nickname"`
	}

	var orders []entity.ChargeOrder
	if err := h.db.WithContext(c.Request.Context()).
		Where("charge_card_id = ?", id).
		Order("created_at DESC").
		Find(&orders).Error; err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}

	result := make([]usageVO, 0, len(orders))
	for _, o := range orders {
		vo := usageVO{ChargeOrder: o}
		if cli, err := h.clientRepo.GetByID(c.Request.Context(), o.ClientID); err == nil {
			vo.ClientEmail = cli.Email
			vo.ClientNickname = cli.Nickname
		}
		result = append(result, vo)
	}

	response.OK(c, gin.H{"items": result, "total": len(result)})
}

// ExportCardsCSV handles GET /api/v1/charge-cards/export-csv.
// v3.3: 导出充值卡 CSV（含明文卡号，仅管理员可导出）
func (h *Handler) ExportCardsCSV(c *gin.Context) {
	status := c.Query("status")
	cardType := c.Query("card_type")
	batchID := c.Query("batch_id")
	// v3.3: 支持按代理员筛选导出
	var agentID *int64
	if agentIDStr := c.Query("agent_id"); agentIDStr != "" {
		if id, err := strconv.ParseInt(agentIDStr, 10, 64); err == nil {
			agentID = &id
		}
	}

	// 上限原来写死 100000 ✗ → 改为设置项（设置页 → 面板设置 → 导出上限）✓
	exportLimit := exportutil.MaxRows(c.Request.Context(), h.db)
	cards, _, err := h.chargeCardRepo.ListAll(c.Request.Context(), status, cardType, agentID, exportLimit, 0)
	if err != nil {
		response.InternalError(c, "EXPORT_FAILED", err.Error())
		return
	}

	// 按批次号筛选（如果指定了 batch_id）
	if batchID != "" {
		var filtered []entity.ChargeCard
		for _, card := range cards {
			if card.BatchID == batchID {
				filtered = append(filtered, card)
			}
		}
		cards = filtered
	}

	c.Header("Content-Disposition", "attachment; filename=charge-cards.csv")
	c.Header("Content-Type", "text/csv; charset=utf-8")

	// BOM for Excel UTF-8 compatibility
	var sb strings.Builder
	sb.WriteString("\xEF\xBB\xBF")
	sb.WriteString("ID,卡号,类型,面值/套餐ID,货币,是否已用,状态,批次号,创建者,创建时间,过期时间,备注\n")

	for _, card := range cards {
		cardCode := card.CardCode // ListAll 已解密填充
		if cardCode == "" {
			cardCode = card.CardCodeMasked
		}
		typeLabel := "金额"
		if card.CardType == entity.ChargeCardTypePlan {
			typeLabel = "套餐"
		} else if card.CardType == entity.ChargeCardTypeTrafficPack {
			typeLabel = "流量包"
		}
		expiresAt := ""
		if card.ExpiresAt != nil {
			expiresAt = card.ExpiresAt.Format("2006-01-02 15:04:05")
		}
		// CSV escape: wrap in quotes if contains comma
		remark := csvEscape(card.Remark)
		sb.WriteString(fmt.Sprintf("%d,%s,%s,%d,%s,%s,%s,%s,%s,%s,%s,%s\n",
			card.ID,
			csvEscape(cardCode),
			typeLabel,
			card.Value,
			card.Currency,
			boolStr(card.UsedCount > 0),
			card.Status,
			csvEscape(card.BatchID),
			csvEscape(card.CreatedByAdminName),
			card.CreatedAt.Format("2006-01-02 15:04:05"),
			expiresAt,
			remark,
		))
	}

	c.String(http.StatusOK, sb.String())
}

// csvEscape 保留原名以免大范围改动 ✓，实现委托给共用转义（RFC 4180 ✓）。
// ⚠ 别再各写一份 ✗：转义规则一旦不一致，导出的 CSV 就会在 Excel 里列错位 ✓。
func csvEscape(s string) string {
	return exportutil.Escape(s)
}

// boolStr returns "是" or "否" for CSV export.
func boolStr(b bool) string {
	if b {
		return "是"
	}
	return "否"
}

// ============================================================
// Client Balance Management
// ============================================================

// ListBalances handles GET /api/v1/client-balances.
func (h *Handler) ListBalances(c *gin.Context) {
	limit, offset := getPagination(c)
	var items []entity.ClientBalance
	var total int64
	q := h.db.WithContext(c.Request.Context()).Model(&entity.ClientBalance{})
	q.Count(&total)
	if err := q.Order("id ASC").Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	// Enrich with client email & nickname
	type balanceVO struct {
		entity.ClientBalance
		ClientEmail    string `json:"client_email"`
		ClientNickname string `json:"client_nickname"`
	}
	result := make([]balanceVO, 0, len(items))
	for _, b := range items {
		vo := balanceVO{ClientBalance: b}
		if cli, err := h.clientRepo.GetByID(c.Request.Context(), b.ClientID); err == nil {
			vo.ClientEmail = cli.Email
			vo.ClientNickname = cli.Nickname
		}
		result = append(result, vo)
	}
	response.Paginated(c, result, total, limit, offset)
}

// ListBalanceTransactionsAdmin handles GET /api/v1/client-balances/transactions.
func (h *Handler) ListBalanceTransactionsAdmin(c *gin.Context) {
	clientIDStr := c.Query("client_id")
	txType := c.Query("type")
	limit, offset := getPagination(c)

	if clientIDStr != "" {
		clientID, err := strconv.ParseInt(clientIDStr, 10, 64)
		if err != nil {
			response.BadRequest(c, "INVALID_CLIENT_ID", "invalid client_id")
			return
		}
		txs, total, err := h.balanceTxRepo.ListByClientID(c.Request.Context(), clientID, txType, limit, offset)
		if err != nil {
			response.InternalError(c, "LIST_FAILED", err.Error())
			return
		}
		response.Paginated(c, txs, total, limit, offset)
		return
	}

	var items []entity.BalanceTransaction
	var total int64
	q := h.db.WithContext(c.Request.Context()).Model(&entity.BalanceTransaction{})
	if txType != "" {
		q = q.Where("type = ?", txType)
	}
	q.Count(&total)
	if err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	response.Paginated(c, items, total, limit, offset)
}

// ============================================================
// Helpers
// ============================================================

// GenerateCardCode generates a unique card code.
// If format is empty, uses the default format: CF-XXXXXXXX-XXXXXXXX-XXXXXXXX-XXXXXXXX.
// Custom format uses {N} placeholders where N is the number of hex chars per segment.
// Examples: "{8}-{8}-{8}-{8}" -> XXXXXXXX-XXXXXXXX-XXXXXXXX-XXXXXXXX
//
//	"{32}"             -> 32 hex chars without separator
//	"CF-{8}-{8}"       -> CF-XXXXXXXX-XXXXXXXX
func GenerateCardCode(format ...string) string {
	fmtStr := "CF-{8}-{8}-{8}-{8}" // default
	if len(format) > 0 && format[0] != "" {
		fmtStr = format[0]
	}

	// Calculate total hex chars needed
	totalHex := 0
	i := 0
	for i < len(fmtStr) {
		if fmtStr[i] == '{' {
			j := i + 1
			for j < len(fmtStr) && fmtStr[j] != '}' {
				j++
			}
			if j < len(fmtStr) {
				n, err := strconv.Atoi(fmtStr[i+1 : j])
				if err == nil && n > 0 {
					totalHex += n
				}
				i = j + 1
				continue
			}
		}
		i++
	}
	if totalHex == 0 {
		totalHex = 32
	}

	// Generate random hex
	byteLen := (totalHex + 1) / 2
	b := make([]byte, byteLen)
	rand.Read(b)
	hexStr := hex.EncodeToString(b)
	if len(hexStr) < totalHex {
		// unlikely, but pad if needed
		extra := make([]byte, (totalHex-len(hexStr)+1)/2)
		rand.Read(extra)
		hexStr += hex.EncodeToString(extra)
	}

	// Build result by replacing {N} with hex segments
	var result []byte
	hexIdx := 0
	i = 0
	for i < len(fmtStr) {
		if fmtStr[i] == '{' {
			j := i + 1
			for j < len(fmtStr) && fmtStr[j] != '}' {
				j++
			}
			if j < len(fmtStr) {
				n, err := strconv.Atoi(fmtStr[i+1 : j])
				if err == nil && n > 0 {
					end := hexIdx + n
					if end > len(hexStr) {
						end = len(hexStr)
					}
					result = append(result, hexStr[hexIdx:end]...)
					hexIdx = end
					i = j + 1
					continue
				}
			}
		}
		result = append(result, fmtStr[i])
		i++
	}
	return string(result)
}

func generateBatchID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("B%s%s", time.Now().Format("20060102"), hex.EncodeToString(b))
}

// extractDomain extracts the hostname from a URL string.
func extractDomain(rawURL string) string {
	u := rawURL
	if strings.HasPrefix(u, "https://") {
		u = strings.TrimPrefix(u, "https://")
	} else if strings.HasPrefix(u, "http://") {
		u = strings.TrimPrefix(u, "http://")
	}
	return strings.Split(u, "/")[0]
}

func getPagination(c *gin.Context) (limit, offset int) {
	limit = 20
	offset = 0
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 && l <= 100 {
		limit = l
	}
	if o, err := strconv.Atoi(c.Query("offset")); err == nil && o >= 0 {
		offset = o
	}
	return
}

// generateCallbackSecret generates a strong random callback secret (48 chars hex = 192-bit entropy).
// v3.3: 用于 CreateSite / RotateSecret 自动生成，避免管理员手动设置弱密钥。
func generateCallbackSecret() string {
	b := make([]byte, 24) // 24 bytes = 48 hex chars
	rand.Read(b)
	return "cs_" + hex.EncodeToString(b) // cs_ 前缀便于识别
}

// maskSiteSecret masks the callback_secret for API responses and hides the encrypted_secret.
func (h *Handler) maskSiteSecret(site *entity.ChargeSite) {
	// Try to decrypt for masking; if decryption fails, check plaintext fallback
	plain := ""
	if site.EncryptedSecret != "" {
		if decrypted, err := crypto.DecryptSecret(site.EncryptedSecret); err == nil {
			plain = decrypted
		}
	}
	if plain == "" && site.CallbackSecret != "" {
		// Backward compat: plaintext stored directly
		plain = site.CallbackSecret
	}
	site.CallbackSecret = crypto.MaskSecret(plain)
	site.EncryptedSecret = ""     // Never expose encrypted_secret to frontend
	site.EncryptedSecretPrev = "" // Never expose to frontend
}

// ============================================================
// Callback Logs & Retry Management
// ============================================================

// ListCallbackEvents handles GET /api/v1/charge-callback-events.
// v3.1: merged callback_logs into callback_events — events ARE logs.
func (h *Handler) ListCallbackEvents(c *gin.Context) {
	siteIDStr := c.Query("site_id")
	result := c.Query("result")
	limit, offset := getPagination(c)

	var items []entity.ChargeCallbackEvent
	var total int64
	q := h.db.WithContext(c.Request.Context()).Model(&entity.ChargeCallbackEvent{})
	if siteIDStr != "" {
		q = q.Where("site_id = ?", siteIDStr)
	}
	if result != "" {
		q = q.Where("result = ?", result)
	}
	q.Count(&total)
	if err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	response.Paginated(c, items, total, limit, offset)
}

// ListCallbackRetries handles GET /api/v1/charge-callback-retries.
func (h *Handler) ListCallbackRetries(c *gin.Context) {
	status := c.Query("status")
	limit, offset := getPagination(c)

	var items []entity.ChargeCallbackRetry
	var total int64
	q := h.db.WithContext(c.Request.Context()).Model(&entity.ChargeCallbackRetry{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	q.Count(&total)
	if err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	response.Paginated(c, items, total, limit, offset)
}

// RetryCallback handles POST /api/v1/charge-callback-retries/:id/retry.
// Manually triggers a retry for a specific callback.
func (h *Handler) RetryCallback(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	var retry entity.ChargeCallbackRetry
	if err := h.db.WithContext(c.Request.Context()).First(&retry, id).Error; err != nil {
		response.NotFound(c, "NOT_FOUND", "retry record not found")
		return
	}
	// Mark for immediate retry
	now := time.Now()
	h.db.Model(&retry).Updates(map[string]interface{}{
		"status":        "pending",
		"next_retry_at": now,
	})
	response.OK(c, gin.H{"message": "retry scheduled"})
}

// ============================================================
// Secret Rotation — 密钥轮换
// ============================================================

// RotateSecretInput is the input for rotating the callback secret.
// v3.3: new_secret 改为可选 — 为空时自动生成强随机密钥，避免人工出错
type RotateSecretInput struct {
	NewSecret string `json:"new_secret"` // 可选，为空则自动生成
}

// RotateSecret handles POST /api/v1/charge-sites/:id/rotate-secret.
// v3.2: Uses charge_site_secrets table for multi-version key management.
// v3.3: new_secret 为空时自动生成，返回明文密钥（仅此一次）
func (h *Handler) RotateSecret(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	site, err := h.chargeSiteRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "NOT_FOUND", "charge site not found")
		return
	}
	var input RotateSecretInput
	_ = c.ShouldBindJSON(&input) // v3.3: 不再 required

	// v3.3: 为空时自动生成强随机密钥
	plaintextSecret := input.NewSecret
	if plaintextSecret == "" {
		plaintextSecret = generateCallbackSecret()
	}

	// 加密新密钥
	encrypted, err := crypto.EncryptSecret(plaintextSecret)
	if err != nil {
		response.InternalError(c, "ENCRYPT_FAILED", err.Error())
		return
	}

	newVersion := site.SecretVersion + 1

	// v3.2: 在 charge_site_secrets 表中操作
	err = h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		// 将当前 active 密钥降级为 legacy
		tx.Model(&entity.ChargeSiteSecret{}).
			Where("site_id = ? AND status = ?", id, entity.SecretStatusActive).
			Update("status", entity.SecretStatusLegacy)

		// 插入新密钥为 active
		newSecretRecord := &entity.ChargeSiteSecret{
			SiteID:          id,
			Version:         newVersion,
			EncryptedSecret: encrypted,
			Status:          entity.SecretStatusActive,
			CreatedAt:       time.Now(),
		}
		if err := tx.Create(newSecretRecord).Error; err != nil {
			return err
		}

		// 更新 ChargeSite 兼容字段（旧逻辑回退用）
		site.SecretVersion = newVersion
		site.EncryptedSecret = encrypted
		site.CallbackSecret = ""
		return tx.Model(&entity.ChargeSite{}).Where("id = ?", id).
			Updates(map[string]interface{}{
				"secret_version":   newVersion,
				"encrypted_secret": encrypted,
				"callback_secret":  "",
			}).Error
	})
	if err != nil {
		response.InternalError(c, "ROTATE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{
		"message":           "secret rotated",
		"new_version":       newVersion,
		"plaintext_secret":  plaintextSecret,
		"secret_shown_once": true,
		"secret_message":    "请立即复制此密钥到支付平台配置，此密钥仅显示一次",
	})
}

// ClearPrevSecret handles POST /api/v1/charge-sites/:id/clear-prev-secret.
// v3.2: Expires all legacy secrets in charge_site_secrets table.
func (h *Handler) ClearPrevSecret(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	site, err := h.chargeSiteRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "NOT_FOUND", "charge site not found")
		return
	}
	now := time.Now()
	if err := h.db.WithContext(c.Request.Context()).Model(&entity.ChargeSiteSecret{}).
		Where("site_id = ? AND status = ?", id, entity.SecretStatusLegacy).
		Updates(map[string]interface{}{
			"status":     entity.SecretStatusExpired,
			"expired_at": now,
		}).Error; err != nil {
		response.InternalError(c, "CLEAR_FAILED", err.Error())
		return
	}
	// 清除兼容字段旧密钥
	site.EncryptedSecretPrev = ""
	h.chargeSiteRepo.Update(c.Request.Context(), site)
	response.OK(c, gin.H{"message": "legacy secrets expired"})
}

// ============================================================
// CSV Exports（2026-09-23 补齐）
// ============================================================
//
// 背景（代码完成度体检）：充值数据里**只有充值卡能导出**（v3.3 ✓），
// 而**充值订单**与**余额流水**只有列表、没有导出 ✗ —— 这两样恰恰是财务对账
// 和排查"钱去哪了"最常用的数据。这里各补一个导出端点。
// 行数上限统一走设置项 panel.export_max_rows ✓（与其它导出一致 ✓）。

// ExportOrdersCSV handles GET /api/v1/charge-orders/export-csv.
//
// 筛选与 ListOrders 一致（status ✓）。金额按**最小货币单位**导出（与充值卡导出一致 ✓），
// 表头里写明单位，避免财务误读 ✗。
func (h *Handler) ExportOrdersCSV(c *gin.Context) {
	ctx := c.Request.Context()
	limit := exportutil.Limit(c, exportutil.MaxRows(ctx, h.db))

	var orders []entity.ChargeOrder
	q := h.db.WithContext(ctx).Model(&entity.ChargeOrder{})
	if status := c.Query("status"); status != "" {
		q = q.Where("status = ?", status)
	}
	// 导出按**时间正序**：财务对着表格从上往下核对，倒序会很难读 ✓
	if err := q.Order("created_at ASC").Limit(limit).Find(&orders).Error; err != nil {
		response.InternalError(c, "EXPORT_FAILED", err.Error())
		return
	}

	c.Header("Content-Disposition",
		`attachment; filename="charge-orders-`+exportutil.Timestamp()+`.csv"`)
	c.Header("Content-Type", "text/csv; charset=utf-8")

	var sb strings.Builder
	sb.WriteString(exportutil.BOM)
	sb.WriteString("ID,交易号,站点ID,站点交易号,客户端ID,客户端邮箱,订单类型,金额(最小单位),币种,实付金额(最小单位),实付币种,汇率,支付方式,状态,创建时间,支付时间,备注\n")
	for i := range orders {
		o := &orders[i]
		email := ""
		if cli, err := h.clientRepo.GetByID(ctx, o.ClientID); err == nil {
			email = cli.Email
		}
		siteID := ""
		if o.SiteID != nil {
			siteID = strconv.FormatInt(*o.SiteID, 10)
		}
		paidAt := ""
		if o.PaidAt != nil {
			paidAt = o.PaidAt.Format("2006-01-02 15:04:05")
		}
		cols := []string{
			strconv.FormatInt(o.ID, 10),
			exportutil.Escape(o.TradeNo),
			siteID,
			exportutil.Escape(o.SiteTradeNo),
			strconv.FormatInt(o.ClientID, 10),
			exportutil.Escape(email),
			exportutil.Escape(o.OrderType),
			strconv.FormatInt(o.Amount, 10),
			o.Currency,
			strconv.FormatInt(o.PaidAmount, 10),
			o.PaidCurrency,
			strconv.FormatFloat(o.ExchangeRate, 'f', -1, 64),
			exportutil.Escape(o.Method),
			o.Status,
			o.CreatedAt.Format("2006-01-02 15:04:05"),
			paidAt,
			exportutil.Escape(o.Remark),
		}
		sb.WriteString(strings.Join(cols, ",") + "\n")
	}
	c.String(http.StatusOK, sb.String())
}

// ExportBalanceTransactionsCSV handles GET /api/v1/client-balances/transactions/export-csv.
//
// 余额流水原来只能在"余额管理 → 某客户端的流水弹窗"里逐个看 ✗，无法整体导出对账 ✓。
// 筛选与 ListBalanceTransactionsAdmin 一致（client_id / type ✓）。
func (h *Handler) ExportBalanceTransactionsCSV(c *gin.Context) {
	ctx := c.Request.Context()
	limit := exportutil.Limit(c, exportutil.MaxRows(ctx, h.db))

	var txs []entity.BalanceTransaction
	q := h.db.WithContext(ctx).Model(&entity.BalanceTransaction{})
	if clientIDStr := c.Query("client_id"); clientIDStr != "" {
		if clientID, err := strconv.ParseInt(clientIDStr, 10, 64); err == nil {
			q = q.Where("client_id = ?", clientID)
		}
	}
	if txType := c.Query("type"); txType != "" {
		q = q.Where("type = ?", txType)
	}
	// 同样按时间正序 ✓（对账要从早往晚看余额变化 ✓）
	if err := q.Order("created_at ASC").Limit(limit).Find(&txs).Error; err != nil {
		response.InternalError(c, "EXPORT_FAILED", err.Error())
		return
	}

	c.Header("Content-Disposition",
		`attachment; filename="balance-transactions-`+exportutil.Timestamp()+`.csv"`)
	c.Header("Content-Type", "text/csv; charset=utf-8")

	var sb strings.Builder
	sb.WriteString(exportutil.BOM)
	sb.WriteString("ID,客户端ID,客户端邮箱,类型,金额(最小单位),变动前(最小单位),变动后(最小单位),币种,关联充值订单ID,关联套餐订单ID,备注,时间\n")
	for i := range txs {
		tx := &txs[i]
		email := ""
		if cli, err := h.clientRepo.GetByID(ctx, tx.ClientID); err == nil {
			email = cli.Email
		}
		relatedCharge := ""
		if tx.RelatedChargeOrderID != nil {
			relatedCharge = strconv.FormatInt(*tx.RelatedChargeOrderID, 10)
		}
		relatedPlan := ""
		if tx.RelatedPlanOrderID != nil {
			relatedPlan = strconv.FormatInt(*tx.RelatedPlanOrderID, 10)
		}
		cols := []string{
			strconv.FormatInt(tx.ID, 10),
			strconv.FormatInt(tx.ClientID, 10),
			exportutil.Escape(email),
			exportutil.Escape(tx.Type),
			strconv.FormatInt(tx.Amount, 10),
			strconv.FormatInt(tx.BalanceBefore, 10),
			strconv.FormatInt(tx.BalanceAfter, 10),
			tx.Currency,
			relatedCharge,
			relatedPlan,
			exportutil.Escape(tx.Remark),
			tx.CreatedAt.Format("2006-01-02 15:04:05"),
		}
		sb.WriteString(strings.Join(cols, ",") + "\n")
	}
	c.String(http.StatusOK, sb.String())
}
