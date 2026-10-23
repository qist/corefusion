<!-- source: README.md | locale: en | updated: 2026-10-23 -->
# CoreFusion Documentation

> Languages: [简体中文](README.md) · English · [繁體中文](README.zh-TW.md) · [Русский](README.ru.md) · [日本語](README.ja.md) · [한국어](README.ko.md) · [Deutsch](README.de.md) · [Français](README.fr.md) · [Español](README.es.md) · [Português](README.pt.md) · [Türkçe](README.tr.md) · [Bahasa Indonesia](README.id.md) · [ไทย](README.th.md) · [Tiếng Việt](README.vi.md) · [العربية](README.ar.md) · [فارسی](README.fa.md)

## [USER-GUIDE.en.md](./USER-GUIDE.en.md) — User guide

A complete operations guide for **users** (operations / administrators / operators): installation, initial configuration, feature modules, FAQ and appendix.

## [api/](./api/) — Public API reference

For integrators, secondary development and third-party clients:

- [api/README.en.md](./api/README.en.md) — overview: base URL, authentication, response format, rate limits
- [api/subscription.en.md](./api/subscription.en.md) — subscription endpoint (`/sub/:token`), formats, response headers, examples
- [api/portal-api.en.md](./api/portal-api.en.md) — client portal API (sign-up / sign-in, plans, orders, balance, tickets)
- [api/charge-callback.en.md](./api/charge-callback.en.md) — payment callback signature and verification
- [api/admin-api.en.md](./api/admin-api.en.md) — admin API route table grouped by module

## Design documents

Design documents are internal materials (core intellectual property) and are **not distributed** with this repository. Contact the maintainer if you need them.

> **Scope**: the client app (local UI, TUN takeover, tray, multi-subscription merging) is a separate product and is out of scope for this repository. The documentation here covers the server side only (panel / nodes / subscription delivery).

## Other operations guides

- `cert-google-eab.md` — Google EAB certificate issuance *(Chinese)*
- `oauth-google-verification.md` — Google OAuth verification *(Chinese)*
- `notification-templates.md` — notification template variables *(Chinese)*

## Translation coverage

| Document | zh-CN | en | zh-TW | Other locales |
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

See [I18N.md](./I18N.md) for naming, language navigation and review rules *(Chinese)*.
