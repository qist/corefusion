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
	ApplyRule(ctx context.Context, ip string, mark uint32, rate *RateConfig) error

	// UpdateRate 更新一个订阅的限速速率（只改 tc class rate，不改 nft 规则）。
	UpdateRate(ctx context.Context, mark uint32, rate *RateConfig) error

	// RemoveRule 移除一个 IP 的限速规则（删除 nft 规则 + tc class + filter）。
	RemoveRule(ctx context.Context, ip string, mark uint32) error

	// RemoveSubscription 移除一个订阅的所有限速规则。
	RemoveSubscription(ctx context.Context, mark uint32) error

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
}
