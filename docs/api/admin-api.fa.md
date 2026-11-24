<!-- source: admin-api.md | locale: fa | updated: 2026-11-16 -->
# ادمین API

> زبان‌ها: [简体中文](admin-api.md) · [English](admin-api.en.md) · [繁體中文](admin-api.zh-TW.md) · [Русский](admin-api.ru.md) · [日本語](admin-api.ja.md) · [한국어](admin-api.ko.md) · [Deutsch](admin-api.de.md) · [Français](admin-api.fr.md) · [Español](admin-api.es.md) · [Português](admin-api.pt.md) · [Türkçe](admin-api.tr.md) · [Bahasa Indonesia](admin-api.id.md) · [ไทย](admin-api.th.md) · [Tiếng Việt](admin-api.vi.md) · [العربية](admin-api.ar.md) · فارسی

جدول مسیر مدیریت کامل API برای **توسعه پنل / اتوماسیون / یکپارچه سازها **.

- پایه URL: `https://<panel-domain>/api/v1`
- اعتبار: `Authorization: Bearer <admin_token>` (از `POST /api/v1/auth/login`)
- فرمت‌های رایج پاسخ/خطا و قراردادهای محدودیت نرخ: به [README.md](./README.fa.md) مراجعه کنید
- منبع حقیقت: `internal/api/router/router.go` (تعداد بر اساس نسخه متفاوت است؛ کد معتبر است). اگر این جدول مخالف باشد، **کد برنده می شود**

## نشانگرهای مجوز

| نشانگر | معنی |
|---|---|
| عمومی | نیازی به ورود به سیستم نیست (ممکن است گارد تعمیر و نگهداری / محدودیت نرخ همچنان اعمال شود) |
| عمومی · نرخ محدود | بدون ورود به سیستم، با محدودیت نرخ اختصاصی |
| وارد شده است | به یک رمز ورود به سیستم مدیریت نیاز دارد (`AuthRequired`) |
| اپراتور | به نقش ≥ `operator` (`RequireRole(RoleOperator)`) نیاز دارد |
| مدیر | به نقش ≥ `admin` (`RequireRole(RoleAdmin)`) نیاز دارد |
| فوق ادمین | نیاز به نقش = `super_admin` |
| مشتری (عمومی) | بدون نیاز به رمز مشتری (ورود به سیستم / ثبت نام و غیره) |
| مشتری (ورود به سیستم) | به `Authorization: Bearer <client_token>` نیاز دارد (یک توکن مشتری پورتال، نه یک توکن مدیریت) |

سطوح نقش: `viewer(10) < operator(20) < admin(30) < super_admin(40)`; نقش های ناکافی 403 `INSUFFICIENT_ROLE` را برمی گرداند.

---

## شروع سریع (نمونه های واقعی)

این نمونه ها از پاسخ های محلی واقعی می آیند. توکن ها و داده ها ویرایش می شوند.

### 1. وارد شوید و یک رمز دریافت کنید

```bash
curl -X POST https://<panel-domain>/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"your-password"}'
```

پاسخ:

```json
{
  "data": {
    "access_token": "<JWT, valid 24h>",
    "refresh_token": "<JWT, valid 7 days>",
    "user": { "id": 1, "role": "super_admin", "username": "admin" }
  }
}
```

### 2. با رمز یک نقطه پایانی کسب و کار تماس بگیرید

```bash
curl https://<panel-domain>/api/v1/clients?limit=1 \
  -H 'Authorization: Bearer <access_token>'
```

پاسخ (لیست ها از ساختار `items` + `total` استفاده می کنند):

```json
{
  "data": {
    "items": [
      {
        "id": 27,
        "email": "user@example.com",
        "status": "active",
        "quota_bytes": 10737418240,
        "used_bytes": 0,
        "plan_id": 1,
        "expires_at": null,
        "created_at": "2026-09-30T15:54:32+08:00"
      }
    ],
    "total": 22,
    "limit": 1,
    "offset": 0
  }
}
```

### 3. نمونه های پاسخ به خطا

**401 احراز هویت نشده:**

```json
{
  "code": "MISSING_TOKEN",
  "message": "missing authorization header",
  "request_id": "a02cfdf9-6be0-46be-8584-824caab34dc7",
  "can_retry": false,
  "requires_action": false
}
```

**منبع 404 یافت نشد:**

```json
{
  "code": "CLIENT_NOT_FOUND",
  "message": "client not found",
  "request_id": "099949bd-4ded-468d-a6f8-f8e798af2273",
  "can_retry": false,
  "requires_action": false
}
```

### 4. صفحه بندی

همه نقاط پایانی لیست از `limit` (اندازه صفحه، پیش‌فرض 20) و `offset` (بر اساس 0) استفاده می‌کنند و `{items, total, limit, offset}` را برمی‌گردانند. برخی از نقاط پایانی قدیمی یک آرایه خالی را برمی‌گردانند (در جدول ذکر شده است).

---

## مطالب

- [احراز هویت و امنیت ادمین](#احراز-هویت-و-امنیت-ادمین)
- [حساب های مدیریت](#حساب-های-مدیریت)
- [مدیریت مشتری](#مدیریت-مشتری)
- الگوهای [Plan](#plan-templates)
- انواع [Plan](#plan-variants)
- [قالب های پلان](#قالب-های-پلان)
- کوپن [](#coupons)
- [انواع طرح](#انواع-طرح)
- سرورهای [Node](#node-servers)
- [بسته های ترافیکی](#بسته-های-ترافیکی)
- [کوپن](#کوپن)
- [گروه های گره](#گروه-های-گره)
- اشتراک [](#subscriptions)
- فرمت های اشتراک [](#subscription-formats)
- [سرورهای نود](#سرورهای-نود)
- [گزارش حسابرسی گره](#گزارش-حسابرسی-گره)
- [گره ها (مدل قدیمی، بازنشسته)](#گره-ها-مدل-قدیمی-بازنشسته)
- پیکربندی اشتراک [ (Mihomo/Clash)](#subscription-config-mihomoclash)
- [نمای کلی منطقه](#نمای-کلی-منطقه)
- [اشتراک ها](#اشتراک-ها)
- کارت شارژ [](#recharge-cards)
- [فرمت های اشتراک](#فرمت-های-اشتراک)
- [نقاط پایانی اشتراک](#نقاط-پایانی-اشتراک)
- [دامنه ها را به اشتراک بگذارید](#دامنه-ها-را-به-اشتراک-بگذارید)
- [IP Cloudflare](#ip-cloudflare)
- [پیکربندی اشتراک (Mihomo/Clash)](#پیکربندی-اشتراک-mihomoclash)
- [سایت ها را شارژ کنید](#سایت-ها-را-شارژ-کنید)
- لیست های مجاز [IP (خلاصه)](#ip-allow-lists-summary)
- [سفارشات شارژ](#سفارشات-شارژ)
- [کارت های شارژ](#کارت-های-شارژ)
- [رویدادهای برگشت به تماس را شارژ کنید](#رویدادهای-برگشت-به-تماس-را-شارژ-کنید)
- [شارژ مجدد تماس ها](#شارژ-مجدد-تماس-ها)
- [موجودی مشتری](#موجودی-مشتری)
- [هدایا](#هدایا)
- پیکربندی [EAB](#eab-configuration)
- حساب های [ACME](#acme-accounts)
- [ CA](#built-in-ca) داخلی
- قوانین [Geo](#geo-rules)
- کتابخانه قوانین [WAF](#waf-rule-library)
- [پیکربندی ارجاع](#پیکربندی-ارجاع)
- [برداشت های ارجاعی](#برداشت-های-ارجاعی)
- [لیست های مجاز IP (خلاصه)](#لیست-های-مجاز-ip-خلاصه)
- [الزامات OAuth مشتری](#الزامات-oauth-مشتری)
- [بلیط (ادمین)](#بلیط-ادمین)
- [مقالات](#مقالات)
- [حساب های مشترک](#حساب-های-مشترک)
- [گواهینامه ها](#گواهینامه-ها)
- [کانال های گواهی](#کانال-های-گواهی)
- [پیکربندی EAB](#پیکربندی-eab)
- [حساب های ACME](#حساب-های-acme)
- [CA داخلی](#ca-داخلی)
- [قوانین جغرافیایی](#قوانین-جغرافیایی)
- [کتابخانه قوانین WAF](#کتابخانه-قوانین-waf)
- [مدیریت لاگ](#مدیریت-لاگ)
- [پشتیبان گیری](#پشتیبان-گیری)
- صفحه پورتال [](#portal-page)
- چالش های [ACME](#acme-challenges)

---

### احراز هویت و امنیت ادمین

ورود به سیستم، خروج از سیستم، تغییر رمز عبور، 2FA (TOTP)، Passkey (WebAuthn) و بلیط های یکبار مصرف برای پنل مدیریت.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| POST | `/api/v1/auth/2fa/disable` | وارد شده است | غیر فعال کردن |
| POST | `/api/v1/auth/2fa/enable` | وارد شده است | فعال کردن |
| POST | `/api/v1/auth/2fa/recovery-codes` | وارد شده است | بازیابی کدهای بازیابی |
| POST | `/api/v1/auth/2fa/setup` | وارد شده است | تنظیمات |
| GET | `/api/v1/auth/2fa/status` | وارد شده است | وضعیت |
| POST | `/api/v1/auth/change-password` | وارد شده است | رمز عبور را تغییر دهید |
| POST | `/api/v1/auth/login` | عمومی | وارد شوید |
| POST | `/api/v1/auth/login/2fa` | عمومی | ورود به سیستم با کد پویا (2FA) |
| GET | `/api/v1/auth/login/2fa-status` | عمومی | بررسی کنید که آیا حساب 2FA فعال است یا خیر |
| POST | `/api/v1/auth/logout` | وارد شده است | از سیستم خارج شوید |
| GET | `/api/v1/auth/me` | وارد شده است | کاربر فعلی وارد شده به سیستم |
| GET | `/api/v1/auth/passkey/credentials` | وارد شده است | لیست اعتبار Passkey |
| DELETE | `/api/v1/auth/passkey/credentials/:id` | وارد شده است | اعتبار Passkey را حذف کنید |
| POST | `/api/v1/auth/passkey/login/begin` | عمومی | ورود به سیستم را شروع کنید |
| POST | `/api/v1/auth/passkey/login/finish` | عمومی | ورود به سیستم کامل |
| POST | `/api/v1/auth/passkey/register/begin` | وارد شده است | شروع ثبت نام |
| POST | `/api/v1/auth/passkey/register/finish` | وارد شده است | ثبت نام کامل |
| GET | `/api/v1/auth/passkey/rpids` | مدیر | لیست مجاز RP ID |
| POST | `/api/v1/auth/passkey/rpids` | مدیر | لیست مجاز RP ID را به روز کنید |
| GET | `/api/v1/auth/passkey/status` | عمومی | وضعیت |
| POST | `/api/v1/auth/refresh` | عمومی | تازه کردن |
| POST | `/api/v1/auth/ticket` | وارد شده است | صدور بلیط یک بار کوتاه مدت |

### حساب های مدیریت

حساب های کاربری پنل مدیریت لیست نیاز به مدیر. ایجاد/به‌روزرسانی/حذف به ادمین فوق‌العاده نیاز دارد (از افزایش امتیاز جلوگیری می‌کند).

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/users` | مدیر | فهرست کنید |
| POST | `/api/v1/users` | فوق ادمین | ایجاد کنید |
| DELETE | `/api/v1/users/:id` | فوق ادمین | حذف کنید |
| PUT | `/api/v1/users/:id` | فوق ادمین | به روز رسانی |

### مدیریت مشتری

CRUD، تعلیق/رزومه، ارتقا/کاهش، تمدید و بازنشانی استفاده برای مشترکین (حساب های پورتال).

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/clients` | وارد شده است | مشتریان را فهرست کنید |
| POST | `/api/v1/clients` | اپراتور | مشتری ایجاد کنید. بدنه: `{"email":"user@example.com","password":"plaintext-password","plan_id":1,"quota_bytes":10737418240,"expires_at":"2026-12-31T00:00:00Z","remark":""}`. `plan_id` مورد نیاز است. `password` ممکن است خالی باشد (مشتریان می توانند آن را بعداً در پورتال تنظیم کنند) |
| DELETE | `/api/v1/clients/:id` | اپراتور | مشتری را حذف کنید |
| GET | `/api/v1/clients/:id` | وارد شده است | مشتری بگیر |
| PUT | `/api/v1/clients/:id` | اپراتور | مشتری را به روز کنید |
| POST | `/api/v1/clients/:id/downgrade` | اپراتور | کاهش رتبه مشتری |
| GET | `/api/v1/clients/:id/orders` | وارد شده است | لیست سفارش مشتری |
| POST | `/api/v1/clients/:id/renew` | اپراتور | تمدید مشتری |
| POST | `/api/v1/clients/:id/reset-usage` | اپراتور | بازنشانی استفاده |
| POST | `/api/v1/clients/:id/restore` | اپراتور | رزومه مشتری |
| POST | `/api/v1/clients/:id/suspend` | اپراتور | تعلیق مشتری |
| POST | `/api/v1/clients/:id/upgrade` | اپراتور | مشتری را ارتقا دهید |

### قالب های پلان

طرح‌ها (انواع، چرخه‌های صورت‌حساب، قیمت‌ها، سهمیه ترافیک) و مجوز گره در سطح الگو.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/plan-templates` | وارد شده است | لیست قالب |
| POST | `/api/v1/plan-templates` | مدیر | قالب ایجاد کنید |
| DELETE | `/api/v1/plan-templates/:id` | مدیر | حذف قالب |
| GET | `/api/v1/plan-templates/:id` | وارد شده است | جزئیات الگو |
| PUT | `/api/v1/plan-templates/:id` | مدیر | به روز رسانی قالب |
| GET | `/api/v1/plan-templates/:id/accessible-nodes` | وارد شده است | گره های موجود را دریافت کنید |
| GET | `/api/v1/plan-templates/:id/billing-node-options` | وارد شده است | گزینه های گره قابل پرداخت را دریافت کنید |
| PUT | `/api/v1/plan-templates/:id/node-groups` | مدیر | گروه های گره الگو را تنظیم کنید |
| POST | `/api/v1/plan-templates/:id/sync-accounts` | مدیر | همگام سازی حساب ها با الگو |
| GET | `/api/v1/plan-templates/:id/variants` | وارد شده است | لیست انواع |
| POST | `/api/v1/plan-templates/:id/variants` | مدیر | ایجاد نوع |
| POST | `/api/v1/plan-templates/sync-all-accounts` | مدیر | همگام سازی همه حساب ها |

### انواع طرح

به روز رسانی و حذف انواع طرح (ایجاد در زیر الگوهای طرح تو در تو است).

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| DELETE | `/api/v1/plan-variants/:id` | مدیر | حذف نوع |
| PUT | `/api/v1/plan-variants/:id` | مدیر | به روز رسانی نسخه |

### بسته های ترافیکی

بسته های ترافیکی که می توان جداگانه خریداری کرد.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/traffic-packs` | وارد شده است | لیست بسته ترافیک |
| POST | `/api/v1/traffic-packs` | مدیر | بسته ترافیکی ایجاد کنید |
| DELETE | `/api/v1/traffic-packs/:id` | مدیر | بسته ترافیک را حذف کنید |
| PUT | `/api/v1/traffic-packs/:id` | مدیر | بسته ترافیک را به روز کنید |

### کوپن

ایجاد، اعتبارسنجی و بازخرید کوپن.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/promotions` | وارد شده است | لیست کوپن ها |
| POST | `/api/v1/promotions` | مدیر | ایجاد کوپن |
| DELETE | `/api/v1/promotions/:id` | مدیر | حذف کوپن |
| PUT | `/api/v1/promotions/:id` | مدیر | به روز رسانی کوپن |
| POST | `/api/v1/promotions/validate` | وارد شده است | اعتبار کوپن |

### گروه های گره

گره ها را به گروه هایی که توسط پلان ها ارجاع داده شده اند سازماندهی کنید.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/node-groups` | وارد شده است | گروه های گره را فهرست کنید |
| POST | `/api/v1/node-groups` | مدیر | ایجاد گروه گره |
| DELETE | `/api/v1/node-groups/:id` | مدیر | حذف گروه گره |
| PUT | `/api/v1/node-groups/:id` | مدیر | به روز رسانی گروه گره |
| GET | `/api/v1/node-groups/:id/members` | وارد شده است | لیست اعضای گروه |
| POST | `/api/v1/node-groups/:id/members` | مدیر | عضو گروه را اضافه کنید |
| DELETE | `/api/v1/node-groups/:id/members/:mid` | مدیر | حذف عضو گروه |

### سرورهای نود

منابع اصلی: نود CRUD، هسته (xray/sing-box)، تحویل پیکربندی، پیکربندی ورودی/خروجی/نقطه پایانی، کنترل فرآیند، گواهی‌ها، WAF، محدودیت‌های نرخ، پرش پورت، پراکسی‌های زنجیره‌ای، آینه‌ای/اولیه-پشتیبان، کنترل زمان اجرای xray. بسیاری از مسیرهای فرعی، گروه بندی شده بر اساس خانواده ویژگی.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/node-servers` | مدیر | فهرست کنید |
| POST | `/api/v1/node-servers` | مدیر | ایجاد کنید |
| DELETE | `/api/v1/node-servers/:id` | مدیر | حذف کنید |
| GET | `/api/v1/node-servers/:id` | مدیر | جزئیات |
| PUT | `/api/v1/node-servers/:id` | مدیر | به روز رسانی |
| GET | `/api/v1/node-servers/:id/binaries` | مدیر | لیست هسته نصب شده |
| POST | `/api/v1/node-servers/:id/binaries/ech-generate` | مدیر | جفت کلید ECH ایجاد کنید |
| POST | `/api/v1/node-servers/:id/binaries/install` | مدیر | نصب/ارتقا هسته |
| GET | `/api/v1/node-servers/:id/binaries/interfaces` | مدیر | لیست رابط شبکه |
| POST | `/api/v1/node-servers/:id/binaries/mldsa65-key` | مدیر | کلید MLDSA65 را ایجاد کنید |
| POST | `/api/v1/node-servers/:id/binaries/reality-key` | مدیر | جفت کلید REALITY ایجاد کنید |
| POST | `/api/v1/node-servers/:id/binaries/reality-public-key` | مدیر | کلید عمومی را از کلید خصوصی REALITY استخراج کنید |
| GET | `/api/v1/node-servers/:id/binaries/releases` | مدیر | نسخه های موجود |
| POST | `/api/v1/node-servers/:id/binaries/vlessenc` | مدیر | پارامترهای رمزگذاری VLESS را ایجاد کنید |
| POST | `/api/v1/node-servers/:id/binaries/wg-key` | مدیر | کلید WireGuard را ایجاد کنید |
| POST | `/api/v1/node-servers/:id/binaries/x25519-key` | مدیر | جفت کلید X25519 ایجاد کنید |
| GET | `/api/v1/node-servers/:id/bootstrap-cmd` | مدیر | دستور Agent bootstrap را دریافت کنید |
| POST | `/api/v1/node-servers/:id/bootstrap-token` | مدیر | توکن بوت استرپ تولید کنید |
| POST | `/api/v1/node-servers/:id/caddy/batch-update-log-enabled` | مدیر | سوئیچ های گزارش به روز رسانی انبوه |
| POST | `/api/v1/node-servers/:id/caddy/batch-update-ports` | مدیر | پورت های به روز رسانی انبوه |
| POST | `/api/v1/node-servers/:id/caddy/generate-config` | مدیر | ایجاد پیکربندی |
| DELETE | `/api/v1/node-servers/:id/caddy/l4-rules` | مدیر | تمام قوانین L4 را حذف کنید |
| POST | `/api/v1/node-servers/:id/caddy/save-and-apply` | مدیر | ذخیره و اعمال کنید |
| GET | `/api/v1/node-servers/:id/caddy/sites` | مدیر | لیست سایت |
| POST | `/api/v1/node-servers/:id/caddy/sites` | مدیر | سایت ایجاد کنید |
| POST | `/api/v1/node-servers/:id/caddy/import` | مدیر | سایت های وارداتی |
| POST | `/api/v1/node-servers/:id/caddy/preview-site` | مدیر | پیش نمایش رندر شده سایت Caddy |
| DELETE | `/api/v1/node-servers/:id/caddy/sites/:siteId` | مدیر | حذف سایت |
| PUT | `/api/v1/node-servers/:id/caddy/sites/:siteId` | مدیر | سایت را به روز کنید |
| DELETE | `/api/v1/node-servers/:id/caddy/sni-rules` | مدیر | تمام قوانین SNI را حذف کنید |
| POST | `/api/v1/node-servers/:id/caddy/sni-rules/toggle` | مدیر | قوانین SNI را تغییر دهید |
| GET | `/api/v1/node-servers/:id/caddy/sni-settings` | مدیر | تنظیمات SNI |
| POST | `/api/v1/node-servers/:id/caddy/sni-settings` | مدیر | تنظیمات SNI را ذخیره کنید |
| GET | `/api/v1/node-servers/:id/certificates` | مدیر | لیست گواهی گره |
| DELETE | `/api/v1/node-servers/:id/certificates/:cid` | مدیر | گواهی لغو پیوند |
| POST | `/api/v1/node-servers/:id/certificates/:cid` | مدیر | گواهی صحافی |
| GET | `/api/v1/node-servers/:id/chain-config` | مدیر | ایجاد پیکربندی زنجیره ای |
| GET | `/api/v1/node-servers/:id/chain-health` | مدیر | وضعیت سلامت زنجیره |
| POST | `/api/v1/node-servers/:id/control` | فوق ادمین | هسته گره کنترل بدنه: `{"core_type":"xray","action":"start"}` (عمل = شروع/توقف/راه اندازی مجدد). `core_type` می تواند `xray`/`singbox` باشد |
| GET | `/api/v1/node-servers/:id/degradation-status` | مدیر | وضعیت تنزل یافته |
| POST | `/api/v1/node-servers/:id/distribute-cert` | مدیر | توزیع گواهی |
| GET | `/api/v1/node-servers/:id/endpoint-configs` | مدیر | فهرست کنید |
| POST | `/api/v1/node-servers/:id/endpoint-configs` | مدیر | ایجاد کنید |
| DELETE | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | مدیر | حذف کنید |
| GET | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | مدیر | جزئیات |
| PUT | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | مدیر | به روز رسانی |
| POST | `/api/v1/node-servers/:id/failover` | مدیر | ماشه شکست برای این گره |
| GET | `/api/v1/node-servers/:id/failover-status` | مدیر | وضعیت Failover |
| POST | `/api/v1/node-servers/:id/force-reconnect` | مدیر | اتصال مجدد اجباری |
| GET | `/api/v1/node-servers/:id/free-port` | مدیر | درخواست پورت رایگان |
| POST | `/api/v1/node-servers/:id/geo-rules/push` | مدیر | به این گره فشار دهید |
| GET | `/api/v1/node-servers/:id/firewall/status` | مدیر | تشخیص فایروال (firewalld/ufw/nft/iptables) |
| POST | `/api/v1/node-servers/:id/firewall/allow-port` | مدیر | اجازه دادن به پورت شنیداری ورودی (پشتیبانی از چند دامنه) |
| POST | `/api/v1/node-servers/:id/firewall/remove-port` | مدیر | لغو مجوز پورت ورودی (از جمله موارد تکراری) |
| GET | `/api/v1/node-servers/:id/inbound-configs` | مدیر | فهرست کنید |
| POST | `/api/v1/node-servers/:id/inbound-configs` | مدیر | ایجاد کنید |
| DELETE | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | مدیر | حذف کنید |
| GET | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | مدیر | جزئیات |
| PUT | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | مدیر | به روز رسانی |
| GET | `/api/v1/node-servers/:id/inbound-configs/:cfgId/share-links` | مدیر | پیوندها و قطعات را به اشتراک بگذارید |
| POST | `/api/v1/node-servers/:id/inbound-configs/:cfgId/users/:email/pause` | مدیر | این کاربر را تعلیق کنید |
| POST | `/api/v1/node-servers/:id/inbound-configs/:cfgId/users/:email/resume` | مدیر | این کاربر را بازیابی کنید |
| POST | `/api/v1/node-servers/:id/inbound-configs/reset-all-traffic` | مدیر | تمام ترافیک ورودی را بازنشانی کنید |
| GET | `/api/v1/node-servers/:id/install-script` | مدیر | دریافت اسکریپت نصب عامل |
| GET | `/api/v1/node-servers/:id/metrics-history` | مدیر | تاریخچه متریک را دریافت کنید |
| GET | `/api/v1/node-servers/:id/mirror-status` | مدیر | وضعیت آینه |
| POST | `/api/v1/node-servers/:id/nginx/batch-update-ports` | مدیر | پورت های به روز رسانی انبوه |
| POST | `/api/v1/node-servers/:id/nginx/generate-config` | مدیر | ایجاد پیکربندی |
| DELETE | `/api/v1/node-servers/:id/nginx/l4-rules` | مدیر | تمام قوانین L4 را حذف کنید |
| POST | `/api/v1/node-servers/:id/nginx/save-and-apply` | مدیر | ذخیره و اعمال کنید |
| GET | `/api/v1/node-servers/:id/nginx/settings` | مدیر | تنظیمات را بخوانید |
| POST | `/api/v1/node-servers/:id/nginx/settings` | مدیر | تنظیمات را ذخیره کنید |
| GET | `/api/v1/node-servers/:id/nginx/sites` | مدیر | لیست سایت |
| POST | `/api/v1/node-servers/:id/nginx/sites` | مدیر | سایت ایجاد کنید |
| POST | `/api/v1/node-servers/:id/nginx/import` | مدیر | سایت های وارداتی |
| POST | `/api/v1/node-servers/:id/nginx/preview-site` | مدیر | پیش نمایش رندر شده سایت Nginx |
| POST | `/api/v1/node-servers/:id/nginx/ssl/generate` | مدیر | فایل های گواهی SSL را تولید کنید |
| DELETE | `/api/v1/node-servers/:id/nginx/sites/:siteId` | مدیر | حذف سایت |
| PUT | `/api/v1/node-servers/:id/nginx/sites/:siteId` | مدیر | سایت را به روز کنید |
| DELETE | `/api/v1/node-servers/:id/nginx/sni-rules` | مدیر | تمام قوانین SNI را حذف کنید |
| POST | `/api/v1/node-servers/:id/nginx/sni-rules/toggle` | مدیر | قوانین SNI را تغییر دهید |
| PUT | `/api/v1/node-servers/:id/node-type` | مدیر | به روز رسانی نوع گره |
| GET | `/api/v1/node-servers/:id/online-users` | مدیر | کاربران آنلاین |
| GET | `/api/v1/node-servers/:id/outbound-configs` | مدیر | فهرست کنید |
| POST | `/api/v1/node-servers/:id/outbound-configs` | مدیر | ایجاد کنید |
| DELETE | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | مدیر | حذف کنید |
| GET | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | مدیر | جزئیات |
| PUT | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | مدیر | به روز رسانی |
| PUT | `/api/v1/node-servers/:id/outbound-configs/reorder` | مدیر | مرتب کردن |
| GET | `/api/v1/node-servers/:id/port-hop/rules` | مدیر | پیش نمایش قانون بندر پرش |
| GET | `/api/v1/node-servers/:id/protocols` | مدیر | لیست پروتکل |
| GET | `/api/v1/node-servers/:id/protocols/:coreType` | مدیر | پیکربندی پروتکل |
| PUT | `/api/v1/node-servers/:id/protocols/:coreType` | مدیر | پیکربندی پروتکل را ذخیره کنید |
| GET | `/api/v1/node-servers/:id/protocols/:coreType/config-file` | مدیر | فایل پیکربندی هسته را بخوانید |
| GET | `/api/v1/node-servers/:id/protocols/:coreType/section/:section` | مدیر | بخش پروتکل را بخوانید |
| PUT | `/api/v1/node-servers/:id/protocols/:coreType/section/:section` | مدیر | بخش پروتکل را ذخیره کنید |
| POST | `/api/v1/node-servers/:id/protocols/:coreType/validate` | مدیر | اعتبارسنجی پیکربندی (اجرای خشک هسته واقعی) |
| GET | `/api/v1/node-servers/:id/proxy-status` | مدیر | وضعیت پراکسی راه دور |
| POST | `/api/v1/node-servers/:id/proxy/batch-update-ports` | مدیر | پورت های به روز رسانی انبوه |
| GET | `/api/v1/node-servers/:id/proxy/config` | مدیر | پیکربندی را بخوانید |
| PUT | `/api/v1/node-servers/:id/proxy/config` | مدیر | پیکربندی را به روز کنید |
| POST | `/api/v1/node-servers/:id/proxy/control` | مدیر | کنترل ثبت پروکسی زنجیره ای |
| POST | `/api/v1/node-servers/:id/proxy/download` | مدیر | دانلود نسخه هسته پروکسی |
| POST | `/api/v1/node-servers/:id/proxy/generate-config` | مدیر | ایجاد پیکربندی |
| POST | `/api/v1/node-servers/:id/proxy/guardian` | مدیر | پیکربندی سرپرست را به روز کنید |
| POST | `/api/v1/node-servers/:id/proxy/install` | مدیر | نصب پروکسی |
| DELETE | `/api/v1/node-servers/:id/proxy/l4-rules` | مدیر | تمام قوانین L4 را حذف کنید |
| GET | `/api/v1/node-servers/:id/proxy/l4-settings` | مدیر | تنظیمات L4 |
| POST | `/api/v1/node-servers/:id/proxy/l4-settings` | مدیر | تنظیمات L4 را ذخیره کنید |
| GET | `/api/v1/node-servers/:id/proxy/log-settings` | مدیر | تنظیمات گزارش |
| POST | `/api/v1/node-servers/:id/proxy/log-settings` | مدیر | تنظیمات گزارش را ذخیره کنید |
| GET | `/api/v1/node-servers/:id/proxy/nginx-settings` | مدیر | تنظیمات nginx |
| POST | `/api/v1/node-servers/:id/proxy/nginx-settings` | مدیر | تنظیمات nginx را ذخیره کنید |
| GET | `/api/v1/node-servers/:id/proxy/releases` | مدیر | نسخه های موجود |
| POST | `/api/v1/node-servers/:id/proxy/reload` | مدیر | بارگیری مجدد پیکربندی |
| POST | `/api/v1/node-servers/:id/proxy/restart` | مدیر | راه اندازی مجدد |
| GET | `/api/v1/node-servers/:id/proxy/sites` | مدیر | لیست سایت |
| POST | `/api/v1/node-servers/:id/proxy/sites` | مدیر | سایت ایجاد کنید |
| DELETE | `/api/v1/node-servers/:id/proxy/sites/:siteId` | مدیر | حذف سایت |
| PUT | `/api/v1/node-servers/:id/proxy/sites/:siteId` | مدیر | سایت را به روز کنید |
| DELETE | `/api/v1/node-servers/:id/proxy/sni-rules` | مدیر | تمام قوانین SNI را حذف کنید |
| POST | `/api/v1/node-servers/:id/proxy/sni-rules/toggle` | مدیر | قوانین SNI را تغییر دهید |
| GET | `/api/v1/node-servers/:id/proxy/sni-settings` | مدیر | تنظیمات SNI |
| POST | `/api/v1/node-servers/:id/proxy/sni-settings` | مدیر | تنظیمات SNI را ذخیره کنید |
| POST | `/api/v1/node-servers/:id/proxy/start` | مدیر | شروع کنید |
| GET | `/api/v1/node-servers/:id/proxy/status` | مدیر | وضعیت فرآیند پروکسی |
| POST | `/api/v1/node-servers/:id/proxy/stop` | مدیر | توقف کنید |
| POST | `/api/v1/node-servers/:id/proxy/sync` | مدیر | همگام سازی پیکربندی پروکسی |
| GET | `/api/v1/node-servers/:id/rate-limit` | مدیر | پیکربندی را بخوانید |
| PUT | `/api/v1/node-servers/:id/rate-limit` | مدیر | پیکربندی را به روز کنید |
| GET | `/api/v1/node-servers/:id/rate-limit/status` | مدیر | وضعیت |
| GET | `/api/v1/node-servers/:id/recover-mode` | مدیر | حالت بازیابی گره را بخوانید |
| PUT | `/api/v1/node-servers/:id/recover-mode` | مدیر | حالت بازیابی گره را تنظیم کنید |
| POST | `/api/v1/node-servers/:id/revoke-cert` | مدیر | ابطال گواهی نماینده |
| GET | `/api/v1/node-servers/:id/services` | مدیر | فهرست کنید |
| POST | `/api/v1/node-servers/:id/services` | مدیر | ایجاد کنید |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId` | مدیر | حذف کنید |
| GET | `/api/v1/node-servers/:id/services/:serviceId` | مدیر | جزئیات |
| PUT | `/api/v1/node-servers/:id/services/:serviceId` | مدیر | به روز رسانی |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId/crash-logs` | مدیر | لاگ های خرابی را پاک کنید |
| GET | `/api/v1/node-servers/:id/services/:serviceId/crash-logs` | مدیر | لیست گزارش خرابی |
| GET | `/api/v1/node-servers/:id/services/:serviceId/crash-logs/:logId` | مدیر | جزئیات گزارش خرابی |
| GET | `/api/v1/node-servers/:id/services/:serviceId/endpoints` | مدیر | لیست نقطه پایانی |
| POST | `/api/v1/node-servers/:id/services/:serviceId/endpoints` | مدیر | نقطه پایانی ایجاد کنید |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId/endpoints/:endpointId` | مدیر | نقطه پایانی را حذف کنید |
| PUT | `/api/v1/node-servers/:id/services/:serviceId/endpoints/:endpointId` | مدیر | نقطه پایانی را به روز کنید |
| PUT | `/api/v1/node-servers/:id/services/:serviceId/guardian` | مدیر | پیکربندی سرپرست را به روز کنید |
| GET | `/api/v1/node-servers/:id/services/:serviceId/process-status` | مدیر | وضعیت پردازش را دریافت کنید |
| POST | `/api/v1/node-servers/:id/services/:serviceId/restart` | مدیر | راه اندازی مجدد |
| POST | `/api/v1/node-servers/:id/services/:serviceId/start` | مدیر | شروع کنید |
| POST | `/api/v1/node-servers/:id/services/:serviceId/stop` | مدیر | توقف کنید |
| GET | `/api/v1/node-servers/:id/services/process-status` | مدیر | همه وضعیت های فرآیند |
| GET | `/api/v1/node-servers/:id/share-domains` | مدیر | دامنه ها را برای این گره به اشتراک بگذارید |
| POST | `/api/v1/node-servers/:id/share-domains` | مدیر | ایجاد کنید |
| DELETE | `/api/v1/node-servers/:id/share-domains/:domainId` | مدیر | حذف کنید |
| PUT | `/api/v1/node-servers/:id/share-domains/:domainId` | مدیر | به روز رسانی |
| DELETE | `/api/v1/node-servers/:id/share-domains/all` | مدیر | پاک کردن همه |
| POST | `/api/v1/node-servers/:id/singbox/control/firewall/apply` | مدیر | قوانین فایروال پورت پرش را اعمال کنید |
| POST | `/api/v1/node-servers/:id/singbox/control/firewall/remove` | مدیر | قوانین فایروال port-hopping را حذف کنید |
| GET | `/api/v1/node-servers/:id/singbox/connections` | مدیر | اتصالات زنده sing-box (فقط clash_api، گره محلی) |
| POST | `/api/v1/node-servers/:id/speed-test` | مدیر | تست سرعت گره |
| GET | `/api/v1/node-servers/:id/status` | مدیر | وضعیت گره |
| POST | `/api/v1/node-servers/:id/sync` | مدیر | همگام سازی پیکربندی گره (تولید و فشار دادن) |
| POST | `/api/v1/node-servers/:id/standby-sync` | ادمین | پیکربندی اولیه را با شریک آماده به کار (primary_backup) |
| POST | `/api/v1/node-servers/:id/sync-data` | فوق ادمین | فشار دادن داده‌های پانل به گره‌ها (همگام‌سازی کامل کاربران/اشتراک‌ها/طرح‌ها) |
| POST | `/api/v1/node-servers/:id/sync-mirror` | مدیر | همگام سازی گره آینه ای |
| GET | `/api/v1/node-servers/:id/tags` | مدیر | لیست برچسب |
| POST | `/api/v1/node-servers/:id/tags` | مدیر | برچسب اضافه کنید |
| DELETE | `/api/v1/node-servers/:id/tags/:tagKey` | مدیر | حذف برچسب |
| GET | `/api/v1/node-servers/:id/traffic-history` | مدیر | تاریخچه ترافیک |
| POST | `/api/v1/node-servers/:id/traffic-resume` | مدیر | بازگرداندن ترافیک معلق به دلیل بیش از حد |
| GET | `/api/v1/node-servers/:id/traffic-usage` | مدیر | استفاده از ترافیک |
| POST | `/api/v1/node-servers/:id/update-agent` | مدیر | عامل ارتقاء |
| GET | `/api/v1/node-servers/:id/used-server-names` | مدیر | لیست در حال استفاده REALITY SNI |
| GET | `/api/v1/node-servers/:id/waf/config` | مدیر | پیکربندی را بخوانید |
| PUT | `/api/v1/node-servers/:id/waf/config` | مدیر | ذخیره تنظیمات |
| POST | `/api/v1/node-servers/:id/waf/disable-domain` | مدیر | محافظت از دامنه را غیرفعال کنید |
| GET | `/api/v1/node-servers/:id/waf/domain-config` | مدیر | لیست پیکربندی دامنه |
| PUT | `/api/v1/node-servers/:id/waf/domain-config` | مدیر | ذخیره انبوه تنظیمات دامنه |
| DELETE | `/api/v1/node-servers/:id/waf/domain-config/:domain` | مدیر | تنظیمات دامنه را حذف کنید |
| GET | `/api/v1/node-servers/:id/waf/domain-config/:domain` | مدیر | جزئیات پیکربندی دامنه |
| PUT | `/api/v1/node-servers/:id/waf/domain-config/:domain` | مدیر | ذخیره تنظیمات دامنه |
| GET | `/api/v1/node-servers/:id/waf/domain-rules` | مدیر | دایرکتوری قوانین دامنه |
| GET | `/api/v1/node-servers/:id/waf/domain-rules/:domain/:name` | مدیر | فایل قانون دامنه را بخوانید |
| PUT | `/api/v1/node-servers/:id/waf/domain-rules/:domain/:name` | مدیر | فایل قانون دامنه را ذخیره کنید |
| GET | `/api/v1/node-servers/:id/waf/domain-status` | مدیر | وضعیت حفاظت از دامنه |
| POST | `/api/v1/node-servers/:id/waf/enable-domain` | مدیر | محافظت از دامنه را فعال کنید |
| POST | `/api/v1/node-servers/:id/waf/push` | مدیر | به این گره فشار دهید |
| GET | `/api/v1/node-servers/:id/waf/rules` | مدیر | لیست قوانین گره WAF |
| DELETE | `/api/v1/node-servers/:id/waf/rules/:name` | مدیر | قوانین گره WAF را حذف کنید |
| GET | `/api/v1/node-servers/:id/waf/rules/:name` | مدیر | جزئیات قانون گره WAF |
| PUT | `/api/v1/node-servers/:id/waf/rules/:name` | مدیر | قوانین گره WAF را ذخیره کنید |
| GET | `/api/v1/node-servers/:id/waf/status` | مدیر | وضعیت گره WAF |
| PUT | `/api/v1/node-servers/:id/waf/status` | مدیر | گره WAF را تغییر دهید |
| POST | `/api/v1/node-servers/:id/waf/sync` | مدیر | با این گره همگام سازی کنید |
| GET | `/api/v1/node-servers/:id/warp` | مدیر | فهرست کنید |
| DELETE | `/api/v1/node-servers/:id/warp/:warpId` | مدیر | حذف کنید |
| GET | `/api/v1/node-servers/:id/warp/config` | مدیر | پیکربندی را بخوانید |
| POST | `/api/v1/node-servers/:id/warp/generate-config` | مدیر | ایجاد پیکربندی |
| POST | `/api/v1/node-servers/:id/warp/register` | مدیر | ثبت نام کنید |
| POST | `/api/v1/node-servers/:id/warp/masque/register` | مدیر | ثبت نمایه WARP MASQUE (مرحله 1 از 2) |
| POST | `/api/v1/node-servers/:id/warp/:warpId/masque-outbound` | مدیر | یک sing-box MASQUE خروجی از نمایه WARP ایجاد کنید |
| GET | `/api/v1/node-servers/:id/warp/status` | مدیر | وضعیت |
| GET | `/api/v1/node-servers/:id/xray/control/balancer/:tag` | مدیر | اطلاعات متعادل کننده را دریافت کنید |
| POST | `/api/v1/node-servers/:id/xray/control/balancer/:tag` | مدیر | هدف متعادل کننده را تنظیم کنید |
| POST | `/api/v1/node-servers/:id/xray/control/firewall/apply` | مدیر | قوانین فایروال پورت پرش را اعمال کنید |
| POST | `/api/v1/node-servers/:id/xray/control/firewall/remove` | مدیر | قوانین فایروال port-hopping را حذف کنید |
| POST | `/api/v1/node-servers/:id/xray/control/inbound` | مدیر | ورودی را اضافه کنید |
| DELETE | `/api/v1/node-servers/:id/xray/control/inbound/:tag` | مدیر | ورودی را حذف کنید |
| POST | `/api/v1/node-servers/:id/xray/control/inbound/:tag/user` | مدیر | کاربر اضافه کنید |
| DELETE | `/api/v1/node-servers/:id/xray/control/inbound/:tag/user/:email` | مدیر | حذف کاربر |
| POST | `/api/v1/node-servers/:id/xray/control/outbound` | مدیر | خروجی را اضافه کنید |
| DELETE | `/api/v1/node-servers/:id/xray/control/outbound/:tag` | مدیر | خروجی را حذف کنید |
| POST | `/api/v1/node-servers/:id/xray/control/routing` | مدیر | درخواست مسیر |
| POST | `/api/v1/node-servers/:id/xray/control/routing/rule` | مدیر | قانون مسیریابی را اضافه کنید |
| DELETE | `/api/v1/node-servers/:id/xray/control/routing/rule/:tag` | مدیر | قانون مسیریابی را حذف کنید |
| GET | `/api/v1/node-servers/:id/xray/control/routing/rules` | مدیر | قوانین مسیریابی را فهرست کنید |
| POST | `/api/v1/node-servers/:id/xray/control/sync` | مدیر | همگام سازی پیکربندی xray را به صورت دستی فعال کنید |
| GET | `/api/v1/node-servers/:id/xray/stats` | مدیر | آمار بگیرید |
| POST | `/api/v1/node-servers/:id/xray/stats/reset/:email` | مدیر | بازنشانی آمار کاربران |
| GET | `/api/v1/node-servers/:id/xray/stats/users` | مدیر | دریافت آمار کاربران |
| GET | `/api/v1/node-servers/billable` | مدیر | لیست گره های قابل پرداخت |
| GET | `/api/v1/node-servers/ca-cert` | مدیر | گواهی CA |
| GET | `/api/v1/node-servers/ca-status` | مدیر | وضعیت CA |
| GET | `/api/v1/node-servers/dashboard` | مدیر | وضعیت داشبورد |
| POST | `/api/v1/node-servers/distribute-cert` | مدیر | توزیع گواهی در تمام گره ها |
| POST | `/api/v1/node-servers/failover-check` | مدیر | ماشه بررسی شکست |
| GET | `/api/v1/node-servers/metrics/aggregated` | مدیر | معیارهای جمع آوری شده |
| POST | `/api/v1/node-servers/migrate-cert` | مدیر | گواهی مهاجرت |
| GET | `/api/v1/node-servers/online-summary` | مدیر | خلاصه آنلاین |
| GET | `/api/v1/node-servers/online-users` | مدیر | همه کاربران آنلاین |
| GET | `/api/v1/node-servers/pending-agent-updates` | مدیر | عوامل در انتظار ارتقاء |
| POST | `/api/v1/node-servers/recommend-region` | مدیر | مناطق توصیه شده |
| POST | `/api/v1/node-servers/select-nodes` | مدیر | گره ها را برای اشتراک انتخاب کنید |
| POST | `/api/v1/node-servers/speed-test-all` | مدیر | تست سرعت همه گره ها |
| GET | `/api/v1/node-servers/speed-test-results` | مدیر | نتایج تست سرعت |
| GET | `/api/v1/node-servers/traffic/stats` | مدیر | آمار ترافیک |
| POST | `/api/v1/node-servers/update-all-agents` | مدیر | همه نمایندگان را ارتقا دهید |
| DELETE | `/api/v1/node-servers/users/:client_id` | مدیر | همگام سازی-حذف مشتری |
| GET | `/api/v1/node-servers/users/retry-queue` | مدیر | وضعیت صف را دوباره امتحان کنید |
| POST | `/api/v1/node-servers/users/retry-queue/process` | مدیر | صف تلاش مجدد را پردازش کنید |
| POST | `/api/v1/node-servers/users/sync` | مدیر | همگام سازی یک مشتری |
| POST | `/api/v1/node-servers/validate-chain-config` | مدیر | اعتبار سنجی تنظیمات زنجیره ای |

### گزارش حسابرسی گره

ممیزی عملیات سمت گره و صادرات.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/node-audit-logs` | مدیر | گزارش حسابرسی گره |
| GET | `/api/v1/node-audit-logs/export` | مدیر | گزارش های حسابرسی گره را صادر کنید |

### گره ها (مدل قدیمی، بازنشسته)

> `/api/v1/nodes` CRUD در 20/10/2026 بازنشسته شد: گره ها به طور یکنواخت از طریق `/api/v1/node-servers` مدیریت می شوند.
> درخواست های کاربر آنلاین در `/api/v1/node-servers/{online-users,online-summary,:id/online-users}` باقی می ماند.

### نمای کلی منطقه

نمای گره بر اساس منطقه جمع شده است.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/regions/:region/servers` | وارد شده است | لیست گره ها برای این منطقه |
| GET | `/api/v1/regions/overview` | وارد شده است | نمای کلی منطقه |

### اشتراک ها

مدیریت رکورد اشتراک: لغو، چرخش نشانه، سیاهههای مربوط به دسترسی، بازنشانی استفاده.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/subscriptions` | وارد شده است | فهرست کنید |
| POST | `/api/v1/subscriptions` | اپراتور | ایجاد کنید |
| DELETE | `/api/v1/subscriptions/:id` | اپراتور | حذف کنید |
| GET | `/api/v1/subscriptions/:id` | وارد شده است | جزئیات |
| PUT | `/api/v1/subscriptions/:id` | اپراتور | به روز رسانی |
| GET | `/api/v1/subscriptions/:id/access-logs` | وارد شده است | گزارش دسترسی |
| POST | `/api/v1/subscriptions/:id/enable` | اپراتور | فعال کردن |
| POST | `/api/v1/subscriptions/:id/reset-usage` | اپراتور | بازنشانی استفاده |
| POST | `/api/v1/subscriptions/:id/restore` | اپراتور | رزومه |
| POST | `/api/v1/subscriptions/:id/revoke` | اپراتور | لغو |
| POST | `/api/v1/subscriptions/:id/rotate` | اپراتور | چرخش |
| GET | `/api/v1/subscriptions/:id/rotations` | وارد شده است | تاریخچه چرخش |

### فرمت های اشتراک

> `/api/v1/subscription-formats` CRUD سمت مدیریت در 20/10/2026 بازنشسته شد: نقطه ورود واقعی است
> `/api/v1/mihomo/configs` (مرکز فرمت اشتراک). جدول `subscription_formats` باقی می ماند،
> فقط به عنوان داده اولیه برای نقطه پایانی عمومی `/api/v1/public/client-configs` استفاده می شود.

### نقاط پایانی اشتراک

دامنه های اشتراک اختصاصی (پراکسی معکوس به مسیر `/sub` پانل).

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/subscription-endpoints` | وارد شده است | فهرست کنید |
| POST | `/api/v1/subscription-endpoints` | مدیر | ایجاد کنید |
| DELETE | `/api/v1/subscription-endpoints/:id` | مدیر | حذف کنید |
| GET | `/api/v1/subscription-endpoints/:id` | وارد شده است | جزئیات |
| PUT | `/api/v1/subscription-endpoints/:id` | مدیر | به روز رسانی |

### دامنه ها را به اشتراک بگذارید

دسترسی به استخر دامنه/IP مورد استفاده در پیوندهای مشتری.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/share-domains` | وارد شده است | فهرست کنید |
| POST | `/api/v1/share-domains` | مدیر | ایجاد کنید |
| DELETE | `/api/v1/share-domains/:id` | مدیر | حذف کنید |
| PUT | `/api/v1/share-domains/:id` | مدیر | به روز رسانی |
| DELETE | `/api/v1/share-domains/all` | مدیر | پاک کردن همه |
| POST | `/api/v1/share-domains/seed` | مدیر | داده های پیش فرض را راه اندازی کنید |
| GET | `/api/v1/share-domains/dns-providers` | وارد شده | جدول قابلیت نوشتن DNS برای هر ارائه دهنده (اجرا شده / تأیید شده) |
`PUT /api/v1/share-domains/:id` همچنین `dns_channel_id`، `dns_record_name` و `dns_proxied` را می پذیرد: در یک خطای اولیه/آماده به کار یا سوئیچ برگشتی، رکورد ورودی A به سمت اولیه جدید نشان داده می شود. فقط ارائه‌دهندگانی که دارای برنامه‌نویس DNS پیاده‌سازی شده باشند، این فیلدها را می‌پذیرند (`cloudflare` تأیید شده؛ alidns / DNSPod / GoDaddy / Namecheap / Route53 / Google / Huawei Cloud اجرا شده اما تأیید نشده است). برای جدول قابلیت `GET /api/v1/share-domains/dns-providers` را ببینید.


### IP Cloudflare

Cloudflare CDN مدیریت استخر IP و منبع.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| DELETE | `/api/v1/cfip/entries` | مدیر | ورودی ها را پاک کنید |
| GET | `/api/v1/cfip/entries` | وارد شده است | لیست ورودی |
| POST | `/api/v1/cfip/entries` | مدیر | افزودن ورودی |
| DELETE | `/api/v1/cfip/entries/:id` | مدیر | حذف ورودی |
| POST | `/api/v1/cfip/fetch-all` | مدیر | همه منابع را واکشی کنید |
| GET | `/api/v1/cfip/probe-config` | وارد شده است | پیکربندی پروب IP ترجیحی CF (گره های ورودی + شماره گیرهای نصب شده) |
| PUT | `/api/v1/cfip/probe-config` | مدیر | ذخیره فاصله پروب / محدودیت تاخیر / گره ورودی / شماره گیر |
| POST | `/api/v1/cfip/check` | مدیر | اکنون همه دامنه ها/IP های ترجیحی را بررسی کنید |
| POST | `/api/v1/cfip/seed` | مدیر | داده های پیش فرض را راه اندازی کنید |
| GET | `/api/v1/cfip/sources` | وارد شده است | فهرست منابع داده |
| POST | `/api/v1/cfip/sources` | مدیر | ایجاد منبع داده |
| DELETE | `/api/v1/cfip/sources/:id` | مدیر | حذف منبع داده |
| PUT | `/api/v1/cfip/sources/:id` | مدیر | منبع داده را به روز کنید |
| POST | `/api/v1/cfip/sources/:id/fetch` | مدیر | این منبع را واکشی کنید |

### پیکربندی اشتراک (Mihomo/Clash)

پیکربندی الگوی اشتراک: گروه ها، قوانین، ارائه دهندگان قانون، پیش نمایش YAML.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/mihomo/configs` | وارد شده است | فهرست کنید |
| POST | `/api/v1/mihomo/configs` | مدیر | ایجاد کنید |
| DELETE | `/api/v1/mihomo/configs/:id` | مدیر | حذف کنید |
| GET | `/api/v1/mihomo/configs/:id` | وارد شده است | جزئیات |
| PUT | `/api/v1/mihomo/configs/:id` | مدیر | به روز رسانی |
| GET | `/api/v1/mihomo/configs/:id/rules` | وارد شده است | لیست قوانین |
| POST | `/api/v1/mihomo/configs/:id/rules` | مدیر | قانون ایجاد کنید |
| PUT | `/api/v1/mihomo/configs/:id/rules/order` | مدیر | قوانین را دوباره سفارش دهید |
| PUT | `/api/v1/mihomo/rules/:id` | مدیر | قانون به روز رسانی |
| DELETE | `/api/v1/mihomo/rules/:id` | مدیر | حذف قانون |

### سایت ها را شارژ کنید

ادغام سایت پرداخت شخص ثالث: کلیدهای امضا، فایل های تأیید، تأیید دامنه، اسناد یکپارچه سازی.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/charge-sites` | مدیر | لیست سایت |
| POST | `/api/v1/charge-sites` | مدیر | سایت ایجاد کنید |
| DELETE | `/api/v1/charge-sites/:id` | مدیر | حذف سایت |
| GET | `/api/v1/charge-sites/:id` | مدیر | جزئیات سایت |
| PUT | `/api/v1/charge-sites/:id` | مدیر | سایت را به روز کنید |
| POST | `/api/v1/charge-sites/:id/clear-prev-secret` | مدیر | کلید قدیمی را پاک کنید |
| POST | `/api/v1/charge-sites/:id/generate-verify` | مدیر | ایجاد فایل تایید |
| POST | `/api/v1/charge-sites/:id/generate-verify-file` | مدیر | ایجاد فایل تایید |
| GET | `/api/v1/charge-sites/:id/integration-doc` | مدیر | سند ادغام را دریافت کنید |
| POST | `/api/v1/charge-sites/:id/rotate-secret` | مدیر | چرخاندن کلید امضا |
| POST | `/api/v1/charge-sites/:id/verify-dns` | مدیر | دامنه DNS را تأیید کنید |
| POST | `/api/v1/charge-sites/:id/verify-domain` | مدیر | چالش دامنه HTTP را تأیید کنید |
| POST | `/api/v1/charge-sites/:id/verify-domain-dns` | مدیر | دامنه DNS را تأیید کنید |
| POST | `/api/v1/charge-sites/:id/verify-domain-http` | مدیر | چالش دامنه HTTP را تأیید کنید |

### سفارشات شارژ

درخواست شارژ مجدد، بازپرداخت، صادرات CSV.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/charge-orders` | مدیر | لیست سفارش |
| GET | `/api/v1/charge-orders/:id` | مدیر | جزئیات سفارش |
| POST | `/api/v1/charge-orders/:id/refund` | مدیر | بازپرداخت |
| GET | `/api/v1/charge-orders/export-csv` | مدیر | صادرات سفارشات شارژ CSV |

### کارت های شارژ

تولید کارت انبوه، سوابق استفاده، صادرات CSV.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/charge-cards` | مدیر | لیست کارت |
| POST | `/api/v1/charge-cards` | مدیر | کارت ایجاد کنید |
| DELETE | `/api/v1/charge-cards/:id` | مدیر | حذف کارت |
| GET | `/api/v1/charge-cards/:id/usages` | مدیر | سوابق استفاده از کارت |
| POST | `/api/v1/charge-cards/batch` | مدیر | کارت تولید انبوه |
| GET | `/api/v1/charge-cards/export-csv` | مدیر | کارت های صادراتی CSV |

### رویدادهای برگشت به تماس را شارژ کنید

ممیزی رویداد برگشت به تماس

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/charge-callback-events` | وارد شده است | لیست رویدادهای برگشت به تماس |

### شارژ مجدد تماس ها

سعی مجدد دستی تماس های ناموفق.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/charge-callback-retries` | وارد شده است | لیست پاسخ به تماس ناموفق |
| POST | `/api/v1/charge-callback-retries/:id/retry` | مدیر | تماس مجدد را امتحان کنید |

### موجودی مشتری

نمای ادمین موجودی ها و تراکنش ها.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/client-balances` | وارد شده است | لیست موجودی مشتری |
| GET | `/api/v1/client-balances/transactions` | وارد شده است | تراکنش های موجود (ادمین) |
| GET | `/api/v1/client-balances/transactions/export-csv` | وارد شده است | تراکنش های موجودی صادراتی CSV |

### هدایا

صدور هدایای قابل مطالبه (روز / ترافیک).

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/client-gifts` | وارد شده است | لیست هدیه |
| POST | `/api/v1/client-gifts` | مدیر | هدیه ایجاد کنید |
| POST | `/api/v1/client-gifts/batch` | مدیر | هدایا را به صورت عمده بسازید |
| GET | `/api/v1/client-gifts/search-clients` | وارد شده است | مشتریان را جستجو کنید |

### پیکربندی ارجاع

پیکربندی و آمار کمیسیون ارجاع.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/referral/config` | وارد شده است | پیکربندی ارجاع |
| PUT | `/api/v1/referral/config` | مدیر | پیکربندی ارجاع را ذخیره کنید |
| GET | `/api/v1/referral/records` | وارد شده است | لیست رکورد ارجاع |
| GET | `/api/v1/referral/stats` | وارد شده است | آمار ارجاع |

### برداشت های ارجاعی

درخواست های برداشت را بررسی کنید.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/referral-withdrawals` | مدیر | لیست درخواست برداشت |
| POST | `/api/v1/referral-withdrawals/:id/approve` | مدیر | تصویب خروج |
| POST | `/api/v1/referral-withdrawals/:id/reject` | مدیر | رد انصراف |

### لیست های مجاز IP (خلاصه)

مشاهده فهرست مجاز IP در همه مشتریان.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/ip-whitelists` | وارد شده است | تمام لیست های مجاز IP مشتری |

### الزامات OAuth مشتری

اتصالات حساب شخص ثالث مشتری را پرس و جو کنید.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/client-oauth-bindings` | وارد شده است | لیست الزام آور OAuth مشتری |

### بلیط (ادمین)

پاسخ بلیط، تکلیف و بسته شدن.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/tickets` | وارد شده است | لیست بلیط |
| GET | `/api/v1/tickets/:id` | وارد شده است | جزئیات بلیط |
| POST | `/api/v1/tickets/:id/assign` | اپراتور | تخصیص بلیط |
| POST | `/api/v1/tickets/:id/close` | اپراتور | بستن بلیط |
| POST | `/api/v1/tickets/:id/reply` | اپراتور | پاسخ به بلیط |

### مقالات

CRUD برای مقالات پورتال.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/articles` | وارد شده است | لیست مقاله |
| POST | `/api/v1/articles` | مدیر | مقاله ایجاد کنید |
| DELETE | `/api/v1/articles/:id` | مدیر | حذف مقاله |
| GET | `/api/v1/articles/:id` | وارد شده است | جزئیات مقاله |
| PUT | `/api/v1/articles/:id` | مدیر | به روز رسانی مقاله |

### حساب های مشترک

سوابق صدور و تعمیر برای حساب های مشترک (استریم و غیره).

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/shared-accounts` | وارد شده است | لیست حساب مشترک |
| POST | `/api/v1/shared-accounts` | مدیر | ایجاد حساب مشترک |
| DELETE | `/api/v1/shared-accounts/:id` | مدیر | حذف حساب مشترک |
| GET | `/api/v1/shared-accounts/:id` | وارد شده است | جزئیات حساب مشترک |
| PUT | `/api/v1/shared-accounts/:id` | مدیر | به روز رسانی حساب مشترک |
| GET | `/api/v1/shared-accounts/:id/repair-logs` | وارد شده است | سوابق تعمیر |

### گواهینامه ها

صدور، آپلود، تمدید و صادرات گواهی عمومی (ACME). صدور در یک گره خاص پشتیبانی می شود.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/certificates` | مدیر | فهرست کنید |
| POST | `/api/v1/certificates` | مدیر | ایجاد کنید |
| DELETE | `/api/v1/certificates/:id` | مدیر | حذف کنید |
| GET | `/api/v1/certificates/:id` | مدیر | جزئیات |
| PUT | `/api/v1/certificates/:id` | مدیر | به روز رسانی |
| GET | `/api/v1/certificates/:id/download` | مدیر | دانلود کنید |
| POST | `/api/v1/certificates/:id/renew` | مدیر | تمدید مشتری |
| GET | `/api/v1/certificates/check-port` | مدیر | پورت را بررسی کنید |
| GET | `/api/v1/certificates/detect-ip` | مدیر | پروب IP |
| GET | `/api/v1/certificates/export` | مدیر | صادرات |
| POST | `/api/v1/certificates/issue` | مدیر | موضوع. بدنه: `{"domains":["example.com","*.example.com"],"challenge_type":"dns-01","dns_channel_id":2,"key_type":"rsa","key_bits":4096}`. `challenge_type` پیش‌فرض http-01 است. `dns_channel_id` یک کانال DNS را انتخاب می کند (یک ارائه دهنده DNS پیکربندی شده + اطلاعات کاربری) |
| POST | `/api/v1/certificates/issue-on-node` | مدیر | مشکل به گره |
| POST | `/api/v1/certificates/upload` | مدیر | آپلود کنید |
| GET | `/api/v1/certificate-tasks/:id` | مدیر | پرس و جو از وضعیت کار پس زمینه |

### کانال های گواهی

پیکربندی کانال ACME (CA).

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/cert-channels` | مدیر | فهرست کنید |
| POST | `/api/v1/cert-channels` | مدیر | ایجاد کنید |
| DELETE | `/api/v1/cert-channels/:id` | مدیر | حذف کنید |
| PUT | `/api/v1/cert-channels/:id` | مدیر | به روز رسانی |
| GET | `/api/v1/cert-channels/providers` | مدیر | ارائه دهنده |

### پیکربندی EAB

اعتبارنامه های الزام آور حساب های خارجی مانند ZeroSSL / Google.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/eab-configs` | مدیر | فهرست کنید |
| POST | `/api/v1/eab-configs` | مدیر | ایجاد کنید |
| DELETE | `/api/v1/eab-configs/:id` | مدیر | حذف کنید |
| PUT | `/api/v1/eab-configs/:id` | مدیر | به روز رسانی |
| POST | `/api/v1/eab-configs/fetch-google` | مدیر | Google EAB را از GCP واکشی کنید |
| POST | `/api/v1/eab-configs/fetch-zerossl` | مدیر | ZeroSSL EAB را با یک ایمیل با یک کلیک واکشی کنید |

### حساب های ACME

مدیریت حساب ACME سمت پنل (ایمیل تماس).

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/acme-accounts` | مدیر | فهرست کنید |
| POST | `/api/v1/acme-accounts` | مدیر | ایجاد کنید |
| DELETE | `/api/v1/acme-accounts/:id` | مدیر | حذف کنید |
| PUT | `/api/v1/acme-accounts/:id` | مدیر | به روز رسانی |
| GET | `/api/v1/acme-accounts/default` | مدیر | دریافت پیش فرض ها |

### CA داخلی

پانل داخلی CA: صدور / تمدید / ابطال گواهی نماینده، صادرات CRL، صف توزیع.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/ca/agent-certs` | مدیر | لیست گواهی نمایندگی |
| POST | `/api/v1/ca/agent-certs/:server_id/renew` | مدیر | تمدید گواهی نمایندگی |
| GET | `/api/v1/ca/distribution-queue` | مدیر | صف توزیع گواهی |
| DELETE | `/api/v1/ca/distribution-queue/:id` | مدیر | حذف ورودی صف توزیع |
| POST | `/api/v1/ca/distribution-queue/:id/retry` | مدیر | توزیع گواهی را دوباره امتحان کنید |
| GET | `/api/v1/ca/revocations` | مدیر | لیست ابطال |
| GET | `/api/v1/ca/revocations/export` | مدیر | صادرات CRL |
| GET | `/api/v1/ca/status` | مدیر | وضعیت CA |

### قوانین جغرافیایی

همگام سازی و تحویل منابع و فایل های داده geoip/geosite.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/geo-rules/files` | وارد شده است | لیست فایل |
| DELETE | `/api/v1/geo-rules/files/:fileId` | مدیر | حذف فایل |
| PUT | `/api/v1/geo-rules/files/:fileId/auto-sync` | مدیر | همگام‌سازی خودکار را تغییر دهید |
| POST | `/api/v1/geo-rules/files/:fileId/download` | مدیر | دانلود کنید |
| GET | `/api/v1/geo-rules/files/:fileId/download-file` | وارد شده است | دانلود فایل |
| GET | `/api/v1/geo-rules/github-assets` | وارد شده است | دارایی های مخزن GitHub را واکشی کنید |
| POST | `/api/v1/geo-rules/push-all` | مدیر | فشار به تمام گره ها |
| POST | `/api/v1/geo-rules/seed` | مدیر | داده های پیش فرض را راه اندازی کنید |
| GET | `/api/v1/geo-rules/sources` | وارد شده است | فهرست منابع داده |
| POST | `/api/v1/geo-rules/sources` | مدیر | ایجاد منبع داده |
| DELETE | `/api/v1/geo-rules/sources/:sourceId` | مدیر | حذف منبع داده |
| PUT | `/api/v1/geo-rules/sources/:sourceId` | مدیر | منبع داده را به روز کنید |
| GET | `/api/v1/geo-rules/sources/:sourceId/assets` | وارد شده است | واکشی دارایی‌های انتشار |
| POST | `/api/v1/geo-rules/sources/:sourceId/files` | مدیر | فایل اضافه کنید |
| PUT | `/api/v1/geo-rules/sources/:sourceId/schedule` | مدیر | همگام سازی برنامه ریزی شده را تغییر دهید |
| POST | `/api/v1/geo-rules/sources/:sourceId/select` | مدیر | فایل را انتخاب کنید |
| POST | `/api/v1/geo-rules/sources/:sourceId/sync` | مدیر | این منبع داده را همگام سازی کنید |
| GET | `/api/v1/geo-rules/status` | وارد شده است | وضعیت |
| POST | `/api/v1/geo-rules/sync-all` | مدیر | همگام سازی همه |
| GET | `/api/v1/geo-rules/sync-status/:key` | وارد شده است | همگام سازی پیشرفت |

### کتابخانه قوانین WAF

قوانین جهانی WAF: منبع GitHub، همگام سازی، تحویل به همه گره ها.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/waf/github-source` | وارد شده است | منبع داده GitHub |
| PUT | `/api/v1/waf/github-source` | مدیر | منبع داده GitHub را ذخیره کنید |
| POST | `/api/v1/waf/push-all` | مدیر | فشار به تمام گره ها |
| GET | `/api/v1/waf/rules` | وارد شده است | لیست قوانین |
| DELETE | `/api/v1/waf/rules/:name` | مدیر | حذف قانون |
| GET | `/api/v1/waf/rules/:name` | وارد شده است | قاعده بگیر |
| PUT | `/api/v1/waf/rules/:name` | مدیر | قانون ذخیره |
| POST | `/api/v1/waf/sync-all` | مدیر | همگام سازی همه |
| POST | `/api/v1/waf/sync-github` | مدیر | همگام سازی از GitHub |

### مدیریت لاگ

خواندن و پخش گزارش گره/هسته، سیاست‌های پاکسازی، کنترل گزارش‌نویس.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| POST | `/api/v1/log-manage/cleanup-all` | مدیر | حالا همه چیز را پاک کن |
| GET | `/api/v1/log-manage/cleanup-rules` | مدیر | فهرست قوانین پاکسازی |
| PUT | `/api/v1/log-manage/cleanup-rules/:id` | مدیر | قانون پاکسازی را به روز کنید |
| POST | `/api/v1/log-manage/cleanup-table/:table` | مدیر | اکنون جدول مشخص شده را تمیز کنید |
| GET | `/api/v1/log-manage/core-log-output` | مدیر | پیکربندی خروجی گزارش اصلی |
| PUT | `/api/v1/log-manage/core-log-output` | مدیر | پیکربندی خروجی گزارش هسته را ذخیره کنید |
| GET | `/api/v1/log-manage/core/:type/stdout-log` | مدیر | گزارش‌های اصلی stdout را بخوانید |
| GET | `/api/v1/log-manage/core/:type/stdout-stream` | مدیر | استریم core stdout |
| POST | `/api/v1/log-manage/discover` | مدیر | فایل های گزارش گره را کشف کنید |
| GET | `/api/v1/log-manage/log-output` | مدیر | پیکربندی خروجی ورود به سیستم |
| PUT | `/api/v1/log-manage/log-output` | مدیر | پیکربندی خروجی گزارش را ذخیره کنید |
| GET | `/api/v1/log-manage/nodes/:serverId/logs` | مدیر | لیست فایل گزارش گره |
| GET | `/api/v1/log-manage/nodes/:serverId/logs/:type` | مدیر | گزارش‌های گره را بخوانید |
| POST | `/api/v1/log-manage/nodes/:serverId/logs/:type/rotate` | مدیر | چرخش سیاهههای مربوط به گره |
| GET | `/api/v1/log-manage/nodes/:serverId/logs/:type/stream` | مدیر | گزارش‌های گره جریان |
| GET | `/api/v1/log-manage/policies` | مدیر | لیست سیاست |
| POST | `/api/v1/log-manage/policies` | مدیر | ایجاد خط مشی |
| DELETE | `/api/v1/log-manage/policies/:id` | مدیر | حذف خط مشی |
| GET | `/api/v1/log-manage/policies/:id` | مدیر | جزئیات خط مشی |
| PUT | `/api/v1/log-manage/policies/:id` | مدیر | سیاست به روز رسانی |
| GET | `/api/v1/log-manage/policies/:id/jobs` | مدیر | لیست کار پاکسازی |
| GET | `/api/v1/log-manage/policies/:id/log` | مدیر | لاگ ها را بخوانید |
| GET | `/api/v1/log-manage/policies/:id/log-stream` | مدیر | گزارش های جریان |
| POST | `/api/v1/log-manage/policies/:id/rotate` | مدیر | چرخش |
| GET | `/api/v1/log-manage/proxy/:type/stdout-log` | مدیر | گزارش‌های اصلی stdout را بخوانید |
| GET | `/api/v1/log-manage/proxy/:type/stdout-stream` | مدیر | استریم core stdout |
| GET | `/api/v1/log-manage/scheduler-config` | مدیر | پیکربندی زمان بندی |
| PUT | `/api/v1/log-manage/scheduler-config` | مدیر | پیکربندی زمان بندی را به روز کنید |
| GET | `/api/v1/log-manage/status` | مدیر | وضعیت |
| POST | `/api/v1/log-manage/sync` | مدیر | همگام سازی سیاست های پاکسازی |
| GET | `/api/v1/log-manage/writers` | مدیر | وضعیت نویسنده |

### پشتیبان گیری

پایگاه داده/پشتیبان گیری کامل، بازیابی، کارهای برنامه ریزی شده، ذخیره سازی از راه دور FTP/S3/WebDAV.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| DELETE | `/api/v1/backups/:id` | فوق ادمین | پشتیبان را حذف کنید |
| GET | `/api/v1/backups/:id/download` | مدیر | دانلود نسخه پشتیبان |
| POST | `/api/v1/backups/:id/restore` | فوق ادمین | بازیابی از پشتیبان |
| POST | `/api/v1/backups/:id/upload-ftp` | فوق ادمین | در FTP آپلود کنید |
| POST | `/api/v1/backups/:id/upload-s3` | فوق ادمین | در S3 آپلود کنید |
| POST | `/api/v1/backups/:id/upload-webdav` | فوق ادمین | در WebDAV آپلود کنید |
| POST | `/api/v1/backups/create` | مدیر | ایجاد پشتیبان کامل (با DB dump) |
| GET | `/api/v1/backups/list` | مدیر | لیست کامل پشتیبان |
| POST | `/api/v1/backups/remote-restore` | فوق ادمین | بازیابی از راه دور |
| GET | `/api/v1/backups/restore-status` | فوق ادمین | بازیابی پیشرفت |
| GET | `/api/v1/backups/schedules` | مدیر | لیست مشاغل برنامه ریزی شده |
| POST | `/api/v1/backups/schedules` | مدیر | ایجاد کار برنامه ریزی شده |
| DELETE | `/api/v1/backups/schedules/:id` | مدیر | کار برنامه ریزی شده را حذف کنید |
| PUT | `/api/v1/backups/schedules/:id` | مدیر | کار برنامه ریزی شده را به روز کنید |
| GET | `/api/v1/backups/storages` | مدیر | لیست تنظیمات ذخیره سازی |
| POST | `/api/v1/backups/storages` | مدیر | ایجاد تنظیمات ذخیره سازی |
| DELETE | `/api/v1/backups/storages/:id` | مدیر | تنظیمات ذخیره سازی را حذف کنید |
| PUT | `/api/v1/backups/storages/:id` | مدیر | پیکربندی ذخیره سازی را به روز کنید |
| GET | `/api/v1/backups/storages/:id/files` | مدیر | لیست فایل های پشتیبان گیری از راه دور |
| POST | `/api/v1/backups/upload-restore` | فوق ادمین | آپلود پشتیبان و بازیابی |

### تنظیمات سیستم

پیکربندی و انتقال پایگاه داده، تنظیمات پنل، کلیدهای JWT، آزمایش‌های SMTP/Telegram، الگوهای اعلان، ارتقاء آنلاین (عمدتاً توسط مدیر).

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| PUT | `/api/v1/system/bootstrap-finish` | مدیر | راه اندازی اولین بوت را کامل کنید |
| GET | `/api/v1/system/bootstrap-state` | مدیر | وضعیت اولین راه‌اندازی |
| GET | `/api/v1/system/config-file/:coreType` | مدیر | فایل پیکربندی هسته را بخوانید |
| GET | `/api/v1/system/db-config` | مدیر | پیکربندی پایگاه داده |
| PUT | `/api/v1/system/db-config` | مدیر | پیکربندی پایگاه داده را به روز کنید |
| POST | `/api/v1/system/db-config/migrate` | فوق ادمین | انتقال داده های پایگاه داده |
| POST | `/api/v1/system/db-config/switch` | فوق ادمین | تغییر پایگاه داده (با مهاجرت یک مرحله ای) |
| POST | `/api/v1/system/db-config/test` | مدیر | تست اتصال پایگاه داده |
| GET | `/api/v1/system/db-pool-stats` | مدیر | آمار استخر اتصال |
| GET | `/api/v1/system/db-stats` | مدیر | آمار پایگاه داده |
| GET | `/api/v1/system/db-stats/ping` | مدیر | کاوشگر پایگاه داده |
| POST | `/api/v1/system/db-stats/reset` | مدیر | بازنشانی آمار پایگاه داده |
| GET | `/api/v1/system/encryption-key-status` | مدیر | وضعیت کلید رمزگذاری |
| GET | `/api/v1/system/env-config` | مدیر | پیکربندی env |
| PUT | `/api/v1/system/env-config` | مدیر | پیکربندی .env را به روز کنید |
| POST | `/api/v1/system/generate-encryption-key` | فوق ادمین | ایجاد کلید رمزگذاری |
| GET | `/api/v1/system/github-config` | مدیر | پیکربندی GitHub |
| PUT | `/api/v1/system/github-config` | مدیر | پیکربندی GitHub را به روز کنید |
| GET | `/api/v1/system/jwt-secret-status` | مدیر | وضعیت کلید JWT |
| GET | `/api/v1/system/maintenance` | عمومی | وضعیت حالت نگهداری (عمومی) |
| PUT | `/api/v1/system/maintenance` | مدیر | تغییر حالت نگهداری |
| POST | `/api/v1/system/migrate-data` | فوق ادمین | انتقال داده ها |
| GET | `/api/v1/system/notification-config` | مدیر | پیکربندی اعلان |
| PUT | `/api/v1/system/notification-config` | مدیر | پیکربندی اعلان را به روز کنید |
| GET | `/api/v1/system/notification-templates` | مدیر | لیست قالب |
| POST | `/api/v1/system/notification-templates` | مدیر | قالب ایجاد کنید |
| DELETE | `/api/v1/system/notification-templates/:id` | مدیر | حذف قالب |
| PUT | `/api/v1/system/notification-templates/:id` | مدیر | به روز رسانی قالب |
| POST | `/api/v1/system/notification-templates/:id/broadcast` | مدیر | اعلان پخش بر اساس الگو |
| POST | `/api/v1/system/notification-templates/:id/send` | مدیر | ارسال اعلان آزمون |
| GET | `/api/v1/system/panel-config` | مدیر | پیکربندی پنل |
| PUT | `/api/v1/system/panel-config` | مدیر | پیکربندی پنل را به روز کنید |
| GET | `/api/v1/system/reality-targets` | مدیر | لیست هدف REALITY |
| PUT | `/api/v1/system/reality-targets` | مدیر | هدف REALITY را به روز کنید |
| POST | `/api/v1/system/reality-targets/reset` | فوق ادمین | هدف REALITY را به حالت پیش فرض بازنشانی کنید |
| GET | `/api/v1/system/reality-local-sni` | مدیر | پیکربندی Reality محلی SNI را بخوانید |
| PUT | `/api/v1/system/reality-local-sni` | مدیر | پیکربندی Reality محلی SNI را به روز کنید |
| POST | `/api/v1/system/regenerate-jwt-secret` | فوق ادمین | کلید JWT را بازسازی کنید |
| GET | `/api/v1/system/releases` | مدیر | نسخه های موجود |
| POST | `/api/v1/system/restart` | فوق ادمین | راه اندازی مجدد |
| POST | `/api/v1/system/rollback` | فوق ادمین | بازگشت سیستم |
| GET | `/api/v1/system/settings` | مدیر | تنظیمات را بخوانید |
| PUT | `/api/v1/system/settings/batch` | مدیر | تنظیمات به روز رسانی انبوه |
| GET | `/api/v1/system/stats` | مدیر | وضعیت سیستم |
| GET | `/api/v1/system/subscription-billing-templates` | مدیر | الگوی صورتحساب اشتراک |
| PUT | `/api/v1/system/subscription-billing-templates` | مدیر | الگوی صورت‌حساب اشتراک را به‌روزرسانی کنید |
| POST | `/api/v1/system/telegram-chat-id` | مدیر | شناسه چت Telegram را دریافت کنید |
| POST | `/api/v1/system/test-smtp` | مدیر | تست SMTP |
| POST | `/api/v1/system/test-telegram` | مدیر | تست Telegram |
| POST | `/api/v1/system/update` | فوق ادمین | ارتقاء سیستم |
| GET | `/api/v1/system/update-jobs` | مدیر | لیست مشاغل را ارتقا دهید |
| POST | `/api/v1/system/upload-mtls-ca` | مدیر | گواهی mTLS CA را آپلود کنید |

### ممیزی امنیتی

پرس و جوهای رویداد حسابرسی و صادرات.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/security/audit-events` | مدیر | لیست رویدادهای حسابرسی |
| GET | `/api/v1/security/audit-events/export` | مدیر | رویدادهای حسابرسی صادراتی |
| GET | `/api/v1/security/subscription-access-logs` | مدیر | گزارش‌های واکشی اشتراک (فیلتر بر اساس شناسه اشتراک / IP / ایمیل مشتری) |

### معیارها

سنجه های انبوه پانل (گره ها/گواهینامه ها/سرویس ها).

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/metrics/certificates` | مدیر | معیارهای گواهی |
| GET | `/api/v1/metrics/nodes` | مدیر | معیارهای گره |
| GET | `/api/v1/metrics/overview` | مدیر | نمای کلی |

### قالب های پورتال

قالب های پورتال، پیکربندی فعال سازی و مدیریت کش.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/portal/active` | مدیر | قالب فعال |
| PUT | `/api/v1/portal/active` | مدیر | به روز رسانی قالب فعال |
| DELETE | `/api/v1/portal/cache` | مدیر | کش پورتال را پاک کنید |
| GET | `/api/v1/portal/cache` | مدیر | وضعیت حافظه پنهان |
| PUT | `/api/v1/portal/cache/dev-mode` | مدیر | حالت توسعه حافظه پنهان را تغییر دهید |
| POST | `/api/v1/portal/cache/rebuild` | مدیر | کش پورتال را بازسازی کنید |
| GET | `/api/v1/portal/lang/builtin` | مدیر | بسته های زبان داخلی |
| POST | `/api/v1/portal/reset` | مدیر | بازیابی قالب پیش فرض |
| GET | `/api/v1/portal/templates` | مدیر | فهرست کنید |
| POST | `/api/v1/portal/templates` | مدیر | ایجاد کنید |
| DELETE | `/api/v1/portal/templates/:id` | مدیر | حذف کنید |
| GET | `/api/v1/portal/templates/:id` | مدیر | جزئیات |
| PUT | `/api/v1/portal/templates/:id` | مدیر | به روز رسانی |
| POST | `/api/v1/portal/templates/:id/activate` | مدیر | قالب را فعال کنید |

> `/portal/templates` CRUD یک **پیشرفته API** است: رابط کاربری پورتال در درجه اول از `PUT /api/v1/portal/active` استفاده می کند
> (پیکربندی کامل سایت تک شات)؛ با این گروه چندین الگو را مدیریت/تغییر دهید. هر دو پشتیبانی می شوند و تضاد ندارند.

### نصب عامل

Node Agent اسکریپت را نصب و دانلود کنید (عمومی).

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| POST | `/api/v1/agent/bootstrap` | عمومی | ثبت اولین بوت عامل (گزارش نماینده با توکن بوت استرپ؛ عمومی) |
| GET | `/api/v1/agent/releases` | مدیر | نسخه های موجود نماینده (برای صفحه ارتقاء) |
| GET | `/api/v1/agent/version` | مدیر | نسخه عامل فعلی در پانل |
| GET | `/api/v1/agent/download` | عمومی | عامل دانلود |
| GET | `/api/v1/agent/install` | عمومی | دریافت اسکریپت نصب |
| GET | `/api/v1/agent/origin` | عمومی | آدرس مبدا پانل (عمومی) |

### شارژ ورودی برگشت به تماس

پاسخ به تماس پلت فرم پرداخت شخص ثالث (عمومی، امضا تأیید شده).

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| POST | `/api/v1/charge/callback/:site_id` | عمومی | رسیدگی به تماس شارژ |

### ورودی ها (با گره ها)

لیست ورودی برای انتخاب گره پلان استفاده می شود.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/inbounds-with-server` | وارد شده است | همه ورودی ها (از جمله گره های آنها) |

### پیکربندی عمومی

پیکربندی زمان اجرا پورتال بدون ورود به سیستم (نگهبان تعمیر و نگهداری + محدودیت نرخ عمومی).

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/public/articles` | عمومی | لیست مقالات عمومی |
| GET | `/api/v1/public/articles/:slug` | عمومی | جزئیات مقاله عمومی |
| GET | `/api/v1/public/client-configs` | عمومی | پیکربندی دانلود کلاینت (عمومی) |
| GET | `/api/v1/public/portal-config` | عمومی | پیکربندی زمان اجرا پورتال (نام سایت، موضوع، زبان ها، اسناد انطباق، عمومی) |
| GET | `/api/v1/public/settings` | عمومی | تنظیمات عمومی (به عنوان مثال register_enabled؛ عمومی) |
| GET | `/api/v1/public/subscription-endpoints` | عمومی | لیست نقطه پایانی اشتراک فعال (عمومی) |

### اعتبار مشتری و کسب و کار (همان مجموعه در موتور مدیریت)

نقاط پایانی مشتری مشابه پورتال است. آنها از **توکن مشتری** (`Authorization: Bearer <client_token>`) استفاده می کنند، نه یک توکن مدیریت. برای جزئیات بیشتر به `portal-api.en.md` مراجعه کنید.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/client-auth/articles` | مشتری (ورود به سیستم) | فهرست مقالات |
| GET | `/api/v1/client-auth/articles/:slug` | مشتری (ورود به سیستم) | دریافت مقاله |
| GET | `/api/v1/client-auth/balance` | مشتری (ورود به سیستم) | تعادل را بدست آورید |
| GET | `/api/v1/client-auth/balance/transactions` | مشتری (ورود به سیستم) | تراکنش های موجود |
| POST | `/api/v1/client-auth/change-password` | مشتری (ورود به سیستم) | رمز عبور را تغییر دهید |
| POST | `/api/v1/client-auth/charge/create` | مشتری (ورود به سیستم) | ایجاد سفارش شارژ |
| GET | `/api/v1/client-auth/charge/orders` | مشتری (ورود به سیستم) | لیست سفارشات شارژ |
| GET | `/api/v1/client-auth/charge/orders/:id` | مشتری (ورود به سیستم) | جزئیات سفارش شارژ را دریافت کنید |
| POST | `/api/v1/client-auth/charge/redeem-card` | مشتری (ورود به سیستم) | بازخرید کارت |
| GET | `/api/v1/client-auth/charge/sites` | مشتری (ورود به سیستم) | لیست سایت های شارژ |
| GET | `/api/v1/client-auth/charge/status/:trade_no` | مشتری (ورود به سیستم) | استعلام وضعیت شارژ |
| GET | `/api/v1/client-auth/checktime` | مشتری (ورود به سیستم) | دریافت زمان سرور (همگام سازی ساعت) |
| GET | `/api/v1/client-auth/connection-info` | مشتری (ورود به سیستم) | پارامترهای اتصال طرح را دریافت کنید |
| GET | `/api/v1/client-auth/gifts` | مشتری (ورود به سیستم) | لیست هدایا |
| POST | `/api/v1/client-auth/gifts/:id/claim` | مشتری (ورود به سیستم) | ادعای هدیه |
| GET | `/api/v1/client-auth/invoices` | مشتری (ورود به سیستم) | لیست سفارش |
| GET | `/api/v1/client-auth/invoices/:id` | مشتری (ورود به سیستم) | جزئیات سفارش |
| POST | `/api/v1/client-auth/login` | مشتری (عمومی) | وارد شوید |
| POST | `/api/v1/client-auth/logout` | مشتری (ورود به سیستم) | از سیستم خارج شوید |
| POST | `/api/v1/client-auth/logs/package` | مشتری (ورود به سیستم) | ایجاد بسته گزارش |
| GET | `/api/v1/client-auth/logs/packages` | مشتری (ورود به سیستم) | فهرست بسته‌های گزارش |
| DELETE | `/api/v1/client-auth/logs/packages/:id` | مشتری (ورود به سیستم) | بسته گزارش را حذف کنید |
| GET | `/api/v1/client-auth/logs/packages/:id` | مشتری (ورود به سیستم) | بسته گزارش را دریافت کنید |
| GET | `/api/v1/client-auth/logs/packages/:id/download` | مشتری (ورود به سیستم) | دانلود بسته گزارش |
| GET | `/api/v1/client-auth/logs/types` | مشتری (ورود به سیستم) | بررسی سلامت بسته نرم افزاری گزارش |
| GET | `/api/v1/client-auth/me` | مشتری (ورود به سیستم) | کاربر فعلی وارد شده را دریافت کنید |
| GET | `/api/v1/client-auth/nodes` | مشتری (ورود به سیستم) | لیست گره های من |
| GET | `/api/v1/client-auth/nodes/:id/config` | مشتری (ورود به سیستم) | پیکربندی گره |
| GET | `/api/v1/client-auth/nodes/:id/link` | مشتری (ورود به سیستم) | پیوند اشتراک گره |
| GET | `/api/v1/client-auth/nodes/:id/qrcode` | مشتری (ورود به سیستم) | کد QR گره |
| GET | `/api/v1/client-auth/oauth/:provider` | مشتری (عمومی) | مجاز کردن |
| GET | `/api/v1/client-auth/oauth/:provider/callback` | مشتری (عمومی) | پاسخ به تماس |
| GET | `/api/v1/client-auth/oauth/providers` | مشتری (عمومی) | وضعیت ارائه دهنده |
| GET | `/api/v1/client-auth/orders` | مشتری (ورود به سیستم) | لیست سفارش |
| POST | `/api/v1/client-auth/orders` | مشتری (ورود به سیستم) | نظم ایجاد کنید |
| GET | `/api/v1/client-auth/orders/:id` | مشتری (ورود به سیستم) | جزئیات سفارش |
| GET | `/api/v1/client-auth/plans` | مشتری (ورود به سیستم) | لیست طرح |
| GET | `/api/v1/client-auth/plans/:id` | مشتری (ورود به سیستم) | جزئیات طرح |
| GET | `/api/v1/client-auth/referral/code` | مشتری (ورود به سیستم) | کد تایید ارجاع را دریافت کنید |
| GET | `/api/v1/client-auth/referral/records` | مشتری (ورود به سیستم) | فهرست سوابق ارجاع |
| GET | `/api/v1/client-auth/referral/stats` | مشتری (ورود به سیستم) | آمار ارجاع را دریافت کنید |
| POST | `/api/v1/client-auth/referral/withdraw` | مشتری (ورود به سیستم) | درخواست برداشت |
| GET | `/api/v1/client-auth/referral/withdrawals` | مشتری (ورود به سیستم) | فهرست برداشت های ارجاعی |
| POST | `/api/v1/client-auth/refresh` | مشتری (عمومی) | تازه کردن |
| POST | `/api/v1/client-auth/register` | مشتری (عمومی) | ثبت نام کنید |
| POST | `/api/v1/client-auth/reset-subscription` | مشتری (ورود به سیستم) | اشتراک را بازنشانی کنید |
| POST | `/api/v1/client-auth/rotate-token` | مشتری (ورود به سیستم) | چرخاندن نشانه اشتراک |
| POST | `/api/v1/client-auth/self-unban` | مشتری (ورود به سیستم) | سلف سرویس لغو ممنوعیت |
| PUT | `/api/v1/client-auth/settings` | مشتری (ورود به سیستم) | تنظیمات را به روز کنید |
| POST | `/api/v1/client-auth/settings/auto-add-ip` | مشتری (ورود به سیستم) | افزودن خودکار IP |
| GET | `/api/v1/client-auth/settings/whitelist` | مشتری (ورود به سیستم) | فهرست مجاز IP |
| DELETE | `/api/v1/client-auth/settings/whitelist/:id` | مشتری (ورود به سیستم) | از لیست مجاز IP حذف کنید |
| GET | `/api/v1/client-auth/shared-accounts` | مشتری (ورود به سیستم) | فهرست حساب های مشترک |
| GET | `/api/v1/client-auth/shared-accounts/:id` | مشتری (ورود به سیستم) | حساب مشترک دریافت کنید |
| POST | `/api/v1/client-auth/shared-accounts/:id/apply` | مشتری (ورود به سیستم) | درخواست حساب مشترک |
| POST | `/api/v1/client-auth/shared-accounts/:id/repair` | مشتری (ورود به سیستم) | درخواست تعمیر |
| POST | `/api/v1/client-auth/ticket` | مشتری (ورود به سیستم) | صدور بلیط یک بار کوتاه مدت |
| GET | `/api/v1/client-auth/tickets` | مشتری (ورود به سیستم) | لیست بلیط ها |
| POST | `/api/v1/client-auth/tickets` | مشتری (ورود به سیستم) | بلیط ایجاد کنید |
| GET | `/api/v1/client-auth/tickets/:id` | مشتری (ورود به سیستم) | بلیط بگیر |
| POST | `/api/v1/client-auth/tickets/:id/close` | مشتری (ورود به سیستم) | بستن بلیط |
| POST | `/api/v1/client-auth/tickets/:id/reply` | مشتری (ورود به سیستم) | پاسخ به بلیط |
| GET | `/api/v1/client-auth/traffic-history` | مشتری (ورود به سیستم) | تاریخچه ترافیک |
| GET | `/api/v1/client-auth/traffic-packs` | مشتری (ورود به سیستم) | لیست بسته ترافیک |
| POST | `/api/v1/client-auth/traffic-packs/buy` | مشتری (ورود به سیستم) | خرید پک ترافیک |

### تحویل اشتراک

نقطه ورود عمومی برای مشتریان برای دریافت اشتراک. `subscription.en.md` را ببینید.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/sub/:token` | عمومی | خدمات اشتراک |

### گردش کار پیکربندی زمان اجرا

تصاویر فوری پیکربندی / اجرای خشک / اعمال / برگشت (`apply`، `rollback` و `disable-proxy` نیاز به سرپرست دارند).

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| POST | `/api/v1/runtime/apply` | مدیر | درخواست کنید |
| GET | `/api/v1/runtime/apply-jobs` | وارد شده است | فهرست مشاغل |
| GET | `/api/v1/runtime/apply-jobs/:id` | وارد شده است | کار پیدا کن |
| POST | `/api/v1/runtime/apply-jobs/:id/cancel` | مدیر | کار را لغو کنید |
| POST | `/api/v1/runtime/apply-jobs/:id/retry` | مدیر | دوباره کار را امتحان کنید |
| POST | `/api/v1/runtime/disable-proxy` | مدیر | غیر فعال کردن پروکسی |
| POST | `/api/v1/runtime/dry-run` | وارد شده است | اجرای خشک |
| GET | `/api/v1/runtime/nodes` | وارد شده است | لیست گره ها |
| GET | `/api/v1/runtime/overview` | وارد شده است | نمای کلی |
| POST | `/api/v1/runtime/rollback` | مدیر | به عقب برگرد |
| GET | `/api/v1/runtime/snapshots` | وارد شده است | فهرست عکس های فوری |
| POST | `/api/v1/runtime/snapshots` | وارد شده است | ایجاد عکس فوری |
| DELETE | `/api/v1/runtime/snapshots/:id` | مدیر | حذف عکس فوری |
| GET | `/api/v1/runtime/snapshots/:id` | وارد شده است | دریافت عکس فوری |
| POST | `/api/v1/runtime/verify` | وارد شده است | تأیید کنید |

### خط‌مشی‌های دسترسی به خدمات (بازنشسته)

> `/services/:id/access-policies` و `/services/:id/ip-bans` در 20/10/2026 بازنشسته شدند:
> هیچ تماس مدیریتی وجود ندارد و هیچ مسیر اتصالی این خط مشی ها را مصرف نمی کند.

### نظارت بر سهمیه ترافیک

نمای نظارت فقط خواندنی از سهمیه ها و استفاده.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/traffic/quotas` | وارد شده است | لیست سهمیه |
| GET | `/api/v1/traffic/quotas/:id/usage` | وارد شده است | استفاده از سهمیه |
| GET | `/api/v1/traffic/reset-logs` | وارد شده است | رکوردهای بازنشانی سهمیه |
| GET | `/api/v1/traffic/usage-history` | وارد شده است | سابقه استفاده |

### بررسی های سلامت

کاوشگر زنده بودن

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/health` | عمومی | کاوشگر زنده بودن؛ 200 یعنی سالم |

### صفحه پورتال

صفحه پورتال SPA.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/p` | عمومی | پورتال HTML (ارائه شده توسط سرور + زمان اجرا Vue) |

### چالش های ACME

پاسخ به چالش HTTP-01.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/.well-known/acme-challenge/:token` | عمومی | پاسخ به چالش ACME HTTP-01 (دامنه را در حین صدور تأیید می کند) |
