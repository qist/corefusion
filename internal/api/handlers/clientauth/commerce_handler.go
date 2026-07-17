package clientauth

import (
	"crypto/rand"
	"encoding/hex"
	"strconv"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/service/clientauth"
)

// CommerceHandler handles client commerce HTTP requests.
type CommerceHandler struct {
	svc *clientauth.CommerceService
}

// NewCommerceHandler creates a new CommerceHandler.
func NewCommerceHandler(svc *clientauth.CommerceService) *CommerceHandler {
	return &CommerceHandler{svc: svc}
}

// ============================================================
// P0-1: Plan Display
// ============================================================

// ListPlans handles GET /api/v1/client-auth/plans.
func (h *CommerceHandler) ListPlans(c *gin.Context) {
	plans, err := h.svc.ListPlans(c.Request.Context())
	if err != nil {
		response.InternalError(c, "LIST_PLANS_FAILED", err.Error())
		return
	}
	response.OK(c, plans)
}

// GetPlan handles GET /api/v1/client-auth/plans/:id.
func (h *CommerceHandler) GetPlan(c *gin.Context) {
	planID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid plan ID")
		return
	}
	plan, err := h.svc.GetPlan(c.Request.Context(), planID)
	if err != nil {
		response.NotFound(c, "PLAN_NOT_FOUND", err.Error())
		return
	}
	response.OK(c, plan)
}

// ============================================================
// Traffic Pack (流量叠加包)
// ============================================================

// ListTrafficPacks handles GET /api/v1/client-auth/traffic-packs.
func (h *CommerceHandler) ListTrafficPacks(c *gin.Context) {
	packs, err := h.svc.ListTrafficPacks(c.Request.Context())
	if err != nil {
		response.InternalError(c, "LIST_TRAFFIC_PACKS_FAILED", err.Error())
		return
	}
	response.OK(c, packs)
}

// BuyTrafficPack handles POST /api/v1/client-auth/traffic-packs/buy.
func (h *CommerceHandler) BuyTrafficPack(c *gin.Context) {
	clientID := getClientID(c)
	if clientID == 0 {
		response.Unauthorized(c, "UNAUTHORIZED", "client not authenticated")
		return
	}
	var input clientauth.BuyTrafficPackInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	result, err := h.svc.BuyTrafficPack(c.Request.Context(), clientID, input)
	if err != nil {
		response.BadRequest(c, "BUY_TRAFFIC_PACK_FAILED", err.Error())
		return
	}
	response.Created(c, result)
}

// ============================================================
// P0-2: Order Management
// ============================================================

// CreateOrder handles POST /api/v1/client-auth/orders.
func (h *CommerceHandler) CreateOrder(c *gin.Context) {
	clientID := getClientID(c)
	if clientID == 0 {
		response.Unauthorized(c, "UNAUTHORIZED", "client not authenticated")
		return
	}
	var input clientauth.CreateOrderInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	result, err := h.svc.CreateOrder(c.Request.Context(), clientID, input)
	if err != nil {
		response.BadRequest(c, "CREATE_ORDER_FAILED", err.Error())
		return
	}
	response.Created(c, result)
}

// ListOrders handles GET /api/v1/client-auth/orders.
func (h *CommerceHandler) ListOrders(c *gin.Context) {
	clientID := getClientID(c)
	if clientID == 0 {
		response.Unauthorized(c, "UNAUTHORIZED", "client not authenticated")
		return
	}
	limit, offset := getPagination(c)
	orders, total, err := h.svc.ListOrders(c.Request.Context(), clientID, limit, offset)
	if err != nil {
		response.InternalError(c, "LIST_ORDERS_FAILED", err.Error())
		return
	}
	response.Paginated(c, orders, total, limit, offset)
}

// GetOrder handles GET /api/v1/client-auth/orders/:id.
func (h *CommerceHandler) GetOrder(c *gin.Context) {
	clientID := getClientID(c)
	if clientID == 0 {
		response.Unauthorized(c, "UNAUTHORIZED", "client not authenticated")
		return
	}
	orderID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid order ID")
		return
	}
	order, err := h.svc.GetOrder(c.Request.Context(), clientID, orderID)
	if err != nil {
		response.NotFound(c, "ORDER_NOT_FOUND", err.Error())
		return
	}
	response.OK(c, order)
}

// ============================================================
// P0-3: Charging System
// ============================================================

// ListChargeSites handles GET /api/v1/client-auth/charge/sites.
func (h *CommerceHandler) ListChargeSites(c *gin.Context) {
	sites, err := h.svc.ListChargeSites(c.Request.Context())
	if err != nil {
		response.InternalError(c, "LIST_SITES_FAILED", err.Error())
		return
	}
	response.OK(c, sites)
}

// CreateChargeOrder handles POST /api/v1/client-auth/charge/create.
func (h *CommerceHandler) CreateChargeOrder(c *gin.Context) {
	clientID := getClientID(c)
	if clientID == 0 {
		response.Unauthorized(c, "UNAUTHORIZED", "client not authenticated")
		return
	}
	var input clientauth.CreateChargeInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	result, err := h.svc.CreateChargeOrder(c.Request.Context(), clientID, input)
	if err != nil {
		response.BadRequest(c, "CREATE_CHARGE_FAILED", err.Error())
		return
	}
	response.Created(c, result)
}

// GetChargeStatus handles GET /api/v1/client-auth/charge/status/:trade_no.
func (h *CommerceHandler) GetChargeStatus(c *gin.Context) {
	clientID := getClientID(c)
	if clientID == 0 {
		response.Unauthorized(c, "UNAUTHORIZED", "client not authenticated")
		return
	}
	tradeNo := c.Param("trade_no")
	order, err := h.svc.GetChargeStatus(c.Request.Context(), clientID, tradeNo)
	if err != nil {
		response.NotFound(c, "CHARGE_ORDER_NOT_FOUND", err.Error())
		return
	}
	response.OK(c, order)
}

// ListChargeOrders handles GET /api/v1/client-auth/charge/orders.
func (h *CommerceHandler) ListChargeOrders(c *gin.Context) {
	clientID := getClientID(c)
	if clientID == 0 {
		response.Unauthorized(c, "UNAUTHORIZED", "client not authenticated")
		return
	}
	status := c.Query("status")
	limit, offset := getPagination(c)
	orders, total, err := h.svc.ListChargeOrders(c.Request.Context(), clientID, status, limit, offset)
	if err != nil {
		response.InternalError(c, "LIST_CHARGE_ORDERS_FAILED", err.Error())
		return
	}
	response.Paginated(c, orders, total, limit, offset)
}

// GetChargeOrderDetail handles GET /api/v1/client-auth/charge/orders/:id.
func (h *CommerceHandler) GetChargeOrderDetail(c *gin.Context) {
	clientID := getClientID(c)
	if clientID == 0 {
		response.Unauthorized(c, "UNAUTHORIZED", "client not authenticated")
		return
	}
	orderID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid order ID")
		return
	}
	order, err := h.svc.GetChargeOrderDetail(c.Request.Context(), clientID, orderID)
	if err != nil {
		response.NotFound(c, "CHARGE_ORDER_NOT_FOUND", err.Error())
		return
	}
	response.OK(c, order)
}

// RedeemCard handles POST /api/v1/client-auth/charge/redeem-card.
func (h *CommerceHandler) RedeemCard(c *gin.Context) {
	clientID := getClientID(c)
	if clientID == 0 {
		response.Unauthorized(c, "UNAUTHORIZED", "client not authenticated")
		return
	}
	var input clientauth.RedeemCardInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	order, err := h.svc.RedeemCard(c.Request.Context(), clientID, input)
	if err != nil {
		response.BadRequest(c, "REDEEM_FAILED", err.Error())
		return
	}
	response.OK(c, order)
}

// HandleChargeCallback handles POST /api/v1/charge/callback/:site_id (public endpoint).
func (h *CommerceHandler) HandleChargeCallback(c *gin.Context) {
	// ① 生成 request_id（全链路追踪 §4.10）
	requestID := c.GetHeader("X-Request-ID")
	if requestID == "" {
		requestID = generateRequestID()
	}
	c.Header("X-Request-ID", requestID)

	siteID, err := strconv.ParseInt(c.Param("site_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_SITE_ID", "invalid site ID")
		return
	}
	var input clientauth.ChargeCallbackInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	remoteIP := c.ClientIP()
	if err := h.svc.HandleChargeCallback(c.Request.Context(), siteID, remoteIP, requestID, input); err != nil {
		response.BadRequest(c, "CALLBACK_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"success": true, "message": "ok"})
}

// ============================================================
// P0-4: Balance System
// ============================================================

// GetBalance handles GET /api/v1/client-auth/balance.
func (h *CommerceHandler) GetBalance(c *gin.Context) {
	clientID := getClientID(c)
	if clientID == 0 {
		response.Unauthorized(c, "UNAUTHORIZED", "client not authenticated")
		return
	}
	balance, err := h.svc.GetBalance(c.Request.Context(), clientID)
	if err != nil {
		response.InternalError(c, "GET_BALANCE_FAILED", err.Error())
		return
	}
	response.OK(c, balance)
}

// ListBalanceTransactions handles GET /api/v1/client-auth/balance/transactions.
func (h *CommerceHandler) ListBalanceTransactions(c *gin.Context) {
	clientID := getClientID(c)
	if clientID == 0 {
		response.Unauthorized(c, "UNAUTHORIZED", "client not authenticated")
		return
	}
	txType := c.Query("type")
	limit, offset := getPagination(c)
	transactions, total, err := h.svc.ListBalanceTransactions(c.Request.Context(), clientID, txType, limit, offset)
	if err != nil {
		response.InternalError(c, "LIST_TRANSACTIONS_FAILED", err.Error())
		return
	}
	response.Paginated(c, transactions, total, limit, offset)
}

// ============================================================
// Helpers
// ============================================================

// getPagination extracts limit and offset from query parameters.
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

// generateRequestID generates a unique request ID: req_ + 12 hex chars.
func generateRequestID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return "req_" + hex.EncodeToString(b)
}
