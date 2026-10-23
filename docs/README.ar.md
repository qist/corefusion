<!-- source: README.md | locale: ar | updated: 2026-10-23 -->
# وثائق CoreFusion

> اللغات: [简体中文](README.md) · [English](README.en.md) · [繁體中文](README.zh-TW.md) · [Русский](README.ru.md) · [日本語](README.ja.md) · [한국어](README.ko.md) · [Deutsch](README.de.md) · [Français](README.fr.md) · [Español](README.es.md) · [Português](README.pt.md) · [Türkçe](README.tr.md) · [Bahasa Indonesia](README.id.md) · [ไทย](README.th.md) · [Tiếng Việt](README.vi.md) · العربية · [فارسی](README.fa.md)

## [USER-GUIDE.ar.md](./USER-GUIDE.ar.md) — دليل المستخدم

دليل عمليات كامل **المستخدمين** (العمليات/المسؤولين/المشغلين): التثبيت والتكوين الأولي ووحدات الميزات والأسئلة الشائعة والملحق.

## [api/](./api/) — مرجع API العام

بالنسبة للتكاملين والتطوير الثانوي وعملاء الطرف الثالث:

- [api/README.ar.md](./api/README.ar.md) - نظرة عامة: قاعدة URL، المصادقة، تنسيق الاستجابة، حدود المعدل
- [api/subscription.ar.md](./api/subscription.ar.md) — نقطة نهاية الاشتراك (`/sub/:token`)، التنسيقات، رؤوس الاستجابة، أمثلة
- [api/portal-api.ar.md](./api/portal-api.ar.md) — بوابة العميل API (التسجيل / تسجيل الدخول، الخطط، الطلبات، الرصيد، التذاكر)
- [api/charge-callback.ar.md](./api/charge-callback.ar.md) - التوقيع والتحقق من رد الاتصال بالدفع
- [api/admin-api.ar.md](./api/admin-api.ar.md) - جدول توجيه المشرف API مجمعًا حسب الوحدة النمطية

## وثائق التصميم

وثائق التصميم هي مواد داخلية (ملكية فكرية أساسية) و**لا يتم توزيعها** مع هذا المستودع. اتصل بالمشرف إذا كنت في حاجة إليها.

> **النطاق**: تطبيق العميل (واجهة المستخدم المحلية، واستيلاء TUN، والدرج، ودمج الاشتراكات المتعددة) هو منتج منفصل وهو خارج نطاق هذا المستودع. تغطي الوثائق هنا جانب الخادم فقط (اللوحة/العقد/تسليم الاشتراك).

## أدلة العمليات الأخرى

- `cert-google-eab.md` — إصدار شهادة Google EAB *(الصينية)*
- `oauth-google-verification.md` — Google التحقق من OAuth *(الصينية)*
- `notification-templates.md` — متغيرات قالب الإشعارات *(الصينية)*

## تغطية الترجمة

| وثيقة | zh-CN | en | zh-TW | أماكن أخرى |
|----------|-------|----|-------|---------------|
| README.md | ✅ | ✅ | ✅ | ru ✅ |
| USER-GUIDE.md | ✅ | ✅ | ✅ | ru ✅ |
| I18N.md | ✅ | — | — | — |
| api/README.md | ✅ | ✅ | ✅ | ru ✅ |
| api/subscription.md | ✅ | ✅ | ✅ | ru ✅ |
| api/portal-api.md | ✅ | ✅ | ✅ | ru ✅ |
| api/charge-callback.md | ✅ | ✅ | ✅ | ru ✅ |
| api/admin-api.md | ✅ | ✅ | ✅ | ru ✅ |
| cert-google-eab.md | ✅ | ✅ | ✅ | ru ✅ |
| oauth-google-verification.md | ✅ | ✅ | ✅ | ru ✅ |
| notification-templates.md | ✅ | ✅ | ✅ | ru ✅ |
| LEGAL.md | ✅ | ✅ | ✅ | ru ✅ |
| THIRD_PARTY_LICENSES.md | ✅ | ✅ | ✅ | ru ✅ |

راجع [I18N.md](README.ar.md) للتسمية والتنقل اللغوي وقواعد المراجعة *(الصينية)*.
