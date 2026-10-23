<!-- source: subscription.md | locale: fr | updated: 2026-10-23 -->
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
| `Profile-Web-Page-Url` | `https://…/portal` | Lien vers la page du portail (« page Web ouverte » chez les clients) |
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

## 6. Remarques

- **Le format et la configuration sont étroitement couplés** : le modèle sélectionné par `config`/`platform` doit correspondre au `format` de cette requête ; les modèles incompatibles sont ignorés — cela empêche l'injection de règles Mihomo dans sing-box/Xray JSON.
- **Limitation de débit** : le point de terminaison d'abonnement a sa propre limite (`SubscribeRateLimit`) ; les clients abusifs sont rejetés.
- **Journal d'accès** : chaque récupération enregistre les informations IP / UA / CDN, visibles sous « Journal d'accès aux abonnements » dans l'interface utilisateur d'administration.
- Sans domaine configuré sur un port d'abonnement dédié, les requêtes `X-Forwarded-Proto` / `X-Forwarded-Host` sont utilisées ; assurez-vous que votre proxy inverse transmet les deux en-têtes.
