<!-- source: charge-callback.md | locale: th | updated: 2026-10-23 -->
# ชาร์จโทรกลับ API

> ภาษา: [简体中文](charge-callback.md) · [English](charge-callback.en.md) · [繁體中文](charge-callback.zh-TW.md) · [Русский](charge-callback.ru.md) · [日本語](charge-callback.ja.md) · [한국어](charge-callback.ko.md) · [Deutsch](charge-callback.de.md) · [Français](charge-callback.fr.md) · [Español](charge-callback.es.md) · [Português](charge-callback.pt.md) · [Türkçe](charge-callback.tr.md) · [Bahasa Indonesia](charge-callback.id.md) · ไทย · [Tiếng Việt](charge-callback.vi.md) · [العربية](charge-callback.ar.md) · [فارسی](charge-callback.fa.md)

สำหรับ **แพลตฟอร์มการชำระเงิน / ผู้รวมเว็บไซต์**: หลังจากที่ผู้ใช้ชำระเงินบนไซต์ที่ติดตั้งแล้ว เซิร์ฟเวอร์ของแพลตฟอร์มการชำระเงินจะแจ้งเตือนแผงควบคุม

## 1. จุดสิ้นสุด

```
POST /api/v1/charge/callback/{site_id}
Content-Type: application/json
```

`site_id` คือ ID “ไซต์ที่ติดตั้ง” ฝั่งผู้ดูแลระบบ (จำนวนเต็ม) การเรียกกลับเป็นแบบเซิร์ฟเวอร์ต่อเซิร์ฟเวอร์ และใช้ **การลงนาม HMAC** บวกกับรายการอนุญาต IP เสริมแทนการตรวจสอบสิทธิ์ Bearer

## 2. ขอร่างกาย

```json
{
  "trade_no":       "CF20261010120000abcd1234",
  "site_trade_no":  "payment-platform-order-no",
  "status":         "success",
  "amount":         990,
  "currency":       "CNY",
  "timestamp":      1791640000,
  "nonce":          "random-unique-string",
  "paid_currency":  "CNY",
  "paid_amount":    990,
  "method":         "alipay",
  "exchange_rate":  1.0,
  "paid_at":        "2026-10-10 12:00:01",
  "sign":           "hex(signature)",
  "extra":          {}
}
```

| สนาม | คำอธิบาย |
|---|---|
| `trade_no` | หมายเลขธุรกรรมฝั่งแผง (ออกเมื่อมีการสร้างใบสั่ง คำนำหน้า `CF`) |
| `site_trade_no` | หมายเลขคำสั่งซื้อแพลตฟอร์มการชำระเงิน (ลงนาม) |
| `status` | สถานภาพทางธุรกิจ **ไม่ได้ลงนาม** |
| `amount` / `currency` | จำนวนคำสั่งซื้อ (จำนวนเต็มเซนต์) และสกุลเงิน (ลงนามทั้งคู่) |
| `timestamp` | ยูนิกซ์วินาที; ถูกปฏิเสธนอกหน้าต่าง (ลงนาม) |
| `nonce` | การป้องกันการเล่นซ้ำ: atomic INSERT แต่ละ nonce ประมวลผลครั้งเดียว (ลงนาม) |
| `paid_*` / `method` / `exchange_rate` / `paid_at` | ข้อมูลการชำระเงินตามจริง **ไม่ได้ลงนาม** (ความสมบูรณ์ครอบคลุมโดย `body_hash`) |
| `sign` | ลายเซ็น HMAC-SHA256 เข้ารหัสเลขฐานสิบหก (คำนวณในส่วนเนื้อหาโดยไม่มี `sign`) |
| `extra` | ฟิลด์ส่วนขยาย |

## 3. อัลกอริธึมการลงนาม (v3.2 / `version: "v1"`)

1. สร้าง **เนื้อความดิบที่ไม่มี `sign`** และคำนวณ `body_hash = SHA256(raw body bytes)` (เลขฐานสิบหกตัวพิมพ์เล็ก)
   > แผงควบคุมยังยอมรับรูปแบบเดิมที่มีการแฮชแบบเต็มรวมถึง `sign`; การบูรณาการใหม่ควรเป็นไปตามแผนงานที่บันทึกไว้
2. สร้างเพย์โหลดการลงนาม (**คีย์ตามลำดับพจนานุกรม**, ซีแมนทิกส์ Go `json.Marshal`):

```json
{
  "amount": 990,
  "body_hash": "<hash from step 1>",
  "currency": "CNY",
  "nonce": "<nonce>",
  "site_trade_no": "<site_trade_no>",
  "timestamp": 1791640000,
  "trade_no": "<trade_no>",
  "version": "v1"
}
```

3. `sign = hex( HMAC_SHA256( key=callback secret, msg=JSON bytes from step 2) )`.
   > ข้อมูลลับคือข้อมูลลับในการเรียกกลับของไซต์ที่ติดตั้งใน UI ผู้ดูแลระบบ (หลังจากการหมุน ข้อมูลลับก่อนหน้านี้จะถูกเก็บไว้สำหรับหน้าต่างการเปลี่ยนภาพเดียว)

```python
import hashlib, hmac, json

def sign_callback(body: dict, secret: str) -> str:
    # 1) body_hash first: body must not contain sign here
    body_hash = hashlib.sha256(json.dumps(body, separators=(",", ":"), ensure_ascii=False).encode()).hexdigest()
    # 2) signing payload (key order irrelevant: json.dumps(sort_keys=True))
    payload = {
        "amount": body["amount"], "body_hash": body_hash,
        "currency": body["currency"], "nonce": body["nonce"],
        "site_trade_no": body["site_trade_no"], "timestamp": body["timestamp"],
        "trade_no": body["trade_no"], "version": "v1",
    }
    msg = json.dumps(payload, separators=(",", ":"), sort_keys=True).encode()
    return hmac.new(secret.encode(), msg, hashlib.sha256).hexdigest()
```

## 4. ห่วงโซ่การตรวจสอบฝั่งแผง (กำหนดข้อผิดพลาดที่คุณได้รับ)

1. มีไซต์อยู่และโดเมนได้รับการยืนยันแล้ว → 404 / 403
2. รายการที่อนุญาต IP (หากกำหนดค่า `callback_whitelist` แล้ว IP ที่ไม่อยู่ในรายการจะถูก **ฮาร์ดบล็อก** มิฉะนั้นจะข้ามไปและตรวจสอบเฉพาะลายเซ็นเท่านั้น)
3. หน้าต่าง `timestamp` + การป้องกันการเล่นซ้ำ `nonce` (การแทรกอะตอมมิก)
4. การยืนยัน HMAC (ยอมรับทั้งความลับที่ใช้งานอยู่และความลับดั้งเดิม)
5. มีคำสั่งซื้ออยู่และเป็นของไซต์ → ตัวตน (ชำระแล้ว ส่งคืนสำเร็จ) → **การตรวจสอบการล็อคจำนวนเงินและสกุลเงิน** → การผ่านรายการธุรกรรม (บัญชีแยกประเภทยอดคงเหลือ + สถานะคำสั่งซื้อ)

กำลังประมวลผล **idempotent**: การเรียกกลับซ้ำด้วย `trade_no` เดียวกันจะไม่ถูกโพสต์สองครั้ง ความผิดปกติของลายเซ็น/อัตราจะถูกบันทึกเป็น “เหตุการณ์การโทรกลับ” ซึ่งผู้ดูแลระบบสามารถตรวจสอบและลองใหม่อีกครั้งด้วยตนเอง

## 5. คำตอบ

- สำเร็จ: 2xx (หลังจากที่แผงได้โพสต์การชำระเงินแล้ว)
- ล้มเหลว: 4xx/5xx + `{ "code": "...", "message": "..." }`; รหัสทั่วไป ได้แก่ `INVALID_SIGNATURE`, `IP_BLOCKED`, `ORDER_NOT_FOUND`, จำนวนไม่ตรงกัน ฯลฯ

> การโทรกลับทุกครั้งจะมี `X-Request-ID` (สร้างโดยแผงควบคุมหากไม่มีอยู่) บันทึกและเหตุการณ์ต่างๆ สามารถตรวจสอบย้อนกลับได้ตั้งแต่ต้นจนจบ มอบ ID นี้ให้กับผู้ดูแลระบบของคุณเมื่อแก้ไขปัญหา

## 6. การทดสอบและการแก้ไขปัญหา

- ผู้ดูแลระบบสามารถเรียก **เอกสารบูรณาการ** ของแต่ละไซต์: `GET /api/v1/charge-sites/{id}/integration-doc` (รวมตัวอย่างการโทรกลับจริง URL ของไซต์)
- เหตุการณ์การติดต่อกลับ: `GET /api/v1/charge-callback-events`; ลองอีกครั้ง: `GET /api/v1/charge-callback-retries`, `POST /api/v1/charge-callback-retries/{id}/retry`
- การหมุนเวียนข้อมูลลับ: `POST /api/v1/charge-sites/{id}/rotate-secret` (ข้อมูลลับก่อนหน้านี้ถูกเก็บไว้สำหรับหน้าต่างการเปลี่ยนแปลงเดียว)
