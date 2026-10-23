<!-- source: oauth-google-verification.md | locale: de | updated: 2026-10-23 -->
# Google OAuth-Verifizierungshandbuch

> Sprachen: [简体中文](oauth-google-verification.md) · [English](oauth-google-verification.en.md) · [繁體中文](oauth-google-verification.zh-TW.md) · [Русский](oauth-google-verification.ru.md) · [日本語](oauth-google-verification.ja.md) · [한국어](oauth-google-verification.ko.md) · Deutsch · [Français](oauth-google-verification.fr.md) · [Español](oauth-google-verification.es.md) · [Português](oauth-google-verification.pt.md) · [Türkçe](oauth-google-verification.tr.md) · [Bahasa Indonesia](oauth-google-verification.id.md) · [ไทย](oauth-google-verification.th.md) · [Tiếng Việt](oauth-google-verification.vi.md) · [العربية](oauth-google-verification.ar.md) · [فارسی](oauth-google-verification.fa.md)

> Geltungsbereich: Google-Anmeldung für das CoreFusion-Portal (`internal/service/oauth`, Geltungsbereiche: `openid`, `email`, `profile`).
> GitHub-Vergleich am Ende (GitHub hat keinen Überprüfungsablauf).

## 1. Test- vs. Produktionsmodus

Der OAuth-Zustimmungsbildschirm Google hat zwei Veröffentlichungsstatus:

| Staat | Wer kann sich anmelden? | Verifizierung erforderlich |
|---|---|---|
| **Testen** | Nur Google-Konten in der Liste „Testbenutzer“. | Nein |
| **In Produktion** | Jedes Google-Konto | **Ja** (siehe unten) |

Sobald „Systemeinstellungen → OAuth“ aktiviert ist, wird auf der Portal-Anmeldeseite jedem Besucher die Schaltfläche Google angezeigt.
Wenn sich die App noch im Test befindet, sehen Besucher, die nicht auf der Testbenutzerliste stehen
`Error 403: access_denied` (App nicht verifiziert / Sie sind kein Testbenutzer).
Um die Google-Anmeldung öffentlich anzubieten, **schalten Sie die App auf Produktion um und bestehen Sie die Überprüfung**.

## 2. Harte Anforderungen an die Verifizierung

Die Überprüfung von Google (Basisverifizierung für nicht eingeschränkte Bereiche) erfordert:

1. **Homepage**: ein öffentlich zugängliches URL auf der autorisierten Domain (gleiche Domain oder eine Subdomain).
2. **Link zur Datenschutzerklärung**: muss **öffentlich zugänglich** sein (keine Anmeldung) und erklären, welche Daten Sie sammeln und wie Sie diese verwenden/speichern/löschen. Für die Google-Anmeldung muss mindestens Folgendes abgedeckt sein:
 die erfassten Google-Kontodaten (E-Mail, Name, Avatar), der Zweck (Erstellen/Anmelden beim Portalkonto), kein Verkauf an Dritte,
 und wie Benutzer ihr Konto und ihre Daten löschen können.
3. **Link zu den Nutzungsbedingungen**: öffentlich zugänglich, Beschreibung der Serviceregeln.
4. **Autorisierte Domänen**: Registrieren Sie die Domäne der App im Einwilligungsbildschirm (z. B. `example.com`); home/privacy/terms-Links müssen darauf oder in einer Subdomain vorhanden sein.
5. **App-Logo (optional, aber empfohlen)**: 120×120px.
6. **Begründung des Umfangs**: Dieses Projekt verwendet nur `openid / email / profile` (nicht eingeschränkt, nicht sensibel).
 Dies fällt unter die Basisverifizierung – keine Begründung pro Bereich und **keine Sicherheitsbewertung durch Dritte**.
 (Eingeschränkte Bereiche wie der vollständige Gmail-Zugriff würden sie erfordern; dieses Projekt verwendet keine.)

Verifizierungseintrag: [Google Cloud Console](https://console.cloud.google.com/) →
**APIs und Dienste → OAuth-Zustimmungsbildschirm → App veröffentlichen → Zur Überprüfung einreichen**.
Die Basisverifizierung ist in der Regel innerhalb weniger Tage abgeschlossen. Sie können in der Zwischenzeit im Testmodus arbeiten.

## 3. Was für dieses Projekt konfiguriert werden soll

### 3.1 Autorisierte Weiterleitung URI

Der Rückruf wird dynamisch von der Domäne generiert, die die Anmeldung startet (siehe `Authorize` in `internal/api/handlers/oauth/handler.go`):

```
https://<portal-domain>/api/v1/client-auth/oauth/google/callback
```

- Die Anmeldung wurde über die Portaldomäne (z. B. `https://p.example.com`) gestartet → `https://p.example.com/...` registrieren;
- Gestartet mit der Panel-Domäne `/p` → Registrieren Sie `https://panel.example.com/...`.
- Google erlaubt **mehrere** Umleitungs-URIs; Registrieren Sie beide Domains.
- ⚠ Für Google müssen Umleitungs-URIs **https** sein (außer `http://localhost`);
 Der Reverse-Proxy muss `X-Forwarded-Host` / `X-Forwarded-Proto` weiterleiten, sonst baut das Panel eine auf
 http oder interner Redirect_uri → Google lehnt es ab (`redirect_uri_mismatch`).

Nach der Einstellung „Systemeinstellungen → OAuth → Portal-Callback-Domain“ wird der auf der Seite angezeigte Callback URL in Google eingefügt.

### 3.2 Datenschutzrichtlinien/AGB-Links auf dem Portal

Die Anmeldekarte enthält Links zu „Datenschutzerklärung · Nutzungsbedingungen“ (die nach der Konfiguration angezeigt werden) und beide Dokumente verfügen über **eigenständige Seiten**
(Der Dialog und die eigenständige Seite haben denselben Inhalt):

- Hauptpanel-Domäne: `https://<panel-domain>/p/legal/privacy`, `/p/legal/terms`
- Dedizierte Portaldomäne (Root-Bereitstellung): `https://<portal-domain>/legal/privacy`, `/legal/terms`
- Unterstützt `?lang=xx`; Ohne diese wird der `Accept-Language` des Browsers verwendet, dann der Portal-Standard.
- Seiten werden **vom Server gerendert** (kein JS), was dem Verifizierungs-Crawler von Google hilft.

**Konfiguration (so oder so; Links gewinnen)**:

1. **Inhaltsmodus (empfohlen, funktioniert sofort)**: Portaleditor → Einstellungen → Allgemein → Compliance-Dokumente →
 Wählen Sie „Sprache bearbeiten“ und fügen Sie die §4-Vorlagen in „Inhalt der Datenschutzrichtlinie“ / „Inhalt der Nutzungsbedingungen“ ein.
 (**Markdown, je Sprache bearbeitet**) → Nach dem Speichern zeigt die Anmeldekarte Links an, die einen **Dialog** öffnen;
 Der „Standalone-Link“ des Redakteurs kann direkt im Google-Zustimmungsbildschirm eingegeben werden.
2. **Externer Linkmodus**: Geben Sie „Datenschutzrichtlinie URL“ / „Nutzungsbedingungen URL“ ein (zeigen Sie auf die von Ihnen gehosteten Seiten) → wird in einem neuen Tab geöffnet.

Inhalte und Links werden über `/api/v1/public/portal-config` (`privacy_policy_content` /
`terms_of_service_content` sind **Markdown-Maps pro Sprache** `{"zh-CN":"...","en":"..."}`;
`privacy_policy_url` / `terms_of_service_url` sind optionale externe Links). **Sie müssen ohne Anmeldung zugänglich sein** ✓.

### 3.3 Schalttafelseitiger Schalter

`System settings → OAuth`: Aktivieren Sie Google und geben Sie die Client-ID/das Client-Geheimnis ein (das Geheimnis wird im Ruhezustand mit AES-256-GCM verschlüsselt).
Die Schaltfläche Google wird auf der Portal-Anmeldeseite nur angezeigt, wenn alles konfiguriert ist (`/api/v1/client-auth/oauth/providers` wird aktiviert zurückgegeben).

## 4. Vorlagen für Datenschutzrichtlinien/Nutzungsbedingungen

> Minimale Compliance-Vorlagen, die Sie anpassen können. Vor der Verwendung: Ersetzen Sie die Platzhalter `【】`, fügen Sie Klauseln für Ihr Unternehmen hinzu bzw. entfernen Sie sie.
> und lassen Sie sie von der Betreibergesellschaft überprüfen. Diese Vorlagen stellen keine Rechtsberatung dar.

### 4.1 Datenschutzrichtlinie (Vorlage)

```markdown
# Privacy Policy

Last updated: [DATE]

[OPERATOR NAME] ("we") operates [PORTAL NAME] ("the Service"). We respect your privacy;
this policy explains what information we collect and how we use it.

## 1. Information we collect
- **Account information**: the email address and password you provide at registration (stored only as a one-way hash);
- **Google sign-in information**: when you choose "Sign in with Google", we receive, with your authorization,
  your Google account email, display name and avatar URL (scopes: openid, email, profile);
- **Service data**: subscription configuration, traffic usage, device IP allow-list, tickets and order records.

## 2. How we use information
Only to: create and sign in to your account, provide the proxy subscription service, bill you and provide support,
and for security controls (sign-in protection, IP allow-list). We **do not** sell your personal information.

## 3. Third-party services
- **Google**: receives account information within the authorized scope only when you actively use "Sign in with Google";
- Payment providers: receive order information only when you initiate a payment.

## 4. Storage and security
Data is stored on our servers; credentials are encrypted at rest and all traffic uses TLS.

## 5. Your rights
You can update your profile/password and end sessions in the portal at any time; to delete your account and all data,
contact [CONTACT EMAIL] and we will process the request within [N] business days.

## 6. Policy changes
Material changes will be announced in the portal notice board.
```

### 4.2 Nutzungsbedingungen (Vorlage)

```markdown
# Terms of Service

Last updated: [DATE]

## 1. Service
The Service provides [PROXY / NETWORK ACCESS] to registered users; specific plans are described on the store page.

## 2. Accounts
- You are responsible for the security of your account and password; for accounts created with Google sign-in, credentials are managed by Google;
- You may not lend or transfer your account, or use the Service for any unlawful purpose (including but not limited to:
  unauthorized access to others' systems, distributing illegal content, or circumventing mandatory legal requirements).

## 3. Payment and refunds
Plans are billed [monthly / by usage]; traffic and device limits follow the plan description;
[REFUND POLICY: e.g. full refund within 7 days if unused].

## 4. Changes and termination
We may suspend or terminate the Service due to force majeure or your breach of these terms; deleting an account deletes the related data.

## 5. Disclaimer
The Service is provided "as is"; to the extent permitted by law, we are not liable for indirect losses caused by use of the Service.

## 6. Miscellaneous
These terms are governed by the laws of [JURISDICTION]. Contact: [CONTACT EMAIL].
```

## 5. Checkliste vor dem Start

- [ ] OAuth-Zustimmungsbildschirm: App-Name / Logo / Startseite / Datenschutzrichtlinie / Bedingungen alle vorhanden; Domain registriert
- [ ] Seiten mit Datenschutzrichtlinien und Nutzungsbedingungen **zugänglich ohne Anmeldung** (benutzerdefinierte Portalseite oder eigenständige statische Seite)
- [ ] Beide Links sind auf der Anmeldekarte/Fußzeile des Portals sichtbar
- [ ] Weiterleitung URI entspricht der tatsächlichen Domäne und ist https
- [ ] App veröffentlichen → zur Überprüfung einreichen; Die Testbenutzerbeschränkung wird nach der Genehmigung aufgehoben
- [ ] Panel-OAuth-Einstellungen: aktiviert=wahr mit ausgefüllter Client-ID/Geheimnis (Endpunkt des Anbieters gibt „aktiviert:wahr“ zurück)

## 6. GitHub-Vergleich

GitHub OAuth-Apps haben **keinen** Veröffentlichungsstatus oder Überprüfungsablauf: Erstellen Sie die App, legen Sie den Rückruf URL fest, und sie funktioniert öffentlich.
Beachten Sie, dass eine GitHub OAuth-App nur **einen** Autorisierungsrückruf URL (Präfixübereinstimmung) akzeptiert; für mehrere Domänen,
Nutzen Sie den Multi-URI-Ansatz von Google oder erstellen Sie eine separate App pro Portaldomäne.
