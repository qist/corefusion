<!-- source: THIRD_PARTY_LICENSES.md | locale: ar | updated: 2026-11-19 -->
# مكونات وتراخيص الطرف الثالث

> اللغات: [简体中文](THIRD_PARTY_LICENSES.md) · [English](THIRD_PARTY_LICENSES.en.md) · [繁體中文](THIRD_PARTY_LICENSES.zh-TW.md) · [Русский](THIRD_PARTY_LICENSES.ru.md) · [日本語](THIRD_PARTY_LICENSES.ja.md) · [한국어](THIRD_PARTY_LICENSES.ko.md) · [Deutsch](THIRD_PARTY_LICENSES.de.md) · [Français](THIRD_PARTY_LICENSES.fr.md) · [Español](THIRD_PARTY_LICENSES.es.md) · [Português](THIRD_PARTY_LICENSES.pt.md) · [Türkçe](THIRD_PARTY_LICENSES.tr.md) · [Bahasa Indonesia](THIRD_PARTY_LICENSES.id.md) · [ไทย](THIRD_PARTY_LICENSES.th.md) · [Tiếng Việt](THIRD_PARTY_LICENSES.vi.md) · العربية · [فارسی](THIRD_PARTY_LICENSES.fa.md)

يستخدم CoreFusion المكونات مفتوحة المصدر التالية. يسرد هذا الملف المكونات والتراخيص الرئيسية؛ **يتم تحديد القائمة الكاملة بواسطة `go.mod` / `go.sum` و`web/admin/package.json` وملف الترخيص الخاص بكل وحدة**.

عند توزيع الثنائيات لهذا المشروع، احتفظ بهذا الملف، والمشروع `LICENSE`، وإشعارات حقوق الطبع والنشر/الترخيص لجميع مكونات الطرف الثالث.

## 1. انتقل إلى التبعيات المجمعة في الملف الثنائي (الرئيسي)

| مكون | الترخيص | الغرض |
|---|---|---|
| `github.com/xtls/xray-core` | MPL-2.0 | تحليل تكوين Xray وأوامر gRPC وتعريفات البروتوكول |
| `github.com/xtls/reality` | MPL-2.0 | دعم بروتوكول REALITY |
| `github.com/sagernet/sing` | **GPL-3.0-or-later** | مرتبط بشكل غير مباشر عبر xray-core |
| `github.com/sagernet/sing-shadowsocks` | **GPL-3.0-or-later** | مرتبط بشكل غير مباشر عبر xray-core (Shadowsocks-2022) |
| `github.com/go-sql-driver/mysql` | MPL-2.0 | سائق MySQL |
| `github.com/hashicorp/golang-lru/v2` | MPL-2.0 | ذاكرة التخزين المؤقت LRU |
| `github.com/go-acme/lego/v5` | MIT | إصدار شهادة ACME |
| `github.com/gin-gonic/gin` | MIT | إطار HTTP |
| `gorm.io/gorm` | MIT | ORM |
| `gorm.io/driver/mysql` | MIT | برنامج تشغيل MySQL لـ GORM |
| `gorm.io/driver/postgres` | MIT | برنامج تشغيل PostgreSQL لـ GORM |
| `github.com/jackc/pgx/v5` | MIT | سائق PostgreSQL |
| `github.com/qist/sqlite` | MIT | برنامج تشغيل SQLite (قاعدة البيانات الافتراضية للوحة / الوكيل) |
| `github.com/aws/aws-sdk-go-v2` | Apache-2.0 | النسخ الاحتياطية S3، Route53 DNS-01 |
| `google.golang.org/grpc` | Apache-2.0 | اتصالات وكيل gRPC |
| `google.golang.org/protobuf` | BSD-3-Clause | وقت تشغيل البروتوبوف |
| `golang.org/x/crypto` | BSD-3-Clause | بدايات التشفير |
| `golang.org/x/oauth2` | BSD-3-Clause | OAuth2 / Google EAB |
| `golang.org/x/sys` | BSD-3-Clause | مكالمات النظام (تحديد معدل nft/tc، وإدارة العمليات) |
| `github.com/golang-jwt/jwt/v5` | MIT | JWT |
| `github.com/google/uuid` | BSD-3-Clause | UUID |
| `github.com/go-webauthn/webauthn` | BSD-3-Clause | تسجيل الدخول Passkey / WebAuthn |
| `github.com/pquerna/otp` | Apache-2.0 | TOTP المصادقة الثنائية |
| `github.com/jlaffaye/ftp` | ISC | تحميل النسخة الاحتياطية FTP |
| `gopkg.in/yaml.v3` | MIT / Apache-2.0 | تحليل التكوين YAML |

> ⚠ **مكونات GPL-3.0**: `sing` و`sing-shadowsocks` هما GPL-3.0 أو أحدث. ويتم ربطها بشكل ثابت باللوحة وثنائيات الوكيل، لذا **يتطلب توزيع الثنائيات توفير المصدر المقابل** (راجع القسم 3 من [LEGAL.md](./LEGAL.md)).

## 2. تبعيات الواجهة الأمامية (المترجمة في اللوحة الثنائية)

تم إنشاء واجهة المستخدم الإدارية باستخدام Vite وتم تضمينها في اللوحة الثنائية عبر `go:embed`، لذلك يتم توزيع المكونات التالية مع اللوحة الثنائية أيضًا:

| مكون | الترخيص | الغرض |
|---|---|---|
| Vue 3 / Vue Router / Pinia / Element Plus / axios / vue-i18n / marked / js-yaml / qrcode / Monaco Editor | MIT | واجهة المستخدم الإدارية، والتوجيه والحالة، وi18n، وعرض Markdown، ورموز QR، ومحرر الأكواد |
| DOMPurify | Apache-2.0 / MPL-2.0 | تعقيم XSS قبل عرض Markdown / HTML |

## 3. البرامج المستقلة التي يتم استدعاؤها في وقت التشغيل (غير موزعة في هذا المستودع)

يجوز للوحة / الوكيل تنزيل البرامج المستقلة التالية أو تثبيتها أو استدعائها في وقت التشغيل. إنهم يتفاعلون مع هذا المشروع من خلال العمليات/الواجهات، ولكن إذا قمت بشحنهم مع التسليمات الخاصة بك، فيجب عليك الالتزام بالتراخيص الخاصة بهم:

| البرنامج | الترخيص | ملاحظات |
|---|---|---|
| Xray-core | MPL-2.0 | الوكيل الأساسي، عملية منفصلة |
| sing-box | **GPL-3.0-or-later** | الوكيل الأساسي، عملية منفصلة؛ توفير المصدر المقابل عند التوزيع |
| Caddy | Apache-2.0 | الوكيل العكسي / TLS |
| Nginx | BSD-2-Clause | الوكيل العكسي (مثبت على النظام، إذا تم استخدامه) |

> Hysteria2 وTUIC هما **بروتوكولات يحملها sing-box**، وليست مراكز مستقلة أو ثنائيات، لذلك لم يتم إدراجهما بشكل منفصل.

## 4. ملفات البيانات

| البيانات | المصدر/ الترخيص |
|---|---|
| `geoip.dat` / `geosite.dat` | كما أعلن مستودع مصدر البيانات (عادةً GPL-3.0 أو CC-BY-SA) |
| GeoLite2 (`GeoLite2-ASN.mmdb`، وما إلى ذلك) | MaxMind GeoLite2 EULA / CC BY-SA 4.0؛ التحقق من الترخيص قبل الاستخدام التجاري |
| `country.mmdb` | كما أعلن مصدر بياناته |

## 5. توصيات الامتثال

- عند تسليم الثنائيات، قم بتضمين: `LICENSE`، وهذا الملف، ونص الترخيص أو الحصول على URL لكل مكون GPL/MPL.
- إذا قمت بتعديل الملفات المصدر لمكونات MPL-2.0 مثل `xray-core`، فقم بنشر مصدر الملفات المعدلة.
- إذا قمت بتوزيع برامج GPL مثل sing-box مع منتجك، فقم بتزويد المستلمين بالمصدر المطابق الكامل لذلك الإصدار (أو عرض مكتوب فعال).
- العلامات التجارية والأسماء: لا تستخدم أسماء أو شعارات المشاريع الأولية بطريقة تنطوي على تأييد رسمي.
