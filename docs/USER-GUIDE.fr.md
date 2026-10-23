<!-- source: USER-GUIDE.md | locale: fr | updated: 2026-10-23 -->
# Guide d'utilisation CoreFusion

> Langues : [简体中文](USER-GUIDE.md) · [English](USER-GUIDE.en.md) · [繁體中文](USER-GUIDE.zh-TW.md) · [Русский](USER-GUIDE.ru.md) · [日本語](USER-GUIDE.ja.md) · [한국어](USER-GUIDE.ko.md) · [Deutsch](USER-GUIDE.de.md) · Français · [Español](USER-GUIDE.es.md) · [Português](USER-GUIDE.pt.md) · [Türkçe](USER-GUIDE.tr.md) · [Bahasa Indonesia](USER-GUIDE.id.md) · [ไทย](USER-GUIDE.th.md) · [Tiếng Việt](USER-GUIDE.vi.md) · [العربية](USER-GUIDE.ar.md) · [فارسی](USER-GUIDE.fa.md)

> Ce guide est destiné aux utilisateurs du CoreFusion et couvre l'intégralité du flux de travail, de l'installation aux opérations quotidiennes.

---

## Contenu

1. [Présentation du système](#1-présentation-du-système)
2. [Mise en place](#2-mise-en-place)
3. [Configuration initiale](#3-configuration-initiale)
4. [Tableau de bord](#4-tableau-de-bord)
5. [Gestion des nœuds](#5-gestion-des-nœuds)
6. [Centre de protocole](#6-centre-de-protocole)
7. [Gestion des procurations](#7-gestion-des-procurations)
8. [Ressources de routage](#8-ressources-de-routage)
9. [Centre d'abonnement](#9-centre-dabonnement)
10. [Gestion des clients](#10-gestion-des-clients)
11. [Gestion des régimes](#11-gestion-des-régimes)
12. [Gestion des recharges](#12-gestion-des-recharges)
13. [Gestion des billets](#13-gestion-des-billets)
14. [Gestion des certificats](#14-gestion-des-certificats)
15. [Quotas de trafic](#15-quotas-de-trafic)
16. [Proxy chaînés](#16-proxy-chaînés)
17. [Utilisateurs en ligne](#17-utilisateurs-en-ligne)
18. [Gestion des utilisateurs](#18-gestion-des-utilisateurs)
19. [Site portail](#19-site-portail)
20. [Paramètres système](#20-paramètres-système)
21. [Console d'opérations](#21-console-dopérations)
22. [Surveillance de base de données](#22-surveillance-de-base-de-données)
23. [Journal d'audit](#23-journal-daudit)
24. [Gestion des journaux](#24-gestion-des-journaux)
25. [Extensions d'opérations](#25-extensions-dopérations)
26. [FAQ](#26-faq)
- [Annexe](#annexe)

---

## 1. Présentation du système

CoreFusion est un panneau de gestion de proxy multiprotocole prenant en charge VLESS, VMess, Trojan, Shadowsocks, Hysteria2, TUIC et plus encore, avec gestion des nœuds, livraison d'abonnements, gestion des clients, facturation, certificats et proxys chaînés.

### 1.1 Fonctionnalités principales

| Module | Descriptif |
|--------|-------------|
| Multi-protocole | VLESS, VMess, Trojan, Shadowsocks, Hysteria2, TUIC, WireGuard, etc. |
| Multicœur | Xray-core et sing-box (Hysteria2/TUIC et autres protocoles QUIC exécutés sur sing-box) |
| Multi-nœuds | Cinq types de nœuds : autonome, miroir, actif/veille, entrée chaînée, sortie chaînée |
| Livraison d'abonnement | Plusieurs formats d'abonnement, points de terminaison et gestion des formats |
| Facturation | Modèles de plan, cartes de recharge, solde, commandes, commission revendeur |
| Certificats | Émission de Let's Encrypt, validation DNS-01/HTTP-01, distribution de certificats |
| Proxy chaîné | Équilibrage de charge multi-sorties, enregistrement de nœud externe, routage en chaîne |
| Quotas de trafic | Surveillance mensuelle du trafic par nœud, suspension automatique, réinitialisation des journaux |
| Portail | Éditeur de portail visuel, multilingue, modèles personnalisés |
| Notifications | Email et Telegram, modèles et diffusions |

### 1.2 Architecture

- **Backend** : Allez, gRPC
- **Frontend** : Vue 3 + Element Plus
- **Base de données** : SQLite (par défaut) / MySQL / PostgreSQL
- **Proxy** : Proxy inverse Caddy / Nginx
- **Port du panneau** : attribué par l'assistant de configuration (port libre aléatoire par défaut ; 8080 est un choix courant)
- **Port gRPC de l'agent** : 18443 par défaut

---

## 2. Mise en place

### 2.1 Environnements pris en charge

| Article | Assistance |
|------|---------|
| Distribution | Debian / Ubuntu, RHEL / CentOS / Rocky / AlmaLinux / Fedora, Alpine, Arch, openSUSE |
| Architectures CPU | Binaires officiels : `amd64`, `arm64` ; les versions sources prennent également en charge `armv7`, `armv6`, `386`, `ppc64le`, `s390x`, `riscv64`, `loong64` |
| Responsable des services | systemd (recommandé) ; revient à `nohup` dans les conteneurs ou autres environnements sans systemd |
| Mémoire/disque | ≥ 1 Go / ≥ 10 Go (la base de données, les certificats, les journaux et les binaires principaux se trouvent dans le répertoire de données) |
| Port du panneau | choisi dans l'assistant de configuration (port libre aléatoire par défaut ; `8080` et similaires conviennent) |
| Agent gRPC | `18443` par défaut |

Les commandes externes d'exécution sont installées automatiquement par l'installateur (`scripts/install.sh` et le script d'installation de l'agent généré par le panneau ; ignorez avec `--no-deps` / `CF_SKIP_DEPS=1`) :

| Objectif | Forfait |
|---------|---------|
| Pare-feu / limite de débit / marquage de concurrence | `nftables` (`nft`) |
| Mise en forme du trafic, liens et sockets | `iproute2` (`tc` / `ip` / `ss`) |
| Solution de secours lorsque NFT n'est pas disponible | `iptables` (`iptables-nft` sur EL8+, `iptables` sur EL7) |
| Suivi des connexions | `conntrack-tools` (`conntrack`) |
| Charge `nf_conntrack` / `ifb` | `kmod` (`modprobe` / `lsmod`) |
| Détection et nettoyage des processus | `procps` (`pgrep` / `pkill`) |
| Vérifications des ports / paramètres DH | `lsof` / `openssl` |
| Création d'un utilisateur proxy | `shadow-utils` (Debian `passwd`, Alpine/Arch `shadow`) |
| Téléchargements et archives | `curl` / `ca-certificates` / `tar` / `gzip` / `xz` |

> L'absence de `nft`/`tc` affecte uniquement les fonctionnalités de limitation de débit et de pare-feu ; le panneau démarre toujours.
> Exécutez d'abord `--cpu-info` pour voir comment l'environnement est détecté et quels packages sont manquants.

### 2.2 Installation sur une seule ligne (recommandée)

```bash
curl -fsSL https://raw.githubusercontent.com/qist/corefusion/main/scripts/install.sh -o install.sh
sudo bash install.sh
```

Ou dirigez-le directement (pas d'interaction, pas d'indicateurs supplémentaires) :

```bash
curl -fsSL https://raw.githubusercontent.com/qist/corefusion/main/scripts/install.sh | sudo bash
```

Le script va :

1. Détectez la distribution, l'architecture du processeur et la libc, puis installez les packages de base et les dépendances d'exécution ;
2. Téléchargez `corefusion-linux-<arch>` à partir des versions GitHub et vérifiez-le avec le `.sha256` correspondant ;
3. Installez sur `/opt/corefusion` et créez `data/{sqlite,certs,logs,runtime}` ;
4. Écrivez uniquement un espace réservé `.env` avec `SAFE_PATHS=data` — pas de configuration de base de données — afin que le premier démarrage entre dans l'assistant de configuration (le `.env` complet est généré par l'assistant) ;
5. Installez et démarrez un service systemd (ou exécutez-le sous `nohup` lorsque systemd n'est pas disponible) ;
6. Attendez d'être prêt, puis imprimez la configuration URL, les chemins de jeton et de répertoire.

Exemple de sortie :

```text
==================================================================
 CoreFusion installation complete
------------------------------------------------------------------
 Install dir : /opt/corefusion
 Data dir    : /opt/corefusion/data
 Binary      : /opt/corefusion/corefusion
 Logs        : journalctl -u corefusion -f
------------------------------------------------------------------
 Setup wizard : http://<server-ip>:<random-port>/init?init_token=<token>
 Setup token  : <token>
 Valid for    : 2 hours (see startup log)
 Note         : the token is shown once; after setup this port closes and the panel switches to its normal port.
==================================================================
```

Si le panneau écoute uniquement sur `127.0.0.1` (par exemple derrière un proxy inverse), le script imprime une commande de tunnel SSH au lieu d'un URL inaccessible.

> Vous avez perdu le jeton ? Relisez le journal de démarrage : `journalctl -u corefusion -n 50 --no-pager`
> (sans systemd : `tail -n 50 /opt/corefusion/data/logs/panel.log`).

### 2.3 Épinglage de version / miroirs / installation hors ligne

```bash
# Pin a version
sudo bash install.sh --version v0.13.0-alpha.2

# GitHub download mirror (used as a URL prefix)
sudo bash install.sh --mirror https://ghproxy.net --version latest

# Use a local binary (offline / intranet / pre-built)
sudo bash install.sh --local ./dist/corefusion-linux-arm64

# Explicit target architecture
sudo bash install.sh --arch arm64 --local ./corefusion-linux-arm64
```

> Sans `--version`, le script utilise l'entrée la plus récente dans la liste des versions GitHub.
> La plupart des versions sont des versions préliminaires, le point de terminaison `/releases/latest` n'est donc pas utilisé.
>
> Les versions officielles fournissent uniquement `amd64` / `arm64` ; utilisez `--source` ou un binaire compilé de manière croisée avec `--local` pour d'autres architectures.

### 2.4 Construire à partir des sources

```bash
sudo bash install.sh --source --version v0.13.0-alpha.2
```

`--source` clone le référentiel (ou utilise l'extraction existante lorsque le script s'y trouve) et exécute `make panel`. Les versions de la chaîne d'outils sont choisies par environnement :

| Dépendance | Sélection |
|------------|-----------|
| Allez | Lecture de `toolchain` / `go` dans `go.mod` (actuellement **Go 1.27+**) ; si la version du système est trop ancienne, la chaîne d'outils correspondante est téléchargée sur `<install-dir>/.toolchain` |
| Nœud | **Nœud 22 LTS** par défaut (Vite nécessite ≥ 18) ; une version officielle correspondante est téléchargée en cas de besoin |
| Nœud (glibc < 2.28) | Utilise la version `unofficial-builds` glibc-217 (par exemple CentOS 7) |
| Nœud (Alpin / Musl) | Utilise la distribution `nodejs` / `npm` |

Remplacements :

```bash
# Force Go / Node versions
sudo CF_GO_VERSION=1.27.5 CF_NODE_MAJOR=22 bash install.sh --source

# Go toolchain mirror (default https://go.dev/dl)
sudo CF_GO_MIRROR=https://golang.google.cn/dl bash install.sh --source
```

Construction manuelle (développement/débogage) :

```bash
git clone https://github.com/qist/corefusion.git
cd corefusion

# Frontend + panel for the current architecture; the frontend is embedded via go:embed
make panel

# Cross-compile
make panel-linux-amd64
make panel-linux-arm64
```

> `go build ./...` ne produit pas de binaire. Construisez le panneau avec
> `go build -o corefusion ./cmd/corefusion/` ou `make panel`.

### 2.5 Indicateurs de l'installateur

| Drapeau | Descriptif |
|------|-------------|
| `-d, --dir <PATH>` | Répertoire d'installation, par défaut `/opt/corefusion` |
| `-v, --version <TAG>` | Version panneau, par ex. `v0.13.0-alpha.2` ; par défaut `latest` |
| `--binary` | Utiliser le binaire GitHub Release (par défaut) |
| `--source` | Construire à partir des sources et préparer la correspondance Go/Node |
| `--local <FILE>` | Utiliser un binaire local (installation hors ligne) |
| `--arch <ARCH>` | Architecture cible : `amd64` / `arm64` / `armv7` / `armv6` / `386` / `ppc64le` / `s390x` / `riscv64` / `loong64` |
| `--mirror <URL>` | Préfixe de téléchargement GitHub, par ex. `https://ghproxy.net` |
| `--repo <URL>` | Dépôt source, `https://github.com/qist/corefusion.git` par défaut |
| `--ref <REF>` | Branche/balise source ; par défaut, `--version`, sinon `main` |
| `--no-deps` | Ignorer l'installation des dépendances du système |
| `--no-start` | Installer et écrire le fichier de service sans démarrer |
| `--cpu-info` | Détection de l'environnement d'impression et dépendances manquantes, puis quittez |
| `--uninstall` | Arrêtez le service et supprimez le programme (conserve les données et `.env`) |
| `--purge` | Avec `--uninstall`, supprimez également le répertoire de données et `.env` |

Variables d'environnement associées :

| Variable | Objectif |
|----------|---------|
| `CF_GO_VERSION` | Remplacer la version Go requise pour les builds sources |
| `CF_NODE_MAJOR` | Remplacer la version majeure du nœud (22 par défaut) |
| `CF_GO_MIRROR` | Aller au miroir de téléchargement de la chaîne d'outils |
| `CF_SKIP_DEPS` | Définissez sur `1` dans le script d'installation de l'agent pour ignorer l'installation des dépendances. |

### 2.6 Disposition du répertoire

```text
/opt/corefusion/
├── corefusion                 # panel binary
├── corefusion.bak             # previous version (when upgrading)
├── .env                       # environment file (0600, generated by the setup wizard)
├── .toolchain/                # Go / Node downloaded for source builds
└── data/                      # data directory (SAFE_PATHS)
    ├── sqlite/app.db          # default SQLite database
    ├── certs/                 # certificates and keys
    ├── logs/                  # runtime logs (without systemd)
    ├── runtime/               # core runtime config and state
    ├── binaries/              # downloaded xray / sing-box cores
    └── proxy/                 # proxy (Caddy/Nginx) sites and config
```

> Le programme d'installation crée uniquement `sqlite/`, `certs/`, `logs/` et `runtime/` ; `binaries/`, `proxy/`, `rules/` et autres sont créés par le panneau ou lorsque les noyaux sont installés.
> Le répertoire de données provient de `SAFE_PATHS` dans `.env` et peut être un chemin absolu. La modification du répertoire d'installation ne déplace pas les données existantes.

### 2.7 Gestion des services

systemd (la plupart des distributions) :

```bash
systemctl status corefusion --no-pager
systemctl restart corefusion
systemctl stop corefusion
journalctl -u corefusion -f          # follow logs
journalctl -u corefusion -n 100      # last 100 lines
```

Sans systemd (conteneurs, etc.) :

```bash
cd /opt/corefusion && ./corefusion            # foreground (debugging)
cd /opt/corefusion && setsid nohup ./corefusion >> data/logs/panel.log 2>&1 &
tail -f data/logs/panel.log
```

> Sans systemd, le processus ne démarre pas au démarrage ; redémarrez-le manuellement après un redémarrage (ou réexécutez le programme d'installation).

### 2.8 Mise à niveau et désinstallation

Mise à niveau (conserve les données et la configuration) :

```bash
sudo bash install.sh --version <new-version>
```

Le script arrête le service, sauvegarde l'ancien binaire sous le nom `corefusion.bak`, le remplace et redémarre. Les `.env` et `data/` existants restent intacts.

Désinstaller :

```bash
# Remove the program only; keep data and .env
sudo bash install.sh --uninstall

# Also remove the data directory and .env (irreversible)
sudo bash install.sh --uninstall --purge
```

### 2.9 Fichier d'environnement (.env)

Le `.env` complet est **généré par l'assistant d'installation** lors de l'enregistrement de la configuration (DSN de la base de données, répertoire de données, secrets, ports, domaines).

Lors d'une nouvelle installation, le programme d'installation écrit uniquement cet espace réservé, afin que le panneau ne confonde pas le `data/` existant avec une installation initialisée et ignore l'assistant :

```env
SAFE_PATHS=data
```

> Lorsqu'une configuration complète (`.env` avec `CF_DB_DSN`) existe déjà, le programme d'installation ne l'écrase jamais et l'assistant n'apparaît plus.
> Les modifications manuelles ultérieures apportées à `.env` prennent effet après un redémarrage.

#### Répertoire de données

```env
SAFE_PATHS=data                       # relative to the install directory
# SAFE_PATHS=/srv/corefusion-data     # or an absolute path
```

#### Base de données

`CF_DB_DSN` est détecté par le préfixe :

| Préfixe | Base de données |
|--------|----------|
| non défini ou un chemin nu | SQLite (par défaut) |
| `sqlite://` | SQLite (explicite) |
| `mysql://` | MySQL |
| `postgres://` / `postgresql://` | PostgreSQL |

`CF_DB_TYPE` est facultatif et remplace la détection (`sqlite` / `mysql` / `postgres`).

> **Mots de passe cryptés** : DSN MySQL/PostgreSQL écrits par le panneau (assistant de configuration/commutateur de base de données)
> sont stockés dans `.env` sous le nom `enc:...` (AES-256-GCM, clé `CF_SECRET_ENCRYPTION_KEY`).
> Ils sont décryptés en lecture ; Les DSN en texte clair écrits à la main fonctionnent toujours.

**SQLite (par défaut)**

```env
# Bare path (detected as SQLite)
CF_DB_DSN=data/sqlite/app.db

# Or explicit sqlite:// prefix
CF_DB_DSN=sqlite:///opt/corefusion/data/sqlite/app.db
```

**MySQL**

```env
# Format: mysql://user:password@tcp(host:port)/dbname?params
CF_DB_DSN=mysql://corefusion:password@tcp(127.0.0.1:3306)/corefusion?charset=utf8mb4&parseTime=True&loc=Local
```

**PostgreSQL**

```env
# Format: postgres://user:password@host:port/dbname?params
CF_DB_DSN=postgres://corefusion:password@127.0.0.1:5432/corefusion?sslmode=disable
```

#### Pool de connexions (en option, MySQL/PostgreSQL)

```env
CF_DB_MAX_OPEN_CONNS=100
CF_DB_MAX_IDLE_CONNS=10
```

#### Autres variables courantes

| Variable | Descriptif |
|----------|-------------|
| `JWT_SECRET` | Clé de signature pour les jetons de connexion ; généré au premier démarrage en cas d'absence |
| `CF_SECRET_ENCRYPTION_KEY` | Clé de cryptage des paramètres sensibles (AES-256-GCM) ; généré en cas d'absence |
| `CF_INITIAL_ADMIN_PASSWORD` | Mot de passe administrateur initial (facultatif ; aléatoire et imprimé une fois par défaut) |
| `CF_NETWORK_TRAFFIC_ACCOUNTING` | Définissez sur `0` pour désactiver la comptabilité du trafic basée sur iptables (activée par défaut) |
| `CF_NETWORK_TRAFFIC_TABLE` | Table iptables utilisée pour la comptabilité du trafic, par défaut `corefusion` |

> Les proxys inverses de confiance pour résoudre les adresses IP des clients réels ne sont plus configurés via des variables d'environnement ;
> ils sont stockés sous le nom `panel.trusted_proxies` dans **Paramètres → Réseau**.

> `.env` contient des secrets ; le panneau resserre ses autorisations sur `0600` au démarrage. Ne le commettez jamais.

### 2.10 Docker (facultatif, créez votre propre image)

Le référentiel ne livre **pas** de Dockerfile. Si vous le conteneurisez, écrivez votre propre image. Invocation de référence :

```bash
docker run -d \
  --name corefusion \
  -p 8080:8080 \
  -p 18443:18443 \
  -v /opt/corefusion/data:/app/data \
  -v /opt/corefusion/.env:/app/.env \
  <your-image>
```

> Les conteneurs n'ont généralement pas de privilèges systemd ni `nft`, donc les fonctionnalités de limitation de débit/pare-feu ne fonctionneront pas.
> Le répertoire de travail doit correspondre à la base du relatif `SAFE_PATHS`.

### 2.11 Vérifications post-installation

```bash
# Environment and dependencies
sudo bash install.sh --cpu-info

# Service state
systemctl is-active corefusion

# Startup log (contains the setup URL and token)
journalctl -u corefusion -n 50 --no-pager

# Listening ports
ss -lntp | grep corefusion
```

---

## 3. Configuration initiale

Au premier démarrage, si `.env` n'a pas de `CF_DB_DSN` valide, le panneau ne dessert **pas** encore l'interface utilisateur d'administration. Cela démarre le
**Assistant de configuration** sur un port aléatoire entre 18000 et 18999 avec un jeton unique, puis redémarre comme le panneau normal une fois enregistré.

### 3.1 Ouvrez l'assistant de configuration

1. Copiez « l'assistant d'installation » URL à partir de la sortie du programme d'installation ; ou
2. Lisez le journal sur le serveur :

```bash
journalctl -u corefusion -n 50 --no-pager | grep -A2 'setup wizard'
```

```text
CoreFusion first start - setup wizard
  URL   : http://<ip>:<port>/init?init_token=<token>
  Token : <token>
  Valid : 2h0m0s
```

3. Ouvrez le URL dans un navigateur (il contient déjà le `init_token`, aucune saisie manuelle n'est donc nécessaire).

> La session de l'assistant expire après **2 heures** par défaut. S'il expire ou si le panneau redémarre, procurez-vous-en un nouveau.
> Ne partagez pas le jeton. Une fois la configuration terminée, ce port se ferme et seul le port normal du panneau reste.

### 3.2 Étapes de l'assistant

L'assistant comporte 8 étapes. Vous pouvez revenir à tout moment ; rien n'est engagé jusqu'à la sauvegarde finale.

| Étape | Contenu |
|------|---------|
| 1. Base de données | SQLite / MySQL / PostgreSQL |
| 2. Répertoire de données | Écrit à `SAFE_PATHS` dans `.env` |
| 3. Clés de cryptage | `JWT_SECRET` et `CF_SECRET_ENCRYPTION_KEY` |
| 4. Noyaux | Quels cœurs proxy installer |
| 5. Mandataire | Aucun/Nginx/Caddy |
| 6. Sites et domaines | Ports, chemin Web, compte administrateur, devise, fuseau horaire, domaines |
| 7. Certificat HTTPS | Désactivé / téléchargement / ACME |
| 8. Résumé | Vérifiez, téléchargez `.env`, enregistrez et redémarrez |

#### Étape 1 : Base de données

- **SQLite (fichier local, recommandé)** : laissez le chemin vide pour utiliser `<data-dir>/sqlite/app.db`.
- **MySQL / PostgreSQL** : hôte, port, utilisateur, mot de passe, nom de la base de données ; PostgreSQL prend également en charge `disable` / `require` / `verify-full`.
- Cliquez sur « Tester la connexion » pour valider. La sauvegarde le teste également. Une base de données manquante est créée automatiquement (créez-la manuellement si le compte ne dispose pas de `CREATE`).
- Le DSN (y compris le mot de passe) est crypté avant d'être écrit sur `.env`.

#### Étape 2 : Répertoire de données

Stocke la base de données, les certificats, les journaux, les binaires principaux et bien plus encore ; il s'agit de `SAFE_PATHS` dans `.env`. La valeur par défaut est `data`
(par rapport au répertoire d'installation) ; un chemin absolu est autorisé. **Ne le modifiez pas par hasard plus tard**, ou le panneau
ne trouvera pas la base de données et les certificats existants.

#### Étape 3 : Clés de chiffrement

- `JWT_SECRET` : signe les jetons de connexion.
- `CF_SECRET_ENCRYPTION_KEY` : crypte les paramètres sensibles (mot de passe de la base de données, identifiants DNS, cartes de recharge, etc.).
- Les deux sont générés aléatoirement par défaut ; utilisez « Régénérer » pour obtenir de nouvelles valeurs. **La modification du `CF_SECRET_ENCRYPTION_KEY` rend les données précédemment chiffrées illisibles** — sauvegardez-les.

#### Étape 4 : Cœurs

Sélectionnez les cœurs proxy à installer :

- **Xray** : VLESS / VMess / Trojan / Shadowsocks, etc. ;
- **sing-box** : supporte les protocoles Hysteria2 / TUIC et d'autres protocoles QUIC.

Après avoir enregistré et redémarré, le panneau les télécharge et les installe automatiquement. Vérifiez les versions et l'état sous « Protocoles ». Vous pouvez également les installer plus tard.

#### Étape 5 : Procuration

- **Aucun** : le panel sert directement les clients (gérez vous-même le TLS).
- **Nginx / Caddy** : le panel gère le reverse proxy et l'hébergement des certificats ; choisissez-en un.

Avec un proxy sélectionné, le panneau écoute généralement uniquement sur `127.0.0.1` et le proxy dessert le monde extérieur.

#### Étape 6 : Sites et domaines

| Champ | Descriptif |
|-------|-------------|
| Adresse d'écoute | Dérivé automatiquement : proxy → `127.0.0.1` ; pas de proxy avec certificat → `0.0.0.0` ; sinon → `127.0.0.1` |
| Port du panneau | Port gratuit aléatoire par défaut ; changez-le si vous le souhaitez (par exemple 8080) |
| Port d'abonnement | Facultatif ; `0` partage le port du panneau |
| Port portail | Facultatif; `0` partage le port du panneau |
| Chemin Internet | Chemin de l'interface utilisateur d'administration, `/` par défaut ; le API est toujours `/api/v1` |
| Nom d'utilisateur/mot de passe administrateur | Aléatoire par défaut ; **le mot de passe n'est affiché qu'une seule fois dans le résumé** — enregistrez-le |
| Mode d'accès | « Domaine » ou « IP » |
| Devise/fuseau horaire par défaut | Devise par défaut pour les cartes de recharge et les soldes ; Fuseau horaire du panneau et du journal proxy (par exemple `Asia/Shanghai`) |
| Ajouter au proxy inverse | Lorsqu'un proxy et des domaines sont définis, ajoutez des domaines de panneau/abonnement/portail au proxy en une seule fois. |

La saisie des domaines écrit les paramètres correspondants (domaine de rappel, point de terminaison d'abonnement, domaine de partage).

#### Étape 7 : certificat HTTPS

| Origine | Descriptif |
|--------|-------------|
| Désactiver HTTPS | Le panel/portail/abonnements sont servis sur HTTP (non sécurisé) ; avec un proxy inverse, vous pouvez ajouter TLS côté proxy plus tard |
| Télécharger le certificat | Coller le certificat PEM et la clé privée PEM (mode d'accès au domaine uniquement) |
| ACME | Émet le certificat lorsque vous soumettez cette étape ; prend en charge les certificats de domaine et IP |

Options du ACME :

- **CA** : Let's Encrypt, Let's Encrypt (mise en scène), ZeroSSL, Google CA publique ;
- **Cible** : le domaine configuré, ou l'IP publique (les certificats IP sont pris en charge par Let's Encrypt uniquement) ;
- **Défi** : `HTTP-01` (nécessite le port 80) ou `DNS-01` (caractères génériques, pas de port 80 ; choisissez un fournisseur DNS et saisissez les informations d'identification) ;
- **Type/taille de clé** : ECC 256/384 ou RSA 2048/3072/4096 ;
- **Autorité de certification publique Google** : nécessite EAB ; saisissez Key ID / HMAC manuellement ou utilisez un compte de service GCP.

> Les certificats IP détectent automatiquement l'IPv4/IPv6 public de l'hôte. Derrière un proxy, l'adresse détectée est celle de l'hôte et non celle de la sortie du proxy.
> Les certificats de domaine peuvent également être délivrés ultérieurement sous « Certificats ».

#### Étape 8 : Résumé

Affiche la base de données, le répertoire de données, les cœurs, le proxy, les ports, le compte administrateur, les domaines et les clés `.env` (secrets masqués) et fournit :

- **Télécharger .env** : conserver une copie de la configuration ;
- **URL d'accès** : URL finales du panel / de l'abonnement / du portail ; HTTP est marqué comme non sécurisé.

Cliquez sur « Enregistrer et redémarrer » pour écrire la base de données et `.env` ; le panneau redémarre en service normal.

### 3.3 Après la configuration

- Le service redémarre et le journal de démarrage imprime l'adresse d'écoute finale ;
- Connectez-vous avec le compte administrateur, accédez au URL et au chemin Web à partir du résumé ;
- Si le mot de passe est perdu, réinitialisez-le hors ligne avec `./corefusion admin reset-password`.

### 3.4 Configuration non interactive du CLI

Pour les serveurs d'automatisation ou sans tête, écrivez la configuration et marquez la configuration terminée directement :

```bash
cd /opt/corefusion
./corefusion init \
  --db-type sqlite \
  --data-dir /opt/corefusion/data \
  --admin-username admin \
  --admin-password 'change-me-strong-password' \
  --listen-port 8080 \
  --proxy-type caddy \
  --cores xray,singbox \
  --panel-domain panel.example.com \
  --tls-mode acme --tls-domain panel.example.com --tls-email you@example.com
```

Drapeaux courants :

| Drapeau | Descriptif |
|------|-------------|
| `--db-type` / `--db-host` / `--db-port` / `--db-user` / `--db-password` / `--db-name` | Paramètres de la base de données |
| `--data-dir` | Répertoire de données ; par défaut, `SAFE_PATHS` ou `./data` |
| `--listen-port` / `--subscription-port` / `--portal-port` / `--web-path` | Ports et chemin web (port libre aléatoire par défaut) |
| `--admin-username` / `--admin-password` | Compte administrateur (mot de passe aléatoire imprimé une fois en cas d'omission) |
| `--proxy-type` | `none` / `nginx` / `caddy` |
| `--cores` | Noyaux à installer, séparés par des virgules : `xray,singbox` |
| `--panel-domain` / `--subscription-domain` / `--portal-domain` | Domaines (écrits dans les paramètres et ajoutés au proxy inverse) |
| `--tls-mode` | `none` / `self_signed` / `acme` |
| `--tls-domain` / `--tls-email` / `--tls-challenge` / `--tls-key-type` / `--tls-key-bits` | Options de certificat (`--tls-challenge` est par défaut `http-01` ; `dns-01` nécessite `--tls-dns-channel` ou `--tls-dns-provider`) |
| `--tls-ip-cert` | Émettre un certificat pour l'adresse IP publique détectée (Let's Encrypt uniquement) |
| `--acme-provider` | `lets_encrypt` / `lets_encrypt_staging` / `zerossl` / `google` |
| `--currency` / `--timezone` | Devise/fuseau horaire par défaut |

Liste complète : `./corefusion init --help`. Démarrez ensuite le service :

```bash
systemctl restart corefusion
```

### 3.5 Réexécution de l'assistant de configuration

Pour recommencer (supprime `.env` et les marques de configuration, **conserve les données**) :

```bash
cd /opt/corefusion
./corefusion reset-init      # type RESET to confirm
systemctl restart corefusion
```

Utilisez ensuite le nouveau URL et le jeton du journal de démarrage.

> La réinitialisation réinitialise le compte administrateur et le chemin Web. Les données professionnelles de l'ancienne base de données sont conservées, mais vous devrez peut-être recréer le compte de connexion.
>
> `reset-init` supprime l'intégralité du `.env` (y compris `SAFE_PATHS`). **Si votre répertoire de données est un chemin absolu personnalisé**, restaurez `SAFE_PATHS` avant de redémarrer, sinon le panneau apparaît dans `./data` :
>
> ```bash
> printf 'SAFE_PATHS=/srv/corefusion-data\n' > /opt/corefusion/.env
> ```

### 3.6 Sécurité après la première connexion

#### Changer le mot de passe initial

1. Cliquez sur le nom d'utilisateur dans le coin supérieur droit ;
2. Choisissez « Changer le mot de passe » ;
3. Entrez l'ancien et le nouveau mot de passe ;
4. Cliquez sur « Enregistrer ».

#### Activer la connexion par code dynamique (2FA / TOTP)

> Une fois activé, le compte passe en **login par code dynamique** : le mot de passe statique n'est plus accepté ; seulement le
> Le code à 6 chiffres de votre application d'authentification (ou un code de récupération unique) fonctionne. Ceci **remplace** le mot de passe de connexion — c'est
> pas de vérification en deux étapes « mot de passe + code ».

1. Cliquez sur le nom d'utilisateur dans le coin supérieur droit ;
2. Choisissez « Activer 2FA » ;
3. Entrez le mot de passe actuel (uniquement pour confirmer ce changement);
4. Scannez le code QR avec une application d'authentification (Google Authenticator / 1Password, etc.), ou saisissez la clé manuellement ;
5. Entrez le code à 6 chiffres pour terminer l'activation ;
6. **Enregistrez immédiatement les codes de récupération** (téléchargeables) ; chaque code fonctionne une fois.

Connectez-vous ensuite :

- Après avoir saisi le nom d'utilisateur, le champ du mot de passe devient « Code à 6 chiffres / code de récupération » et le bouton devient « Se connecter avec le code » ;
- Entrez le code actuel ; **le mot de passe statique ne peut plus être utilisé** ;
- Les codes de récupération sont à usage unique ; régénérez-les sous « Dynamic Code Login (2FA) » lorsqu’ils sont épuisés (cela invalide tous les anciens codes).

Si l'authentificateur est perdu et que les codes de récupération sont épuisés, désactivez 2FA hors ligne sur le serveur :

```bash
cd /opt/corefusion
./corefusion admin 2fa reset -u admin    # disables 2FA, clears the secret and recovery codes, revokes all sessions
```

> Un seul super-administrateur ne disposant ni de l'authentificateur ni des codes de récupération ne peut pas effectuer de récupération via l'interface utilisateur Web et doit utiliser la commande ci-dessus.

#### Enregistrez un Passkey (facultatif)

1. Cliquez sur le nom d'utilisateur dans le coin supérieur droit ;
2. Choisissez « Gestion Passkey » ;
3. Cliquez sur « Enregistrer le nouveau Passkey » ;
4. Complétez l'invite du périphérique.

> **Remarque** : Les clés d'accès nécessitent HTTPS ou localhost, et le domaine doit être sur la liste verte.

---

## 4. Tableau de bord

Le tableau de bord affiche la présentation du cluster multi-nœuds et l'état des nœuds.

### 4.1 Ce qu'il montre

- **Informations système** : nom d'hôte, version du système d'exploitation, disponibilité, processeur, mémoire
- **Trafic** : bande passante en temps réel, trafic total, tendance mensuelle
- **Santé des nœuds** : nombres en ligne/hors ligne/dégradés, nœuds à charge élevée
- **Alertes** : nœuds hors ligne, nœuds dégradés, certificats expirant bientôt, avertissements de quota
- **Ressources du cluster** : CPU, mémoire, disque et débit moyens

### 4.2 Actions

- Cliquez sur « Afficher les détails » pour ouvrir la page de détails d'un nœud.
- Cliquez sur une carte de nœud pour voir son statut en direct
- Utilisez les filtres pour affiner par statut, type ou région

---

## 5. Gestion des nœuds

La gestion des nœuds est au cœur du CoreFusion et prend en charge plusieurs types de nœuds.

### 5.1 Types de nœuds

| Tapez | Descriptif |
|------|-------------|
| Autonome (`standalone`) | Un nœud central indépendant |
| Miroir (`mirror`) | Synchronise la configuration à partir d'un nœud cible |
| Principal/sauvegarde (`primary_backup`) | Nœuds partenaires qui basculent les uns sur les autres |
| Entrée de chaîne (`chain_entry`) | Nœud d'entrée d'un proxy chaîné |
| Sortie de chaîne (`chain_exit`) | Nœud de sortie d'un proxy chaîné |

### 5.2 Ajout d'un nœud

1. Ouvrez « Gestion des nœuds » ;
2. Cliquez sur « Ajouter un serveur » ;
3. Remplissez les bases :
   - Nom, par ex. `HongKong-HK01`
   - Nom d'hôte : IP ou domaine
   - Région, par ex. `HK`, `JP`, `US`
   - Type de nœud ;
4. Définissez le port de l'agent (`18443` par défaut) ;
5. Sélectionnez les cœurs pris en charge ;
6. Cliquez sur « Enregistrer ».

### 5.3 Déploiement en un clic

1. Après avoir ajouté le nœud, cliquez sur « Commande Bootstrap » ;
2. Copiez la commande d'installation ;
3. Exécutez-le en tant qu'utilisateur root sur le nœud distant :

   ```bash
   curl -fsSL https://your-panel/install.sh | bash
   ```

4. L'agent télécharge, s'enregistre et démarre automatiquement.

### 5.4 Détails du nœud

Cliquez sur un nœud pour ouvrir sa page de détails :

- **Aperçu** : informations de base, statut de l'agent, statut du certificat
- **Centre de protocole** : configuration du processus principal
- **Proxy** : configuration Caddy / Nginx
- **Processus principaux** : démarrer/arrêter/redémarrer
- **WARP** : configuration WARP
- **Domaines directs** : gestion de domaine
- **Limite de débit** : gestion du trafic
- **Certificats** : liaisons de certificat
- **Utilisateurs en ligne** : utilisateurs en ligne en temps réel

### 5.5 Actions des nœuds

- **Sync config** : poussez la configuration du panneau vers le nœud
- **Forcer la reconnexion** : abandonnez la connexion de l'agent et reconnectez-vous
- **Mettre à jour l'agent** : envoyer une mise à jour de l'agent
- **Révoquer le certificat** : révoquer le certificat d'agent du nœud

---

## 6. Centre de protocole

Le centre de protocole configure les paramètres de protocole pour les processus principaux.

### 6.1 Cœurs pris en charge

| Noyau | Descriptif |
|------|-------------|
| Xray-noyau | VLESS / VMess / Trojan / Shadowsocks et plus |
| sing-box | Plateforme proxy universelle modulaire avec prise en charge TUN ; supporte les protocoles Hysteria2 / TUIC et d'autres protocoles QUIC |

### 6.2 Modules de configuration

- **Journal** : journal d'accès, journal d'erreurs, mode de sortie
- **API** : adresse d'écoute gRPC API, liste de services
- **DNS** : serveurs DNS, stratégie de requête, hôtes statiques
- **Routage** : règles de routage, équilibrage de charge, routage VLESS
- **Entrants** : liste entrante, paramètres de protocole, transport, TLS
- **Sortants** : liste sortante, paramètres de protocole, transport
- **Transport** : WebSocket, gRPC, XHTTP, TUN, etc.
- **TLS** : certificats, ALPN, empreinte digitale uTLS, ECH
- **Reality** : gestion des cibles, bi-clés, identifiants courts
- **Bilan de santé** : sonde URL, intervalle, simultanéité
- **FakeDNS** : pool IP, taille du pool
- **Métriques** : écouteur de métriques Prometheus
- **Politique** : délais d'attente de connexion, tampons, statistiques
- **Données géographiques** : mise à jour automatique, fichiers de ressources
- **Limites de version** : version minimale/maximale

### 6.3 Configuration entrante

1. Sélectionnez le noyau et le protocole ;
2. Remplissez les bases :
   - Étiquette
   - Adresse et port d'écoute
   - Type de protocole
3. Définir les paramètres du protocole (UUID, mot de passe, etc.) ;
4. Choisissez le transport (TCP, WebSocket, gRPC, etc.) ;
5. Configurer TLS (certificat, ALPN) ;
6. Enregistrez et appliquez au noyau.

### 6.4 Configuration sortante

1. Ajoutez un sortant ;
2. Définir la balise et le protocole ;
3. Définissez l'adresse et le port du serveur ;
4. Configurez le transport et TLS ;
5. Enregistrez et postulez.

### 6.5 Partager des liens

1. Cliquez sur « Partager le lien » dans la liste entrante ;
2. Choisissez un format :
   - Partager URI
   - Mihomo YAML
   - sing-box JSON
   - Xray JSON
3. Copiez le lien ou générez un code QR.

---

## 7. Gestion des procurations

La gestion du proxy configure le proxy inverse Caddy ou Nginx.

### 7.1 Types de proxy

- **Caddy** : automatique HTTPS, HTTP/2, HTTP/3
- **Nginx** : configuration flexible et hautes performances

### 7.2 Configuration du site

1. Ouvrez « Gestion du proxy » ;
2. Choisissez le type de proxy (Caddy / Nginx) ;
3. Ajouter un site :
   - Domaine
   - En amont
   - Port local
4. Enregistrez.

### 7.3 Champs

- **en amont** : adresse du backend, par ex. `127.0.0.1:8082`
- **local_port** : port d'écoute local, par ex. `8443`
- **portal_forward** : règle de transfert de portail
- **subscription_forward** : règle de transfert d'abonnement

---

## 8. Ressources de routage

Les ressources de routage gèrent les règles de routage des abonnements.

### 8.1 Caractéristiques

- Gérer les règles de routage
- Configurer les politiques de routage
- Prend en charge plusieurs formats de routage

### 8.2 Utilisation

1. Ouvrez « Ressources de routage » ;
2. Ajoutez une règle de routage ;
3. Définissez le nom, le type et les conditions de la règle ;
4. Enregistrez et postulez.

---

## 9. Centre d'abonnement

Le centre d'abonnement gère les liens d'abonnement et les points de terminaison.

### 9.1 Formats d'abonnement

1. Ouvrez « Centre d'abonnement » → « Formats d'abonnement » ;
2. Ajoutez un format :
   - Nom du format
   - Format de sortie (Base64, YAML, JSON, etc.)
   - Protocoles inclus
3. Enregistrez.

### 9.2 Points de terminaison d'abonnement

1. Ouvrez « Centre d'abonnement » → « Points de terminaison d'abonnement » ;
2. Ajoutez un point de terminaison :
   - Nom du point de terminaison
   - Adresse d'écoute
   - Chemin
   - Format associé
3. Enregistrez.

### 9.3 Liens d'abonnement

- Récupérer un abonnement sur `http://your-domain/sub/:token`
- Utilisez des paramètres tels que `?format=yaml` pour sélectionner un format

---

## 10. Gestion des clients

La gestion client gère les utilisateurs finaux.

### 10.1 Ajouter un client

1. Ouvrez « Gestion des clients » ;
2. Cliquez sur « Ajouter un client » ;
3. Remplissez :
   - Courriel
   - Mot de passe
   - Planifier
   - Quota de trafic
   - Heure d'expiration
4. Enregistrez.

### 10.2 Actions du client

- **Modifier** : modifier les détails du client
- **Suspendre** : suspendre le service
- **Reprendre** : restaurer un client suspendu
- **Réinitialiser le trafic** : réinitialiser le trafic utilisé
- **Réinitialiser le mot de passe** : réinitialiser le mot de passe du client
- **Renouvellement** : prolonger la date d'expiration
- **Upgrade / Downgrade** : changez de forfait
- **Supprimer** : supprimer le client

### 10.3 Détails de l'abonnement

Cliquez sur un client pour voir :

- Lien d'abonnement
- Utilisation du trafic
- Historique des commandes
- Historique des transactions

---

## 11. Gestion des régimes

La gestion des forfaits configure les forfaits que vous vendez.

### 11.1 Modèles de forfaits

1. Ouvrez « Gestion du plan » → « Modèles de plan » ;
2. Ajoutez un modèle :
   - Nom du modèle
   - Catégorie (standard / premium / entreprise / essai)
   - Renouvellement automatique
   - Paramètres d'essai
3. Enregistrez.

### 11.2 Variantes de forfaits

1. Ajoutez une variante sous le modèle :
   - Nom de la variante (par exemple mensuel, annuel)
   - Prix (en centimes)
   - Durée (jours)
   - Trafic (GB, `0` = illimité)
   - Limite de concurrence
   - Limite de débit (Mbps)
2. Sauvegarder.

### 11.3 Modules supplémentaires de trafic

1. Ouvrez « Modules complémentaires de trafic » ;
2. Ajoutez un module complémentaire :
   - Nom
   - Prix
   - Trafic
   - Mode d'expiration (prolonger les jours / identique au plan)
3. Enregistrez.

### 11.4 Bons de réduction

1. Ouvrez « Coupons » ;
2. Ajouter un coupon :
   - Coder
   - Type (pourcentage / montant fixe)
   - Valeur de remise
   - Limite d'utilisation
   - Période de validité
   - Forfaits applicables
3. Enregistrez.

---

## 12. Gestion des recharges

La gestion des recharges gère les sites de recharge, les commandes et les cartes de recharge.

### 12.1 Sites de recharge

1. Ouvrez « Gestion des recharges » → « Sites de recharge » ;
2. Ajouter un site :
   - Nom du site
   - Méthodes de paiement prises en charge (Alipay, WeChat, USDT, Stripe, etc.)
   - Devise
   - Taux de change
3. Enregistrer ;
4. Vérifiez le site (fichier HTTP ou vérification DNS).

### 12.2 Ordres de recharge

- Voir toutes les commandes de recharge
- Filtrer par statut (en attente, payé, échoué, expiré, remboursé)
- Afficher les détails de la commande
- Émettre des remboursements administratifs

### 12.3 Cartes de recharge

1. Ouvrez « Cartes de recharge » ;
2. Ajouter une carte :
   - Type (montant, forfait, module complémentaire de trafic)
   - Valeur nominale / plan
   - Heure d'expiration
3. Ou générez en masse :
   - Quantité (jusqu'à 1000)
   - Préfixe du numéro de carte
   - Format du numéro de carte
4. Exporter sous CSV.

### 12.4 Gestion du solde

- Afficher les soldes des clients
- Ajuster les soldes (augmenter / diminuer / régler)
- Ajuster les quantités gelées
- Afficher l'historique des transactions

### 12.5 Revendeurs

- Ajouter des revendeurs
- Fixer les taux de commission
- Voir la commission accumulée
- Retirer la commission

---

## 13. Gestion des billets

La gestion des tickets gère les demandes d’assistance client.

### 13.1 Liste des billets

- Voir tous les billets
- Filter by status (open, replied, closed)
- Filtrer par catégorie (technique, facturation, compte, autre)
- Filtrer par priorité (faible, normale, élevée, urgente)

### 13.2 Actions sur les tickets

- **View / reply**: open a ticket and reply
- **Assign**: assign a ticket to an administrator
- **Close**: close a resolved ticket

---

## 14. Gestion des certificats

La gestion des certificats gère les certificats TLS.

### 14.1 Issuing a certificate

1. Open “Certificate management”;
2. Cliquez sur « Émettre un certificat » ;
3. Remplissez :
   - Liste de domaines (une par ligne)
   - Type de certificat (domaine / IP)
   - Type de clé (ECC/RSA)
   - Validation method (DNS-01 / HTTP-01)
   - Fournisseur DNS
   - Issuance location (panel host / node)
4. Cliquez sur « Problème ».

### 14.2 Actions de certificat

- **Renew**: renew manually
- **Deploy**: deploy to a node
- **Distribute**: push to all associated nodes
- **Migrer** : déplacer les références d'un ancien certificat vers un nouveau
- **Télécharger** : télécharger `cert` / `fullchain` / `privkey`

### 14.3 Télécharger un certificat

1. Click “Upload certificate”;
2. Choisissez la méthode :
   - Collez le contenu
   - Télécharger des fichiers
3. Téléchargez la chaîne de certificats et la clé privée ;
4. Sauvegarder.

### 14.4 Configuration du EAB

- Gérer les informations d'identification EAB pour Google Public CA / ZeroSSL
- Obtenez automatiquement le EAB
- Attachez-le automatiquement lors de l'émission

---

## 15. Quotas de trafic

Les quotas de trafic surveillent le trafic mensuel de chaque nœud.

### 15.1 Liste des quotas

- Afficher les paramètres de quota pour tous les nœuds
- Afficher la taille du quota, le trafic utilisé et l'allocation restante
- Afficher le pourcentage d'utilisation

### 15.2 Utilisation du cycle

- Afficher les détails d'utilisation du cycle par nœud
- Afficher le trafic entrant/sortant
- Afficher l'heure de début/fin du cycle

### 15.3 Réinitialiser le journal

- Afficher les enregistrements de réinitialisation du trafic
- Afficher le cycle de réinitialisation, la source de déclenchement et l'opérateur

---

## 16. Proxy chaînés

Les proxys chaînés configurent des liens à sorties multiples.

### 16.1 Créer un lien

1. Ouvrez « Proxies chaînés » ;
2. Cliquez sur « Nouveau lien » ;
3. Sélectionnez le nœud d'entrée ;
4. Sélectionnez l'entrée entrante ;
5. Sélectionnez les nœuds de sortie (plusieurs autorisés);
6. Sélectionnez un entrant pour chaque sortie ;
7. Définir la priorité/le poids ;
8. Confirmer.

### 16.2 Gestion des liens

- Afficher la présentation de la topologie
- Modifier un lien (priorité, poids, balise sortante)
- Supprimer un lien
- Vérifier l'état de santé

### 16.3 Nœuds externes

- Importer des nœuds externes (v2ray URI, Base64 URI, abonnement URL)
- Enregistrer les entrées externes
- Gérer les nœuds externes

---

## 17. Utilisateurs en ligne

Les utilisateurs en ligne affichent leur statut en ligne en temps réel.

### 17.1 Ce qu'il montre

- Abonnements en ligne
- Appareils/IP en ligne
- Nœuds en ligne
- Tarif en temps réel
- Trafic total

### 17.2 Filtrage

- Filtrer par nœud
- Recherche par email / client / nœud / IP

---

## 18. Gestion des utilisateurs

La gestion des utilisateurs gère les utilisateurs du panneau.

### 18.1 Rôles

| Rôle | Descriptif |
|------|-------------|
| Super administrateur (`superAdmin`) | Privilèges les plus élevés |
| Administrateur (`admin`) | Privilèges de gestion |
| Opérateur (`operator`) | Privilèges opérationnels |
| Visionneuse (`viewer`) | Lecture seule |

### 18.2 Ajouter un utilisateur

1. Ouvrez « Gestion des utilisateurs » ;
2. Cliquez sur « Ajouter un utilisateur » ;
3. Remplissez :
   - Nom d'utilisateur
   - Courriel
   - Mot de passe
   - Rôle
4. Enregistrez.

### 18.3 Actions de l'utilisateur

- **Modifier** : modifier les détails de l'utilisateur
- **Supprimer** : supprimer l'utilisateur

---

## 19. Site portail

Le site portail configure le portail client.

### 19.1 Général

- Nom du site
- Description du site
- Inscription ouverte
- Chemin du portail
- Port portail

### 19.2 Apparence

- Titre du site
- Logo URL
- Couleurs primaires, d'arrière-plan, de texte et d'accentuation
- Police

### 19.3 Navigation

- Ajouter des pages de navigation
- Configurer le nom de la page, l'icône et le lien externe
- Gérer les éléments de navigation

### 19.4 Modules

- Ajouter/modifier des modules
- Configurer le titre, la description et l'icône du module
- Prend en charge Markdown, le texte enrichi et le HTML personnalisé

### 19.5 Langues

- Ajouter des langues
- Gérer la copie
- Importation/exportation JSON

### 19.6 Modèles

- Changer de modèle
- Personnaliser HTML/CSS/JS
- Réinitialiser au modèle par défaut

---

## 20. Paramètres système

Les paramètres système configurent les paramètres du panneau.

### 20.1 Panneau

- Adresse et port d'écoute
- Répertoire de données
- Fuseau horaire
- Port d'abonnement
- Limite d'exportation
- Inscription ouverte
- Mode panneau (complet / gestion uniquement)

### 20.2 Sécurité

- Gestion des clés de chiffrement
- Gestion des clés JWT
- Liste noire IP
- Domaines autorisés Passkey

### 20.3 OAuth

- GitHub OAuth
- Google OAuth
- Configuration de rappel URL

### 20.4 Notifications

- Notifications par courrier électronique (SMTP)
- Notifications Telegram
- Modèles de notifications
- Paramètres de surveillance

### 20.5 Sauvegarde et restauration

- Créer une sauvegarde
- Liste de sauvegarde
- Télécharger une sauvegarde
- Restaurer une sauvegarde
- Sauvegardes planifiées
- Stockage distant (FTP / S3 / WebDAV)

### 20.6 Déblocage automatique

- Activer le déblocage automatique
- Temps de recharge
- Statuts applicables

---

## 21. Console d'opérations

La console d'opérations gère les instantanés de configuration et applique les tâches.

### 21.1 Instantanés de configuration

- Créer un instantané
- Répertorier les instantanés
- Revenir à un instantané
- Supprimer un instantané

### 21.2 Appliquer des tâches

- Afficher les emplois récents
- Appliquer la configuration maintenant (DB → runtime)
- Pré-vérification (valider les conflits de ports et la configuration JSON)
- Annuler un travail
- Réessayer une tâche

### 21.3 Validation des nœuds

- Sélectionnez un nœud et validez
- Afficher le résultat de la validation
- Valider tous les nœuds

### 21.4 Arrêt d'urgence

- Arrêtez d'urgence le proxy (écrit une configuration entrante vide)
- Arrêtez immédiatement tout le trafic entrant

---

## 22. Surveillance de base de données

La surveillance de la base de données montre l’état de la base de données.

### 22.1 Ce qu'il montre

- Moteur de base de données
- Taille de la base de données
- Connectivité
- Latence
- État du pool de connexions
- Statistiques du cache
- Statistiques de requête

### 22.2 Moteurs pris en charge

- SQLite
- MySQL
- PostgreSQL

---

## 23. Journal d'audit

Le journal d'audit enregistre les opérations.

### 23.1 Ce qu'il montre

- Catégorie
- Type d'événement
- Résultat
- Adresse IP source
- Utilisateur
- Détails

### 23.2 Journal des opérations du nœud

- Afficher les enregistrements d'opérations de nœud
- Filtrer par nœud

---

## 24. Gestion des journaux

La gestion des journaux affiche les journaux système.

### 24.1 Types de journaux

- Journaux du panneau
- Journaux de nœud
- Journaux de base
- Journaux proxy

### 24.2 Actions

- Afficher les journaux
- Télécharger les journaux
- Nettoyer les journaux

---

## 25. Extensions d'opérations

Les extensions d'opérations configurent des extensions facultatives.

### 25.1 Caractéristiques

- Extensions personnalisées
- Configuration des extensions
- Gestion des extensions

---

## 26. FAQ

### 26.1 Le panneau ne démarre pas

- Vérifiez si le port est déjà utilisé
- Vérifiez que `.env` est valide
- Vérifiez les journaux : `journalctl -u corefusion -f` ; sans systemd, `tail -f <install-dir>/data/logs/panel.log`

### 26.2 Nœud hors ligne

- Vérifiez que l'agent est en cours d'exécution : `systemctl status corefusion-agent`
- Vérifier la connectivité réseau
- Vérifier les règles de pare-feu (port `18443`)

### 26.3 Échec de la délivrance du certificat

- Vérifiez la résolution du DNS
- Vérifiez que le port 80 est accessible
- Vérifiez la configuration du fournisseur DNS
- Vérifiez les journaux de certificat

### 26.4 L'abonnement ne fonctionne pas

- Vérifiez le lien d'abonnement
- Vérifiez que le nœud est en ligne
- Vérifiez que le processus principal est en cours d'exécution
- Vérifier les règles de pare-feu

### 26.5 Le rappel de recharge échoue

- Vérifiez le rappel URL
- Vérifiez que la clé du site correspond
- Vérifiez la liste d'autorisation IP
- Consultez les journaux de commandes

### 26.6 La page du portail est inaccessible

- Vérifiez que le port du portail est ouvert
- Vérifiez la configuration du chemin du portail
- **Important** : assurez-vous que le proxy inverse transfère le domaine du portail vers le **port du moteur du portail**, et non vers le port du panneau principal : le transfert vers le mauvais port affiche une page vierge ou le panneau au lieu du portail. Vérifiez `upstream` / `local_port` dans l'enregistrement du site proxy.
- Ouvrez `http://<portal-domain>:<portal-port>` directement pour vérifier le moteur de portail lui-même
- Vérifiez la configuration du proxy inverse
- Vérifiez les journaux du portail

---

## Annexe

### A. Ports par défaut

| Port | Objectif |
|------|---------|
| Attribué par l'assistant d'installation (port libre aléatoire par défaut) | Panneau d'administration (modifiable) |
| 18443 | Agent gRPC (modifiable) |
| 0 (désactivé par défaut) | Port du moteur d'abonnement dédié (vide ou `0` partage le port principal) |
| 0 (désactivé par défaut) | Port du moteur de portail dédié (vide ou `0` partage le port principal) |
| 80 / 443 | Proxy inverse (Caddy / Nginx) HTTP / HTTPS |

### B. Chemins par défaut

| Chemin | Objectif |
|------|---------|
| `/` | Panneau d'administration (le portail réside également sur `/` lorsqu'aucun moteur de portail dédié n'est configuré) |
| `/sub/:token` | Récupération d'abonnement (lorsqu'aucun port d'abonnement dédié n'est configuré) |
| `/p` | Page du portail (lorsqu'aucun port de portail dédié n'est configuré) |
| `/:token` | Récupération d'abonnement (avec un port d'abonnement dédié, sans le préfixe `/sub`) |

### C. Outil de ligne de commande

Le binaire du panneau prend en charge ces sous-commandes (aucun argument ne démarre le serveur) :

```bash
# Start the panel (same as ./corefusion server)
./corefusion

# Run database migrations
./corefusion migrate

# Offline administration (when you cannot sign in)
./corefusion admin reset-password -u admin -p 'new-password'
./corefusion admin 2fa reset -u admin
./corefusion admin list

# Version / help
./corefusion version
./corefusion help
```

> Sans unité systemd, redémarrez en arrêtant l'ancien processus et en le redémarrant avec `nohup` ; les journaux sont dans `<install-dir>/data/logs/panel.log`.

---

> **Version du document** : v1.0
> **Dernière mise à jour** : 2026-10-23
> **S'applique à** : CoreFusion v1.0+
