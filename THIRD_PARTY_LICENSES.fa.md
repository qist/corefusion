<!-- source: THIRD_PARTY_LICENSES.md | locale: fa | updated: 2026-10-23 -->
# قطعات و مجوزهای شخص ثالث

> زبان‌ها: [简体中文](THIRD_PARTY_LICENSES.md) · [English](THIRD_PARTY_LICENSES.en.md) · [繁體中文](THIRD_PARTY_LICENSES.zh-TW.md) · [Русский](THIRD_PARTY_LICENSES.ru.md) · [日本語](THIRD_PARTY_LICENSES.ja.md) · [한국어](THIRD_PARTY_LICENSES.ko.md) · [Deutsch](THIRD_PARTY_LICENSES.de.md) · [Français](THIRD_PARTY_LICENSES.fr.md) · [Español](THIRD_PARTY_LICENSES.es.md) · [Português](THIRD_PARTY_LICENSES.pt.md) · [Türkçe](THIRD_PARTY_LICENSES.tr.md) · [Bahasa Indonesia](THIRD_PARTY_LICENSES.id.md) · [ไทย](THIRD_PARTY_LICENSES.th.md) · [Tiếng Việt](THIRD_PARTY_LICENSES.vi.md) · [العربية](THIRD_PARTY_LICENSES.ar.md) · فارسی

CoreFusion از اجزای منبع باز زیر استفاده می کند. این فایل کامپوننت ها و لایسنس های اصلی را فهرست می کند. **لیست کامل توسط `go.mod` / `go.sum` و فایل LICENSE خود هر ماژول تعریف شده است**.

هنگام توزیع فایل های باینری این پروژه، این فایل، پروژه `LICENSE` و اعلامیه های حق چاپ/مجوز همه اجزای شخص ثالث را نگه دارید.

## 1. برو وابستگی ها به باینری کامپایل شده (اصلی)

| جزء | مجوز | هدف |
|---|---|---|
| `github.com/xtls/xray-core` | MPL-2.0 | تجزیه پیکربندی Xray، دستورات gRPC، تعاریف پروتکل |
| `github.com/xtls/reality` | MPL-2.0 | پشتیبانی از پروتکل REALITY |
| `github.com/sagernet/sing` | **GPL-3.0-یا جدیدتر** | به طور غیر مستقیم از طریق هسته xray پیوند داده شده است |
| `github.com/sagernet/sing-shadowsocks` | **GPL-3.0-یا جدیدتر** | پیوند غیر مستقیم از طریق هسته xray (Shadowsocks-2022) |
| `github.com/go-sql-driver/mysql` | MPL-2.0 | درایور MySQL |
| `github.com/hashicorp/golang-lru/v2` | MPL-2.0 | کش LRU |
| `github.com/go-acme/lego/v4` | MIT | صدور گواهی ACME |
| `github.com/gin-gonic/gin` | MIT | چارچوب HTTP |
| `gorm.io/gorm` | MIT | ORM |
| `github.com/aws/aws-sdk-go-v2` | آپاچی-2.0 | پشتیبان گیری S3، Route53 DNS-01 |
| `google.golang.org/grpc` | آپاچی-2.0 | ارتباط عامل gRPC |
| `google.golang.org/protobuf` | BSD-3-بند | زمان اجرا Protobuf |
| `golang.org/x/crypto` | BSD-3-بند | رمزنگاری های اولیه |
| `golang.org/x/oauth2` | BSD-3-بند | OAuth2 / Google EAB |
| `github.com/golang-jwt/jwt/v5` | MIT | JWT |
| `github.com/google/uuid` | BSD-3-بند | UUID |

> ⚠ ** اجزای GPL-3.0 **: `sing` و `sing-shadowsocks` GPL-3.0 یا بالاتر هستند. آنها به صورت ایستا به پنل و باینری های عامل مرتبط هستند، بنابراین **توزیع باینری ها مستلزم ارائه منبع مربوطه** است (به بخش 3 از [LEGAL.md](THIRD_PARTY_LICENSES.fa.md) مراجعه کنید).

## 2. برنامه های مستقل فراخوانی شده در زمان اجرا (در این مخزن توزیع نشده است)

پنل / عامل ممکن است برنامه های مستقل زیر را در زمان اجرا دانلود، نصب یا فراخوانی کند. آنها از طریق فرآیندها/رابط ها با این پروژه تعامل دارند، اما اگر آنها را با محصولات تحویلی خود ارسال می کنید، همچنان باید مجوزهای آنها را رعایت کنید:

| برنامه | مجوز | یادداشت ها |
|---|---|---|
| هسته Xray | MPL-2.0 | هسته پروکسی، فرآیند جداگانه |
| sing-box | **GPL-3.0-یا جدیدتر** | هسته پروکسی، فرآیند جداگانه. هنگام توزیع منبع مربوطه را ارائه دهید |
| کدی | آپاچی-2.0 | پروکسی معکوس / TLS |
| Nginx | BSD-2-Clause | پروکسی معکوس (در صورت استفاده در سیستم نصب شده است) |

> Hysteria2 و TUIC **پروتکل‌هایی هستند که توسط sing-box** حمل می‌شوند، نه هسته‌های مستقل یا باینری، بنابراین به طور جداگانه فهرست نشده‌اند.

## 3. فایل های داده

| داده ها | منبع / مجوز |
|---|---|
| `geoip.dat` / `geosite.dat` | همانطور که توسط مخزن منبع داده (معمولاً GPL-3.0 یا CC-BY-SA) اعلام شده است. |
| GeoLite2 (`GeoLite2-ASN.mmdb` و غیره) | MaxMind GeoLite2 EULA / CC BY-SA 4.0; تأیید مجوز قبل از استفاده تجاری |
| `country.mmdb` | همانطور که توسط منبع داده آن اعلام شده است |

## 4. توصیه های انطباق

- هنگام ارائه فایل های باینری، شامل: `LICENSE`، این فایل، و متن مجوز یا کسب URL هر جزء GPL/MPL باشد.
- اگر فایل های منبع مؤلفه های MPL-2.0 مانند `xray-core` را تغییر می دهید، منبع فایل های اصلاح شده را منتشر کنید.
- اگر برنامه های GPL مانند sing-box را با محصول خود توزیع می کنید، منبع متناظر کامل آن نسخه (یا یک پیشنهاد کتبی موثر) را در اختیار گیرندگان قرار دهید.
- علائم تجاری و نام‌ها: از نام‌ها یا آرم‌های پروژه بالادستی به گونه‌ای استفاده نکنید که به معنای تایید رسمی باشد.
