<!-- source: oauth-google-verification.md | locale: fr | updated: 2026-10-23 -->
# Guide de vérification OAuth Google

> Langues : [简体中文](oauth-google-verification.md) · [English](oauth-google-verification.en.md) · [繁體中文](oauth-google-verification.zh-TW.md) · [Русский](oauth-google-verification.ru.md) · [日本語](oauth-google-verification.ja.md) · [한국어](oauth-google-verification.ko.md) · [Deutsch](oauth-google-verification.de.md) · Français · [Español](oauth-google-verification.es.md) · [Português](oauth-google-verification.pt.md) · [Türkçe](oauth-google-verification.tr.md) · [Bahasa Indonesia](oauth-google-verification.id.md) · [ไทย](oauth-google-verification.th.md) · [Tiếng Việt](oauth-google-verification.vi.md) · [العربية](oauth-google-verification.ar.md) · [فارسی](oauth-google-verification.fa.md)

> Portée : connexion Google pour le portail CoreFusion (`internal/service/oauth`, portées : `openid`, `email`, `profile`).
> Comparaison GitHub à la fin (GitHub n'a pas de flux de vérification).

## 1. Mode test vs mode production

L'écran de consentement OAuth Google comporte deux états de publication :

| État | Qui peut se connecter | Vérification requise |
|---|---|---|
| **Test** | Uniquement les comptes Google dans la liste « utilisateurs test » | Non |
| **En production** | Tout compte Google | **Oui** (voir ci-dessous) |

Une fois « Paramètres système → OAuth » activé, la page de connexion au portail affiche le bouton Google à chaque visiteur ;
si l'application est encore en test, les visiteurs ne figurant pas sur la liste des utilisateurs de test voient
`Error 403: access_denied` (application non vérifiée / vous n'êtes pas un utilisateur test).
Pour proposer publiquement la connexion à Google, **passez l'application en production et réussissez la vérification**.

## 2. Exigences strictes en matière de vérification

L'examen de Google (vérification de base pour les portées non restreintes) nécessite :

1. **Page d'accueil** : un URL accessible publiquement sur le domaine autorisé (même domaine ou un sous-domaine).
2. **Lien vers la politique de confidentialité** : doit être **accessible au public** (pas de connexion) et expliquer quelles données vous collectez et comment vous les utilisez/stockez/supprimez. Pour la connexion Google, elle doit au moins couvrir :
 les données du compte Google collectées (email, nom, avatar), la finalité (création/connexion au compte portail), pas de vente à des tiers,
 et comment les utilisateurs peuvent supprimer leur compte et leurs données.
3. **Lien vers les conditions de service** : accessible au public, décrivant les règles du service.
4. **Domaines autorisés** : enregistrez le domaine de l'application dans l'écran de consentement (par exemple `example.com`) ; Les liens accueil/confidentialité/conditions doivent figurer sur celui-ci ou sur un sous-domaine.
5. **Logo de l'application (facultatif mais recommandé)** : 120×120px.
6. **Justification de la portée** : ce projet utilise uniquement `openid / email / profile` (non restreint, non sensible),
 qui relève de la vérification de base – aucune justification par portée et **aucune évaluation de sécurité par un tiers**.
 (Les portées restreintes telles que l'accès complet à Gmail les nécessiteraient ; ce projet n'en utilise pas.)

Entrée de vérification : [Google Cloud Console](https://console.cloud.google.com/) →
**API et services → Écran de consentement OAuth → Publier l'application → Soumettre pour vérification**.
La vérification de base se termine généralement en quelques jours ; vous pouvez fonctionner en mode test en attendant.

## 3. Que configurer pour ce projet

### 3.1 Redirection autorisée URI

Le rappel est généré dynamiquement à partir du domaine qui démarre la connexion (voir `Authorize` dans `internal/api/handlers/oauth/handler.go`) :

```
https://<portal-domain>/api/v1/client-auth/oauth/google/callback
```

- Connexion démarrée à partir du domaine du portail (par exemple `https://p.example.com`) → enregistrez `https://p.example.com/...` ;
- Démarré à partir du domaine du panneau `/p` → enregistrer `https://panel.example.com/...`.
- Google autorise **plusieurs** URI de redirection ; enregistrez les deux domaines.
- ⚠ Google nécessite que les URI de redirection soient **https** (sauf `http://localhost`) ;
 le proxy inverse doit transmettre `X-Forwarded-Host` / `X-Forwarded-Proto`, sinon le panneau crée un
 http ou redirect_uri interne → Google le rejette (`redirect_uri_mismatch`).

Après avoir défini « Paramètres système → OAuth → Domaine de rappel du portail », le rappel URL affiché sur la page est celui à coller dans Google.

### 3.2 Liens Politique de confidentialité / Conditions sur le portail

La carte de connexion comprend des liens « Politique de confidentialité · Conditions d'utilisation » (affichés une fois configurés), et les deux documents comportent des **pages autonomes**.
(la boîte de dialogue et la page autonome partagent le même contenu) :

- Domaine du panneau principal : `https://<panel-domain>/p/legal/privacy`, `/p/legal/terms`
- Domaine de portail dédié (déploiement racine) : `https://<portal-domain>/legal/privacy`, `/legal/terms`
- Prend en charge `?lang=xx` ; sans cela, le `Accept-Language` du navigateur est utilisé, puis le portail par défaut.
- Les pages sont **rendues par le serveur** (pas de JS), ce qui aide le robot de vérification de Google.

**Configuration (dans tous les cas ; les liens gagnent)** :

1. **Mode contenu (recommandé, fonctionne immédiatement)** : éditeur de portail → Paramètres → Général → Documents de conformité →
 choisissez « Modifier la langue » et collez les modèles §4 dans « Contenu de la politique de confidentialité » / « Contenu des conditions de service »
 (**Markdown, édité par langue**) → après l'enregistrement, la carte de connexion affiche des liens qui ouvrent une **boîte de dialogue** ;
 le « lien autonome » de l'éditeur peut être saisi directement dans l'écran de consentement Google.
2. **Mode lien externe** : saisissez « Politique de confidentialité URL » / « Conditions d'utilisation URL » (pointant vers les pages que vous hébergez) → s'ouvre dans un nouvel onglet.

Le contenu et les liens sont fournis via `/api/v1/public/portal-config` (`privacy_policy_content` /
`terms_of_service_content` sont des **cartes Markdown par langue** `{"zh-CN":"...","en":"..."}` ;
`privacy_policy_url` / `terms_of_service_url` sont des liens externes facultatifs). **Ils doivent être accessibles sans connexion** ✓.

### 3.3 Interrupteur côté panneau

`System settings → OAuth` : activez Google et saisissez l'ID client / le secret client (le secret est chiffré avec AES-256-GCM au repos).
Le bouton Google apparaît sur la page de connexion au portail uniquement lorsque tout est configuré (`/api/v1/client-auth/oauth/providers` renvoie activé).

## 4. Modèles de politique de confidentialité/conditions d’utilisation

> Modèles de conformité minimale que vous pouvez adapter. Avant utilisation : remplacez les espaces réservés `【】`, ajoutez/supprimez des clauses pour votre entreprise,
> et demandez à l'entité opérationnelle de les examiner. Ces modèles ne constituent pas des conseils juridiques.

### 4.1 Politique de confidentialité (modèle)

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

### 4.2 Conditions de service (modèle)

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

## 5. Liste de contrôle de pré-lancement

- [ ] Écran de consentement OAuth : nom de l'application/logo/page d'accueil/politique de confidentialité/conditions toutes présentes ; domaine enregistré
- [ ] Pages de politique de confidentialité et de conditions **accessibles sans connexion** (page personnalisée du portail ou page statique autonome)
- [ ] Les deux liens visibles sur la carte de connexion/pied de page du portail
- [ ] La redirection URI correspond au domaine réel et est https
- [ ] Publier l'application → soumettre pour vérification ; la restriction de l'utilisateur test est levée après approbation
- [ ] Paramètres OAuth du panneau : activé = vrai avec l'ID client/le secret renseigné (le point de terminaison du fournisseur renvoie activé : vrai)

## 6. Comparaison GitHub

Les applications OAuth GitHub n'ont **aucun** état de publication ni flux de vérification : créez l'application, définissez le rappel URL et elle fonctionne publiquement.
Notez qu'une application OAuth GitHub n'accepte qu'un **un** rappel d'autorisation URL (correspondance de préfixe) ; pour plusieurs domaines,
utilisez l'approche multi-URI de Google ou créez une application distincte par domaine de portail.
