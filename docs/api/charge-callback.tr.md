<!-- source: charge-callback.md | locale: tr | updated: 2026-10-23 -->
# Geri Aramayı Şarj Et API

> Diller: [简体中文](charge-callback.md) · [English](charge-callback.en.md) · [繁體中文](charge-callback.zh-TW.md) · [Русский](charge-callback.ru.md) · [日本語](charge-callback.ja.md) · [한국어](charge-callback.ko.md) · [Deutsch](charge-callback.de.md) · [Français](charge-callback.fr.md) · [Español](charge-callback.es.md) · [Português](charge-callback.pt.md) · Türkçe · [Bahasa Indonesia](charge-callback.id.md) · [ไทย](charge-callback.th.md) · [Tiếng Việt](charge-callback.vi.md) · [العربية](charge-callback.ar.md) · [فارسی](charge-callback.fa.md)

**Ödeme platformları / site entegratörleri** için: Bir kullanıcı bağlı bir sitede ödeme yaptıktan sonra ödeme platformunun sunucusu paneli bilgilendirir.

## 1. Uç nokta

```
POST /api/v1/charge/callback/{site_id}
Content-Type: application/json
```

`site_id`, yönetici tarafındaki "bağlı site" kimliğidir (tamsayı). Geri arama, sunucudan sunucuya yapılır ve **HMAC imzalama** ile Taşıyıcı kimlik doğrulaması yerine isteğe bağlı bir IP izin verilenler listesi kullanır.

## 2. Talep gövdesi

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

| Alan | Açıklama |
|---|---|
| `trade_no` | Panel tarafı işlem numarası (sipariş oluşturulurken verilir, `CF` öneki) |
| `site_trade_no` | Ödeme platformu sipariş numarası (imzalı) |
| `status` | İşletme durumu, **imzalanmadı** |
| `amount` / `currency` | Sipariş tutarı (tamsayı sent) ve para birimi (her ikisi de imzalı) |
| `timestamp` | Unix saniyeleri; pencerenin dışında reddedildi (imzalı) |
| `nonce` | Tekrar oynatma koruması: atomik INSERT, her tekrar bir kez işlenir (imzalı) |
| `paid_*` / `method` / `exchange_rate` / `paid_at` | Gerçek ödeme bilgileri, **imzalanmamış** (bütünlük `body_hash` kapsamındadır) |
| `sign` | HMAC-SHA256 imzası, altıgen kodlu (`sign` olmadan gövde üzerinden hesaplanır) |
| `extra` | Uzantı alanları |

## 3. İmzalama algoritması (v3.2 / `version: "v1"`)

1. **`sign`'siz** ham gövdeyi oluşturun ve `body_hash = SHA256(raw body bytes)`'yi (küçük harf onaltılı) hesaplayın.
   > Panel ayrıca `sign` dahil tüm gövdeyi karmalayan eski şemayı da kabul ediyor; yeni entegrasyonlar belgelenen şemayı takip etmelidir.
2. İmzalama yükünü oluşturun (**sözlük sırasına göre anahtarlar**, Go `json.Marshal` semantiği):

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
   > Sır, bağlanan sitenin yönetici arayüzündeki geri arama sırrıdır (döndürmeden sonra önceki sır bir geçiş penceresi boyunca tutulur).

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

## 4. Panel tarafı doğrulama zinciri (aldığınız hataları belirler)

1. Site mevcut ve alan adı doğrulandı → 404 / 403
2. IP izin verilenler listesi (`callback_whitelist` yapılandırılmışsa, listelenmeyen IP'ler **tam olarak engellenir**; aksi halde atlanır ve yalnızca imza kontrol edilir)
3. `timestamp` penceresi + `nonce` tekrar oynatma koruması (atomik INSERT)
4. HMAC doğrulaması (hem aktif hem de eski gizli dizileri kabul eder)
5. Sipariş mevcut ve siteye ait → kayıtsızlık (zaten ödenmiş iade başarısı) → **tutar ve para birimi kilidi doğrulama** → işlem kaydı (bakiye defteri + sipariş durumu)

İşleme **aynı derecede etkilidir**: aynı `trade_no` ile tekrarlanan geri aramalar iki kez gönderilmez. İmza/oran anormallikleri, yöneticilerin manuel olarak inceleyip yeniden deneyebileceği "geri arama etkinlikleri" olarak kaydedilir.

## 5. Yanıtlar

- Başarı: 2xx (panel ödemeyi gönderdikten sonra)
- Arıza: 4xx/5xx + `{ "code": "...", "message": "..." }`; tipik kodlar arasında `INVALID_SIGNATURE`, `IP_BLOCKED`, `ORDER_NOT_FOUND`, miktar uyumsuzluğu vb. bulunur.

> Her geri arama `X-Request-ID`'yi taşır (yoksa panel tarafından oluşturulur); Günlükler ve olaylar uçtan uca izlenebilir. Sorun giderme sırasında bu kimliği yöneticinize verin.

## 6. Test etme ve sorun giderme

- Yöneticiler her sitenin **entegrasyon belgesini** getirebilir: `GET /api/v1/charge-sites/{id}/integration-doc` (sitenin gerçek geri arama URL örneklerini içerir).
- Geri arama olayları: `GET /api/v1/charge-callback-events`; yeniden denemeler: `GET /api/v1/charge-callback-retries`, `POST /api/v1/charge-callback-retries/{id}/retry`.
- Gizli rotasyon: `POST /api/v1/charge-sites/{id}/rotate-secret` (önceki gizli bilgi bir geçiş penceresi boyunca tutulur).
