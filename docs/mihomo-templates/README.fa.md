# قالب‌های فرمت اشتراک mihomo

<sub>[简体中文](README.md) · [English](README.en.md) · [Русский](README.ru.md) · فارسی</sub>

سه قالب: **Windows / Android / OpenClash (روتر)**.

این‌ها همان YAML ذخیره‌شده در پنل زیر **فرمت اشتراک** هستند: بدون `proxies` و بدون
`proxy-providers`. گره‌ها هنگام تولید اشتراک توسط پنل و بر اساس طرح ساخته می‌شوند.

| فایل | کلاینت | روش گرفتن ترافیک |
| --- | --- | --- |
| [`mihomo-windows.yaml`](./mihomo-windows.yaml) | Clash Verge Rev / Mihomo Party / FlClash PC | `mixed-port` + پروکسی سیستم |
| [`mihomo-android.yaml`](./mihomo-android.yaml) | Clash Meta for Android / FlClash | VPN/TUN توسط برنامه |
| [`mihomo-openclash.yaml`](./mihomo-openclash.yaml) | OpenClash (OpenWrt) | redirect/tproxy، TUN اختیاری |

## ۱. تولید پویا

| محل | معنی |
| --- | --- |
| `_hasExternalNodes: true` | «گره‌های خارجی در این گروه قرار بگیرند»: در حالت عادی نام گره‌ها، در حالت provider مقدار `use` |
| `proxies:` سطح بالا | فقط خروجی‌های ویژه (مثل `dns-out`) |
| `proxy-providers:` سطح بالا | **اضافه نکنید**: پنل آن را نادیده می‌گیرد (یک YAML برای چندین طرح) |
| `rules:` | پس از ایمپورت، قواعد را پنل تولید می‌کند؛ نسخه داخل قالب فقط نمونه است |

```text
چسباندن قالب → ذخیره به‌عنوان فرمت اشتراک → کلاینت /sub/<token> را می‌گیرد
    پیش‌فرض      : گره‌ها داخل گروه‌های _hasExternalNodes قرار می‌گیرند
    ?providers=1 : proxy-providers تولید و به گروه‌ها `use` اضافه می‌شود
```

## ۲. ایمپورت در پنل

1. پنل → **فرمت اشتراک** (پیکربندی mihomo) → ایجاد/ویرایش;
2. YAML را بچسبانید → ذخیره;
3. به طرح متصل کنید و منبع را در **طرح → منابع اشتراک** تنظیم کنید.

نشانی‌های کلاینت:

```text
https://<panel>/sub/<token>
https://<panel>/sub/<token>?format=mihomo_yaml&config=<format>&providers=1
```

## ۳. پورت 22 (SSH) فقط از گره‌های انتخابی

```yaml
- DST-PORT,22,🖥️ SSH,no-resolve

- name: 🖥️ SSH
  type: select
  proxies:
    - DIRECT
```

به‌طور پیش‌فرض گروه فقط `DIRECT` دارد و هیچ گره‌ای خودکار اضافه نمی‌شود.
برای اجازه دادن به یک گروه گره مشخص:

```text
پنل → طرح → منابع اشتراک → گروه → منابع
  گروه: 🖥️ SSH
  منبع: <گروه گره، مثلاً hysteria>
```

نتیجه: `22 → 🖥️ SSH → use: [hysteria]` (فقط همان گره‌ها) و سایر گروه‌ها تغییر نمی‌کنند.

> این نگاشت در طرح ذخیره می‌شود (نه در قالب) و بر اساس **نام دقیق گروه** تطبیق داده می‌شود.
> نام قبلی `ACCESS-DENIED` بود؛ پس از تغییر به `🖥️ SSH` نگاشت را هم به‌روز کنید،
> وگرنه پورت 22 فقط از `DIRECT` عبور می‌کند.

## ۴. قواعد و DNS

- `rule-providers` به `https://rules.example.com/clash/Ruleset/...` اشاره می‌کند —
  آن را با کتابخانه قواعد خودتان عوض کنید (منابع عمومی در نسخه چینی README آمده است).
- geodata از انتشار عمومی MetaCubeX استفاده می‌کند:

```yaml
geox-url:
  geoip: "https://fastly.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/geoip.dat"
  geosite: "https://fastly.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/geosite.dat"
  mmdb: "https://fastly.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/geoip.metadb"
```

- DNS فقط DoH عمومی: Alibaba/Tencent برای چین، Google/Cloudflare برای بقیه.

## ۵. تفاوت پلتفرم‌ها

| مورد | Windows | Android | OpenClash |
| --- | --- | --- | --- |
| `allow-lan` | false | false | **true** |
| `ipv6` | false | true | true |
| `find-process-mode` | strict | strict | **"off"** (با کوتیشن، وگرنه YAML آن را `false` می‌کند) |
| `tun` | ندارد (پروکسی سیستم) | ندارد (برنامه خودش) | **فعال**، `stack: system` |
| `external-controller` | `127.0.0.1:9090` | `127.0.0.1:9090` | `0.0.0.0:9090` |

## ۶. بررسی

```bash
curl -s "https://<panel>/sub/<token>?format=mihomo_yaml" -o test.yaml
mihomo -t -d <tmpdir> -f test.yaml
# configuration file ... test is successful
```

## ۷. پیوندهای بیرونی

- [پوشه مخزن](https://github.com/qist/corefusion/tree/main/docs/mihomo-templates)
- فایل‌های خام:
  - [mihomo-windows.yaml](https://raw.githubusercontent.com/qist/corefusion/main/docs/mihomo-templates/mihomo-windows.yaml)
  - [mihomo-android.yaml](https://raw.githubusercontent.com/qist/corefusion/main/docs/mihomo-templates/mihomo-android.yaml)
  - [mihomo-openclash.yaml](https://raw.githubusercontent.com/qist/corefusion/main/docs/mihomo-templates/mihomo-openclash.yaml)
- منابع عمومی قواعد:
  - Loyalsoldier/clash-rules: <https://github.com/Loyalsoldier/clash-rules>
  - ispip (IP چین): <https://ispip.clang.cn/>
  - MetaCubeX meta-rules-dat: <https://github.com/MetaCubeX/meta-rules-dat>

- مخزن پروژه: <https://github.com/qist/corefusion>
- مخزن قواعد (قواعد clash همراه این قالب‌ها): <https://github.com/qist/xray>
