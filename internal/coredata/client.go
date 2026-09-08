package coredata

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"

	statsService "github.com/xtls/xray-core/app/stats/command"
)

// DefaultStatsAddr 是 xray stats API 的默认地址。
const DefaultStatsAddr = "127.0.0.1:10085"

// LocalServerID 是本地节点（面板的 xray 直连节点）的固定 server ID。
// 此约定此前在 xraystats/xraycontrol handler 中各自硬编码为 localServerID = 1，
// 抽到此统一，保证单一来源。
const LocalServerID int64 = 1

// IsLocalNode 判断 serverID 是否为本地直连节点。
func IsLocalNode(serverID int64) bool {
	return serverID == LocalServerID
}

// StatsAddrDeps 描述解析 stats 地址所需的依赖。所有字段可选，nil 表示跳过。
type StatsAddrDeps struct {
	// SettingGetter 从配置存储读取 key（如 "xray.stats_addr"）。
	SettingGetter func(ctx context.Context, key string) (string, error)
	// APIListenGetter 从核心配置解析 api.listen 作为兜底（写操作层无此依赖时可传 nil）。
	// 用于 xray: 解析 full_config["api"]["listen"]。
	APIListenGetter func(ctx context.Context) (string, error)
	// V2RayAPIListenGetter 从 sing-box 核心配置解析 experimental.v2ray_api.listen 作为兜底。
	// 用于 sing-box: 解析 full_config["experimental"]["v2ray_api"]["listen"]。
	V2RayAPIListenGetter func(ctx context.Context) (string, error)
}

// ResolveStatsAddr 统一解析 stats gRPC 地址：
//   xray:      DB "xray.stats_addr" → 核心配置 "api.listen" → DefaultStatsAddr
//   singbox:   DB "singbox.stats_addr" → 核心配置 "experimental.v2ray_api.listen" → DefaultStatsAddr
//
// 对于 sing-box，设置 V2RayAPIListenGetter 而非 APIListenGetter。
// SettingGetter 会尝试两个 key：先 "singbox.stats_addr" 再 "xray.stats_addr"，
// 以兼容统一设置场景。
//
// 此前该逻辑在 xraystats/handler.go 与 trafficsync/service.go 各有一份，
// 且后者缺少 api.listen 兜底，行为不一致。抽到此统一，保证单一来源。
func ResolveStatsAddr(ctx context.Context, deps StatsAddrDeps) string {
	if deps.SettingGetter != nil {
		// Try singbox-specific key first, then xray key
		for _, key := range []string{"singbox.stats_addr", "xray.stats_addr"} {
			if v, err := deps.SettingGetter(ctx, key); err == nil && v != "" {
				return v
			}
		}
	}
	// For sing-box: try V2Ray API listen address
	if deps.V2RayAPIListenGetter != nil {
		if v, err := deps.V2RayAPIListenGetter(ctx); err == nil && v != "" {
			return v
		}
	}
	// For xray: try api.listen
	if deps.APIListenGetter != nil {
		if v, err := deps.APIListenGetter(ctx); err == nil && v != "" {
			return v
		}
	}
	return DefaultStatsAddr
}

// XrayClient 封装 xray gRPC 建连与原始调用，是 xray gRPC 连接唯一来源。
// xraystats/handler.go、trafficsync/service.go 及 agent/xrayapi.go 的 getXrayConn
// 均复用本 XrayClient.Conn，避免重复建连逻辑。
//
// 注意：每次 RawStats 重建连接（参考原 xraystats.connect 注释，避免 xray
// 重启/重载后传输层失效）。如需连接池可在此扩展，但保持 fail-safe 语义。
type XrayClient struct {
	addr   string
	connMu sync.Mutex
	conn   *grpc.ClientConn
}

// NewXrayClient 创建客户端，addr 为空时回退到 DefaultStatsAddr。
func NewXrayClient(addr string) *XrayClient {
	if addr == "" {
		addr = DefaultStatsAddr
	}
	return &XrayClient{addr: addr}
}

// Addr 返回当前使用的连接地址。
func (c *XrayClient) Addr() string { return c.addr }

// Close 关闭底层连接（若已建立）。
func (c *XrayClient) Close() {
	c.connMu.Lock()
	defer c.connMu.Unlock()
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}

// Conn 返回底层 gRPC 连接（懒建连并复用），供需要其他 xray 服务客户端
// （如 proxyman/router）的调用方复用同一建连逻辑，避免在写操作层重复
// grpc.NewClient + WaitForReady 建连。
func (c *XrayClient) Conn(ctx context.Context) (*grpc.ClientConn, error) {
	return c.dial(ctx)
}

// dial 为本次请求建立（或复用）连接。复用仅在连接仍 Ready 时生效。
//
// 建连使用 grpc.NewClient（非阻塞，官方推荐的 DialContext 替代）。
// NewClient 创建时不会真正拨号，错误会延迟到首次 RPC，因此这里显式
// Connect 并等待 Ready（带 5s 超时），保留原 DialContext + WithBlock 的
// fail-safe 语义：xray 未启动/地址错误时立即报错，而非把错误推迟到 RPC。
func (c *XrayClient) dial(ctx context.Context) (*grpc.ClientConn, error) {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	if c.conn != nil {
		if c.conn.GetState() != connectivity.Shutdown {
			return c.conn, nil
		}
		c.conn.Close()
		c.conn = nil
	}

	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	conn, err := grpc.NewClient(c.addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("create xray client %s: %w", c.addr, err)
	}
	if err := WaitForReady(dialCtx, conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("dial xray stats %s: %w", c.addr, err)
	}
	c.conn = conn
	return conn, nil
}

// WaitForReady 阻塞直到 conn 进入 Ready 或 ctx 超时/取消。用于在 grpc.NewClient
// （非阻塞）创建连接后显式等待传输层就绪，等价于旧 DialContext + WithBlock 的
// fail-safe 语义。遇到 Shutdown/非瞬态失败会立即返回错误。
func WaitForReady(ctx context.Context, conn *grpc.ClientConn) error {
	conn.Connect()
	for {
		state := conn.GetState()
		if state == connectivity.Ready {
			return nil
		}
		if state == connectivity.Shutdown {
			return fmt.Errorf("connection shutdown before ready")
		}
		if !conn.WaitForStateChange(ctx, state) {
			// ctx 超时或取消：WaitForStateChange 返回 false。
			if state == connectivity.TransientFailure {
				return fmt.Errorf("connection failed (transient): %w", ctx.Err())
			}
			return fmt.Errorf("timeout waiting for connection: %w", ctx.Err())
		}
	}
}

// RawStats 返回 xray QueryStats 的原始响应。reset 控制是否复位计数器：
//   - false：累计值（面板展示用）
//   - true： 增量值并复位（trafficsync / agent 采集用）
//
// 解析交给 ParseStatsResponse，避免解析逻辑散落。
func (c *XrayClient) RawStats(ctx context.Context, reset bool) (*statsService.QueryStatsResponse, error) {
	conn, err := c.dial(ctx)
	if err != nil {
		return nil, err
	}
	client := statsService.NewStatsServiceClient(conn)
	resp, err := client.QueryStats(ctx, &statsService.QueryStatsRequest{Reset_: reset})
	if err != nil {
		return nil, fmt.Errorf("query xray stats: %w", err)
	}
	return resp, nil
}

// GetUsersStats 返回在线用户（含 IP 与可选流量）。在线用户/限速采集复用此方法，
// 替代此前在 cmd/agent/online_users.go、ratelimit/online_stats_poller.go 中重复的
// grpc.DialContext + NewStatsServiceClient 建连。
func (c *XrayClient) GetUsersStats(ctx context.Context, includeTraffic bool) (*statsService.GetUsersStatsResponse, error) {
	conn, err := c.dial(ctx)
	if err != nil {
		return nil, err
	}
	client := statsService.NewStatsServiceClient(conn)
	resp, err := client.GetUsersStats(ctx, &statsService.GetUsersStatsRequest{
		IncludeTraffic: includeTraffic,
	})
	if err != nil {
		return nil, fmt.Errorf("get xray users stats: %w", err)
	}
	return resp, nil
}

// ResetUserStats 复位指定用户的流量计数器。
func (c *XrayClient) ResetUserStats(ctx context.Context, email string) error {
	conn, err := c.dial(ctx)
	if err != nil {
		return err
	}
	client := statsService.NewStatsServiceClient(conn)
	resp, err := client.QueryStats(ctx, &statsService.QueryStatsRequest{Reset_: false})
	if err != nil {
		return fmt.Errorf("query xray stats for reset: %w", err)
	}
	for _, stat := range resp.GetStat() {
		name := stat.GetName()
		if contains(name, "user>>>"+email+">>>traffic>>>") {
			if _, err := client.QueryStats(ctx, &statsService.QueryStatsRequest{
				Pattern: name,
				Reset_:  true,
			}); err != nil {
				return fmt.Errorf("reset stat %s: %w", name, err)
			}
		}
	}
	return nil
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
