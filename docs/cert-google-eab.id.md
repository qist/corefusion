<!-- source: cert-google-eab.md | locale: id | updated: 2026-10-23 -->
# Panduan CA Publik Google (penerbitan EAB otomatis)

> Bahasa: [简体中文](cert-google-eab.md) · [English](cert-google-eab.en.md) · [繁體中文](cert-google-eab.zh-TW.md) · [Русский](cert-google-eab.ru.md) · [日本語](cert-google-eab.ja.md) · [한국어](cert-google-eab.ko.md) · [Deutsch](cert-google-eab.de.md) · [Français](cert-google-eab.fr.md) · [Español](cert-google-eab.es.md) · [Português](cert-google-eab.pt.md) · [Türkçe](cert-google-eab.tr.md) · Bahasa Indonesia · [ไทย](cert-google-eab.th.md) · [Tiếng Việt](cert-google-eab.vi.md) · [العربية](cert-google-eab.ar.md) · [فارسی](cert-google-eab.fa.md)

> Berlaku untuk: panel v0.12.101+ | Halaman: **Sertifikat → Konfigurasi EAB**
> Kode: `internal/service/certificate/eab_google.go`, `internal/api/handlers/certificates/eab_config.go`

## 1. Mengapa Google memerlukan EAB

| CA | EAB wajib untuk ACME? | Bagaimana cara mendapatkan EAB |
|---|---|---|
| Mari Enkripsi | ❌ Tidak | — |
| ZeroSSL | ✅ Ya | **Tukarkan email dengan satu email** (`api.zerossl.com/acme/eab-credentials-email`, sama seperti acme.sh); panel dapat mengambilnya secara otomatis |
| **CA Publik Google** | ✅ **Ya** | **Harus dibuat dengan otorisasi GCP** (email saja tidak cukup) → fokus halaman ini |

Dengan kata lain, Google **tidak dapat menghindari "otorisasi"**: baik file JSON akun layanan atau token akses sementara.

## 2. Aturan keras Google (baca dulu)

| Aturan | Deskripsi |
|---|---|
| **Gunakan dalam 7 hari** | Kunci EAB **kedaluwarsa** jika tidak digunakan dalam 7 hari dan harus dibuat ulang |
| **Hanya satu akun** | Satu kunci EAB mendaftarkan tepat satu akun ACME; setelah registrasi kunci **tidak valid** dan tidak dapat digunakan kembali |
| **Akun ACME tidak kedaluwarsa** | Akun ACME yang dibuat dengannya **berumur panjang** → **perpanjangan tidak memerlukan EAB** |
| ⚠ **Menghapus proyek GCP akan mematikan semua akun** | Menghapus proyek akan segera membatalkan **semua akun ACME** di bawahnya dan **perpanjangan menjadi tidak mungkin** |

> Kesimpulan: **EAB adalah "kredensial orientasi" satu kali; akun ACME itulah yang bertahan lama.**
> Panel menyimpan akun di `data/acme/accounts/<ca>.json` — **buat cadangan direktori ini**.

## 3. Tiga jalur (mulai dengan jalur A — tanpa berburu menu)

> Titik masuk Cloud Console telah berubah beberapa kali, jadi tautan langsung dan Cloud Shell didahulukan; jalur menu adalah fallback.

### Jalur A (paling mudah, disarankan dulu): satu perintah di GCP Cloud Shell

Cloud Shell adalah terminal browser dengan **gcloud sudah terinstal dan sudah masuk**; tidak ada pengaturan lokal dan tidak ada menu.

1. Buka <https://console.cloud.google.com/cloudshell> (atau ikon `>_` di kanan atas konsol)
2. Pilih proyek (ID proyek terlihat di pemilih proyek di bagian atas):
   ```bash
   gcloud config set project <your-project-id>
   ```
3. Aktifkan CA Publik API (**setara** dengan mengaktifkannya di Perpustakaan API):
   ```bash
   gcloud services enable publicca.googleapis.com
   ```
4. Buat EAB (**ini adalah dua string yang Anda perlukan**):
   ```bash
   gcloud publicca external-account-keys create
   ```
 Pada keluaran: `keyId` → panel **EAB-KID**; `b64MacKey` → **EAB-HMAC-Key** pada panel
5. Kembali ke panel: **Sertifikat → Konfigurasi EAB → Tambahkan konfigurasi EAB → CA: Google Public CA** →
 masukkan kedua nilai tersebut ke dalam `EAB-KID` / `EAB-HMAC-Key` → simpan ✓

> ⚠ Biaya: Kunci Google EAB adalah **7 hari, satu kali** ✗ → setelah 7 hari (atau saat berpindah akun) **ulangi langkah 4**.
> Untuk menghindari hal tersebut, gunakan jalur B (simpan akun layanan satu kali dan biarkan panel meminta ulang secara otomatis).

### Jalur B (pengaturan satu kali, lalu otomatis): kunci akun layanan JSON

**① Aktifkan CA Publik API** (tautan langsung paling mudah)
<https://console.cloud.google.com/apis/library/publicca.googleapis.com?project=YOUR_PROJECT_ID> → klik “Aktifkan”
Pengganti: ☰ → **API & Layanan → Perpustakaan** → cari `Public CA` → buka → “Aktifkan”

**② Buat akun layanan**
<https://console.cloud.google.com/iam-admin/serviceaccounts?project=YOUR_PROJECT_ID>
→ “**+ Buat akun layanan**” → nama apa pun (misalnya `corefusion-eab`) → “Buat dan lanjutkan” → Anda dapat **melewati** langkah peran → “Selesai”
Penggantian: ☰ → **IAM & Admin → Akun layanan**

**③ Berikan peran minimal** (paling mudah di halaman yang sama)
Klik akun yang baru dibuat → **tab “Izin”** → “Berikan akses” → prinsipal baru: **email akun layanan**
→ cari peran untuk `publicca` → pilih **“Pembuat Kunci Akun Eksternal CA Publik”** → simpan
Pengganti: ☰ → **IAM & Admin → IAM** → “Berikan akses” → email + peran yang sama
⚠ **Jangan berikan Pemilik/Editor** ✗

**④ Unduh kunci JSON**
Akun layanan → buka akun → **tab “Kunci”** → “**Tambahkan kunci**” → “**Buat kunci baru**” → **JSON** → buat
(browser mengunduh `xxx.json`; atau baris `⋮` → “Kelola kunci” → alur yang sama)

**⑤ Kembali ke panel**: masukkan **ID proyek** dan tempel **seluruh konten** JSON ke dalam “File kunci akun layanan” → klik “Minta EAB secara otomatis”
→ setelah itu panel **meminta ulang EAB secara otomatis** ketika masa berlakunya habis ✓, tidak ada lagi kunjungan GCP ✓

### Jalur C (hanya pengujian satu kali): tempelkan token akses

Di Cloud Shell:
```bash
gcloud auth print-access-token
```
Tempel hasilnya ke dalam panel “Token akses sementara” ditambah ID proyek → klik “Minta EAB secara otomatis”
⚠ Berlaku sekitar 1 jam dan **tidak disimpan** ✓ → verifikasi saja, tidak ada permintaan ulang otomatis ✗

### Lembar contekan ketika Anda tidak dapat menemukan titik masuk (menu konsol berubah)

| Tugas | Tautan langsung (paling dapat diandalkan) | Pengganti: menu |
|---|---|---|
| Aktifkan CA Publik API | `https://console.cloud.google.com/apis/library/publicca.googleapis.com?project=<PROJECT_ID>` | ☰ → API & Layanan → Perpustakaan → cari `Public CA` |
| Akun layanan | `https://console.cloud.google.com/iam-admin/serviceaccounts?project=<PROJECT_ID>` | ☰ → IAM & Admin → Akun layanan |
| Berikan peran | `https://console.cloud.google.com/iam-admin/iam?project=<PROJECT_ID>` | ☰ → IAM & Admin → IAM → Berikan akses |
| Cangkang Awan | `https://console.cloud.google.com/cloudshell` | Ikon `>_` di kanan atas |

> Tip: ganti `<PROJECT_ID>` / `YOUR_PROJECT_ID` dengan ID proyek sebenarnya (seperti `my-project-123456`) untuk langsung membuka halaman.

## 4. Mengonfigurasi panel (dua cara)

Buka **Sertifikat → Konfigurasi EAB → Tambahkan konfigurasi EAB** dan pilih **Google Public CA**:

### Opsi A (disarankan): tempelkan akun layanan JSON

| Bidang | Nilai |
|---|---|
| ID proyek GCP | ID proyek dari bagian 3, mis. `my-project-123456` |
| Akun layanan JSON | Tempel **seluruh konten** `corefusion-eab.json` |
| Token akses sementara | Biarkan kosong |
| Nama | Dibuat secara otomatis sebagai `Google Public CA - <project-id>`; dapat diedit |

Klik **Minta EAB secara otomatis** → jika berhasil, kunci KID/HMAC diisi dan disimpan; nanti pilih saja `CA = Google Public CA` saat menerbitkan (EAB dikaitkan per CA; tidak ada pilihan manual).

> Akun layanan JSON **dienkripsi ke dalam konfigurasi EAB database panel** (`json:"-"`, **tidak pernah dikembalikan ke frontend**).
> Tujuannya adalah untuk **meminta ulang kunci secara otomatis setelah masa berlaku 7 hari**, bukan kunjungan GCP manual.

### Opsi B: token akses sementara (satu kali)

```bash
gcloud auth print-access-token        # valid ~1 hour
```

Tempelkan ke “Token akses sementara”, isi ID proyek, klik **Minta EAB secara otomatis**.
⚠ Token ini **tidak disimpan** dan harus diperoleh kembali setelah habis masa berlakunya → baik untuk pengujian satu kali, bukan untuk penggunaan jangka panjang.

### Setara dengan baris perintah (opsional, untuk perbandingan)

```bash
gcloud publicca external-account-keys create
# returns keyId / b64MacKey, corresponding to ACME's --eab-kid / --eab-hmac-key
```

## 4.5 Mengapa JSON bukannya "satu kunci"?

Ini adalah pertanyaan paling umum. Pisahkan dua jenis “kunci”:

| Apa yang mungkin Anda harapkan | Apa yang sebenarnya dibutuhkan |
|---|---|
| **Kunci API** (string `AIza…`) | ❌ **Tidak berguna** — kunci API hanya merupakan pengenal publik dan **tidak membawa identitas**; membuat EAB memerlukan **izin IAM**, sehingga kredensial penegasan identitas diperlukan |
| — | ✅ **Kunci akun layanan**: **file** yang diunduh dari GCP “IAM → Akun layanan → Kunci → Buat kunci baru → **JSON**” |

Poin penting: **file JSON itu sendiri adalah "kunci"** — GCP mengemas identitas akun (`client_email`) ditambah kunci pribadi (`private_key`) ke dalam satu file. Panel menandatangani JWT dengannya → menukar token akses OAuth2 → memanggil CA Publik API.
Jadi “paste JSON” ≠ mengisi banyak kolom; **tempel seluruh file** (panel juga mengisi kolom opsional seperti `type` / `token_uri`).

**EAB yang sebenarnya adalah dua string** (`EAB-KID` + `EAB-HMAC-Key`), yang merupakan **hasil** dari permintaan; panel mengisinya secara otomatis.

### Tidak ingin menyerahkan berkasnya? Dua alternatif

| Metode | Bagaimana | Biaya |
|---|---|---|
| **Token akses sementara** | `gcloud auth print-access-token` → tempel ke “Token akses sementara” | ~Validitas 1 jam, **tidak disimpan** → hanya sekali, tidak ada permintaan ulang otomatis |
| **Manual ANAK/HMAC** | Jalankan `gcloud publicca external-account-keys create` secara lokal → masukkan `keyId` / `b64MacKey` ke dalam “EAB-KID / EAB-HMAC-Key” | Tidak ada kredensial yang diserahkan ✓; tapi Google EAB adalah **7 hari, satu kali** → ulangi setelah habis masa berlakunya |

> Panel merekomendasikan akun layanan JSON karena satu alasan: **EAB kedaluwarsa setelah 7 hari**, dan hanya akun layanan tersimpan yang mengizinkan **permintaan ulang otomatis** tanpa pekerjaan manual.

## 4.6 FAQ: bisakah saya menggunakan kunci API?

**Tidak.** Ini bukan pilihan penerapan — **Google secara eksplisit menolaknya** (diverifikasi dalam dua cara):

```bash
# 1) Query parameter
curl -X POST "https://publicca.googleapis.com/v1/projects/<ID>/locations/global/externalAccountKeys?key=AIza..." -d '{}'
# 2) x-goog-api-key header recommended by the official docs
curl -X POST "https://publicca.googleapis.com/v1/projects/<ID>/locations/global/externalAccountKeys" \
     -H "x-goog-api-key: AIza..." -d '{}'
```

Keduanya kembali:

```json
{"error":{"code":401,"status":"UNAUTHENTICATED",
  "message":"API keys are not supported by this API. Expected OAuth2 access token
             or other authentication credentials that assert a principal."}}
```

Sebagai perbandingan, **(palsu) OAuth Bearer** kembali
`Request had invalid authentication credentials. Expected OAuth 2 access token...` —
jadi OAuth dikenali sementara **kunci API langsung ditolak** ✗.

**Mengapa**: membuat EAB adalah **operasi administratif** yang mengharuskan Anda mengetahui “siapa Anda dan apakah Anda memiliki `roles/publicca.externalAccountKeyCreator`” ✗
— kunci API hanya mengidentifikasi proyek/aplikasi ✓ dan **tidak menyatakan identitas utama** ✓.
("Praktik terbaik untuk mengelola kunci API" Google merekomendasikan **IAM + kredensial akun layanan berumur pendek** untuk skenario administratif ✓ — pendekatan yang sama digunakan di sini ✓.)

**Jadi hanya ada tiga jalur** ✓ (lihat §3): akun layanan JSON ✓ / token akses sementara ✓ /
atau **tidak ada kredensial sama sekali** — jalankan `gcloud publicca external-account-keys create` di Cloud Shell ✓
dan masukkan `keyId` / `b64MacKey` ke dalam panel ✓ (**gunakan ini jika GCP IAM memblokir Anda** ✓).

## 5. Pendaftaran dan perpanjangan otomatis (penting)

```
Panel: CA = Google Public CA
  ├─ usable EAB present in the config → use it
  └─ missing/expired → request a new one on the spot with the stored service-account JSON (auto-registration)
        │
First issuance on the node/panel ── registers the ACME account with EAB (one-time!)
        │
Account persisted: data/acme/accounts/google.json (panel) / <Agent data_dir>/acme/accounts/google.json (node)
        │
Every later renewal ── uses the account key only; **EAB is never needed again**
```

Oleh karena itu:

- **Penerbitan pertama terdaftar secara otomatis**; tidak diperlukan langkah GCP manual;
- Perpanjangan **tidak** menggunakan EAB (selama file akun ada dan direktori data tetap stabil);
- ⚠ Direktori data ACME sisi Agen adalah jalur absolut tetap (`/var/lib/corefusion-agent/acme` default);
 **jangan mengubahnya begitu saja**, atau akan dianggap sebagai akun baru → diperlukan kunci EAB lainnya.

## 6. Pemecahan masalah

| Gejala/kesalahan | Sebab | Perbaiki |
|---|---|---|
| `PERMISSION_DENIED` (403) | Akun layanan tidak memiliki `roles/publicca.externalAccountKeyCreator` | Kembali ke bagian 3 ④ |
| `SERVICE_DISABLED` (403) | `publicca.googleapis.com` tidak diaktifkan untuk proyek ini | Bagian 3 ② |
| `NOT_FOUND` (404) | ID proyek salah/proyek dihapus | Verifikasi ID proyek |
| Laporan penerbitan `externalAccountRequired` | CA ini tidak memiliki EAB | Minta/konfigurasi EAB di panel terlebih dahulu |
| Laporan penerbitan `Accounts must have at least one contact.` | Tidak ada email yang diberikan saat pendaftaran | Atur email akun ACME pada halaman sertifikat (panel menggunakan akun default) |
| Perpanjangan minta EAB lagi / registrasi gagal | File akun ACME hilang (direktori data diubah/dibersihkan) | **Pulihkan cadangan**; tanpa satu pun, minta EAB baru menggunakan akun layanan |
| Penerbit berubah dari Google menjadi Let's Encrypt | Perintah pembaruan tidak memiliki CA (bug lama, diperbaiki di v0.12.101) | Tingkatkan dan terbitkan ulang |

## 7. Rekomendasi keamanan dan operasi

1. **Hak istimewa terkecil**: hanya memberikan akun layanan `roles/publicca.externalAccountKeyCreator`, tidak pernah kepada Pemilik/Editor.
2. **Jangan hapus proyek**: menghapus proyek GCP akan membatalkan semua akun ACME → sertifikat tidak dapat diperpanjang.
3. **Selalu cadangan** `data/acme/accounts/` (identitas akun itu sendiri) dan konfigurasi EAB database panel.
4. **Jangan panik tentang masa berlaku EAB**: selama file akun ACME ada, perpanjangan tidak terpengaruh; EAB baru hanya diperlukan untuk mendaftar akun baru.
5. Jika kunci akun layanan bocor: hapus kunci tersebut di GCP dan tempelkan JSON baru ke panel.

## 8. Titik akhir terkait (untuk pembuatan skrip)

| Titik akhir | Deskripsi |
|---|---|
| `POST /api/v1/eab-configs/fetch-google` | Buat EAB menggunakan otorisasi GCP. Tubuh: `{project_id, service_account_json?, access_token?}` |
| `POST /api/v1/eab-configs/fetch-zerossl` | Dapatkan ZeroSSL EAB dengan email. Tubuh: `{email}` |
| `GET  /api/v1/eab-configs` | Cantumkan konfigurasi EAB (**tanpa** kunci HMAC atau akun layanan JSON; hanya `has_mac_key` / `has_google_sa`) |
