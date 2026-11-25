<!-- source: USER-GUIDE.md | locale: de | updated: 2026-11-23 -->
# CoreFusion Betriebshandbuch

> Sprachen: [简体中文](USER-GUIDE.md) · [English](USER-GUIDE.en.md) · [繁體中文](USER-GUIDE.zh-TW.md) · [Русский](USER-GUIDE.ru.md) · [日本語](USER-GUIDE.ja.md) · [한국어](USER-GUIDE.ko.md) · Deutsch · [Français](USER-GUIDE.fr.md) · [Español](USER-GUIDE.es.md) · [Português](USER-GUIDE.pt.md) · [Türkçe](USER-GUIDE.tr.md) · [Bahasa Indonesia](USER-GUIDE.id.md) · [ไทย](USER-GUIDE.th.md) · [Tiếng Việt](USER-GUIDE.vi.md) · [العربية](USER-GUIDE.ar.md) · [فارسی](USER-GUIDE.fa.md)

> Dieses Handbuch richtet sich an CoreFusion-Benutzer und deckt den gesamten Arbeitsablauf von der Installation bis zum täglichen Betrieb ab.

---

## Inhalt

1. [Systemübersicht](#1-systemübersicht)
2. [Installation](#2-installation)
3. [Erstkonfiguration](#3-erstkonfiguration)
4. [Armaturenbrett](#4-armaturenbrett)
5. [Knotenverwaltung](#5-knotenverwaltung)
6. [Protokollzentrum](#6-protokollzentrum)
7. [Proxy-Verwaltung](#7-proxy-verwaltung)
8. [Routing-Ressourcen](#8-routing-ressourcen)
9. [Abonnementcenter](#9-abonnementcenter)
10. [Kundenmanagement](#10-kundenmanagement)
11. [Planverwaltung](#11-planverwaltung)
12. [Auflademanagement](#12-auflademanagement)
13. [Ticketverwaltung](#13-ticketverwaltung)
14. [Zertifikatsverwaltung](#14-zertifikatsverwaltung)
15. [Verkehrskontingente](#15-verkehrskontingente)
16. [Verkettete Proxys](#16-verkettete-proxys)
17. [Online-Benutzer](#17-online-benutzer)
18. [Benutzerverwaltung](#18-benutzerverwaltung)
19. [Portalseite](#19-portalseite)
20. [Systemeinstellungen](#20-systemeinstellungen)
21. [Betriebskonsole](#21-betriebskonsole)
22. [Datenbanküberwachung](#22-datenbanküberwachung)
23. [Audit-Protokoll](#23-audit-protokoll)
24. [Protokollverwaltung](#24-protokollverwaltung)
25. [Betriebserweiterungen](#25-betriebserweiterungen)
26. [FAQ](#26-faq)
- [Anhang](#anhang)

---

## 1. Systemübersicht

CoreFusion ist ein Multiprotokoll-Proxy-Verwaltungspanel, das VLESS, VMess, Trojan, Shadowsocks, Hysteria2, TUIC und mehr unterstützt, mit Knotenverwaltung, Abonnementbereitstellung, Kundenverwaltung, Abrechnung, Zertifikaten und verketteten Proxys.

### 1.1 Kernfunktionen

| Modul | Beschreibung |
|--------|-------------|
| Multiprotokoll | VLESS, VMess, Trojan, Shadowsocks, Hysteria2, TUIC, WireGuard usw. |
| Mehrkernig | Xray-core und sing-box (Hysteria2/TUIC und andere QUIC-Protokolle laufen auf sing-box) |
| Multi-Knoten | Fünf Knotentypen: Standalone, Spiegel, Aktiv/Standby, verketteter Eingang, verketteter Ausgang |
| Abonnementlieferung | Mehrere Abonnementformate, Endpunkte und Formatverwaltung |
| Abrechnung | Planvorlagen, Aufladekarten, Guthaben, Bestellungen, Wiederverkäuferprovision |
| Zertifikate | Let's Encrypt-Ausstellung, DNS-01/HTTP-01-Validierung, Zertifikatverteilung |
| Verketteter Proxy | Multi-Exit-Lastausgleich, Registrierung externer Knoten, verkettetes Routing |
| Verkehrskontingente | Monatliche Traffic-Überwachung pro Knoten, automatische Suspendierung und Reset-Protokolle |
| Portal | Visueller Portal-Editor, mehrsprachig, benutzerdefinierte Vorlagen |
| Benachrichtigungen | E-Mail und Telegram, Vorlagen und Sendungen |

### 1.2 Architektur

- **Backend**: Los, gRPC
- **Frontend**: Vue 3 + Element Plus
- **Datenbank**: SQLite (Standard) / MySQL / PostgreSQL
- **Proxy**: Caddy / Nginx Reverse-Proxy
- **Panel-Port**: vom Setup-Assistenten zugewiesen (standardmäßig zufälliger freier Port; 8080 ist eine häufige Wahl)
- **Agent-gRPC-Port**: standardmäßig 18443

---

## 2. Installation

### 2.1 Unterstützte Umgebungen

| Artikel | Unterstützung |
|------|---------|
| Verteilungen | Debian / Ubuntu, RHEL / CentOS / Rocky / AlmaLinux / Fedora, Alpine, Arch, openSUSE |
| CPU-Architekturen | Offizielle Binärdateien: `amd64`, `arm64`; Quell-Builds unterstützen auch `armv7`, `armv6`, `386`, `ppc64le`, `s390x`, `riscv64`, `loong64` |
| Serviceleiter | systemd (empfohlen); Fällt in Containern oder anderen Umgebungen ohne systemd auf `nohup` zurück |
| Speicher / Festplatte | ≥ 1 GB / ≥ 10 GB (Datenbank, Zertifikate, Protokolle und Kernbinärdateien leben im Datenverzeichnis) |
| Panel-Anschluss | im Setup-Assistenten ausgewählt (standardmäßig zufälliger freier Port; `8080` und ähnliches sind in Ordnung) |
| Agent gRPC | Standardmäßig `18443` |

Externe Laufzeitbefehle werden automatisch vom Installationsprogramm installiert (`scripts/install.sh` und das vom Panel generierte Agent-Installationsskript; überspringen Sie mit `--no-deps` / `CF_SKIP_DEPS=1`):

| Zweck | Paket |
|---------|---------|
| Firewall / Ratenbegrenzung / Parallelitätsmarkierung | `nftables` (`nft`) |
| Traffic Shaping, Links und Sockets | `iproute2` (`tc` / `ip` / `ss`) |
| Fallback, wenn NFT nicht verfügbar ist | `iptables` (`iptables-nft` auf EL8+, `iptables` auf EL7) |
| Verbindungsverfolgung | `conntrack-tools` (`conntrack`) |
| Laden Sie `nf_conntrack` / `ifb` | `kmod` (`modprobe` / `lsmod`) |
| Prozesserkennung und -bereinigung | `procps` (`pgrep` / `pkill`) |
| Portprüfungen / DH-Parameter | `lsof` / `openssl` |
| Erstellung eines Proxy-Benutzers | `shadow-utils` (Debian `passwd`, Alpine/Arch `shadow`) |
| Downloads und Archive | `curl` / `ca-certificates` / `tar` / `gzip` / `xz` |

> Das Fehlen von `nft` / `tc` betrifft nur die Ratenbegrenzungs- und Firewall-Funktionen; Das Panel startet immer noch.
> Führen Sie zunächst `--cpu-info` aus, um zu sehen, wie die Umgebung erkannt wird und welche Pakete fehlen.

### 2.2 Einzeilige Installation (empfohlen)

```bash
curl -fsSL https://raw.githubusercontent.com/qist/corefusion/main/scripts/install.sh -o install.sh
sudo bash install.sh
```

Oder leiten Sie es direkt weiter (keine Interaktion, keine zusätzlichen Flags):

```bash
curl -fsSL https://raw.githubusercontent.com/qist/corefusion/main/scripts/install.sh | sudo bash
```

Das Skript wird:

1. Ermitteln Sie die Distribution, die CPU-Architektur und die libc und installieren Sie dann Basispakete und Laufzeitabhängigkeiten.
2. Laden Sie `corefusion-linux-<arch>` von GitHub Releases herunter und überprüfen Sie es mit dem passenden `.sha256`;
3. Installieren Sie `/opt/corefusion` und erstellen Sie `data/{sqlite,certs,logs,runtime}`.
4. Schreiben Sie nur einen Platzhalter `.env` mit `SAFE_PATHS=data` – keine Datenbankkonfiguration – damit beim ersten Start der Setup-Assistent aufgerufen wird (der vollständige `.env` wird vom Assistenten generiert);
5. Installieren und starten Sie einen systemd-Dienst (oder führen Sie ihn unter `nohup` aus, wenn systemd nicht verfügbar ist);
6. Warten Sie, bis es fertig ist, und drucken Sie dann das Setup URL, das Token und die Verzeichnispfade aus.

Beispielausgabe:

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

Wenn das Panel nur `127.0.0.1` überwacht (z. B. hinter einem Reverse-Proxy), gibt das Skript einen Tunnelbefehl SSH anstelle eines nicht erreichbaren URL aus.

> Token verloren? Lesen Sie das Startprotokoll erneut: `journalctl -u corefusion -n 50 --no-pager`
> (ohne systemd: `tail -n 50 /opt/corefusion/data/logs/panel.log`).

### 2.2.1 Mehrsprachige Ausgabe

Das Installationsprogramm umfasst vier UI-Sprachen: vereinfachtes Chinesisch, Englisch, Persisch (فارسی) und Russisch (Русский). Es erkennt das System `LANG` automatisch, oder Sie können explizit Folgendes auswählen:

```bash
# English
sudo bash install.sh --lang en

# Persian via environment variable
sudo CF_LANG=fa bash install.sh

# Pass options through a pipe
curl -fsSL https://raw.githubusercontent.com/qist/corefusion/main/scripts/install.sh | sudo bash -s -- --lang ru
```

Unterstützte Werte: `zh` / `en` / `fa` / `ru`; `--lang=ru` wird ebenfalls akzeptiert. Ein nicht unterstützter expliziter Wert schlägt in der Liste der verfügbaren Sprachen fehl.

### 2.3 Versionsfixierung / Spiegelung / Offline-Installation

```bash
# Pin a version
sudo bash install.sh --version v0.13.1

# GitHub download mirror (used as a URL prefix)
sudo bash install.sh --mirror https://ghproxy.net --version latest

# Use a local binary (offline / intranet / pre-built)
sudo bash install.sh --local ./dist/corefusion-linux-arm64

# Explicit target architecture
sudo bash install.sh --arch arm64 --local ./corefusion-linux-arm64
```

> Ohne `--version` verwendet das Skript den neuesten Eintrag in der GitHub-Releases-Liste.
> Bei den meisten Versionen handelt es sich um Vorabversionen, daher wird der Endpunkt `/releases/latest` nicht verwendet.
>
> Offizielle Veröffentlichungen bieten nur `amd64` / `arm64`; Verwenden Sie `--source` oder eine mit `--local` überkompilierte Binärdatei für andere Architekturen.

### 2.4 Aufbau aus der Quelle

```bash
sudo bash install.sh --source --version v0.13.1
```

`--source` klont das Repository (oder verwendet den vorhandenen Checkout, wenn sich das Skript darin befindet) und führt `make panel` aus. Toolchain-Versionen werden pro Umgebung ausgewählt:

| Abhängigkeit | Auswahl |
|------------|-----------|
| Geh | Lesen von `toolchain` / `go` in `go.mod` (derzeit **Go 1.27+**); Wenn die Systemversion zu alt ist, wird die passende Toolchain nach `<install-dir>/.toolchain` heruntergeladen |
| Knoten | **Knoten 22 LTS** standardmäßig (Vite erfordert ≥ 18); Bei Bedarf wird ein passender offizieller Build heruntergeladen |
| Knoten (glibc < 2,28) | Verwendet den Build `unofficial-builds` glibc-217 (z. B. CentOS 7) |
| Knoten (Alpin / Mussel) | Verwendet die Distribution `nodejs` / `npm` |

Überschreibungen:

```bash
# Force Go / Node versions
sudo CF_GO_VERSION=1.27.5 CF_NODE_MAJOR=22 bash install.sh --source

# Go toolchain mirror (default https://go.dev/dl)
sudo CF_GO_MIRROR=https://golang.google.cn/dl bash install.sh --source
```

Manueller Build (Entwicklung / Debugging):

```bash
git clone https://github.com/qist/corefusion.git
cd corefusion

# Frontend + panel for the current architecture; the frontend is embedded via go:embed
make panel

# Cross-compile
make panel-linux-amd64
make panel-linux-arm64
```

> `go build ./...` erzeugt keine Binärdatei. Bauen Sie das Panel mit auf
> `go build -o corefusion ./cmd/corefusion/` oder `make panel`.

### 2.5 Installer-Flags

| Flagge | Beschreibung |
|------|-------------|
| `-d, --dir <PATH>` | Installationsverzeichnis, Standard `/opt/corefusion` |
| `-v, --version <TAG>` | Panel-Version, z.B. `v0.13.1`; Standard `latest` |
| `--binary` | Verwenden Sie die Release-Binärdatei GitHub (Standard). |
| `--source` | Erstellen Sie aus dem Quellcode und bereiten Sie den passenden Go/Node vor |
| `--local <FILE>` | Verwenden Sie eine lokale Binärdatei (Offline-Installation) |
| `--arch <ARCH>` | Zielarchitektur: `amd64` / `arm64` / `armv7` / `armv6` / `386` / `ppc64le` / `s390x` / `riscv64` / `loong64` |
| `--mirror <URL>` | GitHub Download-Präfix, z.B. `https://ghproxy.net` |
| `--repo <URL>` | Quell-Repository, Standard `https://github.com/qist/corefusion.git` |
| `--ref <REF>` | Quellzweig/Tag; Der Standardwert ist `--version`, andernfalls `main` |
| `--no-deps` | Überspringen Sie die Installation der Systemabhängigkeit |
| `--no-start` | Installieren und schreiben Sie die Dienstdatei, ohne zu starten |
| `--cpu-info` | Druckumgebungserkennung und fehlende Abhängigkeiten, dann beenden |
| `--lang <LANG>` | Ausgabesprache: `zh` / `en` / `fa` / `ru` (Standard: System `LANG`) |
| `--uninstall` | Stoppen Sie den Dienst und entfernen Sie das Programm (behält Daten und `.env`) |
| `--purge` | Entfernen Sie bei `--uninstall` auch das Datenverzeichnis und `.env` |

> Offizielle Versionen enthalten nur die Binärdateien `amd64`/`arm64`. Andere Architekturen erfordern `--source`,
> andernfalls gibt der Download 404 zurück.

Verwandte Umgebungsvariablen:

| Variabel | Zweck |
|----------|---------|
| `CF_GO_VERSION` | Überschreiben Sie die für Quell-Builds erforderliche Go-Version |
| `CF_NODE_MAJOR` | Überschreiben Sie die Hauptversion des Knotens (Standard 22). |
| `CF_GO_MIRROR` | Gehen Sie zum Toolchain-Download-Spiegel |
| `CF_SKIP_DEPS` | Legen Sie im Agent-Installationsskript `1` fest, um die Abhängigkeitsinstallation zu überspringen |
| `CF_LANG` | Ausgabesprache des Installationsprogramms überschreiben (wie `--lang`) |

### 2.6 Verzeichnislayout

```text
/opt/corefusion/
├── corefusion                 # panel binary
├── corefusion.bak             # previous version (when upgrading)
├── .env                       # environment file (0600, generated by the setup wizard)
├── .env.secret                # encryption key file (0600, generated on first start)
├── .toolchain/                # Go / Node downloaded for source builds
└── data/                      # data directory (SAFE_PATHS)
    ├── sqlite/app.db          # default SQLite database
    ├── certs/                 # certificates and keys
    ├── logs/                  # runtime logs (without systemd)
    ├── runtime/               # core runtime config and state
    ├── binaries/              # downloaded xray / sing-box cores
    └── proxy/                 # proxy (Caddy/Nginx) sites and config
```

> Das Installationsprogramm erstellt nur `sqlite/`, `certs/`, `logs/` und `runtime/`; `binaries/`, `proxy/`, `rules/` und andere werden vom Panel oder bei der Installation von Kernen erstellt.
> Das Datenverzeichnis stammt von `SAFE_PATHS` in `.env` und kann ein absoluter Pfad sein. Beim Ändern des Installationsverzeichnisses werden vorhandene Daten nicht verschoben.

### 2.7 Servicemanagement

systemd (die meisten Distributionen):

```bash
systemctl status corefusion --no-pager
systemctl restart corefusion
systemctl stop corefusion
journalctl -u corefusion -f          # follow logs
journalctl -u corefusion -n 100      # last 100 lines
```

Ohne systemd (Behälter etc.):

```bash
cd /opt/corefusion && ./corefusion            # foreground (debugging)
cd /opt/corefusion && setsid nohup ./corefusion >> data/logs/panel.log 2>&1 &
tail -f data/logs/panel.log
```

> Ohne systemd startet der Prozess beim Booten nicht; Starten Sie es nach einem Neustart manuell neu (oder führen Sie das Installationsprogramm erneut aus).

### 2.8 Upgrade und Deinstallation

Upgrade (Daten und Konfiguration bleiben erhalten):

```bash
sudo bash install.sh --version <new-version>
```

Das Skript stoppt den Dienst, sichert die alte Binärdatei als `corefusion.bak`, ersetzt sie und startet neu. Die vorhandenen Modelle `.env` und `data/` bleiben unberührt.

Deinstallieren:

```bash
# Remove the program only; keep data and .env
sudo bash install.sh --uninstall

# Also remove the data directory and .env (irreversible)
sudo bash install.sh --uninstall --purge
```

### 2.9 Umgebungsdatei (.env)

Der vollständige `.env` wird beim Speichern der Konfiguration **vom Setup-Assistenten generiert** (Datenbank-DSN, Datenverzeichnis, Geheimnisse, Ports, Domänen).

Bei einer Neuinstallation schreibt das Installationsprogramm nur diesen Platzhalter, damit das Panel den vorhandenen `data/` nicht mit einer initialisierten Installation verwechselt und den Assistenten überspringt:

```env
SAFE_PATHS=data
```

> Wenn bereits eine vollständige Konfiguration (`.env` mit `CF_DB_DSN`) vorhanden ist, überschreibt das Installationsprogramm diese nie und der Assistent wird nicht erneut angezeigt.
> Spätere manuelle Änderungen an `.env` werden nach einem Neustart wirksam.

#### Datenverzeichnis

```env
SAFE_PATHS=data                       # relative to the install directory
# SAFE_PATHS=/srv/corefusion-data     # or an absolute path
```

#### Datenbank

`CF_DB_DSN` wird anhand des Präfixes erkannt:

| Präfix | Datenbank |
|--------|----------|
| unbefestigter oder kahler Weg | SQLite (Standard) |
| `sqlite://` | SQLite (explizit) |
| `mysql://` | MySQL |
| `postgres://` / `postgresql://` | PostgreSQL |

`CF_DB_TYPE` ist optional und überschreibt die Erkennung (`sqlite` / `mysql` / `postgres`).

> **Verschlüsselte Passwörter**: MySQL/PostgreSQL DSNs, die vom Panel geschrieben werden (Setup-Assistent / Datenbankwechsel)
> werden in `.env` als `enc:...` (AES-256-GCM, Schlüssel `CF_SECRET_ENCRYPTION_KEY`) gespeichert.
> Sie werden beim Lesen entschlüsselt; Von Hand geschriebene Klartext-DSNs funktionieren weiterhin.

**SQLite (Standard)**

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

#### Verbindungspool (optional, MySQL/PostgreSQL)

```env
CF_DB_MAX_OPEN_CONNS=100
CF_DB_MAX_IDLE_CONNS=10
```

#### Andere häufige Variablen

| Variabel | Beschreibung |
|----------|-------------|
| `JWT_SECRET` | Signaturschlüssel für Login-Tokens; wird beim ersten Start generiert, wenn es fehlt |
| `CF_SECRET_ENCRYPTION_KEY` | Schlüssel zur Verschlüsselung sensibler Einstellungen (AES-256-GCM); generiert, wenn es fehlt |
| `CF_INITIAL_ADMIN_PASSWORD` | Anfängliches Administratorkennwort (optional; zufällig und standardmäßig einmal gedruckt) |
| `CF_NETWORK_TRAFFIC_ACCOUNTING` | Auf `0` einstellen, um die iptables-basierte Datenverkehrsabrechnung zu deaktivieren (standardmäßig aktiviert). |
| `CF_NETWORK_TRAFFIC_TABLE` | iptables-Tabelle, die für die Verkehrsabrechnung verwendet wird, Standard `corefusion` |

> **Der Verschlüsselungsschlüssel befindet sich in `.env.secret`**: Beim ersten Start wird `CF_SECRET_ENCRYPTION_KEY` in `.env.secret` (0600, dasselbe Verzeichnis wie `.env`) statt in `.env` geschrieben; Ein Legacy-Schlüssel in `.env` wird automatisch migriert. Sichern Sie diese Datei separat – wenn Sie sie verlieren, ist jeder `enc:`-Wert nicht mehr entschlüsselbar.

> Vertrauenswürdige Reverse-Proxys zum Auflösen realer Client-IPs werden nicht mehr über Umgebungsvariablen konfiguriert;
> Sie werden als `panel.trusted_proxies` unter **Einstellungen → Netzwerk** gespeichert.

> Sowohl `.env` als auch `.env.secret` enthalten Geheimnisse; Das Panel verschärft beim Start seine Berechtigungen auf `0600`. Begehen Sie sie niemals.

### 2.10 Docker (optional, erstellen Sie Ihr eigenes Image)

Das Repository liefert **keine** Docker-Datei. Wenn Sie es in einen Container umwandeln, schreiben Sie Ihr eigenes Image. Referenzaufruf:

```bash
docker run -d \
  --name corefusion \
  -p 8080:8080 \
  -p 18443:18443 \
  -v /opt/corefusion/data:/app/data \
  -v /opt/corefusion/.env:/app/.env \
  <your-image>
```

> Container haben normalerweise keine systemd- und keine `nft`-Berechtigungen, sodass Ratenbegrenzungs-/Firewall-Funktionen nicht funktionieren.
> Das Arbeitsverzeichnis muss mit der Basis für das relative `SAFE_PATHS` übereinstimmen.

### 2.11 Überprüfungen nach der Installation

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

## 3. Erstkonfiguration

Wenn `.env` beim ersten Start kein gültiges `CF_DB_DSN` hat, stellt das Panel die Admin-Benutzeroberfläche noch **nicht** bereit. Es beginnt die
**Setup-Assistent** an einem zufälligen Port im Zeitraum 18000–18999 mit einem einmaligen Token, startet dann nach dem Speichern als normales Panel neu.

### 3.1 Öffnen Sie den Setup-Assistenten

1. Kopieren Sie den „Setup-Assistenten“ URL aus der Ausgabe des Installationsprogramms. oder
2. Lesen Sie das Protokoll auf dem Server:

```bash
journalctl -u corefusion -n 50 --no-pager | grep -A2 'setup wizard'
```

```text
CoreFusion first start - setup wizard
  URL   : http://<ip>:<port>/init?init_token=<token>
  Token : <token>
  Valid : 2h0m0s
```

3. Öffnen Sie URL in einem Browser (es enthält bereits `init_token`, sodass keine manuelle Eingabe erforderlich ist).

> Die Assistentensitzung läuft standardmäßig nach **2 Stunden** ab. Wenn es abläuft oder das Panel neu startet, besorgen Sie sich ein neues.
> Geben Sie den Token nicht weiter. Sobald die Einrichtung abgeschlossen ist, wird dieser Port geschlossen und nur der normale Panel-Port bleibt übrig.

### 3.2 Schritte des Assistenten

Der Assistent besteht aus 8 Schritten. Sie können jederzeit zurückkehren; Bis zum endgültigen Speichern wird nichts festgeschrieben.

| Schritt | Inhalt |
|------|---------|
| 1. Datenbank | SQLite / MySQL / PostgreSQL |
| 2. Datenverzeichnis | Geschrieben an `SAFE_PATHS` in `.env` |
| 3. Verschlüsselungsschlüssel | `JWT_SECRET` und `CF_SECRET_ENCRYPTION_KEY` |
| 4. Kerne | Welche Proxy-Kerne installiert werden sollen |
| 5. Proxy | Keine / Nginx / Caddy |
| 6. Websites und Domänen | Ports, Webpfad, Administratorkonto, Währung, Zeitzone, Domänen |
| 7. HTTPS-Zertifikat | Deaktiviert / Hochladen / ACME |
| 8. Zusammenfassung | Überprüfen Sie, laden Sie `.env` herunter, speichern Sie es und starten Sie es neu |

#### Schritt 1: Datenbank

- **SQLite (lokale Datei, empfohlen)**: Lassen Sie den Pfad leer, um `<data-dir>/sqlite/app.db` zu verwenden.
- **MySQL / PostgreSQL**: Host, Port, Benutzer, Passwort, Datenbankname; PostgreSQL unterstützt auch `disable` / `require` / `verify-full`.
- Klicken Sie zur Bestätigung auf „Verbindung testen“. Das Speichern testet es auch. Eine fehlende Datenbank wird automatisch erstellt (erstellen Sie sie manuell, wenn dem Konto `CREATE` fehlt).
- Der DSN (einschließlich des Passworts) wird verschlüsselt, bevor er in `.env` geschrieben wird.

#### Schritt 2: Datenverzeichnis

Speichert die Datenbank, Zertifikate, Protokolle, Kernbinärdateien und mehr; das ist `SAFE_PATHS` in `.env`. Der Standardwert ist `data`
(relativ zum Installationsverzeichnis); ein absoluter Pfad ist erlaubt. **Ändern Sie es später nicht versehentlich** oder das Panel
Die vorhandene Datenbank und die vorhandenen Zertifikate werden nicht gefunden.

#### Schritt 3: Verschlüsselungsschlüssel

- `JWT_SECRET`: Signiert Anmeldetoken.
- `CF_SECRET_ENCRYPTION_KEY`: Verschlüsselt vertrauliche Einstellungen (Datenbankpasswort, DNS-Zugangsdaten, Aufladekarten usw.).
- Beide werden standardmäßig zufällig generiert; Verwenden Sie „Regenerieren“, um neue Werte zu erhalten. **Durch die Änderung von `CF_SECRET_ENCRYPTION_KEY` werden zuvor verschlüsselte Daten unlesbar** – sichern Sie sie.

#### Schritt 4: Kerne

Wählen Sie die zu installierenden Proxy-Kerne aus:

- **Xray**: VLESS / VMess / Trojan / Shadowsocks usw.;
- **sing-box**: trägt die Protokolle Hysteria2 / TUIC und andere QUIC-Protokolle.

Nach dem Speichern und Neustart lädt das Panel sie automatisch herunter und installiert sie. Überprüfen Sie Versionen und Status unter „Protokolle“. Sie können sie auch nachträglich installieren.

#### Schritt 5: Proxy

- **Keine**: Das Panel bedient Kunden direkt (verwalten Sie TLS selbst).
- **Nginx / Caddy**: Das Panel verwaltet den Reverse-Proxy und das Zertifikat-Hosting; wähle eins aus.

Wenn ein Proxy ausgewählt ist, lauscht das Panel normalerweise nur auf `127.0.0.1` und der Proxy bedient die Außenwelt.

#### Schritt 6: Websites und Domänen

| Feld | Beschreibung |
|-------|-------------|
| Ansprache anhören | Automatisch abgeleitet: Proxy → `127.0.0.1`; kein Proxy mit Zertifikat → `0.0.0.0`; sonst → `127.0.0.1` |
| Panel-Anschluss | Standardmäßig zufälliger freier Port; Ändern Sie es, wenn Sie möchten (z. B. 8080) |
| Abonnementport | Optional; `0` teilt den Panel-Port |
| Portalport | Optional; `0` teilt sich den Panel-Port |
| Webpfad | Admin-UI-Pfad, Standard `/`; Der API ist immer `/api/v1` |
| Admin-Benutzername/Passwort | Standardmäßig zufällig; **Das Passwort wird in der Zusammenfassung nur einmal angezeigt** – speichern Sie es |
| Zugriffsmodus | „Domain“ oder „IP“ |
| Standardwährung/Zeitzone | Standardwährung für Aufladekarten und Guthaben; Zeitzone des Panels und des Proxy-Protokolls (z. B. `Asia/Shanghai`) |
| Zum Reverse-Proxy hinzufügen | Wenn ein Proxy und Domänen festgelegt sind, fügen Sie dem Proxy auf einmal Panel-/Abonnement-/Portaldomänen hinzu |

Durch die Eingabe von Domains werden die entsprechenden Einstellungen geschrieben (Callback-Domain, Abonnement-Endpunkt, Share-Domain).

#### Schritt 7: HTTPS-Zertifikat

| Quelle | Beschreibung |
|--------|-------------|
| Deaktivieren Sie HTTPS | Panel/Portal/Abonnements werden über HTTP (unsicher) bereitgestellt; Bei einem Reverse-Proxy können Sie TLS später auf der Proxy-Seite hinzufügen |
| Zertifikat hochladen | Zertifikat-PEM und privaten Schlüssel-PEM einfügen (nur Domänenzugriffsmodus) |
| ACME | Stellt das Zertifikat aus, wenn Sie diesen Schritt absenden; unterstützt Domänen- und IP-Zertifikate |

ACME-Optionen:

- **CA**: Let's Encrypt, Let's Encrypt (Staging), ZeroSSL, Google Öffentliche CA;
- **Ziel**: die konfigurierte Domäne oder die öffentliche IP (IP-Zertifikate werden nur von Let's Encrypt unterstützt);
- **Herausforderung**: `HTTP-01` (erfordert Port 80) oder `DNS-01` (Platzhalter, kein Port 80; wählen Sie einen DNS-Anbieter und geben Sie Anmeldeinformationen ein);
- **Schlüsseltyp/-größe**: ECC 256/384 oder RSA 2048/3072/4096;
- **Google Öffentliche CA**: erfordert EAB; Geben Sie die Schlüssel-ID/HMAC manuell ein oder verwenden Sie ein GCP-Dienstkonto.

> IP-Zertifikate erkennen automatisch die öffentliche IPv4/IPv6 des Hosts. Hinter einem Proxy handelt es sich bei der erkannten Adresse um die des Hosts und nicht um die des Proxy-Ausgangs.
> Domänenzertifikate können auch nachträglich unter „Zertifikate“ ausgestellt werden.

#### Schritt 8: Zusammenfassung

Zeigt die Datenbank, das Datenverzeichnis, die Kerne, den Proxy, die Ports, das Administratorkonto, die Domänen und die `.env`-Schlüssel (geheime Geheimnisse) an und bietet:

- **.env herunterladen**: Behalten Sie eine Kopie der Konfiguration;
- **Zugriffs-URLs**: endgültige URLs für Panel/Abonnement/Portal; HTTP ist als unsicher markiert.

Klicken Sie auf „Speichern und neu starten“, um die Datenbank und `.env` zu schreiben. Das Panel startet wieder im Normalbetrieb.

### 3.3 Nach der Einrichtung

- Der Dienst wird neu gestartet und das Startprotokoll gibt die endgültige Abhöradresse aus.
- Melden Sie sich mit dem Administratorkonto an, greifen Sie über die Zusammenfassung auf URL und den Webpfad zu.
- Wenn das Passwort verloren geht, setzen Sie es mit `./corefusion admin reset-password` offline zurück.

### 3.4 Nicht interaktives CLI-Setup

Für Automatisierungs- oder Headless-Server schreiben Sie die Konfiguration und markieren Sie das Setup direkt als abgeschlossen:

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

Gemeinsame Flaggen:

| Flagge | Beschreibung |
|------|-------------|
| `--db-type` / `--db-host` / `--db-port` / `--db-user` / `--db-password` / `--db-name` | Datenbankeinstellungen |
| `--data-dir` | Datenverzeichnis; Der Standardwert ist `SAFE_PATHS` oder `./data` |
| `--listen-port` / `--subscription-port` / `--portal-port` / `--web-path` | Ports und Webpfad (standardmäßig zufälliger freier Port) |
| `--admin-username` / `--admin-password` | Admin-Konto (zufälliges Passwort wird einmal gedruckt, wenn es weggelassen wird) |
| `--proxy-type` | `none` / `nginx` / `caddy` |
| `--cores` | Zu installierende Kerne, durch Kommas getrennt: `xray,singbox` |
| `--panel-domain` / `--subscription-domain` / `--portal-domain` | Domänen (in die Einstellungen geschrieben und zum Reverse-Proxy hinzugefügt) |
| `--tls-mode` | `none` / `self_signed` / `acme` |
| `--tls-domain` / `--tls-email` / `--tls-challenge` / `--tls-key-type` / `--tls-key-bits` | Zertifikatsoptionen (`--tls-challenge` ist standardmäßig `http-01`; `dns-01` benötigt `--tls-dns-channel` oder `--tls-dns-provider`) |
| `--tls-ip-cert` | Stellen Sie ein Zertifikat für die erkannte öffentliche IP aus (nur Let's Encrypt) |
| `--acme-provider` | `lets_encrypt` / `lets_encrypt_staging` / `zerossl` / `google` |
| `--currency` / `--timezone` | Standardwährung/Zeitzone |

Vollständige Liste: `./corefusion init --help`. Dann starten Sie den Dienst:

```bash
systemctl restart corefusion
```

### 3.5 Erneutes Ausführen des Setup-Assistenten

Um von vorne zu beginnen (entfernt `.env` und die Setup-Markierungen, **behält Daten**):

```bash
cd /opt/corefusion
./corefusion reset-init      # type RESET to confirm
systemctl restart corefusion
```

Verwenden Sie dann den neuen URL und das Token aus dem Startprotokoll.

> Durch die Neuinitialisierung werden das Administratorkonto und der Webpfad zurückgesetzt. Die Geschäftsdaten in der alten Datenbank bleiben erhalten, Sie müssen jedoch möglicherweise das Anmeldekonto neu erstellen.
>
> `reset-init` löscht das gesamte `.env` (einschließlich `SAFE_PATHS`). **Wenn es sich bei Ihrem Datenverzeichnis um einen benutzerdefinierten absoluten Pfad handelt**, stellen Sie `SAFE_PATHS` vor dem Neustart wieder her, andernfalls sieht das Panel in `./data` aus:
>
> ```bash
> printf 'SAFE_PATHS=/srv/corefusion-data\n' > /opt/corefusion/.env
> ```

### 3.6 Sicherheit nach der ersten Anmeldung

#### Ändern Sie das anfängliche Passwort

1. Klicken Sie oben rechts auf den Benutzernamen.
2. Wählen Sie „Passwort ändern“;
3. Geben Sie das alte und das neue Passwort ein.
4. Klicken Sie auf „Speichern“.

#### Anmeldung mit dynamischem Code aktivieren (2FA / TOTP)

> Nach der Aktivierung wechselt das Konto zur **Anmeldung mit dynamischem Code**: Das statische Passwort wird nicht mehr akzeptiert; nur die
> Der 6-stellige Code Ihrer Authentifizierungs-App (oder ein einmaliger Wiederherstellungscode) funktioniert. Dies **ersetzt** die Passwort-Anmeldung – das ist es
> nicht die zweistufige Verifizierung „Passwort + Code“.

1. Klicken Sie oben rechts auf den Benutzernamen.
2. Wählen Sie „2FA aktivieren“;
3. Geben Sie das aktuelle Passwort ein (nur um diese Änderung zu bestätigen);
4. Scannen Sie den QR-Code mit einer Authentifizierungs-App (Google Authenticator / 1Password usw.) oder geben Sie den Schlüssel manuell ein;
5. Geben Sie den 6-stelligen Code ein, um die Aktivierung abzuschließen;
6. **Speichern Sie die Wiederherstellungscodes sofort** (herunterladbar); Jeder Code funktioniert einmal.

Anschließende Anmeldung:

- Nach Eingabe des Benutzernamens wird das Passwortfeld zu „6-stelliger Code / Wiederherstellungscode“ und die Schaltfläche zu „Mit Code anmelden“;
- Geben Sie den aktuellen Code ein; **Das statische Passwort kann nicht mehr verwendet werden**;
- Wiederherstellungscodes sind nur für den einmaligen Gebrauch bestimmt. Generieren Sie sie unter „Dynamic Code Login (2FA)“ neu, wenn sie aufgebraucht sind (dadurch werden alle alten Codes ungültig).

Wenn der Authentifikator verloren geht und die Wiederherstellungscodes erschöpft sind, deaktivieren Sie 2FA offline auf dem Server:

```bash
cd /opt/corefusion
./corefusion admin 2fa reset -u admin    # disables 2FA, clears the secret and recovery codes, revokes all sessions
```

> Ein einzelner Superadministrator, der weder über den Authentifikator noch über Wiederherstellungscodes verfügt, kann keine Wiederherstellung über die Web-Benutzeroberfläche durchführen und muss den oben genannten Befehl verwenden.

#### Registrieren Sie einen Passkey (optional)

1. Klicken Sie oben rechts auf den Benutzernamen.
2. Wählen Sie „Passkey-Verwaltung“;
3. Klicken Sie auf „Neues Passkey registrieren“;
4. Vervollständigen Sie die Geräteaufforderung.

> **Hinweis**: Für Passkeys ist HTTPS oder localhost erforderlich, und die Domäne muss auf der Zulassungsliste stehen.

---

## 4. Armaturenbrett

Das Dashboard zeigt die Übersicht über den Multi-Knoten-Cluster und den Knotenzustand.

### 4.1 Was es zeigt

- **Systeminformationen**: Hostname, Betriebssystemversion, Betriebszeit, CPU, Speicher
- **Verkehr**: Echtzeitbandbreite, Gesamtverkehr, monatlicher Trend
- **Knotenzustand**: Online-/Offline-/degradierte Anzahl, Knoten mit hoher Auslastung
- **Warnungen**: Offline-Knoten, heruntergestufte Knoten, bald ablaufende Zertifikate, Kontingentwarnungen
- **Clusterressourcen**: durchschnittliche CPU, Arbeitsspeicher, Festplatte und Durchsatz

### 4.2 Aktionen

- Klicken Sie auf „Details anzeigen“, um die Detailseite eines Knotens zu öffnen
- Klicken Sie auf eine Knotenkarte, um deren Live-Status anzuzeigen
- Verwenden Sie die Filter, um nach Status, Typ oder Region einzugrenzen

---

## 5. Knotenverwaltung

Die Knotenverwaltung ist der Kern von CoreFusion und unterstützt mehrere Knotentypen.

### 5.1 Knotentypen

| Typ | Beschreibung |
|------|-------------|
| Standalone (`standalone`) | Ein unabhängiger Kernknoten |
| Spiegel (`mirror`) | Synchronisiert die Konfiguration von einem Zielknoten |
| Primär/Backup (`primary_backup`) | Partnerknoten, die untereinander ein Failover durchführen |
| Ketteneintrag (`chain_entry`) | Eintrittsknoten eines verketteten Proxys |
| Kettenausgang (`chain_exit`) | Ausgangsknoten eines verketteten Proxys |

### 5.2 Knoten hinzufügen

1. Öffnen Sie „Knotenverwaltung“;
2. Klicken Sie auf „Server hinzufügen“;
3. Füllen Sie die Grundlagen aus:
   - Name, z.B. `HongKong-HK01`
   - Hostname: IP oder Domäne
   - Region, z.B. `HK`, `JP`, `US`
   - Knotentyp;
4. Legen Sie den Agent-Port fest (Standard `18443`).
5. Wählen Sie die unterstützten Kerne aus;
6. Klicken Sie auf „Speichern“.

### 5.3 Bereitstellung mit einem Klick

1. Nachdem Sie den Knoten hinzugefügt haben, wählen Sie „Befehl bereitstellen“ in der Knotenliste (die Detailseite bietet dies auch an).
 „Bootstrap-Installationsbefehl“);
2. Kopieren Sie den Befehl – ​​er enthält ein einmaliges Registrierungstoken (gültig für 24 Stunden, nicht wiederverwenden);
3. Führen Sie es als Root auf dem Remote-Knoten aus. es sieht aus wie:

   ```bash
   bash <(curl -fsSL "http://<panel-host>:<panel-port>/api/v1/agent/install?id=<node-id>&token=<one-time-token>&port=<panel-port>")
   ```

4. Der Agent lädt das Paket herunter, registriert sich beim Panel und startet.

### 5.4 Knotendetails

Klicken Sie auf einen Knoten, um dessen Detailseite zu öffnen:

- **Übersicht**: Basisinformationen, Agentenstatus, Zertifikatsstatus; Ressourcenkarten zeigen CPU-/Speicher-/Festplattennutzung, NIC-Up/Down-Rate, Kernbenutzer-Traffic-Rate, monatlich genutzten Traffic, TCP-Verbindungen, Systemlast und ausgehenden NIC-kumulierten Traffic (stille Aktualisierung alle 10 Sekunden)
- **Protokollzentrum**: Kernprozesskonfiguration
- **Proxy**: Caddy / Nginx-Konfiguration
- **Kernprozesse**: Starten / Stoppen / Neustarten
- **Kernverkehr**: Vom Kern weitergeleiteter Benutzerverkehr und seine monatliche Gesamtsumme
- **WARP**: WARP-Konfiguration
- **Direkte Domänen**: Domänenverwaltung
- **Ratenbegrenzung**: Traffic Shaping
- **Zertifikate**: Zertifikatbindungen
- **Online-Benutzer**: Echtzeit-Online-Benutzer

### 5.5 Knotenaktionen

- **Konfiguration synchronisieren**: Schieben Sie die Panel-Konfiguration auf den Knoten
- **Wiederverbindung erzwingen**: Trennen Sie die Agent-Verbindung und stellen Sie die Verbindung wieder her
- **Agent aktualisieren**: Pushen Sie ein Agent-Update
- **Zertifikat widerrufen**: Widerrufen Sie das Agent-Zertifikat des Knotens

---

## 6. Protokollzentrum

Das Protokollzentrum konfiguriert Protokollparameter für die Kernprozesse.

### 6.1 Unterstützte Kerne

| Kern | Beschreibung |
|------|-------------|
| Xray-Kern | VLESS / VMess / Trojan / Shadowsocks und mehr |
| sing-box | Modulare universelle Proxy-Plattform mit TUN-Unterstützung; trägt Hysteria2 / TUIC und andere QUIC-Protokolle |

### 6.2 Konfigurationsmodule

Die Module unterscheiden sich zwischen den beiden Kernen; Das Panel zeigt den Satz an, der dem ausgewählten Kern entspricht:

**Xray-Kern**

- **Protokoll**: Zugriffsprotokoll, Fehlerprotokoll, Ausgabemodus
- **API**: gRPC API-Abhöradresse, Dienstliste
- **DNS**: DNS-Server, Abfragestrategie, statische Hosts
- **Routing**: Routing-Regeln, Lastausgleich, VLESS-Routing
- **Eingänge/Ausgänge**: Protokoll, Transport, TLS
- **Transport**: WebSocket, gRPC, XHTTP usw. (TUN ist ein eingehendes Protokoll, kein Transport)
- **TLS**: Zertifikate, ALPN, uTLS-Fingerabdruck, ECH
- **Reality**: Zielverwaltung, Schlüsselpaare, Kurz-IDs
- **Gesundheitsprüfung / FakeDNS / Metriken / Richtlinien / Geodaten / Versionsbeschränkungen**: Sonden, IP-Pool, Prometheus, Zeitüberschreitungen und Puffer, Ressourcendateien, Kernversionsbereich

**sing-box**

- **Protokoll**: Ebene, Zeitstempel, Ausgabe
- **NTP**: Zeitsynchronisation
- **DNS**: DNS Server und Regeln
- **Routing**: `final` Standard-Outbound, **Regelsätze**, **Regeln**, Standard-Domain-Resolver, Netzwerkstrategie und Fallback (siehe 6.6)
- **Eingänge/Ausgänge**: Protokoll, Transport, TLS
- **Endpunkte**: WireGuard (einschließlich WARP-Konfigurationen), MASQUE-Clients und ähnliches; mehrere können nebeneinander existieren und werden durch `tag` unterschieden

> Wenn „Systemnetzwerkintegration verwenden“ aktiviert ist, ist der **Name** des Endpunkts der Name der Kernel-Schnittstelle:
> höchstens 15 Zeichen, nur Buchstaben/Ziffern/`._-`. Ein zu langer oder ungültiger Name greift automatisch zurück
> zum Endpunkt `tag`. Neue Endpunkte werden pro Protokoll vorangestellt (`wg-` / `ts-` / `oc-` / `ovpn-c-` /
> `ovpn-s-` / `mq-c-` / `mq-s-`) und Legacy-Datensätze wie `WireGuard-xxxxxx` werden normalisiert angezeigt
> sobald der Editor geöffnet wird.
- **Dienste**: das Array sing-box `services` der obersten Ebene, z. B. `hysteria-realm`, DERP oder `resolved`
- **Zertifikate/HTTP-Clients/Netzwerk-Namespaces**: JSON-Abschnitte, die Sie nur bei Bedarf erweitern
- **Experimentell**: `clash_api` und ähnlich; **Es wird nichts in die generierte Konfiguration geschrieben, es sei denn, es ist aktiviert**

> sing-box 1.12 entfernte die Routing-Felder `geosite` / `geoip`; Geobasiertes Routing wird jetzt durchgeführt
> `rule_set`. Das Beibehalten der alten Felder führt dazu, dass der Kern die Konfiguration nicht dekodieren kann (`unknown field`).

### 6.3 Inbound-Konfiguration

1. Wählen Sie den Kern und das Protokoll aus.
2. Füllen Sie die Grundlagen aus:
   - Etikett
   - Hören Sie Adresse und Port
   - Protokolltyp
3. Protokollparameter festlegen (UUID, Passwort usw.);
4. Wählen Sie den Transport (TCP, WebSocket, gRPC usw.);
5. Konfigurieren Sie TLS (Zertifikat, ALPN);
6. Speichern und auf den Kern anwenden.

> Neue eingehende Nachrichten werden standardmäßig auf `::` überwacht (Dual-Stack auf Linux, der sowohl IPv4 als auch IPv6 verarbeitet).
> Ändern Sie sie nur dann in `0.0.0.0` oder eine bestimmte Adresse, wenn Sie die Schnittstelle einschränken müssen.

Die Portmodi „Eingehenden Port verwenden“ bieten auch **Firewall-Zulassungsregeln automatisch erstellen**: Öffnen des
Der Eingangseditor prüft zuerst die Knoten-Firewall und dann, wenn firewalld / ufw / nftables / iptables erkannt wird
Sie können die Speicherung des Ports gemäß dem Überwachungsprotokoll (TCP/UDP) zulassen.

- hy2 / Hysteria-Port-Hopping ermöglicht sowohl den Listening-Port als auch jeden dynamischen Portbereich (z. B. `20000-30000`); mehrere Bereiche werden durch Kommas getrennt;
- Zulässigkeit ist idempotent: Eine bestehende Abdeckungsregel (Portbereich/Multiport) wird nicht erneut erstellt;
- Beim Ändern des Ports wird die alte Regel aufgehoben, bevor die neue zugelassen wird. Durch Löschen des eingehenden Ports, Deaktivieren des Port-Hoppings oder Wechseln zu Unix Socket/tun (kein Überwachungsport) wird alles bereinigt.
- Wenn keine aktivierte Firewall erkannt wird, wird nur ein Hinweis angezeigt und keine Regel geschrieben.

**Transport und Fluss**:

- `xtls-rprx-vision` funktioniert nur über rohes TCP: Bei gRPC / XHTTP / WebSocket schlägt der Client komplett fehl (sing-box: `initialize vision: not a valid supported TLS connection`) oder ist mit der Serverflussprüfung nicht einverstanden (`flow mismatch`). Das Panel entfernt die Vision beim Speichern und beim Generieren der Laufzeitkonfiguration von allen nicht-raw/TCP eingehenden Daten;
- sing-box-Transporte erfolgen nur über http / ws / quic / grpc / httpupgrade – **rohes TCP bedeutet überhaupt keinen Transport**. Der erste (leere) Eintrag der Transport-Dropdown-Liste ist raw TCP; Das Schreiben von `type: "tcp"` führt dazu, dass der Kern die Konfiguration ablehnt.

### 6.3.1 Reverse-Proxy-Pfad (Zugriffsdomäne → Vorderseite TLS → Reverse-Proxy-Pfad)

Wenn der eigene Transportpfad des Inbounds `/` ist, ihn aber ein äußerer nginx/caddy unter einem Unterpfad einhängt
(z. B. `location /apps { proxy_pass http://127.0.0.1:8080/; }`), Clients müssen eine Verbindung zu `/apps` oder dem herstellen
Die Anfrage erreicht diesen Speicherort nie – das Abonnement wird weiterhin heruntergeladen, aber der Knoten stellt keine Verbindung her.
Füllen Sie **Reverse-Proxy-Pfad** unter „Zugriffsdomäne → Vorderseite TLS“ aus:

- Geben Sie `/apps` oder `/apps?ed=2560` (mit frühen Daten) ein. Abonnements und Freigabelinks **ersetzen** damit den eingehenden Transportpfad;
- Gilt für ws, httpupgrade, xhttp und h2/http; Transporte ohne Pfad (raw/tcp, kcp) werden ignoriert;
- Es ist unabhängig davon, wo TLS endet: Der Pfad gilt weiterhin, wenn Front TLS ausgeschaltet ist (dem Kern folgen);
- gRPC hat kein URL-Präfixkonzept (der Clientpfad ist immer `/{serviceName}/…`), daher wird der Pfad **in serviceName zusammengeführt**
(`/apps` + `svc` → `apps/svc`); Der Reverse-Proxy muss dieses Präfix in `grpc_pass` entfernen.

### 6.3.2 SNI-basiertes Routing (nginx vs. Caddy)

Die vordere TLS-Schicht teilt Verbindungen auf `:443` anhand des ClientHello-SNI auf: REALITY-Eingänge im
Protokollzentrum leiten Regeln automatisch ab, Regeln können auch manuell unter „Proxy-Verwaltung → SNI“ angelegt werden.
Beide Proxys können nach Domäne aufteilen; der Unterschied liegt im Geltungsbereich des PROXY-Protokolls.

| | nginx (stream) | Caddy (layer4) |
|---|---|---|
| Geltungsbereich von `proxy_protocol` | Vom Panel gepacktes nginx (mit `stream_pp_var`-Patch): **pro SNI-Regel** v1 / v2 / aus. Offizielles oder ungepatchtes nginx: die gesamte Listen-Gruppe (`listen`-Ebene), pro Port alles an oder alles aus | Pro Route (`proxy ... { proxy_protocol v2 }`) |
| xray + sing-box auf demselben `:443` | Mit Patch unterstützt: xray-REALITY erhält v1/v2, die sing-box-Regel wird auf aus gesetzt | Unterstützt: ein SNI zu xray (PP v2), ein anderer zu sing-box (kein PP) |
| Erreichbarkeit von sing-box | Mit Patch gemeinsam mit xray auf `:443` (PP für diese Domäne aus); sonst eigener Port oder separate Listen-Gruppe ohne PP | Teilt `:443` mit xray, getrennt nach SNI |

Hinweise:

- Standard bleibt `v1` (bisheriges `proxy_protocol on`). Jede SNI-Site kann auf `v1`, `v2` oder aus gestellt werden. `v2` ist
  ein Binär-Header (28 Byte bei IPv4) und kürzer als der v1-Textheader (~50–60 Byte); der Unterschied tritt nur einmal pro
  Verbindung auf und ist praktisch nicht messbar.
- Das Panel führt über den Agent `nginx -V` aus und sucht `NGX_STREAM_PP_VAR` (Fähigkeitsflag `stream_pp_var`), Ergebnis
  60 Sekunden gecacht. Bei ungepatchtem nginx, fehlgeschlagener Probe oder altem Agent wird auf gruppenweites
  `proxy_protocol on` (v1) zurückgefallen und abgeleitete sing-box-SNI-Regeln werden übersprungen (Log
  `skipped N sing-box REALITY SNI rule(s)`); eine Konfiguration, die `nginx -t` nicht besteht, wird nie erzeugt.
- sing-box REALITY / anytls können keine PROXY-Header empfangen, abgeleitete Regeln sind daher immer aus. Erzwungenes v1/v2
  für eine solche Domäne bricht den Handshake.
- Hinter einem Reverse-Proxy benötigt xray PROXY protocol für echte Client-IP und Port (`sockopt.acceptProxyProtocol`);
  `realitySettings.xver` ist das, was xray an sein Dest sendet, und hiermit nicht verwandt.
- `singbox_listen_port` in der „Reality-Vorlage“ ist ein **lokaler TLS-Endpunkt** (xray-Dest / sing-box-Handshake), von beiden
  Proxys unterstützt und unabhängig vom öffentlichen SNI-Routing oben.

### 6.4 Outbound-Konfiguration

1. Fügen Sie einen ausgehenden Eintrag hinzu;
2. Legen Sie Tag und Protokoll fest.
3. Legen Sie die Serveradresse und den Port fest;
4. Transport konfigurieren und TLS;
5. Speichern und bewerben.

### 6.5 Links teilen

1. Klicken Sie in der Eingangsliste auf „Link teilen“;
2. Wählen Sie ein Format:
   - Teilen URI
   - Mihomo YAML
   - sing-box JSON
   - Xray JSON
3. Kopieren Sie den Link oder generieren Sie einen QR-Code.

### 6.6 sing-box Routing und Regelsätze

Im „Protokollzentrum“ → sing-box → „Routing“ gibt es zwei Teile: **Regelsätze** als wiederverwendbar deklarieren
Regeldateien und **Regeln** entscheiden über die Übereinstimmungsbedingungen und den ausgehenden Datenverkehr. Regeln werden von ausgewertet
von oben nach unten und das erste Spiel gewinnt.

#### 6.6.1 Regelsätze

| Feld | Beschreibung |
|-------|-------------|
| Tag | Regelsatz-ID, auf die durch Regeln verwiesen wird; muss eindeutig sein |
| Typ | `local` liest eine Datei auf dem Knoten; Mit `remote` kann der Kernel es auf `update_interval` herunterladen |
| Formatieren | `binary` für `.srs` (Standard); `source` für Quellformat `.json` |
| Pfad | `local`-Pfade sind relativ zum Knotendatenverzeichnis; Dateien in `data/rules/` verwenden `rules/xxx.srs` |
| URL | Download-Adresse für `remote`; `download_detour` wählt den für den Download verwendeten Ausgang aus (normalerweise `direct`). |

Lokale Regelsätze (empfohlen; laden Sie die Dateien zuerst unter „Dienstressourcen → Regeldateien“ herunter):

```jsonc
"route": {
  "rule_set": [
    { "type": "local", "tag": "geosite-cn", "format": "binary", "path": "rules/geosite-cn.srs" },
    { "type": "local", "tag": "geoip-cn", "format": "binary", "path": "rules/geoip-cn.srs" }
  ]
}
```

> Das Herunterladen der Datei auf das Panel reicht nicht aus: Ein `local`-Regelsatz muss zunächst auf das Panel übertragen werden
> Knoten („Push to all nodes“), sodass die Datei im `data/rules/` des Knotens vorhanden ist; sonst der Kern
> startet nicht, da die Datei nicht gefunden werden kann.

Remote-Regelsätze (der Knoten muss URL erreichen können):

```jsonc
{ "type": "remote", "tag": "geosite-openai", "format": "binary",
  "url": "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-openai.srs",
  "download_detour": "direct", "update_interval": "1d" }
```

Wenn Sie eine lokale Datei auswählen, wird das Tag vom Dateinamen abgeleitet (`rules/geosite-cn.srs` →
`geosite-cn`); Bei Remote-Sets kommt es vom Ende des URL. Das Format wird ebenfalls ausgefüllt
die Erweiterung (`.srs` → `binary`, `.json` → `source`). Abgeleitete Werte können frei bearbeitet werden
Bereits eingegebene Werte werden nie überschrieben.

> `download_detour` ist seit sing-box 1.14 veraltet und wird in 1.16 entfernt; es immer noch
> Funktioniert in 1.15. Neue Konfigurationen sollten `http_clients` der obersten Ebene mit verwenden
> Stattdessen `route.default_http_client`.

#### 6.6.2 Regeln

| Feld | Streichhölzer |
|-------|---------|
| `domain` | Exakte Domäne |
| `domain_suffix` | Domain-Suffix, einschließlich Subdomains |
| `domain_keyword` | Teilstring-Übereinstimmung |
| `domain_regex` | Regulärer Ausdruck |
| `ip_cidr` / `ip_is_private` | Ziel-IP |
| `source_ip_cidr` | Quell-IP |
| `port` / `port_range` | Zielport |
| `network` | `tcp` / `udp` |
| `rule_set` | Tags der zu verwendenden Regelsätze |
| `outbound` | Zu verwendendes ausgehendes Tag (muss unter „Ausgänge“ oder „Endpunkte“ vorhanden sein) |
| `type: "logical"` | Kombinierte Bedingungen mit `mode` eingestellt auf `and` / `or` |

Mehrere Felder in einer Regel werden mit AND kombiniert (zum Beispiel `network` + `port`), also nein
Es wird ein `logical`-Wrapper benötigt. Verwenden Sie `type: "logical"` nur für ODER/NICHT-Kombinationen. `rule_set`
muss innerhalb der `rules`-Unterregeln platziert werden, nicht neben `rules`:

```jsonc
"rules": [
  { "domain_suffix": ["gstatic.com", "googleapis.com"], "outbound": "direct" },
  { "domain_keyword": ["ip.sb", "yandex.com"], "outbound": "warp" },
  { "rule_set": ["geosite-openai", "geosite-google"], "outbound": "warp" },
  { "rule_set": ["geoip-cn"], "outbound": "warp" },
  { "network": "tcp", "port": 443,
    "rule_set": ["geosite-google"], "outbound": "warp" },
  { "type": "logical", "mode": "or",
    "rules": [
      { "rule_set": ["geosite-google"] },
      { "rule_set": ["geosite-openai"] }
    ],
    "outbound": "warp" }
]
```

#### 6.6.3 Migrieren von xray-Regeln

| xray | sing-box |
|------|----------|
| Nackte Zeichenfolge (einschließlich nackter Werte in `domain`; Teilzeichenfolgenübereinstimmung) | `"domain_keyword": ["ip.sb"]` |
| `"domain": ["a.com"]` | `"domain_keyword": ["a.com"]`; Verwenden Sie `"domain_suffix": ["a.com"]` für eine echte Suffixübereinstimmung oder `"domain": ["a.com"]` für eine genaue Übereinstimmung |
| `"domain:clstorage.net"` | `"domain_suffix": ["clstorage.net"]` |
| `"full:a.com"` | `"domain": ["a.com"]` |
| `"regexp:..."` | `"domain_regex": ["..."]` |
| `"ip": ["geoip:cn"]` | `"rule_set": ["geoip-cn"]` |
| `"domain": ["geosite:cn"]` | `"rule_set": ["geosite-cn"]` |
| `"outboundTag": "direct"` | `"outbound": "direct"` |

Hinweise:

- **Tags werden nicht zwischen Kernen geteilt**: Ein ausgehendes xray-Tag kann nicht in einer sing-box-Regel verwendet werden;
 Verwenden Sie einen Tag, der tatsächlich unter sing-box „Outbounds“ oder „Endpoints“ existiert.
- Ein falsches Ausgangs- oder Endpunkt-Tag meldet `outbound not found`; Es wird ein falsches Regelsatz-Tag gemeldet
 `rule-set not found`.
- Setzen Sie Direkt-/Zulassungslistenregeln an die erste Stelle; Sobald eine Regel zutrifft, gelten spätere Regeln nicht mehr.

---

## 7. Proxy-Verwaltung

Die Proxy-Verwaltung konfiguriert den Reverse-Proxy Caddy oder Nginx.

### 7.1 Proxy-Typen

- **Caddy**: automatisch HTTPS, HTTP/1.1 und HTTP/2 (HTTP/3 / QUIC ist deaktiviert, um einen UDP 443-Konflikt mit Layer4-Routing zu vermeiden)
- **Nginx**: hohe Leistung, flexible Konfiguration

### 7.2 Site-Konfiguration

1. Öffnen Sie „Proxy-Verwaltung“;
2. Wählen Sie den Proxy-Typ (Caddy / Nginx);
3. Fügen Sie eine Website hinzu:
   - Domäne
   - Stromaufwärts
   - Lokaler Hafen
4. Speichern.

### 7.3 Felder

- **Upstream**: Backend-Adresse, z.B. `127.0.0.1:8082`
- **local_port**: lokaler Listen-Port, z.B. `8443`
- **portal_forward**: Portal-Weiterleitungsregel
- **subscription_forward**: Abonnementweiterleitungsregel
- **SNI-Routing**: Wenn ein eingehender Anruf TLS/REALITY verwendet und Clients sich immer über 443 verbinden, leitet das Panel ab
 443 TCP Routing-Regeln vom ClientHello SNI (sowohl xray `streamSettings` als auch sing-box `tls.reality`);
 Der Front-Proxy leitet nur TCP weiter und TLS wird weiterhin im Kern terminiert

---

## 8. Routing-Ressourcen

Routing-Ressourcen sammeln die gemeinsam genutzten Assets, die von Abonnements und Routing verwendet werden. Die Seite verfügt über fünf Registerkarten:

| Tab | Zweck |
|-----|---------|
| Bevorzugte Direct- und CF-Domains | Direkte Ingress-Domänen/IPs und bevorzugte Cloudflare-Domänen, die beim Umschreiben von Zugriffsadressen in Abonnements und externen Knoten verwendet werden |
| CF-IP-Pool | Bevorzugte IP-Quellen, Ports und Sondeneinstellungen, verbraucht von „Zugriffsdomäne“ |
| WAF globale Regeln | Regelquellen und Regeldateien für den Reverse-Proxy WAF, an Knoten gepusht |
| Regeldateien | Herunterladen, geplante Synchronisierung und Push von Regeldateien (`.srs` / `.json` / `.dat`) |
| Externe Knoten | Externe Knoten aus v2ray URI / Base64 / Abonnement URL importieren (siehe 16.3) |

### 8.1 Direkt- und CF-bevorzugte Domains

- **Direkter Zugriff**: Eingangsdomäne oder IP, standardmäßig Port 443; Knoten binden oder für global leer lassen;
- **CF-bevorzugte Domänen**: Cloudflare bevorzugte Domäne plus Port, mit Konnektivitätsprüfung;
- Beide füttern die „Zugriffsdomänen“-Kandidaten, die von Abonnements und dem Umschreiben externer Knoten verwendet werden (siehe 16.3).

- **Eintragsdrift (primär/Standby)**: Ein direkter Eintrag kann einen DNS-Kanal und einen Datensatznamen binden; Nach einem Failover oder Switchback verweist das Panel darauf, dass der A-Datensatz auf die öffentliche IP des aktuellen Primärservers verweist (interne IP, wenn leer). Es werden nur Anbieter mit implementiertem DNS-Writer akzeptiert (`cloudflare` verifiziert; alidns / DNSPod / GoDaddy / Namecheap / Route53 / Google / Huawei Cloud implementiert, aber nicht verifiziert). Die Ausbreitung von Cloudflare wurde bei etwa 2–2,5 Minuten gemessen, wobei TTL auf 60 festgelegt war; Halten Sie die orangefarbene Wolke für TCP/REALITY-Einträge ausgeschaltet.

### 8.2 CF-IP-Pool

- Behalten Sie bevorzugte IP-Quellen (einschließlich Quellen im `bestproxy`-Stil), Abrufintervall und Ports bei;
- „Probe-Einstellungen“ steuern das Probe-Intervall, den Latenzschwellenwert und den Probe-Eintrittsknoten; lokale Sondierung
 Filtert nur eindeutig nicht erreichbare IPs, während die Auswahl eines Eingangsknotens die Leitung dieses Knotens simuliert.
- Der „Dialer“ kann automatisch sein (installierte Kerne), `singbox`, `xray` oder `mihomo`; Kerne, die es nicht sind
 installiert sind, werden nicht aufgeführt. Die Prüfung wird als temporärer Prozess ausgeführt, der nach Abschluss beendet wird und niemals liest oder
 schreibt die eigene Konfiguration des Kerns;
- Wenn „Zugriffsdomäne“ den CF-IP-Pool ohne konkreten Wert verwendet, ist ein Eintrag (mit seinem Port) erforderlich
 zufällig ausgewählt.

### 8.3 WAF globale Regeln

- Konfigurieren Sie die Regelquelle WAF URL und die zu synchronisierenden Regeldateien.
- Nach der Synchronisierung werden sie an die Knoten gepusht und vom Knoten-Proxy geladen (Nginx / Caddy);
- Von diesem URL wird eine Datei mit einem vollständigen URL heruntergeladen. andernfalls wird „Quelle URL + /Dateiname“ verwendet;
- „Match-Cache“ (`match_cache` / `config_match_cache`) ist standardmäßig aktiviert: eine prozessweite Begrenzung
 Der Cache mit Hot-Rule-Match-Ergebnissen reduziert die Regeldurchquerung pro Anfrage. Es ist nur global mit Nein
 Überschreibung pro Domäne; Deaktivieren Sie es vorübergehend für den A/B-Vergleich, wenn Sie Fehlalarme untersuchen
 (Deaktivieren bedeutet eine vollständige Regelauswertung bei jeder Anfrage und eine höhere CPU-Auslastung).

### 8.4 Regeldateien (Routing-Regeldateien)

Diese Registerkarte lädt nur Regeldateien auf das Panel herunter und verschiebt sie an Knoten; nichts wird wirksam
automatisch. Um sie beim Routing zu verwenden, referenzieren Sie sie unter „Protokollzentrum → sing-box → Routing“.
in Regelsätzen und Regeln (siehe 6.6).

- **Regel-Repositorys**: Geben Sie `owner/repo` (GitHub, durchsuchbar) oder einen vollständigen Download URL ein; geplant
 Die Synchronisierung kann alle 6/12 Stunden, täglich um 03:00 Uhr, täglich um Mitternacht oder wöchentlich am Sonntag erfolgen;
- **Datei hinzufügen**: für einzelne Dateien außerhalb einer Repository-Version (z. B. SagerNet `.srs`-Zweig).
 Dateien); Geben Sie den vollständigen URL ein und er wird automatisch heruntergeladen.
- **Dateiliste**: Zeigt Name, Größe, letzte Aktualisierung und den Schalter „Automatische Synchronisierung“ mit „Download“ pro Datei an;
 Die Schaltfläche „Auf alle Knoten übertragen“ oben verschiebt jede Regeldatei auf alle Knoten.
- Dateien werden in den `data/rules/` des Panels heruntergeladen und sind nach dem Push unter den Knotendaten verfügbar
 Verzeichnisse `rules/`; verweisen Sie sie in sing-box als `rules/<file name>`;
- Formate:
  - `.srs`: Binärer Regelsatz sing-box, empfohlen und kleinster;
  - `.json`: Regelsatz für das Quellformat; `rule_set` muss `format: "source"` einstellen;
  - `.dat`: xray `geosite.dat` / `geoip.dat`, die sing-box nicht direkt verwenden kann.

> Der ausklappbare Block „Verwendung“ oben auf der Seite enthält Referenzbeispiele, das xray-Mapping
> Tabelle und häufige Fehler.

### 8.5 Externe Knoten

Das Importieren und Verwalten externer Knoten wird in [16.3 Externe Knoten](#163-external-nodes) behandelt.

---

## 9. Abonnementcenter

Das Abonnement-Center bündelt Knoten in kundenbereite Abonnements: Definieren Sie Abonnement-**Formate** (was ausgegeben werden soll, welche Regeln, welche Pläne können sie sehen), dann Abonnement-**Endpunkte** (öffentliche Domäne/Pfad) und verteilen Sie dann die Links.

### 9.1 Abonnementformate

„Abonnement-Center“ → „Abonnementformate“; eine Zeile pro Client-Ausgabe:

| Feld | Beschreibung |
|---|---|
| Name | Einzigartig; `?config=<name>` stimmt darauf überein und der Kundendialog zeigt es an |
| Kundentyp | clash / shadowrocket / singbox / v2ray – nur Gruppierung anzeigen |
| Ausgabeformat | `mihomo_yaml` (Clash/Mihomo), `singbox_json`, `xray_json`, `base64_uri`, `raw_uri` |
| Inhaltstyp/Erweiterung | Antwortheader und Dateiname |
| Plattform | Win / Mac / android / ios / Universal; passend zu `?platform=`, leer = universell |
| Gebundene Pläne | Leer = für jeden Plan verfügbar; Andernfalls können nur diese Pläne es verwenden |
| Regeln | Routing-Regeln pro Format (strukturierter Editor für Mihomo, roher JSON pro Regel für sing-box / Xray). **Regeln werden nicht zwischen Formaten konvertiert** |

Hinweise:

- Für dasselbe Ausgabeformat können mehrere Konfigurationen vorhanden sein (z. B. `out`, `home`); Wählen Sie eines mit `config=` im Link aus;
- Ohne `config` lautet die Reihenfolge „Konfiguration an das Abonnement gebunden → `platform`-Übereinstimmung → Standard“;
- Eine sing-box-Vorlage kann ihre eigene `route.rule_set` für Clients enthalten, normalerweise als `remote`
 Der Client lädt es herunter. Nur lokale Dateien, die unter „Regeldateien“ auf den `rules/` eines Knotens übertragen werden, sind verfügbar
 zum Kern auf diesem Knoten, nicht zu Client-Abonnements;
- Beim Generieren von sing-box-/Xray-Vorlagen entfernt das Panel fehlende Referenzen (`final`, `dns.final`,
 `default_domain_resolver`, `download_detour`, lokal `rule_set`, ungültige `rule_set`-Definitionen, leer
 `balancer` usw.), damit der Client beim Start nicht mit einem FATAL-Fehler ausfällt;
- Ausgehende Aliase in Regeln werden anhand echter Tags aufgelöst (`REJECT` / `blocked` → `block`, `DIRECT` →).
 `direct`, Groß- und Kleinschreibung wird nicht beachtet); Für nicht auflösbare ausgehende Daten wird auf den Standardausgang und die Regel zurückgegriffen
 behält seine Position;
- Es wird immer eine Gruppe mit `tag = proxy` generiert: ein `selector` für sing-box und ein `random` Balancer für
 Xray und `outbound: proxy` in den Regeln werden in den entsprechenden `balancerTag` umgeschrieben. Der Regeleditor wird geladen
 Outbound-Kandidaten dynamisch aus „Proxy-Gruppen + benutzerdefinierte Outbounds“ und manuell eingegebene Tags
 noch validiert;
- sing-box / Xray-Konfigurationen müssen separat erstellt werden; Wenn sie nicht vorhanden sind, erscheinen sie nicht in Kundenlinks (siehe 10.3).

- Der strukturierte Regeleditor von Mihomo unterstützt logische Regeln `AND` / `OR` / `NOT` / `SUB-RULE`: Schreiben Sie die Bedingung als `((NETWORK,UDP),(DST-PORT,3478/19302))`; Für `SUB-RULE` ist die dritte Spalte der Name der Unterregelgruppe. Bedingungen müssen in Klammern eingeschlossen werden;
- Regeltypen sind auf das neueste Mihomo abgestimmt: `IP-SUFFIX`, `SRC-IP-CIDR/SRC-IP-SUFFIX/SRC-IP-ASN/SRC-GEOIP`, `IN-PORT/IN-TYPE/IN-USER/IN-NAME/REMATCH-NAME`, `PROCESS-NAME-WILDCARD/REGEX`, `PROCESS-PATH-WILDCARD/REGEX`, `UID`, `DSCP` sind alle verfügbar;
- `SUB-RULE` verweist auf den `sub-rules:`-Block der Mihomo-Konfiguration; Das Panel bearbeitet diesen Block noch nicht. Definieren Sie ihn daher zunächst über den YAML-Import oder den freigegebenen config_json.

### 9.2 Abonnementendpunkte

„Abonnement-Center“ → „Abonnement-Endpunkte“; eine Zeile pro öffentlicher Adresse:

| Feld | Beschreibung |
|---|---|
| Domäne | Öffentliche Abonnementdomäne |
| Pfad | Präfix unter der Domäne, Standard `/` |
| Aktiviert | Deaktivierte Endpunkte werden beim Erstellen von Links ignoriert |
| Beschreibung | Freitextnotiz |

- **Mehrere Zeilen (Multi-Domain)** werden unterstützt; wenn mehrere aktiviert sind, folgt die Anbieterdomäne der Domäne, die von der aktuellen Anfrage verwendet wird (siehe 9.4);
- Ein dedizierter Abonnement-Port dient im Stammverzeichnis (`/<token>`, kein `/sub`); bei gemeinsamer Nutzung des Panel-Ports ist es `/sub/<token>`;
- Reverse-Proxys müssen `X-Forwarded-Proto` / `X-Forwarded-Host` weiterleiten, andernfalls wird der erste aktivierte Endpunkt verwendet. Die vom Panel generierten Caddy/Nginx-Site-Vorlagen enthalten beide Header; Bei manuell verwalteten Proxy-Konfigurationen müssen diese manuell hinzugefügt werden.

### 9.3 Abonnement-Links

Grundform: `https://<sub-domain>/<token>` (Panel-Anschluss: `https://<panel-domain>/sub/<token>`)

| Parameter | Beschreibung |
|---|---|
| `format` | Ausgabeformat; Aliase: `clash` → mihomo_yaml, `singbox`/`sing-box` → singbox_json, `xray` → xray_json, `raw` → raw_uri, `base64`/`v2ray` → base64_uri |
| `config` | Wenden Sie ein Abonnementformat nach Namen an. muss mit `format` übereinstimmen, andernfalls wird es ignoriert |
| `platform` | Ordnen Sie eine Konfiguration der Plattform zu (wenn `config` weggelassen wird) |
| `providers` | `1` / `true`: Clash Provider-Modus (`proxy-providers` + `use`); Standard-Inlines-Knoten |

Beispiele:

```bash
# Clash dynamic subscription (config template + providers)
https://sub.example.com/<token>?format=clash&config=out&providers=1

# sing-box / Xray (only meaningful when such a config exists)
https://sub.example.com/<token>?format=singbox_json&config=sing-box-home
https://sub.example.com/<token>?format=xray_json&config=xray-home

# generic clients / plain text
https://sub.example.com/<token>?format=base64_uri
https://sub.example.com/<token>?format=raw_uri
```

### 9.4 Multi-Domain- und Provider-Modus

Bei `providers=1` verweist jeder `proxy-providers`-Eintrag zurück auf den Abonnementendpunkt. Domainauswahl:

1. **Die von dieser Anfrage verwendete Domäne** (`X-Forwarded-Host` erster Wert, Port entfernt, sonst `Host`), wenn sie mit einem **aktivierten** Endpunkt übereinstimmt;
2. Ansonsten der aktivierte Endpunkt mit der **niedrigsten ID**;
3. Der Pfad kommt von diesem Endpunkt: `https://<domain><path>/<token>?format=mihomo_proxies_yaml...`.

Für die Änderung der Domäne ist also keine Änderung des Panels erforderlich: Clients, die über die neue Domäne Daten abrufen, erhalten die neue Domäne innerhalb der Provider, und eine alte Domäne, die aktiviert bleibt, ist nicht mehr fest codiert. Wenn ein Client den alten Anbieter URL zwischengespeichert hat, aktualisieren Sie das Abonnement (oder leeren Sie den Anbieter-Cache / starten Sie den Client neu).

Zusätzliche Hinweise:

- `proxy-providers` innerhalb eines gemeinsam genutzten Formats (`config_json`) wird ignoriert (vermeidet planübergreifende Lecks); Anbieter stammen aus Planknotengruppen und externen Quellen;
- Interne und externe Anbieter-Cache-Pfade werden immer von „internen Anbieter-Standardeinstellungen → Cache-Pfad-Vorlage“ abgeleitet (Standard `./providers/{key}.yaml`); externe Quellen können ihre eigenen nicht festlegen;
- Der alte reservierte Anbieter `subscription-nodes` wird nicht mehr ausgegeben: Gruppen, die „alle Knoten“ verwenden, werden zu konkreten Anbietern oder Inline-Knoten erweitert, wenn der Plan keine enthält.

---

## 10. Kundenmanagement

Das Kundenmanagement kümmert sich um Endbenutzer.

### 10.1 Einen Kunden hinzufügen

1. Öffnen Sie „Kundenverwaltung“;
2. Klicken Sie auf „Kunde hinzufügen“;
3. Füllen Sie aus:
   - E-Mail
   - Passwort
   - Planen
   - Verkehrskontingent
   - Ablaufzeit
4. Speichern.

### 10.2 Kundenaktionen

- **Bearbeiten**: Kundendaten ändern
- **Suspend**: Dienst unterbrechen
- **Fortsetzen**: Einen gesperrten Kunden wiederherstellen
- **Verkehr zurücksetzen**: Verwendeten Verkehr zurücksetzen
- **Passwort zurücksetzen**: Setzen Sie das Passwort des Kunden zurück
- **Erneuern**: Verlängern Sie das Ablaufdatum
- **Upgrade/Downgrade**: Plan ändern
- **Löschen**: Den Kunden entfernen

### 10.3 Abonnementdetails

Kundenverwaltung → Spalte „Abonnement“ → „Details“. Das Dialogfeld listet Links für jedes Format auf, das für den Plan des Kunden verfügbar ist:

- **Clash / Mihomo**: zwei Zeilen – Inline-Knoten und `providers=1` (Proxy-Anbieter), beide tragen `config=<name>`;
- **Sing-box / Xray**: wird nur angezeigt, wenn eine Konfiguration dieses Formats vorhanden und für den Plan sichtbar ist; Links tragen `config=<name>`;
- **Base64 URI / Raw URI**: immer angezeigt (kein `config`);
- Jede Zeile verfügt über eine eigene Schaltfläche zum Kopieren. Durch Klicken auf eine Zeile wird der QR-Code unten in dieses Format geändert.

Wenn also nur eine Clash-Konfiguration vorhanden ist, zeigt der Dialog Clash + Base64 + Raw an; sing-box/Xray-Links werden erst angezeigt, nachdem ihre Konfigurationen erstellt wurden.

Derselbe Dialog zeigt auch die Verkehrsnutzung, den Bestellverlauf und den Transaktionsverlauf an.

---

## 11. Planverwaltung

Die Planverwaltung konfiguriert die von Ihnen verkauften Pläne.

### 11.1 Planvorlagen

1. Öffnen Sie „Planverwaltung“ → „Planvorlagen“;
2. Fügen Sie eine Vorlage hinzu:
   - Vorlagenname
   - Kategorie (Standard / Premium / Enterprise / Testversion)
   - Automatische Verlängerung
   - Testeinstellungen
3. Speichern.

### 11.2 Planvarianten

1. Fügen Sie eine Variante unter der Vorlage hinzu:
   - Variantenname (z. B. monatlich, jährlich)
   - Preis (in Cent)
   - Dauer (Tage)
   - Traffic (GB, `0` = unbegrenzt)
   - Parallelitätslimit
   - Ratenbegrenzung (Mbit/s)
2. Speichern.

### 11.3 Traffic-Add-ons

1. Öffnen Sie „Verkehrs-Add-ons“;
2. Fügen Sie ein Add-on hinzu:
   - Name
   - Preis
   - Verkehr
   - Ablaufmodus (Tage verlängern / wie Plan)
3. Speichern.

### 11.4 Gutscheine

1. Öffnen Sie „Gutscheine“;
2. Gutschein hinzufügen:
   - Code
   - Art (Prozentsatz / Festbetrag)
   - Rabattwert
   - Nutzungsbeschränkung
   - Gültigkeitsdauer
   - Anwendbare Pläne
3. Speichern.

---

## 12. Auflademanagement

Das Auflademanagement verwaltet Aufladestandorte, Bestellungen und Aufladekarten.

> **Zahlungsintegration**
>
> Das Panel lässt sich nicht direkt mit Zahlungsinstituten integrieren (Alipay, WeChat Pay, Stripe, Banken usw.). Es stellt lediglich eine Verbindung zu einer **zwischengeschalteten Zahlungsplattform** her, die sich wiederum mit dem entsprechenden Zahlungsinstitut integriert, Vertragsabwicklung, Acquiring und Abrechnung abwickelt und Zahlungsergebnisse über die [charge-Callback-API](api/charge-callback.md) zurückmeldet.
>
> Die Zwischenzahlungsplattform ist **nicht Teil dieses Projekts; Betreiber müssen es selbst entwickeln (oder ihr eigenes System verwenden)**. Registrieren Sie es nach der Bereitstellung unter „Aufladeseiten“ und führen Sie die Domänenüberprüfung durch, um es als Aufladeeinstiegspunkt zu verwenden. Die Aufladeseite ist hier die Zwischenplattform, nicht das Zahlungsinstitut selbst. Zahlungsbedingungen, Gebühren, Abwicklung und Compliance werden vom Betreiber, dem Zahlungsinstitut und der zwischengeschalteten Plattform vereinbart und getragen.

### 12.1 Ladestationen

1. Öffnen Sie „Aufladeverwaltung“ → „Aufladestandorte“;
2. Fügen Sie eine Website hinzu:
   - Site-Name
   - Von dieser Zwischenplattform unterstützte Zahlungsmethoden (Alipay, WeChat, USDT, Stripe usw., bereitgestellt durch die Integration mit Zahlungsinstituten)
   - Währung
   - Wechselkurs
3. Speichern;
4. Überprüfen Sie die Site (HTTP-Datei oder DNS-Verifizierung).

### 12.2 Aufladeaufträge

- Alle Aufladeaufträge anzeigen
- Nach Status filtern (ausstehend, bezahlt, fehlgeschlagen, abgelaufen, erstattet)
- Bestelldetails anzeigen
- Stellen Sie Admin-Rückerstattungen aus

### 12.3 Karten aufladen

1. Öffnen Sie „Karten aufladen“;
2. Eine Karte hinzufügen:
   - Typ (Betrag, Plan, Traffic-Add-on)
   - Nennwert / Plan
   - Ablaufzeit
3. Oder in großen Mengen generieren:
   - Menge (bis zu 1000)
   - Präfix der Kartennummer
   - Kartennummernformat
4. Exportieren als CSV.

### 12.4 Balance-Management

- Kundensalden anzeigen
- Salden anpassen (erhöhen / verringern / einstellen)
- Passen Sie die eingefrorenen Mengen an
- Transaktionsverlauf anzeigen

### 12.5 Wiederverkäufer

- Wiederverkäufer hinzufügen
- Legen Sie Provisionssätze fest
- Kumulierte Provision anzeigen
- Provision abheben

---

## 13. Ticketverwaltung

Das Ticketmanagement bearbeitet Kundensupportanfragen.

### 13.1 Ticketliste

- Alle Tickets ansehen
- Nach Status filtern (offen, beantwortet, geschlossen)
- Nach Kategorie filtern (technisch, Abrechnung, Konto, Sonstiges)
- Nach Priorität filtern (niedrig, normal, hoch, dringend)

### 13.2 Ticketaktionen

- **Anzeigen/Antworten**: Ticket öffnen und antworten
- **Zuweisen**: Weisen Sie einem Administrator ein Ticket zu
- **Schließen**: Ein gelöstes Ticket schließen

---

## 14. Zertifikatsverwaltung

Die Zertifikatsverwaltung verwaltet TLS-Zertifikate.

### 14.1 Ausstellung eines Zertifikats

1. Öffnen Sie „Zertifikatsverwaltung“;
2. Klicken Sie auf „Zertifikat ausstellen“;
3. Füllen Sie aus:
   - Domainliste (eine pro Zeile)
   - Zertifikatstyp (Domäne / IP)
   - Schlüsseltyp (ECC / RSA)
   - Validierungsmethode (DNS-01 / HTTP-01)
   - DNS-Anbieter
   - Ausstellungsort (Panel-Host/Knoten)
4. Klicken Sie auf „Problem“.

### 14.2 Zertifikatsaktionen

- **Erneuern**: manuell erneuern
- **Bereitstellen**: Bereitstellung auf einem Knoten
- **Verteilen**: Push an alle zugehörigen Knoten
- **Migrieren**: Referenzen von einem alten Zertifikat auf ein neues verschieben
- **Herunterladen**: `cert` / `fullchain` / `privkey` herunterladen

### 14.3 Hochladen eines Zertifikats

1. Klicken Sie auf „Zertifikat hochladen“;
2. Wählen Sie die Methode:
   - Fügen Sie den Inhalt ein
   - Dateien hochladen
3. Laden Sie die Zertifikatskette und den privaten Schlüssel hoch.
4. Speichern.

### 14.4 EAB-Konfiguration

- Verwalten Sie die EAB-Anmeldeinformationen für die öffentliche Zertifizierungsstelle Google / ZeroSSL
- Erhalten Sie EAB automatisch
- Hängen Sie es bei der Ausstellung automatisch an

---

## 15. Verkehrskontingente

Verkehrskontingente überwachen den monatlichen Verkehr jedes Knotens.

### 15.1 Quotenliste

- Kontingenteinstellungen für alle Knoten anzeigen
- Kontingentgröße, verbrauchten Traffic und verbleibendes Kontingent anzeigen
- Nutzungsprozentsatz anzeigen

### 15.2 Fahrradnutzung

- Details zur Zyklusnutzung pro Knoten anzeigen
- Eingehenden/ausgehenden Datenverkehr anzeigen
- Zeigt die Start-/Endzeit des Zyklus an

### 15.3 Protokoll zurücksetzen

- Sehen Sie sich Datensätze zum Zurücksetzen des Datenverkehrs an
- Zeigt den Reset-Zyklus, die Triggerquelle und den Operator an

---

## 16. Verkettete Proxys

Verkettete Proxys konfigurieren Multi-Exit-Links.

### 16.1 Link erstellen

1. Öffnen Sie „Verkettete Proxys“;
2. Klicken Sie auf „Neuer Link“;
3. Wählen Sie den Eingangsknoten aus.
4. Wählen Sie den Eintrag Eingang;
5. Exit-Knoten auswählen (mehrere zulässig);
6. Wählen Sie für jeden Ausgang einen Eingang aus;
7. Priorität/Gewicht festlegen;
8. Bestätigen.

### 16.2 Linkverwaltung

- Sehen Sie sich die Topologieübersicht an
- Einen Link bearbeiten (Priorität, Gewichtung, ausgehendes Tag)
- Einen Link löschen
- Überprüfen Sie den Gesundheitszustand

### 16.3 Externe Knoten

- Externe Knoten importieren (v2ray URI, Base64 URI, Abonnement URL)
- Registrieren Sie externe Einträge
- Externe Knoten verwalten (einzeln bearbeiten/löschen)
- Zugriffsdomäne: CDN-Beschleunigung / bevorzugte IP pro Knoten konfigurieren (direkte Domäne, CF bevorzugt).
 Domäne, CF-IP-Pool, benutzerdefiniert). Nur der in Abonnements bereitgestellte Server/Port wird neu geschrieben;
 SNI/Host bleibt wie importiert. Wenn deaktiviert, wird die importierte Adresse/der importierte Port verwendet
- Wenn „CF-bevorzugte Domäne/CF-IP-Pool“ ohne einen bestimmten Wert ausgewählt wird, wird ein Eintrag ausgewählt
 zufällig aus dem Pool (mit seinem Port); „direct/custom“ erfordert andernfalls einen expliziten Wert
 es greift auf die eigene Adresse des Knotens zurück

---

## 17. Online-Benutzer

Online-Benutzer zeigen den Online-Status in Echtzeit an.

### 17.1 Was es zeigt

- Online-Abonnements
- Online-Geräte/IPs
- Knoten, IP und Verbindungszeit für jedes Abonnement
- Echtzeit-Uplink-/Downlink-Rate
- Gesamt-Uplink / Gesamt-Downlink / Gesamtverkehr

> Der gesamte Uplink, der gesamte Downlink und der gesamte Datenverkehr summieren sich über eine **kontinuierliche Online-Sitzung**:
> Kurzfristige erneute Verbindungen, Neustarts des Kerns und Zurücksetzen des Zählers löschen sie nicht. Erst nach 60
> Sekunden ohne einen neuen Bericht beginnt eine neue Sitzung bei Null. Das ist eine andere Maßnahme
> aus dem Core-eigenen „Verkehr seit diesem Start“-Zähler und spiegelt besser wider, was ein Benutzer tatsächlich hat
> Wird während der aktuellen Sitzung verwendet.

### 17.2 Filtern

- Nach Knoten filtern
- Suche nach E-Mail / Kunde / Knoten / IP

---

## 18. Benutzerverwaltung

Die Benutzerverwaltung verwaltet Panel-Benutzer.

### 18.1 Rollen

| Rolle | Beschreibung |
|------|-------------|
| Superadministrator (`superAdmin`) | Höchste Privilegien |
| Administrator (`admin`) | Verwaltungsrechte |
| Betreiber (`operator`) | Betriebsrechte |
| Betrachter (`viewer`) | Schreibgeschützt |

### 18.2 Benutzer hinzufügen

1. Öffnen Sie „Benutzerverwaltung“;
2. Klicken Sie auf „Benutzer hinzufügen“;
3. Füllen Sie aus:
   - Benutzername
   - E-Mail
   - Passwort
   - Rolle
4. Speichern.

### 18.3 Benutzeraktionen

- **Bearbeiten**: Benutzerdetails ändern
- **Löschen**: Benutzer entfernen

---

## 19. Portalseite

Die Portal-Site konfiguriert das Kundenportal.

### 19.1 Allgemeines

- Site-Name
- Beschreibung der Website
- Offene Anmeldung
- Portalpfad
- Portalport

### 19.2 Aussehen

- Titel der Website
- Logo URL
- Primär-, Hintergrund-, Text- und Akzentfarben
- Schriftart

### 19.3 Navigation

- Navigationsseiten hinzufügen
- Konfigurieren Sie Seitennamen, Symbol und externen Link
- Navigationselemente verwalten

### 19.4 Module

- Module hinzufügen/bearbeiten
- Konfigurieren Sie Modultitel, Beschreibung und Symbol
- Unterstützt Markdown, Rich Text und benutzerdefiniertes HTML

### 19,5 Sprachen

- Sprachen hinzufügen
- Kopie verwalten
- Import/Export JSON

### 19.6 Vorlagen

- Vorlagen wechseln
- Passen Sie HTML/CSS/JS an
- Zurücksetzen auf die Standardvorlage

---

## 20. Systemeinstellungen

Systemeinstellungen konfigurieren Panel-Parameter.

### 20.1 Panel

- Hören Sie Adresse und Port
- Datenverzeichnis
- Zeitzone
- Abonnementport
- Exportlimit
- Offene Anmeldung
- Panel-Modus (vollständig/nur Verwaltung)

### 20.2 Sicherheit

- Verwaltung von Verschlüsselungsschlüsseln
- JWT Schlüsselverwaltung
- IP-Blacklist
- Passkey Domänen auf der Zulassungsliste

### 20.3 OAuth

- GitHub OAuth
- Google OAuth
- Rückruf URL-Konfiguration

### 20.4 Benachrichtigungen

- E-Mail-Benachrichtigungen (SMTP)
- Telegram-Benachrichtigungen
- Benachrichtigungsvorlagen
- Überwachungseinstellungen

### 20.5 Sichern und Wiederherstellen

- Erstellen Sie ein Backup
- Backup-Liste
- Laden Sie ein Backup herunter
- Stellen Sie ein Backup wieder her
- Geplante Backups
- Remote-Speicher (FTP / S3 / WebDAV)

### 20.6 Automatische Entsperrung

- Aktivieren Sie die automatische Entsperrung
- Abklingzeit
- Anwendbare Status

---

## 21. Betriebskonsole

Die Betriebskonsole verwaltet Konfigurations-Snapshots und wendet Jobs an.

### 21.1 Konfigurations-Snapshots

- Erstellen Sie einen Schnappschuss
- Schnappschüsse auflisten
- Führen Sie einen Rollback zu einem Snapshot durch
- Einen Schnappschuss löschen

### 21.2 Jobs bewerben

- Aktuelle Jobs anzeigen
- Konfiguration jetzt anwenden (DB → Laufzeit)
- Vorabprüfung (Portkonflikte und Konfiguration validieren JSON)
- Einen Job abbrechen
- Wiederholen Sie einen Job

### 21.3 Knotenvalidierung

- Wählen Sie einen Knoten aus und validieren Sie ihn
- Sehen Sie sich das Validierungsergebnis an
- Validieren Sie alle Knoten

### 21.4 Not-Aus

- Stoppen Sie den Proxy im Notfall (schreibt eine leere eingehende Konfiguration)
- Stoppen Sie sofort den gesamten eingehenden Datenverkehr

---

## 22. Datenbanküberwachung

Die Datenbanküberwachung zeigt den Zustand der Datenbank an.

### 22.1 Was es zeigt

- Datenbank-Engine
- Datenbankgröße
- Konnektivität
- Latenz
- Status des Verbindungspools
- Cache-Statistiken
- Statistiken abfragen

### 22.2 Unterstützte Engines

- SQLite
- MySQL
- PostgreSQL

---

## 23. Audit-Protokoll

Das Audit-Protokoll zeichnet Vorgänge auf.

### 23.1 Was es zeigt

- Kategorie
- Ereignistyp
- Ergebnis
- Quell-IP
- Benutzer
- Details

### 23.2 Knotenbetriebsprotokoll

- Knotenbetriebsdatensätze anzeigen
- Nach Knoten filtern

---

## 24. Protokollverwaltung

Die Protokollverwaltung zeigt Systemprotokolle an.

### 24.1 Protokolltypen

- Panel-Protokolle
- Knotenprotokolle
- Kernprotokolle
- Proxy-Protokolle

### 24.2 Aktionen

- Protokolle anzeigen
- Protokolle herunterladen
- Protokolle bereinigen

---

## 25. Betriebserweiterungen

„Operations-Erweiterungen“ (`/client-ext`) konfiguriert Empfehlungsmarketing, Provisionen und Auszahlungen;
„Rechtliche Dokumente“ (`/legal-documents`) verwaltet die portalseitigen Vereinbarungen.

### 25.1 Empfehlungen und Provision

- **Rabattkonfiguration**: Rabattsatz (Prozentsatz der ersten Zahlung des Eingeladenen, `0` = deaktiviert), Provisionsmodus (nur Erstkauf / alle Käufe), Mindestauszahlungsbetrag (der Provisionssaldo muss diesen Wert erreichen, bevor eine Auszahlung beantragt werden kann)
- **Einladungscodes**: Wenn diese Option aktiviert ist, ist für die Registrierung ein Einladungscode erforderlich. Legen Sie die Codelänge, die Ablauftage und die maximale Nutzung fest (`0` = unbegrenzt) und generieren/zeigen Sie die Codeliste an
- **Auszahlungsüberprüfung**: Filtern nach ausstehend/genehmigt/abgelehnt; Fügen Sie einen Bearbeitungsvermerk hinzu und notieren Sie die Auszahlungsmethode und die Wallet-Adresse
- **Empfehlungsdatensätze**: Einladender, Eingeladener, Provisionsbetrag und Provisionsstatus

### 25.2 Rechtsdokumente

- Pflegen Sie „Datenschutzrichtlinie“ und „Nutzungsbedingungen“ separat, ein Markdown-Dokument pro Gebietsschema, mit Live-Vorschau;
- Jedes Dokument kann anstelle der integrierten Seite eine externe URL verwenden; im leeren Zustand wird die integrierte Seite URL verwendet (kopierbar auf der Seite);
- Das Portal zeigt die Version für das aktuelle Gebietsschema an und greift bei Fehlen auf das Standardgebietsschema zurück.

---

## 26. FAQ

### 26.1 Das Panel startet nicht

- Überprüfen Sie, ob der Port bereits verwendet wird
- Überprüfen Sie, ob `.env` gültig ist
- Überprüfen Sie die Protokolle: `journalctl -u corefusion -f`; ohne systemd, `tail -f <install-dir>/data/logs/panel.log`

### 26.2 Knoten offline

- Überprüfen Sie, ob der Agent ausgeführt wird: `systemctl status corefusion-agent`
- Überprüfen Sie die Netzwerkkonnektivität
- Firewall-Regeln prüfen (Port `18443`)

### 26.3 Die Ausstellung des Zertifikats schlägt fehl

- Überprüfen Sie die DNS-Auflösung
- Überprüfen Sie, ob Port 80 erreichbar ist
- Überprüfen Sie die Providerkonfiguration DNS
- Überprüfen Sie die Zertifikatprotokolle

### 26.4 Das Abonnement funktioniert nicht

- Überprüfen Sie den Abonnement-Link
- Überprüfen Sie, ob der Knoten online ist
- Überprüfen Sie, ob der Kernprozess ausgeführt wird
- Überprüfen Sie die Firewall-Regeln

### 26.5 Rückruf zum Aufladen schlägt fehl

- Überprüfen Sie den Rückruf URL
- Überprüfen Sie, ob der Site-Schlüssel übereinstimmt
- Überprüfen Sie die IP-Zulassungsliste
- Überprüfen Sie die Bestellprotokolle

### 26.6 Die Portalseite ist nicht zugänglich

- Überprüfen Sie, ob der Portal-Port geöffnet ist
- Überprüfen Sie die Portalpfadkonfiguration
- **Wichtig**: Stellen Sie sicher, dass der Reverse-Proxy die Portaldomäne an den **Portal-Engine-Port** und nicht an den Hauptpanel-Port weiterleitet. Bei der Weiterleitung an den falschen Port wird eine leere Seite oder das Panel anstelle des Portals angezeigt. Überprüfen Sie `upstream` / `local_port` im Proxy-Site-Datensatz.
- Öffnen Sie `http://<portal-domain>:<portal-port>` direkt, um die Portal-Engine selbst zu überprüfen
- Überprüfen Sie die Reverse-Proxy-Konfiguration
- Überprüfen Sie die Portalprotokolle

---

## Anhang

### A. Standardports

| Hafen | Zweck |
|------|---------|
| Vom Setup-Assistenten zugewiesen (standardmäßig zufälliger freier Port) | Admin-Panel (änderbar) |
| 18443 | Agent gRPC (änderbar) |
| 0 (standardmäßig deaktiviert) | Dedizierter Abonnement-Engine-Port (leer oder `0` teilt den Hauptport) |
| 0 (standardmäßig deaktiviert) | Dedizierter Portal-Engine-Port (leer oder `0` teilt sich den Hauptport) |
| 80 / 443 | Reverse-Proxy (Caddy / Nginx) HTTP / HTTPS |

### B. Standardpfade

| Pfad | Zweck |
|------|---------|
| `/` | Admin-Panel (das Portal befindet sich auch unter `/`, wenn keine dedizierte Portal-Engine konfiguriert ist) |
| `/sub/:token` | Abonnementabruf (wenn kein dedizierter Abonnementport konfiguriert ist) |
| `/p` | Portalseite (wenn kein dedizierter Portal-Port konfiguriert ist) |
| `/:token` | Abonnementabruf (mit einem dedizierten Abonnementport, ohne das Präfix `/sub`) |

### C. Befehlszeilentool

Die Panel-Binärdatei unterstützt diese Unterbefehle (keine Argumente starten den Server):

```bash
# Start the panel (same as ./corefusion server)
./corefusion

# Run database migrations
./corefusion migrate

# Non-interactive init / reset init state (see 3.4, 3.5)
./corefusion init --help
./corefusion reset-init

# Offline administration (when you cannot sign in)
./corefusion admin reset-password -u admin -p 'new-password'
./corefusion admin 2fa reset -u admin
./corefusion admin list

# Version / help
./corefusion version
./corefusion help
```

> Verwenden Sie bei systemd `systemctl restart corefusion` und `journalctl -u corefusion -f`; ohne
> systemd (Container und ähnliches), stoppen Sie den alten Prozess und starten Sie ihn erneut mit `nohup`, Protokolle sind
> in `data/logs/panel.log`.

---

## Änderungen in v0.13.0-rc.1

- **Online-Update des Panels**: „Systemeinstellungen → Panel-Update“ — Versionsauswahl
  (`pre` = Vorabversion), Update, Aufgabenhistorie und Rollback auf das vorherige Binary.
- **Online-Update des Agent**: Node-Details → „Agent-Update“ — aus dem lokalen Binary des
  Panels oder aus GitHub Releases; von älteren Versionen aktualisierte Nodes erhalten beim
  ersten Verbinden automatisch `update.public_key` (0600) und prüfen danach ed25519-Signaturen.
- **WAF-Synchronisation**: Die letzte Synchronisation steht in
  `waf.github_source.last_sync`; Neustarts setzen den Timer nicht zurück; `0 3 * * *` läuft um
  03:00 in der Zeitzone `panel.timezone` (Standard Asia/Shanghai); verpasste Fenster werden ca.
  15 Sekunden nach dem Start nachgeholt.
- **WAF-Whitelist**: Für die Panel-Domain wird `whiteurl.rule` automatisch gepflegt;
  URL-/Argument-/POST-Prüfungen werden nur für Admin-APIs übersprungen, daher lässt sich eine
  Zahlungsseite mit `http://127.0.0.1:18081` wieder speichern (kein SSRF-Fehler mehr).
- **Kontingentrichtlinie**: Addieren bei Verlängerung / Löschen bei Ablauf / Zurücksetzen bei
  Verlängerung (Variante kann den Tarif erben); bei „monatlich“ prüft das Panel stündlich und
  nullt den Verbrauch.
- **Zahlungsseiten**: Ein-/Ausschalten ändert keine Schlüssel oder Zahlungsmethoden;
  Aufladebestellungen lassen sich nach Typ filtern und als CSV exportieren.
- **Portal**: „Aufladen“ unterstützt Online-Zahlung über eine aktive Zahlungsseite und
  Gutscheinkarten; ohne aktive Seite wird nur die Karteneinlösung angezeigt.

---

> **Dokumentversion**: v1.2
> **Letzte Aktualisierung**: 20.11.2026
> **Gilt für**: CoreFusion v0.13.0-rc.1
