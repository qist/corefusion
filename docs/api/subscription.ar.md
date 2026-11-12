<!-- source: subscription.md | locale: ar | updated: 2026-11-12 -->
# اشتراك API (عام)

> اللغات: [简体中文](subscription.md) · [English](subscription.en.md) · [繁體中文](subscription.zh-TW.md) · [Русский](subscription.ru.md) · [日本語](subscription.ja.md) · [한국어](subscription.ko.md) · [Deutsch](subscription.de.md) · [Français](subscription.fr.md) · [Español](subscription.es.md) · [Português](subscription.pt.md) · [Türkçe](subscription.tr.md) · [Bahasa Indonesia](subscription.id.md) · [ไทย](subscription.th.md) · [Tiếng Việt](subscription.vi.md) · العربية · [فارسی](subscription.fa.md)

جلب قائمة عقدة اشتراك المستخدم. **هذه هي نقطة النهاية الأكثر استخدامًا للمستخدمين النهائيين**؛ لا توجد رؤوس مطلوبة - يتم منح الوصول بواسطة الرمز المميز الموجود في URL.

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
| `providers` | `mihomo_yaml` فقط: `1` / `true` يعمل على تمكين **وضع الموفر** (أصدر `proxy-providers` وقم بالإشارة إليها عبر `use`)؛ القيمة الافتراضية أو أي قيمة أخرى تحافظ على العقد مضمنة تحت `proxies`. انظر الفقرة 6 |

### قيم `format` المدعومة

| القيمة | الإخراج | العملاء |
|---|---|---|
| `base64_uri` (افتراضي) | روابط مشاركة مرتبطة بخط جديد، مشفرة بـ base64 ككل | V2RayN وShadowrocket وSurge وعملاء عامون آخرون |
| `raw_uri` | روابط مشاركة مرتبطة بسطر جديد، نص عادي | العملاء الذين يحتاجون إلى روابط / تصحيح أخطاء عادي |
| `mihomo_yaml` | التكوين الكامل لـ Clash/Mihomo YAML (مع مجموعات الوكيل والقواعد) | Clash.Meta / mihomo / Clash Verge Rev |
| `mihomo_proxies_yaml` | YAML مع قائمة العقد `proxies:` فقط | مصدر موفر الوكيل mihomo (مع التحديث المجدول) |
| `singbox_json` | التكوين الكامل لـ sing-box JSON | sing-box / SFI / FLClash (sing-box كور) |
| `xray_json` | التكوين الكامل لـ Xray JSON | Xray-النواة |

> ينطبق توافق البروتوكول/التنسيق: على سبيل المثال، `anytls` يظهر فقط في تنسيقات sing-box/Mihomo؛ **تم استبعاد العقد REALITY التي تخدمها xray من اشتراكات sing-box** (sing-box لا يدعم تطبيق xray REALITY، راجع sing-box#4520) ولكنها لا تزال تظهر في اشتراكات Xray، بينما **REALITY تعمل العقد التي تخدمها sing-box في كل من اشتراكات Xray وsing-box**. يتم إسقاط العقد التي لا يستطيع العميل التعرف عليها تلقائيًا بدلاً من فشل الاشتراك بالكامل.

## 3. رؤوس الاستجابة

| رأس | مثال | الوصف |
|---|---|---|
| `Subscription-Userinfo` | `upload=1024; download=2048; total=1073741824; expire=1760000000` | الاستخدام وانتهاء الصلاحية (بايت / ثانية يونكس). **تم الإرسال أيضًا على الرقم 403** حتى يتمكن العملاء من إظهار الرصيد المتبقي |
| `Profile-Update-Interval` | `6` | الفاصل الزمني للتحديث المقترح (ساعات) |
| `Profile-Title` | `CoreFusion` | اسم الملف الشخصي (يتم عرضه بواسطة Clash Verge Rev / V2RayN، وما إلى ذلك) |
| `Profile-Web-Page-Url` | `https://portal.example.com` | Portal URL ("صفحة الويب المفتوحة" في العملاء؛ المسار الجذر عندما يكون للبوابة منفذ خاص بها، وإلا /p) |
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

# 5) Clash dynamic subscription: config template + Provider mode
curl -L "https://sub.example.com/abcd1234efgh5678?format=clash&config=out&providers=1"

# 6) sing-box / Xray with their own config templates (rules come from the template)
curl -L "https://sub.example.com/abcd1234efgh5678?format=singbox_json&config=sing-box-home"
curl -L "https://sub.example.com/abcd1234efgh5678?format=xray_json&config=xray-home"
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

## 6. وضع الموفر ومجالات الاشتراك المتعددة

### 6.1 ما ينبعث من `providers=1`

مع `format=mihomo_yaml&providers=1` (أو `format=clash&providers=1`) YAML:

- لم تعد تتضمن العقد في المستوى الأعلى - فهي تنبعث منها `proxy-providers:`؛
- مجموعات الوكيل تشير إلى مصادر العقدة عبر `use: [<provider-key>...]`؛
- أي `proxy-providers` مكتوب بتنسيق اشتراك مشترك (`mihomo_configs.config_json`) يتم **تجاهله** (يخدم التكوين المشترك العديد من الخطط؛ وقد يتسرب مقدمو الخدمة المشفرون عبر الخطط). المجموعة ← تعيينات المصدر مباشرة ضمن "إدارة الخطة ← المجموعة ← تعيين المصدر".

أنواع المزود:

| نوع | مفتاح | ملاحظات |
|---|---|---|
| داخلي | اسم مجموعة العقدة المعقمة | يتم إنشاؤها لكل مجموعة عقدة خطة؛ يحتوي فقط على عقد تلك المجموعة |
| محفوظة | `subscription-nodes` | جميع العقد في الخطة؛ مصدره URL ** يتجاهل ** معلمة `provider` |
| خارجي | الاسم الذي تم تكوينه على الخطة | مصدر خارجي (`http` YAML أو `file`) |

الحقول المشتركة لكل مزود:

- `url` - يشير إلى هذا الاشتراك مع `format=mihomo_proxies_yaml`، حيث يرث `config` / `platform` من الطلب الرئيسي؛ يقوم الموفرون الداخليون بإلحاق `provider=<key>`؛
- `path` — **مستمد دائمًا من "افتراضيات الموفر الداخلي ← قالب مسار ذاكرة التخزين المؤقت"** (الافتراضي `./providers/{key}.yaml`، تم استبدال `{key}`). ولا يستطيع مقدمو الخدمات الخارجيون تحديد المسار الخاص بهم أيضًا؛
- `health-check` - `enable / url / interval`؛ يرث مقدمو الخدمة الداخليون الإعدادات الافتراضية، ويقوم مقدمو الخدمات الخارجيون بتكوينها على الخطة؛
- `interval` - الفاصل الزمني للتحديث بالثواني.

### 6.2 المجال الذي ينتقل إلى الموفر URL (متعدد المجالات)

قد يحتوي "مركز الاشتراك → نقاط نهاية الاشتراك" على **عدة** سجلات (المجال + المسار + ممكّن). يختار الموفر `url` المجال بهذا الترتيب:

1. **المجال الذي استخدمه هذا الطلب بالفعل**: القيمة الأولى لـ `X-Forwarded-Host` (تم إزالة المنفذ)، وإلا `Host`؛ يجب أن يتطابق مع نقطة نهاية **ممكّنة**؛
2. إذا لم يتطابق أي شيء، فارجع إلى نقطة النهاية الممكّنة ذات **المعرف الأدنى**؛
3. يأتي المسار من نقطة النهاية المطابقة (`path`، `/` الافتراضية)، مما يؤدي إلى `https://<domain><path>/<token>?format=mihomo_proxies_yaml...`.

العواقب:

- يحصل العميل الذي يقوم بجلب نطاق جديد على هذا النطاق داخل الموفر - **لا حاجة إلى تغيير اللوحة**؛
- المجال القديم الذي لا يزال ممكّنًا في القائمة لم يعد مشفرًا؛
- إذا قام العميل بتخزين الموفر القديم URL مؤقتًا، فإن تحديث الاشتراك (أو مسح ذاكرة التخزين المؤقت للموفر / إعادة تشغيل العميل) يؤدي إلى تبديله.

يجب أن يقوم الوكلاء العكسيون بإعادة توجيه `X-Forwarded-Proto` و`X-Forwarded-Host`؛ وإلا يتم استخدام نقطة النهاية الاحتياطية. تتضمن قوالب موقع Caddy / Nginx التي تم إنشاؤها بواسطة اللوحة بالفعل كلا الرأسين (`header_up X-Forwarded-Host {host}` / `proxy_set_header X-Forwarded-Host $host;`)؛ قم بإضافتها يدويًا إذا كنت تحافظ على تكوين الوكيل يدويًا.

## 7. ملاحظات

- **التنسيق والتكوين مقترنان بإحكام**: يجب أن يتطابق القالب المحدد بواسطة `config`/`platform` مع `format` لهذا الطلب؛ يتم تجاهل القوالب غير المتطابقة — وهذا يمنع إدخال قواعد Mihomo في sing-box/Xray JSON.
- ** `providers=1` يؤثر فقط على Mihomo YAML **: تجاهلها sing-box / Xray / base64 / الخام (تظل العقد مضمنة).
- **تحديد المعدل**: نقطة نهاية الاشتراك لها حد خاص بها (`SubscribeRateLimit`)؛ يتم رفض العملاء المسيئين.
- **سجل الوصول**: تسجل كل عملية جلب معلومات IP / UA / CDN، والتي تظهر ضمن "سجل الوصول إلى الاشتراك" في واجهة المستخدم الإدارية.
- بدون تكوين مجال على منفذ اشتراك مخصص، يتم استخدام `X-Forwarded-Proto` / `X-Forwarded-Host` للطلب؛ تأكد من قيام الوكيل العكسي بإعادة توجيه كلا الرأسين.
- في عمليات النشر متعددة النطاقات، احتفظ بقائمة نقاط النهاية متوافقة مع المجالات التي يستخدمها العملاء فعليًا؛ انظر §6.2.
