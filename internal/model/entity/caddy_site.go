package entity

import "time"

// CaddySite is the GORM model for the caddy_sites table.
type CaddySite struct {
	ID           int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	ServerID     int64  `json:"server_id" gorm:"not null;default:1;index"`
	Domain       string `json:"domain" gorm:"size:255;not null"`
	Upstream     string `json:"upstream" gorm:"size:255;not null"`
	Template     string `json:"template" gorm:"size:32"`
	Path         string `json:"path" gorm:"size:255"`
	BackendPath  string `json:"backend_path" gorm:"size:255"`
	CertPath     string `json:"cert_path" gorm:"size:512"`
	KeyPath      string `json:"key_path" gorm:"size:512"`
	ProxyConfig  string `json:"proxy_config" gorm:"type:longtext"`  // inside reverse_proxy { }
	CustomConfig string `json:"custom_config" gorm:"type:longtext"` // outside reverse_proxy (e.g. encode gzip)
	WafEnabled   *bool  `json:"waf_enabled" gorm:"default:true"`    // per-site WAF switch (nil=true=inherit, false=disable WAF for this site's path)
	// DualCert：是否把**同域名的多张证书**（ECC + RSA）一起挂到本站点。
	// nil = true = 开（默认 ✓，与 WafEnabled 同范式 ✓）；显式 false = 只用本站点自己那对。
	// 用途：同域名签了 ECC+RSA 时，个别站点可能只想挂其中一张 ✓。
	DualCert   *bool  `json:"dual_cert" gorm:"default:true"`
	LbPolicy   string `json:"lb_policy" gorm:"size:32"` // round_robin, least_conn, etc.
	LogEnabled bool   `json:"log_enabled" gorm:"not null;default:false"`
	LogPath    string `json:"log_path" gorm:"size:255"`  // e.g. logs/example.com.log
	LogFormat  string `json:"log_format" gorm:"size:32"` // json, console
	L4Protocol string `json:"l4_protocol" gorm:"size:16"`
	SNITarget  string `json:"sni_target" gorm:"size:255"`
	// UpstreamTLS 表示"**回源这一段**（nginx/caddy → 本地内核）要不要走 TLS" ✓。
	// grpc 原来写死 `grpc_pass grpc://`（h2c 明文）✗；入站自带 TLS 时必须 `grpcs://` ✗。
	// 「从入站一键导入」按入站 security 自动置位 ✓，表单可手改 ✓。
	UpstreamTLS   bool      `json:"upstream_tls" gorm:"not null;default:false"`
	LocalPort     string    `json:"local_port" gorm:"size:16"`
	ProxyProtocol bool      `json:"proxy_protocol" gorm:"not null;default:false"`
	Enabled       bool      `json:"enabled" gorm:"not null;default:true"`
	IsPanel       bool      `json:"is_panel" gorm:"not null;default:false"`
	Priority      int       `json:"priority" gorm:"not null;default:100"`
	CreatedAt     time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt     time.Time `json:"updated_at" gorm:"not null"`
}

func (CaddySite) TableName() string { return "caddy_sites" }
