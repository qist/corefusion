package entity

import "time"

// Subscription is the GORM model for the subscriptions table.
type Subscription struct {
	ID int64 `json:"id" gorm:"primaryKey;autoIncrement"`
	ClientID int64 `json:"client_id" gorm:"index;not null"`
	MihomoConfigID *int64 `json:"mihomo_config_id" gorm:"index"` // Associated mihomo config
	Format string `json:"format" gorm:"size:32;not null"`
	Token string `json:"token" gorm:"size:128;uniqueIndex;not null"`
	TokenVersion int `json:"token_version" gorm:"not null;default:1"`
	Status string `json:"status" gorm:"size:32;not null;default:active"`
	SecurityMode string `json:"security_mode" gorm:"size:32;not null;default:token_only"`
	Signature string `json:"signature" gorm:"size:255"`
	BindMode string `json:"bind_mode" gorm:"size:32;not null;default:none"`
	BindValue string `json:"bind_value" gorm:"size:255"`
	ActivatedAt             *time.Time `json:"activated_at"`
	MaxActiveDays           *int
	ExpiresAt               *time.Time `json:"expires_at"`
	QuotaBytes int64 `json:"quota_bytes" gorm:"not null;default:0"`
	UsedBytesCache int64 `json:"used_bytes_cache" gorm:"not null;default:0"`
	QuotaResetCycle string `json:"quota_reset_cycle" gorm:"size:32;not null;default:none"`
	QuotaResetDay           *int
	LastResetAt             *time.Time `json:"last_reset_at"`
	NextResetAt             *time.Time `json:"next_reset_at"`
	SuspendOnQuotaExhausted int `json:"suspend_on_quota_exhausted" gorm:"not null;default:1"`
	QuotaState string `json:"quota_state" gorm:"size:32;not null;default:normal"`
	SuspendedAt             *time.Time `json:"suspended_at"`
	RevokedAt               *time.Time `json:"revoked_at"`
	LastAccessAt            *time.Time `json:"last_access_at"`
	AccessCount int64 `json:"access_count" gorm:"not null;default:0"`
	LastIP string `json:"ip" gorm:"size:64"`
	LastUserAgent string `json:"user_agent" gorm:"size:512"`
	BillingNodeIDs         Int64List  `json:"billing_node_ids" gorm:"type:text"`
	CreatedAt               time.Time  `json:"created_at" gorm:"not null"`
	UpdatedAt     time.Time `json:"updated_at" gorm:"not null"`
}

func (Subscription) TableName() string { return "subscriptions" }

// SubscriptionAccessLog is the GORM model for subscription access logs.
type SubscriptionAccessLog struct {
	ID int64 `json:"id" gorm:"primaryKey;autoIncrement"`
	SubscriptionID int64 `json:"subscription_id" gorm:"index:idx_sub_access_logs_sub_time,priority:1;not null"`
	IP string `json:"ip" gorm:"size:64"`
	RealIP string `json:"real_ip" gorm:"size:64"`
	CDNIP string `json:"cdn_ip" gorm:"size:64"`
	CDNProvider string `json:"cdn_provider" gorm:"size:32"`
	AccessType string `json:"access_type" gorm:"size:16"`
	UserAgent string `json:"user_agent" gorm:"size:512"`
	StatusCode int `json:"status_code" gorm:"not null"`
	HitResult string `json:"hit_result" gorm:"size:32;not null"`
	RiskLevel string `json:"risk_level" gorm:"size:16"`
	CreatedAt time.Time `json:"created_at" gorm:"index:idx_sub_access_logs_sub_time,priority:2;not null"`
}

func (SubscriptionAccessLog) TableName() string { return "subscription_access_logs" }

// SubscriptionRotation is the GORM model for subscription token rotation history.
type SubscriptionRotation struct {
	ID int64 `json:"id" gorm:"primaryKey;autoIncrement"`
	SubscriptionID int64 `json:"subscription_id" gorm:"index:idx_sub_rotations_sub_time,priority:1;not null"`
	OldToken string `json:"old_token" gorm:"size:128;not null"`
	NewToken string `json:"new_token" gorm:"size:128;not null"`
	RotatedBy int64 `json:"rotated_by" gorm:"not null"`
	Reason string `json:"reason" gorm:"size:255"`
	CreatedAt time.Time `json:"created_at" gorm:"index:idx_sub_rotations_sub_time,priority:2;not null"`
}

func (SubscriptionRotation) TableName() string { return "subscription_rotations" }

// ServerShareProfile is the GORM model for server share profiles.
type ServerShareProfile struct {
	ID int64 `json:"id" gorm:"primaryKey;autoIncrement"`
	Name string `json:"name" gorm:"size:128;not null"`
	ClientID            *int64
	TargetClientType string `json:"target_client_type" gorm:"size:32;not null"`
	OutputFormat string `json:"output_format" gorm:"size:32;not null"`
	DeliveryMode string `json:"delivery_mode" gorm:"size:32;not null"`
	TemplateID          *int64
	RouteTemplateID     *int64
	DefaultOutboundMode string `json:"default_outbound_mode" gorm:"size:32;not null;default:rule"`
	SecurityMode string `json:"security_mode" gorm:"size:32;not null;default:token_only"`
	BindMode string `json:"bind_mode" gorm:"size:32;not null;default:none"`
	IncludeRuleURLs int `json:"include_rule_urls" gorm:"not null;default:1"`
	IncludeCustomRules int `json:"include_custom_rules" gorm:"not null;default:1"`
	HTTPSRequired int `json:"https_required" gorm:"not null;default:1"`
	Status string `json:"status" gorm:"size:32;not null;default:draft"`
	LastPreviewAt       *time.Time `json:"last_preview_at"`
	LastPublishedAt     *time.Time `json:"last_published_at"`
	CreatedAt           time.Time  `json:"created_at" gorm:"not null"`
	UpdatedAt     time.Time `json:"updated_at" gorm:"not null"`
}

func (ServerShareProfile) TableName() string { return "server_share_profiles" }
