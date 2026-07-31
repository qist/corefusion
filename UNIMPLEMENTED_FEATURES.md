# CoreFusion 未实现功能审计报告（修订版）

> 审计时间：2026-07-30（初版后经逐项二次核实修订）
> 审计范围：后端（`cmd/`、`internal/`、`pkg/`）、前端（`web/admin/`）、全部设计文档（根目录 9 篇 + `docs/` 22 篇）
> 修订原则：代码与设计文档存在出入（代码改了文档没跟），凡「代码已用其他方式实现等价功能」「与其他已实现功能重复」「客观上无法实现」的项一律从缺失清单剔除或归入"文档过时"章节，本报告只保留**真实缺口**。

---

## 一、总体结论（修订后）

初版报告中的部分"缺失"经核实为**文档过时或误判**，实际情况：

| 类别 | 数量 | 说明 |
|---|---|---|
| 真实缺失 | 2 项 | 见第二章（2.1–2.7、2.9–2.11 已于 2026-07-30 实现；剩余 2.8 面板自更新[后期]、2.12 初始化向导[最后]） |
| 文档过时（代码已有等价实现） | 8 项 | 见第三章，应从设计文档/placeholder 中清理 |
| 死代码/代码卫生 | 10 项 | 见第四章 |
| 前端问题 | 6 项 | 见第五章 |

关键修正：**流量子系统并非"整体不生效"**。用量统计已生效（trafficsync + TrafficEventConsumer → `subscriptions.*_bytes_cache`），限速/设备数/并发限制已由 ratelimit 模块（nftables+tc）实际执行。真正缺的只是"订阅级流量超额自动封禁"和"非 xray 协议流量统计"等三块（见 2.1、2.2）。

---

## 二、真实缺失（按优先级）

### P1 — 核心功能闭环

#### 2.1 订阅级流量超额/自然过期的自动处理（**已实现 2026-07-30**）

- **用量统计已生效**（不计缺失）：本机走 `service/trafficsync`（xray `QueryStats reset_=true` 轮询），远程走 Agent heartbeat → `traffic_consumer.go:105`，两条路径都汇聚到 `inboundconfig.Service.UpdateTraffic`，原子累加 `subscriptions.used_bytes_cache`（`inboundconfig/service.go:193-199`）
- **实现要点**（新增）：
  - `store/repository/subscription.go` 新增 `ListQuotaExhausted`（活跃订阅中 `quota_bytes>0 && used_bytes_cache >= quota_bytes` 且非终态）与 `ListExpired`（活跃订阅中 `expires_at` 已过）
  - `service/subscription.Service` 新增 `EnforceQuotaAndExpiry`：超额时 `quota_state=suspended`+`Status=suspended`（若 `SuspendOnQuotaExhausted=1`，否则 `over_limit_hard`），自然过期时 `Status=expired`+`quota_state=expired_days`；两者均经 `UserSyncer.SyncUserToServers(remove)` 从核心移除用户
  - `Restore` 现会在恢复订阅时把被移除的用户经 `userSyncer` 重新加回核心（`add`）
  - `StartEnforcer(ctx, interval)` 用 `sync.Once` 起定时扫描（默认 5 分钟，启动即跑一次），在 `router.NewWithFullConfig` 注入 `sharedUserSyncer` 后启动
- （对照：节点级流量配额封禁已实现，`nodeserver/traffic_quota.go:210-300` 超额会 suspendNode）
- 已知隐患（未解决，留待 2.2 的 IP 计费接线）：`UpdateTraffic` 只有在某 inbound config 的 JSON 里匹配到 email 才更新订阅缓存（`inboundconfig/service.go:157`），用户不在任何 inbound 中时缓存不更新

#### 2.2 非 xray 协议流量统计（**已实现 2026-07-30**）

> **原则（用户明确）**：xray 协议的流量**走 xray 自己的统计**（`inboundconfig.UpdateTraffic` 按 email 累加 `used_bytes_cache`）；**只有不被 xray 统计的订阅才走 nft 网络层计费**，两者不重复计数。

- **实现要点**（新增/接线）：
  - `internal/service/traffic/ip_tracker.go`：`SubscriptionIPTracker` 现已实例化并接线
    - 修复 `readIPCounter`：按源 IP 精确匹配 nft rule 的 counter（原实现把整 chain 所有 counter 求和，多 IP 订阅流量会被放大 N 倍）
    - 订阅访问时调用 `RecordAccess` 建 `sub_<id>` 链 + counter 规则；`SetEventChannel` 接 `NetworkTrafficBridge`
  - 新增 `internal/service/traffic/bridge.go`：`NetworkTrafficBridge` 消费 `TrafficDeltaEvent`（nft **累计值**，按 sub 算增量）；对每个订阅先做 `isXrayAccounted` 判定（其 client 的 email 是否配置在任一 `CoreType=xray` 的 inbound config 中），**是则跳过（xray 已统计），否则** `subRepo.AddUsedBytes` 累加增量
  - `store/repository/subscription.go` 新增 `AddUsedBytes(ctx, subID, delta)`（原子 `used_bytes_cache + delta`）
  - `router.go`：`initNetworkTrafficAccounting` 包级单例创建 tracker 并 `go bridge.Run(ctx, 30s)`（`sync.Once` 防重），在 admin 与 portal 两条 `NewPublicHandler` 路径注入 `SetNetworkTracker`；受 `CF_NETWORK_TRAFFIC_ACCOUNTING=0` 关闭、`CF_NETWORK_TRAFFIC_TABLE` 配置表名（默认 `cf_traffic`）
- 说明：nft 计的是上下行合计、无法拆分，故只更新 `used_bytes_cache`（与 xray 拆分 up/down 不同，但总量口径一致）；`used_bytes_cache` 仅被 xray 或 nft 之一累加，不重复

#### 2.3 Agent 端 SpeedTest（**已实现 2026-07-30**）

- **路径**：`SpeedTestManager.testRemoteServer`（`runtime/agent/speedtest.go`）经 `nodeMgr.SpeedTestOnServer`（manager.go:872）通过 **Stream** 下发 `PanelCommand_SpeedTest`，落到 `cmd/agent/command_handler.go` 的 `case *agentpb.PanelCommand_SpeedTest`
- **实现要点**：该 case 原固定返回 `not implemented`，现改为调用新增的 `handleSpeedTest`：
  - 用 `net.Dialer` 对 `SpeedTestRequest.{target_host, target_port, timeout_ms, samples}` 做 TCP 连接探测（默认 443、3 次采样、跳过首次 warmup）
  - 计算平均延迟、抖动（延迟标准差）、丢包率，返回 `SpeedTestResponse{ success, latency_ms, jitter_ms, packet_loss }`
  - 带宽字段（download_bps/upload_bps）留 0（轻量连接探测，非吞吐测试）
- 说明：unary gRPC `SpeedTest` RPC（proto:41）仍由 `UnimplementedAgentServiceServer` 默认占位（manager 走 stream 路径未使用），不影响功能；远程节点延迟测速/按延迟排序现已可用

#### 2.4 访问控制管理面（**已实现 2026-07-30**）

- **实现要点**（新增/接线）：
  - `internal/service/access/repository.go`：`gormBanRepository` 实现 `BanRepository`（Create/FindActive/DeleteExpired，操作 `service_ip_bans`）；新增 `PolicyRepository` 接口 + `gormPolicyRepository`（按 service/endpoint 加载启用策略）
  - `service/access.Service` 增加 `policyRepo` 字段，`NewService(banRepo, policyRepo)`；`CheckConnection` 现加载策略并应用并发限制（`ConcurrentTracker.TryConnect`）、速率限制（`RateLimitManager.Check`）与自动封禁（`AutoBanManager.OnRateLimitHit`），拒绝返回 `ErrAccessDeniedByRate`（新增于 `networkpolicy`）
  - `internal/api/handlers/services/access_handler.go`：新增 `AccessHandler`，实现 `access-policies`（list/create/update/delete）与 `ip-bans`（list/create/delete）共 7 个真实端点
  - `router.go`：`getAccessService` 包级单例实例化 `access.NewService(NewBanRepository(db), NewPolicyRepository(db))`，并将 7 个 placeholder 路由替换为 `AccessHandler` 方法
- 说明：`access.Service` 已实例化并暴露（供后续在入站连接生命周期调用 `CheckConnection`）；当前尚无连接前调用钩子，管理面与策略评估逻辑均已可用

#### 2.5 Notifier 模板通知（**已实现 2026-07-30**）

- **实现要点**（新增/接线）：
  - `internal/store/repository/notification_template.go` 新增 `GetByName`，按名查询模板
  - `internal/service/system/notifier_singleton.go`（新增）：包级 `RegisterNotifier(notifier, tmplRepo)` + `NotifyTemplate(ctx, name, vars)` 便捷入口——按名查启用模板并发送，未配置/无启用模板时静默 no-op，供业务事件 fire-and-forget 调用
  - `router.go`：创建 `notifier` 后 `system.RegisterNotifier(notifier, repository.NewNotificationTemplateRepository(database))`，移除 `_ = notifier` 丢弃
  - 业务接入：`clientauth/commerce.go` 的 `CreateOrder` 余额支付完成时异步调用 `system.NotifyTemplate(ctx, "order_paid", {order_id, amount, email, plan_name})`
- 说明：发送引擎已闭环（模板 CRUD → 引擎 → 业务触发）。工单、证书到期等其余场景可复用同一 `system.NotifyTemplate` 入口；设置页"测试 SMTP/Telegram"走 `settingSvc.TestSMTP/TestTelegram` 独立路径不变

#### 2.6 Agent 节点配置快照（数据本地化）（**已实现 2026-07-30**）

- 来源：`docs/design/node-server-architecture.md` 阶段3
- **实现要点**（新增/接线）：
  - `internal/model/entity/config_snapshot.go`（新增）：`ConfigSnapshot{ServerID, CoreType, Version, Config, CreatedAt}`，表 `config_snapshots`
  - `runtime/agent/local.go`：`LocalAgent` 增加 `db`/`serverID` 字段（`WithDB`/`WithServerID` 注入）；`SyncConfig` 写入运行时配置成功后同步 `Create` 一条快照记录；新增 `RestoreFromSnapshot(ctx)`——启动时按核心类型（xray/singbox/hysteria2/tuic）各取最新快照经 `procMgr.WriteRuntimeConfig` 恢复
  - `cmd/agent/app.go`：`NewLocalAgent` 后注入 `WithDB(db.DB)`/`WithServerID(agentServerID)` 并调用 `RestoreFromSnapshot`；`cmd/agent/migrate.go` AutoMigrate 增加 `&entity.ConfigSnapshot{}`
- 效果：面板失联/宕机时，远程 Agent 重启后可从本地 `agent.db` 快照恢复上次下发的配置，节点自治

### P2 — 功能补齐

#### 2.7 安全审计事件写入逻辑（**已实现 2026-07-30**）

- 修正：表**已建**（`entity/access.go:58-73` 已 AutoMigrate 注册），过期清理逻辑也有（`log_cleanup.go:103`）
- **实现要点**：
  - `internal/service/audit/audit.go`（新增）：包级单例 `audit.Register(db)` + `audit.Record(Event)`（异步 fire-and-forget 写 `security_audit_events`）；router 启动时注册
  - 写入点：管理端登录成功/失败（`handlers/auth`，`admin_login`）、门户登录成功/失败（`handlers/clientauth`，`client_login`）、访问控制拒绝（`service/access`：黑名单/封禁/速率/并发 → `connection_denied`/`rate_limit_exceeded`/`concurrent_limit_exceeded`）、自动封禁触发（`ip_auto_banned`）、管理员手动封禁/解封（`ip_ban_created`/`ip_ban_deleted`）
  - `handlers/security/handler.go` 桩全部做实：审计事件分页查询（category/event_type/result/remote_ip 过滤）、告警分页查询（status/severity/alert_type 过滤）、告警渠道 CRUD（含新增 DELETE 路由）、告警 resolve

#### 2.8 面板自身更新/回滚缺失（**后期实现**）

- `handlers/system/handler.go:360-374` SystemUpdate/SystemRollback/ListReleases/ListUpdateJobs 全是桩
- 指**面板自身**更新（节点 Agent 更新是另一套已实现机制）；仓库无部署/升级脚本，前端无调用入口
- **排期：后期实现**（暂不作为近期缺口处理，但实现前需先解决分发基础设施——构建产物源，可复用 Agent 自更新机制）

#### 2.9 试用套餐自动转正/过期机制（**已实现 2026-07-30**）

- 来源：`docs/design/plan-design.md`
- **实现要点**：
  - `internal/service/plan/trial.go`（新增）：`StartTrialScheduler`（router 启动，每小时扫描，sync.Once）；`ProcessTrialExpiry` 查找 `is_trial` 模板下 `expires_at` 已过期且 active 的客户端——`findTrialPurchaseVariant` 查最近一笔已完成 new/renew/upgrade 订单（to_variant 须为非试用模板 plan 规格），有则 `convertTrialToVariant` 转正为**用户实际购买的套餐**（创建零金额 `trial_convert` 订单，更新客户端配额/到期/限速并同步订阅），否则 `expireTrial`（客户端+订阅置 expired）
  - `clientauth/commerce.go`：`activatePlan` 新购试用套餐时到期时间按 `trial_duration_hours` 计算（而非规格天数）；`CreateOrder` 增加"试用套餐每客户端仅可领取一次"校验（历史订单关联 is_trial 模板则拒绝）；试用期内客户端可直接以 `order_type=new` 购买非试用套餐，购买即替换试用（到期从购买日重算，不叠加试用剩余天数）
  - **2026-07-30 修订**：废弃并移除 `trial_auto_convert_template_id`（写死转正模板存在零金额白送付费套餐问题），实体/服务输入/前端表单及文案均已删除该字段；历史库中该列可保留，代码不再读写

#### 2.10 Runtime 配置工作流（**已实现 2026-07-30**）

- **实现要点**：
  - `internal/service/runtimeapply/service.go`（新增）：基于 configsync 管线的运行时工作流——`CreateSnapshot`（抓取 `<dataDir>/runtime/*.jsonc` 存 `runtime_snapshots`，含 sha256 校验和）、`DryRun`（启用入站端口冲突 + config JSON 合法性校验）、`Apply`（先 pre_apply 快照 → 建 `runtime_apply_jobs`/targets/events → `SyncNow` 立即下发）、`Rollback`（先 pre_rollback 快照 → 从快照恢复运行时文件，经 `process.Manager.WriteRuntimeConfig`）、`DisableProxy`（紧急停用：xray/singbox 写空入站配置，`/runtime/apply` 恢复）、`RetryJob`/`CancelJob`/`ListJobs`/`GetJob`（含 targets+events）/`Overview`
  - `internal/api/handlers/runtimecfg/handler.go`（新增）：11 个 `/runtime/*` placeholder 路由全部做实
  - 注：DB 仍是配置源头，回滚为应急手段（周期 configsync 会按 DB 重新生成，事件中已记录警示）

##### 2.10.1 操作按节点定位（**已实现 2026-07-31**）

> 来源：运维反馈——创建快照/预检/应用配置/回滚/紧急停用代理原先无法指定或识别目标节点，无法判断"针对哪个节点"。

- **实现要点**（在 2.10 工作流之上扩展，不改动既有管线）：
  - `serverIDs []int64` 作为通用节点过滤参数贯穿 `runtimeapply` 全部方法（`CreateSnapshot`/`DryRun`/`Apply`/`Rollback`/`DisableProxy`/`RetryJob`/`VerifySelected`/`runVerification`）；**空切片 = 全部节点**
  - `RuntimeApplyJob` 与 `RuntimeSnapshot` 实体新增 `TargetServerIDsJSON string` 列（AutoMigrate 自动建列），持久化本次操作的目标节点 ID，供列表/详情展示
  - `configsync.Service` 新增 `SyncServers(ctx, only []int64)`（仅同步指定节点集合，原 `SyncNow` 全量仍保留）；`sync`/`syncCore` 在 `len(only)>0` 时按 `idSet` 过滤
  - `VerifySelected(ctx, serverIDs)` 取代原全量 `VerifyNodes`；新增 `POST /api/v1/runtime/verify`（body `{server_ids:[...]}`），按选中节点在线状态逐个校验
  - 前端 `web/admin/src/views/RuntimeConfig.vue`：操作页顶部新增共享 `opNodes` 多选器；创建快照/应用/回滚/预检/停用代理均带 `server_ids`；快照列表与任务详情新增「目标节点」列，按 `server_id → 节点名 (#ID)` 解析展示（未选则显示"全部节点"）；每个操作卡片内实时显示"目标节点：节点A、节点B / 全部节点"
- 说明：快照本身抓取的是面板**全局**运行时配置文件（`dataDir/runtime/*.jsonc`），"目标节点"记录的是本次操作预期下发的节点范围（配合 `SyncServers` 只向选中节点下发/验证）；回滚/应用会据此范围同步，避免误触全部节点。

#### 2.11 xray dokodemo-door / TUN 入站配置生成（**已实现 2026-07-30**）

- **实现要点**：
  - `domain/common.go`：新增 `ProtocolDokodemo`（"dokodemo-door"）与 `ProtocolTUN`（"tun"）常量；`configgen/validate/validator.go` xray 支持列表加入两者
  - `configgen/xray/generator.go`：`InboundSettings` 扩展 dokodemo 字段（`address`/`port`/`network`/`followRedirect`/`userLevel`/`portMap`）与 tun 字段（`name`/`mtu`/`gateway`/`dns`/`autoSystemRoutingTable`/`autoOutboundsInterface`）；`switch spec.Protocol` 补齐两个分支，字段从 `spec.Extra` 提取（兼容前端 `rewriteAddress`/`allowedNetwork` 等命名与 snake_case 别名），并新增 `extraString/extraInt/extraBool/extraStrings` 辅助函数
  - 与前端 `XrayInbounds.vue` 的 settings 字段命名一致（dokodemo：allowedNetwork/rewriteAddress/rewritePort/followRedirect/portMap；tun：name/mtu/gateway/dns/autoSystemRoutingTable/autoOutboundsInterface）

### P3 — 规划类/低优先级

| 项 | 说明 |
|---|---|
| Client merge-profiles / route-profiles（22 条 placeholder）✅ 路由已清理 2026-07-30 | 多订阅源合并、客户端路由规则集 profile，完全新概念，无实体无实现，前端也无调用 |
| Server Share profiles（12 条 placeholder）✅ 路由已清理 2026-07-30（实际挂载于 `/server-share/profiles/*`；`ServerShareProfile` 实体 schema 保留） | 比已实现的 share-domains 更高层：发布/撤销/版本历史/预览/二维码/导出，部分重叠（见 3.4） |
| Routing Regions CRUD（4 条 placeholder）✅ 路由已清理 2026-07-30（保留 `routing_regions` 空表） | `routing_regions` 表已建（`entity/runtime.go:168-180`）无实现；与 geo-rules/region_selector 用途不同（见 3.5） |
| `client-auth/traffic-history` | ✅ **后端已实现 2026-07-31**：`GET /api/v1/client-auth/traffic-history`（受 client-auth 鉴权，可选 `?months=N` 1-24 默认 6），按当前登录用户订阅的 `BillingNodeIDs` 聚合 `node_traffic_usage`，按月返回流量趋势 `[{period_start, rx_bytes, tx_bytes, total_bytes}]`；前端门户尚未调用（仍为未来扩展） |
| Metrics `ConnectionFailures`/`UDPQUICMetrics` | 经代码核查，当前代码中**不存在**这两个端点（仅本文档自引用）。实际 metrics 路由为 `metricsGroup.GET` 的 `/overview`、`/services`、`/nodes`、`/certificates`（`router.go:1845-1848`）与 `node-servers` 的 `/:id/metrics-history`、`/metrics/aggregated`（`router.go:1514-1515`）；前端仅调用 `/node-servers/:id/metrics-history`，其余 metrics 端点前端无调用，属低优先补充指标，**非死端点** |
| 安全能力：2FA/RBAC/管理端 mTLS/CLI 恢复命令/`corefusion client` 模式/`web/client` 前端 | `CODE_STRUCTURE_SECURITY.md` 的建议项，均未落地 |

#### 2.12 首次启动初始化向导（**暂缓开发**）

> 原 P0 安全相关项，经评估当前不急于开发，下调至 P3 暂缓。

- 来源：`docs/design/first-boot-initialization.md`（文档自述"状态：设计阶段"）
- 现状：全库无 `/api/v1/init/*` 路由、无 `IsFirstBoot`、无 `reset-init` 命令、无初始化前端页面；`cmd/corefusion/main.go:330` 硬编码 `CreateInitialAdmin("admin","admin123")`
- 缺失内容：随机初始化端口/Token、五步配置向导（端口/管理员/访问路径/HTTPS）、初始化前端
- 关联：`handlers/system/handler.go` 的 `GetBootstrapState`/`FinishBootstrap`（L350-358）是无前端调用的死端点

> **明确不实现项（已排除）**：**CSRF 防护**——系统使用 Bearer Token 鉴权、无 Cookie 会话，CSRF 无攻击面，**决定不实现**，从缺失清单移除。
> **gRPC `RegistrationService.Register`**——节点注册已实现并走 REST 公开端点 `v1.POST("/agent/bootstrap")`（用 bootstrap token 下发证书与 agent 二进制/配置，见 `router.go:1989`），gRPC `Register` 仅返回"未接线"响应，**决定不实现**，从 P3 缺失清单移除。

> **客户端设计（服务端不实现，从缺失清单剔除）**：
> `router.go` 中 `/route-runtime/openwrt/*`（7 条 placeholder）以及 `domain/common.go:141-142` 的 `Dnsmasq`/`Fw4` 枚举常量，描述的是 **OpenWrt 客户端设备端**的能力（fw4 防火墙、dnsmasq、OpenWrt 集成下发），属于客户端（`web/client` / `corefusion client` 模式）范畴，**服务端明确不实现**。
> 因此这些路由不应作为"服务端未实现功能"计入；**已于 2026-07-30 删除**服务端的 7 条 `/route-runtime/openwrt/*` placeholder 路由（避免造成"服务端缺失"的误判）；
> - 相关 dnsmasq/fw4 枚举常量保留，归入后续客户端设计文档，不计入本服务端审计报告。

---

## 三、文档过时项（代码已有等价实现，不计缺失）

以下各项是"设计文档/placeholder 未跟上代码演进"造成的假象，**建议从设计文档标注过时、删除对应 placeholder 路由**：

| # | 项 | 实际等价实现 | 证据 |
|---|---|---|---|
| 3.1 | 证书部署 Deploy() 桩 | 证书签发/续期/上传后自动分发：`fireCertUpdated` → `DistributeCertificateToAll` → Agent 磁盘 + ReloadCore；反代证书随 SyncProxyConfig 携带；断线重连补发 | `certificates/handler.go:203,239,305`、`router.go:1249-1254`、`cert_distributor.go:36,165` |
| 3.2 | `/reverse-proxies` 6 条 placeholder ✅ 路由已清理 2026-07-30 | 反代管理已完整实现：`/node-servers/:id/proxy/*`（通用）、`/caddy/*`、`/nginx/*` 全套 CRUD/reload/配置查看，前端 CaddyProxy/NginxProxy 完备 | `router.go:1504-1565`、`system/caddy_handler.go`、`nginx_handler.go`、`proxy.go` |
| 3.3 | 流量聚合器被注释（"aggregator 死代码致配额不生效"） | 用量统计已改道：trafficsync（本机）+ TrafficEventConsumer（远程）→ `inboundconfig.UpdateTraffic` → `subscriptions.*_bytes_cache`；`TrafficUsageLedger/Summary` 是被废弃的旧设计 | `trafficsync/service.go:222`、`traffic_consumer.go:105`、`inboundconfig/service.go:133-208` |
| 3.4 | Server Share 的"分享域名"能力 | `/share-domains` + `/node-servers/:id/share-domains` 全套 CRUD 已实现，前端 ShareDomainTab 在用；placeholder 多出的只是发布/版本/二维码等增强 | `router.go:937-945,1635-1639`、`sharedomainconfig/handler.go` |
| 3.5 | 区域能力的两个相近实现 | geo-rules（GeoIP 数据文件管理，`/node-servers/:id/geo-rules`）+ region_selector（按用户 IP 推荐区域）均已在用；`/routing/regions` CRUD 是第三种用途（自定义区域定义），表已建但需求需重新确认 | `router.go:1608-1624`、`georule/service.go`、`region_selector.go` |
| 3.6 | `adapters/` 四个流量采集器"空实现/近似实现" | 该包整体**无人 import**，是未完成即被废弃的平行新架构；实际采集由 trafficsync（xray）承担 | `runtime/adapters/*` 无引用；`trafficsync/service.go` |
| 3.7 | proto `AgentService` 20 个 RPC "未注册未实现" | 命令全部走 Heartbeat 双向 stream 的 PanelCommand（`SendCommandAndWait` 20+ 处调用），proto 的 AgentService 是消除反向拨号后的死定义 | `eliminate-reverse-dial.md`、`manager.go:1023` |
| 3.8 | main.go:301 RegionSelector 创建后丢弃 | `router.go:1307-1311` 已正确接线到 `nodeserver.Service.WithRegionSelector`，功能正常；main.go 那行是冗余 | `router.go:1307`、`nodeserver/service.go:899` |
| 3.9 | CreateConfigBackup 写空文件 | "完整备份"已打包整个 dataDir（含全部配置，仅排除 backup/rules_tmp/cache/logs）+ DB dump，配置不会丢；config 单项端点是冗余桩 | `backup/service.go:44-86` |

---

## 四、死代码 / 代码卫生（2026-07-30 已清理）

> 本章原列的死代码已执行清理。**保留项**（`domain/runtime/interfaces.go`、`entity/runtime.go` 的快照/区域表、`ip_tracker.go`）是 2.10 Runtime 工作流 / 2.2 网络层计费所需的脚手架，删除会与规划功能冲突，故保留。

| 项 | 位置 | 处理结果 |
|---|---|---|
| `service/quota` 包整体 | `quota/quota.go`、`enforcer.go` | ✅ **已删除**（无调用方，2.1 将走 ratelimit 路径） |
| `traffic/aggregator.go`、`multi_collector.go` | `service/traffic/` | ✅ **已删除**（被 main.go 停用、无外部引用）；`ip_tracker.go` 保留（2.2 所需） |
| 空目录模块 | `internal/observer/`、`scheduler/`、`region/`、`platform/`、`api/handlers/networkpolicy/`、`service/networkpolicy/` | ✅ **已删除**（确认无文件） |
| proto AgentService 死定义 | `runtime/agent/proto/corefusion_agent.proto:29-71` | ⏸️ **未删**：与 HeartbeatService 同文件生成，删除需重新生成 pb.go，留待手动处理 |
| 孤儿前端文件 | `views/Subscriptions.vue`、`SubscriptionEndpoints.vue`、`SubscriptionFormats.vue`、`Nodes.vue`、`services/ProcessTab.vue` | ✅ **已删除 5 个**（被 SubscriptionCenter/CoreProcessTab/NodeServers 取代） |
| ~~`ProxyManagement.vue`~~ | `views/ProxyManagement.vue` | ⚠️ **更正**：经核实 `NodeServerDetail.vue:265` 实际 import 了它，并非孤儿，**已保留** |
| Legacy portal API | `api/portal.js:33-56` 的 6 个函数 | ✅ **已删除**（仅文件内部引用，无调用方） |
| main.go 冗余初始化 | `main.go:300-302` | ✅ **已删除**（`_ = subscription.NewRegionSelector(...)` 及 `geoIPResolver` 变量），并移除多余 import |
| Metrics 2 个死端点 | `metrics/handler.go:102-111` + `router.go:1772-1773` | ✅ **已删除** handler 方法与路由注册 |
| 废弃 domain 接口 | `domain/runtime/interfaces.go` | ⏸️ **保留**：2.10 RuntimeApplyService 需要 |
| `routing_regions`/`runtime_*` 空骨架表 | `entity/runtime.go` | ⏸️ **保留**：2.10 Runtime 工作流 / 路由区域需要 |
| P3 / 第3章 死 placeholder 路由 | `router.go` `registerPlaceholderRoutes` | ✅ **已删除 52 条**：`/client/merge-profiles`(11)+`/client/route-profiles`(11)、`/routing/regions`(4)、`/route-runtime/openwrt/*`(7)、`/reverse-proxies`(6)、`/server-share/profiles`(12)、`/services/:id/logs`(1)；`/traffic/*`(8) 属"保留项"未动 |

> 清理后已执行 `go build ./...` 通过。前端孤儿文件删除已核实无 import 引用。

---

## 五、前端问题（真实存在）

| # | 问题 | 位置 | 说明 |
|---|---|---|---|
| 5.1 | Dashboard 核心流量表四列硬编码为 0 | `Dashboard.vue:279-284` | 数据**部分现成**：`GET /node-servers/:id/xray/stats` 返回按 inbound tag 的累计 uplink/downlink（可聚合到核心维度）；实时速率无现成 API，需轮询取差值 |
| 5.2 | Dashboard 核心数卡片硬编码为 4 | `Dashboard.vue:211,273-275` | 数据现成：`fetchCoreTraffic` 已调的 process-status 数组长度即是（本地固定 seed 4 核心），一行修复 |
| 5.3 | Services 页面与 i18n 设计不符 | `Services.vue`（25 行）vs `zh-CN.js:1030-1036` | i18n 定义 5 个 tab，页面只渲染 2 个；但 Process/Rule/Warp 在 NodeServerDetail 已正常可用（用的是 `CoreProcessTab.vue`），services/ProcessTab.vue 是孤儿——倾向定性为设计变更遗留，确认后清理 i18n 与孤儿文件即可 |
| 5.4 | 硬编码 server ID=1 | `Dashboard.vue:281` 等 8 处 | 本地节点 ID 固定为 1（`xraystats/handler.go:84` 等多处后端同样硬编码），属架构约定而非 bug，但建议常量化 |
| 5.5 | 约 150 处空 catch | `Settings.vue` 36 处（保存配置/测试 SMTP/Telegram 失败无提示，L816-831）、`PlanManagement.vue` 21 处、`ChargeManagement.vue` 19 处等 | 错误处理缺失，操作失败用户无感知 |
| 5.6 | 修改密码失败无提示；工单指派手输 admin ID | `Layout.vue:104-115`、`TicketManagement.vue:142-151` | 半成品 UX |

前端无调用任何 placeholder 路由（已全量搜索确认），placeholder 是纯后端预留。

---

## 六、已验证无缺失的设计文档（21 篇）

`PAGINATION_PLAN.md`、`SUBSCRIPTION_DESIGN.md`、`UI_DESIGN.md`、`agent-push-update.md`、`CHARGE_SITE_ACCESS_GUIDE.md`、`CHARGE_SITE_INTEGRATION.md`、`eliminate-reverse-dial.md`（除 SpeedTest）、`log-management.md`、`MISSING_CLIENT_API_DESIGN.md`、`one-line-agent-deploy.md`、`plan-integration.md`、`plan-node-account-sync.md`、`PORTAL_EDITOR_DESIGN.md`、`PORTAL_FREEFORM_DESIGN.md`、`PORTAL_REGION_EDITOR_DESIGN.md`、`PORTAL_TEMPLATE_FRONTEND_DESIGN.md`、`portal-redesign.md`（除 traffic-history）、`rate-limit-design.md`、`remote-cert-landing.md`、`remote-traffic-sync.md`、`process-management-optimization.md`

需要更新以匹配代码现状的文档：`TRAFFIC_QUOTA_DESIGN.md`（配额实现路径已变）、`CODE_LEVEL_DESIGN.md` 第 6-8 章（aggregator/quota 设计与现行 trafficsync 路径不符）、`NETWORK_CONTROL_UI.md`（**其中 fw4/dnsmasq/OpenWrt 部分为客户端设备端设计，服务端不实现**；其余服务端部分如路由标签、策略路由、防火墙规则、IPv4/IPv6 与 QUIC 控制、策略预览/diff 等均未实施，建议整体标注"规划暂停"或拆分客户端/服务端两部分）、`inbound-design.md`（dokodemo/tun UI 已有）、`node-server-architecture.md`（阶段3未实施）、`plan-design.md`（trial 机制未实施）、`CODE_STRUCTURE_SECURITY.md`（安全建议多数未落地，建议标注状态）。
