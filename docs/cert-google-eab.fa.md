<!-- source: cert-google-eab.md | locale: fa | updated: 2026-10-23 -->
# راهنمای CA عمومی Google (صدور خودکار EAB)

> زبان‌ها: [简体中文](cert-google-eab.md) · [English](cert-google-eab.en.md) · [繁體中文](cert-google-eab.zh-TW.md) · [Русский](cert-google-eab.ru.md) · [日本語](cert-google-eab.ja.md) · [한국어](cert-google-eab.ko.md) · [Deutsch](cert-google-eab.de.md) · [Français](cert-google-eab.fr.md) · [Español](cert-google-eab.es.md) · [Português](cert-google-eab.pt.md) · [Türkçe](cert-google-eab.tr.md) · [Bahasa Indonesia](cert-google-eab.id.md) · [ไทย](cert-google-eab.th.md) · [Tiếng Việt](cert-google-eab.vi.md) · [العربية](cert-google-eab.ar.md) · فارسی

> برای: panel v0.12.101+ | صفحه: **گواهینامه ها → پیکربندی EAB**
> کد: `internal/service/certificate/eab_google.go`, `internal/api/handlers/certificates/eab_config.go`

## 1. چرا Google به EAB نیاز دارد

| CA | EAB برای ACME اجباری است؟ | چگونه EAB را دریافت کنیم |
|---|---|---|
| بیایید رمزگذاری کنیم | ❌ خیر | — |
| ZeroSSL | ✅ بله | **تجارت یک ایمیل برای یک ** (`api.zerossl.com/acme/eab-credentials-email`، همان acme.sh)؛ پانل می تواند آن را به طور خودکار واکشی کند |
| **Google عمومی CA** | ✅ **بله** | **باید با مجوز GCP ایجاد شود** (ایمیل کافی نیست) → تمرکز این صفحه |

به عبارت دیگر، Google **نمی تواند از "مجوز" اجتناب کند**: یا یک فایل JSON با حساب سرویس یا یک رمز دسترسی موقت.

## 2. قوانین سخت Google (ابتدا بخوانید)

| قانون | توضیحات |
|---|---|
| **در مدت 7 روز استفاده کنید** | یک کلید EAB **در صورت عدم استفاده در عرض 7 روز منقضی می شود** و باید دوباره ایجاد شود. |
| **فقط یک حساب** | یک کلید EAB دقیقاً یک حساب ACME را ثبت می کند. پس از ثبت نام، کلید **بی اعتبار است** و قابل استفاده مجدد نیست |
| **حساب های ACME منقضی نمی شوند** | حساب ACME ایجاد شده با آن **دوامدار است** → **تجدید نیازی به EAB نیست** |
| ⚠ **حذف پروژه GCP همه حساب ها را از بین می برد** | حذف پروژه بلافاصله **همه حساب های ACME** تحت آن را باطل می کند و **تمدید غیرممکن می شود** |

> نتیجه‌گیری: **EAB یک "اعتبار ورود" یک بار مصرف است. حساب ACME همان چیزی است که دوام می آورد.**
> این پنل حساب ها را در `data/acme/accounts/<ca>.json` ذخیره می کند — **از این دایرکتوری پشتیبان تهیه کنید**.

## 3. سه مسیر (شروع با مسیر A - بدون شکار منو)

> نقاط ورودی Cloud Console چندین بار تغییر کرده است، بنابراین پیوندهای مستقیم و Cloud Shell در اولویت قرار دارند. مسیرهای منو بازگشتی هستند.

### مسیر A (ساده ترین، ابتدا توصیه می شود): یک دستور در GCP Cloud Shell

Cloud Shell یک ترمینال مرورگر با **gcloud از پیش نصب شده و قبلاً وارد شده** است. بدون تنظیمات محلی و بدون منو.

1. <https://console.cloud.google.com/cloudshell> را باز کنید (یا نماد `>_` در سمت راست بالای کنسول)
2. پروژه را انتخاب کنید (شناسه پروژه در انتخابگر پروژه در بالا قابل مشاهده است):
   ```bash
   gcloud config set project <your-project-id>
   ```
3. Public CA API را فعال کنید (**معادل** برای فعال کردن آن در کتابخانه API):
   ```bash
   gcloud services enable publicca.googleapis.com
   ```
4. EAB را ایجاد کنید (**این دو رشته ای هستند که شما نیاز دارید**):
   ```bash
   gcloud publicca external-account-keys create
   ```
 در خروجی: `keyId` → پنل **EAB-KID**؛ `b64MacKey` → **EAB-HMAC-Key** پنل
5. بازگشت به پنل: **گواهینامه ها → پیکربندی EAB → افزودن پیکربندی EAB → CA: Google CA عمومی ** →
 دو مقدار را در `EAB-KID` / `EAB-HMAC-Key` → ذخیره ✓ وارد کنید

> ⚠ هزینه: کلیدهای Google EAB **7 روزه، یکبار مصرف** ✗ → بعد از 7 روز (یا هنگام تعویض حساب) **مرحله 4** را تکرار کنید.
> برای جلوگیری از آن، از مسیر B استفاده کنید (یک حساب سرویس را یک بار ذخیره کنید و اجازه دهید پانل دوباره به طور خودکار درخواست کند).

### مسیر B (تنظیم یکباره، سپس خودکار): کلید سرویس-حساب JSON

**① عمومی CA API** را فعال کنید (لینک مستقیم ساده ترین است)
<https://console.cloud.google.com/apis/library/publicca.googleapis.com?project=YOUR_PROJECT_ID> → روی «فعال کردن» کلیک کنید
بازگشتی: ☰ → **API ها و خدمات → کتابخانه** → جستجوی `Public CA` → باز کردن → «فعال کردن»

**② یک حساب خدمات ایجاد کنید**
<https://console.cloud.google.com/iam-admin/serviceaccounts?project=YOUR_PROJECT_ID>
→ «**+ ایجاد حساب سرویس**» → هر نامی (به عنوان مثال `corefusion-eab`) → «ایجاد و ادامه» → می توانید **از مرحله نقش بگذرید → «انجام شد»
بازگشتی: ☰ → **IAM & Admin → حساب های خدمات **

**③ اعطای حداقل نقش ** (ساده ترین نقش در همان صفحه)
روی حساب جدید ایجاد شده کلیک کنید → **برگه «مجوزها»** → «اعطای دسترسی» → اصل جدید: **ایمیل حساب خدمات**
← نقش `publicca` را جستجو کنید ← **"Public CA External Account Creator"** → ذخیره را انتخاب کنید
بازگشتی: ☰ → **IAM & Admin → IAM** → "Grant Access" → همان ایمیل + نقش
⚠ **به مالک/ویرایشگر اعطا نکنید** ✗

**④ دانلود کلید JSON**
حساب‌های خدماتی → باز کردن حساب → **برگه "کلیدها"** → "**افزودن کلید**" → "**ایجاد کلید جدید**" → **JSON** → ایجاد
(مرورگر یک `xxx.json` دانلود می کند؛ یا ردیف `⋮` → «مدیریت کلیدها» → جریان مشابه)

**⑤ بازگشت به پنل**: **شناسه پروژه** را وارد کنید و **کل محتوای** JSON را در "فایل کلید حساب سرویس" قرار دهید → روی "درخواست خودکار EAB" کلیک کنید.
→ پس از آن، پانل **زمانی که منقضی شد، EAB را مجدداً درخواست می کند** ✓، دیگر از GCP بازدید نمی شود ✓

### مسیر C (فقط تست یکباره): یک نشانه دسترسی را بچسبانید

در Cloud Shell:
```bash
gcloud auth print-access-token
```
خروجی را در "Token Access Temporary Access" پانل به اضافه شناسه پروژه بچسبانید → روی "Request EAB automatically" کلیک کنید.
⚠ حدود 1 ساعت معتبر است و **ذخیره نشده** ✓ → فقط تأیید، بدون درخواست مجدد خودکار ✗

### برگه تقلب زمانی که نمی توانید نقطه ورودی را پیدا کنید (منوهای کنسول تغییر می کند)

| وظیفه | لینک مستقیم (معتبرترین) | بازگشت: منو |
|---|---|---|
| Public CA API را فعال کنید | `https://console.cloud.google.com/apis/library/publicca.googleapis.com?project=<PROJECT_ID>` | ☰ → API ها و خدمات → کتابخانه → جستجوی `Public CA` |
| حساب های خدماتی | `https://console.cloud.google.com/iam-admin/serviceaccounts?project=<PROJECT_ID>` | ☰ → IAM & Admin → حساب های خدمات |
| نقش اعطا کنید | `https://console.cloud.google.com/iam-admin/iam?project=<PROJECT_ID>` | ☰ → IAM & Admin → IAM → اجازه دسترسی |
| پوسته ابری | `https://console.cloud.google.com/cloudshell` | نماد `>_` در بالا سمت راست |

> نکته: `<PROJECT_ID>` / `YOUR_PROJECT_ID` را با شناسه پروژه واقعی (مانند `my-project-123456`) جایگزین کنید تا مستقیماً به صفحه بروید.

## 4. پیکربندی پنل (دو روش)

به **Certificates → EAB configuration → Add EAB configuration** بروید و **Google Public CA** را انتخاب کنید:

### گزینه A (توصیه می شود): حساب سرویس JSON را جایگذاری کنید

| میدان | ارزش |
|---|---|
| شناسه پروژه GCP | شناسه پروژه از بخش 3، به عنوان مثال. `my-project-123456` |
| حساب سرویس JSON | **کل محتوای** `corefusion-eab.json` را جایگذاری کنید |
| رمز دسترسی موقت | خالی بگذارید |
| نام | تولید خودکار به عنوان `Google Public CA - <project-id>`؛ قابل ویرایش |

روی **درخواست خودکار EAB** کلیک کنید → در صورت موفقیت، کلید KID/HMAC پر شده و ذخیره می شود. بعداً فقط `CA = Google Public CA` را هنگام صدور انتخاب کنید (EAB در هر CA مرتبط است؛ انتخاب دستی وجود ندارد).

> حساب سرویس JSON **در پیکربندی EAB پایگاه داده پنل رمزگذاری شده است** (`json:"-"`، **هرگز به قسمت جلویی بازنگشت**).
> هدف این است که پس از انقضای 7 روزه **به جای بازدیدهای دستی GCP، یک کلید به صورت خودکار درخواست شود.

### گزینه B: رمز دسترسی موقت (یکباره)

```bash
gcloud auth print-access-token        # valid ~1 hour
```

آن را در «توکن دسترسی موقت» قرار دهید، شناسه پروژه را پر کنید، روی ** درخواست EAB به طور خودکار ** کلیک کنید.
⚠ این نشانه **ذخیره نمی شود** و باید پس از انقضا دوباره تهیه شود → برای آزمایش یکباره جریمه است، نه برای استفاده طولانی مدت.

### معادل خط فرمان (اختیاری، برای مقایسه)

```bash
gcloud publicca external-account-keys create
# returns keyId / b64MacKey, corresponding to ACME's --eab-kid / --eab-hmac-key
```

## 4.5 چرا JSON به جای "یک کلید"؟

این رایج ترین سوال است. دو نوع "کلید" را از هم جدا کنید:

| آنچه شما ممکن است انتظار داشته باشید | آنچه در واقع مورد نیاز است |
|---|---|
| **کلید API** (رشته `AIza…`) | ❌ **بی فایده** — یک کلید API فقط یک شناسه عمومی است و **هیچ هویتی ندارد**. ایجاد EAB به **مجوزهای IAM** نیاز دارد، بنابراین اعتبار تایید هویت مورد نیاز است |
| — | ✅ **کلید حساب سرویس**: **فایل** دانلود شده از GCP "IAM → حساب های سرویس → کلیدها → ایجاد کلید جدید → **JSON**" |

نکته کلیدی: **فایل JSON خود یک "کلید" است** — GCP هویت حساب (`client_email`) و کلید خصوصی (`private_key`) را در یک فایل بسته بندی می کند. پانل یک JWT را با آن امضا می کند ← برای یک توکن دسترسی OAuth2 مبادله می کند ← CA عمومی API را فراخوانی می کند.
بنابراین "JSON" را بچسبانید ≠ بسیاری از فیلدها را پر کنید. **کل فایل را جایگذاری کنید** (پنل همچنین فیلدهای اختیاری مانند `type` / `token_uri` را پر می کند).

**EAB واقعی دو رشته** (`EAB-KID` + `EAB-HMAC-Key`) است که **نتیجه** درخواست است. پانل آنها را به طور خودکار پر می کند.

### آیا نمی خواهید پرونده را تحویل دهید؟ دو جایگزین

| روش | چگونه | هزینه |
|---|---|---|
| **توکن دسترسی موقت** | `gcloud auth print-access-token` ← چسباندن در «توکن دسترسی موقت» | اعتبار ~ 1 ساعت، **ذخیره نشده** → فقط یکبار، بدون درخواست مجدد خودکار |
| **دستی KID/HMAC** | `gcloud publicca external-account-keys create` را به صورت محلی اجرا کنید → `keyId` / `b64MacKey` را در "EAB-KID / EAB-HMAC-Key" وارد کنید | هیچ اعتباری تحویل داده نشد ✓؛ اما Google EAB **7 روزه است، یک بار** → بعد از انقضا تکرار کنید |

> پانل حساب سرویس JSON را به یک دلیل توصیه می کند: **EAB پس از 7 روز منقضی می شود**، و فقط یک حساب سرویس ذخیره شده امکان ** درخواست مجدد خودکار ** بدون کار دستی را می دهد.

## 4.6 سؤالات متداول: آیا می توانم از کلید API استفاده کنم؟

**خیر** این یک انتخاب پیاده سازی نیست — **Google به صراحت آن را رد می کند** (هر دو روش تأیید شده است):

```bash
# 1) Query parameter
curl -X POST "https://publicca.googleapis.com/v1/projects/<ID>/locations/global/externalAccountKeys?key=AIza..." -d '{}'
# 2) x-goog-api-key header recommended by the official docs
curl -X POST "https://publicca.googleapis.com/v1/projects/<ID>/locations/global/externalAccountKeys" \
     -H "x-goog-api-key: AIza..." -d '{}'
```

هر دو برمی گردند:

```json
{"error":{"code":401,"status":"UNAUTHENTICATED",
  "message":"API keys are not supported by this API. Expected OAuth2 access token
             or other authentication credentials that assert a principal."}}
```

برای مقایسه، یک **(جعلی) OAuth Bearer** برمی گردد
`Request had invalid authentication credentials. Expected OAuth 2 access token...` -
بنابراین OAuth شناسایی می شود در حالی که **کلیدهای API کاملاً رد می شوند** ✗.

**چرا**: ایجاد EAB یک **عملیات اداری** است که نیاز به دانستن "چه کسی هستید و آیا `roles/publicca.externalAccountKeyCreator` دارید" است ✗
— یک کلید API فقط یک پروژه/برنامه را شناسایی می کند ✓ و **هویت اصلی را بیان نمی کند** ✓.
(«بهترین شیوه‌ها برای مدیریت کلیدهای API» Google ** IAM + اعتبارنامه‌های حساب خدمات کوتاه مدت** را برای سناریوهای اداری توصیه می‌کند - همان رویکردی که در اینجا استفاده می‌شود ✓.)

**بنابراین فقط سه مسیر وجود دارد** ✓ (نگاه کنید به §3): حساب سرویس JSON ✓ / رمز دسترسی موقت ✓ /
یا **اصلاً فاقد اعتبار** — `gcloud publicca external-account-keys create` را در Cloud Shell اجرا کنید ✓
و `keyId` / `b64MacKey` را در پانل وارد کنید ✓ (**اگر GCP IAM شما را مسدود کرد از این استفاده کنید** ✓).

## 5. ثبت نام و تمدید خودکار (مهم)

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

بنابراین:

- **اولین صدور به صورت خودکار ثبت می شود**; هیچ مرحله GCP دستی مورد نیاز نیست.
- تمدیدها ** EAB را مصرف نمی کنند (تا زمانی که فایل حساب وجود دارد و فهرست داده ها ثابت می ماند).
- ⚠ فهرست راهنمای داده ACME سمت نماینده یک مسیر مطلق ثابت است (پیش‌فرض `/var/lib/corefusion-agent/acme`).
 **به طور اتفاقی آن را تغییر ندهید**، در غیر این صورت به عنوان یک حساب جدید → کلید EAB دیگری مورد نیاز است.

## 6. عیب یابی

| علامت / خطا | علت | رفع کنید |
|---|---|---|
| `PERMISSION_DENIED` (403) | حساب سرویس فاقد `roles/publicca.externalAccountKeyCreator` است | به بخش 3 ④ برگردید |
| `SERVICE_DISABLED` (403) | `publicca.googleapis.com` برای پروژه فعال نیست | بخش 3 ② |
| `NOT_FOUND` (404) | شناسه پروژه اشتباه / پروژه حذف شد | شناسه پروژه را تایید کنید |
| گزارش های صدور `externalAccountRequired` | این CA فاقد EAB است | ابتدا EAB را در پانل درخواست/پیکربندی کنید |
| گزارش های صدور `Accounts must have at least one contact.` | در هنگام ثبت نام ایمیلی ارائه نشده است | ایمیل حساب ACME را در صفحه گواهی تنظیم کنید (پنل از حساب پیش فرض استفاده می کند) |
| تمدید دوباره EAB را درخواست می کند / ثبت نام انجام نمی شود | فایل حساب ACME گم شد (دایرکتوری داده تغییر / پاک شد) | **بازیابی نسخه پشتیبان**؛ بدون یکی، با استفاده از حساب سرویس، یک EAB جدید درخواست کنید |
| صادرکننده از Google به Let's Encrypt تغییر می کند | دستور تمدید فاقد CA (اشکال قدیمی، رفع شده در نسخه 0.12.101) | ارتقا و انتشار مجدد |

## 7. توصیه های امنیتی و عملیاتی

1. **حداقل امتیاز**: به حساب سرویس فقط `roles/publicca.externalAccountKeyCreator` اعطا کنید، هرگز مالک/ویرایشگر.
2. **پروژه را حذف نکنید**: حذف پروژه GCP همه حساب های ACME آن را باطل می کند → گواهی ها نمی توانند تمدید شوند.
3. **همیشه از `data/acme/accounts/` (خود هویت حساب) و پیکربندی EAB پایگاه داده پانل نسخه پشتیبان تهیه کنید.
4. **درمورد انقضای EAB وحشت نکنید**: تا زمانی که فایل حساب ACME وجود دارد، تمدیدها تحت تأثیر قرار نمی گیرند. یک EAB جدید فقط برای ثبت حساب جدید مورد نیاز است.
5. اگر ممکن است کلید حساب سرویس لو رفته باشد: آن کلید را در GCP حذف کنید و یک JSON جدید را در پانل قرار دهید.

## 8. نقاط پایانی مرتبط (برای اسکریپت نویسی)

| نقطه پایانی | توضیحات |
|---|---|
| `POST /api/v1/eab-configs/fetch-google` | با استفاده از مجوز GCP یک EAB ایجاد کنید. بدنه: `{project_id, service_account_json?, access_token?}` |
| `POST /api/v1/eab-configs/fetch-zerossl` | یک ZeroSSL EAB را با یک ایمیل دریافت کنید. بدنه: `{email}` |
| `GET  /api/v1/eab-configs` | پیکربندی‌های EAB را فهرست کنید (**بدون ** کلید HMAC یا حساب سرویس JSON؛ فقط `has_mac_key` / `has_google_sa`) |
