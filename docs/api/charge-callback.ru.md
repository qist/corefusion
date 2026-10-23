<!-- source: charge-callback.md | locale: ru | updated: 2026-10-23 -->
# Обратный вызов по оплате API

> Языки: [简体中文](charge-callback.md) · [English](charge-callback.en.md) · [繁體中文](charge-callback.zh-TW.md) · Русский · [日本語](charge-callback.ja.md) · [한국어](charge-callback.ko.md) · [Deutsch](charge-callback.de.md) · [Français](charge-callback.fr.md) · [Español](charge-callback.es.md) · [Português](charge-callback.pt.md) · [Türkçe](charge-callback.tr.md) · [Bahasa Indonesia](charge-callback.id.md) · [ไทย](charge-callback.th.md) · [Tiếng Việt](charge-callback.vi.md) · [العربية](charge-callback.ar.md) · [فارسی](charge-callback.fa.md)

Для **платёжных платформ/интеграторов сайтов**: после оплаты пользователем на смонтированном сайте сервер платёжной платформы уведомляет панель.

## 1. Конечная точка

```
POST /api/v1/charge/callback/{site_id}
Content-Type: application/json
```

`site_id` — это идентификатор «подключенного сайта» на стороне администратора (целое число). Обратный вызов осуществляется между серверами и использует **подпись HMAC** плюс дополнительный список разрешенных IP-адресов вместо аутентификации носителя.

## 2. Тело запроса

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

| Поле | Описание |
|---|---|
| `trade_no` | Номер транзакции на стороне панели (выдается при создании заказа, префикс `CF`) |
| `site_trade_no` | Номер заказа Платёжной платформы (подписанный) |
| `status` | Статус компании, **не подписан** |
| `amount` / `currency` | Сумма заказа (целое число в центах) и валюта (оба подписаны) |
| `timestamp` | Unix-секунды; отклонено за окном (подпись) |
| `nonce` | Защита от повторного воспроизведения: атомарный INSERT, каждый одноразовый номер обрабатывается один раз (подписанный). |
| `paid_*` / `method` / `exchange_rate` / `paid_at` | Фактическая информация о платеже, **не подписанная** (целостность обеспечивается `body_hash`) |
| `sign` | Подпись HMAC-SHA256, шестнадцатеричная (вычисляется по телу без `sign`) |
| `extra` | Поля расширения |

## 3. Алгоритм подписи (v3.2/`version: "v1"`)

1. Создайте **необработанное тело без `sign`** и вычислите `body_hash = SHA256(raw body bytes)` (шестнадцатеричный строчный регистр).
   > Панель также поддерживает устаревшую схему, которая хеширует все тело, включая `sign`; новые интеграции должны следовать документированной схеме.
2. Создайте полезные данные для подписи (**ключи в лексикографическом порядке**, семантика Go `json.Marshal`):

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
   > Секрет — это секрет обратного вызова смонтированного сайта в пользовательском интерфейсе администратора (после ротации предыдущий секрет сохраняется в течение одного окна перехода).

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

## 4. Цепочка проверки на стороне панели (определяет получаемые вами ошибки)

1. Сайт существует и его домен подтвержден → 404 / 403
2. Список разрешенных IP-адресов (если настроен `callback_whitelist`, IP-адреса, не указанные в списке, **жестко блокируются**; в противном случае они пропускаются и проверяется только подпись)
3. Окно `timestamp` + защита от повтора `nonce` (атомарная вставка)
4. Проверка HMAC (принимает как активные, так и устаревшие секреты)
5. Заказ существует и принадлежит сайту → идемпотентность (уже оплаченный возврат успешен) → **проверка блокировки суммы и валюты** → проводка транзакций (регистр баланса + статус заказа)

Обработка **идемпотентна**: повторяющиеся обратные вызовы с одним и тем же `trade_no` не отправляются дважды. Аномалии сигнатуры/скорости записываются как «события обратного вызова», которые администраторы могут проверить и повторить вручную.

## 5. Ответы

- Успех: 2xx (после того, как панель разместила платеж)
- Сбой: 4xx/5xx + `{ "code": "...", "message": "..." }`; типичные коды включают `INVALID_SIGNATURE`, `IP_BLOCKED`, `ORDER_NOT_FOUND`, несоответствие суммы и т. д.

> Каждый обратный вызов содержит `X-Request-ID` (генерируется панелью, если он отсутствует); журналы и события отслеживаются от начала до конца. Сообщите этот идентификатор своему администратору при устранении неполадок.

## 6. Тестирование и устранение неполадок

- Администраторы могут получить **документ по интеграции** для каждого сайта: `GET /api/v1/charge-sites/{id}/integration-doc` (включает фактические примеры обратного вызова URL для сайта).
- События обратного вызова: `GET /api/v1/charge-callback-events`; повторные попытки: `GET /api/v1/charge-callback-retries`, `POST /api/v1/charge-callback-retries/{id}/retry`.
- Ротация секрета: `POST /api/v1/charge-sites/{id}/rotate-secret` (предыдущий секрет сохраняется в течение одного окна перехода).
