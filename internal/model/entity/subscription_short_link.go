package entity

import (
	"log"
	"time"

	"gorm.io/gorm"

	"corefusion/internal/security/crypto"
)

// SubscriptionShortLink 是订阅短链。
//
// 短链只保存「订阅 ID + 预设的渲染参数」，不保存节点快照；每次请求都按订阅
// 当前状态实时渲染，因此订阅被吊销/暂停/到期时短链自动失效（与长链接同一套
// 授权判断）。
//
// Code 是对外的随机路径段（不含任何语义前缀），与 64 位 hex token 通过长度区分。
type SubscriptionShortLink struct {
	ID             int64 `json:"id" gorm:"primaryKey;autoIncrement"`
	SubscriptionID int64 `json:"subscription_id" gorm:"index;not null"`
	// Code 以随机 AES-256-GCM 密文落库（用于管理端/门户回显完整短链）。
	Code string `json:"code" gorm:"size:255;not null"`
	// CodeHMAC 是 Code 的 HMAC-SHA256 指纹（64 位 hex），只用于等值查询/唯一约束；
	// 不返回给前端，也不可逆。查询走这一列，不按 Code 密文查。
	// 允许 NULL：存量表加列时旧行为 NULL，唯一索引不会因空串冲突；迁移会回填。
	CodeHMAC string `json:"-" gorm:"size:64;uniqueIndex"`

	// 预设渲染参数；空值表示跟随订阅默认。
	Format         string `json:"format" gorm:"size:32"`
	MihomoConfigID *int64 `json:"mihomo_config_id" gorm:"index"`
	ConfigName     string `json:"config_name" gorm:"size:128"`
	Platform       string `json:"platform" gorm:"size:64"`
	Providers      bool   `json:"providers" gorm:"not null;default:false"`
	LockParams     bool   `json:"lock_params" gorm:"not null;default:false"`

	// 生命周期。Enabled=false / RevokedAt!=nil / ExpiresAt 已过 都返回 404。
	Enabled   bool       `json:"enabled" gorm:"not null;default:true"`
	ExpiresAt *time.Time `json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at"`

	// 与订阅 token 轮换的联动：
	//   - FollowTokenRotation=false（默认）：创建时快照 TokenVersion，轮换后快照不匹配 → 短链失效；
	//   - FollowTokenRotation=true：轮换时同步更新快照，短链长期有效。
	TokenVersion        int  `json:"-" gorm:"not null;default:1"`
	FollowTokenRotation bool `json:"follow_token_rotation" gorm:"not null;default:false"`

	// 访问统计。
	LastAccessAt *time.Time `json:"last_access_at"`
	AccessCount  int64      `json:"access_count" gorm:"not null;default:0"`

	CreatedBy int64     `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// codePlain 仅在 BeforeSave → AfterSave 之间暂存明文，保证落库是密文、
	// 返回给调用方的仍是可直接拼 URL 的明文 code。
	codePlain string `gorm:"-" json:"-"`
}

func (SubscriptionShortLink) TableName() string { return "subscription_short_links" }

// BeforeSave 落库前：Code 用随机 nonce 加密（可回显），CodeHMAC 存单向指纹（用于查询）。
func (l *SubscriptionShortLink) BeforeSave(*gorm.DB) error {
	if l.Code == "" || crypto.IsEncrypted(l.Code) {
		return nil
	}
	fp, err := crypto.FingerprintSecret(l.Code)
	if err != nil {
		return err
	}
	enc, err := crypto.EncryptSecret(l.Code)
	if err != nil {
		return err
	}
	l.CodeHMAC = fp
	l.codePlain = l.Code
	l.Code = enc
	return nil
}

func (l *SubscriptionShortLink) AfterSave(*gorm.DB) error {
	if l.codePlain != "" {
		l.Code = l.codePlain
		l.codePlain = ""
	}
	return nil
}

// AfterFind 把库里的密文短码解回明文；存量明文原样返回。
func (l *SubscriptionShortLink) AfterFind(*gorm.DB) error {
	if !crypto.IsEncrypted(l.Code) {
		return nil
	}
	plain, err := crypto.DecryptSecret(l.Code)
	if err != nil {
		log.Printf("[entity] decrypt short link code failed: %v", err)
		return nil
	}
	l.Code = plain
	return nil
}
