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
	TokenVersion int       `json:"token_version" gorm:"not null;default:0"`
	Status       int       `json:"status" gorm:"not null;default:1"`
	CreatedAt    time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt    time.Time `json:"updated_at" gorm:"not null"`
}

func (User) TableName() string { return "users" }
