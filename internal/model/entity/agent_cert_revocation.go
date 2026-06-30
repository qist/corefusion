package entity

import "time"

// CertRevocationReason constants.
const (
	RevocationReasonCompromised   = "compromised"    // 证书泄露
	RevocationReasonDecommissioned = "decommissioned" // 服务器下线
	RevocationReasonReplaced      = "replaced"        // 证书被新证书替换
)

// AgentCertRevocation records a revoked Agent mTLS certificate.
// Used to build the CRL (Certificate Revocation List) checked during mTLS handshake.
type AgentCertRevocation struct {
	ID          int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	ServerID    int64     `json:"server_id" gorm:"not null;index"`
	CertSerial  string    `json:"cert_serial" gorm:"size:128;not null;index"`
	RevokedAt   time.Time `json:"revoked_at" gorm:"not null"`
	RevokedBy   int64     `json:"revoked_by" gorm:"not null"` // admin user ID
	Reason      string    `json:"reason" gorm:"size:64;not null;default:''"`
	EffectiveAt time.Time `json:"effective_at" gorm:"not null"` // can be delayed
	CreatedAt   time.Time `json:"created_at" gorm:"not null"`
}

func (AgentCertRevocation) TableName() string { return "agent_cert_revocations" }
