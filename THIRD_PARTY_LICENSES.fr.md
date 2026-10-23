<!-- source: THIRD_PARTY_LICENSES.md | locale: fr | updated: 2026-10-23 -->
# Composants et licences tiers

> Langues : [简体中文](THIRD_PARTY_LICENSES.md) · [English](THIRD_PARTY_LICENSES.en.md) · [繁體中文](THIRD_PARTY_LICENSES.zh-TW.md) · [Русский](THIRD_PARTY_LICENSES.ru.md) · [日本語](THIRD_PARTY_LICENSES.ja.md) · [한국어](THIRD_PARTY_LICENSES.ko.md) · [Deutsch](THIRD_PARTY_LICENSES.de.md) · Français · [Español](THIRD_PARTY_LICENSES.es.md) · [Português](THIRD_PARTY_LICENSES.pt.md) · [Türkçe](THIRD_PARTY_LICENSES.tr.md) · [Bahasa Indonesia](THIRD_PARTY_LICENSES.id.md) · [ไทย](THIRD_PARTY_LICENSES.th.md) · [Tiếng Việt](THIRD_PARTY_LICENSES.vi.md) · [العربية](THIRD_PARTY_LICENSES.ar.md) · [فارسی](THIRD_PARTY_LICENSES.fa.md)

CoreFusion utilise les composants open source suivants. Ce fichier répertorie les principaux composants et licences ; **la liste complète est définie par `go.mod` / `go.sum` et le fichier LICENSE de chaque module**.

Lors de la distribution des binaires de ce projet, conservez ce fichier, le projet `LICENSE` et les mentions de copyright/licence de tous les composants tiers.

## 1. Aller aux dépendances compilées dans le binaire (principal)

| Composant | Licence | Objectif |
|---|---|---|
| `github.com/xtls/xray-core` | MPL-2.0 | Analyse de la configuration Xray, commandes gRPC, définitions de protocole |
| `github.com/xtls/reality` | MPL-2.0 | Prise en charge du protocole REALITY |
| `github.com/sagernet/sing` | **GPL-3.0 ou version ultérieure** | Lié indirectement via le noyau xray |
| `github.com/sagernet/sing-shadowsocks` | **GPL-3.0 ou version ultérieure** | Lié indirectement via le noyau xray (Shadowsocks-2022) |
| `github.com/go-sql-driver/mysql` | MPL-2.0 | Pilote MySQL |
| `github.com/hashicorp/golang-lru/v2` | MPL-2.0 | Cache LRU |
| `github.com/go-acme/lego/v4` | MIT | Délivrance du certificat ACME |
| `github.com/gin-gonic/gin` | MIT | Cadre HTTP |
| `gorm.io/gorm` | MIT | ORM |
| `github.com/aws/aws-sdk-go-v2` | Apache-2.0 | Sauvegardes S3, Route53 DNS-01 |
| `google.golang.org/grpc` | Apache-2.0 | Communication gRPC de l'agent |
| `google.golang.org/protobuf` | Clause BSD-3 | Exécution Protobuf |
| `golang.org/x/crypto` | Clause BSD-3 | Primitives cryptographiques |
| `golang.org/x/oauth2` | Clause BSD-3 | OAuth2 / Google EAB |
| `github.com/golang-jwt/jwt/v5` | MIT | JWT |
| `github.com/google/uuid` | Clause BSD-3 | UUID |

> ⚠ **Composants GPL-3.0** : `sing` et `sing-shadowsocks` sont GPL-3.0 ou version ultérieure. Ils sont liés statiquement aux binaires du panneau et de l'agent, donc **la distribution des binaires nécessite de fournir la source correspondante** (voir la section 3 de [LEGAL.md](THIRD_PARTY_LICENSES.fr.md)).

## 2. Programmes autonomes invoqués lors de l'exécution (non distribués dans ce référentiel)

Le panneau/l'agent peut télécharger, installer ou appeler les programmes autonomes suivants au moment de l'exécution. Ils interagissent avec ce projet via des processus/interfaces, mais si vous les expédiez avec vos livrables, vous devez toujours vous conformer à leurs licences :

| Programme | Licence | Remarques |
|---|---|---|
| Xray-noyau | MPL-2.0 | Noyau proxy, processus distinct |
| sing-box | **GPL-3.0 ou version ultérieure** | Noyau proxy, processus distinct ; fournir la source correspondante lors de la distribution |
| Caddie | Apache-2.0 | Proxy inverse / TLS |
| Nginx | Clause BSD-2 | Proxy inverse (installé sur le système, si utilisé) |

> Hysteria2 et TUIC sont des **protocoles portés par sing-box**, et non des cœurs ou des binaires autonomes, ils ne sont donc pas répertoriés séparément.

## 3. Fichiers de données

| Données | Source/licence |
|---|---|
| `geoip.dat` / `geosite.dat` | Tel que déclaré par le référentiel de la source de données (généralement GPL-3.0 ou CC-BY-SA) |
| GéoLite2 (`GeoLite2-ASN.mmdb`, etc.) | CLUF MaxMind GeoLite2 / CC BY-SA 4.0 ; vérifier la licence avant une utilisation commerciale |
| `country.mmdb` | Comme déclaré par sa source de données |

## 4. Recommandations de conformité

- Lors de la livraison de fichiers binaires, incluez : `LICENSE`, ce fichier et le texte de licence ou d'acquisition URL de chaque composant GPL/MPL.
- Si vous modifiez les fichiers source des composants MPL-2.0 tels que `xray-core`, publiez la source des fichiers modifiés.
- Si vous distribuez des programmes GPL tels que sing-box avec votre produit, fournissez aux destinataires la source complète correspondante de cette version (ou une offre écrite efficace).
- Marques et noms commerciaux : n'utilisez pas les noms ou logos des projets en amont d'une manière qui implique une approbation officielle.
