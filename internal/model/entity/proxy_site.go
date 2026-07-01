package entity

import "time"

// ProxySite is the GORM model for the proxy_sites table.
type ProxySite struct {
	ID             int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	ServerID       int64     `json:"server_id" gorm:"not null;default:1;index"`
	ProxyType      string    `json:"proxy_type" gorm:"size:16;not null;default:'caddy'"`
	Domain         string    `json:"domain" gorm:"size:255;not null"`
	Upstream       string    `json:"upstream" gorm:"size:255;not null"`
	Template       string    `json:"template" gorm:"size:32"`
	Path           string    `json:"path" gorm:"size:255"`
	ListenPort     string    `json:"listen_port" gorm:"size:16"`
	ListenSocket   string    `json:"listen_socket" gorm:"size:255"`
	CertPath       string    `json:"cert_path" gorm:"size:512"`
	KeyPath        string    `json:"key_path" gorm:"size:512"`
	CustomConfig   string    `json:"custom_config" gorm:"type:text"`    // Caddy: site-level directives
	ServerConfig   string    `json:"server_config" gorm:"type:text"`    // Nginx: server block directives
	LocationConfig string    `json:"location_config" gorm:"type:text"`  // Nginx: location block directives
	L4Protocol     string    `json:"l4_protocol" gorm:"size:16"`
	SNITarget      string    `json:"sni_target" gorm:"size:255"`
	LocalPort      string    `json:"local_port" gorm:"size:16"`
	ProxyProtocol  bool      `json:"proxy_protocol" gorm:"not null;default:false"`
	Enabled        bool      `json:"enabled" gorm:"not null;default:true"`
	IsPanel        bool      `json:"is_panel" gorm:"not null;default:false"`
	Priority       int       `json:"priority" gorm:"not null;default:100"`

	// TLS options (Nginx)
	EnableHttp2    bool   `json:"enable_http2" gorm:"not null;default:true"`
	EnableQuic     bool   `json:"enable_quic" gorm:"not null;default:false"`
	EnableIpv6     bool   `json:"enable_ipv6" gorm:"not null;default:false"`
	EnableEarlyData bool  `json:"enable_early_data" gorm:"not null;default:false"`
	EnableStapling bool   `json:"enable_stapling" gorm:"not null;default:false"`
	SslProtocols   string `json:"ssl_protocols" gorm:"size:128"`
	SslCiphers     string `json:"ssl_ciphers" gorm:"size:512"`
	SslEcdhCurve   string `json:"ssl_ecdh_curve" gorm:"size:255"`

	CreatedAt      time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt      time.Time `json:"updated_at" gorm:"not null"`
}

func (ProxySite) TableName() string { return "proxy_sites" }
