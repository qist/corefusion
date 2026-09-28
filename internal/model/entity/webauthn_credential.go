package entity

import "time"

// WebAuthnCredential 是用户注册的 Passkey（WebAuthn 凭证）。
//
// 设计要点（与 go-webauthn 官方 Storage 指南对齐）：
//   - CredentialID = base64url(rawID)：登录时按 (RPID, CredentialID) 定位记录；
//   - CredentialJSON = 完整 webauthn.Credential 的 JSON 序列化（含公钥、attestation 原始数据），
//     每次成功 FinishLogin 后要**整体回写**（SignCount / 克隆告警 / flags 会随断言更新）；
//   - UserHandle（webauthn_handles 语义）挂在 users.webauthn_handle 上，由 User.WebAuthnHandle 承载，
//     不再单独建 webauthn_users 表 —— 同一用户在所有 RP ID 下共用同一 handle，凭证仍按 RPID 分区。
type WebAuthnCredential struct {
	ID             int64      `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID         int64      `json:"user_id" gorm:"not null;index:idx_webauthn_user_rpid"`
	RPID           string     `json:"rpid" gorm:"size:512;not null;index:idx_webauthn_user_rpid;uniqueIndex:idx_webauthn_kid"`
	CredentialID   string     `json:"credential_id" gorm:"size:1024;not null;uniqueIndex:idx_webauthn_kid"` // base64url(rawID)
	CredentialJSON string     `json:"-" gorm:"type:text;not null"`                                          // 完整 webauthn.Credential 序列化
	SignCount      uint32     `json:"sign_count" gorm:"not null;default:0"`
	Transport      string     `json:"transport" gorm:"size:256"` // 逗号分隔的认证器传输类型
	Attachment     string     `json:"attachment" gorm:"size:64"` // platform / cross-platform
	Name           string     `json:"name" gorm:"size:128"`      // 用户自定义名称（如"我的 iPhone"）
	CreatedAt      time.Time  `json:"created_at" gorm:"not null"`
	UpdatedAt      time.Time  `json:"updated_at" gorm:"not null"`
	LastUsedAt     *time.Time `json:"last_used_at"`
}

func (WebAuthnCredential) TableName() string { return "webauthn_credentials" }
