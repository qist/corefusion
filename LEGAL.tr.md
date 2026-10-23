<!-- source: LEGAL.md | locale: tr | updated: 2026-10-23 -->
# Yasal ve Uyumluluk Notları

> Bu çeviri yalnızca kolaylık sağlamak içindir. Tutarsızlık olması durumunda Çince asıl metin geçerlidir.

> Diller: [简体中文](LEGAL.md) · [English](LEGAL.en.md) · [繁體中文](LEGAL.zh-TW.md) · [Русский](LEGAL.ru.md) · [日本語](LEGAL.ja.md) · [한국어](LEGAL.ko.md) · [Deutsch](LEGAL.de.md) · [Français](LEGAL.fr.md) · [Español](LEGAL.es.md) · [Português](LEGAL.pt.md) · Türkçe · [Bahasa Indonesia](LEGAL.id.md) · [ไทย](LEGAL.th.md) · [Tiếng Việt](LEGAL.vi.md) · [العربية](LEGAL.ar.md) · [فارسی](LEGAL.fa.md)

> Bu belge CoreFusion'nin lisansını, ticari kullanım politikasını, üçüncü taraf yükümlülüklerini ve kullanıcıların kendi yetki alanlarında dikkat etmesi gereken uyumluluk noktalarını açıklamaktadır.
>
> **Bu belge yasal tavsiye niteliğinde değildir.** Uyumluluk dağıtım bölgenize, müşterilerinize ve işletim modelinize bağlıdır; Nitelikli bir yerel avukata danışın.
>
> *Bu çeviri kolaylık sağlamak amacıyla sağlanmıştır. Çince orijinalinden farklı olması durumunda Çince versiyonu geçerli olacaktır.*

## 1. Proje lisansı

CoreFusion, **GNU Affero Genel Kamu Lisansı v3.0 (AGPL-3.0)** kapsamında yayımlanmıştır; tam metin için bkz. [LICENSE](./LICENSE).

```
Copyright (C) 2026 qist

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published
by the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
```

## 2. Ticari kullanım ve ticari barındırma: açıkça izin verilir

**Herhangi bir kişi veya kuruluş CoreFusion'yi aşağıdakiler dahil ancak bunlarla sınırlı olmamak üzere ticari amaçlarla kullanabilir:**

- Kendi kendine barındırma ve dahili dağıtım;
- Üçüncü taraflar için ücretli bir proxy/düğüm hizmeti çalıştırmak;
- Ücret karşılığında üçüncü taraflara SaaS, barındırma, yönetilen operasyonlar veya sistem entegrasyonu sağlamak;
- İkincil geliştirme, özelleştirme ve dağıtım (donanımla birlikte veya bir çözümün parçası olarak teslimat dahil).

**Yukarıdaki ticari kullanım ve ticari barındırma ayrı bir yetkilendirme, lisans ücreti veya gelir paylaşımı gerektirmez.**

Tek yükümlülük AGPL-3.0'ın kendisidir, esas olarak iki tanesi:

1. **İkili dosyaları dağıtırken** (§4–§6): alıcılara İlgili Kaynağı (ilgili kaynağın tamamını) veya §6 uyarınca etkili bir yazılı teklif sağlayın; telif hakkı bildirimlerini ve lisans metnini saklayın.
2. **Ağ üzerinden değiştirilmiş bir sürüm sağlarken** (§13): Değiştirilmiş bir sürüm yayınlarsanız ve üçüncü tarafların ağ üzerinden onunla etkileşime girmesine izin verirseniz, **bu kullanıcılara değiştirilmiş sürümün İlgili Kaynağını ücretsiz olarak sunmalısınız** (örneğin, panelde belirgin bir kaynak indirme/depo bağlantısı).

Ek notlar:

- **Değiştirilmemiş** bir kopyanın dağıtılması ve kullanıcılara sunulması tek başına §13 kaynak yükümlülüğünü tetiklemez; ikili dosyaları dağıtmak hala 1. maddenin kapsamına giriyor.
- Para istemek yasak değildir; AGPL, iş modelini değil, **kaynak kullanılabilirliğini** yönetir. Sırf ticari olarak veya kapalı kaynakla çalıştığınız için §13'ün değiştirilmiş bir versiyonunu reddedemezsiniz.
- **AGPL'ye tabi olmayan kapalı kaynaklı bir ticari lisans** istiyorsanız telif hakkı sahibiyle ayrıca iletişime geçin. Bu projenin üçüncü taraf GPL-3.0 bileşenlerini birbirine bağladığını unutmayın (bkz. bölüm 3); herhangi bir kapalı kaynak sürümünün öncelikle bunları kaldırması gerekir.

## 3. Üçüncü taraf bileşenler ve yeniden dağıtım

Ana üçüncü taraf bileşenleri ve lisansları [THIRD_PARTY_LICENSES.md](LEGAL.tr.md)'de listelenmiştir.

Özel notlar:

- Bu proje, `github.com/xtls/xray-core` aracılığıyla dolaylı olarak **GPL-3.0-veya üstü** `github.com/sagernet/sing` ve `sing-shadowsocks`'yi birbirine bağlar. Bu nedenle **panel veya Aracı ikili dosyalarının dağıtımı aynı zamanda GPL-3.0'ın karşılık gelen kaynak yükümlülüklerinin de yerine getirilmesini gerektirir**.
- Çalışma zamanında panel, Xray-core, sing-box ve Caddy gibi bağımsız programları indirebilir veya çağırabilir. Bunlar **bu depoda dağıtılmaz**, ancak bunları teslimatlarınızla birlikte gönderirseniz lisanslarına uymanız gerekir (özellikle sing-box'nin GPL-3.0'ı: ilgili kaynağı sağlayın). Hysteria2 ve TUIC, bağımsız ikili dosyalar değil, sing-box tarafından taşınan protokollerdir.
- Paketlenmiş GeoIP/GeoSite verileri (GeoLite2, `geoip.dat`, `geosite.dat`) kendi veri lisanslarına tabidir; Ticari kullanımdan önce şartları doğrulayın.

## 4. Bölgesel uyumluluk ve kullanıcı sorumluluğu

CoreFusion **genel amaçlı bir yazılımdır**: Herhangi bir proxy veya ağ hizmeti sağlamaz, düğüm, bant genişliği veya kimlik bilgisi göndermez ve hiçbir ücret toplamaz. Operasyonlar konuşlandırıcının sorumluluğundadır.

Dağıtıcılar/operatörler, aşağıdakiler dahil ancak bunlarla sınırlı olmamak üzere, işletmelerinin kendi yetki alanlarında lisans, dosyalama veya güvenlik değerlendirmesi gerektirip gerektirmediğini belirlemelidir:

- **Çin Anakarası**: katma değerli telekomünikasyon işletme lisansları (ICP/IDC/ISP, vb.), ICP dosyalama, MLPS, sınır ötesi veri aktarımı güvenliği değerlendirmesi ve Siber Güvenlik Yasası, Veri Güvenliği Yasası ve Kişisel Bilgilerin Korunması Yasası kapsamındaki yükümlülükler. Bu yazılımı yerel yasaları ihlal eden amaçlarla kullanmayın.
- **AB/İngiltere**: GDPR / Birleşik Krallık GDPR, eGizlilik, Dijital Hizmetler Yasası (DSA) işlemci/denetleyici yükümlülükleri, günlük kaydı ve çerez uyumluluğu.
- **Amerika Birleşik Devletleri**: eyalet gizlilik yasaları, şifreleme ihracat kontrolleri (EAR) ve yaptırım taraması.
- **Diğer bölgeler**: yerel telekom, şifreli iletişim, içerik yönetimi, veri yerelleştirmesi ve tüketiciyi koruma kuralları.

Kullanıcılar bu yazılımı aşağıdaki amaçlarla kullanmamayı kabul ederler:

- Yürürlükteki kanunları veya ihracat kontrol/yaptırım kurallarını ihlal eden faaliyetler;
- Fikri mülkiyeti, gizliliği veya diğer yasal hakları ihlal etmek;
- Yasadışı içerik dağıtmak, ağ saldırıları, dolandırıcılık veya diğer suç faaliyetleri.

**Bu yazılımın kullanımından veya yanlış kullanımından doğacak her türlü hukuki sorumluluk kullanıcıya aittir.**

## 5. Veri koruma ve gizlilik

Panel, hesapları, iletişim ayrıntılarını, siparişleri, ödeme geri aramalarını, abonelikleri, trafik istatistiklerini ve oturum açma/işlem günlüklerini saklayabilir. İşlemci/kontrolör olarak operatörlerin şunları yapması gerekir:

- Kullanıcılara bir gizlilik politikası sağlamak ve gerekli onayları almak;
- Veri minimizasyonunu takip edin ve günlükler ve istatistikler için makul saklama süreleri belirleyin (proje, operatörlerin yapılandırması gereken günlük temizleme politikaları sağlar);
- Kişisel bilgilere erişim, düzeltme ve silme taleplerini desteklemek;
- Veritabanlarına, yedeklemelere, sertifika özel anahtarlarına ve ödeme kimlik bilgilerine erişim kontrolü ve şifreleme uygulayın;
- Sınır ötesi dağıtım sırasında sınır ötesi veri aktarımı değerlendirmelerini tamamlayın.

## 6. Sorumluluk reddi beyanı

Bu yazılım, AGPL-3.0 §§15–17 kapsamında, ticarete elverişlilik, belirli bir amaca uygunluk ve ihlal etmeme dahil olmak üzere herhangi bir açık veya zımni garanti olmaksızın "olduğu gibi" sağlanmaktadır. Telif hakkı sahibi, bu yazılımın kullanımından kaynaklanan doğrudan, dolaylı, tesadüfi veya sonuç olarak ortaya çıkan zararlardan sorumlu değildir.

## 7. İletişim

- Proje: <https://github.com/qist/corefusion>
- Ticari lisanslama/uyumluluk soruları: Proje Sorunlarını veya veri havuzu sayfasında yayınlanan iletişim bilgilerini kullanın.
