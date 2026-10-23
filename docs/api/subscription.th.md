<!-- source: subscription.md | locale: th | updated: 2026-10-23 -->
# การสมัครสมาชิก API (สาธารณะ)

> ภาษา: [简体中文](subscription.md) · [English](subscription.en.md) · [繁體中文](subscription.zh-TW.md) · [Русский](subscription.ru.md) · [日本語](subscription.ja.md) · [한국어](subscription.ko.md) · [Deutsch](subscription.de.md) · [Français](subscription.fr.md) · [Español](subscription.es.md) · [Português](subscription.pt.md) · [Türkçe](subscription.tr.md) · [Bahasa Indonesia](subscription.id.md) · ไทย · [Tiếng Việt](subscription.vi.md) · [العربية](subscription.ar.md) · [فارسی](subscription.fa.md)

ดึงรายการโหนดการสมัครสมาชิกของผู้ใช้ **นี่คือจุดสิ้นสุดที่ใช้มากที่สุดสำหรับผู้ใช้ปลายทาง**; ไม่จำเป็นต้องมีส่วนหัว — โทเค็นให้สิทธิ์การเข้าถึงใน URL

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

### ค่า `format` ที่รองรับ

| ความคุ้มค่า | เอาท์พุต | ลูกค้า |
|---|---|---|
| `base64_uri` (ค่าเริ่มต้น) | ลิงก์แชร์ที่เข้าร่วมบรรทัดใหม่ เข้ารหัส base64 โดยรวม | V2RayN, Shadowrocket, Surge และไคลเอนต์ทั่วไปอื่นๆ |
| `raw_uri` | ลิงก์แชร์ที่ขึ้นบรรทัดใหม่ ข้อความธรรมดา | ไคลเอนต์ที่ต้องการลิงก์ธรรมดา / การแก้ไขจุดบกพร่อง |
| `mihomo_yaml` | การกำหนดค่า Clash/Mihomo YAML แบบเต็ม (พร้อมกลุ่มพร็อกซี กฎ) | Clash.Meta / mihomo / Clash Verge Rev |
| `mihomo_proxies_yaml` | YAML ที่มีเฉพาะรายการโหนด `proxies:` | แหล่งที่มาของผู้ให้บริการพร็อกซี mihomo (พร้อมการรีเฟรชตามกำหนดเวลา) |
| `singbox_json` | การกำหนดค่า sing-box JSON แบบเต็ม | sing-box / SFI / FLClash (แกน sing-box) |
| `xray_json` | การกำหนดค่า Xray JSON แบบเต็ม | Xray-คอร์ |

> ใช้ความเข้ากันได้ของโปรโตคอล/รูปแบบ: ตัวอย่างเช่น `anytls` จะปรากฏในรูปแบบ sing-box/Mihomo เท่านั้น โหนด REALITY บนเซิร์ฟเวอร์ sing-box ไม่ปรากฏในการสมัครใช้งาน Xray/sing-box JSON โหนดที่ไคลเอ็นต์ไม่รู้จักจะถูกทิ้งโดยอัตโนมัติแทนที่จะทำให้การสมัครรับข้อมูลทั้งหมดล้มเหลว

## 3. ส่วนหัวของการตอบกลับ

| ส่วนหัว | ตัวอย่าง | คำอธิบาย |
|---|---|---|
| `Subscription-Userinfo` | `upload=1024; download=2048; total=1073741824; expire=1760000000` | การใช้งานและการหมดอายุ (ไบต์ / Unix วินาที) **ส่งวันที่ 403 ด้วย** เพื่อให้ลูกค้าสามารถแสดงเบี้ยคงเหลือได้ |
| `Profile-Update-Interval` | `6` | ช่วงเวลารีเฟรชที่แนะนำ (ชั่วโมง) |
| `Profile-Title` | `CoreFusion` | ชื่อโปรไฟล์ (แสดงโดย Clash Verge Rev / V2RayN ฯลฯ) |
| `Profile-Web-Page-Url` | `https://…/portal` | ลิงก์หน้าพอร์ทัล (“เปิดหน้าเว็บ” ในไคลเอนต์) |
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
```

การสมัครสมาชิกแบบไดนามิกของ Clash/Mihomo (รีเฟรชเป็นระยะ):

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

## 6. หมายเหตุ

- **รูปแบบและการกำหนดค่าเชื่อมโยงกันอย่างแน่นหนา**: เทมเพลตที่เลือกโดย `config`/`platform` จะต้องตรงกับ `format` ของคำขอนี้ เทมเพลตที่ไม่ตรงกันจะถูกละเว้น - ซึ่งจะป้องกันการแทรกกฎ Mihomo ลงใน sing-box/Xray JSON
- **การจำกัดอัตรา**: จุดสิ้นสุดการสมัครสมาชิกมีขีดจำกัดของตัวเอง (`SubscribeRateLimit`) ลูกค้าที่ไม่เหมาะสมจะถูกปฏิเสธ
- **บันทึกการเข้าถึง**: ทุกการดึงข้อมูลจะบันทึกข้อมูล IP / UA / CDN ซึ่งมองเห็นได้ภายใต้ “บันทึกการเข้าถึงการสมัครสมาชิก” ใน UI ผู้ดูแลระบบ
- หากไม่มีโดเมนที่กำหนดค่าบนพอร์ตการสมัครสมาชิกเฉพาะ ระบบจะใช้ `X-Forwarded-Proto` / `X-Forwarded-Host` ของคำขอ ตรวจสอบให้แน่ใจว่าพร็อกซีย้อนกลับของคุณส่งต่อส่วนหัวทั้งสอง
