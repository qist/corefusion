<!-- source: admin-api.md | locale: id | updated: 2026-10-23 -->
# Admin API

> Bahasa: [简体中文](admin-api.md) · [English](admin-api.en.md) · [繁體中文](admin-api.zh-TW.md) · [Русский](admin-api.ru.md) · [日本語](admin-api.ja.md) · [한국어](admin-api.ko.md) · [Deutsch](admin-api.de.md) · [Français](admin-api.fr.md) · [Español](admin-api.es.md) · [Português](admin-api.pt.md) · [Türkçe](admin-api.tr.md) · Bahasa Indonesia · [ไทย](admin-api.th.md) · [Tiếng Việt](admin-api.vi.md) · [العربية](admin-api.ar.md) · [فارسی](admin-api.fa.md)

Tabel rute admin lengkap API untuk **pengembangan panel/otomatisasi/integrator**.

- Basis URL: `https://<panel-domain>/api/v1`
- Otentikasi: `Authorization: Bearer <admin_token>` (dari `POST /api/v1/auth/login`)
- Format respons/kesalahan umum dan konvensi batas kecepatan: lihat [README.md](./README.id.md)
- Sumber kebenaran: `internal/api/router/router.go` (739 rute admin); jika tabel ini tidak sesuai, **kode menang**

## Penanda izin

| Penanda | Artinya |
|---|---|
| Publik | Tidak perlu login (penjaga pemeliharaan/pembatasan tarif mungkin masih berlaku) |
| Publik · tarif terbatas | Tidak perlu masuk, dengan batas tarif khusus |
| Masuk | Memerlukan token admin masuk (`AuthRequired`) |
| Admin | Membutuhkan peran ≥ `admin` (`RequireRole(RoleAdmin)`) |
| Admin super | Membutuhkan peran = `super_admin` |

Tingkat peran: `viewer(10) < operator(20) < admin(30) < super_admin(40)`; peran yang tidak mencukupi mengembalikan 403 `INSUFFICIENT_ROLE`.

---

## Mulai cepat (contoh nyata)

Contoh-contoh ini berasal dari respons lokal yang nyata; token dan data disunting.

### 1. Masuk dan dapatkan token

```bash
curl -X POST https://<panel-domain>/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"your-password"}'
```

Tanggapan:

```json
{
  "data": {
    "access_token": "<JWT, valid 24h>",
    "refresh_token": "<JWT, valid 7 days>",
    "user": { "id": 1, "role": "super_admin", "username": "admin" }
  }
}
```

### 2. Hubungi titik akhir bisnis dengan token

```bash
curl https://<panel-domain>/api/v1/clients?limit=1 \
  -H 'Authorization: Bearer <access_token>'
```

Respons (daftar menggunakan struktur `items` + `total`):

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

### 3. Contoh respons kesalahan

**401 tidak diautentikasi:**

```json
{
  "code": "MISSING_TOKEN",
  "message": "missing authorization header",
  "request_id": "a02cfdf9-6be0-46be-8584-824caab34dc7",
  "can_retry": false,
  "requires_action": false
}
```

**404 sumber daya tidak ditemukan:**

```json
{
  "code": "CLIENT_NOT_FOUND",
  "message": "client not found",
  "request_id": "099949bd-4ded-468d-a6f8-f8e798af2273",
  "can_retry": false,
  "requires_action": false
}
```

### 4. Paginasi

Semua titik akhir daftar menggunakan `limit` (ukuran halaman, default 20) dan `offset` (berbasis 0) dan mengembalikan `{items, total, limit, offset}`. Beberapa titik akhir lama mengembalikan array kosong (dicatat per tabel).

---

## Isi

- [Otentikasi dan keamanan admin](#otentikasi-dan-keamanan-admin)
- [akun admin](#akun-admin)
- [Manajemen pelanggan](#manajemen-pelanggan)
- [Templat rencana](#templat-rencana)
- [Varian rencana](#varian-rencana)
- [Paket lalu lintas](#paket-lalu-lintas)
- [Kupon](#kupon)
- [Grup simpul](#grup-simpul)
- [Server simpul](#server-simpul)
- [Log audit simpul](#log-audit-simpul)
- [Node (model lama, sudah tidak digunakan lagi)](#node-model-lama-sudah-tidak-digunakan-lagi)
- [Ikhtisar wilayah](#ikhtisar-wilayah)
- [Langganan](#langganan)
- [Format berlangganan](#format-berlangganan)
- [Titik akhir langganan](#titik-akhir-langganan)
- [Bagikan domain](#bagikan-domain)
- [IP Cloudflare](#ip-cloudflare)
- [Konfigurasi langganan (Mihomo/Clash)](#konfigurasi-langganan-mihomoclash)
- [Isi ulang situs](#isi-ulang-situs)
- [Isi ulang pesanan](#isi-ulang-pesanan)
- [Isi ulang kartu](#isi-ulang-kartu)
- [Mengisi acara panggilan balik](#mengisi-acara-panggilan-balik)
- [Biaya percobaan ulang panggilan balik](#biaya-percobaan-ulang-panggilan-balik)
- [Saldo pelanggan](#saldo-pelanggan)
- [Hadiah](#hadiah)
- [Konfigurasi rujukan](#konfigurasi-rujukan)
- [Penarikan rujukan](#penarikan-rujukan)
- [Daftar IP yang diizinkan (ringkasan)](#daftar-ip-yang-diizinkan-ringkasan)
- [Pengikatan OAuth Pelanggan](#pengikatan-oauth-pelanggan)
- [Tiket (admin)](#tiket-admin)
- [Artikel](#artikel)
- [Akun bersama](#akun-bersama)
- [Sertifikat](#sertifikat)
- [Saluran sertifikat](#saluran-sertifikat)
- [Konfigurasi EAB](#konfigurasi-eab)
- [akun ACME](#akun-acme)
- [CA](#built-in-ca) bawaan
- [CA bawaan](#ca-bawaan)
- [Aturan geografis](#aturan-geografis)
- [Pustaka aturan WAF](#pustaka-aturan-waf)
- [Manajemen catatan](#manajemen-catatan)
- [Cadangan](#cadangan)
- [Pengaturan sistem](#pengaturan-sistem)
- [Audit keamanan](#audit-keamanan)
- [Metrik](#metrik)
- [Templat portal](#templat-portal)
- [Instalasi agen](#instalasi-agen)
- [Biaya entri panggilan balik](#biaya-entri-panggilan-balik)
- [Masuk (dengan node)](#masuk-dengan-node)
- [Konfigurasi publik](#konfigurasi-publik)
- [Otentikasi klien dan bisnis (set yang sama di mesin admin)](#otentikasi-klien-dan-bisnis-set-yang-sama-di-mesin-admin)
- [Pengiriman langganan](#pengiriman-langganan)
- [Alur kerja konfigurasi runtime](#alur-kerja-konfigurasi-runtime)
- [Kebijakan akses layanan (pensiun)](#kebijakan-akses-layanan-pensiun)
- [Pemantauan kuota lalu lintas](#pemantauan-kuota-lalu-lintas)
- [Pemeriksaan kesehatan](#pemeriksaan-kesehatan)
- [Halaman portal](#halaman-portal)

---

### Otentikasi dan keamanan admin

Masuk, keluar, ubah kata sandi, 2FA (TOTP), Passkey (WebAuthn) dan tiket satu kali untuk panel admin.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| POST | `/api/v1/auth/2fa/disable` | Masuk | Nonaktifkan |
| POST | `/api/v1/auth/2fa/enable` | Masuk | Aktifkan |
| POST | `/api/v1/auth/2fa/recovery-codes` | Masuk | Buat ulang kode pemulihan |
| POST | `/api/v1/auth/2fa/setup` | Masuk | Pengaturan |
| GET | `/api/v1/auth/2fa/status` | Masuk | Status |
| POST | `/api/v1/auth/change-password` | Masuk | Ubah kata sandi |
| POST | `/api/v1/auth/login` | Publik | Masuk |
| POST | `/api/v1/auth/login/2fa` | Publik | Masuk dengan kode dinamis (2FA) |
| GET | `/api/v1/auth/login/2fa-status` | Publik | Periksa apakah akun tersebut mengaktifkan 2FA |
| POST | `/api/v1/auth/logout` | Masuk | Keluar |
| GET | `/api/v1/auth/me` | Masuk | Pengguna yang masuk saat ini |
| GET | `/api/v1/auth/passkey/credentials` | Masuk | Daftar kredensial Passkey |
| DELETE | `/api/v1/auth/passkey/credentials/:id` | Masuk | Hapus kredensial Passkey |
| POST | `/api/v1/auth/passkey/login/begin` | Publik | Mulai masuk |
| POST | `/api/v1/auth/passkey/login/finish` | Publik | Selesaikan proses masuk |
| POST | `/api/v1/auth/passkey/register/begin` | Masuk | Mulai pendaftaran |
| POST | `/api/v1/auth/passkey/register/finish` | Masuk | Pendaftaran lengkap |
| GET | `/api/v1/auth/passkey/rpids` | Masuk | Daftar RP ID yang diizinkan |
| POST | `/api/v1/auth/passkey/rpids` | Masuk | Perbarui daftar izin ID RP |
| GET | `/api/v1/auth/passkey/status` | Publik | Status |
| POST | `/api/v1/auth/refresh` | Publik | Segarkan |
| POST | `/api/v1/auth/ticket` | Masuk | Menerbitkan tiket berumur pendek satu kali |

### akun admin

Akun pengguna panel admin. Daftar memerlukan admin; buat/perbarui/hapus memerlukan admin super (mencegah peningkatan hak istimewa).

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/users` | Admin | Daftar |
| POST | `/api/v1/users` | Admin super | Buat |
| DELETE | `/api/v1/users/:id` | Admin super | Hapus |
| PUT | `/api/v1/users/:id` | Admin super | Pembaruan |

### Manajemen pelanggan

CRUD, suspend/resume, upgrade/downgrade, pembaharuan dan reset penggunaan untuk pelanggan (akun portal).

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/clients` | Masuk | Daftar pelanggan |
| POST | `/api/v1/clients` | Masuk | Buat pelanggan. tubuh: `{"email":"user@example.com","password":"plaintext-password","plan_id":1,"quota_bytes":10737418240,"expires_at":"2026-12-31T00:00:00Z","remark":""}`. `plan_id` diperlukan; `password` mungkin kosong (pelanggan dapat mengaturnya nanti di portal) |
| DELETE | `/api/v1/clients/:id` | Masuk | Hapus pelanggan |
| GET | `/api/v1/clients/:id` | Masuk | Dapatkan pelanggan |
| PUT | `/api/v1/clients/:id` | Masuk | Perbarui pelanggan |
| POST | `/api/v1/clients/:id/downgrade` | Masuk | Turunkan versi pelanggan |
| GET | `/api/v1/clients/:id/orders` | Masuk | Daftar pesanan pelanggan |
| POST | `/api/v1/clients/:id/renew` | Masuk | Perbarui pelanggan |
| POST | `/api/v1/clients/:id/reset-usage` | Masuk | Setel ulang penggunaan |
| POST | `/api/v1/clients/:id/restore` | Masuk | Lanjutkan pelanggan |
| POST | `/api/v1/clients/:id/suspend` | Masuk | Tangguhkan pelanggan |
| POST | `/api/v1/clients/:id/upgrade` | Masuk | Tingkatkan pelanggan |

### Templat rencana

Paket (varian, siklus penagihan, harga, kuota lalu lintas) dan otorisasi node tingkat templat.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/plan-templates` | Masuk | Daftar templat |
| POST | `/api/v1/plan-templates` | Masuk | Buat templat |
| DELETE | `/api/v1/plan-templates/:id` | Masuk | Hapus templat |
| GET | `/api/v1/plan-templates/:id` | Masuk | Detail templat |
| PUT | `/api/v1/plan-templates/:id` | Masuk | Perbarui templat |
| GET | `/api/v1/plan-templates/:id/accessible-nodes` | Masuk | Dapatkan node yang tersedia |
| GET | `/api/v1/plan-templates/:id/billing-node-options` | Masuk | Dapatkan opsi node yang dapat ditagih |
| PUT | `/api/v1/plan-templates/:id/node-groups` | Masuk | Tetapkan grup simpul templat |
| POST | `/api/v1/plan-templates/:id/sync-accounts` | Masuk | Sinkronkan akun ke templat |
| GET | `/api/v1/plan-templates/:id/variants` | Masuk | Daftar varian |
| POST | `/api/v1/plan-templates/:id/variants` | Masuk | Buat varian |
| POST | `/api/v1/plan-templates/sync-all-accounts` | Masuk | Sinkronkan semua akun |

### Varian rencana

Perbarui dan hapus varian paket (pembuatan disarangkan di bawah templat paket).

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| DELETE | `/api/v1/plan-variants/:id` | Masuk | Hapus varian |
| PUT | `/api/v1/plan-variants/:id` | Masuk | Perbarui varian |

### Paket lalu lintas

Paket lalu lintas yang dapat dibeli secara terpisah.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/traffic-packs` | Masuk | Daftar paket lalu lintas |
| POST | `/api/v1/traffic-packs` | Masuk | Buat paket lalu lintas |
| DELETE | `/api/v1/traffic-packs/:id` | Masuk | Hapus paket lalu lintas |
| PUT | `/api/v1/traffic-packs/:id` | Masuk | Perbarui paket lalu lintas |

### Kupon

Buat, validasi, dan tukarkan kupon.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/promotions` | Masuk | Daftar kupon |
| POST | `/api/v1/promotions` | Masuk | Buat kupon |
| DELETE | `/api/v1/promotions/:id` | Masuk | Hapus kupon |
| PUT | `/api/v1/promotions/:id` | Masuk | Perbarui kupon |
| POST | `/api/v1/promotions/validate` | Masuk | Validasi kupon |

### Grup simpul

Atur node ke dalam kelompok yang direferensikan oleh rencana.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/node-groups` | Masuk | Daftar grup simpul |
| POST | `/api/v1/node-groups` | Masuk | Buat grup simpul |
| DELETE | `/api/v1/node-groups/:id` | Masuk | Hapus grup simpul |
| PUT | `/api/v1/node-groups/:id` | Masuk | Perbarui grup simpul |
| GET | `/api/v1/node-groups/:id/members` | Masuk | Daftar anggota kelompok |
| POST | `/api/v1/node-groups/:id/members` | Masuk | Tambahkan anggota grup |
| DELETE | `/api/v1/node-groups/:id/members/:mid` | Masuk | Hapus anggota grup |

### Server simpul

Sumber daya inti: node CRUD, pengiriman konfigurasi inti (xray/sing-box), konfigurasi masuk/keluar/titik akhir, kontrol proses, sertifikat, WAF, batas kecepatan, port hopping, proxy berantai, mirror/cadangan utama, Penginstalan/peningkatan agen, kontrol waktu proses xray. Banyak sub-jalur, dikelompokkan berdasarkan kelompok fitur.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/node-servers` | Admin | Daftar |
| POST | `/api/v1/node-servers` | Admin | Buat |
| DELETE | `/api/v1/node-servers/:id` | Admin | Hapus |
| GET | `/api/v1/node-servers/:id` | Admin | Detail |
| PUT | `/api/v1/node-servers/:id` | Admin | Pembaruan |
| GET | `/api/v1/node-servers/:id/binaries` | Admin | Daftar inti yang diinstal |
| POST | `/api/v1/node-servers/:id/binaries/ech-generate` | Admin | Hasilkan pasangan kunci ECH |
| POST | `/api/v1/node-servers/:id/binaries/install` | Admin | Instal/tingkatkan inti |
| GET | `/api/v1/node-servers/:id/binaries/interfaces` | Admin | Daftar antarmuka jaringan |
| POST | `/api/v1/node-servers/:id/binaries/mldsa65-key` | Admin | Hasilkan kunci MLDSA65 |
| POST | `/api/v1/node-servers/:id/binaries/reality-key` | Admin | Hasilkan pasangan kunci REALITY |
| POST | `/api/v1/node-servers/:id/binaries/reality-public-key` | Admin | Dapatkan kunci publik dari kunci pribadi REALITY |
| GET | `/api/v1/node-servers/:id/binaries/releases` | Admin | Versi yang tersedia |
| POST | `/api/v1/node-servers/:id/binaries/vlessenc` | Admin | Hasilkan parameter enkripsi VLESS |
| POST | `/api/v1/node-servers/:id/binaries/wg-key` | Admin | Hasilkan kunci WireGuard |
| POST | `/api/v1/node-servers/:id/binaries/x25519-key` | Admin | Hasilkan pasangan kunci X25519 |
| GET | `/api/v1/node-servers/:id/bootstrap-cmd` | Admin | Dapatkan perintah Agen bootstrap |
| POST | `/api/v1/node-servers/:id/bootstrap-token` | Admin | Hasilkan token bootstrap |
| POST | `/api/v1/node-servers/:id/caddy/batch-update-log-enabled` | Admin | Sakelar log pembaruan massal |
| POST | `/api/v1/node-servers/:id/caddy/batch-update-ports` | Admin | Port pembaruan massal |
| POST | `/api/v1/node-servers/:id/caddy/generate-config` | Admin | Hasilkan konfigurasi |
| DELETE | `/api/v1/node-servers/:id/caddy/l4-rules` | Admin | Hapus semua aturan L4 |
| POST | `/api/v1/node-servers/:id/caddy/save-and-apply` | Admin | Simpan dan terapkan |
| GET | `/api/v1/node-servers/:id/caddy/sites` | Admin | Daftar situs |
| POST | `/api/v1/node-servers/:id/caddy/sites` | Admin | Buat situs |
| DELETE | `/api/v1/node-servers/:id/caddy/sites/:siteId` | Admin | Hapus situs |
| PUT | `/api/v1/node-servers/:id/caddy/sites/:siteId` | Admin | Perbarui situs |
| DELETE | `/api/v1/node-servers/:id/caddy/sni-rules` | Admin | Hapus semua aturan SNI |
| POST | `/api/v1/node-servers/:id/caddy/sni-rules/toggle` | Admin | Alihkan aturan SNI |
| GET | `/api/v1/node-servers/:id/caddy/sni-settings` | Admin | Pengaturan SNI |
| POST | `/api/v1/node-servers/:id/caddy/sni-settings` | Admin | Simpan pengaturan SNI |
| GET | `/api/v1/node-servers/:id/certificates` | Admin | Daftar sertifikat simpul |
| DELETE | `/api/v1/node-servers/:id/certificates/:cid` | Admin | Melepaskan ikatan sertifikat |
| POST | `/api/v1/node-servers/:id/certificates/:cid` | Admin | Mengikat sertifikat |
| GET | `/api/v1/node-servers/:id/chain-config` | Admin | Hasilkan konfigurasi berantai |
| GET | `/api/v1/node-servers/:id/chain-health` | Admin | Status kesehatan rantai |
| POST | `/api/v1/node-servers/:id/control` | Admin super | Inti simpul kontrol. badan: `{"core_type":"xray","action":"start"}` (aksi = mulai/berhenti/mulai ulang). `core_type` bisa berupa `xray`/`singbox` |
| GET | `/api/v1/node-servers/:id/degradation-status` | Admin | Statusnya terdegradasi |
| POST | `/api/v1/node-servers/:id/distribute-cert` | Admin | Bagikan sertifikat |
| GET | `/api/v1/node-servers/:id/endpoint-configs` | Admin | Daftar |
| POST | `/api/v1/node-servers/:id/endpoint-configs` | Admin | Buat |
| DELETE | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | Admin | Hapus |
| GET | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | Admin | Detail |
| PUT | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | Admin | Pembaruan |
| POST | `/api/v1/node-servers/:id/failover` | Admin | Memicu failover untuk node ini |
| GET | `/api/v1/node-servers/:id/failover-status` | Admin | Status kegagalan |
| POST | `/api/v1/node-servers/:id/force-reconnect` | Admin | Paksa sambungkan kembali |
| GET | `/api/v1/node-servers/:id/free-port` | Admin | Minta port gratis |
| POST | `/api/v1/node-servers/:id/geo-rules/push` | Admin | Dorong ke simpul ini |
| GET | `/api/v1/node-servers/:id/inbound-configs` | Admin | Daftar |
| POST | `/api/v1/node-servers/:id/inbound-configs` | Admin | Buat |
| DELETE | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | Admin | Hapus |
| GET | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | Admin | Detail |
| PUT | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | Admin | Pembaruan |
| GET | `/api/v1/node-servers/:id/inbound-configs/:cfgId/share-links` | Admin | Bagikan tautan dan cuplikan |
| POST | `/api/v1/node-servers/:id/inbound-configs/:cfgId/users/:email/pause` | Admin | Tangguhkan pengguna ini |
| POST | `/api/v1/node-servers/:id/inbound-configs/:cfgId/users/:email/resume` | Admin | Pulihkan pengguna ini |
| POST | `/api/v1/node-servers/:id/inbound-configs/reset-all-traffic` | Admin | Setel ulang semua lalu lintas masuk |
| GET | `/api/v1/node-servers/:id/install-script` | Admin | Dapatkan skrip pemasangan Agen |
| GET | `/api/v1/node-servers/:id/metrics-history` | Admin | Dapatkan riwayat metrik |
| GET | `/api/v1/node-servers/:id/mirror-status` | Admin | Status cermin |
| POST | `/api/v1/node-servers/:id/nginx/batch-update-ports` | Admin | Port pembaruan massal |
| POST | `/api/v1/node-servers/:id/nginx/generate-config` | Admin | Hasilkan konfigurasi |
| DELETE | `/api/v1/node-servers/:id/nginx/l4-rules` | Admin | Hapus semua aturan L4 |
| POST | `/api/v1/node-servers/:id/nginx/save-and-apply` | Admin | Simpan dan terapkan |
| GET | `/api/v1/node-servers/:id/nginx/settings` | Admin | Baca pengaturan |
| POST | `/api/v1/node-servers/:id/nginx/settings` | Admin | Simpan pengaturan |
| GET | `/api/v1/node-servers/:id/nginx/sites` | Admin | Daftar situs |
| POST | `/api/v1/node-servers/:id/nginx/sites` | Admin | Buat situs |
| DELETE | `/api/v1/node-servers/:id/nginx/sites/:siteId` | Admin | Hapus situs |
| PUT | `/api/v1/node-servers/:id/nginx/sites/:siteId` | Admin | Perbarui situs |
| DELETE | `/api/v1/node-servers/:id/nginx/sni-rules` | Admin | Hapus semua aturan SNI |
| POST | `/api/v1/node-servers/:id/nginx/sni-rules/toggle` | Admin | Alihkan aturan SNI |
| PUT | `/api/v1/node-servers/:id/node-type` | Admin | Perbarui jenis simpul |
| GET | `/api/v1/node-servers/:id/online-users` | Admin | Pengguna daring |
| GET | `/api/v1/node-servers/:id/outbound-configs` | Admin | Daftar |
| POST | `/api/v1/node-servers/:id/outbound-configs` | Admin | Buat |
| DELETE | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | Admin | Hapus |
| GET | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | Admin | Detail |
| PUT | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | Admin | Pembaruan |
| PUT | `/api/v1/node-servers/:id/outbound-configs/reorder` | Admin | Sortir |
| GET | `/api/v1/node-servers/:id/port-hop/rules` | Admin | Pratinjau aturan port-hopping |
| GET | `/api/v1/node-servers/:id/protocols` | Admin | Daftar protokol |
| GET | `/api/v1/node-servers/:id/protocols/:coreType` | Admin | Konfigurasi protokol |
| PUT | `/api/v1/node-servers/:id/protocols/:coreType` | Admin | Simpan konfigurasi protokol |
| GET | `/api/v1/node-servers/:id/protocols/:coreType/config-file` | Admin | Baca file konfigurasi inti |
| GET | `/api/v1/node-servers/:id/protocols/:coreType/section/:section` | Admin | Baca bagian protokol |
| PUT | `/api/v1/node-servers/:id/protocols/:coreType/section/:section` | Admin | Simpan bagian protokol |
| POST | `/api/v1/node-servers/:id/protocols/:coreType/validate` | Admin | Validasi konfigurasi (uji coba inti sebenarnya) |
| GET | `/api/v1/node-servers/:id/proxy-status` | Admin | Status proksi jarak jauh |
| POST | `/api/v1/node-servers/:id/proxy/batch-update-ports` | Admin | Port pembaruan massal |
| GET | `/api/v1/node-servers/:id/proxy/config` | Admin | Baca konfigurasi |
| PUT | `/api/v1/node-servers/:id/proxy/config` | Admin | Perbarui konfigurasi |
| POST | `/api/v1/node-servers/:id/proxy/control` | Admin | Kontrol pendaftaran proxy berantai |
| POST | `/api/v1/node-servers/:id/proxy/download` | Admin | Unduh versi inti proxy |
| POST | `/api/v1/node-servers/:id/proxy/generate-config` | Admin | Hasilkan konfigurasi |
| POST | `/api/v1/node-servers/:id/proxy/guardian` | Admin | Perbarui konfigurasi supervisor |
| POST | `/api/v1/node-servers/:id/proxy/install` | Admin | Instal proksi |
| DELETE | `/api/v1/node-servers/:id/proxy/l4-rules` | Admin | Hapus semua aturan L4 |
| GET | `/api/v1/node-servers/:id/proxy/l4-settings` | Admin | Pengaturan L4 |
| POST | `/api/v1/node-servers/:id/proxy/l4-settings` | Admin | Simpan pengaturan L4 |
| GET | `/api/v1/node-servers/:id/proxy/log-settings` | Admin | Pengaturan log |
| POST | `/api/v1/node-servers/:id/proxy/log-settings` | Admin | Simpan pengaturan log |
| GET | `/api/v1/node-servers/:id/proxy/nginx-settings` | Admin | Pengaturan nginx |
| POST | `/api/v1/node-servers/:id/proxy/nginx-settings` | Admin | Simpan pengaturan nginx |
| GET | `/api/v1/node-servers/:id/proxy/releases` | Admin | Versi yang tersedia |
| POST | `/api/v1/node-servers/:id/proxy/reload` | Admin | Muat ulang konfigurasi |
| POST | `/api/v1/node-servers/:id/proxy/restart` | Admin | Mulai ulang |
| GET | `/api/v1/node-servers/:id/proxy/sites` | Admin | Daftar situs |
| POST | `/api/v1/node-servers/:id/proxy/sites` | Admin | Buat situs |
| DELETE | `/api/v1/node-servers/:id/proxy/sites/:siteId` | Admin | Hapus situs |
| PUT | `/api/v1/node-servers/:id/proxy/sites/:siteId` | Admin | Perbarui situs |
| DELETE | `/api/v1/node-servers/:id/proxy/sni-rules` | Admin | Hapus semua aturan SNI |
| POST | `/api/v1/node-servers/:id/proxy/sni-rules/toggle` | Admin | Alihkan aturan SNI |
| GET | `/api/v1/node-servers/:id/proxy/sni-settings` | Admin | Pengaturan SNI |
| POST | `/api/v1/node-servers/:id/proxy/sni-settings` | Admin | Simpan pengaturan SNI |
| POST | `/api/v1/node-servers/:id/proxy/start` | Admin | Mulai |
| GET | `/api/v1/node-servers/:id/proxy/status` | Admin | Status proses proksi |
| POST | `/api/v1/node-servers/:id/proxy/stop` | Admin | Berhenti |
| POST | `/api/v1/node-servers/:id/proxy/sync` | Admin | Sinkronkan konfigurasi proxy |
| GET | `/api/v1/node-servers/:id/rate-limit` | Admin | Baca konfigurasi |
| PUT | `/api/v1/node-servers/:id/rate-limit` | Admin | Perbarui konfigurasi |
| GET | `/api/v1/node-servers/:id/rate-limit/status` | Admin | Status |
| GET | `/api/v1/node-servers/:id/recover-mode` | Admin | Baca mode pemulihan simpul |
| PUT | `/api/v1/node-servers/:id/recover-mode` | Admin | Setel mode pemulihan simpul |
| POST | `/api/v1/node-servers/:id/revoke-cert` | Admin | Cabut sertifikat Agen |
| GET | `/api/v1/node-servers/:id/services` | Admin | Daftar |
| POST | `/api/v1/node-servers/:id/services` | Admin | Buat |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId` | Admin | Hapus |
| GET | `/api/v1/node-servers/:id/services/:serviceId` | Admin | Detail |
| PUT | `/api/v1/node-servers/:id/services/:serviceId` | Admin | Pembaruan |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId/crash-logs` | Admin | Hapus log kerusakan |
| GET | `/api/v1/node-servers/:id/services/:serviceId/crash-logs` | Admin | Daftar log kerusakan |
| GET | `/api/v1/node-servers/:id/services/:serviceId/crash-logs/:logId` | Admin | Detail log kerusakan |
| GET | `/api/v1/node-servers/:id/services/:serviceId/endpoints` | Admin | Daftar titik akhir |
| POST | `/api/v1/node-servers/:id/services/:serviceId/endpoints` | Admin | Buat titik akhir |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId/endpoints/:endpointId` | Admin | Hapus titik akhir |
| PUT | `/api/v1/node-servers/:id/services/:serviceId/endpoints/:endpointId` | Admin | Perbarui titik akhir |
| PUT | `/api/v1/node-servers/:id/services/:serviceId/guardian` | Admin | Perbarui konfigurasi supervisor |
| GET | `/api/v1/node-servers/:id/services/:serviceId/process-status` | Admin | Dapatkan status pemrosesan |
| POST | `/api/v1/node-servers/:id/services/:serviceId/restart` | Admin | Mulai ulang |
| POST | `/api/v1/node-servers/:id/services/:serviceId/start` | Admin | Mulai |
| POST | `/api/v1/node-servers/:id/services/:serviceId/stop` | Admin | Berhenti |
| GET | `/api/v1/node-servers/:id/services/process-status` | Admin | Semua status proses |
| GET | `/api/v1/node-servers/:id/share-domains` | Admin | Bagikan domain untuk node ini |
| POST | `/api/v1/node-servers/:id/share-domains` | Admin | Buat |
| DELETE | `/api/v1/node-servers/:id/share-domains/:domainId` | Admin | Hapus |
| PUT | `/api/v1/node-servers/:id/share-domains/:domainId` | Admin | Pembaruan |
| DELETE | `/api/v1/node-servers/:id/share-domains/all` | Admin | Hapus semuanya |
| POST | `/api/v1/node-servers/:id/singbox/control/firewall/apply` | Admin | Terapkan aturan firewall port-hopping |
| POST | `/api/v1/node-servers/:id/singbox/control/firewall/remove` | Admin | Hapus aturan firewall port-hopping |
| POST | `/api/v1/node-servers/:id/speed-test` | Admin | Tes kecepatan simpul |
| GET | `/api/v1/node-servers/:id/status` | Admin | Status simpul |
| POST | `/api/v1/node-servers/:id/sync` | Admin | Konfigurasi simpul sinkronisasi (hasilkan dan dorong) |
| POST | `/api/v1/node-servers/:id/sync-data` | Admin super | Dorong data panel ke node (sinkronisasi penuh pengguna/langganan/paket) |
| POST | `/api/v1/node-servers/:id/sync-mirror` | Admin | Sinkronkan simpul cermin |
| GET | `/api/v1/node-servers/:id/tags` | Admin | Daftar tanda |
| POST | `/api/v1/node-servers/:id/tags` | Admin | Tambahkan tanda |
| DELETE | `/api/v1/node-servers/:id/tags/:tagKey` | Admin | Hapus tanda |
| GET | `/api/v1/node-servers/:id/traffic-history` | Admin | Sejarah lalu lintas |
| POST | `/api/v1/node-servers/:id/traffic-resume` | Admin | Pulihkan lalu lintas yang ditangguhkan karena kelebihan penggunaan |
| GET | `/api/v1/node-servers/:id/traffic-usage` | Admin | Penggunaan lalu lintas |
| POST | `/api/v1/node-servers/:id/update-agent` | Admin | Agen Peningkatan |
| GET | `/api/v1/node-servers/:id/used-server-names` | Admin | Daftar REALITY SNI yang sedang digunakan |
| GET | `/api/v1/node-servers/:id/waf/config` | Admin | Baca konfigurasi |
| PUT | `/api/v1/node-servers/:id/waf/config` | Admin | Simpan konfigurasi |
| POST | `/api/v1/node-servers/:id/waf/disable-domain` | Admin | Nonaktifkan perlindungan domain |
| GET | `/api/v1/node-servers/:id/waf/domain-config` | Admin | Daftar konfigurasi domain |
| PUT | `/api/v1/node-servers/:id/waf/domain-config` | Admin | Simpan konfigurasi domain secara massal |
| DELETE | `/api/v1/node-servers/:id/waf/domain-config/:domain` | Admin | Hapus konfigurasi domain |
| GET | `/api/v1/node-servers/:id/waf/domain-config/:domain` | Admin | Detail konfigurasi domain |
| PUT | `/api/v1/node-servers/:id/waf/domain-config/:domain` | Admin | Simpan konfigurasi domain |
| GET | `/api/v1/node-servers/:id/waf/domain-rules` | Admin | Direktori aturan domain |
| GET | `/api/v1/node-servers/:id/waf/domain-rules/:domain/:name` | Admin | Baca file aturan domain |
| PUT | `/api/v1/node-servers/:id/waf/domain-rules/:domain/:name` | Admin | Simpan file aturan domain |
| GET | `/api/v1/node-servers/:id/waf/domain-status` | Admin | Status perlindungan domain |
| POST | `/api/v1/node-servers/:id/waf/enable-domain` | Admin | Aktifkan perlindungan domain |
| POST | `/api/v1/node-servers/:id/waf/push` | Admin | Dorong ke simpul ini |
| GET | `/api/v1/node-servers/:id/waf/rules` | Admin | Daftar aturan node WAF |
| DELETE | `/api/v1/node-servers/:id/waf/rules/:name` | Admin | Hapus aturan simpul WAF |
| GET | `/api/v1/node-servers/:id/waf/rules/:name` | Admin | Detail aturan node WAF |
| PUT | `/api/v1/node-servers/:id/waf/rules/:name` | Admin | Simpan aturan simpul WAF |
| GET | `/api/v1/node-servers/:id/waf/status` | Admin | Status simpul WAF |
| PUT | `/api/v1/node-servers/:id/waf/status` | Admin | Alihkan simpul WAF |
| POST | `/api/v1/node-servers/:id/waf/sync` | Admin | Sinkronkan ke node ini |
| GET | `/api/v1/node-servers/:id/warp` | Admin | Daftar |
| DELETE | `/api/v1/node-servers/:id/warp/:warpId` | Admin | Hapus |
| GET | `/api/v1/node-servers/:id/warp/config` | Admin | Baca konfigurasi |
| POST | `/api/v1/node-servers/:id/warp/generate-config` | Admin | Hasilkan konfigurasi |
| POST | `/api/v1/node-servers/:id/warp/register` | Admin | Daftar |
| GET | `/api/v1/node-servers/:id/warp/status` | Admin | Status |
| GET | `/api/v1/node-servers/:id/xray/control/balancer/:tag` | Admin | Dapatkan info penyeimbang |
| POST | `/api/v1/node-servers/:id/xray/control/balancer/:tag` | Admin | Tetapkan target penyeimbang |
| POST | `/api/v1/node-servers/:id/xray/control/firewall/apply` | Admin | Terapkan aturan firewall port-hopping |
| POST | `/api/v1/node-servers/:id/xray/control/firewall/remove` | Admin | Hapus aturan firewall port-hopping |
| POST | `/api/v1/node-servers/:id/xray/control/inbound` | Admin | Tambahkan masuk |
| DELETE | `/api/v1/node-servers/:id/xray/control/inbound/:tag` | Admin | Hapus masuk |
| POST | `/api/v1/node-servers/:id/xray/control/inbound/:tag/user` | Admin | Tambahkan pengguna |
| DELETE | `/api/v1/node-servers/:id/xray/control/inbound/:tag/user/:email` | Admin | Hapus pengguna |
| POST | `/api/v1/node-servers/:id/xray/control/outbound` | Admin | Tambahkan keluar |
| DELETE | `/api/v1/node-servers/:id/xray/control/outbound/:tag` | Admin | Hapus keluar |
| POST | `/api/v1/node-servers/:id/xray/control/routing` | Admin | Minta rute |
| POST | `/api/v1/node-servers/:id/xray/control/routing/rule` | Admin | Tambahkan aturan perutean |
| DELETE | `/api/v1/node-servers/:id/xray/control/routing/rule/:tag` | Admin | Hapus aturan perutean |
| GET | `/api/v1/node-servers/:id/xray/control/routing/rules` | Admin | Daftar aturan perutean |
| POST | `/api/v1/node-servers/:id/xray/control/sync` | Admin | Picu sinkronisasi konfigurasi xray secara manual |
| GET | `/api/v1/node-servers/:id/xray/stats` | Admin | Dapatkan statistik |
| POST | `/api/v1/node-servers/:id/xray/stats/reset/:email` | Admin | Setel ulang statistik pengguna |
| GET | `/api/v1/node-servers/:id/xray/stats/users` | Admin | Dapatkan statistik pengguna |
| GET | `/api/v1/node-servers/billable` | Admin | Daftar node yang dapat ditagih |
| GET | `/api/v1/node-servers/ca-cert` | Admin | sertifikat CA |
| GET | `/api/v1/node-servers/ca-status` | Admin | status CA |
| GET | `/api/v1/node-servers/dashboard` | Admin | Status dasbor |
| POST | `/api/v1/node-servers/distribute-cert` | Admin | Distribusikan sertifikat ke semua node |
| POST | `/api/v1/node-servers/failover-check` | Admin | Memicu pemeriksaan failover |
| GET | `/api/v1/node-servers/metrics/aggregated` | Admin | Metrik gabungan |
| POST | `/api/v1/node-servers/migrate-cert` | Admin | Migrasi sertifikat |
| GET | `/api/v1/node-servers/online-summary` | Admin | Ringkasan daring |
| GET | `/api/v1/node-servers/online-users` | Admin | Semua pengguna daring |
| GET | `/api/v1/node-servers/pending-agent-updates` | Admin | Agen menunggu peningkatan |
| POST | `/api/v1/node-servers/recommend-region` | Admin | Wilayah yang direkomendasikan |
| POST | `/api/v1/node-servers/select-nodes` | Admin | Pilih node untuk berlangganan |
| POST | `/api/v1/node-servers/speed-test-all` | Admin | Uji kecepatan semua node |
| GET | `/api/v1/node-servers/speed-test-results` | Admin | Hasil tes kecepatan |
| GET | `/api/v1/node-servers/traffic/stats` | Admin | Statistik lalu lintas |
| POST | `/api/v1/node-servers/update-all-agents` | Admin | Tingkatkan semua Agen |
| DELETE | `/api/v1/node-servers/users/:client_id` | Admin | Sinkronkan-hapus pelanggan |
| GET | `/api/v1/node-servers/users/retry-queue` | Admin | Coba lagi status antrian |
| POST | `/api/v1/node-servers/users/retry-queue/process` | Admin | Proses antrian coba lagi |
| POST | `/api/v1/node-servers/users/sync` | Admin | Sinkronkan satu pelanggan |
| POST | `/api/v1/node-servers/validate-chain-config` | Admin | Validasi konfigurasi berantai |

### Log audit simpul

Audit dan ekspor operasi sisi node.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/node-audit-logs` | Masuk | Log audit simpul |
| GET | `/api/v1/node-audit-logs/export` | Masuk | Ekspor log audit simpul |

### Node (model lama, sudah tidak digunakan lagi)

> `/api/v1/nodes` CRUD dihentikan pada 20-10-2026: node dikelola secara seragam melalui `/api/v1/node-servers`.
> Permintaan pengguna online tetap di `/api/v1/node-servers/{online-users,online-summary,:id/online-users}`.

### Ikhtisar wilayah

Tampilan node dikumpulkan berdasarkan wilayah.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/regions/:region/servers` | Masuk | Daftar node untuk wilayah ini |
| GET | `/api/v1/regions/overview` | Masuk | Ikhtisar wilayah |

### Langganan

Manajemen catatan langganan: mencabut, memutar token, mengakses log, mengatur ulang penggunaan.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/subscriptions` | Masuk | Daftar |
| POST | `/api/v1/subscriptions` | Masuk | Buat |
| DELETE | `/api/v1/subscriptions/:id` | Masuk | Hapus |
| GET | `/api/v1/subscriptions/:id` | Masuk | Detail |
| PUT | `/api/v1/subscriptions/:id` | Masuk | Pembaruan |
| GET | `/api/v1/subscriptions/:id/access-logs` | Masuk | Catatan akses |
| POST | `/api/v1/subscriptions/:id/enable` | Masuk | Aktifkan |
| POST | `/api/v1/subscriptions/:id/reset-usage` | Masuk | Setel ulang penggunaan |
| POST | `/api/v1/subscriptions/:id/restore` | Masuk | Lanjutkan |
| POST | `/api/v1/subscriptions/:id/revoke` | Masuk | Cabut |
| POST | `/api/v1/subscriptions/:id/rotate` | Masuk | Putar |
| GET | `/api/v1/subscriptions/:id/rotations` | Masuk | Sejarah rotasi |

### Format berlangganan

> `/api/v1/subscription-formats` CRUD sisi admin dihentikan pada 20-10-2026: titik masuk sebenarnya adalah
> `/api/v1/mihomo/configs` (pusat format berlangganan). Tabel `subscription_formats` tetap ada,
> hanya digunakan sebagai data awal untuk titik akhir `/api/v1/public/client-configs` publik.

### Titik akhir langganan

Domain langganan khusus (diproksi terbalik ke jalur `/sub` panel).

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/subscription-endpoints` | Masuk | Daftar |
| POST | `/api/v1/subscription-endpoints` | Masuk | Buat |
| DELETE | `/api/v1/subscription-endpoints/:id` | Masuk | Hapus |
| GET | `/api/v1/subscription-endpoints/:id` | Masuk | Detail |
| PUT | `/api/v1/subscription-endpoints/:id` | Masuk | Pembaruan |

### Bagikan domain

Akses domain/kumpulan IP yang digunakan dalam tautan klien.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/share-domains` | Masuk | Daftar |
| POST | `/api/v1/share-domains` | Masuk | Buat |
| DELETE | `/api/v1/share-domains/:id` | Masuk | Hapus |
| PUT | `/api/v1/share-domains/:id` | Masuk | Pembaruan |
| DELETE | `/api/v1/share-domains/all` | Masuk | Hapus semuanya |
| POST | `/api/v1/share-domains/seed` | Masuk | Inisialisasi data default |

### IP Cloudflare

Cloudflare CDN Kumpulan IP dan manajemen sumber.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| DELETE | `/api/v1/cfip/entries` | Masuk | Hapus entri |
| GET | `/api/v1/cfip/entries` | Masuk | Daftar entri |
| POST | `/api/v1/cfip/entries` | Masuk | Tambahkan entri |
| DELETE | `/api/v1/cfip/entries/:id` | Masuk | Hapus entri |
| POST | `/api/v1/cfip/fetch-all` | Masuk | Ambil semua sumber |
| POST | `/api/v1/cfip/seed` | Masuk | Inisialisasi data default |
| GET | `/api/v1/cfip/sources` | Masuk | Daftar sumber data |
| POST | `/api/v1/cfip/sources` | Masuk | Buat sumber data |
| DELETE | `/api/v1/cfip/sources/:id` | Masuk | Hapus sumber data |
| PUT | `/api/v1/cfip/sources/:id` | Masuk | Perbarui sumber data |
| POST | `/api/v1/cfip/sources/:id/fetch` | Masuk | Ambil sumber ini |

### Konfigurasi langganan (Mihomo/Clash)

Konfigurasi templat langganan: grup, aturan, penyedia aturan, pratinjau YAML.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/mihomo/configs` | Masuk | Daftar |
| POST | `/api/v1/mihomo/configs` | Masuk | Buat |
| DELETE | `/api/v1/mihomo/configs/:id` | Masuk | Hapus |
| GET | `/api/v1/mihomo/configs/:id` | Masuk | Detail |
| PUT | `/api/v1/mihomo/configs/:id` | Masuk | Pembaruan |
| GET | `/api/v1/mihomo/configs/:id/proxy-groups` | Masuk | Daftar grup |
| POST | `/api/v1/mihomo/configs/:id/proxy-groups` | Masuk | Buat grup |
| GET | `/api/v1/mihomo/configs/:id/rule-providers` | Masuk | Daftar penyedia aturan |
| POST | `/api/v1/mihomo/configs/:id/rule-providers` | Masuk | Buat penyedia aturan |
| GET | `/api/v1/mihomo/configs/:id/rules` | Masuk | Daftar aturan |
| POST | `/api/v1/mihomo/configs/:id/rules` | Masuk | Buat aturan |
| GET | `/api/v1/mihomo/configs/:id/yaml` | Masuk | Pratinjau menghasilkan YAML |
| DELETE | `/api/v1/mihomo/proxy-groups/:id` | Masuk | Hapus grup |
| DELETE | `/api/v1/mihomo/rule-providers/:id` | Masuk | Hapus penyedia aturan |
| DELETE | `/api/v1/mihomo/rules/:id` | Masuk | Hapus aturan |

### Isi ulang situs

Integrasi situs pembayaran pihak ketiga: kunci penandatanganan, file verifikasi, verifikasi domain, dokumen integrasi.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/charge-sites` | Masuk | Daftar situs |
| POST | `/api/v1/charge-sites` | Masuk | Buat situs |
| DELETE | `/api/v1/charge-sites/:id` | Masuk | Hapus situs |
| GET | `/api/v1/charge-sites/:id` | Masuk | Detail situs |
| PUT | `/api/v1/charge-sites/:id` | Masuk | Perbarui situs |
| POST | `/api/v1/charge-sites/:id/clear-prev-secret` | Masuk | Hapus kunci lama |
| POST | `/api/v1/charge-sites/:id/generate-verify` | Masuk | Hasilkan file verifikasi |
| POST | `/api/v1/charge-sites/:id/generate-verify-file` | Masuk | Hasilkan file verifikasi |
| GET | `/api/v1/charge-sites/:id/integration-doc` | Masuk | Dapatkan dokumen integrasi |
| POST | `/api/v1/charge-sites/:id/rotate-secret` | Masuk | Putar kunci penandatanganan |
| POST | `/api/v1/charge-sites/:id/verify-dns` | Masuk | Verifikasi domain DNS |
| POST | `/api/v1/charge-sites/:id/verify-domain` | Masuk | Verifikasi tantangan domain HTTP |
| POST | `/api/v1/charge-sites/:id/verify-domain-dns` | Masuk | Verifikasi domain DNS |
| POST | `/api/v1/charge-sites/:id/verify-domain-http` | Masuk | Verifikasi tantangan domain HTTP |

### Isi ulang pesanan

Isi ulang pertanyaan pesanan, pengembalian uang, ekspor CSV.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/charge-orders` | Masuk | Daftar pesanan |
| GET | `/api/v1/charge-orders/:id` | Masuk | Detail pesanan |
| POST | `/api/v1/charge-orders/:id/refund` | Masuk | Pengembalian dana |
| GET | `/api/v1/charge-orders/export-csv` | Masuk | Ekspor pesanan isi ulang CSV |

### Isi ulang kartu

Pembuatan kartu massal, catatan penggunaan, ekspor CSV.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/charge-cards` | Masuk | Daftar kartu |
| POST | `/api/v1/charge-cards` | Masuk | Buat kartu |
| DELETE | `/api/v1/charge-cards/:id` | Masuk | Hapus kartu |
| GET | `/api/v1/charge-cards/:id/usages` | Masuk | Catatan penggunaan kartu |
| POST | `/api/v1/charge-cards/batch` | Masuk | Buat kartu secara massal |
| GET | `/api/v1/charge-cards/export-csv` | Masuk | Ekspor kartu CSV |

### Mengisi acara panggilan balik

Audit acara panggilan balik.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/charge-callback-events` | Masuk | Daftar acara panggilan balik |

### Biaya percobaan ulang panggilan balik

Coba ulang panggilan balik yang gagal secara manual.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/charge-callback-retries` | Masuk | Daftar panggilan balik yang gagal |
| POST | `/api/v1/charge-callback-retries/:id/retry` | Admin | Coba lagi panggilan balik |

### Saldo pelanggan

Tampilan admin saldo dan transaksi.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/client-balances` | Masuk | Daftar saldo pelanggan |
| GET | `/api/v1/client-balances/transactions` | Masuk | Transaksi saldo (admin) |
| GET | `/api/v1/client-balances/transactions/export-csv` | Masuk | Transaksi saldo ekspor CSV |

### Hadiah

Mengeluarkan hadiah yang dapat diklaim (hari/lalu lintas).

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/client-gifts` | Masuk | Daftar hadiah |
| POST | `/api/v1/client-gifts` | Masuk | Buat hadiah |
| POST | `/api/v1/client-gifts/batch` | Masuk | Buat hadiah secara massal |
| GET | `/api/v1/client-gifts/search-clients` | Masuk | Cari pelanggan |

### Konfigurasi rujukan

Konfigurasi dan statistik komisi referensi.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/referral/config` | Masuk | Konfigurasi rujukan |
| PUT | `/api/v1/referral/config` | Admin | Simpan konfigurasi rujukan |
| GET | `/api/v1/referral/records` | Masuk | Daftar catatan rujukan |
| GET | `/api/v1/referral/stats` | Masuk | Statistik rujukan |

### Penarikan rujukan

Tinjau permintaan penarikan.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/referral-withdrawals` | Masuk | Daftar permintaan penarikan |
| POST | `/api/v1/referral-withdrawals/:id/approve` | Masuk | Setujui penarikan |
| POST | `/api/v1/referral-withdrawals/:id/reject` | Masuk | Tolak penarikan |

### Daftar IP yang diizinkan (ringkasan)

Tampilan daftar izin IP di semua pelanggan.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/ip-whitelists` | Masuk | Semua daftar IP pelanggan yang diizinkan |

### Pengikatan OAuth Pelanggan

Mengkueri pengikatan akun pihak ketiga pelanggan.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/client-oauth-bindings` | Masuk | Daftar pengikatan OAuth Pelanggan |

### Tiket (admin)

Balasan tiket, penugasan, dan penutupan.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/tickets` | Masuk | Daftar tiket |
| GET | `/api/v1/tickets/:id` | Masuk | Detail tiket |
| POST | `/api/v1/tickets/:id/assign` | Masuk | Tetapkan tiket |
| POST | `/api/v1/tickets/:id/close` | Masuk | Tutup tiket |
| POST | `/api/v1/tickets/:id/reply` | Masuk | Membalas tiket |

### Artikel

CRUD untuk artikel portal.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/articles` | Masuk | Daftar artikel |
| POST | `/api/v1/articles` | Masuk | Buat artikel |
| DELETE | `/api/v1/articles/:id` | Masuk | Hapus artikel |
| GET | `/api/v1/articles/:id` | Masuk | Detail artikel |
| PUT | `/api/v1/articles/:id` | Masuk | Perbarui artikel |

### Akun bersama

Catatan penerbitan dan perbaikan untuk akun bersama (streaming, dll.).

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/shared-accounts` | Masuk | Daftar akun bersama |
| POST | `/api/v1/shared-accounts` | Masuk | Buat akun bersama |
| DELETE | `/api/v1/shared-accounts/:id` | Masuk | Hapus akun bersama |
| GET | `/api/v1/shared-accounts/:id` | Masuk | Detail akun bersama |
| PUT | `/api/v1/shared-accounts/:id` | Masuk | Perbarui akun bersama |
| GET | `/api/v1/shared-accounts/:id/repair-logs` | Masuk | Catatan perbaikan |

### Sertifikat

Penerbitan, pengunggahan, pembaruan dan ekspor sertifikat publik (ACME); penerbitan pada node tertentu didukung.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/certificates` | Masuk | Daftar |
| POST | `/api/v1/certificates` | Masuk | Buat |
| DELETE | `/api/v1/certificates/:id` | Masuk | Hapus |
| GET | `/api/v1/certificates/:id` | Masuk | Detail |
| PUT | `/api/v1/certificates/:id` | Masuk | Pembaruan |
| GET | `/api/v1/certificates/:id/download` | Masuk | Unduh |
| POST | `/api/v1/certificates/:id/renew` | Masuk | Perbarui pelanggan |
| GET | `/api/v1/certificates/check-port` | Masuk | Periksa pelabuhan |
| GET | `/api/v1/certificates/detect-ip` | Masuk | Selidiki IP |
| GET | `/api/v1/certificates/export` | Masuk | Ekspor |
| POST | `/api/v1/certificates/issue` | Masuk | Masalah. tubuh: `{"domains":["example.com","*.example.com"],"challenge_type":"dns-01","dns_channel_id":2,"key_type":"rsa","key_bits":4096}`. `challenge_type` defaultnya adalah http-01; `dns_channel_id` memilih saluran DNS (penyedia DNS yang dikonfigurasi + kredensial) |
| POST | `/api/v1/certificates/issue-on-node` | Masuk | Masalah ke node |
| POST | `/api/v1/certificates/upload` | Masuk | Unggah |

### Saluran sertifikat

Konfigurasi saluran ACME (CA).

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/cert-channels` | Masuk | Daftar |
| POST | `/api/v1/cert-channels` | Masuk | Buat |
| DELETE | `/api/v1/cert-channels/:id` | Masuk | Hapus |
| PUT | `/api/v1/cert-channels/:id` | Masuk | Pembaruan |
| GET | `/api/v1/cert-channels/providers` | Masuk | Penyedia |

### Konfigurasi EAB

Kredensial mengikat akun eksternal seperti ZeroSSL / Google.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/eab-configs` | Masuk | Daftar |
| POST | `/api/v1/eab-configs` | Masuk | Buat |
| DELETE | `/api/v1/eab-configs/:id` | Masuk | Hapus |
| PUT | `/api/v1/eab-configs/:id` | Masuk | Pembaruan |
| POST | `/api/v1/eab-configs/fetch-google` | Masuk | Ambil Google EAB dari GCP |
| POST | `/api/v1/eab-configs/fetch-zerossl` | Masuk | Ambil ZeroSSL EAB dengan email dalam satu klik |

### akun ACME

Manajemen akun ACME sisi panel (email kontak).

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/acme-accounts` | Masuk | Daftar |
| POST | `/api/v1/acme-accounts` | Masuk | Buat |
| DELETE | `/api/v1/acme-accounts/:id` | Masuk | Hapus |
| PUT | `/api/v1/acme-accounts/:id` | Masuk | Pembaruan |
| GET | `/api/v1/acme-accounts/default` | Masuk | Dapatkan default |

### CA bawaan

Panel internal CA: Penerbitan/perpanjangan/pencabutan sertifikat agen, ekspor CRL, antrian distribusi.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/ca/agent-certs` | Masuk | Daftar sertifikat agen |
| POST | `/api/v1/ca/agent-certs/:server_id/renew` | Masuk | Perpanjang sertifikat Agen |
| GET | `/api/v1/ca/distribution-queue` | Masuk | Antrian distribusi sertifikat |
| DELETE | `/api/v1/ca/distribution-queue/:id` | Masuk | Hapus entri antrian distribusi |
| POST | `/api/v1/ca/distribution-queue/:id/retry` | Masuk | Coba lagi distribusi sertifikat |
| GET | `/api/v1/ca/revocations` | Masuk | Daftar pencabutan |
| GET | `/api/v1/ca/revocations/export` | Masuk | Ekspor CRL |
| GET | `/api/v1/ca/status` | Masuk | status CA |

### Aturan geografis

Sinkronisasi dan pengiriman sumber data dan file geoip/geosite.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/geo-rules/files` | Masuk | Daftar berkas |
| DELETE | `/api/v1/geo-rules/files/:fileId` | Masuk | Hapus berkas |
| PUT | `/api/v1/geo-rules/files/:fileId/auto-sync` | Masuk | Alihkan sinkronisasi otomatis |
| POST | `/api/v1/geo-rules/files/:fileId/download` | Masuk | Unduh |
| GET | `/api/v1/geo-rules/files/:fileId/download-file` | Masuk | Unduh berkas |
| GET | `/api/v1/geo-rules/github-assets` | Masuk | Ambil aset repositori GitHub |
| POST | `/api/v1/geo-rules/push-all` | Masuk | Dorong ke semua node |
| POST | `/api/v1/geo-rules/seed` | Masuk | Inisialisasi data default |
| GET | `/api/v1/geo-rules/sources` | Masuk | Daftar sumber data |
| POST | `/api/v1/geo-rules/sources` | Masuk | Buat sumber data |
| DELETE | `/api/v1/geo-rules/sources/:sourceId` | Masuk | Hapus sumber data |
| PUT | `/api/v1/geo-rules/sources/:sourceId` | Masuk | Perbarui sumber data |
| GET | `/api/v1/geo-rules/sources/:sourceId/assets` | Masuk | Ambil aset Rilis |
| POST | `/api/v1/geo-rules/sources/:sourceId/files` | Masuk | Tambahkan berkas |
| PUT | `/api/v1/geo-rules/sources/:sourceId/schedule` | Masuk | Alihkan sinkronisasi terjadwal |
| POST | `/api/v1/geo-rules/sources/:sourceId/select` | Masuk | Pilih berkas |
| POST | `/api/v1/geo-rules/sources/:sourceId/sync` | Masuk | Sinkronkan sumber data ini |
| GET | `/api/v1/geo-rules/status` | Masuk | Status |
| POST | `/api/v1/geo-rules/sync-all` | Masuk | Sinkronkan semua |
| GET | `/api/v1/geo-rules/sync-status/:key` | Masuk | Sinkronkan kemajuan |

### Pustaka aturan WAF

Aturan WAF global: sumber GitHub, sinkronisasi, pengiriman ke semua node.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/waf/github-source` | Masuk | Sumber data GitHub |
| PUT | `/api/v1/waf/github-source` | Masuk | Simpan sumber data GitHub |
| POST | `/api/v1/waf/push-all` | Masuk | Dorong ke semua node |
| GET | `/api/v1/waf/rules` | Masuk | Daftar aturan |
| DELETE | `/api/v1/waf/rules/:name` | Masuk | Hapus aturan |
| GET | `/api/v1/waf/rules/:name` | Masuk | Dapatkan aturan |
| PUT | `/api/v1/waf/rules/:name` | Masuk | Simpan aturan |
| POST | `/api/v1/waf/sync-all` | Masuk | Sinkronkan semua |
| POST | `/api/v1/waf/sync-github` | Masuk | Sinkronisasi dari GitHub |

### Manajemen catatan

Pembacaan dan streaming log node/inti, kebijakan pembersihan, kontrol penulis log.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| POST | `/api/v1/log-manage/cleanup-all` | Masuk | Bersihkan semuanya sekarang |
| GET | `/api/v1/log-manage/cleanup-rules` | Masuk | Daftar aturan pembersihan |
| PUT | `/api/v1/log-manage/cleanup-rules/:id` | Masuk | Perbarui aturan pembersihan |
| POST | `/api/v1/log-manage/cleanup-table/:table` | Masuk | Bersihkan tabel yang ditentukan sekarang |
| GET | `/api/v1/log-manage/core-log-output` | Masuk | Konfigurasi keluaran log inti |
| PUT | `/api/v1/log-manage/core-log-output` | Masuk | Simpan konfigurasi keluaran log inti |
| GET | `/api/v1/log-manage/core/:type/stdout-log` | Masuk | Baca log stdout inti |
| GET | `/api/v1/log-manage/core/:type/stdout-stream` | Masuk | Streaming inti stdout |
| POST | `/api/v1/log-manage/discover` | Masuk | Temukan file log simpul |
| GET | `/api/v1/log-manage/log-output` | Masuk | Konfigurasi keluaran log |
| PUT | `/api/v1/log-manage/log-output` | Masuk | Simpan konfigurasi keluaran log |
| POST | `/api/v1/log-manage/log-output/release/:type` | Masuk | Rilis log pengambilalihan stdout |
| POST | `/api/v1/log-manage/log-output/takeover/:type` | Masuk | Ambil alih log stdout |
| GET | `/api/v1/log-manage/nodes/:serverId/logs` | Masuk | Daftar file log simpul |
| GET | `/api/v1/log-manage/nodes/:serverId/logs/:type` | Masuk | Baca log simpul |
| POST | `/api/v1/log-manage/nodes/:serverId/logs/:type/rotate` | Masuk | Memutar log simpul |
| GET | `/api/v1/log-manage/nodes/:serverId/logs/:type/stream` | Masuk | Log simpul aliran |
| GET | `/api/v1/log-manage/policies` | Masuk | Daftar kebijakan |
| POST | `/api/v1/log-manage/policies` | Masuk | Buat kebijakan |
| DELETE | `/api/v1/log-manage/policies/:id` | Masuk | Hapus kebijakan |
| GET | `/api/v1/log-manage/policies/:id` | Masuk | Detail kebijakan |
| PUT | `/api/v1/log-manage/policies/:id` | Masuk | Perbarui kebijakan |
| GET | `/api/v1/log-manage/policies/:id/jobs` | Masuk | Daftar pekerjaan pembersihan |
| GET | `/api/v1/log-manage/policies/:id/log` | Masuk | Baca log |
| GET | `/api/v1/log-manage/policies/:id/log-stream` | Masuk | Log aliran |
| POST | `/api/v1/log-manage/policies/:id/rotate` | Masuk | Putar |
| GET | `/api/v1/log-manage/proxy/:type/stdout-log` | Masuk | Baca log stdout inti |
| GET | `/api/v1/log-manage/proxy/:type/stdout-stream` | Masuk | Streaming inti stdout |
| GET | `/api/v1/log-manage/scheduler-config` | Masuk | Konfigurasi penjadwalan |
| PUT | `/api/v1/log-manage/scheduler-config` | Masuk | Perbarui konfigurasi penjadwalan |
| GET | `/api/v1/log-manage/status` | Masuk | Status |
| POST | `/api/v1/log-manage/sync` | Masuk | Sinkronkan kebijakan pembersihan |
| GET | `/api/v1/log-manage/writers` | Masuk | Status penulis |
| PUT | `/api/v1/log-manage/writers/:type/config` | Masuk | Perbarui konfigurasi penulis |
| GET | `/api/v1/log-manage/writers/:type/files` | Masuk | Daftar file log penulis |
| POST | `/api/v1/log-manage/writers/:type/start` | Masuk | Mulai penulis |
| POST | `/api/v1/log-manage/writers/:type/stop` | Masuk | Hentikan penulis |

### Cadangan

Basis data/pencadangan penuh, pemulihan, pekerjaan terjadwal, penyimpanan jarak jauh FTP/S3/WebDAV.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| DELETE | `/api/v1/backups/:id` | Admin | Hapus cadangan |
| GET | `/api/v1/backups/:id/download` | Admin | Unduh cadangan |
| POST | `/api/v1/backups/:id/restore` | Admin | Pulihkan dari cadangan |
| POST | `/api/v1/backups/:id/upload-ftp` | Admin | Unggah ke FTP |
| POST | `/api/v1/backups/:id/upload-s3` | Admin | Unggah ke S3 |
| POST | `/api/v1/backups/:id/upload-webdav` | Admin | Unggah ke WebDAV |
| POST | `/api/v1/backups/create` | Admin | Buat cadangan penuh (dengan DB dump) |
| GET | `/api/v1/backups/list` | Admin | Daftar cadangan lengkap |
| POST | `/api/v1/backups/remote-restore` | Admin | Pulihkan dari jarak jauh |
| GET | `/api/v1/backups/restore-status` | Admin | Pulihkan kemajuan |
| GET | `/api/v1/backups/schedules` | Admin | Daftar pekerjaan terjadwal |
| POST | `/api/v1/backups/schedules` | Admin | Buat pekerjaan terjadwal |
| DELETE | `/api/v1/backups/schedules/:id` | Admin | Hapus pekerjaan terjadwal |
| PUT | `/api/v1/backups/schedules/:id` | Admin | Perbarui pekerjaan terjadwal |
| GET | `/api/v1/backups/storages` | Admin | Daftar konfigurasi penyimpanan |
| POST | `/api/v1/backups/storages` | Admin | Buat konfigurasi penyimpanan |
| DELETE | `/api/v1/backups/storages/:id` | Admin | Hapus konfigurasi penyimpanan |
| PUT | `/api/v1/backups/storages/:id` | Admin | Perbarui konfigurasi penyimpanan |
| GET | `/api/v1/backups/storages/:id/files` | Admin | Daftar file cadangan jarak jauh |
| POST | `/api/v1/backups/upload-restore` | Admin | Unggah cadangan dan pulihkan |

### Pengaturan sistem

Konfigurasi dan migrasi database, pengaturan panel, kunci JWT, tes SMTP/Telegram, templat notifikasi, peningkatan online (kebanyakan admin).

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| PUT | `/api/v1/system/bootstrap-finish` | Admin | Selesaikan pengaturan boot pertama |
| GET | `/api/v1/system/bootstrap-state` | Admin | Status booting pertama |
| GET | `/api/v1/system/config-file/:coreType` | Admin | Baca file konfigurasi inti |
| GET | `/api/v1/system/db-config` | Admin | Konfigurasi basis data |
| PUT | `/api/v1/system/db-config` | Admin | Perbarui konfigurasi basis data |
| POST | `/api/v1/system/db-config/migrate` | Admin | Migrasikan data basis data |
| POST | `/api/v1/system/db-config/switch` | Admin | Beralih database (dengan migrasi satu langkah) |
| POST | `/api/v1/system/db-config/test` | Admin | Uji koneksi basis data |
| GET | `/api/v1/system/db-pool-stats` | Admin | Statistik kumpulan koneksi |
| GET | `/api/v1/system/db-stats` | Admin | Statistik basis data |
| GET | `/api/v1/system/db-stats/ping` | Admin | Penyelidikan basis data |
| POST | `/api/v1/system/db-stats/reset` | Admin | Setel ulang statistik basis data |
| GET | `/api/v1/system/encryption-key-status` | Admin | Status kunci enkripsi |
| GET | `/api/v1/system/env-config` | Admin | konfigurasi .env |
| PUT | `/api/v1/system/env-config` | Admin | Perbarui konfigurasi .env |
| POST | `/api/v1/system/generate-encryption-key` | Admin | Hasilkan kunci enkripsi |
| GET | `/api/v1/system/github-config` | Admin | Konfigurasi GitHub |
| PUT | `/api/v1/system/github-config` | Admin | Perbarui konfigurasi GitHub |
| GET | `/api/v1/system/jwt-secret-status` | Admin | Status kunci JWT |
| GET | `/api/v1/system/maintenance` | Publik | Status mode pemeliharaan (publik) |
| PUT | `/api/v1/system/maintenance` | Admin | Alihkan mode pemeliharaan |
| POST | `/api/v1/system/migrate-data` | Admin | Migrasikan data |
| GET | `/api/v1/system/notification-config` | Admin | Konfigurasi notifikasi |
| PUT | `/api/v1/system/notification-config` | Admin | Perbarui konfigurasi notifikasi |
| GET | `/api/v1/system/notification-templates` | Admin | Daftar templat |
| POST | `/api/v1/system/notification-templates` | Admin | Buat templat |
| DELETE | `/api/v1/system/notification-templates/:id` | Admin | Hapus templat |
| PUT | `/api/v1/system/notification-templates/:id` | Admin | Perbarui templat |
| POST | `/api/v1/system/notification-templates/:id/broadcast` | Admin | Pemberitahuan siaran berdasarkan templat |
| POST | `/api/v1/system/notification-templates/:id/send` | Admin | Kirim pemberitahuan tes |
| GET | `/api/v1/system/panel-config` | Admin | Konfigurasi panel |
| PUT | `/api/v1/system/panel-config` | Admin | Perbarui konfigurasi panel |
| GET | `/api/v1/system/reality-targets` | Admin | Daftar target REALITY |
| PUT | `/api/v1/system/reality-targets` | Admin | Perbarui target REALITY |
| POST | `/api/v1/system/reality-targets/reset` | Admin | Setel ulang target REALITY ke default |
| POST | `/api/v1/system/regenerate-jwt-secret` | Admin | Regenerasi kunci JWT |
| GET | `/api/v1/system/releases` | Admin | Versi yang tersedia |
| POST | `/api/v1/system/restart` | Admin | Mulai ulang |
| POST | `/api/v1/system/rollback` | Admin | Kemunduran sistem |
| GET | `/api/v1/system/settings` | Admin | Baca pengaturan |
| PUT | `/api/v1/system/settings/batch` | Admin | Pengaturan pembaruan massal |
| GET | `/api/v1/system/stats` | Admin | Status sistem |
| GET | `/api/v1/system/subscription-billing-templates` | Admin | Templat penagihan berlangganan |
| PUT | `/api/v1/system/subscription-billing-templates` | Admin | Perbarui templat penagihan langganan |
| POST | `/api/v1/system/telegram-chat-id` | Admin | Dapatkan ID Obrolan Telegram |
| POST | `/api/v1/system/test-smtp` | Admin | Uji SMTP |
| POST | `/api/v1/system/test-telegram` | Admin | Uji Telegram |
| POST | `/api/v1/system/update` | Admin | Peningkatan sistem |
| GET | `/api/v1/system/update-jobs` | Admin | Tingkatkan daftar pekerjaan |
| POST | `/api/v1/system/upload-mtls-ca` | Admin | Unggah sertifikat mTLS CA |

### Audit keamanan

Audit kueri acara dan ekspor.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/security/audit-events` | Masuk | Daftar acara audit |
| GET | `/api/v1/security/audit-events/export` | Masuk | Ekspor peristiwa audit |

### Metrik

Metrik gabungan panel (node/sertifikat/layanan).

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/metrics/certificates` | Masuk | Metrik sertifikat |
| GET | `/api/v1/metrics/nodes` | Masuk | Metrik simpul |
| GET | `/api/v1/metrics/overview` | Masuk | Ikhtisar |

### Templat portal

Templat portal, konfigurasi aktivasi, dan manajemen cache.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/portal/active` | Masuk | Templat aktif |
| PUT | `/api/v1/portal/active` | Masuk | Perbarui template aktif |
| DELETE | `/api/v1/portal/cache` | Masuk | Hapus cache portal |
| GET | `/api/v1/portal/cache` | Masuk | Status cache |
| PUT | `/api/v1/portal/cache/dev-mode` | Masuk | Alihkan mode pengembangan cache |
| POST | `/api/v1/portal/cache/rebuild` | Masuk | Membangun kembali cache portal |
| POST | `/api/v1/portal/reset` | Masuk | Kembalikan templat default |
| GET | `/api/v1/portal/templates` | Masuk | Daftar |
| POST | `/api/v1/portal/templates` | Masuk | Buat |
| DELETE | `/api/v1/portal/templates/:id` | Masuk | Hapus |
| GET | `/api/v1/portal/templates/:id` | Masuk | Detail |
| PUT | `/api/v1/portal/templates/:id` | Masuk | Pembaruan |
| POST | `/api/v1/portal/templates/:id/activate` | Masuk | Aktifkan templat |

> `/portal/templates` CRUD adalah **API tingkat lanjut**: UI portal terutama menggunakan `PUT /api/v1/portal/active`
> (konfigurasi situs lengkap sekali pakai); kelola/ganti beberapa templat dengan grup ini. Keduanya mendukung dan tidak bertentangan.

### Instalasi agen

Skrip instalasi Node Agent dan unduh (publik).

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| POST | `/api/v1/agent/bootstrap` | Publik | Pendaftaran boot pertama agen (Agen melaporkan dengan token bootstrap; publik) |
| GET | `/api/v1/agent/releases` | Admin | Versi Agen yang tersedia (untuk halaman peningkatan) |
| GET | `/api/v1/agent/version` | Admin | Versi Agen saat ini di panel |
| GET | `/api/v1/agent/download` | Publik | Unduh Agen |
| GET | `/api/v1/agent/install` | Publik | Dapatkan skrip pemasangan |

### Biaya entri panggilan balik

Callback platform pembayaran pihak ketiga (publik, terverifikasi tanda tangan).

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| POST | `/api/v1/charge/callback/:site_id` | Publik | Menangani panggilan balik biaya |

### Masuk (dengan node)

Daftar masuk yang digunakan untuk pemilihan simpul rencana.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/inbounds-with-server` | Masuk | Semua masuk (termasuk nodenya) |

### Konfigurasi publik

Konfigurasi waktu proses portal tanpa masuk (penjaga pemeliharaan + batas tarif publik).

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/public/articles` | Publik | Daftar artikel publik |
| GET | `/api/v1/public/articles/:slug` | Publik | Detail artikel publik |
| GET | `/api/v1/public/client-configs` | Publik | Konfigurasi pengunduhan klien (publik) |
| GET | `/api/v1/public/portal-config` | Publik | Konfigurasi runtime portal (nama situs, tema, bahasa, dokumen kepatuhan; publik) |
| GET | `/api/v1/public/settings` | Publik | Pengaturan publik (misalnya registrasi_diaktifkan; publik) |
| GET | `/api/v1/public/subscription-endpoints` | Publik | Daftar titik akhir langganan yang diaktifkan (publik) |

### Otentikasi klien dan bisnis (set yang sama di mesin admin)

Titik akhir klien identik dengan portal; lihat `portal-api.en.md` untuk detailnya.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/client-auth/articles` | Publik | Daftar artikel |
| GET | `/api/v1/client-auth/articles/:slug` | Publik | Dapatkan artikel |
| GET | `/api/v1/client-auth/balance` | Publik | Dapatkan keseimbangan |
| GET | `/api/v1/client-auth/balance/transactions` | Publik | Transaksi saldo |
| GET | `/api/v1/client-auth/bills` | Publik | Daftar pesanan |
| POST | `/api/v1/client-auth/change-password` | Publik | Ubah kata sandi |
| POST | `/api/v1/client-auth/charge/create` | Publik | Buat pesanan isi ulang |
| GET | `/api/v1/client-auth/charge/orders` | Publik | Daftar pesanan isi ulang |
| GET | `/api/v1/client-auth/charge/orders/:id` | Publik | Dapatkan detail pesanan isi ulang |
| POST | `/api/v1/client-auth/charge/redeem-card` | Publik | Tukarkan kartu |
| GET | `/api/v1/client-auth/charge/sites` | Publik | Daftar situs isi ulang |
| GET | `/api/v1/client-auth/charge/status/:trade_no` | Publik | Status isi ulang kueri |
| GET | `/api/v1/client-auth/checktime` | Publik | Dapatkan waktu server (sinkronisasi jam) |
| GET | `/api/v1/client-auth/connection-info` | Publik | Dapatkan parameter koneksi paket |
| GET | `/api/v1/client-auth/gifts` | Publik | Daftar hadiah |
| POST | `/api/v1/client-auth/gifts/:id/claim` | Publik | Klaim hadiah |
| GET | `/api/v1/client-auth/invoices` | Publik | Daftar pesanan |
| GET | `/api/v1/client-auth/invoices/:id` | Publik | Detail pesanan |
| GET | `/api/v1/client-auth/ip-whitelist` | Publik | Daftar daftar IP yang diizinkan |
| DELETE | `/api/v1/client-auth/ip-whitelist/:id` | Publik | Hapus dari daftar IP yang diizinkan |
| POST | `/api/v1/client-auth/ip-whitelist/auto-add` | Publik | Tambahkan IP secara otomatis |
| POST | `/api/v1/client-auth/login` | Publik | Masuk |
| POST | `/api/v1/client-auth/logout` | Publik | Keluar |
| POST | `/api/v1/client-auth/logs/package` | Publik | Buat bundel log |
| GET | `/api/v1/client-auth/logs/packages` | Publik | Daftar bundel log |
| DELETE | `/api/v1/client-auth/logs/packages/:id` | Publik | Hapus bundel log |
| GET | `/api/v1/client-auth/logs/packages/:id` | Publik | Dapatkan bundel log |
| GET | `/api/v1/client-auth/logs/packages/:id/download` | Publik | Unduh bundel log |
| GET | `/api/v1/client-auth/logs/types` | Publik | Pemeriksaan kesehatan bundel log |
| GET | `/api/v1/client-auth/me` | Publik | Dapatkan pengguna yang masuk saat ini |
| GET | `/api/v1/client-auth/nodes` | Publik | Daftar simpul saya |
| GET | `/api/v1/client-auth/nodes/:id/config` | Publik | Konfigurasi simpul |
| GET | `/api/v1/client-auth/nodes/:id/link` | Publik | Tautan berbagi simpul |
| GET | `/api/v1/client-auth/nodes/:id/qrcode` | Publik | Kode QR simpul |
| GET | `/api/v1/client-auth/oauth/:provider` | Publik | Otorisasi |
| GET | `/api/v1/client-auth/oauth/:provider/callback` | Publik | Panggilan balik |
| GET | `/api/v1/client-auth/oauth/providers` | Publik | Status penyedia |
| GET | `/api/v1/client-auth/orders` | Publik | Daftar pesanan |
| POST | `/api/v1/client-auth/orders` | Publik | Buat pesanan |
| GET | `/api/v1/client-auth/orders/:id` | Publik | Detail pesanan |
| GET | `/api/v1/client-auth/plans` | Publik | Daftar rencana |
| GET | `/api/v1/client-auth/plans/:id` | Publik | Detail rencana |
| GET | `/api/v1/client-auth/profile` | Publik | Dapatkan profil |
| PUT | `/api/v1/client-auth/profile` | Publik | Perbarui profil |
| POST | `/api/v1/client-auth/profile/bind-oauth` | Publik | Ikat akun pihak ketiga |
| DELETE | `/api/v1/client-auth/profile/unbind-oauth/:provider` | Publik | Lepas ikatan akun pihak ketiga |
| GET | `/api/v1/client-auth/referral/code` | Publik | Dapatkan kode verifikasi rujukan |
| GET | `/api/v1/client-auth/referral/records` | Publik | Daftar catatan rujukan |
| GET | `/api/v1/client-auth/referral/stats` | Publik | Dapatkan statistik rujukan |
| POST | `/api/v1/client-auth/referral/withdraw` | Publik | Minta penarikan |
| GET | `/api/v1/client-auth/referral/withdrawals` | Publik | Daftar penarikan rujukan |
| POST | `/api/v1/client-auth/refresh` | Publik | Segarkan |
| POST | `/api/v1/client-auth/register` | Publik | Daftar |
| POST | `/api/v1/client-auth/reset-subscription` | Publik | Setel ulang langganan |
| POST | `/api/v1/client-auth/rotate-token` | Publik | Putar token berlangganan |
| POST | `/api/v1/client-auth/self-unban` | Publik | Pembatalan larangan layanan mandiri |
| PUT | `/api/v1/client-auth/settings` | Publik | Perbarui pengaturan |
| POST | `/api/v1/client-auth/settings/auto-add-ip` | Publik | Tambahkan IP secara otomatis |
| GET | `/api/v1/client-auth/settings/whitelist` | Publik | Daftar daftar IP yang diizinkan |
| DELETE | `/api/v1/client-auth/settings/whitelist/:id` | Publik | Hapus dari daftar IP yang diizinkan |
| GET | `/api/v1/client-auth/shared-accounts` | Publik | Daftar akun bersama |
| GET | `/api/v1/client-auth/shared-accounts/:id` | Publik | Dapatkan akun bersama |
| POST | `/api/v1/client-auth/shared-accounts/:id/apply` | Publik | Minta akun bersama |
| POST | `/api/v1/client-auth/shared-accounts/:id/repair` | Publik | Permintaan perbaikan |
| POST | `/api/v1/client-auth/ticket` | Publik | Menerbitkan tiket berumur pendek satu kali |
| GET | `/api/v1/client-auth/tickets` | Publik | Daftar tiket |
| POST | `/api/v1/client-auth/tickets` | Publik | Buat tiket |
| GET | `/api/v1/client-auth/tickets/:id` | Publik | Dapatkan tiket |
| POST | `/api/v1/client-auth/tickets/:id/close` | Publik | Tutup tiket |
| POST | `/api/v1/client-auth/tickets/:id/reply` | Publik | Membalas tiket |
| GET | `/api/v1/client-auth/traffic-history` | Publik | Sejarah lalu lintas |
| GET | `/api/v1/client-auth/traffic-packs` | Publik | Daftar paket lalu lintas |
| POST | `/api/v1/client-auth/traffic-packs/buy` | Publik | Beli paket lalu lintas |

### Pengiriman langganan

Titik masuk publik bagi klien untuk mengambil langganan; lihat `subscription.en.md`.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/sub/:token` | Publik | Sajikan langganan |

### Alur kerja konfigurasi runtime

Konfigurasikan snapshot / dry-run / apply / rollback (`apply`, `rollback` dan `disable-proxy` memerlukan admin).

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| POST | `/runtime/apply` | Publik | Terapkan |
| GET | `/runtime/apply-jobs` | Masuk | Daftar pekerjaan |
| GET | `/runtime/apply-jobs/:id` | Masuk | Dapatkan pekerjaan |
| POST | `/runtime/apply-jobs/:id/cancel` | Publik | Batalkan pekerjaan |
| POST | `/runtime/apply-jobs/:id/retry` | Publik | Coba lagi pekerjaan |
| POST | `/runtime/disable-proxy` | Publik | Nonaktifkan proksi |
| POST | `/runtime/dry-run` | Masuk | Eksekusi uji coba |
| GET | `/runtime/nodes` | Masuk | Daftar node |
| GET | `/runtime/overview` | Masuk | Ikhtisar |
| POST | `/runtime/rollback` | Publik | Putar kembali |
| GET | `/runtime/snapshots` | Masuk | Daftar cuplikan |
| POST | `/runtime/snapshots` | Masuk | Buat cuplikan |
| DELETE | `/runtime/snapshots/:id` | Publik | Hapus cuplikan |
| GET | `/runtime/snapshots/:id` | Masuk | Dapatkan cuplikan |
| POST | `/runtime/verify` | Masuk | Verifikasi |

### Kebijakan akses layanan (pensiun)

> `/services/:id/access-policies` dan `/services/:id/ip-bans` dihentikan pada 20-10-2026:
> tidak ada panggilan admin dan tidak ada jalur koneksi yang menggunakan kebijakan ini.

### Pemantauan kuota lalu lintas

Tampilan pemantauan kuota dan penggunaan hanya baca.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/traffic/quotas` | Masuk | Daftar kuota |
| GET | `/traffic/quotas/:id/usage` | Masuk | Penggunaan kuota |
| GET | `/traffic/reset-logs` | Masuk | Catatan penyetelan ulang kuota |
| GET | `/traffic/usage-history` | Masuk | Riwayat penggunaan |

### Pemeriksaan kesehatan

Pemeriksaan keaktifan.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/health` | Publik | Pemeriksaan keaktifan; 200 berarti sehat |

### Halaman portal

Halaman Portal SPA.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/p` | Publik | Portal HTML (yang dirender oleh server + runtime Vue) |

### Tantangan ACME

Panggilan balik tantangan HTTP-01.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/.well-known/acme-challenge/:token` | Publik | Panggilan balik tantangan ACME HTTP-01 (memverifikasi domain selama penerbitan) |
