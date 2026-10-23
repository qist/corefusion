<!-- source: THIRD_PARTY_LICENSES.md | locale: ar | updated: 2026-10-23 -->
# مكونات وتراخيص الطرف الثالث

> اللغات: [简体中文](THIRD_PARTY_LICENSES.md) · [English](THIRD_PARTY_LICENSES.en.md) · [繁體中文](THIRD_PARTY_LICENSES.zh-TW.md) · [Русский](THIRD_PARTY_LICENSES.ru.md) · [日本語](THIRD_PARTY_LICENSES.ja.md) · [한국어](THIRD_PARTY_LICENSES.ko.md) · [Deutsch](THIRD_PARTY_LICENSES.de.md) · [Français](THIRD_PARTY_LICENSES.fr.md) · [Español](THIRD_PARTY_LICENSES.es.md) · [Português](THIRD_PARTY_LICENSES.pt.md) · [Türkçe](THIRD_PARTY_LICENSES.tr.md) · [Bahasa Indonesia](THIRD_PARTY_LICENSES.id.md) · [ไทย](THIRD_PARTY_LICENSES.th.md) · [Tiếng Việt](THIRD_PARTY_LICENSES.vi.md) · العربية · [فارسی](THIRD_PARTY_LICENSES.fa.md)

يستخدم CoreFusion المكونات مفتوحة المصدر التالية. يسرد هذا الملف المكونات والتراخيص الرئيسية؛ **يتم تعريف القائمة الكاملة بواسطة `go.mod` / `go.sum` وملف الترخيص الخاص بكل وحدة **.

عند توزيع الثنائيات لهذا المشروع، احتفظ بهذا الملف، والمشروع `LICENSE`، وإشعارات حقوق الطبع والنشر/الترخيص لجميع مكونات الطرف الثالث.

## 1. انتقل إلى التبعيات المجمعة في الملف الثنائي (الرئيسي)

| مكون | الترخيص | الغرض |
|---|---|---|
| `github.com/xtls/xray-core` | MPL-2.0 | تحليل تكوين Xray وأوامر gRPC وتعريفات البروتوكول |
| `github.com/xtls/reality` | MPL-2.0 | دعم بروتوكول REALITY |
| `github.com/sagernet/sing` | **GPL-3.0 أو الأحدث** | مرتبط بشكل غير مباشر عبر xray-core |
| `github.com/sagernet/sing-shadowsocks` | **GPL-3.0 أو الأحدث** | مرتبط بشكل غير مباشر عبر xray-core (Shadowsocks-2022) |
| `github.com/go-sql-driver/mysql` | MPL-2.0 | سائق MySQL |
| `github.com/hashicorp/golang-lru/v2` | MPL-2.0 | ذاكرة التخزين المؤقت LRU |
| `github.com/go-acme/lego/v4` | معهد ماساتشوستس للتكنولوجيا | إصدار شهادة ACME |
| `github.com/gin-gonic/gin` | معهد ماساتشوستس للتكنولوجيا | إطار HTTP |
| `gorm.io/gorm` | معهد ماساتشوستس للتكنولوجيا | ORM |
| `github.com/aws/aws-sdk-go-v2` | أباتشي-2.0 | النسخ الاحتياطية S3، Route53 DNS-01 |
| `google.golang.org/grpc` | أباتشي-2.0 | اتصالات وكيل gRPC |
| `google.golang.org/protobuf` | بسد-3-بند | وقت تشغيل البروتوبوف |
| `golang.org/x/crypto` | بسد-3-بند | بدايات التشفير |
| `golang.org/x/oauth2` | بسد-3-بند | OAuth2 / Google EAB |
| `github.com/golang-jwt/jwt/v5` | معهد ماساتشوستس للتكنولوجيا | JWT |
| `github.com/google/uuid` | بسد-3-بند | UUID |

> ⚠ **مكونات GPL-3.0**: `sing` و`sing-shadowsocks` هما GPL-3.0 أو أحدث. ويتم ربطها بشكل ثابت باللوحة وثنائيات الوكيل، لذا **يتطلب توزيع الثنائيات توفير المصدر المقابل** (راجع القسم 3 من [LEGAL.md](THIRD_PARTY_LICENSES.ar.md)).

## 2. البرامج المستقلة التي يتم استدعاؤها في وقت التشغيل (غير موزعة في هذا المستودع)

يجوز للوحة / الوكيل تنزيل البرامج المستقلة التالية أو تثبيتها أو استدعائها في وقت التشغيل. إنهم يتفاعلون مع هذا المشروع من خلال العمليات/الواجهات، ولكن إذا قمت بشحنهم مع التسليمات الخاصة بك، فيجب عليك الالتزام بالتراخيص الخاصة بهم:

| البرنامج | الترخيص | ملاحظات |
|---|---|---|
| Xray-النواة | MPL-2.0 | الوكيل الأساسي، عملية منفصلة |
| sing-box | **GPL-3.0 أو الأحدث** | الوكيل الأساسي، عملية منفصلة؛ توفير المصدر المقابل عند التوزيع |
| العلبة | أباتشي-2.0 | الوكيل العكسي / TLS |
| Nginx | بسد-2-بند | الوكيل العكسي (مثبت على النظام، إذا تم استخدامه) |

> Hysteria2 وTUIC هما **بروتوكولات يحملها sing-box**، وليست مراكز مستقلة أو ثنائيات، لذلك لم يتم إدراجهما بشكل منفصل.

## 3. ملفات البيانات

| البيانات | المصدر/ الترخيص |
|---|---|
| `geoip.dat` / `geosite.dat` | كما أعلن مستودع مصدر البيانات (عادةً GPL-3.0 أو CC-BY-SA) |
| GeoLite2 (`GeoLite2-ASN.mmdb`، وما إلى ذلك) | MaxMind GeoLite2 EULA / CC BY-SA 4.0؛ التحقق من الترخيص قبل الاستخدام التجاري |
| `country.mmdb` | كما أعلن مصدر بياناته |

## 4. توصيات الامتثال

- عند تسليم الثنائيات، قم بتضمين: `LICENSE`، وهذا الملف، ونص الترخيص أو الحصول على URL لكل مكون GPL/MPL.
- إذا قمت بتعديل الملفات المصدر لمكونات MPL-2.0 مثل `xray-core`، فقم بنشر مصدر الملفات المعدلة.
- إذا قمت بتوزيع برامج GPL مثل sing-box مع منتجك، فقم بتزويد المستلمين بالمصدر المطابق الكامل لذلك الإصدار (أو عرض مكتوب فعال).
- العلامات التجارية والأسماء: لا تستخدم أسماء المشاريع أو شعاراتها بطريقة تنطوي على تأييد رسمي.
