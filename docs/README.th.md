<!-- source: README.md | locale: th | updated: 2026-10-23 -->
# เอกสารประกอบ CoreFusion

> ภาษา: [简体中文](README.md) · [English](README.en.md) · [繁體中文](README.zh-TW.md) · [Русский](README.ru.md) · [日本語](README.ja.md) · [한국어](README.ko.md) · [Deutsch](README.de.md) · [Français](README.fr.md) · [Español](README.es.md) · [Português](README.pt.md) · [Türkçe](README.tr.md) · [Bahasa Indonesia](README.id.md) · ไทย · [Tiếng Việt](README.vi.md) · [العربية](README.ar.md) · [فارسی](README.fa.md)

## [USER-GUIDE.th.md](./USER-GUIDE.th.md) — คู่มือผู้ใช้

คู่มือการใช้งานฉบับสมบูรณ์สำหรับ **ผู้ใช้** (ปฏิบัติการ / ผู้ดูแลระบบ / ผู้ปฏิบัติงาน): การติดตั้ง การกำหนดค่าเริ่มต้น โมดูลคุณลักษณะ คำถามที่พบบ่อย และภาคผนวก

## [api/](./api/) — ข้อมูลอ้างอิง API สาธารณะ

สำหรับผู้ประกอบระบบ การพัฒนารอง และลูกค้าบุคคลที่สาม:

- [api/README.th.md](./api/README.th.md) — ภาพรวม: ฐาน URL, การรับรองความถูกต้อง, รูปแบบการตอบสนอง, ขีดจำกัดอัตรา
- [api/subscription.th.md](./api/subscription.th.md) — จุดสิ้นสุดการสมัครสมาชิก (`/sub/:token`) รูปแบบ ส่วนหัวการตอบกลับ ตัวอย่าง
- [api/portal-api.th.md](./api/portal-api.th.md) — พอร์ทัลไคลเอ็นต์ API (ลงทะเบียน / ลงชื่อเข้าใช้ แผน คำสั่งซื้อ ยอดคงเหลือ ตั๋ว)
- [api/charge-callback.th.md](./api/charge-callback.th.md) — ลายเซ็นการโทรกลับการชำระเงินและการตรวจสอบ
- [api/admin-api.th.md](./api/admin-api.th.md) — ตารางเส้นทางผู้ดูแลระบบ API จัดกลุ่มตามโมดูล

## เอกสารการออกแบบ

เอกสารการออกแบบเป็นวัสดุภายใน (ทรัพย์สินทางปัญญาหลัก) และ **ไม่มีการแจกจ่าย** กับพื้นที่เก็บข้อมูลนี้ ติดต่อผู้ดูแลหากคุณต้องการ

> **ขอบเขต**: แอปไคลเอ็นต์ (UI ภายในเครื่อง, การครอบครอง TUN, ถาด, การรวมการสมัครสมาชิกหลายรายการ) เป็นผลิตภัณฑ์ที่แยกต่างหากและอยู่นอกขอบเขตสำหรับพื้นที่เก็บข้อมูลนี้ เอกสารประกอบที่นี่ครอบคลุมเฉพาะฝั่งเซิร์ฟเวอร์เท่านั้น (การจัดส่งพาเนล / โหนด / การสมัครสมาชิก)

## คู่มือการใช้งานอื่นๆ

- `cert-google-eab.md` — Google การออกใบรับรอง EAB *(ภาษาจีน)*
- `oauth-google-verification.md` — การตรวจสอบ OAuth Google *(ภาษาจีน)*
- `notification-templates.md` — ตัวแปรเทมเพลตการแจ้งเตือน *(ภาษาจีน)*

## ครอบคลุมการแปล

| เอกสาร | zh-CN | en | zh-TW | สถานที่อื่นๆ |
|----------|-------|----|-------|---------------|
| README.md | ✅ | ✅ | ✅ | ru ✅ |
| USER-GUIDE.md | ✅ | ✅ | ✅ | ru ✅ |
| I18N.md | ✅ | — | — | — |
| api/README.md | ✅ | ✅ | ✅ | ru ✅ |
| api/subscription.md | ✅ | ✅ | ✅ | ru ✅ |
| api/portal-api.md | ✅ | ✅ | ✅ | ru ✅ |
| api/charge-callback.md | ✅ | ✅ | ✅ | ru ✅ |
| api/admin-api.md | ✅ | ✅ | ✅ | ru ✅ |
| cert-google-eab.md | ✅ | ✅ | ✅ | ru ✅ |
| oauth-google-verification.md | ✅ | ✅ | ✅ | ru ✅ |
| notification-templates.md | ✅ | ✅ | ✅ | ru ✅ |
| LEGAL.md | ✅ | ✅ | ✅ | ru ✅ |
| THIRD_PARTY_LICENSES.md | ✅ | ✅ | ✅ | ru ✅ |

ดู [I18N.md](README.th.md) สำหรับการตั้งชื่อ การนำทางภาษา และกฎการทบทวน *(ภาษาจีน)*
