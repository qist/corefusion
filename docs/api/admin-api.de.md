<!-- source: admin-api.md | locale: de | updated: 2026-10-23 -->
# Admin API

> Sprachen: [简体中文](admin-api.md) · [English](admin-api.en.md) · [繁體中文](admin-api.zh-TW.md) · [Русский](admin-api.ru.md) · [日本語](admin-api.ja.md) · [한국어](admin-api.ko.md) · Deutsch · [Français](admin-api.fr.md) · [Español](admin-api.es.md) · [Português](admin-api.pt.md) · [Türkçe](admin-api.tr.md) · [Bahasa Indonesia](admin-api.id.md) · [ไทย](admin-api.th.md) · [Tiếng Việt](admin-api.vi.md) · [العربية](admin-api.ar.md) · [فارسی](admin-api.fa.md)

Die vollständige Admin-API-Routentabelle für **Panel-Entwicklung/Automatisierung/Integratoren**.

- Basis URL: `https://<panel-domain>/api/v1`
- Authentifizierung: `Authorization: Bearer <admin_token>` (von `POST /api/v1/auth/login`)
- Gängige Antwort-/Fehlerformate und Ratenbegrenzungskonventionen: siehe [README.md](./README.de.md)
- Quelle der Wahrheit: `internal/api/router/router.go` (739 Admin-Routen); Wenn diese Tabelle nicht übereinstimmt, gewinnt **der Code**

## Berechtigungsmarkierungen

| Markierung | Bedeutung |
|---|---|
| Öffentlich | Keine Anmeldung erforderlich (Wartungsschutz/Ratenbegrenzung kann weiterhin gelten) |
| Öffentlich · gebührenbegrenzt | Keine Anmeldung, mit einem dedizierten Ratenlimit |
| Angemeldet | Erfordert ein angemeldetes Administrator-Token (`AuthRequired`) |
| Admin | Erfordert Rolle ≥ `admin` (`RequireRole(RoleAdmin)`) |
| Super Admin | Erfordert Rolle = `super_admin` |

Rollenebenen: `viewer(10) < operator(20) < admin(30) < super_admin(40)`; Unzureichende Rollen geben 403 `INSUFFICIENT_ROLE` zurück.

---

## Schnellstart (echte Beispiele)

Diese Beispiele stammen aus realen lokalen Reaktionen; Token und Daten werden geschwärzt.

### 1. Melden Sie sich an und erhalten Sie ein Token

```bash
curl -X POST https://<panel-domain>/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"your-password"}'
```

Antwort:

```json
{
  "data": {
    "access_token": "<JWT, valid 24h>",
    "refresh_token": "<JWT, valid 7 days>",
    "user": { "id": 1, "role": "super_admin", "username": "admin" }
  }
}
```

### 2. Rufen Sie mit dem Token einen Geschäftsendpunkt auf

```bash
curl https://<panel-domain>/api/v1/clients?limit=1 \
  -H 'Authorization: Bearer <access_token>'
```

Antwort (Listen verwenden die Struktur `items` + `total`):

```json
{
  "data": {
    "items": [
      {
        "id": 27,
        "email": "user@example.com",
        "status": "active",
        "quota_bytes": 10737418240,
        "used_bytes": 0,
        "plan_id": 1,
        "expires_at": null,
        "created_at": "2026-09-30T15:54:32+08:00"
      }
    ],
    "total": 22,
    "limit": 1,
    "offset": 0
  }
}
```

### 3. Beispiele für Fehlerreaktionen

**401 nicht authentifiziert:**

```json
{
  "code": "MISSING_TOKEN",
  "message": "missing authorization header",
  "request_id": "a02cfdf9-6be0-46be-8584-824caab34dc7",
  "can_retry": false,
  "requires_action": false
}
```

**404-Ressource nicht gefunden:**

```json
{
  "code": "CLIENT_NOT_FOUND",
  "message": "client not found",
  "request_id": "099949bd-4ded-468d-a6f8-f8e798af2273",
  "can_retry": false,
  "requires_action": false
}
```

### 4. Paginierung

Alle Listenendpunkte verwenden `limit` (Seitengröße, Standard 20) und `offset` (0-basiert) und geben `{items, total, limit, offset}` zurück. Einige Legacy-Endpunkte geben ein bloßes Array zurück (in der Tabelle angegeben).

---

## Inhalt

- [Administratorauthentifizierung und Sicherheit](#administratorauthentifizierung-und-sicherheit)
- [Admin-Konten](#admin-konten)
- [Kundenmanagement](#kundenmanagement)
- [Planvorlagen](#planvorlagen)
- [Planvarianten](#planvarianten)
- [Verkehrspakete](#verkehrspakete)
- [Gutscheine](#gutscheine)
- [Knotengruppen](#knotengruppen)
- [Knotenserver](#knotenserver)
- [Knotenüberwachungsprotokoll](#knotenüberwachungsprotokoll)
- [Knoten (Legacy-Modell, im Ruhestand)](#knoten-legacy-modell-im-ruhestand)
- [Regionsübersicht](#regionsübersicht)
- [Abonnements](#abonnements)
- [Abonnementformate](#abonnementformate)
- [Abonnementendpunkte](#abonnementendpunkte)
- [Domains teilen](#domains-teilen)
- [Cloudflare IP](#cloudflare-ip)
- [Abonnementkonfiguration (Mihomo/Clash)](#abonnementkonfiguration-mihomoclash)
- [Aufladeseiten](#aufladeseiten)
- [Bestellungen aufladen](#bestellungen-aufladen)
- [Karten aufladen](#karten-aufladen)
- [Gebührenrückrufereignisse](#gebührenrückrufereignisse)
- [Wiederholungsversuche für kostenpflichtige Rückrufe](#wiederholungsversuche-für-kostenpflichtige-rückrufe)
- [Kundensaldo](#kundensaldo)
- [Geschenke](#geschenke)
- [Empfehlungskonfiguration](#empfehlungskonfiguration)
- [Auszahlungen aufgrund von Empfehlungen](#auszahlungen-aufgrund-von-empfehlungen)
- [IP-Zulassungslisten (Zusammenfassung)](#ip-zulassungslisten-zusammenfassung)
- [Kunden-OAuth-Bindungen](#kunden-oauth-bindungen)
- [Tickets (admin)](#tickets-admin)
- [Artikel](#artikel)
- [Geteilte Konten](#geteilte-konten)
- [Zertifikate](#zertifikate)
- [Zertifikatskanäle](#zertifikatskanäle)
- [EAB-Konfiguration](#eab-konfiguration)
- [ACME-Konten](#acme-konten)
- [Integrierte CA](#integrierte-ca)
- [Geo-Regeln](#geo-regeln)
- [WAF Regelbibliothek](#waf-regelbibliothek)
- [Protokollverwaltung](#protokollverwaltung)
- [Backups](#backups)
- [Systemeinstellungen](#systemeinstellungen)
- [Sicherheitsaudit](#sicherheitsaudit)
- [Metriken](#metriken)
- [Portalvorlagen](#portalvorlagen)
- [Agenteninstallation](#agenteninstallation)
- [Kostenpflichtiger Rückrufeintrag](#kostenpflichtiger-rückrufeintrag)
- [Eingehend (mit Knoten)](#eingehend-mit-knoten)
- [Öffentliche Konfiguration](#öffentliche-konfiguration)
- [Client-Authentifizierung und Geschäft (gleiches Set auf der Admin-Engine)](#client-authentifizierung-und-geschäft-gleiches-set-auf-der-admin-engine)
- [Abonnementlieferung](#abonnementlieferung)
- [Workflow für die Laufzeitkonfiguration](#workflow-für-die-laufzeitkonfiguration)
- [Dienstzugriffsrichtlinien (eingestellt)](#dienstzugriffsrichtlinien-eingestellt)
- [Überwachung der Verkehrsquote](#überwachung-der-verkehrsquote)
- [Gesundheitschecks](#gesundheitschecks)
- [Portalseite](#portalseite)
- [ACME Herausforderungen](#acme-herausforderungen)

---

### Administratorauthentifizierung und Sicherheit

Anmeldung, Abmeldung, Passwortänderung, 2FA (TOTP), Passkey (WebAuthn) und einmalige Tickets für das Admin-Panel.

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| POST | `/api/v1/auth/2fa/disable` | Angemeldet | Deaktivieren |
| POST | `/api/v1/auth/2fa/enable` | Angemeldet | Aktivieren |
| POST | `/api/v1/auth/2fa/recovery-codes` | Angemeldet | Wiederherstellungscodes neu generieren |
| POST | `/api/v1/auth/2fa/setup` | Angemeldet | Einstellungen |
| GET | `/api/v1/auth/2fa/status` | Angemeldet | Status |
| POST | `/api/v1/auth/change-password` | Angemeldet | Passwort ändern |
| POST | `/api/v1/auth/login` | Öffentlich | Melden Sie sich an |
| POST | `/api/v1/auth/login/2fa` | Öffentlich | Anmeldung mit dynamischem Code (2FA) |
| GET | `/api/v1/auth/login/2fa-status` | Öffentlich | Überprüfen Sie, ob für das Konto 2FA aktiviert ist |
| POST | `/api/v1/auth/logout` | Angemeldet | Abmelden |
| GET | `/api/v1/auth/me` | Angemeldet | Aktuell angemeldeter Benutzer |
| GET | `/api/v1/auth/passkey/credentials` | Angemeldet | Passkey-Anmeldeinformationsliste |
| DELETE | `/api/v1/auth/passkey/credentials/:id` | Angemeldet | Löschen Sie die Passkey-Anmeldeinformationen |
| POST | `/api/v1/auth/passkey/login/begin` | Öffentlich | Beginnen Sie mit der Anmeldung |
| POST | `/api/v1/auth/passkey/login/finish` | Öffentlich | Vollständige Anmeldung |
| POST | `/api/v1/auth/passkey/register/begin` | Angemeldet | Beginnen Sie mit der Registrierung |
| POST | `/api/v1/auth/passkey/register/finish` | Angemeldet | Vollständige Registrierung |
| GET | `/api/v1/auth/passkey/rpids` | Angemeldet | Zulassungsliste für RP-IDs |
| POST | `/api/v1/auth/passkey/rpids` | Angemeldet | Aktualisieren Sie die RP-ID-Zulassungsliste |
| GET | `/api/v1/auth/passkey/status` | Öffentlich | Status |
| POST | `/api/v1/auth/refresh` | Öffentlich | Aktualisieren |
| POST | `/api/v1/auth/ticket` | Angemeldet | Einmaliges, kurzlebiges Ticket ausstellen |

### Admin-Konten

Benutzerkonten des Admin-Panels. Für die Auflistung ist ein Administrator erforderlich. Zum Erstellen/Aktualisieren/Löschen ist ein Superadministrator erforderlich (verhindert eine Rechteausweitung).

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| GET | `/api/v1/users` | Admin | Liste |
| POST | `/api/v1/users` | Super Admin | Erstellen |
| DELETE | `/api/v1/users/:id` | Super Admin | Löschen |
| PUT | `/api/v1/users/:id` | Super Admin | Aktualisieren |

### Kundenmanagement

CRUD, Suspend/Resume, Upgrade/Downgrade, Erneuerung und Nutzungsrücksetzung für Abonnenten (Portalkonten).

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| GET | `/api/v1/clients` | Angemeldet | Kunden auflisten |
| POST | `/api/v1/clients` | Angemeldet | Kunde anlegen. Körper: `{"email":"user@example.com","password":"plaintext-password","plan_id":1,"quota_bytes":10737418240,"expires_at":"2026-12-31T00:00:00Z","remark":""}`. `plan_id` ist erforderlich; `password` kann leer sein (Kunden können es später im Portal festlegen) |
| DELETE | `/api/v1/clients/:id` | Angemeldet | Kunde löschen |
| GET | `/api/v1/clients/:id` | Angemeldet | Kunden gewinnen |
| PUT | `/api/v1/clients/:id` | Angemeldet | Kunde aktualisieren |
| POST | `/api/v1/clients/:id/downgrade` | Angemeldet | Kunden herabstufen |
| GET | `/api/v1/clients/:id/orders` | Angemeldet | Bestellliste des Kunden |
| POST | `/api/v1/clients/:id/renew` | Angemeldet | Kunde erneuern |
| POST | `/api/v1/clients/:id/reset-usage` | Angemeldet | Nutzung zurücksetzen |
| POST | `/api/v1/clients/:id/restore` | Angemeldet | Kunde fortsetzen |
| POST | `/api/v1/clients/:id/suspend` | Angemeldet | Kunde sperren |
| POST | `/api/v1/clients/:id/upgrade` | Angemeldet | Upgrade-Kunde |

### Planvorlagen

Pläne (Varianten, Abrechnungszyklen, Preise, Verkehrskontingente) und Knotenautorisierung auf Vorlagenebene.

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| GET | `/api/v1/plan-templates` | Angemeldet | Vorlagenliste |
| POST | `/api/v1/plan-templates` | Angemeldet | Vorlage erstellen |
| DELETE | `/api/v1/plan-templates/:id` | Angemeldet | Vorlage löschen |
| GET | `/api/v1/plan-templates/:id` | Angemeldet | Details zur Vorlage |
| PUT | `/api/v1/plan-templates/:id` | Angemeldet | Vorlage aktualisieren |
| GET | `/api/v1/plan-templates/:id/accessible-nodes` | Angemeldet | Erhalten Sie verfügbare Knoten |
| GET | `/api/v1/plan-templates/:id/billing-node-options` | Angemeldet | Erhalten Sie kostenpflichtige Knotenoptionen |
| PUT | `/api/v1/plan-templates/:id/node-groups` | Angemeldet | Legen Sie Vorlagenknotengruppen fest |
| POST | `/api/v1/plan-templates/:id/sync-accounts` | Angemeldet | Konten mit Vorlage synchronisieren |
| GET | `/api/v1/plan-templates/:id/variants` | Angemeldet | Variantenliste |
| POST | `/api/v1/plan-templates/:id/variants` | Angemeldet | Variante erstellen |
| POST | `/api/v1/plan-templates/sync-all-accounts` | Angemeldet | Alle Konten synchronisieren |

### Planvarianten

Planvarianten aktualisieren und löschen (Erstellung erfolgt unter Planvorlagen).

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| DELETE | `/api/v1/plan-variants/:id` | Angemeldet | Variante löschen |
| PUT | `/api/v1/plan-variants/:id` | Angemeldet | Update-Variante |

### Verkehrspakete

Traffic-Pakete, die separat erworben werden können.

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| GET | `/api/v1/traffic-packs` | Angemeldet | Traffic-Packliste |
| POST | `/api/v1/traffic-packs` | Angemeldet | Traffic-Paket erstellen |
| DELETE | `/api/v1/traffic-packs/:id` | Angemeldet | Verkehrspaket löschen |
| PUT | `/api/v1/traffic-packs/:id` | Angemeldet | Verkehrspaket aktualisieren |

### Gutscheine

Gutscheine erstellen, validieren und einlösen.

| Verfahren | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| GET | `/api/v1/promotions` | Angemeldet | Gutscheine auflisten |
| POST | `/api/v1/promotions` | Angemeldet | Gutschein erstellen |
| DELETE | `/api/v1/promotions/:id` | Angemeldet | Gutschein löschen |
| PUT | `/api/v1/promotions/:id` | Angemeldet | Gutschein aktualisieren |
| POST | `/api/v1/promotions/validate` | Angemeldet | Gutschein validieren |

### Knotengruppen

Organisieren Sie Knoten in Gruppen, auf die in Plänen verwiesen wird.

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| GET | `/api/v1/node-groups` | Angemeldet | Knotengruppen auflisten |
| POST | `/api/v1/node-groups` | Angemeldet | Knotengruppe erstellen |
| DELETE | `/api/v1/node-groups/:id` | Angemeldet | Knotengruppe löschen |
| PUT | `/api/v1/node-groups/:id` | Angemeldet | Knotengruppe aktualisieren |
| GET | `/api/v1/node-groups/:id/members` | Angemeldet | Gruppenmitglieder auflisten |
| POST | `/api/v1/node-groups/:id/members` | Angemeldet | Gruppenmitglied hinzufügen |
| DELETE | `/api/v1/node-groups/:id/members/:mid` | Angemeldet | Gruppenmitglied entfernen |

### Knotenserver

Kernressourcen: Knoten-CRUD, Bereitstellung der Kernkonfiguration (xray/sing-box), Eingangs-/Ausgangs-/Endpunktkonfiguration, Prozesssteuerung, Zertifikate, WAF, Ratenbegrenzungen, Port-Hopping, verkettete Proxys, Spiegelung/primäre Sicherung, Agenteninstallation/-aktualisierung, xray-Laufzeitsteuerung. Viele Unterpfade, gruppiert nach Feature-Familie.

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| GET | `/api/v1/node-servers` | Admin | Liste |
| POST | `/api/v1/node-servers` | Admin | Erstellen |
| DELETE | `/api/v1/node-servers/:id` | Admin | Löschen |
| GET | `/api/v1/node-servers/:id` | Admin | Einzelheiten |
| PUT | `/api/v1/node-servers/:id` | Admin | Aktualisieren |
| GET | `/api/v1/node-servers/:id/binaries` | Admin | Installierte Kernliste |
| POST | `/api/v1/node-servers/:id/binaries/ech-generate` | Admin | Generieren Sie das Schlüsselpaar ECH |
| POST | `/api/v1/node-servers/:id/binaries/install` | Admin | Kern installieren/aktualisieren |
| GET | `/api/v1/node-servers/:id/binaries/interfaces` | Admin | Liste der Netzwerkschnittstellen |
| POST | `/api/v1/node-servers/:id/binaries/mldsa65-key` | Admin | Generieren Sie den Schlüssel MLDSA65 |
| POST | `/api/v1/node-servers/:id/binaries/reality-key` | Admin | Generieren Sie das Schlüsselpaar REALITY |
| POST | `/api/v1/node-servers/:id/binaries/reality-public-key` | Admin | Leiten Sie den öffentlichen Schlüssel vom privaten Schlüssel REALITY ab |
| GET | `/api/v1/node-servers/:id/binaries/releases` | Admin | Verfügbare Versionen |
| POST | `/api/v1/node-servers/:id/binaries/vlessenc` | Admin | Generieren Sie VLESS-Verschlüsselungsparameter |
| POST | `/api/v1/node-servers/:id/binaries/wg-key` | Admin | Generieren Sie den WireGuard-Schlüssel |
| POST | `/api/v1/node-servers/:id/binaries/x25519-key` | Admin | Generieren Sie das Schlüsselpaar X25519 |
| GET | `/api/v1/node-servers/:id/bootstrap-cmd` | Admin | Holen Sie sich den Agent-Bootstrap-Befehl |
| POST | `/api/v1/node-servers/:id/bootstrap-token` | Admin | Bootstrap-Token generieren |
| POST | `/api/v1/node-servers/:id/caddy/batch-update-log-enabled` | Admin | Protokollwechsel zur Massenaktualisierung |
| POST | `/api/v1/node-servers/:id/caddy/batch-update-ports` | Admin | Massenaktualisierungsports |
| POST | `/api/v1/node-servers/:id/caddy/generate-config` | Admin | Konfiguration generieren |
| DELETE | `/api/v1/node-servers/:id/caddy/l4-rules` | Admin | Löschen Sie alle L4-Regeln |
| POST | `/api/v1/node-servers/:id/caddy/save-and-apply` | Admin | Speichern und bewerben |
| GET | `/api/v1/node-servers/:id/caddy/sites` | Admin | Site-Liste |
| POST | `/api/v1/node-servers/:id/caddy/sites` | Admin | Website erstellen |
| DELETE | `/api/v1/node-servers/:id/caddy/sites/:siteId` | Admin | Site löschen |
| PUT | `/api/v1/node-servers/:id/caddy/sites/:siteId` | Admin | Website aktualisieren |
| DELETE | `/api/v1/node-servers/:id/caddy/sni-rules` | Admin | Löschen Sie alle SNI-Regeln |
| POST | `/api/v1/node-servers/:id/caddy/sni-rules/toggle` | Admin | Schalten Sie die SNI-Regeln um |
| GET | `/api/v1/node-servers/:id/caddy/sni-settings` | Admin | SNI-Einstellungen |
| POST | `/api/v1/node-servers/:id/caddy/sni-settings` | Admin | Speichern Sie die SNI-Einstellungen |
| GET | `/api/v1/node-servers/:id/certificates` | Admin | Liste der Knotenzertifikate |
| DELETE | `/api/v1/node-servers/:id/certificates/:cid` | Admin | Zertifikat entbinden |
| POST | `/api/v1/node-servers/:id/certificates/:cid` | Admin | Zertifikat binden |
| GET | `/api/v1/node-servers/:id/chain-config` | Admin | Generieren Sie eine verkettete Konfiguration |
| GET | `/api/v1/node-servers/:id/chain-health` | Admin | Gesundheitszustand der Kette |
| POST | `/api/v1/node-servers/:id/control` | Super Admin | Steuerknotenkern. Körper: `{"core_type":"xray","action":"start"}` (Aktion = Start/Stopp/Neustart). `core_type` kann `xray`/`singbox` sein |
| GET | `/api/v1/node-servers/:id/degradation-status` | Admin | Degradierter Status |
| POST | `/api/v1/node-servers/:id/distribute-cert` | Admin | Zertifikat verteilen |
| GET | `/api/v1/node-servers/:id/endpoint-configs` | Admin | Liste |
| POST | `/api/v1/node-servers/:id/endpoint-configs` | Admin | Erstellen |
| DELETE | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | Admin | Löschen |
| GET | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | Admin | Einzelheiten |
| PUT | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | Admin | Aktualisieren |
| POST | `/api/v1/node-servers/:id/failover` | Admin | Failover für diesen Knoten auslösen |
| GET | `/api/v1/node-servers/:id/failover-status` | Admin | Failover-Status |
| POST | `/api/v1/node-servers/:id/force-reconnect` | Admin | Wiederverbindung erzwingen |
| GET | `/api/v1/node-servers/:id/free-port` | Admin | Fordern Sie einen kostenlosen Port an |
| POST | `/api/v1/node-servers/:id/geo-rules/push` | Admin | Push zu diesem Knoten |
| GET | `/api/v1/node-servers/:id/inbound-configs` | Admin | Liste |
| POST | `/api/v1/node-servers/:id/inbound-configs` | Admin | Erstellen |
| DELETE | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | Admin | Löschen |
| GET | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | Admin | Einzelheiten |
| PUT | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | Admin | Aktualisieren |
| GET | `/api/v1/node-servers/:id/inbound-configs/:cfgId/share-links` | Admin | Teilen Sie Links und Snippets |
| POST | `/api/v1/node-servers/:id/inbound-configs/:cfgId/users/:email/pause` | Admin | Sperren Sie diesen Benutzer |
| POST | `/api/v1/node-servers/:id/inbound-configs/:cfgId/users/:email/resume` | Admin | Stellen Sie diesen Benutzer wieder her |
| POST | `/api/v1/node-servers/:id/inbound-configs/reset-all-traffic` | Admin | Setzen Sie den gesamten eingehenden Datenverkehr zurück |
| GET | `/api/v1/node-servers/:id/install-script` | Admin | Rufen Sie das Agent-Installationsskript ab |
| GET | `/api/v1/node-servers/:id/metrics-history` | Admin | Rufen Sie den Metrikverlauf ab |
| GET | `/api/v1/node-servers/:id/mirror-status` | Admin | Spiegelstatus |
| POST | `/api/v1/node-servers/:id/nginx/batch-update-ports` | Admin | Massenaktualisierungsports |
| POST | `/api/v1/node-servers/:id/nginx/generate-config` | Admin | Konfiguration generieren |
| DELETE | `/api/v1/node-servers/:id/nginx/l4-rules` | Admin | Löschen Sie alle L4-Regeln |
| POST | `/api/v1/node-servers/:id/nginx/save-and-apply` | Admin | Speichern und bewerben |
| GET | `/api/v1/node-servers/:id/nginx/settings` | Admin | Einstellungen lesen |
| POST | `/api/v1/node-servers/:id/nginx/settings` | Admin | Einstellungen speichern |
| GET | `/api/v1/node-servers/:id/nginx/sites` | Admin | Site-Liste |
| POST | `/api/v1/node-servers/:id/nginx/sites` | Admin | Website erstellen |
| DELETE | `/api/v1/node-servers/:id/nginx/sites/:siteId` | Admin | Site löschen |
| PUT | `/api/v1/node-servers/:id/nginx/sites/:siteId` | Admin | Website aktualisieren |
| DELETE | `/api/v1/node-servers/:id/nginx/sni-rules` | Admin | Löschen Sie alle SNI-Regeln |
| POST | `/api/v1/node-servers/:id/nginx/sni-rules/toggle` | Admin | Schalten Sie die SNI-Regeln um |
| PUT | `/api/v1/node-servers/:id/node-type` | Admin | Knotentyp aktualisieren |
| GET | `/api/v1/node-servers/:id/online-users` | Admin | Online-Benutzer |
| GET | `/api/v1/node-servers/:id/outbound-configs` | Admin | Liste |
| POST | `/api/v1/node-servers/:id/outbound-configs` | Admin | Erstellen |
| DELETE | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | Admin | Löschen |
| GET | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | Admin | Einzelheiten |
| PUT | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | Admin | Aktualisieren |
| PUT | `/api/v1/node-servers/:id/outbound-configs/reorder` | Admin | Sortieren |
| GET | `/api/v1/node-servers/:id/port-hop/rules` | Admin | Vorschau der Port-Hopping-Regeln |
| GET | `/api/v1/node-servers/:id/protocols` | Admin | Protokollliste |
| GET | `/api/v1/node-servers/:id/protocols/:coreType` | Admin | Protokollkonfiguration |
| PUT | `/api/v1/node-servers/:id/protocols/:coreType` | Admin | Protokollkonfiguration speichern |
| GET | `/api/v1/node-servers/:id/protocols/:coreType/config-file` | Admin | Kernkonfigurationsdatei lesen |
| GET | `/api/v1/node-servers/:id/protocols/:coreType/section/:section` | Admin | Protokollabschnitt lesen |
| PUT | `/api/v1/node-servers/:id/protocols/:coreType/section/:section` | Admin | Protokollabschnitt speichern |
| POST | `/api/v1/node-servers/:id/protocols/:coreType/validate` | Admin | Konfiguration validieren (realer Kern-Trockenlauf) |
| GET | `/api/v1/node-servers/:id/proxy-status` | Admin | Remote-Proxy-Status |
| POST | `/api/v1/node-servers/:id/proxy/batch-update-ports` | Admin | Massenaktualisierungsports |
| GET | `/api/v1/node-servers/:id/proxy/config` | Admin | Konfiguration lesen |
| PUT | `/api/v1/node-servers/:id/proxy/config` | Admin | Konfiguration aktualisieren |
| POST | `/api/v1/node-servers/:id/proxy/control` | Admin | Verkettete Proxy-Registrierungskontrolle |
| POST | `/api/v1/node-servers/:id/proxy/download` | Admin | Laden Sie die Proxy-Core-Version herunter |
| POST | `/api/v1/node-servers/:id/proxy/generate-config` | Admin | Konfiguration generieren |
| POST | `/api/v1/node-servers/:id/proxy/guardian` | Admin | Supervisor-Konfiguration aktualisieren |
| POST | `/api/v1/node-servers/:id/proxy/install` | Admin | Proxy installieren |
| DELETE | `/api/v1/node-servers/:id/proxy/l4-rules` | Admin | Löschen Sie alle L4-Regeln |
| GET | `/api/v1/node-servers/:id/proxy/l4-settings` | Admin | L4-Einstellungen |
| POST | `/api/v1/node-servers/:id/proxy/l4-settings` | Admin | L4-Einstellungen speichern |
| GET | `/api/v1/node-servers/:id/proxy/log-settings` | Admin | Protokolleinstellungen |
| POST | `/api/v1/node-servers/:id/proxy/log-settings` | Admin | Protokolleinstellungen speichern |
| GET | `/api/v1/node-servers/:id/proxy/nginx-settings` | Admin | nginx-Einstellungen |
| POST | `/api/v1/node-servers/:id/proxy/nginx-settings` | Admin | Speichern Sie die nginx-Einstellungen |
| GET | `/api/v1/node-servers/:id/proxy/releases` | Admin | Verfügbare Versionen |
| POST | `/api/v1/node-servers/:id/proxy/reload` | Admin | Konfiguration neu laden |
| POST | `/api/v1/node-servers/:id/proxy/restart` | Admin | Neustart |
| GET | `/api/v1/node-servers/:id/proxy/sites` | Admin | Site-Liste |
| POST | `/api/v1/node-servers/:id/proxy/sites` | Admin | Website erstellen |
| DELETE | `/api/v1/node-servers/:id/proxy/sites/:siteId` | Admin | Site löschen |
| PUT | `/api/v1/node-servers/:id/proxy/sites/:siteId` | Admin | Website aktualisieren |
| DELETE | `/api/v1/node-servers/:id/proxy/sni-rules` | Admin | Löschen Sie alle SNI-Regeln |
| POST | `/api/v1/node-servers/:id/proxy/sni-rules/toggle` | Admin | Schalten Sie die SNI-Regeln um |
| GET | `/api/v1/node-servers/:id/proxy/sni-settings` | Admin | SNI-Einstellungen |
| POST | `/api/v1/node-servers/:id/proxy/sni-settings` | Admin | Speichern Sie die SNI-Einstellungen |
| POST | `/api/v1/node-servers/:id/proxy/start` | Admin | Starten |
| GET | `/api/v1/node-servers/:id/proxy/status` | Admin | Proxy-Prozessstatus |
| POST | `/api/v1/node-servers/:id/proxy/stop` | Admin | Stopp |
| POST | `/api/v1/node-servers/:id/proxy/sync` | Admin | Proxy-Konfiguration synchronisieren |
| GET | `/api/v1/node-servers/:id/rate-limit` | Admin | Konfiguration lesen |
| PUT | `/api/v1/node-servers/:id/rate-limit` | Admin | Konfiguration aktualisieren |
| GET | `/api/v1/node-servers/:id/rate-limit/status` | Admin | Status |
| GET | `/api/v1/node-servers/:id/recover-mode` | Admin | Knotenwiederherstellungsmodus lesen |
| PUT | `/api/v1/node-servers/:id/recover-mode` | Admin | Stellen Sie den Knotenwiederherstellungsmodus ein |
| POST | `/api/v1/node-servers/:id/revoke-cert` | Admin | Agent-Zertifikat widerrufen |
| GET | `/api/v1/node-servers/:id/services` | Admin | Liste |
| POST | `/api/v1/node-servers/:id/services` | Admin | Erstellen |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId` | Admin | Löschen |
| GET | `/api/v1/node-servers/:id/services/:serviceId` | Admin | Einzelheiten |
| PUT | `/api/v1/node-servers/:id/services/:serviceId` | Admin | Aktualisieren |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId/crash-logs` | Admin | Absturzprotokolle löschen |
| GET | `/api/v1/node-servers/:id/services/:serviceId/crash-logs` | Admin | Absturzprotokollliste |
| GET | `/api/v1/node-servers/:id/services/:serviceId/crash-logs/:logId` | Admin | Details zum Absturzprotokoll |
| GET | `/api/v1/node-servers/:id/services/:serviceId/endpoints` | Admin | Endpunktliste |
| POST | `/api/v1/node-servers/:id/services/:serviceId/endpoints` | Admin | Endpunkt erstellen |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId/endpoints/:endpointId` | Admin | Endpunkt löschen |
| PUT | `/api/v1/node-servers/:id/services/:serviceId/endpoints/:endpointId` | Admin | Endpunkt aktualisieren |
| PUT | `/api/v1/node-servers/:id/services/:serviceId/guardian` | Admin | Supervisor-Konfiguration aktualisieren |
| GET | `/api/v1/node-servers/:id/services/:serviceId/process-status` | Admin | Bearbeitungsstatus abrufen |
| POST | `/api/v1/node-servers/:id/services/:serviceId/restart` | Admin | Neustart |
| POST | `/api/v1/node-servers/:id/services/:serviceId/start` | Admin | Starten |
| POST | `/api/v1/node-servers/:id/services/:serviceId/stop` | Admin | Stopp |
| GET | `/api/v1/node-servers/:id/services/process-status` | Admin | Alle Prozesszustände |
| GET | `/api/v1/node-servers/:id/share-domains` | Admin | Geben Sie Domänen für diesen Knoten frei |
| POST | `/api/v1/node-servers/:id/share-domains` | Admin | Erstellen |
| DELETE | `/api/v1/node-servers/:id/share-domains/:domainId` | Admin | Löschen |
| PUT | `/api/v1/node-servers/:id/share-domains/:domainId` | Admin | Aktualisieren |
| DELETE | `/api/v1/node-servers/:id/share-domains/all` | Admin | Alles löschen |
| POST | `/api/v1/node-servers/:id/singbox/control/firewall/apply` | Admin | Wenden Sie Port-Hopping-Firewallregeln an |
| POST | `/api/v1/node-servers/:id/singbox/control/firewall/remove` | Admin | Entfernen Sie die Port-Hopping-Firewallregeln |
| POST | `/api/v1/node-servers/:id/speed-test` | Admin | Knotengeschwindigkeitstest |
| GET | `/api/v1/node-servers/:id/status` | Admin | Knotenstatus |
| POST | `/api/v1/node-servers/:id/sync` | Admin | Knotenkonfiguration synchronisieren (generieren und pushen) |
| POST | `/api/v1/node-servers/:id/sync-data` | Super Admin | Panel-Daten an Knoten übertragen (vollständige Synchronisierung von Benutzern/Abonnements/Plänen) |
| POST | `/api/v1/node-servers/:id/sync-mirror` | Admin | Spiegelknoten synchronisieren |
| GET | `/api/v1/node-servers/:id/tags` | Admin | Tag-Liste |
| POST | `/api/v1/node-servers/:id/tags` | Admin | Tag hinzufügen |
| DELETE | `/api/v1/node-servers/:id/tags/:tagKey` | Admin | Tag löschen |
| GET | `/api/v1/node-servers/:id/traffic-history` | Admin | Verkehrsgeschichte |
| POST | `/api/v1/node-servers/:id/traffic-resume` | Admin | Stellen Sie den wegen Überschreitung unterbrochenen Datenverkehr wieder her |
| GET | `/api/v1/node-servers/:id/traffic-usage` | Admin | Verkehrsnutzung |
| POST | `/api/v1/node-servers/:id/update-agent` | Admin | Upgrade-Agent |
| GET | `/api/v1/node-servers/:id/used-server-names` | Admin | Liste der verwendeten REALITY SNI |
| GET | `/api/v1/node-servers/:id/waf/config` | Admin | Konfiguration lesen |
| PUT | `/api/v1/node-servers/:id/waf/config` | Admin | Konfiguration speichern |
| POST | `/api/v1/node-servers/:id/waf/disable-domain` | Admin | Deaktivieren Sie den Domänenschutz |
| GET | `/api/v1/node-servers/:id/waf/domain-config` | Admin | Liste der Domänenkonfigurationen |
| PUT | `/api/v1/node-servers/:id/waf/domain-config` | Admin | Massenspeicherung von Domänenkonfigurationen |
| DELETE | `/api/v1/node-servers/:id/waf/domain-config/:domain` | Admin | Domänenkonfiguration löschen |
| GET | `/api/v1/node-servers/:id/waf/domain-config/:domain` | Admin | Details zur Domänenkonfiguration |
| PUT | `/api/v1/node-servers/:id/waf/domain-config/:domain` | Admin | Domänenkonfiguration speichern |
| GET | `/api/v1/node-servers/:id/waf/domain-rules` | Admin | Verzeichnis der Domänenregeln |
| GET | `/api/v1/node-servers/:id/waf/domain-rules/:domain/:name` | Admin | Domänenregeldatei lesen |
| PUT | `/api/v1/node-servers/:id/waf/domain-rules/:domain/:name` | Admin | Domänenregeldatei speichern |
| GET | `/api/v1/node-servers/:id/waf/domain-status` | Admin | Domain-Schutzstatus |
| POST | `/api/v1/node-servers/:id/waf/enable-domain` | Admin | Aktivieren Sie den Domänenschutz |
| POST | `/api/v1/node-servers/:id/waf/push` | Admin | Push zu diesem Knoten |
| GET | `/api/v1/node-servers/:id/waf/rules` | Admin | Regelliste des Knotens WAF |
| DELETE | `/api/v1/node-servers/:id/waf/rules/:name` | Admin | Löschen Sie die Regeln des Knotens WAF |
| GET | `/api/v1/node-servers/:id/waf/rules/:name` | Admin | Details zur Regel des Knotens WAF |
| PUT | `/api/v1/node-servers/:id/waf/rules/:name` | Admin | Speichern Sie die Regeln des Knotens WAF |
| GET | `/api/v1/node-servers/:id/waf/status` | Admin | Knoten WAF-Status |
| PUT | `/api/v1/node-servers/:id/waf/status` | Admin | Toggle-Knoten WAF |
| POST | `/api/v1/node-servers/:id/waf/sync` | Admin | Mit diesem Knoten synchronisieren |
| GET | `/api/v1/node-servers/:id/warp` | Admin | Liste |
| DELETE | `/api/v1/node-servers/:id/warp/:warpId` | Admin | Löschen |
| GET | `/api/v1/node-servers/:id/warp/config` | Admin | Konfiguration lesen |
| POST | `/api/v1/node-servers/:id/warp/generate-config` | Admin | Konfiguration generieren |
| POST | `/api/v1/node-servers/:id/warp/register` | Admin | Registrieren |
| GET | `/api/v1/node-servers/:id/warp/status` | Admin | Status |
| GET | `/api/v1/node-servers/:id/xray/control/balancer/:tag` | Admin | Erhalten Sie Balancer-Informationen |
| POST | `/api/v1/node-servers/:id/xray/control/balancer/:tag` | Admin | Balancer-Ziel festlegen |
| POST | `/api/v1/node-servers/:id/xray/control/firewall/apply` | Admin | Wenden Sie Port-Hopping-Firewallregeln an |
| POST | `/api/v1/node-servers/:id/xray/control/firewall/remove` | Admin | Entfernen Sie die Port-Hopping-Firewallregeln |
| POST | `/api/v1/node-servers/:id/xray/control/inbound` | Admin | Eingehend hinzufügen |
| DELETE | `/api/v1/node-servers/:id/xray/control/inbound/:tag` | Admin | Eingehend entfernen |
| POST | `/api/v1/node-servers/:id/xray/control/inbound/:tag/user` | Admin | Benutzer hinzufügen |
| DELETE | `/api/v1/node-servers/:id/xray/control/inbound/:tag/user/:email` | Admin | Benutzer entfernen |
| POST | `/api/v1/node-servers/:id/xray/control/outbound` | Admin | Ausgehend hinzufügen |
| DELETE | `/api/v1/node-servers/:id/xray/control/outbound/:tag` | Admin | Ausgehend entfernen |
| POST | `/api/v1/node-servers/:id/xray/control/routing` | Admin | Route anfordern |
| POST | `/api/v1/node-servers/:id/xray/control/routing/rule` | Admin | Routing-Regel hinzufügen |
| DELETE | `/api/v1/node-servers/:id/xray/control/routing/rule/:tag` | Admin | Routing-Regel entfernen |
| GET | `/api/v1/node-servers/:id/xray/control/routing/rules` | Admin | Routing-Regeln auflisten |
| POST | `/api/v1/node-servers/:id/xray/control/sync` | Admin | Manuelles Auslösen der xray-Konfigurationssynchronisierung |
| GET | `/api/v1/node-servers/:id/xray/stats` | Admin | Statistiken abrufen |
| POST | `/api/v1/node-servers/:id/xray/stats/reset/:email` | Admin | Benutzerstatistiken zurücksetzen |
| GET | `/api/v1/node-servers/:id/xray/stats/users` | Admin | Erhalten Sie Benutzerstatistiken |
| GET | `/api/v1/node-servers/billable` | Admin | Liste der abrechenbaren Knoten |
| GET | `/api/v1/node-servers/ca-cert` | Admin | CA-Zertifikat |
| GET | `/api/v1/node-servers/ca-status` | Admin | CA-Status |
| GET | `/api/v1/node-servers/dashboard` | Admin | Dashboard-Status |
| POST | `/api/v1/node-servers/distribute-cert` | Admin | Verteilen Sie das Zertifikat an alle Knoten |
| POST | `/api/v1/node-servers/failover-check` | Admin | Failover-Prüfung auslösen |
| GET | `/api/v1/node-servers/metrics/aggregated` | Admin | Aggregierte Metriken |
| POST | `/api/v1/node-servers/migrate-cert` | Admin | Zertifikat migrieren |
| GET | `/api/v1/node-servers/online-summary` | Admin | Online-Zusammenfassung |
| GET | `/api/v1/node-servers/online-users` | Admin | Alle Online-Benutzer |
| GET | `/api/v1/node-servers/pending-agent-updates` | Admin | Agenten warten auf Upgrade |
| POST | `/api/v1/node-servers/recommend-region` | Admin | Empfohlene Regionen |
| POST | `/api/v1/node-servers/select-nodes` | Admin | Wählen Sie Knoten für das Abonnement aus |
| POST | `/api/v1/node-servers/speed-test-all` | Admin | Geschwindigkeitstest aller Knoten |
| GET | `/api/v1/node-servers/speed-test-results` | Admin | Ergebnisse des Geschwindigkeitstests |
| GET | `/api/v1/node-servers/traffic/stats` | Admin | Verkehrsstatistik |
| POST | `/api/v1/node-servers/update-all-agents` | Admin | Aktualisieren Sie alle Agenten |
| DELETE | `/api/v1/node-servers/users/:client_id` | Admin | Einen Kunden synchronisieren oder löschen |
| GET | `/api/v1/node-servers/users/retry-queue` | Admin | Status der Wiederholungswarteschlange |
| POST | `/api/v1/node-servers/users/retry-queue/process` | Admin | Warteschlange für Prozesswiederholungen |
| POST | `/api/v1/node-servers/users/sync` | Admin | Synchronisieren Sie einen einzelnen Kunden |
| POST | `/api/v1/node-servers/validate-chain-config` | Admin | Verkettete Konfiguration validieren |

### Knotenüberwachungsprotokoll

Knotenseitige Betriebsprüfung und Export.

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| GET | `/api/v1/node-audit-logs` | Angemeldet | Knotenüberwachungsprotokoll |
| GET | `/api/v1/node-audit-logs/export` | Angemeldet | Exportieren Sie Knotenüberwachungsprotokolle |

### Knoten (Legacy-Modell, im Ruhestand)

> `/api/v1/nodes` CRUD wurde am 20.10.2026 eingestellt: Knoten werden einheitlich über `/api/v1/node-servers` verwaltet.
> Online-Benutzeranfragen bleiben bei `/api/v1/node-servers/{online-users,online-summary,:id/online-users}`.

### Regionsübersicht

Knotenansicht aggregiert nach Region.

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| GET | `/api/v1/regions/:region/servers` | Angemeldet | Knotenliste für diese Region |
| GET | `/api/v1/regions/overview` | Angemeldet | Regionsübersicht |

### Abonnements

Verwaltung von Abonnementdatensätzen: widerrufen, Token rotieren, Zugriffsprotokolle, Nutzungsrücksetzung.

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| GET | `/api/v1/subscriptions` | Angemeldet | Liste |
| POST | `/api/v1/subscriptions` | Angemeldet | Erstellen |
| DELETE | `/api/v1/subscriptions/:id` | Angemeldet | Löschen |
| GET | `/api/v1/subscriptions/:id` | Angemeldet | Einzelheiten |
| PUT | `/api/v1/subscriptions/:id` | Angemeldet | Aktualisieren |
| GET | `/api/v1/subscriptions/:id/access-logs` | Angemeldet | Zugriffsprotokoll |
| POST | `/api/v1/subscriptions/:id/enable` | Angemeldet | Aktivieren |
| POST | `/api/v1/subscriptions/:id/reset-usage` | Angemeldet | Nutzung zurücksetzen |
| POST | `/api/v1/subscriptions/:id/restore` | Angemeldet | Lebenslauf |
| POST | `/api/v1/subscriptions/:id/revoke` | Angemeldet | Widerrufen |
| POST | `/api/v1/subscriptions/:id/rotate` | Angemeldet | Drehen |
| GET | `/api/v1/subscriptions/:id/rotations` | Angemeldet | Rotationsgeschichte |

### Abonnementformate

> Admin-seitiger `/api/v1/subscription-formats` CRUD wurde am 20.10.2026 eingestellt: Der eigentliche Einstiegspunkt ist
> `/api/v1/mihomo/configs` (Abonnementformatzentrum). Die `subscription_formats`-Tabelle bleibt bestehen,
> Wird nur als Seed-Daten für den öffentlichen `/api/v1/public/client-configs`-Endpunkt verwendet.

### Abonnementendpunkte

Dedizierte Abonnementdomänen (Reverse-Proxy zum `/sub`-Pfad des Panels).

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| GET | `/api/v1/subscription-endpoints` | Angemeldet | Liste |
| POST | `/api/v1/subscription-endpoints` | Angemeldet | Erstellen |
| DELETE | `/api/v1/subscription-endpoints/:id` | Angemeldet | Löschen |
| GET | `/api/v1/subscription-endpoints/:id` | Angemeldet | Einzelheiten |
| PUT | `/api/v1/subscription-endpoints/:id` | Angemeldet | Aktualisieren |

### Domains teilen

Zugriffsdomäne/IP-Pool, der in Client-Links verwendet wird.

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| GET | `/api/v1/share-domains` | Angemeldet | Liste |
| POST | `/api/v1/share-domains` | Angemeldet | Erstellen |
| DELETE | `/api/v1/share-domains/:id` | Angemeldet | Löschen |
| PUT | `/api/v1/share-domains/:id` | Angemeldet | Aktualisieren |
| DELETE | `/api/v1/share-domains/all` | Angemeldet | Alles löschen |
| POST | `/api/v1/share-domains/seed` | Angemeldet | Standarddaten initialisieren |

### Cloudflare IP

Cloudflare CDN IP-Pool- und Quellenverwaltung.

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| DELETE | `/api/v1/cfip/entries` | Angemeldet | Einträge löschen |
| GET | `/api/v1/cfip/entries` | Angemeldet | Eintragsliste |
| POST | `/api/v1/cfip/entries` | Angemeldet | Eintrag hinzufügen |
| DELETE | `/api/v1/cfip/entries/:id` | Angemeldet | Eintrag löschen |
| POST | `/api/v1/cfip/fetch-all` | Angemeldet | Rufen Sie alle Quellen ab |
| POST | `/api/v1/cfip/seed` | Angemeldet | Standarddaten initialisieren |
| GET | `/api/v1/cfip/sources` | Angemeldet | Liste der Datenquellen |
| POST | `/api/v1/cfip/sources` | Angemeldet | Datenquelle erstellen |
| DELETE | `/api/v1/cfip/sources/:id` | Angemeldet | Datenquelle löschen |
| PUT | `/api/v1/cfip/sources/:id` | Angemeldet | Datenquelle aktualisieren |
| POST | `/api/v1/cfip/sources/:id/fetch` | Angemeldet | Holen Sie sich diese Quelle |

### Abonnementkonfiguration (Mihomo/Clash)

Konfiguration der Abonnementvorlage: Gruppen, Regeln, Regelanbieter, YAML-Vorschau.

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| GET | `/api/v1/mihomo/configs` | Angemeldet | Liste |
| POST | `/api/v1/mihomo/configs` | Angemeldet | Erstellen |
| DELETE | `/api/v1/mihomo/configs/:id` | Angemeldet | Löschen |
| GET | `/api/v1/mihomo/configs/:id` | Angemeldet | Einzelheiten |
| PUT | `/api/v1/mihomo/configs/:id` | Angemeldet | Aktualisieren |
| GET | `/api/v1/mihomo/configs/:id/proxy-groups` | Angemeldet | Gruppenliste |
| POST | `/api/v1/mihomo/configs/:id/proxy-groups` | Angemeldet | Gruppe erstellen |
| GET | `/api/v1/mihomo/configs/:id/rule-providers` | Angemeldet | Liste der Regelanbieter |
| POST | `/api/v1/mihomo/configs/:id/rule-providers` | Angemeldet | Regelanbieter erstellen |
| GET | `/api/v1/mihomo/configs/:id/rules` | Angemeldet | Regelliste |
| POST | `/api/v1/mihomo/configs/:id/rules` | Angemeldet | Regel erstellen |
| GET | `/api/v1/mihomo/configs/:id/yaml` | Angemeldet | Vorschau generiert YAML |
| DELETE | `/api/v1/mihomo/proxy-groups/:id` | Angemeldet | Gruppe löschen |
| DELETE | `/api/v1/mihomo/rule-providers/:id` | Angemeldet | Regelanbieter löschen |
| DELETE | `/api/v1/mihomo/rules/:id` | Angemeldet | Regel löschen |

### Aufladeseiten

Integration von Zahlungsseiten von Drittanbietern: Signaturschlüssel, Verifizierungsdateien, Domänenverifizierung, Integrationsdokumente.

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| GET | `/api/v1/charge-sites` | Angemeldet | Site-Liste |
| POST | `/api/v1/charge-sites` | Angemeldet | Website erstellen |
| DELETE | `/api/v1/charge-sites/:id` | Angemeldet | Site löschen |
| GET | `/api/v1/charge-sites/:id` | Angemeldet | Details zur Website |
| PUT | `/api/v1/charge-sites/:id` | Angemeldet | Website aktualisieren |
| POST | `/api/v1/charge-sites/:id/clear-prev-secret` | Angemeldet | Alten Schlüssel löschen |
| POST | `/api/v1/charge-sites/:id/generate-verify` | Angemeldet | Verifizierungsdatei erstellen |
| POST | `/api/v1/charge-sites/:id/generate-verify-file` | Angemeldet | Verifizierungsdatei erstellen |
| GET | `/api/v1/charge-sites/:id/integration-doc` | Angemeldet | Holen Sie sich das Integrationsdokument |
| POST | `/api/v1/charge-sites/:id/rotate-secret` | Angemeldet | Signierschlüssel drehen |
| POST | `/api/v1/charge-sites/:id/verify-dns` | Angemeldet | Überprüfen Sie die Domäne DNS |
| POST | `/api/v1/charge-sites/:id/verify-domain` | Angemeldet | Überprüfen Sie die Domäne HTTP-Challenge |
| POST | `/api/v1/charge-sites/:id/verify-domain-dns` | Angemeldet | Überprüfen Sie die Domäne DNS |
| POST | `/api/v1/charge-sites/:id/verify-domain-http` | Angemeldet | Überprüfen Sie die Domäne HTTP-Challenge |

### Bestellungen aufladen

Abfragen zu Aufladebestellungen, Rückerstattungen, CSV-Export.

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| GET | `/api/v1/charge-orders` | Angemeldet | Bestellliste |
| GET | `/api/v1/charge-orders/:id` | Angemeldet | Bestelldetails |
| POST | `/api/v1/charge-orders/:id/refund` | Angemeldet | Rückerstattung |
| GET | `/api/v1/charge-orders/export-csv` | Angemeldet | Aufladeaufträge exportieren CSV |

### Karten aufladen

Massenkartengenerierung, Nutzungsdatensätze, CSV-Export.

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| GET | `/api/v1/charge-cards` | Angemeldet | Kartenliste |
| POST | `/api/v1/charge-cards` | Angemeldet | Karte erstellen |
| DELETE | `/api/v1/charge-cards/:id` | Angemeldet | Karte löschen |
| GET | `/api/v1/charge-cards/:id/usages` | Angemeldet | Aufzeichnungen zur Kartennutzung |
| POST | `/api/v1/charge-cards/batch` | Angemeldet | Massengenerierung von Karten |
| GET | `/api/v1/charge-cards/export-csv` | Angemeldet | Exportkarten CSV |

### Gebührenrückrufereignisse

Prüfung von Rückrufereignissen.

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| GET | `/api/v1/charge-callback-events` | Angemeldet | Liste der Rückrufereignisse |

### Wiederholungsversuche für kostenpflichtige Rückrufe

Manuelle Wiederholung fehlgeschlagener Rückrufe.

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| GET | `/api/v1/charge-callback-retries` | Angemeldet | Liste fehlgeschlagener Rückrufe |
| POST | `/api/v1/charge-callback-retries/:id/retry` | Admin | Rückruf erneut versuchen |

### Kundensaldo

Admin-Ansicht der Salden und Transaktionen.

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| GET | `/api/v1/client-balances` | Angemeldet | Kundensaldoliste |
| GET | `/api/v1/client-balances/transactions` | Angemeldet | Saldotransaktionen (Administrator) |
| GET | `/api/v1/client-balances/transactions/export-csv` | Angemeldet | Exportsaldotransaktionen CSV |

### Geschenke

Geben Sie beanspruchbare Geschenke aus (Tage/Verkehr).

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| GET | `/api/v1/client-gifts` | Angemeldet | Geschenkliste |
| POST | `/api/v1/client-gifts` | Angemeldet | Geschenk erstellen |
| POST | `/api/v1/client-gifts/batch` | Angemeldet | Geschenke in großen Mengen erstellen |
| GET | `/api/v1/client-gifts/search-clients` | Angemeldet | Kunden suchen |

### Empfehlungskonfiguration

Konfiguration und Statistik der Empfehlungsprovision.

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| GET | `/api/v1/referral/config` | Angemeldet | Empfehlungskonfiguration |
| PUT | `/api/v1/referral/config` | Admin | Empfehlungskonfiguration speichern |
| GET | `/api/v1/referral/records` | Angemeldet | Liste der Empfehlungsdatensätze |
| GET | `/api/v1/referral/stats` | Angemeldet | Empfehlungsstatistiken |

### Auszahlungen aufgrund von Empfehlungen

Überprüfen Sie Auszahlungsanträge.

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| GET | `/api/v1/referral-withdrawals` | Angemeldet | Liste der Auszahlungsanfragen |
| POST | `/api/v1/referral-withdrawals/:id/approve` | Angemeldet | Entnahme genehmigen |
| POST | `/api/v1/referral-withdrawals/:id/reject` | Angemeldet | Auszahlung ablehnen |

### IP-Zulassungslisten (Zusammenfassung)

Ansicht der IP-Zulassungsliste für alle Kunden.

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| GET | `/api/v1/ip-whitelists` | Angemeldet | Alle IP-Zulassungslisten des Kunden |

### Kunden-OAuth-Bindungen

Fragen Sie Kundenkontobindungen von Drittanbietern ab.

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| GET | `/api/v1/client-oauth-bindings` | Angemeldet | Kunden-OAuth-Bindungsliste |

### Tickets (admin)

Antwort, Zuweisung und Abschluss des Tickets.

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| GET | `/api/v1/tickets` | Angemeldet | Ticketliste |
| GET | `/api/v1/tickets/:id` | Angemeldet | Ticketdetails |
| POST | `/api/v1/tickets/:id/assign` | Angemeldet | Ticket zuweisen |
| POST | `/api/v1/tickets/:id/close` | Angemeldet | Ticket schließen |
| POST | `/api/v1/tickets/:id/reply` | Angemeldet | Auf Ticket antworten |

### Artikel

CRUD für Portalartikel.

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| GET | `/api/v1/articles` | Angemeldet | Artikelliste |
| POST | `/api/v1/articles` | Angemeldet | Artikel erstellen |
| DELETE | `/api/v1/articles/:id` | Angemeldet | Artikel löschen |
| GET | `/api/v1/articles/:id` | Angemeldet | Artikeldetails |
| PUT | `/api/v1/articles/:id` | Angemeldet | Artikel aktualisieren |

### Geteilte Konten

Ausstellungs- und Reparaturaufzeichnungen für gemeinsame Konten (Streaming usw.).

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| GET | `/api/v1/shared-accounts` | Angemeldet | Geteilte Kontoliste |
| POST | `/api/v1/shared-accounts` | Angemeldet | Gemeinsames Konto erstellen |
| DELETE | `/api/v1/shared-accounts/:id` | Angemeldet | Gemeinsames Konto löschen |
| GET | `/api/v1/shared-accounts/:id` | Angemeldet | Geteilte Kontodaten |
| PUT | `/api/v1/shared-accounts/:id` | Angemeldet | Gemeinsames Konto aktualisieren |
| GET | `/api/v1/shared-accounts/:id/repair-logs` | Angemeldet | Reparaturaufzeichnungen |

### Zertifikate

Ausstellung, Hochladen, Erneuerung und Export öffentlicher Zertifikate (ACME); Die Ausgabe auf einem bestimmten Knoten wird unterstützt.

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| GET | `/api/v1/certificates` | Angemeldet | Liste |
| POST | `/api/v1/certificates` | Angemeldet | Erstellen |
| DELETE | `/api/v1/certificates/:id` | Angemeldet | Löschen |
| GET | `/api/v1/certificates/:id` | Angemeldet | Einzelheiten |
| PUT | `/api/v1/certificates/:id` | Angemeldet | Aktualisieren |
| GET | `/api/v1/certificates/:id/download` | Angemeldet | Herunterladen |
| POST | `/api/v1/certificates/:id/renew` | Angemeldet | Kunde erneuern |
| GET | `/api/v1/certificates/check-port` | Angemeldet | Überprüfen Sie den Port |
| GET | `/api/v1/certificates/detect-ip` | Angemeldet | Sonden-IP |
| GET | `/api/v1/certificates/export` | Angemeldet | Exportieren |
| POST | `/api/v1/certificates/issue` | Angemeldet | Problem. Körper: `{"domains":["example.com","*.example.com"],"challenge_type":"dns-01","dns_channel_id":2,"key_type":"rsa","key_bits":4096}`. `challenge_type` ist standardmäßig http-01; `dns_channel_id` wählt einen DNS-Kanal aus (einen konfigurierten DNS-Anbieter + Anmeldeinformationen). |
| POST | `/api/v1/certificates/issue-on-node` | Angemeldet | Ausgabe an Knoten |
| POST | `/api/v1/certificates/upload` | Angemeldet | Hochladen |

### Zertifikatskanäle

ACME-Kanalkonfiguration (CA).

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| GET | `/api/v1/cert-channels` | Angemeldet | Liste |
| POST | `/api/v1/cert-channels` | Angemeldet | Erstellen |
| DELETE | `/api/v1/cert-channels/:id` | Angemeldet | Löschen |
| PUT | `/api/v1/cert-channels/:id` | Angemeldet | Aktualisieren |
| GET | `/api/v1/cert-channels/providers` | Angemeldet | Anbieter |

### EAB-Konfiguration

Anmeldeinformationen, die externe Konten wie ZeroSSL / Google binden.

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| GET | `/api/v1/eab-configs` | Angemeldet | Liste |
| POST | `/api/v1/eab-configs` | Angemeldet | Erstellen |
| DELETE | `/api/v1/eab-configs/:id` | Angemeldet | Löschen |
| PUT | `/api/v1/eab-configs/:id` | Angemeldet | Aktualisieren |
| POST | `/api/v1/eab-configs/fetch-google` | Angemeldet | Rufen Sie Google EAB von GCP ab |
| POST | `/api/v1/eab-configs/fetch-zerossl` | Angemeldet | Rufen Sie ZeroSSL EAB mit einer E-Mail mit einem Klick ab |

### ACME-Konten

Panelseitige Verwaltung des ACME-Kontos (Kontakt-E-Mail).

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| GET | `/api/v1/acme-accounts` | Angemeldet | Liste |
| POST | `/api/v1/acme-accounts` | Angemeldet | Erstellen |
| DELETE | `/api/v1/acme-accounts/:id` | Angemeldet | Löschen |
| PUT | `/api/v1/acme-accounts/:id` | Angemeldet | Aktualisieren |
| GET | `/api/v1/acme-accounts/default` | Angemeldet | Standardwerte abrufen |

### Integrierte CA

Interne Panel-CA: Ausstellung/Erneuerung/Widerruf des Agentenzertifikats, CRL-Export, Verteilungswarteschlange.

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| GET | `/api/v1/ca/agent-certs` | Angemeldet | Liste der Agentenzertifikate |
| POST | `/api/v1/ca/agent-certs/:server_id/renew` | Angemeldet | Agentenzertifikat erneuern |
| GET | `/api/v1/ca/distribution-queue` | Angemeldet | Zertifikatverteilungswarteschlange |
| DELETE | `/api/v1/ca/distribution-queue/:id` | Angemeldet | Eintrag in der Verteilungswarteschlange löschen |
| POST | `/api/v1/ca/distribution-queue/:id/retry` | Angemeldet | Versuchen Sie die Zertifikatsverteilung erneut |
| GET | `/api/v1/ca/revocations` | Angemeldet | Widerrufsliste |
| GET | `/api/v1/ca/revocations/export` | Angemeldet | CRL exportieren |
| GET | `/api/v1/ca/status` | Angemeldet | CA-Status |

### Geo-Regeln

Synchronisierung und Bereitstellung von GeoIP-/Geosite-Datenquellen und -Dateien.

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| GET | `/api/v1/geo-rules/files` | Angemeldet | Dateiliste |
| DELETE | `/api/v1/geo-rules/files/:fileId` | Angemeldet | Datei löschen |
| PUT | `/api/v1/geo-rules/files/:fileId/auto-sync` | Angemeldet | Automatische Synchronisierung umschalten |
| POST | `/api/v1/geo-rules/files/:fileId/download` | Angemeldet | Herunterladen |
| GET | `/api/v1/geo-rules/files/:fileId/download-file` | Angemeldet | Datei herunterladen |
| GET | `/api/v1/geo-rules/github-assets` | Angemeldet | Rufen Sie GitHub-Repository-Assets ab |
| POST | `/api/v1/geo-rules/push-all` | Angemeldet | Push an alle Knoten |
| POST | `/api/v1/geo-rules/seed` | Angemeldet | Standarddaten initialisieren |
| GET | `/api/v1/geo-rules/sources` | Angemeldet | Liste der Datenquellen |
| POST | `/api/v1/geo-rules/sources` | Angemeldet | Datenquelle erstellen |
| DELETE | `/api/v1/geo-rules/sources/:sourceId` | Angemeldet | Datenquelle löschen |
| PUT | `/api/v1/geo-rules/sources/:sourceId` | Angemeldet | Datenquelle aktualisieren |
| GET | `/api/v1/geo-rules/sources/:sourceId/assets` | Angemeldet | Release-Assets abrufen |
| POST | `/api/v1/geo-rules/sources/:sourceId/files` | Angemeldet | Datei hinzufügen |
| PUT | `/api/v1/geo-rules/sources/:sourceId/schedule` | Angemeldet | Geplante Synchronisierung umschalten |
| POST | `/api/v1/geo-rules/sources/:sourceId/select` | Angemeldet | Datei auswählen |
| POST | `/api/v1/geo-rules/sources/:sourceId/sync` | Angemeldet | Synchronisieren Sie diese Datenquelle |
| GET | `/api/v1/geo-rules/status` | Angemeldet | Status |
| POST | `/api/v1/geo-rules/sync-all` | Angemeldet | Alles synchronisieren |
| GET | `/api/v1/geo-rules/sync-status/:key` | Angemeldet | Fortschritt synchronisieren |

### WAF Regelbibliothek

Globale WAF-Regeln: GitHub Quelle, Synchronisierung, Lieferung an alle Knoten.

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| GET | `/api/v1/waf/github-source` | Angemeldet | GitHub-Datenquelle |
| PUT | `/api/v1/waf/github-source` | Angemeldet | Speichern Sie die Datenquelle GitHub |
| POST | `/api/v1/waf/push-all` | Angemeldet | Push an alle Knoten |
| GET | `/api/v1/waf/rules` | Angemeldet | Regelliste |
| DELETE | `/api/v1/waf/rules/:name` | Angemeldet | Regel löschen |
| GET | `/api/v1/waf/rules/:name` | Angemeldet | Holen Sie sich die Herrschaft |
| PUT | `/api/v1/waf/rules/:name` | Angemeldet | Regel speichern |
| POST | `/api/v1/waf/sync-all` | Angemeldet | Alles synchronisieren |
| POST | `/api/v1/waf/sync-github` | Angemeldet | Synchronisierung von GitHub |

### Protokollverwaltung

Lesen und Streamen von Knoten-/Kernprotokollen, Bereinigungsrichtlinien, Protokollschreibersteuerung.

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| POST | `/api/v1/log-manage/cleanup-all` | Angemeldet | Jetzt alles aufräumen |
| GET | `/api/v1/log-manage/cleanup-rules` | Angemeldet | Liste der Bereinigungsregeln |
| PUT | `/api/v1/log-manage/cleanup-rules/:id` | Angemeldet | Bereinigungsregel aktualisieren |
| POST | `/api/v1/log-manage/cleanup-table/:table` | Angemeldet | Bereinigen Sie jetzt die angegebene Tabelle |
| GET | `/api/v1/log-manage/core-log-output` | Angemeldet | Konfiguration der Kernprotokollausgabe |
| PUT | `/api/v1/log-manage/core-log-output` | Angemeldet | Speichern Sie die Konfiguration der Kernprotokollausgabe |
| GET | `/api/v1/log-manage/core/:type/stdout-log` | Angemeldet | Kern-Stdout-Protokolle lesen |
| GET | `/api/v1/log-manage/core/:type/stdout-stream` | Angemeldet | Stream-Core-Standardausgabe |
| POST | `/api/v1/log-manage/discover` | Angemeldet | Ermitteln Sie Knotenprotokolldateien |
| GET | `/api/v1/log-manage/log-output` | Angemeldet | Protokollausgabekonfiguration |
| PUT | `/api/v1/log-manage/log-output` | Angemeldet | Protokollausgabekonfiguration speichern |
| POST | `/api/v1/log-manage/log-output/release/:type` | Angemeldet | Veröffentlichungsprotokoll zur Stdout-Übernahme |
| POST | `/api/v1/log-manage/log-output/takeover/:type` | Angemeldet | Übernehmen Sie log stdout |
| GET | `/api/v1/log-manage/nodes/:serverId/logs` | Angemeldet | Liste der Knotenprotokolldateien |
| GET | `/api/v1/log-manage/nodes/:serverId/logs/:type` | Angemeldet | Knotenprotokolle lesen |
| POST | `/api/v1/log-manage/nodes/:serverId/logs/:type/rotate` | Angemeldet | Knotenprotokolle rotieren |
| GET | `/api/v1/log-manage/nodes/:serverId/logs/:type/stream` | Angemeldet | Knotenprotokolle streamen |
| GET | `/api/v1/log-manage/policies` | Angemeldet | Richtlinienliste |
| POST | `/api/v1/log-manage/policies` | Angemeldet | Richtlinie erstellen |
| DELETE | `/api/v1/log-manage/policies/:id` | Angemeldet | Richtlinie löschen |
| GET | `/api/v1/log-manage/policies/:id` | Angemeldet | Richtliniendetails |
| PUT | `/api/v1/log-manage/policies/:id` | Angemeldet | Richtlinie aktualisieren |
| GET | `/api/v1/log-manage/policies/:id/jobs` | Angemeldet | Liste der Aufräumarbeiten |
| GET | `/api/v1/log-manage/policies/:id/log` | Angemeldet | Protokolle lesen |
| GET | `/api/v1/log-manage/policies/:id/log-stream` | Angemeldet | Stream-Protokolle |
| POST | `/api/v1/log-manage/policies/:id/rotate` | Angemeldet | Drehen |
| GET | `/api/v1/log-manage/proxy/:type/stdout-log` | Angemeldet | Kern-Stdout-Protokolle lesen |
| GET | `/api/v1/log-manage/proxy/:type/stdout-stream` | Angemeldet | Stream-Core-Standardausgabe |
| GET | `/api/v1/log-manage/scheduler-config` | Angemeldet | Planungskonfiguration |
| PUT | `/api/v1/log-manage/scheduler-config` | Angemeldet | Planungskonfiguration aktualisieren |
| GET | `/api/v1/log-manage/status` | Angemeldet | Status |
| POST | `/api/v1/log-manage/sync` | Angemeldet | Bereinigungsrichtlinien synchronisieren |
| GET | `/api/v1/log-manage/writers` | Angemeldet | Autorenstatus |
| PUT | `/api/v1/log-manage/writers/:type/config` | Angemeldet | Writer-Konfiguration aktualisieren |
| GET | `/api/v1/log-manage/writers/:type/files` | Angemeldet | Liste der Writer-Protokolldateien |
| POST | `/api/v1/log-manage/writers/:type/start` | Angemeldet | Beginnen Sie mit dem Schreiben |
| POST | `/api/v1/log-manage/writers/:type/stop` | Angemeldet | Hör auf zu schreiben |

### Backups

Datenbank-/Vollsicherungen, Wiederherstellung, geplante Jobs, FTP/S3/WebDAV Remote-Speicher.

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| DELETE | `/api/v1/backups/:id` | Admin | Sicherung löschen |
| GET | `/api/v1/backups/:id/download` | Admin | Backup herunterladen |
| POST | `/api/v1/backups/:id/restore` | Admin | Aus Backup wiederherstellen |
| POST | `/api/v1/backups/:id/upload-ftp` | Admin | Hochladen auf FTP |
| POST | `/api/v1/backups/:id/upload-s3` | Admin | Hochladen auf S3 |
| POST | `/api/v1/backups/:id/upload-webdav` | Admin | Hochladen auf WebDAV |
| POST | `/api/v1/backups/create` | Admin | Vollständiges Backup erstellen (mit DB-Dump) |
| GET | `/api/v1/backups/list` | Admin | Vollständige Backup-Liste |
| POST | `/api/v1/backups/remote-restore` | Admin | Wiederherstellung per Fernzugriff |
| GET | `/api/v1/backups/restore-status` | Admin | Fortschritt wiederherstellen |
| GET | `/api/v1/backups/schedules` | Admin | Liste der geplanten Jobs |
| POST | `/api/v1/backups/schedules` | Admin | Geplanten Job erstellen |
| DELETE | `/api/v1/backups/schedules/:id` | Admin | Geplanten Job löschen |
| PUT | `/api/v1/backups/schedules/:id` | Admin | Geplanten Auftrag aktualisieren |
| GET | `/api/v1/backups/storages` | Admin | Speicherkonfigurationsliste |
| POST | `/api/v1/backups/storages` | Admin | Speicherkonfiguration erstellen |
| DELETE | `/api/v1/backups/storages/:id` | Admin | Speicherkonfiguration löschen |
| PUT | `/api/v1/backups/storages/:id` | Admin | Speicherkonfiguration aktualisieren |
| GET | `/api/v1/backups/storages/:id/files` | Admin | Liste der Remote-Backup-Dateien |
| POST | `/api/v1/backups/upload-restore` | Admin | Backup hochladen und wiederherstellen |

### Systemeinstellungen

Datenbankkonfiguration und -migration, Panel-Einstellungen, JWT-Schlüssel, SMTP/Telegram-Tests, Benachrichtigungsvorlagen, Online-Upgrades (hauptsächlich Administrator).

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| PUT | `/api/v1/system/bootstrap-finish` | Admin | Schließen Sie das Erststart-Setup ab |
| GET | `/api/v1/system/bootstrap-state` | Admin | Erststartstatus |
| GET | `/api/v1/system/config-file/:coreType` | Admin | Kernkonfigurationsdatei lesen |
| GET | `/api/v1/system/db-config` | Admin | Datenbankkonfiguration |
| PUT | `/api/v1/system/db-config` | Admin | Datenbankkonfiguration aktualisieren |
| POST | `/api/v1/system/db-config/migrate` | Admin | Datenbankdaten migrieren |
| POST | `/api/v1/system/db-config/switch` | Admin | Datenbank wechseln (mit einstufiger Migration) |
| POST | `/api/v1/system/db-config/test` | Admin | Testen Sie die Datenbankverbindung |
| GET | `/api/v1/system/db-pool-stats` | Admin | Statistiken zum Verbindungspool |
| GET | `/api/v1/system/db-stats` | Admin | Datenbankstatistiken |
| GET | `/api/v1/system/db-stats/ping` | Admin | Datenbanksonde |
| POST | `/api/v1/system/db-stats/reset` | Admin | Datenbankstatistiken zurücksetzen |
| GET | `/api/v1/system/encryption-key-status` | Admin | Status des Verschlüsselungsschlüssels |
| GET | `/api/v1/system/env-config` | Admin | .env-Konfiguration |
| PUT | `/api/v1/system/env-config` | Admin | Aktualisieren Sie die .env-Konfiguration |
| POST | `/api/v1/system/generate-encryption-key` | Admin | Verschlüsselungsschlüssel generieren |
| GET | `/api/v1/system/github-config` | Admin | GitHub-Konfiguration |
| PUT | `/api/v1/system/github-config` | Admin | Aktualisieren Sie die GitHub-Konfiguration |
| GET | `/api/v1/system/jwt-secret-status` | Admin | JWT Schlüsselstatus |
| GET | `/api/v1/system/maintenance` | Öffentlich | Status des Wartungsmodus (öffentlich) |
| PUT | `/api/v1/system/maintenance` | Admin | Wartungsmodus umschalten |
| POST | `/api/v1/system/migrate-data` | Admin | Daten migrieren |
| GET | `/api/v1/system/notification-config` | Admin | Benachrichtigungskonfiguration |
| PUT | `/api/v1/system/notification-config` | Admin | Benachrichtigungskonfiguration aktualisieren |
| GET | `/api/v1/system/notification-templates` | Admin | Vorlagenliste |
| POST | `/api/v1/system/notification-templates` | Admin | Vorlage erstellen |
| DELETE | `/api/v1/system/notification-templates/:id` | Admin | Vorlage löschen |
| PUT | `/api/v1/system/notification-templates/:id` | Admin | Vorlage aktualisieren |
| POST | `/api/v1/system/notification-templates/:id/broadcast` | Admin | Broadcast-Benachrichtigung per Vorlage |
| POST | `/api/v1/system/notification-templates/:id/send` | Admin | Testbenachrichtigung senden |
| GET | `/api/v1/system/panel-config` | Admin | Panel-Konfiguration |
| PUT | `/api/v1/system/panel-config` | Admin | Panel-Konfiguration aktualisieren |
| GET | `/api/v1/system/reality-targets` | Admin | REALITY Zielliste |
| PUT | `/api/v1/system/reality-targets` | Admin | Ziel REALITY aktualisieren |
| POST | `/api/v1/system/reality-targets/reset` | Admin | REALITY-Ziel auf Standard zurücksetzen |
| POST | `/api/v1/system/regenerate-jwt-secret` | Admin | JWT-Schlüssel neu generieren |
| GET | `/api/v1/system/releases` | Admin | Verfügbare Versionen |
| POST | `/api/v1/system/restart` | Admin | Neustart |
| POST | `/api/v1/system/rollback` | Admin | System-Rollback |
| GET | `/api/v1/system/settings` | Admin | Einstellungen lesen |
| PUT | `/api/v1/system/settings/batch` | Admin | Massenaktualisierungseinstellungen |
| GET | `/api/v1/system/stats` | Admin | Systemstatus |
| GET | `/api/v1/system/subscription-billing-templates` | Admin | Vorlage für die Abonnementabrechnung |
| PUT | `/api/v1/system/subscription-billing-templates` | Admin | Aktualisieren Sie die Abrechnungsvorlage für das Abonnement |
| POST | `/api/v1/system/telegram-chat-id` | Admin | Holen Sie sich die Chat-ID Telegram |
| POST | `/api/v1/system/test-smtp` | Admin | Testen Sie SMTP |
| POST | `/api/v1/system/test-telegram` | Admin | Testen Sie Telegram |
| POST | `/api/v1/system/update` | Admin | System-Upgrade |
| GET | `/api/v1/system/update-jobs` | Admin | Jobliste aktualisieren |
| POST | `/api/v1/system/upload-mtls-ca` | Admin | Laden Sie das mTLS-CA-Zertifikat hoch |

### Sicherheitsaudit

Audit-Ereignisabfragen und -Export.

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| GET | `/api/v1/security/audit-events` | Angemeldet | Audit-Ereignisliste |
| GET | `/api/v1/security/audit-events/export` | Angemeldet | Prüfereignisse exportieren |

### Metriken

Panel-aggregierte Metriken (Knoten/Zertifikate/Dienste).

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| GET | `/api/v1/metrics/certificates` | Angemeldet | Zertifikatsmetriken |
| GET | `/api/v1/metrics/nodes` | Angemeldet | Knotenmetriken |
| GET | `/api/v1/metrics/overview` | Angemeldet | Übersicht |

### Portalvorlagen

Portalvorlagen, Aktivierungskonfiguration und Cache-Verwaltung.

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| GET | `/api/v1/portal/active` | Angemeldet | Aktive Vorlage |
| PUT | `/api/v1/portal/active` | Angemeldet | Aktive Vorlage aktualisieren |
| DELETE | `/api/v1/portal/cache` | Angemeldet | Leeren Sie den Portal-Cache |
| GET | `/api/v1/portal/cache` | Angemeldet | Cache-Status |
| PUT | `/api/v1/portal/cache/dev-mode` | Angemeldet | Cache-Entwicklungsmodus umschalten |
| POST | `/api/v1/portal/cache/rebuild` | Angemeldet | Portal-Cache neu erstellen |
| POST | `/api/v1/portal/reset` | Angemeldet | Standardvorlage wiederherstellen |
| GET | `/api/v1/portal/templates` | Angemeldet | Liste |
| POST | `/api/v1/portal/templates` | Angemeldet | Erstellen |
| DELETE | `/api/v1/portal/templates/:id` | Angemeldet | Löschen |
| GET | `/api/v1/portal/templates/:id` | Angemeldet | Einzelheiten |
| PUT | `/api/v1/portal/templates/:id` | Angemeldet | Aktualisieren |
| POST | `/api/v1/portal/templates/:id/activate` | Angemeldet | Vorlage aktivieren |

> `/portal/templates` CRUD ist ein **erweitertes API**: Die Portal-Benutzeroberfläche verwendet hauptsächlich `PUT /api/v1/portal/active`
> (One-Shot-Konfiguration für die gesamte Site); Mit dieser Gruppe mehrere Vorlagen verwalten/wechseln. Beide werden unterstützt und stehen nicht in Konflikt.

### Agenteninstallation

Node Agent-Installationsskript und Download (öffentlich).

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| POST | `/api/v1/agent/bootstrap` | Öffentlich | Agent-First-Boot-Registrierung (Agent-Berichte mit einem Bootstrap-Token; öffentlich) |
| GET | `/api/v1/agent/releases` | Admin | Verfügbare Agent-Versionen (für die Upgrade-Seite) |
| GET | `/api/v1/agent/version` | Admin | Aktuelle Agentenversion auf dem Panel |
| GET | `/api/v1/agent/download` | Öffentlich | Laden Sie den Agenten herunter |
| GET | `/api/v1/agent/install` | Öffentlich | Holen Sie sich das Installationsskript |

### Kostenpflichtiger Rückrufeintrag

Rückruf einer Zahlungsplattform eines Drittanbieters (öffentlich, Signatur überprüft).

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| POST | `/api/v1/charge/callback/:site_id` | Öffentlich | Behandeln Sie den kostenpflichtigen Rückruf |

### Eingehend (mit Knoten)

Eingehende Liste, die für die Auswahl des Planknotens verwendet wird.

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| GET | `/api/v1/inbounds-with-server` | Angemeldet | Alle eingehenden Daten (einschließlich ihrer Knoten) |

### Öffentliche Konfiguration

Portal-Laufzeitkonfiguration ohne Anmeldung (Wartungsschutz + öffentliche Ratenbegrenzung).

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| GET | `/api/v1/public/articles` | Öffentlich | Öffentliche Artikelliste |
| GET | `/api/v1/public/articles/:slug` | Öffentlich | Details zum öffentlichen Artikel |
| GET | `/api/v1/public/client-configs` | Öffentlich | Client-Download-Konfiguration (öffentlich) |
| GET | `/api/v1/public/portal-config` | Öffentlich | Portal-Laufzeitkonfiguration (Site-Name, Thema, Sprachen, Compliance-Dokumente; öffentlich) |
| GET | `/api/v1/public/settings` | Öffentlich | Öffentliche Einstellungen (z. B. Registration_Enabled; Public) |
| GET | `/api/v1/public/subscription-endpoints` | Öffentlich | Liste der aktivierten Abonnementendpunkte (öffentlich) |

### Client-Authentifizierung und Geschäft (gleiches Set auf der Admin-Engine)

Client-Endpunkte identisch mit dem Portal; Weitere Informationen finden Sie unter `portal-api.en.md`.

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| GET | `/api/v1/client-auth/articles` | Öffentlich | Artikel auflisten |
| GET | `/api/v1/client-auth/articles/:slug` | Öffentlich | Artikel abrufen |
| GET | `/api/v1/client-auth/balance` | Öffentlich | Schaffen Sie Balance |
| GET | `/api/v1/client-auth/balance/transactions` | Öffentlich | Saldotransaktionen |
| GET | `/api/v1/client-auth/bills` | Öffentlich | Bestellliste |
| POST | `/api/v1/client-auth/change-password` | Öffentlich | Passwort ändern |
| POST | `/api/v1/client-auth/charge/create` | Öffentlich | Aufladeauftrag erstellen |
| GET | `/api/v1/client-auth/charge/orders` | Öffentlich | Aufladeaufträge auflisten |
| GET | `/api/v1/client-auth/charge/orders/:id` | Öffentlich | Erhalten Sie Details zur Aufladebestellung |
| POST | `/api/v1/client-auth/charge/redeem-card` | Öffentlich | Karte einlösen |
| GET | `/api/v1/client-auth/charge/sites` | Öffentlich | Aufladestellen auflisten |
| GET | `/api/v1/client-auth/charge/status/:trade_no` | Öffentlich | Ladestatus abfragen |
| GET | `/api/v1/client-auth/checktime` | Öffentlich | Serverzeit abrufen (Uhrsynchronisierung) |
| GET | `/api/v1/client-auth/connection-info` | Öffentlich | Planverbindungsparameter abrufen |
| GET | `/api/v1/client-auth/gifts` | Öffentlich | Geschenke auflisten |
| POST | `/api/v1/client-auth/gifts/:id/claim` | Öffentlich | Geschenk beanspruchen |
| GET | `/api/v1/client-auth/invoices` | Öffentlich | Bestellliste |
| GET | `/api/v1/client-auth/invoices/:id` | Öffentlich | Bestelldetails |
| GET | `/api/v1/client-auth/ip-whitelist` | Öffentlich | IP-Zulassungsliste auflisten |
| DELETE | `/api/v1/client-auth/ip-whitelist/:id` | Öffentlich | Von der IP-Zulassungsliste entfernen |
| POST | `/api/v1/client-auth/ip-whitelist/auto-add` | Öffentlich | IP automatisch hinzufügen |
| POST | `/api/v1/client-auth/login` | Öffentlich | Melden Sie sich an |
| POST | `/api/v1/client-auth/logout` | Öffentlich | Abmelden |
| POST | `/api/v1/client-auth/logs/package` | Öffentlich | Erstellen Sie ein Protokollpaket |
| GET | `/api/v1/client-auth/logs/packages` | Öffentlich | Protokollpakete auflisten |
| DELETE | `/api/v1/client-auth/logs/packages/:id` | Öffentlich | Protokollbündel löschen |
| GET | `/api/v1/client-auth/logs/packages/:id` | Öffentlich | Holen Sie sich das Protokollpaket |
| GET | `/api/v1/client-auth/logs/packages/:id/download` | Öffentlich | Protokollpaket herunterladen |
| GET | `/api/v1/client-auth/logs/types` | Öffentlich | Zustandsprüfung des Protokollpakets |
| GET | `/api/v1/client-auth/me` | Öffentlich | Aktuellen angemeldeten Benutzer abrufen |
| GET | `/api/v1/client-auth/nodes` | Öffentlich | Meine Knotenliste |
| GET | `/api/v1/client-auth/nodes/:id/config` | Öffentlich | Knotenkonfiguration |
| GET | `/api/v1/client-auth/nodes/:id/link` | Öffentlich | Link zur Knotenfreigabe |
| GET | `/api/v1/client-auth/nodes/:id/qrcode` | Öffentlich | Knoten-QR-Code |
| GET | `/api/v1/client-auth/oauth/:provider` | Öffentlich | Autorisieren |
| GET | `/api/v1/client-auth/oauth/:provider/callback` | Öffentlich | Rückruf |
| GET | `/api/v1/client-auth/oauth/providers` | Öffentlich | Anbieterstatus |
| GET | `/api/v1/client-auth/orders` | Öffentlich | Bestellliste |
| POST | `/api/v1/client-auth/orders` | Öffentlich | Ordnung schaffen |
| GET | `/api/v1/client-auth/orders/:id` | Öffentlich | Bestelldetails |
| GET | `/api/v1/client-auth/plans` | Öffentlich | Planliste |
| GET | `/api/v1/client-auth/plans/:id` | Öffentlich | Plandetails |
| GET | `/api/v1/client-auth/profile` | Öffentlich | Holen Sie sich Profil |
| PUT | `/api/v1/client-auth/profile` | Öffentlich | Profil aktualisieren |
| POST | `/api/v1/client-auth/profile/bind-oauth` | Öffentlich | Drittanbieterkonto binden |
| DELETE | `/api/v1/client-auth/profile/unbind-oauth/:provider` | Öffentlich | Bindung des Drittanbieterkontos aufheben |
| GET | `/api/v1/client-auth/referral/code` | Öffentlich | Holen Sie sich den Bestätigungscode für die Empfehlung |
| GET | `/api/v1/client-auth/referral/records` | Öffentlich | Empfehlungsdatensätze auflisten |
| GET | `/api/v1/client-auth/referral/stats` | Öffentlich | Erhalten Sie Empfehlungsstatistiken |
| POST | `/api/v1/client-auth/referral/withdraw` | Öffentlich | Auszahlung beantragen |
| GET | `/api/v1/client-auth/referral/withdrawals` | Öffentlich | Empfehlungsabhebungen auflisten |
| POST | `/api/v1/client-auth/refresh` | Öffentlich | Aktualisieren |
| POST | `/api/v1/client-auth/register` | Öffentlich | Registrieren |
| POST | `/api/v1/client-auth/reset-subscription` | Öffentlich | Abonnement zurücksetzen |
| POST | `/api/v1/client-auth/rotate-token` | Öffentlich | Abonnement-Token drehen |
| POST | `/api/v1/client-auth/self-unban` | Öffentlich | Selbstbedienungsverbot aufgehoben |
| PUT | `/api/v1/client-auth/settings` | Öffentlich | Einstellungen aktualisieren |
| POST | `/api/v1/client-auth/settings/auto-add-ip` | Öffentlich | IP automatisch hinzufügen |
| GET | `/api/v1/client-auth/settings/whitelist` | Öffentlich | IP-Zulassungsliste auflisten |
| DELETE | `/api/v1/client-auth/settings/whitelist/:id` | Öffentlich | Von der IP-Zulassungsliste entfernen |
| GET | `/api/v1/client-auth/shared-accounts` | Öffentlich | Gemeinsame Konten auflisten |
| GET | `/api/v1/client-auth/shared-accounts/:id` | Öffentlich | Holen Sie sich ein gemeinsames Konto |
| POST | `/api/v1/client-auth/shared-accounts/:id/apply` | Öffentlich | Gemeinsames Konto anfordern |
| POST | `/api/v1/client-auth/shared-accounts/:id/repair` | Öffentlich | Reparaturanfrage |
| POST | `/api/v1/client-auth/ticket` | Öffentlich | Einmaliges, kurzlebiges Ticket ausstellen |
| GET | `/api/v1/client-auth/tickets` | Öffentlich | Tickets auflisten |
| POST | `/api/v1/client-auth/tickets` | Öffentlich | Ticket erstellen |
| GET | `/api/v1/client-auth/tickets/:id` | Öffentlich | Ticket besorgen |
| POST | `/api/v1/client-auth/tickets/:id/close` | Öffentlich | Ticket schließen |
| POST | `/api/v1/client-auth/tickets/:id/reply` | Öffentlich | Auf Ticket antworten |
| GET | `/api/v1/client-auth/traffic-history` | Öffentlich | Verkehrsgeschichte |
| GET | `/api/v1/client-auth/traffic-packs` | Öffentlich | Traffic-Packliste |
| POST | `/api/v1/client-auth/traffic-packs/buy` | Öffentlich | Verkehrspaket kaufen |

### Abonnementlieferung

Öffentlicher Einstiegspunkt für Kunden zum Abrufen von Abonnements; siehe `subscription.en.md`.

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| GET | `/sub/:token` | Öffentlich | Abonnement servieren |

### Workflow für die Laufzeitkonfiguration

Konfigurations-Snapshots / Probelauf / Anwenden / Rollback (`apply`, `rollback` und `disable-proxy` erfordern Administrator).

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| POST | `/runtime/apply` | Öffentlich | Bewerben |
| GET | `/runtime/apply-jobs` | Angemeldet | Jobs auflisten |
| GET | `/runtime/apply-jobs/:id` | Angemeldet | Job bekommen |
| POST | `/runtime/apply-jobs/:id/cancel` | Öffentlich | Auftrag abbrechen |
| POST | `/runtime/apply-jobs/:id/retry` | Öffentlich | Auftrag wiederholen |
| POST | `/runtime/disable-proxy` | Öffentlich | Proxy deaktivieren |
| POST | `/runtime/dry-run` | Angemeldet | Probelaufausführung |
| GET | `/runtime/nodes` | Angemeldet | Knoten auflisten |
| GET | `/runtime/overview` | Angemeldet | Übersicht |
| POST | `/runtime/rollback` | Öffentlich | Zurückrollen |
| GET | `/runtime/snapshots` | Angemeldet | Schnappschüsse auflisten |
| POST | `/runtime/snapshots` | Angemeldet | Schnappschuss erstellen |
| DELETE | `/runtime/snapshots/:id` | Öffentlich | Snapshot löschen |
| GET | `/runtime/snapshots/:id` | Angemeldet | Holen Sie sich einen Schnappschuss |
| POST | `/runtime/verify` | Angemeldet | Überprüfen |

### Dienstzugriffsrichtlinien (eingestellt)

> `/services/:id/access-policies` und `/services/:id/ip-bans` wurden am 20.10.2026 außer Dienst gestellt:
> Es gibt keine Administratoraufrufe und kein Verbindungspfad nutzt diese Richtlinien.

### Überwachung der Verkehrsquote

Schreibgeschützte Überwachungsansicht von Kontingenten und Nutzung.

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| GET | `/traffic/quotas` | Angemeldet | Quotenliste |
| GET | `/traffic/quotas/:id/usage` | Angemeldet | Kontingentnutzung |
| GET | `/traffic/reset-logs` | Angemeldet | Datensätze zum Zurücksetzen des Kontingents |
| GET | `/traffic/usage-history` | Angemeldet | Nutzungshistorie |

### Gesundheitschecks

Lebendigkeitssonde.

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| GET | `/health` | Öffentlich | Lebendigkeitssonde; 200 bedeutet gesund |

### Portalseite

Portal-SPA-Seite.

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| GET | `/p` | Öffentlich | Portal-HTML (vom Server gerendert + Vue-Laufzeit) |

### ACME Herausforderungen

HTTP-01 Challenge-Rückruf.

| Methode | Pfad | Erlaubnis | Beschreibung |
|---|---|---|---|
| GET | `/.well-known/acme-challenge/:token` | Öffentlich | ACME HTTP-01 Challenge-Callback (überprüft die Domain während der Ausstellung) |
