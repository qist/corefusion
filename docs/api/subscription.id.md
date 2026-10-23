<!-- source: subscription.md | locale: id | updated: 2026-10-23 -->
# Berlangganan API (publik)

> Bahasa: [简体中文](subscription.md) · [English](subscription.en.md) · [繁體中文](subscription.zh-TW.md) · [Русский](subscription.ru.md) · [日本語](subscription.ja.md) · [한국어](subscription.ko.md) · [Deutsch](subscription.de.md) · [Français](subscription.fr.md) · [Español](subscription.es.md) · [Português](subscription.pt.md) · [Türkçe](subscription.tr.md) · Bahasa Indonesia · [ไทย](subscription.th.md) · [Tiếng Việt](subscription.vi.md) · [العربية](subscription.ar.md) · [فارسی](subscription.fa.md)

Mengambil daftar simpul langganan pengguna. **Ini adalah titik akhir yang paling banyak digunakan oleh pengguna akhir**; tidak diperlukan header — akses diberikan oleh token di URL.

## 1. Titik akhir

| Penempatan | URL |
|---|---|
| Panel utama | `GET https://<panel-domain>/sub/<token>` |
| Port berlangganan khusus | `GET https://<sub-domain>:<port>/<token>` (tanpa awalan `/sub`) |

`<token>` adalah token akses unik langganan, yang dihasilkan oleh sisi admin (terlihat di bawah “Langganan saya” di portal).

## 2. Parameter kueri

| Parameter | Deskripsi |
|---|---|
| `format` | Format keluaran; mengesampingkan default langganan. Lihat di bawah |
| `config` | Nama template konfigurasi (ditentukan pada “Format langganan” di UI admin). Harus menggunakan format yang sama dengan `format` permintaan ini, jika tidak maka akan diabaikan |
| `platform` | Cocokkan template konfigurasi berdasarkan nama platform (digunakan ketika `config` dihilangkan); formatnya juga harus cocok |

### Nilai `format` yang didukung

| Nilai | Keluaran | Klien |
|---|---|---|
| `base64_uri` (standar) | Tautan berbagi yang bergabung dengan baris baru, dikodekan base64 secara keseluruhan | V2RayN, Shadowrocket, Surge dan klien umum lainnya |
| `raw_uri` | Tautan berbagi yang bergabung dengan baris baru, teks biasa | Klien yang membutuhkan tautan/debug biasa |
| `mihomo_yaml` | Konfigurasi penuh Clash/Mihomo YAML (dengan grup proxy, aturan) | Clash.Meta / mihomo / Clash Verge Rev |
| `mihomo_proxies_yaml` | YAML hanya dengan daftar node `proxies:` | sumber penyedia proxy mihomo (dengan penyegaran terjadwal) |
| `singbox_json` | Konfigurasi penuh sing-box JSON | sing-box / SFI / FLClash (inti sing-box) |
| `xray_json` | Konfigurasi penuh Xray JSON | Xray-inti |

> Kompatibilitas protokol/format berlaku: misalnya, `anytls` hanya muncul dalam format sing-box/Mihomo; Node REALITY di server sing-box tidak muncul di langganan Xray/sing-box JSON. Node yang tidak dapat dikenali klien akan dihapus secara otomatis alih-alih menggagalkan seluruh langganan.

## 3. Header respons

| Tajuk | Contoh | Deskripsi |
|---|---|---|
| `Subscription-Userinfo` | `upload=1024; download=2048; total=1073741824; expire=1760000000` | Penggunaan dan kedaluwarsa (byte / Unix detik). **Dikirim juga pada 403**, sehingga klien dapat menunjukkan sisa uang saku |
| `Profile-Update-Interval` | `6` | Interval penyegaran yang disarankan (jam) |
| `Profile-Title` | `CoreFusion` | Nama profil (ditunjukkan oleh Clash Verge Rev / V2RayN, dll.) |
| `Profile-Web-Page-Url` | `https://…/portal` | Tautan halaman portal (“halaman web terbuka” di klien) |
| `Content-Type` | Per format: `text/plain` / `text/yaml` / `application/json` |  |
| `Content-Disposition` | `attachment; filename="subscription.txt"` |  |

## 4. Kode status

| HTTP | kode | Artinya |
|---|---|---|
| 200 | — | Normal (isinya adalah langganan yang dirender dalam `format`) |
| 400 | `MISSING_TOKEN` | Tidak ada token di URL |
| 403 | (teks biasa) | Langganan ditangguhkan/kedaluwarsa/tidak aktif. Pesan dilokalkan (zh/en/ja/ko) melalui `Accept-Language` dan merupakan alasan satu baris |
| 404 | `SUBSCRIPTION_NOT_FOUND` | Tokennya tidak ada |

> Langganan yang dicabut dianggap tidak ada (404).

## 5. Contoh

```bash
# 1) Fetch with the default format
curl -L "https://panel.example.com/sub/abcd1234efgh5678"

# 2) Request Mihomo YAML with the "clash-global" config template
curl -L "https://panel.example.com/sub/abcd1234efgh5678?format=mihomo_yaml&config=clash-global"

# 3) Node list only (for a mihomo proxy-provider)
curl -L "https://panel.example.com/sub/abcd1234efgh5678?format=mihomo_proxies_yaml"

# 4) Check usage (headers only)
curl -sI "https://panel.example.com/sub/abcd1234efgh5678" | grep -i subscription-userinfo
```

Langganan dinamis Clash/Mihomo (penyegaran berkala):

```yaml
proxy-providers:
  subscription-nodes:
    type: http
    url: "https://panel.example.com/sub/abcd1234efgh5678?format=mihomo_proxies_yaml"
    interval: 3600
    health-check:
      enable: true
      url: https://www.gstatic.com/generate_204
      interval: 300

proxy-groups:
  - name: PROXY
    type: select
    use:
      - subscription-nodes
```

## 6. Catatan

- **Format dan konfigurasi digabungkan secara erat**: templat yang dipilih oleh `config`/`platform` harus cocok dengan `format` permintaan ini; templat yang tidak cocok diabaikan — ini mencegah memasukkan aturan Mihomo ke sing-box/Xray JSON.
- **Pembatasan tarif**: titik akhir langganan memiliki batasnya sendiri (`SubscribeRateLimit`); klien yang kasar ditolak.
- **Access log**: setiap pengambilan mencatat informasi IP / UA / CDN, terlihat di bawah "Log akses langganan" di UI admin.
- Tanpa domain yang dikonfigurasi pada port berlangganan khusus, `X-Forwarded-Proto` / `X-Forwarded-Host` permintaan akan digunakan; pastikan proxy terbalik Anda meneruskan kedua header.
