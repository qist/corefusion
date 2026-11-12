<!-- source: subscription.md | locale: fr | updated: 2026-11-12 -->
# Abonnement API (public)

> Langues : [简体中文](subscription.md) · [English](subscription.en.md) · [繁體中文](subscription.zh-TW.md) · [Русский](subscription.ru.md) · [日本語](subscription.ja.md) · [한국어](subscription.ko.md) · [Deutsch](subscription.de.md) · Français · [Español](subscription.es.md) · [Português](subscription.pt.md) · [Türkçe](subscription.tr.md) · [Bahasa Indonesia](subscription.id.md) · [ไทย](subscription.th.md) · [Tiếng Việt](subscription.vi.md) · [العربية](subscription.ar.md) · [فارسی](subscription.fa.md)

Récupère la liste des nœuds d'abonnement d'un utilisateur. **Il s'agit du point de terminaison le plus utilisé par les utilisateurs finaux** ; aucun en-tête n'est requis — l'accès est accordé par le jeton dans le URL.

## 1. Point de terminaison

| Déploiement | URL |
|---|---|
| Panneau principal | `GET https://<panel-domain>/sub/<token>` |
| Port d'abonnement dédié | `GET https://<sub-domain>:<port>/<token>` (pas de préfixe `/sub`) |

`<token>` est le jeton d'accès unique d'un abonnement, généré par le côté administrateur (visible sous « Mes abonnements » dans le portail).

## 2. Paramètres de requête

| Paramètre | Descriptif |
|---|---|
| `format` | Format de sortie ; remplace la valeur par défaut de l'abonnement. Voir ci-dessous |
| `config` | Nom du modèle de configuration (défini sous « Formats d'abonnement » dans l'interface utilisateur d'administration). Doit utiliser le même format que le `format` de cette requête, sinon il est ignoré |
| `platform` | Faire correspondre un modèle de configuration par nom de plate-forme (utilisé lorsque `config` est omis) ; le format doit également correspondre |
| `providers` | `mihomo_yaml` uniquement : `1` / `true` active le **Mode fournisseur** (émet des `proxy-providers` et les référence via `use`) ; par défaut ou toute autre valeur conserve les nœuds intégrés sous `proxies`. Voir §6 |

### Valeurs `format` prises en charge

| Valeur | Sortie | Clientèle |
|---|---|---|
| `base64_uri` (par défaut) | Liens de partage joints à une nouvelle ligne, codés en base64 dans leur ensemble | V2RayN, Shadowrocket, Surge et autres clients génériques |
| `raw_uri` | Liens de partage joints à une nouvelle ligne, texte brut | Clients qui ont besoin de liens simples/débogage |
| `mihomo_yaml` | Configuration complète de Clash/Mihomo YAML (avec groupes proxy, règles) | Clash.Meta / mihomo / Clash Verge Rev |
| `mihomo_proxies_yaml` | YAML avec uniquement la liste de nœuds `proxies:` | source du fournisseur de proxy mihomo (avec actualisation programmée) |
| `singbox_json` | Configuration complète sing-box JSON | sing-box/SFI/FLClash (noyau sing-box) |
| `xray_json` | Configuration complète Xray JSON | Xray-noyau |

> La compatibilité protocole/format s'applique : par exemple, `anytls` n'apparaît qu'aux formats sing-box/Mihomo ; Les nœuds REALITY sur un serveur sing-box n'apparaissent pas dans les abonnements Xray/sing-box JSON. Les nœuds que le client ne peut pas reconnaître sont automatiquement supprimés au lieu de faire échouer l'intégralité de l'abonnement.

## 3. En-têtes de réponse

| En-tête | Exemple | Descriptif |
|---|---|---|
| `Subscription-Userinfo` | `upload=1024; download=2048; total=1073741824; expire=1760000000` | Utilisation et expiration (octets / secondes Unix). **Également envoyé sur 403**, afin que les clients puissent montrer l'allocation restante |
| `Profile-Update-Interval` | `6` | Intervalle d'actualisation suggéré (heures) |
| `Profile-Title` | `CoreFusion` | Nom du profil (affiché par Clash Verge Rev / V2RayN, etc.) |
| `Profile-Web-Page-Url` | `https://portal.example.com` | Portail URL (« ouvrir la page web » dans les clients ; chemin racine lorsque le portail possède son propre port, sinon /p) |
| `Content-Type` | Par format : `text/plain` / `text/yaml` / `application/json` |  |
| `Content-Disposition` | `attachment; filename="subscription.txt"` |  |

## 4. Codes d'état

| HTTP | coder | Signification |
|---|---|---|
| 200 | — | Normal (le corps est l'abonnement rendu dans `format`) |
| 400 | `MISSING_TOKEN` | Aucun jeton dans le URL |
| 403 | (texte brut) | Abonnement suspendu / expiré / inactif. Le message est localisé (zh/en/ja/ko) via `Accept-Language` et constitue un motif sur une seule ligne. |
| 404 | `SUBSCRIPTION_NOT_FOUND` | Le jeton n'existe pas |

> Les abonnements révoqués sont traités comme inexistants (404).

## 5. Exemples

```bash
# 1) Fetch with the default format
curl -L "https://panel.example.com/sub/abcd1234efgh5678"

# 2) Request Mihomo YAML with the "clash-global" config template
curl -L "https://panel.example.com/sub/abcd1234efgh5678?format=mihomo_yaml&config=clash-global"

# 3) Node list only (for a mihomo proxy-provider)
curl -L "https://panel.example.com/sub/abcd1234efgh5678?format=mihomo_proxies_yaml"

# 4) Check usage (headers only)
curl -sI "https://panel.example.com/sub/abcd1234efgh5678" | grep -i subscription-userinfo

# 5) Clash dynamic subscription: config template + Provider mode
curl -L "https://sub.example.com/abcd1234efgh5678?format=clash&config=out&providers=1"

# 6) sing-box / Xray with their own config templates (rules come from the template)
curl -L "https://sub.example.com/abcd1234efgh5678?format=singbox_json&config=sing-box-home"
curl -L "https://sub.example.com/abcd1234efgh5678?format=xray_json&config=xray-home"
```

Abonnement dynamique Clash/Mihomo (actualisation périodique) :

```yaml
proxy-providers:
  subscription-nodes:
    type: http
    url: "https://panel.example.com/sub/abcd1234efgh5678?format=mihomo_proxies_yaml"
    interval: 3600
    health-check:
      enable: true
      url: https://www.gstatic.com/generate_204
      interval: 300

proxy-groups:
  - name: PROXY
    type: select
    use:
      - subscription-nodes
```

## 6. Mode fournisseur et plusieurs domaines d'abonnement

### 6.1 Ce qu'émet le `providers=1`

Avec `format=mihomo_yaml&providers=1` (ou `format=clash&providers=1`) le YAML :

- n'intègre plus les nœuds au niveau supérieur - il émet `proxy-providers:` ;
- Les groupes proxy référencent les sources de nœuds via `use: [<provider-key>...]` ;
- tout `proxy-providers` écrit dans un format d'abonnement partagé (`mihomo_configs.config_json`) est **ignoré** (une configuration partagée sert plusieurs forfaits ; les fournisseurs codés en dur fuiraient entre les forfaits). Groupe → mappages sources en direct sous « Gestion du plan → groupe → mappages sources ».

Types de fournisseurs :

| Genre | Clé | Remarques |
|---|---|---|
| Interne | Nom du groupe de nœuds nettoyé | Généré par groupe de nœuds de plan ; contient uniquement les nœuds de ce groupe |
| Réservé | `subscription-nodes` | Tous les nœuds du plan ; sa source URL **omet** le paramètre `provider` |
| Externe | Nom configuré sur le forfait | Source externe (`http` YAML ou `file`) |

Champs communs par fournisseur :

- `url` — renvoie à cet abonnement avec `format=mihomo_proxies_yaml`, héritant de `config` / `platform` de la requête principale ; les fournisseurs internes ajoutent `provider=<key>` ;
- `path` — **toujours dérivé des « valeurs par défaut du fournisseur interne → modèle de chemin de cache »** (`./providers/{key}.yaml` par défaut, remplacé par `{key}`). Les fournisseurs externes ne peuvent pas non plus définir leur propre chemin ;
- `health-check` — `enable / url / interval` ; les fournisseurs internes héritent des valeurs par défaut, les fournisseurs externes les configurent sur le plan ;
- `interval` — intervalle de rafraîchissement en secondes.

### 6.2 Quel domaine va au fournisseur URL (multi-domaine)

« Centre d'abonnement → Points de terminaison d'abonnement » peut contenir **plusieurs** enregistrements (domaine + chemin + activé). Le fournisseur `url` sélectionne le domaine dans cet ordre :

1. **Le domaine réellement utilisé par cette requête** : la première valeur de `X-Forwarded-Host` (port supprimé), sinon `Host` ; il doit correspondre à un point de terminaison **activé** ;
2. Si rien ne correspond, revenez au point de terminaison activé avec l'**ID le plus bas** ;
3. Le chemin provient du point de terminaison correspondant (`path`, `/` par défaut), donnant `https://<domain><path>/<token>?format=mihomo_proxies_yaml...`.

Conséquences :

- Un client récupérant via un nouveau domaine obtient ce domaine au sein du fournisseur — **aucun changement de panneau n'est nécessaire** ;
- Un ancien domaine toujours activé dans la liste n'est plus codé en dur ;
- Si un client a mis en cache l'ancien fournisseur URL, la mise à jour de l'abonnement (ou l'effacement du cache du fournisseur/le redémarrage du client) le fait basculer.

Les proxys inverses doivent transmettre les `X-Forwarded-Proto` et `X-Forwarded-Host` ; sinon, le point de terminaison de secours est utilisé.

## 7. Remarques

- **Le format et la configuration sont étroitement couplés** : le modèle sélectionné par `config`/`platform` doit correspondre au `format` de cette requête ; les modèles incompatibles sont ignorés — cela empêche l'injection de règles Mihomo dans sing-box/Xray JSON.
- **`providers=1` n'affecte que Mihomo YAML** : sing-box / Xray / base64 / raw l'ignore (les nœuds restent en ligne).
- **Limitation de débit** : le point de terminaison d'abonnement a sa propre limite (`SubscribeRateLimit`) ; les clients abusifs sont rejetés.
- **Journal d'accès** : chaque récupération enregistre les informations IP / UA / CDN, visibles sous « Journal d'accès aux abonnements » dans l'interface utilisateur d'administration.
- Sans domaine configuré sur un port d'abonnement dédié, les requêtes `X-Forwarded-Proto` / `X-Forwarded-Host` sont utilisées ; assurez-vous que votre proxy inverse transmet les deux en-têtes.
- Dans les déploiements multi-domaines, gardez la liste des points de terminaison alignée sur les domaines que les clients utilisent réellement ; voir §6.2.
