# 门户 API 参考（Client / Portal API）

> 面向门户前端（`/p` 或独立门户端口）与第三方客户端的**完整**接口参考。
> 路由定义：`internal/api/router/router.go`（`clientAuthGroup` / `clientAuthProtected` / `/api/v1/public/*`）。

## 1. 通用约定

### 1.1 Base URL

| 部署形态 | Base URL |
|---|---|
| 主面板引擎 | `https://<面板域名>/api/v1`（门户页在 `/p`） |
| 独立门户端口 | `https://<门户域名>[:port]/api/v1`（门户页在根路径 `/`） |
| 独立门户域名反代 | 同上，反代需同时转发页面路径与 `/api/v1` |

### 1.2 认证

- 登录后携带 `Authorization: Bearer <access_token>`；
- access token 有效期 24h，refresh token 7 天；
- JWT 内含 `tv`（token_version）：登出/踢下线后版本 +1，旧 token 全部失效
  （`clients.token_version`，见 `internal/service/clientauth/service.go`）；
- 所有受保护接口都经过 `ClientAuthMiddleware`（版本校验 + 维护模式守卫）。

### 1.3 响应包装

大多数端点返回 `{ "code": 0, "data": ..., "message": "..." }`；
部分列表端点直接返回 `{ "items": [...], "total": N }` 或裸数组（下文逐个标注）。
错误返回 `{ "code": "ERROR_CODE", "message": "...", "request_id": "...", "can_retry": ... }`
（HTTP 4xx/5xx）。

### 1.4 金额单位

订单/余额类字段为**整数分**：`990` = ¥9.90（`currency: "CNY"`）。

### 1.5 真实示例（登录 → 拉取资料）

```bash
# 注册（若 registration_enabled=true）
curl -X POST https://<门户域名>/api/v1/client-auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","password":"明文密码","referral_code":""}'
# → 201 {"data":{"access_token":"<JWT>","refresh_token":"<JWT>","client":{...}}}

# 登录
curl -X POST https://<门户域名>/api/v1/client-auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","password":"明文密码"}'
# → 200 同上

# 拉取当前客户资料（带 token）
curl https://<门户域名>/api/v1/client-auth/me \
  -H 'Authorization: Bearer <access_token>'
# → 200 {"data":{"id":27,"email":"user@example.com","nickname":"","status":"active","quota_bytes":10737418240,"used_bytes":0,...}}

# 订阅拉取（无需 Header，token 在路径里）
curl 'https://<门户域名>/sub/<subscription_token>?format=mihomo_yaml'
# → 200 Mihomo YAML（或 base64_uri / singbox_json / xray_json）

# 错误样例：token 失效
# → 401 {"code":"errClientTokenRevoked","message":"...","request_id":"...","can_retry":false}
```

401 时先 `POST /api/v1/client-auth/refresh` 换新 token 重试一次，仍失败则清除本地登录态（门户模板已内置）。

---

## 2. 公开端点（无需登录）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/p`（独立门户引擎为 `/`） | 门户页 HTML（服务端渲染 + Vue 运行时） |
| GET | `/api/v1/public/portal-config` | 门户运行时配置：站点名、主题色、导航、模块、多语言包、`registration_enabled`、`regions`（登录区源码）、`privacy_policy_url` / `terms_of_service_url` / `privacy_policy_content` / `terms_of_service_content`（合规文档；内容按语言映射；独立页面 `/p/legal/*` ✓） |
| GET | `/api/v1/public/settings` | 公开设置（当前含 `registration_enabled`） |
| GET | `/api/v1/public/subscription-endpoints` | 订阅接入点列表（裸数组） |
| GET | `/api/v1/public/articles` | 公开文章列表（已发布） |
| GET | `/api/v1/public/articles/:slug` | 文章详情 |
| GET | `/api/v1/public/client-configs` | 客户端下载配置 |
| GET | `/sub/:token` | 订阅下发（按 `format` 参数返回 base64_uri / raw_uri / mihomo_yaml / singbox_json / xray_json 等；已吊销/不存在返回 404，暂停/到期/未激活返回 403。详见 [subscription.md](./subscription.md)） |
| GET | `/api/v1/client-auth/oauth/providers` | OAuth 提供商显隐：`{"data":{"providers":[{"provider":"github","enabled":true},...]}}` |
| GET | `/api/v1/system/maintenance` | 维护模式状态（公开）。维护开启时门户显示维护页 |

> 以上均带维护模式守卫与公共 API 限流。

## 3. 认证 / OAuth

| 方法 | 路径 | 认证 | 说明 |
|---|---|---|---|
| POST | `/api/v1/client-auth/register` | — | 注册。body：`{email, password, referral_code?}`。返回 `{access_token, refresh_token, client}`；受注册限流 |
| POST | `/api/v1/client-auth/login` | — | 登录。body：`{email, password}`。返回同上；签发带 `token_version` |
| POST | `/api/v1/client-auth/refresh` | — | 续签。body：`{refresh_token}`。返回新的 access/refresh（版本校验，登出后旧 refresh 立即失效） |
| POST | `/api/v1/client-auth/logout` | ✓ | 登出：服务端吊销（`token_version +1`）→ 该客户所有旧 token 失效 |
| POST | `/api/v1/client-auth/rotate-token` | ✓ | 轮换订阅 token |
| POST | `/api/v1/client-auth/ticket` | ✓ | 签发**一次性短时票据**（门户"下载日志包"等 `window.open` 场景用，无法带 Header） |
| GET | `/api/v1/client-auth/oauth/:provider` | — | 发起 OAuth 授权：307 → GitHub/Google 授权页（`state` 防 CSRF，10 分钟有效）；未启用的 provider 返回 400 |
| GET | `/api/v1/client-auth/oauth/:provider/callback` | — | OAuth 回调：换 code → 建/绑客户 → 签发带版本的 JWT → 302 跳回门户页（`/<portalPath>?access_token=...&refresh_token=...`） |

> OAuth 回调是**浏览器跳转**（非 API 调用）：登录成功后门户页在 `onMounted` 里
> 从 URL 收 token（或经同源 `postMessage`，见 `web` 门户模板 `handleOAuthCallback`）。

## 4. 账号

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/v1/client-auth/me` | 当前客户：`{id, email, plan_id, plan_name, status, quota_bytes, used_bytes, upload_bytes, download_bytes, expires_at, ip_whitelist, device_limit, sub_token, sub_status, created_at, ...}`（`data` 包装） |
| POST | `/api/v1/client-auth/change-password` | body：`{old_password, new_password}` |
| GET | `/api/v1/client-auth/checktime` | 服务器时间（客户端校时） |
| GET | `/api/v1/client-auth/connection-info` | 当前套餐连接参数：`{concurrent, device_limit, protocol, speed_limit, tls, transport}` |

## 5. 订阅

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/v1/client-auth/reset-subscription` | 重置订阅；重置会轮换 sub_token |

> 订阅链接由 `/me` 返回的 `sub_token` 在前端拼接（`/sub/:token`），不再有独立的
> `subscription-link` / `subscription-formats` 端点。

## 6. 节点

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/v1/client-auth/nodes` | 当前套餐可见节点：`{items:[...]}`（含名称/协议/传输/地区等展示字段） |
| GET | `/api/v1/client-auth/nodes/:id/config` | 节点完整配置 |
| GET | `/api/v1/client-auth/nodes/:id/link` | 分享链接（vless:// 等） |
| GET | `/api/v1/client-auth/nodes/:id/qrcode` | 二维码图片 |

## 7. 商店 / 套餐 / 订单

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/v1/client-auth/plans` | 套餐列表（裸数组）：`{id, name, description, category, sort_order, variants:[{id, name, billing_cycle, duration_days, price(分), original_price, currency, quota_bytes, ...}]}` |
| GET | `/api/v1/client-auth/plans/:id` | 套餐详情 |
| GET | `/api/v1/client-auth/traffic-packs` | 流量包列表 |
| POST | `/api/v1/client-auth/traffic-packs/buy` | 购买流量包 |
| GET | `/api/v1/client-auth/orders` | 我的套餐订单（分页） |
| POST | `/api/v1/client-auth/orders` | 下单（购买/续费套餐变体） |
| GET | `/api/v1/client-auth/orders/:id` | 订单详情 |
| GET | `/api/v1/client-auth/invoices` | 账单 |
| GET | `/api/v1/client-auth/invoices/:id` | 账单详情 |

## 8. 余额 / 充值

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/v1/client-auth/balance` | `{data:{client_id, balance(分), frozen}}` |
| GET | `/api/v1/client-auth/balance/transactions` | 余额流水（`type`: charge/consume/refund；分页） |
| GET | `/api/v1/client-auth/charge/sites` | 可用充值站点（`ListChargeSites`：仅 enabled） |
| POST | `/api/v1/client-auth/charge/create` | 创建充值订单（跳转第三方支付） |
| GET | `/api/v1/client-auth/charge/orders` | 充值订单列表 |
| GET | `/api/v1/client-auth/charge/orders/:id` | 充值订单详情 |
| GET | `/api/v1/client-auth/charge/status/:trade_no` | 按 trade_no 轮询支付状态 |
| POST | `/api/v1/client-auth/charge/redeem-card` | 卡密兑换（充值卡） |

> 客户删除时其余额与流水**级联删除**（见 `internal/service/client/service.go DeleteClient`）。

## 9. 礼品 / 流量历史

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/v1/client-auth/gifts` | 可领取礼品（裸数组；`type`: days/bytes） |
| POST | `/api/v1/client-auth/gifts/:id/claim` | 领取 |
| GET | `/api/v1/client-auth/traffic-history` | 月度流量历史：`{data:{history:[{period_start,total_bytes,...}], months}}` |

## 10. 工单

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/v1/client-auth/tickets` | 工单列表：`{items,total}` |
| POST | `/api/v1/client-auth/tickets` | 创建工单 |
| GET | `/api/v1/client-auth/tickets/:id` | 工单详情（含回复） |
| POST | `/api/v1/client-auth/tickets/:id/reply` | 回复 |
| POST | `/api/v1/client-auth/tickets/:id/close` | 关闭 |

## 11. 文章 / 教程

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/v1/client-auth/articles` | 文章列表（含分类筛选） |
| GET | `/api/v1/client-auth/articles/:slug` | 文章详情 |

## 12. 推广邀请（默认关闭，门户编辑器可开）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/v1/client-auth/referral/code` | 我的邀请码与邀请链接 |
| GET | `/api/v1/client-auth/referral/stats` | 佣金统计（`commission_rate`、`available_commission`、`min_withdraw_amount`） |
| GET | `/api/v1/client-auth/referral/records` | 邀请记录 |
| GET | `/api/v1/client-auth/referral/withdrawals` | 提现记录 |
| POST | `/api/v1/client-auth/referral/withdraw` | 申请提现 |

## 13. 共享账户

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/v1/client-auth/shared-accounts` | 可用共享账号（流媒体等） |
| GET | `/api/v1/client-auth/shared-accounts/:id` | 详情 |
| POST | `/api/v1/client-auth/shared-accounts/:id/apply` | 申请使用 |
| POST | `/api/v1/client-auth/shared-accounts/:id/repair` | 一键修复 |

## 14. 日志包（订阅用户诊断）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/v1/client-auth/logs/types` | 可打包的日志类型 |
| GET | `/api/v1/client-auth/logs/packages` | 我的日志包列表 |
| POST | `/api/v1/client-auth/logs/package` | 创建日志打包任务（仅入口节点可用） |
| GET | `/api/v1/client-auth/logs/packages/:id` | 任务状态（ready/processing/failed） |
| GET | `/api/v1/client-auth/logs/packages/:id/download` | 下载（建议先 `POST /ticket` 换一次性票据拼 `?ticket=`） |
| DELETE | `/api/v1/client-auth/logs/packages/:id` | 删除日志包 |

## 15. 安全设置 / IP 白名单 / 自解封

| 方法 | 路径 | 说明 |
|---|---|---|
| GET/PUT | `/api/v1/client-auth/settings` | 客户端设置：`{ip_whitelist, device_limit}`（PUT） |
| GET | `/api/v1/client-auth/settings/whitelist` | 白名单列表（裸数组） |
| DELETE | `/api/v1/client-auth/settings/whitelist/:id` | 删除白名单项 |
| POST | `/api/v1/client-auth/settings/auto-add-ip` | 把当前 IP 加入白名单 |
| POST | `/api/v1/client-auth/self-unban` | 自助解封（按策略限制次数） |

> IP 白名单**只作用于订阅地址**（`/sub/:token`），不拦截门户登录/OAuth 或其它
> client-auth 端点；用户未配置白名单或没有订阅地址时一律放行。白名单是用户级配置，
> 没有全局开关。

**自解封流程**：仅 `auto_quota`（流量超额自动封禁）+ 管理端 `self_unban.enabled` 开启时可用。
登录（密码或 OAuth 回调）返回 `require_self_unban=true` + `self_unban_token`（typ=self_unban，
15 分钟有效，**只能**调 `POST /self-unban`，访问其它端点一律 403/`SELF_UNBAN_TOKEN_SCOPE`）；
门户据此显示解封卡，解封成功后重新登录。OAuth 回调经 URL fragment 下发
`#require_self_unban=1&self_unban_token=..&self_unban_hint=..`。

## 16. 错误码摘录

| code | 场景 |
|---|---|
| `INVALID_TOKEN` | refresh token 无效/过期/版本不匹配 |
| `INVALID_REQUEST` | 参数校验失败 |
| `NOT_FOUND` | 资源不存在（工单/订单/白名单等） |
| `RELOAD_FAILED` / `SYNC_FAILED` | （管理端）代理配置同步失败 |
| 401 + `errClientTokenRevoked` | token_version 不匹配（已登出/被踢）→ 客户端应走 refresh，失败即清除本地 token |

## 17. 合规文档（隐私政策 / 服务条款）

登录卡片底部的"隐私政策 · 服务条款"链接（与常规站点一致，点击打开条款内容）：

- **配置入口**：门户编辑器 → 设置 → 基础设置 → 合规文档（**按语言**分别编辑内容）；
  也可 `PUT /api/v1/portal/active` 传结构化字段 `privacy_policy_url` / `privacy_policy_content` /
  `terms_of_service_url` / `terms_of_service_content`（nil=不改；内容为 `{"zh-CN":"...","en":"..."}` 映射）。
- **独立可访问页面**（服务端渲染，便于 Google 验证抓取，支持 `?lang=xx`）：
  - 主面板域名：`GET /p/legal/privacy`、`GET /p/legal/terms`
  - 独立门户域名（根路径部署）：`GET /legal/privacy`、`GET /legal/terms`
- **展示逻辑**：链接与内容二选一，**URL 优先**（新标签打开外链）；填了内容则登录卡片点击后
  弹窗渲染 Markdown（`renderMd`），按当前门户语言挑选内容。
- 运行时通过 `/api/v1/public/portal-config` 下发，登录区组件经 `ctx.privacyUrl / termsUrl /
  privacyContent / termsContent / openLegal / hasLegalLinks` 消费。
- Google OAuth 正式授权要求这些链接公共可访问 → 用上面的独立链接填入同意屏幕；
  见 `docs/oauth-google-verification.md`（内含隐私政策/服务条款模板文本）。

## 18. 接入示例

```js
// 登录
const { data } = await fetch('/api/v1/client-auth/login', {
  method: 'POST', headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ email, password })
}).then(r => r.json());
localStorage.setItem('client_token', data.access_token);
localStorage.setItem('client_refresh_token', data.refresh_token);

// 带 token 请求
const me = await fetch('/api/v1/client-auth/me', {
  headers: { Authorization: 'Bearer ' + data.access_token }
}).then(r => r.json());
```

401 时先 `POST /refresh` 换新 token 重试一次，仍失败则清除本地登录态
（门户模板 `api()` 已内置该流程）。
