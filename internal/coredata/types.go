// Package coredata 提供核心数据公共层：将分散在多个文件中的 xray gRPC API
// 访问（stats / sysstats / online users）收敛为统一接口与结构，使前端/面板
// 只依赖 CoreDataProvider，而不感知具体核心协议与本地/远程路由。
//
// 设计见 docs/design/core-data-layer-design.md
package coredata

// CoreKind 标记核心类型，便于未来扩展（sing-box / hysteria2 / tuic）。
type CoreKind string

const (
	KindXray CoreKind = "xray"
	// KindSingBox / KindHysteria 预留
)

// 流量方向
const (
	DirUplink   = "uplink"
	DirDownlink = "downlink"
)

// UserTraffic 单用户累计流量（来自 user>>>email>>>traffic>>>）。
// 字段与 JSON 键与原 xraystats 包保持一致，便于平滑替换。
type UserTraffic struct {
	Email    string `json:"email"`
	Uplink   int64  `json:"uplink"`
	Downlink int64  `json:"downlink"`
	Total    int64  `json:"total"`
}

// InboundTraffic 单 inbound 累计流量（来自 inbound>>>tag>>>traffic>>>）。
type InboundTraffic struct {
	Tag      string `json:"tag"`
	Uplink   int64  `json:"uplink"`
	Downlink int64  `json:"downlink"`
	Total    int64  `json:"total"`
}

// SysStats xray 系统级统计（StatsService.GetSysStats）。
type SysStats struct {
	NumGoroutine uint32 `json:"numGoroutine"`
	Alloc        int64  `json:"alloc"`
}

// OnlineUser 在线用户 + 实时速率（HandlerService.GetUsers + 两次采样差分由调用方计算）。
type OnlineUser struct {
	Email         string `json:"email"`
	SubID         int64  `json:"subId,omitempty"`
	UplinkSpeed   int64  `json:"uplinkSpeed"`
	DownlinkSpeed int64  `json:"downlinkSpeed"`
	LastSeenUnix  int64  `json:"lastSeenUnix"`
}

// StatsResponse 聚合统计响应，对应原 xraystats.StatsResponse。
type StatsResponse struct {
	Users    []UserTraffic    `json:"users"`
	Inbounds []InboundTraffic `json:"inbounds"`
}
