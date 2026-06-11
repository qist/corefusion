package entity

import "time"

// User is the GORM model for the users table.
type User struct {
	ID int64 `json:"id" gorm:"primaryKey;autoIncrement"`
	Username string `json:"username" gorm:"size:64;uniqueIndex;not null"`
	Email string `json:"email" gorm:"size:255"`
	PasswordHash string `json:"password_hash" gorm:"size:255;not null"`
	Role string `json:"role" gorm:"size:32;not null;default:viewer"`
	Status int `json:"status" gorm:"not null;default:1"`
	CreatedAt    time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt     time.Time `json:"updated_at" gorm:"not null"`
}

func (User) TableName() string { return "users" }
