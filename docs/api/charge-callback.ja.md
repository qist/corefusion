<!-- source: charge-callback.md | locale: ja | updated: 2026-10-23 -->
# チャージコールバック API

> 言語： [简体中文](charge-callback.md) · [English](charge-callback.en.md) · [繁體中文](charge-callback.zh-TW.md) · [Русский](charge-callback.ru.md) · 日本語 · [한국어](charge-callback.ko.md) · [Deutsch](charge-callback.de.md) · [Français](charge-callback.fr.md) · [Español](charge-callback.es.md) · [Português](charge-callback.pt.md) · [Türkçe](charge-callback.tr.md) · [Bahasa Indonesia](charge-callback.id.md) · [ไทย](charge-callback.th.md) · [Tiếng Việt](charge-callback.vi.md) · [العربية](charge-callback.ar.md) · [فارسی](charge-callback.fa.md)

**支払いプラットフォーム/サイト インテグレータ**の場合: ユーザーがマウントされたサイトで支払いを行った後、支払いプラットフォームのサーバーがパネルに通知します。

## 1. エンドポイント

```
POST /api/v1/charge/callback/{site_id}
Content-Type: application/json
```

`site_id` は、管理者側の「マウントされたサイト」の ID (整数) です。コールバックはサーバー間で行われ、**HMAC 署名** に加えて、ベアラー認証の代わりにオプションの IP 許可リストを使用します。

## 2. リクエストボディ

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

| フィールド | 説明 |
|---|---|
| `trade_no` | パネル側トランザクション番号 (注文作成時に発行、`CF` プレフィックス) |
| `site_trade_no` | 決済プラットフォーム注文番号（署名済み） |
| `status` | ビジネスステータス、**未署名** |
| `amount` / `currency` | 注文金額 (セントの整数) と通貨 (両方とも符号付き) |
| `timestamp` | Unix 秒。窓の外で拒否されました（署名済み） |
| `nonce` | リプレイ保護: アトミック INSERT、各 nonce が 1 回処理 (署名済み) |
| `paid_*` / `method` / `exchange_rate` / `paid_at` | 実際の支払い情報、**署名なし** (`body_hash` で完全性が保証されています) |
| `sign` | HMAC-SHA256 署名、16 進エンコード (`sign` なしで本体に対して計算) |
| `extra` | 拡張フィールド |

## 3. 署名アルゴリズム (v3.2 / `version: "v1"`)

1. **`sign` を含まない生の本体**を構築し、`body_hash = SHA256(raw body bytes)` (小文字の 16 進数) を計算します。
   > パネルは、`sign` を含む本文全体をハッシュする従来のスキームも受け入れます。新しい統合は文書化されたスキームに従う必要があります。
2. 署名ペイロードを構築します (**辞書順のキー**、Go `json.Marshal` セマンティクス):

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
   > シークレットは、管理 UI 内のマウントされたサイトのコールバック シークレットです (ローテーション後、以前のシークレットは 1 つの移行ウィンドウの間保持されます)。

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

## 4. パネル側の検証チェーン (発生するエラーを特定します)

1. サイトが存在し、そのドメインが認証されている → 404 / 403
2. IP許可リスト(`callback_whitelist`が構成されている場合、リストにないIPは**ハードブロック**されます。それ以外の場合はスキップされ、署名のみがチェックされます)
3. `timestamp` ウィンドウ + `nonce` リプレイ保護 (アトミック INSERT)
4. HMAC 検証 (アクティブ シークレットとレガシー シークレットの両方を受け入れます)
5. 注文が存在し、サイトに属している → 冪等性 (支払い済みの返品が成功) → **金額と通貨ロックの検証** → トランザクション転記 (残高元帳 + 注文ステータス)

処理は **冪等** です。同じ `trade_no` で繰り返されるコールバックは 2 回ポストされません。署名/レートの異常は「コールバック イベント」として記録され、管理者はこれを検査して手動で再試行できます。

## 5. 応答

- 成功: 2xx (パネルが支払いを転記した後)
- 失敗: 4xx/5xx + `{ "code": "...", "message": "..." }`;典型的なコードには、`INVALID_SIGNATURE`、`IP_BLOCKED`、`ORDER_NOT_FOUND`、金額の不一致などが含まれます。

> すべてのコールバックには `X-Request-ID` (存在しない場合はパネルによって生成される) が含まれます。ログとイベントはエンドツーエンドで追跡可能です。トラブルシューティングの際には、この ID を管理者に伝えてください。

## 6. テストとトラブルシューティング

- 管理者は、各サイトの **統合ドキュメント**: `GET /api/v1/charge-sites/{id}/integration-doc` (サイトの実際のコールバック URL の例を含む) を取得できます。
- コールバック イベント: `GET /api/v1/charge-callback-events`;再試行: `GET /api/v1/charge-callback-retries`、`POST /api/v1/charge-callback-retries/{id}/retry`。
- シークレット ローテーション: `POST /api/v1/charge-sites/{id}/rotate-secret` (以前のシークレットは 1 つの遷移ウィンドウの間保持されます)。
