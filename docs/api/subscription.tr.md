<!-- source: subscription.md | locale: tr | updated: 2026-10-23 -->
# Abonelik API (genel)

> Diller: [简体中文](subscription.md) · [English](subscription.en.md) · [繁體中文](subscription.zh-TW.md) · [Русский](subscription.ru.md) · [日本語](subscription.ja.md) · [한국어](subscription.ko.md) · [Deutsch](subscription.de.md) · [Français](subscription.fr.md) · [Español](subscription.es.md) · [Português](subscription.pt.md) · Türkçe · [Bahasa Indonesia](subscription.id.md) · [ไทย](subscription.th.md) · [Tiếng Việt](subscription.vi.md) · [العربية](subscription.ar.md) · [فارسی](subscription.fa.md)

Bir kullanıcının abonelik düğümü listesini getirir. **Bu, son kullanıcılar için en çok kullanılan uç noktadır**; başlık gerekmez; erişim URL'deki belirteç tarafından sağlanır.

## 1. Uç nokta

| Dağıtım | URL |
|---|---|
| Ana panel | `GET https://<panel-domain>/sub/<token>` |
| Özel abonelik bağlantı noktası | `GET https://<sub-domain>:<port>/<token>` (`/sub` öneki yok) |

`<token>`, yönetici tarafı tarafından oluşturulan bir aboneliğin benzersiz erişim belirtecidir (portalda "Aboneliklerim" altında görünür).

## 2. Sorgu parametreleri

| Parametre | Açıklama |
|---|---|
| `format` | Çıkış formatı; abonelik varsayılanını geçersiz kılar. Aşağıya bakın |
| `config` | Yapılandırma şablonu adı (yönetici arayüzünde "Abonelik biçimleri" altında tanımlanır). Bu isteğin `format`'si ile aynı format kullanılmalıdır, aksi takdirde dikkate alınmaz |
| `platform` | Bir yapılandırma şablonunu platform adına göre eşleştirin (`config` atlandığında kullanılır); formatın da eşleşmesi gerekir |

### Desteklenen `format` değerleri

| Değer | Çıkış | Müşteriler |
|---|---|---|
| `base64_uri` (varsayılan) | Yeni satıra birleştirilmiş paylaşım bağlantıları, bir bütün olarak base64 ile kodlanmış | V2RayN, Shadowrocket, Surge ve diğer genel istemciler |
| `raw_uri` | Yeni satıra katılan paylaşım bağlantıları, düz metin | Düz bağlantılara/hata ayıklamaya ihtiyaç duyan istemciler |
| `mihomo_yaml` | Tam Clash/Mihomo YAML yapılandırması (proxy grupları ve kurallarla) | Clash.Meta / mihomo / Clash Verge Rev |
| `mihomo_proxies_yaml` | Yalnızca `proxies:` düğüm listesiyle YAML | mihomo proxy sağlayıcı kaynağı (planlanmış yenilemeyle) |
| `singbox_json` | Tam sing-box JSON konfigürasyonu | sing-box / SFI / FLClash (sing-box çekirdeği) |
| `xray_json` | Tam Xray JSON yapılandırması | Xray-çekirdek |

> Protokol/format uyumluluğu geçerlidir: örneğin, `anytls` yalnızca sing-box/Mihomo formatlarında görünür; sing-box sunucusundaki REALITY düğümleri, Xray/sing-box JSON aboneliklerinde görünmez. Aboneliğin tamamının başarısız olması yerine istemcinin tanıyamadığı düğümler otomatik olarak bırakılır.

## 3. Yanıt başlıkları

| Başlık | Örnek | Açıklama |
|---|---|---|
| `Subscription-Userinfo` | `upload=1024; download=2048; total=1073741824; expire=1760000000` | Kullanım ve sona erme (bayt / Unix saniye). **Ayrıca 403'e de gönderilir**, böylece müşteriler kalan ödeneği gösterebilirler |
| `Profile-Update-Interval` | `6` | Önerilen yenileme aralığı (saat) |
| `Profile-Title` | `CoreFusion` | Profil adı (Clash Verge Rev / V2RayN vb. tarafından gösterilir) |
| `Profile-Web-Page-Url` | `https://…/portal` | Portal sayfası bağlantısı (istemcilerde “web sayfasını aç”) |
| `Content-Type` | Format başına: `text/plain` / `text/yaml` / `application/json` |  |
| `Content-Disposition` | `attachment; filename="subscription.txt"` |  |

## 4. Durum kodları

| HTTP | kod | Anlamı |
|---|---|---|
| 200 | — | Normal (gövde, `format`'de oluşturulan aboneliktir) |
| 400 | `MISSING_TOKEN` | URL'de jeton yok |
| 403 | (düz metin) | Abonelik askıya alındı / süresi doldu / etkin değil. Mesaj `Accept-Language` aracılığıyla yerelleştirilmiştir (zh/en/ja/ko) ve tek satırlık bir nedendir |
| 404 | `SUBSCRIPTION_NOT_FOUND` | Jeton mevcut değil |

> İptal edilen abonelikler mevcut olmayan abonelikler olarak kabul edilir (404).

## 5. Örnekler

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

Clash/Mihomo dinamik aboneliği (periyodik yenileme):

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

## 6. Notlar

- **Biçim ve yapılandırma birbirine sıkı sıkıya bağlıdır**: `config`/`platform` tarafından seçilen şablon bu isteğin `format`'siyle eşleşmelidir; eşleşmeyen şablonlar göz ardı edilir; bu, Mihomo kurallarının sing-box/Xray JSON'ye enjekte edilmesini önler.
- **Hız sınırlaması**: Abonelik uç noktasının kendi sınırı vardır (`SubscribeRateLimit`); Kötü niyetli müşteriler reddedilir.
- **Erişim günlüğü**: Her getirme işleminde IP / UA / CDN bilgileri kaydedilir ve yönetici arayüzünde "Abonelik erişim günlüğü" altında görünür.
- Özel bir abonelik bağlantı noktasında yapılandırılan bir etki alanı olmadan, isteğin `X-Forwarded-Proto` / `X-Forwarded-Host`'si kullanılır; ters proxy'nizin her iki başlığı da ilettiğinden emin olun.
