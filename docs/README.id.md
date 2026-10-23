<!-- source: README.md | locale: id | updated: 2026-10-23 -->
# Dokumentasi CoreFusion

> Bahasa: [简体中文](README.md) · [English](README.en.md) · [繁體中文](README.zh-TW.md) · [Русский](README.ru.md) · [日本語](README.ja.md) · [한국어](README.ko.md) · [Deutsch](README.de.md) · [Français](README.fr.md) · [Español](README.es.md) · [Português](README.pt.md) · [Türkçe](README.tr.md) · Bahasa Indonesia · [ไทย](README.th.md) · [Tiếng Việt](README.vi.md) · [العربية](README.ar.md) · [فارسی](README.fa.md)

## [USER-GUIDE.id.md](./USER-GUIDE.id.md) — Panduan pengguna

Panduan pengoperasian lengkap untuk **pengguna** (operasi / administrator / operator): instalasi, konfigurasi awal, modul fitur, FAQ, dan lampiran.

## [api/](./api/) — Referensi API publik

Untuk integrator, pengembangan sekunder, dan klien pihak ketiga:

- [api/README.id.md](./api/README.id.md) — ikhtisar: URL dasar, autentikasi, format respons, batas kecepatan
- [api/subscription.id.md](./api/subscription.id.md) — titik akhir langganan (`/sub/:token`), format, header respons, contoh
- [api/portal-api.id.md](./api/portal-api.id.md) — portal klien API (pendaftaran / masuk, paket, pesanan, saldo, tiket)
- [api/charge-callback.id.md](./api/charge-callback.id.md) — tanda tangan dan verifikasi panggilan balik pembayaran
- [api/admin-api.id.md](./api/admin-api.id.md) — admin API tabel rute dikelompokkan berdasarkan modul

## Dokumen desain

Dokumen desain adalah materi internal (kekayaan intelektual inti) dan **tidak didistribusikan** bersama repositori ini. Hubungi pengelola jika Anda membutuhkannya.

> **Cakupan**: aplikasi klien (UI lokal, pengambilalihan TUN, baki, penggabungan multi-langganan) adalah produk terpisah dan berada di luar cakupan repositori ini. Dokumentasi di sini hanya mencakup sisi server (panel/node/pengiriman langganan).

## Panduan pengoperasian lainnya

- `cert-google-eab.md` — Google EAB penerbitan sertifikat *(Cina)*
- `oauth-google-verification.md` — Google Verifikasi OAuth *(Bahasa Mandarin)*
- `notification-templates.md` — variabel template notifikasi *(Bahasa Mandarin)*

## Cakupan terjemahan

| Dokumen | zh-CN | en | zh-TW | Lokal lainnya |
|----------|-------|----|-------|---------------|
| README.md | ✅ | ✅ | ✅ | ru ✅ |
| USER-GUIDE.md | ✅ | ✅ | ✅ | ru ✅ |
| I18N.md | ✅ | — | — | — |
| api/README.md | ✅ | ✅ | ✅ | ru ✅ |
| api/subscription.md | ✅ | ✅ | ✅ | ru ✅ |
| api/portal-api.md | ✅ | ✅ | ✅ | ru ✅ |
| api/charge-callback.md | ✅ | ✅ | ✅ | ru ✅ |
| api/admin-api.md | ✅ | ✅ | ✅ | ru ✅ |
| cert-google-eab.md | ✅ | ✅ | ✅ | ru ✅ |
| oauth-google-verification.md | ✅ | ✅ | ✅ | ru ✅ |
| notification-templates.md | ✅ | ✅ | ✅ | ru ✅ |
| LEGAL.md | ✅ | ✅ | ✅ | ru ✅ |
| THIRD_PARTY_LICENSES.md | ✅ | ✅ | ✅ | ru ✅ |

Lihat [I18N.md](README.id.md) untuk aturan penamaan, navigasi bahasa, dan peninjauan *(Bahasa Mandarin)*.
