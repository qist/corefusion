<!-- source: charge-callback.md | locale: id | updated: 2026-10-23 -->
# Biaya Panggilan Balik API

> Bahasa: [简体中文](charge-callback.md) · [English](charge-callback.en.md) · [繁體中文](charge-callback.zh-TW.md) · [Русский](charge-callback.ru.md) · [日本語](charge-callback.ja.md) · [한국어](charge-callback.ko.md) · [Deutsch](charge-callback.de.md) · [Français](charge-callback.fr.md) · [Español](charge-callback.es.md) · [Português](charge-callback.pt.md) · [Türkçe](charge-callback.tr.md) · Bahasa Indonesia · [ไทย](charge-callback.th.md) · [Tiếng Việt](charge-callback.vi.md) · [العربية](charge-callback.ar.md) · [فارسی](charge-callback.fa.md)

Untuk **platform pembayaran/integrator situs**: setelah pengguna membayar di situs yang dipasang, server platform pembayaran akan memberi tahu panel.

## 1. Titik akhir

```
POST /api/v1/charge/callback/{site_id}
Content-Type: application/json
```

`site_id` adalah ID "situs terpasang" sisi admin (bilangan bulat). Callback ini bersifat server-ke-server dan menggunakan **penandatanganan HMAC** ditambah daftar IP opsional yang diizinkan, bukan autentikasi Bearer.

## 2. Permintaan tubuh

```json
{
  "trade_no":       "CF20261010120000abcd1234",
  "site_trade_no":  "payment-platform-order-no",
  "status":         "success",
  "amount":         990,
  "currency":       "CNY",
  "timestamp":      1791640000,
  "nonce":          "random-unique-string",
  "paid_currency":  "CNY",
  "paid_amount":    990,
  "method":         "alipay",
  "exchange_rate":  1.0,
  "paid_at":        "2026-10-10 12:00:01",
  "sign":           "hex(signature)",
  "extra":          {}
}
```

| Bidang | Deskripsi |
|---|---|
| `trade_no` | Nomor transaksi sisi panel (dikeluarkan saat pembuatan pesanan, awalan `CF`) |
| `site_trade_no` | Nomor pesanan platform pembayaran (ditandatangani) |
| `status` | Status bisnis, **belum ditandatangani** |
| `amount` / `currency` | Jumlah pesanan (sen bilangan bulat) dan mata uang (keduanya ditandatangani) |
| `timestamp` | Unix detik; ditolak di luar jendela (ditandatangani) |
| `nonce` | Perlindungan pemutaran ulang: INSERT atom, setiap nonce diproses satu kali (ditandatangani) |
| `paid_*` / `method` / `exchange_rate` / `paid_at` | Info pembayaran aktual, **tidak ditandatangani** (integritas tercakup dalam `body_hash`) |
| `sign` | Tanda tangan HMAC-SHA256, berkode hex (dihitung di seluruh bodi tanpa `sign`) |
| `extra` | Bidang ekstensi |

## 3. Algoritma penandatanganan (v3.2 / `version: "v1"`)

1. Bangun **tubuh mentah tanpa `sign`** dan hitung `body_hash = SHA256(raw body bytes)` (huruf kecil hex).
   > Panel juga menerima skema lama yang melakukan hashing pada seluruh bodi termasuk `sign`; integrasi baru harus mengikuti skema yang terdokumentasi.
2. Buat payload penandatanganan (**kunci dalam urutan leksikografis**, semantik Go `json.Marshal`):

```json
{
  "amount": 990,
  "body_hash": "<hash from step 1>",
  "currency": "CNY",
  "nonce": "<nonce>",
  "site_trade_no": "<site_trade_no>",
  "timestamp": 1791640000,
  "trade_no": "<trade_no>",
  "version": "v1"
}
```

3. `sign = hex( HMAC_SHA256( key=callback secret, msg=JSON bytes from step 2) )`.
   > Rahasianya adalah rahasia panggilan balik situs yang dipasang di UI admin (setelah rotasi, rahasia sebelumnya disimpan untuk satu jendela transisi).

```python
import hashlib, hmac, json

def sign_callback(body: dict, secret: str) -> str:
    # 1) body_hash first: body must not contain sign here
    body_hash = hashlib.sha256(json.dumps(body, separators=(",", ":"), ensure_ascii=False).encode()).hexdigest()
    # 2) signing payload (key order irrelevant: json.dumps(sort_keys=True))
    payload = {
        "amount": body["amount"], "body_hash": body_hash,
        "currency": body["currency"], "nonce": body["nonce"],
        "site_trade_no": body["site_trade_no"], "timestamp": body["timestamp"],
        "trade_no": body["trade_no"], "version": "v1",
    }
    msg = json.dumps(payload, separators=(",", ":"), sort_keys=True).encode()
    return hmac.new(secret.encode(), msg, hashlib.sha256).hexdigest()
```

## 4. Rantai validasi sisi panel (menentukan kesalahan yang Anda dapatkan)

1. Situs ada dan domainnya diverifikasi → 404/403
2. Daftar IP yang diizinkan (jika `callback_whitelist` dikonfigurasi, IP yang tidak terdaftar akan **diblokir secara paksa**; jika tidak, dilewati dan hanya tanda tangan yang diperiksa)
3. Jendela `timestamp` + perlindungan pemutaran ulang `nonce` (INSERT atom)
4. Verifikasi HMAC (menerima rahasia aktif dan lama)
5. Pesanan ada dan milik situs → idempotensi (pengembalian berhasil dibayar) → **validasi kunci jumlah dan mata uang** → posting transaksional (buku besar saldo + status pesanan)

Pemrosesan bersifat **idempoten**: callback berulang dengan `trade_no` yang sama tidak diposting dua kali. Anomali tanda tangan/nilai dicatat sebagai “peristiwa panggilan balik”, yang dapat diperiksa dan dicoba ulang oleh admin secara manual.

## 5. Tanggapan

- Berhasil: 2xx (setelah panel memposting pembayaran)
- Kegagalan: 4xx/5xx + `{ "code": "...", "message": "..." }`; kode tipikal termasuk `INVALID_SIGNATURE`, `IP_BLOCKED`, `ORDER_NOT_FOUND`, ketidakcocokan jumlah, dll.

> Setiap panggilan balik membawa `X-Request-ID` (dihasilkan oleh panel jika tidak ada); log dan peristiwa dapat dilacak dari ujung ke ujung. Berikan ID ini kepada administrator Anda saat memecahkan masalah.

## 6. Pengujian dan pemecahan masalah

- Admin dapat mengambil **dokumen integrasi** setiap situs: `GET /api/v1/charge-sites/{id}/integration-doc` (termasuk contoh callback URL situs yang sebenarnya).
- Acara panggilan balik: `GET /api/v1/charge-callback-events`; percobaan ulang: `GET /api/v1/charge-callback-retries`, `POST /api/v1/charge-callback-retries/{id}/retry`.
- Rotasi rahasia: `POST /api/v1/charge-sites/{id}/rotate-secret` (rahasia sebelumnya disimpan untuk satu jendela transisi).
