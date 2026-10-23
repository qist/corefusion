<!-- source: THIRD_PARTY_LICENSES.md | locale: en | updated: 2026-10-23 -->
# Third-Party Components and Licenses

> Languages: [简体中文](THIRD_PARTY_LICENSES.md) · English · [繁體中文](THIRD_PARTY_LICENSES.zh-TW.md) · [Русский](THIRD_PARTY_LICENSES.ru.md) · [日本語](THIRD_PARTY_LICENSES.ja.md) · [한국어](THIRD_PARTY_LICENSES.ko.md) · [Deutsch](THIRD_PARTY_LICENSES.de.md) · [Français](THIRD_PARTY_LICENSES.fr.md) · [Español](THIRD_PARTY_LICENSES.es.md) · [Português](THIRD_PARTY_LICENSES.pt.md) · [Türkçe](THIRD_PARTY_LICENSES.tr.md) · [Bahasa Indonesia](THIRD_PARTY_LICENSES.id.md) · [ไทย](THIRD_PARTY_LICENSES.th.md) · [Tiếng Việt](THIRD_PARTY_LICENSES.vi.md) · [العربية](THIRD_PARTY_LICENSES.ar.md) · [فارسی](THIRD_PARTY_LICENSES.fa.md)

CoreFusion uses the following open-source components. This file lists the main components and licenses; **the complete list is defined by `go.mod` / `go.sum` and each module's own LICENSE file**.

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
| `github.com/go-acme/lego/v4` | MIT | ACME certificate issuance |
| `github.com/gin-gonic/gin` | MIT | HTTP framework |
| `gorm.io/gorm` | MIT | ORM |
| `github.com/aws/aws-sdk-go-v2` | Apache-2.0 | S3 backups, Route53 DNS-01 |
| `google.golang.org/grpc` | Apache-2.0 | Agent gRPC communication |
| `google.golang.org/protobuf` | BSD-3-Clause | Protobuf runtime |
| `golang.org/x/crypto` | BSD-3-Clause | Cryptographic primitives |
| `golang.org/x/oauth2` | BSD-3-Clause | OAuth2 / Google EAB |
| `github.com/golang-jwt/jwt/v5` | MIT | JWT |
| `github.com/google/uuid` | BSD-3-Clause | UUID |

> ⚠ **GPL-3.0 components**: `sing` and `sing-shadowsocks` are GPL-3.0-or-later. They are statically linked into the panel and Agent binaries, so **distributing binaries requires providing the Corresponding Source** (see section 3 of [LEGAL.md](./LEGAL.md)).

## 2. Standalone programs invoked at runtime (not distributed in this repository)

The panel / Agent may download, install or invoke the following standalone programs at runtime. They interact with this project through processes/interfaces, but if you ship them with your deliverables you must still comply with their licenses:

| Program | License | Notes |
|---|---|---|
| Xray-core | MPL-2.0 | Proxy core, separate process |
| sing-box | **GPL-3.0-or-later** | Proxy core, separate process; provide corresponding source when distributing |
| Caddy | Apache-2.0 | Reverse proxy / TLS |
| Nginx | BSD-2-Clause | Reverse proxy (system-installed, if used) |

> Hysteria2 and TUIC are **protocols carried by sing-box**, not standalone cores or binaries, so they are not listed separately.

## 3. Data files

| Data | Source / license |
|---|---|
| `geoip.dat` / `geosite.dat` | As declared by the data source repository (commonly GPL-3.0 or CC-BY-SA) |
| GeoLite2 (`GeoLite2-ASN.mmdb`, etc.) | MaxMind GeoLite2 EULA / CC BY-SA 4.0; verify licensing before commercial use |
| `country.mmdb` | As declared by its data source |

## 4. Compliance recommendations

- When delivering binaries, include: `LICENSE`, this file, and the license text or acquisition URL of each GPL/MPL component.
- If you modify source files of MPL-2.0 components such as `xray-core`, publish the modified files' source.
- If you distribute GPL programs such as sing-box with your product, provide recipients with the complete corresponding source of that version (or an effective written offer).
- Trademarks and names: do not use upstream project names or logos in a way that implies official endorsement.
