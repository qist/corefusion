<!-- source: cert-google-eab.md | locale: tr | updated: 2026-10-23 -->
# Google Genel CA Kılavuzu (otomatik EAB düzenleme)

> Diller: [简体中文](cert-google-eab.md) · [English](cert-google-eab.en.md) · [繁體中文](cert-google-eab.zh-TW.md) · [Русский](cert-google-eab.ru.md) · [日本語](cert-google-eab.ja.md) · [한국어](cert-google-eab.ko.md) · [Deutsch](cert-google-eab.de.md) · [Français](cert-google-eab.fr.md) · [Español](cert-google-eab.es.md) · [Português](cert-google-eab.pt.md) · Türkçe · [Bahasa Indonesia](cert-google-eab.id.md) · [ไทย](cert-google-eab.th.md) · [Tiếng Việt](cert-google-eab.vi.md) · [العربية](cert-google-eab.ar.md) · [فارسی](cert-google-eab.fa.md)

> Şunlar için geçerlidir: panel v0.12.101+ | Sayfa: **Sertifikalar → EAB yapılandırması**
> Kod: `internal/service/certificate/eab_google.go`, `internal/api/handlers/certificates/eab_config.go`

## 1. Google neden EAB gerektiriyor?

| CA | ACME için EAB zorunlu mu? | EAB nasıl edinilir |
|---|---|---|
| Haydi Şifreleyelim | ❌ Hayır | — |
| ZeroSSL | ✅ Evet | **Bir e-posta karşılığında bir e-posta alın** (`api.zerossl.com/acme/eab-credentials-email`, acme.sh ile aynı); panel onu otomatik olarak getirebilir |
| **Google Genel CA** | ✅ **Evet** | **GCP yetkilendirmesi ile oluşturulmalıdır** (e-posta yeterli değildir) → bu sayfanın odak noktası |

Başka bir deyişle, Google **"yetkilendirmeden" kaçınamaz**: bir hizmet hesabı JSON dosyası veya geçici bir erişim belirteci.

## 2. Google'nin katı kuralları (önce okuyun)

| Kural | Açıklama |
|---|---|
| **7 gün içinde kullanın** | EAB anahtarının 7 gün içinde kullanılmaması durumunda **süresi dolar** ve yeniden oluşturulması gerekir |
| **Yalnızca bir hesap** | Bir EAB anahtarı tam olarak bir ACME hesabını kaydeder; kayıttan sonra anahtar **geçersizdir** ve yeniden kullanılamaz |
| **ACME hesaplarının süresi dolmaz** | Bununla oluşturulan ACME hesabı **uzun ömürlüdür** → **yenilemeler için EAB'ye gerek yoktur** |
| ⚠ **GCP projesinin silinmesi tüm hesapların kapatılmasına neden olur** | Projenin silinmesi, altındaki **tüm ACME hesaplarının** anında geçersiz kılınmasına neden olur ve **yenileme imkansız hale gelir** |

> Sonuç: **EAB tek seferlik bir "ilk katılım kimlik bilgisidir"; ACME hesabı kalıcıdır.**
> Panel, hesapları `data/acme/accounts/<ca>.json` adresinde saklar — **bu dizini yedekleyin**.

## 3. Üç yol (A yolu ile başlayın — menü arama yok)

> Cloud Console giriş noktaları birkaç kez değiştiği için doğrudan bağlantılar ve Cloud Shell önce gelir; menü yolları geri dönüşlerdir.

### Yol A (en kolayı, önce önerilir): GCP Cloud Shell'de tek komut

Cloud Shell, **gcloud'un önceden yüklendiği ve önceden oturum açıldığı** bir tarayıcı terminalidir; yerel kurulum yok ve menü yok.

1. <https://console.cloud.google.com/cloudshell>'yi (veya konsolun sağ üst köşesindeki `>_` simgesini) açın
2. Projeyi seçin (proje kimliği üstteki proje seçicide görünür):
   ```bash
   gcloud config set project <your-project-id>
   ```
3. Genel CA API'yi etkinleştirin (**eşdeğer**, API Kitaplığında etkinleştirmeye eşdeğerdir):
   ```bash
   gcloud services enable publicca.googleapis.com
   ```
4. EAB'yi oluşturun (**bunlar ihtiyacınız olan iki dizedir**):
   ```bash
   gcloud publicca external-account-keys create
   ```
 Çıktıda: `keyId` → panelin **EAB-KID**; `b64MacKey` → panelin **EAB-HMAC-Anahtarı**
5. Panele geri dönün: **Sertifikalar → EAB yapılandırması → EAB yapılandırmasını ekle → CA: Google Genel CA** →
 iki değeri `EAB-KID` / `EAB-HMAC-Key`'ye girin → kaydedin ✓

> ⚠ Maliyet: Google EAB anahtarları **7 günlük, tek seferliktir** ✗ → 7 gün sonra (veya hesap değiştirirken) **4. adımı tekrarlayın**.
> Bunu önlemek için B yolunu kullanın (bir hizmet hesabını bir kez saklayın ve panelin otomatik olarak yeniden talepte bulunmasına izin verin).

### Yol B (tek seferlik kurulum, ardından otomatik): hizmet hesabı anahtarı JSON

**① Genel CA API'yi etkinleştirin** (doğrudan bağlantı en kolayıdır)
<https://console.cloud.google.com/apis/library/publicca.googleapis.com?project=YOUR_PROJECT_ID> → “Etkinleştir”e tıklayın
Geri dönüş: ☰ → **API'ler ve Hizmetler → Kitaplık** → `Public CA`'de ara → aç → “Etkinleştir”

**② Hizmet hesabı oluşturun**
<https://console.cloud.google.com/iam-admin/serviceaccounts?project=YOUR_PROJECT_ID>
→ “**+ Hizmet hesabı oluştur**” → herhangi bir ad (örn. `corefusion-eab`) → “Oluştur ve devam et” → rol adımını **atlayabilirsiniz** → “Bitti”
Yedek: ☰ → **IAM ve Yönetici → Hizmet hesapları**

**③ Minimum rolü verin** (aynı sayfada en kolayı)
Yeni oluşturulan hesaba tıklayın → **"İzinler" sekmesi** → "Erişim izni ver" → yeni sorumlu: **hizmet hesabı e-postası**
→ `publicca` rolünü arayın → **“Genel CA Harici Hesap Anahtarı Oluşturucu”** seçeneğini seçin → kaydedin
Geri dönüş: ☰ → **IAM ve Yönetici → IAM** → “Erişim izni ver” → aynı e-posta + rol
⚠ **Sahip/Düzenleyiciye izin vermeyin** ✗

**④ JSON anahtarını indirin**
Hizmet hesapları → hesabı açın → **“Anahtarlar” sekmesi** → “**Anahtar ekle**” → “**Yeni anahtar oluştur**” → **JSON** → oluştur
(tarayıcı bir `xxx.json` indirir; veya `⋮` satırı → “Anahtarları yönet” → aynı akış)

**⑤ Panele geri dönün**: **proje kimliğini** girin ve JSON'nin **tüm içeriğini** "Hizmet hesabı anahtar dosyası"na yapıştırın → "EAB'yi otomatik olarak iste"ye tıklayın
→ daha sonra panel, süresi dolduğunda **EAB'yi otomatik olarak yeniden talep eder** ✓, artık GCP ziyareti yapılmaz ✓

### Yol C (yalnızca tek seferlik test): bir erişim belirteci yapıştırın

Cloud Shell'de:
```bash
gcloud auth print-access-token
```
Çıktıyı panelin "Geçici erişim belirtecine" artı proje kimliğine yapıştırın → "EAB'yi otomatik olarak iste"ye tıklayın
⚠ Yaklaşık 1 saat geçerlidir ve **kaydedilmez** ✓ → yalnızca doğrulama, otomatik yeniden istek yoktur ✗

### Bir giriş noktası bulamadığınızda kopya kağıdı (konsol menüleri değişir)

| Görev | Doğrudan bağlantı (en güvenilir) | Geri dönüş: menü |
|---|---|---|
| Genel CA API'yi etkinleştirin | `https://console.cloud.google.com/apis/library/publicca.googleapis.com?project=<PROJECT_ID>` | ☰ → API'ler ve Hizmetler → Kütüphane → `Public CA`'de arama yapın |
| Hizmet hesapları | `https://console.cloud.google.com/iam-admin/serviceaccounts?project=<PROJECT_ID>` | ☰ → IAM ve Yönetici → Hizmet hesapları |
| Rol ver | `https://console.cloud.google.com/iam-admin/iam?project=<PROJECT_ID>` | ☰ → IAM ve Yönetici → IAM → Erişim izni ver |
| Bulut Kabuğu | `https://console.cloud.google.com/cloudshell` | Sağ üstteki `>_` simgesi |

> İpucu: Doğrudan sayfaya atlamak için `<PROJECT_ID>` / `YOUR_PROJECT_ID`'yi gerçek proje kimliğiyle (`my-project-123456` gibi) değiştirin.

## 4. Paneli yapılandırma (iki yol)

**Sertifikalar → EAB yapılandırması → EAB yapılandırmasını ekle** seçeneğine gidin ve **Google Genel CA**'yı seçin:

### A Seçeneği (önerilir): JSON hizmet hesabını yapıştırın

| Alan | Değer |
|---|---|
| GCP proje kimliği | Bölüm 3'teki proje kimliği, ör. `my-project-123456` |
| Hizmet hesabı JSON | `corefusion-eab.json`'nin **tüm içeriğini** yapıştırın |
| Geçici erişim belirteci | Boş bırakın |
| İsim | `Google Public CA - <project-id>` olarak otomatik olarak oluşturulmuştur; düzenlenebilir |

**EAB'yi otomatik olarak iste** seçeneğine tıklayın → başarılı olunduğunda KID/HMAC anahtarı doldurulur ve kaydedilir; daha sonra düzenleme sırasında sadece `CA = Google Public CA`'yi seçin (EAB, CA başına ilişkilendirilir; manuel seçim yoktur).

> JSON hizmet hesabı **panel veritabanının EAB yapılandırmasına şifrelenmiştir** (`json:"-"`, **hiçbir zaman ön uca geri dönmemiştir**).
> Amaç, manuel GCP ziyaretleri yerine **7 günlük süre dolduktan sonra otomatik olarak bir anahtarın yeniden istenmesini sağlamaktır**.

### Seçenek B: geçici erişim belirteci (tek seferlik)

```bash
gcloud auth print-access-token        # valid ~1 hour
```

Bunu "Geçici erişim belirteci"ne yapıştırın, proje kimliğini doldurun ve **EAB'yi otomatik olarak iste**'ye tıklayın.
⚠ Bu jeton **saklanmaz** ve son kullanma tarihinden sonra yeniden alınması gerekir → tek seferlik bir test için para cezası verilir, uzun süreli kullanım için geçerli değildir.

### Komut satırı eşdeğeri (karşılaştırma için isteğe bağlı)

```bash
gcloud publicca external-account-keys create
# returns keyId / b64MacKey, corresponding to ACME's --eab-kid / --eab-hmac-key
```

## 4.5 Neden "tek anahtar" yerine JSON?

Bu en yaygın sorudur. İki tür “anahtarı” ayırın:

| Ne bekleyebilirsiniz? | Aslında ihtiyaç duyulan şey |
|---|---|
| **API anahtarı** (`AIza…` dizisi) | ❌ **İşe yaramaz** — API anahtarı yalnızca genel bir tanımlayıcıdır ve **hiçbir kimlik taşımaz**; EAB oluşturmak **IAM izinlerine** ihtiyaç duyduğundan kimlik doğrulama kimlik bilgileri gereklidir |
| — | ✅ **Hizmet hesabı anahtarı**: GCP'den indirilen **dosya** "IAM → Hizmet hesapları → Anahtarlar → Yeni anahtar oluştur → **JSON**" |

Önemli nokta: **JSON dosyasının kendisi bir "anahtardır"** — GCP, hesap kimliğini (`client_email`) artı özel anahtarı (`private_key`) tek bir dosyada paketler. Panel onunla bir JWT imzalar → OAuth2 erişim jetonunu değiştirir → Genel CA API'yi çağırır.
Yani “JSON'yi yapıştırın” ≠ birçok alanı dolduruyor; **dosyanın tamamını yapıştırın** (panel ayrıca `type` / `token_uri` gibi isteğe bağlı alanları da doldurur).

**Gerçek EAB, isteğin **sonucu** olan iki dizedir** (`EAB-KID` + `EAB-HMAC-Key`); panel bunları otomatik olarak doldurur.

### Dosyayı teslim etmek istemiyor musunuz? İki alternatif

| Yöntem | Nasıl | Maliyet |
|---|---|---|
| **Geçici erişim belirteci** | `gcloud auth print-access-token` → “Geçici erişim belirtecine” yapıştırın | ~1 saat geçerlilik, **kaydedilmemiş** → yalnızca bir defaya mahsus, otomatik yeniden talep yok |
| **Manuel KID/HMAC** | Yerel olarak `gcloud publicca external-account-keys create`'yi çalıştırın → `keyId` / `b64MacKey`'yi “EAB-KID / EAB-HMAC-Key” içine girin | Hiçbir kimlik bilgisi teslim edilmedi ✓; ancak Google EAB **7 günlük, tek seferliktir** → son kullanma tarihinden sonra tekrarlanır |

> Panel, JSON hizmet hesabını bir nedenden dolayı önerir: **EAB'nin süresi 7 gün sonra dolar** ve yalnızca depolanan bir hizmet hesabı, manuel çalışma olmadan **otomatik yeniden isteğe** izin verir.

## 4.6 SSS: API anahtarını kullanabilir miyim?

**Hayır.** Bu bir uygulama seçeneği değildir — **Google bunu açıkça reddeder** (her iki şekilde de doğrulanmıştır):

```bash
# 1) Query parameter
curl -X POST "https://publicca.googleapis.com/v1/projects/<ID>/locations/global/externalAccountKeys?key=AIza..." -d '{}'
# 2) x-goog-api-key header recommended by the official docs
curl -X POST "https://publicca.googleapis.com/v1/projects/<ID>/locations/global/externalAccountKeys" \
     -H "x-goog-api-key: AIza..." -d '{}'
```

İkisi de geri dönüyor:

```json
{"error":{"code":401,"status":"UNAUTHENTICATED",
  "message":"API keys are not supported by this API. Expected OAuth2 access token
             or other authentication credentials that assert a principal."}}
```

Karşılaştırma için, **(sahte) OAuth Taşıyıcısı** şunu döndürür:
`Request had invalid authentication credentials. Expected OAuth 2 access token...` —
böylece OAuth tanınırken **API anahtarları doğrudan reddedilir** ✗.

**Neden**: EAB'yi oluşturmak, "kim olduğunuzu ve `roles/publicca.externalAccountKeyCreator`'ye sahip olup olmadığınızı" bilmenizi gerektiren **idari bir işlemdir** ✗
— API anahtarı yalnızca bir projeyi/uygulamayı tanımlar ✓ ve **bir ana kimliği ifade etmez** ✓.
(Google'nin "API anahtarlarını yönetmek için en iyi uygulamalar", yönetim senaryoları için **IAM + kısa ömürlü hizmet hesabı kimlik bilgilerini** önerir ✓ — burada kullanılan yaklaşımın aynısı ✓.)

**Yani yalnızca üç yol var** ✓ (bkz. §3): hizmet hesabı JSON ✓ / geçici erişim belirteci ✓ /
veya **hiç kimlik bilgisi yok** — Cloud Shell'de `gcloud publicca external-account-keys create`'yi çalıştırın ✓
ve panele `keyId` / `b64MacKey` girin ✓ (**GCP IAM sizi engelliyorsa bunu kullanın** ✓).

## 5. Otomatik kayıt ve yenilemeler (önemli)

```
Panel: CA = Google Public CA
  ├─ usable EAB present in the config → use it
  └─ missing/expired → request a new one on the spot with the stored service-account JSON (auto-registration)
        │
First issuance on the node/panel ── registers the ACME account with EAB (one-time!)
        │
Account persisted: data/acme/accounts/google.json (panel) / <Agent data_dir>/acme/accounts/google.json (node)
        │
Every later renewal ── uses the account key only; **EAB is never needed again**
```

Bu nedenle:

- **İlk düzenleme otomatik olarak kaydedilir**; manuel GCP adımına gerek yoktur;
- Yenilemeler **EAB'yi tüketmez** (hesap dosyası mevcut olduğu ve veri dizini sabit kaldığı sürece);
- ⚠ Aracı tarafı ACME veri dizini sabit bir mutlak yoldur (varsayılan `/var/lib/corefusion-agent/acme`);
 **rastgele değiştirmeyin**, aksi takdirde yeni bir hesap olarak değerlendirilecektir → başka bir EAB anahtarı gerekli olacaktır.

## 6. Sorun giderme

| Belirti / hata | Sebep | Düzeltme |
|---|---|---|
| `PERMISSION_DENIED` (403) | Hizmet hesabında `roles/publicca.externalAccountKeyCreator` yok | 3. bölüme geri dönün ④ |
| `SERVICE_DISABLED` (403) | `publicca.googleapis.com` proje için etkinleştirilmedi | Bölüm 3 ② |
| `NOT_FOUND` (404) | Yanlış proje kimliği / proje silindi | Proje kimliğini doğrulayın |
| Düzenleme raporları `externalAccountRequired` | Bu CA'da EAB yok | Önce panelde EAB'yi isteyin/yapılandırın |
| Düzenleme raporları `Accounts must have at least one contact.` | Kayıt sırasında e-posta sağlanmadı | Sertifika sayfasında ACME hesap e-postasını ayarlayın (panel varsayılan hesabı kullanır) |
| Yenilemede tekrar EAB isteniyor / kayıt başarısız oluyor | ACME hesap dosyası kayboldu (veri dizini değiştirildi / temizlendi) | **Yedeklemeyi geri yükleyin**; EAB yoksa hizmet hesabını kullanarak yeni bir EAB talep edin |
| Veren kuruluş Google'den Let's Encrypt'e geçti | Yenileme komutunda CA eksik (eski hata, v0.12.101'de düzeltildi) | Yükseltme ve yeniden yayınlama |

## 7. Güvenlik ve operasyon önerileri

1. **En az ayrıcalık**: Hizmet hesabına yalnızca `roles/publicca.externalAccountKeyCreator` verin, Sahip/Düzenleyici vermeyin.
2. **Projeyi silmeyin**: GCP projesinin silinmesi tüm ACME hesaplarını geçersiz kılar → sertifikalar yenilenemez.
3. **Her zaman yedekleyin** `data/acme/accounts/`'yi (hesap kimliğinin kendisini) ve panel veritabanının EAB yapılandırmasını.
4. **EAB'nin süresinin dolması konusunda panik yapmayın**: ACME hesap dosyası mevcut olduğu sürece yenilemeler etkilenmez; yeni bir EAB yalnızca yeni bir hesap kaydetmek için gereklidir.
5. Hizmet hesabı anahtarı sızdırılmış olabilirse: GCP'deki bu anahtarı silin ve panele yeni bir JSON yapıştırın.

## 8. İlgili uç noktalar (komut dosyası oluşturma için)

| Uç nokta | Açıklama |
|---|---|
| `POST /api/v1/eab-configs/fetch-google` | GCP yetkilendirmesini kullanarak bir EAB oluşturun. Gövde: `{project_id, service_account_json?, access_token?}` |
| `POST /api/v1/eab-configs/fetch-zerossl` | ZeroSSL EAB'yi e-postayla edinin. Gövde: `{email}` |
| `GET  /api/v1/eab-configs` | EAB yapılandırmalarını listeleyin (**HMAC anahtarı veya JSON hizmet hesabı olmadan**; yalnızca `has_mac_key` / `has_google_sa`) |
