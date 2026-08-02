package coredata

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	statsService "github.com/xtls/xray-core/app/stats/command"
)

// DefaultStatsAddr 是 xray stats API 的默认地址。
const DefaultStatsAddr = "127.0.0.1:10085"

// XrayClient 封装 xray gRPC 建连与原始调用，统一此前在 xraystats/handler.go
// 与 agent/xrayapi.go 中重复的 grpc.DialContext 逻辑。
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

// dial 为本次请求建立（或复用）连接。复用仅在连接仍 Ready 时生效。
func (c *XrayClient) dial(ctx context.Context) (*grpc.ClientConn, error) {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	if c.conn != nil {
		if c.conn.GetState().String() != "SHUTDOWN" {
			return c.conn, nil
		}
		c.conn.Close()
		c.conn = nil
	}

	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(dialCtx, c.addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("dial xray stats %s: %w", c.addr, err)
	}
	c.conn = conn
	return conn, nil
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
