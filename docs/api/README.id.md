<!-- source: README.md | locale: id | updated: 2026-10-23 -->
# Dokumentasi CoreFusion API

> Bahasa: [简体中文](README.md) · [English](README.en.md) · [繁體中文](README.zh-TW.md) · [Русский](README.ru.md) · [日本語](README.ja.md) · [한국어](README.ko.md) · [Deutsch](README.de.md) · [Français](README.fr.md) · [Español](README.es.md) · [Português](README.pt.md) · [Türkçe](README.tr.md) · Bahasa Indonesia · [ไทย](README.th.md) · [Tiếng Việt](README.vi.md) · [العربية](README.ar.md) · [فارسی](README.fa.md)

Dokumentasi API untuk **integrator eksternal, pengembangan sekunder, dan klien pihak ketiga**. Semua jalur bersifat relatif terhadap domain penerapan.

| Dokumen | Penonton | Konten |
|---|---|---|
| [langganan.md](./subscription.id.md) | Pelanggan / pengembang klien | Titik akhir langganan (`/sub/:token`): format, header, kode status, contoh |
| [portal-api.md](README.id.md) | Frontend portal/klien pihak ketiga | Portal klien API (`/api/v1/client-auth/*`, `/api/v1/public/*`): pendaftaran, masuk, paket, pesanan, saldo, tiket |
| [biaya-panggilan balik.md](./charge-callback.id.md) | Penyedia pembayaran | Penandatanganan dan verifikasi panggilan balik pembayaran |
| [admin-api.id.md](./admin-api.id.md) | Pengembangan / otomatisasi panel | Tabel rute admin lengkap API (`/api/v1/*`, dikelompokkan berdasarkan modul) |

---

## 1. Port dan URL dasar

Panel dapat mengekspos tiga mesin secara bersamaan, berbagi satu database:

| Mesin | Akses bawaan | Tujuan |
|---|---|---|
| Panel utama | `https://<domain>/api/v1` | Backend admin + portal + langganan |
| Mesin berlangganan (port khusus) | `https://<domain>:<port>/:token` | Pengambilan langganan konkurensi tinggi, tanpa awalan `/sub` |
| Mesin portal (port khusus) | `https://<portal-domain>[:port]/api/v1` | Portal khusus klien, halaman di `/` |

> Proksi terbalik harus meneruskan jalur halaman dan `/api/v1`; `X-Forwarded-Proto` / `X-Forwarded-Host` merasa terhormat dapat memulihkan skema dan domain sebenarnya.

## 2. Otentikasi (tiga jenis, tidak dapat dipertukarkan)

| Penulis | Kredensial | Berlaku untuk |
|---|---|---|
| **Otorisasi Admin** | `Authorization: Bearer <admin_token>` (dari `/api/v1/auth/login`) | Admin API |
| **Otentikasi klien** | `Authorization: Bearer <client_token>` (dari `/api/v1/client-auth/login`) | Portal API |
| **Token berlangganan** | Token di URL: `/sub/<token>` | Pengambilan langganan, tanpa header |

- Token akses klien bertahan 24 jam, menyegarkan token 7 hari; JWT berisi `tv` (token_version), jadi logout/keluar paksa akan membatalkan semua token lama.
- Peran Admin: `admin` / `operator` / `super_admin`; beberapa operasi tulis memerlukan `RequireRole(RoleAdmin)` atau lebih tinggi.

## 3. Format tanggapan

Berhasil (sebagian besar titik akhir):

```json
{ "code": 0, "data": { ... }, "message": "ok" }
```

Daftar titik akhir (beberapa) mengembalikan `{ "items": [...], "total": N }` atau array kosong (dicatat per titik akhir di bawah).

Kesalahan (HTTP 4xx/5xx):

```json
{ "code": "ERROR_CODE", "message": "human-readable description", "request_id": "…", "can_retry": false }
```

**Jumlahnya adalah bilangan bulat dalam sen**: `990` = ¥9,90 (`currency: "CNY"`).
**Lalu lintas dalam byte**: permintaan dan respons menggunakan byte.

## 4. Batas tarif dan mode pemeliharaan

- Titik akhir publik (`/api/v1/public/*`, `/sub/*`, pendaftaran/masuk) memiliki tarif terbatas;
- Proses masuk juga menggunakan `LoginRateLimit`, pengambilan langganan menggunakan `SubscribeRateLimit`;
- Dalam mode pemeliharaan, titik akhir publik dan langganan mengembalikan pesan pemeliharaan; admin API tidak terpengaruh.

## 5. Rute halaman depan (bukan API)

Rute ini mengembalikan aset HTML/statis dan berada di luar dokumen API; terdaftar sehingga mereka dapat dikecualikan selama integrasi:

| Metode | Jalan | Deskripsi |
|---|---|---|
| GET | `/` | Panel admin SPA (setelah masuk) |
| GET | `/p` | Portal HTML (`/` pada mesin portal khusus) |
| GET | `/:token` | Pengambilan langganan pada port khusus (tanpa awalan `/sub`, semantik yang sama seperti `/sub/:token`) |
| GET | `/static/:file` | Aset statis frontend |

## 6. Sumber kebenaran

`internal/api/router/router.go` (746 rute, 743 didokumentasikan). Tabel rute di [admin-api.md](README.id.md) dihasilkan dari kode; jika tidak sesuai dengan kode, **kode menang**.
