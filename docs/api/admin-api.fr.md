<!-- source: admin-api.md | locale: fr | updated: 2026-11-16 -->
# Administrateur API

> Langues : [简体中文](admin-api.md) · [English](admin-api.en.md) · [繁體中文](admin-api.zh-TW.md) · [Русский](admin-api.ru.md) · [日本語](admin-api.ja.md) · [한국어](admin-api.ko.md) · [Deutsch](admin-api.de.md) · Français · [Español](admin-api.es.md) · [Português](admin-api.pt.md) · [Türkçe](admin-api.tr.md) · [Bahasa Indonesia](admin-api.id.md) · [ไทย](admin-api.th.md) · [Tiếng Việt](admin-api.vi.md) · [العربية](admin-api.ar.md) · [فارسی](admin-api.fa.md)

La table de routage d'administration complète API pour le **développement de panneaux/automatisation/intégrateurs**.

- Base URL : `https://<panel-domain>/api/v1`
- Auteur : `Authorization: Bearer <admin_token>` (à partir de `POST /api/v1/auth/login`)
- Formats de réponse/erreur courants et conventions de limite de débit : voir [README.md](./README.fr.md)
- Source de vérité : `internal/api/router/router.go` (le nombre varie selon la version ; le code fait autorité) ; si ce tableau n'est pas d'accord, **le code gagne**

## Marqueurs d'autorisation

| Marqueur | Signification |
|---|---|
| Publique | Aucune connexion requise (un agent de maintenance/une limitation de taux peut toujours s'appliquer) |
| Public · tarif limité | Pas de connexion, avec une limite de tarif dédiée |
| Connecté | Nécessite un jeton d'administrateur connecté (`AuthRequired`) |
| Opérateur | Requiert le rôle ≥ `operator` (`RequireRole(RoleOperator)`) |
| Administrateur | Nécessite un rôle ≥ `admin` (`RequireRole(RoleAdmin)`) |
| Super administrateur | Requiert le rôle = `super_admin` |
| Client (public) | Aucun jeton client requis (connexion/inscription, etc.) |
| Client (connecté) | Nécessite `Authorization: Bearer <client_token>` (un jeton client de portail, pas un jeton d'administrateur) |

Niveaux de rôle : `viewer(10) < operator(20) < admin(30) < super_admin(40)` ; les rôles insuffisants renvoient 403 `INSUFFICIENT_ROLE`.

---

## Démarrage rapide (exemples réels)

Ces exemples proviennent de véritables réponses locales ; les jetons et les données sont expurgés.

### 1. Connectez-vous et obtenez un jeton

```bash
curl -X POST https://<panel-domain>/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"your-password"}'
```

Réponse :

```json
{
  "data": {
    "access_token": "<JWT, valid 24h>",
    "refresh_token": "<JWT, valid 7 days>",
    "user": { "id": 1, "role": "super_admin", "username": "admin" }
  }
}
```

### 2. Appeler un point de terminaison métier avec le jeton

```bash
curl https://<panel-domain>/api/v1/clients?limit=1 \
  -H 'Authorization: Bearer <access_token>'
```

Réponse (les listes utilisent la structure `items` + `total`) :

```json
{
  "data": {
    "items": [
      {
        "id": 27,
        "email": "user@example.com",
        "status": "active",
        "quota_bytes": 10737418240,
        "used_bytes": 0,
        "plan_id": 1,
        "expires_at": null,
        "created_at": "2026-09-30T15:54:32+08:00"
      }
    ],
    "total": 22,
    "limit": 1,
    "offset": 0
  }
}
```

### 3. Exemples de réponses d'erreur

**401 non authentifié :**

```json
{
  "code": "MISSING_TOKEN",
  "message": "missing authorization header",
  "request_id": "a02cfdf9-6be0-46be-8584-824caab34dc7",
  "can_retry": false,
  "requires_action": false
}
```

**Ressource 404 introuvable :**

```json
{
  "code": "CLIENT_NOT_FOUND",
  "message": "client not found",
  "request_id": "099949bd-4ded-468d-a6f8-f8e798af2273",
  "can_retry": false,
  "requires_action": false
}
```

### 4. Pagination

Tous les points de terminaison de liste utilisent `limit` (taille de page, 20 par défaut) et `offset` (basé sur 0) et renvoient `{items, total, limit, offset}`. Certains points de terminaison hérités renvoient un tableau nu (noté par tableau).

---

## Contenu

- [Authentification et sécurité de l'administrateur](#authentification-et-sécurité-de-ladministrateur)
- [Comptes administrateur](#comptes-administrateur)
- [Gestion des clients](#gestion-des-clients)
- [Modèles de forfaits](#modèles-de-forfaits)
- [Plan variants](#plan-variants)
- [Traffic packs](#traffic-packs)
- [Coupons](#coupons)
- [Groupes de nœuds](#groupes-de-nœuds)
- [Serveurs de nœuds](#serveurs-de-nœuds)
- [Journal d'audit du nœud](#journal-daudit-du-nœud)
- [Nœuds (ancien modèle, retiré)](#nœuds-ancien-modèle-retiré)
- [Aperçu de la région](#aperçu-de-la-région)
- [Abonnements](#abonnements)
- [Formats d'abonnement](#formats-dabonnement)
- [Points de terminaison d'abonnement](#points-de-terminaison-dabonnement)
- [Partager des domaines](#partager-des-domaines)
- [CloudflareIP](#cloudflareip)
- [Configuration de l'abonnement (Mihomo/Clash)](#configuration-de-labonnement-mihomoclash)
- [Sites de recharge](#sites-de-recharge)
- [Recharger les commandes](#recharger-les-commandes)
- [Recharger les cartes](#recharger-les-cartes)
- [Événements de rappel de facturation](#événements-de-rappel-de-facturation)
- [Facturer les tentatives de rappel](#facturer-les-tentatives-de-rappel)
- [Solde client](#solde-client)
- [Cadeaux](#cadeaux)
- [Configuration de référence](#configuration-de-référence)
- [Retraits de référence](#retraits-de-référence)
- Listes autorisées [IP (résumé)](#ip-allow-lists-summary)
- [Listes d'autorisation IP (résumé)](#listes-dautorisation-ip-résumé)
- [Liaisons OAuth du client](#liaisons-oauth-du-client)
- [Billets (administrateur)](#billets-administrateur)
- [Articles](#articles)
- [Comptes partagés](#comptes-partagés)
- [Certificats](#certificats)
- Configuration [EAB](#eab-configuration)
- Comptes [ACME](#acme-accounts)
- [CA](#built-in-ca) intégré
- [Canaux de certificat](#canaux-de-certificat)
- Bibliothèque de règles [WAF](#waf-rule-library)
- [Configuration EAB](#configuration-eab)
- [Comptes ACME](#comptes-acme)
- [Autorité de certification intégrée](#autorité-de-certification-intégrée)
- [Règles géographiques](#règles-géographiques)
- [Bibliothèque de règles WAF](#bibliothèque-de-règles-waf)
- [Gestion des journaux](#gestion-des-journaux)
- [Sauvegardes](#sauvegardes)
- [Paramètres système](#paramètres-système)
- [Audit de sécurité](#audit-de-sécurité)
- [Métriques](#métriques)
- [Modèles de portail](#modèles-de-portail)
- [Installation des agents](#installation-des-agents)
- [Entrée de rappel de facturation](#entrée-de-rappel-de-facturation)
- [Entrées (avec nœuds)](#entrées-avec-nœuds)
- [Configuration publique](#configuration-publique)
- [Authentification client et activité (identiques au moteur d'administration)](#authentification-client-et-activité-identiques-au-moteur-dadministration)
- [Livraison d'abonnement](#livraison-dabonnement)
- [Flux de travail de configuration d'exécution](#flux-de-travail-de-configuration-dexécution)

---

### Authentification et sécurité de l'administrateur

Connexion, déconnexion, changement de mot de passe, 2FA (TOTP), Passkey (WebAuthn) et tickets uniques pour le panneau d'administration.

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| POST | `/api/v1/auth/2fa/disable` | Connecté | Désactiver |
| POST | `/api/v1/auth/2fa/enable` | Connecté | Activer |
| POST | `/api/v1/auth/2fa/recovery-codes` | Connecté | Régénérer les codes de récupération |
| POST | `/api/v1/auth/2fa/setup` | Connecté | Paramètres |
| GET | `/api/v1/auth/2fa/status` | Connecté | Statut |
| POST | `/api/v1/auth/change-password` | Connecté | Changer le mot de passe |
| POST | `/api/v1/auth/login` | Publique | Connectez-vous |
| POST | `/api/v1/auth/login/2fa` | Publique | Connexion par code dynamique (2FA) |
| GET | `/api/v1/auth/login/2fa-status` | Publique | Vérifiez si le compte a 2FA activé |
| POST | `/api/v1/auth/logout` | Connecté | Se déconnecter |
| GET | `/api/v1/auth/me` | Connecté | Utilisateur actuellement connecté |
| GET | `/api/v1/auth/passkey/credentials` | Connecté | Liste des informations d'identification Passkey |
| DELETE | `/api/v1/auth/passkey/credentials/:id` | Connecté | Supprimer les informations d'identification Passkey |
| POST | `/api/v1/auth/passkey/login/begin` | Publique | Commencer la connexion |
| POST | `/api/v1/auth/passkey/login/finish` | Publique | Connexion complète |
| POST | `/api/v1/auth/passkey/register/begin` | Connecté | Commencer l'inscription |
| POST | `/api/v1/auth/passkey/register/finish` | Connecté | Inscription complète |
| GET | `/api/v1/auth/passkey/rpids` | Administrateur | Liste autorisée des ID RP |
| POST | `/api/v1/auth/passkey/rpids` | Administrateur | Mettre à jour la liste verte des ID RP |
| GET | `/api/v1/auth/passkey/status` | Publique | Statut |
| POST | `/api/v1/auth/refresh` | Publique | Actualiser |
| POST | `/api/v1/auth/ticket` | Connecté | Émettre un ticket unique de courte durée |

### Comptes administrateur

Comptes d'utilisateurs du panneau d'administration. La liste nécessite un administrateur ; la création/mise à jour/suppression nécessite un super administrateur (empêche l'élévation des privilèges).

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| GET | `/api/v1/users` | Administrateur | Liste |
| POST | `/api/v1/users` | Super administrateur | Créer |
| DELETE | `/api/v1/users/:id` | Super administrateur | Supprimer |
| PUT | `/api/v1/users/:id` | Super administrateur | Mise à jour |

### Gestion des clients

CRUD, suspension/reprise, mise à niveau/rétrogradation, renouvellement et réinitialisation de l'utilisation pour les abonnés (comptes de portail).

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| GET | `/api/v1/clients` | Connecté | Liste des clients |
| POST | `/api/v1/clients` | Opérateur | Créer un client. corps : `{"email":"user@example.com","password":"plaintext-password","plan_id":1,"quota_bytes":10737418240,"expires_at":"2026-12-31T00:00:00Z","remark":""}`. `plan_id` est requis ; `password` peut être vide (les clients peuvent le définir ultérieurement dans le portail) |
| DELETE | `/api/v1/clients/:id` | Opérateur | Supprimer le client |
| GET | `/api/v1/clients/:id` | Connecté | Obtenir un client |
| PUT | `/api/v1/clients/:id` | Opérateur | Mettre à jour le client |
| POST | `/api/v1/clients/:id/downgrade` | Opérateur | Rétrograder le client |
| GET | `/api/v1/clients/:id/orders` | Connecté | Liste des commandes clients |
| POST | `/api/v1/clients/:id/renew` | Opérateur | Renouveler le client |
| POST | `/api/v1/clients/:id/reset-usage` | Opérateur | Réinitialiser l'utilisation |
| POST | `/api/v1/clients/:id/restore` | Opérateur | Reprendre le client |
| POST | `/api/v1/clients/:id/suspend` | Opérateur | Suspendre le client |
| POST | `/api/v1/clients/:id/upgrade` | Opérateur | Client de mise à niveau |

### Modèles de forfaits

Plans (variantes, cycles de facturation, prix, quotas de trafic) et autorisation de nœud au niveau du modèle.

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| GET | `/api/v1/plan-templates` | Connecté | Liste des modèles |
| POST | `/api/v1/plan-templates` | Administrateur | Créer un modèle |
| DELETE | `/api/v1/plan-templates/:id` | Administrateur | Supprimer le modèle |
| GET | `/api/v1/plan-templates/:id` | Connecté | Détails du modèle |
| PUT | `/api/v1/plan-templates/:id` | Administrateur | Mettre à jour le modèle |
| GET | `/api/v1/plan-templates/:id/accessible-nodes` | Connecté | Obtenir les nœuds disponibles |
| GET | `/api/v1/plan-templates/:id/billing-node-options` | Connecté | Obtenez des options de nœuds facturables |
| PUT | `/api/v1/plan-templates/:id/node-groups` | Administrateur | Set template node groups |
| POST | `/api/v1/plan-templates/:id/sync-accounts` | Administrateur | Synchroniser les comptes avec le modèle |
| GET | `/api/v1/plan-templates/:id/variants` | Connecté | Variant list |
| POST | `/api/v1/plan-templates/:id/variants` | Administrateur | Create variant |
| POST | `/api/v1/plan-templates/sync-all-accounts` | Administrateur | Sync all accounts |

### Plan variants

Mettre à jour et supprimer des variantes de plan (la création est imbriquée sous les modèles de plan).

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| DELETE | `/api/v1/plan-variants/:id` | Administrateur | Delete variant |
| PUT | `/api/v1/plan-variants/:id` | Administrateur | Update variant |

### Traffic packs

Packs de trafic pouvant être achetés séparément.

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| GET | `/api/v1/traffic-packs` | Connecté | Liste des packs de trafic |
| POST | `/api/v1/traffic-packs` | Administrateur | Create traffic pack |
| DELETE | `/api/v1/traffic-packs/:id` | Administrateur | Supprimer le pack de trafic |
| PUT | `/api/v1/traffic-packs/:id` | Administrateur | Mettre à jour le pack de trafic |

### Coupons

Créez, validez et échangez des coupons.

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| GET | `/api/v1/promotions` | Connecté | Liste des coupons |
| POST | `/api/v1/promotions` | Administrateur | Créer un coupon |
| DELETE | `/api/v1/promotions/:id` | Administrateur | Supprimer le coupon |
| PUT | `/api/v1/promotions/:id` | Administrateur | Mettre à jour le coupon |
| POST | `/api/v1/promotions/validate` | Connecté | Valider le coupon |

### Groupes de nœuds

Organisez les nœuds en groupes référencés par des plans.

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| GET | `/api/v1/node-groups` | Connecté | Répertorier les groupes de nœuds |
| POST | `/api/v1/node-groups` | Administrateur | Créer un groupe de nœuds |
| DELETE | `/api/v1/node-groups/:id` | Administrateur | Supprimer le groupe de nœuds |
| PUT | `/api/v1/node-groups/:id` | Administrateur | Mettre à jour le groupe de nœuds |
| GET | `/api/v1/node-groups/:id/members` | Connecté | Répertorier les membres du groupe |
| POST | `/api/v1/node-groups/:id/members` | Administrateur | Ajouter un membre du groupe |
| DELETE | `/api/v1/node-groups/:id/members/:mid` | Administrateur | Supprimer un membre du groupe |

### Serveurs de nœuds

Ressources de base : nœud CRUD, livraison de la configuration de base (xray/sing-box), configuration entrante/sortante/point de terminaison, contrôle de processus, certificats, WAF, limites de débit, saut de port, proxys chaînés, sauvegarde miroir/primaire, installation/mise à niveau de l'agent, contrôle d'exécution xray. De nombreux sous-chemins, regroupés par famille de fonctionnalités.

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| GET | `/api/v1/node-servers` | Administrateur | Liste |
| POST | `/api/v1/node-servers` | Administrateur | Créer |
| DELETE | `/api/v1/node-servers/:id` | Administrateur | Supprimer |
| GET | `/api/v1/node-servers/:id` | Administrateur | Détails |
| PUT | `/api/v1/node-servers/:id` | Administrateur | Mise à jour |
| GET | `/api/v1/node-servers/:id/binaries` | Administrateur | Liste des noyaux installés |
| POST | `/api/v1/node-servers/:id/binaries/ech-generate` | Administrateur | Générer la paire de clés ECH |
| POST | `/api/v1/node-servers/:id/binaries/install` | Administrateur | Installer/mettre à niveau le noyau |
| GET | `/api/v1/node-servers/:id/binaries/interfaces` | Administrateur | Liste des interfaces réseau |
| POST | `/api/v1/node-servers/:id/binaries/mldsa65-key` | Administrateur | Générer la clé MLDSA65 |
| POST | `/api/v1/node-servers/:id/binaries/reality-key` | Administrateur | Générer la paire de clés REALITY |
| POST | `/api/v1/node-servers/:id/binaries/reality-public-key` | Administrateur | Dériver la clé publique à partir de la clé privée REALITY |
| GET | `/api/v1/node-servers/:id/binaries/releases` | Administrateur | Versions disponibles |
| POST | `/api/v1/node-servers/:id/binaries/vlessenc` | Administrateur | Générer les paramètres de chiffrement VLESS |
| POST | `/api/v1/node-servers/:id/binaries/wg-key` | Administrateur | Générer la clé WireGuard |
| POST | `/api/v1/node-servers/:id/binaries/x25519-key` | Administrateur | Générer la paire de clés X25519 |
| GET | `/api/v1/node-servers/:id/bootstrap-cmd` | Administrateur | Obtenir la commande d'amorçage de l'agent |
| POST | `/api/v1/node-servers/:id/bootstrap-token` | Administrateur | Générer un jeton d'amorçage |
| POST | `/api/v1/node-servers/:id/caddy/batch-update-log-enabled` | Administrateur | Commutateurs de journal de mise à jour groupée |
| POST | `/api/v1/node-servers/:id/caddy/batch-update-ports` | Administrateur | Ports de mise à jour groupée |
| POST | `/api/v1/node-servers/:id/caddy/generate-config` | Administrateur | Générer la configuration |
| DELETE | `/api/v1/node-servers/:id/caddy/l4-rules` | Administrateur | Supprimer toutes les règles L4 |
| POST | `/api/v1/node-servers/:id/caddy/save-and-apply` | Administrateur | Enregistrer et postuler |
| GET | `/api/v1/node-servers/:id/caddy/sites` | Administrateur | Liste des sites |
| POST | `/api/v1/node-servers/:id/caddy/sites` | Administrateur | Créer un site |
| POST | `/api/v1/node-servers/:id/caddy/import` | Administrateur | Sites d'importation |
| POST | `/api/v1/node-servers/:id/caddy/preview-site` | Administrateur | Aperçu du rendu du site Caddy |
| DELETE | `/api/v1/node-servers/:id/caddy/sites/:siteId` | Administrateur | Supprimer le site |
| PUT | `/api/v1/node-servers/:id/caddy/sites/:siteId` | Administrateur | Mettre à jour le site |
| DELETE | `/api/v1/node-servers/:id/caddy/sni-rules` | Administrateur | Supprimer toutes les règles SNI |
| POST | `/api/v1/node-servers/:id/caddy/sni-rules/toggle` | Administrateur | Basculer les règles SNI |
| GET | `/api/v1/node-servers/:id/caddy/sni-settings` | Administrateur | Paramètres du SNI |
| POST | `/api/v1/node-servers/:id/caddy/sni-settings` | Administrateur | Enregistrer les paramètres du SNI |
| GET | `/api/v1/node-servers/:id/certificates` | Administrateur | Liste des certificats de nœud |
| DELETE | `/api/v1/node-servers/:id/certificates/:cid` | Administrateur | Dissocier le certificat |
| POST | `/api/v1/node-servers/:id/certificates/:cid` | Administrateur | Lier le certificat |
| GET | `/api/v1/node-servers/:id/chain-config` | Administrateur | Générer une configuration chaînée |
| GET | `/api/v1/node-servers/:id/chain-health` | Administrateur | État de santé de la chaîne |
| POST | `/api/v1/node-servers/:id/control` | Super administrateur | Noyau du nœud de contrôle. corps : `{"core_type":"xray","action":"start"}` (action = démarrage/arrêt/redémarrage). `core_type` peut être `xray`/`singbox` |
| GET | `/api/v1/node-servers/:id/degradation-status` | Administrateur | Statut dégradé |
| POST | `/api/v1/node-servers/:id/distribute-cert` | Administrateur | Distribuer le certificat |
| GET | `/api/v1/node-servers/:id/endpoint-configs` | Administrateur | Liste |
| POST | `/api/v1/node-servers/:id/endpoint-configs` | Administrateur | Créer |
| DELETE | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | Administrateur | Supprimer |
| GET | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | Administrateur | Détails |
| PUT | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | Administrateur | Mise à jour |
| POST | `/api/v1/node-servers/:id/failover` | Administrateur | Déclencher le basculement pour ce nœud |
| GET | `/api/v1/node-servers/:id/failover-status` | Administrateur | Statut de basculement |
| POST | `/api/v1/node-servers/:id/force-reconnect` | Administrateur | Forcer la reconnexion |
| GET | `/api/v1/node-servers/:id/free-port` | Administrateur | Demander un port gratuit |
| POST | `/api/v1/node-servers/:id/geo-rules/push` | Administrateur | Pousser vers ce nœud |
| GET | `/api/v1/node-servers/:id/firewall/status` | Administrateur | Détection de pare-feu (firewalld/ufw/nft/iptables) |
| POST | `/api/v1/node-servers/:id/firewall/allow-port` | Administrateur | Autoriser le port d'écoute entrant (multi-gamme pris en charge) |
| POST | `/api/v1/node-servers/:id/firewall/remove-port` | Administrateur | Révoquer l'autorisation de port d'arrivée (y compris les doublons) |
| GET | `/api/v1/node-servers/:id/inbound-configs` | Administrateur | Liste |
| POST | `/api/v1/node-servers/:id/inbound-configs` | Administrateur | Créer |
| DELETE | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | Administrateur | Supprimer |
| GET | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | Administrateur | Détails |
| PUT | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | Administrateur | Mise à jour |
| GET | `/api/v1/node-servers/:id/inbound-configs/:cfgId/share-links` | Administrateur | Partager des liens et des extraits |
| POST | `/api/v1/node-servers/:id/inbound-configs/:cfgId/users/:email/pause` | Administrateur | Suspendre cet utilisateur |
| POST | `/api/v1/node-servers/:id/inbound-configs/:cfgId/users/:email/resume` | Administrateur | Restaurer cet utilisateur |
| POST | `/api/v1/node-servers/:id/inbound-configs/reset-all-traffic` | Administrateur | Réinitialiser tout le trafic entrant |
| GET | `/api/v1/node-servers/:id/install-script` | Administrateur | Obtenir le script d'installation de l'agent |
| GET | `/api/v1/node-servers/:id/metrics-history` | Administrateur | Obtenir l'historique des métriques |
| GET | `/api/v1/node-servers/:id/mirror-status` | Administrateur | Statut du miroir |
| POST | `/api/v1/node-servers/:id/nginx/batch-update-ports` | Administrateur | Ports de mise à jour groupée |
| POST | `/api/v1/node-servers/:id/nginx/generate-config` | Administrateur | Générer la configuration |
| DELETE | `/api/v1/node-servers/:id/nginx/l4-rules` | Administrateur | Supprimer toutes les règles L4 |
| POST | `/api/v1/node-servers/:id/nginx/save-and-apply` | Administrateur | Enregistrer et postuler |
| GET | `/api/v1/node-servers/:id/nginx/settings` | Administrateur | Lire les paramètres |
| POST | `/api/v1/node-servers/:id/nginx/settings` | Administrateur | Enregistrer les paramètres |
| GET | `/api/v1/node-servers/:id/nginx/sites` | Administrateur | Liste des sites |
| POST | `/api/v1/node-servers/:id/nginx/sites` | Administrateur | Créer un site |
| POST | `/api/v1/node-servers/:id/nginx/import` | Administrateur | Sites d'importation |
| POST | `/api/v1/node-servers/:id/nginx/preview-site` | Administrateur | Aperçu du site Nginx rendu |
| POST | `/api/v1/node-servers/:id/nginx/ssl/generate` | Administrateur | Générer des fichiers de certificat SSL |
| DELETE | `/api/v1/node-servers/:id/nginx/sites/:siteId` | Administrateur | Supprimer le site |
| PUT | `/api/v1/node-servers/:id/nginx/sites/:siteId` | Administrateur | Mettre à jour le site |
| DELETE | `/api/v1/node-servers/:id/nginx/sni-rules` | Administrateur | Supprimer toutes les règles SNI |
| POST | `/api/v1/node-servers/:id/nginx/sni-rules/toggle` | Administrateur | Basculer les règles SNI |
| PUT | `/api/v1/node-servers/:id/node-type` | Administrateur | Mettre à jour le type de nœud |
| GET | `/api/v1/node-servers/:id/online-users` | Administrateur | Utilisateurs en ligne |
| GET | `/api/v1/node-servers/:id/outbound-configs` | Administrateur | Liste |
| POST | `/api/v1/node-servers/:id/outbound-configs` | Administrateur | Créer |
| DELETE | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | Administrateur | Supprimer |
| GET | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | Administrateur | Détails |
| PUT | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | Administrateur | Mise à jour |
| PUT | `/api/v1/node-servers/:id/outbound-configs/reorder` | Administrateur | Trier |
| GET | `/api/v1/node-servers/:id/port-hop/rules` | Administrateur | Aperçu de la règle de saut de port |
| GET | `/api/v1/node-servers/:id/protocols` | Administrateur | Liste des protocoles |
| GET | `/api/v1/node-servers/:id/protocols/:coreType` | Administrateur | Configuration du protocole |
| PUT | `/api/v1/node-servers/:id/protocols/:coreType` | Administrateur | Enregistrer la configuration du protocole |
| GET | `/api/v1/node-servers/:id/protocols/:coreType/config-file` | Administrateur | Lire le fichier de configuration de base |
| GET | `/api/v1/node-servers/:id/protocols/:coreType/section/:section` | Administrateur | Lire la section protocole |
| PUT | `/api/v1/node-servers/:id/protocols/:coreType/section/:section` | Administrateur | Enregistrer la section du protocole |
| POST | `/api/v1/node-servers/:id/protocols/:coreType/validate` | Administrateur | Valider la configuration (véritable essai à sec) |
| GET | `/api/v1/node-servers/:id/proxy-status` | Administrateur | Statut du proxy distant |
| POST | `/api/v1/node-servers/:id/proxy/batch-update-ports` | Administrateur | Ports de mise à jour groupée |
| GET | `/api/v1/node-servers/:id/proxy/config` | Administrateur | Lire la configuration |
| PUT | `/api/v1/node-servers/:id/proxy/config` | Administrateur | Mettre à jour la configuration |
| POST | `/api/v1/node-servers/:id/proxy/control` | Administrateur | Contrôle d'enregistrement de proxy chaîné |
| POST | `/api/v1/node-servers/:id/proxy/download` | Administrateur | Télécharger la version principale du proxy |
| POST | `/api/v1/node-servers/:id/proxy/generate-config` | Administrateur | Générer la configuration |
| POST | `/api/v1/node-servers/:id/proxy/guardian` | Administrateur | Mettre à jour la configuration du superviseur |
| POST | `/api/v1/node-servers/:id/proxy/install` | Administrateur | Installer le proxy |
| DELETE | `/api/v1/node-servers/:id/proxy/l4-rules` | Administrateur | Supprimer toutes les règles L4 |
| GET | `/api/v1/node-servers/:id/proxy/l4-settings` | Administrateur | Paramètres L4 |
| POST | `/api/v1/node-servers/:id/proxy/l4-settings` | Administrateur | Enregistrer les paramètres L4 |
| GET | `/api/v1/node-servers/:id/proxy/log-settings` | Administrateur | Paramètres du journal |
| POST | `/api/v1/node-servers/:id/proxy/log-settings` | Administrateur | Enregistrer les paramètres du journal |
| GET | `/api/v1/node-servers/:id/proxy/nginx-settings` | Administrateur | Paramètres du nginx |
| POST | `/api/v1/node-servers/:id/proxy/nginx-settings` | Administrateur | Enregistrer les paramètres du nginx |
| GET | `/api/v1/node-servers/:id/proxy/releases` | Administrateur | Versions disponibles |
| POST | `/api/v1/node-servers/:id/proxy/reload` | Administrateur | Recharger la configuration |
| POST | `/api/v1/node-servers/:id/proxy/restart` | Administrateur | Redémarrer |
| GET | `/api/v1/node-servers/:id/proxy/sites` | Administrateur | Liste des sites |
| POST | `/api/v1/node-servers/:id/proxy/sites` | Administrateur | Créer un site |
| DELETE | `/api/v1/node-servers/:id/proxy/sites/:siteId` | Administrateur | Supprimer le site |
| PUT | `/api/v1/node-servers/:id/proxy/sites/:siteId` | Administrateur | Mettre à jour le site |
| DELETE | `/api/v1/node-servers/:id/proxy/sni-rules` | Administrateur | Supprimer toutes les règles SNI |
| POST | `/api/v1/node-servers/:id/proxy/sni-rules/toggle` | Administrateur | Basculer les règles SNI |
| GET | `/api/v1/node-servers/:id/proxy/sni-settings` | Administrateur | Paramètres du SNI |
| POST | `/api/v1/node-servers/:id/proxy/sni-settings` | Administrateur | Enregistrer les paramètres du SNI |
| POST | `/api/v1/node-servers/:id/proxy/start` | Administrateur | Commencer |
| GET | `/api/v1/node-servers/:id/proxy/status` | Administrateur | État du processus de proxy |
| POST | `/api/v1/node-servers/:id/proxy/stop` | Administrateur | Arrêter |
| POST | `/api/v1/node-servers/:id/proxy/sync` | Administrateur | Synchroniser la configuration du proxy |
| GET | `/api/v1/node-servers/:id/rate-limit` | Administrateur | Lire la configuration |
| PUT | `/api/v1/node-servers/:id/rate-limit` | Administrateur | Mettre à jour la configuration |
| GET | `/api/v1/node-servers/:id/rate-limit/status` | Administrateur | Statut |
| GET | `/api/v1/node-servers/:id/recover-mode` | Administrateur | Mode de récupération du nœud de lecture |
| PUT | `/api/v1/node-servers/:id/recover-mode` | Administrateur | Définir le mode de récupération du nœud |
| POST | `/api/v1/node-servers/:id/revoke-cert` | Administrateur | Révoquer le certificat de l'agent |
| GET | `/api/v1/node-servers/:id/services` | Administrateur | Liste |
| POST | `/api/v1/node-servers/:id/services` | Administrateur | Créer |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId` | Administrateur | Supprimer |
| GET | `/api/v1/node-servers/:id/services/:serviceId` | Administrateur | Détails |
| PUT | `/api/v1/node-servers/:id/services/:serviceId` | Administrateur | Mise à jour |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId/crash-logs` | Administrateur | Effacer les journaux de crash |
| GET | `/api/v1/node-servers/:id/services/:serviceId/crash-logs` | Administrateur | Liste des journaux de crash |
| GET | `/api/v1/node-servers/:id/services/:serviceId/crash-logs/:logId` | Administrateur | Détails du journal des incidents |
| GET | `/api/v1/node-servers/:id/services/:serviceId/endpoints` | Administrateur | Liste des points de terminaison |
| POST | `/api/v1/node-servers/:id/services/:serviceId/endpoints` | Administrateur | Créer un point de terminaison |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId/endpoints/:endpointId` | Administrateur | Supprimer le point de terminaison |
| PUT | `/api/v1/node-servers/:id/services/:serviceId/endpoints/:endpointId` | Administrateur | Mettre à jour le point de terminaison |
| PUT | `/api/v1/node-servers/:id/services/:serviceId/guardian` | Administrateur | Mettre à jour la configuration du superviseur |
| GET | `/api/v1/node-servers/:id/services/:serviceId/process-status` | Administrateur | Obtenir l'état du traitement |
| POST | `/api/v1/node-servers/:id/services/:serviceId/restart` | Administrateur | Redémarrer |
| POST | `/api/v1/node-servers/:id/services/:serviceId/start` | Administrateur | Commencer |
| POST | `/api/v1/node-servers/:id/services/:serviceId/stop` | Administrateur | Arrêter |
| GET | `/api/v1/node-servers/:id/services/process-status` | Administrateur | Tous les statuts de processus |
| GET | `/api/v1/node-servers/:id/share-domains` | Administrateur | Partager des domaines pour ce nœud |
| POST | `/api/v1/node-servers/:id/share-domains` | Administrateur | Créer |
| DELETE | `/api/v1/node-servers/:id/share-domains/:domainId` | Administrateur | Supprimer |
| PUT | `/api/v1/node-servers/:id/share-domains/:domainId` | Administrateur | Mise à jour |
| DELETE | `/api/v1/node-servers/:id/share-domains/all` | Administrateur | Tout effacer |
| POST | `/api/v1/node-servers/:id/singbox/control/firewall/apply` | Administrateur | Appliquer des règles de pare-feu de saut de port |
| POST | `/api/v1/node-servers/:id/singbox/control/firewall/remove` | Administrateur | Supprimer les règles de pare-feu de saut de port |
| GET | `/api/v1/node-servers/:id/singbox/connections` | Administrateur | Connexions sing-box en direct (clash_api, nœud local uniquement) |
| POST | `/api/v1/node-servers/:id/speed-test` | Administrateur | Test de vitesse des nœuds |
| GET | `/api/v1/node-servers/:id/status` | Administrateur | Statut du nœud |
| POST | `/api/v1/node-servers/:id/sync` | Administrateur | Synchroniser la configuration du nœud (générer et pousser) |
| POST | `/api/v1/node-servers/:id/standby-sync` | Administrateur | Réconcilier la configuration principale avec son partenaire de secours (primary_backup) |
| POST | `/api/v1/node-servers/:id/sync-data` | Super administrateur | Transférer les données du panneau vers les nœuds (synchronisation complète des utilisateurs/abonnements/plans) |
| POST | `/api/v1/node-servers/:id/sync-mirror` | Administrateur | Nœud miroir de synchronisation |
| GET | `/api/v1/node-servers/:id/tags` | Administrateur | Liste des balises |
| POST | `/api/v1/node-servers/:id/tags` | Administrateur | Ajouter une balise |
| DELETE | `/api/v1/node-servers/:id/tags/:tagKey` | Administrateur | Supprimer la balise |
| GET | `/api/v1/node-servers/:id/traffic-history` | Administrateur | Historique du trafic |
| POST | `/api/v1/node-servers/:id/traffic-resume` | Administrateur | Restaurer le trafic suspendu pour dépassement |
| GET | `/api/v1/node-servers/:id/traffic-usage` | Administrateur | Utilisation du trafic |
| POST | `/api/v1/node-servers/:id/update-agent` | Administrateur | Agent de mise à niveau |
| GET | `/api/v1/node-servers/:id/used-server-names` | Administrateur | Liste REALITY SNI en cours d'utilisation |
| GET | `/api/v1/node-servers/:id/waf/config` | Administrateur | Lire la configuration |
| PUT | `/api/v1/node-servers/:id/waf/config` | Administrateur | Enregistrer la configuration |
| POST | `/api/v1/node-servers/:id/waf/disable-domain` | Administrateur | Désactiver la protection du domaine |
| GET | `/api/v1/node-servers/:id/waf/domain-config` | Administrateur | Liste de configuration de domaine |
| PUT | `/api/v1/node-servers/:id/waf/domain-config` | Administrateur | Sauvegarder en masse les configurations de domaine |
| DELETE | `/api/v1/node-servers/:id/waf/domain-config/:domain` | Administrateur | Supprimer la configuration du domaine |
| GET | `/api/v1/node-servers/:id/waf/domain-config/:domain` | Administrateur | Détails de la configuration du domaine |
| PUT | `/api/v1/node-servers/:id/waf/domain-config/:domain` | Administrateur | Enregistrer la configuration du domaine |
| GET | `/api/v1/node-servers/:id/waf/domain-rules` | Administrateur | Répertoire des règles de domaine |
| GET | `/api/v1/node-servers/:id/waf/domain-rules/:domain/:name` | Administrateur | Lire le fichier de règles de domaine |
| PUT | `/api/v1/node-servers/:id/waf/domain-rules/:domain/:name` | Administrateur | Enregistrer le fichier de règles de domaine |
| GET | `/api/v1/node-servers/:id/waf/domain-status` | Administrateur | Statut de protection du domaine |
| POST | `/api/v1/node-servers/:id/waf/enable-domain` | Administrateur | Activer la protection du domaine |
| POST | `/api/v1/node-servers/:id/waf/push` | Administrateur | Pousser vers ce nœud |
| GET | `/api/v1/node-servers/:id/waf/rules` | Administrateur | Liste de règles du nœud WAF |
| DELETE | `/api/v1/node-servers/:id/waf/rules/:name` | Administrateur | Supprimer les règles du nœud WAF |
| GET | `/api/v1/node-servers/:id/waf/rules/:name` | Administrateur | Détails de la règle du nœud WAF |
| PUT | `/api/v1/node-servers/:id/waf/rules/:name` | Administrateur | Enregistrer les règles du nœud WAF |
| GET | `/api/v1/node-servers/:id/waf/status` | Administrateur | Statut du nœud WAF |
| PUT | `/api/v1/node-servers/:id/waf/status` | Administrateur | Basculer le nœud WAF |
| POST | `/api/v1/node-servers/:id/waf/sync` | Administrateur | Synchroniser avec ce nœud |
| GET | `/api/v1/node-servers/:id/warp` | Administrateur | Liste |
| DELETE | `/api/v1/node-servers/:id/warp/:warpId` | Administrateur | Supprimer |
| GET | `/api/v1/node-servers/:id/warp/config` | Administrateur | Lire la configuration |
| POST | `/api/v1/node-servers/:id/warp/generate-config` | Administrateur | Générer la configuration |
| POST | `/api/v1/node-servers/:id/warp/register` | Administrateur | S'inscrire |
| POST | `/api/v1/node-servers/:id/warp/masque/register` | Administrateur | Enregistrez le profil WARP MASQUE (étape 1 sur 2) |
| POST | `/api/v1/node-servers/:id/warp/:warpId/masque-outbound` | Administrateur | Créer un sing-box MASQUE sortant à partir d'un profil WARP |
| GET | `/api/v1/node-servers/:id/warp/status` | Administrateur | Statut |
| GET | `/api/v1/node-servers/:id/xray/control/balancer/:tag` | Administrateur | Obtenir des informations sur l'équilibreur |
| POST | `/api/v1/node-servers/:id/xray/control/balancer/:tag` | Administrateur | Définir la cible de l'équilibreur |
| POST | `/api/v1/node-servers/:id/xray/control/firewall/apply` | Administrateur | Appliquer des règles de pare-feu de saut de port |
| POST | `/api/v1/node-servers/:id/xray/control/firewall/remove` | Administrateur | Supprimer les règles de pare-feu de saut de port |
| POST | `/api/v1/node-servers/:id/xray/control/inbound` | Administrateur | Ajouter un message entrant |
| DELETE | `/api/v1/node-servers/:id/xray/control/inbound/:tag` | Administrateur | Supprimer les appels entrants |
| POST | `/api/v1/node-servers/:id/xray/control/inbound/:tag/user` | Administrateur | Ajouter un utilisateur |
| DELETE | `/api/v1/node-servers/:id/xray/control/inbound/:tag/user/:email` | Administrateur | Supprimer un utilisateur |
| POST | `/api/v1/node-servers/:id/xray/control/outbound` | Administrateur | Ajouter un message sortant |
| DELETE | `/api/v1/node-servers/:id/xray/control/outbound/:tag` | Administrateur | Supprimer les appels sortants |
| POST | `/api/v1/node-servers/:id/xray/control/routing` | Administrateur | Demander un itinéraire |
| POST | `/api/v1/node-servers/:id/xray/control/routing/rule` | Administrateur | Ajouter une règle de routage |
| DELETE | `/api/v1/node-servers/:id/xray/control/routing/rule/:tag` | Administrateur | Supprimer la règle de routage |
| GET | `/api/v1/node-servers/:id/xray/control/routing/rules` | Administrateur | Répertorier les règles de routage |
| POST | `/api/v1/node-servers/:id/xray/control/sync` | Administrateur | Déclenchez manuellement la synchronisation de la configuration xray |
| GET | `/api/v1/node-servers/:id/xray/stats` | Administrateur | Obtenir des statistiques |
| POST | `/api/v1/node-servers/:id/xray/stats/reset/:email` | Administrateur | Réinitialiser les statistiques des utilisateurs |
| GET | `/api/v1/node-servers/:id/xray/stats/users` | Administrateur | Obtenir des statistiques sur les utilisateurs |
| GET | `/api/v1/node-servers/billable` | Administrateur | Liste des nœuds facturables |
| GET | `/api/v1/node-servers/ca-cert` | Administrateur | Certificat d'autorité de certification |
| GET | `/api/v1/node-servers/ca-status` | Administrateur | Statut de l'autorité de certification |
| GET | `/api/v1/node-servers/dashboard` | Administrateur | Statut du tableau de bord |
| POST | `/api/v1/node-servers/distribute-cert` | Administrateur | Distribuer le certificat à tous les nœuds |
| POST | `/api/v1/node-servers/failover-check` | Administrateur | Déclencher la vérification du basculement |
| GET | `/api/v1/node-servers/metrics/aggregated` | Administrateur | Métriques agrégées |
| POST | `/api/v1/node-servers/migrate-cert` | Administrateur | Migrer le certificat |
| GET | `/api/v1/node-servers/online-summary` | Administrateur | Résumé en ligne |
| GET | `/api/v1/node-servers/online-users` | Administrateur | Tous les utilisateurs en ligne |
| GET | `/api/v1/node-servers/pending-agent-updates` | Administrateur | Agents en attente de mise à niveau |
| POST | `/api/v1/node-servers/recommend-region` | Administrateur | Régions recommandées |
| POST | `/api/v1/node-servers/select-nodes` | Administrateur | Sélectionner les nœuds pour l'abonnement |
| POST | `/api/v1/node-servers/speed-test-all` | Administrateur | Test de vitesse de tous les nœuds |
| GET | `/api/v1/node-servers/speed-test-results` | Administrateur | Résultats des tests de vitesse |
| GET | `/api/v1/node-servers/traffic/stats` | Administrateur | Statistiques de trafic |
| POST | `/api/v1/node-servers/update-all-agents` | Administrateur | Mettre à niveau tous les agents |
| DELETE | `/api/v1/node-servers/users/:client_id` | Administrateur | Synchroniser-supprimer un client |
| GET | `/api/v1/node-servers/users/retry-queue` | Administrateur | État de la file d'attente des nouvelles tentatives |
| POST | `/api/v1/node-servers/users/retry-queue/process` | Administrateur | File d’attente des nouvelles tentatives de traitement |
| POST | `/api/v1/node-servers/users/sync` | Administrateur | Synchronisez un seul client |
| POST | `/api/v1/node-servers/validate-chain-config` | Administrateur | Valider la configuration chaînée |

### Journal d'audit du nœud

Audit et exportation des opérations côté nœud.

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| GET | `/api/v1/node-audit-logs` | Administrateur | Journal d'audit du nœud |
| GET | `/api/v1/node-audit-logs/export` | Administrateur | Exporter les journaux d'audit des nœuds |

### Nœuds (ancien modèle, retiré)

> `/api/v1/nodes` CRUD a été retiré le 20/10/2026 : les nœuds sont gérés uniformément via `/api/v1/node-servers`.
> Les requêtes des utilisateurs en ligne restent sur `/api/v1/node-servers/{online-users,online-summary,:id/online-users}`.

### Aperçu de la région

Vue des nœuds agrégée par région.

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| GET | `/api/v1/regions/:region/servers` | Connecté | Liste de nœuds pour cette région |
| GET | `/api/v1/regions/overview` | Connecté | Aperçu de la région |

### Abonnements

Gestion des enregistrements d'abonnement : révocation, rotation du jeton, journaux d'accès, réinitialisation de l'utilisation.

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| GET | `/api/v1/subscriptions` | Connecté | Liste |
| POST | `/api/v1/subscriptions` | Opérateur | Créer |
| DELETE | `/api/v1/subscriptions/:id` | Opérateur | Supprimer |
| GET | `/api/v1/subscriptions/:id` | Connecté | Détails |
| PUT | `/api/v1/subscriptions/:id` | Opérateur | Mise à jour |
| GET | `/api/v1/subscriptions/:id/access-logs` | Connecté | Journal d'accès |
| POST | `/api/v1/subscriptions/:id/enable` | Opérateur | Activer |
| POST | `/api/v1/subscriptions/:id/reset-usage` | Opérateur | Réinitialiser l'utilisation |
| POST | `/api/v1/subscriptions/:id/restore` | Opérateur | CV |
| POST | `/api/v1/subscriptions/:id/revoke` | Opérateur | Révoquer |
| POST | `/api/v1/subscriptions/:id/rotate` | Opérateur | Rotation |
| GET | `/api/v1/subscriptions/:id/rotations` | Connecté | Historique des rotations |

### Formats d'abonnement

> Le CRUD `/api/v1/subscription-formats` côté administrateur a été retiré le 20/10/2026 : le point d'entrée réel est
> `/api/v1/mihomo/configs` (centre de format d'abonnement). La table `subscription_formats` reste,
> utilisé uniquement comme données de départ pour le point de terminaison public `/api/v1/public/client-configs`.

### Points de terminaison d'abonnement

Domaines d'abonnement dédiés (proxy inverse au chemin `/sub` du panneau).

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| GET | `/api/v1/subscription-endpoints` | Connecté | Liste |
| POST | `/api/v1/subscription-endpoints` | Administrateur | Créer |
| DELETE | `/api/v1/subscription-endpoints/:id` | Administrateur | Supprimer |
| GET | `/api/v1/subscription-endpoints/:id` | Connecté | Détails |
| PUT | `/api/v1/subscription-endpoints/:id` | Administrateur | Mise à jour |

### Partager des domaines

Accédez au domaine/pool IP utilisé dans les liens clients.

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| GET | `/api/v1/share-domains` | Connecté | Liste |
| POST | `/api/v1/share-domains` | Administrateur | Créer |
| DELETE | `/api/v1/share-domains/:id` | Administrateur | Supprimer |
| PUT | `/api/v1/share-domains/:id` | Administrateur | Mise à jour |
| DELETE | `/api/v1/share-domains/all` | Administrateur | Tout effacer |
| POST | `/api/v1/share-domains/seed` | Administrateur | Initialiser les données par défaut |
| GET | `/api/v1/share-domains/dns-providers` | Connecté | Tableau des capacités d'écriture DNS par fournisseur (implémenté/vérifié) |
`PUT /api/v1/share-domains/:id` accepte également `dns_channel_id`, `dns_record_name` et `dns_proxied` : lors d'un basculement ou d'un basculement principal/de secours, l'enregistrement d'entrée A pointe vers le nouveau principal. Seuls les fournisseurs disposant d'un enregistreur DNS implémenté acceptent ces champs (`cloudflare` vérifié ; alidns / DNSPod / GoDaddy / Namecheap / Route53 / Google / Huawei Cloud implémenté mais non vérifié). Voir `GET /api/v1/share-domains/dns-providers` pour le tableau des capacités.


### CloudflareIP

Cloudflare CDN Gestion du pool IP et des sources.

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| DELETE | `/api/v1/cfip/entries` | Administrateur | Effacer les entrées |
| GET | `/api/v1/cfip/entries` | Connecté | Liste des inscrits |
| POST | `/api/v1/cfip/entries` | Administrateur | Ajouter une entrée |
| DELETE | `/api/v1/cfip/entries/:id` | Administrateur | Supprimer l'entrée |
| POST | `/api/v1/cfip/fetch-all` | Administrateur | Récupérer toutes les sources |
| GET | `/api/v1/cfip/probe-config` | Connecté | Configuration de la sonde IP préférée CF (nœuds d'entrée + numéroteurs installés) |
| PUT | `/api/v1/cfip/probe-config` | Administrateur | Enregistrer l'intervalle de sonde / la limite de latence / le nœud d'entrée / le numéroteur |
| POST | `/api/v1/cfip/check` | Administrateur | Sondez tous les domaines/IP préférés maintenant |
| POST | `/api/v1/cfip/seed` | Administrateur | Initialiser les données par défaut |
| GET | `/api/v1/cfip/sources` | Connecté | Liste des sources de données |
| POST | `/api/v1/cfip/sources` | Administrateur | Créer une source de données |
| DELETE | `/api/v1/cfip/sources/:id` | Administrateur | Supprimer la source de données |
| PUT | `/api/v1/cfip/sources/:id` | Administrateur | Mettre à jour la source de données |
| POST | `/api/v1/cfip/sources/:id/fetch` | Administrateur | Récupérer cette source |

### Configuration de l'abonnement (Mihomo/Clash)

Configuration du modèle d'abonnement : groupes, règles, fournisseurs de règles, aperçu YAML.

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| GET | `/api/v1/mihomo/configs` | Connecté | Liste |
| POST | `/api/v1/mihomo/configs` | Administrateur | Créer |
| DELETE | `/api/v1/mihomo/configs/:id` | Administrateur | Supprimer |
| GET | `/api/v1/mihomo/configs/:id` | Connecté | Détails |
| PUT | `/api/v1/mihomo/configs/:id` | Administrateur | Mise à jour |
| GET | `/api/v1/mihomo/configs/:id/rules` | Connecté | Liste de règles |
| POST | `/api/v1/mihomo/configs/:id/rules` | Administrateur | Créer une règle |
| PUT | `/api/v1/mihomo/configs/:id/rules/order` | Administrateur | Règles de réorganisation |
| PUT | `/api/v1/mihomo/rules/:id` | Administrateur | Mettre à jour la règle |
| DELETE | `/api/v1/mihomo/rules/:id` | Administrateur | Supprimer la règle |

### Sites de recharge

Intégration de sites de paiement tiers : clés de signature, fichiers de vérification, vérification de domaine, documents d'intégration.

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| GET | `/api/v1/charge-sites` | Administrateur | Liste des sites |
| POST | `/api/v1/charge-sites` | Administrateur | Créer un site |
| DELETE | `/api/v1/charge-sites/:id` | Administrateur | Supprimer le site |
| GET | `/api/v1/charge-sites/:id` | Administrateur | Détails du site |
| PUT | `/api/v1/charge-sites/:id` | Administrateur | Mettre à jour le site |
| POST | `/api/v1/charge-sites/:id/clear-prev-secret` | Administrateur | Effacer l'ancienne clé |
| POST | `/api/v1/charge-sites/:id/generate-verify` | Administrateur | Générer un fichier de vérification |
| POST | `/api/v1/charge-sites/:id/generate-verify-file` | Administrateur | Générer un fichier de vérification |
| GET | `/api/v1/charge-sites/:id/integration-doc` | Administrateur | Obtenir le document d'intégration |
| POST | `/api/v1/charge-sites/:id/rotate-secret` | Administrateur | Rotation de la clé de signature |
| POST | `/api/v1/charge-sites/:id/verify-dns` | Administrateur | Vérifier le domaine DNS |
| POST | `/api/v1/charge-sites/:id/verify-domain` | Administrateur | Vérifier le défi HTTP du domaine |
| POST | `/api/v1/charge-sites/:id/verify-domain-dns` | Administrateur | Vérifier le domaine DNS |
| POST | `/api/v1/charge-sites/:id/verify-domain-http` | Administrateur | Vérifier le défi HTTP du domaine |

### Recharger les commandes

Rechargez les demandes de commande, les remboursements, l'export CSV.

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| GET | `/api/v1/charge-orders` | Administrateur | Liste de commandes |
| GET | `/api/v1/charge-orders/:id` | Administrateur | Détails de la commande |
| POST | `/api/v1/charge-orders/:id/refund` | Administrateur | Remboursement |
| GET | `/api/v1/charge-orders/export-csv` | Administrateur | Exporter les commandes de recharge CSV |

### Recharger les cartes

Génération de cartes en masse, enregistrements d'utilisation, exportation CSV.

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| GET | `/api/v1/charge-cards` | Administrateur | Liste des cartes |
| POST | `/api/v1/charge-cards` | Administrateur | Créer une carte |
| DELETE | `/api/v1/charge-cards/:id` | Administrateur | Supprimer la carte |
| GET | `/api/v1/charge-cards/:id/usages` | Administrateur | Enregistrements d'utilisation de la carte |
| POST | `/api/v1/charge-cards/batch` | Administrateur | Générer des cartes en masse |
| GET | `/api/v1/charge-cards/export-csv` | Administrateur | Cartes d'exportation CSV |

### Événements de rappel de facturation

Audit des événements de rappel.

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| GET | `/api/v1/charge-callback-events` | Connecté | Liste des événements de rappel |

### Facturer les tentatives de rappel

Nouvelle tentative manuelle des rappels ayant échoué.

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| GET | `/api/v1/charge-callback-retries` | Connecté | Liste de rappel ayant échoué |
| POST | `/api/v1/charge-callback-retries/:id/retry` | Administrateur | Réessayer le rappel |

### Solde client

Vue administrative des soldes et des transactions.

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| GET | `/api/v1/client-balances` | Connecté | Liste des soldes clients |
| GET | `/api/v1/client-balances/transactions` | Connecté | Transactions de solde (administrateur) |
| GET | `/api/v1/client-balances/transactions/export-csv` | Connecté | Opérations du solde d'exportation CSV |

### Cadeaux

Émettez des cadeaux récupérables (jours/trafic).

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| GET | `/api/v1/client-gifts` | Connecté | Liste de cadeaux |
| POST | `/api/v1/client-gifts` | Administrateur | Créer un cadeau |
| POST | `/api/v1/client-gifts/batch` | Administrateur | Créer des cadeaux en masse |
| GET | `/api/v1/client-gifts/search-clients` | Connecté | Rechercher des clients |

### Configuration de référence

Configuration et statistiques des commissions de référencement.

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| GET | `/api/v1/referral/config` | Connecté | Configuration de référence |
| PUT | `/api/v1/referral/config` | Administrateur | Enregistrer la configuration du parrainage |
| GET | `/api/v1/referral/records` | Connecté | Liste des dossiers de référence |
| GET | `/api/v1/referral/stats` | Connecté | Statistiques de référencement |

### Retraits de référence

Examinez les demandes de retrait.

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| GET | `/api/v1/referral-withdrawals` | Administrateur | Liste des demandes de retrait |
| POST | `/api/v1/referral-withdrawals/:id/approve` | Administrateur | Approuver le retrait |
| POST | `/api/v1/referral-withdrawals/:id/reject` | Administrateur | Refuser le retrait |

### Listes d'autorisation IP (résumé)

Affichage de la liste blanche IP pour tous les clients.

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| GET | `/api/v1/ip-whitelists` | Connecté | Listes autorisées d'adresses IP de tous les clients |

### Liaisons OAuth du client

Interrogez les liaisons de compte tiers du client.

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| GET | `/api/v1/client-oauth-bindings` | Connecté | Liste de liaisons OAuth du client |

### Billets (administrateur)

Réponse, affectation et clôture du ticket.

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| GET | `/api/v1/tickets` | Connecté | Liste des billets |
| GET | `/api/v1/tickets/:id` | Connecté | Détails du billet |
| POST | `/api/v1/tickets/:id/assign` | Opérateur | Attribuer un ticket |
| POST | `/api/v1/tickets/:id/close` | Opérateur | Fermer le ticket |
| POST | `/api/v1/tickets/:id/reply` | Opérateur | Répondre au ticket |

### Articles

CRUD pour les articles du portail.

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| GET | `/api/v1/articles` | Connecté | Liste des articles |
| POST | `/api/v1/articles` | Administrateur | Créer un article |
| DELETE | `/api/v1/articles/:id` | Administrateur | Supprimer l'article |
| GET | `/api/v1/articles/:id` | Connecté | Détails de l'article |
| PUT | `/api/v1/articles/:id` | Administrateur | Mettre à jour l'article |

### Comptes partagés

Dossiers d'émission et de réparation des comptes partagés (streaming, etc.).

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| GET | `/api/v1/shared-accounts` | Connecté | Liste de comptes partagés |
| POST | `/api/v1/shared-accounts` | Administrateur | Créer un compte partagé |
| DELETE | `/api/v1/shared-accounts/:id` | Administrateur | Supprimer le compte partagé |
| GET | `/api/v1/shared-accounts/:id` | Connecté | Détails du compte partagé |
| PUT | `/api/v1/shared-accounts/:id` | Administrateur | Mettre à jour le compte partagé |
| GET | `/api/v1/shared-accounts/:id/repair-logs` | Connecté | Dossiers de réparation |

### Certificats

Émission, téléchargement, renouvellement et exportation de certificat public (ACME) ; l'émission sur un nœud spécifique est prise en charge.

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| GET | `/api/v1/certificates` | Administrateur | Liste |
| POST | `/api/v1/certificates` | Administrateur | Créer |
| DELETE | `/api/v1/certificates/:id` | Administrateur | Supprimer |
| GET | `/api/v1/certificates/:id` | Administrateur | Détails |
| PUT | `/api/v1/certificates/:id` | Administrateur | Mise à jour |
| GET | `/api/v1/certificates/:id/download` | Administrateur | Télécharger |
| POST | `/api/v1/certificates/:id/renew` | Administrateur | Renouveler le client |
| GET | `/api/v1/certificates/check-port` | Administrateur | Vérifier le port |
| GET | `/api/v1/certificates/detect-ip` | Administrateur | IP de la sonde |
| GET | `/api/v1/certificates/export` | Administrateur | Exporter |
| POST | `/api/v1/certificates/issue` | Administrateur | Problème. corps : `{"domains":["example.com","*.example.com"],"challenge_type":"dns-01","dns_channel_id":2,"key_type":"rsa","key_bits":4096}`. `challenge_type` est par défaut http-01 ; `dns_channel_id` sélectionne un canal DNS (un fournisseur DNS configuré + informations d'identification) |
| POST | `/api/v1/certificates/issue-on-node` | Administrateur | Problème vers le nœud |
| POST | `/api/v1/certificates/upload` | Administrateur | Télécharger |
| GET | `/api/v1/certificate-tasks/:id` | Administrateur | Interroger l'état de la tâche en arrière-plan |

### Canaux de certificat

Configuration du canal ACME (CA).

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| GET | `/api/v1/cert-channels` | Administrateur | Liste |
| POST | `/api/v1/cert-channels` | Administrateur | Créer |
| DELETE | `/api/v1/cert-channels/:id` | Administrateur | Supprimer |
| PUT | `/api/v1/cert-channels/:id` | Administrateur | Mise à jour |
| GET | `/api/v1/cert-channels/providers` | Administrateur | Fournisseur |

### Configuration EAB

Identifiants liant des comptes externes tels que ZeroSSL / Google.

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| GET | `/api/v1/eab-configs` | Administrateur | Liste |
| POST | `/api/v1/eab-configs` | Administrateur | Créer |
| DELETE | `/api/v1/eab-configs/:id` | Administrateur | Supprimer |
| PUT | `/api/v1/eab-configs/:id` | Administrateur | Mise à jour |
| POST | `/api/v1/eab-configs/fetch-google` | Administrateur | Récupérer Google EAB depuis GCP |
| POST | `/api/v1/eab-configs/fetch-zerossl` | Administrateur | Récupérez le ZeroSSL EAB avec un e-mail en un clic |

### Comptes ACME

Gestion du compte ACME (e-mail de contact) côté panneau.

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| GET | `/api/v1/acme-accounts` | Administrateur | Liste |
| POST | `/api/v1/acme-accounts` | Administrateur | Créer |
| DELETE | `/api/v1/acme-accounts/:id` | Administrateur | Supprimer |
| PUT | `/api/v1/acme-accounts/:id` | Administrateur | Mise à jour |
| GET | `/api/v1/acme-accounts/default` | Administrateur | Obtenir les valeurs par défaut |

### Autorité de certification intégrée

Panel interne CA : émission/renouvellement/révocation de certificat d'agent, export CRL, file d'attente de distribution.

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| GET | `/api/v1/ca/agent-certs` | Administrateur | Liste des certificats d'agent |
| POST | `/api/v1/ca/agent-certs/:server_id/renew` | Administrateur | Renouveler le certificat d'agent |
| GET | `/api/v1/ca/distribution-queue` | Administrateur | File d'attente de distribution de certificat |
| DELETE | `/api/v1/ca/distribution-queue/:id` | Administrateur | Supprimer l'entrée de la file d'attente de distribution |
| POST | `/api/v1/ca/distribution-queue/:id/retry` | Administrateur | Réessayer la distribution du certificat |
| GET | `/api/v1/ca/revocations` | Administrateur | Liste de révocation |
| GET | `/api/v1/ca/revocations/export` | Administrateur | Exporter la liste de révocation de certificats |
| GET | `/api/v1/ca/status` | Administrateur | Statut de l'autorité de certification |

### Règles géographiques

Synchronisation et livraison des sources et fichiers de données géoip/géosite.

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| GET | `/api/v1/geo-rules/files` | Connecté | Liste de fichiers |
| DELETE | `/api/v1/geo-rules/files/:fileId` | Administrateur | Supprimer le fichier |
| PUT | `/api/v1/geo-rules/files/:fileId/auto-sync` | Administrateur | Activer la synchronisation automatique |
| POST | `/api/v1/geo-rules/files/:fileId/download` | Administrateur | Télécharger |
| GET | `/api/v1/geo-rules/files/:fileId/download-file` | Connecté | Télécharger le fichier |
| GET | `/api/v1/geo-rules/github-assets` | Connecté | Récupérer les actifs du référentiel GitHub |
| POST | `/api/v1/geo-rules/push-all` | Administrateur | Pousser vers tous les nœuds |
| POST | `/api/v1/geo-rules/seed` | Administrateur | Initialiser les données par défaut |
| GET | `/api/v1/geo-rules/sources` | Connecté | Liste des sources de données |
| POST | `/api/v1/geo-rules/sources` | Administrateur | Créer une source de données |
| DELETE | `/api/v1/geo-rules/sources/:sourceId` | Administrateur | Supprimer la source de données |
| PUT | `/api/v1/geo-rules/sources/:sourceId` | Administrateur | Mettre à jour la source de données |
| GET | `/api/v1/geo-rules/sources/:sourceId/assets` | Connecté | Récupérer les ressources de version |
| POST | `/api/v1/geo-rules/sources/:sourceId/files` | Administrateur | Ajouter un fichier |
| PUT | `/api/v1/geo-rules/sources/:sourceId/schedule` | Administrateur | Activer/désactiver la synchronisation programmée |
| POST | `/api/v1/geo-rules/sources/:sourceId/select` | Administrateur | Sélectionner un fichier |
| POST | `/api/v1/geo-rules/sources/:sourceId/sync` | Administrateur | Synchroniser cette source de données |
| GET | `/api/v1/geo-rules/status` | Connecté | Statut |
| POST | `/api/v1/geo-rules/sync-all` | Administrateur | Synchroniser tout |
| GET | `/api/v1/geo-rules/sync-status/:key` | Connecté | Progression de la synchronisation |

### Bibliothèque de règles WAF

Règles globales WAF : source GitHub, synchronisation, livraison à tous les nœuds.

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| GET | `/api/v1/waf/github-source` | Connecté | Source de données GitHub |
| PUT | `/api/v1/waf/github-source` | Administrateur | Enregistrer la source de données GitHub |
| POST | `/api/v1/waf/push-all` | Administrateur | Pousser vers tous les nœuds |
| GET | `/api/v1/waf/rules` | Connecté | Liste de règles |
| DELETE | `/api/v1/waf/rules/:name` | Administrateur | Supprimer la règle |
| GET | `/api/v1/waf/rules/:name` | Connecté | Obtenir la règle |
| PUT | `/api/v1/waf/rules/:name` | Administrateur | Enregistrer la règle |
| POST | `/api/v1/waf/sync-all` | Administrateur | Synchroniser tout |
| POST | `/api/v1/waf/sync-github` | Administrateur | Synchronisation depuis GitHub |

### Gestion des journaux

Lecture et diffusion en continu des journaux de nœuds/cœurs, politiques de nettoyage, contrôle de l'auteur de journaux.

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| POST | `/api/v1/log-manage/cleanup-all` | Administrateur | Nettoyez tout maintenant |
| GET | `/api/v1/log-manage/cleanup-rules` | Administrateur | Liste des règles de nettoyage |
| PUT | `/api/v1/log-manage/cleanup-rules/:id` | Administrateur | Mettre à jour la règle de nettoyage |
| POST | `/api/v1/log-manage/cleanup-table/:table` | Administrateur | Nettoyer la table spécifiée maintenant |
| GET | `/api/v1/log-manage/core-log-output` | Administrateur | Configuration de sortie du journal principal |
| PUT | `/api/v1/log-manage/core-log-output` | Administrateur | Enregistrer la configuration de sortie du journal principal |
| GET | `/api/v1/log-manage/core/:type/stdout-log` | Administrateur | Lire les journaux de base de la sortie standard |
| GET | `/api/v1/log-manage/core/:type/stdout-stream` | Administrateur | Sortie standard du flux principal |
| POST | `/api/v1/log-manage/discover` | Administrateur | Découvrir les fichiers journaux des nœuds |
| GET | `/api/v1/log-manage/log-output` | Administrateur | Configuration de la sortie du journal |
| PUT | `/api/v1/log-manage/log-output` | Administrateur | Enregistrer la configuration de sortie du journal |
| GET | `/api/v1/log-manage/nodes/:serverId/logs` | Administrateur | Liste des fichiers journaux des nœuds |
| GET | `/api/v1/log-manage/nodes/:serverId/logs/:type` | Administrateur | Lire les journaux des nœuds |
| POST | `/api/v1/log-manage/nodes/:serverId/logs/:type/rotate` | Administrateur | Rotation des journaux de nœuds |
| GET | `/api/v1/log-manage/nodes/:serverId/logs/:type/stream` | Administrateur | Journaux des nœuds de flux |
| GET | `/api/v1/log-manage/policies` | Administrateur | Liste des politiques |
| POST | `/api/v1/log-manage/policies` | Administrateur | Créer une politique |
| DELETE | `/api/v1/log-manage/policies/:id` | Administrateur | Supprimer la stratégie |
| GET | `/api/v1/log-manage/policies/:id` | Administrateur | Détails de la politique |
| PUT | `/api/v1/log-manage/policies/:id` | Administrateur | Politique de mise à jour |
| GET | `/api/v1/log-manage/policies/:id/jobs` | Administrateur | Liste des tâches de nettoyage |
| GET | `/api/v1/log-manage/policies/:id/log` | Administrateur | Lire les journaux |
| GET | `/api/v1/log-manage/policies/:id/log-stream` | Administrateur | Journaux de flux |
| POST | `/api/v1/log-manage/policies/:id/rotate` | Administrateur | Rotation |
| GET | `/api/v1/log-manage/proxy/:type/stdout-log` | Administrateur | Lire les journaux de base de la sortie standard |
| GET | `/api/v1/log-manage/proxy/:type/stdout-stream` | Administrateur | Sortie standard du flux principal |
| GET | `/api/v1/log-manage/scheduler-config` | Administrateur | Configuration de la planification |
| PUT | `/api/v1/log-manage/scheduler-config` | Administrateur | Mettre à jour la configuration de planification |
| GET | `/api/v1/log-manage/status` | Administrateur | Statut |
| POST | `/api/v1/log-manage/sync` | Administrateur | Politiques de nettoyage de synchronisation |
| GET | `/api/v1/log-manage/writers` | Administrateur | Statut d'écrivain |

### Sauvegardes

Base de données/sauvegardes complètes, restauration, tâches planifiées, stockage à distance FTP/S3/WebDAV.

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| DELETE | `/api/v1/backups/:id` | Super administrateur | Supprimer la sauvegarde |
| GET | `/api/v1/backups/:id/download` | Administrateur | Télécharger la sauvegarde |
| POST | `/api/v1/backups/:id/restore` | Super administrateur | Restaurer à partir d'une sauvegarde |
| POST | `/api/v1/backups/:id/upload-ftp` | Super administrateur | Télécharger sur FTP |
| POST | `/api/v1/backups/:id/upload-s3` | Super administrateur | Télécharger sur S3 |
| POST | `/api/v1/backups/:id/upload-webdav` | Super administrateur | Télécharger sur WebDAV |
| POST | `/api/v1/backups/create` | Administrateur | Créer une sauvegarde complète (avec dump DB) |
| GET | `/api/v1/backups/list` | Administrateur | Liste de sauvegarde complète |
| POST | `/api/v1/backups/remote-restore` | Super administrateur | Restaurer à distance |
| GET | `/api/v1/backups/restore-status` | Super administrateur | Restaurer la progression |
| GET | `/api/v1/backups/schedules` | Administrateur | Liste des tâches planifiées |
| POST | `/api/v1/backups/schedules` | Administrateur | Créer une tâche planifiée |
| DELETE | `/api/v1/backups/schedules/:id` | Administrateur | Supprimer le travail planifié |
| PUT | `/api/v1/backups/schedules/:id` | Administrateur | Mettre à jour la tâche planifiée |
| GET | `/api/v1/backups/storages` | Administrateur | Liste de configuration de stockage |
| POST | `/api/v1/backups/storages` | Administrateur | Créer une configuration de stockage |
| DELETE | `/api/v1/backups/storages/:id` | Administrateur | Supprimer la configuration de stockage |
| PUT | `/api/v1/backups/storages/:id` | Administrateur | Mettre à jour la configuration du stockage |
| GET | `/api/v1/backups/storages/:id/files` | Administrateur | Liste des fichiers de sauvegarde à distance |
| POST | `/api/v1/backups/upload-restore` | Super administrateur | Télécharger la sauvegarde et la restauration |

### Paramètres système

Configuration et migration de la base de données, paramètres du panneau, clés JWT, tests SMTP/Telegram, modèles de notification, mises à niveau en ligne (principalement admin).

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| PUT | `/api/v1/system/bootstrap-finish` | Administrateur | Terminer la configuration au premier démarrage |
| GET | `/api/v1/system/bootstrap-state` | Administrateur | État du premier démarrage |
| GET | `/api/v1/system/config-file/:coreType` | Administrateur | Lire le fichier de configuration de base |
| GET | `/api/v1/system/db-config` | Administrateur | Configuration de la base de données |
| PUT | `/api/v1/system/db-config` | Administrateur | Mettre à jour la configuration de la base de données |
| POST | `/api/v1/system/db-config/migrate` | Super administrateur | Migrer les données de la base de données |
| POST | `/api/v1/system/db-config/switch` | Super administrateur | Changer de base de données (avec migration en une étape) |
| POST | `/api/v1/system/db-config/test` | Administrateur | Tester la connexion à la base de données |
| GET | `/api/v1/system/db-pool-stats` | Administrateur | Statistiques du pool de connexions |
| GET | `/api/v1/system/db-stats` | Administrateur | Statistiques de la base de données |
| GET | `/api/v1/system/db-stats/ping` | Administrateur | Sonde de base de données |
| POST | `/api/v1/system/db-stats/reset` | Administrateur | Réinitialiser les statistiques de la base de données |
| GET | `/api/v1/system/encryption-key-status` | Administrateur | État de la clé de chiffrement |
| GET | `/api/v1/system/env-config` | Administrateur | Configuration .env |
| PUT | `/api/v1/system/env-config` | Administrateur | Mettre à jour la configuration .env |
| POST | `/api/v1/system/generate-encryption-key` | Super administrateur | Générer une clé de chiffrement |
| GET | `/api/v1/system/github-config` | Administrateur | Configuration GitHub |
| PUT | `/api/v1/system/github-config` | Administrateur | Mettre à jour la configuration du GitHub |
| GET | `/api/v1/system/jwt-secret-status` | Administrateur | État de la clé JWT |
| GET | `/api/v1/system/maintenance` | Publique | Statut du mode maintenance (public) |
| PUT | `/api/v1/system/maintenance` | Administrateur | Basculer en mode maintenance |
| POST | `/api/v1/system/migrate-data` | Super administrateur | Migrer les données |
| GET | `/api/v1/system/notification-config` | Administrateur | Configuration des notifications |
| PUT | `/api/v1/system/notification-config` | Administrateur | Mettre à jour la configuration des notifications |
| GET | `/api/v1/system/notification-templates` | Administrateur | Liste des modèles |
| POST | `/api/v1/system/notification-templates` | Administrateur | Créer un modèle |
| DELETE | `/api/v1/system/notification-templates/:id` | Administrateur | Supprimer le modèle |
| PUT | `/api/v1/system/notification-templates/:id` | Administrateur | Mettre à jour le modèle |
| POST | `/api/v1/system/notification-templates/:id/broadcast` | Administrateur | Notification de diffusion par modèle |
| POST | `/api/v1/system/notification-templates/:id/send` | Administrateur | Envoyer une notification de test |
| GET | `/api/v1/system/panel-config` | Administrateur | Configuration du panneau |
| PUT | `/api/v1/system/panel-config` | Administrateur | Mettre à jour la configuration du panneau |
| GET | `/api/v1/system/reality-targets` | Administrateur | Liste des cibles REALITY |
| PUT | `/api/v1/system/reality-targets` | Administrateur | Mettre à jour la cible REALITY |
| POST | `/api/v1/system/reality-targets/reset` | Super administrateur | Réinitialiser la cible REALITY par défaut |
| GET | `/api/v1/system/reality-local-sni` | Administrateur | Lire la configuration locale du SNI |
| PUT | `/api/v1/system/reality-local-sni` | Administrateur | Mettre à jour la configuration locale du Reality SNI |
| POST | `/api/v1/system/regenerate-jwt-secret` | Super administrateur | Régénérer la clé JWT |
| GET | `/api/v1/system/releases` | Administrateur | Versions disponibles |
| POST | `/api/v1/system/restart` | Super administrateur | Redémarrer |
| POST | `/api/v1/system/rollback` | Super administrateur | Restauration du système |
| GET | `/api/v1/system/settings` | Administrateur | Lire les paramètres |
| PUT | `/api/v1/system/settings/batch` | Administrateur | Paramètres de mise à jour groupée |
| GET | `/api/v1/system/stats` | Administrateur | État du système |
| GET | `/api/v1/system/subscription-billing-templates` | Administrateur | Modèle de facturation d'abonnement |
| PUT | `/api/v1/system/subscription-billing-templates` | Administrateur | Mettre à jour le modèle de facturation des abonnements |
| POST | `/api/v1/system/telegram-chat-id` | Administrateur | Obtenir l'identifiant de discussion Telegram |
| POST | `/api/v1/system/test-smtp` | Administrateur | Test SMTP |
| POST | `/api/v1/system/test-telegram` | Administrateur | Test Telegram |
| POST | `/api/v1/system/update` | Super administrateur | Mise à niveau du système |
| GET | `/api/v1/system/update-jobs` | Administrateur | Liste des tâches de mise à niveau |
| POST | `/api/v1/system/upload-mtls-ca` | Administrateur | Télécharger le certificat mTLS CA |

### Audit de sécurité

Auditez les requêtes d’événements et exportez-les.

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| GET | `/api/v1/security/audit-events` | Administrateur | Liste des événements d'audit |
| GET | `/api/v1/security/audit-events/export` | Administrateur | Exporter les événements d'audit |
| GET | `/api/v1/security/subscription-access-logs` | Administrateur | Journaux de récupération d'abonnement (filtrer par ID d'abonnement/IP/e-mail client) |

### Métriques

Métriques agrégées du panel (nœuds/certificats/services).

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| GET | `/api/v1/metrics/certificates` | Administrateur | Métriques du certificat |
| GET | `/api/v1/metrics/nodes` | Administrateur | Métriques de nœud |
| GET | `/api/v1/metrics/overview` | Administrateur | Aperçu |

### Modèles de portail

Modèles de portail, configuration d'activation et gestion du cache.

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| GET | `/api/v1/portal/active` | Administrateur | Modèle actif |
| PUT | `/api/v1/portal/active` | Administrateur | Mettre à jour le modèle actif |
| DELETE | `/api/v1/portal/cache` | Administrateur | Vider le cache du portail |
| GET | `/api/v1/portal/cache` | Administrateur | État du cache |
| PUT | `/api/v1/portal/cache/dev-mode` | Administrateur | Basculer le mode de développement du cache |
| POST | `/api/v1/portal/cache/rebuild` | Administrateur | Reconstruire le cache du portail |
| GET | `/api/v1/portal/lang/builtin` | Administrateur | Packs de langue intégrés |
| POST | `/api/v1/portal/reset` | Administrateur | Restaurer le modèle par défaut |
| GET | `/api/v1/portal/templates` | Administrateur | Liste |
| POST | `/api/v1/portal/templates` | Administrateur | Créer |
| DELETE | `/api/v1/portal/templates/:id` | Administrateur | Supprimer |
| GET | `/api/v1/portal/templates/:id` | Administrateur | Détails |
| PUT | `/api/v1/portal/templates/:id` | Administrateur | Mise à jour |
| POST | `/api/v1/portal/templates/:id/activate` | Administrateur | Activer le modèle |

> `/portal/templates` CRUD est un **API avancé** : l'interface utilisateur du portail utilise principalement `PUT /api/v1/portal/active`
> (configuration unique du site complet) ; gérer/changer plusieurs modèles avec ce groupe. Les deux sont pris en charge et n’entrent pas en conflit.

### Installation des agents

Script d'installation et téléchargement de Node Agent (public).

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| POST | `/api/v1/agent/bootstrap` | Publique | Enregistrement au premier démarrage de l'agent (rapports de l'agent avec un jeton d'amorçage ; public) |
| GET | `/api/v1/agent/releases` | Administrateur | Versions d'agent disponibles (pour la page de mise à niveau) |
| GET | `/api/v1/agent/version` | Administrateur | Version actuelle de l'agent sur le panneau |
| GET | `/api/v1/agent/download` | Publique | Agent de téléchargement |
| GET | `/api/v1/agent/install` | Publique | Obtenir le script d'installation |
| GET | `/api/v1/agent/origin` | Publique | Adresse d'origine du panel (publique) |

### Entrée de rappel de facturation

Rappel de la plateforme de paiement tierce (publique, signature vérifiée).

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| POST | `/api/v1/charge/callback/:site_id` | Publique | Gérer le rappel des frais |

### Entrées (avec nœuds)

Liste entrante utilisée pour la sélection des nœuds de plan.

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| GET | `/api/v1/inbounds-with-server` | Connecté | Tous les entrants (y compris leurs nœuds) |

### Configuration publique

Configuration d'exécution du portail sans connexion (garde de maintenance + limite de débit public).

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| GET | `/api/v1/public/articles` | Publique | Liste d'articles publics |
| GET | `/api/v1/public/articles/:slug` | Publique | Détails de l'article public |
| GET | `/api/v1/public/client-configs` | Publique | Configuration de téléchargement du client (publique) |
| GET | `/api/v1/public/portal-config` | Publique | Configuration d'exécution du portail (nom du site, thème, langues, documents de conformité ; public) |
| GET | `/api/v1/public/settings` | Publique | Paramètres publics (par exemple, Registration_enabled ; public) |
| GET | `/api/v1/public/subscription-endpoints` | Publique | Liste des points de terminaison d'abonnement activés (publique) |

### Authentification client et activité (identiques au moteur d'administration)

Points de terminaison client identiques au portail. Ils utilisent un **jeton client** (`Authorization: Bearer <client_token>`), pas un jeton d'administrateur ; voir `portal-api.en.md` pour plus de détails.

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| GET | `/api/v1/client-auth/articles` | Client (connecté) | Liste des articles |
| GET | `/api/v1/client-auth/articles/:slug` | Client (connecté) | Obtenir l'article |
| GET | `/api/v1/client-auth/balance` | Client (connecté) | Obtenez l'équilibre |
| GET | `/api/v1/client-auth/balance/transactions` | Client (connecté) | Opérations de solde |
| POST | `/api/v1/client-auth/change-password` | Client (connecté) | Changer le mot de passe |
| POST | `/api/v1/client-auth/charge/create` | Client (connecté) | Créer une commande de recharge |
| GET | `/api/v1/client-auth/charge/orders` | Client (connecté) | Répertorier les commandes de recharge |
| GET | `/api/v1/client-auth/charge/orders/:id` | Client (connecté) | Obtenir les détails de la commande de recharge |
| POST | `/api/v1/client-auth/charge/redeem-card` | Client (connecté) | Utiliser la carte |
| GET | `/api/v1/client-auth/charge/sites` | Client (connecté) | Liste des sites de recharge |
| GET | `/api/v1/client-auth/charge/status/:trade_no` | Client (connecté) | Requête sur l'état de recharge |
| GET | `/api/v1/client-auth/checktime` | Client (connecté) | Obtenir l'heure du serveur (synchronisation de l'horloge) |
| GET | `/api/v1/client-auth/connection-info` | Client (connecté) | Obtenir les paramètres de connexion du plan |
| GET | `/api/v1/client-auth/gifts` | Client (connecté) | Liste des cadeaux |
| POST | `/api/v1/client-auth/gifts/:id/claim` | Client (connecté) | Réclamer un cadeau |
| GET | `/api/v1/client-auth/invoices` | Client (connecté) | Liste de commandes |
| GET | `/api/v1/client-auth/invoices/:id` | Client (connecté) | Détails de la commande |
| POST | `/api/v1/client-auth/login` | Client (public) | Connectez-vous |
| POST | `/api/v1/client-auth/logout` | Client (connecté) | Se déconnecter |
| POST | `/api/v1/client-auth/logs/package` | Client (connecté) | Créer un groupe de journaux |
| GET | `/api/v1/client-auth/logs/packages` | Client (connecté) | Répertorier les lots de journaux |
| DELETE | `/api/v1/client-auth/logs/packages/:id` | Client (connecté) | Supprimer le groupe de journaux |
| GET | `/api/v1/client-auth/logs/packages/:id` | Client (connecté) | Obtenir un ensemble de journaux |
| GET | `/api/v1/client-auth/logs/packages/:id/download` | Client (connecté) | Télécharger le paquet de journaux |
| GET | `/api/v1/client-auth/logs/types` | Client (connecté) | Vérification de l'état du paquet de journaux |
| GET | `/api/v1/client-auth/me` | Client (connecté) | Obtenir l'utilisateur actuellement connecté |
| GET | `/api/v1/client-auth/nodes` | Client (connecté) | Ma liste de nœuds |
| GET | `/api/v1/client-auth/nodes/:id/config` | Client (connecté) | Configuration du nœud |
| GET | `/api/v1/client-auth/nodes/:id/link` | Client (connecté) | Lien de partage de nœud |
| GET | `/api/v1/client-auth/nodes/:id/qrcode` | Client (connecté) | Code QR du nœud |
| GET | `/api/v1/client-auth/oauth/:provider` | Client (public) | Autoriser |
| GET | `/api/v1/client-auth/oauth/:provider/callback` | Client (public) | Rappel |
| GET | `/api/v1/client-auth/oauth/providers` | Client (public) | Statut du fournisseur |
| GET | `/api/v1/client-auth/orders` | Client (connecté) | Liste de commandes |
| POST | `/api/v1/client-auth/orders` | Client (connecté) | Créer une commande |
| GET | `/api/v1/client-auth/orders/:id` | Client (connecté) | Détails de la commande |
| GET | `/api/v1/client-auth/plans` | Client (connecté) | Liste des forfaits |
| GET | `/api/v1/client-auth/plans/:id` | Client (connecté) | Détails du forfait |
| GET | `/api/v1/client-auth/referral/code` | Client (connecté) | Obtenez le code de vérification de référence |
| GET | `/api/v1/client-auth/referral/records` | Client (connecté) | Répertorier les enregistrements de référence |
| GET | `/api/v1/client-auth/referral/stats` | Client (connecté) | Obtenez des statistiques de référencement |
| POST | `/api/v1/client-auth/referral/withdraw` | Client (connecté) | Demander le retrait |
| GET | `/api/v1/client-auth/referral/withdrawals` | Client (connecté) | Répertorier les retraits de parrainage |
| POST | `/api/v1/client-auth/refresh` | Client (public) | Actualiser |
| POST | `/api/v1/client-auth/register` | Client (public) | S'inscrire |
| POST | `/api/v1/client-auth/reset-subscription` | Client (connecté) | Réinitialiser l'abonnement |
| POST | `/api/v1/client-auth/rotate-token` | Client (connecté) | Faire pivoter le jeton d'abonnement |
| POST | `/api/v1/client-auth/self-unban` | Client (connecté) | Annulation du bannissement en libre-service |
| PUT | `/api/v1/client-auth/settings` | Client (connecté) | Mettre à jour les paramètres |
| POST | `/api/v1/client-auth/settings/auto-add-ip` | Client (connecté) | Ajouter automatiquement une adresse IP |
| GET | `/api/v1/client-auth/settings/whitelist` | Client (connecté) | Liste des adresses IP autorisées |
| DELETE | `/api/v1/client-auth/settings/whitelist/:id` | Client (connecté) | Supprimer de la liste verte IP |
| GET | `/api/v1/client-auth/shared-accounts` | Client (connecté) | Répertorier les comptes partagés |
| GET | `/api/v1/client-auth/shared-accounts/:id` | Client (connecté) | Obtenir un compte partagé |
| POST | `/api/v1/client-auth/shared-accounts/:id/apply` | Client (connecté) | Demander un compte partagé |
| POST | `/api/v1/client-auth/shared-accounts/:id/repair` | Client (connecté) | Demande de réparation |
| POST | `/api/v1/client-auth/ticket` | Client (connecté) | Émettre un ticket unique de courte durée |
| GET | `/api/v1/client-auth/tickets` | Client (connecté) | Liste des billets |
| POST | `/api/v1/client-auth/tickets` | Client (connecté) | Créer un ticket |
| GET | `/api/v1/client-auth/tickets/:id` | Client (connecté) | Obtenir un billet |
| POST | `/api/v1/client-auth/tickets/:id/close` | Client (connecté) | Fermer le ticket |
| POST | `/api/v1/client-auth/tickets/:id/reply` | Client (connecté) | Répondre au ticket |
| GET | `/api/v1/client-auth/traffic-history` | Client (connecté) | Historique du trafic |
| GET | `/api/v1/client-auth/traffic-packs` | Client (connecté) | Liste des packs de trafic |
| POST | `/api/v1/client-auth/traffic-packs/buy` | Client (connecté) | Acheter un pack de trafic |

### Livraison d'abonnement

Point d'entrée public permettant aux clients de récupérer des abonnements ; voir `subscription.en.md`.

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| GET | `/sub/:token` | Publique | Servir l'abonnement |

### Flux de travail de configuration d'exécution

Instantanés de configuration/exécution à sec/application/restauration (`apply`, `rollback` et `disable-proxy` nécessitent un administrateur).

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| POST | `/api/v1/runtime/apply` | Administrateur | Postuler |
| GET | `/api/v1/runtime/apply-jobs` | Connecté | Liste des emplois |
| GET | `/api/v1/runtime/apply-jobs/:id` | Connecté | Obtenir un emploi |
| POST | `/api/v1/runtime/apply-jobs/:id/cancel` | Administrateur | Annuler le travail |
| POST | `/api/v1/runtime/apply-jobs/:id/retry` | Administrateur | Réessayer le travail |
| POST | `/api/v1/runtime/disable-proxy` | Administrateur | Désactiver le proxy |
| POST | `/api/v1/runtime/dry-run` | Connecté | Exécution à sec |
| GET | `/api/v1/runtime/nodes` | Connecté | Répertorier les nœuds |
| GET | `/api/v1/runtime/overview` | Connecté | Aperçu |
| POST | `/api/v1/runtime/rollback` | Administrateur | Revenir en arrière |
| GET | `/api/v1/runtime/snapshots` | Connecté | Répertorier les instantanés |
| POST | `/api/v1/runtime/snapshots` | Connecté | Créer un instantané |
| DELETE | `/api/v1/runtime/snapshots/:id` | Administrateur | Supprimer l'instantané |
| GET | `/api/v1/runtime/snapshots/:id` | Connecté | Obtenir un instantané |
| POST | `/api/v1/runtime/verify` | Connecté | Vérifier |

### Politiques d'accès aux services (retraitées)

> Les `/services/:id/access-policies` et `/services/:id/ip-bans` ont été retirées le 20/10/2026 :
> il n'y a aucun appel d'administrateur et aucun chemin de connexion ne consomme ces politiques.

### Surveillance des quotas de trafic

Vue de surveillance en lecture seule des quotas et de l'utilisation.

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| GET | `/api/v1/traffic/quotas` | Connecté | Liste des quotas |
| GET | `/api/v1/traffic/quotas/:id/usage` | Connecté | Utilisation des quotas |
| GET | `/api/v1/traffic/reset-logs` | Connecté | Enregistrements de réinitialisation de quota |
| GET | `/api/v1/traffic/usage-history` | Connecté | Historique d'utilisation |

### Bilans de santé

Sonde de vivacité.

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| GET | `/health` | Publique | Sonde de vivacité ; 200 signifie en bonne santé |

### Page du portail

Page du portail SPA.

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| GET | `/p` | Publique | Portail HTML (rendu par le serveur + runtime Vue) |

### Défis ACME

Rappel de défi HTTP-01.

| Méthode | Chemin | Autorisation | Descriptif |
|---|---|---|---|
| GET | `/.well-known/acme-challenge/:token` | Publique | Rappel de défi ACME HTTP-01 (vérifie le domaine lors de l'émission) |
