package entity

import "time"

// SubscriptionFormat represents a client product subscription format configuration.
type SubscriptionFormat struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string    `gorm:"size:100;uniqueIndex;not null" json:"name"`
	ClientType  string    `gorm:"size:50;not null;index" json:"client_type"`
	Format      string    `gorm:"size:50;not null" json:"format"`
	ContentType string    `gorm:"size:100" json:"content_type"`
	Extension   string    `gorm:"size:20" json:"extension"`
	Description string    `gorm:"size:500" json:"description"`
	IsDefault   bool      `gorm:"default:false" json:"is_default"`
	Enabled     bool      `gorm:"default:true" json:"enabled"`
	SortOrder   int       `gorm:"default:0" json:"sort_order"`
	// Mihomo/Clash specific config sections (JSON)
	ListenersJSON      string `gorm:"type:text" json:"listeners_json"`
	ProxyGroupsJSON    string `gorm:"type:text" json:"proxy_groups_json"`
	RuleProvidersJSON  string `gorm:"type:text" json:"rule_providers_json"`
	ProxyProvidersJSON string `gorm:"type:text" json:"proxy_providers_json"`
	DNSJSON            string `gorm:"type:text" json:"dns_json"`
	GeneralJSON        string `gorm:"type:text" json:"general_json"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

func (SubscriptionFormat) TableName() string { return "subscription_formats" }

// SubscriptionRule represents a routing rule for a subscription format.
type SubscriptionRule struct {
	ID           int64  `gorm:"primaryKey;autoIncrement" json:"id"`
	FormatID     int64  `gorm:"index;not null" json:"format_id"`
	RuleType     string `gorm:"size:50;not null" json:"rule_type"`
	MatchValue   string `gorm:"size:500;not null" json:"match_value"`
	OutboundTag  string `gorm:"size:100;not null" json:"outbound_tag"`
	RuleProvider string `gorm:"size:100" json:"rule_provider"` // Reference to rule-provider name
	RuleAction   string `gorm:"size:200" json:"rule_action"`   // Extra params: no-resolve etc.
	Priority     int    `gorm:"default:0" json:"priority"`
	Enabled      bool   `gorm:"default:true" json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (SubscriptionRule) TableName() string { return "subscription_rules" }
