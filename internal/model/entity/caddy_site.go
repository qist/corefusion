package entity

import "time"

// CaddySite is the GORM model for the caddy_sites table.
type CaddySite struct {
	ID            int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	ServerID      int64     `json:"server_id" gorm:"not null;default:1;index"`
	Domain        string    `json:"domain" gorm:"size:255;not null"`
	Upstream      string    `json:"upstream" gorm:"size:255;not null"`
	Template      string    `json:"template" gorm:"size:32"`
	Path          string    `json:"path" gorm:"size:255"`
	CertPath      string    `json:"cert_path" gorm:"size:512"`
	KeyPath       string    `json:"key_path" gorm:"size:512"`
	ProxyConfig   string    `json:"proxy_config" gorm:"type:text"`   // inside reverse_proxy { }
	CustomConfig  string    `json:"custom_config" gorm:"type:text"`  // outside reverse_proxy (e.g. encode gzip)
	LbPolicy      string    `json:"lb_policy" gorm:"size:32"`        // round_robin, least_conn, etc.
	LogEnabled    bool      `json:"log_enabled" gorm:"not null;default:false"`
	LogPath       string    `json:"log_path" gorm:"size:255"`        // e.g. logs/example.com.log
	LogFormat     string    `json:"log_format" gorm:"size:32"`       // json, console
	L4Protocol    string    `json:"l4_protocol" gorm:"size:16"`
	SNITarget     string    `json:"sni_target" gorm:"size:255"`
	LocalPort     string    `json:"local_port" gorm:"size:16"`
	ProxyProtocol bool      `json:"proxy_protocol" gorm:"not null;default:false"`
	Enabled       bool      `json:"enabled" gorm:"not null;default:true"`
	IsPanel       bool      `json:"is_panel" gorm:"not null;default:false"`
	Priority      int       `json:"priority" gorm:"not null;default:100"`
	CreatedAt     time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt     time.Time `json:"updated_at" gorm:"not null"`
}

func (CaddySite) TableName() string { return "caddy_sites" }
