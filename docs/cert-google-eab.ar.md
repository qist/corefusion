<!-- source: cert-google-eab.md | locale: ar | updated: 2026-10-23 -->
# دليل CA العام Google (إصدار EAB التلقائي)

> اللغات: [简体中文](cert-google-eab.md) · [English](cert-google-eab.en.md) · [繁體中文](cert-google-eab.zh-TW.md) · [Русский](cert-google-eab.ru.md) · [日本語](cert-google-eab.ja.md) · [한국어](cert-google-eab.ko.md) · [Deutsch](cert-google-eab.de.md) · [Français](cert-google-eab.fr.md) · [Español](cert-google-eab.es.md) · [Português](cert-google-eab.pt.md) · [Türkçe](cert-google-eab.tr.md) · [Bahasa Indonesia](cert-google-eab.id.md) · [ไทย](cert-google-eab.th.md) · [Tiếng Việt](cert-google-eab.vi.md) · العربية · [فارسی](cert-google-eab.fa.md)

> ينطبق على: اللوحة v0.12.101+ | الصفحة: **الشهادات ← تكوين EAB**
> الرمز: `internal/service/certificate/eab_google.go`، `internal/api/handlers/certificates/eab_config.go`

## 1. لماذا يتطلب Google EAB

| كاليفورنيا | EAB إلزامي لـ ACME؟ | كيفية الحصول على EAB |
|---|---|---|
| دعونا تشفير | ❌ لا | — |
| ZeroSSL | ✅ نعم | **تداول بريدًا إلكترونيًا مقابل واحد** (`api.zerossl.com/acme/eab-credentials-email`، نفس acme.sh)؛ يمكن للوحة جلبها تلقائيًا |
| ** Google CA العامة ** | ✅ **نعم** | **يجب أن يتم إنشاؤه بتفويض من Google Cloud Platform** (البريد الإلكتروني ليس كافيًا) → محور هذه الصفحة |

بمعنى آخر، Google **لا يمكنه تجنب "التفويض"**: إما ملف JSON لحساب الخدمة أو رمز وصول مؤقت.

## 2. القواعد الصارمة لـ Google (اقرأ أولاً)

| القاعدة | الوصف |
|---|---|
| **الاستخدام خلال 7 أيام** | مفتاح EAB **تنتهي صلاحيته** إذا لم يتم استخدامه خلال 7 أيام ويجب إعادة إنشائه |
| **حساب واحد فقط** | يقوم مفتاح EAB بتسجيل حساب ACME واحد بالضبط؛ بعد التسجيل، يصبح المفتاح **غير صالح** ولا يمكن إعادة استخدامه |
| ** حسابات ACME لا تنتهي صلاحيتها ** | حساب ACME الذي تم إنشاؤه به **طويل العمر** → **التجديدات لا تحتاج إلى EAB** |
| ⚠ **يؤدي حذف مشروع Google Cloud Platform إلى إنهاء جميع الحسابات** | يؤدي حذف المشروع على الفور إلى إبطال **جميع حسابات ACME** التابعة له و**يصبح التجديد مستحيلًا** |

> الاستنتاج: **EAB عبارة عن "بيانات اعتماد تأهيل" لمرة واحدة؛ حساب ACME هو ما يدوم.**
> تقوم اللوحة بتخزين الحسابات في `data/acme/accounts/<ca>.json` — **نسخ هذا الدليل احتياطيًا**.

## 3. ثلاثة مسارات (تبدأ بالمسار A — لا توجد قائمة للبحث)

> لقد تغيرت نقاط إدخال Cloud Console عدة مرات، لذا تأتي الروابط المباشرة وCloud Shell في المقام الأول؛ مسارات القائمة هي مسارات احتياطية.

### المسار أ (الأسهل، يوصى به أولاً): أمر واحد في GCP Cloud Shell

Cloud Shell عبارة عن محطة متصفح تم تثبيت gcloud عليها مسبقًا وتم تسجيل الدخول إليها بالفعل**؛ لا يوجد إعداد محلي ولا قوائم.

1. افتح <https://console.cloud.google.com/cloudshell> (أو أيقونة `>_` في الجزء العلوي الأيمن من وحدة التحكم)
2. حدد المشروع (معرف المشروع مرئي في منتقي المشروع في الأعلى):
   ```bash
   gcloud config set project <your-project-id>
   ```
3. تمكين CA العام API (** المكافئ ** لتمكينه في مكتبة API):
   ```bash
   gcloud services enable publicca.googleapis.com
   ```
4. قم بإنشاء EAB (** هاتان السلسلتان اللتان تحتاجهما **):
   ```bash
   gcloud publicca external-account-keys create
   ```
 في الإخراج: `keyId` → اللوحة **EAB-KID**؛ `b64MacKey` → اللوحة **EAB-HMAC-Key**
5. رجوع إلى اللوحة: **الشهادات → تكوين EAB → إضافة تكوين EAB → CA: Google Public CA** →
 أدخل القيمتين في `EAB-KID` / `EAB-HMAC-Key` → حفظ ✓

> ⚠ التكلفة: مفاتيح Google EAB **7 أيام، لمرة واحدة** ✗ → بعد 7 أيام (أو عند تبديل الحسابات) **كرر الخطوة 4**.
> لتجنب ذلك، استخدم المسار B (قم بتخزين حساب الخدمة مرة واحدة واترك اللوحة تعيد الطلب تلقائيًا).

### المسار B (الإعداد لمرة واحدة، ثم تلقائي): مفتاح حساب الخدمة JSON

**① تمكين CA العام API** (الرابط المباشر هو الأسهل)
<https://console.cloud.google.com/apis/library/publicca.googleapis.com?project=YOUR_PROJECT_ID> → انقر فوق "تمكين"
الإجراء الاحتياطي: ☰ → **واجهات برمجة التطبيقات والخدمات → المكتبة** → ابحث عن `Public CA` → فتح → "تمكين"

**② إنشاء حساب خدمة**
<https://console.cloud.google.com/iam-admin/serviceaccounts?project=YOUR_PROJECT_ID>
→ "**+ إنشاء حساب خدمة**" → أي اسم (على سبيل المثال، `corefusion-eab`) → "إنشاء ومتابعة" → يمكنك **تخطي** خطوة الدور → "تم"
الإجراء الاحتياطي: ☰ → **IAM والمسؤول → حسابات الخدمة**

**③منح الدور الأدنى** (الأسهل في نفس الصفحة)
انقر فوق الحساب الذي تم إنشاؤه حديثًا ← **علامة التبويب "الأذونات"** ← "منح الوصول" ← المدير الجديد: **البريد الإلكتروني لحساب الخدمة**
→ ابحث عن دور `publicca` → اختر **"منشئ مفتاح الحساب الخارجي العام لـ CA"** → حفظ
الإجراء الاحتياطي: ☰ → **IAM & Admin → IAM** → "منح الوصول" → نفس البريد الإلكتروني + الدور
⚠ **لا تمنح المالك/المحرر** ✗

**④ قم بتنزيل المفتاح JSON**
حسابات الخدمة ← افتح الحساب ← **علامة التبويب "مفاتيح"** ← "**إضافة مفتاح**" ← "**إنشاء مفتاح جديد**" → **JSON** ← إنشاء
(يقوم المتصفح بتنزيل `xxx.json`؛ أو الصف `⋮` → "إدارة المفاتيح" → نفس التدفق)

**⑤ مرة أخرى في اللوحة**: أدخل **معرف المشروع** والصق **المحتوى الكامل** الخاص بـ JSON في "ملف مفتاح حساب الخدمة" ← انقر على "طلب EAB تلقائيًا"
→ بعد ذلك تقوم اللوحة ** بإعادة طلب EAB تلقائيًا ** عند انتهاء صلاحيتها ✓، لا مزيد من زيارات Google Cloud Platform ✓

### المسار C (اختبار لمرة واحدة فقط): الصق رمز الوصول

في Cloud Shell:
```bash
gcloud auth print-access-token
```
الصق الإخراج في "رمز الوصول المؤقت" الخاص باللوحة بالإضافة إلى معرف المشروع → انقر فوق "طلب EAB تلقائيًا"
⚠ صالح لمدة ساعة تقريبًا و**غير مُخزن** ✓ → التحقق فقط، لا يوجد إعادة طلب تلقائي ✗

### ورقة الغش عندما لا تتمكن من العثور على نقطة دخول (تتغير قوائم وحدة التحكم)

| المهمة | الرابط المباشر (الأكثر موثوقية) | الرجوع: القائمة |
|---|---|---|
| تمكين CA العام API | `https://console.cloud.google.com/apis/library/publicca.googleapis.com?project=<PROJECT_ID>` | ☰ → واجهات برمجة التطبيقات والخدمات → المكتبة → ابحث عن `Public CA` |
| حسابات الخدمة | `https://console.cloud.google.com/iam-admin/serviceaccounts?project=<PROJECT_ID>` | ☰ → IAM والمسؤول → حسابات الخدمة |
| دور المنحة | `https://console.cloud.google.com/iam-admin/iam?project=<PROJECT_ID>` | ☰ → IAM والمسؤول → IAM → منح الوصول |
| سحابة شل | `https://console.cloud.google.com/cloudshell` | أيقونة `>_` في أعلى اليمين |

> نصيحة: استبدل `<PROJECT_ID>` / `YOUR_PROJECT_ID` بمعرف المشروع الحقيقي (مثل `my-project-123456`) للانتقال مباشرة إلى الصفحة.

## 4. تكوين اللوحة (طريقتان)

انتقل إلى **الشهادات → تكوين EAB → إضافة تكوين EAB** واختر **Google CA العام**:

### الخيار أ (مستحسن): الصق حساب الخدمة JSON

| الميدان | القيمة |
|---|---|
| رقم تعريف مشروع Google Cloud Platform | معرف المشروع من القسم 3، على سبيل المثال. `my-project-123456` |
| حساب الخدمة JSON | الصق **المحتوى الكامل** لـ `corefusion-eab.json` |
| رمز الوصول المؤقت | اتركها فارغة |
| الاسم | تم إنشاؤه تلقائيًا كـ `Google Public CA - <project-id>`؛ قابل للتحرير |

انقر **طلب EAB تلقائيًا** → عند النجاح، يتم ملء مفتاح KID/HMAC وحفظه؛ لاحقًا، ما عليك سوى اختيار `CA = Google Public CA` عند الإصدار (يرتبط EAB بكل مرجع مصدق؛ لا يوجد تحديد يدوي).

> حساب الخدمة JSON ** مشفر في تكوين EAB لقاعدة بيانات اللوحة ** (`json:"-"`، ** لم يتم إرجاعه مطلقًا إلى الواجهة الأمامية **).
> والغرض من ذلك هو **إعادة طلب المفتاح تلقائيًا بعد انتهاء الصلاحية لمدة 7 أيام** بدلاً من زيارات Google Cloud Platform اليدوية.

### الخيار ب: رمز الوصول المؤقت (لمرة واحدة)

```bash
gcloud auth print-access-token        # valid ~1 hour
```

الصقه في "رمز الوصول المؤقت"، املأ معرف المشروع، وانقر فوق **طلب EAB تلقائيًا**.
⚠ هذا الرمز **غير مخزن** ويجب إعادة الحصول عليه بعد انتهاء الصلاحية → جيد لاختبار لمرة واحدة، وليس للاستخدام على المدى الطويل.

### مكافئ سطر الأوامر (اختياري، للمقارنة)

```bash
gcloud publicca external-account-keys create
# returns keyId / b64MacKey, corresponding to ACME's --eab-kid / --eab-hmac-key
```

## 4.5 لماذا JSON بدلاً من "مفتاح واحد"؟

هذا هو السؤال الأكثر شيوعا. افصل بين نوعين من "المفتاح":

| ما قد تتوقعه | ما هو المطلوب فعلا |
|---|---|
| **مفتاح API** (سلسلة `AIza…`) | ❌ **عديم الفائدة** — مفتاح API هو مجرد معرف عام و**لا يحمل أي هوية**؛ يتطلب إنشاء EAB **أذونات IAM**، لذا يلزم وجود بيانات اعتماد تأكيد الهوية |
| — | ✅ **مفتاح حساب الخدمة**: **الملف** الذي تم تنزيله من GCP "IAM → حسابات الخدمة → المفاتيح → إنشاء مفتاح جديد → **JSON**" |

النقطة الأساسية: **ملف JSON نفسه هو "مفتاح"** — يجمع GCP هوية الحساب (`client_email`) بالإضافة إلى المفتاح الخاص (`private_key`) في ملف واحد. تقوم اللوحة بتوقيع JWT معها ← التبادل لرمز وصول OAuth2 ← استدعاء CA العام API.
لذا "الصق JSON" ≠ املأ العديد من الحقول؛ ** الصق الملف بالكامل ** (تملأ اللوحة أيضًا الحقول الاختيارية مثل `type` / `token_uri`).

**EAB الحقيقي هو السلسلتان** (`EAB-KID` + `EAB-HMAC-Key`)، وهما **نتيجة** الطلب؛ تقوم اللوحة بملئها تلقائيًا.

### لا تريد تسليم الملف؟ بديلان

| الطريقة | كيف | التكلفة |
|---|---|---|
| ** رمز الوصول المؤقت ** | `gcloud auth print-access-token` → الصق في "رمز الوصول المؤقت" | ~1 ساعة صلاحية، **غير مخزنة** → لمرة واحدة فقط، لا يوجد إعادة طلب تلقائي |
| **دليل KID/HMAC** | قم بتشغيل `gcloud publicca external-account-keys create` محليًا → أدخل `keyId` / `b64MacKey` في "EAB-KID / EAB-HMAC-Key" | لم يتم تسليم أي أوراق اعتماد ✓؛ لكن Google EAB ** 7 أيام، لمرة واحدة ** → كرر بعد انتهاء الصلاحية |

> توصي اللجنة باستخدام حساب الخدمة JSON لسبب واحد: **تنتهي صلاحية EAB بعد 7 أيام**، ولا يسمح سوى حساب الخدمة المخزن **بإعادة الطلب التلقائي** بدون عمل يدوي.

## 4.6 الأسئلة الشائعة: هل يمكنني استخدام مفتاح API؟

**لا.** هذا ليس خيار تنفيذ — **Google يرفضه صراحةً** (تم التحقق منه في كلا الاتجاهين):

```bash
# 1) Query parameter
curl -X POST "https://publicca.googleapis.com/v1/projects/<ID>/locations/global/externalAccountKeys?key=AIza..." -d '{}'
# 2) x-goog-api-key header recommended by the official docs
curl -X POST "https://publicca.googleapis.com/v1/projects/<ID>/locations/global/externalAccountKeys" \
     -H "x-goog-api-key: AIza..." -d '{}'
```

يعود كلا من:

```json
{"error":{"code":401,"status":"UNAUTHENTICATED",
  "message":"API keys are not supported by this API. Expected OAuth2 access token
             or other authentication credentials that assert a principal."}}
```

للمقارنة، يتم إرجاع **(وهمي) OAuth Bearer**
`Request had invalid authentication credentials. Expected OAuth 2 access token...` —
لذلك يتم التعرف على OAuth بينما يتم رفض مفاتيح API تمامًا** ✗.

**السبب**: إنشاء EAB هو **عملية إدارية** تتطلب معرفة "من أنت وما إذا كان لديك `roles/publicca.externalAccountKeyCreator`" ✗
— مفتاح API يحدد فقط المشروع/التطبيق ✓ و **لا يعبر عن الهوية الرئيسية** ✓.
(توصي "أفضل الممارسات لإدارة مفاتيح Google" الخاصة بـ API بـ **IAM + بيانات اعتماد حساب الخدمة قصيرة العمر ** للسيناريوهات الإدارية ✓ - نفس النهج المستخدم هنا ✓.)

** إذن هناك ثلاثة مسارات فقط ** ✓ (انظر §3): حساب الخدمة JSON ✓ / رمز الوصول المؤقت ✓ /
أو **لا توجد بيانات اعتماد على الإطلاق** — قم بتشغيل `gcloud publicca external-account-keys create` في Cloud Shell ✓
وأدخل `keyId` / `b64MacKey` في اللوحة ✓ (** استخدم هذا إذا قام GCP IAM بحظرك ** ✓).

## 5. التسجيل والتجديد التلقائي (مهم)

```
Panel: CA = Google Public CA
  ├─ usable EAB present in the config → use it
  └─ missing/expired → request a new one on the spot with the stored service-account JSON (auto-registration)
        │
First issuance on the node/panel ── registers the ACME account with EAB (one-time!)
        │
Account persisted: data/acme/accounts/google.json (panel) / <Agent data_dir>/acme/accounts/google.json (node)
        │
Every later renewal ── uses the account key only; **EAB is never needed again**
```

لذلك:

- **يتم تسجيل الإصدار الأول تلقائيًا**; ليست هناك حاجة إلى خطوة يدوية في برنامج Google Cloud Platform؛
- التجديدات **لا** تستهلك EAB (طالما أن ملف الحساب موجود ودليل البيانات مستقر)؛
- ⚠ دليل بيانات ACME من جانب الوكيل هو مسار مطلق ثابت (`/var/lib/corefusion-agent/acme` الافتراضي)؛
 **لا تغيره بشكل عرضي**، وإلا فسيتم التعامل معه كحساب جديد → مطلوب مفتاح EAB آخر.

## 6. استكشاف الأخطاء وإصلاحها

| الأعراض / الخطأ | السبب | إصلاح |
|---|---|---|
| `PERMISSION_DENIED` (403) | حساب الخدمة يفتقر إلى `roles/publicca.externalAccountKeyCreator` | ارجع إلى القسم 3 ④ |
| `SERVICE_DISABLED` (403) | لم يتم تمكين `publicca.googleapis.com` للمشروع | القسم 3 ② |
| `NOT_FOUND` (404) | معرف مشروع خاطئ / تم حذف المشروع | التحقق من معرف المشروع |
| تقارير الإصدار `externalAccountRequired` | هذا المرجع المصدق لا يحتوي على EAB | اطلب/قم بتكوين EAB في اللوحة أولاً |
| تقارير الإصدار `Accounts must have at least one contact.` | لم يتم توفير البريد الإلكتروني أثناء التسجيل | قم بتعيين البريد الإلكتروني لحساب ACME على صفحة الشهادة (تستخدم اللوحة الحساب الافتراضي) |
| يطلب التجديد EAB مرة أخرى / فشل التسجيل | تم فقد ملف حساب ACME (تم تغيير/تنظيف دليل البيانات) | **استعادة النسخة الاحتياطية**; بدون واحد، اطلب EAB جديد باستخدام حساب الخدمة |
| يتغير المُصدر من Google إلى Let's Encrypt | أمر التجديد يفتقد CA (خطأ قديم، تم إصلاحه في الإصدار 0.12.101) | ترقية وإعادة إصدار |

## 7. توصيات الأمن والعمليات

1. **الامتياز الأقل**: امنح حساب الخدمة `roles/publicca.externalAccountKeyCreator` فقط، وليس المالك/المحرر أبدًا.
2. **لا تحذف المشروع**: يؤدي حذف مشروع Google Cloud Platform إلى إبطال جميع حسابات ACME الخاصة به → لا يمكن تجديد الشهادات.
3. **النسخ الاحتياطي دائمًا** `data/acme/accounts/` (هوية الحساب نفسها) وتكوين EAB لقاعدة بيانات اللوحة.
4. **لا داعي للذعر بشأن انتهاء صلاحية EAB**: طالما أن ملف حساب ACME موجود، فلن تتأثر عمليات التجديد؛ هناك حاجة إلى EAB جديد فقط لتسجيل حساب جديد.
5. في حالة تسرب مفتاح حساب الخدمة: احذف هذا المفتاح في GCP والصق JSON جديد في اللوحة.

## 8. نقاط النهاية ذات الصلة (للبرمجة النصية)

| نقطة النهاية | الوصف |
|---|---|
| `POST /api/v1/eab-configs/fetch-google` | قم بإنشاء EAB باستخدام تفويض GCP. الجسم: `{project_id, service_account_json?, access_token?}` |
| `POST /api/v1/eab-configs/fetch-zerossl` | احصل على ZeroSSL EAB عبر البريد الإلكتروني. الجسم: `{email}` |
| `GET  /api/v1/eab-configs` | قائمة تكوينات EAB (** بدون ** مفتاح HMAC أو حساب الخدمة JSON؛ `has_mac_key` / `has_google_sa` فقط) |
