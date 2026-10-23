<!-- source: cert-google-eab.md | locale: de | updated: 2026-10-23 -->
# Google Public CA Guide (automatische EAB-Ausstellung)

> Sprachen: [简体中文](cert-google-eab.md) · [English](cert-google-eab.en.md) · [繁體中文](cert-google-eab.zh-TW.md) · [Русский](cert-google-eab.ru.md) · [日本語](cert-google-eab.ja.md) · [한국어](cert-google-eab.ko.md) · Deutsch · [Français](cert-google-eab.fr.md) · [Español](cert-google-eab.es.md) · [Português](cert-google-eab.pt.md) · [Türkçe](cert-google-eab.tr.md) · [Bahasa Indonesia](cert-google-eab.id.md) · [ไทย](cert-google-eab.th.md) · [Tiếng Việt](cert-google-eab.vi.md) · [العربية](cert-google-eab.ar.md) · [فارسی](cert-google-eab.fa.md)

> Gilt für: Panel v0.12.101+ | Seite: **Zertifikate → EAB-Konfiguration**
> Code: `internal/service/certificate/eab_google.go`, `internal/api/handlers/certificates/eab_config.go`

## 1. Warum Google EAB erfordert

| CA | EAB obligatorisch für ACME? | So erhalten Sie EAB |
|---|---|---|
| Lassen Sie uns verschlüsseln | ❌ Nein | — |
| ZeroSSL | ✅ Ja | **Eine E-Mail gegen eine eintauschen** (`api.zerossl.com/acme/eab-credentials-email`, dasselbe wie acme.sh); Das Panel kann es automatisch abrufen |
| **Google Öffentliche CA** | ✅ **Ja** | **Muss mit GCP-Autorisierung erstellt werden** (eine E-Mail reicht nicht aus) → der Schwerpunkt dieser Seite |

Mit anderen Worten: Google **kann eine „Autorisierung“ nicht vermeiden**: entweder eine Dienstkonto-JSON-Datei oder ein temporäres Zugriffstoken.

## 2. Die harten Regeln von Google (zuerst lesen)

| Regel | Beschreibung |
|---|---|
| **Innerhalb von 7 Tagen verbrauchen** | Ein EAB-Schlüssel **verfällt**, wenn er innerhalb von 7 Tagen nicht verwendet wird, und muss neu erstellt werden |
| **Nur ein Konto** | Ein EAB-Schlüssel registriert genau ein ACME-Konto; Nach der Registrierung ist der Schlüssel ungültig und kann nicht wiederverwendet werden |
| **ACME-Konten laufen nicht ab** | Das damit erstellte ACME-Konto ist **langlebig** → **Verlängerungen sind nicht erforderlich EAB** |
| ⚠ **Durch das Löschen des GCP-Projekts werden alle Konten gelöscht** | Durch das Löschen des Projekts werden **alle darin enthaltenen ACME-Konten** sofort ungültig und **eine Verlängerung wird unmöglich** |

> Fazit: **EAB ist eine einmalige „Onboarding-Berechtigung“; Das ACME-Konto ist das, was bleibt.**
> Das Panel speichert Konten unter `data/acme/accounts/<ca>.json` – **sichern Sie dieses Verzeichnis**.

## 3. Drei Pfade (beginnen Sie mit Pfad A – keine Menüsuche)

> Die Einstiegspunkte der Cloud Console haben sich mehrfach geändert, sodass direkte Links und Cloud Shell an erster Stelle stehen. Menüpfade sind Fallbacks.

### Pfad A (am einfachsten, empfohlen zuerst): ein Befehl in GCP Cloud Shell

Cloud Shell ist ein Browser-Terminal mit **gcloud vorinstalliert und bereits angemeldet**; Keine lokale Einrichtung und keine Menüs.

1. Öffnen Sie <https://console.cloud.google.com/cloudshell> (oder das `>_`-Symbol oben rechts in der Konsole).
2. Wählen Sie das Projekt aus (die Projekt-ID ist in der Projektauswahl oben sichtbar):
   ```bash
   gcloud config set project <your-project-id>
   ```
3. Aktivieren Sie die öffentliche Zertifizierungsstelle API (**entspricht** der Aktivierung in der API-Bibliothek):
   ```bash
   gcloud services enable publicca.googleapis.com
   ```
4. Erstellen Sie EAB (**dies sind die beiden Zeichenfolgen, die Sie benötigen**):
   ```bash
   gcloud publicca external-account-keys create
   ```
 In der Ausgabe: `keyId` → die **EAB-KID** des Panels; `b64MacKey` → der **EAB-HMAC-Schlüssel** des Panels
5. Zurück im Panel: **Zertifikate → EAB-Konfiguration → EAB-Konfiguration hinzufügen → CA: Google Public CA** →
 Geben Sie die beiden Werte in `EAB-KID` / `EAB-HMAC-Key` ein → Speichern ✓

> ⚠ Kosten: Google EAB-Schlüssel gelten **7 Tage, einmalig** ✗ → nach 7 Tagen (oder beim Kontowechsel) **Schritt 4 wiederholen**.
> Um dies zu vermeiden, verwenden Sie Pfad B (speichern Sie einmal ein Dienstkonto und lassen Sie das Panel automatisch erneut anfordern).

### Pfad B (einmalige Einrichtung, dann automatisch): Dienstkontoschlüssel JSON

**① Aktivieren Sie die öffentliche CA API** (direkter Link ist am einfachsten)
<https://console.cloud.google.com/apis/library/publicca.googleapis.com?project=YOUR_PROJECT_ID> → Klicken Sie auf „Aktivieren“
Fallback: ☰ → **APIs & Services → Bibliothek** → Suche `Public CA` → Öffnen → „Aktivieren“

**② Erstellen Sie ein Dienstkonto**
<https://console.cloud.google.com/iam-admin/serviceaccounts?project=YOUR_PROJECT_ID>
→ „**+ Dienstkonto erstellen**“ → beliebiger Name (z. B. `corefusion-eab`) → „Erstellen und fortfahren“ → Sie können den Rollenschritt **überspringen** → „Fertig“
Fallback: ☰ → **IAM & Admin → Dienstkonten**

**③ Gewähren Sie die minimale Rolle** (am einfachsten auf derselben Seite)
Klicken Sie auf das neu erstellte Konto → **Registerkarte „Berechtigungen“** → „Zugriff gewähren“ → neuer Prinzipal: die **E-Mail-Adresse des Dienstkontos**
→ Suchen Sie die Rolle nach `publicca` → Wählen Sie **„Public CA External Account Key Creator“** → Speichern
Fallback: ☰ → **IAM & Admin → IAM** → „Zugriff gewähren“ → gleiche E-Mail + Rolle
⚠ **Kein Eigentümer/Herausgeber gewähren** ✗

**④ Laden Sie den Schlüssel JSON herunter**
Dienstkonten → Konto öffnen → **Registerkarte „Schlüssel“** → „**Schlüssel hinzufügen**“ → „**Neuen Schlüssel erstellen**“ → **JSON** → erstellen
(Der Browser lädt einen `xxx.json` herunter; oder Zeile `⋮` → „Schlüssel verwalten“ → gleicher Ablauf)

**⑤ Zurück im Panel**: Geben Sie die **Projekt-ID** ein und fügen Sie den **gesamten Inhalt** des JSON in „Dienstkonto-Schlüsseldatei“ ein → klicken Sie auf „EAB automatisch anfordern“
→ Danach fordert das Panel EAB automatisch erneut an**, wenn es abläuft ✓, keine GCP-Besuche mehr ✓

### Pfad C (nur einmaliger Test): Fügen Sie ein Zugriffstoken ein

In Cloud Shell:
```bash
gcloud auth print-access-token
```
Fügen Sie die Ausgabe in das „Temporäre Zugriffstoken“ des Panels sowie die Projekt-ID ein → klicken Sie auf „EAB automatisch anfordern“
⚠ Gültig für ca. 1 Stunde und **nicht gespeichert** ✓ → nur Verifizierung, keine automatische Neuanforderung ✗

### Spickzettel, wenn Sie keinen Einstiegspunkt finden (Konsolenmenüs ändern sich)

| Aufgabe | Direkter Link (am zuverlässigsten) | Fallback: Menü |
|---|---|---|
| Aktivieren Sie die öffentliche CA API | `https://console.cloud.google.com/apis/library/publicca.googleapis.com?project=<PROJECT_ID>` | ☰ → APIs & Dienste → Bibliothek → Suche `Public CA` |
| Dienstkonten | `https://console.cloud.google.com/iam-admin/serviceaccounts?project=<PROJECT_ID>` | ☰ → IAM & Admin → Dienstkonten |
| Rolle gewähren | `https://console.cloud.google.com/iam-admin/iam?project=<PROJECT_ID>` | ☰ → IAM & Admin → IAM → Zugriff gewähren |
| Cloud Shell | `https://console.cloud.google.com/cloudshell` | `>_`-Symbol oben rechts |

> Tipp: Ersetzen Sie `<PROJECT_ID>` / `YOUR_PROJECT_ID` durch die echte Projekt-ID (z. B. `my-project-123456`), um direkt zur Seite zu gelangen.

## 4. Konfigurieren des Panels (zwei Möglichkeiten)

Gehen Sie zu **Zertifikate → EAB-Konfiguration → EAB-Konfiguration hinzufügen** und wählen Sie **Google Public CA**:

### Option A (empfohlen): Fügen Sie das Dienstkonto JSON ein

| Feld | Wert |
|---|---|
| GCP-Projekt-ID | Die Projekt-ID aus Abschnitt 3, z.B. `my-project-123456` |
| Dienstkonto JSON | Fügen Sie den **gesamten Inhalt** von `corefusion-eab.json` ein |
| Temporäres Zugriffstoken | Leer lassen |
| Name | Automatisch generiert als `Google Public CA - <project-id>`; editierbar |

Klicken Sie auf **EAB automatisch anfordern** → bei Erfolg werden die KID/HMAC-Schlüssel ausgefüllt und gespeichert; Wählen Sie später bei der Ausstellung einfach `CA = Google Public CA` aus (EAB ist pro Zertifizierungsstelle zugeordnet; keine manuelle Auswahl).

> Das Dienstkonto JSON ist **in der EAB-Konfiguration der Panel-Datenbank verschlüsselt** (`json:"-"`, **nie an das Frontend zurückgegeben**).
> Der Zweck besteht darin, **nach Ablauf von 7 Tagen automatisch einen Schlüssel erneut anzufordern**, anstatt manuelle GCP-Besuche durchzuführen.

### Option B: temporäres Zugriffstoken (einmalig)

```bash
gcloud auth print-access-token        # valid ~1 hour
```

Fügen Sie es in „Temporäres Zugriffstoken“ ein, geben Sie die Projekt-ID ein und klicken Sie auf **EAB automatisch anfordern**.
⚠ Dieser Token wird **nicht gespeichert** und muss nach Ablauf erneut erworben werden → gut für einen einmaligen Test, nicht für den Langzeitgebrauch.

### Befehlszeilenäquivalent (optional, zum Vergleich)

```bash
gcloud publicca external-account-keys create
# returns keyId / b64MacKey, corresponding to ACME's --eab-kid / --eab-hmac-key
```

## 4.5 Warum JSON statt „ein einzelner Schlüssel“?

Dies ist die häufigste Frage. Trennen Sie die beiden Arten von „Schlüsseln“:

| Was Sie erwarten könnten | Was tatsächlich benötigt wird |
|---|---|
| **API-Schlüssel** (die `AIza…`-Zeichenfolge) | ❌ **Nutzlos** – ein API-Schlüssel ist nur eine öffentliche Kennung und **trägt keine Identität**; Für die Erstellung von EAB sind **IAM-Berechtigungen** erforderlich, daher sind identitätsbestätigende Anmeldeinformationen erforderlich |
| — | ✅ **Dienstkontoschlüssel**: die **Datei**, die von GCP heruntergeladen wurde „IAM → Dienstkonten → Schlüssel → Neuen Schlüssel erstellen → **JSON**“ |

Kernpunkt: **Die JSON-Datei selbst ist „ein Schlüssel“** – GCP packt die Kontoidentität (`client_email`) plus den privaten Schlüssel (`private_key`) in eine Datei. Das Panel signiert damit ein JWT → tauscht gegen ein OAuth2-Zugriffstoken → ruft die Public CA API auf.
Also „JSON einfügen“ ≠ viele Felder ausfüllen; **Fügen Sie die gesamte Datei ein** (das Panel füllt auch optionale Felder wie `type` / `token_uri` aus).

**Der echte EAB sind die beiden Strings** (`EAB-KID` + `EAB-HMAC-Key`), die das **Ergebnis** der Anfrage sind; Das Panel füllt sie automatisch aus.

### Sie möchten die Akte nicht herausgeben? Zwei Alternativen

| Methode | Wie | Kosten |
|---|---|---|
| **Temporäres Zugriffstoken** | `gcloud auth print-access-token` → Einfügen in „Temporäres Zugriffstoken“ | ~1 Stunde Gültigkeit, **nicht gespeichert** → nur einmalig, keine automatische Neuanforderung |
| **Handbuch KID/HMAC** | Führen Sie `gcloud publicca external-account-keys create` lokal aus → geben Sie `keyId` / `b64MacKey` in „EAB-KID / EAB-HMAC-Key“ ein. | Keine Zeugnisübergabe ✓; aber Google EAB ist **7 Tage, einmalig** → Wiederholung nach Ablauf |

> Das Gremium empfiehlt das Dienstkonto JSON aus einem Grund: **EAB läuft nach 7 Tagen ab** und nur ein gespeichertes Dienstkonto ermöglicht **automatische Neuanforderung** ohne manuelle Arbeit.

## 4.6 FAQ: Kann ich einen API-Schlüssel verwenden?

**Nein.** Dies ist keine Implementierungsauswahl – **Google lehnt sie ausdrücklich ab** (in beide Richtungen überprüft):

```bash
# 1) Query parameter
curl -X POST "https://publicca.googleapis.com/v1/projects/<ID>/locations/global/externalAccountKeys?key=AIza..." -d '{}'
# 2) x-goog-api-key header recommended by the official docs
curl -X POST "https://publicca.googleapis.com/v1/projects/<ID>/locations/global/externalAccountKeys" \
     -H "x-goog-api-key: AIza..." -d '{}'
```

Beide geben zurück:

```json
{"error":{"code":401,"status":"UNAUTHENTICATED",
  "message":"API keys are not supported by this API. Expected OAuth2 access token
             or other authentication credentials that assert a principal."}}
```

Zum Vergleich wird ein **(gefälschter) OAuth-Bearer** zurückgegeben
`Request had invalid authentication credentials. Expected OAuth 2 access token...` —
Daher wird OAuth erkannt, während **API-Schlüssel komplett abgelehnt werden** ✗.

**Warum**: Das Erstellen von EAB ist ein **administrativer Vorgang**, bei dem Sie wissen müssen, „wer Sie sind und ob Sie `roles/publicca.externalAccountKeyCreator` haben“ ✗
– ein API-Schlüssel identifiziert nur ein Projekt/eine Anwendung ✓ und **drückt keine Hauptidentität aus** ✓.
(Googles „Best Practices für die Verwaltung von API-Schlüsseln“ empfiehlt **IAM + kurzlebige Anmeldeinformationen für Dienstkonten** für Verwaltungsszenarien ✓ – derselbe Ansatz, der hier verwendet wird ✓.)

**Es gibt also nur drei Pfade** ✓ (siehe §3): service-account JSON ✓ / temporäres Zugriffstoken ✓ /
oder **überhaupt keine Anmeldeinformationen** – führen Sie `gcloud publicca external-account-keys create` in Cloud Shell aus ✓
und geben Sie `keyId` / `b64MacKey` in das Panel ein ✓ (**verwenden Sie dies, wenn GCP IAM Sie blockiert** ✓).

## 5. Automatische Registrierung und Verlängerungen (wichtig)

```
Panel: CA = Google Public CA
  ├─ usable EAB present in the config → use it
  └─ missing/expired → request a new one on the spot with the stored service-account JSON (auto-registration)
        │
First issuance on the node/panel ── registers the ACME account with EAB (one-time!)
        │
Account persisted: data/acme/accounts/google.json (panel) / <Agent data_dir>/acme/accounts/google.json (node)
        │
Every later renewal ── uses the account key only; **EAB is never needed again**
```

Deshalb:

- **Die erste Ausstellung wird automatisch registriert**; es ist kein manueller GCP-Schritt erforderlich;
- Verlängerungen verbrauchen EAB **nicht** (solange die Kontodatei vorhanden ist und das Datenverzeichnis stabil bleibt);
- ⚠ Das agentenseitige Datenverzeichnis ACME ist ein fester absoluter Pfad (Standard `/var/lib/corefusion-agent/acme`);
 **Ändern Sie es nicht nachlässig**, sonst wird es als neues Konto behandelt → ein weiterer EAB-Schlüssel ist erforderlich.

## 6. Fehlerbehebung

| Symptom/Fehler | Ursache | Beheben |
|---|---|---|
| `PERMISSION_DENIED` (403) | Dem Dienstkonto fehlt `roles/publicca.externalAccountKeyCreator` | Gehen Sie zurück zu Abschnitt 3 ④ |
| `SERVICE_DISABLED` (403) | `publicca.googleapis.com` ist für das Projekt nicht aktiviert | Abschnitt 3 ② |
| `NOT_FOUND` (404) | Falsche Projekt-ID / Projekt gelöscht | Überprüfen Sie die Projekt-ID |
| Emissionsberichte `externalAccountRequired` | Diese CA hat kein EAB | Fordern/konfigurieren Sie zunächst EAB im Panel |
| Emissionsberichte `Accounts must have at least one contact.` | Bei der Registrierung wurde keine E-Mail-Adresse angegeben | Legen Sie die E-Mail-Adresse des ACME-Kontos auf der Zertifikatsseite fest (das Panel verwendet das Standardkonto). |
| Bei der Verlängerung wird erneut nach EAB gefragt / Registrierung schlägt fehl | ACME-Kontodatei verloren (Datenverzeichnis geändert/bereinigt) | **Backup wiederherstellen**; Wenn Sie kein Konto haben, fordern Sie über das Dienstkonto ein neues EAB an |
| Der Aussteller wechselt von Google zu Let's Encrypt | Beim Erneuerungsbefehl fehlt die CA (alter Fehler, behoben in v0.12.101) | Upgrade und Neuauflage |

## 7. Empfehlungen zu Sicherheit und Betrieb

1. **Geringeste Berechtigung**: Gewähren Sie dem Dienstkonto nur `roles/publicca.externalAccountKeyCreator`, niemals Besitzer/Herausgeber.
2. **Projekt nicht löschen**: Durch das Löschen des GCP-Projekts werden alle seine ACME-Konten ungültig → Zertifikate können nicht erneuert werden.
3. **Immer sichern** `data/acme/accounts/` (die Kontoidentität selbst) und die EAB-Konfiguration der Panel-Datenbank.
4. **Keine Panik wegen des Ablaufs von EAB**: Solange die ACME-Kontodatei vorhanden ist, sind Verlängerungen nicht betroffen; Ein neuer EAB wird nur benötigt, um ein neues Konto zu registrieren.
5. Wenn der Dienstkontoschlüssel möglicherweise durchgesickert ist: Löschen Sie diesen Schlüssel in GCP und fügen Sie einen neuen JSON in das Panel ein.

## 8. Zugehörige Endpunkte (für Skripterstellung)

| Endpunkt | Beschreibung |
|---|---|
| `POST /api/v1/eab-configs/fetch-google` | Erstellen Sie mithilfe der GCP-Autorisierung einen EAB. Körper: `{project_id, service_account_json?, access_token?}` |
| `POST /api/v1/eab-configs/fetch-zerossl` | Fordern Sie per E-Mail einen ZeroSSL EAB an. Körper: `{email}` |
| `GET  /api/v1/eab-configs` | EAB-Konfigurationen auflisten (**ohne** HMAC-Schlüssel oder Dienstkonto JSON; nur `has_mac_key` / `has_google_sa`) |
