package entity

import "time"

// ACMEAccountConfig is the GORM model for the acme_account_configs table.
type ACMEAccountConfig struct {
	ID        int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	Email     string `json:"email" gorm:"size:255;not null"`
	IsDefault bool   `json:"is_default" gorm:"not null;default:false"`
	Status    string `json:"status" gorm:"size:32;not null;default:'active'"`

	// CAProvider 这张账号用于哪个 CA ✓（lets_encrypt / lets_encrypt_staging / zerossl / google ✓）。
	//
	// ⚠ 两条约束都要避开 ✗（都踩过 ✓）：
	//   1. 不加 `default:` ✗ —— GORM 对"带默认值的字段"会**跳过零值** ✗（AutoRenew 上踩过 ✓）；
	//   2. 不加 `not null` ✗ —— 给**已有表**新增 NOT NULL 列（无默认值）SQLite 会直接拒绝 ✗ →
	//      AutoMigrate 失败 → **面板起不来** ✗（真机踩到 ✓）。写入侧由 normalizeCAProvider 保证非空 ✓。
	CAProvider string `json:"ca_provider" gorm:"size:32"`
	// EABKeyID / EABMACKey 是**外部账号绑定**（EAB ✓）：ZeroSSL、Google Public CA **必填** ✗。
	// 存在账号里 → 签发时按所选账号自动带上 ✓，不用每次手填 ✗（用户要的"做个配置" ✓）。
	//
	// ⚠ MAC Key 是**密钥** ✗ → `json:"-"` 永不外泄 ✓（前端只看 has_eab ✓）。
	EABKeyID  string `json:"eab_key_id" gorm:"size:255"`
	EABMACKey string `json:"-" gorm:"size:1024"`
	// HasEAB 只用于回给前端"这个账号配过 EAB 没有" ✓（不落库 ✓）。
	HasEAB bool `json:"has_eab" gorm:"-"`

	CreatedAt time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt time.Time `json:"updated_at" gorm:"not null"`
}

func (ACMEAccountConfig) TableName() string { return "acme_account_configs" }
