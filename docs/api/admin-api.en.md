<!-- source: admin-api.md | locale: en | updated: 2026-10-23 -->
# Admin API

> Languages: [简体中文](admin-api.md) · English · [繁體中文](admin-api.zh-TW.md) · [Русский](admin-api.ru.md) · [日本語](admin-api.ja.md) · [한국어](admin-api.ko.md) · [Deutsch](admin-api.de.md) · [Français](admin-api.fr.md) · [Español](admin-api.es.md) · [Português](admin-api.pt.md) · [Türkçe](admin-api.tr.md) · [Bahasa Indonesia](admin-api.id.md) · [ไทย](admin-api.th.md) · [Tiếng Việt](admin-api.vi.md) · [العربية](admin-api.ar.md) · [فارسی](admin-api.fa.md)

The complete admin API route table for **panel development / automation / integrators**.

- Base URL: `https://<panel-domain>/api/v1`
- Auth: `Authorization: Bearer <admin_token>` (from `POST /api/v1/auth/login`)
- Common response/error formats and rate-limit conventions: see [README.md](./README.en.md)
- Source of truth: `internal/api/router/router.go` (739 admin routes); if this table disagrees, **the code wins**

## Permission markers

| Marker | Meaning |
|---|---|
| Public | No sign-in required (maintenance guard / rate limiting may still apply) |
| Public · rate-limited | No sign-in, with a dedicated rate limit |
| Signed in | Requires a signed-in admin token (`AuthRequired`) |
| Admin | Requires role ≥ `admin` (`RequireRole(RoleAdmin)`) |
| Super admin | Requires role = `super_admin` |

Role levels: `viewer(10) < operator(20) < admin(30) < super_admin(40)`; insufficient roles return 403 `INSUFFICIENT_ROLE`.

---

## Quick start (real examples)

These examples come from real local responses; tokens and data are redacted.

### 1. Sign in and get a token

```bash
curl -X POST https://<panel-domain>/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"your-password"}'
```

Response:

```json
{
  "data": {
    "access_token": "<JWT, valid 24h>",
    "refresh_token": "<JWT, valid 7 days>",
    "user": { "id": 1, "role": "super_admin", "username": "admin" }
  }
}
```

### 2. Call a business endpoint with the token

```bash
curl https://<panel-domain>/api/v1/clients?limit=1 \
  -H 'Authorization: Bearer <access_token>'
```

Response (lists use the `items` + `total` structure):

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

### 3. Error response examples

**401 unauthenticated:**

```json
{
  "code": "MISSING_TOKEN",
  "message": "missing authorization header",
  "request_id": "a02cfdf9-6be0-46be-8584-824caab34dc7",
  "can_retry": false,
  "requires_action": false
}
```

**404 resource not found:**

```json
{
  "code": "CLIENT_NOT_FOUND",
  "message": "client not found",
  "request_id": "099949bd-4ded-468d-a6f8-f8e798af2273",
  "can_retry": false,
  "requires_action": false
}
```

### 4. Pagination

All list endpoints use `limit` (page size, default 20) and `offset` (0-based) and return `{items, total, limit, offset}`. Some legacy endpoints return a bare array (noted per table).

---

## Contents

- [Admin authentication and security](#admin-authentication-and-security)
- [Admin accounts](#admin-accounts)
- [Customer management](#customer-management)
- [Plan templates](#plan-templates)
- [Plan variants](#plan-variants)
- [Traffic packs](#traffic-packs)
- [Coupons](#coupons)
- [Node groups](#node-groups)
- [Node servers](#node-servers)
- [Node audit log](#node-audit-log)
- [Nodes (legacy model, retired)](#nodes-legacy-model-retired)
- [Region overview](#region-overview)
- [Subscriptions](#subscriptions)
- [Subscription formats](#subscription-formats)
- [Subscription endpoints](#subscription-endpoints)
- [Share domains](#share-domains)
- [Cloudflare IP](#cloudflare-ip)
- [Subscription config (Mihomo/Clash)](#subscription-config-mihomoclash)
- [Recharge sites](#recharge-sites)
- [Recharge orders](#recharge-orders)
- [Recharge cards](#recharge-cards)
- [Charge callback events](#charge-callback-events)
- [Charge callback retries](#charge-callback-retries)
- [Customer balance](#customer-balance)
- [Gifts](#gifts)
- [Referral config](#referral-config)
- [Referral withdrawals](#referral-withdrawals)
- [IP allow-lists (summary)](#ip-allow-lists-summary)
- [Customer OAuth bindings](#customer-oauth-bindings)
- [Tickets (admin)](#tickets-admin)
- [Articles](#articles)
- [Shared accounts](#shared-accounts)
- [Certificates](#certificates)
- [Certificate channels](#certificate-channels)
- [EAB configuration](#eab-configuration)
- [ACME accounts](#acme-accounts)
- [Built-in CA](#built-in-ca)
- [Geo rules](#geo-rules)
- [WAF rule library](#waf-rule-library)
- [Log management](#log-management)
- [Backups](#backups)
- [System settings](#system-settings)
- [Security audit](#security-audit)
- [Metrics](#metrics)
- [Portal templates](#portal-templates)
- [Agent installation](#agent-installation)
- [Charge callback entry](#charge-callback-entry)
- [Inbounds (with nodes)](#inbounds-with-nodes)
- [Public config](#public-config)
- [Client auth and business (same set on the admin engine)](#client-auth-and-business-same-set-on-the-admin-engine)
- [Subscription delivery](#subscription-delivery)
- [Runtime configuration workflow](#runtime-configuration-workflow)
- [Service access policies (retired)](#service-access-policies-retired)
- [Traffic quota monitoring](#traffic-quota-monitoring)
- [Health checks](#health-checks)
- [Portal page](#portal-page)
- [ACME challenges](#acme-challenges)

---

### Admin authentication and security

Sign-in, sign-out, password change, 2FA (TOTP), Passkey (WebAuthn) and one-time tickets for the admin panel.

| Method | Path | Permission | Description |
|---|---|---|---|
| POST | `/api/v1/auth/2fa/disable` | Signed in | Disable |
| POST | `/api/v1/auth/2fa/enable` | Signed in | Enable |
| POST | `/api/v1/auth/2fa/recovery-codes` | Signed in | Regenerate recovery codes |
| POST | `/api/v1/auth/2fa/setup` | Signed in | Settings |
| GET | `/api/v1/auth/2fa/status` | Signed in | Status |
| POST | `/api/v1/auth/change-password` | Signed in | Change password |
| POST | `/api/v1/auth/login` | Public | Sign in |
| POST | `/api/v1/auth/login/2fa` | Public | Dynamic-code sign-in (2FA) |
| GET | `/api/v1/auth/login/2fa-status` | Public | Check whether the account has 2FA enabled |
| POST | `/api/v1/auth/logout` | Signed in | Sign out |
| GET | `/api/v1/auth/me` | Signed in | Current signed-in user |
| GET | `/api/v1/auth/passkey/credentials` | Signed in | Passkey credential list |
| DELETE | `/api/v1/auth/passkey/credentials/:id` | Signed in | Delete Passkey credential |
| POST | `/api/v1/auth/passkey/login/begin` | Public | Start sign-in |
| POST | `/api/v1/auth/passkey/login/finish` | Public | Complete sign-in |
| POST | `/api/v1/auth/passkey/register/begin` | Signed in | Start registration |
| POST | `/api/v1/auth/passkey/register/finish` | Signed in | Complete registration |
| GET | `/api/v1/auth/passkey/rpids` | Signed in | RP ID allow-list |
| POST | `/api/v1/auth/passkey/rpids` | Signed in | Update RP ID allow-list |
| GET | `/api/v1/auth/passkey/status` | Public | Status |
| POST | `/api/v1/auth/refresh` | Public | Refresh |
| POST | `/api/v1/auth/ticket` | Signed in | Issue one-time short-lived ticket |

### Admin accounts

Admin panel user accounts. Listing requires admin; create/update/delete require super admin (prevents privilege escalation).

| Method | Path | Permission | Description |
|---|---|---|---|
| GET | `/api/v1/users` | Admin | List |
| POST | `/api/v1/users` | Super admin | Create |
| DELETE | `/api/v1/users/:id` | Super admin | Delete |
| PUT | `/api/v1/users/:id` | Super admin | Update |

### Customer management

CRUD, suspend/resume, upgrade/downgrade, renewal and usage reset for subscribers (portal accounts).

| Method | Path | Permission | Description |
|---|---|---|---|
| GET | `/api/v1/clients` | Signed in | List customers |
| POST | `/api/v1/clients` | Signed in | Create customer. body: `{"email":"user@example.com","password":"plaintext-password","plan_id":1,"quota_bytes":10737418240,"expires_at":"2026-12-31T00:00:00Z","remark":""}`. `plan_id` is required; `password` may be empty (customers can set it later in the portal) |
| DELETE | `/api/v1/clients/:id` | Signed in | Delete customer |
| GET | `/api/v1/clients/:id` | Signed in | Get customer |
| PUT | `/api/v1/clients/:id` | Signed in | Update customer |
| POST | `/api/v1/clients/:id/downgrade` | Signed in | Downgrade customer |
| GET | `/api/v1/clients/:id/orders` | Signed in | Customer order list |
| POST | `/api/v1/clients/:id/renew` | Signed in | Renew customer |
| POST | `/api/v1/clients/:id/reset-usage` | Signed in | Reset usage |
| POST | `/api/v1/clients/:id/restore` | Signed in | Resume customer |
| POST | `/api/v1/clients/:id/suspend` | Signed in | Suspend customer |
| POST | `/api/v1/clients/:id/upgrade` | Signed in | Upgrade customer |

### Plan templates

Plans (variants, billing cycles, prices, traffic quotas) and template-level node authorization.

| Method | Path | Permission | Description |
|---|---|---|---|
| GET | `/api/v1/plan-templates` | Signed in | Template list |
| POST | `/api/v1/plan-templates` | Signed in | Create template |
| DELETE | `/api/v1/plan-templates/:id` | Signed in | Delete template |
| GET | `/api/v1/plan-templates/:id` | Signed in | Template details |
| PUT | `/api/v1/plan-templates/:id` | Signed in | Update template |
| GET | `/api/v1/plan-templates/:id/accessible-nodes` | Signed in | Get available nodes |
| GET | `/api/v1/plan-templates/:id/billing-node-options` | Signed in | Get billable node options |
| PUT | `/api/v1/plan-templates/:id/node-groups` | Signed in | Set template node groups |
| POST | `/api/v1/plan-templates/:id/sync-accounts` | Signed in | Sync accounts to template |
| GET | `/api/v1/plan-templates/:id/variants` | Signed in | Variant list |
| POST | `/api/v1/plan-templates/:id/variants` | Signed in | Create variant |
| POST | `/api/v1/plan-templates/sync-all-accounts` | Signed in | Sync all accounts |

### Plan variants

Update and delete plan variants (creation is nested under plan templates).

| Method | Path | Permission | Description |
|---|---|---|---|
| DELETE | `/api/v1/plan-variants/:id` | Signed in | Delete variant |
| PUT | `/api/v1/plan-variants/:id` | Signed in | Update variant |

### Traffic packs

Traffic packs that can be purchased separately.

| Method | Path | Permission | Description |
|---|---|---|---|
| GET | `/api/v1/traffic-packs` | Signed in | Traffic pack list |
| POST | `/api/v1/traffic-packs` | Signed in | Create traffic pack |
| DELETE | `/api/v1/traffic-packs/:id` | Signed in | Delete traffic pack |
| PUT | `/api/v1/traffic-packs/:id` | Signed in | Update traffic pack |

### Coupons

Create, validate and redeem coupons.

| Method | Path | Permission | Description |
|---|---|---|---|
| GET | `/api/v1/promotions` | Signed in | List coupons |
| POST | `/api/v1/promotions` | Signed in | Create coupon |
| DELETE | `/api/v1/promotions/:id` | Signed in | Delete coupon |
| PUT | `/api/v1/promotions/:id` | Signed in | Update coupon |
| POST | `/api/v1/promotions/validate` | Signed in | Validate coupon |

### Node groups

Organize nodes into groups referenced by plans.

| Method | Path | Permission | Description |
|---|---|---|---|
| GET | `/api/v1/node-groups` | Signed in | List node groups |
| POST | `/api/v1/node-groups` | Signed in | Create node group |
| DELETE | `/api/v1/node-groups/:id` | Signed in | Delete node group |
| PUT | `/api/v1/node-groups/:id` | Signed in | Update node group |
| GET | `/api/v1/node-groups/:id/members` | Signed in | List group members |
| POST | `/api/v1/node-groups/:id/members` | Signed in | Add group member |
| DELETE | `/api/v1/node-groups/:id/members/:mid` | Signed in | Remove group member |

### Node servers

Core resources: node CRUD, core (xray/sing-box) config delivery, inbound/outbound/endpoint config, process control, certificates, WAF, rate limits, port hopping, chained proxies, mirror/primary-backup, Agent install/upgrade, xray runtime control. Many sub-paths, grouped by feature family.

| Method | Path | Permission | Description |
|---|---|---|---|
| GET | `/api/v1/node-servers` | Admin | List |
| POST | `/api/v1/node-servers` | Admin | Create |
| DELETE | `/api/v1/node-servers/:id` | Admin | Delete |
| GET | `/api/v1/node-servers/:id` | Admin | Details |
| PUT | `/api/v1/node-servers/:id` | Admin | Update |
| GET | `/api/v1/node-servers/:id/binaries` | Admin | Installed core list |
| POST | `/api/v1/node-servers/:id/binaries/ech-generate` | Admin | Generate ECH key pair |
| POST | `/api/v1/node-servers/:id/binaries/install` | Admin | Install/upgrade core |
| GET | `/api/v1/node-servers/:id/binaries/interfaces` | Admin | Network interface list |
| POST | `/api/v1/node-servers/:id/binaries/mldsa65-key` | Admin | Generate MLDSA65 key |
| POST | `/api/v1/node-servers/:id/binaries/reality-key` | Admin | Generate REALITY key pair |
| POST | `/api/v1/node-servers/:id/binaries/reality-public-key` | Admin | Derive public key from REALITY private key |
| GET | `/api/v1/node-servers/:id/binaries/releases` | Admin | Available versions |
| POST | `/api/v1/node-servers/:id/binaries/vlessenc` | Admin | Generate VLESS encryption parameters |
| POST | `/api/v1/node-servers/:id/binaries/wg-key` | Admin | Generate WireGuard key |
| POST | `/api/v1/node-servers/:id/binaries/x25519-key` | Admin | Generate X25519 key pair |
| GET | `/api/v1/node-servers/:id/bootstrap-cmd` | Admin | Get Agent bootstrap command |
| POST | `/api/v1/node-servers/:id/bootstrap-token` | Admin | Generate bootstrap token |
| POST | `/api/v1/node-servers/:id/caddy/batch-update-log-enabled` | Admin | Bulk update log switches |
| POST | `/api/v1/node-servers/:id/caddy/batch-update-ports` | Admin | Bulk update ports |
| POST | `/api/v1/node-servers/:id/caddy/generate-config` | Admin | Generate config |
| DELETE | `/api/v1/node-servers/:id/caddy/l4-rules` | Admin | Delete all L4 rules |
| POST | `/api/v1/node-servers/:id/caddy/save-and-apply` | Admin | Save and apply |
| GET | `/api/v1/node-servers/:id/caddy/sites` | Admin | Site list |
| POST | `/api/v1/node-servers/:id/caddy/sites` | Admin | Create site |
| DELETE | `/api/v1/node-servers/:id/caddy/sites/:siteId` | Admin | Delete site |
| PUT | `/api/v1/node-servers/:id/caddy/sites/:siteId` | Admin | Update site |
| DELETE | `/api/v1/node-servers/:id/caddy/sni-rules` | Admin | Delete all SNI rules |
| POST | `/api/v1/node-servers/:id/caddy/sni-rules/toggle` | Admin | Toggle SNI rules |
| GET | `/api/v1/node-servers/:id/caddy/sni-settings` | Admin | SNI settings |
| POST | `/api/v1/node-servers/:id/caddy/sni-settings` | Admin | Save SNI settings |
| GET | `/api/v1/node-servers/:id/certificates` | Admin | Node certificate list |
| DELETE | `/api/v1/node-servers/:id/certificates/:cid` | Admin | Unbind certificate |
| POST | `/api/v1/node-servers/:id/certificates/:cid` | Admin | Bind certificate |
| GET | `/api/v1/node-servers/:id/chain-config` | Admin | Generate chained config |
| GET | `/api/v1/node-servers/:id/chain-health` | Admin | Chain health status |
| POST | `/api/v1/node-servers/:id/control` | Super admin | Control node core. body: `{"core_type":"xray","action":"start"}` (action = start/stop/restart). `core_type` can be `xray`/`singbox` |
| GET | `/api/v1/node-servers/:id/degradation-status` | Admin | Degraded status |
| POST | `/api/v1/node-servers/:id/distribute-cert` | Admin | Distribute certificate |
| GET | `/api/v1/node-servers/:id/endpoint-configs` | Admin | List |
| POST | `/api/v1/node-servers/:id/endpoint-configs` | Admin | Create |
| DELETE | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | Admin | Delete |
| GET | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | Admin | Details |
| PUT | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | Admin | Update |
| POST | `/api/v1/node-servers/:id/failover` | Admin | Trigger failover for this node |
| GET | `/api/v1/node-servers/:id/failover-status` | Admin | Failover status |
| POST | `/api/v1/node-servers/:id/force-reconnect` | Admin | Force reconnect |
| GET | `/api/v1/node-servers/:id/free-port` | Admin | Request a free port |
| POST | `/api/v1/node-servers/:id/geo-rules/push` | Admin | Push to this node |
| GET | `/api/v1/node-servers/:id/inbound-configs` | Admin | List |
| POST | `/api/v1/node-servers/:id/inbound-configs` | Admin | Create |
| DELETE | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | Admin | Delete |
| GET | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | Admin | Details |
| PUT | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | Admin | Update |
| GET | `/api/v1/node-servers/:id/inbound-configs/:cfgId/share-links` | Admin | Share links and snippets |
| POST | `/api/v1/node-servers/:id/inbound-configs/:cfgId/users/:email/pause` | Admin | Suspend this user |
| POST | `/api/v1/node-servers/:id/inbound-configs/:cfgId/users/:email/resume` | Admin | Restore this user |
| POST | `/api/v1/node-servers/:id/inbound-configs/reset-all-traffic` | Admin | Reset all inbound traffic |
| GET | `/api/v1/node-servers/:id/install-script` | Admin | Get Agent install script |
| GET | `/api/v1/node-servers/:id/metrics-history` | Admin | Get metric history |
| GET | `/api/v1/node-servers/:id/mirror-status` | Admin | Mirror status |
| POST | `/api/v1/node-servers/:id/nginx/batch-update-ports` | Admin | Bulk update ports |
| POST | `/api/v1/node-servers/:id/nginx/generate-config` | Admin | Generate config |
| DELETE | `/api/v1/node-servers/:id/nginx/l4-rules` | Admin | Delete all L4 rules |
| POST | `/api/v1/node-servers/:id/nginx/save-and-apply` | Admin | Save and apply |
| GET | `/api/v1/node-servers/:id/nginx/settings` | Admin | Read settings |
| POST | `/api/v1/node-servers/:id/nginx/settings` | Admin | Save settings |
| GET | `/api/v1/node-servers/:id/nginx/sites` | Admin | Site list |
| POST | `/api/v1/node-servers/:id/nginx/sites` | Admin | Create site |
| DELETE | `/api/v1/node-servers/:id/nginx/sites/:siteId` | Admin | Delete site |
| PUT | `/api/v1/node-servers/:id/nginx/sites/:siteId` | Admin | Update site |
| DELETE | `/api/v1/node-servers/:id/nginx/sni-rules` | Admin | Delete all SNI rules |
| POST | `/api/v1/node-servers/:id/nginx/sni-rules/toggle` | Admin | Toggle SNI rules |
| PUT | `/api/v1/node-servers/:id/node-type` | Admin | Update node type |
| GET | `/api/v1/node-servers/:id/online-users` | Admin | Online users |
| GET | `/api/v1/node-servers/:id/outbound-configs` | Admin | List |
| POST | `/api/v1/node-servers/:id/outbound-configs` | Admin | Create |
| DELETE | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | Admin | Delete |
| GET | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | Admin | Details |
| PUT | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | Admin | Update |
| PUT | `/api/v1/node-servers/:id/outbound-configs/reorder` | Admin | Sort |
| GET | `/api/v1/node-servers/:id/port-hop/rules` | Admin | Port-hopping rule preview |
| GET | `/api/v1/node-servers/:id/protocols` | Admin | Protocol list |
| GET | `/api/v1/node-servers/:id/protocols/:coreType` | Admin | Protocol config |
| PUT | `/api/v1/node-servers/:id/protocols/:coreType` | Admin | Save protocol config |
| GET | `/api/v1/node-servers/:id/protocols/:coreType/config-file` | Admin | Read core config file |
| GET | `/api/v1/node-servers/:id/protocols/:coreType/section/:section` | Admin | Read protocol section |
| PUT | `/api/v1/node-servers/:id/protocols/:coreType/section/:section` | Admin | Save protocol section |
| POST | `/api/v1/node-servers/:id/protocols/:coreType/validate` | Admin | Validate config (real core dry-run) |
| GET | `/api/v1/node-servers/:id/proxy-status` | Admin | Remote proxy status |
| POST | `/api/v1/node-servers/:id/proxy/batch-update-ports` | Admin | Bulk update ports |
| GET | `/api/v1/node-servers/:id/proxy/config` | Admin | Read config |
| PUT | `/api/v1/node-servers/:id/proxy/config` | Admin | Update config |
| POST | `/api/v1/node-servers/:id/proxy/control` | Admin | Chained proxy registration control |
| POST | `/api/v1/node-servers/:id/proxy/download` | Admin | Download proxy core version |
| POST | `/api/v1/node-servers/:id/proxy/generate-config` | Admin | Generate config |
| POST | `/api/v1/node-servers/:id/proxy/guardian` | Admin | Update supervisor config |
| POST | `/api/v1/node-servers/:id/proxy/install` | Admin | Install proxy |
| DELETE | `/api/v1/node-servers/:id/proxy/l4-rules` | Admin | Delete all L4 rules |
| GET | `/api/v1/node-servers/:id/proxy/l4-settings` | Admin | L4 settings |
| POST | `/api/v1/node-servers/:id/proxy/l4-settings` | Admin | Save L4 settings |
| GET | `/api/v1/node-servers/:id/proxy/log-settings` | Admin | Log settings |
| POST | `/api/v1/node-servers/:id/proxy/log-settings` | Admin | Save log settings |
| GET | `/api/v1/node-servers/:id/proxy/nginx-settings` | Admin | nginx settings |
| POST | `/api/v1/node-servers/:id/proxy/nginx-settings` | Admin | Save nginx settings |
| GET | `/api/v1/node-servers/:id/proxy/releases` | Admin | Available versions |
| POST | `/api/v1/node-servers/:id/proxy/reload` | Admin | Reload config |
| POST | `/api/v1/node-servers/:id/proxy/restart` | Admin | Restart |
| GET | `/api/v1/node-servers/:id/proxy/sites` | Admin | Site list |
| POST | `/api/v1/node-servers/:id/proxy/sites` | Admin | Create site |
| DELETE | `/api/v1/node-servers/:id/proxy/sites/:siteId` | Admin | Delete site |
| PUT | `/api/v1/node-servers/:id/proxy/sites/:siteId` | Admin | Update site |
| DELETE | `/api/v1/node-servers/:id/proxy/sni-rules` | Admin | Delete all SNI rules |
| POST | `/api/v1/node-servers/:id/proxy/sni-rules/toggle` | Admin | Toggle SNI rules |
| GET | `/api/v1/node-servers/:id/proxy/sni-settings` | Admin | SNI settings |
| POST | `/api/v1/node-servers/:id/proxy/sni-settings` | Admin | Save SNI settings |
| POST | `/api/v1/node-servers/:id/proxy/start` | Admin | Start |
| GET | `/api/v1/node-servers/:id/proxy/status` | Admin | Proxy process status |
| POST | `/api/v1/node-servers/:id/proxy/stop` | Admin | Stop |
| POST | `/api/v1/node-servers/:id/proxy/sync` | Admin | Sync proxy config |
| GET | `/api/v1/node-servers/:id/rate-limit` | Admin | Read config |
| PUT | `/api/v1/node-servers/:id/rate-limit` | Admin | Update config |
| GET | `/api/v1/node-servers/:id/rate-limit/status` | Admin | Status |
| GET | `/api/v1/node-servers/:id/recover-mode` | Admin | Read node recovery mode |
| PUT | `/api/v1/node-servers/:id/recover-mode` | Admin | Set node recovery mode |
| POST | `/api/v1/node-servers/:id/revoke-cert` | Admin | Revoke Agent certificate |
| GET | `/api/v1/node-servers/:id/services` | Admin | List |
| POST | `/api/v1/node-servers/:id/services` | Admin | Create |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId` | Admin | Delete |
| GET | `/api/v1/node-servers/:id/services/:serviceId` | Admin | Details |
| PUT | `/api/v1/node-servers/:id/services/:serviceId` | Admin | Update |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId/crash-logs` | Admin | Clear crash logs |
| GET | `/api/v1/node-servers/:id/services/:serviceId/crash-logs` | Admin | Crash log list |
| GET | `/api/v1/node-servers/:id/services/:serviceId/crash-logs/:logId` | Admin | Crash log details |
| GET | `/api/v1/node-servers/:id/services/:serviceId/endpoints` | Admin | Endpoint list |
| POST | `/api/v1/node-servers/:id/services/:serviceId/endpoints` | Admin | Create endpoint |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId/endpoints/:endpointId` | Admin | Delete endpoint |
| PUT | `/api/v1/node-servers/:id/services/:serviceId/endpoints/:endpointId` | Admin | Update endpoint |
| PUT | `/api/v1/node-servers/:id/services/:serviceId/guardian` | Admin | Update supervisor config |
| GET | `/api/v1/node-servers/:id/services/:serviceId/process-status` | Admin | Get processing status |
| POST | `/api/v1/node-servers/:id/services/:serviceId/restart` | Admin | Restart |
| POST | `/api/v1/node-servers/:id/services/:serviceId/start` | Admin | Start |
| POST | `/api/v1/node-servers/:id/services/:serviceId/stop` | Admin | Stop |
| GET | `/api/v1/node-servers/:id/services/process-status` | Admin | All process statuses |
| GET | `/api/v1/node-servers/:id/share-domains` | Admin | Share domains for this node |
| POST | `/api/v1/node-servers/:id/share-domains` | Admin | Create |
| DELETE | `/api/v1/node-servers/:id/share-domains/:domainId` | Admin | Delete |
| PUT | `/api/v1/node-servers/:id/share-domains/:domainId` | Admin | Update |
| DELETE | `/api/v1/node-servers/:id/share-domains/all` | Admin | Clear all |
| POST | `/api/v1/node-servers/:id/singbox/control/firewall/apply` | Admin | Apply port-hopping firewall rules |
| POST | `/api/v1/node-servers/:id/singbox/control/firewall/remove` | Admin | Remove port-hopping firewall rules |
| POST | `/api/v1/node-servers/:id/speed-test` | Admin | Node speed test |
| GET | `/api/v1/node-servers/:id/status` | Admin | Node status |
| POST | `/api/v1/node-servers/:id/sync` | Admin | Sync node config (generate and push) |
| POST | `/api/v1/node-servers/:id/sync-data` | Super admin | Push panel data to nodes (full sync of users/subscriptions/plans) |
| POST | `/api/v1/node-servers/:id/sync-mirror` | Admin | Sync mirror node |
| GET | `/api/v1/node-servers/:id/tags` | Admin | Tag list |
| POST | `/api/v1/node-servers/:id/tags` | Admin | Add tag |
| DELETE | `/api/v1/node-servers/:id/tags/:tagKey` | Admin | Delete tag |
| GET | `/api/v1/node-servers/:id/traffic-history` | Admin | Traffic history |
| POST | `/api/v1/node-servers/:id/traffic-resume` | Admin | Restore traffic suspended for overage |
| GET | `/api/v1/node-servers/:id/traffic-usage` | Admin | Traffic usage |
| POST | `/api/v1/node-servers/:id/update-agent` | Admin | Upgrade Agent |
| GET | `/api/v1/node-servers/:id/used-server-names` | Admin | In-use REALITY SNI list |
| GET | `/api/v1/node-servers/:id/waf/config` | Admin | Read config |
| PUT | `/api/v1/node-servers/:id/waf/config` | Admin | Save config |
| POST | `/api/v1/node-servers/:id/waf/disable-domain` | Admin | Disable domain protection |
| GET | `/api/v1/node-servers/:id/waf/domain-config` | Admin | Domain config list |
| PUT | `/api/v1/node-servers/:id/waf/domain-config` | Admin | Bulk save domain configs |
| DELETE | `/api/v1/node-servers/:id/waf/domain-config/:domain` | Admin | Delete domain config |
| GET | `/api/v1/node-servers/:id/waf/domain-config/:domain` | Admin | Domain config details |
| PUT | `/api/v1/node-servers/:id/waf/domain-config/:domain` | Admin | Save domain config |
| GET | `/api/v1/node-servers/:id/waf/domain-rules` | Admin | Domain rule directory |
| GET | `/api/v1/node-servers/:id/waf/domain-rules/:domain/:name` | Admin | Read domain rule file |
| PUT | `/api/v1/node-servers/:id/waf/domain-rules/:domain/:name` | Admin | Save domain rule file |
| GET | `/api/v1/node-servers/:id/waf/domain-status` | Admin | Domain protection status |
| POST | `/api/v1/node-servers/:id/waf/enable-domain` | Admin | Enable domain protection |
| POST | `/api/v1/node-servers/:id/waf/push` | Admin | Push to this node |
| GET | `/api/v1/node-servers/:id/waf/rules` | Admin | Node WAF rule list |
| DELETE | `/api/v1/node-servers/:id/waf/rules/:name` | Admin | Delete node WAF rules |
| GET | `/api/v1/node-servers/:id/waf/rules/:name` | Admin | Node WAF rule details |
| PUT | `/api/v1/node-servers/:id/waf/rules/:name` | Admin | Save node WAF rules |
| GET | `/api/v1/node-servers/:id/waf/status` | Admin | Node WAF status |
| PUT | `/api/v1/node-servers/:id/waf/status` | Admin | Toggle node WAF |
| POST | `/api/v1/node-servers/:id/waf/sync` | Admin | Sync to this node |
| GET | `/api/v1/node-servers/:id/warp` | Admin | List |
| DELETE | `/api/v1/node-servers/:id/warp/:warpId` | Admin | Delete |
| GET | `/api/v1/node-servers/:id/warp/config` | Admin | Read config |
| POST | `/api/v1/node-servers/:id/warp/generate-config` | Admin | Generate config |
| POST | `/api/v1/node-servers/:id/warp/register` | Admin | Register |
| GET | `/api/v1/node-servers/:id/warp/status` | Admin | Status |
| GET | `/api/v1/node-servers/:id/xray/control/balancer/:tag` | Admin | Get balancer info |
| POST | `/api/v1/node-servers/:id/xray/control/balancer/:tag` | Admin | Set balancer target |
| POST | `/api/v1/node-servers/:id/xray/control/firewall/apply` | Admin | Apply port-hopping firewall rules |
| POST | `/api/v1/node-servers/:id/xray/control/firewall/remove` | Admin | Remove port-hopping firewall rules |
| POST | `/api/v1/node-servers/:id/xray/control/inbound` | Admin | Add inbound |
| DELETE | `/api/v1/node-servers/:id/xray/control/inbound/:tag` | Admin | Remove inbound |
| POST | `/api/v1/node-servers/:id/xray/control/inbound/:tag/user` | Admin | Add user |
| DELETE | `/api/v1/node-servers/:id/xray/control/inbound/:tag/user/:email` | Admin | Remove user |
| POST | `/api/v1/node-servers/:id/xray/control/outbound` | Admin | Add outbound |
| DELETE | `/api/v1/node-servers/:id/xray/control/outbound/:tag` | Admin | Remove outbound |
| POST | `/api/v1/node-servers/:id/xray/control/routing` | Admin | Request route |
| POST | `/api/v1/node-servers/:id/xray/control/routing/rule` | Admin | Add routing rule |
| DELETE | `/api/v1/node-servers/:id/xray/control/routing/rule/:tag` | Admin | Remove routing rule |
| GET | `/api/v1/node-servers/:id/xray/control/routing/rules` | Admin | List routing rules |
| POST | `/api/v1/node-servers/:id/xray/control/sync` | Admin | Manually trigger xray config sync |
| GET | `/api/v1/node-servers/:id/xray/stats` | Admin | Get statistics |
| POST | `/api/v1/node-servers/:id/xray/stats/reset/:email` | Admin | Reset user statistics |
| GET | `/api/v1/node-servers/:id/xray/stats/users` | Admin | Get user statistics |
| GET | `/api/v1/node-servers/billable` | Admin | Billable node list |
| GET | `/api/v1/node-servers/ca-cert` | Admin | CA certificate |
| GET | `/api/v1/node-servers/ca-status` | Admin | CA status |
| GET | `/api/v1/node-servers/dashboard` | Admin | Dashboard status |
| POST | `/api/v1/node-servers/distribute-cert` | Admin | Distribute certificate to all nodes |
| POST | `/api/v1/node-servers/failover-check` | Admin | Trigger failover check |
| GET | `/api/v1/node-servers/metrics/aggregated` | Admin | Aggregated metrics |
| POST | `/api/v1/node-servers/migrate-cert` | Admin | Migrate certificate |
| GET | `/api/v1/node-servers/online-summary` | Admin | Online summary |
| GET | `/api/v1/node-servers/online-users` | Admin | All online users |
| GET | `/api/v1/node-servers/pending-agent-updates` | Admin | Agents pending upgrade |
| POST | `/api/v1/node-servers/recommend-region` | Admin | Recommended regions |
| POST | `/api/v1/node-servers/select-nodes` | Admin | Select nodes for subscription |
| POST | `/api/v1/node-servers/speed-test-all` | Admin | Speed test all nodes |
| GET | `/api/v1/node-servers/speed-test-results` | Admin | Speed test results |
| GET | `/api/v1/node-servers/traffic/stats` | Admin | Traffic statistics |
| POST | `/api/v1/node-servers/update-all-agents` | Admin | Upgrade all Agents |
| DELETE | `/api/v1/node-servers/users/:client_id` | Admin | Sync-delete a customer |
| GET | `/api/v1/node-servers/users/retry-queue` | Admin | Retry queue status |
| POST | `/api/v1/node-servers/users/retry-queue/process` | Admin | Process retry queue |
| POST | `/api/v1/node-servers/users/sync` | Admin | Sync a single customer |
| POST | `/api/v1/node-servers/validate-chain-config` | Admin | Validate chained config |

### Node audit log

Node-side operation audit and export.

| Method | Path | Permission | Description |
|---|---|---|---|
| GET | `/api/v1/node-audit-logs` | Signed in | Node audit log |
| GET | `/api/v1/node-audit-logs/export` | Signed in | Export node audit logs |

### Nodes (legacy model, retired)

> `/api/v1/nodes` CRUD was retired on 2026-10-20: nodes are managed uniformly through `/api/v1/node-servers`.
> Online-user queries remain at `/api/v1/node-servers/{online-users,online-summary,:id/online-users}`.

### Region overview

Node view aggregated by region.

| Method | Path | Permission | Description |
|---|---|---|---|
| GET | `/api/v1/regions/:region/servers` | Signed in | Node list for this region |
| GET | `/api/v1/regions/overview` | Signed in | Region overview |

### Subscriptions

Subscription record management: revoke, rotate token, access logs, usage reset.

| Method | Path | Permission | Description |
|---|---|---|---|
| GET | `/api/v1/subscriptions` | Signed in | List |
| POST | `/api/v1/subscriptions` | Signed in | Create |
| DELETE | `/api/v1/subscriptions/:id` | Signed in | Delete |
| GET | `/api/v1/subscriptions/:id` | Signed in | Details |
| PUT | `/api/v1/subscriptions/:id` | Signed in | Update |
| GET | `/api/v1/subscriptions/:id/access-logs` | Signed in | Access log |
| POST | `/api/v1/subscriptions/:id/enable` | Signed in | Enable |
| POST | `/api/v1/subscriptions/:id/reset-usage` | Signed in | Reset usage |
| POST | `/api/v1/subscriptions/:id/restore` | Signed in | Resume |
| POST | `/api/v1/subscriptions/:id/revoke` | Signed in | Revoke |
| POST | `/api/v1/subscriptions/:id/rotate` | Signed in | Rotate |
| GET | `/api/v1/subscriptions/:id/rotations` | Signed in | Rotation history |

### Subscription formats

> Admin-side `/api/v1/subscription-formats` CRUD was retired on 2026-10-20: the actual entry point is
> `/api/v1/mihomo/configs` (subscription format center). The `subscription_formats` table remains,
> used only as seed data for the public `/api/v1/public/client-configs` endpoint.

### Subscription endpoints

Dedicated subscription domains (reverse-proxied to the panel's `/sub` path).

| Method | Path | Permission | Description |
|---|---|---|---|
| GET | `/api/v1/subscription-endpoints` | Signed in | List |
| POST | `/api/v1/subscription-endpoints` | Signed in | Create |
| DELETE | `/api/v1/subscription-endpoints/:id` | Signed in | Delete |
| GET | `/api/v1/subscription-endpoints/:id` | Signed in | Details |
| PUT | `/api/v1/subscription-endpoints/:id` | Signed in | Update |

### Share domains

Access domain/IP pool used in client links.

| Method | Path | Permission | Description |
|---|---|---|---|
| GET | `/api/v1/share-domains` | Signed in | List |
| POST | `/api/v1/share-domains` | Signed in | Create |
| DELETE | `/api/v1/share-domains/:id` | Signed in | Delete |
| PUT | `/api/v1/share-domains/:id` | Signed in | Update |
| DELETE | `/api/v1/share-domains/all` | Signed in | Clear all |
| POST | `/api/v1/share-domains/seed` | Signed in | Initialize default data |

### Cloudflare IP

Cloudflare CDN IP pool and source management.

| Method | Path | Permission | Description |
|---|---|---|---|
| DELETE | `/api/v1/cfip/entries` | Signed in | Clear entries |
| GET | `/api/v1/cfip/entries` | Signed in | Entry list |
| POST | `/api/v1/cfip/entries` | Signed in | Add entry |
| DELETE | `/api/v1/cfip/entries/:id` | Signed in | Delete entry |
| POST | `/api/v1/cfip/fetch-all` | Signed in | Fetch all sources |
| POST | `/api/v1/cfip/seed` | Signed in | Initialize default data |
| GET | `/api/v1/cfip/sources` | Signed in | Data source list |
| POST | `/api/v1/cfip/sources` | Signed in | Create data source |
| DELETE | `/api/v1/cfip/sources/:id` | Signed in | Delete data source |
| PUT | `/api/v1/cfip/sources/:id` | Signed in | Update data source |
| POST | `/api/v1/cfip/sources/:id/fetch` | Signed in | Fetch this source |

### Subscription config (Mihomo/Clash)

Subscription template configuration: groups, rules, rule-providers, YAML preview.

| Method | Path | Permission | Description |
|---|---|---|---|
| GET | `/api/v1/mihomo/configs` | Signed in | List |
| POST | `/api/v1/mihomo/configs` | Signed in | Create |
| DELETE | `/api/v1/mihomo/configs/:id` | Signed in | Delete |
| GET | `/api/v1/mihomo/configs/:id` | Signed in | Details |
| PUT | `/api/v1/mihomo/configs/:id` | Signed in | Update |
| GET | `/api/v1/mihomo/configs/:id/proxy-groups` | Signed in | Group list |
| POST | `/api/v1/mihomo/configs/:id/proxy-groups` | Signed in | Create group |
| GET | `/api/v1/mihomo/configs/:id/rule-providers` | Signed in | Rule provider list |
| POST | `/api/v1/mihomo/configs/:id/rule-providers` | Signed in | Create rule provider |
| GET | `/api/v1/mihomo/configs/:id/rules` | Signed in | Rule list |
| POST | `/api/v1/mihomo/configs/:id/rules` | Signed in | Create rule |
| GET | `/api/v1/mihomo/configs/:id/yaml` | Signed in | Preview generated YAML |
| DELETE | `/api/v1/mihomo/proxy-groups/:id` | Signed in | Delete group |
| DELETE | `/api/v1/mihomo/rule-providers/:id` | Signed in | Delete rule provider |
| DELETE | `/api/v1/mihomo/rules/:id` | Signed in | Delete rule |

### Recharge sites

Third-party payment site integration: signing keys, verification files, domain verification, integration docs.

| Method | Path | Permission | Description |
|---|---|---|---|
| GET | `/api/v1/charge-sites` | Signed in | Site list |
| POST | `/api/v1/charge-sites` | Signed in | Create site |
| DELETE | `/api/v1/charge-sites/:id` | Signed in | Delete site |
| GET | `/api/v1/charge-sites/:id` | Signed in | Site details |
| PUT | `/api/v1/charge-sites/:id` | Signed in | Update site |
| POST | `/api/v1/charge-sites/:id/clear-prev-secret` | Signed in | Clear old key |
| POST | `/api/v1/charge-sites/:id/generate-verify` | Signed in | Generate verification file |
| POST | `/api/v1/charge-sites/:id/generate-verify-file` | Signed in | Generate verification file |
| GET | `/api/v1/charge-sites/:id/integration-doc` | Signed in | Get integration doc |
| POST | `/api/v1/charge-sites/:id/rotate-secret` | Signed in | Rotate signing key |
| POST | `/api/v1/charge-sites/:id/verify-dns` | Signed in | Verify domain DNS |
| POST | `/api/v1/charge-sites/:id/verify-domain` | Signed in | Verify domain HTTP challenge |
| POST | `/api/v1/charge-sites/:id/verify-domain-dns` | Signed in | Verify domain DNS |
| POST | `/api/v1/charge-sites/:id/verify-domain-http` | Signed in | Verify domain HTTP challenge |

### Recharge orders

Recharge order queries, refunds, CSV export.

| Method | Path | Permission | Description |
|---|---|---|---|
| GET | `/api/v1/charge-orders` | Signed in | Order list |
| GET | `/api/v1/charge-orders/:id` | Signed in | Order details |
| POST | `/api/v1/charge-orders/:id/refund` | Signed in | Refund |
| GET | `/api/v1/charge-orders/export-csv` | Signed in | Export recharge orders CSV |

### Recharge cards

Bulk card generation, usage records, CSV export.

| Method | Path | Permission | Description |
|---|---|---|---|
| GET | `/api/v1/charge-cards` | Signed in | Card list |
| POST | `/api/v1/charge-cards` | Signed in | Create card |
| DELETE | `/api/v1/charge-cards/:id` | Signed in | Delete card |
| GET | `/api/v1/charge-cards/:id/usages` | Signed in | Card usage records |
| POST | `/api/v1/charge-cards/batch` | Signed in | Bulk generate cards |
| GET | `/api/v1/charge-cards/export-csv` | Signed in | Export cards CSV |

### Charge callback events

Callback event audit.

| Method | Path | Permission | Description |
|---|---|---|---|
| GET | `/api/v1/charge-callback-events` | Signed in | Callback event list |

### Charge callback retries

Manual retry of failed callbacks.

| Method | Path | Permission | Description |
|---|---|---|---|
| GET | `/api/v1/charge-callback-retries` | Signed in | Failed callback list |
| POST | `/api/v1/charge-callback-retries/:id/retry` | Admin | Retry callback |

### Customer balance

Admin view of balances and transactions.

| Method | Path | Permission | Description |
|---|---|---|---|
| GET | `/api/v1/client-balances` | Signed in | Customer balance list |
| GET | `/api/v1/client-balances/transactions` | Signed in | Balance transactions (admin) |
| GET | `/api/v1/client-balances/transactions/export-csv` | Signed in | Export balance transactions CSV |

### Gifts

Issue claimable gifts (days/traffic).

| Method | Path | Permission | Description |
|---|---|---|---|
| GET | `/api/v1/client-gifts` | Signed in | Gift list |
| POST | `/api/v1/client-gifts` | Signed in | Create gift |
| POST | `/api/v1/client-gifts/batch` | Signed in | Bulk create gifts |
| GET | `/api/v1/client-gifts/search-clients` | Signed in | Search customers |

### Referral config

Referral commission configuration and statistics.

| Method | Path | Permission | Description |
|---|---|---|---|
| GET | `/api/v1/referral/config` | Signed in | Referral config |
| PUT | `/api/v1/referral/config` | Admin | Save referral config |
| GET | `/api/v1/referral/records` | Signed in | Referral record list |
| GET | `/api/v1/referral/stats` | Signed in | Referral stats |

### Referral withdrawals

Review withdrawal requests.

| Method | Path | Permission | Description |
|---|---|---|---|
| GET | `/api/v1/referral-withdrawals` | Signed in | Withdrawal request list |
| POST | `/api/v1/referral-withdrawals/:id/approve` | Signed in | Approve withdrawal |
| POST | `/api/v1/referral-withdrawals/:id/reject` | Signed in | Reject withdrawal |

### IP allow-lists (summary)

IP allow-list view across all customers.

| Method | Path | Permission | Description |
|---|---|---|---|
| GET | `/api/v1/ip-whitelists` | Signed in | All customer IP allow-lists |

### Customer OAuth bindings

Query customer third-party account bindings.

| Method | Path | Permission | Description |
|---|---|---|---|
| GET | `/api/v1/client-oauth-bindings` | Signed in | Customer OAuth binding list |

### Tickets (admin)

Ticket reply, assignment and closure.

| Method | Path | Permission | Description |
|---|---|---|---|
| GET | `/api/v1/tickets` | Signed in | Ticket list |
| GET | `/api/v1/tickets/:id` | Signed in | Ticket details |
| POST | `/api/v1/tickets/:id/assign` | Signed in | Assign ticket |
| POST | `/api/v1/tickets/:id/close` | Signed in | Close ticket |
| POST | `/api/v1/tickets/:id/reply` | Signed in | Reply to ticket |

### Articles

CRUD for portal articles.

| Method | Path | Permission | Description |
|---|---|---|---|
| GET | `/api/v1/articles` | Signed in | Article list |
| POST | `/api/v1/articles` | Signed in | Create article |
| DELETE | `/api/v1/articles/:id` | Signed in | Delete article |
| GET | `/api/v1/articles/:id` | Signed in | Article details |
| PUT | `/api/v1/articles/:id` | Signed in | Update article |

### Shared accounts

Issuance and repair records for shared accounts (streaming, etc.).

| Method | Path | Permission | Description |
|---|---|---|---|
| GET | `/api/v1/shared-accounts` | Signed in | Shared account list |
| POST | `/api/v1/shared-accounts` | Signed in | Create shared account |
| DELETE | `/api/v1/shared-accounts/:id` | Signed in | Delete shared account |
| GET | `/api/v1/shared-accounts/:id` | Signed in | Shared account details |
| PUT | `/api/v1/shared-accounts/:id` | Signed in | Update shared account |
| GET | `/api/v1/shared-accounts/:id/repair-logs` | Signed in | Repair records |

### Certificates

Public certificate (ACME) issuance, upload, renewal and export; issuance on a specific node is supported.

| Method | Path | Permission | Description |
|---|---|---|---|
| GET | `/api/v1/certificates` | Signed in | List |
| POST | `/api/v1/certificates` | Signed in | Create |
| DELETE | `/api/v1/certificates/:id` | Signed in | Delete |
| GET | `/api/v1/certificates/:id` | Signed in | Details |
| PUT | `/api/v1/certificates/:id` | Signed in | Update |
| GET | `/api/v1/certificates/:id/download` | Signed in | Download |
| POST | `/api/v1/certificates/:id/renew` | Signed in | Renew customer |
| GET | `/api/v1/certificates/check-port` | Signed in | Check port |
| GET | `/api/v1/certificates/detect-ip` | Signed in | Probe IP |
| GET | `/api/v1/certificates/export` | Signed in | Export |
| POST | `/api/v1/certificates/issue` | Signed in | Issue. body: `{"domains":["example.com","*.example.com"],"challenge_type":"dns-01","dns_channel_id":2,"key_type":"rsa","key_bits":4096}`. `challenge_type` defaults to http-01; `dns_channel_id` selects a DNS channel (a configured DNS provider + credentials) |
| POST | `/api/v1/certificates/issue-on-node` | Signed in | Issue to node |
| POST | `/api/v1/certificates/upload` | Signed in | Upload |

### Certificate channels

ACME channel (CA) configuration.

| Method | Path | Permission | Description |
|---|---|---|---|
| GET | `/api/v1/cert-channels` | Signed in | List |
| POST | `/api/v1/cert-channels` | Signed in | Create |
| DELETE | `/api/v1/cert-channels/:id` | Signed in | Delete |
| PUT | `/api/v1/cert-channels/:id` | Signed in | Update |
| GET | `/api/v1/cert-channels/providers` | Signed in | Provider |

### EAB configuration

Credentials binding external accounts such as ZeroSSL / Google.

| Method | Path | Permission | Description |
|---|---|---|---|
| GET | `/api/v1/eab-configs` | Signed in | List |
| POST | `/api/v1/eab-configs` | Signed in | Create |
| DELETE | `/api/v1/eab-configs/:id` | Signed in | Delete |
| PUT | `/api/v1/eab-configs/:id` | Signed in | Update |
| POST | `/api/v1/eab-configs/fetch-google` | Signed in | Fetch Google EAB from GCP |
| POST | `/api/v1/eab-configs/fetch-zerossl` | Signed in | Fetch ZeroSSL EAB with an email in one click |

### ACME accounts

Panel-side ACME account (contact email) management.

| Method | Path | Permission | Description |
|---|---|---|---|
| GET | `/api/v1/acme-accounts` | Signed in | List |
| POST | `/api/v1/acme-accounts` | Signed in | Create |
| DELETE | `/api/v1/acme-accounts/:id` | Signed in | Delete |
| PUT | `/api/v1/acme-accounts/:id` | Signed in | Update |
| GET | `/api/v1/acme-accounts/default` | Signed in | Get defaults |

### Built-in CA

Internal panel CA: Agent certificate issuance/renewal/revocation, CRL export, distribution queue.

| Method | Path | Permission | Description |
|---|---|---|---|
| GET | `/api/v1/ca/agent-certs` | Signed in | Agent certificate list |
| POST | `/api/v1/ca/agent-certs/:server_id/renew` | Signed in | Renew Agent certificate |
| GET | `/api/v1/ca/distribution-queue` | Signed in | Certificate distribution queue |
| DELETE | `/api/v1/ca/distribution-queue/:id` | Signed in | Delete distribution queue entry |
| POST | `/api/v1/ca/distribution-queue/:id/retry` | Signed in | Retry certificate distribution |
| GET | `/api/v1/ca/revocations` | Signed in | Revocation list |
| GET | `/api/v1/ca/revocations/export` | Signed in | Export CRL |
| GET | `/api/v1/ca/status` | Signed in | CA status |

### Geo rules

Sync and delivery of geoip/geosite data sources and files.

| Method | Path | Permission | Description |
|---|---|---|---|
| GET | `/api/v1/geo-rules/files` | Signed in | File list |
| DELETE | `/api/v1/geo-rules/files/:fileId` | Signed in | Delete file |
| PUT | `/api/v1/geo-rules/files/:fileId/auto-sync` | Signed in | Toggle auto sync |
| POST | `/api/v1/geo-rules/files/:fileId/download` | Signed in | Download |
| GET | `/api/v1/geo-rules/files/:fileId/download-file` | Signed in | Download file |
| GET | `/api/v1/geo-rules/github-assets` | Signed in | Fetch GitHub repository assets |
| POST | `/api/v1/geo-rules/push-all` | Signed in | Push to all nodes |
| POST | `/api/v1/geo-rules/seed` | Signed in | Initialize default data |
| GET | `/api/v1/geo-rules/sources` | Signed in | Data source list |
| POST | `/api/v1/geo-rules/sources` | Signed in | Create data source |
| DELETE | `/api/v1/geo-rules/sources/:sourceId` | Signed in | Delete data source |
| PUT | `/api/v1/geo-rules/sources/:sourceId` | Signed in | Update data source |
| GET | `/api/v1/geo-rules/sources/:sourceId/assets` | Signed in | Fetch Release assets |
| POST | `/api/v1/geo-rules/sources/:sourceId/files` | Signed in | Add file |
| PUT | `/api/v1/geo-rules/sources/:sourceId/schedule` | Signed in | Toggle scheduled sync |
| POST | `/api/v1/geo-rules/sources/:sourceId/select` | Signed in | Select file |
| POST | `/api/v1/geo-rules/sources/:sourceId/sync` | Signed in | Sync this data source |
| GET | `/api/v1/geo-rules/status` | Signed in | Status |
| POST | `/api/v1/geo-rules/sync-all` | Signed in | Sync all |
| GET | `/api/v1/geo-rules/sync-status/:key` | Signed in | Sync progress |

### WAF rule library

Global WAF rules: GitHub source, sync, delivery to all nodes.

| Method | Path | Permission | Description |
|---|---|---|---|
| GET | `/api/v1/waf/github-source` | Signed in | GitHub data source |
| PUT | `/api/v1/waf/github-source` | Signed in | Save GitHub data source |
| POST | `/api/v1/waf/push-all` | Signed in | Push to all nodes |
| GET | `/api/v1/waf/rules` | Signed in | Rule list |
| DELETE | `/api/v1/waf/rules/:name` | Signed in | Delete rule |
| GET | `/api/v1/waf/rules/:name` | Signed in | Get rule |
| PUT | `/api/v1/waf/rules/:name` | Signed in | Save rule |
| POST | `/api/v1/waf/sync-all` | Signed in | Sync all |
| POST | `/api/v1/waf/sync-github` | Signed in | Sync from GitHub |

### Log management

Node/core log reading and streaming, cleanup policies, log writer control.

| Method | Path | Permission | Description |
|---|---|---|---|
| POST | `/api/v1/log-manage/cleanup-all` | Signed in | Clean up everything now |
| GET | `/api/v1/log-manage/cleanup-rules` | Signed in | Cleanup rule list |
| PUT | `/api/v1/log-manage/cleanup-rules/:id` | Signed in | Update cleanup rule |
| POST | `/api/v1/log-manage/cleanup-table/:table` | Signed in | Clean up the specified table now |
| GET | `/api/v1/log-manage/core-log-output` | Signed in | Core log output config |
| PUT | `/api/v1/log-manage/core-log-output` | Signed in | Save core log output config |
| GET | `/api/v1/log-manage/core/:type/stdout-log` | Signed in | Read core stdout logs |
| GET | `/api/v1/log-manage/core/:type/stdout-stream` | Signed in | Stream core stdout |
| POST | `/api/v1/log-manage/discover` | Signed in | Discover node log files |
| GET | `/api/v1/log-manage/log-output` | Signed in | Log output config |
| PUT | `/api/v1/log-manage/log-output` | Signed in | Save log output config |
| POST | `/api/v1/log-manage/log-output/release/:type` | Signed in | Release log stdout takeover |
| POST | `/api/v1/log-manage/log-output/takeover/:type` | Signed in | Take over log stdout |
| GET | `/api/v1/log-manage/nodes/:serverId/logs` | Signed in | Node log file list |
| GET | `/api/v1/log-manage/nodes/:serverId/logs/:type` | Signed in | Read node logs |
| POST | `/api/v1/log-manage/nodes/:serverId/logs/:type/rotate` | Signed in | Rotate node logs |
| GET | `/api/v1/log-manage/nodes/:serverId/logs/:type/stream` | Signed in | Stream node logs |
| GET | `/api/v1/log-manage/policies` | Signed in | Policy list |
| POST | `/api/v1/log-manage/policies` | Signed in | Create policy |
| DELETE | `/api/v1/log-manage/policies/:id` | Signed in | Delete policy |
| GET | `/api/v1/log-manage/policies/:id` | Signed in | Policy details |
| PUT | `/api/v1/log-manage/policies/:id` | Signed in | Update policy |
| GET | `/api/v1/log-manage/policies/:id/jobs` | Signed in | Cleanup job list |
| GET | `/api/v1/log-manage/policies/:id/log` | Signed in | Read logs |
| GET | `/api/v1/log-manage/policies/:id/log-stream` | Signed in | Stream logs |
| POST | `/api/v1/log-manage/policies/:id/rotate` | Signed in | Rotate |
| GET | `/api/v1/log-manage/proxy/:type/stdout-log` | Signed in | Read core stdout logs |
| GET | `/api/v1/log-manage/proxy/:type/stdout-stream` | Signed in | Stream core stdout |
| GET | `/api/v1/log-manage/scheduler-config` | Signed in | Scheduling config |
| PUT | `/api/v1/log-manage/scheduler-config` | Signed in | Update scheduling config |
| GET | `/api/v1/log-manage/status` | Signed in | Status |
| POST | `/api/v1/log-manage/sync` | Signed in | Sync cleanup policies |
| GET | `/api/v1/log-manage/writers` | Signed in | Writer status |
| PUT | `/api/v1/log-manage/writers/:type/config` | Signed in | Update writer config |
| GET | `/api/v1/log-manage/writers/:type/files` | Signed in | Writer log file list |
| POST | `/api/v1/log-manage/writers/:type/start` | Signed in | Start writer |
| POST | `/api/v1/log-manage/writers/:type/stop` | Signed in | Stop writer |

### Backups

Database/full backups, restore, scheduled jobs, FTP/S3/WebDAV remote storage.

| Method | Path | Permission | Description |
|---|---|---|---|
| DELETE | `/api/v1/backups/:id` | Admin | Delete backup |
| GET | `/api/v1/backups/:id/download` | Admin | Download backup |
| POST | `/api/v1/backups/:id/restore` | Admin | Restore from backup |
| POST | `/api/v1/backups/:id/upload-ftp` | Admin | Upload to FTP |
| POST | `/api/v1/backups/:id/upload-s3` | Admin | Upload to S3 |
| POST | `/api/v1/backups/:id/upload-webdav` | Admin | Upload to WebDAV |
| POST | `/api/v1/backups/create` | Admin | Create full backup (with DB dump) |
| GET | `/api/v1/backups/list` | Admin | Full backup list |
| POST | `/api/v1/backups/remote-restore` | Admin | Restore from remote |
| GET | `/api/v1/backups/restore-status` | Admin | Restore progress |
| GET | `/api/v1/backups/schedules` | Admin | Scheduled job list |
| POST | `/api/v1/backups/schedules` | Admin | Create scheduled job |
| DELETE | `/api/v1/backups/schedules/:id` | Admin | Delete scheduled job |
| PUT | `/api/v1/backups/schedules/:id` | Admin | Update scheduled job |
| GET | `/api/v1/backups/storages` | Admin | Storage config list |
| POST | `/api/v1/backups/storages` | Admin | Create storage config |
| DELETE | `/api/v1/backups/storages/:id` | Admin | Delete storage config |
| PUT | `/api/v1/backups/storages/:id` | Admin | Update storage config |
| GET | `/api/v1/backups/storages/:id/files` | Admin | Remote backup file list |
| POST | `/api/v1/backups/upload-restore` | Admin | Upload backup and restore |

### System settings

Database config and migration, panel settings, JWT keys, SMTP/Telegram tests, notification templates, online upgrades (mostly admin).

| Method | Path | Permission | Description |
|---|---|---|---|
| PUT | `/api/v1/system/bootstrap-finish` | Admin | Complete first-boot setup |
| GET | `/api/v1/system/bootstrap-state` | Admin | First-boot status |
| GET | `/api/v1/system/config-file/:coreType` | Admin | Read core config file |
| GET | `/api/v1/system/db-config` | Admin | Database config |
| PUT | `/api/v1/system/db-config` | Admin | Update database config |
| POST | `/api/v1/system/db-config/migrate` | Admin | Migrate database data |
| POST | `/api/v1/system/db-config/switch` | Admin | Switch database (with one-step migration) |
| POST | `/api/v1/system/db-config/test` | Admin | Test database connection |
| GET | `/api/v1/system/db-pool-stats` | Admin | Connection pool statistics |
| GET | `/api/v1/system/db-stats` | Admin | Database statistics |
| GET | `/api/v1/system/db-stats/ping` | Admin | Database probe |
| POST | `/api/v1/system/db-stats/reset` | Admin | Reset database statistics |
| GET | `/api/v1/system/encryption-key-status` | Admin | Encryption key status |
| GET | `/api/v1/system/env-config` | Admin | .env configuration |
| PUT | `/api/v1/system/env-config` | Admin | Update .env configuration |
| POST | `/api/v1/system/generate-encryption-key` | Admin | Generate encryption key |
| GET | `/api/v1/system/github-config` | Admin | GitHub configuration |
| PUT | `/api/v1/system/github-config` | Admin | Update GitHub config |
| GET | `/api/v1/system/jwt-secret-status` | Admin | JWT key status |
| GET | `/api/v1/system/maintenance` | Public | Maintenance mode status (public) |
| PUT | `/api/v1/system/maintenance` | Admin | Toggle maintenance mode |
| POST | `/api/v1/system/migrate-data` | Admin | Migrate data |
| GET | `/api/v1/system/notification-config` | Admin | Notification config |
| PUT | `/api/v1/system/notification-config` | Admin | Update notification config |
| GET | `/api/v1/system/notification-templates` | Admin | Template list |
| POST | `/api/v1/system/notification-templates` | Admin | Create template |
| DELETE | `/api/v1/system/notification-templates/:id` | Admin | Delete template |
| PUT | `/api/v1/system/notification-templates/:id` | Admin | Update template |
| POST | `/api/v1/system/notification-templates/:id/broadcast` | Admin | Broadcast notification by template |
| POST | `/api/v1/system/notification-templates/:id/send` | Admin | Send test notification |
| GET | `/api/v1/system/panel-config` | Admin | Panel config |
| PUT | `/api/v1/system/panel-config` | Admin | Update panel config |
| GET | `/api/v1/system/reality-targets` | Admin | REALITY target list |
| PUT | `/api/v1/system/reality-targets` | Admin | Update REALITY target |
| POST | `/api/v1/system/reality-targets/reset` | Admin | Reset REALITY target to default |
| POST | `/api/v1/system/regenerate-jwt-secret` | Admin | Regenerate JWT key |
| GET | `/api/v1/system/releases` | Admin | Available versions |
| POST | `/api/v1/system/restart` | Admin | Restart |
| POST | `/api/v1/system/rollback` | Admin | System rollback |
| GET | `/api/v1/system/settings` | Admin | Read settings |
| PUT | `/api/v1/system/settings/batch` | Admin | Bulk update settings |
| GET | `/api/v1/system/stats` | Admin | System status |
| GET | `/api/v1/system/subscription-billing-templates` | Admin | Subscription billing template |
| PUT | `/api/v1/system/subscription-billing-templates` | Admin | Update subscription billing template |
| POST | `/api/v1/system/telegram-chat-id` | Admin | Get Telegram Chat ID |
| POST | `/api/v1/system/test-smtp` | Admin | Test SMTP |
| POST | `/api/v1/system/test-telegram` | Admin | Test Telegram |
| POST | `/api/v1/system/update` | Admin | System upgrade |
| GET | `/api/v1/system/update-jobs` | Admin | Upgrade job list |
| POST | `/api/v1/system/upload-mtls-ca` | Admin | Upload mTLS CA certificate |

### Security audit

Audit event queries and export.

| Method | Path | Permission | Description |
|---|---|---|---|
| GET | `/api/v1/security/audit-events` | Signed in | Audit event list |
| GET | `/api/v1/security/audit-events/export` | Signed in | Export audit events |

### Metrics

Panel aggregated metrics (nodes/certificates/services).

| Method | Path | Permission | Description |
|---|---|---|---|
| GET | `/api/v1/metrics/certificates` | Signed in | Certificate metrics |
| GET | `/api/v1/metrics/nodes` | Signed in | Node metrics |
| GET | `/api/v1/metrics/overview` | Signed in | Overview |

### Portal templates

Portal templates, activation config and cache management.

| Method | Path | Permission | Description |
|---|---|---|---|
| GET | `/api/v1/portal/active` | Signed in | Active template |
| PUT | `/api/v1/portal/active` | Signed in | Update active template |
| DELETE | `/api/v1/portal/cache` | Signed in | Clear portal cache |
| GET | `/api/v1/portal/cache` | Signed in | Cache status |
| PUT | `/api/v1/portal/cache/dev-mode` | Signed in | Toggle cache development mode |
| POST | `/api/v1/portal/cache/rebuild` | Signed in | Rebuild portal cache |
| POST | `/api/v1/portal/reset` | Signed in | Restore default template |
| GET | `/api/v1/portal/templates` | Signed in | List |
| POST | `/api/v1/portal/templates` | Signed in | Create |
| DELETE | `/api/v1/portal/templates/:id` | Signed in | Delete |
| GET | `/api/v1/portal/templates/:id` | Signed in | Details |
| PUT | `/api/v1/portal/templates/:id` | Signed in | Update |
| POST | `/api/v1/portal/templates/:id/activate` | Signed in | Activate template |

> `/portal/templates` CRUD is an **advanced API**: the portal UI primarily uses `PUT /api/v1/portal/active`
> (one-shot full-site config); manage/switch multiple templates with this group. Both are supported and do not conflict.

### Agent installation

Node Agent install script and download (public).

| Method | Path | Permission | Description |
|---|---|---|---|
| POST | `/api/v1/agent/bootstrap` | Public | Agent first-boot registration (Agent reports with a bootstrap token; public) |
| GET | `/api/v1/agent/releases` | Admin | Available Agent versions (for the upgrade page) |
| GET | `/api/v1/agent/version` | Admin | Current Agent version on the panel |
| GET | `/api/v1/agent/download` | Public | Download Agent |
| GET | `/api/v1/agent/install` | Public | Get install script |

### Charge callback entry

Third-party payment platform callback (public, signature verified).

| Method | Path | Permission | Description |
|---|---|---|---|
| POST | `/api/v1/charge/callback/:site_id` | Public | Handle charge callback |

### Inbounds (with nodes)

Inbound list used for plan node selection.

| Method | Path | Permission | Description |
|---|---|---|---|
| GET | `/api/v1/inbounds-with-server` | Signed in | All inbounds (including their nodes) |

### Public config

Portal runtime config without sign-in (maintenance guard + public rate limit).

| Method | Path | Permission | Description |
|---|---|---|---|
| GET | `/api/v1/public/articles` | Public | Public article list |
| GET | `/api/v1/public/articles/:slug` | Public | Public article details |
| GET | `/api/v1/public/client-configs` | Public | Client download config (public) |
| GET | `/api/v1/public/portal-config` | Public | Portal runtime config (site name, theme, languages, compliance docs; public) |
| GET | `/api/v1/public/settings` | Public | Public settings (e.g. registration_enabled; public) |
| GET | `/api/v1/public/subscription-endpoints` | Public | Enabled subscription endpoint list (public) |

### Client auth and business (same set on the admin engine)

Client endpoints identical to the portal; see `portal-api.en.md` for details.

| Method | Path | Permission | Description |
|---|---|---|---|
| GET | `/api/v1/client-auth/articles` | Public | List articles |
| GET | `/api/v1/client-auth/articles/:slug` | Public | Get article |
| GET | `/api/v1/client-auth/balance` | Public | Get balance |
| GET | `/api/v1/client-auth/balance/transactions` | Public | Balance transactions |
| GET | `/api/v1/client-auth/bills` | Public | Order list |
| POST | `/api/v1/client-auth/change-password` | Public | Change password |
| POST | `/api/v1/client-auth/charge/create` | Public | Create recharge order |
| GET | `/api/v1/client-auth/charge/orders` | Public | List recharge orders |
| GET | `/api/v1/client-auth/charge/orders/:id` | Public | Get recharge order details |
| POST | `/api/v1/client-auth/charge/redeem-card` | Public | Redeem card |
| GET | `/api/v1/client-auth/charge/sites` | Public | List recharge sites |
| GET | `/api/v1/client-auth/charge/status/:trade_no` | Public | Query recharge status |
| GET | `/api/v1/client-auth/checktime` | Public | Get server time (clock sync) |
| GET | `/api/v1/client-auth/connection-info` | Public | Get plan connection parameters |
| GET | `/api/v1/client-auth/gifts` | Public | List gifts |
| POST | `/api/v1/client-auth/gifts/:id/claim` | Public | Claim gift |
| GET | `/api/v1/client-auth/invoices` | Public | Order list |
| GET | `/api/v1/client-auth/invoices/:id` | Public | Order details |
| GET | `/api/v1/client-auth/ip-whitelist` | Public | List IP allow-list |
| DELETE | `/api/v1/client-auth/ip-whitelist/:id` | Public | Remove from IP allow-list |
| POST | `/api/v1/client-auth/ip-whitelist/auto-add` | Public | Auto-add IP |
| POST | `/api/v1/client-auth/login` | Public | Sign in |
| POST | `/api/v1/client-auth/logout` | Public | Sign out |
| POST | `/api/v1/client-auth/logs/package` | Public | Create log bundle |
| GET | `/api/v1/client-auth/logs/packages` | Public | List log bundles |
| DELETE | `/api/v1/client-auth/logs/packages/:id` | Public | Delete log bundle |
| GET | `/api/v1/client-auth/logs/packages/:id` | Public | Get log bundle |
| GET | `/api/v1/client-auth/logs/packages/:id/download` | Public | Download log bundle |
| GET | `/api/v1/client-auth/logs/types` | Public | Log bundle health check |
| GET | `/api/v1/client-auth/me` | Public | Get current signed-in user |
| GET | `/api/v1/client-auth/nodes` | Public | My node list |
| GET | `/api/v1/client-auth/nodes/:id/config` | Public | Node config |
| GET | `/api/v1/client-auth/nodes/:id/link` | Public | Node share link |
| GET | `/api/v1/client-auth/nodes/:id/qrcode` | Public | Node QR code |
| GET | `/api/v1/client-auth/oauth/:provider` | Public | Authorize |
| GET | `/api/v1/client-auth/oauth/:provider/callback` | Public | Callback |
| GET | `/api/v1/client-auth/oauth/providers` | Public | Provider status |
| GET | `/api/v1/client-auth/orders` | Public | Order list |
| POST | `/api/v1/client-auth/orders` | Public | Create order |
| GET | `/api/v1/client-auth/orders/:id` | Public | Order details |
| GET | `/api/v1/client-auth/plans` | Public | Plan list |
| GET | `/api/v1/client-auth/plans/:id` | Public | Plan details |
| GET | `/api/v1/client-auth/profile` | Public | Get profile |
| PUT | `/api/v1/client-auth/profile` | Public | Update profile |
| POST | `/api/v1/client-auth/profile/bind-oauth` | Public | Bind third-party account |
| DELETE | `/api/v1/client-auth/profile/unbind-oauth/:provider` | Public | Unbind third-party account |
| GET | `/api/v1/client-auth/referral/code` | Public | Get referral verification code |
| GET | `/api/v1/client-auth/referral/records` | Public | List referral records |
| GET | `/api/v1/client-auth/referral/stats` | Public | Get referral stats |
| POST | `/api/v1/client-auth/referral/withdraw` | Public | Request withdrawal |
| GET | `/api/v1/client-auth/referral/withdrawals` | Public | List referral withdrawals |
| POST | `/api/v1/client-auth/refresh` | Public | Refresh |
| POST | `/api/v1/client-auth/register` | Public | Register |
| POST | `/api/v1/client-auth/reset-subscription` | Public | Reset subscription |
| POST | `/api/v1/client-auth/rotate-token` | Public | Rotate subscription token |
| POST | `/api/v1/client-auth/self-unban` | Public | Self-service unban |
| PUT | `/api/v1/client-auth/settings` | Public | Update settings |
| POST | `/api/v1/client-auth/settings/auto-add-ip` | Public | Auto-add IP |
| GET | `/api/v1/client-auth/settings/whitelist` | Public | List IP allow-list |
| DELETE | `/api/v1/client-auth/settings/whitelist/:id` | Public | Remove from IP allow-list |
| GET | `/api/v1/client-auth/shared-accounts` | Public | List shared accounts |
| GET | `/api/v1/client-auth/shared-accounts/:id` | Public | Get shared account |
| POST | `/api/v1/client-auth/shared-accounts/:id/apply` | Public | Request shared account |
| POST | `/api/v1/client-auth/shared-accounts/:id/repair` | Public | Repair request |
| POST | `/api/v1/client-auth/ticket` | Public | Issue one-time short-lived ticket |
| GET | `/api/v1/client-auth/tickets` | Public | List tickets |
| POST | `/api/v1/client-auth/tickets` | Public | Create ticket |
| GET | `/api/v1/client-auth/tickets/:id` | Public | Get ticket |
| POST | `/api/v1/client-auth/tickets/:id/close` | Public | Close ticket |
| POST | `/api/v1/client-auth/tickets/:id/reply` | Public | Reply to ticket |
| GET | `/api/v1/client-auth/traffic-history` | Public | Traffic history |
| GET | `/api/v1/client-auth/traffic-packs` | Public | Traffic pack list |
| POST | `/api/v1/client-auth/traffic-packs/buy` | Public | Buy traffic pack |

### Subscription delivery

Public entry point for clients to fetch subscriptions; see `subscription.en.md`.

| Method | Path | Permission | Description |
|---|---|---|---|
| GET | `/sub/:token` | Public | Serve subscription |

### Runtime configuration workflow

Config snapshots / dry-run / apply / rollback (`apply`, `rollback` and `disable-proxy` require admin).

| Method | Path | Permission | Description |
|---|---|---|---|
| POST | `/runtime/apply` | Public | Apply |
| GET | `/runtime/apply-jobs` | Signed in | List jobs |
| GET | `/runtime/apply-jobs/:id` | Signed in | Get job |
| POST | `/runtime/apply-jobs/:id/cancel` | Public | Cancel job |
| POST | `/runtime/apply-jobs/:id/retry` | Public | Retry job |
| POST | `/runtime/disable-proxy` | Public | Disable proxy |
| POST | `/runtime/dry-run` | Signed in | Dry-run execution |
| GET | `/runtime/nodes` | Signed in | List nodes |
| GET | `/runtime/overview` | Signed in | Overview |
| POST | `/runtime/rollback` | Public | Roll back |
| GET | `/runtime/snapshots` | Signed in | List snapshots |
| POST | `/runtime/snapshots` | Signed in | Create snapshot |
| DELETE | `/runtime/snapshots/:id` | Public | Delete snapshot |
| GET | `/runtime/snapshots/:id` | Signed in | Get snapshot |
| POST | `/runtime/verify` | Signed in | Verify |

### Service access policies (retired)

> `/services/:id/access-policies` and `/services/:id/ip-bans` were retired on 2026-10-20:
> there are no admin calls and no connection path consumes these policies.

### Traffic quota monitoring

Read-only monitoring view of quotas and usage.

| Method | Path | Permission | Description |
|---|---|---|---|
| GET | `/traffic/quotas` | Signed in | Quota list |
| GET | `/traffic/quotas/:id/usage` | Signed in | Quota usage |
| GET | `/traffic/reset-logs` | Signed in | Quota reset records |
| GET | `/traffic/usage-history` | Signed in | Usage history |

### Health checks

Liveness probe.

| Method | Path | Permission | Description |
|---|---|---|---|
| GET | `/health` | Public | Liveness probe; 200 means healthy |

### Portal page

Portal SPA page.

| Method | Path | Permission | Description |
|---|---|---|---|
| GET | `/p` | Public | Portal HTML (server-rendered + Vue runtime) |

### ACME challenges

HTTP-01 challenge callback.

| Method | Path | Permission | Description |
|---|---|---|---|
| GET | `/.well-known/acme-challenge/:token` | Public | ACME HTTP-01 challenge callback (verifies the domain during issuance) |
