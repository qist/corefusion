<!-- source: oauth-google-verification.md | locale: ar | updated: 2026-10-23 -->
# Google دليل التحقق من OAuth

> اللغات: [简体中文](oauth-google-verification.md) · [English](oauth-google-verification.en.md) · [繁體中文](oauth-google-verification.zh-TW.md) · [Русский](oauth-google-verification.ru.md) · [日本語](oauth-google-verification.ja.md) · [한국어](oauth-google-verification.ko.md) · [Deutsch](oauth-google-verification.de.md) · [Français](oauth-google-verification.fr.md) · [Español](oauth-google-verification.es.md) · [Português](oauth-google-verification.pt.md) · [Türkçe](oauth-google-verification.tr.md) · [Bahasa Indonesia](oauth-google-verification.id.md) · [ไทย](oauth-google-verification.th.md) · [Tiếng Việt](oauth-google-verification.vi.md) · العربية · [فارسی](oauth-google-verification.fa.md)

> النطاق: تسجيل الدخول إلى Google لبوابة CoreFusion (`internal/service/oauth`، النطاقات: `openid`، `email`، `profile`).
> مقارنة GitHub في النهاية (لا يوجد لدى GitHub تدفق تحقق).

## 1. الاختبار مقابل وضع الإنتاج

تحتوي شاشة موافقة Google OAuth على حالتين للنشر:

| الدولة | من يمكنه تسجيل الدخول | التحقق مطلوب |
|---|---|---|
| **الاختبار** | حسابات Google فقط في قائمة "المستخدمين الاختباريين". | لا |
| **في الإنتاج** | أي حساب Google | ** نعم ** (انظر أدناه) |

بمجرد تمكين "إعدادات النظام ← OAuth"، تعرض صفحة تسجيل الدخول إلى البوابة زر Google لكل زائر؛
إذا كان التطبيق لا يزال قيد الاختبار، فسيرى الزوار غير المدرجين في قائمة المستخدمين الاختباريين
`Error 403: access_denied` (لم يتم التحقق من التطبيق / أنت لست مستخدمًا تجريبيًا).
لعرض تسجيل الدخول إلى Google بشكل علني، **قم بتبديل التطبيق إلى الإنتاج واجتياز عملية التحقق**.

## 2. المتطلبات الصعبة للتحقق

تتطلب مراجعة Google (التحقق الأساسي للنطاقات غير المقيدة):

1. **الصفحة الرئيسية**: URL يمكن الوصول إليها بشكل عام على النطاق المعتمد (نفس النطاق أو نطاق فرعي).
2. **رابط سياسة الخصوصية**: يجب أن يكون **متاحًا للعامة** (بدون تسجيل الدخول) ويشرح البيانات التي تجمعها وكيفية استخدامها/تخزينها/حذفها. لتسجيل الدخول إلى Google، يجب أن يغطي على الأقل ما يلي:
 بيانات حساب Google التي تم جمعها (البريد الإلكتروني، الاسم، الصورة الرمزية)، الغرض (إنشاء/تسجيل الدخول إلى حساب البوابة)، عدم البيع لأطراف ثالثة،
 وكيف يمكن للمستخدمين حذف حساباتهم وبياناتهم.
3. **رابط شروط الخدمة**: يمكن الوصول إليه بشكل عام، ويصف قواعد الخدمة.
4. **النطاقات المعتمدة**: قم بتسجيل نطاق التطبيق في شاشة الموافقة (على سبيل المثال، `example.com`)؛ يجب أن تكون روابط الصفحة الرئيسية/الخصوصية/المصطلحات موجودة عليه أو على نطاق فرعي.
5. **شعار التطبيق (اختياري لكن موصى به)**: 120×120 بكسل.
6. **تبرير النطاق**: يستخدم هذا المشروع `openid / email / profile` فقط (غير مقيد وغير حساس)،
 والذي يندرج تحت التحقق الأساسي - لا يوجد مبرر لكل نطاق و **لا يوجد تقييم أمني من طرف ثالث**.
 (ستتطلب النطاقات المقيدة، مثل الوصول الكامل إلى Gmail، هذه النطاقات؛ ولا يستخدم هذا المشروع أيًا منها.)

إدخال التحقق: [Google Cloud Console](https://console.cloud.google.com/) →
**واجهات برمجة التطبيقات والخدمات → شاشة موافقة OAuth → نشر التطبيق → إرسال للتحقق**.
عادةً ما يتم إكمال عملية التحقق الأساسية خلال أيام؛ يمكنك العمل في وضع الاختبار في هذه الأثناء.

## 3. ما يجب تكوينه لهذا المشروع

### 3.1 إعادة التوجيه المعتمدة URI

يتم إنشاء رد الاتصال ديناميكيًا من المجال الذي يبدأ تسجيل الدخول (راجع `Authorize` في `internal/api/handlers/oauth/handler.go`):

```
https://<portal-domain>/api/v1/client-auth/oauth/google/callback
```

- بدأ تسجيل الدخول من مجال البوابة الإلكترونية (على سبيل المثال، `https://p.example.com`) → سجل `https://p.example.com/...`؛
- بدأت من مجال اللوحة `/p` → سجل `https://panel.example.com/...`.
- يتيح Google **متعددة** إعادة توجيه عناوين URI؛ تسجيل كلا النطاقين.
- ⚠ يتطلب Google أن تكون عناوين URI لإعادة التوجيه **https** (باستثناء `http://localhost`)؛
 يجب أن يقوم الوكيل العكسي بإعادة توجيه `X-Forwarded-Host` / `X-Forwarded-Proto`، وإلا ستقوم اللوحة بإنشاء
 http أو redirect_uri الداخلي → Google يرفضه (`redirect_uri_mismatch`).

بعد ضبط "إعدادات النظام ← OAuth ← مجال رد اتصال البوابة الإلكترونية"، فإن رد الاتصال URL الموضح على الصفحة هو الذي سيتم لصقه في Google.

### 3.2 روابط سياسة الخصوصية / الشروط على البوابة

تتضمن بطاقة تسجيل الدخول روابط "سياسة الخصوصية · شروط الخدمة" (تظهر بمجرد تهيئتها)، ويحتوي كلا المستندين على **صفحات مستقلة**
(يشترك مربع الحوار والصفحة المستقلة في نفس المحتوى):

- مجال اللوحة الرئيسية: `https://<panel-domain>/p/legal/privacy`، `/p/legal/terms`
- مجال البوابة الإلكترونية المخصص (النشر الجذري): `https://<portal-domain>/legal/privacy`، `/legal/terms`
- يدعم `?lang=xx`. وبدون ذلك، يتم استخدام `Accept-Language` للمتصفح، ثم البوابة الافتراضية.
- يتم عرض الصفحات ** بواسطة الخادم ** (بدون JS)، مما يساعد زاحف التحقق الخاص بـ Google.

** التكوين (في كلتا الحالتين؛ تفوز الروابط) **:

1. **وضع المحتوى (موصى به، يعمل خارج الصندوق)**: محرر البوابة الإلكترونية ← الإعدادات ← عام ← مستندات الامتثال ←
 اختر "تحرير اللغة" والصق قوالب §4 في "محتوى سياسة الخصوصية" / "محتوى شروط الخدمة"
 (** تخفيض السعر، تم تعديله حسب اللغة **) → بعد الحفظ، تعرض بطاقة تسجيل الدخول الروابط التي تفتح **مربع الحوار**؛
 يمكن إدخال "الرابط المستقل" الخاص بالمحرر مباشرةً في شاشة موافقة Google.
2. **وضع الرابط الخارجي**: أدخل "سياسة الخصوصية URL" / "شروط الخدمة URL" (مع الإشارة إلى الصفحات التي تستضيفها) ← يفتح في علامة تبويب جديدة.

يتم تسليم المحتوى والروابط عبر `/api/v1/public/portal-config` (`privacy_policy_content` /
`terms_of_service_content` هي **خرائط Markdown لكل لغة** `{"zh-CN":"...","en":"..."}`؛
`privacy_policy_url` / `terms_of_service_url` هي روابط خارجية اختيارية). **يجب أن يكون الوصول إليها ممكنًا دون تسجيل الدخول** ✓.

### 3.3 التبديل على جانب اللوحة

`System settings → OAuth`: قم بتمكين Google وأدخل معرف العميل / سر العميل (يتم تشفير السر باستخدام AES-256-GCM في حالة عدم النشاط).
يظهر الزر Google على صفحة تسجيل الدخول إلى البوابة الإلكترونية فقط عندما يتم تكوين كل شيء (يتم تمكين إرجاع `/api/v1/client-auth/oauth/providers`).

## 4. سياسة الخصوصية / قوالب الشروط

> الحد الأدنى من نماذج الامتثال التي يمكنك التكيف معها. قبل الاستخدام: استبدل العناصر النائبة `【】`، وأضف/أزل بنودًا لعملك،
> واطلب من جهة التشغيل مراجعتها. هذه القوالب ليست مشورة قانونية.

### 4.1 سياسة الخصوصية (نموذج)

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

### 4.2 شروط الخدمة (نموذج)

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

## 5. قائمة التحقق قبل الإطلاق

- [ ] شاشة موافقة OAuth: اسم التطبيق / الشعار / الصفحة الرئيسية / سياسة الخصوصية / الشروط كلها موجودة؛ المجال المسجل
- [ ] صفحات سياسة الخصوصية والشروط **يمكن الوصول إليها دون تسجيل الدخول** (صفحة مخصصة للمدخل أو صفحة ثابتة مستقلة)
- [ ] كلا الرابطين مرئيان على بطاقة/تذييل تسجيل الدخول إلى البوابة الإلكترونية
- [ ] إعادة التوجيه URI يتطابق مع المجال الفعلي وهو https
- [ ] نشر التطبيق → إرسال للتحقق؛ يتم رفع قيود اختبار المستخدم بعد الموافقة
- [ ] إعدادات OAuth في اللوحة: ممكّنة=صحيحة مع ملء معرف العميل/السر (تمكين إرجاع نقطة النهاية للموفرين:صحيح)

## 6. مقارنة GitHub

لا تحتوي تطبيقات GitHub OAuth على **لا** حالة نشر أو تدفق تحقق: قم بإنشاء التطبيق، وقم بتعيين رد الاتصال URL، ويعمل بشكل عام.
لاحظ أن تطبيق GitHub OAuth يقبل فقط رد اتصال التفويض **واحد** URL (مطابقة البادئة)؛ لمجالات متعددة،
استخدم أسلوب URI المتعدد من Google أو قم بإنشاء تطبيق منفصل لكل مجال بوابة.
