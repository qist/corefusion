<!-- source: charge-callback.md | locale: ko | updated: 2026-10-23 -->
# 충전 콜백 API

> 언어: [简体中文](charge-callback.md) · [English](charge-callback.en.md) · [繁體中文](charge-callback.zh-TW.md) · [Русский](charge-callback.ru.md) · [日本語](charge-callback.ja.md) · 한국어 · [Deutsch](charge-callback.de.md) · [Français](charge-callback.fr.md) · [Español](charge-callback.es.md) · [Português](charge-callback.pt.md) · [Türkçe](charge-callback.tr.md) · [Bahasa Indonesia](charge-callback.id.md) · [ไทย](charge-callback.th.md) · [Tiếng Việt](charge-callback.vi.md) · [العربية](charge-callback.ar.md) · [فارسی](charge-callback.fa.md)

**결제 플랫폼/사이트 통합업체**의 경우: 사용자가 탑재된 사이트에서 결제한 후 결제 플랫폼의 서버가 패널에 알립니다.

## 1. 엔드포인트

```
POST /api/v1/charge/callback/{site_id}
Content-Type: application/json
```

`site_id`는 관리자 측 "마운트된 사이트" ID(정수)입니다. 콜백은 서버 간이며 Bearer 인증 대신 **HMAC 서명**과 선택적 IP 허용 목록을 사용합니다.

## 2. 요청 본문

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

| 필드 | 설명 |
|---|---|
| `trade_no` | 패널측 거래 번호(주문 생성 시 발행, 접두사 `CF`) |
| `site_trade_no` | 결제 플랫폼 주문 번호(서명됨) |
| `status` | 비즈니스 상태, **서명되지 않음** |
| `amount` / `currency` | 주문 금액(센트 정수) 및 통화(둘 다 서명됨) |
| `timestamp` | 유닉스 초; 창 밖에서 거부됨 (서명됨) |
| `nonce` | 재생 보호: 원자적 INSERT, 각 nonce는 한 번 처리됩니다(서명됨). |
| `paid_*` / `method` / `exchange_rate` / `paid_at` | 실제 결제 정보, **서명되지 않음**(`body_hash`에서 무결성 보장) |
| `sign` | HMAC-SHA256 서명, 16진수 인코딩(`sign` 없이 본문을 통해 계산됨) |
| `extra` | 확장 필드 |

## 3. 서명 알고리즘(v3.2 / `version: "v1"`)

1. **`sign` 없이 원시 본문**을 빌드하고 `body_hash = SHA256(raw body bytes)`(소문자 16진수)를 계산합니다.
   > 또한 패널은 `sign`를 포함하여 전체 본문을 해시하는 레거시 체계를 허용합니다. 새로운 통합은 문서화된 체계를 따라야 합니다.
2. 서명 페이로드를 빌드합니다(**사전순으로 된 키**, Go `json.Marshal` 의미 체계):

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

3. `sign = hex( HMAC_SHA256( key=callback secret, msg=JSON bytes from step 2) )`.
   > 비밀은 관리 UI에 탑재된 사이트의 콜백 비밀입니다(회전 후 이전 비밀은 하나의 전환 창 동안 유지됩니다).

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

## 4. 패널 측 검증 체인(발생하는 오류 결정)

1. 사이트가 존재하고 해당 도메인이 확인되었습니다 → 404 / 403
2. IP 허용 목록(`callback_whitelist`가 구성된 경우 목록에 없는 IP는 **하드 차단**됩니다. 그렇지 않으면 건너뛰고 서명만 확인됩니다.)
3. `timestamp` 창 + `nonce` 재생 보호(Atomic INSERT)
4. HMAC 확인(활성 및 레거시 비밀 모두 허용)
5. 주문이 존재하고 해당 사이트에 속함 → 멱등성(이미 결제된 반품 성공) → **금액 및 통화 잠금 유효성 검사** → 거래 게시(잔액 원장 + 주문 상태)

처리는 **멱등성**입니다. 동일한 `trade_no`를 사용한 반복 콜백은 두 번 게시되지 않습니다. 서명/속도 이상은 관리자가 수동으로 검사하고 재시도할 수 있는 "콜백 이벤트"로 기록됩니다.

## 5. 응답

- 성공: 2xx(패널이 지불을 게시한 후)
- 실패: 4xx/5xx + `{ "code": "...", "message": "..." }`; 일반적인 코드에는 `INVALID_SIGNATURE`, `IP_BLOCKED`, `ORDER_NOT_FOUND`, 금액 불일치 등이 포함됩니다.

> 모든 콜백에는 `X-Request-ID`(없는 경우 패널에서 생성됨)가 포함됩니다. 로그와 이벤트는 처음부터 끝까지 추적 가능합니다. 문제를 해결할 때 관리자에게 이 ID를 제공하세요.

## 6. 테스트 및 문제 해결

- 관리자는 각 사이트의 **통합 문서**: `GET /api/v1/charge-sites/{id}/integration-doc`(사이트의 실제 콜백 URL 예제 포함)를 가져올 수 있습니다.
- 콜백 이벤트: `GET /api/v1/charge-callback-events`; 재시도: `GET /api/v1/charge-callback-retries`, `POST /api/v1/charge-callback-retries/{id}/retry`.
- 비밀 순환: `POST /api/v1/charge-sites/{id}/rotate-secret`(이전 비밀은 하나의 전환 기간 동안 유지됩니다).
