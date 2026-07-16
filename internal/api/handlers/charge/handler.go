package charge

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"corefusion/internal/api/response"
	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// Handler handles admin charge management HTTP requests.
type Handler struct {
	chargeSiteRepo  *repository.ChargeSiteRepository
	chargeOrderRepo *repository.ChargeOrderRepository
	chargeCardRepo  *repository.ChargeCardRepository
	balanceRepo    *repository.ClientBalanceRepository
	balanceTxRepo   *repository.BalanceTransactionRepository
	clientRepo     *repository.ClientRepository
	db             *gorm.DB
}

// NewHandler creates a new charge admin handler.
func NewHandler(
	chargeSiteRepo *repository.ChargeSiteRepository,
	chargeOrderRepo *repository.ChargeOrderRepository,
	chargeCardRepo *repository.ChargeCardRepository,
	balanceRepo *repository.ClientBalanceRepository,
	balanceTxRepo *repository.BalanceTransactionRepository,
	clientRepo *repository.ClientRepository,
	db *gorm.DB,
) *Handler {
	return &Handler{
		chargeSiteRepo:  chargeSiteRepo,
		chargeOrderRepo: chargeOrderRepo,
		chargeCardRepo:  chargeCardRepo,
		balanceRepo:     balanceRepo,
		balanceTxRepo:   balanceTxRepo,
		clientRepo:      clientRepo,
		db:              db,
	}
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
	response.OK(c, site)
}

// CreateSite handles POST /api/v1/charge-sites.
func (h *Handler) CreateSite(c *gin.Context) {
	var site entity.ChargeSite
	if err := c.ShouldBindJSON(&site); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if err := h.chargeSiteRepo.Create(c.Request.Context(), &site); err != nil {
		response.InternalError(c, "CREATE_FAILED", err.Error())
		return
	}
	response.Created(c, site)
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
	if err := c.ShouldBindJSON(site); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if err := h.chargeSiteRepo.Update(c.Request.Context(), site); err != nil {
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}
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
	response.Paginated(c, items, total, limit, offset)
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

	// Update order status to refunded
	order.Status = entity.ChargeStatusRefunded
	remark := input.Remark
	if remark == "" {
		remark = "管理员退款"
	}
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
				ClientID:      order.ClientID,
				Type:          entity.BalanceTxTypeRefund,
				Amount:        order.Amount,
				BalanceAfter:  bal.Balance,
				RelatedChargeOrderID: &order.ID,
				Remark:        remark,
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
	limit, offset := getPagination(c)
	cards, total, err := h.chargeCardRepo.ListAll(c.Request.Context(), status, cardType, limit, offset)
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	response.Paginated(c, cards, total, limit, offset)
}

// CreateCard handles POST /api/v1/charge-cards.
func (h *Handler) CreateCard(c *gin.Context) {
	var card entity.ChargeCard
	if err := c.ShouldBindJSON(&card); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if card.CardCode == "" {
		card.CardCode = GenerateCardCode()
	}
	if err := h.chargeCardRepo.Create(c.Request.Context(), &card); err != nil {
		response.InternalError(c, "CREATE_FAILED", err.Error())
		return
	}
	response.Created(c, card)
}

// BatchCreateCardsInput is the input for batch creating charge cards.
type BatchCreateCardsInput struct {
	CardType  string  `json:"card_type" binding:"required"`
	Value     int64   `json:"value" binding:"required"`
	MaxUses   int     `json:"max_uses"`
	Count     int     `json:"count" binding:"required,min=1,max=1000"`
	ExpiresAt *string `json:"expires_at"`
	BatchID   string  `json:"batch_id"`
	Remark    string  `json:"remark"`
}

// BatchCreateCards handles POST /api/v1/charge-cards/batch.
func (h *Handler) BatchCreateCards(c *gin.Context) {
	var input BatchCreateCardsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if input.BatchID == "" {
		input.BatchID = generateBatchID()
	}
	if input.MaxUses == 0 {
		input.MaxUses = 1
	}

	cards := make([]entity.ChargeCard, input.Count)
	for i := 0; i < input.Count; i++ {
		cards[i] = entity.ChargeCard{
			CardCode: GenerateCardCode(),
			CardType: input.CardType,
			Value:    input.Value,
			MaxUses:  input.MaxUses,
			Status:   entity.ChargeCardStatusActive,
			BatchID:  input.BatchID,
			Remark:   input.Remark,
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
	response.Paginated(c, items, total, limit, offset)
}

// AdjustBalanceInput is the input for adjusting client balance.
type AdjustBalanceInput struct {
	ClientID int64  `json:"client_id" binding:"required"`
	Amount   int64  `json:"amount" binding:"required"` // positive=add, negative=deduct
	Remark   string `json:"remark"`
}

// AdjustBalance handles POST /api/v1/client-balances/adjust.
func (h *Handler) AdjustBalance(c *gin.Context) {
	var input AdjustBalanceInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	bal, err := h.balanceRepo.GetOrCreate(c.Request.Context(), input.ClientID)
	if err != nil {
		response.NotFound(c, "NOT_FOUND", "client not found")
		return
	}

	if input.Amount > 0 {
		bal, err = h.balanceRepo.AddBalance(c.Request.Context(), input.ClientID, input.Amount)
	} else if input.Amount < 0 {
		bal, err = h.balanceRepo.DeductBalance(c.Request.Context(), input.ClientID, -input.Amount)
	}
	if err != nil {
		response.BadRequest(c, "ADJUST_FAILED", err.Error())
		return
	}

	remark := input.Remark
	if remark == "" {
		remark = "管理员调整"
	}
	txType := entity.BalanceTxTypeCharge
	amount := input.Amount
	if input.Amount < 0 {
		txType = entity.BalanceTxTypeConsume
	}

	if input.Amount != 0 {
		txRecord := &entity.BalanceTransaction{
			ClientID: input.ClientID, Type: txType, Amount: amount,
			BalanceAfter: bal.Balance, Remark: remark,
		}
		h.balanceTxRepo.Create(c.Request.Context(), txRecord)
	}
	response.OK(c, bal)
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

// GenerateCardCode generates a unique card code in format CF-XXXX-XXXX-XXXX.
func GenerateCardCode() string {
	b := make([]byte, 6)
	rand.Read(b)
	hexStr := hex.EncodeToString(b)
	return fmt.Sprintf("CF-%s-%s-%s", hexStr[0:4], hexStr[4:8], hexStr[8:12])
}

func generateBatchID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("B%s%s", time.Now().Format("20060102"), hex.EncodeToString(b))
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
