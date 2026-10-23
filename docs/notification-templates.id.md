<!-- source: notification-templates.md | locale: id | updated: 2026-10-23 -->
# Variabel Templat Pemberitahuan

> Bahasa: [简体中文](notification-templates.md) · [English](notification-templates.en.md) · [繁體中文](notification-templates.zh-TW.md) · [Русский](notification-templates.ru.md) · [日本語](notification-templates.ja.md) · [한국어](notification-templates.ko.md) · [Deutsch](notification-templates.de.md) · [Français](notification-templates.fr.md) · [Español](notification-templates.es.md) · [Português](notification-templates.pt.md) · [Türkçe](notification-templates.tr.md) · Bahasa Indonesia · [ไทย](notification-templates.th.md) · [Tiếng Việt](notification-templates.vi.md) · [العربية](notification-templates.ar.md) · [فارسی](notification-templates.fa.md)

Templat notifikasi menggunakan placeholder `{variable}` yang diganti dengan nilai sebenarnya saat pengiriman. Dokumen ini mencantumkan semua variabel yang didukung, variabel yang tersedia per peristiwa, dan aturan pemicu otomatis.

## 1. Jenis template dan audiens

| Penonton | Saluran | Penerima | Penggunaan khas |
|---|---|---|---|
| **Pengguna panel** (admin) | Telegram / Surel | Saluran dan penerima dikonfigurasi oleh administrator | Peringatan node, laporan cadangan, pengumuman sistem |
| **Pelanggan** | Surel | Email pelanggan sendiri (terisi secara otomatis) | Saldo rendah, pengingat berakhirnya langganan |

## 2. Variabel umum

Tersedia di semua template:

| Variabel | Deskripsi | Contoh |
|---|---|---|
| `{time}` | Kirim waktu | `2026-09-28 14:30:00` |
| `{user}` | Email pelanggan (templat pelanggan) / pengenal pengguna | `user@example.com` |
| `{message}` | Badan pemberitahuan | Teks pengumuman pemeliharaan |

## 3. Variabel peristiwa pelanggan

### Pengingat berakhirnya langganan (event_key = `subscription_expire`)

Dipindai setiap hari dan dikirim ke pelanggan aktif yang habis masa berlakunya dalam 7 hari:

| Variabel | Deskripsi | Contoh |
|---|---|---|
| `{user}` | Email pelanggan | `user@example.com` |
| `{expire_date}` | Masa berlaku langganan sudah habis | `2026-10-05 09:00` |
| `{days}` | Hari tersisa | `7` |
| `{time}` | Kirim waktu | `2026-09-28 09:05:00` |

### Pengingat saldo rendah (event_key = `balance_low`)

Dipindai setiap hari dan dikirim ke pelanggan yang saldonya di bawah ambang batas:

| Variabel | Deskripsi | Contoh |
|---|---|---|
| `{user}` | Email pelanggan | `user@example.com` |
| `{balance}` | Saldo saat ini (satuan mata uang, dua desimal) | `0.85` |
| `{time}` | Kirim waktu | `2026-09-28 09:05:00` |

## 4. Variabel contoh template operasi

Saat Anda mengeklik “Kirim” untuk pengujian konektivitas, templat pengguna panel mengisi placeholder dengan contoh berikut:

| Variabel | Deskripsi | Contoh |
|---|---|---|
| `{node}` | Nama simpul | `node-01` |
| `{backup_file}` | Nama file cadangan | `corefusion-20260101-0900.tar.gz` |
| `{size}` | Ukuran berkas | `128MB` |
| `{domain}` | Domain | `example.com` |
| `{expire_date}` | Tanggal kadaluwarsa | `2026-10-12` |
| `{days}` | Hari tersisa | `14` |
| `{count}` | Hitung | `3` |

## 5. Aturan pemicu otomatis

- **Jadwal**: pemindaian pertama dijalankan 30 detik setelah panel dimulai, lalu setiap hari pada **09:05**
- **Kedaluwarsa langganan**: memindai langganan aktif (`status=active`) yang kedaluwarsa dalam 7 hari; pelanggan juga harus aktif
- **Saldo rendah**: memindai pelanggan aktif dengan saldo ≤ ambang batas; ambang batasnya adalah `notify.balance_threshold`, **dalam sen** (default `100` = 1,00)
- **Deduplikasi**: maksimal satu pesan per pelanggan per acara **per hari** (`notification_send_logs` deduplikasi per hari)

## 6. Pemantauan kesehatan node

Detak jantung node diperiksa terus menerus; **pemberitahuan dikirim hanya ketika keadaan berubah — putaran tanpa perubahan akan senyap**:

| Kunci pengaturan | Deskripsi | Bawaan |
|---|---|---|
| `monitor.node_interval_seconds` | Periksa interval (detik) | `60` (minimal 30) |
| `monitor.node_offline_timeout_seconds` | Batas waktu detak jantung (detik) | `120` |

- **Peringatan offline**: ketika sebuah node beralih dari online ke offline, mengirimkan template admin “**Peringatan offline node**” dengan `{node}` (nama) dan `{time}`
- **Pemberitahuan pemulihan**: ketika sebuah node pulih, kirimkan “**Pemberitahuan pemulihan node**” dengan variabel yang sama
- **Putaran senyap**: tidak ada pesan saat status tidak berubah; setelah panel dimulai ulang, putaran pertama hanya menetapkan garis dasar
- **Monitor tanpa mengirim**: nonaktifkan templat yang sesuai (Diaktifkan = MATI)

## 7. Peringatan kuota lalu lintas bulanan node (otomatis)

- **Pemicu**: ketika lalu lintas bulanan node mencapai ambang kuota, mengirimkan “**Peringatan kuota lalu lintas node**”; ketika melebihi 100% dan ditangguhkan, mengirimkan peringatan melebihi batas (kunci acara `traffic_quota_threshold` / `traffic_quota_exceeded`)
- **Ambang**: `monitor.traffic_quota_threshold_percent` (persentase, `80` default; atur `0` untuk menonaktifkan peringatan ambang batas)
- **Deduplikasi**: paling banyak satu per node per siklus penagihan (siklus mengikuti `traffic_quota_reset_day` node)
- Variabel yang tersedia: `{node}` (nama), `{used}` (GB terpakai), `{quota}` (GB kuota), `{percent}` (persentase), `{time}`

## 8. Notifikasi hasil pencadangan (otomatis)

- **Pemicu**: dikirim segera setelah pencadangan terjadwal (`BackupScheduler`) berjalan — keberhasilan mengirimkan “**Laporan pencadangan harian**”, kegagalan (membuat cadangan / penyimpanan bacaan / pengunggahan ke jarak jauh) mengirimkan “**Peringatan kegagalan pencadangan**”
- Variabel yang tersedia: `{backup_file}` (nama), `{size}` (ukuran, laporan keberhasilan saja), `{message}` (alasan kegagalan, peringatan kegagalan saja), `{time}`
- **Monitor tanpa mengirim**: dilewati secara diam-diam saat templat dinonaktifkan (Diaktifkan = MATI)

## 9. Pengingat masa berlaku sertifikat (otomatis)

- **Pemicu**: setiap hari pukul 09:05, memindai sertifikat `active` yang hampir habis masa berlakunya (**Sertifikat IP dengan sisa 3 hari**, **sertifikat domain dengan sisa 7 hari**) dan mengirimkan templat admin “**Pengingat masa berlaku sertifikat**” per sertifikat
- **Deduplikasi**: maksimal satu pengingat per sertifikat per hari; setelah perpanjangan masa berlakunya berpindah ke luar jendela (IP 3 hari / domain 7 hari) dan pengingat berhenti
- Variabel yang tersedia: `{domain}` (domain sertifikat), `{expire_date}` (tanggal kedaluwarsa), `{days}` (sisa hari), `{time}`

## 10. Peringatan kerusakan proses inti (otomatis)

- **Pemicu**: ketika proses inti (xray/singbox) keluar secara tidak normal atau proses proxy (caddy/nginx) mogok, segera mengirimkan templat admin “**Peringatan kerusakan proses inti**”; supervisor kemudian memulai kembali dengan backoff eksponensial
- Variabel yang tersedia: `{core}` (tipe proses), `{restarts}` / `{max}` (hitungan/batas restart), `{message}` (pesan keluar, hanya proses inti), `{time}`
- **Monitor tanpa mengirim**: dilewati secara diam-diam saat templat dinonaktifkan (Diaktifkan = MATI)

## 11. Pemberitahuan pendaftaran pengguna baru (otomatis)

- **Pemicu**: dikirim segera setelah pelanggan berhasil mendaftar, menggunakan template admin “**Pemberitahuan pendaftaran pengguna baru**”
- Variabel yang tersedia: `{user}` (email pelanggan), `{time}`

## 12. Penggunaan

1. **Tambahkan templat**: Pengaturan notifikasi → Templat notifikasi → Tambahkan templat → “Isi dari preset” dan sesuaikan sesuai kebutuhan
2. **Aktifkan**: centang Diaktifkan dan simpan; pemicu otomatis hanya memproses templat yang diaktifkan
3. **Tes**: klik “Kirim” dalam daftar untuk mengirim pesan nyata dengan variabel contoh (email pengujian template pelanggan dikirim ke alamat pengirim administrator)
4. **Pemberitahuan segera** (kode bisnis): hubungi `NotifyClientEvent(ctx, db, settingSvc, eventKey, clientEmail, vars)` di lokasi acara (isi ulang, pengurangan, perpanjangan) untuk segera memberi tahu satu pelanggan, di luar batas pemindaian harian
