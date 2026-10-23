<!-- source: portal-api.md | locale: en | updated: 2026-10-23 -->
# Portal API Reference (Client / Portal API)

> Languages: [简体中文](portal-api.md) · English · [繁體中文](portal-api.zh-TW.md) · [Русский](portal-api.ru.md) · [日本語](portal-api.ja.md) · [한국어](portal-api.ko.md) · [Deutsch](portal-api.de.md) · [Français](portal-api.fr.md) · [Español](portal-api.es.md) · [Português](portal-api.pt.md) · [Türkçe](portal-api.tr.md) · [Bahasa Indonesia](portal-api.id.md) · [ไทย](portal-api.th.md) · [Tiếng Việt](portal-api.vi.md) · [العربية](portal-api.ar.md) · [فارسی](portal-api.fa.md)

> The **complete** interface reference for the portal frontend (`/p` or a dedicated portal port) and third-party clients.
> Route definitions: `internal/api/router/router.go` (`clientAuthGroup` / `clientAuthProtected` / `/api/v1/public/*`).

## 1. General conventions

### 1.1 Base URL

| Deployment | Base URL |
|---|---|
| Main panel engine | `https://<panel-domain>/api/v1` (portal page at `/p`) |
| Dedicated portal port | `https://<portal-domain>[:port]/api/v1` (portal page at `/`) |
| Dedicated portal domain + reverse proxy | Same as above; the proxy must forward both page paths and `/api/v1` |

### 1.2 Authentication

- After sign-in, send `Authorization: Bearer <access_token>`;
- Access tokens last 24h, refresh tokens 7 days;
- The JWT contains `tv` (token_version): logout / forced sign-out increments it and invalidates all old tokens
  (`clients.token_version`, see `internal/service/clientauth/service.go`);
- Every protected endpoint passes through `ClientAuthMiddleware` (version check + maintenance-mode guard).

### 1.3 Response envelope

Most endpoints return `{ "code": 0, "data": ..., "message": "..." }`;
some list endpoints return `{ "items": [...], "total": N }` or a bare array (noted per endpoint below).
Errors return `{ "code": "ERROR_CODE", "message": "...", "request_id": "...", "can_retry": ... }`
(HTTP 4xx/5xx).

### 1.4 Amount units

Order / balance fields are **integer cents**: `990` = ¥9.90 (`currency: "CNY"`).

### 1.5 Real example (sign in → fetch profile)

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

On 401, call `POST /api/v1/client-auth/refresh` once and retry; if it still fails, clear the local session (the portal template already does this).

---

## 2. Public endpoints (no sign-in)

| Method | Path | Description |
|---|---|---|
| GET | `/p` (`/` on a dedicated portal engine) | Portal HTML (server-rendered + Vue runtime) |
| GET | `/api/v1/public/portal-config` | Portal runtime config: site name, theme colors, navigation, modules, language packs, `registration_enabled`, `regions` (sign-in region source), `privacy_policy_url` / `terms_of_service_url` / `privacy_policy_content` / `terms_of_service_content` (compliance docs; content mapped per language; standalone pages `/p/legal/*` ✓) |
| GET | `/api/v1/public/settings` | Public settings (currently `registration_enabled`) |
| GET | `/api/v1/public/subscription-endpoints` | Subscription endpoint list (bare array) |
| GET | `/api/v1/public/articles` | Public articles (published only) |
| GET | `/api/v1/public/articles/:slug` | Article detail |
| GET | `/api/v1/public/client-configs` | Client download configuration |
| GET | `/sub/:token` | Subscription delivery (`format` returns base64_uri / raw_uri / mihomo_yaml / singbox_json / xray_json, etc.; revoked/missing → 404, suspended/expired/inactive → 403. See [subscription.md](./subscription.md)) |
| GET | `/api/v1/client-auth/oauth/providers` | OAuth provider visibility: `{"data":{"providers":[{"provider":"github","enabled":true},...]}}` |
| GET | `/api/v1/system/maintenance` | Maintenance status (public). The portal shows a maintenance page when enabled |

> All of these include the maintenance guard and public API rate limiting.

## 3. Auth / OAuth

| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/api/v1/client-auth/register` | — | Register. Body: `{email, password, referral_code?}`. Returns `{access_token, refresh_token, client}`; registration rate limited |
| POST | `/api/v1/client-auth/login` | — | Sign in. Body: `{email, password}`. Same response; issues JWTs with `token_version` |
| POST | `/api/v1/client-auth/refresh` | — | Refresh. Body: `{refresh_token}`. Returns new access/refresh tokens (version checked; old refresh tokens die immediately after logout) |
| POST | `/api/v1/client-auth/logout` | ✓ | Logout: server-side revoke (`token_version +1`) → all old tokens for the client are invalid |
| POST | `/api/v1/client-auth/rotate-token` | ✓ | Rotate the subscription token |
| POST | `/api/v1/client-auth/ticket` | ✓ | Issue a **one-time short-lived ticket** (for `window.open` flows such as “download log bundle” where headers cannot be set) |
| GET | `/api/v1/client-auth/oauth/:provider` | — | Start OAuth: 307 → GitHub/Google consent (`state` anti-CSRF, 10 minutes); disabled providers return 400 |
| GET | `/api/v1/client-auth/oauth/:provider/callback` | — | OAuth callback: exchange code → create/link client → issue versioned JWT → 302 back to the portal (`/<portalPath>?access_token=...&refresh_token=...`) |

> The OAuth callback is a **browser redirect** (not an API call): after a successful sign-in the portal reads the token
> from the URL in `onMounted` (or via same-origin `postMessage`, see `handleOAuthCallback` in the `web` portal template).

## 4. Account

| Method | Path | Description |
|---|---|---|
| GET | `/api/v1/client-auth/me` | Current client: `{id, email, plan_id, plan_name, status, quota_bytes, used_bytes, upload_bytes, download_bytes, expires_at, ip_whitelist, device_limit, sub_token, sub_status, created_at, ...}` (`data` envelope) |
| POST | `/api/v1/client-auth/change-password` | Body: `{old_password, new_password}` |
| GET | `/api/v1/client-auth/checktime` | Server time (client clock sync) |
| GET | `/api/v1/client-auth/connection-info` | Current plan connection parameters: `{concurrent, device_limit, protocol, speed_limit, tls, transport}` |

## 5. Subscriptions

| Method | Path | Description |
|---|---|---|
| POST | `/api/v1/client-auth/reset-subscription` | Reset the subscription; this rotates `sub_token` |

> The subscription link is built on the frontend from `sub_token` returned by `/me` (`/sub/:token`); there are no
> separate `subscription-link` / `subscription-formats` endpoints anymore.

## 6. Nodes

| Method | Path | Description |
|---|---|---|
| GET | `/api/v1/client-auth/nodes` | Nodes visible to the current plan: `{items:[...]}` (display fields such as name/protocol/transport/region) |
| GET | `/api/v1/client-auth/nodes/:id/config` | Full node configuration |
| GET | `/api/v1/client-auth/nodes/:id/link` | Share link (vless:// etc.) |
| GET | `/api/v1/client-auth/nodes/:id/qrcode` | QR code image |

## 7. Store / plans / orders

| Method | Path | Description |
|---|---|---|
| GET | `/api/v1/client-auth/plans` | Plan list (bare array): `{id, name, description, category, sort_order, variants:[{id, name, billing_cycle, duration_days, price(cents), original_price, currency, quota_bytes, ...}]}` |
| GET | `/api/v1/client-auth/plans/:id` | Plan detail |
| GET | `/api/v1/client-auth/traffic-packs` | Traffic pack list |
| POST | `/api/v1/client-auth/traffic-packs/buy` | Buy a traffic pack |
| GET | `/api/v1/client-auth/orders` | My plan orders (paginated) |
| POST | `/api/v1/client-auth/orders` | Create an order (buy/renew a plan variant) |
| GET | `/api/v1/client-auth/orders/:id` | Order detail |
| GET | `/api/v1/client-auth/invoices` | Invoices |
| GET | `/api/v1/client-auth/invoices/:id` | Invoice detail |

## 8. Balance / recharge

| Method | Path | Description |
|---|---|---|
| GET | `/api/v1/client-auth/balance` | `{data:{client_id, balance(cents), frozen}}` |
| GET | `/api/v1/client-auth/balance/transactions` | Balance transactions (`type`: charge/consume/refund; paginated) |
| GET | `/api/v1/client-auth/charge/sites` | Available recharge sites (`ListChargeSites`: enabled only) |
| POST | `/api/v1/client-auth/charge/create` | Create a recharge order (redirects to third-party payment) |
| GET | `/api/v1/client-auth/charge/orders` | Recharge order list |
| GET | `/api/v1/client-auth/charge/orders/:id` | Recharge order detail |
| GET | `/api/v1/client-auth/charge/status/:trade_no` | Poll payment status by trade_no |
| POST | `/api/v1/client-auth/charge/redeem-card` | Redeem a recharge card |

> Deleting a client **cascades** to its balance and transactions (see `DeleteClient` in `internal/service/client/service.go`).

## 9. Gifts / traffic history

| Method | Path | Description |
|---|---|---|
| GET | `/api/v1/client-auth/gifts` | Claimable gifts (bare array; `type`: days/bytes) |
| POST | `/api/v1/client-auth/gifts/:id/claim` | Claim |
| GET | `/api/v1/client-auth/traffic-history` | Monthly traffic history: `{data:{history:[{period_start,total_bytes,...}], months}}` |

## 10. Tickets

| Method | Path | Description |
|---|---|---|
| GET | `/api/v1/client-auth/tickets` | Ticket list: `{items,total}` |
| POST | `/api/v1/client-auth/tickets` | Create a ticket |
| GET | `/api/v1/client-auth/tickets/:id` | Ticket detail (with replies) |
| POST | `/api/v1/client-auth/tickets/:id/reply` | Reply |
| POST | `/api/v1/client-auth/tickets/:id/close` | Close |

## 11. Articles / guides

| Method | Path | Description |
|---|---|---|
| GET | `/api/v1/client-auth/articles` | Article list (with category filter) |
| GET | `/api/v1/client-auth/articles/:slug` | Article detail |

## 12. Referrals (disabled by default; can be enabled in the portal editor)

| Method | Path | Description |
|---|---|---|
| GET | `/api/v1/client-auth/referral/code` | My referral code and link |
| GET | `/api/v1/client-auth/referral/stats` | Commission stats (`commission_rate`, `available_commission`, `min_withdraw_amount`) |
| GET | `/api/v1/client-auth/referral/records` | Referral records |
| GET | `/api/v1/client-auth/referral/withdrawals` | Withdrawal records |
| POST | `/api/v1/client-auth/referral/withdraw` | Request a withdrawal |

## 13. Shared accounts

| Method | Path | Description |
|---|---|---|
| GET | `/api/v1/client-auth/shared-accounts` | Available shared accounts (streaming, etc.) |
| GET | `/api/v1/client-auth/shared-accounts/:id` | Detail |
| POST | `/api/v1/client-auth/shared-accounts/:id/apply` | Request access |
| POST | `/api/v1/client-auth/shared-accounts/:id/repair` | One-click repair |

## 14. Log bundles (subscriber diagnostics)

| Method | Path | Description |
|---|---|---|
| GET | `/api/v1/client-auth/logs/types` | Log types that can be bundled |
| GET | `/api/v1/client-auth/logs/packages` | My log bundles |
| POST | `/api/v1/client-auth/logs/package` | Create a log bundling job (entry nodes only) |
| GET | `/api/v1/client-auth/logs/packages/:id` | Job status (ready/processing/failed) |
| GET | `/api/v1/client-auth/logs/packages/:id/download` | Download (prefer `POST /ticket` and append `?ticket=`) |
| DELETE | `/api/v1/client-auth/logs/packages/:id` | Delete a log bundle |

## 15. Security settings / IP allow-list / self-unban

| Method | Path | Description |
|---|---|---|
| GET/PUT | `/api/v1/client-auth/settings` | Client settings: `{ip_whitelist, device_limit}` (PUT) |
| GET | `/api/v1/client-auth/settings/whitelist` | Allow-list (bare array) |
| DELETE | `/api/v1/client-auth/settings/whitelist/:id` | Delete an allow-list entry |
| POST | `/api/v1/client-auth/settings/auto-add-ip` | Add the current IP to the allow-list |
| POST | `/api/v1/client-auth/self-unban` | Self-service unban (attempts limited by policy) |

> The IP allow-list **only applies to subscription URLs** (`/sub/:token`); it does not block portal sign-in/OAuth or
> other client-auth endpoints. If the user has no allow-list or no subscription URL, access is always allowed. The
> allow-list is a per-user setting; there is no global switch.

**Self-unban flow**: available only for `auto_quota` (automatic ban on traffic overage) when the admin-side
`self_unban.enabled` is on. Sign-in (password or OAuth callback) returns `require_self_unban=true` + `self_unban_token`
(typ=self_unban, valid 15 minutes, **only** usable on `POST /self-unban`; other endpoints return 403/`SELF_UNBAN_TOKEN_SCOPE`).
The portal then shows the unban card; after a successful unban the user signs in again. OAuth callbacks deliver this via
the URL fragment `#require_self_unban=1&self_unban_token=..&self_unban_hint=..`.

## 16. Error codes (excerpt)

| code | Scenario |
|---|---|
| `INVALID_TOKEN` | Refresh token invalid/expired/version mismatch |
| `INVALID_REQUEST` | Parameter validation failed |
| `NOT_FOUND` | Resource missing (ticket/order/allow-list entry, etc.) |
| `RELOAD_FAILED` / `SYNC_FAILED` | (Admin side) proxy configuration sync failed |
| 401 + `errClientTokenRevoked` | token_version mismatch (logged out/kicked) → clients should refresh; if that fails, clear local tokens |

## 17. Compliance documents (privacy policy / terms of service)

The “Privacy Policy · Terms of Service” links at the bottom of the sign-in card (same as ordinary sites; clicking opens the content):

- **Configuration**: portal editor → Settings → General → Compliance documents (**edit content per language**);
  you can also `PUT /api/v1/portal/active` with structured fields `privacy_policy_url` / `privacy_policy_content` /
  `terms_of_service_url` / `terms_of_service_content` (nil = unchanged; content is a `{"zh-CN":"...","en":"..."}` map).
- **Standalone accessible pages** (server-rendered, easy for Google verification crawlers; supports `?lang=xx`):
  - Main panel domain: `GET /p/legal/privacy`, `GET /p/legal/terms`
  - Dedicated portal domain (root deployment): `GET /legal/privacy`, `GET /legal/terms`
- **Display logic**: URL or content, **URL wins** (opens externally in a new tab); if content is provided, clicking the
  sign-in card renders Markdown in a dialog (`renderMd`) using the current portal language.
- Delivered at runtime via `/api/v1/public/portal-config`; the sign-in component consumes
  `ctx.privacyUrl / termsUrl / privacyContent / termsContent / openLegal / hasLegalLinks`.
- Google OAuth verification requires these links to be publicly accessible → use the standalone links above in the
  consent screen; see `docs/oauth-google-verification.md` (contains privacy policy / terms templates).

## 18. Integration example

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

On 401, call `POST /refresh` once and retry; if it still fails, clear the local session
(the portal template's `api()` already implements this).
