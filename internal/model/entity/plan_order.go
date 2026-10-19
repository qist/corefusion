package entity

import "time"

// Order type constants.
const (
	OrderTypeNew          = "new"
	OrderTypeRenew        = "renew"
	OrderTypeUpgrade      = "upgrade"
	OrderTypeDowngrade    = "downgrade"
	OrderTypeTrialConvert = "trial_convert"
)

// Order status constants.
const (
	OrderStatusPending   = "pending"
	OrderStatusPaid      = "paid"
	// OrderStatusScheduled 表示「已付款、待生效」：套餐变更（升级/降级）已成交，
	// 但要等当前计费周期结束后才切换，由 plan 调度器在 effective_from 到达时执行。
	OrderStatusScheduled = "scheduled"
	OrderStatusCompleted = "completed"
	OrderStatusCancelled = "cancelled"
	OrderStatusRefunded  = "refunded"
)

// PlanOrder records a plan purchase/renewal/upgrade/downgrade transaction.
type PlanOrder struct {
	ID             int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	ClientID       int64      `gorm:"not null;index" json:"client_id"`
	OrderType      string     `gorm:"size:32;not null" json:"order_type"`
	FromVariantID  *int64     `json:"from_variant_id"`
	ToVariantID    int64      `gorm:"not null" json:"to_variant_id"`
	OriginalAmount int64      `gorm:"not null;default:0" json:"original_amount"`
	DiscountAmount int64      `gorm:"not null;default:0" json:"discount_amount"`
	PaidAmount     int64      `gorm:"not null;default:0" json:"paid_amount"`
	PromotionID    *int64     `json:"promotion_id"`
	PromotionCode  string     `gorm:"size:64" json:"promotion_code"`
	EffectiveFrom  time.Time  `gorm:"not null" json:"effective_from"`
	EffectiveTo    *time.Time `json:"effective_to"`
	Status         string     `gorm:"size:32;not null;default:completed" json:"status"`
	OperatorID     *int64     `json:"operator_id"`
	Remark         string     `gorm:"size:500" json:"remark"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func (PlanOrder) TableName() string { return "plan_orders" }
