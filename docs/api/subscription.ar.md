<!-- source: subscription.md | locale: ar | updated: 2026-10-23 -->
# اشتراك API (عام)

> اللغات: [简体中文](subscription.md) · [English](subscription.en.md) · [繁體中文](subscription.zh-TW.md) · [Русский](subscription.ru.md) · [日本語](subscription.ja.md) · [한국어](subscription.ko.md) · [Deutsch](subscription.de.md) · [Français](subscription.fr.md) · [Español](subscription.es.md) · [Português](subscription.pt.md) · [Türkçe](subscription.tr.md) · [Bahasa Indonesia](subscription.id.md) · [ไทย](subscription.th.md) · [Tiếng Việt](subscription.vi.md) · العربية · [فارسی](subscription.fa.md)

جلب قائمة عقدة اشتراك المستخدم. **هذه هي نقطة النهاية الأكثر استخدامًا للمستخدمين النهائيين**؛ ليست هناك حاجة إلى رؤوس - يتم منح الوصول بواسطة الرمز المميز الموجود في URL.

## 1. نقطة النهاية

| النشر | URL |
|---|---|
| اللوحة الرئيسية | `GET https://<panel-domain>/sub/<token>` |
| منفذ اشتراك مخصص | `GET https://<sub-domain>:<port>/<token>` (بدون بادئة `/sub`) |

`<token>` هو رمز الوصول الفريد للاشتراك، الذي تم إنشاؤه بواسطة جانب المسؤول (مرئي ضمن "اشتراكاتي" في البوابة).

## 2. معلمات الاستعلام

| المعلمة | الوصف |
|---|---|
| `format` | تنسيق الإخراج؛ يتجاوز الاشتراك الافتراضي. انظر أدناه |
| `config` | اسم قالب التكوين (المحدد ضمن "تنسيقات الاشتراك" في واجهة مستخدم المسؤول). يجب استخدام نفس تنسيق `format` لهذا الطلب، وإلا فسيتم تجاهله |
| `platform` | مطابقة قالب التكوين حسب اسم النظام الأساسي (يُستخدم عند حذف `config`)؛ يجب أن يتطابق التنسيق أيضًا |

### قيم `format` المدعومة

| القيمة | الإخراج | العملاء |
|---|---|---|
| `base64_uri` (افتراضي) | روابط مشاركة مرتبطة بخط جديد، مشفرة بـ base64 ككل | V2RayN وShadowrocket وSurge وعملاء عامون آخرون |
| `raw_uri` | روابط مشاركة مرتبطة بسطر جديد، نص عادي | العملاء الذين يحتاجون إلى روابط / تصحيح أخطاء عادي |
| `mihomo_yaml` | التكوين الكامل لـ Clash/Mihomo YAML (مع مجموعات الوكيل والقواعد) | Clash.Meta / mihomo / Clash Verge Rev |
| `mihomo_proxies_yaml` | YAML مع قائمة العقد `proxies:` فقط | مصدر مزود وكيل mihomo (مع التحديث المجدول) |
| `singbox_json` | التكوين الكامل لـ sing-box JSON | sing-box / SFI / FLClash (sing-box كور) |
| `xray_json` | التكوين الكامل لـ Xray JSON | Xray-النواة |

> ينطبق توافق البروتوكول/التنسيق: على سبيل المثال، `anytls` يظهر فقط في تنسيقات sing-box/Mihomo؛ لا تظهر عقد REALITY الموجودة على خادم sing-box في اشتراكات Xray/sing-box JSON. يتم إسقاط العقد التي لا يستطيع العميل التعرف عليها تلقائيًا بدلاً من فشل الاشتراك بالكامل.

## 3. رؤوس الاستجابة

| رأس | مثال | الوصف |
|---|---|---|
| `Subscription-Userinfo` | `upload=1024; download=2048; total=1073741824; expire=1760000000` | الاستخدام وانتهاء الصلاحية (بايت / ثانية يونكس). **تم الإرسال أيضًا على الرقم 403** حتى يتمكن العملاء من إظهار الرصيد المتبقي |
| `Profile-Update-Interval` | `6` | الفاصل الزمني للتحديث المقترح (ساعات) |
| `Profile-Title` | `CoreFusion` | اسم الملف الشخصي (يتم عرضه بواسطة Clash Verge Rev / V2RayN، وما إلى ذلك) |
| `Profile-Web-Page-Url` | `https://…/portal` | رابط صفحة البوابة ("فتح صفحة الويب" في العملاء) |
| `Content-Type` | لكل تنسيق: `text/plain` / `text/yaml` / `application/json` |  |
| `Content-Disposition` | `attachment; filename="subscription.txt"` |  |

## 4. رموز الحالة

| HTTP | كود | معنى |
|---|---|---|
| 200 | — | عادي (النص هو الاشتراك المقدم في `format`) |
| 400 | `MISSING_TOKEN` | لا يوجد رمز مميز في URL |
| 403 | (نص عادي) | الاشتراك معلق / منتهي الصلاحية / غير نشط. الرسالة مترجمة (zh/en/ja/ko) عبر `Accept-Language` وهي سبب من سطر واحد |
| 404 | `SUBSCRIPTION_NOT_FOUND` | الرمز المميز غير موجود |

> يتم التعامل مع الاشتراكات الملغاة على أنها غير موجودة (404).

## 5. أمثلة

```bash
# 1) Fetch with the default format
curl -L "https://panel.example.com/sub/abcd1234efgh5678"

# 2) Request Mihomo YAML with the "clash-global" config template
curl -L "https://panel.example.com/sub/abcd1234efgh5678?format=mihomo_yaml&config=clash-global"

# 3) Node list only (for a mihomo proxy-provider)
curl -L "https://panel.example.com/sub/abcd1234efgh5678?format=mihomo_proxies_yaml"

# 4) Check usage (headers only)
curl -sI "https://panel.example.com/sub/abcd1234efgh5678" | grep -i subscription-userinfo
```

الاشتراك الديناميكي في Clash/Mihomo (تحديث دوري):

```yaml
proxy-providers:
  subscription-nodes:
    type: http
    url: "https://panel.example.com/sub/abcd1234efgh5678?format=mihomo_proxies_yaml"
    interval: 3600
    health-check:
      enable: true
      url: https://www.gstatic.com/generate_204
      interval: 300

proxy-groups:
  - name: PROXY
    type: select
    use:
      - subscription-nodes
```

## 6. ملاحظات

- **التنسيق والتكوين مقترنان بإحكام**: يجب أن يتطابق القالب المحدد بواسطة `config`/`platform` مع `format` لهذا الطلب؛ يتم تجاهل القوالب غير المتطابقة - وهذا يمنع إدخال قواعد Mihomo في sing-box/Xray JSON.
- **تحديد المعدل**: نقطة نهاية الاشتراك لها حد خاص بها (`SubscribeRateLimit`)؛ يتم رفض العملاء المسيئين.
- **سجل الوصول**: تسجل كل عملية جلب معلومات IP / UA / CDN، والتي تظهر ضمن "سجل الوصول إلى الاشتراك" في واجهة المستخدم الإدارية.
- بدون تكوين مجال على منفذ اشتراك مخصص، يتم استخدام `X-Forwarded-Proto` / `X-Forwarded-Host` للطلب؛ تأكد من قيام الوكيل العكسي بإعادة توجيه كلا الرأسين.
