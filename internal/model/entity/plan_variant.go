package entity

import "time"

// Billing cycle constants.
const (
	BillingCycleMonthly    = "monthly"
	BillingCycleQuarterly  = "quarterly"
	BillingCycleSemiAnnual = "semi_annual"
	BillingCycleAnnual     = "annual"
	BillingCycleLifetime   = "lifetime"
	BillingCycleOneTime    = "one_time"
)

// PlanVariant is a pricing variant of a PlanTemplate.
// E.g. a template "Standard" may have monthly, quarterly, and annual variants.
type PlanVariant struct {
	ID              int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	TemplateID      int64     `gorm:"not null;index" json:"template_id"`
	Name            string    `gorm:"size:100;not null" json:"name"`
	BillingCycle    string    `gorm:"size:32;not null;default:monthly" json:"billing_cycle"`
	DurationDays    int       `gorm:"not null;default:30" json:"duration_days"`
	Price           int64     `gorm:"not null;default:0" json:"price"`           // in cents
	OriginalPrice   int64     `gorm:"not null;default:0" json:"original_price"`  // in cents
	Currency        string    `gorm:"size:8;not null;default:CNY" json:"currency"`
	QuotaBytes      int64     `gorm:"not null;default:0" json:"quota_bytes"`     // 0 = inherit template
	QuotaResetCycle string    `gorm:"size:32;not null;default:''" json:"quota_reset_cycle"`
	QuotaResetDay   *int      `json:"quota_reset_day"`
	IsRecommended   bool      `gorm:"not null;default:false" json:"is_recommended"`
	IsDefault       bool      `gorm:"not null;default:false" json:"is_default"`
	Enabled         bool      `gorm:"not null;default:true" json:"enabled"`
	SortOrder       int       `gorm:"not null;default:0" json:"sort_order"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (PlanVariant) TableName() string { return "plan_variants" }
