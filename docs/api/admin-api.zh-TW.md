<!-- source: admin-api.md | locale: zh-TW | updated: 2026-11-16 -->
# 管理員 API

> 語言： [简体中文](admin-api.md) · [English](admin-api.en.md) · 繁體中文 · [Русский](admin-api.ru.md) · [日本語](admin-api.ja.md) · [한국어](admin-api.ko.md) · [Deutsch](admin-api.de.md) · [Français](admin-api.fr.md) · [Español](admin-api.es.md) · [Português](admin-api.pt.md) · [Türkçe](admin-api.tr.md) · [Bahasa Indonesia](admin-api.id.md) · [ไทย](admin-api.th.md) · [Tiếng Việt](admin-api.vi.md) · [العربية](admin-api.ar.md) · [فارسی](admin-api.fa.md)

適用於**面板開發/自動化/整合商**的完整管理 API 路由表。

- 底座 URL：`https://<panel-domain>/api/v1`
- 授权：`Authorization: Bearer <admin_token>`（来自`POST /api/v1/auth/login`）
- 常見回應/錯誤格式與速率限制約定：請參閱 [README.md](./README.zh-TW.md)
- 真相來源：`internal/api/router/router.go`（版本不同計數不同，代碼權威）；如果此表不同意，**代碼獲勝**

## 權限標記

| 標記 | 意義 |
|---|---|
| 公有 | 無需登入（維護警衛/速率限制可能仍然適用） |
| 公開·限速 | 无需登录，有专门的速率限制 |
| 已登入 | 需要登录的管理员令牌 (`AuthRequired`) |
| 操作員 | 需要角色 ≥ `operator` (`RequireRole(RoleOperator)`) |
| 管理員 | 需要角色 ≥ `admin` (`RequireRole(RoleAdmin)`) |
| 超管理員 | 需要角色 = `super_admin` |
| 客戶（公眾） | 不需要客户端令牌（登录/注册等） |
| 用戶端（已登入） | 需要 `Authorization: Bearer <client_token>`（入口網站客戶端令牌，而不是管理令牌） |

角色等級：`viewer(10) < operator(20) < admin(30) < super_admin(40)`；角色不足返回 403 `INSUFFICIENT_ROLE`。

---

## 快速入門（實例）

這些例子均來自當地真實的反應；令牌和數據被編輯。

### 1. 登入並取得令牌

```bash
curl -X POST https://<panel-domain>/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"your-password"}'
```

回應：

```json
{
  "data": {
    "access_token": "<JWT, valid 24h>",
    "refresh_token": "<JWT, valid 7 days>",
    "user": { "id": 1, "role": "super_admin", "username": "admin" }
  }
}
```

### 2. 使用令牌調用業務端點

```bash
curl https://<panel-domain>/api/v1/clients?limit=1 \
  -H 'Authorization: Bearer <access_token>'
```

回應（清單使用 `items` + `total` 結構）：

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

### 3. 錯誤回應範例

**401 未經身份驗證：**

```json
{
  "code": "MISSING_TOKEN",
  "message": "missing authorization header",
  "request_id": "a02cfdf9-6be0-46be-8584-824caab34dc7",
  "can_retry": false,
  "requires_action": false
}
```

**404 資源未找到：**

```json
{
  "code": "CLIENT_NOT_FOUND",
  "message": "client not found",
  "request_id": "099949bd-4ded-468d-a6f8-f8e798af2273",
  "can_retry": false,
  "requires_action": false
}
```

### 4. 分頁

所有清單端點都使用`limit`（頁面大小，預設為20）和`offset`（從0開始）並傳回`{items, total, limit, offset}`。有些舊端點傳回一個裸數組（按表註明）。

---

## 內容

- [管理員身份驗證和安全](#管理員身份驗證和安全)
- [管理員帳戶](#管理員帳戶)
- [客戶管理](#客戶管理)
- [計劃模板](#計劃模板)
- [計劃變體](#計劃變體)
- [流量包](#流量包)
- [優惠券](#優惠券)
- [節點群組](#節點群組)
- [節點伺服器](#節點伺服器)
- [節點審計日誌](#節點審計日誌)
- [節點（舊模型，已退役）](#節點舊模型已退役)
- [地區概況](#地區概況)
- [訂閱](#訂閱)
- [訂閱格式](#訂閱格式)
- [訂閱端點](#訂閱端點)
- [共享域名](#共享域名)
- [Cloudflare IP](#cloudflare-ip)
- [訂閱配置（Mihomo/Clash）](#訂閱配置mihomoclash)
- [儲值地點](#儲值地點)
- [儲值訂單](#儲值訂單)
- [儲值卡](#儲值卡)
- [收費回呼事件](#收費回呼事件)
- [收費回調重試](#收費回調重試)
- [客戶餘額](#客戶餘額)
- [禮物](#禮物)
- [推薦配置](#推薦配置)
- [推薦提款](#推薦提款)
- [IP 允許清單（摘要）](#ip-允許清單摘要)
- [客戶 OAuth 綁定](#客戶-oauth-綁定)
- [門票（管理員）](#門票管理員)
- [文章](#文章)
- [共享帳戶](#共享帳戶)
- [證書](#證書)
- [證書頻道](#證書頻道)
- [EAB配置](#eab配置)
- [ACME帳戶](#acme帳戶)
- [內建CA](#內建ca)
- [地理規則](#地理規則)
- [WAF規則庫](#waf規則庫)
- [日誌管理](#日誌管理)
- [備份](#備份)
- [系統設定](#系統設定)
- [安全審計](#安全審計)
- [指標](#指標)
- [門戶模板](#門戶模板)
- [代理安裝](#代理安裝)
- [收費回調入口](#收費回調入口)
- [入站（含節點）](#入站含節點)
- [公共配置](#公共配置)
- [客戶端身份驗證和業務（管理引擎上的設定相同）](#客戶端身份驗證和業務管理引擎上的設定相同)
- [訂閱交付](#訂閱交付)
- [運行時配置工作流程](#運行時配置工作流程)
- [服务访问策略（已停用）](#服务访问策略已停用)
- [流量配額監控](#流量配額監控)
- [健康檢查](#健康檢查)
- [入口網頁](#入口網頁)
- [ACME挑戰](#acme挑戰)

---

### 管理員身份驗證和安全

登入、登出、密碼變更、2FA (TOTP)、Passkey (WebAuthn) 以及管理面板的一次性票證。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| POST | `/api/v1/auth/2fa/disable` | 已登入 | 停用 |
| POST | `/api/v1/auth/2fa/enable` | 已登入 | 啟用 |
| POST | `/api/v1/auth/2fa/recovery-codes` | 已登入 | 重新產生恢復程式碼 |
| POST | `/api/v1/auth/2fa/setup` | 已登入 | 設定 |
| GET | `/api/v1/auth/2fa/status` | 已登入 | 狀態 |
| POST | `/api/v1/auth/change-password` | 已登入 | 更改密碼 |
| POST | `/api/v1/auth/login` | 公有 | 登入 |
| POST | `/api/v1/auth/login/2fa` | 公有 | 動態程式碼登入 (2FA) |
| GET | `/api/v1/auth/login/2fa-status` | 公有 | 檢查帳戶是否啟用了2FA |
| POST | `/api/v1/auth/logout` | 已登入 | 退出 |
| GET | `/api/v1/auth/me` | 已登入 | 目前登入用戶 |
| GET | `/api/v1/auth/passkey/credentials` | 已登入 | Passkey 憑證列表 |
| DELETE | `/api/v1/auth/passkey/credentials/:id` | 已登入 | 刪除Passkey憑證 |
| POST | `/api/v1/auth/passkey/login/begin` | 公有 | 開始登入 |
| POST | `/api/v1/auth/passkey/login/finish` | 公有 | 完成登入 |
| POST | `/api/v1/auth/passkey/register/begin` | 已登入 | 開始註冊 |
| POST | `/api/v1/auth/passkey/register/finish` | 已登入 | 完成註冊 |
| GET | `/api/v1/auth/passkey/rpids` | 管理員 | RP ID 允許列表 |
| POST | `/api/v1/auth/passkey/rpids` | 管理員 | 更新 RP ID 允許列表 |
| GET | `/api/v1/auth/passkey/status` | 公有 | 狀態 |
| POST | `/api/v1/auth/refresh` | 公有 | 重新整理 |
| POST | `/api/v1/auth/ticket` | 已登入 | 發行一次性短期票 |

### 管理員帳戶

管理面板使用者帳戶。清單需要管理員；建立/更新/刪除需要超級管理員（防止權限升級）。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| GET | `/api/v1/users` | 管理員 | 清單 |
| POST | `/api/v1/users` | 超管理員 | 創建 |
| DELETE | `/api/v1/users/:id` | 超管理員 | 刪除 |
| PUT | `/api/v1/users/:id` | 超管理員 | 更新 |

### 客戶管理

訂戶（入口網站帳戶）的 CRUD、暫停/恢復、升級/降級、續約和使用重設。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| GET | `/api/v1/clients` | 已登入 | 列出客戶 |
| POST | `/api/v1/clients` | 操作員 | 創建客戶。機身：`{"email":"user@example.com","password":"plaintext-password","plan_id":1,"quota_bytes":10737418240,"expires_at":"2026-12-31T00:00:00Z","remark":""}`。需要`plan_id`；`password`可能為空（客戶可以稍後在入口網站中設定） |
| DELETE | `/api/v1/clients/:id` | 操作員 | 刪除客戶 |
| GET | `/api/v1/clients/:id` | 已登入 | 獲取客戶 |
| PUT | `/api/v1/clients/:id` | 操作員 | 更新客戶 |
| POST | `/api/v1/clients/:id/downgrade` | 操作員 | 降級客戶 |
| GET | `/api/v1/clients/:id/orders` | 已登入 | 客戶訂單清單 |
| POST | `/api/v1/clients/:id/renew` | 操作員 | 續訂客戶 |
| POST | `/api/v1/clients/:id/reset-usage` | 操作員 | 重置使用情況 |
| POST | `/api/v1/clients/:id/restore` | 操作員 | 恢復客戶 |
| POST | `/api/v1/clients/:id/suspend` | 操作員 | 暫停客戶 |
| POST | `/api/v1/clients/:id/upgrade` | 操作員 | 升級客戶 |

### 計劃模板

計劃（變體、計費週期、價格、流量配額）和模板級節點授權。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| GET | `/api/v1/plan-templates` | 已登入 | 模板列表 |
| POST | `/api/v1/plan-templates` | 管理員 | 建立模板 |
| DELETE | `/api/v1/plan-templates/:id` | 管理員 | 刪除模板 |
| GET | `/api/v1/plan-templates/:id` | 已登入 | 模板詳細信息 |
| PUT | `/api/v1/plan-templates/:id` | 管理員 | 更新模板 |
| GET | `/api/v1/plan-templates/:id/accessible-nodes` | 已登入 | 取得可用節點 |
| GET | `/api/v1/plan-templates/:id/billing-node-options` | 已登入 | 取得計費節點選項 |
| PUT | `/api/v1/plan-templates/:id/node-groups` | 管理員 | 設定模板節點組 |
| POST | `/api/v1/plan-templates/:id/sync-accounts` | 管理員 | 將帳戶同步到模板 |
| GET | `/api/v1/plan-templates/:id/variants` | 已登入 | 變體列表 |
| POST | `/api/v1/plan-templates/:id/variants` | 管理員 | 創建變體 |
| POST | `/api/v1/plan-templates/sync-all-accounts` | 管理員 | 同步所有帳戶 |

### 計劃變體

更新和刪除計劃變體（建立嵌套在計劃範本下）。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| DELETE | `/api/v1/plan-variants/:id` | 管理員 | 刪除變體 |
| PUT | `/api/v1/plan-variants/:id` | 管理員 | 更新變體 |

### 流量包

流量包可以單獨購買。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| GET | `/api/v1/traffic-packs` | 已登入 | 流量包列表 |
| POST | `/api/v1/traffic-packs` | 管理員 | 建立流量包 |
| DELETE | `/api/v1/traffic-packs/:id` | 管理員 | 刪除流量包 |
| PUT | `/api/v1/traffic-packs/:id` | 管理員 | 更新流量包 |

### 優惠券

創建、驗證和兌換優惠券。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| GET | `/api/v1/promotions` | 已登入 | 列出優惠券 |
| POST | `/api/v1/promotions` | 管理員 | 創建優惠券 |
| DELETE | `/api/v1/promotions/:id` | 管理員 | 刪除優惠券 |
| PUT | `/api/v1/promotions/:id` | 管理員 | 更新優惠券 |
| POST | `/api/v1/promotions/validate` | 已登入 | 驗證優惠券 |

### 節點群組

將節點組織成計劃引用的群組。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| GET | `/api/v1/node-groups` | 已登入 | 列出節點組 |
| POST | `/api/v1/node-groups` | 管理員 | 建立節點組 |
| DELETE | `/api/v1/node-groups/:id` | 管理員 | 刪除節點群組 |
| PUT | `/api/v1/node-groups/:id` | 管理員 | 更新節點群組 |
| GET | `/api/v1/node-groups/:id/members` | 已登入 | 列出群組成員 |
| POST | `/api/v1/node-groups/:id/members` | 管理員 | 新增群組成員 |
| DELETE | `/api/v1/node-groups/:id/members/:mid` | 管理員 | 刪除群組成員 |

### 節點伺服器

核心資源：節點CRUD、核心（xray/sing-box）配置交付、入站/出站/端點配置、流程控制、憑證、WAF、速率限制、連接埠跳躍、鍊式代理、鏡像/主備份、代理安裝/升級、xray運行時控制。許多子路徑，按功能族分組。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| GET | `/api/v1/node-servers` | 管理員 | 清單 |
| POST | `/api/v1/node-servers` | 管理員 | 創建 |
| DELETE | `/api/v1/node-servers/:id` | 管理員 | 刪除 |
| GET | `/api/v1/node-servers/:id` | 管理員 | 詳情 |
| PUT | `/api/v1/node-servers/:id` | 管理員 | 更新 |
| GET | `/api/v1/node-servers/:id/binaries` | 管理員 | 已安裝核心列表 |
| POST | `/api/v1/node-servers/:id/binaries/ech-generate` | 管理員 | 產生ECH金鑰對 |
| POST | `/api/v1/node-servers/:id/binaries/install` | 管理員 | 安裝/升級核心 |
| GET | `/api/v1/node-servers/:id/binaries/interfaces` | 管理員 | 網路介面列表 |
| POST | `/api/v1/node-servers/:id/binaries/mldsa65-key` | 管理員 | 產生MLDSA65金鑰 |
| POST | `/api/v1/node-servers/:id/binaries/reality-key` | 管理員 | 產生REALITY金鑰對 |
| POST | `/api/v1/node-servers/:id/binaries/reality-public-key` | 管理員 | 从REALITY私钥推导出公钥 |
| GET | `/api/v1/node-servers/:id/binaries/releases` | 管理員 | 可用版本 |
| POST | `/api/v1/node-servers/:id/binaries/vlessenc` | 管理員 | 生成VLESS加密参数 |
| POST | `/api/v1/node-servers/:id/binaries/wg-key` | 管理員 | 產生WireGuard金鑰 |
| POST | `/api/v1/node-servers/:id/binaries/x25519-key` | 管理員 | 產生X25519金鑰對 |
| GET | `/api/v1/node-servers/:id/bootstrap-cmd` | 管理員 | 取得代理引導命令 |
| POST | `/api/v1/node-servers/:id/bootstrap-token` | 管理員 | 產生引導令牌 |
| POST | `/api/v1/node-servers/:id/caddy/batch-update-log-enabled` | 管理員 | 批次更新日誌開關 |
| POST | `/api/v1/node-servers/:id/caddy/batch-update-ports` | 管理員 | 批量更新連接埠 |
| POST | `/api/v1/node-servers/:id/caddy/generate-config` | 管理員 | 產生配置 |
| DELETE | `/api/v1/node-servers/:id/caddy/l4-rules` | 管理員 | 刪除所有L4規則 |
| POST | `/api/v1/node-servers/:id/caddy/save-and-apply` | 管理員 | 儲存並應用 |
| GET | `/api/v1/node-servers/:id/caddy/sites` | 管理員 | 站點列表 |
| POST | `/api/v1/node-servers/:id/caddy/sites` | 管理員 | 建立網站 |
| POST | `/api/v1/node-servers/:id/caddy/import` | 管理員 | 導入站點 |
| POST | `/api/v1/node-servers/:id/caddy/preview-site` | 管理員 | 預覽渲染的 Caddy 站點 |
| DELETE | `/api/v1/node-servers/:id/caddy/sites/:siteId` | 管理員 | 刪除站點 |
| PUT | `/api/v1/node-servers/:id/caddy/sites/:siteId` | 管理員 | 更新站點 |
| DELETE | `/api/v1/node-servers/:id/caddy/sni-rules` | 管理員 | 刪除所有SNI規則 |
| POST | `/api/v1/node-servers/:id/caddy/sni-rules/toggle` | 管理員 | 切換 SNI 規則 |
| GET | `/api/v1/node-servers/:id/caddy/sni-settings` | 管理員 | SNI設定 |
| POST | `/api/v1/node-servers/:id/caddy/sni-settings` | 管理員 | 儲存SNI設定 |
| GET | `/api/v1/node-servers/:id/certificates` | 管理員 | 節點證書列表 |
| DELETE | `/api/v1/node-servers/:id/certificates/:cid` | 管理員 | 解除綁定證書 |
| POST | `/api/v1/node-servers/:id/certificates/:cid` | 管理員 | 綁定憑證 |
| GET | `/api/v1/node-servers/:id/chain-config` | 管理員 | 產生鍊式配置 |
| GET | `/api/v1/node-servers/:id/chain-health` | 管理員 | 鏈條健康狀況 |
| POST | `/api/v1/node-servers/:id/control` | 超管理員 | 控制節點核心。主體：`{"core_type":"xray","action":"start"}`（操作=啟動/停止/重新啟動）。 `core_type`可以是`xray`/`singbox` |
| GET | `/api/v1/node-servers/:id/degradation-status` | 管理員 | 降級狀態 |
| POST | `/api/v1/node-servers/:id/distribute-cert` | 管理員 | 分發證書 |
| GET | `/api/v1/node-servers/:id/endpoint-configs` | 管理員 | 清單 |
| POST | `/api/v1/node-servers/:id/endpoint-configs` | 管理員 | 創建 |
| DELETE | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | 管理員 | 刪除 |
| GET | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | 管理員 | 詳情 |
| PUT | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | 管理員 | 更新 |
| POST | `/api/v1/node-servers/:id/failover` | 管理員 | 觸發該節點的故障轉移 |
| GET | `/api/v1/node-servers/:id/failover-status` | 管理員 | 故障轉移狀態 |
| POST | `/api/v1/node-servers/:id/force-reconnect` | 管理員 | 強制重新連接 |
| GET | `/api/v1/node-servers/:id/free-port` | 管理員 | 請求自由港 |
| POST | `/api/v1/node-servers/:id/geo-rules/push` | 管理員 | 推送到該節點 |
| GET | `/api/v1/node-servers/:id/firewall/status` | 管理員 | 防火牆偵測（firewalld/ufw/nft/iptables） |
| POST | `/api/v1/node-servers/:id/firewall/allow-port` | 管理員 | 允許入站監聽連接埠（支援多範圍） |
| POST | `/api/v1/node-servers/:id/firewall/remove-port` | 管理員 | 撤銷入境口岸配額（含重複） |
| GET | `/api/v1/node-servers/:id/inbound-configs` | 管理員 | 清單 |
| POST | `/api/v1/node-servers/:id/inbound-configs` | 管理員 | 創建 |
| DELETE | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | 管理員 | 刪除 |
| GET | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | 管理員 | 詳情 |
| PUT | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | 管理員 | 更新 |
| GET | `/api/v1/node-servers/:id/inbound-configs/:cfgId/share-links` | 管理員 | 分享連結和片段 |
| POST | `/api/v1/node-servers/:id/inbound-configs/:cfgId/users/:email/pause` | 管理員 | 暫停該用戶 |
| POST | `/api/v1/node-servers/:id/inbound-configs/:cfgId/users/:email/resume` | 管理員 | 恢復該用戶 |
| POST | `/api/v1/node-servers/:id/inbound-configs/reset-all-traffic` | 管理員 | 重置所有入站流量 |
| GET | `/api/v1/node-servers/:id/install-script` | 管理員 | 取得代理安裝腳本 |
| GET | `/api/v1/node-servers/:id/metrics-history` | 管理員 | 取得指標歷史記錄 |
| GET | `/api/v1/node-servers/:id/mirror-status` | 管理員 | 鏡像狀態 |
| POST | `/api/v1/node-servers/:id/nginx/batch-update-ports` | 管理員 | 批量更新連接埠 |
| POST | `/api/v1/node-servers/:id/nginx/generate-config` | 管理員 | 產生配置 |
| DELETE | `/api/v1/node-servers/:id/nginx/l4-rules` | 管理員 | 刪除所有L4規則 |
| POST | `/api/v1/node-servers/:id/nginx/save-and-apply` | 管理員 | 儲存並應用 |
| GET | `/api/v1/node-servers/:id/nginx/settings` | 管理員 | 讀取設定 |
| POST | `/api/v1/node-servers/:id/nginx/settings` | 管理員 | 儲存設定 |
| GET | `/api/v1/node-servers/:id/nginx/sites` | 管理員 | 站點列表 |
| POST | `/api/v1/node-servers/:id/nginx/sites` | 管理員 | 建立網站 |
| POST | `/api/v1/node-servers/:id/nginx/import` | 管理員 | 導入站點 |
| POST | `/api/v1/node-servers/:id/nginx/preview-site` | 管理員 | 預覽渲染的 Nginx 站點 |
| POST | `/api/v1/node-servers/:id/nginx/ssl/generate` | 管理員 | 產生 SSL 憑證文件 |
| DELETE | `/api/v1/node-servers/:id/nginx/sites/:siteId` | 管理員 | 刪除站點 |
| PUT | `/api/v1/node-servers/:id/nginx/sites/:siteId` | 管理員 | 更新站點 |
| DELETE | `/api/v1/node-servers/:id/nginx/sni-rules` | 管理員 | 刪除所有SNI規則 |
| POST | `/api/v1/node-servers/:id/nginx/sni-rules/toggle` | 管理員 | 切換 SNI 規則 |
| PUT | `/api/v1/node-servers/:id/node-type` | 管理員 | 更新節點類型 |
| GET | `/api/v1/node-servers/:id/online-users` | 管理員 | 線上用戶 |
| GET | `/api/v1/node-servers/:id/outbound-configs` | 管理員 | 清單 |
| POST | `/api/v1/node-servers/:id/outbound-configs` | 管理員 | 創建 |
| DELETE | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | 管理員 | 刪除 |
| GET | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | 管理員 | 詳情 |
| PUT | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | 管理員 | 更新 |
| PUT | `/api/v1/node-servers/:id/outbound-configs/reorder` | 管理員 | 排序 |
| GET | `/api/v1/node-servers/:id/port-hop/rules` | 管理員 | 埠跳變規則預覽 |
| GET | `/api/v1/node-servers/:id/protocols` | 管理員 | 協議列表 |
| GET | `/api/v1/node-servers/:id/protocols/:coreType` | 管理員 | 協定配置 |
| PUT | `/api/v1/node-servers/:id/protocols/:coreType` | 管理員 | 儲存協定配置 |
| GET | `/api/v1/node-servers/:id/protocols/:coreType/config-file` | 管理員 | 讀取核心設定檔 |
| GET | `/api/v1/node-servers/:id/protocols/:coreType/section/:section` | 管理員 | 閱讀協議部分 |
| PUT | `/api/v1/node-servers/:id/protocols/:coreType/section/:section` | 管理員 | 保存協議部分 |
| POST | `/api/v1/node-servers/:id/protocols/:coreType/validate` | 管理員 | 驗證配置（真正的核心試運行） |
| GET | `/api/v1/node-servers/:id/proxy-status` | 管理員 | 遠端代理狀態 |
| POST | `/api/v1/node-servers/:id/proxy/batch-update-ports` | 管理員 | 批量更新連接埠 |
| GET | `/api/v1/node-servers/:id/proxy/config` | 管理員 | 讀取配置 |
| PUT | `/api/v1/node-servers/:id/proxy/config` | 管理員 | 更新配置 |
| POST | `/api/v1/node-servers/:id/proxy/control` | 管理員 | 鍊式代理註冊控制 |
| POST | `/api/v1/node-servers/:id/proxy/download` | 管理員 | 下載代理核心版本 |
| POST | `/api/v1/node-servers/:id/proxy/generate-config` | 管理員 | 產生配置 |
| POST | `/api/v1/node-servers/:id/proxy/guardian` | 管理員 | 更新主管配置 |
| POST | `/api/v1/node-servers/:id/proxy/install` | 管理員 | 安裝代理 |
| DELETE | `/api/v1/node-servers/:id/proxy/l4-rules` | 管理員 | 刪除所有L4規則 |
| GET | `/api/v1/node-servers/:id/proxy/l4-settings` | 管理員 | L4設定 |
| POST | `/api/v1/node-servers/:id/proxy/l4-settings` | 管理員 | 儲存 L4 設定 |
| GET | `/api/v1/node-servers/:id/proxy/log-settings` | 管理員 | 日誌設定 |
| POST | `/api/v1/node-servers/:id/proxy/log-settings` | 管理員 | 儲存日誌設定 |
| GET | `/api/v1/node-servers/:id/proxy/nginx-settings` | 管理員 | nginx設定 |
| POST | `/api/v1/node-servers/:id/proxy/nginx-settings` | 管理員 | 儲存nginx設定 |
| GET | `/api/v1/node-servers/:id/proxy/releases` | 管理員 | 可用版本 |
| POST | `/api/v1/node-servers/:id/proxy/reload` | 管理員 | 重新載入配置 |
| POST | `/api/v1/node-servers/:id/proxy/restart` | 管理員 | 重新啟動 |
| GET | `/api/v1/node-servers/:id/proxy/sites` | 管理員 | 站點列表 |
| POST | `/api/v1/node-servers/:id/proxy/sites` | 管理員 | 建立網站 |
| DELETE | `/api/v1/node-servers/:id/proxy/sites/:siteId` | 管理員 | 刪除站點 |
| PUT | `/api/v1/node-servers/:id/proxy/sites/:siteId` | 管理員 | 更新站點 |
| DELETE | `/api/v1/node-servers/:id/proxy/sni-rules` | 管理員 | 刪除所有SNI規則 |
| POST | `/api/v1/node-servers/:id/proxy/sni-rules/toggle` | 管理員 | 切換 SNI 規則 |
| GET | `/api/v1/node-servers/:id/proxy/sni-settings` | 管理員 | SNI設定 |
| POST | `/api/v1/node-servers/:id/proxy/sni-settings` | 管理員 | 儲存SNI設定 |
| POST | `/api/v1/node-servers/:id/proxy/start` | 管理員 | 開始 |
| GET | `/api/v1/node-servers/:id/proxy/status` | 管理員 | 代理程式狀態 |
| POST | `/api/v1/node-servers/:id/proxy/stop` | 管理員 | 停止 |
| POST | `/api/v1/node-servers/:id/proxy/sync` | 管理員 | 同步代理配置 |
| GET | `/api/v1/node-servers/:id/rate-limit` | 管理員 | 讀取配置 |
| PUT | `/api/v1/node-servers/:id/rate-limit` | 管理員 | 更新配置 |
| GET | `/api/v1/node-servers/:id/rate-limit/status` | 管理員 | 狀態 |
| GET | `/api/v1/node-servers/:id/recover-mode` | 管理員 | 讀節點恢復模式 |
| PUT | `/api/v1/node-servers/:id/recover-mode` | 管理員 | 設定節點恢復模式 |
| POST | `/api/v1/node-servers/:id/revoke-cert` | 管理員 | 吊銷代理證書 |
| GET | `/api/v1/node-servers/:id/services` | 管理員 | 清單 |
| POST | `/api/v1/node-servers/:id/services` | 管理員 | 創建 |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId` | 管理員 | 刪除 |
| GET | `/api/v1/node-servers/:id/services/:serviceId` | 管理員 | 詳情 |
| PUT | `/api/v1/node-servers/:id/services/:serviceId` | 管理員 | 更新 |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId/crash-logs` | 管理員 | 清除崩潰日誌 |
| GET | `/api/v1/node-servers/:id/services/:serviceId/crash-logs` | 管理員 | 崩潰日誌列表 |
| GET | `/api/v1/node-servers/:id/services/:serviceId/crash-logs/:logId` | 管理員 | 崩潰日誌詳細信息 |
| GET | `/api/v1/node-servers/:id/services/:serviceId/endpoints` | 管理員 | 端點列表 |
| POST | `/api/v1/node-servers/:id/services/:serviceId/endpoints` | 管理員 | 建立端點 |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId/endpoints/:endpointId` | 管理員 | 刪除端點 |
| PUT | `/api/v1/node-servers/:id/services/:serviceId/endpoints/:endpointId` | 管理員 | 更新端點 |
| PUT | `/api/v1/node-servers/:id/services/:serviceId/guardian` | 管理員 | 更新主管配置 |
| GET | `/api/v1/node-servers/:id/services/:serviceId/process-status` | 管理員 | 取得處理狀態 |
| POST | `/api/v1/node-servers/:id/services/:serviceId/restart` | 管理員 | 重新啟動 |
| POST | `/api/v1/node-servers/:id/services/:serviceId/start` | 管理員 | 開始 |
| POST | `/api/v1/node-servers/:id/services/:serviceId/stop` | 管理員 | 停止 |
| GET | `/api/v1/node-servers/:id/services/process-status` | 管理員 | 所有行程狀態 |
| GET | `/api/v1/node-servers/:id/share-domains` | 管理員 | 共享該節點的域 |
| POST | `/api/v1/node-servers/:id/share-domains` | 管理員 | 創建 |
| DELETE | `/api/v1/node-servers/:id/share-domains/:domainId` | 管理員 | 刪除 |
| PUT | `/api/v1/node-servers/:id/share-domains/:domainId` | 管理員 | 更新 |
| DELETE | `/api/v1/node-servers/:id/share-domains/all` | 管理員 | 全部清除 |
| POST | `/api/v1/node-servers/:id/singbox/control/firewall/apply` | 管理員 | 應用程式連接埠跳躍防火牆規則 |
| POST | `/api/v1/node-servers/:id/singbox/control/firewall/remove` | 管理員 | 刪除連接埠跳躍防火牆規則 |
| GET | `/api/v1/node-servers/:id/singbox/connections` | 管理員 | 即時 sing-box 連線（clash_api，僅限本地節點） |
| POST | `/api/v1/node-servers/:id/speed-test` | 管理員 | 節點速度測試 |
| GET | `/api/v1/node-servers/:id/status` | 管理員 | 節點狀態 |
| POST | `/api/v1/node-servers/:id/sync` | 管理員 | 同步節點配置（產生和推送） |
| POST | `/api/v1/node-servers/:id/standby-sync` | 管理員 | 協調主設定與其備用夥伴 (primary_backup) |
| POST | `/api/v1/node-servers/:id/sync-data` | 超管理員 | 將面板資料推送到節點（用戶/訂閱/計劃完全同步） |
| POST | `/api/v1/node-servers/:id/sync-mirror` | 管理員 | 同步鏡像節點 |
| GET | `/api/v1/node-servers/:id/tags` | 管理員 | 標籤列表 |
| POST | `/api/v1/node-servers/:id/tags` | 管理員 | 添加標籤 |
| DELETE | `/api/v1/node-servers/:id/tags/:tagKey` | 管理員 | 刪除標籤 |
| GET | `/api/v1/node-servers/:id/traffic-history` | 管理員 | 交通歷史記錄 |
| POST | `/api/v1/node-servers/:id/traffic-resume` | 管理員 | 恢復因超限而暫停的流量 |
| GET | `/api/v1/node-servers/:id/traffic-usage` | 管理員 | 流量使用情況 |
| POST | `/api/v1/node-servers/:id/update-agent` | 管理員 | 升級代理 |
| GET | `/api/v1/node-servers/:id/used-server-names` | 管理員 | 正在使用的REALITY SNI列表 |
| GET | `/api/v1/node-servers/:id/waf/config` | 管理員 | 讀取配置 |
| PUT | `/api/v1/node-servers/:id/waf/config` | 管理員 | 儲存配置 |
| POST | `/api/v1/node-servers/:id/waf/disable-domain` | 管理員 | 禁用域保护 |
| GET | `/api/v1/node-servers/:id/waf/domain-config` | 管理員 | 域配置列表 |
| PUT | `/api/v1/node-servers/:id/waf/domain-config` | 管理員 | 批量保存域配置 |
| DELETE | `/api/v1/node-servers/:id/waf/domain-config/:domain` | 管理員 | 删除域配置 |
| GET | `/api/v1/node-servers/:id/waf/domain-config/:domain` | 管理員 | 域配置详细信息 |
| PUT | `/api/v1/node-servers/:id/waf/domain-config/:domain` | 管理員 | 保存域配置 |
| GET | `/api/v1/node-servers/:id/waf/domain-rules` | 管理員 | 域规则目录 |
| GET | `/api/v1/node-servers/:id/waf/domain-rules/:domain/:name` | 管理員 | 读取域规则文件 |
| PUT | `/api/v1/node-servers/:id/waf/domain-rules/:domain/:name` | 管理員 | 保存域规则文件 |
| GET | `/api/v1/node-servers/:id/waf/domain-status` | 管理員 | 域名保护状态 |
| POST | `/api/v1/node-servers/:id/waf/enable-domain` | 管理員 | 启用域保护 |
| POST | `/api/v1/node-servers/:id/waf/push` | 管理員 | 推送到該節點 |
| GET | `/api/v1/node-servers/:id/waf/rules` | 管理員 | 節點WAF規則列表 |
| DELETE | `/api/v1/node-servers/:id/waf/rules/:name` | 管理員 | 刪除節點WAF規則 |
| GET | `/api/v1/node-servers/:id/waf/rules/:name` | 管理員 | 節點WAF規則詳情 |
| PUT | `/api/v1/node-servers/:id/waf/rules/:name` | 管理員 | 保存節點WAF規則 |
| GET | `/api/v1/node-servers/:id/waf/status` | 管理員 | 節點WAF狀態 |
| PUT | `/api/v1/node-servers/:id/waf/status` | 管理員 | 撥動節點 WAF |
| POST | `/api/v1/node-servers/:id/waf/sync` | 管理員 | 同步到該節點 |
| GET | `/api/v1/node-servers/:id/warp` | 管理員 | 清單 |
| DELETE | `/api/v1/node-servers/:id/warp/:warpId` | 管理員 | 刪除 |
| GET | `/api/v1/node-servers/:id/warp/config` | 管理員 | 讀取配置 |
| POST | `/api/v1/node-servers/:id/warp/generate-config` | 管理員 | 產生配置 |
| POST | `/api/v1/node-servers/:id/warp/register` | 管理員 | 註冊 |
| POST | `/api/v1/node-servers/:id/warp/masque/register` | 管理員 | 註冊 WARP MASQUE 設定檔（第 1 步，共 2 步） |
| POST | `/api/v1/node-servers/:id/warp/:warpId/masque-outbound` | 管理員 | 從 WARP 設定檔建立 sing-box MASQUE 出站 |
| GET | `/api/v1/node-servers/:id/warp/status` | 管理員 | 狀態 |
| GET | `/api/v1/node-servers/:id/xray/control/balancer/:tag` | 管理員 | 獲取平衡器信息 |
| POST | `/api/v1/node-servers/:id/xray/control/balancer/:tag` | 管理員 | 設定平衡器目標 |
| POST | `/api/v1/node-servers/:id/xray/control/firewall/apply` | 管理員 | 應用程式連接埠跳躍防火牆規則 |
| POST | `/api/v1/node-servers/:id/xray/control/firewall/remove` | 管理員 | 刪除連接埠跳躍防火牆規則 |
| POST | `/api/v1/node-servers/:id/xray/control/inbound` | 管理員 | 新增入站 |
| DELETE | `/api/v1/node-servers/:id/xray/control/inbound/:tag` | 管理員 | 刪除入站 |
| POST | `/api/v1/node-servers/:id/xray/control/inbound/:tag/user` | 管理員 | 新增用戶 |
| DELETE | `/api/v1/node-servers/:id/xray/control/inbound/:tag/user/:email` | 管理員 | 刪除用戶 |
| POST | `/api/v1/node-servers/:id/xray/control/outbound` | 管理員 | 新增出站 |
| DELETE | `/api/v1/node-servers/:id/xray/control/outbound/:tag` | 管理員 | 刪除出站 |
| POST | `/api/v1/node-servers/:id/xray/control/routing` | 管理員 | 請求路線 |
| POST | `/api/v1/node-servers/:id/xray/control/routing/rule` | 管理員 | 新增路由規則 |
| DELETE | `/api/v1/node-servers/:id/xray/control/routing/rule/:tag` | 管理員 | 刪除路由規則 |
| GET | `/api/v1/node-servers/:id/xray/control/routing/rules` | 管理員 | 列出路由規則 |
| POST | `/api/v1/node-servers/:id/xray/control/sync` | 管理員 | 手動觸發xray配置同步 |
| GET | `/api/v1/node-servers/:id/xray/stats` | 管理員 | 取得統計數據 |
| POST | `/api/v1/node-servers/:id/xray/stats/reset/:email` | 管理員 | 重置用戶統計數據 |
| GET | `/api/v1/node-servers/:id/xray/stats/users` | 管理員 | 取得用戶統計數據 |
| GET | `/api/v1/node-servers/billable` | 管理員 | 可計費節點列表 |
| GET | `/api/v1/node-servers/ca-cert` | 管理員 | CA憑證 |
| GET | `/api/v1/node-servers/ca-status` | 管理員 | CA狀態 |
| GET | `/api/v1/node-servers/dashboard` | 管理員 | 儀表板狀態 |
| POST | `/api/v1/node-servers/distribute-cert` | 管理員 | 向所有节点分发证书 |
| POST | `/api/v1/node-servers/failover-check` | 管理員 | 觸發故障轉移檢查 |
| GET | `/api/v1/node-servers/metrics/aggregated` | 管理員 | 聚合指標 |
| POST | `/api/v1/node-servers/migrate-cert` | 管理員 | 遷移憑證 |
| GET | `/api/v1/node-servers/online-summary` | 管理員 | 網路上總結 |
| GET | `/api/v1/node-servers/online-users` | 管理員 | 所有線上用戶 |
| GET | `/api/v1/node-servers/pending-agent-updates` | 管理員 | 代理等待升級 |
| POST | `/api/v1/node-servers/recommend-region` | 管理員 | 推薦地區 |
| POST | `/api/v1/node-servers/select-nodes` | 管理員 | 選擇訂閱節點 |
| POST | `/api/v1/node-servers/speed-test-all` | 管理員 | 速度測試所有節點 |
| GET | `/api/v1/node-servers/speed-test-results` | 管理員 | 速度測試結果 |
| GET | `/api/v1/node-servers/traffic/stats` | 管理員 | 流量統計 |
| POST | `/api/v1/node-servers/update-all-agents` | 管理員 | 升級所有代理 |
| DELETE | `/api/v1/node-servers/users/:client_id` | 管理員 | 同步-刪除客戶 |
| GET | `/api/v1/node-servers/users/retry-queue` | 管理員 | 重試隊列狀態 |
| POST | `/api/v1/node-servers/users/retry-queue/process` | 管理員 | 進程重試佇列 |
| POST | `/api/v1/node-servers/users/sync` | 管理員 | 同步單一客戶 |
| POST | `/api/v1/node-servers/validate-chain-config` | 管理員 | 驗證連結配置 |

### 節點審計日誌

節點端操作審計和導出。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| GET | `/api/v1/node-audit-logs` | 管理員 | 節點審計日誌 |
| GET | `/api/v1/node-audit-logs/export` | 管理員 | 匯出節點稽核日誌 |

### 節點（舊模型，已退役）

> `/api/v1/nodes` CRUD於2026年10月20日退役：節點透過`/api/v1/node-servers`統一管理。
> 線上用戶查詢仍為`/api/v1/node-servers/{online-users,online-summary,:id/online-users}`。

### 地區概況

按區域聚合的節點視圖。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| GET | `/api/v1/regions/:region/servers` | 已登入 | 該區域的節點列表 |
| GET | `/api/v1/regions/overview` | 已登入 | 地區概況 |

### 訂閱

訂閱記錄管理：撤銷、輪替令牌、存取日誌、使用重設。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| GET | `/api/v1/subscriptions` | 已登入 | 清單 |
| POST | `/api/v1/subscriptions` | 操作員 | 創建 |
| DELETE | `/api/v1/subscriptions/:id` | 操作員 | 刪除 |
| GET | `/api/v1/subscriptions/:id` | 已登入 | 詳情 |
| PUT | `/api/v1/subscriptions/:id` | 操作員 | 更新 |
| GET | `/api/v1/subscriptions/:id/access-logs` | 已登入 | 訪問日誌 |
| POST | `/api/v1/subscriptions/:id/enable` | 操作員 | 啟用 |
| POST | `/api/v1/subscriptions/:id/reset-usage` | 操作員 | 重置使用情況 |
| POST | `/api/v1/subscriptions/:id/restore` | 操作員 | 履歷 |
| POST | `/api/v1/subscriptions/:id/revoke` | 操作員 | 撤銷 |
| POST | `/api/v1/subscriptions/:id/rotate` | 操作員 | 旋轉 |
| GET | `/api/v1/subscriptions/:id/rotations` | 已登入 | 輪換歷史 |

### 訂閱格式

> 管理端`/api/v1/subscription-formats` CRUD已於2026年10月20日退役：實際入口點是
> `/api/v1/mihomo/configs`（訂閱格式中心）。 `subscription_formats` 表仍然存在，
> 僅用作公共 `/api/v1/public/client-configs` 端點的種子資料。

### 訂閱端點

專用訂閱域（反向代理到面板的 `/sub` 路徑）。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| GET | `/api/v1/subscription-endpoints` | 已登入 | 清單 |
| POST | `/api/v1/subscription-endpoints` | 管理員 | 創建 |
| DELETE | `/api/v1/subscription-endpoints/:id` | 管理員 | 刪除 |
| GET | `/api/v1/subscription-endpoints/:id` | 已登入 | 詳情 |
| PUT | `/api/v1/subscription-endpoints/:id` | 管理員 | 更新 |

### 共享域名

客户端链接中使用的访问域/IP 池。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| GET | `/api/v1/share-domains` | 已登入 | 清單 |
| POST | `/api/v1/share-domains` | 管理員 | 創建 |
| DELETE | `/api/v1/share-domains/:id` | 管理員 | 刪除 |
| PUT | `/api/v1/share-domains/:id` | 管理員 | 更新 |
| DELETE | `/api/v1/share-domains/all` | 管理員 | 全部清除 |
| POST | `/api/v1/share-domains/seed` | 管理員 | 初始化預設數據 |
| GET | `/api/v1/share-domains/dns-providers` | 登入 | 每個提供者的 DNS 寫入能力表（已實施/已驗證） |
`PUT /api/v1/share-domains/:id` 也接受 `dns_channel_id`、`dns_record_name` 和 `dns_proxied`：當主/備用故障轉移或切回時，條目 A 記錄指向新的主伺服器。只有已實施 DNS 編寫器的供應商才接受這些欄位（`cloudflare` 已驗證；alidns / DNSPod / GoDaddy / Namecheap / Route53 / Google / Huawei Cloud 已實施但未驗證）。能力表請參考`GET /api/v1/share-domains/dns-providers`。


### Cloudflare IP

Cloudflare CDN IP池和源管理。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| DELETE | `/api/v1/cfip/entries` | 管理員 | 清除條目 |
| GET | `/api/v1/cfip/entries` | 已登入 | 參賽名單 |
| POST | `/api/v1/cfip/entries` | 管理員 | 新增條目 |
| DELETE | `/api/v1/cfip/entries/:id` | 管理員 | 刪除條目 |
| POST | `/api/v1/cfip/fetch-all` | 管理員 | 獲取所有來源 |
| GET | `/api/v1/cfip/probe-config` | 已登入 | CF 首選 IP 探測配置（入口節點 + 安裝的撥號器） |
| PUT | `/api/v1/cfip/probe-config` | 管理員 | 儲存探測間隔/延遲限制/入口節點/撥號器 |
| POST | `/api/v1/cfip/check` | 管理員 | 立即探測所有首選域/IP |
| POST | `/api/v1/cfip/seed` | 管理員 | 初始化預設數據 |
| GET | `/api/v1/cfip/sources` | 已登入 | 資料來源列表 |
| POST | `/api/v1/cfip/sources` | 管理員 | 建立資料來源 |
| DELETE | `/api/v1/cfip/sources/:id` | 管理員 | 刪除資料來源 |
| PUT | `/api/v1/cfip/sources/:id` | 管理員 | 更新資料來源 |
| POST | `/api/v1/cfip/sources/:id/fetch` | 管理員 | 取得此來源 |

### 訂閱配置（Mihomo/Clash）

訂閱範本配置：群組、規則、規則提供者、YAML 預覽。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| GET | `/api/v1/mihomo/configs` | 已登入 | 清單 |
| POST | `/api/v1/mihomo/configs` | 管理員 | 創建 |
| DELETE | `/api/v1/mihomo/configs/:id` | 管理員 | 刪除 |
| GET | `/api/v1/mihomo/configs/:id` | 已登入 | 詳情 |
| PUT | `/api/v1/mihomo/configs/:id` | 管理員 | 更新 |
| GET | `/api/v1/mihomo/configs/:id/rules` | 已登入 | 規則列表 |
| POST | `/api/v1/mihomo/configs/:id/rules` | 管理員 | 建立規則 |
| PUT | `/api/v1/mihomo/configs/:id/rules/order` | 管理員 | 重新排序規則 |
| PUT | `/api/v1/mihomo/rules/:id` | 管理員 | 更新規則 |
| DELETE | `/api/v1/mihomo/rules/:id` | 管理員 | 刪除規則 |

### 儲值地點

第三方支付網站整合：簽署金鑰、驗證文件、網域驗證、整合文件。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| GET | `/api/v1/charge-sites` | 管理員 | 站點列表 |
| POST | `/api/v1/charge-sites` | 管理員 | 建立網站 |
| DELETE | `/api/v1/charge-sites/:id` | 管理員 | 刪除站點 |
| GET | `/api/v1/charge-sites/:id` | 管理員 | 網站詳情 |
| PUT | `/api/v1/charge-sites/:id` | 管理員 | 更新站點 |
| POST | `/api/v1/charge-sites/:id/clear-prev-secret` | 管理員 | 清除舊密鑰 |
| POST | `/api/v1/charge-sites/:id/generate-verify` | 管理員 | 產生驗證文件 |
| POST | `/api/v1/charge-sites/:id/generate-verify-file` | 管理員 | 產生驗證文件 |
| GET | `/api/v1/charge-sites/:id/integration-doc` | 管理員 | 取得整合文檔 |
| POST | `/api/v1/charge-sites/:id/rotate-secret` | 管理員 | 輪換簽名金鑰 |
| POST | `/api/v1/charge-sites/:id/verify-dns` | 管理員 | 驗證網域 DNS |
| POST | `/api/v1/charge-sites/:id/verify-domain` | 管理員 | 驗證網域 HTTP 質詢 |
| POST | `/api/v1/charge-sites/:id/verify-domain-dns` | 管理員 | 驗證網域 DNS |
| POST | `/api/v1/charge-sites/:id/verify-domain-http` | 管理員 | 驗證網域 HTTP 質詢 |

### 儲值訂單

儲值訂單查詢、退款、CSV匯出。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| GET | `/api/v1/charge-orders` | 管理員 | 訂單清單 |
| GET | `/api/v1/charge-orders/:id` | 管理員 | 訂單詳情 |
| POST | `/api/v1/charge-orders/:id/refund` | 管理員 | 退款 |
| GET | `/api/v1/charge-orders/export-csv` | 管理員 | 出口儲值訂單CSV |

### 儲值卡

批次卡片產生、使用記錄、CSV匯出。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| GET | `/api/v1/charge-cards` | 管理員 | 卡牌列表 |
| POST | `/api/v1/charge-cards` | 管理員 | 創建卡片 |
| DELETE | `/api/v1/charge-cards/:id` | 管理員 | 刪除卡 |
| GET | `/api/v1/charge-cards/:id/usages` | 管理員 | 卡片使用記錄 |
| POST | `/api/v1/charge-cards/batch` | 管理員 | 大量生成卡片 |
| GET | `/api/v1/charge-cards/export-csv` | 管理員 | 出口卡CSV |

### 收費回呼事件

回調事件審核。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| GET | `/api/v1/charge-callback-events` | 已登入 | 回呼事件列表 |

### 收費回調重試

手動重試失敗的回呼。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| GET | `/api/v1/charge-callback-retries` | 已登入 | 失敗回呼列表 |
| POST | `/api/v1/charge-callback-retries/:id/retry` | 管理員 | 重試回調 |

### 客戶餘額

餘額和交易的管理視圖。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| GET | `/api/v1/client-balances` | 已登入 | 客戶餘額清單 |
| GET | `/api/v1/client-balances/transactions` | 已登入 | 餘額交易（管理） |
| GET | `/api/v1/client-balances/transactions/export-csv` | 已登入 | 出口餘額交易CSV |

### 禮物

發放可領取的禮物（天數/流量）。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| GET | `/api/v1/client-gifts` | 已登入 | 禮物清單 |
| POST | `/api/v1/client-gifts` | 管理員 | 製作禮物 |
| POST | `/api/v1/client-gifts/batch` | 管理員 | 批量製作禮物 |
| GET | `/api/v1/client-gifts/search-clients` | 已登入 | 搜尋客戶 |

### 推薦配置

推薦佣金配置和統計。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| GET | `/api/v1/referral/config` | 已登入 | 推薦配置 |
| PUT | `/api/v1/referral/config` | 管理員 | 儲存推薦配置 |
| GET | `/api/v1/referral/records` | 已登入 | 推薦記錄列表 |
| GET | `/api/v1/referral/stats` | 已登入 | 推薦統計 |

### 推薦提款

審查提款請求。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| GET | `/api/v1/referral-withdrawals` | 管理員 | 提款請求列表 |
| POST | `/api/v1/referral-withdrawals/:id/approve` | 管理員 | 批准提款 |
| POST | `/api/v1/referral-withdrawals/:id/reject` | 管理員 | 拒絕提款 |

### IP 允許清單（摘要）

所有客戶的 IP 允許清單視圖。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| GET | `/api/v1/ip-whitelists` | 已登入 | 所有客戶 IP 允許列表 |

### 客戶 OAuth 綁定

查詢客戶第三方帳號綁定。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| GET | `/api/v1/client-oauth-bindings` | 已登入 | 客戶 OAuth 綁定列表 |

### 門票（管理員）

工單回覆、分配和關閉。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| GET | `/api/v1/tickets` | 已登入 | 門票清單 |
| GET | `/api/v1/tickets/:id` | 已登入 | 門票詳情 |
| POST | `/api/v1/tickets/:id/assign` | 操作員 | 分配票證 |
| POST | `/api/v1/tickets/:id/close` | 操作員 | 關閉票證 |
| POST | `/api/v1/tickets/:id/reply` | 操作員 | 回覆票 |

### 文章

入口網站文章的 CRUD。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| GET | `/api/v1/articles` | 已登入 | 文章列表 |
| POST | `/api/v1/articles` | 管理員 | 創建文章 |
| DELETE | `/api/v1/articles/:id` | 管理員 | 刪除文章 |
| GET | `/api/v1/articles/:id` | 已登入 | 文章詳情 |
| PUT | `/api/v1/articles/:id` | 管理員 | 更新文章 |

### 共享帳戶

共享帳戶（串流媒體等）的發行和修復記錄。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| GET | `/api/v1/shared-accounts` | 已登入 | 共享帳戶列表 |
| POST | `/api/v1/shared-accounts` | 管理員 | 建立共享帳戶 |
| DELETE | `/api/v1/shared-accounts/:id` | 管理員 | 刪除共享帳戶 |
| GET | `/api/v1/shared-accounts/:id` | 已登入 | 共享帳戶詳細信息 |
| PUT | `/api/v1/shared-accounts/:id` | 管理員 | 更新共享帳戶 |
| GET | `/api/v1/shared-accounts/:id/repair-logs` | 已登入 | 維修記錄 |

### 證書

公共憑證（ACME）頒發、上傳、更新和匯出；支援在特定節點上發行。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| GET | `/api/v1/certificates` | 管理員 | 清單 |
| POST | `/api/v1/certificates` | 管理員 | 創建 |
| DELETE | `/api/v1/certificates/:id` | 管理員 | 刪除 |
| GET | `/api/v1/certificates/:id` | 管理員 | 詳情 |
| PUT | `/api/v1/certificates/:id` | 管理員 | 更新 |
| GET | `/api/v1/certificates/:id/download` | 管理員 | 下載 |
| POST | `/api/v1/certificates/:id/renew` | 管理員 | 續訂客戶 |
| GET | `/api/v1/certificates/check-port` | 管理員 | 檢查連接埠 |
| GET | `/api/v1/certificates/detect-ip` | 管理員 | 探針IP |
| GET | `/api/v1/certificates/export` | 管理員 | 出口 |
| POST | `/api/v1/certificates/issue` | 管理員 | 問題。機身：`{"domains":["example.com","*.example.com"],"challenge_type":"dns-01","dns_channel_id":2,"key_type":"rsa","key_bits":4096}`。 `challenge_type`預設為http-01；`dns_channel_id` 選擇 DNS 通道（已設定的 DNS 提供者 + 憑證） |
| POST | `/api/v1/certificates/issue-on-node` | 管理員 | 發給節點 |
| POST | `/api/v1/certificates/upload` | 管理員 | 上傳 |
| GET | `/api/v1/certificate-tasks/:id` | 管理員 | 查詢後台任務狀態 |

### 證書頻道

ACME 通道（CA）配置。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| GET | `/api/v1/cert-channels` | 管理員 | 清單 |
| POST | `/api/v1/cert-channels` | 管理員 | 創建 |
| DELETE | `/api/v1/cert-channels/:id` | 管理員 | 刪除 |
| PUT | `/api/v1/cert-channels/:id` | 管理員 | 更新 |
| GET | `/api/v1/cert-channels/providers` | 管理員 | 提供者 |

### EAB配置

綁定外部帳戶的憑證，例如ZeroSSL / Google。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| GET | `/api/v1/eab-configs` | 管理員 | 清單 |
| POST | `/api/v1/eab-configs` | 管理員 | 創建 |
| DELETE | `/api/v1/eab-configs/:id` | 管理員 | 刪除 |
| PUT | `/api/v1/eab-configs/:id` | 管理員 | 更新 |
| POST | `/api/v1/eab-configs/fetch-google` | 管理員 | 從 GCP 取得 Google EAB |
| POST | `/api/v1/eab-configs/fetch-zerossl` | 管理員 | 一鍵透過電子郵件取得 ZeroSSL EAB |

### ACME帳戶

面板端ACME帳號（聯絡信箱）管理。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| GET | `/api/v1/acme-accounts` | 管理員 | 清單 |
| POST | `/api/v1/acme-accounts` | 管理員 | 創建 |
| DELETE | `/api/v1/acme-accounts/:id` | 管理員 | 刪除 |
| PUT | `/api/v1/acme-accounts/:id` | 管理員 | 更新 |
| GET | `/api/v1/acme-accounts/default` | 管理員 | 取得預設值 |

### 內建CA

內部面板CA：代理證書頒發/續約/吊銷、CRL匯出、分發佇列。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| GET | `/api/v1/ca/agent-certs` | 管理員 | 代理證書清單 |
| POST | `/api/v1/ca/agent-certs/:server_id/renew` | 管理員 | 更新代理證書 |
| GET | `/api/v1/ca/distribution-queue` | 管理員 | 證書分發佇列 |
| DELETE | `/api/v1/ca/distribution-queue/:id` | 管理員 | 刪除分配佇列條目 |
| POST | `/api/v1/ca/distribution-queue/:id/retry` | 管理員 | 重試證書分發 |
| GET | `/api/v1/ca/revocations` | 管理員 | 撤銷清單 |
| GET | `/api/v1/ca/revocations/export` | 管理員 | 匯出憑證撤銷列表 |
| GET | `/api/v1/ca/status` | 管理員 | CA狀態 |

### 地理規則

同步和交付 geoip/geosite 資料來源和檔案。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| GET | `/api/v1/geo-rules/files` | 已登入 | 文件列表 |
| DELETE | `/api/v1/geo-rules/files/:fileId` | 管理員 | 刪除文件 |
| PUT | `/api/v1/geo-rules/files/:fileId/auto-sync` | 管理員 | 切換自動同步 |
| POST | `/api/v1/geo-rules/files/:fileId/download` | 管理員 | 下載 |
| GET | `/api/v1/geo-rules/files/:fileId/download-file` | 已登入 | 下載文件 |
| GET | `/api/v1/geo-rules/github-assets` | 已登入 | 取得 GitHub 儲存庫資產 |
| POST | `/api/v1/geo-rules/push-all` | 管理員 | 推送到所有節點 |
| POST | `/api/v1/geo-rules/seed` | 管理員 | 初始化預設數據 |
| GET | `/api/v1/geo-rules/sources` | 已登入 | 資料來源列表 |
| POST | `/api/v1/geo-rules/sources` | 管理員 | 建立資料來源 |
| DELETE | `/api/v1/geo-rules/sources/:sourceId` | 管理員 | 刪除資料來源 |
| PUT | `/api/v1/geo-rules/sources/:sourceId` | 管理員 | 更新資料來源 |
| GET | `/api/v1/geo-rules/sources/:sourceId/assets` | 已登入 | 獲取釋放資產 |
| POST | `/api/v1/geo-rules/sources/:sourceId/files` | 管理員 | 新增文件 |
| PUT | `/api/v1/geo-rules/sources/:sourceId/schedule` | 管理員 | 切換計劃同步 |
| POST | `/api/v1/geo-rules/sources/:sourceId/select` | 管理員 | 選擇文件 |
| POST | `/api/v1/geo-rules/sources/:sourceId/sync` | 管理員 | 同步此資料來源 |
| GET | `/api/v1/geo-rules/status` | 已登入 | 狀態 |
| POST | `/api/v1/geo-rules/sync-all` | 管理員 | 同步全部 |
| GET | `/api/v1/geo-rules/sync-status/:key` | 已登入 | 同步進度 |

### WAF規則庫

全域WAF規則：GitHub來源、同步、傳遞到所有節點。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| GET | `/api/v1/waf/github-source` | 已登入 | GitHub資料來源 |
| PUT | `/api/v1/waf/github-source` | 管理員 | 保存GitHub資料來源 |
| POST | `/api/v1/waf/push-all` | 管理員 | 推送到所有節點 |
| GET | `/api/v1/waf/rules` | 已登入 | 規則列表 |
| DELETE | `/api/v1/waf/rules/:name` | 管理員 | 刪除規則 |
| GET | `/api/v1/waf/rules/:name` | 已登入 | 取得規則 |
| PUT | `/api/v1/waf/rules/:name` | 管理員 | 保存規則 |
| POST | `/api/v1/waf/sync-all` | 管理員 | 同步全部 |
| POST | `/api/v1/waf/sync-github` | 管理員 | 從 GitHub 同步 |

### 日誌管理

節點/核心日誌讀取和串流、清理策略、日誌寫入器控制。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| POST | `/api/v1/log-manage/cleanup-all` | 管理員 | 現在清理一切 |
| GET | `/api/v1/log-manage/cleanup-rules` | 管理員 | 清理規則列表 |
| PUT | `/api/v1/log-manage/cleanup-rules/:id` | 管理員 | 更新清理規則 |
| POST | `/api/v1/log-manage/cleanup-table/:table` | 管理員 | 立即清理指定表 |
| GET | `/api/v1/log-manage/core-log-output` | 管理員 | 核心日誌輸出配置 |
| PUT | `/api/v1/log-manage/core-log-output` | 管理員 | 儲存核心日誌輸出配置 |
| GET | `/api/v1/log-manage/core/:type/stdout-log` | 管理員 | 讀取核心標準輸出日誌 |
| GET | `/api/v1/log-manage/core/:type/stdout-stream` | 管理員 | 流核心標準輸出 |
| POST | `/api/v1/log-manage/discover` | 管理員 | 發現節點日誌文件 |
| GET | `/api/v1/log-manage/log-output` | 管理員 | 日誌輸出配置 |
| PUT | `/api/v1/log-manage/log-output` | 管理員 | 保存日誌輸出配置 |
| GET | `/api/v1/log-manage/nodes/:serverId/logs` | 管理員 | 節點日誌檔案列表 |
| GET | `/api/v1/log-manage/nodes/:serverId/logs/:type` | 管理員 | 讀取節點日誌 |
| POST | `/api/v1/log-manage/nodes/:serverId/logs/:type/rotate` | 管理員 | 輪換節點日誌 |
| GET | `/api/v1/log-manage/nodes/:serverId/logs/:type/stream` | 管理員 | 流節點日誌 |
| GET | `/api/v1/log-manage/policies` | 管理員 | 政策清單 |
| POST | `/api/v1/log-manage/policies` | 管理員 | 創建政策 |
| DELETE | `/api/v1/log-manage/policies/:id` | 管理員 | 刪除政策 |
| GET | `/api/v1/log-manage/policies/:id` | 管理員 | 政策詳情 |
| PUT | `/api/v1/log-manage/policies/:id` | 管理員 | 更新政策 |
| GET | `/api/v1/log-manage/policies/:id/jobs` | 管理員 | 清理作業清單 |
| GET | `/api/v1/log-manage/policies/:id/log` | 管理員 | 讀取日誌 |
| GET | `/api/v1/log-manage/policies/:id/log-stream` | 管理員 | 流日誌 |
| POST | `/api/v1/log-manage/policies/:id/rotate` | 管理員 | 旋轉 |
| GET | `/api/v1/log-manage/proxy/:type/stdout-log` | 管理員 | 讀取核心標準輸出日誌 |
| GET | `/api/v1/log-manage/proxy/:type/stdout-stream` | 管理員 | 流核心標準輸出 |
| GET | `/api/v1/log-manage/scheduler-config` | 管理員 | 調度配置 |
| PUT | `/api/v1/log-manage/scheduler-config` | 管理員 | Update scheduling config |
| GET | `/api/v1/log-manage/status` | 管理員 | 狀態 |
| POST | `/api/v1/log-manage/sync` | 管理員 | Sync cleanup policies |
| GET | `/api/v1/log-manage/writers` | 管理員 | Writer status |

### 備份

資料庫/完整備份、復原、排程作業、FTP/S3/WebDAV 遠端儲存。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| DELETE | `/api/v1/backups/:id` | 超管理員 | Delete backup |
| GET | `/api/v1/backups/:id/download` | 管理員 | 下載備份 |
| POST | `/api/v1/backups/:id/restore` | 超管理員 | 從備份中恢復 |
| POST | `/api/v1/backups/:id/upload-ftp` | 超管理員 | Upload to FTP |
| POST | `/api/v1/backups/:id/upload-s3` | 超管理員 | 上傳至S3 |
| POST | `/api/v1/backups/:id/upload-webdav` | 超管理員 | 上傳至WebDAV |
| POST | `/api/v1/backups/create` | 管理員 | 建立完整備份（使用資料庫轉儲） |
| GET | `/api/v1/backups/list` | 管理員 | 完整備份列表 |
| POST | `/api/v1/backups/remote-restore` | 超管理員 | 從遠端恢復 |
| GET | `/api/v1/backups/restore-status` | 超管理員 | 恢復進度 |
| GET | `/api/v1/backups/schedules` | 管理員 | 預定工作清單 |
| POST | `/api/v1/backups/schedules` | 管理員 | 建立預定作業 |
| DELETE | `/api/v1/backups/schedules/:id` | 管理員 | 刪除預定作業 |
| PUT | `/api/v1/backups/schedules/:id` | 管理員 | 更新預定作業 |
| GET | `/api/v1/backups/storages` | 管理員 | 儲存配置列表 |
| POST | `/api/v1/backups/storages` | 管理員 | 建立儲存配置 |
| DELETE | `/api/v1/backups/storages/:id` | 管理員 | 刪除儲存配置 |
| PUT | `/api/v1/backups/storages/:id` | 管理員 | 更新儲存配置 |
| GET | `/api/v1/backups/storages/:id/files` | 管理員 | 遠端備份檔案列表 |
| POST | `/api/v1/backups/upload-restore` | 超管理員 | 上傳備份並恢復 |

### 系統設定

資料庫配置和遷移、面板設定、JWT 金鑰、SMTP/Telegram 測試、通知範本、線上升級（主要是管理）。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| PUT | `/api/v1/system/bootstrap-finish` | 管理員 | 完成首次啟動設定 |
| GET | `/api/v1/system/bootstrap-state` | 管理員 | 首次啟動狀態 |
| GET | `/api/v1/system/config-file/:coreType` | 管理員 | 讀取核心設定檔 |
| GET | `/api/v1/system/db-config` | 管理員 | 資料庫配置 |
| PUT | `/api/v1/system/db-config` | 管理員 | 更新資料庫配置 |
| POST | `/api/v1/system/db-config/migrate` | 超管理員 | 遷移資料庫數據 |
| POST | `/api/v1/system/db-config/switch` | 超管理員 | 切換資料庫（一步遷移） |
| POST | `/api/v1/system/db-config/test` | 管理員 | 測試資料庫連接 |
| GET | `/api/v1/system/db-pool-stats` | 管理員 | 連線池統計 |
| GET | `/api/v1/system/db-stats` | 管理員 | 資料庫統計 |
| GET | `/api/v1/system/db-stats/ping` | 管理員 | 資料庫偵測 |
| POST | `/api/v1/system/db-stats/reset` | 管理員 | 重置資料庫統計信息 |
| GET | `/api/v1/system/encryption-key-status` | 管理員 | 加密金鑰狀態 |
| GET | `/api/v1/system/env-config` | 管理員 | .env 配置 |
| PUT | `/api/v1/system/env-config` | 管理員 | 更新 .env 配置 |
| POST | `/api/v1/system/generate-encryption-key` | 超管理員 | 產生加密金鑰 |
| GET | `/api/v1/system/github-config` | 管理員 | GitHub配置 |
| PUT | `/api/v1/system/github-config` | 管理員 | 更新GitHub配置 |
| GET | `/api/v1/system/jwt-secret-status` | 管理員 | JWT按鍵狀態 |
| GET | `/api/v1/system/maintenance` | 公有 | 維護模式狀態（公開） |
| PUT | `/api/v1/system/maintenance` | 管理員 | 切換維護模式 |
| POST | `/api/v1/system/migrate-data` | 超管理員 | 遷移資料 |
| GET | `/api/v1/system/notification-config` | 管理員 | 通知配置 |
| PUT | `/api/v1/system/notification-config` | 管理員 | 更新通知配置 |
| GET | `/api/v1/system/notification-templates` | 管理員 | 模板列表 |
| POST | `/api/v1/system/notification-templates` | 管理員 | 建立模板 |
| DELETE | `/api/v1/system/notification-templates/:id` | 管理員 | 刪除模板 |
| PUT | `/api/v1/system/notification-templates/:id` | 管理員 | 更新模板 |
| POST | `/api/v1/system/notification-templates/:id/broadcast` | 管理員 | 通过模板广播通知 |
| POST | `/api/v1/system/notification-templates/:id/send` | 管理員 | 發送測試通知 |
| GET | `/api/v1/system/panel-config` | 管理員 | 面板配置 |
| PUT | `/api/v1/system/panel-config` | 管理員 | 更新面板配置 |
| GET | `/api/v1/system/reality-targets` | 管理員 | REALITY目标清单 |
| PUT | `/api/v1/system/reality-targets` | 管理員 | 更新REALITY目标 |
| POST | `/api/v1/system/reality-targets/reset` | 超管理員 | 將 REALITY 目標重設為預設值 |
| GET | `/api/v1/system/reality-local-sni` | 管理員 | 讀取Reality本地SNI配置 |
| PUT | `/api/v1/system/reality-local-sni` | 管理員 | 更新Reality本地SNI配置 |
| POST | `/api/v1/system/regenerate-jwt-secret` | 超管理員 | 重新產生JWT金鑰 |
| GET | `/api/v1/system/releases` | 管理員 | 可用版本 |
| POST | `/api/v1/system/restart` | 超管理員 | 重新啟動 |
| POST | `/api/v1/system/rollback` | 超管理員 | 系統復原 |
| GET | `/api/v1/system/settings` | 管理員 | 讀取設定 |
| PUT | `/api/v1/system/settings/batch` | 管理員 | 批量更新設定 |
| GET | `/api/v1/system/stats` | 管理員 | 系統狀態 |
| GET | `/api/v1/system/subscription-billing-templates` | 管理員 | 訂閱計費模板 |
| PUT | `/api/v1/system/subscription-billing-templates` | 管理員 | 更新訂閱計費模板 |
| POST | `/api/v1/system/telegram-chat-id` | 管理員 | 取得 Telegram 聊天 ID |
| POST | `/api/v1/system/test-smtp` | 管理員 | 測試SMTP |
| POST | `/api/v1/system/test-telegram` | 管理員 | 測試Telegram |
| POST | `/api/v1/system/update` | 超管理員 | 系統升級 |
| GET | `/api/v1/system/update-jobs` | 管理員 | 升級工作清單 |
| POST | `/api/v1/system/upload-mtls-ca` | 管理員 | 上傳 mTLS CA 憑證 |

### 安全審計

審計事件查詢和匯出。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| GET | `/api/v1/security/audit-events` | 管理員 | 審核事件列表 |
| GET | `/api/v1/security/audit-events/export` | 管理員 | 導出審核事件 |
| GET | `/api/v1/security/subscription-access-logs` | 管理員 | 订阅获取日志（按订阅 ID/IP/客户电子邮件过滤） |

### 指標

面板聚合指標（節點/憑證/服務）。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| GET | `/api/v1/metrics/certificates` | 管理員 | 證書指標 |
| GET | `/api/v1/metrics/nodes` | 管理員 | 節點指標 |
| GET | `/api/v1/metrics/overview` | 管理員 | 概述 |

### 門戶模板

門戶模板、啟動配置和快取管理。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| GET | `/api/v1/portal/active` | 管理員 | 活動模板 |
| PUT | `/api/v1/portal/active` | 管理員 | 更新活動模板 |
| DELETE | `/api/v1/portal/cache` | 管理員 | 清除入口網站快取 |
| GET | `/api/v1/portal/cache` | 管理員 | 快取狀態 |
| PUT | `/api/v1/portal/cache/dev-mode` | 管理員 | 切換快取開發模式 |
| POST | `/api/v1/portal/cache/rebuild` | 管理員 | 重建入口網站緩存 |
| GET | `/api/v1/portal/lang/builtin` | 管理員 | 內建語言包 |
| POST | `/api/v1/portal/reset` | 管理員 | 恢復預設模板 |
| GET | `/api/v1/portal/templates` | 管理員 | 清單 |
| POST | `/api/v1/portal/templates` | 管理員 | 創建 |
| DELETE | `/api/v1/portal/templates/:id` | 管理員 | 刪除 |
| GET | `/api/v1/portal/templates/:id` | 管理員 | 詳情 |
| PUT | `/api/v1/portal/templates/:id` | 管理員 | 更新 |
| POST | `/api/v1/portal/templates/:id/activate` | 管理員 | 啟動模板 |

> `/portal/templates` CRUD 是**進階 API**：入口網站 UI 主要使用 `PUT /api/v1/portal/active`
> （一次性全站點配置）；使用此群組管理/切換多個範本。兩者都是支持的，並不衝突。

### 代理安裝

節點代理安裝腳本和下載（公共）。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| POST | `/api/v1/agent/bootstrap` | 公有 | 代理首次啟動註冊（代理使用引導令牌進行報告；公共） |
| GET | `/api/v1/agent/releases` | 管理員 | 可用的代理版本（用於升級頁面） |
| GET | `/api/v1/agent/version` | 管理員 | 面板上的目前代理版本 |
| GET | `/api/v1/agent/download` | 公有 | 下載代理 |
| GET | `/api/v1/agent/install` | 公有 | 取得安裝腳本 |
| GET | `/api/v1/agent/origin` | 公有 | 面板起始位址（公開） |

### 收費回調入口

第三方支付平台回檔（公開、簽名驗證）。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| POST | `/api/v1/charge/callback/:site_id` | 公有 | 處理費用回調 |

### 入站（含節點）

用於規劃節點選擇的入站清單。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| GET | `/api/v1/inbounds-with-server` | 已登入 | 所有入站（包括其節點） |

### 公共配置

無需登入的Portal運行時配置（維護衛士+公共限速）。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| GET | `/api/v1/public/articles` | 公有 | 公開文章列表 |
| GET | `/api/v1/public/articles/:slug` | 公有 | 公開文章詳情 |
| GET | `/api/v1/public/client-configs` | 公有 | 客戶端下載配置（公開） |
| GET | `/api/v1/public/portal-config` | 公有 | 入口網站執行階段配置（網站名稱、主題、語言、合規性文件；公共） |
| GET | `/api/v1/public/settings` | 公有 | 公共設定（例如，registration_enabled；公共） |
| GET | `/api/v1/public/subscription-endpoints` | 公有 | 啟用的訂閱端點清單（公用） |

### 客戶端身份驗證和業務（管理引擎上的設定相同）

客戶端端點與入口網站相同。他們使用 **客戶端令牌** (`Authorization: Bearer <client_token>`)，而不是管理令牌；詳情請參閱`portal-api.en.md`。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| GET | `/api/v1/client-auth/articles` | 用戶端（已登入） | 列出文章 |
| GET | `/api/v1/client-auth/articles/:slug` | 用戶端（已登入） | 取得文章 |
| GET | `/api/v1/client-auth/balance` | 用戶端（已登入） | 獲得平衡 |
| GET | `/api/v1/client-auth/balance/transactions` | 用戶端（已登入） | 餘額交易 |
| POST | `/api/v1/client-auth/change-password` | 用戶端（已登入） | 更改密碼 |
| POST | `/api/v1/client-auth/charge/create` | 用戶端（已登入） | 建立儲值訂單 |
| GET | `/api/v1/client-auth/charge/orders` | 用戶端（已登入） | 列出儲值訂單 |
| GET | `/api/v1/client-auth/charge/orders/:id` | 用戶端（已登入） | 取得儲值訂單詳情 |
| POST | `/api/v1/client-auth/charge/redeem-card` | 用戶端（已登入） | 兌換卡 |
| GET | `/api/v1/client-auth/charge/sites` | 用戶端（已登入） | 列出儲值站點 |
| GET | `/api/v1/client-auth/charge/status/:trade_no` | 用戶端（已登入） | 查詢儲值狀態 |
| GET | `/api/v1/client-auth/checktime` | 用戶端（已登入） | 取得伺服器時間（時鐘同步） |
| GET | `/api/v1/client-auth/connection-info` | 用戶端（已登入） | 取得計劃連線參數 |
| GET | `/api/v1/client-auth/gifts` | 用戶端（已登入） | 列出禮物 |
| POST | `/api/v1/client-auth/gifts/:id/claim` | 用戶端（已登入） | 索取禮物 |
| GET | `/api/v1/client-auth/invoices` | 用戶端（已登入） | 訂單清單 |
| GET | `/api/v1/client-auth/invoices/:id` | 用戶端（已登入） | 訂單詳情 |
| POST | `/api/v1/client-auth/login` | 客戶（公眾） | 登入 |
| POST | `/api/v1/client-auth/logout` | 用戶端（已登入） | 退出 |
| POST | `/api/v1/client-auth/logs/package` | 用戶端（已登入） | 建立日誌包 |
| GET | `/api/v1/client-auth/logs/packages` | 用戶端（已登入） | 列出日誌包 |
| DELETE | `/api/v1/client-auth/logs/packages/:id` | 用戶端（已登入） | 刪除日誌包 |
| GET | `/api/v1/client-auth/logs/packages/:id` | 用戶端（已登入） | 取得日誌包 |
| GET | `/api/v1/client-auth/logs/packages/:id/download` | 用戶端（已登入） | 下載日誌包 |
| GET | `/api/v1/client-auth/logs/types` | 用戶端（已登入） | 日誌包健康檢查 |
| GET | `/api/v1/client-auth/me` | 用戶端（已登入） | 取得目前登入用戶 |
| GET | `/api/v1/client-auth/nodes` | 用戶端（已登入） | 我的節點列表 |
| GET | `/api/v1/client-auth/nodes/:id/config` | 用戶端（已登入） | 節點配置 |
| GET | `/api/v1/client-auth/nodes/:id/link` | 用戶端（已登入） | 節點分享連結 |
| GET | `/api/v1/client-auth/nodes/:id/qrcode` | 用戶端（已登入） | 節點二維碼 |
| GET | `/api/v1/client-auth/oauth/:provider` | 客戶（公眾） | 授權 |
| GET | `/api/v1/client-auth/oauth/:provider/callback` | 客戶（公眾） | 回調 |
| GET | `/api/v1/client-auth/oauth/providers` | 客戶（公眾） | 提供者狀態 |
| GET | `/api/v1/client-auth/orders` | 用戶端（已登入） | 訂單清單 |
| POST | `/api/v1/client-auth/orders` | 用戶端（已登入） | 建立訂單 |
| GET | `/api/v1/client-auth/orders/:id` | 用戶端（已登入） | 訂單詳情 |
| GET | `/api/v1/client-auth/plans` | 用戶端（已登入） | 計劃清單 |
| GET | `/api/v1/client-auth/plans/:id` | 用戶端（已登入） | 計劃詳情 |
| GET | `/api/v1/client-auth/referral/code` | 用戶端（已登入） | 取得推薦驗證碼 |
| GET | `/api/v1/client-auth/referral/records` | 用戶端（已登入） | 列出推薦記錄 |
| GET | `/api/v1/client-auth/referral/stats` | 用戶端（已登入） | 取得推薦統計數據 |
| POST | `/api/v1/client-auth/referral/withdraw` | 用戶端（已登入） | 請求提款 |
| GET | `/api/v1/client-auth/referral/withdrawals` | 用戶端（已登入） | 列出推薦提款 |
| POST | `/api/v1/client-auth/refresh` | 客戶（公眾） | 重新整理 |
| POST | `/api/v1/client-auth/register` | 客戶（公眾） | 註冊 |
| POST | `/api/v1/client-auth/reset-subscription` | 用戶端（已登入） | 重置訂閱 |
| POST | `/api/v1/client-auth/rotate-token` | 用戶端（已登入） | 輪換訂閱令牌 |
| POST | `/api/v1/client-auth/self-unban` | 用戶端（已登入） | 自助解禁 |
| PUT | `/api/v1/client-auth/settings` | 用戶端（已登入） | 更新設定 |
| POST | `/api/v1/client-auth/settings/auto-add-ip` | 用戶端（已登入） | 自動添加IP |
| GET | `/api/v1/client-auth/settings/whitelist` | 用戶端（已登入） | 列出 IP 允許列表 |
| DELETE | `/api/v1/client-auth/settings/whitelist/:id` | 用戶端（已登入） | 從 IP 允許清單中刪除 |
| GET | `/api/v1/client-auth/shared-accounts` | 用戶端（已登入） | 列出共享帳戶 |
| GET | `/api/v1/client-auth/shared-accounts/:id` | 用戶端（已登入） | 取得共享帳戶 |
| POST | `/api/v1/client-auth/shared-accounts/:id/apply` | 用戶端（已登入） | 請求共享帳戶 |
| POST | `/api/v1/client-auth/shared-accounts/:id/repair` | 用戶端（已登入） | 維修請求 |
| POST | `/api/v1/client-auth/ticket` | 用戶端（已登入） | 發行一次性短期票 |
| GET | `/api/v1/client-auth/tickets` | 用戶端（已登入） | 列出門票 |
| POST | `/api/v1/client-auth/tickets` | 用戶端（已登入） | 創建票證 |
| GET | `/api/v1/client-auth/tickets/:id` | 用戶端（已登入） | 取得門票 |
| POST | `/api/v1/client-auth/tickets/:id/close` | 用戶端（已登入） | 關閉票證 |
| POST | `/api/v1/client-auth/tickets/:id/reply` | 用戶端（已登入） | 回覆票 |
| GET | `/api/v1/client-auth/traffic-history` | 用戶端（已登入） | 交通歷史記錄 |
| GET | `/api/v1/client-auth/traffic-packs` | 用戶端（已登入） | 流量包列表 |
| POST | `/api/v1/client-auth/traffic-packs/buy` | 用戶端（已登入） | 購買流量包 |

### 訂閱交付

客戶端取得訂閱的公共入口點；請參閱`subscription.en.md`。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| GET | `/sub/:token` | 公有 | 服務訂閱 |

### 運行時配置工作流程

設定快照/試運行/應用/回滾（`apply`、`rollback` 和 `disable-proxy` 需要管理員）。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| POST | `/api/v1/runtime/apply` | 管理員 | 申請 |
| GET | `/api/v1/runtime/apply-jobs` | 已登入 | 列出職位 |
| GET | `/api/v1/runtime/apply-jobs/:id` | 已登入 | 找工作 |
| POST | `/api/v1/runtime/apply-jobs/:id/cancel` | 管理員 | 取消作業 |
| POST | `/api/v1/runtime/apply-jobs/:id/retry` | 管理員 | 重試作業 |
| POST | `/api/v1/runtime/disable-proxy` | 管理員 | 停用代理 |
| POST | `/api/v1/runtime/dry-run` | 已登入 | 試運行執行 |
| GET | `/api/v1/runtime/nodes` | 已登入 | 列出節點 |
| GET | `/api/v1/runtime/overview` | 已登入 | 概述 |
| POST | `/api/v1/runtime/rollback` | 管理員 | 復原 |
| GET | `/api/v1/runtime/snapshots` | 已登入 | 列出快照 |
| POST | `/api/v1/runtime/snapshots` | 已登入 | 建立快照 |
| DELETE | `/api/v1/runtime/snapshots/:id` | 管理員 | 刪除快照 |
| GET | `/api/v1/runtime/snapshots/:id` | 已登入 | 取得快照 |
| POST | `/api/v1/runtime/verify` | 已登入 | 驗證 |

### 服务访问策略（已停用）

> `/services/:id/access-policies`和`/services/:id/ip-bans`於2026年10月20日退休：
> 沒有管理調用，也沒有連線路徑消耗這些策略。

### 流量配額監控

配额和使用情况的只读监控视图。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| GET | `/api/v1/traffic/quotas` | 已登入 | 配額清單 |
| GET | `/api/v1/traffic/quotas/:id/usage` | 已登入 | 配額使用情況 |
| GET | `/api/v1/traffic/reset-logs` | 已登入 | 配額重置記錄 |
| GET | `/api/v1/traffic/usage-history` | 已登入 | 使用歷史 |

### 健康檢查

活性探針。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| GET | `/health` | 公有 | 活性探針； 200 代表健康 |

### 入口網頁

入口網站SPA頁。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| GET | `/p` | 公有 | Portal HTML（伺服器渲染 + Vue 運行時） |

### ACME挑戰

HTTP-01挑戰回檔。

| 方法 | 路徑 | 授權 | 描述 |
|---|---|---|---|
| GET | `/.well-known/acme-challenge/:token` | 公有 | ACME HTTP-01 質詢回呼（簽發時驗證網域） |
