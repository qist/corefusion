package entity

import "time"

// Discount type constants.
const (
	DiscountTypePercentage   = "percentage"
	DiscountTypeFixedAmount  = "fixed_amount"
)

// Promotion scope constants.
const (
	PromoScopeAll             = "all"
	PromoScopeTemplate        = "template"
	PromoScopeSpecificVariant = "specific_variant"
)

// Promotion represents a promotional discount code.
type Promotion struct {
	ID                int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	Code              string     `gorm:"size:64;uniqueIndex;not null" json:"code"`
	Name              string     `gorm:"size:200;not null" json:"name"`
	Description       string     `gorm:"size:500" json:"description"`
	DiscountType      string     `gorm:"size:32;not null;default:percentage" json:"discount_type"`
	DiscountValue     int64      `gorm:"not null;default:0" json:"discount_value"`
	MaxDiscountAmount int64      `gorm:"not null;default:0" json:"max_discount_amount"`
	ApplicableScope   string     `gorm:"size:32;not null;default:all" json:"applicable_scope"`
	ApplicableIDs     Int64List  `gorm:"type:text;not null" json:"applicable_ids"`
	MaxUses           int        `gorm:"not null;default:0" json:"max_uses"`
	MaxUsesPerUser    int        `gorm:"not null;default:1" json:"max_uses_per_user"`
	UsedCount         int        `gorm:"not null;default:0" json:"used_count"`
	StartsAt          *time.Time `json:"starts_at"`
	EndsAt            *time.Time `json:"ends_at"`
	Enabled           bool       `gorm:"not null;default:true" json:"enabled"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

func (Promotion) TableName() string { return "promotions" }

// IsPromotionApplicable checks whether a promotion applies to the given variant.
// It handles all three scopes: all, template, and specific_variant.
// For scope=template, it compares the variant's TemplateID against ApplicableIDs.
// For scope=specific_variant, it compares the variant's ID against ApplicableIDs.
// Returns true if the promotion is applicable (or when ApplicableIDs is empty).
func (p Promotion) IsPromotionApplicable(variant *PlanVariant) bool {
	switch p.ApplicableScope {
	case PromoScopeAll:
		return true
	case PromoScopeTemplate:
		if len(p.ApplicableIDs) == 0 {
			return true
		}
		for _, id := range p.ApplicableIDs {
			if id == variant.TemplateID {
				return true
			}
		}
		return false
	case PromoScopeSpecificVariant:
		if len(p.ApplicableIDs) == 0 {
			return true
		}
		for _, id := range p.ApplicableIDs {
			if id == variant.ID {
				return true
			}
		}
		return false
	}
	return false
}
