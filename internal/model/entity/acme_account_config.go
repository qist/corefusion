package entity

import "time"

// ACMEAccountConfig is the GORM model for the acme_account_configs table.
type ACMEAccountConfig struct {
	ID        int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Email     string    `json:"email" gorm:"size:255;not null"`
	IsDefault bool      `json:"is_default" gorm:"not null;default:false"`
	Status    string    `json:"status" gorm:"size:32;not null;default:'active'"`
	CreatedAt time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt time.Time `json:"updated_at" gorm:"not null"`
}

func (ACMEAccountConfig) TableName() string { return "acme_account_configs" }
