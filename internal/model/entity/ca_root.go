package entity

import "time"

// CARoot stores the panel's internal CA root certificate used for Agent mTLS.
//
// The CA private key is encrypted at rest (AES-256-GCM via crypto.EncryptSecret)
// and stored in KeyPEMEnc. The plaintext key is never persisted to the database.
// On startup the CA is loaded from the DB, decrypted, and the PEM files are
// materialized to disk under dataDir/certs/mtls/ so Agents can download the CA
// cert during bootstrap.
type CARoot struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	CommonName string    `gorm:"column:common_name;type:varchar(255);not null" json:"common_name"`
	CertPEM    string    `gorm:"column:cert_pem;type:longtext;not null" json:"cert_pem"` // CA root cert (plaintext PEM)
	KeyPEMEnc  string    `gorm:"column:key_pem_enc;type:longtext;not null" json:"-"`     // CA private key (encrypted via crypto.EncryptSecret)
	ExpiresAt  time.Time `gorm:"column:expires_at;not null" json:"expires_at"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
}

// TableName specifies the database table name for CARoot.
func (CARoot) TableName() string {
	return "ca_roots"
}
