<!-- source: THIRD_PARTY_LICENSES.md | locale: fa | updated: 2026-11-19 -->
# قطعات و مجوزهای شخص ثالث

> زبان‌ها: [简体中文](THIRD_PARTY_LICENSES.md) · [English](THIRD_PARTY_LICENSES.en.md) · [繁體中文](THIRD_PARTY_LICENSES.zh-TW.md) · [Русский](THIRD_PARTY_LICENSES.ru.md) · [日本語](THIRD_PARTY_LICENSES.ja.md) · [한국어](THIRD_PARTY_LICENSES.ko.md) · [Deutsch](THIRD_PARTY_LICENSES.de.md) · [Français](THIRD_PARTY_LICENSES.fr.md) · [Español](THIRD_PARTY_LICENSES.es.md) · [Português](THIRD_PARTY_LICENSES.pt.md) · [Türkçe](THIRD_PARTY_LICENSES.tr.md) · [Bahasa Indonesia](THIRD_PARTY_LICENSES.id.md) · [ไทย](THIRD_PARTY_LICENSES.th.md) · [Tiếng Việt](THIRD_PARTY_LICENSES.vi.md) · [العربية](THIRD_PARTY_LICENSES.ar.md) · فارسی

CoreFusion از اجزای منبع باز زیر استفاده می کند. این فایل کامپوننت ها و لایسنس های اصلی را فهرست می کند. **لیست کامل توسط `go.mod` / `go.sum`، `web/admin/package.json` و فایل LICENSE خود هر ماژول تعریف شده است**.

هنگام توزیع فایل‌های باینری این پروژه، این فایل، پروژه `LICENSE` و اعلامیه‌های حق چاپ/مجوز همه اجزای شخص ثالث را نگه دارید.

## 1. برو وابستگی ها به باینری کامپایل شده (اصلی)

| جزء | مجوز | هدف |
|---|---|---|
| `github.com/xtls/xray-core` | MPL-2.0 | تجزیه پیکربندی Xray، دستورات gRPC، تعاریف پروتکل |
| `github.com/xtls/reality` | MPL-2.0 | پشتیبانی از پروتکل REALITY |
| `github.com/sagernet/sing` | **GPL-3.0-or-later** | به طور غیر مستقیم از طریق هسته xray پیوند داده شده است |
| `github.com/sagernet/sing-shadowsocks` | **GPL-3.0-or-later** | پیوند غیر مستقیم از طریق هسته xray (Shadowsocks-2022) |
| `github.com/go-sql-driver/mysql` | MPL-2.0 | درایور MySQL |
| `github.com/hashicorp/golang-lru/v2` | MPL-2.0 | کش LRU |
| `github.com/go-acme/lego/v5` | MIT | صدور گواهی ACME |
| `github.com/gin-gonic/gin` | MIT | چارچوب HTTP |
| `gorm.io/gorm` | MIT | ORM |
| `gorm.io/driver/mysql` | MIT | درایور MySQL برای GORM |
| `gorm.io/driver/postgres` | MIT | درایور PostgreSQL برای GORM |
| `github.com/jackc/pgx/v5` | MIT | درایور PostgreSQL |
| `github.com/qist/sqlite` | MIT | درایور SQLite (پایگاه داده پیش فرض برای پنل / عامل) |
| `github.com/aws/aws-sdk-go-v2` | Apache-2.0 | پشتیبان گیری S3، Route53 DNS-01 |
| `google.golang.org/grpc` | Apache-2.0 | ارتباط عامل gRPC |
| `google.golang.org/protobuf` | BSD-3-Clause | زمان اجرا Protobuf |
| `golang.org/x/crypto` | BSD-3-Clause | اصول رمزنگاری اولیه |
| `golang.org/x/oauth2` | BSD-3-Clause | OAuth2 / Google EAB |
| `golang.org/x/sys` | BSD-3-Clause | تماس های سیستمی (محدود کردن نرخ nft/tc، مدیریت فرآیند) |
| `github.com/golang-jwt/jwt/v5` | MIT | JWT |
| `github.com/google/uuid` | BSD-3-Clause | UUID |
| `github.com/go-webauthn/webauthn` | BSD-3-Clause | ورود به سیستم Passkey / WebAuthn |
| `github.com/pquerna/otp` | Apache-2.0 | احراز هویت دو مرحله ای TOTP |
| `github.com/jlaffaye/ftp` | ISC | آپلود پشتیبان FTP |
| `gopkg.in/yaml.v3` | MIT / Apache-2.0 | تجزیه پیکربندی YAML |

> ⚠ ** اجزای GPL-3.0**: `sing` و `sing-shadowsocks` GPL-3.0 یا بالاتر هستند. آنها به صورت ایستا به پنل و باینری های عامل مرتبط هستند، بنابراین **توزیع باینری ها مستلزم ارائه منبع مربوطه** است (به بخش 3 از [LEGAL.md](./LEGAL.md) مراجعه کنید).

## 2. وابستگی های Frontend (کامپایل شده در پانل باینری)

رابط کاربری مدیریت با Vite ساخته شده و از طریق `go:embed` در پنل باینری تعبیه شده است، بنابراین اجزای زیر نیز با باینری توزیع می شوند:

| جزء | مجوز | هدف |
|---|---|---|
| Vue 3 / Vue Router / Pinia / Element Plus / axios / vue-i18n / marked / js-yaml / qrcode / Monaco Editor | MIT | رابط کاربری مدیریت، مسیریابی و وضعیت، i18n، رندر Markdown، کدهای QR و ویرایشگر کد |
| DOMPurify | Apache-2.0 / MPL-2.0 | پاکسازی XSS قبل از رندر Markdown / HTML |

## 3. برنامه های مستقلی که در زمان اجرا توسط پنل بارگیری / فراخوانی شده اند

هنگام نصب یا ارتقای هسته‌ها، پنل / عامل باینری‌های زیر را از GitHub دانلود می‌کند و آنها را به عنوان فرآیندهای جداگانه اجرا می‌کند. **مصنوعات با علامت "ساخته شده توسط این پروژه" توسط این سازمان از منبع بالادستی** (`qist/*-binaries`) گردآوری و منتشر می شوند و همراه با محصول توزیع می شوند، بنابراین تعهدات مجوز مربوطه (از جمله در دسترس بودن منبع برای GPL) بر عهده این پروژه است. بقیه URL های دانلود نسخه های رسمی بالادستی هستند.

| برنامه | مجوز | یادداشت ها |
|---|---|---|
| Xray-core | MPL-2.0 | هسته پروکسی، فرآیند جداگانه. انتشار رسمی بالادستی (`XTLS/Xray-core`) |
| sing-box | **GPL-3.0-or-later** | هسته پروکسی، فرآیند جداگانه. **ساخته شده توسط این پروژه** (`qist/singbox-binaries`؛ نسخه های رسمی فاقد `with_v2ray_api` هستند که برای حسابداری ترافیک هر کاربر مورد نیاز است). هنگام توزیع منبع مربوطه را ارائه دهید |
| Caddy | Apache-2.0 | پروکسی معکوس / TLS; **ساخته شده توسط این پروژه** (`qist/caddy-binaries`) |
| Nginx | BSD-2-Clause | پروکسی معکوس؛ **ساخته شده توسط این پروژه** (`qist/nginx-binaries`، تقسیم بر glibc / musl) |

> Hysteria2 و TUIC **پروتکل‌هایی هستند که توسط sing-box** حمل می‌شوند، نه هسته‌های مستقل یا باینری، بنابراین به طور جداگانه فهرست نشده‌اند.

## 4. فایل های داده

| داده ها | منبع / مجوز |
|---|---|
| `geoip.dat` / `geosite.dat` | همانطور که توسط مخزن منبع داده (معمولاً GPL-3.0 یا CC-BY-SA) اعلام شده است. |
| GeoLite2 (`GeoLite2-ASN.mmdb` و غیره) | MaxMind GeoLite2 EULA / CC BY-SA 4.0; تأیید مجوز قبل از استفاده تجاری |
| `country.mmdb` | همانطور که توسط منبع داده آن اعلام شده است |

## 5. توصیه های انطباق

- هنگام ارائه فایل های باینری، شامل: `LICENSE`، این فایل، و متن مجوز یا کسب URL هر جزء GPL/MPL باشد.
- باینری های ساخته شده توسط این پروژه (sing-box / Caddy / Nginx) با پنل ارسال می شوند: از آنجایی که آنها از منبع بالادستی کامپایل شده اند، نحوه ساخت آنها را منتشر کنید (نسخه، تگ های ساخت، پیکربندی پرچم ها). اجزای GPL (sing-box) علاوه بر این به منبع متناظر کامل آن نسخه یا یک مسیر اکتساب مؤثر نیاز دارند.
- اگر فایل های منبع مؤلفه های MPL-2.0 مانند `xray-core` را تغییر می دهید، منبع فایل های اصلاح شده را منتشر کنید.
- اگر برنامه های GPL مانند sing-box را با محصول خود توزیع می کنید، منبع متناظر کامل آن نسخه (یا یک پیشنهاد کتبی موثر) را در اختیار گیرندگان قرار دهید.
- علائم تجاری و نام‌ها: از نام‌ها یا آرم‌های پروژه بالادستی به گونه‌ای استفاده نکنید که به معنای تایید رسمی باشد.
