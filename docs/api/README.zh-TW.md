<!-- source: README.md | locale: zh-TW | updated: 2026-10-23 -->
# CoreFusion API 文件

> 語言： [简体中文](README.md) · [English](README.en.md) · 繁體中文 · [Русский](README.ru.md) · [日本語](README.ja.md) · [한국어](README.ko.md) · [Deutsch](README.de.md) · [Français](README.fr.md) · [Español](README.es.md) · [Português](README.pt.md) · [Türkçe](README.tr.md) · [Bahasa Indonesia](README.id.md) · [ไทย](README.th.md) · [Tiếng Việt](README.vi.md) · [العربية](README.ar.md) · [فارسی](README.fa.md)

面向**外部整合者 / 二次開發 / 第三方客戶端**的 API 說明。所有路徑均相對於部署域名。

| 文件 | 讀者 | 內容 |
|---|---|---|
| [subscription.md](./subscription.md) | 訂閱使用者 / 客戶端開發者 | 訂閱拉取介面（`/sub/:token`）：格式、請求頭、狀態碼、示例 |
| [portal-api.md](./portal-api.md) | 門戶前端 / 第三方客戶端 | 客戶端門戶 API（`/api/v1/client-auth/*`、`/api/v1/public/*`）：註冊登入、套餐、訂單、餘額、工單等 |
| [charge-callback.md](./charge-callback.md) | 支付渠道對接方 | 支付回撥簽名與驗籤 |
| [admin-api.md](./admin-api.md) | 面板二開 / 自動化運維 | 管理端 API 全量路由表（`/api/v1/*`，按模組分組） |

---

## 1. 服務埠與 Base URL

面板可同時暴露三種引擎，共享同一資料庫：

| 引擎 | 預設訪問方式 | 用途 |
|---|---|---|
| 主面板 | `https://<域名>/api/v1` | 管理後臺 + 門戶 + 訂閱 |
| 訂閱引擎（獨立埠） | `https://<域名>:<埠>/:token` | 高併發訂閱拉取，無 `/sub` 字首 |
| 門戶引擎（獨立埠） | `https://<門戶域名>[:port]/api/v1` | 純客戶端門戶，頁面在根路徑 `/` |

> 反向代理需同時轉發頁面路徑與 `/api/v1`；支援 `X-Forwarded-Proto` / `X-Forwarded-Host` 頭還原真實協議與域名。

## 2. 認證方式（三類，互不通用）

| 認證 | 憑據 | 適用 |
|---|---|---|
| **管理員認證** | `Authorization: Bearer <admin_token>`（`/api/v1/auth/login` 獲取） | 管理端 API |
| **客戶端認證** | `Authorization: Bearer <client_token>`（`/api/v1/client-auth/login` 獲取） | 門戶 API |
| **訂閱 Token** | URL 路徑中的 token：`/sub/<token>` | 訂閱拉取，無需 Header |

- 客戶端 access token 有效期 24h，refresh token 7 天；JWT 含 `tv`（token_version），登出/踢下線後舊 token 全部失效。
- 管理端角色：`admin` / `operator` / `super_admin`，部分寫操作要求 `RequireRole(RoleAdmin)` 以上。

## 3. 響應格式約定

成功（多數端點）：

```json
{ "code": 0, "data": { ... }, "message": "ok" }
```

列表端點（部分）返回 `{ "items": [...], "total": N }` 或裸陣列（下文逐個標註）。

錯誤（HTTP 4xx/5xx）：

```json
{ "code": "ERROR_CODE", "message": "人類可讀描述", "request_id": "…", "can_retry": false }
```

**金額單位統一為整數分**：`990` = ¥9.90（`currency: "CNY"`）。
**流量單位統一為位元組**：介面返回與請求均為 bytes。

## 4. 限流與維護模式

- 公共端點（`/api/v1/public/*`、`/sub/*`、註冊/登入）自帶限流；
- 登入另有 `LoginRateLimit`，訂閱拉取有 `SubscribeRateLimit`；
- 維護模式開啟時公共端點與訂閱返回維護提示，管理端不受影響。

## 5. 前端頁面路由（非 API）

以下路由返回 HTML/靜態資源，不在 API 文件範圍內，列出以便聯調時排除：

| 方法 | 路徑 | 說明 |
|---|---|---|
| GET | `/` | 管理端面板 SPA（登入後） |
| GET | `/p` | 門戶頁 HTML（獨立門戶引擎為 `/`） |
| GET | `/:token` | 獨立訂閱埠的訂閱拉取（無 `/sub` 字首，語義同 `/sub/:token`） |
| GET | `/static/:file` | 前端靜態資源 |

## 6. 路由權威來源

`internal/api/router/router.go`（共 746 條路由，其中 743 條已文件化）。[admin-api.md](./admin-api.md) 的路由表由程式碼提取生成；如與程式碼不一致，**以程式碼為準**。
