<!-- source: portal-api.md | locale: id | updated: 2026-10-23 -->
# Referensi Portal API (Klien / Portal API)

> Bahasa: [简体中文](portal-api.md) · [English](portal-api.en.md) · [繁體中文](portal-api.zh-TW.md) · [Русский](portal-api.ru.md) · [日本語](portal-api.ja.md) · [한국어](portal-api.ko.md) · [Deutsch](portal-api.de.md) · [Français](portal-api.fr.md) · [Español](portal-api.es.md) · [Português](portal-api.pt.md) · [Türkçe](portal-api.tr.md) · Bahasa Indonesia · [ไทย](portal-api.th.md) · [Tiếng Việt](portal-api.vi.md) · [العربية](portal-api.ar.md) · [فارسی](portal-api.fa.md)

> Referensi antarmuka **lengkap** untuk frontend portal (`/p` atau port portal khusus) dan klien pihak ketiga.
> Definisi rute: `internal/api/router/router.go` (`clientAuthGroup` / `clientAuthProtected` / `/api/v1/public/*`).

## 1. Konvensi umum

### 1.1 Basis URL

| Penempatan | Basis URL |
|---|---|
| Mesin panel utama | `https://<panel-domain>/api/v1` (halaman portal di `/p`) |
| Port portal khusus | `https://<portal-domain>[:port]/api/v1` (halaman portal di `/`) |
| Domain portal khusus + proksi terbalik | Sama seperti di atas; proksi harus meneruskan jalur halaman dan `/api/v1` |

### 1.2 Otentikasi

- Setelah masuk, kirim `Authorization: Bearer <access_token>`;
- Akses token bertahan 24 jam, segarkan token 7 hari;
- JWT berisi `tv` (token_version): logout/keluar paksa menambahnya dan membatalkan semua token lama
 (`clients.token_version`, lihat `internal/service/clientauth/service.go`);
- Setiap titik akhir yang dilindungi melewati `ClientAuthMiddleware` (pemeriksaan versi + penjaga mode pemeliharaan).

### 1.3 Amplop tanggapan

Sebagian besar titik akhir mengembalikan `{ "code": 0, "data": ..., "message": "..." }`;
beberapa titik akhir daftar mengembalikan `{ "items": [...], "total": N }` atau array kosong (dicatat per titik akhir di bawah).
Kesalahan mengembalikan `{ "code": "ERROR_CODE", "message": "...", "request_id": "...", "can_retry": ... }`
(HTTP 4xx/5xx).

### 1.4 Jumlah satuan

Bidang pesanan/saldo adalah **sen bilangan bulat**: `990` = ¥9,90 (`currency: "CNY"`).

### 1.5 Contoh nyata (masuk → ambil profil)

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

Pada 401, panggil `POST /api/v1/client-auth/refresh` sekali dan coba lagi; jika masih gagal, hapus sesi lokal (templat portal sudah melakukan ini).

---

## 2. Titik akhir publik (tidak ada proses masuk)

| Metode | Jalan | Deskripsi |
|---|---|---|
| GET | `/p` (`/` pada mesin portal khusus) | Portal HTML (yang dirender oleh server + runtime Vue) |
| GET | `/api/v1/public/portal-config` | Konfigurasi runtime portal: nama situs, warna tema, navigasi, modul, paket bahasa, `registration_enabled`, `regions` (sumber wilayah masuk), `privacy_policy_url` / `terms_of_service_url` / `privacy_policy_content` / `terms_of_service_content` (dokumen kepatuhan; konten dipetakan per bahasa; halaman mandiri `/p/legal/*` ✓) |
| GET | `/api/v1/public/settings` | Pengaturan publik (saat ini `registration_enabled`) |
| GET | `/api/v1/public/subscription-endpoints` | Daftar titik akhir langganan (array kosong) |
| GET | `/api/v1/public/articles` | Artikel publik (hanya diterbitkan) |
| GET | `/api/v1/public/articles/:slug` | Detil artikel |
| GET | `/api/v1/public/client-configs` | Konfigurasi pengunduhan klien |
| GET | `/sub/:token` | Pengiriman langganan (`format` mengembalikan base64_uri / raw_uri / mihomo_yaml / singbox_json / xray_json, dll.; dicabut/hilang → 404, ditangguhkan/kedaluwarsa/tidak aktif → 403. Lihat [subscription.md](portal-api.id.md)) |
| GET | `/api/v1/client-auth/oauth/providers` | Visibilitas penyedia OAuth: `{"data":{"providers":[{"provider":"github","enabled":true},...]}}` |
| GET | `/api/v1/system/maintenance` | Status pemeliharaan (publik). Portal menampilkan halaman pemeliharaan saat diaktifkan |

> Semua ini termasuk penjagaan pemeliharaan dan pembatasan tarif API publik.

## 3. Otentikasi / OAuth

| Metode | Jalan | Penulis | Deskripsi |
|---|---|---|---|
| POST | `/api/v1/client-auth/register` | — | Daftar. Tubuh: `{email, password, referral_code?}`. Mengembalikan `{access_token, refresh_token, client}`; tingkat pendaftaran terbatas |
| POST | `/api/v1/client-auth/login` | — | Masuk. Isi: `{email, password}`. Respon yang sama; menerbitkan JWT dengan `token_version` |
| POST | `/api/v1/client-auth/refresh` | — | Segarkan. Tubuh: `{refresh_token}`. Mengembalikan token akses/penyegaran baru (versi dicentang; token penyegaran lama langsung mati setelah logout) |
| POST | `/api/v1/client-auth/logout` | ✓ | Logout: pencabutan sisi server (`token_version +1`) → semua token lama untuk klien tidak valid |
| POST | `/api/v1/client-auth/rotate-token` | ✓ | Putar token berlangganan |
| POST | `/api/v1/client-auth/ticket` | ✓ | Menerbitkan **tiket berumur pendek satu kali** (untuk alur `window.open` seperti “unduh paket log” yang headernya tidak dapat disetel) |
| GET | `/api/v1/client-auth/oauth/:provider` | — | Mulai OAuth: 307 → persetujuan GitHub/Google (`state` anti-CSRF, 10 menit); penyedia yang dinonaktifkan mengembalikan 400 |
| GET | `/api/v1/client-auth/oauth/:provider/callback` | — | Panggilan balik OAuth: tukar kode → buat/tautkan klien → terbitkan versi JWT → 302 kembali ke portal (`/<portalPath>?access_token=...&refresh_token=...`) |

> Callback OAuth adalah **pengalihan browser** (bukan panggilan API): setelah login berhasil, portal akan membaca token
> dari URL di `onMounted` (atau melalui `postMessage` asal yang sama, lihat `handleOAuthCallback` di templat portal `web`).

## 4. Akun

| Metode | Jalan | Deskripsi |
|---|---|---|
| GET | `/api/v1/client-auth/me` | Klien saat ini: `{id, email, plan_id, plan_name, status, quota_bytes, used_bytes, upload_bytes, download_bytes, expires_at, ip_whitelist, device_limit, sub_token, sub_status, created_at, ...}` (amplop `data`) |
| POST | `/api/v1/client-auth/change-password` | Tubuh: `{old_password, new_password}` |
| GET | `/api/v1/client-auth/checktime` | Waktu server (sinkronisasi jam klien) |
| GET | `/api/v1/client-auth/connection-info` | Parameter koneksi paket saat ini: `{concurrent, device_limit, protocol, speed_limit, tls, transport}` |

## 5. Langganan

| Metode | Jalan | Deskripsi |
|---|---|---|
| POST | `/api/v1/client-auth/reset-subscription` | Setel ulang langganan; ini memutar `sub_token` |

> Tautan berlangganan dibuat di frontend dari `sub_token` yang dikembalikan oleh `/me` (`/sub/:token`); tidak ada
> pisahkan lagi titik akhir `subscription-link` / `subscription-formats`.

## 6. Node

| Metode | Jalan | Deskripsi |
|---|---|---|
| GET | `/api/v1/client-auth/nodes` | Node yang terlihat pada paket saat ini: `{items:[...]}` (bidang tampilan seperti nama/protokol/transportasi/wilayah) |
| GET | `/api/v1/client-auth/nodes/:id/config` | Konfigurasi simpul penuh |
| GET | `/api/v1/client-auth/nodes/:id/link` | Bagikan tautan (vless:// dll.) |
| GET | `/api/v1/client-auth/nodes/:id/qrcode` | Gambar kode QR |

## 7. Simpan / rencana / pesanan

| Metode | Jalan | Deskripsi |
|---|---|---|
| GET | `/api/v1/client-auth/plans` | Daftar paket (array kosong): `{id, name, description, category, sort_order, variants:[{id, name, billing_cycle, duration_days, price(cents), original_price, currency, quota_bytes, ...}]}` |
| GET | `/api/v1/client-auth/plans/:id` | Detil rencana |
| GET | `/api/v1/client-auth/traffic-packs` | Daftar paket lalu lintas |
| POST | `/api/v1/client-auth/traffic-packs/buy` | Beli paket lalu lintas |
| GET | `/api/v1/client-auth/orders` | Pesanan paket saya (diberi nomor halaman) |
| POST | `/api/v1/client-auth/orders` | Buat pesanan (beli/perpanjang varian paket) |
| GET | `/api/v1/client-auth/orders/:id` | Detil pesanan |
| GET | `/api/v1/client-auth/invoices` | Faktur |
| GET | `/api/v1/client-auth/invoices/:id` | Detail faktur |

## 8. Saldo / isi ulang

| Metode | Jalan | Deskripsi |
|---|---|---|
| GET | `/api/v1/client-auth/balance` | `{data:{client_id, balance(cents), frozen}}` |
| GET | `/api/v1/client-auth/balance/transactions` | Transaksi saldo (`type`: biaya/konsumsi/pengembalian dana; diberi nomor halaman) |
| GET | `/api/v1/client-auth/charge/sites` | Situs isi ulang yang tersedia (`ListChargeSites`: hanya diaktifkan) |
| POST | `/api/v1/client-auth/charge/create` | Buat pesanan isi ulang (pengalihan ke pembayaran pihak ketiga) |
| GET | `/api/v1/client-auth/charge/orders` | Isi ulang daftar pesanan |
| GET | `/api/v1/client-auth/charge/orders/:id` | Isi ulang detail pesanan |
| GET | `/api/v1/client-auth/charge/status/:trade_no` | Status pembayaran jajak pendapat berdasarkan trade_no |
| POST | `/api/v1/client-auth/charge/redeem-card` | Tukarkan kartu isi ulang |

> Menghapus klien **bertingkat** ke saldo dan transaksinya (lihat `DeleteClient` di `internal/service/client/service.go`).

## 9. Hadiah / riwayat lalu lintas

| Metode | Jalan | Deskripsi |
|---|---|---|
| GET | `/api/v1/client-auth/gifts` | Hadiah yang dapat diklaim (array kosong; `type`: hari/byte) |
| POST | `/api/v1/client-auth/gifts/:id/claim` | Klaim |
| GET | `/api/v1/client-auth/traffic-history` | Riwayat lalu lintas bulanan: `{data:{history:[{period_start,total_bytes,...}], months}}` |

## 10. Tiket

| Metode | Jalan | Deskripsi |
|---|---|---|
| GET | `/api/v1/client-auth/tickets` | Daftar tiket: `{items,total}` |
| POST | `/api/v1/client-auth/tickets` | Buat tiket |
| GET | `/api/v1/client-auth/tickets/:id` | Detail tiket (dengan balasan) |
| POST | `/api/v1/client-auth/tickets/:id/reply` | Membalas |
| POST | `/api/v1/client-auth/tickets/:id/close` | Tutup |

## 11. Artikel/panduan

| Metode | Jalan | Deskripsi |
|---|---|---|
| GET | `/api/v1/client-auth/articles` | Daftar artikel (dengan filter kategori) |
| GET | `/api/v1/client-auth/articles/:slug` | Detil artikel |

## 12. Rujukan (dinonaktifkan secara default; dapat diaktifkan di editor portal)

| Metode | Jalan | Deskripsi |
|---|---|---|
| GET | `/api/v1/client-auth/referral/code` | Kode dan tautan referensi saya |
| GET | `/api/v1/client-auth/referral/stats` | Statistik komisi (`commission_rate`, `available_commission`, `min_withdraw_amount`) |
| GET | `/api/v1/client-auth/referral/records` | Catatan rujukan |
| GET | `/api/v1/client-auth/referral/withdrawals` | Catatan penarikan |
| POST | `/api/v1/client-auth/referral/withdraw` | Minta penarikan |

## 13. Akun bersama

| Metode | Jalan | Deskripsi |
|---|---|---|
| GET | `/api/v1/client-auth/shared-accounts` | Akun bersama yang tersedia (streaming, dll.) |
| GET | `/api/v1/client-auth/shared-accounts/:id` | Detil |
| POST | `/api/v1/client-auth/shared-accounts/:id/apply` | Minta akses |
| POST | `/api/v1/client-auth/shared-accounts/:id/repair` | Perbaikan sekali klik |

## 14. Bundel log (diagnostik pelanggan)

| Metode | Jalan | Deskripsi |
|---|---|---|
| GET | `/api/v1/client-auth/logs/types` | Jenis log yang dapat dibundel |
| GET | `/api/v1/client-auth/logs/packages` | Paket log saya |
| POST | `/api/v1/client-auth/logs/package` | Membuat pekerjaan bundling log (hanya node entri) |
| GET | `/api/v1/client-auth/logs/packages/:id` | Status pekerjaan (siap/sedang diproses/gagal) |
| GET | `/api/v1/client-auth/logs/packages/:id/download` | Unduh (pilih `POST /ticket` dan tambahkan `?ticket=`) |
| DELETE | `/api/v1/client-auth/logs/packages/:id` | Hapus bundel log |

## 15. Pengaturan keamanan / daftar IP yang diizinkan / batalkan pemblokiran mandiri

| Metode | Jalan | Deskripsi |
|---|---|---|
| GET/PUT | `/api/v1/client-auth/settings` | Pengaturan klien: `{ip_whitelist, device_limit}` (PUT) |
| GET | `/api/v1/client-auth/settings/whitelist` | Daftar yang diizinkan (array kosong) |
| DELETE | `/api/v1/client-auth/settings/whitelist/:id` | Hapus entri daftar yang diizinkan |
| POST | `/api/v1/client-auth/settings/auto-add-ip` | Tambahkan IP saat ini ke daftar yang diizinkan |
| POST | `/api/v1/client-auth/self-unban` | Pembatalan larangan layanan mandiri (upaya dibatasi oleh kebijakan) |

> Daftar IP yang diizinkan **hanya berlaku untuk URL langganan** (`/sub/:token`); itu tidak memblokir masuk portal/OAuth atau
> titik akhir autentikasi klien lainnya. Jika pengguna tidak memiliki daftar yang diizinkan atau tidak memiliki langganan URL, akses selalu diizinkan. Itu
> daftar yang diizinkan adalah pengaturan per pengguna; tidak ada peralihan global.

**Aliran pembatalan pemblokiran mandiri**: hanya tersedia untuk `auto_quota` (larangan otomatis atas kelebihan lalu lintas) ketika sisi admin
`self_unban.enabled` aktif. Masuk (kata sandi atau panggilan balik OAuth) mengembalikan `require_self_unban=true` + `self_unban_token`
(typ=self_unban, valid 15 menit, **hanya** dapat digunakan di `POST /self-unban`; titik akhir lainnya menampilkan 403/`SELF_UNBAN_TOKEN_SCOPE`).
Portal kemudian menampilkan kartu pembatalan pelarangan; setelah berhasil membatalkan pemblokiran, pengguna masuk lagi. Panggilan balik OAuth mengirimkan ini melalui
fragmen URL `#require_self_unban=1&self_unban_token=..&self_unban_hint=..`.

## 16. Kode kesalahan (kutipan)

| kode | Skenario |
|---|---|
| `INVALID_TOKEN` | Segarkan token tidak valid/kedaluwarsa/versi tidak cocok |
| `INVALID_REQUEST` | Validasi parameter gagal |
| `NOT_FOUND` | Sumber daya hilang (tiket/pesanan/entri daftar yang diizinkan, dll.) |
| `RELOAD_FAILED` / `SYNC_FAILED` | Sinkronisasi konfigurasi proxy (sisi Admin) gagal |
| 401 + `errClientTokenRevoked` | token_version mismatch (keluar/ditendang) → klien harus menyegarkan; jika gagal, hapus token lokal |

## 17. Dokumen kepatuhan (kebijakan privasi / persyaratan layanan)

Tautan “Kebijakan Privasi · Ketentuan Layanan” di bagian bawah kartu masuk (sama seperti situs biasa; mengklik akan membuka konten):

- **Konfigurasi**: editor portal → Pengaturan → Umum → Dokumen kepatuhan (**edit konten per bahasa**);
 Anda juga dapat `PUT /api/v1/portal/active` dengan bidang terstruktur `privacy_policy_url` / `privacy_policy_content` /
 `terms_of_service_url` / `terms_of_service_content` (nihil = tidak berubah; konten adalah peta `{"zh-CN":"...","en":"..."}`).
- **Halaman mandiri yang dapat diakses** (dirender oleh server, mudah untuk crawler verifikasi Google; mendukung `?lang=xx`):
  - Domain panel utama: `GET /p/legal/privacy`, `GET /p/legal/terms`
  - Domain portal khusus (penyebaran root): `GET /legal/privacy`, `GET /legal/terms`
- **Logika tampilan**: URL atau konten, **URL menang** (terbuka secara eksternal di tab baru); jika konten disediakan, klik
 kartu masuk menampilkan Penurunan harga dalam dialog (`renderMd`) menggunakan bahasa portal saat ini.
- Dikirim saat runtime melalui `/api/v1/public/portal-config`; komponen masuk dikonsumsi
 `ctx.privacyUrl / termsUrl / privacyContent / termsContent / openLegal / hasLegalLinks`.
- Verifikasi OAuth Google mengharuskan tautan ini dapat diakses publik → gunakan tautan mandiri di atas di
 layar persetujuan; lihat `docs/oauth-google-verification.md` (berisi templat kebijakan privasi / persyaratan).

## 18. Contoh integrasi

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

Pada 401, panggil `POST /refresh` sekali dan coba lagi; jika masih gagal, hapus sesi lokal
(`api()` template portal sudah mengimplementasikan ini).
