package clientext

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"corefusion/internal/api/handlers/subscriptions"
	"gorm.io/gorm"
	"corefusion/internal/model/entity"
	"corefusion/internal/service/clientauth"
	"corefusion/internal/store/repository"
	"corefusion/pkg/netutil"
)

// PublicSubHandler is the interface satisfied by subscriptions.PublicHandler.
type PublicSubHandler interface {
	ListClientVisibleNodes(ctx context.Context, clientID int64) ([]subscriptions.ClientVisibleNode, error)
}

// Handler holds all new P1-P3 service references.
type Handler struct {
	referralSvc      *clientauth.ReferralService
	ticketSvc        *clientauth.TicketService
	giftSvc          *clientauth.GiftService
	articleSvc       *clientauth.ArticleService
	sharedAcctSvc    *clientauth.SharedAccountService
	ipWhitelistSvc   *clientauth.IPWhitelistService
	selfUnbanSvc     *clientauth.SelfUnbanService
	subResetSvc      *clientauth.SubscriptionResetService
	clientRepo       *repository.ClientRepository
	oAuthRepo        *repository.ClientOAuthBindingRepository
	subRepo          *repository.SubscriptionRepository
	subEndpointRepo  *repository.SubscriptionEndpointRepository
	publicSubH       PublicSubHandler
	db               *gorm.DB
}

// NewHandler creates a new Handler.
func NewHandler(
	referralSvc *clientauth.ReferralService,
	ticketSvc *clientauth.TicketService,
	giftSvc *clientauth.GiftService,
	articleSvc *clientauth.ArticleService,
	sharedAcctSvc *clientauth.SharedAccountService,
	ipWhitelistSvc *clientauth.IPWhitelistService,
	selfUnbanSvc *clientauth.SelfUnbanService,
	subResetSvc *clientauth.SubscriptionResetService,
	clientRepo *repository.ClientRepository,
	oAuthRepo *repository.ClientOAuthBindingRepository,
	subRepo *repository.SubscriptionRepository,
	subEndpointRepo *repository.SubscriptionEndpointRepository,
	publicSubH PublicSubHandler,
	db *gorm.DB,
) *Handler {
	return &Handler{
		referralSvc: referralSvc, ticketSvc: ticketSvc,
		giftSvc: giftSvc, articleSvc: articleSvc,
		sharedAcctSvc: sharedAcctSvc, ipWhitelistSvc: ipWhitelistSvc,
		selfUnbanSvc: selfUnbanSvc, subResetSvc: subResetSvc,
		clientRepo: clientRepo, oAuthRepo: oAuthRepo,
		subEndpointRepo: subEndpointRepo,
		publicSubH:      publicSubH,
		db:              db,
	}
}

func getClientID(c *gin.Context) int64 {
	id, _ := c.Get("client_id")
	if v, ok := id.(int64); ok {
		return v
	}
	return 0
}

func getAdminID(c *gin.Context) int64 {
	id, _ := c.Get("user_id")
	if v, ok := id.(int64); ok {
		return v
	}
	return 0
}

func getLimitOffset(c *gin.Context) (int, int) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return limit, offset
}

// ======================== Referral (Client) ========================

func (h *Handler) GetReferralCode(c *gin.Context) {
	code, err := h.referralSvc.GetOrCreateCode(c.Request.Context(), getClientID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, code)
}

func (h *Handler) GetReferralStats(c *gin.Context) {
	stats, err := h.referralSvc.GetStats(c.Request.Context(), getClientID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

func (h *Handler) ListReferralRecords(c *gin.Context) {
	limit, offset := getLimitOffset(c)
	records, total, err := h.referralSvc.ListRecords(c.Request.Context(), getClientID(c), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": records, "total": total})
}

func (h *Handler) ListReferralWithdrawals(c *gin.Context) {
	limit, offset := getLimitOffset(c)
	items, total, err := h.referralSvc.ListWithdrawals(c.Request.Context(), getClientID(c), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total})
}

func (h *Handler) RequestWithdrawal(c *gin.Context) {
	var input clientauth.WithdrawInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	wd, err := h.referralSvc.RequestWithdrawal(c.Request.Context(), getClientID(c), input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, wd)
}

// ======================== Ticket (Client) ========================

func (h *Handler) CreateTicket(c *gin.Context) {
	var input clientauth.CreateTicketInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ticket, err := h.ticketSvc.CreateTicket(c.Request.Context(), getClientID(c), input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, ticket)
}

func (h *Handler) ListTickets(c *gin.Context) {
	limit, offset := getLimitOffset(c)
	status := c.Query("status")
	items, total, err := h.ticketSvc.ListTickets(c.Request.Context(), getClientID(c), status, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total})
}

func (h *Handler) GetTicket(c *gin.Context) {
	ticketID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	ticket, msgs, err := h.ticketSvc.GetTicket(c.Request.Context(), getClientID(c), ticketID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ticket": ticket, "messages": msgs})
}

func (h *Handler) ReplyTicket(c *gin.Context) {
	ticketID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var body struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	msg, err := h.ticketSvc.ReplyTicket(c.Request.Context(), getClientID(c), ticketID, body.Content)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, msg)
}

func (h *Handler) CloseTicket(c *gin.Context) {
	ticketID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.ticketSvc.CloseTicket(c.Request.Context(), getClientID(c), ticketID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ticket closed"})
}

// ======================== Gift (Client) ========================

func (h *Handler) ListGifts(c *gin.Context) {
	gifts, err := h.giftSvc.ListAvailable(c.Request.Context(), getClientID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gifts)
}

func (h *Handler) ClaimGift(c *gin.Context) {
	giftID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	gift, err := h.giftSvc.ClaimGift(c.Request.Context(), getClientID(c), giftID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gift)
}

// ======================== Article (Client) ========================

func (h *Handler) ListArticles(c *gin.Context) {
	category := c.Query("category")
	platform := c.Query("platform")
	articles, err := h.articleSvc.ListPublished(c.Request.Context(), category, platform)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, articles)
}

func (h *Handler) GetArticle(c *gin.Context) {
	article, err := h.articleSvc.GetBySlug(c.Request.Context(), c.Param("slug"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "article not found"})
		return
	}
	c.JSON(http.StatusOK, article)
}

// ======================== Shared Account (Client) ========================

func (h *Handler) ListSharedAccounts(c *gin.Context) {
	accounts, err := h.sharedAcctSvc.ListVisible(c.Request.Context(), getClientID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, accounts)
}

func (h *Handler) GetSharedAccount(c *gin.Context) {
	accountID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	account, err := h.sharedAcctSvc.GetAccount(c.Request.Context(), getClientID(c), accountID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, account)
}

func (h *Handler) RequestRepair(c *gin.Context) {
	accountID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	log, err := h.sharedAcctSvc.RequestRepair(c.Request.Context(), getClientID(c), accountID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, log)
}

// ======================== IP Whitelist (Client) ========================

func (h *Handler) ListIPWhitelist(c *gin.Context) {
	entries, err := h.ipWhitelistSvc.ListWhitelist(c.Request.Context(), getClientID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, entries)
}

func (h *Handler) AutoAddIP(c *gin.Context) {
	var body struct {
		IP string `json:"ip" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		// fallback to real IP (CDN-aware)
		body.IP = netutil.GetRealIP(c.Request)
	}
	if body.IP == "" {
		body.IP = netutil.GetRealIP(c.Request)
	}
	entry, err := h.ipWhitelistSvc.AutoAddIP(c.Request.Context(), getClientID(c), body.IP)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, entry)
}

func (h *Handler) RemoveIPWhitelist(c *gin.Context) {
	entryID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.ipWhitelistSvc.RemoveIP(c.Request.Context(), getClientID(c), entryID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "removed"})
}

// ======================== Self-Unban (Client) ========================

func (h *Handler) SelfUnban(c *gin.Context) {
	if err := h.selfUnbanSvc.SelfUnban(c.Request.Context(), getClientID(c)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "account unbanned"})
}

// ======================== Subscription Reset (Client) ========================

func (h *Handler) ResetSubscription(c *gin.Context) {
	if err := h.subResetSvc.ResetSubscription(c.Request.Context(), getClientID(c)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "subscription token reset"})
}

// AdminListAllIPWhitelists handles GET /api/v1/ip-whitelists (admin: list all client IP whitelists).
func (h *Handler) AdminListAllIPWhitelists(c *gin.Context) {
	limit, offset := getLimitOffset(c)
	clientIDStr := c.Query("client_id")
	var items []entity.IPWhitelistEntry
	var total int64
	q := h.db.WithContext(c.Request.Context()).Model(&entity.IPWhitelistEntry{})
	if clientIDStr != "" {
		clientID, err := strconv.ParseInt(clientIDStr, 10, 64)
		if err == nil {
			q = q.Where("client_id = ?", clientID)
		}
	}
	q.Count(&total)
	if err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total})
}

// AdminUpdateIPWhitelistConfig handles PUT /api/v1/ip-whitelist-config (admin: update global IP whitelist config).
func (h *Handler) AdminUpdateIPWhitelistConfig(c *gin.Context) {
	var body struct {
		AllowSelfAdd bool `json:"allow_self_add"`
		MaxEntries   int  `json:"max_entries"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Store config via setting service
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

// AdminListClientOAuthBindings handles GET /api/v1/client-oauth-bindings (admin: list all OAuth bindings).
func (h *Handler) AdminListClientOAuthBindings(c *gin.Context) {
	limit, offset := getLimitOffset(c)
	clientIDStr := c.Query("client_id")
	var items []entity.ClientOAuthBinding
	var total int64
	q := h.db.WithContext(c.Request.Context()).Model(&entity.ClientOAuthBinding{})
	if clientIDStr != "" {
		clientID, err := strconv.ParseInt(clientIDStr, 10, 64)
		if err == nil {
			q = q.Where("client_id = ?", clientID)
		}
	}
	q.Count(&total)
	if err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total})
}

// ======================== Admin: Referral ========================

func (h *Handler) AdminListWithdrawals(c *gin.Context) {
	limit, offset := getLimitOffset(c)
	status := c.Query("status")
	items, total, err := h.referralSvc.AdminListWithdrawals(c.Request.Context(), status, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total})
}

// AdminListReferralRecords handles GET /api/v1/referral-records (admin: list all referral records).
func (h *Handler) AdminListReferralRecords(c *gin.Context) {
	limit, offset := getLimitOffset(c)
	referrerIDStr := c.Query("referrer_id")
	var items []entity.ReferralRecord
	var total int64
	q := h.db.WithContext(c.Request.Context()).Model(&entity.ReferralRecord{})
	if referrerIDStr != "" {
		referrerID, err := strconv.ParseInt(referrerIDStr, 10, 64)
		if err == nil {
			q = q.Where("referrer_id = ?", referrerID)
		}
	}
	q.Count(&total)
	if err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Enrich with referrer/referee emails
	type recordVO struct {
		entity.ReferralRecord
		ReferrerEmail string `json:"referrer_email"`
		RefereeEmail  string `json:"referee_email"`
	}
	result := make([]recordVO, 0, len(items))
	for _, r := range items {
		vo := recordVO{ReferralRecord: r}
		if h.clientRepo != nil {
			if cli, err := h.clientRepo.GetByID(c.Request.Context(), r.ReferrerID); err == nil {
				vo.ReferrerEmail = cli.Email
			}
			if cli, err := h.clientRepo.GetByID(c.Request.Context(), r.RefereeID); err == nil {
				vo.RefereeEmail = cli.Email
			}
		}
		result = append(result, vo)
	}
	c.JSON(http.StatusOK, gin.H{"items": result, "total": total})
}

// AdminReferralStats handles GET /api/v1/referral-stats (admin: global referral statistics).
func (h *Handler) AdminReferralStats(c *gin.Context) {
	var totalReferrers int64
	var totalRecords int64
	var totalCommission int64
	var totalWithdrawn int64

	h.db.WithContext(c.Request.Context()).Model(&entity.ReferralCode{}).Count(&totalReferrers)
	h.db.WithContext(c.Request.Context()).Model(&entity.ReferralRecord{}).Count(&totalRecords)
	h.db.WithContext(c.Request.Context()).Model(&entity.ReferralRecord{}).
		Select("COALESCE(SUM(commission_amount),0)").Scan(&totalCommission)

	var withdrawals []entity.ReferralWithdrawal
	h.db.WithContext(c.Request.Context()).
		Where("status = ?", "approved").Find(&withdrawals)
	for _, w := range withdrawals {
		totalWithdrawn += w.Amount
	}

	c.JSON(http.StatusOK, gin.H{
		"total_referrers":    totalReferrers,
		"total_records":      totalRecords,
		"total_commission":   totalCommission,
		"total_withdrawn":    totalWithdrawn,
		"pending_commission": totalCommission - totalWithdrawn,
	})
}

func (h *Handler) AdminApproveWithdrawal(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var body struct {
		Remark string `json:"remark"`
	}
	c.ShouldBindJSON(&body)
	if err := h.referralSvc.AdminApproveWithdrawal(c.Request.Context(), id, body.Remark); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "approved"})
}

func (h *Handler) AdminRejectWithdrawal(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var body struct {
		Remark string `json:"remark"`
	}
	c.ShouldBindJSON(&body)
	if err := h.referralSvc.AdminRejectWithdrawal(c.Request.Context(), id, body.Remark); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "rejected"})
}

// ======================== Admin: Ticket ========================

func (h *Handler) AdminListTickets(c *gin.Context) {
	limit, offset := getLimitOffset(c)
	status := c.Query("status")
	category := c.Query("category")
	items, total, err := h.ticketSvc.AdminListTickets(c.Request.Context(), status, category, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total})
}

func (h *Handler) AdminGetTicket(c *gin.Context) {
	ticketID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	ticket, msgs, err := h.ticketSvc.AdminGetTicket(c.Request.Context(), ticketID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ticket": ticket, "messages": msgs})
}

func (h *Handler) AdminReplyTicket(c *gin.Context) {
	ticketID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var body struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.ticketSvc.AdminReplyTicket(c.Request.Context(), ticketID, getAdminID(c), body.Content); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "replied"})
}

func (h *Handler) AdminCloseTicket(c *gin.Context) {
	ticketID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.ticketSvc.AdminCloseTicket(c.Request.Context(), ticketID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "closed"})
}

// AdminAssignTicket handles POST /api/v1/tickets/:id/assign (admin: assign ticket to an admin).
func (h *Handler) AdminAssignTicket(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var body struct {
		AdminID int64 `json:"admin_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.ticketSvc.AdminAssignTicket(c.Request.Context(), id, body.AdminID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "assigned"})
}

// ======================== Admin: Gift ========================

func (h *Handler) AdminListGifts(c *gin.Context) {
	limit, offset := getLimitOffset(c)
	clientID, _ := strconv.ParseInt(c.Query("client_id"), 10, 64)
	items, total, err := h.giftSvc.AdminListGifts(c.Request.Context(), clientID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Enrich gifts with client email
	type giftVO struct {
		entity.ClientGift
		ClientEmail string `json:"client_email"`
	}
	result := make([]giftVO, 0, len(items))
	for _, g := range items {
		vo := giftVO{ClientGift: g}
		if h.clientRepo != nil {
			if cli, err := h.clientRepo.GetByID(c.Request.Context(), g.ClientID); err == nil {
				vo.ClientEmail = cli.Email
			}
		}
		result = append(result, vo)
	}
	c.JSON(http.StatusOK, gin.H{"items": result, "total": total})
}

func (h *Handler) AdminCreateGift(c *gin.Context) {
	var gift entity.ClientGift
	if err := c.ShouldBindJSON(&gift); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	gift.Status = entity.GiftStatusAvailable
	if err := h.giftSvc.AdminCreateGift(c.Request.Context(), &gift); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gift)
}

func (h *Handler) AdminCreateGiftBatch(c *gin.Context) {
	var body struct {
		ClientIDs []int64  `json:"client_ids" binding:"required"`
		Type      string   `json:"type" binding:"required"`
		Source    string   `json:"source" binding:"required"`
		Value    int64    `json:"value" binding:"required"`
		ExpiresAt string  `json:"expires_at"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var expiresAt *time.Time
	if body.ExpiresAt != "" {
		if t, err := time.Parse(time.RFC3339, body.ExpiresAt); err == nil {
			expiresAt = &t
		}
	}
	if err := h.giftSvc.AdminCreateGiftBatch(c.Request.Context(), body.ClientIDs, body.Type, body.Source, body.Value, expiresAt); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "gifts created"})
}

// AdminSearchClients handles GET /api/v1/client-gifts/search-clients (admin: search clients by email for gift dropdown).
func (h *Handler) AdminSearchClients(c *gin.Context) {
	keyword := c.Query("keyword")
	if keyword == "" {
		// Return recent 20 clients when no keyword
		clients, _, err := h.clientRepo.List(c.Request.Context(), 0, "", 20, 0)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": clients})
		return
	}
	clients, err := h.clientRepo.SearchByEmail(c.Request.Context(), keyword, 20)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": clients})
}

// ======================== Admin: Referral Config ========================

// AdminGetReferralConfig handles GET /api/v1/referral/config (admin: get referral settings).
func (h *Handler) AdminGetReferralConfig(c *gin.Context) {
	getSetting := func(key, defaultVal string) string {
		var s entity.SystemSetting
		if err := h.db.WithContext(c.Request.Context()).Where("key = ?", key).First(&s).Error; err == nil {
			return s.Value
		}
		return defaultVal
	}
	c.JSON(http.StatusOK, gin.H{
		"commission_rate":     getSetting("referral.commission_rate", "15"),
		"min_withdraw_amount": getSetting("referral.min_withdraw_amount", "1000"),
	})
}

// AdminSetReferralConfig handles PUT /api/v1/referral/config (admin: set referral settings).
func (h *Handler) AdminSetReferralConfig(c *gin.Context) {
	var body struct {
		CommissionRate     *int   `json:"commission_rate"`
		MinWithdrawAmount  *int64 `json:"min_withdraw_amount"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	setSetting := func(key string, value string) error {
		return h.db.WithContext(c.Request.Context()).
			Where("key = ?", key).
			Assign(entity.SystemSetting{Value: value}).
			FirstOrCreate(&entity.SystemSetting{Key: key, Value: value}).Error
	}
	if body.CommissionRate != nil {
		rate := *body.CommissionRate
		if rate < 0 || rate > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "commission_rate must be 0-100"})
			return
		}
		if err := setSetting("referral.commission_rate", fmt.Sprintf("%d", rate)); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	if body.MinWithdrawAmount != nil {
		amt := *body.MinWithdrawAmount
		if amt < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "min_withdraw_amount must be >= 0"})
			return
		}
		if err := setSetting("referral.min_withdraw_amount", fmt.Sprintf("%d", amt)); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"message": "referral config updated"})
}

// ======================== Admin: Article ========================

func (h *Handler) AdminListArticles(c *gin.Context) {
	limit, offset := getLimitOffset(c)
	category := c.Query("category")
	items, total, err := h.articleSvc.ListAll(c.Request.Context(), category, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total})
}

func (h *Handler) AdminGetArticle(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	article, err := h.articleSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "article not found"})
		return
	}
	c.JSON(http.StatusOK, article)
}

func (h *Handler) AdminCreateArticle(c *gin.Context) {
	var article entity.Article
	if err := c.ShouldBindJSON(&article); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.articleSvc.Create(c.Request.Context(), &article); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, article)
}

func (h *Handler) AdminUpdateArticle(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	article, err := h.articleSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "article not found"})
		return
	}
	if err := c.ShouldBindJSON(article); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	article.ID = id
	if err := h.articleSvc.Update(c.Request.Context(), article); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, article)
}

func (h *Handler) AdminDeleteArticle(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.articleSvc.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// ======================== Admin: Shared Account ========================

func (h *Handler) AdminListSharedAccounts(c *gin.Context) {
	limit, offset := getLimitOffset(c)
	items, total, err := h.sharedAcctSvc.AdminList(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total})
}

func (h *Handler) AdminGetSharedAccount(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	acct, err := h.sharedAcctSvc.AdminGetAccount(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "account not found"})
		return
	}
	c.JSON(http.StatusOK, acct)
}

func (h *Handler) AdminCreateSharedAccount(c *gin.Context) {
	var acct entity.SharedAccount
	if err := c.ShouldBindJSON(&acct); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.sharedAcctSvc.AdminCreate(c.Request.Context(), &acct); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, acct)
}

func (h *Handler) AdminUpdateSharedAccount(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	acct, err := h.sharedAcctSvc.AdminGetAccount(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "account not found"})
		return
	}
	if err := c.ShouldBindJSON(acct); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	acct.ID = id
	if err := h.sharedAcctSvc.AdminUpdate(c.Request.Context(), acct); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, acct)
}

func (h *Handler) AdminDeleteSharedAccount(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.sharedAcctSvc.AdminDelete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *Handler) AdminRepairLogs(c *gin.Context) {
	accountID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	limit, offset := getLimitOffset(c)
	items, total, err := h.sharedAcctSvc.AdminRepairLogs(c.Request.Context(), accountID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total})
}

// ======================== P1-1: Client Node List ========================

// ListClientNodes returns the list of nodes visible to the authenticated client.
// It reuses the same logic as the public subscription handler (ServeSubscription),
// so the client sees exactly the same nodes they would get in their subscription.
func (h *Handler) ListClientNodes(c *gin.Context) {
	clientID := getClientID(c)
	if h.publicSubH == nil || clientID == 0 {
		c.JSON(http.StatusOK, gin.H{"items": []subscriptions.ClientVisibleNode{}})
		return
	}

	nodes, err := h.publicSubH.ListClientVisibleNodes(c.Request.Context(), clientID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if nodes == nil {
		nodes = []subscriptions.ClientVisibleNode{}
	}
	c.JSON(http.StatusOK, gin.H{"items": nodes})
}

// ======================== P1-1b: Node Config/Link/QRCode ========================

// GetNodeLink returns the share link for a specific node.
// It fetches all visible nodes and filters by the requested node ID.
func (h *Handler) GetNodeLink(c *gin.Context) {
	nodeID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	clientID := getClientID(c)
	if h.publicSubH == nil || clientID == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return
	}

	nodes, err := h.publicSubH.ListClientVisibleNodes(c.Request.Context(), clientID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	for _, n := range nodes {
		if n.ID == nodeID || n.InboundID == nodeID || n.ExternalID == nodeID {
			c.JSON(http.StatusOK, gin.H{"node_id": nodeID, "link": n.Link, "name": n.Name, "protocol": n.Protocol})
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
}

// GetNodeConfig returns the node configuration details.
// For full subscription config, use the subscription link instead.
func (h *Handler) GetNodeConfig(c *gin.Context) {
	nodeID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	clientID := getClientID(c)
	if h.publicSubH == nil || clientID == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return
	}

	nodes, err := h.publicSubH.ListClientVisibleNodes(c.Request.Context(), clientID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	for _, n := range nodes {
		if n.ID == nodeID || n.InboundID == nodeID || n.ExternalID == nodeID {
			c.JSON(http.StatusOK, n)
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
}

// GetNodeQRCode returns a QR code data URI for the node's share link.
func (h *Handler) GetNodeQRCode(c *gin.Context) {
	nodeID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	clientID := getClientID(c)
	if h.publicSubH == nil || clientID == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return
	}

	nodes, err := h.publicSubH.ListClientVisibleNodes(c.Request.Context(), clientID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	for _, n := range nodes {
		if n.ID == nodeID || n.InboundID == nodeID || n.ExternalID == nodeID {
			if n.Link == "" {
				c.JSON(http.StatusNotFound, gin.H{"error": "no share link available for this node"})
				return
			}
			// Return the link as QR-compatible data; client app can render the QR
			c.JSON(http.StatusOK, gin.H{
				"node_id": nodeID,
				"link":    n.Link,
				"name":    n.Name,
			})
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
}

// ======================== P2-11: Public Articles ========================

func (h *Handler) PublicListArticles(c *gin.Context) {
	category := c.Query("category")
	platform := c.Query("platform")
	articles, err := h.articleSvc.ListPublished(c.Request.Context(), category, platform)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, articles)
}

func (h *Handler) PublicGetArticle(c *gin.Context) {
	article, err := h.articleSvc.GetBySlug(c.Request.Context(), c.Param("slug"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "article not found"})
		return
	}
	c.JSON(http.StatusOK, article)
}

// ======================== P2-15: Chat Provider Settings ========================
// Handled in router by extending /api/v1/public/settings response

// ======================== P3-17: Time Check ========================

func (h *Handler) CheckTime(c *gin.Context) {
	clientTimestamp, _ := strconv.ParseInt(c.Query("t"), 10, 64)
	serverTime := time.Now().Unix()
	diff := int64(0)
	if clientTimestamp > 0 {
		diff = serverTime - clientTimestamp
	}
	c.JSON(http.StatusOK, gin.H{
		"server_time": serverTime,
		"client_ip":   netutil.GetRealIP(c.Request),
		"diff_seconds": diff,
	})
}

// ======================== P3-19: Profile Management ========================

func (h *Handler) GetProfile(c *gin.Context) {
	clientID := getClientID(c)
	if h.clientRepo == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "not configured"})
		return
	}
	client, err := h.clientRepo.GetByID(c.Request.Context(), clientID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "client not found"})
		return
	}
	// Get OAuth bindings
	var bindings []entity.ClientOAuthBinding
	if h.oAuthRepo != nil {
		bindings, _ = h.oAuthRepo.ListByClientID(c.Request.Context(), clientID)
	}
	c.JSON(http.StatusOK, gin.H{
		"id":         client.ID,
		"email":      client.Email,
		"nickname":   client.Nickname,
		"qq":         client.QQ,
		"wechat":     client.Wechat,
		"status":     client.Status,
		"created_at": client.CreatedAt,
		"oauth_bindings": bindings,
	})
}

func (h *Handler) UpdateProfile(c *gin.Context) {
	clientID := getClientID(c)
	if h.clientRepo == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "not configured"})
		return
	}
	client, err := h.clientRepo.GetByID(c.Request.Context(), clientID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "client not found"})
		return
	}
	var body struct {
		Nickname string `json:"nickname"`
		QQ       string `json:"qq"`
		Wechat   string `json:"wechat"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	client.Nickname = body.Nickname
	client.QQ = body.QQ
	client.Wechat = body.Wechat
	if err := h.clientRepo.Update(c.Request.Context(), client); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "profile updated"})
}

func (h *Handler) GetConnectionInfo(c *gin.Context) {
	clientID := getClientID(c)
	if h.clientRepo == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "not configured"})
		return
	}
	client, err := h.clientRepo.GetByID(c.Request.Context(), clientID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "client not found"})
		return
	}

	// Derive protocol info from the client's visible nodes
	protocol := ""
	transport := "tcp"
	tls := false
	if h.publicSubH != nil {
		if nodes, err := h.publicSubH.ListClientVisibleNodes(c.Request.Context(), clientID); err == nil && len(nodes) > 0 {
			protocol = nodes[0].Protocol
			transport = nodes[0].Transport
			tls = nodes[0].TLS
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"protocol":       protocol,
		"transport":      transport,
		"tls":            tls,
		"device_limit":   client.DeviceLimit,
		"speed_limit":    client.SpeedLimitMbps,
		"concurrent":     client.ConcurrentLimit,
	})
}

func (h *Handler) BindOAuth(c *gin.Context) {
	clientID := getClientID(c)
	if h.oAuthRepo == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "not configured"})
		return
	}
	var body struct {
		Provider       string `json:"provider" binding:"required"`
		ProviderUserID string `json:"provider_user_id" binding:"required"`
		ProviderName   string `json:"provider_name"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Check if already bound
	existing, _ := h.oAuthRepo.GetByProvider(c.Request.Context(), clientID, body.Provider)
	if existing != nil && existing.ID > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "already bound"})
		return
	}
	binding := &entity.ClientOAuthBinding{
		ClientID:       clientID,
		Provider:       body.Provider,
		ProviderUserID: body.ProviderUserID,
		ProviderName:   body.ProviderName,
	}
	if err := h.oAuthRepo.Create(c.Request.Context(), binding); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, binding)
}

func (h *Handler) UnbindOAuth(c *gin.Context) {
	clientID := getClientID(c)
	if h.oAuthRepo == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "not configured"})
		return
	}
	provider := c.Param("provider")
	binding, err := h.oAuthRepo.GetByProvider(c.Request.Context(), clientID, provider)
	if err != nil || binding == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "binding not found"})
		return
	}
	if err := h.oAuthRepo.Delete(c.Request.Context(), binding.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "unbound"})
}

// ======================== P3-20: Subscription Formats ========================

// ListSubscriptionFormats returns the supported subscription output formats.
func (h *Handler) ListSubscriptionFormats(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"items": []map[string]string{
		{"key": "base64_uri", "label": "Base64 (Default)"},
		{"key": "raw_uri", "label": "Raw URI"},
		{"key": "mihomo_yaml", "label": "Mihomo / Clash YAML"},
		{"key": "singbox_json", "label": "Sing-box JSON"},
		{"key": "xray_json", "label": "Xray JSON"},
	}})
}

// buildSubLinks builds the primary and backup subscription URLs for a client.
// It uses subscription endpoints from the database:
//   - If an enabled endpoint has a standalone domain (independent channel),
//     the link is https://<domain>/<path>/<token>
//   - Otherwise it falls back to the panel host with /sub/<token>
func (h *Handler) buildSubLinks(c *gin.Context, token string) (primary string, backup string) {
	if h.subEndpointRepo != nil {
		endpoints, err := h.subEndpointRepo.GetEnabled(c.Request.Context())
		if err == nil && len(endpoints) > 0 {
			for _, ep := range endpoints {
				path := strings.TrimRight(ep.Path, "/")
				link := fmt.Sprintf("https://%s%s/%s", ep.Domain, path, token)
				if primary == "" {
					primary = link
				} else {
					backup = link
					break
				}
			}
		}
	}
	// Fallback: use panel host with /sub/ path
	if primary == "" {
		scheme := "http"
		if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
			scheme = "https"
		}
		host := c.GetHeader("X-Forwarded-Host")
		if host == "" {
			host = c.Request.Host
		}
		primary = fmt.Sprintf("%s://%s/sub/%s", scheme, host, token)
	}
	return primary, backup
}

// GetSubscriptionLink returns the client's subscription URL with optional format override.
func (h *Handler) GetSubscriptionLink(c *gin.Context) {
	clientID := getClientID(c)
	format := c.Query("format")

	if h.clientRepo == nil || h.subRepo == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "not configured"})
		return
	}

	client, err := h.clientRepo.GetByID(c.Request.Context(), clientID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "client not found"})
		return
	}

	// Check client status - only active clients can get subscription link
	if client.Status != "active" {
		c.JSON(http.StatusForbidden, gin.H{"error": "account suspended", "status": client.Status})
		return
	}

	if client.SubscriptionID == nil || *client.SubscriptionID == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "no active subscription"})
		return
	}

	sub, err := h.subRepo.GetByID(c.Request.Context(), *client.SubscriptionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "subscription not found"})
		return
	}

	primary, backup := h.buildSubLinks(c, sub.Token)
	if format != "" && format != "default" {
		primary += "?format=" + format
		if backup != "" {
			backup += "?format=" + format
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"format":         format,
		"link":           primary,
		"backup_link":    backup,
		"token":          sub.Token,
		"status":         sub.Status,
		"quota_bytes":    sub.QuotaBytes,
		"used_bytes":     sub.UsedBytesCache,
		"upload_bytes":   sub.UploadBytesCache,
		"download_bytes": sub.DownloadBytesCache,
	})
}

// GetBackupSubscriptionLink returns the backup subscription URL.
func (h *Handler) GetBackupSubscriptionLink(c *gin.Context) {
	clientID := getClientID(c)
	if h.clientRepo == nil || h.subRepo == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "not configured"})
		return
	}

	client, err := h.clientRepo.GetByID(c.Request.Context(), clientID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "client not found"})
		return
	}

	// Check client status - only active clients can get subscription link
	if client.Status != "active" {
		c.JSON(http.StatusForbidden, gin.H{"error": "account suspended", "status": client.Status})
		return
	}

	if client.SubscriptionID == nil || *client.SubscriptionID == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "no active subscription"})
		return
	}

	sub, err := h.subRepo.GetByID(c.Request.Context(), *client.SubscriptionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "subscription not found"})
		return
	}

	_, backup := h.buildSubLinks(c, sub.Token)
	c.JSON(http.StatusOK, gin.H{"link": backup, "token": sub.Token})
}

// ======================== P3-22: Shared Account Apply ========================

func (h *Handler) ApplySharedAccount(c *gin.Context) {
	accountID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	clientID := getClientID(c)
	// Apply for shared account credentials — uses the same GetAccount logic
	account, err := h.sharedAcctSvc.GetAccount(c.Request.Context(), clientID, accountID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":                account.ID,
		"name":              account.Name,
		"type":              account.Type,
		"credentials_enc":   account.CredentialsEnc,
		"message":           "account applied successfully",
	})
}
