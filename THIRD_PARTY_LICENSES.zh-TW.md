<!-- source: THIRD_PARTY_LICENSES.md | locale: zh-TW | updated: 2026-10-23 -->
# 第三方元件與許可證

> 語言： [简体中文](THIRD_PARTY_LICENSES.md) · [English](THIRD_PARTY_LICENSES.en.md) · 繁體中文 · [Русский](THIRD_PARTY_LICENSES.ru.md) · [日本語](THIRD_PARTY_LICENSES.ja.md) · [한국어](THIRD_PARTY_LICENSES.ko.md) · [Deutsch](THIRD_PARTY_LICENSES.de.md) · [Français](THIRD_PARTY_LICENSES.fr.md) · [Español](THIRD_PARTY_LICENSES.es.md) · [Português](THIRD_PARTY_LICENSES.pt.md) · [Türkçe](THIRD_PARTY_LICENSES.tr.md) · [Bahasa Indonesia](THIRD_PARTY_LICENSES.id.md) · [ไทย](THIRD_PARTY_LICENSES.th.md) · [Tiếng Việt](THIRD_PARTY_LICENSES.vi.md) · [العربية](THIRD_PARTY_LICENSES.ar.md) · [فارسی](THIRD_PARTY_LICENSES.fa.md)

CoreFusion 使用了以下開源元件。本檔案列出主要元件及許可證，**完整清單以 `go.mod` / `go.sum` 及各模組自帶的 LICENSE 檔案為準**。

分發本專案的二進位制時，請同時保留本檔案、專案 `LICENSE` 以及各第三方元件的版權與許可宣告。

## 1. 編譯進二進位制的 Go 依賴（主要）

| 元件 | 許可證 | 用途 |
|---|---|---|
| `github.com/xtls/xray-core` | MPL-2.0 | Xray 配置解析、gRPC 命令、協議定義 |
| `github.com/xtls/reality` | MPL-2.0 | REALITY 協議支援 |
| `github.com/sagernet/sing` | **GPL-3.0-or-later** | 經 xray-core 間接連結 |
| `github.com/sagernet/sing-shadowsocks` | **GPL-3.0-or-later** | 經 xray-core 間接連結（Shadowsocks-2022） |
| `github.com/go-sql-driver/mysql` | MPL-2.0 | MySQL 驅動 |
| `github.com/hashicorp/golang-lru/v2` | MPL-2.0 | LRU 快取 |
| `github.com/go-acme/lego/v4` | MIT | ACME 證書籤發 |
| `github.com/gin-gonic/gin` | MIT | HTTP 框架 |
| `gorm.io/gorm` | MIT | ORM |
| `github.com/aws/aws-sdk-go-v2` | Apache-2.0 | S3 備份、Route53 DNS-01 |
| `google.golang.org/grpc` | Apache-2.0 | Agent gRPC 通訊 |
| `google.golang.org/protobuf` | BSD-3-Clause | Protobuf 執行時 |
| `golang.org/x/crypto` | BSD-3-Clause | 加密原語 |
| `golang.org/x/oauth2` | BSD-3-Clause | OAuth2 / Google EAB |
| `github.com/golang-jwt/jwt/v5` | MIT | JWT |
| `github.com/google/uuid` | BSD-3-Clause | UUID |

> ⚠ **GPL-3.0 元件提示**：`sing` 與 `sing-shadowsocks` 採用 GPL-3.0-or-later。它們被靜態連結進面板與 Agent 二進位制，因此**分發二進位制時必須提供 Corresponding Source**（見 [LEGAL.md](./LEGAL.zh-TW.md) 第 3 節）。

## 2. 執行時呼叫的獨立程式（不在本倉庫內分發）

面板 / Agent 執行時可下載、安裝或呼叫以下獨立程式。它們與本專案透過程序/介面互動，但如果你在交付物中一併分發，仍需遵守各自的許可證：

| 程式 | 許可證 | 說明 |
|---|---|---|
| Xray-core | MPL-2.0 | 代理核心，獨立程序 |
| sing-box | **GPL-3.0-or-later** | 代理核心，獨立程序；分發時需提供對應原始碼 |
| Caddy | Apache-2.0 | 反向代理 / TLS |
| Nginx | BSD-2-Clause | 反向代理（系統安裝，如使用） |

> Hysteria2、TUIC 是**由 sing-box 承載的協議**，不是獨立核心，也沒有獨立二進位制，因此不作為單獨的第三方程式列出。

## 3. 資料檔案

| 資料 | 來源/許可證 |
|---|---|
| `geoip.dat` / `geosite.dat` | 以其資料來源倉庫宣告為準（常見為 GPL-3.0 或 CC-BY-SA） |
| GeoLite2 (`GeoLite2-ASN.mmdb` 等) | MaxMind GeoLite2 EULA / CC BY-SA 4.0，商用前請確認授權 |
| `country.mmdb` | 以其資料來源宣告為準 |

## 4. 合規建議

- 交付二進位制時附帶：`LICENSE`、本檔案、各 GPL/MPL 元件的許可證文字或獲取地址。
- 若修改了 `xray-core` 等 MPL-2.0 元件的原始檔，需公開被修改檔案的原始碼。
- 若隨產品分發 sing-box 等 GPL 程式，需向接收者提供對應版本的完整原始碼（或有效的書面獲取途徑）。
- 商標與名稱：不得使用各上游專案的名稱、Logo 暗示官方背書。
