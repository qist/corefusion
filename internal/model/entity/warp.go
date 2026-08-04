package entity

import "time"

// WarpConfig is the GORM model for the warp_configs table.
// Each WARP config is scoped to a specific node server and optionally
// to a specific core type (e.g. "xray", "singbox"), so that different
// cores on the same node can use independent WARP exits.
type WarpConfig struct {
	ID             int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	ServerID       int64     `json:"server_id" gorm:"not null;default:1;index:idx_warp_server_core"`
	CoreType       string    `json:"core_type" gorm:"size:32;not null;default:'';index:idx_warp_server_core"`
	Remark         string    `json:"remark" gorm:"size:128"`
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
	ConfigJSON     string    `json:"config_json" gorm:"type:longtext"`
	CreatedAt      time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt      time.Time `json:"updated_at" gorm:"not null"`
}

func (WarpConfig) TableName() string { return "warp_configs" }
