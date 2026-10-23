<!-- source: THIRD_PARTY_LICENSES.md | locale: tr | updated: 2026-10-23 -->
# Üçüncü Taraf Bileşenler ve Lisanslar

> Diller: [简体中文](THIRD_PARTY_LICENSES.md) · [English](THIRD_PARTY_LICENSES.en.md) · [繁體中文](THIRD_PARTY_LICENSES.zh-TW.md) · [Русский](THIRD_PARTY_LICENSES.ru.md) · [日本語](THIRD_PARTY_LICENSES.ja.md) · [한국어](THIRD_PARTY_LICENSES.ko.md) · [Deutsch](THIRD_PARTY_LICENSES.de.md) · [Français](THIRD_PARTY_LICENSES.fr.md) · [Español](THIRD_PARTY_LICENSES.es.md) · [Português](THIRD_PARTY_LICENSES.pt.md) · Türkçe · [Bahasa Indonesia](THIRD_PARTY_LICENSES.id.md) · [ไทย](THIRD_PARTY_LICENSES.th.md) · [Tiếng Việt](THIRD_PARTY_LICENSES.vi.md) · [العربية](THIRD_PARTY_LICENSES.ar.md) · [فارسی](THIRD_PARTY_LICENSES.fa.md)

CoreFusion aşağıdaki açık kaynak bileşenlerini kullanır. Bu dosya ana bileşenleri ve lisansları listeler; **tam liste `go.mod` / `go.sum` ve her modülün kendi LİSANS dosyası tarafından tanımlanır**.

Bu projenin ikili dosyalarını dağıtırken bu dosyayı, `LICENSE` projesini ve tüm üçüncü taraf bileşenlerin telif hakkı/lisans bildirimlerini saklayın.

## 1. İkili dosyada derlenen bağımlılıklar (ana)

| Bileşen | Lisans | Amaç |
|---|---|---|
| `github.com/xtls/xray-core` | MPL-2.0 | Xray yapılandırma ayrıştırma, gRPC komutları, protokol tanımları |
| `github.com/xtls/reality` | MPL-2.0 | REALITY protokol desteği |
| `github.com/sagernet/sing` | **GPL-3.0 veya üzeri** | xray-core aracılığıyla dolaylı olarak bağlantılı |
| `github.com/sagernet/sing-shadowsocks` | **GPL-3.0 veya üzeri** | xray-core (Shadowsocks-2022) aracılığıyla dolaylı olarak bağlantılı |
| `github.com/go-sql-driver/mysql` | MPL-2.0 | MySQL sürücüsü |
| `github.com/hashicorp/golang-lru/v2` | MPL-2.0 | LRU önbelleği |
| `github.com/go-acme/lego/v4` | MİT | ACME sertifika verilmesi |
| `github.com/gin-gonic/gin` | MİT | HTTP çerçevesi |
| `gorm.io/gorm` | MİT | ORM |
| `github.com/aws/aws-sdk-go-v2` | Apache-2.0 | S3 yedeklemeleri, Route53 DNS-01 |
| `google.golang.org/grpc` | Apache-2.0 | Aracı gRPC iletişimi |
| `google.golang.org/protobuf` | BSD-3-Madde | Protobuf çalışma zamanı |
| `golang.org/x/crypto` | BSD-3-Madde | Kriptografik ilkeller |
| `golang.org/x/oauth2` | BSD-3-Madde | OAuth2 / Google EAB |
| `github.com/golang-jwt/jwt/v5` | MİT | JWT |
| `github.com/google/uuid` | BSD-3-Madde | UUID |

> ⚠ **GPL-3.0 bileşenleri**: `sing` ve `sing-shadowsocks`, GPL-3.0 veya üzeridir. Bunlar panele ve Aracı ikili dosyalarına statik olarak bağlıdır, bu nedenle **ikili dosyaları dağıtmak Karşılık Gelen Kaynağın sağlanmasını gerektirir** (bkz. [LEGAL.md](THIRD_PARTY_LICENSES.tr.md)'nin 3. bölümü).

## 2. Çalışma zamanında çağrılan bağımsız programlar (bu depoda dağıtılmaz)

Panel / Temsilci aşağıdaki bağımsız programları çalışma zamanında indirebilir, kurabilir veya çalıştırabilir. Bu projeyle süreçler/arayüzler aracılığıyla etkileşime girerler, ancak bunları teslimatlarınızla birlikte gönderirseniz yine de lisanslarına uymanız gerekir:

| programı | Lisans | Notlar |
|---|---|---|
| Xray-çekirdek | MPL-2.0 | Proxy çekirdeği, ayrı süreç |
| sing-box | **GPL-3.0 veya üzeri** | Proxy çekirdeği, ayrı süreç; dağıtım sırasında ilgili kaynağı sağlayın |
| Caddy | Apache-2.0 | Ters proxy / TLS |
| Nginx | BSD-2-Madde | Ters proxy (kullanılıyorsa sistem yüklü) |

> Hysteria2 ve TUIC, bağımsız çekirdekler veya ikili dosyalar değil, **sing-box** tarafından taşınan protokollerdir, dolayısıyla ayrı olarak listelenmezler.

## 3. Veri dosyaları

| Veri | Kaynak / lisans |
|---|---|
| `geoip.dat` / `geosite.dat` | Veri kaynağı deposu tarafından bildirildiği üzere (genellikle GPL-3.0 veya CC-BY-SA) |
| GeoLite2 (`GeoLite2-ASN.mmdb`, vb.) | MaxMind GeoLite2 EULA / CC BY-SA 4.0; ticari kullanımdan önce lisanslamayı doğrulayın |
| `country.mmdb` | Veri kaynağı tarafından beyan edildiği gibi |

## 4. Uyumluluk önerileri

- İkili dosyaları teslim ederken şunları ekleyin: `LICENSE`, bu dosya ve her GPL/MPL bileşeninin lisans metni veya satın alma URL.
- `xray-core` gibi MPL-2.0 bileşenlerinin kaynak dosyalarını değiştirirseniz, değiştirilen dosyaların kaynağını yayınlayın.
- sing-box gibi GPL programlarını ürününüzle birlikte dağıtıyorsanız, alıcılara söz konusu sürümün ilgili kaynağının tamamını (veya etkili bir yazılı teklif) sağlayın.
- Ticari markalar ve adlar: Yukarı yöndeki proje adlarını veya logolarını, resmi onayı ima edecek şekilde kullanmayın.
