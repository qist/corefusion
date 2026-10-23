<!-- source: notification-templates.md | locale: de | updated: 2026-10-23 -->
# Variablen der Benachrichtigungsvorlage

> Sprachen: [简体中文](notification-templates.md) · [English](notification-templates.en.md) · [繁體中文](notification-templates.zh-TW.md) · [Русский](notification-templates.ru.md) · [日本語](notification-templates.ja.md) · [한국어](notification-templates.ko.md) · Deutsch · [Français](notification-templates.fr.md) · [Español](notification-templates.es.md) · [Português](notification-templates.pt.md) · [Türkçe](notification-templates.tr.md) · [Bahasa Indonesia](notification-templates.id.md) · [ไทย](notification-templates.th.md) · [Tiếng Việt](notification-templates.vi.md) · [العربية](notification-templates.ar.md) · [فارسی](notification-templates.fa.md)

Benachrichtigungsvorlagen verwenden `{variable}`-Platzhalter, die beim Senden durch tatsächliche Werte ersetzt werden. Dieses Dokument listet alle unterstützten Variablen, die pro Ereignis verfügbaren Variablen und die automatischen Auslöseregeln auf.

## 1. Vorlagentypen und Zielgruppen

| Publikum | Kanal | Empfänger | Typische Verwendung |
|---|---|---|---|
| **Panel-Benutzer** (Administrator) | Telegram / E-Mail | Vom Administrator konfigurierte Kanäle und Empfänger | Knotenwarnungen, Backup-Berichte, Systemankündigungen |
| **Abonnenten** | E-Mail | Die eigene E-Mail-Adresse des Kunden (wird automatisch ausgefüllt) | Niedriges Guthaben, Erinnerungen zum Ablauf des Abonnements |

## 2. Gemeinsame Variablen

Verfügbar in allen Vorlagen:

| Variabel | Beschreibung | Beispiel |
|---|---|---|
| `{time}` | Zeit senden | `2026-09-28 14:30:00` |
| `{user}` | Kunden-E-Mail (Abonnentenvorlagen)/Benutzerkennung | `user@example.com` |
| `{message}` | Benachrichtigungsstelle | Text der Wartungsankündigung |

## 3. Abonnentenereignisvariablen

### Erinnerung zum Ablauf des Abonnements (event_key = `subscription_expire`)

Täglich gescannt und an aktive Abonnenten gesendet, die innerhalb von 7 Tagen ablaufen:

| Variabel | Beschreibung | Beispiel |
|---|---|---|
| `{user}` | Kunden-E-Mail | `user@example.com` |
| `{expire_date}` | Ablauf des Abonnements | `2026-10-05 09:00` |
| `{days}` | Tage verbleibend | `7` |
| `{time}` | Zeit senden | `2026-09-28 09:05:00` |

### Erinnerung bei niedrigem Guthaben (event_key = `balance_low`)

Täglich gescannt und an Kunden gesendet, deren Guthaben unter dem Schwellenwert liegt:

| Variabel | Beschreibung | Beispiel |
|---|---|---|
| `{user}` | Kunden-E-Mail | `user@example.com` |
| `{balance}` | Aktueller Kontostand (Währungseinheiten, zwei Dezimalstellen) | `0.85` |
| `{time}` | Zeit senden | `2026-09-28 09:05:00` |

## 4. Beispielvariablen für Operationsvorlagen

Wenn Sie für einen Konnektivitätstest auf „Senden“ klicken, füllen Panel-Benutzervorlagen Platzhalter mit diesen Beispielen:

| Variabel | Beschreibung | Beispiel |
|---|---|---|
| `{node}` | Knotenname | `node-01` |
| `{backup_file}` | Name der Sicherungsdatei | `corefusion-20260101-0900.tar.gz` |
| `{size}` | Dateigröße | `128MB` |
| `{domain}` | Domäne | `example.com` |
| `{expire_date}` | Ablaufdatum | `2026-10-12` |
| `{days}` | Tage verbleibend | `14` |
| `{count}` | Zählen | `3` |

## 5. Automatische Auslöseregeln

- **Zeitplan**: Der erste Scan wird 30 Sekunden nach dem Start des Panels ausgeführt, dann täglich um **09:05**
- **Abonnementablauf**: Scannt aktive Abonnements (`status=active`), die innerhalb von 7 Tagen ablaufen; Der Kunde muss ebenfalls aktiv sein
- **Geringes Guthaben**: Scannt aktive Kunden mit einem Guthaben ≤ Schwellenwert; Der Schwellenwert ist `notify.balance_threshold`, **in Cent** (Standard `100` = 1,00)
- **Deduplizierung**: höchstens eine Nachricht pro Kunde und Ereignis **pro Tag** (`notification_send_logs` dedupliziert pro Tag)

## 6. Überwachung des Knotenzustands

Knoten-Heartbeats werden kontinuierlich überprüft; **Benachrichtigungen werden nur gesendet, wenn sich der Status ändert – Runden ohne Änderung sind still**:

| Einstellschlüssel | Beschreibung | Standard |
|---|---|---|
| `monitor.node_interval_seconds` | Prüfintervall (Sekunden) | `60` (mindestens 30) |
| `monitor.node_offline_timeout_seconds` | Heartbeat-Timeout (Sekunden) | `120` |

- **Offline-Warnung**: Wenn ein Knoten online in offline wechselt, wird die Admin-Vorlage „**Knoten-Offline-Warnung**“ mit `{node}` (Name) und `{time}` gesendet
- **Wiederherstellungsbenachrichtigung**: Wenn ein Knoten wiederhergestellt wird, wird „**Knotenwiederherstellungsbenachrichtigung**“ mit denselben Variablen gesendet
- **Stille Runden**: keine Meldung, wenn der Zustand unverändert ist; Nach einem Panel-Neustart wird in der ersten Runde lediglich eine Grundlinie festgelegt
- **Überwachen ohne Senden**: Deaktivieren Sie die entsprechende Vorlage (Aktiviert = AUS)

## 7. Monatliche Node-Traffic-Kontingentwarnungen (automatisch)

- **Auslöser**: Wenn der monatliche Datenverkehr eines Knotens den Kontingentschwellenwert erreicht, wird „**Knotenverkehrskontingentwarnung**“ gesendet; Wenn es 100 % überschreitet und ausgesetzt wird, wird eine Überschreitungswarnung gesendet (Ereignisschlüssel `traffic_quota_threshold` / `traffic_quota_exceeded`).
- **Schwellenwert**: `monitor.traffic_quota_threshold_percent` (Prozentsatz, Standardwert `80`; legen Sie `0` fest, um Schwellenwertwarnungen zu deaktivieren)
- **Deduplizierung**: höchstens eine pro Knoten und Abrechnungszyklus (der Zyklus folgt dem `traffic_quota_reset_day` des Knotens)
- Verfügbare Variablen: `{node}` (Name), `{used}` (GB verwendet), `{quota}` (GB-Kontingent), `{percent}` (Prozentsatz), `{time}`

## 8. Benachrichtigungen über Sicherungsergebnisse (automatisch)

- **Auslöser**: wird unmittelbar nach der Ausführung einer geplanten Sicherung (`BackupScheduler`) gesendet – bei Erfolg wird „**Täglicher Sicherungsbericht**“ gesendet, bei Fehler (Erstellen der Sicherung / Lesen des Speichers / Hochladen auf Remote) wird „**Warnung zu Sicherungsfehlern**“ gesendet.
- Verfügbare Variablen: `{backup_file}` (Name), `{size}` (Größe, nur Erfolgsbericht), `{message}` (Fehlergrund, nur Fehlerwarnung), `{time}`
- **Überwachen ohne Senden**: wird stillschweigend übersprungen, wenn die Vorlage deaktiviert ist (Aktiviert = AUS)

## 9. Erinnerungen zum Ablauf des Zertifikats (automatisch)

- **Auslöser**: Täglich um 09:05 Uhr, scannt `active`-Zertifikate, die kurz vor dem Ablauf stehen (**IP-Zertifikate mit noch 3 Tagen**, **Domänenzertifikate mit 7 Tagen Rest**) und sendet die Admin-Vorlage „**Erinnerung an den Ablauf des Zertifikats**“ pro Zertifikat
- **Deduplizierung**: höchstens eine Erinnerung pro Zertifikat und Tag; Nach der Verlängerung verschiebt sich der Ablauf aus dem Zeitfenster (IP 3 Tage / Domain 7 Tage) und die Erinnerungen werden eingestellt
- Verfügbare Variablen: `{domain}` (Zertifikatdomäne), `{expire_date}` (Ablaufdatum), `{days}` (verbleibende Tage), `{time}`

## 10. Absturzwarnungen für Kernprozesse (automatisch)

- **Auslöser**: Wenn ein Kernprozess (xray/singbox) abnormal beendet wird oder der Proxy-Prozess (caddy/nginx) abstürzt, wird sofort die Admin-Vorlage „**Kernprozess-Absturzwarnung**“ gesendet. Der Supervisor startet es dann mit exponentiellem Backoff neu
- Verfügbare Variablen: `{core}` (Prozesstyp), `{restarts}` / `{max}` (Neustartanzahl/-limit), `{message}` (Exit-Nachricht, nur Kernprozesse), `{time}`
- **Überwachen ohne Senden**: wird stillschweigend übersprungen, wenn die Vorlage deaktiviert ist (Aktiviert = AUS)

## 11. Benachrichtigungen zur Registrierung neuer Benutzer (automatisch)

- **Auslöser**: wird sofort nach der erfolgreichen Registrierung eines Kunden gesendet, unter Verwendung der Admin-Vorlage „**Neuer Benutzerregistrierungshinweis**“
- Verfügbare Variablen: `{user}` (Kunden-E-Mail), `{time}`

## 12. Nutzung

1. **Vorlage hinzufügen**: Benachrichtigungseinstellungen → Benachrichtigungsvorlagen → Vorlage hinzufügen → „Aus Voreinstellung füllen“ und nach Bedarf anpassen
2. **Aktivieren**: Aktiviert ankreuzen und speichern; Automatische Trigger verarbeiten nur aktivierte Vorlagen
3. **Test**: Klicken Sie in der Liste auf „Senden“, um eine echte Nachricht mit Beispielvariablen zu senden (Test-E-Mails für Abonnentenvorlagen gehen an die Absenderadresse des Administrators).
4. **Sofortige Benachrichtigungen** (Geschäftscode): Rufen Sie `NotifyClientEvent(ctx, db, settingSvc, eventKey, clientEmail, vars)` am Veranstaltungsort an (Aufladung, Abzug, Verlängerung), um einen einzelnen Kunden sofort zu benachrichtigen, außerhalb des täglichen Scanlimits
