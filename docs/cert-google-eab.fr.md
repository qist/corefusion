<!-- source: cert-google-eab.md | locale: fr | updated: 2026-10-23 -->
# Guide CA public Google (émission automatique EAB)

> Langues : [简体中文](cert-google-eab.md) · [English](cert-google-eab.en.md) · [繁體中文](cert-google-eab.zh-TW.md) · [Русский](cert-google-eab.ru.md) · [日本語](cert-google-eab.ja.md) · [한국어](cert-google-eab.ko.md) · [Deutsch](cert-google-eab.de.md) · Français · [Español](cert-google-eab.es.md) · [Português](cert-google-eab.pt.md) · [Türkçe](cert-google-eab.tr.md) · [Bahasa Indonesia](cert-google-eab.id.md) · [ไทย](cert-google-eab.th.md) · [Tiếng Việt](cert-google-eab.vi.md) · [العربية](cert-google-eab.ar.md) · [فارسی](cert-google-eab.fa.md)

> S'applique à : panneau v0.12.101+ | Page : **Certificats → Configuration EAB**
> Code : `internal/service/certificate/eab_google.go`, `internal/api/handlers/certificates/eab_config.go`

## 1. Pourquoi Google nécessite EAB

| CA | EAB obligatoire pour ACME ? | Comment obtenir EAB |
|---|---|---|
| Chiffrons | ❌ Non | — |
| ZeroSSL | ✅ Oui | **Échangez un e-mail contre un** (`api.zerossl.com/acme/eab-credentials-email`, identique à acme.sh) ; le panneau peut le récupérer automatiquement |
| **Google Autorité de certification publique** | ✅**Oui** | **Doit être créé avec autorisation GCP** (un email ne suffit pas) → le sujet de cette page |

En d'autres termes, Google **ne peut pas éviter « l'autorisation »** : soit un fichier JSON de compte de service, soit un jeton d'accès temporaire.

## 2. Les règles strictes de Google (à lire en premier)

| Règle | Descriptif |
|---|---|
| **Utiliser dans les 7 jours** | Une clé EAB **expire** si elle n'est pas utilisée dans les 7 jours et doit être recréée |
| **Un seul compte** | Une clé EAB enregistre exactement un compte ACME ; après l'enregistrement, la clé **est invalidée** et ne peut pas être réutilisée |
| **Les comptes ACME n'expirent pas** | Le compte ACME créé avec celui-ci est **de longue durée** → **les renouvellements n'ont pas besoin de EAB** |
| ⚠ **La suppression du projet GCP tue tous les comptes** | La suppression du projet invalide immédiatement **tous les comptes ACME** et **le renouvellement devient impossible** |

> Conclusion : **EAB est un « identifiant d'intégration » unique ; le compte ACME est ce qui dure.**
> Le panneau stocke les comptes dans `data/acme/accounts/<ca>.json` — **sauvegardez ce répertoire**.

## 3. Trois chemins (commencez par le chemin A – pas de recherche de menu)

> Les points d'entrée de Cloud Console ont changé plusieurs fois, les liens directs et Cloud Shell passent donc en premier ; les chemins de menu sont des solutions de secours.

### Chemin A (le plus simple, recommandé en premier) : une commande dans GCP Cloud Shell

Cloud Shell est un terminal de navigation avec **gcloud préinstallé et déjà connecté** ; pas de configuration locale et pas de menus.

1. Ouvrez <https://console.cloud.google.com/cloudshell> (ou l'icône `>_` en haut à droite de la console)
2. Sélectionnez le projet (l'ID du projet est visible dans le sélecteur de projet en haut) :
   ```bash
   gcloud config set project <your-project-id>
   ```
3. Activez l'autorité de certification publique API (**équivalent** à son activation dans la bibliothèque API) :
   ```bash
   gcloud services enable publicca.googleapis.com
   ```
4. Créez le EAB (**voici les deux chaînes dont vous avez besoin**) :
   ```bash
   gcloud publicca external-account-keys create
   ```
 Dans la sortie : `keyId` → le **EAB-KID** du panneau ; `b64MacKey` → la **EAB-HMAC-Key** du panneau
5. De retour dans le panneau : **Certificats → Configuration EAB → Ajouter une configuration EAB → CA : Google Public CA** →
 saisir les deux valeurs dans `EAB-KID` / `EAB-HMAC-Key` → sauvegarder ✓

> ⚠ Coût : les clés Google EAB sont **7 jours, uniques** ✗ → après 7 jours (ou lors d'un changement de compte) **répétez l'étape 4**.
> Pour éviter cela, utilisez le chemin B (stockez un compte de service une fois et laissez le panneau redemander automatiquement).

### Chemin B (configuration unique, puis automatique) : clé de compte de service JSON

**① Activez l'autorité de certification publique API** (le lien direct est le plus simple)
<https://console.cloud.google.com/apis/library/publicca.googleapis.com?project=YOUR_PROJECT_ID> → cliquez sur « Activer »
Solution de secours : ☰ → **API et services → Bibliothèque** → rechercher `Public CA` → ouvrir → « Activer »

**② Créez un compte de service**
<https://console.cloud.google.com/iam-admin/serviceaccounts?project=YOUR_PROJECT_ID>
→ «**+ Créer un compte de service**» → n'importe quel nom (par exemple `corefusion-eab`) → « Créer et continuer » → vous pouvez **ignorer** l'étape du rôle → « Terminé »
Solution de secours : ☰ → **IAM et administrateur → Comptes de service**

**③ Accordez le rôle minimal** (le plus simple sur la même page)
Cliquez sur le compte nouvellement créé → **onglet « Autorisations »** → « Accorder l'accès » → nouveau principal : l'**e-mail du compte de service**
→ recherchez le rôle `publicca` → choisissez ** « Créateur de clé de compte externe de l'autorité de certification publique »** → enregistrez
Solution de secours : ☰ → **IAM et administrateur → IAM** → « Accorder l'accès » → même e-mail + rôle
⚠ **Ne pas accorder de propriétaire/éditeur** ✗

**④ Téléchargez la clé JSON**
Comptes de service → ouvrir le compte → **onglet « Clés »** → « **Ajouter une clé** » → « **Créer une nouvelle clé** » → **JSON** → créer
(le navigateur télécharge un `xxx.json` ; ou ligne `⋮` → « Gérer les clés » → même flux)

**⑤ De retour dans le panneau** : saisissez l'**ID du projet** et collez le **contenu entier** du JSON dans « Fichier de clé du compte de service » → cliquez sur « Demander automatiquement le EAB ».
→ ensuite le panel **redemande automatiquement EAB** à son expiration ✓, plus de visites GCP ✓

### Chemin C (test ponctuel uniquement) : coller un jeton d'accès

Dans Cloud Shell :
```bash
gcloud auth print-access-token
```
Collez le résultat dans le « Jeton d'accès temporaire » du panneau ainsi que l'ID du projet → cliquez sur « Demander automatiquement EAB »
⚠ Valable environ 1 heure et **non stocké** ✓ → vérification uniquement, pas de nouvelle demande automatique ✗

### Aide-mémoire lorsque vous ne trouvez pas de point d'entrée (les menus de la console changent)

| Tâche | Lien direct (le plus fiable) | Solution de secours : menu |
|---|---|---|
| Activer l'autorité de certification publique API | `https://console.cloud.google.com/apis/library/publicca.googleapis.com?project=<PROJECT_ID>` | ☰ → API et services → Bibliothèque → recherche `Public CA` |
| Comptes de service | `https://console.cloud.google.com/iam-admin/serviceaccounts?project=<PROJECT_ID>` | ☰ → IAM et administrateur → Comptes de service |
| Accorder un rôle | `https://console.cloud.google.com/iam-admin/iam?project=<PROJECT_ID>` | ☰ → IAM et administrateur → IAM → Accorder l'accès |
| Coquille de nuage | `https://console.cloud.google.com/cloudshell` | Icône `>_` en haut à droite |

> Astuce : remplacez `<PROJECT_ID>` / `YOUR_PROJECT_ID` par le véritable ID du projet (comme `my-project-123456`) pour accéder directement à la page.

## 4. Configuration du panneau (deux façons)

Accédez à **Certificats → Configuration EAB → Ajouter une configuration EAB** et choisissez **Google Public CA** :

### Option A (recommandée) : collez le compte de service JSON

| Champ | Valeur |
|---|---|
| ID du projet GCP | L'ID du projet de la section 3, par ex. `my-project-123456` |
| Compte de service JSON | Collez le **tout le contenu** de `corefusion-eab.json` |
| Jeton d'accès temporaire | Laisser vide |
| Nom | Généré automatiquement sous le nom `Google Public CA - <project-id>` ; modifiable |

Cliquez sur **Demander automatiquement le EAB** → en cas de succès, les clés KID/HMAC sont renseignées et enregistrées ; plus tard, choisissez simplement `CA = Google Public CA` lors de l'émission (EAB est associé par CA ; pas de sélection manuelle).

> Le compte de service JSON est **chiffré dans la configuration EAB de la base de données du panel** (`json:"-"`, **jamais renvoyé au frontend**).
> L'objectif est de **demander à nouveau une clé automatiquement après l'expiration de 7 jours** au lieu de visites GCP manuelles.

### Option B : jeton d'accès temporaire (unique)

```bash
gcloud auth print-access-token        # valid ~1 hour
```

Collez-le dans « Jeton d'accès temporaire », remplissez l'ID du projet, cliquez sur **Demander automatiquement EAB**.
⚠ Ce token n'est **pas stocké** et doit être réobtenu après expiration → bien pour un test ponctuel, pas pour une utilisation à long terme.

### Équivalent en ligne de commande (facultatif, à titre de comparaison)

```bash
gcloud publicca external-account-keys create
# returns keyId / b64MacKey, corresponding to ACME's --eab-kid / --eab-hmac-key
```

## 4.5 Pourquoi JSON au lieu de « une seule clé » ?

C'est la question la plus courante. Séparez les deux types de « clé » :

| À quoi vous pouvez vous attendre | Ce qui est réellement nécessaire |
|---|---|
| **Clé API** (la chaîne `AIza…`) | ❌ **Inutile** — une clé API n'est qu'un identifiant public et **ne porte aucune identité** ; la création de EAB nécessite des **autorisations IAM**, des informations d'identification permettant de vérifier l'identité sont donc requises |
| — | ✅ **Clé de compte de service** : le **fichier** téléchargé depuis GCP « IAM → Comptes de service → Clés → Créer une nouvelle clé → **JSON** » |

Point clé : **le fichier JSON lui-même est "une clé"** — GCP regroupe l'identité du compte (`client_email`) ainsi que la clé privée (`private_key`) dans un seul fichier. Le panneau signe un JWT avec celui-ci → échange contre un jeton d'accès OAuth2 → appelle l'autorité de certification publique API.
Donc « coller JSON » ≠ en remplissant de nombreux champs ; **collez l'intégralité du fichier** (le panneau remplit également les champs facultatifs tels que `type` / `token_uri`).

**Le vrai EAB est constitué des deux chaînes** (`EAB-KID` + `EAB-HMAC-Key`), qui sont le **résultat** de la requête ; le panneau les remplit automatiquement.

### Vous ne souhaitez pas remettre le dossier ? Deux alternatives

| Méthode | Comment | Coût |
|---|---|---|
| **Jeton d'accès temporaire** | `gcloud auth print-access-token` → coller dans « Jeton d'accès temporaire » | ~Validité 1 heure, **non stocké** → unique, pas de nouvelle demande automatique |
| **Manuel KID/HMAC** | Exécutez localement `gcloud publicca external-account-keys create` → entrez `keyId` / `b64MacKey` dans « EAB-KID / EAB-HMAC-Key » | Aucune pièce d'identité remise ✓ ; mais Google EAB est **7 jours, unique** → répéter après expiration |

> Le panel recommande le compte de service JSON pour une raison : **EAB expire après 7 jours**, et seul un compte de service stocké permet une **nouvelle demande** automatique sans travail manuel.

## 4.6 FAQ : puis-je utiliser une clé API ?

**Non.** Il ne s'agit pas d'un choix d'implémentation — **Google le rejette explicitement** (vérifié dans les deux sens) :

```bash
# 1) Query parameter
curl -X POST "https://publicca.googleapis.com/v1/projects/<ID>/locations/global/externalAccountKeys?key=AIza..." -d '{}'
# 2) x-goog-api-key header recommended by the official docs
curl -X POST "https://publicca.googleapis.com/v1/projects/<ID>/locations/global/externalAccountKeys" \
     -H "x-goog-api-key: AIza..." -d '{}'
```

Les deux reviennent :

```json
{"error":{"code":401,"status":"UNAUTHENTICATED",
  "message":"API keys are not supported by this API. Expected OAuth2 access token
             or other authentication credentials that assert a principal."}}
```

À titre de comparaison, un **(faux) porteur OAuth** renvoie
`Request had invalid authentication credentials. Expected OAuth 2 access token...` —
donc OAuth est reconnu tandis que les **clés API sont purement et simplement rejetées** ✗.

**Pourquoi** : créer EAB est une **opération administrative** qui nécessite de savoir « qui vous êtes et si vous avez `roles/publicca.externalAccountKeyCreator` » ✗
— une clé API identifie uniquement un projet/application ✓ et **n'exprime pas d'identité principale** ✓.
(Les « Meilleures pratiques de gestion des clés API » de Google recommandent **IAM + informations d'identification de compte de service de courte durée** pour les scénarios d'administration ✓ — la même approche utilisée ici ✓.)

**Il n'y a donc que trois chemins** ✓ (voir §3) : compte de service JSON ✓ / jeton d'accès temporaire ✓ /
ou **aucune information d'identification** : exécutez `gcloud publicca external-account-keys create` dans Cloud Shell ✓
et entrez `keyId` / `b64MacKey` dans le panneau ✓ (**utilisez ceci si GCP IAM vous bloque** ✓).

## 5. Inscription et renouvellements automatiques (important)

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

Par conséquent :

- **La première émission s'enregistre automatiquement** ; aucune étape GCP manuelle n'est nécessaire ;
- Les renouvellements **ne consomment pas** de EAB (tant que le fichier de compte existe et que le répertoire de données reste stable) ;
- ⚠ Le répertoire de données ACME côté agent est un chemin absolu fixe (`/var/lib/corefusion-agent/acme` par défaut) ;
 **ne le modifiez pas par hasard**, sinon il sera traité comme un nouveau compte → une autre clé EAB requise.

## 6. Dépannage

| Symptôme/erreur | Parce que | Corriger |
|---|---|---|
| `PERMISSION_DENIED` (403) | Le compte de service ne contient pas `roles/publicca.externalAccountKeyCreator` | Retournez à la section 3 ④ |
| `SERVICE_DISABLED` (403) | `publicca.googleapis.com` non activé pour le projet | Article 3 ② |
| `NOT_FOUND` (404) | Mauvais ID de projet/projet supprimé | Vérifier l'ID du projet |
| Rapports d'émission `externalAccountRequired` | Cette autorité de certification n'a pas de EAB | Demandez/configurez d'abord le EAB dans le panneau |
| Rapports d'émission `Accounts must have at least one contact.` | Aucun email n'a été fourni lors de l'inscription | Définissez l'e-mail du compte ACME sur la page du certificat (le panneau utilise le compte par défaut) |
| Le renouvellement demande à nouveau le EAB / l'enregistrement échoue | Fichier de compte ACME perdu (répertoire de données modifié/nettoyé) | **Restaurer la sauvegarde** ; sans un, demandez un nouveau EAB en utilisant le compte de service |
| L'émetteur passe de Google à Let's Encrypt | Commande de renouvellement manquant l'autorité de certification (ancien bug, corrigé dans la v0.12.101) | Mise à niveau et réédition |

## 7. Recommandations en matière de sécurité et d'exploitation

1. **Moins de privilèges** : accordez le compte de service uniquement `roles/publicca.externalAccountKeyCreator`, jamais Propriétaire/Éditeur.
2. **Ne supprimez pas le projet** : la suppression du projet GCP invalide tous ses comptes ACME → les certificats ne peuvent pas être renouvelés.
3. **Toujours sauvegarder** `data/acme/accounts/` (l'identité du compte elle-même) et la configuration EAB de la base de données du panneau.
4. **Pas de panique concernant l'expiration du EAB** : tant que le fichier de compte ACME existe, les renouvellements ne sont pas affectés ; un nouveau EAB n'est nécessaire que pour créer un nouveau compte.
5. Si la clé du compte de service a pu fuir : supprimez cette clé dans GCP et collez un nouveau JSON dans le panneau.

## 8. Points de terminaison associés (pour les scripts)

| Point de terminaison | Descriptif |
|---|---|
| `POST /api/v1/eab-configs/fetch-google` | Créez un EAB à l'aide de l'autorisation GCP. Corps : `{project_id, service_account_json?, access_token?}` |
| `POST /api/v1/eab-configs/fetch-zerossl` | Obtenez un ZeroSSL EAB par e-mail. Corps : `{email}` |
| `GET  /api/v1/eab-configs` | Répertoriez les configurations EAB (**sans** la clé HMAC ou le compte de service JSON ; uniquement `has_mac_key` / `has_google_sa`) |
