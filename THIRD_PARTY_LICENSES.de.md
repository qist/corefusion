<!-- source: THIRD_PARTY_LICENSES.md | locale: de | updated: 2026-10-23 -->
# Komponenten und Lizenzen von Drittanbietern

> Sprachen: [简体中文](THIRD_PARTY_LICENSES.md) · [English](THIRD_PARTY_LICENSES.en.md) · [繁體中文](THIRD_PARTY_LICENSES.zh-TW.md) · [Русский](THIRD_PARTY_LICENSES.ru.md) · [日本語](THIRD_PARTY_LICENSES.ja.md) · [한국어](THIRD_PARTY_LICENSES.ko.md) · Deutsch · [Français](THIRD_PARTY_LICENSES.fr.md) · [Español](THIRD_PARTY_LICENSES.es.md) · [Português](THIRD_PARTY_LICENSES.pt.md) · [Türkçe](THIRD_PARTY_LICENSES.tr.md) · [Bahasa Indonesia](THIRD_PARTY_LICENSES.id.md) · [ไทย](THIRD_PARTY_LICENSES.th.md) · [Tiếng Việt](THIRD_PARTY_LICENSES.vi.md) · [العربية](THIRD_PARTY_LICENSES.ar.md) · [فارسی](THIRD_PARTY_LICENSES.fa.md)

CoreFusion verwendet die folgenden Open-Source-Komponenten. In dieser Datei sind die Hauptkomponenten und Lizenzen aufgeführt. **Die vollständige Liste wird durch `go.mod` / `go.sum` und die eigene LIZENZdatei jedes Moduls definiert**.

Wenn Sie Binärdateien dieses Projekts verteilen, bewahren Sie diese Datei, das Projekt `LICENSE` und die Urheberrechts-/Lizenzhinweise aller Komponenten Dritter auf.

## 1. In die Binärdatei (Haupt) kompilierte Go-Abhängigkeiten

| Komponente | Lizenz | Zweck |
|---|---|---|
| `github.com/xtls/xray-core` | MPL-2.0 | Xray-Konfigurationsanalyse, gRPC-Befehle, Protokolldefinitionen |
| `github.com/xtls/reality` | MPL-2.0 | Unterstützung des REALITY-Protokolls |
| `github.com/sagernet/sing` | **GPL-3.0 oder höher** | Indirekt verbunden über xray-core |
| `github.com/sagernet/sing-shadowsocks` | **GPL-3.0 oder höher** | Indirekt verknüpft über xray-core (Shadowsocks-2022) |
| `github.com/go-sql-driver/mysql` | MPL-2.0 | MySQL-Treiber |
| `github.com/hashicorp/golang-lru/v2` | MPL-2.0 | LRU-Cache |
| `github.com/go-acme/lego/v4` | MIT | Ausstellung des ACME-Zertifikats |
| `github.com/gin-gonic/gin` | MIT | HTTP-Framework |
| `gorm.io/gorm` | MIT | ORM |
| `github.com/aws/aws-sdk-go-v2` | Apache-2.0 | S3-Backups, Route53 DNS-01 |
| `google.golang.org/grpc` | Apache-2.0 | Agent-gRPC-Kommunikation |
| `google.golang.org/protobuf` | BSD-3-Klausel | Protobuf-Laufzeit |
| `golang.org/x/crypto` | BSD-3-Klausel | Kryptografische Grundelemente |
| `golang.org/x/oauth2` | BSD-3-Klausel | OAuth2 / Google EAB |
| `github.com/golang-jwt/jwt/v5` | MIT | JWT |
| `github.com/google/uuid` | BSD-3-Klausel | UUID |

> ⚠ **GPL-3.0-Komponenten**: `sing` und `sing-shadowsocks` sind GPL-3.0 oder höher. Sie sind statisch mit den Panel- und Agent-Binärdateien verknüpft, sodass **für die Verteilung von Binärdateien die Bereitstellung der entsprechenden Quelle erforderlich ist** (siehe Abschnitt 3 von [LEGAL.md](THIRD_PARTY_LICENSES.de.md)).

## 2. Eigenständige Programme, die zur Laufzeit aufgerufen werden (nicht in diesem Repository verteilt)

Das Panel/der Agent kann zur Laufzeit die folgenden eigenständigen Programme herunterladen, installieren oder aufrufen. Sie interagieren über Prozesse/Schnittstellen mit diesem Projekt, aber wenn Sie sie mit Ihren Liefergegenständen versenden, müssen Sie dennoch ihre Lizenzen einhalten:

| Programm | Lizenz | Notizen |
|---|---|---|
| Xray-Kern | MPL-2.0 | Proxy-Kern, separater Prozess |
| sing-box | **GPL-3.0 oder höher** | Proxy-Kern, separater Prozess; Geben Sie bei der Weitergabe die entsprechende Quelle an |
| Caddy | Apache-2.0 | Reverse-Proxy / TLS |
| Nginx | BSD-2-Klausel | Reverse-Proxy (vom System installiert, falls verwendet) |

> Hysteria2 und TUIC sind **von sing-box getragene Protokolle**, keine eigenständigen Kerne oder Binärdateien, daher werden sie nicht separat aufgeführt.

## 3. Datendateien

| Daten | Quelle/Lizenz |
|---|---|
| `geoip.dat` / `geosite.dat` | Wie vom Datenquellen-Repository deklariert (üblicherweise GPL-3.0 oder CC-BY-SA) |
| GeoLite2 (`GeoLite2-ASN.mmdb` usw.) | MaxMind GeoLite2 EULA / CC BY-SA 4.0; Überprüfen Sie vor der kommerziellen Nutzung die Lizenzierung |
| `country.mmdb` | Wie in der Datenquelle angegeben |

## 4. Compliance-Empfehlungen

- Wenn Sie Binärdateien bereitstellen, schließen Sie Folgendes ein: `LICENSE`, diese Datei und den Lizenztext oder Erwerb URL jeder GPL/MPL-Komponente.
- Wenn Sie Quelldateien von MPL-2.0-Komponenten wie `xray-core` ändern, veröffentlichen Sie die Quelle der geänderten Dateien.
- Wenn Sie GPL-Programme wie sing-box mit Ihrem Produkt vertreiben, stellen Sie den Empfängern die vollständige entsprechende Quelle dieser Version (oder ein wirksames schriftliches Angebot) zur Verfügung.
- Marken und Namen: Verwenden Sie Namen oder Logos von Upstream-Projekten nicht in einer Weise, die den Eindruck einer offiziellen Billigung erwecken könnte.
