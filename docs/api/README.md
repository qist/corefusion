# CoreFusion API 文档

> 语言： 简体中文 · [English](README.en.md) · [繁體中文](README.zh-TW.md) · [Русский](README.ru.md) · [日本語](README.ja.md) · [한국어](README.ko.md) · [Deutsch](README.de.md) · [Français](README.fr.md) · [Español](README.es.md) · [Português](README.pt.md) · [Türkçe](README.tr.md) · [Bahasa Indonesia](README.id.md) · [ไทย](README.th.md) · [Tiếng Việt](README.vi.md) · [العربية](README.ar.md) · [فارسی](README.fa.md)

面向**外部集成者 / 二次开发 / 第三方客户端**的 API 说明。所有路径均相对于部署域名。

| 文档 | 读者 | 内容 |
|---|---|---|
| [subscription.md](./subscription.md) | 订阅用户 / 客户端开发者 | 订阅拉取接口（`/sub/:token`）：格式、请求头、状态码、示例 |
| [portal-api.md](./portal-api.md) | 门户前端 / 第三方客户端 | 客户端门户 API（`/api/v1/client-auth/*`、`/api/v1/public/*`）：注册登录、套餐、订单、余额、工单等 |
| [charge-callback.md](./charge-callback.md) | 支付渠道对接方 | 支付回调签名与验签 |
| [admin-api.md](./admin-api.md) | 面板二开 / 自动化运维 | 管理端 API 全量路由表（`/api/v1/*`，按模块分组） |

---

## 1. 服务端口与 Base URL

面板可同时暴露三种引擎，共享同一数据库：

| 引擎 | 默认访问方式 | 用途 |
|---|---|---|
| 主面板 | `https://<域名>/api/v1` | 管理后台 + 门户 + 订阅 |
| 订阅引擎（独立端口） | `https://<域名>:<端口>/:token` | 高并发订阅拉取，无 `/sub` 前缀 |
| 门户引擎（独立端口） | `https://<门户域名>[:port]/api/v1` | 纯客户端门户，页面在根路径 `/` |

> 反向代理需同时转发页面路径与 `/api/v1`；支持 `X-Forwarded-Proto` / `X-Forwarded-Host` 头还原真实协议与域名。

## 2. 认证方式（三类，互不通用）

| 认证 | 凭据 | 适用 |
|---|---|---|
| **管理员认证** | `Authorization: Bearer <admin_token>`（`/api/v1/auth/login` 获取） | 管理端 API |
| **客户端认证** | `Authorization: Bearer <client_token>`（`/api/v1/client-auth/login` 获取） | 门户 API |
| **订阅 Token** | URL 路径中的 token：`/sub/<token>` | 订阅拉取，无需 Header |

- 客户端 access token 有效期 24h，refresh token 7 天；JWT 含 `tv`（token_version），登出/踢下线后旧 token 全部失效。
- 管理端角色：`admin` / `operator` / `super_admin`，部分写操作要求 `RequireRole(RoleAdmin)` 以上。

## 3. 响应格式约定

成功（多数端点）：

```json
{ "code": 0, "data": { ... }, "message": "ok" }
```

列表端点（部分）返回 `{ "items": [...], "total": N }` 或裸数组（下文逐个标注）。

错误（HTTP 4xx/5xx）：

```json
{ "code": "ERROR_CODE", "message": "人类可读描述", "request_id": "…", "can_retry": false }
```

**金额单位统一为整数分**：`990` = ¥9.90（`currency: "CNY"`）。
**流量单位统一为字节**：接口返回与请求均为 bytes。

## 4. 限流与维护模式

- 公共端点（`/api/v1/public/*`、`/sub/*`、注册/登录）自带限流；
- 登录另有 `LoginRateLimit`，订阅拉取有 `SubscribeRateLimit`；
- 维护模式开启时公共端点与订阅返回维护提示，管理端不受影响。

## 5. 前端页面路由（非 API）

以下路由返回 HTML/静态资源，不在 API 文档范围内，列出以便联调时排除：

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/` | 管理端面板 SPA（登录后） |
| GET | `/p` | 门户页 HTML（独立门户引擎为 `/`） |
| GET | `/:token` | 独立订阅端口的订阅拉取（无 `/sub` 前缀，语义同 `/sub/:token`） |
| GET | `/static/:file` | 前端静态资源 |

## 6. 路由权威来源

`internal/api/router/router.go`（共 746 条路由，其中 743 条已文档化）。[admin-api.md](./admin-api.md) 的路由表由代码提取生成；如与代码不一致，**以代码为准**。
