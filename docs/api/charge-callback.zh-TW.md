<!-- source: charge-callback.md | locale: zh-TW | updated: 2026-10-23 -->
# 支付回撥 API（Charge Callback）

> 語言： [简体中文](charge-callback.md) · [English](charge-callback.en.md) · 繁體中文 · [Русский](charge-callback.ru.md) · [日本語](charge-callback.ja.md) · [한국어](charge-callback.ko.md) · [Deutsch](charge-callback.de.md) · [Français](charge-callback.fr.md) · [Español](charge-callback.es.md) · [Português](charge-callback.pt.md) · [Türkçe](charge-callback.tr.md) · [Bahasa Indonesia](charge-callback.id.md) · [ไทย](charge-callback.th.md) · [Tiếng Việt](charge-callback.vi.md) · [العربية](charge-callback.ar.md) · [فارسی](charge-callback.fa.md)

面向**支付平臺 / 站點對接方**：使用者在掛載站點完成支付後，支付平臺伺服器向面板回撥通知。

## 1. 端點

```
POST /api/v1/charge/callback/{site_id}
Content-Type: application/json
```

`site_id` 是管理端"掛載站點"的 ID（整數）。回撥是 server-to-server 請求，無需 Bearer 認證，改用 **HMAC 簽名** + 可選 IP 白名單。

## 2. 請求體

```json
{
  "trade_no":       "CF20261010120000abcd1234",
  "site_trade_no":  "PAY平臺訂單號",
  "status":         "success",
  "amount":         990,
  "currency":       "CNY",
  "timestamp":      1791640000,
  "nonce":          "隨機唯一字串",
  "paid_currency":  "CNY",
  "paid_amount":    990,
  "method":         "alipay",
  "exchange_rate":  1.0,
  "paid_at":        "2026-10-10 12:00:01",
  "sign":           "hex(簽名)",
  "extra":          {}
}
```

| 欄位 | 說明 |
|---|---|
| `trade_no` | 面板側交易號（下單時由面板簽發，`CF` 字首） |
| `site_trade_no` | 支付平臺側訂單號（參與簽名） |
| `status` | 業務狀態，**不參與簽名** |
| `amount` / `currency` | 訂單金額（整數分）與貨幣（均參與簽名） |
| `timestamp` | Unix 秒，超窗即拒（參與簽名） |
| `nonce` | 防重放：原子 INSERT，同一 nonce 只處理一次（參與簽名） |
| `paid_*` / `method` / `exchange_rate` / `paid_at` | 實付資訊，**不參與簽名**（由 `body_hash` 約束完整性） |
| `sign` | HMAC-SHA256 簽名，hex 編碼（參與 `body_hash` 的"去掉 sign 再算"口徑） |
| `extra` | 擴充套件欄位 |

## 3. 簽名演算法（v3.2 / `version: "v1"`）

1. 構造**不含 `sign` 的原始 body**，計算 `body_hash = SHA256(原始 body 位元組)`（十六進位制小寫）。
   > 面板同時相容"對含 sign 的完整 body 求 hash"的舊口徑，但新對接請按文件口徑實現。
2. 組裝簽名載荷（**鍵按字典序**，Go `json.Marshal` 語義）：

```json
{
  "amount": 990,
  "body_hash": "<步驟1的hash>",
  "currency": "CNY",
  "nonce": "<nonce>",
  "site_trade_no": "<site_trade_no>",
  "timestamp": 1791640000,
  "trade_no": "<trade_no>",
  "version": "v1"
}
```

3. `sign = hex( HMAC_SHA256( key=回撥金鑰, msg=步驟2的JSON位元組) )`。
   > 金鑰即管理端"掛載站點"的回撥金鑰（rotate 後舊金鑰仍保留一版用於過渡驗證）。

```python
import hashlib, hmac, json

def sign_callback(body: dict, secret: str) -> str:
    # 1) 先算 body_hash：body 此時不含 sign
    body_hash = hashlib.sha256(json.dumps(body, separators=(",", ":"), ensure_ascii=False).encode()).hexdigest()
    # 2) 簽名載荷（鍵序無關：json.dumps(sort_keys=True)）
    payload = {
        "amount": body["amount"], "body_hash": body_hash,
        "currency": body["currency"], "nonce": body["nonce"],
        "site_trade_no": body["site_trade_no"], "timestamp": body["timestamp"],
        "trade_no": body["trade_no"], "version": "v1",
    }
    msg = json.dumps(payload, separators=(",", ":"), sort_keys=True).encode()
    return hmac.new(secret.encode(), msg, hashlib.sha256).hexdigest()
```

## 4. 面板側校驗鏈（決定了你會收到什麼錯誤）

1. site 存在且已驗證域名 → 404 / 403
2. IP 白名單（站點配置了 `callback_whitelist` 時**硬阻斷**非白名單 IP；未配置則跳過，僅靠簽名）
3. `timestamp` 時間窗 + `nonce` 防重放（原子 INSERT）
4. HMAC 驗籤（相容 active + legacy 兩版金鑰）
5. 訂單存在且屬於該站點 → 冪等（已支付直接成功返回）→ **金額與幣種鎖定校驗** → 事務內入賬（餘額賬本 + 訂單狀態）

處理是**冪等**的：同一 `trade_no` 重複回撥不會重複入賬。簽名/頻率異常會記錄到"回撥事件"，管理端可查、可手動重試。

## 5. 響應

- 成功：2xx（面板入賬完成後應答）
- 失敗：4xx/5xx + `{ "code": "...", "message": "..." }`；典型 `INVALID_SIGNATURE`、`IP_BLOCKED`、`ORDER_NOT_FOUND`、金額不符等。

> 每次回撥都帶 `X-Request-ID`（無則面板生成），日誌與事件全鏈路可追蹤；排查時把這個 ID 給管理員即可。

## 6. 測試與排查

- 管理端可拉取每站點的**接入文件**：`GET /api/v1/charge-sites/{id}/integration-doc`（含該站點實際配置的回撥 URL 示例）。
- 回撥事件：`GET /api/v1/charge-callback-events`；重試：`GET /api/v1/charge-callback-retries`、`POST /api/v1/charge-callback-retries/{id}/retry`。
- 輪換金鑰：`POST /api/v1/charge-sites/{id}/rotate-secret`（舊金鑰保留一版過渡）。
