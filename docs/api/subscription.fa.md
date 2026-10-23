<!-- source: subscription.md | locale: fa | updated: 2026-10-23 -->
# اشتراک API (عمومی)

> زبان‌ها: [简体中文](subscription.md) · [English](subscription.en.md) · [繁體中文](subscription.zh-TW.md) · [Русский](subscription.ru.md) · [日本語](subscription.ja.md) · [한국어](subscription.ko.md) · [Deutsch](subscription.de.md) · [Français](subscription.fr.md) · [Español](subscription.es.md) · [Português](subscription.pt.md) · [Türkçe](subscription.tr.md) · [Bahasa Indonesia](subscription.id.md) · [ไทย](subscription.th.md) · [Tiếng Việt](subscription.vi.md) · [العربية](subscription.ar.md) · فارسی

لیست گره اشتراک کاربر را واکشی می کند. **این نقطه پایانی پر استفاده برای کاربران نهایی است**. هیچ هدری لازم نیست - دسترسی توسط توکن موجود در URL اعطا می شود.

## 1. نقطه پایانی

| استقرار | URL |
|---|---|
| پنل اصلی | `GET https://<panel-domain>/sub/<token>` |
| پورت اشتراک اختصاصی | `GET https://<sub-domain>:<port>/<token>` (بدون پیشوند `/sub`) |

`<token>` نشانه دسترسی منحصربه‌فرد یک اشتراک است که توسط طرف مدیریت ایجاد می‌شود (در قسمت «اشتراک‌های من» در پورتال قابل مشاهده است).

## 2. پارامترهای پرس و جو

| پارامتر | توضیحات |
|---|---|
| `format` | فرمت خروجی؛ پیش فرض اشتراک را لغو می کند. زیر را ببینید |
| `config` | نام الگوی پیکربندی (در قسمت «فرمت‌های اشتراک» در رابط کاربری سرپرست تعریف شده است). باید از همان قالب `format` این درخواست استفاده کند، در غیر این صورت نادیده گرفته می شود |
| `platform` | یک الگوی پیکربندی را با نام پلتفرم مطابقت دهید (زمانی که `config` حذف شده است استفاده می شود). قالب نیز باید مطابقت داشته باشد |

### پشتیبانی از مقادیر `format`

| ارزش | خروجی | مشتریان |
|---|---|---|
| `base64_uri` (پیش‌فرض) | پیوندهای اشتراک گذاری با خط جدید پیوسته، به طور کلی با پایه 64 کدگذاری شده اند | V2RayN، Shadowrocket، Surge و سایر مشتریان عمومی |
| `raw_uri` | پیوندهای اشتراک‌گذاری با خط جدید، متن ساده | مشتریانی که به پیوندهای ساده / اشکال زدایی نیاز دارند |
| `mihomo_yaml` | پیکربندی کامل Clash/Mihomo YAML (با پروکسی-گروه ها، قوانین) | Clash.Meta / mihomo / Clash Verge Rev |
| `mihomo_proxies_yaml` | YAML فقط با لیست گره های `proxies:` | منبع ارائه دهنده پروکسی mihomo (با به روز رسانی برنامه ریزی شده) |
| `singbox_json` | پیکربندی کامل sing-box JSON | sing-box / SFI / FLClash (هسته sing-box) |
| `xray_json` | پیکربندی کامل Xray JSON | هسته Xray |

> سازگاری پروتکل/فرمت اعمال می شود: برای مثال، `anytls` فقط در قالب های sing-box/Mihomo ظاهر می شود. گره های REALITY در سرور sing-box در اشتراک های Xray/sing-box JSON ظاهر نمی شوند. گره‌هایی که مشتری نمی‌تواند تشخیص دهد به‌جای شکست کل اشتراک، به‌طور خودکار حذف می‌شوند.

## 3. سرصفحه های پاسخ

| سربرگ | مثال | توضیحات |
|---|---|---|
| `Subscription-Userinfo` | `upload=1024; download=2048; total=1073741824; expire=1760000000` | استفاده و انقضا (بایت / یونیکس ثانیه). **همچنین به شماره 403** ارسال شد، بنابراین مشتریان می توانند کمک هزینه باقیمانده را نشان دهند |
| `Profile-Update-Interval` | `6` | فاصله پیشنهادی به‌روزرسانی (ساعت‌ها) |
| `Profile-Title` | `CoreFusion` | نام نمایه (نشان داده شده توسط Clash Verge Rev / V2RayN و غیره) |
| `Profile-Web-Page-Url` | `https://…/portal` | پیوند صفحه پورتال («صفحه وب باز» در مشتریان) |
| `Content-Type` | در قالب: `text/plain` / `text/yaml` / `application/json` |  |
| `Content-Disposition` | `attachment; filename="subscription.txt"` |  |

## 4. کدهای وضعیت

| HTTP | کد | معنی |
|---|---|---|
| 200 | — | عادی (بدن اشتراکی است که در `format` ارائه شده است) |
| 400 | `MISSING_TOKEN` | توکنی در URL وجود ندارد |
| 403 | (متن ساده) | اشتراک معلق / منقضی شده / غیرفعال است. پیام از طریق `Accept-Language` بومی سازی شده است (zh/en/ja/ko) و یک دلیل تک خطی است. |
| 404 | `SUBSCRIPTION_NOT_FOUND` | توکن وجود ندارد |

> اشتراک های لغو شده به عنوان ناموجود تلقی می شوند (404).

## 5. نمونه ها

```bash
# 1) Fetch with the default format
curl -L "https://panel.example.com/sub/abcd1234efgh5678"

# 2) Request Mihomo YAML with the "clash-global" config template
curl -L "https://panel.example.com/sub/abcd1234efgh5678?format=mihomo_yaml&config=clash-global"

# 3) Node list only (for a mihomo proxy-provider)
curl -L "https://panel.example.com/sub/abcd1234efgh5678?format=mihomo_proxies_yaml"

# 4) Check usage (headers only)
curl -sI "https://panel.example.com/sub/abcd1234efgh5678" | grep -i subscription-userinfo
```

اشتراک پویا Clash/Mihomo (بازسازی دوره ای):

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

## 6. یادداشت ها

- **قالب و پیکربندی کاملاً مرتبط هستند**: الگوی انتخاب شده توسط `config`/`platform` باید با `format` این درخواست مطابقت داشته باشد. الگوهای ناهماهنگ نادیده گرفته می شوند - این از تزریق قوانین Mihomo به sing-box/Xray JSON جلوگیری می کند.
- **محدود کردن نرخ**: نقطه پایانی اشتراک محدودیت خاص خود را دارد (`SubscribeRateLimit`). مشتریان بدسرپرست رد می شوند.
- **گزارش دسترسی**: هر واکشی اطلاعات IP/UA/CDN را ثبت می‌کند، که در زیر «گزارش دسترسی به اشتراک» در رابط کاربری سرپرست قابل مشاهده است.
- بدون یک دامنه پیکربندی شده بر روی یک پورت اشتراک اختصاصی، از `X-Forwarded-Proto` / `X-Forwarded-Host` درخواست استفاده می شود. مطمئن شوید که پراکسی معکوس شما هر دو هدر را به جلو می برد.
