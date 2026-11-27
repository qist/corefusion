package entity

import "time"

// SubscriptionShortLinkRevocation 记录已撤销短码的 HMAC 指纹。
//
// 短链删除/轮换后，原 Code 只留指纹、不留明文；唯一索引保证同一个 Code
// 不会在撤销后被重新签发。短码查询本身走 SubscriptionShortLink.CodeHMAC，
// 这张表只用于"撤销后不得复用"的判定。
type SubscriptionShortLinkRevocation struct {
	ID        int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	CodeHMAC  string    `json:"-" gorm:"size:64;uniqueIndex;not null"`
	Reason    string    `json:"reason" gorm:"size:64"`
	RevokedBy int64     `json:"revoked_by" gorm:"not null;default:0"`
	RevokedAt time.Time `json:"revoked_at" gorm:"not null"`
}

func (SubscriptionShortLinkRevocation) TableName() string {
	return "subscription_short_link_revocations"
}
