<!-- source: charge-callback.md | locale: fa | updated: 2026-10-23 -->
# شارژ مجدد تماس API

> زبان‌ها: [简体中文](charge-callback.md) · [English](charge-callback.en.md) · [繁體中文](charge-callback.zh-TW.md) · [Русский](charge-callback.ru.md) · [日本語](charge-callback.ja.md) · [한국어](charge-callback.ko.md) · [Deutsch](charge-callback.de.md) · [Français](charge-callback.fr.md) · [Español](charge-callback.es.md) · [Português](charge-callback.pt.md) · [Türkçe](charge-callback.tr.md) · [Bahasa Indonesia](charge-callback.id.md) · [ไทย](charge-callback.th.md) · [Tiếng Việt](charge-callback.vi.md) · [العربية](charge-callback.ar.md) · فارسی

برای **پلتفرم های پرداخت / یکپارچه سازهای سایت**: پس از اینکه کاربر در یک سایت نصب شده پرداخت می کند، سرور پلت فرم پرداخت به پنل اطلاع می دهد.

## 1. نقطه پایانی

```
POST /api/v1/charge/callback/{site_id}
Content-Type: application/json
```

`site_id` شناسه «سایت نصب‌شده» (عدد صحیح) سمت مدیریت است. تماس برگشتی سرور به سرور است و از **امضای HMAC** به اضافه یک لیست مجاز IP اختیاری به جای تأیید حامل استفاده می کند.

## 2. درخواست بدن

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

| میدان | توضیحات |
|---|---|
| `trade_no` | شماره تراکنش سمت پنل (صدور در هنگام ایجاد سفارش، پیشوند `CF`) |
| `site_trade_no` | شماره سفارش بستر پرداخت (امضا) |
| `status` | وضعیت کسب و کار، **امضا نشده** |
| `amount` / `currency` | مبلغ سفارش (سنت عدد صحیح) و ارز (هر دو امضا شده) |
| `timestamp` | یونیکس ثانیه؛ خارج از پنجره رد شد (امضا) |
| `nonce` | محافظت از پخش مجدد: INSERT اتمی، هر یک یک بار پردازش شده (امضا) |
| `paid_*` / `method` / `exchange_rate` / `paid_at` | اطلاعات پرداخت واقعی، **امضا نشده** (یکپارچگی تحت پوشش `body_hash`) |
| `sign` | امضای HMAC-SHA256، کد شش‌گانه (محاسبه شده روی بدنه بدون `sign`) |
| `extra` | فیلدهای پسوند |

## 3. الگوریتم امضا (نسخه 3.2 / `version: "v1"`)

1. **بدنه خام را بدون `sign`** بسازید و `body_hash = SHA256(raw body bytes)` (حروف کوچک هگز) را محاسبه کنید.
   > پانل همچنین طرح قدیمی را می پذیرد که کل بدنه از جمله `sign` را هش می کند. ادغام های جدید باید از طرح مستند پیروی کنند.
2. بار امضا را بسازید (**کلیدها به ترتیب واژگان**، Go `json.Marshal` semantics):

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
   > راز رمز پاسخ تماس سایت نصب شده در رابط کاربری مدیریت است (پس از چرخش، راز قبلی برای یک پنجره انتقال نگه داشته می شود).

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

## 4. زنجیره اعتبارسنجی سمت پنل (خطایی که دریافت می کنید را تعیین می کند)

1. سایت وجود دارد و دامنه آن تأیید شده است → 404 / 403
2. لیست مجاز IP (اگر `callback_whitelist` پیکربندی شده باشد، IP های غیر فهرست شده **مسدود مسدود می شوند**؛ در غیر این صورت از قلم افتاده و فقط امضا بررسی می شود)
3. پنجره `timestamp` + حفاظت از پخش مجدد `nonce` (INSERT اتمی)
4. تأیید HMAC (رازهای فعال و قدیمی را می پذیرد)
5. سفارش وجود دارد و به سایت تعلق دارد → idempotency (بازگشت از قبل پرداخت شده موفقیت آمیز) → **تأیید قفل مبلغ و ارز** → ارسال تراکنش (دفتر کل موجودی + وضعیت سفارش)

پردازش **ناتوان** است: تماس های مکرر با همان `trade_no` دو بار پست نمی شوند. ناهنجاری‌های امضا/نرخ به‌عنوان «رویدادهای پاسخ به تماس» ثبت می‌شوند، که مدیران می‌توانند به صورت دستی بازرسی و دوباره امتحان کنند.

## 5. پاسخ ها

- موفقیت: 2xx (بعد از اینکه پانل پرداخت را ارسال کرد)
- خرابی: 4xx/5xx + `{ "code": "...", "message": "..." }`؛ کدهای معمولی شامل `INVALID_SIGNATURE`، `IP_BLOCKED`، `ORDER_NOT_FOUND`، عدم تطابق مقدار و غیره است.

> هر تماس برگشتی دارای `X-Request-ID` است (در صورت عدم وجود توسط پانل تولید می شود). گزارش‌ها و رویدادها پایان به انتها قابل ردیابی هستند. هنگام عیب یابی این شناسه را به سرپرست خود بدهید.

## 6. تست و عیب یابی

- مدیران می‌توانند **سند یکپارچه‌سازی** هر سایت را واکشی کنند: `GET /api/v1/charge-sites/{id}/integration-doc` (شامل نمونه‌های پاسخ تماس واقعی سایت URL).
- رویدادهای پاسخ به تماس: `GET /api/v1/charge-callback-events`; تکرار: `GET /api/v1/charge-callback-retries`، `POST /api/v1/charge-callback-retries/{id}/retry`.
- چرخش مخفی: `POST /api/v1/charge-sites/{id}/rotate-secret` (راز قبلی برای یک پنجره انتقال نگهداری می شود).
