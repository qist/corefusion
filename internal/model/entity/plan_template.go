package entity

import "time"

// Plan category constants.
const (
	PlanCategoryBasic    = "basic"
	PlanCategoryStandard = "standard"
	PlanCategoryPremium  = "premium"
	PlanCategoryTrial    = "trial"
	PlanCategoryCustom   = "custom"
)

// PlanTemplate is the "product definition" of a subscription plan.
// It defines core attributes, node group permissions, feature limits, and trial config.
// Multiple PlanVariants (pricing tiers) belong to one template.
type PlanTemplate struct {
	ID                        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Name                      string    `gorm:"size:100;not null" json:"name"`
	Description               string    `gorm:"size:500" json:"description"`
	Category                  string    `gorm:"size:32;not null;default:general" json:"category"`
	NodeGroupIDs              Int64List `gorm:"type:text;not null" json:"node_group_ids"`
	DeviceLimit               int       `gorm:"not null;default:0" json:"device_limit"`
	SpeedLimitMbps            int       `gorm:"not null;default:0" json:"speed_limit_mbps"`
	ConcurrentLimit           int       `gorm:"not null;default:0" json:"concurrent_limit"`
	DefaultQuotaBytes         int64     `gorm:"not null;default:0" json:"default_quota_bytes"`
	DefaultQuotaResetCycle    string    `gorm:"size:32;not null;default:none" json:"default_quota_reset_cycle"`
	DefaultQuotaResetDay      *int      `json:"default_quota_reset_day"`
	SuspendOnQuotaExhausted   int       `gorm:"not null;default:1" json:"suspend_on_quota_exhausted"`
	IsTrial                   bool      `gorm:"not null;default:false" json:"is_trial"`
	TrialDurationHours        int       `gorm:"not null;default:0" json:"trial_duration_hours"`
	TrialAutoConvertTemplateID *int64   `json:"trial_auto_convert_template_id"`
	BillingUsedTemplate       string    `gorm:"type:text" json:"billing_used_template"`
	BillingRemainingTemplate  string    `gorm:"type:text" json:"billing_remaining_template"`
	BillingResetTemplate      string    `gorm:"type:text" json:"billing_reset_template"`
	BillingExpiryTemplate     string    `gorm:"type:text" json:"billing_expiry_template"`
	BillingNodeIDs            Int64List `gorm:"type:text" json:"billing_node_ids"` // NodeGroupMember IDs selected for billing info rendering
	SortOrder                 int       `gorm:"not null;default:0" json:"sort_order"`
	Enabled                   bool      `gorm:"not null;default:true" json:"enabled"`
	Metadata                  string    `gorm:"type:text" json:"metadata"`
	CreatedAt                 time.Time `json:"created_at"`
	UpdatedAt                 time.Time `json:"updated_at"`
}

func (PlanTemplate) TableName() string { return "plan_templates" }
