<!-- source: portal-api.md | locale: fa | updated: 2026-10-23 -->
# مرجع پورتال API (کارفرما / پورتال API)

> زبان‌ها: [简体中文](portal-api.md) · [English](portal-api.en.md) · [繁體中文](portal-api.zh-TW.md) · [Русский](portal-api.ru.md) · [日本語](portal-api.ja.md) · [한국어](portal-api.ko.md) · [Deutsch](portal-api.de.md) · [Français](portal-api.fr.md) · [Español](portal-api.es.md) · [Português](portal-api.pt.md) · [Türkçe](portal-api.tr.md) · [Bahasa Indonesia](portal-api.id.md) · [ไทย](portal-api.th.md) · [Tiếng Việt](portal-api.vi.md) · [العربية](portal-api.ar.md) · فارسی

> مرجع رابط **کامل** برای فرانت اند پورتال (`/p` یا یک پورت پورتال اختصاصی) و مشتریان شخص ثالث.
> تعاریف مسیر: `internal/api/router/router.go` (`clientAuthGroup` / `clientAuthProtected` / `/api/v1/public/*`).

## 1. کنوانسیون های عمومی

### 1.1 پایه URL

| استقرار | پایه URL |
|---|---|
| موتور پنل اصلی | `https://<panel-domain>/api/v1` (صفحه پورتال در `/p`) |
| پورتال اختصاصی | `https://<portal-domain>[:port]/api/v1` (صفحه پورتال در `/`) |
| دامنه پورتال اختصاصی + پروکسی معکوس | همانند بالا؛ پروکسی باید هر دو مسیر صفحه و `/api/v1` را ارسال کند |

### 1.2 احراز هویت

- پس از ورود به سیستم، `Authorization: Bearer <access_token>` را ارسال کنید.
- توکن‌های دسترسی 24 ساعت طول می‌کشد، توکن‌ها 7 روز به‌روزرسانی می‌شوند.
- JWT حاوی `tv` (نسخه_token): خروج / خروج اجباری آن را افزایش می‌دهد و همه توکن‌های قدیمی را باطل می‌کند.
 (`clients.token_version`، `internal/service/clientauth/service.go` را ببینید).
- هر نقطه پایانی محافظت شده از `ClientAuthMiddleware` (بررسی نسخه + محافظ حالت نگهداری) عبور می کند.

### 1.3 پاکت پاسخ

اکثر نقاط پایانی `{ "code": 0, "data": ..., "message": "..." }` را برمی‌گردانند.
برخی از نقاط پایانی لیست `{ "items": [...], "total": N }` یا یک آرایه خالی را برمی‌گردانند (در نقطه پایانی زیر ذکر شده است).
خطاها `{ "code": "ERROR_CODE", "message": "...", "request_id": "...", "can_retry": ... }` را برمی گرداند
(HTTP 4xx/5xx).

### 1.4 واحد مقدار

فیلدهای سفارش / موجودی **سنت عدد صحیح** هستند: `990` = 9.90 ¥ (`currency: "CNY"`).

### 1.5 مثال واقعی (ورود به سیستم → واکشی نمایه)

```bash
# Register (when registration_enabled=true)
curl -X POST https://<portal-domain>/api/v1/client-auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","password":"plaintext-password","referral_code":""}'
# → 201 {"data":{"access_token":"<JWT>","refresh_token":"<JWT>","client":{...}}}

# Sign in
curl -X POST https://<portal-domain>/api/v1/client-auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","password":"plaintext-password"}'
# → 200 same as above

# Fetch the current client profile (with token)
curl https://<portal-domain>/api/v1/client-auth/me \
  -H 'Authorization: Bearer <access_token>'
# → 200 {"data":{"id":27,"email":"user@example.com","nickname":"","status":"active","quota_bytes":10737418240,"used_bytes":0,...}}

# Fetch a subscription (no header, token in the path)
curl 'https://<portal-domain>/sub/<subscription_token>?format=mihomo_yaml'
# → 200 Mihomo YAML (or base64_uri / singbox_json / xray_json)

# Error example: expired token
# → 401 {"code":"errClientTokenRevoked","message":"...","request_id":"...","can_retry":false}
```

در 401، یک بار با `POST /api/v1/client-auth/refresh` تماس بگیرید و دوباره امتحان کنید. اگر باز هم شکست خورد، جلسه محلی را پاک کنید (الگوی پورتال قبلاً این کار را انجام می دهد).

---

## 2. نقاط پایانی عمومی (بدون ورود به سیستم)

| روش | مسیر | توضیحات |
|---|---|---|
| GET | `/p` (`/` در موتور پورتال اختصاصی) | پورتال HTML (ارائه شده توسط سرور + زمان اجرا Vue) |
| GET | `/api/v1/public/portal-config` | پیکربندی زمان اجرا پورتال: نام سایت، رنگ‌های تم، پیمایش، ماژول‌ها، بسته‌های زبان، `registration_enabled`، `regions` (منبع منطقه ورود)، `privacy_policy_url` / `terms_of_service_url` / `privacy_policy_content` / `terms_of_service_content`; `/p/legal/*` ✓) |
| GET | `/api/v1/public/settings` | تنظیمات عمومی (در حال حاضر `registration_enabled`) |
| GET | `/api/v1/public/subscription-endpoints` | لیست نقطه پایانی اشتراک (آرایه خالی) |
| GET | `/api/v1/public/articles` | مقالات عمومی (فقط منتشر شده) |
| GET | `/api/v1/public/articles/:slug` | جزئیات مقاله |
| GET | `/api/v1/public/client-configs` | پیکربندی دانلود مشتری |
| GET | `/sub/:token` | تحویل اشتراک (`format` base64_uri / raw_uri / mihomo_yaml / singbox_json / xray_json و غیره را برمی‌گرداند؛ لغو/غایب → 404، معلق/منقضی شده/غیرفعال → 403 را ببینید. [subscription.md](portal-api.fa.md)) |
| GET | `/api/v1/client-auth/oauth/providers` | قابلیت مشاهده ارائه دهنده OAuth: `{"data":{"providers":[{"provider":"github","enabled":true},...]}}` |
| GET | `/api/v1/system/maintenance` | وضعیت نگهداری (عمومی). در صورت فعال بودن، پورتال یک صفحه تعمیر و نگهداری را نشان می دهد |

> همه اینها شامل محافظ تعمیر و نگهداری و محدودیت نرخ عمومی API است.

## 3. Auth / OAuth

| روش | مسیر | احراز هویت | توضیحات |
|---|---|---|---|
| POST | `/api/v1/client-auth/register` | — | ثبت نام کنید. بدنه: `{email, password, referral_code?}`. `{access_token, refresh_token, client}` را برمی گرداند. نرخ ثبت نام محدود |
| POST | `/api/v1/client-auth/login` | — | وارد شوید. بدنه: `{email, password}`. پاسخ مشابه؛ JWT ها را با `token_version` صادر می کند |
| POST | `/api/v1/client-auth/refresh` | — | تازه کردن. بدنه: `{refresh_token}`. نشانه‌های دسترسی/بازخوانی جدید را برمی‌گرداند (نسخه بررسی شده؛ نشانه‌های تازه‌سازی قدیمی بلافاصله پس از خروج از سیستم می‌میرند) |
| POST | `/api/v1/client-auth/logout` | ✓ | خروج: ابطال سمت سرور (`token_version +1`) → همه توکن های قدیمی برای مشتری نامعتبر هستند |
| POST | `/api/v1/client-auth/rotate-token` | ✓ | رمز اشتراک را بچرخانید |
| POST | `/api/v1/client-auth/ticket` | ✓ | صدور یک **بلیط کوتاه مدت یکبار مصرف** (برای جریانهای `window.open` مانند "دانلود log bundle" که در آن سرصفحه ها قابل تنظیم نیستند) |
| GET | `/api/v1/client-auth/oauth/:provider` | — | شروع OAuth: 307 → GitHub/Google رضایت (`state` anti-CSRF، 10 دقیقه)؛ ارائه دهندگان غیرفعال 400 را برمی گردانند |
| GET | `/api/v1/client-auth/oauth/:provider/callback` | — | پاسخ به تماس OAuth: کد تبادل ← ایجاد/پیوند کلاینت ← نسخه نسخه شده JWT ← 302 بازگشت به پورتال (`/<portalPath>?access_token=...&refresh_token=...`) |

> پاسخ تماس OAuth یک **تغییر مسیر مرورگر** است (نه یک تماس API): پس از ورود موفقیت آمیز به سیستم، پورتال توکن را می خواند.
> از URL در `onMounted` (یا از طریق `postMessage` همان مبدا، به `handleOAuthCallback` در قالب پورتال `web` مراجعه کنید).

## 4. حساب

| روش | مسیر | توضیحات |
|---|---|---|
| GET | `/api/v1/client-auth/me` | مشتری فعلی: `{id, email, plan_id, plan_name, status, quota_bytes, used_bytes, upload_bytes, download_bytes, expires_at, ip_whitelist, device_limit, sub_token, sub_status, created_at, ...}` (پاکت `data`) |
| POST | `/api/v1/client-auth/change-password` | بدنه: `{old_password, new_password}` |
| GET | `/api/v1/client-auth/checktime` | زمان سرور (همگام سازی ساعت مشتری) |
| GET | `/api/v1/client-auth/connection-info` | پارامترهای اتصال طرح فعلی: `{concurrent, device_limit, protocol, speed_limit, tls, transport}` |

## 5. اشتراک ها

| روش | مسیر | توضیحات |
|---|---|---|
| POST | `/api/v1/client-auth/reset-subscription` | بازنشانی اشتراک؛ این `sub_token` را می چرخاند |

> پیوند اشتراک در قسمت جلویی از `sub_token` ساخته شده است که توسط `/me` (`/sub/:token`) بازگردانده شده است. وجود ندارد
> دیگر نقاط پایانی `subscription-link` / `subscription-formats` را جدا کنید.

## 6. گره ها

| روش | مسیر | توضیحات |
|---|---|---|
| GET | `/api/v1/client-auth/nodes` | گره های قابل مشاهده برای طرح فعلی: `{items:[...]}` (فیلدهای نمایش مانند نام/پروتکل/حمل و نقل/منطقه) |
| GET | `/api/v1/client-auth/nodes/:id/config` | پیکربندی کامل گره |
| GET | `/api/v1/client-auth/nodes/:id/link` | پیوند را به اشتراک بگذارید (vless:// و غیره) |
| GET | `/api/v1/client-auth/nodes/:id/qrcode` | تصویر کد QR |

## 7. فروشگاه / طرح ها / سفارشات

| روش | مسیر | توضیحات |
|---|---|---|
| GET | `/api/v1/client-auth/plans` | لیست پلان (آرایه خالی): `{id, name, description, category, sort_order, variants:[{id, name, billing_cycle, duration_days, price(cents), original_price, currency, quota_bytes, ...}]}` |
| GET | `/api/v1/client-auth/plans/:id` | جزئیات طرح |
| GET | `/api/v1/client-auth/traffic-packs` | لیست بسته ترافیک |
| POST | `/api/v1/client-auth/traffic-packs/buy` | یک بسته ترافیکی بخرید |
| GET | `/api/v1/client-auth/orders` | سفارشات طرح من (صفحه بندی شده) |
| POST | `/api/v1/client-auth/orders` | ایجاد یک سفارش (خرید / تمدید یک نوع طرح) |
| GET | `/api/v1/client-auth/orders/:id` | جزئیات سفارش |
| GET | `/api/v1/client-auth/invoices` | فاکتورها |
| GET | `/api/v1/client-auth/invoices/:id` | جزئیات فاکتور |

## 8. تعادل / شارژ مجدد

| روش | مسیر | توضیحات |
|---|---|---|
| GET | `/api/v1/client-auth/balance` | `{data:{client_id, balance(cents), frozen}}` |
| GET | `/api/v1/client-auth/balance/transactions` | تراکنش های موجود (`type`: شارژ/مصرف/بازپرداخت؛ صفحه بندی شده) |
| GET | `/api/v1/client-auth/charge/sites` | سایت های شارژ موجود (`ListChargeSites`: فقط فعال) |
| POST | `/api/v1/client-auth/charge/create` | ایجاد یک سفارش شارژ (به پرداخت شخص ثالث هدایت می شود) |
| GET | `/api/v1/client-auth/charge/orders` | لیست سفارش شارژ |
| GET | `/api/v1/client-auth/charge/orders/:id` | جزئیات سفارش شارژ مجدد |
| GET | `/api/v1/client-auth/charge/status/:trade_no` | وضعیت پرداخت نظرسنجی توسط trade_no |
| POST | `/api/v1/client-auth/charge/redeem-card` | بازخرید کارت شارژ |

> حذف یک مشتری **آبشار** به موجودی و تراکنش های آن (به `DeleteClient` در `internal/service/client/service.go` مراجعه کنید).

## 9. هدایا / سابقه ترافیک

| روش | مسیر | توضیحات |
|---|---|---|
| GET | `/api/v1/client-auth/gifts` | هدایای قابل مطالبه (آرایه خالی؛ `type`: روز/بایت) |
| POST | `/api/v1/client-auth/gifts/:id/claim` | ادعا کنید |
| GET | `/api/v1/client-auth/traffic-history` | تاریخچه ترافیک ماهانه: `{data:{history:[{period_start,total_bytes,...}], months}}` |

## 10. بلیط

| روش | مسیر | توضیحات |
|---|---|---|
| GET | `/api/v1/client-auth/tickets` | لیست بلیط: `{items,total}` |
| POST | `/api/v1/client-auth/tickets` | یک بلیط ایجاد کنید |
| GET | `/api/v1/client-auth/tickets/:id` | جزئیات بلیط (با پاسخ) |
| POST | `/api/v1/client-auth/tickets/:id/reply` | پاسخ دهید |
| POST | `/api/v1/client-auth/tickets/:id/close` | بستن |

## 11. مقالات / راهنماها

| روش | مسیر | توضیحات |
|---|---|---|
| GET | `/api/v1/client-auth/articles` | فهرست مقاله (با فیلتر دسته بندی) |
| GET | `/api/v1/client-auth/articles/:slug` | جزئیات مقاله |

## 12. ارجاعات (به طور پیش فرض غیرفعال است؛ می توان آن را در ویرایشگر پورتال فعال کرد)

| روش | مسیر | توضیحات |
|---|---|---|
| GET | `/api/v1/client-auth/referral/code` | کد و لینک ارجاع من |
| GET | `/api/v1/client-auth/referral/stats` | آمار کمیسیون (`commission_rate`، `available_commission`، `min_withdraw_amount`) |
| GET | `/api/v1/client-auth/referral/records` | سوابق ارجاع |
| GET | `/api/v1/client-auth/referral/withdrawals` | سوابق برداشت |
| POST | `/api/v1/client-auth/referral/withdraw` | درخواست برداشت |

## 13. حساب های مشترک

| روش | مسیر | توضیحات |
|---|---|---|
| GET | `/api/v1/client-auth/shared-accounts` | حساب‌های اشتراک‌گذاری شده موجود (جریان‌گذاری و غیره) |
| GET | `/api/v1/client-auth/shared-accounts/:id` | جزئیات |
| POST | `/api/v1/client-auth/shared-accounts/:id/apply` | درخواست دسترسی |
| POST | `/api/v1/client-auth/shared-accounts/:id/repair` | تعمیر با یک کلیک |

## 14. بسته‌های گزارش (تشخیص مشترک)

| روش | مسیر | توضیحات |
|---|---|---|
| GET | `/api/v1/client-auth/logs/types` | انواع گزارش‌هایی که می‌توان آنها را دسته‌بندی کرد |
| GET | `/api/v1/client-auth/logs/packages` | دسته های سیاهه من |
| POST | `/api/v1/client-auth/logs/package` | ایجاد یک کار بسته‌بندی گزارش (فقط گره‌های ورودی) |
| GET | `/api/v1/client-auth/logs/packages/:id` | وضعیت شغلی (آماده / در حال پردازش / ناموفق) |
| GET | `/api/v1/client-auth/logs/packages/:id/download` | دانلود کنید (`POST /ticket` را ترجیح دهید و `?ticket=` را اضافه کنید) |
| DELETE | `/api/v1/client-auth/logs/packages/:id` | یک بسته گزارش را حذف کنید |

## 15. تنظیمات امنیتی / فهرست مجاز IP / خود لغو ممنوعیت

| روش | مسیر | توضیحات |
|---|---|---|
| GET/PUT | `/api/v1/client-auth/settings` | تنظیمات مشتری: `{ip_whitelist, device_limit}` (PUT) |
| GET | `/api/v1/client-auth/settings/whitelist` | فهرست مجاز (آرایه خالی) |
| DELETE | `/api/v1/client-auth/settings/whitelist/:id` | یک ورودی لیست مجاز را حذف کنید |
| POST | `/api/v1/client-auth/settings/auto-add-ip` | IP فعلی را به لیست مجاز اضافه کنید |
| POST | `/api/v1/client-auth/self-unban` | سلف سرویس لغو ممنوعیت (تلاش ها توسط خط مشی محدود شده است) |

> فهرست مجاز IP **فقط برای URL های اشتراک اعمال می شود** (`/sub/:token`). ورود به سیستم/OAuth یا پورتال را مسدود نمی کند
> سایر نقاط پایانی تأیید اعتبار مشتری اگر کاربر لیست مجاز یا اشتراک URL نداشته باشد، دسترسی همیشه مجاز است. این
> لیست مجاز یک تنظیم برای هر کاربر است. سوئیچ جهانی وجود ندارد

**جریان خود لغو ممنوعیت**: فقط برای `auto_quota` (ممنوعیت خودکار بیش از حد ترافیک) زمانی که سمت سرپرست
`self_unban.enabled` روشن است. ورود به سیستم (گذرواژه یا پاسخ به تماس OAuth) `require_self_unban=true` + `self_unban_token` را برمی‌گرداند
(typ=self_unban، 15 دقیقه معتبر، **فقط** قابل استفاده در `POST /self-unban`؛ سایر نقاط پایانی 403/`SELF_UNBAN_TOKEN_SCOPE` را برمی‌گردانند).
سپس پورتال کارت unban را نشان می دهد. پس از لغو ممنوعیت موفقیت آمیز، کاربر دوباره وارد سیستم می شود. تماس‌های OAuth این را از طریق ارائه می‌کنند
قطعه URL `#require_self_unban=1&self_unban_token=..&self_unban_hint=..`.

## 16. کدهای خطا (گزیده)

| کد | سناریو |
|---|---|
| `INVALID_TOKEN` | نشانه بازخوانی نامعتبر/منقضی شده/تناسب نسخه |
| `INVALID_REQUEST` | اعتبارسنجی پارامتر ناموفق بود |
| `NOT_FOUND` | منبع از دست رفته (بلیت/سفارش/ورود در لیست مجاز و غیره) |
| `RELOAD_FAILED` / `SYNC_FAILED` | (سمت سرپرست) همگام سازی پیکربندی پروکسی انجام نشد |
| 401 + `errClientTokenRevoked` | عدم تطابق token_version (از سیستم خارج شده/لگد شده) → مشتریان باید refresh شوند. اگر موفق نشد، توکن‌های محلی را پاک کنید |

## 17. اسناد انطباق (سیاست حفظ حریم خصوصی / شرایط خدمات)

پیوندهای «خط‌مشی رازداری · شرایط خدمات» در پایین کارت ورود به سیستم (همانند سایت‌های معمولی؛ کلیک کردن محتوا را باز می‌کند):

- **پیکربندی**: ویرایشگر پورتال → تنظیمات → عمومی → اسناد مطابقت (**ویرایش محتوا در هر زبان**).
 همچنین می توانید `PUT /api/v1/portal/active` با فیلدهای ساختاریافته `privacy_policy_url` / `privacy_policy_content` /
 `terms_of_service_url` / `terms_of_service_content` (نفر = بدون تغییر؛ محتوا یک نقشه `{"zh-CN":"...","en":"..."}` است).
- **صفحات قابل دسترس مستقل** (ارائه شده توسط سرور، آسان برای خزنده های تأیید Google؛ پشتیبانی از `?lang=xx`):
  - دامنه پنل اصلی: `GET /p/legal/privacy`، `GET /p/legal/terms`
  - دامنه پورتال اختصاصی (استقرار ریشه): `GET /legal/privacy`، `GET /legal/terms`
- **منطق نمایش**: URL یا محتوا، **URL برنده** (به صورت خارجی در یک برگه جدید باز می شود). در صورت ارائه محتوا، روی آن کلیک کنید
 کارت ورود به سیستم Markdown را در یک گفتگو (`renderMd`) با استفاده از زبان پورتال فعلی ارائه می‌کند.
- در زمان اجرا از طریق `/api/v1/public/portal-config` تحویل داده شد. جزء ورود به سیستم مصرف می کند
 `ctx.privacyUrl / termsUrl / privacyContent / termsContent / openLegal / hasLegalLinks`.
- تأیید Google OAuth مستلزم این است که این پیوندها برای عموم قابل دسترسی باشند → از پیوندهای مستقل بالا در
 صفحه رضایت؛ `docs/oauth-google-verification.md` را ببینید (شامل الگوهای خط مشی رازداری / شرایط).

## 18. مثال یکپارچه سازی

```js
// Sign in
const { data } = await fetch('/api/v1/client-auth/login', {
  method: 'POST', headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ email, password })
}).then(r => r.json());
localStorage.setItem('client_token', data.access_token);
localStorage.setItem('client_refresh_token', data.refresh_token);

// Authenticated request
const me = await fetch('/api/v1/client-auth/me', {
  headers: { Authorization: 'Bearer ' + data.access_token }
}).then(r => r.json());
```

در 401، یک بار با `POST /refresh` تماس بگیرید و دوباره امتحان کنید. اگر باز هم شکست خورد، جلسه محلی را پاک کنید
(`api()` قالب پورتال قبلاً این را پیاده سازی کرده است).
