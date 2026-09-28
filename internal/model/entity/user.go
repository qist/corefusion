package entity

import "time"

// User is the GORM model for the users table.
type User struct {
	ID           int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	Username     string `json:"username" gorm:"size:64;uniqueIndex;not null"`
	Email        string `json:"email" gorm:"size:255"`
	PasswordHash string `json:"password_hash" gorm:"size:255;not null"`
	Role         string `json:"role" gorm:"size:32;not null;default:viewer"`
	// TokenVersion：登出/吊销时 +1，使此前签发的所有 token 立即失效（见 middleware.AuthRequired）。
	// 默认 0，历史 token 缺该声明时也是 0 → 升级不踢人。
	TokenVersion int `json:"token_version" gorm:"not null;default:0"`
	Status       int `json:"status" gorm:"not null;default:1"`
	// WebAuthnHandle：该用户的 WebAuthn User Handle（base64url，随机 32 字节）。
	// 对同一用户**全局稳定**（所有 RP ID 共用），用于 Passkey discoverable 登录时
	// 由认证器返回的 userHandle 反查用户。首次注册 Passkey 时懒生成。
	WebAuthnHandle string `json:"webauthn_handle,omitempty" gorm:"size:128;index"`
	// TOTPSecret：2FA（TOTP）密钥，AES-256-GCM 加密落库（crypto.EncryptSecret 的 "enc:" 格式）。
	// 绝不通过 API 输出；绑定流程只在生成后一次性下发 otpauth URI。
	TOTPSecret string `json:"-" gorm:"size:512"`
	// TOTPEnabled：是否已启用 2FA。启用后密码登录需额外验证 TOTP 动态码或恢复码。
	TOTPEnabled bool      `json:"totp_enabled" gorm:"not null;default:false"`
	CreatedAt   time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"not null"`
}

func (User) TableName() string { return "users" }
