<!-- source: THIRD_PARTY_LICENSES.md | locale: id | updated: 2026-11-19 -->
# Komponen dan Lisensi Pihak Ketiga

> Bahasa: [简体中文](THIRD_PARTY_LICENSES.md) · [English](THIRD_PARTY_LICENSES.en.md) · [繁體中文](THIRD_PARTY_LICENSES.zh-TW.md) · [Русский](THIRD_PARTY_LICENSES.ru.md) · [日本語](THIRD_PARTY_LICENSES.ja.md) · [한국어](THIRD_PARTY_LICENSES.ko.md) · [Deutsch](THIRD_PARTY_LICENSES.de.md) · [Français](THIRD_PARTY_LICENSES.fr.md) · [Español](THIRD_PARTY_LICENSES.es.md) · [Português](THIRD_PARTY_LICENSES.pt.md) · [Türkçe](THIRD_PARTY_LICENSES.tr.md) · Bahasa Indonesia · [ไทย](THIRD_PARTY_LICENSES.th.md) · [Tiếng Việt](THIRD_PARTY_LICENSES.vi.md) · [العربية](THIRD_PARTY_LICENSES.ar.md) · [فارسی](THIRD_PARTY_LICENSES.fa.md)

CoreFusion menggunakan komponen sumber terbuka berikut. File ini mencantumkan komponen utama dan lisensi; **daftar lengkap ditentukan oleh `go.mod` / `go.sum`, `web/admin/package.json` dan file LISENSI masing-masing modul**.

Saat mendistribusikan biner proyek ini, simpan file ini, proyek `LICENSE`, dan pemberitahuan hak cipta/lisensi dari semua komponen pihak ketiga.

## 1. Dependensi Go dikompilasi ke dalam biner (utama)

| Komponen | Lisensi | Tujuan |
|---|---|---|
| `github.com/xtls/xray-core` | MPL-2.0 | Penguraian konfigurasi Xray, perintah gRPC, definisi protokol |
| `github.com/xtls/reality` | MPL-2.0 | Dukungan protokol REALITY |
| `github.com/sagernet/sing` | **GPL-3.0-or-later** | Terhubung secara tidak langsung melalui xray-core |
| `github.com/sagernet/sing-shadowsocks` | **GPL-3.0-or-later** | Terhubung secara tidak langsung melalui xray-core (Shadowsocks-2022) |
| `github.com/go-sql-driver/mysql` | MPL-2.0 | Pengemudi MySQL |
| `github.com/hashicorp/golang-lru/v2` | MPL-2.0 | cache LRU |
| `github.com/go-acme/lego/v5` | MIT | Penerbitan sertifikat ACME |
| `github.com/gin-gonic/gin` | MIT | Kerangka kerja HTTP |
| `gorm.io/gorm` | MIT | ORM |
| `gorm.io/driver/mysql` | MIT | Driver MySQL untuk GORM |
| `gorm.io/driver/postgres` | MIT | Driver PostgreSQL untuk GORM |
| `github.com/jackc/pgx/v5` | MIT | Pengemudi PostgreSQL |
| `github.com/qist/sqlite` | MIT | Driver SQLite (database default untuk panel / Agen) |
| `github.com/aws/aws-sdk-go-v2` | Apache-2.0 | Pencadangan S3, Route53 DNS-01 |
| `google.golang.org/grpc` | Apache-2.0 | Komunikasi agen gRPC |
| `google.golang.org/protobuf` | BSD-3-Clause | Waktu proses Protobuf |
| `golang.org/x/crypto` | BSD-3-Clause | Primitif kriptografi |
| `golang.org/x/oauth2` | BSD-3-Clause | OAuth2 / Google EAB |
| `golang.org/x/sys` | BSD-3-Clause | Panggilan sistem (pembatasan kecepatan nft/tc, manajemen proses) |
| `github.com/golang-jwt/jwt/v5` | MIT | JWT |
| `github.com/google/uuid` | BSD-3-Clause | UUID |
| `github.com/go-webauthn/webauthn` | BSD-3-Clause | Passkey / WebAuthn masuk |
| `github.com/pquerna/otp` | Apache-2.0 | Otentikasi dua faktor TOTP |
| `github.com/jlaffaye/ftp` | ISC | Unggahan cadangan FTP |
| `gopkg.in/yaml.v3` | MIT / Apache-2.0 | Penguraian konfigurasi YAML |

> ⚠ **Komponen GPL-3.0**: `sing` dan `sing-shadowsocks` adalah GPL-3.0-atau-yang lebih baru. Mereka ditautkan secara statis ke dalam biner panel dan Agen, jadi **mendistribusikan binari memerlukan penyediaan Sumber yang Sesuai** (lihat bagian 3 dari [LEGAL.md](./LEGAL.md)).

## 2. Ketergantungan frontend (dikompilasi ke dalam biner panel)

UI admin dibangun dengan Vite dan tertanam ke dalam biner panel melalui `go:embed`, sehingga komponen berikut juga didistribusikan dengan biner:

| Komponen | Lisensi | Tujuan |
|---|---|---|
| Vue 3 / Vue Router / Pinia / Element Plus / axios / vue-i18n / marked / js-yaml / qrcode / Monaco Editor | MIT | UI Admin, perutean dan status, i18n, rendering penurunan harga, kode QR, dan editor kode |
| DOMPurify | Apache-2.0 / MPL-2.0 | Sanitasi XSS sebelum merender Markdown / HTML |

## 3. Program mandiri yang dipanggil saat runtime (tidak didistribusikan dalam repositori ini)

Panel/Agen dapat mengunduh, menginstal, atau menjalankan program mandiri berikut saat runtime. Mereka berinteraksi dengan proyek ini melalui proses/antarmuka, namun jika Anda mengirimkannya bersama kiriman Anda, Anda tetap harus mematuhi lisensi mereka:

| Program | Lisensi | Catatan |
|---|---|---|
| Xray-core | MPL-2.0 | Inti proxy, proses terpisah |
| sing-box | **GPL-3.0-or-later** | Inti proxy, proses terpisah; menyediakan sumber yang sesuai saat mendistribusikan |
| Caddy | Apache-2.0 | Proksi terbalik / TLS |
| Nginx | BSD-2-Clause | Proksi terbalik (diinstal sistem, jika digunakan) |

> Hysteria2 dan TUIC adalah **protokol yang dibawa oleh sing-box**, bukan inti atau biner yang berdiri sendiri, sehingga tidak dicantumkan secara terpisah.

## 4. File data

| Data | Sumber / lisensi |
|---|---|
| `geoip.dat` / `geosite.dat` | Seperti yang dinyatakan oleh repositori sumber data (umumnya GPL-3.0 atau CC-BY-SA) |
| GeoLite2 (`GeoLite2-ASN.mmdb`, dll.) | MaxMind GeoLite2 EULA / CC BY-SA 4.0; memverifikasi lisensi sebelum penggunaan komersial |
| `country.mmdb` | Seperti yang dinyatakan oleh sumber datanya |

## 5. Rekomendasi kepatuhan

- Saat mengirimkan biner, sertakan: `LICENSE`, file ini, dan teks lisensi atau akuisisi URL dari setiap komponen GPL/MPL.
- Jika Anda memodifikasi file sumber komponen MPL-2.0 seperti `xray-core`, publikasikan sumber file yang dimodifikasi.
- Jika Anda mendistribusikan program GPL seperti sing-box bersama produk Anda, berikan kepada penerima sumber lengkap yang sesuai dengan versi tersebut (atau penawaran tertulis yang efektif).
- Merek dagang dan nama: jangan menggunakan nama atau logo proyek hulu dengan cara yang menyiratkan dukungan resmi.
