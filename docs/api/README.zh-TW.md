<!-- source: README.md | locale: zh-TW | updated: 2026-11-16 -->
# CoreFusion API 文檔

> 語言： [简体中文](README.md) · [English](README.en.md) · 繁體中文 · [Русский](README.ru.md) · [日本語](README.ja.md) · [한국어](README.ko.md) · [Deutsch](README.de.md) · [Français](README.fr.md) · [Español](README.es.md) · [Português](README.pt.md) · [Türkçe](README.tr.md) · [Bahasa Indonesia](README.id.md) · [ไทย](README.th.md) · [Tiếng Việt](README.vi.md) · [العربية](README.ar.md) · [فارسی](README.fa.md)

API 用於**外部整合商、二次開發和第三方用戶端**的文件。所有路徑都相對於部署域。

| 文件 | 觀眾 | 內容 |
|---|---|---|
| [訂閱.md](./subscription.zh-TW.md) | 訂閱者/客戶端開發者 | 訂閱端點 (`/sub/:token`)：格式、標頭、狀態代碼、範例 |
| [入口網站-api.md](./portal-api.md) | 入口網站前端/第三方用戶端 | 客戶端API（`/api/v1/client-auth/*`、`/api/v1/public/*`）：註冊、登入、計畫、訂單、餘額、門票 |
| [收費回檔.md](./charge-callback.zh-TW.md) | 支付提供者 | 支付回調簽名和驗證 |
| [admin-api.en.md](./admin-api.zh-TW.md) | 面板開發/自動化 | 完整管理 API 路由表（`/api/v1/*`，依模組分組） |

---

## 1. 連接埠和基本 URL

此面板可以同時暴露三個引擎，共享一個資料庫：

| 引擎 | 預設訪問 | 目的 |
|---|---|---|
| 主面板 | `https://<domain>/api/v1` | 管理後台+門戶+訂閱 |
| 訂閱引擎（專用連接埠） | `https://<domain>:<port>/:token` | 高並發訂閱抓取，無`/sub`前綴 |
| 門戶引擎（專用連接埠） | `https://<portal-domain>[:port]/api/v1` | 僅限客戶端的門戶，頁面位於 `/` |

> 反向代理必須轉發頁面路徑和`/api/v1`； `X-Forwarded-Proto` / `X-Forwarded-Host`很榮幸還原真實的方案和網域。

## 2. 認證（三種，不可互換）

| 授權 | 憑證 | 適用於 |
|---|---|---|
| **管理员授权** | `Authorization: Bearer <admin_token>`（來自`/api/v1/auth/login`） | 管理员 API |
| **客户端授权** | `Authorization: Bearer <client_token>`（來自`/api/v1/client-auth/login`） | 门户 API |
| **订阅令牌** | URL中的代幣：`/sub/<token>` | 订阅获取，无标题 |

- 用戶端存取令牌持續 24 小時，刷新令牌 7 天； JWT 包含 `tv` (token_version)，因此註銷/強制註銷會使所有舊令牌失效。
- 管理員角色等級：`viewer(10) < operator(20) < admin(30) < super_admin(40)`。讀取通常只需要登入工作階段；每個模組寫入需要 `operator` / `admin` / `super_admin`。請參閱[admin-api.md](./admin-api.zh-TW.md)中的「權限」欄位。

## 3. 回應格式

成功（大多數端點）：

```json
{ "code": 0, "data": { ... }, "message": "ok" }
```

列表端點（某些）傳回 `{ "items": [...], "total": N }` 或裸數組（下面按端點註明）。

錯誤（HTTP 4xx/5xx）：

```json
{ "code": "ERROR_CODE", "message": "human-readable description", "request_id": "…", "can_retry": false }
```

**金額為以分為單位的整數**：`990` = ¥9.90 (`currency: "CNY"`)。
**流量以位元組為單位**：請求和回應都使用位元組。

## 4. 速率限制和維護模式

- 公共端點（`/api/v1/public/*`、`/sub/*`、註冊/登入）有速率限制；
- 登入也使用`LoginRateLimit`，訂閱取得使用`SubscribeRateLimit`；
- 在維護模式下，公共端點和訂閱返回維護訊息；管理員 API 不受影響。

## 5. 前端頁面路由（不是 API）

這些路由會傳回 HTML/靜態資源，並且位於 API 文件之外；列出以便在整合過程中將其排除：

| 方法 | 路徑 | 描述 |
|---|---|---|
| GET | `/` | 管理面板 SPA（登入後） |
| GET | `/p` | 門戶 HTML（專用入口網站引擎上的 `/`） |
| GET | `/:token` | 在專用連接埠上取得訂閱（無 `/sub` 前綴，與 `/sub/:token` 語意相同） |
| GET | `/static/:file` | 前端靜態資產 |

## 6. 真相來源

`internal/api/router/router.go`（數量因版本而異；代碼具有權威性）。 [admin-api.md](./admin-api.md)中的路由表是由程式碼產生的；如果它與程式碼不一致，**程式碼獲勝**。
