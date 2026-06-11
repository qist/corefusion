package entity

import "time"

// WarpConfig is the GORM model for the warp_configs table.
type WarpConfig struct {
	ID             int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	DeviceID       string    `json:"device_id" gorm:"size:255"`
	Token          string    `json:"token" gorm:"size:512"`
	PrivateKey     string    `json:"private_key" gorm:"size:255"`
	PublicKey      string    `json:"public_key" gorm:"size:255"`
	Reserved       string    `json:"reserved" gorm:"size:128"`
	AddressIPv4    string    `json:"address_ipv4" gorm:"size:64"`
	AddressIPv6    string    `json:"address_ipv6" gorm:"size:128"`
	Endpoint       string    `json:"endpoint" gorm:"size:255"`
	ClientID       string    `json:"client_id" gorm:"size:512"`
	AccountID      string    `json:"account_id" gorm:"size:255"`
	AccountType    string    `json:"account_type" gorm:"size:64"`
	LicenseKey     string    `json:"license_key" gorm:"size:255"`
	RegistrationAt time.Time `json:"registration_at"`
	Status         string    `json:"status" gorm:"size:32;not null;default:'not_registered'"`
	ConfigJSON     string    `json:"config_json" gorm:"type:text"`
	CreatedAt      time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt      time.Time `json:"updated_at" gorm:"not null"`
}

func (WarpConfig) TableName() string { return "warp_configs" }
