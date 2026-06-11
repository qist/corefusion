package entity

import "time"

// RuleSource is the GORM model for the rule_sources table.
type RuleSource struct {
	ID           int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Name         string    `json:"name" gorm:"size:128;not null"`
	RepoURL      string    `json:"repo_url" gorm:"size:512;not null"` // e.g. "Loyalsoldier/v2ray-rules-dat"
	Enabled      bool      `json:"enabled" gorm:"not null;default:false"`
	SyncCron     string    `json:"sync_cron" gorm:"size:64"`          // e.g. "0 3 * * *" (daily 3am)
	LastSyncAt   time.Time `json:"last_sync_at"`
	LastSyncTag  string    `json:"last_sync_tag" gorm:"size:128"`     // last release tag
	Status       string    `json:"status" gorm:"size:32;not null;default:'idle'"`
	CreatedAt    time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt    time.Time `json:"updated_at" gorm:"not null"`
}

func (RuleSource) TableName() string { return "rule_sources" }

// GeoRuleFile is the GORM model for the geo_rule_files table.
type GeoRuleFile struct {
	ID          int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	SourceID    int64     `json:"source_id" gorm:"index;not null;default:0"`
	Name        string    `json:"name" gorm:"size:255;not null"`
	FileName    string    `json:"file_name" gorm:"size:255;not null"`
	DownloadURL string    `json:"download_url" gorm:"size:1024;not null"`
	FileSize    int64     `json:"file_size" gorm:"not null;default:0"`
	Version     string    `json:"version" gorm:"size:128"` // release tag
	LastUpdated time.Time `json:"last_updated"`
	AutoSync    bool      `json:"auto_sync" gorm:"not null;default:true"` // include in scheduled sync
	Status      string    `json:"status" gorm:"size:32;not null;default:'available'"`
	CreatedAt   time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"not null"`
}

func (GeoRuleFile) TableName() string { return "geo_rule_files" }
