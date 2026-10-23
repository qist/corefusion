<!-- source: oauth-google-verification.md | locale: th | updated: 2026-10-23 -->
# คู่มือการตรวจสอบ OAuth Google

> ภาษา: [简体中文](oauth-google-verification.md) · [English](oauth-google-verification.en.md) · [繁體中文](oauth-google-verification.zh-TW.md) · [Русский](oauth-google-verification.ru.md) · [日本語](oauth-google-verification.ja.md) · [한국어](oauth-google-verification.ko.md) · [Deutsch](oauth-google-verification.de.md) · [Français](oauth-google-verification.fr.md) · [Español](oauth-google-verification.es.md) · [Português](oauth-google-verification.pt.md) · [Türkçe](oauth-google-verification.tr.md) · [Bahasa Indonesia](oauth-google-verification.id.md) · ไทย · [Tiếng Việt](oauth-google-verification.vi.md) · [العربية](oauth-google-verification.ar.md) · [فارسی](oauth-google-verification.fa.md)

> ขอบเขต: การลงชื่อเข้าใช้ Google สำหรับพอร์ทัล CoreFusion (`internal/service/oauth` ขอบเขต: `openid`, `email`, `profile`)
> การเปรียบเทียบ GitHub ในตอนท้าย (GitHub ไม่มีขั้นตอนการตรวจสอบ)

## 1. โหมดการทดสอบเทียบกับโหมดการใช้งานจริง

หน้าจอยินยอม OAuth ของ Google มีสถานะการเผยแพร่ 2 สถานะ:

| รัฐ | ใครบ้างที่สามารถลงชื่อเข้าใช้ได้ | จำเป็นต้องมีการยืนยัน |
|---|---|---|
| **การทดสอบ** | เฉพาะบัญชี Google ในรายการ "ผู้ใช้ทดสอบ" | ไม่ |
| **อยู่ระหว่างการผลิต** | บัญชี Google ใด ๆ | **ใช่** (ดูด้านล่าง) |

เมื่อเปิดใช้งาน “การตั้งค่าระบบ → OAuth” หน้าลงชื่อเข้าใช้พอร์ทัลจะแสดงปุ่ม Google แก่ผู้เยี่ยมชมทุกคน
หากแอปยังอยู่ในการทดสอบ ผู้เยี่ยมชมที่ไม่อยู่ในรายชื่อผู้ใช้ทดสอบจะเห็น
`Error 403: access_denied` (แอปไม่ได้รับการยืนยัน / คุณไม่ใช่ผู้ใช้ทดสอบ)
หากต้องการเสนอการลงชื่อเข้าใช้ Google แบบสาธารณะ **เปลี่ยนแอปเป็นเวอร์ชันที่ใช้งานจริงและผ่านการยืนยัน**

## 2. ข้อกำหนดที่ยากสำหรับการตรวจสอบ

การตรวจสอบของ Google (การตรวจสอบขั้นพื้นฐานสำหรับขอบเขตที่ไม่จำกัด) ต้องการ:

1. **หน้าแรก**: URL ที่เข้าถึงได้แบบสาธารณะบนโดเมนที่ได้รับอนุญาต (โดเมนเดียวกันหรือโดเมนย่อย)
2. **ลิงก์นโยบายความเป็นส่วนตัว**: ต้อง **เข้าถึงได้แบบสาธารณะ** (ไม่ต้องลงชื่อเข้าใช้) และอธิบายว่าข้อมูลใดที่คุณรวบรวม และวิธีที่คุณใช้/จัดเก็บ/ลบข้อมูล สำหรับการลงชื่อเข้าใช้ Google อย่างน้อยต้องครอบคลุม:
 ข้อมูลบัญชี Google ที่รวบรวม (อีเมล ชื่อ อวตาร) วัตถุประสงค์ (การสร้าง/ลงชื่อเข้าใช้บัญชีพอร์ทัล) ไม่มีการขายให้กับบุคคลที่สาม
 และผู้ใช้สามารถลบบัญชีและข้อมูลของตนได้อย่างไร
3. **ลิงก์ข้อกำหนดในการให้บริการ**: เข้าถึงได้แบบสาธารณะ โดยอธิบายกฎการให้บริการ
4. **โดเมนที่ได้รับอนุญาต**: ลงทะเบียนโดเมนของแอปในหน้าจอคำยินยอม (เช่น `example.com`) ลิงก์หน้าแรก/ความเป็นส่วนตัว/ข้อกำหนดจะต้องอยู่ในลิงก์หรือโดเมนย่อย
5. **โลโก้แอป (ไม่บังคับ แต่แนะนำ)**: 120×120px
6. **การให้เหตุผลขอบเขต**: โปรเจ็กต์นี้ใช้เฉพาะ `openid / email / profile` เท่านั้น (ไม่จำกัด และไม่ละเอียดอ่อน)
 ซึ่งอยู่ภายใต้การตรวจสอบขั้นพื้นฐาน — ไม่มีเหตุผลตามขอบเขตและ **ไม่มีการประเมินความปลอดภัยจากบุคคลที่สาม**
 (ขอบเขตที่จำกัด เช่น การเข้าถึง Gmail เต็มรูปแบบ จำเป็นต้องมี โปรเจ็กต์นี้ไม่ได้ใช้ขอบเขตใดๆ เลย)

รายการยืนยัน: [Google Cloud Console](https://console.cloud.google.com/) →
**API และบริการ → หน้าจอยินยอม OAuth → เผยแพร่แอป → ส่งเพื่อตรวจสอบ**
การตรวจสอบขั้นพื้นฐานมักจะเสร็จสิ้นภายในไม่กี่วัน คุณสามารถทำงานในโหมดทดสอบได้ในระหว่างนี้

## 3. สิ่งที่ต้องกำหนดค่าสำหรับโปรเจ็กต์นี้

### 3.1 การเปลี่ยนเส้นทางที่ได้รับอนุญาต URI

การโทรกลับจะถูกสร้างขึ้นแบบไดนามิกจากโดเมนที่เริ่มการลงชื่อเข้าใช้ (ดู `Authorize` ใน `internal/api/handlers/oauth/handler.go`):

```
https://<portal-domain>/api/v1/client-auth/oauth/google/callback
```

- การลงชื่อเข้าใช้เริ่มต้นจากโดเมนพอร์ทัล (เช่น `https://p.example.com`) → ลงทะเบียน `https://p.example.com/...`
- เริ่มต้นจากโดเมนพาเนล `/p` → ลงทะเบียน `https://panel.example.com/...`
- Google อนุญาต **หลาย** URI การเปลี่ยนเส้นทาง จดทะเบียนทั้งสองโดเมน
- ⚠ Google กำหนดให้ URI การเปลี่ยนเส้นทางเป็น **https** (ยกเว้น `http://localhost`)
 พร็อกซีย้อนกลับจะต้องส่งต่อ `X-Forwarded-Host` / `X-Forwarded-Proto` มิฉะนั้นแผงควบคุมจะสร้าง
 http หรือภายใน เปลี่ยนเส้นทาง_uri → Google ปฏิเสธ (`redirect_uri_mismatch`)

หลังจากตั้งค่า “การตั้งค่าระบบ → OAuth → โดเมนการเรียกกลับของพอร์ทัล” การเรียกกลับ URL ที่แสดงบนหน้านั้นเป็นรายการที่จะวางใน Google

### 3.2 ลิงก์นโยบายความเป็นส่วนตัว / ข้อกำหนดบนพอร์ทัล

การ์ดลงชื่อเข้าใช้ประกอบด้วยลิงก์ “นโยบายความเป็นส่วนตัว · ข้อกำหนดในการให้บริการ” (แสดงเมื่อกำหนดค่าแล้ว) และเอกสารทั้งสองฉบับมี **หน้าเดี่ยว**
(กล่องโต้ตอบและเพจแบบสแตนด์อโลนแชร์เนื้อหาเดียวกัน):

- โดเมนแผงหลัก: `https://<panel-domain>/p/legal/privacy`, `/p/legal/terms`
- โดเมนพอร์ทัลเฉพาะ (การปรับใช้รูท): `https://<portal-domain>/legal/privacy`, `/legal/terms`
- รองรับ `?lang=xx`; หากไม่มี จะใช้ `Accept-Language` ของเบราว์เซอร์ จากนั้นจะเป็นค่าเริ่มต้นของพอร์ทัล
- หน้าต่างๆ **แสดงผลโดยเซิร์ฟเวอร์** (ไม่มี JS) ซึ่งช่วยโปรแกรมรวบรวมข้อมูลการตรวจสอบของ Google

**การกำหนดค่า (ไม่ว่าจะด้วยวิธีใด ลิงก์จะชนะ)**:

1. **โหมดเนื้อหา (แนะนำ ใช้งานได้ทันที)**: ตัวแก้ไขพอร์ทัล → การตั้งค่า → ทั่วไป → เอกสารการปฏิบัติตามข้อกำหนด →
 เลือก "แก้ไขภาษา" และวางเทมเพลต §4 ลงใน "เนื้อหานโยบายความเป็นส่วนตัว" / "เนื้อหาข้อกำหนดในการให้บริการ"
 (**มาร์กดาวน์ แก้ไขตามภาษา**) → หลังจากบันทึก การ์ดลงชื่อเข้าใช้จะแสดงลิงก์ที่เปิด **กล่องโต้ตอบ**;
 “ลิงก์แบบสแตนด์อโลน” ของบรรณาธิการสามารถป้อนได้โดยตรงในหน้าจอยินยอม Google
2. **โหมดลิงก์ภายนอก**: ป้อน “นโยบายความเป็นส่วนตัว URL” / “ข้อกำหนดในการให้บริการ URL” (ชี้ไปที่เพจที่คุณโฮสต์) → เปิดในแท็บใหม่

เนื้อหาและลิงค์จัดส่งผ่าน `/api/v1/public/portal-config` (`privacy_policy_content` /
`terms_of_service_content` เป็น **แผนที่ Markdown ต่อภาษา** `{"zh-CN":"...","en":"..."}`;
`privacy_policy_url` / `terms_of_service_url` เป็นลิงก์ภายนอกเสริม) **ต้องสามารถเข้าถึงได้โดยไม่ต้องลงชื่อเข้าใช้** ✓

### 3.3 สวิตช์ข้างแผง

`System settings → OAuth`: เปิดใช้งาน Google และป้อน Client ID / Client Secret (ข้อมูลลับจะถูกเข้ารหัสด้วย AES-256-GCM ที่เหลือ)
ปุ่ม Google จะปรากฏบนหน้าลงชื่อเข้าใช้พอร์ทัลเฉพาะเมื่อมีการกำหนดค่าทุกอย่างแล้ว (เปิดใช้งานการส่งคืน `/api/v1/client-auth/oauth/providers`)

## 4. นโยบายความเป็นส่วนตัว / เทมเพลตข้อกำหนด

> เทมเพลตการปฏิบัติตามข้อกำหนดขั้นต่ำที่คุณสามารถปรับเปลี่ยนได้ ก่อนใช้งาน: เปลี่ยนตัวยึดตำแหน่ง `【】`, เพิ่ม/ลบส่วนคำสั่งสำหรับธุรกิจของคุณ
> และให้หน่วยงานปฏิบัติการตรวจสอบสิ่งเหล่านั้น เทมเพลตเหล่านี้ไม่ใช่คำแนะนำทางกฎหมาย

### 4.1 นโยบายความเป็นส่วนตัว (เทมเพลต)

```markdown
# Privacy Policy

Last updated: [DATE]

[OPERATOR NAME] ("we") operates [PORTAL NAME] ("the Service"). We respect your privacy;
this policy explains what information we collect and how we use it.

## 1. Information we collect
- **Account information**: the email address and password you provide at registration (stored only as a one-way hash);
- **Google sign-in information**: when you choose "Sign in with Google", we receive, with your authorization,
  your Google account email, display name and avatar URL (scopes: openid, email, profile);
- **Service data**: subscription configuration, traffic usage, device IP allow-list, tickets and order records.

## 2. How we use information
Only to: create and sign in to your account, provide the proxy subscription service, bill you and provide support,
and for security controls (sign-in protection, IP allow-list). We **do not** sell your personal information.

## 3. Third-party services
- **Google**: receives account information within the authorized scope only when you actively use "Sign in with Google";
- Payment providers: receive order information only when you initiate a payment.

## 4. Storage and security
Data is stored on our servers; credentials are encrypted at rest and all traffic uses TLS.

## 5. Your rights
You can update your profile/password and end sessions in the portal at any time; to delete your account and all data,
contact [CONTACT EMAIL] and we will process the request within [N] business days.

## 6. Policy changes
Material changes will be announced in the portal notice board.
```

### 4.2 ข้อกำหนดในการให้บริการ (เทมเพลต)

```markdown
# Terms of Service

Last updated: [DATE]

## 1. Service
The Service provides [PROXY / NETWORK ACCESS] to registered users; specific plans are described on the store page.

## 2. Accounts
- You are responsible for the security of your account and password; for accounts created with Google sign-in, credentials are managed by Google;
- You may not lend or transfer your account, or use the Service for any unlawful purpose (including but not limited to:
  unauthorized access to others' systems, distributing illegal content, or circumventing mandatory legal requirements).

## 3. Payment and refunds
Plans are billed [monthly / by usage]; traffic and device limits follow the plan description;
[REFUND POLICY: e.g. full refund within 7 days if unused].

## 4. Changes and termination
We may suspend or terminate the Service due to force majeure or your breach of these terms; deleting an account deletes the related data.

## 5. Disclaimer
The Service is provided "as is"; to the extent permitted by law, we are not liable for indirect losses caused by use of the Service.

## 6. Miscellaneous
These terms are governed by the laws of [JURISDICTION]. Contact: [CONTACT EMAIL].
```

## 5. รายการตรวจสอบก่อนการเปิดตัว

- [ ] หน้าจอยินยอม OAuth: ชื่อแอป / โลโก้ / หน้าแรก / นโยบายความเป็นส่วนตัว / ข้อกำหนดทั้งหมดปรากฏ; จดทะเบียนโดเมนแล้ว
- [ ] หน้านโยบายความเป็นส่วนตัวและข้อกำหนด **สามารถเข้าถึงได้โดยไม่ต้องลงชื่อเข้าใช้** (เพจแบบกำหนดเองของพอร์ทัลหรือเพจสแตติกแบบสแตนด์อโลน)
- [ ] ลิงก์ทั้งสองปรากฏบนการ์ดลงชื่อเข้าใช้พอร์ทัล/ส่วนท้าย
- [ ] การเปลี่ยนเส้นทาง URI ตรงกับโดเมนจริงและเป็น https
- [ ] เผยแพร่แอป → ส่งเพื่อตรวจสอบ; ข้อจำกัดผู้ใช้ทดสอบจะถูกยกเลิกหลังจากได้รับการอนุมัติ
- [ ] การตั้งค่า OAuth ของแผง: Enable=true โดยกรอกรหัสไคลเอ็นต์/ข้อมูลลับ (เปิดใช้งานปลายทางของผู้ให้บริการแล้ว:true)

## 6. การเปรียบเทียบ GitHub

แอป GitHub OAuth มีสถานะการเผยแพร่หรือขั้นตอนการตรวจสอบ **ไม่มี**: สร้างแอป ตั้งค่าการโทรกลับ URL และแอปจะทำงานแบบสาธารณะ
โปรดทราบว่าแอป OAuth ของ GitHub ยอมรับการโทรกลับการอนุญาต **หนึ่งรายการเท่านั้น** URL (การจับคู่คำนำหน้า); สำหรับหลายโดเมน
ใช้แนวทาง multi-URI จาก Google หรือสร้างแอปแยกต่างหากต่อโดเมนพอร์ทัล
