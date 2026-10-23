<!-- source: charge-callback.md | locale: ar | updated: 2026-10-23 -->
# رد الاتصال المسؤول API

> اللغات: [简体中文](charge-callback.md) · [English](charge-callback.en.md) · [繁體中文](charge-callback.zh-TW.md) · [Русский](charge-callback.ru.md) · [日本語](charge-callback.ja.md) · [한국어](charge-callback.ko.md) · [Deutsch](charge-callback.de.md) · [Français](charge-callback.fr.md) · [Español](charge-callback.es.md) · [Português](charge-callback.pt.md) · [Türkçe](charge-callback.tr.md) · [Bahasa Indonesia](charge-callback.id.md) · [ไทย](charge-callback.th.md) · [Tiếng Việt](charge-callback.vi.md) · العربية · [فارسی](charge-callback.fa.md)

بالنسبة إلى **منصات الدفع / أدوات تكامل المواقع**: بعد أن يدفع المستخدم على موقع مثبت، يقوم خادم منصة الدفع بإعلام اللوحة.

## 1. نقطة النهاية

```
POST /api/v1/charge/callback/{site_id}
Content-Type: application/json
```

`site_id` هو معرف "الموقع المثبت" من جانب المسؤول (عدد صحيح). رد الاتصال هو من خادم إلى خادم ويستخدم **توقيع HMAC** بالإضافة إلى قائمة السماح لـ IP الاختيارية بدلاً من مصادقة Bearer.

## 2. هيئة الطلب

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

| الميدان | الوصف |
|---|---|
| `trade_no` | رقم المعاملة على جانب اللوحة (تم إصداره عند إنشاء الطلب، البادئة `CF`) |
| `site_trade_no` | رقم طلب منصة الدفع (موقع) |
| `status` | حالة العمل، **غير موقع** |
| `amount` / `currency` | مبلغ الطلب (عدد صحيح) والعملة (كلاهما موقع) |
| `timestamp` | ثواني يونيكس؛ مرفوض خارج النافذة (توقيع) |
| `nonce` | حماية إعادة التشغيل: إدراج ذري، تتم معالجة كل رقم مرة واحدة (موقع) |
| `paid_*` / `method` / `exchange_rate` / `paid_at` | معلومات الدفع الفعلية، **غير موقعة** (تغطي `body_hash` تكاملها) |
| `sign` | توقيع HMAC-SHA256، مشفر سداسي عشري (محسوب على الجسم بدون `sign`) |
| `extra` | حقول الامتداد |

## 3. خوارزمية التوقيع (الإصدار 3.2 / `version: "v1"`)

1. قم ببناء ** الجسم الخام بدون `sign` ** واحسب `body_hash = SHA256(raw body bytes)` (أحرف صغيرة ست عشرية).
   > تقبل اللوحة أيضًا النظام القديم الذي يقوم بتجزئة الجسم بالكامل بما في ذلك `sign`؛ يجب أن تتبع عمليات التكامل الجديدة المخطط الموثق.
2. أنشئ حمولة التوقيع (** المفاتيح بترتيب معجمي**، دلالات Go `json.Marshal`):

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
   > السر هو سر رد الاتصال الخاص بالموقع المثبت في واجهة المستخدم الإدارية (بعد التدوير، يتم الاحتفاظ بالسر السابق لنافذة انتقالية واحدة).

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

## 4. سلسلة التحقق من جانب اللوحة (تحدد الأخطاء التي تحصل عليها)

1. الموقع موجود وتم التحقق من المجال الخاص به → 404 / 403
2. قائمة IP المسموح بها (إذا تم تكوين `callback_whitelist`، تكون عناوين IP غير المدرجة **محظورة بشدة**؛ وإلا فسيتم تخطيها ويتم التحقق من التوقيع فقط)
3. نافذة `timestamp` + حماية إعادة التشغيل `nonce` (إدراج ذري)
4. التحقق من HMAC (يقبل الأسرار النشطة والقديمة)
5. الطلب موجود وينتمي إلى الموقع → العجز (نجاح العوائد المدفوعة بالفعل) → **التحقق من صحة قفل المبلغ والعملة** → ترحيل المعاملات (دفتر الأستاذ الرصيد + حالة الطلب)

المعالجة ** غير فعالة **: لا يتم نشر عمليات الاسترجاعات المتكررة بنفس `trade_no` مرتين. يتم تسجيل الحالات الشاذة في التوقيع/المعدل على أنها "أحداث رد اتصال"، والتي يمكن للمسؤولين فحصها وإعادة المحاولة يدويًا.

## 5. الردود

- النجاح: 2xx (بعد قيام اللجنة بنشر الدفعة)
- الفشل: 4xx/5xx + `{ "code": "...", "message": "..." }`؛ تتضمن الرموز النموذجية `INVALID_SIGNATURE`، و`IP_BLOCKED`، و`ORDER_NOT_FOUND`، وعدم تطابق المبلغ، وما إلى ذلك.

> يحمل كل رد اتصال `X-Request-ID` (الذي تم إنشاؤه بواسطة اللوحة في حالة غيابه)؛ يمكن تتبع السجلات والأحداث من البداية إلى النهاية. قم بإعطاء هذا المعرف إلى المسؤول عند استكشاف الأخطاء وإصلاحها.

## 6. الاختبار واستكشاف الأخطاء وإصلاحها

- يمكن للمسؤولين جلب **مستند التكامل** الخاص بكل موقع: `GET /api/v1/charge-sites/{id}/integration-doc` (يتضمن أمثلة رد الاتصال الفعلية للموقع URL).
- أحداث رد الاتصال: `GET /api/v1/charge-callback-events`؛ إعادة المحاولة: `GET /api/v1/charge-callback-retries`، `POST /api/v1/charge-callback-retries/{id}/retry`.
- التدوير السري: `POST /api/v1/charge-sites/{id}/rotate-secret` (يتم الاحتفاظ بالسر السابق لنافذة انتقالية واحدة).
