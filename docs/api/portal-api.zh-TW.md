<!-- source: portal-api.md | locale: zh-TW | updated: 2026-11-16 -->
# 入口網站 API 參考（客戶端/入口網站 API）

> 語言： [简体中文](portal-api.md) · [English](portal-api.en.md) · 繁體中文 · [Русский](portal-api.ru.md) · [日本語](portal-api.ja.md) · [한국어](portal-api.ko.md) · [Deutsch](portal-api.de.md) · [Français](portal-api.fr.md) · [Español](portal-api.es.md) · [Português](portal-api.pt.md) · [Türkçe](portal-api.tr.md) · [Bahasa Indonesia](portal-api.id.md) · [ไทย](portal-api.th.md) · [Tiếng Việt](portal-api.vi.md) · [العربية](portal-api.ar.md) · [فارسی](portal-api.fa.md)

> 入口網站前端（`/p` 或專用入口網站連接埠）和第三方用戶端的**完整**介面參考。
> 路由定義：`internal/api/router/router.go` (`clientAuthGroup` / `clientAuthProtected` / `/api/v1/public/*`)。

## 1. 一般約定

### 1.1 底座 URL

| 部署 | 底座 URL |
|---|---|
| 主面板引擎 | `https://<panel-domain>/api/v1`（入口網頁位於`/p`） |
| 專用入口網站連接埠 | `https://<portal-domain>[:port]/api/v1`（入口網頁位於`/`） |
| 專用門戶域+反向代理 | 同上；代理必須轉送頁面路徑和 `/api/v1` |

### 1.2 認證

- 登入後發送`Authorization: Bearer <access_token>`；
- 訪問令牌最近 24 小時，刷新令牌 7 天；
- JWT 包含 `tv` (token_version)：註銷/強制註銷會增加它並使所有舊令牌失效
 （`clients.token_version`，見`internal/service/clientauth/service.go`）；
- 每個受保護端點都經過`ClientAuthMiddleware`（版本檢查+維護模式防護）。

### 1.3 響應包絡

大多數端點返回`{ "code": 0, "data": ..., "message": "..." }`；
某些清單端點傳回 `{ "items": [...], "total": N }` 或裸數組（下方按端點註明）。
錯誤返回`{ "code": "ERROR_CODE", "message": "...", "request_id": "...", "can_retry": ... }`
（HTTP 4xx/5xx）。

### 1.4 金額單位

訂單/餘額欄位為 **整數分**：`990` = ¥9.90 (`currency: "CNY"`)。

### 1.5 真實範例（登入→取得個人資料）

```bash
# Register (when registration_enabled=true)
curl -X POST https://<portal-domain>/api/v1/client-auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","password":"plaintext-password","referral_code":""}'
# → 201 {"data":{"access_token":"<JWT>","refresh_token":"<JWT>","client":{...}}}

# Sign in
curl -X POST https://<portal-domain>/api/v1/client-auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","password":"plaintext-password"}'
# → 200 same as above

# Fetch the current client profile (with token)
curl https://<portal-domain>/api/v1/client-auth/me \
  -H 'Authorization: Bearer <access_token>'
# → 200 {"data":{"id":27,"email":"user@example.com","nickname":"","status":"active","quota_bytes":10737418240,"used_bytes":0,...}}

# Fetch a subscription (no header, token in the path)
curl 'https://<portal-domain>/sub/<subscription_token>?format=mihomo_yaml'
# → 200 Mihomo YAML (or base64_uri / singbox_json / xray_json)

# Error example: expired token
# → 401 {"code":"errClientTokenRevoked","message":"...","request_id":"...","can_retry":false}
```

在401上，撥打`POST /api/v1/client-auth/refresh`一次，然後重試；如果仍然失敗，請清除本機會話（入口網站範本已執行此操作）。

---

## 2. 公共端點（無需登入）

| 方法 | 路徑 | 描述 |
|---|---|---|
| GET | `/p`（專用入口網站引擎上的`/`） | Portal HTML（伺服器渲染 + Vue 運行時） |
| GET | `/api/v1/public/portal-config` | 入口網站執行時間設定：網站名稱、主題顏色、導覽、模組、語言套件、`registration_enabled`、`regions`（登入區域來源）、`privacy_policy_url` / `terms_of_service_url` / `privacy_policy_content` / `terms_of_service_content`（合規性的內容； |
| GET | `/api/v1/public/settings` | 公共設定（目前為`registration_enabled`） |
| GET | `/api/v1/public/subscription-endpoints` | 訂閱端點清單（裸數組） |
| GET | `/api/v1/public/articles` | 公開文章（僅限發表） |
| GET | `/api/v1/public/articles/:slug` | 文章詳情 |
| GET | `/api/v1/public/client-configs` | 客戶端下載配置 |
| GET | `/sub/:token` | 訂閱交付（`format`返回base64_uri / raw_uri / mihomo_yaml / singbox_json / xray_json等；撤銷/遺失→404，暫停/過期/無效→403。請參閱[subscription.mdZX004004210404004140404040404004214040042214040042） |
| GET | `/api/v1/client-auth/oauth/providers` | OAuth 提供者可見性：`{"data":{"providers":[{"provider":"github","enabled":true},...]}}` |
| GET | `/api/v1/system/maintenance` | 維護狀態（公共）。啟用後，入口網站會顯示維護頁面 |

> 所有這些都包括維護防護和公共 API 速率限制。

## 3. 身份驗證/OAuth

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| POST | `/api/v1/client-auth/register` | — | 註冊。機身：`{email, password, referral_code?}`。返回`{access_token, refresh_token, client}`；註冊率有限 |
| POST | `/api/v1/client-auth/login` | — | 登入。正文：`{email, password}`。相同的反應；使用 `token_version` 發出 JWT |
| POST | `/api/v1/client-auth/refresh` | — | 刷新。機身：`{refresh_token}`。返回新的存取/刷新令牌（已檢查版本；舊的刷新令牌在登出後立即失效） |
| POST | `/api/v1/client-auth/logout` | ✓ | 登出：伺服器端撤銷（`token_version +1`）→客戶端的所有舊令牌均無效 |
| POST | `/api/v1/client-auth/rotate-token` | ✓ | 輪換訂閱令牌 |
| POST | `/api/v1/client-auth/ticket` | ✓ | 發出**一次性短期票據**（對於 `window.open` 串流，例如無法設定標頭的「下載日誌包」） |
| GET | `/api/v1/client-auth/oauth/:provider` | — | 啟動OAuth：307→GitHub/Google同意（`state`抗CSRF，10分鐘）；殘疾提供者返回 400 |
| GET | `/api/v1/client-auth/oauth/:provider/callback` | — | OAuth 回呼：交換代碼 → 建立/連結客戶端 → 發布版本 JWT → 302 返回入口網站 (`/<portalPath>?access_token=...&refresh_token=...`) |

> OAuth 回呼是**瀏覽器重定向**（不是 API 呼叫）：成功登入後，入口網站讀取令牌
> 來自 `onMounted` 中的 URL（或透過同源 `postMessage`，請參閱 `web` 入口網站範本中的 `handleOAuthCallback`）。

## 4. 帳戶

| 方法 | 路徑 | 描述 |
|---|---|---|
| GET | `/api/v1/client-auth/me` | 目前客戶：`{id, email, plan_id, plan_name, status, quota_bytes, used_bytes, upload_bytes, download_bytes, expires_at, ip_whitelist, device_limit, sub_token, sub_status, created_at, ...}`（`data`信封） |
| PUT | `/api/v1/client-auth/settings` | 更新客戶端自助服務設定：正文：`{ip_whitelist, device_limit}`；空的 `ip_whitelist` 表示沒有來源 IP 限制 |
| POST | `/api/v1/client-auth/change-password` | 本體：`{old_password, new_password}` |
| GET | `/api/v1/client-auth/checktime` | 伺服器時間（客戶端時鐘同步） |
| GET | `/api/v1/client-auth/connection-info` | 目前規劃連接參數：`{concurrent, device_limit, protocol, speed_limit, tls, transport}` |

## 5. 訂閱

| 方法 | 路徑 | 描述 |
|---|---|---|
| POST | `/api/v1/client-auth/reset-subscription` | 重置訂閱；這會旋轉 `sub_token` |

> 訂閱連結建立在`/me`（`/sub/:token`）返回的`sub_token`的前端上；沒有
> 不再單獨的 `subscription-link` / `subscription-formats` 端點。

## 6. 節點

| 方法 | 路徑 | 描述 |
|---|---|---|
| GET | `/api/v1/client-auth/nodes` | 目前計劃可見的節點：`{items:[...]}`（顯示名稱/協定/傳輸/區域等欄位） |
| GET | `/api/v1/client-auth/nodes/:id/config` | 全節點配置 |
| GET | `/api/v1/client-auth/nodes/:id/link` | 分享連結（vless://等） |
| GET | `/api/v1/client-auth/nodes/:id/qrcode` | QR 圖碼圖片 |

## 7. 商店/計劃/訂單

| 方法 | 路徑 | 描述 |
|---|---|---|
| GET | `/api/v1/client-auth/plans` | 計畫清單（裸陣列）：`{id, name, description, category, sort_order, variants:[{id, name, billing_cycle, duration_days, price(cents), original_price, currency, quota_bytes, ...}]}` |
| GET | `/api/v1/client-auth/plans/:id` | 計劃細節 |
| GET | `/api/v1/client-auth/traffic-packs` | 流量包列表 |
| POST | `/api/v1/client-auth/traffic-packs/buy` | 購買流量包 |
| GET | `/api/v1/client-auth/orders` | 我的計劃訂單（分頁） |
| POST | `/api/v1/client-auth/orders` | 建立訂單（購買/續訂計劃變體） |
| GET | `/api/v1/client-auth/orders/:id` | 訂單詳情 |
| GET | `/api/v1/client-auth/invoices` | 發票 |
| GET | `/api/v1/client-auth/invoices/:id` | 發票詳細信息 |

## 8. 餘額/儲值

| 方法 | 路徑 | 描述 |
|---|---|---|
| GET | `/api/v1/client-auth/balance` | `{data:{client_id, balance(cents), frozen}}` |
| GET | `/api/v1/client-auth/balance/transactions` | 餘額交易（`type`：儲值/消費/退款；分頁） |
| GET | `/api/v1/client-auth/charge/sites` | 可用儲值站點（`ListChargeSites`：僅啟用） |
| POST | `/api/v1/client-auth/charge/create` | 建立儲值訂單（重新導向至第三方付款） |
| GET | `/api/v1/client-auth/charge/orders` | 儲值訂單列表 |
| GET | `/api/v1/client-auth/charge/orders/:id` | 儲值訂單詳情 |
| GET | `/api/v1/client-auth/charge/status/:trade_no` | 按 trade_no 輪詢付款狀態 |
| POST | `/api/v1/client-auth/charge/redeem-card` | 兌換儲值卡 |

> 刪除客戶端**級聯**到其餘額和交易（請參閱 `internal/service/client/service.go` 中的 `DeleteClient`）。

## 9. 禮物/交通紀錄

| 方法 | 路徑 | 描述 |
|---|---|---|
| GET | `/api/v1/client-auth/gifts` | 可領取的禮物（裸數組；`type`：天/字節） |
| POST | `/api/v1/client-auth/gifts/:id/claim` | 索賠 |
| GET | `/api/v1/client-auth/traffic-history` | 每月流量歷史記錄：`{data:{history:[{period_start,total_bytes,...}], months}}` |

## 10. 門票

| 方法 | 路徑 | 描述 |
|---|---|---|
| GET | `/api/v1/client-auth/tickets` | 門票清單：`{items,total}` |
| POST | `/api/v1/client-auth/tickets` | 創建票證 |
| GET | `/api/v1/client-auth/tickets/:id` | 票證詳細資料（含回覆） |
| POST | `/api/v1/client-auth/tickets/:id/reply` | 回覆 |
| POST | `/api/v1/client-auth/tickets/:id/close` | 關閉 |

## 11. 文章/指南

| 方法 | 路徑 | 描述 |
|---|---|---|
| GET | `/api/v1/client-auth/articles` | 文章列表（帶類別過濾器） |
| GET | `/api/v1/client-auth/articles/:slug` | 文章詳情 |

## 12. 建議（預設為停用；可在入口網站編輯器中啟用）

| 方法 | 路徑 | 描述 |
|---|---|---|
| GET | `/api/v1/client-auth/referral/code` | 我的推薦碼和鏈接 |
| GET | `/api/v1/client-auth/referral/stats` | 佣金統計（`commission_rate`、`available_commission`、`min_withdraw_amount`） |
| GET | `/api/v1/client-auth/referral/records` | 推薦記錄 |
| GET | `/api/v1/client-auth/referral/withdrawals` | 提幣記錄 |
| POST | `/api/v1/client-auth/referral/withdraw` | 請求提款 |

## 13. 共享帳戶

| 方法 | 路徑 | 描述 |
|---|---|---|
| GET | `/api/v1/client-auth/shared-accounts` | 可用的共享帳戶（串流媒體等） |
| GET | `/api/v1/client-auth/shared-accounts/:id` | 詳情 |
| POST | `/api/v1/client-auth/shared-accounts/:id/apply` | 請求存取權限 |
| POST | `/api/v1/client-auth/shared-accounts/:id/repair` | 一鍵修復 |

## 14. 日誌包（訂閱者診斷）

| 方法 | 路徑 | 描述 |
|---|---|---|
| GET | `/api/v1/client-auth/logs/types` | 可捆綁的日誌類型 |
| GET | `/api/v1/client-auth/logs/packages` | 我的日誌包 |
| POST | `/api/v1/client-auth/logs/package` | 建立日誌捆綁作業（僅限入口節點） |
| GET | `/api/v1/client-auth/logs/packages/:id` | 作業狀態（就緒/處理中/失敗） |
| GET | `/api/v1/client-auth/logs/packages/:id/download` | 下載（首選`POST /ticket`並附加`?ticket=`） |
| DELETE | `/api/v1/client-auth/logs/packages/:id` | 刪除日誌包 |

## 15. 安全設定/IP白名單/自行解禁

| 方法 | 路徑 | 描述 |
|---|---|---|
| GET/PUT | `/api/v1/client-auth/settings` | 客戶端設定：`{ip_whitelist, device_limit}`（PUT） |
| GET | `/api/v1/client-auth/settings/whitelist` | 允許列表（裸數組） |
| DELETE | `/api/v1/client-auth/settings/whitelist/:id` | 刪除允許清單條目 |
| POST | `/api/v1/client-auth/settings/auto-add-ip` | 將目前IP加入白名單 |
| POST | `/api/v1/client-auth/self-unban` | 自助解禁（受政策限制嘗試） |

> IP 允許清單**僅適用於訂閱 URL** (`/sub/:token`)；它不會阻止入口網站登入/OAuth 或
> 其他客戶端身份驗證端點。如果使用者沒有允許清單或沒有訂閱 URL，則始終允許存取。的
> 允許列表是每個使用者的設定；沒有全域開關。

**自行解禁流量**：僅適用於`auto_quota`（流量超限自動解禁），管理端
`self_unban.enabled` 已開啟。登入（密碼或 OAuth 回呼）返回 `require_self_unban=true` + `self_unban_token`
（typ=self_unban，有效期15分鐘，**僅**在`POST /self-unban`上可用；其他端點返回403/`SELF_UNBAN_TOKEN_SCOPE`）。
然後入口網站會顯示解封卡；成功解禁後，用戶再次登入。 OAuth 回呼透過以下方式傳遞此訊息
URL 片段 `#require_self_unban=1&self_unban_token=..&self_unban_hint=..`。

## 16. 錯誤代碼（摘錄）

| 程式碼 | 場景 |
|---|---|
| `INVALID_TOKEN` | 刷新令牌無效/過期/版本不匹配 |
| `INVALID_REQUEST` | 參數驗證失敗 |
| `NOT_FOUND` | 資源缺失（票據/訂單/允許清單條目等） |
| `RELOAD_FAILED` / `SYNC_FAILED` | （管理員端）代理程式配置同步失敗 |
| 401+`errClientTokenRevoked` | token_version 不符（登出/踢出）→ 用戶端應該刷新；如果失敗，清除本機令牌 |

## 17. 合規文件（隱私權政策/服務條款）

登入卡底部的「隱私權政策·服務條款」連結（與一般網站相同，點選開啟內容）：

- **配置**：入口網站編輯器→設定→常規→合規性文件（**編輯每種語言的內容**）；
 您也可以將 `PUT /api/v1/portal/active` 與結構化欄位 `privacy_policy_url` / `privacy_policy_content` /
 `terms_of_service_url` / `terms_of_service_content`（nil = 不變；內容為 `{"zh-CN":"...","en":"..."}` 地圖）。
- **獨立可存取頁面**（伺服器渲染，方便Google驗證爬蟲；支援`?lang=xx`）：
  - 主機板域：`GET /p/legal/privacy`、`GET /p/legal/terms`
  - 專用門戶網域（根部署）：`GET /legal/privacy`、`GET /legal/terms`
- **顯示邏輯**：URL或內容，**URL獲勝**（在新分頁中外部開啟）；如果提供了內容，請按一下
 登入卡使用目前入口網站語言在對話方塊 (`renderMd`) 中呈現 Markdown。
- 運行時透過`/api/v1/public/portal-config`傳遞；登入組件消耗
 `ctx.privacyUrl / termsUrl / privacyContent / termsContent / openLegal / hasLegalLinks`。
- Google OAuth 驗證要求這些連結可公開訪問 → 使用上面的獨立鏈接
 同意畫面；請參閱 `docs/oauth-google-verification.md`（包含隱私權政策/條款範本）。

## 18. 整合範例

```js
// Sign in
const { data } = await fetch('/api/v1/client-auth/login', {
  method: 'POST', headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ email, password })
}).then(r => r.json());
localStorage.setItem('client_token', data.access_token);
localStorage.setItem('client_refresh_token', data.refresh_token);

// Authenticated request
const me = await fetch('/api/v1/client-auth/me', {
  headers: { Authorization: 'Bearer ' + data.access_token }
}).then(r => r.json());
```

401上，撥打`POST /refresh`一次，重試；如果仍然失敗，清除本地會話
（門戶模板的 `api()` 已經實現了這一點）。
