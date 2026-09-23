package entity

import "time"

// ProxySite is the GORM model for the proxy_sites table.
type ProxySite struct {
	ID             int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	ServerID       int64  `json:"server_id" gorm:"not null;default:1;index"`
	ProxyType      string `json:"proxy_type" gorm:"size:16;not null;default:'caddy'"`
	Domain         string `json:"domain" gorm:"size:255;not null"`
	Upstream       string `json:"upstream" gorm:"size:255;not null"`
	Template       string `json:"template" gorm:"size:32"`
	Path           string `json:"path" gorm:"size:255"`
	ListenPort     string `json:"listen_port" gorm:"size:16"`
	ListenSocket   string `json:"listen_socket" gorm:"size:255"`
	CertPath       string `json:"cert_path" gorm:"size:512"`
	KeyPath        string `json:"key_path" gorm:"size:512"`
	CustomConfig   string `json:"custom_config" gorm:"type:longtext"`   // Caddy: site-level directives
	ServerConfig   string `json:"server_config" gorm:"type:longtext"`   // Nginx: server block directives
	LocationConfig string `json:"location_config" gorm:"type:longtext"` // Nginx: location block directives
	L4Protocol     string `json:"l4_protocol" gorm:"size:16"`
	SNITarget      string `json:"sni_target" gorm:"size:255"`
	// UpstreamTLS 表示"**回源这一段**要不要走 TLS" ✓ —— 即 nginx/caddy → 本地内核 的连接。
	//
	// 为什么必须有它 ✗（真机场景 ✓）：grpc 转发原来**写死** `grpc_pass grpc://`
	//（= h2c 明文 ✓）。但入站若自带 TLS（xray `security: tls` / sing-box `tls.enabled=true` ✓），
	// 核心那个端口是在说 TLS ✗ → 必须 `grpcs://` + `grpc_ssl_*` ✓，否则握手失败 ✗
	//（现象就是"配置看着对、就是不通" ✗）。ws/http 同理：`http://` ↔ `https://` ✓。
	//
	// 「从入站一键导入」时按入站 security **自动置位** ✓，站点表单里也可以手动改 ✓。
	// ⚠ `not null;default:false` 在这里是安全的 ✓（false 就是零值 ✓，不会像 AutoRenew 那样
	//   被 DB 默认值把"显式写入的 0"改回 1 ✗）。
	UpstreamTLS   bool   `json:"upstream_tls" gorm:"not null;default:false"`
	LocalPort     string `json:"local_port" gorm:"size:16"`
	ProxyProtocol bool   `json:"proxy_protocol" gorm:"not null;default:false"`
	Enabled       bool   `json:"enabled" gorm:"not null;default:true"`
	IsPanel       bool   `json:"is_panel" gorm:"not null;default:false"`
	// DualCert：同 CaddySite.DualCert ✓ —— 是否挂同域名的多张证书（ECC+RSA）；nil=是。
	DualCert *bool `json:"dual_cert" gorm:"default:true"`
	Priority int   `json:"priority" gorm:"not null;default:100"`

	// TLS options (Nginx)
	EnableHttp2     bool   `json:"enable_http2" gorm:"not null;default:true"`
	EnableQuic      bool   `json:"enable_quic" gorm:"not null;default:false"`
	EnableIpv6      bool   `json:"enable_ipv6" gorm:"not null;default:false"`
	EnableEarlyData bool   `json:"enable_early_data" gorm:"not null;default:false"`
	EnableStapling  bool   `json:"enable_stapling" gorm:"not null;default:false"`
	SslProtocols    string `json:"ssl_protocols" gorm:"size:128"`
	SslCiphers      string `json:"ssl_ciphers" gorm:"size:512"`
	SslEcdhCurve    string `json:"ssl_ecdh_curve" gorm:"size:255"`

	CreatedAt time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt time.Time `json:"updated_at" gorm:"not null"`
}

func (ProxySite) TableName() string { return "proxy_sites" }
