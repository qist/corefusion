<!-- source: THIRD_PARTY_LICENSES.md | locale: ja | updated: 2026-11-19 -->
# サードパーティのコンポーネントとライセンス

> 言語： [简体中文](THIRD_PARTY_LICENSES.md) · [English](THIRD_PARTY_LICENSES.en.md) · [繁體中文](THIRD_PARTY_LICENSES.zh-TW.md) · [Русский](THIRD_PARTY_LICENSES.ru.md) · 日本語 · [한국어](THIRD_PARTY_LICENSES.ko.md) · [Deutsch](THIRD_PARTY_LICENSES.de.md) · [Français](THIRD_PARTY_LICENSES.fr.md) · [Español](THIRD_PARTY_LICENSES.es.md) · [Português](THIRD_PARTY_LICENSES.pt.md) · [Türkçe](THIRD_PARTY_LICENSES.tr.md) · [Bahasa Indonesia](THIRD_PARTY_LICENSES.id.md) · [ไทย](THIRD_PARTY_LICENSES.th.md) · [Tiếng Việt](THIRD_PARTY_LICENSES.vi.md) · [العربية](THIRD_PARTY_LICENSES.ar.md) · [فارسی](THIRD_PARTY_LICENSES.fa.md)

CoreFusion は、次のオープンソース コンポーネントを使用します。このファイルには、主要なコンポーネントとライセンスがリストされています。 **完全なリストは、`go.mod` / `go.sum`、`web/admin/package.json` および各モジュール独自の LICENSE ファイルによって定義されます**。

このプロジェクトのバイナリを配布する場合は、このファイル、プロジェクト `LICENSE`、およびすべてのサードパーティ コンポーネントの著作権/ライセンス通知を保管してください。

## 1. バイナリにコンパイルされた Go 依存関係 (メイン)

| コンポーネント | ライセンス | 目的 |
|---|---|---|
| `github.com/xtls/xray-core` | MPL-2.0 | Xray 構成解析、gRPC コマンド、プロトコル定義 |
| `github.com/xtls/reality` | MPL-2.0 | REALITY プロトコルのサポート |
| `github.com/sagernet/sing` | **GPL-3.0-or-later** | xray-core 経由で間接的にリンク |
| `github.com/sagernet/sing-shadowsocks` | **GPL-3.0-or-later** | xray コア (Shadowsocks-2022) 経由で間接的にリンク |
| `github.com/go-sql-driver/mysql` | MPL-2.0 | MySQLドライバー |
| `github.com/hashicorp/golang-lru/v2` | MPL-2.0 | LRUキャッシュ |
| `github.com/go-acme/lego/v5` | MIT | ACME 証明書の発行 |
| `github.com/gin-gonic/gin` | MIT | HTTP フレームワーク |
| `gorm.io/gorm` | MIT | ORM |
| `gorm.io/driver/mysql` | MIT | GORM用MySQLドライバー |
| `gorm.io/driver/postgres` | MIT | GORM用PostgreSQLドライバー |
| `github.com/jackc/pgx/v5` | MIT | PostgreSQLドライバー |
| `github.com/qist/sqlite` | MIT | SQLite ドライバー (パネル/エージェントのデフォルト データベース) |
| `github.com/aws/aws-sdk-go-v2` | Apache-2.0 | S3 バックアップ、Route53 DNS-01 |
| `google.golang.org/grpc` | Apache-2.0 | エージェント gRPC 通信 |
| `google.golang.org/protobuf` | BSD-3-Clause | Protobuf ランタイム |
| `golang.org/x/crypto` | BSD-3-Clause | 暗号プリミティブ |
| `golang.org/x/oauth2` | BSD-3-Clause | OAuth2 / Google EAB |
| `golang.org/x/sys` | BSD-3-Clause | システムコール（nft/tcレート制限、プロセス管理） |
| `github.com/golang-jwt/jwt/v5` | MIT | JWT |
| `github.com/google/uuid` | BSD-3-Clause | UUID |
| `github.com/go-webauthn/webauthn` | BSD-3-Clause | Passkey / WebAuthn サインイン |
| `github.com/pquerna/otp` | Apache-2.0 | TOTP 二要素認証 |
| `github.com/jlaffaye/ftp` | ISC | FTP バックアップアップロード |
| `gopkg.in/yaml.v3` | MIT / Apache-2.0 | YAML 構成の解析 |

> ⚠ **GPL-3.0 コンポーネント**: `sing` および `sing-shadowsocks` は GPL-3.0 以降です。これらはパネルおよびエージェントのバイナリに静的にリンクされているため、**バイナリを配布するには、対応するソースを提供する必要があります** ([LEGAL.md](./LEGAL.md) のセクション 3 を参照)。

## 2. フロントエンドの依存関係 (パネルバイナリにコンパイルされる)

管理 UI は Vite で構築され、`go:embed` 経由でパネル バイナリに埋め込まれているため、次のコンポーネントもバイナリとともに配布されます。

| コンポーネント | ライセンス | 目的 |
|---|---|---|
| Vue 3 / Vue Router / Pinia / Element Plus / axios / vue-i18n / marked / js-yaml / qrcode / Monaco Editor | MIT | 管理 UI、ルーティングと状態、i18n、Markdown レンダリング、QR コード、コード エディター |
| DOMPurify | Apache-2.0 / MPL-2.0 | Markdown / HTML をレンダリングする前の XSS サニタイズ |

## 3. 実行時にパネルによってダウンロード/呼び出されるスタンドアロン プログラム

コアをインストールまたはアップグレードするとき、パネル/エージェントは GitHub から次のバイナリをダウンロードし、別のプロセスとして実行します。 **「このプロジェクトによって構築された」とマークされたアーティファクトは、上流のソース** (`qist/*-binaries`) からこの組織によってコンパイルおよび公開され、製品と一緒に配布されるため、対応するライセンス義務 (GPL のソースの可用性を含む) はこのプロジェクトにあります。他のものは、公式のアップストリーム リリースのダウンロード URL です。

| プログラム | ライセンス | 注意事項 |
|---|---|---|
| Xray-core | MPL-2.0 | プロキシ コア、別のプロセス。公式アップストリーム リリース (`XTLS/Xray-core`) |
| sing-box | **GPL-3.0-or-later** | プロキシ コア、別のプロセス。 **このプロジェクトによって構築されました** (`qist/singbox-binaries`; 公式リリースには、ユーザーごとのトラフィック アカウンティングに必要な `with_v2ray_api` がありません)。配布時に対応するソースを提供する |
| Caddy | Apache-2.0 | リバースプロキシ/TLS; **このプロジェクトによって構築されました** (`qist/caddy-binaries`) |
| Nginx | BSD-2-Clause | リバースプロキシ。 **このプロジェクトによって構築されました** (`qist/nginx-binaries`、glibc / musl によって分割) |

> Hysteria2 および TUIC は **sing-box** によって伝送されるプロトコルであり、スタンドアロン コアやバイナリではないため、個別にリストされていません。

## 4. データファイル

| データ | 出典/ライセンス |
|---|---|
| `geoip.dat` / `geosite.dat` | データ ソース リポジトリによって宣言されているとおり (通常は GPL-3.0 または CC-BY-SA) |
| GeoLite2（`GeoLite2-ASN.mmdb`など） | MaxMind GeoLite2 EULA / CC BY-SA 4.0;商用利用の前にライセンスを確認する |
| `country.mmdb` | データソースで宣言されているとおり |

## 5. コンプライアンスの推奨事項

- バイナリを配布するときは、`LICENSE`、このファイル、および各 GPL/MPL コンポーネントのライセンス テキストまたは取得 URL を含めます。
- このプロジェクトによってビルドされたバイナリ (sing-box / Caddy / Nginx) はパネルに同梱されています。バイナリは上流のソースからコンパイルされているため、ビルド方法 (バージョン、ビルド タグ、構成フラグ) を公開します。さらに、GPL コンポーネント (sing-box) には、そのバージョンの完全に対応するソース、または有効な入手ルートが必要です。
- `xray-core` などの MPL-2.0 コンポーネントのソース ファイルを変更した場合は、変更したファイルのソースを公開します。
- sing-box などの GPL プログラムを製品とともに配布する場合は、そのバージョンに対応する完全なソース (または効果的な書面によるオファー) を受信者に提供してください。
- 商標と名前: 公式承認を示唆するような方法でアップストリーム プロジェクトの名前やロゴを使用しないでください。