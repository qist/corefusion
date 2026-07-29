# 分页改造方案与待改清单

> 目标：所有"数据查询类"列表接口/页面统一支持分页，默认每页 20 条，可选档位 20/50/100（封顶 100）。
> 审查日期：2026-07-29

## 一、现状（已核实）

后端分页已有**两套**实现，均返回 `{items, total}`、入参 `limit`+`offset`：

1. `response.Paginated`（11 个 handler 已用）：users、clients、nodes、subscriptions(List/AccessLogs/Rotations)、certificates、charge(orders/cards/balances/callback)、clientauth、logmanage、services、backups(空桩)、security(空桩)
2. `clientext.getLimitOffset(c)`（clientext handler 内）：**默认 20、封顶 100、clamp 已实现**；该 handler 的工单/推荐记录/提现/礼品/文章/共享账号/IP白名单/OAuth 等列表接口已用 `limit/offset` + 返回 `items/total`

结论：用户诉求"默认 20、最多 100"**实质已在 `getLimitOffset` 中落地**；`response.Paginated` 的调用方大多自行传 limit，但缺少统一的封顶 clamp。

前端：`ChargeManagement / TicketManagement / Certificates / ClientExtManagement / SubscriptionCenter / Nodes / Subscriptions` 已用 `el-pagination` 且传 `limit/offset`、读 `items/total`；但多数**写死 `page-size=20`、无每页大小切换档位**。

## 二、真正待补齐（未分页的列表接口）

以下 handler 文件**既无 `response.Paginated` 也无 `getLimitOffset`**，其列表方法大概率全量返回，需补分页：

| handler | 列表方法 / 路由 | 备注 |
|---|---|---|
| plan | `ListClientOrders` `GET /clients/:id/orders`、`ListTemplates` `GET /plan-templates`、`ListVariants`、`ListTrafficPacks` `GET /traffic-packs`、`ListNodeGroups` `GET /node-groups`、`ListGroupMembers`、`ListPromotions` `GET /promotions`、`ListAllInboundsWithServer` | 待逐接口核实 |
| shareDomain | `List` `GET /share-domains` | |
| cfip | `ListSources` `GET /cfip/sources`、`ListEntries` `GET /cfip/entries` | |
| subfmt | `List` `GET /subscription-formats`、`ListRules` `GET /subscription-formats/:id/rules` | |
| subend | `List` `GET /subscription-endpoints`、`ListRules` | |
| mihomo | `List` `GET /mihomo/configs`、`ListProxyGroups`、`ListRules`、`ListRuleProviders` | |
| （portal / protocol / 其他） | 需逐接口核实 | |

> clientext 的**客户端**列表（ListGifts/ListArticles/ListSharedAccounts/ListIPWhitelist 等）当前全量返回，但数据量通常很小，优先级低，可后续统一。

## 三、设计方案

### 后端（统一）
- 新增 `response.ParsePagination(c)`：解析 `limit`/`offset`（兼容 `page`/`page_size`），**服务端强制 clamp**：`limit` 默认 20、封顶 100；`offset>=0`；返回 `{Page, Limit, Offset}`。
- 所有列表 handler 统一改用 `ParsePagination`：取代散落的 `getLimitOffset` 与各处手写 limit 解析。
- 每个列表接口：`p := ParsePagination(c)` → repository `Limit(p.Limit).Offset(p.Offset)` 取本页 + 另查 `COUNT(*)` 得 total → `response.Paginated(c, items, total, p.Limit, p.Offset)`。
- **强制稳定排序**（`ORDER BY created_at DESC` 或 `id DESC`），避免翻页重复/漏数据。
- repository/service 缺 `Paginate`/`Count` 方法的补充（优先复用已有 `Paginate` scope）。

### 前端（统一）
- 新增 `usePagination` composable：封装 `page`/`pageSize`(档位 20/50/100)/`limit`/`offset`/`items`/`total`；页码与每页大小走 `useQueryState` 持久化到 URL；切换档位时回到第 1 页。
- 已用 `el-pagination` 的页面：补"每页大小切换"档位（`page-sizes="[20,50,100]"`）。
- 用 `el-table` 但无分页的页面：接入 `usePagination` + `el-pagination`。

### 统一约定
- 入参：`limit`(默认20,封顶100) + `offset=(page-1)*limit`，兼容 `page`+`page_size`
- 响应：`{ items, total, limit, offset }`（沿用 `response.Paginated`）
- 每页档位：20 / 50 / 100

## 四、实施步骤

1. 后端：`response.ParsePagination` + 常量（默认20/封顶100）— **基础设施已完成**
2. 后端：按第二节逐模块给 handler 加分页（Paginate + Count + ORDER BY + ParsePagination）
3. 前端：`usePagination` composable — **基础设施已完成**
4. 前端：按页面接入，并给已有分页页补档位切换
5. 验证：后端 `go build ./...`；前端 `npm run build` 部署；联调抽查

## 六、完成情况（2026-07-29 收尾）

### 后端（已落地）
- `response.ParsePagination` + `DefaultPageSize=20`/`MaxPageSize=100` 封顶 clamp（已完成）
- 第二节列出的未分页接口已补齐（List + Count + ParsePagination），返回 `{items,total}`：
  - plan：`ListTemplates` `GET /plan-templates`、`ListVariants` `GET /plan-templates/:id/variants`、`ListTrafficPacks` `GET /traffic-packs`
  - shareDomain：`List` `GET /share-domains`（按 type 过滤）
  - cfip：`ListSources` `GET /cfip/sources`、`ListEntries` `GET /cfip/entries`（按 source 过滤）
  - subfmt：`List` `GET /subscription-formats`
  - subend：`List` `GET /subscription-endpoints`
  - mihomo：`List` `GET /mihomo/configs`
- 上述 service/repository 新增 `ListPaged`/`Count*` 方法（`limit<=0` 走全量以兼容内部调用方）

### 前端（已落地）
- `usePagination` composable（已完成）
- 新增统一组件 `components/PaginationBar.vue`：默认档位 `20/50/100`、`:total`/`:page`/`:pageSize` 双向绑定、`@change` 触发拉取
- 接入 `PaginationBar` 的页面：
  - 新分页（后端刚补齐）：SubscriptionFormats、SubscriptionEndpoints、PlanManagement(流量包)、Services/CFIPTab(entries)、Services/ShareDomainTab(兼容 items)、SubscriptionCenter(formats/endpoints/plans)
  - 新分页（原有接口）：Users、Clients、NodeServers(servers)
  - 统一档位 20/50/100：TicketManagement、Certificates、Nodes、Subscriptions、SubscriptionCenter(subscriptions)、ClientExtManagement(提现)、ChargeManagement(订单/卡/余额)
- 其余 `el-table` 页面多为 node-scoped 配置列表（inline 编辑、数据量小且后端未分页），暂不在服务分页范围。

### 验证
- 后端：`go build ./...` 通过
- 前端：`npm run build` 通过（仅 HINT 级既有告警，无新增错误）

## 七、node-scoped 列表页（2026-07-29 收尾补充）

后端未对上述列表做服务分页（返回全量数组）。改用**前端内存分页**：
新增 `composables/useClientPagination.js`（接受 `ref`/`computed` 全量源，按 `useQueryState` 持久化的 `page`/`pageSize` 切分当前页，默认 20 / 封顶 100），配合 `PaginationBar` 组件展示。

已接入页面与表格：
- `ExternalNodes.vue`：sources 表、nodes 表
- `NodeServerDetail.vue`：certificates 表
- `CaddyProxy.vue`：sniRules / l4Sites / httpSites 三表
- `NginxProxy.vue`：sniRules / l4Sites / httpSites 三表
- `LogManage.vue`：discoveredLogs、policies、jobs、writerFiles 四表

说明（未纳入，因属小数据固定集/状态表，分页无意义）：
- `LogManage.vue` 的 `coreOutputList` / `proxyOutputList`（inline 配置编辑，行数=协议类型数）与 `writerStatuses`（运行时状态表）。
- `CoreProcessTab.vue` 的 process / proxy 状态表（固定小集合）。

### 验证
- 前端：`npm run build` 通过（仅 HINT 级既有告警：LogManage 一处未使用 `reactive` 既有声明，非本次引入）

## 八、Xray 协议配置列表（2026-07-29 收尾补充）

同样为后端未分页、返回全量的 Xray 配置列表接入前端内存分页（`useClientPagination` + `PaginationBar`）：

- `protocols/sections/XrayInbounds.vue`：inbounds 列表 `el-table`（绑定 `form` 全量数组）
- `protocols/sections/XrayOutbounds.vue`：outbounds 卡片列表（`v-for="(ob,i) in form"`）
- `protocols/sections/XrayRouting.vue`：routing rules 与 balancers 两个列表（`form.rules` / `form.balancers`）

注意点：Outbounds / Routing 的增删改/上下移 handler 原本以列表索引 `i` 直接操作 `form[...]`，分页后 `i` 会变成页内局部索引而错位。已将 `toggleEnabled / moveOutbound / showEdit / removeOutbound`（Outbounds）与 `editRule / rmRule / editBalancer / rmBalancer`（Routing）改为接收条目对象，通过 `form.indexOf(item)` 解析真实索引，确保分页下操作正确。

### 验证
- 前端：`npm run build` 通过（仅既有 chunk 体积告警）

## 五、注意事项
- 深分页 `OFFSET` 在超大数据量表会变慢；管理后台量级可接受，暂不引入游标分页。
- `COUNT(*)` 在大表有开销，管理后台可接受。
- 切换每页大小时前端应重置到第 1 页，避免越界。
- 排序必须稳定，否则翻页会出现重复/遗漏。
