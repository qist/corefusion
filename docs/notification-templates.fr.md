<!-- source: notification-templates.md | locale: fr | updated: 2026-10-23 -->
# Variables du modèle de notification

> Langues : [简体中文](notification-templates.md) · [English](notification-templates.en.md) · [繁體中文](notification-templates.zh-TW.md) · [Русский](notification-templates.ru.md) · [日本語](notification-templates.ja.md) · [한국어](notification-templates.ko.md) · [Deutsch](notification-templates.de.md) · Français · [Español](notification-templates.es.md) · [Português](notification-templates.pt.md) · [Türkçe](notification-templates.tr.md) · [Bahasa Indonesia](notification-templates.id.md) · [ไทย](notification-templates.th.md) · [Tiếng Việt](notification-templates.vi.md) · [العربية](notification-templates.ar.md) · [فارسی](notification-templates.fa.md)

Les modèles de notification utilisent des espaces réservés `{variable}` qui sont remplacés par des valeurs réelles lors de l'envoi. Ce document répertorie toutes les variables prises en charge, les variables disponibles par événement et les règles de déclenchement automatique.

## 1. Types de modèles et publics

| Public | Chaîne | Destinataire | Utilisation typique |
|---|---|---|---|
| **Utilisateurs du panel** (administrateur) | Telegram / Courriel | Canaux et destinataires configurés par l'administrateur | Alertes de nœud, rapports de sauvegarde, annonces système |
| **Abonnés** | Courriel | L'e-mail du client (rempli automatiquement) | Solde faible, rappels d'expiration d'abonnement |

## 2. Variables communes

Disponible dans tous les modèles :

| Variable | Descriptif | Exemple |
|---|---|---|
| `{time}` | Heure d'envoi | `2026-09-28 14:30:00` |
| `{user}` | Email client (modèles d'abonné) / identifiant utilisateur | `user@example.com` |
| `{message}` | Corps de notification | Texte d'annonce de maintenance |

## 3. Variables d'événement d'abonné

### Rappel d'expiration de l'abonnement (event_key = `subscription_expire`)

Scanné quotidiennement et envoyé aux abonnés actifs expirant dans les 7 jours :

| Variable | Descriptif | Exemple |
|---|---|---|
| `{user}` | E-mail du client | `user@example.com` |
| `{expire_date}` | Expiration de l'abonnement | `2026-10-05 09:00` |
| `{days}` | Jours restants | `7` |
| `{time}` | Heure d'envoi | `2026-09-28 09:05:00` |

### Rappel de solde faible (event_key = `balance_low`)

Scanné quotidiennement et envoyé aux clients dont le solde est inférieur au seuil :

| Variable | Descriptif | Exemple |
|---|---|---|
| `{user}` | E-mail du client | `user@example.com` |
| `{balance}` | Solde courant (unités monétaires, deux décimales) | `0.85` |
| `{time}` | Heure d'envoi | `2026-09-28 09:05:00` |

## 4. Exemples de variables de modèle d'opérations

Lorsque vous cliquez sur « Envoyer » pour un test de connectivité, les modèles d'utilisateur du panel remplissent les espaces réservés avec ces exemples :

| Variable | Descriptif | Exemple |
|---|---|---|
| `{node}` | Nom du nœud | `node-01` |
| `{backup_file}` | Nom du fichier de sauvegarde | `corefusion-20260101-0900.tar.gz` |
| `{size}` | Taille du fichier | `128MB` |
| `{domain}` | Domaine | `example.com` |
| `{expire_date}` | Date d'expiration | `2026-10-12` |
| `{days}` | Jours restants | `14` |
| `{count}` | Compter | `3` |

## 5. Règles de déclenchement automatique

- **Planification** : la première analyse s'exécute 30 secondes après le démarrage du panneau, puis quotidiennement à **09h05**
- **Expiration de l'abonnement** : analyse les abonnements actifs (`status=active`) expirant dans les 7 jours ; le client doit également être actif
- **Solde faible** : analyse les clients actifs dont le solde est ≤ seuil ; le seuil est `notify.balance_threshold`, **en cents** (par défaut `100` = 1,00)
- **Déduplication** : au maximum un message par client et par événement **par jour** (déduplications `notification_send_logs` par jour)

## 6. Surveillance de l'état des nœuds

Les battements de cœur des nœuds sont vérifiés en permanence ; **les notifications sont envoyées uniquement lorsque l'état change — les tours sans changement sont silencieux** :

| Clé de réglage | Descriptif | Par défaut |
|---|---|---|
| `monitor.node_interval_seconds` | Intervalle de vérification (secondes) | `60` (minimum 30) |
| `monitor.node_offline_timeout_seconds` | Délai d'expiration du battement de coeur (secondes) | `120` |

- **Alerte hors ligne** : lorsqu'un nœud passe de en ligne à hors ligne, envoie le modèle d'administrateur « **Alerte hors ligne de nœud** » avec `{node}` (nom) et `{time}`.
- **Avis de récupération** : lorsqu'un nœud récupère, envoie « **Avis de récupération de nœud** » avec les mêmes variables
- **Tours silencieux** : aucun message lorsque l'état est inchangé ; après un redémarrage du panel, le premier tour établit uniquement une ligne de base
- **Surveiller sans envoi** : désactiver le modèle correspondant (Enabled = OFF)

## 7. Alertes mensuelles de quota de trafic de nœud (automatique)

- **Déclencheur** : lorsque le trafic mensuel d'un nœud atteint le seuil de quota, envoie « **Alerte de quota de trafic de nœud** » ; lorsqu'il dépasse 100 % et est suspendu, envoie une alerte de dépassement de limite (clés d'événement `traffic_quota_threshold` / `traffic_quota_exceeded`)
- **Seuil** : `monitor.traffic_quota_threshold_percent` (pourcentage, `80` par défaut ; définissez `0` pour désactiver les alertes de seuil)
- **Déduplication** : au maximum une par nœud et par cycle de facturation (le cycle suit le `traffic_quota_reset_day` du nœud)
- Variables disponibles : `{node}` (nom), `{used}` (Go utilisé), `{quota}` (quota de Go), `{percent}` (pourcentage), `{time}`

## 8. Notifications de résultat de sauvegarde (automatique)

- **Déclencheur** : envoyé immédiatement après l'exécution d'une sauvegarde planifiée (`BackupScheduler`) — la réussite envoie « **Rapport de sauvegarde quotidien** », l'échec (création de la sauvegarde/lecture du stockage/téléchargement vers un site distant) envoie « **Alerte d'échec de sauvegarde** »
- Variables disponibles : `{backup_file}` (nom), `{size}` (taille, rapport de réussite uniquement), `{message}` (raison de l'échec, alerte d'échec uniquement), `{time}`
- **Surveiller sans envoi** : ignoré silencieusement lorsque le modèle est désactivé (Enabled = OFF)

## 9. Rappels d'expiration du certificat (automatique)

- **Déclencheur** : tous les jours à 09h05, analyse les certificats `active` en voie d'expiration (**certificats IP avec 3 jours restants**, **certificats de domaine avec 7 jours restants**) et envoie le modèle d'administrateur « **Rappel d'expiration du certificat** » par certificat
- **Déduplication** : au maximum un rappel par certificat et par jour ; après le renouvellement, l'expiration se déplace en dehors de la fenêtre (IP 3 jours / domaine 7 jours) et les rappels s'arrêtent
- Variables disponibles : `{domain}` (domaine du certificat), `{expire_date}` (date d'expiration), `{days}` (jours restants), `{time}`

## 10. Alertes de crash des processus principaux (automatiques)

- **Déclencheur** : lorsqu'un processus principal (xray/singbox) se termine anormalement ou que le processus proxy (caddy/nginx) plante, envoie immédiatement le modèle d'administrateur « **Alerte de crash du processus principal** » ; le superviseur le redémarre ensuite avec un recul exponentiel
- Variables disponibles : `{core}` (type de processus), `{restarts}` / `{max}` (compte/limite de redémarrage), `{message}` (message de sortie, processus principaux uniquement), `{time}`
- **Surveiller sans envoi** : ignoré silencieusement lorsque le modèle est désactivé (Enabled = OFF)

## 11. Avis d’enregistrement de nouveaux utilisateurs (automatique)

- **Déclencheur** : envoyé immédiatement après l'inscription réussie d'un client, à l'aide du modèle d'administration « **Avis d'enregistrement d'un nouvel utilisateur** »
- Variables disponibles : `{user}` (e-mail client), `{time}`

## 12. Utilisation

1. **Ajouter un modèle** : Paramètres de notification → Modèles de notification → Ajouter un modèle → « Remplir à partir du préréglage » et ajuster si nécessaire
2. **Activer** : cochez Activé et enregistrez ; les déclencheurs automatiques traitent uniquement les modèles activés
3. **Test** : cliquez sur "Envoyer" dans la liste pour envoyer un vrai message avec des exemples de variables (les e-mails de test du modèle d'abonné vont à l'adresse de l'expéditeur de l'administrateur)
4. **Notifications immédiates** (code entreprise) : appelez `NotifyClientEvent(ctx, db, settingSvc, eventKey, clientEmail, vars)` sur le site de l'événement (recharge, déduction, renouvellement) pour avertir immédiatement un seul client, en dehors de la limite d'analyse quotidienne
