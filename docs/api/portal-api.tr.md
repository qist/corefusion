<!-- source: portal-api.md | locale: tr | updated: 2026-10-23 -->
# Portal API Referansı (İstemci / Portal API)

> Diller: [简体中文](portal-api.md) · [English](portal-api.en.md) · [繁體中文](portal-api.zh-TW.md) · [Русский](portal-api.ru.md) · [日本語](portal-api.ja.md) · [한국어](portal-api.ko.md) · [Deutsch](portal-api.de.md) · [Français](portal-api.fr.md) · [Español](portal-api.es.md) · [Português](portal-api.pt.md) · Türkçe · [Bahasa Indonesia](portal-api.id.md) · [ไทย](portal-api.th.md) · [Tiếng Việt](portal-api.vi.md) · [العربية](portal-api.ar.md) · [فارسی](portal-api.fa.md)

> Portal ön ucu (`/p` veya özel bir portal bağlantı noktası) ve üçüncü taraf istemciler için **tam** arayüz referansı.
> Rota tanımları: `internal/api/router/router.go` (`clientAuthGroup` / `clientAuthProtected` / `/api/v1/public/*`).

## 1. Genel sözleşmeler

### 1.1 Taban URL

| Dağıtım | Baz URL |
|---|---|
| Ana panel motoru | `https://<panel-domain>/api/v1` (`/p`'deki portal sayfası) |
| Özel portal bağlantı noktası | `https://<portal-domain>[:port]/api/v1` (`/`'deki portal sayfası) |
| Özel portal alanı + ters proxy | Yukarıdakinin aynısı; proxy'nin hem sayfa yollarını hem de `/api/v1`'yi iletmesi gerekir |

### 1.2 Kimlik Doğrulama

- Oturum açtıktan sonra `Authorization: Bearer <access_token>`'yi gönderin;
- Erişim belirteçleri 24 saat sürer, belirteçleri 7 gün yenileyin;
- JWT, `tv` (token_version) içerir: oturum kapatma / zorunlu oturum kapatma bunu artırır ve tüm eski belirteçleri geçersiz kılar
 (`clients.token_version`, bkz. `internal/service/clientauth/service.go`);
- Korunan her uç nokta `ClientAuthMiddleware`'den (sürüm kontrolü + bakım modu koruması) geçer.

### 1.3 Yanıt zarfı

Çoğu uç nokta `{ "code": 0, "data": ..., "message": "..." }` değerini döndürür;
bazı liste uç noktaları `{ "items": [...], "total": N }` veya çıplak bir dizi döndürür (aşağıda uç nokta başına belirtilmiştir).
Hatalar `{ "code": "ERROR_CODE", "message": "...", "request_id": "...", "can_retry": ... }`'yi döndürüyor
(HTTP 4xx/5xx).

### 1.4 Tutar birimleri

Sipariş/bakiye alanları **tamsayı sent** şeklindedir: `990` = 9,90 ¥ (`currency: "CNY"`).

### 1.5 Gerçek örnek (oturum açın → profili getir)

```bash
# Register (when registration_enabled=true)
curl -X POST https://<portal-domain>/api/v1/client-auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","password":"plaintext-password","referral_code":""}'
# → 201 {"data":{"access_token":"<JWT>","refresh_token":"<JWT>","client":{...}}}

# Sign in
curl -X POST https://<portal-domain>/api/v1/client-auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","password":"plaintext-password"}'
# → 200 same as above

# Fetch the current client profile (with token)
curl https://<portal-domain>/api/v1/client-auth/me \
  -H 'Authorization: Bearer <access_token>'
# → 200 {"data":{"id":27,"email":"user@example.com","nickname":"","status":"active","quota_bytes":10737418240,"used_bytes":0,...}}

# Fetch a subscription (no header, token in the path)
curl 'https://<portal-domain>/sub/<subscription_token>?format=mihomo_yaml'
# → 200 Mihomo YAML (or base64_uri / singbox_json / xray_json)

# Error example: expired token
# → 401 {"code":"errClientTokenRevoked","message":"...","request_id":"...","can_retry":false}
```

401'den `POST /api/v1/client-auth/refresh`'yi bir kez arayın ve yeniden deneyin; yine de başarısız olursa yerel oturumu temizleyin (portal şablonu bunu zaten yapıyor).

---

## 2. Genel uç noktalar (oturum açma yok)

| Yöntem | Yol | Açıklama |
|---|---|---|
| GET | `/p` (özel bir portal motorunda `/`) | Portal HTML (sunucu tarafından oluşturulan + Vue çalışma zamanı) |
| GET | `/api/v1/public/portal-config` | Portal çalışma zamanı yapılandırması: site adı, tema renkleri, gezinme, modüller, dil paketleri, `registration_enabled`, `regions` (oturum açma bölgesi kaynağı), `privacy_policy_url` / `terms_of_service_url` / `privacy_policy_content` / `terms_of_service_content` (uyumluluk belgeleri; dile göre eşlenen içerik; bağımsız sayfalar `/p/legal/*` ✓) |
| GET | `/api/v1/public/settings` | Genel ayarlar (şu anda `registration_enabled`) |
| GET | `/api/v1/public/subscription-endpoints` | Abonelik uç noktası listesi (çıplak dizi) |
| GET | `/api/v1/public/articles` | Genel makaleler (yalnızca yayınlanan) |
| GET | `/api/v1/public/articles/:slug` | Makale ayrıntısı |
| GET | `/api/v1/public/client-configs` | İstemci indirme yapılandırması |
| GET | `/sub/:token` | Abonelik teslimi (`format`, base64_uri / raw_uri / mihomo_yaml / singbox_json / xray_json vb. döndürür; iptal edildi/eksik → 404, askıya alındı/süresi doldu/etkin değil → 403. Bkz. [subscription.md](portal-api.tr.md)) |
| GET | `/api/v1/client-auth/oauth/providers` | OAuth sağlayıcı görünürlüğü: `{"data":{"providers":[{"provider":"github","enabled":true},...]}}` |
| GET | `/api/v1/system/maintenance` | Bakım durumu (genel). Portal etkinleştirildiğinde bir bakım sayfası gösterir |

> Bunların tümü bakım korumasını ve genel API hız sınırlamasını içerir.

## 3. Kimlik Doğrulama / OAuth

| Yöntem | Yol | Yetki | Açıklama |
|---|---|---|---|
| POST | `/api/v1/client-auth/register` | — | Kayıt olun. Gövde: `{email, password, referral_code?}`. `{access_token, refresh_token, client}`'yi döndürür; kayıt oranı sınırlı |
| POST | `/api/v1/client-auth/login` | — | Oturum açın. Gövde: `{email, password}`. Aynı yanıt; `token_version` ile JWT'ler yayınlıyor |
| POST | `/api/v1/client-auth/refresh` | — | Yenile. Gövde: `{refresh_token}`. Yeni erişim/yenileme belirteçlerini döndürür (sürüm kontrol edildi; eski yenileme belirteçleri, oturum kapatıldıktan hemen sonra ölür) |
| POST | `/api/v1/client-auth/logout` | ✓ | Oturumu kapatma: sunucu tarafı iptali (`token_version +1`) → istemcinin tüm eski belirteçleri geçersiz |
| POST | `/api/v1/client-auth/rotate-token` | ✓ | Abonelik jetonunu döndürün |
| POST | `/api/v1/client-auth/ticket` | ✓ | **Tek seferlik kısa ömürlü bir bilet** düzenleyin (başlıkların ayarlanamadığı "günlük paketini indirme" gibi `window.open` akışları için) |
| GET | `/api/v1/client-auth/oauth/:provider` | — | OAuth'u başlat: 307 → GitHub/Google onayı (`state` anti-CSRF, 10 dakika); engelli sağlayıcılar 400 döndürüyor |
| GET | `/api/v1/client-auth/oauth/:provider/callback` | — | OAuth geri araması: kodu değiştir → istemci oluştur/bağla → JWT sürümünü yayınla → 302 portalına geri dön (`/<portalPath>?access_token=...&refresh_token=...`) |

> OAuth geri araması bir **tarayıcı yönlendirmesidir** (API çağrısı değil): başarılı bir oturum açma işleminden sonra portal belirteci okur
> `onMounted`'deki URL'den (veya aynı kökenli `postMessage` aracılığıyla, `web` portal şablonundaki `handleOAuthCallback`'ye bakın).

## 4. Hesap

| Yöntem | Yol | Açıklama |
|---|---|---|
| GET | `/api/v1/client-auth/me` | Geçerli istemci: `{id, email, plan_id, plan_name, status, quota_bytes, used_bytes, upload_bytes, download_bytes, expires_at, ip_whitelist, device_limit, sub_token, sub_status, created_at, ...}` (`data` zarf) |
| POST | `/api/v1/client-auth/change-password` | Gövde: `{old_password, new_password}` |
| GET | `/api/v1/client-auth/checktime` | Sunucu zamanı (istemci saat senkronizasyonu) |
| GET | `/api/v1/client-auth/connection-info` | Mevcut plan bağlantı parametreleri: `{concurrent, device_limit, protocol, speed_limit, tls, transport}` |

## 5. Abonelikler

| Yöntem | Yol | Açıklama |
|---|---|---|
| POST | `/api/v1/client-auth/reset-subscription` | Aboneliği sıfırlayın; bu `sub_token`'yi döndürür |

> Abonelik bağlantısı, `/me` (`/sub/:token`) tarafından döndürülen `sub_token`'nin ön ucunda oluşturulmuştur; hayır
> artık `subscription-link` / `subscription-formats` uç noktalarını ayırın.

## 6. Düğümler

| Yöntem | Yol | Açıklama |
|---|---|---|
| GET | `/api/v1/client-auth/nodes` | Geçerli planda görünen düğümler: `{items:[...]}` (ad/protokol/aktarım/bölge gibi alanları görüntüleme) |
| GET | `/api/v1/client-auth/nodes/:id/config` | Tam düğüm yapılandırması |
| GET | `/api/v1/client-auth/nodes/:id/link` | Bağlantıyı paylaş (vless:// vb.) |
| GET | `/api/v1/client-auth/nodes/:id/qrcode` | QR kodu resmi |

## 7. Mağaza / planlar / siparişler

| Yöntem | Yol | Açıklama |
|---|---|---|
| GET | `/api/v1/client-auth/plans` | Plan listesi (çıplak dizi): `{id, name, description, category, sort_order, variants:[{id, name, billing_cycle, duration_days, price(cents), original_price, currency, quota_bytes, ...}]}` |
| GET | `/api/v1/client-auth/plans/:id` | Plan detayı |
| GET | `/api/v1/client-auth/traffic-packs` | Trafik paketi listesi |
| POST | `/api/v1/client-auth/traffic-packs/buy` | Trafik paketi satın alın |
| GET | `/api/v1/client-auth/orders` | Plan siparişlerim (sayfalandırılmış) |
| POST | `/api/v1/client-auth/orders` | Sipariş oluşturun (bir plan çeşidi satın alın/yenileyin) |
| GET | `/api/v1/client-auth/orders/:id` | Sipariş detayı |
| GET | `/api/v1/client-auth/invoices` | Faturalar |
| GET | `/api/v1/client-auth/invoices/:id` | Fatura detayı |

## 8. Dengeleme / şarj etme

| Yöntem | Yol | Açıklama |
|---|---|---|
| GET | `/api/v1/client-auth/balance` | `{data:{client_id, balance(cents), frozen}}` |
| GET | `/api/v1/client-auth/balance/transactions` | Bakiye işlemleri (`type`: ücretlendirme/tüketim/geri ödeme; sayfalandırılmış) |
| GET | `/api/v1/client-auth/charge/sites` | Mevcut şarj yerleri (`ListChargeSites`: yalnızca etkin) |
| POST | `/api/v1/client-auth/charge/create` | Yeniden yükleme siparişi oluşturun (üçüncü taraf ödemesine yönlendirir) |
| GET | `/api/v1/client-auth/charge/orders` | Sipariş listesini yeniden doldur |
| GET | `/api/v1/client-auth/charge/orders/:id` | Sipariş detayını yeniden yükle |
| GET | `/api/v1/client-auth/charge/status/:trade_no` | Trade_no'ya göre ödeme durumunu anketleyin |
| POST | `/api/v1/client-auth/charge/redeem-card` | Yeniden yükleme kartını kullanın |

> Bir müşterinin silinmesi **bakiyesine ve işlemlerine **kademeli** uygulanır (bkz. `internal/service/client/service.go`'deki `DeleteClient`).

## 9. Hediyeler / trafik geçmişi

| Yöntem | Yol | Açıklama |
|---|---|---|
| GET | `/api/v1/client-auth/gifts` | Talep edilebilir hediyeler (çıplak dizi; `type`: gün/bayt) |
| POST | `/api/v1/client-auth/gifts/:id/claim` | Talep |
| GET | `/api/v1/client-auth/traffic-history` | Aylık trafik geçmişi: `{data:{history:[{period_start,total_bytes,...}], months}}` |

## 10. Biletler

| Yöntem | Yol | Açıklama |
|---|---|---|
| GET | `/api/v1/client-auth/tickets` | Bilet listesi: `{items,total}` |
| POST | `/api/v1/client-auth/tickets` | Bilet oluştur |
| GET | `/api/v1/client-auth/tickets/:id` | Bilet ayrıntıları (yanıtlarla birlikte) |
| POST | `/api/v1/client-auth/tickets/:id/reply` | Yanıtla |
| POST | `/api/v1/client-auth/tickets/:id/close` | Kapat |

## 11. Makaleler / kılavuzlar

| Yöntem | Yol | Açıklama |
|---|---|---|
| GET | `/api/v1/client-auth/articles` | Makale listesi (kategori filtreli) |
| GET | `/api/v1/client-auth/articles/:slug` | Makale ayrıntısı |

## 12. Yönlendirmeler (varsayılan olarak devre dışıdır; portal düzenleyicisinde etkinleştirilebilir)

| Yöntem | Yol | Açıklama |
|---|---|---|
| GET | `/api/v1/client-auth/referral/code` | Referans kodum ve bağlantım |
| GET | `/api/v1/client-auth/referral/stats` | Komisyon istatistikleri (`commission_rate`, `available_commission`, `min_withdraw_amount`) |
| GET | `/api/v1/client-auth/referral/records` | Yönlendirme kayıtları |
| GET | `/api/v1/client-auth/referral/withdrawals` | Para çekme kayıtları |
| POST | `/api/v1/client-auth/referral/withdraw` | Para çekme talebinde bulunun |

## 13. Paylaşılan hesaplar

| Yöntem | Yol | Açıklama |
|---|---|---|
| GET | `/api/v1/client-auth/shared-accounts` | Kullanılabilir paylaşılan hesaplar (akış vb.) |
| GET | `/api/v1/client-auth/shared-accounts/:id` | Detay |
| POST | `/api/v1/client-auth/shared-accounts/:id/apply` | Erişim iste |
| POST | `/api/v1/client-auth/shared-accounts/:id/repair` | Tek tıklamayla onarım |

## 14. Günlük paketleri (abone teşhisi)

| Yöntem | Yol | Açıklama |
|---|---|---|
| GET | `/api/v1/client-auth/logs/types` | Paketlenebilecek günlük türleri |
| GET | `/api/v1/client-auth/logs/packages` | Günlük paketlerim |
| POST | `/api/v1/client-auth/logs/package` | Günlük paketleme işi oluşturma (yalnızca giriş düğümleri) |
| GET | `/api/v1/client-auth/logs/packages/:id` | İş durumu (hazır/işleniyor/başarısız) |
| GET | `/api/v1/client-auth/logs/packages/:id/download` | İndirin (`POST /ticket`'yi tercih edin ve `?ticket=`'yi ekleyin) |
| DELETE | `/api/v1/client-auth/logs/packages/:id` | Günlük paketini silme |

## 15. Güvenlik ayarları / IP izin verilenler listesi / kendi yasağını kaldırma

| Yöntem | Yol | Açıklama |
|---|---|---|
| GET/PUT | `/api/v1/client-auth/settings` | İstemci ayarları: `{ip_whitelist, device_limit}` (PUT) |
| GET | `/api/v1/client-auth/settings/whitelist` | İzin verilenler listesi (çıplak dizi) |
| DELETE | `/api/v1/client-auth/settings/whitelist/:id` | İzin verilenler listesi girişini silme |
| POST | `/api/v1/client-auth/settings/auto-add-ip` | Geçerli IP'yi izin verilenler listesine ekle |
| POST | `/api/v1/client-auth/self-unban` | Self servis yasağını kaldırma (girişimler politikayla sınırlıdır) |

> IP izin verilenler listesi **yalnızca abonelik URL'leri için geçerlidir** (`/sub/:token`); portalda oturum açmayı/OAuth'u engellemez veya
> diğer istemci kimlik doğrulama uç noktaları. Kullanıcının izin verilenler listesi yoksa veya URL aboneliği yoksa erişime her zaman izin verilir.
> izin verilenler listesi kullanıcı başına bir ayardır; küresel bir anahtar yoktur.

**Kendi kendine yasağı kaldırma akışı**: yalnızca `auto_quota` (trafik aşımının otomatik olarak yasaklanması) için yönetici tarafı olduğunda kullanılabilir
`self_unban.enabled` açık. Oturum açma (şifre veya OAuth geri araması) `require_self_unban=true` + `self_unban_token` değerini döndürür
(typ=self_unban, 15 dakika geçerli, **yalnızca** `POST /self-unban`'de kullanılabilir; diğer uç noktalar 403/`SELF_UNBAN_TOKEN_SCOPE` değerini döndürür).
Portal daha sonra yasağı kaldırma kartını gösterir; yasağı başarıyla kaldırdıktan sonra kullanıcı tekrar oturum açar. OAuth geri aramaları bunu şu yolla sağlar:
URL parçası `#require_self_unban=1&self_unban_token=..&self_unban_hint=..`.

## 16. Hata kodları (alıntı)

| kod | Senaryo |
|---|---|
| `INVALID_TOKEN` | Belirteci yenile geçersiz/süresi dolmuş/sürüm uyuşmazlığı |
| `INVALID_REQUEST` | Parametre doğrulaması başarısız oldu |
| `NOT_FOUND` | Kaynak eksik (bilet/sipariş/izin verilenler listesi girişi vb.) |
| `RELOAD_FAILED` / `SYNC_FAILED` | (Yönetici tarafı) proxy yapılandırma senkronizasyonu başarısız oldu |
| 401 + `errClientTokenRevoked` | token_version uyumsuzluğu (oturum kapatıldı/atıldı) → istemciler yenilenmeli; bu başarısız olursa yerel belirteçleri temizleyin |

## 17. Uyumluluk belgeleri (gizlilik politikası / hizmet şartları)

Oturum açma kartının alt kısmındaki “Gizlilik Politikası · Hizmet Şartları” bağlantıları (sıradan sitelerle aynı; tıklandığında içerik açılır):

- **Yapılandırma**: portal düzenleyici → Ayarlar → Genel → Uyumluluk belgeleri (**içeriği dile göre düzenleyin**);
 ayrıca `PUT /api/v1/portal/active`'yi `privacy_policy_url` / `privacy_policy_content` / yapılandırılmış alanlarıyla da yapabilirsiniz.
 `terms_of_service_url` / `terms_of_service_content` (nil = değişmedi; içerik bir `{"zh-CN":"...","en":"..."}` haritasıdır).
- **Bağımsız erişilebilir sayfalar** (sunucu tarafından oluşturulmuş, Google doğrulama tarayıcıları için kolay; `?lang=xx`'yi destekler):
  - Ana panel alanı: `GET /p/legal/privacy`, `GET /p/legal/terms`
  - Özel portal etki alanı (kök dağıtım): `GET /legal/privacy`, `GET /legal/terms`
- **Görüntüleme mantığı**: URL veya içerik, **URL kazanır** (harici olarak yeni bir sekmede açılır); içerik sağlanmışsa,
 oturum açma kartı, Markdown'ı geçerli portal dilini kullanarak bir iletişim kutusunda (`renderMd`) oluşturur.
- `/api/v1/public/portal-config` aracılığıyla çalışma zamanında teslim edilir; oturum açma bileşeni tüketir
 `ctx.privacyUrl / termsUrl / privacyContent / termsContent / openLegal / hasLegalLinks`.
- Google OAuth doğrulaması, bu bağlantıların herkese açık olarak erişilebilir olmasını gerektirir → yukarıdaki bağımsız bağlantıları kullanın.
 onay ekranı; bkz. `docs/oauth-google-verification.md` (gizlilik politikası/şartlar şablonlarını içerir).

## 18. Entegrasyon örneği

```js
// Sign in
const { data } = await fetch('/api/v1/client-auth/login', {
  method: 'POST', headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ email, password })
}).then(r => r.json());
localStorage.setItem('client_token', data.access_token);
localStorage.setItem('client_refresh_token', data.refresh_token);

// Authenticated request
const me = await fetch('/api/v1/client-auth/me', {
  headers: { Authorization: 'Bearer ' + data.access_token }
}).then(r => r.json());
```

401'den `POST /refresh`'yi bir kez arayın ve yeniden deneyin; hala başarısız olursa yerel oturumu temizleyin
(portal şablonunun `api()`'si bunu zaten uyguluyor).
