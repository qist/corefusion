package entity

import "time"

// Plan represents a subscription plan/package.
type Plan struct {
	ID                       int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Name                     string    `gorm:"size:100;uniqueIndex;not null" json:"name"`
	Description              string    `gorm:"size:500" json:"description"`
	QuotaBytes               int64     `gorm:"not null;default:0" json:"quota_bytes"`
	Duration                 int       `gorm:"not null;default:30" json:"duration"`
	Enabled                  bool      `gorm:"default:true" json:"enabled"`
	SortOrder                int       `gorm:"default:0" json:"sort_order"`
	BillingUsedTemplate      string    `gorm:"type:longtext" json:"billing_used_template"`
	BillingRemainingTemplate string    `gorm:"type:longtext" json:"billing_remaining_template"`
	BillingResetTemplate     string    `gorm:"type:longtext" json:"billing_reset_template"`
	BillingExpiryTemplate    string    `gorm:"type:longtext" json:"billing_expiry_template"`
	BillingInboundConfigIDs  Int64List `gorm:"type:longtext" json:"billing_inbound_config_ids"`
	BillingExternalNodeIDs   Int64List `gorm:"type:longtext" json:"billing_external_node_ids"`
	CreatedAt                time.Time `json:"created_at"`
	UpdatedAt                time.Time `json:"updated_at"`
}

func (Plan) TableName() string { return "plans" }

// PlanNode maps a plan to inbound configs or external nodes.
type PlanNode struct {
	ID              int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	PlanID          int64     `gorm:"not null" json:"plan_id"`
	InboundConfigID int64     `gorm:"default:0" json:"inbound_config_id"`
	ExternalNodeID  int64     `gorm:"default:0" json:"external_node_id"`
	Enabled         bool      `gorm:"default:true" json:"enabled"`
	CreatedAt       time.Time `json:"created_at"`
}

func (PlanNode) TableName() string { return "plan_nodes" }

// Client represents a subscription client/user.
type Client struct {
	ID    int64  `gorm:"primaryKey;autoIncrement" json:"id"`
	Email string `gorm:"size:255;uniqueIndex;not null" json:"email"`
	// Password 存 bcrypt 哈希，禁止序列化进任何 API 响应（安全审计 M-1）：
	// 请求绑定走独立的 Input 结构，不受此 tag 影响。
	Password       string `gorm:"size:255" json:"-"`
	Nickname       string `gorm:"size:100" json:"nickname"`
	QQ             string `gorm:"size:64" json:"qq"`
	Wechat         string `gorm:"size:64" json:"wechat"`
	PlanID         int64  `gorm:"index" json:"plan_id"`                            // legacy, maps to PlanTemplate
	PlanVariantID  int64  `gorm:"not null;default:0;index" json:"plan_variant_id"` // new: links to PlanVariant
	SubscriptionID *int64 `gorm:"index" json:"subscription_id"`
	Status         string `gorm:"size:32;not null;default:active" json:"status"`
	// TokenVersion：登出/踢下线时 +1，使该客户端此前签发的**所有** token（含 refresh）立即失效
	// （与 users.token_version 同一套机制；审计 v5 M8 的管理端方案，M8 遗留的客户端侧补齐）。
	TokenVersion     int        `gorm:"not null;default:0" json:"-"`
	QuotaBytes       int64      `gorm:"not null;default:0" json:"quota_bytes"`
	UsedBytes        int64      `gorm:"not null;default:0" json:"used_bytes"`
	ExpiresAt        *time.Time `json:"expires_at"`
	SuspendedAt      *time.Time `json:"suspended_at"`
	IPWhitelist      string     `gorm:"size:1000" json:"ip_whitelist"`
	DeviceLimit      int        `gorm:"not null;default:0" json:"device_limit"`
	SpeedLimitMbps   int        `gorm:"not null;default:0" json:"speed_limit_mbps"`    // 下载限速 Mbps, inherited from template, can override
	SpeedLimitUpMbps int        `gorm:"not null;default:0" json:"speed_limit_up_mbps"` // 上传限速 Mbps, inherited from template, can override
	ConcurrentLimit  int        `gorm:"not null;default:0" json:"concurrent_limit"`    // inherited from template, can override
	LastNodeUsageAt  *time.Time `json:"last_node_usage_at"`
	SubUpdatedAt     *time.Time `json:"sub_updated_at"`
	Remark           string     `gorm:"size:255" json:"remark"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

func (Client) TableName() string { return "clients" }
