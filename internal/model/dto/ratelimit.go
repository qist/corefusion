package dto

// RateLimitConfigUpdate is the request body for updating rate limit config.
type RateLimitConfigUpdate struct {
	Enabled            *bool   `json:"enabled" binding:"required"`
	Iface              *string `json:"iface"`
	IfbDevice          *string `json:"ifb_device"`
	DefaultMaxDownMbps *int32  `json:"default_max_down_mbps"`
	DefaultMaxUpMbps   *int32  `json:"default_max_up_mbps"`
	IdleTTLSeconds     *int64  `json:"idle_ttl_seconds"`
	CleanupInterval    *int64  `json:"cleanup_interval"`
	BackendType        *string `json:"backend_type"`
}

// RateLimitStatusResponse is the response for rate limit status queries.
type RateLimitStatusResponse struct {
	ServerID      int64                 `json:"server_id"`
	Config        *RateLimitConfigVO    `json:"config"`
	ActiveEntries []RateLimitEntryVO    `json:"active_entries"`
	Availability  RateLimitAvailability `json:"availability"`
}

// RateLimitConfigVO is the view object for rate limit config.
type RateLimitConfigVO struct {
	ID                 int64  `json:"id"`
	ServerID           int64  `json:"server_id"`
	Enabled            bool   `json:"enabled"`
	Iface              string `json:"iface"`
	IfbDevice          string `json:"ifb_device"`
	DefaultMaxDownMbps int32  `json:"default_max_down_mbps"`
	DefaultMaxUpMbps   int32  `json:"default_max_up_mbps"`
	IdleTTLSeconds     int64  `json:"idle_ttl_seconds"`
	CleanupInterval    int64  `json:"cleanup_interval"`
	BackendType        string `json:"backend_type"`
	NftAvailable       bool   `json:"nft_available"`
	TcAvailable        bool   `json:"tc_available"`
	IfbAvailable       bool   `json:"ifb_available"`
	// 内核/conntrack 能力（Agent 实时探测，非 DB 静态值）
	ConnTrackAvailable bool   `json:"conntrack_available"`
	KernelSupported    bool   `json:"kernel_supported"`
	UnsupportedReason  string `json:"unsupported_reason"`
	ActiveEntries      int    `json:"active_entries"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
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
// 全部来自 Agent 的**实时探测**（功能探测 + 内核版本 + conntrack），不是 DB 静态值。
type RateLimitAvailability struct {
	NftAvailable       bool   `json:"nft_available"`
	TcAvailable        bool   `json:"tc_available"`
	IfbAvailable       bool   `json:"ifb_available"`
	ConnTrackAvailable bool   `json:"conntrack_available"`
	KernelSupported    bool   `json:"kernel_supported"`
	UnsupportedReason  string `json:"unsupported_reason"`
}

// OverallSupported 限速功能整体可用（下载/上传限速的最小依赖：nft + tc）。
func (a RateLimitAvailability) OverallSupported() bool {
	return a.NftAvailable && a.TcAvailable
}

// ConnLimitSupported 账号并发计数可用（nft 动态集合 + ct count + conntrack）。
func (a RateLimitAvailability) ConnLimitSupported() bool {
	return a.NftAvailable && a.ConnTrackAvailable && a.KernelSupported
}

// AgentRateLimitStatus is the agent's live rate limit status
// (get_rate_limit_status 同步查询的返回：活跃条目 + 实时能力探测)。
// ⚠ Agent 端 sendSyncResult 的是**扁平** map（能力字段在顶层，不在 availability 里）。
type AgentRateLimitStatus struct {
	Entries []RateLimitEntryVO `json:"entries"`

	NftAvailable       bool   `json:"nft_available"`
	TcAvailable        bool   `json:"tc_available"`
	IfbAvailable       bool   `json:"ifb_available"`
	ConnTrackAvailable bool   `json:"conntrack_available"`
	KernelSupported    bool   `json:"kernel_supported"`
	UnsupportedReason  string `json:"unsupported_reason"`
}

// Availability 把扁平字段装配成 RateLimitAvailability。
func (s *AgentRateLimitStatus) Availability() RateLimitAvailability {
	return RateLimitAvailability{
		NftAvailable:       s.NftAvailable,
		TcAvailable:        s.TcAvailable,
		IfbAvailable:       s.IfbAvailable,
		ConnTrackAvailable: s.ConnTrackAvailable,
		KernelSupported:    s.KernelSupported,
		UnsupportedReason:  s.UnsupportedReason,
	}
}
