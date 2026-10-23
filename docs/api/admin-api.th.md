<!-- source: admin-api.md | locale: th | updated: 2026-10-23 -->
# แอดมิน API

> ภาษา: [简体中文](admin-api.md) · [English](admin-api.en.md) · [繁體中文](admin-api.zh-TW.md) · [Русский](admin-api.ru.md) · [日本語](admin-api.ja.md) · [한국어](admin-api.ko.md) · [Deutsch](admin-api.de.md) · [Français](admin-api.fr.md) · [Español](admin-api.es.md) · [Português](admin-api.pt.md) · [Türkçe](admin-api.tr.md) · [Bahasa Indonesia](admin-api.id.md) · ไทย · [Tiếng Việt](admin-api.vi.md) · [العربية](admin-api.ar.md) · [فارسی](admin-api.fa.md)

ตารางเส้นทางผู้ดูแลระบบ API ที่สมบูรณ์สำหรับ **การพัฒนาแผงควบคุม / ระบบอัตโนมัติ / ผู้รวมระบบ**

- ฐาน URL: `https://<panel-domain>/api/v1`
- การรับรองความถูกต้อง: `Authorization: Bearer <admin_token>` (จาก `POST /api/v1/auth/login`)
- รูปแบบการตอบสนอง/ข้อผิดพลาดทั่วไปและแบบแผนการจำกัดอัตรา: ดู [README.md](./README.th.md)
- แหล่งที่มาของความจริง: `internal/api/router/router.go` (เส้นทางผู้ดูแลระบบ 739 เส้นทาง); หากตารางนี้ไม่เห็นด้วย **โค้ดจะชนะ**

## เครื่องหมายอนุญาต

| เครื่องหมาย | ความหมาย |
|---|---|
| สาธารณะ | ไม่จำเป็นต้องลงชื่อเข้าใช้ (อาจมีเจ้าหน้าที่ดูแลบำรุงรักษา / การจำกัดอัตรา) |
| สาธารณะ · จำกัดอัตรา | ไม่มีการลงชื่อเข้าใช้ โดยมีการจำกัดอัตราเฉพาะ |
| ลงชื่อเข้าใช้แล้ว | ต้องมีโทเค็นผู้ดูแลระบบที่ลงชื่อเข้าใช้ (`AuthRequired`) |
| ผู้ดูแลระบบ | ต้องการบทบาท ≥ `admin` (`RequireRole(RoleAdmin)`) |
| ผู้ดูแลระบบขั้นสูง | ต้องการบทบาท = `super_admin` |

ระดับบทบาท: `viewer(10) < operator(20) < admin(30) < super_admin(40)`; บทบาทไม่เพียงพอส่งคืน 403 `INSUFFICIENT_ROLE`

---

## เริ่มต้นอย่างรวดเร็ว (ตัวอย่างจริง)

ตัวอย่างเหล่านี้มาจากคำตอบในท้องถิ่นที่แท้จริง โทเค็นและข้อมูลจะถูกแก้ไข

### 1. ลงชื่อเข้าใช้และรับโทเค็น

```bash
curl -X POST https://<panel-domain>/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"your-password"}'
```

การตอบสนอง:

```json
{
  "data": {
    "access_token": "<JWT, valid 24h>",
    "refresh_token": "<JWT, valid 7 days>",
    "user": { "id": 1, "role": "super_admin", "username": "admin" }
  }
}
```

### 2. โทรหาจุดสิ้นสุดธุรกิจด้วยโทเค็น

```bash
curl https://<panel-domain>/api/v1/clients?limit=1 \
  -H 'Authorization: Bearer <access_token>'
```

การตอบสนอง (รายการใช้โครงสร้าง `items` + `total`):

```json
{
  "data": {
    "items": [
      {
        "id": 27,
        "email": "user@example.com",
        "status": "active",
        "quota_bytes": 10737418240,
        "used_bytes": 0,
        "plan_id": 1,
        "expires_at": null,
        "created_at": "2026-09-30T15:54:32+08:00"
      }
    ],
    "total": 22,
    "limit": 1,
    "offset": 0
  }
}
```

### 3. ตัวอย่างการตอบสนองข้อผิดพลาด

**401 ไม่ผ่านการรับรองความถูกต้อง:**

```json
{
  "code": "MISSING_TOKEN",
  "message": "missing authorization header",
  "request_id": "a02cfdf9-6be0-46be-8584-824caab34dc7",
  "can_retry": false,
  "requires_action": false
}
```

**ไม่พบทรัพยากร 404:**

```json
{
  "code": "CLIENT_NOT_FOUND",
  "message": "client not found",
  "request_id": "099949bd-4ded-468d-a6f8-f8e798af2273",
  "can_retry": false,
  "requires_action": false
}
```

### 4. การแบ่งหน้า

จุดสิ้นสุดรายการทั้งหมดใช้ `limit` (ขนาดหน้า ค่าเริ่มต้น 20) และ `offset` (แบบ 0) และส่งคืน `{items, total, limit, offset}` จุดสิ้นสุดแบบเดิมบางจุดส่งคืนอาร์เรย์เปล่า (ระบุไว้ต่อตาราง)

---

## สารบัญ

- [การรับรองความถูกต้องและความปลอดภัยของผู้ดูแลระบบ](#การรบรองความถกตองและความปลอดภยของผดแลระบบ)
- [บัญชีผู้ดูแลระบบ](#บญชผดแลระบบ)
- [การจัดการลูกค้า](#การจดการลกคา)
- [แม่แบบแผน](#แมแบบแผน)
- [ตัวแปรแผน](#ตวแปรแผน)
- [ชุดจราจร](#ชดจราจร)
- [คูปอง](#คปอง)
- [กลุ่มโหนด](#กลมโหนด)
- [เซิร์ฟเวอร์โหนด](#เซรฟเวอรโหนด)
- [บันทึกการตรวจสอบโหนด](#บนทกการตรวจสอบโหนด)
- [โหนด (โมเดลดั้งเดิม เลิกใช้แล้ว)](#โหนด-โมเดลดงเดม-เลกใชแลว)
- [ภาพรวมภูมิภาค](#ภาพรวมภมภาค)
- [การสมัครรับข้อมูล](#การสมครรบขอมล)
- [รูปแบบการสมัครสมาชิก](#รปแบบการสมครสมาชก)
- จุดสิ้นสุดการสมัครสมาชิก [](#subscription-endpoints)
- [จุดสิ้นสุดการสมัครสมาชิก](#จดสนสดการสมครสมาชก)
- [แบ่งปันโดเมน](#แบงปนโดเมน)
- [Cloudflare ไอพี](#cloudflare-ไอพ)
- [การกำหนดค่าการสมัครสมาชิก (Mihomo/Clash)](#การกำหนดคาการสมครสมาชก-mihomoclash)
- [เว็บไซต์เติมเงิน](#เวบไซตเตมเงน)
- [สั่งเติมเงิน](#สงเตมเงน)
- [บัตรเติมเงิน](#บตรเตมเงน)
- [ชาร์จกิจกรรมการโทรกลับ](#ชารจกจกรรมการโทรกลบ)
- [ลองโทรกลับเรียกเก็บเงินอีกครั้ง](#ลองโทรกลบเรยกเกบเงนอกครง)
- [ยอดลูกค้า](#ยอดลกคา)
- [ของขวัญ](#ของขวญ)
- [การกำหนดค่าการอ้างอิง](#การกำหนดคาการอางอง)
- [การถอนการอ้างอิง](#การถอนการอางอง)
- [รายการอนุญาต IP (สรุป)](#รายการอนญาต-ip-สรป)
- [การเชื่อมโยง OAuth ของลูกค้า](#การเชอมโยง-oauth-ของลกคา)
- [ตั๋ว (ผู้ดูแลระบบ)](#ตว-ผดแลระบบ)
- [บทความ](#บทความ)
- [บัญชีที่ใช้ร่วมกัน](#บญชทใชรวมกน)
- [ใบรับรอง](#ใบรบรอง)
- [ช่องทางการรับรอง](#ชองทางการรบรอง)
- [การกำหนดค่า EAB](#การกำหนดคา-eab)
- [CA](#built-in-ca) ในตัว
- [บัญชี ACME](#บญช-acme)
- [CA ในตัว](#ca-ในตว)
- [กฎทางภูมิศาสตร์](#กฎทางภมศาสตร)
- [ไลบรารีกฎ WAF](#ไลบรารกฎ-waf)
- [การจัดการบันทึก](#การจดการบนทก)
- [การสำรองข้อมูล](#การสำรองขอมล)
- [การตั้งค่าระบบ](#การตงคาระบบ)
- [การตรวจสอบความปลอดภัย](#การตรวจสอบความปลอดภย)
- [เมตริก](#เมตรก)
- [เทมเพลตพอร์ทัล](#เทมเพลตพอรทล)
- [การติดตั้งตัวแทน](#การตดตงตวแทน)
- [เรียกเก็บเงินรายการโทรกลับ](#เรยกเกบเงนรายการโทรกลบ)
- [ขาเข้า (พร้อมโหนด)](#ขาเขา-พรอมโหนด)
- [การกำหนดค่าสาธารณะ](#การกำหนดคาสาธารณะ)
- [การตรวจสอบสิทธิ์ลูกค้าและธุรกิจ (ชุดเดียวกันในเครื่องมือผู้ดูแลระบบ)](#การตรวจสอบสทธลกคาและธรกจ-ชดเดยวกนในเครองมอผดแลระบบ)
- [การจัดส่งการสมัครสมาชิก](#การจดสงการสมครสมาชก)
- [เวิร์กโฟลว์การกำหนดค่ารันไทม์](#เวรกโฟลวการกำหนดคารนไทม)
- [นโยบายการเข้าถึงบริการ (เลิกใช้แล้ว)](#นโยบายการเขาถงบรการ-เลกใชแลว)
- [การตรวจสอบโควต้าการจราจร](#การตรวจสอบโควตาการจราจร)
- [การตรวจสุขภาพ](#การตรวจสขภาพ)

---

### การรับรองความถูกต้องและความปลอดภัยของผู้ดูแลระบบ

ลงชื่อเข้าใช้ ออกจากระบบ การเปลี่ยนรหัสผ่าน 2FA (TOTP), Passkey (WebAuthn) และตั๋วแบบครั้งเดียวสำหรับแผงผู้ดูแลระบบ

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| POST | `/api/v1/auth/2fa/disable` | ลงชื่อเข้าใช้แล้ว | ปิดการใช้งาน |
| POST | `/api/v1/auth/2fa/enable` | ลงชื่อเข้าใช้แล้ว | เปิดใช้งาน |
| POST | `/api/v1/auth/2fa/recovery-codes` | ลงชื่อเข้าใช้แล้ว | สร้างรหัสการกู้คืนใหม่ |
| POST | `/api/v1/auth/2fa/setup` | ลงชื่อเข้าใช้แล้ว | การตั้งค่า |
| GET | `/api/v1/auth/2fa/status` | ลงชื่อเข้าใช้แล้ว | สถานะ |
| POST | `/api/v1/auth/change-password` | ลงชื่อเข้าใช้แล้ว | เปลี่ยนรหัสผ่าน |
| POST | `/api/v1/auth/login` | สาธารณะ | เข้าสู่ระบบ |
| POST | `/api/v1/auth/login/2fa` | สาธารณะ | การลงชื่อเข้าใช้ด้วยโค้ดแบบไดนามิก (2FA) |
| GET | `/api/v1/auth/login/2fa-status` | สาธารณะ | ตรวจสอบว่าบัญชีเปิดใช้งาน 2FA หรือไม่ |
| POST | `/api/v1/auth/logout` | ลงชื่อเข้าใช้แล้ว | ออกจากระบบ |
| GET | `/api/v1/auth/me` | ลงชื่อเข้าใช้แล้ว | ผู้ใช้ที่ลงชื่อเข้าใช้ปัจจุบัน |
| GET | `/api/v1/auth/passkey/credentials` | ลงชื่อเข้าใช้แล้ว | รายการข้อมูลรับรอง Passkey |
| DELETE | `/api/v1/auth/passkey/credentials/:id` | ลงชื่อเข้าใช้แล้ว | ลบข้อมูลรับรอง Passkey |
| POST | `/api/v1/auth/passkey/login/begin` | สาธารณะ | เริ่มการลงชื่อเข้าใช้ |
| POST | `/api/v1/auth/passkey/login/finish` | สาธารณะ | ลงชื่อเข้าใช้ให้เสร็จสิ้น |
| POST | `/api/v1/auth/passkey/register/begin` | ลงชื่อเข้าใช้แล้ว | เริ่มการลงทะเบียน |
| POST | `/api/v1/auth/passkey/register/finish` | ลงชื่อเข้าใช้แล้ว | ลงทะเบียนให้เสร็จสมบูรณ์ |
| GET | `/api/v1/auth/passkey/rpids` | ลงชื่อเข้าใช้แล้ว | รายการที่อนุญาต RP ID |
| POST | `/api/v1/auth/passkey/rpids` | ลงชื่อเข้าใช้แล้ว | อัปเดตรายการที่อนุญาตของ RP ID |
| GET | `/api/v1/auth/passkey/status` | สาธารณะ | สถานะ |
| POST | `/api/v1/auth/refresh` | สาธารณะ | รีเฟรช |
| POST | `/api/v1/auth/ticket` | ลงชื่อเข้าใช้แล้ว | ออกตั๋วอายุสั้นครั้งเดียว |

### บัญชีผู้ดูแลระบบ

บัญชีผู้ใช้แผงผู้ดูแลระบบ รายการต้องมีผู้ดูแลระบบ สร้าง/อัปเดต/ลบ ต้องใช้ผู้ดูแลระบบขั้นสูง (ป้องกันการเลื่อนระดับสิทธิ์)

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| GET | `/api/v1/users` | ผู้ดูแลระบบ | รายการ |
| POST | `/api/v1/users` | ผู้ดูแลระบบขั้นสูง | สร้าง |
| DELETE | `/api/v1/users/:id` | ผู้ดูแลระบบขั้นสูง | ลบ |
| PUT | `/api/v1/users/:id` | ผู้ดูแลระบบขั้นสูง | อัปเดต |

### การจัดการลูกค้า

CRUD, ระงับ/ดำเนินการต่อ, อัปเกรด/ดาวน์เกรด, ต่ออายุ และรีเซ็ตการใช้งานสำหรับผู้สมัครสมาชิก (บัญชีพอร์ทัล)

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| GET | `/api/v1/clients` | ลงชื่อเข้าใช้แล้ว | รายชื่อลูกค้า |
| POST | `/api/v1/clients` | ลงชื่อเข้าใช้แล้ว | สร้างลูกค้า. ร่างกาย: `{"email":"user@example.com","password":"plaintext-password","plan_id":1,"quota_bytes":10737418240,"expires_at":"2026-12-31T00:00:00Z","remark":""}`. จำเป็นต้องมี `plan_id`; `password` อาจว่างเปล่า (ลูกค้าสามารถตั้งค่าได้ในภายหลังในพอร์ทัล) |
| DELETE | `/api/v1/clients/:id` | ลงชื่อเข้าใช้แล้ว | ลบลูกค้า |
| GET | `/api/v1/clients/:id` | ลงชื่อเข้าใช้แล้ว | รับลูกค้า |
| PUT | `/api/v1/clients/:id` | ลงชื่อเข้าใช้แล้ว | อัพเดทลูกค้าครับ |
| POST | `/api/v1/clients/:id/downgrade` | ลงชื่อเข้าใช้แล้ว | ดาวน์เกรดลูกค้า |
| GET | `/api/v1/clients/:id/orders` | ลงชื่อเข้าใช้แล้ว | รายการสั่งซื้อของลูกค้า |
| POST | `/api/v1/clients/:id/renew` | ลงชื่อเข้าใช้แล้ว | ต่ออายุลูกค้า |
| POST | `/api/v1/clients/:id/reset-usage` | ลงชื่อเข้าใช้แล้ว | รีเซ็ตการใช้งาน |
| POST | `/api/v1/clients/:id/restore` | ลงชื่อเข้าใช้แล้ว | ดำเนินการต่อลูกค้า |
| POST | `/api/v1/clients/:id/suspend` | ลงชื่อเข้าใช้แล้ว | ระงับลูกค้า |
| POST | `/api/v1/clients/:id/upgrade` | ลงชื่อเข้าใช้แล้ว | อัพเกรดลูกค้า |

### แม่แบบแผน

แผน (ตัวแปร รอบการเรียกเก็บเงิน ราคา โควต้าการรับส่งข้อมูล) และการอนุญาตโหนดระดับเทมเพลต

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| GET | `/api/v1/plan-templates` | ลงชื่อเข้าใช้แล้ว | รายการเทมเพลต |
| POST | `/api/v1/plan-templates` | ลงชื่อเข้าใช้แล้ว | สร้างเทมเพลต |
| DELETE | `/api/v1/plan-templates/:id` | ลงชื่อเข้าใช้แล้ว | ลบเทมเพลต |
| GET | `/api/v1/plan-templates/:id` | ลงชื่อเข้าใช้แล้ว | รายละเอียดเทมเพลต |
| PUT | `/api/v1/plan-templates/:id` | ลงชื่อเข้าใช้แล้ว | อัปเดตเทมเพลต |
| GET | `/api/v1/plan-templates/:id/accessible-nodes` | ลงชื่อเข้าใช้แล้ว | รับโหนดที่พร้อมใช้งาน |
| GET | `/api/v1/plan-templates/:id/billing-node-options` | ลงชื่อเข้าใช้แล้ว | รับตัวเลือกโหนดที่เรียกเก็บเงินได้ |
| PUT | `/api/v1/plan-templates/:id/node-groups` | ลงชื่อเข้าใช้แล้ว | ตั้งค่ากลุ่มโหนดเทมเพลต |
| POST | `/api/v1/plan-templates/:id/sync-accounts` | ลงชื่อเข้าใช้แล้ว | ซิงค์บัญชีกับเทมเพลต |
| GET | `/api/v1/plan-templates/:id/variants` | ลงชื่อเข้าใช้แล้ว | รายการตัวแปร |
| POST | `/api/v1/plan-templates/:id/variants` | ลงชื่อเข้าใช้แล้ว | สร้างตัวแปร |
| POST | `/api/v1/plan-templates/sync-all-accounts` | ลงชื่อเข้าใช้แล้ว | ซิงค์บัญชีทั้งหมด |

### ตัวแปรแผน

อัปเดตและลบแผนแบบต่างๆ (การสร้างซ้อนอยู่ใต้เทมเพลตแผน)

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| DELETE | `/api/v1/plan-variants/:id` | ลงชื่อเข้าใช้แล้ว | ลบตัวแปร |
| PUT | `/api/v1/plan-variants/:id` | ลงชื่อเข้าใช้แล้ว | อัปเดตตัวแปร |

### ชุดจราจร

ชุดจราจรที่สามารถซื้อแยกต่างหากได้

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| GET | `/api/v1/traffic-packs` | ลงชื่อเข้าใช้แล้ว | รายการชุดการจราจร |
| POST | `/api/v1/traffic-packs` | ลงชื่อเข้าใช้แล้ว | สร้างชุดการรับส่งข้อมูล |
| DELETE | `/api/v1/traffic-packs/:id` | ลงชื่อเข้าใช้แล้ว | ลบชุดการรับส่งข้อมูล |
| PUT | `/api/v1/traffic-packs/:id` | ลงชื่อเข้าใช้แล้ว | อัพเดตชุดการจราจร |

### คูปอง

สร้าง ตรวจสอบ และแลกคูปอง

| วิธี | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| GET | `/api/v1/promotions` | ลงชื่อเข้าใช้แล้ว | แสดงรายการคูปอง |
| POST | `/api/v1/promotions` | ลงชื่อเข้าใช้แล้ว | สร้างคูปอง |
| DELETE | `/api/v1/promotions/:id` | ลงชื่อเข้าใช้แล้ว | ลบคูปอง |
| PUT | `/api/v1/promotions/:id` | ลงชื่อเข้าใช้แล้ว | อัพเดทคูปอง |
| POST | `/api/v1/promotions/validate` | ลงชื่อเข้าใช้แล้ว | ตรวจสอบคูปอง |

### กลุ่มโหนด

จัดระเบียบโหนดเป็นกลุ่มที่อ้างอิงตามแผน

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| GET | `/api/v1/node-groups` | ลงชื่อเข้าใช้แล้ว | แสดงรายการกลุ่มโหนด |
| POST | `/api/v1/node-groups` | ลงชื่อเข้าใช้แล้ว | สร้างกลุ่มโหนด |
| DELETE | `/api/v1/node-groups/:id` | ลงชื่อเข้าใช้แล้ว | ลบกลุ่มโหนด |
| PUT | `/api/v1/node-groups/:id` | ลงชื่อเข้าใช้แล้ว | อัปเดตกลุ่มโหนด |
| GET | `/api/v1/node-groups/:id/members` | ลงชื่อเข้าใช้แล้ว | รายชื่อสมาชิกกลุ่ม |
| POST | `/api/v1/node-groups/:id/members` | ลงชื่อเข้าใช้แล้ว | เพิ่มสมาชิกกลุ่ม |
| DELETE | `/api/v1/node-groups/:id/members/:mid` | ลงชื่อเข้าใช้แล้ว | ลบสมาชิกกลุ่ม |

### เซิร์ฟเวอร์โหนด

ทรัพยากรหลัก: โหนด CRUD, การส่งมอบการกำหนดค่าคอร์ (xray/sing-box), การกำหนดค่าขาเข้า/ขาออก/ปลายทาง, การควบคุมกระบวนการ, ใบรับรอง, WAF, ขีดจำกัดอัตรา, การข้ามพอร์ต, พร็อกซีแบบลูกโซ่, มิเรอร์/การสำรองข้อมูลหลัก, การติดตั้ง/อัปเกรดเอเจนต์, การควบคุมรันไทม์ xray เส้นทางย่อยจำนวนมาก จัดกลุ่มตามตระกูลฟีเจอร์

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| GET | `/api/v1/node-servers` | ผู้ดูแลระบบ | รายการ |
| POST | `/api/v1/node-servers` | ผู้ดูแลระบบ | สร้าง |
| DELETE | `/api/v1/node-servers/:id` | ผู้ดูแลระบบ | ลบ |
| GET | `/api/v1/node-servers/:id` | ผู้ดูแลระบบ | รายละเอียด |
| PUT | `/api/v1/node-servers/:id` | ผู้ดูแลระบบ | อัปเดต |
| GET | `/api/v1/node-servers/:id/binaries` | ผู้ดูแลระบบ | รายการหลักที่ติดตั้ง |
| POST | `/api/v1/node-servers/:id/binaries/ech-generate` | ผู้ดูแลระบบ | สร้างคู่คีย์ ECH |
| POST | `/api/v1/node-servers/:id/binaries/install` | ผู้ดูแลระบบ | ติดตั้ง/อัพเกรดคอร์ |
| GET | `/api/v1/node-servers/:id/binaries/interfaces` | ผู้ดูแลระบบ | รายการอินเทอร์เฟซเครือข่าย |
| POST | `/api/v1/node-servers/:id/binaries/mldsa65-key` | ผู้ดูแลระบบ | สร้างคีย์ MLDSA65 |
| POST | `/api/v1/node-servers/:id/binaries/reality-key` | ผู้ดูแลระบบ | สร้างคู่คีย์ REALITY |
| POST | `/api/v1/node-servers/:id/binaries/reality-public-key` | ผู้ดูแลระบบ | รับรหัสสาธารณะจากรหัสส่วนตัว REALITY |
| GET | `/api/v1/node-servers/:id/binaries/releases` | ผู้ดูแลระบบ | รุ่นที่มีจำหน่าย |
| POST | `/api/v1/node-servers/:id/binaries/vlessenc` | ผู้ดูแลระบบ | สร้างพารามิเตอร์การเข้ารหัส VLESS |
| POST | `/api/v1/node-servers/:id/binaries/wg-key` | ผู้ดูแลระบบ | สร้างคีย์ WireGuard |
| POST | `/api/v1/node-servers/:id/binaries/x25519-key` | ผู้ดูแลระบบ | สร้างคู่คีย์ X25519 |
| GET | `/api/v1/node-servers/:id/bootstrap-cmd` | ผู้ดูแลระบบ | รับคำสั่งบูตเอเจนต์ |
| POST | `/api/v1/node-servers/:id/bootstrap-token` | ผู้ดูแลระบบ | สร้างโทเค็นบูตสแตรป |
| POST | `/api/v1/node-servers/:id/caddy/batch-update-log-enabled` | ผู้ดูแลระบบ | สวิตช์บันทึกการอัพเดตจำนวนมาก |
| POST | `/api/v1/node-servers/:id/caddy/batch-update-ports` | ผู้ดูแลระบบ | พอร์ตการอัพเดตจำนวนมาก |
| POST | `/api/v1/node-servers/:id/caddy/generate-config` | ผู้ดูแลระบบ | สร้างการกำหนดค่า |
| DELETE | `/api/v1/node-servers/:id/caddy/l4-rules` | ผู้ดูแลระบบ | ลบกฎ L4 ทั้งหมด |
| POST | `/api/v1/node-servers/:id/caddy/save-and-apply` | ผู้ดูแลระบบ | บันทึกและนำไปใช้ |
| GET | `/api/v1/node-servers/:id/caddy/sites` | ผู้ดูแลระบบ | รายการไซต์ |
| POST | `/api/v1/node-servers/:id/caddy/sites` | ผู้ดูแลระบบ | สร้างไซต์ |
| DELETE | `/api/v1/node-servers/:id/caddy/sites/:siteId` | ผู้ดูแลระบบ | ลบไซต์ |
| PUT | `/api/v1/node-servers/:id/caddy/sites/:siteId` | ผู้ดูแลระบบ | อัปเดตไซต์ |
| DELETE | `/api/v1/node-servers/:id/caddy/sni-rules` | ผู้ดูแลระบบ | ลบกฎ SNI ทั้งหมด |
| POST | `/api/v1/node-servers/:id/caddy/sni-rules/toggle` | ผู้ดูแลระบบ | สลับกฎ SNI |
| GET | `/api/v1/node-servers/:id/caddy/sni-settings` | ผู้ดูแลระบบ | การตั้งค่า SNI |
| POST | `/api/v1/node-servers/:id/caddy/sni-settings` | ผู้ดูแลระบบ | บันทึกการตั้งค่า SNI |
| GET | `/api/v1/node-servers/:id/certificates` | ผู้ดูแลระบบ | รายการใบรับรองโหนด |
| DELETE | `/api/v1/node-servers/:id/certificates/:cid` | ผู้ดูแลระบบ | เลิกผูกใบรับรอง |
| POST | `/api/v1/node-servers/:id/certificates/:cid` | ผู้ดูแลระบบ | ใบรับรองการผูกมัด |
| GET | `/api/v1/node-servers/:id/chain-config` | ผู้ดูแลระบบ | สร้างการกำหนดค่าแบบลูกโซ่ |
| GET | `/api/v1/node-servers/:id/chain-health` | ผู้ดูแลระบบ | สถานะสุขภาพของโซ่ |
| POST | `/api/v1/node-servers/:id/control` | ผู้ดูแลระบบขั้นสูง | แกนโหนดควบคุม เนื้อหา: `{"core_type":"xray","action":"start"}` (การกระทำ = เริ่ม/หยุด/รีสตาร์ท) `core_type` สามารถเป็น `xray`/`singbox` |
| GET | `/api/v1/node-servers/:id/degradation-status` | ผู้ดูแลระบบ | สถานะเสื่อมโทรม |
| POST | `/api/v1/node-servers/:id/distribute-cert` | ผู้ดูแลระบบ | แจกใบรับรอง |
| GET | `/api/v1/node-servers/:id/endpoint-configs` | ผู้ดูแลระบบ | รายการ |
| POST | `/api/v1/node-servers/:id/endpoint-configs` | ผู้ดูแลระบบ | สร้าง |
| DELETE | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | ผู้ดูแลระบบ | ลบ |
| GET | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | ผู้ดูแลระบบ | รายละเอียด |
| PUT | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | ผู้ดูแลระบบ | อัปเดต |
| POST | `/api/v1/node-servers/:id/failover` | ผู้ดูแลระบบ | ทริกเกอร์เฟลโอเวอร์สำหรับโหนดนี้ |
| GET | `/api/v1/node-servers/:id/failover-status` | ผู้ดูแลระบบ | สถานะการเฟลโอเวอร์ |
| POST | `/api/v1/node-servers/:id/force-reconnect` | ผู้ดูแลระบบ | บังคับให้เชื่อมต่อใหม่ |
| GET | `/api/v1/node-servers/:id/free-port` | ผู้ดูแลระบบ | ขอพอร์ตฟรี |
| POST | `/api/v1/node-servers/:id/geo-rules/push` | ผู้ดูแลระบบ | ดันไปที่โหนดนี้ |
| GET | `/api/v1/node-servers/:id/inbound-configs` | ผู้ดูแลระบบ | รายการ |
| POST | `/api/v1/node-servers/:id/inbound-configs` | ผู้ดูแลระบบ | สร้าง |
| DELETE | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | ผู้ดูแลระบบ | ลบ |
| GET | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | ผู้ดูแลระบบ | รายละเอียด |
| PUT | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | ผู้ดูแลระบบ | อัปเดต |
| GET | `/api/v1/node-servers/:id/inbound-configs/:cfgId/share-links` | ผู้ดูแลระบบ | แบ่งปันลิงก์และตัวอย่าง |
| POST | `/api/v1/node-servers/:id/inbound-configs/:cfgId/users/:email/pause` | ผู้ดูแลระบบ | ระงับผู้ใช้รายนี้ |
| POST | `/api/v1/node-servers/:id/inbound-configs/:cfgId/users/:email/resume` | ผู้ดูแลระบบ | คืนค่าผู้ใช้รายนี้ |
| POST | `/api/v1/node-servers/:id/inbound-configs/reset-all-traffic` | ผู้ดูแลระบบ | รีเซ็ตการรับส่งข้อมูลขาเข้าทั้งหมด |
| GET | `/api/v1/node-servers/:id/install-script` | ผู้ดูแลระบบ | รับสคริปต์การติดตั้งตัวแทน |
| GET | `/api/v1/node-servers/:id/metrics-history` | ผู้ดูแลระบบ | รับประวัติเมตริก |
| GET | `/api/v1/node-servers/:id/mirror-status` | ผู้ดูแลระบบ | สถานะมิเรอร์ |
| POST | `/api/v1/node-servers/:id/nginx/batch-update-ports` | ผู้ดูแลระบบ | พอร์ตการอัพเดตจำนวนมาก |
| POST | `/api/v1/node-servers/:id/nginx/generate-config` | ผู้ดูแลระบบ | สร้างการกำหนดค่า |
| DELETE | `/api/v1/node-servers/:id/nginx/l4-rules` | ผู้ดูแลระบบ | ลบกฎ L4 ทั้งหมด |
| POST | `/api/v1/node-servers/:id/nginx/save-and-apply` | ผู้ดูแลระบบ | บันทึกและนำไปใช้ |
| GET | `/api/v1/node-servers/:id/nginx/settings` | ผู้ดูแลระบบ | อ่านการตั้งค่า |
| POST | `/api/v1/node-servers/:id/nginx/settings` | ผู้ดูแลระบบ | บันทึกการตั้งค่า |
| GET | `/api/v1/node-servers/:id/nginx/sites` | ผู้ดูแลระบบ | รายการไซต์ |
| POST | `/api/v1/node-servers/:id/nginx/sites` | ผู้ดูแลระบบ | สร้างไซต์ |
| DELETE | `/api/v1/node-servers/:id/nginx/sites/:siteId` | ผู้ดูแลระบบ | ลบไซต์ |
| PUT | `/api/v1/node-servers/:id/nginx/sites/:siteId` | ผู้ดูแลระบบ | อัปเดตไซต์ |
| DELETE | `/api/v1/node-servers/:id/nginx/sni-rules` | ผู้ดูแลระบบ | ลบกฎ SNI ทั้งหมด |
| POST | `/api/v1/node-servers/:id/nginx/sni-rules/toggle` | ผู้ดูแลระบบ | สลับกฎ SNI |
| PUT | `/api/v1/node-servers/:id/node-type` | ผู้ดูแลระบบ | อัปเดตประเภทโหนด |
| GET | `/api/v1/node-servers/:id/online-users` | ผู้ดูแลระบบ | ผู้ใช้ออนไลน์ |
| GET | `/api/v1/node-servers/:id/outbound-configs` | ผู้ดูแลระบบ | รายการ |
| POST | `/api/v1/node-servers/:id/outbound-configs` | ผู้ดูแลระบบ | สร้าง |
| DELETE | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | ผู้ดูแลระบบ | ลบ |
| GET | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | ผู้ดูแลระบบ | รายละเอียด |
| PUT | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | ผู้ดูแลระบบ | อัปเดต |
| PUT | `/api/v1/node-servers/:id/outbound-configs/reorder` | ผู้ดูแลระบบ | จัดเรียง |
| GET | `/api/v1/node-servers/:id/port-hop/rules` | ผู้ดูแลระบบ | การแสดงตัวอย่างกฎการข้ามพอร์ต |
| GET | `/api/v1/node-servers/:id/protocols` | ผู้ดูแลระบบ | รายการโปรโตคอล |
| GET | `/api/v1/node-servers/:id/protocols/:coreType` | ผู้ดูแลระบบ | การกำหนดค่าโปรโตคอล |
| PUT | `/api/v1/node-servers/:id/protocols/:coreType` | ผู้ดูแลระบบ | บันทึกการกำหนดค่าโปรโตคอล |
| GET | `/api/v1/node-servers/:id/protocols/:coreType/config-file` | ผู้ดูแลระบบ | อ่านไฟล์กำหนดค่าหลัก |
| GET | `/api/v1/node-servers/:id/protocols/:coreType/section/:section` | ผู้ดูแลระบบ | อ่านส่วนโปรโตคอล |
| PUT | `/api/v1/node-servers/:id/protocols/:coreType/section/:section` | ผู้ดูแลระบบ | บันทึกส่วนโปรโตคอล |
| POST | `/api/v1/node-servers/:id/protocols/:coreType/validate` | ผู้ดูแลระบบ | ตรวจสอบการกำหนดค่า (การทดลองใช้งานคอร์จริง) |
| GET | `/api/v1/node-servers/:id/proxy-status` | ผู้ดูแลระบบ | สถานะพร็อกซีระยะไกล |
| POST | `/api/v1/node-servers/:id/proxy/batch-update-ports` | ผู้ดูแลระบบ | พอร์ตการอัพเดตจำนวนมาก |
| GET | `/api/v1/node-servers/:id/proxy/config` | ผู้ดูแลระบบ | อ่านการกำหนดค่า |
| PUT | `/api/v1/node-servers/:id/proxy/config` | ผู้ดูแลระบบ | อัปเดตการกำหนดค่า |
| POST | `/api/v1/node-servers/:id/proxy/control` | ผู้ดูแลระบบ | การควบคุมการลงทะเบียนพร็อกซีที่ถูกล่ามโซ่ |
| POST | `/api/v1/node-servers/:id/proxy/download` | ผู้ดูแลระบบ | ดาวน์โหลดเวอร์ชันหลักของพร็อกซี |
| POST | `/api/v1/node-servers/:id/proxy/generate-config` | ผู้ดูแลระบบ | สร้างการกำหนดค่า |
| POST | `/api/v1/node-servers/:id/proxy/guardian` | ผู้ดูแลระบบ | อัปเดตการกำหนดค่าหัวหน้างาน |
| POST | `/api/v1/node-servers/:id/proxy/install` | ผู้ดูแลระบบ | ติดตั้งพร็อกซี |
| DELETE | `/api/v1/node-servers/:id/proxy/l4-rules` | ผู้ดูแลระบบ | ลบกฎ L4 ทั้งหมด |
| GET | `/api/v1/node-servers/:id/proxy/l4-settings` | ผู้ดูแลระบบ | การตั้งค่า L4 |
| POST | `/api/v1/node-servers/:id/proxy/l4-settings` | ผู้ดูแลระบบ | บันทึกการตั้งค่า L4 |
| GET | `/api/v1/node-servers/:id/proxy/log-settings` | ผู้ดูแลระบบ | การตั้งค่าบันทึก |
| POST | `/api/v1/node-servers/:id/proxy/log-settings` | ผู้ดูแลระบบ | บันทึกการตั้งค่าบันทึก |
| GET | `/api/v1/node-servers/:id/proxy/nginx-settings` | ผู้ดูแลระบบ | การตั้งค่า nginx |
| POST | `/api/v1/node-servers/:id/proxy/nginx-settings` | ผู้ดูแลระบบ | บันทึกการตั้งค่า nginx |
| GET | `/api/v1/node-servers/:id/proxy/releases` | ผู้ดูแลระบบ | รุ่นที่มีจำหน่าย |
| POST | `/api/v1/node-servers/:id/proxy/reload` | ผู้ดูแลระบบ | โหลดการกำหนดค่าซ้ำ |
| POST | `/api/v1/node-servers/:id/proxy/restart` | ผู้ดูแลระบบ | รีสตาร์ท |
| GET | `/api/v1/node-servers/:id/proxy/sites` | ผู้ดูแลระบบ | รายการไซต์ |
| POST | `/api/v1/node-servers/:id/proxy/sites` | ผู้ดูแลระบบ | สร้างไซต์ |
| DELETE | `/api/v1/node-servers/:id/proxy/sites/:siteId` | ผู้ดูแลระบบ | ลบไซต์ |
| PUT | `/api/v1/node-servers/:id/proxy/sites/:siteId` | ผู้ดูแลระบบ | อัปเดตไซต์ |
| DELETE | `/api/v1/node-servers/:id/proxy/sni-rules` | ผู้ดูแลระบบ | ลบกฎ SNI ทั้งหมด |
| POST | `/api/v1/node-servers/:id/proxy/sni-rules/toggle` | ผู้ดูแลระบบ | สลับกฎ SNI |
| GET | `/api/v1/node-servers/:id/proxy/sni-settings` | ผู้ดูแลระบบ | การตั้งค่า SNI |
| POST | `/api/v1/node-servers/:id/proxy/sni-settings` | ผู้ดูแลระบบ | บันทึกการตั้งค่า SNI |
| POST | `/api/v1/node-servers/:id/proxy/start` | ผู้ดูแลระบบ | เริ่ม |
| GET | `/api/v1/node-servers/:id/proxy/status` | ผู้ดูแลระบบ | สถานะกระบวนการพร็อกซี |
| POST | `/api/v1/node-servers/:id/proxy/stop` | ผู้ดูแลระบบ | หยุด |
| POST | `/api/v1/node-servers/:id/proxy/sync` | ผู้ดูแลระบบ | ซิงค์การกำหนดค่าพร็อกซี |
| GET | `/api/v1/node-servers/:id/rate-limit` | ผู้ดูแลระบบ | อ่านการกำหนดค่า |
| PUT | `/api/v1/node-servers/:id/rate-limit` | ผู้ดูแลระบบ | อัปเดตการกำหนดค่า |
| GET | `/api/v1/node-servers/:id/rate-limit/status` | ผู้ดูแลระบบ | สถานะ |
| GET | `/api/v1/node-servers/:id/recover-mode` | ผู้ดูแลระบบ | อ่านโหมดการกู้คืนโหนด |
| PUT | `/api/v1/node-servers/:id/recover-mode` | ผู้ดูแลระบบ | ตั้งค่าโหมดการกู้คืนโหนด |
| POST | `/api/v1/node-servers/:id/revoke-cert` | ผู้ดูแลระบบ | เพิกถอนใบรับรองตัวแทน |
| GET | `/api/v1/node-servers/:id/services` | ผู้ดูแลระบบ | รายการ |
| POST | `/api/v1/node-servers/:id/services` | ผู้ดูแลระบบ | สร้าง |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId` | ผู้ดูแลระบบ | ลบ |
| GET | `/api/v1/node-servers/:id/services/:serviceId` | ผู้ดูแลระบบ | รายละเอียด |
| PUT | `/api/v1/node-servers/:id/services/:serviceId` | ผู้ดูแลระบบ | อัปเดต |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId/crash-logs` | ผู้ดูแลระบบ | ล้างบันทึกข้อขัดข้อง |
| GET | `/api/v1/node-servers/:id/services/:serviceId/crash-logs` | ผู้ดูแลระบบ | รายการบันทึกข้อขัดข้อง |
| GET | `/api/v1/node-servers/:id/services/:serviceId/crash-logs/:logId` | ผู้ดูแลระบบ | รายละเอียดบันทึกข้อขัดข้อง |
| GET | `/api/v1/node-servers/:id/services/:serviceId/endpoints` | ผู้ดูแลระบบ | รายการจุดสิ้นสุด |
| POST | `/api/v1/node-servers/:id/services/:serviceId/endpoints` | ผู้ดูแลระบบ | สร้างจุดสิ้นสุด |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId/endpoints/:endpointId` | ผู้ดูแลระบบ | ลบปลายทาง |
| PUT | `/api/v1/node-servers/:id/services/:serviceId/endpoints/:endpointId` | ผู้ดูแลระบบ | อัปเดตจุดสิ้นสุด |
| PUT | `/api/v1/node-servers/:id/services/:serviceId/guardian` | ผู้ดูแลระบบ | อัปเดตการกำหนดค่าหัวหน้างาน |
| GET | `/api/v1/node-servers/:id/services/:serviceId/process-status` | ผู้ดูแลระบบ | รับสถานะการประมวลผล |
| POST | `/api/v1/node-servers/:id/services/:serviceId/restart` | ผู้ดูแลระบบ | รีสตาร์ท |
| POST | `/api/v1/node-servers/:id/services/:serviceId/start` | ผู้ดูแลระบบ | เริ่ม |
| POST | `/api/v1/node-servers/:id/services/:serviceId/stop` | ผู้ดูแลระบบ | หยุด |
| GET | `/api/v1/node-servers/:id/services/process-status` | ผู้ดูแลระบบ | สถานะกระบวนการทั้งหมด |
| GET | `/api/v1/node-servers/:id/share-domains` | ผู้ดูแลระบบ | แชร์โดเมนสำหรับโหนดนี้ |
| POST | `/api/v1/node-servers/:id/share-domains` | ผู้ดูแลระบบ | สร้าง |
| DELETE | `/api/v1/node-servers/:id/share-domains/:domainId` | ผู้ดูแลระบบ | ลบ |
| PUT | `/api/v1/node-servers/:id/share-domains/:domainId` | ผู้ดูแลระบบ | อัปเดต |
| DELETE | `/api/v1/node-servers/:id/share-domains/all` | ผู้ดูแลระบบ | เคลียร์ทั้งหมด |
| POST | `/api/v1/node-servers/:id/singbox/control/firewall/apply` | ผู้ดูแลระบบ | ใช้กฎไฟร์วอลล์การข้ามพอร์ต |
| POST | `/api/v1/node-servers/:id/singbox/control/firewall/remove` | ผู้ดูแลระบบ | ลบกฎไฟร์วอลล์การข้ามพอร์ต |
| POST | `/api/v1/node-servers/:id/speed-test` | ผู้ดูแลระบบ | การทดสอบความเร็วของโหนด |
| GET | `/api/v1/node-servers/:id/status` | ผู้ดูแลระบบ | สถานะของโหนด |
| POST | `/api/v1/node-servers/:id/sync` | ผู้ดูแลระบบ | ซิงค์การกำหนดค่าโหนด (สร้างและพุช) |
| POST | `/api/v1/node-servers/:id/sync-data` | ผู้ดูแลระบบขั้นสูง | พุชข้อมูลพาเนลไปยังโหนด (ซิงค์ผู้ใช้/การสมัครสมาชิก/แผนทั้งหมด) |
| POST | `/api/v1/node-servers/:id/sync-mirror` | ผู้ดูแลระบบ | ซิงค์โหนดมิเรอร์ |
| GET | `/api/v1/node-servers/:id/tags` | ผู้ดูแลระบบ | รายการแท็ก |
| POST | `/api/v1/node-servers/:id/tags` | ผู้ดูแลระบบ | เพิ่มแท็ก |
| DELETE | `/api/v1/node-servers/:id/tags/:tagKey` | ผู้ดูแลระบบ | ลบแท็ก |
| GET | `/api/v1/node-servers/:id/traffic-history` | ผู้ดูแลระบบ | ประวัติการเข้าชม |
| POST | `/api/v1/node-servers/:id/traffic-resume` | ผู้ดูแลระบบ | คืนค่าการรับส่งข้อมูลที่ถูกระงับเนื่องจากเกินกำหนด |
| GET | `/api/v1/node-servers/:id/traffic-usage` | ผู้ดูแลระบบ | การใช้งานด้านการจราจร |
| POST | `/api/v1/node-servers/:id/update-agent` | ผู้ดูแลระบบ | อัพเกรดตัวแทน |
| GET | `/api/v1/node-servers/:id/used-server-names` | ผู้ดูแลระบบ | รายการ REALITY SNI ที่ใช้งานอยู่ |
| GET | `/api/v1/node-servers/:id/waf/config` | ผู้ดูแลระบบ | อ่านการกำหนดค่า |
| PUT | `/api/v1/node-servers/:id/waf/config` | ผู้ดูแลระบบ | บันทึกการกำหนดค่า |
| POST | `/api/v1/node-servers/:id/waf/disable-domain` | ผู้ดูแลระบบ | ปิดใช้งานการป้องกันโดเมน |
| GET | `/api/v1/node-servers/:id/waf/domain-config` | ผู้ดูแลระบบ | รายการกำหนดค่าโดเมน |
| PUT | `/api/v1/node-servers/:id/waf/domain-config` | ผู้ดูแลระบบ | บันทึกการกำหนดค่าโดเมนจำนวนมาก |
| DELETE | `/api/v1/node-servers/:id/waf/domain-config/:domain` | ผู้ดูแลระบบ | ลบการกำหนดค่าโดเมน |
| GET | `/api/v1/node-servers/:id/waf/domain-config/:domain` | ผู้ดูแลระบบ | รายละเอียดการกำหนดค่าโดเมน |
| PUT | `/api/v1/node-servers/:id/waf/domain-config/:domain` | ผู้ดูแลระบบ | บันทึกการกำหนดค่าโดเมน |
| GET | `/api/v1/node-servers/:id/waf/domain-rules` | ผู้ดูแลระบบ | ไดเร็กทอรีกฎโดเมน |
| GET | `/api/v1/node-servers/:id/waf/domain-rules/:domain/:name` | ผู้ดูแลระบบ | อ่านไฟล์กฎโดเมน |
| PUT | `/api/v1/node-servers/:id/waf/domain-rules/:domain/:name` | ผู้ดูแลระบบ | บันทึกไฟล์กฎโดเมน |
| GET | `/api/v1/node-servers/:id/waf/domain-status` | ผู้ดูแลระบบ | สถานะการป้องกันโดเมน |
| POST | `/api/v1/node-servers/:id/waf/enable-domain` | ผู้ดูแลระบบ | เปิดใช้งานการป้องกันโดเมน |
| POST | `/api/v1/node-servers/:id/waf/push` | ผู้ดูแลระบบ | ดันไปที่โหนดนี้ |
| GET | `/api/v1/node-servers/:id/waf/rules` | ผู้ดูแลระบบ | รายการกฎของโหนด WAF |
| DELETE | `/api/v1/node-servers/:id/waf/rules/:name` | ผู้ดูแลระบบ | ลบกฎโหนด WAF |
| GET | `/api/v1/node-servers/:id/waf/rules/:name` | ผู้ดูแลระบบ | รายละเอียดกฎของโหนด WAF |
| PUT | `/api/v1/node-servers/:id/waf/rules/:name` | ผู้ดูแลระบบ | บันทึกกฎโหนด WAF |
| GET | `/api/v1/node-servers/:id/waf/status` | ผู้ดูแลระบบ | สถานะโหนด WAF |
| PUT | `/api/v1/node-servers/:id/waf/status` | ผู้ดูแลระบบ | สลับโหนด WAF |
| POST | `/api/v1/node-servers/:id/waf/sync` | ผู้ดูแลระบบ | ซิงค์กับโหนดนี้ |
| GET | `/api/v1/node-servers/:id/warp` | ผู้ดูแลระบบ | รายการ |
| DELETE | `/api/v1/node-servers/:id/warp/:warpId` | ผู้ดูแลระบบ | ลบ |
| GET | `/api/v1/node-servers/:id/warp/config` | ผู้ดูแลระบบ | อ่านการกำหนดค่า |
| POST | `/api/v1/node-servers/:id/warp/generate-config` | ผู้ดูแลระบบ | สร้างการกำหนดค่า |
| POST | `/api/v1/node-servers/:id/warp/register` | ผู้ดูแลระบบ | ลงทะเบียน |
| GET | `/api/v1/node-servers/:id/warp/status` | ผู้ดูแลระบบ | สถานะ |
| GET | `/api/v1/node-servers/:id/xray/control/balancer/:tag` | ผู้ดูแลระบบ | รับข้อมูลบาลานเซอร์ |
| POST | `/api/v1/node-servers/:id/xray/control/balancer/:tag` | ผู้ดูแลระบบ | ตั้งเป้าหมายบาลานเซอร์ |
| POST | `/api/v1/node-servers/:id/xray/control/firewall/apply` | ผู้ดูแลระบบ | ใช้กฎไฟร์วอลล์การข้ามพอร์ต |
| POST | `/api/v1/node-servers/:id/xray/control/firewall/remove` | ผู้ดูแลระบบ | ลบกฎไฟร์วอลล์การข้ามพอร์ต |
| POST | `/api/v1/node-servers/:id/xray/control/inbound` | ผู้ดูแลระบบ | เพิ่มขาเข้า |
| DELETE | `/api/v1/node-servers/:id/xray/control/inbound/:tag` | ผู้ดูแลระบบ | ลบขาเข้า |
| POST | `/api/v1/node-servers/:id/xray/control/inbound/:tag/user` | ผู้ดูแลระบบ | เพิ่มผู้ใช้ |
| DELETE | `/api/v1/node-servers/:id/xray/control/inbound/:tag/user/:email` | ผู้ดูแลระบบ | ลบผู้ใช้ |
| POST | `/api/v1/node-servers/:id/xray/control/outbound` | ผู้ดูแลระบบ | เพิ่มขาออก |
| DELETE | `/api/v1/node-servers/:id/xray/control/outbound/:tag` | ผู้ดูแลระบบ | ลบขาออก |
| POST | `/api/v1/node-servers/:id/xray/control/routing` | ผู้ดูแลระบบ | ขอเส้นทาง |
| POST | `/api/v1/node-servers/:id/xray/control/routing/rule` | ผู้ดูแลระบบ | เพิ่มกฎการกำหนดเส้นทาง |
| DELETE | `/api/v1/node-servers/:id/xray/control/routing/rule/:tag` | ผู้ดูแลระบบ | ลบกฎการกำหนดเส้นทาง |
| GET | `/api/v1/node-servers/:id/xray/control/routing/rules` | ผู้ดูแลระบบ | แสดงรายการกฎการกำหนดเส้นทาง |
| POST | `/api/v1/node-servers/:id/xray/control/sync` | ผู้ดูแลระบบ | ทริกเกอร์การซิงค์การกำหนดค่า xray ด้วยตนเอง |
| GET | `/api/v1/node-servers/:id/xray/stats` | ผู้ดูแลระบบ | รับสถิติ |
| POST | `/api/v1/node-servers/:id/xray/stats/reset/:email` | ผู้ดูแลระบบ | รีเซ็ตสถิติผู้ใช้ |
| GET | `/api/v1/node-servers/:id/xray/stats/users` | ผู้ดูแลระบบ | รับสถิติผู้ใช้ |
| GET | `/api/v1/node-servers/billable` | ผู้ดูแลระบบ | รายการโหนดที่เรียกเก็บเงินได้ |
| GET | `/api/v1/node-servers/ca-cert` | ผู้ดูแลระบบ | ใบรับรองแคลิฟอร์เนีย |
| GET | `/api/v1/node-servers/ca-status` | ผู้ดูแลระบบ | สถานะแคลิฟอร์เนีย |
| GET | `/api/v1/node-servers/dashboard` | ผู้ดูแลระบบ | สถานะแดชบอร์ด |
| POST | `/api/v1/node-servers/distribute-cert` | ผู้ดูแลระบบ | แจกจ่ายใบรับรองไปยังโหนดทั้งหมด |
| POST | `/api/v1/node-servers/failover-check` | ผู้ดูแลระบบ | ทริกเกอร์การตรวจสอบการเฟลโอเวอร์ |
| GET | `/api/v1/node-servers/metrics/aggregated` | ผู้ดูแลระบบ | ตัวชี้วัดรวม |
| POST | `/api/v1/node-servers/migrate-cert` | ผู้ดูแลระบบ | ย้ายใบรับรอง |
| GET | `/api/v1/node-servers/online-summary` | ผู้ดูแลระบบ | สรุปออนไลน์ |
| GET | `/api/v1/node-servers/online-users` | ผู้ดูแลระบบ | ผู้ใช้ออนไลน์ทุกคน |
| GET | `/api/v1/node-servers/pending-agent-updates` | ผู้ดูแลระบบ | ตัวแทนที่รอการอัปเกรด |
| POST | `/api/v1/node-servers/recommend-region` | ผู้ดูแลระบบ | ภูมิภาคที่แนะนำ |
| POST | `/api/v1/node-servers/select-nodes` | ผู้ดูแลระบบ | เลือกโหนดสำหรับการสมัครสมาชิก |
| POST | `/api/v1/node-servers/speed-test-all` | ผู้ดูแลระบบ | ทดสอบความเร็วทุกโหนด |
| GET | `/api/v1/node-servers/speed-test-results` | ผู้ดูแลระบบ | ผลการทดสอบความเร็ว |
| GET | `/api/v1/node-servers/traffic/stats` | ผู้ดูแลระบบ | สถิติการเข้าชม |
| POST | `/api/v1/node-servers/update-all-agents` | ผู้ดูแลระบบ | อัพเกรดตัวแทนทั้งหมด |
| DELETE | `/api/v1/node-servers/users/:client_id` | ผู้ดูแลระบบ | ซิงค์-ลบลูกค้า |
| GET | `/api/v1/node-servers/users/retry-queue` | ผู้ดูแลระบบ | ลองสถานะคิวอีกครั้ง |
| POST | `/api/v1/node-servers/users/retry-queue/process` | ผู้ดูแลระบบ | ดำเนินการคิวการลองใหม่ |
| POST | `/api/v1/node-servers/users/sync` | ผู้ดูแลระบบ | ซิงค์ลูกค้ารายเดียว |
| POST | `/api/v1/node-servers/validate-chain-config` | ผู้ดูแลระบบ | ตรวจสอบการกำหนดค่าแบบลูกโซ่ |

### บันทึกการตรวจสอบโหนด

การตรวจสอบและส่งออกการดำเนินการฝั่งโหนด

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| GET | `/api/v1/node-audit-logs` | ลงชื่อเข้าใช้แล้ว | บันทึกการตรวจสอบโหนด |
| GET | `/api/v1/node-audit-logs/export` | ลงชื่อเข้าใช้แล้ว | ส่งออกบันทึกการตรวจสอบโหนด |

### โหนด (โมเดลดั้งเดิม เลิกใช้แล้ว)

> `/api/v1/nodes` CRUD ถูกยกเลิกในวันที่ 20-10-2569: โหนดได้รับการจัดการอย่างสม่ำเสมอผ่าน `/api/v1/node-servers`
> ข้อความค้นหาของผู้ใช้ออนไลน์ยังคงอยู่ที่ `/api/v1/node-servers/{online-users,online-summary,:id/online-users}`

### ภาพรวมภูมิภาค

มุมมองโหนดที่รวบรวมตามภูมิภาค

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| GET | `/api/v1/regions/:region/servers` | ลงชื่อเข้าใช้แล้ว | รายการโหนดสำหรับภูมิภาคนี้ |
| GET | `/api/v1/regions/overview` | ลงชื่อเข้าใช้แล้ว | ภาพรวมภูมิภาค |

### การสมัครรับข้อมูล

การจัดการบันทึกการสมัครสมาชิก: เพิกถอน, หมุนเวียนโทเค็น, บันทึกการเข้าถึง, รีเซ็ตการใช้งาน

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| GET | `/api/v1/subscriptions` | ลงชื่อเข้าใช้แล้ว | รายการ |
| POST | `/api/v1/subscriptions` | ลงชื่อเข้าใช้แล้ว | สร้าง |
| DELETE | `/api/v1/subscriptions/:id` | ลงชื่อเข้าใช้แล้ว | ลบ |
| GET | `/api/v1/subscriptions/:id` | ลงชื่อเข้าใช้แล้ว | รายละเอียด |
| PUT | `/api/v1/subscriptions/:id` | ลงชื่อเข้าใช้แล้ว | อัปเดต |
| GET | `/api/v1/subscriptions/:id/access-logs` | ลงชื่อเข้าใช้แล้ว | บันทึกการเข้าถึง |
| POST | `/api/v1/subscriptions/:id/enable` | ลงชื่อเข้าใช้แล้ว | เปิดใช้งาน |
| POST | `/api/v1/subscriptions/:id/reset-usage` | ลงชื่อเข้าใช้แล้ว | รีเซ็ตการใช้งาน |
| POST | `/api/v1/subscriptions/:id/restore` | ลงชื่อเข้าใช้แล้ว | ดำเนินการต่อ |
| POST | `/api/v1/subscriptions/:id/revoke` | ลงชื่อเข้าใช้แล้ว | เพิกถอน |
| POST | `/api/v1/subscriptions/:id/rotate` | ลงชื่อเข้าใช้แล้ว | หมุน |
| GET | `/api/v1/subscriptions/:id/rotations` | ลงชื่อเข้าใช้แล้ว | ประวัติการหมุน |

### รูปแบบการสมัครสมาชิก

> ฝั่งผู้ดูแลระบบ `/api/v1/subscription-formats` CRUD ถูกยกเลิกในวันที่ 2026-10-20: จุดเริ่มต้นที่แท้จริงคือ
> `/api/v1/mihomo/configs` (ศูนย์รูปแบบการสมัครสมาชิก) ตาราง `subscription_formats` ยังคงอยู่
> ใช้เป็นข้อมูลเริ่มต้นสำหรับปลายทาง `/api/v1/public/client-configs` สาธารณะเท่านั้น

### จุดสิ้นสุดการสมัครสมาชิก

โดเมนการสมัครสมาชิกเฉพาะ (พร็อกซีแบบย้อนกลับไปยังเส้นทาง `/sub` ของแผงควบคุม)

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| GET | `/api/v1/subscription-endpoints` | ลงชื่อเข้าใช้แล้ว | รายการ |
| POST | `/api/v1/subscription-endpoints` | ลงชื่อเข้าใช้แล้ว | สร้าง |
| DELETE | `/api/v1/subscription-endpoints/:id` | ลงชื่อเข้าใช้แล้ว | ลบ |
| GET | `/api/v1/subscription-endpoints/:id` | ลงชื่อเข้าใช้แล้ว | รายละเอียด |
| PUT | `/api/v1/subscription-endpoints/:id` | ลงชื่อเข้าใช้แล้ว | อัปเดต |

### แบ่งปันโดเมน

เข้าถึงโดเมน/พูล IP ที่ใช้ในลิงก์ไคลเอ็นต์

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| GET | `/api/v1/share-domains` | ลงชื่อเข้าใช้แล้ว | รายการ |
| POST | `/api/v1/share-domains` | ลงชื่อเข้าใช้แล้ว | สร้าง |
| DELETE | `/api/v1/share-domains/:id` | ลงชื่อเข้าใช้แล้ว | ลบ |
| PUT | `/api/v1/share-domains/:id` | ลงชื่อเข้าใช้แล้ว | อัปเดต |
| DELETE | `/api/v1/share-domains/all` | ลงชื่อเข้าใช้แล้ว | เคลียร์ทั้งหมด |
| POST | `/api/v1/share-domains/seed` | ลงชื่อเข้าใช้แล้ว | เริ่มต้นข้อมูลเริ่มต้น |

### Cloudflare ไอพี

Cloudflare CDN IP พูลและการจัดการแหล่งที่มา

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| DELETE | `/api/v1/cfip/entries` | ลงชื่อเข้าใช้แล้ว | ล้างรายการ |
| GET | `/api/v1/cfip/entries` | ลงชื่อเข้าใช้แล้ว | รายการเข้า |
| POST | `/api/v1/cfip/entries` | ลงชื่อเข้าใช้แล้ว | เพิ่มรายการ |
| DELETE | `/api/v1/cfip/entries/:id` | ลงชื่อเข้าใช้แล้ว | ลบรายการ |
| POST | `/api/v1/cfip/fetch-all` | ลงชื่อเข้าใช้แล้ว | ดึงข้อมูลแหล่งที่มาทั้งหมด |
| POST | `/api/v1/cfip/seed` | ลงชื่อเข้าใช้แล้ว | เริ่มต้นข้อมูลเริ่มต้น |
| GET | `/api/v1/cfip/sources` | ลงชื่อเข้าใช้แล้ว | รายการแหล่งข้อมูล |
| POST | `/api/v1/cfip/sources` | ลงชื่อเข้าใช้แล้ว | สร้างแหล่งข้อมูล |
| DELETE | `/api/v1/cfip/sources/:id` | ลงชื่อเข้าใช้แล้ว | ลบแหล่งข้อมูล |
| PUT | `/api/v1/cfip/sources/:id` | ลงชื่อเข้าใช้แล้ว | อัปเดตแหล่งข้อมูล |
| POST | `/api/v1/cfip/sources/:id/fetch` | ลงชื่อเข้าใช้แล้ว | ดึงแหล่งที่มานี้ |

### การกำหนดค่าการสมัครสมาชิก (Mihomo/Clash)

การกำหนดค่าเทมเพลตการสมัครสมาชิก: กลุ่ม กฎ ผู้ให้บริการกฎ การแสดงตัวอย่าง YAML

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| GET | `/api/v1/mihomo/configs` | ลงชื่อเข้าใช้แล้ว | รายการ |
| POST | `/api/v1/mihomo/configs` | ลงชื่อเข้าใช้แล้ว | สร้าง |
| DELETE | `/api/v1/mihomo/configs/:id` | ลงชื่อเข้าใช้แล้ว | ลบ |
| GET | `/api/v1/mihomo/configs/:id` | ลงชื่อเข้าใช้แล้ว | รายละเอียด |
| PUT | `/api/v1/mihomo/configs/:id` | ลงชื่อเข้าใช้แล้ว | อัปเดต |
| GET | `/api/v1/mihomo/configs/:id/proxy-groups` | ลงชื่อเข้าใช้แล้ว | รายการกลุ่ม |
| POST | `/api/v1/mihomo/configs/:id/proxy-groups` | ลงชื่อเข้าใช้แล้ว | สร้างกลุ่ม |
| GET | `/api/v1/mihomo/configs/:id/rule-providers` | ลงชื่อเข้าใช้แล้ว | รายชื่อผู้ให้บริการกฎ |
| POST | `/api/v1/mihomo/configs/:id/rule-providers` | ลงชื่อเข้าใช้แล้ว | สร้างผู้ให้บริการกฎ |
| GET | `/api/v1/mihomo/configs/:id/rules` | ลงชื่อเข้าใช้แล้ว | รายการกฎ |
| POST | `/api/v1/mihomo/configs/:id/rules` | ลงชื่อเข้าใช้แล้ว | สร้างกฎ |
| GET | `/api/v1/mihomo/configs/:id/yaml` | ลงชื่อเข้าใช้แล้ว | แสดงตัวอย่าง YAML ที่สร้างขึ้น |
| DELETE | `/api/v1/mihomo/proxy-groups/:id` | ลงชื่อเข้าใช้แล้ว | ลบกลุ่ม |
| DELETE | `/api/v1/mihomo/rule-providers/:id` | ลงชื่อเข้าใช้แล้ว | ลบผู้ให้บริการกฎ |
| DELETE | `/api/v1/mihomo/rules/:id` | ลงชื่อเข้าใช้แล้ว | ลบกฎ |

### เว็บไซต์เติมเงิน

Third-party payment site integration: signing keys, verification files, domain verification, integration docs.

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| GET | `/api/v1/charge-sites` | ลงชื่อเข้าใช้แล้ว | รายการไซต์ |
| POST | `/api/v1/charge-sites` | ลงชื่อเข้าใช้แล้ว | สร้างไซต์ |
| DELETE | `/api/v1/charge-sites/:id` | ลงชื่อเข้าใช้แล้ว | ลบไซต์ |
| GET | `/api/v1/charge-sites/:id` | ลงชื่อเข้าใช้แล้ว | รายละเอียดเว็บไซต์ |
| PUT | `/api/v1/charge-sites/:id` | ลงชื่อเข้าใช้แล้ว | อัปเดตไซต์ |
| POST | `/api/v1/charge-sites/:id/clear-prev-secret` | ลงชื่อเข้าใช้แล้ว | ล้างกุญแจเก่า |
| POST | `/api/v1/charge-sites/:id/generate-verify` | ลงชื่อเข้าใช้แล้ว | สร้างไฟล์การยืนยัน |
| POST | `/api/v1/charge-sites/:id/generate-verify-file` | ลงชื่อเข้าใช้แล้ว | สร้างไฟล์การยืนยัน |
| GET | `/api/v1/charge-sites/:id/integration-doc` | ลงชื่อเข้าใช้แล้ว | รับเอกสารบูรณาการ |
| POST | `/api/v1/charge-sites/:id/rotate-secret` | ลงชื่อเข้าใช้แล้ว | หมุนคีย์การลงนาม |
| POST | `/api/v1/charge-sites/:id/verify-dns` | ลงชื่อเข้าใช้แล้ว | ตรวจสอบโดเมน DNS |
| POST | `/api/v1/charge-sites/:id/verify-domain` | ลงชื่อเข้าใช้แล้ว | ยืนยันความท้าทายโดเมน HTTP |
| POST | `/api/v1/charge-sites/:id/verify-domain-dns` | ลงชื่อเข้าใช้แล้ว | ตรวจสอบโดเมน DNS |
| POST | `/api/v1/charge-sites/:id/verify-domain-http` | ลงชื่อเข้าใช้แล้ว | ยืนยันความท้าทายของโดเมน HTTP |

### สั่งเติมเงิน

สอบถามการสั่งซื้อเติมเงิน การคืนเงิน การส่งออก CSV

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| GET | `/api/v1/charge-orders` | ลงชื่อเข้าใช้แล้ว | รายการสั่งซื้อ |
| GET | `/api/v1/charge-orders/:id` | ลงชื่อเข้าใช้แล้ว | รายละเอียดการสั่งซื้อ |
| POST | `/api/v1/charge-orders/:id/refund` | ลงชื่อเข้าใช้แล้ว | การคืนเงิน |
| GET | `/api/v1/charge-orders/export-csv` | ลงชื่อเข้าใช้แล้ว | ส่งออกคำสั่งเติมเงิน CSV |

### บัตรเติมเงิน

การสร้างการ์ดจำนวนมาก บันทึกการใช้งาน การส่งออก CSV

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| GET | `/api/v1/charge-cards` | ลงชื่อเข้าใช้แล้ว | รายการการ์ด |
| POST | `/api/v1/charge-cards` | ลงชื่อเข้าใช้แล้ว | สร้างการ์ด |
| DELETE | `/api/v1/charge-cards/:id` | ลงชื่อเข้าใช้แล้ว | ลบการ์ด |
| GET | `/api/v1/charge-cards/:id/usages` | ลงชื่อเข้าใช้แล้ว | บันทึกการใช้บัตร |
| POST | `/api/v1/charge-cards/batch` | ลงชื่อเข้าใช้แล้ว | สร้างการ์ดจำนวนมาก |
| GET | `/api/v1/charge-cards/export-csv` | ลงชื่อเข้าใช้แล้ว | ส่งออกการ์ด CSV |

### ชาร์จกิจกรรมการโทรกลับ

การตรวจสอบเหตุการณ์การโทรกลับ

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| GET | `/api/v1/charge-callback-events` | ลงชื่อเข้าใช้แล้ว | รายการเหตุการณ์การโทรกลับ |

### ลองโทรกลับเรียกเก็บเงินอีกครั้ง

ลองโทรกลับล้มเหลวอีกครั้งด้วยตนเอง

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| GET | `/api/v1/charge-callback-retries` | ลงชื่อเข้าใช้แล้ว | รายการโทรกลับล้มเหลว |
| POST | `/api/v1/charge-callback-retries/:id/retry` | ผู้ดูแลระบบ | ลองโทรกลับอีกครั้ง |

### ยอดลูกค้า

มุมมองผู้ดูแลระบบของยอดคงเหลือและธุรกรรม

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| GET | `/api/v1/client-balances` | ลงชื่อเข้าใช้แล้ว | รายการยอดคงเหลือของลูกค้า |
| GET | `/api/v1/client-balances/transactions` | ลงชื่อเข้าใช้แล้ว | ธุรกรรมยอดคงเหลือ (ผู้ดูแลระบบ) |
| GET | `/api/v1/client-balances/transactions/export-csv` | ลงชื่อเข้าใช้แล้ว | ส่งออกธุรกรรมยอดดุล CSV |

### ของขวัญ

ออกของขวัญที่เรียกร้องได้ (วัน/การจราจร)

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| GET | `/api/v1/client-gifts` | ลงชื่อเข้าใช้แล้ว | รายการของขวัญ |
| POST | `/api/v1/client-gifts` | ลงชื่อเข้าใช้แล้ว | สร้างของขวัญ |
| POST | `/api/v1/client-gifts/batch` | ลงชื่อเข้าใช้แล้ว | จำนวนมากสร้างของขวัญ |
| GET | `/api/v1/client-gifts/search-clients` | ลงชื่อเข้าใช้แล้ว | ค้นหาลูกค้า |

### การกำหนดค่าการอ้างอิง

การกำหนดค่าคอมมิชชั่นผู้อ้างอิงและสถิติ

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| GET | `/api/v1/referral/config` | ลงชื่อเข้าใช้แล้ว | การกำหนดค่าการอ้างอิง |
| PUT | `/api/v1/referral/config` | ผู้ดูแลระบบ | บันทึกการกำหนดค่าการอ้างอิง |
| GET | `/api/v1/referral/records` | ลงชื่อเข้าใช้แล้ว | รายการบันทึกการอ้างอิง |
| GET | `/api/v1/referral/stats` | ลงชื่อเข้าใช้แล้ว | สถิติการอ้างอิง |

### การถอนการอ้างอิง

ตรวจสอบคำขอถอนเงิน

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| GET | `/api/v1/referral-withdrawals` | ลงชื่อเข้าใช้แล้ว | รายการขอถอนเงิน |
| POST | `/api/v1/referral-withdrawals/:id/approve` | ลงชื่อเข้าใช้แล้ว | อนุมัติการถอนเงิน |
| POST | `/api/v1/referral-withdrawals/:id/reject` | ลงชื่อเข้าใช้แล้ว | ปฏิเสธการถอนเงิน |

### รายการอนุญาต IP (สรุป)

มุมมองรายการที่อนุญาต IP สำหรับลูกค้าทั้งหมด

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| GET | `/api/v1/ip-whitelists` | ลงชื่อเข้าใช้แล้ว | รายการอนุญาต IP ของลูกค้าทั้งหมด |

### การเชื่อมโยง OAuth ของลูกค้า

สอบถามการผูกบัญชีบุคคลที่สามของลูกค้า

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| GET | `/api/v1/client-oauth-bindings` | ลงชื่อเข้าใช้แล้ว | รายการผูก OAuth ของลูกค้า |

### ตั๋ว (ผู้ดูแลระบบ)

การตอบกลับตั๋ว การมอบหมาย และการปิด

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| GET | `/api/v1/tickets` | ลงชื่อเข้าใช้แล้ว | รายการตั๋ว |
| GET | `/api/v1/tickets/:id` | ลงชื่อเข้าใช้แล้ว | รายละเอียดตั๋ว |
| POST | `/api/v1/tickets/:id/assign` | ลงชื่อเข้าใช้แล้ว | กำหนดตั๋ว |
| POST | `/api/v1/tickets/:id/close` | ลงชื่อเข้าใช้แล้ว | ปิดตั๋ว |
| POST | `/api/v1/tickets/:id/reply` | ลงชื่อเข้าใช้แล้ว | ตอบเรื่องตั๋วครับ |

### บทความ

CRUD สำหรับบทความพอร์ทัล

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| GET | `/api/v1/articles` | ลงชื่อเข้าใช้แล้ว | รายการบทความ |
| POST | `/api/v1/articles` | ลงชื่อเข้าใช้แล้ว | สร้างบทความ |
| DELETE | `/api/v1/articles/:id` | ลงชื่อเข้าใช้แล้ว | ลบบทความ |
| GET | `/api/v1/articles/:id` | ลงชื่อเข้าใช้แล้ว | รายละเอียดบทความ |
| PUT | `/api/v1/articles/:id` | ลงชื่อเข้าใช้แล้ว | อัปเดตบทความ |

### บัญชีที่ใช้ร่วมกัน

บันทึกการออกและการซ่อมแซมสำหรับบัญชีที่ใช้ร่วมกัน (สตรีมมิ่ง ฯลฯ)

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| GET | `/api/v1/shared-accounts` | ลงชื่อเข้าใช้แล้ว | รายการบัญชีที่ใช้ร่วมกัน |
| POST | `/api/v1/shared-accounts` | ลงชื่อเข้าใช้แล้ว | สร้างบัญชีที่ใช้ร่วมกัน |
| DELETE | `/api/v1/shared-accounts/:id` | ลงชื่อเข้าใช้แล้ว | ลบบัญชีที่ใช้ร่วมกัน |
| GET | `/api/v1/shared-accounts/:id` | ลงชื่อเข้าใช้แล้ว | รายละเอียดบัญชีที่ใช้ร่วมกัน |
| PUT | `/api/v1/shared-accounts/:id` | ลงชื่อเข้าใช้แล้ว | อัปเดตบัญชีที่ใช้ร่วมกัน |
| GET | `/api/v1/shared-accounts/:id/repair-logs` | ลงชื่อเข้าใช้แล้ว | บันทึกการซ่อมแซม |

### ใบรับรอง

การออกใบรับรองสาธารณะ (ACME) อัปโหลด ต่ออายุ และส่งออก รองรับการออกโหนดเฉพาะ

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| GET | `/api/v1/certificates` | ลงชื่อเข้าใช้แล้ว | รายการ |
| POST | `/api/v1/certificates` | ลงชื่อเข้าใช้แล้ว | สร้าง |
| DELETE | `/api/v1/certificates/:id` | ลงชื่อเข้าใช้แล้ว | ลบ |
| GET | `/api/v1/certificates/:id` | ลงชื่อเข้าใช้แล้ว | รายละเอียด |
| PUT | `/api/v1/certificates/:id` | ลงชื่อเข้าใช้แล้ว | อัปเดต |
| GET | `/api/v1/certificates/:id/download` | ลงชื่อเข้าใช้แล้ว | ดาวน์โหลด |
| POST | `/api/v1/certificates/:id/renew` | ลงชื่อเข้าใช้แล้ว | ต่ออายุลูกค้า |
| GET | `/api/v1/certificates/check-port` | ลงชื่อเข้าใช้แล้ว | ตรวจสอบพอร์ต |
| GET | `/api/v1/certificates/detect-ip` | ลงชื่อเข้าใช้แล้ว | โพรบ IP |
| GET | `/api/v1/certificates/export` | ลงชื่อเข้าใช้แล้ว | ส่งออก |
| POST | `/api/v1/certificates/issue` | ลงชื่อเข้าใช้แล้ว | ปัญหา ร่างกาย: `{"domains":["example.com","*.example.com"],"challenge_type":"dns-01","dns_channel_id":2,"key_type":"rsa","key_bits":4096}`. `challenge_type` มีค่าเริ่มต้นเป็น http-01; `dns_channel_id` เลือกช่อง DNS (ผู้ให้บริการ DNS ที่กำหนดค่าไว้ + ข้อมูลรับรอง) |
| POST | `/api/v1/certificates/issue-on-node` | ลงชื่อเข้าใช้แล้ว | ออกไปยังโหนด |
| POST | `/api/v1/certificates/upload` | ลงชื่อเข้าใช้แล้ว | อัปโหลด |

### ช่องทางการรับรอง

การกำหนดค่าช่อง ACME (CA)

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| GET | `/api/v1/cert-channels` | ลงชื่อเข้าใช้แล้ว | รายการ |
| POST | `/api/v1/cert-channels` | ลงชื่อเข้าใช้แล้ว | สร้าง |
| DELETE | `/api/v1/cert-channels/:id` | ลงชื่อเข้าใช้แล้ว | ลบ |
| PUT | `/api/v1/cert-channels/:id` | ลงชื่อเข้าใช้แล้ว | อัปเดต |
| GET | `/api/v1/cert-channels/providers` | ลงชื่อเข้าใช้แล้ว | ผู้ให้บริการ |

### การกำหนดค่า EAB

ข้อมูลรับรองที่ผูกบัญชีภายนอก เช่น ZeroSSL / Google

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| GET | `/api/v1/eab-configs` | ลงชื่อเข้าใช้แล้ว | รายการ |
| POST | `/api/v1/eab-configs` | ลงชื่อเข้าใช้แล้ว | สร้าง |
| DELETE | `/api/v1/eab-configs/:id` | ลงชื่อเข้าใช้แล้ว | ลบ |
| PUT | `/api/v1/eab-configs/:id` | ลงชื่อเข้าใช้แล้ว | อัปเดต |
| POST | `/api/v1/eab-configs/fetch-google` | ลงชื่อเข้าใช้แล้ว | ดึงข้อมูล Google EAB จาก GCP |
| POST | `/api/v1/eab-configs/fetch-zerossl` | ลงชื่อเข้าใช้แล้ว | ดึงข้อมูล ZeroSSL EAB ด้วยอีเมลได้ในคลิกเดียว |

### บัญชี ACME

การจัดการบัญชี ACME ฝั่งแผง (อีเมลติดต่อ)

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| GET | `/api/v1/acme-accounts` | ลงชื่อเข้าใช้แล้ว | รายการ |
| POST | `/api/v1/acme-accounts` | ลงชื่อเข้าใช้แล้ว | สร้าง |
| DELETE | `/api/v1/acme-accounts/:id` | ลงชื่อเข้าใช้แล้ว | ลบ |
| PUT | `/api/v1/acme-accounts/:id` | ลงชื่อเข้าใช้แล้ว | อัปเดต |
| GET | `/api/v1/acme-accounts/default` | ลงชื่อเข้าใช้แล้ว | รับค่าเริ่มต้น |

### CA ในตัว

CA แผงภายใน: การออก/ต่ออายุ/เพิกถอนใบรับรองตัวแทน, การส่งออก CRL, คิวการแจกจ่าย

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| GET | `/api/v1/ca/agent-certs` | ลงชื่อเข้าใช้แล้ว | รายการใบรับรองตัวแทน |
| POST | `/api/v1/ca/agent-certs/:server_id/renew` | ลงชื่อเข้าใช้แล้ว | ต่ออายุใบรับรองตัวแทน |
| GET | `/api/v1/ca/distribution-queue` | ลงชื่อเข้าใช้แล้ว | คิวการแจกใบรับรอง |
| DELETE | `/api/v1/ca/distribution-queue/:id` | ลงชื่อเข้าใช้แล้ว | ลบรายการคิวการแจกจ่าย |
| POST | `/api/v1/ca/distribution-queue/:id/retry` | ลงชื่อเข้าใช้แล้ว | ลองแจกจ่ายใบรับรองอีกครั้ง |
| GET | `/api/v1/ca/revocations` | ลงชื่อเข้าใช้แล้ว | รายการเพิกถอน |
| GET | `/api/v1/ca/revocations/export` | ลงชื่อเข้าใช้แล้ว | ส่งออก CRL |
| GET | `/api/v1/ca/status` | ลงชื่อเข้าใช้แล้ว | สถานะแคลิฟอร์เนีย |

### กฎทางภูมิศาสตร์

การซิงค์และการส่งมอบแหล่งข้อมูลและไฟล์ geoip/geosite

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| GET | `/api/v1/geo-rules/files` | ลงชื่อเข้าใช้แล้ว | รายการไฟล์ |
| DELETE | `/api/v1/geo-rules/files/:fileId` | ลงชื่อเข้าใช้แล้ว | ลบไฟล์ |
| PUT | `/api/v1/geo-rules/files/:fileId/auto-sync` | ลงชื่อเข้าใช้แล้ว | สลับการซิงค์อัตโนมัติ |
| POST | `/api/v1/geo-rules/files/:fileId/download` | ลงชื่อเข้าใช้แล้ว | ดาวน์โหลด |
| GET | `/api/v1/geo-rules/files/:fileId/download-file` | ลงชื่อเข้าใช้แล้ว | ดาวน์โหลดไฟล์ |
| GET | `/api/v1/geo-rules/github-assets` | ลงชื่อเข้าใช้แล้ว | ดึงข้อมูลเนื้อหาที่เก็บ GitHub |
| POST | `/api/v1/geo-rules/push-all` | ลงชื่อเข้าใช้แล้ว | ดันไปที่โหนดทั้งหมด |
| POST | `/api/v1/geo-rules/seed` | ลงชื่อเข้าใช้แล้ว | เริ่มต้นข้อมูลเริ่มต้น |
| GET | `/api/v1/geo-rules/sources` | ลงชื่อเข้าใช้แล้ว | รายการแหล่งข้อมูล |
| POST | `/api/v1/geo-rules/sources` | ลงชื่อเข้าใช้แล้ว | สร้างแหล่งข้อมูล |
| DELETE | `/api/v1/geo-rules/sources/:sourceId` | ลงชื่อเข้าใช้แล้ว | ลบแหล่งข้อมูล |
| PUT | `/api/v1/geo-rules/sources/:sourceId` | ลงชื่อเข้าใช้แล้ว | อัปเดตแหล่งข้อมูล |
| GET | `/api/v1/geo-rules/sources/:sourceId/assets` | ลงชื่อเข้าใช้แล้ว | ดึงเนื้อหาที่เผยแพร่ |
| POST | `/api/v1/geo-rules/sources/:sourceId/files` | ลงชื่อเข้าใช้แล้ว | เพิ่มไฟล์ |
| PUT | `/api/v1/geo-rules/sources/:sourceId/schedule` | ลงชื่อเข้าใช้แล้ว | สลับการซิงค์ตามกำหนดเวลา |
| POST | `/api/v1/geo-rules/sources/:sourceId/select` | ลงชื่อเข้าใช้แล้ว | เลือกไฟล์ |
| POST | `/api/v1/geo-rules/sources/:sourceId/sync` | ลงชื่อเข้าใช้แล้ว | ซิงค์แหล่งข้อมูลนี้ |
| GET | `/api/v1/geo-rules/status` | ลงชื่อเข้าใช้แล้ว | สถานะ |
| POST | `/api/v1/geo-rules/sync-all` | ลงชื่อเข้าใช้แล้ว | ซิงค์ทั้งหมด |
| GET | `/api/v1/geo-rules/sync-status/:key` | ลงชื่อเข้าใช้แล้ว | ความคืบหน้าการซิงค์ |

### ไลบรารีกฎ WAF

กฎ WAF สากล: แหล่งที่มาของ GitHub, ซิงค์, จัดส่งไปยังโหนดทั้งหมด

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| GET | `/api/v1/waf/github-source` | ลงชื่อเข้าใช้แล้ว | แหล่งข้อมูล GitHub |
| PUT | `/api/v1/waf/github-source` | ลงชื่อเข้าใช้แล้ว | บันทึกแหล่งข้อมูล GitHub |
| POST | `/api/v1/waf/push-all` | ลงชื่อเข้าใช้แล้ว | ดันไปที่โหนดทั้งหมด |
| GET | `/api/v1/waf/rules` | ลงชื่อเข้าใช้แล้ว | รายการกฎ |
| DELETE | `/api/v1/waf/rules/:name` | ลงชื่อเข้าใช้แล้ว | ลบกฎ |
| GET | `/api/v1/waf/rules/:name` | ลงชื่อเข้าใช้แล้ว | รับกฎ |
| PUT | `/api/v1/waf/rules/:name` | ลงชื่อเข้าใช้แล้ว | บันทึกกฎ |
| POST | `/api/v1/waf/sync-all` | ลงชื่อเข้าใช้แล้ว | ซิงค์ทั้งหมด |
| POST | `/api/v1/waf/sync-github` | ลงชื่อเข้าใช้แล้ว | ซิงค์จาก GitHub |

### การจัดการบันทึก

การอ่านและการสตรีมบันทึกโหนด/คอร์ นโยบายการล้างข้อมูล การควบคุมผู้เขียนบันทึก

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| POST | `/api/v1/log-manage/cleanup-all` | ลงชื่อเข้าใช้แล้ว | ทำความสะอาดทุกอย่างตอนนี้ |
| GET | `/api/v1/log-manage/cleanup-rules` | ลงชื่อเข้าใช้แล้ว | รายการกฎการล้างข้อมูล |
| PUT | `/api/v1/log-manage/cleanup-rules/:id` | ลงชื่อเข้าใช้แล้ว | อัปเดตกฎการล้างข้อมูล |
| POST | `/api/v1/log-manage/cleanup-table/:table` | ลงชื่อเข้าใช้แล้ว | Clean up the specified table now |
| GET | `/api/v1/log-manage/core-log-output` | ลงชื่อเข้าใช้แล้ว | การกำหนดค่าเอาต์พุตบันทึกหลัก |
| PUT | `/api/v1/log-manage/core-log-output` | ลงชื่อเข้าใช้แล้ว | Save core log output config |
| GET | `/api/v1/log-manage/core/:type/stdout-log` | ลงชื่อเข้าใช้แล้ว | อ่านบันทึก stdout หลัก |
| GET | `/api/v1/log-manage/core/:type/stdout-stream` | ลงชื่อเข้าใช้แล้ว | สตรีมคอร์ stdout |
| POST | `/api/v1/log-manage/discover` | ลงชื่อเข้าใช้แล้ว | ค้นหาไฟล์บันทึกของโหนด |
| GET | `/api/v1/log-manage/log-output` | ลงชื่อเข้าใช้แล้ว | บันทึกการกำหนดค่าเอาต์พุต |
| PUT | `/api/v1/log-manage/log-output` | ลงชื่อเข้าใช้แล้ว | บันทึกการกำหนดค่าเอาต์พุตบันทึก |
| POST | `/api/v1/log-manage/log-output/release/:type` | ลงชื่อเข้าใช้แล้ว | ปล่อยบันทึกการครอบครอง stdout |
| POST | `/api/v1/log-manage/log-output/takeover/:type` | ลงชื่อเข้าใช้แล้ว | ยึดครอง log stdout |
| GET | `/api/v1/log-manage/nodes/:serverId/logs` | ลงชื่อเข้าใช้แล้ว | รายการไฟล์บันทึกของโหนด |
| GET | `/api/v1/log-manage/nodes/:serverId/logs/:type` | ลงชื่อเข้าใช้แล้ว | อ่านบันทึกของโหนด |
| POST | `/api/v1/log-manage/nodes/:serverId/logs/:type/rotate` | ลงชื่อเข้าใช้แล้ว | หมุนเวียนบันทึกของโหนด |
| GET | `/api/v1/log-manage/nodes/:serverId/logs/:type/stream` | ลงชื่อเข้าใช้แล้ว | บันทึกโหนดสตรีม |
| GET | `/api/v1/log-manage/policies` | ลงชื่อเข้าใช้แล้ว | รายการนโยบาย |
| POST | `/api/v1/log-manage/policies` | ลงชื่อเข้าใช้แล้ว | สร้างนโยบาย |
| DELETE | `/api/v1/log-manage/policies/:id` | ลงชื่อเข้าใช้แล้ว | ลบนโยบาย |
| GET | `/api/v1/log-manage/policies/:id` | ลงชื่อเข้าใช้แล้ว | รายละเอียดนโยบาย |
| PUT | `/api/v1/log-manage/policies/:id` | ลงชื่อเข้าใช้แล้ว | ปรับปรุงนโยบาย |
| GET | `/api/v1/log-manage/policies/:id/jobs` | ลงชื่อเข้าใช้แล้ว | รายการงานทำความสะอาด |
| GET | `/api/v1/log-manage/policies/:id/log` | ลงชื่อเข้าใช้แล้ว | อ่านบันทึก |
| GET | `/api/v1/log-manage/policies/:id/log-stream` | ลงชื่อเข้าใช้แล้ว | บันทึกสตรีม |
| POST | `/api/v1/log-manage/policies/:id/rotate` | ลงชื่อเข้าใช้แล้ว | หมุน |
| GET | `/api/v1/log-manage/proxy/:type/stdout-log` | ลงชื่อเข้าใช้แล้ว | อ่านบันทึก stdout หลัก |
| GET | `/api/v1/log-manage/proxy/:type/stdout-stream` | ลงชื่อเข้าใช้แล้ว | สตรีมคอร์ stdout |
| GET | `/api/v1/log-manage/scheduler-config` | ลงชื่อเข้าใช้แล้ว | การกำหนดค่ากำหนดการ |
| PUT | `/api/v1/log-manage/scheduler-config` | ลงชื่อเข้าใช้แล้ว | อัปเดตการกำหนดค่าการกำหนดเวลา |
| GET | `/api/v1/log-manage/status` | ลงชื่อเข้าใช้แล้ว | สถานะ |
| POST | `/api/v1/log-manage/sync` | ลงชื่อเข้าใช้แล้ว | ซิงค์นโยบายการล้างข้อมูล |
| GET | `/api/v1/log-manage/writers` | ลงชื่อเข้าใช้แล้ว | สถานะนักเขียน |
| PUT | `/api/v1/log-manage/writers/:type/config` | ลงชื่อเข้าใช้แล้ว | อัปเดตการกำหนดค่าตัวเขียน |
| GET | `/api/v1/log-manage/writers/:type/files` | ลงชื่อเข้าใช้แล้ว | รายการไฟล์บันทึกของผู้เขียน |
| POST | `/api/v1/log-manage/writers/:type/start` | ลงชื่อเข้าใช้แล้ว | เริ่มเขียน |
| POST | `/api/v1/log-manage/writers/:type/stop` | ลงชื่อเข้าใช้แล้ว | หยุดนักเขียน. |

### การสำรองข้อมูล

ฐานข้อมูล/การสำรองข้อมูลทั้งหมด การกู้คืน งานตามกำหนดเวลา พื้นที่จัดเก็บข้อมูลระยะไกล FTP/S3/WebDAV

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| DELETE | `/api/v1/backups/:id` | ผู้ดูแลระบบ | ลบข้อมูลสำรอง |
| GET | `/api/v1/backups/:id/download` | ผู้ดูแลระบบ | ดาวน์โหลดข้อมูลสำรอง |
| POST | `/api/v1/backups/:id/restore` | ผู้ดูแลระบบ | กู้คืนจากข้อมูลสำรอง |
| POST | `/api/v1/backups/:id/upload-ftp` | ผู้ดูแลระบบ | อัปโหลดไปที่ FTP |
| POST | `/api/v1/backups/:id/upload-s3` | ผู้ดูแลระบบ | อัปโหลดไปที่ S3 |
| POST | `/api/v1/backups/:id/upload-webdav` | ผู้ดูแลระบบ | อัปโหลดไปที่ WebDAV |
| POST | `/api/v1/backups/create` | ผู้ดูแลระบบ | สร้างการสำรองข้อมูลเต็มรูปแบบ (พร้อมดัมพ์ DB) |
| GET | `/api/v1/backups/list` | ผู้ดูแลระบบ | รายการสำรองข้อมูลทั้งหมด |
| POST | `/api/v1/backups/remote-restore` | ผู้ดูแลระบบ | กู้คืนจากระยะไกล |
| GET | `/api/v1/backups/restore-status` | ผู้ดูแลระบบ | ฟื้นฟูความคืบหน้า |
| GET | `/api/v1/backups/schedules` | ผู้ดูแลระบบ | รายการงานที่กำหนดเวลาไว้ |
| POST | `/api/v1/backups/schedules` | ผู้ดูแลระบบ | สร้างงานที่กำหนดเวลาไว้ |
| DELETE | `/api/v1/backups/schedules/:id` | ผู้ดูแลระบบ | ลบงานที่กำหนดเวลาไว้ |
| PUT | `/api/v1/backups/schedules/:id` | ผู้ดูแลระบบ | อัพเดตงานที่กำหนดเวลาไว้ |
| GET | `/api/v1/backups/storages` | ผู้ดูแลระบบ | รายการกำหนดค่าพื้นที่เก็บข้อมูล |
| POST | `/api/v1/backups/storages` | ผู้ดูแลระบบ | สร้างการกำหนดค่าพื้นที่เก็บข้อมูล |
| DELETE | `/api/v1/backups/storages/:id` | ผู้ดูแลระบบ | ลบการกำหนดค่าพื้นที่เก็บข้อมูล |
| PUT | `/api/v1/backups/storages/:id` | ผู้ดูแลระบบ | อัปเดตการกำหนดค่าพื้นที่เก็บข้อมูล |
| GET | `/api/v1/backups/storages/:id/files` | ผู้ดูแลระบบ | รายการไฟล์สำรองข้อมูลระยะไกล |
| POST | `/api/v1/backups/upload-restore` | ผู้ดูแลระบบ | อัปโหลดการสำรองและกู้คืน |

### การตั้งค่าระบบ

การกำหนดค่าและการย้ายฐานข้อมูล การตั้งค่าแผงควบคุม คีย์ JWT การทดสอบ SMTP/Telegram เทมเพลตการแจ้งเตือน การอัปเกรดออนไลน์ (ส่วนใหญ่เป็นผู้ดูแลระบบ)

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| PUT | `/api/v1/system/bootstrap-finish` | ผู้ดูแลระบบ | ตั้งค่าการบูตครั้งแรกให้เสร็จสิ้น |
| GET | `/api/v1/system/bootstrap-state` | ผู้ดูแลระบบ | สถานะการบูตครั้งแรก |
| GET | `/api/v1/system/config-file/:coreType` | ผู้ดูแลระบบ | อ่านไฟล์กำหนดค่าหลัก |
| GET | `/api/v1/system/db-config` | ผู้ดูแลระบบ | การกำหนดค่าฐานข้อมูล |
| PUT | `/api/v1/system/db-config` | ผู้ดูแลระบบ | อัพเดตการกำหนดค่าฐานข้อมูล |
| POST | `/api/v1/system/db-config/migrate` | ผู้ดูแลระบบ | ย้ายข้อมูลฐานข้อมูล |
| POST | `/api/v1/system/db-config/switch` | ผู้ดูแลระบบ | สลับฐานข้อมูล (พร้อมการโยกย้ายขั้นตอนเดียว) |
| POST | `/api/v1/system/db-config/test` | ผู้ดูแลระบบ | ทดสอบการเชื่อมต่อฐานข้อมูล |
| GET | `/api/v1/system/db-pool-stats` | ผู้ดูแลระบบ | สถิติพูลการเชื่อมต่อ |
| GET | `/api/v1/system/db-stats` | ผู้ดูแลระบบ | สถิติฐานข้อมูล |
| GET | `/api/v1/system/db-stats/ping` | ผู้ดูแลระบบ | ตรวจสอบฐานข้อมูล |
| POST | `/api/v1/system/db-stats/reset` | ผู้ดูแลระบบ | รีเซ็ตสถิติฐานข้อมูล |
| GET | `/api/v1/system/encryption-key-status` | ผู้ดูแลระบบ | สถานะคีย์การเข้ารหัส |
| GET | `/api/v1/system/env-config` | ผู้ดูแลระบบ | การกำหนดค่า .env |
| PUT | `/api/v1/system/env-config` | ผู้ดูแลระบบ | อัปเดตการกำหนดค่า .env |
| POST | `/api/v1/system/generate-encryption-key` | ผู้ดูแลระบบ | สร้างคีย์เข้ารหัส |
| GET | `/api/v1/system/github-config` | ผู้ดูแลระบบ | การกำหนดค่า GitHub |
| PUT | `/api/v1/system/github-config` | ผู้ดูแลระบบ | อัปเดตการกำหนดค่า GitHub |
| GET | `/api/v1/system/jwt-secret-status` | ผู้ดูแลระบบ | สถานะคีย์ JWT |
| GET | `/api/v1/system/maintenance` | สาธารณะ | สถานะโหมดการบำรุงรักษา (สาธารณะ) |
| PUT | `/api/v1/system/maintenance` | ผู้ดูแลระบบ | สลับโหมดการบำรุงรักษา |
| POST | `/api/v1/system/migrate-data` | ผู้ดูแลระบบ | ย้ายข้อมูล |
| GET | `/api/v1/system/notification-config` | ผู้ดูแลระบบ | การกำหนดค่าการแจ้งเตือน |
| PUT | `/api/v1/system/notification-config` | ผู้ดูแลระบบ | อัปเดตการกำหนดค่าการแจ้งเตือน |
| GET | `/api/v1/system/notification-templates` | ผู้ดูแลระบบ | รายการเทมเพลต |
| POST | `/api/v1/system/notification-templates` | ผู้ดูแลระบบ | สร้างเทมเพลต |
| DELETE | `/api/v1/system/notification-templates/:id` | ผู้ดูแลระบบ | ลบเทมเพลต |
| PUT | `/api/v1/system/notification-templates/:id` | ผู้ดูแลระบบ | อัปเดตเทมเพลต |
| POST | `/api/v1/system/notification-templates/:id/broadcast` | ผู้ดูแลระบบ | การแจ้งเตือนการออกอากาศตามเทมเพลต |
| POST | `/api/v1/system/notification-templates/:id/send` | ผู้ดูแลระบบ | ส่งการแจ้งเตือนการทดสอบ |
| GET | `/api/v1/system/panel-config` | ผู้ดูแลระบบ | การกำหนดค่าแผง |
| PUT | `/api/v1/system/panel-config` | ผู้ดูแลระบบ | อัปเดตการกำหนดค่าพาเนล |
| GET | `/api/v1/system/reality-targets` | ผู้ดูแลระบบ | รายการเป้าหมาย REALITY |
| PUT | `/api/v1/system/reality-targets` | ผู้ดูแลระบบ | อัพเดตเป้าหมาย REALITY |
| POST | `/api/v1/system/reality-targets/reset` | ผู้ดูแลระบบ | รีเซ็ตเป้าหมาย REALITY เป็นค่าเริ่มต้น |
| POST | `/api/v1/system/regenerate-jwt-secret` | ผู้ดูแลระบบ | สร้างคีย์ JWT ใหม่ |
| GET | `/api/v1/system/releases` | ผู้ดูแลระบบ | รุ่นที่มีจำหน่าย |
| POST | `/api/v1/system/restart` | ผู้ดูแลระบบ | รีสตาร์ท |
| POST | `/api/v1/system/rollback` | ผู้ดูแลระบบ | การย้อนกลับของระบบ |
| GET | `/api/v1/system/settings` | ผู้ดูแลระบบ | อ่านการตั้งค่า |
| PUT | `/api/v1/system/settings/batch` | ผู้ดูแลระบบ | การตั้งค่าการอัปเดตจำนวนมาก |
| GET | `/api/v1/system/stats` | ผู้ดูแลระบบ | สถานะของระบบ |
| GET | `/api/v1/system/subscription-billing-templates` | ผู้ดูแลระบบ | เทมเพลตการเรียกเก็บเงินการสมัครสมาชิก |
| PUT | `/api/v1/system/subscription-billing-templates` | ผู้ดูแลระบบ | อัปเดตเทมเพลตการเรียกเก็บเงินการสมัครสมาชิก |
| POST | `/api/v1/system/telegram-chat-id` | ผู้ดูแลระบบ | รับ ID แชท Telegram |
| POST | `/api/v1/system/test-smtp` | ผู้ดูแลระบบ | ทดสอบ SMTP |
| POST | `/api/v1/system/test-telegram` | ผู้ดูแลระบบ | ทดสอบ Telegram |
| POST | `/api/v1/system/update` | ผู้ดูแลระบบ | การอัพเกรดระบบ |
| GET | `/api/v1/system/update-jobs` | ผู้ดูแลระบบ | อัพเกรดรายการงาน |
| POST | `/api/v1/system/upload-mtls-ca` | ผู้ดูแลระบบ | อัปโหลดใบรับรอง mTLS CA |

### การตรวจสอบความปลอดภัย

ตรวจสอบการสืบค้นเหตุการณ์และการส่งออก

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| GET | `/api/v1/security/audit-events` | ลงชื่อเข้าใช้แล้ว | รายการกิจกรรมการตรวจสอบ |
| GET | `/api/v1/security/audit-events/export` | ลงชื่อเข้าใช้แล้ว | ส่งออกเหตุการณ์การตรวจสอบ |

### เมตริก

แผงเมตริกรวม (โหนด/ใบรับรอง/บริการ)

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| GET | `/api/v1/metrics/certificates` | ลงชื่อเข้าใช้แล้ว | ตัวชี้วัดใบรับรอง |
| GET | `/api/v1/metrics/nodes` | ลงชื่อเข้าใช้แล้ว | ตัวชี้วัดโหนด |
| GET | `/api/v1/metrics/overview` | ลงชื่อเข้าใช้แล้ว | ภาพรวม |

### เทมเพลตพอร์ทัล

เทมเพลตพอร์ทัล การกำหนดค่าการเปิดใช้งาน และการจัดการแคช

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| GET | `/api/v1/portal/active` | ลงชื่อเข้าใช้แล้ว | เทมเพลตที่ใช้งานอยู่ |
| PUT | `/api/v1/portal/active` | ลงชื่อเข้าใช้แล้ว | อัปเดตเทมเพลตที่ใช้งานอยู่ |
| DELETE | `/api/v1/portal/cache` | ลงชื่อเข้าใช้แล้ว | ล้างแคชพอร์ทัล |
| GET | `/api/v1/portal/cache` | ลงชื่อเข้าใช้แล้ว | สถานะแคช |
| PUT | `/api/v1/portal/cache/dev-mode` | ลงชื่อเข้าใช้แล้ว | สลับโหมดการพัฒนาแคช |
| POST | `/api/v1/portal/cache/rebuild` | ลงชื่อเข้าใช้แล้ว | สร้างแคชพอร์ทัลใหม่ |
| POST | `/api/v1/portal/reset` | ลงชื่อเข้าใช้แล้ว | คืนค่าเทมเพลตเริ่มต้น |
| GET | `/api/v1/portal/templates` | ลงชื่อเข้าใช้แล้ว | รายการ |
| POST | `/api/v1/portal/templates` | ลงชื่อเข้าใช้แล้ว | สร้าง |
| DELETE | `/api/v1/portal/templates/:id` | ลงชื่อเข้าใช้แล้ว | ลบ |
| GET | `/api/v1/portal/templates/:id` | ลงชื่อเข้าใช้แล้ว | รายละเอียด |
| PUT | `/api/v1/portal/templates/:id` | ลงชื่อเข้าใช้แล้ว | อัปเดต |
| POST | `/api/v1/portal/templates/:id/activate` | ลงชื่อเข้าใช้แล้ว | เปิดใช้งานเทมเพลต |

> `/portal/templates` CRUD เป็น **ขั้นสูง API**: UI พอร์ทัลใช้ `PUT /api/v1/portal/active` เป็นหลัก
> (การกำหนดค่าแบบเต็มไซต์เพียงครั้งเดียว); จัดการ/สลับหลายเทมเพลตกับกลุ่มนี้ ทั้งสองได้รับการสนับสนุนและไม่ขัดแย้งกัน

### การติดตั้งตัวแทน

สคริปต์การติดตั้ง Node Agent และดาวน์โหลด (สาธารณะ)

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| POST | `/api/v1/agent/bootstrap` | สาธารณะ | การลงทะเบียนการบูตครั้งแรกของตัวแทน (รายงานตัวแทนด้วยโทเค็นบูตสแตรป สาธารณะ) |
| GET | `/api/v1/agent/releases` | ผู้ดูแลระบบ | เวอร์ชันตัวแทนที่มีอยู่ (สำหรับหน้าอัปเกรด) |
| GET | `/api/v1/agent/version` | ผู้ดูแลระบบ | เวอร์ชันตัวแทนปัจจุบันบนแผงควบคุม |
| GET | `/api/v1/agent/download` | สาธารณะ | ดาวน์โหลดตัวแทน |
| GET | `/api/v1/agent/install` | สาธารณะ | รับสคริปต์การติดตั้ง |

### เรียกเก็บเงินรายการโทรกลับ

การเรียกกลับแพลตฟอร์มการชำระเงินของบุคคลที่สาม (สาธารณะ ตรวจสอบลายเซ็นแล้ว)

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| POST | `/api/v1/charge/callback/:site_id` | สาธารณะ | จัดการการโทรกลับการเรียกเก็บเงิน |

### ขาเข้า (พร้อมโหนด)

รายการขาเข้าที่ใช้สำหรับการเลือกโหนดแผน

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| GET | `/api/v1/inbounds-with-server` | ลงชื่อเข้าใช้แล้ว | ขาเข้าทั้งหมด (รวมถึงโหนด) |

### การกำหนดค่าสาธารณะ

การกำหนดค่ารันไทม์ของพอร์ทัลโดยไม่ต้องลงชื่อเข้าใช้ (เจ้าหน้าที่บำรุงรักษา + ขีดจำกัดอัตราสาธารณะ)

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| GET | `/api/v1/public/articles` | สาธารณะ | รายการบทความสาธารณะ |
| GET | `/api/v1/public/articles/:slug` | สาธารณะ | รายละเอียดบทความสาธารณะ |
| GET | `/api/v1/public/client-configs` | สาธารณะ | การกำหนดค่าการดาวน์โหลดไคลเอนต์ (สาธารณะ) |
| GET | `/api/v1/public/portal-config` | สาธารณะ | การกำหนดค่ารันไทม์ของพอร์ทัล (ชื่อไซต์ ธีม ภาษา เอกสารการปฏิบัติตามข้อกำหนด สาธารณะ) |
| GET | `/api/v1/public/settings` | สาธารณะ | การตั้งค่าสาธารณะ (เช่น การลงทะเบียน_เปิดใช้งาน; สาธารณะ) |
| GET | `/api/v1/public/subscription-endpoints` | สาธารณะ | รายการปลายทางการสมัครสมาชิกที่เปิดใช้งาน (สาธารณะ) |

### การตรวจสอบสิทธิ์ลูกค้าและธุรกิจ (ชุดเดียวกันในเครื่องมือผู้ดูแลระบบ)

จุดสิ้นสุดไคลเอ็นต์เหมือนกับพอร์ทัล ดู `portal-api.en.md` สำหรับรายละเอียด

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| GET | `/api/v1/client-auth/articles` | สาธารณะ | แสดงรายการบทความ |
| GET | `/api/v1/client-auth/articles/:slug` | สาธารณะ | รับบทความ |
| GET | `/api/v1/client-auth/balance` | สาธารณะ | ได้รับความสมดุล |
| GET | `/api/v1/client-auth/balance/transactions` | สาธารณะ | ธุรกรรมยอดคงเหลือ |
| GET | `/api/v1/client-auth/bills` | สาธารณะ | รายการสั่งซื้อ |
| POST | `/api/v1/client-auth/change-password` | สาธารณะ | เปลี่ยนรหัสผ่าน |
| POST | `/api/v1/client-auth/charge/create` | สาธารณะ | สร้างคำสั่งเติมเงิน |
| GET | `/api/v1/client-auth/charge/orders` | สาธารณะ | แสดงรายการคำสั่งเติมเงิน |
| GET | `/api/v1/client-auth/charge/orders/:id` | สาธารณะ | รับรายละเอียดการสั่งซื้อการเติมเงิน |
| POST | `/api/v1/client-auth/charge/redeem-card` | สาธารณะ | แลกบัตร |
| GET | `/api/v1/client-auth/charge/sites` | สาธารณะ | รายชื่อเว็บไซต์เติมเงิน |
| GET | `/api/v1/client-auth/charge/status/:trade_no` | สาธารณะ | สอบถามสถานะการเติมเงิน |
| GET | `/api/v1/client-auth/checktime` | สาธารณะ | รับเวลาเซิร์ฟเวอร์ (ซิงค์นาฬิกา) |
| GET | `/api/v1/client-auth/connection-info` | สาธารณะ | รับพารามิเตอร์การเชื่อมต่อแผน |
| GET | `/api/v1/client-auth/gifts` | สาธารณะ | รายการของขวัญ |
| POST | `/api/v1/client-auth/gifts/:id/claim` | สาธารณะ | เรียกร้องของขวัญ |
| GET | `/api/v1/client-auth/invoices` | สาธารณะ | รายการสั่งซื้อ |
| GET | `/api/v1/client-auth/invoices/:id` | สาธารณะ | รายละเอียดการสั่งซื้อ |
| GET | `/api/v1/client-auth/ip-whitelist` | สาธารณะ | แสดงรายการรายการที่อนุญาต IP |
| DELETE | `/api/v1/client-auth/ip-whitelist/:id` | สาธารณะ | ลบออกจากรายการอนุญาต IP |
| POST | `/api/v1/client-auth/ip-whitelist/auto-add` | สาธารณะ | เพิ่ม IP อัตโนมัติ |
| POST | `/api/v1/client-auth/login` | สาธารณะ | เข้าสู่ระบบ |
| POST | `/api/v1/client-auth/logout` | สาธารณะ | ออกจากระบบ |
| POST | `/api/v1/client-auth/logs/package` | สาธารณะ | สร้างบันเดิลบันทึก |
| GET | `/api/v1/client-auth/logs/packages` | สาธารณะ | แสดงรายการบันเดิลบันทึก |
| DELETE | `/api/v1/client-auth/logs/packages/:id` | สาธารณะ | ลบบันเดิลบันทึก |
| GET | `/api/v1/client-auth/logs/packages/:id` | สาธารณะ | รับบันเดิลบันทึก |
| GET | `/api/v1/client-auth/logs/packages/:id/download` | สาธารณะ | ดาวน์โหลดบันเดิลบันทึก |
| GET | `/api/v1/client-auth/logs/types` | สาธารณะ | การตรวจสุขภาพบันเดิลบันทึก |
| GET | `/api/v1/client-auth/me` | สาธารณะ | รับผู้ใช้ที่ลงชื่อเข้าใช้ปัจจุบัน |
| GET | `/api/v1/client-auth/nodes` | สาธารณะ | รายการโหนดของฉัน |
| GET | `/api/v1/client-auth/nodes/:id/config` | สาธารณะ | การกำหนดค่าโหนด |
| GET | `/api/v1/client-auth/nodes/:id/link` | สาธารณะ | ลิงก์แชร์โหนด |
| GET | `/api/v1/client-auth/nodes/:id/qrcode` | สาธารณะ | รหัส QR ของโหนด |
| GET | `/api/v1/client-auth/oauth/:provider` | สาธารณะ | อนุญาต |
| GET | `/api/v1/client-auth/oauth/:provider/callback` | สาธารณะ | โทรกลับ |
| GET | `/api/v1/client-auth/oauth/providers` | สาธารณะ | สถานะผู้ให้บริการ |
| GET | `/api/v1/client-auth/orders` | สาธารณะ | รายการสั่งซื้อ |
| POST | `/api/v1/client-auth/orders` | สาธารณะ | สร้างคำสั่งซื้อ |
| GET | `/api/v1/client-auth/orders/:id` | สาธารณะ | รายละเอียดการสั่งซื้อ |
| GET | `/api/v1/client-auth/plans` | สาธารณะ | รายการแผน |
| GET | `/api/v1/client-auth/plans/:id` | สาธารณะ | รายละเอียดแผน |
| GET | `/api/v1/client-auth/profile` | สาธารณะ | รับโปรไฟล์ |
| PUT | `/api/v1/client-auth/profile` | สาธารณะ | อัพเดตโปรไฟล์ |
| POST | `/api/v1/client-auth/profile/bind-oauth` | สาธารณะ | ผูกบัญชีบุคคลที่สาม |
| DELETE | `/api/v1/client-auth/profile/unbind-oauth/:provider` | สาธารณะ | เลิกผูกบัญชีบุคคลที่สาม |
| GET | `/api/v1/client-auth/referral/code` | สาธารณะ | รับรหัสยืนยันการอ้างอิง |
| GET | `/api/v1/client-auth/referral/records` | สาธารณะ | แสดงรายการบันทึกการอ้างอิง |
| GET | `/api/v1/client-auth/referral/stats` | สาธารณะ | รับสถิติการอ้างอิง |
| POST | `/api/v1/client-auth/referral/withdraw` | สาธารณะ | ขอถอนเงิน |
| GET | `/api/v1/client-auth/referral/withdrawals` | สาธารณะ | แสดงรายการการถอนการอ้างอิง |
| POST | `/api/v1/client-auth/refresh` | สาธารณะ | รีเฟรช |
| POST | `/api/v1/client-auth/register` | สาธารณะ | ลงทะเบียน |
| POST | `/api/v1/client-auth/reset-subscription` | สาธารณะ | รีเซ็ตการสมัครรับข้อมูล |
| POST | `/api/v1/client-auth/rotate-token` | สาธารณะ | หมุนเวียนโทเค็นการสมัครสมาชิก |
| POST | `/api/v1/client-auth/self-unban` | สาธารณะ | ยกเลิกการแบนแบบบริการตนเอง |
| PUT | `/api/v1/client-auth/settings` | สาธารณะ | อัปเดตการตั้งค่า |
| POST | `/api/v1/client-auth/settings/auto-add-ip` | สาธารณะ | เพิ่ม IP อัตโนมัติ |
| GET | `/api/v1/client-auth/settings/whitelist` | สาธารณะ | แสดงรายการรายการที่อนุญาต IP |
| DELETE | `/api/v1/client-auth/settings/whitelist/:id` | สาธารณะ | ลบออกจากรายการอนุญาต IP |
| GET | `/api/v1/client-auth/shared-accounts` | สาธารณะ | แสดงรายการบัญชีที่ใช้ร่วมกัน |
| GET | `/api/v1/client-auth/shared-accounts/:id` | สาธารณะ | รับบัญชีที่ใช้ร่วมกัน |
| POST | `/api/v1/client-auth/shared-accounts/:id/apply` | สาธารณะ | ขอบัญชีที่ใช้ร่วมกัน |
| POST | `/api/v1/client-auth/shared-accounts/:id/repair` | สาธารณะ | แจ้งซ่อม |
| POST | `/api/v1/client-auth/ticket` | สาธารณะ | ออกตั๋วอายุสั้นครั้งเดียว |
| GET | `/api/v1/client-auth/tickets` | สาธารณะ | รายการตั๋ว |
| POST | `/api/v1/client-auth/tickets` | สาธารณะ | สร้างตั๋ว |
| GET | `/api/v1/client-auth/tickets/:id` | สาธารณะ | รับตั๋ว |
| POST | `/api/v1/client-auth/tickets/:id/close` | สาธารณะ | ปิดตั๋ว |
| POST | `/api/v1/client-auth/tickets/:id/reply` | สาธารณะ | ตอบเรื่องตั๋วครับ |
| GET | `/api/v1/client-auth/traffic-history` | สาธารณะ | ประวัติการเข้าชม |
| GET | `/api/v1/client-auth/traffic-packs` | สาธารณะ | รายการชุดการจราจร |
| POST | `/api/v1/client-auth/traffic-packs/buy` | สาธารณะ | ซื้อชุดจราจร |

### การจัดส่งการสมัครสมาชิก

จุดเริ่มต้นสาธารณะสำหรับลูกค้าในการดึงข้อมูลการสมัครสมาชิก ดู `subscription.en.md`

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| GET | `/sub/:token` | สาธารณะ | ให้บริการสมัครสมาชิก |

### เวิร์กโฟลว์การกำหนดค่ารันไทม์

กำหนดค่าสแนปช็อต / ทดลองรัน / ใช้ / ย้อนกลับ (`apply`, `rollback` และ `disable-proxy` ต้องมีผู้ดูแลระบบ)

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| POST | `/runtime/apply` | สาธารณะ | สมัคร |
| GET | `/runtime/apply-jobs` | ลงชื่อเข้าใช้แล้ว | รายการงาน |
| GET | `/runtime/apply-jobs/:id` | ลงชื่อเข้าใช้แล้ว | รับงาน |
| POST | `/runtime/apply-jobs/:id/cancel` | สาธารณะ | ยกเลิกงาน |
| POST | `/runtime/apply-jobs/:id/retry` | สาธารณะ | ลองงานอีกครั้ง |
| POST | `/runtime/disable-proxy` | สาธารณะ | ปิดการใช้งานพรอกซี |
| POST | `/runtime/dry-run` | ลงชื่อเข้าใช้แล้ว | การดำเนินการแบบทดลองรัน |
| GET | `/runtime/nodes` | ลงชื่อเข้าใช้แล้ว | แสดงรายการโหนด |
| GET | `/runtime/overview` | ลงชื่อเข้าใช้แล้ว | ภาพรวม |
| POST | `/runtime/rollback` | สาธารณะ | ย้อนกลับ |
| GET | `/runtime/snapshots` | ลงชื่อเข้าใช้แล้ว | แสดงรายการสแนปชอต |
| POST | `/runtime/snapshots` | ลงชื่อเข้าใช้แล้ว | สร้างภาพรวม |
| DELETE | `/runtime/snapshots/:id` | สาธารณะ | ลบสแนปชอต |
| GET | `/runtime/snapshots/:id` | ลงชื่อเข้าใช้แล้ว | รับภาพรวม |
| POST | `/runtime/verify` | ลงชื่อเข้าใช้แล้ว | ตรวจสอบ |

### นโยบายการเข้าถึงบริการ (เลิกใช้แล้ว)

> `/services/:id/access-policies` และ `/services/:id/ip-bans` เลิกให้บริการในวันที่ 2026-10-20:
> ไม่มีการเรียกของผู้ดูแลระบบและไม่มีเส้นทางการเชื่อมต่อที่ใช้นโยบายเหล่านี้

### การตรวจสอบโควต้าการจราจร

มุมมองการตรวจสอบโควต้าและการใช้งานแบบอ่านอย่างเดียว

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| GET | `/traffic/quotas` | ลงชื่อเข้าใช้แล้ว | รายการโควต้า |
| GET | `/traffic/quotas/:id/usage` | ลงชื่อเข้าใช้แล้ว | การใช้โควต้า |
| GET | `/traffic/reset-logs` | ลงชื่อเข้าใช้แล้ว | บันทึกการรีเซ็ตโควต้า |
| GET | `/traffic/usage-history` | ลงชื่อเข้าใช้แล้ว | ประวัติการใช้งาน |

### การตรวจสุขภาพ

การสอบสวนความมีชีวิตชีวา

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| GET | `/health` | สาธารณะ | โพรบความมีชีวิตชีวา; 200 หมายถึง สุขภาพแข็งแรง |

### หน้าพอร์ทัล

หน้าพอร์ทัลสปา

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| GET | `/p` | สาธารณะ | พอร์ทัล HTML (แสดงผลโดยเซิร์ฟเวอร์ + รันไทม์ Vue) |

### ความท้าทาย ACME

HTTP-01 ท้าทายการโทรกลับ

| วิธีการ | เส้นทาง | การอนุญาต | คำอธิบาย |
|---|---|---|---|
| GET | `/.well-known/acme-challenge/:token` | สาธารณะ | ACME HTTP-01 การโทรกลับที่ท้าทาย (ตรวจสอบโดเมนในระหว่างการออก) |
