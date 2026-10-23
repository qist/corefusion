<!-- source: README.md | locale: de | updated: 2026-10-23 -->
# CoreFusion API Dokumentation

> Sprachen: [简体中文](README.md) · [English](README.en.md) · [繁體中文](README.zh-TW.md) · [Русский](README.ru.md) · [日本語](README.ja.md) · [한국어](README.ko.md) · Deutsch · [Français](README.fr.md) · [Español](README.es.md) · [Português](README.pt.md) · [Türkçe](README.tr.md) · [Bahasa Indonesia](README.id.md) · [ไทย](README.th.md) · [Tiếng Việt](README.vi.md) · [العربية](README.ar.md) · [فارسی](README.fa.md)

API-Dokumentation für **externe Integratoren, Sekundärentwicklung und Drittanbieter-Clients**. Alle Pfade beziehen sich auf die Bereitstellungsdomäne.

| Dokument | Publikum | Inhalt |
|---|---|---|
| [subscription.md](./subscription.de.md) | Abonnenten/Client-Entwickler | Abonnementendpunkt (`/sub/:token`): Formate, Header, Statuscodes, Beispiele |
| [portal-api.md](README.de.md) | Portal-Frontend / Drittanbieter-Clients | Kundenportal API (`/api/v1/client-auth/*`, `/api/v1/public/*`): Anmeldung, Anmeldung, Pläne, Bestellungen, Guthaben, Tickets |
| [charge-callback.md](./charge-callback.de.md) | Zahlungsanbieter | Signieren und Überprüfen des Zahlungsrückrufs |
| [admin-api.de.md](./admin-api.de.md) | Panelentwicklung / Automatisierung | Vollständige Admin-API-Routentabelle (`/api/v1/*`, gruppiert nach Modul) |

---

## 1. Ports und Basis-URLs

Das Panel kann drei Engines gleichzeitig verfügbar machen und eine Datenbank gemeinsam nutzen:

| Motor | Standardzugriff | Zweck |
|---|---|---|
| Hauptpanel | `https://<domain>/api/v1` | Admin-Backend + Portal + Abonnements |
| Abonnement-Engine (dedizierter Port) | `https://<domain>:<port>/:token` | Abonnementabruf mit hoher Parallelität, kein `/sub`-Präfix |
| Portal-Engine (dedizierter Port) | `https://<portal-domain>[:port]/api/v1` | Nur-Client-Portal, Seiten unter `/` |

> Der Reverse-Proxy muss beide Seitenpfade und `/api/v1` weiterleiten; `X-Forwarded-Proto` / `X-Forwarded-Host` fühlen sich geehrt, das echte Schema und die Domäne wiederherzustellen.

## 2. Authentifizierung (drei Arten, nicht austauschbar)

| Auth | Ausweis | Gilt für |
|---|---|---|
| **Administrator-Authentifizierung** | `Authorization: Bearer <admin_token>` (von `/api/v1/auth/login`) | Admin API |
| **Client-Authentifizierung** | `Authorization: Bearer <client_token>` (von `/api/v1/client-auth/login`) | Portal API |
| **Abonnement-Token** | Token im URL: `/sub/<token>` | Abonnementabruf, kein Header |

- Client-Zugriffstokens halten 24 Stunden, Aktualisierungstokens 7 Tage; Der JWT enthält `tv` (token_version), sodass beim Abmelden/erzwungenen Abmelden alle alten Token ungültig werden.
- Administratorrollen: `admin` / `operator` / `super_admin`; Für einige Schreibvorgänge ist `RequireRole(RoleAdmin)` oder höher erforderlich.

## 3. Antwortformat

Erfolg (die meisten Endpunkte):

```json
{ "code": 0, "data": { ... }, "message": "ok" }
```

Listenendpunkte (einige) geben `{ "items": [...], "total": N }` oder ein bloßes Array zurück (unten pro Endpunkt angegeben).

Fehler (HTTP 4xx/5xx):

```json
{ "code": "ERROR_CODE", "message": "human-readable description", "request_id": "…", "can_retry": false }
```

**Beträge sind ganze Zahlen in Cent**: `990` = ¥9,90 (`currency: "CNY"`).
**Der Datenverkehr erfolgt in Bytes**: Sowohl Anfragen als auch Antworten verwenden Bytes.

## 4. Ratenbegrenzungen und Wartungsmodus

- Öffentliche Endpunkte (`/api/v1/public/*`, `/sub/*`, Anmeldung/Anmeldung) unterliegen einer Ratenbegrenzung;
- Für die Anmeldung wird ebenfalls `LoginRateLimit` verwendet, für den Abonnementabruf wird `SubscribeRateLimit` verwendet.
- Im Wartungsmodus geben öffentliche Endpunkte und Abonnements eine Wartungsmeldung zurück; Der Administrator API ist davon nicht betroffen.

## 5. Frontend-Seitenrouten (nicht API)

Diese Routen geben HTML/statische Assets zurück und liegen außerhalb der API-Dokumente; aufgelistet, damit sie bei der Integration ausgeschlossen werden können:

| Methode | Pfad | Beschreibung |
|---|---|---|
| GET | `/` | Admin-Panel SPA (nach der Anmeldung) |
| GET | `/p` | Portal-HTML (`/` auf einer dedizierten Portal-Engine) |
| GET | `/:token` | Abonnementabruf über einen dedizierten Port (kein `/sub`-Präfix, gleiche Semantik wie `/sub/:token`) |
| GET | `/static/:file` | Statische Frontend-Assets |

## 6. Quelle der Wahrheit

`internal/api/router/router.go` (746 Routen, 743 dokumentiert). Die Routentabelle in [admin-api.md](README.de.md) wird aus Code generiert; Wenn es mit dem Code nicht übereinstimmt, **gewinnt der Code**.
