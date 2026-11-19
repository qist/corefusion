<!-- source: THIRD_PARTY_LICENSES.md | locale: tr | updated: 2026-11-19 -->
# Üçüncü Taraf Bileşenler ve Lisanslar

> Diller: [简体中文](THIRD_PARTY_LICENSES.md) · [English](THIRD_PARTY_LICENSES.en.md) · [繁體中文](THIRD_PARTY_LICENSES.zh-TW.md) · [Русский](THIRD_PARTY_LICENSES.ru.md) · [日本語](THIRD_PARTY_LICENSES.ja.md) · [한국어](THIRD_PARTY_LICENSES.ko.md) · [Deutsch](THIRD_PARTY_LICENSES.de.md) · [Français](THIRD_PARTY_LICENSES.fr.md) · [Español](THIRD_PARTY_LICENSES.es.md) · [Português](THIRD_PARTY_LICENSES.pt.md) · Türkçe · [Bahasa Indonesia](THIRD_PARTY_LICENSES.id.md) · [ไทย](THIRD_PARTY_LICENSES.th.md) · [Tiếng Việt](THIRD_PARTY_LICENSES.vi.md) · [العربية](THIRD_PARTY_LICENSES.ar.md) · [فارسی](THIRD_PARTY_LICENSES.fa.md)

CoreFusion aşağıdaki açık kaynak bileşenlerini kullanır. Bu dosya ana bileşenleri ve lisansları listeler; **tam liste `go.mod` / `go.sum`, `web/admin/package.json` ve her modülün kendi LİSANS dosyasıyla tanımlanır**.

Bu projenin ikili dosyalarını dağıtırken bu dosyayı, `LICENSE` projesini ve tüm üçüncü taraf bileşenlerin telif hakkı/lisans bildirimlerini saklayın.

## 1. İkili dosyada derlenen bağımlılıklar (ana)

| Bileşen | Lisans | Amaç |
|---|---|---|
| `github.com/xtls/xray-core` | MPL-2.0 | Xray yapılandırma ayrıştırma, gRPC komutları, protokol tanımları |
| `github.com/xtls/reality` | MPL-2.0 | REALITY protokol desteği |
| `github.com/sagernet/sing` | **GPL-3.0-or-later** | xray-core aracılığıyla dolaylı olarak bağlantılı |
| `github.com/sagernet/sing-shadowsocks` | **GPL-3.0-or-later** | xray-core (Shadowsocks-2022) aracılığıyla dolaylı olarak bağlantılı |
| `github.com/go-sql-driver/mysql` | MPL-2.0 | MySQL sürücüsü |
| `github.com/hashicorp/golang-lru/v2` | MPL-2.0 | LRU önbelleği |
| `github.com/go-acme/lego/v5` | MIT | ACME sertifika verilmesi |
| `github.com/gin-gonic/gin` | MIT | HTTP çerçevesi |
| `gorm.io/gorm` | MIT | ORM |
| `gorm.io/driver/mysql` | MIT | GORM için MySQL sürücüsü |
| `gorm.io/driver/postgres` | MIT | GORM için PostgreSQL sürücüsü |
| `github.com/jackc/pgx/v5` | MIT | PostgreSQL sürücüsü |
| `github.com/qist/sqlite` | MIT | SQLite sürücüsü (panel / Aracı için varsayılan veritabanı) |
| `github.com/aws/aws-sdk-go-v2` | Apache-2.0 | S3 yedeklemeleri, Route53 DNS-01 |
| `google.golang.org/grpc` | Apache-2.0 | Aracı gRPC iletişimi |
| `google.golang.org/protobuf` | BSD-3-Clause | Protobuf çalışma zamanı |
| `golang.org/x/crypto` | BSD-3-Clause | Kriptografik ilkeller |
| `golang.org/x/oauth2` | BSD-3-Clause | OAuth2 / Google EAB |
| `golang.org/x/sys` | BSD-3-Clause | Sistem çağrıları (nft/tc hız sınırlama, süreç yönetimi) |
| `github.com/golang-jwt/jwt/v5` | MIT | JWT |
| `github.com/google/uuid` | BSD-3-Clause | UUID |
| `github.com/go-webauthn/webauthn` | BSD-3-Clause | Passkey / WebAuthn oturum açma |
| `github.com/pquerna/otp` | Apache-2.0 | TOTP iki faktörlü kimlik doğrulama |
| `github.com/jlaffaye/ftp` | ISC | FTP yedekleme yüklemesi |
| `gopkg.in/yaml.v3` | MIT / Apache-2.0 | YAML yapılandırma ayrıştırma |

> ⚠ **GPL-3.0 bileşenleri**: `sing` ve `sing-shadowsocks`, GPL-3.0 veya üzeridir. Bunlar panele ve Aracı ikili dosyalarına statik olarak bağlıdırlar, dolayısıyla **ikili dosyaları dağıtmak Karşılık Gelen Kaynağın sağlanmasını gerektirir** (bkz. [LEGAL.md](./LEGAL.md)'nin 3. bölümü).

## 2. Ön uç bağımlılıkları (panel ikili dosyasında derlenir)

Yönetici kullanıcı arayüzü Vite ile oluşturulmuştur ve `go:embed` aracılığıyla panel ikili programına yerleştirilmiştir, dolayısıyla aşağıdaki bileşenler de ikili dosyayla birlikte dağıtılır:

| Bileşen | Lisans | Amaç |
|---|---|---|
| Vue 3 / Vue Router / Pinia / Element Plus / axios / vue-i18n / marked / js-yaml / qrcode / Monaco Editor | MIT | Yönetici arayüzü, yönlendirme ve durum, i18n, Markdown oluşturma, QR kodları ve kod düzenleyici |
| DOMPurify | Apache-2.0 / MPL-2.0 | Markdown / HTML'yi oluşturmadan önce XSS temizliği |

## 3. Çalışma zamanında çağrılan bağımsız programlar (bu depoda dağıtılmaz)

Panel / Temsilci aşağıdaki bağımsız programları çalışma zamanında indirebilir, kurabilir veya çalıştırabilir. Bu projeyle süreçler/arayüzler aracılığıyla etkileşime girerler, ancak bunları teslimatlarınızla birlikte gönderirseniz yine de lisanslarına uymanız gerekir:

| programı | Lisans | Notlar |
|---|---|---|
| Xray-core | MPL-2.0 | Proxy çekirdeği, ayrı süreç |
| sing-box | **GPL-3.0-or-later** | Proxy çekirdeği, ayrı süreç; dağıtım sırasında ilgili kaynağı sağlayın |
| Caddy | Apache-2.0 | Ters proxy / TLS |
| Nginx | BSD-2-Clause | Ters proxy (kullanılıyorsa sistem yüklü) |

> Hysteria2 ve TUIC, bağımsız çekirdekler veya ikili dosyalar değil, **sing-box** tarafından taşınan protokollerdir, dolayısıyla ayrı olarak listelenmezler.

## 4. Veri dosyaları

| Veri | Kaynak / lisans |
|---|---|
| `geoip.dat` / `geosite.dat` | Veri kaynağı deposu tarafından bildirildiği üzere (genellikle GPL-3.0 veya CC-BY-SA) |
| GeoLite2 (`GeoLite2-ASN.mmdb`, vb.) | MaxMind GeoLite2 EULA / CC BY-SA 4.0; ticari kullanımdan önce lisanslamayı doğrulayın |
| `country.mmdb` | Veri kaynağı tarafından beyan edildiği gibi |

## 5. Uyumluluk önerileri

- İkili dosyaları teslim ederken şunları ekleyin: `LICENSE`, bu dosya ve her GPL/MPL bileşeninin lisans metni veya satın alma URL.
- `xray-core` gibi MPL-2.0 bileşenlerinin kaynak dosyalarını değiştirirseniz, değiştirilen dosyaların kaynağını yayınlayın.
- sing-box gibi GPL programlarını ürününüzle birlikte dağıtıyorsanız, alıcılara o sürümün ilgili kaynağının tamamını (veya etkili bir yazılı teklif) sağlayın.
- Ticari markalar ve adlar: Yukarı yöndeki proje adlarını veya logolarını, resmi onayı ima edecek şekilde kullanmayın.
