<!-- source: notification-templates.md | locale: tr | updated: 2026-10-23 -->
# Bildirim Şablonu Değişkenleri

> Diller: [简体中文](notification-templates.md) · [English](notification-templates.en.md) · [繁體中文](notification-templates.zh-TW.md) · [Русский](notification-templates.ru.md) · [日本語](notification-templates.ja.md) · [한국어](notification-templates.ko.md) · [Deutsch](notification-templates.de.md) · [Français](notification-templates.fr.md) · [Español](notification-templates.es.md) · [Português](notification-templates.pt.md) · Türkçe · [Bahasa Indonesia](notification-templates.id.md) · [ไทย](notification-templates.th.md) · [Tiếng Việt](notification-templates.vi.md) · [العربية](notification-templates.ar.md) · [فارسی](notification-templates.fa.md)

Bildirim şablonları, gönderim sırasında gerçek değerlerle değiştirilen `{variable}` yer tutucularını kullanır. Bu belgede desteklenen tüm değişkenler, olay başına kullanılabilen değişkenler ve otomatik tetikleme kuralları listelenir.

## 1. Şablon türleri ve hedef kitleler

| Seyirci | Kanal | Alıcı | Tipik kullanım |
|---|---|---|---|
| **Panel kullanıcıları** (yönetici) | Telegram / E-posta | Yönetici tarafından yapılandırılan kanallar ve alıcılar | Düğüm uyarıları, yedekleme raporları, sistem duyuruları |
| **Aboneler** | E-posta | Müşterinin kendi e-postası (otomatik olarak doldurulur) | Düşük bakiye, abonelik sona erme hatırlatıcıları |

## 2. Ortak değişkenler

Tüm şablonlarda mevcuttur:

| Değişken | Açıklama | Örnek |
|---|---|---|
| `{time}` | Gönderme zamanı | `2026-09-28 14:30:00` |
| `{user}` | Müşteri e-postası (abone şablonları) / kullanıcı tanımlayıcı | `user@example.com` |
| `{message}` | Bildirim kuruluşu | Bakım duyuru metni |

## 3. Abone etkinlik değişkenleri

### Abonelik sona erme hatırlatıcısı (event_key = `subscription_expire`)

Günlük olarak taranır ve süresi 7 gün içinde dolacak aktif abonelere gönderilir:

| Değişken | Açıklama | Örnek |
|---|---|---|
| `{user}` | Müşteri e-postası | `user@example.com` |
| `{expire_date}` | Abonelik süresinin sona ermesi | `2026-10-05 09:00` |
| `{days}` | Kalan gün sayısı | `7` |
| `{time}` | Gönderme zamanı | `2026-09-28 09:05:00` |

### Düşük bakiye hatırlatıcısı (event_key = `balance_low`)

Günlük olarak taranır ve bakiyesi eşiğin altında olan müşterilere gönderilir:

| Değişken | Açıklama | Örnek |
|---|---|---|
| `{user}` | Müşteri e-postası | `user@example.com` |
| `{balance}` | Mevcut bakiye (para birimi birimleri, iki ondalık sayı) | `0.85` |
| `{time}` | Gönderme zamanı | `2026-09-28 09:05:00` |

## 4. Operasyon şablonu örnek değişkenleri

Bir bağlantı testi için "Gönder"i tıkladığınızda panel kullanıcı şablonları yer tutucuları şu örneklerle doldurur:

| Değişken | Açıklama | Örnek |
|---|---|---|
| `{node}` | Düğüm adı | `node-01` |
| `{backup_file}` | Yedekleme dosyası adı | `corefusion-20260101-0900.tar.gz` |
| `{size}` | Dosya boyutu | `128MB` |
| `{domain}` | Etki alanı | `example.com` |
| `{expire_date}` | Son kullanma tarihi | `2026-10-12` |
| `{days}` | Kalan gün sayısı | `14` |
| `{count}` | Sayım | `3` |

## 5. Otomatik tetikleme kuralları

- **Program**: ilk tarama panel başladıktan 30 saniye sonra gerçekleştirilir, ardından her gün **09:05**'te çalıştırılır
- **Abonelik sona erme tarihi**: süresi 7 gün içinde dolan aktif abonelikleri (`status=active`) tarar; müşterinin de aktif olması gerekir
- **Düşük bakiye**: Bakiye ≤ eşiği olan aktif müşterileri tarar; eşik `notify.balance_threshold`'dir, **sent cinsinden** (varsayılan `100` = 1,00)
- **Tekilleştirme**: olay başına müşteri başına en fazla bir mesaj **gün başına** (`notification_send_logs` gün bazında tekilleştirmeler)

## 6. Düğüm sağlığı izleme

Düğümün kalp atışları sürekli olarak kontrol edilir; **bildirimler yalnızca durum değiştiğinde gönderilir; değişiklik olmayan turlar sessizdir**:

| Ayar tuşu | Açıklama | Varsayılan |
|---|---|---|
| `monitor.node_interval_seconds` | Kontrol aralığı (saniye) | `60` (minimum 30) |
| `monitor.node_offline_timeout_seconds` | Kalp atışı zaman aşımı (saniye) | `120` |

- **Çevrimdışı uyarı**: Bir düğüm çevrimiçi durumdan çevrimdışına geçtiğinde, `{node}` (ad) ve `{time}` içeren "**Düğüm çevrimdışı uyarısı**" yönetici şablonunu gönderir
- **Kurtarma bildirimi**: Bir düğüm kurtarıldığında aynı değişkenlerle "**Düğüm kurtarma bildirimi**" gönderir
- **Sessiz turlar**: durum değişmediğinde mesaj yok; Panel yeniden başlatıldıktan sonra ilk tur yalnızca bir temel oluşturur
- **Göndermeden izle**: ilgili şablonu devre dışı bırakın (Etkin = KAPALI)

## 7. Düğüm aylık trafik kotası uyarıları (otomatik)

- **Tetikleyici**: Bir düğümün aylık trafiği kota eşiğine ulaştığında, “**Düğüm trafiği kota uyarısı**” gönderir; %100'ü aştığında ve askıya alındığında, limit aşımı uyarısı gönderir (olay anahtarları `traffic_quota_threshold` / `traffic_quota_exceeded`)
- **Eşik**: `monitor.traffic_quota_threshold_percent` (yüzde, varsayılan `80`; eşik uyarılarını devre dışı bırakmak için `0`'yi ayarlayın)
- **Tekilleştirme**: faturalandırma döngüsü başına düğüm başına en fazla bir tane (döngü, düğümün `traffic_quota_reset_day`'sini takip eder)
- Mevcut değişkenler: `{node}` (ad), `{used}` (kullanılan GB), `{quota}` (GB kotası), `{percent}` (yüzde), `{time}`

## 8. Yedekleme sonucu bildirimleri (otomatik)

- **Tetikleyici**: planlanmış bir yedekleme (`BackupScheduler`) çalıştırıldıktan hemen sonra gönderilir — başarı durumunda "**Günlük yedekleme raporu**" gönderilir, başarısızlık (yedek oluşturma / depolamayı okuma / uzaktan yükleme) "**Yedekleme hatası uyarısı**" gönderilir
- Mevcut değişkenler: `{backup_file}` (ad), `{size}` (yalnızca boyut, başarı raporu), `{message}` (arıza nedeni, yalnızca arıza uyarısı), `{time}`
- **Göndermeden izle**: şablon devre dışı bırakıldığında sessizce atlanır (Etkin = KAPALI)

## 9. Sertifika geçerlilik sonu hatırlatıcıları (otomatik)

- **Tetikleyici**: her gün saat 09:05'te, süresi dolmak üzere olan `active` sertifikalarını tarar (**3 gün kalan IP sertifikaları**, **7 gün kalan alan adı sertifikaları**) ve sertifika başına "**Sertifika süre sonu hatırlatıcısı**" yönetici şablonunu gönderir
- **Tekilleştirme**: Sertifika başına günde en fazla bir hatırlatma; yenilemeden sonra süre sonu pencerenin dışına çıkar (IP 3 gün / alan adı 7 gün) ve hatırlatmalar durur
- Mevcut değişkenler: `{domain}` (sertifika alanı), `{expire_date}` (son kullanma tarihi), `{days}` (kalan gün sayısı), `{time}`

## 10. Temel süreç kilitlenme uyarıları (otomatik)

- **Tetikleyici**: bir çekirdek işlem (xray/singbox) anormal bir şekilde çıktığında veya proxy işlemi (caddy/nginx) çöktüğünde, derhal "**Çekirdek işlem kilitlenme uyarısı**" yönetici şablonunu gönderir; süpervizör daha sonra üstel geri çekilmeyle bunu yeniden başlatır
- Mevcut değişkenler: `{core}` (işlem türü), `{restarts}` / `{max}` (yeniden başlatma sayısı/limit), `{message}` (çıkış mesajı, yalnızca çekirdek işlemler), `{time}`
- **Göndermeden izle**: şablon devre dışı bırakıldığında sessizce atlanır (Etkin = KAPALI)

## 11. Yeni kullanıcı kayıt bildirimleri (otomatik)

- **Tetikleyici**: Müşteri başarıyla kaydolduktan hemen sonra "**Yeni kullanıcı kayıt bildirimi**" yönetici şablonu kullanılarak gönderilir
- Mevcut değişkenler: `{user}` (müşteri e-postası), `{time}`

## 12. Kullanım

1. **Şablon ekle**: Bildirim ayarları → Bildirim şablonları → Şablon ekle → "Ön ayardan doldur" ve gerektiği gibi ayarlayın
2. **Etkinleştir**: Etkin'i işaretleyin ve kaydedin; otomatik tetikleyiciler yalnızca etkin şablonları işler
3. **Test**: Örnek değişkenlerle gerçek bir mesaj göndermek için listedeki "Gönder"e tıklayın (abone şablonu test e-postaları yöneticinin gönderen adresine gider)
4. **Anında bildirimler** (iş kodu): Günlük tarama limiti dışında tek bir müşteriye anında bildirimde bulunmak için etkinlik yerinde `NotifyClientEvent(ctx, db, settingSvc, eventKey, clientEmail, vars)`'yi arayın (şarj etme, kesinti, yenileme)
