<!-- source: subscription.md | locale: zh-TW | updated: 2026-10-23 -->
# 訂閱 API（對外）

> 語言： [简体中文](subscription.md) · [English](subscription.en.md) · 繁體中文 · [Русский](subscription.ru.md) · [日本語](subscription.ja.md) · [한국어](subscription.ko.md) · [Deutsch](subscription.de.md) · [Français](subscription.fr.md) · [Español](subscription.es.md) · [Português](subscription.pt.md) · [Türkçe](subscription.tr.md) · [Bahasa Indonesia](subscription.id.md) · [ไทย](subscription.th.md) · [Tiếng Việt](subscription.vi.md) · [العربية](subscription.ar.md) · [فارسی](subscription.fa.md)

拉取使用者訂閱節點列表。**這是面向終端使用者最常用的介面**，無需任何 Header，憑 URL 中的 token 訪問。

## 1. 端點

| 部署形態 | URL |
|---|---|
| 主面板 | `GET https://<面板域名>/sub/<token>` |
| 訂閱獨立埠 | `GET https://<訂閱域名>:<埠>/<token>`（無 `/sub` 字首） |

`<token>` 是每個訂閱唯一的訪問令牌，由管理端生成（可在門戶"我的訂閱"裡檢視）。

## 2. 查詢引數

| 引數 | 說明 |
|---|---|
| `format` | 輸出格式，覆蓋訂閱預設格式。見下表 |
| `config` | 套用的配置模板名稱（管理端"訂閱格式"裡定義的 config），須與本次 `format` 同格式，否則忽略 |
| `platform` | 按平臺名匹配配置模板（`config` 未指定時生效），同樣要求格式一致 |

### 支援的 format 值

| 值 | 輸出 | 適用客戶端 |
|---|---|---|
| `base64_uri`（預設） | 換行拼接的分享連結，整體 base64 | V2RayN、Shadowrocket、Surge 等通用客戶端 |
| `raw_uri` | 換行拼接的分享連結，明文 | 需要明文連結的客戶端 / 除錯 |
| `mihomo_yaml` | 完整 Clash/Mihomo YAML 配置（含 proxy-groups、rules） | Clash.Meta / mihomo / Clash Verge Rev |
| `mihomo_proxies_yaml` | 僅 `proxies:` 節點列表的 YAML | mihomo proxy-provider 回原始檔（配合動態 provider 定時重新整理） |
| `singbox_json` | sing-box 完整 JSON 配置 | sing-box / SFI / FLClash(sing-box 核心) |
| `xray_json` | Xray 完整 JSON 配置 | Xray-core |

> 協議與格式存在相容矩陣：例如 `anytls` 只進 sing-box/Mihomo 格式；sing-box 服務端的 REALITY 節點不會出現在 Xray/sing-box JSON 訂閱中。客戶端不識別的節點會被自動剔除，不會整份失敗。

## 3. 響應頭

| 響應頭 | 示例 | 說明 |
|---|---|---|
| `Subscription-Userinfo` | `upload=1024; download=2048; total=1073741824; expire=1760000000` | 用量與到期時間（位元組 / Unix 秒）。**錯誤響應（403）時也帶**，供客戶端展示餘量 |
| `Profile-Update-Interval` | `6` | 建議重新整理間隔（小時） |
| `Profile-Title` | `CoreFusion` | 訂閱配置名（Clash Verge Rev / V2RayN 等顯示用） |
| `Profile-Web-Page-Url` | `https://…/portal` | 門戶頁連結（客戶端"開啟網頁"入口） |
| `Content-Type` | 按格式：`text/plain` / `text/yaml` / `application/json` | |
| `Content-Disposition` | `attachment; filename="subscription.txt"` | |

## 4. 狀態碼

| HTTP | code | 含義 |
|---|---|---|
| 200 | — | 正常下發（body 為按 format 渲染的訂閱內容） |
| 400 | `MISSING_TOKEN` | URL 缺 token |
| 403 | （純文字訊息） | 訂閱被暫停 / 已到期 / 未啟用。訊息按 `Accept-Language` 出 zh/en/ja/ko，body 為一行原因說明 |
| 404 | `SUBSCRIPTION_NOT_FOUND` | token 不存在 |

> 吊銷（revoke）的訂閱按不存在處理（404）。

## 5. 示例

```bash
# 1) 用預設格式拉取
curl -L "https://panel.example.com/sub/abcd1234efgh5678"

# 2) 指定 Mihomo YAML，並套用名為 "clash-global" 的配置模板
curl -L "https://panel.example.com/sub/abcd1234efgh5678?format=mihomo_yaml&config=clash-global"

# 3) 只拿節點列表（餵給 mihomo proxy-provider）
curl -L "https://panel.example.com/sub/abcd1234efgh5678?format=mihomo_proxies_yaml"

# 4) 檢視用量（不取 body）
curl -sI "https://panel.example.com/sub/abcd1234efgh5678" | grep -i subscription-userinfo
```

Clash/Mihomo 動態訂閱（節點定時回源重新整理）寫法：

```yaml
proxy-providers:
  subscription-nodes:
    type: http
    url: "https://panel.example.com/sub/abcd1234efgh5678?format=mihomo_proxies_yaml"
    interval: 3600
    health-check:
      enable: true
      url: https://www.gstatic.com/generate_204
      interval: 300

proxy-groups:
  - name: PROXY
    type: select
    use:
      - subscription-nodes
```

## 6. 注意事項

- **格式與配置強繫結**：`config`/`platform` 指定的模板格式必須與本次 `format` 一致，面板會自動忽略不匹配的模板——防止把 Mihomo 行格式規則注入 sing-box/Xray 的 JSON。
- **限流**：訂閱端點有獨立限流（`SubscribeRateLimit`），惡意高頻拉取會被拒。
- **訪問記錄**：每次拉取記錄 IP / UA / CDN 資訊，管理端"訂閱訪問日誌"可查。
- 獨立訂閱埠未配置域名時回退到本請求的 `X-Forwarded-Proto` / `X-Forwarded-Host`；反代請正確透傳這兩個頭。
