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
	"gorm.io/gorm/clause"

	"corefusion/internal/model/entity"
	"corefusion/internal/security/crypto"
	"corefusion/internal/service/system"
	"corefusion/internal/store/repository"
)

// ============================================================
// CommerceService — 客户端商业服务
// 套餐展示 / 订单管理 / 充值系统 / 余额系统 / 流量叠加包
// ============================================================

// chargeHTTPClient is a shared HTTP client with timeout for charge site requests.
var chargeHTTPClient = &http.Client{Timeout: 10 * time.Second}

// UserSyncer defines the interface for syncing user credentials to nodes.
// Implemented by usersync.UserSyncService. Used to remove user accounts from
// all nodes when a plan is refunded and expired.
// This avoids circular dependency between clientauth and usersync packages.
type UserSyncer interface {
	SyncUserToServers(ctx context.Context, clientID int64, action string) error
}

// AccountSyncer triggers account synchronization for a plan template.
// Implemented by plan.Service. Reuses the existing SyncAccountsForTemplate
// → SyncAccountsForNewInbound → InboundManager.AddUserToInbound chain:
// generates UUID/Password, persists to config_json, and hot-adds via xray gRPC
// for both local and remote nodes.
// This avoids duplicating account creation logic in commerce.go.
type AccountSyncer interface {
	SyncAccountsForTemplate(ctx context.Context, templateID int64) (int, error)
}

// CommerceService handles plan display, ordering, charging, and balance operations.
type CommerceService struct {
	db              *gorm.DB
	tmplRepo        *repository.PlanTemplateRepository
	variantRepo     *repository.PlanVariantRepository
	orderRepo       *repository.PlanOrderRepository
	promotionRepo   *repository.PromotionRepository
	clientRepo      *repository.ClientRepository
	subRepo         *repository.SubscriptionRepository
	chargeSiteRepo  *repository.ChargeSiteRepository
	chargeOrderRepo *repository.ChargeOrderRepository
	chargeCardRepo  *repository.ChargeCardRepository
	balanceRepo     *repository.ClientBalanceRepository
	balanceTxRepo   *repository.BalanceTransactionRepository
	referralSvc     *ReferralService // P1: 首次付费返利触发
	userSyncer      UserSyncer       // 退款过期后从节点移除用户凭证
	accountSyncer   AccountSyncer    // 套餐激活后同步创建入站用户账号
	settingSvc      SettingStore
	jwtSecret       string
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

// SetReferralService sets the referral service (called from router initialization).
// P1: 首次付费返利触发 — 套餐激活后自动发放返利
func (s *CommerceService) SetReferralService(svc *ReferralService) {
	s.referralSvc = svc
}

// SetUserSyncer sets the user syncer (called from router initialization).
// 用于退款过期后从所有节点移除用户凭证（本地+远程）
func (s *CommerceService) SetUserSyncer(syncer UserSyncer) {
	s.userSyncer = syncer
}

// SetAccountSyncer sets the account syncer (called from router initialization).
// 套餐激活后自动为客户端在所有入站创建账号，复用 plan.Service.SyncAccountsForTemplate
// 已有链路：生成 UUID/Password → 写 config_json → xray gRPC 热添加（本地+远程）
func (s *CommerceService) SetAccountSyncer(syncer AccountSyncer) {
	s.accountSyncer = syncer
}

// triggerFirstPaid triggers the first-paid referral commission if applicable.
// P1: 首次付费返利触发 — 非阻塞，失败仅记录日志
func (s *CommerceService) triggerFirstPaid(ctx context.Context, clientID int64, orderAmount int64) {
	if s.referralSvc == nil {
		return
	}
	if err := s.referralSvc.ProcessFirstPaid(ctx, clientID, orderAmount); err != nil {
		fmt.Printf("[commerce] failed to process first paid referral: %v\n", err)
	}
}

// ============================================================
// P0-1: Plan Display
// ============================================================

// PlanNodeInfo is a simplified node info for plan display (no sensitive connection details).
type PlanNodeInfo struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Protocol  string `json:"protocol"`
	Transport string `json:"transport"`
	Region    string `json:"region"`
	Type      string `json:"type"` // "inbound" or "external"
}

// PlanListItem is a simplified plan template for client display.
type PlanListItem struct {
	ID          int64             `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Category    string            `json:"category"`
	SortOrder   int               `json:"sort_order"`
	Variants    []PlanVariantItem `json:"variants"`
	Nodes       []PlanNodeInfo    `json:"nodes"`
}

// PlanVariantItem is a simplified plan variant for client display.
type PlanVariantItem struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	VariantType   string `json:"variant_type"` // plan/traffic_pack
	Description   string `json:"description"`
	BillingCycle  string `json:"billing_cycle"`
	DurationDays  int    `json:"duration_days"`
	Price         int64  `json:"price"`          // 分
	OriginalPrice int64  `json:"original_price"` // 分
	Currency      string `json:"currency"`
	QuotaBytes    int64  `json:"quota_bytes"`
	ExpiryMode    string `json:"expiry_mode"` // traffic pack: align_with_plan/extend_plan
	IsRecommended bool   `json:"is_recommended"`
	IsDefault     bool   `json:"is_default"`
}

// PlanDetailItem is a detailed plan template for client display.
type PlanDetailItem struct {
	PlanListItem
	DeviceLimit      int     `json:"device_limit"`
	SpeedLimitMbps   int     `json:"speed_limit_mbps"`
	SpeedLimitUpMbps int     `json:"speed_limit_up_mbps"`
	ConcurrentLimit  int     `json:"concurrent_limit"`
	NodeGroupIDs     []int64 `json:"node_group_ids"`
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
			Nodes: s.getPlanNodes(ctx, &tmpl),
		})
	}
	return items, nil
}

// getPlanNodes returns the list of nodes associated with a plan template (names only, no links).
func (s *CommerceService) getPlanNodes(ctx context.Context, tmpl *entity.PlanTemplate) []PlanNodeInfo {
	if len(tmpl.NodeGroupIDs) == 0 {
		return []PlanNodeInfo{}
	}

	var members []entity.NodeGroupMember
	if err := s.db.WithContext(ctx).
		Where("group_id IN ?", []int64(tmpl.NodeGroupIDs)).
		Where("enabled = ?", true).
		Find(&members).Error; err != nil {
		return []PlanNodeInfo{}
	}

	var nodes []PlanNodeInfo
	seenInbound := make(map[int64]bool)
	seenExt := make(map[int64]bool)

	for _, m := range members {
		if m.InboundConfigID > 0 && !seenInbound[m.InboundConfigID] {
			var ib entity.InboundConfig
			if err := s.db.WithContext(ctx).First(&ib, m.InboundConfigID).Error; err == nil && ib.Enabled {
				name := ib.Remark
				if name == "" {
					name = ib.Protocol + "-" + ib.Region
				}
				transport := parseInboundTransport(ib.ConfigJSON)
				nodes = append(nodes, PlanNodeInfo{
					ID: ib.ID, Name: name, Protocol: ib.Protocol, Transport: transport, Region: ib.Region, Type: "inbound",
				})
				seenInbound[m.InboundConfigID] = true
			}
		}
		if m.ExternalNodeID > 0 && !seenExt[m.ExternalNodeID] {
			var en entity.ExternalNode
			if err := s.db.WithContext(ctx).First(&en, m.ExternalNodeID).Error; err == nil && en.Enabled {
				transport := en.Transport
				if en.TLS {
					if transport != "" {
						transport += "+tls"
					} else {
						transport = "tls"
					}
				}
				nodes = append(nodes, PlanNodeInfo{
					ID: en.ID, Name: en.Name, Protocol: en.Protocol, Transport: transport, Region: "", Type: "external",
				})
				seenExt[m.ExternalNodeID] = true
			}
		}
	}
	return nodes
}

// parseInboundTransport extracts the transport network type from an inbound config JSON.
// Returns a human-readable string like "tcp", "ws", "grpc", "tcp+tls", "grpc+reality", etc.
func parseInboundTransport(configJSON string) string {
	if configJSON == "" {
		return ""
	}
	var cfg struct {
		StreamSettings struct {
			Network  string `json:"network"`
			Security string `json:"security"`
		} `json:"streamSettings"`
	}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return ""
	}
	network := cfg.StreamSettings.Network
	security := cfg.StreamSettings.Security
	// Map xray network names to user-friendly labels
	switch network {
	case "raw", "tcp":
		network = "tcp"
	case "ws":
		network = "ws"
	case "grpc":
		network = "grpc"
	case "httpupgrade":
		network = "httpupgrade"
	case "xhttp":
		network = "xhttp"
	}
	result := network
	if security != "" && security != "none" {
		result += "+" + security
	}
	return result
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
		SpeedLimitUpMbps: tmpl.SpeedLimitUpMbps,
		ConcurrentLimit:  tmpl.ConcurrentLimit, NodeGroupIDs: nodeGroupIDs,
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
			ID: v.ID, Name: v.Name, VariantType: v.VariantType, Description: v.Description,
			BillingCycle: v.BillingCycle, DurationDays: v.DurationDays,
			Price: v.Price, OriginalPrice: v.OriginalPrice, Currency: v.Currency,
			QuotaBytes: v.QuotaBytes, ExpiryMode: v.ExpiryMode,
			IsRecommended: v.IsRecommended, IsDefault: v.IsDefault,
		})
	}
	return items, nil
}

// BuyTrafficPackInput is the input for buying a traffic pack.
type BuyTrafficPackInput struct {
	VariantID    int64  `json:"variant_id" binding:"required"`
	PayMethod    string `json:"pay_method"` // balance/charge_site
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
		// 先创建订单再扣款，这样流水可以关联订单 ID
		now := time.Now()
		order := &entity.PlanOrder{
			ClientID: clientID, OrderType: entity.OrderTypeNew, ToVariantID: input.VariantID,
			OriginalAmount: paidAmount, PaidAmount: paidAmount, EffectiveFrom: now,
			Status: entity.OrderStatusCompleted, Remark: "流量叠加包购买",
		}
		if err := s.orderRepo.Create(ctx, order); err != nil {
			return nil, fmt.Errorf("failed to create order: %v", err)
		}
		if paidAmount > 0 {
			if err := s.deductBalanceForOrder(ctx, clientID, paidAmount, &order.ID); err != nil {
				return nil, fmt.Errorf("insufficient balance: %v", err)
			}
		}
		// Apply traffic pack (adds traffic, optionally extends expiry)
		if err := s.applyTrafficPack(ctx, client, variant); err != nil {
			return nil, fmt.Errorf("failed to apply traffic pack: %v", err)
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
			SiteID: input.ChargeSiteID, Amount: paidAmount, Currency: site.Currency, PlanVariantID: &input.VariantID,
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

// applyTrafficPack applies a traffic pack to a client's subscription.
// Depending on the variant's ExpiryMode, it may also extend the plan's expiry date:
//   - align_with_plan (default): only adds traffic bytes, does not change expiry.
//   - extend_plan: adds traffic bytes AND extends client/subscription expiry by DurationDays.
//
// Traffic bytes are CUMULATIVE: they are added to both client.QuotaBytes and
// subscription.QuotaBytes, not replacing the existing quota.
func (s *CommerceService) applyTrafficPack(ctx context.Context, client *entity.Client, variant *entity.PlanVariant) error {
	now := time.Now()

	// 1. Add traffic bytes to subscription (cumulative)
	if err := s.addTrafficToSubscription(ctx, client, variant.QuotaBytes); err != nil {
		return err
	}

	// 2. Also add to client.QuotaBytes (cumulative, keeps client table in sync)
	client.QuotaBytes += variant.QuotaBytes
	client.UpdatedAt = now

	// 3. Extend plan expiry if mode is extend_plan
	if variant.ExpiryMode == entity.ExpiryModeExtendPlan && variant.DurationDays > 0 {
		var newExpiry time.Time
		if client.ExpiresAt != nil && client.ExpiresAt.After(now) {
			newExpiry = client.ExpiresAt.AddDate(0, 0, variant.DurationDays)
		} else {
			newExpiry = now.AddDate(0, 0, variant.DurationDays)
		}
		client.ExpiresAt = &newExpiry

		// Also update subscription expiry
		if client.SubscriptionID != nil {
			sub, err := s.subRepo.GetByID(ctx, *client.SubscriptionID)
			if err == nil {
				sub.ExpiresAt = &newExpiry
				s.subRepo.Update(ctx, sub)
			}
		}

		log.Printf("[commerce] traffic pack %d extended client %d expiry by %d days to %v",
			variant.ID, client.ID, variant.DurationDays, newExpiry)
	}

	// Update client record (quota + optionally expiry)
	if err := s.clientRepo.Update(ctx, client); err != nil {
		return fmt.Errorf("failed to update client after traffic pack: %v", err)
	}

	// Sync updated client config to all core servers
	if s.userSyncer != nil {
		go s.userSyncer.SyncUserToServers(context.Background(), client.ID, "update")
	}

	log.Printf("[commerce] traffic pack %d applied to client %d: +%d bytes (total: %d)",
		variant.ID, client.ID, variant.QuotaBytes, client.QuotaBytes)

	return nil
}

// ============================================================
// P0-2: Order Management
// ============================================================

// CreateOrderInput is the input for creating a plan order.
type CreateOrderInput struct {
	VariantID     int64  `json:"variant_id" binding:"required"`
	OrderType     string `json:"order_type" binding:"required"` // new/renew/upgrade
	PromotionCode string `json:"promotion_code"`
	PayMethod     string `json:"pay_method"` // balance/charge_site/gift_card
	ChargeSiteID  *int64 `json:"charge_site_id"`
	GiftCardCode  string `json:"gift_card_code"` // v3.3: 套餐卡兑换码
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
			// Allow a client on an active trial to buy a real (non-trial) plan:
			// the purchase replaces the trial immediately (buy = convert).
			curTmpl, terr := s.tmplRepo.GetByID(ctx, client.PlanID)
			onTrial := terr == nil && curTmpl != nil && curTmpl.IsTrial
			if !onTrial || tmpl.IsTrial {
				return nil, fmt.Errorf("you already have an active plan, please use renew or upgrade")
			}
		}
		// Trial plans can only be claimed once per client.
		if tmpl.IsTrial {
			var trialOrders int64
			s.db.WithContext(ctx).Model(&entity.PlanOrder{}).
				Where("client_id = ?", clientID).
				Where("to_variant_id IN (?)", s.db.Model(&entity.PlanVariant{}).Select("plan_variants.id").
					Joins("JOIN plan_templates ON plan_templates.id = plan_variants.template_id").
					Where("plan_templates.is_trial = ?", true)).
				Count(&trialOrders)
			if trialOrders > 0 {
				return nil, fmt.Errorf("trial plan can only be claimed once")
			}
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
			s.promotionRepo.TryIncrementUsedCount(ctx, promo.ID, promo.MaxUses)
		}
	}

	paidAmount := originalAmount - discountAmount
	if paidAmount < 0 {
		paidAmount = 0
	}

	now := time.Now()

	if input.PayMethod == "balance" {
		// Pre-check balance for user-friendly error message
		if paidAmount > 0 {
			bal, err := s.balanceRepo.GetByClientID(ctx, clientID)
			haveBalance := int64(0)
			if err == nil && bal != nil {
				haveBalance = bal.Balance
			}
			if err != nil || haveBalance < paidAmount {
				return nil, fmt.Errorf("INSUFFICIENT_BALANCE: need %d, have %d", paidAmount, haveBalance)
			}
		}
		order := &entity.PlanOrder{
			ClientID: clientID, OrderType: input.OrderType, ToVariantID: input.VariantID,
			OriginalAmount: originalAmount, DiscountAmount: discountAmount, PaidAmount: paidAmount,
			PromotionID: promotionID, PromotionCode: input.PromotionCode,
			EffectiveFrom: now, Status: entity.OrderStatusCompleted,
		}
		// S-3 fix: wrap order creation + balance deduction in a transaction so
		// a failure in either step leaves no orphan order or missing deduction.
		var orderID int64
		err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(order).Error; err != nil {
				return fmt.Errorf("failed to create order: %v", err)
			}
			orderID = order.ID
			if paidAmount > 0 {
				// Atomic conditional deduction: only deduct if balance is sufficient.
				result := tx.Model(&entity.ClientBalance{}).
					Where("client_id = ? AND balance >= ?", clientID, paidAmount).
					Updates(map[string]interface{}{
						"balance":    gorm.Expr("balance - ?", paidAmount),
						"updated_at": now,
					})
				if result.Error != nil {
					return result.Error
				}
				if result.RowsAffected == 0 {
					return fmt.Errorf("INSUFFICIENT_BALANCE")
				}
				// Read updated balance for the ledger record
				var bal entity.ClientBalance
				tx.Where("client_id = ?", clientID).First(&bal)
				txRecord := &entity.BalanceTransaction{
					ClientID: clientID, Type: entity.BalanceTxTypeConsume,
					Amount: -paidAmount, BalanceBefore: bal.Balance + paidAmount, BalanceAfter: bal.Balance,
					RelatedPlanOrderID: &orderID, Remark: "余额购买套餐扣款",
				}
				if err := tx.Create(txRecord).Error; err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		if err := s.activatePlan(ctx, client, tmpl, variant, input.OrderType); err != nil {
			return nil, fmt.Errorf("failed to activate plan: %v", err)
		}
		// P1: 首次付费返利触发（余额支付）
		s.triggerFirstPaid(ctx, clientID, paidAmount)
		// 发送订单支付成功通知（模板通知引擎）
		go func() {
			_ = system.NotifyTemplate(context.Background(), "order_paid", map[string]string{
				"order_id":  fmt.Sprintf("%d", order.ID),
				"amount":    fmt.Sprintf("%d", paidAmount),
				"email":     client.Email,
				"plan_name": tmpl.Name,
			})
		}()
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
			SiteID: input.ChargeSiteID, Amount: paidAmount, Currency: site.Currency, PlanVariantID: &input.VariantID,
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

	} else if input.PayMethod == "gift_card" {
		// v3.3: 套餐卡支付
		if input.GiftCardCode == "" {
			return nil, fmt.Errorf("gift_card_code is required for gift_card payment")
		}
		// 查找并验证套餐卡
		card, err := s.chargeCardRepo.GetByCardCode(ctx, input.GiftCardCode)
		if err != nil {
			return nil, fmt.Errorf("invalid gift card code")
		}
		// 验证套餐卡状态
		if card.Status != entity.ChargeCardStatusActive {
			return nil, fmt.Errorf("gift card is not active")
		}
		if card.UsedCount > 0 {
			return nil, fmt.Errorf("gift card has already been used")
		}
		if card.ExpiresAt != nil && card.ExpiresAt.Before(time.Now()) {
			return nil, fmt.Errorf("gift card has expired")
		}
		// 验证套餐卡类型 - 必须是套餐型或流量叠加包型
		if card.CardType != entity.ChargeCardTypePlan && card.CardType != entity.ChargeCardTypeTrafficPack {
			return nil, fmt.Errorf("gift card is not a plan/traffic_pack card")
		}
		// 验证套餐卡价值是否匹配订单
		if card.Value != input.VariantID {
			return nil, fmt.Errorf("gift card does not match the selected plan variant")
		}

		// 流量叠加包类型处理
		if card.CardType == entity.ChargeCardTypeTrafficPack {
			// S-2 fix: wrap card status update + order creation in a transaction with
			// row lock and conditional WHERE to prevent concurrent double-use.
			var trafficPackOrderID int64
			err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
				// Lock card row
				var lockedCard entity.ChargeCard
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
					Where("id = ?", card.ID).First(&lockedCard).Error; err != nil {
					return err
				}
				if lockedCard.Status != entity.ChargeCardStatusActive {
					return fmt.Errorf("gift card is not active")
				}
				// Atomic conditional update: only succeeds if card is still active
				result := tx.Model(&entity.ChargeCard{}).
					Where("id = ? AND status = ?", lockedCard.ID, entity.ChargeCardStatusActive).
					Updates(map[string]interface{}{
						"status":     entity.ChargeCardStatusUsed,
						"used_count": 1,
					})
				if result.Error != nil {
					return result.Error
				}
				if result.RowsAffected == 0 {
					return fmt.Errorf("gift card has already been used")
				}
				// Create ChargeOrder — inherit currency from the card
				chargeOrder := &entity.ChargeOrder{
					ClientID:      clientID,
					TradeNo:       generateTradeNo(),
					Source:        entity.ChargeSourceCard,
					Amount:        0,
					Currency:      card.Currency,
					PlanVariantID: &input.VariantID,
					OrderType:     entity.ChargeOrderTypePurchase,
					Status:        entity.ChargeStatusPaid,
					ChargeCardID:  &card.ID,
					PaidAt:        &now,
				}
				if err := tx.Create(chargeOrder).Error; err != nil {
					return err
				}
				trafficPackOrderID = chargeOrder.ID
				return nil
			})
			if err != nil {
				return nil, fmt.Errorf("failed to redeem gift card: %v", err)
			}
			_ = trafficPackOrderID

			// Apply traffic pack outside transaction (uses s.clientRepo)
			if err := s.applyTrafficPack(ctx, client, variant); err != nil {
				return nil, fmt.Errorf("failed to apply traffic pack: %v", err)
			}

			// P1: 首次付费返利触发（流量包卡支付）
			s.triggerFirstPaid(ctx, clientID, originalAmount)

			return &CreateOrderResult{OrderID: 0, Status: entity.OrderStatusCompleted}, nil
		}

		// 套餐型处理
		// S-2 fix: wrap card status update + order creation in a transaction with
		// row lock and conditional WHERE to prevent concurrent double-use.
		var planOrderID int64
		err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			// Lock card row
			var lockedCard entity.ChargeCard
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("id = ?", card.ID).First(&lockedCard).Error; err != nil {
				return err
			}
			if lockedCard.Status != entity.ChargeCardStatusActive {
				return fmt.Errorf("gift card is not active")
			}
			// Atomic conditional update
			result := tx.Model(&entity.ChargeCard{}).
				Where("id = ? AND status = ?", lockedCard.ID, entity.ChargeCardStatusActive).
				Updates(map[string]interface{}{
					"status":     entity.ChargeCardStatusUsed,
					"used_count": 1,
				})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return fmt.Errorf("gift card has already been used")
			}
			// Create ChargeOrder for tracking — inherit currency from the card
			chargeOrder := &entity.ChargeOrder{
				ClientID:      clientID,
				TradeNo:       generateTradeNo(),
				Source:        entity.ChargeSourceCard,
				Amount:        0,
				Currency:      card.Currency,
				PlanVariantID: &input.VariantID,
				OrderType:     entity.ChargeOrderTypePurchase,
				Status:        entity.ChargeStatusPaid,
				ChargeCardID:  &card.ID,
				PaidAt:        &now,
			}
			if err := tx.Create(chargeOrder).Error; err != nil {
				return err
			}
			// Create completed plan order
			order := &entity.PlanOrder{
				ClientID: clientID, OrderType: input.OrderType, ToVariantID: input.VariantID,
				OriginalAmount: originalAmount, DiscountAmount: discountAmount, PaidAmount: 0,
				PromotionID: promotionID, PromotionCode: input.PromotionCode,
				EffectiveFrom: now, Status: entity.OrderStatusCompleted,
				Remark: "套餐卡支付: " + card.CardCodeMasked,
			}
			if err := tx.Create(order).Error; err != nil {
				return err
			}
			planOrderID = order.ID
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("failed to redeem gift card: %v", err)
		}

		// 激活套餐
		if err := s.activatePlan(ctx, client, tmpl, variant, input.OrderType); err != nil {
			return nil, fmt.Errorf("failed to activate plan: %v", err)
		}

		// P1: 首次付费返利触发（套餐卡支付 — 按原价计算返利，套餐卡等价于预付原价）
		s.triggerFirstPaid(ctx, clientID, originalAmount)

		return &CreateOrderResult{OrderID: planOrderID, Status: entity.OrderStatusCompleted}, nil
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
		SiteID: &input.SiteID, Amount: input.Amount, Currency: site.Currency, PlanVariantID: input.PlanVariantID,
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
	Status       string          `json:"status"` // 业务状态，不参与签名
	Amount       int64           `json:"amount"`
	Currency     string          `json:"currency"` // 订单货币（参与签名）
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
// v3.2 verification chain:
//  1. request_id  2. site check  3. verified check
//  4. timestamp + nonce check  5. nonce atomic INSERT (replay prevention, sign_valid=false)
//  6. HMAC verify (version + body_hash + secret_version) → mark nonce sign_valid
//  7. order lookup + belong  8. callback_count++ / last_callback_at (异常频率检测)
//  9. status check (幂等)  10. amount + currency lock check
//  11. v3.2 Transaction: SELECT order FOR UPDATE → SELECT balance FOR UPDATE → INSERT ledger → UPDATE balance → UPDATE order(paid) → COMMIT
//  12. Async: callback_event (事务外，不阻塞账务)
//
// 安全优先级：HMAC P0 / nonce P0 / 金额+币种 P0 / IP白名单 P2(仅记录不阻断)
//
// v3.2: rawBody 是原始 HTTP 请求体字节，用于计算 body_hash = SHA256(raw_body)。
// 不能用 json.Marshal(input) 重新序列化，因为 key 顺序可能与客户端发送的原始 body 不同。
func (s *CommerceService) HandleChargeCallback(ctx context.Context, siteID int64, remoteIP string, requestID string, rawBody []byte, input ChargeCallbackInput) error {
	if requestID == "" {
		requestID = generateRequestID()
	}
	payloadHash := sha256Hex(rawBody) // 事件/重试记录沿用"完整 body"的哈希 ✓（存储语义不变 ✓）
	// body_hash 两个候选 ✓：见 callbackBodyHashCandidates 的说明 ——
	// 官方文档要求"先算 body_hash、再把 sign 放进 body" ✓，而旧实现对**含 sign 的完整 body**
	// 取哈希 ✗ → 循环依赖 ✗ → 按文档接入的充值站点必然验签失败 ✗（回调恒 400 ✓）。
	bodyHash, bodyHashFull := callbackBodyHashCandidates(rawBody)

	// v3.2: event 改为异步写入，不阻塞账务
	recordEvent := func(result, errorCode, errMsg string, signValid bool, secretVer int, ipMatchRule string) {
		go s.recordCallbackEventAsync(siteID, requestID, input.TradeNo, remoteIP, ipMatchRule, payloadHash, input.Amount, result, errorCode, errMsg, signValid, secretVer)
	}
	raiseSecEvent := func(eventType, severity, tradeNo, detail string) {
		go s.recordSecurityEventAsync(siteID, tradeNo, requestID, eventType, severity, remoteIP, detail)
	}

	// ② site
	site, err := s.chargeSiteRepo.GetByID(ctx, siteID)
	if err != nil {
		log.Printf("[callback] request_id=%s site_id=%d error=site_not_found", requestID, siteID)
		recordEvent("error", "SITE_NOT_FOUND", "charge site not found", false, 0, "")
		return fmt.Errorf("charge site not found")
	}
	log.Printf("[callback] request_id=%s site_id=%d remote_ip=%s", requestID, siteID, remoteIP)

	// ③ verified (含过期检查)
	if !site.Verified {
		recordEvent("error", "SITE_NOT_VERIFIED", "site not verified", false, 0, "")
		return fmt.Errorf("site not verified")
	}
	if site.VerifyExpireAt != nil && site.VerifyExpireAt.Before(time.Now()) {
		recordEvent("error", "SITE_NOT_VERIFIED", "site verification expired", false, 0, "")
		return fmt.Errorf("site verification expired")
	}
	log.Printf("[callback] request_id=%s verified=ok", requestID)

	// ④ IP whitelist — v3.3: 配置了白名单就严格阻断
	// 回调是支付平台服务器间请求（server-to-server），IP 固定可预测。
	// 管理员配置了白名单说明知道支付平台 IP 范围，此时应严格阻断非白名单 IP。
	// 未配置白名单时跳过检查（依赖 HMAC 签名保护）。
	ipMatchRule := ""
	if site.CallbackWhitelist != "" {
		ipMatchRule = findIPMatchRule(remoteIP, site.CallbackWhitelist)
		if ipMatchRule == "" {
			log.Printf("[callback] request_id=%s ip_whitelist=BLOCKED ip=%s", requestID, remoteIP)
			raiseSecEvent(entity.ChargeSecEventIPChanged, entity.ChargeSecSeverityCritical, input.TradeNo,
				fmt.Sprintf("ip not in whitelist, blocked: %s", remoteIP))
			recordEvent("error", "IP_BLOCKED", fmt.Sprintf("ip not in whitelist: %s", remoteIP), false, 0, "")
			return fmt.Errorf("ip not in whitelist: %s", remoteIP)
		}
	} else {
		ipMatchRule = "no_whitelist"
	}
	log.Printf("[callback] request_id=%s ip_whitelist=ok matched=%s", requestID, ipMatchRule)

	// ⑤ timestamp
	now := time.Now().Unix()
	if input.Timestamp == 0 || absInt64(now-input.Timestamp) > 300 {
		log.Printf("[callback] request_id=%s timestamp=expired", requestID)
		recordEvent("replay", "REPLAY_ATTACK", "callback timestamp expired", false, 0, ipMatchRule)
		return fmt.Errorf("callback timestamp expired (possible replay attack)")
	}
	if input.Nonce == "" {
		recordEvent("replay", "REPLAY_ATTACK", "nonce is required", false, 0, ipMatchRule)
		return fmt.Errorf("nonce is required")
	}
	log.Printf("[callback] request_id=%s timestamp=ok", requestID)

	// ⑥ nonce 原子消费（v3.2: sign_valid=false，验签后更新）
	// ⑥ nonce 原子消费（sign_valid=false，验签后更新；同时记录载荷哈希 ✓）
	nonceRecord := &entity.CallbackNonce{
		Nonce:       input.Nonce,
		SiteID:      siteID,
		TradeNo:     input.TradeNo,
		SignValid:   false, // 默认未通过，验签成功后更新
		PayloadHash: payloadHash,
		CreatedAt:   time.Now(),
	}
	if err := s.db.WithContext(ctx).Create(nonceRecord).Error; err != nil {
		// 冲突既可能是**真重放**，也可能是**网关重试** —— 必须区分：
		// 之前一律按重放拒绝，导致"一次因瞬时错误（如 SQLite 写锁）失败的回调，
		// 网关用同一个 nonce 重投也会被拒" → 这笔钱永远入不了账（丢单）。
		// 规则：同 nonce + **同载荷** = 重试（幂等放行；入账侧有唯一约束，不会重复入账）；
		//       同 nonce + **不同载荷** = 真重放（仍然拒绝）。
		var existing entity.CallbackNonce
		lookupErr := s.db.WithContext(ctx).
			Where("site_id = ? AND nonce = ?", siteID, input.Nonce).
			First(&existing).Error
		// 判据（安全且够宽容）：
		//   同 nonce + **同订单号** ⇒ 视为网关重试 → 继续走后面的验签与金额校验
		//     （验签不过照样被拒；金额对不上照样被拒；入账侧幂等，不会重复入账）；
		//   同 nonce + **不同订单号** ⇒ 真重放 → 直接拒绝。
		// 为什么用订单号而不是载荷哈希：真实网关重试有两种形态 ——
		//   ① 逐字节重发同一份 body（载荷相同）；② 重新构造请求（时间戳变了 → 载荷不同，但 nonce/订单不变）。
		// 只认载荷相同会把形态 ② 误判成重放 → 依然丢单，所以用"同一笔订单"作为判据。
		if lookupErr == nil && existing.TradeNo != "" && existing.TradeNo == input.TradeNo {
			log.Printf("[callback] request_id=%s nonce=%s trade_no=%s same_payload=%v 视为网关重试，继续走验签/金额校验",
				requestID, input.Nonce, input.TradeNo, existing.PayloadHash == payloadHash)
			recordEvent("retry", "", "duplicate callback with identical payload (gateway retry)", false, 0, ipMatchRule)
		} else {
			log.Printf("[callback] request_id=%s nonce_replay nonce=%s", requestID, input.Nonce)
			recordEvent("replay", "REPLAY_ATTACK", "nonce already used with different payload", false, 0, ipMatchRule)
			raiseSecEvent(entity.ChargeSecEventNonceReplay, entity.ChargeSecSeverityHigh, input.TradeNo,
				fmt.Sprintf("nonce replay: %s", input.Nonce))
			return fmt.Errorf("nonce already used (replay attack)")
		}
	} else {
		log.Printf("[callback] request_id=%s nonce=consumed", requestID)
	}

	// ⑦ HMAC verify (v3.2: version + body_hash + secret_version)
	// v3.2: 从 charge_site_secrets 表加载所有有效密钥（active + legacy），按版本降序尝试
	secrets, err := s.loadActiveSecrets(ctx, siteID)
	if err != nil {
		log.Printf("[callback] request_id=%s secret=load_error: %v", requestID, err)
	}
	if len(secrets) == 0 {
		// 兼容旧逻辑：从 ChargeSite 字段加载
		secret := resolveSiteSecret(site)
		if secret != "" {
			secrets = []chargeSecretEntry{{version: site.SecretVersion, secret: secret}}
			if prev := resolveSiteSecretPrev(site); prev != "" {
				secrets = append(secrets, chargeSecretEntry{version: site.SecretVersion - 1, secret: prev})
			}
		}
	}
	if len(secrets) == 0 {
		log.Printf("[callback] request_id=%s secret=missing", requestID)
		recordEvent("signature_failed", "SECRET_REQUIRED", "callback_secret not configured", false, 0, ipMatchRule)
		raiseSecEvent(entity.ChargeSecEventSigFailed, entity.ChargeSecSeverityMedium, input.TradeNo, "secret not configured")
		return fmt.Errorf("callback_secret not configured")
	}

	usedSecretVer := 0
	signValid := false
	for _, entry := range secrets {
		// 两个 body_hash 候选都试 ✓：bodyHash=去掉 sign 后算的（文档口径 ✓）、
		// bodyHashFull=完整 body（旧行为 ✓）。安全性不变 ✓（伪造仍需密钥 ✓，
		// body_hash 只约束整体完整性，status/paid_* 本来就不参与签名载荷 ✓）。
		if s.verifyChargeCallback(input, bodyHash, entry.secret) ||
			s.verifyChargeCallback(input, bodyHashFull, entry.secret) {
			signValid = true
			usedSecretVer = entry.version
			break
		}
	}
	if !signValid {
		log.Printf("[callback] request_id=%s verify_signature=failed", requestID)
		// 标记 nonce 签名失败
		s.db.WithContext(ctx).Model(&entity.CallbackNonce{}).
			Where("id = ?", nonceRecord.ID).Update("sign_valid", false)
		recordEvent("signature_failed", "INVALID_SIGNATURE", "invalid signature", false, 0, ipMatchRule)
		raiseSecEvent(entity.ChargeSecEventSigFailed, entity.ChargeSecSeverityCritical, input.TradeNo,
			fmt.Sprintf("invalid signature, remote_ip=%s", remoteIP))
		return fmt.Errorf("invalid signature")
	}

	// 验签成功：标记 nonce sign_valid=true
	s.db.WithContext(ctx).Model(&entity.CallbackNonce{}).
		Where("id = ?", nonceRecord.ID).Update("sign_valid", true)
	log.Printf("[callback] request_id=%s verify_signature=ok v=%d", requestID, usedSecretVer)

	// ⑧ order lookup
	order, err := s.chargeOrderRepo.GetByTradeNo(ctx, input.TradeNo)
	if err != nil {
		log.Printf("[callback] request_id=%s order=not_found trade_no=%s", requestID, input.TradeNo)
		recordEvent("error", "ORDER_NOT_FOUND", fmt.Sprintf("order not found: %s", input.TradeNo), signValid, usedSecretVer, ipMatchRule)
		return fmt.Errorf("order not found: %s", input.TradeNo)
	}

	// ⑨ order belong
	if order.SiteID == nil || *order.SiteID != site.ID {
		log.Printf("[callback] request_id=%s order_not_belong", requestID)
		recordEvent("error", "ORDER_NOT_BELONG", "order does not belong to this site", signValid, usedSecretVer, ipMatchRule)
		return fmt.Errorf("order does not belong to this site")
	}
	// ⑨.5 过期订单：只**记录**这笔"迟到到账"，不改变订单状态（它马上就会被入账成 paid）。
	//
	// 「不再接收付款」在面板侧靠两点实现，都拦在**付款前**：
	//   ① 订单被标注 expired（后台清理器 CleanupExpiredOrders → repo.MarkExpired，默认每 5 分钟；
	//      门户查 GetChargeStatus 就能显示"已过期"，不会让用户再发起这笔付款）；
	//   ② 站点侧自己的收款 TTL（真实网关会关掉付款页）。
	// 而**已经收到的钱必须入账** —— 正常交易逻辑：绝不能"用户付了款还失败"，丢单比过期严重得多。
	//
	// ⚠ 这里刻意**不**再写数据库（原先会先 Update 成 expired 再继续入账）：
	//   那会在入账事务前多插一次写事务，SQLite 下极易撞上 "database is locked"
	//   （真机端到端测试就是这样失败的），而且该状态立刻被 paid 覆盖、毫无意义。
	// 判定条件与 repo.MarkExpired 完全一致（status=pending && expires_at 非空 && expires_at < now）。
	if order.Status == entity.ChargeStatusPending && order.ExpiresAt != nil && order.ExpiresAt.Before(time.Now()) {
		log.Printf("[callback] request_id=%s order=%s late_settlement=1 订单已过收款有效期但收到有效付款，照常入账", requestID, input.TradeNo)
		recordEvent("success", "", "late settlement: expired order paid, credited anyway", signValid, usedSecretVer, ipMatchRule)
	}

	// v3.2: 更新 callback_count / last_callback_at（异常频率检测）
	callbackNow := time.Now()
	s.db.WithContext(ctx).Model(&entity.ChargeOrder{}).
		Where("id = ?", order.ID).
		Updates(map[string]interface{}{
			"callback_count":   gorm.Expr("callback_count + 1"),
			"last_callback_at": callbackNow,
		})
	// 异常频率检测：1分钟内超过10次回调
	if order.LastCallbackAt != nil && callbackNow.Sub(*order.LastCallbackAt) < time.Minute && order.CallbackCount > 10 {
		raiseSecEvent(entity.ChargeSecEventCallbackFlood, entity.ChargeSecSeverityHigh, input.TradeNo,
			fmt.Sprintf("callback flood: %d callbacks in short period", order.CallbackCount))
	}

	// ⑩ callback status whitelist (v3.2: 显式校验，拒绝未知状态)
	switch input.Status {
	case entity.CallbackStatusPaid, entity.CallbackStatusFailed:
		// valid terminal statuses — proceed to state machine
	case entity.CallbackStatusPending:
		// 中间态：支付进行中，仅确认收到回调，不改订单状态
		log.Printf("[callback] request_id=%s order=%s status=pending acknowledged", requestID, input.TradeNo)
		recordEvent("success", "", "callback status is pending, order unchanged", signValid, usedSecretVer, ipMatchRule)
		return nil
	default:
		log.Printf("[callback] request_id=%s order=%s unknown_status=%s", requestID, input.TradeNo, input.Status)
		recordEvent("error", "INVALID_STATUS", fmt.Sprintf("unknown callback status: %s", input.Status), signValid, usedSecretVer, ipMatchRule)
		raiseSecEvent(entity.ChargeSecEventInvalidStatus, entity.ChargeSecSeverityMedium, input.TradeNo,
			fmt.Sprintf("unknown callback status: %s", input.Status))
		return fmt.Errorf("invalid callback status: %s", input.Status)
	}

	// ⑩b order status (幂等 + 状态机)
	if order.Status == entity.ChargeStatusPaid && input.Status == entity.CallbackStatusPaid {
		log.Printf("[callback] request_id=%s order=%s idempotent=paid", requestID, input.TradeNo)
		recordEvent("success", "", "idempotent: order already paid", signValid, usedSecretVer, ipMatchRule)
		return nil
	}
	if order.Status == entity.ChargeStatusProcessing {
		log.Printf("[callback] request_id=%s order=%s processing_skip", requestID, input.TradeNo)
		recordEvent("success", "", "order is processing", signValid, usedSecretVer, ipMatchRule)
		return nil
	}
	targetStatus := entity.ChargeStatusPaid
	if input.Status == entity.CallbackStatusFailed {
		targetStatus = entity.ChargeStatusFailed
	}
	// 订单状态闸门：直接用状态机（单一口径）。
	// 支付网关回调是一次性终态 —— pending → paid/failed 在 chargeOrderTransitions 里
	// 已正式合法，processing → paid/failed 亦然；其余状态（created/expired/refunded）
	// 一律拒绝。这里曾因为状态机不允许 pending → paid 而恒判非法、订单永远到不了账。
	if !entity.CanChargeOrderTransition(order.Status, targetStatus) {
		log.Printf("[callback] request_id=%s invalid_transition from=%s to=%s", requestID, order.Status, targetStatus)
		recordEvent("error", "INVALID_STATUS_TRANSITION", fmt.Sprintf("order status is %s", order.Status), signValid, usedSecretVer, ipMatchRule)
		return fmt.Errorf("order status is %s, cannot process callback", order.Status)
	}

	// ⑪ amount + currency lock check (v3.2: 币种锁定)
	// 充值站点 amount 单位为元，内部 order.Amount 单位为分，需要统一后比较
	// 回调 amount 单位是 **分**（int64）✓，与 order.Amount 同单位 → 直接比较 ✓。
	// 原来按「元」换算（input.Amount*100）✗ → 9.90 元的套餐永远对不上 ✗。
	callbackAmountCents := input.Amount
	if callbackAmountCents != order.Amount {
		log.Printf("[callback] request_id=%s amount_mismatch callback=%d(yuan) order=%d(cents)", requestID, input.Amount, order.Amount)
		recordEvent("amount_error", "AMOUNT_MISMATCH", fmt.Sprintf("callback=%d(yuan) order=%d(cents)", input.Amount, order.Amount), signValid, usedSecretVer, ipMatchRule)
		raiseSecEvent(entity.ChargeSecEventAmountMismatch, entity.ChargeSecSeverityCritical, input.TradeNo,
			fmt.Sprintf("amount mismatch: callback=%d(yuan) order=%d(cents)", input.Amount, order.Amount))
		return fmt.Errorf("amount mismatch: callback=%d(yuan) order=%d(cents)", input.Amount, order.Amount)
	}
	// v3.2: 币种锁定 — order 有币种时，callback.currency 必须非空且一致
	if order.Currency != "" {
		if input.Currency == "" {
			log.Printf("[callback] request_id=%s currency_missing order=%s", requestID, order.Currency)
			recordEvent("amount_error", "CURRENCY_MISSING", fmt.Sprintf("order=%s callback=empty", order.Currency), signValid, usedSecretVer, ipMatchRule)
			raiseSecEvent(entity.ChargeSecEventCurrencyMismatch, entity.ChargeSecSeverityCritical, input.TradeNo,
				fmt.Sprintf("currency missing: order=%s callback=empty", order.Currency))
			return fmt.Errorf("currency missing: order currency is %s but callback has none", order.Currency)
		}
		if input.Currency != order.Currency {
			log.Printf("[callback] request_id=%s currency_mismatch callback=%s order=%s", requestID, input.Currency, order.Currency)
			recordEvent("amount_error", "CURRENCY_MISMATCH", fmt.Sprintf("callback=%s order=%s", input.Currency, order.Currency), signValid, usedSecretVer, ipMatchRule)
			raiseSecEvent(entity.ChargeSecEventCurrencyMismatch, entity.ChargeSecSeverityCritical, input.TradeNo,
				fmt.Sprintf("currency mismatch: callback=%s order=%s", input.Currency, order.Currency))
			return fmt.Errorf("currency mismatch: callback=%s order=%s", input.Currency, order.Currency)
		}
	}
	// v3.2: 内部余额只认 order.amount，paid_amount/exchange_rate 不参与入账

	// ⑫ process payment result
	if input.Status == entity.CallbackStatusPaid {
		paidAt := time.Now()
		if input.PaidAt != "" {
			if t, err := time.Parse(time.RFC3339, input.PaidAt); err == nil {
				paidAt = t
			}
		}

		// v3.2 Transaction: 锁订单 → 锁余额 → 写流水 → 改余额 → 改订单 → COMMIT
		// 不再先改 processing 再锁，整个流程在一个事务内用 SELECT FOR UPDATE 保证原子性
		// SQLite：WAL 下"读→写升级"会立刻 SQLITE_BUSY（busy_timeout 不生效），
		// 有并发写者时就会瞬时失败 → 做有限重试（失败的事务未提交，且入账本身幂等）。
		err := retryOnDBLocked(func() error {
			return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
				// Step 1: SELECT order FOR UPDATE (行锁)
				var lockedOrder entity.ChargeOrder
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
					Where("trade_no = ?", input.TradeNo).First(&lockedOrder).Error; err != nil {
					return err
				}

				// 幂等检查：如果已经是 paid，直接返回
				if lockedOrder.Status == entity.ChargeStatusPaid {
					return nil
				}
				// 如果不是 pending/processing，不能继续
				// 放行 pending / processing / **expired**：expired 只代表"不再接受新付款"，
				// 而钱要是已经收了就必须入账（正常交易逻辑：不能让用户付款了还失败）。
				if lockedOrder.Status != entity.ChargeStatusPending &&
					lockedOrder.Status != entity.ChargeStatusProcessing &&
					lockedOrder.Status != entity.ChargeStatusExpired {
				}

				// Step 2: 余额行锁 —— 只有"充值单"需要（套餐单不动余额，而且客户端可能还没有余额行）
				if lockedOrder.OrderType == entity.ChargeOrderTypeCharge {
					var bal entity.ClientBalance
					if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
						Where("client_id = ?", lockedOrder.ClientID).First(&bal).Error; err != nil {
						return err
					}
				}

				// Step 3~4: 发货 —— 直接复用**权威实现** processChargePayment（tx 版）。
				//
				// ⚠ 这里原来写死成"给余额加钱"：对套餐订单（order_type=purchase）也会把钱充成余额，
				// 却**不激活套餐**、套餐订单永远挂在 pending —— 用户花钱买套餐结果只拿到余额，
				// 这是真机端到端测试抓到的 P0 金额问题。
				// 充值卡那条路（processChargePayment）本来就分两支：充值单 → 加余额；
				// 套餐单 → activatePlanTx + 写 plan_order。现在两条路共用同一份实现，
				// 不再各自维护一套口径（历史上正是"两处口径不一致"埋了多个 P0）。
				if err := s.processChargePayment(ctx, tx, &lockedOrder); err != nil {
					return err
				}

				// Step 5: UPDATE order → paid (processing_at 清空)
				if err := tx.Model(&entity.ChargeOrder{}).
					Where("id = ?", lockedOrder.ID).
					Updates(map[string]interface{}{
						"status":        entity.ChargeStatusPaid,
						"paid_at":       paidAt,
						"paid_currency": input.PaidCurrency,
						"paid_amount":   input.PaidAmount,
						"exchange_rate": input.ExchangeRate,
						"method":        input.Method,
						"processing_at": nil,
					}).Error; err != nil {
					return err
				}

				return nil
			})
		})
		if err != nil {
			log.Printf("[callback] request_id=%s order=%s transaction_error=%v", requestID, input.TradeNo, err)
			recordEvent("error", "CALLBACK_FAILED", err.Error(), signValid, usedSecretVer, ipMatchRule)
			s.enqueueCallbackRetry(ctx, siteID, requestID, input.TradeNo, remoteIP, payloadHash, "CALLBACK_FAILED", err.Error(), true)
			raiseSecEvent(entity.ChargeSecEventTransactionError, entity.ChargeSecSeverityHigh, input.TradeNo,
				fmt.Sprintf("callback transaction failed: %v", err))
			return err
		}
		log.Printf("[callback] request_id=%s order=%s amount=%d status=paid success", requestID, input.TradeNo, order.Amount)
		recordEvent("success", "", "", signValid, usedSecretVer, ipMatchRule)

		// P1: 首次付费返利触发（三方支付购买套餐 — 事务外调用，避免嵌套事务问题）
		// 仅当是套餐购买订单时触发，纯充值不触发返利
		if order.OrderType == entity.ChargeOrderTypePurchase && order.PlanVariantID != nil {
			s.triggerFirstPaid(ctx, order.ClientID, order.Amount)
		}

		return nil
	}

	// payment failed
	s.db.WithContext(ctx).Model(&entity.ChargeOrder{}).
		Where("trade_no = ? AND status IN ?", input.TradeNo, []string{entity.ChargeStatusPending, entity.ChargeStatusProcessing}).
		Update("status", entity.ChargeStatusFailed)
	log.Printf("[callback] request_id=%s order=%s status=failed", requestID, input.TradeNo)
	recordEvent("success", "", "order marked as failed", signValid, usedSecretVer, ipMatchRule)
	return nil
}

// retryOnDBLocked 对数据库**瞬时写锁**做有限重试（目前主要针对 SQLite）。
//
// 为什么需要：SQLite 在 WAL 模式下，"先读后写"的事务（本项目的入账事务就是先
// SELECT ... FOR UPDATE 再 INSERT/UPDATE）在**升级为写事务**时，如果此刻已有别的写者
// （面板自身的定时任务、外部进程），会**立刻**返回 SQLITE_BUSY —— 注意这种情况下
// `busy_timeout` **不生效**（它只对"一开始就申请写锁"的语句有效）。
// 重试是安全的：失败的事务没有提交，且入账逻辑本身幂等（订单已 paid 直接返回、
// balance_transactions 对 (related_charge_order_id,type) 有唯一约束）。
// 最多重试 3 次（退避 100/200/300ms），避免把回调拖太久。
func retryOnDBLocked(fn func() error) error {
	const maxAttempts = 4
	var err error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if err = fn(); err == nil {
			return nil
		}
		if !isDBLockedError(err) || attempt == maxAttempts-1 {
			return err
		}
		time.Sleep(time.Duration(100*(attempt+1)) * time.Millisecond)
	}
	return err
}

// isDBLockedError 判断是否为瞬时写锁错误（SQLite 的 SQLITE_BUSY / SQLITE_LOCKED）。
func isDBLockedError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "database is locked") ||
		strings.Contains(msg, "sqlite_busy") ||
		strings.Contains(msg, "database table is locked")
}

// recordCallbackEventAsync records a callback event asynchronously (v3.2: 事务外，不阻塞账务).
func (s *CommerceService) recordCallbackEventAsync(siteID int64, requestID, tradeNo, remoteIP, ipMatchRule, payloadHash string, amount int64, result, errorCode, errMsg string, signValid bool, secretVer int) {
	event := &entity.ChargeCallbackEvent{
		SiteID:       siteID,
		TradeNo:      tradeNo,
		RequestID:    requestID,
		RemoteIP:     remoteIP,
		IpMatchRule:  ipMatchRule,
		PayloadHash:  payloadHash,
		SignValid:    signValid,
		SecretVer:    secretVer,
		Result:       result,
		ErrorCode:    errorCode,
		ErrorMessage: errMsg,
		Amount:       amount,
	}
	if err := s.db.Create(event).Error; err != nil {
		log.Printf("[callback] request_id=%s failed to record event async: %v", requestID, err)
	}
}

// recordSecurityEventAsync records a charge security event asynchronously (v3.2).
func (s *CommerceService) recordSecurityEventAsync(siteID int64, tradeNo, requestID, eventType, severity, remoteIP, detail string) {
	event := &entity.ChargeSecurityEvent{
		SiteID:    siteID,
		TradeNo:   tradeNo,
		RequestID: requestID,
		EventType: eventType,
		Severity:  severity,
		RemoteIP:  remoteIP,
		Detail:    detail,
	}
	if err := s.db.Create(event).Error; err != nil {
		log.Printf("[callback] request_id=%s failed to record security event: %v", requestID, err)
	}
}

// chargeSecretEntry holds a decrypted secret with its version number.
type chargeSecretEntry struct {
	version int
	secret  string
}

// loadActiveSecrets loads all active and legacy secrets from charge_site_secrets table (v3.2).
// Returns secrets sorted by version descending (newest first).
func (s *CommerceService) loadActiveSecrets(ctx context.Context, siteID int64) ([]chargeSecretEntry, error) {
	var secrets []entity.ChargeSiteSecret
	if err := s.db.WithContext(ctx).
		Where("site_id = ? AND status IN ?", siteID, []string{entity.SecretStatusActive, entity.SecretStatusLegacy}).
		Order("version DESC").
		Find(&secrets).Error; err != nil {
		return nil, err
	}
	var result []chargeSecretEntry
	for _, sec := range secrets {
		if decrypted, err := crypto.DecryptSecret(sec.EncryptedSecret); err == nil {
			result = append(result, chargeSecretEntry{version: sec.Version, secret: decrypted})
		}
	}
	return result, nil
}

// callbackBodyHashCandidates 返回回调 body_hash 的两个可接受取值 ✓。
//
// 背景（真机排查确认的契约 bug ✗）：官方对接文档 CHARGE_SITE_ACCESS_GUIDE.md 第 1 步要求
//
//	body_hash = SHA256(原始请求体字节)   ← 此刻 body 里**还没有** sign
//	sign 是第 4 步才 `body["sign"] = sign` 追加进去的
//
// 而 sign 的签名载荷里**包含** body_hash → "先有 sign 才能算 hash、先有 hash 才能算 sign" ✗
// 是循环的 ✓ → 只能按文档"先算 body_hash、后放 sign"实现 ✓。
// 旧实现却对**收到的完整 body（含 sign）**取哈希 ✗ → 与文档不符 →
// 任何规范实现的充值站点都验签失败 ✗（现象：回调恒 400 INVALID_SIGNATURE ✓）。
//
// 现在返回两个候选，任一匹配即通过 ✓：
//
//	stripped：把顶层 sign 成员从原始字节中剔除后的 sha256 ✓（符合文档 ✓）
//	full    ：完整原始字节的 sha256 ✓（兼容旧行为 ✓）
//
// 安全性不受影响 ✓：伪造签名仍需回调密钥 ✓；body_hash 只约束整体完整性 ✓
// （status / paid_* 等字段本来就不参与签名载荷 ✓）。
func callbackBodyHashCandidates(raw []byte) (stripped, full string) {
	full = sha256Hex(raw)
	stripped = full
	if withoutSign, ok := stripTopLevelSignMember(raw); ok {
		stripped = sha256Hex(withoutSign)
	}
	return stripped, full
}

// stripTopLevelSignMember 从原始 JSON 对象里剔除顶层 "sign" 成员（连同分隔逗号 ✓），
// 使 sha256(结果) 与文档里"加 sign 之前的 body 字节"完全一致 ✓。
//
// 用 json.Decoder 的 InputOffset() 精确定位字节区间 ✓（encoding/json 官方 API ✓），
// 因此不依赖 key 顺序与值类型 ✓（客户端按任意顺序写都能正确还原 ✓）。
// 返回 ok=false 表示 body 不是 JSON 对象或没有 sign 成员 ✓。
func stripTopLevelSignMember(raw []byte) ([]byte, bool) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return nil, false
	}
	for dec.More() {
		keyStart := dec.InputOffset() // key 之前的位置（前一个 token 之后 ✓，可能含空白 ✓）
		kt, err := dec.Token()
		if err != nil {
			return nil, false
		}
		key, _ := kt.(string)
		if _, err := dec.Token(); err != nil { // 跳过值 ✓
			return nil, false
		}
		valEnd := dec.InputOffset()
		if key != "sign" {
			continue
		}
		// 起始：优先吃掉"前置逗号"（正常中间成员 ✓）；若 sign 是第一个成员 ✓
		// 则保留 `{`、从 key 前开始删 ✓ 并改为吃掉"后置逗号" ✓ ——
		// ⚠ 两个逗号只能吃一个 ✗，否则会把前后两个成员粘在一起 ✗。
		removeStart := keyStart
		var removeEnd = valEnd
		found := false
		// ⚠ 必须从 keyStart **本身**开始往回找 ✗：
		// json.Decoder.InputOffset() 返回的是"上一个 token **结束之后**"的偏移 ✓，
		// 也就是**正好落在分隔逗号上** ✓ —— 从 keyStart-1 开始就会多退一位 ✗，
		// 把前一个成员一起圈进删除区间 ✗（真机表现：body_hash 永远对不上 → 回调恒 400 ✗，
		// 这个坑是单测 TestStripTopLevelSignMember 抓出来的 ✓）。
		for i := int(keyStart); i >= 0; i-- {
			if raw[i] == ',' {
				removeStart = int64(i)
				found = true
				break
			}
			if raw[i] == '{' {
				break
			}
		}
		if !found {
			// sign 是第一个成员 ✓ → 吃掉它后面的逗号 ✓
			for i := int(valEnd); i < len(raw); i++ {
				if raw[i] == ',' {
					removeEnd = int64(i) + 1
					break
				}
				if raw[i] == '}' || raw[i] == ']' {
					break
				}
			}
		}
		out := make([]byte, 0, len(raw))
		out = append(out, raw[:removeStart]...)
		out = append(out, raw[removeEnd:]...)
		return out, true
	}
	return nil, false
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

// RotateSiteSecret rotates the callback secret using the v3.2 charge_site_secrets table.
// v3.2: 当前 active 密钥变为 legacy，新密钥插入为 active。
// 支持 N 个历史版本，类似 Stripe/PayPal webhook secret rotation。
func (s *CommerceService) RotateSiteSecret(ctx context.Context, siteID int64, newSecret string) error {
	site, err := s.chargeSiteRepo.GetByID(ctx, siteID)
	if err != nil {
		return fmt.Errorf("charge site not found")
	}
	if newSecret == "" {
		return fmt.Errorf("new secret cannot be empty")
	}

	// 加密新密钥
	encrypted, err := crypto.EncryptSecret(newSecret)
	if err != nil {
		return fmt.Errorf("failed to encrypt new secret: %w", err)
	}

	newVersion := site.SecretVersion + 1

	// v3.2: 在 charge_site_secrets 表中操作
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 将当前 active 密钥降级为 legacy
		tx.Model(&entity.ChargeSiteSecret{}).
			Where("site_id = ? AND status = ?", siteID, entity.SecretStatusActive).
			Update("status", entity.SecretStatusLegacy)

		// 插入新密钥为 active
		newSecretRecord := &entity.ChargeSiteSecret{
			SiteID:          siteID,
			Version:         newVersion,
			EncryptedSecret: encrypted,
			Status:          entity.SecretStatusActive,
			CreatedAt:       time.Now(),
		}
		if err := tx.Create(newSecretRecord).Error; err != nil {
			return err
		}

		// 更新 ChargeSite 的 SecretVersion（兼容旧字段）
		site.SecretVersion = newVersion
		site.EncryptedSecret = encrypted
		site.CallbackSecret = ""
		return tx.Model(&entity.ChargeSite{}).Where("id = ?", siteID).
			Updates(map[string]interface{}{
				"secret_version":   newVersion,
				"encrypted_secret": encrypted,
				"callback_secret":  "",
			}).Error
	})
}

// ClearPrevSecret expires old legacy secrets after the transition period is over.
// v3.2: 将 legacy 密钥标记为 expired，不再用于验签。
func (s *CommerceService) ClearPrevSecret(ctx context.Context, siteID int64) error {
	now := time.Now()
	return s.db.WithContext(ctx).Model(&entity.ChargeSiteSecret{}).
		Where("site_id = ? AND status = ?", siteID, entity.SecretStatusLegacy).
		Updates(map[string]interface{}{
			"status":     entity.SecretStatusExpired,
			"expired_at": now,
		}).Error
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
	return findIPMatchRule(remoteIP, whitelistJSON) != ""
}

// findIPMatchRule returns the matching whitelist rule (IP or CIDR) for remoteIP, or "" if no match.
func findIPMatchRule(remoteIP string, whitelistJSON string) string {
	var whitelist []string
	if err := json.Unmarshal([]byte(whitelistJSON), &whitelist); err != nil {
		return ""
	}
	testIP := net.ParseIP(remoteIP)
	if testIP == nil {
		return ""
	}
	for _, entry := range whitelist {
		if strings.Contains(entry, "/") {
			_, cidr, err := net.ParseCIDR(entry)
			if err == nil && cidr.Contains(testIP) {
				return entry
			}
		} else {
			if entry == remoteIP {
				return entry
			}
		}
	}
	return ""
}

// StartProcessingRecovery starts a background goroutine that recovers stuck processing orders.
// v3.2: Also runs nonce cleanup (24h retention) and expired secret cleanup.
func (s *CommerceService) StartProcessingRecovery(interval time.Duration) chan struct{} {
	stopCh := make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				s.recoverStuckProcessing()
				s.cleanupNonces()
				s.cleanupExpiredSecrets()
			}
		}
	}()
	return stopCh
}

// cleanupNonces removes nonce records older than 24 hours (v3.2: 支付争议审计保留24小时).
func (s *CommerceService) cleanupNonces() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	threshold := time.Now().Add(-24 * time.Hour)
	result := s.db.WithContext(ctx).Where("created_at < ?", threshold).Delete(&entity.CallbackNonce{})
	if result.RowsAffected > 0 {
		log.Printf("[cleanup] deleted %d nonces older than 24h", result.RowsAffected)
	}
}

// cleanupExpiredSecrets removes expired secret records older than 7 days (v3.2).
func (s *CommerceService) cleanupExpiredSecrets() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	threshold := time.Now().Add(-7 * 24 * time.Hour)
	result := s.db.WithContext(ctx).
		Where("status = ? AND expired_at < ?", entity.SecretStatusExpired, threshold).
		Delete(&entity.ChargeSiteSecret{})
	if result.RowsAffected > 0 {
		log.Printf("[cleanup] deleted %d expired secrets older than 7d", result.RowsAffected)
	}
}

// recoverStuckProcessing recovers orders stuck in processing for more than 5 minutes.
// v3.2: 检查 balance_transactions，有流水→paid（防重复入账），无流水→pending（可重新处理）
func (s *CommerceService) recoverStuckProcessing() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	threshold := time.Now().Add(-5 * time.Minute)

	// 查找超时的 processing 订单
	var stuckOrders []entity.ChargeOrder
	if err := s.db.WithContext(ctx).
		Where("status = ? AND processing_at < ?", entity.ChargeStatusProcessing, threshold).
		Find(&stuckOrders).Error; err != nil {
		log.Printf("[recovery] failed to query stuck orders: %v", err)
		return
	}

	for _, order := range stuckOrders {
		// 检查是否已有 balance_transactions 记录
		var txCount int64
		s.db.WithContext(ctx).Model(&entity.BalanceTransaction{}).
			Where("related_charge_order_id = ?", order.ID).Count(&txCount)

		if txCount > 0 {
			// 有流水 → 说明入账已成功，但订单状态未更新为 paid
			// 直接标记为 paid，防止重复入账
			now := time.Now()
			s.db.WithContext(ctx).Model(&entity.ChargeOrder{}).
				Where("id = ? AND status = ?", order.ID, entity.ChargeStatusProcessing).
				Updates(map[string]interface{}{
					"status":        entity.ChargeStatusPaid,
					"paid_at":       now,
					"processing_at": nil,
				})
			log.Printf("[recovery] order %s: has ledger → paid (prevented double charge)", order.TradeNo)
			// 记录安全事件
			siteIDForEvent := int64(0)
			if order.SiteID != nil {
				siteIDForEvent = *order.SiteID
			}
			go s.recordSecurityEventAsync(siteIDForEvent, order.TradeNo, "", entity.ChargeSecEventRecoveryStuck, entity.ChargeSecSeverityHigh, "",
				fmt.Sprintf("order %s recovered to paid (had ledger, was stuck in processing)", order.TradeNo))
		} else {
			// 无流水 → 说明入账未完成，安全回滚到 pending
			s.db.WithContext(ctx).Model(&entity.ChargeOrder{}).
				Where("id = ? AND status = ?", order.ID, entity.ChargeStatusProcessing).
				Updates(map[string]interface{}{
					"status":        entity.ChargeStatusPending,
					"processing_at": nil,
				})
			log.Printf("[recovery] order %s: no ledger → pending (safe to reprocess)", order.TradeNo)
		}
	}
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
	// 1. 预检查（无锁，快速失败）
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

	tradeNo := generateTradeNo()
	now := time.Now()
	cardID := card.ID

	chargeOrder := &entity.ChargeOrder{
		ClientID: clientID, TradeNo: tradeNo, Source: entity.ChargeSourceCard,
		Amount: card.Value, Currency: card.Currency, ChargeCardID: &cardID,
		OrderType: entity.ChargeOrderTypeCharge, Status: entity.ChargeStatusPaid, PaidAt: &now,
	}
	if card.CardType == entity.ChargeCardTypePlan {
		chargeOrder.OrderType = entity.ChargeOrderTypePurchase
		chargeOrder.PlanVariantID = &card.Value
		chargeOrder.Amount = 0
	}

	// 2. 事务：锁卡 → 标记used → 创建订单 → 处理充值（全在一个事务内）
	//    任一步骤失败，整个事务回滚，卡恢复 active，防止"卡消耗但充值未完成"
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 行锁卡
		var lockedCard entity.ChargeCard
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", card.ID).First(&lockedCard).Error; err != nil {
			return err
		}
		if lockedCard.Status != entity.ChargeCardStatusActive {
			return fmt.Errorf("card has already been used")
		}
		// 立即标记为 used（WHERE status=active 防止并发竞争）
		result := tx.Model(&entity.ChargeCard{}).
			Where("id = ? AND status = ?", lockedCard.ID, entity.ChargeCardStatusActive).
			Updates(map[string]interface{}{
				"status":     entity.ChargeCardStatusUsed,
				"used_count": 1,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("card has already been used")
		}
		// 创建订单（用 tx 保证在事务内）
		if err := tx.Create(chargeOrder).Error; err != nil {
			return err
		}
		// 处理充值（传入 tx，使用 savepoint 嵌套事务）
		if err := s.processChargePayment(ctx, tx, chargeOrder); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to redeem card: %v", err)
	}
	// P1: 首次付费返利触发（充值卡购买套餐 — 事务外调用，避免嵌套事务问题）
	if chargeOrder.OrderType == entity.ChargeOrderTypePurchase && chargeOrder.PlanVariantID != nil {
		s.triggerFirstPaid(ctx, chargeOrder.ClientID, chargeOrder.Amount)
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
					// 清理失败不会让面板崩溃，但**必须留下日志** ✗ 原来写的是 `_ = err`：
					// 注释说 "Log but don't crash"，实际既没日志、也把错误丢了 ——
					// 清理不成功时，超时未支付的订单会一直挂在"待支付"，日志里却一条痕迹都没有。
					log.Printf("[charge-cleanup] cleanup expired orders failed: %v", err)
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
// v3.3: 接受 exec 参数 — 传入 s.db 时创建独立事务；传入外层 tx 时使用 savepoint 嵌套事务。
// 这样 RedeemCard 可以把"锁卡→标记used→创建订单→充值"放在一个事务内，任一步失败全部回滚。
func (s *CommerceService) processChargePayment(ctx context.Context, exec *gorm.DB, order *entity.ChargeOrder) error {
	return exec.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if order.OrderType == entity.ChargeOrderTypeCharge && order.Amount > 0 {
			// SELECT balance FOR UPDATE（行锁，防并发）
			var bal entity.ClientBalance
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("client_id = ?", order.ClientID).First(&bal).Error; err != nil {
				if err == gorm.ErrRecordNotFound {
					// 创建零余额记录
					bal = entity.ClientBalance{ClientID: order.ClientID, Balance: 0, Currency: order.Currency}
					if err := tx.Create(&bal).Error; err != nil {
						return err
					}
				} else {
					return err
				}
			}

			balanceBefore := bal.Balance
			balanceAfter := balanceBefore + order.Amount

			// UPDATE balance
			if err := tx.Model(&entity.ClientBalance{}).
				Where("client_id = ?", order.ClientID).
				Update("balance", balanceAfter).Error; err != nil {
				return err
			}

			remark := "充值到账"
			if order.Source == entity.ChargeSourceCard {
				remark = "充值卡兑换到账"
			}
			// INSERT ledger（先记账：balance_before + balance_after）
			txRecord := &entity.BalanceTransaction{
				ClientID: order.ClientID, Type: entity.BalanceTxTypeCharge,
				Amount: order.Amount, BalanceBefore: balanceBefore, BalanceAfter: balanceAfter,
				Currency: bal.Currency, RelatedChargeOrderID: &order.ID, Remark: remark,
			}
			return tx.Create(txRecord).Error
		} else if order.OrderType == entity.ChargeOrderTypePurchase && order.PlanVariantID != nil {
			variant, err := s.variantRepo.GetByID(ctx, *order.PlanVariantID)
			if err != nil {
				return fmt.Errorf("variant not found")
			}
			tmpl, err := s.tmplRepo.GetByID(ctx, variant.TemplateID)
			if err != nil {
				return fmt.Errorf("template not found")
			}
			// S-4 fix: load client using tx so subsequent writes are in the transaction
			var client entity.Client
			if err := tx.Where("id = ?", order.ClientID).First(&client).Error; err != nil {
				return err
			}

			// Traffic pack: add traffic (and optionally extend expiry)
			if variant.VariantType == entity.VariantTypeTrafficPack {
				if err := s.applyTrafficPackTx(ctx, tx, &client, variant); err != nil {
					return err
				}
			} else {
				// Regular plan activation — use tx-aware version
				orderType := entity.OrderTypeNew
				if client.PlanID > 0 && client.Status != "expired" {
					orderType = entity.OrderTypeRenew
				}
				if err := s.activatePlanTx(ctx, tx, &client, tmpl, variant, orderType); err != nil {
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
			if err := tx.Create(planOrder).Error; err != nil {
				return err
			}
			// 把"下单时创建的那条 pending 套餐订单"一起收尾。
			// 否则用户在「我的订单」里会一直看到"待支付"，而钱其实早就付了。
			// 作用域限定 同一客户 + 同一目标变体，避免误伤其它未支付的订单。
			finish := tx.Model(&entity.PlanOrder{}).
				Where("client_id = ? AND status = ?", order.ClientID, entity.OrderStatusPending)
			if order.PlanVariantID != nil {
				finish = finish.Where("to_variant_id = ?", *order.PlanVariantID)
			}
			if err := finish.Updates(map[string]interface{}{
				"status":      entity.OrderStatusCompleted,
				"paid_amount": order.Amount,
			}).Error; err != nil {
				return err
			}
			// 记录余额消费流水（三方充值/充值卡购买套餐）
			if order.Amount > 0 && order.Source != entity.ChargeSourceCard {
				remark := "在线支付购买套餐"
				if order.Source == entity.ChargeSourceSite {
					remark = "充值站点支付购买套餐"
				}
				// S-10 fix: record actual balance before/after for audit trail
				var balAfter entity.ClientBalance
				tx.Where("client_id = ?", order.ClientID).First(&balAfter)
				txRecord := &entity.BalanceTransaction{
					ClientID: order.ClientID, Type: entity.BalanceTxTypeConsume,
					Amount: -order.Amount, BalanceBefore: balAfter.Balance + order.Amount, BalanceAfter: balAfter.Balance,
					RelatedChargeOrderID: &order.ID, RelatedPlanOrderID: &planOrder.ID,
					Remark: remark,
				}
				tx.Create(txRecord)
			}
			return nil
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
		// Trial plans expire after trial_duration_hours instead of the variant duration.
		if tmpl.IsTrial && tmpl.TrialDurationHours > 0 {
			expiresAt = now.Add(time.Duration(tmpl.TrialDurationHours) * time.Hour)
		}
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

	// 限速三字段与设备数不物化模板值（含换到"不限速"模板的情况）：
	// 0=订阅拉取时实时继承模板，非 0=个人覆盖（push 回退见 clientauth/handler.go）。

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

	// 套餐激活成功后，为客户端在所有入站创建/确保账号。复用 plan.Service.SyncAccountsForTemplate
	// 已有链路（SyncAccountsForNewInbound → InboundManager.AddUserToInbound）：
	// 生成 UUID/Password、写入 config_json 持久化、通过 xray gRPC 热添加到运行中的 xray（本地+远程）。
	// 新购和续费都触发：SyncAccountsForNewInbound 内部有 userExistsInConfig 幂等检查，已存在的账号会跳过。
	// 续费时也触发是为了覆盖过期后重新购买的场景（退款可能已从 xray 运行时移除用户）。
	if (orderType == entity.OrderTypeNew || orderType == entity.OrderTypeRenew) && s.accountSyncer != nil {
		go func(planID int64) {
			if _, err := s.accountSyncer.SyncAccountsForTemplate(context.Background(), planID); err != nil {
				log.Printf("[commerce] failed to sync accounts for plan %d: %v", planID, err)
			}
		}(tmpl.ID)
	}

	return nil
}

// activatePlanTx is a tx-aware version of activatePlan that uses the provided
// gorm.DB executor (which may be a transaction) instead of the service's repos.
// This ensures plan activation writes are included in the caller's transaction
// and roll back on failure. (S-4 fix)
func (s *CommerceService) activatePlanTx(ctx context.Context, tx *gorm.DB, client *entity.Client, tmpl *entity.PlanTemplate, variant *entity.PlanVariant, orderType string) error {
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
		if tmpl.IsTrial && tmpl.TrialDurationHours > 0 {
			expiresAt = now.Add(time.Duration(tmpl.TrialDurationHours) * time.Hour)
		}
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

	// 限速三字段与设备数不物化模板值（含换到"不限速"模板的情况）：
	// 0=订阅拉取时实时继承模板，非 0=个人覆盖（push 回退见 clientauth/handler.go）。

	if err := tx.Save(client).Error; err != nil {
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
		if err := tx.Create(sub).Error; err != nil {
			return err
		}
		client.SubscriptionID = &sub.ID
		tx.Save(client)
	} else {
		var sub entity.Subscription
		if err := tx.Where("id = ?", *client.SubscriptionID).First(&sub).Error; err != nil {
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
		tx.Save(&sub)
	}

	// Account sync is async and side-effectful — do it outside the tx path.
	if (orderType == entity.OrderTypeNew || orderType == entity.OrderTypeRenew) && s.accountSyncer != nil {
		go func(planID int64) {
			if _, err := s.accountSyncer.SyncAccountsForTemplate(context.Background(), planID); err != nil {
				log.Printf("[commerce] failed to sync accounts for plan %d: %v", planID, err)
			}
		}(tmpl.ID)
	}

	return nil
}

// applyTrafficPackTx is a tx-aware version of applyTrafficPack. (S-4 fix)
func (s *CommerceService) applyTrafficPackTx(ctx context.Context, tx *gorm.DB, client *entity.Client, variant *entity.PlanVariant) error {
	now := time.Now()

	// 1. Add traffic bytes to subscription (cumulative)
	if client.SubscriptionID != nil {
		if err := tx.Model(&entity.Subscription{}).
			Where("id = ?", *client.SubscriptionID).
			UpdateColumn("quota_bytes", gorm.Expr("quota_bytes + ?", variant.QuotaBytes)).Error; err != nil {
			return err
		}
	}

	// 2. Add to client.QuotaBytes (cumulative)
	client.QuotaBytes += variant.QuotaBytes
	client.UpdatedAt = now

	// 3. Extend plan expiry if mode is extend_plan
	if variant.ExpiryMode == entity.ExpiryModeExtendPlan && variant.DurationDays > 0 {
		var newExpiry time.Time
		if client.ExpiresAt != nil && client.ExpiresAt.After(now) {
			newExpiry = client.ExpiresAt.AddDate(0, 0, variant.DurationDays)
		} else {
			newExpiry = now.AddDate(0, 0, variant.DurationDays)
		}
		client.ExpiresAt = &newExpiry

		if client.SubscriptionID != nil {
			tx.Model(&entity.Subscription{}).
				Where("id = ?", *client.SubscriptionID).
				UpdateColumn("expires_at", newExpiry)
		}
	}

	if err := tx.Save(client).Error; err != nil {
		return fmt.Errorf("failed to update client after traffic pack: %v", err)
	}

	// Async sync — outside tx
	if s.userSyncer != nil {
		go s.userSyncer.SyncUserToServers(context.Background(), client.ID, "update")
	}

	return nil
}

// RefundChargeOrder 退款一个已支付的 ChargeOrder。
// 如果该订单是套餐购买（order_type=purchase），则：
//  1. 回退客户端到期时间（减去对应 variant 的 duration_days）
//  2. 如果回退后已过期，标记 client 为 expired
//  3. 如果是新购退款（client 无其他有效套餐），从所有节点移除用户凭证
//  4. 退还金额到余额（如果 amount > 0）
//
// remark 为退款备注，如为空则使用 "管理员退款"。
func (s *CommerceService) RefundChargeOrder(ctx context.Context, chargeOrderID int64, remark string) error {
	order, err := s.chargeOrderRepo.GetByID(ctx, chargeOrderID)
	if err != nil {
		return fmt.Errorf("charge order not found: %w", err)
	}
	if order.Status != entity.ChargeStatusPaid {
		return fmt.Errorf("only paid orders can be refunded, current status: %s", order.Status)
	}
	if remark == "" {
		remark = "管理员退款"
	}

	// 1. 更新订单状态为 refunded
	order.Status = entity.ChargeStatusRefunded
	order.Remark = remark
	if err := s.chargeOrderRepo.Update(ctx, order); err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}

	// 2. 退还金额到余额
	if order.Amount > 0 {
		bal, err := s.balanceRepo.AddBalance(ctx, order.ClientID, order.Amount)
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
			s.balanceTxRepo.Create(ctx, txRecord)
		}
	}

	// 3. 如果是套餐购买订单，回退套餐时间
	if order.OrderType == entity.ChargeOrderTypePurchase && order.PlanVariantID != nil {
		client, err := s.clientRepo.GetByID(ctx, order.ClientID)
		if err != nil {
			log.Printf("[refund] failed to get client %d: %v", order.ClientID, err)
			return nil // 退款本身已成功，套餐回退失败仅记录
		}

		variant, err := s.variantRepo.GetByID(ctx, *order.PlanVariantID)
		if err != nil {
			log.Printf("[refund] failed to get variant %d: %v", *order.PlanVariantID, err)
			return nil
		}

		// 回退到期时间：减去 variant.DurationDays
		if client.ExpiresAt != nil && variant.DurationDays > 0 {
			newExpiresAt := client.ExpiresAt.AddDate(0, 0, -variant.DurationDays)
			client.ExpiresAt = &newExpiresAt

			// 如果回退后已过期，标记为 expired
			if newExpiresAt.Before(time.Now()) || newExpiresAt.Equal(time.Now()) {
				client.Status = "expired"
				// 从所有节点移除用户凭证
				if s.userSyncer != nil {
					go func(cID int64) {
						if err := s.userSyncer.SyncUserToServers(context.Background(), cID, "remove"); err != nil {
							log.Printf("[refund] failed to remove user %d from servers: %v", cID, err)
						}
					}(client.ID)
				}
			}

			if err := s.clientRepo.Update(ctx, client); err != nil {
				log.Printf("[refund] failed to update client %d expires_at: %v", client.ID, err)
			} else {
				log.Printf("[refund] client %d expires_at rolled back by %d days (order %d)",
					client.ID, variant.DurationDays, chargeOrderID)
			}

			// 同步更新 subscription
			if client.SubscriptionID != nil {
				if sub, err := s.subRepo.GetByID(ctx, *client.SubscriptionID); err == nil {
					sub.ExpiresAt = client.ExpiresAt
					if client.Status == "expired" {
						sub.Status = "expired"
					}
					s.subRepo.Update(ctx, sub)
				}
			}
		}
	}

	return nil
}

// deductBalanceForOrder deducts balance for a plan order and records the transaction.
func (s *CommerceService) deductBalanceForOrder(ctx context.Context, clientID int64, amount int64, planOrderID *int64) error {
	newBal, err := s.balanceRepo.DeductBalance(ctx, clientID, amount)
	if err != nil {
		return fmt.Errorf("insufficient balance")
	}
	txRecord := &entity.BalanceTransaction{
		ClientID: clientID, Type: entity.BalanceTxTypeConsume,
		Amount: -amount, BalanceBefore: newBal.Balance + amount, BalanceAfter: newBal.Balance,
		RelatedPlanOrderID: planOrderID, Remark: "余额购买套餐扣款",
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

	// 金额单位统一为 **分**（与官方对接文档 CHARGE_SITE_ACCESS_GUIDE.md 一致）。
	// 原来这里发的是「元」（float）→ 站点回传时也只能给元 ✗，而回调侧按 int64 元解析 ✗ →
	// 像 9.90 元这种套餐价根本没法回传（回 9.9 解析失败 ✗、回 9 金额不符 ✗）→ 套餐充值永远到不了账 ✗。
	// amount = 分（权威字段 ✓）；amount_yuan 仅供收银台展示 ✓。
	payload := map[string]interface{}{
		"trade_no": order.TradeNo, "amount": order.Amount, "amount_yuan": float64(order.Amount) / 100.0, "currency": order.Currency,
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
	// Check per-user usage limit
	if promo.MaxUsesPerUser > 0 && clientID > 0 {
		usedByUser, err := s.orderRepo.CountByClientAndPromotion(ctx, clientID, promo.ID)
		if err == nil && int(usedByUser) >= promo.MaxUsesPerUser {
			return false
		}
	}
	// Check applicability using shared entity method
	switch promo.ApplicableScope {
	case entity.PromoScopeAll:
		// OK
	case entity.PromoScopeTemplate, entity.PromoScopeSpecificVariant:
		if promo.ApplicableIDs != nil {
			variant, err := s.variantRepo.GetByID(ctx, variantID)
			if err != nil || variant == nil {
				return false
			}
			if !promo.IsPromotionApplicable(variant) {
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
// v3.2: 增加 version 字段到签名 payload，支持未来算法升级。
// 签名字段（字典序）：amount, body_hash, currency, nonce, site_trade_no, timestamp, trade_no, version
func (s *CommerceService) verifyChargeCallback(input ChargeCallbackInput, bodyHash string, secret string) bool {
	payload := map[string]interface{}{
		"amount":        input.Amount,
		"body_hash":     bodyHash,
		"currency":      input.Currency,
		"nonce":         input.Nonce,
		"site_trade_no": input.SiteTradeNo,
		"timestamp":     input.Timestamp,
		"trade_no":      input.TradeNo,
		"version":       "v1", // v3.2: 签名版本号，未来升级为 v2/v3 时不破坏旧接口
	}
	payloadBytes, _ := json.Marshal(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payloadBytes)
	got, err := hex.DecodeString(input.Sign)
	if err != nil {
		return false
	}
	return hmac.Equal(got, mac.Sum(nil))
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
