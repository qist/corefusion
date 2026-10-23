<!-- source: README.md | locale: tr | updated: 2026-10-23 -->
# CoreFusion API Belgeleri

> Diller: [简体中文](README.md) · [English](README.en.md) · [繁體中文](README.zh-TW.md) · [Русский](README.ru.md) · [日本語](README.ja.md) · [한국어](README.ko.md) · [Deutsch](README.de.md) · [Français](README.fr.md) · [Español](README.es.md) · [Português](README.pt.md) · Türkçe · [Bahasa Indonesia](README.id.md) · [ไทย](README.th.md) · [Tiếng Việt](README.vi.md) · [العربية](README.ar.md) · [فارسی](README.fa.md)

**Harici entegratörler, ikincil geliştirme ve üçüncü taraf istemciler** için API belgeleri. Tüm yollar dağıtım etki alanına göredir.

| Belge | Seyirci | İçerik |
|---|---|---|
| [subscription.md](./subscription.tr.md) | Aboneler / müşteri geliştiricileri | Abonelik uç noktası (`/sub/:token`): formatlar, başlıklar, durum kodları, örnekler |
| [portal-api.md](README.tr.md) | Portal ön ucu / üçüncü taraf istemcileri | Müşteri portalı API (`/api/v1/client-auth/*`, `/api/v1/public/*`): kaydolma, oturum açma, planlar, siparişler, bakiye, biletler |
| [şarj-geri arama.md](./charge-callback.tr.md) | Ödeme sağlayıcıları | Ödeme geri arama imzalama ve doğrulama |
| [admin-api.tr.md](./admin-api.tr.md) | Panel geliştirme / otomasyon | Tam yönetici API rota tablosu (`/api/v1/*`, modüle göre gruplandırılmış) |

---

## 1. Bağlantı noktaları ve temel URL'ler

Panel, tek bir veritabanını paylaşarak aynı anda üç motoru açığa çıkarabilir:

| Motor | Varsayılan erişim | Amaç |
|---|---|---|
| Ana panel | `https://<domain>/api/v1` | Yönetici arka ucu + portal + abonelikler |
| Abonelik motoru (özel bağlantı noktası) | `https://<domain>:<port>/:token` | Yüksek eş zamanlı abonelik getirme, `/sub` öneki yok |
| Portal motoru (özel bağlantı noktası) | `https://<portal-domain>[:port]/api/v1` | Yalnızca istemci portalı, `/` adresindeki sayfalar |

> Ters proxy hem sayfa yollarını hem de `/api/v1`'yi iletmelidir; `X-Forwarded-Proto` / `X-Forwarded-Host`, gerçek düzeni ve etki alanını geri yüklemekten onur duyar.

## 2. Kimlik doğrulama (üç tür, birbirinin yerine kullanılamaz)

| Yetki | Kimlik bilgisi | Şunlar için geçerlidir: |
|---|---|---|
| **Yönetici yetkilendirmesi** | `Authorization: Bearer <admin_token>` (`/api/v1/auth/login`'den) | Yönetici API |
| **İstemci kimlik doğrulaması** | `Authorization: Bearer <client_token>` (`/api/v1/client-auth/login`'den) | Portal API |
| **Abonelik jetonu** | URL'deki jeton: `/sub/<token>` | Abonelik getirme, başlık yok |

- Müşteri erişim belirteçleri 24 saat sürer, belirteçleri 7 gün yeniler; JWT, `tv` (token_version) içerir, bu nedenle oturum kapatma / zorunlu oturum kapatma tüm eski belirteçleri geçersiz kılar.
- Yönetici rolleri: `admin` / `operator` / `super_admin`; bazı yazma işlemleri `RequireRole(RoleAdmin)` veya üzerini gerektirir.

## 3. Yanıt formatı

Başarı (çoğu uç nokta):

```json
{ "code": 0, "data": { ... }, "message": "ok" }
```

Uç noktaları listele (bazıları), `{ "items": [...], "total": N }` veya çıplak bir dizi döndürür (aşağıda uç nokta başına belirtilmiştir).

Hatalar (HTTP 4xx/5xx):

```json
{ "code": "ERROR_CODE", "message": "human-readable description", "request_id": "…", "can_retry": false }
```

**Tutarlar sent cinsinden tam sayılardır**: `990` = 9,90 ¥ (`currency: "CNY"`).
**Trafik bayt cinsindendir**: hem istekler hem de yanıtlar bayt kullanır.

## 4. Hız limitleri ve bakım modu

- Genel uç noktalar (`/api/v1/public/*`, `/sub/*`, kaydolma/oturum açma) hız sınırlıdır;
- Oturum açma işlemi aynı zamanda `LoginRateLimit`'yi, abonelik getirme işlemi de `SubscribeRateLimit`'yi kullanır;
- Bakım modunda genel uç noktalar ve abonelikler bir bakım mesajı döndürür; yönetici API etkilenmez.

## 5. Ön uç sayfa rotaları (API değil)

Bu rotalar HTML/statik varlıkları döndürür ve API belgelerinin dışındadır; Entegrasyon sırasında hariç tutulabilmeleri için listelenmiştir:

| Yöntem | Yol | Açıklama |
|---|---|---|
| GET | `/` | Yönetici paneli SPA (oturum açtıktan sonra) |
| GET | `/p` | Portal HTML'si (özel bir portal motorunda `/`) |
| GET | `/:token` | Abonelik özel bir bağlantı noktasından getirilir (`/sub` öneki yoktur, `/sub/:token` ile aynı anlambilime sahiptir) |
| GET | `/static/:file` | Ön uç statik varlıkları |

## 6. Gerçeğin kaynağı

`internal/api/router/router.go` (746 rota, 743 belgelenmiştir). [admin-api.md](README.tr.md)'deki rota tablosu koddan oluşturulmuştur; eğer kodla aynı fikirde değilse **kod kazanır**.
