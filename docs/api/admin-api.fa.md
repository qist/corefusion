<!-- source: admin-api.md | locale: fa | updated: 2026-10-23 -->
# ادمین API

> زبان‌ها: [简体中文](admin-api.md) · [English](admin-api.en.md) · [繁體中文](admin-api.zh-TW.md) · [Русский](admin-api.ru.md) · [日本語](admin-api.ja.md) · [한국어](admin-api.ko.md) · [Deutsch](admin-api.de.md) · [Français](admin-api.fr.md) · [Español](admin-api.es.md) · [Português](admin-api.pt.md) · [Türkçe](admin-api.tr.md) · [Bahasa Indonesia](admin-api.id.md) · [ไทย](admin-api.th.md) · [Tiếng Việt](admin-api.vi.md) · [العربية](admin-api.ar.md) · فارسی

جدول مسیر مدیریت کامل API برای **توسعه پنل / اتوماسیون / یکپارچه سازها **.

- پایه URL: `https://<panel-domain>/api/v1`
- اعتبار: `Authorization: Bearer <admin_token>` (از `POST /api/v1/auth/login`)
- فرمت‌های رایج پاسخ/خطا و قراردادهای محدودیت نرخ: به [README.md](./README.fa.md) مراجعه کنید
- منبع حقیقت: `internal/api/router/router.go` (739 مسیر مدیریت)؛ اگر این جدول مخالف باشد، **کد برنده می شود**

## نشانگرهای مجوز

| نشانگر | معنی |
|---|---|
| عمومی | نیازی به ورود به سیستم نیست (ممکن است گارد تعمیر و نگهداری / محدودیت نرخ همچنان اعمال شود) |
| عمومی · نرخ محدود | بدون ورود به سیستم، با محدودیت نرخ اختصاصی |
| وارد شده است | به یک رمز ورود به سیستم مدیریت نیاز دارد (`AuthRequired`) |
| مدیر | به نقش ≥ `admin` (`RequireRole(RoleAdmin)`) نیاز دارد |
| فوق ادمین | نیاز به نقش = `super_admin` |

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
- [قالب های پلان](#قالب-های-پلان)
- [انواع طرح](#انواع-طرح)
- [بسته های ترافیکی](#بسته-های-ترافیکی)
- کوپن [](#coupons)
- [کوپن](#کوپن)
- [گروه های گره](#گروه-های-گره)
- [سرورهای نود](#سرورهای-نود)
- [گزارش حسابرسی گره](#گزارش-حسابرسی-گره)
- [گره ها (مدل قدیمی، بازنشسته)](#گره-ها-مدل-قدیمی-بازنشسته)
- اشتراک [](#subscriptions)
- فرمت های اشتراک [](#subscription-formats)
- [نمای کلی منطقه](#نمای-کلی-منطقه)
- [اشتراک ها](#اشتراک-ها)
- [فرمت های اشتراک](#فرمت-های-اشتراک)
- [نقاط پایانی اشتراک](#نقاط-پایانی-اشتراک)
- [دامنه ها را به اشتراک بگذارید](#دامنه-ها-را-به-اشتراک-بگذارید)
- [IP Cloudflare](#ip-cloudflare)
- کارت شارژ [](#recharge-cards)
- [پیکربندی اشتراک (Mihomo/Clash)](#پیکربندی-اشتراک-mihomoclash)
- [سایت های شارژ](#سایت-های-شارژ)
- [سفارشات شارژ](#سفارشات-شارژ)
- [کارت های شارژ](#کارت-های-شارژ)
- [رویدادهای برگشت به تماس را شارژ کنید](#رویدادهای-برگشت-به-تماس-را-شارژ-کنید)
- [شارژ مجدد تماس ها](#شارژ-مجدد-تماس-ها)
- [موجودی مشتری](#موجودی-مشتری)
- [هدایا](#هدایا)
- [پیکربندی ارجاع](#پیکربندی-ارجاع)
- [برداشت های ارجاعی](#برداشت-های-ارجاعی)
- [لیست های مجاز IP (خلاصه)](#لیست-های-مجاز-ip-خلاصه)
- [الزامات OAuth مشتری](#الزامات-oauth-مشتری)
- [بلیط (ادمین)](#بلیط-ادمین)
- [مقالات](#مقالات)
- [حساب های مشترک](#حساب-های-مشترک)
- [ CA](#built-in-ca) داخلی
- [گواهینامه ها](#گواهینامه-ها)
- [کانال های گواهی](#کانال-های-گواهی)
- [پیکربندی EAB](#پیکربندی-eab)
- [حساب های ACME](#حساب-های-acme)
- [CA داخلی](#ca-داخلی)
- [قوانین جغرافیایی](#قوانین-جغرافیایی)
- [کتابخانه قوانین WAF](#کتابخانه-قوانین-waf)
- الگوهای پورتال [](#portal-templates)
- [مدیریت لاگ](#مدیریت-لاگ)
- [پشتیبان گیری](#پشتیبان-گیری)
- [تنظیمات سیستم](#تنظیمات-سیستم)
- [ممیزی امنیتی](#ممیزی-امنیتی)
- [معیارها](#معیارها)
- [قالب های پورتال](#قالب-های-پورتال)
- [نصب عامل](#نصب-عامل)
- [شارژ ورودی برگشت به تماس](#شارژ-ورودی-برگشت-به-تماس)
- [ورودی ها (با گره ها)](#ورودی-ها-با-گره-ها)
- [پیکربندی عمومی](#پیکربندی-عمومی)
- صفحه پورتال [](#portal-page)
- [اعتبار مشتری و کسب و کار (همان مجموعه در موتور مدیریت)](#اعتبار-مشتری-و-کسب-و-کار-همان-مجموعه-در-موتور-مدیریت)

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
| GET | `/api/v1/auth/passkey/rpids` | وارد شده است | لیست مجاز RP ID |
| POST | `/api/v1/auth/passkey/rpids` | وارد شده است | لیست مجاز RP ID را به روز کنید |
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
| POST | `/api/v1/clients` | وارد شده است | مشتری ایجاد کنید. بدنه: `{"email":"user@example.com","password":"plaintext-password","plan_id":1,"quota_bytes":10737418240,"expires_at":"2026-12-31T00:00:00Z","remark":""}`. `plan_id` مورد نیاز است. `password` ممکن است خالی باشد (مشتریان می توانند آن را بعداً در پورتال تنظیم کنند) |
| DELETE | `/api/v1/clients/:id` | وارد شده است | مشتری را حذف کنید |
| GET | `/api/v1/clients/:id` | وارد شده است | مشتری بگیر |
| PUT | `/api/v1/clients/:id` | وارد شده است | مشتری را به روز کنید |
| POST | `/api/v1/clients/:id/downgrade` | وارد شده است | کاهش رتبه مشتری |
| GET | `/api/v1/clients/:id/orders` | وارد شده است | لیست سفارش مشتری |
| POST | `/api/v1/clients/:id/renew` | وارد شده است | تمدید مشتری |
| POST | `/api/v1/clients/:id/reset-usage` | وارد شده است | بازنشانی استفاده |
| POST | `/api/v1/clients/:id/restore` | وارد شده است | رزومه مشتری |
| POST | `/api/v1/clients/:id/suspend` | وارد شده است | تعلیق مشتری |
| POST | `/api/v1/clients/:id/upgrade` | وارد شده است | مشتری را ارتقا دهید |

### قالب های پلان

طرح‌ها (انواع، چرخه‌های صورت‌حساب، قیمت‌ها، سهمیه ترافیک) و مجوز گره در سطح الگو.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/plan-templates` | وارد شده است | لیست قالب |
| POST | `/api/v1/plan-templates` | وارد شده است | قالب ایجاد کنید |
| DELETE | `/api/v1/plan-templates/:id` | وارد شده است | حذف قالب |
| GET | `/api/v1/plan-templates/:id` | وارد شده است | جزئیات الگو |
| PUT | `/api/v1/plan-templates/:id` | وارد شده است | به روز رسانی قالب |
| GET | `/api/v1/plan-templates/:id/accessible-nodes` | وارد شده است | گره های موجود را دریافت کنید |
| GET | `/api/v1/plan-templates/:id/billing-node-options` | وارد شده است | گزینه های گره قابل پرداخت را دریافت کنید |
| PUT | `/api/v1/plan-templates/:id/node-groups` | وارد شده است | گروه های گره الگو را تنظیم کنید |
| POST | `/api/v1/plan-templates/:id/sync-accounts` | وارد شده است | همگام سازی حساب ها با الگو |
| GET | `/api/v1/plan-templates/:id/variants` | وارد شده است | لیست انواع |
| POST | `/api/v1/plan-templates/:id/variants` | وارد شده است | ایجاد نوع |
| POST | `/api/v1/plan-templates/sync-all-accounts` | وارد شده است | همگام سازی همه حساب ها |

### انواع طرح

به روز رسانی و حذف انواع طرح (ایجاد در زیر الگوهای طرح تو در تو است).

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| DELETE | `/api/v1/plan-variants/:id` | وارد شده است | حذف نوع |
| PUT | `/api/v1/plan-variants/:id` | وارد شده است | به روز رسانی نسخه |

### بسته های ترافیکی

بسته های ترافیکی که می توان جداگانه خریداری کرد.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/traffic-packs` | وارد شده است | لیست بسته ترافیک |
| POST | `/api/v1/traffic-packs` | وارد شده است | بسته ترافیکی ایجاد کنید |
| DELETE | `/api/v1/traffic-packs/:id` | وارد شده است | بسته ترافیک را حذف کنید |
| PUT | `/api/v1/traffic-packs/:id` | وارد شده است | بسته ترافیک را به روز کنید |

### کوپن

ایجاد، اعتبارسنجی و بازخرید کوپن.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/promotions` | وارد شده است | لیست کوپن ها |
| POST | `/api/v1/promotions` | وارد شده است | ایجاد کوپن |
| DELETE | `/api/v1/promotions/:id` | وارد شده است | حذف کوپن |
| PUT | `/api/v1/promotions/:id` | وارد شده است | به روز رسانی کوپن |
| POST | `/api/v1/promotions/validate` | وارد شده است | اعتبار کوپن |

### گروه های گره

گره ها را در گروه هایی که توسط پلان ها ارجاع داده شده اند سازماندهی کنید.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/node-groups` | وارد شده است | گروه های گره را فهرست کنید |
| POST | `/api/v1/node-groups` | وارد شده است | ایجاد گروه گره |
| DELETE | `/api/v1/node-groups/:id` | وارد شده است | حذف گروه گره |
| PUT | `/api/v1/node-groups/:id` | وارد شده است | به روز رسانی گروه گره |
| GET | `/api/v1/node-groups/:id/members` | وارد شده است | لیست اعضای گروه |
| POST | `/api/v1/node-groups/:id/members` | وارد شده است | عضو گروه را اضافه کنید |
| DELETE | `/api/v1/node-groups/:id/members/:mid` | وارد شده است | حذف عضو گروه |

### سرورهای نود

منابع اصلی: گره CRUD، هسته (xray/sing-box)، تحویل پیکربندی، پیکربندی ورودی/خروجی/نقطه پایانی، کنترل فرآیند، گواهی‌ها، WAF، محدودیت‌های نرخ، پرش پورت، پراکسی‌های زنجیره‌ای، آینه‌ای/اولیه-پشتیبان، کنترل زمان نصب/راه‌اندازی عامل ZXQ0. بسیاری از مسیرهای فرعی، گروه بندی شده بر اساس خانواده ویژگی.

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
| DELETE | `/api/v1/node-servers/:id/share-domains/all` | مدیر | همه را پاک کنید |
| POST | `/api/v1/node-servers/:id/singbox/control/firewall/apply` | مدیر | قوانین فایروال پورت پرش را اعمال کنید |
| POST | `/api/v1/node-servers/:id/singbox/control/firewall/remove` | مدیر | قوانین فایروال port-hopping را حذف کنید |
| POST | `/api/v1/node-servers/:id/speed-test` | مدیر | تست سرعت گره |
| GET | `/api/v1/node-servers/:id/status` | مدیر | وضعیت گره |
| POST | `/api/v1/node-servers/:id/sync` | مدیر | همگام سازی پیکربندی گره (تولید و فشار دادن) |
| POST | `/api/v1/node-servers/:id/sync-data` | فوق ادمین | انتقال داده های پانل به گره ها (همگام سازی کامل کاربران/اشتراک ها/طرح ها) |
| POST | `/api/v1/node-servers/:id/sync-mirror` | مدیر | همگام سازی گره آینه ای |
| GET | `/api/v1/node-servers/:id/tags` | مدیر | لیست برچسب |
| POST | `/api/v1/node-servers/:id/tags` | مدیر | برچسب اضافه کنید |
| DELETE | `/api/v1/node-servers/:id/tags/:tagKey` | مدیر | حذف برچسب |
| GET | `/api/v1/node-servers/:id/traffic-history` | مدیر | تاریخچه ترافیک |
| POST | `/api/v1/node-servers/:id/traffic-resume` | مدیر | بازیابی ترافیک معلق به دلیل بیش از حد |
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
| GET | `/api/v1/node-servers/pending-agent-updates` | مدیر | عوامل در انتظار ارتقا |
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
| POST | `/api/v1/node-servers/validate-chain-config` | مدیر | اعتبار سنجی پیکربندی زنجیره ای |

### گزارش حسابرسی گره

ممیزی عملیات سمت گره و صادرات.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/node-audit-logs` | وارد شده است | گزارش حسابرسی گره |
| GET | `/api/v1/node-audit-logs/export` | وارد شده است | گزارش های حسابرسی گره را صادر کنید |

### گره ها (مدل قدیمی، بازنشسته)

> `/api/v1/nodes` CRUD در 20/10/2026 بازنشسته شد: گره ها به طور یکسان از طریق `/api/v1/node-servers` مدیریت می شوند.
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
| POST | `/api/v1/subscriptions` | وارد شده است | ایجاد کنید |
| DELETE | `/api/v1/subscriptions/:id` | وارد شده است | حذف کنید |
| GET | `/api/v1/subscriptions/:id` | وارد شده است | جزئیات |
| PUT | `/api/v1/subscriptions/:id` | وارد شده است | به روز رسانی |
| GET | `/api/v1/subscriptions/:id/access-logs` | وارد شده است | گزارش دسترسی |
| POST | `/api/v1/subscriptions/:id/enable` | وارد شده است | فعال کردن |
| POST | `/api/v1/subscriptions/:id/reset-usage` | وارد شده است | بازنشانی استفاده |
| POST | `/api/v1/subscriptions/:id/restore` | وارد شده است | رزومه |
| POST | `/api/v1/subscriptions/:id/revoke` | وارد شده است | لغو |
| POST | `/api/v1/subscriptions/:id/rotate` | وارد شده است | چرخش |
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
| POST | `/api/v1/subscription-endpoints` | وارد شده است | ایجاد کنید |
| DELETE | `/api/v1/subscription-endpoints/:id` | وارد شده است | حذف کنید |
| GET | `/api/v1/subscription-endpoints/:id` | وارد شده است | جزئیات |
| PUT | `/api/v1/subscription-endpoints/:id` | وارد شده است | به روز رسانی |

### دامنه ها را به اشتراک بگذارید

دسترسی به استخر دامنه/IP مورد استفاده در پیوندهای مشتری.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/share-domains` | وارد شده است | فهرست کنید |
| POST | `/api/v1/share-domains` | وارد شده است | ایجاد کنید |
| DELETE | `/api/v1/share-domains/:id` | وارد شده است | حذف کنید |
| PUT | `/api/v1/share-domains/:id` | وارد شده است | به روز رسانی |
| DELETE | `/api/v1/share-domains/all` | وارد شده است | همه را پاک کنید |
| POST | `/api/v1/share-domains/seed` | وارد شده است | داده های پیش فرض را راه اندازی کنید |

### IP Cloudflare

Cloudflare CDN مدیریت استخر IP و منبع.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| DELETE | `/api/v1/cfip/entries` | وارد شده است | ورودی ها را پاک کنید |
| GET | `/api/v1/cfip/entries` | وارد شده است | لیست ورودی |
| POST | `/api/v1/cfip/entries` | وارد شده است | افزودن ورودی |
| DELETE | `/api/v1/cfip/entries/:id` | وارد شده است | حذف ورودی |
| POST | `/api/v1/cfip/fetch-all` | وارد شده است | همه منابع را واکشی کنید |
| POST | `/api/v1/cfip/seed` | وارد شده است | داده های پیش فرض را راه اندازی کنید |
| GET | `/api/v1/cfip/sources` | وارد شده است | فهرست منابع داده |
| POST | `/api/v1/cfip/sources` | وارد شده است | ایجاد منبع داده |
| DELETE | `/api/v1/cfip/sources/:id` | وارد شده است | حذف منبع داده |
| PUT | `/api/v1/cfip/sources/:id` | وارد شده است | منبع داده را به روز کنید |
| POST | `/api/v1/cfip/sources/:id/fetch` | وارد شده است | این منبع را واکشی کنید |

### پیکربندی اشتراک (Mihomo/Clash)

پیکربندی الگوی اشتراک: گروه ها، قوانین، ارائه دهندگان قانون، پیش نمایش YAML.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/mihomo/configs` | وارد شده است | فهرست کنید |
| POST | `/api/v1/mihomo/configs` | وارد شده است | ایجاد کنید |
| DELETE | `/api/v1/mihomo/configs/:id` | وارد شده است | حذف کنید |
| GET | `/api/v1/mihomo/configs/:id` | وارد شده است | جزئیات |
| PUT | `/api/v1/mihomo/configs/:id` | وارد شده است | به روز رسانی |
| GET | `/api/v1/mihomo/configs/:id/proxy-groups` | وارد شده است | لیست گروه |
| POST | `/api/v1/mihomo/configs/:id/proxy-groups` | وارد شده است | گروه ایجاد کنید |
| GET | `/api/v1/mihomo/configs/:id/rule-providers` | وارد شده است | لیست ارائه دهنده قوانین |
| POST | `/api/v1/mihomo/configs/:id/rule-providers` | وارد شده است | ایجاد ارائه دهنده قانون |
| GET | `/api/v1/mihomo/configs/:id/rules` | وارد شده است | لیست قوانین |
| POST | `/api/v1/mihomo/configs/:id/rules` | وارد شده است | قانون ایجاد کنید |
| GET | `/api/v1/mihomo/configs/:id/yaml` | وارد شده است | پیش نمایش YAML ایجاد شده |
| DELETE | `/api/v1/mihomo/proxy-groups/:id` | وارد شده است | حذف گروه |
| DELETE | `/api/v1/mihomo/rule-providers/:id` | وارد شده است | ارائه دهنده قوانین را حذف کنید |
| DELETE | `/api/v1/mihomo/rules/:id` | وارد شده است | حذف قانون |

### سایت های شارژ

ادغام سایت پرداخت شخص ثالث: کلیدهای امضا، فایل های تأیید، تأیید دامنه، اسناد یکپارچه سازی.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/charge-sites` | وارد شده است | لیست سایت |
| POST | `/api/v1/charge-sites` | وارد شده است | سایت ایجاد کنید |
| DELETE | `/api/v1/charge-sites/:id` | وارد شده است | حذف سایت |
| GET | `/api/v1/charge-sites/:id` | وارد شده است | جزئیات سایت |
| PUT | `/api/v1/charge-sites/:id` | وارد شده است | سایت را به روز کنید |
| POST | `/api/v1/charge-sites/:id/clear-prev-secret` | وارد شده است | کلید قدیمی را پاک کنید |
| POST | `/api/v1/charge-sites/:id/generate-verify` | وارد شده است | ایجاد فایل تایید |
| POST | `/api/v1/charge-sites/:id/generate-verify-file` | وارد شده است | ایجاد فایل تایید |
| GET | `/api/v1/charge-sites/:id/integration-doc` | وارد شده است | سند ادغام را دریافت کنید |
| POST | `/api/v1/charge-sites/:id/rotate-secret` | وارد شده است | چرخاندن کلید امضا |
| POST | `/api/v1/charge-sites/:id/verify-dns` | وارد شده است | دامنه DNS را تأیید کنید |
| POST | `/api/v1/charge-sites/:id/verify-domain` | وارد شده است | چالش دامنه HTTP را تأیید کنید |
| POST | `/api/v1/charge-sites/:id/verify-domain-dns` | وارد شده است | دامنه DNS را تأیید کنید |
| POST | `/api/v1/charge-sites/:id/verify-domain-http` | وارد شده است | چالش دامنه HTTP را تأیید کنید |

### سفارشات شارژ

درخواست شارژ مجدد، بازپرداخت، صادرات CSV.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/charge-orders` | وارد شده است | لیست سفارش |
| GET | `/api/v1/charge-orders/:id` | وارد شده است | جزئیات سفارش |
| POST | `/api/v1/charge-orders/:id/refund` | وارد شده است | بازپرداخت |
| GET | `/api/v1/charge-orders/export-csv` | وارد شده است | صادرات سفارشات شارژ CSV |

### کارت های شارژ

تولید کارت انبوه، سوابق استفاده، صادرات CSV.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/charge-cards` | وارد شده است | لیست کارت |
| POST | `/api/v1/charge-cards` | وارد شده است | کارت ایجاد کنید |
| DELETE | `/api/v1/charge-cards/:id` | وارد شده است | حذف کارت |
| GET | `/api/v1/charge-cards/:id/usages` | وارد شده است | سوابق استفاده از کارت |
| POST | `/api/v1/charge-cards/batch` | وارد شده است | کارت تولید انبوه |
| GET | `/api/v1/charge-cards/export-csv` | وارد شده است | کارت های صادراتی CSV |

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
| POST | `/api/v1/client-gifts` | وارد شده است | هدیه ایجاد کنید |
| POST | `/api/v1/client-gifts/batch` | وارد شده است | هدایا را به صورت عمده بسازید |
| GET | `/api/v1/client-gifts/search-clients` | وارد شده است | مشتریان را جستجو کنید |

### پیکربندی ارجاع

پیکربندی و آمار کمیسیون ارجاع.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/referral/config` | وارد شده است | پیکربندی ارجاع |
| PUT | `/api/v1/referral/config` | مدیر | ذخیره پیکربندی ارجاع |
| GET | `/api/v1/referral/records` | وارد شده است | لیست رکورد ارجاع |
| GET | `/api/v1/referral/stats` | وارد شده است | آمار ارجاع |

### برداشت های ارجاعی

درخواست های برداشت را بررسی کنید.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/referral-withdrawals` | وارد شده است | لیست درخواست برداشت |
| POST | `/api/v1/referral-withdrawals/:id/approve` | وارد شده است | تصویب خروج |
| POST | `/api/v1/referral-withdrawals/:id/reject` | وارد شده است | رد انصراف |

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
| POST | `/api/v1/tickets/:id/assign` | وارد شده است | تخصیص بلیط |
| POST | `/api/v1/tickets/:id/close` | وارد شده است | بستن بلیط |
| POST | `/api/v1/tickets/:id/reply` | وارد شده است | پاسخ به بلیط |

### مقالات

CRUD برای مقالات پورتال.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/articles` | وارد شده است | فهرست مقالات |
| POST | `/api/v1/articles` | وارد شده است | مقاله ایجاد کنید |
| DELETE | `/api/v1/articles/:id` | وارد شده است | حذف مقاله |
| GET | `/api/v1/articles/:id` | وارد شده است | جزئیات مقاله |
| PUT | `/api/v1/articles/:id` | وارد شده است | به روز رسانی مقاله |

### حساب های مشترک

سوابق صدور و تعمیر برای حساب های مشترک (استریم و غیره).

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/shared-accounts` | وارد شده است | لیست حساب مشترک |
| POST | `/api/v1/shared-accounts` | وارد شده است | ایجاد حساب مشترک |
| DELETE | `/api/v1/shared-accounts/:id` | وارد شده است | حذف حساب مشترک |
| GET | `/api/v1/shared-accounts/:id` | وارد شده است | جزئیات حساب مشترک |
| PUT | `/api/v1/shared-accounts/:id` | وارد شده است | به روز رسانی حساب مشترک |
| GET | `/api/v1/shared-accounts/:id/repair-logs` | وارد شده است | سوابق تعمیر |

### گواهینامه ها

صدور، آپلود، تمدید و صادرات گواهی عمومی (ACME). صدور در یک گره خاص پشتیبانی می شود.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/certificates` | وارد شده است | فهرست کنید |
| POST | `/api/v1/certificates` | وارد شده است | ایجاد کنید |
| DELETE | `/api/v1/certificates/:id` | وارد شده است | حذف کنید |
| GET | `/api/v1/certificates/:id` | وارد شده است | جزئیات |
| PUT | `/api/v1/certificates/:id` | وارد شده است | به روز رسانی |
| GET | `/api/v1/certificates/:id/download` | وارد شده است | دانلود کنید |
| POST | `/api/v1/certificates/:id/renew` | وارد شده است | تمدید مشتری |
| GET | `/api/v1/certificates/check-port` | وارد شده است | پورت را بررسی کنید |
| GET | `/api/v1/certificates/detect-ip` | وارد شده است | پروب IP |
| GET | `/api/v1/certificates/export` | وارد شده است | صادرات |
| POST | `/api/v1/certificates/issue` | وارد شده است | موضوع. بدنه: `{"domains":["example.com","*.example.com"],"challenge_type":"dns-01","dns_channel_id":2,"key_type":"rsa","key_bits":4096}`. `challenge_type` پیش‌فرض http-01 است. `dns_channel_id` یک کانال DNS را انتخاب می کند (یک ارائه دهنده DNS پیکربندی شده + اطلاعات کاربری) |
| POST | `/api/v1/certificates/issue-on-node` | وارد شده است | مشکل به گره |
| POST | `/api/v1/certificates/upload` | وارد شده است | آپلود کنید |

### کانال های گواهی

پیکربندی کانال ACME (CA).

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/cert-channels` | وارد شده است | فهرست کنید |
| POST | `/api/v1/cert-channels` | وارد شده است | ایجاد کنید |
| DELETE | `/api/v1/cert-channels/:id` | وارد شده است | حذف کنید |
| PUT | `/api/v1/cert-channels/:id` | وارد شده است | به روز رسانی |
| GET | `/api/v1/cert-channels/providers` | وارد شده است | ارائه دهنده |

### پیکربندی EAB

اعتبارنامه های الزام آور حساب های خارجی مانند ZeroSSL / Google.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/eab-configs` | وارد شده است | فهرست کنید |
| POST | `/api/v1/eab-configs` | وارد شده است | ایجاد کنید |
| DELETE | `/api/v1/eab-configs/:id` | وارد شده است | حذف کنید |
| PUT | `/api/v1/eab-configs/:id` | وارد شده است | به روز رسانی |
| POST | `/api/v1/eab-configs/fetch-google` | وارد شده است | Google EAB را از GCP واکشی کنید |
| POST | `/api/v1/eab-configs/fetch-zerossl` | وارد شده است | ZeroSSL EAB را با یک ایمیل با یک کلیک واکشی کنید |

### حساب های ACME

مدیریت حساب ACME سمت پنل (ایمیل تماس).

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/acme-accounts` | وارد شده است | فهرست کنید |
| POST | `/api/v1/acme-accounts` | وارد شده است | ایجاد کنید |
| DELETE | `/api/v1/acme-accounts/:id` | وارد شده است | حذف کنید |
| PUT | `/api/v1/acme-accounts/:id` | وارد شده است | به روز رسانی |
| GET | `/api/v1/acme-accounts/default` | وارد شده است | دریافت پیش فرض ها |

### CA داخلی

پانل داخلی CA: صدور / تمدید / ابطال گواهی نماینده، صادرات CRL، صف توزیع.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/ca/agent-certs` | وارد شده است | لیست گواهی نمایندگی |
| POST | `/api/v1/ca/agent-certs/:server_id/renew` | وارد شده است | تمدید گواهی نمایندگی |
| GET | `/api/v1/ca/distribution-queue` | وارد شده است | صف توزیع گواهی |
| DELETE | `/api/v1/ca/distribution-queue/:id` | وارد شده است | حذف ورودی صف توزیع |
| POST | `/api/v1/ca/distribution-queue/:id/retry` | وارد شده است | توزیع گواهی را دوباره امتحان کنید |
| GET | `/api/v1/ca/revocations` | وارد شده است | لیست ابطال |
| GET | `/api/v1/ca/revocations/export` | وارد شده است | صادرات CRL |
| GET | `/api/v1/ca/status` | وارد شده است | وضعیت CA |

### قوانین جغرافیایی

همگام سازی و تحویل منابع و فایل های داده geoip/geosite.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/geo-rules/files` | وارد شده است | لیست فایل |
| DELETE | `/api/v1/geo-rules/files/:fileId` | وارد شده است | حذف فایل |
| PUT | `/api/v1/geo-rules/files/:fileId/auto-sync` | وارد شده است | همگام‌سازی خودکار را تغییر دهید |
| POST | `/api/v1/geo-rules/files/:fileId/download` | وارد شده است | دانلود کنید |
| GET | `/api/v1/geo-rules/files/:fileId/download-file` | وارد شده است | دانلود فایل |
| GET | `/api/v1/geo-rules/github-assets` | وارد شده است | دارایی های مخزن GitHub را واکشی کنید |
| POST | `/api/v1/geo-rules/push-all` | وارد شده است | فشار به تمام گره ها |
| POST | `/api/v1/geo-rules/seed` | وارد شده است | داده های پیش فرض را راه اندازی کنید |
| GET | `/api/v1/geo-rules/sources` | وارد شده است | فهرست منابع داده |
| POST | `/api/v1/geo-rules/sources` | وارد شده است | ایجاد منبع داده |
| DELETE | `/api/v1/geo-rules/sources/:sourceId` | وارد شده است | حذف منبع داده |
| PUT | `/api/v1/geo-rules/sources/:sourceId` | وارد شده است | منبع داده را به روز کنید |
| GET | `/api/v1/geo-rules/sources/:sourceId/assets` | وارد شده است | واکشی دارایی‌های انتشار |
| POST | `/api/v1/geo-rules/sources/:sourceId/files` | وارد شده است | فایل اضافه کنید |
| PUT | `/api/v1/geo-rules/sources/:sourceId/schedule` | وارد شده است | همگام سازی برنامه ریزی شده را تغییر دهید |
| POST | `/api/v1/geo-rules/sources/:sourceId/select` | وارد شده است | فایل را انتخاب کنید |
| POST | `/api/v1/geo-rules/sources/:sourceId/sync` | وارد شده است | این منبع داده را همگام سازی کنید |
| GET | `/api/v1/geo-rules/status` | وارد شده است | وضعیت |
| POST | `/api/v1/geo-rules/sync-all` | وارد شده است | همگام سازی همه |
| GET | `/api/v1/geo-rules/sync-status/:key` | وارد شده است | همگام سازی پیشرفت |

### کتابخانه قوانین WAF

قوانین جهانی WAF: منبع GitHub، همگام سازی، تحویل به همه گره ها.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/waf/github-source` | وارد شده است | منبع داده GitHub |
| PUT | `/api/v1/waf/github-source` | وارد شده است | منبع داده GitHub را ذخیره کنید |
| POST | `/api/v1/waf/push-all` | وارد شده است | فشار به تمام گره ها |
| GET | `/api/v1/waf/rules` | وارد شده است | لیست قوانین |
| DELETE | `/api/v1/waf/rules/:name` | وارد شده است | حذف قانون |
| GET | `/api/v1/waf/rules/:name` | وارد شده است | قاعده بگیر |
| PUT | `/api/v1/waf/rules/:name` | وارد شده است | قانون ذخیره |
| POST | `/api/v1/waf/sync-all` | وارد شده است | همگام سازی همه |
| POST | `/api/v1/waf/sync-github` | وارد شده است | همگام سازی از GitHub |

### مدیریت لاگ

خواندن و پخش گزارش گره/هسته، سیاست‌های پاکسازی، کنترل گزارش‌نویس.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| POST | `/api/v1/log-manage/cleanup-all` | وارد شده است | حالا همه چیز را پاک کن |
| GET | `/api/v1/log-manage/cleanup-rules` | وارد شده است | فهرست قوانین پاکسازی |
| PUT | `/api/v1/log-manage/cleanup-rules/:id` | وارد شده است | قانون پاکسازی را به روز کنید |
| POST | `/api/v1/log-manage/cleanup-table/:table` | وارد شده است | اکنون جدول مشخص شده را تمیز کنید |
| GET | `/api/v1/log-manage/core-log-output` | وارد شده است | پیکربندی خروجی گزارش اصلی |
| PUT | `/api/v1/log-manage/core-log-output` | وارد شده است | پیکربندی خروجی گزارش هسته را ذخیره کنید |
| GET | `/api/v1/log-manage/core/:type/stdout-log` | وارد شده است | گزارش های اصلی stdout را بخوانید |
| GET | `/api/v1/log-manage/core/:type/stdout-stream` | وارد شده است | استریم core stdout |
| POST | `/api/v1/log-manage/discover` | وارد شده است | فایل های گزارش گره را کشف کنید |
| GET | `/api/v1/log-manage/log-output` | وارد شده است | پیکربندی خروجی ورود به سیستم |
| PUT | `/api/v1/log-manage/log-output` | وارد شده است | پیکربندی خروجی گزارش را ذخیره کنید |
| POST | `/api/v1/log-manage/log-output/release/:type` | وارد شده است | log stdout takeover را آزاد کنید |
| POST | `/api/v1/log-manage/log-output/takeover/:type` | وارد شده است | Log stdout را در اختیار بگیرید |
| GET | `/api/v1/log-manage/nodes/:serverId/logs` | وارد شده است | لیست فایل گزارش گره |
| GET | `/api/v1/log-manage/nodes/:serverId/logs/:type` | وارد شده است | گزارش‌های گره را بخوانید |
| POST | `/api/v1/log-manage/nodes/:serverId/logs/:type/rotate` | وارد شده است | چرخش سیاهههای مربوط به گره |
| GET | `/api/v1/log-manage/nodes/:serverId/logs/:type/stream` | وارد شده است | گزارش‌های گره جریان |
| GET | `/api/v1/log-manage/policies` | وارد شده است | لیست سیاست |
| POST | `/api/v1/log-manage/policies` | وارد شده است | ایجاد خط مشی |
| DELETE | `/api/v1/log-manage/policies/:id` | وارد شده است | حذف خط مشی |
| GET | `/api/v1/log-manage/policies/:id` | وارد شده است | جزئیات خط مشی |
| PUT | `/api/v1/log-manage/policies/:id` | وارد شده است | سیاست به روز رسانی |
| GET | `/api/v1/log-manage/policies/:id/jobs` | وارد شده است | لیست کار پاکسازی |
| GET | `/api/v1/log-manage/policies/:id/log` | وارد شده است | لاگ ها را بخوانید |
| GET | `/api/v1/log-manage/policies/:id/log-stream` | وارد شده است | گزارش های جریان |
| POST | `/api/v1/log-manage/policies/:id/rotate` | وارد شده است | چرخش |
| GET | `/api/v1/log-manage/proxy/:type/stdout-log` | وارد شده است | گزارش های اصلی stdout را بخوانید |
| GET | `/api/v1/log-manage/proxy/:type/stdout-stream` | وارد شده است | استریم core stdout |
| GET | `/api/v1/log-manage/scheduler-config` | وارد شده است | پیکربندی زمان بندی |
| PUT | `/api/v1/log-manage/scheduler-config` | وارد شده است | پیکربندی زمان بندی را به روز کنید |
| GET | `/api/v1/log-manage/status` | وارد شده است | وضعیت |
| POST | `/api/v1/log-manage/sync` | وارد شده است | همگام سازی سیاست های پاکسازی |
| GET | `/api/v1/log-manage/writers` | وارد شده است | وضعیت نویسنده |
| PUT | `/api/v1/log-manage/writers/:type/config` | وارد شده است | پیکربندی رایتر را به روز کنید |
| GET | `/api/v1/log-manage/writers/:type/files` | وارد شده است | لیست فایل لاگ Writer |
| POST | `/api/v1/log-manage/writers/:type/start` | وارد شده است | نویسنده را شروع کنید |
| POST | `/api/v1/log-manage/writers/:type/stop` | وارد شده است | بس کن نویسنده |

### پشتیبان گیری

پایگاه داده/پشتیبان گیری کامل، بازیابی، کارهای برنامه ریزی شده، ذخیره سازی از راه دور FTP/S3/WebDAV.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| DELETE | `/api/v1/backups/:id` | مدیر | پشتیبان را حذف کنید |
| GET | `/api/v1/backups/:id/download` | مدیر | دانلود نسخه پشتیبان |
| POST | `/api/v1/backups/:id/restore` | مدیر | بازیابی از پشتیبان |
| POST | `/api/v1/backups/:id/upload-ftp` | مدیر | در FTP آپلود کنید |
| POST | `/api/v1/backups/:id/upload-s3` | مدیر | در S3 آپلود کنید |
| POST | `/api/v1/backups/:id/upload-webdav` | مدیر | در WebDAV آپلود کنید |
| POST | `/api/v1/backups/create` | مدیر | ایجاد پشتیبان کامل (با DB dump) |
| GET | `/api/v1/backups/list` | مدیر | لیست کامل پشتیبان |
| POST | `/api/v1/backups/remote-restore` | مدیر | بازیابی از راه دور |
| GET | `/api/v1/backups/restore-status` | مدیر | بازیابی پیشرفت |
| GET | `/api/v1/backups/schedules` | مدیر | لیست مشاغل برنامه ریزی شده |
| POST | `/api/v1/backups/schedules` | مدیر | ایجاد کار برنامه ریزی شده |
| DELETE | `/api/v1/backups/schedules/:id` | مدیر | کار برنامه ریزی شده را حذف کنید |
| PUT | `/api/v1/backups/schedules/:id` | مدیر | کار برنامه ریزی شده را به روز کنید |
| GET | `/api/v1/backups/storages` | مدیر | لیست تنظیمات ذخیره سازی |
| POST | `/api/v1/backups/storages` | مدیر | ایجاد تنظیمات ذخیره سازی |
| DELETE | `/api/v1/backups/storages/:id` | مدیر | تنظیمات ذخیره سازی را حذف کنید |
| PUT | `/api/v1/backups/storages/:id` | مدیر | پیکربندی ذخیره سازی را به روز کنید |
| GET | `/api/v1/backups/storages/:id/files` | مدیر | لیست فایل های پشتیبان گیری از راه دور |
| POST | `/api/v1/backups/upload-restore` | مدیر | آپلود پشتیبان و بازیابی |

### تنظیمات سیستم

پیکربندی و انتقال پایگاه داده، تنظیمات پانل، کلیدهای JWT، آزمایش‌های SMTP/Telegram، الگوهای اعلان، ارتقاء آنلاین (عمدتاً مدیر).

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| PUT | `/api/v1/system/bootstrap-finish` | مدیر | راه اندازی اولین بوت را کامل کنید |
| GET | `/api/v1/system/bootstrap-state` | مدیر | وضعیت اولین راه‌اندازی |
| GET | `/api/v1/system/config-file/:coreType` | مدیر | فایل پیکربندی هسته را بخوانید |
| GET | `/api/v1/system/db-config` | مدیر | پیکربندی پایگاه داده |
| PUT | `/api/v1/system/db-config` | مدیر | پیکربندی پایگاه داده را به روز کنید |
| POST | `/api/v1/system/db-config/migrate` | مدیر | انتقال داده های پایگاه داده |
| POST | `/api/v1/system/db-config/switch` | مدیر | تغییر پایگاه داده (با مهاجرت یک مرحله ای) |
| POST | `/api/v1/system/db-config/test` | مدیر | تست اتصال پایگاه داده |
| GET | `/api/v1/system/db-pool-stats` | مدیر | آمار استخر اتصال |
| GET | `/api/v1/system/db-stats` | مدیر | آمار پایگاه داده |
| GET | `/api/v1/system/db-stats/ping` | مدیر | کاوشگر پایگاه داده |
| POST | `/api/v1/system/db-stats/reset` | مدیر | بازنشانی آمار پایگاه داده |
| GET | `/api/v1/system/encryption-key-status` | مدیر | وضعیت کلید رمزگذاری |
| GET | `/api/v1/system/env-config` | مدیر | پیکربندی env |
| PUT | `/api/v1/system/env-config` | مدیر | پیکربندی .env را به روز کنید |
| POST | `/api/v1/system/generate-encryption-key` | مدیر | ایجاد کلید رمزگذاری |
| GET | `/api/v1/system/github-config` | مدیر | پیکربندی GitHub |
| PUT | `/api/v1/system/github-config` | مدیر | پیکربندی GitHub را به روز کنید |
| GET | `/api/v1/system/jwt-secret-status` | مدیر | وضعیت کلید JWT |
| GET | `/api/v1/system/maintenance` | عمومی | وضعیت حالت نگهداری (عمومی) |
| PUT | `/api/v1/system/maintenance` | مدیر | تغییر حالت نگهداری |
| POST | `/api/v1/system/migrate-data` | مدیر | انتقال داده ها |
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
| POST | `/api/v1/system/reality-targets/reset` | مدیر | هدف REALITY را به حالت پیش فرض بازنشانی کنید |
| POST | `/api/v1/system/regenerate-jwt-secret` | مدیر | کلید JWT را بازسازی کنید |
| GET | `/api/v1/system/releases` | مدیر | نسخه های موجود |
| POST | `/api/v1/system/restart` | مدیر | راه اندازی مجدد |
| POST | `/api/v1/system/rollback` | مدیر | بازگشت سیستم |
| GET | `/api/v1/system/settings` | مدیر | تنظیمات را بخوانید |
| PUT | `/api/v1/system/settings/batch` | مدیر | تنظیمات به روز رسانی انبوه |
| GET | `/api/v1/system/stats` | مدیر | وضعیت سیستم |
| GET | `/api/v1/system/subscription-billing-templates` | مدیر | الگوی صورتحساب اشتراک |
| PUT | `/api/v1/system/subscription-billing-templates` | مدیر | الگوی صورت‌حساب اشتراک را به‌روزرسانی کنید |
| POST | `/api/v1/system/telegram-chat-id` | مدیر | شناسه چت Telegram را دریافت کنید |
| POST | `/api/v1/system/test-smtp` | مدیر | تست SMTP |
| POST | `/api/v1/system/test-telegram` | مدیر | تست Telegram |
| POST | `/api/v1/system/update` | مدیر | ارتقاء سیستم |
| GET | `/api/v1/system/update-jobs` | مدیر | لیست مشاغل را ارتقا دهید |
| POST | `/api/v1/system/upload-mtls-ca` | مدیر | گواهی mTLS CA را آپلود کنید |

### ممیزی امنیتی

پرس و جوهای رویداد حسابرسی و صادرات.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/security/audit-events` | وارد شده است | لیست رویدادهای حسابرسی |
| GET | `/api/v1/security/audit-events/export` | وارد شده است | رویدادهای حسابرسی صادراتی |

### معیارها

سنجه های انبوه پانل (گره ها/گواهینامه ها/سرویس ها).

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/metrics/certificates` | وارد شده است | معیارهای گواهی |
| GET | `/api/v1/metrics/nodes` | وارد شده است | معیارهای گره |
| GET | `/api/v1/metrics/overview` | وارد شده است | نمای کلی |

### قالب های پورتال

قالب های پورتال، پیکربندی فعال سازی و مدیریت کش.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/portal/active` | وارد شده است | قالب فعال |
| PUT | `/api/v1/portal/active` | وارد شده است | به روز رسانی قالب فعال |
| DELETE | `/api/v1/portal/cache` | وارد شده است | کش پورتال را پاک کنید |
| GET | `/api/v1/portal/cache` | وارد شده است | وضعیت حافظه پنهان |
| PUT | `/api/v1/portal/cache/dev-mode` | وارد شده است | حالت توسعه حافظه پنهان را تغییر دهید |
| POST | `/api/v1/portal/cache/rebuild` | وارد شده است | کش پورتال را بازسازی کنید |
| POST | `/api/v1/portal/reset` | وارد شده است | بازیابی قالب پیش فرض |
| GET | `/api/v1/portal/templates` | وارد شده است | فهرست کنید |
| POST | `/api/v1/portal/templates` | وارد شده است | ایجاد کنید |
| DELETE | `/api/v1/portal/templates/:id` | وارد شده است | حذف کنید |
| GET | `/api/v1/portal/templates/:id` | وارد شده است | جزئیات |
| PUT | `/api/v1/portal/templates/:id` | وارد شده است | به روز رسانی |
| POST | `/api/v1/portal/templates/:id/activate` | وارد شده است | قالب را فعال کنید |

> `/portal/templates` CRUD یک **پیشرفته API** است: رابط کاربری پورتال در درجه اول از `PUT /api/v1/portal/active` استفاده می کند
> (پیکربندی کامل سایت تک شات)؛ با این گروه چندین الگو را مدیریت/تغییر دهید. هر دو پشتیبانی می شوند و تضاد ندارند.

### نصب عامل

Node Agent اسکریپت را نصب و دانلود کنید (عمومی).

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| POST | `/api/v1/agent/bootstrap` | عمومی | ثبت اولین راه‌اندازی نماینده (گزارش‌های نماینده با توکن بوت استرپ؛ عمومی) |
| GET | `/api/v1/agent/releases` | مدیر | نسخه های موجود نماینده (برای صفحه ارتقاء) |
| GET | `/api/v1/agent/version` | مدیر | نسخه عامل فعلی در پانل |
| GET | `/api/v1/agent/download` | عمومی | عامل دانلود |
| GET | `/api/v1/agent/install` | عمومی | دریافت اسکریپت نصب |

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

نقاط پایانی مشتری یکسان با پورتال. برای جزئیات بیشتر به `portal-api.en.md` مراجعه کنید.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/api/v1/client-auth/articles` | عمومی | فهرست مقالات |
| GET | `/api/v1/client-auth/articles/:slug` | عمومی | دریافت مقاله |
| GET | `/api/v1/client-auth/balance` | عمومی | تعادل را بدست آورید |
| GET | `/api/v1/client-auth/balance/transactions` | عمومی | تراکنش های موجود |
| GET | `/api/v1/client-auth/bills` | عمومی | لیست سفارش |
| POST | `/api/v1/client-auth/change-password` | عمومی | رمز عبور را تغییر دهید |
| POST | `/api/v1/client-auth/charge/create` | عمومی | ایجاد سفارش شارژ |
| GET | `/api/v1/client-auth/charge/orders` | عمومی | لیست سفارشات شارژ |
| GET | `/api/v1/client-auth/charge/orders/:id` | عمومی | جزئیات سفارش شارژ را دریافت کنید |
| POST | `/api/v1/client-auth/charge/redeem-card` | عمومی | بازخرید کارت |
| GET | `/api/v1/client-auth/charge/sites` | عمومی | لیست سایت های شارژ |
| GET | `/api/v1/client-auth/charge/status/:trade_no` | عمومی | استعلام وضعیت شارژ |
| GET | `/api/v1/client-auth/checktime` | عمومی | دریافت زمان سرور (همگام سازی ساعت) |
| GET | `/api/v1/client-auth/connection-info` | عمومی | پارامترهای اتصال طرح را دریافت کنید |
| GET | `/api/v1/client-auth/gifts` | عمومی | لیست هدایا |
| POST | `/api/v1/client-auth/gifts/:id/claim` | عمومی | ادعای هدیه |
| GET | `/api/v1/client-auth/invoices` | عمومی | لیست سفارش |
| GET | `/api/v1/client-auth/invoices/:id` | عمومی | جزئیات سفارش |
| GET | `/api/v1/client-auth/ip-whitelist` | عمومی | فهرست مجاز IP |
| DELETE | `/api/v1/client-auth/ip-whitelist/:id` | عمومی | از لیست مجاز IP حذف کنید |
| POST | `/api/v1/client-auth/ip-whitelist/auto-add` | عمومی | افزودن خودکار IP |
| POST | `/api/v1/client-auth/login` | عمومی | وارد شوید |
| POST | `/api/v1/client-auth/logout` | عمومی | از سیستم خارج شوید |
| POST | `/api/v1/client-auth/logs/package` | عمومی | ایجاد بسته گزارش |
| GET | `/api/v1/client-auth/logs/packages` | عمومی | فهرست بسته‌های گزارش |
| DELETE | `/api/v1/client-auth/logs/packages/:id` | عمومی | بسته گزارش را حذف کنید |
| GET | `/api/v1/client-auth/logs/packages/:id` | عمومی | بسته گزارش را دریافت کنید |
| GET | `/api/v1/client-auth/logs/packages/:id/download` | عمومی | دانلود بسته گزارش |
| GET | `/api/v1/client-auth/logs/types` | عمومی | بررسی سلامت بسته نرم افزاری گزارش |
| GET | `/api/v1/client-auth/me` | عمومی | کاربر فعلی وارد شده را دریافت کنید |
| GET | `/api/v1/client-auth/nodes` | عمومی | لیست گره های من |
| GET | `/api/v1/client-auth/nodes/:id/config` | عمومی | پیکربندی گره |
| GET | `/api/v1/client-auth/nodes/:id/link` | عمومی | پیوند اشتراک گره |
| GET | `/api/v1/client-auth/nodes/:id/qrcode` | عمومی | کد QR گره |
| GET | `/api/v1/client-auth/oauth/:provider` | عمومی | مجاز کردن |
| GET | `/api/v1/client-auth/oauth/:provider/callback` | عمومی | پاسخ به تماس |
| GET | `/api/v1/client-auth/oauth/providers` | عمومی | وضعیت ارائه دهنده |
| GET | `/api/v1/client-auth/orders` | عمومی | لیست سفارش |
| POST | `/api/v1/client-auth/orders` | عمومی | نظم ایجاد کنید |
| GET | `/api/v1/client-auth/orders/:id` | عمومی | جزئیات سفارش |
| GET | `/api/v1/client-auth/plans` | عمومی | لیست طرح |
| GET | `/api/v1/client-auth/plans/:id` | عمومی | جزئیات طرح |
| GET | `/api/v1/client-auth/profile` | عمومی | دریافت نمایه |
| PUT | `/api/v1/client-auth/profile` | عمومی | به روز رسانی نمایه |
| POST | `/api/v1/client-auth/profile/bind-oauth` | عمومی | اتصال حساب شخص ثالث |
| DELETE | `/api/v1/client-auth/profile/unbind-oauth/:provider` | عمومی | لغو پیوند حساب شخص ثالث |
| GET | `/api/v1/client-auth/referral/code` | عمومی | کد تایید ارجاع را دریافت کنید |
| GET | `/api/v1/client-auth/referral/records` | عمومی | فهرست سوابق ارجاع |
| GET | `/api/v1/client-auth/referral/stats` | عمومی | آمار ارجاع را دریافت کنید |
| POST | `/api/v1/client-auth/referral/withdraw` | عمومی | درخواست برداشت |
| GET | `/api/v1/client-auth/referral/withdrawals` | عمومی | فهرست برداشت های ارجاعی |
| POST | `/api/v1/client-auth/refresh` | عمومی | تازه کردن |
| POST | `/api/v1/client-auth/register` | عمومی | ثبت نام کنید |
| POST | `/api/v1/client-auth/reset-subscription` | عمومی | اشتراک را بازنشانی کنید |
| POST | `/api/v1/client-auth/rotate-token` | عمومی | چرخاندن نشانه اشتراک |
| POST | `/api/v1/client-auth/self-unban` | عمومی | سلف سرویس لغو ممنوعیت |
| PUT | `/api/v1/client-auth/settings` | عمومی | تنظیمات را به روز کنید |
| POST | `/api/v1/client-auth/settings/auto-add-ip` | عمومی | افزودن خودکار IP |
| GET | `/api/v1/client-auth/settings/whitelist` | عمومی | فهرست مجاز IP |
| DELETE | `/api/v1/client-auth/settings/whitelist/:id` | عمومی | از لیست مجاز IP حذف کنید |
| GET | `/api/v1/client-auth/shared-accounts` | عمومی | فهرست حساب های مشترک |
| GET | `/api/v1/client-auth/shared-accounts/:id` | عمومی | حساب مشترک دریافت کنید |
| POST | `/api/v1/client-auth/shared-accounts/:id/apply` | عمومی | درخواست حساب مشترک |
| POST | `/api/v1/client-auth/shared-accounts/:id/repair` | عمومی | درخواست تعمیر |
| POST | `/api/v1/client-auth/ticket` | عمومی | صدور بلیط یک بار کوتاه مدت |
| GET | `/api/v1/client-auth/tickets` | عمومی | لیست بلیط ها |
| POST | `/api/v1/client-auth/tickets` | عمومی | بلیط ایجاد کنید |
| GET | `/api/v1/client-auth/tickets/:id` | عمومی | بلیط بگیر |
| POST | `/api/v1/client-auth/tickets/:id/close` | عمومی | بستن بلیط |
| POST | `/api/v1/client-auth/tickets/:id/reply` | عمومی | پاسخ به بلیط |
| GET | `/api/v1/client-auth/traffic-history` | عمومی | تاریخچه ترافیک |
| GET | `/api/v1/client-auth/traffic-packs` | عمومی | لیست بسته ترافیک |
| POST | `/api/v1/client-auth/traffic-packs/buy` | عمومی | خرید پک ترافیک |

### تحویل اشتراک

نقطه ورود عمومی برای مشتریان برای دریافت اشتراک. `subscription.en.md` را ببینید.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/sub/:token` | عمومی | خدمات اشتراک |

### گردش کار پیکربندی زمان اجرا

تصاویر فوری پیکربندی / اجرای خشک / اعمال / برگشت (`apply`، `rollback` و `disable-proxy` نیاز به سرپرست دارند).

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| POST | `/runtime/apply` | عمومی | درخواست کنید |
| GET | `/runtime/apply-jobs` | وارد شده است | فهرست مشاغل |
| GET | `/runtime/apply-jobs/:id` | وارد شده است | کار پیدا کن |
| POST | `/runtime/apply-jobs/:id/cancel` | عمومی | کار را لغو کنید |
| POST | `/runtime/apply-jobs/:id/retry` | عمومی | دوباره کار را امتحان کنید |
| POST | `/runtime/disable-proxy` | عمومی | غیر فعال کردن پروکسی |
| POST | `/runtime/dry-run` | وارد شده است | اجرای خشک |
| GET | `/runtime/nodes` | وارد شده است | لیست گره ها |
| GET | `/runtime/overview` | وارد شده است | نمای کلی |
| POST | `/runtime/rollback` | عمومی | به عقب برگرد |
| GET | `/runtime/snapshots` | وارد شده است | فهرست عکس های فوری |
| POST | `/runtime/snapshots` | وارد شده است | ایجاد عکس فوری |
| DELETE | `/runtime/snapshots/:id` | عمومی | حذف عکس فوری |
| GET | `/runtime/snapshots/:id` | وارد شده است | دریافت عکس فوری |
| POST | `/runtime/verify` | وارد شده است | تأیید کنید |

### خط‌مشی‌های دسترسی به خدمات (بازنشسته)

> `/services/:id/access-policies` و `/services/:id/ip-bans` در 20/10/2026 بازنشسته شدند:
> هیچ تماس مدیریتی وجود ندارد و هیچ مسیر اتصالی این خط مشی ها را مصرف نمی کند.

### نظارت بر سهمیه ترافیک

نمای نظارت فقط خواندنی از سهمیه ها و استفاده.

| روش | مسیر | اجازه | توضیحات |
|---|---|---|---|
| GET | `/traffic/quotas` | وارد شده است | لیست سهمیه |
| GET | `/traffic/quotas/:id/usage` | وارد شده است | استفاده از سهمیه |
| GET | `/traffic/reset-logs` | وارد شده است | رکوردهای بازنشانی سهمیه |
| GET | `/traffic/usage-history` | وارد شده است | سابقه استفاده |

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
