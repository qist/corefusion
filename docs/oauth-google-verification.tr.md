<!-- source: oauth-google-verification.md | locale: tr | updated: 2026-10-23 -->
# Google OAuth Doğrulama Kılavuzu

> Diller: [简体中文](oauth-google-verification.md) · [English](oauth-google-verification.en.md) · [繁體中文](oauth-google-verification.zh-TW.md) · [Русский](oauth-google-verification.ru.md) · [日本語](oauth-google-verification.ja.md) · [한국어](oauth-google-verification.ko.md) · [Deutsch](oauth-google-verification.de.md) · [Français](oauth-google-verification.fr.md) · [Español](oauth-google-verification.es.md) · [Português](oauth-google-verification.pt.md) · Türkçe · [Bahasa Indonesia](oauth-google-verification.id.md) · [ไทย](oauth-google-verification.th.md) · [Tiếng Việt](oauth-google-verification.vi.md) · [العربية](oauth-google-verification.ar.md) · [فارسی](oauth-google-verification.fa.md)

> Kapsam: CoreFusion portalı için Google oturum açma (`internal/service/oauth`, kapsamlar: `openid`, `email`, `profile`).
> Sonunda GitHub karşılaştırması (GitHub'de doğrulama akışı yok).

## 1. Test ve üretim modu

Google OAuth izin ekranının iki yayınlama durumu vardır:

| Eyalet | Kimler oturum açabilir? | Doğrulama gerekli |
|---|---|---|
| **Test etme** | Yalnızca "test kullanıcıları" listesindeki Google hesapları | Hayır |
| **Üretimde** | Herhangi bir Google hesabı | **Evet** (aşağıya bakın) |

"Sistem ayarları → OAuth" etkinleştirildiğinde portal oturum açma sayfası her ziyaretçiye Google düğmesini gösterir;
uygulama hâlâ test aşamasındaysa test kullanıcısı listesinde olmayan ziyaretçiler şunları görür:
`Error 403: access_denied` (uygulama doğrulanmadı / test kullanıcısı değilsiniz).
Google'de herkese açık oturum açma olanağı sunmak için **uygulamayı üretim moduna geçirin ve doğrulamayı geçin**.

## 2. Doğrulama için zor gereksinimler

Google'nin incelemesi (kısıtlanmamış kapsamlar için temel doğrulama) şunları gerektirir:

1. **Ana sayfa**: Yetkili etki alanında (aynı etki alanı veya alt etki alanı) herkesin erişebildiği URL.
2. **Gizlilik Politikası bağlantısı**: **herkese açık** olmalıdır (oturum açmaya gerek yoktur) ve hangi verileri topladığınız ve bunları nasıl kullandığınız/depoladığınız/sildiğiniz açıklanmalıdır. Google oturum açma işlemi en azından aşağıdakileri kapsamalıdır:
 Toplanan Google hesap verileri (e-posta, isim, avatar), amaç (portal hesabı oluşturma/oturum açma), üçüncü taraflara satış yapılmaması,
 ve kullanıcıların hesaplarını ve verilerini nasıl silebilecekleri.
3. **Hizmet Şartları bağlantısı**: herkesin erişebileceği, hizmet kurallarını açıklayan bağlantı.
4. **Yetkili alanlar**: Uygulamanın alan adını izin ekranına kaydedin (ör. `example.com`); ana sayfa/gizlilik/koşullar bağlantıları bu alanda veya bir alt alan adında olmalıdır.
5. **Uygulama logosu (isteğe bağlı ancak önerilir)**: 120×120 piksel.
6. **Kapsam gerekçesi**: bu proje yalnızca `openid / email / profile` (kısıtlamasız, hassas olmayan) kullanıyor,
 temel doğrulama kapsamındadır; kapsam bazında gerekçe yoktur ve **üçüncü taraf güvenlik değerlendirmesi yoktur**.
 (Tam Gmail erişimi gibi kısıtlı kapsamlar bunları gerektirir; bu projede herhangi bir kapsam kullanılmamaktadır.)

Doğrulama girişi: [Google Cloud Console](https://console.cloud.google.com/) →
**API'ler ve Hizmetler → OAuth izin ekranı → Uygulamayı Yayınla → Doğrulama için gönder**.
Temel doğrulama genellikle birkaç gün içinde tamamlanır; bu arada test modunda çalışabilirsiniz.

## 3. Bu proje için neyin yapılandırılacağı

### 3.1 Yetkili yönlendirme URI

Geri arama, oturum açmayı başlatan etki alanından dinamik olarak oluşturulur (bkz. `internal/api/handlers/oauth/handler.go`'deki `Authorize`):

```
https://<portal-domain>/api/v1/client-auth/oauth/google/callback
```

- Oturum açma portal etki alanından başlatıldı (ör. `https://p.example.com`) → `https://p.example.com/...`'yi kaydedin;
- `/p` panel etki alanından başlatıldı → `https://panel.example.com/...`'yi kaydedin.
- Google **birden fazla** yönlendirme URI'sine izin verir; her iki alanı da kaydedin.
- ⚠ Google, yönlendirme URI'lerinin **https** (`http://localhost` hariç) olmasını gerektirir;
 ters proxy `X-Forwarded-Host` / `X-Forwarded-Proto`'yi iletmelidir, aksi takdirde panel bir
 http veya dahili yönlendirme_uri → Google bunu reddeder (`redirect_uri_mismatch`).

"Sistem ayarları → OAuth → Portal geri arama alanı" ayarını yaptıktan sonra sayfada gösterilen geri arama URL, Google'ye yapıştırılacak olandır.

### 3.2 Portaldaki Gizlilik Politikası / Şartlar bağlantıları

Oturum açma kartında "Gizlilik Politikası · Hizmet Şartları" bağlantıları bulunur (yapılandırıldıktan sonra gösterilir) ve her iki belgede de **bağımsız sayfalar** bulunur
(iletişim kutusu ve bağımsız sayfa aynı içeriği paylaşır):

- Ana panel alanı: `https://<panel-domain>/p/legal/privacy`, `/p/legal/terms`
- Özel portal etki alanı (kök dağıtım): `https://<portal-domain>/legal/privacy`, `/legal/terms`
- `?lang=xx`'yi destekler; bu olmadan, tarayıcının `Accept-Language`'si kullanılır, ardından portal varsayılanı kullanılır.
- Sayfalar **sunucu tarafından oluşturulmuştur** (JS yoktur), bu da Google'nin doğrulama tarayıcısına yardımcı olur.

**Yapılandırma (her iki durumda da; bağlantılar kazanır)**:

1. **İçerik modu (önerilir, kutudan çıktığı gibi çalışır)**: portal düzenleyici → Ayarlar → Genel → Uyumluluk belgeleri →
 “Dili düzenle”yi seçin ve §4 şablonlarını “Gizlilik politikası içeriği” / “Hizmet şartları içeriği”ne yapıştırın
 (**İşaretleme, dile göre düzenlenmiştir**) → kaydettikten sonra, oturum açma kartı bir **iletişim kutusu** açan bağlantıları gösterir;
 editörün "bağımsız bağlantısına" doğrudan Google izin ekranına girilebilir.
2. **Harici bağlantı modu**: “Gizlilik politikası URL” / “Hizmet şartları URL”yi girin (barındırdığınız sayfaları işaret ederek) → yeni bir sekmede açılır.

İçerik ve bağlantılar `/api/v1/public/portal-config` (`privacy_policy_content` /
`terms_of_service_content` **dil başına Markdown haritalarıdır** `{"zh-CN":"...","en":"..."}`;
`privacy_policy_url` / `terms_of_service_url` isteğe bağlı harici bağlantılardır). **Oturum açmadan erişilebilir olmalıdır** ✓.

### 3.3 Panel tarafındaki anahtar

`System settings → OAuth`: Google'yi etkinleştirin ve İstemci Kimliğini/İstemci Sırrını girin (gizli bilgi, bekleme sırasında AES-256-GCM ile şifrelenir).
Google düğmesi portal oturum açma sayfasında yalnızca her şey yapılandırıldığında görünür (`/api/v1/client-auth/oauth/providers` etkin olarak döner).

## 4. Gizlilik Politikası / Şartlar şablonları

> Uyarlayabileceğiniz minimum uyumluluk şablonları. Kullanmadan önce: `【】` yer tutucularını değiştirin, işletmeniz için maddeler ekleyin/kaldırın,
> ve işletme biriminin bunları incelemesini sağlayın. Bu şablonlar hukuki tavsiye niteliğinde değildir.

### 4.1 Gizlilik Politikası (şablon)

```markdown
# Privacy Policy

Last updated: [DATE]

[OPERATOR NAME] ("we") operates [PORTAL NAME] ("the Service"). We respect your privacy;
this policy explains what information we collect and how we use it.

## 1. Information we collect
- **Account information**: the email address and password you provide at registration (stored only as a one-way hash);
- **Google sign-in information**: when you choose "Sign in with Google", we receive, with your authorization,
  your Google account email, display name and avatar URL (scopes: openid, email, profile);
- **Service data**: subscription configuration, traffic usage, device IP allow-list, tickets and order records.

## 2. How we use information
Only to: create and sign in to your account, provide the proxy subscription service, bill you and provide support,
and for security controls (sign-in protection, IP allow-list). We **do not** sell your personal information.

## 3. Third-party services
- **Google**: receives account information within the authorized scope only when you actively use "Sign in with Google";
- Payment providers: receive order information only when you initiate a payment.

## 4. Storage and security
Data is stored on our servers; credentials are encrypted at rest and all traffic uses TLS.

## 5. Your rights
You can update your profile/password and end sessions in the portal at any time; to delete your account and all data,
contact [CONTACT EMAIL] and we will process the request within [N] business days.

## 6. Policy changes
Material changes will be announced in the portal notice board.
```

### 4.2 Hizmet Şartları (şablon)

```markdown
# Terms of Service

Last updated: [DATE]

## 1. Service
The Service provides [PROXY / NETWORK ACCESS] to registered users; specific plans are described on the store page.

## 2. Accounts
- You are responsible for the security of your account and password; for accounts created with Google sign-in, credentials are managed by Google;
- You may not lend or transfer your account, or use the Service for any unlawful purpose (including but not limited to:
  unauthorized access to others' systems, distributing illegal content, or circumventing mandatory legal requirements).

## 3. Payment and refunds
Plans are billed [monthly / by usage]; traffic and device limits follow the plan description;
[REFUND POLICY: e.g. full refund within 7 days if unused].

## 4. Changes and termination
We may suspend or terminate the Service due to force majeure or your breach of these terms; deleting an account deletes the related data.

## 5. Disclaimer
The Service is provided "as is"; to the extent permitted by law, we are not liable for indirect losses caused by use of the Service.

## 6. Miscellaneous
These terms are governed by the laws of [JURISDICTION]. Contact: [CONTACT EMAIL].
```

## 5. Lansman öncesi kontrol listesi

- [ ] OAuth izin ekranı: uygulama adı / logo / ana sayfa / gizlilik politikası / şartların tamamı mevcut; alan adı kayıtlı
- [ ] Gizlilik Politikası ve Şartlar sayfaları **oturum açmadan erişilebilir** (portal özel sayfası veya bağımsız statik sayfa)
- [ ] Her iki bağlantı da portal oturum açma kartında/altbilgisinde görünür
- [ ] yönlendirme URI gerçek etki alanıyla eşleşir ve https'dir
- [ ] Uygulamayı Yayınla → doğrulama için gönder; onaylandıktan sonra test kullanıcısı kısıtlaması kaldırılır
- [ ] Panel OAuth ayarları: etkin=doğru, İstemci Kimliği/Gizli doldurulmuş halde (sağlayıcıların uç noktası etkin:doğru değerini döndürür)

## 6. GitHub karşılaştırması

GitHub OAuth Uygulamalarının **hiçbir** yayınlama durumu veya doğrulama akışı yoktur: uygulamayı oluşturun, URL geri çağrısını ayarlayın ve uygulama herkese açık olarak çalışır.
Bir GitHub OAuth Uygulamasının yalnızca **bir** Yetkilendirme geri çağrısını URL (ön ek eşleştirme) kabul ettiğini unutmayın; birden fazla alan adı için,
Google'deki multi-URI yaklaşımını kullanın veya portal alanı başına ayrı bir uygulama oluşturun.
