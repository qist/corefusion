<!-- source: THIRD_PARTY_LICENSES.md | locale: en | updated: 2026-11-19 -->
# Third-Party Components and Licenses

> Languages: [简体中文](THIRD_PARTY_LICENSES.md) · English · [繁體中文](THIRD_PARTY_LICENSES.zh-TW.md) · [Русский](THIRD_PARTY_LICENSES.ru.md) · [日本語](THIRD_PARTY_LICENSES.ja.md) · [한국어](THIRD_PARTY_LICENSES.ko.md) · [Deutsch](THIRD_PARTY_LICENSES.de.md) · [Français](THIRD_PARTY_LICENSES.fr.md) · [Español](THIRD_PARTY_LICENSES.es.md) · [Português](THIRD_PARTY_LICENSES.pt.md) · [Türkçe](THIRD_PARTY_LICENSES.tr.md) · [Bahasa Indonesia](THIRD_PARTY_LICENSES.id.md) · [ไทย](THIRD_PARTY_LICENSES.th.md) · [Tiếng Việt](THIRD_PARTY_LICENSES.vi.md) · [العربية](THIRD_PARTY_LICENSES.ar.md) · [فارسی](THIRD_PARTY_LICENSES.fa.md)

CoreFusion uses the following open-source components. This file lists the main components and licenses; **the complete list is defined by `go.mod` / `go.sum`, `web/admin/package.json` and each module's own LICENSE file**.

When distributing binaries of this project, keep this file, the project `LICENSE`, and the copyright/license notices of all third-party components.

## 1. Go dependencies compiled into the binary (main)

| Component | License | Purpose |
|---|---|---|
| `github.com/xtls/xray-core` | MPL-2.0 | Xray config parsing, gRPC commands, protocol definitions |
| `github.com/xtls/reality` | MPL-2.0 | REALITY protocol support |
| `github.com/sagernet/sing` | **GPL-3.0-or-later** | Indirectly linked via xray-core |
| `github.com/sagernet/sing-shadowsocks` | **GPL-3.0-or-later** | Indirectly linked via xray-core (Shadowsocks-2022) |
| `github.com/go-sql-driver/mysql` | MPL-2.0 | MySQL driver |
| `github.com/hashicorp/golang-lru/v2` | MPL-2.0 | LRU cache |
| `github.com/go-acme/lego/v5` | MIT | ACME certificate issuance |
| `github.com/gin-gonic/gin` | MIT | HTTP framework |
| `gorm.io/gorm` | MIT | ORM |
| `gorm.io/driver/mysql` | MIT | MySQL driver for GORM |
| `gorm.io/driver/postgres` | MIT | PostgreSQL driver for GORM |
| `github.com/jackc/pgx/v5` | MIT | PostgreSQL driver |
| `github.com/qist/sqlite` | MIT | SQLite driver (default database for panel / Agent) |
| `github.com/aws/aws-sdk-go-v2` | Apache-2.0 | S3 backups, Route53 DNS-01 |
| `google.golang.org/grpc` | Apache-2.0 | Agent gRPC communication |
| `google.golang.org/protobuf` | BSD-3-Clause | Protobuf runtime |
| `golang.org/x/crypto` | BSD-3-Clause | Cryptographic primitives |
| `golang.org/x/oauth2` | BSD-3-Clause | OAuth2 / Google EAB |
| `golang.org/x/sys` | BSD-3-Clause | System calls (nft/tc rate limiting, process management) |
| `github.com/golang-jwt/jwt/v5` | MIT | JWT |
| `github.com/google/uuid` | BSD-3-Clause | UUID |
| `github.com/go-webauthn/webauthn` | BSD-3-Clause | Passkey / WebAuthn sign-in |
| `github.com/pquerna/otp` | Apache-2.0 | TOTP two-factor authentication |
| `github.com/jlaffaye/ftp` | ISC | FTP backup upload |
| `gopkg.in/yaml.v3` | MIT / Apache-2.0 | YAML config parsing |

> ⚠ **GPL-3.0 components**: `sing` and `sing-shadowsocks` are GPL-3.0-or-later. They are statically linked into the panel and Agent binaries, so **distributing binaries requires providing the Corresponding Source** (see section 3 of [LEGAL.md](./LEGAL.md)).

## 2. Frontend dependencies (compiled into the panel binary)

The admin UI is built with Vite and embedded into the panel binary via `go:embed`, so the following components are distributed with the binary as well:

| Component | License | Purpose |
|---|---|---|
| Vue 3 / Vue Router / Pinia / Element Plus / axios / vue-i18n / marked / js-yaml / qrcode / Monaco Editor | MIT | Admin UI, routing and state, i18n, Markdown rendering, QR codes and code editor |
| DOMPurify | Apache-2.0 / MPL-2.0 | XSS sanitization before rendering Markdown / HTML |

## 3. Standalone programs downloaded / invoked by the panel at runtime

When installing or upgrading cores, the panel / Agent downloads the following binaries from GitHub and runs them as separate processes. **Artifacts marked "built by this project" are compiled and published by this organization from upstream source** (`qist/*-binaries`) and are distributed together with the product, so the corresponding license obligations (including source availability for GPL) rest with this project; the others are download URLs of official upstream releases.

| Program | License | Notes |
|---|---|---|
| Xray-core | MPL-2.0 | Proxy core, separate process; official upstream release (`XTLS/Xray-core`) |
| sing-box | **GPL-3.0-or-later** | Proxy core, separate process; **built by this project** (`qist/singbox-binaries`; official releases lack `with_v2ray_api`, which is required for per-user traffic accounting); provide corresponding source when distributing |
| Caddy | Apache-2.0 | Reverse proxy / TLS; **built by this project** (`qist/caddy-binaries`) |
| Nginx | BSD-2-Clause | Reverse proxy; **built by this project** (`qist/nginx-binaries`, split by glibc / musl) |

> Hysteria2 and TUIC are **protocols carried by sing-box**, not standalone cores or binaries, so they are not listed separately.

## 4. Data files

| Data | Source / license |
|---|---|
| `geoip.dat` / `geosite.dat` | As declared by the data source repository (commonly GPL-3.0 or CC-BY-SA) |
| GeoLite2 (`GeoLite2-ASN.mmdb`, etc.) | MaxMind GeoLite2 EULA / CC BY-SA 4.0; verify licensing before commercial use |
| `country.mmdb` | As declared by its data source |

## 5. Compliance recommendations

- When delivering binaries, include: `LICENSE`, this file, and the license text or acquisition URL of each GPL/MPL component.
- Binaries built by this project (sing-box / Caddy / Nginx) ship with the panel: since they are compiled from upstream source, publish how they were built (version, build tags, configure flags); GPL components (sing-box) additionally require the complete corresponding source of that version or an effective acquisition route.
- If you modify source files of MPL-2.0 components such as `xray-core`, publish the modified files' source.
- If you distribute GPL programs such as sing-box with your product, provide recipients with the complete corresponding source of that version (or an effective written offer).
- Trademarks and names: do not use upstream project names or logos in a way that implies official endorsement.
