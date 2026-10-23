<!-- source: admin-api.md | locale: ar | updated: 2026-10-23 -->
# المشرف API

> اللغات: [简体中文](admin-api.md) · [English](admin-api.en.md) · [繁體中文](admin-api.zh-TW.md) · [Русский](admin-api.ru.md) · [日本語](admin-api.ja.md) · [한국어](admin-api.ko.md) · [Deutsch](admin-api.de.md) · [Français](admin-api.fr.md) · [Español](admin-api.es.md) · [Português](admin-api.pt.md) · [Türkçe](admin-api.tr.md) · [Bahasa Indonesia](admin-api.id.md) · [ไทย](admin-api.th.md) · [Tiếng Việt](admin-api.vi.md) · العربية · [فارسی](admin-api.fa.md)

جدول توجيه المشرف الكامل API ** لتطوير اللوحة / الأتمتة / التكامل **.

- قاعدة URL: `https://<panel-domain>/api/v1`
- المصادقة: `Authorization: Bearer <admin_token>` (من `POST /api/v1/auth/login`)
- تنسيقات الاستجابة/الخطأ الشائعة واصطلاحات الحد الأقصى للمعدل: راجع [README.md](./README.ar.md)
- مصدر الحقيقة: `internal/api/router/router.go` (739 مسارًا إداريًا)؛ إذا لم يتفق هذا الجدول، **الرمز يفوز**

## علامات الإذن

| علامة | معنى |
|---|---|
| عام | لا يلزم تسجيل الدخول (قد يستمر تطبيق حراسة الصيانة / تحديد السعر) |
| عام · السعر محدود | لا يوجد تسجيل دخول، مع حد سعر مخصص |
| تم تسجيل الدخول | يتطلب رمزًا مميزًا للمسؤول عند تسجيل الدخول (`AuthRequired`) |
| المشرف | يتطلب الدور ≥ `admin` (`RequireRole(RoleAdmin)`) |
| المشرف الفائق | يتطلب الدور = `super_admin` |

مستويات الدور: `viewer(10) < operator(20) < admin(30) < super_admin(40)`؛ تقوم الأدوار غير الكافية بإرجاع 403 `INSUFFICIENT_ROLE`.

---

## بداية سريعة (أمثلة حقيقية)

تأتي هذه الأمثلة من استجابات محلية حقيقية؛ يتم تنقيح الرموز والبيانات.

### 1. قم بتسجيل الدخول واحصل على رمز مميز

```bash
curl -X POST https://<panel-domain>/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"your-password"}'
```

الرد:

```json
{
  "data": {
    "access_token": "<JWT, valid 24h>",
    "refresh_token": "<JWT, valid 7 days>",
    "user": { "id": 1, "role": "super_admin", "username": "admin" }
  }
}
```

### 2. اتصل بنقطة نهاية العمل باستخدام الرمز المميز

```bash
curl https://<panel-domain>/api/v1/clients?limit=1 \
  -H 'Authorization: Bearer <access_token>'
```

الاستجابة (تستخدم القوائم البنية `items` + `total`):

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

### 3. أمثلة على الاستجابة للخطأ

**401 غير مصادق عليه:**

```json
{
  "code": "MISSING_TOKEN",
  "message": "missing authorization header",
  "request_id": "a02cfdf9-6be0-46be-8584-824caab34dc7",
  "can_retry": false,
  "requires_action": false
}
```

**لم يتم العثور على مورد 404:**

```json
{
  "code": "CLIENT_NOT_FOUND",
  "message": "client not found",
  "request_id": "099949bd-4ded-468d-a6f8-f8e798af2273",
  "can_retry": false,
  "requires_action": false
}
```

### 4. ترقيم الصفحات

تستخدم كافة نقاط نهاية القائمة `limit` (حجم الصفحة، الافتراضي 20) و`offset` (المستند إلى 0) وترجع `{items, total, limit, offset}`. تُرجع بعض نقاط النهاية القديمة مصفوفة مجردة (مذكورة في الجدول).

---

## المحتويات

- [مصادقة المسؤول والأمن](#مصادقة-المسؤول-والأمن)
- [حسابات المشرف](#حسابات-المشرف)
- [إدارة العملاء](#إدارة-العملاء)
- [قوالب الخطة](#قوالب-الخطة)
- [متغيرات الخطة](#متغيرات-الخطة)
- [حزم المرور](#حزم-المرور)
- [كوبونات](#كوبونات)
- [مجموعات العقدة](#مجموعات-العقدة)
- [خوادم العقدة](#خوادم-العقدة)
- [سجل تدقيق العقدة](#سجل-تدقيق-العقدة)
- [العقد (النموذج القديم، متقاعد)](#العقد-النموذج-القديم-متقاعد)
- [نظرة عامة على المنطقة](#نظرة-عامة-على-المنطقة)
- [الاشتراكات](#الاشتراكات)
- [تنسيقات الاشتراك](#تنسيقات-الاشتراك)
- [نقاط نهاية الاشتراك](#نقاط-نهاية-الاشتراك)
- [مشاركة المجالات](#مشاركة-المجالات)
- [Cloudflare IP](#cloudflare-ip)
- [تكوين الاشتراك (Mihomo/Clash)](#تكوين-الاشتراك-mihomoclash)
- [مواقع إعادة الشحن](#مواقع-إعادة-الشحن)
- [أوامر إعادة الشحن](#أوامر-إعادة-الشحن)
- [بطاقات إعادة الشحن](#بطاقات-إعادة-الشحن)
- [أحداث رد الاتصال](#أحداث-رد-الاتصال)
- [إعادة محاولة رد الاتصال](#إعادة-محاولة-رد-الاتصال)
- [رصيد العميل](#رصيد-العميل)
- [الهدايا](#الهدايا)
- [تكوين الإحالة](#تكوين-الإحالة)
- [عمليات سحب الإحالة](#عمليات-سحب-الإحالة)
- [قوائم IP المسموح بها (ملخص)](#قوائم-ip-المسموح-بها-ملخص)
- [روابط OAuth الخاصة بالعميل](#روابط-oauth-الخاصة-بالعميل)
- [التذاكر (المسؤول)](#التذاكر-المسؤول)
- [مقالات](#مقالات)
- [الحسابات المشتركة](#الحسابات-المشتركة)
- [الشهادات](#الشهادات)
- [قنوات الشهادة](#قنوات-الشهادة)
- [التكوين EAB](#التكوين-eab)
- [حسابات ACME](#حسابات-acme)
- [المدمج في كاليفورنيا](#المدمج-في-كاليفورنيا)
- [القواعد الجغرافية](#القواعد-الجغرافية)
- [مكتبة القواعد WAF](#مكتبة-القواعد-waf)
- [إدارة السجل](#إدارة-السجل)
- [النسخ الاحتياطية](#النسخ-الاحتياطية)
- [إعدادات النظام](#إعدادات-النظام)
- [التدقيق الأمني](#التدقيق-الأمني)
- [المقاييس](#المقاييس)
- [قوالب البوابة](#قوالب-البوابة)
- [تركيب الوكيل](#تركيب-الوكيل)
- [تهمة إدخال رد الاتصال](#تهمة-إدخال-رد-الاتصال)
- [الوارد (مع العقد)](#الوارد-مع-العقد)
- [التكوين العام](#التكوين-العام)
- [مصادقة العميل والأعمال (نفس المجموعة في محرك الإدارة)](#مصادقة-العميل-والأعمال-نفس-المجموعة-في-محرك-الإدارة)
- [تسليم الاشتراك](#تسليم-الاشتراك)
- [سير عمل تكوين وقت التشغيل](#سير-عمل-تكوين-وقت-التشغيل)
- [سياسات الوصول إلى الخدمة (متقاعدة)](#سياسات-الوصول-إلى-الخدمة-متقاعدة)
- [مراقبة الحصص المرورية](#مراقبة-الحصص-المرورية)
- [الفحوصات الصحية](#الفحوصات-الصحية)
- [صفحة البوابة](#صفحة-البوابة)
- [تحديات ACME](#تحديات-acme)

---

### مصادقة المسؤول والأمن

تسجيل الدخول، تسجيل الخروج، تغيير كلمة المرور، 2FA (TOTP)، Passkey (WebAuthn) وتذاكر لمرة واحدة للوحة الإدارة.

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| POST | `/api/v1/auth/2fa/disable` | تم تسجيل الدخول | تعطيل |
| POST | `/api/v1/auth/2fa/enable` | تم تسجيل الدخول | تمكين |
| POST | `/api/v1/auth/2fa/recovery-codes` | تم تسجيل الدخول | إعادة إنشاء رموز الاسترداد |
| POST | `/api/v1/auth/2fa/setup` | تم تسجيل الدخول | الإعدادات |
| GET | `/api/v1/auth/2fa/status` | تم تسجيل الدخول | الحالة |
| POST | `/api/v1/auth/change-password` | تم تسجيل الدخول | تغيير كلمة المرور |
| POST | `/api/v1/auth/login` | عام | تسجيل الدخول |
| POST | `/api/v1/auth/login/2fa` | عام | تسجيل الدخول بالرمز الديناميكي (2FA) |
| GET | `/api/v1/auth/login/2fa-status` | عام | تحقق مما إذا كان الحساب ممكّنًا بالمصادقة الثنائية (2FA). |
| POST | `/api/v1/auth/logout` | تم تسجيل الدخول | تسجيل الخروج |
| GET | `/api/v1/auth/me` | تم تسجيل الدخول | المستخدم الحالي الذي قام بتسجيل الدخول |
| GET | `/api/v1/auth/passkey/credentials` | تم تسجيل الدخول | قائمة بيانات الاعتماد Passkey |
| DELETE | `/api/v1/auth/passkey/credentials/:id` | تم تسجيل الدخول | حذف بيانات اعتماد Passkey |
| POST | `/api/v1/auth/passkey/login/begin` | عام | ابدأ تسجيل الدخول |
| POST | `/api/v1/auth/passkey/login/finish` | عام | إكمال تسجيل الدخول |
| POST | `/api/v1/auth/passkey/register/begin` | تم تسجيل الدخول | ابدأ التسجيل |
| POST | `/api/v1/auth/passkey/register/finish` | تم تسجيل الدخول | التسجيل الكامل |
| GET | `/api/v1/auth/passkey/rpids` | تم تسجيل الدخول | القائمة المسموح بها لمعرف RP |
| POST | `/api/v1/auth/passkey/rpids` | تم تسجيل الدخول | قم بتحديث القائمة المسموح بها لمعرف RP |
| GET | `/api/v1/auth/passkey/status` | عام | الحالة |
| POST | `/api/v1/auth/refresh` | عام | تحديث |
| POST | `/api/v1/auth/ticket` | تم تسجيل الدخول | إصدار تذكرة قصيرة الأجل لمرة واحدة |

### حسابات المشرف

حسابات مستخدمي لوحة الإدارة. الإدراج يتطلب المشرف. يتطلب الإنشاء/التحديث/الحذف مشرفًا متميزًا (يمنع تصعيد الامتيازات).

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| GET | `/api/v1/users` | المشرف | قائمة |
| POST | `/api/v1/users` | المشرف الفائق | إنشاء |
| DELETE | `/api/v1/users/:id` | المشرف الفائق | حذف |
| PUT | `/api/v1/users/:id` | المشرف الفائق | تحديث |

### إدارة العملاء

CRUD، تعليق/استئناف، ترقية/خفض مستوى، التجديد وإعادة تعيين الاستخدام للمشتركين (حسابات البوابة).

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| GET | `/api/v1/clients` | تم تسجيل الدخول | قائمة العملاء |
| POST | `/api/v1/clients` | تم تسجيل الدخول | إنشاء العميل. الجسم: `{"email":"user@example.com","password":"plaintext-password","plan_id":1,"quota_bytes":10737418240,"expires_at":"2026-12-31T00:00:00Z","remark":""}`. مطلوب `plan_id`؛ قد يكون `password` فارغًا (يمكن للعملاء تعيينه لاحقًا في البوابة) |
| DELETE | `/api/v1/clients/:id` | تم تسجيل الدخول | حذف العميل |
| GET | `/api/v1/clients/:id` | تم تسجيل الدخول | احصل على العميل |
| PUT | `/api/v1/clients/:id` | تم تسجيل الدخول | تحديث العميل |
| POST | `/api/v1/clients/:id/downgrade` | تم تسجيل الدخول | خفض مستوى العميل |
| GET | `/api/v1/clients/:id/orders` | تم تسجيل الدخول | قائمة طلبات العملاء |
| POST | `/api/v1/clients/:id/renew` | تم تسجيل الدخول | تجديد العميل |
| POST | `/api/v1/clients/:id/reset-usage` | تم تسجيل الدخول | إعادة ضبط الاستخدام |
| POST | `/api/v1/clients/:id/restore` | تم تسجيل الدخول | استئناف العميل |
| POST | `/api/v1/clients/:id/suspend` | تم تسجيل الدخول | تعليق العميل |
| POST | `/api/v1/clients/:id/upgrade` | تم تسجيل الدخول | ترقية العميل |

### قوالب الخطة

الخطط (المتغيرات ودورات الفوترة والأسعار وحصص حركة المرور) وتفويض العقدة على مستوى القالب.

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| GET | `/api/v1/plan-templates` | تم تسجيل الدخول | قائمة القالب |
| POST | `/api/v1/plan-templates` | تم تسجيل الدخول | إنشاء قالب |
| DELETE | `/api/v1/plan-templates/:id` | تم تسجيل الدخول | حذف القالب |
| GET | `/api/v1/plan-templates/:id` | تم تسجيل الدخول | تفاصيل القالب |
| PUT | `/api/v1/plan-templates/:id` | تم تسجيل الدخول | تحديث القالب |
| GET | `/api/v1/plan-templates/:id/accessible-nodes` | تم تسجيل الدخول | الحصول على العقد المتاحة |
| GET | `/api/v1/plan-templates/:id/billing-node-options` | تم تسجيل الدخول | احصل على خيارات العقدة القابلة للفوترة |
| PUT | `/api/v1/plan-templates/:id/node-groups` | تم تسجيل الدخول | تعيين مجموعات عقدة القالب |
| POST | `/api/v1/plan-templates/:id/sync-accounts` | تم تسجيل الدخول | مزامنة الحسابات مع القالب |
| GET | `/api/v1/plan-templates/:id/variants` | تم تسجيل الدخول | قائمة البديل |
| POST | `/api/v1/plan-templates/:id/variants` | تم تسجيل الدخول | إنشاء البديل |
| POST | `/api/v1/plan-templates/sync-all-accounts` | تم تسجيل الدخول | مزامنة كافة الحسابات |

### متغيرات الخطة

تحديث وحذف متغيرات الخطة (يتم دمج الإنشاء ضمن قوالب الخطة).

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| DELETE | `/api/v1/plan-variants/:id` | تم تسجيل الدخول | حذف البديل |
| PUT | `/api/v1/plan-variants/:id` | تم تسجيل الدخول | تحديث البديل |

### حزم المرور

حزم المرور التي يمكن شراؤها بشكل منفصل.

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| GET | `/api/v1/traffic-packs` | تم تسجيل الدخول | قائمة حزمة المرور |
| POST | `/api/v1/traffic-packs` | تم تسجيل الدخول | إنشاء حزمة المرور |
| DELETE | `/api/v1/traffic-packs/:id` | تم تسجيل الدخول | حذف حزمة المرور |
| PUT | `/api/v1/traffic-packs/:id` | تم تسجيل الدخول | تحديث حزمة المرور |

### كوبونات

إنشاء والتحقق من صحة واسترداد القسائم.

| طريقة | المسار | إذن | الوصف |
|---|---|---|---|
| GET | `/api/v1/promotions` | تم تسجيل الدخول | قائمة الكوبونات |
| POST | `/api/v1/promotions` | تم تسجيل الدخول | إنشاء قسيمة |
| DELETE | `/api/v1/promotions/:id` | تم تسجيل الدخول | حذف القسيمة |
| PUT | `/api/v1/promotions/:id` | تم تسجيل الدخول | تحديث القسيمة |
| POST | `/api/v1/promotions/validate` | تم تسجيل الدخول | التحقق من صحة القسيمة |

### مجموعات العقدة

تنظيم العقد في مجموعات تمت الإشارة إليها بواسطة الخطط.

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| GET | `/api/v1/node-groups` | تم تسجيل الدخول | قائمة مجموعات العقدة |
| POST | `/api/v1/node-groups` | تم تسجيل الدخول | إنشاء مجموعة العقدة |
| DELETE | `/api/v1/node-groups/:id` | تم تسجيل الدخول | حذف مجموعة العقدة |
| PUT | `/api/v1/node-groups/:id` | تم تسجيل الدخول | تحديث مجموعة العقدة |
| GET | `/api/v1/node-groups/:id/members` | تم تسجيل الدخول | قائمة أعضاء المجموعة |
| POST | `/api/v1/node-groups/:id/members` | تم تسجيل الدخول | إضافة عضو المجموعة |
| DELETE | `/api/v1/node-groups/:id/members/:mid` | تم تسجيل الدخول | إزالة عضو المجموعة |

### خوادم العقدة

الموارد الأساسية: العقدة CRUD، تسليم التكوين الأساسي (xray/sing-box)، تكوين الوارد/الصادر/نقطة النهاية، التحكم في العمليات، الشهادات، WAF، حدود المعدل، التنقل بين المنافذ، الوكلاء المتسلسلون، النسخ الاحتياطي/النسخ الاحتياطي الأساسي، تثبيت/ترقية الوكيل، التحكم في وقت التشغيل xray. العديد من المسارات الفرعية، مجمعة حسب عائلة المعالم.

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| GET | `/api/v1/node-servers` | المشرف | قائمة |
| POST | `/api/v1/node-servers` | المشرف | إنشاء |
| DELETE | `/api/v1/node-servers/:id` | المشرف | حذف |
| GET | `/api/v1/node-servers/:id` | المشرف | التفاصيل |
| PUT | `/api/v1/node-servers/:id` | المشرف | تحديث |
| GET | `/api/v1/node-servers/:id/binaries` | المشرف | القائمة الأساسية المثبتة |
| POST | `/api/v1/node-servers/:id/binaries/ech-generate` | المشرف | إنشاء زوج مفاتيح ECH |
| POST | `/api/v1/node-servers/:id/binaries/install` | المشرف | تثبيت/ترقية الأساسية |
| GET | `/api/v1/node-servers/:id/binaries/interfaces` | المشرف | قائمة واجهة الشبكة |
| POST | `/api/v1/node-servers/:id/binaries/mldsa65-key` | المشرف | قم بإنشاء مفتاح MLDSA65 |
| POST | `/api/v1/node-servers/:id/binaries/reality-key` | المشرف | إنشاء زوج مفاتيح REALITY |
| POST | `/api/v1/node-servers/:id/binaries/reality-public-key` | المشرف | اشتقاق المفتاح العام من المفتاح الخاص REALITY |
| GET | `/api/v1/node-servers/:id/binaries/releases` | المشرف | الإصدارات المتاحة |
| POST | `/api/v1/node-servers/:id/binaries/vlessenc` | المشرف | إنشاء معلمات التشفير VLESS |
| POST | `/api/v1/node-servers/:id/binaries/wg-key` | المشرف | قم بإنشاء مفتاح WireGuard |
| POST | `/api/v1/node-servers/:id/binaries/x25519-key` | المشرف | إنشاء زوج المفاتيح X25519 |
| GET | `/api/v1/node-servers/:id/bootstrap-cmd` | المشرف | الحصول على أمر bootstrap للوكيل |
| POST | `/api/v1/node-servers/:id/bootstrap-token` | المشرف | إنشاء رمز التمهيد |
| POST | `/api/v1/node-servers/:id/caddy/batch-update-log-enabled` | المشرف | مفاتيح سجل التحديث المجمع |
| POST | `/api/v1/node-servers/:id/caddy/batch-update-ports` | المشرف | منافذ التحديث بالجملة |
| POST | `/api/v1/node-servers/:id/caddy/generate-config` | المشرف | إنشاء التكوين |
| DELETE | `/api/v1/node-servers/:id/caddy/l4-rules` | المشرف | احذف جميع قواعد L4 |
| POST | `/api/v1/node-servers/:id/caddy/save-and-apply` | المشرف | حفظ وتطبيق |
| GET | `/api/v1/node-servers/:id/caddy/sites` | المشرف | قائمة الموقع |
| POST | `/api/v1/node-servers/:id/caddy/sites` | المشرف | إنشاء موقع |
| DELETE | `/api/v1/node-servers/:id/caddy/sites/:siteId` | المشرف | حذف الموقع |
| PUT | `/api/v1/node-servers/:id/caddy/sites/:siteId` | المشرف | تحديث الموقع |
| DELETE | `/api/v1/node-servers/:id/caddy/sni-rules` | المشرف | احذف جميع قواعد SNI |
| POST | `/api/v1/node-servers/:id/caddy/sni-rules/toggle` | المشرف | تبديل قواعد SNI |
| GET | `/api/v1/node-servers/:id/caddy/sni-settings` | المشرف | إعدادات SNI |
| POST | `/api/v1/node-servers/:id/caddy/sni-settings` | المشرف | احفظ إعدادات SNI |
| GET | `/api/v1/node-servers/:id/certificates` | المشرف | قائمة شهادات العقدة |
| DELETE | `/api/v1/node-servers/:id/certificates/:cid` | المشرف | إلغاء ربط الشهادة |
| POST | `/api/v1/node-servers/:id/certificates/:cid` | المشرف | ربط الشهادة |
| GET | `/api/v1/node-servers/:id/chain-config` | المشرف | إنشاء التكوين المتسلسل |
| GET | `/api/v1/node-servers/:id/chain-health` | المشرف | الحالة الصحية للسلسلة |
| POST | `/api/v1/node-servers/:id/control` | المشرف الفائق | عقدة التحكم الأساسية. الجسم: `{"core_type":"xray","action":"start"}` (الإجراء = البدء/الإيقاف/إعادة التشغيل). `core_type` يمكن أن يكون `xray`/`singbox` |
| GET | `/api/v1/node-servers/:id/degradation-status` | المشرف | الوضع المتدهور |
| POST | `/api/v1/node-servers/:id/distribute-cert` | المشرف | توزيع الشهادة |
| GET | `/api/v1/node-servers/:id/endpoint-configs` | المشرف | قائمة |
| POST | `/api/v1/node-servers/:id/endpoint-configs` | المشرف | إنشاء |
| DELETE | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | المشرف | حذف |
| GET | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | المشرف | التفاصيل |
| PUT | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | المشرف | تحديث |
| POST | `/api/v1/node-servers/:id/failover` | المشرف | تشغيل تجاوز الفشل لهذه العقدة |
| GET | `/api/v1/node-servers/:id/failover-status` | المشرف | حالة تجاوز الفشل |
| POST | `/api/v1/node-servers/:id/force-reconnect` | المشرف | فرض إعادة الاتصال |
| GET | `/api/v1/node-servers/:id/free-port` | المشرف | طلب منفذ مجاني |
| POST | `/api/v1/node-servers/:id/geo-rules/push` | المشرف | ادفع إلى هذه العقدة |
| GET | `/api/v1/node-servers/:id/inbound-configs` | المشرف | قائمة |
| POST | `/api/v1/node-servers/:id/inbound-configs` | المشرف | إنشاء |
| DELETE | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | المشرف | حذف |
| GET | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | المشرف | التفاصيل |
| PUT | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | المشرف | تحديث |
| GET | `/api/v1/node-servers/:id/inbound-configs/:cfgId/share-links` | المشرف | مشاركة الروابط والمقتطفات |
| POST | `/api/v1/node-servers/:id/inbound-configs/:cfgId/users/:email/pause` | المشرف | تعليق هذا المستخدم |
| POST | `/api/v1/node-servers/:id/inbound-configs/:cfgId/users/:email/resume` | المشرف | استعادة هذا المستخدم |
| POST | `/api/v1/node-servers/:id/inbound-configs/reset-all-traffic` | المشرف | إعادة ضبط كافة حركة المرور الواردة |
| GET | `/api/v1/node-servers/:id/install-script` | المشرف | احصل على البرنامج النصي لتثبيت الوكيل |
| GET | `/api/v1/node-servers/:id/metrics-history` | المشرف | الحصول على التاريخ المتري |
| GET | `/api/v1/node-servers/:id/mirror-status` | المشرف | حالة المرآة |
| POST | `/api/v1/node-servers/:id/nginx/batch-update-ports` | المشرف | منافذ التحديث بالجملة |
| POST | `/api/v1/node-servers/:id/nginx/generate-config` | المشرف | إنشاء التكوين |
| DELETE | `/api/v1/node-servers/:id/nginx/l4-rules` | المشرف | احذف جميع قواعد L4 |
| POST | `/api/v1/node-servers/:id/nginx/save-and-apply` | المشرف | حفظ وتطبيق |
| GET | `/api/v1/node-servers/:id/nginx/settings` | المشرف | قراءة الإعدادات |
| POST | `/api/v1/node-servers/:id/nginx/settings` | المشرف | حفظ الإعدادات |
| GET | `/api/v1/node-servers/:id/nginx/sites` | المشرف | قائمة الموقع |
| POST | `/api/v1/node-servers/:id/nginx/sites` | المشرف | إنشاء موقع |
| DELETE | `/api/v1/node-servers/:id/nginx/sites/:siteId` | المشرف | حذف الموقع |
| PUT | `/api/v1/node-servers/:id/nginx/sites/:siteId` | المشرف | تحديث الموقع |
| DELETE | `/api/v1/node-servers/:id/nginx/sni-rules` | المشرف | احذف جميع قواعد SNI |
| POST | `/api/v1/node-servers/:id/nginx/sni-rules/toggle` | المشرف | تبديل قواعد SNI |
| PUT | `/api/v1/node-servers/:id/node-type` | المشرف | تحديث نوع العقدة |
| GET | `/api/v1/node-servers/:id/online-users` | المشرف | مستخدمي الإنترنت |
| GET | `/api/v1/node-servers/:id/outbound-configs` | المشرف | قائمة |
| POST | `/api/v1/node-servers/:id/outbound-configs` | المشرف | إنشاء |
| DELETE | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | المشرف | حذف |
| GET | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | المشرف | التفاصيل |
| PUT | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | المشرف | تحديث |
| PUT | `/api/v1/node-servers/:id/outbound-configs/reorder` | المشرف | فرز |
| GET | `/api/v1/node-servers/:id/port-hop/rules` | المشرف | معاينة قاعدة التنقل بين المنافذ |
| GET | `/api/v1/node-servers/:id/protocols` | المشرف | قائمة البروتوكول |
| GET | `/api/v1/node-servers/:id/protocols/:coreType` | المشرف | تكوين البروتوكول |
| PUT | `/api/v1/node-servers/:id/protocols/:coreType` | المشرف | حفظ تكوين البروتوكول |
| GET | `/api/v1/node-servers/:id/protocols/:coreType/config-file` | المشرف | قراءة ملف التكوين الأساسي |
| GET | `/api/v1/node-servers/:id/protocols/:coreType/section/:section` | المشرف | قراءة قسم البروتوكول |
| PUT | `/api/v1/node-servers/:id/protocols/:coreType/section/:section` | المشرف | حفظ قسم البروتوكول |
| POST | `/api/v1/node-servers/:id/protocols/:coreType/validate` | المشرف | التحقق من صحة التكوين (التشغيل الجاف الأساسي الحقيقي) |
| GET | `/api/v1/node-servers/:id/proxy-status` | المشرف | حالة الوكيل عن بعد |
| POST | `/api/v1/node-servers/:id/proxy/batch-update-ports` | المشرف | منافذ التحديث بالجملة |
| GET | `/api/v1/node-servers/:id/proxy/config` | المشرف | قراءة التكوين |
| PUT | `/api/v1/node-servers/:id/proxy/config` | المشرف | تحديث التكوين |
| POST | `/api/v1/node-servers/:id/proxy/control` | المشرف | التحكم في تسجيل الوكيل المتسلسل |
| POST | `/api/v1/node-servers/:id/proxy/download` | المشرف | تحميل النسخة الأساسية الوكيل |
| POST | `/api/v1/node-servers/:id/proxy/generate-config` | المشرف | إنشاء التكوين |
| POST | `/api/v1/node-servers/:id/proxy/guardian` | المشرف | تحديث تكوين المشرف |
| POST | `/api/v1/node-servers/:id/proxy/install` | المشرف | تثبيت الوكيل |
| DELETE | `/api/v1/node-servers/:id/proxy/l4-rules` | المشرف | احذف جميع قواعد L4 |
| GET | `/api/v1/node-servers/:id/proxy/l4-settings` | المشرف | إعدادات L4 |
| POST | `/api/v1/node-servers/:id/proxy/l4-settings` | المشرف | حفظ إعدادات L4 |
| GET | `/api/v1/node-servers/:id/proxy/log-settings` | المشرف | إعدادات السجل |
| POST | `/api/v1/node-servers/:id/proxy/log-settings` | المشرف | حفظ إعدادات السجل |
| GET | `/api/v1/node-servers/:id/proxy/nginx-settings` | المشرف | إعدادات nginx |
| POST | `/api/v1/node-servers/:id/proxy/nginx-settings` | المشرف | احفظ إعدادات nginx |
| GET | `/api/v1/node-servers/:id/proxy/releases` | المشرف | الإصدارات المتاحة |
| POST | `/api/v1/node-servers/:id/proxy/reload` | المشرف | إعادة تحميل التكوين |
| POST | `/api/v1/node-servers/:id/proxy/restart` | المشرف | إعادة التشغيل |
| GET | `/api/v1/node-servers/:id/proxy/sites` | المشرف | قائمة الموقع |
| POST | `/api/v1/node-servers/:id/proxy/sites` | المشرف | إنشاء موقع |
| DELETE | `/api/v1/node-servers/:id/proxy/sites/:siteId` | المشرف | حذف الموقع |
| PUT | `/api/v1/node-servers/:id/proxy/sites/:siteId` | المشرف | تحديث الموقع |
| DELETE | `/api/v1/node-servers/:id/proxy/sni-rules` | المشرف | احذف جميع قواعد SNI |
| POST | `/api/v1/node-servers/:id/proxy/sni-rules/toggle` | المشرف | تبديل قواعد SNI |
| GET | `/api/v1/node-servers/:id/proxy/sni-settings` | المشرف | إعدادات SNI |
| POST | `/api/v1/node-servers/:id/proxy/sni-settings` | المشرف | احفظ إعدادات SNI |
| POST | `/api/v1/node-servers/:id/proxy/start` | المشرف | ابدأ |
| GET | `/api/v1/node-servers/:id/proxy/status` | المشرف | حالة عملية الوكيل |
| POST | `/api/v1/node-servers/:id/proxy/stop` | المشرف | توقف |
| POST | `/api/v1/node-servers/:id/proxy/sync` | المشرف | مزامنة تكوين الوكيل |
| GET | `/api/v1/node-servers/:id/rate-limit` | المشرف | قراءة التكوين |
| PUT | `/api/v1/node-servers/:id/rate-limit` | المشرف | تحديث التكوين |
| GET | `/api/v1/node-servers/:id/rate-limit/status` | المشرف | الحالة |
| GET | `/api/v1/node-servers/:id/recover-mode` | المشرف | قراءة وضع استرداد العقدة |
| PUT | `/api/v1/node-servers/:id/recover-mode` | المشرف | ضبط وضع استرداد العقدة |
| POST | `/api/v1/node-servers/:id/revoke-cert` | المشرف | إبطال شهادة الوكيل |
| GET | `/api/v1/node-servers/:id/services` | المشرف | قائمة |
| POST | `/api/v1/node-servers/:id/services` | المشرف | إنشاء |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId` | المشرف | حذف |
| GET | `/api/v1/node-servers/:id/services/:serviceId` | المشرف | التفاصيل |
| PUT | `/api/v1/node-servers/:id/services/:serviceId` | المشرف | تحديث |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId/crash-logs` | المشرف | مسح سجلات الأعطال |
| GET | `/api/v1/node-servers/:id/services/:serviceId/crash-logs` | المشرف | قائمة سجل الأعطال |
| GET | `/api/v1/node-servers/:id/services/:serviceId/crash-logs/:logId` | المشرف | تفاصيل سجل الأعطال |
| GET | `/api/v1/node-servers/:id/services/:serviceId/endpoints` | المشرف | قائمة نقاط النهاية |
| POST | `/api/v1/node-servers/:id/services/:serviceId/endpoints` | المشرف | إنشاء نقطة النهاية |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId/endpoints/:endpointId` | المشرف | حذف نقطة النهاية |
| PUT | `/api/v1/node-servers/:id/services/:serviceId/endpoints/:endpointId` | المشرف | تحديث نقطة النهاية |
| PUT | `/api/v1/node-servers/:id/services/:serviceId/guardian` | المشرف | تحديث تكوين المشرف |
| GET | `/api/v1/node-servers/:id/services/:serviceId/process-status` | المشرف | الحصول على حالة المعالجة |
| POST | `/api/v1/node-servers/:id/services/:serviceId/restart` | المشرف | إعادة التشغيل |
| POST | `/api/v1/node-servers/:id/services/:serviceId/start` | المشرف | ابدأ |
| POST | `/api/v1/node-servers/:id/services/:serviceId/stop` | المشرف | توقف |
| GET | `/api/v1/node-servers/:id/services/process-status` | المشرف | جميع حالات العملية |
| GET | `/api/v1/node-servers/:id/share-domains` | المشرف | مشاركة المجالات لهذه العقدة |
| POST | `/api/v1/node-servers/:id/share-domains` | المشرف | إنشاء |
| DELETE | `/api/v1/node-servers/:id/share-domains/:domainId` | المشرف | حذف |
| PUT | `/api/v1/node-servers/:id/share-domains/:domainId` | المشرف | تحديث |
| DELETE | `/api/v1/node-servers/:id/share-domains/all` | المشرف | مسح الكل |
| POST | `/api/v1/node-servers/:id/singbox/control/firewall/apply` | المشرف | تطبيق قواعد جدار الحماية للتنقل بين المنافذ |
| POST | `/api/v1/node-servers/:id/singbox/control/firewall/remove` | المشرف | إزالة قواعد جدار الحماية للتنقل بين المنافذ |
| POST | `/api/v1/node-servers/:id/speed-test` | المشرف | اختبار سرعة العقدة |
| GET | `/api/v1/node-servers/:id/status` | المشرف | حالة العقدة |
| POST | `/api/v1/node-servers/:id/sync` | المشرف | تكوين عقدة المزامنة (إنشاء ودفع) |
| POST | `/api/v1/node-servers/:id/sync-data` | المشرف الفائق | دفع بيانات اللوحة إلى العقد (مزامنة كاملة للمستخدمين/الاشتراكات/الخطط) |
| POST | `/api/v1/node-servers/:id/sync-mirror` | المشرف | عقدة مرآة المزامنة |
| GET | `/api/v1/node-servers/:id/tags` | المشرف | قائمة العلامات |
| POST | `/api/v1/node-servers/:id/tags` | المشرف | إضافة علامة |
| DELETE | `/api/v1/node-servers/:id/tags/:tagKey` | المشرف | حذف العلامة |
| GET | `/api/v1/node-servers/:id/traffic-history` | المشرف | تاريخ المرور |
| POST | `/api/v1/node-servers/:id/traffic-resume` | المشرف | استعادة حركة المرور المعلقة بسبب تجاوزها |
| GET | `/api/v1/node-servers/:id/traffic-usage` | المشرف | استخدام حركة المرور |
| POST | `/api/v1/node-servers/:id/update-agent` | المشرف | وكيل الترقية |
| GET | `/api/v1/node-servers/:id/used-server-names` | المشرف | قائمة REALITY SNI قيد الاستخدام |
| GET | `/api/v1/node-servers/:id/waf/config` | المشرف | قراءة التكوين |
| PUT | `/api/v1/node-servers/:id/waf/config` | المشرف | حفظ التكوين |
| POST | `/api/v1/node-servers/:id/waf/disable-domain` | المشرف | تعطيل حماية المجال |
| GET | `/api/v1/node-servers/:id/waf/domain-config` | المشرف | قائمة تكوين المجال |
| PUT | `/api/v1/node-servers/:id/waf/domain-config` | المشرف | حفظ تكوينات المجال بالجملة |
| DELETE | `/api/v1/node-servers/:id/waf/domain-config/:domain` | المشرف | حذف تكوين المجال |
| GET | `/api/v1/node-servers/:id/waf/domain-config/:domain` | المشرف | تفاصيل تكوين المجال |
| PUT | `/api/v1/node-servers/:id/waf/domain-config/:domain` | المشرف | حفظ تكوين المجال |
| GET | `/api/v1/node-servers/:id/waf/domain-rules` | المشرف | دليل قواعد المجال |
| GET | `/api/v1/node-servers/:id/waf/domain-rules/:domain/:name` | المشرف | قراءة ملف قاعدة المجال |
| PUT | `/api/v1/node-servers/:id/waf/domain-rules/:domain/:name` | المشرف | حفظ ملف قاعدة المجال |
| GET | `/api/v1/node-servers/:id/waf/domain-status` | المشرف | حالة حماية المجال |
| POST | `/api/v1/node-servers/:id/waf/enable-domain` | المشرف | تمكين حماية المجال |
| POST | `/api/v1/node-servers/:id/waf/push` | المشرف | ادفع إلى هذه العقدة |
| GET | `/api/v1/node-servers/:id/waf/rules` | المشرف | قائمة قواعد العقدة WAF |
| DELETE | `/api/v1/node-servers/:id/waf/rules/:name` | المشرف | حذف قواعد العقدة WAF |
| GET | `/api/v1/node-servers/:id/waf/rules/:name` | المشرف | تفاصيل قاعدة العقدة WAF |
| PUT | `/api/v1/node-servers/:id/waf/rules/:name` | المشرف | حفظ قواعد العقدة WAF |
| GET | `/api/v1/node-servers/:id/waf/status` | المشرف | حالة العقدة WAF |
| PUT | `/api/v1/node-servers/:id/waf/status` | المشرف | تبديل العقدة WAF |
| POST | `/api/v1/node-servers/:id/waf/sync` | المشرف | المزامنة مع هذه العقدة |
| GET | `/api/v1/node-servers/:id/warp` | المشرف | قائمة |
| DELETE | `/api/v1/node-servers/:id/warp/:warpId` | المشرف | حذف |
| GET | `/api/v1/node-servers/:id/warp/config` | المشرف | قراءة التكوين |
| POST | `/api/v1/node-servers/:id/warp/generate-config` | المشرف | إنشاء التكوين |
| POST | `/api/v1/node-servers/:id/warp/register` | المشرف | سجل |
| GET | `/api/v1/node-servers/:id/warp/status` | المشرف | الحالة |
| GET | `/api/v1/node-servers/:id/xray/control/balancer/:tag` | المشرف | الحصول على معلومات الموازن |
| POST | `/api/v1/node-servers/:id/xray/control/balancer/:tag` | المشرف | تحديد هدف الموازن |
| POST | `/api/v1/node-servers/:id/xray/control/firewall/apply` | المشرف | تطبيق قواعد جدار الحماية للتنقل بين المنافذ |
| POST | `/api/v1/node-servers/:id/xray/control/firewall/remove` | المشرف | إزالة قواعد جدار الحماية للتنقل بين المنافذ |
| POST | `/api/v1/node-servers/:id/xray/control/inbound` | المشرف | إضافة واردة |
| DELETE | `/api/v1/node-servers/:id/xray/control/inbound/:tag` | المشرف | إزالة الواردة |
| POST | `/api/v1/node-servers/:id/xray/control/inbound/:tag/user` | المشرف | إضافة مستخدم |
| DELETE | `/api/v1/node-servers/:id/xray/control/inbound/:tag/user/:email` | المشرف | إزالة المستخدم |
| POST | `/api/v1/node-servers/:id/xray/control/outbound` | المشرف | إضافة الصادرة |
| DELETE | `/api/v1/node-servers/:id/xray/control/outbound/:tag` | المشرف | إزالة الصادرة |
| POST | `/api/v1/node-servers/:id/xray/control/routing` | المشرف | طلب الطريق |
| POST | `/api/v1/node-servers/:id/xray/control/routing/rule` | المشرف | إضافة قاعدة التوجيه |
| DELETE | `/api/v1/node-servers/:id/xray/control/routing/rule/:tag` | المشرف | إزالة قاعدة التوجيه |
| GET | `/api/v1/node-servers/:id/xray/control/routing/rules` | المشرف | قائمة قواعد التوجيه |
| POST | `/api/v1/node-servers/:id/xray/control/sync` | المشرف | قم بتشغيل مزامنة التكوين xray يدويًا |
| GET | `/api/v1/node-servers/:id/xray/stats` | المشرف | احصل على الإحصائيات |
| POST | `/api/v1/node-servers/:id/xray/stats/reset/:email` | المشرف | إعادة تعيين إحصائيات المستخدم |
| GET | `/api/v1/node-servers/:id/xray/stats/users` | المشرف | الحصول على إحصائيات المستخدم |
| GET | `/api/v1/node-servers/billable` | المشرف | قائمة العقدة القابلة للفوترة |
| GET | `/api/v1/node-servers/ca-cert` | المشرف | شهادة CA |
| GET | `/api/v1/node-servers/ca-status` | المشرف | حالة المرجع المصدق (CA). |
| GET | `/api/v1/node-servers/dashboard` | المشرف | حالة لوحة القيادة |
| POST | `/api/v1/node-servers/distribute-cert` | المشرف | توزيع الشهادة على كافة العقد |
| POST | `/api/v1/node-servers/failover-check` | المشرف | تشغيل فحص الفشل |
| GET | `/api/v1/node-servers/metrics/aggregated` | المشرف | المقاييس المجمعة |
| POST | `/api/v1/node-servers/migrate-cert` | المشرف | ترحيل الشهادة |
| GET | `/api/v1/node-servers/online-summary` | المشرف | ملخص على الانترنت |
| GET | `/api/v1/node-servers/online-users` | المشرف | جميع مستخدمي الإنترنت |
| GET | `/api/v1/node-servers/pending-agent-updates` | المشرف | وكلاء في انتظار الترقية |
| POST | `/api/v1/node-servers/recommend-region` | المشرف | المناطق الموصى بها |
| POST | `/api/v1/node-servers/select-nodes` | المشرف | حدد العقد للاشتراك |
| POST | `/api/v1/node-servers/speed-test-all` | المشرف | اختبار السرعة لجميع العقد |
| GET | `/api/v1/node-servers/speed-test-results` | المشرف | نتائج اختبار السرعة |
| GET | `/api/v1/node-servers/traffic/stats` | المشرف | إحصائيات المرور |
| POST | `/api/v1/node-servers/update-all-agents` | المشرف | ترقية جميع الوكلاء |
| DELETE | `/api/v1/node-servers/users/:client_id` | المشرف | مزامنة حذف العميل |
| GET | `/api/v1/node-servers/users/retry-queue` | المشرف | إعادة محاولة وضع قائمة الانتظار |
| POST | `/api/v1/node-servers/users/retry-queue/process` | المشرف | معالجة قائمة انتظار إعادة المحاولة |
| POST | `/api/v1/node-servers/users/sync` | المشرف | مزامنة عميل واحد |
| POST | `/api/v1/node-servers/validate-chain-config` | المشرف | التحقق من صحة التكوين المتسلسل |

### سجل تدقيق العقدة

تدقيق وتصدير العمليات من جانب العقدة.

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| GET | `/api/v1/node-audit-logs` | تم تسجيل الدخول | سجل تدقيق العقدة |
| GET | `/api/v1/node-audit-logs/export` | تم تسجيل الدخول | تصدير سجلات تدقيق العقدة |

### العقد (النموذج القديم، متقاعد)

> تم إيقاف `/api/v1/nodes` CRUD في 2026-10-20: تتم إدارة العقد بشكل موحد من خلال `/api/v1/node-servers`.
> تظل استعلامات المستخدم عبر الإنترنت في `/api/v1/node-servers/{online-users,online-summary,:id/online-users}`.

### نظرة عامة على المنطقة

عرض العقدة مجمعة حسب المنطقة.

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| GET | `/api/v1/regions/:region/servers` | تم تسجيل الدخول | قائمة العقدة لهذه المنطقة |
| GET | `/api/v1/regions/overview` | تم تسجيل الدخول | نظرة عامة على المنطقة |

### الاشتراكات

إدارة سجل الاشتراك: الإلغاء، تدوير الرمز، سجلات الوصول، إعادة تعيين الاستخدام.

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| GET | `/api/v1/subscriptions` | تم تسجيل الدخول | قائمة |
| POST | `/api/v1/subscriptions` | تم تسجيل الدخول | إنشاء |
| DELETE | `/api/v1/subscriptions/:id` | تم تسجيل الدخول | حذف |
| GET | `/api/v1/subscriptions/:id` | تم تسجيل الدخول | التفاصيل |
| PUT | `/api/v1/subscriptions/:id` | تم تسجيل الدخول | تحديث |
| GET | `/api/v1/subscriptions/:id/access-logs` | تم تسجيل الدخول | سجل الوصول |
| POST | `/api/v1/subscriptions/:id/enable` | تم تسجيل الدخول | تمكين |
| POST | `/api/v1/subscriptions/:id/reset-usage` | تم تسجيل الدخول | إعادة ضبط الاستخدام |
| POST | `/api/v1/subscriptions/:id/restore` | تم تسجيل الدخول | استئناف |
| POST | `/api/v1/subscriptions/:id/revoke` | تم تسجيل الدخول | إبطال |
| POST | `/api/v1/subscriptions/:id/rotate` | تم تسجيل الدخول | تدوير |
| GET | `/api/v1/subscriptions/:id/rotations` | تم تسجيل الدخول | تاريخ التناوب |

### تنسيقات الاشتراك

> تم إيقاف `/api/v1/subscription-formats` CRUD من الجانب الإداري بتاريخ 2026-10-20: نقطة الدخول الفعلية هي
> `/api/v1/mihomo/configs` (مركز تنسيق الاشتراك). يبقى الجدول `subscription_formats`،
> تُستخدم فقط كبيانات أولية لنقطة النهاية `/api/v1/public/client-configs` العامة.

### نقاط نهاية الاشتراك

مجالات الاشتراك المخصصة (الوكيل العكسي لمسار `/sub` الخاص باللوحة).

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| GET | `/api/v1/subscription-endpoints` | تم تسجيل الدخول | قائمة |
| POST | `/api/v1/subscription-endpoints` | تم تسجيل الدخول | إنشاء |
| DELETE | `/api/v1/subscription-endpoints/:id` | تم تسجيل الدخول | حذف |
| GET | `/api/v1/subscription-endpoints/:id` | تم تسجيل الدخول | التفاصيل |
| PUT | `/api/v1/subscription-endpoints/:id` | تم تسجيل الدخول | تحديث |

### مشاركة المجالات

مجال الوصول/تجمع IP المستخدم في روابط العميل.

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| GET | `/api/v1/share-domains` | تم تسجيل الدخول | قائمة |
| POST | `/api/v1/share-domains` | تم تسجيل الدخول | إنشاء |
| DELETE | `/api/v1/share-domains/:id` | تم تسجيل الدخول | حذف |
| PUT | `/api/v1/share-domains/:id` | تم تسجيل الدخول | تحديث |
| DELETE | `/api/v1/share-domains/all` | تم تسجيل الدخول | مسح الكل |
| POST | `/api/v1/share-domains/seed` | تم تسجيل الدخول | تهيئة البيانات الافتراضية |

### Cloudflare IP

Cloudflare CDN تجمع IP وإدارة المصدر.

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| DELETE | `/api/v1/cfip/entries` | تم تسجيل الدخول | مسح الإدخالات |
| GET | `/api/v1/cfip/entries` | تم تسجيل الدخول | قائمة الدخول |
| POST | `/api/v1/cfip/entries` | تم تسجيل الدخول | إضافة إدخال |
| DELETE | `/api/v1/cfip/entries/:id` | تم تسجيل الدخول | حذف الإدخال |
| POST | `/api/v1/cfip/fetch-all` | تم تسجيل الدخول | جلب كافة المصادر |
| POST | `/api/v1/cfip/seed` | تم تسجيل الدخول | تهيئة البيانات الافتراضية |
| GET | `/api/v1/cfip/sources` | تم تسجيل الدخول | قائمة مصادر البيانات |
| POST | `/api/v1/cfip/sources` | تم تسجيل الدخول | إنشاء مصدر البيانات |
| DELETE | `/api/v1/cfip/sources/:id` | تم تسجيل الدخول | حذف مصدر البيانات |
| PUT | `/api/v1/cfip/sources/:id` | تم تسجيل الدخول | تحديث مصدر البيانات |
| POST | `/api/v1/cfip/sources/:id/fetch` | تم تسجيل الدخول | جلب هذا المصدر |

### تكوين الاشتراك (Mihomo/Clash)

تكوين قالب الاشتراك: المجموعات، والقواعد، وموفري القواعد، ومعاينة YAML.

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| GET | `/api/v1/mihomo/configs` | تم تسجيل الدخول | قائمة |
| POST | `/api/v1/mihomo/configs` | تم تسجيل الدخول | إنشاء |
| DELETE | `/api/v1/mihomo/configs/:id` | تم تسجيل الدخول | حذف |
| GET | `/api/v1/mihomo/configs/:id` | تم تسجيل الدخول | التفاصيل |
| PUT | `/api/v1/mihomo/configs/:id` | تم تسجيل الدخول | تحديث |
| GET | `/api/v1/mihomo/configs/:id/proxy-groups` | تم تسجيل الدخول | قائمة المجموعة |
| POST | `/api/v1/mihomo/configs/:id/proxy-groups` | تم تسجيل الدخول | إنشاء مجموعة |
| GET | `/api/v1/mihomo/configs/:id/rule-providers` | تم تسجيل الدخول | قائمة موفر القاعدة |
| POST | `/api/v1/mihomo/configs/:id/rule-providers` | تم تسجيل الدخول | إنشاء مزود القاعدة |
| GET | `/api/v1/mihomo/configs/:id/rules` | تم تسجيل الدخول | قائمة القواعد |
| POST | `/api/v1/mihomo/configs/:id/rules` | تم تسجيل الدخول | إنشاء القاعدة |
| GET | `/api/v1/mihomo/configs/:id/yaml` | تم تسجيل الدخول | تم إنشاء المعاينة YAML |
| DELETE | `/api/v1/mihomo/proxy-groups/:id` | تم تسجيل الدخول | حذف المجموعة |
| DELETE | `/api/v1/mihomo/rule-providers/:id` | تم تسجيل الدخول | حذف مزود القاعدة |
| DELETE | `/api/v1/mihomo/rules/:id` | تم تسجيل الدخول | حذف القاعدة |

### مواقع إعادة الشحن

تكامل موقع الدفع التابع لجهة خارجية: مفاتيح التوقيع، وملفات التحقق، والتحقق من النطاق، ومستندات التكامل.

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| GET | `/api/v1/charge-sites` | تم تسجيل الدخول | قائمة الموقع |
| POST | `/api/v1/charge-sites` | تم تسجيل الدخول | إنشاء موقع |
| DELETE | `/api/v1/charge-sites/:id` | تم تسجيل الدخول | حذف الموقع |
| GET | `/api/v1/charge-sites/:id` | تم تسجيل الدخول | تفاصيل الموقع |
| PUT | `/api/v1/charge-sites/:id` | تم تسجيل الدخول | تحديث الموقع |
| POST | `/api/v1/charge-sites/:id/clear-prev-secret` | تم تسجيل الدخول | مسح المفتاح القديم |
| POST | `/api/v1/charge-sites/:id/generate-verify` | تم تسجيل الدخول | إنشاء ملف التحقق |
| POST | `/api/v1/charge-sites/:id/generate-verify-file` | تم تسجيل الدخول | إنشاء ملف التحقق |
| GET | `/api/v1/charge-sites/:id/integration-doc` | تم تسجيل الدخول | الحصول على وثيقة التكامل |
| POST | `/api/v1/charge-sites/:id/rotate-secret` | تم تسجيل الدخول | تدوير مفتاح التوقيع |
| POST | `/api/v1/charge-sites/:id/verify-dns` | تم تسجيل الدخول | التحقق من المجال DNS |
| POST | `/api/v1/charge-sites/:id/verify-domain` | تم تسجيل الدخول | التحقق من المجال HTTP التحدي |
| POST | `/api/v1/charge-sites/:id/verify-domain-dns` | تم تسجيل الدخول | التحقق من المجال DNS |
| POST | `/api/v1/charge-sites/:id/verify-domain-http` | تم تسجيل الدخول | تحدي التحقق من المجال HTTP |

### أوامر إعادة الشحن

استعلامات طلب إعادة الشحن، والمبالغ المستردة، وتصدير CSV.

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| GET | `/api/v1/charge-orders` | تم تسجيل الدخول | قائمة الطلبات |
| GET | `/api/v1/charge-orders/:id` | تم تسجيل الدخول | تفاصيل الطلب |
| POST | `/api/v1/charge-orders/:id/refund` | تم تسجيل الدخول | استرداد |
| GET | `/api/v1/charge-orders/export-csv` | تم تسجيل الدخول | تصدير أوامر إعادة الشحن CSV |

### بطاقات إعادة الشحن

إنشاء البطاقات المجمعة، وسجلات الاستخدام، وتصدير CSV.

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| GET | `/api/v1/charge-cards` | تم تسجيل الدخول | قائمة البطاقات |
| POST | `/api/v1/charge-cards` | تم تسجيل الدخول | إنشاء بطاقة |
| DELETE | `/api/v1/charge-cards/:id` | تم تسجيل الدخول | حذف البطاقة |
| GET | `/api/v1/charge-cards/:id/usages` | تم تسجيل الدخول | سجلات استخدام البطاقة |
| POST | `/api/v1/charge-cards/batch` | تم تسجيل الدخول | إنشاء بطاقات بكميات كبيرة |
| GET | `/api/v1/charge-cards/export-csv` | تم تسجيل الدخول | بطاقات التصدير CSV |

### أحداث رد الاتصال

تدقيق حدث رد الاتصال.

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| GET | `/api/v1/charge-callback-events` | تم تسجيل الدخول | قائمة أحداث رد الاتصال |

### إعادة محاولة رد الاتصال

إعادة المحاولة اليدوية لعمليات الاسترجاعات الفاشلة.

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| GET | `/api/v1/charge-callback-retries` | تم تسجيل الدخول | قائمة رد الاتصال الفاشلة |
| POST | `/api/v1/charge-callback-retries/:id/retry` | المشرف | أعد محاولة رد الاتصال |

### رصيد العميل

عرض المشرف للأرصدة والمعاملات.

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| GET | `/api/v1/client-balances` | تم تسجيل الدخول | قائمة رصيد العملاء |
| GET | `/api/v1/client-balances/transactions` | تم تسجيل الدخول | معاملات الرصيد (المشرف) |
| GET | `/api/v1/client-balances/transactions/export-csv` | تم تسجيل الدخول | معاملات رصيد التصدير CSV |

### الهدايا

إصدار الهدايا المستحقة (الأيام/المرور).

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| GET | `/api/v1/client-gifts` | تم تسجيل الدخول | قائمة الهدايا |
| POST | `/api/v1/client-gifts` | تم تسجيل الدخول | إنشاء هدية |
| POST | `/api/v1/client-gifts/batch` | تم تسجيل الدخول | إنشاء الهدايا بكميات كبيرة |
| GET | `/api/v1/client-gifts/search-clients` | تم تسجيل الدخول | عملاء البحث |

### تكوين الإحالة

تكوين لجنة الإحالة والإحصائيات.

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| GET | `/api/v1/referral/config` | تم تسجيل الدخول | تكوين الإحالة |
| PUT | `/api/v1/referral/config` | المشرف | حفظ تكوين الإحالة |
| GET | `/api/v1/referral/records` | تم تسجيل الدخول | قائمة سجل الإحالة |
| GET | `/api/v1/referral/stats` | تم تسجيل الدخول | إحصائيات الإحالة |

### عمليات سحب الإحالة

مراجعة طلبات السحب.

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| GET | `/api/v1/referral-withdrawals` | تم تسجيل الدخول | قائمة طلبات السحب |
| POST | `/api/v1/referral-withdrawals/:id/approve` | تم تسجيل الدخول | الموافقة على الانسحاب |
| POST | `/api/v1/referral-withdrawals/:id/reject` | تم تسجيل الدخول | رفض الانسحاب |

### قوائم IP المسموح بها (ملخص)

عرض قائمة IP المسموح بها لجميع العملاء.

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| GET | `/api/v1/ip-whitelists` | تم تسجيل الدخول | جميع قوائم IP المسموح بها للعملاء |

### روابط OAuth الخاصة بالعميل

الاستعلام عن ارتباطات حساب الطرف الثالث للعميل.

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| GET | `/api/v1/client-oauth-bindings` | تم تسجيل الدخول | قائمة ربط OAuth للعميل |

### التذاكر (المسؤول)

الرد على التذكرة والتنازل والإغلاق.

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| GET | `/api/v1/tickets` | تم تسجيل الدخول | قائمة التذاكر |
| GET | `/api/v1/tickets/:id` | تم تسجيل الدخول | تفاصيل التذكرة |
| POST | `/api/v1/tickets/:id/assign` | تم تسجيل الدخول | تخصيص تذكرة |
| POST | `/api/v1/tickets/:id/close` | تم تسجيل الدخول | إغلاق التذكرة |
| POST | `/api/v1/tickets/:id/reply` | تم تسجيل الدخول | الرد على التذكرة |

### مقالات

CRUD لمقالات البوابة.

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| GET | `/api/v1/articles` | تم تسجيل الدخول | قائمة المقالات |
| POST | `/api/v1/articles` | تم تسجيل الدخول | إنشاء مقالة |
| DELETE | `/api/v1/articles/:id` | تم تسجيل الدخول | حذف المادة |
| GET | `/api/v1/articles/:id` | تم تسجيل الدخول | تفاصيل المادة |
| PUT | `/api/v1/articles/:id` | تم تسجيل الدخول | تحديث المادة |

### الحسابات المشتركة

إصدار وإصلاح السجلات للحسابات المشتركة (البث، الخ).

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| GET | `/api/v1/shared-accounts` | تم تسجيل الدخول | قائمة الحسابات المشتركة |
| POST | `/api/v1/shared-accounts` | تم تسجيل الدخول | إنشاء حساب مشترك |
| DELETE | `/api/v1/shared-accounts/:id` | تم تسجيل الدخول | حذف الحساب المشترك |
| GET | `/api/v1/shared-accounts/:id` | تم تسجيل الدخول | تفاصيل الحساب المشترك |
| PUT | `/api/v1/shared-accounts/:id` | تم تسجيل الدخول | تحديث الحساب المشترك |
| GET | `/api/v1/shared-accounts/:id/repair-logs` | تم تسجيل الدخول | إصلاح السجلات |

### الشهادات

إصدار الشهادة العامة (ACME) وتحميلها وتجديدها وتصديرها؛ يتم دعم الإصدار على عقدة محددة.

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| GET | `/api/v1/certificates` | تم تسجيل الدخول | قائمة |
| POST | `/api/v1/certificates` | تم تسجيل الدخول | إنشاء |
| DELETE | `/api/v1/certificates/:id` | تم تسجيل الدخول | حذف |
| GET | `/api/v1/certificates/:id` | تم تسجيل الدخول | التفاصيل |
| PUT | `/api/v1/certificates/:id` | تم تسجيل الدخول | تحديث |
| GET | `/api/v1/certificates/:id/download` | تم تسجيل الدخول | تحميل |
| POST | `/api/v1/certificates/:id/renew` | تم تسجيل الدخول | تجديد العميل |
| GET | `/api/v1/certificates/check-port` | تم تسجيل الدخول | تحقق من المنفذ |
| GET | `/api/v1/certificates/detect-ip` | تم تسجيل الدخول | دقق في IP |
| GET | `/api/v1/certificates/export` | تم تسجيل الدخول | تصدير |
| POST | `/api/v1/certificates/issue` | تم تسجيل الدخول | قضية. الجسم: `{"domains":["example.com","*.example.com"],"challenge_type":"dns-01","dns_channel_id":2,"key_type":"rsa","key_bits":4096}`. `challenge_type` افتراضيًا http-01؛ `dns_channel_id` يحدد قناة DNS (موفر DNS الذي تم تكوينه + بيانات الاعتماد) |
| POST | `/api/v1/certificates/issue-on-node` | تم تسجيل الدخول | مشكلة في العقدة |
| POST | `/api/v1/certificates/upload` | تم تسجيل الدخول | تحميل |

### قنوات الشهادة

تكوين قناة ACME (CA).

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| GET | `/api/v1/cert-channels` | تم تسجيل الدخول | قائمة |
| POST | `/api/v1/cert-channels` | تم تسجيل الدخول | إنشاء |
| DELETE | `/api/v1/cert-channels/:id` | تم تسجيل الدخول | حذف |
| PUT | `/api/v1/cert-channels/:id` | تم تسجيل الدخول | تحديث |
| GET | `/api/v1/cert-channels/providers` | تم تسجيل الدخول | مزود |

### التكوين EAB

ربط بيانات الاعتماد بالحسابات الخارجية مثل ZeroSSL / Google.

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| GET | `/api/v1/eab-configs` | تم تسجيل الدخول | قائمة |
| POST | `/api/v1/eab-configs` | تم تسجيل الدخول | إنشاء |
| DELETE | `/api/v1/eab-configs/:id` | تم تسجيل الدخول | حذف |
| PUT | `/api/v1/eab-configs/:id` | تم تسجيل الدخول | تحديث |
| POST | `/api/v1/eab-configs/fetch-google` | تم تسجيل الدخول | قم بإحضار Google EAB من GCP |
| POST | `/api/v1/eab-configs/fetch-zerossl` | تم تسجيل الدخول | قم بإحضار ZeroSSL EAB بالبريد الإلكتروني بنقرة واحدة |

### حسابات ACME

إدارة حساب ACME (البريد الإلكتروني للاتصال) من جانب اللوحة.

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| GET | `/api/v1/acme-accounts` | تم تسجيل الدخول | قائمة |
| POST | `/api/v1/acme-accounts` | تم تسجيل الدخول | إنشاء |
| DELETE | `/api/v1/acme-accounts/:id` | تم تسجيل الدخول | حذف |
| PUT | `/api/v1/acme-accounts/:id` | تم تسجيل الدخول | تحديث |
| GET | `/api/v1/acme-accounts/default` | تم تسجيل الدخول | الحصول على الإعدادات الافتراضية |

### المدمج في كاليفورنيا

اللوحة الداخلية CA: إصدار/تجديد/إلغاء شهادة الوكيل، تصدير CRL، قائمة انتظار التوزيع.

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| GET | `/api/v1/ca/agent-certs` | تم تسجيل الدخول | قائمة شهادات الوكيل |
| POST | `/api/v1/ca/agent-certs/:server_id/renew` | تم تسجيل الدخول | تجديد شهادة الوكيل |
| GET | `/api/v1/ca/distribution-queue` | تم تسجيل الدخول | قائمة انتظار توزيع الشهادات |
| DELETE | `/api/v1/ca/distribution-queue/:id` | تم تسجيل الدخول | حذف إدخال قائمة انتظار التوزيع |
| POST | `/api/v1/ca/distribution-queue/:id/retry` | تم تسجيل الدخول | أعد محاولة توزيع الشهادة |
| GET | `/api/v1/ca/revocations` | تم تسجيل الدخول | قائمة الإلغاء |
| GET | `/api/v1/ca/revocations/export` | تم تسجيل الدخول | تصدير CRL |
| GET | `/api/v1/ca/status` | تم تسجيل الدخول | حالة المرجع المصدق (CA). |

### القواعد الجغرافية

مزامنة وتسليم مصادر وملفات بيانات الموقع الجغرافي/الموقع الجغرافي.

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| GET | `/api/v1/geo-rules/files` | تم تسجيل الدخول | قائمة الملفات |
| DELETE | `/api/v1/geo-rules/files/:fileId` | تم تسجيل الدخول | حذف الملف |
| PUT | `/api/v1/geo-rules/files/:fileId/auto-sync` | تم تسجيل الدخول | تبديل المزامنة التلقائية |
| POST | `/api/v1/geo-rules/files/:fileId/download` | تم تسجيل الدخول | تحميل |
| GET | `/api/v1/geo-rules/files/:fileId/download-file` | تم تسجيل الدخول | تنزيل الملف |
| GET | `/api/v1/geo-rules/github-assets` | تم تسجيل الدخول | جلب أصول مستودع GitHub |
| POST | `/api/v1/geo-rules/push-all` | تم تسجيل الدخول | ادفع إلى جميع العقد |
| POST | `/api/v1/geo-rules/seed` | تم تسجيل الدخول | تهيئة البيانات الافتراضية |
| GET | `/api/v1/geo-rules/sources` | تم تسجيل الدخول | قائمة مصادر البيانات |
| POST | `/api/v1/geo-rules/sources` | تم تسجيل الدخول | إنشاء مصدر البيانات |
| DELETE | `/api/v1/geo-rules/sources/:sourceId` | تم تسجيل الدخول | حذف مصدر البيانات |
| PUT | `/api/v1/geo-rules/sources/:sourceId` | تم تسجيل الدخول | تحديث مصدر البيانات |
| GET | `/api/v1/geo-rules/sources/:sourceId/assets` | تم تسجيل الدخول | جلب أصول الإصدار |
| POST | `/api/v1/geo-rules/sources/:sourceId/files` | تم تسجيل الدخول | إضافة ملف |
| PUT | `/api/v1/geo-rules/sources/:sourceId/schedule` | تم تسجيل الدخول | تبديل المزامنة المجدولة |
| POST | `/api/v1/geo-rules/sources/:sourceId/select` | تم تسجيل الدخول | حدد الملف |
| POST | `/api/v1/geo-rules/sources/:sourceId/sync` | تم تسجيل الدخول | مزامنة مصدر البيانات هذا |
| GET | `/api/v1/geo-rules/status` | تم تسجيل الدخول | الحالة |
| POST | `/api/v1/geo-rules/sync-all` | تم تسجيل الدخول | مزامنة الكل |
| GET | `/api/v1/geo-rules/sync-status/:key` | تم تسجيل الدخول | تقدم المزامنة |

### مكتبة القواعد WAF

قواعد WAF العالمية: مصدر GitHub، والمزامنة، والتسليم إلى جميع العقد.

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| GET | `/api/v1/waf/github-source` | تم تسجيل الدخول | مصدر البيانات GitHub |
| PUT | `/api/v1/waf/github-source` | تم تسجيل الدخول | حفظ مصدر البيانات GitHub |
| POST | `/api/v1/waf/push-all` | تم تسجيل الدخول | ادفع إلى جميع العقد |
| GET | `/api/v1/waf/rules` | تم تسجيل الدخول | قائمة القواعد |
| DELETE | `/api/v1/waf/rules/:name` | تم تسجيل الدخول | حذف القاعدة |
| GET | `/api/v1/waf/rules/:name` | تم تسجيل الدخول | الحصول على القاعدة |
| PUT | `/api/v1/waf/rules/:name` | تم تسجيل الدخول | حفظ القاعدة |
| POST | `/api/v1/waf/sync-all` | تم تسجيل الدخول | مزامنة الكل |
| POST | `/api/v1/waf/sync-github` | تم تسجيل الدخول | المزامنة من GitHub |

### إدارة السجل

قراءة وتدفق سجل العقدة/الأساسية، وسياسات التنظيف، والتحكم في كاتب السجل.

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| POST | `/api/v1/log-manage/cleanup-all` | تم تسجيل الدخول | تنظيف كل شيء الآن |
| GET | `/api/v1/log-manage/cleanup-rules` | تم تسجيل الدخول | قائمة قواعد التنظيف |
| PUT | `/api/v1/log-manage/cleanup-rules/:id` | تم تسجيل الدخول | تحديث قاعدة التنظيف |
| POST | `/api/v1/log-manage/cleanup-table/:table` | تم تسجيل الدخول | قم بتنظيف الجدول المحدد الآن |
| GET | `/api/v1/log-manage/core-log-output` | تم تسجيل الدخول | تكوين إخراج السجل الأساسي |
| PUT | `/api/v1/log-manage/core-log-output` | تم تسجيل الدخول | حفظ تكوين إخراج السجل الأساسي |
| GET | `/api/v1/log-manage/core/:type/stdout-log` | تم تسجيل الدخول | قراءة سجلات stdout الأساسية |
| GET | `/api/v1/log-manage/core/:type/stdout-stream` | تم تسجيل الدخول | تيار stdout الأساسية |
| POST | `/api/v1/log-manage/discover` | تم تسجيل الدخول | اكتشف ملفات سجل العقدة |
| GET | `/api/v1/log-manage/log-output` | تم تسجيل الدخول | سجل تكوين الإخراج |
| PUT | `/api/v1/log-manage/log-output` | تم تسجيل الدخول | حفظ تكوين إخراج السجل |
| POST | `/api/v1/log-manage/log-output/release/:type` | تم تسجيل الدخول | الافراج عن سجل الاستيلاء stdout |
| POST | `/api/v1/log-manage/log-output/takeover/:type` | تم تسجيل الدخول | تولي سجل stdout |
| GET | `/api/v1/log-manage/nodes/:serverId/logs` | تم تسجيل الدخول | قائمة ملفات سجل العقدة |
| GET | `/api/v1/log-manage/nodes/:serverId/logs/:type` | تم تسجيل الدخول | قراءة سجلات العقدة |
| POST | `/api/v1/log-manage/nodes/:serverId/logs/:type/rotate` | تم تسجيل الدخول | تدوير سجلات العقدة |
| GET | `/api/v1/log-manage/nodes/:serverId/logs/:type/stream` | تم تسجيل الدخول | سجلات عقدة الدفق |
| GET | `/api/v1/log-manage/policies` | تم تسجيل الدخول | قائمة السياسات |
| POST | `/api/v1/log-manage/policies` | تم تسجيل الدخول | إنشاء سياسة |
| DELETE | `/api/v1/log-manage/policies/:id` | تم تسجيل الدخول | حذف السياسة |
| GET | `/api/v1/log-manage/policies/:id` | تم تسجيل الدخول | تفاصيل السياسة |
| PUT | `/api/v1/log-manage/policies/:id` | تم تسجيل الدخول | تحديث السياسة |
| GET | `/api/v1/log-manage/policies/:id/jobs` | تم تسجيل الدخول | قائمة مهام التنظيف |
| GET | `/api/v1/log-manage/policies/:id/log` | تم تسجيل الدخول | قراءة السجلات |
| GET | `/api/v1/log-manage/policies/:id/log-stream` | تم تسجيل الدخول | سجلات الدفق |
| POST | `/api/v1/log-manage/policies/:id/rotate` | تم تسجيل الدخول | تدوير |
| GET | `/api/v1/log-manage/proxy/:type/stdout-log` | تم تسجيل الدخول | قراءة سجلات stdout الأساسية |
| GET | `/api/v1/log-manage/proxy/:type/stdout-stream` | تم تسجيل الدخول | تيار stdout الأساسية |
| GET | `/api/v1/log-manage/scheduler-config` | تم تسجيل الدخول | جدولة التكوين |
| PUT | `/api/v1/log-manage/scheduler-config` | تم تسجيل الدخول | تحديث تكوين الجدولة |
| GET | `/api/v1/log-manage/status` | تم تسجيل الدخول | الحالة |
| POST | `/api/v1/log-manage/sync` | تم تسجيل الدخول | سياسات تنظيف المزامنة |
| GET | `/api/v1/log-manage/writers` | تم تسجيل الدخول | حالة الكاتب |
| PUT | `/api/v1/log-manage/writers/:type/config` | تم تسجيل الدخول | تحديث تكوين الكاتب |
| GET | `/api/v1/log-manage/writers/:type/files` | تم تسجيل الدخول | قائمة ملفات سجل الكاتب |
| POST | `/api/v1/log-manage/writers/:type/start` | تم تسجيل الدخول | ابدأ الكاتب |
| POST | `/api/v1/log-manage/writers/:type/stop` | تم تسجيل الدخول | وقف الكاتب |

### النسخ الاحتياطية

قاعدة البيانات/النسخ الاحتياطي الكامل، والاستعادة، والمهام المجدولة، والتخزين عن بعد FTP/S3/WebDAV.

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| DELETE | `/api/v1/backups/:id` | المشرف | حذف النسخة الاحتياطية |
| GET | `/api/v1/backups/:id/download` | المشرف | تحميل النسخة الاحتياطية |
| POST | `/api/v1/backups/:id/restore` | المشرف | استعادة من النسخة الاحتياطية |
| POST | `/api/v1/backups/:id/upload-ftp` | المشرف | تحميل إلى FTP |
| POST | `/api/v1/backups/:id/upload-s3` | المشرف | تحميل إلى S3 |
| POST | `/api/v1/backups/:id/upload-webdav` | المشرف | تحميل إلى WebDAV |
| POST | `/api/v1/backups/create` | المشرف | إنشاء نسخة احتياطية كاملة (مع تفريغ قاعدة البيانات) |
| GET | `/api/v1/backups/list` | المشرف | قائمة النسخ الاحتياطي الكاملة |
| POST | `/api/v1/backups/remote-restore` | المشرف | استعادة من بعيد |
| GET | `/api/v1/backups/restore-status` | المشرف | استعادة التقدم |
| GET | `/api/v1/backups/schedules` | المشرف | قائمة الوظائف المجدولة |
| POST | `/api/v1/backups/schedules` | المشرف | إنشاء مهمة مجدولة |
| DELETE | `/api/v1/backups/schedules/:id` | المشرف | حذف المهمة المجدولة |
| PUT | `/api/v1/backups/schedules/:id` | المشرف | تحديث المهمة المجدولة |
| GET | `/api/v1/backups/storages` | المشرف | قائمة تكوين التخزين |
| POST | `/api/v1/backups/storages` | المشرف | إنشاء تكوين التخزين |
| DELETE | `/api/v1/backups/storages/:id` | المشرف | حذف تكوين التخزين |
| PUT | `/api/v1/backups/storages/:id` | المشرف | تحديث تكوين التخزين |
| GET | `/api/v1/backups/storages/:id/files` | المشرف | قائمة ملفات النسخ الاحتياطي عن بعد |
| POST | `/api/v1/backups/upload-restore` | المشرف | تحميل النسخ الاحتياطي والاستعادة |

### إعدادات النظام

تكوين قاعدة البيانات وترحيلها، وإعدادات اللوحة، ومفاتيح JWT، واختبارات SMTP/Telegram، وقوالب الإشعارات، والترقيات عبر الإنترنت (معظمها إداري).

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| PUT | `/api/v1/system/bootstrap-finish` | المشرف | أكمل إعداد التمهيد الأول |
| GET | `/api/v1/system/bootstrap-state` | المشرف | حالة التمهيد الأول |
| GET | `/api/v1/system/config-file/:coreType` | المشرف | قراءة ملف التكوين الأساسي |
| GET | `/api/v1/system/db-config` | المشرف | تكوين قاعدة البيانات |
| PUT | `/api/v1/system/db-config` | المشرف | تحديث تكوين قاعدة البيانات |
| POST | `/api/v1/system/db-config/migrate` | المشرف | ترحيل بيانات قاعدة البيانات |
| POST | `/api/v1/system/db-config/switch` | المشرف | تبديل قاعدة البيانات (مع الترحيل بخطوة واحدة) |
| POST | `/api/v1/system/db-config/test` | المشرف | اختبار اتصال قاعدة البيانات |
| GET | `/api/v1/system/db-pool-stats` | المشرف | إحصائيات تجمع الاتصال |
| GET | `/api/v1/system/db-stats` | المشرف | إحصائيات قاعدة البيانات |
| GET | `/api/v1/system/db-stats/ping` | المشرف | مسبار قاعدة البيانات |
| POST | `/api/v1/system/db-stats/reset` | المشرف | إعادة تعيين إحصائيات قاعدة البيانات |
| GET | `/api/v1/system/encryption-key-status` | المشرف | حالة مفتاح التشفير |
| GET | `/api/v1/system/env-config` | المشرف | تكوين .env |
| PUT | `/api/v1/system/env-config` | المشرف | قم بتحديث تكوين .env |
| POST | `/api/v1/system/generate-encryption-key` | المشرف | إنشاء مفتاح التشفير |
| GET | `/api/v1/system/github-config` | المشرف | التكوين GitHub |
| PUT | `/api/v1/system/github-config` | المشرف | تحديث تكوين GitHub |
| GET | `/api/v1/system/jwt-secret-status` | المشرف | حالة المفتاح JWT |
| GET | `/api/v1/system/maintenance` | عام | حالة وضع الصيانة (عام) |
| PUT | `/api/v1/system/maintenance` | المشرف | تبديل وضع الصيانة |
| POST | `/api/v1/system/migrate-data` | المشرف | ترحيل البيانات |
| GET | `/api/v1/system/notification-config` | المشرف | تكوين الإخطار |
| PUT | `/api/v1/system/notification-config` | المشرف | تحديث تكوين الإخطار |
| GET | `/api/v1/system/notification-templates` | المشرف | قائمة القالب |
| POST | `/api/v1/system/notification-templates` | المشرف | إنشاء قالب |
| DELETE | `/api/v1/system/notification-templates/:id` | المشرف | حذف القالب |
| PUT | `/api/v1/system/notification-templates/:id` | المشرف | تحديث القالب |
| POST | `/api/v1/system/notification-templates/:id/broadcast` | المشرف | إشعار البث عن طريق القالب |
| POST | `/api/v1/system/notification-templates/:id/send` | المشرف | إرسال إشعار الاختبار |
| GET | `/api/v1/system/panel-config` | المشرف | تكوين اللوحة |
| PUT | `/api/v1/system/panel-config` | المشرف | تحديث تكوين اللوحة |
| GET | `/api/v1/system/reality-targets` | المشرف | قائمة الأهداف REALITY |
| PUT | `/api/v1/system/reality-targets` | المشرف | تحديث هدف REALITY |
| POST | `/api/v1/system/reality-targets/reset` | المشرف | إعادة تعيين هدف REALITY إلى الوضع الافتراضي |
| POST | `/api/v1/system/regenerate-jwt-secret` | المشرف | إعادة إنشاء مفتاح JWT |
| GET | `/api/v1/system/releases` | المشرف | الإصدارات المتاحة |
| POST | `/api/v1/system/restart` | المشرف | إعادة التشغيل |
| POST | `/api/v1/system/rollback` | المشرف | التراجع عن النظام |
| GET | `/api/v1/system/settings` | المشرف | قراءة الإعدادات |
| PUT | `/api/v1/system/settings/batch` | المشرف | إعدادات التحديث بالجملة |
| GET | `/api/v1/system/stats` | المشرف | حالة النظام |
| GET | `/api/v1/system/subscription-billing-templates` | المشرف | نموذج فاتورة الاشتراك |
| PUT | `/api/v1/system/subscription-billing-templates` | المشرف | تحديث قالب فاتورة الاشتراك |
| POST | `/api/v1/system/telegram-chat-id` | المشرف | احصل على معرف الدردشة Telegram |
| POST | `/api/v1/system/test-smtp` | المشرف | اختبار SMTP |
| POST | `/api/v1/system/test-telegram` | المشرف | اختبار Telegram |
| POST | `/api/v1/system/update` | المشرف | ترقية النظام |
| GET | `/api/v1/system/update-jobs` | المشرف | ترقية قائمة الوظائف |
| POST | `/api/v1/system/upload-mtls-ca` | المشرف | تحميل شهادة mTLS CA |

### التدقيق الأمني

تدقيق استعلامات الحدث والتصدير.

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| GET | `/api/v1/security/audit-events` | تم تسجيل الدخول | قائمة أحداث التدقيق |
| GET | `/api/v1/security/audit-events/export` | تم تسجيل الدخول | أحداث تدقيق التصدير |

### المقاييس

مقاييس اللوحة المجمعة (العقد/الشهادات/الخدمات).

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| GET | `/api/v1/metrics/certificates` | تم تسجيل الدخول | مقاييس الشهادة |
| GET | `/api/v1/metrics/nodes` | تم تسجيل الدخول | مقاييس العقدة |
| GET | `/api/v1/metrics/overview` | تم تسجيل الدخول | نظرة عامة |

### قوالب البوابة

قوالب البوابة وتكوين التنشيط وإدارة ذاكرة التخزين المؤقت.

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| GET | `/api/v1/portal/active` | تم تسجيل الدخول | قالب نشط |
| PUT | `/api/v1/portal/active` | تم تسجيل الدخول | تحديث القالب النشط |
| DELETE | `/api/v1/portal/cache` | تم تسجيل الدخول | مسح ذاكرة التخزين المؤقت للبوابة |
| GET | `/api/v1/portal/cache` | تم تسجيل الدخول | حالة ذاكرة التخزين المؤقت |
| PUT | `/api/v1/portal/cache/dev-mode` | تم تسجيل الدخول | تبديل وضع تطوير ذاكرة التخزين المؤقت |
| POST | `/api/v1/portal/cache/rebuild` | تم تسجيل الدخول | إعادة بناء ذاكرة التخزين المؤقت للبوابة |
| POST | `/api/v1/portal/reset` | تم تسجيل الدخول | استعادة القالب الافتراضي |
| GET | `/api/v1/portal/templates` | تم تسجيل الدخول | قائمة |
| POST | `/api/v1/portal/templates` | تم تسجيل الدخول | إنشاء |
| DELETE | `/api/v1/portal/templates/:id` | تم تسجيل الدخول | حذف |
| GET | `/api/v1/portal/templates/:id` | تم تسجيل الدخول | التفاصيل |
| PUT | `/api/v1/portal/templates/:id` | تم تسجيل الدخول | تحديث |
| POST | `/api/v1/portal/templates/:id/activate` | تم تسجيل الدخول | تفعيل القالب |

> `/portal/templates` CRUD هو **API** متقدم: تستخدم واجهة مستخدم البوابة الإلكترونية بشكل أساسي `PUT /api/v1/portal/active`
> (تكوين الموقع الكامل بلقطة واحدة)؛ إدارة/تبديل قوالب متعددة مع هذه المجموعة. كلاهما مدعوم ولا يتعارضان.

### تركيب الوكيل

تثبيت البرنامج النصي لـ Node Agent وتنزيله (عام).

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| POST | `/api/v1/agent/bootstrap` | عام | تسجيل التمهيد الأول للوكيل (تقارير الوكيل باستخدام رمز التمهيد؛ عام) |
| GET | `/api/v1/agent/releases` | المشرف | إصدارات الوكيل المتاحة (لصفحة الترقية) |
| GET | `/api/v1/agent/version` | المشرف | إصدار الوكيل الحالي على اللوحة |
| GET | `/api/v1/agent/download` | عام | وكيل التحميل |
| GET | `/api/v1/agent/install` | عام | الحصول على البرنامج النصي للتثبيت |

### تهمة إدخال رد الاتصال

رد الاتصال بمنصة الدفع التابعة لجهة خارجية (عام، تم التحقق من التوقيع).

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| POST | `/api/v1/charge/callback/:site_id` | عام | التعامل مع رد الاتصال تهمة |

### الوارد (مع العقد)

القائمة الواردة المستخدمة لاختيار عقدة الخطة.

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| GET | `/api/v1/inbounds-with-server` | تم تسجيل الدخول | جميع الواردات (بما في ذلك العقد الخاصة بها) |

### التكوين العام

تكوين وقت تشغيل البوابة الإلكترونية بدون تسجيل الدخول (حارس الصيانة + حد السعر العام).

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| GET | `/api/v1/public/articles` | عام | قائمة المقالات العامة |
| GET | `/api/v1/public/articles/:slug` | عام | تفاصيل المقال العام |
| GET | `/api/v1/public/client-configs` | عام | تكوين تنزيل العميل (عام) |
| GET | `/api/v1/public/portal-config` | عام | تكوين وقت تشغيل البوابة الإلكترونية (اسم الموقع، والموضوع، واللغات، ومستندات الامتثال؛ عامة) |
| GET | `/api/v1/public/settings` | عام | الإعدادات العامة (على سبيل المثال، Registration_enabled؛ public) |
| GET | `/api/v1/public/subscription-endpoints` | عام | قائمة نقاط نهاية الاشتراك الممكّنة (عامة) |

### مصادقة العميل والأعمال (نفس المجموعة في محرك الإدارة)

نقاط نهاية العميل المطابقة للبوابة؛ راجع `portal-api.en.md` للحصول على التفاصيل.

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| GET | `/api/v1/client-auth/articles` | عام | قائمة المقالات |
| GET | `/api/v1/client-auth/articles/:slug` | عام | الحصول على المادة |
| GET | `/api/v1/client-auth/balance` | عام | احصل على التوازن |
| GET | `/api/v1/client-auth/balance/transactions` | عام | معاملات الرصيد |
| GET | `/api/v1/client-auth/bills` | عام | قائمة الطلبات |
| POST | `/api/v1/client-auth/change-password` | عام | تغيير كلمة المرور |
| POST | `/api/v1/client-auth/charge/create` | عام | إنشاء أمر إعادة الشحن |
| GET | `/api/v1/client-auth/charge/orders` | عام | قائمة أوامر إعادة الشحن |
| GET | `/api/v1/client-auth/charge/orders/:id` | عام | احصل على تفاصيل طلب إعادة الشحن |
| POST | `/api/v1/client-auth/charge/redeem-card` | عام | استرداد البطاقة |
| GET | `/api/v1/client-auth/charge/sites` | عام | قائمة مواقع إعادة الشحن |
| GET | `/api/v1/client-auth/charge/status/:trade_no` | عام | الاستعلام عن حالة إعادة الشحن |
| GET | `/api/v1/client-auth/checktime` | عام | الحصول على وقت الخادم (مزامنة الساعة) |
| GET | `/api/v1/client-auth/connection-info` | عام | احصل على معلمات اتصال الخطة |
| GET | `/api/v1/client-auth/gifts` | عام | قائمة الهدايا |
| POST | `/api/v1/client-auth/gifts/:id/claim` | عام | المطالبة بالهدية |
| GET | `/api/v1/client-auth/invoices` | عام | قائمة الطلبات |
| GET | `/api/v1/client-auth/invoices/:id` | عام | تفاصيل الطلب |
| GET | `/api/v1/client-auth/ip-whitelist` | عام | قائمة IP المسموح بها |
| DELETE | `/api/v1/client-auth/ip-whitelist/:id` | عام | إزالة من قائمة IP المسموح بها |
| POST | `/api/v1/client-auth/ip-whitelist/auto-add` | عام | إضافة IP تلقائيًا |
| POST | `/api/v1/client-auth/login` | عام | تسجيل الدخول |
| POST | `/api/v1/client-auth/logout` | عام | تسجيل الخروج |
| POST | `/api/v1/client-auth/logs/package` | عام | إنشاء حزمة السجل |
| GET | `/api/v1/client-auth/logs/packages` | عام | قائمة حزم السجل |
| DELETE | `/api/v1/client-auth/logs/packages/:id` | عام | حذف حزمة السجل |
| GET | `/api/v1/client-auth/logs/packages/:id` | عام | الحصول على حزمة السجل |
| GET | `/api/v1/client-auth/logs/packages/:id/download` | عام | قم بتنزيل حزمة السجل |
| GET | `/api/v1/client-auth/logs/types` | عام | سجل فحص صحة الحزمة |
| GET | `/api/v1/client-auth/me` | عام | احصل على المستخدم الحالي الذي قام بتسجيل الدخول |
| GET | `/api/v1/client-auth/nodes` | عام | قائمة العقدة الخاصة بي |
| GET | `/api/v1/client-auth/nodes/:id/config` | عام | تكوين العقدة |
| GET | `/api/v1/client-auth/nodes/:id/link` | عام | رابط مشاركة العقدة |
| GET | `/api/v1/client-auth/nodes/:id/qrcode` | عام | عقدة رمز الاستجابة السريعة |
| GET | `/api/v1/client-auth/oauth/:provider` | عام | يأذن |
| GET | `/api/v1/client-auth/oauth/:provider/callback` | عام | رد الاتصال |
| GET | `/api/v1/client-auth/oauth/providers` | عام | حالة المزود |
| GET | `/api/v1/client-auth/orders` | عام | قائمة الطلبات |
| POST | `/api/v1/client-auth/orders` | عام | إنشاء الطلب |
| GET | `/api/v1/client-auth/orders/:id` | عام | تفاصيل الطلب |
| GET | `/api/v1/client-auth/plans` | عام | قائمة الخطة |
| GET | `/api/v1/client-auth/plans/:id` | عام | تفاصيل الخطة |
| GET | `/api/v1/client-auth/profile` | عام | احصل على الملف الشخصي |
| PUT | `/api/v1/client-auth/profile` | عام | تحديث الملف الشخصي |
| POST | `/api/v1/client-auth/profile/bind-oauth` | عام | ربط حساب طرف ثالث |
| DELETE | `/api/v1/client-auth/profile/unbind-oauth/:provider` | عام | إلغاء ربط حساب الطرف الثالث |
| GET | `/api/v1/client-auth/referral/code` | عام | الحصول على رمز التحقق من الإحالة |
| GET | `/api/v1/client-auth/referral/records` | عام | قائمة سجلات الإحالة |
| GET | `/api/v1/client-auth/referral/stats` | عام | احصل على إحصائيات الإحالة |
| POST | `/api/v1/client-auth/referral/withdraw` | عام | طلب الانسحاب |
| GET | `/api/v1/client-auth/referral/withdrawals` | عام | قائمة عمليات سحب الإحالة |
| POST | `/api/v1/client-auth/refresh` | عام | تحديث |
| POST | `/api/v1/client-auth/register` | عام | سجل |
| POST | `/api/v1/client-auth/reset-subscription` | عام | إعادة تعيين الاشتراك |
| POST | `/api/v1/client-auth/rotate-token` | عام | تدوير رمز الاشتراك |
| POST | `/api/v1/client-auth/self-unban` | عام | إلغاء حظر الخدمة الذاتية |
| PUT | `/api/v1/client-auth/settings` | عام | تحديث الإعدادات |
| POST | `/api/v1/client-auth/settings/auto-add-ip` | عام | إضافة IP تلقائيًا |
| GET | `/api/v1/client-auth/settings/whitelist` | عام | قائمة IP المسموح بها |
| DELETE | `/api/v1/client-auth/settings/whitelist/:id` | عام | إزالة من قائمة IP المسموح بها |
| GET | `/api/v1/client-auth/shared-accounts` | عام | قائمة الحسابات المشتركة |
| GET | `/api/v1/client-auth/shared-accounts/:id` | عام | الحصول على حساب مشترك |
| POST | `/api/v1/client-auth/shared-accounts/:id/apply` | عام | طلب حساب مشترك |
| POST | `/api/v1/client-auth/shared-accounts/:id/repair` | عام | طلب إصلاح |
| POST | `/api/v1/client-auth/ticket` | عام | إصدار تذكرة قصيرة الأجل لمرة واحدة |
| GET | `/api/v1/client-auth/tickets` | عام | قائمة التذاكر |
| POST | `/api/v1/client-auth/tickets` | عام | إنشاء تذكرة |
| GET | `/api/v1/client-auth/tickets/:id` | عام | احصل على تذكرة |
| POST | `/api/v1/client-auth/tickets/:id/close` | عام | إغلاق التذكرة |
| POST | `/api/v1/client-auth/tickets/:id/reply` | عام | الرد على التذكرة |
| GET | `/api/v1/client-auth/traffic-history` | عام | تاريخ المرور |
| GET | `/api/v1/client-auth/traffic-packs` | عام | قائمة حزمة المرور |
| POST | `/api/v1/client-auth/traffic-packs/buy` | عام | شراء حزمة المرور |

### تسليم الاشتراك

نقطة دخول عامة للعملاء لجلب الاشتراكات؛ انظر `subscription.en.md`.

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| GET | `/sub/:token` | عام | يخدم الاشتراك |

### سير عمل تكوين وقت التشغيل

لقطات التكوين / التشغيل الجاف / التطبيق / التراجع (تتطلب `apply` و`rollback` و`disable-proxy` المشرف).

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| POST | `/runtime/apply` | عام | تطبيق |
| GET | `/runtime/apply-jobs` | تم تسجيل الدخول | قائمة الوظائف |
| GET | `/runtime/apply-jobs/:id` | تم تسجيل الدخول | احصل على وظيفة |
| POST | `/runtime/apply-jobs/:id/cancel` | عام | إلغاء المهمة |
| POST | `/runtime/apply-jobs/:id/retry` | عام | أعد محاولة المهمة |
| POST | `/runtime/disable-proxy` | عام | تعطيل الوكيل |
| POST | `/runtime/dry-run` | تم تسجيل الدخول | التنفيذ الجاف |
| GET | `/runtime/nodes` | تم تسجيل الدخول | قائمة العقد |
| GET | `/runtime/overview` | تم تسجيل الدخول | نظرة عامة |
| POST | `/runtime/rollback` | عام | التراجع |
| GET | `/runtime/snapshots` | تم تسجيل الدخول | قائمة اللقطات |
| POST | `/runtime/snapshots` | تم تسجيل الدخول | إنشاء لقطة |
| DELETE | `/runtime/snapshots/:id` | عام | حذف لقطة |
| GET | `/runtime/snapshots/:id` | تم تسجيل الدخول | احصل على لقطة |
| POST | `/runtime/verify` | تم تسجيل الدخول | تحقق |

### سياسات الوصول إلى الخدمة (متقاعدة)

> تم إيقاف `/services/:id/access-policies` و`/services/:id/ip-bans` في 2026-10-20:
> لا توجد مكالمات إدارية ولا يوجد مسار اتصال يستهلك هذه السياسات.

### مراقبة الحصص المرورية

عرض مراقبة للقراءة فقط للحصص والاستخدام.

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| GET | `/traffic/quotas` | تم تسجيل الدخول | قائمة الحصص |
| GET | `/traffic/quotas/:id/usage` | تم تسجيل الدخول | استخدام الحصص |
| GET | `/traffic/reset-logs` | تم تسجيل الدخول | سجلات إعادة تعيين الحصة النسبية |
| GET | `/traffic/usage-history` | تم تسجيل الدخول | تاريخ الاستخدام |

### الفحوصات الصحية

مسبار الحياة.

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| GET | `/health` | عام | مسبار الحياة؛ 200 يعني صحي |

### صفحة البوابة

صفحة بوابة SPA.

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| GET | `/p` | عام | HTML للبوابة (يتم عرضه بواسطة الخادم + وقت تشغيل Vue) |

### تحديات ACME

رد اتصال التحدي HTTP-01.

| الطريقة | المسار | إذن | الوصف |
|---|---|---|---|
| GET | `/.well-known/acme-challenge/:token` | عام | رد الاتصال بالتحدي ACME HTTP-01 (التحقق من المجال أثناء الإصدار) |
