<!-- source: README.md | locale: fr | updated: 2026-10-23 -->
# Documentation CoreFusion API

> Langues : [简体中文](README.md) · [English](README.en.md) · [繁體中文](README.zh-TW.md) · [Русский](README.ru.md) · [日本語](README.ja.md) · [한국어](README.ko.md) · [Deutsch](README.de.md) · Français · [Español](README.es.md) · [Português](README.pt.md) · [Türkçe](README.tr.md) · [Bahasa Indonesia](README.id.md) · [ไทย](README.th.md) · [Tiếng Việt](README.vi.md) · [العربية](README.ar.md) · [فارسی](README.fa.md)

Documentation API pour **intégrateurs externes, développement secondaire et clients tiers**. Tous les chemins sont relatifs au domaine de déploiement.

| Documenter | Public | Contenu |
|---|---|---|
| [abonnement.md](./subscription.fr.md) | Abonnés / développeurs clients | Point de terminaison d'abonnement (`/sub/:token`) : formats, en-têtes, codes d'état, exemples |
| [portal-api.md](README.fr.md) | Frontend du portail / clients tiers | Portail client API (`/api/v1/client-auth/*`, `/api/v1/public/*`) : inscription, connexion, plans, commandes, solde, billets |
| [charge-callback.md](./charge-callback.fr.md) | Fournisseurs de paiement | Signature et vérification du rappel de paiement |
| [admin-api.fr.md](./admin-api.fr.md) | Développement / automatisation de panneaux | Table de routage d'administration complète API (`/api/v1/*`, regroupée par module) |

---

## 1. Ports et URL de base

Le panneau peut exposer trois moteurs en même temps, partageant une seule base de données :

| Moteur | Accès par défaut | Objectif |
|---|---|---|
| Panneau principal | `https://<domain>/api/v1` | Backend d'administration + portail + abonnements |
| Moteur d'abonnement (port dédié) | `https://<domain>:<port>/:token` | Récupération d'abonnement à haute concurrence, pas de préfixe `/sub` |
| Moteur de portail (port dédié) | `https://<portal-domain>[:port]/api/v1` | Portail réservé aux clients, pages sur `/` |

> Le proxy inverse doit transmettre à la fois les chemins de page et `/api/v1` ; `X-Forwarded-Proto` / `X-Forwarded-Host` sont honorés de restaurer le schéma et le domaine réels.

## 2. Authentification (trois types, non interchangeables)

| Authentification | Informations d'identification | S'applique à |
|---|---|---|
| **Authentification administrateur** | `Authorization: Bearer <admin_token>` (de `/api/v1/auth/login`) | Administrateur API |
| **Authentification du client** | `Authorization: Bearer <client_token>` (de `/api/v1/client-auth/login`) | Portail API |
| **Jeton d'abonnement** | Jeton dans le URL : `/sub/<token>` | Récupération d'abonnement, pas d'en-tête |

- Les jetons d'accès client durent 24 heures, actualisent les jetons 7 jours ; le JWT contient `tv` (token_version), donc la déconnexion/déconnexion forcée invalide tous les anciens jetons.
- Rôles d'administrateur : `admin` / `operator` / `super_admin` ; certaines opérations d'écriture nécessitent `RequireRole(RoleAdmin)` ou une version ultérieure.

## 3. Format de réponse

Succès (la plupart des points de terminaison) :

```json
{ "code": 0, "data": { ... }, "message": "ok" }
```

Les points de terminaison de la liste (certains) renvoient `{ "items": [...], "total": N }` ou un tableau nu (noté par point de terminaison ci-dessous).

Erreurs (HTTP 4xx/5xx) :

```json
{ "code": "ERROR_CODE", "message": "human-readable description", "request_id": "…", "can_retry": false }
```

**Les montants sont des nombres entiers en cents** : `990` = 9,90 ¥ (`currency: "CNY"`).
**Le trafic est en octets** : les requêtes et les réponses utilisent toutes deux des octets.

## 4. Limites de débit et mode de maintenance

- Les points de terminaison publics (`/api/v1/public/*`, `/sub/*`, inscription/connexion) sont limités en débit ;
- La connexion utilise également `LoginRateLimit`, la récupération d'abonnement utilise `SubscribeRateLimit` ;
- En mode maintenance, les points de terminaison publics et les abonnements renvoient un message de maintenance ; l'administrateur API n'est pas affecté.

## 5. Itinéraires de page frontale (pas API)

Ces routes renvoient des ressources HTML/statiques et se trouvent en dehors de la documentation API ; répertoriés afin qu'ils puissent être exclus lors de l'intégration :

| Méthode | Chemin | Descriptif |
|---|---|---|
| GET | `/` | Panneau d'administration SPA (après connexion) |
| GET | `/p` | Portail HTML (`/` sur un moteur de portail dédié) |
| GET | `/:token` | Récupération d'abonnement sur un port dédié (pas de préfixe `/sub`, même sémantique que `/sub/:token`) |
| GET | `/static/:file` | Actifs statiques frontend |

## 6. Source de vérité

`internal/api/router/router.go` (746 itinéraires, 743 documentés). La table de routage dans [admin-api.md](README.fr.md) est générée à partir du code ; s'il n'est pas d'accord avec le code, **le code gagne**.
