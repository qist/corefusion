package entity

import "time"

// ServerTag is a key-value label attached to a node server for filtering and grouping.
type ServerTag struct {
	ID        int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	ServerID  int64     `json:"server_id" gorm:"not null;index;uniqueIndex:idx_server_tag_key"`
	TagKey    string    `json:"tag_key" gorm:"size:64;not null;uniqueIndex:idx_server_tag_key"`
	TagValue  string    `json:"tag_value" gorm:"size:256;not null;default:''"`
	CreatedAt time.Time `json:"created_at" gorm:"not null"`
}

func (ServerTag) TableName() string { return "server_tags" }
