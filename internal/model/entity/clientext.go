package entity

import "time"

// ============================================================
// Referral / 邀请推广
// ============================================================

type ReferralCode struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ClientID  int64     `gorm:"uniqueIndex;not null" json:"client_id"`
	Code      string    `gorm:"size:32;uniqueIndex;not null" json:"code"`
	CreatedAt time.Time `json:"created_at"`
}

func (ReferralCode) TableName() string { return "referral_codes" }

// Referral record status constants.
const (
	CommissionPending   = "pending"
	CommissionAvailable = "available"
	CommissionWithdrawn = "withdrawn"
)

type ReferralRecord struct {
	ID               int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	ReferrerID       int64      `gorm:"not null;index" json:"referrer_id"`
	RefereeID        int64      `gorm:"not null;index" json:"referee_id"`
	RefereeEmail     string     `gorm:"size:255" json:"referee_email"`
	RegisteredAt     time.Time  `json:"registered_at"`
	FirstPaidAt      *time.Time `json:"first_paid_at"`
	CommissionAmount int64      `gorm:"not null;default:0" json:"commission_amount"` // 分
	CommissionStatus string     `gorm:"size:32;not null;default:pending" json:"commission_status"`
	CreatedAt        time.Time  `json:"created_at"`
}

func (ReferralRecord) TableName() string { return "referral_records" }

// Withdrawal status constants.
const (
	WithdrawalPending  = "pending"
	WithdrawalApproved = "approved"
	WithdrawalRejected = "rejected"
)

// Withdraw type constants.
const (
	WithdrawTypeBalance = "balance"
	WithdrawTypeUSDT   = "usdt"
)

type ReferralWithdrawal struct {
	ID             int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	ClientID       int64      `gorm:"not null;index" json:"client_id"`
	Amount         int64      `gorm:"not null" json:"amount"` // 分
	WithdrawType   string     `gorm:"size:32;not null;default:balance" json:"withdraw_type"`
	WalletAddress  string     `gorm:"size:255" json:"wallet_address"`
	Status         string     `gorm:"size:32;not null;default:pending" json:"status"`
	ProcessedAt    *time.Time `json:"processed_at"`
	ProcessRemark  string     `gorm:"size:500" json:"process_remark"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func (ReferralWithdrawal) TableName() string { return "referral_withdrawals" }

// ============================================================
// Ticket / 工单
// ============================================================

// Ticket category constants.
const (
	TicketCategoryTechnical = "technical"
	TicketCategoryBilling   = "billing"
	TicketCategoryAccount   = "account"
	TicketCategoryOther     = "other"
)

// Ticket priority constants.
const (
	TicketPriorityLow    = "low"
	TicketPriorityNormal = "normal"
	TicketPriorityHigh   = "high"
	TicketPriorityUrgent = "urgent"
)

// Ticket status constants.
const (
	TicketStatusOpen    = "open"
	TicketStatusReplied = "replied"
	TicketStatusClosed  = "closed"
)

type Ticket struct {
	ID             int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	ClientID       int64      `gorm:"not null;index" json:"client_id"`
	Subject        string     `gorm:"size:255;not null" json:"subject"`
	Category       string     `gorm:"size:32;not null;default:other" json:"category"`
	Priority       string     `gorm:"size:32;not null;default:normal" json:"priority"`
	Status         string     `gorm:"size:32;not null;default:open" json:"status"`
	AssignedAdminID *int64     `json:"assigned_admin_id"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func (Ticket) TableName() string { return "tickets" }

// Sender type constants.
const (
	SenderTypeClient = "client"
	SenderTypeAdmin  = "admin"
)

type TicketMessage struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	TicketID    int64     `gorm:"not null;index" json:"ticket_id"`
	SenderType  string    `gorm:"size:32;not null" json:"sender_type"` // client/admin
	SenderID    int64     `gorm:"not null" json:"sender_id"`
	Content     string    `gorm:"type:text;not null" json:"content"`
	Attachments string    `gorm:"type:text" json:"attachments"` // JSON array of URLs
	CreatedAt   time.Time `json:"created_at"`
}

func (TicketMessage) TableName() string { return "ticket_messages" }

// ============================================================
// Gift / 福利
// ============================================================

// Gift type constants.
const (
	GiftTypeDays    = "days"
	GiftTypeTraffic = "traffic"
)

// Gift source constants.
const (
	GiftSourceFaultCompensation = "fault_compensation"
	GiftSourceMonthlyBonus      = "monthly_bonus"
	GiftSourcePromotion         = "promotion"
	GiftSourceAdmin             = "admin"
)

// Gift status constants.
const (
	GiftStatusAvailable = "available"
	GiftStatusClaimed   = "claimed"
	GiftStatusExpired   = "expired"
)

type ClientGift struct {
	ID         int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	ClientID   int64      `gorm:"not null;index" json:"client_id"`
	Type       string     `gorm:"size:32;not null" json:"type"`    // days/traffic
	Value      int64      `gorm:"not null" json:"value"`           // 天数 或 字节数
	Source     string     `gorm:"size:64;not null" json:"source"` // 来源
	Status     string     `gorm:"size:32;not null;default:available" json:"status"`
	ExpiresAt  *time.Time `json:"expires_at"`
	ClaimedAt  *time.Time `json:"claimed_at"`
	CreatedAt  time.Time  `json:"created_at"`
}

func (ClientGift) TableName() string { return "client_gifts" }

// ============================================================
// Article / 教程文章
// ============================================================

// Article category constants.
const (
	ArticleCategoryTutorial = "tutorial"
	ArticleCategoryFAQ      = "faq"
	ArticleCategoryNotice   = "notice"
	ArticleCategoryGeneral  = "general"
)

// Article platform constants.
const (
	ArticlePlatformWindows = "windows"
	ArticlePlatformIOS     = "ios"
	ArticlePlatformAndroid = "android"
	ArticlePlatformMac     = "mac"
	ArticlePlatformLinux   = "linux"
	ArticlePlatformGame    = "game"
	ArticlePlatformOther   = "other"
)

type Article struct {
	ID          int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	Title       string     `gorm:"size:255;not null" json:"title"`
	Slug        string     `gorm:"size:128;uniqueIndex;not null" json:"slug"`
	Category    string     `gorm:"size:32;not null;default:general" json:"category"`
	Platform    string     `gorm:"size:32" json:"platform"`
	Content     string     `gorm:"type:longtext;not null" json:"content"` // Markdown
	SortOrder   int        `gorm:"not null;default:0" json:"sort_order"`
	IsPublished bool       `gorm:"not null;default:false" json:"is_published"`
	IsPopup     bool       `gorm:"not null;default:false" json:"is_popup"` // 标记为首页弹窗公告
	PublishedAt *time.Time `json:"published_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (Article) TableName() string { return "articles" }

// ============================================================
// Shared Account / 共享账号
// ============================================================

// Shared account type constants.
const (
	SharedAccountTypeAppleID = "apple_id"
	SharedAccountTypeNetflix = "netflix"
	SharedAccountTypeOther  = "other"
)

type SharedAccount struct {
	ID              int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Name            string    `gorm:"size:255;not null" json:"name"`
	Type            string    `gorm:"size:32;not null;default:other" json:"type"`
	IconURL         string    `gorm:"size:500" json:"icon_url"`
	Credentials     string    `gorm:"type:text;not null" json:"-"`    // encrypted, never JSON exposed
	CredentialsEnc  string    `gorm:"type:text;not null" json:"credentials_enc"`
	MinPlanCategory string    `gorm:"size:32" json:"min_plan_category"` // minimum plan category to view
	SortOrder       int       `gorm:"not null;default:0" json:"sort_order"`
	RepairCooldown  int       `gorm:"not null;default:1440" json:"repair_cooldown"` // minutes
	Enabled         bool      `gorm:"not null;default:true" json:"enabled"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (SharedAccount) TableName() string { return "shared_accounts" }

// Repair log status constants.
const (
	RepairStatusPending   = "pending"
	RepairStatusCompleted = "completed"
	RepairStatusFailed    = "failed"
)

type SharedAccountRepairLog struct {
	ID            int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	AccountID     int64      `gorm:"not null;index" json:"account_id"`
	ClientID      int64      `gorm:"not null;index" json:"client_id"`
	Status        string     `gorm:"size:32;not null;default:pending" json:"status"`
	CompletedAt   *time.Time `json:"completed_at"`
	CreatedAt     time.Time  `json:"created_at"`
}

func (SharedAccountRepairLog) TableName() string { return "shared_account_repair_logs" }

// ============================================================
// OAuth Binding / 第三方绑定
// ============================================================

type ClientOAuthBinding struct {
	ID             int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ClientID       int64     `gorm:"not null;index" json:"client_id"`
	Provider       string    `gorm:"size:32;not null;index" json:"provider"` // github/google/facebook
	ProviderUserID string    `gorm:"size:255;not null" json:"provider_user_id"`
	ProviderName   string    `gorm:"size:255" json:"provider_name"` // display name from OAuth
	BoundAt        time.Time `json:"bound_at"`
	CreatedAt      time.Time `gorm:"not null" json:"created_at"`
	UpdatedAt      time.Time `gorm:"not null" json:"updated_at"`
}

func (ClientOAuthBinding) TableName() string { return "client_oauth_bindings" }

// ============================================================
// Self-Unban / 自解封
// ============================================================

type ClientSelfUnbanLog struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ClientID  int64     `gorm:"not null;index" json:"client_id"`
	Reason    string    `gorm:"size:500" json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

func (ClientSelfUnbanLog) TableName() string { return "client_self_unban_logs" }

// ============================================================
// IP Whitelist Log / IP白名单来源记录
// ============================================================

// IP whitelist source constants.
const (
	IPSourceManual = "manual"
	IPSourceAuto   = "auto"
)

type IPWhitelistEntry struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ClientID  int64     `gorm:"not null;index" json:"client_id"`
	IPAddress string    `gorm:"size:64;not null" json:"ip_address"`
	Source    string    `gorm:"size:32;not null;default:manual" json:"source"` // manual/auto
	CreatedAt time.Time `json:"created_at"`
}

func (IPWhitelistEntry) TableName() string { return "ip_whitelist_entries" }
