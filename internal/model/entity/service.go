package entity

import "time"

// InboundService is the GORM model for the inbound_services table.
type InboundService struct {
	ID int64 `json:"id" gorm:"primaryKey;autoIncrement"`
	ServerID int64 `json:"server_id" gorm:"not null;default:1;index"` // per-node scoping
	Name string `json:"name" gorm:"size:128;not null"`
	CoreType string `json:"core_type" gorm:"size:32;not null"`
	Protocol string `json:"protocol" gorm:"size:32;not null"`
	ListenHost string `json:"listen_host" gorm:"size:255;not null"`
	ListenPort int `json:"listen_port" gorm:"not null"`
	Transport string `json:"transport" gorm:"size:32"`
	TLSEnabled int `json:"tls_enabled" gorm:"not null;default:0"`
	ConfigJSON string `json:"config_json" gorm:"type:longtext;not null"`
	ProcessStatus string `json:"process_status" gorm:"size:32"`
	AutoStart     bool      `json:"auto_start" gorm:"not null;default:false"`
	AutoRestart   bool      `json:"auto_restart" gorm:"not null;default:false"`
	CreatedAt     time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt     time.Time `json:"updated_at" gorm:"not null"`
}

func (InboundService) TableName() string { return "inbound_services" }

// ServiceEndpoint is the GORM model for the service_endpoints table.
type ServiceEndpoint struct {
	ID int64 `json:"id" gorm:"primaryKey;autoIncrement"`
	ServiceID int64 `json:"service_id" gorm:"index;not null"`
	EndpointType string `json:"endpoint_type" gorm:"size:16;not null"`
	Address string `json:"address" gorm:"size:255;not null"`
	ServerName string `json:"server_name" gorm:"size:255"`
	ListenHost string `json:"listen_host" gorm:"size:255"`
	ListenPort int `json:"listen_port" gorm:"not null"`
	Transport string `json:"transport" gorm:"size:32"`
	TLSEnabled int `json:"tls_enabled" gorm:"not null;default:0"`
	CertificateID *int64
	IsDefault int `json:"is_default" gorm:"not null;default:0"`
	Priority int `json:"priority" gorm:"not null;default:100"`
	Status int `json:"status" gorm:"not null;default:1"`
	CreatedAt     time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt     time.Time `json:"updated_at" gorm:"not null"`
}

func (ServiceEndpoint) TableName() string { return "service_endpoints" }
