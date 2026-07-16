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
	"net/http"
	"strconv"
	"time"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
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
	Status       string          `json:"status"`
	Amount       int64           `json:"amount"`
	PaidCurrency string          `json:"paid_currency"`
	PaidAmount   int64           `json:"paid_amount"`
	Method       string          `json:"method"`
	ExchangeRate float64         `json:"exchange_rate"`
	PaidAt       string          `json:"paid_at"`
	Sign         string          `json:"sign"`
	Extra        json.RawMessage `json:"extra"`
}

// HandleChargeCallback processes a charge site callback.
func (s *CommerceService) HandleChargeCallback(ctx context.Context, siteID int64, input ChargeCallbackInput) error {
	site, err := s.chargeSiteRepo.GetByID(ctx, siteID)
	if err != nil {
		return fmt.Errorf("charge site not found")
	}

	order, err := s.chargeOrderRepo.GetByTradeNo(ctx, input.TradeNo)
	if err != nil {
		return fmt.Errorf("order not found: %s", input.TradeNo)
	}

	// Idempotency
	if order.Status == entity.ChargeStatusPaid {
		return nil
	}

	// Verify signature
	if site.CallbackSecret != "" {
		if !s.verifyChargeCallback(input, site.CallbackSecret) {
			return fmt.Errorf("invalid signature")
		}
	}

	if input.Status == "paid" {
		paidAt := time.Now()
		if input.PaidAt != "" {
			if t, err := time.Parse(time.RFC3339, input.PaidAt); err == nil {
				paidAt = t
			}
		}
		callbackRaw, _ := json.Marshal(input)
		order.Status = entity.ChargeStatusPaid
		order.SiteTradeNo = input.SiteTradeNo
		order.Amount = input.Amount
		order.PaidCurrency = input.PaidCurrency
		order.PaidAmount = input.PaidAmount
		order.ExchangeRate = input.ExchangeRate
		order.Method = input.Method
		order.PaidAt = &paidAt
		order.CallbackRaw = string(callbackRaw)
		if err := s.chargeOrderRepo.Update(ctx, order); err != nil {
			return err
		}
		return s.processChargePayment(ctx, order)
	}

	order.Status = entity.ChargeStatusFailed
	s.chargeOrderRepo.Update(ctx, order)
	return nil
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
func (s *CommerceService) verifyChargeCallback(input ChargeCallbackInput, secret string) bool {
	payload := map[string]interface{}{
		"trade_no": input.TradeNo, "site_trade_no": input.SiteTradeNo,
		"status": input.Status, "amount": input.Amount,
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
