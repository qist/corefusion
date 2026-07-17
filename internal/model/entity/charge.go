package entity

import "time"

// ============================================================
// Charge Sites — 充值站点配置
// ============================================================

// ChargeSite represents a configured third-party payment site.
type ChargeSite struct {
	ID                  int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Name                string    `gorm:"size:100;not null" json:"name"`
	SiteURL             string    `gorm:"size:500;not null" json:"site_url"`
	Domain              string    `gorm:"size:255" json:"domain"`                        // 从 SiteURL 提取的域名，用于验证记录
	APIKey              string    `gorm:"size:255" json:"api_key"`
	CallbackSecret      string    `gorm:"size:255" json:"callback_secret"`               // 明文（仅内存中使用，写库前加密）
	EncryptedSecret     string    `gorm:"type:text" json:"encrypted_secret"`             // AES-256-GCM 加密后的当前 callback_secret
	EncryptedSecretPrev string    `gorm:"type:text" json:"encrypted_secret_prev"`        // AES-256-GCM 加密后的旧 callback_secret（密钥轮换过渡期）
	SecretVersion       int       `gorm:"not null;default:1" json:"secret_version"`        // 密钥版本号，轮换时递增，用于审计追踪
	CallbackWhitelist   string    `gorm:"size:1000" json:"callback_whitelist"`           // JSON array of allowed IPs/CIDRs
	SupportedMethods    string    `gorm:"type:text" json:"supported_methods"`            // JSON array
	Currency            string    `gorm:"size:16;not null;default:CNY" json:"currency"`  // 站点主货币
	IconURL             string    `gorm:"size:500" json:"icon_url"`
	Enabled             bool      `gorm:"not null;default:true" json:"enabled"`
	SortOrder           int       `gorm:"not null;default:0" json:"sort_order"`
	VerifyToken         string    `gorm:"size:64" json:"verify_token"`
	VerifyContent       string    `gorm:"size:128" json:"verify_content"`
	Verified            bool      `gorm:"not null;default:false" json:"verified"`
	VerifiedMethod      string    `gorm:"size:16" json:"verified_method"`                // http / dns_txt
	VerifiedAt          *time.Time `gorm:"index" json:"verified_at"`                     // 验证时间
	VerifyExpireAt      *time.Time `gorm:"index" json:"verify_expire_at"`                // 验证过期时间（180天后需重新验证）
	ConfigJSON          string    `gorm:"type:text" json:"config_json"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
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
	ChargeStatusCreated    = "created"
	ChargeStatusPending    = "pending"
	ChargeStatusProcessing = "processing" // 中间态：已获得处理权，正在入账
	ChargeStatusPaid       = "paid"
	ChargeStatusFailed     = "failed"
	ChargeStatusExpired    = "expired"
	ChargeStatusRefunded   = "refunded"
)

// chargeOrderTransitions defines the allowed state machine transitions.
var chargeOrderTransitions = map[string]map[string]bool{
	ChargeStatusCreated:    {ChargeStatusPending: true},
	ChargeStatusPending:    {ChargeStatusProcessing: true, ChargeStatusFailed: true, ChargeStatusExpired: true},
	ChargeStatusProcessing: {ChargeStatusPaid: true, ChargeStatusFailed: true},
	ChargeStatusPaid:       {ChargeStatusRefunded: true},
	ChargeStatusFailed:     {ChargeStatusPending: true}, // 允许重新支付
	ChargeStatusExpired:    {},                          // 终态
	ChargeStatusRefunded:   {},                          // 终态
}

// CanChargeOrderTransition checks whether a status transition is allowed.
func CanChargeOrderTransition(from, to string) bool {
	if from == to {
		return true // 幂等
	}
	if allowed, ok := chargeOrderTransitions[from]; ok {
		return allowed[to]
	}
	return false
}

// Charge order type constants.
const (
	ChargeOrderTypeCharge   = "charge"
	ChargeOrderTypePurchase = "purchase"
)

type ChargeOrder struct {
	ID            int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	ClientID      int64      `gorm:"not null;index" json:"client_id"`
	TradeNo       string     `gorm:"size:64;uniqueIndex;not null" json:"trade_no"`
	Source        string     `gorm:"size:32;not null" json:"source"`
	SiteID        *int64     `gorm:"index" json:"site_id"`
	SiteTradeNo   string     `gorm:"size:128" json:"site_trade_no"`
	Amount        int64      `gorm:"not null;default:0" json:"amount"`
	PaidCurrency  string     `gorm:"size:16" json:"paid_currency"`
	PaidAmount    int64      `json:"paid_amount"`
	ExchangeRate  float64    `json:"exchange_rate"`
	Method        string     `gorm:"size:64" json:"method"`
	PlanVariantID *int64     `json:"plan_variant_id"`
	OrderType     string     `gorm:"size:32;not null;default:charge" json:"order_type"`
	Status        string     `gorm:"size:32;not null;default:pending" json:"status"`
	PaymentURL    string     `gorm:"size:500" json:"payment_url"`
	PaidAt        *time.Time `json:"paid_at"`
	ProcessingAt  *time.Time `gorm:"index" json:"processing_at"`                   // 进入 processing 状态的时间，用于超时恢复
	ExpiresAt     *time.Time `json:"expires_at"`
	CallbackRaw   string     `gorm:"type:text" json:"callback_raw"`
	ChargeCardID  *int64     `json:"charge_card_id"`
	Remark        string     `gorm:"size:500" json:"remark"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func (ChargeOrder) TableName() string { return "charge_orders" }

// ============================================================
// Charge Cards — 充值卡号
// ============================================================

const (
	ChargeCardTypeBalance = "balance"
	ChargeCardTypePlan    = "plan"
)

const (
	ChargeCardStatusActive   = "active"
	ChargeCardStatusUsed     = "used"
	ChargeCardStatusExpired  = "expired"
	ChargeCardStatusDisabled = "disabled"
)

type ChargeCard struct {
	ID         int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	CardCode   string     `gorm:"size:64;uniqueIndex;not null" json:"card_code"`
	CardType   string     `gorm:"size:32;not null" json:"card_type"`
	Value      int64      `gorm:"not null;default:0" json:"value"`
	Currency   string     `gorm:"size:16;not null;default:CNY" json:"currency"`
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
	Balance   int64     `gorm:"not null;default:0" json:"balance"`
	Frozen    int64     `gorm:"not null;default:0" json:"frozen"`
	Currency  string    `gorm:"size:16;not null;default:CNY" json:"currency"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (ClientBalance) TableName() string { return "client_balances" }

// ============================================================
// Balance Transactions — 余额流水
// ============================================================

const (
	BalanceTxTypeCharge  = "charge"
	BalanceTxTypeConsume = "consume"
	BalanceTxTypeRefund  = "refund"
	BalanceTxTypeGift    = "gift"
)

type BalanceTransaction struct {
	ID                    int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ClientID              int64     `gorm:"not null;index" json:"client_id"`
	Type                  string    `gorm:"size:32;not null" json:"type"`
	Amount                int64     `gorm:"not null" json:"amount"`
	BalanceBefore         int64     `gorm:"not null;default:0" json:"balance_before"`        // 变动前余额（审计快照）
	BalanceAfter          int64     `gorm:"not null" json:"balance_after"`                   // 变动后余额
	Currency              string    `gorm:"size:16;not null;default:CNY" json:"currency"`
	RelatedChargeOrderID  *int64    `gorm:"uniqueIndex:idx_balance_tx_order_type,priority:3" json:"related_charge_order_id"`
	RelatedPlanOrderID    *int64    `json:"related_plan_order_id"`
	Remark                string    `gorm:"size:500" json:"remark"`
	CreatedAt             time.Time `json:"created_at"`
}

func (BalanceTransaction) TableName() string { return "balance_transactions" }

// ============================================================
// Callback Nonces — 回调防重放（站点+nonce 联合唯一）
// ============================================================

// CallbackNonce records a used nonce to prevent replay attacks.
// Unique constraint on (site_id, nonce) — different sites may generate the same nonce.
type CallbackNonce struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Nonce     string    `gorm:"size:64;uniqueIndex:idx_site_nonce,priority:2;not null" json:"nonce"`
	SiteID    int64     `gorm:"uniqueIndex:idx_site_nonce,priority:1;not null" json:"site_id"`
	TradeNo   string    `gorm:"size:64;not null" json:"trade_no"`
	CreatedAt time.Time `gorm:"index:idx_nonce_created" json:"created_at"`
}

func (CallbackNonce) TableName() string { return "callback_nonces" }

// ============================================================
// Charge Callback Events — 回调事件记录（每次回调一条，用于争议排查）
// ============================================================

// ChargeCallbackEvent records every callback as a unified event+log for dispute resolution and debugging.
// v3.1: Merged charge_callback_logs into this table — events ARE logs.
type ChargeCallbackEvent struct {
	ID           int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	SiteID       int64     `gorm:"not null;index" json:"site_id"`
	TradeNo      string    `gorm:"size:64;index" json:"trade_no"`
	RequestID    string    `gorm:"size:64;index" json:"request_id"`
	RemoteIP     string    `gorm:"size:64" json:"remote_ip"`
	IpMatchRule  string    `gorm:"size:64" json:"ip_match_rule"`      // 匹配的 IP 白名单规则（如 10.0.0.0/8），便于排查
	PayloadHash  string    `gorm:"size:64" json:"payload_hash"`      // SHA-256(rawBody), 用于争议比对
	SignValid    bool      `gorm:"not null;default:false" json:"sign_valid"`
	SecretVer    int       `json:"secret_version"`                   // 验签使用的密钥版本
	Result       string    `gorm:"size:32;not null" json:"result"`   // received / signature_failed / replay / amount_error / success / error
	ErrorCode    string    `gorm:"size:64" json:"error_code"`
	ErrorMessage string    `gorm:"size:500" json:"error_message"`
	Amount       int64     `json:"amount"`                            // 回调金额（用于审计）
	CreatedAt    time.Time `json:"created_at"`
}

func (ChargeCallbackEvent) TableName() string { return "charge_callback_events" }

// ============================================================
// Charge Callback Retry Queue — 回调重试队列
// ============================================================

// Retry error classification: only transient errors are retried.
const (
	RetryableError    = "retryable"     // DB_TIMEOUT, network_error, balance_service_error
	NonRetryableError = "non_retryable" // INVALID_SIGNATURE, AMOUNT_MISMATCH, ORDER_NOT_FOUND
)

// ChargeCallbackRetry stores a failed callback for retry processing.
type ChargeCallbackRetry struct {
	ID            int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	SiteID        int64      `gorm:"not null;index" json:"site_id"`
	RequestID     string     `gorm:"size:64" json:"request_id"`
	TradeNo       string     `gorm:"size:64;index" json:"trade_no"`
	RemoteIP      string     `gorm:"size:64" json:"remote_ip"`
	PayloadHash   string     `gorm:"size:64" json:"payload_hash"`        // SHA-256 of raw body
	ErrorCode     string     `gorm:"size:64" json:"error_code"`          // 错误码，用于分类
	ErrorMessage  string     `gorm:"size:500" json:"error_message"`
	Retryable     bool       `gorm:"not null;default:true" json:"retryable"` // 是否可重试
	RetryCount    int        `gorm:"not null;default:0" json:"retry_count"`
	MaxRetries    int        `gorm:"not null;default:5" json:"max_retries"`
	Status        string     `gorm:"size:32;not null;default:pending;index" json:"status"`
	NextRetryAt   *time.Time `gorm:"index" json:"next_retry_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func (ChargeCallbackRetry) TableName() string { return "charge_callback_retries" }
