<!-- source: THIRD_PARTY_LICENSES.md | locale: de | updated: 2026-11-19 -->
# Komponenten und Lizenzen von Drittanbietern

> Sprachen: [简体中文](THIRD_PARTY_LICENSES.md) · [English](THIRD_PARTY_LICENSES.en.md) · [繁體中文](THIRD_PARTY_LICENSES.zh-TW.md) · [Русский](THIRD_PARTY_LICENSES.ru.md) · [日本語](THIRD_PARTY_LICENSES.ja.md) · [한국어](THIRD_PARTY_LICENSES.ko.md) · Deutsch · [Français](THIRD_PARTY_LICENSES.fr.md) · [Español](THIRD_PARTY_LICENSES.es.md) · [Português](THIRD_PARTY_LICENSES.pt.md) · [Türkçe](THIRD_PARTY_LICENSES.tr.md) · [Bahasa Indonesia](THIRD_PARTY_LICENSES.id.md) · [ไทย](THIRD_PARTY_LICENSES.th.md) · [Tiếng Việt](THIRD_PARTY_LICENSES.vi.md) · [العربية](THIRD_PARTY_LICENSES.ar.md) · [فارسی](THIRD_PARTY_LICENSES.fa.md)

CoreFusion verwendet die folgenden Open-Source-Komponenten. In dieser Datei sind die Hauptkomponenten und Lizenzen aufgeführt. **Die vollständige Liste wird durch `go.mod` / `go.sum`, `web/admin/package.json` und die eigene LIZENZdatei jedes Moduls definiert**.

Wenn Sie Binärdateien dieses Projekts verteilen, bewahren Sie diese Datei, das Projekt `LICENSE` und die Urheberrechts-/Lizenzhinweise aller Komponenten Dritter auf.

## 1. In die Binärdatei (Haupt) kompilierte Go-Abhängigkeiten

| Komponente | Lizenz | Zweck |
|---|---|---|
| `github.com/xtls/xray-core` | MPL-2.0 | Xray-Konfigurationsanalyse, gRPC-Befehle, Protokolldefinitionen |
| `github.com/xtls/reality` | MPL-2.0 | Unterstützung des REALITY-Protokolls |
| `github.com/sagernet/sing` | **GPL-3.0-or-later** | Indirekt verbunden über xray-core |
| `github.com/sagernet/sing-shadowsocks` | **GPL-3.0-or-later** | Indirekt verknüpft über xray-core (Shadowsocks-2022) |
| `github.com/go-sql-driver/mysql` | MPL-2.0 | MySQL-Treiber |
| `github.com/hashicorp/golang-lru/v2` | MPL-2.0 | LRU-Cache |
| `github.com/go-acme/lego/v5` | MIT | Ausstellung des ACME-Zertifikats |
| `github.com/gin-gonic/gin` | MIT | HTTP-Framework |
| `gorm.io/gorm` | MIT | ORM |
| `gorm.io/driver/mysql` | MIT | MySQL-Treiber für GORM |
| `gorm.io/driver/postgres` | MIT | PostgreSQL-Treiber für GORM |
| `github.com/jackc/pgx/v5` | MIT | PostgreSQL-Treiber |
| `github.com/qist/sqlite` | MIT | SQLite-Treiber (Standarddatenbank für Panel/Agent) |
| `github.com/aws/aws-sdk-go-v2` | Apache-2.0 | S3-Backups, Route53 DNS-01 |
| `google.golang.org/grpc` | Apache-2.0 | Agent-gRPC-Kommunikation |
| `google.golang.org/protobuf` | BSD-3-Clause | Protobuf-Laufzeit |
| `golang.org/x/crypto` | BSD-3-Clause | Kryptografische Grundelemente |
| `golang.org/x/oauth2` | BSD-3-Clause | OAuth2 / Google EAB |
| `golang.org/x/sys` | BSD-3-Clause | Systemaufrufe (NFT/TC-Ratenbegrenzung, Prozessmanagement) |
| `github.com/golang-jwt/jwt/v5` | MIT | JWT |
| `github.com/google/uuid` | BSD-3-Clause | UUID |
| `github.com/go-webauthn/webauthn` | BSD-3-Clause | Passkey / WebAuthn-Anmeldung |
| `github.com/pquerna/otp` | Apache-2.0 | TOTP Zwei-Faktor-Authentifizierung |
| `github.com/jlaffaye/ftp` | ISC | FTP Backup-Upload |
| `gopkg.in/yaml.v3` | MIT / Apache-2.0 | YAML-Konfigurationsanalyse |

> ⚠ **GPL-3.0-Komponenten**: `sing` und `sing-shadowsocks` sind GPL-3.0 oder höher. Sie sind statisch mit den Panel- und Agent-Binärdateien verknüpft, sodass **für die Verteilung von Binärdateien die Bereitstellung der entsprechenden Quelle erforderlich ist** (siehe Abschnitt 3 von [LEGAL.md](./LEGAL.md)).

## 2. Frontend-Abhängigkeiten (kompiliert in die Panel-Binärdatei)

Die Admin-Benutzeroberfläche wird mit Vite erstellt und über `go:embed` in die Panel-Binärdatei eingebettet, sodass die folgenden Komponenten auch mit der Binärdatei verteilt werden:

| Komponente | Lizenz | Zweck |
|---|---|---|
| Vue 3 / Vue Router / Pinia / Element Plus / axios / vue-i18n / marked / js-yaml / qrcode / Monaco Editor | MIT | Admin-Benutzeroberfläche, Routing und Status, i18n, Markdown-Rendering, QR-Codes und Code-Editor |
| DOMPurify | Apache-2.0 / MPL-2.0 | XSS-Bereinigung vor dem Rendern von Markdown/HTML |

## 3. Eigenständige Programme, die zur Laufzeit vom Panel heruntergeladen/aufgerufen werden

Beim Installieren oder Aktualisieren von Kernen lädt das Panel/der Agent die folgenden Binärdateien von GitHub herunter und führt sie als separate Prozesse aus. **Artefakte, die mit „von diesem Projekt erstellt“ gekennzeichnet sind, werden von dieser Organisation aus Upstream-Quellen zusammengestellt und veröffentlicht** (`qist/*-binaries`) und werden zusammen mit dem Produkt vertrieben, sodass die entsprechenden Lizenzverpflichtungen (einschließlich Quellverfügbarkeit für GPL) bei diesem Projekt liegen; Bei den anderen handelt es sich um Download-URLs offizieller Upstream-Veröffentlichungen.

| Programm | Lizenz | Notizen |
|---|---|---|
| Xray-core | MPL-2.0 | Proxy-Kern, separater Prozess; offizielle Upstream-Version (`XTLS/Xray-core`) |
| sing-box | **GPL-3.0-or-later** | Proxy-Kern, separater Prozess; **von diesem Projekt erstellt** (`qist/singbox-binaries`; in offiziellen Versionen fehlt `with_v2ray_api`, das für die Datenverkehrsabrechnung pro Benutzer erforderlich ist); Geben Sie bei der Weitergabe die entsprechende Quelle an |
| Caddy | Apache-2.0 | Reverse-Proxy / TLS; **von diesem Projekt erstellt** (`qist/caddy-binaries`) |
| Nginx | BSD-2-Clause | Reverse-Proxy; **von diesem Projekt erstellt** (`qist/nginx-binaries`, geteilt durch glibc / musl) |

> Hysteria2 und TUIC sind **von sing-box getragene Protokolle**, keine eigenständigen Kerne oder Binärdateien, daher werden sie nicht separat aufgeführt.

## 4. Datendateien

| Daten | Quelle/Lizenz |
|---|---|
| `geoip.dat` / `geosite.dat` | Wie vom Datenquellen-Repository deklariert (üblicherweise GPL-3.0 oder CC-BY-SA) |
| GeoLite2 (`GeoLite2-ASN.mmdb` usw.) | MaxMind GeoLite2 EULA / CC BY-SA 4.0; Überprüfen Sie vor der kommerziellen Nutzung die Lizenzierung |
| `country.mmdb` | Wie in der Datenquelle angegeben |

## 5. Compliance-Empfehlungen

- Wenn Sie Binärdateien bereitstellen, schließen Sie Folgendes ein: `LICENSE`, diese Datei und den Lizenztext oder Erwerb URL jeder GPL/MPL-Komponente.
- Von diesem Projekt erstellte Binärdateien (sing-box / Caddy / Nginx) werden mit dem Panel geliefert: Da sie aus der Upstream-Quelle kompiliert werden, veröffentlichen Sie, wie sie erstellt wurden (Version, Build-Tags, Konfigurationsflags). GPL-Komponenten (sing-box) erfordern zusätzlich die vollständige entsprechende Quelle dieser Version oder einen gültigen Erwerbsweg.
- Wenn Sie Quelldateien von MPL-2.0-Komponenten wie `xray-core` ändern, veröffentlichen Sie die Quelle der geänderten Dateien.
- Wenn Sie GPL-Programme wie sing-box mit Ihrem Produkt vertreiben, stellen Sie den Empfängern die vollständige entsprechende Quelle dieser Version (oder ein wirksames schriftliches Angebot) zur Verfügung.
- Marken und Namen: Verwenden Sie Namen oder Logos von Upstream-Projekten nicht in einer Weise, die den Eindruck einer offiziellen Billigung erweckt.
