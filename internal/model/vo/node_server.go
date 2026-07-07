package vo

import "time"

// NodeServerResponse is the API response for a node server.
type NodeServerResponse struct {
	ID                int64      `json:"id"`
	Name              string     `json:"name"`
	Hostname          string     `json:"hostname"`
	Region            string     `json:"region"`
	RegionLabel       string     `json:"region_label"`
	NodeType          string     `json:"node_type"`
	PartnerServerID   int64      `json:"partner_server_id"`
	FailoverRole      string     `json:"failover_role"`
	FailoverPriority  int        `json:"failover_priority"`
	ChainConfig       string     `json:"chain_config,omitempty"`
	AgentPort         int        `json:"agent_port"`
	AgentStatus       string     `json:"agent_status"`
	AgentVersion      string     `json:"agent_version"`
	LastHeartbeatAt   *time.Time `json:"last_heartbeat_at"`

	// Bootstrap status (never expose the token itself)
	BootstrapRequired    bool       `json:"bootstrap_required"`
	BootstrapTokenExpiry *time.Time `json:"bootstrap_token_expires_at,omitempty"`
	BootstrapUsedAt      *time.Time `json:"bootstrap_used_at,omitempty"`

	// Agent cert status
	AgentCertExpiresAt     *time.Time `json:"agent_cert_expires_at,omitempty"`
	AgentCertLastRenewedAt *time.Time `json:"agent_cert_last_renewed_at,omitempty"`

	// Certificate association
	CertificateID int64 `json:"certificate_id"`

	IsLocal        bool   `json:"is_local"`
	SupportedCores string `json:"supported_cores"`
	MaxInbounds    int    `json:"max_inbounds"`
	PublicIP       string `json:"public_ip"`
	InternalIP     string `json:"internal_ip"`
	BandwidthMbps  int    `json:"bandwidth_mbps"`
	BillingEnabled bool   `json:"billing_enabled"`
	Metadata       string `json:"metadata,omitempty"`
	SortOrder      int    `json:"sort_order"`
	Enabled        bool   `json:"enabled"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// BootstrapTokenResponse is returned when generating a bootstrap token.
// The token is only shown once — the admin must copy it immediately.
type BootstrapTokenResponse struct {
	ServerID    int64     `json:"server_id"`
	Token       string    `json:"token"`
	ExpiresAt   time.Time `json:"expires_at"`
	Instruction string   `json:"instruction"`
}

// CreateServerResponse is returned when creating a node server.
// It includes the server data plus a one-time bootstrap token.
// The frontend builds the curl | bash command from window.location
// (scheme/host/port) since the backend can't reliably detect TLS
// behind a reverse proxy (Caddy/Nginx).
type CreateServerResponse struct {
	*NodeServerResponse
	BootstrapToken string `json:"bootstrap_token,omitempty"` // Only returned on creation
}
