package subscription

import (
	"time"

	"corefusion/internal/domain"
)

// Subscription represents a server-side subscription credential.
type Subscription struct {
	ID                     int64                  `json:"id"`
	ClientID               int64                  `json:"client_id"`
	Format                 string                 `json:"format"`
	Token                  string                 `json:"token"`
	TokenVersion           int                    `json:"token_version"`
	Status                 string                 `json:"status"`
	SecurityMode           string                 `json:"security_mode"`
	Signature              string                 `json:"signature,omitempty"`
	BindMode               string                 `json:"bind_mode"`
	BindValue              string                 `json:"bind_value,omitempty"`
	ActivatedAt            *time.Time             `json:"activated_at,omitempty"`
	MaxActiveDays          *int                   `json:"max_active_days,omitempty"`
	ExpiresAt              *time.Time             `json:"expires_at,omitempty"`
	QuotaBytes             int64                  `json:"quota_bytes"`
	UsedBytesCache         int64                  `json:"used_bytes_cache"`
	QuotaResetCycle        string                 `json:"quota_reset_cycle"`
	QuotaResetDay          *int                   `json:"quota_reset_day,omitempty"`
	LastResetAt            *time.Time             `json:"last_reset_at,omitempty"`
	NextResetAt            *time.Time             `json:"next_reset_at,omitempty"`
	SuspendOnQuotaExhausted bool                  `json:"suspend_on_quota_exhausted"`
	QuotaState             domain.QuotaState      `json:"quota_state"`
	SuspendedAt            *time.Time             `json:"suspended_at,omitempty"`
	RevokedAt              *time.Time             `json:"revoked_at,omitempty"`
	LastAccessAt           *time.Time             `json:"last_access_at,omitempty"`
	AccessCount            int64                  `json:"access_count"`
	LastIP                 string                 `json:"last_ip,omitempty"`
	LastUserAgent          string                 `json:"last_user_agent,omitempty"`
	CreatedAt              time.Time              `json:"created_at"`
	UpdatedAt              time.Time              `json:"updated_at"`
}

// Subscription status constants.
const (
	StatusActive   = "active"
	StatusRevoked  = "revoked"
	StatusExpired  = "expired"
	StatusDisabled = "disabled"
)

// SecurityMode constants.
const (
	SecurityModeTokenOnly        = "token_only"
	SecurityModeTokenWithExpiry  = "token_with_expiry"
	SecurityModeTokenWithSignature = "token_with_signature"
	SecurityModeTokenWithBinding = "token_with_binding"
)

// ServerShareProfile represents a server-side node share profile.
type ServerShareProfile struct {
	ID                 int64     `json:"id"`
	Name               string    `json:"name"`
	ClientID           *int64    `json:"client_id,omitempty"`
	TargetClientType   string    `json:"target_client_type"`
	OutputFormat       string    `json:"output_format"`
	DeliveryMode       string    `json:"delivery_mode"`
	TemplateID         *int64    `json:"template_id,omitempty"`
	RouteTemplateID    *int64    `json:"route_template_id,omitempty"`
	DefaultOutboundMode string   `json:"default_outbound_mode"`
	SecurityMode       string    `json:"security_mode"`
	BindMode           string    `json:"bind_mode"`
	IncludeRuleURLs    bool      `json:"include_rule_urls"`
	IncludeCustomRules bool      `json:"include_custom_rules"`
	HTTPSRequired      bool      `json:"https_required"`
	Status             string    `json:"status"`
	LastPreviewAt      *time.Time `json:"last_preview_at,omitempty"`
	LastPublishedAt    *time.Time `json:"last_published_at,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}
