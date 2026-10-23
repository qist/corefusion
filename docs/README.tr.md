<!-- source: README.md | locale: tr | updated: 2026-10-23 -->
# CoreFusion Belgeleri

> Diller: [简体中文](README.md) · [English](README.en.md) · [繁體中文](README.zh-TW.md) · [Русский](README.ru.md) · [日本語](README.ja.md) · [한국어](README.ko.md) · [Deutsch](README.de.md) · [Français](README.fr.md) · [Español](README.es.md) · [Português](README.pt.md) · Türkçe · [Bahasa Indonesia](README.id.md) · [ไทย](README.th.md) · [Tiếng Việt](README.vi.md) · [العربية](README.ar.md) · [فارسی](README.fa.md)

## [USER-GUIDE.tr.md](./USER-GUIDE.tr.md) — Kullanıcı kılavuzu

**Kullanıcılar** (işlemler / yöneticiler / operatörler) için eksiksiz bir işlem kılavuzu: kurulum, ilk yapılandırma, özellik modülleri, SSS ve ek.

## [api/](./api/) — Genel API referansı

Entegratörler, ikincil geliştirme ve üçüncü taraf müşteriler için:

- [api/README.tr.md](./api/README.tr.md) — genel bakış: temel URL, kimlik doğrulama, yanıt biçimi, hız sınırları
- [api/subscription.tr.md](./api/subscription.tr.md) — abonelik uç noktası (`/sub/:token`), formatlar, yanıt başlıkları, örnekler
- [api/portal-api.tr.md](./api/portal-api.tr.md) — müşteri portalı API (kaydolma / oturum açma, planlar, siparişler, bakiye, biletler)
- [api/charge-callback.tr.md](./api/charge-callback.tr.md) — ödeme geri arama imzası ve doğrulaması
- [api/admin-api.tr.md](./api/admin-api.tr.md) — admin API modüle göre gruplandırılmış rota tablosu

## Tasarım belgeleri

Tasarım belgeleri dahili materyallerdir (temel fikri mülkiyet) ve bu depoyla **dağıtılmaz**. İhtiyacınız olursa bakımcıyla iletişime geçin.

> **Kapsam**: İstemci uygulaması (yerel kullanıcı arayüzü, TUN'un devralınması, tepsi, çoklu abonelik birleştirme) ayrı bir üründür ve bu havuzun kapsamı dışındadır. Buradaki belgeler yalnızca sunucu tarafını kapsar (panel/düğümler/abonelik teslimi).

## Diğer işlem kılavuzları

- `cert-google-eab.md` — Google EAB sertifika verilmesi *(Çince)*
- `oauth-google-verification.md` — Google OAuth doğrulaması *(Çince)*
- `notification-templates.md` — bildirim şablonu değişkenleri *(Çince)*

## Çeviri kapsamı

| Belge | zh-CN | en | zh-TW | Diğer yerel ayarlar |
|----------|-------|----|-------|---------------|
| README.md | ✅ | ✅ | ✅ | ru ✅ |
| USER-GUIDE.md | ✅ | ✅ | ✅ | ru ✅ |
| I18N.md | ✅ | — | — | — |
| api/README.md | ✅ | ✅ | ✅ | ru ✅ |
| api/subscription.md | ✅ | ✅ | ✅ | ru ✅ |
| api/portal-api.md | ✅ | ✅ | ✅ | ru ✅ |
| api/charge-callback.md | ✅ | ✅ | ✅ | ru ✅ |
| api/admin-api.md | ✅ | ✅ | ✅ | ru ✅ |
| cert-google-eab.md | ✅ | ✅ | ✅ | ru ✅ |
| oauth-google-verification.md | ✅ | ✅ | ✅ | ru ✅ |
| notification-templates.md | ✅ | ✅ | ✅ | ru ✅ |
| LEGAL.md | ✅ | ✅ | ✅ | ru ✅ |
| THIRD_PARTY_LICENSES.md | ✅ | ✅ | ✅ | ru ✅ |

Adlandırma, dilde gezinme ve inceleme kuralları için [I18N.md](README.tr.md)'ye bakın *(Çince)*.
