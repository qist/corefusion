<!-- source: portal-api.md | locale: fr | updated: 2026-10-23 -->
# Référence du portail API (Client / Portail API)

> Langues : [简体中文](portal-api.md) · [English](portal-api.en.md) · [繁體中文](portal-api.zh-TW.md) · [Русский](portal-api.ru.md) · [日本語](portal-api.ja.md) · [한국어](portal-api.ko.md) · [Deutsch](portal-api.de.md) · Français · [Español](portal-api.es.md) · [Português](portal-api.pt.md) · [Türkçe](portal-api.tr.md) · [Bahasa Indonesia](portal-api.id.md) · [ไทย](portal-api.th.md) · [Tiếng Việt](portal-api.vi.md) · [العربية](portal-api.ar.md) · [فارسی](portal-api.fa.md)

> La référence d'interface **complète** pour l'interface du portail (`/p` ou un port de portail dédié) et les clients tiers.
> Définitions d'itinéraire : `internal/api/router/router.go` (`clientAuthGroup` / `clientAuthProtected` / `/api/v1/public/*`).

## 1. Conventions générales

### 1.1 Base URL

| Déploiement | Socle URL |
|---|---|
| Moteur du panneau principal | `https://<panel-domain>/api/v1` (page du portail sur `/p`) |
| Port portail dédié | `https://<portal-domain>[:port]/api/v1` (page du portail sur `/`) |
| Domaine de portail dédié + proxy inverse | Comme ci-dessus ; le proxy doit transmettre les deux chemins de page et `/api/v1` |

### 1.2 Authentification

- Après vous être connecté, envoyez `Authorization: Bearer <access_token>` ;
- Les jetons d'accès durent 24h, actualisent les jetons 7 jours ;
- Le JWT contient `tv` (token_version) : la déconnexion/déconnexion forcée l'incrémente et invalide tous les anciens jetons
 (`clients.token_version`, voir `internal/service/clientauth/service.go`) ;
- Chaque point de terminaison protégé passe par `ClientAuthMiddleware` (vérification de version + garde en mode maintenance).

### 1.3 Enveloppe de réponse

La plupart des points de terminaison renvoient `{ "code": 0, "data": ..., "message": "..." }` ;
certains points de terminaison de liste renvoient `{ "items": [...], "total": N }` ou un tableau nu (noté par point de terminaison ci-dessous).
Les erreurs renvoient `{ "code": "ERROR_CODE", "message": "...", "request_id": "...", "can_retry": ... }`
(HTTP4xx/5xx).

### 1.4 Montant unités

Les champs de commande/solde sont des **cents entiers** : `990` = 9,90 ¥ (`currency: "CNY"`).

### 1.5 Exemple réel (connexion → récupérer le profil)

```bash
# Register (when registration_enabled=true)
curl -X POST https://<portal-domain>/api/v1/client-auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","password":"plaintext-password","referral_code":""}'
# → 201 {"data":{"access_token":"<JWT>","refresh_token":"<JWT>","client":{...}}}

# Sign in
curl -X POST https://<portal-domain>/api/v1/client-auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","password":"plaintext-password"}'
# → 200 same as above

# Fetch the current client profile (with token)
curl https://<portal-domain>/api/v1/client-auth/me \
  -H 'Authorization: Bearer <access_token>'
# → 200 {"data":{"id":27,"email":"user@example.com","nickname":"","status":"active","quota_bytes":10737418240,"used_bytes":0,...}}

# Fetch a subscription (no header, token in the path)
curl 'https://<portal-domain>/sub/<subscription_token>?format=mihomo_yaml'
# → 200 Mihomo YAML (or base64_uri / singbox_json / xray_json)

# Error example: expired token
# → 401 {"code":"errClientTokenRevoked","message":"...","request_id":"...","can_retry":false}
```

Au 401, appelez `POST /api/v1/client-auth/refresh` une fois et réessayez ; si l'échec persiste, effacez la session locale (le modèle de portail le fait déjà).

---

## 2. Points de terminaison publics (pas de connexion)

| Méthode | Chemin | Descriptif |
|---|---|---|
| GET | `/p` (`/` sur un moteur de portail dédié) | Portail HTML (rendu par le serveur + runtime Vue) |
| GET | `/api/v1/public/portal-config` | Configuration d'exécution du portail : nom du site, couleurs du thème, navigation, modules, modules linguistiques, `registration_enabled`, `regions` (source de région de connexion), `privacy_policy_url` / `terms_of_service_url` / `privacy_policy_content` / `terms_of_service_content` (documents de conformité ; contenu mappé par langue ; pages autonomes `/p/legal/*` ✓) |
| GET | `/api/v1/public/settings` | Paramètres publics (actuellement `registration_enabled`) |
| GET | `/api/v1/public/subscription-endpoints` | Liste de points de terminaison d'abonnement (tableau nu) |
| GET | `/api/v1/public/articles` | Articles publics (publiés uniquement) |
| GET | `/api/v1/public/articles/:slug` | Détail de l'article |
| GET | `/api/v1/public/client-configs` | Configuration du téléchargement du client |
| GET | `/sub/:token` | Livraison de l'abonnement (`format` renvoie base64_uri / raw_uri / mihomo_yaml / singbox_json / xray_json, etc. ; révoqué/manquant → 404, suspendu/expiré/inactif → 403. Voir [subscription.md](portal-api.fr.md)) |
| GET | `/api/v1/client-auth/oauth/providers` | Visibilité du fournisseur OAuth : `{"data":{"providers":[{"provider":"github","enabled":true},...]}}` |
| GET | `/api/v1/system/maintenance` | Statut de maintenance (public). Le portail affiche une page de maintenance lorsqu'il est activé |

> Tous ces éléments incluent la protection de maintenance et la limitation de débit publique API.

## 3. Authentification/OAuth

| Méthode | Chemin | Authentification | Descriptif |
|---|---|---|---|
| POST | `/api/v1/client-auth/register` | — | Inscrivez-vous. Corps : `{email, password, referral_code?}`. Renvoie `{access_token, refresh_token, client}` ; taux d'inscription limité |
| POST | `/api/v1/client-auth/login` | — | Connectez-vous. Corps : `{email, password}`. Même réponse ; émet des JWT avec `token_version` |
| POST | `/api/v1/client-auth/refresh` | — | Actualiser. Corps : `{refresh_token}`. Renvoie les nouveaux jetons d'accès/actualisation (version vérifiée ; les anciens jetons d'actualisation meurent immédiatement après la déconnexion) |
| POST | `/api/v1/client-auth/logout` | ✓ | Déconnexion : révocation côté serveur (`token_version +1`) → tous les anciens jetons du client ne sont pas valides |
| POST | `/api/v1/client-auth/rotate-token` | ✓ | Faites pivoter le jeton d'abonnement |
| POST | `/api/v1/client-auth/ticket` | ✓ | Émettez un **ticket unique de courte durée** (pour les flux `window.open` tels que « lot de journaux de téléchargement » pour lequel les en-têtes ne peuvent pas être définis) |
| GET | `/api/v1/client-auth/oauth/:provider` | — | Démarrer OAuth : 307 → Consentement GitHub/Google (`state` anti-CSRF, 10 minutes) ; les prestataires handicapés rendent 400 |
| GET | `/api/v1/client-auth/oauth/:provider/callback` | — | Rappel OAuth : code d'échange → créer/lier un client → émettre un JWT versionné → 302 vers le portail (`/<portalPath>?access_token=...&refresh_token=...`) |

> Le rappel OAuth est une **redirection de navigateur** (et non un appel API) : après une connexion réussie, le portail lit le jeton
> à partir du URL dans `onMounted` (ou via `postMessage` de même origine, voir `handleOAuthCallback` dans le modèle de portail `web`).

## 4. Compte

| Méthode | Chemin | Descriptif |
|---|---|---|
| GET | `/api/v1/client-auth/me` | Client actuel : `{id, email, plan_id, plan_name, status, quota_bytes, used_bytes, upload_bytes, download_bytes, expires_at, ip_whitelist, device_limit, sub_token, sub_status, created_at, ...}` (enveloppe `data`) |
| POST | `/api/v1/client-auth/change-password` | Corps : `{old_password, new_password}` |
| GET | `/api/v1/client-auth/checktime` | Heure du serveur (synchronisation de l'horloge client) |
| GET | `/api/v1/client-auth/connection-info` | Paramètres de connexion du plan actuel : `{concurrent, device_limit, protocol, speed_limit, tls, transport}` |

## 5. Abonnements

| Méthode | Chemin | Descriptif |
|---|---|---|
| POST | `/api/v1/client-auth/reset-subscription` | Réinitialiser l'abonnement ; cela fait tourner `sub_token` |

> Le lien d'abonnement est construit sur le frontend à partir de `sub_token` renvoyé par `/me` (`/sub/:token`) ; il n'y a pas
> ne séparez plus les points de terminaison `subscription-link` / `subscription-formats`.

## 6. Nœuds

| Méthode | Chemin | Descriptif |
|---|---|---|
| GET | `/api/v1/client-auth/nodes` | Nœuds visibles pour le plan actuel : `{items:[...]}` (champs d'affichage tels que nom/protocole/transport/région) |
| GET | `/api/v1/client-auth/nodes/:id/config` | Configuration complète du nœud |
| GET | `/api/v1/client-auth/nodes/:id/link` | Partager le lien (vless:// etc.) |
| GET | `/api/v1/client-auth/nodes/:id/qrcode` | Image du code QR |

## 7. Boutique / plans / commandes

| Méthode | Chemin | Descriptif |
|---|---|---|
| GET | `/api/v1/client-auth/plans` | Liste de plans (matrice nue) : `{id, name, description, category, sort_order, variants:[{id, name, billing_cycle, duration_days, price(cents), original_price, currency, quota_bytes, ...}]}` |
| GET | `/api/v1/client-auth/plans/:id` | Détail du forfait |
| GET | `/api/v1/client-auth/traffic-packs` | Liste des packs de trafic |
| POST | `/api/v1/client-auth/traffic-packs/buy` | Acheter un pack de trafic |
| GET | `/api/v1/client-auth/orders` | Mes commandes de plan (paginées) |
| POST | `/api/v1/client-auth/orders` | Créer une commande (acheter/renouveller une variante de plan) |
| GET | `/api/v1/client-auth/orders/:id` | Détail de la commande |
| GET | `/api/v1/client-auth/invoices` | Factures |
| GET | `/api/v1/client-auth/invoices/:id` | Détail de la facture |

## 8. Solde/recharge

| Méthode | Chemin | Descriptif |
|---|---|---|
| GET | `/api/v1/client-auth/balance` | `{data:{client_id, balance(cents), frozen}}` |
| GET | `/api/v1/client-auth/balance/transactions` | Transactions de solde (`type` : charge/consommation/remboursement ; paginé) |
| GET | `/api/v1/client-auth/charge/sites` | Sites de recharge disponibles (`ListChargeSites` : activé uniquement) |
| POST | `/api/v1/client-auth/charge/create` | Créer une commande de recharge (redirection vers un tiers payant) |
| GET | `/api/v1/client-auth/charge/orders` | Liste des commandes de recharge |
| GET | `/api/v1/client-auth/charge/orders/:id` | Détail de la commande de recharge |
| GET | `/api/v1/client-auth/charge/status/:trade_no` | Sonder le statut de paiement par trade_no |
| POST | `/api/v1/client-auth/charge/redeem-card` | Échanger une carte de recharge |

> La suppression d'un client **en cascade** sur son solde et ses transactions (voir `DeleteClient` dans `internal/service/client/service.go`).

## 9. Cadeaux / historique du trafic

| Méthode | Chemin | Descriptif |
|---|---|---|
| GET | `/api/v1/client-auth/gifts` | Cadeaux récupérables (tableau nu ; `type` : jours/octets) |
| POST | `/api/v1/client-auth/gifts/:id/claim` | Réclamation |
| GET | `/api/v1/client-auth/traffic-history` | Historique mensuel du trafic : `{data:{history:[{period_start,total_bytes,...}], months}}` |

## 10. Billets

| Méthode | Chemin | Descriptif |
|---|---|---|
| GET | `/api/v1/client-auth/tickets` | Liste des billets : `{items,total}` |
| POST | `/api/v1/client-auth/tickets` | Créer un billet |
| GET | `/api/v1/client-auth/tickets/:id` | Détail du ticket (avec réponses) |
| POST | `/api/v1/client-auth/tickets/:id/reply` | Répondre |
| POST | `/api/v1/client-auth/tickets/:id/close` | Fermer |

## 11. Articles/guides

| Méthode | Chemin | Descriptif |
|---|---|---|
| GET | `/api/v1/client-auth/articles` | Liste d'articles (avec filtre de catégorie) |
| GET | `/api/v1/client-auth/articles/:slug` | Détail de l'article |

## 12. Parrainages (désactivés par défaut ; peuvent être activés dans l'éditeur du portail)

| Méthode | Chemin | Descriptif |
|---|---|---|
| GET | `/api/v1/client-auth/referral/code` | Mon code de parrainage et mon lien |
| GET | `/api/v1/client-auth/referral/stats` | Statistiques de commissions (`commission_rate`, `available_commission`, `min_withdraw_amount`) |
| GET | `/api/v1/client-auth/referral/records` | Dossiers de référence |
| GET | `/api/v1/client-auth/referral/withdrawals` | Dossiers de retrait |
| POST | `/api/v1/client-auth/referral/withdraw` | Demander un retrait |

## 13. Comptes partagés

| Méthode | Chemin | Descriptif |
|---|---|---|
| GET | `/api/v1/client-auth/shared-accounts` | Comptes partagés disponibles (streaming, etc.) |
| GET | `/api/v1/client-auth/shared-accounts/:id` | Détail |
| POST | `/api/v1/client-auth/shared-accounts/:id/apply` | Demander l'accès |
| POST | `/api/v1/client-auth/shared-accounts/:id/repair` | Réparation en un clic |

## 14. Ensembles de journaux (diagnostics des abonnés)

| Méthode | Chemin | Descriptif |
|---|---|---|
| GET | `/api/v1/client-auth/logs/types` | Types de journaux pouvant être regroupés |
| GET | `/api/v1/client-auth/logs/packages` | Mes lots de journaux |
| POST | `/api/v1/client-auth/logs/package` | Créer une tâche de regroupement de journaux (nœuds d'entrée uniquement) |
| GET | `/api/v1/client-auth/logs/packages/:id` | Statut de la tâche (prêt/en cours/en échec) |
| GET | `/api/v1/client-auth/logs/packages/:id/download` | Télécharger (préférer `POST /ticket` et ajouter `?ticket=`) |
| DELETE | `/api/v1/client-auth/logs/packages/:id` | Supprimer un groupe de journaux |

## 15. Paramètres de sécurité/liste autorisée IP/auto-bannissement

| Méthode | Chemin | Descriptif |
|---|---|---|
| GET/PUT | `/api/v1/client-auth/settings` | Paramètres client : `{ip_whitelist, device_limit}` (PUT) |
| GET | `/api/v1/client-auth/settings/whitelist` | Liste verte (tableau nu) |
| DELETE | `/api/v1/client-auth/settings/whitelist/:id` | Supprimer une entrée de liste verte |
| POST | `/api/v1/client-auth/settings/auto-add-ip` | Ajouter l'adresse IP actuelle à la liste verte |
| POST | `/api/v1/client-auth/self-unban` | Annulation du bannissement en libre-service (tentatives limitées par la politique) |

> La liste verte IP **s'applique uniquement aux URL d'abonnement** (`/sub/:token`) ; il ne bloque pas la connexion au portail/OAuth ou
> d'autres points de terminaison d'authentification client. Si l'utilisateur n'a pas de liste verte ou d'abonnement URL, l'accès est toujours autorisé. Le
> la liste verte est un paramètre par utilisateur ; il n'y a pas de commutateur global.

**Flux d'auto-débannissement** : disponible uniquement pour `auto_quota` (interdiction automatique de dépassement de trafic) lorsque le côté administrateur
`self_unban.enabled` est allumé. La connexion (mot de passe ou rappel OAuth) renvoie `require_self_unban=true` + `self_unban_token`
(typ=self_unban, valide 15 minutes, **uniquement** utilisable sur `POST /self-unban` ; les autres points de terminaison renvoient 403/`SELF_UNBAN_TOKEN_SCOPE`).
Le portail affiche alors la carte de déblocage ; après un bannissement réussi, l'utilisateur se reconnecte. Les rappels OAuth fournissent cela via
le fragment URL `#require_self_unban=1&self_unban_token=..&self_unban_hint=..`.

## 16. Codes d'erreur (extrait)

| coder | Scénario |
|---|---|
| `INVALID_TOKEN` | Jeton d'actualisation invalide/expiré/incompatibilité de version |
| `INVALID_REQUEST` | La validation des paramètres a échoué |
| `NOT_FOUND` | Ressource manquante (ticket/commande/entrée dans la liste verte, etc.) |
| `RELOAD_FAILED` / `SYNC_FAILED` | (Côté administrateur) La synchronisation de la configuration du proxy a échoué |
| 401 + `errClientTokenRevoked` | non-concordance de token_version (déconnecté/expulsé) → les clients doivent s'actualiser ; si cela échoue, effacez les jetons locaux |

## 17. Documents de conformité (politique de confidentialité / conditions d'utilisation)

Les liens « Politique de confidentialité · Conditions d'utilisation » au bas de la carte de connexion (identiques aux sites ordinaires ; cliquer pour ouvrir le contenu) :

- **Configuration** : éditeur de portail → Paramètres → Général → Documents de conformité (**modifier le contenu par langue**) ;
 vous pouvez également `PUT /api/v1/portal/active` avec des champs structurés `privacy_policy_url` / `privacy_policy_content` /
 `terms_of_service_url` / `terms_of_service_content` (nil = inchangé ; le contenu est une carte `{"zh-CN":"...","en":"..."}`).
- **Pages accessibles de manière autonome** (rendu par le serveur, facile pour les robots de vérification Google ; prend en charge `?lang=xx`) :
  - Domaine du panneau principal : `GET /p/legal/privacy`, `GET /p/legal/terms`
  - Domaine de portail dédié (déploiement racine) : `GET /legal/privacy`, `GET /legal/terms`
- **Logique d'affichage** : URL ou contenu, **URL gagne** (s'ouvre en externe dans un nouvel onglet) ; si du contenu est fourni, en cliquant sur le
 la carte de connexion affiche Markdown dans une boîte de dialogue (`renderMd`) en utilisant la langue actuelle du portail.
- Livré au moment de l'exécution via `/api/v1/public/portal-config` ; le composant de connexion consomme
 `ctx.privacyUrl / termsUrl / privacyContent / termsContent / openLegal / hasLegalLinks`.
- La vérification OAuth Google nécessite que ces liens soient accessibles publiquement → utilisez les liens autonomes ci-dessus dans le
 écran de consentement ; voir `docs/oauth-google-verification.md` (contient des modèles de politique de confidentialité/conditions).

## 18. Exemple d'intégration

```js
// Sign in
const { data } = await fetch('/api/v1/client-auth/login', {
  method: 'POST', headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ email, password })
}).then(r => r.json());
localStorage.setItem('client_token', data.access_token);
localStorage.setItem('client_refresh_token', data.refresh_token);

// Authenticated request
const me = await fetch('/api/v1/client-auth/me', {
  headers: { Authorization: 'Bearer ' + data.access_token }
}).then(r => r.json());
```

Au 401, appelez `POST /refresh` une fois et réessayez ; si cela échoue toujours, effacez la session locale
(le `api()` du modèle de portail l'implémente déjà).
