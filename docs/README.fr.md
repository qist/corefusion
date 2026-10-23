<!-- source: README.md | locale: fr | updated: 2026-10-23 -->
# Documentation CoreFusion

> Langues : [简体中文](README.md) · [English](README.en.md) · [繁體中文](README.zh-TW.md) · [Русский](README.ru.md) · [日本語](README.ja.md) · [한국어](README.ko.md) · [Deutsch](README.de.md) · Français · [Español](README.es.md) · [Português](README.pt.md) · [Türkçe](README.tr.md) · [Bahasa Indonesia](README.id.md) · [ไทย](README.th.md) · [Tiếng Việt](README.vi.md) · [العربية](README.ar.md) · [فارسی](README.fa.md)

## [USER-GUIDE.fr.md](./USER-GUIDE.fr.md) — Guide de l'utilisateur

Un guide d'exploitation complet pour les **utilisateurs** (opérations / administrateurs / opérateurs) : installation, configuration initiale, modules de fonctionnalités, FAQ et annexe.

## [api/](./api/) — Référence publique API

Pour les intégrateurs, les développeurs secondaires et les clients tiers :

- [api/README.fr.md](./api/README.fr.md) — présentation : base URL, authentification, format de réponse, limites de débit
- [api/subscription.fr.md](./api/subscription.fr.md) — point de terminaison d'abonnement (`/sub/:token`), formats, en-têtes de réponse, exemples
- [api/portal-api.fr.md](./api/portal-api.fr.md) — portail client API (inscription/connexion, plans, commandes, solde, billets)
- [api/charge-callback.fr.md](./api/charge-callback.fr.md) — signature et vérification du rappel de paiement
- [api/admin-api.fr.md](./api/admin-api.fr.md) — Table de routage admin API regroupée par module

## Documents de conception

Les documents de conception sont des documents internes (propriété intellectuelle de base) et ne sont **pas distribués** avec ce référentiel. Contactez le responsable si vous en avez besoin.

> **Portée** : l'application client (interface utilisateur locale, prise de contrôle TUN, plateau, fusion multi-abonnements) est un produit distinct et n'est pas couvert par ce référentiel. La documentation ici couvre uniquement le côté serveur (panneau / nœuds / livraison d'abonnement).

## Autres guides d'opérations

- `cert-google-eab.md` — Délivrance du certificat Google EAB *(chinois)*
- `oauth-google-verification.md` — Google Vérification OAuth *(Chinois)*
- `notification-templates.md` — variables du modèle de notification *(chinois)*

## Couverture de traduction

| Documenter | zh-CN | en | zh-TW | Autres lieux |
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

Voir [I18N.md](README.fr.md) pour connaître les règles de dénomination, de navigation linguistique et de révision *(chinois)*.
