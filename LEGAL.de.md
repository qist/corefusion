<!-- source: LEGAL.md | locale: de | updated: 2026-10-23 -->
# Rechtliche Hinweise und Compliance-Hinweise

> Diese Übersetzung dient nur der Orientierung. Bei Abweichungen ist der chinesische Originaltext maßgeblich.

> Sprachen: [简体中文](LEGAL.md) · [English](LEGAL.en.md) · [繁體中文](LEGAL.zh-TW.md) · [Русский](LEGAL.ru.md) · [日本語](LEGAL.ja.md) · [한국어](LEGAL.ko.md) · Deutsch · [Français](LEGAL.fr.md) · [Español](LEGAL.es.md) · [Português](LEGAL.pt.md) · [Türkçe](LEGAL.tr.md) · [Bahasa Indonesia](LEGAL.id.md) · [ไทย](LEGAL.th.md) · [Tiếng Việt](LEGAL.vi.md) · [العربية](LEGAL.ar.md) · [فارسی](LEGAL.fa.md)

> In diesem Dokument werden die Lizenz von CoreFusion, die Richtlinie zur kommerziellen Nutzung, die Verpflichtungen Dritter und die Compliance-Punkte erläutert, die Benutzer in ihren eigenen Gerichtsbarkeiten beachten sollten.
>
> **Dieses Dokument stellt keine Rechtsberatung dar.** Die Einhaltung hängt von Ihrer Bereitstellungsregion, Ihren Kunden und Ihrem Betriebsmodell ab. Konsultieren Sie einen qualifizierten Anwalt vor Ort.
>
> *Diese Übersetzung dient der Vereinfachung. Bei Abweichungen vom chinesischen Original gilt die chinesische Version.*

## 1. Projektlizenz

CoreFusion wird unter der **GNU Affero General Public License v3.0 (AGPL-3.0)** veröffentlicht; Den vollständigen Text finden Sie unter [LICENSE](./LICENSE).

```
Copyright (C) 2026 qist

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published
by the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
```

## 2. Kommerzielle Nutzung und kommerzielles Hosting: ausdrücklich erlaubt

**Jede Einzelperson oder Organisation darf CoreFusion für kommerzielle Zwecke nutzen, einschließlich, aber nicht beschränkt auf:**

- Selbsthosting und interne Bereitstellung;
- Betrieb eines kostenpflichtigen Proxy-/Knotendienstes für Dritte;
- Bereitstellung von SaaS, Hosting, verwaltetem Betrieb oder Systemintegration für Dritte gegen Gebühr;
- Sekundärentwicklung, Anpassung und Vertrieb (einschließlich Lieferung mit Hardware oder als Teil einer Lösung).

**Für die oben genannte kommerzielle Nutzung und das kommerzielle Hosting ist keine gesonderte Genehmigung und keine Lizenzgebühr oder Umsatzbeteiligung erforderlich.**

Die einzigen Verpflichtungen sind die von AGPL-3.0 selbst, hauptsächlich zwei:

1. **Beim Verteilen von Binärdateien** (§4–§6): Stellen Sie den Empfängern die entsprechende Quelle (vollständige entsprechende Quelle) oder ein wirksames schriftliches Angebot gemäß §6 zur Verfügung; Urheberrechtshinweise und den Lizenztext behalten.
2. **Bei der Bereitstellung einer modifizierten Version über ein Netzwerk** (§13): Wenn Sie eine modifizierte Version veröffentlichen und Dritte über ein Netzwerk damit interagieren lassen, müssen Sie **diesen Benutzern die entsprechende Quelle der modifizierten Version kostenlos anbieten** (z. B. einen prominenten Quell-Download-/Repository-Link im Panel).

Zusätzliche Hinweise:

- Die Bereitstellung einer **unveränderten** Kopie und die Bereitstellung von Benutzern löst für sich genommen nicht die Quellenpflicht gemäß §13 aus; Das Verteilen von Binärdateien fällt immer noch unter Punkt 1.
- Das Aufladen von Geld ist nicht verboten; AGPL regelt die **Quellenverfügbarkeit**, nicht das Geschäftsmodell. Sie können §13 für eine geänderte Version nicht einfach deshalb ablehnen, weil Sie kommerziell oder Closed-Source arbeiten.
- Wenn Sie eine **kommerzielle Closed-Source-Lizenz wünschen, die nicht der AGPL unterliegt**, wenden Sie sich separat an den Urheberrechtsinhaber. Beachten Sie, dass dieses Projekt GPL-3.0-Komponenten von Drittanbietern verknüpft (siehe Abschnitt 3); Jede Closed-Source-Version muss sie zuerst entfernen.

## 3. Komponenten von Drittanbietern und Weiterverbreitung

Die wichtigsten Komponenten von Drittanbietern und ihre Lizenzen sind in [THIRD_PARTY_LICENSES.md](LEGAL.de.md) aufgeführt.

Besondere Hinweise:

- Über `github.com/xtls/xray-core` verknüpft dieses Projekt indirekt **GPL-3.0 oder höher** `github.com/sagernet/sing` und `sing-shadowsocks`. Daher erfordert **das Verteilen von Panel- oder Agent-Binärdateien auch die Erfüllung der entsprechenden Quellverpflichtungen von GPL-3.0**.
- Zur Laufzeit kann das Panel eigenständige Programme wie Xray-core, sing-box und Caddy herunterladen oder aufrufen. Diese werden **nicht in diesem Repository verteilt**, aber wenn Sie sie mit Ihren Lieferungen versenden, müssen Sie deren Lizenzen einhalten (insbesondere sing-boxs GPL-3.0: Geben Sie die entsprechende Quelle an). Hysteria2 und TUIC sind Protokolle, die von sing-box getragen werden, keine eigenständigen Binärdateien.
- Gebündelte GeoIP-/GeoSite-Daten (GeoLite2, `geoip.dat`, `geosite.dat`) unterliegen eigenen Datenlizenzen; Überprüfen Sie die Bedingungen vor der kommerziellen Nutzung.

## 4. Regionale Compliance und Benutzerverantwortung

CoreFusion ist **Allzwecksoftware**: Sie stellt keinen Proxy oder Netzwerkdienst bereit, versendet keine Knoten, Bandbreite oder Anmeldeinformationen und erhebt keine Gebühren. Der Betrieb liegt in der Verantwortung des Bereitstellers.

Betreiber/Betreiber müssen feststellen, ob für ihr Unternehmen in ihrem Zuständigkeitsbereich Lizenzen, Einreichungen oder Sicherheitsbewertungen erforderlich sind, einschließlich, aber nicht beschränkt auf:

- **Festlandchina**: Mehrwertlizenzen für Telekommunikationsunternehmen (ICP/IDC/ISP usw.), ICP-Einreichung, MLPS, Sicherheitsbewertung der grenzüberschreitenden Datenübertragung sowie Verpflichtungen gemäß dem Cybersicherheitsgesetz, dem Datensicherheitsgesetz und dem Gesetz zum Schutz personenbezogener Daten. Verwenden Sie diese Software nicht für Zwecke, die gegen lokale Gesetze verstoßen.
- **EU/UK**: DSGVO/UK DSGVO, ePrivacy, Digital Services Act (DSA) Verarbeiter-/Controller-Pflichten, Protokollierung und Cookie-Compliance.
- **Vereinigte Staaten**: staatliche Datenschutzgesetze, Verschlüsselungsexportkontrollen (EAR) und Sanktionsprüfung.
- **Andere Regionen**: lokale Telekommunikation, verschlüsselte Kommunikation, Inhaltsverwaltung, Datenlokalisierung und Verbraucherschutzregeln.

Benutzer erklären sich damit einverstanden, diese Software nicht für Folgendes zu verwenden:

- Aktivitäten, die gegen geltendes Recht oder Exportkontroll-/Sanktionsvorschriften verstoßen;
- Verletzung des geistigen Eigentums, der Privatsphäre oder anderer gesetzlicher Rechte;
- Verbreitung illegaler Inhalte, Netzwerkangriffe, Betrug oder andere kriminelle Aktivitäten.

**Jede rechtliche Haftung, die sich aus der Nutzung oder dem Missbrauch dieser Software ergibt, liegt beim Benutzer.**

## 5. Datenschutz und Privatsphäre

Das Panel kann Konten, Kontaktdaten, Bestellungen, Zahlungsrückrufe, Abonnements, Verkehrsstatistiken und Anmelde-/Vorgangsprotokolle speichern. Als Auftragsverarbeiter/Verantwortlicher müssen Betreiber:

- Stellen Sie den Benutzern eine Datenschutzrichtlinie zur Verfügung und holen Sie die erforderliche Einwilligung ein.
- Befolgen Sie die Datenminimierung und legen Sie angemessene Aufbewahrungsfristen für Protokolle und Statistiken fest (das Projekt stellt Protokollbereinigungsrichtlinien bereit, die Betreiber konfigurieren müssen);
- Supportanfragen zum Zugriff, zur Korrektur und Löschung personenbezogener Daten;
- Wenden Sie Zugriffskontrolle und Verschlüsselung auf Datenbanken, Backups, private Schlüssel von Zertifikaten und Zahlungsanmeldeinformationen an.
- Führen Sie bei der grenzüberschreitenden Bereitstellung grenzüberschreitende Datenübertragungsbewertungen durch.

## 6. Haftungsausschluss

Diese Software wird „wie besehen“ gemäß AGPL-3.0 §§15–17 bereitgestellt, ohne jegliche ausdrückliche oder stillschweigende Garantie, einschließlich Marktgängigkeit, Eignung für einen bestimmten Zweck und Nichtverletzung. Der Urheberrechtsinhaber haftet nicht für direkte, indirekte, zufällige oder Folgeschäden, die sich aus der Nutzung dieser Software ergeben.

## 7. Kontakt

- Projekt: <https://github.com/qist/corefusion>
- Fragen zur kommerziellen Lizenzierung/Compliance: Nutzen Sie die Projektprobleme oder die auf der Repository-Seite veröffentlichten Kontaktdaten.
