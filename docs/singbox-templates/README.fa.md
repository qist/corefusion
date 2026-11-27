# قالب فرمت اشتراک sing-box

<sub>[简体中文](README.md) · [English](README.en.md) · [Русский](README.ru.md) · فارسی</sub>

[`config.json`](./config.json) همان `config_json` فرمت اشتراک **sing-box** در پنل است.
فقط بخش‌های ثابت سمت کلاینت را دارد:

```text
log / dns / inbounds / route / experimental
```

`outbounds` توسط پنل از گره‌های اشتراک ساخته می‌شود و همیشه یک **selector با
`tag: proxy`** اضافه می‌گردد. یعنی اشتراک همیشه `proxy` دارد و قواعد بدون ویرایش دستی
به آن اشاره می‌کنند. اگر نام گرهی `proxy` / `direct` / `block` باشد، به `proxy-2`
و … تغییر می‌کند تا tag تکراری نشود.

## ۱. بارگذاری

```text
پنل → فرمت اشتراک → ایجاد
  client_type: singbox
  format:      singbox_json
  محتوا:       چسباندن config.json
```

نشانی اشتراک:

```text
https://<panel>/sub/<token>?format=singbox_json&config=<format>
```

> قبلاً `inbounds` ادغام نمی‌شد (فقط log/dns/route/experimental)؛ اکنون ادغام می‌شود،
> وگرنه کلاینت پیکربندی بدون پورت شنود دریافت می‌کرد.

## ۲. مشترکان و استفاده مستقل

- **مشترک فقط به نشانی اشتراک نیاز دارد** — بدون حساب پنل و بدون ویرایش دستی.
- **بدون پنل** از [`standalone.json`](./standalone.json) استفاده کنید: پیکربندی کامل که
  `sing-box check` را پاس می‌کند. مقدار `node-example` در `outbounds`
  (نمونه `127.0.0.1:1080` socks5) را با گره خود عوض کنید.

## ۳. ساختار

| بخش | کار |
| --- | --- |
| `inbounds` | `mixed` روی `127.0.0.1:7890`؛ TUN را خودتان اضافه کنید |
| `dns` | قالب جدید DNS (sing-box 1.12+): DoH چین → `direct`، جهانی → `proxy` |
| `route.rule_set` | فایل‌های `.srs` از MetaCubeX: `cn`، `category-ads-all`، `geolocation-!cn`، `geoip/cn` |
| `route.rules` | تبلیغات → reject؛ STUN (3478/19302) → reject؛ چین → direct؛ جهانی → proxy |
| `route.final` | `proxy` (selector تزریق‌شده) |

## ۴. DNS (قالب جدید)

```json
{ "type": "https", "tag": "dns-cn", "server": "223.5.5.5", "path": "/dns-query", "detour": "direct" }
```

قالب قدیمی `{"address": "...", "detour": "..."}` در sing-box 1.12 حذف شده است.

## ۵. TUN (اختیاری)

```json
{
  "type": "tun",
  "tag": "tun-in",
  "address": ["172.19.0.1/30"],
  "auto_route": true,
  "strict_route": false,
  "stack": "system"
}
```

`stack` در sing-box 1.15 منسوخ شده ولی هنوز کار می‌کند (فقط WARN).

## ۶. مجموعه قواعد

```text
https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/sing/geo/geosite/<name>.srs
https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/sing/geo/geoip/<name>.srs
```

فایل‌های `.txt`/`.yaml` کلش مستقیماً پشتیبانی نمی‌شوند؛ ابتدا به `.srs` تبدیل کنید
(`sing-box rule-set convert`).

## ۷. بررسی

```bash
curl -s "https://<panel>/sub/<token>?format=singbox_json&config=<format>" -o sb.json
sing-box check -c sb.json
# exit 0 = OK
```

## ۸. پیوندهای بیرونی

- [پوشه مخزن](https://github.com/qist/corefusion/tree/main/docs/singbox-templates)
- فایل‌های خام:
  - [config.json](https://raw.githubusercontent.com/qist/corefusion/main/docs/singbox-templates/config.json)
  - [standalone.json](https://raw.githubusercontent.com/qist/corefusion/main/docs/singbox-templates/standalone.json)
- مستندات:
  - sing-box: <https://sing-box.sagernet.org/>
  - مجموعه قواعد: <https://github.com/MetaCubeX/meta-rules-dat>

- مخزن پروژه: <https://github.com/qist/corefusion>
- مخزن قواعد (قواعد clash همراه این قالب‌ها): <https://github.com/qist/xray>
