package entity

import "time"

// ============================================================
// Charge Sites — 充值站点配置
// ============================================================

// ChargeSite represents a configured third-party payment site.
type ChargeSite struct {
	ID               int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Name             string    `gorm:"size:100;not null" json:"name"`
	SiteURL          string    `gorm:"size:500;not null" json:"site_url"`
	APIKey           string    `gorm:"size:255" json:"api_key"`
	CallbackSecret   string    `gorm:"size:255" json:"callback_secret"`
	SupportedMethods  string   `gorm:"type:text" json:"supported_methods"` // JSON array, e.g. ["alipay","wechat","usdt"]
	Currency         string    `gorm:"size:16;not null;default:CNY" json:"currency"` // 站点主货币
	IconURL          string    `gorm:"size:500" json:"icon_url"`
	Enabled          bool      `gorm:"not null;default:true" json:"enabled"`
	SortOrder        int       `gorm:"not null;default:0" json:"sort_order"`
	ConfigJSON       string    `gorm:"type:text" json:"config_json"` // extended config
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func (ChargeSite) TableName() string { return "charge_sites" }

// ============================================================
// Charge Orders — 充值订单（统一记录所有充值来源）
// ============================================================

// Charge order source constants.
const (
	ChargeSourceSite    = "site"
	ChargeSourceCard    = "card"
	ChargeSourceBalance = "balance"
	ChargeSourceAdmin   = "admin"
)

// Charge order status constants.
const (
	ChargeStatusPending  = "pending"
	ChargeStatusPaid     = "paid"
	ChargeStatusFailed   = "failed"
	ChargeStatusExpired  = "expired"
	ChargeStatusRefunded = "refunded"
)

// Charge order type constants.
const (
	ChargeOrderTypeCharge   = "charge"   // 纯充值（到余额）
	ChargeOrderTypePurchase = "purchase" // 购买套餐
)

type ChargeOrder struct {
	ID            int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	ClientID      int64      `gorm:"not null;index" json:"client_id"`
	TradeNo       string     `gorm:"size:64;uniqueIndex;not null" json:"trade_no"`
	Source        string     `gorm:"size:32;not null" json:"source"`                // site/card/balance/admin
	SiteID        *int64     `gorm:"index" json:"site_id"`
	SiteTradeNo   string     `gorm:"size:128" json:"site_trade_no"`
	Amount        int64      `gorm:"not null;default:0" json:"amount"`              // 主站货币金额（分）
	PaidCurrency  string     `gorm:"size:16" json:"paid_currency"`                 // 实际支付币种
	PaidAmount    int64      `json:"paid_amount"`                                    // 实际支付金额
	ExchangeRate  float64    `json:"exchange_rate"`                                  // 汇率
	Method        string     `gorm:"size:64" json:"method"`                         // 支付方式（回调时站点返回）
	PlanVariantID *int64     `json:"plan_variant_id"`                                // 关联套餐（purchase 类型）
	OrderType     string     `gorm:"size:32;not null;default:charge" json:"order_type"` // charge/purchase
	Status        string     `gorm:"size:32;not null;default:pending" json:"status"`
	PaymentURL    string     `gorm:"size:500" json:"payment_url"`                   // 支付跳转链接
	PaidAt        *time.Time `json:"paid_at"`
	ExpiresAt     *time.Time `json:"expires_at"`                                     // 支付有效期
	CallbackRaw   string     `gorm:"type:text" json:"callback_raw"`                  // 回调原始数据
	ChargeCardID  *int64     `json:"charge_card_id"`                                 // 关联充值卡
	Remark        string     `gorm:"size:500" json:"remark"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func (ChargeOrder) TableName() string { return "charge_orders" }

// ============================================================
// Charge Cards — 充值卡号
// ============================================================

// Charge card type constants.
const (
	ChargeCardTypeBalance = "balance" // 金额型
	ChargeCardTypePlan    = "plan"     // 套餐型
)

// Charge card status constants.
const (
	ChargeCardStatusActive   = "active"
	ChargeCardStatusUsed     = "used"
	ChargeCardStatusExpired  = "expired"
	ChargeCardStatusDisabled = "disabled"
)

type ChargeCard struct {
	ID         int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	CardCode   string     `gorm:"size:64;uniqueIndex;not null" json:"card_code"`
	CardType   string     `gorm:"size:32;not null" json:"card_type"`              // balance/plan
	Value      int64      `gorm:"not null;default:0" json:"value"`                 // 金额(最小单位)或 plan_variant_id
	Currency   string     `gorm:"size:16;not null;default:CNY" json:"currency"`     // 货币类型: CNY/RUB/IRR/USD/EUR等
	MaxUses    int        `gorm:"not null;default:1" json:"max_uses"`
	UsedCount  int        `gorm:"not null;default:0" json:"used_count"`
	ExpiresAt  *time.Time `json:"expires_at"`
	Status     string     `gorm:"size:32;not null;default:active" json:"status"`
	BatchID    string     `gorm:"size:64" json:"batch_id"`
	Remark     string     `gorm:"size:500" json:"remark"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

func (ChargeCard) TableName() string { return "charge_cards" }

// ============================================================
// Client Balances — 客户余额
// ============================================================

type ClientBalance struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ClientID  int64     `gorm:"uniqueIndex;not null" json:"client_id"`
	Balance   int64     `gorm:"not null;default:0" json:"balance"`              // 余额（最小单位）
	Frozen    int64     `gorm:"not null;default:0" json:"frozen"`               // 冻结金额（最小单位）
	Currency  string    `gorm:"size:16;not null;default:CNY" json:"currency"` // 货币类型
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (ClientBalance) TableName() string { return "client_balances" }

// ============================================================
// Balance Transactions — 余额流水
// ============================================================

// Balance transaction type constants.
const (
	BalanceTxTypeCharge  = "charge"  // 充值
	BalanceTxTypeConsume = "consume" // 消费
	BalanceTxTypeRefund  = "refund"  // 退款
	BalanceTxTypeGift    = "gift"    // 赠送
)

type BalanceTransaction struct {
	ID                    int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ClientID              int64     `gorm:"not null;index" json:"client_id"`
	Type                  string    `gorm:"size:32;not null" json:"type"`                     // charge/consume/refund/gift
	Amount                int64     `gorm:"not null" json:"amount"`                            // 变动金额（最小单位）
	BalanceAfter          int64     `gorm:"not null" json:"balance_after"`                     // 变动后余额
	Currency              string    `gorm:"size:16;not null;default:CNY" json:"currency"`     // 货币类型
	RelatedChargeOrderID  *int64    `json:"related_charge_order_id"`
	RelatedPlanOrderID    *int64    `json:"related_plan_order_id"`
	Remark                string    `gorm:"size:500" json:"remark"`
	CreatedAt             time.Time `json:"created_at"`
}

func (BalanceTransaction) TableName() string { return "balance_transactions" }
