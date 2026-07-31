# 节点实时在线订阅人（ONLINE_USERS）设计文档

> 状态：已实现（方案 A：Agent 主动上报）
> 关联：限速体系（`ratelimit`）、`OnlineStatsPoller`、`MarkRegistry`、`AgentReport` gRPC 流

---

## 1. 背景与目标

当前面板在「节点详情页」只能看到节点本身在线/离线状态，**看不到正在使用该核心节点的订阅人**。

核心数据其实已经在 Agent 侧存在：`OnlineStatsPoller` 周期性调用 xray 的 `GetUsersStats` 拿到
`email → [IP, last_seen, 上下行字节]`，写入 `MarkRegistry`，但**这套数据只为本地限速（nft/tc 打标）服务，没有回传面板**。

目标：

- 节点详情页**实时**展示「正在使用该核心节点的订阅人」：订阅、IP/设备、入站协议、实时上下行速率、连接时长、限速。
- 与限速**解耦**：即使节点未开启限速，也能看到在线订阅人。
- 复用现有 `MarkRegistry`、`emailToSubID` 映射与 `AgentReport` 推送机制，改动可控。

---

## 2. 现状分析（为什么现在没有）

- Agent 侧 `OnlineStatsPoller`（`internal/runtime/agent/ratelimit/online_stats_poller.go`）已在轮询 xray
  `GetUsersStats`，写进 `MarkRegistry`（`ip → SubscriptionID`）。**数据只用于本地限速**。
- `AgentReport` 当前类型：`STATUS / TRAFFIC / CONFIG / RATE_LIMIT_STATUS / STATUS_SERVERS`，
  **没有 `online_users`**。全代码库搜 `OnlineUsers` = 0 处。
- xray 的 `statsUserOnline: true` 策略**仅在开启限速时**才由 `EnsureOnlineStatsPolicy` 注入；
  没开限速时 xray 不统计在线用户。
- xray 用户邮箱格式 `<baseEmail>_<shortTag>`（`generateInboundEmail`），Agent 已有 `emailToSubID`
  映射可反查到订阅 ID。

---

## 3. 总体架构（方案 A：推送，实时）

```
xray GetUsersStats ──► OnlinePoller(常驻, 与限速解耦)
        │                      │
        │                      ▼
        │            email → subID 映射 + 速率差分(两次采样)
        │                      │
        ▼                      ▼
   AgentReport{ ONLINE_USERS, online_users:[…] }  ──► gRPC stream ──►
        │
        ▼
   Panel: NodeOnlineStore(内存, per serverID, TTL 过期)
        │
        ▼
   GET /api/v1/nodes/{id}/online-users
        │
        ▼
   NodeServerDetail.vue 新「在线订阅人」标签页
   (+ NodeServers.vue 行内徽章 + Dashboard 总卡)
```

数据流：**Agent 主动推送（每 ~15s 可配置）→ 面板内存缓存（TTL 过期防僵尸）→ 前端轮询接口（每 15s）**。

---

## 4. 数据模型（Proto）

文件：`internal/runtime/agent/proto/corefusion_agent.proto`

### 4.1 新增枚举值

```proto
enum ReportType {
  // ... 现有值 ...
  ONLINE_USERS = 6;
}
```

### 4.2 新增消息

```proto
// 单个在线会话（一个 IP + 一个入站 = 一个 session）
message OnlineSession {
  string ip              = 1;  // 单个在线 IP（多设备 = 多 session）
  string inbound_tag     = 2;  // 入站标签，如 vless-in-20296
  string protocol        = 3;  // vless / vmess / trojan / ss ...
  int64  connected_since = 4;  // 本次连接开始时间(unix 秒)
  int64  last_seen       = 5;  // 最后活跃(unix 秒)
  int64  downlink_bytes  = 6;  // 本期累计下行字节
  int64  uplink_bytes    = 7;  // 本期累计上行字节
  int64  downlink_rate   = 8;  // 下行速率 bps（Agent 差分计算）
  int64  uplink_rate     = 9;  // 上行速率 bps（Agent 差分计算）
}

// 一个订阅的在线聚合（含其下所有 session）
message OnlineUserEntry {
  int64  subscription_id = 1;
  string email           = 2;  // baseEmail（已去掉 _shortTag 后缀）
  int64  first_seen      = 3;  // 该订阅本次会话最早连接(unix 秒)
  int64  last_seen       = 4;
  repeated OnlineSession sessions = 5;  // 每个 IP/入站一行
}
```

### 4.3 AgentReport 新增字段

```proto
message AgentReport {
  // ... 现有字段 ...
  repeated OnlineUserEntry online_users = 63;
}
```

> 速率由 Agent 两次采样差分计算（xray 返回的是累计字节，bps 必须差分管）。

---

## 5. Agent 侧改动

### 5.1 在线轮询常驻化

把在线轮询从「仅限速时跑」改为**常驻**：

- 新增 `OnlineUsersPoller`（或复用现有 poller 并去掉限速前置条件），周期默认 15s（可配置）。
- `EnsureOnlineStatsPolicy` 中将 `statsUserOnline: true` 改为**始终注入**（无副作用，仅开启计数）。

### 5.2 速率差分

Agent 维护一个小型「上一次采样」表（key = `email+ip`），保存上次累计字节与采样时间，
每次采样后计算 `rate = (bytes_now - bytes_prev) / (t_now - t_prev)`，得到 bps。

### 5.3 上报

- 新增 `sendReport(ONLINE_USERS)`：遍历 `MarkRegistry` + 当前 `GetUsersStats` 结果，
  按 `subscription_id` 聚合为 `OnlineUserEntry`，填充 `OnlineSession`，序列化成 `AgentReport.online_users`。
- 上报周期与轮询一致（~15s）。

---

## 6. Panel（服务端）改动

### 6.1 内存存储

`NodeOnlineStore`（建议放 `internal/service/nodeserver` 或独立 package）：

```go
type NodeOnlineStore struct {
    mu    sync.RWMutex
    data  map[int64]*NodeOnlineSnapshot // serverID -> snapshot
}

type NodeOnlineSnapshot struct {
    Users     []OnlineUserEntry  // 来自 Agent
    UpdatedAt int64              // 最近刷新 unix 秒
}
```

- 收到 `ONLINE_USERS` 报告即刷新对应 `serverID`。
- **TTL 过期清空**：节点掉线（超过 N 秒未更新）自动清空，避免僵尸数据。

### 6.2 查询接口

- `NodeServerManager.GetOnlineUsers(serverID) ([]OnlineUserEntry, time, ok)`
- `NodeServerManager.GetOnlineCount(serverID) (subscriptions, devices int)` —— 供节点列表徽章 / Dashboard 用。

### 6.3 前端富化

面板侧从 DB 富化（Agent 保持瘦，不持有业务字段）：

- `name`：订阅人名称
- `plan`：套餐名（徽章）
- `limit_up_mbps` / `limit_down_mbps`：限速（来自订阅/套餐配置）

---

## 7. 接口定义

### 7.1 节点在线订阅人

```
GET /api/v1/nodes/{id}/online-users
```

响应：

```json
{
  "online_subscriptions": 12,
  "online_devices": 18,
  "total_uplink_rate": 12345678,
  "total_downlink_rate": 98765432,
  "updated_at": 1764531200,
  "users": [
    {
      "subscription_id": 42,
      "email": "user@example.com",
      "name": "张三",
      "plan": "旗舰包",
      "limit_up_mbps": 50,
      "limit_down_mbps": 50,
      "first_seen": 1764528000,
      "last_seen": 1764531197,
      "sessions": [
        {
          "ip": "1.2.3.4",
          "inbound_tag": "vless-in-20296",
          "protocol": "vless",
          "connected_since": 1764528011,
          "last_seen": 1764531197,
          "downlink_bytes": 4512345678,
          "uplink_bytes": 123456789,
          "downlink_rate": 3123456,
          "uplink_rate": 812345
        }
      ]
    }
  ]
}
```

### 7.2 节点列表徽章（复用）

`NodeServers.vue` 每行加「在线订阅 N」徽章，读 `GetOnlineCount(serverID)`，不另起请求。

### 7.3 Dashboard 总卡（复用）

Dashboard 加「当前在线订阅人」总卡，汇总所有节点 `online_subscriptions`。

---

## 8. 前端 UI（细化）

### 8.1 节点详情页 `NodeServerDetail.vue` —— 新增「在线订阅人」标签页

**顶部汇总卡**：在线订阅数 · 在线设备/IP 数 · 实时总上行 · 实时总下行（带刷新指示）

**主表（按订阅聚合，可展开）**：

| 订阅 | 设备/IP | 入站/协议 | 上行 | 下行 | 时长 | 最近活跃 | 限速 |
|---|---|---|---|---|---|---|---|
| 邮箱+名称+套餐徽章(可点跳转) | 3 | vless/trojan chips | 1.2 MB/s (累计) | 5.4 MB/s (累计) | 02:13:40 | 3s前 | ↑↓ 50Mbps |

**点订阅行展开** → 该订阅每个 IP 明细：

| IP | 归属地(可选) | 入站 | 协议 | 上行速率 | 下行速率 | 累计↑↓ | 连接时长 |
|---|---|---|---|---|---|---|---|
| 1.2.3.4 | 🇯🇵 JP | vless-in-20296 | vless | 0.8 MB/s | 3.1 MB/s | 1.2G/4.5G | 01:02:11 |

**交互**：

- 搜索框（邮箱 / IP）
- 按速率 / 时长排序
- 15s 自动刷新（带倒计时 + 手动刷新按钮）
- 速率列自动 KB/s·MB/s 单位换算
- 行可点订阅跳转到订阅详情页

> 归属地依赖 IP 库（geoip / 纯真等），为可选项，先留字段，不强制实现。

---

## 9. 关键坑位

1. **`statsUserOnline` 必须始终开启**，否则 xray 不返回在线数据（方案 A、B 都要改）。
2. **多入站同订阅会生成多个 email**（不同 shortTag），面板端要**按 `subscription_id` 聚合**，IP 合并展示。
3. **上报频率权衡**：太快增加 gRPC 流量，太慢不「实时」，建议 15s 且可配置。
4. **节点掉线用 TTL 清空**在线列表，避免显示僵尸数据。
5. **速率差分**需 Agent 保存上次采样；首帧速率可为 0。

---

## 10. 工作量与落地步骤

| 步骤 | 内容 | 规模 |
|---|---|---|
| 1 | proto 新增 `ONLINE_USERS` 枚举 + `OnlineSession` / `OnlineUserEntry` 消息 + `AgentReport.online_users` 字段，protoc 重新生成 | 小 |
| 2 | Agent：`OnlineUsersPoller` 常驻 + 速率差分 + `sendReport(ONLINE_USERS)`；`EnsureOnlineStatsPolicy` 始终注入 `statsUserOnline` | ~120 行 |
| 3 | Panel：`NodeOnlineStore` 内存存储 + TTL + `GetOnlineUsers/GetOnlineCount` | ~150 行 |
| 4 | 接口：`GET /api/v1/nodes/{id}/online-users` + 路由 + handler | ~80 行 |
| 5 | 前端：`NodeServerDetail.vue` 标签页 + `NodeServers.vue` 徽章 + Dashboard 卡 | ~200 行 |
| 6 | 联调与单测 | — |

合计约半天量级。优先级：步骤 1→2→3→4→5。

---

## 11. 已实现增强（在线列表富化 + 列表徽章 + Dashboard 总卡）

在核心方案 A 落地后，补充了以下增强（均已实现、后端 `go build ./...` 通过、前端 0 lint 错误）：

### 11.1 在线列表富化（订阅 → 客户端 → 套餐 联表）
- Panel `service/node.GetOnlineUsers` 中按上报的 `subscription_id` 联表：
  `Subscription → Client（Nickname/Email/Remark、SpeedLimitMbps/SpeedLimitUpMbps）→ PlanTemplate（Name）`。
- 经 `SubscriptionRepository/ClientRepository/PlanTemplateRepository` 的批量 `FindByIDs` 一次性拉取，避免 N+1；查询失败静默跳过（保留原始 email 展示）。
- 富化写入 proto `OnlineUserEntry` 的扩展字段 `client_name`、`plan_name`、`speed_limit_down_mbps`、`speed_limit_up_mbps`（proto 中定义，向后兼容，Agent 侧留空）。
- 前端 `NodeServerDetail.vue` 在线表格新增「客户端 / 套餐」列与「限速」列。

### 11.2 节点列表徽章（批量接口，避免 N+1）
- 新增批量汇总接口 `GET /api/v1/node-servers/online-summary`：`NodeOnlineStore.GetAllSummary` 一次性返回所有未过期节点的 `online_subscriptions / online_devices`，响应含 `nodes`（serverID→汇总）与全局 `total_subscriptions / total_devices`。
- `NodeServers.vue` 每行新增「在线订阅」徽章，读 `onlineSummary[row.id]`，随列表刷新（30s）自动更新，不额外轮询。

### 11.3 Dashboard 总卡
- `Dashboard.vue` 顶部汇总区新增「实时在线订阅人」总卡，读 `online-summary` 的 `total_subscriptions / total_devices`，随 Dashboard 每 8s 刷新实时更新。

### 11.4 与文档 7.x 的差异
- 文档 7.1 示例的 `name/plan/limit_*` 字段，实际命名为 `client_name/plan_name/speed_limit_*_mbps`，字段定义在 proto `OnlineUserEntry` 上。
- 文档 7.2/7.3 设想复用 `GetOnlineCount`，实际改为独立的 `online-summary` 批量接口，避免逐节点轮询（N+1），更契合性能预期。

---

## 12. 全局在线视图（跨节点总表）

> 背景：初版在线订阅人只能在「单个节点详情页」查看，节点一多就要逐个进入，无法一眼看到"全集群当前谁在线"。因此补充**全局在线视图**。

### 12.1 设计取舍
- **全局明细接口（非 N+1）**：新增 `GET /api/v1/node-servers/online-users`，由 `NodeOnlineStore.GetAllDetail()` 一次性取所有未过期节点快照，Panel 合并为一张表返回。与 `online-summary`（仅计数）不同，本接口返回**每行明细**（含节点名 + 富化字段）。
- **节点名联表**：行内携带 `node_name`（`NodeRepository.GetByID` 带缓存），前端可直接展示/筛选/跳转。
- **复用富化**：全局视图复用 `enrichOnlineUsers`，对合并后的所有 user 统一联表填充客户端/套餐/限速，避免逐节点重复查询。
- **排序**：默认按 `last_seen` 倒序（最近活跃在前），便于运营关注当前活跃用户。

### 12.2 前端
- 新增一级页面 `OnlineUsers.vue`（菜单「在线用户」，路由 `/online-users`）：
  - 顶部汇总卡（在线订阅 / 在线设备 / 在线节点数）；
  - 主表：节点（可点击进节点详情）· 客户端（含邮箱）· 套餐 · 设备数 · 上下行速率（可排序）· 累计流量 · 限速 · 最近活跃 · 时长；行可展开各 IP 明细；
  - 节点下拉筛选 + 关键字搜索（邮箱/客户端/节点/IP，前端过滤）；15s 自动刷新 + 手动刷新。
- `Dashboard.vue`「实时在线订阅人」总卡支持**点击直达** `/online-users`。

### 12.3 与单节点视图的关系
- **单节点详情页**：聚焦"该节点"在线明细与排查；
- **全局在线用户页**：聚焦"全集群"在线总览与快速定位用户所在节点；
- 两者数据同源（同一 `NodeOnlineStore`），入口互补，不重复建设。

### 12.4 其他已做的健壮性优化
- **Agent 空列表也上报**：无在线用户时同样发送 `ONLINE_USERS`，让面板能区分"节点未上报"与"节点 0 人在线"。
- **reporter 与限速彻底解耦**：`startOnlineUsersReporter` 独立启动，不再受 `initRateLimitFromDB` 中"限速未开启/未初始化"提前 return 的影响。
- **空列表序列化为 `[]`**：`users` 字段空时返回 `[]` 而非 `null`，前端无需额外判空。
