<!-- source: USER-GUIDE.md | locale: de | updated: 2026-10-23 -->
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
9. [Abo-Center](#9-abo-center)
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

1. Ermitteln Sie die Distribution, CPU-Architektur und libc und installieren Sie dann Basispakete und Laufzeitabhängigkeiten.
2. Laden Sie `corefusion-linux-<arch>` von GitHub Releases herunter und überprüfen Sie es mit dem passenden `.sha256`;
3. Installieren Sie `/opt/corefusion` und erstellen Sie `data/{sqlite,certs,logs,runtime}`.
4. Schreiben Sie nur einen Platzhalter `.env` mit `SAFE_PATHS=data` – keine Datenbankkonfiguration – damit beim ersten Start der Setup-Assistent aufgerufen wird (der vollständige `.env` wird vom Assistenten generiert);
5. Installieren und starten Sie einen systemd-Dienst (oder führen Sie ihn unter `nohup` aus, wenn systemd nicht verfügbar ist).
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

Wenn das Panel nur `127.0.0.1` abhört (z. B. hinter einem Reverse-Proxy), gibt das Skript einen Tunnelbefehl SSH anstelle eines nicht erreichbaren URL aus.

> Token verloren? Lesen Sie das Startprotokoll erneut: `journalctl -u corefusion -n 50 --no-pager`
> (ohne systemd: `tail -n 50 /opt/corefusion/data/logs/panel.log`).

### 2.3 Versionsfixierung / Spiegelung / Offline-Installation

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

> Ohne `--version` verwendet das Skript den neuesten Eintrag in der GitHub-Releases-Liste.
> Bei den meisten Versionen handelt es sich um Vorabversionen, daher wird der Endpunkt `/releases/latest` nicht verwendet.
>
> Offizielle Veröffentlichungen bieten nur `amd64` / `arm64`; Verwenden Sie `--source` oder eine mit `--local` überkompilierte Binärdatei für andere Architekturen.

### 2.4 Aufbau aus der Quelle

```bash
sudo bash install.sh --source --version v0.13.0-alpha.2
```

`--source` klont das Repository (oder verwendet den vorhandenen Checkout, wenn sich das Skript darin befindet) und führt `make panel` aus. Toolchain-Versionen werden pro Umgebung ausgewählt:

| Abhängigkeit | Auswahl |
|------------|-----------|
| Geh | Lesen von `toolchain` / `go` in `go.mod` (derzeit **Go 1.27+**); Wenn die Systemversion zu alt ist, wird die passende Toolchain auf `<install-dir>/.toolchain` heruntergeladen |
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
| `-v, --version <TAG>` | Panel-Version, z.B. `v0.13.0-alpha.2`; Standard `latest` |
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
| `--uninstall` | Stoppen Sie den Dienst und entfernen Sie das Programm (behält Daten und `.env`) |
| `--purge` | Entfernen Sie bei `--uninstall` auch das Datenverzeichnis und `.env` |

Verwandte Umgebungsvariablen:

| Variabel | Zweck |
|----------|---------|
| `CF_GO_VERSION` | Überschreiben Sie die für Quell-Builds erforderliche Go-Version |
| `CF_NODE_MAJOR` | Überschreiben Sie die Hauptversion des Knotens (Standard 22). |
| `CF_GO_MIRROR` | Gehen Sie zum Toolchain-Download-Spiegel |
| `CF_SKIP_DEPS` | Legen Sie im Agent-Installationsskript `1` fest, um die Abhängigkeitsinstallation zu überspringen |

### 2.6 Verzeichnislayout

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

Das Skript stoppt den Dienst, sichert die alte Binärdatei als `corefusion.bak`, ersetzt sie und startet neu. Vorhandene `.env` und `data/` bleiben unberührt.

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

> Vertrauenswürdige Reverse-Proxys zum Auflösen realer Client-IPs werden nicht mehr über Umgebungsvariablen konfiguriert;
> Sie werden als `panel.trusted_proxies` unter **Einstellungen → Netzwerk** gespeichert.

> `.env` enthält Geheimnisse; Das Panel verschärft seine Berechtigungen beim Start auf `0600`. Begehen Sie es niemals.

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

> Container haben normalerweise keine systemd- und keine `nft`-Berechtigungen, daher funktionieren Ratenbegrenzungs-/Firewall-Funktionen nicht.
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

Wenn `.env` beim ersten Start kein gültiges `CF_DB_DSN` hat, bedient das Panel die Admin-Benutzeroberfläche noch nicht. Es beginnt die
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
- Beide werden standardmäßig zufällig generiert. Verwenden Sie „Regenerieren“, um neue Werte zu erhalten. **Durch die Änderung von `CF_SECRET_ENCRYPTION_KEY` werden zuvor verschlüsselte Daten unlesbar** – sichern Sie sie.

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
| Abonnementport | Optional; `0` teilt sich den Panel-Port |
| Portalport | Optional; `0` teilt sich den Panel-Port |
| Webpfad | Admin-UI-Pfad, Standard `/`; Der API ist immer `/api/v1` |
| Admin-Benutzername/Passwort | Standardmäßig zufällig; **Das Passwort wird in der Zusammenfassung nur einmal angezeigt** – speichern Sie es |
| Zugriffsmodus | „Domain“ oder „IP“ |
| Standardwährung/Zeitzone | Standardwährung für Aufladekarten und Guthaben; Zeitzone des Panels und des Proxy-Protokolls (z. B. `Asia/Shanghai`) |
| Zum Reverse-Proxy hinzufügen | Wenn ein Proxy und Domänen festgelegt sind, fügen Sie dem Proxy auf einmal Panel-/Abonnement-/Portaldomänen hinzu |

Durch die Eingabe von Domains werden die entsprechenden Einstellungen (Callback-Domain, Abonnement-Endpunkt, Share-Domain) geschrieben.

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
> `reset-init` löscht den gesamten `.env` (einschließlich `SAFE_PATHS`). **Wenn es sich bei Ihrem Datenverzeichnis um einen benutzerdefinierten absoluten Pfad handelt**, stellen Sie `SAFE_PATHS` vor dem Neustart wieder her, andernfalls sieht das Panel in `./data` aus:
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
| Eigenständig (`standalone`) | Ein unabhängiger Kernknoten |
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

1. Klicken Sie nach dem Hinzufügen des Knotens auf „Bootstrap-Befehl“;
2. Kopieren Sie den Installationsbefehl.
3. Führen Sie es als Root auf dem Remote-Knoten aus:

   ```bash
   curl -fsSL https://your-panel/install.sh | bash
   ```

4. Der Agent lädt herunter, registriert sich und startet automatisch.

### 5.4 Knotendetails

Klicken Sie auf einen Knoten, um dessen Detailseite zu öffnen:

- **Übersicht**: Basisinformationen, Agentenstatus, Zertifikatsstatus
- **Protokollzentrum**: Kernprozesskonfiguration
- **Proxy**: Caddy / Nginx-Konfiguration
- **Kernprozesse**: Starten / Stoppen / Neustarten
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

- **Protokoll**: Zugriffsprotokoll, Fehlerprotokoll, Ausgabemodus
- **API**: gRPC API-Abhöradresse, Dienstliste
- **DNS**: DNS-Server, Abfragestrategie, statische Hosts
- **Routing**: Routing-Regeln, Lastausgleich, VLESS-Routing
- **Eingänge**: Eingangsliste, Protokolleinstellungen, Transport, TLS
- **Ausgänge**: Ausgangsliste, Protokolleinstellungen, Transport
- **Transport**: WebSocket, gRPC, XHTTP, TUN usw.
- **TLS**: Zertifikate, ALPN, uTLS-Fingerabdruck, ECH
- **Reality**: Zielverwaltung, Schlüsselpaare, Kurz-IDs
- **Gesundheitsprüfung**: Probe URL, Intervall, Parallelität
- **FakeDNS**: IP-Pool, Poolgröße
- **Metriken**: Prometheus-Metrik-Listener
- **Richtlinie**: Verbindungszeitüberschreitungen, Puffer, Statistiken
- **Geodaten**: automatische Aktualisierung, Ressourcendateien
- **Versionsbeschränkungen**: Mindest-/Höchstversion

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

---

## 7. Proxy-Verwaltung

Die Proxy-Verwaltung konfiguriert den Caddy- oder Nginx-Reverse-Proxy.

### 7.1 Proxy-Typen

- **Caddy**: Automatik HTTPS, HTTP/2, HTTP/3
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

---

## 8. Routing-Ressourcen

Routing-Ressourcen verwalten die Routing-Regeln für Abonnements.

### 8.1 Funktionen

- Routing-Regeln verwalten
- Konfigurieren Sie Routing-Richtlinien
- Unterstützt mehrere Routing-Formate

### 8.2 Nutzung

1. Öffnen Sie „Routing-Ressourcen“;
2. Fügen Sie eine Routing-Regel hinzu.
3. Legen Sie den Namen, den Typ und die Bedingungen der Regel fest.
4. Speichern und bewerben.

---

## 9. Abo-Center

Das Abonnementcenter verwaltet Abonnementlinks und Endpunkte.

### 9.1 Abonnementformate

1. Öffnen Sie „Abonnement-Center“ → „Abonnementformate“;
2. Fügen Sie ein Format hinzu:
   - Namen formatieren
   - Ausgabeformat (Base64, YAML, JSON usw.)
   - Enthaltene Protokolle
3. Speichern.

### 9.2 Abonnementendpunkte

1. Öffnen Sie „Abonnement-Center“ → „Abonnement-Endpunkte“;
2. Fügen Sie einen Endpunkt hinzu:
   - Endpunktname
   - Ansprache anhören
   - Pfad
   - Zugehöriges Format
3. Speichern.

### 9.3 Abonnement-Links

- Holen Sie sich ein Abonnement unter `http://your-domain/sub/:token`
- Verwenden Sie Parameter wie `?format=yaml`, um ein Format auszuwählen

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

Klicken Sie auf einen Kunden, um Folgendes anzuzeigen:

- Abo-Link
- Verkehrsnutzung
- Bestellhistorie
- Transaktionsverlauf

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

### 12.1 Ladestationen

1. Öffnen Sie „Aufladeverwaltung“ → „Aufladestandorte“;
2. Fügen Sie eine Website hinzu:
   - Site-Name
   - Unterstützte Zahlungsmethoden (Alipay, WeChat, USDT, Stripe usw.)
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
3. Wählen Sie den Eingangsknoten aus;
4. Wählen Sie den Eintrag Eingang;
5. Exit-Knoten auswählen (mehrere zulässig);
6. Wählen Sie für jeden Ausgang einen Eingang aus.
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
- Externe Knoten verwalten

---

## 17. Online-Benutzer

Online-Benutzer zeigen den Online-Status in Echtzeit an.

### 17.1 Was es zeigt

- Online-Abonnements
- Online-Geräte/IPs
- Online-Knoten
- Echtzeitkurs
- Gesamtverkehr

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
- Einzelheiten

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

Betriebserweiterungen konfigurieren optionale Erweiterungen.

### 25.1 Funktionen

- Benutzerdefinierte Erweiterungen
- Erweiterungskonfiguration
- Erweiterungsmanagement

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
| 0 (standardmäßig deaktiviert) | Dedizierter Abonnement-Engine-Port (leer oder `0` teilt sich den Hauptport) |
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

# Offline administration (when you cannot sign in)
./corefusion admin reset-password -u admin -p 'new-password'
./corefusion admin 2fa reset -u admin
./corefusion admin list

# Version / help
./corefusion version
./corefusion help
```

> Ohne systemd-Einheit starten Sie neu, indem Sie den alten Prozess stoppen und ihn mit `nohup` erneut starten. Protokolle befinden sich in `<install-dir>/data/logs/panel.log`.

---

> **Dokumentversion**: v1.0
> **Letzte Aktualisierung**: 23.10.2026
> **Gilt für**: CoreFusion v1.0+
