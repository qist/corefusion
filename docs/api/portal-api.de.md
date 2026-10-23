<!-- source: portal-api.md | locale: de | updated: 2026-10-23 -->
# Portal API Referenz (Client / Portal API)

> Sprachen: [简体中文](portal-api.md) · [English](portal-api.en.md) · [繁體中文](portal-api.zh-TW.md) · [Русский](portal-api.ru.md) · [日本語](portal-api.ja.md) · [한국어](portal-api.ko.md) · Deutsch · [Français](portal-api.fr.md) · [Español](portal-api.es.md) · [Português](portal-api.pt.md) · [Türkçe](portal-api.tr.md) · [Bahasa Indonesia](portal-api.id.md) · [ไทย](portal-api.th.md) · [Tiếng Việt](portal-api.vi.md) · [العربية](portal-api.ar.md) · [فارسی](portal-api.fa.md)

> Die **vollständige** Schnittstellenreferenz für das Portal-Frontend (`/p` oder ein dedizierter Portal-Port) und Drittanbieter-Clients.
> Routendefinitionen: `internal/api/router/router.go` (`clientAuthGroup` / `clientAuthProtected` / `/api/v1/public/*`).

## 1. Allgemeine Konventionen

### 1.1 Basis URL

| Bereitstellung | Basis URL |
|---|---|
| Hauptpanel-Motor | `https://<panel-domain>/api/v1` (Portalseite unter `/p`) |
| Dedizierter Portal-Port | `https://<portal-domain>[:port]/api/v1` (Portalseite unter `/`) |
| Dedizierte Portaldomäne + Reverse-Proxy | Das Gleiche wie oben; Der Proxy muss beide Seitenpfade und `/api/v1` weiterleiten |

### 1.2 Authentifizierung

- Senden Sie nach der Anmeldung `Authorization: Bearer <access_token>`;
- Zugriffstokens halten 24 Stunden, Aktualisierungstokens 7 Tage;
- Das JWT enthält `tv` (token_version): Logout/erzwungenes Sign-out erhöht es und macht alle alten Token ungültig
 (`clients.token_version`, siehe `internal/service/clientauth/service.go`);
- Jeder geschützte Endpunkt durchläuft `ClientAuthMiddleware` (Versionsprüfung + Wartungsmodus-Schutz).

### 1.3 Antwortumschlag

Die meisten Endpunkte geben `{ "code": 0, "data": ..., "message": "..." }` zurück;
Einige Listenendpunkte geben `{ "items": [...], "total": N }` oder ein bloßes Array zurück (unten pro Endpunkt angegeben).
Fehler geben `{ "code": "ERROR_CODE", "message": "...", "request_id": "...", "can_retry": ... }` zurück
(HTTP 4xx/5xx).

### 1.4 Betragseinheiten

Bestell-/Saldofelder sind **ganzzahlige Cent**: `990` = ¥9,90 (`currency: "CNY"`).

### 1.5 Reales Beispiel (Anmelden → Profil abrufen)

```bash
# Register (when registration_enabled=true)
curl -X POST https://<portal-domain>/api/v1/client-auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","password":"plaintext-password","referral_code":""}'
# → 201 {"data":{"access_token":"<JWT>","refresh_token":"<JWT>","client":{...}}}

# Sign in
curl -X POST https://<portal-domain>/api/v1/client-auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","password":"plaintext-password"}'
# → 200 same as above

# Fetch the current client profile (with token)
curl https://<portal-domain>/api/v1/client-auth/me \
  -H 'Authorization: Bearer <access_token>'
# → 200 {"data":{"id":27,"email":"user@example.com","nickname":"","status":"active","quota_bytes":10737418240,"used_bytes":0,...}}

# Fetch a subscription (no header, token in the path)
curl 'https://<portal-domain>/sub/<subscription_token>?format=mihomo_yaml'
# → 200 Mihomo YAML (or base64_uri / singbox_json / xray_json)

# Error example: expired token
# → 401 {"code":"errClientTokenRevoked","message":"...","request_id":"...","can_retry":false}
```

Rufen Sie bei 401 einmal `POST /api/v1/client-auth/refresh` auf und versuchen Sie es erneut. Wenn es immer noch fehlschlägt, löschen Sie die lokale Sitzung (die Portalvorlage erledigt dies bereits).

---

## 2. Öffentliche Endpunkte (keine Anmeldung)

| Methode | Pfad | Beschreibung |
|---|---|---|
| GET | `/p` (`/` auf einer dedizierten Portal-Engine) | Portal-HTML (vom Server gerendert + Vue-Laufzeit) |
| GET | `/api/v1/public/portal-config` | Portal-Laufzeitkonfiguration: Site-Name, Designfarben, Navigation, Module, Sprachpakete, `registration_enabled`, `regions` (Quelle der Anmelderegion), `privacy_policy_url` / `terms_of_service_url` / `privacy_policy_content` / `terms_of_service_content` (Konformitätsdokumente; pro Sprache zugeordneter Inhalt; eigenständige Seiten `/p/legal/*` ✓) |
| GET | `/api/v1/public/settings` | Öffentliche Einstellungen (derzeit `registration_enabled`) |
| GET | `/api/v1/public/subscription-endpoints` | Liste der Abonnementendpunkte (Bare-Array) |
| GET | `/api/v1/public/articles` | Öffentliche Artikel (nur veröffentlicht) |
| GET | `/api/v1/public/articles/:slug` | Artikeldetail |
| GET | `/api/v1/public/client-configs` | Client-Download-Konfiguration |
| GET | `/sub/:token` | Abonnementlieferung (`format` gibt base64_uri / raw_uri / mihomo_yaml / singbox_json / xray_json usw. zurück; widerrufen/fehlend → 404, ausgesetzt/abgelaufen/inaktiv → 403. Siehe [subscription.md](portal-api.de.md)) |
| GET | `/api/v1/client-auth/oauth/providers` | Sichtbarkeit des OAuth-Anbieters: `{"data":{"providers":[{"provider":"github","enabled":true},...]}}` |
| GET | `/api/v1/system/maintenance` | Wartungsstatus (öffentlich). Das Portal zeigt bei Aktivierung eine Wartungsseite an |

> All dies beinhaltet den Wartungsschutz und die öffentliche API-Ratenbegrenzung.

## 3. Auth / OAuth

| Methode | Pfad | Auth | Beschreibung |
|---|---|---|---|
| POST | `/api/v1/client-auth/register` | — | Registrieren. Körper: `{email, password, referral_code?}`. Gibt `{access_token, refresh_token, client}` zurück; Anmelderate begrenzt |
| POST | `/api/v1/client-auth/login` | — | Anmelden. Text: `{email, password}`. Gleiche Antwort; gibt JWTs mit `token_version` aus |
| POST | `/api/v1/client-auth/refresh` | — | Aktualisieren. Körper: `{refresh_token}`. Gibt neue Zugriffs-/Aktualisierungs-Tokens zurück (Version überprüft; alte Aktualisierungs-Tokens sterben sofort nach der Abmeldung) |
| POST | `/api/v1/client-auth/logout` | ✓ | Abmelden: serverseitiger Widerruf (`token_version +1`) → alle alten Token für den Client sind ungültig |
| POST | `/api/v1/client-auth/rotate-token` | ✓ | Drehen Sie den Abonnement-Token |
| POST | `/api/v1/client-auth/ticket` | ✓ | Stellen Sie ein **einmaliges, kurzlebiges Ticket** aus (für `window.open`-Flows wie „Protokollpaket herunterladen“, bei dem keine Header festgelegt werden können) |
| GET | `/api/v1/client-auth/oauth/:provider` | — | OAuth starten: 307 → GitHub/Google Zustimmung (`state` Anti-CSRF, 10 Minuten); Behinderte Anbieter geben 400 zurück |
| GET | `/api/v1/client-auth/oauth/:provider/callback` | — | OAuth-Rückruf: Code austauschen → Client erstellen/verknüpfen → Versionsnummer JWT ausgeben → 302 zurück zum Portal (`/<portalPath>?access_token=...&refresh_token=...`) |

> Der OAuth-Rückruf ist eine **Browser-Umleitung** (kein API-Aufruf): Nach einer erfolgreichen Anmeldung liest das Portal das Token
> aus URL in `onMounted` (oder über `postMessage` mit demselben Ursprung, siehe `handleOAuthCallback` in der Portalvorlage `web`).

## 4. Konto

| Methode | Pfad | Beschreibung |
|---|---|---|
| GET | `/api/v1/client-auth/me` | Aktueller Kunde: `{id, email, plan_id, plan_name, status, quota_bytes, used_bytes, upload_bytes, download_bytes, expires_at, ip_whitelist, device_limit, sub_token, sub_status, created_at, ...}` (`data`-Umschlag) |
| POST | `/api/v1/client-auth/change-password` | Körper: `{old_password, new_password}` |
| GET | `/api/v1/client-auth/checktime` | Serverzeit (Client-Uhrsynchronisierung) |
| GET | `/api/v1/client-auth/connection-info` | Aktuelle Planverbindungsparameter: `{concurrent, device_limit, protocol, speed_limit, tls, transport}` |

## 5. Abonnements

| Methode | Pfad | Beschreibung |
|---|---|---|
| POST | `/api/v1/client-auth/reset-subscription` | Setzen Sie das Abonnement zurück. Dies dreht `sub_token` |

> Der Abonnement-Link basiert auf dem Frontend von `sub_token`, das von `/me` (`/sub/:token`) zurückgegeben wird. es gibt keine
> separate `subscription-link` / `subscription-formats`-Endpunkte mehr.

## 6. Knoten

| Methode | Pfad | Beschreibung |
|---|---|---|
| GET | `/api/v1/client-auth/nodes` | Für den aktuellen Plan sichtbare Knoten: `{items:[...]}` (Anzeigefelder wie Name/Protokoll/Transport/Region) |
| GET | `/api/v1/client-auth/nodes/:id/config` | Vollständige Knotenkonfiguration |
| GET | `/api/v1/client-auth/nodes/:id/link` | Link zum Teilen (vless:// usw.) |
| GET | `/api/v1/client-auth/nodes/:id/qrcode` | QR-Code-Bild |

## 7. Shop / Pläne / Bestellungen

| Methode | Pfad | Beschreibung |
|---|---|---|
| GET | `/api/v1/client-auth/plans` | Planliste (Bare-Array): `{id, name, description, category, sort_order, variants:[{id, name, billing_cycle, duration_days, price(cents), original_price, currency, quota_bytes, ...}]}` |
| GET | `/api/v1/client-auth/plans/:id` | Plandetails |
| GET | `/api/v1/client-auth/traffic-packs` | Traffic-Packliste |
| POST | `/api/v1/client-auth/traffic-packs/buy` | Kaufen Sie ein Verkehrspaket |
| GET | `/api/v1/client-auth/orders` | Meine Planbestellungen (paginiert) |
| POST | `/api/v1/client-auth/orders` | Eine Bestellung erstellen (eine Planvariante kaufen/erneuern) |
| GET | `/api/v1/client-auth/orders/:id` | Bestelldetails |
| GET | `/api/v1/client-auth/invoices` | Rechnungen |
| GET | `/api/v1/client-auth/invoices/:id` | Rechnungsdetails |

## 8. Ausgleichen / Aufladen

| Methode | Pfad | Beschreibung |
|---|---|---|
| GET | `/api/v1/client-auth/balance` | `{data:{client_id, balance(cents), frozen}}` |
| GET | `/api/v1/client-auth/balance/transactions` | Saldotransaktionen (`type`: Belastung/Verbrauch/Rückerstattung; paginiert) |
| GET | `/api/v1/client-auth/charge/sites` | Verfügbare Aufladestandorte (`ListChargeSites`: nur aktiviert) |
| POST | `/api/v1/client-auth/charge/create` | Einen Aufladeauftrag erstellen (Weiterleitung zur Drittanbieterzahlung) |
| GET | `/api/v1/client-auth/charge/orders` | Bestellliste zum Aufladen |
| GET | `/api/v1/client-auth/charge/orders/:id` | Details zur Aufladebestellung |
| GET | `/api/v1/client-auth/charge/status/:trade_no` | Zahlungsstatus nach Handelsnr. abfragen |
| POST | `/api/v1/client-auth/charge/redeem-card` | Lösen Sie eine Aufladekarte ein |

> Das Löschen eines Kunden führt zu **Kaskaden** auf dessen Kontostand und Transaktionen (siehe `DeleteClient` in `internal/service/client/service.go`).

## 9. Geschenke / Verkehrsgeschichte

| Methode | Pfad | Beschreibung |
|---|---|---|
| GET | `/api/v1/client-auth/gifts` | Anspruchsberechtigte Geschenke (nacktes Array; `type`: Tage/Bytes) |
| POST | `/api/v1/client-auth/gifts/:id/claim` | Anspruch |
| GET | `/api/v1/client-auth/traffic-history` | Monatlicher Verkehrsverlauf: `{data:{history:[{period_start,total_bytes,...}], months}}` |

## 10. Tickets

| Methode | Pfad | Beschreibung |
|---|---|---|
| GET | `/api/v1/client-auth/tickets` | Ticketliste: `{items,total}` |
| POST | `/api/v1/client-auth/tickets` | Erstellen Sie ein Ticket |
| GET | `/api/v1/client-auth/tickets/:id` | Ticketdetails (mit Antworten) |
| POST | `/api/v1/client-auth/tickets/:id/reply` | Antwort |
| POST | `/api/v1/client-auth/tickets/:id/close` | Schließen |

## 11. Artikel / Leitfäden

| Methode | Pfad | Beschreibung |
|---|---|---|
| GET | `/api/v1/client-auth/articles` | Artikelliste (mit Kategoriefilter) |
| GET | `/api/v1/client-auth/articles/:slug` | Artikeldetail |

## 12. Empfehlungen (standardmäßig deaktiviert; kann im Portal-Editor aktiviert werden)

| Methode | Pfad | Beschreibung |
|---|---|---|
| GET | `/api/v1/client-auth/referral/code` | Mein Empfehlungscode und Link |
| GET | `/api/v1/client-auth/referral/stats` | Provisionsstatistiken (`commission_rate`, `available_commission`, `min_withdraw_amount`) |
| GET | `/api/v1/client-auth/referral/records` | Überweisungsaufzeichnungen |
| GET | `/api/v1/client-auth/referral/withdrawals` | Auszahlungsunterlagen |
| POST | `/api/v1/client-auth/referral/withdraw` | Beantragen Sie eine Auszahlung |

## 13. Geteilte Konten

| Methode | Pfad | Beschreibung |
|---|---|---|
| GET | `/api/v1/client-auth/shared-accounts` | Verfügbare gemeinsame Konten (Streaming usw.) |
| GET | `/api/v1/client-auth/shared-accounts/:id` | Detailliert |
| POST | `/api/v1/client-auth/shared-accounts/:id/apply` | Zugriff anfordern |
| POST | `/api/v1/client-auth/shared-accounts/:id/repair` | Reparatur mit einem Klick |

## 14. Protokollpakete (Abonnentendiagnose)

| Methode | Pfad | Beschreibung |
|---|---|---|
| GET | `/api/v1/client-auth/logs/types` | Protokolltypen, die gebündelt werden können |
| GET | `/api/v1/client-auth/logs/packages` | Meine Protokollpakete |
| POST | `/api/v1/client-auth/logs/package` | Erstellen Sie einen Protokollbündelungsauftrag (nur Eingangsknoten). |
| GET | `/api/v1/client-auth/logs/packages/:id` | Auftragsstatus (bereit/in Bearbeitung/fehlgeschlagen) |
| GET | `/api/v1/client-auth/logs/packages/:id/download` | Herunterladen (bevorzugen Sie `POST /ticket` und fügen Sie `?ticket=` hinzu) |
| DELETE | `/api/v1/client-auth/logs/packages/:id` | Löschen Sie ein Protokollpaket |

## 15. Sicherheitseinstellungen / IP-Zulassungsliste / Selbstentsperrung

| Methode | Pfad | Beschreibung |
|---|---|---|
| GET/PUT | `/api/v1/client-auth/settings` | Client-Einstellungen: `{ip_whitelist, device_limit}` (PUT) |
| GET | `/api/v1/client-auth/settings/whitelist` | Zulassungsliste (nacktes Array) |
| DELETE | `/api/v1/client-auth/settings/whitelist/:id` | Löschen Sie einen Zulassungslisteneintrag |
| POST | `/api/v1/client-auth/settings/auto-add-ip` | Fügen Sie die aktuelle IP zur Zulassungsliste hinzu |
| POST | `/api/v1/client-auth/self-unban` | Selbstbedienungssperre aufheben (Versuche durch Richtlinien begrenzt) |

> Die IP-Zulassungsliste gilt **nur für Abonnement-URLs** (`/sub/:token`); Es blockiert nicht die Portalanmeldung/OAuth oder
> andere Client-Authentifizierungsendpunkte. Wenn der Benutzer keine Zulassungsliste oder kein Abonnement URL hat, ist der Zugriff immer erlaubt. Die
> Die Zulassungsliste ist eine benutzerspezifische Einstellung. Es gibt keinen globalen Schalter.

**Selbst-Unban-Fluss**: nur für `auto_quota` verfügbar (automatisches Verbot bei Verkehrsüberschreitung), wenn die Admin-Seite
`self_unban.enabled` ist eingeschaltet. Die Anmeldung (Passwort oder OAuth-Rückruf) gibt `require_self_unban=true` + `self_unban_token` zurück
(typ=self_unban, gültig 15 Minuten, **nur** verwendbar auf `POST /self-unban`; andere Endpunkte geben 403/`SELF_UNBAN_TOKEN_SCOPE` zurück).
Das Portal zeigt dann die Entbannungskarte an; Nach einer erfolgreichen Aufhebung der Sperre meldet sich der Benutzer erneut an. OAuth-Rückrufe liefern dies über
das URL-Fragment `#require_self_unban=1&self_unban_token=..&self_unban_hint=..`.

## 16. Fehlercodes (Auszug)

| Code | Szenario |
|---|---|
| `INVALID_TOKEN` | Aktualisierungstoken ungültig/abgelaufen/versionskonflikt |
| `INVALID_REQUEST` | Die Parametervalidierung ist fehlgeschlagen |
| `NOT_FOUND` | Fehlende Ressource (Ticket/Bestellung/Zulassungslisteneintrag usw.) |
| `RELOAD_FAILED` / `SYNC_FAILED` | (Administratorseite) Die Synchronisierung der Proxy-Konfiguration ist fehlgeschlagen |
| 401 + `errClientTokenRevoked` | token_version stimmt nicht überein (abgemeldet/gestrichen) → Clients sollten aktualisiert werden; Wenn dies fehlschlägt, löschen Sie die lokalen Token |

## 17. Compliance-Dokumente (Datenschutzerklärung / Nutzungsbedingungen)

Die Links „Datenschutzerklärung · Nutzungsbedingungen“ unten auf der Anmeldekarte (wie bei normalen Websites; durch Klicken wird der Inhalt geöffnet):

- **Konfiguration**: Portaleditor → Einstellungen → Allgemein → Compliance-Dokumente (**Inhalt pro Sprache bearbeiten**);
 Sie können `PUT /api/v1/portal/active` auch mit strukturierten Feldern `privacy_policy_url` / `privacy_policy_content` /
 `terms_of_service_url` / `terms_of_service_content` (null = unverändert; Inhalt ist eine `{"zh-CN":"...","en":"..."}`-Karte).
- **Eigenständig zugängliche Seiten** (vom Server gerendert, einfach für Google-Verifizierungscrawler; unterstützt `?lang=xx`):
  - Hauptpanel-Domäne: `GET /p/legal/privacy`, `GET /p/legal/terms`
  - Dedizierte Portaldomäne (Root-Bereitstellung): `GET /legal/privacy`, `GET /legal/terms`
- **Anzeigelogik**: URL oder Inhalt, **URL gewinnt** (öffnet sich extern in einem neuen Tab); Wenn Inhalte bereitgestellt werden, klicken Sie auf
 Die Anmeldekarte rendert Markdown in einem Dialog (`renderMd`) unter Verwendung der aktuellen Portalsprache.
- Wird zur Laufzeit über `/api/v1/public/portal-config` bereitgestellt; Die Anmeldekomponente verbraucht
 `ctx.privacyUrl / termsUrl / privacyContent / termsContent / openLegal / hasLegalLinks`.
- Für die Google-OAuth-Überprüfung müssen diese Links öffentlich zugänglich sein → verwenden Sie die oben genannten eigenständigen Links im
 Einwilligungsbildschirm; siehe `docs/oauth-google-verification.md` (enthält Vorlagen für Datenschutzrichtlinien/Bedingungen).

## 18. Integrationsbeispiel

```js
// Sign in
const { data } = await fetch('/api/v1/client-auth/login', {
  method: 'POST', headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ email, password })
}).then(r => r.json());
localStorage.setItem('client_token', data.access_token);
localStorage.setItem('client_refresh_token', data.refresh_token);

// Authenticated request
const me = await fetch('/api/v1/client-auth/me', {
  headers: { Authorization: 'Bearer ' + data.access_token }
}).then(r => r.json());
```

Rufen Sie bei 401 einmal `POST /refresh` auf und versuchen Sie es erneut. Wenn es immer noch fehlschlägt, löschen Sie die lokale Sitzung
(Das `api()` der Portalvorlage implementiert dies bereits).
