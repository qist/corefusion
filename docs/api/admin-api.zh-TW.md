<!-- source: admin-api.md | locale: zh-TW | updated: 2026-10-23 -->
# 管理端 API（Admin API）

> 語言： [简体中文](admin-api.md) · [English](admin-api.en.md) · 繁體中文 · [Русский](admin-api.ru.md) · [日本語](admin-api.ja.md) · [한국어](admin-api.ko.md) · [Deutsch](admin-api.de.md) · [Français](admin-api.fr.md) · [Español](admin-api.es.md) · [Português](admin-api.pt.md) · [Türkçe](admin-api.tr.md) · [Bahasa Indonesia](admin-api.id.md) · [ไทย](admin-api.th.md) · [Tiếng Việt](admin-api.vi.md) · [العربية](admin-api.ar.md) · [فارسی](admin-api.fa.md)

面向**面板二開 / 自動化運維 / 對接方**的管理端介面全量路由表。

- Base URL：`https://<面板域名>/api/v1`
- 認證：`Authorization: Bearer <admin_token>`（`POST /api/v1/auth/login` 獲取）
- 通用響應/錯誤格式、限流約定見 [README.md](./README.zh-TW.md)
- 路由權威來源：`internal/api/router/router.go`（共 739 條管理端路由）；與本表不一致時**以程式碼為準**

## 許可權標記

| 標記 | 含義 |
|---|---|
| 公開 | 無需登入（仍可能有維護模式守衛 / 限流） |
| 公開·限流 | 無需登入，且帶專用限流 |
| 登入 | 需已登入的管理端 token（`AuthRequired`） |
| 管理員 | 需角色 ≥ `admin`（`RequireRole(RoleAdmin)`） |
| 超級管理員 | 需角色 = `super_admin` |

角色等級：`viewer(10) < operator(20) < admin(30) < super_admin(40)`；不滿足返回 403 `INSUFFICIENT_ROLE`。

---

## 快速開始（真實示例）

以下示例均取自本機真實響應，token 與資料已脫敏。

### 1. 登入獲取 token

```bash
curl -X POST https://<面板域名>/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"你的密碼"}'
```

響應：

```json
{
  "data": {
    "access_token": "<JWT，有效期 24h>",
    "refresh_token": "<JWT，有效期 7 天>",
    "user": { "id": 1, "role": "super_admin", "username": "admin" }
  }
}
```

### 2. 帶 token 呼叫業務介面

```bash
curl https://<面板域名>/api/v1/clients?limit=1 \
  -H 'Authorization: Bearer <access_token>'
```

響應（列表統一 `items` + `total` 結構）：

```json
{
  "data": {
    "items": [
      {
        "id": 27,
        "email": "user@example.com",
        "status": "active",
        "quota_bytes": 10737418240,
        "used_bytes": 0,
        "plan_id": 1,
        "expires_at": null,
        "created_at": "2026-09-30T15:54:32+08:00"
      }
    ],
    "total": 22,
    "limit": 1,
    "offset": 0
  }
}
```

### 3. 錯誤響應樣例

**401 未認證：**

```json
{
  "code": "MISSING_TOKEN",
  "message": "missing authorization header",
  "request_id": "a02cfdf9-6be0-46be-8584-824caab34dc7",
  "can_retry": false,
  "requires_action": false
}
```

**404 資源不存在：**

```json
{
  "code": "CLIENT_NOT_FOUND",
  "message": "client not found",
  "request_id": "099949bd-4ded-468d-a6f8-f8e798af2273",
  "can_retry": false,
  "requires_action": false
}
```

### 4. 分頁約定

所有列表介面統一使用 `limit`（每頁條數，預設 20）與 `偏移量 offset`（從 0 開始），返回 `{items, total, limit, offset}`。部分舊介面返回裸陣列（見各表標註）。

---

## 目錄

- [管理員認證與安全](#管理員認證與安全)
- [管理員賬號](#管理員賬號)
- [客戶管理](#客戶管理)
- [套餐模板](#套餐模板)
- [套餐變體](#套餐變體)
- [流量包](#流量包)
- [優惠碼](#優惠碼)
- [節點分組](#節點分組)
- [節點伺服器](#節點伺服器)
- [節點審計日誌](#節點審計日誌)
- [節點（舊版模型，已下線）](#節點舊版模型已下線)
- [地區總覽](#地區總覽)
- [訂閱](#訂閱)
- [訂閱格式](#訂閱格式)
- [訂閱接入點](#訂閱接入點)
- [分享域名](#分享域名)
- [Cloudflare IP](#cloudflare-ip)
- [訂閱配置（Mihomo/Clash）](#訂閱配置mihomoclash)
- [充值站點](#充值站點)
- [充值訂單](#充值訂單)
- [充值卡密](#充值卡密)
- [支付回撥事件](#支付回撥事件)
- [支付回撥重試](#支付回撥重試)
- [客戶餘額](#客戶餘額)
- [禮品](#禮品)
- [推廣配置](#推廣配置)
- [推廣提現](#推廣提現)
- [IP 白名單（彙總）](#ip-白名單彙總)
- [客戶 OAuth 繫結](#客戶-oauth-繫結)
- [工單（管理端）](#工單管理端)
- [文章](#文章)
- [共享賬號](#共享賬號)
- [證書](#證書)
- [證書通道](#證書通道)
- [EAB 配置](#eab-配置)
- [ACME 賬號](#acme-賬號)
- [內建 CA](#內建-ca)
- [Geo 規則](#geo-規則)
- [WAF 規則庫](#waf-規則庫)
- [日誌管理](#日誌管理)
- [備份](#備份)
- [系統設定](#系統設定)
- [安全審計](#安全審計)
- [指標](#指標)
- [門戶模板](#門戶模板)
- [Agent 安裝](#agent-安裝)
- [支付回撥入口](#支付回撥入口)
- [入站（含節點）](#入站含節點)
- [公開配置](#公開配置)
- [客戶認證與業務（管理端引擎上的同一套）](#客戶認證與業務管理端引擎上的同一套)
- [訂閱下發](#訂閱下發)
- [執行時配置工作流](#執行時配置工作流)
- [服務訪問策略（已下線）](#服務訪問策略已下線)
- [流量配額監控](#流量配額監控)
- [健康檢查](#健康檢查)
- [門戶頁](#門戶頁)
- [ACME 挑戰](#acme-挑戰)

---

### 管理員認證與安全

面板管理端的登入、登出、改密、2FA（TOTP）、Passkey（WebAuthn）與一次性票據。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| POST | `/api/v1/auth/2fa/disable` | 登入 | 禁用 |
| POST | `/api/v1/auth/2fa/enable` | 登入 | 啟用 |
| POST | `/api/v1/auth/2fa/recovery-codes` | 登入 | 重新生成恢復碼 |
| POST | `/api/v1/auth/2fa/setup` | 登入 | 設定 |
| GET | `/api/v1/auth/2fa/status` | 登入 | 狀態 |
| POST | `/api/v1/auth/change-password` | 登入 | 修改密碼 |
| POST | `/api/v1/auth/login` | 公開 | 登入 |
| POST | `/api/v1/auth/login/2fa` | 公開 | 動態碼登入（2FA） |
| GET | `/api/v1/auth/login/2fa-status` | 公開 | 查詢賬號是否啟用 2FA |
| POST | `/api/v1/auth/logout` | 登入 | 登出 |
| GET | `/api/v1/auth/me` | 登入 | 當前登入使用者 |
| GET | `/api/v1/auth/passkey/credentials` | 登入 | Passkey 憑據列表 |
| DELETE | `/api/v1/auth/passkey/credentials/:id` | 登入 | 刪除 Passkey 憑據 |
| POST | `/api/v1/auth/passkey/login/begin` | 公開 | 登入發起 |
| POST | `/api/v1/auth/passkey/login/finish` | 公開 | 登入完成 |
| POST | `/api/v1/auth/passkey/register/begin` | 登入 | 註冊發起 |
| POST | `/api/v1/auth/passkey/register/finish` | 登入 | 註冊完成 |
| GET | `/api/v1/auth/passkey/rpids` | 登入 | RP ID 白名單列表 |
| POST | `/api/v1/auth/passkey/rpids` | 登入 | 更新 RP ID 白名單 |
| GET | `/api/v1/auth/passkey/status` | 公開 | 狀態 |
| POST | `/api/v1/auth/refresh` | 公開 | 重新整理 |
| POST | `/api/v1/auth/ticket` | 登入 | 簽發一次性短時票據 |

### 管理員賬號

管理面板使用者賬號。列表需管理員，增刪改僅超級管理員（防止普通管理員自我提權）。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| GET | `/api/v1/users` | 管理員 | 列表 |
| POST | `/api/v1/users` | 超級管理員 | 建立 |
| DELETE | `/api/v1/users/:id` | 超級管理員 | 刪除 |
| PUT | `/api/v1/users/:id` | 超級管理員 | 更新 |

### 客戶管理

訂閱客戶（Portal 賬號）的增刪改查、暫停/恢復、升降級、續費、用量重置。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| GET | `/api/v1/clients` | 登入 | 列表客戶 |
| POST | `/api/v1/clients` | 登入 | 建立客戶。body：`{"email":"user@example.com","password":"明文密碼","plan_id":1,"quota_bytes":10737418240,"expires_at":"2026-12-31T00:00:00Z","remark":""}`。`plan_id` 必填，`password` 可空（客戶後續在門戶自設） |
| DELETE | `/api/v1/clients/:id` | 登入 | 刪除客戶 |
| GET | `/api/v1/clients/:id` | 登入 | 獲取客戶 |
| PUT | `/api/v1/clients/:id` | 登入 | 更新客戶 |
| POST | `/api/v1/clients/:id/downgrade` | 登入 | 客戶降級 |
| GET | `/api/v1/clients/:id/orders` | 登入 | 客戶的訂單列表 |
| POST | `/api/v1/clients/:id/renew` | 登入 | 客戶續費 |
| POST | `/api/v1/clients/:id/reset-usage` | 登入 | 重置用量 |
| POST | `/api/v1/clients/:id/restore` | 登入 | 恢復客戶 |
| POST | `/api/v1/clients/:id/suspend` | 登入 | 暫停客戶 |
| POST | `/api/v1/clients/:id/upgrade` | 登入 | 客戶升級 |

### 套餐模板

套餐（變體、計費週期、價格、流量額度）與模板級節點授權。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| GET | `/api/v1/plan-templates` | 登入 | 模板列表 |
| POST | `/api/v1/plan-templates` | 登入 | 建立模板 |
| DELETE | `/api/v1/plan-templates/:id` | 登入 | 刪除模板 |
| GET | `/api/v1/plan-templates/:id` | 登入 | 模板詳情 |
| PUT | `/api/v1/plan-templates/:id` | 登入 | 更新模板 |
| GET | `/api/v1/plan-templates/:id/accessible-nodes` | 登入 | 獲取可用節點 |
| GET | `/api/v1/plan-templates/:id/billing-node-options` | 登入 | 獲取計費節點選項 |
| PUT | `/api/v1/plan-templates/:id/node-groups` | 登入 | 設定模板節點分組 |
| POST | `/api/v1/plan-templates/:id/sync-accounts` | 登入 | 同步賬號給模板 |
| GET | `/api/v1/plan-templates/:id/variants` | 登入 | 變體列表 |
| POST | `/api/v1/plan-templates/:id/variants` | 登入 | 建立變體 |
| POST | `/api/v1/plan-templates/sync-all-accounts` | 登入 | 同步全部賬號 |

### 套餐變體

套餐變體的更新與刪除（建立掛在套餐模板下）。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| DELETE | `/api/v1/plan-variants/:id` | 登入 | 刪除變體 |
| PUT | `/api/v1/plan-variants/:id` | 登入 | 更新變體 |

### 流量包

可單獨購買的流量包。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| GET | `/api/v1/traffic-packs` | 登入 | 流量包列表 |
| POST | `/api/v1/traffic-packs` | 登入 | 建立流量包 |
| DELETE | `/api/v1/traffic-packs/:id` | 登入 | 刪除流量包 |
| PUT | `/api/v1/traffic-packs/:id` | 登入 | 更新流量包 |

### 優惠碼

促銷/優惠碼的建立、校驗與核銷配置。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| GET | `/api/v1/promotions` | 登入 | 列表優惠碼 |
| POST | `/api/v1/promotions` | 登入 | 建立優惠碼 |
| DELETE | `/api/v1/promotions/:id` | 登入 | 刪除優惠碼 |
| PUT | `/api/v1/promotions/:id` | 登入 | 更新優惠碼 |
| POST | `/api/v1/promotions/validate` | 登入 | 校驗優惠碼 |

### 節點分組

把節點組織成分組，供套餐引用。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| GET | `/api/v1/node-groups` | 登入 | 列表節點分組 |
| POST | `/api/v1/node-groups` | 登入 | 建立節點分組 |
| DELETE | `/api/v1/node-groups/:id` | 登入 | 刪除節點分組 |
| PUT | `/api/v1/node-groups/:id` | 登入 | 更新節點分組 |
| GET | `/api/v1/node-groups/:id/members` | 登入 | 列表分組成員 |
| POST | `/api/v1/node-groups/:id/members` | 登入 | 新增分組成員 |
| DELETE | `/api/v1/node-groups/:id/members/:mid` | 登入 | 移除分組成員 |

### 節點伺服器

核心資源：節點增刪改查、核心（xray/sing-box）配置下發、入站/出站/端點配置、程序控制、證書、WAF、限速、埠跳躍、鏈式代理、映象/主備、Agent 安裝升級、xray 執行時控制等。子路徑極多，按功能族列出。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| GET | `/api/v1/node-servers` | 管理員 | 列表 |
| POST | `/api/v1/node-servers` | 管理員 | 建立 |
| DELETE | `/api/v1/node-servers/:id` | 管理員 | 刪除 |
| GET | `/api/v1/node-servers/:id` | 管理員 | 詳情 |
| PUT | `/api/v1/node-servers/:id` | 管理員 | 更新 |
| GET | `/api/v1/node-servers/:id/binaries` | 管理員 | 已裝核心列表 |
| POST | `/api/v1/node-servers/:id/binaries/ech-generate` | 管理員 | 生成 ECH 金鑰對 |
| POST | `/api/v1/node-servers/:id/binaries/install` | 管理員 | 安裝/升級核心 |
| GET | `/api/v1/node-servers/:id/binaries/interfaces` | 管理員 | 網絡卡列表 |
| POST | `/api/v1/node-servers/:id/binaries/mldsa65-key` | 管理員 | 生成 MLDSA65 金鑰 |
| POST | `/api/v1/node-servers/:id/binaries/reality-key` | 管理員 | 生成 REALITY 金鑰對 |
| POST | `/api/v1/node-servers/:id/binaries/reality-public-key` | 管理員 | 由 REALITY 私鑰推導公鑰 |
| GET | `/api/v1/node-servers/:id/binaries/releases` | 管理員 | 可用版本列表 |
| POST | `/api/v1/node-servers/:id/binaries/vlessenc` | 管理員 | 生成 VLESS 加密引數 |
| POST | `/api/v1/node-servers/:id/binaries/wg-key` | 管理員 | 生成 WireGuard 金鑰 |
| POST | `/api/v1/node-servers/:id/binaries/x25519-key` | 管理員 | 生成 X25519 金鑰對 |
| GET | `/api/v1/node-servers/:id/bootstrap-cmd` | 管理員 | 獲取 Agent 引導命令 |
| POST | `/api/v1/node-servers/:id/bootstrap-token` | 管理員 | 生成引導令牌 |
| POST | `/api/v1/node-servers/:id/caddy/batch-update-log-enabled` | 管理員 | 批次更新日誌開關 |
| POST | `/api/v1/node-servers/:id/caddy/batch-update-ports` | 管理員 | 批次更新埠 |
| POST | `/api/v1/node-servers/:id/caddy/generate-config` | 管理員 | 生成配置 |
| DELETE | `/api/v1/node-servers/:id/caddy/l4-rules` | 管理員 | 刪除全部 L4 規則 |
| POST | `/api/v1/node-servers/:id/caddy/save-and-apply` | 管理員 | 儲存並應用 |
| GET | `/api/v1/node-servers/:id/caddy/sites` | 管理員 | 站點列表 |
| POST | `/api/v1/node-servers/:id/caddy/sites` | 管理員 | 建立站點 |
| DELETE | `/api/v1/node-servers/:id/caddy/sites/:siteId` | 管理員 | 刪除站點 |
| PUT | `/api/v1/node-servers/:id/caddy/sites/:siteId` | 管理員 | 更新站點 |
| DELETE | `/api/v1/node-servers/:id/caddy/sni-rules` | 管理員 | 刪除全部 SNI 規則 |
| POST | `/api/v1/node-servers/:id/caddy/sni-rules/toggle` | 管理員 | 開關 SNI 規則 |
| GET | `/api/v1/node-servers/:id/caddy/sni-settings` | 管理員 | SNI 設定 |
| POST | `/api/v1/node-servers/:id/caddy/sni-settings` | 管理員 | 儲存 SNI 設定 |
| GET | `/api/v1/node-servers/:id/certificates` | 管理員 | 節點證書列表 |
| DELETE | `/api/v1/node-servers/:id/certificates/:cid` | 管理員 | 解綁證書 |
| POST | `/api/v1/node-servers/:id/certificates/:cid` | 管理員 | 繫結證書 |
| GET | `/api/v1/node-servers/:id/chain-config` | 管理員 | 生成鏈式配置 |
| GET | `/api/v1/node-servers/:id/chain-health` | 管理員 | 鏈式健康狀態 |
| POST | `/api/v1/node-servers/:id/control` | 超級管理員 | 控制節點核心。body：`{"core_type":"xray","action":"start"}`（action = start/stop/restart）。`core_type` 可選 `xray`/`singbox` |
| GET | `/api/v1/node-servers/:id/degradation-status` | 管理員 | 降級狀態 |
| POST | `/api/v1/node-servers/:id/distribute-cert` | 管理員 | 分發證書 |
| GET | `/api/v1/node-servers/:id/endpoint-configs` | 管理員 | 列表 |
| POST | `/api/v1/node-servers/:id/endpoint-configs` | 管理員 | 建立 |
| DELETE | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | 管理員 | 刪除 |
| GET | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | 管理員 | 詳情 |
| PUT | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | 管理員 | 更新 |
| POST | `/api/v1/node-servers/:id/failover` | 管理員 | 觸發該節點故障切換 |
| GET | `/api/v1/node-servers/:id/failover-status` | 管理員 | 故障切換狀態 |
| POST | `/api/v1/node-servers/:id/force-reconnect` | 管理員 | 強制重連 |
| GET | `/api/v1/node-servers/:id/free-port` | 管理員 | 申請空閒埠 |
| POST | `/api/v1/node-servers/:id/geo-rules/push` | 管理員 | 下發到該節點 |
| GET | `/api/v1/node-servers/:id/inbound-configs` | 管理員 | 列表 |
| POST | `/api/v1/node-servers/:id/inbound-configs` | 管理員 | 建立 |
| DELETE | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | 管理員 | 刪除 |
| GET | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | 管理員 | 詳情 |
| PUT | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | 管理員 | 更新 |
| GET | `/api/v1/node-servers/:id/inbound-configs/:cfgId/share-links` | 管理員 | 分享連結與片段 |
| POST | `/api/v1/node-servers/:id/inbound-configs/:cfgId/users/:email/pause` | 管理員 | 暫停該使用者 |
| POST | `/api/v1/node-servers/:id/inbound-configs/:cfgId/users/:email/resume` | 管理員 | 恢復該使用者 |
| POST | `/api/v1/node-servers/:id/inbound-configs/reset-all-traffic` | 管理員 | 重置全部入站流量 |
| GET | `/api/v1/node-servers/:id/install-script` | 管理員 | 獲取 Agent 安裝指令碼 |
| GET | `/api/v1/node-servers/:id/metrics-history` | 管理員 | 獲取指標歷史 |
| GET | `/api/v1/node-servers/:id/mirror-status` | 管理員 | 映象狀態 |
| POST | `/api/v1/node-servers/:id/nginx/batch-update-ports` | 管理員 | 批次更新埠 |
| POST | `/api/v1/node-servers/:id/nginx/generate-config` | 管理員 | 生成配置 |
| DELETE | `/api/v1/node-servers/:id/nginx/l4-rules` | 管理員 | 刪除全部 L4 規則 |
| POST | `/api/v1/node-servers/:id/nginx/save-and-apply` | 管理員 | 儲存並應用 |
| GET | `/api/v1/node-servers/:id/nginx/settings` | 管理員 | 讀取設定 |
| POST | `/api/v1/node-servers/:id/nginx/settings` | 管理員 | 儲存設定 |
| GET | `/api/v1/node-servers/:id/nginx/sites` | 管理員 | 站點列表 |
| POST | `/api/v1/node-servers/:id/nginx/sites` | 管理員 | 建立站點 |
| DELETE | `/api/v1/node-servers/:id/nginx/sites/:siteId` | 管理員 | 刪除站點 |
| PUT | `/api/v1/node-servers/:id/nginx/sites/:siteId` | 管理員 | 更新站點 |
| DELETE | `/api/v1/node-servers/:id/nginx/sni-rules` | 管理員 | 刪除全部 SNI 規則 |
| POST | `/api/v1/node-servers/:id/nginx/sni-rules/toggle` | 管理員 | 開關 SNI 規則 |
| PUT | `/api/v1/node-servers/:id/node-type` | 管理員 | 更新節點型別 |
| GET | `/api/v1/node-servers/:id/online-users` | 管理員 | 線上使用者 |
| GET | `/api/v1/node-servers/:id/outbound-configs` | 管理員 | 列表 |
| POST | `/api/v1/node-servers/:id/outbound-configs` | 管理員 | 建立 |
| DELETE | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | 管理員 | 刪除 |
| GET | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | 管理員 | 詳情 |
| PUT | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | 管理員 | 更新 |
| PUT | `/api/v1/node-servers/:id/outbound-configs/reorder` | 管理員 | 排序 |
| GET | `/api/v1/node-servers/:id/port-hop/rules` | 管理員 | 埠跳躍規則預覽 |
| GET | `/api/v1/node-servers/:id/protocols` | 管理員 | 協議列表 |
| GET | `/api/v1/node-servers/:id/protocols/:coreType` | 管理員 | 協議配置 |
| PUT | `/api/v1/node-servers/:id/protocols/:coreType` | 管理員 | 儲存協議配置 |
| GET | `/api/v1/node-servers/:id/protocols/:coreType/config-file` | 管理員 | 讀取核心配置檔案 |
| GET | `/api/v1/node-servers/:id/protocols/:coreType/section/:section` | 管理員 | 讀取協議段落 |
| PUT | `/api/v1/node-servers/:id/protocols/:coreType/section/:section` | 管理員 | 儲存協議段落 |
| POST | `/api/v1/node-servers/:id/protocols/:coreType/validate` | 管理員 | 校驗配置（真實核心 dry-run） |
| GET | `/api/v1/node-servers/:id/proxy-status` | 管理員 | 遠端代理狀態 |
| POST | `/api/v1/node-servers/:id/proxy/batch-update-ports` | 管理員 | 批次更新埠 |
| GET | `/api/v1/node-servers/:id/proxy/config` | 管理員 | 讀取配置 |
| PUT | `/api/v1/node-servers/:id/proxy/config` | 管理員 | 更新配置 |
| POST | `/api/v1/node-servers/:id/proxy/control` | 管理員 | 鏈式代理註冊控制 |
| POST | `/api/v1/node-servers/:id/proxy/download` | 管理員 | 下載代理核心版本 |
| POST | `/api/v1/node-servers/:id/proxy/generate-config` | 管理員 | 生成配置 |
| POST | `/api/v1/node-servers/:id/proxy/guardian` | 管理員 | 更新守護配置 |
| POST | `/api/v1/node-servers/:id/proxy/install` | 管理員 | 安裝代理 |
| DELETE | `/api/v1/node-servers/:id/proxy/l4-rules` | 管理員 | 刪除全部 L4 規則 |
| GET | `/api/v1/node-servers/:id/proxy/l4-settings` | 管理員 | L4 設定 |
| POST | `/api/v1/node-servers/:id/proxy/l4-settings` | 管理員 | 儲存 L4 設定 |
| GET | `/api/v1/node-servers/:id/proxy/log-settings` | 管理員 | 日誌設定 |
| POST | `/api/v1/node-servers/:id/proxy/log-settings` | 管理員 | 儲存日誌設定 |
| GET | `/api/v1/node-servers/:id/proxy/nginx-settings` | 管理員 | nginx 設定 |
| POST | `/api/v1/node-servers/:id/proxy/nginx-settings` | 管理員 | 儲存 nginx 設定 |
| GET | `/api/v1/node-servers/:id/proxy/releases` | 管理員 | 可用版本列表 |
| POST | `/api/v1/node-servers/:id/proxy/reload` | 管理員 | 過載配置 |
| POST | `/api/v1/node-servers/:id/proxy/restart` | 管理員 | 重啟 |
| GET | `/api/v1/node-servers/:id/proxy/sites` | 管理員 | 站點列表 |
| POST | `/api/v1/node-servers/:id/proxy/sites` | 管理員 | 建立站點 |
| DELETE | `/api/v1/node-servers/:id/proxy/sites/:siteId` | 管理員 | 刪除站點 |
| PUT | `/api/v1/node-servers/:id/proxy/sites/:siteId` | 管理員 | 更新站點 |
| DELETE | `/api/v1/node-servers/:id/proxy/sni-rules` | 管理員 | 刪除全部 SNI 規則 |
| POST | `/api/v1/node-servers/:id/proxy/sni-rules/toggle` | 管理員 | 開關 SNI 規則 |
| GET | `/api/v1/node-servers/:id/proxy/sni-settings` | 管理員 | SNI 設定 |
| POST | `/api/v1/node-servers/:id/proxy/sni-settings` | 管理員 | 儲存 SNI 設定 |
| POST | `/api/v1/node-servers/:id/proxy/start` | 管理員 | 啟動 |
| GET | `/api/v1/node-servers/:id/proxy/status` | 管理員 | 代理程序狀態 |
| POST | `/api/v1/node-servers/:id/proxy/stop` | 管理員 | 停止 |
| POST | `/api/v1/node-servers/:id/proxy/sync` | 管理員 | 同步代理配置 |
| GET | `/api/v1/node-servers/:id/rate-limit` | 管理員 | 讀取配置 |
| PUT | `/api/v1/node-servers/:id/rate-limit` | 管理員 | 更新配置 |
| GET | `/api/v1/node-servers/:id/rate-limit/status` | 管理員 | 狀態 |
| GET | `/api/v1/node-servers/:id/recover-mode` | 管理員 | 讀取節點恢復模式 |
| PUT | `/api/v1/node-servers/:id/recover-mode` | 管理員 | 設定節點恢復模式 |
| POST | `/api/v1/node-servers/:id/revoke-cert` | 管理員 | 吊銷 Agent 證書 |
| GET | `/api/v1/node-servers/:id/services` | 管理員 | 列表 |
| POST | `/api/v1/node-servers/:id/services` | 管理員 | 建立 |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId` | 管理員 | 刪除 |
| GET | `/api/v1/node-servers/:id/services/:serviceId` | 管理員 | 詳情 |
| PUT | `/api/v1/node-servers/:id/services/:serviceId` | 管理員 | 更新 |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId/crash-logs` | 管理員 | 清空崩潰日誌 |
| GET | `/api/v1/node-servers/:id/services/:serviceId/crash-logs` | 管理員 | 崩潰日誌列表 |
| GET | `/api/v1/node-servers/:id/services/:serviceId/crash-logs/:logId` | 管理員 | 崩潰日誌詳情 |
| GET | `/api/v1/node-servers/:id/services/:serviceId/endpoints` | 管理員 | 端點列表 |
| POST | `/api/v1/node-servers/:id/services/:serviceId/endpoints` | 管理員 | 建立端點 |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId/endpoints/:endpointId` | 管理員 | 刪除端點 |
| PUT | `/api/v1/node-servers/:id/services/:serviceId/endpoints/:endpointId` | 管理員 | 更新端點 |
| PUT | `/api/v1/node-servers/:id/services/:serviceId/guardian` | 管理員 | 更新守護配置 |
| GET | `/api/v1/node-servers/:id/services/:serviceId/process-status` | 管理員 | 獲取處理狀態 |
| POST | `/api/v1/node-servers/:id/services/:serviceId/restart` | 管理員 | 重啟 |
| POST | `/api/v1/node-servers/:id/services/:serviceId/start` | 管理員 | 啟動 |
| POST | `/api/v1/node-servers/:id/services/:serviceId/stop` | 管理員 | 停止 |
| GET | `/api/v1/node-servers/:id/services/process-status` | 管理員 | 全部程序狀態 |
| GET | `/api/v1/node-servers/:id/share-domains` | 管理員 | 該節點的分享域名 |
| POST | `/api/v1/node-servers/:id/share-domains` | 管理員 | 建立 |
| DELETE | `/api/v1/node-servers/:id/share-domains/:domainId` | 管理員 | 刪除 |
| PUT | `/api/v1/node-servers/:id/share-domains/:domainId` | 管理員 | 更新 |
| DELETE | `/api/v1/node-servers/:id/share-domains/all` | 管理員 | 清空全部 |
| POST | `/api/v1/node-servers/:id/singbox/control/firewall/apply` | 管理員 | 應用埠跳躍防火牆規則 |
| POST | `/api/v1/node-servers/:id/singbox/control/firewall/remove` | 管理員 | 移除埠跳躍防火牆規則 |
| POST | `/api/v1/node-servers/:id/speed-test` | 管理員 | 節點測速 |
| GET | `/api/v1/node-servers/:id/status` | 管理員 | 節點狀態 |
| POST | `/api/v1/node-servers/:id/sync` | 管理員 | 同步節點配置（生成並下發） |
| POST | `/api/v1/node-servers/:id/sync-data` | 超級管理員 | 向節點推送面板資料（使用者/訂閱/套餐等全量同步） |
| POST | `/api/v1/node-servers/:id/sync-mirror` | 管理員 | 同步映象節點 |
| GET | `/api/v1/node-servers/:id/tags` | 管理員 | 標籤列表 |
| POST | `/api/v1/node-servers/:id/tags` | 管理員 | 新增標籤 |
| DELETE | `/api/v1/node-servers/:id/tags/:tagKey` | 管理員 | 刪除標籤 |
| GET | `/api/v1/node-servers/:id/traffic-history` | 管理員 | 流量歷史 |
| POST | `/api/v1/node-servers/:id/traffic-resume` | 管理員 | 恢復因超額暫停的流量 |
| GET | `/api/v1/node-servers/:id/traffic-usage` | 管理員 | 流量用量 |
| POST | `/api/v1/node-servers/:id/update-agent` | 管理員 | 升級 Agent |
| GET | `/api/v1/node-servers/:id/used-server-names` | 管理員 | 已佔用的 REALITY SNI 列表 |
| GET | `/api/v1/node-servers/:id/waf/config` | 管理員 | 讀取配置 |
| PUT | `/api/v1/node-servers/:id/waf/config` | 管理員 | 儲存配置 |
| POST | `/api/v1/node-servers/:id/waf/disable-domain` | 管理員 | 停用域名防護 |
| GET | `/api/v1/node-servers/:id/waf/domain-config` | 管理員 | 域名配置列表 |
| PUT | `/api/v1/node-servers/:id/waf/domain-config` | 管理員 | 批次儲存域名配置 |
| DELETE | `/api/v1/node-servers/:id/waf/domain-config/:domain` | 管理員 | 刪除域名配置 |
| GET | `/api/v1/node-servers/:id/waf/domain-config/:domain` | 管理員 | 域名配置詳情 |
| PUT | `/api/v1/node-servers/:id/waf/domain-config/:domain` | 管理員 | 儲存域名配置 |
| GET | `/api/v1/node-servers/:id/waf/domain-rules` | 管理員 | 域名規則目錄 |
| GET | `/api/v1/node-servers/:id/waf/domain-rules/:domain/:name` | 管理員 | 讀取域名規則檔案 |
| PUT | `/api/v1/node-servers/:id/waf/domain-rules/:domain/:name` | 管理員 | 儲存域名規則檔案 |
| GET | `/api/v1/node-servers/:id/waf/domain-status` | 管理員 | 域名防護狀態 |
| POST | `/api/v1/node-servers/:id/waf/enable-domain` | 管理員 | 啟用域名防護 |
| POST | `/api/v1/node-servers/:id/waf/push` | 管理員 | 下發到該節點 |
| GET | `/api/v1/node-servers/:id/waf/rules` | 管理員 | 節點 WAF 規則列表 |
| DELETE | `/api/v1/node-servers/:id/waf/rules/:name` | 管理員 | 刪除節點 WAF 規則 |
| GET | `/api/v1/node-servers/:id/waf/rules/:name` | 管理員 | 節點 WAF 規則詳情 |
| PUT | `/api/v1/node-servers/:id/waf/rules/:name` | 管理員 | 儲存節點 WAF 規則 |
| GET | `/api/v1/node-servers/:id/waf/status` | 管理員 | 節點 WAF 狀態 |
| PUT | `/api/v1/node-servers/:id/waf/status` | 管理員 | 開關節點 WAF |
| POST | `/api/v1/node-servers/:id/waf/sync` | 管理員 | 同步到該節點 |
| GET | `/api/v1/node-servers/:id/warp` | 管理員 | 列表 |
| DELETE | `/api/v1/node-servers/:id/warp/:warpId` | 管理員 | 刪除 |
| GET | `/api/v1/node-servers/:id/warp/config` | 管理員 | 讀取配置 |
| POST | `/api/v1/node-servers/:id/warp/generate-config` | 管理員 | 生成配置 |
| POST | `/api/v1/node-servers/:id/warp/register` | 管理員 | 註冊 |
| GET | `/api/v1/node-servers/:id/warp/status` | 管理員 | 狀態 |
| GET | `/api/v1/node-servers/:id/xray/control/balancer/:tag` | 管理員 | 獲取均衡器資訊 |
| POST | `/api/v1/node-servers/:id/xray/control/balancer/:tag` | 管理員 | 設定均衡器目標 |
| POST | `/api/v1/node-servers/:id/xray/control/firewall/apply` | 管理員 | 應用埠跳躍防火牆規則 |
| POST | `/api/v1/node-servers/:id/xray/control/firewall/remove` | 管理員 | 移除埠跳躍防火牆規則 |
| POST | `/api/v1/node-servers/:id/xray/control/inbound` | 管理員 | 新增入站 |
| DELETE | `/api/v1/node-servers/:id/xray/control/inbound/:tag` | 管理員 | 移除入站 |
| POST | `/api/v1/node-servers/:id/xray/control/inbound/:tag/user` | 管理員 | 新增使用者 |
| DELETE | `/api/v1/node-servers/:id/xray/control/inbound/:tag/user/:email` | 管理員 | 移除使用者 |
| POST | `/api/v1/node-servers/:id/xray/control/outbound` | 管理員 | 新增出站 |
| DELETE | `/api/v1/node-servers/:id/xray/control/outbound/:tag` | 管理員 | 移除出站 |
| POST | `/api/v1/node-servers/:id/xray/control/routing` | 管理員 | 申請路由 |
| POST | `/api/v1/node-servers/:id/xray/control/routing/rule` | 管理員 | 新增路由規則 |
| DELETE | `/api/v1/node-servers/:id/xray/control/routing/rule/:tag` | 管理員 | 移除路由規則 |
| GET | `/api/v1/node-servers/:id/xray/control/routing/rules` | 管理員 | 列表路由規則 |
| POST | `/api/v1/node-servers/:id/xray/control/sync` | 管理員 | 手動觸發 xray 配置同步下發 |
| GET | `/api/v1/node-servers/:id/xray/stats` | 管理員 | 獲取統計 |
| POST | `/api/v1/node-servers/:id/xray/stats/reset/:email` | 管理員 | 重置使用者統計 |
| GET | `/api/v1/node-servers/:id/xray/stats/users` | 管理員 | 獲取使用者統計 |
| GET | `/api/v1/node-servers/billable` | 管理員 | 可計費節點列表 |
| GET | `/api/v1/node-servers/ca-cert` | 管理員 | CA 證書 |
| GET | `/api/v1/node-servers/ca-status` | 管理員 | CA 狀態 |
| GET | `/api/v1/node-servers/dashboard` | 管理員 | 儀表盤狀態 |
| POST | `/api/v1/node-servers/distribute-cert` | 管理員 | 向全部節點分發證書 |
| POST | `/api/v1/node-servers/failover-check` | 管理員 | 觸發故障切換檢查 |
| GET | `/api/v1/node-servers/metrics/aggregated` | 管理員 | 聚合指標 |
| POST | `/api/v1/node-servers/migrate-cert` | 管理員 | 遷移證書 |
| GET | `/api/v1/node-servers/online-summary` | 管理員 | 線上彙總 |
| GET | `/api/v1/node-servers/online-users` | 管理員 | 全部線上使用者 |
| GET | `/api/v1/node-servers/pending-agent-updates` | 管理員 | 待升級的 Agent |
| POST | `/api/v1/node-servers/recommend-region` | 管理員 | 推薦地區 |
| POST | `/api/v1/node-servers/select-nodes` | 管理員 | 為訂閱挑選節點 |
| POST | `/api/v1/node-servers/speed-test-all` | 管理員 | 全部節點測速 |
| GET | `/api/v1/node-servers/speed-test-results` | 管理員 | 測速結果 |
| GET | `/api/v1/node-servers/traffic/stats` | 管理員 | 流量統計 |
| POST | `/api/v1/node-servers/update-all-agents` | 管理員 | 升級全部 Agent |
| DELETE | `/api/v1/node-servers/users/:client_id` | 管理員 | 同步刪除某客戶 |
| GET | `/api/v1/node-servers/users/retry-queue` | 管理員 | 重試佇列狀態 |
| POST | `/api/v1/node-servers/users/retry-queue/process` | 管理員 | 處理重試佇列 |
| POST | `/api/v1/node-servers/users/sync` | 管理員 | 同步單個客戶 |
| POST | `/api/v1/node-servers/validate-chain-config` | 管理員 | 校驗鏈式配置 |

### 節點審計日誌

節點側操作審計與匯出。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| GET | `/api/v1/node-audit-logs` | 登入 | 節點審計日誌 |
| GET | `/api/v1/node-audit-logs/export` | 登入 | 匯出節點審計日誌 |

### 節點（舊版模型，已下線）

> `/api/v1/nodes` CRUD 已於 2026-10-20 下線：節點統一由 `/api/v1/node-servers` 管理。
> 線上使用者查詢保留在 `/api/v1/node-servers/{online-users,online-summary,:id/online-users}`。

### 地區總覽

按地區聚合的節點檢視。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| GET | `/api/v1/regions/:region/servers` | 登入 | 該地區節點列表 |
| GET | `/api/v1/regions/overview` | 登入 | 地區總覽 |

### 訂閱

訂閱記錄管理：吊銷、輪換 token、訪問日誌、用量重置。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| GET | `/api/v1/subscriptions` | 登入 | 列表 |
| POST | `/api/v1/subscriptions` | 登入 | 建立 |
| DELETE | `/api/v1/subscriptions/:id` | 登入 | 刪除 |
| GET | `/api/v1/subscriptions/:id` | 登入 | 詳情 |
| PUT | `/api/v1/subscriptions/:id` | 登入 | 更新 |
| GET | `/api/v1/subscriptions/:id/access-logs` | 登入 | 訪問日誌 |
| POST | `/api/v1/subscriptions/:id/enable` | 登入 | 啟用 |
| POST | `/api/v1/subscriptions/:id/reset-usage` | 登入 | 重置用量 |
| POST | `/api/v1/subscriptions/:id/restore` | 登入 | 恢復 |
| POST | `/api/v1/subscriptions/:id/revoke` | 登入 | 吊銷 |
| POST | `/api/v1/subscriptions/:id/rotate` | 登入 | 輪換 |
| GET | `/api/v1/subscriptions/:id/rotations` | 登入 | 輪換歷史 |

### 訂閱格式

> 管理端 `/api/v1/subscription-formats` CRUD 已於 2026-10-20 下線：實際維護入口是
> `/api/v1/mihomo/configs`（訂閱格式中心）。`subscription_formats` 表仍保留，
> 僅用於公共介面 `/api/v1/public/client-configs` 的種子資料。

### 訂閱接入點

獨立訂閱域名（反代回面板的 /sub 路徑）。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| GET | `/api/v1/subscription-endpoints` | 登入 | 列表 |
| POST | `/api/v1/subscription-endpoints` | 登入 | 建立 |
| DELETE | `/api/v1/subscription-endpoints/:id` | 登入 | 刪除 |
| GET | `/api/v1/subscription-endpoints/:id` | 登入 | 詳情 |
| PUT | `/api/v1/subscription-endpoints/:id` | 登入 | 更新 |

### 分享域名

客戶端連結使用的接入域名/IP 池。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| GET | `/api/v1/share-domains` | 登入 | 列表 |
| POST | `/api/v1/share-domains` | 登入 | 建立 |
| DELETE | `/api/v1/share-domains/:id` | 登入 | 刪除 |
| PUT | `/api/v1/share-domains/:id` | 登入 | 更新 |
| DELETE | `/api/v1/share-domains/all` | 登入 | 清空全部 |
| POST | `/api/v1/share-domains/seed` | 登入 | 初始化預設資料 |

### Cloudflare IP

CF CDN IP 池與來源管理。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| DELETE | `/api/v1/cfip/entries` | 登入 | 清空條目 |
| GET | `/api/v1/cfip/entries` | 登入 | 條目列表 |
| POST | `/api/v1/cfip/entries` | 登入 | 新增條目 |
| DELETE | `/api/v1/cfip/entries/:id` | 登入 | 刪除條目 |
| POST | `/api/v1/cfip/fetch-all` | 登入 | 拉取全部來源 |
| POST | `/api/v1/cfip/seed` | 登入 | 初始化預設資料 |
| GET | `/api/v1/cfip/sources` | 登入 | 資料來源列表 |
| POST | `/api/v1/cfip/sources` | 登入 | 建立資料來源 |
| DELETE | `/api/v1/cfip/sources/:id` | 登入 | 刪除資料來源 |
| PUT | `/api/v1/cfip/sources/:id` | 登入 | 更新資料來源 |
| POST | `/api/v1/cfip/sources/:id/fetch` | 登入 | 拉取該來源 |

### 訂閱配置（Mihomo/Clash）

訂閱模板配置：分組、規則、rule-providers、YAML 預覽。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| GET | `/api/v1/mihomo/configs` | 登入 | 列表 |
| POST | `/api/v1/mihomo/configs` | 登入 | 建立 |
| DELETE | `/api/v1/mihomo/configs/:id` | 登入 | 刪除 |
| GET | `/api/v1/mihomo/configs/:id` | 登入 | 詳情 |
| PUT | `/api/v1/mihomo/configs/:id` | 登入 | 更新 |
| GET | `/api/v1/mihomo/configs/:id/proxy-groups` | 登入 | 分組列表 |
| POST | `/api/v1/mihomo/configs/:id/proxy-groups` | 登入 | 建立分組 |
| GET | `/api/v1/mihomo/configs/:id/rule-providers` | 登入 | 規則提供者列表 |
| POST | `/api/v1/mihomo/configs/:id/rule-providers` | 登入 | 建立規則提供者 |
| GET | `/api/v1/mihomo/configs/:id/rules` | 登入 | 規則列表 |
| POST | `/api/v1/mihomo/configs/:id/rules` | 登入 | 建立規則 |
| GET | `/api/v1/mihomo/configs/:id/yaml` | 登入 | 預覽生成的 YAML |
| DELETE | `/api/v1/mihomo/proxy-groups/:id` | 登入 | 刪除分組 |
| DELETE | `/api/v1/mihomo/rule-providers/:id` | 登入 | 刪除規則提供者 |
| DELETE | `/api/v1/mihomo/rules/:id` | 登入 | 刪除規則 |

### 充值站點

第三方支付站點接入：簽名金鑰、驗籤檔案、域名驗證、對接文件。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| GET | `/api/v1/charge-sites` | 登入 | 站點列表 |
| POST | `/api/v1/charge-sites` | 登入 | 建立站點 |
| DELETE | `/api/v1/charge-sites/:id` | 登入 | 刪除站點 |
| GET | `/api/v1/charge-sites/:id` | 登入 | 站點詳情 |
| PUT | `/api/v1/charge-sites/:id` | 登入 | 更新站點 |
| POST | `/api/v1/charge-sites/:id/clear-prev-secret` | 登入 | 清除舊金鑰 |
| POST | `/api/v1/charge-sites/:id/generate-verify` | 登入 | 生成驗籤檔案 |
| POST | `/api/v1/charge-sites/:id/generate-verify-file` | 登入 | 生成驗籤檔案 |
| GET | `/api/v1/charge-sites/:id/integration-doc` | 登入 | 獲取對接文件 |
| POST | `/api/v1/charge-sites/:id/rotate-secret` | 登入 | 輪換籤名金鑰 |
| POST | `/api/v1/charge-sites/:id/verify-dns` | 登入 | 驗證域名 DNS |
| POST | `/api/v1/charge-sites/:id/verify-domain` | 登入 | 驗證域名 HTTP 挑戰 |
| POST | `/api/v1/charge-sites/:id/verify-domain-dns` | 登入 | 驗證域名 DNS |
| POST | `/api/v1/charge-sites/:id/verify-domain-http` | 登入 | 驗證域名 HTTP 挑戰 |

### 充值訂單

充值訂單查詢、退款、CSV 匯出。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| GET | `/api/v1/charge-orders` | 登入 | 訂單列表 |
| GET | `/api/v1/charge-orders/:id` | 登入 | 訂單詳情 |
| POST | `/api/v1/charge-orders/:id/refund` | 登入 | 退款 |
| GET | `/api/v1/charge-orders/export-csv` | 登入 | 匯出充值訂單 CSV |

### 充值卡密

卡密批次生成、使用記錄、CSV 匯出。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| GET | `/api/v1/charge-cards` | 登入 | 卡密列表 |
| POST | `/api/v1/charge-cards` | 登入 | 建立卡密 |
| DELETE | `/api/v1/charge-cards/:id` | 登入 | 刪除卡密 |
| GET | `/api/v1/charge-cards/:id/usages` | 登入 | 卡密使用記錄 |
| POST | `/api/v1/charge-cards/batch` | 登入 | 批次生成卡密 |
| GET | `/api/v1/charge-cards/export-csv` | 登入 | 匯出卡密 CSV |

### 支付回撥事件

回撥事件審計。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| GET | `/api/v1/charge-callback-events` | 登入 | 回撥事件列表 |

### 支付回撥重試

失敗回撥的手動重試。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| GET | `/api/v1/charge-callback-retries` | 登入 | 失敗回撥列表 |
| POST | `/api/v1/charge-callback-retries/:id/retry` | 管理員 | 重試回撥 |

### 客戶餘額

餘額與流水的管理端檢視。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| GET | `/api/v1/client-balances` | 登入 | 客戶餘額列表 |
| GET | `/api/v1/client-balances/transactions` | 登入 | 餘額流水（管理端） |
| GET | `/api/v1/client-balances/transactions/export-csv` | 登入 | 匯出餘額流水 CSV |

### 禮品

可領取禮品（天數/流量）的發放。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| GET | `/api/v1/client-gifts` | 登入 | 禮品列表 |
| POST | `/api/v1/client-gifts` | 登入 | 建立禮品 |
| POST | `/api/v1/client-gifts/batch` | 登入 | 批次建立禮品 |
| GET | `/api/v1/client-gifts/search-clients` | 登入 | 搜尋客戶 |

### 推廣配置

推廣返傭配置與統計。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| GET | `/api/v1/referral/config` | 登入 | 推廣配置 |
| PUT | `/api/v1/referral/config` | 管理員 | 儲存推廣配置 |
| GET | `/api/v1/referral/records` | 登入 | 推廣記錄列表 |
| GET | `/api/v1/referral/stats` | 登入 | 推廣統計 |

### 推廣提現

提現申請的稽核。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| GET | `/api/v1/referral-withdrawals` | 登入 | 提現申請列表 |
| POST | `/api/v1/referral-withdrawals/:id/approve` | 登入 | 批准提現 |
| POST | `/api/v1/referral-withdrawals/:id/reject` | 登入 | 拒絕提現 |

### IP 白名單（彙總）

全部客戶的 IP 白名單檢視。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| GET | `/api/v1/ip-whitelists` | 登入 | 全部客戶 IP 白名單 |

### 客戶 OAuth 繫結

客戶第三方賬號繫結查詢。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| GET | `/api/v1/client-oauth-bindings` | 登入 | 客戶 OAuth 繫結列表 |

### 工單（管理端）

工單回覆、指派、關閉。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| GET | `/api/v1/tickets` | 登入 | 工單列表 |
| GET | `/api/v1/tickets/:id` | 登入 | 工單詳情 |
| POST | `/api/v1/tickets/:id/assign` | 登入 | 指派工單 |
| POST | `/api/v1/tickets/:id/close` | 登入 | 關閉工單 |
| POST | `/api/v1/tickets/:id/reply` | 登入 | 回覆工單 |

### 文章

門戶文章的增刪改查。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| GET | `/api/v1/articles` | 登入 | 文章列表 |
| POST | `/api/v1/articles` | 登入 | 建立文章 |
| DELETE | `/api/v1/articles/:id` | 登入 | 刪除文章 |
| GET | `/api/v1/articles/:id` | 登入 | 文章詳情 |
| PUT | `/api/v1/articles/:id` | 登入 | 更新文章 |

### 共享賬號

流媒體等共享賬號的發放與修復記錄。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| GET | `/api/v1/shared-accounts` | 登入 | 共享賬號列表 |
| POST | `/api/v1/shared-accounts` | 登入 | 建立共享賬號 |
| DELETE | `/api/v1/shared-accounts/:id` | 登入 | 刪除共享賬號 |
| GET | `/api/v1/shared-accounts/:id` | 登入 | 共享賬號詳情 |
| PUT | `/api/v1/shared-accounts/:id` | 登入 | 更新共享賬號 |
| GET | `/api/v1/shared-accounts/:id/repair-logs` | 登入 | 修復記錄 |

### 證書

公信證書（ACME）簽發、上傳、續期、匯出；支援指定節點簽發。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| GET | `/api/v1/certificates` | 登入 | 列表 |
| POST | `/api/v1/certificates` | 登入 | 建立 |
| DELETE | `/api/v1/certificates/:id` | 登入 | 刪除 |
| GET | `/api/v1/certificates/:id` | 登入 | 詳情 |
| PUT | `/api/v1/certificates/:id` | 登入 | 更新 |
| GET | `/api/v1/certificates/:id/download` | 登入 | 下載 |
| POST | `/api/v1/certificates/:id/renew` | 登入 | 客戶續費 |
| GET | `/api/v1/certificates/check-port` | 登入 | 檢查埠 |
| GET | `/api/v1/certificates/detect-ip` | 登入 | 探測IP |
| GET | `/api/v1/certificates/export` | 登入 | 匯出 |
| POST | `/api/v1/certificates/issue` | 登入 | 簽發。body：`{"domains":["example.com","*.example.com"],"challenge_type":"dns-01","dns_channel_id":2,"key_type":"rsa","key_bits":4096}`。`challenge_type` 預設 http-01；`dns_channel_id` 指定 DNS 通道（面板配置好的 DNS 提供者+憑據） |
| POST | `/api/v1/certificates/issue-on-node` | 登入 | 簽發到節點 |
| POST | `/api/v1/certificates/upload` | 登入 | 上傳 |

### 證書通道

ACME 通道（CA）配置。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| GET | `/api/v1/cert-channels` | 登入 | 列表 |
| POST | `/api/v1/cert-channels` | 登入 | 建立 |
| DELETE | `/api/v1/cert-channels/:id` | 登入 | 刪除 |
| PUT | `/api/v1/cert-channels/:id` | 登入 | 更新 |
| GET | `/api/v1/cert-channels/providers` | 登入 | 提供商 |

### EAB 配置

ZeroSSL / Google 等外部賬號繫結憑據。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| GET | `/api/v1/eab-configs` | 登入 | 列表 |
| POST | `/api/v1/eab-configs` | 登入 | 建立 |
| DELETE | `/api/v1/eab-configs/:id` | 登入 | 刪除 |
| PUT | `/api/v1/eab-configs/:id` | 登入 | 更新 |
| POST | `/api/v1/eab-configs/fetch-google` | 登入 | 從 GCP 獲取 Google EAB |
| POST | `/api/v1/eab-configs/fetch-zerossl` | 登入 | 用郵箱一鍵獲取 ZeroSSL EAB |

### ACME 賬號

面板側 ACME 賬號（聯絡郵箱）管理。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| GET | `/api/v1/acme-accounts` | 登入 | 列表 |
| POST | `/api/v1/acme-accounts` | 登入 | 建立 |
| DELETE | `/api/v1/acme-accounts/:id` | 登入 | 刪除 |
| PUT | `/api/v1/acme-accounts/:id` | 登入 | 更新 |
| GET | `/api/v1/acme-accounts/default` | 登入 | 獲取預設 |

### 內建 CA

面板內部 CA：Agent 證書籤發/續期/吊銷、CRL 匯出、分發佇列。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| GET | `/api/v1/ca/agent-certs` | 登入 | Agent 證書列表 |
| POST | `/api/v1/ca/agent-certs/:server_id/renew` | 登入 | 續期 Agent 證書 |
| GET | `/api/v1/ca/distribution-queue` | 登入 | 證書分發佇列 |
| DELETE | `/api/v1/ca/distribution-queue/:id` | 登入 | 刪除分發佇列條目 |
| POST | `/api/v1/ca/distribution-queue/:id/retry` | 登入 | 重試證書分發 |
| GET | `/api/v1/ca/revocations` | 登入 | 吊銷列表 |
| GET | `/api/v1/ca/revocations/export` | 登入 | 匯出 CRL |
| GET | `/api/v1/ca/status` | 登入 | CA 狀態 |

### Geo 規則

geoip/geosite 資料來源與檔案的同步、下發。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| GET | `/api/v1/geo-rules/files` | 登入 | 檔案列表 |
| DELETE | `/api/v1/geo-rules/files/:fileId` | 登入 | 刪除檔案 |
| PUT | `/api/v1/geo-rules/files/:fileId/auto-sync` | 登入 | 開關自動同步 |
| POST | `/api/v1/geo-rules/files/:fileId/download` | 登入 | 下載 |
| GET | `/api/v1/geo-rules/files/:fileId/download-file` | 登入 | 下載檔案 |
| GET | `/api/v1/geo-rules/github-assets` | 登入 | 拉取 GitHub 倉庫資產 |
| POST | `/api/v1/geo-rules/push-all` | 登入 | 下發到全部節點 |
| POST | `/api/v1/geo-rules/seed` | 登入 | 初始化預設資料 |
| GET | `/api/v1/geo-rules/sources` | 登入 | 資料來源列表 |
| POST | `/api/v1/geo-rules/sources` | 登入 | 建立資料來源 |
| DELETE | `/api/v1/geo-rules/sources/:sourceId` | 登入 | 刪除資料來源 |
| PUT | `/api/v1/geo-rules/sources/:sourceId` | 登入 | 更新資料來源 |
| GET | `/api/v1/geo-rules/sources/:sourceId/assets` | 登入 | 拉取 Release 資產 |
| POST | `/api/v1/geo-rules/sources/:sourceId/files` | 登入 | 新增檔案 |
| PUT | `/api/v1/geo-rules/sources/:sourceId/schedule` | 登入 | 開關定時同步 |
| POST | `/api/v1/geo-rules/sources/:sourceId/select` | 登入 | 選擇檔案 |
| POST | `/api/v1/geo-rules/sources/:sourceId/sync` | 登入 | 同步該資料來源 |
| GET | `/api/v1/geo-rules/status` | 登入 | 狀態 |
| POST | `/api/v1/geo-rules/sync-all` | 登入 | 同步全部 |
| GET | `/api/v1/geo-rules/sync-status/:key` | 登入 | 同步進度 |

### WAF 規則庫

全域性 WAF 規則：GitHub 源、同步、下發到全部節點。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| GET | `/api/v1/waf/github-source` | 登入 | GitHub 資料來源 |
| PUT | `/api/v1/waf/github-source` | 登入 | 儲存 GitHub 資料來源 |
| POST | `/api/v1/waf/push-all` | 登入 | 下發到全部節點 |
| GET | `/api/v1/waf/rules` | 登入 | 規則列表 |
| DELETE | `/api/v1/waf/rules/:name` | 登入 | 刪除規則 |
| GET | `/api/v1/waf/rules/:name` | 登入 | 獲取規則 |
| PUT | `/api/v1/waf/rules/:name` | 登入 | 儲存規則 |
| POST | `/api/v1/waf/sync-all` | 登入 | 同步全部 |
| POST | `/api/v1/waf/sync-github` | 登入 | 從 GitHub 同步 |

### 日誌管理

節點/核心日誌讀取與流式跟隨、清理策略、日誌寫入器控制。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| POST | `/api/v1/log-manage/cleanup-all` | 登入 | 立即全量清理 |
| GET | `/api/v1/log-manage/cleanup-rules` | 登入 | 清理規則列表 |
| PUT | `/api/v1/log-manage/cleanup-rules/:id` | 登入 | 更新清理規則 |
| POST | `/api/v1/log-manage/cleanup-table/:table` | 登入 | 立即清理指定表 |
| GET | `/api/v1/log-manage/core-log-output` | 登入 | 核心日誌輸出配置 |
| PUT | `/api/v1/log-manage/core-log-output` | 登入 | 儲存核心日誌輸出配置 |
| GET | `/api/v1/log-manage/core/:type/stdout-log` | 登入 | 讀取核心 stdout 日誌 |
| GET | `/api/v1/log-manage/core/:type/stdout-stream` | 登入 | 流式跟隨核心 stdout |
| POST | `/api/v1/log-manage/discover` | 登入 | 發現節點日誌檔案 |
| GET | `/api/v1/log-manage/log-output` | 登入 | 日誌輸出配置 |
| PUT | `/api/v1/log-manage/log-output` | 登入 | 儲存日誌輸出配置 |
| POST | `/api/v1/log-manage/log-output/release/:type` | 登入 | 釋放日誌 stdout 接管 |
| POST | `/api/v1/log-manage/log-output/takeover/:type` | 登入 | 接管日誌 stdout |
| GET | `/api/v1/log-manage/nodes/:serverId/logs` | 登入 | 節點日誌檔案列表 |
| GET | `/api/v1/log-manage/nodes/:serverId/logs/:type` | 登入 | 讀取節點日誌 |
| POST | `/api/v1/log-manage/nodes/:serverId/logs/:type/rotate` | 登入 | 輪轉節點日誌 |
| GET | `/api/v1/log-manage/nodes/:serverId/logs/:type/stream` | 登入 | 流式跟隨節點日誌 |
| GET | `/api/v1/log-manage/policies` | 登入 | 策略列表 |
| POST | `/api/v1/log-manage/policies` | 登入 | 建立策略 |
| DELETE | `/api/v1/log-manage/policies/:id` | 登入 | 刪除策略 |
| GET | `/api/v1/log-manage/policies/:id` | 登入 | 策略詳情 |
| PUT | `/api/v1/log-manage/policies/:id` | 登入 | 更新策略 |
| GET | `/api/v1/log-manage/policies/:id/jobs` | 登入 | 清理任務列表 |
| GET | `/api/v1/log-manage/policies/:id/log` | 登入 | 讀取日誌 |
| GET | `/api/v1/log-manage/policies/:id/log-stream` | 登入 | 流式跟隨日誌 |
| POST | `/api/v1/log-manage/policies/:id/rotate` | 登入 | 輪換 |
| GET | `/api/v1/log-manage/proxy/:type/stdout-log` | 登入 | 讀取核心 stdout 日誌 |
| GET | `/api/v1/log-manage/proxy/:type/stdout-stream` | 登入 | 流式跟隨核心 stdout |
| GET | `/api/v1/log-manage/scheduler-config` | 登入 | 排程配置 |
| PUT | `/api/v1/log-manage/scheduler-config` | 登入 | 更新排程配置 |
| GET | `/api/v1/log-manage/status` | 登入 | 狀態 |
| POST | `/api/v1/log-manage/sync` | 登入 | 同步清理策略 |
| GET | `/api/v1/log-manage/writers` | 登入 | 寫入器狀態 |
| PUT | `/api/v1/log-manage/writers/:type/config` | 登入 | 更新寫入器配置 |
| GET | `/api/v1/log-manage/writers/:type/files` | 登入 | 寫入器日誌檔案列表 |
| POST | `/api/v1/log-manage/writers/:type/start` | 登入 | 啟動寫入器 |
| POST | `/api/v1/log-manage/writers/:type/stop` | 登入 | 停止寫入器 |

### 備份

資料庫/全量備份、恢復、定時任務、FTP/S3/WebDAV 遠端儲存。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| DELETE | `/api/v1/backups/:id` | 管理員 | 刪除備份 |
| GET | `/api/v1/backups/:id/download` | 管理員 | 下載備份 |
| POST | `/api/v1/backups/:id/restore` | 管理員 | 從備份恢復 |
| POST | `/api/v1/backups/:id/upload-ftp` | 管理員 | 上傳到 FTP |
| POST | `/api/v1/backups/:id/upload-s3` | 管理員 | 上傳到 S3 |
| POST | `/api/v1/backups/:id/upload-webdav` | 管理員 | 上傳到 WebDAV |
| POST | `/api/v1/backups/create` | 管理員 | 建立全量備份（含 DB dump） |
| GET | `/api/v1/backups/list` | 管理員 | 全量備份列表 |
| POST | `/api/v1/backups/remote-restore` | 管理員 | 從遠端恢復 |
| GET | `/api/v1/backups/restore-status` | 管理員 | 恢復進度 |
| GET | `/api/v1/backups/schedules` | 管理員 | 定時任務列表 |
| POST | `/api/v1/backups/schedules` | 管理員 | 建立定時任務 |
| DELETE | `/api/v1/backups/schedules/:id` | 管理員 | 刪除定時任務 |
| PUT | `/api/v1/backups/schedules/:id` | 管理員 | 更新定時任務 |
| GET | `/api/v1/backups/storages` | 管理員 | 儲存配置列表 |
| POST | `/api/v1/backups/storages` | 管理員 | 建立儲存配置 |
| DELETE | `/api/v1/backups/storages/:id` | 管理員 | 刪除儲存配置 |
| PUT | `/api/v1/backups/storages/:id` | 管理員 | 更新儲存配置 |
| GET | `/api/v1/backups/storages/:id/files` | 管理員 | 遠端備份檔案列表 |
| POST | `/api/v1/backups/upload-restore` | 管理員 | 上傳備份並恢復 |

### 系統設定

資料庫配置與遷移、面板設定、JWT 金鑰、SMTP/Telegram 測試、通知模板、線上升級等（多數需管理員）。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| PUT | `/api/v1/system/bootstrap-finish` | 管理員 | 完成首次引導 |
| GET | `/api/v1/system/bootstrap-state` | 管理員 | 首次引導狀態 |
| GET | `/api/v1/system/config-file/:coreType` | 管理員 | 讀取核心配置檔案 |
| GET | `/api/v1/system/db-config` | 管理員 | 資料庫配置 |
| PUT | `/api/v1/system/db-config` | 管理員 | 更新資料庫配置 |
| POST | `/api/v1/system/db-config/migrate` | 管理員 | 遷移資料庫資料 |
| POST | `/api/v1/system/db-config/switch` | 管理員 | 切換資料庫（含一步遷移） |
| POST | `/api/v1/system/db-config/test` | 管理員 | 測試資料庫連線 |
| GET | `/api/v1/system/db-pool-stats` | 管理員 | 連線池統計 |
| GET | `/api/v1/system/db-stats` | 管理員 | 資料庫統計 |
| GET | `/api/v1/system/db-stats/ping` | 管理員 | 資料庫探測 |
| POST | `/api/v1/system/db-stats/reset` | 管理員 | 重置資料庫統計 |
| GET | `/api/v1/system/encryption-key-status` | 管理員 | 加密金鑰狀態 |
| GET | `/api/v1/system/env-config` | 管理員 | .env 配置 |
| PUT | `/api/v1/system/env-config` | 管理員 | 更新 .env 配置 |
| POST | `/api/v1/system/generate-encryption-key` | 管理員 | 生成加密金鑰 |
| GET | `/api/v1/system/github-config` | 管理員 | GitHub 配置 |
| PUT | `/api/v1/system/github-config` | 管理員 | 更新 GitHub 配置 |
| GET | `/api/v1/system/jwt-secret-status` | 管理員 | JWT 金鑰狀態 |
| GET | `/api/v1/system/maintenance` | 公開 | 維護模式狀態（公開） |
| PUT | `/api/v1/system/maintenance` | 管理員 | 切換維護模式 |
| POST | `/api/v1/system/migrate-data` | 管理員 | 遷移資料 |
| GET | `/api/v1/system/notification-config` | 管理員 | 通知配置 |
| PUT | `/api/v1/system/notification-config` | 管理員 | 更新通知配置 |
| GET | `/api/v1/system/notification-templates` | 管理員 | 模板列表 |
| POST | `/api/v1/system/notification-templates` | 管理員 | 建立模板 |
| DELETE | `/api/v1/system/notification-templates/:id` | 管理員 | 刪除模板 |
| PUT | `/api/v1/system/notification-templates/:id` | 管理員 | 更新模板 |
| POST | `/api/v1/system/notification-templates/:id/broadcast` | 管理員 | 按模板群發通知 |
| POST | `/api/v1/system/notification-templates/:id/send` | 管理員 | 傳送測試通知 |
| GET | `/api/v1/system/panel-config` | 管理員 | 面板配置 |
| PUT | `/api/v1/system/panel-config` | 管理員 | 更新面板配置 |
| GET | `/api/v1/system/reality-targets` | 管理員 | REALITY 目標列表 |
| PUT | `/api/v1/system/reality-targets` | 管理員 | 更新 REALITY 目標 |
| POST | `/api/v1/system/reality-targets/reset` | 管理員 | 重置 REALITY 目標為預設 |
| POST | `/api/v1/system/regenerate-jwt-secret` | 管理員 | 重新生成 JWT 金鑰 |
| GET | `/api/v1/system/releases` | 管理員 | 可用版本列表 |
| POST | `/api/v1/system/restart` | 管理員 | 重啟 |
| POST | `/api/v1/system/rollback` | 管理員 | 系統回滾 |
| GET | `/api/v1/system/settings` | 管理員 | 讀取設定 |
| PUT | `/api/v1/system/settings/batch` | 管理員 | 批次更新設定 |
| GET | `/api/v1/system/stats` | 管理員 | 系統狀態 |
| GET | `/api/v1/system/subscription-billing-templates` | 管理員 | 訂閱賬單模板 |
| PUT | `/api/v1/system/subscription-billing-templates` | 管理員 | 更新訂閱賬單模板 |
| POST | `/api/v1/system/telegram-chat-id` | 管理員 | 獲取 Telegram Chat ID |
| POST | `/api/v1/system/test-smtp` | 管理員 | 測試 SMTP |
| POST | `/api/v1/system/test-telegram` | 管理員 | 測試 Telegram |
| POST | `/api/v1/system/update` | 管理員 | 系統升級 |
| GET | `/api/v1/system/update-jobs` | 管理員 | 升級任務列表 |
| POST | `/api/v1/system/upload-mtls-ca` | 管理員 | 上傳 mTLS CA 證書 |

### 安全審計

審計事件查詢與匯出。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| GET | `/api/v1/security/audit-events` | 登入 | 審計事件列表 |
| GET | `/api/v1/security/audit-events/export` | 登入 | 匯出審計事件 |

### 指標

面板聚合指標（節點/證書/服務）。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| GET | `/api/v1/metrics/certificates` | 登入 | 證書指標 |
| GET | `/api/v1/metrics/nodes` | 登入 | 節點指標 |
| GET | `/api/v1/metrics/overview` | 登入 | 總覽 |

### 門戶模板

門戶（Portal）模板與啟用配置、快取管理。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| GET | `/api/v1/portal/active` | 登入 | 當前啟用模板 |
| PUT | `/api/v1/portal/active` | 登入 | 更新啟用模板 |
| DELETE | `/api/v1/portal/cache` | 登入 | 清空門戶快取 |
| GET | `/api/v1/portal/cache` | 登入 | 快取狀態 |
| PUT | `/api/v1/portal/cache/dev-mode` | 登入 | 切換快取開發模式 |
| POST | `/api/v1/portal/cache/rebuild` | 登入 | 重建門戶快取 |
| POST | `/api/v1/portal/reset` | 登入 | 恢復預設模板 |
| GET | `/api/v1/portal/templates` | 登入 | 列表 |
| POST | `/api/v1/portal/templates` | 登入 | 建立 |
| DELETE | `/api/v1/portal/templates/:id` | 登入 | 刪除 |
| GET | `/api/v1/portal/templates/:id` | 登入 | 詳情 |
| PUT | `/api/v1/portal/templates/:id` | 登入 | 更新 |
| POST | `/api/v1/portal/templates/:id/activate` | 登入 | 啟用模板 |

> `/portal/templates` CRUD 是**高階 API**：門戶 UI 的主路徑是 `PUT /api/v1/portal/active`
> （整站配置一次提交），多模板管理/切換才需要走這組介面。兩者都受支援，不衝突。

### Agent 安裝

節點 Agent 安裝指令碼與下載（公開）。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| POST | `/api/v1/agent/bootstrap` | 公開 | Agent 首次引導註冊（Agent 憑 bootstrap token 上報，公開） |
| GET | `/api/v1/agent/releases` | 管理員 | Agent 可用版本列表（供升級頁選擇） |
| GET | `/api/v1/agent/version` | 管理員 | 面板側 Agent 當前版本 |
| GET | `/api/v1/agent/download` | 公開 | 下載Agent |
| GET | `/api/v1/agent/install` | 公開 | 獲取安裝指令碼 |

### 支付回撥入口

第三方支付平臺回撥（公開、驗籤）。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| POST | `/api/v1/charge/callback/:site_id` | 公開 | Handle充值回撥 |

### 入站（含節點）

套餐選節點用的入站列表。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| GET | `/api/v1/inbounds-with-server` | 登入 | 全部入站（含所屬節點） |

### 公開配置

無需登入的門戶執行時配置（帶維護模式守衛 + 公網限流）。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| GET | `/api/v1/public/articles` | 公開 | 公開文章列表 |
| GET | `/api/v1/public/articles/:slug` | 公開 | 公開文章詳情 |
| GET | `/api/v1/public/client-configs` | 公開 | 客戶端下載配置（公開） |
| GET | `/api/v1/public/portal-config` | 公開 | 門戶執行時配置（站點名、主題、多語言、合規文件；公開） |
| GET | `/api/v1/public/settings` | 公開 | 公開設定（如 registration_enabled；公開） |
| GET | `/api/v1/public/subscription-endpoints` | 公開 | 啟用的訂閱接入點列表（公開） |

### 客戶認證與業務（管理端引擎上的同一套）

與門戶一致的客戶介面，完整說明見 portal-api.md。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| GET | `/api/v1/client-auth/articles` | 公開 | 列表文章 |
| GET | `/api/v1/client-auth/articles/:slug` | 公開 | 獲取文章 |
| GET | `/api/v1/client-auth/balance` | 公開 | 獲取餘額 |
| GET | `/api/v1/client-auth/balance/transactions` | 公開 | 餘額流水 |
| GET | `/api/v1/client-auth/bills` | 公開 | 訂單列表 |
| POST | `/api/v1/client-auth/change-password` | 公開 | 修改密碼 |
| POST | `/api/v1/client-auth/charge/create` | 公開 | 建立充值訂單 |
| GET | `/api/v1/client-auth/charge/orders` | 公開 | 列表充值訂單 |
| GET | `/api/v1/client-auth/charge/orders/:id` | 公開 | 獲取充值訂單詳情 |
| POST | `/api/v1/client-auth/charge/redeem-card` | 公開 | 卡密兌換 |
| GET | `/api/v1/client-auth/charge/sites` | 公開 | 列表充值站點 |
| GET | `/api/v1/client-auth/charge/status/:trade_no` | 公開 | 查詢充值狀態 |
| GET | `/api/v1/client-auth/checktime` | 公開 | 獲取伺服器時間（校時） |
| GET | `/api/v1/client-auth/connection-info` | 公開 | 獲取套餐連線引數 |
| GET | `/api/v1/client-auth/gifts` | 公開 | 列表禮品 |
| POST | `/api/v1/client-auth/gifts/:id/claim` | 公開 | 領取禮品 |
| GET | `/api/v1/client-auth/invoices` | 公開 | 訂單列表 |
| GET | `/api/v1/client-auth/invoices/:id` | 公開 | 訂單詳情 |
| GET | `/api/v1/client-auth/ip-whitelist` | 公開 | 列表IP白名單 |
| DELETE | `/api/v1/client-auth/ip-whitelist/:id` | 公開 | 移除IP白名單 |
| POST | `/api/v1/client-auth/ip-whitelist/auto-add` | 公開 | 自動新增IP |
| POST | `/api/v1/client-auth/login` | 公開 | 登入 |
| POST | `/api/v1/client-auth/logout` | 公開 | 登出 |
| POST | `/api/v1/client-auth/logs/package` | 公開 | 建立日誌日誌包 |
| GET | `/api/v1/client-auth/logs/packages` | 公開 | 列表日誌日誌包 |
| DELETE | `/api/v1/client-auth/logs/packages/:id` | 公開 | 刪除日誌日誌包 |
| GET | `/api/v1/client-auth/logs/packages/:id` | 公開 | 獲取日誌日誌包 |
| GET | `/api/v1/client-auth/logs/packages/:id/download` | 公開 | 下載日誌日誌包 |
| GET | `/api/v1/client-auth/logs/types` | 公開 | 日誌日誌包健康檢查 |
| GET | `/api/v1/client-auth/me` | 公開 | 獲取當前登入使用者 |
| GET | `/api/v1/client-auth/nodes` | 公開 | 我的節點列表 |
| GET | `/api/v1/client-auth/nodes/:id/config` | 公開 | 節點配置 |
| GET | `/api/v1/client-auth/nodes/:id/link` | 公開 | 節點分享連結 |
| GET | `/api/v1/client-auth/nodes/:id/qrcode` | 公開 | 節點二維碼 |
| GET | `/api/v1/client-auth/oauth/:provider` | 公開 | 授權 |
| GET | `/api/v1/client-auth/oauth/:provider/callback` | 公開 | 回撥 |
| GET | `/api/v1/client-auth/oauth/providers` | 公開 | 提供商狀態 |
| GET | `/api/v1/client-auth/orders` | 公開 | 訂單列表 |
| POST | `/api/v1/client-auth/orders` | 公開 | 建立訂單 |
| GET | `/api/v1/client-auth/orders/:id` | 公開 | 訂單詳情 |
| GET | `/api/v1/client-auth/plans` | 公開 | 套餐列表 |
| GET | `/api/v1/client-auth/plans/:id` | 公開 | 套餐詳情 |
| GET | `/api/v1/client-auth/profile` | 公開 | 獲取資料 |
| PUT | `/api/v1/client-auth/profile` | 公開 | 更新資料 |
| POST | `/api/v1/client-auth/profile/bind-oauth` | 公開 | 繫結第三方賬號 |
| DELETE | `/api/v1/client-auth/profile/unbind-oauth/:provider` | 公開 | 解綁第三方賬號 |
| GET | `/api/v1/client-auth/referral/code` | 公開 | 獲取推廣驗證碼 |
| GET | `/api/v1/client-auth/referral/records` | 公開 | 列表推廣記錄 |
| GET | `/api/v1/client-auth/referral/stats` | 公開 | 獲取推廣統計 |
| POST | `/api/v1/client-auth/referral/withdraw` | 公開 | Request提現 |
| GET | `/api/v1/client-auth/referral/withdrawals` | 公開 | 列表推廣提現 |
| POST | `/api/v1/client-auth/refresh` | 公開 | 重新整理 |
| POST | `/api/v1/client-auth/register` | 公開 | 註冊 |
| POST | `/api/v1/client-auth/reset-subscription` | 公開 | 重置訂閱 |
| POST | `/api/v1/client-auth/rotate-token` | 公開 | 輪換訂閱 token |
| POST | `/api/v1/client-auth/self-unban` | 公開 | 自助解封 |
| PUT | `/api/v1/client-auth/settings` | 公開 | 更新設定 |
| POST | `/api/v1/client-auth/settings/auto-add-ip` | 公開 | 自動新增IP |
| GET | `/api/v1/client-auth/settings/whitelist` | 公開 | 列表IP白名單 |
| DELETE | `/api/v1/client-auth/settings/whitelist/:id` | 公開 | 移除IP白名單 |
| GET | `/api/v1/client-auth/shared-accounts` | 公開 | 列表共享賬號 |
| GET | `/api/v1/client-auth/shared-accounts/:id` | 公開 | 獲取共享賬號 |
| POST | `/api/v1/client-auth/shared-accounts/:id/apply` | 公開 | 申請共享賬號 |
| POST | `/api/v1/client-auth/shared-accounts/:id/repair` | 公開 | Request修復 |
| POST | `/api/v1/client-auth/ticket` | 公開 | 簽發一次性短時票據 |
| GET | `/api/v1/client-auth/tickets` | 公開 | 列表工單 |
| POST | `/api/v1/client-auth/tickets` | 公開 | 建立工單 |
| GET | `/api/v1/client-auth/tickets/:id` | 公開 | 獲取工單 |
| POST | `/api/v1/client-auth/tickets/:id/close` | 公開 | 關閉工單 |
| POST | `/api/v1/client-auth/tickets/:id/reply` | 公開 | 回覆工單 |
| GET | `/api/v1/client-auth/traffic-history` | 公開 | 流量歷史 |
| GET | `/api/v1/client-auth/traffic-packs` | 公開 | 流量包列表 |
| POST | `/api/v1/client-auth/traffic-packs/buy` | 公開 | 購買流量包 |

### 訂閱下發

客戶端拉取訂閱的公開入口，詳見 subscription-api.md。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| GET | `/sub/:token` | 公開 | Serve訂閱 |

### 執行時配置工作流

配置快照 / 幹跑 / 應用 / 回滾（apply、rollback、disable-proxy 需管理員）。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| POST | `/runtime/apply` | 公開 | 申請 |
| GET | `/runtime/apply-jobs` | 登入 | 列表任務 |
| GET | `/runtime/apply-jobs/:id` | 登入 | 獲取任務 |
| POST | `/runtime/apply-jobs/:id/cancel` | 公開 | 取消任務 |
| POST | `/runtime/apply-jobs/:id/retry` | 公開 | 重試任務 |
| POST | `/runtime/disable-proxy` | 公開 | 禁用代理 |
| POST | `/runtime/dry-run` | 登入 | 試算執行 |
| GET | `/runtime/nodes` | 登入 | 列表節點 |
| GET | `/runtime/overview` | 登入 | 總覽 |
| POST | `/runtime/rollback` | 公開 | 回滾 |
| GET | `/runtime/snapshots` | 登入 | 列表快照 |
| POST | `/runtime/snapshots` | 登入 | 建立快照 |
| DELETE | `/runtime/snapshots/:id` | 公開 | 刪除快照 |
| GET | `/runtime/snapshots/:id` | 登入 | 獲取快照 |
| POST | `/runtime/verify` | 登入 | 驗證 |

### 服務訪問策略（已下線）

> `/services/:id/access-policies` 與 `/services/:id/ip-bans` 已於 2026-10-20 下線：
> 管理端零呼叫，且沒有任何連線鏈路消費這些策略。

### 流量配額監控

配額與用量的只讀監控檢視。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| GET | `/traffic/quotas` | 登入 | 配額列表 |
| GET | `/traffic/quotas/:id/usage` | 登入 | 配額用量 |
| GET | `/traffic/reset-logs` | 登入 | 配額重置記錄 |
| GET | `/traffic/usage-history` | 登入 | 用量歷史 |

### 健康檢查

存活探針。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| GET | `/health` | 公開 | 存活探針，返回 200 即健康 |

### 門戶頁

門戶 SPA 頁面。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| GET | `/p` | 公開 | 門戶頁 HTML（服務端渲染 + Vue 執行時） |

### ACME 挑戰

HTTP-01 挑戰回撥。

| 方法 | 路徑 | 許可權 | 說明 |
|---|---|---|---|
| GET | `/.well-known/acme-challenge/:token` | 公開 | ACME HTTP-01 挑戰回撥（證書籤發時驗證域名） |
