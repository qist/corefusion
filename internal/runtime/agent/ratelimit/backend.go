package ratelimit

import "context"

// RateLimitBackend 是限速后端的抽象接口。
// 第一阶段实现 NftTcBackend（nft 打标记 + tc htb 限速），
// 未来可扩展 EbpfBackend。
//
// 所有方法都是幂等的：重复调用不会产生副作用。
type RateLimitBackend interface {
	// Init 初始化限速基础设施（创建 nft 表/链、tc qdisc、IFB 设备等）。
	// 应在启动时调用一次。如果基础设施已存在，不报错。
	Init(ctx context.Context, iface string) error

	// ApplyRule 为一个 IP 应用限速规则：
	//   1. nft: ip saddr X → meta mark set Y
	//   2. tc: 创建 htb class (rate=maxDownMbps)
	//   3. tc: 创建 filter (handle mark Y → classid 1:Y)
	//   4. tc/ifb: 创建上传限速 class + filter
	//   （并发连接限制是**账号/订阅维度**，见 ApplyConnLimit —— 不在这里按 IP 加）
	ApplyRule(ctx context.Context, ip string, mark uint32, rate *RateConfig) error

	// UpdateRate 更新一个订阅的限速速率（只改 tc class rate，不改 nft 规则）。
	UpdateRate(ctx context.Context, mark uint32, rate *RateConfig) error

	// RemoveRule 移除一个 IP 的限速规则（删除 nft 规则 + tc class + filter）。
	RemoveRule(ctx context.Context, ip string, mark uint32) error

	// RemoveSubscription 移除一个订阅的所有限速规则。
	RemoveSubscription(ctx context.Context, mark uint32) error

	// ApplyConnLimit 应用**账号（订阅）维度**的并发连接限制：
	// 用 nft 动态集合按 meta mark（= 订阅唯一标记）聚合 conntrack 计数，
	// 该订阅所有设备的并发连接**合计**超过 connLimit 时丢弃新连接。
	// connLimit = 0 表示移除限制（不限并发）。
	ApplyConnLimit(ctx context.Context, mark uint32, connLimit int32) error

	// RemoveConnLimit 移除一个订阅的并发连接限制。
	RemoveConnLimit(ctx context.Context, mark uint32) error

	// BlockIP 在 nftables 中阻断一个 IP 的所有流量（用于设备数限制实时拦截）。
	BlockIP(ctx context.Context, ip string, mark uint32) error

	// UnblockIP 移除对 IP 的阻断。
	UnblockIP(ctx context.Context, ip string, mark uint32) error

	// FlushAll 清除所有限速规则（用于重建或关闭）。
	FlushAll(ctx context.Context) error

	// IsAvailable 检查后端依赖是否可用（nft、tc、ifb 等）。
	IsAvailable() Availability

	// Close 清理资源。
	Close() error
}

// Availability 描述后端的可用性。
type Availability struct {
	NftAvailable bool `json:"nft_available"`
	TcAvailable  bool `json:"tc_available"`
	IfbAvailable bool `json:"ifb_available"`
	// ConnTrackAvailable：内核 conntrack（nf_conntrack）是否可用 —— 账号并发计数依赖它
	ConnTrackAvailable bool `json:"conntrack_available"`
	// KernelSupported：内核是否支持账号并发计数所需能力
	// （nft 动态集合 + ct count，实测探测：需内核 ≥4.18），不只是命令存在
	KernelSupported bool `json:"kernel_supported"`
	// UnsupportedReason：不支持时的原因（供 UI 直接展示）
	UnsupportedReason string `json:"unsupported_reason"`
}

// OverallSupported 返回"限速功能整体可用"（下载/上传限速的最小依赖）。
func (a Availability) OverallSupported() bool {
	return a.NftAvailable && a.TcAvailable
}

// ConnLimitSupported 返回"账号并发计数可用"。
func (a Availability) ConnLimitSupported() bool {
	return a.NftAvailable && a.ConnTrackAvailable && a.KernelSupported
}
