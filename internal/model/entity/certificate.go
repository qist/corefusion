package entity

import "time"

// Certificate is the GORM model for the certificates table.
type Certificate struct {
	ID             int64      `json:"id" gorm:"primaryKey;autoIncrement"`
	Name           string     `json:"name" gorm:"size:255;not null"`
	Domain         string     `json:"domain" gorm:"size:255;not null"`
	Domains        string     `json:"domains" gorm:"type:text"`
	Issuer         string     `json:"issuer" gorm:"size:128;not null"`
	ProviderType   string     `json:"provider_type" gorm:"size:32;not null"`
	KeyType        string     `json:"key_type" gorm:"size:16"`
	KeyBits        int        `json:"key_bits"`
	CertPath       string     `json:"cert_path" gorm:"size:512;not null"`
	KeyPath        string     `json:"key_path" gorm:"size:512;not null"`
	FullchainPath  string     `json:"fullchain_path" gorm:"size:512"`
	CertContent    string     `json:"cert_content" gorm:"type:text"`
	KeyContent     string     `json:"key_content" gorm:"type:text"`
	Status         string     `json:"status" gorm:"size:32;not null"`
	AutoRenew      int        `json:"auto_renew" gorm:"not null;default:1"`
	ExpiresAt      time.Time  `json:"expires_at" gorm:"not null"`
	LastRenewedAt  *time.Time `json:"last_renewed_at"`
	CreatedAt      time.Time  `json:"created_at" gorm:"not null"`
	UpdatedAt      time.Time  `json:"updated_at" gorm:"not null"`
}

func (Certificate) TableName() string { return "certificates" }
