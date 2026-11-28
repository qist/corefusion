<!-- source: USER-GUIDE.md | locale: id | updated: 2026-11-27 -->
# Panduan Pengoperasian CoreFusion

> Bahasa: [简体中文](USER-GUIDE.md) · [English](USER-GUIDE.en.md) · [繁體中文](USER-GUIDE.zh-TW.md) · [Русский](USER-GUIDE.ru.md) · [日本語](USER-GUIDE.ja.md) · [한국어](USER-GUIDE.ko.md) · [Deutsch](USER-GUIDE.de.md) · [Français](USER-GUIDE.fr.md) · [Español](USER-GUIDE.es.md) · [Português](USER-GUIDE.pt.md) · [Türkçe](USER-GUIDE.tr.md) · Bahasa Indonesia · [ไทย](USER-GUIDE.th.md) · [Tiếng Việt](USER-GUIDE.vi.md) · [العربية](USER-GUIDE.ar.md) · [فارسی](USER-GUIDE.fa.md)

> Panduan ini ditujukan untuk pengguna CoreFusion dan mencakup alur kerja lengkap mulai dari instalasi hingga pengoperasian sehari-hari.

---

## Isi

1. [Ikhtisar sistem](#1-ikhtisar-sistem)
2. [Instalasi](#2-instalasi)
3. [Konfigurasi awal](#3-konfigurasi-awal)
4. [Dasbor](#4-dasbor)
5. [Manajemen simpul](#5-manajemen-simpul)
6. [Pusat protokol](#6-pusat-protokol)
7. [Manajemen proksi](#7-manajemen-proksi)
8. [Merutekan sumber daya](#8-merutekan-sumber-daya)
9. [Pusat berlangganan](#9-pusat-berlangganan)
10. [Manajemen pelanggan](#10-manajemen-pelanggan)
11. [Manajemen rencana](#11-manajemen-rencana)
12. [Manajemen isi ulang](#12-manajemen-isi-ulang)
13. [Manajemen tiket](#13-manajemen-tiket)
14. [Manajemen sertifikat](#14-manajemen-sertifikat)
15. [Kuota lalu lintas](#15-kuota-lalu-lintas)
16. [Proxy yang dirantai](#16-proxy-yang-dirantai)
17. [Pengguna daring](#17-pengguna-daring)
18. [Manajemen pengguna](#18-manajemen-pengguna)
19. [Situs portal](#19-situs-portal)
20. [Pengaturan sistem](#20-pengaturan-sistem)
21. [Konsol operasi](#21-konsol-operasi)
22. [Pemantauan basis data](#22-pemantauan-basis-data)
23. [Catatan audit](#23-catatan-audit)
24. [Manajemen catatan](#24-manajemen-catatan)
25. [Ekstensi operasi](#25-ekstensi-operasi)
26. [Pertanyaan Umum](#26-pertanyaan-umum)
- [Lampiran](#lampiran)

---

## 1. Ikhtisar sistem

CoreFusion adalah panel manajemen proxy multi-protokol yang mendukung VLESS, VMess, Trojan, Shadowsocks, Hysteria2, TUIC dan banyak lagi, dengan manajemen node, pengiriman langganan, manajemen pelanggan, penagihan, sertifikat, dan proxy berantai.

### 1.1 Fitur inti

| Modul | Deskripsi |
|--------|-------------|
| Multi-protokol | VLESS, VMess, Trojan, Shadowsocks, Hysteria2, TUIC, WireGuard, dll. |
| Multi-inti | Xray-core dan sing-box (Hysteria2/TUIC dan protokol QUIC lainnya berjalan di sing-box) |
| Multi-simpul | Lima jenis node: mandiri, cermin, aktif/siaga, entri berantai, keluar berantai |
| Pengiriman langganan | Berbagai format langganan, titik akhir, dan manajemen format |
| Penagihan | Templat paket, kartu isi ulang, saldo, pesanan, komisi pengecer |
| Sertifikat | Mari Enkripsi penerbitan, validasi DNS-01/HTTP-01, distribusi sertifikat |
| Proksi berantai | Penyeimbangan beban multi-keluar, registrasi node eksternal, perutean berantai |
| Kuota lalu lintas | Pemantauan lalu lintas bulanan per node, penangguhan otomatis, reset log |
| Pintu gerbang | Editor portal visual, multibahasa, templat khusus |
| Pemberitahuan | Email dan Telegram, templat dan siaran |

### 1.2 Arsitektur

- **Backend**: Mulai, gRPC
- **Bagian depan**: Vue 3 + Elemen Plus
- **Database**: SQLite (default) / MySQL / PostgreSQL
- **Proksi**: Proksi terbalik Caddy / Nginx
- **Port panel**: ditetapkan oleh wizard pengaturan (port bebas acak secara default; 8080 adalah pilihan umum)
- **Port gRPC agen**: 18443 secara default

---

## 2. Instalasi

### 2.1 Lingkungan yang didukung

| Barang | Dukungan |
|------|---------|
| Distribusi | Debian / Ubuntu, RHEL / CentOS / Rocky / AlmaLinux / Fedora, Alpine, Arch, openSUSE |
| Arsitektur CPU | Biner resmi: `amd64`, `arm64`; build sumber juga mendukung `armv7`, `armv6`, `386`, `ppc64le`, `s390x`, `riscv64`, `loong64` |
| Manajer layanan | systemd (disarankan); kembali ke `nohup` dalam kontainer atau lingkungan lain tanpa systemd |
| Memori / disk | ≥ 1 GB / ≥ 10 GB (database, sertifikat, log, dan biner inti berada di direktori data) |
| Pelabuhan panel | dipilih di wizard pengaturan (port bebas acak secara default; `8080` dan sejenisnya baik-baik saja) |
| Agen gRPC | `18443` secara default |

Perintah eksternal runtime diinstal secara otomatis oleh penginstal (`scripts/install.sh` dan skrip instalasi Agen yang dihasilkan oleh panel; lewati dengan `--no-deps` / `CF_SKIP_DEPS=1`):

| Tujuan | Paket |
|---------|---------|
| Firewall / batas kecepatan / penandaan konkurensi | `nftables` (`nft`) |
| Pembentukan lalu lintas, tautan dan soket | `iproute2` (`tc` / `ip` / `ss`) |
| Penggantian ketika nft tidak tersedia | `iptables` (`iptables-nft` pada EL8+, `iptables` pada EL7) |
| Pelacakan koneksi | `conntrack-tools` (`conntrack`) |
| Muat `nf_conntrack` / `ifb` | `kmod` (`modprobe` / `lsmod`) |
| Deteksi dan pembersihan proses | `procps` (`pgrep` / `pkill`) |
| Pemeriksaan port / parameter DH | `lsof` / `openssl` |
| Pembuatan pengguna proxy | `shadow-utils` (Debian `passwd`, Alpine/Arch `shadow`) |
| Unduhan dan arsip | `curl` / `ca-certificates` / `tar` / `gzip` / `xz` |

> `nft` / `tc` yang hilang hanya memengaruhi fitur pembatasan kecepatan dan firewall; panel masih dimulai.
> Jalankan `--cpu-info` terlebih dahulu untuk melihat bagaimana lingkungan terdeteksi dan paket mana yang hilang.

### 2.2 Instalasi satu jalur (disarankan)

```bash
curl -fsSL https://raw.githubusercontent.com/qist/corefusion/main/scripts/install.sh -o install.sh
sudo bash install.sh
```

Atau kirimkan secara langsung (tidak ada interaksi, tidak ada tanda tambahan):

```bash
curl -fsSL https://raw.githubusercontent.com/qist/corefusion/main/scripts/install.sh | sudo bash
```

Skripnya akan:

1. Deteksi distribusi, arsitektur CPU, dan libc, lalu instal paket dasar dan dependensi runtime;
2. Unduh `corefusion-linux-<arch>` dari Rilis GitHub dan verifikasi dengan `.sha256` yang cocok;
3. Instal ke `/opt/corefusion` dan buat `data/{sqlite,certs,logs,runtime}`;
4. Tulis hanya placeholder `.env` dengan `SAFE_PATHS=data` — tanpa konfigurasi database — sehingga permulaan pertama memasuki wizard pengaturan (`.env` lengkap dihasilkan oleh wizard);
5. Instal dan mulai layanan systemd (atau jalankan di bawah `nohup` ketika systemd tidak tersedia);
6. Tunggu hingga siap, lalu print setup URL, token dan path direktori.

Contoh keluaran:

```text
==================================================================
 CoreFusion installation complete
------------------------------------------------------------------
 Install dir : /opt/corefusion
 Data dir    : /opt/corefusion/data
 Binary      : /opt/corefusion/corefusion
 Logs        : journalctl -u corefusion -f
------------------------------------------------------------------
 Setup wizard : http://<server-ip>:<random-port>/init?init_token=<token>
 Setup token  : <token>
 Valid for    : 2 hours (see startup log)
 Note         : the token is shown once; after setup this port closes and the panel switches to its normal port.
==================================================================
```

Jika panel hanya mendengarkan di `127.0.0.1` (misalnya di belakang proksi terbalik), skrip akan mencetak perintah terowongan SSH dan bukan URL yang tidak dapat dijangkau.

> Kehilangan tokennya? Baca lagi log startup: `journalctl -u corefusion -n 50 --no-pager`
> (tanpa systemd: `tail -n 50 /opt/corefusion/data/logs/panel.log`).

### 2.2.1 Keluaran multibahasa

Penginstal mencakup empat bahasa UI: Cina Sederhana, Inggris, Persia (فارسی), dan Rusia (Русский). Ini secara otomatis mendeteksi sistem `LANG`, atau Anda dapat memilih secara eksplisit:

```bash
# English
sudo bash install.sh --lang en

# Persian via environment variable
sudo CF_LANG=fa bash install.sh

# Pass options through a pipe
curl -fsSL https://raw.githubusercontent.com/qist/corefusion/main/scripts/install.sh | sudo bash -s -- --lang ru
```

Nilai yang didukung: `zh` / `en` / `fa` / `ru`; `--lang=ru` juga diterima. Nilai eksplisit yang tidak didukung gagal dengan daftar bahasa yang tersedia.

### Penyematan versi 2.3 / mirror / instalasi offline

```bash
# Pin a version
sudo bash install.sh --version v0.14.5

# GitHub download mirror (used as a URL prefix)
sudo bash install.sh --mirror https://ghproxy.net --version latest

# Use a local binary (offline / intranet / pre-built)
sudo bash install.sh --local ./dist/corefusion-linux-arm64

# Explicit target architecture
sudo bash install.sh --arch arm64 --local ./corefusion-linux-arm64
```

> Tanpa `--version`, skrip menggunakan entri terbaru dalam daftar Rilis GitHub.
> Sebagian besar rilis adalah pra-rilis, sehingga titik akhir `/releases/latest` tidak digunakan.
>
> Rilis resmi hanya menyediakan `amd64` / `arm64`; gunakan `--source` atau biner kompilasi silang dengan `--local` untuk arsitektur lain.

### 2.4 Membangun dari sumbernya

```bash
sudo bash install.sh --source --version v0.14.5
```

`--source` mengkloning repositori (atau menggunakan checkout yang ada saat skrip ada di dalamnya) dan menjalankan `make panel`. Versi Toolchain dipilih per lingkungan:

| Ketergantungan | Seleksi |
|------------|-----------|
| Pergi | Baca dari `toolchain` / `go` di `go.mod` (saat ini **Go 1.27+**); jika versi sistem terlalu lama, rantai alat yang cocok diunduh ke `<install-dir>/.toolchain` |
| simpul | **Node 22 LTS** secara default (Vite memerlukan ≥ 18); versi resmi yang cocok diunduh bila diperlukan |
| Simpul (glibc <2.28) | Menggunakan build `unofficial-builds` glibc-217 (mis. CentOS 7) |
| Node (Alpine / musl) | Menggunakan distribusi `nodejs` / `npm` |

Penggantian:

```bash
# Force Go / Node versions
sudo CF_GO_VERSION=1.27.5 CF_NODE_MAJOR=22 bash install.sh --source

# Go toolchain mirror (default https://go.dev/dl)
sudo CF_GO_MIRROR=https://golang.google.cn/dl bash install.sh --source
```

Pembuatan manual (pengembangan/debug):

```bash
git clone https://github.com/qist/corefusion.git
cd corefusion

# Frontend + panel for the current architecture; the frontend is embedded via go:embed
make panel

# Cross-compile
make panel-linux-amd64
make panel-linux-arm64
```

> `go build ./...` tidak menghasilkan biner. Bangun panel dengan
> `go build -o corefusion ./cmd/corefusion/` atau `make panel`.

### 2.5 Bendera penginstal

| Bendera | Deskripsi |
|------|-------------|
| `-d, --dir <PATH>` | Instal direktori, default `/opt/corefusion` |
| `-v, --version <TAG>` | Versi panel, mis. `v0.14.5`; bawaan `latest` |
| `--binary` | Gunakan biner Rilis GitHub (default) |
| `--source` | Bangun dari sumber dan siapkan Go/Node.js yang cocok |
| `--local <FILE>` | Gunakan biner lokal (instal offline) |
| `--arch <ARCH>` | Arsitektur target: `amd64` / `arm64` / `armv7` / `armv6` / `386` / `ppc64le` / `s390x` / `riscv64` / `loong64` |
| `--mirror <URL>` | Awalan unduhan GitHub, mis. `https://ghproxy.net` |
| `--repo <URL>` | Repositori sumber, `https://github.com/qist/corefusion.git` default |
| `--ref <REF>` | Cabang sumber/tag; defaultnya adalah `--version`, jika tidak `main` |
| `--no-deps` | Lewati instalasi ketergantungan sistem |
| `--no-start` | Instal dan tulis file layanan tanpa memulai |
| `--cpu-info` | Cetak deteksi lingkungan dan dependensi yang hilang, lalu keluar |
| `--lang <LANG>` | Bahasa keluaran: `zh` / `en` / `fa` / `ru` (default: sistem `LANG`) |
| `--uninstall` | Hentikan layanan dan hapus program (menyimpan data dan `.env`) |
| `--purge` | Dengan `--uninstall`, hapus juga direktori data dan `.env` |

> Rilis resmi hanya mengirimkan biner `amd64` / `arm64`. Arsitektur lain memerlukan `--source`,
> jika tidak, unduhan akan menghasilkan 404.

Variabel lingkungan terkait:

| Variabel | Tujuan |
|----------|---------|
| `CF_GO_VERSION` | Ganti versi Go yang diperlukan untuk build sumber |
| `CF_NODE_MAJOR` | Ganti versi utama Node (default 22) |
| `CF_GO_MIRROR` | Buka cermin unduhan toolchain |
| `CF_SKIP_DEPS` | Atur ke `1` dalam skrip pemasangan Agen untuk melewati pemasangan ketergantungan |
| `CF_LANG` | Ganti bahasa keluaran penginstal (sama seperti `--lang`) |

### 2.6 Tata letak direktori

```text
/opt/corefusion/
├── corefusion                 # panel binary
├── corefusion.bak             # previous version (when upgrading)
├── .env                       # environment file (0600, generated by the setup wizard)
├── .env.secret                # encryption key file (0600, generated on first start)
├── .toolchain/                # Go / Node downloaded for source builds
└── data/                      # data directory (SAFE_PATHS)
    ├── sqlite/app.db          # default SQLite database
    ├── certs/                 # certificates and keys
    ├── logs/                  # runtime logs (without systemd)
    ├── runtime/               # core runtime config and state
    ├── binaries/              # downloaded xray / sing-box cores
    └── proxy/                 # proxy (Caddy/Nginx) sites and config
```

> Pemasang hanya membuat `sqlite/`, `certs/`, `logs/` dan `runtime/`; `binaries/`, `proxy/`, `rules/` dan lainnya dibuat oleh panel atau saat inti dipasang.
> Direktori data berasal dari `SAFE_PATHS` di `.env` dan mungkin merupakan jalur absolut. Mengubah direktori instalasi tidak memindahkan data yang ada.

### 2.7 Manajemen layanan

systemd (sebagian besar distribusi):

```bash
systemctl status corefusion --no-pager
systemctl restart corefusion
systemctl stop corefusion
journalctl -u corefusion -f          # follow logs
journalctl -u corefusion -n 100      # last 100 lines
```

Tanpa systemd (wadah, dll.):

```bash
cd /opt/corefusion && ./corefusion            # foreground (debugging)
cd /opt/corefusion && setsid nohup ./corefusion >> data/logs/panel.log 2>&1 &
tail -f data/logs/panel.log
```

> Tanpa systemd, proses tidak dimulai saat boot; restart secara manual setelah reboot (atau jalankan kembali penginstal).

### 2.8 Tingkatkan dan hapus instalan

Tingkatkan (menyimpan data dan konfigurasi):

```bash
sudo bash install.sh --version <new-version>
```

Skrip menghentikan layanan, mencadangkan biner lama sebagai `corefusion.bak`, menggantinya, dan memulai ulang. `.env` dan `data/` yang ada tidak tersentuh.

Copot pemasangan:

```bash
# Remove the program only; keep data and .env
sudo bash install.sh --uninstall

# Also remove the data directory and .env (irreversible)
sudo bash install.sh --uninstall --purge
```

**Sejak v0.14.4 kredensial langganan dienkripsi saat disimpan**: pada start pertama, token langganan, kode pendek, riwayat rotasi, dan URL langganan upstream dienkripsi, dan tabel pencabutan tautan pendek dibuat. Cadangkan database sebelum meningkatkan; rollback juga harus memulihkan database sebelum peningkatan, karena biner lama tidak dapat membaca database baru dan semua langganan akan 404.

### 2.9 File lingkungan (.env)

`.env` yang lengkap **dihasilkan oleh wizard pengaturan** saat konfigurasi disimpan (database DSN, direktori data, rahasia, port, domain).

Pada instalasi baru, penginstal hanya menulis placeholder ini, sehingga panel tidak salah mengira `data/` yang ada sebagai instalasi yang diinisialisasi dan melewati wizard:

```env
SAFE_PATHS=data
```

> Ketika konfigurasi lengkap (`.env` dengan `CF_DB_DSN`) sudah ada, penginstal tidak pernah menimpanya dan wizard tidak muncul lagi.
> Pengeditan manual selanjutnya pada `.env` akan berlaku setelah restart.

#### Direktori data

```env
SAFE_PATHS=data                       # relative to the install directory
# SAFE_PATHS=/srv/corefusion-data     # or an absolute path
```

#### Basis data

`CF_DB_DSN` terdeteksi oleh awalan:

| Awalan | Basis data |
|--------|----------|
| tidak disetel atau jalan kosong | SQLite (standar) |
| `sqlite://` | SQLite (eksplisit) |
| `mysql://` | MySQL |
| `postgres://` / `postgresql://` | PostgreSQL |

`CF_DB_TYPE` bersifat opsional dan mengesampingkan deteksi (`sqlite` / `mysql` / `postgres`).

> **Kata sandi terenkripsi**: DSN MySQL/PostgreSQL yang ditulis oleh panel (wizard pengaturan / switch database)
> disimpan di `.env` sebagai `enc:...` (AES-256-GCM, kunci `CF_SECRET_ENCRYPTION_KEY`).
> Mereka didekripsi saat dibaca; DSN teks biasa yang ditulis dengan tangan masih berfungsi.

**SQLite (bawaan)**

```env
# Bare path (detected as SQLite)
CF_DB_DSN=data/sqlite/app.db

# Or explicit sqlite:// prefix
CF_DB_DSN=sqlite:///opt/corefusion/data/sqlite/app.db
```

**MySQL**

```env
# Format: mysql://user:password@tcp(host:port)/dbname?params
CF_DB_DSN=mysql://corefusion:password@tcp(127.0.0.1:3306)/corefusion?charset=utf8mb4&parseTime=True&loc=Local
```

**PostgreSQL**

```env
# Format: postgres://user:password@host:port/dbname?params
CF_DB_DSN=postgres://corefusion:password@127.0.0.1:5432/corefusion?sslmode=disable
```

#### Kumpulan koneksi (opsional, MySQL/PostgreSQL)

```env
CF_DB_MAX_OPEN_CONNS=100
CF_DB_MAX_IDLE_CONNS=10
```

#### Variabel umum lainnya

| Variabel | Deskripsi |
|----------|-------------|
| `JWT_SECRET` | Menandatangani kunci untuk token login; dihasilkan pada permulaan pertama ketika hilang |
| `CF_SECRET_ENCRYPTION_KEY` | Kunci enkripsi pengaturan sensitif dan kredensial langganan (AES-256-GCM); dibuat otomatis jika tidak ada |
| `CF_INITIAL_ADMIN_PASSWORD` | Kata sandi admin awal (opsional; acak dan dicetak sekali secara default) |
| `CF_NETWORK_TRAFFIC_ACCOUNTING` | Atur ke `0` untuk menonaktifkan penghitungan lalu lintas berbasis iptables (diaktifkan secara default) |
| `CF_NETWORK_TRAFFIC_TABLE` | tabel iptables digunakan untuk penghitungan lalu lintas, default `corefusion` |

> **Kunci enkripsi berada di `.env.secret`**: pada permulaan pertama `CF_SECRET_ENCRYPTION_KEY` ditulis ke `.env.secret` (0600, direktori yang sama dengan `.env`) dan bukan `.env`; kunci lama di `.env` dimigrasikan secara otomatis. Cadangkan file ini secara terpisah — kehilangannya membuat setiap nilai `enc:` tidak dapat didekripsi.
>
> Sejak v0.14.4, `subscriptions.token`, kode pendek dan riwayat rotasi, serta `external_node_sources.source_url` juga memakai kunci ini: token/URL ditampilkan dari ciphertext AES-256-GCM dan pencarian tautan pendek memakai sidik jari HMAC-SHA256. Jika kunci hilang, kredensial ini juga tidak dapat didekripsi atau dicocokkan.

> Proksi terbalik tepercaya untuk menyelesaikan IP klien sebenarnya tidak lagi dikonfigurasi melalui variabel lingkungan;
> mereka disimpan sebagai `panel.trusted_proxies` di **Pengaturan → Jaringan**.

> Baik `.env` dan `.env.secret` mengandung rahasia; panel memperketat izin mereka ke `0600` saat dimulai. Jangan pernah melakukannya.

### 2.10 Docker (opsional, buat gambar Anda sendiri)

Repositori **tidak** mengirimkan Dockerfile. Jika Anda memasukkannya ke dalam container, tulis gambar Anda sendiri. Permintaan referensi:

```bash
docker run -d \
  --name corefusion \
  -p 8080:8080 \
  -p 18443:18443 \
  -v /opt/corefusion/data:/app/data \
  -v /opt/corefusion/.env:/app/.env \
  <your-image>
```

> Kontainer biasanya tidak memiliki systemd dan tidak memiliki hak istimewa `nft`, sehingga fitur pembatasan laju/firewall tidak akan berfungsi.
> Direktori kerja harus sesuai dengan basis untuk `SAFE_PATHS` relatif.

### 2.11 Pemeriksaan pasca pemasangan

```bash
# Environment and dependencies
sudo bash install.sh --cpu-info

# Service state
systemctl is-active corefusion

# Startup log (contains the setup URL and token)
journalctl -u corefusion -n 50 --no-pager

# Listening ports
ss -lntp | grep corefusion
```

---

## 3. Konfigurasi awal

Pada permulaan pertama, jika `.env` tidak memiliki `CF_DB_DSN` yang valid, panel **belum** melayani UI admin. Ini memulai
**penyiapan wizard** pada port acak pada 18000–18999 dengan token satu kali, lalu dimulai ulang sebagai panel normal setelah disimpan.

### 3.1 Buka panduan pengaturan

1. Salin "setup wizard" URL dari keluaran penginstal; atau
2. Baca log di server:

```bash
journalctl -u corefusion -n 50 --no-pager | grep -A2 'setup wizard'
```

```text
CoreFusion first start - setup wizard
  URL   : http://<ip>:<port>/init?init_token=<token>
  Token : <token>
  Valid : 2h0m0s
```

3. Buka URL di browser (sudah berisi `init_token`, jadi tidak perlu entri manual).

> Sesi wizard berakhir setelah **2 jam** secara default. Jika masa berlakunya habis atau panel dimulai ulang, dapatkan yang baru.
> Jangan bagikan tokennya. Setelah pengaturan selesai, port ini ditutup dan hanya port panel normal yang tersisa.

### 3.2 Langkah-langkah penyihir

Wizard memiliki 8 langkah. Anda dapat kembali kapan saja; tidak ada yang dilakukan sampai penyelamatan terakhir.

| Langkah | Konten |
|------|---------|
| 1. Basis Data | SQLite / MySQL / PostgreSQL |
| 2. Direktori data | Ditulis ke `SAFE_PATHS` di `.env` |
| 3. Kunci enkripsi | `JWT_SECRET` dan `CF_SECRET_ENCRYPTION_KEY` |
| 4. Inti | Inti proxy mana yang akan dipasang |
| 5. Proksi | Tidak ada / Nginx / Caddy |
| 6. Situs dan domain | Port, jalur web, akun admin, mata uang, zona waktu, domain |
| 7. Sertifikat HTTPS | Dinonaktifkan / unggah / ACME |
| 8. Ringkasan | Tinjau, unduh `.env`, simpan dan mulai ulang |

#### Langkah 1: Basis Data

- **SQLite (file lokal, disarankan)**: biarkan jalur kosong untuk menggunakan `<data-dir>/sqlite/app.db`.
- **MySQL / PostgreSQL**: host, port, pengguna, kata sandi, nama database; PostgreSQL juga mendukung `disable` / `require` / `verify-full`.
- Klik "Uji koneksi" untuk memvalidasi. Menyimpan juga mengujinya. Basis data yang hilang dibuat secara otomatis (buat secara manual jika akun tidak memiliki `CREATE`).
- DSN (termasuk kata sandi) dienkripsi sebelum ditulis ke `.env`.

#### Langkah 2: Direktori data

Menyimpan database, sertifikat, log, biner inti, dan lainnya; ini `SAFE_PATHS` di `.env`. Defaultnya adalah `data`
(relatif terhadap direktori instalasi); jalur absolut diperbolehkan. **Jangan mengubahnya sembarangan nanti**, atau panel
tidak akan menemukan database dan sertifikat yang ada.

#### Langkah 3: Kunci enkripsi

- `JWT_SECRET`: menandatangani token login.
- `CF_SECRET_ENCRYPTION_KEY`: mengenkripsi pengaturan sensitif (kata sandi database, kredensial DNS sertifikat, kartu isi ulang, dll.) serta token langganan / kode pendek / riwayat rotasi / URL langganan upstream.
- Keduanya dihasilkan secara acak secara default; gunakan "Regenerasi" untuk mendapatkan nilai baru. **Mengubah `CF_SECRET_ENCRYPTION_KEY` membuat data yang dienkripsi sebelumnya tidak dapat dibaca** — buat cadangannya.

#### Langkah 4: Inti

Pilih inti proxy yang akan diinstal:

- **Xray**: VLESS / VMess / Trojan / Shadowsocks, dll.;
- **sing-box**: membawa Hysteria2 / TUIC dan protokol QUIC lainnya.

Setelah menyimpan dan memulai ulang panel, unduh dan instal secara otomatis. Periksa versi dan status di bawah “Protokol”. Anda juga dapat menginstalnya nanti.

#### Langkah 5: Proksi

- **Tidak ada**: panel melayani klien secara langsung (tangani sendiri TLS).
- **Nginx / Caddy**: panel mengelola proxy terbalik dan hosting sertifikat; pilih satu.

Dengan proxy yang dipilih, panel biasanya hanya mendengarkan di `127.0.0.1` dan proxy melayani dunia luar.

#### Langkah 6: Situs dan domain

| Bidang | Deskripsi |
|-------|-------------|
| Dengarkan alamatnya | Berasal secara otomatis: proxy → `127.0.0.1`; tidak ada proxy dengan sertifikat → `0.0.0.0`; jika tidak → `127.0.0.1` |
| Pelabuhan panel | Port bebas acak secara default; ubah jika Anda mau (mis. 8080) |
| Pelabuhan berlangganan | Opsional; `0` berbagi port panel |
| Pelabuhan portal | Opsional; `0` berbagi port panel |
| jalur web | Jalur UI Admin, `/` default; API selalu `/api/v1` |
| Nama pengguna/kata sandi Admin | Acak secara default; **kata sandi hanya ditampilkan sekali dalam ringkasan** — simpan |
| Modus akses | “Domain” atau “IP” |
| Mata uang/zona waktu default | Mata uang default untuk isi ulang kartu dan saldo; zona waktu log panel dan proxy (misalnya `Asia/Shanghai`) |
| Tambahkan ke proksi terbalik | Saat proksi dan domain disetel, tambahkan domain panel/langganan/portal ke proksi sekaligus |

Memasukkan domain akan menulis pengaturan yang sesuai (domain panggilan balik, titik akhir langganan, domain berbagi).

#### Langkah 7: Sertifikat HTTPS

| Sumber | Deskripsi |
|--------|-------------|
| Nonaktifkan HTTPS | Panel/portal/langganan dilayani melalui HTTP (tidak aman); dengan proxy terbalik Anda dapat menambahkan TLS di sisi proxy nanti |
| Unggah sertifikat | Tempelkan PEM sertifikat dan PEM kunci pribadi (hanya mode akses domain) |
| ACME | Menerbitkan sertifikat saat Anda mengirimkan langkah ini; mendukung domain dan sertifikat IP |

Opsi ACME:

- **CA**: Ayo Enkripsi, Ayo Enkripsi (Staging), ZeroSSL, Google Public CA;
- **Target**: domain yang dikonfigurasi, atau IP publik (sertifikat IP hanya didukung oleh Let's Encrypt);
- **Tantangan**: `HTTP-01` (memerlukan port 80) atau `DNS-01` (wildcard, tanpa port 80; pilih penyedia DNS dan masukkan kredensial);
- **Jenis/ukuran kunci**: ECC 256/384 atau RSA 2048/3072/4096;
- **Google Public CA**: memerlukan EAB; masukkan Key ID/HMAC secara manual atau gunakan akun layanan GCP.

> Sertifikat IP secara otomatis mendeteksi IPv4/IPv6 publik host. Di belakang proxy, alamat yang terdeteksi adalah alamat host, bukan alamat keluar proxy.
> Sertifikat domain juga dapat diterbitkan kemudian di bawah “Sertifikat”.

#### Langkah 8: Ringkasan

Menampilkan database, direktori data, inti, proksi, port, akun admin, domain, dan kunci `.env` (rahasia terselubung), dan menyediakan:

- **Unduh .env**: simpan salinan konfigurasi;
- **URL Akses**: URL final untuk panel/langganan/portal; HTTP ditandai sebagai tidak aman.

Klik “Simpan dan mulai ulang” untuk menulis database dan `.env`; panel memulai ulang ke layanan normal.

### 3.3 Setelah pengaturan

- Layanan dimulai ulang dan log permulaan mencetak alamat pendengaran terakhir;
- Masuk dengan akun admin, akses URL dan jalur web dari ringkasan;
- Jika kata sandi hilang, setel ulang secara offline dengan `./corefusion admin reset-password`.

### 3.4 Pengaturan CLI non-interaktif

Untuk server otomatisasi atau tanpa kepala, tulis konfigurasi dan tandai penyiapan selesai secara langsung:

```bash
cd /opt/corefusion
./corefusion init \
  --db-type sqlite \
  --data-dir /opt/corefusion/data \
  --admin-username admin \
  --admin-password 'change-me-strong-password' \
  --listen-port 8080 \
  --proxy-type caddy \
  --cores xray,singbox \
  --panel-domain panel.example.com \
  --tls-mode acme --tls-domain panel.example.com --tls-email you@example.com
```

Bendera umum:

| Bendera | Deskripsi |
|------|-------------|
| `--db-type` / `--db-host` / `--db-port` / `--db-user` / `--db-password` / `--db-name` | Pengaturan basis data |
| `--data-dir` | Direktori data; defaultnya adalah `SAFE_PATHS` atau `./data` |
| `--listen-port` / `--subscription-port` / `--portal-port` / `--web-path` | Port dan jalur web (port bebas acak secara default) |
| `--admin-username` / `--admin-password` | Akun admin (kata sandi acak dicetak satu kali jika dihilangkan) |
| `--proxy-type` | `none` / `nginx` / `caddy` |
| `--cores` | Inti yang akan dipasang, dipisahkan koma: `xray,singbox` |
| `--panel-domain` / `--subscription-domain` / `--portal-domain` | Domain (ditulis ke pengaturan dan ditambahkan ke proxy terbalik) |
| `--tls-mode` | `none` / `self_signed` / `acme` |
| `--tls-domain` / `--tls-email` / `--tls-challenge` / `--tls-key-type` / `--tls-key-bits` | Opsi sertifikat (`--tls-challenge` defaultnya adalah `http-01`; `dns-01` memerlukan `--tls-dns-channel` atau `--tls-dns-provider`) |
| `--tls-ip-cert` | Menerbitkan sertifikat untuk IP publik yang terdeteksi (hanya Mari Enkripsi) |
| `--acme-provider` | `lets_encrypt` / `lets_encrypt_staging` / `zerossl` / `google` |
| `--currency` / `--timezone` | Mata uang/zona waktu default |

Daftar lengkap: `./corefusion init --help`. Kemudian mulai layanan:

```bash
systemctl restart corefusion
```

### 3.5 Menjalankan kembali wizard pengaturan

Untuk memulai kembali (menghapus `.env` dan tanda pengaturan, **menyimpan data**):

```bash
cd /opt/corefusion
./corefusion reset-init      # type RESET to confirm
systemctl restart corefusion
```

Kemudian gunakan URL baru dan token dari log startup.

> Inisialisasi ulang mengatur ulang akun admin dan jalur web. Data bisnis di database lama tetap ada, namun Anda mungkin perlu membuat ulang akun login.
>
> `reset-init` menghapus seluruh `.env` (termasuk `SAFE_PATHS`). **Jika direktori data Anda adalah jalur absolut khusus**, pulihkan `SAFE_PATHS` sebelum memulai ulang, jika tidak, panel akan terlihat di `./data`:
>
> ``` pesta
> printf 'SAFE_PATHS=/srv/corefusion-data\n' > /opt/corefusion/.env
> ```

### 3.6 Keamanan setelah masuk pertama kali

#### Ubah kata sandi awal

1. Klik nama pengguna di pojok kanan atas;
2. Pilih “Ubah kata sandi”;
3. Masukkan kata sandi lama dan baru;
4. Klik "Simpan".

#### Aktifkan login kode dinamis (2FA / TOTP)

> Setelah diaktifkan, akun beralih ke **login kode dinamis**: kata sandi statis tidak lagi diterima; hanya itu
> Kode 6 digit dari aplikasi pengautentikasi Anda (atau kode pemulihan satu kali) berfungsi. Ini **menggantikan** login kata sandi — memang benar
> bukan verifikasi dua langkah “kata sandi + kode”.

1. Klik nama pengguna di pojok kanan atas;
2. Pilih “Aktifkan 2FA”;
3. Masukkan kata sandi saat ini (hanya untuk mengonfirmasi perubahan ini);
4. Pindai kode QR dengan aplikasi autentikator (Google Authenticator / 1Password, dll.), atau masukkan kunci secara manual;
5. Masukkan kode 6 digit untuk menyelesaikan pengaktifan;
6. **Simpan kode pemulihan segera** (dapat diunduh); setiap kode berfungsi satu kali.

Masuk setelahnya:

- Setelah memasukkan nama pengguna, kolom kata sandi menjadi “kode 6 digit / kode pemulihan” dan tombol menjadi “Masuk dengan Kode”;
- Masukkan kode saat ini; **kata sandi statis tidak dapat digunakan lagi**;
- Kode pemulihan hanya sekali pakai; buat ulang di bawah “Login Kode Dinamis (2FA)” ketika habis (ini membuat semua kode lama tidak valid).

Jika autentikator hilang dan kode pemulihan habis, nonaktifkan 2FA offline di server:

```bash
cd /opt/corefusion
./corefusion admin 2fa reset -u admin    # disables 2FA, clears the secret and recovery codes, revokes all sessions
```

> Admin super tunggal yang tidak memiliki autentikator atau kode pemulihan tidak dapat memulihkan melalui UI web dan harus menggunakan perintah di atas.

#### Daftarkan Passkey (opsional)

1. Klik nama pengguna di pojok kanan atas;
2. Pilih “manajemen Passkey”;
3. Klik “Daftar Passkey baru”;
4. Selesaikan perintah perangkat.

> **Catatan**: Kunci sandi memerlukan HTTPS atau localhost, dan domain harus terdaftar dalam daftar yang diizinkan.

---

## 4. Dasbor

Dasbor menampilkan gambaran klaster multi-node dan kesehatan node.

### 4.1 Apa yang ditunjukkannya

- **Info sistem**: nama host, versi OS, waktu aktif, CPU, memori
- **Lalu Lintas**: bandwidth real-time, total lalu lintas, tren bulanan
- **Kesehatan node**: jumlah online / offline / terdegradasi, node dengan beban tinggi
- **Peringatan**: node offline, node terdegradasi, masa berlaku sertifikat akan segera habis, peringatan kuota
- **Sumber daya cluster**: rata-rata CPU, memori, disk, dan throughput

### 4.2 Tindakan

- Klik “Lihat detail” untuk membuka halaman detail node
- Klik kartu node untuk melihat status aktifnya
- Gunakan filter untuk mempersempit berdasarkan status, jenis, atau wilayah

---

## 5. Manajemen simpul

Manajemen node adalah inti dari CoreFusion dan mendukung beberapa jenis node.

### 5.1 Jenis simpul

| Ketik | Deskripsi |
|------|-------------|
| Mandiri (`standalone`) | Node inti independen |
| Cermin (`mirror`) | Menyinkronkan konfigurasi dari node target |
| Utama/cadangan (`primary_backup`) | Node mitra yang melakukan failover satu sama lain |
| Entri rantai (`chain_entry`) | Node entri dari proxy yang dirantai |
| Rantai keluar (`chain_exit`) | Keluar dari simpul proxy yang dirantai |

### 5.2 Menambahkan simpul

1. Buka “Manajemen simpul”;
2. Klik “Tambahkan server”;
3. Isi dasar-dasarnya:
   - Nama, mis. `HongKong-HK01`
   - Nama host: IP atau domain
   - Wilayah, mis. `HK`, `JP`, `US`
   - Jenis simpul;
4. Atur port Agen (default `18443`);
5. Pilih inti yang didukung;
6. Klik "Simpan".

### 5.3 Penerapan sekali klik

1. Setelah menambahkan node, pilih “Deploy command” di daftar node (halaman detail juga menawarkan
 “Perintah pemasangan bootstrap”);
2. Salin perintah — ini berisi token pendaftaran satu kali (berlaku selama 24 jam, jangan digunakan kembali);
3. Jalankan sebagai root pada node jarak jauh; sepertinya:

   ```bash
   bash <(curl -fsSL "http://<panel-host>:<panel-port>/api/v1/agent/install?id=<node-id>&token=<one-time-token>&port=<panel-port>")
   ```

4. Agen mengunduh paket, mendaftar ke panel dan memulai.

### 5.4 Detail simpul

Klik sebuah node untuk membuka halaman detailnya:

- **Ikhtisar**: info dasar, status Agen, status sertifikat; kartu sumber daya menunjukkan penggunaan CPU/memori/disk, laju naik/turun NIC, laju lalu lintas pengguna inti, lalu lintas yang digunakan bulanan, koneksi TCP, beban sistem dan lalu lintas kumulatif NIC keluar (penyegaran senyap setiap 10 detik)
- **Pusat protokol**: konfigurasi proses inti
- **Proksi**: Konfigurasi Caddy / Nginx
- **Proses inti**: mulai/berhenti/mulai ulang
- **Lalu lintas inti**: lalu lintas pengguna yang diteruskan oleh inti dan total bulanannya
- **WARP**: Konfigurasi WARP
- **Domain langsung**: pengelolaan domain
- **Batas tarif**: pembentukan lalu lintas
- **Sertifikat**: pengikatan sertifikat
- **Pengguna daring**: pengguna daring waktu nyata

### 5.5 Tindakan simpul

- **Konfigurasi sinkronisasi**: mendorong konfigurasi panel ke node
- **Paksa menyambung kembali**: putuskan koneksi Agen dan sambungkan kembali
- **Perbarui Agen**: dorong pembaruan Agen
- **Cabut sertifikat**: mencabut sertifikat Agen node

---

### 5.6 Pemeriksaan keamanan node

Detail node → "Pemeriksaan": pengumpulan hanya-baca, tidak pernah mengubah konfigurasi apa pun.

- **Cakupan**: info sistem, SSH (kata sandi / keyboard-interaktif / login root / MaxAuthTries / izin kunci /
  apakah port 22 dibatasi sumbernya), deteksi firewall (firewalld / ufw / nftables / iptables),
  parameter kernel (backlog, buffer, kontrol kemacetan, conntrack, IPv6) dan prioritas antara file konfigurasi.
- **Keparahan**: tinggi / peringatan / info / oke. Hanya item yang tidak terpenuhi yang menghasilkan saran, untuk menghindari kebisingan.
- **Perintah yang disarankan ditulis ke disk**: parameter kernel beralih ke `/etc/sysctl.d/99-corefusion.conf`
  (hapus duplikat per kunci, lalu `sysctl --system`); SSH menjadi `/etc/ssh/sshd_config.d/00-corefusion-*.conf`;
  modul BBR menuju ke `/etc/modules-load.d/`. Semua selamat dari reboot.
- **Prioritas konfigurasi**: Daftar `CFG-01`, per kunci sysctl, file dan baris yang berlaku terakhir —
  `/etc/sysctl.conf` selalu dimuat terakhir dan menggantikan `99-corefusion.conf`; `CFG-02` mencantumkan yang pertama
  cocok dengan arahan sshd — sshd menggunakan nilai pertama yang dilihatnya, drop-in memerlukan awalan `00-`, dan
  `Include /etc/ssh/sshd_config.d/*.conf` harus muncul sebelum arahan tersebut, jika tidak, drop-in akan muncul
  diabaikan seluruhnya (dilaporkan sebagai peringatan).
- **IPv6**: hanya diperiksa ketika node benar-benar memiliki alamat IPv6 global. Alamat terowongan seperti
  `singbr0` / `wg0` / `tun0` diberi label `tun`, ditetapkan oleh proxy dan **tidak boleh diedit dengan tangan**;
  mereka dikecualikan dari IPv6 publik node.
- **Login root**: `PermitRootLogin yes` dinilai oke ketika `PasswordAuthentication` dan
  `KbdInteractiveAuthentication` adalah `no` (hanya kunci); pemeriksaan tidak pernah meminta Anda untuk menonaktifkan root.
- **Dokumen penyetelan lengkap**: dialog dapat menyalin/mengunduh seluruh Penurunan Harga, dan "Lihat online" terbuka
  `docs/node-tuning.<lang>.md` dalam repositori untuk bahasa UI saat ini (16 bahasa).
- **Cache**: hasil di-cache selama 10 menit, `refresh=1` memaksa koleksi baru; pemeriksaan terjadwal adalah
  mati secara default.

## 6. Pusat protokol

Pusat protokol mengonfigurasi parameter protokol untuk proses inti.

### 6.1 Inti yang didukung

| Inti | Deskripsi |
|------|-------------|
| Xray-inti | VLESS / VMess / Trojan / Shadowsocks dan banyak lagi |
| sing-box | Platform proxy universal modular dengan dukungan TUN; membawa Hysteria2 / TUIC dan protokol QUIC lainnya |

### 6.2 Modul konfigurasi

Modul berbeda antara kedua inti; panel menunjukkan set yang cocok dengan inti yang dipilih:

**Xray-inti**

- **Log**: log akses, log kesalahan, mode keluaran
- **API**: alamat mendengarkan gRPC API, daftar layanan
- **DNS**: Server DNS, strategi kueri, host statis
- **Perutean**: aturan perutean, penyeimbangan beban, perutean VLESS
- **Masuk / Keluar**: protokol, transportasi, TLS
- **Transportasi**: WebSocket, gRPC, XHTTP, dll. (TUN adalah protokol masuk, bukan transport)
- **TLS**: sertifikat, ALPN, sidik jari uTLS, ECH
- **Reality**: manajemen target, pasangan kunci, ID pendek
- **Pemeriksaan kesehatan / FakeDNS / Metrik / Kebijakan / Data geografis / Batasan versi**: probe, kumpulan IP, Prometheus, batas waktu dan buffer, file sumber daya, rentang versi inti

**sing-box**

- **Log**: level, stempel waktu, keluaran
- **NTP**: sinkronisasi waktu
- **DNS**: Server dan aturan DNS
- **Perutean**: Keluar default `final`, **Kumpulan Aturan**, **Aturan**, penyelesai domain default, strategi jaringan, dan penggantian (lihat 6.6)
- **Masuk / Keluar**: protokol, transportasi, TLS
- **Titik Akhir**: WireGuard (termasuk konfigurasi WARP), klien MASQUE dan sejenisnya; beberapa dapat hidup berdampingan dan dibedakan oleh `tag`

> Dengan mengaktifkan “gunakan integrasi jaringan sistem”, **nama** titik akhir adalah nama antarmuka kernel:
> maksimal 15 karakter, huruf/angka/`._-` saja. Nama yang terlalu panjang atau tidak valid otomatis dikembalikan
> ke titik akhir `tag`. Titik akhir baru diawali per protokol (`wg-` / `ts-` / `oc-` / `ovpn-c-` /
> `ovpn-s-` / `mq-c-` / `mq-s-`), dan catatan lama seperti `WireGuard-xxxxxx` ditampilkan dinormalisasi
> segera setelah editor dibuka.
- **Layanan**: rangkaian sing-box `services` tingkat atas, seperti `hysteria-realm`, DERP, atau `resolved`
- **Sertifikat / klien HTTP / Namespace jaringan**: Bagian JSON yang Anda perluas hanya bila diperlukan
- **Eksperimental**: `clash_api` dan sejenisnya; **tidak ada yang ditulis ke konfigurasi yang dihasilkan kecuali diaktifkan**

> sing-box 1.12 menghapus bidang perutean `geosite` / `geoip`; Perutean berbasis geografis kini berjalan
> `rule_set`. Mempertahankan bidang lama membuat inti gagal memecahkan kode konfigurasi (`unknown field`).

### 6.3 Konfigurasi masuk

1. Pilih inti dan protokol;
2. Isi dasar-dasarnya:
   - Menandai
   - Dengarkan alamat dan port
   - Jenis protokol
3. Tetapkan parameter protokol (UUID, kata sandi, dll.);
4. Pilih transportasi (TCP, WebSocket, gRPC, dll.);
5. Konfigurasikan TLS (sertifikat, ALPN);
6. Simpan dan terapkan pada inti.

> Masuk baru mendengarkan di `::` secara default (tumpukan ganda di Linux, menangani IPv4 dan IPv6).
> Ubah ke `0.0.0.0` atau alamat tertentu hanya jika Anda perlu membatasi antarmuka.

Mode port “Gunakan port masuk” juga menawarkan **buat aturan izin firewall secara otomatis**: membuka
editor masuk memeriksa firewall node terlebih dahulu, dan ketika firewalld/ufw/nftables/iptables terdeteksi
Anda dapat mengizinkan port untuk disimpan sesuai dengan protokol mendengarkan (TCP/UDP).

- hy2 / hysteria port hopping memungkinkan port mendengarkan dan setiap rentang port dinamis (misalnya `20000-30000`); beberapa rentang dipisahkan dengan koma;
- Tunjangan bersifat idempoten: aturan cakupan yang ada (rentang port/multiport) tidak dibuat lagi;
- Mengubah port akan mencabut aturan lama sebelum mengizinkan aturan baru; menghapus port masuk, menonaktifkan port hopping atau beralih ke Unix Socket / tun (tidak ada port mendengarkan) membersihkannya;
- Ketika tidak ada firewall aktif yang terdeteksi, hanya petunjuk yang ditampilkan dan tidak ada aturan yang ditulis.

**Transportasi dan arus**:

- `xtls-rprx-vision` hanya berfungsi pada TCP mentah: dengan gRPC / XHTTP / WebSocket klien langsung gagal (sing-box: `initialize vision: not a valid supported TLS connection`) atau tidak setuju dengan pemeriksaan aliran server (`flow mismatch`). Panel menghapus visi dari semua non mentah/TCP yang masuk saat penyimpanan dan saat membuat konfigurasi runtime;
- Transportasi sing-box hanya http / ws / quic / grpc / httpupgrade — **TCP mentah berarti tidak ada transportasi sama sekali**. Entri pertama (kosong) dari dropdown transport adalah TCP mentah; menulis `type: "tcp"` membuat inti menolak konfigurasi.

### 6.3.1 Jalur proksi terbalik (Domain akses → TLS Depan → Jalur proksi terbalik)

Ketika jalur transportasi masuknya sendiri adalah `/` tetapi nginx / caddy bagian luar memasangnya di bawah sub-jalur
(misalnya `location /apps { proxy_pass http://127.0.0.1:8080/; }`), klien harus terhubung ke `/apps` atau
permintaan tidak pernah mencapai lokasi tersebut — langganan masih diunduh, tetapi node tidak terhubung.
Isi **Jalur proxy terbalik** di bawah “Akses domain → Front TLS”:

- Masukkan `/apps` atau `/apps?ed=2560` (dengan data awal); berlangganan dan berbagi tautan **ganti** jalur transportasi masuk dengan jalur tersebut;
- Berlaku untuk ws, httpupgrade, xhttp dan h2/http; transportasi tanpa jalur (raw/tcp, kcp) diabaikan;
- Ini tidak tergantung di mana TLS berakhir: jalur masih berlaku ketika TLS Depan mati (ikuti inti);
- gRPC tidak memiliki konsep awalan URL (jalur klien selalu `/{serviceName}/…`), sehingga jalur tersebut **digabung ke dalam ServiceName**
(`/apps` + `svc` → `apps/svc`); proxy terbalik harus menghapus awalan itu di `grpc_pass`.

### 6.3.2 Perutean berbasis SNI (nginx vs Caddy)

Lapisan TLS depan membagi koneksi pada `:443` berdasarkan SNI ClientHello: inbound REALITY di pusat protokol
menurunkan aturan secara otomatis, dan aturan juga bisa ditulis manual di «Manajemen proxy → SNI». Kedua proxy dapat membagi
berdasarkan domain; bedanya ada pada cakupan PROXY protocol.

| | nginx (stream) | Caddy (layer4) |
|---|---|---|
| Cakupan `proxy_protocol` | nginx paket panel (dengan patch `stream_pp_var`): **per aturan SNI**, v1 / v2 / mati. nginx resmi atau tanpa patch: seluruh grup listen (level `listen`), per port semua nyala atau semua mati | Per rute (`proxy ... { proxy_protocol v1|v2 }`) |
| xray + sing-box pada `:443` yang sama | Didukung setelah patch: REALITY xray mendapat v1/v2, aturan sing-box diatur mati | Didukung: satu SNI ke xray (PP v2), lainnya ke sing-box (tanpa PP) |
| Cara menjangkau sing-box | Setelah patch dapat berbagi `:443` dengan xray (PP mati untuk domain itu); jika tidak, port sendiri atau grup listen tanpa PP | Berbagi `:443` dengan xray, dipisah oleh SNI |

Catatan:

- Caddy memilih versi PROXY protocol per situs (rute): v1 / v2 / mati, default tetap v2; jika mati, situs tersebut tidak mengirim header PROXY.
- Default tetap `v1` (perilaku lama `proxy_protocol on`). Setiap situs SNI dapat diubah ke `v1`, `v2`, atau mati. `v2` adalah
  header biner (28 byte untuk IPv4), lebih pendek dari header teks v1 (~50–60 byte); perbedaannya hanya sekali per koneksi dan
  tidak terasa.
- Panel menjalankan `nginx -V` lewat Agent dan mencari `NGX_STREAM_PP_VAR` (flag kemampuan `stream_pp_var`), di-cache 60
  detik. Pada nginx tanpa patch, probe gagal, atau Agent lama, panel kembali ke `proxy_protocol on` seluruh grup (v1) dan
  melewati aturan SNI turunan sing-box (log `skipped N sing-box REALITY SNI rule(s)`), sehingga tidak pernah menghasilkan
  konfigurasi yang gagal `nginx -t`.
- sing-box REALITY / anytls tidak dapat menerima header PROXY, jadi aturan turunannya selalu mati. Memaksa v1/v2 untuk domain
  tersebut akan merusak handshake.
- Di belakang reverse proxy, xray memerlukan PROXY protocol untuk melihat IP dan port klien sebenarnya
  (`sockopt.acceptProxyProtocol`); `realitySettings.xver` adalah yang dikirim xray ke dest dan tidak terkait.
- `singbox_listen_port` pada «Template Reality» adalah **endpoint TLS lokal** (dest xray / handshake sing-box), didukung kedua
  proxy dan terpisah dari routing SNI publik di atas.

### 6.4 Konfigurasi keluar

1. Tambahkan keluar;
2. Tetapkan tag dan protokol;
3. Tetapkan alamat dan port server;
4. Konfigurasikan transportasi dan TLS;
5. Simpan dan terapkan.

### 6.5 Bagikan tautan

1. Klik “Bagikan tautan” di daftar masuk;
2. Pilih format:
   - Bagikan URI
   - Mihomo YAML
   - sing-box JSON
   - Xray JSON
3. Salin tautan atau buat kode QR.

### 6.6 Perutean dan kumpulan aturan sing-box

Di “Pusat Protokol” → sing-box → “Perutean” ada dua bagian: **Kumpulan Aturan** menyatakan dapat digunakan kembali
file aturan, dan **Aturan** menentukan kondisi kecocokan dan keluar. Aturan dievaluasi dari
dari atas ke bawah dan pertandingan pertama menang.

#### 6.6.1 Kumpulan Aturan

| Bidang | Deskripsi |
|-------|-------------|
| menandai | Pengidentifikasi kumpulan aturan yang direferensikan oleh Aturan; harus unik |
| Ketik | `local` membaca file di node; `remote` memungkinkan inti mengunduhnya di `update_interval` |
| Format | `binary` untuk `.srs` (standar); `source` untuk format sumber `.json` |
| Jalan | Jalur `local` relatif terhadap direktori data node; file di `data/rules/` menggunakan `rules/xxx.srs` |
| URL | Unduh alamat untuk `remote`; `download_detour` memilih outbound yang digunakan untuk download (biasanya `direct`) |

Kumpulan aturan lokal (disarankan; unduh file terlebih dahulu di “Sumber daya layanan → File aturan”):

```jsonc
"route": {
  "rule_set": [
    { "type": "local", "tag": "geosite-cn", "format": "binary", "path": "rules/geosite-cn.srs" },
    { "type": "local", "tag": "geoip-cn", "format": "binary", "path": "rules/geoip-cn.srs" }
  ]
}
```

> Mengunduh file di panel saja tidak cukup: kumpulan aturan `local` harus dimasukkan terlebih dahulu ke
> node (“Push to all node”) sehingga file tersebut ada di `data/rules/` node; jika tidak, intinya
> gagal memulai karena file tidak dapat ditemukan.

Kumpulan aturan jarak jauh (node harus dapat mencapai URL):

```jsonc
{ "type": "remote", "tag": "geosite-openai", "format": "binary",
  "url": "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-openai.srs",
  "download_detour": "direct", "update_interval": "1d" }
```

Saat Anda memilih file lokal, tagnya berasal dari nama file (`rules/geosite-cn.srs` →
`geosite-cn`); untuk set jarak jauh berasal dari akhir URL. Formatnya juga diisi dari
ekstensi (`.srs` → `binary`, `.json` → `source`). Nilai turunan dapat diedit secara bebas, dan
nilai yang sudah Anda ketikkan tidak akan pernah ditimpa.

> `download_detour` sudah tidak digunakan lagi sejak sing-box 1.14 dan akan dihapus di 1.16; itu masih
> bekerja di 1.15. Konfigurasi baru harus menggunakan `http_clients` tingkat atas dengan
> `route.default_http_client` sebagai gantinya.

#### 6.6.2 Aturan

| Bidang | Cocok |
|-------|---------|
| `domain` | Domain yang tepat |
| `domain_suffix` | Akhiran domain, termasuk subdomain |
| `domain_keyword` | Pertandingan substring |
| `domain_regex` | Ekspresi reguler |
| `ip_cidr` / `ip_is_private` | IP Tujuan |
| `source_ip_cidr` | Sumber IP |
| `port` / `port_range` | Pelabuhan tujuan |
| `network` | `tcp` / `udp` |
| `rule_set` | Tag Kumpulan Aturan yang akan digunakan |
| `outbound` | Tag keluar yang akan digunakan (harus ada di bawah “Keluar” atau “Titik Akhir”) |
| `type: "logical"` | Kondisi gabungan dengan `mode` diatur ke `and` / `or` |

Beberapa bidang dalam satu aturan digabungkan dengan AND (misalnya `network` + `port`), jadi tidak
Pembungkus `logical` diperlukan. Gunakan `type: "logical"` hanya untuk kombinasi OR / NOT. `rule_set`
harus ditempatkan di dalam sub-aturan `rules`, bukan di sebelah `rules`:

```jsonc
"rules": [
  { "domain_suffix": ["gstatic.com", "googleapis.com"], "outbound": "direct" },
  { "domain_keyword": ["ip.sb", "yandex.com"], "outbound": "warp" },
  { "rule_set": ["geosite-openai", "geosite-google"], "outbound": "warp" },
  { "rule_set": ["geoip-cn"], "outbound": "warp" },
  { "network": "tcp", "port": 443,
    "rule_set": ["geosite-google"], "outbound": "warp" },
  { "type": "logical", "mode": "or",
    "rules": [
      { "rule_set": ["geosite-google"] },
      { "rule_set": ["geosite-openai"] }
    ],
    "outbound": "warp" }
]
```

#### 6.6.3 Memigrasikan aturan xray

| xray | sing-box |
|------|----------|
| String kosong (termasuk nilai kosong di `domain`; pencocokan substring) | `"domain_keyword": ["ip.sb"]` |
| `"domain": ["a.com"]` | `"domain_keyword": ["a.com"]`; gunakan `"domain_suffix": ["a.com"]` untuk pencocokan sufiks sebenarnya atau `"domain": ["a.com"]` untuk pencocokan tepat |
| `"domain:clstorage.net"` | `"domain_suffix": ["clstorage.net"]` |
| `"full:a.com"` | `"domain": ["a.com"]` |
| `"regexp:..."` | `"domain_regex": ["..."]` |
| `"ip": ["geoip:cn"]` | `"rule_set": ["geoip-cn"]` |
| `"domain": ["geosite:cn"]` | `"rule_set": ["geosite-cn"]` |
| `"outboundTag": "direct"` | `"outbound": "direct"` |

Catatan:

- **Tag tidak dibagikan antar inti**: tag keluar xray tidak dapat digunakan dalam aturan sing-box;
 gunakan tag yang benar-benar ada di bawah sing-box “Outbounds” atau “Endpoints”.
- Tag keluar atau titik akhir yang salah melaporkan `outbound not found`; laporan tag kumpulan aturan yang salah
 `rule-set not found`.
- Tempatkan aturan langsung/daftar yang diizinkan terlebih dahulu; setelah suatu aturan cocok, aturan selanjutnya tidak berlaku lagi.

### 6.7 Urutan aturan rute dan sakelar aktif/nonaktif (xray / sing-box)

Aturan rute dicocokkan dari atas ke bawah dan berhenti pada kecocokan pertama, sehingga **urutan adalah prioritas**. Halaman rute kedua core mendukung:

- **↑ / ↓ untuk mengurutkan**: sama seperti halaman Outbound. Xray langsung menyimpan dan menerapkan ke core yang berjalan; pada sing-box klik «Simpan» di kanan atas (tidak ada hot-reload untuk rute; core dimulai ulang bila perlu). Item pertama tidak bisa naik dan item terakhir tidak bisa turun.
- **Sakelar aktif/nonaktif**: sakelar di sisi kiri setiap aturan. Aturan yang dinonaktifkan tetap ada dalam daftar (terlihat setelah muat ulang dan bisa diaktifkan lagi) tetapi tidak ditulis ke konfigurasi runtime. Mengedit aturan yang dinonaktifkan tidak mengaktifkannya kembali secara otomatis.
- **Templat format langganan**: aturan nonaktif difilter dan kolom internal panel tidak pernah dikirim ke klien.
- **Urutan dasar default**: pada node baru dan setelah migrasi sekali jalan untuk node yang ada, direct `gstatic.com` / `googleapis.com` (pemeriksaan konektivitas) berada di urutan pertama, diikuti direct jaringan privat dan pemblokiran iklan. Migrasi hanya mengurutkan ulang aturan yang ada dan tidak menambahkan atau memulihkan aturan yang dihapus.

> Letakkan aturan direct / daftar putih sebelum pemblokiran iklan atau aturan proxy agar tidak tertutup oleh aturan berikutnya.

---

## 7. Manajemen proksi

Manajemen proksi mengonfigurasi proksi terbalik Caddy atau Nginx.

### 7.1 Jenis proksi

- **Caddy**: HTTPS otomatis, HTTP/1.1 dan HTTP/2 (HTTP/3 / QUIC dinonaktifkan untuk menghindari konflik UDP 443 dengan perutean layer4)
- **Nginx**: performa tinggi, konfigurasi fleksibel

### 7.2 Konfigurasi situs

1. Buka “Manajemen proxy”;
2. Pilih jenis proksi (Caddy / Nginx);
3. Tambahkan situs:
   - Domain
   - Hulu
   - Pelabuhan lokal
4. Simpan.

### 7.3 Bidang

- **upstream**: alamat backend, mis. `127.0.0.1:8082`
- **local_port**: port pendengaran lokal, mis. `8443`
- **portal_forward**: aturan penerusan portal
- **subscription_forward**: aturan penerusan langganan
- **Perutean SNI**: ketika masuk menggunakan TLS / REALITY dan klien selalu terhubung pada 443, panel akan diturunkan
 443 aturan perutean TCP dari ClientHello SNI (baik xray `streamSettings` dan sing-box `tls.reality`);
 proxy depan hanya meneruskan TCP dan TLS masih diakhiri di dalam inti

---

## 8. Merutekan sumber daya

Sumber daya perutean mengumpulkan aset bersama yang digunakan oleh langganan dan perutean. Halaman ini memiliki lima tab:

| tab | Tujuan |
|-----|---------|
| Domain pilihan langsung & CF | Domain/IP langsung masuk dan domain pilihan Cloudflare, digunakan saat menulis ulang alamat akses di langganan dan node eksternal |
| kumpulan CF IP | Sumber IP pilihan, port dan pengaturan probe, digunakan oleh “Domain akses” |
| Aturan global WAF | Sumber aturan dan file aturan untuk proxy terbalik WAF, dikirim ke node |
| File aturan | Unduh, sinkronisasi terjadwal, dan dorongan file aturan (`.srs` / `.json` / `.dat`) |
| Node eksternal | Impor node eksternal dari v2ray URI / Base64 / berlangganan URL (lihat 16.3) |

### 8.1 Domain pilihan langsung & CF

- **Akses langsung**: domain atau IP masuk, port 443 secara default; ikat node atau biarkan kosong untuk global;
- **Domain pilihan CF**: Domain pilihan Cloudflare plus port, dengan pemeriksaan konektivitas;
- Keduanya memberi makan kandidat “Domain akses” yang digunakan oleh langganan dan penulisan ulang node eksternal (lihat 16.3).

- **Entri drift (utama/siaga)**: entri langsung dapat mengikat saluran DNS dan nama rekaman; setelah failover atau switch-back, panel menunjuk rekaman A pada IP publik primer saat ini (IP internal bila kosong). Hanya penyedia dengan penulis DNS yang diterapkan yang diterima (`cloudflare` terverifikasi; alidns / DNSPod / GoDaddy / Namecheap / Route53 / Google / Huawei Cloud diterapkan tetapi tidak diverifikasi). Propagasi Cloudflare diukur sekitar 2–2,5 menit dengan TTL ditetapkan pada 60; matikan awan oranye untuk entri TCP/REALITY.

### 8.2 kumpulan IP CF

- Pertahankan sumber IP pilihan (termasuk sumber gaya `bestproxy`), interval pengambilan dan port;
- “Pengaturan probe” mengontrol interval probe, ambang latensi, node masuk dan node eksekusi (host panel atau
  Agent), ditambah NIC jalan keluar opsional; memilih node entri akan melakukan panggilan melalui saluran
  tersebut, sedangkan tanpa entri, hanya panggilan langsung lokal yang dapat dilakukan (ini hanya memfilter IP
  yang jelas-jelas tidak dapat dijangkau);
- Inti dialer harus dipasang di **node eksekusi yang dipilih**: halaman mencantumkan inti dan versi yang
  dilaporkan oleh node tersebut, dan Instal / Instal Ulang menginstalnya langsung di sana (unduh node jarak
  jauh melalui Agent, tidak diperlukan SSH);
- “Dialer” mungkin otomatis (inti terpasang), `singbox`, `xray` atau `mihomo`; core yang tidak diinstal tidak
  terdaftar. Probing berjalan sebagai proses sementara yang keluar setelah selesai dan tidak pernah membaca
  atau menulis konfigurasi inti itu sendiri;
- “Periksa konektivitas” hanya mencakup halaman yang Anda buka: halaman kumpulan IP CF memperbarui IP pilihan,
  halaman domain bersama memperbarui domain CF, dan tidak ada yang bersentuhan satu sama lain; tanpa
  sejenisnya ia masih memeriksa keduanya (kompatibel ke belakang);
- "Pemeriksaan proksi rantai" adalah kartu terpisah: kartu ini melakukan panggilan dari pintu keluar yang
  dipilih melalui domain/IP CDN kembali ke pemeriksaan masuk dan mengeluarkan permintaan nyata (diperlukan
  HTTP 200 dengan lalu lintas di kedua arah). Ini saling eksklusif dengan probe IP pilihan **pada jadwal**—
  bila diaktifkan, pemeriksaan IP pilihan terjadwal akan dijeda dan hanya pemeriksaan proksi rantai yang
  berjalan (manual “Periksa konektivitas” masih berfungsi); itu dapat mewarisi atau mengganti interval probe
  dan dialer, dan juga mendukung node eksekusi dan NIC jalan keluar;
- Ketika “Access domain” menggunakan kumpulan CF IP tanpa nilai konkret, satu entri (dengan portnya) dipilih
  secara acak.

### 8.3 Aturan global WAF

- Konfigurasikan sumber aturan WAF URL dan file aturan yang akan disinkronkan;
- Setelah disinkronkan, mereka didorong ke node dan dimuat oleh proxy node (Nginx / Caddy);
- File dengan URL lengkap diunduh dari URL itu; jika tidak, "sumber URL + / nama file" digunakan;
- "Match cache" (`match_cache` / `config_match_cache`) diaktifkan secara default: batasan seluruh proses
 cache hasil pencocokan aturan panas mengurangi traversal aturan per permintaan. Ini hanya bersifat global tanpa no
 penggantian per domain; nonaktifkan sementara untuk perbandingan A/B saat menyelidiki positif palsu
 (menonaktifkan berarti evaluasi aturan penuh pada setiap permintaan dan penggunaan CPU yang lebih tinggi).

### 8.4 File aturan (file aturan perutean)

Tab ini hanya mengunduh file aturan di panel dan mengirimkannya ke node; tidak ada yang berpengaruh
secara otomatis. Untuk menggunakannya dalam perutean, referensikan pada “Pusat protokol → sing-box → Perutean”
dalam Kumpulan Aturan dan Aturan (lihat 6.6).

- **Repositori aturan**: masukkan `owner/repo` (GitHub, dapat dijelajahi) atau unduh lengkap URL; dijadwalkan
 sinkronisasi dapat berjalan setiap 6/12 jam, setiap hari pada pukul 03:00, setiap hari pada tengah malam, atau setiap minggu pada hari Minggu;
- **Tambahkan file**: untuk satu file di luar rilis repositori (misalnya cabang SagerNet `.srs`
 file); masukkan URL lengkap dan diunduh secara otomatis;
- **Daftar file**: menampilkan nama, ukuran, pembaruan terakhir, dan tombol “Sinkronisasi otomatis”, dengan “Unduh” per file;
 tombol “Push to all node” di bagian atas mendorong setiap file aturan ke semua node;
- File diunduh ke panel `data/rules/` dan, setelah didorong, langsung berada di bawah data node
 `rules/` direktori; referensikan mereka di sing-box sebagai `rules/<file name>`;
- Format:
  - `.srs`: Kumpulan aturan biner sing-box, direkomendasikan dan terkecil;
  - `.json`: kumpulan aturan format sumber; `rule_set` harus menyetel `format: "source"`;
  - `.dat`: xray `geosite.dat` / `geoip.dat`, yang tidak dapat digunakan secara langsung oleh sing-box.

> Blok "Penggunaan" yang dapat dilipat di bagian atas halaman berisi contoh referensi, pemetaan xray
> tabel dan kesalahan umum.

### 8.5 Node eksternal

Mengimpor dan mengelola node eksternal tercakup dalam [16.3 Node eksternal](#163-external-nodes).

---

## 9. Pusat berlangganan

Pusat langganan mengemas node ke dalam langganan yang siap klien: tentukan **format** langganan (apa yang akan dipancarkan, aturan mana, paket mana yang dapat melihatnya), lalu **titik akhir** langganan (domain/jalur publik), lalu bagikan tautannya.

### 9.1 Format berlangganan

“Pusat langganan” → “Format langganan”; satu baris per keluaran klien:

| Bidang | Deskripsi |
|---|---|
| Nama | Unik; `?config=<name>` cocok dengannya dan dialog pelanggan menunjukkannya |
| Tipe klien | clash / shadowrocket / singbox / v2ray — hanya menampilkan pengelompokan |
| Format keluaran | `mihomo_yaml` (Clash/Mihomo), `singbox_json`, `xray_json`, `base64_uri`, `raw_uri` |
| Tipe Konten / ekstensi | Header respons dan nama file |
| Peron | menangkan / mac / android / ios / universal; cocok dengan `?platform=`, kosong = universal |
| Rencana terikat | Kosong = tersedia untuk setiap paket; jika tidak, hanya paket tersebut yang dapat menggunakannya |
| Aturan | Aturan perutean per format (editor terstruktur untuk Mihomo, JSON mentah per aturan untuk sing-box / Xray). **Aturan tidak dikonversi antar format** |

Catatan:

- Beberapa konfigurasi mungkin ada untuk format keluaran yang sama (misalnya `out`, `home`); pilih satu dengan `config=` di tautan;
- Tanpa `config`, urutannya adalah “konfigurasi terikat ke langganan → `platform` cocok → default”;
- Templat sing-box dapat membawa `route.rule_set` sendiri untuk klien, biasanya sebagai `remote` sehingga
 klien mengunduhnya; file lokal yang dikirim ke `rules/` node di bawah "File aturan" hanya tersedia
 ke inti pada node tersebut, bukan ke langganan klien;
- Saat membuat templat sing-box / Xray, panel menghapus referensi yang menggantung (`final`, `dns.final`,
 `default_domain_resolver`, `download_detour`, `rule_set` lokal, definisi `rule_set` tidak valid, kosong
 `balancer`, dll.) sehingga klien tidak gagal saat startup dengan kesalahan FATAL;
- Alias keluar dalam aturan diselesaikan dengan tag asli (`REJECT` / `blocked` → `block`, `DIRECT` →
 `direct`, tidak peka huruf besar-kecil); keluar yang tidak dapat diselesaikan kembali ke keluar default dan aturannya
 mempertahankan posisinya;
- Grup dengan `tag = proxy` selalu dibuat: `selector` untuk sing-box dan penyeimbang `random` untuk
 Xray, dan `outbound: proxy` dalam aturan ditulis ulang menjadi `balancerTag` yang cocok. Editor aturan dimuat
 kandidat keluar secara dinamis dari “grup proxy + keluar khusus”, dan tag yang dimasukkan secara manual
 masih tervalidasi;
- Konfigurasi sing-box / Xray harus dibuat secara terpisah; jika tidak ada maka tidak muncul di tautan pelanggan (lihat 10.3).

- Editor aturan terstruktur Mihomo mendukung aturan logis `AND` / `OR` / `NOT` / `SUB-RULE`: tulis kondisi sebagai `((NETWORK,UDP),(DST-PORT,3478/19302))`; untuk `SUB-RULE` kolom ketiga adalah nama grup sub-aturan. Ketentuan harus diapit tanda kurung;
- Jenis aturan diselaraskan dengan mihomo terbaru: `IP-SUFFIX`, `SRC-IP-CIDR/SRC-IP-SUFFIX/SRC-IP-ASN/SRC-GEOIP`, `IN-PORT/IN-TYPE/IN-USER/IN-NAME/REMATCH-NAME`, `PROCESS-NAME-WILDCARD/REGEX`, `PROCESS-PATH-WILDCARD/REGEX`, `UID`, `DSCP` semuanya tersedia;
- `SUB-RULE` mereferensikan blok `sub-rules:` dari konfigurasi mihomo; panel belum mengedit blok itu, jadi tentukan terlebih dahulu melalui impor YAML atau config_json yang dibagikan.

### 9.2 Titik akhir langganan

“Pusat langganan” → “Titik akhir langganan”; satu baris per alamat publik:

| Bidang | Deskripsi |
|---|---|
| Domain | Domain langganan publik |
| Jalan | Awalan di bawah domain, default `/` |
| Diaktifkan | Titik akhir yang dinonaktifkan diabaikan saat membuat tautan |
| Deskripsi | Catatan teks bebas |

- **Beberapa baris (multi-domain)** didukung; ketika beberapa diaktifkan, domain penyedia mengikuti domain yang digunakan oleh permintaan saat ini (lihat 9.5);
- Port berlangganan khusus berfungsi di root (`/<token>`, tanpa `/sub`); saat berbagi port panel adalah `/sub/<token>`;
- Proksi terbalik harus meneruskan `X-Forwarded-Proto` / `X-Forwarded-Host`, jika tidak, titik akhir pertama yang diaktifkan akan digunakan. Templat situs Caddy / Nginx yang dihasilkan panel mencakup kedua header; konfigurasi proxy yang dikelola sendiri perlu ditambahkan secara manual.

### 9.3 Tautan berlangganan

Bentuk dasar: `https://<sub-domain>/<token>` (port panel: `https://<panel-domain>/sub/<token>`)

| Parameter | Deskripsi |
|---|---|
| `format` | Format keluaran; alias: `clash` → mihomo_yaml, `singbox`/`sing-box` → singbox_json, `xray` → xray_json, `raw` → raw_uri, `base64`/`v2ray` → base64_uri |
| `config` | Terapkan format berlangganan berdasarkan nama; harus cocok dengan `format`, jika tidak maka akan diabaikan |
| `platform` | Cocokkan konfigurasi berdasarkan platform (bila `config` dihilangkan) |
| `providers` | `1` / `true`: Mode Penyedia Clash (`proxy-providers` + `use`); node sebaris default |

Contoh:

```bash
# Clash dynamic subscription (config template + providers)
https://sub.example.com/<token>?format=clash&config=out&providers=1

# sing-box / Xray (only meaningful when such a config exists)
https://sub.example.com/<token>?format=singbox_json&config=sing-box-home
https://sub.example.com/<token>?format=xray_json&config=xray-home

# generic clients / plain text
https://sub.example.com/<token>?format=base64_uri
https://sub.example.com/<token>?format=raw_uri
```

### 9.4 Tautan pendek langganan (opsional)

Tombol "Tautan pendek" di setiap baris langganan membuat kode acak 20 karakter:

```text
https://<sub-domain>/<20-char code>     # dedicated subscription port
https://<panel-domain>/<code>           # shared panel port (prefix follows the endpoint path)
```

- Tautan pendek mengembalikan isi langganan secara langsung (tanpa 302) dan memakai otorisasi yang sama dengan tautan panjang: langganan dicabut/ditangguhkan/kedaluwarsa, klien diblokir, rotasi token, tautan dinonaktifkan/kedaluwarsa semuanya langsung gagal.
- Tautan sudah mengunci format / konfigurasi / platform / providers, jadi klien tidak perlu menambahkan `?format=&config=`; saat membuat, matikan "kunci parameter" jika ingin mengizinkan penimpaan `?format=`.
- Portal dan detail klien mengutamakan tautan pendek. Sakelar global ada di Pengaturan → Panel → Tautan pendek langganan; jika dimatikan semua tautan pendek mengembalikan 404 dan tautan panjang tetap berfungsi. Log akses mencatat `access_type=short_link` dan `short_link_id`.
- Tautan di portal/detail klien dibuat otomatis untuk setiap kombinasi "format × konfigurasi (× providers)" yang terlihat, sehingga tidak pernah muncul "tautan pendek + parameter".
- Tautan pendek `mihomo_yaml` wajib terhubung ke konfigurasi: seperti sing-box / Xray, tautan hanya dibuat dari konfigurasi nyata di "Format langganan"; tautan Mihomo tanpa konfigurasi tidak lagi dibuat, membuka dialog "Tautan pendek" membersihkan tautan lama tanpa konfigurasi, dan pembuatan/pembaruan manual tanpa konfigurasi ditolak.
- Kode pendek tidak pernah disimpan dalam teks biasa: database hanya menyimpan ciphertext AES-256-GCM acak (untuk menampilkan URL lengkap) dan sidik jari HMAC-SHA256 (untuk pencarian kesamaan); kode yang dirotasi/dihapus ditulis ke tabel pencabutan dan tidak dapat diselesaikan lagi.

### 9.5 Mode multi-domain dan Penyedia

Dengan `providers=1`, setiap titik masuk `proxy-providers` kembali ke titik akhir langganan. Pemilihan domain:

1. **Domain yang digunakan permintaan ini** (nilai pertama `X-Forwarded-Host`, port dihapus, selain itu `Host`) jika cocok dengan titik akhir yang **diaktifkan**;
2. Jika tidak, titik akhir yang diaktifkan dengan **ID terendah**;
3. Jalurnya berasal dari titik akhir itu: `https://<domain><path>/<token>?format=mihomo_proxies_yaml...`.

Jadi, mengubah domain tidak memerlukan perubahan panel: klien yang mengambil melalui domain baru akan mendapatkan domain baru di dalam penyedia, dan domain lama yang dibiarkan aktif tidak lagi di-hardcode. Jika klien menyimpan cache penyedia lama URL, perbarui langganan (atau hapus cache penyedia/mulai ulang klien).

Catatan tambahan:

- `proxy-providers` di dalam format bersama (`config_json`) diabaikan (menghindari kebocoran lintas paket); penyedia berasal dari kelompok simpul rencana dan sumber eksternal;
- Jalur cache penyedia internal dan eksternal selalu berasal dari “default penyedia internal → templat jalur cache” (default `./providers/{key}.yaml`); sumber eksternal tidak dapat menentukan sendiri;
- Penyedia cadangan lama `subscription-nodes` tidak lagi dikeluarkan: grup yang menggunakan "semua node" diperluas ke penyedia konkret, atau node inline ketika paket tidak memilikinya.

---

## 10. Manajemen pelanggan

Manajemen pelanggan menangani pengguna akhir.

### 10.1 Menambahkan pelanggan

1. Buka “Manajemen Pelanggan”;
2. Klik “Tambahkan pelanggan”;
3. Isi:
   - Surel
   - Kata sandi
   - Rencana
   - Kuota lalu lintas
   - Waktu kadaluwarsa
4. Menyimpan.

### 10.2 Tindakan Pelanggan

- **Edit**: ubah detail pelanggan
- **Tangguhkan**: menangguhkan layanan
- **Lanjutkan**: memulihkan pelanggan yang ditangguhkan
- **Setel ulang lalu lintas**: menyetel ulang lalu lintas yang digunakan
- **Reset kata sandi**: mereset kata sandi pelanggan
- **Renew**: memperpanjang tanggal kadaluwarsa
- **Tingkatkan / turunkan versi**: ubah paket
- **Hapus**: menghapus pelanggan

### 10.3 Detail langganan

Manajemen pelanggan → kolom “Langganan” → “Detail”. Dialog mencantumkan tautan untuk setiap format yang tersedia untuk paket pelanggan:

- **Clash / Mihomo**: dua baris — node sebaris dan `providers=1` (penyedia proxy), keduanya membawa `config=<name>`;
- **Sing-box / Xray**: hanya ditampilkan ketika konfigurasi format tersebut ada dan terlihat pada rencana; tautan membawa `config=<name>`;
- **Base64 URI / Raw URI**: selalu ditampilkan (tidak ada `config`);
- Setiap baris memiliki tombol salinnya sendiri; mengklik satu baris akan mengalihkan kode QR di bawah ke format tersebut.

Jadi hanya dengan konfigurasi Clash, dialog menampilkan Clash + Base64 + Raw; Tautan sing-box / Xray hanya muncul setelah konfigurasinya dibuat.

Dialog yang sama juga menunjukkan penggunaan lalu lintas, riwayat pesanan, dan riwayat transaksi.

---

## 11. Manajemen rencana

Manajemen paket mengonfigurasi paket yang Anda jual.

### 11.1 Templat rencana

1. Buka “Manajemen rencana” → “Template rencana”;
2. Tambahkan templat:
   - Nama templat
   - Kategori (standar / premium / perusahaan / uji coba)
   - Perpanjangan otomatis
   - Pengaturan percobaan
3. Simpan.

### 11.2 Varian paket

1. Tambahkan varian di bawah template:
   - Nama varian (misalnya bulanan, tahunan)
   - Harga (dalam sen)
   - Durasi (hari)
   - Lalu lintas (GB, `0` = tidak terbatas)
   - Batas konkurensi
   - Batas kecepatan (Mbps)
2. Simpan.

### 11.3 Pengaya lalu lintas

1. Buka “Pengaya lalu lintas”;
2. Tambahkan tambahan:
   - Nama
   - Harga
   - Lalu lintas
   - Mode kedaluwarsa (perpanjang hari / sama seperti paket)
3. Simpan.

### 11.4 Kupon

1. Buka “Kupon”;
2. Tambahkan kupon:
   - Kode
   - Jenis (persentase/jumlah tetap)
   - Nilai diskon
   - Batas penggunaan
   - Masa berlaku
   - Rencana yang berlaku
3. Simpan.

---

## 12. Manajemen isi ulang

Manajemen isi ulang menangani situs isi ulang, pesanan, dan kartu isi ulang.

> **Integrasi pembayaran**
>
> Panel **tidak terintegrasi langsung dengan lembaga pembayaran** (Alipay, WeChat Pay, Stripe, bank, dll.). Ini hanya terhubung ke **platform pembayaran perantara**, yang pada gilirannya terintegrasi dengan lembaga pembayaran terkait, menangani kontrak, perolehan dan penyelesaian, dan melaporkan kembali hasil pembayaran melalui panggilan balik [charge API](api/charge-callback.md).
>
> Platform pembayaran perantara **bukan bagian dari proyek ini; operator harus mengembangkannya sendiri (atau menggunakan sistem mereka sendiri)**. Setelah diterapkan, daftarkan di bawah “Situs isi ulang” dan selesaikan verifikasi domain untuk menggunakannya sebagai titik masuk isi ulang; situs isi ulang di sini adalah platform perantara itu, bukan lembaga pembayaran itu sendiri. Kualifikasi pembayaran, biaya, penyelesaian dan kepatuhan disepakati dan ditanggung oleh operator, lembaga pembayaran, dan platform perantara.

### 12.1 Isi ulang situs

1. Buka “Manajemen isi ulang” → “Situs isi ulang”;
2. Tambahkan situs:
   - Nama situs
   - Metode pembayaran yang didukung oleh platform perantara tersebut (Alipay, WeChat, USDT, Stripe, dll., disediakan melalui integrasinya dengan lembaga pembayaran)
   - Mata uang
   - Nilai tukar
3. Simpan;
4. Verifikasi situs (file HTTP atau verifikasi DNS).

### 12.2 Isi ulang pesanan

- Lihat semua pesanan isi ulang
- Filter berdasarkan status (menunggu keputusan, dibayar, gagal, kedaluwarsa, dikembalikan)
- Lihat detail pesanan
- Mengeluarkan pengembalian dana admin

### 12.3 Isi ulang kartu

1. Buka “Isi ulang kartu”;
2. Tambahkan kartu:
   - Jenis (jumlah, paket, tambahan lalu lintas)
   - Nilai nominal / rencana
   - Waktu kadaluwarsa
3. Atau hasilkan secara massal:
   - Kuantitas (hingga 1000)
   - Awalan nomor kartu
   - Format nomor kartu
4. Ekspor sebagai CSV.

### 12.4 Manajemen keseimbangan

- Lihat saldo pelanggan
- Menyesuaikan saldo (menambah/menurunkan/menetapkan)
- Sesuaikan jumlah beku
- Lihat riwayat transaksi

### 12.5 Pengecer

- Tambahkan pengecer
- Tetapkan tarif komisi
- Lihat akumulasi komisi
- Tarik komisi

---

## 13. Manajemen tiket

Manajemen tiket menangani permintaan dukungan pelanggan.

### 13.1 Daftar tiket

- Lihat semua tiket
- Filter berdasarkan status (buka, dibalas, ditutup)
- Filter berdasarkan kategori (teknis, penagihan, akun, lainnya)
- Filter berdasarkan prioritas (rendah, normal, tinggi, mendesak)

### 13.2 Tindakan tiket

- **Lihat / balas**: buka tiket dan balas
- **Tetapkan**: menetapkan tiket ke administrator
- **Tutup**: menutup tiket yang terselesaikan

---

## 14. Manajemen sertifikat

Manajemen sertifikat menangani sertifikat TLS.

### 14.1 Penerbitan sertifikat

1. Buka “Manajemen Sertifikat”;
2. Klik “Terbitkan sertifikat”;
3. Isi:
   - Daftar domain (satu per baris)
   - Jenis sertifikat (domain / IP)
   - Jenis kunci (ECC / RSA)
   - Metode validasi (DNS-01 / HTTP-01)
   - Penyedia DNS
   - Lokasi penerbitan (panel host/node)
4. Klik "Masalah".

### 14.2 Tindakan sertifikat

- **Memperbarui**: memperbarui secara manual
- **Deploy**: menyebarkan ke sebuah node
- **Distribusi**: dorong ke semua node terkait
- **Migrasi**: memindahkan referensi dari sertifikat lama ke sertifikat baru
- **Unduh**: unduh `cert` / `fullchain` / `privkey`

### 14.3 Mengunggah sertifikat

1. Klik “Unggah sertifikat”;
2. Pilih metode:
   - Tempelkan kontennya
   - Unggah file
3. Unggah rantai sertifikat dan kunci pribadi;
4. Menyimpan.

### 14.4 konfigurasi EAB

- Kelola kredensial EAB untuk Google Public CA / ZeroSSL
- Dapatkan EAB secara otomatis
- Lampirkan secara otomatis saat diterbitkan

---

## 15. Kuota lalu lintas

Kuota lalu lintas memantau lalu lintas bulanan setiap node.

### 15.1 Daftar kuota

- Lihat pengaturan kuota untuk semua node
- Tampilkan ukuran kuota, lalu lintas yang digunakan, dan sisa kuota
- Tampilkan persentase penggunaan

### 15.2 Penggunaan siklus

- Lihat detail penggunaan siklus per node
- Tampilkan lalu lintas masuk / keluar
- Tampilkan waktu mulai/berakhir siklus

### 15.3 Setel ulang log

- Lihat catatan pengaturan ulang lalu lintas
- Tampilkan siklus reset, sumber pemicu dan operator

---

## 16. Proxy yang dirantai

Proksi berantai mengonfigurasi tautan multi-keluar.

### 16.1 Membuat tautan

1. Buka “Proksi berantai”;
2. Klik “Tautan baru”;
3. Pilih simpul entri;
4. Pilih entri masuk;
5. Pilih node keluar (beberapa diperbolehkan);
6. Pilih jalur masuk untuk setiap pintu keluar;
7. Tetapkan prioritas/bobot;
8. Mengonfirmasi.

### 16.2 Manajemen tautan

- Lihat ikhtisar topologi
- Edit tautan (prioritas, bobot, tag keluar)
- Hapus tautan
- Periksa status kesehatan

### 16.3 Node eksternal

- Impor node eksternal (v2ray URI, Base64 URI, langganan URL)
- Daftarkan entri eksternal
- Kelola node eksternal (edit / hapus satu per satu)
- Akses domain: konfigurasikan akselerasi CDN / IP pilihan per node (domain langsung, CF lebih disukai
 domain, kumpulan CF IP, khusus). Hanya server/port yang dikirimkan dalam langganan yang ditulis ulang;
 SNI/Host tetap seperti yang diimpor. Jika dinonaktifkan, alamat/port yang diimpor akan digunakan
- Ketika "domain pilihan CF / kumpulan IP CF" dipilih tanpa nilai tertentu, satu entri akan dipilih
 secara acak dari pool (dengan portnya); "langsung / khusus" memerlukan nilai eksplisit, jika tidak
 itu jatuh kembali ke alamat node itu sendiri

---

## 17. Pengguna daring

Pengguna online menunjukkan status online waktu nyata.

### 17.1 Apa yang ditunjukkannya

- Langganan daring
- Perangkat / IP online
- Node, IP dan waktu koneksi untuk setiap langganan
- Tingkat uplink / downlink waktu nyata
- Total uplink / total downlink / total lalu lintas

> Total uplink, total downlink, dan total lalu lintas terakumulasi selama **sesi online berkelanjutan**:
> penyambungan kembali yang berumur pendek, pengaktifan ulang inti, dan penyetelan ulang penghitung tidak menghapusnya. Hanya setelah 60
> detik tanpa laporan baru, sesi baru dimulai dari nol. Ini adalah ukuran yang berbeda
> dari penghitung “lalu lintas sejak awal ini” milik inti dan lebih mencerminkan apa yang sebenarnya dilakukan pengguna
> digunakan selama sesi saat ini.

### 17.2 Penyaringan

- Filter berdasarkan simpul
- Cari berdasarkan email / pelanggan / node / IP

---

## 18. Manajemen pengguna

Manajemen pengguna menangani pengguna panel.

### 18.1 Peran

| Peran | Deskripsi |
|------|-------------|
| Admin super (`superAdmin`) | Hak istimewa tertinggi |
| Admin (`admin`) | Hak istimewa manajemen |
| Operator (`operator`) | Hak istimewa operasional |
| Penampil (`viewer`) | Hanya baca |

### 18.2 Menambahkan pengguna

1. Buka “Manajemen pengguna”;
2. Klik “Tambahkan pengguna”;
3. Isi:
   - Nama pengguna
   - Surel
   - Kata sandi
   - Peran
4. Simpan.

### 18.3 Tindakan pengguna

- **Edit**: ubah detail pengguna
- **Hapus**: menghapus pengguna

---

## 19. Situs portal

Situs portal mengkonfigurasi portal klien.

### 19.1 Umum

- Nama situs
- Deskripsi situs
- Buka pendaftaran
- Jalur portal
- Pelabuhan portal

### 19.2 Penampilan

- Judul situs
- Logo URL
- Warna primer, latar belakang, teks dan aksen
- huruf

### 19.3 Navigasi

- Tambahkan halaman navigasi
- Konfigurasikan nama halaman, ikon, dan tautan eksternal
- Kelola item navigasi

### 19.4 Modul

- Tambahkan / edit modul
- Konfigurasikan judul modul, deskripsi dan ikon
- Mendukung penurunan harga, teks kaya, dan HTML khusus

### 19.5 Bahasa

- Tambahkan bahasa
- Kelola salinan
- Impor / ekspor JSON

### 19.6 Templat

- Ganti templat
- Sesuaikan HTML/CSS/JS
- Setel ulang ke templat default

---

## 20. Pengaturan sistem

Pengaturan sistem mengonfigurasi parameter panel.

### 20.1 Panel

- Dengarkan alamat dan port
- Direktori data
- Zona waktu
- Pelabuhan berlangganan
- Batas ekspor
- Buka pendaftaran
- Mode panel (penuh/hanya manajemen)

### 20.2 Keamanan

- Manajemen kunci enkripsi
- Manajemen kunci JWT
- Daftar hitam IP
- Passkey domain yang diizinkan

### 20.3 OAuth

- GitHub OAuth
- Google OAuth
- Konfigurasi panggilan balik URL

### 20.4 Pemberitahuan

- Notifikasi email (SMTP)
- Notifikasi Telegram
- Templat pemberitahuan
- Pengaturan pemantauan

### 20.5 Pencadangan dan pemulihan

- Buat cadangan
- Daftar cadangan
- Unduh cadangan
- Pulihkan cadangan
- Pencadangan terjadwal
- Penyimpanan jarak jauh (FTP / S3 / WebDAV)

### 20.6 Buka blokir otomatis

- Aktifkan buka blokir otomatis
- Pendinginan
- Status yang berlaku

---

## 21. Konsol operasi

Konsol operasi mengelola snapshot konfigurasi dan menerapkan pekerjaan.

### 21.1 Cuplikan konfigurasi

- Buat cuplikan
- Daftar cuplikan
- Kembalikan ke snapshot
- Hapus cuplikan

### 21.2 Melamar pekerjaan

- Lihat pekerjaan terkini
- Terapkan konfigurasi sekarang (DB → runtime)
- Pra-pemeriksaan (validasi konflik port dan konfigurasi JSON)
- Batalkan pekerjaan
- Coba lagi pekerjaan

### 21.3 Validasi simpul

- Pilih node dan validasi
- Lihat hasil validasi
- Validasi semua node

### 21.4 Perhentian darurat

- Hentikan darurat proxy (menulis konfigurasi masuk yang kosong)
- Segera hentikan semua lalu lintas masuk

---

## 22. Pemantauan basis data

Pemantauan basis data menunjukkan kesehatan basis data.

### 22.1 Apa yang ditunjukkannya

- Mesin basis data
- Ukuran basis data
- Konektivitas
- Latensi
- Status kumpulan koneksi
- Statistik cache
- Statistik kueri

### 22.2 Mesin yang didukung

- SQLite
- MySQL
- PostgreSQL

---

## 23. Catatan audit

Log audit mencatat operasi.

### 23.1 Apa yang ditunjukkannya

- Kategori
- Jenis acara
- Hasil
- Sumber IP
- Pengguna
- Detail

### 23.2 Log operasi simpul

- Lihat catatan operasi simpul
- Filter berdasarkan simpul

---

## 24. Manajemen catatan

Manajemen log menunjukkan log sistem.

### 24.1 Jenis log

- Log panel
- Log simpul
- Log inti
- Log proksi

### 24.2 Tindakan

- Lihat log
- Unduh log
- Bersihkan log

---

## 25. Ekstensi operasi

“Ekstensi operasi” (`/client-ext`) mengonfigurasi pemasaran rujukan, komisi, dan penarikan;
“Dokumen hukum” (`/legal-documents`) mempertahankan perjanjian sisi portal.

### 25.1 Referensi dan komisi

- **Konfigurasi rabat**: tingkat rabat (persentase pembayaran pertama orang yang diundang, `0` = dinonaktifkan), mode komisi (hanya pembelian pertama / semua pembelian), jumlah penarikan minimum (saldo komisi harus mencapainya sebelum penarikan dapat diminta)
- **Kode undangan**: bila diaktifkan, pendaftaran memerlukan kode undangan; atur panjang kode, hari kedaluwarsa dan penggunaan maksimal (`0` = tidak terbatas), dan buat/lihat daftar kode
- **Tinjauan penarikan**: filter berdasarkan tertunda/disetujui/ditolak; tambahkan catatan pemrosesan dan catat metode pembayaran dan alamat dompet
- **Catatan referensi**: pengundang, orang yang diundang, jumlah komisi, dan status komisi

### 25.2 Dokumen hukum

- Pertahankan “Kebijakan Privasi” dan “Ketentuan Layanan” secara terpisah, satu dokumen Penurunan Harga per lokal, dengan pratinjau langsung;
- Setiap dokumen mungkin menggunakan URL eksternal, bukan halaman bawaan; ketika kosong, halaman bawaan URL digunakan (dapat disalin pada halaman);
- Portal menampilkan versi lokal saat ini dan kembali ke lokal default jika tidak ada.

---

## 26. Pertanyaan Umum

### 26.1 Panel tidak menyala

- Periksa apakah port sudah digunakan
- Periksa apakah `.env` valid
- Periksa log: `journalctl -u corefusion -f`; tanpa systemd, `tail -f <install-dir>/data/logs/panel.log`

### 26.2 Simpul offline

- Periksa apakah Agen sedang berjalan: `systemctl status corefusion-agent`
- Periksa konektivitas jaringan
- Periksa aturan firewall (port `18443`)

### 26.3 Penerbitan sertifikat gagal

- Periksa resolusi DNS
- Periksa apakah port 80 dapat dijangkau
- Periksa konfigurasi penyedia DNS
- Periksa log sertifikat

### 26.4 Langganan tidak berfungsi

- Periksa tautan berlangganan
- Periksa apakah node sedang online
- Periksa apakah proses inti sedang berjalan
- Periksa aturan firewall

### 26.5 Panggilan balik isi ulang gagal

- Periksa panggilan balik URL
- Periksa apakah kunci situs cocok
- Periksa daftar IP yang diizinkan
- Periksa log pesanan

### 26.6 Halaman portal tidak dapat diakses

- Periksa apakah port portal terbuka
- Periksa konfigurasi jalur portal
- **Penting**: pastikan proksi terbalik meneruskan domain portal ke **port mesin portal**, bukan port panel utama — meneruskan ke port yang salah akan menampilkan halaman atau panel kosong, bukan portal. Periksa `upstream` / `local_port` di catatan situs proxy.
- Buka `http://<portal-domain>:<portal-port>` secara langsung untuk memverifikasi mesin portal itu sendiri
- Periksa konfigurasi proksi terbalik
- Periksa log portal

---

## Lampiran

### A. Port bawaan

| Pelabuhan | Tujuan |
|------|---------|
| Ditugaskan oleh wizard pengaturan (port bebas acak secara default) | Panel admin (dapat diubah) |
| 18443 | Agen gRPC (dapat diubah) |
| 0 (dinonaktifkan secara default) | Port mesin berlangganan khusus (kosong atau `0` berbagi port utama) |
| 0 (dinonaktifkan secara default) | Port mesin portal khusus (kosong atau `0` berbagi port utama) |
| 80/443 | Proksi terbalik (Caddy / Nginx) HTTP / HTTPS |

### B. Jalur default

| Jalan | Tujuan |
|------|---------|
| `/` | Panel admin (portal juga berada di `/` ketika tidak ada mesin portal khusus yang dikonfigurasi) |
| `/sub/:token` | Pengambilan langganan (bila tidak ada port langganan khusus yang dikonfigurasi) |
| `/p` | Halaman portal (bila tidak ada port portal khusus yang dikonfigurasi) |
| `/:token` | Pengambilan langganan (dengan port berlangganan khusus, tanpa awalan `/sub`) |

### C. Alat baris perintah

Biner panel mendukung subperintah ini (tidak ada argumen yang memulai server):

```bash
# Start the panel (same as ./corefusion server)
./corefusion

# Run database migrations
./corefusion migrate

# Non-interactive init / reset init state (see 3.4, 3.5)
./corefusion init --help
./corefusion reset-init

# Offline administration (when you cannot sign in)
./corefusion admin reset-password -u admin -p 'new-password'
./corefusion admin 2fa reset -u admin
./corefusion admin list

# Version / help
./corefusion version
./corefusion help
```

> Dengan systemd, gunakan `systemctl restart corefusion` dan `journalctl -u corefusion -f`; tanpa
> systemd (kontainer dan sejenisnya), hentikan proses lama dan mulai lagi dengan `nohup`, lognya
> di `data/logs/panel.log`.

---

## Perubahan v0.13.0-rc.1

- **Pembaruan panel daring**: «Pengaturan sistem → Pembaruan panel» — pilih versi (`pre` =
  pra-rilis), perbarui, riwayat tugas, dan rollback ke biner sebelumnya.
- **Pembaruan Agent daring**: di detail node «Perbarui Agent» dari biner lokal panel atau
  GitHub Releases; node yang diperbarui dari versi lama otomatis menerima `update.public_key`
  (0600) saat terhubung dan kemudian memverifikasi tanda tangan ed25519.
- **Sinkronisasi WAF**: waktu sinkronisasi terakhir disimpan di
  `waf.github_source.last_sync`; restart tidak mengulang timer; `0 3 * * *` berjalan pukul 03:00
  pada zona `panel.timezone` (default Asia/Shanghai); jendela yang terlewat dikejar ~15 detik
  setelah mulai.
- **Daftar putih WAF**: domain panel memelihara `whiteurl.rule` otomatis; pemeriksaan
  URL/argumen/POST dilewati hanya untuk API admin, sehingga menyimpan situs pembayaran dengan
  `http://127.0.0.1:18081` tidak lagi diblokir aturan SSRF.
- **Kebijakan kuota**: akumulasi saat perpanjangan / kosongkan saat kedaluwarsa / reset saat
  perpanjangan (varian dapat mewarisi paket); pada «bulanan» panel memeriksa tiap jam dan
  mengosongkan pemakaian.
- **Situs pembayaran**: aktif/nonaktif tidak lagi mengubah kunci atau metode pembayaran;
  pesanan isi ulang dapat difilter per tipe dan diekspor CSV.
- **Portal**: «Isi ulang» mendukung pembayaran daring lewat situs aktif dan kartu; tanpa situs
  aktif hanya penukaran kartu yang ditampilkan.

---

> **Versi dokumen**: v1.4
> **Terakhir diperbarui**: 2026-11-27
> **Berlaku untuk**: CoreFusion v0.14.5
