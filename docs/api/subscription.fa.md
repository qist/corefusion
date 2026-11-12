<!-- source: subscription.md | locale: fa | updated: 2026-11-12 -->
# اشتراک API (عمومی)

> زبان‌ها: [简体中文](subscription.md) · [English](subscription.en.md) · [繁體中文](subscription.zh-TW.md) · [Русский](subscription.ru.md) · [日本語](subscription.ja.md) · [한국어](subscription.ko.md) · [Deutsch](subscription.de.md) · [Français](subscription.fr.md) · [Español](subscription.es.md) · [Português](subscription.pt.md) · [Türkçe](subscription.tr.md) · [Bahasa Indonesia](subscription.id.md) · [ไทย](subscription.th.md) · [Tiếng Việt](subscription.vi.md) · [العربية](subscription.ar.md) · فارسی

لیست گره اشتراک کاربر را واکشی می کند. **این نقطه پایانی پر استفاده برای کاربران نهایی است**. نیازی به هدر نیست - دسترسی توسط توکن موجود در URL اعطا می شود.

## 1. نقطه پایانی

| استقرار | URL |
|---|---|
| پنل اصلی | `GET https://<panel-domain>/sub/<token>` |
| پورت اشتراک اختصاصی | `GET https://<sub-domain>:<port>/<token>` (بدون پیشوند `/sub`) |

`<token>` نشانه دسترسی منحصربه‌فرد یک اشتراک است که توسط سمت سرپرست ایجاد می‌شود (در قسمت «اشتراک‌های من» در پورتال قابل مشاهده است).

## 2. پارامترهای پرس و جو

| پارامتر | توضیحات |
|---|---|
| `format` | فرمت خروجی؛ پیش فرض اشتراک را لغو می کند. زیر را ببینید |
| `config` | نام الگوی پیکربندی (تعریف شده در «فرمت‌های اشتراک» در رابط کاربری سرپرست). باید از همان قالب `format` این درخواست استفاده کند، در غیر این صورت نادیده گرفته می شود |
| `platform` | یک الگوی پیکربندی را با نام پلتفرم مطابقت دهید (زمانی که `config` حذف می شود استفاده می شود). قالب نیز باید مطابقت داشته باشد |
| `providers` | فقط `mihomo_yaml`: `1` / `true` **حالت ارائه دهنده** را فعال می کند (`proxy-providers` را منتشر کرده و آنها را از طریق `use` ارجاع دهید). پیش‌فرض یا هر مقدار دیگری، گره‌ها را در زیر `proxies` قرار می‌دهد. §6 را ببینید |

### پشتیبانی از مقادیر `format`

| ارزش | خروجی | مشتریان |
|---|---|---|
| `base64_uri` (پیش‌فرض) | پیوندهای اشتراک گذاری با خط جدید پیوسته، به طور کلی با پایه 64 کدگذاری شده اند | V2RayN، Shadowrocket، Surge و سایر مشتریان عمومی |
| `raw_uri` | پیوندهای اشتراک‌گذاری با خط جدید، متن ساده | مشتریانی که به پیوندهای ساده / اشکال زدایی نیاز دارند |
| `mihomo_yaml` | پیکربندی کامل Clash/Mihomo YAML (با پروکسی-گروه ها، قوانین) | Clash.Meta / mihomo / Clash Verge Rev |
| `mihomo_proxies_yaml` | YAML فقط با لیست گره های `proxies:` | منبع ارائه دهنده پروکسی mihomo (با بازخوانی برنامه ریزی شده) |
| `singbox_json` | پیکربندی کامل sing-box JSON | sing-box / SFI / FLClash (هسته sing-box) |
| `xray_json` | پیکربندی کامل Xray JSON | هسته Xray |

> سازگاری پروتکل/فرمت اعمال می شود: برای مثال، `anytls` فقط در قالب های sing-box/Mihomo ظاهر می شود. **گره‌های REALITY که توسط xray ارائه می‌شوند از اشتراک‌های sing-box مستثنی هستند** (sing-box از اجرای REALITY xray پشتیبانی نمی‌کند، sing-box#4520QZ را ببینید، در حالی که زیر اسکریپت‌ها0، sing-box هنوز ظاهر می‌شود) **گره‌های REALITY که توسط sing-box ارائه می‌شوند در هر دو اشتراک Xray و sing-box کار می‌کنند**. گره‌هایی که مشتری نمی‌تواند تشخیص دهد به‌جای شکست کل اشتراک، به‌طور خودکار حذف می‌شوند.

## 3. سرصفحه های پاسخ

| سربرگ | مثال | توضیحات |
|---|---|---|
| `Subscription-Userinfo` | `upload=1024; download=2048; total=1073741824; expire=1760000000` | استفاده و انقضا (بایت / یونیکس ثانیه). **همچنین به شماره 403** ارسال شد، بنابراین مشتریان می توانند کمک هزینه باقیمانده را نشان دهند |
| `Profile-Update-Interval` | `6` | فاصله پیشنهادی به‌روزرسانی (ساعت‌ها) |
| `Profile-Title` | `CoreFusion` | نام نمایه (نشان داده شده توسط Clash Verge Rev / V2RayN و غیره) |
| `Profile-Web-Page-Url` | `https://portal.example.com` | پورتال URL ("صفحه وب باز" در کلاینت ها؛ مسیر ریشه زمانی که پورتال پورت خود را دارد، در غیر این صورت /p) |
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

# 5) Clash dynamic subscription: config template + Provider mode
curl -L "https://sub.example.com/abcd1234efgh5678?format=clash&config=out&providers=1"

# 6) sing-box / Xray with their own config templates (rules come from the template)
curl -L "https://sub.example.com/abcd1234efgh5678?format=singbox_json&config=sing-box-home"
curl -L "https://sub.example.com/abcd1234efgh5678?format=xray_json&config=xray-home"
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

## 6. حالت ارائه دهنده و دامنه های اشتراک چندگانه

### 6.1 آنچه `providers=1` منتشر می کند

با `format=mihomo_yaml&providers=1` (یا `format=clash&providers=1`) YAML:

- دیگر گره ها را در سطح بالا خط نمی دهد - `proxy-providers:` را منتشر می کند.
- گروه های پروکسی به منابع گره مرجع از طریق `use: [<provider-key>...]`.
- هر `proxy-providers` نوشته شده در قالب اشتراک مشترک (`mihomo_configs.config_json`) **نادیده گرفته می‌شود** (پیکربندی مشترک چندین طرح را ارائه می‌دهد؛ ارائه‌دهندگان سخت‌کد در سراسر برنامه‌ها به بیرون درز می‌کنند). گروه → نگاشت منبع در زیر "مدیریت طرح → گروه → نگاشت منبع" زندگی می کنند.

انواع ارائه دهنده:

| مهربان | کلید | یادداشت ها |
|---|---|---|
| داخلی | نام گروه گره پاکسازی شده | تولید در هر گروه گره طرح. فقط شامل گره های آن گروه است |
| رزرو شده است | `subscription-nodes` | تمام گره های طرح؛ منبع آن URL **پارامتر `provider` را حذف می کند** |
| خارجی | نام در طرح پیکربندی شده است | منبع خارجی (`http` YAML یا `file`) |

فیلدهای رایج برای هر ارائه دهنده:

- `url` - به این اشتراک با `format=mihomo_proxies_yaml` اشاره می کند و `config` / `platform` را از درخواست اصلی به ارث می برد. ارائه دهندگان داخلی `provider=<key>` را اضافه می کنند.
- `path` — **همیشه از «پیش‌فرض‌های ارائه‌دهنده داخلی → الگوی مسیر حافظه پنهان»** مشتق شده است (پیش‌فرض `./providers/{key}.yaml`، `{key}` جایگزین شده است). ارائه دهندگان خارجی نیز نمی توانند مسیر خود را تعیین کنند.
- `health-check` — `enable / url / interval`; ارائه دهندگان داخلی پیش فرض ها را به ارث می برند، ارائه دهندگان خارجی آن را بر روی طرح پیکربندی می کنند.
- `interval` - فاصله بازخوانی در ثانیه.

### 6.2 کدام دامنه به ارائه دهنده URL (چند دامنه) می رود

"مرکز اشتراک → نقاط پایانی اشتراک" ممکن است **چند رکورد** (دامنه + مسیر + فعال) را در خود نگه دارد. ارائه دهنده `url` دامنه را به این ترتیب انتخاب می کند:

1. **دامنه‌ای که این درخواست واقعاً استفاده می‌کند**: اولین مقدار `X-Forwarded-Host` (پورت حذف شده)، در غیر این صورت `Host`. باید با نقطه پایانی **فعال** مطابقت داشته باشد.
2. اگر چیزی مطابقت نداشت، به نقطه پایانی فعال شده با **پایین ترین شناسه** برگردید.
3. مسیر از نقطه پایانی همسان (`path`، پیش‌فرض `/`) می‌آید که `https://<domain><path>/<token>?format=mihomo_proxies_yaml...` را به دست می‌دهد.

عواقب:

- مشتری که از طریق یک دامنه جدید واکشی می‌کند، آن دامنه را در داخل ارائه‌دهنده می‌گیرد — **بدون نیاز به تغییر پانل**.
- دامنه قدیمی که هنوز در لیست فعال است، دیگر کدگذاری سختی ندارد.
- اگر یک سرویس گیرنده ارائه دهنده قدیمی URL را در حافظه پنهان ذخیره کند، به روز رسانی اشتراک (یا پاک کردن حافظه پنهان ارائه دهنده / راه اندازی مجدد مشتری) آن را تغییر می دهد.

پراکسی های معکوس باید `X-Forwarded-Proto` و `X-Forwarded-Host` را ارسال کنند. در غیر این صورت از نقطه پایانی بازگشتی استفاده می شود. الگوهای سایت Caddy / Nginx ایجاد شده پانل در حال حاضر شامل هر دو سرصفحه (`header_up X-Forwarded-Host {host}` / `proxy_set_header X-Forwarded-Host $host;`) است. اگر پیکربندی پروکسی را به صورت دستی حفظ می کنید، آنها را به صورت دستی اضافه کنید.

## 7. یادداشت ها

- **قالب و پیکربندی کاملاً مرتبط هستند**: الگوی انتخاب شده توسط `config`/`platform` باید با `format` این درخواست مطابقت داشته باشد. الگوهای ناهماهنگ نادیده گرفته می شوند - این از تزریق قوانین Mihomo به sing-box/Xray JSON جلوگیری می کند.
- **`providers=1` فقط روی Mihomo YAML تأثیر می‌گذارد**: sing-box / Xray / base64 / خام آن را نادیده بگیرید (گره‌ها به صورت خطی می‌مانند).
- **محدود کردن نرخ**: نقطه پایانی اشتراک محدودیت خاص خود را دارد (`SubscribeRateLimit`). مشتریان بدسرپرست رد می شوند.
- **گزارش دسترسی**: هر واکشی اطلاعات IP/UA/CDN را ثبت می‌کند، که در زیر «گزارش دسترسی به اشتراک» در رابط کاربری سرپرست قابل مشاهده است.
- بدون یک دامنه پیکربندی شده بر روی یک پورت اشتراک اختصاصی، از `X-Forwarded-Proto` / `X-Forwarded-Host` درخواست استفاده می شود. مطمئن شوید که پراکسی معکوس شما هر دو هدر را به جلو می برد.
- در استقرار چند دامنه‌ای، فهرست نقاط پایانی را با دامنه‌هایی که کلاینت‌ها واقعاً استفاده می‌کنند، در یک راستا نگه دارید. §6.2 را ببینید.
