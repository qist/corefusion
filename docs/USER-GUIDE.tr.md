<!-- source: USER-GUIDE.md | locale: tr | updated: 2026-10-26 -->
# CoreFusion Operasyon Kılavuzu

> Diller: [简体中文](USER-GUIDE.md) · [English](USER-GUIDE.en.md) · [繁體中文](USER-GUIDE.zh-TW.md) · [Русский](USER-GUIDE.ru.md) · [日本語](USER-GUIDE.ja.md) · [한국어](USER-GUIDE.ko.md) · [Deutsch](USER-GUIDE.de.md) · [Français](USER-GUIDE.fr.md) · [Español](USER-GUIDE.es.md) · [Português](USER-GUIDE.pt.md) · Türkçe · [Bahasa Indonesia](USER-GUIDE.id.md) · [ไทย](USER-GUIDE.th.md) · [Tiếng Việt](USER-GUIDE.vi.md) · [العربية](USER-GUIDE.ar.md) · [فارسی](USER-GUIDE.fa.md)

> Bu kılavuz CoreFusion kullanıcıları içindir ve kurulumdan günlük işlemlere kadar tüm iş akışını kapsar.

---

## İçindekiler

1. [Sisteme genel bakış](#1-sisteme-genel-bakış)
2. [Kurulum](#2-kurulum)
3. [İlk konfigürasyon](#3-ilk-konfigürasyon)
4. [Kontrol Paneli](#4-kontrol-paneli)
5. [Düğüm yönetimi](#5-düğüm-yönetimi)
6. [Protokol merkezi](#6-protokol-merkezi)
7. [Proxy yönetimi](#7-proxy-yönetimi)
8. [Yönlendirme kaynakları](#8-yönlendirme-kaynakları)
9. [Abonelik merkezi](#9-abonelik-merkezi)
10. [Müşteri yönetimi](#10-müşteri-yönetimi)
11. [Plan yönetimi](#11-plan-yönetimi)
12. [Şarj yönetimi](#12-şarj-yönetimi)
13. [Bilet yönetimi](#13-bilet-yönetimi)
14. [Sertifika yönetimi](#14-sertifika-yönetimi)
15. [Trafik kotaları](#15-trafik-kotaları)
16. [Zincirlenmiş proxy'ler](#16-zincirlenmiş-proxyler)
17. [Çevrimiçi kullanıcılar](#17-çevrimiçi-kullanıcılar)
18. [Kullanıcı yönetimi](#18-kullanıcı-yönetimi)
19. [Portal sitesi](#19-portal-sitesi)
20. [Sistem ayarları](#20-sistem-ayarları)
21. [Operasyon konsolu](#21-operasyon-konsolu)
22. [Veritabanı izleme](#22-veritabanı-izleme)
23. [Denetim günlüğü](#23-denetim-günlüğü)
24. [Günlük yönetimi](#24-günlük-yönetimi)
25. [Operasyon uzantıları](#25-operasyon-uzantıları)
26. [SSS](#26-sss)
- [Ek](#ek)

---

## 1. Sisteme genel bakış

CoreFusion, düğüm yönetimi, abonelik teslimi, müşteri yönetimi, faturalandırma, sertifikalar ve zincirleme proxy'ler ile VLESS, VMess, Trojan, Shadowsocks, Hysteria2, TUIC ve daha fazlasını destekleyen çok protokollü bir proxy yönetim panelidir.

### 1.1 Temel özellikler

| Modül | Açıklama |
|--------|-------------|
| Çoklu protokol | VLESS, VMess, Trojan, Shadowsocks, Hysteria2, TUIC, WireGuard, vb. |
| Çok çekirdekli | Xray-core ve sing-box (Hysteria2/TUIC ve diğer QUIC protokolleri sing-box üzerinde çalışır) |
| Çok düğümlü | Beş düğüm türü: bağımsız, ayna, etkin/yedek, zincirleme giriş, zincirleme çıkış |
| Abonelik teslimi | Çoklu abonelik formatları, uç noktalar ve format yönetimi |
| Faturalandırma | Plan şablonları, yükleme kartları, bakiye, siparişler, bayi komisyonu |
| Sertifikalar | Let's Encrypt verilmesi, DNS-01/HTTP-01 doğrulaması, sertifika dağıtımı |
| Zincirlenmiş proxy | Çok çıkışlı yük dengeleme, harici düğüm kaydı, zincirleme yönlendirme |
| Trafik kotaları | Düğüm başına aylık trafik izleme, otomatik askıya alma, günlükleri sıfırlama |
| Portal | Görsel portal düzenleyici, çok dilli, özel şablonlar |
| Bildirimler | E-posta ve Telegram, şablonlar ve yayınlar |

### 1.2 Mimarlık

- **Arka uç**: Git, gRPC
- **Ön uç**: Vue 3 + Element Plus
- **Veritabanı**: SQLite (varsayılan) / MySQL / PostgreSQL
- **Proxy**: Caddy / Nginx ters proxy
- **Panel bağlantı noktası**: kurulum sihirbazı tarafından atanır (varsayılan olarak rastgele boş bağlantı noktası; 8080 yaygın bir seçimdir)
- **Ajan gRPC bağlantı noktası**: varsayılan olarak 18443

---

## 2. Kurulum

### 2.1 Desteklenen ortamlar

| Öğe | Destek |
|------|---------|
| Dağılımlar | Debian / Ubuntu, RHEL / CentOS / Rocky / AlmaLinux / Fedora, Alpine, Arch, openSUSE |
| CPU mimarileri | Resmi ikili dosyalar: `amd64`, `arm64`; kaynak yapıları aynı zamanda `armv7`, `armv6`, `386`, `ppc64le`, `s390x`, `riscv64`, `loong64`'yi de destekler |
| Servis yöneticisi | systemd (önerilen); systemd'nin bulunmadığı kaplarda veya diğer ortamlarda `nohup`'ye geri döner |
| Bellek / disk | ≥ 1 GB / ≥ 10 GB (veritabanı, sertifikalar, günlükler ve çekirdek ikili dosyalar veri dizininde bulunur) |
| Panel bağlantı noktası | kurulum sihirbazında seçilmiştir (varsayılan olarak rastgele boş bağlantı noktası; `8080` ve benzerleri uygundur) |
| Aracı gRPC'si | Varsayılan olarak `18443` |

Çalışma zamanı harici komutları yükleyici tarafından otomatik olarak yüklenir (`scripts/install.sh` ve panel tarafından oluşturulan Agent yükleme komut dosyası; `--no-deps` / `CF_SKIP_DEPS=1` ile atlayın):

| Amaç | Paket |
|---------|---------|
| Güvenlik duvarı / hız sınırı / eşzamanlılık işaretlemesi | `nftables` (`nft`) |
| Trafik şekillendirme, bağlantılar ve soketler | `iproute2` (`tc` / `ip` / `ss`) |
| NFT kullanılamadığında geri dönüş | `iptables` (EL8+ üzerinde `iptables-nft`, EL7 üzerinde `iptables`) |
| Bağlantı izleme | `conntrack-tools` (`conntrack`) |
| `nf_conntrack` / `ifb`'yi yükleyin | `kmod` (`modprobe` / `lsmod`) |
| Proses algılama ve temizleme | `procps` (`pgrep` / `pkill`) |
| Bağlantı noktası kontrolleri / DH parametreleri | `lsof` / `openssl` |
| Proxy kullanıcı oluşturma | `shadow-utils` (Debian `passwd`, Alp/Arch `shadow`) |
| İndirmeler ve arşivler | `curl` / `ca-certificates` / `tar` / `gzip` / `xz` |

> Eksik `nft` / `tc` yalnızca hız sınırlamayı ve güvenlik duvarı özelliklerini etkiler; panel hala başlıyor.
> Ortamın nasıl algılandığını ve hangi paketlerin eksik olduğunu görmek için önce `--cpu-info`'yi çalıştırın.

### 2.2 Tek hatlı kurulum (önerilir)

```bash
curl -fsSL https://raw.githubusercontent.com/qist/corefusion/main/scripts/install.sh -o install.sh
sudo bash install.sh
```

Veya doğrudan aktarın (etkileşim yok, ekstra işaret yok):

```bash
curl -fsSL https://raw.githubusercontent.com/qist/corefusion/main/scripts/install.sh | sudo bash
```

Senaryo şunları yapacaktır:

1. Dağıtımı, CPU mimarisini ve libc'yi tespit edin, ardından temel paketleri ve çalışma zamanı bağımlılıklarını yükleyin;
2. GitHub Sürümlerinden `corefusion-linux-<arch>`'yi indirin ve eşleşen `.sha256` ile doğrulayın;
3. `/opt/corefusion`'ye yükleyin ve `data/{sqlite,certs,logs,runtime}`'yi oluşturun;
4. `SAFE_PATHS=data` ile yalnızca bir `.env` yer tutucusu yazın — veritabanı yapılandırması yok — böylece ilk başlangıç ​​kurulum sihirbazına girer (`.env`'nin tamamı sihirbaz tarafından oluşturulur);
5. Bir systemd hizmetini yükleyin ve başlatın (veya systemd kullanılamadığında `nohup` altında çalıştırın);
6. Hazır olana kadar bekleyin, ardından URL kurulumunu, belirteci ve dizin yollarını yazdırın.

Örnek çıktı:

```text
==================================================================
 CoreFusion installation complete
------------------------------------------------------------------
 Install dir : /opt/corefusion
 Data dir    : /opt/corefusion/data
 Binary      : /opt/corefusion/corefusion
 Logs        : journalctl -u corefusion -f
------------------------------------------------------------------
 Setup wizard : http://<server-ip>:<random-port>/init?init_token=<token>
 Setup token  : <token>
 Valid for    : 2 hours (see startup log)
 Note         : the token is shown once; after setup this port closes and the panel switches to its normal port.
==================================================================
```

Panel yalnızca `127.0.0.1`'yi dinliyorsa (örneğin ters proxy'nin arkasında), komut dosyası, erişilemeyen URL yerine bir SSH tünel komutu yazdırır.

> Jetonunu mu kaybettin? Başlangıç günlüğünü tekrar okuyun: `journalctl -u corefusion -n 50 --no-pager`
> (systemd olmadan: `tail -n 50 /opt/corefusion/data/logs/panel.log`).

### 2.2.1 Çok dilli çıktı

Kurulum betiği dört arayüz dili içerir: basitleştirilmiş Çince, İngilizce, Farsça (فارسی) ve Rusça (Русский). Sistem `LANG` değerinden otomatik algılar veya açıkça seçebilirsiniz:

```bash
# İngilizce
sudo bash install.sh --lang en

# Ortam değişkeniyle Farsça
sudo CF_LANG=fa bash install.sh

# Boru ile kurulumda seçenek geçirme
curl -fsSL https://raw.githubusercontent.com/qist/corefusion/main/scripts/install.sh | sudo bash -s -- --lang ru
```

Desteklenen değerler: `zh` / `en` / `fa` / `ru`; `--lang=ru` de kabul edilir. Açıkça desteklenmeyen bir değer, kullanılabilir dillerin listesiyle birlikte hata verir.

### 2.3 Sürüm sabitleme / aynalar / çevrimdışı kurulum

```bash
# Pin a version
sudo bash install.sh --version v0.13.0-alpha.2

# GitHub download mirror (used as a URL prefix)
sudo bash install.sh --mirror https://ghproxy.net --version latest

# Use a local binary (offline / intranet / pre-built)
sudo bash install.sh --local ./dist/corefusion-linux-arm64

# Explicit target architecture
sudo bash install.sh --arch arm64 --local ./corefusion-linux-arm64
```

> `--version` olmadan, komut dosyası GitHub Sürümleri listesindeki en yeni girişi kullanır.
> Çoğu sürüm ön sürüm olduğundan `/releases/latest` uç noktası kullanılmaz.
>
> Resmi sürümler yalnızca `amd64` / `arm64`'yi sağlar; diğer mimariler için `--source`'yi veya `--local` ile çapraz derlenmiş bir ikili programı kullanın.

### 2.4 Kaynaktan inşa etmek

```bash
sudo bash install.sh --source --version v0.13.0-alpha.2
```

`--source`, depoyu klonlar (veya komut dosyası içindeyken mevcut ödemeyi kullanır) ve `make panel`'yi çalıştırır. Araç zinciri versiyonları ortama göre seçilir:

| Bağımlılık | Seçim |
|------------|-----------|
| Git | `go.mod`'deki `toolchain` / `go`'den okuyun (şu anda **Go 1.27+**); sistem sürümü çok eskiyse eşleşen takım zinciri `<install-dir>/.toolchain`'ye indirilir |
| Düğüm | **Düğüm 22 LTS** varsayılan olarak (Vite ≥ 18 gerektirir); gerektiğinde eşleşen bir resmi yapı indirilir |
| Düğüm (glibc < 2,28) | `unofficial-builds` glibc-217 yapısını kullanır (örn. CentOS 7) |
| Düğüm (Alp / müsl) | `nodejs` / `npm` dağıtımını kullanır |

Geçersiz kılmalar:

```bash
# Force Go / Node versions
sudo CF_GO_VERSION=1.27.5 CF_NODE_MAJOR=22 bash install.sh --source

# Go toolchain mirror (default https://go.dev/dl)
sudo CF_GO_MIRROR=https://golang.google.cn/dl bash install.sh --source
```

Manuel oluşturma (geliştirme / hata ayıklama):

```bash
git clone https://github.com/qist/corefusion.git
cd corefusion

# Frontend + panel for the current architecture; the frontend is embedded via go:embed
make panel

# Cross-compile
make panel-linux-amd64
make panel-linux-arm64
```

> `go build ./...` bir ikili dosya üretmez. Paneli şununla oluşturun:
> `go build -o corefusion ./cmd/corefusion/` veya `make panel`.

### 2.5 Kurulumcu bayrakları

| Bayrak | Açıklama |
|------|-------------|
| `-d, --dir <PATH>` | Kurulum dizini, varsayılan `/opt/corefusion` |
| `-v, --version <TAG>` | Panel versiyonu, ör. `v0.13.0-alpha.2`; varsayılan `latest` |
| `--binary` | GitHub Sürüm ikili dosyasını kullanın (varsayılan) |
| `--source` | Kaynaktan derleyin ve eşleşen Go/Node'u hazırlayın |
| `--local <FILE>` | Yerel bir ikili dosya kullanın (çevrimdışı kurulum) |
| `--arch <ARCH>` | Hedef mimari: `amd64` / `arm64` / `armv7` / `armv6` / `386` / `ppc64le` / `s390x` / `riscv64` / `loong64` |
| `--mirror <URL>` | GitHub indirme öneki, ör. `https://ghproxy.net` |
| `--repo <URL>` | Kaynak deposu, varsayılan `https://github.com/qist/corefusion.git` |
| `--ref <REF>` | Kaynak dalı/etiketi; varsayılan olarak `--version`'dir, aksi halde `main` |
| `--no-deps` | Sistem bağımlılığı kurulumunu atla |
| `--no-start` | Hizmet dosyasını başlatmadan yükleyin ve yazın |
| `--cpu-info` | Yazdırma ortamı algılama ve eksik bağımlılıklar, ardından çıkın |
| `--lang <LANG>` | Çıktı dili: `zh` / `en` / `fa` / `ru` (varsayılan: sistem `LANG`) |
| `--uninstall` | Hizmeti durdurun ve programı kaldırın (verileri ve `.env`'yi tutar) |
| `--purge` | `--uninstall` ile veri dizinini ve `.env`'yi de kaldırın. |

İlgili ortam değişkenleri:

| Değişken | Amaç |
|----------|---------|
| `CF_GO_VERSION` | Kaynak derlemeler için gereken Go sürümünü geçersiz kılın |
| `CF_NODE_MAJOR` | Düğümün ana sürümünü geçersiz kıl (varsayılan 22) |
| `CF_GO_MIRROR` | Araç zinciri indirme aynasına git |
| `CF_SKIP_DEPS` | Bağımlılık kurulumunu atlamak için Agent yükleme komut dosyasında `1` olarak ayarlayın |
| `CF_LANG` | Kurulum betiği çıktı dilini geçersiz kıl (`--lang` ile aynı) |

### 2.6 Dizin düzeni

```text
/opt/corefusion/
├── corefusion                 # panel binary
├── corefusion.bak             # previous version (when upgrading)
├── .env                       # environment file (0600, generated by the setup wizard)
├── .toolchain/                # Go / Node downloaded for source builds
└── data/                      # data directory (SAFE_PATHS)
    ├── sqlite/app.db          # default SQLite database
    ├── certs/                 # certificates and keys
    ├── logs/                  # runtime logs (without systemd)
    ├── runtime/               # core runtime config and state
    ├── binaries/              # downloaded xray / sing-box cores
    └── proxy/                 # proxy (Caddy/Nginx) sites and config
```

> Yükleyici yalnızca `sqlite/`, `certs/`, `logs/` ve `runtime/`'yi oluşturur; `binaries/`, `proxy/`, `rules/` ve diğerleri panel tarafından veya çekirdekler takıldığında oluşturulur.
> Veri dizini `.env`'deki `SAFE_PATHS`'den gelir ve mutlak bir yol olabilir. Kurulum dizinini değiştirmek mevcut verileri taşımaz.

### 2.7 Hizmet yönetimi

systemd (çoğu dağıtım):

```bash
systemctl status corefusion --no-pager
systemctl restart corefusion
systemctl stop corefusion
journalctl -u corefusion -f          # follow logs
journalctl -u corefusion -n 100      # last 100 lines
```

systemd olmadan (konteynerler vb.):

```bash
cd /opt/corefusion && ./corefusion            # foreground (debugging)
cd /opt/corefusion && setsid nohup ./corefusion >> data/logs/panel.log 2>&1 &
tail -f data/logs/panel.log
```

> systemd olmadan süreç önyüklemede başlamaz; yeniden başlatmanın ardından manuel olarak yeniden başlatın (veya yükleyiciyi yeniden çalıştırın).

### 2.8 Yükseltme ve kaldırma

Yükseltme (verileri ve yapılandırmayı korur):

```bash
sudo bash install.sh --version <new-version>
```

Komut dosyası hizmeti durdurur, eski ikili dosyayı `corefusion.bak` olarak yedekler, değiştirir ve yeniden başlatır. Mevcut `.env` ve `data/`'ye dokunulmaz.

Kaldırma:

```bash
# Remove the program only; keep data and .env
sudo bash install.sh --uninstall

# Also remove the data directory and .env (irreversible)
sudo bash install.sh --uninstall --purge
```

### 2.9 Ortam dosyası (.env)

Yapılandırma kaydedildiğinde `.env`'nin tamamı **kurulum sihirbazı tarafından oluşturulur** (veritabanı DSN'si, veri dizini, sırlar, bağlantı noktaları, etki alanları).

Yeni kurulumda yükleyici yalnızca bu yer tutucuyu yazar, böylece panel mevcut `data/`'yi başlatılmış kurulum zannetmez ve sihirbazı atlar:

```env
SAFE_PATHS=data
```

> Tam bir yapılandırma (`CF_DB_DSN` ile `.env`) zaten mevcut olduğunda, yükleyici hiçbir zaman bunun üzerine yazmaz ve sihirbaz bir daha görünmez.
> `.env`'de daha sonra manuel olarak yapılan düzenlemeler, yeniden başlatmanın ardından etkili olur.

#### Veri dizini

```env
SAFE_PATHS=data                       # relative to the install directory
# SAFE_PATHS=/srv/corefusion-data     # or an absolute path
```

#### Veritabanı

`CF_DB_DSN` önekle algılanır:

| Önek | Veritabanı |
|--------|----------|
| ayarlanmamış veya çıplak bir yol | SQLite (varsayılan) |
| `sqlite://` | SQLite (açık) |
| `mysql://` | MySQL |
| `postgres://` / `postgresql://` | PostgreSQL |

`CF_DB_TYPE` isteğe bağlıdır ve algılamayı geçersiz kılar (`sqlite` / `mysql` / `postgres`).

> **Şifreli şifreler**: Panel tarafından yazılan MySQL/PostgreSQL DSN'ler (kurulum sihirbazı/veritabanı anahtarı)
> `.env`'de `enc:...` (AES-256-GCM, anahtar `CF_SECRET_ENCRYPTION_KEY`) olarak depolanır.
> Okunduğunda şifreleri çözülür; Elle yazılan düz metin DSN'ler hâlâ çalışıyor.

**SQLite (varsayılan)**

```env
# Bare path (detected as SQLite)
CF_DB_DSN=data/sqlite/app.db

# Or explicit sqlite:// prefix
CF_DB_DSN=sqlite:///opt/corefusion/data/sqlite/app.db
```

**MySQL**

```env
# Format: mysql://user:password@tcp(host:port)/dbname?params
CF_DB_DSN=mysql://corefusion:password@tcp(127.0.0.1:3306)/corefusion?charset=utf8mb4&parseTime=True&loc=Local
```

**PostgreSQL**

```env
# Format: postgres://user:password@host:port/dbname?params
CF_DB_DSN=postgres://corefusion:password@127.0.0.1:5432/corefusion?sslmode=disable
```

#### Bağlantı havuzu (isteğe bağlı, MySQL/PostgreSQL)

```env
CF_DB_MAX_OPEN_CONNS=100
CF_DB_MAX_IDLE_CONNS=10
```

#### Diğer ortak değişkenler

| Değişken | Açıklama |
|----------|-------------|
| `JWT_SECRET` | Oturum açma belirteçleri için imzalama anahtarı; eksik olduğunda ilk çalıştırmada oluşturulur |
| `CF_SECRET_ENCRYPTION_KEY` | Hassas ayarları şifrelemek için anahtar (AES-256-GCM); eksik olduğunda oluşturulan |
| `CF_INITIAL_ADMIN_PASSWORD` | İlk yönetici şifresi (isteğe bağlı; rastgele ve varsayılan olarak bir kez yazdırılır) |
| `CF_NETWORK_TRAFFIC_ACCOUNTING` | İptables tabanlı trafik hesaplamasını devre dışı bırakmak için `0` olarak ayarlayın (varsayılan olarak etkindir) |
| `CF_NETWORK_TRAFFIC_TABLE` | Trafik muhasebesi için kullanılan iptables tablosu, varsayılan `corefusion` |

> Gerçek istemci IP'lerini çözümlemeye yönelik güvenilir ters proxy'ler artık ortam değişkenleri aracılığıyla yapılandırılmamaktadır;
> **Ayarlar → Ağ**'da `panel.trusted_proxies` olarak depolanırlar.

> `.env` sırlar içerir; panel başlangıçta izinlerini `0600`'ye sıkılaştırıyor. Asla taahhüt etmeyin.

### 2.10 Docker (isteğe bağlı, kendi görüntünüzü oluşturun)

Depo bir Docker dosyası **göndermez**. Kapsayıcı hale getirirseniz kendi görselinizi yazın. Referans çağrısı:

```bash
docker run -d \
  --name corefusion \
  -p 8080:8080 \
  -p 18443:18443 \
  -v /opt/corefusion/data:/app/data \
  -v /opt/corefusion/.env:/app/.env \
  <your-image>
```

> Container'larda genellikle systemd ve `nft` ayrıcalıkları bulunmadığından hız sınırlama/güvenlik duvarı özellikleri çalışmaz.
> Çalışma dizini, ilgili `SAFE_PATHS`'nin tabanıyla eşleşmelidir.

### 2.11 Kurulum sonrası kontroller

```bash
# Environment and dependencies
sudo bash install.sh --cpu-info

# Service state
systemctl is-active corefusion

# Startup log (contains the setup URL and token)
journalctl -u corefusion -n 50 --no-pager

# Listening ports
ss -lntp | grep corefusion
```

---

## 3. İlk konfigürasyon

İlk başlangıçta, `.env`'nin geçerli bir `CF_DB_DSN`'si yoksa panel henüz yönetici kullanıcı arayüzüne **sunmaz**. Şunu başlatır:
**kurulum sihirbazı** 18000-18999'da rastgele bir bağlantı noktasında tek seferlik bir jetonla çalışır ve kaydedildiğinde normal panel olarak yeniden başlar.

### 3.1 Kurulum sihirbazını açın

1. Kurulum sihirbazı URL'yi yükleyici çıkışından kopyalayın; veya
2. Sunucudaki günlüğü okuyun:

```bash
journalctl -u corefusion -n 50 --no-pager | grep -A2 'setup wizard'
```

```text
CoreFusion first start - setup wizard
  URL   : http://<ip>:<port>/init?init_token=<token>
  Token : <token>
  Valid : 2h0m0s
```

3. URL'yi bir tarayıcıda açın (zaten `init_token` içeriyor, dolayısıyla manuel girişe gerek yok).

> Sihirbaz oturumu varsayılan olarak **2 saat** sonra sona erer. Süresi dolarsa veya panel yeniden başlarsa yenisini alın.
> Tokenı paylaşmayın. Kurulum tamamlandığında bu bağlantı noktası kapanır ve yalnızca normal panel bağlantı noktası kalır.

### 3.2 Sihirbaz adımları

Sihirbazın 8 adımı vardır. İstediğiniz zaman geri dönebilirsiniz; son kaydetmeye kadar hiçbir şey yapılmaz.

| Adım | İçerik |
|------|---------|
| 1. Veritabanı | SQLite / MySQL / PostgreSQL |
| 2. Veri dizini | `.env`'de `SAFE_PATHS`'ye yazıldı |
| 3. Şifreleme anahtarları | `JWT_SECRET` ve `CF_SECRET_ENCRYPTION_KEY` |
| 4. Çekirdekler | Hangi proxy çekirdeklerinin kurulacağı |
| 5. Vekil | Yok / Nginx / Caddy |
| 6. Siteler ve alanlar | Bağlantı noktaları, web yolu, yönetici hesabı, para birimi, saat dilimi, alanlar |
| 7.HTTPS sertifikası | Devre dışı / yükleme / ACME |
| 8. Özet | `.env`'yi inceleyin, indirin, kaydedin ve yeniden başlatın |

#### Adım 1: Veritabanı

- **SQLite (yerel dosya, önerilir)**: `<data-dir>/sqlite/app.db`'yi kullanmak için yolu boş bırakın.
- **MySQL / PostgreSQL**: ana bilgisayar, bağlantı noktası, kullanıcı, parola, veritabanı adı; PostgreSQL ayrıca `disable` / `require` / `verify-full`'yi de destekler.
- Doğrulamak için “Bağlantıyı test et”e tıklayın. Kaydetme aynı zamanda bunu test eder. Eksik bir veritabanı otomatik olarak oluşturulur (hesapta `CREATE` yoksa manuel olarak oluşturun).
- DSN (şifre dahil) `.env`'ye yazılmadan önce şifrelenir.

#### Adım 2: Veri dizini

Veritabanını, sertifikaları, günlükleri, çekirdek ikili dosyaları ve daha fazlasını saklar; bu `.env`'deki `SAFE_PATHS`'dir. Varsayılanlar `data`'dir
(kurulum dizinine göre); mutlak bir yola izin verilir. **Daha sonra rastgele değiştirmeyin** veya paneli
mevcut veritabanını ve sertifikaları bulamayacaktır.

#### 3. Adım: Şifreleme anahtarları

- `JWT_SECRET`: oturum açma belirteçlerini imzalar.
- `CF_SECRET_ENCRYPTION_KEY`: hassas ayarları (veritabanı şifresi, DNS kimlik bilgileri, şarj kartları vb.) şifreler.
- Her ikisi de varsayılan olarak rastgele oluşturulur; yeni değerler elde etmek için “Yeniden Oluştur”u kullanın. **`CF_SECRET_ENCRYPTION_KEY`'yi değiştirmek, önceden şifrelenmiş verileri okunamaz hale getirir** — yedekleyin.

#### Adım 4: Çekirdekler

Yüklenecek proxy çekirdeklerini seçin:

- **Xray**: VLESS / VMess / Trojan / Shadowsocks, vb.;
- **sing-box**: Hysteria2 / TUIC ve diğer QUIC protokollerini taşır.

Kaydedip yeniden başlattıktan sonra panel otomatik olarak indirilir ve kurulur. “Protokoller” altında sürümleri ve durumu kontrol edin. Bunları daha sonra da yükleyebilirsiniz.

#### Adım 5: Vekil

- **Yok**: panel müşterilere doğrudan hizmet verir (TLS'yi kendiniz halledin).
- **Nginx / Caddy**: panel, ters proxy ve sertifika barındırmayı yönetir; birini seç.

Bir proxy seçildiğinde panel genellikle yalnızca `127.0.0.1`'yi dinler ve proxy dış dünyaya hizmet eder.

#### 6. Adım: Siteler ve alanlar

| Alan | Açıklama |
|-------|-------------|
| Adresi dinle | Otomatik olarak türetilmiştir: proxy → `127.0.0.1`; sertifikalı vekil yok → `0.0.0.0`; aksi takdirde → `127.0.0.1` |
| Panel bağlantı noktası | Varsayılan olarak rastgele ücretsiz bağlantı noktası; isterseniz değiştirin (ör. 8080) |
| Abonelik bağlantı noktası | İsteğe bağlı; `0` panel bağlantı noktasını paylaşıyor |
| Portal bağlantı noktası | İsteğe bağlı; `0` panel bağlantı noktasını paylaşıyor |
| Web yolu | Yönetici kullanıcı arayüzü yolu, varsayılan `/`; API her zaman `/api/v1`'dir |
| Yönetici kullanıcı adı / şifresi | Varsayılan olarak rastgele; **şifre özette yalnızca bir kez gösterilir** - kaydedin |
| Erişim modu | “Etki alanı” veya “IP” |
| Varsayılan para birimi/saat dilimi | Yeniden yükleme kartları ve bakiyeler için varsayılan para birimi; panel ve proxy günlüğü saat dilimi (ör. `Asia/Shanghai`) |
| Ters proxy'ye ekle | Proxy ve etki alanları ayarlandığında tek seferde panel/abonelik/portal etki alanlarını proxy'ye ekleyin |

Etki alanlarının girilmesi, ilgili ayarların (geri arama etki alanı, abonelik uç noktası, paylaşım etki alanı) yazılmasına neden olur.

#### Adım 7: HTTPS sertifikası

| Kaynak | Açıklama |
|--------|-------------|
| HTTPS'yi devre dışı bırakın | Panel/portal/abonelikler HTTP (güvenli değil) üzerinden sunulur; ters proxy ile daha sonra proxy tarafına TLS ekleyebilirsiniz |
| Sertifikayı yükle | PEM sertifikasını ve PEM özel anahtarını yapıştırın (yalnızca etki alanı erişim modu) |
| ACME | Bu adımı gönderdiğinizde sertifikayı verir; etki alanı ve IP sertifikalarını destekler |

ACME seçenekleri:

- **CA**: Let's Encrypt, Let's Encrypt (Hazırlama), ZeroSSL, Google Genel CA;
- **Hedef**: yapılandırılmış alan adı veya genel IP (IP sertifikaları yalnızca Let's Encrypt tarafından desteklenir);
- **Zorluk**: `HTTP-01` (80 numaralı bağlantı noktasını gerektirir) veya `DNS-01` (joker karakterler, 80 numaralı bağlantı noktası yok; bir DNS sağlayıcısı seçin ve kimlik bilgilerini girin);
- **Anahtar türü/boyutu**: ECC 256/384 veya RSA 2048/3072/4096;
- **Google Genel CA**: EAB gerektirir; Anahtar Kimliğini / HMAC'yi manuel olarak girin veya bir GCP hizmet hesabı kullanın.

> IP sertifikaları, ana bilgisayarın genel IPv4/IPv6'sını otomatik olarak algılar. Bir proxy'nin arkasında, algılanan adres proxy çıkışı değil, ana bilgisayarın adresidir.
> Alan adı sertifikaları daha sonra “Sertifikalar” altında da verilebilir.

#### Adım 8: Özet

Veritabanını, veri dizinini, çekirdekleri, proxy'yi, bağlantı noktalarını, yönetici hesabını, etki alanlarını ve `.env` anahtarlarını (gizli sırlar maskelenmiş) gösterir ve şunları sağlar:

- **.env dosyasını indirin**: yapılandırmanın bir kopyasını saklayın;
- **Erişim URL'leri**: panel/abonelik/portal için nihai URL'ler; HTTP güvensiz olarak işaretlendi.

Veritabanını ve `.env`'yi yazmak için “Kaydet ve yeniden başlat”a tıklayın; panel normal hizmete yeniden başlar.

### 3.3 Kurulumdan sonra

- Hizmet yeniden başlatılır ve başlangıç günlüğü son dinleme adresini yazdırır;
- Yönetici hesabıyla oturum açın, özetten URL'ye ve web yoluna erişin;
- Parola kaybolursa `./corefusion admin reset-password` ile çevrimdışı olarak sıfırlayın.

### 3.4 Etkileşimli olmayan CLI kurulumu

Otomasyon veya başsız sunucular için yapılandırmayı yazın ve kurulumun tamamlandı olarak işaretlenmesini doğrudan yapın:

```bash
cd /opt/corefusion
./corefusion init \
  --db-type sqlite \
  --data-dir /opt/corefusion/data \
  --admin-username admin \
  --admin-password 'change-me-strong-password' \
  --listen-port 8080 \
  --proxy-type caddy \
  --cores xray,singbox \
  --panel-domain panel.example.com \
  --tls-mode acme --tls-domain panel.example.com --tls-email you@example.com
```

Ortak bayraklar:

| Bayrak | Açıklama |
|------|-------------|
| `--db-type` / `--db-host` / `--db-port` / `--db-user` / `--db-password` / `--db-name` | Veritabanı ayarları |
| `--data-dir` | Veri dizini; varsayılan olarak `SAFE_PATHS` veya `./data`'dir |
| `--listen-port` / `--subscription-port` / `--portal-port` / `--web-path` | Bağlantı noktaları ve web yolu (varsayılan olarak rastgele boş bağlantı noktası) |
| `--admin-username` / `--admin-password` | Yönetici hesabı (atlandığında rastgele şifre bir kez yazdırılır) |
| `--proxy-type` | `none` / `nginx` / `caddy` |
| `--cores` | Yüklenecek çekirdekler, virgülle ayrılmış: `xray,singbox` |
| `--panel-domain` / `--subscription-domain` / `--portal-domain` | Etki alanları (ayarlara yazılır ve ters proxy'ye eklenir) |
| `--tls-mode` | `none` / `self_signed` / `acme` |
| `--tls-domain` / `--tls-email` / `--tls-challenge` / `--tls-key-type` / `--tls-key-bits` | Sertifika seçenekleri (`--tls-challenge` varsayılan olarak `http-01`'dir; `dns-01`'nin `--tls-dns-channel` veya `--tls-dns-provider`'ye ihtiyacı vardır) |
| `--tls-ip-cert` | Algılanan genel IP için bir sertifika verin (yalnızca Let's Encrypt) |
| `--acme-provider` | `lets_encrypt` / `lets_encrypt_staging` / `zerossl` / `google` |
| `--currency` / `--timezone` | Varsayılan para birimi/saat dilimi |

Tam liste: `./corefusion init --help`. Ardından hizmeti başlatın:

```bash
systemctl restart corefusion
```

### 3.5 Kurulum sihirbazını yeniden çalıştırma

Baştan başlamak için (`.env`'yi ve kurulum işaretlerini kaldırır, **verileri tutar**):

```bash
cd /opt/corefusion
./corefusion reset-init      # type RESET to confirm
systemctl restart corefusion
```

Ardından yeni URL'yi ve başlangıç günlüğündeki belirteci kullanın.

> Yeniden başlatma, yönetici hesabını ve web yolunu sıfırlar. Eski veritabanındaki iş verileri kalır ancak oturum açma hesabını yeniden oluşturmanız gerekebilir.
>
> `reset-init`, `.env`'nin tamamını (`SAFE_PATHS` dahil) siler. **Veri dizininiz özel bir mutlak yolsa**, yeniden başlatmadan önce `SAFE_PATHS`'yi geri yükleyin, aksi takdirde panel `./data`'de görünür:
>
> ``'bash
> printf 'SAFE_PATHS=/srv/corefusion-data\n' > /opt/corefusion/.env
> ``'

### 3.6 İlk oturum açmanın ardından güvenlik

#### Başlangıç şifresini değiştirin

1. Sağ üst köşedeki kullanıcı adına tıklayın;
2. “Şifreyi değiştir” seçeneğini seçin;
3. Eski ve yeni şifreleri girin;
4. “Kaydet”e tıklayın.

#### Dinamik kodla oturum açmayı etkinleştir (2FA / TOTP)

> Etkinleştirildiğinde hesap **dinamik kodlu oturum açma** moduna geçer: statik şifre artık kabul edilmez; sadece
> Kimlik doğrulama uygulamanızdaki 6 haneli kod (veya tek seferlik kurtarma kodu) çalışır. Bu, oturum açma parolasının **yerine geçer** — öyledir
> “şifre + kod” iki adımlı doğrulama değil.

1. Sağ üst köşedeki kullanıcı adına tıklayın;
2. “2FA'yı Etkinleştir”i seçin;
3. Geçerli şifreyi girin (yalnızca bu değişikliği onaylamak için);
4. QR kodunu bir kimlik doğrulama uygulamasıyla (Google Authenticator / 1Password vb.) tarayın veya anahtarı manuel olarak girin;
5. Etkinleştirmeyi tamamlamak için 6 haneli kodu girin;
6. **Kurtarma kodlarını hemen kaydedin** (indirilebilir); her kod bir kez çalışır.

Daha sonra oturum açma:

- Kullanıcı adını girdikten sonra şifre alanı “6 haneli kod/kurtarma kodu” olur ve buton “Kodla Giriş Yap” olur;
- Geçerli kodu girin; **statik şifre artık kullanılamaz**;
- Kurtarma kodları tek kullanımlıktır; Bittiğinde bunları “Dinamik Kod Girişi (2FA)” altında yeniden oluşturun (bu, tüm eski kodları geçersiz kılar).

Kimlik doğrulayıcı kaybolursa ve kurtarma kodları tükenirse sunucuda çevrimdışı 2FA'yı devre dışı bırakın:

```bash
cd /opt/corefusion
./corefusion admin 2fa reset -u admin    # disables 2FA, clears the secret and recovery codes, revokes all sessions
```

> Kimlik doğrulayıcıya veya kurtarma kodlarına sahip olmayan tek bir süper yönetici, web kullanıcı arayüzü aracılığıyla kurtarma işlemi gerçekleştiremez ve yukarıdaki komutu kullanmalıdır.

#### Passkey'yi kaydedin (isteğe bağlı)

1. Sağ üst köşedeki kullanıcı adına tıklayın;
2. “Passkey yönetimi”ni seçin;
3. “Yeni Passkey'yi kaydet”e tıklayın;
4. Cihaz istemini tamamlayın.

> **Not**: Geçiş anahtarları HTTPS veya localhost gerektirir ve alan adının izin verilenler listesinde olması gerekir.

---

## 4. Kontrol Paneli

Kontrol paneli, çok düğümlü kümeye genel bakışı ve düğüm durumunu gösterir.

### 4.1 Ne gösterir?

- **Sistem bilgisi**: ana bilgisayar adı, işletim sistemi sürümü, çalışma süresi, CPU, bellek
- **Trafik**: gerçek zamanlı bant genişliği, toplam trafik, aylık trend
- **Düğüm durumu**: çevrimiçi/çevrimdışı/düşük sayımlar, yüksek yüklü düğümler
- **Uyarılar**: çevrimdışı düğümler, düzeyi düşürülmüş düğümler, süresi yakında dolacak sertifikalar, kota uyarıları
- **Küme kaynakları**: ortalama CPU, bellek, disk ve aktarım hızı

### 4.2 Eylemler

- Bir düğümün ayrıntı sayfasını açmak için "Ayrıntıları görüntüle"ye tıklayın
- Canlı durumunu görmek için bir düğüm kartını tıklayın
- Duruma, türe veya bölgeye göre daraltmak için filtreleri kullanın

---

## 5. Düğüm yönetimi

Düğüm yönetimi CoreFusion'nin temelidir ve çeşitli düğüm türlerini destekler.

### 5.1 Düğüm türleri

| Tür | Açıklama |
|------|-------------|
| Bağımsız (`standalone`) | Bağımsız bir çekirdek düğüm |
| Ayna (`mirror`) | Bir hedef düğümden yapılandırmayı senkronize eder |
| Birincil/yedek (`primary_backup`) | Birbirine yük devreden ortak düğümler |
| Zincir girişi (`chain_entry`) | Zincirlenmiş bir proxy'nin giriş düğümü |
| Zincir çıkışı (`chain_exit`) | Zincirlenmiş bir proxy'nin çıkış düğümü |

### 5.2 Düğüm ekleme

1. “Düğüm yönetimi” ni açın;
2. “Sunucu ekle”ye tıklayın;
3. Temel bilgileri doldurun:
   - Ad, ör. `HongKong-HK01`
   - Ana bilgisayar adı: IP veya etki alanı
   - Bölge, ör. `HK`, `JP`, `US`
   - Düğüm türü;
4. Aracı bağlantı noktasını ayarlayın (varsayılan `18443`);
5. Desteklenen çekirdekleri seçin;
6. “Kaydet”e tıklayın.

### 5.3 Tek tıklamayla dağıtım

1. Düğümü ekledikten sonra “Bootstrap komutu”na tıklayın;
2. Kurulum komutunu kopyalayın;
3. Uzak düğümde kök olarak çalıştırın:

   ```bash
   curl -fsSL https://your-panel/install.sh | bash
   ```

4. Aracı otomatik olarak indirilir, kaydedilir ve başlar.

### 5.4 Düğüm ayrıntıları

Ayrıntı sayfasını açmak için bir düğüme tıklayın:

- **Genel Bakış**: temel bilgiler, Temsilci durumu, sertifika durumu
- **Protokol merkezi**: temel süreç yapılandırması
- **Proxy**: Caddy / Nginx yapılandırması
- **Temel işlemler**: başlat / durdur / yeniden başlat
- **WARP**: WARP yapılandırması
- **Doğrudan alanlar**: alan yönetimi
- **Oran sınırı**: trafiği şekillendirme
- **Sertifikalar**: sertifika bağlamaları
- **Çevrimiçi kullanıcılar**: gerçek zamanlı çevrimiçi kullanıcılar

### 5.5 Düğüm eylemleri

- **Yapılandırmayı senkronize et**: panel yapılandırmasını düğüme aktarın
- **Yeniden bağlanmaya zorla**: Agent bağlantısını kesip yeniden bağlanın
- **Acenteyi Güncelle**: Bir Aracı güncellemesi gönderin
- **Sertifikayı iptal et**: düğümün Aracı sertifikasını iptal et

---

## 6. Protokol merkezi

Protokol merkezi, çekirdek işlemler için protokol parametrelerini yapılandırır.

### 6.1 Desteklenen çekirdekler

| Çekirdek | Açıklama |
|------|-------------|
| Xray-çekirdek | VLESS / VMess / Trojan / Shadowsocks ve daha fazlası |
| sing-box | TUN destekli modüler evrensel proxy platformu; Hysteria2 / TUIC ve diğer QUIC protokollerini taşır |

### 6.2 Konfigürasyon modülleri

- **Günlük**: erişim günlüğü, hata günlüğü, çıkış modu
- **API**: gRPC API dinleme adresi, hizmet listesi
- **DNS**: DNS sunucuları, sorgu stratejisi, statik ana bilgisayarlar
- **Yönlendirme**: yönlendirme kuralları, yük dengeleme, VLESS yönlendirme
- **Gelenler**: gelen listesi, protokol ayarları, aktarım, TLS
- **Gidenler**: giden listesi, protokol ayarları, aktarım
- **Aktarım**: WebSocket, gRPC, XHTTP, TUN vb.
- **TLS**: sertifikalar, ALPN, uTLS parmak izi, ECH
- **Reality**: hedef yönetimi, anahtar çiftleri, kısa kimlikler
- **Durum kontrolü**: URL probu, aralık, eşzamanlılık
- **FakeDNS**: IP havuzu, havuz boyutu
- **Metrics**: Prometheus metrikleri dinleyicisi
- **Politika**: bağlantı zaman aşımları, arabellekler, istatistikler
- **Coğrafi veriler**: otomatik güncelleme, kaynak dosyaları
- **Sürüm sınırları**: minimum / maksimum sürüm

### 6.3 Gelen yapılandırma

1. Çekirdeği ve protokolü seçin;
2. Temel bilgileri doldurun:
   - Etiket
   - Adresi ve bağlantı noktasını dinle
   - Protokol türü
3. Protokol parametrelerini ayarlayın (UUID, şifre vb.);
4. Aktarımı seçin (TCP, WebSocket, gRPC, vb.);
5. TLS'yi (sertifika, ALPN) yapılandırın;
6. Kaydedin ve çekirdeğe uygulayın.

### 6.4 Giden yapılandırma

1. Giden ekleyin;
2. Etiketi ve protokolü ayarlayın;
3. Sunucu adresini ve bağlantı noktasını ayarlayın;
4. Aktarımı ve TLS'yi yapılandırın;
5. Kaydet ve uygula.

### 6.5 Bağlantıları paylaşma

1. Gelen listesinde "Bağlantıyı paylaş"ı tıklayın;
2. Bir format seçin:
   - URI'yi paylaşın
   - Mihomo YAML
   - sing-box JSON
   - Xray JSON
3. Bağlantıyı kopyalayın veya bir QR kodu oluşturun.

---

## 7. Proxy yönetimi

Proxy yönetimi Caddy veya Nginx ters proxy'yi yapılandırır.

### 7.1 Proxy türleri

- **Caddy**: otomatik HTTPS, HTTP/2, HTTP/3
- **Nginx**: yüksek performans, esnek yapılandırma

### 7.2 Saha konfigürasyonu

1. “Proxy yönetimi” ni açın;
2. Proxy türünü seçin (Caddy / Nginx);
3. Bir site ekleyin:
   - Etki alanı
   - Yukarı akış
   - Yerel liman
4. Kaydet.

### 7.3 Alanlar

- **yukarı akış**: arka uç adresi, ör. `127.0.0.1:8082`
- **local_port**: yerel dinleme bağlantı noktası, ör. `8443`
- **portal_forward**: portal yönlendirme kuralı
- **subscription_forward**: abonelik yönlendirme kuralı

---

## 8. Yönlendirme kaynakları

Yönlendirme kaynakları abonelik yönlendirme kurallarını yönetir.

### 8.1 Özellikler

- Yönlendirme kurallarını yönet
- Yönlendirme politikalarını yapılandırma
- Çoklu yönlendirme formatlarını destekleyin

### 8.2 Kullanım

1. “Yönlendirme kaynakları”nı açın;
2. Bir yönlendirme kuralı ekleyin;
3. Kural adını, türünü ve koşullarını ayarlayın;
4. Kaydet ve uygula.

---

## 9. Abonelik merkezi

Abonelik merkezi abonelik bağlantılarını ve uç noktalarını yönetir.

### 9.1 Abonelik formatları

1. “Abonelik merkezi” → “Abonelik formatları”nı açın;
2. Bir biçim ekleyin:
   - Biçim adı
   - Çıkış formatı (Base64, YAML, JSON, vb.)
   - Dahil edilen protokoller
3. Kaydet.

### 9.2 Abonelik uç noktaları

1. “Abonelik merkezi” → “Abonelik uç noktaları”nı açın;
2. Bir uç nokta ekleyin:
   - Uç nokta adı
   - Adresi dinle
   - Yol
   - İlgili format
3. Kaydet.

### 9.3 Abonelik bağlantıları

- `http://your-domain/sub/:token` adresinden bir abonelik alın
- Format seçmek için `?format=yaml` gibi parametreleri kullanın

---

## 10. Müşteri yönetimi

Müşteri yönetimi son kullanıcıları yönetir.

### 10.1 Müşteri ekleme

1. “Müşteri yönetimi” ni açın;
2. "Müşteri ekle"ye tıklayın;
3. Doldurun:
   - E-posta
   - Şifre
   - Planı
   - Trafik kotası
   - sona erme süresi
4. Kaydet.

### 10.2 Müşteri eylemleri

- **Düzenle**: müşteri ayrıntılarını değiştir
- **Askıya al**: hizmeti askıya al
- **Devam ettir**: Askıya alınan bir müşteriyi geri yükleme
- **Trafiği sıfırla**: kullanılan trafiği sıfırla
- **Şifreyi sıfırla**: müşterinin şifresini sıfırlayın
- **Yenile**: son kullanma tarihini uzat
- **Yükseltme / düşürme**: planı değiştirin
- **Sil**: müşteriyi kaldırın

### 10.3 Abonelik ayrıntıları

Görmek için bir müşteriyi tıklayın:

- Abonelik bağlantısı
- Trafik kullanımı
- Sipariş geçmişi
- İşlem geçmişi

---

## 11. Plan yönetimi

Plan yönetimi sattığınız planları yapılandırır.

### 11.1 Plan şablonları

1. “Plan yönetimi” → “Plan şablonları”nı açın;
2. Bir şablon ekleyin:
   - Şablon adı
   - Kategori (standart / premium / kurumsal / deneme)
   - Otomatik yenile
   - Deneme ayarları
3. Kaydet.

### 11.2 Plan çeşitleri

1. Şablonun altına bir varyant ekleyin:
   - Varyant adı (ör. aylık, yıllık)
   - Fiyat (cent cinsinden)
   - Süre (gün)
   - Trafik (GB, `0` = sınırsız)
   - Eşzamanlılık sınırı
   - Hız sınırı (Mbps)
2. Kaydetmek.

### 11.3 Trafik eklentileri

1. “Trafik eklentileri”ni açın;
2. Bir eklenti ekleyin:
   - İsim
   - Fiyat
   - Trafik
   - Son kullanma modu (günleri uzat / planla aynı)
3. Kaydet.

### 11.4 Kuponlar

1. “Kuponlar”ı açın;
2. Kupon ekleyin:
   - Kod
   - Tür (yüzde / sabit tutar)
   - İndirim değeri
   - Kullanım sınırı
   - Geçerlilik süresi
   - Uygulanabilir planlar
3. Kaydet.

---

## 12. Şarj yönetimi

Şarj yönetimi, şarj sitelerini, siparişleri ve şarj kartlarını yönetir.

> **Ödeme entegrasyonu notu**
>
> Panel, **ödeme kuruluşlarına doğrudan bağlanmaz** (Alipay, WeChat Pay, Stripe, bankalar vb.); yalnızca bir **aracı ödeme platformuna** bağlanır. Aracı platform ilgili ödeme kuruluşunu entegre eder, sözleşme, tahsilat ve mutabakatı yürütür ve ödeme sonuçlarını [ödeme geri çağırma API'si](api/charge-callback.md) aracılığıyla panele bildirir.
>
> Aracı ödeme platformu **bu projenin kapsamında değildir ve işletmeci tarafından geliştirilmelidir (veya kendi sistemi kullanılmalıdır)**. Dağıtımdan sonra «Şarj siteleri» altında kaydedin ve alan adı doğrulamasını tamamlayarak şarj girişi olarak kullanın; buradaki şarj sitesi ödeme kuruluşunun kendisi değil, aracı platformdur. Ödeme yetkinlikleri, ücretler, mutabakat ve uyumluluk, işletmeci ile ödeme kuruluşu ve aracı platform arasında kararlaştırılır ve işletmeci tarafından üstlenilir.

### 12.1 Şarj yerleri

1. “Şarj yönetimi” → “Şarj siteleri”ni açın;
2. Bir site ekleyin:
   - Site adı
   - Bu aracı platformun desteklediği ödeme yöntemleri (Alipay, WeChat, USDT, Stripe vb.; platformun ödeme kuruluşlarıyla entegrasyonu üzerinden sunulur)
   - Para birimi
   - Döviz kuru
3. Kaydet;
4. Siteyi doğrulayın (HTTP dosyası veya DNS doğrulaması).

### 12.2 Siparişleri yeniden yükleme

- Tüm yükleme siparişlerini görüntüle
- Duruma göre filtrele (beklemede, ödendi, başarısız oldu, süresi dolmuş, geri ödeme yapıldı)
- Sipariş ayrıntılarını görüntüle
- Yönetici geri ödemeleri yapın

### 12.3 Yeniden yükleme kartları

1. “Kartları şarj et” seçeneğini açın;
2. Bir kart ekleyin:
   - Tür (tutar, plan, trafik eklentisi)
   - Nominal değer / plan
   - sona erme süresi
3. Veya toplu olarak oluşturun:
   - Adet (1000'e kadar)
   - Kart numarası öneki
   - Kart numarası formatı
4. CSV olarak dışa aktarın.

### 12.4 Bakiye yönetimi

- Müşteri bakiyelerini görüntüleyin
- Bakiyeleri ayarlayın (artırma / azaltma / ayarlama)
- Dondurulmuş miktarları ayarlayın
- İşlem geçmişini görüntüle

### 12.5 Bayiler

- Bayi ekle
- Komisyon oranlarını ayarlayın
- Birikmiş komisyonu görüntüle
- Komisyonu geri çekme

---

## 13. Bilet yönetimi

Bilet yönetimi müşteri destek isteklerini yönetir.

### 13.1 Bilet listesi

- Tüm biletleri görüntüle
- Duruma göre filtrele (açık, yanıtlandı, kapalı)
- Kategoriye göre filtrele (teknik, faturalandırma, hesap, diğer)
- Önceliğe göre filtrele (düşük, normal, yüksek, acil)

### 13.2 Bilet işlemleri

- **Görüntüle / yanıtla**: bir bildirim açın ve yanıtlayın
- **Ata**: Yöneticiye bir bilet atayın
- **Kapat**: çözümlenmiş bir bildirimi kapatır

---

## 14. Sertifika yönetimi

Sertifika yönetimi TLS sertifikalarını yönetir.

### 14.1 Sertifika verilmesi

1. “Sertifika yönetimi”ni açın;
2. “Sertifika ver”e tıklayın;
3. Doldurun:
   - Etki alanı listesi (her satıra bir tane)
   - Sertifika türü (etki alanı / IP)
   - Anahtar türü (ECC/RSA)
   - Doğrulama yöntemi (DNS-01 / HTTP-01)
   - DNS sağlayıcı
   - Düzenleme konumu (panel ana bilgisayarı / düğüm)
4. “Sorun”a tıklayın.

### 14.2 Sertifika eylemleri

- **Yenile**: manuel olarak yenileyin
- **Dağıt**: bir düğüme dağıtın
- **Dağıt**: ilişkili tüm düğümlere gönder
- **Taşı**: Referansları eski bir sertifikadan yenisine taşıyın
- **İndir**: `cert` / `fullchain` / `privkey`'yi indirin

### 14.3 Sertifika yükleme

1. “Sertifikayı yükle”ye tıklayın;
2. Yöntemi seçin:
   - İçeriği yapıştırın
   - Dosyaları yükle
3. Sertifika zincirini ve özel anahtarı yükleyin;
4. Kaydetmek.

### 14.4 EAB yapılandırması

- Google Genel CA / ZeroSSL için EAB kimlik bilgilerini yönetin
- EAB'yi otomatik olarak edinin
- Verirken otomatik olarak ekleyin

---

## 15. Trafik kotaları

Trafik kotaları her düğümün aylık trafiğini izler.

### 15.1 Kota listesi

- Tüm düğümler için kota ayarlarını görüntüleyin
- Kota boyutunu, kullanılan trafiği ve kalan ödeneği göster
- Kullanım yüzdesini göster

### 15.2 Döngü kullanımı

- Düğüm başına döngü kullanım ayrıntılarını görüntüleyin
- Gelen/giden trafiği göster
- Döngü başlangıç/bitiş zamanını göster

### 15.3 Günlüğü sıfırla

- Trafik sıfırlama kayıtlarını görüntüle
- Sıfırlama döngüsünü, tetikleme kaynağını ve operatörünü gösterin

---

## 16. Zincirlenmiş proxy'ler

Zincirlenmiş proxy'ler çoklu çıkış bağlantılarını yapılandırır.

### 16.1 Bağlantı oluşturma

1. “Zincirlenmiş proxy'ler”i açın;
2. “Yeni bağlantı”ya tıklayın;
3. Giriş düğümünü seçin;
4. Gelen girişi seçin;
5. Çıkış düğümlerini seçin (birden fazla izin verilir);
6. Her çıkış için bir gelen seçin;
7. Önceliği / ağırlığı ayarlayın;
8. Onaylamak.

### 16.2 Bağlantı yönetimi

- Topolojiye genel bakışı görüntüleyin
- Bağlantıyı düzenleme (öncelik, ağırlık, giden etiketi)
- Bağlantıyı silme
- Sağlık durumunu kontrol edin

### 16.3 Dış düğümler

- Harici düğümleri içe aktarın (v2ray URI, Base64 URI, URL aboneliği)
- Harici girişleri kaydedin
- Harici düğümleri yönet

---

## 17. Çevrimiçi kullanıcılar

Çevrimiçi kullanıcılar gerçek zamanlı çevrimiçi durumu gösterir.

### 17.1 Ne gösterir?

- Çevrimiçi abonelikler
- Çevrimiçi cihazlar / IP'ler
- Çevrimiçi düğümler
- Gerçek zamanlı oran
- Toplam trafik

### 17.2 Filtreleme

- Düğüme göre filtrele
- E-postaya/müşteriye/düğüme/IP'ye göre arayın

---

## 18. Kullanıcı yönetimi

Kullanıcı yönetimi panel kullanıcılarını yönetir.

### 18.1 Roller

| Rol | Açıklama |
|------|-------------|
| Süper yönetici (`superAdmin`) | En yüksek ayrıcalıklar |
| Yönetici (`admin`) | Yönetim ayrıcalıkları |
| Operatör (`operator`) | Operasyonel ayrıcalıklar |
| Görüntüleyici (`viewer`) | Salt okunur |

### 18.2 Kullanıcı ekleme

1. “Kullanıcı yönetimi”ni açın;
2. “Kullanıcı ekle”ye tıklayın;
3. Doldurun:
   - Kullanıcı adı
   - E-posta
   - Şifre
   - Rol
4. Kaydet.

### 18.3 Kullanıcı işlemleri

- **Düzenle**: kullanıcı ayrıntılarını değiştir
- **Sil**: kullanıcıyı kaldırır

---

## 19. Portal sitesi

Portal sitesi istemci portalını yapılandırır.

### 19.1 Genel

- Site adı
- Site açıklaması
- Kayıt aç
- Portal yolu
- Portal bağlantı noktası

### 19.2 Görünüm

- Site başlığı
- Logo URL
- Birincil, arka plan, metin ve vurgu renkleri
- Yazı tipi

### 19.3 Gezinme

- Gezinme sayfaları ekleyin
- Sayfa adını, simgeyi ve harici bağlantıyı yapılandırın
- Gezinme öğelerini yönet

### 19.4 Modüller

- Modül ekleme/düzenleme
- Modül başlığını, açıklamasını ve simgesini yapılandırın
- Markdown'ı, zengin metni ve özel HTML'yi destekler

### 19.5 Diller

- Dil ekle
- Kopyayı yönet
- JSON'yi içe / dışa aktar

### 19.6 Şablonlar

- Şablonları değiştir
- HTML / CSS / JS'yi özelleştirin
- Varsayılan şablona sıfırla

---

## 20. Sistem ayarları

Sistem ayarları panel parametrelerini yapılandırır.

### 20.1 Paneli

- Adresi ve bağlantı noktasını dinle
- Veri dizini
- Saat dilimi
- Abonelik bağlantı noktası
- İhracat limiti
- Kayıt aç
- Panel modu (tam / yalnızca yönetim)

### 20.2 Güvenlik

- Şifreleme anahtarı yönetimi
- JWT anahtar yönetimi
- IP kara listesi
- Passkey izin verilenler listesindeki alanlar

### 20.3 OAuth

- GitHub OAuth
- Google OAuth
- Geri arama URL yapılandırması

### 20.4 Bildirimler

- E-posta bildirimleri (SMTP)
- Telegram bildirimleri
- Bildirim şablonları
- İzleme ayarları

### 20.5 Yedekleme ve geri yükleme

- Yedek oluştur
- Yedekleme listesi
- Bir yedekleme indirin
- Bir yedeği geri yükleme
- Zamanlanmış yedeklemeler
- Uzak depolama (FTP / S3 / WebDAV)

### 20.6 Otomatik engellemeyi kaldırma

- Otomatik engellemeyi kaldırmayı etkinleştir
- Bekleme süresi
- Geçerli durumlar

---

## 21. Operasyon konsolu

Operasyon konsolu, yapılandırma anlık görüntülerini yönetir ve işleri uygular.

### 21.1 Yapılandırma anlık görüntüleri

- Anlık görüntü oluştur
- Anlık görüntüleri listeleme
- Anlık görüntüye geri dönme
- Anlık görüntüyü silme

### 21.2 İş başvurusu

- Son işleri görüntüle
- Yapılandırmayı şimdi uygula (DB → çalışma zamanı)
- Ön kontrol (bağlantı noktası çakışmalarını ve JSON yapılandırmasını doğrulayın)
- Bir işi iptal etme
- Bir işi yeniden dene

### 21.3 Düğüm doğrulaması

- Bir düğüm seçin ve doğrulayın
- Doğrulama sonucunu görüntüle
- Tüm düğümleri doğrula

### 21.4 Acil durdurma

- Proxy'yi acil durdurma (boş bir gelen yapılandırma yazar)
- Tüm gelen trafiği derhal durdurun

---

## 22. Veritabanı izleme

Veritabanı izleme, veritabanı sağlığını gösterir.

### 22.1 Ne gösterir?

- Veritabanı motoru
- Veritabanı boyutu
- Bağlantı
- Gecikme
- Bağlantı havuzu durumu
- Önbellek istatistikleri
- Sorgu istatistikleri

### 22.2 Desteklenen motorlar

- SQLite
- MySQL
- PostgreSQL

---

## 23. Denetim günlüğü

Denetim günlüğü işlemleri kaydeder.

### 23.1 Ne gösterir?

- Kategori
- Etkinlik türü
- Sonuç
- Kaynak IP'si
- Kullanıcı
- Ayrıntılar

### 23.2 Düğüm işlem günlüğü

- Düğüm işlem kayıtlarını görüntüleyin
- Düğüme göre filtrele

---

## 24. Günlük yönetimi

Günlük yönetimi sistem günlüklerini gösterir.

### 24.1 Günlük türleri

- Panel günlükleri
- Düğüm günlükleri
- Çekirdek günlükleri
- Proxy günlükleri

### 24.2 Eylemler

- Günlükleri görüntüle
- Günlükleri indir
- Günlükleri temizle

---

## 25. Operasyon uzantıları

Operasyon uzantıları isteğe bağlı uzantıları yapılandırır.

### 25.1 Özellikler

- Özel uzantılar
- Uzantı yapılandırması
- Uzantı yönetimi

---

## 26. SSS

### 26.1 Panel başlamıyor

- Bağlantı noktasının zaten kullanımda olup olmadığını kontrol edin
- `.env`'nin geçerli olup olmadığını kontrol edin
- Günlükleri kontrol edin: `journalctl -u corefusion -f`; systemd, `tail -f <install-dir>/data/logs/panel.log` olmadan

### 26.2 Düğüm çevrimdışı

- Aracının çalışıp çalışmadığını kontrol edin: `systemctl status corefusion-agent`
- Ağ bağlantısını kontrol edin
- Güvenlik duvarı kurallarını kontrol edin (`18443` bağlantı noktası)

### 26.3 Sertifika verilmesi başarısız oluyor

- DNS çözünürlüğünü kontrol edin
- 80 numaralı bağlantı noktasının erişilebilir olup olmadığını kontrol edin
- DNS sağlayıcı yapılandırmasını kontrol edin
- Sertifika günlüklerini kontrol edin

### 26.4 Abonelik çalışmıyor

- Abonelik bağlantısını kontrol edin
- Düğümün çevrimiçi olup olmadığını kontrol edin
- Çekirdek işlemin çalışıp çalışmadığını kontrol edin
- Güvenlik duvarı kurallarını kontrol edin

### 26.5 Yeniden doldurma geri araması başarısız oluyor

- Geri aramayı kontrol edin URL
- Site anahtarının eşleşip eşleşmediğini kontrol edin
- IP izin verilenler listesini kontrol edin
- Sipariş günlüklerini kontrol edin

### 26.6 Portal sayfasına erişilemiyor

- Portal bağlantı noktasının açık olup olmadığını kontrol edin
- Portal yolu yapılandırmasını kontrol edin
- **Önemli**: Ters proxy'nin portal etki alanını ana panel bağlantı noktasına değil **portal motoru bağlantı noktasına** ilettiğinden emin olun; yanlış bağlantı noktasına yönlendirme, portal yerine boş bir sayfa veya panel gösterir. Proxy sitesi kaydında `upstream` / `local_port`'yi kontrol edin.
- Portal motorunun kendisini doğrulamak için doğrudan `http://<portal-domain>:<portal-port>`'yi açın
- Ters proxy yapılandırmasını kontrol edin
- Portal günlüklerini kontrol edin

---

## Ek

### A. Varsayılan bağlantı noktaları

| Liman | Amaç |
|------|---------|
| Kurulum sihirbazı tarafından atanır (varsayılan olarak rastgele boş bağlantı noktası) | Yönetici paneli (değiştirilebilir) |
| 18443 | Aracı gRPC (değiştirilebilir) |
| 0 (varsayılan olarak devre dışıdır) | Özel abonelik motoru bağlantı noktası (boş veya `0` ana bağlantı noktasını paylaşır) |
| 0 (varsayılan olarak devre dışıdır) | Özel portal motoru bağlantı noktası (boş veya `0` ana bağlantı noktasını paylaşır) |
| 80 / 443 | Ters proxy (Caddy / Nginx) HTTP / HTTPS |

### B. Varsayılan yollar

| Yol | Amaç |
|------|---------|
| `/` | Yönetici paneli (özel bir portal motoru yapılandırılmadığında portal ayrıca `/`'de bulunur) |
| `/sub/:token` | Abonelik getirme (özel bir abonelik bağlantı noktası yapılandırılmadığında) |
| `/p` | Portal sayfası (özel bir portal bağlantı noktası yapılandırılmadığında) |
| `/:token` | Abonelik getirme (özel bir abonelik bağlantı noktasıyla, `/sub` öneki olmadan) |

### C. Komut satırı aracı

Panel ikili programı şu alt komutları destekler (sunucuyu hiçbir argüman başlatmaz):

```bash
# Start the panel (same as ./corefusion server)
./corefusion

# Run database migrations
./corefusion migrate

# Offline administration (when you cannot sign in)
./corefusion admin reset-password -u admin -p 'new-password'
./corefusion admin 2fa reset -u admin
./corefusion admin list

# Version / help
./corefusion version
./corefusion help
```

> systemd ünitesi olmadan eski işlemi durdurup `nohup` ile yeniden başlatarak yeniden başlatın; günlükler `<install-dir>/data/logs/panel.log`'dedir.

---

> **Belge sürümü**: v1.0
> **Son güncelleme**: 2026-10-23
> **Şunlar için geçerlidir**: CoreFusion v1.0+
