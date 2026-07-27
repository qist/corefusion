package dto

// RateLimitConfigUpdate is the request body for updating rate limit config.
type RateLimitConfigUpdate struct {
	Enabled           *bool   `json:"enabled" binding:"required"`
	Iface             *string `json:"iface"`
	IfbDevice         *string `json:"ifb_device"`
	DefaultMaxDownMbps *int32 `json:"default_max_down_mbps"`
	DefaultMaxUpMbps  *int32  `json:"default_max_up_mbps"`
	IdleTTLSeconds    *int64  `json:"idle_ttl_seconds"`
	CleanupInterval   *int64  `json:"cleanup_interval"`
	BackendType       *string `json:"backend_type"`
}

// RateLimitStatusResponse is the response for rate limit status queries.
type RateLimitStatusResponse struct {
	ServerID       int64                `json:"server_id"`
	Config         *RateLimitConfigVO   `json:"config"`
	ActiveEntries  []RateLimitEntryVO   `json:"active_entries"`
	Availability   RateLimitAvailability `json:"availability"`
}

// RateLimitConfigVO is the view object for rate limit config.
type RateLimitConfigVO struct {
	ID                int64  `json:"id"`
	ServerID          int64  `json:"server_id"`
	Enabled           bool   `json:"enabled"`
	Iface             string `json:"iface"`
	IfbDevice         string `json:"ifb_device"`
	DefaultMaxDownMbps int32 `json:"default_max_down_mbps"`
	DefaultMaxUpMbps  int32  `json:"default_max_up_mbps"`
	IdleTTLSeconds    int64  `json:"idle_ttl_seconds"`
	CleanupInterval   int64  `json:"cleanup_interval"`
	BackendType       string `json:"backend_type"`
	NftAvailable      bool   `json:"nft_available"`
	TcAvailable       bool   `json:"tc_available"`
	IfbAvailable      bool   `json:"ifb_available"`
	ActiveEntries     int    `json:"active_entries"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
}

// RateLimitEntryVO is the view object for an active rate limit entry.
type RateLimitEntryVO struct {
	SubscriptionID int64  `json:"subscription_id"`
	IP             string `json:"ip"`
	Mark           uint32 `json:"mark"`
	MaxDownMbps    int32  `json:"max_down_mbps"`
	MaxUpMbps      int32  `json:"max_up_mbps"`
	LastSeen       int64  `json:"last_seen"`
}

// RateLimitAvailability represents the backend availability.
type RateLimitAvailability struct {
	NftAvailable bool `json:"nft_available"`
	TcAvailable  bool `json:"tc_available"`
	IfbAvailable bool `json:"ifb_available"`
}
