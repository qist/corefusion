<!-- source: subscription.md | locale: id | updated: 2026-11-12 -->
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
| `providers` | Hanya `mihomo_yaml`: `1` / `true` mengaktifkan **Mode Penyedia** (memancarkan `proxy-providers` dan mereferensikannya melalui `use`); default atau nilai lainnya membuat node tetap berada di bawah `proxies`. Lihat §6 |

### Nilai `format` yang didukung

| Nilai | Keluaran | Klien |
|---|---|---|
| `base64_uri` (standar) | Tautan berbagi yang bergabung dengan baris baru, dikodekan base64 secara keseluruhan | V2RayN, Shadowrocket, Surge dan klien umum lainnya |
| `raw_uri` | Tautan berbagi yang bergabung dengan baris baru, teks biasa | Klien yang membutuhkan tautan/debug biasa |
| `mihomo_yaml` | Konfigurasi penuh Clash/Mihomo YAML (dengan grup proxy, aturan) | Clash.Meta / mihomo / Clash Verge Rev |
| `mihomo_proxies_yaml` | YAML hanya dengan daftar node `proxies:` | sumber penyedia proxy mihomo (dengan penyegaran terjadwal) |
| `singbox_json` | Konfigurasi penuh sing-box JSON | sing-box / SFI / FLClash (inti sing-box) |
| `xray_json` | Konfigurasi penuh Xray JSON | Xray-inti |

> Kompatibilitas protokol/format berlaku: misalnya, `anytls` hanya muncul dalam format sing-box/Mihomo; **REALITY node yang dilayani oleh xray dikecualikan dari langganan sing-box** (sing-box tidak mendukung implementasi REALITY xray, lihat sing-box#4520) namun masih muncul di langganan Xray, sementara **REALITY node dilayani oleh sing-box berfungsi di langganan Xray dan sing-box**. Node yang tidak dapat dikenali klien akan dihapus secara otomatis alih-alih menggagalkan seluruh langganan.

## 3. Header respons

| Tajuk | Contoh | Deskripsi |
|---|---|---|
| `Subscription-Userinfo` | `upload=1024; download=2048; total=1073741824; expire=1760000000` | Penggunaan dan kedaluwarsa (byte / Unix detik). **Dikirim juga pada 403**, sehingga klien dapat menunjukkan sisa uang saku |
| `Profile-Update-Interval` | `6` | Interval penyegaran yang disarankan (jam) |
| `Profile-Title` | `CoreFusion` | Nama profil (ditunjukkan oleh Clash Verge Rev / V2RayN, dll.) |
| `Profile-Web-Page-Url` | `https://portal.example.com` | Portal URL (“halaman web terbuka” di klien; jalur root ketika portal memiliki portnya sendiri, jika tidak /p) |
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

# 5) Clash dynamic subscription: config template + Provider mode
curl -L "https://sub.example.com/abcd1234efgh5678?format=clash&config=out&providers=1"

# 6) sing-box / Xray with their own config templates (rules come from the template)
curl -L "https://sub.example.com/abcd1234efgh5678?format=singbox_json&config=sing-box-home"
curl -L "https://sub.example.com/abcd1234efgh5678?format=xray_json&config=xray-home"
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

## 6. Mode penyedia dan beberapa domain langganan

### 6.1 Apa yang dikeluarkan `providers=1`

Dengan `format=mihomo_yaml&providers=1` (atau `format=clash&providers=1`) YAML:

- tidak lagi menyejajarkan node di tingkat atas — ia memancarkan `proxy-providers:`;
- sumber node referensi grup proxy melalui `use: [<provider-key>...]`;
- `proxy-providers` apa pun yang ditulis dalam format langganan bersama (`mihomo_configs.config_json`) **diabaikan** (konfigurasi bersama melayani beberapa paket; penyedia hard-code akan membocorkan seluruh paket). Grup → pemetaan sumber berada di bawah “Manajemen rencana → grup → pemetaan sumber”.

Jenis penyedia:

| Baik hati | Kunci | Catatan |
|---|---|---|
| Dalaman | Nama grup node yang disanitasi | Dihasilkan per grup simpul rencana; hanya berisi node grup itu |
| Dicadangkan | `subscription-nodes` | Semua simpul rencana; sumbernya URL **menghilangkan** parameter `provider` |
| Eksternal | Nama dikonfigurasikan pada paket | Sumber eksternal (`http` YAML atau `file`) |

Bidang umum per penyedia:

- `url` — menunjuk kembali ke langganan ini dengan `format=mihomo_proxies_yaml`, mewarisi `config` / `platform` dari permintaan utama; penyedia internal menambahkan `provider=<key>`;
- `path` — **selalu berasal dari “default penyedia internal → templat jalur cache”** (default `./providers/{key}.yaml`, diganti `{key}`). Penyedia eksternal juga tidak dapat menentukan jalurnya sendiri;
- `health-check` — `enable / url / interval`; penyedia internal mewarisi default, penyedia eksternal mengonfigurasinya sesuai rencana;
- `interval` — interval penyegaran dalam hitungan detik.

### 6.2 Domain mana yang masuk ke penyedia URL (multi-domain)

“Pusat langganan → Titik akhir langganan” mungkin menyimpan **beberapa** catatan (domain + jalur + diaktifkan). Penyedia `url` memilih domain dalam urutan ini:

1. **Domain yang sebenarnya digunakan oleh permintaan ini**: nilai pertama `X-Forwarded-Host` (port dilucuti), jika tidak, `Host`; itu harus cocok dengan titik akhir yang **diaktifkan**;
2. Jika tidak ada yang cocok, kembali ke titik akhir yang diaktifkan dengan **ID terendah**;
3. Jalur berasal dari titik akhir yang cocok (`path`, `/` default), menghasilkan `https://<domain><path>/<token>?format=mihomo_proxies_yaml...`.

Konsekuensi:

- Klien yang mengambil domain baru akan memasukkan domain tersebut ke dalam penyedia — **tidak perlu perubahan panel**;
- Domain lama yang masih diaktifkan dalam daftar tidak lagi di-hardcode;
- Jika klien menyimpan cache penyedia lama URL, memperbarui langganan (atau menghapus cache penyedia/memulai ulang klien) akan mengalihkannya.

Proksi terbalik harus meneruskan `X-Forwarded-Proto` dan `X-Forwarded-Host`; jika tidak, titik akhir cadangan akan digunakan. Templat situs Caddy/Nginx yang dihasilkan panel sudah menyertakan kedua header (`header_up X-Forwarded-Host {host}`/`proxy_set_header X-Forwarded-Host $host;`); tambahkan secara manual jika Anda mempertahankan konfigurasi proxy dengan tangan.

## 7. Catatan

- **Format dan konfigurasi digabungkan secara erat**: templat yang dipilih oleh `config`/`platform` harus cocok dengan `format` permintaan ini; templat yang tidak cocok diabaikan — ini mencegah memasukkan aturan Mihomo ke sing-box/Xray JSON.
- **`providers=1` hanya mempengaruhi Mihomo YAML**: sing-box / Xray / base64 / raw abaikan saja (node ​​tetap sebaris).
- **Pembatasan tarif**: titik akhir langganan memiliki batasnya sendiri (`SubscribeRateLimit`); klien yang kasar ditolak.
- **Access log**: setiap pengambilan mencatat informasi IP / UA / CDN, terlihat di bawah "Log akses langganan" di UI admin.
- Tanpa domain yang dikonfigurasi pada port berlangganan khusus, `X-Forwarded-Proto` / `X-Forwarded-Host` permintaan akan digunakan; pastikan proksi terbalik Anda meneruskan kedua header.
- Dalam penerapan multi-domain, jaga agar daftar titik akhir tetap selaras dengan domain yang sebenarnya digunakan klien; lihat §6.2.
