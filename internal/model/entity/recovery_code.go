package entity

import "time"

// RecoveryCode 是 2FA（TOTP）的备用恢复码。
//
// 设计要点：
//   - 只在"启用 2FA / 重新生成恢复码"时一次性下发明文；
//   - 数据库只存 sha256(明文) 的 hex —— 丢失明文即彻底不可用，库被拖走也无法反推；
//   - 每码单次使用（UsedAt 置位），登录时校验通过即标记；
//   - 删除用户时随用户级联清理（调用方负责按 UserID 删除）。
type RecoveryCode struct {
	ID        int64      `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID    int64      `json:"user_id" gorm:"not null;index:idx_recovery_user"`
	CodeHash  string     `json:"-" gorm:"size:64;not null"` // sha256 hex（不含盐：恢复码本身已是 128bit 高熵，暴力不可行）
	UsedAt    *time.Time `json:"used_at"`
	CreatedAt time.Time  `json:"created_at" gorm:"not null"`
	UpdatedAt time.Time  `json:"updated_at" gorm:"not null"`
}

func (RecoveryCode) TableName() string { return "recovery_codes" }
