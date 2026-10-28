package entity

import "time"

// RuleSource is the GORM model for the rule_sources table.
type RuleSource struct {
	ID       int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	ServerID int64  `json:"server_id" gorm:"not null;default:0;index"` // 0=global, shared by all nodes
	Name     string `json:"name" gorm:"size:128;not null"`
	RepoURL  string `json:"repo_url" gorm:"size:512;not null"` // e.g. "Loyalsoldier/v2ray-rules-dat"
	Enabled  bool   `json:"enabled" gorm:"not null;default:false"`
	SyncCron string `json:"sync_cron" gorm:"size:64"` // e.g. "0 3 * * *" (daily 3am)
	// LastSyncAt 用指针：从未同步时是 NULL。
	// 非指针 time.Time 的零值会被 GORM 写成 '0000-00-00'，MySQL 8 严格模式
	// 直接报 1292（Incorrect datetime value），导致默认规则源播种失败。
	LastSyncAt  *time.Time `json:"last_sync_at"`
	LastSyncTag string     `json:"last_sync_tag" gorm:"size:128"` // last release tag
	Status      string     `json:"status" gorm:"size:32;not null;default:'idle'"`
	CreatedAt   time.Time  `json:"created_at" gorm:"not null"`
	UpdatedAt   time.Time  `json:"updated_at" gorm:"not null"`
}

func (RuleSource) TableName() string { return "rule_sources" }

// GeoRuleFile is the GORM model for the geo_rule_files table.
type GeoRuleFile struct {
	ID          int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	ServerID    int64  `json:"server_id" gorm:"not null;default:0;index"` // 0=global, shared by all nodes
	SourceID    int64  `json:"source_id" gorm:"index;not null;default:0"`
	Name        string `json:"name" gorm:"size:255;not null"`
	FileName    string `json:"file_name" gorm:"size:255;not null"`
	DownloadURL string `json:"download_url" gorm:"size:1024;not null"`
	FileSize    int64  `json:"file_size" gorm:"not null;default:0"`
	Version     string `json:"version" gorm:"size:128"` // release tag
	// LastUpdated 同样用指针：播种默认文件时从未下载过，应为 NULL，
	// 否则 MySQL 严格模式会拒绝零值时间。
	LastUpdated *time.Time `json:"last_updated"`
	AutoSync    bool       `json:"auto_sync" gorm:"not null;default:true"` // include in scheduled sync
	Status      string     `json:"status" gorm:"size:32;not null;default:'available'"`
	CreatedAt   time.Time  `json:"created_at" gorm:"not null"`
	UpdatedAt   time.Time  `json:"updated_at" gorm:"not null"`
}

func (GeoRuleFile) TableName() string { return "geo_rule_files" }
