<!-- source: oauth-google-verification.md | locale: id | updated: 2026-10-23 -->
# Panduan Verifikasi OAuth Google

> Bahasa: [简体中文](oauth-google-verification.md) · [English](oauth-google-verification.en.md) · [繁體中文](oauth-google-verification.zh-TW.md) · [Русский](oauth-google-verification.ru.md) · [日本語](oauth-google-verification.ja.md) · [한국어](oauth-google-verification.ko.md) · [Deutsch](oauth-google-verification.de.md) · [Français](oauth-google-verification.fr.md) · [Español](oauth-google-verification.es.md) · [Português](oauth-google-verification.pt.md) · [Türkçe](oauth-google-verification.tr.md) · Bahasa Indonesia · [ไทย](oauth-google-verification.th.md) · [Tiếng Việt](oauth-google-verification.vi.md) · [العربية](oauth-google-verification.ar.md) · [فارسی](oauth-google-verification.fa.md)

> Cakupan: Google masuk untuk portal CoreFusion (`internal/service/oauth`, cakupan: `openid`, `email`, `profile`).
> Perbandingan GitHub di akhir (GitHub tidak memiliki alur verifikasi).

## 1. Mode pengujian vs produksi

Layar izin OAuth Google memiliki dua status penerbitan:

| Negara | Siapa yang dapat masuk | Diperlukan verifikasi |
|---|---|---|
| **Pengujian** | Hanya akun Google yang ada dalam daftar "pengguna uji". | Tidak |
| **Dalam produksi** | Akun Google apa saja | **Ya** (lihat di bawah) |

Setelah “Pengaturan sistem → OAuth” diaktifkan, halaman masuk portal menampilkan tombol Google kepada setiap pengunjung;
jika aplikasi masih dalam pengujian, pengunjung yang tidak ada dalam daftar pengguna uji akan melihatnya
`Error 403: access_denied` (aplikasi tidak terverifikasi / Anda bukan pengguna uji).
Untuk menawarkan proses masuk Google secara publik, **alihkan aplikasi ke produksi dan lewati verifikasi**.

## 2. Persyaratan sulit untuk verifikasi

Tinjauan Google (verifikasi dasar untuk cakupan tidak terbatas) memerlukan:

1. **Beranda**: URL yang dapat diakses publik di domain resmi (domain atau subdomain yang sama).
2. **Tautan Kebijakan Privasi**: harus **dapat diakses publik** (tidak perlu masuk) dan menjelaskan data apa yang Anda kumpulkan dan cara Anda menggunakan/menyimpan/menghapusnya. Untuk proses masuk Google setidaknya harus mencakup:
 data akun Google yang dikumpulkan (email, nama, avatar), tujuan (membuat/masuk ke akun portal), tidak ada penjualan kepada pihak ketiga,
 dan bagaimana pengguna dapat menghapus akun dan datanya.
3. **Tautan Ketentuan Layanan**: dapat diakses publik, menjelaskan aturan layanan.
4. **Domain resmi**: daftarkan domain aplikasi di layar persetujuan (misalnya `example.com`); tautan home/privacy/terms harus ada di dalamnya atau subdomain.
5. **Logo aplikasi (opsional tetapi disarankan)**: 120×120 piksel.
6. **Pembenaran cakupan**: proyek ini hanya menggunakan `openid / email / profile` (tidak dibatasi, tidak sensitif),
 yang termasuk dalam verifikasi dasar — tidak ada pembenaran per cakupan dan **tidak ada penilaian keamanan pihak ketiga**.
 (Cakupan terbatas seperti akses penuh Gmail akan memerlukannya; proyek ini tidak menggunakan cakupan apa pun.)

Entri verifikasi: [Google Cloud Console](https://console.cloud.google.com/) →
**API & Layanan → Layar persetujuan OAuth → Publikasikan Aplikasi → Kirim untuk verifikasi**.
Verifikasi dasar biasanya selesai dalam beberapa hari; Anda dapat beroperasi dalam mode pengujian untuk sementara.

## 3. Apa yang harus dikonfigurasi untuk proyek ini

### 3.1 Pengalihan resmi URI

Callback dihasilkan secara dinamis dari domain yang memulai proses masuk (lihat `Authorize` di `internal/api/handlers/oauth/handler.go`):

```
https://<portal-domain>/api/v1/client-auth/oauth/google/callback
```

- Masuk dimulai dari domain portal (mis. `https://p.example.com`) → daftarkan `https://p.example.com/...`;
- Dimulai dari domain panel `/p` → daftarkan `https://panel.example.com/...`.
- Google memungkinkan **beberapa** URI pengalihan; mendaftarkan kedua domain.
- ⚠ Google memerlukan URI pengalihan menjadi **https** (kecuali `http://localhost`);
 proxy terbalik harus meneruskan `X-Forwarded-Host` / `X-Forwarded-Proto`, jika tidak, panel akan membuat
 http atau internal redirect_uri → Google menolaknya (`redirect_uri_mismatch`).

Setelah mengatur “Pengaturan sistem → OAuth → Domain panggilan balik portal”, panggilan balik URL yang ditampilkan di halaman adalah yang akan ditempelkan ke Google.

### 3.2 Tautan Kebijakan Privasi/Ketentuan pada portal

Kartu masuk mencakup tautan “Kebijakan Privasi · Ketentuan Layanan” (ditampilkan setelah dikonfigurasi), dan kedua dokumen memiliki **halaman mandiri**
(dialog dan halaman mandiri memiliki konten yang sama):

- Domain panel utama: `https://<panel-domain>/p/legal/privacy`, `/p/legal/terms`
- Domain portal khusus (penyebaran root): `https://<portal-domain>/legal/privacy`, `/legal/terms`
- Mendukung `?lang=xx`; tanpanya, `Accept-Language` browser digunakan, lalu portal default.
- Halaman **dirender di server** (tanpa JS), yang membantu crawler verifikasi Google.

**Konfigurasi (apa pun pilihannya; tautan menang)**:

1. **Mode konten (disarankan, langsung berfungsi)**: editor portal → Pengaturan → Umum → Dokumen kepatuhan →
 pilih "Edit bahasa" dan tempelkan templat §4 ke dalam "Konten kebijakan privasi" / "Konten ketentuan layanan"
 (**Penurunan harga, diedit per bahasa**) → setelah disimpan, kartu masuk menampilkan tautan yang membuka **dialog**;
 “tautan mandiri” editor dapat dimasukkan langsung di layar persetujuan Google.
2. **Mode tautan eksternal**: masukkan “Kebijakan privasi URL” / “Ketentuan layanan URL” (menunjuk ke halaman yang Anda host) → terbuka di tab baru.

Konten dan tautan dikirimkan melalui `/api/v1/public/portal-config` (`privacy_policy_content` /
`terms_of_service_content` adalah **peta penurunan harga per bahasa** `{"zh-CN":"...","en":"..."}`;
`privacy_policy_url` / `terms_of_service_url` adalah tautan eksternal opsional). **Mereka harus dapat diakses tanpa masuk** ✓.

### 3.3 Sakelar sisi panel

`System settings → OAuth`: aktifkan Google dan masukkan ID Klien / Rahasia Klien (rahasianya dienkripsi dengan AES-256-GCM saat disimpan).
Tombol Google muncul di halaman masuk portal hanya ketika semuanya sudah dikonfigurasi (`/api/v1/client-auth/oauth/providers` kembali diaktifkan).

## 4. Templat Kebijakan Privasi / Ketentuan

> Templat kepatuhan minimal yang dapat Anda adaptasi. Sebelum digunakan: ganti placeholder `【】`, tambahkan/hapus klausa untuk bisnis Anda,
> dan meminta entitas pengoperasi untuk meninjaunya. Templat ini bukan nasihat hukum.

### 4.1 Kebijakan Privasi (templat)

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

### 4.2 Ketentuan Layanan (templat)

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

## 5. Daftar periksa pra-peluncuran

- [ ] Layar persetujuan OAuth: nama aplikasi / logo / halaman beranda / kebijakan privasi / ketentuan semuanya ada; domain terdaftar
- [ ] Halaman Kebijakan Privasi dan Ketentuan **dapat diakses tanpa masuk** (halaman khusus portal atau halaman statis mandiri)
- [ ] Kedua tautan terlihat di kartu masuk/footer portal
- [ ] pengalihan URI cocok dengan domain sebenarnya dan merupakan https
- [ ] Publikasikan Aplikasi → kirim untuk verifikasi; pembatasan pengguna uji dicabut setelah disetujui
- [ ] Pengaturan OAuth panel: diaktifkan=true dengan ID Klien/Rahasia terisi (pengembalian titik akhir penyedia diaktifkan:true)

## 6. Perbandingan GitHub

GitHub Aplikasi OAuth **tidak** memiliki status penerbitan atau alur verifikasi: buat aplikasi, setel callback URL, dan aplikasi berfungsi secara publik.
Perhatikan bahwa Aplikasi OAuth GitHub hanya menerima **satu** Panggilan balik otorisasi URL (pencocokan awalan); untuk beberapa domain,
gunakan pendekatan multi-URI dari Google atau buat aplikasi terpisah per domain portal.
