<!-- source: portal-api.md | locale: th | updated: 2026-10-23 -->
# การอ้างอิงพอร์ทัล API (ไคลเอนต์ / พอร์ทัล API)

> ภาษา: [简体中文](portal-api.md) · [English](portal-api.en.md) · [繁體中文](portal-api.zh-TW.md) · [Русский](portal-api.ru.md) · [日本語](portal-api.ja.md) · [한국어](portal-api.ko.md) · [Deutsch](portal-api.de.md) · [Français](portal-api.fr.md) · [Español](portal-api.es.md) · [Português](portal-api.pt.md) · [Türkçe](portal-api.tr.md) · [Bahasa Indonesia](portal-api.id.md) · ไทย · [Tiếng Việt](portal-api.vi.md) · [العربية](portal-api.ar.md) · [فارسی](portal-api.fa.md)

> การอ้างอิงอินเทอร์เฟซ **สมบูรณ์** สำหรับส่วนหน้าของพอร์ทัล (`/p` หรือพอร์ตพอร์ทัลเฉพาะ) และไคลเอนต์บุคคลที่สาม
> คำจำกัดความของเส้นทาง: `internal/api/router/router.go` (`clientAuthGroup` / `clientAuthProtected` / `/api/v1/public/*`)

## 1. อนุสัญญาทั่วไป

### 1.1 ฐาน URL

| การปรับใช้ | ฐาน URL |
|---|---|
| เครื่องยนต์แผงหลัก | `https://<panel-domain>/api/v1` (หน้าพอร์ทัลที่ `/p`) |
| พอร์ตพอร์ทัลเฉพาะ | `https://<portal-domain>[:port]/api/v1` (หน้าพอร์ทัลที่ `/`) |
| โดเมนพอร์ทัลเฉพาะ + พร็อกซีย้อนกลับ | เหมือนข้างบน; พร็อกซีต้องส่งต่อทั้งเส้นทางเพจและ `/api/v1` |

### 1.2 การรับรองความถูกต้อง

- หลังจากลงชื่อเข้าใช้แล้ว ให้ส่ง `Authorization: Bearer <access_token>`;
- โทเค็นการเข้าถึงมีอายุ 24 ชั่วโมง รีเฟรชโทเค็น 7 วัน
- JWT มี `tv` (token_version): ออกจากระบบ / บังคับให้ออกจากระบบเพิ่มขึ้น และทำให้โทเค็นเก่าทั้งหมดเป็นโมฆะ
 (`clients.token_version` ดู `internal/service/clientauth/service.go`);
- จุดสิ้นสุดที่ได้รับการป้องกันทุกจุดจะผ่าน `ClientAuthMiddleware` (การตรวจสอบเวอร์ชัน + การป้องกันโหมดการบำรุงรักษา)

### 1.3 ซองตอบกลับ

ตำแหน่งข้อมูลส่วนใหญ่ส่งคืน `{ "code": 0, "data": ..., "message": "..." }`;
จุดสิ้นสุดของรายการบางรายการส่งคืน `{ "items": [...], "total": N }` หรืออาร์เรย์เปล่า (ระบุไว้ต่อจุดสิ้นสุดด้านล่าง)
ข้อผิดพลาดส่งคืน `{ "code": "ERROR_CODE", "message": "...", "request_id": "...", "can_retry": ... }`
(HTTP 4xx/5xx)

### 1.4 จำนวนหน่วย

ช่องคำสั่งซื้อ / ยอดคงเหลือคือ **จำนวนเต็มเซนต์**: `990` = ¥9.90 (`currency: "CNY"`)

### 1.5 ตัวอย่างจริง (ลงชื่อเข้าใช้ → ดึงโปรไฟล์)

```bash
# Register (when registration_enabled=true)
curl -X POST https://<portal-domain>/api/v1/client-auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","password":"plaintext-password","referral_code":""}'
# → 201 {"data":{"access_token":"<JWT>","refresh_token":"<JWT>","client":{...}}}

# Sign in
curl -X POST https://<portal-domain>/api/v1/client-auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","password":"plaintext-password"}'
# → 200 same as above

# Fetch the current client profile (with token)
curl https://<portal-domain>/api/v1/client-auth/me \
  -H 'Authorization: Bearer <access_token>'
# → 200 {"data":{"id":27,"email":"user@example.com","nickname":"","status":"active","quota_bytes":10737418240,"used_bytes":0,...}}

# Fetch a subscription (no header, token in the path)
curl 'https://<portal-domain>/sub/<subscription_token>?format=mihomo_yaml'
# → 200 Mihomo YAML (or base64_uri / singbox_json / xray_json)

# Error example: expired token
# → 401 {"code":"errClientTokenRevoked","message":"...","request_id":"...","can_retry":false}
```

บน 401 ให้โทรไปที่ `POST /api/v1/client-auth/refresh` หนึ่งครั้งแล้วลองอีกครั้ง หากยังคงล้มเหลว ให้ล้างเซสชันภายในเครื่อง (เทมเพลตพอร์ทัลดำเนินการนี้อยู่แล้ว)

---

## 2. ปลายทางสาธารณะ (ไม่มีการลงชื่อเข้าใช้)

| วิธีการ | เส้นทาง | คำอธิบาย |
|---|---|---|
| GET | `/p` (`/` บนพอร์ทัลเอ็นจิ้นเฉพาะ) | พอร์ทัล HTML (แสดงผลโดยเซิร์ฟเวอร์ + รันไทม์ Vue) |
| GET | `/api/v1/public/portal-config` | การกำหนดค่ารันไทม์ของพอร์ทัล: ชื่อไซต์ สีของธีม การนำทาง โมดูล ชุดภาษา `registration_enabled`, `regions` (แหล่งที่มาของภูมิภาคที่ลงชื่อเข้าใช้), `privacy_policy_url` / `terms_of_service_url` / `privacy_policy_content` / `terms_of_service_content` (เอกสารการปฏิบัติตามข้อกำหนด เนื้อหาที่แมปตามภาษา หน้าสแตนด์อโลน `/p/legal/*` ✓) |
| GET | `/api/v1/public/settings` | การตั้งค่าสาธารณะ (ปัจจุบันคือ `registration_enabled`) |
| GET | `/api/v1/public/subscription-endpoints` | รายการปลายทางการสมัครสมาชิก (อาร์เรย์เปล่า) |
| GET | `/api/v1/public/articles` | บทความสาธารณะ (เผยแพร่เท่านั้น) |
| GET | `/api/v1/public/articles/:slug` | รายละเอียดบทความ |
| GET | `/api/v1/public/client-configs` | การกำหนดค่าการดาวน์โหลดไคลเอ็นต์ |
| GET | `/sub/:token` | การจัดส่งการสมัครสมาชิก (`format` ส่งคืน base64_uri / raw_uri / mihomo_yaml / singbox_json / xray_json ฯลฯ ; เพิกถอน/หายไป → 404, ถูกระงับ/หมดอายุ/ไม่ได้ใช้งาน → 403 ดู [subscription.md](portal-api.th.md)) |
| GET | `/api/v1/client-auth/oauth/providers` | การมองเห็นผู้ให้บริการ OAuth: `{"data":{"providers":[{"provider":"github","enabled":true},...]}}` |
| GET | `/api/v1/system/maintenance` | สถานะการบำรุงรักษา (สาธารณะ) พอร์ทัลแสดงหน้าการบำรุงรักษาเมื่อเปิดใช้งาน |

> ทั้งหมดนี้รวมถึงเจ้าหน้าที่บำรุงรักษาและการจำกัดอัตราสาธารณะ API

## 3. การรับรองความถูกต้อง/OAuth

| วิธีการ | เส้นทาง | การรับรองความถูกต้อง | คำอธิบาย |
|---|---|---|---|
| POST | `/api/v1/client-auth/register` | — | ลงทะเบียน ร่างกาย: `{email, password, referral_code?}`. ส่งกลับ `{access_token, refresh_token, client}`; อัตราการลงทะเบียนจำกัด |
| POST | `/api/v1/client-auth/login` | — | ลงชื่อเข้าใช้ เนื้อหา: `{email, password}` คำตอบเดียวกัน ออก JWT กับ `token_version` |
| POST | `/api/v1/client-auth/refresh` | — | รีเฟรช ร่างกาย: `{refresh_token}`. ส่งคืนโทเค็นการเข้าถึง/รีเฟรชใหม่ (ตรวจสอบเวอร์ชันแล้ว โทเค็นการรีเฟรชเก่าจะตายทันทีหลังจากออกจากระบบ) |
| POST | `/api/v1/client-auth/logout` | ✓ | ออกจากระบบ: การเพิกถอนฝั่งเซิร์ฟเวอร์ (`token_version +1`) → โทเค็นเก่าทั้งหมดสำหรับไคลเอนต์ไม่ถูกต้อง |
| POST | `/api/v1/client-auth/rotate-token` | ✓ | หมุนเวียนโทเค็นการสมัครสมาชิก |
| POST | `/api/v1/client-auth/ticket` | ✓ | ออก **ตั๋วระยะสั้นแบบครั้งเดียว** (สำหรับโฟลว์ `window.open` เช่น “ชุดบันทึกการดาวน์โหลด” ซึ่งไม่สามารถตั้งค่าส่วนหัวได้) |
| GET | `/api/v1/client-auth/oauth/:provider` | — | เริ่ม OAuth: 307 → ความยินยอม GitHub/Google (`state` anti-CSRF, 10 นาที) ผู้ให้บริการที่พิการส่งคืน 400 |
| GET | `/api/v1/client-auth/oauth/:provider/callback` | — | การโทรกลับ OAuth: แลกเปลี่ยนรหัส → สร้าง / เชื่อมโยงไคลเอนต์ → ออกรุ่น JWT → 302 กลับไปที่พอร์ทัล (`/<portalPath>?access_token=...&refresh_token=...`) |

> การเรียกกลับ OAuth คือ **การเปลี่ยนเส้นทางเบราว์เซอร์** (ไม่ใช่การเรียก API): หลังจากการลงชื่อเข้าใช้สำเร็จ พอร์ทัลจะอ่านโทเค็น
> จาก URL ใน `onMounted` (หรือผ่าน `postMessage` ที่มีต้นกำเนิดเดียวกัน โปรดดู `handleOAuthCallback` ในเทมเพลตพอร์ทัล `web`)

## 4. บัญชี

| วิธีการ | เส้นทาง | คำอธิบาย |
|---|---|---|
| GET | `/api/v1/client-auth/me` | ไคลเอ็นต์ปัจจุบัน: `{id, email, plan_id, plan_name, status, quota_bytes, used_bytes, upload_bytes, download_bytes, expires_at, ip_whitelist, device_limit, sub_token, sub_status, created_at, ...}` (ซอง `data`) |
| POST | `/api/v1/client-auth/change-password` | ร่างกาย: `{old_password, new_password}` |
| GET | `/api/v1/client-auth/checktime` | เวลาเซิร์ฟเวอร์ (ซิงค์นาฬิกาไคลเอนต์) |
| GET | `/api/v1/client-auth/connection-info` | พารามิเตอร์การเชื่อมต่อแผนปัจจุบัน: `{concurrent, device_limit, protocol, speed_limit, tls, transport}` |

## 5. การสมัครรับข้อมูล

| วิธีการ | เส้นทาง | คำอธิบาย |
|---|---|---|
| POST | `/api/v1/client-auth/reset-subscription` | รีเซ็ตการสมัครสมาชิก; สิ่งนี้จะหมุน `sub_token` |

> ลิงก์การสมัครสมาชิกสร้างขึ้นที่ส่วนหน้าจาก `sub_token` ที่ส่งคืนโดย `/me` (`/sub/:token`) ไม่มี
> แยกปลายทาง `subscription-link` / `subscription-formats` อีกต่อไป

## 6. โหนด

| วิธีการ | เส้นทาง | คำอธิบาย |
|---|---|---|
| GET | `/api/v1/client-auth/nodes` | โหนดที่มองเห็นได้ในแผนปัจจุบัน: `{items:[...]}` (ฟิลด์ที่แสดง เช่น ชื่อ/โปรโตคอล/การขนส่ง/ภูมิภาค) |
| GET | `/api/v1/client-auth/nodes/:id/config` | การกำหนดค่าโหนดแบบเต็ม |
| GET | `/api/v1/client-auth/nodes/:id/link` | แชร์ลิงก์ (vless:// ฯลฯ) |
| GET | `/api/v1/client-auth/nodes/:id/qrcode` | รูปภาพรหัส QR |

## 7. ร้านค้า/แผน/คำสั่งซื้อ

| วิธีการ | เส้นทาง | คำอธิบาย |
|---|---|---|
| GET | `/api/v1/client-auth/plans` | รายการแผน (อาร์เรย์เปล่า): `{id, name, description, category, sort_order, variants:[{id, name, billing_cycle, duration_days, price(cents), original_price, currency, quota_bytes, ...}]}` |
| GET | `/api/v1/client-auth/plans/:id` | รายละเอียดแผน |
| GET | `/api/v1/client-auth/traffic-packs` | รายการชุดการจราจร |
| POST | `/api/v1/client-auth/traffic-packs/buy` | ซื้อชุดจราจร |
| GET | `/api/v1/client-auth/orders` | คำสั่งแผนของฉัน (แบ่งหน้า) |
| POST | `/api/v1/client-auth/orders` | สร้างคำสั่งซื้อ (ซื้อ/ต่ออายุแผนแบบต่างๆ) |
| GET | `/api/v1/client-auth/orders/:id` | รายละเอียดการสั่งซื้อ |
| GET | `/api/v1/client-auth/invoices` | ใบแจ้งหนี้ |
| GET | `/api/v1/client-auth/invoices/:id` | รายละเอียดใบแจ้งหนี้ |

## 8. ยอดคงเหลือ / เติมเงิน

| วิธีการ | เส้นทาง | คำอธิบาย |
|---|---|---|
| GET | `/api/v1/client-auth/balance` | `{data:{client_id, balance(cents), frozen}}` |
| GET | `/api/v1/client-auth/balance/transactions` | ธุรกรรมยอดคงเหลือ (`type`: เรียกเก็บเงิน/ใช้/คืนเงิน; มีการแบ่งหน้า) |
| GET | `/api/v1/client-auth/charge/sites` | ไซต์เติมเงินที่มีอยู่ (`ListChargeSites`: เปิดใช้งานเท่านั้น) |
| POST | `/api/v1/client-auth/charge/create` | สร้างคำสั่งเติมเงิน (เปลี่ยนเส้นทางไปยังการชำระเงินของบุคคลที่สาม) |
| GET | `/api/v1/client-auth/charge/orders` | รายการสั่งซื้อเติมเงิน |
| GET | `/api/v1/client-auth/charge/orders/:id` | รายละเอียดการสั่งซื้อเติมเงิน |
| GET | `/api/v1/client-auth/charge/status/:trade_no` | สถานะการชำระเงินแบบสำรวจความคิดเห็นโดย trade_no |
| POST | `/api/v1/client-auth/charge/redeem-card` | แลกบัตรเติมเงิน |

> การลบไคลเอ็นต์ **เรียงซ้อน** ไปยังยอดคงเหลือและธุรกรรม (ดู `DeleteClient` ใน `internal/service/client/service.go`)

## 9. ของขวัญ/ประวัติการเข้าชม

| วิธีการ | เส้นทาง | คำอธิบาย |
|---|---|---|
| GET | `/api/v1/client-auth/gifts` | ของขวัญที่สามารถรับสิทธิ์ได้ (อาร์เรย์เปลือย `type`: วัน/ไบต์) |
| POST | `/api/v1/client-auth/gifts/:id/claim` | การเรียกร้อง |
| GET | `/api/v1/client-auth/traffic-history` | ประวัติการเข้าชมรายเดือน: `{data:{history:[{period_start,total_bytes,...}], months}}` |

## 10. ตั๋ว

| วิธีการ | เส้นทาง | คำอธิบาย |
|---|---|---|
| GET | `/api/v1/client-auth/tickets` | รายการตั๋ว: `{items,total}` |
| POST | `/api/v1/client-auth/tickets` | สร้างตั๋ว |
| GET | `/api/v1/client-auth/tickets/:id` | รายละเอียดตั๋ว (พร้อมคำตอบ) |
| POST | `/api/v1/client-auth/tickets/:id/reply` | ตอบกลับ |
| POST | `/api/v1/client-auth/tickets/:id/close` | ปิด |

## 11. บทความ/คำแนะนำ

| วิธีการ | เส้นทาง | คำอธิบาย |
|---|---|---|
| GET | `/api/v1/client-auth/articles` | รายการบทความ (พร้อมตัวกรองหมวดหมู่) |
| GET | `/api/v1/client-auth/articles/:slug` | รายละเอียดบทความ |

## 12. การอ้างอิง (ปิดใช้งานโดยค่าเริ่มต้น สามารถเปิดใช้งานได้ในโปรแกรมแก้ไขพอร์ทัล)

| วิธีการ | เส้นทาง | คำอธิบาย |
|---|---|---|
| GET | `/api/v1/client-auth/referral/code` | รหัสอ้างอิงและลิงค์ของฉัน |
| GET | `/api/v1/client-auth/referral/stats` | สถิติค่าคอมมิชชั่น (`commission_rate`, `available_commission`, `min_withdraw_amount`) |
| GET | `/api/v1/client-auth/referral/records` | บันทึกการอ้างอิง |
| GET | `/api/v1/client-auth/referral/withdrawals` | บันทึกการถอนเงิน |
| POST | `/api/v1/client-auth/referral/withdraw` | ขอถอนเงิน |

## 13. บัญชีที่ใช้ร่วมกัน

| วิธีการ | เส้นทาง | คำอธิบาย |
|---|---|---|
| GET | `/api/v1/client-auth/shared-accounts` | บัญชีที่ใช้ร่วมกันที่มีอยู่ (สตรีมมิ่ง ฯลฯ ) |
| GET | `/api/v1/client-auth/shared-accounts/:id` | รายละเอียด |
| POST | `/api/v1/client-auth/shared-accounts/:id/apply` | ร้องขอการเข้าถึง |
| POST | `/api/v1/client-auth/shared-accounts/:id/repair` | ซ่อมแซมด้วยคลิกเดียว |

## 14. บันเดิลบันทึก (การวินิจฉัยสมาชิก)

| วิธีการ | เส้นทาง | คำอธิบาย |
|---|---|---|
| GET | `/api/v1/client-auth/logs/types` | ประเภทบันทึกที่สามารถรวมกลุ่มได้ |
| GET | `/api/v1/client-auth/logs/packages` | ชุดบันทึกของฉัน |
| POST | `/api/v1/client-auth/logs/package` | สร้างงานการรวมบันทึก (โหนดรายการเท่านั้น) |
| GET | `/api/v1/client-auth/logs/packages/:id` | สถานะของงาน (พร้อม/กำลังดำเนินการ/ล้มเหลว) |
| GET | `/api/v1/client-auth/logs/packages/:id/download` | ดาวน์โหลด (ต้องการ `POST /ticket` และผนวก `?ticket=`) |
| DELETE | `/api/v1/client-auth/logs/packages/:id` | ลบบันเดิลบันทึก |

## 15. การตั้งค่าความปลอดภัย / รายการที่อนุญาต IP / ยกเลิกการแบนตัวเอง

| วิธีการ | เส้นทาง | คำอธิบาย |
|---|---|---|
| GET/PUT | `/api/v1/client-auth/settings` | การตั้งค่าไคลเอ็นต์: `{ip_whitelist, device_limit}` (PUT) |
| GET | `/api/v1/client-auth/settings/whitelist` | รายการที่อนุญาต (อาร์เรย์เปลือย) |
| DELETE | `/api/v1/client-auth/settings/whitelist/:id` | ลบรายการอนุญาต |
| POST | `/api/v1/client-auth/settings/auto-add-ip` | เพิ่ม IP ปัจจุบันลงในรายการที่อนุญาต |
| POST | `/api/v1/client-auth/self-unban` | ยกเลิกการแบนแบบบริการตนเอง (ความพยายามถูกจำกัดโดยนโยบาย) |

> รายการที่อนุญาต IP **ใช้กับ URL การสมัครสมาชิกเท่านั้น** (`/sub/:token`) มันไม่ได้บล็อกการลงชื่อเข้าใช้พอร์ทัล/OAuth หรือ
> จุดสิ้นสุดการตรวจสอบสิทธิ์ไคลเอ็นต์อื่นๆ หากผู้ใช้ไม่มีรายการที่อนุญาตหรือไม่มีการสมัครสมาชิก URL การเข้าถึงจะได้รับอนุญาตเสมอ ที่
> รายการที่อนุญาตคือการตั้งค่าต่อผู้ใช้ ไม่มีสวิตช์ส่วนกลาง

**ขั้นตอนการยกเลิกการแบนด้วยตนเอง**: ใช้ได้เฉพาะกับ `auto_quota` (การแบนอัตโนมัติสำหรับปริมาณการเข้าชมที่มากเกินไป) เมื่อฝั่งผู้ดูแลระบบ
`self_unban.enabled` เปิดอยู่ การลงชื่อเข้าใช้ (รหัสผ่านหรือการโทรกลับ OAuth) ส่งคืน `require_self_unban=true` + `self_unban_token`
(typ=self_unban ใช้ได้ 15 นาที **เท่านั้น** ใช้งานได้บน `POST /self-unban` จุดปลายอื่นส่งคืน 403/`SELF_UNBAN_TOKEN_SCOPE`)
พอร์ทัลจะแสดงการ์ดปลดแบน หลังจากยกเลิกการแบนสำเร็จ ผู้ใช้จะลงชื่อเข้าใช้อีกครั้ง การเรียกกลับ OAuth จะส่งสิ่งนี้ผ่านทาง
แฟรกเมนต์ URL `#require_self_unban=1&self_unban_token=..&self_unban_hint=..`

## 16. รหัสข้อผิดพลาด (ข้อความที่ตัดตอนมา)

| รหัส | สถานการณ์ |
|---|---|
| `INVALID_TOKEN` | โทเค็นการรีเฟรชไม่ถูกต้อง/หมดอายุ/เวอร์ชันไม่ตรงกัน |
| `INVALID_REQUEST` | การตรวจสอบพารามิเตอร์ล้มเหลว |
| `NOT_FOUND` | ทรัพยากรขาดหายไป (รายการตั๋ว/คำสั่งซื้อ/รายการที่อนุญาต ฯลฯ) |
| `RELOAD_FAILED` / `SYNC_FAILED` | การซิงค์การกำหนดค่าพร็อกซี (ฝั่งผู้ดูแลระบบ) ล้มเหลว |
| 401 + `errClientTokenRevoked` | token_version ไม่ตรงกัน (ออกจากระบบ/ถูกเตะ) → ไคลเอ็นต์ควรรีเฟรช หากล้มเหลว ให้ล้างโทเค็นในเครื่อง |

## 17. เอกสารการปฏิบัติตามข้อกำหนด (นโยบายความเป็นส่วนตัว / เงื่อนไขการให้บริการ)

ลิงก์ “นโยบายความเป็นส่วนตัว · ข้อกำหนดในการให้บริการ” ที่ด้านล่างของการ์ดลงชื่อเข้าใช้ (เช่นเดียวกับไซต์ทั่วไป การคลิกจะเปิดเนื้อหา):

- **การกำหนดค่า**: ตัวแก้ไขพอร์ทัล → การตั้งค่า → ทั่วไป → เอกสารการปฏิบัติตามข้อกำหนด (**แก้ไขเนื้อหาตามภาษา**);
 คุณยังสามารถ `PUT /api/v1/portal/active` ด้วยฟิลด์ที่มีโครงสร้าง `privacy_policy_url` / `privacy_policy_content` /
 `terms_of_service_url` / `terms_of_service_content` (ไม่มี = ไม่เปลี่ยนแปลง เนื้อหาเป็นแมป `{"zh-CN":"...","en":"..."}`)
- **หน้าที่เข้าถึงได้แบบสแตนด์อโลน** (แสดงผลโดยเซิร์ฟเวอร์ ง่ายสำหรับโปรแกรมรวบรวมข้อมูลการตรวจสอบ Google รองรับ `?lang=xx`):
  - โดเมนแผงหลัก: `GET /p/legal/privacy`, `GET /p/legal/terms`
  - โดเมนพอร์ทัลเฉพาะ (การปรับใช้รูท): `GET /legal/privacy`, `GET /legal/terms`
- **ตรรกะการแสดงผล**: URL หรือเนื้อหา **URL ชนะ** (เปิดจากภายนอกในแท็บใหม่) หากมีเนื้อหาให้คลิก
 การ์ดลงชื่อเข้าใช้จะแสดง Markdown ในกล่องโต้ตอบ (`renderMd`) โดยใช้ภาษาพอร์ทัลปัจจุบัน
- ส่งที่รันไทม์ผ่าน `/api/v1/public/portal-config`; องค์ประกอบการลงชื่อเข้าใช้สิ้นเปลือง
 `ctx.privacyUrl / termsUrl / privacyContent / termsContent / openLegal / hasLegalLinks`.
- การตรวจสอบ Google OAuth กำหนดให้ลิงก์เหล่านี้เข้าถึงได้แบบสาธารณะ → ใช้ลิงก์แบบสแตนด์อโลนด้านบนใน
 หน้าจอยินยอม ดู `docs/oauth-google-verification.md` (ประกอบด้วยนโยบายความเป็นส่วนตัว / เทมเพลตข้อกำหนด)

## 18. ตัวอย่างการบูรณาการ

```js
// Sign in
const { data } = await fetch('/api/v1/client-auth/login', {
  method: 'POST', headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ email, password })
}).then(r => r.json());
localStorage.setItem('client_token', data.access_token);
localStorage.setItem('client_refresh_token', data.refresh_token);

// Authenticated request
const me = await fetch('/api/v1/client-auth/me', {
  headers: { Authorization: 'Bearer ' + data.access_token }
}).then(r => r.json());
```

บน 401 ให้โทรไปที่ `POST /refresh` หนึ่งครั้งแล้วลองอีกครั้ง หากยังคงล้มเหลว ให้ล้างเซสชันในเครื่อง
(`api()` ของเทมเพลตพอร์ทัลใช้สิ่งนี้แล้ว)
