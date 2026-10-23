<!-- source: LEGAL.md | locale: id | updated: 2026-10-23 -->
# Catatan Hukum dan Kepatuhan

> Terjemahan ini disediakan untuk kemudahan. Jika ada perbedaan, teks asli bahasa Mandarin yang berlaku.

> Bahasa: [简体中文](LEGAL.md) · [English](LEGAL.en.md) · [繁體中文](LEGAL.zh-TW.md) · [Русский](LEGAL.ru.md) · [日本語](LEGAL.ja.md) · [한국어](LEGAL.ko.md) · [Deutsch](LEGAL.de.md) · [Français](LEGAL.fr.md) · [Español](LEGAL.es.md) · [Português](LEGAL.pt.md) · [Türkçe](LEGAL.tr.md) · Bahasa Indonesia · [ไทย](LEGAL.th.md) · [Tiếng Việt](LEGAL.vi.md) · [العربية](LEGAL.ar.md) · [فارسی](LEGAL.fa.md)

> Dokumen ini menjelaskan lisensi CoreFusion, kebijakan penggunaan komersial, kewajiban pihak ketiga, dan poin kepatuhan yang harus diperhatikan pengguna di yurisdiksi mereka sendiri.
>
> **Dokumen ini bukan nasihat hukum.** Kepatuhan bergantung pada wilayah penerapan, pelanggan, dan model operasi Anda; berkonsultasilah dengan pengacara lokal yang berkualifikasi.
>
> *Terjemahan ini disediakan untuk kenyamanan. Jika berbeda dengan versi asli Tiongkok, versi Tiongkok yang berlaku.*

## 1. Lisensi proyek

CoreFusion dirilis di bawah **GNU Affero General Public License v3.0 (AGPL-3.0)**; lihat [LICENSE](./LICENSE) untuk teks selengkapnya.

```
Copyright (C) 2026 qist

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published
by the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
```

## 2. Penggunaan komersial dan hosting komersial: diperbolehkan secara tegas

**Setiap individu atau organisasi dapat menggunakan CoreFusion untuk tujuan komersial, termasuk namun tidak terbatas pada:**

- Hosting mandiri dan penerapan internal;
- Menjalankan layanan proxy/node berbayar untuk pihak ketiga;
- Menyediakan SaaS, hosting, operasi terkelola, atau integrasi sistem kepada pihak ketiga dengan biaya tertentu;
- Pengembangan, penyesuaian, dan distribusi sekunder (termasuk pengiriman dengan perangkat keras atau sebagai bagian dari solusi).

**Penggunaan komersial dan hosting komersial di atas tidak memerlukan otorisasi terpisah dan tidak ada biaya lisensi atau bagi hasil.**

Satu-satunya kewajiban adalah kewajiban AGPL-3.0 itu sendiri, terutama dua:

1. **Saat mendistribusikan binari** (§4–§6): berikan Sumber Koresponden kepada penerima (sumber koresponden lengkap), atau penawaran tertulis yang efektif berdasarkan §6; menyimpan pemberitahuan hak cipta dan teks lisensi.
2. **Saat menyediakan versi modifikasi melalui jaringan** (§13): jika Anda memublikasikan versi modifikasi dan membiarkan pihak ketiga berinteraksi dengannya melalui jaringan, Anda harus **menawarkan kepada pengguna tersebut Sumber Sesuai dari versi modifikasi secara gratis** (misalnya, tautan unduhan/repositori sumber terkemuka di panel).

Catatan tambahan:

- Menyebarkan salinan **yang tidak dimodifikasi** dan melayani pengguna tidak dengan sendirinya memicu kewajiban sumber §13; mendistribusikan biner masih termasuk dalam item 1.
- Menagih uang tidak dilarang; AGPL mengatur **ketersediaan sumber**, bukan model bisnis. Anda tidak dapat menolak §13 untuk versi modifikasi hanya karena Anda beroperasi secara komersial atau sumber tertutup.
- Jika Anda menginginkan **lisensi komersial sumber tertutup yang tidak tunduk pada AGPL**, hubungi pemegang hak cipta secara terpisah. Perhatikan bahwa proyek ini menghubungkan komponen GPL-3.0 pihak ketiga (lihat bagian 3); versi sumber tertutup apa pun harus menghapusnya terlebih dahulu.

## 3. Komponen pihak ketiga dan redistribusi

Komponen pihak ketiga utama dan lisensinya tercantum di [THIRD_PARTY_LICENSES.md](LEGAL.id.md).

Catatan khusus:

- Melalui `github.com/xtls/xray-core`, proyek ini secara tidak langsung menghubungkan **GPL-3.0-atau-yang lebih baru** `github.com/sagernet/sing` dan `sing-shadowsocks`. Oleh karena itu **panel distribusi atau binari Agen juga memerlukan pemenuhan kewajiban sumber terkait GPL-3.0**.
- Saat runtime, panel dapat mengunduh atau menjalankan program mandiri seperti Xray-core, sing-box, dan Caddy. Ini **tidak didistribusikan dalam repositori ini**, tetapi jika Anda mengirimkannya bersama kiriman Anda, Anda harus mematuhi lisensinya (terutama GPL-3.0 sing-box: berikan sumber yang sesuai). Hysteria2 dan TUIC adalah protokol yang dibawa oleh sing-box, bukan biner mandiri.
- Data GeoIP/GeoSite yang dibundel (GeoLite2, `geoip.dat`, `geosite.dat`) tunduk pada lisensi datanya sendiri; verifikasi persyaratan sebelum penggunaan komersial.

## 4. Kepatuhan regional dan tanggung jawab pengguna

CoreFusion adalah **perangkat lunak tujuan umum**: tidak menyediakan proxy atau layanan jaringan apa pun, tidak mengirimkan node, bandwidth, atau kredensial, dan tidak memungut biaya. Operasi adalah tanggung jawab penyebar.

Penyebar/operator harus menentukan apakah bisnis mereka memerlukan izin, pengajuan, atau penilaian keamanan di yurisdiksi mereka, termasuk namun tidak terbatas pada:

- **Tiongkok Daratan**: izin usaha telekomunikasi bernilai tambah (ICP/IDC/ISP, dll.), pengajuan ICP, MLPS, penilaian keamanan transfer data lintas batas, dan kewajiban berdasarkan Undang-undang Keamanan Siber, Undang-undang Keamanan Data, dan Undang-undang Perlindungan Informasi Pribadi. Jangan gunakan perangkat lunak ini untuk tujuan yang melanggar hukum setempat.
- **EU/UK**: GDPR / GDPR Inggris, ePrivasi, kewajiban pemroses/pengontrol Digital Services Act (DSA), kepatuhan logging dan cookie.
- **Amerika Serikat**: undang-undang privasi negara bagian, kontrol ekspor enkripsi (EAR) dan penyaringan sanksi.
- **Wilayah lain**: telekomunikasi lokal, komunikasi terenkripsi, pengelolaan konten, lokalisasi data, dan aturan perlindungan konsumen.

Pengguna setuju untuk tidak menggunakan perangkat lunak ini untuk:

- Kegiatan yang melanggar hukum yang berlaku atau aturan pengendalian/sanksi ekspor;
- Melanggar kekayaan intelektual, privasi, atau hak sah lainnya;
- Mendistribusikan konten ilegal, serangan jaringan, penipuan, atau aktivitas kriminal lainnya.

**Setiap tanggung jawab hukum yang timbul dari penggunaan atau penyalahgunaan perangkat lunak ini berada di tangan pengguna.**

## 5. Perlindungan data dan privasi

Panel dapat menyimpan akun, detail kontak, pesanan, panggilan balik pembayaran, langganan, statistik lalu lintas, dan log masuk/operasi. Sebagai pemroses/pengontrol, operator harus:

- Memberikan kebijakan privasi kepada pengguna dan mendapatkan persetujuan yang diperlukan;
- Mengikuti minimalisasi data dan menetapkan periode penyimpanan yang wajar untuk log dan statistik (proyek menyediakan kebijakan pembersihan log yang harus dikonfigurasi oleh operator);
- Mendukung permintaan untuk mengakses, memperbaiki dan menghapus informasi pribadi;
- Menerapkan kontrol akses dan enkripsi ke database, cadangan, kunci pribadi sertifikat, dan kredensial pembayaran;
- Selesaikan penilaian transfer data lintas batas saat diterapkan lintas batas.

## 6. Penafian

Perangkat lunak ini disediakan “sebagaimana adanya” berdasarkan AGPL-3.0 §§15–17, tanpa jaminan tersurat maupun tersirat, termasuk kelayakan untuk diperdagangkan, kesesuaian untuk tujuan tertentu, dan non-pelanggaran. Pemegang hak cipta tidak bertanggung jawab atas kerugian langsung, tidak langsung, insidental, atau konsekuensial yang timbul dari penggunaan perangkat lunak ini.

## 7. Kontak

- Proyek: <https://github.com/qist/corefusion>
- Pertanyaan perizinan komersial/kepatuhan: gunakan Masalah proyek atau rincian kontak yang dipublikasikan di halaman repositori.
