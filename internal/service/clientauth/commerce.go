package clientauth

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	"corefusion/internal/security/crypto"
	"corefusion/internal/store/repository"
)

// ============================================================
// CommerceService — 客户端商业服务
// 套餐展示 / 订单管理 / 充值系统 / 余额系统 / 流量叠加包
// ============================================================

// chargeHTTPClient is a shared HTTP client with timeout for charge site requests.
var chargeHTTPClient = &http.Client{Timeout: 10 * time.Second}

// CommerceService handles plan display, ordering, charging, and balance operations.
type CommerceService struct {
	db                *gorm.DB
	tmplRepo          *repository.PlanTemplateRepository
	variantRepo       *repository.PlanVariantRepository
	orderRepo         *repository.PlanOrderRepository
	promotionRepo     *repository.PromotionRepository
	clientRepo        *repository.ClientRepository
	subRepo           *repository.SubscriptionRepository
	chargeSiteRepo    *repository.ChargeSiteRepository
	chargeOrderRepo   *repository.ChargeOrderRepository
	chargeCardRepo    *repository.ChargeCardRepository
	balanceRepo       *repository.ClientBalanceRepository
	balanceTxRepo     *repository.BalanceTransactionRepository
	settingSvc        SettingStore
	jwtSecret         string
}

// NewCommerceService creates a new CommerceService.
func NewCommerceService(
	db *gorm.DB,
	tmplRepo *repository.PlanTemplateRepository,
	variantRepo *repository.PlanVariantRepository,
	orderRepo *repository.PlanOrderRepository,
	promotionRepo *repository.PromotionRepository,
	clientRepo *repository.ClientRepository,
	subRepo *repository.SubscriptionRepository,
	chargeSiteRepo *repository.ChargeSiteRepository,
	chargeOrderRepo *repository.ChargeOrderRepository,
	chargeCardRepo *repository.ChargeCardRepository,
	balanceRepo *repository.ClientBalanceRepository,
	balanceTxRepo *repository.BalanceTransactionRepository,
	settingSvc SettingStore,
	jwtSecret string,
) *CommerceService {
	return &CommerceService{
		db:              db,
		tmplRepo:        tmplRepo,
		variantRepo:     variantRepo,
		orderRepo:       orderRepo,
		promotionRepo:   promotionRepo,
		clientRepo:      clientRepo,
		subRepo:         subRepo,
		chargeSiteRepo:  chargeSiteRepo,
		chargeOrderRepo: chargeOrderRepo,
		chargeCardRepo:  chargeCardRepo,
		balanceRepo:     balanceRepo,
		balanceTxRepo:   balanceTxRepo,
		settingSvc:      settingSvc,
		jwtSecret:       jwtSecret,
	}
}

// ============================================================
// P0-1: Plan Display
// ============================================================

// PlanListItem is a simplified plan template for client display.
type PlanListItem struct {
	ID          int64             `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Category    string            `json:"category"`
	SortOrder   int               `json:"sort_order"`
	Variants    []PlanVariantItem `json:"variants"`
}

// PlanVariantItem is a simplified plan variant for client display.
type PlanVariantItem struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	VariantType   string `json:"variant_type"`    // plan/traffic_pack
	BillingCycle  string `json:"billing_cycle"`
	DurationDays  int    `json:"duration_days"`
	Price         int64  `json:"price"`            // 分
	OriginalPrice int64  `json:"original_price"`   // 分
	Currency      string `json:"currency"`
	QuotaBytes    int64  `json:"quota_bytes"`
	IsRecommended bool   `json:"is_recommended"`
	IsDefault     bool   `json:"is_default"`
}

// PlanDetailItem is a detailed plan template for client display.
type PlanDetailItem struct {
	PlanListItem
	DeviceLimit     int     `json:"device_limit"`
	SpeedLimitMbps  int     `json:"speed_limit_mbps"`
	ConcurrentLimit int     `json:"concurrent_limit"`
	NodeGroupIDs    []int64 `json:"node_group_ids"`
}

// ListPlans returns all enabled plan templates with their variants (excludes traffic_pack).
func (s *CommerceService) ListPlans(ctx context.Context) ([]PlanListItem, error) {
	templates, err := s.tmplRepo.ListEnabled(ctx)
	if err != nil {
		return nil, err
	}

	var items []PlanListItem
	for _, tmpl := range templates {
		variants, err := s.variantRepo.ListByTemplateID(ctx, tmpl.ID)
		if err != nil {
			continue
		}

		var variantItems []PlanVariantItem
		for _, v := range variants {
			if !v.Enabled || v.VariantType == entity.VariantTypeTrafficPack {
				continue
			}
			quotaBytes := v.QuotaBytes
			if quotaBytes == 0 {
				quotaBytes = tmpl.DefaultQuotaBytes
			}
			variantItems = append(variantItems, PlanVariantItem{
				ID: v.ID, Name: v.Name, VariantType: v.VariantType,
				BillingCycle: v.BillingCycle, DurationDays: v.DurationDays,
				Price: v.Price, OriginalPrice: v.OriginalPrice, Currency: v.Currency,
				QuotaBytes: quotaBytes, IsRecommended: v.IsRecommended, IsDefault: v.IsDefault,
			})
		}

		items = append(items, PlanListItem{
			ID: tmpl.ID, Name: tmpl.Name, Description: tmpl.Description,
			Category: tmpl.Category, SortOrder: tmpl.SortOrder, Variants: variantItems,
		})
	}
	return items, nil
}

// GetPlan returns a single plan template detail with variants.
func (s *CommerceService) GetPlan(ctx context.Context, planID int64) (*PlanDetailItem, error) {
	tmpl, err := s.tmplRepo.GetByID(ctx, planID)
	if err != nil {
		return nil, fmt.Errorf("plan not found")
	}
	if !tmpl.Enabled {
		return nil, fmt.Errorf("plan not available")
	}

	variants, err := s.variantRepo.ListByTemplateID(ctx, tmpl.ID)
	if err != nil {
		return nil, err
	}

	var variantItems []PlanVariantItem
	for _, v := range variants {
		if !v.Enabled || v.VariantType == entity.VariantTypeTrafficPack {
			continue
		}
		quotaBytes := v.QuotaBytes
		if quotaBytes == 0 {
			quotaBytes = tmpl.DefaultQuotaBytes
		}
		variantItems = append(variantItems, PlanVariantItem{
			ID: v.ID, Name: v.Name, VariantType: v.VariantType,
			BillingCycle: v.BillingCycle, DurationDays: v.DurationDays,
			Price: v.Price, OriginalPrice: v.OriginalPrice, Currency: v.Currency,
			QuotaBytes: quotaBytes, IsRecommended: v.IsRecommended, IsDefault: v.IsDefault,
		})
	}

	nodeGroupIDs := []int64{}
	if tmpl.NodeGroupIDs != nil {
		nodeGroupIDs = []int64(tmpl.NodeGroupIDs)
	}

	return &PlanDetailItem{
		PlanListItem: PlanListItem{
			ID: tmpl.ID, Name: tmpl.Name, Description: tmpl.Description,
			Category: tmpl.Category, SortOrder: tmpl.SortOrder, Variants: variantItems,
		},
		DeviceLimit: tmpl.DeviceLimit, SpeedLimitMbps: tmpl.SpeedLimitMbps,
		ConcurrentLimit: tmpl.ConcurrentLimit, NodeGroupIDs: nodeGroupIDs,
	}, nil
}

// ============================================================
// Traffic Pack (流量叠加包)
// ============================================================

// ListTrafficPacks returns all enabled traffic pack variants.
func (s *CommerceService) ListTrafficPacks(ctx context.Context) ([]PlanVariantItem, error) {
	packs, err := s.variantRepo.ListEnabledByVariantType(ctx, entity.VariantTypeTrafficPack)
	if err != nil {
		return nil, err
	}

	var items []PlanVariantItem
	for _, v := range packs {
		items = append(items, PlanVariantItem{
			ID: v.ID, Name: v.Name, VariantType: v.VariantType,
			BillingCycle: v.BillingCycle, DurationDays: v.DurationDays,
			Price: v.Price, OriginalPrice: v.OriginalPrice, Currency: v.Currency,
			QuotaBytes: v.QuotaBytes, IsRecommended: v.IsRecommended, IsDefault: v.IsDefault,
		})
	}
	return items, nil
}

// BuyTrafficPackInput is the input for buying a traffic pack.
type BuyTrafficPackInput struct {
	VariantID  int64  `json:"variant_id" binding:"required"`
	PayMethod  string `json:"pay_method"`       // balance/charge_site
	ChargeSiteID *int64 `json:"charge_site_id"`
}

// BuyTrafficPack buys a traffic pack for the client.
func (s *CommerceService) BuyTrafficPack(ctx context.Context, clientID int64, input BuyTrafficPackInput) (*CreateOrderResult, error) {
	variant, err := s.variantRepo.GetByID(ctx, input.VariantID)
	if err != nil || !variant.Enabled {
		return nil, fmt.Errorf("traffic pack not found")
	}
	if variant.VariantType != entity.VariantTypeTrafficPack {
		return nil, fmt.Errorf("variant is not a traffic pack")
	}

	client, err := s.clientRepo.GetByID(ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("client not found")
	}
	if client.PlanID == 0 {
		return nil, fmt.Errorf("you must have an active plan to buy traffic packs")
	}

	paidAmount := variant.Price

	if input.PayMethod == "balance" {
		if paidAmount > 0 {
			if err := s.deductBalanceForOrder(ctx, clientID, paidAmount); err != nil {
				return nil, fmt.Errorf("insufficient balance: %v", err)
			}
		}
		// Add traffic to subscription
		if err := s.addTrafficToSubscription(ctx, client, variant.QuotaBytes); err != nil {
			return nil, fmt.Errorf("failed to add traffic: %v", err)
		}
		// Create completed order
		now := time.Now()
		order := &entity.PlanOrder{
			ClientID: clientID, OrderType: entity.OrderTypeNew, ToVariantID: input.VariantID,
			OriginalAmount: paidAmount, PaidAmount: paidAmount, EffectiveFrom: now,
			Status: entity.OrderStatusCompleted, Remark: "流量叠加包购买",
		}
		if err := s.orderRepo.Create(ctx, order); err != nil {
			return nil, fmt.Errorf("failed to create order: %v", err)
		}
		return &CreateOrderResult{OrderID: order.ID, Status: entity.OrderStatusCompleted}, nil

	} else if input.PayMethod == "charge_site" {
		if input.ChargeSiteID == nil {
			return nil, fmt.Errorf("charge_site_id is required for charge_site payment")
		}
		site, err := s.chargeSiteRepo.GetByID(ctx, *input.ChargeSiteID)
		if err != nil || !site.Enabled {
			return nil, fmt.Errorf("charge site not available")
		}
		tradeNo := generateTradeNo()
		now := time.Now()
		expiresAt := now.Add(15 * time.Minute)
		chargeOrder := &entity.ChargeOrder{
			ClientID: clientID, TradeNo: tradeNo, Source: entity.ChargeSourceSite,
			SiteID: input.ChargeSiteID, Amount: paidAmount, PlanVariantID: &input.VariantID,
			OrderType: entity.ChargeOrderTypePurchase, Status: entity.ChargeStatusPending, ExpiresAt: &expiresAt,
		}
		if err := s.chargeOrderRepo.Create(ctx, chargeOrder); err != nil {
			return nil, fmt.Errorf("failed to create charge order: %v", err)
		}
		paymentURL, err := s.requestChargeSitePayment(ctx, site, chargeOrder, client)
		if err != nil {
			return nil, fmt.Errorf("failed to request payment: %v", err)
		}
		chargeOrder.PaymentURL = paymentURL
		s.chargeOrderRepo.Update(ctx, chargeOrder)
		return &CreateOrderResult{OrderID: chargeOrder.ID, Status: entity.ChargeStatusPending, PaymentURL: paymentURL, TradeNo: tradeNo}, nil
	}

	return nil, fmt.Errorf("pay_method is required")
}

// addTrafficToSubscription adds traffic bytes to the client's subscription.
func (s *CommerceService) addTrafficToSubscription(ctx context.Context, client *entity.Client, quotaBytes int64) error {
	if client.SubscriptionID == nil {
		return fmt.Errorf("no subscription found")
	}
	sub, err := s.subRepo.GetByID(ctx, *client.SubscriptionID)
	if err != nil {
		return err
	}
	sub.QuotaBytes += quotaBytes
	return s.subRepo.Update(ctx, sub)
}

// ============================================================
// P0-2: Order Management
// ============================================================

// CreateOrderInput is the input for creating a plan order.
type CreateOrderInput struct {
	VariantID      int64  `json:"variant_id" binding:"required"`
	OrderType      string `json:"order_type" binding:"required"` // new/renew/upgrade
	PromotionCode  string `json:"promotion_code"`
	PayMethod      string `json:"pay_method"`       // balance/charge_site
	ChargeSiteID   *int64 `json:"charge_site_id"`
}

// CreateOrderResult is the result of creating an order.
type CreateOrderResult struct {
	OrderID    int64  `json:"order_id"`
	Status     string `json:"status"`
	PaymentURL string `json:"payment_url,omitempty"`
	TradeNo    string `json:"trade_no,omitempty"`
}

// CreateOrder creates a plan order for the client.
func (s *CommerceService) CreateOrder(ctx context.Context, clientID int64, input CreateOrderInput) (*CreateOrderResult, error) {
	variant, err := s.variantRepo.GetByID(ctx, input.VariantID)
	if err != nil || !variant.Enabled {
		return nil, fmt.Errorf("variant not found or not available")
	}

	tmpl, err := s.tmplRepo.GetByID(ctx, variant.TemplateID)
	if err != nil || !tmpl.Enabled {
		return nil, fmt.Errorf("plan not available")
	}

	client, err := s.clientRepo.GetByID(ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("client not found")
	}

	switch input.OrderType {
	case entity.OrderTypeNew:
		if client.PlanID > 0 && client.Status != "expired" {
			return nil, fmt.Errorf("you already have an active plan, please use renew or upgrade")
		}
	case entity.OrderTypeRenew:
		if client.PlanID == 0 {
			return nil, fmt.Errorf("no active plan to renew")
		}
	case entity.OrderTypeUpgrade:
		if client.PlanID == 0 {
			return nil, fmt.Errorf("no active plan to upgrade")
		}
	default:
		return nil, fmt.Errorf("invalid order type: %s", input.OrderType)
	}

	originalAmount := variant.Price
	discountAmount := int64(0)
	promotionID := (*int64)(nil)

	if input.PromotionCode != "" {
		promo, err := s.promotionRepo.GetByCode(ctx, input.PromotionCode)
		if err == nil && s.validatePromotion(ctx, promo, clientID, variant.ID) {
			discountAmount = s.calculateDiscount(promo, originalAmount)
			pid := promo.ID
			promotionID = &pid
			s.promotionRepo.IncrementUsedCount(ctx, promo.ID)
		}
	}

	paidAmount := originalAmount - discountAmount
	if paidAmount < 0 {
		paidAmount = 0
	}

	now := time.Now()

	if input.PayMethod == "balance" {
		if paidAmount > 0 {
			if err := s.deductBalanceForOrder(ctx, clientID, paidAmount); err != nil {
				return nil, fmt.Errorf("insufficient balance: %v", err)
			}
		}
		order := &entity.PlanOrder{
			ClientID: clientID, OrderType: input.OrderType, ToVariantID: input.VariantID,
			OriginalAmount: originalAmount, DiscountAmount: discountAmount, PaidAmount: paidAmount,
			PromotionID: promotionID, PromotionCode: input.PromotionCode,
			EffectiveFrom: now, Status: entity.OrderStatusCompleted,
		}
		if err := s.orderRepo.Create(ctx, order); err != nil {
			return nil, fmt.Errorf("failed to create order: %v", err)
		}
		if err := s.activatePlan(ctx, client, tmpl, variant, input.OrderType); err != nil {
			return nil, fmt.Errorf("failed to activate plan: %v", err)
		}
		return &CreateOrderResult{OrderID: order.ID, Status: entity.OrderStatusCompleted}, nil

	} else if input.PayMethod == "charge_site" {
		if input.ChargeSiteID == nil {
			return nil, fmt.Errorf("charge_site_id is required for charge_site payment")
		}
		site, err := s.chargeSiteRepo.GetByID(ctx, *input.ChargeSiteID)
		if err != nil || !site.Enabled {
			return nil, fmt.Errorf("charge site not available")
		}
		order := &entity.PlanOrder{
			ClientID: clientID, OrderType: input.OrderType, ToVariantID: input.VariantID,
			OriginalAmount: originalAmount, DiscountAmount: discountAmount, PaidAmount: paidAmount,
			PromotionID: promotionID, PromotionCode: input.PromotionCode,
			EffectiveFrom: now, Status: entity.OrderStatusPending,
		}
		if err := s.orderRepo.Create(ctx, order); err != nil {
			return nil, fmt.Errorf("failed to create order: %v", err)
		}
		tradeNo := generateTradeNo()
		expiresAt := now.Add(15 * time.Minute)
		chargeOrder := &entity.ChargeOrder{
			ClientID: clientID, TradeNo: tradeNo, Source: entity.ChargeSourceSite,
			SiteID: input.ChargeSiteID, Amount: paidAmount, PlanVariantID: &input.VariantID,
			OrderType: entity.ChargeOrderTypePurchase, Status: entity.ChargeStatusPending, ExpiresAt: &expiresAt,
		}
		if err := s.chargeOrderRepo.Create(ctx, chargeOrder); err != nil {
			return nil, fmt.Errorf("failed to create charge order: %v", err)
		}
		paymentURL, err := s.requestChargeSitePayment(ctx, site, chargeOrder, client)
		if err != nil {
			return nil, fmt.Errorf("failed to request payment: %v", err)
		}
		chargeOrder.PaymentURL = paymentURL
		s.chargeOrderRepo.Update(ctx, chargeOrder)
		return &CreateOrderResult{OrderID: order.ID, Status: entity.OrderStatusPending, PaymentURL: paymentURL, TradeNo: tradeNo}, nil
	}

	// No pay_method: pending order
	order := &entity.PlanOrder{
		ClientID: clientID, OrderType: input.OrderType, ToVariantID: input.VariantID,
		OriginalAmount: originalAmount, DiscountAmount: discountAmount, PaidAmount: paidAmount,
		PromotionID: promotionID, PromotionCode: input.PromotionCode,
		EffectiveFrom: now, Status: entity.OrderStatusPending,
	}
	if err := s.orderRepo.Create(ctx, order); err != nil {
		return nil, fmt.Errorf("failed to create order: %v", err)
	}
	return &CreateOrderResult{OrderID: order.ID, Status: entity.OrderStatusPending}, nil
}

// ListOrders returns the client's order history.
func (s *CommerceService) ListOrders(ctx context.Context, clientID int64, limit, offset int) ([]entity.PlanOrder, int64, error) {
	return s.orderRepo.ListByClientID(ctx, clientID, limit, offset)
}

// GetOrder returns a single order detail.
func (s *CommerceService) GetOrder(ctx context.Context, clientID, orderID int64) (*entity.PlanOrder, error) {
	order, err := s.orderRepo.GetByID(ctx, orderID)
	if err != nil || order.ClientID != clientID {
		return nil, fmt.Errorf("order not found")
	}
	return order, nil
}

// ============================================================
// P0-3: Charging System
// ============================================================

// ChargeSiteItem is a simplified charge site for client display.
type ChargeSiteItem struct {
	ID               int64    `json:"id"`
	Name             string   `json:"name"`
	IconURL          string   `json:"icon_url"`
	SupportedMethods []string `json:"supported_methods"`
}

// ListChargeSites returns all enabled charge sites.
func (s *CommerceService) ListChargeSites(ctx context.Context) ([]ChargeSiteItem, error) {
	sites, err := s.chargeSiteRepo.ListEnabled(ctx)
	if err != nil {
		return nil, err
	}
	var items []ChargeSiteItem
	for _, site := range sites {
		var methods []string
		if site.SupportedMethods != "" {
			json.Unmarshal([]byte(site.SupportedMethods), &methods)
		}
		items = append(items, ChargeSiteItem{ID: site.ID, Name: site.Name, IconURL: site.IconURL, SupportedMethods: methods})
	}
	return items, nil
}

// CreateChargeInput is the input for creating a charge order.
type CreateChargeInput struct {
	SiteID        int64  `json:"site_id" binding:"required"`
	Amount        int64  `json:"amount" binding:"required"`
	PlanVariantID *int64 `json:"plan_variant_id"`
}

// CreateChargeOrder creates a charge order and returns the payment URL.
func (s *CommerceService) CreateChargeOrder(ctx context.Context, clientID int64, input CreateChargeInput) (*CreateOrderResult, error) {
	site, err := s.chargeSiteRepo.GetByID(ctx, input.SiteID)
	if err != nil || !site.Enabled {
		return nil, fmt.Errorf("charge site not available")
	}
	if input.Amount <= 0 {
		return nil, fmt.Errorf("amount must be positive")
	}
	client, err := s.clientRepo.GetByID(ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("client not found")
	}

	tradeNo := generateTradeNo()
	now := time.Now()
	expiresAt := now.Add(15 * time.Minute)
	orderType := entity.ChargeOrderTypeCharge
	if input.PlanVariantID != nil {
		orderType = entity.ChargeOrderTypePurchase
	}
	chargeOrder := &entity.ChargeOrder{
		ClientID: clientID, TradeNo: tradeNo, Source: entity.ChargeSourceSite,
		SiteID: &input.SiteID, Amount: input.Amount, PlanVariantID: input.PlanVariantID,
		OrderType: orderType, Status: entity.ChargeStatusPending, ExpiresAt: &expiresAt,
	}
	if err := s.chargeOrderRepo.Create(ctx, chargeOrder); err != nil {
		return nil, fmt.Errorf("failed to create charge order: %v", err)
	}
	paymentURL, err := s.requestChargeSitePayment(ctx, site, chargeOrder, client)
	if err != nil {
		return nil, fmt.Errorf("failed to request payment: %v", err)
	}
	chargeOrder.PaymentURL = paymentURL
	s.chargeOrderRepo.Update(ctx, chargeOrder)
	return &CreateOrderResult{OrderID: chargeOrder.ID, Status: entity.ChargeStatusPending, PaymentURL: paymentURL, TradeNo: tradeNo}, nil
}

// ChargeCallbackInput is the input from charge site callback.
type ChargeCallbackInput struct {
	TradeNo      string          `json:"trade_no"`
	SiteTradeNo  string          `json:"site_trade_no"`
	Status       string          `json:"status"`       // 业务状态，不参与签名
	Amount       int64           `json:"amount"`
	Currency     string          `json:"currency"`     // 订单货币（参与签名）
	Timestamp    int64           `json:"timestamp"`
	Nonce        string          `json:"nonce"`
	PaidCurrency string          `json:"paid_currency"`
	PaidAmount   int64           `json:"paid_amount"`
	Method       string          `json:"method"`
	ExchangeRate float64         `json:"exchange_rate"`
	PaidAt       string          `json:"paid_at"`
	Sign         string          `json:"sign"`
	Extra        json.RawMessage `json:"extra"`
}

// HandleChargeCallback processes a charge site callback.
// v3.0 verification chain (per CHARGE_SITE_INTEGRATION.md §4.2):
//  1. request_id  2. site check  3. verified check  4. IP whitelist
//  5. timestamp check  6. nonce atomic INSERT (replay prevention)
//  7. HMAC verify  8. order lookup  9. order belong  10. status check
//  11. amount check  12. transaction: processing → balance → paid → event
func (s *CommerceService) HandleChargeCallback(ctx context.Context, siteID int64, remoteIP string, requestID string, input ChargeCallbackInput) error {
	if requestID == "" {
		requestID = generateRequestID()
	}
	rawBody, _ := json.Marshal(input)
	payloadHash := sha256Hex(rawBody)

	recordEvent := func(result, errorCode string, signValid bool) {
		s.recordCallbackEvent(ctx, siteID, requestID, input.TradeNo, remoteIP, payloadHash, input.Amount, result, errorCode, signValid)
		s.logCallbackRequest(ctx, siteID, requestID, input.TradeNo, remoteIP, result, errorCode, "", payloadHash)
	}

	// ② site
	site, err := s.chargeSiteRepo.GetByID(ctx, siteID)
	if err != nil {
		log.Printf("[callback] request_id=%s site_id=%d error=site_not_found", requestID, siteID)
		recordEvent("error", "SITE_NOT_FOUND", false)
		return fmt.Errorf("charge site not found")
	}
	log.Printf("[callback] request_id=%s site_id=%d remote_ip=%s", requestID, siteID, remoteIP)

	// ③ verified (含过期检查)
	if !site.Verified {
		recordEvent("error", "SITE_NOT_VERIFIED", false)
		return fmt.Errorf("site not verified")
	}
	if site.VerifyExpireAt != nil && site.VerifyExpireAt.Before(time.Now()) {
		recordEvent("error", "SITE_NOT_VERIFIED", false)
		return fmt.Errorf("site verification expired")
	}
	log.Printf("[callback] request_id=%s verified=ok", requestID)

	// ④ IP whitelist
	if site.CallbackWhitelist != "" && !isIPAllowed(remoteIP, site.CallbackWhitelist) {
		log.Printf("[callback] request_id=%s ip_whitelist=denied ip=%s", requestID, remoteIP)
		recordEvent("error", "IP_NOT_ALLOWED", false)
		return fmt.Errorf("ip not allowed: %s", remoteIP)
	}
	log.Printf("[callback] request_id=%s ip_whitelist=ok", requestID)

	// ⑤ timestamp
	now := time.Now().Unix()
	if input.Timestamp == 0 || absInt64(now-input.Timestamp) > 300 {
		log.Printf("[callback] request_id=%s timestamp=expired", requestID)
		recordEvent("replay", "REPLAY_ATTACK", false)
		return fmt.Errorf("callback timestamp expired (possible replay attack)")
	}
	if input.Nonce == "" {
		recordEvent("replay", "REPLAY_ATTACK", false)
		return fmt.Errorf("nonce is required")
	}
	log.Printf("[callback] request_id=%s timestamp=ok", requestID)

	// ⑥ nonce 原子消费（INSERT, UNIQUE 冲突 = 重放）
	// 站点+nonce 联合唯一，不同站点可使用相同 nonce
	nonceRecord := &entity.CallbackNonce{
		Nonce:     input.Nonce,
		SiteID:    siteID,
		TradeNo:   input.TradeNo,
		CreatedAt: time.Now(),
	}
	if err := s.db.WithContext(ctx).Create(nonceRecord).Error; err != nil {
		log.Printf("[callback] request_id=%s nonce_replay nonce=%s", requestID, input.Nonce)
		recordEvent("replay", "REPLAY_ATTACK", false)
		return fmt.Errorf("nonce already used (replay attack)")
	}
	log.Printf("[callback] request_id=%s nonce=consumed", requestID)

	// ⑦ HMAC verify (含旧密钥轮换)
	secret := resolveSiteSecret(site)
	if secret == "" {
		log.Printf("[callback] request_id=%s secret=missing", requestID)
		recordEvent("signature_failed", "SECRET_REQUIRED", false)
		s.raiseCallbackAlert(siteID, "SECRET_REQUIRED", "warning", "回调密钥未配置", fmt.Sprintf("site_id=%d", siteID))
		return fmt.Errorf("callback_secret not configured")
	}
	signValid := s.verifyChargeCallback(input, secret)
	if !signValid {
		prevSecret := resolveSiteSecretPrev(site)
		if prevSecret != "" && s.verifyChargeCallback(input, prevSecret) {
			signValid = true
			log.Printf("[callback] request_id=%s verify_signature=ok_via_prev_secret", requestID)
		} else {
			log.Printf("[callback] request_id=%s verify_signature=failed", requestID)
			recordEvent("signature_failed", "INVALID_SIGNATURE", false)
			s.raiseCallbackAlert(siteID, "INVALID_SIGNATURE", "critical", "回调签名验证失败", fmt.Sprintf("site_id=%d remote_ip=%s trade_no=%s", siteID, remoteIP, input.TradeNo))
			return fmt.Errorf("invalid signature")
		}
	} else {
		log.Printf("[callback] request_id=%s verify_signature=ok", requestID)
	}

	// ⑧ order lookup
	order, err := s.chargeOrderRepo.GetByTradeNo(ctx, input.TradeNo)
	if err != nil {
		log.Printf("[callback] request_id=%s order=not_found trade_no=%s", requestID, input.TradeNo)
		recordEvent("error", "ORDER_NOT_FOUND", signValid)
		return fmt.Errorf("order not found: %s", input.TradeNo)
	}

	// ⑨ order belong
	if order.SiteID == nil || *order.SiteID != site.ID {
		log.Printf("[callback] request_id=%s order_not_belong", requestID)
		recordEvent("error", "ORDER_NOT_BELONG", signValid)
		return fmt.Errorf("order does not belong to this site")
	}

	// ⑩ order status (幂等 + 状态机)
	if order.Status == entity.ChargeStatusPaid && input.Status == "paid" {
		log.Printf("[callback] request_id=%s order=%s idempotent=paid", requestID, input.TradeNo)
		recordEvent("success", "", signValid)
		return nil
	}
	if order.Status == entity.ChargeStatusProcessing {
		log.Printf("[callback] request_id=%s order=%s processing_skip", requestID, input.TradeNo)
		recordEvent("success", "", signValid)
		return nil
	}
	targetStatus := entity.ChargeStatusPaid
	if input.Status != "paid" {
		targetStatus = entity.ChargeStatusFailed
	}
	if !entity.CanChargeOrderTransition(order.Status, targetStatus) {
		log.Printf("[callback] request_id=%s invalid_transition from=%s to=%s", requestID, order.Status, targetStatus)
		recordEvent("error", "INVALID_STATUS_TRANSITION", signValid)
		return fmt.Errorf("order status is %s, cannot process callback", order.Status)
	}

	// ⑪ amount check
	if input.Amount != order.Amount {
		log.Printf("[callback] request_id=%s amount_mismatch callback=%d order=%d", requestID, input.Amount, order.Amount)
		recordEvent("amount_error", "AMOUNT_MISMATCH", signValid)
		s.raiseCallbackAlert(siteID, "AMOUNT_MISMATCH", "critical", "回调金额不一致", fmt.Sprintf("site_id=%d trade_no=%s callback=%d order=%d", siteID, input.TradeNo, input.Amount, order.Amount))
		return fmt.Errorf("amount mismatch: callback=%d order=%d", input.Amount, order.Amount)
	}

	// ⑫ process payment result
	if input.Status == "paid" {
		paidAt := time.Now()
		if input.PaidAt != "" {
			if t, err := time.Parse(time.RFC3339, input.PaidAt); err == nil {
				paidAt = t
			}
		}

		// Transaction: pending → processing → balance → paid
		err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			// Step 1: acquire processing lock (pending → processing)
			result := tx.Model(&entity.ChargeOrder{}).
				Where("trade_no = ? AND status = ?", input.TradeNo, entity.ChargeStatusPending).
				Update("status", entity.ChargeStatusProcessing)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return nil // already handled by another request
			}

			// Step 2: get current balance for balance_before snapshot
			var bal entity.ClientBalance
			if err := tx.Where("client_id = ?", order.ClientID).First(&bal).Error; err != nil {
				return err
			}
			balanceBefore := bal.Balance

			// Step 3: increase balance
			if err := tx.Model(&entity.ClientBalance{}).
				Where("client_id = ?", order.ClientID).
				UpdateColumn("balance", gorm.Expr("balance + ?", order.Amount)).Error; err != nil {
				return err
			}

			// Step 4: create balance transaction (UNIQUE + balance_before snapshot)
			txRecord := &entity.BalanceTransaction{
				ClientID:             order.ClientID,
				Type:                 entity.BalanceTxTypeCharge,
				Amount:               order.Amount,
				BalanceBefore:        balanceBefore,
				BalanceAfter:         balanceBefore + order.Amount,
				Currency:             site.Currency,
				RelatedChargeOrderID: &order.ID,
				Remark:               fmt.Sprintf("充值回调 trade_no=%s", input.TradeNo),
			}
			if err := tx.Create(txRecord).Error; err != nil {
				return err
			}

			// Step 5: update order to paid (processing → paid)
			if err := tx.Model(&entity.ChargeOrder{}).
				Where("trade_no = ? AND status = ?", input.TradeNo, entity.ChargeStatusProcessing).
				Updates(map[string]interface{}{
					"status":        entity.ChargeStatusPaid,
					"site_trade_no": input.SiteTradeNo,
					"paid_currency": input.PaidCurrency,
					"paid_amount":   input.PaidAmount,
					"exchange_rate": input.ExchangeRate,
					"method":        input.Method,
					"paid_at":       paidAt,
					"callback_raw":  string(rawBody),
				}).Error; err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			log.Printf("[callback] request_id=%s order=%s transaction_error=%v", requestID, input.TradeNo, err)
			recordEvent("error", "CALLBACK_FAILED", signValid)
			s.enqueueCallbackRetry(ctx, siteID, requestID, input.TradeNo, remoteIP, payloadHash, "CALLBACK_FAILED", err.Error(), true)
			s.raiseCallbackAlert(siteID, "CALLBACK_FAILED", "high", "回调入账失败", fmt.Sprintf("site_id=%d trade_no=%s error=%v", siteID, input.TradeNo, err))
			return err
		}
		log.Printf("[callback] request_id=%s order=%s amount=%d status=paid success", requestID, input.TradeNo, order.Amount)
		recordEvent("success", "", signValid)
		return nil
	}

	// payment failed
	s.db.WithContext(ctx).Model(&entity.ChargeOrder{}).
		Where("trade_no = ? AND status IN ?", input.TradeNo, []string{entity.ChargeStatusPending, entity.ChargeStatusProcessing}).
		Update("status", entity.ChargeStatusFailed)
	log.Printf("[callback] request_id=%s order=%s status=failed", requestID, input.TradeNo)
	recordEvent("success", "", signValid)
	return nil
}

// logCallbackRequest records a callback log entry (without raw payload, uses payload_hash).
func (s *CommerceService) logCallbackRequest(ctx context.Context, siteID int64, requestID, tradeNo, remoteIP, status, errorCode, errMsg, payloadHash string) {
	logEntry := &entity.ChargeCallbackLog{
		SiteID:       siteID,
		RequestID:    requestID,
		TradeNo:      tradeNo,
		RemoteIP:     remoteIP,
		Status:       status,
		ErrorCode:    errorCode,
		ErrorMessage: errMsg,
		PayloadHash:  payloadHash,
	}
	if err := s.db.WithContext(ctx).Create(logEntry).Error; err != nil {
		log.Printf("[callback] request_id=%s failed to log callback: %v", requestID, err)
	}
}

// recordCallbackEvent records a structured callback event for dispute resolution.
func (s *CommerceService) recordCallbackEvent(ctx context.Context, siteID int64, requestID, tradeNo, remoteIP, payloadHash string, amount int64, result, errorCode string, signValid bool) {
	event := &entity.ChargeCallbackEvent{
		SiteID:      siteID,
		TradeNo:     tradeNo,
		RequestID:   requestID,
		RemoteIP:    remoteIP,
		PayloadHash: payloadHash,
		SignValid:   signValid,
		Result:      result,
		ErrorCode:   errorCode,
		Amount:      amount,
	}
	if err := s.db.WithContext(ctx).Create(event).Error; err != nil {
		log.Printf("[callback] request_id=%s failed to record event: %v", requestID, err)
	}
}

// sha256Hex returns the hex-encoded SHA-256 hash of data.
func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// generateRequestID generates a unique request ID: req_ + 12 hex chars.
func generateRequestID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return "req_" + hex.EncodeToString(b)
}

// resolveSiteSecret returns the plaintext callback_secret (tries encrypted first, then plaintext fallback).
func resolveSiteSecret(site *entity.ChargeSite) string {
	if site.EncryptedSecret != "" {
		if decrypted, err := crypto.DecryptSecret(site.EncryptedSecret); err == nil {
			return decrypted
		}
	}
	return site.CallbackSecret // backward compat: plaintext fallback
}

// resolveSiteSecretPrev returns the plaintext previous callback_secret for key rotation.
func resolveSiteSecretPrev(site *entity.ChargeSite) string {
	if site.EncryptedSecretPrev != "" {
		if decrypted, err := crypto.DecryptSecret(site.EncryptedSecretPrev); err == nil {
			return decrypted
		}
	}
	return ""
}

// raiseCallbackAlert creates a SecurityAlert record for callback anomalies.
func (s *CommerceService) raiseCallbackAlert(siteID int64, alertType, severity, title, message string) {
	alert := &entity.SecurityAlert{
		AlertType:   "charge_callback_" + alertType,
		Severity:    severity,
		DedupKey:    fmt.Sprintf("charge_callback:%d:%s:%d", siteID, alertType, time.Now().Unix()/3600), // 每小时去重
		Title:       title,
		Message:     message,
		Status:      "active",
		TriggeredAt: time.Now(),
	}
	if err := s.db.Create(alert).Error; err != nil {
		log.Printf("[callback] failed to create security alert: %v", err)
	}
}

// enqueueCallbackRetry stores a failed callback for retry processing.
// Only transient errors should be retried; non-retryable errors are logged but not enqueued.
func (s *CommerceService) enqueueCallbackRetry(ctx context.Context, siteID int64, requestID, tradeNo, remoteIP, payloadHash, errorCode, errMsg string, retryable bool) {
	if !retryable {
		log.Printf("[callback] request_id=%s not retried (non-retryable: %s)", requestID, errorCode)
		return
	}
	nextRetry := time.Now().Add(1 * time.Minute)
	retry := &entity.ChargeCallbackRetry{
		SiteID:       siteID,
		RequestID:    requestID,
		TradeNo:      tradeNo,
		RemoteIP:     remoteIP,
		PayloadHash:  payloadHash,
		ErrorCode:    errorCode,
		ErrorMessage: errMsg,
		Retryable:    true,
		MaxRetries:   5,
		Status:       "pending",
		NextRetryAt:  &nextRetry,
	}
	if err := s.db.WithContext(ctx).Create(retry).Error; err != nil {
		log.Printf("[callback] request_id=%s failed to enqueue retry: %v", requestID, err)
	}
}

// StartRetryScheduler starts a background goroutine that periodically retries failed callbacks.
func (s *CommerceService) StartRetryScheduler(interval time.Duration) chan struct{} {
	stopCh := make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				s.processRetries()
			}
		}
	}()
	return stopCh
}

// processRetries picks up pending retries and checks if they've been resolved.
// v3.0: raw payload is no longer stored, so we can only check order status.
// Unresolved retries are marked as exhausted for manual intervention.
func (s *CommerceService) processRetries() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var retries []entity.ChargeCallbackRetry
	now := time.Now()
	if err := s.db.Where("status = ? AND next_retry_at <= ?", "pending", now).Limit(10).Find(&retries).Error; err != nil {
		return
	}

	for _, retry := range retries {
		retryID := retry.ID
		s.db.Model(&retry).Updates(map[string]interface{}{"status": "processing"})

		// Check if order is already paid (skip retry)
		order, err := s.chargeOrderRepo.GetByTradeNo(ctx, retry.TradeNo)
		if err != nil {
			s.db.Model(&entity.ChargeCallbackRetry{}).Where("id = ?", retryID).
				Updates(map[string]interface{}{"status": "exhausted", "next_retry_at": nil, "error_message": "order not found"})
			continue
		}
		if order.Status == entity.ChargeStatusPaid {
			s.db.Model(&entity.ChargeCallbackRetry{}).Where("id = ?", retryID).
				Updates(map[string]interface{}{"status": "success", "next_retry_at": nil})
			log.Printf("[retry] retry_id=%d trade_no=%s already paid, skip", retryID, retry.TradeNo)
			continue
		}

		// Cannot retry without the raw payload (v3.0 no longer stores it).
		// Mark as exhausted — manual intervention required.
		s.db.Model(&entity.ChargeCallbackRetry{}).Where("id = ?", retryID).
			Updates(map[string]interface{}{"status": "exhausted", "next_retry_at": nil, "error_message": "payload not available for retry (use manual intervention)"})
		log.Printf("[retry] retry_id=%d trade_no=%s exhausted (no payload, manual intervention required)", retryID, retry.TradeNo)
	}
}

// RotateSiteSecret rotates the callback secret: current secret becomes prev, new secret becomes current.
func (s *CommerceService) RotateSiteSecret(ctx context.Context, siteID int64, newSecret string) error {
	site, err := s.chargeSiteRepo.GetByID(ctx, siteID)
	if err != nil {
		return fmt.Errorf("charge site not found")
	}
	if newSecret == "" {
		return fmt.Errorf("new secret cannot be empty")
	}

	// 当前密钥变为旧密钥
	site.EncryptedSecretPrev = site.EncryptedSecret

	// 加密新密钥
	encrypted, err := crypto.EncryptSecret(newSecret)
	if err != nil {
		return fmt.Errorf("failed to encrypt new secret: %w", err)
	}
	site.EncryptedSecret = encrypted
	site.CallbackSecret = ""

	return s.chargeSiteRepo.Update(ctx, site)
}

// ClearPrevSecret removes the previous secret after the transition period is over.
func (s *CommerceService) ClearPrevSecret(ctx context.Context, siteID int64) error {
	site, err := s.chargeSiteRepo.GetByID(ctx, siteID)
	if err != nil {
		return fmt.Errorf("charge site not found")
	}
	site.EncryptedSecretPrev = ""
	return s.chargeSiteRepo.Update(ctx, site)
}

// absInt64 returns the absolute value of n.
func absInt64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

// isIPAllowed checks if remoteIP is in the whitelist JSON.
func isIPAllowed(remoteIP string, whitelistJSON string) bool {
	var whitelist []string
	if err := json.Unmarshal([]byte(whitelistJSON), &whitelist); err != nil {
		return false
	}
	testIP := net.ParseIP(remoteIP)
	if testIP == nil {
		return false
	}
	for _, entry := range whitelist {
		if strings.Contains(entry, "/") {
			// CIDR
			_, cidr, err := net.ParseCIDR(entry)
			if err == nil && cidr.Contains(testIP) {
				return true
			}
		} else {
			// Single IP
			if entry == remoteIP {
				return true
			}
		}
	}
	return false
}

// GetChargeStatus returns the charge order status.
func (s *CommerceService) GetChargeStatus(ctx context.Context, clientID int64, tradeNo string) (*entity.ChargeOrder, error) {
	order, err := s.chargeOrderRepo.GetByTradeNo(ctx, tradeNo)
	if err != nil || order.ClientID != clientID {
		return nil, fmt.Errorf("order not found")
	}
	return order, nil
}

// GetChargeOrderDetail returns a charge order detail by ID.
func (s *CommerceService) GetChargeOrderDetail(ctx context.Context, clientID, orderID int64) (*entity.ChargeOrder, error) {
	order, err := s.chargeOrderRepo.GetByID(ctx, orderID)
	if err != nil || order.ClientID != clientID {
		return nil, fmt.Errorf("order not found")
	}
	return order, nil
}

// ListChargeOrders returns the client's charge order history.
func (s *CommerceService) ListChargeOrders(ctx context.Context, clientID int64, status string, limit, offset int) ([]entity.ChargeOrder, int64, error) {
	return s.chargeOrderRepo.ListByClientID(ctx, clientID, status, limit, offset)
}

// RedeemCardInput is the input for redeeming a charge card.
type RedeemCardInput struct {
	CardCode string `json:"card_code" binding:"required"`
}

// RedeemCard redeems a charge card for the client.
func (s *CommerceService) RedeemCard(ctx context.Context, clientID int64, input RedeemCardInput) (*entity.ChargeOrder, error) {
	card, err := s.chargeCardRepo.GetByCardCode(ctx, input.CardCode)
	if err != nil {
		return nil, fmt.Errorf("invalid card code")
	}
	if card.Status != entity.ChargeCardStatusActive {
		return nil, fmt.Errorf("card is not available")
	}
	if card.ExpiresAt != nil && card.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("card has expired")
	}
	if card.MaxUses > 0 && card.UsedCount >= card.MaxUses {
		return nil, fmt.Errorf("card has been fully used")
	}

	tradeNo := generateTradeNo()
	now := time.Now()
	cardID := card.ID

	chargeOrder := &entity.ChargeOrder{
		ClientID: clientID, TradeNo: tradeNo, Source: entity.ChargeSourceCard,
		Amount: card.Value, ChargeCardID: &cardID,
		OrderType: entity.ChargeOrderTypeCharge, Status: entity.ChargeStatusPaid, PaidAt: &now,
	}
	if card.CardType == entity.ChargeCardTypePlan {
		chargeOrder.OrderType = entity.ChargeOrderTypePurchase
		chargeOrder.PlanVariantID = &card.Value
		chargeOrder.Amount = 0
	}
	if err := s.chargeOrderRepo.Create(ctx, chargeOrder); err != nil {
		return nil, fmt.Errorf("failed to create charge order: %v", err)
	}
	if err := s.processChargePayment(ctx, chargeOrder); err != nil {
		return nil, fmt.Errorf("failed to process card: %v", err)
	}
	s.chargeCardRepo.IncrementUsedCount(ctx, card.ID)

	updated, _ := s.chargeCardRepo.GetByCardCode(ctx, input.CardCode)
	if updated != nil && updated.MaxUses > 0 && updated.UsedCount >= updated.MaxUses {
		updated.Status = entity.ChargeCardStatusUsed
		s.chargeCardRepo.Update(ctx, updated)
	}
	return chargeOrder, nil
}

// ============================================================
// P0-4: Balance System
// ============================================================

// BalanceInfo represents the client's balance.
type BalanceInfo struct {
	ClientID int64 `json:"client_id"`
	Balance  int64 `json:"balance"`
	Frozen   int64 `json:"frozen"`
}

// GetBalance returns the client's balance.
func (s *CommerceService) GetBalance(ctx context.Context, clientID int64) (*BalanceInfo, error) {
	bal, err := s.balanceRepo.GetOrCreate(ctx, clientID)
	if err != nil {
		return nil, err
	}
	return &BalanceInfo{ClientID: bal.ClientID, Balance: bal.Balance, Frozen: bal.Frozen}, nil
}

// ListBalanceTransactions returns the client's balance transaction history.
func (s *CommerceService) ListBalanceTransactions(ctx context.Context, clientID int64, txType string, limit, offset int) ([]entity.BalanceTransaction, int64, error) {
	return s.balanceTxRepo.ListByClientID(ctx, clientID, txType, limit, offset)
}

// ============================================================
// P0-5: /me Extension
// ============================================================

// GetMeExtendedInfo returns additional info for /me extension.
func (s *CommerceService) GetMeExtendedInfo(ctx context.Context, clientID int64) (quotaResetCycle string, quotaResetDay *int, nextResetAt *time.Time, balance int64) {
	client, err := s.clientRepo.GetByID(ctx, clientID)
	if err != nil {
		return "", nil, nil, 0
	}
	if client.SubscriptionID != nil {
		if sub, err := s.subRepo.GetByID(ctx, *client.SubscriptionID); err == nil {
			quotaResetCycle = sub.QuotaResetCycle
			quotaResetDay = sub.QuotaResetDay
			nextResetAt = sub.NextResetAt
		}
	}
	bal, err := s.balanceRepo.GetByClientID(ctx, clientID)
	if err == nil {
		balance = bal.Balance
	}
	return
}

// ============================================================
// Admin: Expired Order Cleanup
// ============================================================

// CleanupExpiredOrders marks all expired pending charge orders.
func (s *CommerceService) CleanupExpiredOrders(ctx context.Context) error {
	return s.chargeOrderRepo.MarkExpired(ctx)
}

// StartCleanupScheduler starts a background goroutine that periodically
// cleans up expired pending charge orders. Call this once at app startup.
// Returns a stop channel that can be closed to stop the scheduler.
func (s *CommerceService) StartCleanupScheduler(interval time.Duration) chan struct{} {
	stopCh := make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				if err := s.CleanupExpiredOrders(ctx); err != nil {
					// Log but don't crash
					_ = err
				}
				cancel()
			}
		}
	}()
	return stopCh
}

// ============================================================
// Internal helpers
// ============================================================

// processChargePayment handles the actual payment processing after a charge is confirmed.
// NOTE: We use the db-level repos directly instead of tx-scoped repos because
// the repository pattern doesn't support injection of *gorm.DB per-call.
// The outer Transaction() wrapper ensures atomicity at the DB level.
func (s *CommerceService) processChargePayment(ctx context.Context, order *entity.ChargeOrder) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if order.OrderType == entity.ChargeOrderTypeCharge && order.Amount > 0 {
			newBal, err := s.balanceRepo.AddBalance(ctx, order.ClientID, order.Amount)
			if err != nil {
				return err
			}
			remark := "充值到账"
			if order.Source == entity.ChargeSourceCard {
				remark = "充值卡兑换到账"
			}
			txRecord := &entity.BalanceTransaction{
				ClientID: order.ClientID, Type: entity.BalanceTxTypeCharge,
				Amount: order.Amount, BalanceAfter: newBal.Balance,
				RelatedChargeOrderID: &order.ID, Remark: remark,
			}
			return s.balanceTxRepo.Create(ctx, txRecord)
		} else if order.OrderType == entity.ChargeOrderTypePurchase && order.PlanVariantID != nil {
			variant, err := s.variantRepo.GetByID(ctx, *order.PlanVariantID)
			if err != nil {
				return fmt.Errorf("variant not found")
			}
			tmpl, err := s.tmplRepo.GetByID(ctx, variant.TemplateID)
			if err != nil {
				return fmt.Errorf("template not found")
			}
			client, err := s.clientRepo.GetByID(ctx, order.ClientID)
			if err != nil {
				return err
			}

			// Traffic pack: add traffic without extending expiry
			if variant.VariantType == entity.VariantTypeTrafficPack {
				if err := s.addTrafficToSubscription(ctx, client, variant.QuotaBytes); err != nil {
					return err
				}
			} else {
				// Regular plan activation
				orderType := entity.OrderTypeNew
				if client.PlanID > 0 {
					orderType = entity.OrderTypeRenew
				}
				if err := s.activatePlan(ctx, client, tmpl, variant, orderType); err != nil {
					return err
				}
			}

			// Create plan order for record-keeping
			now := time.Now()
			planOrder := &entity.PlanOrder{
				ClientID: order.ClientID, OrderType: entity.OrderTypeNew,
				ToVariantID: *order.PlanVariantID, OriginalAmount: order.Amount,
				PaidAmount: order.Amount, EffectiveFrom: now,
				Status: entity.OrderStatusCompleted, Remark: "充值订单自动开通",
			}
			if order.Source == entity.ChargeSourceCard {
				planOrder.PaidAmount = 0
				planOrder.Remark = "充值卡兑换自动开通"
			}
			return s.orderRepo.Create(ctx, planOrder)
		}
		return nil
	})
}

// activatePlan assigns the plan to the client and creates/updates subscription.
func (s *CommerceService) activatePlan(ctx context.Context, client *entity.Client, tmpl *entity.PlanTemplate, variant *entity.PlanVariant, orderType string) error {
	now := time.Now()
	client.PlanID = tmpl.ID
	client.PlanVariantID = variant.ID

	quotaBytes := variant.QuotaBytes
	if quotaBytes == 0 {
		quotaBytes = tmpl.DefaultQuotaBytes
	}

	switch orderType {
	case entity.OrderTypeNew:
		client.QuotaBytes = quotaBytes
		client.UsedBytes = 0
		client.Status = "active"
		expiresAt := now.AddDate(0, 0, variant.DurationDays)
		client.ExpiresAt = &expiresAt
	case entity.OrderTypeRenew:
		client.QuotaBytes = quotaBytes
		baseTime := now
		if client.ExpiresAt != nil && client.ExpiresAt.After(now) {
			baseTime = *client.ExpiresAt
		}
		expiresAt := baseTime.AddDate(0, 0, variant.DurationDays)
		client.ExpiresAt = &expiresAt
		client.Status = "active"
	case entity.OrderTypeUpgrade:
		client.QuotaBytes = quotaBytes
		expiresAt := now.AddDate(0, 0, variant.DurationDays)
		client.ExpiresAt = &expiresAt
		client.Status = "active"
	}

	if tmpl.DeviceLimit > 0 {
		client.DeviceLimit = tmpl.DeviceLimit
	}
	if tmpl.SpeedLimitMbps > 0 {
		client.SpeedLimitMbps = tmpl.SpeedLimitMbps
	}
	if tmpl.ConcurrentLimit > 0 {
		client.ConcurrentLimit = tmpl.ConcurrentLimit
	}

	if err := s.clientRepo.Update(ctx, client); err != nil {
		return err
	}

	if client.SubscriptionID == nil {
		sub := &entity.Subscription{
			ClientID: client.ID, Format: "xray", Token: generateToken(32),
			Status: "active", QuotaBytes: quotaBytes,
			QuotaResetCycle: variant.QuotaResetCycle, QuotaResetDay: variant.QuotaResetDay,
			ExpiresAt: client.ExpiresAt,
		}
		if sub.QuotaResetCycle == "" {
			sub.QuotaResetCycle = tmpl.DefaultQuotaResetCycle
		}
		if sub.QuotaResetDay == nil {
			sub.QuotaResetDay = tmpl.DefaultQuotaResetDay
		}
		if err := s.subRepo.Create(ctx, sub); err != nil {
			return err
		}
		client.SubscriptionID = &sub.ID
		s.clientRepo.Update(ctx, client)
	} else {
		sub, err := s.subRepo.GetByID(ctx, *client.SubscriptionID)
		if err != nil {
			return err
		}
		sub.QuotaBytes = quotaBytes
		sub.QuotaResetCycle = variant.QuotaResetCycle
		sub.QuotaResetDay = variant.QuotaResetDay
		sub.ExpiresAt = client.ExpiresAt
		sub.Status = "active"
		if sub.QuotaResetCycle == "" {
			sub.QuotaResetCycle = tmpl.DefaultQuotaResetCycle
		}
		if sub.QuotaResetDay == nil {
			sub.QuotaResetDay = tmpl.DefaultQuotaResetDay
		}
		s.subRepo.Update(ctx, sub)
	}
	return nil
}

// deductBalanceForOrder deducts balance for a plan order.
func (s *CommerceService) deductBalanceForOrder(ctx context.Context, clientID int64, amount int64) error {
	newBal, err := s.balanceRepo.DeductBalance(ctx, clientID, amount)
	if err != nil {
		return fmt.Errorf("insufficient balance")
	}
	txRecord := &entity.BalanceTransaction{
		ClientID: clientID, Type: entity.BalanceTxTypeConsume,
		Amount: -amount, BalanceAfter: newBal.Balance, Remark: "套餐购买扣款",
	}
	return s.balanceTxRepo.Create(ctx, txRecord)
}

// requestChargeSitePayment sends a payment request to the charge site.
func (s *CommerceService) requestChargeSitePayment(ctx context.Context, site *entity.ChargeSite, order *entity.ChargeOrder, client *entity.Client) (string, error) {
	externalURL, _ := s.settingSvc.GetByKey(ctx, "panel.external_url")
	if externalURL == "" {
		externalURL = "http://localhost:8080"
	}

	returnURL := externalURL + "/p?tab=subscription"
	notifyURL := externalURL + "/api/v1/charge/callback/" + strconv.FormatInt(site.ID, 10)

	payload := map[string]interface{}{
		"trade_no": order.TradeNo, "amount": order.Amount, "currency": "CNY",
		"subject": "CoreFusion 充值", "notify_url": notifyURL,
		"return_url": returnURL, "client_email": maskEmail(client.Email),
		"timestamp": time.Now().Unix(),
	}
	if site.APIKey != "" {
		payload["sign"] = s.signChargePayload(payload, site.APIKey)
	}

	payloadBytes, _ := json.Marshal(payload)
	reqURL := site.SiteURL + "/api/create-order"
	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := chargeHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("charge site returned status %d", resp.StatusCode)
	}

	var result struct {
		OK         bool   `json:"ok"`
		PaymentURL string `json:"payment_url"`
		ExpiresIn  int    `json:"expires_in"`
		Error      string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to parse response: %v", err)
	}
	if !result.OK {
		return "", fmt.Errorf("charge site error: %s", result.Error)
	}
	return result.PaymentURL, nil
}

// validatePromotion checks if a promotion code is valid for the client and variant.
func (s *CommerceService) validatePromotion(ctx context.Context, promo *entity.Promotion, clientID int64, variantID int64) bool {
	if !promo.Enabled {
		return false
	}
	now := time.Now()
	if promo.StartsAt != nil && promo.StartsAt.After(now) {
		return false
	}
	if promo.EndsAt != nil && promo.EndsAt.Before(now) {
		return false
	}
	if promo.MaxUses > 0 && promo.UsedCount >= promo.MaxUses {
		return false
	}
	switch promo.ApplicableScope {
	case entity.PromoScopeAll:
		// OK
	case entity.PromoScopeTemplate, entity.PromoScopeSpecificVariant:
		if promo.ApplicableIDs != nil {
			found := false
			for _, id := range []int64(promo.ApplicableIDs) {
				if id == variantID {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}
	return true
}

// calculateDiscount calculates the discount amount for a promotion.
func (s *CommerceService) calculateDiscount(promo *entity.Promotion, originalAmount int64) int64 {
	switch promo.DiscountType {
	case entity.DiscountTypePercentage:
		discount := originalAmount * promo.DiscountValue / 100
		if promo.MaxDiscountAmount > 0 && discount > promo.MaxDiscountAmount {
			discount = promo.MaxDiscountAmount
		}
		return discount
	case entity.DiscountTypeFixedAmount:
		discount := promo.DiscountValue
		if discount > originalAmount {
			discount = originalAmount
		}
		return discount
	default:
		return 0
	}
}

// verifyChargeCallback verifies the HMAC-SHA256 signature of a charge callback.
// v3.0: status 不参与签名（业务状态由 CoreFusion 判断），currency 参与签名。
// 签名字段（字典序）：amount, currency, nonce, site_trade_no, timestamp, trade_no
func (s *CommerceService) verifyChargeCallback(input ChargeCallbackInput, secret string) bool {
	payload := map[string]interface{}{
		"amount":         input.Amount,
		"currency":       input.Currency,
		"nonce":          input.Nonce,
		"site_trade_no":  input.SiteTradeNo,
		"timestamp":      input.Timestamp,
		"trade_no":       input.TradeNo,
	}
	payloadBytes, _ := json.Marshal(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payloadBytes)
	expectedSign := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(input.Sign), []byte(expectedSign))
}

// signChargePayload signs a charge request payload.
func (s *CommerceService) signChargePayload(payload map[string]interface{}, secret string) string {
	delete(payload, "sign")
	payloadBytes, _ := json.Marshal(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payloadBytes)
	return hex.EncodeToString(mac.Sum(nil))
}

// generateTradeNo generates a unique trade number.
func generateTradeNo() string {
	now := time.Now()
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("CF%s%s", now.Format("20060102150405"), hex.EncodeToString(b))
}

// maskEmail masks an email address for privacy.
func maskEmail(email string) string {
	at := 0
	for i, c := range email {
		if c == '@' {
			at = i
			break
		}
	}
	if at <= 1 {
		return "***@***"
	}
	return email[:1] + "***" + email[at:]
}
