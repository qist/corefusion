<!-- source: THIRD_PARTY_LICENSES.md | locale: th | updated: 2026-10-23 -->
# ส่วนประกอบและใบอนุญาตของบุคคลที่สาม

> ภาษา: [简体中文](THIRD_PARTY_LICENSES.md) · [English](THIRD_PARTY_LICENSES.en.md) · [繁體中文](THIRD_PARTY_LICENSES.zh-TW.md) · [Русский](THIRD_PARTY_LICENSES.ru.md) · [日本語](THIRD_PARTY_LICENSES.ja.md) · [한국어](THIRD_PARTY_LICENSES.ko.md) · [Deutsch](THIRD_PARTY_LICENSES.de.md) · [Français](THIRD_PARTY_LICENSES.fr.md) · [Español](THIRD_PARTY_LICENSES.es.md) · [Português](THIRD_PARTY_LICENSES.pt.md) · [Türkçe](THIRD_PARTY_LICENSES.tr.md) · [Bahasa Indonesia](THIRD_PARTY_LICENSES.id.md) · ไทย · [Tiếng Việt](THIRD_PARTY_LICENSES.vi.md) · [العربية](THIRD_PARTY_LICENSES.ar.md) · [فارسی](THIRD_PARTY_LICENSES.fa.md)

CoreFusion ใช้ส่วนประกอบโอเพ่นซอร์สต่อไปนี้ ไฟล์นี้แสดงรายการส่วนประกอบหลักและใบอนุญาต **รายการทั้งหมดกำหนดโดย `go.mod` / `go.sum` และไฟล์ LICENSE ของแต่ละโมดูล**

เมื่อแจกจ่ายไบนารีของโปรเจ็กต์นี้ ให้เก็บไฟล์นี้ โปรเจ็กต์ `LICENSE` และประกาศเกี่ยวกับลิขสิทธิ์/ใบอนุญาตของส่วนประกอบของบุคคลที่สามทั้งหมด

## 1. Go dependencies ที่คอมไพล์เป็นไบนารี่ (หลัก)

| ส่วนประกอบ | ใบอนุญาต | วัตถุประสงค์ |
|---|---|---|
| `github.com/xtls/xray-core` | เอ็มพีแอล-2.0 | การแยกวิเคราะห์การกำหนดค่า Xray, คำสั่ง gRPC, คำจำกัดความของโปรโตคอล |
| `github.com/xtls/reality` | เอ็มพีแอล-2.0 | รองรับโปรโตคอล REALITY |
| `github.com/sagernet/sing` | **GPL-3.0 หรือใหม่กว่า** | เชื่อมโยงทางอ้อมผ่าน xray-core |
| `github.com/sagernet/sing-shadowsocks` | **GPL-3.0 หรือใหม่กว่า** | เชื่อมโยงทางอ้อมผ่าน xray-core (Shadowsocks-2022) |
| `github.com/go-sql-driver/mysql` | เอ็มพีแอล-2.0 | ไดรเวอร์ MySQL |
| `github.com/hashicorp/golang-lru/v2` | เอ็มพีแอล-2.0 | แคช LRU |
| `github.com/go-acme/lego/v4` | เอ็มไอที | การออกใบรับรอง ACME |
| `github.com/gin-gonic/gin` | เอ็มไอที | เฟรมเวิร์ก HTTP |
| `gorm.io/gorm` | เอ็มไอที | ออม |
| `github.com/aws/aws-sdk-go-v2` | อาปาเช่-2.0 | การสำรองข้อมูล S3, Route53 DNS-01 |
| `google.golang.org/grpc` | อาปาเช่-2.0 | การสื่อสารตัวแทน gRPC |
| `google.golang.org/protobuf` | BSD-3-ข้อ | รันไทม์ของ Protobuf |
| `golang.org/x/crypto` | BSD-3-ข้อ | พื้นฐานการเข้ารหัส |
| `golang.org/x/oauth2` | BSD-3-ข้อ | OAuth2 / Google EAB |
| `github.com/golang-jwt/jwt/v5` | เอ็มไอที | JWT |
| `github.com/google/uuid` | BSD-3-ข้อ | UUID |

> ⚠ **ส่วนประกอบ GPL-3.0**: `sing` และ `sing-shadowsocks` เป็น GPL-3.0 หรือใหม่กว่า ไบนารีเหล่านี้เชื่อมโยงแบบคงที่กับพาเนลและไบนารีของตัวแทน ดังนั้น **การกระจายไบนารีจำเป็นต้องจัดเตรียมแหล่งที่มาที่สอดคล้องกัน** (ดูส่วนที่ 3 ของ [LEGAL.md](THIRD_PARTY_LICENSES.th.md))

## 2. โปรแกรมแบบสแตนด์อโลนที่เรียกใช้ขณะรันไทม์ (ไม่ได้แจกจ่ายในพื้นที่เก็บข้อมูลนี้)

แผงควบคุม / ตัวแทนสามารถดาวน์โหลด ติดตั้ง หรือเรียกใช้โปรแกรมสแตนด์อโลนต่อไปนี้ขณะรันไทม์ พวกเขาโต้ตอบกับโปรเจ็กต์นี้ผ่านกระบวนการ/อินเทอร์เฟซ แต่หากคุณจัดส่งให้พวกเขาพร้อมกับสิ่งที่ส่งมอบ คุณยังคงต้องปฏิบัติตามใบอนุญาตของพวกเขา:

| โปรแกรม | ใบอนุญาต | หมายเหตุ |
|---|---|---|
| Xray-คอร์ | เอ็มพีแอล-2.0 | แกนพร็อกซี กระบวนการแยกต่างหาก |
| sing-box | **GPL-3.0 หรือใหม่กว่า** | แกนพร็อกซี กระบวนการแยก ระบุแหล่งที่มาที่เกี่ยวข้องเมื่อเผยแพร่ |
| แคดดี้ | อาปาเช่-2.0 | ย้อนกลับพร็อกซี / TLS |
| Nginx | BSD-2-ข้อ | Reverse proxy (ติดตั้งโดยระบบ หากใช้) |

> Hysteria2 และ TUIC เป็น **โปรโตคอลที่ดำเนินการโดย sing-box** ไม่ใช่คอร์หรือไบนารีแบบสแตนด์อโลน ดังนั้นจึงไม่ได้แสดงรายการแยกกัน

## 3. ไฟล์ข้อมูล

| ข้อมูล | แหล่งที่มา/ใบอนุญาต |
|---|---|
| `geoip.dat` / `geosite.dat` | ตามที่ประกาศโดยแหล่งเก็บข้อมูล (โดยทั่วไปคือ GPL-3.0 หรือ CC-BY-SA) |
| GeoLite2 (`GeoLite2-ASN.mmdb` ฯลฯ) | MaxMind GeoLite2 EULA / CC BY-SA 4.0; ตรวจสอบใบอนุญาตก่อนนำไปใช้ในเชิงพาณิชย์ |
| `country.mmdb` | ตามที่แหล่งข้อมูลประกาศไว้ |

## 4. คำแนะนำการปฏิบัติตามข้อกำหนด

- เมื่อส่งมอบไบนารี ให้รวม: `LICENSE`, ไฟล์นี้ และข้อความสิทธิ์การใช้งานหรือการได้มา URL ของแต่ละองค์ประกอบ GPL/MPL
- หากคุณแก้ไขไฟล์ต้นฉบับของส่วนประกอบ MPL-2.0 เช่น `xray-core` ให้เผยแพร่แหล่งที่มาของไฟล์ที่แก้ไข
- หากคุณแจกจ่ายโปรแกรม GPL เช่น sing-box พร้อมกับผลิตภัณฑ์ของคุณ ให้จัดเตรียมแหล่งที่มาของเวอร์ชันนั้นให้ครบถ้วนแก่ผู้รับ (หรือข้อเสนอที่เป็นลายลักษณ์อักษรที่มีประสิทธิผล)
- เครื่องหมายการค้าและชื่อ: ห้ามใช้ชื่อหรือโลโก้โครงการอัปสตรีมในลักษณะที่แสดงถึงการรับรองอย่างเป็นทางการ
