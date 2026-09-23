# 流量监控与配额控制设计文档

> **实现状态（2026-09-23 复核）**：设计目标已部分落地，但实现路径有出入：
> - 订阅级用量：本机走 `service/trafficsync`（xray `QueryStats reset_=true`），远程走 Agent heartbeat → `traffic_consumer.go` → `inboundconfig.UpdateTraffic`（按 inbound 中的 email 匹配累加 `subscriptions.used_bytes_cache`）；非 xray 协议走网络层计费（`traffic/ip_tracker.go` 的 `SubscriptionIPTracker` —— **已接线**：`router.go` 两处 `SetNetworkTracker`（admin 与 portal 路径））。
> - 节点级用量：Agent 上报出口网卡累计 rx/tx（`EgressRxBytes/EgressTxBytes`），面板做差累计到 `NodeServer.traffic_used_bytes` 并触发超额 suspendNode。
> - 原设计的 `TrafficUsageLedger` / `TrafficUsageSummary` 表未被写入（旧方案废弃）。
- **（2026-09-23 复核）订阅级超额自动封禁与自然过期自动下线已实现**：`subscription.EnforceQuotaAndExpiry`
  （`ListQuotaExhausted` / `ListExpired` → 置 suspended/expired + 拉黑客户端 + 从核心移除用户），
  并由 `StartEnforcer` 随面板启动（每 5 分钟，启动即跑一次）。
> 详见 `UNIMPLEMENTED_FEATURES.md` 2.2、2.3。

## 1. 目标

本设计用于解决以下问题：

- 监控用户、节点、服务的流量使用情况
- 当用户或实例超过对应流量配额后拒绝新连接
- 在多协议场景下保持流量统计口径一致
- 让流量计算尽可能准确，并可审计、可追溯

本设计覆盖：

- `Xray-core`
- `sing-box`
- `Hysteria 2`
- `TUIC`

## 2. 核心结论

流量统计必须拆成两套口径，不能混在一起。

### 2.1 计费流量

用于：

- 用户流量配额
- 套餐余量
- 超量拒绝连接
- 账单和限额控制

计费流量建议定义为：

- 用户通过代理核心成功建立会话后
- 由核心实际转发的上行字节数 + 下行字节数
- 按用户、设备、订阅、节点、协议维度归属

### 2.2 物理带宽

用于：

- 运维监控
- 出口成本分析
- 网卡带宽容量规划
- 峰值流量分析

物理带宽建议定义为：

- 操作系统网卡层真实收发字节
- 包含协议开销、TLS/QUIC 开销、可能的重传影响

结论：

- 配额判断必须基于计费流量
- 运维监控可以额外展示物理带宽
- 不要拿网卡字节直接做用户计费

## 3. 为什么不能直接用网卡流量做用户配额

直接使用系统网卡统计有几个问题：

- 无法精确归属到具体用户
- 无法稳定区分不同协议实例
- 可能混入面板流量、证书流量、订阅流量
- 会包含 TCP 重传、TLS/QUIC 开销
- 会把绕过直连流量也混进去

因此：

- 网卡流量适合做系统监控
- 不适合直接做用户配额结算

## 4. 推荐的准确计量口径

### 4.1 配额统计口径

最推荐的口径是：

- 在代理核心认证成功后开始计量
- 以每个已认证会话的转发字节为准
- 分别记录上行和下行
- 总流量 = 上行 + 下行

建议纳入配额的流量：

- 进入代理核心并被成功转发的 TCP 流量
- 进入代理核心并被成功转发的 UDP 流量
- `TUN` 模式下最终进入代理核心处理的流量

建议不纳入配额的流量：

- 绕过规则命中的直连流量
- 面板管理流量
- 订阅下发流量
- 证书申请和续期流量
- 探针与健康检查流量

### 4.2 物理带宽统计口径

物理带宽建议从以下来源获取：

- 网卡字节计数器
- `nftables` / `iptables` 规则计数器
- `eBPF` 网络统计

它用于：

- 观测真实出口带宽
- 辅助核对核心计费流量

但不直接参与用户限额判断。

## 5. 多协议统一计量原则

为了让 `Xray-core`、`sing-box`、`Hysteria 2`、`TUIC` 的统计一致，必须统一规则：

- 都按“认证后、成功转发”的字节数统计
- 都同时记录上行和下行
- 都以字节整数存储，不用浮点
- 都以服务端采样为准，不信任客户端上报
- 都保留原始增量和聚合结果

### 5.1 不同协议的差异处理

不同协议在网络层的开销不同：

- `Hysteria 2` 与 `TUIC` 基于 `QUIC`
- `Xray-core` 与 `sing-box` 可能走 `TCP`、`gRPC`、`WS`、`HTTP/2`

如果直接按网卡层计量：

- 协议开销差异会直接反映到用户配额
- 对用户不公平

所以建议：

- 用户配额按逻辑转发字节算
- 物理成本按网卡层字节另算

## 6. 采集架构设计

### 6.1 总体思路

架构前提：面板不在流量路径上。用户直连核心，或经过 nginx/caddy 反向代理到核心。面板是纯管理面，只能读取核心统计 API 和修改核心配置。

推荐设计为核心 Stats API 采集：

- 面板定时轮询核心 Stats API（每 30 秒）
- 核心负责实际的流量转发和用户认证
- 面板负责统计汇总、配额管理、用户移除/恢复

这样做的好处：

- 面板不在流量路径上，不引入任何延迟
- 核心 Stats API 是核心自身的统计能力，精度高
- 面板只做异步采集和管理，不影响核心性能

### 6.2 核心 Stats API 采集（主方案）

面板通过轮询核心 Stats API 获取流量数据：

| 核心 | API | 增量获取 | 用户级 | 配置要求 |
| --- | --- | --- | --- | --- |
| Xray-core | gRPC `StatsService.QueryStats` | `reset_=true` 查询后重置 | 按 email | `stats: {}` + `api.services` |
| sing-box | HTTP `GET /traffic` | 计算差值 | 按 tag | `experimental.traffic_api` |
| Hysteria 2 | HTTP `GET /traffic?clear=true` | `clear=true` 直接增量 | 按 auth 用户 | `trafficStats.listen` |
| TUIC | 无稳定 API | 降级为 `/proc/<pid>/net/dev` | 无 | 无 |

采集流程：

```text
面板定时轮询（每 30 秒，独立 goroutine）
    │
    ├── Xray: gRPC QueryStats(reset_=true) → 按 email 解析增量
    ├── sing-box: HTTP GET /traffic → 计算差值
    ├── Hysteria 2: HTTP GET /traffic?clear=true → 直接增量
    └── TUIC: 读 /proc/<pid>/net/dev → 进程级增量
    │
    ▼
TrafficAggregator → 账本 → 配额状态机 → 超额时调用核心 API 移除用户
```

平台支持：

| 平台 | 支持情况 | 说明 |
| --- | --- | --- |
| Linux | 完整支持 | 所有核心 Stats API 均可用 |
| OpenWrt | 完整支持 | 同 Linux |
| macOS | 完整支持 | 同 Linux（面板通过 localhost 调用核心 API） |
| Windows | 完整支持 | 同 Linux（面板通过 localhost 调用核心 API） |

### 6.3 核心 Stats API 采集

作为网络层计数的交叉校验手段：

优先顺序建议：

1. 核心原生统计接口（Xray gRPC Stats API、sing-box HTTP API、Hysteria 2 控制台 API）
2. 核心管理 API
3. 面板托管的统计适配器

校验逻辑：

- 定时（每 5 分钟）对比网络层计数和核心 API 计数
- 偏差超过 10% 时触发告警
- 两者都失败时降级为进程级 `/proc/net/dev`

### 6.4 系统侧采集

系统侧用于采集：

- 网卡总流量
- 进程级带宽
- 端口级带宽
- 路由标签命中的直连流量

可用于交叉校验：

- 代理计费流量
- 直连绕过流量
- 系统总出口

## 7. 计量事件模型

流量统计必须尽量保留原始增量，而不是只保留最终总数。

建议最小事件模型：

```json
{
  "session_id": "sess_abc123",
  "user_id": 1001,
  "subscription_id": 2001,
  "service_id": 3001,
  "core_type": "xray",
  "protocol": "vless",
  "direction": "uplink",
  "delta_bytes": 16384,
  "seq": 12,
  "occurred_at": "2026-05-22T12:00:00Z"
}
```

关键点：

- 使用 `delta_bytes` 增量上报
- 使用 `session_id + seq` 去重
- 全部使用整数 byte
- 以服务端时间为准

## 8. 数据库设计

### 8.1 traffic_quotas

```sql
CREATE TABLE traffic_quotas (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  subject_type TEXT NOT NULL,
  subject_id INTEGER NOT NULL,
  reset_cycle TEXT NOT NULL DEFAULT 'monthly',
  quota_bytes INTEGER NOT NULL,
  hard_limit INTEGER NOT NULL DEFAULT 1,
  grace_bytes INTEGER NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
);
```

说明：

- `subject_type` 可为 `user`、`subscription`、`device`
- `hard_limit` 表示是否超量立即拒绝
- `grace_bytes` 用于宽限
- 服务端订阅配额建议使用 `subject_type = subscription`

### 8.2 traffic_usage_ledger

```sql
CREATE TABLE traffic_usage_ledger (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  session_id TEXT NOT NULL,
  seq INTEGER NOT NULL,
  user_id INTEGER,
  subscription_id INTEGER,
  service_id INTEGER,
  core_type TEXT NOT NULL,
  protocol TEXT NOT NULL,
  uplink_bytes INTEGER NOT NULL DEFAULT 0,
  downlink_bytes INTEGER NOT NULL DEFAULT 0,
  billed_bytes INTEGER NOT NULL DEFAULT 0,
  occurred_at DATETIME NOT NULL,
  UNIQUE(session_id, seq)
);
```

说明：

- 这是原始增量账本
- 用于准确审计和去重

### 8.3 traffic_usage_summary

```sql
CREATE TABLE traffic_usage_summary (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  subject_type TEXT NOT NULL,
  subject_id INTEGER NOT NULL,
  period_start DATETIME NOT NULL,
  period_end DATETIME NOT NULL,
  uplink_bytes INTEGER NOT NULL DEFAULT 0,
  downlink_bytes INTEGER NOT NULL DEFAULT 0,
  billed_bytes INTEGER NOT NULL DEFAULT 0,
  updated_at DATETIME NOT NULL,
  UNIQUE(subject_type, subject_id, period_start, period_end)
);
```

说明：

- 用于面板快速查询
- 由账本聚合产生

### 8.4 traffic_sessions

```sql
CREATE TABLE traffic_sessions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  session_id TEXT NOT NULL UNIQUE,
  user_id INTEGER,
  subscription_id INTEGER,
  service_id INTEGER NOT NULL,
  core_type TEXT NOT NULL,
  protocol TEXT NOT NULL,
  status TEXT NOT NULL,
  started_at DATETIME NOT NULL,
  ended_at DATETIME,
  final_uplink_bytes INTEGER NOT NULL DEFAULT 0,
  final_downlink_bytes INTEGER NOT NULL DEFAULT 0
);
```

用途：

- 记录连接生命周期
- 支持会话级追踪

### 8.5 traffic_quota_reset_logs

```sql
CREATE TABLE traffic_quota_reset_logs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  subject_type TEXT NOT NULL,
  subject_id INTEGER NOT NULL,
  reset_cycle TEXT NOT NULL,
  period_start DATETIME NOT NULL,
  period_end DATETIME NOT NULL,
  billed_bytes_before_reset INTEGER NOT NULL DEFAULT 0,
  trigger_source TEXT NOT NULL,
  operator_id INTEGER,
  created_at DATETIME NOT NULL
);
```

用途：

- 记录月度自动重置
- 记录管理员手动重置
- 用于配额审计和账务核对

## 9. 流量计算流程

推荐流程：

1. 用户建立连接
2. 核心完成认证
3. 检查当前配额状态
4. 若已超量，直接拒绝连接
5. 若未超量，允许会话建立
6. 会话中持续上报流量增量
7. 聚合器累加到账本和摘要表
8. 如果达到硬限额，立即标记超量
9. 拒绝后续新连接
10. 可选地中断当前活跃连接

## 10. 超量拒绝连接设计

### 10.1 拒绝策略

建议支持两种模式：

- `hard_stop`
- `soft_stop`

#### `hard_stop`

- 达到配额后拒绝所有新连接
- 可选断开当前连接
- 适合严格套餐场景

#### `soft_stop`

- 达到配额后拒绝新连接
- 已建立连接允许运行到结束
- 适合用户体验优先场景

### 10.2 最推荐默认策略

默认建议：

- 拒绝新连接
- 当前活跃连接可配置是否断开

原因：

- 能确保不会继续无限超量
- 也避免频繁误杀短连接

### 10.3 拒绝检查点

必须至少在以下时机检查：

- 新连接认证后
- 会话增量累计后
- 周期性配额同步后

### 10.4 服务端订阅月度重置与有效天数

针对 `subject_type = subscription`，建议额外支持：

- 固定过期时间
- 激活后有效天数
- 月度流量重置

规则：

- `activated_at` 以首次成功代理认证时间为准，不以首次拉取订阅时间为准
- `max_active_days` 到期后，状态进入 `expired_days`
- `reset_cycle = monthly` 时，按 `reset_day` 或首次激活日计算 `next_reset_at`
- 到达 `next_reset_at` 后由后台任务执行清零，并写入 `traffic_quota_reset_logs`
- 自动重置失败时不清零，继续维持当前超量状态并告警

### 10.5 超量后的默认动作

默认建议：

- 拒绝新的代理连接
- 保留订阅 URL 可访问性，避免客户端导入链路失真
- 是否中断现有连接由 `hard_limit` 控制
- 若业务策略要求“彻底停用”，再把订阅状态切到 `suspended`

## 11. 怎样保证统计尽可能准确

### 11.1 不用浮点

所有流量必须使用：

- `INTEGER`
- 单位统一为 `byte`

### 11.2 不做覆盖写

不要只更新一个总量字段。

必须：

- 保留增量账本
- 再做聚合汇总

### 11.3 做幂等去重

核心或 agent 重试上报时：

- 必须能按 `session_id + seq` 去重
- 防止重复累计

### 11.4 用服务端时间

不要信任客户端时间戳。

### 11.5 以核心认证后统计为准

必须在用户身份明确后统计，否则无法正确归属。

### 11.6 直连绕过流量单独统计

绕过流量可以监控，但不能记入代理配额。

### 11.7 核心统计与网卡统计双重核对

建议每日或每小时做偏差检查：

- 核心计费流量总和
- 物理带宽总流量
- 绕过直连流量

如果偏差超阈值，需要告警。

## 12. `TUN`、绕过和配额关系

这里必须明确：

- `TUN` 只是接管流量入口
- 不是所有 `TUN` 流量都记入代理配额

规则：

- 命中绕过并直连的流量，不计入代理配额
- 命中代理策略并进入核心的流量，计入代理配额
- `IPv6` 与 `QUIC` 的代理开关只影响代理路径上的流量统计
- 直连流量即使走 `IPv6` 或 `QUIC`，也不计入代理配额

## 13. UI 与监控展示建议

监控中心建议拆成两类卡片：

### 13.1 配额流量

展示：

- 用户已用流量
- 剩余流量
- 当前周期
- 是否已超量
- 最近触发拒绝时间

### 13.2 物理带宽

展示：

- 网卡实时上行
- 网卡实时下行
- 24 小时峰值
- 直连流量
- 代理流量

### 13.3 差异展示

必须明确告诉用户：

- 计费流量
- 物理带宽
- 绕过直连流量

三者不是同一个数。

### 13.4 订阅额度卡片建议

展示：

- 当前周期已用流量
- 月度总配额
- `quota_state`
- `activated_at`
- `max_active_days`
- `next_reset_at`
- 最近一次超量时间
- 最近一次重置时间

## 14. 实现建议

### 14.1 第一版最务实方案

第一版建议这样做：

- 每个核心做一个流量统计适配器
- 优先对接核心已有统计能力
- 统一转换成增量事件
- 本地 agent 或 panel 写入 SQLite
- 配额判断只基于统一账本

### 14.2 服务端订阅第一版建议

- 第一版先做 `subscription` 维度月度配额
- 第一版先支持 `none` 和 `monthly` 两种重置周期
- 第一版先支持“超量拒绝新连接，不自动吊销订阅 URL”
- 有效天数从首次成功代理认证开始计算

不要第一版就依赖：

- 仅日志解析
- 仅网卡计数
- 仅进程流量估算

这些都不够准确。

### 14.3 长期建议

长期建议增加：

- `eBPF` 辅助核对
- 实时事件总线
- 分布式配额缓存
- 多主机统一聚合

## 15. 最终建议

如果你要“超过对应流量就拒绝连接”，并且还要求“计算必须准确”，最合适的设计是：

- 用户配额按核心认证后的逻辑转发字节计算
- 系统网卡流量只做运维监控和交叉核对
- 通过增量账本 + 汇总表保证准确性与可审计性
- 通过认证后预检查 + 会话中持续累计 + 超量后拒绝新连接实现限额控制

最关键的一句：

- 配额统计基于核心会话字节
- 不是基于网卡总字节

## 16. 多核心流量采集适配器设计

多核心流量采集不能写死在单一核心实现里，建议统一成适配器体系。

### 16.1 设计目标

适配器体系需要解决：

- 不同核心统计接口差异
- 不同协议会话标识差异
- 不同主机系统部署差异
- 统一输出流量增量事件

### 16.2 统一适配器接口

建议定义统一接口：

```go
type TrafficCollectorAdapter interface {
    CoreType() string
    Start(ctx context.Context, in CollectorStartInput) error
    Stop(ctx context.Context) error
    Snapshot(ctx context.Context) (*CollectorSnapshot, error)
    Subscribe(ctx context.Context, ch chan<- TrafficDeltaEvent) error
}
```

建议输入输出：

```go
type CollectorStartInput struct {
    ServiceID   int64
    HostID      int64
    CoreType    string
    RuntimeMeta map[string]string
}

type CollectorSnapshot struct {
    ServiceID      int64
    ActiveSessions int64
    UplinkBytes    int64
    DownlinkBytes  int64
    CollectedAt    time.Time
}

type TrafficDeltaEvent struct {
    SessionID      string
    UserID         int64
    SubscriptionID int64
    ServiceID      int64
    CoreType       string
    Protocol       string
    EndpointID     int64
    UplinkBytes    int64
    DownlinkBytes  int64
    Seq            int64
    OccurredAt     time.Time
}
```

### 16.3 适配器职责边界

适配器只负责：

- 从核心或 agent 读取统计数据
- 识别会话和归属对象
- 产出标准增量事件

适配器不负责：

- 配额判断
- 账本聚合
- UI 展示
- 套餐结算

### 16.4 每个核心的适配策略

#### `Xray-core`

推荐优先顺序：

1. 管理 API / stats 接口
2. 运行时统计导出
3. 受控日志辅助解析

要求：

- 能映射到用户、入站、出站、邮箱或 UUID
- 能提取 uplink / downlink
- 能识别单连接或单用户累计值

#### `sing-box`

推荐优先顺序：

1. 实验性或正式管理 API
2. 运行时统计对象
3. 面板托管侧桥接

要求：

- 能关联到入站用户标识
- 能提取会话或用户累计字节

#### `Hysteria 2`

推荐优先顺序：

1. 服务端统计接口
2. 管理命令或控制面
3. 受控事件输出

要求：

- 按认证用户统计
- 区分 `QUIC` 会话累计值

#### `TUIC`

推荐优先顺序：

1. 服务端管理接口
2. 托管运行时指标
3. 受控统计输出

要求：

- 识别单用户会话
- 输出累计上行下行字节

### 16.5 统一归属模型

无论哪个核心，最终都要统一归属到：

- `user_id`
- `subscription_id`
- `service_id`
- `endpoint_id`
- `protocol`
- `core_type`

其中：

- `service_id` 表示服务实例
- `endpoint_id` 表示使用的域名或 IP 入口

### 16.6 入口域名或 IP 列表的归属

服务端一个实例可绑定多个入口：

- 域名
- IPv4
- IPv6

连接建立时，面板生成的配置可以选择具体入口。

因此流量事件建议增加：

- `endpoint_id`
- `endpoint_address`
- `endpoint_type`

这样监控就能统计：

- 某个域名入口流量
- 某个 IP 入口流量
- 哪个入口最常被使用

### 16.7 采集部署模式

当前阶段只支持单节点本机采集，不考虑远程采集，也不保留对应接口。

#### 本机采集

适合：

- panel 与核心同机部署

优点：

- 接口调用简单
- 延迟低
- 归属解析更稳定
- 与单节点配额拒绝状态机更容易保持一致

### 16.8 适配器输出规范

为了保证聚合器稳定，适配器必须遵守：

- 增量事件必须幂等可去重
- 同一会话必须单调递增 `seq`
- 不允许输出负数字节
- 缺少归属信息时先进入隔离队列

## 17. 配额拒绝连接状态机

超量拒绝连接不应只靠一个布尔值，而应设计成明确状态机。

### 17.1 配额状态定义

建议状态：

- `normal`
- `near_limit`
- `over_limit_soft`
- `over_limit_hard`
- `suspended`
- `restored`
- `expired_days`

说明：

- `normal`：未接近限额
- `near_limit`：接近阈值，触发预警
- `over_limit_soft`：超量，拒绝新连接
- `over_limit_hard`：超量且强制断开活跃连接
- `suspended`：管理员手动暂停
- `restored`：续费、重置周期或人工恢复后的过渡状态
- `expired_days`：激活后有效天数到期（`max_active_days` 已过）

### 17.2 状态流转

推荐流转：

```text
normal -> near_limit
near_limit -> over_limit_soft
near_limit -> over_limit_hard
over_limit_soft -> restored
over_limit_hard -> restored
normal -> suspended
near_limit -> suspended
over_limit_soft -> suspended
suspended -> restored
restored -> normal
normal -> expired_days (max_active_days 到期)
near_limit -> expired_days (max_active_days 到期)
expired_days -> restored (管理员续期或重置)
```

### 17.3 状态触发条件

建议：

- 使用率 >= 80%：进入 `near_limit`
- 使用率 >= 100% 且策略为软拒绝：进入 `over_limit_soft`
- 使用率 >= 100% 且策略为硬拒绝：进入 `over_limit_hard`
- 管理员手动停用：进入 `suspended`
- 新周期重置或扩容成功：进入 `restored`

### 17.4 连接处理规则

#### `normal`

- 正常允许新连接
- 正常累计流量

#### `near_limit`

- 允许新连接
- 发送告警
- UI 显示即将超量

#### `over_limit_soft`

- 拒绝所有新连接
- 已建立连接可继续到自然结束
- 订阅和面板中显示已超量

#### `over_limit_hard`

- 拒绝所有新连接
- 尽快断开当前活跃连接
- 适合严格限额产品

#### `suspended`

- 无论剩余额度如何，统一拒绝连接

#### `restored`

- 清除拒绝标记
- 恢复新连接
- 下一个稳定周期回到 `normal`

### 17.5 拒绝决策检查点

必须检查：

1. 用户认证后
2. 会话建立前
3. 增量聚合后
4. 管理员操作后
5. 周期结算或配额重置后

### 17.6 拒绝连接返回语义

由于不同核心协议不同，用户侧报错文本不能完全一致。

统一要求：

- 后端内部记录标准错误码
- 核心侧返回其允许表达的最接近拒绝语义
- 审计日志记录真实拒绝原因

建议内部错误码：

- `QUOTA_NEAR_LIMIT`
- `QUOTA_SOFT_REJECT`
- `QUOTA_HARD_REJECT`
- `QUOTA_SUSPENDED`

### 17.7 状态同步策略

为了避免并发连接导致超量穿透，建议：

- 配额状态缓存在内存或本地 KV
- 每次连接前读快速状态
- 账本聚合后立即刷新状态
- 定时与数据库做强一致校正

### 17.8 和订阅状态的关系

订阅状态和配额状态不要混为一体。

建议：

- 订阅是否可访问由订阅模块控制
- 代理连接是否允许建立由配额状态控制

这样可以避免：

- 订阅仍然能更新，但连接被限额拒绝
- 或订阅已吊销，但配额仍显示正常

## 18. 服务端入口域名或 IP 列表设计

服务端实例应支持配置多个连接入口，并在生成配置或下发节点时可选。

### 18.1 支持的入口类型

- 域名
- IPv4
- IPv6

### 18.2 使用场景

- 同一服务绑定多个域名
- 同时提供域名和裸 IP 入口
- 分地区下发不同入口
- IPv4 / IPv6 分流
- 某入口被封锁时快速切换其他入口

### 18.3 设计原则

- 入口列表属于服务实例，而不是单个用户
- 客户端配置生成时从入口列表中选择目标入口
- 订阅可按策略下发默认入口或多入口候选
- 监控流量要能按入口维度查看

### 18.4 建议数据结构

建议一个服务包含：

```json
{
  "service_id": 3001,
  "name": "hk-vless-main",
  "endpoints": [
    {
      "id": 1,
      "endpoint_type": "domain",
      "address": "a.example.com",
      "server_name": "a.example.com",
      "listen_port": 443,
      "tls_enabled": true,
      "is_default": true
    },
    {
      "id": 2,
      "endpoint_type": "ipv4",
      "address": "203.0.113.10",
      "listen_port": 443,
      "tls_enabled": true,
      "is_default": false
    }
  ]
}
```

### 18.5 配置生成行为

配置生成时建议支持：

- 使用默认入口
- 手动指定入口
- 按平台选择入口
- 按网络栈优先选择 IPv4 或 IPv6

### 18.6 监控展示

监控中心应支持按入口展示：

- 当前连接数
- 计费流量
- 物理带宽
- 错误率
- 最近拒绝连接次数

## 19. 最终实现建议补充

如果下一步进入代码实现，最适合先做：

- `TrafficCollectorAdapter` 统一接口
- `Xray`、`sing-box`、`Hysteria 2`、`TUIC` 四个适配器骨架
- `endpoint_id` 维度的流量事件模型
- 配额状态机服务
- 连接前拒绝检查中间层

这样后续既能按域名或 IP 入口做下发，也能把流量和拒绝控制串起来。
