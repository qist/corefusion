<!-- source: subscription.md | locale: ja | updated: 2026-10-23 -->
# サブスクリプション API (パブリック)

> 言語： [简体中文](subscription.md) · [English](subscription.en.md) · [繁體中文](subscription.zh-TW.md) · [Русский](subscription.ru.md) · 日本語 · [한국어](subscription.ko.md) · [Deutsch](subscription.de.md) · [Français](subscription.fr.md) · [Español](subscription.es.md) · [Português](subscription.pt.md) · [Türkçe](subscription.tr.md) · [Bahasa Indonesia](subscription.id.md) · [ไทย](subscription.th.md) · [Tiếng Việt](subscription.vi.md) · [العربية](subscription.ar.md) · [فارسی](subscription.fa.md)

ユーザーのサブスクリプション ノード リストを取得します。 **これはエンド ユーザーにとって最もよく使用されるエンドポイントです**。ヘッダーは必要ありません。アクセスは URL のトークンによって許可されます。

## 1. エンドポイント

| 導入 | URL |
|---|---|
| メインパネル | `GET https://<panel-domain>/sub/<token>` |
| 専用のサブスクリプション ポート | `GET https://<sub-domain>:<port>/<token>` (`/sub` プレフィックスなし) |

`<token>` は、管理者側によって生成されるサブスクリプションの一意のアクセス トークンです (ポータルの [マイ サブスクリプション] の下に表示されます)。

## 2. クエリパラメータ

| パラメータ | 説明 |
|---|---|
| `format` | 出力形式。サブスクリプションのデフォルトをオーバーライドします。以下を参照してください |
| `config` | 構成テンプレート名 (管理 UI の「サブスクリプション形式」で定義)。このリクエストの `format` と同じ形式を使用する必要があります。それ以外の場合は無視されます |
| `platform` | 構成テンプレートをプラットフォーム名で照合します (`config` が省略された場合に使用されます)。形式も一致する必要があります |

### サポートされている `format` 値

| 値 | 出力 | クライアント |
|---|---|---|
| `base64_uri` (デフォルト) | 改行で結合された共有リンク、全体として Base64 エンコード | V2RayN、Shadowrocket、Surge、その他の汎用クライアント |
| `raw_uri` | 改行で結合された共有リンク、プレーンテキスト | プレーンリンク/デバッグが必要なクライアント |
| `mihomo_yaml` | 完全な Clash/Mihomo YAML 構成 (プロキシ グループ、ルール付き) | Clash.Meta / mihomo / Clash Verge Rev |
| `mihomo_proxies_yaml` | `proxies:` ノード リストのみを含む YAML | mihomo プロキシ プロバイダー ソース (スケジュールされた更新あり) |
| `singbox_json` | 完全な sing-box JSON 構成 | sing-box / SFI / FLClash (sing-box コア) |
| `xray_json` | 完全な Xray JSON 構成 | Xrayコア |

> プロトコル/形式の互換性が適用されます。たとえば、`anytls` は sing-box/Mihomo 形式でのみ表示されます。 sing-box サーバー上の REALITY ノードは、Xray/sing-box JSON サブスクリプションには表示されません。クライアントが認識できないノードは、サブスクリプション全体が失敗するのではなく、自動的に削除されます。

## 3. 応答ヘッダー

| ヘッダー | 例 | 説明 |
|---|---|---|
| `Subscription-Userinfo` | `upload=1024; download=2048; total=1073741824; expire=1760000000` | 使用量と有効期限 (バイト/Unix 秒)。 **403 でも送信される**ため、クライアントは残りの手当を表示できます |
| `Profile-Update-Interval` | `6` | 推奨される更新間隔 (時間) |
| `Profile-Title` | `CoreFusion` | プロファイル名 (Clash Verge Rev / V2RayN などで表示) |
| `Profile-Web-Page-Url` | `https://…/portal` | ポータル ページのリンク (クライアントの「Web ページを開く」) |
| `Content-Type` | フォーマットごと: `text/plain` / `text/yaml` / `application/json` |  |
| `Content-Disposition` | `attachment; filename="subscription.txt"` |  |

## 4. ステータスコード

| HTTP | コード | 意味 |
|---|---|---|
| 200 | — | 通常 (本文は `format` でレンダリングされたサブスクリプションです) |
| 400 | `MISSING_TOKEN` | URL にトークンがありません |
| 403 | (プレーンテキスト) | サブスクリプションが一時停止/期限切れ/非アクティブです。メッセージは `Accept-Language` 経由でローカライズ (zh/en/ja/ko) されており、理由は 1 行です |
| 404 | `SUBSCRIPTION_NOT_FOUND` | トークンが存在しません |

> 取り消されたサブスクリプションは存在しないものとして扱われます (404)。

## 5. 例

```bash
# 1) Fetch with the default format
curl -L "https://panel.example.com/sub/abcd1234efgh5678"

# 2) Request Mihomo YAML with the "clash-global" config template
curl -L "https://panel.example.com/sub/abcd1234efgh5678?format=mihomo_yaml&config=clash-global"

# 3) Node list only (for a mihomo proxy-provider)
curl -L "https://panel.example.com/sub/abcd1234efgh5678?format=mihomo_proxies_yaml"

# 4) Check usage (headers only)
curl -sI "https://panel.example.com/sub/abcd1234efgh5678" | grep -i subscription-userinfo
```

Clash/Mihomo 動的サブスクリプション (定期更新):

```yaml
proxy-providers:
  subscription-nodes:
    type: http
    url: "https://panel.example.com/sub/abcd1234efgh5678?format=mihomo_proxies_yaml"
    interval: 3600
    health-check:
      enable: true
      url: https://www.gstatic.com/generate_204
      interval: 300

proxy-groups:
  - name: PROXY
    type: select
    use:
      - subscription-nodes
```

## 6. 注意事項

- **形式と構成は密接に結合しています**: `config`/`platform` によって選択されたテンプレートは、このリクエストの `format` と一致する必要があります。一致しないテンプレートは無視されます。これにより、Mihomo ルールが sing-box/Xray JSON に挿入されなくなります。
- **レート制限**: サブスクリプション エンドポイントには独自の制限があります (`SubscribeRateLimit`)。虐待的なクライアントは拒否されます。
- **アクセス ログ**: すべての取得で IP / UA / CDN 情報が記録され、管理 UI の「サブスクリプション アクセス ログ」の下に表示されます。
- 専用のサブスクリプション ポートにドメインが構成されていない場合、リクエストの `X-Forwarded-Proto` / `X-Forwarded-Host` が使用されます。リバース プロキシが両方のヘッダーを転送することを確認してください。
