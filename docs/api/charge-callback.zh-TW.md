<!-- source: charge-callback.md | locale: zh-TW | updated: 2026-11-16 -->
# 收費回檔 API

> 語言： [简体中文](charge-callback.md) · [English](charge-callback.en.md) · 繁體中文 · [Русский](charge-callback.ru.md) · [日本語](charge-callback.ja.md) · [한국어](charge-callback.ko.md) · [Deutsch](charge-callback.de.md) · [Français](charge-callback.fr.md) · [Español](charge-callback.es.md) · [Português](charge-callback.pt.md) · [Türkçe](charge-callback.tr.md) · [Bahasa Indonesia](charge-callback.id.md) · [ไทย](charge-callback.th.md) · [Tiếng Việt](charge-callback.vi.md) · [العربية](charge-callback.ar.md) · [فارسی](charge-callback.fa.md)

對於**支付平台/網站整合商**：使用者在安裝的網站上付款後，支付平台的伺服器通知面板。

## 1. 端點

```
POST /api/v1/charge/callback/{site_id}
Content-Type: application/json
```

`site_id` 是管理端「已安裝站點」ID（整數）。回調是伺服器到伺服器的，並使用 **HMAC 簽章** 加上可選的 IP 允許清單而不是承載身份驗證。

## 2. 請求正文

```json
{
  "trade_no":       "CF20261010120000abcd1234",
  "site_trade_no":  "payment-platform-order-no",
  "status":         "success",
  "amount":         990,
  "currency":       "CNY",
  "timestamp":      1791640000,
  "nonce":          "random-unique-string",
  "paid_currency":  "CNY",
  "paid_amount":    990,
  "method":         "alipay",
  "exchange_rate":  1.0,
  "paid_at":        "2026-10-10 12:00:01",
  "sign":           "hex(signature)",
  "extra":          {}
}
```

| 領域 | 描述 |
|---|---|
| `trade_no` | 面板端交易號碼（建立訂單時發出，`CF`前綴） |
| `site_trade_no` | 支付平台訂單號碼（簽名） |
| `status` | 業務狀況，**未簽署** |
| `amount` / `currency` | 訂單金額（整數美分）和貨幣（均已簽名） |
| `timestamp` | Unix 秒；在窗外被拒絕（簽名） |
| `nonce` | 重播保護：原子插入，每個隨機數處理一次（簽名） |
| `paid_*` / `method` / `exchange_rate` / `paid_at` | 實際付款信息，**未簽名**（`body_hash` 保證完整性） |
| `sign` | HMAC-SHA256 簽名，十六進位編碼（在沒有 `sign` 的主體上計算） |
| `extra` | 擴充字段 |

## 3. 簽名演算法（v3.2 / `version: "v1"`）

1. 建構**不帶 `sign` 的原始主體**併計算 `body_hash = SHA256(raw body bytes)`（小寫十六進位）。
   > 該小組還接受對包括 `sign` 在內的完整主體進行哈希處理的遺留方案；新的整合應遵循記錄的方案。
2. 建構簽名有效負載（**按字典順序排列的金鑰**，Go `json.Marshal` 語意）：

```json
{
  "amount": 990,
  "body_hash": "<hash from step 1>",
  "currency": "CNY",
  "nonce": "<nonce>",
  "site_trade_no": "<site_trade_no>",
  "timestamp": 1791640000,
  "trade_no": "<trade_no>",
  "version": "v1"
}
```

3. `sign = hex( HMAC_SHA256( key=callback secret, msg=JSON bytes from step 2) )`。
   > 該秘密是管理 UI 中已安裝網站的回調秘密（旋轉後，為一個轉換視窗保留前一個秘密）。

```python
import hashlib, hmac, json

def sign_callback(body: dict, secret: str) -> str:
    # 1) body_hash first: body must not contain sign here
    body_hash = hashlib.sha256(json.dumps(body, separators=(",", ":"), ensure_ascii=False).encode()).hexdigest()
    # 2) signing payload (key order irrelevant: json.dumps(sort_keys=True))
    payload = {
        "amount": body["amount"], "body_hash": body_hash,
        "currency": body["currency"], "nonce": body["nonce"],
        "site_trade_no": body["site_trade_no"], "timestamp": body["timestamp"],
        "trade_no": body["trade_no"], "version": "v1",
    }
    msg = json.dumps(payload, separators=(",", ":"), sort_keys=True).encode()
    return hmac.new(secret.encode(), msg, hashlib.sha256).hexdigest()
```

## 4. 面板端驗證鏈（確定您得到的錯誤）

1. 網站存在且其網域名稱已驗證 → 404 / 403
2. IP 允許清單（如果配置了 `callback_whitelist`，則未列出的 IP 被**硬阻止**；否則跳過，僅檢查簽名）
3. `timestamp` 視窗 + `nonce` 重播保護（原子 INSERT）
4. HMAC 驗證（接受活動金鑰和舊金鑰）
5. 訂單存在且屬於站點→冪等性（已支付返回成功）→**金額和幣種鎖定驗證**→交易過帳（餘額帳本+訂單狀態）

處理是**冪等**：具有相同 `trade_no` 的重複回呼不會發布兩次。簽名/速率異常被記錄為“回調事件”，管理員可以手動檢查和重試。

## 5. 回應

- 成功：HTTP 200、`{"code":0,"data":{"success":true,"message":"ok"}}`（面板過帳付款後）
- 故障：HTTP 400、`{"code":"CALLBACK_FAILED","message":"..."}`；具體原因在`message`（簽章錯誤、IP未列入白名單、訂單未找到、金額/幣種不符、隨機數重播等）
- 路徑中無效的`site_id`返回`INVALID_SITE_ID`；非 JSON 主體回傳 `INVALID_REQUEST`

> 每個回呼都帶有`X-Request-ID`（如果不存在則由面板產生）；日誌和事件是端到端可追溯的。故障排除時將此 ID 提供給您的管理員。

## 6. 測試和故障排除

- 管理員可以取得每個網站的**整合文件**：`GET /api/v1/charge-sites/{id}/integration-doc`（包括網站的實際回呼 URL 範例）。
- 回呼事件：`GET /api/v1/charge-callback-events`；重試：`GET /api/v1/charge-callback-retries`、`POST /api/v1/charge-callback-retries/{id}/retry`。
- 秘密輪換：`POST /api/v1/charge-sites/{id}/rotate-secret`（前一個秘密保留轉換視窗）。
