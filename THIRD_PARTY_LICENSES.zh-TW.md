<!-- source: THIRD_PARTY_LICENSES.md | locale: zh-TW | updated: 2026-11-19 -->
# 第三方組件和許可證

> 語言： [简体中文](THIRD_PARTY_LICENSES.md) · [English](THIRD_PARTY_LICENSES.en.md) · 繁體中文 · [Русский](THIRD_PARTY_LICENSES.ru.md) · [日本語](THIRD_PARTY_LICENSES.ja.md) · [한국어](THIRD_PARTY_LICENSES.ko.md) · [Deutsch](THIRD_PARTY_LICENSES.de.md) · [Français](THIRD_PARTY_LICENSES.fr.md) · [Español](THIRD_PARTY_LICENSES.es.md) · [Português](THIRD_PARTY_LICENSES.pt.md) · [Türkçe](THIRD_PARTY_LICENSES.tr.md) · [Bahasa Indonesia](THIRD_PARTY_LICENSES.id.md) · [ไทย](THIRD_PARTY_LICENSES.th.md) · [Tiếng Việt](THIRD_PARTY_LICENSES.vi.md) · [العربية](THIRD_PARTY_LICENSES.ar.md) · [فارسی](THIRD_PARTY_LICENSES.fa.md)

CoreFusion 使用下列開源元件。該文件列出了主要組件和許可證； **完整清單由 `go.mod` / `go.sum`、`web/admin/package.json` 和每個模組自己的許可證文件定義**。

散佈此項目的二進位檔案時，請保留此檔案、專案 `LICENSE` 以及所有第三方元件的版權/授權聲明。

## 1. Go 依賴項編譯成二進位（主要）

| 組件 | 許可證 | 目的 |
|---|---|---|
| `github.com/xtls/xray-core` | MPL-2.0 | Xray 設定解析、gRPC 指令、協定定義 |
| `github.com/xtls/reality` | MPL-2.0 | REALITY協議支持 |
| `github.com/sagernet/sing` | **GPL-3.0-or-later** | 透過 xray-core 間接鏈接 |
| `github.com/sagernet/sing-shadowsocks` | **GPL-3.0-or-later** | 透過 xray-core (Shadowsocks-2022) 間接連結 |
| `github.com/go-sql-driver/mysql` | MPL-2.0 | MySQL驅動器 |
| `github.com/hashicorp/golang-lru/v2` | MPL-2.0 | LRU緩存 |
| `github.com/go-acme/lego/v5` | MIT | ACME證書頒發 |
| `github.com/gin-gonic/gin` | MIT | HTTP框架 |
| `gorm.io/gorm` | MIT | ORM |
| `gorm.io/driver/mysql` | MIT | MySQL GORM 驅動程式 |
| `gorm.io/driver/postgres` | MIT | PostgreSQL GORM 驅動程式 |
| `github.com/jackc/pgx/v5` | MIT | PostgreSQL驅動器 |
| `github.com/qist/sqlite` | MIT | SQLite驅動程式（面板/代理程式的預設資料庫） |
| `github.com/aws/aws-sdk-go-v2` | Apache-2.0 | S3 備份，Route53 DNS-01 |
| `google.golang.org/grpc` | Apache-2.0 | 代理 gRPC 通信 |
| `google.golang.org/protobuf` | BSD-3-Clause | Protobuf運行時 |
| `golang.org/x/crypto` | BSD-3-Clause | 密碼原語 |
| `golang.org/x/oauth2` | BSD-3-Clause | OAuth2 / Google EAB |
| `golang.org/x/sys` | BSD-3-Clause | 系統呼叫（nft/tc速率限制、進程管理） |
| `github.com/golang-jwt/jwt/v5` | MIT | JWT |
| `github.com/google/uuid` | BSD-3-Clause | UUID |
| `github.com/go-webauthn/webauthn` | BSD-3-Clause | Passkey / WebAuthn 登入 |
| `github.com/pquerna/otp` | Apache-2.0 | TOTP 雙因素認證 |
| `github.com/jlaffaye/ftp` | ISC | FTP 備份上傳 |
| `gopkg.in/yaml.v3` | MIT / Apache-2.0 | YAML設定解析 |

> ⚠ **GPL-3.0 元件**：`sing` 和 `sing-shadowsocks` 是 GPL-3.0 或更高版本。它們靜態連結到面板和代理二進位檔案中，因此**分發二進位檔案需要提供相應的來源**（請參閱 [LEGAL.md](./LEGAL.md) 的第 3 節）。

## 2. 前端相依性（編譯到面板二進位檔案）

管理 UI 是使用 Vite 建構的，並透過 `go:embed` 嵌入到面板二進位檔案中，因此以下元件也隨二進位檔案一起分發：

| 組件 | 許可證 | 目的 |
|---|---|---|
| Vue 3 / Vue Router / Pinia / Element Plus / axios / vue-i18n / marked / js-yaml / qrcode / Monaco Editor | MIT | 管理 UI、路由和狀態、i18n、Markdown 渲染、QR 程式碼和程式碼編輯器 |
| DOMPurify | Apache-2.0 / MPL-2.0 | 渲染 Markdown / HTML 之前的 XSS 清理 |

## 3. 在運行時調用的獨立程序（未在此存儲庫中分發）

面板/代理可以在運行時下載、安裝或呼叫以下獨立程式。他們透過流程/介面與該專案交互，但如果您將它們與可交付成果一起交付，您仍然必須遵守他們的許可證：

| 節目 | 許可證 | 註解 |
|---|---|---|
| Xray-core | MPL-2.0 | Proxy核心，獨立進程 |
| sing-box | **GPL-3.0-or-later** | Proxy核心，獨立進程；分發時提供對應來源 |
| Caddy | Apache-2.0 | 反向代理/TLS |
| Nginx | BSD-2-Clause | 反向代理（系統安裝，如果使用） |

> Hysteria2和TUIC是**sing-box攜帶的協定**，不是獨立的核心或二進位文件，因此沒有單獨列出。

## 4. 資料檔案

| 數據 | 來源/許可 |
|---|---|
| `geoip.dat` / `geosite.dat` | 由資料來源儲存庫聲明（通常為 GPL-3.0 或 CC-BY-SA） |
| GeoLite2（`GeoLite2-ASN.mmdb`等） | MaxMind GeoLite2 EULA / CC BY-SA 4.0；商業使用前驗證許可 |
| `country.mmdb` | 正如其資料來源所聲明的 |

## 5. 合規建議

- 交付二進位檔案時，請包含：`LICENSE`、此檔案以及每個 GPL/MPL 元件的授權文字或取得 URL。
- 如果您修改了MPL-2.0元件（例如`xray-core`）的原始文件，請發布修改後的文件的源代碼。
- 如果您隨產品分發 GPL 程式（例如 sing-box），請向接收者提供該版本的完整相應來源（或有效的書面報價）。
- 商標和名稱：請勿以暗示官方認可的方式使用上游項目名稱或標誌。
