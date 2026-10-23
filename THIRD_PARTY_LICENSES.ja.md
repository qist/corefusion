<!-- source: THIRD_PARTY_LICENSES.md | locale: ja | updated: 2026-10-23 -->
# サードパーティのコンポーネントとライセンス

> 言語： [简体中文](THIRD_PARTY_LICENSES.md) · [English](THIRD_PARTY_LICENSES.en.md) · [繁體中文](THIRD_PARTY_LICENSES.zh-TW.md) · [Русский](THIRD_PARTY_LICENSES.ru.md) · 日本語 · [한국어](THIRD_PARTY_LICENSES.ko.md) · [Deutsch](THIRD_PARTY_LICENSES.de.md) · [Français](THIRD_PARTY_LICENSES.fr.md) · [Español](THIRD_PARTY_LICENSES.es.md) · [Português](THIRD_PARTY_LICENSES.pt.md) · [Türkçe](THIRD_PARTY_LICENSES.tr.md) · [Bahasa Indonesia](THIRD_PARTY_LICENSES.id.md) · [ไทย](THIRD_PARTY_LICENSES.th.md) · [Tiếng Việt](THIRD_PARTY_LICENSES.vi.md) · [العربية](THIRD_PARTY_LICENSES.ar.md) · [فارسی](THIRD_PARTY_LICENSES.fa.md)

CoreFusion は、次のオープンソース コンポーネントを使用します。このファイルには、主要なコンポーネントとライセンスがリストされています。 **完全なリストは、`go.mod` / `go.sum` および各モジュール独自の LICENSE ファイルによって定義されます**。

このプロジェクトのバイナリを配布する場合は、このファイル、プロジェクト `LICENSE`、およびすべてのサードパーティ コンポーネントの著作権/ライセンス通知を保管してください。

## 1. バイナリにコンパイルされた Go 依存関係 (メイン)

| コンポーネント | ライセンス | 目的 |
|---|---|---|
| `github.com/xtls/xray-core` | MPL-2.0 | Xray 構成解析、gRPC コマンド、プロトコル定義 |
| `github.com/xtls/reality` | MPL-2.0 | REALITY プロトコルのサポート |
| `github.com/sagernet/sing` | **GPL-3.0 以降** | xray-core 経由で間接的にリンク |
| `github.com/sagernet/sing-shadowsocks` | **GPL-3.0 以降** | xray コア (Shadowsocks-2022) 経由で間接的にリンク |
| `github.com/go-sql-driver/mysql` | MPL-2.0 | MySQLドライバー |
| `github.com/hashicorp/golang-lru/v2` | MPL-2.0 | LRUキャッシュ |
| `github.com/go-acme/lego/v4` | マサチューセッツ工科大学 | ACME 証明書の発行 |
| `github.com/gin-gonic/gin` | マサチューセッツ工科大学 | HTTP フレームワーク |
| `gorm.io/gorm` | マサチューセッツ工科大学 | ORM |
| `github.com/aws/aws-sdk-go-v2` | アパッチ-2.0 | S3 バックアップ、Route53 DNS-01 |
| `google.golang.org/grpc` | アパッチ-2.0 | エージェント gRPC 通信 |
| `google.golang.org/protobuf` | BSD-3-Claes | Protobuf ランタイム |
| `golang.org/x/crypto` | BSD-3-Claes | 暗号プリミティブ |
| `golang.org/x/oauth2` | BSD-3-Claes | OAuth2 / Google EAB |
| `github.com/golang-jwt/jwt/v5` | マサチューセッツ工科大学 | JWT |
| `github.com/google/uuid` | BSD-3-Claes | UUID |

> ⚠ **GPL-3.0 コンポーネント**: `sing` および `sing-shadowsocks` は GPL-3.0 以降です。これらはパネルおよびエージェントのバイナリに静的にリンクされているため、**バイナリを配布するには、対応するソースを提供する必要があります** ([LEGAL.md](THIRD_PARTY_LICENSES.ja.md) のセクション 3 を参照)。

## 2. 実行時に呼び出されるスタンドアロン プログラム (このリポジトリでは配布されません)

パネル/エージェントは、実行時に次のスタンドアロン プログラムをダウンロード、インストール、または起動する場合があります。これらはプロセス/インターフェイスを通じてこのプロジェクトと対話しますが、成果物と一緒に出荷する場合は、引き続きライセンスに準拠する必要があります。

| プログラム | ライセンス | 注意事項 |
|---|---|---|
| Xray-コア | MPL-2.0 | プロキシコア、別プロセス |
| sing-box | **GPL-3.0 以降** | プロキシ コア、別のプロセス。配布時に対応するソースを提供する |
| キャディ | アパッチ-2.0 | リバースプロキシ / TLS |
| Nginx | BSD-2 条項 | リバース プロキシ (システムにインストールされている場合、使用されている場合) |

> Hysteria2 および TUIC は **sing-box** によって伝送されるプロトコルであり、スタンドアロン コアやバイナリではないため、個別にリストされていません。

## 3. データファイル

| データ | 出典/ライセンス |
|---|---|
| `geoip.dat` / `geosite.dat` | データ ソース リポジトリによって宣言されているとおり (通常は GPL-3.0 または CC-BY-SA) |
| GeoLite2（`GeoLite2-ASN.mmdb`など） | MaxMind GeoLite2 EULA / CC BY-SA 4.0;商用利用の前にライセンスを確認する |
| `country.mmdb` | データソースで宣言されているとおり |

## 4. コンプライアンスの推奨事項

- バイナリを配布するときは、`LICENSE`、このファイル、および各 GPL/MPL コンポーネントのライセンス テキストまたは取得 URL を含めます。
- `xray-core` などの MPL-2.0 コンポーネントのソース ファイルを変更する場合は、変更したファイルのソースを公開します。
- sing-box などの GPL プログラムを製品とともに配布する場合は、そのバージョンに対応する完全なソース (または効果的な書面によるオファー) を受信者に提供してください。
- 商標と名前: 公式承認を示唆するような方法でアップストリーム プロジェクトの名前やロゴを使用しないでください。
