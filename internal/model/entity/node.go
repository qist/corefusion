package entity

import "time"

// Node is the GORM model for the nodes table.
type Node struct {
	ID            int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Name          string    `json:"name" gorm:"size:128;not null"`
	CoreType      string    `json:"core_type" gorm:"size:32;not null"`
	Protocol      string    `json:"protocol" gorm:"size:32;not null"`
	Host          string    `json:"host" gorm:"size:255;not null"`
	Port          int       `json:"port" gorm:"not null"`
	Transport     string    `json:"transport" gorm:"size:32"`
	TLSMode       string    `json:"tls_mode" gorm:"size:32"`
	CredentialRef string    `json:"credential_ref" gorm:"size:255"`
	Region        string    `json:"region" gorm:"size:64"`
	Tags          string    `json:"tags" gorm:"size:512"`
	Status        int       `json:"status" gorm:"not null;default:1"`
	CreatedAt     time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt     time.Time `json:"updated_at" gorm:"not null"`
}

func (Node) TableName() string { return "nodes" }
