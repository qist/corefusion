<!-- source: subscription.md | locale: zh-TW | updated: 2026-11-12 -->
# 訂閱API（公開）

> 語言： [简体中文](subscription.md) · [English](subscription.en.md) · 繁體中文 · [Русский](subscription.ru.md) · [日本語](subscription.ja.md) · [한국어](subscription.ko.md) · [Deutsch](subscription.de.md) · [Français](subscription.fr.md) · [Español](subscription.es.md) · [Português](subscription.pt.md) · [Türkçe](subscription.tr.md) · [Bahasa Indonesia](subscription.id.md) · [ไทย](subscription.th.md) · [Tiếng Việt](subscription.vi.md) · [العربية](subscription.ar.md) · [فارسی](subscription.fa.md)

取得使用者的訂閱節點清單。 **這是最終用戶最常用的端點**；不需要標頭 — 存取由 URL 中的令牌授予。

## 1. 端點

| 部署 | URL |
|---|---|
| 主面板 | `GET https://<panel-domain>/sub/<token>` |
| 專用訂閱端口 | `GET https://<sub-domain>:<port>/<token>`（無 `/sub` 字首） |

`<token>` 是訂閱的唯一存取令牌，由管理端產生（在入口網站中的「我的訂閱」下可見）。

## 2. 查詢參數

| 參數 | 描述 |
|---|---|
| `format` | 輸出格式；覆蓋訂閱預設值。見下文 |
| `config` | 配置範本名稱（在管理 UI 中的「訂閱格式」下定義）。必須使用與該請求的`format`相同的格式，否則將被忽略 |
| `platform` | 依平台名稱比對組態範本（省略`config`時使用）；格式也必須符合 |
| `providers` | 僅 `mihomo_yaml`：`1` / `true` 啟用**提供者模式**（發出 `proxy-providers` 並透過 `use` 引用它們）；預設值或任何其他值使節點內聯在 `proxies` 下。參見§6 |

### 支援的 `format` 值

| 價值 | 輸出 | 客戶 |
|---|---|---|
| `base64_uri`（預設） | 換行加入的共享鏈接，整體進行base64編碼 | V2RayN、Shadowrocket、Surge 等通用客戶端 |
| `raw_uri` | 換行符連接的共享鏈接，純文本 | 需要簡單連結/調試的客戶端 |
| `mihomo_yaml` | 完整的 Clash/Mihomo YAML 配置（附代理組、規則） | Clash.Meta / mihomo / Clash Verge Rev |
| `mihomo_proxies_yaml` | YAML 僅具有 `proxies:` 節點列表 | mihomo 代理提供者來源（含計畫刷新） |
| `singbox_json` | sing-box JSON 完整配置 | sing-box/SFI/FLClash（sing-box核心） |
| `xray_json` | Xray JSON 完整配置 | Xray-核心 |

> 協定/格式相容性適用：例如，`anytls` 僅出現在 sing-box/Mihomo 格式中； **xray 服務的 REALITY 節點不包含在 sing-box 訂閱中**（ZXQQ00320Z 不支援sing-box#4520），但仍出現在 Xray 訂閱中，而 **REALITY 節點提供服務sing-box 的作品適用於 Xray 和 sing-box 訂閱**。客戶端無法辨識的節點會自動刪除，而不是讓整個訂閱失敗。

## 3. 響應標頭

| 標頭 | 範例 | 描述 |
|---|---|---|
| `Subscription-Userinfo` | `upload=1024; download=2048; total=1073741824; expire=1760000000` | 使用情況和過期時間（位元組/Unix 秒）。 **也透過403發送**，以便客戶可以顯示剩餘的津貼 |
| `Profile-Update-Interval` | `6` | 建議的刷新間隔（小時） |
| `Profile-Title` | `CoreFusion` | 設定檔名稱（由 Clash Verge Rev / V2RayN 等顯示） |
| `Profile-Web-Page-Url` | `https://portal.example.com` | Portal URL（客戶端中的「開啟網頁」；當入口網站有自己的連接埠時的根路徑，否則為/p） |
| `Content-Type` | 每種格式：`text/plain` / `text/yaml` / `application/json` |  |
| `Content-Disposition` | `attachment; filename="subscription.txt"` |  |

## 4. 狀態碼

| HTTP | 程式碼 | 意義 |
|---|---|---|
| 200 | — | 正常（body為`format`中渲染的訂閱） |
| 400 | `MISSING_TOKEN` | URL中沒有令牌 |
| 403 | （純文字） | 訂閱暫停/過期/無效。該訊息透過 `Accept-Language` 本地化 (zh/en/ja/ko)，並且是單行原因 |
| 404 | `SUBSCRIPTION_NOT_FOUND` | 令牌不存在 |

> 撤銷的訂閱被視為不存在 (404)。

## 5. 範例

```bash
# 1) Fetch with the default format
curl -L "https://panel.example.com/sub/abcd1234efgh5678"

# 2) Request Mihomo YAML with the "clash-global" config template
curl -L "https://panel.example.com/sub/abcd1234efgh5678?format=mihomo_yaml&config=clash-global"

# 3) Node list only (for a mihomo proxy-provider)
curl -L "https://panel.example.com/sub/abcd1234efgh5678?format=mihomo_proxies_yaml"

# 4) Check usage (headers only)
curl -sI "https://panel.example.com/sub/abcd1234efgh5678" | grep -i subscription-userinfo

# 5) Clash dynamic subscription: config template + Provider mode
curl -L "https://sub.example.com/abcd1234efgh5678?format=clash&config=out&providers=1"

# 6) sing-box / Xray with their own config templates (rules come from the template)
curl -L "https://sub.example.com/abcd1234efgh5678?format=singbox_json&config=sing-box-home"
curl -L "https://sub.example.com/abcd1234efgh5678?format=xray_json&config=xray-home"
```

Clash/Mihomo動態訂閱（定期刷新）：

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

## 6. 提供者模式和多個訂閱域

### 6.1 `providers=1` 發出什麼

對於 `format=mihomo_yaml&providers=1`（或 `format=clash&providers=1`）YAML：

- 不再在頂層內聯節點 - 它發出 `proxy-providers:`；
- 透過 `use: [<provider-key>...]` 代理群組參考節點來源；
- 任何以共享訂閱格式 (`mihomo_configs.config_json`) 編寫的 `proxy-providers` 都會被**忽略**（共享配置服務於多個計劃；硬編碼的提供者會跨計劃洩漏）。群組→來源對應位於「計畫管理→群組→來源對應」下。

提供者種類：

| 種類 | 鑰匙 | 註解 |
|---|---|---|
| 內部 | 清理後的節點群組名稱 | 按計劃節點組產生；僅包含該組的節點 |
| 保留 | `subscription-nodes` | 計畫的所有節點；其來源 URL **省略** `provider` 參數 |
| 外部 | 計劃中配置的名稱 | 外部來源（`http` YAML 或 `file`） |

每個提供者的常用欄位：

- `url` — 以 `format=mihomo_proxies_yaml` 指向此訂閱，從主請求繼承 `config` / `platform`；內部提供者附加 `provider=<key>`；
- `path` — **始終源自「內部提供者預設值 → 快取路徑範本」**（預設值 `./providers/{key}.yaml`、`{key}` 被取代）。外部提供者也無法設定自己的路徑；
- `health-check` — `enable / url / interval`；內部提供者繼承預設值，外部提供者在計畫上配置它；
- `interval` — 刷新間隔（以秒為單位）。

### 6.2 哪個網域進入提供者URL（多網域）

「訂閱中心→訂閱端點」可能會儲存**多個**記錄（網域名稱+路徑+啟用）。提供者 `url` 依下列順序選擇域：

1. **此請求實際使用的域**：`X-Forwarded-Host`的第一個值（連接埠剝離），否則為`Host`；它必須與**啟用**端點相符；
2. 如果沒有符合的內容，則回退到具有**最低 ID** 的已啟用端點；
3. 此路徑來自匹配的端點（`path`，預設 `/`），產生 `https://<domain><path>/<token>?format=mihomo_proxies_yaml...`。

後果：

- 透過新網域取得的用戶端會在提供者內取得該網域 - **無需變更面板**；
- 清單中仍啟用的舊網域不再是硬編碼；
- 如果用戶端快取了舊的提供者 URL，則更新訂閱（或清除提供者快取/重新啟動用戶端）會將其切換。

反向代理必須轉發`X-Forwarded-Proto`和`X-Forwarded-Host`；否則使用回退端點。此面板產生的 Caddy / Nginx 網站範本已包含兩個標頭 (`header_up X-Forwarded-Host {host}` / `proxy_set_header X-Forwarded-Host $host;`)；如果您手動維護代理配置，請手動新增它們。

## 7. 註解

- **格式和配置緊密耦合**：`config`/`platform`選擇的模板必須與該要求的`format`匹配；不匹配的模板將被忽略——這可以防止將 Mihomo 規則注入 sing-box/Xray ZXQ0091。
- **`providers=1` 僅影響 Mihomo YAML**：sing-box / Xray / base64 / raw 忽略它（節點保持內聯）。
- 
- **存取日誌**：每次抓取都會記錄IP / UA / CDN訊息，在管理介面的「訂閱存取日誌」下可見。
- 如果專用訂閱埠上沒有設定網域，則使用要求的 `X-Forwarded-Proto` / `X-Forwarded-Host`；請確保您的反向代理轉送兩個標頭。
- 在多域部署中，使端點清單與用戶端實際使用的網域保持一致；請參閱第 6.2 節。
