<!-- source: oauth-google-verification.md | locale: fa | updated: 2026-10-23 -->
# راهنمای تایید OAuth Google

> زبان‌ها: [简体中文](oauth-google-verification.md) · [English](oauth-google-verification.en.md) · [繁體中文](oauth-google-verification.zh-TW.md) · [Русский](oauth-google-verification.ru.md) · [日本語](oauth-google-verification.ja.md) · [한국어](oauth-google-verification.ko.md) · [Deutsch](oauth-google-verification.de.md) · [Français](oauth-google-verification.fr.md) · [Español](oauth-google-verification.es.md) · [Português](oauth-google-verification.pt.md) · [Türkçe](oauth-google-verification.tr.md) · [Bahasa Indonesia](oauth-google-verification.id.md) · [ไทย](oauth-google-verification.th.md) · [Tiếng Việt](oauth-google-verification.vi.md) · [العربية](oauth-google-verification.ar.md) · فارسی

> محدوده: ورود به سیستم Google برای پورتال CoreFusion (`internal/service/oauth`، محدوده: `openid`، `email`، `profile`).
> مقایسه GitHub در پایان (GitHub جریان تأیید ندارد).

## 1. تست در مقابل حالت تولید

صفحه رضایت Google OAuth دارای دو حالت انتشار است:

| ایالت | چه کسی می تواند وارد سیستم شود | تأیید لازم است |
|---|---|---|
| **تست** | فقط Google در لیست «کاربران آزمایشی» حساب می‌شود | خیر |
| **در حال تولید** | هر حساب Google | **بله** (به زیر مراجعه کنید) |

هنگامی که "تنظیمات سیستم → OAuth" فعال شد، صفحه ورود به پورتال دکمه Google را به هر بازدید کننده نشان می دهد.
اگر برنامه هنوز در حال آزمایش است، بازدیدکنندگانی که در لیست کاربران آزمایشی نیستند، می‌بینند
`Error 403: access_denied` (برنامه تأیید نشده است / شما کاربر آزمایشی نیستید).
برای ارائه عمومی ورود به سیستم Google، **برنامه را به تولید و تأیید تأیید عبور دهید**.

## 2. الزامات سخت برای تأیید

بررسی Google (تأیید اولیه برای محدوده‌های غیرمحدود) به موارد زیر نیاز دارد:

1. **صفحه اصلی**: یک URL در دامنه مجاز (همان دامنه یا یک زیر دامنه) در دسترس عموم قرار دارد.
2. **پیوند سیاست حفظ حریم خصوصی**: باید **در دسترس عموم باشد** (بدون ورود به سیستم) و توضیح دهد که چه داده هایی را جمع آوری می کنید و چگونه از آنها استفاده می کنید/ذخیره می کنید/حذف می کنید. برای ورود به سیستم Google باید حداقل موارد زیر را پوشش دهد:
 داده های حساب Google جمع آوری شده (ایمیل، نام، آواتار)، هدف (ایجاد/ورود به حساب پورتال)، عدم فروش به اشخاص ثالث،
 و چگونه کاربران می توانند حساب و داده های خود را حذف کنند.
3. **پیوند شرایط خدمات**: در دسترس عموم است که قوانین سرویس را شرح می دهد.
4. **دامنه های مجاز**: دامنه برنامه را در صفحه رضایت ثبت کنید (به عنوان مثال `example.com`)؛ پیوندهای خانه/حریم خصوصی/شرایط باید روی آن یا یک زیر دامنه باشد.
5. **لوگوی برنامه (اختیاری اما توصیه می شود)**: 120×120 پیکسل.
6. **توجیه محدوده**: این پروژه فقط از `openid / email / profile` (غیر محدود، غیر حساس) استفاده می کند.
 که تحت راستی‌آزمایی اولیه قرار می‌گیرد - بدون توجیه برای هر محدوده و **بدون ارزیابی امنیتی شخص ثالث**.
 (محدوده های محدود مانند دسترسی کامل به جیمیل به آنها نیاز دارد؛ این پروژه از هیچ کدام استفاده نمی کند.)

ورودی تأیید: [Google Cloud Console](https://console.cloud.google.com/) →
**APIها و خدمات → صفحه رضایت OAuth → انتشار برنامه → ارسال برای تأیید **.
تأیید اولیه معمولاً ظرف چند روز تکمیل می شود. در عین حال می توانید در حالت تست کار کنید.

## 3. چه چیزی برای این پروژه پیکربندی شود

### 3.1 تغییر مسیر مجاز URI

تماس برگشتی به صورت پویا از دامنه ای که ورود به سیستم را شروع می کند ایجاد می شود (به `Authorize` در `internal/api/handlers/oauth/handler.go` مراجعه کنید):

```
https://<portal-domain>/api/v1/client-auth/oauth/google/callback
```

- ورود به سیستم از دامنه پورتال شروع شد (به عنوان مثال `https://p.example.com`) → ثبت نام `https://p.example.com/...`؛
- از دامنه پانل `/p` → ثبت `https://panel.example.com/...` شروع شده است.
- Google به **چندین** URI تغییر مسیر می دهد. ثبت هر دو دامنه
- ⚠ Google نیاز دارد که URIهای تغییر مسیر **https** (به جز `http://localhost`) باشند.
 پراکسی معکوس باید `X-Forwarded-Host` / `X-Forwarded-Proto` را ارسال کند، در غیر این صورت پانل یک
 http یا داخلی redirect_uri → Google آن را رد می کند (`redirect_uri_mismatch`).

پس از تنظیم "تنظیمات سیستم ← OAuth ← دامنه پاسخ به تماس پورتال"، پاسخ تماس URL نشان داده شده در صفحه همان چیزی است که باید در Google جایگذاری شود.

### 3.2 سیاست حفظ حریم خصوصی / پیوندهای شرایط در پورتال

کارت ورود به سیستم شامل پیوندهای «خط‌مشی رازداری · شرایط خدمات» است (پس از پیکربندی نشان داده می‌شود)، و هر دو سند دارای **صفحات مستقل** هستند.
(دیالوگ و صفحه مستقل محتوای یکسانی دارند):

- دامنه پنل اصلی: `https://<panel-domain>/p/legal/privacy`، `/p/legal/terms`
- دامنه پورتال اختصاصی (استقرار ریشه): `https://<portal-domain>/legal/privacy`، `/legal/terms`
- پشتیبانی از `?lang=xx`؛ بدون آن، از `Accept-Language` مرورگر استفاده می شود، سپس از پیش فرض پورتال استفاده می شود.
- صفحات **ارائه شده توسط سرور** (بدون JS)، که به خزنده تاییدیه Google کمک می کند.

**پیکربندی (در هر صورت؛ پیوندها برنده می شوند)**:

1. **حالت محتوا (توصیه می شود، خارج از جعبه کار می کند)**: ویرایشگر پورتال → تنظیمات → عمومی → اسناد مطابقت →
 «زبان ویرایش» را انتخاب کنید و الگوهای §4 را در «محتوای خط‌مشی رازداری» / «محتوای شرایط خدمات» جای‌گذاری کنید.
 (**Markdown، ویرایش شده در هر زبان**) → پس از ذخیره، کارت ورود به سیستم پیوندهایی را نشان می دهد که یک **گفتگو** را باز می کنند.
 "پیوند مستقل" ویرایشگر را می توان مستقیماً در صفحه رضایت Google وارد کرد.
2. **حالت پیوند خارجی**: "خط مشی رازداری URL" / "شرایط خدمات URL" را وارد کنید (به صفحاتی که میزبان شما اشاره می کند) → در یک برگه جدید باز می شود.

محتوا و پیوندها از طریق `/api/v1/public/portal-config` (`privacy_policy_content` /
`terms_of_service_content` **نقشه های Markdown برای هر زبان** `{"zh-CN":"...","en":"..."}` هستند.
`privacy_policy_url` / `terms_of_service_url` پیوندهای خارجی اختیاری هستند). **آنها باید بدون ورود به سیستم در دسترس باشند** ✓.

### 3.3 سوئیچ سمت پنل

`System settings → OAuth`: Google را فعال کنید و Client ID / Client Secret را وارد کنید (راز با AES-256-GCM در حالت استراحت رمزگذاری شده است).
دکمه Google تنها زمانی در صفحه ورود به پورتال ظاهر می شود که همه چیز پیکربندی شده باشد (بازگرداندن `/api/v1/client-auth/oauth/providers` فعال است).

## 4. الگوهای سیاست حفظ حریم خصوصی / شرایط

> حداقل الگوهای انطباق را می توانید تطبیق دهید. قبل از استفاده: متغیرهای `【】` را جایگزین کنید، بندهایی را برای کسب و کار خود اضافه یا حذف کنید،
> و از واحد عامل بخواهید آنها را بررسی کند. این الگوها مشاوره حقوقی نیستند.

### 4.1 سیاست حفظ حریم خصوصی (الگو)

```markdown
# Privacy Policy

Last updated: [DATE]

[OPERATOR NAME] ("we") operates [PORTAL NAME] ("the Service"). We respect your privacy;
this policy explains what information we collect and how we use it.

## 1. Information we collect
- **Account information**: the email address and password you provide at registration (stored only as a one-way hash);
- **Google sign-in information**: when you choose "Sign in with Google", we receive, with your authorization,
  your Google account email, display name and avatar URL (scopes: openid, email, profile);
- **Service data**: subscription configuration, traffic usage, device IP allow-list, tickets and order records.

## 2. How we use information
Only to: create and sign in to your account, provide the proxy subscription service, bill you and provide support,
and for security controls (sign-in protection, IP allow-list). We **do not** sell your personal information.

## 3. Third-party services
- **Google**: receives account information within the authorized scope only when you actively use "Sign in with Google";
- Payment providers: receive order information only when you initiate a payment.

## 4. Storage and security
Data is stored on our servers; credentials are encrypted at rest and all traffic uses TLS.

## 5. Your rights
You can update your profile/password and end sessions in the portal at any time; to delete your account and all data,
contact [CONTACT EMAIL] and we will process the request within [N] business days.

## 6. Policy changes
Material changes will be announced in the portal notice board.
```

### 4.2 شرایط خدمات (الگو)

```markdown
# Terms of Service

Last updated: [DATE]

## 1. Service
The Service provides [PROXY / NETWORK ACCESS] to registered users; specific plans are described on the store page.

## 2. Accounts
- You are responsible for the security of your account and password; for accounts created with Google sign-in, credentials are managed by Google;
- You may not lend or transfer your account, or use the Service for any unlawful purpose (including but not limited to:
  unauthorized access to others' systems, distributing illegal content, or circumventing mandatory legal requirements).

## 3. Payment and refunds
Plans are billed [monthly / by usage]; traffic and device limits follow the plan description;
[REFUND POLICY: e.g. full refund within 7 days if unused].

## 4. Changes and termination
We may suspend or terminate the Service due to force majeure or your breach of these terms; deleting an account deletes the related data.

## 5. Disclaimer
The Service is provided "as is"; to the extent permitted by law, we are not liable for indirect losses caused by use of the Service.

## 6. Miscellaneous
These terms are governed by the laws of [JURISDICTION]. Contact: [CONTACT EMAIL].
```

## 5. چک لیست قبل از راه اندازی

- [ ] صفحه رضایت OAuth: نام برنامه / آرم / صفحه اصلی / خط مشی رازداری / شرایط همه موجود. دامنه ثبت شد
- [ ] صفحات خط مشی رازداری و شرایط **قابل دسترسی بدون ورود به سیستم** (صفحه سفارشی پورتال یا صفحه استاتیک مستقل)
- [ ] هر دو پیوند در کارت ورود به سیستم/پانویس پورتال قابل مشاهده است
- [ ] تغییر مسیر URI با دامنه واقعی مطابقت دارد و https است
- [ ] انتشار برنامه → ارسال برای تأیید؛ محدودیت کاربر آزمایشی پس از تأیید برداشته می شود
- [ ] تنظیمات پانل OAuth: enabled=true با Client ID/Secret پر شده (نقطه پایانی ارائه دهنده فعال است:true)

## 6. مقایسه GitHub

برنامه‌های GitHub OAuth دارای **بدون ** وضعیت انتشار یا جریان تأیید هستند: برنامه را ایجاد کنید، پاسخ تماس URL را تنظیم کنید و به صورت عمومی کار می‌کند.
توجه داشته باشید که یک برنامه GitHub OAuth فقط **یک** پاسخ تماس مجوز URL (تطبیق پیشوند) را می پذیرد. برای چندین دامنه،
از رویکرد چند URI از Google استفاده کنید یا یک برنامه جداگانه در هر دامنه پورتال ایجاد کنید.
