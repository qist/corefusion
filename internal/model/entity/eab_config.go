package entity

import "time"

// EABConfig 是**独立管理**的 EAB（外部账号绑定）配置 ✓ ——
// 和"ACME 邮箱""DNS 通道"一样是单独的配置项 ✗，签发时只需**选一下关联** ✓，
// 不用在签发弹窗里每次手填 EAB-KID / EAB-HMAC-Key ✗。
//
// 哪些 CA 需要 EAB ✓：Google Public CA ✓、ZeroSSL ✓（Let's Encrypt 不需要 ✓）。
type EABConfig struct {
	ID int64 `json:"id" gorm:"primaryKey;autoIncrement"`
	// Name 是给人看的名字 ✓（例如 "Google CA - example-user" ✓）
	Name string `json:"name" gorm:"size:128;not null"`
	// CAProvider 这份 EAB 属于哪个 CA ✓（google / zerossl ✓ …）
	// ⚠ 不加 `default:` ✗（GORM 会跳过零值 ✗）、不加 `not null` ✗（给已有的表加 NOT NULL 列 SQLite 会拒绝 ✗ → 面板起不来 ✗）
	CAProvider string `json:"ca_provider" gorm:"size:32"`
	// KeyID 即 EAB-KID ✓
	KeyID string `json:"eab_key_id" gorm:"size:255"`
	// MACKey 即 EAB-HMAC-Key ✓ —— 是**密钥** ✗ → `json:"-"` 永不外泄 ✓（前端只看 has_mac_key ✓）
	MACKey string `json:"-" gorm:"size:1024"`
	// HasMACKey 只用于回给前端"这份配置填过 HMAC-Key 没有" ✓（不落库 ✓）
	HasMACKey bool      `json:"has_mac_key" gorm:"-"`
	Status    string    `json:"status" gorm:"size:32;not null;default:'active'"`
	CreatedAt time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt time.Time `json:"updated_at" gorm:"not null"`
}

func (EABConfig) TableName() string { return "eab_configs" }
