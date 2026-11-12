<!-- source: subscription.md | locale: tr | updated: 2026-11-12 -->
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
| `providers` | Yalnızca `mihomo_yaml`: `1` / `true`, **Sağlayıcı modunu** etkinleştirir (`proxy-providers`'yi yayınlayın ve `use` aracılığıyla bunlara referans verin); varsayılan veya başka herhangi bir değer, düğümleri `proxies` altında satır içi tutar. Bkz. §6 |

### Desteklenen `format` değerleri

| Değer | Çıkış | Müşteriler |
|---|---|---|
| `base64_uri` (varsayılan) | Yeni satıra birleştirilmiş paylaşım bağlantıları, bir bütün olarak base64 kodlu | V2RayN, Shadowrocket, Surge ve diğer genel istemciler |
| `raw_uri` | Yeni satıra katılan paylaşım bağlantıları, düz metin | Düz bağlantılara/hata ayıklamaya ihtiyaç duyan istemciler |
| `mihomo_yaml` | Tam Clash/Mihomo YAML yapılandırması (proxy grupları ve kurallarla) | Clash.Meta / mihomo / Clash Verge Rev |
| `mihomo_proxies_yaml` | Yalnızca `proxies:` düğüm listesiyle YAML | mihomo proxy sağlayıcı kaynağı (planlanmış yenilemeyle) |
| `singbox_json` | Tam sing-box JSON yapılandırması | sing-box / SFI / FLClash (sing-box çekirdeği) |
| `xray_json` | Tam Xray JSON yapılandırması | Xray-çekirdek |

> Protokol/format uyumluluğu geçerlidir: örneğin, `anytls` yalnızca sing-box/Mihomo formatlarında görünür; **xray tarafından sunulan REALITY düğümleri, sing-box aboneliklerinden hariç tutulur** (sing-box, xray'nin REALITY uygulamasını desteklemez, bkz. sing-box#4520) ancak yine de Xray aboneliklerinde görünürken **REALITY düğümleri hizmet verir sing-box tarafından hem Xray hem de sing-box aboneliklerinde çalışır**. İstemcinin tanıyamadığı düğümler, tüm aboneliğin başarısız olması yerine otomatik olarak bırakılır.

## 3. Yanıt başlıkları

| Başlık | Örnek | Açıklama |
|---|---|---|
| `Subscription-Userinfo` | `upload=1024; download=2048; total=1073741824; expire=1760000000` | Kullanım ve sona erme (bayt / Unix saniye). **Ayrıca 403'e de gönderilir**, böylece müşteriler kalan ödeneği gösterebilirler |
| `Profile-Update-Interval` | `6` | Önerilen yenileme aralığı (saat) |
| `Profile-Title` | `CoreFusion` | Profil adı (Clash Verge Rev / V2RayN vb. tarafından gösterilir) |
| `Profile-Web-Page-Url` | `https://portal.example.com` | Portal URL (istemcilerde “web sayfasını aç”; portalın kendi bağlantı noktası olduğunda kök yol, aksi halde /p) |
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

# 5) Clash dynamic subscription: config template + Provider mode
curl -L "https://sub.example.com/abcd1234efgh5678?format=clash&config=out&providers=1"

# 6) sing-box / Xray with their own config templates (rules come from the template)
curl -L "https://sub.example.com/abcd1234efgh5678?format=singbox_json&config=sing-box-home"
curl -L "https://sub.example.com/abcd1234efgh5678?format=xray_json&config=xray-home"
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

## 6. Sağlayıcı modu ve birden fazla abonelik alanı

### 6.1 `providers=1`'nin yaydığı şeyler

`format=mihomo_yaml&providers=1` (veya `format=clash&providers=1`) ile YAML:

- artık en üst seviyedeki satır içi düğümleri kullanmıyor; `proxy-providers:` yayınlıyor;
- proxy grupları `use: [<provider-key>...]` aracılığıyla düğüm kaynaklarına referans verir;
- paylaşılan abonelik biçiminde (`mihomo_configs.config_json`) yazılan herhangi bir `proxy-providers` **yoksayılır** (paylaşılan bir yapılandırma çeşitli planlara hizmet eder; sabit kodlanmış sağlayıcılar planlar arasında sızıntı yapar). Grup → kaynak eşlemeleri “Plan yönetimi → grup → kaynak eşleme” altında yayınlanır.

Sağlayıcı türleri:

| tür | Anahtar | Notlar |
|---|---|---|
| Dahili | Temizlenmiş düğüm grubu adı | Plan düğüm grubu başına oluşturulur; yalnızca o grubun düğümlerini içerir |
| Ayrılmış | `subscription-nodes` | Planın tüm düğümleri; kaynağı URL `provider` parametresini **çıkarır** |
| Harici | Planda yapılandırılan ad | Harici kaynak (`http` YAML veya `file`) |

Sağlayıcı başına ortak alanlar:

- `url` — ana istekten `config` / `platform`'yi devralan `format=mihomo_proxies_yaml` ile bu aboneliğe işaret eder; dahili sağlayıcılar `provider=<key>`'yi ekler;
- `path` — **her zaman “dahili sağlayıcı varsayılanlarından → önbellek yolu şablonundan” türetilmiştir** (varsayılan `./providers/{key}.yaml`, `{key}` değiştirildi). Dış sağlayıcılar da kendi yollarını belirleyemez;
- `health-check` — `enable / url / interval`; dahili sağlayıcılar varsayılanları devralır, harici sağlayıcılar bunu planda yapılandırır;
- `interval` — saniye cinsinden yenileme aralığı.

### 6.2 URL (çoklu alan adı) sağlayıcısına hangi alan adı girer?

"Abonelik merkezi → Abonelik uç noktaları" **birden fazla** kayıt tutabilir (etki alanı + yol + etkin). Sağlayıcı `url` alan adını şu sırayla seçer:

1. **Bu isteğin gerçekte kullandığı etki alanı**: `X-Forwarded-Host`'nin ilk değeri (bağlantı noktası ayrılmış), aksi takdirde `Host`; **etkin** bir uç noktayla eşleşmelidir;
2. Hiçbir şey eşleşmezse **en düşük kimliğe** sahip etkin uç noktaya geri dönün;
3. Yol, eşleşen uç noktadan (`path`, varsayılan `/`) gelir ve `https://<domain><path>/<token>?format=mihomo_proxies_yaml...` sonucunu verir.

Sonuçlar:

- Yeni bir alan adı üzerinden veri getiren bir istemci, bu alan adını sağlayıcının içine alır — **panel değişikliğine gerek yoktur**;
- Listede hâlâ etkin olan eski bir alan adı artık sabit kodlu değildir;
- Bir istemci eski sağlayıcı URL'yi önbelleğe aldıysa, aboneliğin güncellenmesi (veya sağlayıcı önbelleğinin temizlenmesi / istemcinin yeniden başlatılması) onu değiştirir.

Ters proxy'ler `X-Forwarded-Proto` ve `X-Forwarded-Host`'yi iletmelidir; aksi halde geri dönüş uç noktası kullanılır. Panelin oluşturulan Caddy / Nginx site şablonları zaten her iki başlığı da içerir (`header_up X-Forwarded-Host {host}` / `proxy_set_header X-Forwarded-Host $host;`); Proxy yapılandırmasını elle koruyorsanız bunları manuel olarak ekleyin.

## 7. Notlar

- **Biçim ve yapılandırma birbirine sıkı sıkıya bağlıdır**: `config`/`platform` tarafından seçilen şablon, bu isteğin `format`'siyle eşleşmelidir; eşleşmeyen şablonlar göz ardı edilir; bu, Mihomo kurallarının sing-box/Xray JSON'ye enjekte edilmesini önler.
- **`providers=1` yalnızca Mihomo YAML'yi etkiler**: sing-box / Xray / base64 / raw bunu yok sayar (düğümler satır içi kalır).
- **Hız sınırlaması**: Abonelik uç noktasının kendi sınırı vardır (`SubscribeRateLimit`); Kötü niyetli müşteriler reddedilir.
- **Erişim günlüğü**: Her getirme işleminde IP / UA / CDN bilgileri kaydedilir ve yönetici arayüzünde "Abonelik erişim günlüğü" altında görünür.
- Özel bir abonelik bağlantı noktasında yapılandırılan bir etki alanı olmadan, isteğin `X-Forwarded-Proto` / `X-Forwarded-Host`'si kullanılır; ters proxy'nizin her iki başlığı da ilettiğinden emin olun.
- Çok alanlı dağıtımlarda, uç nokta listesini istemcilerin gerçekte kullandığı alan adlarıyla uyumlu tutun; bkz. §6.2.
