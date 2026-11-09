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

// Variant type constants.
const (
	VariantTypePlan        = "plan"         // 常规套餐（月付/季付/年付）
	VariantTypeTrafficPack = "traffic_pack" // 流量叠加包（一次性流量）
)

// DurationPermanent 是"永久有效"的魔法值。DurationDays 取该值时表示规格
// 不设到期时间（客户端/订阅的 expires_at 保持 NULL），而不是具体的天数。
const DurationPermanent = -1

// Expiry mode constants for traffic packs.
const (
	ExpiryModeAlignWithPlan = "align_with_plan" // 流量包与套餐同期过期，不延长套餐时间（默认）
	ExpiryModeExtendPlan    = "extend_plan"     // 购买后延长套餐过期时间，延长天数 = DurationDays
)

// PlanVariant is a pricing variant of a PlanTemplate.
// E.g. a template "Standard" may have monthly, quarterly, and annual variants.
type PlanVariant struct {
	ID              int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	TemplateID      int64     `gorm:"not null;index" json:"template_id"`
	VariantType     string    `gorm:"size:32;not null;default:plan" json:"variant_type"` // plan/traffic_pack
	Name            string    `gorm:"size:100;not null" json:"name"`
	Description     string    `gorm:"size:500" json:"description"`
	BillingCycle    string    `gorm:"size:32;not null;default:monthly" json:"billing_cycle"`
	DurationDays    int       `gorm:"not null;default:30" json:"duration_days"`
	Price           int64     `gorm:"not null;default:0" json:"price"`          // in cents
	OriginalPrice   int64     `gorm:"not null;default:0" json:"original_price"` // in cents
	Currency        string    `gorm:"size:8;not null;default:CNY" json:"currency"`
	QuotaBytes      int64     `gorm:"not null;default:0" json:"quota_bytes"` // 0 = inherit template
	QuotaResetCycle string    `gorm:"size:32;not null;default:''" json:"quota_reset_cycle"`
	QuotaResetDay   *int      `json:"quota_reset_day"`
	ExpiryMode      string    `gorm:"size:32;not null;default:align_with_plan" json:"expiry_mode"` // traffic pack: align_with_plan/extend_plan
	IsRecommended   bool      `gorm:"not null;default:false" json:"is_recommended"`
	IsDefault       bool      `gorm:"not null;default:false" json:"is_default"`
	Enabled         bool      `gorm:"not null;default:true" json:"enabled"`
	SortOrder       int       `gorm:"not null;default:0" json:"sort_order"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (PlanVariant) TableName() string { return "plan_variants" }

// IsPermanent reports whether the variant never expires.
func (v *PlanVariant) IsPermanent() bool {
	return v != nil && v.DurationDays < 0
}

// ExpiresFrom returns the new expiry after applying this variant on top of base.
// 永久有效的规格返回 nil（不设到期时间），其余按 DurationDays 天累加。
func (v *PlanVariant) ExpiresFrom(base time.Time) *time.Time {
	if v.IsPermanent() {
		return nil
	}
	t := base.AddDate(0, 0, v.DurationDays)
	return &t
}
