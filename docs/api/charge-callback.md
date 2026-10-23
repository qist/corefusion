# 支付回调 API（Charge Callback）

> 语言： 简体中文 · [English](charge-callback.en.md) · [繁體中文](charge-callback.zh-TW.md) · [Русский](charge-callback.ru.md) · [日本語](charge-callback.ja.md) · [한국어](charge-callback.ko.md) · [Deutsch](charge-callback.de.md) · [Français](charge-callback.fr.md) · [Español](charge-callback.es.md) · [Português](charge-callback.pt.md) · [Türkçe](charge-callback.tr.md) · [Bahasa Indonesia](charge-callback.id.md) · [ไทย](charge-callback.th.md) · [Tiếng Việt](charge-callback.vi.md) · [العربية](charge-callback.ar.md) · [فارسی](charge-callback.fa.md)

面向**支付平台 / 站点对接方**：用户在挂载站点完成支付后，支付平台服务器向面板回调通知。

## 1. 端点

```
POST /api/v1/charge/callback/{site_id}
Content-Type: application/json
```

`site_id` 是管理端"挂载站点"的 ID（整数）。回调是 server-to-server 请求，无需 Bearer 认证，改用 **HMAC 签名** + 可选 IP 白名单。

## 2. 请求体

```json
{
  "trade_no":       "CF20261010120000abcd1234",
  "site_trade_no":  "PAY平台订单号",
  "status":         "success",
  "amount":         990,
  "currency":       "CNY",
  "timestamp":      1791640000,
  "nonce":          "随机唯一字符串",
  "paid_currency":  "CNY",
  "paid_amount":    990,
  "method":         "alipay",
  "exchange_rate":  1.0,
  "paid_at":        "2026-10-10 12:00:01",
  "sign":           "hex(签名)",
  "extra":          {}
}
```

| 字段 | 说明 |
|---|---|
| `trade_no` | 面板侧交易号（下单时由面板签发，`CF` 前缀） |
| `site_trade_no` | 支付平台侧订单号（参与签名） |
| `status` | 业务状态，**不参与签名** |
| `amount` / `currency` | 订单金额（整数分）与货币（均参与签名） |
| `timestamp` | Unix 秒，超窗即拒（参与签名） |
| `nonce` | 防重放：原子 INSERT，同一 nonce 只处理一次（参与签名） |
| `paid_*` / `method` / `exchange_rate` / `paid_at` | 实付信息，**不参与签名**（由 `body_hash` 约束完整性） |
| `sign` | HMAC-SHA256 签名，hex 编码（参与 `body_hash` 的"去掉 sign 再算"口径） |
| `extra` | 扩展字段 |

## 3. 签名算法（v3.2 / `version: "v1"`）

1. 构造**不含 `sign` 的原始 body**，计算 `body_hash = SHA256(原始 body 字节)`（十六进制小写）。
   > 面板同时兼容"对含 sign 的完整 body 求 hash"的旧口径，但新对接请按文档口径实现。
2. 组装签名载荷（**键按字典序**，Go `json.Marshal` 语义）：

```json
{
  "amount": 990,
  "body_hash": "<步骤1的hash>",
  "currency": "CNY",
  "nonce": "<nonce>",
  "site_trade_no": "<site_trade_no>",
  "timestamp": 1791640000,
  "trade_no": "<trade_no>",
  "version": "v1"
}
```

3. `sign = hex( HMAC_SHA256( key=回调密钥, msg=步骤2的JSON字节) )`。
   > 密钥即管理端"挂载站点"的回调密钥（rotate 后旧密钥仍保留一版用于过渡验证）。

```python
import hashlib, hmac, json

def sign_callback(body: dict, secret: str) -> str:
    # 1) 先算 body_hash：body 此时不含 sign
    body_hash = hashlib.sha256(json.dumps(body, separators=(",", ":"), ensure_ascii=False).encode()).hexdigest()
    # 2) 签名载荷（键序无关：json.dumps(sort_keys=True)）
    payload = {
        "amount": body["amount"], "body_hash": body_hash,
        "currency": body["currency"], "nonce": body["nonce"],
        "site_trade_no": body["site_trade_no"], "timestamp": body["timestamp"],
        "trade_no": body["trade_no"], "version": "v1",
    }
    msg = json.dumps(payload, separators=(",", ":"), sort_keys=True).encode()
    return hmac.new(secret.encode(), msg, hashlib.sha256).hexdigest()
```

## 4. 面板侧校验链（决定了你会收到什么错误）

1. site 存在且已验证域名 → 404 / 403
2. IP 白名单（站点配置了 `callback_whitelist` 时**硬阻断**非白名单 IP；未配置则跳过，仅靠签名）
3. `timestamp` 时间窗 + `nonce` 防重放（原子 INSERT）
4. HMAC 验签（兼容 active + legacy 两版密钥）
5. 订单存在且属于该站点 → 幂等（已支付直接成功返回）→ **金额与币种锁定校验** → 事务内入账（余额账本 + 订单状态）

处理是**幂等**的：同一 `trade_no` 重复回调不会重复入账。签名/频率异常会记录到"回调事件"，管理端可查、可手动重试。

## 5. 响应

- 成功：2xx（面板入账完成后应答）
- 失败：4xx/5xx + `{ "code": "...", "message": "..." }`；典型 `INVALID_SIGNATURE`、`IP_BLOCKED`、`ORDER_NOT_FOUND`、金额不符等。

> 每次回调都带 `X-Request-ID`（无则面板生成），日志与事件全链路可追踪；排查时把这个 ID 给管理员即可。

## 6. 测试与排查

- 管理端可拉取每站点的**接入文档**：`GET /api/v1/charge-sites/{id}/integration-doc`（含该站点实际配置的回调 URL 示例）。
- 回调事件：`GET /api/v1/charge-callback-events`；重试：`GET /api/v1/charge-callback-retries`、`POST /api/v1/charge-callback-retries/{id}/retry`。
- 轮换密钥：`POST /api/v1/charge-sites/{id}/rotate-secret`（旧密钥保留一版过渡）。
