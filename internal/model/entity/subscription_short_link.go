package entity

import "time"

// SubscriptionShortLink 是订阅短链。
//
// 短链只保存「订阅 ID + 预设的渲染参数」，不保存节点快照；每次请求都按订阅
// 当前状态实时渲染，因此订阅被吊销/暂停/到期时短链自动失效（与长链接同一套
// 授权判断）。
//
// Code 是对外的随机路径段（不含任何语义前缀），与 64 位 hex token 通过长度区分。
type SubscriptionShortLink struct {
	ID             int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	SubscriptionID int64  `json:"subscription_id" gorm:"index;not null"`
	Code           string `json:"code" gorm:"size:32;uniqueIndex;not null"`

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
}

func (SubscriptionShortLink) TableName() string { return "subscription_short_links" }
