package entity

import "time"

// Certificate is the GORM model for the certificates table.
type Certificate struct {
	ID            int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	Name          string `json:"name" gorm:"size:255;not null"`
	Domain        string `json:"domain" gorm:"size:255;not null"`
	Domains       string `json:"domains" gorm:"type:longtext"`
	Issuer        string `json:"issuer" gorm:"size:128;not null"`
	ProviderType  string `json:"provider_type" gorm:"size:32;not null"`
	KeyType       string `json:"key_type" gorm:"size:16"`
	KeyBits       int    `json:"key_bits"`
	CertPath      string `json:"cert_path" gorm:"size:512;not null"`
	KeyPath       string `json:"key_path" gorm:"size:512;not null"`
	FullchainPath string `json:"fullchain_path" gorm:"size:512"`
	CertContent   string `json:"cert_content" gorm:"type:longtext"`
	KeyContent    string `json:"key_content" gorm:"type:longtext"`
	Status        string `json:"status" gorm:"size:32;not null"`
	// ⚠ 不要加 `default:1` ✗：GORM 对"带 default 的字段"会**跳过零值** ✗ →
	// 代码里显式设成 0（例如上传的证书不该被自动续期）会被 DB 默认值改回 1 ✗
	// （真机实测：上传路径设 0，库里却是 1 ✗）。去掉 default 后零值能正常写入 ✓。
	AutoRenew     int        `json:"auto_renew" gorm:"not null"`
	ExpiresAt     time.Time  `json:"expires_at" gorm:"not null"`
	LastRenewedAt *time.Time `json:"last_renewed_at"`
	CreatedAt     time.Time  `json:"created_at" gorm:"not null"`
	UpdatedAt     time.Time  `json:"updated_at" gorm:"not null"`
}

func (Certificate) TableName() string { return "certificates" }
