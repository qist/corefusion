<!-- source: portal-api.md | locale: zh-TW | updated: 2026-10-23 -->
# 門戶 API 參考（Client / Portal API）

> 語言： [简体中文](portal-api.md) · [English](portal-api.en.md) · 繁體中文 · [Русский](portal-api.ru.md) · [日本語](portal-api.ja.md) · [한국어](portal-api.ko.md) · [Deutsch](portal-api.de.md) · [Français](portal-api.fr.md) · [Español](portal-api.es.md) · [Português](portal-api.pt.md) · [Türkçe](portal-api.tr.md) · [Bahasa Indonesia](portal-api.id.md) · [ไทย](portal-api.th.md) · [Tiếng Việt](portal-api.vi.md) · [العربية](portal-api.ar.md) · [فارسی](portal-api.fa.md)

> 面向門戶前端（`/p` 或獨立門戶埠）與第三方客戶端的**完整**介面參考。
> 路由定義：`internal/api/router/router.go`（`clientAuthGroup` / `clientAuthProtected` / `/api/v1/public/*`）。

## 1. 通用約定

### 1.1 Base URL

| 部署形態 | Base URL |
|---|---|
| 主面板引擎 | `https://<面板域名>/api/v1`（門戶頁在 `/p`） |
| 獨立門戶埠 | `https://<門戶域名>[:port]/api/v1`（門戶頁在根路徑 `/`） |
| 獨立門戶域名反代 | 同上，反代需同時轉發頁面路徑與 `/api/v1` |

### 1.2 認證

- 登入後攜帶 `Authorization: Bearer <access_token>`；
- access token 有效期 24h，refresh token 7 天；
- JWT 內含 `tv`（token_version）：登出/踢下線後版本 +1，舊 token 全部失效
  （`clients.token_version`，見 `internal/service/clientauth/service.go`）；
- 所有受保護介面都經過 `ClientAuthMiddleware`（版本校驗 + 維護模式守衛）。

### 1.3 響應包裝

大多數端點返回 `{ "code": 0, "data": ..., "message": "..." }`；
部分列表端點直接返回 `{ "items": [...], "total": N }` 或裸陣列（下文逐個標註）。
錯誤返回 `{ "code": "ERROR_CODE", "message": "...", "request_id": "...", "can_retry": ... }`
（HTTP 4xx/5xx）。

### 1.4 金額單位

訂單/餘額類欄位為**整數分**：`990` = ¥9.90（`currency: "CNY"`）。

### 1.5 真實示例（登入 → 拉取資料）

```bash
# 註冊（若 registration_enabled=true）
curl -X POST https://<門戶域名>/api/v1/client-auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","password":"明文密碼","referral_code":""}'
# → 201 {"data":{"access_token":"<JWT>","refresh_token":"<JWT>","client":{...}}}

# 登入
curl -X POST https://<門戶域名>/api/v1/client-auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","password":"明文密碼"}'
# → 200 同上

# 拉取當前客戶資料（帶 token）
curl https://<門戶域名>/api/v1/client-auth/me \
  -H 'Authorization: Bearer <access_token>'
# → 200 {"data":{"id":27,"email":"user@example.com","nickname":"","status":"active","quota_bytes":10737418240,"used_bytes":0,...}}

# 訂閱拉取（無需 Header，token 在路徑裡）
curl 'https://<門戶域名>/sub/<subscription_token>?format=mihomo_yaml'
# → 200 Mihomo YAML（或 base64_uri / singbox_json / xray_json）

# 錯誤樣例：token 失效
# → 401 {"code":"errClientTokenRevoked","message":"...","request_id":"...","can_retry":false}
```

401 時先 `POST /api/v1/client-auth/refresh` 換新 token 重試一次，仍失敗則清除本地登入態（門戶模板已內建）。

---

## 2. 公開端點（無需登入）

| 方法 | 路徑 | 說明 |
|---|---|---|
| GET | `/p`（獨立門戶引擎為 `/`） | 門戶頁 HTML（服務端渲染 + Vue 執行時） |
| GET | `/api/v1/public/portal-config` | 門戶執行時配置：站點名、主題色、導航、模組、多語言包、`registration_enabled`、`regions`（登入區原始碼）、`privacy_policy_url` / `terms_of_service_url` / `privacy_policy_content` / `terms_of_service_content`（合規文件；內容按語言對映；獨立頁面 `/p/legal/*` ✓） |
| GET | `/api/v1/public/settings` | 公開設定（當前含 `registration_enabled`） |
| GET | `/api/v1/public/subscription-endpoints` | 訂閱接入點列表（裸陣列） |
| GET | `/api/v1/public/articles` | 公開文章列表（已釋出） |
| GET | `/api/v1/public/articles/:slug` | 文章詳情 |
| GET | `/api/v1/public/client-configs` | 客戶端下載配置 |
| GET | `/sub/:token` | 訂閱下發（按 `format` 引數返回 base64_uri / raw_uri / mihomo_yaml / singbox_json / xray_json 等；已吊銷/不存在返回 404，暫停/到期/未啟用返回 403。詳見 [subscription.md](./subscription.zh-TW.md)） |
| GET | `/api/v1/client-auth/oauth/providers` | OAuth 提供商顯隱：`{"data":{"providers":[{"provider":"github","enabled":true},...]}}` |
| GET | `/api/v1/system/maintenance` | 維護模式狀態（公開）。維護開啟時門戶顯示維護頁 |

> 以上均帶維護模式守衛與公共 API 限流。

## 3. 認證 / OAuth

| 方法 | 路徑 | 認證 | 說明 |
|---|---|---|---|
| POST | `/api/v1/client-auth/register` | — | 註冊。body：`{email, password, referral_code?}`。返回 `{access_token, refresh_token, client}`；受註冊限流 |
| POST | `/api/v1/client-auth/login` | — | 登入。body：`{email, password}`。返回同上；簽發帶 `token_version` |
| POST | `/api/v1/client-auth/refresh` | — | 續簽。body：`{refresh_token}`。返回新的 access/refresh（版本校驗，登出後舊 refresh 立即失效） |
| POST | `/api/v1/client-auth/logout` | ✓ | 登出：服務端吊銷（`token_version +1`）→ 該客戶所有舊 token 失效 |
| POST | `/api/v1/client-auth/rotate-token` | ✓ | 輪換訂閱 token |
| POST | `/api/v1/client-auth/ticket` | ✓ | 簽發**一次性短時票據**（門戶"下載日誌包"等 `window.open` 場景用，無法帶 Header） |
| GET | `/api/v1/client-auth/oauth/:provider` | — | 發起 OAuth 授權：307 → GitHub/Google 授權頁（`state` 防 CSRF，10 分鐘有效）；未啟用的 provider 返回 400 |
| GET | `/api/v1/client-auth/oauth/:provider/callback` | — | OAuth 回撥：換 code → 建/綁客戶 → 簽發帶版本的 JWT → 302 跳回門戶頁（`/<portalPath>?access_token=...&refresh_token=...`） |

> OAuth 回撥是**瀏覽器跳轉**（非 API 呼叫）：登入成功後門戶頁在 `onMounted` 裡
> 從 URL 收 token（或經同源 `postMessage`，見 `web` 門戶模板 `handleOAuthCallback`）。

## 4. 賬號

| 方法 | 路徑 | 說明 |
|---|---|---|
| GET | `/api/v1/client-auth/me` | 當前客戶：`{id, email, plan_id, plan_name, status, quota_bytes, used_bytes, upload_bytes, download_bytes, expires_at, ip_whitelist, device_limit, sub_token, sub_status, created_at, ...}`（`data` 包裝） |
| POST | `/api/v1/client-auth/change-password` | body：`{old_password, new_password}` |
| GET | `/api/v1/client-auth/checktime` | 伺服器時間（客戶端校時） |
| GET | `/api/v1/client-auth/connection-info` | 當前套餐連線引數：`{concurrent, device_limit, protocol, speed_limit, tls, transport}` |

## 5. 訂閱

| 方法 | 路徑 | 說明 |
|---|---|---|
| POST | `/api/v1/client-auth/reset-subscription` | 重置訂閱；重置會輪換 sub_token |

> 訂閱連結由 `/me` 返回的 `sub_token` 在前端拼接（`/sub/:token`），不再有獨立的
> `subscription-link` / `subscription-formats` 端點。

## 6. 節點

| 方法 | 路徑 | 說明 |
|---|---|---|
| GET | `/api/v1/client-auth/nodes` | 當前套餐可見節點：`{items:[...]}`（含名稱/協議/傳輸/地區等展示欄位） |
| GET | `/api/v1/client-auth/nodes/:id/config` | 節點完整配置 |
| GET | `/api/v1/client-auth/nodes/:id/link` | 分享連結（vless:// 等） |
| GET | `/api/v1/client-auth/nodes/:id/qrcode` | 二維碼圖片 |

## 7. 商店 / 套餐 / 訂單

| 方法 | 路徑 | 說明 |
|---|---|---|
| GET | `/api/v1/client-auth/plans` | 套餐列表（裸陣列）：`{id, name, description, category, sort_order, variants:[{id, name, billing_cycle, duration_days, price(分), original_price, currency, quota_bytes, ...}]}` |
| GET | `/api/v1/client-auth/plans/:id` | 套餐詳情 |
| GET | `/api/v1/client-auth/traffic-packs` | 流量包列表 |
| POST | `/api/v1/client-auth/traffic-packs/buy` | 購買流量包 |
| GET | `/api/v1/client-auth/orders` | 我的套餐訂單（分頁） |
| POST | `/api/v1/client-auth/orders` | 下單（購買/續費套餐變體） |
| GET | `/api/v1/client-auth/orders/:id` | 訂單詳情 |
| GET | `/api/v1/client-auth/invoices` | 賬單 |
| GET | `/api/v1/client-auth/invoices/:id` | 賬單詳情 |

## 8. 餘額 / 充值

| 方法 | 路徑 | 說明 |
|---|---|---|
| GET | `/api/v1/client-auth/balance` | `{data:{client_id, balance(分), frozen}}` |
| GET | `/api/v1/client-auth/balance/transactions` | 餘額流水（`type`: charge/consume/refund；分頁） |
| GET | `/api/v1/client-auth/charge/sites` | 可用充值站點（`ListChargeSites`：僅 enabled） |
| POST | `/api/v1/client-auth/charge/create` | 建立充值訂單（跳轉第三方支付） |
| GET | `/api/v1/client-auth/charge/orders` | 充值訂單列表 |
| GET | `/api/v1/client-auth/charge/orders/:id` | 充值訂單詳情 |
| GET | `/api/v1/client-auth/charge/status/:trade_no` | 按 trade_no 輪詢支付狀態 |
| POST | `/api/v1/client-auth/charge/redeem-card` | 卡密兌換（充值卡） |

> 客戶刪除時其餘額與流水**級聯刪除**（見 `internal/service/client/service.go DeleteClient`）。

## 9. 禮品 / 流量歷史

| 方法 | 路徑 | 說明 |
|---|---|---|
| GET | `/api/v1/client-auth/gifts` | 可領取禮品（裸陣列；`type`: days/bytes） |
| POST | `/api/v1/client-auth/gifts/:id/claim` | 領取 |
| GET | `/api/v1/client-auth/traffic-history` | 月度流量歷史：`{data:{history:[{period_start,total_bytes,...}], months}}` |

## 10. 工單

| 方法 | 路徑 | 說明 |
|---|---|---|
| GET | `/api/v1/client-auth/tickets` | 工單列表：`{items,total}` |
| POST | `/api/v1/client-auth/tickets` | 建立工單 |
| GET | `/api/v1/client-auth/tickets/:id` | 工單詳情（含回覆） |
| POST | `/api/v1/client-auth/tickets/:id/reply` | 回覆 |
| POST | `/api/v1/client-auth/tickets/:id/close` | 關閉 |

## 11. 文章 / 教程

| 方法 | 路徑 | 說明 |
|---|---|---|
| GET | `/api/v1/client-auth/articles` | 文章列表（含分類篩選） |
| GET | `/api/v1/client-auth/articles/:slug` | 文章詳情 |

## 12. 推廣邀請（預設關閉，門戶編輯器可開）

| 方法 | 路徑 | 說明 |
|---|---|---|
| GET | `/api/v1/client-auth/referral/code` | 我的邀請碼與邀請連結 |
| GET | `/api/v1/client-auth/referral/stats` | 佣金統計（`commission_rate`、`available_commission`、`min_withdraw_amount`） |
| GET | `/api/v1/client-auth/referral/records` | 邀請記錄 |
| GET | `/api/v1/client-auth/referral/withdrawals` | 提現記錄 |
| POST | `/api/v1/client-auth/referral/withdraw` | 申請提現 |

## 13. 共享賬戶

| 方法 | 路徑 | 說明 |
|---|---|---|
| GET | `/api/v1/client-auth/shared-accounts` | 可用共享賬號（流媒體等） |
| GET | `/api/v1/client-auth/shared-accounts/:id` | 詳情 |
| POST | `/api/v1/client-auth/shared-accounts/:id/apply` | 申請使用 |
| POST | `/api/v1/client-auth/shared-accounts/:id/repair` | 一鍵修復 |

## 14. 日誌包（訂閱使用者診斷）

| 方法 | 路徑 | 說明 |
|---|---|---|
| GET | `/api/v1/client-auth/logs/types` | 可打包的日誌型別 |
| GET | `/api/v1/client-auth/logs/packages` | 我的日誌包列表 |
| POST | `/api/v1/client-auth/logs/package` | 建立日誌打包任務（僅入口節點可用） |
| GET | `/api/v1/client-auth/logs/packages/:id` | 任務狀態（ready/processing/failed） |
| GET | `/api/v1/client-auth/logs/packages/:id/download` | 下載（建議先 `POST /ticket` 換一次性票據拼 `?ticket=`） |
| DELETE | `/api/v1/client-auth/logs/packages/:id` | 刪除日誌包 |

## 15. 安全設定 / IP 白名單 / 自解封

| 方法 | 路徑 | 說明 |
|---|---|---|
| GET/PUT | `/api/v1/client-auth/settings` | 客戶端設定：`{ip_whitelist, device_limit}`（PUT） |
| GET | `/api/v1/client-auth/settings/whitelist` | 白名單列表（裸陣列） |
| DELETE | `/api/v1/client-auth/settings/whitelist/:id` | 刪除白名單項 |
| POST | `/api/v1/client-auth/settings/auto-add-ip` | 把當前 IP 加入白名單 |
| POST | `/api/v1/client-auth/self-unban` | 自助解封（按策略限制次數） |

> IP 白名單**只作用於訂閱地址**（`/sub/:token`），不攔截門戶登入/OAuth 或其它
> client-auth 端點；使用者未配置白名單或沒有訂閱地址時一律放行。白名單是使用者級配置，
> 沒有全域性開關。

**自解封流程**：僅 `auto_quota`（流量超額自動封禁）+ 管理端 `self_unban.enabled` 開啟時可用。
登入（密碼或 OAuth 回撥）返回 `require_self_unban=true` + `self_unban_token`（typ=self_unban，
15 分鐘有效，**只能**調 `POST /self-unban`，訪問其它端點一律 403/`SELF_UNBAN_TOKEN_SCOPE`）；
門戶據此顯示解封卡，解封成功後重新登入。OAuth 回撥經 URL fragment 下發
`#require_self_unban=1&self_unban_token=..&self_unban_hint=..`。

## 16. 錯誤碼摘錄

| code | 場景 |
|---|---|
| `INVALID_TOKEN` | refresh token 無效/過期/版本不匹配 |
| `INVALID_REQUEST` | 引數校驗失敗 |
| `NOT_FOUND` | 資源不存在（工單/訂單/白名單等） |
| `RELOAD_FAILED` / `SYNC_FAILED` | （管理端）代理配置同步失敗 |
| 401 + `errClientTokenRevoked` | token_version 不匹配（已登出/被踢）→ 客戶端應走 refresh，失敗即清除本地 token |

## 17. 合規文件（隱私政策 / 服務條款）

登入卡片底部的"隱私政策 · 服務條款"連結（與常規站點一致，點選開啟條款內容）：

- **配置入口**：門戶編輯器 → 設定 → 基礎設定 → 合規文件（**按語言**分別編輯內容）；
  也可 `PUT /api/v1/portal/active` 傳結構化欄位 `privacy_policy_url` / `privacy_policy_content` /
  `terms_of_service_url` / `terms_of_service_content`（nil=不改；內容為 `{"zh-CN":"...","en":"..."}` 對映）。
- **獨立可訪問頁面**（服務端渲染，便於 Google 驗證抓取，支援 `?lang=xx`）：
  - 主面板域名：`GET /p/legal/privacy`、`GET /p/legal/terms`
  - 獨立門戶域名（根路徑部署）：`GET /legal/privacy`、`GET /legal/terms`
- **展示邏輯**：連結與內容二選一，**URL 優先**（新標籤開啟外鏈）；填了內容則登入卡片點選後
  彈窗渲染 Markdown（`renderMd`），按當前門戶語言挑選內容。
- 執行時透過 `/api/v1/public/portal-config` 下發，登入區元件經 `ctx.privacyUrl / termsUrl /
  privacyContent / termsContent / openLegal / hasLegalLinks` 消費。
- Google OAuth 正式授權要求這些連結公共可訪問 → 用上面的獨立連結填入同意螢幕；
  見 `docs/oauth-google-verification.md`（內含隱私政策/服務條款模板文字）。

## 18. 接入示例

```js
// 登入
const { data } = await fetch('/api/v1/client-auth/login', {
  method: 'POST', headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ email, password })
}).then(r => r.json());
localStorage.setItem('client_token', data.access_token);
localStorage.setItem('client_refresh_token', data.refresh_token);

// 帶 token 請求
const me = await fetch('/api/v1/client-auth/me', {
  headers: { Authorization: 'Bearer ' + data.access_token }
}).then(r => r.json());
```

401 時先 `POST /refresh` 換新 token 重試一次，仍失敗則清除本地登入態
（門戶模板 `api()` 已內建該流程）。
