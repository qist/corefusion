<!-- source: portal-api.md | locale: ar | updated: 2026-10-23 -->
# مرجع البوابة API (العميل / البوابة API)

> اللغات: [简体中文](portal-api.md) · [English](portal-api.en.md) · [繁體中文](portal-api.zh-TW.md) · [Русский](portal-api.ru.md) · [日本語](portal-api.ja.md) · [한국어](portal-api.ko.md) · [Deutsch](portal-api.de.md) · [Français](portal-api.fr.md) · [Español](portal-api.es.md) · [Português](portal-api.pt.md) · [Türkçe](portal-api.tr.md) · [Bahasa Indonesia](portal-api.id.md) · [ไทย](portal-api.th.md) · [Tiếng Việt](portal-api.vi.md) · العربية · [فارسی](portal-api.fa.md)

> مرجع الواجهة **الكامل** للواجهة الأمامية للبوابة الإلكترونية (`/p` أو منفذ بوابة مخصص) وعملاء الجهات الخارجية.
> تعريفات المسار: `internal/api/router/router.go` (`clientAuthGroup` / `clientAuthProtected` / `/api/v1/public/*`).

## 1. الاتفاقيات العامة

### 1.1 قاعدة URL

| النشر | قاعدة URL |
|---|---|
| محرك اللوحة الرئيسية | `https://<panel-domain>/api/v1` (صفحة البوابة على `/p`) |
| منفذ بوابة مخصص | `https://<portal-domain>[:port]/api/v1` (صفحة البوابة على `/`) |
| مجال البوابة المخصصة + الوكيل العكسي | نفس ما ورد أعلاه؛ يجب أن يقوم الوكيل بإعادة توجيه مسارات الصفحة و`/api/v1` |

### 1.2 المصادقة

- بعد تسجيل الدخول أرسل `Authorization: Bearer <access_token>`;
- رموز الوصول تدوم 24 ساعة، رموز التحديث 7 أيام؛
- يحتوي JWT على `tv` (token_version): يؤدي تسجيل الخروج/تسجيل الخروج القسري إلى زيادته وإبطال جميع الرموز المميزة القديمة
 (`clients.token_version`، راجع `internal/service/clientauth/service.go`)؛
- تمر كل نقطة نهاية محمية عبر `ClientAuthMiddleware` (فحص الإصدار + حماية وضع الصيانة).

### 1.3 مغلف الاستجابة

تُرجع معظم نقاط النهاية `{ "code": 0, "data": ..., "message": "..." }`؛
تُرجع بعض نقاط نهاية القائمة `{ "items": [...], "total": N }` أو مصفوفة مجردة (مذكورة لكل نقطة نهاية أدناه).
أخطاء ترجع `{ "code": "ERROR_CODE", "message": "...", "request_id": "...", "can_retry": ... }`
(HTTP 4xx/5xx).

### 1.4 وحدات المبلغ

حقول الطلب/الرصيد هي **عدد صحيح**: `990` = ¥9.90 (`currency: "CNY"`).

### 1.5 مثال حقيقي (تسجيل الدخول → جلب الملف الشخصي)

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

على الرقم 401، اتصل بـ `POST /api/v1/client-auth/refresh` مرة واحدة ثم أعد المحاولة؛ إذا استمر الفشل، فامسح الجلسة المحلية (يقوم قالب البوابة بذلك بالفعل).

---

## 2. نقاط النهاية العامة (بدون تسجيل الدخول)

| الطريقة | المسار | الوصف |
|---|---|---|
| GET | `/p` (`/` على محرك بوابة مخصص) | HTML للبوابة (يتم عرضه بواسطة الخادم + وقت تشغيل Vue) |
| GET | `/api/v1/public/portal-config` | تكوين وقت تشغيل البوابة الإلكترونية: اسم الموقع، ألوان السمات، التنقل، الوحدات النمطية، حزم اللغات، `registration_enabled`، `regions` (مصدر منطقة تسجيل الدخول)، `privacy_policy_url` / `terms_of_service_url` / `privacy_policy_content` / `terms_of_service_content` (مستندات الامتثال؛ المحتوى المعين لكل لغة؛ الصفحات المستقلة `/p/legal/*` ✓) |
| GET | `/api/v1/public/settings` | الإعدادات العامة (حاليًا `registration_enabled`) |
| GET | `/api/v1/public/subscription-endpoints` | قائمة نقاط نهاية الاشتراك (مصفوفة عارية) |
| GET | `/api/v1/public/articles` | المقالات العامة (المنشورة فقط) |
| GET | `/api/v1/public/articles/:slug` | تفاصيل المادة |
| GET | `/api/v1/public/client-configs` | تكوين تنزيل العميل |
| GET | `/sub/:token` | تسليم الاشتراك (`format` يُرجع base64_uri / Raw_uri / mihomo_yaml /ingbox_json / xray_json، وما إلى ذلك؛ ملغى/مفقود → 404، معلق/منتهية الصلاحية/غير نشط → 403. راجع [subscription.md](portal-api.ar.md)) |
| GET | `/api/v1/client-auth/oauth/providers` | رؤية موفر OAuth: `{"data":{"providers":[{"provider":"github","enabled":true},...]}}` |
| GET | `/api/v1/system/maintenance` | حالة الصيانة (عامة). تعرض البوابة صفحة الصيانة عند تمكينها |

> كل هذه تشمل حارس الصيانة وتحديد معدل API العام.

## 3. المصادقة / OAuth

| الطريقة | المسار | مصادقة | الوصف |
|---|---|---|---|
| POST | `/api/v1/client-auth/register` | — | سجل. الجسم: `{email, password, referral_code?}`. إرجاع `{access_token, refresh_token, client}`؛ معدل التسجيل محدود |
| POST | `/api/v1/client-auth/login` | — | تسجيل الدخول. الجسم: `{email, password}`. نفس الرد؛ يصدر JWTs مع `token_version` |
| POST | `/api/v1/client-auth/refresh` | — | تحديث. الجسم: `{refresh_token}`. إرجاع رموز الوصول/التحديث الجديدة (تم فحص الإصدار؛ تموت رموز التحديث القديمة مباشرة بعد تسجيل الخروج) |
| POST | `/api/v1/client-auth/logout` | ✓ | تسجيل الخروج: إبطال جانب الخادم (`token_version +1`) → جميع الرموز المميزة القديمة للعميل غير صالحة |
| POST | `/api/v1/client-auth/rotate-token` | ✓ | قم بتدوير رمز الاشتراك |
| POST | `/api/v1/client-auth/ticket` | ✓ | قم بإصدار **تذكرة قصيرة الأجل لمرة واحدة** (لتدفقات `window.open` مثل "حزمة سجل التنزيل" حيث لا يمكن تعيين الرؤوس) |
| GET | `/api/v1/client-auth/oauth/:provider` | — | بدء OAuth: 307 → GitHub/Google موافقة (`state` anti-CSRF، 10 دقائق)؛ مقدمو الخدمات المعوقين يعودون 400 |
| GET | `/api/v1/client-auth/oauth/:provider/callback` | — | رد اتصال OAuth: رمز التبادل ← إنشاء/ربط العميل ← إصدار الإصدار JWT → 302 العودة إلى البوابة الإلكترونية (`/<portalPath>?access_token=...&refresh_token=...`) |

> رد اتصال OAuth هو **إعادة توجيه المتصفح** (وليس استدعاء API): بعد تسجيل الدخول الناجح، تقرأ البوابة الإلكترونية الرمز المميز
> من URL في `onMounted` (أو عبر `postMessage` من نفس الأصل، راجع `handleOAuthCallback` في قالب البوابة `web`).

## 4. الحساب

| الطريقة | المسار | الوصف |
|---|---|---|
| GET | `/api/v1/client-auth/me` | العميل الحالي: `{id, email, plan_id, plan_name, status, quota_bytes, used_bytes, upload_bytes, download_bytes, expires_at, ip_whitelist, device_limit, sub_token, sub_status, created_at, ...}` (مغلف `data`) |
| POST | `/api/v1/client-auth/change-password` | الجسم: `{old_password, new_password}` |
| GET | `/api/v1/client-auth/checktime` | وقت الخادم (مزامنة ساعة العميل) |
| GET | `/api/v1/client-auth/connection-info` | معلمات اتصال الخطة الحالية: `{concurrent, device_limit, protocol, speed_limit, tls, transport}` |

## 5. الاشتراكات

| الطريقة | المسار | الوصف |
|---|---|---|
| POST | `/api/v1/client-auth/reset-subscription` | إعادة تعيين الاشتراك؛ هذا يدور `sub_token` |

> تم إنشاء رابط الاشتراك على الواجهة الأمامية من `sub_token` الذي تم إرجاعه بواسطة `/me` (`/sub/:token`)؛ لا يوجد
> فصل نقاط النهاية `subscription-link` / `subscription-formats` بعد الآن.

## 6. العقد

| الطريقة | المسار | الوصف |
|---|---|---|
| GET | `/api/v1/client-auth/nodes` | العقد المرئية للخطة الحالية: `{items:[...]}` (عرض الحقول مثل الاسم/البروتوكول/النقل/المنطقة) |
| GET | `/api/v1/client-auth/nodes/:id/config` | تكوين العقدة الكاملة |
| GET | `/api/v1/client-auth/nodes/:id/link` | مشاركة الرابط (vless://إلخ.) |
| GET | `/api/v1/client-auth/nodes/:id/qrcode` | صورة رمز الاستجابة السريعة |

## 7. تخزين / خطط / أوامر

| الطريقة | المسار | الوصف |
|---|---|---|
| GET | `/api/v1/client-auth/plans` | قائمة الخطط (المصفوفة العارية): `{id, name, description, category, sort_order, variants:[{id, name, billing_cycle, duration_days, price(cents), original_price, currency, quota_bytes, ...}]}` |
| GET | `/api/v1/client-auth/plans/:id` | تفاصيل الخطة |
| GET | `/api/v1/client-auth/traffic-packs` | قائمة حزمة المرور |
| POST | `/api/v1/client-auth/traffic-packs/buy` | شراء حزمة المرور |
| GET | `/api/v1/client-auth/orders` | أوامر خطتي (مرقّمة الصفحات) |
| POST | `/api/v1/client-auth/orders` | إنشاء أمر (شراء/تجديد متغير الخطة) |
| GET | `/api/v1/client-auth/orders/:id` | تفاصيل الطلب |
| GET | `/api/v1/client-auth/invoices` | الفواتير |
| GET | `/api/v1/client-auth/invoices/:id` | تفاصيل الفاتورة |

## 8. التوازن / إعادة الشحن

| الطريقة | المسار | الوصف |
|---|---|---|
| GET | `/api/v1/client-auth/balance` | `{data:{client_id, balance(cents), frozen}}` |
| GET | `/api/v1/client-auth/balance/transactions` | معاملات الرصيد (`type`: تحصيل الرسوم/الاستهلاك/استرداد الأموال؛ مرقّمة) |
| GET | `/api/v1/client-auth/charge/sites` | مواقع إعادة الشحن المتاحة (`ListChargeSites`: ممكّنة فقط) |
| POST | `/api/v1/client-auth/charge/create` | إنشاء طلب إعادة شحن (يعيد التوجيه إلى دفع طرف ثالث) |
| GET | `/api/v1/client-auth/charge/orders` | إعادة شحن قائمة الطلبات |
| GET | `/api/v1/client-auth/charge/orders/:id` | تفاصيل طلب إعادة الشحن |
| GET | `/api/v1/client-auth/charge/status/:trade_no` | حالة دفع الاستطلاع بواسطة trade_no |
| POST | `/api/v1/client-auth/charge/redeem-card` | استبدال بطاقة إعادة الشحن |

> يؤدي حذف العميل **التتالي** إلى رصيده ومعاملاته (راجع `DeleteClient` في `internal/service/client/service.go`).

## 9. الهدايا / تاريخ المرور

| الطريقة | المسار | الوصف |
|---|---|---|
| GET | `/api/v1/client-auth/gifts` | الهدايا التي يمكن المطالبة بها (المصفوفة العارية؛ `type`: أيام/بايت) |
| POST | `/api/v1/client-auth/gifts/:id/claim` | المطالبة |
| GET | `/api/v1/client-auth/traffic-history` | سجل حركة المرور الشهري: `{data:{history:[{period_start,total_bytes,...}], months}}` |

## 10. التذاكر

| الطريقة | المسار | الوصف |
|---|---|---|
| GET | `/api/v1/client-auth/tickets` | قائمة التذاكر: `{items,total}` |
| POST | `/api/v1/client-auth/tickets` | إنشاء تذكرة |
| GET | `/api/v1/client-auth/tickets/:id` | تفاصيل التذكرة (مع الردود) |
| POST | `/api/v1/client-auth/tickets/:id/reply` | رد |
| POST | `/api/v1/client-auth/tickets/:id/close` | إغلاق |

## 11. مقالات / أدلة

| الطريقة | المسار | الوصف |
|---|---|---|
| GET | `/api/v1/client-auth/articles` | قائمة المقالات (مع مرشح الفئة) |
| GET | `/api/v1/client-auth/articles/:slug` | تفاصيل المادة |

## 12. الإحالات (يتم تعطيلها افتراضيًا، ويمكن تمكينها في محرر البوابة الإلكترونية)

| الطريقة | المسار | الوصف |
|---|---|---|
| GET | `/api/v1/client-auth/referral/code` | رمز الإحالة الخاص بي والرابط |
| GET | `/api/v1/client-auth/referral/stats` | إحصائيات العمولة (`commission_rate`، `available_commission`، `min_withdraw_amount`) |
| GET | `/api/v1/client-auth/referral/records` | سجلات الإحالة |
| GET | `/api/v1/client-auth/referral/withdrawals` | سجلات الانسحاب |
| POST | `/api/v1/client-auth/referral/withdraw` | طلب الانسحاب |

## 13. الحسابات المشتركة

| الطريقة | المسار | الوصف |
|---|---|---|
| GET | `/api/v1/client-auth/shared-accounts` | الحسابات المشتركة المتاحة (البث، الخ) |
| GET | `/api/v1/client-auth/shared-accounts/:id` | التفاصيل |
| POST | `/api/v1/client-auth/shared-accounts/:id/apply` | طلب الوصول |
| POST | `/api/v1/client-auth/shared-accounts/:id/repair` | إصلاح بنقرة واحدة |

## 14. حزم السجل (تشخيصات المشترك)

| الطريقة | المسار | الوصف |
|---|---|---|
| GET | `/api/v1/client-auth/logs/types` | أنواع السجلات التي يمكن تجميعها |
| GET | `/api/v1/client-auth/logs/packages` | حزم السجل الخاص بي |
| POST | `/api/v1/client-auth/logs/package` | إنشاء مهمة تجميع السجل (عقد الإدخال فقط) |
| GET | `/api/v1/client-auth/logs/packages/:id` | حالة الوظيفة (جاهز / قيد المعالجة / فاشلة) |
| GET | `/api/v1/client-auth/logs/packages/:id/download` | تنزيل (فضل `POST /ticket` وألحق `?ticket=`) |
| DELETE | `/api/v1/client-auth/logs/packages/:id` | حذف حزمة السجل |

## 15. إعدادات الأمان / قائمة IP المسموح بها / إلغاء الحظر الذاتي

| الطريقة | المسار | الوصف |
|---|---|---|
| GET/PUT | `/api/v1/client-auth/settings` | إعدادات العميل: `{ip_whitelist, device_limit}` (PUT) |
| GET | `/api/v1/client-auth/settings/whitelist` | القائمة المسموح بها (المصفوفة العارية) |
| DELETE | `/api/v1/client-auth/settings/whitelist/:id` | حذف إدخال القائمة المسموح بها |
| POST | `/api/v1/client-auth/settings/auto-add-ip` | أضف عنوان IP الحالي إلى القائمة المسموح بها |
| POST | `/api/v1/client-auth/self-unban` | إلغاء حظر الخدمة الذاتية (المحاولات المقيدة بالسياسة) |

> قائمة IP المسموح بها **تنطبق فقط على عناوين URL الخاصة بالاشتراك** (`/sub/:token`)؛ لا يمنع تسجيل الدخول إلى البوابة الإلكترونية/OAuth أو
> نقاط نهاية مصادقة العميل الأخرى. إذا لم يكن لدى المستخدم قائمة مسموح بها أو لم يكن لديه اشتراك URL، فسيتم السماح بالوصول دائمًا. ال
> القائمة المسموح بها هي إعداد لكل مستخدم؛ لا يوجد التبديل العالمي.

**تدفق إلغاء الحظر الذاتي**: متاح فقط لـ `auto_quota` (الحظر التلقائي على زيادة حركة المرور) عندما يكون الجانب الإداري
`self_unban.enabled` قيد التشغيل. يؤدي تسجيل الدخول (كلمة المرور أو رد اتصال OAuth) إلى إرجاع `require_self_unban=true` + `self_unban_token`
(typ=self_unban، صالحة لمدة 15 دقيقة، **فقط** قابلة للاستخدام على `POST /self-unban`؛ وترجع نقاط النهاية الأخرى 403/`SELF_UNBAN_TOKEN_SCOPE`).
ثم تظهر البوابة بطاقة إلغاء الحظر؛ بعد إلغاء الحظر بنجاح، يقوم المستخدم بتسجيل الدخول مرة أخرى. تقوم عمليات رد اتصال OAuth بتسليم هذا عبر
جزء URL `#require_self_unban=1&self_unban_token=..&self_unban_hint=..`.

## 16. رموز الخطأ (مقتطفات)

| كود | السيناريو |
|---|---|
| `INVALID_TOKEN` | رمز التحديث غير صالح/منتهي الصلاحية/الإصدار غير متطابق |
| `INVALID_REQUEST` | فشل التحقق من صحة المعلمة |
| `NOT_FOUND` | المورد مفقود (تذكرة/طلب/إدخال القائمة المسموح بها، وما إلى ذلك) |
| `RELOAD_FAILED` / `SYNC_FAILED` | (جانب المسؤول) فشلت مزامنة تكوين الوكيل |
| 401 + `errClientTokenRevoked` | عدم تطابق token_version (تسجيل الخروج/الطرد) → يجب على العملاء التحديث؛ إذا فشل ذلك، قم بمسح الرموز المميزة المحلية |

## 17. وثائق الامتثال (سياسة الخصوصية / شروط الخدمة)

روابط "سياسة الخصوصية · شروط الخدمة" الموجودة أسفل بطاقة تسجيل الدخول (مثل المواقع العادية؛ يؤدي النقر فوق إلى فتح المحتوى):

- **التكوين**: محرر البوابة الإلكترونية ← الإعدادات ← عام ← مستندات الامتثال (**تحرير المحتوى لكل لغة**)؛
 يمكنك أيضًا `PUT /api/v1/portal/active` مع الحقول المنظمة `privacy_policy_url` / `privacy_policy_content` /
 `terms_of_service_url` / `terms_of_service_content` (لا شيء = لم يتغير؛ المحتوى عبارة عن خريطة `{"zh-CN":"...","en":"..."}`).
- **صفحات مستقلة يمكن الوصول إليها** (يتم عرضها بواسطة الخادم، وسهلة لبرامج زحف التحقق Google؛ وتدعم `?lang=xx`):
  - مجال اللوحة الرئيسية: `GET /p/legal/privacy`، `GET /p/legal/terms`
  - مجال البوابة الإلكترونية المخصص (النشر الجذري): `GET /legal/privacy`، `GET /legal/terms`
- **منطق العرض**: URL أو المحتوى، **URL يفوز** (يفتح خارجيًا في علامة تبويب جديدة)؛ إذا تم توفير المحتوى، فانقر فوق
 تعرض بطاقة تسجيل الدخول Markdown في مربع حوار (`renderMd`) باستخدام لغة البوابة الإلكترونية الحالية.
- يتم التسليم في وقت التشغيل عبر `/api/v1/public/portal-config`؛ يستهلك مكون تسجيل الدخول
 `ctx.privacyUrl / termsUrl / privacyContent / termsContent / openLegal / hasLegalLinks`.
- يتطلب التحقق من OAuth Google أن تكون هذه الروابط متاحة للعامة ← استخدم الروابط المستقلة أعلاه في
 شاشة الموافقة؛ راجع `docs/oauth-google-verification.md` (يحتوي على قوالب سياسة الخصوصية/المصطلحات).

## 18. مثال التكامل

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

على الرقم 401، اتصل بـ `POST /refresh` مرة واحدة ثم أعد المحاولة؛ إذا استمر الفشل، قم بمسح الجلسة المحلية
(يطبق `api()` الخاص بقالب البوابة هذا بالفعل).
