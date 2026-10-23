<!-- source: admin-api.md | locale: tr | updated: 2026-10-23 -->
# Yönetici API

> Diller: [简体中文](admin-api.md) · [English](admin-api.en.md) · [繁體中文](admin-api.zh-TW.md) · [Русский](admin-api.ru.md) · [日本語](admin-api.ja.md) · [한국어](admin-api.ko.md) · [Deutsch](admin-api.de.md) · [Français](admin-api.fr.md) · [Español](admin-api.es.md) · [Português](admin-api.pt.md) · Türkçe · [Bahasa Indonesia](admin-api.id.md) · [ไทย](admin-api.th.md) · [Tiếng Việt](admin-api.vi.md) · [العربية](admin-api.ar.md) · [فارسی](admin-api.fa.md)

**Panel geliştirme / otomasyon / entegratörler** için eksiksiz yönetici API rota tablosu.

- Temel URL: `https://<panel-domain>/api/v1`
- Yetki: `Authorization: Bearer <admin_token>` (`POST /api/v1/auth/login`'den)
- Yaygın yanıt/hata formatları ve hız sınırı kuralları: bkz. [README.md](./README.tr.md)
- Gerçeğin kaynağı: `internal/api/router/router.go` (739 yönetici yolu); eğer bu tablo aynı fikirde değilse **kod kazanır**

## İzin işaretleri

| işaretleyici | Anlamı |
|---|---|
| halka açık | Oturum açmanıza gerek yok (bakım koruması/hız sınırlaması hâlâ geçerli olabilir) |
| Herkese açık · ücret sınırlı | Özel bir ücret sınırıyla oturum açmaya gerek yok |
| Oturum açıldı | Oturum açılmış bir yönetici belirteci gerektirir (`AuthRequired`) |
| Yönetici | Rol gerektirir ≥ `admin` (`RequireRole(RoleAdmin)`) |
| Süper yönetici | Rol gerektirir = `super_admin` |

Rol seviyeleri: `viewer(10) < operator(20) < admin(30) < super_admin(40)`; yetersiz roller 403 `INSUFFICIENT_ROLE` döndürüyor.

---

## Hızlı başlangıç (gerçek örnekler)

Bu örnekler gerçek yerel tepkilerden alınmıştır; belirteçler ve veriler çıkarılır.

### 1. Oturum açın ve jeton alın

```bash
curl -X POST https://<panel-domain>/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"your-password"}'
```

Yanıt:

```json
{
  "data": {
    "access_token": "<JWT, valid 24h>",
    "refresh_token": "<JWT, valid 7 days>",
    "user": { "id": 1, "role": "super_admin", "username": "admin" }
  }
}
```

### 2. Belirteçle bir iş uç noktasını arayın

```bash
curl https://<panel-domain>/api/v1/clients?limit=1 \
  -H 'Authorization: Bearer <access_token>'
```

Yanıt (listeler `items` + `total` yapısını kullanır):

```json
{
  "data": {
    "items": [
      {
        "id": 27,
        "email": "user@example.com",
        "status": "active",
        "quota_bytes": 10737418240,
        "used_bytes": 0,
        "plan_id": 1,
        "expires_at": null,
        "created_at": "2026-09-30T15:54:32+08:00"
      }
    ],
    "total": 22,
    "limit": 1,
    "offset": 0
  }
}
```

### 3. Hata yanıtı örnekleri

**401 kimliği doğrulanmadı:**

```json
{
  "code": "MISSING_TOKEN",
  "message": "missing authorization header",
  "request_id": "a02cfdf9-6be0-46be-8584-824caab34dc7",
  "can_retry": false,
  "requires_action": false
}
```

**404 kaynak bulunamadı:**

```json
{
  "code": "CLIENT_NOT_FOUND",
  "message": "client not found",
  "request_id": "099949bd-4ded-468d-a6f8-f8e798af2273",
  "can_retry": false,
  "requires_action": false
}
```

### 4. Sayfalandırma

Tüm liste uç noktaları `limit` (sayfa boyutu, varsayılan 20) ve `offset` (0 tabanlı) kullanır ve `{items, total, limit, offset}` değerini döndürür. Bazı eski uç noktalar çıplak bir dizi döndürür (tablo başına belirtilir).

---

## İçindekiler

- [Yönetici kimlik doğrulaması ve güvenliği](#yönetici-kimlik-doğrulaması-ve-güvenliği)
- [Yönetici hesapları](#yönetici-hesapları)
- [Müşteri yönetimi](#müşteri-yönetimi)
- [Şablonları planlayın](#şablonları-planlayın)
- [Plan varyantları](#plan-varyantları)
- [Trafik paketleri](#trafik-paketleri)
- [Kuponlar](#kuponlar)
- [Düğüm grupları](#düğüm-grupları)
- [Düğüm sunucuları](#düğüm-sunucuları)
- [Düğüm denetim günlüğü](#düğüm-denetim-günlüğü)
- [Düğümler (eski model, kullanımdan kaldırıldı)](#düğümler-eski-model-kullanımdan-kaldırıldı)
- [Bölgeye genel bakış](#bölgeye-genel-bakış)
- [Abonelikler](#abonelikler)
- [Abonelik formatları](#abonelik-formatları)
- [Abonelik uç noktaları](#abonelik-uç-noktaları)
- [Alanları paylaş](#alanları-paylaş)
- [Cloudflare IP](#cloudflare-ip)
- [Abonelik yapılandırması (Mihomo/Clash)](#abonelik-yapılandırması-mihomoclash)
- [Siteleri şarj edin](#siteleri-şarj-edin)
- [Siparişleri yeniden doldur](#siparişleri-yeniden-doldur)
- [Kartları şarj et](#kartları-şarj-et)
- [Geri arama olaylarını ücretlendir](#geri-arama-olaylarını-ücretlendir)
- [Geri arama denemelerini ücretlendir](#geri-arama-denemelerini-ücretlendir)
- [Müşteri bakiyesi](#müşteri-bakiyesi)
- [Hediyeler](#hediyeler)
- [Yönlendirme yapılandırması](#yönlendirme-yapılandırması)
- [Yönlendirmenin geri çekilmesi](#yönlendirmenin-geri-çekilmesi)
- [IP izin verilenler listeleri (özet)](#ip-izin-verilenler-listeleri-özet)
- [Müşteri OAuth bağlamaları](#müşteri-oauth-bağlamaları)
- [Biletler (yönetici)](#biletler-yönetici)
- [Makaleler](#makaleler)
- [Paylaşılan hesaplar](#paylaşılan-hesaplar)
- [Sertifikalar](#sertifikalar)
- [Sertifika kanalları](#sertifika-kanalları)
- [EAB yapılandırması](#eab-yapılandırması)
- [ACME hesapları](#acme-hesapları)
- [Dahili CA](#dahili-ca)
- [Coğrafi kurallar](#coğrafi-kurallar)
- [WAF kural kitaplığı](#waf-kural-kitaplığı)
- [Günlük yönetimi](#günlük-yönetimi)
- [Yedeklemeler](#yedeklemeler)
- [Sistem ayarları](#sistem-ayarları)
- [Güvenlik denetimi](#güvenlik-denetimi)
- [Metrikler](#metrikler)
- [Portal şablonları](#portal-şablonları)
- [Aracı kurulumu](#aracı-kurulumu)
- [Geri arama girişini ücretlendir](#geri-arama-girişini-ücretlendir)
- [Gelenler (düğümlerle)](#gelenler-düğümlerle)
- [Genel yapılandırma](#genel-yapılandırma)
- [İstemci kimlik doğrulaması ve iş (yönetici motorunda aynı ayar)](#istemci-kimlik-doğrulaması-ve-iş-yönetici-motorunda-aynı-ayar)
- [Abonelik teslimi](#abonelik-teslimi)
- [Çalışma zamanı yapılandırma iş akışı](#çalışma-zamanı-yapılandırma-iş-akışı)
- [Hizmet erişim politikaları (kullanımdan kaldırıldı)](#hizmet-erişim-politikaları-kullanımdan-kaldırıldı)
- [Trafik kotası izleme](#trafik-kotası-izleme)
- [Sağlık kontrolleri](#sağlık-kontrolleri)
- [Portal sayfası](#portal-sayfası)
- [ACME zorlukları](#acme-zorlukları)

---

### Yönetici kimlik doğrulaması ve güvenliği

Oturum açma, çıkış yapma, şifre değiştirme, 2FA (TOTP), Passkey (WebAuthn) ve yönetici paneli için tek seferlik biletler.

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| POST | `/api/v1/auth/2fa/disable` | Oturum açıldı | Devre dışı bırak |
| POST | `/api/v1/auth/2fa/enable` | Oturum açıldı | Etkinleştir |
| POST | `/api/v1/auth/2fa/recovery-codes` | Oturum açıldı | Kurtarma kodlarını yeniden oluşturun |
| POST | `/api/v1/auth/2fa/setup` | Oturum açıldı | Ayarlar |
| GET | `/api/v1/auth/2fa/status` | Oturum açıldı | Durum |
| POST | `/api/v1/auth/change-password` | Oturum açıldı | Şifreyi değiştir |
| POST | `/api/v1/auth/login` | halka açık | Oturum aç |
| POST | `/api/v1/auth/login/2fa` | halka açık | Dinamik kodla oturum açma (2FA) |
| GET | `/api/v1/auth/login/2fa-status` | halka açık | Hesapta 2FA'nın etkin olup olmadığını kontrol edin |
| POST | `/api/v1/auth/logout` | Oturum açıldı | Oturumu kapat |
| GET | `/api/v1/auth/me` | Oturum açıldı | Geçerli oturum açmış kullanıcı |
| GET | `/api/v1/auth/passkey/credentials` | Oturum açıldı | Passkey kimlik bilgisi listesi |
| DELETE | `/api/v1/auth/passkey/credentials/:id` | Oturum açıldı | Passkey kimlik bilgisini silin |
| POST | `/api/v1/auth/passkey/login/begin` | halka açık | Oturum açmayı başlat |
| POST | `/api/v1/auth/passkey/login/finish` | halka açık | Oturum açma işlemini tamamlayın |
| POST | `/api/v1/auth/passkey/register/begin` | Oturum açıldı | Kaydı başlat |
| POST | `/api/v1/auth/passkey/register/finish` | Oturum açıldı | Kaydı tamamla |
| GET | `/api/v1/auth/passkey/rpids` | Oturum açıldı | RP ID izin verilenler listesi |
| POST | `/api/v1/auth/passkey/rpids` | Oturum açıldı | RP Kimliği izin verilenler listesini güncelle |
| GET | `/api/v1/auth/passkey/status` | halka açık | Durum |
| POST | `/api/v1/auth/refresh` | halka açık | Yenile |
| POST | `/api/v1/auth/ticket` | Oturum açıldı | Bir kerelik kısa ömürlü bilet düzenleyin |

### Yönetici hesapları

Yönetici paneli kullanıcı hesapları. Listeleme yönetici gerektirir; oluşturma/güncelleme/silme süper yönetici gerektirir (ayrıcalığın artırılmasını önler).

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| GET | `/api/v1/users` | Yönetici | Liste |
| POST | `/api/v1/users` | Süper yönetici | Oluştur |
| DELETE | `/api/v1/users/:id` | Süper yönetici | Sil |
| PUT | `/api/v1/users/:id` | Süper yönetici | Güncelleme |

### Müşteri yönetimi

Aboneler (portal hesapları) için CRUD, askıya alma/devam ettirme, yükseltme/düşürme, yenileme ve kullanım sıfırlama.

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| GET | `/api/v1/clients` | Oturum açıldı | Müşterileri listeleyin |
| POST | `/api/v1/clients` | Oturum açıldı | Müşteri oluşturun. gövde: `{"email":"user@example.com","password":"plaintext-password","plan_id":1,"quota_bytes":10737418240,"expires_at":"2026-12-31T00:00:00Z","remark":""}`. `plan_id` gereklidir; `password` boş olabilir (müşteriler bunu daha sonra portalda ayarlayabilir) |
| DELETE | `/api/v1/clients/:id` | Oturum açıldı | Müşteriyi sil |
| GET | `/api/v1/clients/:id` | Oturum açıldı | Müşteri edinin |
| PUT | `/api/v1/clients/:id` | Oturum açıldı | Müşteriyi güncelle |
| POST | `/api/v1/clients/:id/downgrade` | Oturum açıldı | Müşterinin düzeyini düşür |
| GET | `/api/v1/clients/:id/orders` | Oturum açıldı | Müşteri sipariş listesi |
| POST | `/api/v1/clients/:id/renew` | Oturum açıldı | Müşteriyi yenile |
| POST | `/api/v1/clients/:id/reset-usage` | Oturum açıldı | Kullanımı sıfırla |
| POST | `/api/v1/clients/:id/restore` | Oturum açıldı | Müşteriyi devam ettir |
| POST | `/api/v1/clients/:id/suspend` | Oturum açıldı | Müşteriyi askıya al |
| POST | `/api/v1/clients/:id/upgrade` | Oturum açıldı | Müşteriyi yükseltin |

### Şablonları planlayın

Planlar (varyantlar, fatura döngüleri, fiyatlar, trafik kotaları) ve şablon düzeyinde düğüm yetkilendirmesi.

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| GET | `/api/v1/plan-templates` | Oturum açıldı | Şablon listesi |
| POST | `/api/v1/plan-templates` | Oturum açıldı | Şablon oluştur |
| DELETE | `/api/v1/plan-templates/:id` | Oturum açıldı | Şablonu sil |
| GET | `/api/v1/plan-templates/:id` | Oturum açıldı | Şablon ayrıntıları |
| PUT | `/api/v1/plan-templates/:id` | Oturum açıldı | Şablonu güncelle |
| GET | `/api/v1/plan-templates/:id/accessible-nodes` | Oturum açıldı | Kullanılabilir düğümleri al |
| GET | `/api/v1/plan-templates/:id/billing-node-options` | Oturum açıldı | Faturalandırılabilir düğüm seçeneklerini edinin |
| PUT | `/api/v1/plan-templates/:id/node-groups` | Oturum açıldı | Şablon düğüm gruplarını ayarlama |
| POST | `/api/v1/plan-templates/:id/sync-accounts` | Oturum açıldı | Hesapları şablona senkronize et |
| GET | `/api/v1/plan-templates/:id/variants` | Oturum açıldı | Varyant listesi |
| POST | `/api/v1/plan-templates/:id/variants` | Oturum açıldı | Varyant oluştur |
| POST | `/api/v1/plan-templates/sync-all-accounts` | Oturum açıldı | Tüm hesapları senkronize et |

### Plan varyantları

Plan değişkenlerini güncelleyin ve silin (oluşturma, plan şablonlarının altına yerleştirilmiştir).

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| DELETE | `/api/v1/plan-variants/:id` | Oturum açıldı | Varyantı sil |
| PUT | `/api/v1/plan-variants/:id` | Oturum açıldı | Varyantı güncelle |

### Trafik paketleri

Ayrı olarak satın alınabilen trafik paketleri.

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| GET | `/api/v1/traffic-packs` | Oturum açıldı | Trafik paketi listesi |
| POST | `/api/v1/traffic-packs` | Oturum açıldı | Trafik paketi oluştur |
| DELETE | `/api/v1/traffic-packs/:id` | Oturum açıldı | Trafik paketini sil |
| PUT | `/api/v1/traffic-packs/:id` | Oturum açıldı | Trafik paketini güncelle |

### Kuponlar

Kuponları oluşturun, doğrulayın ve kullanın.

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| GET | `/api/v1/promotions` | Oturum açıldı | Kuponları listele |
| POST | `/api/v1/promotions` | Oturum açıldı | Kupon oluştur |
| DELETE | `/api/v1/promotions/:id` | Oturum açıldı | Kuponu sil |
| PUT | `/api/v1/promotions/:id` | Oturum açıldı | Kuponu güncelle |
| POST | `/api/v1/promotions/validate` | Oturum açıldı | Kuponu doğrula |

### Düğüm grupları

Düğümleri planların referans verdiği gruplar halinde düzenleyin.

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| GET | `/api/v1/node-groups` | Oturum açıldı | Düğüm gruplarını listeleyin |
| POST | `/api/v1/node-groups` | Oturum açıldı | Düğüm grubu oluştur |
| DELETE | `/api/v1/node-groups/:id` | Oturum açıldı | Düğüm grubunu sil |
| PUT | `/api/v1/node-groups/:id` | Oturum açıldı | Düğüm grubunu güncelle |
| GET | `/api/v1/node-groups/:id/members` | Oturum açıldı | Grup üyelerini listeleyin |
| POST | `/api/v1/node-groups/:id/members` | Oturum açıldı | Grup üyesi ekle |
| DELETE | `/api/v1/node-groups/:id/members/:mid` | Oturum açıldı | Grup üyesini kaldır |

### Düğüm sunucuları

Çekirdek kaynaklar: düğüm CRUD, çekirdek (xray/sing-box) yapılandırma teslimi, gelen/giden/uç nokta yapılandırması, süreç kontrolü, sertifikalar, WAF, hız sınırları, bağlantı noktası atlama, zincirleme proxy'ler, yansıtma/birincil yedekleme, Aracı yükleme/yükseltme, xray çalışma zamanı kontrolü. Özellik ailesine göre gruplandırılmış birçok alt yol.

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| GET | `/api/v1/node-servers` | Yönetici | Liste |
| POST | `/api/v1/node-servers` | Yönetici | Oluştur |
| DELETE | `/api/v1/node-servers/:id` | Yönetici | Sil |
| GET | `/api/v1/node-servers/:id` | Yönetici | Ayrıntılar |
| PUT | `/api/v1/node-servers/:id` | Yönetici | Güncelleme |
| GET | `/api/v1/node-servers/:id/binaries` | Yönetici | Yüklü çekirdek listesi |
| POST | `/api/v1/node-servers/:id/binaries/ech-generate` | Yönetici | ECH anahtar çifti oluştur |
| POST | `/api/v1/node-servers/:id/binaries/install` | Yönetici | Çekirdek yükleme/yükseltme |
| GET | `/api/v1/node-servers/:id/binaries/interfaces` | Yönetici | Ağ arayüzü listesi |
| POST | `/api/v1/node-servers/:id/binaries/mldsa65-key` | Yönetici | MLDSA65 anahtarı oluştur |
| POST | `/api/v1/node-servers/:id/binaries/reality-key` | Yönetici | REALITY anahtar çifti oluştur |
| POST | `/api/v1/node-servers/:id/binaries/reality-public-key` | Yönetici | Genel anahtarı REALITY özel anahtarından türetin |
| GET | `/api/v1/node-servers/:id/binaries/releases` | Yönetici | Mevcut versiyonlar |
| POST | `/api/v1/node-servers/:id/binaries/vlessenc` | Yönetici | VLESS şifreleme parametreleri oluşturun |
| POST | `/api/v1/node-servers/:id/binaries/wg-key` | Yönetici | WireGuard anahtarı oluştur |
| POST | `/api/v1/node-servers/:id/binaries/x25519-key` | Yönetici | X25519 anahtar çifti oluştur |
| GET | `/api/v1/node-servers/:id/bootstrap-cmd` | Yönetici | Agent önyükleme komutunu al |
| POST | `/api/v1/node-servers/:id/bootstrap-token` | Yönetici | Önyükleme jetonu oluştur |
| POST | `/api/v1/node-servers/:id/caddy/batch-update-log-enabled` | Yönetici | Toplu güncelleme günlüğü anahtarları |
| POST | `/api/v1/node-servers/:id/caddy/batch-update-ports` | Yönetici | Toplu güncelleme bağlantı noktaları |
| POST | `/api/v1/node-servers/:id/caddy/generate-config` | Yönetici | Yapılandırma oluştur |
| DELETE | `/api/v1/node-servers/:id/caddy/l4-rules` | Yönetici | Tüm L4 kurallarını sil |
| POST | `/api/v1/node-servers/:id/caddy/save-and-apply` | Yönetici | Kaydet ve uygula |
| GET | `/api/v1/node-servers/:id/caddy/sites` | Yönetici | Site listesi |
| POST | `/api/v1/node-servers/:id/caddy/sites` | Yönetici | Site oluştur |
| DELETE | `/api/v1/node-servers/:id/caddy/sites/:siteId` | Yönetici | Siteyi sil |
| PUT | `/api/v1/node-servers/:id/caddy/sites/:siteId` | Yönetici | Siteyi güncelle |
| DELETE | `/api/v1/node-servers/:id/caddy/sni-rules` | Yönetici | Tüm SNI kurallarını sil |
| POST | `/api/v1/node-servers/:id/caddy/sni-rules/toggle` | Yönetici | SNI kurallarını değiştir |
| GET | `/api/v1/node-servers/:id/caddy/sni-settings` | Yönetici | SNI ayarları |
| POST | `/api/v1/node-servers/:id/caddy/sni-settings` | Yönetici | SNI ayarlarını kaydet |
| GET | `/api/v1/node-servers/:id/certificates` | Yönetici | Düğüm sertifikası listesi |
| DELETE | `/api/v1/node-servers/:id/certificates/:cid` | Yönetici | Sertifikanın bağlantısını kaldır |
| POST | `/api/v1/node-servers/:id/certificates/:cid` | Yönetici | Sertifikayı bağla |
| GET | `/api/v1/node-servers/:id/chain-config` | Yönetici | Zincirleme yapılandırma oluştur |
| GET | `/api/v1/node-servers/:id/chain-health` | Yönetici | Zincir sağlığı durumu |
| POST | `/api/v1/node-servers/:id/control` | Süper yönetici | Kontrol düğümü çekirdeği. gövde: `{"core_type":"xray","action":"start"}` (eylem = başlat/durdur/yeniden başlat). `core_type`, `xray`/`singbox` olabilir |
| GET | `/api/v1/node-servers/:id/degradation-status` | Yönetici | Bozulmuş durum |
| POST | `/api/v1/node-servers/:id/distribute-cert` | Yönetici | Sertifikayı dağıt |
| GET | `/api/v1/node-servers/:id/endpoint-configs` | Yönetici | Liste |
| POST | `/api/v1/node-servers/:id/endpoint-configs` | Yönetici | Oluştur |
| DELETE | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | Yönetici | Sil |
| GET | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | Yönetici | Ayrıntılar |
| PUT | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | Yönetici | Güncelleme |
| POST | `/api/v1/node-servers/:id/failover` | Yönetici | Bu düğüm için yük devretmeyi tetikleyin |
| GET | `/api/v1/node-servers/:id/failover-status` | Yönetici | Yük devretme durumu |
| POST | `/api/v1/node-servers/:id/force-reconnect` | Yönetici | Yeniden bağlanmaya zorla |
| GET | `/api/v1/node-servers/:id/free-port` | Yönetici | Ücretsiz bir bağlantı noktası isteyin |
| POST | `/api/v1/node-servers/:id/geo-rules/push` | Yönetici | Bu düğüme gönder |
| GET | `/api/v1/node-servers/:id/inbound-configs` | Yönetici | Liste |
| POST | `/api/v1/node-servers/:id/inbound-configs` | Yönetici | Oluştur |
| DELETE | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | Yönetici | Sil |
| GET | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | Yönetici | Ayrıntılar |
| PUT | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | Yönetici | Güncelleme |
| GET | `/api/v1/node-servers/:id/inbound-configs/:cfgId/share-links` | Yönetici | Bağlantıları ve parçacıkları paylaşın |
| POST | `/api/v1/node-servers/:id/inbound-configs/:cfgId/users/:email/pause` | Yönetici | Bu kullanıcıyı askıya al |
| POST | `/api/v1/node-servers/:id/inbound-configs/:cfgId/users/:email/resume` | Yönetici | Bu kullanıcıyı geri yükle |
| POST | `/api/v1/node-servers/:id/inbound-configs/reset-all-traffic` | Yönetici | Tüm gelen trafiği sıfırla |
| GET | `/api/v1/node-servers/:id/install-script` | Yönetici | Agent yükleme komut dosyasını edinin |
| GET | `/api/v1/node-servers/:id/metrics-history` | Yönetici | Metrik geçmişini alın |
| GET | `/api/v1/node-servers/:id/mirror-status` | Yönetici | Ayna durumu |
| POST | `/api/v1/node-servers/:id/nginx/batch-update-ports` | Yönetici | Toplu güncelleme bağlantı noktaları |
| POST | `/api/v1/node-servers/:id/nginx/generate-config` | Yönetici | Yapılandırma oluştur |
| DELETE | `/api/v1/node-servers/:id/nginx/l4-rules` | Yönetici | Tüm L4 kurallarını sil |
| POST | `/api/v1/node-servers/:id/nginx/save-and-apply` | Yönetici | Kaydet ve uygula |
| GET | `/api/v1/node-servers/:id/nginx/settings` | Yönetici | Ayarları oku |
| POST | `/api/v1/node-servers/:id/nginx/settings` | Yönetici | Ayarları kaydet |
| GET | `/api/v1/node-servers/:id/nginx/sites` | Yönetici | Site listesi |
| POST | `/api/v1/node-servers/:id/nginx/sites` | Yönetici | Site oluştur |
| DELETE | `/api/v1/node-servers/:id/nginx/sites/:siteId` | Yönetici | Siteyi sil |
| PUT | `/api/v1/node-servers/:id/nginx/sites/:siteId` | Yönetici | Siteyi güncelle |
| DELETE | `/api/v1/node-servers/:id/nginx/sni-rules` | Yönetici | Tüm SNI kurallarını sil |
| POST | `/api/v1/node-servers/:id/nginx/sni-rules/toggle` | Yönetici | SNI kurallarını değiştir |
| PUT | `/api/v1/node-servers/:id/node-type` | Yönetici | Düğüm türünü güncelle |
| GET | `/api/v1/node-servers/:id/online-users` | Yönetici | Çevrimiçi kullanıcılar |
| GET | `/api/v1/node-servers/:id/outbound-configs` | Yönetici | Liste |
| POST | `/api/v1/node-servers/:id/outbound-configs` | Yönetici | Oluştur |
| DELETE | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | Yönetici | Sil |
| GET | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | Yönetici | Ayrıntılar |
| PUT | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | Yönetici | Güncelleme |
| PUT | `/api/v1/node-servers/:id/outbound-configs/reorder` | Yönetici | Sırala |
| GET | `/api/v1/node-servers/:id/port-hop/rules` | Yönetici | Bağlantı noktası atlama kuralı önizlemesi |
| GET | `/api/v1/node-servers/:id/protocols` | Yönetici | Protokol listesi |
| GET | `/api/v1/node-servers/:id/protocols/:coreType` | Yönetici | Protokol yapılandırması |
| PUT | `/api/v1/node-servers/:id/protocols/:coreType` | Yönetici | Protokol yapılandırmasını kaydet |
| GET | `/api/v1/node-servers/:id/protocols/:coreType/config-file` | Yönetici | Çekirdek yapılandırma dosyasını okuyun |
| GET | `/api/v1/node-servers/:id/protocols/:coreType/section/:section` | Yönetici | Protokol bölümünü okuyun |
| PUT | `/api/v1/node-servers/:id/protocols/:coreType/section/:section` | Yönetici | Protokol bölümünü kaydet |
| POST | `/api/v1/node-servers/:id/protocols/:coreType/validate` | Yönetici | Yapılandırmayı doğrula (gerçek çekirdek deneme çalışması) |
| GET | `/api/v1/node-servers/:id/proxy-status` | Yönetici | Uzak proxy durumu |
| POST | `/api/v1/node-servers/:id/proxy/batch-update-ports` | Yönetici | Toplu güncelleme bağlantı noktaları |
| GET | `/api/v1/node-servers/:id/proxy/config` | Yönetici | Yapılandırmayı oku |
| PUT | `/api/v1/node-servers/:id/proxy/config` | Yönetici | Yapılandırmayı güncelle |
| POST | `/api/v1/node-servers/:id/proxy/control` | Yönetici | Zincirlenmiş proxy kayıt kontrolü |
| POST | `/api/v1/node-servers/:id/proxy/download` | Yönetici | Proxy çekirdek sürümünü indirin |
| POST | `/api/v1/node-servers/:id/proxy/generate-config` | Yönetici | Yapılandırma oluştur |
| POST | `/api/v1/node-servers/:id/proxy/guardian` | Yönetici | Yönetici yapılandırmasını güncelle |
| POST | `/api/v1/node-servers/:id/proxy/install` | Yönetici | Proxy'yi yükle |
| DELETE | `/api/v1/node-servers/:id/proxy/l4-rules` | Yönetici | Tüm L4 kurallarını sil |
| GET | `/api/v1/node-servers/:id/proxy/l4-settings` | Yönetici | L4 ayarları |
| POST | `/api/v1/node-servers/:id/proxy/l4-settings` | Yönetici | L4 ayarlarını kaydet |
| GET | `/api/v1/node-servers/:id/proxy/log-settings` | Yönetici | Günlük ayarları |
| POST | `/api/v1/node-servers/:id/proxy/log-settings` | Yönetici | Günlük ayarlarını kaydet |
| GET | `/api/v1/node-servers/:id/proxy/nginx-settings` | Yönetici | nginx ayarları |
| POST | `/api/v1/node-servers/:id/proxy/nginx-settings` | Yönetici | nginx ayarlarını kaydet |
| GET | `/api/v1/node-servers/:id/proxy/releases` | Yönetici | Mevcut versiyonlar |
| POST | `/api/v1/node-servers/:id/proxy/reload` | Yönetici | Yapılandırmayı yeniden yükle |
| POST | `/api/v1/node-servers/:id/proxy/restart` | Yönetici | Yeniden başlat |
| GET | `/api/v1/node-servers/:id/proxy/sites` | Yönetici | Site listesi |
| POST | `/api/v1/node-servers/:id/proxy/sites` | Yönetici | Site oluştur |
| DELETE | `/api/v1/node-servers/:id/proxy/sites/:siteId` | Yönetici | Siteyi sil |
| PUT | `/api/v1/node-servers/:id/proxy/sites/:siteId` | Yönetici | Siteyi güncelle |
| DELETE | `/api/v1/node-servers/:id/proxy/sni-rules` | Yönetici | Tüm SNI kurallarını sil |
| POST | `/api/v1/node-servers/:id/proxy/sni-rules/toggle` | Yönetici | SNI kurallarını değiştir |
| GET | `/api/v1/node-servers/:id/proxy/sni-settings` | Yönetici | SNI ayarları |
| POST | `/api/v1/node-servers/:id/proxy/sni-settings` | Yönetici | SNI ayarlarını kaydet |
| POST | `/api/v1/node-servers/:id/proxy/start` | Yönetici | Başlat |
| GET | `/api/v1/node-servers/:id/proxy/status` | Yönetici | Proxy işlem durumu |
| POST | `/api/v1/node-servers/:id/proxy/stop` | Yönetici | Durdur |
| POST | `/api/v1/node-servers/:id/proxy/sync` | Yönetici | Proxy yapılandırmasını senkronize et |
| GET | `/api/v1/node-servers/:id/rate-limit` | Yönetici | Yapılandırmayı oku |
| PUT | `/api/v1/node-servers/:id/rate-limit` | Yönetici | Yapılandırmayı güncelle |
| GET | `/api/v1/node-servers/:id/rate-limit/status` | Yönetici | Durum |
| GET | `/api/v1/node-servers/:id/recover-mode` | Yönetici | Düğüm kurtarma modunu oku |
| PUT | `/api/v1/node-servers/:id/recover-mode` | Yönetici | Düğüm kurtarma modunu ayarlayın |
| POST | `/api/v1/node-servers/:id/revoke-cert` | Yönetici | Temsilci sertifikasını iptal et |
| GET | `/api/v1/node-servers/:id/services` | Yönetici | Liste |
| POST | `/api/v1/node-servers/:id/services` | Yönetici | Oluştur |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId` | Yönetici | Sil |
| GET | `/api/v1/node-servers/:id/services/:serviceId` | Yönetici | Ayrıntılar |
| PUT | `/api/v1/node-servers/:id/services/:serviceId` | Yönetici | Güncelleme |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId/crash-logs` | Yönetici | Kilitlenme günlüklerini temizle |
| GET | `/api/v1/node-servers/:id/services/:serviceId/crash-logs` | Yönetici | Kilitlenme günlüğü listesi |
| GET | `/api/v1/node-servers/:id/services/:serviceId/crash-logs/:logId` | Yönetici | Kilitlenme günlüğü ayrıntıları |
| GET | `/api/v1/node-servers/:id/services/:serviceId/endpoints` | Yönetici | Uç nokta listesi |
| POST | `/api/v1/node-servers/:id/services/:serviceId/endpoints` | Yönetici | Uç nokta oluştur |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId/endpoints/:endpointId` | Yönetici | Uç noktayı sil |
| PUT | `/api/v1/node-servers/:id/services/:serviceId/endpoints/:endpointId` | Yönetici | Uç noktayı güncelle |
| PUT | `/api/v1/node-servers/:id/services/:serviceId/guardian` | Yönetici | Yönetici yapılandırmasını güncelle |
| GET | `/api/v1/node-servers/:id/services/:serviceId/process-status` | Yönetici | İşleme durumunu al |
| POST | `/api/v1/node-servers/:id/services/:serviceId/restart` | Yönetici | Yeniden başlat |
| POST | `/api/v1/node-servers/:id/services/:serviceId/start` | Yönetici | Başlat |
| POST | `/api/v1/node-servers/:id/services/:serviceId/stop` | Yönetici | Durdur |
| GET | `/api/v1/node-servers/:id/services/process-status` | Yönetici | Tüm süreç durumları |
| GET | `/api/v1/node-servers/:id/share-domains` | Yönetici | Bu düğüm için alanları paylaşın |
| POST | `/api/v1/node-servers/:id/share-domains` | Yönetici | Oluştur |
| DELETE | `/api/v1/node-servers/:id/share-domains/:domainId` | Yönetici | Sil |
| PUT | `/api/v1/node-servers/:id/share-domains/:domainId` | Yönetici | Güncelleme |
| DELETE | `/api/v1/node-servers/:id/share-domains/all` | Yönetici | Tümünü temizle |
| POST | `/api/v1/node-servers/:id/singbox/control/firewall/apply` | Yönetici | Bağlantı noktası atlamalı güvenlik duvarı kurallarını uygulama |
| POST | `/api/v1/node-servers/:id/singbox/control/firewall/remove` | Yönetici | Bağlantı noktası atlamalı güvenlik duvarı kurallarını kaldırın |
| POST | `/api/v1/node-servers/:id/speed-test` | Yönetici | Düğüm hızı testi |
| GET | `/api/v1/node-servers/:id/status` | Yönetici | Düğüm durumu |
| POST | `/api/v1/node-servers/:id/sync` | Yönetici | Düğüm yapılandırmasını senkronize et (oluştur ve gönder) |
| POST | `/api/v1/node-servers/:id/sync-data` | Süper yönetici | Panel verilerini düğümlere aktarma (kullanıcıların/aboneliklerin/planların tam senkronizasyonu) |
| POST | `/api/v1/node-servers/:id/sync-mirror` | Yönetici | Ayna düğümünü senkronize et |
| GET | `/api/v1/node-servers/:id/tags` | Yönetici | Etiket listesi |
| POST | `/api/v1/node-servers/:id/tags` | Yönetici | Etiket ekle |
| DELETE | `/api/v1/node-servers/:id/tags/:tagKey` | Yönetici | Etiketi sil |
| GET | `/api/v1/node-servers/:id/traffic-history` | Yönetici | Trafik geçmişi |
| POST | `/api/v1/node-servers/:id/traffic-resume` | Yönetici | Aşım nedeniyle askıya alınan trafiği geri yükleme |
| GET | `/api/v1/node-servers/:id/traffic-usage` | Yönetici | Trafik kullanımı |
| POST | `/api/v1/node-servers/:id/update-agent` | Yönetici | Yükseltme Aracısı |
| GET | `/api/v1/node-servers/:id/used-server-names` | Yönetici | Kullanımdaki REALITY SNI listesi |
| GET | `/api/v1/node-servers/:id/waf/config` | Yönetici | Yapılandırmayı oku |
| PUT | `/api/v1/node-servers/:id/waf/config` | Yönetici | Yapılandırmayı kaydet |
| POST | `/api/v1/node-servers/:id/waf/disable-domain` | Yönetici | Etki alanı korumasını devre dışı bırak |
| GET | `/api/v1/node-servers/:id/waf/domain-config` | Yönetici | Etki alanı yapılandırma listesi |
| PUT | `/api/v1/node-servers/:id/waf/domain-config` | Yönetici | Etki alanı yapılandırmalarını toplu kaydetme |
| DELETE | `/api/v1/node-servers/:id/waf/domain-config/:domain` | Yönetici | Etki alanı yapılandırmasını sil |
| GET | `/api/v1/node-servers/:id/waf/domain-config/:domain` | Yönetici | Etki alanı yapılandırma ayrıntıları |
| PUT | `/api/v1/node-servers/:id/waf/domain-config/:domain` | Yönetici | Etki alanı yapılandırmasını kaydet |
| GET | `/api/v1/node-servers/:id/waf/domain-rules` | Yönetici | Etki alanı kuralı dizini |
| GET | `/api/v1/node-servers/:id/waf/domain-rules/:domain/:name` | Yönetici | Etki alanı kural dosyasını oku |
| PUT | `/api/v1/node-servers/:id/waf/domain-rules/:domain/:name` | Yönetici | Alan kuralı dosyasını kaydet |
| GET | `/api/v1/node-servers/:id/waf/domain-status` | Yönetici | Etki alanı koruma durumu |
| POST | `/api/v1/node-servers/:id/waf/enable-domain` | Yönetici | Etki alanı korumasını etkinleştir |
| POST | `/api/v1/node-servers/:id/waf/push` | Yönetici | Bu düğüme gönder |
| GET | `/api/v1/node-servers/:id/waf/rules` | Yönetici | Düğüm WAF kural listesi |
| DELETE | `/api/v1/node-servers/:id/waf/rules/:name` | Yönetici | Düğüm WAF kurallarını sil |
| GET | `/api/v1/node-servers/:id/waf/rules/:name` | Yönetici | Düğüm WAF kural ayrıntıları |
| PUT | `/api/v1/node-servers/:id/waf/rules/:name` | Yönetici | Düğüm WAF kurallarını kaydet |
| GET | `/api/v1/node-servers/:id/waf/status` | Yönetici | Düğüm WAF durumu |
| PUT | `/api/v1/node-servers/:id/waf/status` | Yönetici | WAF düğümünü değiştir |
| POST | `/api/v1/node-servers/:id/waf/sync` | Yönetici | Bu düğümle senkronize et |
| GET | `/api/v1/node-servers/:id/warp` | Yönetici | Liste |
| DELETE | `/api/v1/node-servers/:id/warp/:warpId` | Yönetici | Sil |
| GET | `/api/v1/node-servers/:id/warp/config` | Yönetici | Yapılandırmayı oku |
| POST | `/api/v1/node-servers/:id/warp/generate-config` | Yönetici | Yapılandırma oluştur |
| POST | `/api/v1/node-servers/:id/warp/register` | Yönetici | Kayıt ol |
| GET | `/api/v1/node-servers/:id/warp/status` | Yönetici | Durum |
| GET | `/api/v1/node-servers/:id/xray/control/balancer/:tag` | Yönetici | Dengeleyici bilgilerini alın |
| POST | `/api/v1/node-servers/:id/xray/control/balancer/:tag` | Yönetici | Dengeleyici hedefini ayarla |
| POST | `/api/v1/node-servers/:id/xray/control/firewall/apply` | Yönetici | Bağlantı noktası atlamalı güvenlik duvarı kurallarını uygulama |
| POST | `/api/v1/node-servers/:id/xray/control/firewall/remove` | Yönetici | Bağlantı noktası atlamalı güvenlik duvarı kurallarını kaldırın |
| POST | `/api/v1/node-servers/:id/xray/control/inbound` | Yönetici | Gelen ekle |
| DELETE | `/api/v1/node-servers/:id/xray/control/inbound/:tag` | Yönetici | Gelenleri kaldır |
| POST | `/api/v1/node-servers/:id/xray/control/inbound/:tag/user` | Yönetici | Kullanıcı ekle |
| DELETE | `/api/v1/node-servers/:id/xray/control/inbound/:tag/user/:email` | Yönetici | Kullanıcıyı kaldır |
| POST | `/api/v1/node-servers/:id/xray/control/outbound` | Yönetici | Giden ekle |
| DELETE | `/api/v1/node-servers/:id/xray/control/outbound/:tag` | Yönetici | Giden postayı kaldır |
| POST | `/api/v1/node-servers/:id/xray/control/routing` | Yönetici | Rota iste |
| POST | `/api/v1/node-servers/:id/xray/control/routing/rule` | Yönetici | Yönlendirme kuralı ekle |
| DELETE | `/api/v1/node-servers/:id/xray/control/routing/rule/:tag` | Yönetici | Yönlendirme kuralını kaldır |
| GET | `/api/v1/node-servers/:id/xray/control/routing/rules` | Yönetici | Yönlendirme kurallarını listeleyin |
| POST | `/api/v1/node-servers/:id/xray/control/sync` | Yönetici | xray yapılandırma senkronizasyonunu manuel olarak tetikleyin |
| GET | `/api/v1/node-servers/:id/xray/stats` | Yönetici | İstatistikleri al |
| POST | `/api/v1/node-servers/:id/xray/stats/reset/:email` | Yönetici | Kullanıcı istatistiklerini sıfırla |
| GET | `/api/v1/node-servers/:id/xray/stats/users` | Yönetici | Kullanıcı istatistiklerini alın |
| GET | `/api/v1/node-servers/billable` | Yönetici | Faturalandırılabilir düğüm listesi |
| GET | `/api/v1/node-servers/ca-cert` | Yönetici | CA sertifikası |
| GET | `/api/v1/node-servers/ca-status` | Yönetici | CA durumu |
| GET | `/api/v1/node-servers/dashboard` | Yönetici | Kontrol paneli durumu |
| POST | `/api/v1/node-servers/distribute-cert` | Yönetici | Sertifikayı tüm düğümlere dağıtın |
| POST | `/api/v1/node-servers/failover-check` | Yönetici | Yük devretme denetimini tetikleyin |
| GET | `/api/v1/node-servers/metrics/aggregated` | Yönetici | Toplu metrikler |
| POST | `/api/v1/node-servers/migrate-cert` | Yönetici | Sertifikayı taşı |
| GET | `/api/v1/node-servers/online-summary` | Yönetici | Çevrimiçi özet |
| GET | `/api/v1/node-servers/online-users` | Yönetici | Tüm çevrimiçi kullanıcılar |
| GET | `/api/v1/node-servers/pending-agent-updates` | Yönetici | Yükseltmeyi bekleyen aracılar |
| POST | `/api/v1/node-servers/recommend-region` | Yönetici | Önerilen bölgeler |
| POST | `/api/v1/node-servers/select-nodes` | Yönetici | Abonelik için düğümleri seçin |
| POST | `/api/v1/node-servers/speed-test-all` | Yönetici | Tüm düğümlerde hız testi yapın |
| GET | `/api/v1/node-servers/speed-test-results` | Yönetici | Hız testi sonuçları |
| GET | `/api/v1/node-servers/traffic/stats` | Yönetici | Trafik istatistikleri |
| POST | `/api/v1/node-servers/update-all-agents` | Yönetici | Tüm Temsilcileri Yükselt |
| DELETE | `/api/v1/node-servers/users/:client_id` | Yönetici | Bir müşteriyi senkronize ederek silin |
| GET | `/api/v1/node-servers/users/retry-queue` | Yönetici | Sıra durumunu yeniden dene |
| POST | `/api/v1/node-servers/users/retry-queue/process` | Yönetici | İşlem yeniden deneme kuyruğu |
| POST | `/api/v1/node-servers/users/sync` | Yönetici | Tek bir müşteriyi senkronize edin |
| POST | `/api/v1/node-servers/validate-chain-config` | Yönetici | Zincirleme yapılandırmayı doğrula |

### Düğüm denetim günlüğü

Düğüm tarafı operasyon denetimi ve dışa aktarımı.

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| GET | `/api/v1/node-audit-logs` | Oturum açıldı | Düğüm denetim günlüğü |
| GET | `/api/v1/node-audit-logs/export` | Oturum açıldı | Düğüm denetim günlüklerini dışa aktar |

### Düğümler (eski model, kullanımdan kaldırıldı)

> `/api/v1/nodes` CRUD, 2026-10-20 tarihinde kullanımdan kaldırıldı: düğümler `/api/v1/node-servers` aracılığıyla eşit şekilde yönetiliyor.
> Çevrimiçi kullanıcı sorguları `/api/v1/node-servers/{online-users,online-summary,:id/online-users}`'de kalır.

### Bölgeye genel bakış

Bölgeye göre toplanan düğüm görünümü.

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| GET | `/api/v1/regions/:region/servers` | Oturum açıldı | Bu bölge için düğüm listesi |
| GET | `/api/v1/regions/overview` | Oturum açıldı | Bölgeye genel bakış |

### Abonelikler

Abonelik kaydı yönetimi: iptal etme, jetonu döndürme, günlüklere erişim, kullanım sıfırlama.

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| GET | `/api/v1/subscriptions` | Oturum açıldı | Liste |
| POST | `/api/v1/subscriptions` | Oturum açıldı | Oluştur |
| DELETE | `/api/v1/subscriptions/:id` | Oturum açıldı | Sil |
| GET | `/api/v1/subscriptions/:id` | Oturum açıldı | Ayrıntılar |
| PUT | `/api/v1/subscriptions/:id` | Oturum açıldı | Güncelleme |
| GET | `/api/v1/subscriptions/:id/access-logs` | Oturum açıldı | Erişim günlüğü |
| POST | `/api/v1/subscriptions/:id/enable` | Oturum açıldı | Etkinleştir |
| POST | `/api/v1/subscriptions/:id/reset-usage` | Oturum açıldı | Kullanımı sıfırla |
| POST | `/api/v1/subscriptions/:id/restore` | Oturum açıldı | Devam et |
| POST | `/api/v1/subscriptions/:id/revoke` | Oturum açıldı | İptal et |
| POST | `/api/v1/subscriptions/:id/rotate` | Oturum açıldı | Döndür |
| GET | `/api/v1/subscriptions/:id/rotations` | Oturum açıldı | Rotasyon geçmişi |

### Abonelik formatları

> Yönetici tarafı `/api/v1/subscription-formats` CRUD, 2026-10-20 tarihinde kullanımdan kaldırıldı: gerçek giriş noktası
> `/api/v1/mihomo/configs` (abonelik formatı merkezi). `subscription_formats` tablosu kalır,
> yalnızca genel `/api/v1/public/client-configs` uç noktası için çekirdek verileri olarak kullanılır.

### Abonelik uç noktaları

Özel abonelik etki alanları (panelin `/sub` yoluna ters proxy uygulanır).

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| GET | `/api/v1/subscription-endpoints` | Oturum açıldı | Liste |
| POST | `/api/v1/subscription-endpoints` | Oturum açıldı | Oluştur |
| DELETE | `/api/v1/subscription-endpoints/:id` | Oturum açıldı | Sil |
| GET | `/api/v1/subscription-endpoints/:id` | Oturum açıldı | Ayrıntılar |
| PUT | `/api/v1/subscription-endpoints/:id` | Oturum açıldı | Güncelleme |

### Alanları paylaş

İstemci bağlantılarında kullanılan etki alanı/IP havuzuna erişim.

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| GET | `/api/v1/share-domains` | Oturum açıldı | Liste |
| POST | `/api/v1/share-domains` | Oturum açıldı | Oluştur |
| DELETE | `/api/v1/share-domains/:id` | Oturum açıldı | Sil |
| PUT | `/api/v1/share-domains/:id` | Oturum açıldı | Güncelleme |
| DELETE | `/api/v1/share-domains/all` | Oturum açıldı | Tümünü temizle |
| POST | `/api/v1/share-domains/seed` | Oturum açıldı | Varsayılan verileri başlat |

### Cloudflare IP

Cloudflare CDN IP havuzu ve kaynak yönetimi.

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| DELETE | `/api/v1/cfip/entries` | Oturum açıldı | Girişleri temizle |
| GET | `/api/v1/cfip/entries` | Oturum açıldı | Giriş listesi |
| POST | `/api/v1/cfip/entries` | Oturum açıldı | Giriş ekle |
| DELETE | `/api/v1/cfip/entries/:id` | Oturum açıldı | Girişi sil |
| POST | `/api/v1/cfip/fetch-all` | Oturum açıldı | Tüm kaynakları getir |
| POST | `/api/v1/cfip/seed` | Oturum açıldı | Varsayılan verileri başlat |
| GET | `/api/v1/cfip/sources` | Oturum açıldı | Veri kaynağı listesi |
| POST | `/api/v1/cfip/sources` | Oturum açıldı | Veri kaynağı oluştur |
| DELETE | `/api/v1/cfip/sources/:id` | Oturum açıldı | Veri kaynağını sil |
| PUT | `/api/v1/cfip/sources/:id` | Oturum açıldı | Veri kaynağını güncelle |
| POST | `/api/v1/cfip/sources/:id/fetch` | Oturum açıldı | Bu kaynağı getir |

### Abonelik yapılandırması (Mihomo/Clash)

Abonelik şablonu yapılandırması: gruplar, kurallar, kural sağlayıcılar, YAML önizlemesi.

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| GET | `/api/v1/mihomo/configs` | Oturum açıldı | Liste |
| POST | `/api/v1/mihomo/configs` | Oturum açıldı | Oluştur |
| DELETE | `/api/v1/mihomo/configs/:id` | Oturum açıldı | Sil |
| GET | `/api/v1/mihomo/configs/:id` | Oturum açıldı | Ayrıntılar |
| PUT | `/api/v1/mihomo/configs/:id` | Oturum açıldı | Güncelleme |
| GET | `/api/v1/mihomo/configs/:id/proxy-groups` | Oturum açıldı | Grup listesi |
| POST | `/api/v1/mihomo/configs/:id/proxy-groups` | Oturum açıldı | Grup oluştur |
| GET | `/api/v1/mihomo/configs/:id/rule-providers` | Oturum açıldı | Kural sağlayıcı listesi |
| POST | `/api/v1/mihomo/configs/:id/rule-providers` | Oturum açıldı | Kural sağlayıcı oluştur |
| GET | `/api/v1/mihomo/configs/:id/rules` | Oturum açıldı | Kural listesi |
| POST | `/api/v1/mihomo/configs/:id/rules` | Oturum açıldı | Kural oluştur |
| GET | `/api/v1/mihomo/configs/:id/yaml` | Oturum açıldı | Önizleme oluşturuldu YAML |
| DELETE | `/api/v1/mihomo/proxy-groups/:id` | Oturum açıldı | Grubu sil |
| DELETE | `/api/v1/mihomo/rule-providers/:id` | Oturum açıldı | Kural sağlayıcıyı sil |
| DELETE | `/api/v1/mihomo/rules/:id` | Oturum açıldı | Kuralı sil |

### Siteleri şarj edin

Üçüncü taraf ödeme sitesi entegrasyonu: imzalama anahtarları, doğrulama dosyaları, alan adı doğrulama, entegrasyon belgeleri.

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| GET | `/api/v1/charge-sites` | Oturum açıldı | Site listesi |
| POST | `/api/v1/charge-sites` | Oturum açıldı | Site oluştur |
| DELETE | `/api/v1/charge-sites/:id` | Oturum açıldı | Siteyi sil |
| GET | `/api/v1/charge-sites/:id` | Oturum açıldı | Site ayrıntıları |
| PUT | `/api/v1/charge-sites/:id` | Oturum açıldı | Siteyi güncelle |
| POST | `/api/v1/charge-sites/:id/clear-prev-secret` | Oturum açıldı | Eski anahtarı temizle |
| POST | `/api/v1/charge-sites/:id/generate-verify` | Oturum açıldı | Doğrulama dosyası oluştur |
| POST | `/api/v1/charge-sites/:id/generate-verify-file` | Oturum açıldı | Doğrulama dosyası oluştur |
| GET | `/api/v1/charge-sites/:id/integration-doc` | Oturum açıldı | Entegrasyon belgesini alın |
| POST | `/api/v1/charge-sites/:id/rotate-secret` | Oturum açıldı | İmzalama anahtarını döndür |
| POST | `/api/v1/charge-sites/:id/verify-dns` | Oturum açıldı | DNS alan adını doğrulayın |
| POST | `/api/v1/charge-sites/:id/verify-domain` | Oturum açıldı | Etki alanı HTTP sorgulamasını doğrulayın |
| POST | `/api/v1/charge-sites/:id/verify-domain-dns` | Oturum açıldı | DNS alan adını doğrulayın |
| POST | `/api/v1/charge-sites/:id/verify-domain-http` | Oturum açıldı | Etki alanı HTTP sorgulamasını doğrulayın |

### Siparişleri yeniden doldur

Sipariş sorgularını, geri ödemeleri ve CSV ihracatını yeniden doldurun.

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| GET | `/api/v1/charge-orders` | Oturum açıldı | Sipariş listesi |
| GET | `/api/v1/charge-orders/:id` | Oturum açıldı | Sipariş ayrıntıları |
| POST | `/api/v1/charge-orders/:id/refund` | Oturum açıldı | Geri ödeme |
| GET | `/api/v1/charge-orders/export-csv` | Oturum açıldı | Yükleme siparişlerini dışa aktar CSV |

### Kartları şarj et

Toplu kart oluşturma, kullanım kayıtları, CSV dışa aktarma.

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| GET | `/api/v1/charge-cards` | Oturum açıldı | Kart listesi |
| POST | `/api/v1/charge-cards` | Oturum açıldı | Kart oluştur |
| DELETE | `/api/v1/charge-cards/:id` | Oturum açıldı | Kartı sil |
| GET | `/api/v1/charge-cards/:id/usages` | Oturum açıldı | Kart kullanım kayıtları |
| POST | `/api/v1/charge-cards/batch` | Oturum açıldı | Toplu kart oluşturma |
| GET | `/api/v1/charge-cards/export-csv` | Oturum açıldı | Kartları dışa aktarma CSV |

### Geri arama olaylarını ücretlendir

Geri arama olayı denetimi.

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| GET | `/api/v1/charge-callback-events` | Oturum açıldı | Geri arama olay listesi |

### Geri arama denemelerini ücretlendir

Başarısız geri aramaların manuel olarak yeniden denenmesi.

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| GET | `/api/v1/charge-callback-retries` | Oturum açıldı | Başarısız geri arama listesi |
| POST | `/api/v1/charge-callback-retries/:id/retry` | Yönetici | Geri aramayı yeniden dene |

### Müşteri bakiyesi

Bakiyelerin ve işlemlerin yönetici görünümü.

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| GET | `/api/v1/client-balances` | Oturum açıldı | Müşteri bakiye listesi |
| GET | `/api/v1/client-balances/transactions` | Oturum açıldı | Bakiye işlemleri (yönetici) |
| GET | `/api/v1/client-balances/transactions/export-csv` | Oturum açıldı | İhracat bakiyesi işlemleri CSV |

### Hediyeler

Talep edilebilir hediyeler verin (günler/trafik).

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| GET | `/api/v1/client-gifts` | Oturum açıldı | Hediye listesi |
| POST | `/api/v1/client-gifts` | Oturum açıldı | Hediye oluştur |
| POST | `/api/v1/client-gifts/batch` | Oturum açıldı | Toplu olarak hediyeler oluşturun |
| GET | `/api/v1/client-gifts/search-clients` | Oturum açıldı | Müşterileri arayın |

### Yönlendirme yapılandırması

Yönlendirme komisyonu yapılandırması ve istatistikleri.

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| GET | `/api/v1/referral/config` | Oturum açıldı | Yönlendirme yapılandırması |
| PUT | `/api/v1/referral/config` | Yönetici | Yönlendirme yapılandırmasını kaydet |
| GET | `/api/v1/referral/records` | Oturum açıldı | Yönlendirme kayıt listesi |
| GET | `/api/v1/referral/stats` | Oturum açıldı | Yönlendirme istatistikleri |

### Yönlendirmenin geri çekilmesi

Para çekme isteklerini inceleyin.

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| GET | `/api/v1/referral-withdrawals` | Oturum açıldı | Para çekme talebi listesi |
| POST | `/api/v1/referral-withdrawals/:id/approve` | Oturum açıldı | Para çekmeyi onayla |
| POST | `/api/v1/referral-withdrawals/:id/reject` | Oturum açıldı | Para çekmeyi reddet |

### IP izin verilenler listeleri (özet)

Tüm müşteriler genelinde IP izin verilenler listesi görünümü.

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| GET | `/api/v1/ip-whitelists` | Oturum açıldı | Tüm müşteri IP'sine izin verilenler listeleri |

### Müşteri OAuth bağlamaları

Müşterinin üçüncü taraf hesap bağlamalarını sorgulayın.

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| GET | `/api/v1/client-oauth-bindings` | Oturum açıldı | Müşteri OAuth bağlama listesi |

### Biletler (yönetici)

Bilet yanıtı, atama ve kapatma.

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| GET | `/api/v1/tickets` | Oturum açıldı | Bilet listesi |
| GET | `/api/v1/tickets/:id` | Oturum açıldı | Bilet ayrıntıları |
| POST | `/api/v1/tickets/:id/assign` | Oturum açıldı | Bilet ata |
| POST | `/api/v1/tickets/:id/close` | Oturum açıldı | Bileti kapat |
| POST | `/api/v1/tickets/:id/reply` | Oturum açıldı | Bilete yanıt ver |

### Makaleler

Portal makaleleri için CRUD.

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| GET | `/api/v1/articles` | Oturum açıldı | Makale listesi |
| POST | `/api/v1/articles` | Oturum açıldı | Makale oluştur |
| DELETE | `/api/v1/articles/:id` | Oturum açıldı | Makaleyi sil |
| GET | `/api/v1/articles/:id` | Oturum açıldı | Makale ayrıntıları |
| PUT | `/api/v1/articles/:id` | Oturum açıldı | Makaleyi güncelle |

### Paylaşılan hesaplar

Paylaşılan hesaplar (akış vb.) için kayıtların düzenlenmesi ve onarılması.

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| GET | `/api/v1/shared-accounts` | Oturum açıldı | Paylaşılan hesap listesi |
| POST | `/api/v1/shared-accounts` | Oturum açıldı | Paylaşılan hesap oluştur |
| DELETE | `/api/v1/shared-accounts/:id` | Oturum açıldı | Paylaşılan hesabı sil |
| GET | `/api/v1/shared-accounts/:id` | Oturum açıldı | Paylaşılan hesap ayrıntıları |
| PUT | `/api/v1/shared-accounts/:id` | Oturum açıldı | Paylaşılan hesabı güncelle |
| GET | `/api/v1/shared-accounts/:id/repair-logs` | Oturum açıldı | Kayıtları onar |

### Sertifikalar

Kamu sertifikası (ACME) verilmesi, yüklenmesi, yenilenmesi ve dışa aktarılması; belirli bir düğümde düzenleme desteklenir.

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| GET | `/api/v1/certificates` | Oturum açıldı | Liste |
| POST | `/api/v1/certificates` | Oturum açıldı | Oluştur |
| DELETE | `/api/v1/certificates/:id` | Oturum açıldı | Sil |
| GET | `/api/v1/certificates/:id` | Oturum açıldı | Ayrıntılar |
| PUT | `/api/v1/certificates/:id` | Oturum açıldı | Güncelleme |
| GET | `/api/v1/certificates/:id/download` | Oturum açıldı | İndir |
| POST | `/api/v1/certificates/:id/renew` | Oturum açıldı | Müşteriyi yenile |
| GET | `/api/v1/certificates/check-port` | Oturum açıldı | Bağlantı noktasını kontrol et |
| GET | `/api/v1/certificates/detect-ip` | Oturum açıldı | Prob IP'si |
| GET | `/api/v1/certificates/export` | Oturum açıldı | İhracat |
| POST | `/api/v1/certificates/issue` | Oturum açıldı | Sayı. gövde: `{"domains":["example.com","*.example.com"],"challenge_type":"dns-01","dns_channel_id":2,"key_type":"rsa","key_bits":4096}`. `challenge_type` varsayılan olarak http-01'e ayarlanır; `dns_channel_id`, bir DNS kanalını seçer (yapılandırılmış bir DNS sağlayıcı + kimlik bilgileri) |
| POST | `/api/v1/certificates/issue-on-node` | Oturum açıldı | Düğüme sorun |
| POST | `/api/v1/certificates/upload` | Oturum açıldı | Yükle |

### Sertifika kanalları

ACME kanal (CA) yapılandırması.

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| GET | `/api/v1/cert-channels` | Oturum açıldı | Liste |
| POST | `/api/v1/cert-channels` | Oturum açıldı | Oluştur |
| DELETE | `/api/v1/cert-channels/:id` | Oturum açıldı | Sil |
| PUT | `/api/v1/cert-channels/:id` | Oturum açıldı | Güncelleme |
| GET | `/api/v1/cert-channels/providers` | Oturum açıldı | sağlayıcı |

### EAB yapılandırması

ZeroSSL / Google gibi harici hesapları bağlayan kimlik bilgileri.

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| GET | `/api/v1/eab-configs` | Oturum açıldı | Liste |
| POST | `/api/v1/eab-configs` | Oturum açıldı | Oluştur |
| DELETE | `/api/v1/eab-configs/:id` | Oturum açıldı | Sil |
| PUT | `/api/v1/eab-configs/:id` | Oturum açıldı | Güncelleme |
| POST | `/api/v1/eab-configs/fetch-google` | Oturum açıldı | GCP'den Google EAB'yi getir |
| POST | `/api/v1/eab-configs/fetch-zerossl` | Oturum açıldı | Tek tıklamayla ZeroSSL EAB'yi e-postayla alın |

### ACME hesapları

Panel tarafı ACME hesabı (iletişim e-postası) yönetimi.

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| GET | `/api/v1/acme-accounts` | Oturum açıldı | Liste |
| POST | `/api/v1/acme-accounts` | Oturum açıldı | Oluştur |
| DELETE | `/api/v1/acme-accounts/:id` | Oturum açıldı | Sil |
| PUT | `/api/v1/acme-accounts/:id` | Oturum açıldı | Güncelleme |
| GET | `/api/v1/acme-accounts/default` | Oturum açıldı | Varsayılanları al |

### Dahili CA

Dahili panel CA: Aracı sertifikası verme/yenileme/iptal etme, CRL dışa aktarma, dağıtım kuyruğu.

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| GET | `/api/v1/ca/agent-certs` | Oturum açıldı | Temsilci sertifika listesi |
| POST | `/api/v1/ca/agent-certs/:server_id/renew` | Oturum açıldı | Temsilci sertifikasını yenileyin |
| GET | `/api/v1/ca/distribution-queue` | Oturum açıldı | Sertifika dağıtım kuyruğu |
| DELETE | `/api/v1/ca/distribution-queue/:id` | Oturum açıldı | Dağıtım kuyruğu girişini sil |
| POST | `/api/v1/ca/distribution-queue/:id/retry` | Oturum açıldı | Sertifika dağıtımını yeniden dene |
| GET | `/api/v1/ca/revocations` | Oturum açıldı | İptal listesi |
| GET | `/api/v1/ca/revocations/export` | Oturum açıldı | CRL'yi dışa aktar |
| GET | `/api/v1/ca/status` | Oturum açıldı | CA durumu |

### Coğrafi kurallar

Geoip/jeosit veri kaynaklarının ve dosyalarının senkronizasyonu ve teslimi.

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| GET | `/api/v1/geo-rules/files` | Oturum açıldı | Dosya listesi |
| DELETE | `/api/v1/geo-rules/files/:fileId` | Oturum açıldı | Dosyayı sil |
| PUT | `/api/v1/geo-rules/files/:fileId/auto-sync` | Oturum açıldı | Otomatik senkronizasyonu aç/kapat |
| POST | `/api/v1/geo-rules/files/:fileId/download` | Oturum açıldı | İndir |
| GET | `/api/v1/geo-rules/files/:fileId/download-file` | Oturum açıldı | Dosyayı indir |
| GET | `/api/v1/geo-rules/github-assets` | Oturum açıldı | GitHub deposu varlıklarını getir |
| POST | `/api/v1/geo-rules/push-all` | Oturum açıldı | Tüm düğümlere gönder |
| POST | `/api/v1/geo-rules/seed` | Oturum açıldı | Varsayılan verileri başlat |
| GET | `/api/v1/geo-rules/sources` | Oturum açıldı | Veri kaynağı listesi |
| POST | `/api/v1/geo-rules/sources` | Oturum açıldı | Veri kaynağı oluştur |
| DELETE | `/api/v1/geo-rules/sources/:sourceId` | Oturum açıldı | Veri kaynağını sil |
| PUT | `/api/v1/geo-rules/sources/:sourceId` | Oturum açıldı | Veri kaynağını güncelle |
| GET | `/api/v1/geo-rules/sources/:sourceId/assets` | Oturum açıldı | Sürüm varlıklarını getir |
| POST | `/api/v1/geo-rules/sources/:sourceId/files` | Oturum açıldı | Dosya ekle |
| PUT | `/api/v1/geo-rules/sources/:sourceId/schedule` | Oturum açıldı | Planlanmış senkronizasyonu değiştir |
| POST | `/api/v1/geo-rules/sources/:sourceId/select` | Oturum açıldı | Dosya seç |
| POST | `/api/v1/geo-rules/sources/:sourceId/sync` | Oturum açıldı | Bu veri kaynağını senkronize edin |
| GET | `/api/v1/geo-rules/status` | Oturum açıldı | Durum |
| POST | `/api/v1/geo-rules/sync-all` | Oturum açıldı | Tümünü senkronize et |
| GET | `/api/v1/geo-rules/sync-status/:key` | Oturum açıldı | Senkronizasyon ilerlemesi |

### WAF kural kitaplığı

Genel WAF kuralları: GitHub kaynağı, senkronizasyonu, tüm düğümlere teslimatı.

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| GET | `/api/v1/waf/github-source` | Oturum açıldı | GitHub veri kaynağı |
| PUT | `/api/v1/waf/github-source` | Oturum açıldı | GitHub veri kaynağını kaydet |
| POST | `/api/v1/waf/push-all` | Oturum açıldı | Tüm düğümlere gönder |
| GET | `/api/v1/waf/rules` | Oturum açıldı | Kural listesi |
| DELETE | `/api/v1/waf/rules/:name` | Oturum açıldı | Kuralı sil |
| GET | `/api/v1/waf/rules/:name` | Oturum açıldı | Kuralı al |
| PUT | `/api/v1/waf/rules/:name` | Oturum açıldı | Kuralı kaydet |
| POST | `/api/v1/waf/sync-all` | Oturum açıldı | Tümünü senkronize et |
| POST | `/api/v1/waf/sync-github` | Oturum açıldı | GitHub'den senkronizasyon |

### Günlük yönetimi

Düğüm/çekirdek günlük okuma ve akışı, temizleme politikaları, günlük yazıcı kontrolü.

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| POST | `/api/v1/log-manage/cleanup-all` | Oturum açıldı | Şimdi her şeyi temizle |
| GET | `/api/v1/log-manage/cleanup-rules` | Oturum açıldı | Temizleme kuralı listesi |
| PUT | `/api/v1/log-manage/cleanup-rules/:id` | Oturum açıldı | Temizleme kuralını güncelle |
| POST | `/api/v1/log-manage/cleanup-table/:table` | Oturum açıldı | Belirtilen tabloyu şimdi temizleyin |
| GET | `/api/v1/log-manage/core-log-output` | Oturum açıldı | Çekirdek günlük çıkışı yapılandırması |
| PUT | `/api/v1/log-manage/core-log-output` | Oturum açıldı | Çekirdek günlük çıkışı yapılandırmasını kaydet |
| GET | `/api/v1/log-manage/core/:type/stdout-log` | Oturum açıldı | Çekirdek stdout günlüklerini okuyun |
| GET | `/api/v1/log-manage/core/:type/stdout-stream` | Oturum açıldı | Çekirdek stdout akışı |
| POST | `/api/v1/log-manage/discover` | Oturum açıldı | Düğüm günlük dosyalarını keşfedin |
| GET | `/api/v1/log-manage/log-output` | Oturum açıldı | Günlük çıkışı yapılandırması |
| PUT | `/api/v1/log-manage/log-output` | Oturum açıldı | Günlük çıkışı yapılandırmasını kaydet |
| POST | `/api/v1/log-manage/log-output/release/:type` | Oturum açıldı | Günlük stdout'un devralınmasını serbest bırakın |
| POST | `/api/v1/log-manage/log-output/takeover/:type` | Oturum açıldı | Log stdout'u devralın |
| GET | `/api/v1/log-manage/nodes/:serverId/logs` | Oturum açıldı | Düğüm günlük dosyası listesi |
| GET | `/api/v1/log-manage/nodes/:serverId/logs/:type` | Oturum açıldı | Düğüm günlüklerini okuyun |
| POST | `/api/v1/log-manage/nodes/:serverId/logs/:type/rotate` | Oturum açıldı | Düğüm günlüklerini döndür |
| GET | `/api/v1/log-manage/nodes/:serverId/logs/:type/stream` | Oturum açıldı | Düğüm günlüklerini akışla |
| GET | `/api/v1/log-manage/policies` | Oturum açıldı | Politika listesi |
| POST | `/api/v1/log-manage/policies` | Oturum açıldı | Politika oluştur |
| DELETE | `/api/v1/log-manage/policies/:id` | Oturum açıldı | Politikayı sil |
| GET | `/api/v1/log-manage/policies/:id` | Oturum açıldı | Politika ayrıntıları |
| PUT | `/api/v1/log-manage/policies/:id` | Oturum açıldı | Politikayı güncelle |
| GET | `/api/v1/log-manage/policies/:id/jobs` | Oturum açıldı | Temizleme işi listesi |
| GET | `/api/v1/log-manage/policies/:id/log` | Oturum açıldı | Günlükleri oku |
| GET | `/api/v1/log-manage/policies/:id/log-stream` | Oturum açıldı | Akış günlükleri |
| POST | `/api/v1/log-manage/policies/:id/rotate` | Oturum açıldı | Döndür |
| GET | `/api/v1/log-manage/proxy/:type/stdout-log` | Oturum açıldı | Çekirdek stdout günlüklerini okuyun |
| GET | `/api/v1/log-manage/proxy/:type/stdout-stream` | Oturum açıldı | Çekirdek stdout akışı |
| GET | `/api/v1/log-manage/scheduler-config` | Oturum açıldı | Planlama yapılandırması |
| PUT | `/api/v1/log-manage/scheduler-config` | Oturum açıldı | Planlama yapılandırmasını güncelle |
| GET | `/api/v1/log-manage/status` | Oturum açıldı | Durum |
| POST | `/api/v1/log-manage/sync` | Oturum açıldı | Temizleme politikalarını senkronize et |
| GET | `/api/v1/log-manage/writers` | Oturum açıldı | Yazar durumu |
| PUT | `/api/v1/log-manage/writers/:type/config` | Oturum açıldı | Yazar yapılandırmasını güncelle |
| GET | `/api/v1/log-manage/writers/:type/files` | Oturum açıldı | Yazar günlük dosyası listesi |
| POST | `/api/v1/log-manage/writers/:type/start` | Oturum açıldı | Yazarı başlat |
| POST | `/api/v1/log-manage/writers/:type/stop` | Oturum açıldı | Yazarı durdur |

### Yedeklemeler

Veritabanı/tam yedeklemeler, geri yükleme, planlanmış işler, FTP/S3/WebDAV uzaktan depolama.

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| DELETE | `/api/v1/backups/:id` | Yönetici | Yedeği sil |
| GET | `/api/v1/backups/:id/download` | Yönetici | Yedeği indir |
| POST | `/api/v1/backups/:id/restore` | Yönetici | Yedeklemeden geri yükle |
| POST | `/api/v1/backups/:id/upload-ftp` | Yönetici | FTP'ye yükle |
| POST | `/api/v1/backups/:id/upload-s3` | Yönetici | S3'ye yükle |
| POST | `/api/v1/backups/:id/upload-webdav` | Yönetici | WebDAV'ye yükle |
| POST | `/api/v1/backups/create` | Yönetici | Tam yedekleme oluştur (DB dökümüyle) |
| GET | `/api/v1/backups/list` | Yönetici | Tam yedekleme listesi |
| POST | `/api/v1/backups/remote-restore` | Yönetici | Uzaktan geri yükleme |
| GET | `/api/v1/backups/restore-status` | Yönetici | İlerlemeyi geri yükle |
| GET | `/api/v1/backups/schedules` | Yönetici | Zamanlanmış iş listesi |
| POST | `/api/v1/backups/schedules` | Yönetici | Zamanlanmış iş oluştur |
| DELETE | `/api/v1/backups/schedules/:id` | Yönetici | Planlanan işi sil |
| PUT | `/api/v1/backups/schedules/:id` | Yönetici | Planlanan işi güncelle |
| GET | `/api/v1/backups/storages` | Yönetici | Depolama yapılandırma listesi |
| POST | `/api/v1/backups/storages` | Yönetici | Depolama yapılandırması oluştur |
| DELETE | `/api/v1/backups/storages/:id` | Yönetici | Depolama yapılandırmasını sil |
| PUT | `/api/v1/backups/storages/:id` | Yönetici | Depolama yapılandırmasını güncelleyin |
| GET | `/api/v1/backups/storages/:id/files` | Yönetici | Uzaktan yedekleme dosya listesi |
| POST | `/api/v1/backups/upload-restore` | Yönetici | Yedeklemeyi yükleyin ve geri yükleyin |

### Sistem ayarları

Veritabanı yapılandırması ve geçişi, panel ayarları, JWT anahtarları, SMTP/Telegram testleri, bildirim şablonları, çevrimiçi yükseltmeler (çoğunlukla yönetici).

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| PUT | `/api/v1/system/bootstrap-finish` | Yönetici | İlk önyükleme kurulumunu tamamlayın |
| GET | `/api/v1/system/bootstrap-state` | Yönetici | İlk önyükleme durumu |
| GET | `/api/v1/system/config-file/:coreType` | Yönetici | Çekirdek yapılandırma dosyasını okuyun |
| GET | `/api/v1/system/db-config` | Yönetici | Veritabanı yapılandırması |
| PUT | `/api/v1/system/db-config` | Yönetici | Veritabanı yapılandırmasını güncelle |
| POST | `/api/v1/system/db-config/migrate` | Yönetici | Veritabanı verilerini taşı |
| POST | `/api/v1/system/db-config/switch` | Yönetici | Veritabanını değiştir (tek adımlı geçişle) |
| POST | `/api/v1/system/db-config/test` | Yönetici | Veritabanı bağlantısını test edin |
| GET | `/api/v1/system/db-pool-stats` | Yönetici | Bağlantı havuzu istatistikleri |
| GET | `/api/v1/system/db-stats` | Yönetici | Veritabanı istatistikleri |
| GET | `/api/v1/system/db-stats/ping` | Yönetici | Veritabanı araştırması |
| POST | `/api/v1/system/db-stats/reset` | Yönetici | Veritabanı istatistiklerini sıfırla |
| GET | `/api/v1/system/encryption-key-status` | Yönetici | Şifreleme anahtarı durumu |
| GET | `/api/v1/system/env-config` | Yönetici | .env yapılandırması |
| PUT | `/api/v1/system/env-config` | Yönetici | .env yapılandırmasını güncelleyin |
| POST | `/api/v1/system/generate-encryption-key` | Yönetici | Şifreleme anahtarı oluştur |
| GET | `/api/v1/system/github-config` | Yönetici | GitHub yapılandırması |
| PUT | `/api/v1/system/github-config` | Yönetici | GitHub yapılandırmasını güncelleyin |
| GET | `/api/v1/system/jwt-secret-status` | Yönetici | JWT anahtar durumu |
| GET | `/api/v1/system/maintenance` | halka açık | Bakım modu durumu (genel) |
| PUT | `/api/v1/system/maintenance` | Yönetici | Bakım modunu değiştir |
| POST | `/api/v1/system/migrate-data` | Yönetici | Verileri taşı |
| GET | `/api/v1/system/notification-config` | Yönetici | Bildirim yapılandırması |
| PUT | `/api/v1/system/notification-config` | Yönetici | Bildirim yapılandırmasını güncelle |
| GET | `/api/v1/system/notification-templates` | Yönetici | Şablon listesi |
| POST | `/api/v1/system/notification-templates` | Yönetici | Şablon oluştur |
| DELETE | `/api/v1/system/notification-templates/:id` | Yönetici | Şablonu sil |
| PUT | `/api/v1/system/notification-templates/:id` | Yönetici | Şablonu güncelle |
| POST | `/api/v1/system/notification-templates/:id/broadcast` | Yönetici | Şablona göre yayın bildirimi |
| POST | `/api/v1/system/notification-templates/:id/send` | Yönetici | Test bildirimi gönder |
| GET | `/api/v1/system/panel-config` | Yönetici | Panel yapılandırması |
| PUT | `/api/v1/system/panel-config` | Yönetici | Panel yapılandırmasını güncelle |
| GET | `/api/v1/system/reality-targets` | Yönetici | REALITY hedef listesi |
| PUT | `/api/v1/system/reality-targets` | Yönetici | REALITY hedefini güncelleyin |
| POST | `/api/v1/system/reality-targets/reset` | Yönetici | REALITY hedefini varsayılana sıfırla |
| POST | `/api/v1/system/regenerate-jwt-secret` | Yönetici | JWT anahtarını yeniden oluştur |
| GET | `/api/v1/system/releases` | Yönetici | Mevcut versiyonlar |
| POST | `/api/v1/system/restart` | Yönetici | Yeniden başlat |
| POST | `/api/v1/system/rollback` | Yönetici | Sistem geri alma |
| GET | `/api/v1/system/settings` | Yönetici | Ayarları oku |
| PUT | `/api/v1/system/settings/batch` | Yönetici | Toplu güncelleme ayarları |
| GET | `/api/v1/system/stats` | Yönetici | Sistem durumu |
| GET | `/api/v1/system/subscription-billing-templates` | Yönetici | Abonelik faturalandırma şablonu |
| PUT | `/api/v1/system/subscription-billing-templates` | Yönetici | Abonelik faturalandırma şablonunu güncelleyin |
| POST | `/api/v1/system/telegram-chat-id` | Yönetici | Telegram Sohbet Kimliğini Alın |
| POST | `/api/v1/system/test-smtp` | Yönetici | SMTP'yi test edin |
| POST | `/api/v1/system/test-telegram` | Yönetici | Telegram'yi test edin |
| POST | `/api/v1/system/update` | Yönetici | Sistem yükseltmesi |
| GET | `/api/v1/system/update-jobs` | Yönetici | İş listesini yükselt |
| POST | `/api/v1/system/upload-mtls-ca` | Yönetici | mTLS CA sertifikasını yükleyin |

### Güvenlik denetimi

Olay sorgularını denetleyin ve dışa aktarın.

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| GET | `/api/v1/security/audit-events` | Oturum açıldı | Denetim etkinliği listesi |
| GET | `/api/v1/security/audit-events/export` | Oturum açıldı | Denetim olaylarını dışa aktar |

### Metrikler

Panelde toplu ölçümler (düğümler/sertifikalar/hizmetler).

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| GET | `/api/v1/metrics/certificates` | Oturum açıldı | Sertifika metrikleri |
| GET | `/api/v1/metrics/nodes` | Oturum açıldı | Düğüm metrikleri |
| GET | `/api/v1/metrics/overview` | Oturum açıldı | Genel Bakış |

### Portal şablonları

Portal şablonları, etkinleştirme yapılandırması ve önbellek yönetimi.

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| GET | `/api/v1/portal/active` | Oturum açıldı | Aktif şablon |
| PUT | `/api/v1/portal/active` | Oturum açıldı | Etkin şablonu güncelle |
| DELETE | `/api/v1/portal/cache` | Oturum açıldı | Portal önbelleğini temizle |
| GET | `/api/v1/portal/cache` | Oturum açıldı | Önbellek durumu |
| PUT | `/api/v1/portal/cache/dev-mode` | Oturum açıldı | Önbellek geliştirme modunu değiştir |
| POST | `/api/v1/portal/cache/rebuild` | Oturum açıldı | Portal önbelleğini yeniden oluştur |
| POST | `/api/v1/portal/reset` | Oturum açıldı | Varsayılan şablonu geri yükle |
| GET | `/api/v1/portal/templates` | Oturum açıldı | Liste |
| POST | `/api/v1/portal/templates` | Oturum açıldı | Oluştur |
| DELETE | `/api/v1/portal/templates/:id` | Oturum açıldı | Sil |
| GET | `/api/v1/portal/templates/:id` | Oturum açıldı | Ayrıntılar |
| PUT | `/api/v1/portal/templates/:id` | Oturum açıldı | Güncelleme |
| POST | `/api/v1/portal/templates/:id/activate` | Oturum açıldı | Şablonu etkinleştir |

> `/portal/templates` CRUD **gelişmiş bir API**'dir: portal kullanıcı arayüzü öncelikle `PUT /api/v1/portal/active`'yi kullanır
> (tek seferlik tam site yapılandırması); Bu grupla birden fazla şablonu yönetin/değiştirin. Her ikisi de desteklenir ve çakışmaz.

### Aracı kurulumu

Düğüm Aracısı komut dosyasını yükler ve indirir (genel).

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| POST | `/api/v1/agent/bootstrap` | halka açık | Aracının ilk önyükleme kaydı (Ajan, önyükleme belirteciyle raporlar; genel) |
| GET | `/api/v1/agent/releases` | Yönetici | Mevcut Agent sürümleri (yükseltme sayfası için) |
| GET | `/api/v1/agent/version` | Yönetici | Paneldeki mevcut Agent sürümü |
| GET | `/api/v1/agent/download` | halka açık | Aracıyı İndir |
| GET | `/api/v1/agent/install` | halka açık | Yükleme komut dosyasını al |

### Geri arama girişini ücretlendir

Üçüncü taraf ödeme platformu geri araması (herkese açık, imza doğrulanmış).

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| POST | `/api/v1/charge/callback/:site_id` | halka açık | Ücret geri aramasını yönetin |

### Gelenler (düğümlerle)

Plan düğümü seçimi için kullanılan gelen listesi.

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| GET | `/api/v1/inbounds-with-server` | Oturum açıldı | Tüm gelenler (düğümleri dahil) |

### Genel yapılandırma

Oturum açmadan portal çalışma zamanı yapılandırması (bakım koruması + genel ücret sınırı).

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| GET | `/api/v1/public/articles` | halka açık | Herkese açık makale listesi |
| GET | `/api/v1/public/articles/:slug` | halka açık | Herkese açık makale ayrıntıları |
| GET | `/api/v1/public/client-configs` | halka açık | İstemci indirme yapılandırması (genel) |
| GET | `/api/v1/public/portal-config` | halka açık | Portal çalışma zamanı yapılandırması (site adı, tema, diller, uyumluluk belgeleri; genel) |
| GET | `/api/v1/public/settings` | halka açık | Herkese açık ayarlar (ör., kayıt_enabled; herkese açık) |
| GET | `/api/v1/public/subscription-endpoints` | halka açık | Etkin abonelik uç noktası listesi (genel) |

### İstemci kimlik doğrulaması ve iş (yönetici motorunda aynı ayar)

Portalla aynı istemci uç noktaları; ayrıntılar için `portal-api.en.md`'ye bakın.

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| GET | `/api/v1/client-auth/articles` | halka açık | Makaleleri listele |
| GET | `/api/v1/client-auth/articles/:slug` | halka açık | Makaleyi al |
| GET | `/api/v1/client-auth/balance` | halka açık | Bakiyeyi alın |
| GET | `/api/v1/client-auth/balance/transactions` | halka açık | Bakiye işlemleri |
| GET | `/api/v1/client-auth/bills` | halka açık | Sipariş listesi |
| POST | `/api/v1/client-auth/change-password` | halka açık | Şifreyi değiştir |
| POST | `/api/v1/client-auth/charge/create` | halka açık | Yükleme siparişi oluştur |
| GET | `/api/v1/client-auth/charge/orders` | halka açık | Yeniden yükleme siparişlerini listeleyin |
| GET | `/api/v1/client-auth/charge/orders/:id` | halka açık | Yeniden yükleme siparişi ayrıntılarını alın |
| POST | `/api/v1/client-auth/charge/redeem-card` | halka açık | Kartı kullan |
| GET | `/api/v1/client-auth/charge/sites` | halka açık | Şarj sitelerini listeleyin |
| GET | `/api/v1/client-auth/charge/status/:trade_no` | halka açık | Yeniden yükleme durumunu sorgula |
| GET | `/api/v1/client-auth/checktime` | halka açık | Sunucu saatini alın (saat senkronizasyonu) |
| GET | `/api/v1/client-auth/connection-info` | halka açık | Plan bağlantı parametrelerini alın |
| GET | `/api/v1/client-auth/gifts` | halka açık | Hediyeleri listele |
| POST | `/api/v1/client-auth/gifts/:id/claim` | halka açık | Hediye talep et |
| GET | `/api/v1/client-auth/invoices` | halka açık | Sipariş listesi |
| GET | `/api/v1/client-auth/invoices/:id` | halka açık | Sipariş ayrıntıları |
| GET | `/api/v1/client-auth/ip-whitelist` | halka açık | IP izin verilenler listesini listele |
| DELETE | `/api/v1/client-auth/ip-whitelist/:id` | halka açık | IP izin verilenler listesinden kaldır |
| POST | `/api/v1/client-auth/ip-whitelist/auto-add` | halka açık | Otomatik IP ekle |
| POST | `/api/v1/client-auth/login` | halka açık | Oturum aç |
| POST | `/api/v1/client-auth/logout` | halka açık | Oturumu kapat |
| POST | `/api/v1/client-auth/logs/package` | halka açık | Günlük paketi oluştur |
| GET | `/api/v1/client-auth/logs/packages` | halka açık | Günlük paketlerini listeleme |
| DELETE | `/api/v1/client-auth/logs/packages/:id` | halka açık | Günlük paketini sil |
| GET | `/api/v1/client-auth/logs/packages/:id` | halka açık | Günlük paketini al |
| GET | `/api/v1/client-auth/logs/packages/:id/download` | halka açık | Günlük paketini indirin |
| GET | `/api/v1/client-auth/logs/types` | halka açık | Günlük paketi durum denetimi |
| GET | `/api/v1/client-auth/me` | halka açık | Geçerli oturum açmış kullanıcıyı al |
| GET | `/api/v1/client-auth/nodes` | halka açık | Düğüm listem |
| GET | `/api/v1/client-auth/nodes/:id/config` | halka açık | Düğüm yapılandırması |
| GET | `/api/v1/client-auth/nodes/:id/link` | halka açık | Düğüm paylaşım bağlantısı |
| GET | `/api/v1/client-auth/nodes/:id/qrcode` | halka açık | Düğüm QR kodu |
| GET | `/api/v1/client-auth/oauth/:provider` | halka açık | Yetkilendir |
| GET | `/api/v1/client-auth/oauth/:provider/callback` | halka açık | Geri arama |
| GET | `/api/v1/client-auth/oauth/providers` | halka açık | Sağlayıcı durumu |
| GET | `/api/v1/client-auth/orders` | halka açık | Sipariş listesi |
| POST | `/api/v1/client-auth/orders` | halka açık | Sipariş oluştur |
| GET | `/api/v1/client-auth/orders/:id` | halka açık | Sipariş ayrıntıları |
| GET | `/api/v1/client-auth/plans` | halka açık | Plan listesi |
| GET | `/api/v1/client-auth/plans/:id` | halka açık | Plan ayrıntıları |
| GET | `/api/v1/client-auth/profile` | halka açık | Profili al |
| PUT | `/api/v1/client-auth/profile` | halka açık | Profili güncelle |
| POST | `/api/v1/client-auth/profile/bind-oauth` | halka açık | Üçüncü taraf hesabını bağla |
| DELETE | `/api/v1/client-auth/profile/unbind-oauth/:provider` | halka açık | Üçüncü taraf hesabının bağlantısını kaldırın |
| GET | `/api/v1/client-auth/referral/code` | halka açık | Yönlendirme doğrulama kodunu alın |
| GET | `/api/v1/client-auth/referral/records` | halka açık | Yönlendirme kayıtlarını listeleyin |
| GET | `/api/v1/client-auth/referral/stats` | halka açık | Yönlendirme istatistiklerini alın |
| POST | `/api/v1/client-auth/referral/withdraw` | halka açık | Para çekme isteği |
| GET | `/api/v1/client-auth/referral/withdrawals` | halka açık | Yönlendirmeden çekilenleri listeleme |
| POST | `/api/v1/client-auth/refresh` | halka açık | Yenile |
| POST | `/api/v1/client-auth/register` | halka açık | Kayıt ol |
| POST | `/api/v1/client-auth/reset-subscription` | halka açık | Aboneliği sıfırla |
| POST | `/api/v1/client-auth/rotate-token` | halka açık | Abonelik jetonunu döndür |
| POST | `/api/v1/client-auth/self-unban` | halka açık | Self servis yasağını kaldır |
| PUT | `/api/v1/client-auth/settings` | halka açık | Ayarları güncelle |
| POST | `/api/v1/client-auth/settings/auto-add-ip` | halka açık | Otomatik IP ekle |
| GET | `/api/v1/client-auth/settings/whitelist` | halka açık | IP izin verilenler listesini listele |
| DELETE | `/api/v1/client-auth/settings/whitelist/:id` | halka açık | IP izin verilenler listesinden kaldır |
| GET | `/api/v1/client-auth/shared-accounts` | halka açık | Paylaşılan hesapları listeleme |
| GET | `/api/v1/client-auth/shared-accounts/:id` | halka açık | Paylaşılan hesabı al |
| POST | `/api/v1/client-auth/shared-accounts/:id/apply` | halka açık | Paylaşılan hesap iste |
| POST | `/api/v1/client-auth/shared-accounts/:id/repair` | halka açık | Onarım talebi |
| POST | `/api/v1/client-auth/ticket` | halka açık | Bir kerelik kısa ömürlü bilet düzenleyin |
| GET | `/api/v1/client-auth/tickets` | halka açık | Biletleri listele |
| POST | `/api/v1/client-auth/tickets` | halka açık | Bilet oluştur |
| GET | `/api/v1/client-auth/tickets/:id` | halka açık | Bilet al |
| POST | `/api/v1/client-auth/tickets/:id/close` | halka açık | Bileti kapat |
| POST | `/api/v1/client-auth/tickets/:id/reply` | halka açık | Bilete yanıt ver |
| GET | `/api/v1/client-auth/traffic-history` | halka açık | Trafik geçmişi |
| GET | `/api/v1/client-auth/traffic-packs` | halka açık | Trafik paketi listesi |
| POST | `/api/v1/client-auth/traffic-packs/buy` | halka açık | Trafik paketi satın al |

### Abonelik teslimi

Müşterilerin abonelikleri alması için genel giriş noktası; bkz. `subscription.en.md`.

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| GET | `/sub/:token` | halka açık | Abonelik sun |

### Çalışma zamanı yapılandırma iş akışı

Anlık görüntüleri yapılandırma / prova / uygulama / geri alma (`apply`, `rollback` ve `disable-proxy` yönetici gerektirir).

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| POST | `/runtime/apply` | halka açık | Uygula |
| GET | `/runtime/apply-jobs` | Oturum açıldı | İşleri listele |
| GET | `/runtime/apply-jobs/:id` | Oturum açıldı | İş bul |
| POST | `/runtime/apply-jobs/:id/cancel` | halka açık | İşi iptal et |
| POST | `/runtime/apply-jobs/:id/retry` | halka açık | İşi yeniden dene |
| POST | `/runtime/disable-proxy` | halka açık | Proxy'yi devre dışı bırak |
| POST | `/runtime/dry-run` | Oturum açıldı | Deneme çalıştırması yürütme |
| GET | `/runtime/nodes` | Oturum açıldı | Düğümleri listele |
| GET | `/runtime/overview` | Oturum açıldı | Genel Bakış |
| POST | `/runtime/rollback` | halka açık | Geri al |
| GET | `/runtime/snapshots` | Oturum açıldı | Anlık görüntüleri listeleme |
| POST | `/runtime/snapshots` | Oturum açıldı | Anlık görüntü oluştur |
| DELETE | `/runtime/snapshots/:id` | halka açık | Anlık görüntüyü sil |
| GET | `/runtime/snapshots/:id` | Oturum açıldı | Anlık görüntü al |
| POST | `/runtime/verify` | Oturum açıldı | Doğrula |

### Hizmet erişim politikaları (kullanımdan kaldırıldı)

> `/services/:id/access-policies` ve `/services/:id/ip-bans`, 2026-10-20 tarihinde kullanımdan kaldırıldı:
> yönetici çağrısı yoktur ve bu ilkeleri kullanan hiçbir bağlantı yolu yoktur.

### Trafik kotası izleme

Kotaların ve kullanımın salt okunur izleme görünümü.

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| GET | `/traffic/quotas` | Oturum açıldı | Kota listesi |
| GET | `/traffic/quotas/:id/usage` | Oturum açıldı | Kota kullanımı |
| GET | `/traffic/reset-logs` | Oturum açıldı | Kota sıfırlama kayıtları |
| GET | `/traffic/usage-history` | Oturum açıldı | Kullanım geçmişi |

### Sağlık kontrolleri

Canlılık araştırması.

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| GET | `/health` | halka açık | Canlılık araştırması; 200 sağlıklı demektir |

### Portal sayfası

Portal SPA sayfası.

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| GET | `/p` | halka açık | Portal HTML (sunucu tarafından oluşturulan + Vue çalışma zamanı) |

### ACME zorlukları

HTTP-01 geri arama sorgulaması.

| Yöntem | Yol | İzin | Açıklama |
|---|---|---|---|
| GET | `/.well-known/acme-challenge/:token` | halka açık | ACME HTTP-01 geri arama sorgulaması (verim sırasında alanı doğrular) |
