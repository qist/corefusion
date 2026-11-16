<!-- source: admin-api.md | locale: id | updated: 2026-11-16 -->
# Admin API

> Bahasa: [简体中文](admin-api.md) · [English](admin-api.en.md) · [繁體中文](admin-api.zh-TW.md) · [Русский](admin-api.ru.md) · [日本語](admin-api.ja.md) · [한국어](admin-api.ko.md) · [Deutsch](admin-api.de.md) · [Français](admin-api.fr.md) · [Español](admin-api.es.md) · [Português](admin-api.pt.md) · [Türkçe](admin-api.tr.md) · Bahasa Indonesia · [ไทย](admin-api.th.md) · [Tiếng Việt](admin-api.vi.md) · [العربية](admin-api.ar.md) · [فارسی](admin-api.fa.md)

Tabel rute lengkap admin API untuk **pengembangan panel/otomatisasi/integrator**.

- Basis URL: `https://<panel-domain>/api/v1`
- Otentikasi: `Authorization: Bearer <admin_token>` (dari `POST /api/v1/auth/login`)
- Format respons/kesalahan umum dan konvensi batas kecepatan: lihat [README.md](./README.id.md)
- Sumber kebenaran: `internal/api/router/router.go` (jumlahnya bervariasi menurut versi; kodenya resmi); jika tabel ini tidak sesuai, **kode menang**

## Penanda izin

| Penanda | Artinya |
|---|---|
| Publik | Tidak perlu login (penjaga pemeliharaan/pembatasan tarif mungkin masih berlaku) |
| Publik · tarif terbatas | Tidak perlu masuk, dengan batas tarif khusus |
| Masuk | Memerlukan token admin masuk (`AuthRequired`) |
| Operator | Membutuhkan peran ≥ `operator` (`RequireRole(RoleOperator)`) |
| Admin | Membutuhkan peran ≥ `admin` (`RequireRole(RoleAdmin)`) |
| Admin super | Membutuhkan peran = `super_admin` |
| Klien (publik) | Tidak diperlukan token klien (masuk/daftar, dll.) |
| Klien (masuk) | Memerlukan `Authorization: Bearer <client_token>` (token klien portal, bukan token admin) |

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
- Konfigurasi [EAB](#eab-configuration)
- Akun [ACME](#acme-accounts)
- [CA](#built-in-ca) bawaan
- [Konfigurasi EAB](#konfigurasi-eab)
- [akun ACME](#akun-acme)
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
| GET | `/api/v1/auth/passkey/rpids` | Admin | Daftar RP ID yang diizinkan |
| POST | `/api/v1/auth/passkey/rpids` | Admin | Perbarui daftar izin ID RP |
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
| POST | `/api/v1/clients` | Operator | Buat pelanggan. tubuh: `{"email":"user@example.com","password":"plaintext-password","plan_id":1,"quota_bytes":10737418240,"expires_at":"2026-12-31T00:00:00Z","remark":""}`. `plan_id` diperlukan; `password` mungkin kosong (pelanggan dapat mengaturnya nanti di portal) |
| DELETE | `/api/v1/clients/:id` | Operator | Hapus pelanggan |
| GET | `/api/v1/clients/:id` | Masuk | Dapatkan pelanggan |
| PUT | `/api/v1/clients/:id` | Operator | Perbarui pelanggan |
| POST | `/api/v1/clients/:id/downgrade` | Operator | Turunkan versi pelanggan |
| GET | `/api/v1/clients/:id/orders` | Masuk | Daftar pesanan pelanggan |
| POST | `/api/v1/clients/:id/renew` | Operator | Perbarui pelanggan |
| POST | `/api/v1/clients/:id/reset-usage` | Operator | Setel ulang penggunaan |
| POST | `/api/v1/clients/:id/restore` | Operator | Lanjutkan pelanggan |
| POST | `/api/v1/clients/:id/suspend` | Operator | Tangguhkan pelanggan |
| POST | `/api/v1/clients/:id/upgrade` | Operator | Tingkatkan pelanggan |

### Templat rencana

Paket (varian, siklus penagihan, harga, kuota lalu lintas) dan otorisasi node tingkat template.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/plan-templates` | Masuk | Daftar templat |
| POST | `/api/v1/plan-templates` | Admin | Buat templat |
| DELETE | `/api/v1/plan-templates/:id` | Admin | Hapus templat |
| GET | `/api/v1/plan-templates/:id` | Masuk | Detail templat |
| PUT | `/api/v1/plan-templates/:id` | Admin | Perbarui templat |
| GET | `/api/v1/plan-templates/:id/accessible-nodes` | Masuk | Dapatkan node yang tersedia |
| GET | `/api/v1/plan-templates/:id/billing-node-options` | Masuk | Dapatkan opsi node yang dapat ditagih |
| PUT | `/api/v1/plan-templates/:id/node-groups` | Admin | Tetapkan grup simpul templat |
| POST | `/api/v1/plan-templates/:id/sync-accounts` | Admin | Sinkronkan akun ke templat |
| GET | `/api/v1/plan-templates/:id/variants` | Masuk | Daftar varian |
| POST | `/api/v1/plan-templates/:id/variants` | Admin | Buat varian |
| POST | `/api/v1/plan-templates/sync-all-accounts` | Admin | Sinkronkan semua akun |

### Varian rencana

Perbarui dan hapus varian paket (pembuatan disarangkan di bawah templat paket).

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| DELETE | `/api/v1/plan-variants/:id` | Admin | Hapus varian |
| PUT | `/api/v1/plan-variants/:id` | Admin | Perbarui varian |

### Paket lalu lintas

Paket lalu lintas yang dapat dibeli secara terpisah.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/traffic-packs` | Masuk | Daftar paket lalu lintas |
| POST | `/api/v1/traffic-packs` | Admin | Buat paket lalu lintas |
| DELETE | `/api/v1/traffic-packs/:id` | Admin | Hapus paket lalu lintas |
| PUT | `/api/v1/traffic-packs/:id` | Admin | Perbarui paket lalu lintas |

### Kupon

Buat, validasi, dan tukarkan kupon.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/promotions` | Masuk | Daftar kupon |
| POST | `/api/v1/promotions` | Admin | Buat kupon |
| DELETE | `/api/v1/promotions/:id` | Admin | Hapus kupon |
| PUT | `/api/v1/promotions/:id` | Admin | Perbarui kupon |
| POST | `/api/v1/promotions/validate` | Masuk | Validasi kupon |

### Grup simpul

Atur node ke dalam kelompok yang direferensikan oleh rencana.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/node-groups` | Masuk | Daftar grup simpul |
| POST | `/api/v1/node-groups` | Admin | Buat grup simpul |
| DELETE | `/api/v1/node-groups/:id` | Admin | Hapus grup simpul |
| PUT | `/api/v1/node-groups/:id` | Admin | Perbarui grup simpul |
| GET | `/api/v1/node-groups/:id/members` | Masuk | Daftar anggota kelompok |
| POST | `/api/v1/node-groups/:id/members` | Admin | Tambahkan anggota grup |
| DELETE | `/api/v1/node-groups/:id/members/:mid` | Admin | Hapus anggota grup |

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
| POST | `/api/v1/node-servers/:id/binaries/reality-public-key` | Admin | Turunkan kunci publik dari kunci pribadi REALITY |
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
| POST | `/api/v1/node-servers/:id/caddy/import` | Admin | Impor situs |
| POST | `/api/v1/node-servers/:id/caddy/preview-site` | Admin | Pratinjau situs Caddy yang dirender |
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
| GET | `/api/v1/node-servers/:id/firewall/status` | Admin | Deteksi firewall (firewalld/ufw/nft/iptables) |
| POST | `/api/v1/node-servers/:id/firewall/allow-port` | Admin | Izinkan port mendengarkan masuk (mendukung multi-rentang) |
| POST | `/api/v1/node-servers/:id/firewall/remove-port` | Admin | Cabut tunjangan port masuk (termasuk duplikat) |
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
| POST | `/api/v1/node-servers/:id/nginx/import` | Admin | Impor situs |
| POST | `/api/v1/node-servers/:id/nginx/preview-site` | Admin | Pratinjau yang diberikan situs Nginx |
| POST | `/api/v1/node-servers/:id/nginx/ssl/generate` | Admin | Hasilkan file sertifikat SSL |
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
| GET | `/api/v1/node-servers/:id/singbox/connections` | Admin | Koneksi sing-box langsung (clash_api, khusus node lokal) |
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
| POST | `/api/v1/node-servers/:id/warp/masque/register` | Admin | Daftarkan profil WARP MASQUE (langkah 1 dari 2) |
| POST | `/api/v1/node-servers/:id/warp/:warpId/masque-outbound` | Admin | Buat keluar sing-box MASQUE dari profil WARP |
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
| GET | `/api/v1/node-audit-logs` | Admin | Log audit simpul |
| GET | `/api/v1/node-audit-logs/export` | Admin | Ekspor log audit simpul |

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
| POST | `/api/v1/subscriptions` | Operator | Buat |
| DELETE | `/api/v1/subscriptions/:id` | Operator | Hapus |
| GET | `/api/v1/subscriptions/:id` | Masuk | Detail |
| PUT | `/api/v1/subscriptions/:id` | Operator | Pembaruan |
| GET | `/api/v1/subscriptions/:id/access-logs` | Masuk | Catatan akses |
| POST | `/api/v1/subscriptions/:id/enable` | Operator | Aktifkan |
| POST | `/api/v1/subscriptions/:id/reset-usage` | Operator | Setel ulang penggunaan |
| POST | `/api/v1/subscriptions/:id/restore` | Operator | Lanjutkan |
| POST | `/api/v1/subscriptions/:id/revoke` | Operator | Cabut |
| POST | `/api/v1/subscriptions/:id/rotate` | Operator | Putar |
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
| POST | `/api/v1/subscription-endpoints` | Admin | Buat |
| DELETE | `/api/v1/subscription-endpoints/:id` | Admin | Hapus |
| GET | `/api/v1/subscription-endpoints/:id` | Masuk | Detail |
| PUT | `/api/v1/subscription-endpoints/:id` | Admin | Pembaruan |

### Bagikan domain

Akses domain/kumpulan IP yang digunakan dalam tautan klien.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/share-domains` | Masuk | Daftar |
| POST | `/api/v1/share-domains` | Admin | Buat |
| DELETE | `/api/v1/share-domains/:id` | Admin | Hapus |
| PUT | `/api/v1/share-domains/:id` | Admin | Pembaruan |
| DELETE | `/api/v1/share-domains/all` | Admin | Hapus semuanya |
| POST | `/api/v1/share-domains/seed` | Admin | Inisialisasi data default |

### IP Cloudflare

Cloudflare CDN Kumpulan IP dan manajemen sumber.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| DELETE | `/api/v1/cfip/entries` | Admin | Hapus entri |
| GET | `/api/v1/cfip/entries` | Masuk | Daftar entri |
| POST | `/api/v1/cfip/entries` | Admin | Tambahkan entri |
| DELETE | `/api/v1/cfip/entries/:id` | Admin | Hapus entri |
| POST | `/api/v1/cfip/fetch-all` | Admin | Ambil semua sumber |
| GET | `/api/v1/cfip/probe-config` | Masuk | Konfigurasi probe IP pilihan CF (node entri + dialer terpasang) |
| PUT | `/api/v1/cfip/probe-config` | Admin | Simpan interval pemeriksaan / batas latensi / simpul entri / dialer |
| POST | `/api/v1/cfip/check` | Admin | Selidiki semua domain/IP pilihan sekarang |
| POST | `/api/v1/cfip/seed` | Admin | Inisialisasi data default |
| GET | `/api/v1/cfip/sources` | Masuk | Daftar sumber data |
| POST | `/api/v1/cfip/sources` | Admin | Buat sumber data |
| DELETE | `/api/v1/cfip/sources/:id` | Admin | Hapus sumber data |
| PUT | `/api/v1/cfip/sources/:id` | Admin | Perbarui sumber data |
| POST | `/api/v1/cfip/sources/:id/fetch` | Admin | Ambil sumber ini |

### Konfigurasi langganan (Mihomo/Clash)

Konfigurasi templat langganan: grup, aturan, penyedia aturan, pratinjau YAML.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/mihomo/configs` | Masuk | Daftar |
| POST | `/api/v1/mihomo/configs` | Admin | Buat |
| DELETE | `/api/v1/mihomo/configs/:id` | Admin | Hapus |
| GET | `/api/v1/mihomo/configs/:id` | Masuk | Detail |
| PUT | `/api/v1/mihomo/configs/:id` | Admin | Pembaruan |
| GET | `/api/v1/mihomo/configs/:id/rules` | Masuk | Daftar aturan |
| POST | `/api/v1/mihomo/configs/:id/rules` | Admin | Buat aturan |
| PUT | `/api/v1/mihomo/configs/:id/rules/order` | Admin | Susun ulang aturan |
| PUT | `/api/v1/mihomo/rules/:id` | Admin | Perbarui aturan |
| DELETE | `/api/v1/mihomo/rules/:id` | Admin | Hapus aturan |

### Isi ulang situs

Integrasi situs pembayaran pihak ketiga: kunci penandatanganan, file verifikasi, verifikasi domain, dokumen integrasi.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/charge-sites` | Admin | Daftar situs |
| POST | `/api/v1/charge-sites` | Admin | Buat situs |
| DELETE | `/api/v1/charge-sites/:id` | Admin | Hapus situs |
| GET | `/api/v1/charge-sites/:id` | Admin | Detail situs |
| PUT | `/api/v1/charge-sites/:id` | Admin | Perbarui situs |
| POST | `/api/v1/charge-sites/:id/clear-prev-secret` | Admin | Hapus kunci lama |
| POST | `/api/v1/charge-sites/:id/generate-verify` | Admin | Hasilkan file verifikasi |
| POST | `/api/v1/charge-sites/:id/generate-verify-file` | Admin | Hasilkan file verifikasi |
| GET | `/api/v1/charge-sites/:id/integration-doc` | Admin | Dapatkan dokumen integrasi |
| POST | `/api/v1/charge-sites/:id/rotate-secret` | Admin | Putar kunci penandatanganan |
| POST | `/api/v1/charge-sites/:id/verify-dns` | Admin | Verifikasi domain DNS |
| POST | `/api/v1/charge-sites/:id/verify-domain` | Admin | Verifikasi tantangan domain HTTP |
| POST | `/api/v1/charge-sites/:id/verify-domain-dns` | Admin | Verifikasi domain DNS |
| POST | `/api/v1/charge-sites/:id/verify-domain-http` | Admin | Verifikasi tantangan domain HTTP |

### Isi ulang pesanan

Isi ulang pertanyaan pesanan, pengembalian uang, ekspor CSV.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/charge-orders` | Admin | Daftar pesanan |
| GET | `/api/v1/charge-orders/:id` | Admin | Detail pesanan |
| POST | `/api/v1/charge-orders/:id/refund` | Admin | Pengembalian dana |
| GET | `/api/v1/charge-orders/export-csv` | Admin | Ekspor pesanan isi ulang CSV |

### Isi ulang kartu

Pembuatan kartu massal, catatan penggunaan, ekspor CSV.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/charge-cards` | Admin | Daftar kartu |
| POST | `/api/v1/charge-cards` | Admin | Buat kartu |
| DELETE | `/api/v1/charge-cards/:id` | Admin | Hapus kartu |
| GET | `/api/v1/charge-cards/:id/usages` | Admin | Catatan penggunaan kartu |
| POST | `/api/v1/charge-cards/batch` | Admin | Buat kartu secara massal |
| GET | `/api/v1/charge-cards/export-csv` | Admin | Ekspor kartu CSV |

### Mengisi acara panggilan balik

Audit acara panggilan balik.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/charge-callback-events` | Masuk | Daftar acara panggilan balik |

### Biaya percobaan ulang panggilan balik

Percobaan ulang manual dari panggilan balik yang gagal.

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
| GET | `/api/v1/client-balances/transactions/export-csv` | Masuk | Ekspor saldo transaksi CSV |

### Hadiah

Mengeluarkan hadiah yang dapat diklaim (hari/lalu lintas).

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/client-gifts` | Masuk | Daftar hadiah |
| POST | `/api/v1/client-gifts` | Admin | Buat hadiah |
| POST | `/api/v1/client-gifts/batch` | Admin | Buat hadiah secara massal |
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
| GET | `/api/v1/referral-withdrawals` | Admin | Daftar permintaan penarikan |
| POST | `/api/v1/referral-withdrawals/:id/approve` | Admin | Setujui penarikan |
| POST | `/api/v1/referral-withdrawals/:id/reject` | Admin | Tolak penarikan |

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
| POST | `/api/v1/tickets/:id/assign` | Operator | Tetapkan tiket |
| POST | `/api/v1/tickets/:id/close` | Operator | Tutup tiket |
| POST | `/api/v1/tickets/:id/reply` | Operator | Membalas tiket |

### Artikel

CRUD untuk artikel portal.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/articles` | Masuk | Daftar artikel |
| POST | `/api/v1/articles` | Admin | Buat artikel |
| DELETE | `/api/v1/articles/:id` | Admin | Hapus artikel |
| GET | `/api/v1/articles/:id` | Masuk | Detail artikel |
| PUT | `/api/v1/articles/:id` | Admin | Perbarui artikel |

### Akun bersama

Catatan penerbitan dan perbaikan untuk akun bersama (streaming, dll.).

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/shared-accounts` | Masuk | Daftar akun bersama |
| POST | `/api/v1/shared-accounts` | Admin | Buat akun bersama |
| DELETE | `/api/v1/shared-accounts/:id` | Admin | Hapus akun bersama |
| GET | `/api/v1/shared-accounts/:id` | Masuk | Detail akun bersama |
| PUT | `/api/v1/shared-accounts/:id` | Admin | Perbarui akun bersama |
| GET | `/api/v1/shared-accounts/:id/repair-logs` | Masuk | Catatan perbaikan |

### Sertifikat

Penerbitan, pengunggahan, pembaruan dan ekspor sertifikat publik (ACME); penerbitan pada node tertentu didukung.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/certificates` | Admin | Daftar |
| POST | `/api/v1/certificates` | Admin | Buat |
| DELETE | `/api/v1/certificates/:id` | Admin | Hapus |
| GET | `/api/v1/certificates/:id` | Admin | Detail |
| PUT | `/api/v1/certificates/:id` | Admin | Pembaruan |
| GET | `/api/v1/certificates/:id/download` | Admin | Unduh |
| POST | `/api/v1/certificates/:id/renew` | Admin | Perbarui pelanggan |
| GET | `/api/v1/certificates/check-port` | Admin | Periksa pelabuhan |
| GET | `/api/v1/certificates/detect-ip` | Admin | Selidiki IP |
| GET | `/api/v1/certificates/export` | Admin | Ekspor |
| POST | `/api/v1/certificates/issue` | Admin | Masalah. tubuh: `{"domains":["example.com","*.example.com"],"challenge_type":"dns-01","dns_channel_id":2,"key_type":"rsa","key_bits":4096}`. `challenge_type` defaultnya adalah http-01; `dns_channel_id` memilih saluran DNS (penyedia DNS yang dikonfigurasi + kredensial) |
| POST | `/api/v1/certificates/issue-on-node` | Admin | Masalah ke node |
| POST | `/api/v1/certificates/upload` | Admin | Unggah |
| GET | `/api/v1/certificate-tasks/:id` | Admin | Status tugas latar belakang kueri |

### Saluran sertifikat

Konfigurasi saluran ACME (CA).

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/cert-channels` | Admin | Daftar |
| POST | `/api/v1/cert-channels` | Admin | Buat |
| DELETE | `/api/v1/cert-channels/:id` | Admin | Hapus |
| PUT | `/api/v1/cert-channels/:id` | Admin | Pembaruan |
| GET | `/api/v1/cert-channels/providers` | Admin | Penyedia |

### Konfigurasi EAB

Kredensial mengikat akun eksternal seperti ZeroSSL / Google.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/eab-configs` | Admin | Daftar |
| POST | `/api/v1/eab-configs` | Admin | Buat |
| DELETE | `/api/v1/eab-configs/:id` | Admin | Hapus |
| PUT | `/api/v1/eab-configs/:id` | Admin | Pembaruan |
| POST | `/api/v1/eab-configs/fetch-google` | Admin | Ambil Google EAB dari GCP |
| POST | `/api/v1/eab-configs/fetch-zerossl` | Admin | Ambil ZeroSSL EAB dengan email dalam satu klik |

### akun ACME

Manajemen akun ACME sisi panel (email kontak).

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/acme-accounts` | Admin | Daftar |
| POST | `/api/v1/acme-accounts` | Admin | Buat |
| DELETE | `/api/v1/acme-accounts/:id` | Admin | Hapus |
| PUT | `/api/v1/acme-accounts/:id` | Admin | Pembaruan |
| GET | `/api/v1/acme-accounts/default` | Admin | Dapatkan default |

### CA bawaan

Panel internal CA: Penerbitan/perpanjangan/pencabutan sertifikat agen, ekspor CRL, antrian distribusi.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/ca/agent-certs` | Admin | Daftar sertifikat agen |
| POST | `/api/v1/ca/agent-certs/:server_id/renew` | Admin | Perpanjang sertifikat Agen |
| GET | `/api/v1/ca/distribution-queue` | Admin | Antrian distribusi sertifikat |
| DELETE | `/api/v1/ca/distribution-queue/:id` | Admin | Hapus entri antrian distribusi |
| POST | `/api/v1/ca/distribution-queue/:id/retry` | Admin | Coba lagi distribusi sertifikat |
| GET | `/api/v1/ca/revocations` | Admin | Daftar pencabutan |
| GET | `/api/v1/ca/revocations/export` | Admin | Ekspor CRL |
| GET | `/api/v1/ca/status` | Admin | status CA |

### Aturan geografis

Sinkronisasi dan pengiriman sumber data dan file geoip/geosite.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/geo-rules/files` | Masuk | Daftar berkas |
| DELETE | `/api/v1/geo-rules/files/:fileId` | Admin | Hapus berkas |
| PUT | `/api/v1/geo-rules/files/:fileId/auto-sync` | Admin | Alihkan sinkronisasi otomatis |
| POST | `/api/v1/geo-rules/files/:fileId/download` | Admin | Unduh |
| GET | `/api/v1/geo-rules/files/:fileId/download-file` | Masuk | Unduh berkas |
| GET | `/api/v1/geo-rules/github-assets` | Masuk | Ambil aset repositori GitHub |
| POST | `/api/v1/geo-rules/push-all` | Admin | Dorong ke semua node |
| POST | `/api/v1/geo-rules/seed` | Admin | Inisialisasi data default |
| GET | `/api/v1/geo-rules/sources` | Masuk | Daftar sumber data |
| POST | `/api/v1/geo-rules/sources` | Admin | Buat sumber data |
| DELETE | `/api/v1/geo-rules/sources/:sourceId` | Admin | Hapus sumber data |
| PUT | `/api/v1/geo-rules/sources/:sourceId` | Admin | Perbarui sumber data |
| GET | `/api/v1/geo-rules/sources/:sourceId/assets` | Masuk | Ambil aset Rilis |
| POST | `/api/v1/geo-rules/sources/:sourceId/files` | Admin | Tambahkan berkas |
| PUT | `/api/v1/geo-rules/sources/:sourceId/schedule` | Admin | Alihkan sinkronisasi terjadwal |
| POST | `/api/v1/geo-rules/sources/:sourceId/select` | Admin | Pilih berkas |
| POST | `/api/v1/geo-rules/sources/:sourceId/sync` | Admin | Sinkronkan sumber data ini |
| GET | `/api/v1/geo-rules/status` | Masuk | Status |
| POST | `/api/v1/geo-rules/sync-all` | Admin | Sinkronkan semua |
| GET | `/api/v1/geo-rules/sync-status/:key` | Masuk | Sinkronkan kemajuan |

### Pustaka aturan WAF

Aturan WAF global: sumber GitHub, sinkronisasi, pengiriman ke semua node.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/waf/github-source` | Masuk | Sumber data GitHub |
| PUT | `/api/v1/waf/github-source` | Admin | Simpan sumber data GitHub |
| POST | `/api/v1/waf/push-all` | Admin | Dorong ke semua node |
| GET | `/api/v1/waf/rules` | Masuk | Daftar aturan |
| DELETE | `/api/v1/waf/rules/:name` | Admin | Hapus aturan |
| GET | `/api/v1/waf/rules/:name` | Masuk | Dapatkan aturan |
| PUT | `/api/v1/waf/rules/:name` | Admin | Simpan aturan |
| POST | `/api/v1/waf/sync-all` | Admin | Sinkronkan semua |
| POST | `/api/v1/waf/sync-github` | Admin | Sinkronisasi dari GitHub |

### Manajemen catatan

Pembacaan dan streaming log node/inti, kebijakan pembersihan, kontrol penulis log.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| POST | `/api/v1/log-manage/cleanup-all` | Admin | Bersihkan semuanya sekarang |
| GET | `/api/v1/log-manage/cleanup-rules` | Admin | Daftar aturan pembersihan |
| PUT | `/api/v1/log-manage/cleanup-rules/:id` | Admin | Perbarui aturan pembersihan |
| POST | `/api/v1/log-manage/cleanup-table/:table` | Admin | Bersihkan tabel yang ditentukan sekarang |
| GET | `/api/v1/log-manage/core-log-output` | Admin | Konfigurasi keluaran log inti |
| PUT | `/api/v1/log-manage/core-log-output` | Admin | Simpan konfigurasi keluaran log inti |
| GET | `/api/v1/log-manage/core/:type/stdout-log` | Admin | Baca log stdout inti |
| GET | `/api/v1/log-manage/core/:type/stdout-stream` | Admin | Streaming inti stdout |
| POST | `/api/v1/log-manage/discover` | Admin | Temukan file log simpul |
| GET | `/api/v1/log-manage/log-output` | Admin | Konfigurasi keluaran log |
| PUT | `/api/v1/log-manage/log-output` | Admin | Simpan konfigurasi keluaran log |
| GET | `/api/v1/log-manage/nodes/:serverId/logs` | Admin | Daftar file log simpul |
| GET | `/api/v1/log-manage/nodes/:serverId/logs/:type` | Admin | Baca log simpul |
| POST | `/api/v1/log-manage/nodes/:serverId/logs/:type/rotate` | Admin | Memutar log simpul |
| GET | `/api/v1/log-manage/nodes/:serverId/logs/:type/stream` | Admin | Log simpul aliran |
| GET | `/api/v1/log-manage/policies` | Admin | Daftar kebijakan |
| POST | `/api/v1/log-manage/policies` | Admin | Buat kebijakan |
| DELETE | `/api/v1/log-manage/policies/:id` | Admin | Hapus kebijakan |
| GET | `/api/v1/log-manage/policies/:id` | Admin | Detail kebijakan |
| PUT | `/api/v1/log-manage/policies/:id` | Admin | Perbarui kebijakan |
| GET | `/api/v1/log-manage/policies/:id/jobs` | Admin | Daftar pekerjaan pembersihan |
| GET | `/api/v1/log-manage/policies/:id/log` | Admin | Baca log |
| GET | `/api/v1/log-manage/policies/:id/log-stream` | Admin | Log aliran |
| POST | `/api/v1/log-manage/policies/:id/rotate` | Admin | Putar |
| GET | `/api/v1/log-manage/proxy/:type/stdout-log` | Admin | Baca log stdout inti |
| GET | `/api/v1/log-manage/proxy/:type/stdout-stream` | Admin | Streaming inti stdout |
| GET | `/api/v1/log-manage/scheduler-config` | Admin | Konfigurasi penjadwalan |
| PUT | `/api/v1/log-manage/scheduler-config` | Admin | Perbarui konfigurasi penjadwalan |
| GET | `/api/v1/log-manage/status` | Admin | Status |
| POST | `/api/v1/log-manage/sync` | Admin | Sinkronkan kebijakan pembersihan |
| GET | `/api/v1/log-manage/writers` | Admin | Status penulis |

### Cadangan

Basis data/pencadangan penuh, pemulihan, pekerjaan terjadwal, penyimpanan jarak jauh FTP/S3/WebDAV.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| DELETE | `/api/v1/backups/:id` | Admin super | Hapus cadangan |
| GET | `/api/v1/backups/:id/download` | Admin | Unduh cadangan |
| POST | `/api/v1/backups/:id/restore` | Admin super | Pulihkan dari cadangan |
| POST | `/api/v1/backups/:id/upload-ftp` | Admin super | Unggah ke FTP |
| POST | `/api/v1/backups/:id/upload-s3` | Admin super | Unggah ke S3 |
| POST | `/api/v1/backups/:id/upload-webdav` | Admin super | Unggah ke WebDAV |
| POST | `/api/v1/backups/create` | Admin | Buat cadangan penuh (dengan DB dump) |
| GET | `/api/v1/backups/list` | Admin | Daftar cadangan lengkap |
| POST | `/api/v1/backups/remote-restore` | Admin super | Pulihkan dari jarak jauh |
| GET | `/api/v1/backups/restore-status` | Admin super | Pulihkan kemajuan |
| GET | `/api/v1/backups/schedules` | Admin | Daftar pekerjaan terjadwal |
| POST | `/api/v1/backups/schedules` | Admin | Buat pekerjaan terjadwal |
| DELETE | `/api/v1/backups/schedules/:id` | Admin | Hapus pekerjaan terjadwal |
| PUT | `/api/v1/backups/schedules/:id` | Admin | Perbarui pekerjaan terjadwal |
| GET | `/api/v1/backups/storages` | Admin | Daftar konfigurasi penyimpanan |
| POST | `/api/v1/backups/storages` | Admin | Buat konfigurasi penyimpanan |
| DELETE | `/api/v1/backups/storages/:id` | Admin | Hapus konfigurasi penyimpanan |
| PUT | `/api/v1/backups/storages/:id` | Admin | Perbarui konfigurasi penyimpanan |
| GET | `/api/v1/backups/storages/:id/files` | Admin | Daftar file cadangan jarak jauh |
| POST | `/api/v1/backups/upload-restore` | Admin super | Unggah cadangan dan pulihkan |

### Pengaturan sistem

Konfigurasi dan migrasi database, pengaturan panel, kunci JWT, tes SMTP/Telegram, templat notifikasi, peningkatan online (kebanyakan admin).

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| PUT | `/api/v1/system/bootstrap-finish` | Admin | Selesaikan pengaturan boot pertama |
| GET | `/api/v1/system/bootstrap-state` | Admin | Status booting pertama |
| GET | `/api/v1/system/config-file/:coreType` | Admin | Baca file konfigurasi inti |
| GET | `/api/v1/system/db-config` | Admin | Konfigurasi basis data |
| PUT | `/api/v1/system/db-config` | Admin | Perbarui konfigurasi basis data |
| POST | `/api/v1/system/db-config/migrate` | Admin super | Migrasikan data basis data |
| POST | `/api/v1/system/db-config/switch` | Admin super | Beralih database (dengan migrasi satu langkah) |
| POST | `/api/v1/system/db-config/test` | Admin | Uji koneksi basis data |
| GET | `/api/v1/system/db-pool-stats` | Admin | Statistik kumpulan koneksi |
| GET | `/api/v1/system/db-stats` | Admin | Statistik basis data |
| GET | `/api/v1/system/db-stats/ping` | Admin | Penyelidikan basis data |
| POST | `/api/v1/system/db-stats/reset` | Admin | Setel ulang statistik basis data |
| GET | `/api/v1/system/encryption-key-status` | Admin | Status kunci enkripsi |
| GET | `/api/v1/system/env-config` | Admin | konfigurasi .env |
| PUT | `/api/v1/system/env-config` | Admin | Perbarui konfigurasi .env |
| POST | `/api/v1/system/generate-encryption-key` | Admin super | Hasilkan kunci enkripsi |
| GET | `/api/v1/system/github-config` | Admin | Konfigurasi GitHub |
| PUT | `/api/v1/system/github-config` | Admin | Perbarui konfigurasi GitHub |
| GET | `/api/v1/system/jwt-secret-status` | Admin | Status kunci JWT |
| GET | `/api/v1/system/maintenance` | Publik | Status mode pemeliharaan (publik) |
| PUT | `/api/v1/system/maintenance` | Admin | Alihkan mode pemeliharaan |
| POST | `/api/v1/system/migrate-data` | Admin super | Migrasikan data |
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
| POST | `/api/v1/system/reality-targets/reset` | Admin super | Setel ulang target REALITY ke default |
| GET | `/api/v1/system/reality-local-sni` | Admin | Baca konfigurasi Reality lokal SNI |
| PUT | `/api/v1/system/reality-local-sni` | Admin | Perbarui konfigurasi SNI lokal SNI |
| POST | `/api/v1/system/regenerate-jwt-secret` | Admin super | Regenerasi kunci JWT |
| GET | `/api/v1/system/releases` | Admin | Versi yang tersedia |
| POST | `/api/v1/system/restart` | Admin super | Mulai ulang |
| POST | `/api/v1/system/rollback` | Admin super | Kemunduran sistem |
| GET | `/api/v1/system/settings` | Admin | Baca pengaturan |
| PUT | `/api/v1/system/settings/batch` | Admin | Pengaturan pembaruan massal |
| GET | `/api/v1/system/stats` | Admin | Status sistem |
| GET | `/api/v1/system/subscription-billing-templates` | Admin | Templat penagihan berlangganan |
| PUT | `/api/v1/system/subscription-billing-templates` | Admin | Perbarui templat penagihan langganan |
| POST | `/api/v1/system/telegram-chat-id` | Admin | Dapatkan ID Obrolan Telegram |
| POST | `/api/v1/system/test-smtp` | Admin | Uji SMTP |
| POST | `/api/v1/system/test-telegram` | Admin | Uji Telegram |
| POST | `/api/v1/system/update` | Admin super | Peningkatan sistem |
| GET | `/api/v1/system/update-jobs` | Admin | Tingkatkan daftar pekerjaan |
| POST | `/api/v1/system/upload-mtls-ca` | Admin | Unggah sertifikat mTLS CA |

### Audit keamanan

Audit kueri acara dan ekspor.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/security/audit-events` | Admin | Daftar acara audit |
| GET | `/api/v1/security/audit-events/export` | Admin | Ekspor peristiwa audit |
| GET | `/api/v1/security/subscription-access-logs` | Admin | Log pengambilan langganan (filter berdasarkan ID langganan/IP/email klien) |

### Metrik

Metrik gabungan panel (node/sertifikat/layanan).

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/metrics/certificates` | Admin | Metrik sertifikat |
| GET | `/api/v1/metrics/nodes` | Admin | Metrik simpul |
| GET | `/api/v1/metrics/overview` | Admin | Ikhtisar |

### Templat portal

Templat portal, konfigurasi aktivasi, dan manajemen cache.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/portal/active` | Admin | Templat aktif |
| PUT | `/api/v1/portal/active` | Admin | Perbarui template aktif |
| DELETE | `/api/v1/portal/cache` | Admin | Hapus cache portal |
| GET | `/api/v1/portal/cache` | Admin | Status cache |
| PUT | `/api/v1/portal/cache/dev-mode` | Admin | Alihkan mode pengembangan cache |
| POST | `/api/v1/portal/cache/rebuild` | Admin | Membangun kembali cache portal |
| GET | `/api/v1/portal/lang/builtin` | Admin | Paket bahasa bawaan |
| POST | `/api/v1/portal/reset` | Admin | Kembalikan templat default |
| GET | `/api/v1/portal/templates` | Admin | Daftar |
| POST | `/api/v1/portal/templates` | Admin | Buat |
| DELETE | `/api/v1/portal/templates/:id` | Admin | Hapus |
| GET | `/api/v1/portal/templates/:id` | Admin | Detail |
| PUT | `/api/v1/portal/templates/:id` | Admin | Pembaruan |
| POST | `/api/v1/portal/templates/:id/activate` | Admin | Aktifkan templat |

> `/portal/templates` CRUD adalah **API tingkat lanjut**: UI portal utamanya menggunakan `PUT /api/v1/portal/active`
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
| GET | `/api/v1/agent/origin` | Publik | Alamat asal panel (publik) |

### Biaya entri panggilan balik

Callback platform pembayaran pihak ketiga (publik, tanda tangan terverifikasi).

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
| GET | `/api/v1/public/settings` | Publik | Pengaturan publik (mis. registrasi_diaktifkan; publik) |
| GET | `/api/v1/public/subscription-endpoints` | Publik | Daftar titik akhir langganan yang diaktifkan (publik) |

### Otentikasi klien dan bisnis (set yang sama di mesin admin)

Titik akhir klien identik dengan portal. Mereka menggunakan **token klien** (`Authorization: Bearer <client_token>`), bukan token admin; lihat `portal-api.en.md` untuk detailnya.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/client-auth/articles` | Klien (masuk) | Daftar artikel |
| GET | `/api/v1/client-auth/articles/:slug` | Klien (masuk) | Dapatkan artikel |
| GET | `/api/v1/client-auth/balance` | Klien (masuk) | Dapatkan keseimbangan |
| GET | `/api/v1/client-auth/balance/transactions` | Klien (masuk) | Transaksi saldo |
| POST | `/api/v1/client-auth/change-password` | Klien (masuk) | Ubah kata sandi |
| POST | `/api/v1/client-auth/charge/create` | Klien (masuk) | Buat pesanan isi ulang |
| GET | `/api/v1/client-auth/charge/orders` | Klien (masuk) | Daftar pesanan isi ulang |
| GET | `/api/v1/client-auth/charge/orders/:id` | Klien (masuk) | Dapatkan detail pesanan isi ulang |
| POST | `/api/v1/client-auth/charge/redeem-card` | Klien (masuk) | Tukarkan kartu |
| GET | `/api/v1/client-auth/charge/sites` | Klien (masuk) | Daftar situs isi ulang |
| GET | `/api/v1/client-auth/charge/status/:trade_no` | Klien (masuk) | Status isi ulang kueri |
| GET | `/api/v1/client-auth/checktime` | Klien (masuk) | Dapatkan waktu server (sinkronisasi jam) |
| GET | `/api/v1/client-auth/connection-info` | Klien (masuk) | Dapatkan parameter koneksi paket |
| GET | `/api/v1/client-auth/gifts` | Klien (masuk) | Daftar hadiah |
| POST | `/api/v1/client-auth/gifts/:id/claim` | Klien (masuk) | Klaim hadiah |
| GET | `/api/v1/client-auth/invoices` | Klien (masuk) | Daftar pesanan |
| GET | `/api/v1/client-auth/invoices/:id` | Klien (masuk) | Detail pesanan |
| POST | `/api/v1/client-auth/login` | Klien (publik) | Masuk |
| POST | `/api/v1/client-auth/logout` | Klien (masuk) | Keluar |
| POST | `/api/v1/client-auth/logs/package` | Klien (masuk) | Buat bundel log |
| GET | `/api/v1/client-auth/logs/packages` | Klien (masuk) | Daftar bundel log |
| DELETE | `/api/v1/client-auth/logs/packages/:id` | Klien (masuk) | Hapus bundel log |
| GET | `/api/v1/client-auth/logs/packages/:id` | Klien (masuk) | Dapatkan bundel log |
| GET | `/api/v1/client-auth/logs/packages/:id/download` | Klien (masuk) | Unduh bundel log |
| GET | `/api/v1/client-auth/logs/types` | Klien (masuk) | Pemeriksaan kesehatan bundel log |
| GET | `/api/v1/client-auth/me` | Klien (masuk) | Dapatkan pengguna yang masuk saat ini |
| GET | `/api/v1/client-auth/nodes` | Klien (masuk) | Daftar simpul saya |
| GET | `/api/v1/client-auth/nodes/:id/config` | Klien (masuk) | Konfigurasi simpul |
| GET | `/api/v1/client-auth/nodes/:id/link` | Klien (masuk) | Tautan berbagi simpul |
| GET | `/api/v1/client-auth/nodes/:id/qrcode` | Klien (masuk) | Kode QR simpul |
| GET | `/api/v1/client-auth/oauth/:provider` | Klien (publik) | Otorisasi |
| GET | `/api/v1/client-auth/oauth/:provider/callback` | Klien (publik) | Panggilan balik |
| GET | `/api/v1/client-auth/oauth/providers` | Klien (publik) | Status penyedia |
| GET | `/api/v1/client-auth/orders` | Klien (masuk) | Daftar pesanan |
| POST | `/api/v1/client-auth/orders` | Klien (masuk) | Buat pesanan |
| GET | `/api/v1/client-auth/orders/:id` | Klien (masuk) | Detail pesanan |
| GET | `/api/v1/client-auth/plans` | Klien (masuk) | Daftar rencana |
| GET | `/api/v1/client-auth/plans/:id` | Klien (masuk) | Detail rencana |
| GET | `/api/v1/client-auth/referral/code` | Klien (masuk) | Dapatkan kode verifikasi rujukan |
| GET | `/api/v1/client-auth/referral/records` | Klien (masuk) | Daftar catatan rujukan |
| GET | `/api/v1/client-auth/referral/stats` | Klien (masuk) | Dapatkan statistik rujukan |
| POST | `/api/v1/client-auth/referral/withdraw` | Klien (masuk) | Minta penarikan |
| GET | `/api/v1/client-auth/referral/withdrawals` | Klien (masuk) | Daftar penarikan rujukan |
| POST | `/api/v1/client-auth/refresh` | Klien (publik) | Segarkan |
| POST | `/api/v1/client-auth/register` | Klien (publik) | Daftar |
| POST | `/api/v1/client-auth/reset-subscription` | Klien (masuk) | Setel ulang langganan |
| POST | `/api/v1/client-auth/rotate-token` | Klien (masuk) | Putar token berlangganan |
| POST | `/api/v1/client-auth/self-unban` | Klien (masuk) | Pembatalan larangan layanan mandiri |
| PUT | `/api/v1/client-auth/settings` | Klien (masuk) | Perbarui pengaturan |
| POST | `/api/v1/client-auth/settings/auto-add-ip` | Klien (masuk) | Tambahkan IP secara otomatis |
| GET | `/api/v1/client-auth/settings/whitelist` | Klien (masuk) | Daftar daftar IP yang diizinkan |
| DELETE | `/api/v1/client-auth/settings/whitelist/:id` | Klien (masuk) | Hapus dari daftar IP yang diizinkan |
| GET | `/api/v1/client-auth/shared-accounts` | Klien (masuk) | Daftar akun bersama |
| GET | `/api/v1/client-auth/shared-accounts/:id` | Klien (masuk) | Dapatkan akun bersama |
| POST | `/api/v1/client-auth/shared-accounts/:id/apply` | Klien (masuk) | Minta akun bersama |
| POST | `/api/v1/client-auth/shared-accounts/:id/repair` | Klien (masuk) | Permintaan perbaikan |
| POST | `/api/v1/client-auth/ticket` | Klien (masuk) | Menerbitkan tiket berumur pendek satu kali |
| GET | `/api/v1/client-auth/tickets` | Klien (masuk) | Daftar tiket |
| POST | `/api/v1/client-auth/tickets` | Klien (masuk) | Buat tiket |
| GET | `/api/v1/client-auth/tickets/:id` | Klien (masuk) | Dapatkan tiket |
| POST | `/api/v1/client-auth/tickets/:id/close` | Klien (masuk) | Tutup tiket |
| POST | `/api/v1/client-auth/tickets/:id/reply` | Klien (masuk) | Membalas tiket |
| GET | `/api/v1/client-auth/traffic-history` | Klien (masuk) | Sejarah lalu lintas |
| GET | `/api/v1/client-auth/traffic-packs` | Klien (masuk) | Daftar paket lalu lintas |
| POST | `/api/v1/client-auth/traffic-packs/buy` | Klien (masuk) | Beli paket lalu lintas |

### Pengiriman langganan

Titik masuk publik bagi klien untuk mengambil langganan; lihat `subscription.en.md`.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/sub/:token` | Publik | Sajikan langganan |

### Alur kerja konfigurasi runtime

Konfigurasikan snapshot / dry-run / apply / rollback (`apply`, `rollback` dan `disable-proxy` memerlukan admin).

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| POST | `/api/v1/runtime/apply` | Admin | Terapkan |
| GET | `/api/v1/runtime/apply-jobs` | Masuk | Daftar pekerjaan |
| GET | `/api/v1/runtime/apply-jobs/:id` | Masuk | Dapatkan pekerjaan |
| POST | `/api/v1/runtime/apply-jobs/:id/cancel` | Admin | Batalkan pekerjaan |
| POST | `/api/v1/runtime/apply-jobs/:id/retry` | Admin | Coba lagi pekerjaan |
| POST | `/api/v1/runtime/disable-proxy` | Admin | Nonaktifkan proksi |
| POST | `/api/v1/runtime/dry-run` | Masuk | Eksekusi uji coba |
| GET | `/api/v1/runtime/nodes` | Masuk | Daftar node |
| GET | `/api/v1/runtime/overview` | Masuk | Ikhtisar |
| POST | `/api/v1/runtime/rollback` | Admin | Putar kembali |
| GET | `/api/v1/runtime/snapshots` | Masuk | Daftar cuplikan |
| POST | `/api/v1/runtime/snapshots` | Masuk | Buat cuplikan |
| DELETE | `/api/v1/runtime/snapshots/:id` | Admin | Hapus cuplikan |
| GET | `/api/v1/runtime/snapshots/:id` | Masuk | Dapatkan cuplikan |
| POST | `/api/v1/runtime/verify` | Masuk | Verifikasi |

### Kebijakan akses layanan (pensiun)

> `/services/:id/access-policies` dan `/services/:id/ip-bans` dihentikan pada 20-10-2026:
> tidak ada panggilan admin dan tidak ada jalur koneksi yang menggunakan kebijakan ini.

### Pemantauan kuota lalu lintas

Tampilan pemantauan kuota dan penggunaan hanya baca.

| Metode | Jalan | Izin | Deskripsi |
|---|---|---|---|
| GET | `/api/v1/traffic/quotas` | Masuk | Daftar kuota |
| GET | `/api/v1/traffic/quotas/:id/usage` | Masuk | Penggunaan kuota |
| GET | `/api/v1/traffic/reset-logs` | Masuk | Catatan penyetelan ulang kuota |
| GET | `/api/v1/traffic/usage-history` | Masuk | Riwayat penggunaan |

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
