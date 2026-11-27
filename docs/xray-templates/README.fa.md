# قالب فرمت اشتراک Xray

<sub>[简体中文](README.md) · [English](README.en.md) · [Русский](README.ru.md) · فارسی</sub>

[`config.json`](./config.json) همان `config_json` فرمت اشتراک **xray_json** است و فقط
بخش‌های ثابت سمت کلاینت را دارد:

```text
log / dns / inbounds / routing
```

`outbounds` را پنل می‌سازد و `direct` / `block` و یک **balancer با `tag: proxy`**
اضافه می‌کند (Xray مفهوم selector ندارد؛ balancer تصادفی نقش گروه پروکسی را دارد).
قواعدی که `"outboundTag": "proxy"` دارند به `"balancerTag": "proxy"` تبدیل می‌شوند.
**اشتراک همیشه `proxy` دارد** و ویرایش دستی لازم نیست. نام گره‌های هم‌نام با
`proxy` / `direct` / `block` خودکار تغییر می‌کند.

## ۱. بارگذاری

```text
پنل → فرمت اشتراک → ایجاد
  client_type: v2ray
  format:      xray_json
  محتوا:       چسباندن config.json
```

```text
https://<panel>/sub/<token>?format=xray_json&config=<format>
```

## ۲. مشترکان و استفاده مستقل

- **مشترک فقط به نشانی اشتراک نیاز دارد** — بدون حساب پنل و بدون ویرایش دستی.
- **بدون پنل** از [`standalone.json`](./standalone.json) استفاده کنید: پیکربندی کامل با
  پاس‌شدن `xray run -test`. مقدار `node-example` در `outbounds`
  (نمونه `127.0.0.1:1080` socks) را با گره خود عوض کنید.

## ۳. ساختار

| بخش | کار |
| --- | --- |
| `inbounds` | SOCKS روی `127.0.0.1:10808` + HTTP روی `127.0.0.1:10809` |
| `dns` | دامنه‌های چین → Alibaba DoH، جهانی → Google DoH، بقیه → `1.1.1.1` |
| `routing.rules` | تبلیغات → `block`؛ STUN (3478/19302) → `block`؛ چین → `direct`؛ بقیه → `proxy` |
| `routing.domainStrategy` | `IPIfNonMatch` |

## ۴. Geodata

قواعد از تگ‌های داخلی `geosite:` / `geoip:` استفاده می‌کنند:

```text
geosite:category-ads-all
geosite:cn
geosite:geolocation-!cn
geoip:cn
```

کلاینت به `geosite.dat` / `geoip.dat` نیاز دارد (پنل آن‌ها را در `data/rules/` همگام می‌کند).
برای داده سفارشی، آن‌ها را در مسیر `XRAY_LOCATION_ASSET` بگذارید.

## ۵. بررسی

```bash
curl -s "https://<panel>/sub/<token>?format=xray_json&config=<format>" -o xr.json
XRAY_LOCATION_ASSET=/opt/corefusion/data/rules xray run -test -c xr.json
# Configuration OK.
```

## ۶. هشدارهای شناخته‌شده

WebSocket با `headers.Host` و Trojan بدون flow در Xray 26.x هشدار منسوخ‌شدن می‌دهند.
این مربوط به پروتکل گره است و بارگذاری را متوقف نمی‌کند.

## ۷. پیوندهای بیرونی

- [پوشه مخزن](https://github.com/qist/corefusion/tree/main/docs/xray-templates)
- فایل‌های خام:
  - [config.json](https://raw.githubusercontent.com/qist/corefusion/main/docs/xray-templates/config.json)
  - [standalone.json](https://raw.githubusercontent.com/qist/corefusion/main/docs/xray-templates/standalone.json)
- مستندات:
  - Xray-core: <https://xtls.github.io/>
  - geosite / geoip: <https://github.com/v2fly/domain-list-community>

- مخزن پروژه: <https://github.com/qist/corefusion>
- مخزن قواعد (قواعد clash همراه این قالب‌ها): <https://github.com/qist/xray>
