# 代码级设计文档

> **实现状态（2026-07-30 修订）**：第 6-8 章（多核心流量采集适配器、配额拒绝连接状态机、服务端 IP 限速与黑名单）与现行实现存在差异：
> - 流量采集：原设计的 `multi_collector/aggregator`（`service/traffic/aggregator.go`）因与 `TrafficEventConsumer` 争用同一 channel 导致约 50% 远程流量丢失，已在 `main.go` 注释停用；现行实现走 `service/trafficsync` + `traffic_consumer.go` → `inboundconfig.UpdateTraffic` → `subscriptions.*_bytes_cache`，非 xray 协议改用网络层计费（nftables 按 IP 计数 / 节点出口网卡计数器）。
> - 配额执行：实际由 `service/ratelimit`（nftables + tc）落地，`quota.Enforcer` / `quota.Service` 当前无调用方（死代码）。
> - IP 限速/黑名单：`access.Service` 底层组件已实现但未接线，管理面 API 为 placeholder。
> 详见 `UNIMPLEMENTED_FEATURES.md` 第 2、3 章。

## 1. 目标

本文件把前面的功能设计继续细化到可直接落地编码的层级，重点覆盖：

- 服务端多入口域名或 IP 列表
- 全协议自由组合与能力校验
- 多核心流量采集适配器
- 配额拒绝连接状态机
- 服务端 IP 限速与黑名单

本文件不追求最终源码细节，而是提供稳定的 Go 结构、接口和分层边界。

## 2. 目录建议

建议新增或明确以下目录：

```text
internal/
  domain/
    service/
    endpoint/
    traffic/
    quota/
    access/
  service/
    services/
    traffic/
    quota/
    access/
  store/
    models/
    repositories/
  runtime/
    adapters/
      xray/
      singbox/
      hysteria2/
      tuic/
    listeners/
  configgen/
    common/
    validate/
```

职责建议：

- `domain`：放纯领域模型和枚举
- `service`：放业务服务和流程编排
- `store`：放数据库模型与 repository
- `runtime/adapters`：放运行时核心适配器
- `configgen/validate`：放协议组合校验器

## 2.1 仍需特别关注的遗漏点

在当前设计基础上，服务端入口侧还必须补齐以下点：

- IP 新建连接限速
- IP 并发连接限制
- IP 黑名单与白名单
- 自动封禁与自动解封
- 按服务、入口、用户、订阅维度叠加访问控制
- 服务入口连接日志与封禁审计

## 3. 全协议自由组合设计

### 3.1 设计原则

协议不应被预设模板限制。

应采用：

- 自由组合
- 能力校验
- 自动提示不兼容项

而不是：

- 固定只允许几种预置组合

### 3.2 核心组合模型

建议统一使用能力描述模型：

```go
type CoreType string

const (
    CoreTypeXray      CoreType = "xray"
    CoreTypeSingBox   CoreType = "singbox"
    CoreTypeHysteria2 CoreType = "hysteria2"
    CoreTypeTUIC      CoreType = "tuic"
)

type ProtocolType string

const (
    ProtocolVLESS          ProtocolType = "vless"
    ProtocolVMess          ProtocolType = "vmess"
    ProtocolTrojan         ProtocolType = "trojan"
    ProtocolShadowsocks    ProtocolType = "shadowsocks"
    ProtocolShadowsocks2022 ProtocolType = "shadowsocks2022"
    ProtocolHysteria2      ProtocolType = "hysteria2"
    ProtocolTUIC           ProtocolType = "tuic"
)

type TransportType string

const (
    TransportTCP   TransportType = "tcp"
    TransportWS    TransportType = "ws"
    TransportGRPC  TransportType = "grpc"
    TransportHTTP2 TransportType = "http2"
    TransportQUIC  TransportType = "quic"
    TransportReality TransportType = "reality"
)

type TLSMode string

const (
    TLSModeNone     TLSMode = "none"
    TLSModeTLS      TLSMode = "tls"
    TLSModeReality  TLSMode = "reality"
    TLSModeUTLS     TLSMode = "utls"
)
```

### 3.3 服务组合请求结构

```go
type ServiceSpec struct {
    Name       string
    CoreType   CoreType
    Protocol   ProtocolType
    Transport  TransportType
    TLSMode    TLSMode
    ListenPort int
    Endpoints  []ServiceEndpointSpec
    Users      []InboundUserSpec
    Extra      map[string]any
}
```

### 3.4 能力校验器接口

```go
type CapabilityValidator interface {
    Validate(spec ServiceSpec) error
    SupportsProtocol(core CoreType, protocol ProtocolType) bool
    SupportsTransport(core CoreType, protocol ProtocolType, transport TransportType) bool
    SupportsTLS(core CoreType, protocol ProtocolType, transport TransportType, mode TLSMode) bool
}
```

### 3.5 校验规则示例

校验应包括：

- 核心是否支持该协议
- 协议是否允许该传输
- 是否需要域名入口
- 是否必须启用证书
- 是否能导出到目标客户端

示例：

```go
func (v *DefaultCapabilityValidator) Validate(spec ServiceSpec) error {
    if !v.SupportsProtocol(spec.CoreType, spec.Protocol) {
        return ErrUnsupportedProtocol
    }
    if !v.SupportsTransport(spec.CoreType, spec.Protocol, spec.Transport) {
        return ErrUnsupportedTransport
    }
    if !v.SupportsTLS(spec.CoreType, spec.Protocol, spec.Transport, spec.TLSMode) {
        return ErrUnsupportedTLSMode
    }
    if err := ValidateEndpoints(spec); err != nil {
        return err
    }
    return nil
}
```

## 4. 多入口域名或 IP 列表设计

### 4.1 领域模型

```go
type EndpointType string

const (
    EndpointTypeDomain EndpointType = "domain"
    EndpointTypeIPv4   EndpointType = "ipv4"
    EndpointTypeIPv6   EndpointType = "ipv6"
)

type ServiceEndpoint struct {
    ID            int64
    ServiceID      int64
    EndpointType   EndpointType
    Address        string
    ServerName     string
    ListenHost     string
    ListenPort     int
    Transport      TransportType
    TLSEnabled     bool
    CertificateID  *int64
    IsDefault      bool
    Priority       int
    Status         int
    CreatedAt      time.Time
    UpdatedAt      time.Time
}
```

### 4.2 创建输入结构

```go
type CreateServiceInput struct {
    Spec      ServiceSpec
    Endpoints []CreateServiceEndpointInput
}

type CreateServiceEndpointInput struct {
    EndpointType  EndpointType
    Address       string
    ServerName    string
    ListenHost    string
    ListenPort    int
    TLSEnabled    bool
    CertificateID *int64
    IsDefault     bool
    Priority      int
}
```

### 4.3 入口校验规则

至少校验：

- 域名格式合法
- IPv4 / IPv6 格式合法
- 端口范围合法
- 同一服务不能有重复入口
- 默认入口只能有一个
- 要求域名的 TLS 模式下不能缺证书

### 4.4 配置生成选项

```go
type RenderEndpointPolicy string

const (
    RenderEndpointDefault      RenderEndpointPolicy = "default"
    RenderEndpointManual       RenderEndpointPolicy = "manual"
    RenderEndpointPreferIPv4   RenderEndpointPolicy = "prefer_ipv4"
    RenderEndpointPreferIPv6   RenderEndpointPolicy = "prefer_ipv6"
    RenderEndpointAllCandidate RenderEndpointPolicy = "all_candidate"
)
```

## 5. `service_endpoints` 的存储模型

### 5.1 Gorm 模型建议

```go
type ServiceEndpointModel struct {
    ID            int64      `gorm:"primaryKey"`
    ServiceID      int64      `gorm:"index;not null"`
    EndpointType   string     `gorm:"size:16;not null"`
    Address        string     `gorm:"size:255;not null"`
    ServerName     string     `gorm:"size:255"`
    ListenHost     string     `gorm:"size:255"`
    ListenPort     int        `gorm:"not null"`
    Transport      string     `gorm:"size:32"`
    TLSEnabled     bool       `gorm:"not null;default:false"`
    CertificateID  *int64
    IsDefault      bool       `gorm:"not null;default:false"`
    Priority       int        `gorm:"not null;default:100"`
    Status         int        `gorm:"not null;default:1"`
    CreatedAt      time.Time  `gorm:"not null"`
    UpdatedAt      time.Time  `gorm:"not null"`
}
```

### 5.2 Repository 接口建议

```go
type ServiceEndpointRepository interface {
    Create(ctx context.Context, m *ServiceEndpointModel) error
    BatchCreate(ctx context.Context, items []*ServiceEndpointModel) error
    ListByServiceID(ctx context.Context, serviceID int64) ([]*ServiceEndpointModel, error)
    GetDefaultByServiceID(ctx context.Context, serviceID int64) (*ServiceEndpointModel, error)
    Update(ctx context.Context, m *ServiceEndpointModel) error
    Delete(ctx context.Context, id int64) error
}
```

## 6. 多核心流量采集适配器代码级设计

### 6.1 标准事件

```go
type TrafficDeltaEvent struct {
    SessionID        string    `json:"session_id"`
    UserID           int64     `json:"user_id"`
    SubscriptionID   int64     `json:"subscription_id"`
    ServiceID        int64     `json:"service_id"`
    EndpointID       int64     `json:"endpoint_id"`
    CoreType         CoreType  `json:"core_type"`
    Protocol         ProtocolType `json:"protocol"`
    UplinkBytes      int64     `json:"uplink_bytes"`
    DownlinkBytes    int64     `json:"downlink_bytes"`
    BillingBytes     int64     `json:"billing_bytes"`
    Sequence         int64     `json:"seq"`
    SourceCollector  string    `json:"source_collector"`
    CollectedAt      time.Time `json:"collected_at"`
}
```

### 6.2 采集器接口

```go
type TrafficCollectorAdapter interface {
    CoreType() CoreType
    Start(ctx context.Context, in CollectorStartInput) error
    Stop(ctx context.Context) error
    Snapshot(ctx context.Context) (*CollectorSnapshot, error)
    Subscribe(ctx context.Context, ch chan<- TrafficDeltaEvent) error
}
```

### 6.3 采集器注册中心

```go
type TrafficCollectorRegistry interface {
    Register(adapter TrafficCollectorAdapter) error
    Get(coreType CoreType) (TrafficCollectorAdapter, error)
}
```

### 6.4 聚合器接口

```go
type TrafficAggregator interface {
    ApplyDelta(ctx context.Context, event TrafficDeltaEvent) error
    Flush(ctx context.Context) error
}
```

### 6.5 归属解析器

因为不同核心拿到的标识不同，建议增加归属解析层：

```go
// RawCollectorEvent 是核心采集器输出的原始事件，格式因核心而异
type RawCollectorEvent struct {
    CoreType       CoreType
    UserIdentifier string         // email / tag / auth_user，各核心不同
    SessionID      string
    UplinkBytes    int64
    DownlinkBytes  int64
    Metadata       map[string]any // 核心特定的额外信息
}

type SessionOwnershipResolver interface {
    Resolve(ctx context.Context, raw RawCollectorEvent) (*TrafficOwnership, error)
}

type TrafficOwnership struct {
    UserID         int64
    SubscriptionID int64
    ServiceID      int64
    EndpointID     int64
    Protocol       ProtocolType
}
```

### 6.6 适配器分层建议

建议每个核心适配器拆成三层：

- `collector`
- `parser`
- `mapper`

职责：

- `collector`：读取核心原始统计
- `parser`：转成统一原始事件
- `mapper`：映射到用户、订阅、入口

### 6.7 文件布局建议

```text
internal/runtime/adapters/
  xray/
    traffic_collector.go
    traffic_parser.go
  singbox/
    traffic_collector.go
    traffic_parser.go
  hysteria2/
    traffic_collector.go
    traffic_parser.go
  tuic/
    traffic_collector.go
    traffic_parser.go
internal/service/traffic/
  aggregator.go
  ownership_resolver.go
  collector_registry.go
```

### 6.8 各核心流量采集可行性矩阵

不同核心的用户级流量统计能力差异较大，适配器实现必须基于实际能力：

| 核心 | 用户标识方式 | 统计接口 | 实时增量 | 第一版实现策略 |
| --- | --- | --- | --- | --- |
| Xray-core | email / UUID | gRPC Stats API (`StatsService.GetStats`) | 支持（流式查询） | 直接对接 Stats API，按 email 归属用户 |
| sing-box | 用户名 / tag | 实验性 Traffic Manager API | 不确定 | 优先对接实验性 API，不可用时降级为进程级统计 |
| Hysteria 2 | auth 用户名 | 内置统计接口 | 支持 | 对接内置接口，按 auth 用户归属 |
| TUIC | token | 有限管理接口 | 不确定 | 第一版使用进程级统计 + 网卡辅助核对 |

实现要求：

- 每个适配器必须实现 `TrafficCollectorAdapter` 接口
- 若核心不支持用户级统计，适配器的 `Subscribe` 方法应输出进程级事件，`UserID` 和 `SubscriptionID` 留空
- 聚合器遇到归属信息缺失的事件时，应进入隔离队列等待归属解析，而不是直接丢弃
- UI 中对不支持用户级统计的核心应标注"流量统计为估算值"

### 6.9 流量计算完整数据流实现

架构前提：**面板不在流量路径上**。流量路径是 用户 → nginx/caddy → 核心 或 用户直连核心。面板是纯管理面，只能：

1. 读取核心的统计 API（被动采集）
2. 修改核心配置或调用核心管理 API（主动控制）

面板不能：

- 在连接路径上做实时检查
- 设置 connmark（看不到数据包）
- 做流量转发或拦截

流量计数架构：

```text
流量路径（面板不参与）：
  用户 ──────────────────────▶ nginx/caddy ──────────▶ 核心进程 ──────────▶ 出站
                              （反向代理）             （代理转发）

管理路径（面板负责）：
  ┌─────────────────────────────────────────────────────────────┐
  │  面板（管理面，不在流量路径上）                                  │
  │                                                              │
  │  1. 定时轮询核心 Stats API（每 30 秒）                         │
  │     Xray: gRPC StatsService.QueryStats(reset_=true)          │
  │     sing-box: HTTP GET /traffic?name=<tag>                   │
  │     Hysteria 2: HTTP GET /traffic?clear=true                 │
  │     TUIC: /proc/<pid>/net/dev（降级）                         │
  │                                                              │
  │  2. 计算增量，写入流量账本                                      │
  │                                                              │
  │  3. 配额状态机评估                                             │
  │                                                              │
  │  4. 状态变更时，调用核心管理 API 执行拦截/恢复                   │
  │     Xray: gRPC HandlerService.AlterInbound (移除/添加用户)     │
  │     sing-box: HTTP POST /experimental/users/remove            │
  │     Hysteria 2: 修改配置 + SIGHUP 热重载                      │
  │     TUIC: 修改配置 + 重启                                     │
  │                                                              │
  │  5. 可选：修改 nginx/caddy 配置做反向代理层限流                  │
  └─────────────────────────────────────────────────────────────┘
```

核心设计原则：

- 面板只做两件事：**读统计** 和 **改配置**
- 流量计数是异步轮询，不是实时拦截
- 配额执行是通过核心管理 API 移除/恢复用户，不是在连接路径上拦截
- 超额用户的连接被核心直接拒绝（核心配置里没有这个用户了）
- 面板不引入额外延迟，不影响正常用户的连接性能

### 6.9.1 Xray-core 采集器具体实现

Xray-core 通过 gRPC Stats API 提供流量统计，采集器实现：

```go
type XrayTrafficCollector struct {
    serviceID   int64
    statsAddr   string // Xray gRPC stats 地址，如 "127.0.0.1:10085"
    conn        *grpc.ClientConn
    client      stats.StatsServiceClient
    subscribers []chan<- TrafficDeltaEvent
    lastValues  map[string]int64 // 上次累计值，用于计算增量
    mu          sync.RWMutex
}

func (c *XrayTrafficCollector) Start(ctx context.Context, in CollectorStartInput) error {
    conn, err := grpc.Dial(in.RuntimeMeta["stats_addr"], grpc.WithInsecure())
    if err != nil {
        return err
    }
    c.conn = conn
    c.client = stats.NewStatsServiceClient(conn)
    c.lastValues = make(map[string]int64)
    return nil
}

func (c *XrayTrafficCollector) Subscribe(ctx context.Context, ch chan<- TrafficDeltaEvent) error {
    c.mu.Lock()
    c.subscribers = append(c.subscribers, ch)
    c.mu.Unlock()
    return nil
}

// 定时轮询 Xray stats API，计算增量并分发
func (c *XrayTrafficCollector) pollLoop(ctx context.Context, interval time.Duration) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            c.collectAndEmit(ctx)
        }
    }
}

func (c *XrayTrafficCollector) collectAndEmit(ctx context.Context) {
    // 查询所有 inbound 的流量统计
    // Xray stats 格式: "inbound>>>tag>>>traffic>>>uplink"
    //                  "user>>>email>>>traffic>>>downlink"
    resp, err := c.client.QueryStats(ctx, &stats.QueryStatsRequest{
        Pattern: "user>>>*>>>traffic>>>*",
        Reset_:  true, // 查询后重置计数器，获取增量
    })
    if err != nil {
        return
    }

    for _, stat := range resp.GetStat() {
        // stat.Name 格式: "user>>>alice@example.com>>>traffic>>>uplink"
        parts := parseStatName(stat.Name) // ["user", "alice@example.com", "traffic", "uplink"]
        if len(parts) != 4 {
            continue
        }

        email := parts[1]
        direction := parts[3] // "uplink" or "downlink"

        // 通过 email 映射到 user_id 和 subscription_id
        ownership := c.resolveOwnership(email)

        event := TrafficDeltaEvent{
            SessionID:  fmt.Sprintf("xray-%s-%d", email, time.Now().UnixNano()),
            UserID:     ownership.UserID,
            ServiceID:  c.serviceID,
            CoreType:   CoreTypeXray,
            Seq:        time.Now().UnixNano(),
            OccurredAt: time.Now(),
        }

        switch direction {
        case "uplink":
            event.UplinkBytes = stat.Value
        case "downlink":
            event.DownlinkBytes = stat.Value
        }

        // 分发给所有订阅者
        c.mu.RLock()
        for _, ch := range c.subscribers {
            select {
            case ch <- event:
            default:
                // channel 满了丢弃，避免阻塞采集
            }
        }
        c.mu.RUnlock()
    }
}

// Xray stats name 解析
func parseStatName(name string) []string {
    return strings.Split(name, ">>>")
}

// email 到 user_id 的映射
func (c *XrayTrafficCollector) resolveOwnership(email string) TrafficOwnership {
    // 从数据库查询 email 对应的用户和订阅
    // email 格式约定: "user_<user_id>_sub_<subscription_id>"
    // 或通过独立的映射表查询
}
```

Xray-core 配置要求（需要在 Xray 配置中启用 stats）：

```json
{
  "stats": {},
  "api": {
    "tag": "api",
    "services": ["StatsService"]
  },
  "policy": {
    "levels": {
      "0": {
        "statsUserUplink": true,
        "statsUserDownlink": true
      }
    },
    "system": {
      "statsInboundUplink": true,
      "statsInboundDownlink": true
    }
  }
}
```

### 6.9.1.1 sing-box 采集器具体实现

sing-box 从 1.8.0 开始提供实验性 Traffic Manager API，通过 HTTP API 获取流量统计：

```go
type SingBoxTrafficCollector struct {
    serviceID   int64
    apiAddr     string // sing-box API 地址，如 "127.0.0.1:9090"
    apiSecret   string // API 认证密钥
    httpClient  *http.Client
    subscribers []chan<- TrafficDeltaEvent
    lastValues  map[string]int64
    mu          sync.RWMutex
}

func (c *SingBoxTrafficCollector) Start(ctx context.Context, in CollectorStartInput) error {
    c.apiAddr = in.RuntimeMeta["api_addr"]
    c.apiSecret = in.RuntimeMeta["api_secret"]
    c.httpClient = &http.Client{Timeout: 5 * time.Second}
    c.lastValues = make(map[string]int64)
    return nil
}

func (c *SingBoxTrafficCollector) Subscribe(ctx context.Context, ch chan<- TrafficDeltaEvent) error {
    c.mu.Lock()
    c.subscribers = append(c.subscribers, ch)
    c.mu.Unlock()
    return nil
}

// sing-box Traffic API 响应结构
type SingBoxTrafficResponse struct {
    Upload   int64 `json:"upload"`
    Download int64 `json:"download"`
}

// sing-box 接口流量查询
// GET /traffic?name=<inbound_tag>
// sing-box 用户流量查询（需要 1.9.0+ 或 experimental API）
// GET /traffic/users?name=<inbound_tag>

func (c *SingBoxTrafficCollector) pollLoop(ctx context.Context, interval time.Duration) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            c.collectAndEmit(ctx)
        }
    }
}

func (c *SingBoxTrafficCollector) collectAndEmit(ctx context.Context) {
    // 1. 查询各 inbound 的总流量
    inbounds := c.listInbounds(ctx)
    for _, inbound := range inbounds {
        traffic := c.queryInboundTraffic(ctx, inbound)
        if traffic == nil {
            continue
        }

        key := fmt.Sprintf("inbound:%s:up", inbound)
        lastUp := c.lastValues[key]
        deltaUp := traffic.Upload - lastUp
        if deltaUp > 0 {
            c.lastValues[key] = traffic.Upload
            c.emit(ctx, TrafficDeltaEvent{
                SessionID:  fmt.Sprintf("singbox-%s-%d", inbound, time.Now().UnixNano()),
                ServiceID:  c.serviceID,
                CoreType:   CoreTypeSingBox,
                UplinkBytes: deltaUp,
                OccurredAt: time.Now(),
            })
        }

        key = fmt.Sprintf("inbound:%s:down", inbound)
        lastDown := c.lastValues[key]
        deltaDown := traffic.Download - lastDown
        if deltaDown > 0 {
            c.lastValues[key] = traffic.Download
            c.emit(ctx, TrafficDeltaEvent{
                SessionID:  fmt.Sprintf("singbox-%s-%d", inbound, time.Now().UnixNano()),
                ServiceID:  c.serviceID,
                CoreType:   CoreTypeSingBox,
                DownlinkBytes: deltaDown,
                OccurredAt: time.Now(),
            })
        }
    }

    // 2. 查询用户级流量（如果 API 支持）
    users := c.queryUserTraffic(ctx)
    for _, user := range users {
        ownership := c.resolveOwnership(user.Name)
        // 计算增量并分发，逻辑同上
        c.emitDelta(ctx, ownership, user.Upload, user.Download)
    }
}

func (c *SingBoxTrafficCollector) queryInboundTraffic(ctx context.Context, name string) *SingBoxTrafficResponse {
    req, _ := http.NewRequestWithContext(ctx, "GET",
        fmt.Sprintf("http://%s/traffic?name=%s", c.apiAddr, name), nil)
    if c.apiSecret != "" {
        req.Header.Set("Authorization", "Bearer "+c.apiSecret)
    }

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        return nil
    }

    var traffic SingBoxTrafficResponse
    json.NewDecoder(resp.Body).Decode(&traffic)
    return &traffic
}

func (c *SingBoxTrafficCollector) emit(ctx context.Context, event TrafficDeltaEvent) {
    c.mu.RLock()
    for _, ch := range c.subscribers {
        select {
        case ch <- event:
        default:
        }
    }
    c.mu.RUnlock()
}
```

sing-box 配置要求（需要启用 experimental API）：

```json
{
  "experimental": {
    "traffic_api": {
      "server": {
        "listen": "127.0.0.1:9090",
        "secret": "your-api-secret"
      }
    }
  }
}
```

降级策略：

- 若 sing-box 版本不支持 Traffic API，降级为进程级统计
- 通过读取 sing-box 进程的 `/proc/<pid>/net/dev` 获取网卡级流量
- 无法区分用户，`UserID` 和 `SubscriptionID` 留空
- UI 标注"此核心流量统计为估算值"

### 6.9.1.2 Hysteria 2 采集器具体实现

Hysteria 2 内置流量统计，通过 UDP 控制台或 HTTP API 获取：

```go
type Hysteria2TrafficCollector struct {
    serviceID   int64
    controlAddr string // 控制台地址，如 "127.0.0.1:25601"
    secret      string
    httpClient  *http.Client
    subscribers []chan<- TrafficDeltaEvent
    lastValues  map[string]int64 // key: "user:<auth_user>"
    mu          sync.RWMutex
}

// Hysteria 2 控制台 API 响应
type Hy2TrafficStats struct {
    Tx int64 `json:"tx"` // 上行字节
    Rx int64 `json:"rx"` // 下行字节
}

type Hy2UserStats struct {
    Auth   string        `json:"auth"`   // 认证用户名
    Tx     int64         `json:"tx"`
    Rx     int64         `json:"rx"`
    Conns  int           `json:"conns"`  // 当前连接数
}

func (c *Hysteria2TrafficCollector) Start(ctx context.Context, in CollectorStartInput) error {
    c.controlAddr = in.RuntimeMeta["control_addr"]
    c.secret = in.RuntimeMeta["control_secret"]
    c.httpClient = &http.Client{Timeout: 5 * time.Second}
    c.lastValues = make(map[string]int64)
    return nil
}

func (c *Hysteria2TrafficCollector) pollLoop(ctx context.Context, interval time.Duration) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            c.collectAndEmit(ctx)
        }
    }
}

func (c *Hysteria2TrafficCollector) collectAndEmit(ctx context.Context) {
    // Hysteria 2 提供 /traffic API 端点
    // GET http://<control_addr>/traffic?clear=true
    // clear=true 表示查询后重置计数器（获取增量）
    users := c.queryAllUsers(ctx)

    for _, user := range users {
        ownership := c.resolveOwnership(user.Auth)

        // Hysteria 2 的 Tx/Rx 已经是增量（clear=true）
        if user.Tx > 0 || user.Rx > 0 {
            event := TrafficDeltaEvent{
                SessionID:     fmt.Sprintf("hy2-%s-%d", user.Auth, time.Now().UnixNano()),
                UserID:        ownership.UserID,
                SubscriptionID: ownership.SubscriptionID,
                ServiceID:     c.serviceID,
                EndpointID:    ownership.EndpointID,
                CoreType:      CoreTypeHysteria2,
                Protocol:      ProtocolHysteria2,
                UplinkBytes:   user.Tx,
                DownlinkBytes: user.Rx,
                Seq:           time.Now().UnixNano(),
                OccurredAt:    time.Now(),
            }

            c.mu.RLock()
            for _, ch := range c.subscribers {
                select {
                case ch <- event:
                default:
                }
            }
            c.mu.RUnlock()
        }
    }
}

func (c *Hysteria2TrafficCollector) queryAllUsers(ctx context.Context) []Hy2UserStats {
    req, _ := http.NewRequestWithContext(ctx, "GET",
        fmt.Sprintf("http://%s/traffic?clear=true", c.controlAddr), nil)
    if c.secret != "" {
        req.Header.Set("Authorization", c.secret)
    }

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil
    }
    defer resp.Body.Close()

    var users []Hy2UserStats
    json.NewDecoder(resp.Body).Decode(&users)
    return users
}

// Hysteria 2 auth 用户名到 user_id 的映射
// 约定：auth 用户名格式为 "u<user_id>s<subscription_id>"
// 例如：auth = "u1001s2001" → user_id=1001, subscription_id=2001
func (c *Hysteria2TrafficCollector) resolveOwnership(auth string) TrafficOwnership {
    ownership := TrafficOwnership{}
    // 解析约定格式
    if strings.HasPrefix(auth, "u") {
        parts := strings.Split(auth, "s")
        if len(parts) == 2 {
            uid, _ := strconv.ParseInt(parts[0][1:], 10, 64)
            sid, _ := strconv.ParseInt(parts[1], 10, 64)
            ownership.UserID = uid
            ownership.SubscriptionID = sid
        }
    }
    // 若格式不匹配，查数据库映射表
    return ownership
}
```

Hysteria 2 配置要求（需要启用控制台）：

```yaml
# hysteria2 服务端配置
listen: :443
tls:
  cert: /path/to/cert.pem
  key: /path/to/key.pem
auth:
  type: userpass
  userpass:
    u1001s2001: password1  # 用户名格式: u<user_id>s<sub_id>
    u1002s2002: password2
# 启用控制台
trafficStats:
  listen: 127.0.0.1:25601
  secret: your-control-secret
```

关键点：

- Hysteria 2 使用 `clear=true` 参数，查询后自动重置计数器，天然支持增量计算
- auth 用户名需要约定格式以便映射到 user_id/subscription_id
- QUIC 会话级别的流量已经包含多路复用流的汇总

### 6.9.1.3 TUIC 采集器具体实现

TUIC 的管理接口能力有限，第一版采用混合策略：

```go
type TUICTrafficCollector struct {
    serviceID   int64
    controlAddr string // 控制台地址
    httpClient  *http.Client
    subscribers []chan<- TrafficDeltaEvent
    // TUIC 用户级统计能力有限，使用进程级 + 用户级混合
    processLastBytes int64
    mu               sync.RWMutex
}

func (c *TUICTrafficCollector) Start(ctx context.Context, in CollectorStartInput) error {
    c.controlAddr = in.RuntimeMeta["control_addr"]
    c.httpClient = &http.Client{Timeout: 5 * time.Second}
    return nil
}

func (c *TUICTrafficCollector) pollLoop(ctx context.Context, interval time.Duration) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            c.collectAndEmit(ctx)
        }
    }
}

func (c *TUICTrafficCollector) collectAndEmit(ctx context.Context) {
    // 方案 1：尝试查询 TUIC 控制台 API（如果支持）
    stats := c.queryControlAPI(ctx)
    if stats != nil && len(stats.Users) > 0 {
        // 控制台返回了用户级统计
        for _, user := range stats.Users {
            ownership := c.resolveOwnership(user.Token)
            if user.TxBytes > 0 || user.RxBytes > 0 {
                c.emitDelta(ctx, ownership, user.TxBytes, user.RxBytes)
            }
        }
        return
    }

    // 方案 2：降级为进程级统计
    // 读取 TUIC 进程监听的端口的系统级流量
    totalBytes := c.readProcessNetworkStats(ctx)
    delta := totalBytes - c.processLastBytes
    if delta > 0 {
        c.processLastBytes = totalBytes
        c.emit(ctx, TrafficDeltaEvent{
            SessionID:    fmt.Sprintf("tuic-proc-%d", c.serviceID),
            ServiceID:    c.serviceID,
            CoreType:     CoreTypeTUIC,
            UplinkBytes:  delta / 2,  // 无法区分上下行，平分估算
            DownlinkBytes: delta / 2,
            Seq:          time.Now().UnixNano(),
            OccurredAt:   time.Now(),
            // UserID 和 SubscriptionID 留空，表示进程级统计
        })
    }
}

// 从 /proc/<pid>/net/dev 读取进程网络统计
func (c *TUICTrafficCollector) readProcessNetworkStats(ctx context.Context) int64 {
    pid := c.getTUICPID(ctx)
    if pid == 0 {
        return 0
    }

    data, err := os.ReadFile(fmt.Sprintf("/proc/%d/net/dev", pid))
    if err != nil {
        return 0
    }

    var totalBytes int64
    for _, line := range strings.Split(string(data), "\n") {
        fields := strings.Fields(line)
        if len(fields) < 10 || !strings.Contains(fields[0], ":") {
            continue
        }
        // 跳过 lo 接口
        if strings.HasPrefix(fields[0], "lo:") {
            continue
        }
        rx, _ := strconv.ParseInt(fields[1], 10, 64)
        tx, _ := strconv.ParseInt(fields[9], 10, 64)
        totalBytes += rx + tx
    }
    return totalBytes
}

func (c *TUICTrafficCollector) emitDelta(ctx context.Context, ownership TrafficOwnership, uplink, downlink int64) {
    event := TrafficDeltaEvent{
        SessionID:      fmt.Sprintf("tuic-%d-%d", ownership.UserID, time.Now().UnixNano()),
        UserID:         ownership.UserID,
        SubscriptionID: ownership.SubscriptionID,
        ServiceID:      c.serviceID,
        CoreType:       CoreTypeTUIC,
        Protocol:       ProtocolTUIC,
        UplinkBytes:    uplink,
        DownlinkBytes:  downlink,
        Seq:            time.Now().UnixNano(),
        OccurredAt:     time.Now(),
    }

    c.mu.RLock()
    for _, ch := range c.subscribers {
        select {
        case ch <- event:
        default:
        }
    }
    c.mu.RUnlock()
}
```

TUIC 第一版策略：

- 优先尝试 TUIC 控制台 API（如果版本支持用户级统计）
- 不支持时降级为进程级统计（读取 `/proc/<pid>/net/dev`）
- 进程级统计无法区分用户，`UserID` 和 `SubscriptionID` 留空
- 进程级统计无法区分上下行，按平分估算
- UI 中必须标注"此核心流量统计为估算值"
- 后续 TUIC 版本如果增加管理 API，适配器只需升级 `queryControlAPI` 方法

### 6.9.1.4 四核心采集器对比总结

| 核心 | 统计粒度 | 增量获取方式 | 上下行区分 | 用户归属 | 降级方案 |
| --- | --- | --- | --- | --- | --- |
| Xray-core | 用户级 | `QueryStats(reset_=true)` | 区分 | email 映射 | 无，API 稳定 |
| sing-box | 用户级（实验性） | HTTP API 轮询 | 区分 | 用户名映射 | 进程级 `/proc/net/dev` |
| Hysteria 2 | 用户级 | HTTP API `clear=true` | 区分 | auth 用户名约定格式 | 进程级 `/proc/net/dev` |
| TUIC | 进程级（第一版） | `/proc/<pid>/net/dev` | 平分估算 | 无（留空） | 后续升级控制台 API |

用户归属映射方案：

- Xray-core：email 格式约定 `u<user_id>s<sub_id>@corefusion`
- sing-box：inbound tag 或用户名格式约定
- Hysteria 2：auth 用户名格式约定 `u<user_id>s<sub_id>`
- TUIC：token 格式约定（当 API 支持后）
- 所有核心都支持通过独立映射表查询（当约定格式不适用时）

### 6.9.1.5 流量计数方案

面板不在流量路径上，但有一个关键信息来源：**订阅 URL 访问**。

当客户端访问 `GET /sub/:token` 拉取订阅配置时，面板可以：
1. 拿到客户端 IP（真实 IP 或 CDN IP）
2. 通过 token 关联到 subscription_id 和 user_id
3. 建立 IP → 订阅 的映射关系

#### CDN 场景检测

客户端可能直连核心，也可能经过 CDN 加速。面板需要区分这两种情况：

```go
type AccessInfo struct {
    RealIP    string // 从 X-Forwarded-For / CF-Connecting-IP 提取
    SourceIP  string // TCP 连接来源 IP（可能是 CDN IP）
    IsCDN     bool   // 是否经过 CDN
    CDNProvider string // CDN 提供商（cloudflare, cloudfront, etc）
}

func (h *SubscriptionHandler) detectAccess(c *gin.Context) AccessInfo {
    sourceIP := c.ClientIP()

    // 检查常见 CDN 头
    cfIP := c.GetHeader("CF-Connecting-IP")
    xff := c.GetHeader("X-Forwarded-For")
    xrIP := c.GetHeader("X-Real-IP")

    realIP := sourceIP
    isCDN := false
    cdnProvider := ""

    if cfIP != "" {
        realIP = cfIP
        isCDN = true
        cdnProvider = "cloudflare"
    } else if xrIP != "" && xrIP != sourceIP {
        realIP = xrIP
        isCDN = true
        cdnProvider = "generic_proxy"
    } else if xff != "" {
        // X-Forwarded-For: client, proxy1, proxy2
        parts := strings.Split(xff, ",")
        firstIP := strings.TrimSpace(parts[0])
        if firstIP != sourceIP {
            realIP = firstIP
            isCDN = true
            cdnProvider = "generic_proxy"
        }
    }

    // 也可以通过 IP 段判断是否为已知 CDN
    if !isCDN && h.isCDNIP(sourceIP) {
        isCDN = true
        cdnProvider = h.identifyCDN(sourceIP)
    }

    return AccessInfo{
        RealIP:    realIP,
        SourceIP:  sourceIP,
        IsCDN:     isCDN,
        CDNProvider: cdnProvider,
    }
}

// 已知 CDN IP 段（Cloudflare, AWS CloudFront, Akamai 等）
func (h *SubscriptionHandler) isCDNIP(ip string) bool {
    // 从本地缓存的 CDN IP 段列表中查询
    // Cloudflare IP 段: 173.245.48.0/20, 103.21.244.0/22, ...
    // 定期从 CDN 官方更新
}
```

#### 双模式流量计数

根据是否经过 CDN，采用不同的计数策略：

```text
直连场景（面板看到真实 IP）：
    用户 IP 1.2.3.4 ──────────────────▶ 核心
    │
    │ 面板记录: 1.2.3.4 → subscription_1001
    │ nftables: sub_1001 ip saddr 1.2.3.4 counter
    │ 计数方式: nftables（精确，实时）
    ▼

CDN 场景（面板看到 CDN IP）：
    用户 ──▶ CDN IP 104.16.0.1 ──────▶ 核心
    │
    │ 面板记录: 104.16.0.1 → CDN（不能关联到具体订阅）
    │ nftables: 只能按 CDN IP 计数（聚合值，不精确）
    │ 计数方式: 核心 Stats API（按用户 email/tag 精确计数）
    ▼
```

```go
type TrafficCollector struct {
    ipTracker    *SubscriptionIPTracker  // nftables 方案
    coreReaders  map[CoreType]CoreStatsReader // 核心 API 方案
    db           *gorm.DB
}

func (c *TrafficCollector) RecordSubscriptionAccess(subID, userID int64, info AccessInfo) {
    if !info.IsCDN {
        // 直连：记录真实 IP，用 nftables 计数
        c.ipTracker.RecordAccess(subID, userID, info.RealIP)
        c.db.Save(&SubscriptionIPMapping{
            SubscriptionID: subID,
            UserID:         userID,
            IP:             info.RealIP,
            AccessType:     "direct",
        })
    } else {
        // CDN：记录 CDN IP 和真实 IP，用核心 API 计数
        c.db.Save(&SubscriptionIPMapping{
            SubscriptionID: subID,
            UserID:         userID,
            IP:             info.RealIP,      // 真实 IP（从 header 提取）
            CDNIP:          info.SourceIP,     // CDN IP
            CDNProvider:    info.CDNProvider,
            AccessType:     "cdn",
        })
        // nftables 只能按 CDN IP 做聚合计数，不能精确到用户
        // 精确计数依赖核心 Stats API
    }
}

// 流量采集：根据接入方式选择计数源
func (c *TrafficCollector) CollectTraffic(ctx context.Context) {
    // 1. 直连订阅：从 nftables 读取
    directSubs := c.db.FindDirectSubscriptions()
    for _, sub := range directSubs {
        bytes := c.ipTracker.ReadSubscriptionTraffic(sub.ID)
        if bytes > 0 {
            c.emitEvent(sub.UserID, sub.ID, bytes, "nftables")
        }
    }

    // 2. CDN 订阅：从核心 Stats API 读取
    cdnSubs := c.db.FindCDNSubscriptions()
    for _, sub := range cdnSubs {
        reader := c.coreReaders[sub.CoreType]
        if reader == nil {
            continue
        }
        deltas := reader.ReadStats(ctx)
        for _, delta := range deltas {
            ownership := c.resolveOwnership(delta.UserIdentifier)
            if ownership.SubscriptionID == sub.ID {
                c.emitEvent(ownership.UserID, sub.ID,
                    delta.UplinkBytes+delta.DownlinkBytes, "core_api")
            }
        }
    }
}
```

#### CDN 场景的超额拦截

CDN 场景下 nftables 不能按真实 IP 拦截（面板看不到真实 IP 的流量），需要两种方式配合：

```go
func (e *QuotaEnforcer) OnQuotaExceeded(ctx context.Context, subID int64, state QuotaState) {
    sub := e.db.FindSubscription(subID)

    if sub.AccessType == "direct" {
        // 直连：nftables 拦截（精确到 IP）
        e.ipTracker.BlockSubscription(subID)
        if state == QuotaStateOverLimitHard || state == QuotaStateSuspended {
            e.flushConntrack(subID)
        }
    } else {
        // CDN：核心 API 拦截（移除用户配置）
        // nftables 只能按 CDN IP 拦截，会影响同 CDN 的其他用户，不能用
        user := e.db.FindUser(sub.UserID)
        manager := e.coreManagers[user.CoreType]
        manager.RemoveUser(ctx, user.ServiceID, user.CoreIdentifier)
    }
}
```

#### 两种场景对比

| 场景 | 流量计数 | 超额拦截 | 同订阅人数 |
| --- | --- | --- | --- |
| 直连 | nftables 按 IP 精确计数 | nftables flush + conntrack | 活跃 IP 数 |
| CDN | 核心 Stats API 按用户计数 | 核心 API 移除用户 | 订阅访问日志中的不同真实 IP 数 |

#### 订阅访问日志增强

无论直连还是 CDN，都记录完整的访问信息：

```sql
-- 扩展 subscription_access_logs 表
ALTER TABLE subscription_access_logs ADD COLUMN real_ip TEXT;
ALTER TABLE subscription_access_logs ADD COLUMN cdn_ip TEXT;
ALTER TABLE subscription_access_logs ADD COLUMN cdn_provider TEXT;
ALTER TABLE subscription_access_logs ADD COLUMN access_type TEXT; -- 'direct' 或 'cdn'
```

```go
// 订阅访问时记录
func (h *SubscriptionHandler) HandleSubRequest(c *gin.Context) {
    info := h.detectAccess(c)

    h.auditRepo.Save(&SubscriptionAccessLog{
        SubscriptionID: sub.ID,
        IP:             info.RealIP,
        RealIP:         info.RealIP,
        CDNIP:          info.SourceIP,
        CDNProvider:    info.CDNProvider,
        AccessType:     accessType(info),
        UserAgent:      c.GetHeader("User-Agent"),
        StatusCode:     200,
        HitResult:      "served",
    })

    // 根据接入方式选择计数策略
    h.trafficCollector.RecordSubscriptionAccess(sub.ID, sub.UserID, info)
}

func accessType(info AccessInfo) string {
    if info.IsCDN {
        return "cdn"
    }
    return "direct"
}
```

#### 总结

| 计数方式 | 适用场景 | 精度 | 实时性 |
| --- | --- | --- | --- |
| nftables 按订阅 IP | 直连 | 精确到 IP | 实时 |
| 核心 Stats API | CDN 或 nftables 不可用 | 精确到用户 | 30 秒轮询 |

面板自动检测接入方式，选择最优计数策略。用户不需要手动配置。

```text
客户端访问面板 GET /sub/:token
    │
    │ 面板记录: client_ip → subscription_id → user_id
    ▼
面板生成 nftables 规则:
    │  nft add rule inet corefusion sub_<sub_id> ip saddr <client_ip> counter
    ▼
核心处理客户端的代理连接（流量经过 nftables）
    │
    │ nftables counter 自动计数
    ▼
面板定时读取 nftables 计数器
    │
    │ 按 subscription_id 聚合所有 IP 的流量
    ▼
配额状态机 → 超额时删除该订阅所有 IP 的 nftables 规则
```

完整数据流：

```go
// 订阅访问时记录 IP 映射
func (h *SubscriptionHandler) HandleSubRequest(c *gin.Context) {
    token := c.Param("token")
    clientIP := c.ClientIP()

    // 查找订阅
    sub := h.subRepo.FindByToken(token)
    if sub == nil {
        c.JSON(404, gin.H{"error": "not found"})
        return
    }

    // 记录 IP → 订阅映射
    h.ipTracker.RecordAccess(sub.ID, sub.UserID, clientIP)

    // 返回订阅内容（正常业务逻辑）
    content := h.renderer.Render(sub)
    c.Data(200, sub.ContentType(), content)
}
```

nftables 规则管理：

```go
type SubscriptionIPTracker struct {
    tableName string // "corefusion"
    // 内存缓存: subscription_id -> []IPRecord
    ipMap sync.Map
}

type IPRecord struct {
    IP            string
    SubscriptionID int64
    UserID        int64
    FirstSeen     time.Time
    LastSeen      time.Time
}

// 记录订阅访问 IP
func (t *SubscriptionIPTracker) RecordAccess(subID, userID int64, ip string) {
    key := fmt.Sprintf("sub:%d", subID)

    v, _ := t.ipMap.LoadOrStore(key, &sync.Map{})
    ipMap := v.(*sync.Map)

    now := time.Now()
    ipMap.Store(ip, &IPRecord{
        IP:            ip,
        SubscriptionID: subID,
        UserID:        userID,
        FirstSeen:     now,
        LastSeen:      now,
    })

    // 同步到 nftables
    t.addNftRule(subID, ip)
}

// 添加 nftables 计数规则
func (t *SubscriptionIPTracker) addNftRule(subID int64, ip string) {
    // nft add rule inet corefusion sub_<sub_id> ip saddr <ip> counter accept
    chainName := fmt.Sprintf("sub_%d", subID)

    // 确保链存在
    exec.Command("nft", "add", "chain", "inet", t.tableName, chainName).Run()

    // 添加计数规则
    exec.Command("nft", "add", "rule", "inet", t.tableName, chainName,
        "ip", "saddr", ip, "counter", "accept").Run()
}

// 读取订阅的总流量
func (t *SubscriptionIPTracker) ReadTraffic(subID int64) (uplink, downlink int64) {
    chainName := fmt.Sprintf("sub_%d", subID)

    // nft list chain inet corefusion sub_<sub_id> -j
    out, err := exec.Command("nft", "-j", "list", "chain", "inet", t.tableName, chainName).Output()
    if err != nil {
        return 0, 0
    }

    // 解析 JSON，累加所有规则的 counter
    counters := t.parseCounters(out)
    for _, c := range counters {
        uplink += c.Bytes
    }
    return uplink, downlink
}

// 定时轮询所有订阅的流量（每 30 秒）
func (t *SubscriptionIPTracker) PollAll(ctx context.Context, interval time.Duration) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            t.ipMap.Range(func(key, value interface{}) bool {
                subID := t.parseSubID(key.(string))
                uplink, _ := t.ReadTraffic(subID)
                if uplink > 0 {
                    // 发送流量事件到聚合器
                    t.emitTrafficEvent(subID, uplink)
                }
                return true
            })
        }
    }
}

// 超额拦截：删除订阅所有 IP 的 nftables 规则
func (t *SubscriptionIPTracker) BlockSubscription(subID int64) {
    chainName := fmt.Sprintf("sub_%d", subID)

    // 直接 flush 整个链（删除所有规则）
    exec.Command("nft", "flush", "chain", "inet", t.tableName, chainName).Run()

    // 添加默认 drop 规则
    exec.Command("nft", "add", "rule", "inet", t.tableName, chainName, "drop").Run()
}

// 恢复：删除 drop 规则，重新添加 IP 的计数规则
func (t *SubscriptionIPTracker) UnblockSubscription(subID int64) {
    chainName := fmt.Sprintf("sub_%d", subID)

    // flush 所有规则
    exec.Command("nft", "flush", "chain", "inet", t.tableName, chainName).Run()

    // 从缓存中恢复所有 IP 的计数规则
    key := fmt.Sprintf("sub:%d", subID)
    if v, ok := t.ipMap.Load(key); ok {
        ipMap := v.(*sync.Map)
        ipMap.Range(func(ip, record interface{}) bool {
            t.addNftRule(subID, ip.(string))
            return true
        })
    }
}
```

nftables 链结构：

```bash
# 每个订阅一条链
nft add chain inet corefusion sub_1001
nft add chain inet corefusion sub_1002

# 订阅的 IP 计数规则（动态添加）
nft add rule inet corefusion sub_1001 ip saddr 1.2.3.4 counter accept
nft add rule inet corefusion sub_1001 ip saddr 5.6.7.8 counter accept

# 超额时：flush 链 + 添加 drop
# nft flush chain inet corefusion sub_1001
# nft add rule inet corefusion sub_1001 drop

# 恢复时：flush 链 + 重新添加 IP 计数规则
```

多设备同订阅的处理：

```go
// 同一订阅多个设备（多个 IP）的流量合并统计
func (t *SubscriptionIPTracker) ReadSubscriptionTraffic(subID int64) int64 {
    key := fmt.Sprintf("sub:%d", subID)
    v, ok := t.ipMap.Load(key)
    if !ok {
        return 0
    }

    ipMap := v.(*sync.Map)
    var totalBytes int64

    ipMap.Range(func(ip, record interface{}) bool {
        // 读取该 IP 的 nftables 计数器
        bytes := t.readIPCounter(subID, ip.(string))
        totalBytes += bytes
        return true
    })

    return totalBytes
}
```

同订阅人数监控：

```go
// 统计同一订阅的活跃 IP 数（同订阅使用人数）
func (t *SubscriptionIPTracker) ActiveDeviceCount(subID int64) int {
    key := fmt.Sprintf("sub:%d", subID)
    v, ok := t.ipMap.Load(key)
    if !ok {
        return 0
    }

    ipMap := v.(*sync.Map)
    count := 0
    now := time.Now()

    ipMap.Range(func(ip, record interface{}) bool {
        r := record.(*IPRecord)
        // 只统计最近 24 小时内活跃的 IP
        if now.Sub(r.LastSeen) < 24*time.Hour {
            count++
        }
        return true
    })

    return count
}

// API: 查询订阅使用情况
// GET /api/v1/subscriptions/:id/usage
// 响应: { "active_devices": 3, "total_bytes": 1073741824, "ips": ["1.2.3.4", "5.6.7.8"] }
```

这个方案的优势：

| 对比项 | 核心 Stats API | 订阅 IP + nftables |
| --- | --- | --- |
| 用户归属 | 依赖核心 API 的 email/tag | 面板自己通过订阅 token 关联 |
| 跨核心统一 | 每个核心 API 不同 | nftables 统一，不区分核心 |
| 实时性 | 30 秒轮询延迟 | nftables 实时计数 |
| 多设备统计 | 需要核心支持 | 自然按 IP 聚合 |
| 超额拦截 | 调用核心 API 移除用户 | nftables flush 链，立即生效 |
| 同订阅人数 | 无法统计 | 活跃 IP 数 = 使用人数 |

降级补充：

- 核心 Stats API 仍作为交叉校验手段
- nftables 计数和核心 API 计数对比，偏差超 10% 告警
- nftables 不可用时（macOS/Windows）降级为核心 Stats API

### 6.9.1.6 流量计数性能保证

面板轮询 Stats API 是旁路操作，不影响用户连接：

- 轮询间隔 30 秒，每次耗时 < 100ms（本地 gRPC/HTTP）
- 轮询在独立 goroutine 中执行，不阻塞任何连接处理
- Stats API 查询失败只记录错误，不影响正常流量
- 流量统计有短暂延迟（最多 30 秒）是可接受的

### 6.9.2 聚合器具体实现

聚合器负责将增量事件写入数据库，并更新汇总：

```go
type TrafficAggregatorImpl struct {
    db          *gorm.DB
    quotaSvc    QuotaService
    eventCh     chan TrafficDeltaEvent
    batchSize   int           // 批量写入大小，默认 100
    flushInterval time.Duration // 刷盘间隔，默认 5 秒
}

func (a *TrafficAggregatorImpl) Start(ctx context.Context) {
    go a.consumeLoop(ctx)
}

func (a *TrafficAggregatorImpl) consumeLoop(ctx context.Context) {
    batch := make([]TrafficDeltaEvent, 0, a.batchSize)
    ticker := time.NewTicker(a.flushInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            a.flush(ctx, batch)
            return
        case event := <-a.eventCh:
            batch = append(batch, event)
            if len(batch) >= a.batchSize {
                a.flush(ctx, batch)
                batch = batch[:0]
            }
        case <-ticker.C:
            if len(batch) > 0 {
                a.flush(ctx, batch)
                batch = batch[:0]
            }
        }
    }
}

func (a *TrafficAggregatorImpl) flush(ctx context.Context, events []TrafficDeltaEvent) {
    if len(events) == 0 {
        return
    }

    a.db.Transaction(func(tx *gorm.DB) error {
        // 1. 写入账本（幂等，session_id + seq 唯一键冲突则跳过）
        for _, e := range events {
            ledger := TrafficUsageLedger{
                SessionID:      e.SessionID,
                Seq:            e.Sequence,
                UserID:         e.UserID,
                SubscriptionID: e.SubscriptionID,
                ServiceID:      e.ServiceID,
                CoreType:       string(e.CoreType),
                Protocol:       string(e.Protocol),
                UplinkBytes:    e.UplinkBytes,
                DownlinkBytes:  e.DownlinkBytes,
                BilledBytes:    e.UplinkBytes + e.DownlinkBytes,
                OccurredAt:     e.OccurredAt,
            }
            tx.Where("session_id = ? AND seq = ?", e.SessionID, e.Sequence).
                FirstOrCreate(&ledger)
        }

        // 2. 更新汇总表
        for _, e := range events {
            if e.SubscriptionID > 0 {
                a.upsertSummary(tx, "subscription", e.SubscriptionID,
                    e.UplinkBytes, e.DownlinkBytes)
            }
            if e.UserID > 0 {
                a.upsertSummary(tx, "user", e.UserID,
                    e.UplinkBytes, e.DownlinkBytes)
            }
        }

        // 3. 更新 subscriptions 表的 used_bytes_cache
        subDeltas := a.aggregateBySubscription(events)
        for subID, delta := range subDeltas {
            tx.Model(&Subscription{}).
                Where("id = ?", subID).
                Update("used_bytes_cache", gorm.Expr("used_bytes_cache + ?", delta))
        }

        return nil
    })

    // 4. 刷新配额状态缓存
    for _, e := range events {
        if e.SubscriptionID > 0 {
            a.quotaSvc.RefreshSnapshot(ctx, "subscription", e.SubscriptionID)
        }
    }
}

func (a *TrafficAggregatorImpl) upsertSummary(tx *gorm.DB, subjectType string, subjectID int64, uplink, downlink int64) {
    now := time.Now()
    periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
    periodEnd := periodStart.AddDate(0, 1, 0)

    tx.Where("subject_type = ? AND subject_id = ? AND period_start = ?",
        subjectType, subjectID, periodStart).
        Assign(map[string]interface{}{
            "uplink_bytes":   gorm.Expr("uplink_bytes + ?", uplink),
            "downlink_bytes": gorm.Expr("downlink_bytes + ?", downlink),
            "billed_bytes":   gorm.Expr("billed_bytes + ?", uplink+downlink),
            "updated_at":     now,
        }).
        FirstOrCreate(&TrafficUsageSummary{
            SubjectType:  subjectType,
            SubjectID:    subjectID,
            PeriodStart:  periodStart,
            PeriodEnd:    periodEnd,
            UplinkBytes:  uplink,
            DownlinkBytes: downlink,
            BilledBytes:  uplink + downlink,
        })
}
```

### 6.9.3 配额检查与连接拦截实现

配额检查通过内存缓存实现，避免每次连接都查数据库：

```go
type QuotaCache struct {
    snapshots map[string]*QuotaSnapshot // key: "subscription:123"
    mu        sync.RWMutex
}

type QuotaServiceImpl struct {
    cache     *QuotaCache
    db        *gorm.DB
    machine   QuotaStateMachine
}

// 刷新配额快照（聚合器写入后调用）
func (q *QuotaServiceImpl) RefreshSnapshot(ctx context.Context, subjectType string, subjectID int64) {
    key := fmt.Sprintf("%s:%d", subjectType, subjectID)

    var summary TrafficUsageSummary
    q.db.Where("subject_type = ? AND subject_id = ? AND period_start = ?",
        subjectType, subjectID, currentPeriodStart()).
        First(&summary)

    var quota TrafficQuota
    q.db.Where("subject_type = ? AND subject_id = ?", subjectType, subjectID).
        First(&quota)

    snapshot := QuotaSnapshot{
        SubjectType: subjectType,
        SubjectID:   subjectID,
        UsedBytes:   summary.BilledBytes,
        QuotaBytes:  quota.QuotaBytes,
        GraceBytes:  quota.GraceBytes,
        UsageRatio:  float64(summary.BilledBytes) / float64(quota.QuotaBytes+quota.GraceBytes),
        RejectMode:  rejectModeFromBool(quota.HardLimit),
        UpdatedAt:   time.Now(),
    }
    snapshot.State = q.machine.Evaluate(snapshot)

    q.cache.mu.Lock()
    q.cache.snapshots[key] = &snapshot
    q.cache.mu.Unlock()
}

// 连接前检查（热路径，必须快）
func (q *QuotaServiceImpl) CheckConnection(ctx context.Context, subjectType string, subjectID int64) error {
    key := fmt.Sprintf("%s:%d", subjectType, subjectID)

    q.cache.mu.RLock()
    snapshot, ok := q.cache.snapshots[key]
    q.cache.mu.RUnlock()

    if !ok {
        // 缓存未命中，查数据库
        snapshot = q.loadFromDB(ctx, subjectType, subjectID)
    }

    state := q.machine.Evaluate(snapshot)
    switch state {
    case QuotaStateOverLimitSoft:
        return ErrQuotaSoftRejected
    case QuotaStateOverLimitHard:
        return ErrQuotaHardRejected
    case QuotaStateSuspended:
        return ErrQuotaSuspended
    default:
        return nil
    }
}
```

### 6.9.4 限速实现（令牌桶）

服务端入口限速使用令牌桶算法，纯内存实现：

```go
type TokenBucket struct {
    rate       float64   // 每秒补充令牌数
    maxTokens  float64   // 桶容量
    tokens     float64   // 当前令牌数
    lastRefill time.Time // 上次补充时间
    mu         sync.Mutex
}

func NewTokenBucket(rate float64, maxTokens float64) *TokenBucket {
    return &TokenBucket{
        rate:       rate,
        maxTokens:  maxTokens,
        tokens:     maxTokens,
        lastRefill: time.Now(),
    }
}

func (b *TokenBucket) Allow() bool {
    b.mu.Lock()
    defer b.mu.Unlock()

    now := time.Now()
    elapsed := now.Sub(b.lastRefill).Seconds()
    b.tokens += elapsed * b.rate
    if b.tokens > b.maxTokens {
        b.tokens = b.maxTokens
    }
    b.lastRefill = now

    if b.tokens >= 1 {
        b.tokens--
        return true
    }
    return false
}

// 限速管理器，按 service:endpoint:ip 维度维护令牌桶
type RateLimitManager struct {
    buckets map[string]*TokenBucket // key: "svc:1:ep:12:ip:1.2.3.4"
    mu      sync.RWMutex
}

func (m *RateLimitManager) CheckNewConnection(serviceID, endpointID int64, ip string, policy ServiceAccessPolicy) error {
    key := fmt.Sprintf("svc:%d:ep:%d:ip:%s", serviceID, endpointID, ip)

    m.mu.RLock()
    bucket, ok := m.buckets[key]
    m.mu.RUnlock()

    if !ok {
        m.mu.Lock()
        // 双重检查
        if bucket, ok = m.buckets[key]; !ok {
            rate := float64(policy.MaxNewConnectionsPerMin) / 60.0
            bucket = NewTokenBucket(rate, rate*10) // burst 容量为 10 秒的量
            m.buckets[key] = bucket
        }
        m.mu.Unlock()
    }

    if !bucket.Allow() {
        return ErrAccessRateLimited
    }
    return nil
}
```

### 6.9.5 并发连接计数实现

使用原子计数器跟踪每 IP 的并发连接数：

```go
type ConcurrentConnectionTracker struct {
    counters map[string]*atomic.Int64 // key: "svc:1:ep:12:ip:1.2.3.4"
    mu       sync.RWMutex
}

func (t *ConcurrentConnectionTracker) OnConnect(serviceID, endpointID int64, ip string, maxConcurrent int) error {
    key := fmt.Sprintf("svc:%d:ep:%d:ip:%s", serviceID, endpointID, ip)

    t.mu.RLock()
    counter, ok := t.counters[key]
    t.mu.RUnlock()

    if !ok {
        t.mu.Lock()
        if counter, ok = t.counters[key]; !ok {
            counter = &atomic.Int64{}
            t.counters[key] = counter
        }
        t.mu.Unlock()
    }

    current := counter.Load()
    if current >= int64(maxConcurrent) {
        return ErrAccessConcurrentLimited
    }
    counter.Add(1)
    return nil
}

func (t *ConcurrentConnectionTracker) OnDisconnect(serviceID, endpointID int64, ip string) {
    key := fmt.Sprintf("svc:%d:ep:%d:ip:%s", serviceID, endpointID, ip)

    t.mu.RLock()
    counter, ok := t.counters[key]
    t.mu.RUnlock()

    if ok {
        counter.Add(-1)
    }
}
```

### 6.9.6 自动封禁实现

当 IP 连续触发限速时自动封禁：

```go
type AutoBanManager struct {
    violations map[string]*ViolationCounter // key: "svc:1:ep:12:ip:1.2.3.4"
    banRepo    ServiceIPBanRepository
    mu         sync.Mutex
}

type ViolationCounter struct {
    Count     int
    FirstAt   time.Time
    LastAt    time.Time
    BannedAt  *time.Time
}

func (m *AutoBanManager) OnRateLimitHit(ctx context.Context, serviceID, endpointID int64, ip string, policy ServiceAccessPolicy) {
    if !policy.AutoBanEnabled {
        return
    }

    key := fmt.Sprintf("svc:%d:ep:%d:ip:%s", serviceID, endpointID, ip)

    m.mu.Lock()
    vc, ok := m.violations[key]
    if !ok {
        vc = &ViolationCounter{FirstAt: time.Now()}
        m.violations[key] = vc
    }
    vc.Count++
    vc.LastAt = time.Now()
    count := vc.Count
    m.mu.Unlock()

    // 5 分钟内触发限速超过 10 次，自动封禁
    if count >= 10 && time.Since(vc.FirstAt) < 5*time.Minute {
        ban := ServiceIPBan{
            ServiceID:  serviceID,
            EndpointID: &endpointID,
            IPCIDR:     ip + "/32",
            Reason:     "auto_ban: rate limit exceeded 10 times in 5 minutes",
            Source:     "auto_rate_limit",
            ExpiresAt:  timePtr(time.Now().Add(time.Duration(policy.BanDurationSeconds) * time.Second)),
        }
        m.banRepo.Create(ctx, &ban)

        m.mu.Lock()
        now := time.Now()
        vc.BannedAt = &now
        vc.Count = 0 // 重置计数
        m.mu.Unlock()
    }
}

// 定时清理过期封禁
func (m *AutoBanManager) CleanupExpired(ctx context.Context) {
    m.banRepo.DeleteExpired(ctx, time.Now())
}
```

### 6.9.7 完整连接检查流程

连接到达时的完整检查链路：

```go
// 挂接在核心认证成功前后的边界
func (s *ServiceAccessServiceImpl) CheckConnection(ctx context.Context, ac AccessContext) error {
    // 1. 查白名单（缓存 + DB）
    if s.isWhitelisted(ctx, ac) {
        s.audit.Write(ctx, SecurityAuditEvent{
            Category:  "service_access",
            EventType: "hit_whitelist",
            ServiceID: &ac.ServiceID,
            RemoteIP:  ac.RemoteIP,
            Result:    "allowed",
        })
        return nil
    }

    // 2. 查黑名单
    if s.isBlacklisted(ctx, ac) {
        s.audit.Write(ctx, SecurityAuditEvent{...})
        return ErrAccessDeniedByPolicy
    }

    // 3. 查临时封禁（自动封禁 + 手动封禁）
    ban, _ := s.banRepo.FindMatched(ctx, ac.ServiceID, &ac.EndpointID, ac.RemoteIP)
    if ban != nil && (ban.ExpiresAt == nil || ban.ExpiresAt.After(time.Now())) {
        s.audit.Write(ctx, SecurityAuditEvent{...})
        return ErrAccessDeniedByBan
    }

    // 4. 查并发连接限制
    policy := s.getPolicy(ctx, ac)
    if policy.MaxConcurrentConnections > 0 {
        err := s.concurrentTracker.OnConnect(ac.ServiceID, ac.EndpointID, ac.RemoteIP, policy.MaxConcurrentConnections)
        if err != nil {
            s.autoBan.OnRateLimitHit(ctx, ac.ServiceID, ac.EndpointID, ac.RemoteIP, policy)
            s.audit.Write(ctx, SecurityAuditEvent{...})
            return ErrAccessConcurrentLimited
        }
    }

    // 5. 查新建连接速率限制
    if policy.MaxNewConnectionsPerMin > 0 {
        err := s.rateLimitManager.CheckNewConnection(ac.ServiceID, ac.EndpointID, ac.RemoteIP, policy)
        if err != nil {
            s.autoBan.OnRateLimitHit(ctx, ac.ServiceID, ac.EndpointID, ac.RemoteIP, policy)
            s.audit.Write(ctx, SecurityAuditEvent{...})
            return ErrAccessRateLimited
        }
    }

    // 6. 查 GeoIP/ASN/CIDR 风控
    if s.geoRuleSvc != nil {
        decision, _ := s.geoRuleSvc.EvaluateRisk(ctx, ac.ServiceID, &ac.EndpointID, ac.RemoteIP)
        if decision != nil && !decision.Allowed {
            s.audit.Write(ctx, SecurityAuditEvent{...})
            return ErrAccessDeniedByPolicy
        }
    }

    // 7. 查配额（如果连接已经过核心认证，有 user_id）
    if ac.UserID != nil {
        err := s.quotaSvc.CheckConnection(ctx, "user", *ac.UserID)
        if err != nil {
            s.audit.Write(ctx, SecurityAuditEvent{...})
            return err
        }
    }

    // 8. 通过
    return nil
}

// 连接关闭时回调
func (s *ServiceAccessServiceImpl) OnConnectionClosed(ctx context.Context, ac AccessContext) {
    // 递减并发计数
    s.concurrentTracker.OnDisconnect(ac.ServiceID, ac.EndpointID, ac.RemoteIP)
}
```

### 6.9.8 流量计数不能影响正常使用

核心原则：面板不在流量路径上，流量计数和配额执行都是异步操作，对用户连接零影响。

流量路径（面板不参与）：

```text
用户 ──▶ nginx/caddy（可选）──▶ 核心进程 ──▶ 出站
         反向代理               代理转发
```

面板管理路径（全部异步，不阻塞任何连接）：

```text
┌─────────────────────────────────────────────────────┐
│  面板异步操作（独立 goroutine，不影响连接）            │
│                                                       │
│  1. 轮询核心 Stats API（每 30 秒）                    │  ← 本地 gRPC/HTTP，< 100ms
│  2. 写入流量账本（批量异步）                           │  ← channel 缓冲，满了丢弃
│  3. 配额状态评估（事件驱动）                           │  ← 内存计算，微秒级
│  4. 超额时调用核心 API 移除用户（低频）                 │  ← 本地 gRPC/HTTP，毫秒级
│  5. 恢复时调用核心 API 添加用户（低频）                 │  ← 本地 gRPC/HTTP，毫秒级
│                                                       │
│  全部不阻塞连接、不做网络请求到外部                     │
└─────────────────────────────────────────────────────┘
```

连接热路径上绝对不能做的事情：

- 查数据库
- 写数据库
- 读 nftables 计数器
- 调用核心 stats API
- 写日志文件
- 做 HTTP 请求
- 获取锁竞争严重的锁

面板异步系统（独立 goroutine，不阻塞核心连接处理）：

```text
┌─────────────────────────────────────────────────────┐
│  异步 1：核心 Stats API 轮询（每 30 秒）               │
│                                                       │
│  - Xray: gRPC QueryStats(reset_=true)                 │
│  - sing-box: HTTP GET /traffic                        │
│  - Hysteria 2: HTTP GET /traffic?clear=true           │
│  - TUIC: /proc/<pid>/net/dev                          │
│  - 计算增量，写入 channel → 聚合器                      │
│  - 不阻塞任何连接                                      │
└─────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────┐
│  异步 2：配额状态评估（事件驱动）                       │
│                                                       │
│  - 聚合器写入后触发状态评估                             │
│  - 状态变更时调用核心 API 移除/恢复用户                  │
│  - 不阻塞任何连接                                      │
└─────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────┐
│  旁路 3：账本写入（批量异步）                           │
│                                                       │
│  - 增量事件写入带缓冲 channel                          │
│  - 后台 goroutine 批量消费                             │
│  - 每 5 秒或满 100 条刷一次盘                          │
│  - channel 满时丢弃（非关键路径）                       │
│  - 不阻塞任何连接                                      │
└─────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────┐
│  旁路 4：配额状态刷新（事件驱动）                       │
│                                                       │
│  - 聚合器写入后主动推送新状态到内存缓存                  │
│  - 不是定时轮询数据库                                   │
│  - 缓存更新是原子 swap，无锁竞争                       │
└─────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────┐
│  旁路 5：审计日志写入（异步 channel）                   │
│                                                       │
│  - 审计事件写入带缓冲 channel                          │
│  - 后台 goroutine 批量写入数据库                       │
│  - 不阻塞连接检查                                      │
└─────────────────────────────────────────────────────┘
```

各环节耗时预算：

| 操作 | 位置 | 耗时预算 | 实现方式 |
| --- | --- | --- | --- |
| 核心 Stats API 轮询 | 面板异步 | < 100ms | 每 30 秒一次，本地 gRPC/HTTP |
| 账本写入 | 面板异步 | < 5ms | 批量事务，channel 缓冲 10000 |
| 配额状态评估 | 面板异步 | < 1ms | 内存计算，事件驱动 |
| 超额移除用户 | 面板异步（低频） | < 100ms | 核心管理 API，状态变更时触发 |
| 恢复添加用户 | 面板异步（低频） | < 100ms | 核心管理 API，状态变更时触发 |
| 审计日志 | 面板异步 | < 1ms | channel 缓冲 5000，批量写入 |

关键：面板所有操作都在独立 goroutine 中执行，不阻塞核心的连接处理。用户连接完全由核心独立处理，面板只做后台统计和管理。

内存缓存设计（配额检查热路径）：

```go
// 配额缓存使用 RWMutex 保护的 map
// 读操作（热路径）只需要 RLock，无竞争
// 写操作（旁路聚合器）只需要 Lock，低频
type QuotaCache struct {
    snapshots sync.Map // key: "subscription:123" -> *QuotaSnapshot
}

// 热路径读取：无锁，纳秒级
func (c *QuotaCache) Get(subjectType string, subjectID int64) *QuotaSnapshot {
    key := fmt.Sprintf("%s:%d", subjectType, subjectID)
    if v, ok := c.snapshots.Load(key); ok {
        return v.(*QuotaSnapshot)
    }
    return nil // 缓存未命中，视为正常（不查数据库）
}

// 旁路写入：低频，不影响读取
func (c *QuotaCache) Set(subjectType string, subjectID int64, snapshot *QuotaSnapshot) {
    key := fmt.Sprintf("%s:%d", subjectType, subjectID)
    c.snapshots.Store(key, snapshot)
}

// 旁路批量刷新：聚合器写入后调用
func (c *QuotaCache) BatchRefresh(ctx context.Context, db *gorm.DB) {
    // 从数据库加载所有活跃订阅的配额状态
    var quotas []TrafficQuota
    db.Find(&quotas)

    for _, q := range quotas {
        snapshot := c.loadSnapshot(ctx, db, q)
        c.Set(q.SubjectType, q.SubjectID, snapshot)
    }
}
```

限速检查热路径设计：

```go
// 令牌桶：sync.Mutex 粒度极细，每次 Allow 只做浮点加减
type TokenBucket struct {
    rate      float64
    maxTokens float64
    tokens    float64
    lastTime  time.Time
    mu        sync.Mutex  // 粒度：单个桶，不跨桶竞争
}

func (b *TokenBucket) Allow() bool {
    b.mu.Lock()
    defer b.mu.Unlock()

    now := time.Now()
    b.tokens += now.Sub(b.lastTime).Seconds() * b.rate
    if b.tokens > b.maxTokens {
        b.tokens = b.maxTokens
    }
    b.lastTime = now

    if b.tokens >= 1 {
        b.tokens--
        return true
    }
    return false
}

// 限速管理器：每个 IP 一个桶，桶之间无竞争
type RateLimitManager struct {
    buckets sync.Map // key: "svc:1:ep:12:ip:1.2.3.4" -> *TokenBucket
}

func (m *RateLimitManager) Check(serviceID, endpointID int64, ip string, maxPerMin int) bool {
    key := fmt.Sprintf("svc:%d:ep:%d:ip:%s", serviceID, endpointID, ip)

    v, _ := m.buckets.LoadOrStore(key, NewTokenBucket(
        float64(maxPerMin)/60.0,
        float64(maxPerMin)/6.0, // burst = 10 秒的量
    ))
    return v.(*TokenBucket).Allow()
}
```

并发连接计数热路径设计：

```go
// 原子计数器：无锁，CAS 操作
type ConcurrentTracker struct {
    counters sync.Map // key -> *atomic.Int64
}

func (t *ConcurrentTracker) TryConnect(key string, maxConcurrent int) bool {
    v, _ := t.counters.LoadOrStore(key, &atomic.Int64{})
    counter := v.(*atomic.Int64)

    for {
        current := counter.Load()
        if current >= int64(maxConcurrent) {
            return false
        }
        if counter.CompareAndSwap(current, current+1) {
            return true
        }
    }
}

func (t *ConcurrentTracker) OnDisconnect(key string) {
    if v, ok := t.counters.Load(key); ok {
        v.(*atomic.Int64).Add(-1)
    }
}
```

数据库写入完全异步：

```go
// 事件 channel：带缓冲，满时丢弃（非关键路径）
type AsyncWriter struct {
    eventCh chan TrafficDeltaEvent // 缓冲大小 10000
    auditCh chan SecurityAuditEvent // 缓冲大小 5000
    db      *gorm.DB
}

func (w *AsyncWriter) Start(ctx context.Context) {
    go w.consumeTraffic(ctx)
    go w.consumeAudit(ctx)
}

// 流量事件消费：批量写入
func (w *AsyncWriter) consumeTraffic(ctx context.Context) {
    batch := make([]TrafficDeltaEvent, 0, 100)
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case event := <-w.eventCh:
            batch = append(batch, event)
            if len(batch) >= 100 {
                w.flushTraffic(batch)
                batch = batch[:0]
            }
        case <-ticker.C:
            if len(batch) > 0 {
                w.flushTraffic(batch)
                batch = batch[:0]
            }
        }
    }
}

// 写入流量事件（热路径调用，非阻塞）
func (w *AsyncWriter) WriteTraffic(event TrafficDeltaEvent) {
    select {
    case w.eventCh <- event:
        // 成功写入 channel
    default:
        // channel 满了，丢弃事件
        // 流量统计有误差是可接受的，不能阻塞连接
    }
}

// 写入审计事件（热路径调用，非阻塞）
func (w *AsyncWriter) WriteAudit(event SecurityAuditEvent) {
    select {
    case w.auditCh <- event:
    default:
        // 审计日志丢失可接受，不能阻塞连接
    }
}
```

连接拦截只在状态变更时触发：

```go
// 正常流量：配额检查通过，直接放行，不做任何额外操作
// 超额时：触发一次拦截动作（移除用户 + nftables），之后该用户的连接直接被拒绝
// 恢复时：触发一次恢复动作（恢复用户 + 移除规则）

// 关键：拦截动作是低频操作（状态变更时才触发），不是每次连接都做
// 正常连接只做内存读取（微秒级），超额后才触发文件/命令操作
```

总结：

| 场景 | 热路径操作 | 耗时 | 是否阻塞连接 |
| --- | --- | --- | --- |
| 正常连接（配额充足） | 内存读取 3 次（配额+限速+封禁） | < 30μs | 否 |
| 正常连接（首次访问） | 内存 miss + 加载到缓存 | < 100μs | 否 |
| 超额用户连接 | 内存读取，返回拒绝 | < 30μs | 否（直接拒绝） |
| 流量计数 | 完全旁路 | 0 | 否 |
| 账本写入 | channel 投递 | < 1μs | 否（满了丢弃） |
| 审计日志 | channel 投递 | < 1μs | 否（满了丢弃） |
| 配额缓存刷新 | 原子 swap | < 100μs | 否 |
| 拦截动作（状态变更时） | 核心移除 + nftables + conntrack | 毫秒级 | 低频，不影响正常用户 |

### 7.1 状态枚举

```go
type QuotaState string

const (
    QuotaStateNormal        QuotaState = "normal"
    QuotaStateNearLimit     QuotaState = "near_limit"
    QuotaStateOverLimitSoft QuotaState = "over_limit_soft"
    QuotaStateOverLimitHard QuotaState = "over_limit_hard"
    QuotaStateSuspended     QuotaState = "suspended"
    QuotaStateRestored      QuotaState = "restored"
)
```

### 7.2 拒绝策略

```go
type QuotaRejectMode string

const (
    QuotaRejectModeSoft QuotaRejectMode = "soft"
    QuotaRejectModeHard QuotaRejectMode = "hard"
)
```

### 7.3 配额快照

```go
type QuotaSnapshot struct {
    SubjectType   string
    SubjectID     int64
    UsedBytes     int64
    QuotaBytes    int64
    GraceBytes    int64
    UsageRatio    float64
    RejectMode    QuotaRejectMode
    State         QuotaState
    UpdatedAt     time.Time
}
```

### 7.4 状态机接口

```go
type QuotaStateMachine interface {
    Evaluate(snapshot QuotaSnapshot) QuotaState
    AllowNewConnection(snapshot QuotaSnapshot) error
    ShouldTerminateActiveSession(snapshot QuotaSnapshot) bool
}
```

### 7.5 状态机实现建议

```go
type DefaultQuotaStateMachine struct {
    NearLimitThreshold float64
}

func (m DefaultQuotaStateMachine) Evaluate(s QuotaSnapshot) QuotaState {
    totalLimit := s.QuotaBytes + s.GraceBytes
    if s.State == QuotaStateSuspended {
        return QuotaStateSuspended
    }
    if totalLimit <= 0 {
        return QuotaStateNormal
    }

    used := s.UsedBytes
    if used >= totalLimit {
        if s.RejectMode == QuotaRejectModeHard {
            return QuotaStateOverLimitHard
        }
        return QuotaStateOverLimitSoft
    }

    if float64(used) >= float64(totalLimit)*m.NearLimitThreshold {
        return QuotaStateNearLimit
    }

    return QuotaStateNormal
}
```

### 7.6 连接前检查器

```go
type ConnectionGate interface {
    Check(ctx context.Context, subjectType string, subjectID int64) error
}
```

建议错误：

```go
var (
    ErrQuotaSoftRejected = errors.New("quota soft rejected")
    ErrQuotaHardRejected = errors.New("quota hard rejected")
    ErrQuotaSuspended    = errors.New("quota suspended")
)
```

### 7.7 连接检查流程

```go
func (g *DefaultConnectionGate) Check(ctx context.Context, subjectType string, subjectID int64) error {
    snapshot, err := g.quotaStore.GetSnapshot(ctx, subjectType, subjectID)
    if err != nil {
        return err
    }

    state := g.machine.Evaluate(snapshot)
    switch state {
    case QuotaStateOverLimitSoft:
        return ErrQuotaSoftRejected
    case QuotaStateOverLimitHard:
        return ErrQuotaHardRejected
    case QuotaStateSuspended:
        return ErrQuotaSuspended
    default:
        return nil
    }
}
```

### 7.8 过期与超额的实际拦截实现

`ConnectionGate.Check` 只是判断"该不该拒绝"，真正让核心拒绝连接需要落地到核心配置或网络规则。有三种拦截方式，按推荐优先级排列：

#### 方式 1：核心配置热重载（推荐，最通用）

当用户过期或超额时，从核心配置中移除该用户，核心自然拒绝其连接。

```go
// 用户状态同步器：定时检查用户/订阅状态，同步到核心配置
type UserStatusSyncer struct {
    db           *gorm.DB
    quotaSvc     QuotaService
    coreManagers map[CoreType]CoreUserManager // 每个核心一个管理器
    syncInterval time.Duration                // 默认 30 秒
}

// 核心用户管理器接口
type CoreUserManager interface {
    // 从核心配置中移除用户（热重载，不重启核心）
    RemoveUser(ctx context.Context, serviceID int64, userIdentifier string) error
    // 恢复用户到核心配置
    RestoreUser(ctx context.Context, serviceID int64, user UserConfig) error
    // 查询核心当前用户列表
    ListUsers(ctx context.Context, serviceID int64) ([]UserConfig, error)
}

func (s *UserStatusSyncer) Run(ctx context.Context) {
    ticker := time.NewTicker(s.syncInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            s.syncAll(ctx)
        }
    }
}

func (s *UserStatusSyncer) syncAll(ctx context.Context) {
    // 1. 查询所有需要处理的用户/订阅
    //    - quota_state = over_limit_soft/over_limit_hard/suspended
    //    - subscription status = expired/revoked/disabled
    //    - expires_at < now
    users := s.findUsersToBlock(ctx)

    for _, user := range users {
        for coreType, manager := range s.coreManagers {
            // 检查用户是否已在核心中被移除
            if s.isAlreadyBlocked(ctx, coreType, user) {
                continue
            }

            // 从核心配置中移除用户
            err := manager.RemoveUser(ctx, user.ServiceID, user.CoreIdentifier)
            if err != nil {
                // 记录错误，下次重试
                s.recordSyncError(ctx, user, coreType, err)
                continue
            }

            // 记录拦截事件
            s.recordBlocked(ctx, user, coreType)
        }
    }

    // 2. 查询需要恢复的用户
    //    - quota_state = normal/restored
    //    - subscription status = active
    //    - expires_at > now
    usersToRestore := s.findUsersToRestore(ctx)

    for _, user := range usersToRestore {
        for coreType, manager := range s.coreManagers {
            if !s.isAlreadyBlocked(ctx, coreType, user) {
                continue
            }

            // 恢复用户到核心配置
            err := manager.RestoreUser(ctx, user.ServiceID, user.UserConfig)
            if err != nil {
                s.recordSyncError(ctx, user, coreType, err)
                continue
            }

            s.recordRestored(ctx, user, coreType)
        }
    }
}

// 各核心的具体实现
type XrayUserManager struct {
    apiAddr string // Xray gRPC API 地址
}

func (m *XrayUserManager) RemoveUser(ctx context.Context, serviceID int64, email string) error {
    // Xray gRPC: HandlerService.AlterInbound -> 移除用户
    // 或直接修改配置文件后发送 SIGUSR1 热重载
    conn, _ := grpc.Dial(m.apiAddr, grpc.WithInsecure())
    client := serial.NewHandlerServiceClient(conn)

    _, err := client.AlterInbound(ctx, &serial.AlterInboundRequest{
        Tag: fmt.Sprintf("inbound-%d", serviceID),
        Operation: &serial.AlterInboundRequest_RemoveUserOperation{
            RemoveUserOperation: &serial.RemoveUserOperation{
                Email: email,
            },
        },
    })
    return err
}

func (m *XrayUserManager) RestoreUser(ctx context.Context, serviceID int64, user UserConfig) error {
    conn, _ := grpc.Dial(m.apiAddr, grpc.WithInsecure())
    client := serial.NewHandlerServiceClient(conn)

    _, err := client.AlterInbound(ctx, &serial.AlterInboundRequest{
        Tag: fmt.Sprintf("inbound-%d", serviceID),
        Operation: &serial.AlterInboundRequest_AddUserOperation{
            AddUserOperation: &serial.AddUserOperation{
                Account: &serial.Account{
                    Email:    user.Email,
                    Password: user.Password,
                },
            },
        },
    })
    return err
}

type SingBoxUserManager struct {
    apiAddr   string
    apiSecret string
}

func (m *SingBoxUserManager) RemoveUser(ctx context.Context, serviceID int64, userTag string) error {
    // sing-box HTTP API: POST /experimental/users/remove
    body := fmt.Sprintf(`{"tag": "%s", "name": "%s"}`, fmt.Sprintf("inbound-%d", serviceID), userTag)
    req, _ := http.NewRequestWithContext(ctx, "POST",
        fmt.Sprintf("http://%s/experimental/users/remove", m.apiAddr),
        strings.NewReader(body))
    req.Header.Set("Authorization", "Bearer "+m.apiSecret)
    req.Header.Set("Content-Type", "application/json")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        return fmt.Errorf("sing-box API returned %d", resp.StatusCode)
    }
    return nil
}

func (m *SingBoxUserManager) RestoreUser(ctx context.Context, serviceID int64, user UserConfig) error {
    // sing-box HTTP API: POST /experimental/users/add
    body := fmt.Sprintf(`{"tag": "%s", "name": "%s", "password": "%s"}`,
        fmt.Sprintf("inbound-%d", serviceID), user.Email, user.Password)
    req, _ := http.NewRequestWithContext(ctx, "POST",
        fmt.Sprintf("http://%s/experimental/users/add", m.apiAddr),
        strings.NewReader(body))
    req.Header.Set("Authorization", "Bearer "+m.apiSecret)
    req.Header.Set("Content-Type", "application/json")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    return nil
}

type Hysteria2UserManager struct {
    configPath string // Hysteria 2 配置文件路径
}

func (m *Hysteria2UserManager) RemoveUser(ctx context.Context, serviceID int64, authUser string) error {
    // Hysteria 2 没有热重载 API，需要修改配置后发 SIGHUP
    // 读取配置 -> 移除用户 -> 写回 -> SIGHUP
    config := m.readConfig()
    delete(config.Auth.UserPass, authUser)
    m.writeConfig(config)
    m.reload(ctx, serviceID)
    return nil
}

func (m *Hysteria2UserManager) RestoreUser(ctx context.Context, serviceID int64, user UserConfig) error {
    config := m.readConfig()
    config.Auth.UserPass[user.Email] = user.Password
    m.writeConfig(config)
    m.reload(ctx, serviceID)
    return nil
}

// Hysteria 2 支持 SIGHUP 热重载
func (m *Hysteria2UserManager) reload(ctx context.Context, serviceID int64) {
    pid := m.readPID(serviceID)
    if pid > 0 {
        syscall.Kill(pid, syscall.SIGHUP)
    }
}
```

#### 方式 2：反向代理层限流（可选补充）

如果使用 nginx/caddy 做反向代理，可以在反向代理层做连接级限流。面板生成反向代理配置时，为超额用户添加限流规则，然后 reload 反向代理。

注意：反向代理层只能做连接数限制，不能精确按流量配额拒绝。流量配额必须通过核心管理 API 实现。

#### 拦截方式对比

| 方式 | 延迟 | 通用性 | 精度 | 适用场景 |
| --- | --- | --- | --- | --- |
| nftables 按订阅 IP 拦截 | 毫秒级 | Linux/OpenWrt | 精确 | 推荐默认方案 |
| 核心管理 API 移除用户 | 秒级 | 所有核心 | 精确 | nftables 不可用时的降级 |
| 核心配置文件热重载 | 秒级 | 所有核心 | 精确 | API 也不可用时的降级 |

#### 推荐策略：nftables 订阅 IP 为主 + 核心 API 为辅

```go
type QuotaEnforcer struct {
    ipTracker    *SubscriptionIPTracker   // nftables 订阅 IP 管理
    coreManagers map[CoreType]CoreUserManager // 核心 API 降级
    db           *gorm.DB
}

// 超额处理
func (e *QuotaEnforcer) OnQuotaExceeded(ctx context.Context, subID int64, state QuotaState) {
    switch state {
    case QuotaStateOverLimitSoft:
        // 软拒绝：nftables flush + drop
        // 新连接被 nftables 丢弃，已有连接不受影响
        e.ipTracker.BlockSubscription(subID)

    case QuotaStateOverLimitHard:
        // 硬拒绝：nftables flush + drop + conntrack 清理
        // 已有连接也被断开
        e.ipTracker.BlockSubscription(subID)
        e.flushConntrack(subID) // 清理该订阅所有 IP 的 conntrack 条目

    case QuotaStateSuspended:
        // 管理员暂停：同硬拒绝
        e.ipTracker.BlockSubscription(subID)
        e.flushConntrack(subID)

    case QuotaStateNormal, QuotaStateRestored:
        // 恢复：移除 drop 规则，恢复 IP 计数规则
        e.ipTracker.UnblockSubscription(subID)
    }
}

// conntrack 清理：断开已有连接
func (e *QuotaEnforcer) flushConntrack(subID int64) {
    // 获取该订阅的所有 IP
    ips := e.ipTracker.GetIPs(subID)
    for _, ip := range ips {
        // conntrack -D -s <ip>
        exec.Command("conntrack", "-D", "-s", ip).Run()
    }
}

// 核心 API 降级（nftables 不可用时）
func (e *QuotaEnforcer) OnQuotaExceededFallback(ctx context.Context, userID int64, state QuotaState) {
    user := e.db.FindUser(userID)
    manager := e.coreManagers[user.CoreType]

    switch state {
    case QuotaStateOverLimitSoft, QuotaStateOverLimitHard, QuotaStateSuspended:
        manager.RemoveUser(ctx, user.ServiceID, user.CoreIdentifier)
    case QuotaStateNormal, QuotaStateRestored:
        manager.RestoreUser(ctx, user.ServiceID, user.UserConfig)
    }
}
```

// 过期处理
func (e *QuotaEnforcer) OnSubscriptionExpired(ctx context.Context, subscriptionID int64) {
    users := e.db.FindUsersBySubscription(subscriptionID)
    for _, user := range users {
        manager := e.coreManagers[user.CoreType]
        manager.RemoveUser(ctx, user.ServiceID, user.CoreIdentifier)
    }
}
```

### 8. 配额账本与状态同步

### 8.1 关键流程

完整流程：

1. 采集器输出 `TrafficDeltaEvent`（网络层 nftables 或核心 API）
2. 聚合器写入 `traffic_usage_ledger`
3. 聚合器更新 `traffic_usage_summary`
4. 聚合器更新 `subscriptions.used_bytes_cache`
5. 配额服务刷新 `QuotaSnapshot`（内存缓存）
6. `QuotaStateMachine` 重新评估状态
7. 若状态变更（如 normal → over_limit_soft）：
   - 触发 `QuotaEnforcer.OnQuotaExceeded`
   - 从核心移除用户 + 网络层添加拦截规则
   - 写审计事件 + 触发告警
8. 连接前检查器读取最新缓存状态（热路径，微秒级）
9. 定时同步器每 30 秒兜底校验（确保不遗漏）

### 8.2 并发控制建议

为了避免高并发穿透：

- 内存缓存保存热点配额状态
- 账本更新后主动刷新缓存
- 定期用数据库做一致性回刷

## 9. 服务层接口建议

### 9.1 服务管理

```go
type InboundServiceAppService interface {
    Create(ctx context.Context, in CreateServiceInput) (*InboundServiceDTO, error)
    Update(ctx context.Context, id int64, in UpdateServiceInput) (*InboundServiceDTO, error)
    ListEndpoints(ctx context.Context, serviceID int64) ([]ServiceEndpointDTO, error)
    UpsertEndpoints(ctx context.Context, serviceID int64, in []CreateServiceEndpointInput) error
}
```

### 9.2 流量服务

```go
type TrafficAppService interface {
    Ingest(ctx context.Context, event TrafficDeltaEvent) error
    QueryUsage(ctx context.Context, q UsageQuery) (*UsageResult, error)
}
```

### 9.3 配额服务

```go
type QuotaAppService interface {
    Evaluate(ctx context.Context, subjectType string, subjectID int64) (*QuotaSnapshot, error)
    CheckConnection(ctx context.Context, subjectType string, subjectID int64) error
    ResetCycle(ctx context.Context, subjectType string, subjectID int64) error
}
```

## 10. API 结构建议

### 10.1 服务入口管理

```json
POST /api/v1/services/:id/endpoints
{
  "endpoint_type": "domain",
  "address": "a.example.com",
  "server_name": "a.example.com",
  "listen_host": "0.0.0.0",
  "listen_port": 443,
  "tls_enabled": true,
  "certificate_id": 10,
  "is_default": true,
  "priority": 100
}
```

### 10.2 配额检查结果

```json
{
  "subject_type": "user",
  "subject_id": 1001,
  "used_bytes": 1073741824,
  "quota_bytes": 2147483648,
  "grace_bytes": 0,
  "state": "near_limit",
  "reject_mode": "soft"
}
```

## 11. 实现顺序建议

如果开始编码，建议顺序：

1. 先实现 `service_endpoints` 模型、repository、service
2. 再实现协议组合 `CapabilityValidator`
3. 再实现 `TrafficCollectorAdapter` 注册中心与事件模型
4. 再实现账本聚合器
5. 最后实现 `QuotaStateMachine` 和 `ConnectionGate`

这样可以先把服务入口和配置生成打通，再把流量和拒绝控制接进去。

## 12. 服务端 IP 限速与黑名单设计

### 12.1 设计目标

这部分控制的是服务端代理入口，不是管理面板登录入口。

目标：

- 限制异常 IP 高频建连
- 限制单 IP 并发连接数
- 支持 IP 黑名单与白名单
- 发现爆破或异常行为后自动封禁
- 封禁支持定时自动解封

### 12.2 领域模型

```go
type AccessPolicyMode string

const (
    AccessPolicyAllow     AccessPolicyMode = "allow"
    AccessPolicyDeny      AccessPolicyMode = "deny"
    AccessPolicyRateLimit AccessPolicyMode = "rate_limit"
)

type AccessPolicyScope string

const (
    AccessPolicyScopeGlobal       AccessPolicyScope = "global"
    AccessPolicyScopeEndpoint     AccessPolicyScope = "endpoint"
    AccessPolicyScopeUser         AccessPolicyScope = "user"
    AccessPolicyScopeSubscription AccessPolicyScope = "subscription"
)

type ServiceAccessPolicy struct {
    ID                         int64
    ServiceID                  int64
    EndpointID                 *int64
    PolicyScope                AccessPolicyScope
    Mode                       AccessPolicyMode
    IPCIDR                     string
    UserID                     *int64
    SubscriptionID             *int64
    MaxNewConnectionsPerMin    int
    MaxConcurrentConnections   int
    BanDurationSeconds         int
    AutoBanEnabled             bool
    Status                     int
    CreatedAt                  time.Time
    UpdatedAt                  time.Time
}
```

### 12.3 封禁模型

```go
type IPBanSource string

const (
    IPBanSourceManual        IPBanSource = "manual"
    IPBanSourceAutoRateLimit IPBanSource = "auto_rate_limit"
    IPBanSourceAutoRisk      IPBanSource = "auto_risk_control"
)

type ServiceIPBan struct {
    ID         int64
    ServiceID  int64
    EndpointID *int64
    IPCIDR     string
    Reason     string
    Source     IPBanSource
    ExpiresAt  *time.Time
    CreatedAt  time.Time
    UpdatedAt  time.Time
}
```

### 12.4 连接上下文

```go
type AccessContext struct {
    ServiceID        int64
    EndpointID       int64
    UserID           *int64
    SubscriptionID   *int64
    RemoteIP         string
    Protocol         ProtocolType
    CurrentConcurrent int
    AttemptAt        time.Time
}
```

### 12.5 判定顺序

建议固定判定顺序：

1. 查白名单
2. 查黑名单
3. 查临时封禁
4. 查并发连接限制
5. 查新建连接速率限制
6. 查额外风控规则
7. 通过则允许连接

这样做的原因：

- 白名单应能优先放行
- 黑名单和封禁应尽早拦截
- 速率统计只对未提前拒绝的请求生效

### 12.6 访问检查器接口

```go
type AccessGate interface {
    Check(ctx context.Context, ac AccessContext) error
    OnAccepted(ctx context.Context, ac AccessContext) error
    OnClosed(ctx context.Context, ac AccessContext) error
}
```

建议错误：

```go
var (
    ErrAccessDeniedByPolicy    = errors.New("access denied by policy")
    ErrAccessDeniedByBan       = errors.New("access denied by ban")
    ErrAccessRateLimited       = errors.New("access rate limited")
    ErrAccessConcurrentLimited = errors.New("access concurrent limited")
)
```

### 12.7 计数器缓存

服务端入口限速不适合每次都直接查数据库。

建议缓存键：

- `svc:conn:new:<service_id>:<endpoint_id>:<ip>`
- `svc:conn:active:<service_id>:<endpoint_id>:<ip>`
- `svc:ban:<service_id>:<endpoint_id>:<ip>`

要求：

- 支持短 TTL
- 能原子递增
- 进程重启后可从数据库恢复重要封禁状态

### 12.8 自动封禁流程

推荐流程：

1. 入口收到新连接
2. 执行 `AccessGate.Check`
3. 若命中速率阈值则记录违规次数
4. 违规次数达到策略阈值后写入 `ServiceIPBan`
5. 缓存 ban 状态
6. 返回拒绝
7. 到达 `expires_at` 后自动清理

### 12.9 Repository 接口

```go
type ServiceAccessPolicyRepository interface {
    ListByServiceID(ctx context.Context, serviceID int64) ([]*ServiceAccessPolicyModel, error)
    Create(ctx context.Context, m *ServiceAccessPolicyModel) error
    Update(ctx context.Context, m *ServiceAccessPolicyModel) error
    Delete(ctx context.Context, id int64) error
}

type ServiceIPBanRepository interface {
    ListActive(ctx context.Context, serviceID int64, endpointID *int64) ([]*ServiceIPBanModel, error)
    FindMatched(ctx context.Context, serviceID int64, endpointID *int64, ip string) (*ServiceIPBanModel, error)
    Create(ctx context.Context, m *ServiceIPBanModel) error
    Delete(ctx context.Context, id int64) error
    DeleteExpired(ctx context.Context, now time.Time) (int64, error)
}
```

### 12.10 服务层接口

```go
type ServiceAccessAppService interface {
    CheckConnection(ctx context.Context, ac AccessContext) error
    ListPolicies(ctx context.Context, serviceID int64) ([]ServiceAccessPolicy, error)
    UpsertPolicy(ctx context.Context, in ServiceAccessPolicy) error
    ListIPBans(ctx context.Context, serviceID int64) ([]ServiceIPBan, error)
    CreateIPBan(ctx context.Context, in ServiceIPBan) error
    RemoveIPBan(ctx context.Context, banID int64) error
}
```

### 12.11 运行时挂接点

服务端连接控制建议挂在“核心认证成功前后”的边界。

要求：

- 新建连接检查在用户进入正式代理流程前执行
- 活跃连接计数在连接接受后递增
- 连接关闭时递减并写审计

### 12.12 审计与日志

至少记录：

- 服务 ID
- 入口 ID
- 远端 IP
- 命中策略
- 拒绝原因
- 是否自动封禁
- 封禁过期时间

### 12.13 API 建议

```json
POST /api/v1/services/:id/access-policies
{
  "policy_scope": "endpoint",
  "endpoint_id": 12,
  "mode": "rate_limit",
  "ip_cidr": "0.0.0.0/0",
  "max_new_connections_per_min": 60,
  "max_concurrent_connections": 10,
  "ban_duration_seconds": 600,
  "auto_ban_enabled": true
}
```

```json
POST /api/v1/services/:id/ip-bans
{
  "endpoint_id": 12,
  "ip_cidr": "198.51.100.10/32",
  "reason": "manual abuse block",
  "source": "manual",
  "expires_at": "2026-06-01T00:00:00Z"
}
```

### 12.14 本期范围说明

本节到此为止，服务端入口访问控制在当前单节点版本中已经覆盖：

- IP 限速
- 并发限制
- 黑白名单
- 自动封禁
- GeoIP / ASN / CIDR 风控
- 审计与告警联动

若后续要继续增强，只允许作为新版本专题扩展，不影响当前版本实现边界。

## 13. GeoIP、ASN、CIDR 风控设计

### 13.1 目标

除了基于单个 IP 的规则，还需要支持更高层级的来源控制：

- 按国家或地区限制
- 按 ASN 限制
- 按 CIDR 网段限制
- 按风险分动态加权

这样可以避免只靠单 IP 封禁导致规则过碎、维护成本过高。

### 13.2 领域模型

```go
type GeoRuleMode string

const (
    GeoRuleModeAllow GeoRuleMode = "allow"
    GeoRuleModeDeny  GeoRuleMode = "deny"
    GeoRuleModeScore GeoRuleMode = "score"
)

type GeoRuleScope string

const (
    GeoRuleScopeCountry GeoRuleScope = "country"
    GeoRuleScopeRegion  GeoRuleScope = "region"
    GeoRuleScopeASN     GeoRuleScope = "asn"
    GeoRuleScopeCIDR    GeoRuleScope = "cidr"
)

type ServiceGeoRule struct {
    ID              int64
    ServiceID       int64
    EndpointID      *int64
    Scope           GeoRuleScope
    Mode            GeoRuleMode
    MatchValue      string
    RiskScore       int
    Priority        int
    Status          int
    CreatedAt       time.Time
    UpdatedAt       time.Time
}
```

### 13.3 解析结果模型

```go
type GeoResolution struct {
    RemoteIP   string
    Country    string
    Region     string
    ASN        string
    CIDR       string
    ResolvedAt time.Time
}
```

### 13.4 风控解析器接口

```go
type GeoResolver interface {
    Resolve(ctx context.Context, ip string) (*GeoResolution, error)
}
```

### 13.5 判定规则

建议优先级：

1. `allow` 白名单规则
2. `deny` 明确拒绝规则
3. `score` 风险加分规则
4. 汇总风险分后决定是否放行、限速或临时封禁

### 13.6 风险动作建议

建议风险动作：

- 仅记录
- 增加限速
- 增加验证码或二次认证要求
- 临时封禁
- 永久拒绝

### 13.7 服务层接口

```go
type ServiceGeoRuleAppService interface {
    Resolve(ctx context.Context, ip string) (*GeoResolution, error)
    MatchRules(ctx context.Context, serviceID int64, endpointID *int64, ip string) ([]ServiceGeoRule, error)
    EvaluateRisk(ctx context.Context, serviceID int64, endpointID *int64, ip string) (*GeoRiskDecision, error)
}

type GeoRiskDecision struct {
    Allowed       bool
    RiskScore     int
    MatchedRuleIDs []int64
    Action        string
}
```

## 14. 服务端入口访问控制状态机

### 14.1 为什么需要状态机

如果只有“允许”或“拒绝”两个状态，后续很难扩展：

- 观察期
- 临时封禁
- 自动解封后冷却
- 高风险降速

因此入口访问控制建议使用显式状态机。

### 14.2 状态定义

```go
type AccessControlState string

const (
    AccessControlStateAllowed    AccessControlState = "allowed"
    AccessControlStateObserved   AccessControlState = "observed"
    AccessControlStateRateLimited AccessControlState = "rate_limited"
    AccessControlStateTempBanned AccessControlState = "temp_banned"
    AccessControlStatePermBanned AccessControlState = "perm_banned"
    AccessControlStateCooling    AccessControlState = "cooling"
)
```

状态含义：

- `allowed`：正常允许连接
- `observed`：已记录异常，但未触发动作
- `rate_limited`：允许部分请求，通过限速控制
- `temp_banned`：临时封禁
- `perm_banned`：永久封禁
- `cooling`：临时封禁结束后的观察冷却期

### 14.3 状态流转

```text
allowed -> observed
observed -> rate_limited
observed -> temp_banned
rate_limited -> temp_banned
temp_banned -> cooling
cooling -> allowed
cooling -> observed
allowed -> perm_banned
observed -> perm_banned
rate_limited -> perm_banned
temp_banned -> perm_banned
```

### 14.4 触发条件建议

- 高频新建连接：`allowed -> observed`
- 持续超限：`observed -> rate_limited`
- 明确攻击或命中高风险规则：`observed/rate_limited -> temp_banned`
- 多次复犯：`temp_banned -> perm_banned`
- 自动解封：`temp_banned -> cooling`

### 14.5 状态机接口

```go
type AccessStateMachine interface {
    Evaluate(ctx context.Context, ac AccessContext, snapshot AccessRuntimeSnapshot) AccessControlDecision
}

type AccessRuntimeSnapshot struct {
    CurrentState           AccessControlState
    NewConnectionsPerMin   int
    ConcurrentConnections  int
    ViolationCount         int
    RiskScore              int
    BanExpiresAt           *time.Time
}

type AccessControlDecision struct {
    NextState   AccessControlState
    Allowed     bool
    Reason      string
    AutoBan     bool
    BanDuration time.Duration
}
```

## 15. 审计日志与告警联动设计

### 15.1 目标

服务端入口访问控制如果没有审计和告警联动，后续很难排查：

- 为什么某个 IP 被封
- 为什么某个入口突然大量拒绝连接
- 哪些规则经常误伤

### 15.2 审计事件模型

```go
type SecurityAuditEvent struct {
    ID           int64
    Category     string
    EventType    string
    ServiceID    *int64
    EndpointID   *int64
    UserID       *int64
    RemoteIP     string
    Result       string
    Reason       string
    MetadataJSON string
    CreatedAt    time.Time
}
```

建议分类：

- `service_access`
- `service_ban`
- `service_rate_limit`
- `subscription_security`
- `admin_security`

### 15.3 告警事件模型

```go
type SecurityAlert struct {
    ID           int64
    AlertType    string
    Severity     string
    ServiceID    *int64
    EndpointID   *int64
    DedupKey     string
    Title        string
    Message      string
    Status       string
    TriggeredAt  time.Time
    ResolvedAt   *time.Time
}
```

### 15.4 告警触发场景

建议至少触发：

- 短时间大量新建连接被拒绝
- 单 IP 高频触发限速
- 自动封禁次数异常升高
- 同一入口命中多个高风险 Geo/ASN 规则
- 计费流量与物理带宽偏差异常

### 15.5 审计写入点

至少在以下时机写审计：

- 命中白名单
- 命中黑名单
- 命中 Geo/ASN/CIDR 规则
- 触发限速
- 触发自动封禁
- 自动解封
- 管理员手动封禁或解封

### 15.6 告警去重建议

为了避免告警风暴，建议用 `DedupKey` 做窗口去重。

示例：

- `service_access:rate_limit:<service_id>:<endpoint_id>`
- `service_access:auto_ban:<service_id>:<endpoint_id>:<ip>`

### 15.7 服务层接口

```go
type SecurityAuditService interface {
    Write(ctx context.Context, event SecurityAuditEvent) error
}

type SecurityAlertService interface {
    Trigger(ctx context.Context, alert SecurityAlert) error
    Resolve(ctx context.Context, dedupKey string) error
}
```

## 16. 在线下载与更新设计

### 16.1 设计目标

更新系统需要同时覆盖两类对象：

- 各核心二进制
- 控制端自身二进制

两者的共同目标：

- 在线获取版本索引
- 下载指定版本
- 校验哈希与签名
- 保留旧版本
- 切换目标版本
- 失败自动回滚

### 16.2 版本清单模型

```go
type ReleaseChannel string

const (
    ReleaseChannelStable ReleaseChannel = "stable"
    ReleaseChannelBeta   ReleaseChannel = "beta"
    ReleaseChannelLTS    ReleaseChannel = "lts"
)

type BinaryRelease struct {
    ComponentName   string
    ComponentType   string
    Version         string
    OS              string
    Arch            string
    DownloadURL     string
    Checksum        string
    Signature       string
    ReleaseChannel  ReleaseChannel
    MinUpgradeable  string
    Notes           string
}
```

### 16.3 更新任务模型

```go
type UpdateTargetType string

const (
    UpdateTargetCore         UpdateTargetType = "core"
    UpdateTargetControlPlane UpdateTargetType = "control_plane"
)

type UpdateJobStatus string

const (
    UpdateJobPending    UpdateJobStatus = "pending"
    UpdateJobRunning    UpdateJobStatus = "running"
    UpdateJobSucceeded  UpdateJobStatus = "succeeded"
    UpdateJobFailed     UpdateJobStatus = "failed"
    UpdateJobRolledBack UpdateJobStatus = "rolled_back"
)

type UpdateJobPhase string

const (
    UpdatePhaseResolve       UpdateJobPhase = "resolve"
    UpdatePhaseDownload      UpdateJobPhase = "download"
    UpdatePhaseVerify        UpdateJobPhase = "verify"
    UpdatePhaseSnapshot      UpdateJobPhase = "snapshot"
    UpdatePhaseSwitch        UpdateJobPhase = "switch"
    UpdatePhaseMigrate       UpdateJobPhase = "migrate"
    UpdatePhaseHealthCheck   UpdateJobPhase = "health_check"
    UpdatePhaseRollback      UpdateJobPhase = "rollback"
    UpdatePhaseComplete      UpdateJobPhase = "complete"
)

type UpdateJob struct {
    ID                int64
    TargetType        UpdateTargetType
    TargetID          *int64
    ComponentName     string
    FromVersion       string
    ToVersion         string
    ReleaseChannel    ReleaseChannel
    Status            UpdateJobStatus
    Phase             UpdateJobPhase
    ErrorMessage      string
    RollbackAvailable bool
    CreatedAt         time.Time
    UpdatedAt         time.Time
}
```

### 16.4 更新管理器接口

```go
type UpdateManager interface {
    ListReleases(ctx context.Context, q ReleaseQuery) ([]BinaryRelease, error)
    DownloadRelease(ctx context.Context, rel BinaryRelease) (string, error)
    VerifyRelease(ctx context.Context, rel BinaryRelease, filePath string) error
    CreateJob(ctx context.Context, in CreateUpdateJobInput) (*UpdateJob, error)
    RunJob(ctx context.Context, jobID int64) error
    RollbackJob(ctx context.Context, jobID int64) error
}
```

### 16.5 核心更新执行器

核心更新建议与控制端解耦，使用独立执行器：

```go
type CoreUpdateExecutor interface {
    Prepare(ctx context.Context, hostID int64, serviceID int64, rel BinaryRelease) error
    SwitchVersion(ctx context.Context, hostID int64, serviceID int64, version string) error
    HealthCheck(ctx context.Context, hostID int64, serviceID int64) error
    Rollback(ctx context.Context, hostID int64, serviceID int64, version string) error
}
```

推荐策略：

- 下载新版本到独立版本目录
- 当前服务实例只切换引用的版本
- 不删除旧版本
- 失败后立即切回旧版本

### 16.6 控制端自更新执行器

控制端自更新采用"下载到 staging，mv 替换二进制，exec 重启"策略。

建议设计：

```go
type ControlPlaneUpdateExecutor interface {
    PreflightCheck(ctx context.Context, rel BinaryRelease) error
    Download(ctx context.Context, rel BinaryRelease) (stagingDir string, err error)
    Verify(ctx context.Context, rel BinaryRelease, stagingDir string) error
    CreateDatabaseSnapshot(ctx context.Context) (snapshotPath string, err error)
    ReplaceBinary(ctx context.Context, stagingDir string) error
    ExecRestart(ctx context.Context) error
    RunMigrations(ctx context.Context) error
    HealthCheck(ctx context.Context) error
    Rollback(ctx context.Context) error
    CleanupStaging(ctx context.Context) error
    CleanupOldBackups(ctx context.Context, keepCount int) error
}

type ControlPlaneSnapshot struct {
    Version         string
    BackupDir       string
    DatabaseBackup  string
    CreatedAt       time.Time
}
```

推荐实现流程：

1. `PreflightCheck`：校验目标版本兼容性、磁盘空间、os/arch 匹配
2. `Download`：下载到 `data/update/staging/corefusion-<version>/`
3. `Verify`：校验哈希与签名
4. `CreateDatabaseSnapshot`：备份 SQLite 文件到 `data/update/backup/db-<timestamp>.db`
5. `ReplaceBinary`（进程仍在运行）：
   - `mv` 当前运行中的二进制到 `data/update/backup/corefusion-<old-version>-<timestamp>/`
   - `mv` staging 中的新二进制到原文件路径
   - Linux/macOS：进程继续从旧 inode 运行，不受 `mv` 影响
   - Windows：先 `mv` 当前 exe 为 `.old`，再 `mv` 新 exe 到原路径
6. `ExecRestart`：调用 `syscall.Exec` 用新二进制替换当前进程，或向 systemd 发送重启信号
7. 新进程启动后执行 `RunMigrations`：自动检测并执行待处理的数据库迁移
8. `HealthCheck`：检查 API、数据库、关键后台任务
9. 若健康检查失败：`Rollback` 将备份 `mv` 回原路径并再次 `ExecRestart`
10. 成功后 `CleanupStaging` 并按保留策略 `CleanupOldBackups`

关键点：

- Linux/macOS 上 `mv` 运行中的二进制是安全的，内核持有旧 inode，进程不会崩溃
- 整个过程只重启一次（步骤 6），不存在"停了没法启动"的窗口
- `mv` 在同一文件系统内是原子的
- Windows 上运行中的 `.exe` 无法直接 rename，但可以 rename 为不同后缀名（如 `.old`）
- 数据库迁移在新进程启动后执行，迁移前必须先有快照
- 迁移脚本必须记录版本边界
- 回滚前必须判断迁移是否可逆

### 16.7 核心更新执行器

核心进程由面板托管，面板可以主动控制启停，不存在"自己停自己"的问题。

```go
type CoreUpdateExecutor interface {
    Download(ctx context.Context, rel BinaryRelease) (stagingDir string, err error)
    Verify(ctx context.Context, rel BinaryRelease, stagingDir string) error
    StopService(ctx context.Context, serviceID int64) error
    BackupCurrent(ctx context.Context, coreType string) (backupDir string, err error)
    SwitchVersion(ctx context.Context, coreType string, stagingDir string) error
    StartService(ctx context.Context, serviceID int64) error
    HealthCheck(ctx context.Context, serviceID int64) error
    Rollback(ctx context.Context, coreType string, backupDir string) error
}
```

目录布局：

- staging: `data/binaries/staging/<core-type>-<version>/`
- active: `data/binaries/active/<core-type>/`
- backup: `data/binaries/backup/<core-type>-<old-version>-<timestamp>/`

核心更新流程（面板作为外部管理者）：

1. 下载新版本到 staging 目录
2. 校验哈希与签名
3. 面板主动停止核心进程
4. `mv` 旧版本到 backup 目录
5. `mv` 新版本到 active 目录
6. 面板重新启动核心进程
7. 健康检查
8. 失败则 `mv` backup 回 active 并重启

说明：核心进程由面板托管，步骤 3 和 6 由面板执行，不存在自更新的"停了没法启动"问题。

### 16.8 更新前置检查

必须检查：

- 目标版本是否允许从当前版本升级
- 是否匹配当前 `os/arch`
- 磁盘空间是否足够
- 是否有可写版本目录
- 数据库快照是否成功
- 当前是否存在冲突更新任务

### 16.9 Repository 接口

```go
type BinaryReleaseRepository interface {
    Upsert(ctx context.Context, rel *BinaryReleaseModel) error
    List(ctx context.Context, q ReleaseQuery) ([]*BinaryReleaseModel, error)
}

type UpdateJobRepository interface {
    Create(ctx context.Context, job *UpdateJobModel) error
    Update(ctx context.Context, job *UpdateJobModel) error
    Get(ctx context.Context, id int64) (*UpdateJobModel, error)
    List(ctx context.Context, q UpdateJobQuery) ([]*UpdateJobModel, error)
}
```

### 16.10 API 输入输出建议

```json
POST /api/v1/binaries/update
{
  "service_id": 20,
  "core_type": "xray",
  "target_version": "1.8.24",
  "release_channel": "stable"
}
```

```json
POST /api/v1/system/update
{
  "target_version": "0.2.0",
  "release_channel": "stable",
  "maintenance_mode": true
}
```

### 16.11 审计与告警

升级系统至少写入：

- 更新开始
- 下载成功或失败
- 校验成功或失败
- 版本切换成功或失败
- 健康检查结果
- 自动回滚

### 16.12 推荐实现顺序

如果后面要进入代码骨架，建议顺序：

1. 先实现 `BinaryRelease` 与 `UpdateJob` 模型
2. 再实现核心二进制下载与校验
3. 再实现核心版本切换与回滚
4. 最后实现控制端自更新执行器

## 17. 升级任务状态机设计

### 17.1 为什么要单独设计状态机

升级任务不是单步骤动作，而是多阶段长流程：

- 解析版本
- 下载
- 校验
- 备份
- 切换
- 迁移
- 健康检查
- 回滚

如果只用一个 `running/failed/succeeded` 状态，后续很难准确恢复、重试和审计。

### 17.2 任务状态

```go
type UpgradeState string

const (
    UpgradeStateCreated          UpgradeState = "created"
    UpgradeStateQueued           UpgradeState = "queued"
    UpgradeStateResolving        UpgradeState = "resolving"
    UpgradeStateDownloading      UpgradeState = "downloading"
    UpgradeStateVerifying        UpgradeState = "verifying"
    UpgradeStateSnapshotting     UpgradeState = "snapshotting"
    UpgradeStateSwitching        UpgradeState = "switching"
    UpgradeStateMigrating        UpgradeState = "migrating"
    UpgradeStateHealthChecking   UpgradeState = "health_checking"
    UpgradeStateSucceeded        UpgradeState = "succeeded"
    UpgradeStateFailed           UpgradeState = "failed"
    UpgradeStateRollbackQueued   UpgradeState = "rollback_queued"
    UpgradeStateRollingBack      UpgradeState = "rolling_back"
    UpgradeStateRolledBack       UpgradeState = "rolled_back"
    UpgradeStateNeedsIntervention UpgradeState = "needs_intervention"
)
```

### 17.3 状态流转

```text
created -> queued
queued -> resolving
resolving -> downloading
downloading -> verifying
verifying -> snapshotting
snapshotting -> switching
switching -> migrating
migrating -> health_checking
health_checking -> succeeded

任意执行阶段 -> failed
failed -> rollback_queued
rollback_queued -> rolling_back
rolling_back -> rolled_back
rolling_back -> needs_intervention
```

### 17.4 状态机接口

```go
type UpgradeStateMachine interface {
    CanTransit(from UpgradeState, to UpgradeState) bool
    NextOnSuccess(current UpgradeState) UpgradeState
    NextOnFailure(current UpgradeState, rollbackAvailable bool) UpgradeState
}
```

### 17.5 任务恢复策略

建议：

- 进程重启后读取未完成任务
- 根据最后持久化状态决定恢复点
- `switching`、`migrating`、`rolling_back` 阶段必须谨慎恢复
- 无法安全恢复时进入 `needs_intervention`

## 18. 数据库迁移与回滚策略设计

### 18.1 设计原则

控制端升级最危险的部分通常不是二进制切换，而是数据库迁移。

必须遵守：

- 所有迁移都有版本号
- 所有迁移都有执行记录
- 升级前必须快照
- 回滚前必须判断迁移是否可逆

### 18.2 迁移记录模型

```go
type SchemaMigration struct {
    ID              int64
    Version         string
    Direction       string
    Name            string
    Checksum        string
    Reversible      bool
    ExecutedAt      time.Time
}
```

### 18.3 迁移计划模型

```go
type MigrationPlan struct {
    FromVersion       string
    ToVersion         string
    Steps             []MigrationStep
    RequiresSnapshot  bool
    FullyReversible   bool
}

type MigrationStep struct {
    Version      string
    Name         string
    Reversible   bool
}
```

### 18.4 迁移执行器接口

```go
type MigrationExecutor interface {
    BuildPlan(ctx context.Context, fromVersion, toVersion string) (*MigrationPlan, error)
    Apply(ctx context.Context, plan MigrationPlan) error
    Rollback(ctx context.Context, plan MigrationPlan) error
}
```

### 18.5 我推荐的回滚策略

推荐分两层：

- 逻辑回滚
- 快照回滚

逻辑回滚适用：

- 迁移步骤明确可逆
- 数据影响范围小

快照回滚适用：

- 迁移不可逆
- 涉及结构删除或数据重写
- 迁移执行到中间状态失败

### 18.6 SQLite 特别建议

因为当前设计使用 `SQLite`，建议：

- 升级前直接做完整数据库文件快照
- 迁移期间禁止并发写入
- 迁移失败优先回退到快照

### 18.7 版本边界检查

必须检查：

- 当前版本是否允许升级到目标版本
- 是否跨越不兼容版本边界
- 是否需要先升级到中间版本

### 18.8 迁移审计

至少记录：

- 起始版本
- 目标版本
- 迁移步骤
- 是否可逆
- 是否使用快照恢复
- 最终结果

### 18.9 迁移编号规范

建议统一使用：

```text
YYYYMMDDNN_<action>_<object>.<direction>.sql
```

示例：

```text
2026052501_create_server_share_profiles.up.sql
2026052501_create_server_share_profiles.down.sql
2026052502_add_server_share_indexes.up.sql
2026052502_add_server_share_indexes.down.sql
```

规则：

- `YYYYMMDDNN` 为全项目全局递增编号
- `action` 只允许使用 `create`、`alter`、`backfill`、`add`、`drop`
- `direction` 只允许 `up` 或 `down`
- 一次迁移只做一种主动作，避免建表、回填、加索引混在一起

### 18.10 回滚模板建议

可逆迁移模板：

```sql
-- name: 2026052501_create_server_share_profiles
-- reversible: true
-- depends_on:

BEGIN;
CREATE TABLE server_share_profiles (...);
COMMIT;
```

```sql
-- name: 2026052501_create_server_share_profiles
-- reversible: true
-- depends_on:

BEGIN;
DROP TABLE IF EXISTS server_share_profiles;
COMMIT;
```

不可逆迁移模板：

```sql
-- name: 2026052601_backfill_subscription_quota_state
-- reversible: false
-- depends_on: 2026052509_add_subscription_quota_fields

BEGIN;
-- backfill logic
COMMIT;
```

```sql
-- name: 2026052601_backfill_subscription_quota_state
-- reversible: false
-- depends_on: 2026052509_add_subscription_quota_fields

-- not reversible
```

要求：

- 执行器读到 `reversible: false` 时禁止逻辑回滚
- 对不可逆迁移必须先检查数据库快照是否存在
- 失败后优先使用 SQLite 文件快照做整库恢复

## 19. 控制端与核心升级编排

### 19.1 为什么需要协同编排

当前阶段先明确两类可升级对象：

- 控制端
- 各核心二进制

它们之间存在依赖：

- 核心版本可能依赖新的配置字段

因此当前阶段也不能把控制端和核心更新完全割裂。
当前阶段严格限定为单节点一体化升级，不考虑 `Agent`、远程主机或集群编排，也不在当前模型中保留对应目标字段。

### 19.2 依赖关系模型

```go
type VersionCompatibility struct {
    ComponentName      string
    Version            string
    MinControlVersion  string
    MinCoreVersion     string
    MaxCoreVersion     string
}
```

### 19.3 编排策略

我推荐的默认顺序：

1. 校验兼容矩阵
2. 先升级控制端到可管理目标版本
3. 最后分批升级核心

例外情况：

- 如果核心升级不依赖控制端新字段，可单独先做核心升级

### 19.4 协同编排器接口

```go
type UpgradeOrchestrator interface {
    Plan(ctx context.Context, in UpgradePlanInput) (*UpgradePlanResult, error)
    Execute(ctx context.Context, plan UpgradePlanResult) error
    Rollback(ctx context.Context, plan UpgradePlanResult) error
}

type UpgradePlanInput struct {
    ControlPlaneVersion string
    CoreTargets         []CoreUpgradeTarget
    ReleaseChannel      ReleaseChannel
}

type CoreUpgradeTarget struct {
    ServiceID     int64
    CoreType      string
    TargetVersion string
}

type UpgradePlanResult struct {
    Steps []UpgradeStep
}

type UpgradeStep struct {
    Order         int
    TargetType    UpdateTargetType
    TargetID      *int64
    ComponentName string
    TargetVersion string
    CanRollback   bool
}
```

### 19.5 批量升级策略

建议支持：

- 单服务升级
- 单节点内按服务批次升级
- 按百分比灰度升级
- 指定失败阈值自动停止

例如：

- 先升级 10%
- 健康检查通过后再升级剩余 90%

### 19.6 健康检查门槛

每一阶段升级后都需要健康检查：

- 控制端：API、数据库、登录页、关键后台任务
- 核心：监听端口、进程状态、基础探针连接

### 19.7 协同回滚策略

建议规则：

- 控制端失败优先回滚控制端
- 核心灰度升级失败时只回滚受影响批次

### 19.8 推荐默认策略

如果没有特别复杂的依赖，我建议默认采用：

- 控制端先升级
- 核心按服务分批升级
- 每一批都强制健康检查
- 任意阶段失败都暂停后续计划

补充说明：

- 多主机、`Agent` 独立版本升级接口和集群版本编排当前阶段不实现
- 后续若引入，再基于新的多节点领域模型单独扩展

## 20. 连接失败原因分类设计

### 20.1 为什么需要统一分类

如果连接失败原因只写原始日志，后续会很难做：

- 监控统计
- 告警聚合
- UI 可读展示
- 风控命中分析

因此建议使用统一失败码，再附加原始细节。

### 20.2 失败分类模型

```go
type ConnectionFailureCode string

const (
    FailureAuthInvalidCredential   ConnectionFailureCode = "auth_invalid_credential"
    FailureAuthExpiredUser         ConnectionFailureCode = "auth_expired_user"
    FailureAuthDisabledUser        ConnectionFailureCode = "auth_disabled_user"
    FailureQuotaSoftRejected       ConnectionFailureCode = "quota_soft_rejected"
    FailureQuotaHardRejected       ConnectionFailureCode = "quota_hard_rejected"
    FailureAccessDeniedPolicy      ConnectionFailureCode = "access_denied_policy"
    FailureAccessDeniedBan         ConnectionFailureCode = "access_denied_ban"
    FailureAccessRateLimited       ConnectionFailureCode = "access_rate_limited"
    FailureAccessConcurrentLimited ConnectionFailureCode = "access_concurrent_limited"
    FailureTLSHandshake            ConnectionFailureCode = "tls_handshake_failed"
    FailureQUICHandshake           ConnectionFailureCode = "quic_handshake_failed"
    FailureTransportMismatch       ConnectionFailureCode = "transport_mismatch"
    FailureProtocolDecode          ConnectionFailureCode = "protocol_decode_failed"
    FailureUpstreamUnavailable     ConnectionFailureCode = "upstream_unavailable"
    FailureInternalError           ConnectionFailureCode = "internal_error"
)
```

### 20.3 失败事件模型

```go
type ConnectionFailureEvent struct {
    ServiceID      int64
    EndpointID     int64
    UserID         *int64
    SubscriptionID *int64
    RemoteIP       string
    Protocol       ProtocolType
    Transport      TransportType
    FailureCode    ConnectionFailureCode
    Reason         string
    RawMessage     string
    OccurredAt     time.Time
}
```

### 20.4 分类原则

建议：

- 外部展示用标准失败码
- 内部保留原始错误文本
- 不把敏感认证信息写入前端可见字段

### 20.5 聚合用途

失败分类可直接用于：

- 某服务失败率
- 某入口 TLS 握手失败率
- 某 IP 被拒绝连接次数
- 配额拒绝连接次数

## 21. UDP / QUIC 连接与限速专项设计

### 21.1 为什么单独设计

`UDP` 和 `QUIC` 不能完全套用 `TCP` 的连接模型。

差异点：

- `UDP` 没有严格的连接关闭语义
- `QUIC` 有会话，但可能包含多路复用流
- 并发连接统计口径不同
- 短时间爆发式报文更常见

### 21.2 统一会话模型

建议对 `UDP/QUIC` 都使用逻辑会话模型：

```go
type DatagramSession struct {
    SessionID         string
    ServiceID         int64
    EndpointID        int64
    UserID            *int64
    SubscriptionID    *int64
    RemoteIP          string
    Protocol          ProtocolType
    Transport         TransportType
    LastSeenAt        time.Time
    PacketCount       int64
    UplinkBytes       int64
    DownlinkBytes     int64
}
```

### 21.3 逻辑会话超时

建议：

- 若 `UDP` 在一定时间无流量，则认为会话结束
- `QUIC` 可优先使用会话标识，其次使用超时回收

示例：

- `UDP` 逻辑会话空闲 30-120 秒回收
- `QUIC` 会话空闲时间可更长

### 21.4 限速维度

建议区分：

- 每秒报文数
- 每秒字节数
- 并发逻辑会话数

不要只看“连接数”。

### 21.5 专项限速模型

```go
type DatagramRateLimitPolicy struct {
    ServiceID                int64
    EndpointID               *int64
    MaxPacketsPerSecond      int
    MaxBytesPerSecond        int64
    MaxConcurrentSessions    int
    BurstPackets             int
    BurstBytes               int64
}
```

持久化说明：

- `DatagramRateLimitPolicy` 作为 `service_access_policies` 表的扩展配置存储
- 通过 `config_json` 字段中的 `datagram_rate_limit` 键保存
- 不单独建表，复用已有的 `service_access_policies` 数据模型
- 运行时读取后缓存在内存中，策略变更时刷新缓存

### 21.6 QUIC 专项失败分类

建议特别区分：

- 握手失败
- 版本不兼容
- 参数协商失败
- 证书校验失败
- 会话限速拒绝

### 21.7 统计建议

监控中建议增加：

- `UDP` 逻辑会话数
- `QUIC` 活跃会话数
- 报文丢弃数
- `QUIC` 握手失败数

### 21.8 安全建议

`UDP/QUIC` 更容易出现突发洪泛，建议：

- 默认启用 burst 限速
- 默认启用短窗口风控
- 高风险入口支持更严格的 `UDP` 限制

## 22. 告警通知与抑制策略设计

### 22.1 目标

告警系统不仅要会发，还要避免：

- 告警风暴
- 重复通知
- 无法升级高危事件
- 已恢复告警长期挂起

### 22.2 告警渠道模型

```go
type AlertChannelType string

const (
    AlertChannelEmail   AlertChannelType = "email"
    AlertChannelWebhook AlertChannelType = "webhook"
    AlertChannelInApp   AlertChannelType = "in_app"
)

type AlertChannel struct {
    ID          int64
    Name        string
    ChannelType AlertChannelType
    ConfigJSON  string
    Status      int
}
```

### 22.3 抑制策略模型

```go
type AlertSuppressionRule struct {
    ID                 int64
    AlertType          string
    DedupWindowSeconds int
    MuteDurationSeconds int
    MaxNotifications   int
    EscalationEnabled  bool
    Status             int
}
```

### 22.4 通知策略

建议支持：

- 首次触发立即通知
- 窗口内重复触发只累加计数
- 达到升级阈值时提高严重级别
- 恢复时发送恢复通知

### 22.5 升级策略

例如：

- 第 1 次：普通告警
- 5 分钟内连续 10 次：升级为高危
- 30 分钟未恢复：再次通知

### 22.6 服务层接口

```go
type AlertDispatchService interface {
    Dispatch(ctx context.Context, alert SecurityAlert) error
    Suppress(ctx context.Context, alert SecurityAlert) (bool, error)
    Escalate(ctx context.Context, alert SecurityAlert) (*SecurityAlert, error)
}
```

### 22.7 告警恢复

建议：

- 状态恢复后自动关闭告警
- 写入 `resolved_at`
- 发送恢复通知

### 22.8 UI 展示建议

前端建议展示：

- 当前告警状态
- 最近通知时间
- 已抑制次数
- 告警升级链路

## 23. 证书、反向代理与动态生效设计

### 23.1 设计目标

证书申请、配置生成、反向代理管理、进程热重载必须走一条统一链路。

当前原则：

- 使用 Go 原生 ACME 客户端（基于 `go-acme/lego`）作为默认 ACME 执行器，跨平台统一
- `Caddy`、`nginx`、各核心不自行重复申请证书
- 证书更新优先动态生效
- 配置更新优先热重载

### 23.2 领域模型

```go
type CertificateProvider string

const (
    CertificateProviderACME   CertificateProvider = "acme"
    CertificateProviderUpload CertificateProvider = "upload"
    CertificateProviderInternalCA CertificateProvider = "internal_ca"
)

type ReverseProxyType string

const (
    ReverseProxyTypeCaddy ReverseProxyType = "caddy"
    ReverseProxyTypeNginx ReverseProxyType = "nginx"
)

type RuntimeApplyMode string

const (
    RuntimeApplyModeHotReload RuntimeApplyMode = "hot_reload"
    RuntimeApplyModeReload    RuntimeApplyMode = "reload"
    RuntimeApplyModeRestart   RuntimeApplyMode = "restart"
)
```

### 23.3 Go 原生 ACME 客户端接口

使用 `go-acme/lego` 库实现 ACME 协议交互，纯 Go 实现，跨平台统一。

```go
type ACMEClient interface {
    Issue(ctx context.Context, in IssueCertificateInput) (*IssueCertificateResult, error)
    Renew(ctx context.Context, in RenewCertificateInput) (*RenewCertificateResult, error)
    Revoke(ctx context.Context, certID int64) error
}

type IssueCertificateInput struct {
    Domains        []string
    ChallengeType  string            // "http-01", "dns-01", "tls-alpn-01"
    DNSProvider    string            // "cloudflare", "alidns", "dnspod", etc.
    DNSCredentials map[string]string // provider-specific credentials
    AccountID      int64
    CAUrl          string            // ACME CA endpoint URL
}

type IssueCertificateResult struct {
    CertPath       string
    KeyPath        string
    FullchainPath  string
    ExpiresAt      time.Time
}
```

### 23.4 证书编排器接口

```go
type CertificateOrchestrator interface {
    Issue(ctx context.Context, certID int64) error
    Renew(ctx context.Context, certID int64) error
    Deploy(ctx context.Context, certID int64) error
    Rollback(ctx context.Context, certID int64, version string) error
}
```

职责：

- 调用 Go 原生 ACME 客户端（`go-acme/lego`）
- 持久化证书元数据
- 更新依赖服务引用
- 触发反向代理和核心动态应用

### 23.5 反向代理管理器接口

```go
type ReverseProxyManager interface {
    RenderConfig(ctx context.Context, proxyID int64) ([]byte, error)
    ValidateConfig(ctx context.Context, proxyID int64, content []byte) error
    ApplyCertificate(ctx context.Context, proxyID int64, certID int64) error
    Reload(ctx context.Context, proxyID int64) error
    Restart(ctx context.Context, proxyID int64) error
}
```

### 23.6 运行时动态应用协调器

```go
type RuntimeApplyCoordinator interface {
    ApplyCertificate(ctx context.Context, target RuntimeTarget, certID int64) error
    ApplyConfig(ctx context.Context, target RuntimeTarget) error
    Rollback(ctx context.Context, target RuntimeTarget, snapshotID string) error
}

type RuntimeConfigBackupManager interface {
    CreateActiveBackup(ctx context.Context, target RuntimeTarget, reason string) (string, error)
    CreateOriginalConfigSnapshot(ctx context.Context, target RuntimeTarget, reason string) (string, error)
    RestoreSnapshot(ctx context.Context, target RuntimeTarget, snapshotID string) error
    VerifyRestored(ctx context.Context, target RuntimeTarget, snapshotID string) error
}

type RuntimeConfigObjectType string

const (
    RuntimeConfigObjectTypeCoreRuntime  RuntimeConfigObjectType = "core_runtime_config"
    RuntimeConfigObjectTypeReverseProxy RuntimeConfigObjectType = "reverse_proxy_config"
    RuntimeConfigObjectTypeDNSMasq      RuntimeConfigObjectType = "dnsmasq_config"
    RuntimeConfigObjectTypeFW4Runtime   RuntimeConfigObjectType = "fw4_runtime_config"
    RuntimeConfigObjectTypeSystemProxy  RuntimeConfigObjectType = "system_proxy_config"
)

type BackupKind string

const (
    BackupKindManual            BackupKind = "manual"
    BackupKindScheduled         BackupKind = "scheduled"
    BackupKindPreApplyAuto      BackupKind = "pre_apply_auto"
    BackupKindPreDisableProxy   BackupKind = "pre_disable_proxy_auto"
    BackupKindOriginalBaseline  BackupKind = "original_baseline"
)

术语约定：

- `BackupKindOriginalBaseline` 对应文档术语 `原始配置基线`
- `BackupKindPreDisableProxy` 对应文档术语 `关闭代理前自动备份`
- `BackupKindPreApplyAuto` 对应文档术语 `变更前自动备份`
- 代码注释、DTO 说明和 UI 文案映射时，不再单独使用“恢复基线”作为正式术语

type BackupRetentionPolicy struct {
    ID                         int64
    Name                       string
    TargetScope                string
    BackupKind                 BackupKind
    RetainLatestCount          int
    RetainDays                 int
    ProtectOriginalBaseline    bool
    ProtectLatestSuccessCount  int
    CleanupEnabled             bool
}

type BackupCleanupJob struct {
    ID             int64
    PolicyID       *int64
    JobScope       string
    Status         string
    Phase          string
    ScannedCount   int
    ExpiredCount   int
    DeletedCount   int
    ProtectedCount int
    ErrorMessage   string
    StartedAt      *time.Time
    FinishedAt     *time.Time
}

type BackupRetentionService interface {
    EvaluateSnapshotRetention(ctx context.Context, snapshotID int64) error
    RunCleanup(ctx context.Context, scope string) (*BackupCleanupJob, error)
    ProtectSnapshot(ctx context.Context, snapshotID int64, reason string) error
}

type RuntimeSnapshotValidationJob struct {
    ID              int64
    SnapshotID      int64
    ValidationScope string
    Status          string
    Phase           string
    ErrorMessage    string
    StartedAt       *time.Time
    FinishedAt      *time.Time
}

type RuntimeSnapshotValidationResult struct {
    ID               int64
    JobID            int64
    SnapshotID       int64
    ConfigObjectType RuntimeConfigObjectType
    FileExists       bool
    ChecksumMatches  bool
    RestoreReady     bool
    ActualPath       string
    ActualChecksum   string
    ExpectedChecksum string
    DetailJSON       string
    CreatedAt        time.Time
}

type RuntimeSnapshotValidationService interface {
    ValidateSnapshot(ctx context.Context, snapshotID int64, scope string) (*RuntimeSnapshotValidationJob, error)
    GetValidationResults(ctx context.Context, jobID int64) ([]RuntimeSnapshotValidationResult, error)
}

type RuntimeTarget struct {
    TargetType string
    TargetID   int64
    ApplyMode  RuntimeApplyMode
}
```

### 23.7 统一动态生效流程

建议流程：

1. 证书签发或续期成功
2. 写入统一证书目录
3. 生成受影响目标列表
4. 对每个目标执行配置校验
5. 优先尝试热重载
6. 若目标不支持热重载，则进入受控重启
7. 任一步失败则回滚受影响目标

### 23.7.1 单节点完整配置应用事务

为了避免“配置已写入但证书未部署”或“反向代理已 reload 但核心未切换”的半成功状态，单节点下应把运行时变更建模为一个统一事务。

```go
type SingleNodeApplyTransaction interface {
    Prepare(ctx context.Context, in SingleNodeApplyInput) (*SingleNodeApplyPlan, error)
    Execute(ctx context.Context, plan SingleNodeApplyPlan) error
    Commit(ctx context.Context, plan SingleNodeApplyPlan) error
    Rollback(ctx context.Context, plan SingleNodeApplyPlan) error
    DisableProxyAndRestore(ctx context.Context, plan SingleNodeDisableProxyPlan) error
}

type SingleNodeApplyInput struct {
    ServiceIDs        []int64
    ReverseProxyIDs   []int64
    CertificateIDs    []int64
    TriggerSource     string
    Reason            string
}

type SingleNodeApplyPlan struct {
    SnapshotID        string
    CoreConfigPaths   []string
    ProxyConfigPaths  []string
    CertificateFiles  []string
    AffectedTargets   []RuntimeTarget
}

type SingleNodeDisableProxyPlan struct {
    BackupSnapshotID    string
    OriginalSnapshotID  string // points to the original baseline snapshot used for disable-proxy restore
    AffectedTargets     []RuntimeTarget
    RequiresRestart     []RuntimeTarget
    RequiresReload      []RuntimeTarget
}
```

执行顺序建议：

1. 锁定受影响服务与反向代理资源
2. 创建统一运行时快照
3. 渲染核心配置与反向代理配置到暂存目录
4. 校验证书引用、端口冲突和配置合法性
5. 原子替换正式配置文件
6. 先应用证书，再应用反向代理，再应用核心配置
7. 对每个目标执行 reload 或 restart
8. 统一做健康检查
9. 成功后提交快照版本
10. 失败则按相反顺序执行回滚

### 23.7.2 单节点应用状态机

```go
type RuntimeApplyState string

const (
    RuntimeApplyStateCreated             RuntimeApplyState = "created"
    RuntimeApplyStateSnapshotting        RuntimeApplyState = "snapshotting"
    RuntimeApplyStateBackingUpActive     RuntimeApplyState = "backing_up_active"
    RuntimeApplyStateRestoringOriginal   RuntimeApplyState = "restoring_original"
    RuntimeApplyStateRendering           RuntimeApplyState = "rendering"
    RuntimeApplyStateValidating          RuntimeApplyState = "validating"
    RuntimeApplyStateStaging             RuntimeApplyState = "staging"
    RuntimeApplyStateApplyingCertificate RuntimeApplyState = "applying_certificate"
    RuntimeApplyStateApplyingProxy       RuntimeApplyState = "applying_proxy"
    RuntimeApplyStateApplyingCore        RuntimeApplyState = "applying_core"
    RuntimeApplyStateHealthChecking      RuntimeApplyState = "health_checking"
    RuntimeApplyStateSucceeded           RuntimeApplyState = "succeeded"
    RuntimeApplyStateFailed              RuntimeApplyState = "failed"
    RuntimeApplyStateRollingBack         RuntimeApplyState = "rolling_back"
    RuntimeApplyStateRolledBack          RuntimeApplyState = "rolled_back"
    RuntimeApplyStateNeedsIntervention   RuntimeApplyState = "needs_intervention"
)

type RuntimeApplyStateMachine interface {
    CanTransit(from RuntimeApplyState, to RuntimeApplyState) bool
    NextOnSuccess(current RuntimeApplyState) RuntimeApplyState
    NextOnFailure(current RuntimeApplyState, rollbackAvailable bool) RuntimeApplyState
}
```

状态要求：

- `validating` 阶段失败时不得写入正式目录
- `applying_proxy` 失败时必须回滚已部署的证书引用
- `health_checking` 超时视为失败
- `rolling_back` 失败时进入 `needs_intervention`
- 任何正式配置修改前都必须先进入 `backing_up_active`
- “关闭代理恢复原配置”必须经过 `restoring_original`

### 23.7.3 单节点运行目录模型

```go
type RuntimePathLayout struct {
    ServiceRoot        string
    ServiceCurrentDir  string
    ServiceStagedDir   string
    ServiceReleaseDir  string
    ProcessStateDir    string
    ProxyRoot          string
    ProxyCurrentDir    string
    ProxyStagedDir     string
    CertificateRoot    string
    CertificateCurrent string
    CertificateVersion string
}

type RuntimeVersionPointer struct {
    TargetType      string
    TargetID        int64
    CurrentVersion  string
    StagedVersion   string
    SnapshotID      string
    UpdatedAt       time.Time
}
```

要求：

- `current` 与 `staged` 必须物理隔离
- 正式版本切换必须使用原子替换或等效机制
- 回滚只允许回退到已校验通过的历史版本

### 23.7.4 单节点应用任务模型

```go
type RuntimeApplyJob struct {
    ID             int64
    TriggerSource  string
    Reason         string
    OperationKind  string
    State          RuntimeApplyState
    SnapshotID     *int64
    OriginalSnapshotID *int64
    TargetCount    int
    SuccessCount   int
    FailedCount    int
    ErrorMessage   string
    StartedAt      *time.Time
    FinishedAt     *time.Time
    CreatedAt      time.Time
    UpdatedAt      time.Time
}

type RuntimeApplyJobTarget struct {
    ID              int64
    JobID           int64
    TargetType      string
    TargetID        int64
    ApplyMode       RuntimeApplyMode
    State           RuntimeApplyState
    Phase           string
    VersionBefore   string
    VersionAfter    string
    RollbackVersion string
    ErrorMessage    string
    StartedAt       *time.Time
    FinishedAt      *time.Time
    CreatedAt       time.Time
    UpdatedAt       time.Time
}

type RuntimeApplyJobEvent struct {
    ID         int64
    JobID      int64
    TargetID   *int64
    EventLevel string
    EventType  string
    Phase      string
    Message    string
    DetailJSON string
    CreatedAt  time.Time
}

type RuntimeApplyJobQuery struct {
    Status        []RuntimeApplyState
    TriggerSource []string
    TargetType    []string
    CreatedAfter  *time.Time
    CreatedBefore *time.Time
    Limit         int
    Offset        int
}

type RuntimeApplyJobRepository interface {
    Create(ctx context.Context, job *RuntimeApplyJob) error
    Update(ctx context.Context, job *RuntimeApplyJob) error
    Get(ctx context.Context, id int64) (*RuntimeApplyJob, error)
    List(ctx context.Context, q RuntimeApplyJobQuery) ([]*RuntimeApplyJob, error)
    ListTargets(ctx context.Context, jobID int64) ([]*RuntimeApplyJobTarget, error)
    ListEvents(ctx context.Context, jobID int64) ([]*RuntimeApplyJobEvent, error)
}
```

建议：

- `RuntimeApplyJob` 只保存任务总览
- `RuntimeApplyJobTarget` 保存每个运行对象的执行结果
- `RuntimeApplyJobEvent` 保存阶段事件流和结构化错误细节

### 23.7.4.0 Gorm Model 建议

```go
type RuntimeApplyJobModel struct {
    ID            int64      `gorm:"primaryKey"`
    TriggerSource string     `gorm:"size:32;index:idx_runtime_apply_jobs_trigger_created,priority:1;not null"`
    Reason        string     `gorm:"type:text"`
    OperationKind string     `gorm:"size:32;not null;default:'apply'"`
    Status        string     `gorm:"size:32;index:idx_runtime_apply_jobs_status_created,priority:1;not null"`
    Phase         string     `gorm:"size:64"`
    SnapshotID    *int64     `gorm:"index"`
    OriginalSnapshotID *int64 `gorm:"index"`
    TargetCount   int        `gorm:"not null;default:0"`
    SuccessCount  int        `gorm:"not null;default:0"`
    FailedCount   int        `gorm:"not null;default:0"`
    ErrorMessage  string     `gorm:"type:text"`
    StartedAt     *time.Time
    FinishedAt    *time.Time
    CreatedAt     time.Time  `gorm:"index:idx_runtime_apply_jobs_status_created,priority:2;index:idx_runtime_apply_jobs_trigger_created,priority:2;not null"`
    UpdatedAt     time.Time  `gorm:"not null"`
}

type RuntimeApplyJobTargetModel struct {
    ID              int64      `gorm:"primaryKey"`
    JobID           int64      `gorm:"index:idx_runtime_apply_job_targets_job_target,priority:1;not null"`
    TargetType      string     `gorm:"size:32;index:idx_runtime_apply_job_targets_job_target,priority:2;index:idx_runtime_apply_job_targets_type_id,priority:1;not null"`
    TargetID        int64      `gorm:"index:idx_runtime_apply_job_targets_type_id,priority:2;not null"`
    ApplyMode       string     `gorm:"size:32;not null"`
    Status          string     `gorm:"size:32;not null"`
    Phase           string     `gorm:"size:64"`
    VersionBefore   string     `gorm:"size:64"`
    VersionAfter    string     `gorm:"size:64"`
    RollbackVersion string     `gorm:"size:64"`
    ErrorMessage    string     `gorm:"type:text"`
    StartedAt       *time.Time
    FinishedAt      *time.Time
    CreatedAt       time.Time  `gorm:"not null"`
    UpdatedAt       time.Time  `gorm:"not null"`
}

type RuntimeApplyJobEventModel struct {
    ID         int64     `gorm:"primaryKey"`
    JobID      int64     `gorm:"index:idx_runtime_apply_job_events_job_created,priority:1;not null"`
    TargetID   *int64
    EventLevel string    `gorm:"size:16;not null"`
    EventType  string    `gorm:"size:64;not null"`
    Phase      string    `gorm:"size:64"`
    Message    string    `gorm:"type:text;not null"`
    DetailJSON string    `gorm:"type:text"`
    CreatedAt  time.Time `gorm:"index:idx_runtime_apply_job_events_job_created,priority:2;not null"`
}
```

约束建议：

- `RuntimeApplyJobModel` 与 `RuntimeApplyJobTargetModel` 使用逻辑外键关系，由 repository 统一维护
- 删除任务主记录时默认不物理级联删除明细，优先使用归档策略
- 事件表不做更新，只允许插入

### 23.7.4.1 任务服务接口

```go
type RuntimeApplyService interface {
    DryRun(ctx context.Context, in SingleNodeApplyInput) (*RuntimeDryRunResult, error)
    CreateJob(ctx context.Context, in SingleNodeApplyInput) (*RuntimeApplyJob, error)
    ExecuteJob(ctx context.Context, jobID int64) error
    DisableProxy(ctx context.Context, targets []RuntimeTarget, reason string) (*RuntimeApplyJob, error)
    RetryJob(ctx context.Context, jobID int64, reason string) error
    CancelJob(ctx context.Context, jobID int64, reason string) error
    GetJobDetail(ctx context.Context, jobID int64) (*RuntimeApplyJobDetail, error)
}


关闭代理恢复 DTO 建议：

```go
type DisableProxyRequest struct {
    Reason                           string   `json:"reason"`
    TargetTypes                      []string `json:"target_types"`
    RequireOriginalSnapshotValidation bool    `json:"require_original_snapshot_validation"`
    FallbackToPreDisableBackup       bool     `json:"fallback_to_pre_disable_backup"`
}

type DisableProxyResponse struct {
    JobID              int64             `json:"job_id"`
    OperationKind      string            `json:"operation_kind"`
    Status             RuntimeApplyState `json:"status"`
    Phase              string            `json:"phase"`
    SnapshotID         *int64            `json:"snapshot_id"`
    OriginalSnapshotID *int64            `json:"original_snapshot_id"`
    CanRetry           bool              `json:"can_retry"`
    CanRollback        bool              `json:"can_rollback"`
}

type RuntimeRestoreTargetState string

const (
    RuntimeRestoreTargetStatePending    RuntimeRestoreTargetState = "pending"
    RuntimeRestoreTargetStateSkipped    RuntimeRestoreTargetState = "skipped"
    RuntimeRestoreTargetStateSucceeded  RuntimeRestoreTargetState = "succeeded"
    RuntimeRestoreTargetStateFailed     RuntimeRestoreTargetState = "failed"
    RuntimeRestoreTargetStateRolledBack RuntimeRestoreTargetState = "rolled_back"
)

type RuntimeRestoreApplyMode string

const (
    RuntimeRestoreApplyModeHotReload RuntimeRestoreApplyMode = "hot_reload"
    RuntimeRestoreApplyModeReload    RuntimeRestoreApplyMode = "reload"
    RuntimeRestoreApplyModeRestart   RuntimeRestoreApplyMode = "restart"
    RuntimeRestoreApplyModeNoop      RuntimeRestoreApplyMode = "noop"
)

type RuntimeFallbackBackupStatus string

const (
    RuntimeFallbackBackupStatusNotStarted RuntimeFallbackBackupStatus = "not_started"
    RuntimeFallbackBackupStatusRunning    RuntimeFallbackBackupStatus = "running"
    RuntimeFallbackBackupStatusSucceeded  RuntimeFallbackBackupStatus = "succeeded"
    RuntimeFallbackBackupStatusFailed     RuntimeFallbackBackupStatus = "failed"
    RuntimeFallbackBackupStatusNotRequired RuntimeFallbackBackupStatus = "not_required"
)

type RuntimeRestoreTargetStatus struct {
    ConfigObjectType RuntimeConfigObjectType `json:"config_object_type"`
    SnapshotID       *int64                  `json:"snapshot_id"`
    RestoreStatus    RuntimeRestoreTargetState `json:"restore_status"`
    ApplyMode        RuntimeRestoreApplyMode `json:"apply_mode"`
    ErrorCode        string                  `json:"error_code,omitempty"`
    ErrorMessage     string                  `json:"error_message,omitempty"`
}

type RuntimeValidationObjectSummary struct {
    ConfigObjectType  RuntimeConfigObjectType `json:"config_object_type"`
    FileReadyCount    int                     `json:"file_ready_count"`
    RestoreReadyCount int                     `json:"restore_ready_count"`
    FailedCount       int                     `json:"failed_count"`
}

type RuntimeSnapshotValidationSummary struct {
    JobID              int                            `json:"job_id"`
    ValidationScope    string                         `json:"validation_scope"`
    FileReadyCount     int                            `json:"file_ready_count"`
    RestoreReadyCount  int                            `json:"restore_ready_count"`
    FailedCount        int                            `json:"failed_count"`
    ObjectSummaries    []RuntimeValidationObjectSummary `json:"object_summaries,omitempty"`
}

对象统计顺序要求：

- `ObjectSummaries` 必须固定按以下顺序输出，避免不同页面排序不一致：
  1. `core_runtime_config`
  2. `reverse_proxy_config`
  3. `dnsmasq_config`
  4. `fw4_runtime_config`
  5. `system_proxy_config`

type RuntimeDryRunResult struct {
    OK               bool
    SnapshotRequired bool
    AffectedTargets  []RuntimeTarget
    PortConflicts    []string
    MissingCerts     []int64
    ReloadPlan       []RuntimeTarget
}
```

取消规则：

- 仅 `created`、`snapshotting`、`rendering`、`validating` 阶段允许取消
- 进入 `staging` 后只允许失败回滚，不允许普通取消
- `succeeded`、`rolled_back`、`needs_intervention` 状态不允许取消

### 23.7.4.2 API DTO 建议

```go
type RuntimeApplyRequest struct {
    ServiceIDs       []int64 `json:"service_ids"`
    ReverseProxyIDs  []int64 `json:"reverse_proxy_ids"`
    CertificateIDs   []int64 `json:"certificate_ids"`
    TriggerSource    string  `json:"trigger_source"`
    Reason           string  `json:"reason"`
    DryRunToken      string  `json:"dry_run_token,omitempty"`
}

type RuntimeDryRunRequest struct {
    ServiceIDs       []int64 `json:"service_ids"`
    ReverseProxyIDs  []int64 `json:"reverse_proxy_ids"`
    CertificateIDs   []int64 `json:"certificate_ids"`
    TriggerSource    string  `json:"trigger_source"`
    Reason           string  `json:"reason"`
}

type RuntimeJobRetryRequest struct {
    Reason string `json:"reason"`
}

type RuntimeJobCancelRequest struct {
    Reason string `json:"reason"`
}

type RuntimeApplyJobListItem struct {
    ID               int64             `json:"id"`
    TriggerSource    string            `json:"trigger_source"`
    Reason           string            `json:"reason"`
    Status           RuntimeApplyState `json:"status"`
    Phase            string            `json:"phase"`
    TargetCount      int               `json:"target_count"`
    SuccessCount     int               `json:"success_count"`
    FailedCount      int               `json:"failed_count"`
    RetryCount       int               `json:"retry_count"`
    RequiresAction   bool              `json:"requires_action"`
    StartedAt        *time.Time        `json:"started_at"`
    FinishedAt       *time.Time        `json:"finished_at"`
    DurationSeconds  int64             `json:"duration_seconds"`
    FirstError       string            `json:"first_error,omitempty"`
}

type RuntimeApplyJobTargetDTO struct {
    TargetType      string            `json:"target_type"`
    TargetID        int64             `json:"target_id"`
    TargetName      string            `json:"target_name"`
    ApplyMode       RuntimeApplyMode  `json:"apply_mode"`
    Status          RuntimeApplyState `json:"status"`
    Phase           string            `json:"phase"`
    VersionBefore   string            `json:"version_before"`
    VersionAfter    string            `json:"version_after"`
    RollbackVersion string            `json:"rollback_version"`
    ErrorMessage    string            `json:"error_message,omitempty"`
}

type RuntimeApplyJobEventDTO struct {
    EventLevel string          `json:"event_level"`
    EventType  string          `json:"event_type"`
    Phase      string          `json:"phase"`
    Message    string          `json:"message"`
    Detail     json.RawMessage `json:"detail,omitempty"`
    CreatedAt  time.Time       `json:"created_at"`
}

type RuntimeApplyEventDetail struct {
    ErrorCode            string   `json:"error_code,omitempty"`
    Component            string   `json:"component,omitempty"`
    ConfigObjectType     string   `json:"config_object_type,omitempty"`
    SnapshotID           *int64   `json:"snapshot_id,omitempty"`
    ExpectedPath         string   `json:"expected_path,omitempty"`
    ActualPath           string   `json:"actual_path,omitempty"`
    ExpectedChecksum     string   `json:"expected_checksum,omitempty"`
    ActualChecksum       string   `json:"actual_checksum,omitempty"`
    ApplyMode            RuntimeRestoreApplyMode `json:"apply_mode,omitempty"`
    ExitCode             *int     `json:"exit_code,omitempty"`
    StderrExcerpt        string   `json:"stderr_excerpt,omitempty"`
    VerificationItems    []string `json:"verification_items,omitempty"`
    FallbackBackupStatus RuntimeFallbackBackupStatus `json:"fallback_backup_status,omitempty"`
}

type RuntimeApplyJobDetailResponse struct {
    ID                  int64                      `json:"id"`
    TriggerSource       string                     `json:"trigger_source"`
    Reason              string                     `json:"reason"`
    OperationKind       string                     `json:"operation_kind"`
    Status              RuntimeApplyState          `json:"status"`
    Phase               string                     `json:"phase"`
    SnapshotID          *int64                     `json:"snapshot_id"`
    OriginalSnapshotID  *int64                     `json:"original_snapshot_id"`
    RetryCount          int                        `json:"retry_count"`
    RequiresAction      bool                       `json:"requires_action"`
    RequiresActionReason string                    `json:"requires_action_reason,omitempty"`
    CanRetry            bool                       `json:"can_retry"`
    CanCancel           bool                       `json:"can_cancel"`
    CanRollback         bool                       `json:"can_rollback"`
    CurrentErrorCode    string                     `json:"current_error_code,omitempty"`
    FallbackBackupStatus RuntimeFallbackBackupStatus `json:"fallback_backup_status,omitempty"`
    TargetCount         int                        `json:"target_count"`
    SuccessCount        int                        `json:"success_count"`
    FailedCount         int                        `json:"failed_count"`
    StartedAt           *time.Time                 `json:"started_at"`
    FinishedAt          *time.Time                 `json:"finished_at"`
    ValidationSummary   *RuntimeSnapshotValidationSummary `json:"validation_summary,omitempty"`
    RestoreTargets      []RuntimeRestoreTargetStatus      `json:"restore_targets,omitempty"`
    Targets             []*RuntimeApplyJobTargetDTO `json:"targets"`
    Events              []*RuntimeApplyJobEventDTO  `json:"events"`
}
```

说明：

- 列表接口返回 `RuntimeApplyJobListItem`
- 详情接口返回 `RuntimeApplyJobDetailResponse`
- 前端按钮是否可点击尽量由后端返回 `CanRetry`、`CanCancel`、`CanRollback`
- `RequiresAction=true` 时必须同时返回 `RequiresActionReason`
- `RequiresActionReason` 建议使用稳定值，如 `missing_original_baseline`、`corrupted_original_snapshot`、`fallback_restore_failed`、`restore_not_ready`、`manual_conflict_resolution_required`

### 23.7.4.3 列表筛选参数建议

```go
type RuntimeApplyJobListQuery struct {
    Status        []RuntimeApplyState `form:"status"`
    Phase         []string            `form:"phase"`
    TriggerSource []string            `form:"trigger_source"`
    TargetType    []string            `form:"target_type"`
    Keyword       string              `form:"keyword"`
    FailedOnly    bool                `form:"failed_only"`
    StartedAfter  *time.Time          `form:"started_after"`
    StartedBefore *time.Time          `form:"started_before"`
    Limit         int                 `form:"limit"`
    Offset        int                 `form:"offset"`
}
```

建议：

- `Keyword` 同时匹配任务 ID、原因、首个错误摘要
- `FailedOnly=true` 时自动包含 `failed`、`needs_intervention`
- 默认分页返回，避免事件流过大拖慢页面

### 23.7.4.4 Handler 与错误码建议

建议 Handler 放在：

```text
internal/api/handlers/runtime/
  overview.go
  apply_job_list.go
  apply_job_detail.go
  dry_run.go
  apply.go
  retry.go
  cancel.go
  rollback.go
```

建议错误码：

```go
type RuntimeErrorCode string

const (
    RuntimeErrInvalidRequest        RuntimeErrorCode = "runtime_invalid_request"
    RuntimeErrJobNotFound           RuntimeErrorCode = "runtime_job_not_found"
    RuntimeErrJobNotCancelable      RuntimeErrorCode = "runtime_job_not_cancelable"
    RuntimeErrJobNotRetryable       RuntimeErrorCode = "runtime_job_not_retryable"
    RuntimeErrConcurrentApply       RuntimeErrorCode = "runtime_concurrent_apply"
    RuntimeErrDryRunRequired        RuntimeErrorCode = "runtime_dry_run_required"
    RuntimeErrSnapshotMissing       RuntimeErrorCode = "runtime_snapshot_missing"
    RuntimeErrValidationFailed      RuntimeErrorCode = "runtime_validation_failed"
    RuntimeErrRollbackRequired      RuntimeErrorCode = "runtime_rollback_required"
    RuntimeErrNeedsIntervention     RuntimeErrorCode = "runtime_needs_intervention"
    RuntimeErrDisableProxyOriginalSnapshotMissing RuntimeErrorCode = "runtime_disable_proxy_original_snapshot_missing"
    RuntimeErrDisableProxyOriginalSnapshotCorrupted RuntimeErrorCode = "runtime_disable_proxy_original_snapshot_corrupted"
    RuntimeErrDisableProxyCoreRestoreFailed RuntimeErrorCode = "runtime_disable_proxy_core_restore_failed"
    RuntimeErrDisableProxyReverseProxyRestoreFailed RuntimeErrorCode = "runtime_disable_proxy_proxy_restore_failed"
    RuntimeErrDisableProxyDNSMasqRestoreFailed RuntimeErrorCode = "runtime_disable_proxy_dnsmasq_restore_failed"
    RuntimeErrDisableProxyFW4RestoreFailed RuntimeErrorCode = "runtime_disable_proxy_fw4_restore_failed"
    RuntimeErrDisableProxySystemProxyRestoreFailed RuntimeErrorCode = "runtime_disable_proxy_system_proxy_restore_failed"
    RuntimeErrDisableProxyReloadFailed RuntimeErrorCode = "runtime_disable_proxy_reload_failed"
    RuntimeErrDisableProxyVerifyFailed RuntimeErrorCode = "runtime_disable_proxy_verify_failed"
    RuntimeErrDisableProxyRecoverBackupFailed RuntimeErrorCode = "runtime_disable_proxy_recover_backup_failed"
)
```

HTTP 返回建议：

- 参数错误返回 `400`
- 任务不存在返回 `404`
- 状态冲突、并发冲突、不可取消、不可重试返回 `409`
- 校验失败但请求格式正确返回 `422`
- 需要人工介入返回 `409`，并附带 `requires_action=true`

### 23.7.4.5 Repository 扩展建议

```go
type RuntimeApplyJobRepository interface {
    Create(ctx context.Context, job *RuntimeApplyJob) error
    Update(ctx context.Context, job *RuntimeApplyJob) error
    Get(ctx context.Context, id int64) (*RuntimeApplyJob, error)
    List(ctx context.Context, q RuntimeApplyJobQuery) ([]*RuntimeApplyJob, error)
    Count(ctx context.Context, q RuntimeApplyJobQuery) (int64, error)
    ListTargets(ctx context.Context, jobID int64) ([]*RuntimeApplyJobTarget, error)
    UpsertTarget(ctx context.Context, target *RuntimeApplyJobTarget) error
    ListEvents(ctx context.Context, jobID int64) ([]*RuntimeApplyJobEvent, error)
    AppendEvent(ctx context.Context, event *RuntimeApplyJobEvent) error
    WithTx(ctx context.Context, fn func(repo RuntimeApplyJobRepository) error) error
}
```

说明：

- `Count` 用于分页列表
- `UpsertTarget` 用于执行链路中的阶段更新
- `AppendEvent` 只追加不修改
- `WithTx` 保证任务主表、目标表、事件表的状态更新一致

### 23.7.4.5.1 查询策略与排序约定

建议固定如下：

- 列表默认排序：`created_at DESC, id DESC`
- `failed_only=true` 时排序：`updated_at DESC, id DESC`
- 详情事件流排序：`created_at ASC, id ASC`
- 目标明细排序：失败优先，其次运行中，最后成功

Repository 查询建议：

```go
type RuntimeApplyJobRepository interface {
    Create(ctx context.Context, job *RuntimeApplyJob) error
    Update(ctx context.Context, job *RuntimeApplyJob) error
    Get(ctx context.Context, id int64) (*RuntimeApplyJob, error)
    List(ctx context.Context, q RuntimeApplyJobQuery) ([]*RuntimeApplyJob, error)
    Count(ctx context.Context, q RuntimeApplyJobQuery) (int64, error)
    ListTargets(ctx context.Context, jobID int64) ([]*RuntimeApplyJobTarget, error)
    UpsertTarget(ctx context.Context, target *RuntimeApplyJobTarget) error
    ListEvents(ctx context.Context, jobID int64, limit int, offset int) ([]*RuntimeApplyJobEvent, error)
    AppendEvent(ctx context.Context, event *RuntimeApplyJobEvent) error
    BatchFirstErrors(ctx context.Context, jobIDs []int64) (map[int64]string, error)
    WithTx(ctx context.Context, fn func(repo RuntimeApplyJobRepository) error) error
}
```

查询要求：

- 列表页先查主任务，再批量查询首个错误摘要
- 详情页禁止通过一个超大联表一次性加载全部内容
- 事件流查询必须支持分页，避免单任务事件量过大
- 运行中任务允许短期缓存，但缓存失效后必须回读数据库

### 23.7.4.5.2 迁移文件建议

建议命名：

```text
migrations/
  2026052201_create_runtime_apply_jobs.sql
  2026052202_create_runtime_apply_job_targets.sql
  2026052203_create_runtime_apply_job_events.sql
  2026052204_add_runtime_apply_indexes.sql
```

要求：

- 迁移编号按时间递增
- 表结构变更与索引变更分文件管理
- 回滚脚本需要同步维护删除索引与删除表顺序

### 23.7.4.6 路由注册建议

建议注册到：

```go
func RegisterRuntimeRoutes(rg *gin.RouterGroup, h *runtime.Handler) {
    runtimeRG := rg.Group("/runtime")
    {
        runtimeRG.GET("/overview", h.GetOverview)
        runtimeRG.GET("/snapshots", h.ListSnapshots)
        runtimeRG.GET("/apply-jobs", h.ListApplyJobs)
        runtimeRG.GET("/apply-jobs/:id", h.GetApplyJobDetail)
        runtimeRG.POST("/dry-run", h.DryRun)
        runtimeRG.POST("/apply", h.Apply)
        runtimeRG.POST("/apply-jobs/:id/retry", h.RetryApplyJob)
        runtimeRG.POST("/apply-jobs/:id/cancel", h.CancelApplyJob)
        runtimeRG.POST("/rollback", h.Rollback)
    }
}
```

要求：

- `overview` 与 `apply-jobs` 分开，避免列表接口承担总览统计
- 详情接口单独返回事件流，避免列表接口过重
- `dry-run` 与 `apply` 必须拆开，便于前端做确认流程

### 23.7.4.7 前端 Store 设计建议

```ts
export interface RuntimeState {
  overview: RuntimeOverview | null
  jobs: RuntimeApplyJobListItem[]
  total: number
  filters: RuntimeApplyJobListQuery
  selectedJobId: number | null
  jobDetail: RuntimeApplyJobDetailResponse | null
  listLoading: boolean
  detailLoading: boolean
  lastListRefreshAt: string | null
  lastDetailRefreshAt: string | null
  pollingEnabled: boolean
}
```

建议 action：

```ts
fetchOverview()
fetchJobs()
fetchJobDetail(jobId: number)
runDryRun(payload: RuntimeDryRunRequest)
runApply(payload: RuntimeApplyRequest)
retryJob(jobId: number, payload: RuntimeJobRetryRequest)
cancelJob(jobId: number, payload: RuntimeJobCancelRequest)
setFilters(filters: Partial<RuntimeApplyJobListQuery>)
togglePolling(enabled: boolean)
```

要求：

- 列表轮询与详情轮询分离
- 打开抽屉后只刷新当前 `selectedJobId`
- 切换筛选条件时重置分页

### 23.7.4.7.1 Store 生命周期建议

页面首次进入：

1. 先并行拉取 `overview` 与 `jobs`
2. 若路由带 `job_id` 参数，则自动打开详情抽屉
3. 初始化完成后再决定是否开启轮询

打开详情抽屉：

1. 设置 `selectedJobId`
2. 立即请求一次详情
3. 若任务未终态，则启动详情轮询

关闭详情抽屉：

1. 停止详情轮询
2. 清空 `selectedJobId`
3. 保留列表筛选条件与分页位置

切换筛选条件：

1. 停止当前列表轮询
2. 更新 `filters`
3. 重置 `offset=0`
4. 请求新列表
5. 请求成功后恢复轮询

### 23.7.4.7.2 前端错误展示与状态回退

建议规则：

- `409` 并发冲突直接显示后端错误原因，并刷新当前任务状态
- `422` 校验失败时保留用户当前筛选与抽屉上下文
- 轮询失败时不清空 `jobs` 与 `jobDetail`
- 手动操作失败后只回退按钮 loading，不回退整页数据
- 若详情任务已不存在，关闭抽屉并弹出一次性提示

### 23.7.4.8 前端 API 封装建议

```ts
export function getRuntimeOverview()
export function getRuntimeApplyJobs(query: RuntimeApplyJobListQuery)
export function getRuntimeApplyJobDetail(jobId: number)
export function postRuntimeDryRun(payload: RuntimeDryRunRequest)
export function postRuntimeApply(payload: RuntimeApplyRequest)
export function postRuntimeRetry(jobId: number, payload: RuntimeJobRetryRequest)
export function postRuntimeCancel(jobId: number, payload: RuntimeJobCancelRequest)
export function postRuntimeRollback(payload: RuntimeRollbackRequest)
```

要求：

- 所有函数统一放在 `api/runtime.ts`
- 统一透传后端错误码，避免组件层自行猜测错误原因
- 对 `409` 与 `422` 做统一错误解析，直接映射到页面提示

## 25. 前端多语言设计

### 25.1 目标

管理端前端必须支持多语言，并遵循以下优先级：

1. 用户手动选择的语言
2. 浏览器语言
3. 英语默认回退

### 25.2 初始化规则

建议初始化逻辑：

```ts
type SupportedLocale = 'zh-CN' | 'en'

function resolveInitialLocale(): SupportedLocale {
  const saved = localStorage.getItem('console.locale')
  if (saved === 'zh-CN' || saved === 'en') return saved

  const browser = navigator.language
  if (browser.toLowerCase().startsWith('zh')) return 'zh-CN'
  return 'en'
}
```

要求：

- 浏览器语言无法命中已支持语言时，直接回退到 `en`
- 手动切换后写入本地存储
- 语言切换为前端本地行为，不依赖后端会话接口才可生效

### 25.3 语言资源组织

建议目录：

```text
web/admin/src/locales/
  index.ts
  zh-CN.ts
  en.ts
```

建议：

- 按页面域拆分语言 key
- key 使用稳定语义命名，不直接使用中文文案本身
- 新增语言包时不改组件逻辑，只补资源文件

### 25.4 i18n 实例建议

```ts
export const i18n = createI18n({
  legacy: false,
  locale: resolveInitialLocale(),
  fallbackLocale: 'en',
  messages: {
    'zh-CN': zhCNMessages,
    en: enMessages,
  },
})
```

要求：

- `fallbackLocale` 固定为 `en`
- 缺失翻译键统一回退到英语
- 状态枚举与错误码显示文案也走语言包映射

### 25.5 Store 与 UI 交互

建议增加：

```ts
export interface LocaleState {
  locale: 'zh-CN' | 'en'
  followBrowser: boolean
}
```

建议 action：

```ts
setLocale(locale: 'zh-CN' | 'en')
enableFollowBrowser()
hydrateLocale()
```

交互要求：

- 语言切换后立即更新顶部栏、菜单、表格、弹窗文案
- 切换语言不重置当前页面状态
- 运行时任务列表和详情抽屉切换语言后只更新文案，不重新触发业务操作

## 26. 客户端多订阅合并与核心切换设计

边界说明：

- 本章只描述客户端“使用外部订阅”的能力
- 外部订阅包括别人分享的订阅、购买获得的订阅和手动录入的订阅源
- 本章不描述服务端“生成订阅给别人导入”的能力
- 服务端节点分享、二维码生成、节点勾选订阅导出应使用独立的数据模型与 API

### 26.1 目标

客户端配置页需要支持：

- 默认进入时预选 `sing-box`
- 用户进入后可手动切换到 `Xray-core` 或 `Mihomo`
- 支持多个订阅源统一拉取、解析、去重、排序、合并
- 原始订阅同步与新配置生成解耦
- 合并结果作为新的客户端可运行配置保存
- 支持 `proxy` 与 `tun` 两种运行模式，且 `TUN` 可关闭

### 26.2 领域模型

```go
type ClientTargetCore string

const (
    ClientTargetCoreSingBox ClientTargetCore = "sing-box"
    ClientTargetCoreXray    ClientTargetCore = "xray-core"
    ClientTargetCoreMihomo  ClientTargetCore = "mihomo"
)

type MergeConflictPolicy string

const (
    MergeConflictKeepFirst MergeConflictPolicy = "keep_first"
    MergeConflictKeepLast  MergeConflictPolicy = "keep_last"
)

type ClientSubscriptionSource struct {
    ID              int64
    Name            string
    URL             string
    Enabled         bool
    Priority        int
    LastSyncAt      *time.Time
    LastSyncStatus  string
}

type ClientSubscriptionMergeProfile struct {
    ID                int64
    Name              string
    DefaultTargetCore ClientTargetCore
    SelectedCore      ClientTargetCore
    Sources           []ClientSubscriptionSource
    ConflictPolicy    MergeConflictPolicy
    Deduplicate       bool
    Platform          string
    TunEnabled        bool
}

type ClientRunMode string

const (
    ClientRunModeProxy ClientRunMode = "proxy"
    ClientRunModeTun   ClientRunMode = "tun"
)

type ClientGeneratedProfile struct {
    ID                    int64
    ProfileID             int64
    TargetCore            ClientTargetCore
    RunMode               ClientRunMode
    ConfigFormat          string
    ConfigContent         string
    SourceSnapshotVersion string
    RouteProfileVersion   string
    Status                string
    CreatedAt             time.Time
    UpdatedAt             time.Time
}

type ClientProxyMode string

const (
    ClientProxyModeRule   ClientProxyMode = "rule"
    ClientProxyModeGlobal ClientProxyMode = "global"
    ClientProxyModeDirect ClientProxyMode = "direct"
)

type ClientBypassMode string

const (
    ClientBypassModeWhitelist ClientBypassMode = "whitelist"
    ClientBypassModeBlacklist ClientBypassMode = "blacklist"
)

type ClientRouteProfile struct {
    ID          int64
    ProfileID   int64
    Name        string
    ProxyMode   ClientProxyMode
    BypassMode  ClientBypassMode
    TunEnabled  bool
    IPv6Enabled bool
    QUICEnabled bool
    DNSPolicy   string
    FirewallBackend AddressSetBackend
    NameProfileKey  string
    NameScopeStrategy string
    NameRevisionStrategy string
    FW4IncludeName  string
    DNSMasqIncludeName string
    Rules       []ClientRouteRule
}

type ClientRouteRule struct {
    Name        string
    ID          int64
    SourceType  string
    RuleType    string
    MatchType   string
    MatchValue  string
    Action      string
    OutboundTag string
    DirectApplyScope string
    ResolverType string
    AddressSetType string
    Priority    int
    Remark      string
    Status      int
}

type ClientRouteRuleSource struct {
    ID                  int64
    RouteProfileID      int64
    SourceType          string
    Name                string
    SourceURL           string
    SyncIntervalSeconds int
    LastSyncAt          *time.Time
    LastSyncStatus      string
    LastErrorMessage    string
    LastETag            string
    LastModified        string
    LastCheckedAt       *time.Time
    Enabled             bool
}

type ClientRouteRuleSnapshot struct {
    ID                 int64
    SourceID           int64
    SnapshotVersion    string
    RawContent         string
    NormalizedRuleJSON string
    CreatedAt          time.Time
}

type ClientRouteSourceType string

const (
    ClientRouteSourceRemoteCIDRv4 ClientRouteSourceType = "remote_cidr_v4"
    ClientRouteSourceRemoteCIDRv6 ClientRouteSourceType = "remote_cidr_v6"
    ClientRouteSourceLocalManual  ClientRouteSourceType = "local_manual"
)

type ClientRouteOutputBackend string

const (
    ClientRouteOutputIPTables ClientRouteOutputBackend = "iptables"
    ClientRouteOutputIPSet    ClientRouteOutputBackend = "ipset"
    ClientRouteOutputNFT      ClientRouteOutputBackend = "nftables"
    ClientRouteOutputFW4      ClientRouteOutputBackend = "fw4"
)

type ClientRouteRuleSourcePriority string

const (
    ClientRouteRuleSourcePriorityLocalException ClientRouteRuleSourcePriority = "local_exception"
    ClientRouteRuleSourcePriorityLocalNormal    ClientRouteRuleSourcePriority = "local_normal"
    ClientRouteRuleSourcePriorityRemote         ClientRouteRuleSourcePriority = "remote"
    ClientRouteRuleSourcePriorityBuiltin        ClientRouteRuleSourcePriority = "builtin"
)

type ClientRuntimeProfile struct {
    ID                       int64
    ProfileID                int64
    ActiveGeneratedProfileID *int64
    ActiveRevisionNo         *int
    LastAppliedAt            *time.Time
    LastApplyStatus          string
    LastErrorMessage         string
}

type ClientGenerateState string

const (
    ClientGenerateStateCreated             ClientGenerateState = "created"
    ClientGenerateStateResolvingSources    ClientGenerateState = "resolving_sources"
    ClientGenerateStateMerging             ClientGenerateState = "merging"
    ClientGenerateStateApplyingRoute       ClientGenerateState = "applying_route_profile"
    ClientGenerateStateRendering           ClientGenerateState = "rendering"
    ClientGenerateStatePersistingRevision  ClientGenerateState = "persisting_revision"
    ClientGenerateStateGenerated           ClientGenerateState = "generated"
    ClientGenerateStateFailed              ClientGenerateState = "generate_failed"
)

type ClientApplyState string

const (
    ClientApplyStateCreated            ClientApplyState = "created"
    ClientApplyStateSnapshotting       ClientApplyState = "snapshotting_runtime"
    ClientApplyStateValidating         ClientApplyState = "validating_generated_profile"
    ClientApplyStateStaging            ClientApplyState = "staging_runtime_files"
    ClientApplyStateStoppingPrevious   ClientApplyState = "stopping_previous_runtime"
    ClientApplyStateStartingNew        ClientApplyState = "starting_new_runtime"
    ClientApplyStateHealthChecking     ClientApplyState = "health_checking"
    ClientApplyStateApplied            ClientApplyState = "applied"
    ClientApplyStateFailed             ClientApplyState = "apply_failed"
    ClientApplyStateRollingBack        ClientApplyState = "rolling_back"
    ClientApplyStateRolledBack         ClientApplyState = "rolled_back"
    ClientApplyStateNeedsIntervention  ClientApplyState = "needs_intervention"
)

type ClientRuntimeApplyJob struct {
    ID                         int64
    ProfileID                  int64
    GeneratedProfileID         int64
    RevisionNo                 int
    TargetCore                 ClientTargetCore
    RunMode                    ClientRunMode
    Status                     ClientApplyState
    Phase                      string
    ErrorCode                  string
    ErrorMessage               string
    RollbackGeneratedProfileID *int64
    RollbackRevisionNo         *int
    StartedAt                  *time.Time
    FinishedAt                 *time.Time
}

type ClientRuntimeLog struct {
    ID                 int64
    ProfileID          int64
    GeneratedProfileID *int64
    RevisionNo         *int
    Level              string
    LogType            string
    Message            string
    DetailJSON         string
    OccurredAt         time.Time
}

type ClientRuleSyncState string

const (
    ClientRuleSyncStateCreated            ClientRuleSyncState = "created"
    ClientRuleSyncStateFetching           ClientRuleSyncState = "fetching"
    ClientRuleSyncStateValidating         ClientRuleSyncState = "validating"
    ClientRuleSyncStateNormalizing        ClientRuleSyncState = "normalizing"
    ClientRuleSyncStatePersistingSnapshot ClientRuleSyncState = "persisting_snapshot"
    ClientRuleSyncStateSucceeded          ClientRuleSyncState = "succeeded"
    ClientRuleSyncStateFailed             ClientRuleSyncState = "failed"
)

type ClientRouteApplyState string

const (
    ClientRouteApplyStateCreated           ClientRouteApplyState = "created"
    ClientRouteApplyStateCompilingCore     ClientRouteApplyState = "compiling_core_rules"
    ClientRouteApplyStateCompilingFirewall ClientRouteApplyState = "compiling_firewall_rules"
    ClientRouteApplyStateCompilingDNS      ClientRouteApplyState = "compiling_dns_rules"
    ClientRouteApplyStateStagingFirewall   ClientRouteApplyState = "staging_firewall_set"
    ClientRouteApplyStateStagingDNS        ClientRouteApplyState = "staging_dns_set"
    ClientRouteApplyStateSwitchingActive   ClientRouteApplyState = "switching_active_set"
    ClientRouteApplyStateCleaningOld       ClientRouteApplyState = "cleaning_old_set"
    ClientRouteApplyStateSucceeded         ClientRouteApplyState = "succeeded"
    ClientRouteApplyStateFailed            ClientRouteApplyState = "failed"
    ClientRouteApplyStateRollingBack       ClientRouteApplyState = "rolling_back"
    ClientRouteApplyStateRolledBack        ClientRouteApplyState = "rolled_back"
)

type ClientRouteRuleSyncJob struct {
    ID              int64
    RouteProfileID  int64
    SourceID        int64
    Status          ClientRuleSyncState
    Phase           string
    ErrorCode       string
    ErrorMessage    string
    SnapshotVersion string
    StartedAt       *time.Time
    FinishedAt      *time.Time
}

type ClientRouteApplyJob struct {
    ID                int64
    RouteProfileID    int64
    Status            ClientRouteApplyState
    Phase             string
    FirewallBackend   string
    DNSBackend        string
    StagedFirewallSet string
    StagedDNSSet      string
    ActiveFirewallSet string
    ActiveDNSSet      string
    ErrorCode         string
    ErrorMessage      string
    StartedAt         *time.Time
    FinishedAt        *time.Time
}
```

要求：

- `DefaultTargetCore` 当前固定为 `sing-box`
- `SelectedCore` 允许用户进入页面后手动切换
- 切换核心不改变原始订阅源列表
- 切换核心或运行模式后，需要基于最新有效订阅快照重新生成新的配置产物
- 路由规则、区域绕过、运行模式配置独立保存，不跟原始订阅源强绑定
- 规则编辑器字段必须能完整表达规则名、类型、匹配条件、目标出口、优先级、启停与备注
- 规则来源必须同时支持远程同步与本地手动维护
- 规则动作必须区分核心内路由与核心外直连防火墙落地
- 域名直连规则必须支持 `dnsmasq -> 地址集 -> 防火墙` 这一条链路
- 远程地址集源必须支持标准 `CIDR` 文本列表，例如 `all_cn.txt` 与 `all_cn_ipv6.txt`
- 远程 `IPv4` / `IPv6` 地址集源必须分别同步并分别参与合并
- 远程地址集源同步失败时必须保留最后一次成功快照
- `IPv4` 与 `IPv6` 地址集源允许部分成功、部分失败

### 26.3 合并流程

建议流程：

1. 按优先级拉取所有已启用订阅源
2. 把各订阅源解析为统一领域模型
3. 根据去重规则识别重复节点
4. 根据冲突策略决定保留哪一份节点定义
5. 得到合并后的统一结果
6. 应用独立维护的分流规则、路由规则、运行模式参数
7. 生成新的目标核心可运行配置
8. 用户确认后再把某个生成版本应用为当前运行配置

规则合并与编译补充流程：

1. 读取所有已启用远程规则源的最近成功快照
2. 读取本地手动规则快照
3. 统一归一化为面板内部规则模型
4. 按优先级、精确度、动作进行冲突决议
5. 将核心内规则编译为目标核心可支持的规则规格
6. 将核心外直连规则编译为 `nftables` / 系统防火墙规则集合
7. 若存在域名直连规则，则同时生成 `dnsmasq` 地址集同步配置
8. 分别生成核心配置产物与防火墙配置产物

要求：

- 不直接拼接多个原始订阅文本
- 去重应优先基于统一模型关键字段，而不是只比对名称
- 原始订阅源可按定时策略独立同步
- 切换目标核心时不强制立即重新同步订阅，但必须重新生成新的目标配置
- 若用户选择“先同步再生成”，则先执行同步任务，再生成新的可运行配置
- 路由配置变更后，应允许直接重新生成，不强制重新同步订阅
- 生成成功不等于已经应用到当前运行配置
- 更精确前缀只在命中同一目标时优先于更大网段，例如 `192.168.2.3/32` 命中 `192.168.2.3` 时优先于 `192.168.0.0/16`，但 `192.168.2.4` 仍继续命中 `/16`
- 未进入核心的直连流量规则必须在防火墙层独立生效
- 远程地址集支持白名单和黑名单两种模式，并允许本地例外规则覆盖
- 输出后端由运行平台决定，可为 `iptables`、`ipset`、`nftables`，以及 `OpenWrt` 下由 `fw4` 管理的 `nftables`
- 规则决议必须稳定，不能因同优先级无序遍历导致生成结果漂移

### 26.4 服务接口建议

```go
type ClientSubscriptionMergeService interface {
    SyncSources(ctx context.Context, profileID int64) error
    Merge(ctx context.Context, profileID int64) (*MergedClientProfile, error)
    Generate(ctx context.Context, profileID int64, core ClientTargetCore, mode ClientRunMode) (*ClientGeneratedProfile, error)
    ApplyGeneratedProfile(ctx context.Context, profileID int64, generatedProfileID int64, revisionNo int) error
    RollbackAppliedProfile(ctx context.Context, profileID int64, generatedProfileID int64, revisionNo int) error
    CompareGeneratedProfiles(ctx context.Context, leftProfileID int64, leftRevision int, rightProfileID int64, rightRevision int) (*ClientProfileDiffResult, error)
}

type MergedClientProfile struct {
    ProfileID       int64
    SourceCount     int
    NodeCount       int
    DedupedCount    int
    SelectedCore    ClientTargetCore
    Nodes           []ProxyNode
}

type ClientRuleCompiler interface {
    LoadRemoteSnapshots(ctx context.Context, routeProfileID int64) ([]ClientRouteRuleSnapshot, error)
    LoadLocalRules(ctx context.Context, routeProfileID int64) ([]ClientRouteRule, error)
    MergeRules(ctx context.Context, routeProfileID int64) ([]ClientRouteRule, error)
    CompileForCore(ctx context.Context, core ClientTargetCore, rules []ClientRouteRule) ([]byte, error)
    CompileForFirewall(ctx context.Context, rules []ClientRouteRule) ([]byte, error)
    CompileDNSMasq(ctx context.Context, rules []ClientRouteRule) ([]byte, error)
    PreviewConflicts(ctx context.Context, core ClientTargetCore, rules []ClientRouteRule) (*ClientRulePreviewResult, error)
    CompileAddressSet(ctx context.Context, backend ClientRouteOutputBackend, rules []ClientRouteRule) ([]byte, error)
    ResolvePriority(ctx context.Context, rules []ClientRouteRule) ([]ClientRouteRule, error)
}

type FirewallRuleApplier interface {
    StageRuleSet(ctx context.Context, content []byte) (string, error)
    DeletePreviousRuleSet(ctx context.Context, activeSet string) error
    ActivateRuleSet(ctx context.Context, stagedSet string) error
    RollbackRuleSet(ctx context.Context, previousSet string) error
}

type DNSMasqRuleApplier interface {
    StageConfig(ctx context.Context, content []byte) (string, error)
    ActivateConfig(ctx context.Context, stagedConfig string) error
    CleanupPreviousConfig(ctx context.Context, activeConfig string) error
    RollbackConfig(ctx context.Context, previousConfig string) error
}

type RouteRuntimeCoordinator interface {
    StagePlatformRules(ctx context.Context, routeProfileID int64) (string, error)
    ActivatePlatformRules(ctx context.Context, stagedRules string) error
    StageResolverConfig(ctx context.Context, routeProfileID int64) (string, error)
    ActivateResolverConfig(ctx context.Context, stagedConfig string) error
    VerifyRuntime(ctx context.Context, routeProfileID int64) error
    RollbackPlatformRules(ctx context.Context, previousRules string) error
    RollbackResolverConfig(ctx context.Context, previousConfig string) error
}

type OpenWRTRouteRuntimeCoordinator interface {
    StageDNSMasq(ctx context.Context, routeProfileID int64) (string, error)
    StageFW4(ctx context.Context, routeProfileID int64) (string, error)
    ActivateDNSMasq(ctx context.Context, stagedConfig string) error
    ActivateFW4(ctx context.Context, stagedRules string) error
    VerifyRuntime(ctx context.Context, routeProfileID int64) error
    RollbackDNSMasq(ctx context.Context, previousConfig string) error
    RollbackFW4(ctx context.Context, previousRules string) error
}

平台后端要求：

- 路由运行时必须先绑定通用平台后端接口，再选择具体实现
- 当前建议支持的实现族：
  - Linux `nftables`
  - Linux `iptables` / `ipset`
  - OpenWrt `fw4`
  - macOS `pf`
  - Windows 系统防火墙 / `WFP`

OpenWrt `dnsmasq` / `fw4` 规则生成要求：

- `dns_direct` 规则在 OpenWrt 下优先编译为 `nftset=/domain/...` 片段
- `IPv4` 与 `IPv6` 必须分别映射到不同的 `nft set`
- 输出格式应兼容 `fw4` 表结构，例如：

```text
nftset=/services.googleapis.cn/4#inet#fw4#china_ip_route_pass
nftset=/xn--ngstr-lra8j.com/4#inet#fw4#china_ip_route_pass,6#inet#fw4#china_ip6_route_pass
```

- 若目标域名仅要求 `IPv4` 直连，则只输出 `4#...`
- 若目标域名同时要求 `IPv4` 与 `IPv6` 直连，则同一行同时输出 `4#...` 与 `6#...`
- 对导入的 `dnsmasq` 片段必须先做语法校验，再归一化入库
- 类似 `/6#inet#fw4#china_ip6_route_pass` 这类缺失 `nftset=` 与域名匹配段的异常内容，必须判定为非法片段

跨平台能力模型建议：

```go
type RuntimePlatform string

const (
    RuntimePlatformLinux   RuntimePlatform = "linux"
    RuntimePlatformOpenWRT RuntimePlatform = "openwrt"
    RuntimePlatformMacOS   RuntimePlatform = "macos"
    RuntimePlatformWindows RuntimePlatform = "windows"
)

type CapabilitySupportLevel string

const (
    CapabilitySupportFull      CapabilitySupportLevel = "full"
    CapabilitySupportPartial   CapabilitySupportLevel = "partial"
    CapabilitySupportDegraded  CapabilitySupportLevel = "degraded"
    CapabilitySupportUnsupported CapabilitySupportLevel = "unsupported"
)

type DomainDirectSupportMode string

const (
    DomainDirectSupportNative      DomainDirectSupportMode = "native"
    DomainDirectSupportResolverAux DomainDirectSupportMode = "resolver_aux"
    DomainDirectSupportCoreOnly    DomainDirectSupportMode = "core_only"
    DomainDirectSupportNone        DomainDirectSupportMode = "none"
)

type FirewallBackendKind string

const (
    FirewallBackendNFT     FirewallBackendKind = "nftables"
    FirewallBackendIPTables FirewallBackendKind = "iptables"
    FirewallBackendIPSet   FirewallBackendKind = "ipset"
    FirewallBackendFW4     FirewallBackendKind = "fw4"
    FirewallBackendPF      FirewallBackendKind = "pf"
    FirewallBackendWFP     FirewallBackendKind = "wfp"
)

type ResolverBackendKind string

const (
    ResolverBackendDNSMasq        ResolverBackendKind = "dnsmasq"
    ResolverBackendSystemResolver ResolverBackendKind = "system_resolver"
    ResolverBackendLocalDNSProxy  ResolverBackendKind = "local_dns_proxy"
)

type PlatformRouteCapabilities struct {
    Platform               RuntimePlatform
    DefaultFirewallBackend FirewallBackendKind
    AvailableFirewalls     []FirewallBackendKind
    AvailableResolvers     []ResolverBackendKind
    TUNSupport             CapabilitySupportLevel
    FirewallBypassSupport  CapabilitySupportLevel
    DomainDirectSupport    CapabilitySupportLevel
    DomainDirectMode       DomainDirectSupportMode
    AtomicSwitchSupport    CapabilitySupportLevel
    RollbackSupport        CapabilitySupportLevel
    IPv6DualStackSupport   CapabilitySupportLevel
    Notes                  []string
}
```

能力探测与选择接口：

```go
type PlatformCapabilityDetector interface {
    Detect(ctx context.Context) (*PlatformRouteCapabilities, error)
}

type RouteBackendSelector interface {
    SelectFirewallBackend(ctx context.Context, caps PlatformRouteCapabilities, preferred FirewallBackendKind) (FirewallBackendKind, error)
    SelectResolverBackend(ctx context.Context, caps PlatformRouteCapabilities, requireDomainDirect bool) (ResolverBackendKind, error)
}
```

平台边界要求：

- 不允许把“域名直连是否能完整进入地址集”混同为“核心内是否支持域名规则”
- 若平台只能支持核心内域名规则，则必须标记 `DomainDirectSupport = degraded` 且 `DomainDirectMode = core_only`
- 若平台不支持双版本地址集切换或稳定回滚，则 `AtomicSwitchSupport` 或 `RollbackSupport` 不得标记为 `full`
- 平台能力探测结果必须进入预览、编译和应用三个阶段，不能只在 UI 层提示
- `OpenWrt` 下的 `FirewallBackendFW4` 应视为“由 `fw4` 管理的 `nftables`”，而不是与 `FirewallBackendNFT` 平级的第二套底层引擎

推荐能力映射：

```go
var DefaultPlatformCapabilities = map[RuntimePlatform]PlatformRouteCapabilities{
    RuntimePlatformLinux: {
        Platform:               RuntimePlatformLinux,
        DefaultFirewallBackend: FirewallBackendNFT,
        AvailableFirewalls:     []FirewallBackendKind{FirewallBackendNFT, FirewallBackendIPTables, FirewallBackendIPSet},
        AvailableResolvers:     []ResolverBackendKind{ResolverBackendDNSMasq, ResolverBackendSystemResolver},
        TUNSupport:             CapabilitySupportFull,
        FirewallBypassSupport:  CapabilitySupportFull,
        DomainDirectSupport:    CapabilitySupportFull,
        DomainDirectMode:       DomainDirectSupportNative,
        AtomicSwitchSupport:    CapabilitySupportFull,
        RollbackSupport:        CapabilitySupportFull,
        IPv6DualStackSupport:   CapabilitySupportFull,
    },
    RuntimePlatformOpenWRT: {
        Platform:               RuntimePlatformOpenWRT,
        DefaultFirewallBackend: FirewallBackendFW4,
        AvailableFirewalls:     []FirewallBackendKind{FirewallBackendFW4},
        AvailableResolvers:     []ResolverBackendKind{ResolverBackendDNSMasq},
        TUNSupport:             CapabilitySupportFull,
        FirewallBypassSupport:  CapabilitySupportFull,
        DomainDirectSupport:    CapabilitySupportFull,
        DomainDirectMode:       DomainDirectSupportNative,
        AtomicSwitchSupport:    CapabilitySupportFull,
        RollbackSupport:        CapabilitySupportFull,
        IPv6DualStackSupport:   CapabilitySupportFull,
    },
    RuntimePlatformMacOS: {
        Platform:               RuntimePlatformMacOS,
        DefaultFirewallBackend: FirewallBackendPF,
        AvailableFirewalls:     []FirewallBackendKind{FirewallBackendPF},
        AvailableResolvers:     []ResolverBackendKind{ResolverBackendSystemResolver},
        TUNSupport:             CapabilitySupportPartial,
        FirewallBypassSupport:  CapabilitySupportPartial,
        DomainDirectSupport:    CapabilitySupportDegraded,
        DomainDirectMode:       DomainDirectSupportResolverAux,
        AtomicSwitchSupport:    CapabilitySupportPartial,
        RollbackSupport:        CapabilitySupportPartial,
        IPv6DualStackSupport:   CapabilitySupportPartial,
    },
    RuntimePlatformWindows: {
        Platform:               RuntimePlatformWindows,
        DefaultFirewallBackend: FirewallBackendWFP,
        AvailableFirewalls:     []FirewallBackendKind{FirewallBackendWFP},
        AvailableResolvers:     []ResolverBackendKind{ResolverBackendLocalDNSProxy, ResolverBackendSystemResolver},
        TUNSupport:             CapabilitySupportPartial,
        FirewallBypassSupport:  CapabilitySupportPartial,
        DomainDirectSupport:    CapabilitySupportDegraded,
        DomainDirectMode:       DomainDirectSupportResolverAux,
        AtomicSwitchSupport:    CapabilitySupportPartial,
        RollbackSupport:        CapabilitySupportPartial,
        IPv6DualStackSupport:   CapabilitySupportPartial,
    },
}
```

平台编译结果建议：

```go
type RouteCompileResult struct {
    CoreRules              []byte
    FirewallRules          []byte
    ResolverRules          []byte
    Platform               RuntimePlatform
    FirewallBackend        FirewallBackendKind
    ResolverBackend        ResolverBackendKind
    DomainDirectSupport    CapabilitySupportLevel
    DegradationReasons     []string
    BlockingErrors         []string
}
```

降级决策要求：

- `Linux` / `OpenWrt` 下启用域名直连且可用 `dnsmasq` 时，优先输出平台外地址集链路
- `macOS` / `Windows` 下若缺少稳定解析写集能力，则域名直连规则应降级为“核心内规则”或“禁止应用”
- 黑白名单模式、规则来源优先级、CIDR 精确度优先级不得因降级而改变
- 若用户明确要求“完全不进核心”的域名直连，而当前平台只能 `core_only`，则必须进入阻断错误而不是自动降级

建议错误码：

```go
type RouteRuntimeErrorCode string

const (
    RouteErrPlatformUnsupported           RouteRuntimeErrorCode = "route_platform_unsupported"
    RouteErrFirewallBackendUnavailable    RouteRuntimeErrorCode = "route_firewall_backend_unavailable"
    RouteErrResolverBackendUnavailable    RouteRuntimeErrorCode = "route_resolver_backend_unavailable"
    RouteErrDomainDirectRequiresResolver  RouteRuntimeErrorCode = "route_domain_direct_requires_resolver"
    RouteErrDomainDirectCannotBypassCore  RouteRuntimeErrorCode = "route_domain_direct_cannot_bypass_core"
    RouteErrAtomicSwitchUnavailable       RouteRuntimeErrorCode = "route_atomic_switch_unavailable"
    RouteErrRollbackCapabilityInsufficient RouteRuntimeErrorCode = "route_rollback_capability_insufficient"
    RouteErrOpenWRTDNSMasqIncludeMissing  RouteRuntimeErrorCode = "openwrt_dnsmasq_include_missing"
    RouteErrOpenWRTDNSMasqIncludeConflict RouteRuntimeErrorCode = "openwrt_dnsmasq_include_conflict"
    RouteErrOpenWRTDNSMasqStageInvalid    RouteRuntimeErrorCode = "openwrt_dnsmasq_stage_invalid"
    RouteErrOpenWRTDNSMasqReloadFailed    RouteRuntimeErrorCode = "openwrt_dnsmasq_reload_failed"
    RouteErrOpenWRTDNSMasqNFTSetMissing   RouteRuntimeErrorCode = "openwrt_dnsmasq_nftset_missing"
    RouteErrOpenWRTDNSMasqVerifyTimeout   RouteRuntimeErrorCode = "openwrt_dnsmasq_verify_timeout"
    RouteErrOpenWRTFW4IncludeMissing      RouteRuntimeErrorCode = "openwrt_fw4_include_missing"
    RouteErrOpenWRTFW4IncludeConflict     RouteRuntimeErrorCode = "openwrt_fw4_include_conflict"
    RouteErrOpenWRTFW4StageInvalid        RouteRuntimeErrorCode = "openwrt_fw4_stage_invalid"
    RouteErrOpenWRTFW4ReloadFailed        RouteRuntimeErrorCode = "openwrt_fw4_reload_failed"
    RouteErrOpenWRTFW4RuntimeMissing      RouteRuntimeErrorCode = "openwrt_fw4_runtime_missing"
    RouteErrOpenWRTFW4VerifyMismatch      RouteRuntimeErrorCode = "openwrt_fw4_verify_mismatch"
    RouteErrOpenWRTRollbackFailed         RouteRuntimeErrorCode = "openwrt_route_rollback_failed"
)
```

事件类型建议：

```go
type RouteApplyEventType string

const (
    RouteApplyEventJobCreated            RouteApplyEventType = "job_created"
    RouteApplyEventDryRunCompleted       RouteApplyEventType = "dry_run_completed"
    RouteApplyEventValidateStarted       RouteApplyEventType = "validate_started"
    RouteApplyEventValidateFailed        RouteApplyEventType = "validate_failed"
    RouteApplyEventStagedFilesWritten    RouteApplyEventType = "staged_files_written"
    RouteApplyEventDNSMasqReloadStarted  RouteApplyEventType = "dnsmasq_reload_started"
    RouteApplyEventDNSMasqReloadSucceeded RouteApplyEventType = "dnsmasq_reload_succeeded"
    RouteApplyEventDNSMasqReloadFailed   RouteApplyEventType = "dnsmasq_reload_failed"
    RouteApplyEventFW4ReloadStarted      RouteApplyEventType = "fw4_reload_started"
    RouteApplyEventFW4ReloadSucceeded    RouteApplyEventType = "fw4_reload_succeeded"
    RouteApplyEventFW4ReloadFailed       RouteApplyEventType = "fw4_reload_failed"
    RouteApplyEventRuntimeVerifyStarted  RouteApplyEventType = "runtime_verify_started"
    RouteApplyEventRuntimeVerifySucceeded RouteApplyEventType = "runtime_verify_succeeded"
    RouteApplyEventRuntimeVerifyFailed   RouteApplyEventType = "runtime_verify_failed"
    RouteApplyEventRollbackStarted       RouteApplyEventType = "rollback_started"
    RouteApplyEventRollbackSucceeded     RouteApplyEventType = "rollback_succeeded"
    RouteApplyEventRollbackFailed        RouteApplyEventType = "rollback_failed"
    RouteApplyEventNeedsIntervention     RouteApplyEventType = "needs_intervention"
)
```

OpenWrt include 接入模型建议：

```go
type OpenWRTManagedInclude struct {
    Name         string
    Owner        string
    Type         string
    Path         string
    Enabled      bool
    Source       string
    IsManaged    bool
    LastCheckedAt *time.Time
}

type OpenWRTSystemIntegrationState struct {
    DNSMasqInclude OpenWRTManagedInclude
    FW4Include     OpenWRTManagedInclude
    ConflictReason string
}
```

OpenWrt include 初始化与修复 DTO 建议：

```go
type OpenWRTInitializeIncludesRequest struct {
    RouteProfileID         int64  `json:"route_profile_id"`
    DNSMasqIncludeName     string `json:"dnsmasq_include_name"`
    DNSMasqIncludePath     string `json:"dnsmasq_include_path"`
    FW4IncludeName         string `json:"fw4_include_name"`
    FW4IncludePath         string `json:"fw4_include_path"`
    ConfirmWriteSystemUCI  bool   `json:"confirm_write_system_uci"`
}

type OpenWRTRepairIncludesRequest struct {
    RouteProfileID         int64  `json:"route_profile_id"`
    RepairDNSMasqInclude   bool   `json:"repair_dnsmasq_include"`
    RepairFW4Include       bool   `json:"repair_fw4_include"`
    RebuildActivePointers  bool   `json:"rebuild_active_pointers"`
    Reason                 string `json:"reason"`
}

type OpenWRTIntegrationStatusResponse struct {
    RouteProfileID           int64                   `json:"route_profile_id"`
    Initialized              bool                    `json:"initialized"`
    DNSMasqInclude           OpenWRTManagedInclude   `json:"dnsmasq_include"`
    FW4Include               OpenWRTManagedInclude   `json:"fw4_include"`
    CurrentDNSMasqPointer    string                  `json:"current_dnsmasq_pointer"`
    CurrentFW4Pointer        string                  `json:"current_fw4_pointer"`
    RepairActions            []string                `json:"repair_actions"`
    BlockingIssues           []string                `json:"blocking_issues"`
}
```

type ClientProfileDiffResult struct {
    LeftGeneratedProfileID  int64
    LeftRevisionNo          int
    RightGeneratedProfileID int64
    RightRevisionNo         int
    Summary                 []string
    RouteRuleDiffs          []string
    ConfigTextDiff          string
}

type ClientRulePreviewConflict struct {
    ConflictType string
    Summary      string
    Severity     string
}

type ClientRulePreviewWarning struct {
    WarningType string
    Summary     string
}

type ClientRulePreviewResult struct {
    RouteProfileID     int64
    TargetCore         ClientTargetCore
    CoreRuleCount      int
    FirewallRuleCount  int
    DNSRuleCount       int
    Conflicts          []ClientRulePreviewConflict
    Warnings           []ClientRulePreviewWarning
    MatchOrderPreview  []string
    InvalidFragments   []string
}
```

远程地址集同步重试建议：

- 默认同步间隔 6 小时
- 失败后指数退避重试 3 次
- 建议退避窗口为 30 秒、120 秒、300 秒
- 超过最大重试次数后，任务记为失败，但继续使用最后成功快照
- `IPv4` 与 `IPv6` 源分别统计失败次数与最后成功时间

源文件校验建议：

- `remote_cidr_v4` 仅允许 `IPv4` 或 `IPv4 CIDR`
- `remote_cidr_v6` 仅允许 `IPv6` 或 `IPv6 CIDR`
- 允许空行和注释行
- 单 IP 输入应自动归一化为 `/32` 或 `/128`
- 解析器必须输出有效行数、非法行数、非法片段样本
- 非法行占比超过 5% 时，整次同步任务失败
- 解析结果无有效地址时，整次同步任务失败
- 同步失败时不得覆盖最后成功快照

增量同步建议：

- 规则源应持久化 `LastETag`、`LastModified`、`LastCheckedAt`
- 同步器优先发送 `If-None-Match`
- 无 `ETag` 时发送 `If-Modified-Since`
- 对 `304 Not Modified` 响应只更新时间戳，不创建新快照
- 手动强制同步时可跳过条件头，直接执行完整拉取

远程地址集同步 DTO 建议：

```go
type SyncClientRouteRuleSourceRequest struct {
    SourceID      int64 `json:"source_id"`
    ForceFullSync bool  `json:"force_full_sync"`
}

type ClientRouteRuleSourceStatusResponse struct {
    SourceID                 int64      `json:"source_id"`
    SourceURL                string     `json:"source_url"`
    LastSyncAt               *time.Time `json:"last_sync_at"`
    LastCheckedAt            *time.Time `json:"last_checked_at"`
    LastSyncStatus           string     `json:"last_sync_status"`
    LastSyncResult           string     `json:"last_sync_result"`
    LastHTTPStatus           int        `json:"last_http_status"`
    LastETag                 string     `json:"last_etag"`
    LastModified             string     `json:"last_modified"`
    LastNotModified          bool       `json:"last_not_modified"`
    ConsecutiveFailureCount  int        `json:"consecutive_failure_count"`
    UsingSnapshotVersion     string     `json:"using_snapshot_version"`
}
```

优先级决议建议：

1. 过滤已禁用规则
2. 按来源层级排序：`local_exception > local_normal > remote > builtin`
3. 按前缀精确度排序：更长前缀优先
4. 按显式优先级排序：数值越小越优先
5. 按动作排序：`block > proxy > direct`
6. 按创建时间和主键做稳定排序

术语约束建议：

- 使用“精确匹配优先”表示更长前缀只在命中同一目标时优先
- 使用“同目标命中时优先”表示只有当前目标同时命中多条规则时才比较精确度
- 使用“未命中精确例外则回退到大网段规则”表示基础网段规则依然保留
- `192.168.2.3/32 -> proxy` 与 `192.168.0.0/16 -> direct` 的组合中，`192.168.2.4` 仍应继续命中 `/16`

命中决议补充：

- `dns_direct` 作为防火墙直连语义规则时，应在预览中单独标记
- 当域名直连与地址集直连同时命中时，优先保留显式域名规则
- 当本地 `/32` 或 `/128` 例外与远程大网段冲突时，优先保留更精确前缀
- 决议结果必须可输出为逐条命中顺序列表，供前端展示

运行时文件布局建议：

- `data/runtime/routes/ipset/*.conf`
- `data/runtime/routes/nftables/*.nft`
- `data/runtime/routes/fw4/*.nft`
- `data/runtime/routes/pf/*.conf`
- `data/runtime/routes/wfp/*.json`
- `data/runtime/routes/dnsmasq/*.conf`
- `data/runtime/routes/meta/*.json`
- `data/runtime/routes/snapshots/*.json`

OpenWrt 运行时文件布局建议：

- `data/runtime/routes/dnsmasq/<profile_key>/active.conf`
- `data/runtime/routes/dnsmasq/<profile_key>/staged/job_<id>.conf`
- `data/runtime/routes/fw4/<profile_key>/active_sets.nft`
- `data/runtime/routes/fw4/<profile_key>/active_rules.nft`
- `data/runtime/routes/fw4/<profile_key>/staged/job_<id>_sets.nft`
- `data/runtime/routes/fw4/<profile_key>/staged/job_<id>_rules.nft`
- `data/runtime/routes/meta/<profile_key>/active.json`
- `data/runtime/routes/meta/<profile_key>/staged_job_<id>.json`

`fw4` 片段命名模型建议：

```go
type FW4FragmentName struct {
    ProfileKey string
    Action     string
    Family     string
    Stage      string
    Revision   string
}

func (n FW4FragmentName) String() string {
    return fmt.Sprintf("%s_%s_%s_%s_%s", n.ProfileKey, n.Action, n.Family, n.Stage, n.Revision)
}
```

约束建议：

- `ProfileKey` 只允许 `[a-z0-9_]`
- `Action` 固定为 `direct`、`proxy`、`block`
- `Family` 固定为 `v4`、`v6`、`dual`
- `Stage` 固定为 `active`、`staged`
- `Revision` 固定为 `job_<id>` 或 `rev_<no>`

地址集命名模型建议：

```go
type AddressSetBackend string

const (
    AddressSetBackendIPSet AddressSetBackend = "ipset"
    AddressSetBackendNFT   AddressSetBackend = "nft"
    AddressSetBackendFW4   AddressSetBackend = "fw4"
)

type AddressSetName struct {
    ProfileKey string
    Scope      string
    Action     string
    Family     string
    Backend    AddressSetBackend
    Revision   string
}

func (n AddressSetName) BaseName() string {
    return fmt.Sprintf("%s_%s_%s_%s_%s", n.ProfileKey, n.Scope, n.Action, n.Family, n.Backend)
}

func (n AddressSetName) StagedName() string {
    if n.Revision == "" {
        return n.BaseName()
    }
    return fmt.Sprintf("%s_%s", n.BaseName(), n.Revision)
}
```

字段约束建议：

- `ProfileKey` 只允许 `[a-z0-9_]`
- `Scope` 建议固定为 `cidr`、`domain`、`manual`
- `Action` 固定为 `direct`、`proxy`、`block`
- `Family` 固定为 `v4`、`v6`
- `Backend` 固定为 `ipset`、`nft`、`fw4`
- `Revision` 仅在 staged 名称中出现，固定为 `job_<id>` 或 `rev_<no>`

示例：

- `AddressSetName{ProfileKey:"cf", Scope:"cidr", Action:"direct", Family:"v4", Backend:AddressSetBackendIPSet}.BaseName()` -> `cf_cidr_direct_v4_ipset`
- `AddressSetName{ProfileKey:"cf", Scope:"cidr", Action:"direct", Family:"v4", Backend:AddressSetBackendNFT, Revision:"job_1024"}.StagedName()` -> `cf_cidr_direct_v4_nft_job_1024`
- `AddressSetName{ProfileKey:"cf", Scope:"manual", Action:"proxy", Family:"v6", Backend:AddressSetBackendFW4, Revision:"rev_18"}.StagedName()` -> `cf_manual_proxy_v6_fw4_rev_18`

地址集名称生成器建议：

```go
type AddressSetNamePolicy struct {
    ProfileKey       string
    ScopeStrategy    string
    RevisionStrategy string
    Backend          AddressSetBackend
    FW4IncludeName   string
    DNSMasqIncludeName string
}

type AddressSetNameGenerator interface {
    GenerateBaseName(policy AddressSetNamePolicy, scope string, action string, family string) (string, error)
    GenerateStagedName(policy AddressSetNamePolicy, scope string, action string, family string, revisionNo int64, jobID int64) (string, error)
    GenerateFW4IncludeName(policy AddressSetNamePolicy) (string, error)
    ValidateName(name string, backend AddressSetBackend) error
}
```

要求：

- 生成器必须基于 `ClientRouteProfile` 中持久化的命名字段生成，不允许运行时随机拼接
- `RevisionStrategy=job_id` 时优先输出 `job_<id>`
- `RevisionStrategy=revision_no` 时优先输出 `rev_<no>`
- `ValidateName()` 必须按后端长度与字符集限制分别校验 `ipset`、`nft`、`fw4`

平台应用编排建议：

1. stage 平台规则
2. stage 域名解析相关配置
3. 激活平台规则
4. 激活域名解析相关配置
5. 校验运行时结果
6. 失败时先回滚平台规则，再回滚域名解析相关配置

OpenWrt 特化应用编排建议：

1. stage `dnsmasq` 配置
2. stage `fw4` 规则，底层仍落到 `nftables`
3. 激活 `dnsmasq`
4. 激活 `fw4`
5. 校验运行时结果
6. 失败时先回滚 `fw4`，再回滚 `dnsmasq`

OpenWrt `fw4 reload` 状态机建议：

```go
type OpenWRTFW4ApplyState string

const (
    OpenWRTFW4ApplyStateCreated                OpenWRTFW4ApplyState = "created"
    OpenWRTFW4ApplyStateWritingDNSMasqStage    OpenWRTFW4ApplyState = "writing_dnsmasq_stage"
    OpenWRTFW4ApplyStateWritingFW4Stage        OpenWRTFW4ApplyState = "writing_fw4_stage"
    OpenWRTFW4ApplyStateValidatingStage        OpenWRTFW4ApplyState = "validating_stage"
    OpenWRTFW4ApplyStateReloadingDNSMasq       OpenWRTFW4ApplyState = "reloading_dnsmasq"
    OpenWRTFW4ApplyStateVerifyingDNSMasqSets   OpenWRTFW4ApplyState = "verifying_dnsmasq_sets"
    OpenWRTFW4ApplyStateReloadingFW4           OpenWRTFW4ApplyState = "reloading_fw4"
    OpenWRTFW4ApplyStateVerifyingNFTables      OpenWRTFW4ApplyState = "verifying_nftables_runtime"
    OpenWRTFW4ApplyStateSucceeded              OpenWRTFW4ApplyState = "succeeded"
    OpenWRTFW4ApplyStateFailed                 OpenWRTFW4ApplyState = "failed"
    OpenWRTFW4ApplyStateRollingBackFW4         OpenWRTFW4ApplyState = "rolling_back_fw4"
    OpenWRTFW4ApplyStateRollingBackDNSMasq     OpenWRTFW4ApplyState = "rolling_back_dnsmasq"
    OpenWRTFW4ApplyStateRolledBack             OpenWRTFW4ApplyState = "rolled_back"
    OpenWRTFW4ApplyStateNeedsIntervention      OpenWRTFW4ApplyState = "needs_intervention"
)
```

OpenWrt `fw4 reload` 失败恢复规则：

- 若失败发生在 `reloading_dnsmasq` 前，可直接删除 staged 片段并结束
- 若失败发生在 `reloading_dnsmasq` 后但 `reloading_fw4` 前，先回滚 `dnsmasq`
- 若失败发生在 `reloading_fw4` 或之后，先回滚 `fw4`，再回滚 `dnsmasq`
- 若回滚后活动元数据与磁盘片段不一致，必须进入 `needs_intervention`

OpenWrt 路由运行时 DTO 建议：

```go
type OpenWRTRouteDryRunRequest struct {
    RouteProfileID      int64  `json:"route_profile_id"`
    ForceResolverProbe  bool   `json:"force_resolver_probe"`
    ForceRuntimeVerify  bool   `json:"force_runtime_verify"`
    Reason              string `json:"reason"`
    TriggerSource       string `json:"trigger_source"`
}

type OpenWRTRouteValidateRequest struct {
    RouteProfileID          int64 `json:"route_profile_id"`
    ValidateDNSMasqInclude  bool  `json:"validate_dnsmasq_include"`
    ValidateFW4Include      bool  `json:"validate_fw4_include"`
    ValidateStageFragments  bool  `json:"validate_stage_fragments"`
    ValidateRuntimePaths    bool  `json:"validate_runtime_paths"`
}

type OpenWRTRouteApplyRequest struct {
    RouteProfileID          int64  `json:"route_profile_id"`
    ExpectedActiveRevision  int    `json:"expected_active_revision"`
    ForceReplace            bool   `json:"force_replace"`
    ForceReloadDNSMasq      bool   `json:"force_reload_dnsmasq"`
    ForceReloadFW4          bool   `json:"force_reload_fw4"`
    Reason                  string `json:"reason"`
    TriggerSource           string `json:"trigger_source"`
}

type OpenWRTRouteRollbackRequest struct {
    RouteProfileID        int64  `json:"route_profile_id"`
    TargetRevisionNo      int    `json:"target_revision_no"`
    RollbackDNSMasq       bool   `json:"rollback_dnsmasq"`
    RollbackFW4           bool   `json:"rollback_fw4"`
    Reason                string `json:"reason"`
    TriggerSource         string `json:"trigger_source"`
}

type OpenWRTRouteJobResponse struct {
    JobID                  int64                 `json:"job_id"`
    OperationKind          string                `json:"operation_kind"`
    Status                 string                `json:"status"`
    Phase                  string                `json:"phase"`
    RouteProfileID         int64                 `json:"route_profile_id"`
    DNSMasqIncludePath     string                `json:"dnsmasq_include_path"`
    FW4IncludeName         string                `json:"fw4_include_name"`
    FW4IncludePath         string                `json:"fw4_include_path"`
    ActiveRevisionNo       int                   `json:"active_revision_no"`
    TargetRevisionNo       int                   `json:"target_revision_no"`
    ErrorCode              RouteRuntimeErrorCode `json:"error_code"`
    ErrorMessage           string                `json:"error_message"`
    ReloadDNSMasqExitCode  *int                  `json:"reload_dnsmasq_exit_code"`
    ReloadFW4ExitCode      *int                  `json:"reload_fw4_exit_code"`
    NeedsInterventionReason string               `json:"needs_intervention_reason"`
}
```

OpenWrt API 路由建议：

```go
func RegisterOpenWRTRouteRuntimeRoutes(rg *gin.RouterGroup, h *route.Handler) {
    openwrtRG := rg.Group("/route-runtime/openwrt")
    {
        openwrtRG.GET("/integration-status/:routeProfileID", h.GetOpenWRTIntegrationStatus)
        openwrtRG.POST("/initialize-includes", h.InitializeOpenWRTIncludes)
        openwrtRG.POST("/repair-includes", h.RepairOpenWRTIncludes)
        openwrtRG.POST("/dry-run", h.OpenWRTDryRun)
        openwrtRG.POST("/validate", h.OpenWRTValidate)
        openwrtRG.POST("/apply", h.OpenWRTApply)
        openwrtRG.POST("/rollback", h.OpenWRTRollback)
    }
}
```

要求：

- `dry-run` 只返回变更摘要、目标路径、include 接入检查与潜在风险，不写 active 指针
- `validate` 负责校验 include 入口、staged 片段和预期运行时路径，可单独执行
- `apply` 必须写入 `client_route_apply_jobs`，并记录 `reload_dnsmasq_exit_code` 与 `reload_fw4_exit_code`
- `rollback` 必须引用已有成功版本，不允许回滚到未完整应用成功的 revision

### 26.5 前端交互要求

```ts
export interface ClientConfigState {
  defaultCore: 'sing-box'
  selectedCore: 'sing-box' | 'xray-core' | 'mihomo'
  runMode: 'proxy' | 'tun'
  sources: ClientSubscriptionSource[]
  routeRules: ClientRouteRule[]
  ruleSources: ClientRouteRuleSource[]
  routeRuleSyncJobs: ClientRouteRuleSyncJob[]
  routeApplyJobs: ClientRouteApplyJob[]
  generatedPreview: string
}
```

交互要求：

- 页面首次进入时 `selectedCore` 默认等于 `defaultCore`
- 默认运行模式为 `proxy`
- 用户手动切换核心或运行模式后，必须重新生成对应目标配置
- 用户切换核心不清空已加载的订阅源与最近同步结果
- 订阅源排序变化后，需要重新计算合并结果并更新生成配置
- 生成配置与应用运行必须为两个独立动作
- 页面必须展示当前已应用运行版本与最近一次生成版本
- `proxy` 模式下不展示仅 `tun` 专属字段
- `tun` 模式下展示虚拟网卡、自动路由、路由栈与接管范围字段

### 26.6 API DTO 建议

```go
type CreateClientMergeProfileRequest struct {
    Name           string              `json:"name"`
    Platform       string              `json:"platform"`
    ConflictPolicy MergeConflictPolicy `json:"conflict_policy"`
    Deduplicate    bool                `json:"deduplicate"`
}

type UpdateClientRouteProfileRequest struct {
    ProxyMode   ClientProxyMode `json:"proxy_mode"`
    BypassMode  ClientBypassMode `json:"bypass_mode"`
    TunEnabled  bool             `json:"tun_enabled"`
    IPv6Enabled bool             `json:"ipv6_enabled"`
    QUICEnabled bool             `json:"quic_enabled"`
    DNSPolicy   string           `json:"dns_policy"`
    Rules       []ClientRouteRule `json:"rules"`
}

type SyncClientRouteRuleSourceRequest struct {
    SourceID int64 `json:"source_id"`
}

type PreviewClientRouteRulesRequest struct {
    TargetCore ClientTargetCore `json:"target_core"`
    RunMode    ClientRunMode    `json:"run_mode"`
}

type ApplyClientRouteRulesRequest struct {
    TargetCore   ClientTargetCore `json:"target_core"`
    RunMode      ClientRunMode    `json:"run_mode"`
    ForceReplace bool             `json:"force_replace"`
}

type AddClientSubscriptionSourceRequest struct {
    Name      string `json:"name"`
    SourceURL string `json:"source_url"`
    Priority  int    `json:"priority"`
}

type RenderMergedClientProfileRequest struct {
    TargetCore ClientTargetCore `json:"target_core"`
    RunMode    ClientRunMode    `json:"run_mode"`
}

type MergedClientProfileResponse struct {
    ProfileID    int64            `json:"profile_id"`
    TargetCore   ClientTargetCore `json:"target_core"`
    RunMode      ClientRunMode    `json:"run_mode"`
    SourceCount  int              `json:"source_count"`
    NodeCount    int              `json:"node_count"`
    DedupedCount int              `json:"deduped_count"`
    Preview      string           `json:"preview"`
}

type ApplyGeneratedProfileRequest struct {
    GeneratedProfileID int64 `json:"generated_profile_id"`
    RevisionNo         int   `json:"revision_no"`
}

type RollbackGeneratedProfileRequest struct {
    GeneratedProfileID int64 `json:"generated_profile_id"`
    RevisionNo         int   `json:"revision_no"`
    Reason             string `json:"reason"`
}
```

### 26.7 前端模块建议

建议目录：

```text
web/admin/src/
  api/client-config.ts
  store/client-config.ts
  views/client/
    ClientConfigPage.vue
  components/client/
    ClientCoreSwitch.vue
    ClientSubscriptionSourceList.vue
    ClientMergePolicyPanel.vue
    ClientRouteRulePanel.vue
    ClientRunModePanel.vue
    ClientConfigPreview.vue
    ClientRuntimeLogPanel.vue
    ClientVersionDiffDrawer.vue
    ClientRollbackDialog.vue
```

### 26.8 Store 设计建议

```ts
export interface ClientConfigState {
  defaultCore: 'sing-box'
  selectedCore: 'sing-box' | 'xray-core' | 'mihomo'
  runMode: 'proxy' | 'tun'
  sources: ClientSubscriptionSource[]
  routeRules: ClientRouteRule[]
  mergePolicy: 'keep_first' | 'keep_last'
  deduplicate: boolean
  sourceCount: number
  nodeCount: number
  dedupedCount: number
  generatedPreview: string
  activeGeneratedProfileId?: number
  activeRevisionNo?: number
  latestGeneratedProfileId?: number
  latestRevisionNo?: number
  generateState?: string
  applyState?: string
  runtimeLogs: ClientRuntimeLog[]
  loading: boolean
}
```

建议 action：

```ts
fetchMergeProfile(profileId: number)
addSource(payload)
updateSource(sourceId: number, payload)
syncSource(sourceId: number)
syncAllSources()
setSelectedCore(core: 'sing-box' | 'xray-core' | 'mihomo')
setRunMode(mode: 'proxy' | 'tun')
updateRouteProfile(payload)
syncRouteRuleSource(sourceId: number)
previewRouteRules(payload)
applyRouteRules(payload)
generateProfile()
applyGeneratedProfile(generatedProfileId: number, revisionNo: number)
compareGeneratedProfiles(leftGeneratedProfileId: number, leftRevisionNo: number, rightGeneratedProfileId: number, rightRevisionNo: number)
rollbackGeneratedProfile(generatedProfileId: number, revisionNo: number, reason: string)
fetchRuntimeLogs(filters)
```

要求：

- `setSelectedCore()` 不重新拉取原始订阅源，但必须触发新配置生成
- `setRunMode()` 必须触发新配置生成
- `updateRouteProfile()` 保存后必须允许直接重新生成配置
- `syncRouteRuleSource()` 仅刷新对应远程规则源快照与同步状态
- `previewRouteRules()` 必须返回冲突、警告和最终命中顺序预览
- `applyRouteRules()` 必须创建独立规则落地事务，并支持失败回滚
- `syncSource()` 只更新对应订阅源状态和最近同步结果
- `syncAllSources()` 完成后统一刷新合并统计和最新生成配置
- `applyGeneratedProfile()` 成功后更新当前运行版本指针
- `rollbackGeneratedProfile()` 必须创建新的应用任务，而不是直接修改运行版本指针
- `compareGeneratedProfiles()` 默认对比当前运行版本与最近一次生成版本

### 23.7.5 进程控制器补充接口

```go
type ProcessCapability struct {
    SupportsReload      bool
    SupportsHotReload   bool
    RequiresFullRestart bool
}

type ManagedProcessController interface {
    Start(ctx context.Context, target RuntimeTarget) error
    Stop(ctx context.Context, target RuntimeTarget) error
    Restart(ctx context.Context, target RuntimeTarget) error
    Reload(ctx context.Context, target RuntimeTarget) error
    HealthCheck(ctx context.Context, target RuntimeTarget) error
    Capability(ctx context.Context, target RuntimeTarget) (*ProcessCapability, error)
    GetStatus(ctx context.Context, target RuntimeTarget) (*ProcessStatus, error)
    GetCrashLogs(ctx context.Context, serviceID int64, limit int) ([]*CoreCrashLog, error)
    ClearCrashLogs(ctx context.Context, serviceID int64) error
}
```

### 10.1.3 核心进程守护器设计

进程守护器负责监控核心进程状态，异常退出后按策略自动重启。

```go
type ProcessGuardian struct {
    serviceID       int64
    controller      ManagedProcessController
    config          GuardianConfig
    crashCount      int           // 当前窗口内连续崩溃次数
    lastCrashAt     time.Time     // 最近一次崩溃时间
    windowStart     time.Time     // 当前重启窗口起始时间
    stableSince     time.Time     // 上次稳定运行的时间点
    mu              sync.Mutex
}

type GuardianConfig struct {
    MaxRestartAttempts    int           // 最大连续重启次数，默认 5
    RestartWindow         time.Duration // 重启窗口，默认 10 分钟
    StableThreshold       time.Duration // 稳定运行阈值，默认 5 分钟，超过后重置崩溃计数
    LongStableThreshold   time.Duration // 长稳定阈值，默认 1 小时，超过后重置重启窗口
    BaseBackoff           time.Duration // 基础退避时间，默认 5 秒
    MaxBackoff            time.Duration // 最大退避时间，默认 80 秒
    MaxStdoutLines        int           // 崩溃时收集的 stdout 最大行数，默认 200
    MaxStderrLines        int           // 崩溃时收集的 stderr 最大行数，默认 200
    CrashLogRetention     int           // 每个服务保留的崩溃日志条数，默认 50
}

type ProcessStatus struct {
    ServiceID        int64
    State            string    // "running", "crashed", "failed", "stopped", etc.
    PID              int
    UptimeSeconds    int64
    CrashCount       int       // 当前窗口内连续崩溃次数
    MaxRestartAttempts int
    NextRestartAt    *time.Time // 如果正在退避等待，下次重启时间
    LastCrashAt      *time.Time
    LastExitCode     int
    AutoRestartEnabled bool
}

type CoreCrashLog struct {
    ID              int64
    ServiceID       int64
    CoreType        string
    ExitCode        int
    Signal          string
    KilledByOOM     bool
    RuntimeSeconds  int64
    StdoutTail      string
    StderrTail      string
    RestartAttempt  int
    AutoRestarted   bool
    CrashAt         time.Time
    CreatedAt       time.Time
}
```

守护器工作流程：

```go
func (g *ProcessGuardian) Run(ctx context.Context) {
    for {
        err := g.controller.Start(ctx, g.target)
        if err != nil {
            // 启动失败，记录并等待
            g.recordCrash(ctx, 0, "", false, 0)
            if g.shouldStopRestarting() {
                g.enterFailedState(ctx)
                return
            }
            g.waitBackoff(ctx)
            continue
        }

        // 等待进程退出
        exitInfo := g.waitForExit(ctx)

        if exitInfo.Expected {
            // 人为停止或配置变更重启，不计入崩溃
            g.resetCrashCount()
            return
        }

        // 非预期退出，记录崩溃
        g.recordCrash(ctx, exitInfo.ExitCode, exitInfo.Signal, exitInfo.OOM, exitInfo.RuntimeSeconds)

        // 检查是否应停止重启
        if g.shouldStopRestarting() {
            g.enterFailedState(ctx)
            return
        }

        // 退避等待后重启
        g.waitBackoff(ctx)
    }
}

func (g *ProcessGuardian) shouldStopRestarting() bool {
    g.mu.Lock()
    defer g.mu.Unlock()

    // 检查是否在窗口内超过最大重启次数
    if time.Since(g.windowStart) > g.config.RestartWindow {
        // 窗口已过期，重置
        g.crashCount = 0
        g.windowStart = time.Now()
    }
    return g.crashCount >= g.config.MaxRestartAttempts
}

func (g *ProcessGuardian) waitBackoff(ctx context.Context) {
    g.mu.Lock()
    attempt := g.crashCount
    g.mu.Unlock()

    // 指数退避：5s, 10s, 20s, 40s, 80s
    delay := g.config.BaseBackoff * time.Duration(1<<uint(attempt-1))
    if delay > g.config.MaxBackoff {
        delay = g.config.MaxBackoff
    }
    time.Sleep(delay)
}
```

进程稳定运行检测：

```go
func (g *ProcessGuardian) OnProcessStarted() {
    g.mu.Lock()
    defer g.mu.Unlock()
    g.stableSince = time.Now()
}

// 定时检查进程是否稳定运行
func (g *ProcessGuardian) checkStability() {
    g.mu.Lock()
    defer g.mu.Unlock()

    if !g.stableSince.IsZero() && time.Since(g.stableSince) > g.config.StableThreshold {
        // 稳定运行超过阈值，重置崩溃计数
        g.crashCount = 0
    }
    if !g.stableSince.IsZero() && time.Since(g.stableSince) > g.config.LongStableThreshold {
        // 长时间稳定运行，重置重启窗口
        g.windowStart = time.Now()
    }
}
```

OOM 检测：

```go
func detectOOM(pid int) bool {
    // Linux: 检查 /proc/<pid>/oom_score 或 dmesg
    // 返回 true 如果进程被 OOM Killer 杀死
}
```

数据库表：

```sql
CREATE TABLE core_crash_logs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  service_id INTEGER NOT NULL,
  core_type TEXT NOT NULL,
  exit_code INTEGER,
  signal TEXT,
  killed_by_oom INTEGER NOT NULL DEFAULT 0,
  runtime_seconds INTEGER NOT NULL DEFAULT 0,
  stdout_tail TEXT,
  stderr_tail TEXT,
  restart_attempt INTEGER NOT NULL DEFAULT 0,
  auto_restarted INTEGER NOT NULL DEFAULT 0,
  crash_at DATETIME NOT NULL,
  created_at DATETIME NOT NULL
);

CREATE INDEX idx_core_crash_logs_service_crash ON core_crash_logs(service_id, crash_at DESC);
```

API：

```go
// 查询崩溃日志
GET /api/v1/services/:id/crash-logs?limit=20&offset=0

// 响应
{
  "items": [
    {
      "id": 1,
      "service_id": 101,
      "core_type": "xray",
      "exit_code": 137,
      "signal": "SIGKILL",
      "killed_by_oom": true,
      "runtime_seconds": 120,
      "stdout_tail": "...",
      "stderr_tail": "killed",
      "restart_attempt": 3,
      "auto_restarted": true,
      "crash_at": "2026-05-27T10:30:00Z"
    }
  ],
  "total": 15
}

// 查询进程守护状态
GET /api/v1/services/:id/process-status

// 响应
{
  "service_id": 101,
  "state": "running",
  "pid": 12345,
  "uptime_seconds": 3600,
  "crash_count": 0,
  "max_restart_attempts": 5,
  "auto_restart_enabled": true,
  "last_crash_at": "2026-05-27T09:00:00Z",
  "last_exit_code": 0
}

// 手动重置重启计数并启动（用于 failed 状态恢复）
POST /api/v1/services/:id/start
{
  "reset_crash_count": true
}
```

### 23.7.6 运行时事件类型建议

建议事件类型常量：

```go
type RuntimeApplyEventType string

const (
    RuntimeEventSnapshotCreated   RuntimeApplyEventType = "snapshot_created"
    RuntimeEventActiveBackupCreated RuntimeApplyEventType = "active_backup_created"
    RuntimeEventOriginalSnapshotLoaded RuntimeApplyEventType = "original_snapshot_loaded"
    RuntimeEventRenderSucceeded   RuntimeApplyEventType = "render_succeeded"
    RuntimeEventValidateFailed    RuntimeApplyEventType = "validate_failed"
    RuntimeEventCertificateBound  RuntimeApplyEventType = "certificate_bound"
    RuntimeEventProxyReloadFailed RuntimeApplyEventType = "proxy_reload_failed"
    RuntimeEventCoreStarted       RuntimeApplyEventType = "core_started"
    RuntimeEventHealthCheckFailed RuntimeApplyEventType = "health_check_failed"
    RuntimeEventDisableProxyStarted RuntimeApplyEventType = "disable_proxy_started"
    RuntimeEventCoreOriginalRestoreStarted RuntimeApplyEventType = "core_original_restore_started"
    RuntimeEventCoreOriginalRestoreSucceeded RuntimeApplyEventType = "core_original_restore_succeeded"
    RuntimeEventReverseProxyRestoreStarted RuntimeApplyEventType = "reverse_proxy_restore_started"
    RuntimeEventReverseProxyRestoreSucceeded RuntimeApplyEventType = "reverse_proxy_restore_succeeded"
    RuntimeEventDNSMasqRestoreStarted RuntimeApplyEventType = "dnsmasq_restore_started"
    RuntimeEventDNSMasqRestoreSucceeded RuntimeApplyEventType = "dnsmasq_restore_succeeded"
    RuntimeEventFW4RestoreStarted RuntimeApplyEventType = "fw4_restore_started"
    RuntimeEventFW4RestoreSucceeded RuntimeApplyEventType = "fw4_restore_succeeded"
    RuntimeEventSystemProxyRestoreStarted RuntimeApplyEventType = "system_proxy_restore_started"
    RuntimeEventSystemProxyRestoreSucceeded RuntimeApplyEventType = "system_proxy_restore_succeeded"
    RuntimeEventComponentReloadStarted RuntimeApplyEventType = "component_reload_started"
    RuntimeEventComponentReloadSucceeded RuntimeApplyEventType = "component_reload_succeeded"
    RuntimeEventDisableProxyVerifyStarted RuntimeApplyEventType = "disable_proxy_verify_started"
    RuntimeEventDisableProxyVerifySucceeded RuntimeApplyEventType = "disable_proxy_verify_succeeded"
    RuntimeEventDisableProxyVerifyFailed RuntimeApplyEventType = "disable_proxy_verify_failed"
    RuntimeEventFallbackBackupRestoreStarted RuntimeApplyEventType = "fallback_backup_restore_started"
    RuntimeEventFallbackBackupRestoreSucceeded RuntimeApplyEventType = "fallback_backup_restore_succeeded"
    RuntimeEventFallbackBackupRestoreFailed RuntimeApplyEventType = "fallback_backup_restore_failed"
    RuntimeEventRollbackStarted   RuntimeApplyEventType = "rollback_started"
    RuntimeEventRollbackSucceeded RuntimeApplyEventType = "rollback_succeeded"
)
```

建议：

- “关闭代理恢复原配置”至少应记录五类对象级事件：核心、反向代理、`dnsmasq`、`fw4`、系统代理
- 当某一对象不需要恢复时，也应记录跳过原因，避免 UI 误判为漏执行
- `fallback_backup_restore_*` 事件只在关闭代理失败后的回切场景出现

### 23.8 `Caddy` 与 `nginx` 的差异处理

建议：

- `Caddy`：优先热重载
- `nginx`：优先配置校验后 `reload`
- 若 `reload` 失败，不保留半更新状态

### 23.9 运行时回滚顺序

建议固定按相反顺序回滚：

1. 回滚核心配置与进程版本
2. 回滚反向代理配置
3. 回滚证书引用
4. 恢复版本指针与健康状态缓存

原因：

- 先恢复核心可减少入口恢复后后端仍异常的窗口
- 版本指针最后恢复可避免 UI 与真实运行状态不一致

### 23.10 定时任务设计

定时任务应统一在面板调度，不依赖各组件私有任务。

建议任务：

- 证书续期扫描
- 到期预警
- 部署失败重试
- 动态应用失败重试
- 反向代理配置漂移检查

### 23.11 审计与事件

建议记录：

- ACME 证书申请开始/成功/失败
- 证书部署开始/成功/失败
- `Caddy` 热重载结果
- `nginx reload` 结果
- 核心证书动态加载结果
- 回滚结果

## 27. 服务端节点分享与订阅分发设计

边界说明：

- 本章只描述服务端“生成订阅给别人导入”的能力
- 本章不描述客户端导入外部订阅、多订阅合并和客户端运行
- 服务端节点分享需要独立档案、独立渲染版本和独立发布动作

### 27.1 目标

服务端节点分享需要支持：

- 按目标客户端类型生成不同订阅输出
- 按节点勾选生成仅含已选节点的内容
- 把远程规则 URL 与自定义规则注入目标订阅模板
- 支持单节点 URI、二维码、`HTTPS` 订阅链接三类交付
- 对不兼容的规则表达返回结构化降级说明

### 27.2 领域模型

```go
type ServerShareClientType string

const (
    ServerShareClientTypeSingBox    ServerShareClientType = "sing-box"
    ServerShareClientTypeXray       ServerShareClientType = "xray-core"
    ServerShareClientTypeMihomo     ServerShareClientType = "mihomo"
    ServerShareClientTypeUniversal  ServerShareClientType = "universal_uri"
)

type ServerShareOutputFormat string

const (
    ServerShareOutputSingBoxJSON   ServerShareOutputFormat = "singbox_json"
    ServerShareOutputXrayJSON      ServerShareOutputFormat = "xray_json"
    ServerShareOutputMihomoYAML    ServerShareOutputFormat = "mihomo_yaml"
    ServerShareOutputRawURI        ServerShareOutputFormat = "raw_uri"
    ServerShareOutputBase64URIBlob ServerShareOutputFormat = "base64_uri_bundle"
)

type ServerShareDeliveryMode string

const (
    ServerShareDeliverySingleURI         ServerShareDeliveryMode = "single_uri"
    ServerShareDeliverySingleURIQR       ServerShareDeliveryMode = "single_uri_qr"
    ServerShareDeliveryHTTPSSubscription ServerShareDeliveryMode = "https_subscription"
    ServerShareDeliveryHTTPSQR           ServerShareDeliveryMode = "https_subscription_qr"
    ServerShareDeliveryFileDownload      ServerShareDeliveryMode = "file_download"
)

type ServerShareRuleSourceType string

const (
    ServerShareRuleSourceRemoteURL  ServerShareRuleSourceType = "remote_url"
    ServerShareRuleSourceCustomRule ServerShareRuleSourceType = "custom_inline"
)

type ServerShareProfileStatus string

const (
    ServerShareProfileDraft        ServerShareProfileStatus = "draft"
    ServerShareProfilePreviewReady ServerShareProfileStatus = "preview_ready"
    ServerShareProfilePublished    ServerShareProfileStatus = "published"
    ServerShareProfileRevoked      ServerShareProfileStatus = "revoked"
    ServerShareProfileDisabled     ServerShareProfileStatus = "disabled"
)

type ServerShareJobStatus string

const (
    ServerShareJobCreated           ServerShareJobStatus = "created"
    ServerShareJobPreparingSources  ServerShareJobStatus = "preparing_sources"
    ServerShareJobRendering         ServerShareJobStatus = "rendering"
    ServerShareJobIssuingToken      ServerShareJobStatus = "issuing_token"
    ServerShareJobPersistingRevision ServerShareJobStatus = "persisting_revision"
    ServerShareJobSucceeded         ServerShareJobStatus = "succeeded"
    ServerShareJobFailed            ServerShareJobStatus = "failed"
    ServerShareJobNeedsIntervention ServerShareJobStatus = "needs_intervention"
)

type ServerShareJobType string

const (
    ServerShareJobPreview      ServerShareJobType = "preview"
    ServerShareJobPublish      ServerShareJobType = "publish"
    ServerShareJobExportQR     ServerShareJobType = "export_qrcode"
    ServerShareJobExportFile   ServerShareJobType = "export_file"
)

type ServerShareDegradeCode string

const (
    ServerShareDegradeRuleURLIgnored    ServerShareDegradeCode = "rule_url_ignored"
    ServerShareDegradeCustomRuleIgnored ServerShareDegradeCode = "custom_rule_ignored"
    ServerShareDegradeFormatFallback    ServerShareDegradeCode = "format_fallback"
    ServerShareDegradeNodeOnly          ServerShareDegradeCode = "node_only_output"
)

type ServerShareProfile struct {
    ID                 int64
    Name               string
    ClientID           *int64
    TargetClientType   ServerShareClientType
    OutputFormat       ServerShareOutputFormat
    DeliveryMode       ServerShareDeliveryMode
    TemplateID         *int64
    RouteTemplateID    *int64
    SecurityMode       string
    BindMode           string
    ExpiresAt          *time.Time
    HTTPSRequired      bool
    IncludeRuleURLs    bool
    IncludeCustomRules bool
    Status             ServerShareProfileStatus
    Nodes              []ServerShareProfileNode
    RuleSources        []ServerShareRuleSource
    LastPreviewAt      *time.Time
    LastPublishedAt    *time.Time
}

type ServerShareProfileNode struct {
    ID        int64
    ProfileID int64
    NodeID    int64
    Order     int
    Enabled   bool
    IsPrimary bool
    Remark    string
}

type ServerShareRuleSource struct {
    ID                  int64
    ProfileID           int64
    SourceType          ServerShareRuleSourceType
    Name                string
    SourceURL           string
    CustomContent       string
    RuleFormat          string
    Priority            int
    Enabled             bool
    SyncIntervalSeconds int
    LastSyncAt          *time.Time
    LastSyncStatus      string
    LastErrorMessage    string
    LastETag            string
    LastModified        string
}

type ServerShareRenderRevision struct {
    ID                    int64
    ProfileID             int64
    SubscriptionID        *int64
    RenderVersion         string
    TargetClientType      ServerShareClientType
    OutputFormat          ServerShareOutputFormat
    DeliveryMode          ServerShareDeliveryMode
    MIMEType              string
    FileName              string
    NodeCount             int
    IncludedRuleURLCount  int
    IncludedCustomRuleCount int
    DegradeItems          []ServerShareDegradeItem
    RenderHash            string
    QRPayload             string
    ContentPreview        string
    CreatedAt             time.Time
}

type ServerShareDegradeItem struct {
    Code        ServerShareDegradeCode
    Message     string
    SourceType  string
    SourceName  string
    TargetField string
}

type ServerSharePublishJob struct {
    ID                  int64
    ProfileID           int64
    RenderRevisionID    *int64
    SubscriptionID      *int64
    JobType             ServerShareJobType
    Status              ServerShareJobStatus
    Phase               string
    ErrorCode           string
    ErrorMessage        string
    CanRetry            bool
    CanReuseLastRender  bool
    RequiresAction      bool
    RequiresActionReason string
    StartedAt           *time.Time
    FinishedAt          *time.Time
    CreatedAt           time.Time
}

type ServerShareAuditEvent struct {
    ID             int64
    ProfileID      int64
    PublishJobID   *int64
    SubscriptionID *int64
    EventType      string
    OperatorID     *int64
    Level          string
    Message        string
    DetailJSON     string
    CreatedAt      time.Time
}
```

要求：

- `HTTPSRequired` 必须默认开启，且不允许发布 `HTTP` 分享 URL
- 节点顺序必须稳定，避免同一档案重复渲染后输出顺序漂移
- 降级项必须跟随 `RenderVersion` 保存，不能只在前端临时展示
- 任务状态与档案状态必须分开保存，避免“已发布但当前任务失败”这类状态互相覆盖

### 27.3 生成与发布流程

建议状态机：

```text
draft
  -> previewing
  -> preview_ready
  -> publishing
  -> published

draft
  -> previewing
  -> preview_failed

published
  -> publishing
  -> published

published
  -> revoking
  -> revoked
```

建议流程：

1. 读取分享档案和已选节点
2. 拉取远程规则 URL 的最新快照
3. 读取本地自定义规则
4. 统一归一化为内部规则模型
5. 按目标客户端能力编译节点和规则
6. 生成 `RenderVersion`
7. 若为发布动作则签发或轮换 `subscriptions.token`
8. 写入渲染版本、降级项、审计日志和访问策略

关键要求：

- 预览成功不等于已发布
- 发布必须基于某个确定的 `RenderVersion`
- 二维码只允许编码单节点 URI 或 `HTTPS` 订阅链接
- 若目标客户端完全不支持规则注入，应显式降级为仅节点输出

### 27.4 服务接口建议

```go
type ServerShareService interface {
    CreateProfile(ctx context.Context, in CreateServerShareProfileInput) (*ServerShareProfile, error)
    UpdateProfile(ctx context.Context, profileID int64, in UpdateServerShareProfileInput) (*ServerShareProfile, error)
    PreviewProfile(ctx context.Context, profileID int64, in PreviewServerShareProfileInput) (*ServerShareRenderRevision, error)
    PublishProfile(ctx context.Context, profileID int64, in PublishServerShareProfileInput) (*PublishedServerShareResult, error)
    ExportProfile(ctx context.Context, profileID int64, renderVersion string) (*ServerShareExportResult, error)
    GenerateQRCode(ctx context.Context, profileID int64, renderVersion string, format string) (*ServerShareQRCodeResult, error)
    RevokePublishedSubscription(ctx context.Context, profileID int64, reason string) error
    ListRevisions(ctx context.Context, profileID int64) ([]ServerShareRenderRevision, error)
    ListAuditEvents(ctx context.Context, profileID int64, publishJobID *int64) ([]ServerShareAuditEvent, error)
    GetProfileDetail(ctx context.Context, profileID int64) (*ServerShareProfileDetail, error)
}

type ServerShareRuleCompiler interface {
    LoadRemoteRules(ctx context.Context, profileID int64) ([]ServerShareRuleSource, error)
    LoadCustomRules(ctx context.Context, profileID int64) ([]ServerShareRuleSource, error)
    Normalize(ctx context.Context, profileID int64) ([]ClientRouteRule, error)
    CompileForClient(ctx context.Context, clientType ServerShareClientType, nodes []ProxyNode, rules []ClientRouteRule) (*ServerShareCompiledArtifact, error)
}

type ServerShareFormatter interface {
    Format(ctx context.Context, artifact *ServerShareCompiledArtifact) ([]byte, string, error)
    BuildFileName(ctx context.Context, artifact *ServerShareCompiledArtifact) string
    BuildQRCodePayload(ctx context.Context, artifact *ServerShareCompiledArtifact) (string, error)
}

type PublishedServerShareResult struct {
    ProfileID        int64
    SubscriptionID   int64
    RenderVersion    string
    SubscriptionURL  string
    ExpiresAt        *time.Time
    Status           ServerShareProfileStatus
    PublishJobID     int64
}

type ServerShareCompiledArtifact struct {
    ProfileID        int64
    RenderVersion    string
    ClientType       ServerShareClientType
    OutputFormat     ServerShareOutputFormat
    DeliveryMode     ServerShareDeliveryMode
    Nodes            []ProxyNode
    Rules            []ClientRouteRule
    DegradeItems     []ServerShareDegradeItem
}

type ServerShareProfileDetail struct {
    Profile             *ServerShareProfile
    ActiveSubscriptionID *int64
    LastRenderVersion   string
    LastPublishJob      *ServerSharePublishJob
    ActiveRevision      *ServerShareRenderRevision
}
```

### 27.5 API DTO 建议

```go
type CreateServerShareProfileRequest struct {
    Name               string                         `json:"name"`
    ClientID           *int64                         `json:"client_id"`
    TargetClientType   ServerShareClientType         `json:"target_client_type"`
    OutputFormat       ServerShareOutputFormat       `json:"output_format"`
    DeliveryMode       ServerShareDeliveryMode       `json:"delivery_mode"`
    NodeIDs            []int64                       `json:"node_ids"`
    TemplateID         *int64                         `json:"template_id"`
    RouteTemplateID    *int64                         `json:"route_template_id"`
    SecurityMode       string                         `json:"security_mode"`
    BindMode           string                         `json:"bind_mode"`
    ExpiresAt          *time.Time                     `json:"expires_at"`
    RuleSources        []CreateServerShareRuleSourceRequest `json:"rule_sources"`
}

type CreateServerShareRuleSourceRequest struct {
    SourceType    ServerShareRuleSourceType `json:"source_type"`
    Name          string                    `json:"name"`
    SourceURL     string                    `json:"source_url"`
    CustomContent string                    `json:"custom_content"`
    RuleFormat    string                    `json:"rule_format"`
    Priority      int                       `json:"priority"`
}

type PreviewServerShareProfileRequest struct {
    RegenerateRules bool `json:"regenerate_rules"`
    IncludeQRCode   bool `json:"include_qrcode"`
}

type PublishServerShareProfileRequest struct {
    PublishMode             string `json:"publish_mode"`
    InvalidatePreviousToken bool   `json:"invalidate_previous_token"`
}

type ServerShareProfileResponse struct {
    ID                 int64                  `json:"id"`
    Name               string                 `json:"name"`
    TargetClientType   ServerShareClientType  `json:"target_client_type"`
    OutputFormat       ServerShareOutputFormat `json:"output_format"`
    DeliveryMode       ServerShareDeliveryMode `json:"delivery_mode"`
    SelectedNodeCount  int                    `json:"selected_node_count"`
    RemoteRuleURLCount int                    `json:"remote_rule_url_count"`
    CustomRuleCount    int                    `json:"custom_rule_count"`
    Status             ServerShareProfileStatus `json:"status"`
    HTTPSRequired      bool                   `json:"https_required"`
}

type ServerSharePreviewResponse struct {
    ProfileID         int64                    `json:"profile_id"`
    RenderVersion     string                   `json:"render_version"`
    ShareURL          string                   `json:"share_url"`
    QRReady           bool                     `json:"qr_ready"`
    PreviewExcerpt    string                   `json:"preview_excerpt"`
    DegradeItems      []ServerShareDegradeItem `json:"degrade_items"`
}

type ServerShareProfileDetailResponse struct {
    ID                    int64                  `json:"id"`
    Name                  string                 `json:"name"`
    Status                ServerShareProfileStatus `json:"status"`
    LastRenderVersion     string                 `json:"last_render_version"`
    LastPublishJobStatus  ServerShareJobStatus   `json:"last_publish_job_status"`
    LastErrorCode         string                 `json:"last_error_code"`
    CanRetry              bool                   `json:"can_retry"`
    CanReuseLastRender    bool                   `json:"can_reuse_last_render"`
    RequiresAction        bool                   `json:"requires_action"`
    RequiresActionReason  string                 `json:"requires_action_reason"`
    SubscriptionURLMasked string                 `json:"subscription_url_masked"`
}

type ServerShareAuditEventResponse struct {
    ID         int64  `json:"id"`
    EventType  string `json:"event_type"`
    Level      string `json:"level"`
    Message    string `json:"message"`
    DetailJSON string `json:"detail_json"`
    CreatedAt  string `json:"created_at"`
}
```

接口约束：

- `CreateServerShareProfileRequest.NodeIDs` 不能为空
- `DeliveryMode` 为订阅链接类型时必须校验当前站点 `HTTPS` 已就绪
- `PublishServerShareProfileRequest` 必须引用最近一次成功预览或显式触发重新生成
- `ServerShareProfileDetailResponse` 必须直接返回按钮所需的 `CanRetry`、`CanReuseLastRender`、`RequiresAction`

### 27.6 前端状态建议

```ts
type ServerSharePageState = {
  profileId?: number
  targetClientType: 'sing-box' | 'xray-core' | 'mihomo' | 'universal_uri'
  outputFormat: 'singbox_json' | 'xray_json' | 'mihomo_yaml' | 'raw_uri' | 'base64_uri_bundle'
  deliveryMode: 'single_uri' | 'single_uri_qr' | 'https_subscription' | 'https_subscription_qr' | 'file_download'
  selectedNodeIds: number[]
  routeTemplateId?: number
  remoteRuleSources: ServerShareRuleSource[]
  customRuleSources: ServerShareRuleSource[]
  renderVersion?: string
  shareURL?: string
  degradeItems: ServerShareDegradeItem[]
  lastPublishJobStatus?: 'created' | 'preparing_sources' | 'rendering' | 'issuing_token' | 'persisting_revision' | 'succeeded' | 'failed' | 'needs_intervention'
  canRetry: boolean
  canReuseLastRender: boolean
  requiresAction: boolean
  requiresActionReason?: string
}
```

要求：

- 页面切换目标客户端时，只重算预览，不清空已选节点
- 若降级项非空，发布前必须二次确认
- 若当前站点 `HTTPS` 不可用，订阅链接和二维码相关动作必须统一禁用
- 详情页必须直接消费服务端返回的任务状态，而不是前端自行推测按钮可用性

### 27.7 后端目录与路由落点建议

建议目录：

```text
internal/
  api/
    routes/
      server_share.go
    handlers/
      server_share/
        create_profile.go
        update_profile.go
        get_profile.go
        list_profiles.go
        list_revisions.go
        list_events.go
        preview_profile.go
        publish_profile.go
        export_profile.go
        export_qrcode.go
        revoke_profile.go
  service/
    server_share/
      profile_service.go
      render_service.go
      publish_service.go
      export_service.go
      audit_service.go
  repository/
    server_share/
      profile_repository.go
      node_repository.go
      rule_source_repository.go
      render_revision_repository.go
      publish_job_repository.go
      audit_event_repository.go
  security/
    audit/
      server_share_audit_masker.go
    subscription/
      token_issuer.go
      rate_limiter.go
      access_risk_evaluator.go
  subscription/
    formatter/
      singbox_formatter.go
      xray_formatter.go
      mihomo_formatter.go
      raw_uri_formatter.go
      qrcode_payload_builder.go
```

路由注册建议：

```go
func RegisterServerShareRoutes(r chi.Router, deps *Deps) {
    r.Route("/api/v1/server-share", func(r chi.Router) {
        r.Use(AuthMiddleware(deps.Auth, PermissionServerShareManage))
        r.Get("/profiles", deps.ServerShareHandler.ListProfiles)
        r.Post("/profiles", deps.ServerShareHandler.CreateProfile)
        r.Get("/profiles/{id}", deps.ServerShareHandler.GetProfile)
        r.Put("/profiles/{id}", deps.ServerShareHandler.UpdateProfile)
        r.Get("/profiles/{id}/revisions", deps.ServerShareHandler.ListRevisions)
        r.Get("/profiles/{id}/events", deps.ServerShareHandler.ListEvents)
        r.Post("/profiles/{id}/preview", deps.ServerShareHandler.PreviewProfile)
        r.Post("/profiles/{id}/publish", deps.ServerShareHandler.PublishProfile)
        r.Post("/profiles/{id}/revoke", deps.ServerShareHandler.RevokeProfile)
        r.Get("/profiles/{id}/export", deps.ServerShareHandler.ExportProfile)
        r.Get("/profiles/{id}/qrcode", deps.ServerShareHandler.ExportQRCode)
    })
}
```

要求：

- 服务端分享管理接口统一注册在独立 `server_share.go` 路由文件中
- `GET /sub/:token` 继续保留在匿名订阅路由，不挂到管理路由组
- 导出类接口不走匿名地址，避免绕开权限和审计

### 27.8 权限与审计中间件建议

权限建议：

- `PermissionServerShareManage`：创建、编辑、预览、发布、吊销
- `PermissionServerShareExport`：导出文件、导出二维码
- `PermissionServerShareReadSensitive`：查看完整订阅 URL、查看未脱敏二维码载荷

中间件挂载顺序建议：

1. `RequestIDMiddleware`
2. `SessionAuthMiddleware`
3. `PermissionMiddleware`
4. `AuditScopeMiddleware`
5. `Handler`

规则：

- `AuditScopeMiddleware` 为服务端分享接口注入 `resource_type=server_share_profile`
- 预览、发布、导出、吊销动作必须在 handler 返回前后写审计事件
- 默认响应只能返回 `subscription_url_masked`
- 只有命中 `PermissionServerShareReadSensitive` 才允许显式取回完整链接或二维码原始载荷

### 27.9 页面数据装配建议

详情页建议由聚合查询直接返回，避免前端自行拼多个接口推断状态。

```go
type ServerShareDetailAssembler interface {
    BuildProfileDetail(ctx context.Context, profileID int64) (*ServerShareProfileDetailResponse, error)
    BuildRevisionList(ctx context.Context, profileID int64) ([]ServerShareRevisionResponse, error)
    BuildEventList(ctx context.Context, profileID int64, publishJobID *int64) ([]ServerShareAuditEventResponse, error)
}
```

要求：

- 详情接口一次性返回档案主信息、最近任务状态、当前生效版本、当前订阅 URL 脱敏值
- 历史版本接口只返回列表，不重复拼接大体积预览内容
- 审计事件接口允许按 `publish_job_id` 过滤，便于详情页回放单次失败链路

### 27.10 前端组件与 store 建议

```ts
type ServerShareStore = {
  profileList: ServerShareProfileResponse[]
  activeProfile?: ServerShareProfileDetailResponse
  revisions: ServerShareRevisionResponse[]
  events: ServerShareAuditEventResponse[]
  accessLogs: SubscriptionAccessLog[]
  preview?: ServerSharePreviewResponse
  loading: {
    list: boolean
    detail: boolean
    preview: boolean
    publish: boolean
    revisions: boolean
    events: boolean
    accessLogs: boolean
  }
  actions: {
    fetchProfiles(): Promise<void>
    fetchProfileDetail(profileID: number): Promise<void>
    fetchRevisions(profileID: number): Promise<void>
    fetchEvents(profileID: number, publishJobID?: number): Promise<void>
    previewProfile(profileID: number, payload: PreviewServerShareProfileRequest): Promise<void>
    publishProfile(profileID: number, payload: PublishServerShareProfileRequest): Promise<void>
  }
}
```

组件建议：

- `ServerSharePage.vue`
- `ServerShareListTable.vue`
- `ServerShareFormPanel.vue`
- `ServerShareNodeSelector.vue`
- `ServerShareRuleSourcePanel.vue`
- `ServerSharePreviewPanel.vue`
- `ServerShareDetailDrawer.vue`
- `ServerShareRevisionTable.vue`
- `ServerShareEventTimeline.vue`
- `ServerShareAccessLogTable.vue`

要求：

- 详情抽屉打开时并行拉取详情、历史版本、审计事件
- 访问日志按页签懒加载，避免首屏请求过重
- 预览成功后只刷新预览和详情摘要，不强制重载整个列表
- 发布成功后刷新当前档案详情、历史版本和列表摘要

### 27.11 前端接口类型与 SDK 建议

建议目录：

```text
web/src/
  api/
    client.ts
    server-share.ts
  types/
    server-share.ts
  stores/
    serverShare.ts
```

建议类型：

```ts
export type ServerShareProfileSummary = {
  id: number
  name: string
  target_client_type: 'sing-box' | 'xray-core' | 'mihomo' | 'universal_uri'
  output_format: string
  delivery_mode: string
  selected_node_count: number
  remote_rule_url_count: number
  custom_rule_count: number
  status: string
  https_required: boolean
}

export type ServerShareApiError = {
  error: {
    code: string
    message: string
    request_id: string
    can_retry: boolean
    can_reuse_last_render: boolean
    requires_action: boolean
    requires_action_reason: string
    detail: Record<string, unknown>
  }
}
```

建议 SDK：

```ts
export const serverShareApi = {
  listProfiles(params?: Record<string, unknown>) {
    return apiClient.get<Paginated<ServerShareProfileSummary>>('/api/v1/server-share/profiles', { params })
  },
  getProfile(id: number) {
    return apiClient.get<ServerShareProfileDetailResponse>(`/api/v1/server-share/profiles/${id}`)
  },
  getRevisions(id: number) {
    return apiClient.get<ServerShareRevisionResponse[]>(`/api/v1/server-share/profiles/${id}/revisions`)
  },
  getEvents(id: number, publishJobID?: number) {
    return apiClient.get<ServerShareAuditEventResponse[]>(`/api/v1/server-share/profiles/${id}/events`, {
      params: publishJobID ? { publish_job_id: publishJobID } : undefined,
    })
  },
  preview(id: number, payload: PreviewServerShareProfileRequest) {
    return apiClient.post<ServerSharePreviewResponse>(`/api/v1/server-share/profiles/${id}/preview`, payload)
  },
  publish(id: number, payload: PublishServerShareProfileRequest) {
    return apiClient.post<PublishedServerShareResult>(`/api/v1/server-share/profiles/${id}/publish`, payload)
  },
}
```

调用约定：

- `api/client.ts` 统一注入 `request_id`、鉴权头和错误解包逻辑
- `server-share.ts` 只保留资源相关调用，不混入 UI 逻辑
- store 只调用 SDK，不直接拼 URL
- 页面组件只消费 store 暴露的数据和 action

### 27.12 数据库迁移与索引补充建议

服务端节点分享建议单独一组迁移文件：

```text
migrations/
  2026052501_create_server_share_profiles.sql
  2026052502_create_server_share_profile_nodes.sql
  2026052503_create_server_share_rule_sources.sql
  2026052504_create_server_share_render_revisions.sql
  2026052505_create_server_share_publish_jobs.sql
  2026052506_create_server_share_audit_events.sql
  2026052507_add_server_share_relation_to_subscriptions.sql
  2026052508_add_server_share_indexes.sql
```

要求：

- 表结构与索引拆分成不同迁移文件
- 若给 `subscriptions` 增加分享档案关联，必须保证旧版本兼容读取
- 索引迁移放在最后，避免升级窗口过长

## 28. 前端公共 API / DTO 类型库设计

建议目录：

```text
web/src/
  api/
    client.ts
  types/
    common.ts
    api.ts
    errors.ts
    pagination.ts
    async-job.ts
  sdk/
    core.ts
```

公共类型建议：

```ts
export type ISODateTimeString = string

export type IDNamePair = {
  id: number | string
  name: string
}

export type Paginated<T> = {
  items: T[]
  total: number
  page: number
  page_size: number
}

export type ApiSuccess<T> = {
  data: T
  request_id: string
}

export type ApiError = {
  error: {
    code: string
    message: string
    request_id: string
    can_retry?: boolean
    requires_action?: boolean
    requires_action_reason?: string
    detail?: Record<string, unknown>
  }
}

export type AsyncJobRef = {
  job_id: number
  status: string
  detail_url?: string
}
```

SDK 约定：

- `api/client.ts` 只负责 HTTP 基础能力、鉴权头、`request_id` 注入和错误解包
- `sdk/core.ts` 负责把 `ApiSuccess<T>`、`ApiError` 统一转换成前端易用接口
- 各模块 SDK 只封装业务资源方法，不重复声明公共 Envelope
- 公共分页、时间、错误和异步任务引用都从 `web/src/types/` 导出

## 29. 服务端订阅额度与周期控制代码级设计

```go
type SubscriptionQuotaState string

// 统一枚举，与 QuotaState 保持一致
const (
    SubscriptionQuotaStateNormal      SubscriptionQuotaState = "normal"
    SubscriptionQuotaStateNearLimit   SubscriptionQuotaState = "near_limit"
    SubscriptionQuotaStateOverLimitSoft SubscriptionQuotaState = "over_limit_soft"
    SubscriptionQuotaStateOverLimitHard SubscriptionQuotaState = "over_limit_hard"
    SubscriptionQuotaStateSuspended   SubscriptionQuotaState = "suspended"
    SubscriptionQuotaStateRestored    SubscriptionQuotaState = "restored"
    SubscriptionQuotaStateExpiredDays SubscriptionQuotaState = "expired_days"
)

type SubscriptionResetCycle string

const (
    SubscriptionResetCycleNone    SubscriptionResetCycle = "none"
    SubscriptionResetCycleMonthly SubscriptionResetCycle = "monthly"
)

type SubscriptionEntitlement struct {
    SubscriptionID            int64
    QuotaBytes                int64
    UsedBytesCache            int64
    ResetCycle                SubscriptionResetCycle
    ResetDay                  *int
    LastResetAt               *time.Time
    NextResetAt               *time.Time
    MaxActiveDays             *int
    ActivatedAt               *time.Time
    ExpiresAt                 *time.Time
    SuspendOnQuotaExhausted   bool
    QuotaState                SubscriptionQuotaState
    SuspendedAt               *time.Time
}
```

接口建议：

```go
type SubscriptionEntitlementService interface {
    Evaluate(ctx context.Context, subscriptionID int64) (*SubscriptionEntitlement, error)
    MarkActivated(ctx context.Context, subscriptionID int64, activatedAt time.Time) error
    ResetMonthlyUsage(ctx context.Context, subscriptionID int64, operatorID *int64) error
    SuspendOnQuotaExhausted(ctx context.Context, subscriptionID int64, reason string) error
}
```

规则：

- `max_active_days` 从首次成功代理认证时开始计时，不从首次拉取订阅时计时
- 月度流量的真实来源仍以 `traffic_usage_summary` 为准，`UsedBytesCache` 只做快速展示
- 订阅拉取流量不计入代理配额
- 配额用尽默认拒绝新代理连接，不自动吊销订阅 URL

接口 DTO 建议：

```go
type ResetSubscriptionUsageRequest struct {
    Reason         string `json:"reason"`
    ResetUsedBytes bool   `json:"reset_used_bytes"`
}

type RestoreSubscriptionRequest struct {
    Reason                  string `json:"reason"`
    ClearSuspendedState     bool   `json:"clear_suspended_state"`
    RecalculateNextResetAt  bool   `json:"recalculate_next_reset_at"`
}

type SubscriptionEntitlementResponse struct {
    SubscriptionID    int64   `json:"subscription_id"`
    QuotaBytes        int64   `json:"quota_bytes"`
    UsedBytes         int64   `json:"used_bytes"`
    QuotaState        string  `json:"quota_state"`
    ResetCycle        string  `json:"reset_cycle"`
    ResetDay          *int    `json:"reset_day,omitempty"`
    ActivatedAt       *string `json:"activated_at,omitempty"`
    MaxActiveDays     *int    `json:"max_active_days,omitempty"`
    LastResetAt       *string `json:"last_reset_at,omitempty"`
    NextResetAt       *string `json:"next_reset_at,omitempty"`
    SuspendedAt       *string `json:"suspended_at,omitempty"`
}
```

页面装配建议：

```go
type SubscriptionQuotaSummaryResponse struct {
    SubscriptionID      int64   `json:"subscription_id"`
    Status              string  `json:"status"`
    QuotaState          string  `json:"quota_state"`
    QuotaBytes          int64   `json:"quota_bytes"`
    UsedBytes           int64   `json:"used_bytes"`
    RemainingBytes      int64   `json:"remaining_bytes"`
    ActivatedAt         *string `json:"activated_at,omitempty"`
    RemainingActiveDays *int    `json:"remaining_active_days,omitempty"`
    NextResetAt         *string `json:"next_reset_at,omitempty"`
    LastAccessAt        *string `json:"last_access_at,omitempty"`
    SuspendedAt         *string `json:"suspended_at,omitempty"`
    CanResetUsage       bool    `json:"can_reset_usage"`
    CanRestore          bool    `json:"can_restore"`
}

type SubscriptionQuotaDetailResponse struct {
    SubscriptionItem
    Entitlement             SubscriptionEntitlementResponse `json:"entitlement"`
    RemainingBytes          int64                           `json:"remaining_bytes"`
    RemainingActiveDays     *int                            `json:"remaining_active_days,omitempty"`
    EffectiveExpireAt       *string                         `json:"effective_expire_at,omitempty"`
    CanResetUsage           bool                            `json:"can_reset_usage"`
    CanRestore              bool                            `json:"can_restore"`
    CanRotate               bool                            `json:"can_rotate"`
    CanRecalculateNextReset bool                            `json:"can_recalculate_next_reset"`
}
```

前端状态建议：

```ts
type SubscriptionQuotaStoreState = {
  items: SubscriptionQuotaSummaryResponse[]
  activeItem?: SubscriptionQuotaDetailResponse
  resetLogs: SubscriptionUsageResetLog[]
  accessLogs: SubscriptionAccessLog[]
  events: AuditEventItem[]
  loading: {
    list: boolean
    detail: boolean
    resetLogs: boolean
    accessLogs: boolean
    events: boolean
    resetUsage: boolean
    restore: boolean
  }
}
```

接口组织建议：

- `GET /api/v1/subscriptions` 返回列表页额度摘要字段
- `GET /api/v1/subscriptions/:id` 返回详情页完整额度对象
- `POST /api/v1/subscriptions/:id/reset-usage` 返回最新 `SubscriptionQuotaDetailResponse`
- `POST /api/v1/subscriptions/:id/restore` 返回最新 `SubscriptionQuotaDetailResponse`

## 24. 非本期扩展项

以下内容不纳入当前单节点版本实现范围，只作为后续专题：

- 更细粒度的 UDP 独立连接状态与限速模型
- 单节点内跨核心统一风控缓存优化
- 面向反向代理层的联合防护增强
- 面向 `mTLS` 证书指纹的访问控制增强
- 升级维护模式的高级交互优化

本文件到此可直接作为当前版本的编码蓝图使用。
