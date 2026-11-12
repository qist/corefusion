<!-- source: subscription.md | locale: th | updated: 2026-11-12 -->
# การสมัครสมาชิก API (สาธารณะ)

> ภาษา: [简体中文](subscription.md) · [English](subscription.en.md) · [繁體中文](subscription.zh-TW.md) · [Русский](subscription.ru.md) · [日本語](subscription.ja.md) · [한국어](subscription.ko.md) · [Deutsch](subscription.de.md) · [Français](subscription.fr.md) · [Español](subscription.es.md) · [Português](subscription.pt.md) · [Türkçe](subscription.tr.md) · [Bahasa Indonesia](subscription.id.md) · ไทย · [Tiếng Việt](subscription.vi.md) · [العربية](subscription.ar.md) · [فارسی](subscription.fa.md)

ดึงรายการโหนดการสมัครสมาชิกของผู้ใช้ **นี่คือจุดสิ้นสุดที่ใช้มากที่สุดสำหรับผู้ใช้ปลายทาง**; ไม่จำเป็นต้องมีส่วนหัว — โทเค็นใน URL ให้สิทธิ์การเข้าถึง

## 1. จุดสิ้นสุด

| การปรับใช้ | URL |
|---|---|
| แผงหลัก | `GET https://<panel-domain>/sub/<token>` |
| พอร์ตการสมัครสมาชิกเฉพาะ | `GET https://<sub-domain>:<port>/<token>` (ไม่มีคำนำหน้า `/sub`) |

`<token>` เป็นโทเค็นการเข้าถึงเฉพาะของการสมัครสมาชิก สร้างขึ้นโดยฝ่ายผู้ดูแลระบบ (มองเห็นได้ภายใต้ "การสมัครสมาชิกของฉัน" ในพอร์ทัล)

## 2. พารามิเตอร์แบบสอบถาม

| พารามิเตอร์ | คำอธิบาย |
|---|---|
| `format` | รูปแบบเอาต์พุต; แทนที่ค่าเริ่มต้นการสมัครสมาชิก ดูด้านล่าง |
| `config` | ชื่อเทมเพลตการกำหนดค่า (กำหนดไว้ภายใต้ "รูปแบบการสมัครสมาชิก" ใน UI ผู้ดูแลระบบ) ต้องใช้รูปแบบเดียวกันกับ `format` ของคำขอนี้ ไม่เช่นนั้นจะถูกละเว้น |
| `platform` | จับคู่เทมเพลตการกำหนดค่าตามชื่อแพลตฟอร์ม (ใช้เมื่อละเว้น `config`) รูปแบบจะต้องตรงกันด้วย |
| `providers` | `mihomo_yaml` เท่านั้น: `1` / `true` เปิดใช้งาน **โหมดผู้ให้บริการ** (ปล่อย `proxy-providers` และอ้างอิงผ่าน `use`) ค่าเริ่มต้นหรือค่าอื่นใดจะทำให้โหนดอยู่ในบรรทัดภายใต้ `proxies` ดูมาตรา 6 |

### ค่า `format` ที่รองรับ

| ความคุ้มค่า | เอาท์พุต | ลูกค้า |
|---|---|---|
| `base64_uri` (ค่าเริ่มต้น) | ลิงก์แชร์ที่เข้าร่วมบรรทัดใหม่ เข้ารหัส base64 โดยรวม | V2RayN, Shadowrocket, Surge และไคลเอนต์ทั่วไปอื่นๆ |
| `raw_uri` | ลิงก์แชร์ที่ขึ้นบรรทัดใหม่ ข้อความธรรมดา | ไคลเอนต์ที่ต้องการลิงก์ธรรมดา / การแก้ไขจุดบกพร่อง |
| `mihomo_yaml` | การกำหนดค่า Clash/Mihomo YAML แบบเต็ม (พร้อมกลุ่มพร็อกซี กฎ) | Clash.Meta / mihomo / Clash Verge Rev |
| `mihomo_proxies_yaml` | YAML ที่มีเฉพาะรายการโหนด `proxies:` | แหล่งที่มาของผู้ให้บริการพร็อกซี mihomo (พร้อมการรีเฟรชตามกำหนดเวลา) |
| `singbox_json` | การกำหนดค่า sing-box JSON แบบเต็ม | sing-box / SFI / FLClash (แกน sing-box) |
| `xray_json` | การกำหนดค่า Xray JSON แบบเต็ม | Xray-คอร์ |

> ใช้ความเข้ากันได้ของโปรโตคอล/รูปแบบ: ตัวอย่างเช่น `anytls` จะปรากฏในรูปแบบ sing-box/Mihomo เท่านั้น **โหนด REALITY ที่ให้บริการโดย xray จะไม่รวมอยู่ในการสมัครสมาชิก sing-box** (sing-box ไม่รองรับการใช้งาน REALITY ของ xray ดูที่ sing-box#4520) แต่ยังคงปรากฏในการสมัครสมาชิก Xray ในขณะที่โหนด **REALITY ให้บริการโดย sing-box ใช้งานได้ทั้งการสมัครสมาชิก Xray และ sing-box** โหนดที่ไคลเอ็นต์ไม่รู้จักจะถูกทิ้งโดยอัตโนมัติแทนที่จะล้มเหลวในการสมัครสมาชิกทั้งหมด

## 3. ส่วนหัวของการตอบกลับ

| ส่วนหัว | ตัวอย่าง | คำอธิบาย |
|---|---|---|
| `Subscription-Userinfo` | `upload=1024; download=2048; total=1073741824; expire=1760000000` | การใช้งานและการหมดอายุ (ไบต์ / Unix วินาที) **ส่งวันที่ 403 ด้วย** เพื่อให้ลูกค้าสามารถแสดงเบี้ยคงเหลือได้ |
| `Profile-Update-Interval` | `6` | ช่วงเวลารีเฟรชที่แนะนำ (ชั่วโมง) |
| `Profile-Title` | `CoreFusion` | ชื่อโปรไฟล์ (แสดงโดย Clash Verge Rev / V2RayN ฯลฯ) |
| `Profile-Web-Page-Url` | `https://portal.example.com` | พอร์ทัล URL (“เปิดหน้าเว็บ” ในไคลเอนต์ พาธรูทเมื่อพอร์ทัลมีพอร์ตของตัวเอง มิฉะนั้น /p) |
| `Content-Type` | ต่อรูปแบบ: `text/plain` / `text/yaml` / `application/json` |  |
| `Content-Disposition` | `attachment; filename="subscription.txt"` |  |

## 4. รหัสสถานะ

| HTTP | รหัส | ความหมาย |
|---|---|---|
| 200 | — | ปกติ (เนื้อหาคือการสมัครสมาชิกที่แสดงผลใน `format`) |
| 400 | `MISSING_TOKEN` | ไม่มีโทเค็นใน URL |
| 403 | (ข้อความธรรมดา) | การสมัครสมาชิกถูกระงับ / หมดอายุ / ไม่ได้ใช้งาน ข้อความได้รับการแปลเป็นภาษาท้องถิ่น (zh/en/ja/ko) ผ่าน `Accept-Language` และเป็นเหตุผลบรรทัดเดียว |
| 404 | `SUBSCRIPTION_NOT_FOUND` | ไม่มีโทเค็นอยู่ |

> การสมัครสมาชิกที่ถูกเพิกถอนจะถือว่าไม่มีอยู่จริง (404)

## 5. ตัวอย่าง

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

การสมัครสมาชิกแบบไดนามิกของ Clash/Mihomo (รีเฟรชเป็นระยะ):

```yaml
proxy-providers:
  office:
    type: http
    url: "https://panel.example.com/sub/abcd1234efgh5678?format=mihomo_proxies_yaml&provider=office"
    interval: 3600
    health-check:
      enable: true
      url: https://www.gstatic.com/generate_204
      interval: 300

proxy-groups:
  - name: PROXY
    type: select
    use:
      - office
```

## 6. โหมดผู้ให้บริการและโดเมนการสมัครสมาชิกหลายรายการ

### 6.1 สิ่งที่ `providers=1` ปล่อยออกมา

ด้วย `format=mihomo_yaml&providers=1` (หรือ `format=clash&providers=1`) YAML:

- ไม่อินไลน์โหนดที่ระดับบนสุดอีกต่อไป - มันส่งเสียง `proxy-providers:`;
- กลุ่มพร็อกซีอ้างอิงแหล่งที่มาของโหนดผ่าน `use: [<provider-key>...]`;
- `proxy-providers` ใดๆ ที่เขียนในรูปแบบการสมัครสมาชิกที่ใช้ร่วมกัน (`mihomo_configs.config_json`) จะถูก **ละเว้น** (การกำหนดค่าที่ใช้ร่วมกันรองรับหลายแผน ผู้ให้บริการแบบฮาร์ดโค้ดอาจรั่วไหลข้ามแผน) การแมปกลุ่ม → แหล่งที่มาอยู่ภายใต้ “การจัดการแผน → กลุ่ม → การแมปแหล่งที่มา”

ประเภทผู้ให้บริการ:

| ชนิด | คีย์ | หมายเหตุ |
|---|---|---|
| ภายใน | ชื่อกลุ่มโหนดที่ผ่านการฆ่าเชื้อแล้ว | สร้างต่อกลุ่มโหนดแผน มีเพียงโหนดของกลุ่มนั้นเท่านั้น |
| สงวนไว้ (เลิกใช้) | `subscription-nodes` | ไม่ถูกส่งออกใน `proxy-providers` อีกต่อไป กลุ่มที่อ้างถึงจะถูกขยายเป็น provider จริง หรือฝังโหนดโดยตรงหากไม่มี provider |
| ภายนอก | ชื่อที่กำหนดค่าไว้ในแผน | แหล่งสัญญาณภายนอก (`http` YAML หรือ `file`) |

ช่องทั่วไปต่อผู้ให้บริการ:

- `url` — ชี้กลับไปที่การสมัครสมาชิกนี้ด้วย `format=mihomo_proxies_yaml` โดยสืบทอด `config` / `platform` จากคำขอหลัก ผู้ให้บริการภายในผนวก `provider=<key>`;
- `path` — **ได้มาจาก “ค่าเริ่มต้นของผู้ให้บริการภายใน → เทมเพลตเส้นทางแคช”** (แทนที่ `./providers/{key}.yaml` ที่เป็นค่าเริ่มต้น, `{key}` แทน) ผู้ให้บริการภายนอกไม่สามารถกำหนดเส้นทางของตนเองได้เช่นกัน
- `health-check` — `enable / url / interval`; ผู้ให้บริการภายในสืบทอดค่าเริ่มต้น ผู้ให้บริการภายนอกกำหนดค่าไว้ในแผน
- `interval` — ช่วงเวลารีเฟรชเป็นวินาที

### 6.2 โดเมนใดที่จะเข้าสู่ผู้ให้บริการ URL (หลายโดเมน)

“ศูนย์การสมัครใช้งาน → จุดสิ้นสุดการสมัครใช้งาน” อาจมีบันทึก **หลายรายการ** (โดเมน + เส้นทาง + เปิดใช้งาน) ผู้ให้บริการ `url` เลือกโดเมนตามลำดับนี้:

1. **โดเมนที่คำขอนี้ใช้จริง**: ค่าแรกของ `X-Forwarded-Host` (พอร์ตถูกถอดออก) มิฉะนั้น `Host`; ต้องตรงกับปลายทาง **ที่เปิดใช้งาน**
2. หากไม่มีสิ่งใดที่ตรงกัน ให้ถอยกลับไปที่ตำแหน่งข้อมูลที่เปิดใช้งานด้วย **ID ต่ำสุด**;
3. พาธมาจากจุดสิ้นสุดที่ตรงกัน (`path`, `/` ที่เป็นค่าเริ่มต้น) ซึ่งให้ผลลัพธ์เป็น `https://<domain><path>/<token>?format=mihomo_proxies_yaml...`

ผลที่ตามมา:

- ลูกค้าที่ดึงข้อมูลผ่านโดเมนใหม่จะได้รับโดเมนนั้นภายในผู้ให้บริการ — **ไม่จำเป็นต้องเปลี่ยนแผง**;
- โดเมนเก่าที่ยังคงเปิดใช้งานอยู่ในรายการจะไม่มีฮาร์ดโค้ดอีกต่อไป
- หากไคลเอนต์แคชผู้ให้บริการเก่า URL การอัปเดตการสมัครสมาชิก (หรือการล้างแคชของผู้ให้บริการ / รีสตาร์ทไคลเอนต์) จะสลับการสมัครสมาชิก

พร็อกซีย้อนกลับต้องส่งต่อ `X-Forwarded-Proto` และ `X-Forwarded-Host`; มิฉะนั้นจะใช้จุดสิ้นสุดทางเลือก เทมเพลตไซต์ Caddy / Nginx ที่สร้างขึ้นโดยแผงควบคุมมีทั้งส่วนหัวอยู่แล้ว (`header_up X-Forwarded-Host {host}` / `proxy_set_header X-Forwarded-Host $host;`); เพิ่มด้วยตนเองหากคุณรักษาการกำหนดค่าพร็อกซีด้วยตนเอง

## 7. หมายเหตุ

- **รูปแบบและการกำหนดค่าเชื่อมโยงกันอย่างแน่นหนา**: เทมเพลตที่เลือกโดย `config`/`platform` จะต้องตรงกับ `format` ของคำขอนี้ เทมเพลตที่ไม่ตรงกันจะถูกละเว้น - ซึ่งจะป้องกันการแทรกกฎ Mihomo ลงใน sing-box/Xray JSON
- **`providers=1` มีผลกับ Mihomo YAML เท่านั้น**: sing-box / Xray / base64 / raw ละเว้น (โหนดยังคงอยู่ในบรรทัด)
- **การจำกัดอัตรา**: จุดสิ้นสุดการสมัครสมาชิกมีขีดจำกัดของตัวเอง (`SubscribeRateLimit`) ลูกค้าที่ไม่เหมาะสมจะถูกปฏิเสธ
- **บันทึกการเข้าถึง**: ทุกการดึงข้อมูลจะบันทึกข้อมูล IP / UA / CDN ซึ่งมองเห็นได้ภายใต้ “บันทึกการเข้าถึงการสมัครสมาชิก” ใน UI ผู้ดูแลระบบ
- หากไม่มีโดเมนที่กำหนดค่าบนพอร์ตการสมัครสมาชิกเฉพาะ ระบบจะใช้ `X-Forwarded-Proto` / `X-Forwarded-Host` ของคำขอ ตรวจสอบให้แน่ใจว่าพร็อกซีย้อนกลับของคุณส่งต่อส่วนหัวทั้งสอง
- ในการปรับใช้หลายโดเมน ให้รายการตำแหน่งข้อมูลสอดคล้องกับโดเมนที่ไคลเอ็นต์ใช้จริง ดูมาตรา 6.2
