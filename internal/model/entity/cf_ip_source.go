package entity

import "time"

// CFIPSource stores CF IP API source configuration.
type CFIPSource struct {
	ID        int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Name      string    `json:"name" gorm:"size:128;not null"`
	URL       string    `json:"url" gorm:"size:512;not null"`
	Type      string    `json:"type" gorm:"size:32;not null"`          // bestcf, bestproxy, custom
	Ports     string    `json:"ports" gorm:"type:longtext"`            // JSON array
	Interval  int       `json:"interval" gorm:"not null;default:3600"` // seconds
	Enabled   bool      `json:"enabled" gorm:"not null;default:true"`
	LastRun   time.Time `json:"last_run"`
	CreatedAt time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt time.Time `json:"updated_at" gorm:"not null"`
}

func (CFIPSource) TableName() string { return "cf_ip_sources" }

// CFIPEntry stores fetched CF IP entries.
type CFIPEntry struct {
	ID        int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	SourceID  int64     `json:"source_id" gorm:"index"`
	IP        string    `json:"ip" gorm:"size:64;not null"`
	Country   string    `json:"country" gorm:"size:16"`
	Ports     string    `json:"ports" gorm:"type:longtext"` // JSON array
	Enabled   bool      `json:"enabled" gorm:"not null;default:true"`
	CreatedAt time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt time.Time `json:"updated_at" gorm:"not null"`
}

func (CFIPEntry) TableName() string { return "cf_ip_entries" }
