<!-- source: README.md | locale: en | updated: 2026-10-23 -->
# CoreFusion API Documentation

> Languages: [简体中文](README.md) · English · [繁體中文](README.zh-TW.md) · [Русский](README.ru.md) · [日本語](README.ja.md) · [한국어](README.ko.md) · [Deutsch](README.de.md) · [Français](README.fr.md) · [Español](README.es.md) · [Português](README.pt.md) · [Türkçe](README.tr.md) · [Bahasa Indonesia](README.id.md) · [ไทย](README.th.md) · [Tiếng Việt](README.vi.md) · [العربية](README.ar.md) · [فارسی](README.fa.md)

API documentation for **external integrators, secondary development and third-party clients**. All paths are relative to the deployment domain.

| Document | Audience | Content |
|---|---|---|
| [subscription.md](./subscription.en.md) | Subscribers / client developers | Subscription endpoint (`/sub/:token`): formats, headers, status codes, examples |
| [portal-api.md](./portal-api.md) | Portal frontend / third-party clients | Client portal API (`/api/v1/client-auth/*`, `/api/v1/public/*`): sign-up, sign-in, plans, orders, balance, tickets |
| [charge-callback.md](./charge-callback.en.md) | Payment providers | Payment callback signing and verification |
| [admin-api.en.md](./admin-api.en.md) | Panel development / automation | Full admin API route table (`/api/v1/*`, grouped by module) |

---

## 1. Ports and base URLs

The panel can expose three engines at the same time, sharing one database:

| Engine | Default access | Purpose |
|---|---|---|
| Main panel | `https://<domain>/api/v1` | Admin backend + portal + subscriptions |
| Subscription engine (dedicated port) | `https://<domain>:<port>/:token` | High-concurrency subscription fetch, no `/sub` prefix |
| Portal engine (dedicated port) | `https://<portal-domain>[:port]/api/v1` | Client-only portal, pages at `/` |

> The reverse proxy must forward both page paths and `/api/v1`; `X-Forwarded-Proto` / `X-Forwarded-Host` are honored to restore the real scheme and domain.

## 2. Authentication (three kinds, not interchangeable)

| Auth | Credential | Applies to |
|---|---|---|
| **Admin auth** | `Authorization: Bearer <admin_token>` (from `/api/v1/auth/login`) | Admin API |
| **Client auth** | `Authorization: Bearer <client_token>` (from `/api/v1/client-auth/login`) | Portal API |
| **Subscription token** | Token in the URL: `/sub/<token>` | Subscription fetch, no header |

- Client access tokens last 24h, refresh tokens 7 days; the JWT contains `tv` (token_version), so logout / forced sign-out invalidates all old tokens.
- Admin roles: `admin` / `operator` / `super_admin`; some write operations require `RequireRole(RoleAdmin)` or higher.

## 3. Response format

Success (most endpoints):

```json
{ "code": 0, "data": { ... }, "message": "ok" }
```

List endpoints (some) return `{ "items": [...], "total": N }` or a bare array (noted per endpoint below).

Errors (HTTP 4xx/5xx):

```json
{ "code": "ERROR_CODE", "message": "human-readable description", "request_id": "…", "can_retry": false }
```

**Amounts are integers in cents**: `990` = ¥9.90 (`currency: "CNY"`).
**Traffic is in bytes**: requests and responses both use bytes.

## 4. Rate limits and maintenance mode

- Public endpoints (`/api/v1/public/*`, `/sub/*`, sign-up/sign-in) are rate limited;
- Sign-in also uses `LoginRateLimit`, subscription fetch uses `SubscribeRateLimit`;
- In maintenance mode public endpoints and subscriptions return a maintenance message; the admin API is unaffected.

## 5. Frontend page routes (not API)

These routes return HTML/static assets and are outside the API docs; listed so they can be excluded during integration:

| Method | Path | Description |
|---|---|---|
| GET | `/` | Admin panel SPA (after sign-in) |
| GET | `/p` | Portal HTML (`/` on a dedicated portal engine) |
| GET | `/:token` | Subscription fetch on a dedicated port (no `/sub` prefix, same semantics as `/sub/:token`) |
| GET | `/static/:file` | Frontend static assets |

## 6. Source of truth

`internal/api/router/router.go` (746 routes, 743 documented). The route table in [admin-api.md](./admin-api.md) is generated from code; if it disagrees with the code, **the code wins**.
