# CoreFusion 未实现功能审计报告

> **本次重写：2026-09-23**（重写原因见下方"为什么要重写"）
> 审计范围：后端（`cmd/`、`internal/`、`pkg/`）、前端（`web/admin/`）、全部设计文档
> 方法：两路独立深挖（一路查桩/断链/写错，一路做文档↔代码逐条核对）+ **每条关键结论逐处复验**
> （读原文、独立 grep 计数、必要时真机实测）；凡我没复验的，文里都标了"未逐条核实"。

## 为什么要重写

上一版（2026-07-30/08-04）结论是「**真实缺失只有 1 项**」——**这个结论是错的**，它至少漏了三类：

1. **前端在调、后端不存在的接口**（4 条，界面上点了必然 404）；
2. **把失败伪装成成功的实现**（WAF 查询失败回 `200 + 空数组`、`_ = err`、`_, _ = TryIncrementUsedCount`）；
3. **声明了却没有执行点的字段/表**（`activated_at`/`max_active_days` 死字段、`routing_regions`/`server_share_profiles` 空表）。

同时也有一批**反向过时**：文档横幅写着"未实现"，代码其实早已实现。
本版把两个方向都纠正过来，并把"已决议**不做**"的项单列（那不算缺口）。

---

## 一、总体结论（2026-09-23）

| 类别 | 数量 | 说明 |
|---|---|---|
| **真缺的"大件"**（需产品排期） | 4 | 首次启动向导、面板自更新、2FA、网络控制 UI |
| **部分实现 / 遗留** | 2 | 镜像同步讨论稿问题、现有库中的两张空表 |
| **本轮已修复** | 12 | 见第三章（含安全类与"静默失败"类） |
| **已决议不做**（不算缺口） | 3 | 见第五章 |
| **被本版推翻/纠正的旧结论** | 6 | 见第四章 |

模块完成度速览：

| 模块 | 完成度 |
|---|---|
| 代理 / 站点（Nginx·Caddy·SNI·Reality·链式代理·WAF·回源 TLS） | 完成 |
| 客户端 / 套餐 / 订阅下发 / 流量与配额 / 限速 | 完成 |
| 充值支付（站点接入、订单、卡密、余额、回调） | 完成 |
| 节点 / Agent（一键部署、自更新、config_snapshots 节点自治） | 完成 |
| 证书（EAB、远程落地、自动分发） | 完成 |
| 日志管理 / Dashboard 多节点 / Portal 模板 | 完成 |
| 数据导出（CSV、上限可配置） | 完成 |
| 系统设置 / 审计与权限（RBAC、限流、Token 分离、mTLS、logout） | 基本完成（缺 2FA） |
| **首次启动初始化** | **缺失** |
| **面板自更新 / 回滚** | **缺失**（现为诚实 501） |
| **网络控制 UI（TUN / 策略路由 / QUIC …）** | **缺失** |
| **2FA / TOTP、CLI 恢复命令** | **缺失** |
| 镜像同步（mirror-sync） | 部分（讨论稿遗留） |

---

## 二、真实缺失

### P1 — 大件（需要产品决策 / 排期）

| # | 项 | 现状（已复验） |
|---|---|---|
| 2.1 | **首次启动初始化向导** | `docs/design/first-boot-initialization.md` 自述"设计提案，尚未实现"，其 13 项承诺（`init_mode`/`init_token`/`.init_completed`/`/init/*` 路由与前端向导/`reset-init`/强制 HTTPS/随机 `web_path`/守护端口…）在代码中**全仓 0 命中**。现有等价物只有：启动时若无用户则创建 `admin` 并打印一次性口令；以及 `GET /system/bootstrap-state`（已实现）+ `PUT /system/bootstrap-finish`（现为 501）。 |
| 2.2 | **面板自更新 / 回滚** | `GET /system/releases`、`GET /system/update-jobs`、`POST /system/update`、`POST /system/rollback` 现**统一返回 501 NOT_IMPLEMENTED**（原先返回空数组/假成功，见第三章 D 组）。前端不调用这些端点。 |
| 2.3 | **2FA / TOTP** | 全仓 `totp` / `twoFactor` / `2FA` **0 命中**。`CODE_STRUCTURE_SECURITY.md` 建议项。 |
| 2.4 | **网络控制 UI** | `NETWORK_CONTROL_UI.md` 自述"整体尚未实施"。实测 `TunSwitch` / `RouteTag` / `QuicModeSelector` / `RegionBypass` / `policyRouting` 全部 **0 命中**。注意：该文档中 fw4/dnsmasq/OpenWrt 部分属**客户端设备端**设计，服务端本就不实现。 |

### P2 — 部分实现 / 遗留

| # | 项 | 现状 |
|---|---|---|
| 2.5 | 镜像同步（`docs/design/mirror-sync-redesign.md`） | 文档为"讨论稿 v3"，自列 10 个已确认缺陷（DB 克隆一致性、inbound 变更不触发 mirror、`SyncData` 后 xray 不重载等）。**本版未逐条核实**，需要时单独排查。 |
| 2.6 | 两张空表仍留在现有库 | `routing_regions`、`server_share_profiles` 已从 AutoMigrate 摘除（新装库不再创建），但**现有库中的空表未删**——删表属破坏性操作，需要时再确认。 |

---

## 三、本轮（及近期）已修复

| 组 | 内容 | 提交 |
|---|---|---|
| A | **删客户端不吊销其订阅**：`DeleteClient` 原来只删 clients 行 → 孤儿订阅，而 `/sub/:token` 不校验客户端是否存在 → **已删客户端的订阅链接仍能拉配置**（删除形同无效）。新增 `RevokeByClientID`，删除前先吊销；并清理了 10 条历史孤儿（6 条是 active 的"无主可用凭据"）。 | `03894f50` |
| B | **三处静默失败**：WAF `ListNodeRules`/`ListDomainRuleDirs` 出错回 `200 + 空数组`；`commerce` 订单清理调度 `_ = err`（注释说打日志、实际既没日志也丢错误）；`plan` 两处 `_, _ = TryIncrementUsedCount`（丢弃"是否达上限"与 error）。 | `03894f50` |
| C | **订阅「激活后有效天数」**：`activated_at`/`max_active_days` 是死字段（从无执行点）。曾接上执行链路，后**决议撤销**（有效期唯一来源是套餐 `expires_at`），改为 `400 UNSUPPORTED_OPTION` 拒绝写入。 | `3e965fd2` |
| D | **4 条前端断链**：`/security/audit-events/export`（导出按钮必 404）→ 补 CSV 导出；`POST /security/alerts/resolve`（"全部已解决"必 404）→ 补批量解析；`/charge-callback-logs` → 改名后的 `/charge-callback-events`；删除指向已废弃概念的 `getChainExitConfig`。 | `b6da7789` |
| E | **`/system` 5 个桩端点**（空数组 / 假成功）→ 统一**诚实 501**，并抽出 `notImplemented` 助手。 | `b6da7789` |
| F | **导出上限可配置**：原来写死（审计 5 万、充值卡 10 万）→ 新增设置项 `panel.export_max_rows`（默认 50000，范围 1000~2000000，越界自动夹取），并抽出共用包 `internal/api/handlers/exportutil`（上限/转义/BOM/时间戳）。 | `2b6e813f` |
| G | **补充值订单 / 余额流水 CSV 导出**（原来只有列表，对账最需要）。 | `2b6e813f` |
| H | **死代码 / 空 schema**：删除零调用的 `router.placeholder`；`routing_regions`、`server_share_profiles` 从 AutoMigrate 摘除并注明"保留未接线"。 | `aeaa8c72` |

---

## 四、被本版推翻 / 纠正的旧结论

| 旧结论 | 复核结果 |
|---|---|
| "Metrics 端点已实现但前端无任何调用" | **不成立**：`Dashboard.vue` 在调 `getAggregatedMetrics` / `getMetricsOverview`。 |
| "`Services.vue` 是孤儿，i18n 5 个 tab 只渲染 2 个" | **不成立**：该页有多个 tab 且挂在路由/菜单（`menu.services`）。 |
| "`resolveAlert` 调用了不存在的接口" | **部分不成立**：`resolveAlert(id)` → `/alerts/:id/resolve` 与后端一致；真正断的是 `resolveAllAlerts`（已修，见第三章 D）。 |
| "`NoopGeoIPResolver` 是死代码" | **不成立**：有一处真实使用（回落实现）。 |
| "前端 114 处空 catch = 操作失败无提示" | **前提不成立**：`api/client.js` 的响应拦截器**已统一提示**（403/404/5xx/4xx/网络，且支持 `suppressErrorMessage` 单独静默），调用方 catch 掉不影响提示。**反向问题**：上一轮给 83 处加的 `ElMessage.error(t('common.failed'))` 会与拦截器**重复弹两条**，建议后续择机统一（本轮刻意不做这 114 处改动）。 |
| 多处文档横幅"未实现" | **已过时**：见第六章。 |

---

## 五、已决议**不做**（不算缺口）

| 项 | 决定 | 落地方式 |
|---|---|---|
| 订阅 `security_mode` 的 `token_with_expiry` / `token_with_signature` / `token_with_binding`，以及 `bind_mode` 非 `none` | 不做 | 写入即 `400 UNSUPPORTED_OPTION`（fail-closed），避免"设了却无保护"的虚假安全感。 |
| 订阅「激活后有效天数」（`activated_at` + `max_active_days`） | 不做 | 同上；有效期唯一来源是套餐 `expires_at`（`plan_variant.duration_days` → `client.expires_at` → `sub.expires_at`）。将来若要在卡密上支持"兑换后首次使用才起算"，应在卡密流程里新设计入口。 |
| 回调事件 CSV 导出 | 暂不做 | 前端**没有任何页面展示回调事件**，加了会是无入口的半成品；等做了回调日志页面再一并加。 |
| 指标端点 `ConnectionFailures` / `UDPQUICMetrics` | 决议删除 | 代码 0 引用、需 agent 新埋点、收益低。 |

---

## 六、需要更新以匹配代码的文档

| 文档 | 需要的动作 |
|---|---|
| `docs/design/node-server-architecture.md` | 横幅"阶段 3 尚未实现"**已过时** → 已实现（`config_snapshots` + Agent 本地库）。 |
| `docs/design/inbound-design.md` | 横幅"配置生成尚未完成"**已过时** → dokodemo-door / TUN 已实现（`XrayInbounds.vue` + 原始 JSON 直传）。 |
| `docs/design/agent-push-update.md` | 其"未实现清单"**已过时** → Agent 自更新已实现（`PanelCommand_UpdateAgent` → `SelfUpdater`）。 |
| `docs/design/DASHBOARD_MULTINODE_DESIGN.md` | 横幅"设计草案"**已过时** → `Dashboard.vue` 已按多节点实现。 |
| `CODE_STRUCTURE_SECURITY.md` | 需标状态：RBAC / 登录限流 / Token 分离 / mTLS / **logout 已实现**；2FA、CLI 恢复命令**未实现**。 |
| `TRAFFIC_QUOTA_DESIGN.md` | 实现路径已变：实际走 `service/trafficsync` + `traffic_consumer` → `inboundconfig.UpdateTraffic`，**不是**文档里的 `TrafficUsageLedger/Summary` 表驱动方案。 |
| `CODE_LEVEL_DESIGN.md` 第 6–8 章 | `internal/configgen/` 与 aggregator/quota 已删除，现为 `service/coreconfig/` + trafficsync；设计稿与现行实现不符。 |
| `SUBSCRIPTION_DESIGN.md` | 已在本轮标注：`security_mode` 其余 3 种与「激活后有效天数」暂缓（写入即拒绝）。 |

---

## 七、本轮验证方式（可复现）

- **导出上限**：库里审计事件 1004 条，把上限设成 1000 后带 `?limit=100000` 导出 → **正好 1000 行**
  （上限生效且前端突破不了）；还原 50000 后 1005 行。
- **充值导出**：订单 99 行 / 余额流水 74 行，均与库内条数一致，带 UTF-8 BOM 与中文表头。
- **删客户端吊销订阅**：活跃订阅 → `DELETE /clients/:id` → 订阅 `status=revoked`，同一链接立刻 403。
- **空表摘除**：新编译二进制对临时库跑 `corefusion migrate` → 表数 126 → **124**，
  两张表不再创建；现有库的两张空表保留未删。
- **`/system` 501**：5 个端点实测均返回 `501 NOT_IMPLEMENTED`。
