# CoreFusion 未实现功能审计报告（修订版）

> 审计时间：2026-07-30（初版后经逐项二次核实修订）
> 审计范围：后端（`cmd/`、`internal/`、`pkg/`）、前端（`web/admin/`）、全部设计文档（根目录 9 篇 + `docs/` 22 篇）
> 修订原则：代码与设计文档存在出入（代码改了文档没跟），凡「代码已用其他方式实现等价功能」「与其他已实现功能重复」「客观上无法实现」的项一律从缺失清单剔除或归入"文档过时"章节，本报告只保留**真实缺口**。

---

## 一、总体结论（修订后）

初版报告中的部分"缺失"经核实为**文档过时或误判**，实际情况：

| 类别 | 数量 | 说明 |
|---|---|---|
| 真实缺失 | 12 项 | 见第二章 |
| 文档过时（代码已有等价实现） | 8 项 | 见第三章，应从设计文档/placeholder 中清理 |
| 死代码/代码卫生 | 10 项 | 见第四章 |
| 前端问题 | 6 项 | 见第五章 |

关键修正：**流量子系统并非"整体不生效"**。用量统计已生效（trafficsync + TrafficEventConsumer → `subscriptions.*_bytes_cache`），限速/设备数/并发限制已由 ratelimit 模块（nftables+tc）实际执行。真正缺的只是"订阅级流量超额自动封禁"和"非 xray 协议流量统计"等三块（见 2.2、2.3）。

---

## 二、真实缺失（按优先级）

### P0 — 安全相关

#### 2.1 首次启动初始化向导完全缺失

- 来源：`docs/design/first-boot-initialization.md`（文档自述"状态：设计阶段"）
- 现状：全库无 `/api/v1/init/*` 路由、无 `IsFirstBoot`、无 `reset-init` 命令、无初始化前端页面；`cmd/corefusion/main.go:330` 硬编码 `CreateInitialAdmin("admin","admin123")`
- 缺失内容：随机初始化端口/Token、五步配置向导（端口/管理员/访问路径/HTTPS）、初始化前端
- 关联：`handlers/system/handler.go` 的 `GetBootstrapState`/`FinishBootstrap`（L350-358）是无前端调用的死端点

### P1 — 核心功能闭环

#### 2.2 订阅级流量超额/自然过期的自动处理缺失

- **用量统计已生效**（不计缺失）：本机走 `service/trafficsync`（xray `QueryStats reset_=true` 轮询），远程走 Agent heartbeat → `traffic_consumer.go:105`，两条路径都汇聚到 `inboundconfig.Service.UpdateTraffic`，原子累加 `subscriptions.used_bytes_cache`（`inboundconfig/service.go:193-199`）
- **真实缺口**：
  - 没有任何代码在 `used_bytes_cache >= quota_bytes` 时置 `quota_state` 或 suspend 订阅；`SuspendOnQuotaExhausted` 字段（`entity/subscription.go:29`）只写不读
  - 订阅自然到期无定时扫描任务：`client.Status="expired"` 只在退款回退路径出现（`commerce.go:2115`），自然过期不会自动标记、不会自动从核心移除用户
  - （对照：节点级流量配额封禁已实现，`nodeserver/traffic_quota.go:210-300` 超额会 suspendNode）
- 已知隐患：`UpdateTraffic` 只有在某 inbound config 的 JSON 里匹配到 email 才更新订阅缓存（`inboundconfig/service.go:157`），用户不在任何 inbound 中时缓存不更新

#### 2.3 非 xray 协议流量统计：最新实现已就位（网络层计费），仅订阅级接线未闭合

> **更正（按最新实现）**：非 xray 流量统计**不再依赖各核心原生 Stats API**，已改用**网络层计费（对核心类型透明）**，最新实现分两级：

- **节点级（已完整实现并接线）**：`cmd/agent/metrics.go:46-49` 读取出口网卡（`egressInterface`）累计 rx/tx 字节，经心跳 `EgressRxBytes/EgressTxBytes`（`heartbeat.go:190-193`）上报；`nodeserver/traffic_quota.go:21-22` 按心跳做差累计到 `NodeServer.traffic_used_bytes` 并触发超额 suspendNode。该路径对任意核心（xray/singbox/hysteria2/tuic）均生效，**节点级月度配额已闭环**，不再属缺失。
- **订阅级（组件已实现但未接线）**：`internal/service/traffic/ip_tracker.go` 的 `SubscriptionIPTracker` 通过 nftables 为每个订阅 IP 建 `sub_<id>` 链 + `counter` 规则，按 `expr.Counter.Bytes`（`ip_tracker.go:235`）统计订阅级流量（核心无关），本可作为 `used_bytes_cache` 的来源。

- **当前真实缺口（接线未完成，非"从零实现"）**：`SubscriptionIPTracker` 目前是**孤立组件**——
  - `NewSubscriptionIPTracker` 全仓库无实例化、`RecordAccess`（建 nft 规则）从未被调用；
  - 其 `eventCh` 未接到 `Aggregator`，且它自定义的 `TrafficDeltaEvent`（`ip_tracker.go:256`，字段 `SubID`）与 `domain/metric/traffic.go` 的 `TrafficDeltaEvent`（字段 `SubscriptionID`）不是同一结构体，未经桥接；
  - 因此 `sub_<id>` 链不会被创建，`ReadSubscriptionTraffic` 永远读到 0，**订阅级非 xray 流量暂未真正计入 `used_bytes_cache`**（xray 仍走 `inboundconfig.UpdateTraffic` 的 email 匹配）。

- 结论：方案与组件代码已具备，**剩余工作是把 `SubscriptionIPTracker` 接线**（实例化 → 在订阅访问时 `RecordAccess` 建链 → `SetEventChannel` 接 Aggregator → 事件结构桥接到 `SubscriptionID`），属于接线/收尾，而非重新设计。TUIC 无原生 Stats API 的客观限制已由网络层方案规避。

#### 2.4 Agent 端 SpeedTest 未实现

- `cmd/agent/command_handler.go:351-355` 固定返回 `not implemented`
- 后果：远程节点延迟测速/按延迟排序对远端节点完全不可用（本地节点 TCP 测速正常）
- 无任何替代路径

#### 2.5 访问控制管理面缺失（底层能力已在）

- 已实现部分：`service/access/access.go` 的 TokenBucket/ConcurrentTracker/AutoBanManager 组件、`service_access_policies` 和 `service_ip_bans` 表已建（`entity/access.go:6-39`）
- 真实缺口：`access.NewService` 无调用方（服务从未实例化）、`BanRepository` 只有接口无数据库实现、管理面 API（`/services/:id/access-policies`、`/services/:id/ip-bans`）是 placeholder、`CheckConnection` 未串联并发/速率检查

#### 2.6 Notifier 模板通知无发送引擎

- `internal/service/system/notifier.go` 完整实现了 Telegram/SMTP/模板通知（`SendTemplateNotification:117`），通知模板 CRUD 也有 handler
- 但 `router.go:418` 创建后 `_ = notifier` 丢弃，全库无任何业务调用方——订单、工单、证书到期等场景均无通知发送
- 注：设置页"测试 SMTP/Telegram"走 `settingSvc.TestSMTP/TestTelegram` 独立路径，不代表业务通知已通

#### 2.7 Agent 节点配置快照（数据本地化）缺失

- 来源：`docs/design/node-server-architecture.md` 阶段3
- 现状：`config_snapshots` 表/`saveConfigSnapshot` 仅存在于文档，代码无匹配
- 后果：面板失联/宕机时，远程 Agent 重启后无法从本地快照恢复配置，节点无法自治

### P2 — 功能补齐

#### 2.8 安全审计事件无写入逻辑（骨架已搭）

- 修正：表**已建**（`entity/access.go:58-73` 已 AutoMigrate 注册），过期清理逻辑也有（`log_cleanup.go:103`）
- 真实缺口：全库无任何 `SecurityAuditEvent` 写入代码 → 查询恒为空；告警（alerts）、告警渠道 CRUD 为 NOT_IMPLEMENTED；且无登录日志/管理操作日志等替代体系（entity 层无 LoginLog/OperationLog）

#### 2.9 面板自身更新/回滚缺失（**后期实现**）

- `handlers/system/handler.go:360-374` SystemUpdate/SystemRollback/ListReleases/ListUpdateJobs 全是桩
- 指**面板自身**更新（节点 Agent 更新是另一套已实现机制）；仓库无部署/升级脚本，前端无调用入口
- **排期：后期实现**（暂不作为近期缺口处理，但实现前需先解决分发基础设施——构建产物源，可复用 Agent 自更新机制）

#### 2.10 试用套餐自动转正/过期机制缺失

- 来源：`docs/design/plan-design.md`
- 现状：`is_trial`/`trial_duration_hours`/`trial_auto_convert_template_id` 字段已建表，但 router/service 层无任何 trial 处理逻辑

#### 2.11 Runtime 配置工作流缺失（骨架已搭）

- placeholder：`/runtime/snapshots`、`/runtime/apply*`、`/runtime/dry-run`、`/runtime/rollback`、`/runtime/disable-proxy`、`/runtime/overview`
- 骨架：`runtime_snapshots`/`runtime_apply_jobs` 等 4 张表已建（`entity/runtime.go:6-75`）、`RuntimeApplyService` 接口已定义（`domain/runtime/interfaces.go:73-80`）
- 现状：配置下发是"写文件→重启核心"直写模式（`process/manager.go:153` + configsync），无快照/版本/dry-run/回滚机制

#### 2.12 xray dokodemo-door / TUN 入站配置**生成未实现**

- 澄清：前端 `XrayInbounds.vue` **已有** dokodemo-door（L295、L817-847）和 tun（L293、L784-812）的编辑 UI，但**这不代表 xray 已实现**——后端尚未支持。
- 真实缺口：`configgen/xray/generator.go:275` 的 `switch spec.Protocol` 只处理 VLESS/VMess/Trojan/Shadowsocks 四种协议，dokodemo-door / tun 落到 default 分支生成**空的 `InboundSettings`**，即这两类入站生成出的 xray 配置不可用（仅 sing-box 生成器实现了 tun，`singbox/generator.go:321`）。ফলে 用户在前端配置 dokodemo/tun 后，面板下发的 xray 配置是残缺的。
- 修复方式：在 `generator.go` switch 补 dokodemo-door（`address`/`port`/`network`/`followRedirect`）与 tun（`tun`/`auto_route`/`stack` 等）分支。

### P3 — 规划类/低优先级

| 项 | 说明 |
|---|---|
| Client merge-profiles / route-profiles（22 条 placeholder） | 多订阅源合并、客户端路由规则集 profile，完全新概念，无实体无实现，前端也无调用 |
| Server Share profiles（12 条 placeholder） | 比已实现的 share-domains 更高层：发布/撤销/版本历史/预览/二维码/导出，部分重叠（见 3.4） |
| Routing Regions CRUD（4 条 placeholder） | `routing_regions` 表已建（`entity/runtime.go:168-180`）无实现；与 geo-rules/region_selector 用途不同（见 3.5） |
| `client-auth/traffic-history` | 门户自述"未来扩展"；节点级 traffic-history 已有 |
| gRPC `RegistrationService.Register` 未接线 | `manager.go:1503-1512` 返回未接线；现行路径走 REST bootstrap-token（已实现且够用），属可选增强 |
| Metrics `ConnectionFailures`/`UDPQUICMetrics` | 空端点，但前端无任何调用（前端用 `/node-servers/:id/metrics-history`），死端点 |
| 安全能力：2FA/RBAC/管理端 mTLS/CLI 恢复命令/`corefusion client` 模式/`web/client` 前端 | `CODE_STRUCTURE_SECURITY.md` 的建议项，均未落地 |

> **明确不实现项（已排除）**：**CSRF 防护**——系统使用 Bearer Token 鉴权、无 Cookie 会话，CSRF 无攻击面，**决定不实现**，从缺失清单移除。

> **客户端设计（服务端不实现，从缺失清单剔除）**：
> `router.go` 中 `/route-runtime/openwrt/*`（7 条 placeholder）以及 `domain/common.go:141-142` 的 `Dnsmasq`/`Fw4` 枚举常量，描述的是 **OpenWrt 客户端设备端**的能力（fw4 防火墙、dnsmasq、OpenWrt 集成下发），属于客户端（`web/client` / `corefusion client` 模式）范畴，**服务端明确不实现**。
> 因此这些路由不应作为"服务端未实现功能"计入；建议：
> - 删除服务端的 7 条 `/route-runtime/openwrt/*` placeholder 路由（避免造成"服务端缺失"的误判）；
> - 相关 dnsmasq/fw4 枚举常量保留，归入后续客户端设计文档，不计入本服务端审计报告。

---

## 三、文档过时项（代码已有等价实现，不计缺失）

以下各项是"设计文档/placeholder 未跟上代码演进"造成的假象，**建议从设计文档标注过时、删除对应 placeholder 路由**：

| # | 项 | 实际等价实现 | 证据 |
|---|---|---|---|
| 3.1 | 证书部署 Deploy() 桩 | 证书签发/续期/上传后自动分发：`fireCertUpdated` → `DistributeCertificateToAll` → Agent 磁盘 + ReloadCore；反代证书随 SyncProxyConfig 携带；断线重连补发 | `certificates/handler.go:203,239,305`、`router.go:1249-1254`、`cert_distributor.go:36,165` |
| 3.2 | `/reverse-proxies` 6 条 placeholder | 反代管理已完整实现：`/node-servers/:id/proxy/*`（通用）、`/caddy/*`、`/nginx/*` 全套 CRUD/reload/配置查看，前端 CaddyProxy/NginxProxy 完备 | `router.go:1504-1565`、`system/caddy_handler.go`、`nginx_handler.go`、`proxy.go` |
| 3.3 | 流量聚合器被注释（"aggregator 死代码致配额不生效"） | 用量统计已改道：trafficsync（本机）+ TrafficEventConsumer（远程）→ `inboundconfig.UpdateTraffic` → `subscriptions.*_bytes_cache`；`TrafficUsageLedger/Summary` 是被废弃的旧设计 | `trafficsync/service.go:222`、`traffic_consumer.go:105`、`inboundconfig/service.go:133-208` |
| 3.4 | Server Share 的"分享域名"能力 | `/share-domains` + `/node-servers/:id/share-domains` 全套 CRUD 已实现，前端 ShareDomainTab 在用；placeholder 多出的只是发布/版本/二维码等增强 | `router.go:937-945,1635-1639`、`sharedomainconfig/handler.go` |
| 3.5 | 区域能力的两个相近实现 | geo-rules（GeoIP 数据文件管理，`/node-servers/:id/geo-rules`）+ region_selector（按用户 IP 推荐区域）均已在用；`/routing/regions` CRUD 是第三种用途（自定义区域定义），表已建但需求需重新确认 | `router.go:1608-1624`、`georule/service.go`、`region_selector.go` |
| 3.6 | `adapters/` 四个流量采集器"空实现/近似实现" | 该包整体**无人 import**，是未完成即被废弃的平行新架构；实际采集由 trafficsync（xray）承担 | `runtime/adapters/*` 无引用；`trafficsync/service.go` |
| 3.7 | proto `AgentService` 20 个 RPC "未注册未实现" | 命令全部走 Heartbeat 双向 stream 的 PanelCommand（`SendCommandAndWait` 20+ 处调用），proto 的 AgentService 是消除反向拨号后的死定义 | `eliminate-reverse-dial.md`、`manager.go:1023` |
| 3.8 | main.go:301 RegionSelector 创建后丢弃 | `router.go:1307-1311` 已正确接线到 `nodeserver.Service.WithRegionSelector`，功能正常；main.go 那行是冗余 | `router.go:1307`、`nodeserver/service.go:899` |
| 3.9 | CreateConfigBackup 写空文件 | "完整备份"已打包整个 dataDir（含全部配置，仅排除 backup/rules_tmp/cache/logs）+ DB dump，配置不会丢；config 单项端点是冗余桩 | `backup/service.go:44-86` |

---

## 四、死代码 / 代码卫生（2026-07-30 已清理）

> 本章原列的死代码已执行清理。**保留项**（`domain/runtime/interfaces.go`、`entity/runtime.go` 的快照/区域表、`ip_tracker.go`）是 2.11 Runtime 工作流 / 2.3 网络层计费所需的脚手架，删除会与规划功能冲突，故保留。

| 项 | 位置 | 处理结果 |
|---|---|---|
| `service/quota` 包整体 | `quota/quota.go`、`enforcer.go` | ✅ **已删除**（无调用方，2.2 将走 ratelimit 路径） |
| `traffic/aggregator.go`、`multi_collector.go` | `service/traffic/` | ✅ **已删除**（被 main.go 停用、无外部引用）；`ip_tracker.go` 保留（2.3 所需） |
| 空目录模块 | `internal/observer/`、`scheduler/`、`region/`、`platform/`、`api/handlers/networkpolicy/`、`service/networkpolicy/` | ✅ **已删除**（确认无文件） |
| proto AgentService 死定义 | `runtime/agent/proto/corefusion_agent.proto:29-71` | ⏸️ **未删**：与 HeartbeatService 同文件生成，删除需重新生成 pb.go，留待手动处理 |
| 孤儿前端文件 | `views/Subscriptions.vue`、`SubscriptionEndpoints.vue`、`SubscriptionFormats.vue`、`Nodes.vue`、`services/ProcessTab.vue` | ✅ **已删除 5 个**（被 SubscriptionCenter/CoreProcessTab/NodeServers 取代） |
| ~~`ProxyManagement.vue`~~ | `views/ProxyManagement.vue` | ⚠️ **更正**：经核实 `NodeServerDetail.vue:265` 实际 import 了它，并非孤儿，**已保留** |
| Legacy portal API | `api/portal.js:33-56` 的 6 个函数 | ✅ **已删除**（仅文件内部引用，无调用方） |
| main.go 冗余初始化 | `main.go:300-302` | ✅ **已删除**（`_ = subscription.NewRegionSelector(...)` 及 `geoIPResolver` 变量），并移除多余 import |
| Metrics 2 个死端点 | `metrics/handler.go:102-111` + `router.go:1772-1773` | ✅ **已删除** handler 方法与路由注册 |
| 废弃 domain 接口 | `domain/runtime/interfaces.go` | ⏸️ **保留**：2.11 RuntimeApplyService 需要 |
| `routing_regions`/`runtime_*` 空骨架表 | `entity/runtime.go` | ⏸️ **保留**：2.11 Runtime 工作流 / 路由区域需要 |

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
