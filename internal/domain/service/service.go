package service

import (
	"time"

	"corefusion/internal/domain"
)

// InboundService represents a proxy service instance.
type InboundService struct {
	ID            int64                `json:"id"`
	Name          string               `json:"name"`
	CoreType      domain.CoreType      `json:"core_type"`
	Protocol      domain.ProtocolType  `json:"protocol"`
	ListenHost    string               `json:"listen_host"`
	ListenPort    int                  `json:"listen_port"`
	Transport     domain.TransportType `json:"transport"`
	TLSEnabled    bool                 `json:"tls_enabled"`
	ConfigJSON    string               `json:"config_json"`
	ProcessStatus string               `json:"process_status"`
	CreatedAt     time.Time            `json:"created_at"`
	UpdatedAt     time.Time            `json:"updated_at"`
}

// ServiceEndpoint represents a service endpoint (domain/IP entry point).
type ServiceEndpoint struct {
	ID            int64                `json:"id"`
	ServiceID     int64                `json:"service_id"`
	EndpointType  domain.EndpointType  `json:"endpoint_type"`
	Address       string               `json:"address"`
	ServerName    string               `json:"server_name"`
	ListenHost    string               `json:"listen_host"`
	ListenPort    int                  `json:"listen_port"`
	Transport     domain.TransportType `json:"transport"`
	TLSEnabled    bool                 `json:"tls_enabled"`
	CertificateID *int64               `json:"certificate_id"`
	IsDefault     bool                 `json:"is_default"`
	Priority      int                  `json:"priority"`
	Status        int                  `json:"status"`
	CreatedAt     time.Time            `json:"created_at"`
	UpdatedAt     time.Time            `json:"updated_at"`
}

// ServiceSpec defines the specification for creating a service.
type ServiceSpec struct {
	Name       string                `json:"name"`
	CoreType   domain.CoreType       `json:"core_type"`
	Protocol   domain.ProtocolType   `json:"protocol"`
	Transport  domain.TransportType  `json:"transport"`
	TLSMode    domain.TLSMode        `json:"tls_mode"`
	ListenPort int                   `json:"listen_port"`
	Endpoints  []ServiceEndpointSpec `json:"endpoints"`
	Extra      map[string]any        `json:"extra"`
}

// ServiceEndpointSpec defines the specification for creating an endpoint.
type ServiceEndpointSpec struct {
	EndpointType  domain.EndpointType `json:"endpoint_type"`
	Address       string              `json:"address"`
	ServerName    string              `json:"server_name"`
	ListenHost    string              `json:"listen_host"`
	ListenPort    int                 `json:"listen_port"`
	TLSEnabled    bool                `json:"tls_enabled"`
	CertificateID *int64              `json:"certificate_id"`
	IsDefault     bool                `json:"is_default"`
	Priority      int                 `json:"priority"`
}
