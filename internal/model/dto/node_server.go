package dto

// NodeServerCreate is the request body for creating a node server.
type NodeServerCreate struct {
	Name             string `json:"name" binding:"required"`
	Hostname         string `json:"hostname" binding:"required"`
	Region           string `json:"region"`
	RegionLabel      string `json:"region_label"`
	NodeType         string `json:"node_type"`
	PartnerServerID  int64  `json:"partner_server_id"`
	FailoverRole     string `json:"failover_role"`
	FailoverPriority int    `json:"failover_priority"`
	ChainConfig      string `json:"chain_config"`
	AgentPort        int    `json:"agent_port"`
	IsLocal          bool   `json:"is_local"`
	SupportedCores   string `json:"supported_cores"`
	MaxInbounds      int    `json:"max_inbounds"`
	PublicIP         string `json:"public_ip"`
	InternalIP       string `json:"internal_ip"`
	BandwidthMbps    int    `json:"bandwidth_mbps"`
	BillingEnabled   *bool  `json:"billing_enabled"`
	CertificateID    int64  `json:"certificate_id"`
	Metadata         string `json:"metadata"`
	SortOrder        int    `json:"sort_order"`
	Enabled          *bool  `json:"enabled"`

	// 出口网卡流量配额
	EgressInterface      string `json:"egress_interface"`
	TrafficQuotaBytes    int64  `json:"traffic_quota_bytes"`
	TrafficQuotaResetDay int    `json:"traffic_quota_reset_day"`
}

// NodeServerUpdate is the request body for updating a node server.
type NodeServerUpdate struct {
	Name             *string `json:"name"`
	Hostname         *string `json:"hostname"`
	Region           *string `json:"region"`
	RegionLabel      *string `json:"region_label"`
	NodeType         *string `json:"node_type"`
	PartnerServerID  *int64  `json:"partner_server_id"`
	FailoverRole     *string `json:"failover_role"`
	FailoverPriority *int    `json:"failover_priority"`
	ChainConfig      *string `json:"chain_config"`
	AgentPort        *int    `json:"agent_port"`
	SupportedCores   *string `json:"supported_cores"`
	MaxInbounds      *int    `json:"max_inbounds"`
	PublicIP         *string `json:"public_ip"`
	InternalIP       *string `json:"internal_ip"`
	BandwidthMbps    *int    `json:"bandwidth_mbps"`
	BillingEnabled   *bool   `json:"billing_enabled"`
	CertificateID    *int64  `json:"certificate_id"`
	Metadata         *string `json:"metadata"`
	SortOrder        *int    `json:"sort_order"`
	Enabled          *bool   `json:"enabled"`

	// 出口网卡流量配额
	EgressInterface      *string `json:"egress_interface"`
	TrafficQuotaBytes    *int64  `json:"traffic_quota_bytes"`
	TrafficQuotaResetDay *int    `json:"traffic_quota_reset_day"`
}

// NodeServerListQuery is the query parameters for listing node servers.
type NodeServerListQuery struct {
	ListQuery
	Region   string `form:"region"`
	NodeType string `form:"node_type"`
	Status   string `form:"status"`
}
