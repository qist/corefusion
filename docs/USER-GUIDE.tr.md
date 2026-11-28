<!-- source: USER-GUIDE.md | locale: tr | updated: 2026-11-27 -->
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
| Servis yöneticisi | systemd (önerilir); systemd'nin bulunmadığı kaplarda veya diğer ortamlarda `nohup`'ye geri döner |
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
2. `corefusion-linux-<arch>`'yi GitHub Sürümlerinden indirin ve eşleşen `.sha256` ile doğrulayın;
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

Panel yalnızca `127.0.0.1`'yi dinliyorsa (örneğin ters proxy'nin arkasında), komut dosyası, ulaşılamayan URL yerine bir SSH tünel komutu yazdırır.

> Jetonunu mu kaybettin? Başlangıç günlüğünü tekrar okuyun: `journalctl -u corefusion -n 50 --no-pager`
> (systemd olmadan: `tail -n 50 /opt/corefusion/data/logs/panel.log`).

### 2.2.1 Çok dilli çıkış

Yükleyici dört kullanıcı arayüzü dili içerir: Basitleştirilmiş Çince, İngilizce, Farsça (فارسی) ve Rusça (Русский). `LANG` sistemini otomatik olarak algılar veya açıkça seçebilirsiniz:

```bash
# English
sudo bash install.sh --lang en

# Persian via environment variable
sudo CF_LANG=fa bash install.sh

# Pass options through a pipe
curl -fsSL https://raw.githubusercontent.com/qist/corefusion/main/scripts/install.sh | sudo bash -s -- --lang ru
```

Desteklenen değerler: `zh` / `en` / `fa` / `ru`; `--lang=ru` de kabul edilir. Desteklenmeyen açık bir değer, kullanılabilir diller listesinde başarısız oluyor.

### 2.3 Sürüm sabitleme / aynalar / çevrimdışı kurulum

```bash
# Pin a version
sudo bash install.sh --version v0.14.6

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
sudo bash install.sh --source --version v0.14.6
```

`--source`, depoyu klonlar (veya komut dosyası içindeyken mevcut ödemeyi kullanır) ve `make panel`'yi çalıştırır. Araç zinciri versiyonları ortama göre seçilir:

| Bağımlılık | Seçim |
|------------|-----------|
| Git | `go.mod`'deki `toolchain` / `go`'den okuyun (şu anda **Go 1.27+**); sistem sürümü çok eskiyse eşleşen araç zinciri `<install-dir>/.toolchain`'ye indirilir |
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
| `-v, --version <TAG>` | Panel versiyonu, ör. `v0.14.6`; varsayılan `latest` |
| `--binary` | GitHub Sürüm ikili dosyasını kullanın (varsayılan) |
| `--source` | Kaynaktan derleyin ve eşleşen Go/Node'u hazırlayın |
| `--local <FILE>` | Yerel bir ikili dosya kullanın (çevrimdışı kurulum) |
| `--arch <ARCH>` | Hedef mimari: `amd64` / `arm64` / `armv7` / `armv6` / `386` / `ppc64le` / `s390x` / `riscv64` / `loong64` |
| `--mirror <URL>` | GitHub indirme öneki, ör. `https://ghproxy.net` |
| `--repo <URL>` | Kaynak deposu, varsayılan `https://github.com/qist/corefusion.git` |
| `--ref <REF>` | Kaynak dalı/etiketi; varsayılan olarak `--version`'dir, aksi takdirde `main` |
| `--no-deps` | Sistem bağımlılığı kurulumunu atla |
| `--no-start` | Hizmet dosyasını başlatmadan yükleyin ve yazın |
| `--cpu-info` | Yazdırma ortamı algılama ve eksik bağımlılıklar, ardından çıkın |
| `--lang <LANG>` | Çıkış dili: `zh` / `en` / `fa` / `ru` (varsayılan: sistem `LANG`) |
| `--uninstall` | Hizmeti durdurun ve programı kaldırın (verileri ve `.env`'yi tutar) |
| `--purge` | `--uninstall` ile veri dizinini ve `.env`'yi de kaldırın. |

> Resmi sürümler yalnızca `amd64` / `arm64` ikili dosyalarını gönderir. Diğer mimariler `--source` gerektirir,
> aksi halde indirme işlemi 404 değerini döndürür.

İlgili ortam değişkenleri:

| Değişken | Amaç |
|----------|---------|
| `CF_GO_VERSION` | Kaynak derlemeler için gereken Go sürümünü geçersiz kılın |
| `CF_NODE_MAJOR` | Düğümün ana sürümünü geçersiz kıl (varsayılan 22) |
| `CF_GO_MIRROR` | Araç zinciri indirme aynasına git |
| `CF_SKIP_DEPS` | Bağımlılık kurulumunu atlamak için Agent yükleme komut dosyasında `1` olarak ayarlayın |
| `CF_LANG` | Yükleyici çıkış dilini geçersiz kıl (`--lang` ile aynı) |

### 2.6 Dizin düzeni

```text
/opt/corefusion/
├── corefusion                 # panel binary
├── corefusion.bak             # previous version (when upgrading)
├── .env                       # environment file (0600, generated by the setup wizard)
├── .env.secret                # encryption key file (0600, generated on first start)
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

**v0.14.4'ten itibaren abonelik kimlik bilgileri diskte şifrelenir**: ilk başlatmada abonelik token'ları, kısa kodlar, rotasyon geçmişi ve üst kaynak abonelik URL'leri şifrelenir ve kısa bağlantı iptal tablosu oluşturulur. Yükseltmeden önce veritabanını yedekleyin; geri alma işlemi yükseltme öncesi veritabanını da geri yüklemelidir, çünkü eski ikili yeni veritabanını okuyamaz ve tüm abonelikler 404 döner.

### 2.9 Ortam dosyası (.env)

Yapılandırma kaydedildiğinde `.env`'nin tamamı **kurulum sihirbazı tarafından oluşturulur** (veritabanı DSN'si, veri dizini, sırlar, bağlantı noktaları, etki alanları).

Yeni kurulumda yükleyici yalnızca bu yer tutucuyu yazar, böylece panel mevcut `data/`'yi başlatılmış kurulum zannetmez ve sihirbazı atlar:

```env
SAFE_PATHS=data
```

> Tam bir yapılandırma (`CF_DB_DSN` ile `.env`) zaten mevcut olduğunda, yükleyici asla bunun üzerine yazmaz ve sihirbaz bir daha görünmez.
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

> **Şifrelenmiş şifreler**: Panel tarafından yazılan MySQL/PostgreSQL DSN'ler (kurulum sihirbazı / veritabanı anahtarı)
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
| `CF_SECRET_ENCRYPTION_KEY` | Hassas ayarları ve abonelik kimlik bilgilerini şifreleme anahtarı (AES-256-GCM); yoksa otomatik oluşturulur |
| `CF_INITIAL_ADMIN_PASSWORD` | İlk yönetici şifresi (isteğe bağlı; rastgele ve varsayılan olarak bir kez yazdırılır) |
| `CF_NETWORK_TRAFFIC_ACCOUNTING` | İptables tabanlı trafik hesaplamasını devre dışı bırakmak için `0` olarak ayarlayın (varsayılan olarak etkindir) |
| `CF_NETWORK_TRAFFIC_TABLE` | Trafik muhasebesi için kullanılan iptables tablosu, varsayılan `corefusion` |

> **Şifreleme anahtarı `.env.secret`'de bulunur**: ilk başlangıçta `CF_SECRET_ENCRYPTION_KEY`, `.env` yerine `.env.secret`'ye (0600, `.env` ile aynı dizin) yazılır; `.env`'deki eski bir anahtar otomatik olarak taşınır. Bu dosyayı ayrı olarak yedekleyin; onu kaybederseniz, tüm `enc:` değerlerinin şifresi çözülemez hale gelir.
>
> v0.14.4'ten itibaren `subscriptions.token`, kısa kodlar ve rotasyon geçmişi ile `external_node_sources.source_url` de bu anahtarı kullanır: token/URL'ler AES-256-GCM şifreli metninden gösterilir ve kısa bağlantı araması HMAC-SHA256 parmak iziyle yapılır. Anahtar kaybolursa bu kimlik bilgileri de çözülemez veya eşleştirilemez.

> Gerçek istemci IP'lerini çözümlemeye yönelik güvenilir ters proxy'ler artık ortam değişkenleri aracılığıyla yapılandırılmamaktadır;
> **Ayarlar → Ağ**'da `panel.trusted_proxies` olarak depolanırlar.

> Hem `.env` hem de `.env.secret` sırlar içerir; panel başlangıçta izinlerini `0600`'ye sıkılaştırıyor. Bunları asla taahhüt etmeyin.

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
> Tokeni paylaşmayın. Kurulum tamamlandığında bu bağlantı noktası kapanır ve yalnızca normal panel bağlantı noktası kalır.

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
- `CF_SECRET_ENCRYPTION_KEY`: hassas ayarları (veritabanı parolası, sertifika DNS kimlik bilgileri, kredi kartları vb.) ve abonelik token'larını / kısa kodları / rotasyon geçmişini / üst kaynak abonelik URL'lerini şifreler.
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
| Adresi dinle | Otomatik olarak türetilmiştir: proxy → `127.0.0.1`; sertifikalı vekil yok → `0.0.0.0`; aksi halde → `127.0.0.1` |
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

1. Düğümü ekledikten sonra, düğüm listesinden “Komutu dağıt”ı seçin (ayrıntı sayfasında ayrıca
 “Önyükleme yükleme komutu”);
2. Komutu kopyalayın — tek seferlik bir kayıt jetonu içerir (24 saat geçerlidir, yeniden kullanmayın);
3. Uzak düğümde kök olarak çalıştırın; şuna benziyor:

   ```bash
   bash <(curl -fsSL "http://<panel-host>:<panel-port>/api/v1/agent/install?id=<node-id>&token=<one-time-token>&port=<panel-port>")
   ```

4. Agent paketi indirir, panele kaydolur ve başlar.

### 5.4 Düğüm ayrıntıları

Ayrıntı sayfasını açmak için bir düğüme tıklayın:

- **Genel Bakış**: temel bilgiler, Temsilci durumu, sertifika durumu; kaynak kartları CPU / bellek / disk kullanımını, NIC yukarı/aşağı oranını, çekirdek kullanıcı trafiği oranını, aylık kullanılan trafiği, TCP bağlantılarını, sistem yükünü ve çıkış NIC kümülatif trafiğini gösterir (her 10 saniyede bir sessiz yenileme)
- **Protokol merkezi**: temel süreç yapılandırması
- **Proxy**: Caddy / Nginx yapılandırması
- **Temel işlemler**: başlat / durdur / yeniden başlat
- **Çekirdek trafiği**: Çekirdek tarafından iletilen kullanıcı trafiği ve bunun aylık toplamı
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

### 5.6 Düğüm güvenliği kontrolü

Düğüm ayrıntıları → "Kontrol": salt okunur toplama, hiçbir yapılandırmayı değiştirmez.

- **Kapsam**: sistem bilgisi, SSH (şifre / klavye etkileşimli / root girişi / MaxAuthTries / anahtar izinleri /
  port 22'nin kaynak kısıtlı olup olmadığı), güvenlik duvarı tespiti (firewalld / ufw / nftables / iptables),
  çekirdek parametreleri (biriktirme kayıtları, arabellekler, tıkanıklık kontrolü, bağlantı, IPv6) ve yapılandırma dosyaları arasındaki öncelik.
- **Önem derecesi**: yüksek / uyarı / bilgi / tamam. Gürültüyü önlemek için yalnızca karşılanmayan maddeler tavsiye niteliğindedir.
- **Önerilen komutlar diske yazılır**: çekirdek parametreleri `/etc/sysctl.d/99-corefusion.conf`'ye gider
  (anahtar başına çoğaltılmamış, ardından `sysctl --system`); SSH, `/etc/ssh/sshd_config.d/00-corefusion-*.conf`'ye gider;
  BBR modülü `/etc/modules-load.d/`'ye gider. Hepsi yeniden başlatmadan sağ kurtulur.
- **Yapılandırma önceliği**: `CFG-01`, sysctl anahtarı başına en son etkili olan dosyayı ve satırı listeler —
  `/etc/sysctl.conf` her zaman en son yüklenir ve `99-corefusion.conf`'yi geçersiz kılar; `CFG-02` ilkini listeliyor
  eşleşen sshd direktifi — sshd gördüğü ilk değeri kullanır, açılanlar `00-` önekine ihtiyaç duyar ve
  `Include /etc/ssh/sshd_config.d/*.conf` bu yönergelerden önce görünmelidir, aksi takdirde drop-in'ler
  tamamen göz ardı edildi (uyarı olarak rapor edildi).
- **IPv6**: yalnızca düğümün gerçekten genel bir IPv6 adresine sahip olması durumunda kontrol edilir. Tünel adresleri gibi
  `singbr0` / `wg0` / `tun0`, `tun` olarak etiketlenir, proxy tarafından atanır ve **elle düzenlenmemelidir**;
  düğümün genel IPv6'sından hariç tutulurlar.
- **Root oturumu**: `PermitRootLogin yes`, hem `PasswordAuthentication` hem de `PasswordAuthentication` olduğunda iyi olarak değerlendirilir.
  `KbdInteractiveAuthentication`, `no`'dir (yalnızca anahtar); kontrol hiçbir zaman root'u devre dışı bırakmanızı istemez.
- **Tam ayarlama belgesi**: iletişim kutusu Markdown'un tamamını kopyalayabilir/indirebilir ve "Çevrimiçi görüntüle" açılır
  Geçerli kullanıcı arayüzü dili (16 dil) deposunda `docs/node-tuning.<lang>.md`.
- **Önbellek**: sonuçlar 10 dakika süreyle önbelleğe alınır, `refresh=1` yeni bir koleksiyonu zorunlu kılar; planlanmış kontroller
  varsayılan olarak kapalıdır.

## 6. Protokol merkezi

Protokol merkezi, çekirdek işlemler için protokol parametrelerini yapılandırır.

### 6.1 Desteklenen çekirdekler

| Çekirdek | Açıklama |
|------|-------------|
| Xray-çekirdek | VLESS / VMess / Trojan / Shadowsocks ve daha fazlası |
| sing-box | TUN destekli modüler evrensel proxy platformu; Hysteria2 / TUIC ve diğer QUIC protokollerini taşır |

### 6.2 Konfigürasyon modülleri

Modüller iki çekirdek arasında farklılık gösterir; panel, seçilen çekirdekle eşleşen seti gösterir:

**Xray-çekirdek**

- **Günlük**: erişim günlüğü, hata günlüğü, çıkış modu
- **API**: gRPC API dinleme adresi, hizmet listesi
- **DNS**: DNS sunucuları, sorgu stratejisi, statik ana bilgisayarlar
- **Yönlendirme**: yönlendirme kuralları, yük dengeleme, VLESS yönlendirme
- **Gelen / Giden**: protokol, aktarım, TLS
- **Aktarım**: WebSocket, gRPC, XHTTP vb. (TUN bir aktarım değil, gelen bir protokoldür)
- **TLS**: sertifikalar, ALPN, uTLS parmak izi, ECH
- **Reality**: hedef yönetimi, anahtar çiftleri, kısa kimlikler
- **Sağlık kontrolü / FakeDNS / Ölçümler / Politika / Coğrafi veriler / Sürüm sınırları**: problar, IP havuzu, Prometheus, zaman aşımları ve arabellekler, kaynak dosyaları, çekirdek sürüm aralığı

**sing-box**

- **Günlük**: düzey, zaman damgası, çıktı
- **NTP**: zaman senkronizasyonu
- **DNS**: DNS sunucuları ve kuralları
- **Yönlendirme**: `final` varsayılan giden, **Kural Kümeleri**, **Kurallar**, varsayılan etki alanı çözümleyicisi, ağ stratejisi ve geri dönüş (bkz. 6.6)
- **Gelen/Giden**: protokol, aktarım, TLS
- **Uç noktalar**: WireGuard (WARP yapılandırmaları dahil), MASQUE istemcileri ve benzerleri; birkaçı bir arada bulunabilir ve `tag` ile ayırt edilir

> "Sistem ağ entegrasyonunu kullan" etkinken, uç noktanın **adı** çekirdek arayüzü adıdır:
> en fazla 15 karakter, yalnızca harfler/rakamlar/`._-`. Çok uzun veya geçersiz bir ad otomatik olarak geri alınır
> `tag` uç noktasına. Yeni uç noktalara protokol başına ön ek eklenir (`wg-` / `ts-` / `oc-` / `ovpn-c-` /
> `ovpn-s-` / `mq-c-` / `mq-s-`) ve `WireGuard-xxxxxx` gibi eski kayıtlar normalleştirilmiş olarak gösterilir
> Editör açılır açılmaz.
- **Hizmetler**: `hysteria-realm`, DERP veya `resolved` gibi üst düzey sing-box `services` dizisi
- **Sertifikalar / HTTP istemcileri / Ağ ad alanları**: Yalnızca gerektiğinde genişletebileceğiniz JSON bölümleri
- **Deneysel**: `clash_api` ve benzeri; **etkinleştirilmedikçe oluşturulan yapılandırmaya hiçbir şey yazılmaz**

> sing-box 1.12, `geosite` / `geoip` yönlendirme alanlarını kaldırdı; Coğrafi tabanlı yönlendirme artık devam ediyor
> `rule_set`. Eski alanların korunması, çekirdeğin yapılandırmanın (`unknown field`) kodunu çözememesine neden olur.

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

> Yeni gelenler varsayılan olarak `::`'yi dinler (Linux'de çift yığın, hem IPv4 hem de IPv6'yı yönetir).
> Yalnızca arayüzü kısıtlamanız gerektiğinde bunu `0.0.0.0` olarak veya belirli bir adresle değiştirin.

"Gelen bağlantı noktasını kullan" bağlantı noktası modları aynı zamanda **güvenlik duvarı izin verme kurallarını otomatik olarak oluşturma** olanağı da sunar:
Gelen düzenleyici ilk önce düğümün güvenlik duvarını inceler ve güvenlik duvarı / ufw / nftables / iptables algılandığında
dinleme protokolüne (TCP/UDP) göre bağlantı noktasının kaydedilmesine izin verebilirsiniz.

- hy2 / hysteria bağlantı noktası atlamalı, hem dinleme bağlantı noktasına hem de her dinamik bağlantı noktası aralığına izin verir (ör. `20000-30000`); birden çok aralık virgülle ayrılmıştır;
- İzin bağımsızdır: mevcut bir kapsama kuralı (bağlantı noktası aralığı / çoklu bağlantı noktası) yeniden oluşturulmaz;
- Bağlantı noktasını değiştirmek, yenisine izin vermeden önce eski kuralı iptal eder; gelenleri silmek, bağlantı noktası atlamayı devre dışı bırakmak veya Unix Soket / tun'a (dinleme bağlantı noktası yok) geçiş yapmak onu temizler;
- Etkinleştirilmiş bir güvenlik duvarı tespit edilmediğinde yalnızca bir ipucu gösterilir ve herhangi bir kural yazılmaz.

**Taşıma ve akış**:

- `xtls-rprx-vision` yalnızca ham TCP üzerinde çalışır: gRPC / XHTTP / WebSocket ile istemci doğrudan başarısız olur (sing-box: `initialize vision: not a valid supported TLS connection`) veya sunucu akış denetimiyle aynı fikirde değildir (`flow mismatch`). Panel, kaydetme sırasında ve çalışma zamanı yapılandırmasını oluştururken ham olmayan/TCP gelen tüm gelenlerden görüşü çıkarır;
- sing-box aktarımları yalnızca http / ws / quic / grpc / httpupgrade'tir — **ham TCP, hiçbir aktarım olmadığı anlamına gelir**. Taşıma açılır menüsünün ilk (boş) girişi ham TCP'tir; `type: "tcp"` yazmak çekirdeğin yapılandırmayı reddetmesine neden olur.

### 6.3.1 Ters proxy yolu (Erişim etki alanı → Ön TLS → Ters proxy yolu)

Gelenin kendi aktarım yolu `/` olduğunda ancak bir dış nginx / caddy onu bir alt yolun altına bağladığında
(örn. `location /apps { proxy_pass http://127.0.0.1:8080/; }`), istemcilerin `/apps`'ye veya
istek asla bu konuma ulaşmaz; abonelik hâlâ indirilir ancak düğüm bağlanmaz.
“Etki alanına erişim → Ön TLS” altında **Tersine proxy yolunu** doldurun:

- `/apps` veya `/apps?ed=2560` (erken verilerle) girin; abonelikler ve paylaşım bağlantıları gelen aktarım yolunu **değiştirir**;
- ws, httpupgrade, xhttp ve h2/http için geçerlidir; yolu olmayan aktarımlar (raw/tcp, kcp) göz ardı edilir;
- TLS'nin nerede sonlandırıldığından bağımsızdır: Yol, Ön TLS kapalıyken de geçerlidir (çekirdeği takip edin);
- gRPC'de URL öneki kavramı yoktur (istemci yolu her zaman `/{serviceName}/…`'dir), dolayısıyla yol **serviceName ile birleştirilmiştir**
(`/apps` + `svc` → `apps/svc`); ters proxy'nin `grpc_pass`'deki bu öneki çıkarması gerekir.

### 6.3.2 SNI tabanlı yönlendirme (nginx ve Caddy)

Ön TLS katmanı `:443` üzerindeki bağlantıları ClientHello SNI'sına göre böler: protokol merkezindeki REALITY
girişleri kuralları otomatik türetir, kurallar «Proxy yönetimi → SNI» altında elle de yazılabilir. İki proxy de alan adına göre
böler; fark PROXY protocol kapsamındadır.

| | nginx (stream) | Caddy (layer4) |
|---|---|---|
| `proxy_protocol` kapsamı | Panelin paketlediği nginx (`stream_pp_var` yamalı): **SNI kuralı başına** v1 / v2 / kapalı. Resmî veya yamasız nginx: tüm listen grubu (`listen` düzeyi), port başına ya tümü açık ya tümü kapalı | Rota başına (`proxy ... { proxy_protocol v1|v2 }`) |
| Aynı `:443` üzerinde xray + sing-box | Yama sonrası desteklenir: xray REALITY v1/v2 alır, sing-box kuralı kapalıya ayarlanır | Desteklenir: bir SNI xray'e (PP v2), diğeri sing-box'a (PP yok) |
| sing-box'a erişim | Yama sonrası xray ile `:443` paylaşılabilir (o alan adı için PP kapalı); aksi halde ayrı port veya PP'siz ayrı listen grubu | xray ile `:443` paylaşılır, SNI ile ayrılır |

Notlar:

- Caddy, PROXY protocol sürümünü site (rota) başına seçer: v1 / v2 / kapalı; varsayılan eski davranış olan v2'dir, kapalıyken o site PROXY başlığı göndermez.
- Varsayılan `v1` olarak kalır (eski `proxy_protocol on`). Her SNI sitesi `v1`, `v2` veya kapalı yapılabilir. `v2` ikili bir
  başlıktır (IPv4 için 28 bayt) ve v1 metin başlığından (~50–60 bayt) daha kısadır; fark bağlantı başına bir kez oluşur ve
  pratikte hissedilmez.
- Panel, Agent üzerinden `nginx -V` çalıştırıp `NGX_STREAM_PP_VAR` (yetenek bayrağı `stream_pp_var`) arar; sonuç 60 saniye
  önbelleklenir. Yamasız nginx, başarısız sorgu veya eski Agent durumunda grup genelinde `proxy_protocol on` (v1) değerine
  döner ve türetilmiş sing-box SNI kurallarını atlar (log `skipped N sing-box REALITY SNI rule(s)`); `nginx -t` testinden
  geçmeyen yapılandırma asla üretilmez.
- sing-box REALITY / anytls PROXY başlığı alamaz, bu nedenle türetilmiş kurallar her zaman kapalıdır. Böyle bir alan adı için
  zorla v1/v2 seçmek el sıkışmayı bozar.
- Ters proxy arkasındaki xray, gerçek istemci IP'si ve portu için PROXY protocol'e ihtiyaç duyar
  (`sockopt.acceptProxyProtocol`); `realitySettings.xver` xray'in dest'e gönderdiğidir ve bununla ilgisizdir.
- «Reality şablonu» içindeki `singbox_listen_port` **yerel TLS uç noktasıdır** (xray dest / sing-box handshake); iki proxy de
  destekler ve yukarıdaki genel SNI yönlendirmesinden bağımsızdır.

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
   - MihomoYAML
   - sing-box JSON
   - Xray JSON
3. Bağlantıyı kopyalayın veya bir QR kodu oluşturun.

### 6.6 sing-box yönlendirme ve kural kümeleri

"Protokol merkezi" → sing-box → "Yönlendirme" bölümünde iki bölüm vardır: **Kural Kümeleri** yeniden kullanılabilir olduğunu beyan eder
kural dosyaları ve **Kurallar** maç koşullarına ve gidenlere karar verir. Kurallar şu şekilde değerlendirilir:
yukarıdan aşağıya doğru ve ilk maç kazanır.

#### 6.6.1 Kural Setleri

| Alan | Açıklama |
|-------|-------------|
| etiket | Kurallar tarafından başvurulan kural kümesi tanımlayıcısı; benzersiz olmalı |
| Tür | `local` düğümdeki bir dosyayı okur; `remote`, çekirdeğin onu `update_interval`'ye indirmesine izin veriyor |
| Biçim | `.srs` için `binary` (varsayılan); Kaynak formatı `.json` için `source` |
| Yol | `local` yolları, düğüm veri dizinine göredir; `data/rules/`'deki dosyalar `rules/xxx.srs`'yi kullanır |
| URL | `remote` için indirme adresi; `download_detour`, indirme için kullanılan giden bağlantıyı seçer (genellikle `direct`) |

Yerel kural kümeleri (önerilir; dosyaları önce "Hizmet kaynakları → Kural dosyaları" bölümünden indirin):

```jsonc
"route": {
  "rule_set": [
    { "type": "local", "tag": "geosite-cn", "format": "binary", "path": "rules/geosite-cn.srs" },
    { "type": "local", "tag": "geoip-cn", "format": "binary", "path": "rules/geoip-cn.srs" }
  ]
}
```

> Dosyayı panele indirmek yeterli değildir: önce bir `local` kural kümesinin panele gönderilmesi gerekir.
> düğümler ("Tüm düğümlere gönder") böylece dosya düğümün `data/rules/`'sinde bulunur; aksi takdirde çekirdek
> dosya bulunamadığı için başlatılamıyor.

Uzak kural kümeleri (düğümün URL'ye ulaşabilmesi gerekir):

```jsonc
{ "type": "remote", "tag": "geosite-openai", "format": "binary",
  "url": "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-openai.srs",
  "download_detour": "direct", "update_interval": "1d" }
```

Yerel bir dosya seçtiğinizde etiket, dosya adından türetilir (`rules/geosite-cn.srs` →
`geosite-cn`); uzak setler için URL'nin sonundan gelir. Format ayrıca şu adresten de doldurulur:
uzantı (`.srs` → `binary`, `.json` → `source`). Türetilen değerler serbestçe düzenlenebilir ve
Daha önce yazdığınız değerlerin üzerine hiçbir zaman yazılmaz.

> `download_detour`, sing-box 1.14'ten beri kullanımdan kaldırılmıştır ve 1.16'da kaldırılacaktır; hala
> 1.15'te çalışıyor. Yeni konfigürasyonlar üst düzey `http_clients`'yi kullanmalıdır.
> Bunun yerine `route.default_http_client`.

#### 6.6.2 Kurallar

| Alan | Maçlar |
|-------|---------|
| `domain` | Tam alan adı |
| `domain_suffix` | Alt alanlar dahil alan adı son eki |
| `domain_keyword` | Alt dize eşleşmesi |
| `domain_regex` | Düzenli ifade |
| `ip_cidr` / `ip_is_private` | Hedef IP'si |
| `source_ip_cidr` | Kaynak IP'si |
| `port` / `port_range` | Hedef bağlantı noktası |
| `network` | `tcp` / `udp` |
| `rule_set` | Kullanılacak Kural Kümelerinin Etiketleri |
| `outbound` | Kullanılacak giden etiketi ("Gidenler" veya "Uç Noktalar" altında bulunmalıdır) |
| `type: "logical"` | `mode`'nin `and` / `or` olarak ayarlandığı birleşik koşullar |

Bir kuraldaki birden çok alan VE ile birleştirilir (örneğin `network` + `port`), dolayısıyla hayır
`logical` sarıcıya ihtiyaç var. `type: "logical"`'yi yalnızca VEYA / DEĞİL kombinasyonları için kullanın. `rule_set`
`rules`'nin yanına değil, `rules` alt kurallarının içine yerleştirilmelidir:

```jsonc
"rules": [
  { "domain_suffix": ["gstatic.com", "googleapis.com"], "outbound": "direct" },
  { "domain_keyword": ["ip.sb", "yandex.com"], "outbound": "warp" },
  { "rule_set": ["geosite-openai", "geosite-google"], "outbound": "warp" },
  { "rule_set": ["geoip-cn"], "outbound": "warp" },
  { "network": "tcp", "port": 443,
    "rule_set": ["geosite-google"], "outbound": "warp" },
  { "type": "logical", "mode": "or",
    "rules": [
      { "rule_set": ["geosite-google"] },
      { "rule_set": ["geosite-openai"] }
    ],
    "outbound": "warp" }
]
```

#### 6.6.3 xray kurallarını taşıma

| xray | sing-box |
|------|----------|
| Çıplak dize (`domain`'deki çıplak değerler dahil; alt dize eşleşmesi) | `"domain_keyword": ["ip.sb"]` |
| `"domain": ["a.com"]` | `"domain_keyword": ["a.com"]`; Gerçek bir son ek eşleşmesi için `"domain_suffix": ["a.com"]`'yi veya tam bir eşleşme için `"domain": ["a.com"]`'yi kullanın |
| `"domain:clstorage.net"` | `"domain_suffix": ["clstorage.net"]` |
| `"full:a.com"` | `"domain": ["a.com"]` |
| `"regexp:..."` | `"domain_regex": ["..."]` |
| `"ip": ["geoip:cn"]` | `"rule_set": ["geoip-cn"]` |
| `"domain": ["geosite:cn"]` | `"rule_set": ["geosite-cn"]` |
| `"outboundTag": "direct"` | `"outbound": "direct"` |

Notlar:

- **Etiketler çekirdekler arasında paylaşılmaz**: xray giden etiketi, sing-box kuralında kullanılamaz;
 sing-box "Gidenler" veya "Uç Noktalar" altında gerçekten mevcut olan bir etiketi kullanın.
- Yanlış bir giden veya uç nokta etiketi `outbound not found` raporunu veriyor; yanlış bir kural kümesi etiketi raporu
 `rule-set not found`.
- Doğrudan / izin verilenler listesi kurallarını ilk sıraya koyun; Bir kural eşleştiğinde daha sonraki kurallar artık geçerli olmaz.

### 6.7 Yönlendirme kuralları sıralaması ve aç/kapat anahtarı (xray / sing-box)

Yönlendirme kuralları yukarıdan aşağıya doğru eşleştirilir ve ilk eşleşmede durur; bu nedenle **sıra önceliktir**. İki çekirdeğin de yönlendirme sayfası şunları destekler:

- **↑ / ↓ ile sıralama**: Çıkışlar sayfasındakiyle aynı. Xray değişikliği hemen kaydeder ve çalışan çekirdeğe uygular; sing-box için sağ üstteki «Kaydet» düğmesine basın (yollar için sıcak yeniden yükleme yoktur, gerektiğinde çekirdek yeniden başlatılır). İlk öğe yukarı, son öğe aşağı taşınamaz.
- **Aç/kapat anahtarı**: her kuralın solundaki anahtar. Devre dışı kurallar listede kalır (yenilemeden sonra görünür ve yeniden açılabilir) ancak çalışma yapılandırmasına yazılmaz. Devre dışı bir kuralı düzenlemek onu otomatik olarak etkinleştirmez.
- **Abonelik biçimi şablonları**: devre dışı kurallar filtrelenir ve panel içi alanlar istemciye gönderilmez.
- **Varsayılan taban sırası**: yeni düğümlerde ve mevcut düğümlerin tek seferlik taşınmasında `gstatic.com` / `googleapis.com` doğrudan bağlantısı (bağlantı kontrolü) en önce gelir, ardından özel ağ doğrudan bağlantısı ve reklam engelleme gelir. Taşıma yalnızca mevcut kuralları yeniden sıralar, silinenleri geri getirmez.

> Doğrudan / beyaz liste kurallarını reklam engelleme veya proxy kurallarından önce koyun; aksi hâlde sonraki kuralların gölgesinde kalırlar.

---

## 7. Proxy yönetimi

Proxy yönetimi, Caddy veya Nginx ters proxy'yi yapılandırır.

### 7.1 Proxy türleri

- **Caddy**: otomatik HTTPS, HTTP/1.1 ve HTTP/2 (HTTP/3 / QUIC, katman4 yönlendirmeyle UDP 443 çakışmasını önlemek için devre dışı bırakıldı)
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
- **SNI yönlendirme**: gelen bir TLS / REALITY kullandığında ve istemciler her zaman 443'e bağlandığında, panel aşağıdakileri türetir:
 ClientHello SNI'den 443 TCP yönlendirme kuralları (hem xray `streamSettings` hem de sing-box `tls.reality`);
 ön proxy yalnızca TCP'yi iletir ve TLS hala çekirdek içinde sonlandırılır

---

## 8. Yönlendirme kaynakları

Yönlendirme kaynakları, abonelikler ve yönlendirme tarafından kullanılan paylaşılan varlıkları toplar. Sayfada beş sekme vardır:

| Sekme | Amaç |
|-----|---------|
| Doğrudan ve CF tercihli alanlar | Aboneliklerdeki ve harici düğümlerdeki erişim adreslerini yeniden yazarken kullanılan doğrudan etki alanları / IP'ler ve Cloudflare tercih edilen etki alanları |
| CF IP havuzu | "Erişim etki alanı" tarafından tüketilen tercih edilen IP kaynakları, bağlantı noktaları ve araştırma ayarları |
| WAF genel kurallar | Ters proxy WAF için düğümlere aktarılan kural kaynakları ve kural dosyaları |
| Kural dosyaları | Kural dosyalarının indirilmesi, programlı senkronizasyonu ve iletilmesi (`.srs` / `.json` / `.dat`) |
| Dış düğümler | v2ray URI / Base64 / URL aboneliğinden harici düğümleri içe aktarın (bkz. 16.3) |

### 8.1 Doğrudan ve CF tercihli alanlar

- **Doğrudan erişim**: giriş alanı veya IP, varsayılan olarak 443 numaralı bağlantı noktası; düğümleri bağlayın veya genel için boş bırakın;
- **CF tercih edilen etki alanları**: Cloudflare tercih edilen etki alanı artı bağlantı noktası, bağlantı kontrolü ile;
- Her ikisi de abonelikler ve harici düğüm yeniden yazma tarafından kullanılan "Erişim etki alanı" adaylarını besler (bkz. 16.3).

- **Giriş kayması (birincil/yedek)**: doğrudan giriş, bir DNS kanalına ve kayıt adına bağlanabilir; bir yük devretme veya geri dönüş sonrasında panel, A'in geçerli birincilin genel IP'sine (boş olduğunda dahili IP) kayıt yaptığını gösterir. Yalnızca uygulanan DNS yazıcısına sahip sağlayıcılar kabul edilir (`cloudflare` doğrulandı; alidns / DNSPod / GoDaddy / Namecheap / Route53 / Google / Huawei Cloud uygulandı ancak doğrulanmadı). Cloudflare yayılımı, TTL 60'a sabitlenmiş olarak yaklaşık 2–2,5 dakikada ölçülmüştür; TCP/REALITY girişleri için turuncu bulutu uzak tutun.

### 8.2 CF IP havuzu

- Tercih edilen IP kaynaklarını (`bestproxy` tarzı kaynaklar dahil), getirme aralığını ve bağlantı noktalarını
  koruyun;
- "Prob ayarları", prob aralığını, gecikme eşiğini, giriş düğümünü ve yürütme düğümünü (panel ana bilgisayarı
  veya herhangi bir Agent) artı isteğe bağlı bir çıkış NIC'sini kontrol eder; bir giriş düğümü seçildiğinde bu
  hat üzerinden arama yapılır, giriş olmadan yalnızca yerel doğrudan arama mümkündür (yalnızca erişilemeyen
  IP'leri açıkça filtreler);
- Çevirici çekirdeğinin yüklü olması gerekir **seçilen yürütme düğümü**: sayfada o düğüm tarafından bildirilen
  çekirdekler ve sürümler listelenir ve Kur / Yeniden Yükle onu doğrudan buraya yükler (uzak düğümlerin Agent
  aracılığıyla indirilmesi, SSH gerekmez);
- "Çevirici" otomatik (kurulu çekirdekler), `singbox`, `xray` veya `mihomo` olabilir; Kurulu olmayan
  çekirdekler listelenmez. Araştırma, tamamlandığında çıkan ve çekirdeğin kendi yapılandırmasını hiçbir zaman
  okumayan veya yazmayan geçici bir süreç olarak çalışır;
- "Bağlantıyı kontrol et" yalnızca bulunduğunuz sayfayı kapsar: CF IP havuzu sayfası tercih edilen IP'leri
  günceller, paylaşılan etki alanı sayfası CF etki alanlarını günceller ve ikisi de diğerine dokunmaz; bir tür
  olmadan yine de her ikisini de kontrol eder (geriye dönük olarak uyumludur);
- "Zincir proxy probu" ayrı bir karttır: seçilen çıkıştan bir CDN etki alanı/IP'si aracılığıyla gelen proba
  geri döner ve gerçek bir istek gönderir (her iki yönde de trafik gerektiren HTTP 200 gereklidir). Tercih
  edilen IP probu ile birbirini dışlar **programda**— etkinleştirildiğinde, planlanmış tercih edilen IP
  incelemesi duraklatılır ve yalnızca zincir proxy incelemesi çalıştırılır (manuel "Bağlantıyı kontrol et"
  hala çalışır); araştırma aralığını ve çeviriciyi devralabilir veya geçersiz kılabilir ve ayrıca bir yürütme
  düğümünü ve çıkış NIC'sini destekler;
- “Erişim alanı” CF IP havuzunu somut bir değer olmadan kullandığında, bir giriş (bağlantı noktasıyla
  birlikte) rastgele seçilir.

### 8.3 WAF genel kuralları

- WAF kural kaynağı URL'yi ve senkronize edilecek kural dosyalarını yapılandırın;
- Senkronizasyondan sonra bunlar düğümlere gönderilir ve düğüm proxy'si (Nginx / Caddy) tarafından yüklenir;
- Tam URL içeren bir dosya bu URL'den indirilir; aksi takdirde “kaynak URL + /dosya adı” kullanılır;
- "Önbelleği eşleştir" (`match_cache` / `config_match_cache`) varsayılan olarak etkindir: süreç genelinde sınırlanmış
 Sıcak kural eşleşmesi sonuçlarının önbelleği, istek başına kural geçişini azaltır. Yalnızca küreseldir ve hiçbir
 etki alanı başına geçersiz kılma; Yanlış pozitifleri araştırırken A/B karşılaştırması için geçici olarak devre dışı bırakın
 (devre dışı bırakmak, her istekte tam kural değerlendirmesi ve daha yüksek CPU kullanımı anlamına gelir).

### 8.4 Kural dosyaları (yönlendirme kuralı dosyaları)

Bu sekme yalnızca paneldeki kural dosyalarını indirir ve bunları düğümlere aktarır; hiçbir şey etkili olmuyor
otomatik olarak. Bunları yönlendirmede kullanmak için "Protokol merkezi → sing-box → Yönlendirme" altında bunlara başvurun.
Kural Kümeleri ve Kurallar'da (bkz. 6.6).

- **Kural depoları**: `owner/repo` (GitHub, göz atılabilir) girin veya URL'nin tamamını indirin; planlanmış
 senkronizasyon her 6 / 12 saatte bir, her gün saat 03:00'te, her gün gece yarısı veya haftalık Pazar günü çalıştırılabilir;
- **Dosya ekle**: depo sürümü dışındaki tek dosyalar için (örneğin SagerNet `.srs` şubesi)
 dosyalar); URL'nin tamamını girin; otomatik olarak indirilir;
- **Dosya listesi**: dosya başına "İndirme" ile birlikte adı, boyutu, son güncellemeyi ve "Otomatik senkronizasyon" anahtarını gösterir;
 üstteki "Tüm düğümlere gönder" düğmesi her kural dosyasını tüm düğümlere aktarır;
- Dosyalar panelin `data/rules/`'sine indirilir ve gönderildiğinde düğüm verileri altında yayınlanır
 dizinin `rules/`'si; sing-box'de bunlara `rules/<file name>` olarak bakın;
- Formatlar:
  - `.srs`: sing-box ikili kural seti, önerilen ve en küçük;
  - `.json`: kaynak formatı kural seti; `rule_set`, `format: "source"`'yi ayarlamalıdır;
  - `.dat`: xray `geosite.dat` / `geoip.dat`, sing-box'nin doğrudan kullanamayacağı.

> Sayfanın üst kısmındaki daraltılabilir "Kullanım" bloğu referans örneklerini, xray eşlemesini içerir
> tablo ve yaygın hatalar.

### 8.5 Dış düğümler

Harici düğümlerin içe aktarılması ve yönetilmesi [16.3 Harici düğümler](#163-external-nodes) kapsamında ele alınmaktadır.

---

## 9. Abonelik merkezi

Abonelik merkezi, düğümleri istemciye hazır abonelikler halinde paketler: abonelik **biçimlerini** (ne yayınlanacağını, hangi kuralları, hangi planların bunları görebileceğini) tanımlayın, ardından abonelik **uç noktaları** (genel alan / yol) ve ardından bağlantıları dağıtın.

### 9.1 Abonelik formatları

“Abonelik merkezi” → “Abonelik biçimleri”; istemci çıktısı başına bir satır:

| Alan | Açıklama |
|---|---|
| İsim | Benzersiz; `?config=<name>` onunla eşleşiyor ve müşteri iletişim kutusu bunu gösteriyor |
| Müşteri türü | clash / shadowrocket / singbox / v2ray — yalnızca ekran gruplaması |
| Çıkış formatı | `mihomo_yaml` (Clash/Mihomo), `singbox_json`, `xray_json`, `base64_uri`, `raw_uri` |
| İçerik Türü / uzantı | Yanıt başlığı ve dosya adı |
| platformu | kazan / mac / android / ios / evrensel; `?platform=` ile eşleşti, boş = evrensel |
| Bağlı planlar | Boş = her planda kullanılabilir; aksi halde yalnızca bu planlar onu kullanabilir |
| Kurallar | Yönlendirme kurallarına uygun format (Mihomo için yapılandırılmış düzenleyici, sing-box / Xray için kural başına ham JSON). **Kurallar formatlar arasında dönüştürülmez** |

Notlar:

- Aynı çıktı formatı için çeşitli konfigürasyonlar mevcut olabilir (örn. `out`, `home`); bağlantıda `config=` olan birini seçin;
- `config` olmadan, sipariş "aboneliğe bağlı yapılandırma → `platform` eşleşmesi → varsayılan" şeklindedir;
- Bir sing-box şablonu, istemciler için kendi `route.rule_set`'sini (normalde `remote` olarak) taşıyabilir, dolayısıyla
 istemci onu indirir; "Kural dosyaları" altında bir düğümün `rules/`'sine gönderilen yerel dosyalar yalnızca kullanılabilir
 istemci aboneliklerine değil, o düğümdeki çekirdeğe;
- sing-box / Xray şablonları oluştururken panel sarkan referansları kaldırır (`final`, `dns.final`,
 `default_domain_resolver`, `download_detour`, yerel `rule_set`, geçersiz `rule_set` tanımları, boş
 `balancer`, vb.) böylece istemci başlangıçta ÖLÜMCÜL bir hatayla başarısız olmaz;
- Kurallardaki giden takma adlar, gerçek etiketlere göre çözümlenir (`REJECT` / `blocked` → `block`, `DIRECT` →
 `direct`, büyük/küçük harfe duyarlı değil); çözümlenemeyen gidenler varsayılan gidenlere ve kurala geri döner
 konumunu koruyor;
- Her zaman `tag = proxy`'ye sahip bir grup oluşturulur: sing-box için bir `selector` ve `random` için bir `random` dengeleyici.
 Kurallardaki Xray ve `outbound: proxy`, eşleşen `balancerTag`'ye yeniden yazılmıştır. Kural düzenleyici yüklenir
 "Proxy gruplarından + özel gidenlerden" dinamik olarak giden adaylar ve manuel olarak girilen etiketler
 hala doğrulandı;
- sing-box / Xray yapılandırmaları ayrı olarak oluşturulmalıdır; mevcut değilse müşteri bağlantılarında görünmezler (bkz. 10.3).

- Mihomo'in yapılandırılmış kural düzenleyicisi mantıksal kuralları destekler `AND` / `OR` / `NOT` / `SUB-RULE`: koşulu `((NETWORK,UDP),(DST-PORT,3478/19302))` olarak yazın; `SUB-RULE` için üçüncü sütun alt kural grubu adıdır. Koşullar parantez içine alınmalıdır;
- Kural türleri en son mihomo ile uyumludur: `IP-SUFFIX`, `SRC-IP-CIDR/SRC-IP-SUFFIX/SRC-IP-ASN/SRC-GEOIP`, `IN-PORT/IN-TYPE/IN-USER/IN-NAME/REMATCH-NAME`, `PROCESS-NAME-WILDCARD/REGEX`, `PROCESS-PATH-WILDCARD/REGEX`, `UID`, `DSCP`'in tümü mevcuttur;
- `SUB-RULE`, mihomo yapılandırmasının `sub-rules:` bloğuna başvurur; panel henüz bu bloğu düzenlemediğinden, önce onu YAML içe aktarma veya paylaşılan config_json aracılığıyla tanımlayın.

### 9.2 Abonelik uç noktaları

“Abonelik merkezi” → “Abonelik uç noktaları”; genel adres başına bir satır:

| Alan | Açıklama |
|---|---|
| Etki alanı | Herkese açık abonelik alanı |
| Yol | Etki alanı altındaki önek, varsayılan `/` |
| Etkin | Bağlantılar oluşturulurken devre dışı bırakılan uç noktalar göz ardı edilir |
| Açıklama | Serbest metin notu |

- **Birden çok satır (çoklu alan adı)** desteklenir; birkaçı etkinleştirildiğinde, sağlayıcı etki alanı mevcut istek tarafından kullanılan etki alanını takip eder (bkz. 9.5);
- Kökte özel bir abonelik bağlantı noktası hizmet verir (`/<token>`, `/sub` yok); panel bağlantı noktasını paylaşırken `/sub/<token>`'dir;
- Ters proxy'ler `X-Forwarded-Proto` / `X-Forwarded-Host`'yi iletmelidir, aksi takdirde ilk etkin uç nokta kullanılır. Panelin oluşturulan Caddy / Nginx site şablonları her iki başlığı da içerir; elle tutulan proxy yapılandırmalarının manuel olarak eklenmesi gerekir.

### 9.3 Abonelik bağlantıları

Temel form: `https://<sub-domain>/<token>` (panel bağlantı noktası: `https://<panel-domain>/sub/<token>`)

| Parametre | Açıklama |
|---|---|
| `format` | Çıkış formatı; takma adlar: `clash` → mihomo_yaml, `singbox`/`sing-box` → singbox_json, `xray` → xray_json, `raw` → raw_uri, `base64`/`v2ray` → base64_uri |
| `config` | Ada göre bir abonelik biçimi uygulayın; `format` ile eşleşmelidir, aksi halde dikkate alınmaz |
| `platform` | Bir yapılandırmayı platforma göre eşleştirin (`config` atlandığında) |
| `providers` | `1` / `true`: Clash Sağlayıcı modu (`proxy-providers` + `use`); varsayılan satır içi düğümler |

Örnekler:

```bash
# Clash dynamic subscription (config template + providers)
https://sub.example.com/<token>?format=clash&config=out&providers=1

# sing-box / Xray (only meaningful when such a config exists)
https://sub.example.com/<token>?format=singbox_json&config=sing-box-home
https://sub.example.com/<token>?format=xray_json&config=xray-home

# generic clients / plain text
https://sub.example.com/<token>?format=base64_uri
https://sub.example.com/<token>?format=raw_uri
```

### 9.4 Abonelik kısa bağlantıları (isteğe bağlı)

Her abonelik satırındaki "Kısa bağlantı" düğmesi 20 karakterlik rastgele bir kod oluşturur:

```text
https://<sub-domain>/<20-char code>     # dedicated subscription port
https://<panel-domain>/<code>           # shared panel port (prefix follows the endpoint path)
```

- Kısa bağlantılar abonelik gövdesini doğrudan döndürür (302 yok) ve uzun bağlantılarla aynı yetkilendirmeyi kullanır: iptal edilmiş/askıya alınmış/süresi dolmuş abonelik, yasaklı istemci, token rotasyonu, devre dışı/süresi dolmuş bağlantı anında başarısız olur.
- Bağlantı format / yapılandırma / platform / providers değerlerini sabitler; istemci artık `?format=&config=` eklemez. Oluştururken `?format=` ile geçici geçersiz kılmaya izin vermek için "parametreleri kilitle"yi kapatın.
- Portal ve istemci ayrıntıları kısa bağlantıları tercih eder. Genel anahtar Ayarlar → Panel → Abonelik kısa bağlantıları altındadır; kapatıldığında tüm kısa bağlantılar 404 döner, uzun bağlantılar etkilenmez. Erişim günlükleri `access_type=short_link` ve `short_link_id` kaydeder.
- Portal/istemci ayrıntılarındaki bağlantılar görünen her "format × yapılandırma (× providers)" kombinasyonu için otomatik oluşturulur; "kısa bağlantı + parametre" karışımı hiç oluşmaz.
- `mihomo_yaml` kısa bağlantıları bir yapılandırmaya bağlı olmalıdır: sing-box / Xray gibi, bağlantılar yalnızca "Abonelik formatları" altındaki gerçek yapılandırmalardan üretilir; yapılandırmasız Mihomo bağlantıları artık üretilmez, "Kısa bağlantılar" penceresi açıldığında mevcut yapılandırmasız bağlantılar temizlenir ve yapılandırmasız manuel oluşturma/güncelleme reddedilir.
- Kısa kodlar asla düz metin olarak saklanmaz: veritabanı yalnızca rastgele AES-256-GCM şifreli metni (tam URL'yi göstermek için) ve HMAC-SHA256 parmak izini (eşitlik araması için) tutar; döndürülen/silinen kodlar bir iptal tablosuna yazılır ve artık çözülemez.

### 9.5 Çoklu etki alanı ve Sağlayıcı modu

`providers=1` ile her `proxy-providers` girişi, abonelik uç noktasına geri döner. Alan adı seçimi:

1. **Bu isteğin kullandığı etki alanı** (`X-Forwarded-Host` ilk değer, bağlantı noktası çıkarılmış, aksi takdirde `Host`) **etkin** bir uç noktayla eşleşiyorsa;
2. Aksi takdirde **en düşük kimliğe** sahip etkin uç nokta;
3. Yol şu uç noktadan geliyor: `https://<domain><path>/<token>?format=mihomo_proxies_yaml...`.

Yani alan adını değiştirmek için panel değişikliği gerekmez: yeni alan adı aracılığıyla gelen istemciler yeni alan adını sağlayıcıların içinden alır ve etkin bırakılan eski alan adı artık sabit kodlanmaz. Bir istemci eski sağlayıcı URL'yi önbelleğe aldıysa aboneliği güncelleyin (veya sağlayıcı önbelleğini temizleyin/istemciyi yeniden başlatın).

Ek notlar:

- Paylaşılan bir formattaki (`config_json`) `proxy-providers` göz ardı edilir (planlar arası sızıntıyı önler); sağlayıcılar plan düğüm gruplarından ve dış kaynaklardan gelir;
- Dahili ve harici sağlayıcı önbellek yolları her zaman "dahili sağlayıcı varsayılanları → önbellek yolu şablonundan" (varsayılan `./providers/{key}.yaml`) türetilir; dış kaynaklar kendilerininkini belirleyemez;
- Eski ayrılmış sağlayıcı `subscription-nodes` artık yayınlanmamaktadır: "tüm düğümleri" kullanan gruplar, somut sağlayıcılara veya planda hiçbiri bulunmadığında satır içi düğümlere genişler.

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
4. Kaydetmek.

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

Müşteri yönetimi → “Abonelik” sütunu → “Ayrıntılar”. İletişim kutusunda müşterinin planında mevcut olan her formatın bağlantıları listelenir:

- **Clash / Mihomo**: iki sıra — satır içi düğümler ve `providers=1` (proxy sağlayıcıları), her ikisi de `config=<name>` taşır;
- **Sing-box / Xray**: yalnızca bu formatta bir yapılandırma mevcut olduğunda ve planda görülebildiğinde gösterilir; bağlantılar `config=<name>` taşır;
- **Base64 URI / Ham URI**: her zaman gösterilir (`config` yok);
- Her satırın kendi kopyalama düğmesi vardır; bir satıra tıklamak aşağıdaki QR kodunu bu formata dönüştürür.

Dolayısıyla yalnızca Clash yapılandırmasıyla iletişim kutusunda Clash + Base64 + Raw; sing-box / Xray bağlantıları yalnızca yapılandırmaları oluşturulduktan sonra görünür.

Aynı iletişim kutusu aynı zamanda trafik kullanımını, sipariş geçmişini ve işlem geçmişini de gösterir.

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
2. Kaydet.

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

> **Ödeme entegrasyonu**
>
> Panel **ödeme kurumlarıyla** (Alipay, WeChat Pay, Stripe, bankalar vb.) doğrudan entegre değildir. Yalnızca **ara ödeme platformuna** bağlanır; bu platform da ilgili ödeme kurumuyla entegre olur, sözleşme, satın alma ve ödeme işlemlerini gerçekleştirir ve ödeme sonuçlarını [charge geri arama API](api/charge-callback.md) aracılığıyla geri bildirir.
>
> Ara ödeme platformu **bu projenin bir parçası değildir; operatörler bunu kendileri geliştirmeli (veya kendi sistemlerini kullanmalıdır)**. Dağıtıldıktan sonra, onu "Yeniden yükleme siteleri" altında kaydedin ve yeniden yükleme giriş noktası olarak kullanmak için alan adı doğrulamasını tamamlayın; Buradaki şarj sitesi ödeme kuruluşunun kendisi değil, o ara platformdur. Ödeme nitelikleri, ücretler, ödemeler ve uyumluluk, operatör, ödeme kurumu ve aracı platform tarafından kararlaştırılır ve karşılanır.

### 12.1 Şarj yerleri

1. “Şarj yönetimi” → “Şarj siteleri”ni açın;
2. Bir site ekleyin:
   - Site adı
   - Bu ara platformun desteklediği ödeme yöntemleri (ödeme kurumlarıyla entegrasyonu sayesinde sağlanan Alipay, WeChat, USDT, Stripe vb.)
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
- Harici düğümleri yönetin (tek tek düzenleyin / silin)
- Erişim etki alanı: düğüm başına CDN hızlandırmayı / tercih edilen IP'yi yapılandırın (doğrudan etki alanı, CF tercih edilir)
 etki alanı, CF IP havuzu, özel). Yalnızca aboneliklerde sunulan sunucu/bağlantı noktası yeniden yazılır;
 SNI/Ana bilgisayar içe aktarıldığı şekilde kalır. Devre dışı bırakıldığında içe aktarılan adres/bağlantı noktası kullanılır
- Belirli bir değer olmadan "CF tercih edilen alan adı / CF IP havuzu" seçildiğinde bir giriş seçilir
 havuzdan rastgele (limanıyla birlikte); "doğrudan / özel" açık bir değer gerektirir, aksi takdirde
 düğümün kendi adresine geri döner

---

## 17. Çevrimiçi kullanıcılar

Çevrimiçi kullanıcılar gerçek zamanlı çevrimiçi durumu gösterir.

### 17.1 Ne gösterir?

- Çevrimiçi abonelikler
- Çevrimiçi cihazlar / IP'ler
- Her abonelik için düğüm, IP ve bağlantı süresi
- Gerçek zamanlı yukarı bağlantı / aşağı bağlantı oranı
- Toplam yukarı bağlantı / toplam aşağı bağlantı / toplam trafik

> **Sürekli bir çevrimiçi oturum** boyunca toplam yukarı bağlantı, toplam aşağı bağlantı ve toplam trafik birikir:
> kısa ömürlü yeniden bağlanmalar, çekirdek yeniden başlatmaları ve sayaç sıfırlamaları bunları temizlemez. Sadece 60'tan sonra
> Yeni bir rapor olmadan saniyeler içinde yeni bir oturum sıfırdan başlar. Bu farklı bir ölçü
> çekirdeğin kendi "bu başlangıçtan bu yana trafik" sayacından gelir ve kullanıcının gerçekte ne yaptığını daha iyi yansıtır
> geçerli oturum sırasında kullanılır.

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

### 18.3 Kullanıcı eylemleri

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

"Operasyon uzantıları" (`/client-ext`), yönlendirme pazarlamasını, komisyonu ve para çekme işlemlerini yapılandırır;
"Yasal belgeler" (`/legal-documents`), portal tarafı anlaşmalarını korur.

### 25.1 Yönlendirmeler ve komisyon

- **İndirim yapılandırması**: indirim oranı (davetlinin ilk ödemesinin yüzdesi, `0` = devre dışı), komisyon modu (yalnızca ilk satın alma / tüm satın almalar), minimum para çekme tutarı (para çekme talebinde bulunulmadan önce komisyon bakiyesinin bu seviyeye ulaşması gerekir)
- **Davet kodları**: Etkinleştirildiğinde kayıt için bir davet kodu gerekir; kod uzunluğunu, son kullanma günlerini ve maksimum kullanımı ayarlayın (`0` = sınırsız) ve kod listesini oluşturun/görüntüleyin
- **Para çekme incelemesi**: beklemede / onaylandı / reddedildi şeklinde filtreleyin; bir işlem notu ekleyin ve ödeme yöntemini ve cüzdan adresini kaydedin
- **Yönlendirme kayıtları**: davetli, davetli, komisyon tutarı ve komisyon durumu

### 25.2 Yasal belgeler

- "Gizlilik Politikası" ve "Hizmet Şartları"nı ayrı ayrı, yerel ayar başına bir Markdown belgesini ve canlı önizlemeyi koruyun;
- Her belgede yerleşik sayfa yerine harici bir URL kullanılabilir; boş olduğunda yerleşik URL sayfası kullanılır (sayfaya kopyalanabilir);
- Portal, geçerli yerel ayarın sürümünü gösterir ve eksik olduğunda varsayılan yerel ayara geri döner.

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

- URL geri aramasını kontrol edin
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

# Non-interactive init / reset init state (see 3.4, 3.5)
./corefusion init --help
./corefusion reset-init

# Offline administration (when you cannot sign in)
./corefusion admin reset-password -u admin -p 'new-password'
./corefusion admin 2fa reset -u admin
./corefusion admin list

# Version / help
./corefusion version
./corefusion help
```

> systemd ile `systemctl restart corefusion` ve `journalctl -u corefusion -f`'yi kullanın; onsuz
> systemd (konteynerler ve benzeri), eski işlemi durdurun ve `nohup` ile yeniden başlatın, günlükler
> `data/logs/panel.log`'de.

---

## v0.13.0-rc.1 değişiklikleri

- **Panel çevrimiçi güncelleme**: «Sistem ayarları → Panel güncellemesi» — sürüm seçimi
  (`pre` = ön sürüm), güncelleme, görev geçmişi ve önceki ikiliye geri dönme.
- **Agent çevrimiçi güncelleme**: düğüm ayrıntılarında «Agent güncelle» ile paneldeki yerel
  ikili veya GitHub Releases kullanılır; eski sürümlerden güncellenen düğümler ilk bağlantıda
  `update.public_key` (0600) alır ve sonrasında ed25519 imzalarını doğrular.
- **WAF eşitleme**: son eşitleme zamanı `waf.github_source.last_sync` içinde tutulur; yeniden
  başlatma zamanlayıcıyı sıfırlamaz; `0 3 * * *` `panel.timezone` (varsayılan Asia/Shanghai)
  saat 03:00'te çalışır; kaçırılan pencereler başlangıçtan ~15 saniye sonra telafi edilir.
- **WAF beyaz listesi**: panel yönlendirme alan adı `whiteurl.rule` dosyasını otomatik tutar;
  URL/argüman/POST kontrolleri yalnızca yönetim API'si için atlanır, bu yüzden
  `http://127.0.0.1:18081` gibi ödeme siteleri SSRF kuralına takılmaz.
- **Kota politikası**: yenilemede ekle / süre bitiminde temizle / yenilemede sıfırla (varyant
  planı devralabilir); «aylık» seçiliyse panel saatte bir kontrol edip kullanımı sıfırlar.
- **Ödeme siteleri**: etkinleştirme/devre dışı bırakma anahtarları veya ödeme yöntemlerini
  değiştirmez; yükleme siparişleri türe göre filtrelenip CSV olarak dışa aktarılabilir.
- **Portal**: «Yükle» etkin bir ödeme sitesiyle çevrimiçi ödemeyi ve kart kullanımını destekler;
  site yoksa yalnızca kart gösterilir.

---

> **Belge sürümü**: v1.4
> **Son güncelleme**: 2026-11-27
> **Şunlar için geçerlidir**: CoreFusion v0.14.6
