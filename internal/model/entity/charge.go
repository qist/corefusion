package entity

import "time"

// ============================================================
// Charge Sites — 充值站点配置
// ============================================================

// ChargeSite represents a configured third-party payment site.
type ChargeSite struct {
	ID                int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Name              string    `gorm:"size:100;not null" json:"name"`
	SiteURL           string    `gorm:"size:500;not null" json:"site_url"`
	Domain            string    `gorm:"size:255" json:"domain"`
	APIKey            string    `gorm:"size:255" json:"api_key"`
	CallbackSecret    string    `gorm:"size:255" json:"callback_secret"`         // 明文（仅内存中使用，写库前加密）
	EncryptedSecret   string    `gorm:"type:text" json:"encrypted_secret"`       // AES-256-GCM 加密后的当前密钥（兼容旧逻辑，新逻辑用 charge_site_secrets 表）
	EncryptedSecretPrev string  `gorm:"type:text" json:"encrypted_secret_prev"`  // 兼容旧逻辑
	SecretVersion     int       `gorm:"not null;default:1" json:"secret_version"` // 当前密钥版本号
	CallbackWhitelist string    `gorm:"size:1000" json:"callback_whitelist"`     // JSON array of allowed IPs/CIDRs（P2 辅助）
	SupportedMethods  string    `gorm:"type:text" json:"supported_methods"`      // JSON array
	Currency          string    `gorm:"size:16;not null;default:CNY" json:"currency"`
	IconURL           string    `gorm:"size:500" json:"icon_url"`
	Enabled           bool      `gorm:"not null;default:true" json:"enabled"`
	SortOrder         int       `gorm:"not null;default:0" json:"sort_order"`
	VerifyToken       string    `gorm:"size:64" json:"verify_token"`
	VerifyContent     string    `gorm:"size:128" json:"verify_content"`
	Verified          bool      `gorm:"not null;default:false" json:"verified"`
	VerifiedMethod    string    `gorm:"size:16" json:"verified_method"`
	VerifiedAt        *time.Time `gorm:"index" json:"verified_at"`
	VerifyExpireAt    *time.Time `gorm:"index" json:"verify_expire_at"`
	ConfigJSON        string    `gorm:"type:text" json:"config_json"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func (ChargeSite) TableName() string { return "charge_sites" }

// ============================================================
// Charge Site Secrets — 密钥版本表（v3.2 新增）
// 支持多版本密钥管理，类似 Stripe/PayPal webhook secret rotation
// ============================================================

// ChargeSiteSecretStatus constants.
const (
	SecretStatusActive  = "active"  // 当前有效密钥
	SecretStatusLegacy  = "legacy"  // 旧密钥，仍可用于验签（轮换过渡期）
	SecretStatusExpired = "expired" // 已过期，不再使用
)

// ChargeSiteSecret represents a versioned callback secret for a charge site.
type ChargeSiteSecret struct {
	ID              int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	SiteID          int64      `gorm:"not null;index:idx_site_version,unique,priority:1" json:"site_id"`
	Version         int        `gorm:"not null;index:idx_site_version,unique,priority:2" json:"version"` // (site_id, version) 联合唯一
	EncryptedSecret string     `gorm:"type:text;not null" json:"encrypted_secret"`                       // AES-256-GCM 加密
	Status          string     `gorm:"size:16;not null;default:active;index" json:"status"`              // active / legacy / expired
	CreatedAt       time.Time  `json:"created_at"`
	ExpiredAt       *time.Time `gorm:"index" json:"expired_at"` // 过期时间（legacy → expired 的自动转换）
}

func (ChargeSiteSecret) TableName() string { return "charge_site_secrets" }

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
	ChargeStatusProcessing = "processing" // 中间态：事务内锁定，处理中
	ChargeStatusPaid       = "paid"
	ChargeStatusFailed     = "failed"
	ChargeStatusExpired    = "expired"
	ChargeStatusRefunded   = "refunded"
)

// Callback status values (from payment gateway).
// Only these values are accepted; unknown statuses are rejected.
const (
	CallbackStatusPaid    = "paid"    // 支付成功
	CallbackStatusFailed  = "failed"  // 支付失败
	CallbackStatusPending = "pending" // 支付进行中（中间态，不改订单状态）
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
	ID             int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	ClientID       int64      `gorm:"not null;index" json:"client_id"`
	TradeNo        string     `gorm:"size:64;uniqueIndex;not null" json:"trade_no"`
	Source         string     `gorm:"size:32;not null" json:"source"`
	SiteID         *int64     `gorm:"index" json:"site_id"`
	SiteTradeNo    string     `gorm:"size:128" json:"site_trade_no"`
	Amount         int64      `gorm:"not null;default:0" json:"amount"`
	Currency       string     `gorm:"size:16;not null;default:CNY" json:"currency"` // v3.2: 订单创建时锁定币种
	PaidCurrency   string     `gorm:"size:16" json:"paid_currency"`
	PaidAmount     int64      `json:"paid_amount"`
	ExchangeRate   float64    `json:"exchange_rate"`
	Method         string     `gorm:"size:64" json:"method"`
	PlanVariantID  *int64     `json:"plan_variant_id"`
	OrderType      string     `gorm:"size:32;not null;default:charge" json:"order_type"`
	Status         string     `gorm:"size:32;not null;default:pending" json:"status"`
	PaymentURL     string     `gorm:"size:500" json:"payment_url"`
	PaidAt         *time.Time `json:"paid_at"`
	ProcessingAt   *time.Time `gorm:"index" json:"processing_at"` // 进入 processing 状态的时间，用于超时恢复
	ExpiresAt      *time.Time `json:"expires_at"`
	CallbackRaw    string     `gorm:"type:text" json:"callback_raw"`
	CallbackCount  int        `gorm:"not null;default:0" json:"callback_count"`     // v3.2: 回调次数（异常频率检测）
	LastCallbackAt *time.Time `gorm:"index" json:"last_callback_at"`                // v3.2: 最后回调时间
	ChargeCardID   *int64     `json:"charge_card_id"`
	Remark         string     `gorm:"size:500" json:"remark"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
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
	ID                int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	CardCode          string     `gorm:"-" json:"card_code"`                                          // 明文，仅内存中使用（从 CardCodeEncrypted 解密填充）
	CardCodeHash      string     `gorm:"size:64;uniqueIndex;not null" json:"-"`                       // SHA-256 hash，用于快速查找（不返回前端）
	CardCodeEncrypted string     `gorm:"type:text" json:"-"`                                          // AES-256-GCM 加密存储（不返回前端）
	CardCodeMasked    string     `gorm:"-" json:"card_code_masked"`                                  // 脱敏卡号（列表展示用）
	CardType          string     `gorm:"size:32;not null" json:"card_type"`
	Value             int64      `gorm:"not null;default:0" json:"value"`
	Currency          string     `gorm:"size:16;not null;default:CNY" json:"currency"`
	MaxUses           int        `gorm:"not null;default:1" json:"-"`                                // v3.3: 废弃，固定为1（保留字段兼容旧DB）
	UsedCount         int        `gorm:"not null;default:0" json:"used_count"`                       // v3.3: 0=未使用，1=已使用
	ExpiresAt         *time.Time `json:"expires_at"`
	Status            string     `gorm:"size:32;not null;default:active" json:"status"`
	BatchID           string     `gorm:"size:64" json:"batch_id"`
	Remark            string     `gorm:"size:500" json:"remark"`
	CreatedByAdminID  int64      `gorm:"not null;default:0;index" json:"created_by_admin"`           // 创建者管理员ID（关联 users 表）
	CreatedByAdminName string    `gorm:"size:64;not null;default:''" json:"created_by_admin_name"`   // 创建者用户名（冗余，便于展示）
	AgentID           *int64     `gorm:"index" json:"agent_id"`                                      // 代理员/推广员管理员ID（关联 users 表），为空表示官方
	AgentName         string     `gorm:"size:64" json:"agent_name"`                                  // 代理员用户名（冗余，便于展示）
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
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
	Type                  string    `gorm:"size:32;not null;uniqueIndex:idx_balance_tx_order_type,priority:1" json:"type"`
	Amount                int64     `gorm:"not null;default:0" json:"amount"`
	BalanceBefore         int64     `gorm:"not null;default:0" json:"balance_before"`
	BalanceAfter          int64     `gorm:"not null;default:0" json:"balance_after"`
	Currency              string    `gorm:"size:16;not null;default:CNY" json:"currency"`
	RelatedChargeOrderID  *int64    `gorm:"uniqueIndex:idx_balance_tx_order_type,priority:2" json:"related_charge_order_id"`
	RelatedPlanOrderID    *int64    `json:"related_plan_order_id"`
	Remark                string    `gorm:"size:500" json:"remark"`
	CreatedAt             time.Time `json:"created_at"`
}

func (BalanceTransaction) TableName() string { return "balance_transactions" }

// ============================================================
// Callback Nonces — 回调防重放（站点+nonce 联合唯一）
// v3.2: 增加 sign_valid 字段，保留24小时
// ============================================================

// CallbackNonce records a used nonce to prevent replay attacks.
// Unique constraint on (site_id, nonce) — different sites may generate the same nonce.
type CallbackNonce struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Nonce      string    `gorm:"size:64;uniqueIndex:idx_site_nonce,priority:2;not null" json:"nonce"`
	SiteID     int64     `gorm:"uniqueIndex:idx_site_nonce,priority:1;not null" json:"site_id"`
	TradeNo    string    `gorm:"size:64;not null" json:"trade_no"`
	SignValid  bool      `gorm:"not null;default:false" json:"sign_valid"` // v3.2: 签名是否通过
	CreatedAt  time.Time `gorm:"index:idx_nonce_created" json:"created_at"`
}

func (CallbackNonce) TableName() string { return "callback_nonces" }

// ============================================================
// Charge Callback Events — 回调事件记录
// v3.2: 事务外异步写入，不阻塞账务
// ============================================================

// ChargeCallbackEvent records every callback as a unified event+log for dispute resolution and debugging.
type ChargeCallbackEvent struct {
	ID           int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	SiteID       int64     `gorm:"not null;index" json:"site_id"`
	TradeNo      string    `gorm:"size:64;index" json:"trade_no"`
	RequestID    string    `gorm:"size:64;index" json:"request_id"`
	RemoteIP     string    `gorm:"size:64" json:"remote_ip"`
	IpMatchRule  string    `gorm:"size:64" json:"ip_match_rule"`
	PayloadHash  string    `gorm:"size:64" json:"payload_hash"`
	SignValid    bool      `gorm:"not null;default:false" json:"sign_valid"`
	SecretVer    int       `json:"secret_version"`
	Result       string    `gorm:"size:32;not null" json:"result"`
	ErrorCode    string    `gorm:"size:64" json:"error_code"`
	ErrorMessage string    `gorm:"size:500" json:"error_message"`
	Amount       int64     `json:"amount"`
	CreatedAt    time.Time `json:"created_at"`
}

func (ChargeCallbackEvent) TableName() string { return "charge_callback_events" }

// ============================================================
// Charge Callback Retry Queue — 回调重试队列
// ============================================================

const (
	RetryableError    = "retryable"
	NonRetryableError = "non_retryable"
)

type ChargeCallbackRetry struct {
	ID            int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	SiteID        int64      `gorm:"not null;index" json:"site_id"`
	RequestID     string     `gorm:"size:64" json:"request_id"`
	TradeNo       string     `gorm:"size:64;index" json:"trade_no"`
	RemoteIP      string     `gorm:"size:64" json:"remote_ip"`
	PayloadHash   string     `gorm:"size:64" json:"payload_hash"`
	ErrorCode     string     `gorm:"size:64" json:"error_code"`
	ErrorMessage  string     `gorm:"size:500" json:"error_message"`
	Retryable     bool       `gorm:"not null;default:true" json:"retryable"`
	RetryCount    int        `gorm:"not null;default:0" json:"retry_count"`
	MaxRetries    int        `gorm:"not null;default:5" json:"max_retries"`
	Status        string     `gorm:"size:32;not null;default:pending;index" json:"status"`
	NextRetryAt   *time.Time `gorm:"index" json:"next_retry_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func (ChargeCallbackRetry) TableName() string { return "charge_callback_retries" }

// ============================================================
// Charge Security Events — 充值风控事件（v3.2 新增）
// 用于 Grafana/Loki 告警，与 SecurityAlert 互补
// ============================================================

// Charge security event severity levels.
const (
	ChargeSecSeverityLow      = "low"
	ChargeSecSeverityMedium   = "medium"
	ChargeSecSeverityHigh     = "high"
	ChargeSecSeverityCritical = "critical"
)

// Charge security event types.
const (
	ChargeSecEventSigFailed        = "signature_failed"
	ChargeSecEventNonceReplay      = "nonce_replay"
	ChargeSecEventAmountMismatch   = "amount_mismatch"
	ChargeSecEventCurrencyMismatch = "currency_mismatch"
	ChargeSecEventIPChanged        = "ip_changed"
	ChargeSecEventCallbackFlood    = "callback_flood"
	ChargeSecEventRecoveryStuck    = "recovery_stuck"
	ChargeSecEventInvalidStatus    = "invalid_status"      // 回调状态值未知
	ChargeSecEventTransactionError = "transaction_error"   // 事务执行失败
)

// ChargeSecurityEvent records a security-relevant event in the charge flow.
type ChargeSecurityEvent struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	SiteID     int64     `gorm:"not null;index" json:"site_id"`
	TradeNo    string    `gorm:"size:64;index" json:"trade_no"`
	RequestID  string    `gorm:"size:64" json:"request_id"`
	EventType  string    `gorm:"size:32;not null;index" json:"event_type"`
	Severity   string    `gorm:"size:16;not null;default:medium" json:"severity"`
	RemoteIP   string    `gorm:"size:64" json:"remote_ip"`
	Detail     string    `gorm:"type:text" json:"detail"`
	CreatedAt  time.Time `gorm:"index" json:"created_at"`
}

func (ChargeSecurityEvent) TableName() string { return "charge_security_events" }
