<!-- source: subscription.md | locale: ja | updated: 2026-11-12 -->
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
| `providers` | `mihomo_yaml` のみ: `1` / `true` は **プロバイダー モード** を有効にします (`proxy-providers` を発行し、`use` 経由で参照します)。デフォルトまたはその他の値では、ノードは `proxies` の下にインライン化されます。 §6 を参照 |

### サポートされている `format` 値

| 値 | 出力 | クライアント |
|---|---|---|
| `base64_uri` (デフォルト) | 改行で結合された共有リンク、全体として Base64 エンコード | V2RayN、Shadowrocket、Surge、その他の汎用クライアント |
| `raw_uri` | 改行で結合された共有リンク、プレーンテキスト | プレーンリンク/デバッグが必要なクライアント |
| `mihomo_yaml` | 完全な Clash/Mihomo YAML 構成 (プロキシ グループ、ルール付き) | Clash.Meta / mihomo / Clash Verge Rev |
| `mihomo_proxies_yaml` | `proxies:` ノード リストのみを含む YAML | mihomo プロキシ プロバイダー ソース (スケジュールされた更新あり) |
| `singbox_json` | 完全な sing-box JSON 構成 | sing-box / SFI / FLClash (sing-box コア) |
| `xray_json` | 完全な Xray JSON 構成 | Xray-コア |

> プロトコル/形式の互換性が適用されます。たとえば、`anytls` は sing-box/Mihomo 形式でのみ表示されます。 **xray によってサービスされる REALITY ノードは、sing-box サブスクリプションから除外されます** (sing-box は、xray の REALITY 実装をサポートしません。sing-box#4520 を参照) が、依然として Xray サブスクリプションに表示され、**REALITY ノードはサービスされますsing-box によるものは、Xray と sing-box の両方のサブスクリプションで動作します**。クライアントが認識できないノードは、サブスクリプション全体が失敗するのではなく、自動的に削除されます。

## 3. 応答ヘッダー

| ヘッダー | 例 | 説明 |
|---|---|---|
| `Subscription-Userinfo` | `upload=1024; download=2048; total=1073741824; expire=1760000000` | 使用量と有効期限 (バイト/Unix 秒)。 **403 でも送信される**ため、クライアントは残りの手当を表示できます |
| `Profile-Update-Interval` | `6` | 推奨される更新間隔 (時間) |
| `Profile-Title` | `CoreFusion` | プロファイル名 (Clash Verge Rev / V2RayN などで表示) |
| `Profile-Web-Page-Url` | `https://portal.example.com` | ポータル URL (クライアントの「Web ページを開く」。ポータルに独自のポートがある場合はルート パス、それ以外の場合は /p) |
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

# 5) Clash dynamic subscription: config template + Provider mode
curl -L "https://sub.example.com/abcd1234efgh5678?format=clash&config=out&providers=1"

# 6) sing-box / Xray with their own config templates (rules come from the template)
curl -L "https://sub.example.com/abcd1234efgh5678?format=singbox_json&config=sing-box-home"
curl -L "https://sub.example.com/abcd1234efgh5678?format=xray_json&config=xray-home"
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

## 6. プロバイダー モードと複数のサブスクリプション ドメイン

### 6.1 `providers=1` が発するもの

`format=mihomo_yaml&providers=1` (または `format=clash&providers=1`) の場合:

- 最上位レベルでノードをインライン化しなくなり、`proxy-providers:` が発行されます。
- プロキシ グループは、`use: [<provider-key>...]` 経由でノード ソースを参照します。
- 共有サブスクリプション形式 (`mihomo_configs.config_json`) で書かれた `proxy-providers` は **無視**されます (共有設定は複数のプランに対応します。ハードコードされたプロバイダーはプラン間で漏洩します)。グループ → ソース マッピングは、「プラン管理 → グループ → ソース マッピング」の下に存在します。

プロバイダーの種類:

| 種類 | キー | 注意事項 |
|---|---|---|
| 内部 | サニタイズされたノードグループ名 | プラン ノード グループごとに生成されます。そのグループのノードのみが含まれます |
| 予約済み | `subscription-nodes` | プランのすべてのノード。ソース URL は `provider` パラメータを **省略** |
| 外部 | プランに設定されている名前 | 外部ソース (`http` YAML または `file`) |

プロバイダーごとの共通フィールド:

- `url` — `format=mihomo_proxies_yaml` でこのサブスクリプションをポイントし、メイン リクエストから `config` / `platform` を継承します。内部プロバイダーは `provider=<key>` を追加します。
- `path` — **常に「内部プロバイダーのデフォルト → キャッシュ パス テンプレート」から導出されます** (デフォルトの `./providers/{key}.yaml`、`{key}` が置き換えられます)。外部プロバイダーも独自のパスを設定できません。
- `health-check` — `enable / url / interval`;内部プロバイダーはデフォルトを継承し、外部プロバイダーはプランでそれを構成します。
- `interval` — 秒単位の更新間隔。

### 6.2 プロバイダー URL (マルチドメイン) に入るドメイン

「サブスクリプション センター → サブスクリプション エンドポイント」には、**複数** のレコード (ドメイン + パス + 有効) が保持される場合があります。プロバイダー `url` は次の順序でドメインを選択します。

1. **このリクエストが実際に使用したドメイン**: `X-Forwarded-Host` (ポート ストリップ) の最初の値、それ以外の場合は `Host`。 **有効な**エンドポイントと一致する必要があります。
2. 何も一致しない場合は、**最小の ID** を持つ有効なエンドポイントにフォールバックします。
3. パスは一致したエンドポイント (`path`、デフォルトは `/`) から取得され、`https://<domain><path>/<token>?format=mihomo_proxies_yaml...` になります。

結果:

- 新しいドメインを介して取得するクライアントは、プロバイダー内でそのドメインを取得します。**パネルの変更は必要ありません**。
- リスト内でまだ有効になっている古いドメインはハードコーディングされなくなりました。
- クライアントが古いプロバイダー URL をキャッシュしていた場合、サブスクリプションを更新する (またはプロバイダーのキャッシュをクリアする/クライアントを再起動する) と、プロバイダーが切り替わります。

リバース プロキシは `X-Forwarded-Proto` および `X-Forwarded-Host` を転送する必要があります。それ以外の場合は、フォールバック エンドポイントが使用されます。パネルで生成された Caddy / Nginx サイト テンプレートには、両方のヘッダー (`header_up X-Forwarded-Host {host}` / `proxy_set_header X-Forwarded-Host $host;`) がすでに含まれています。プロキシ構成を手動で保守する場合は、それらを手動で追加します。

## 7. 注意事項

- **形式と構成は密接に結合しています**: `config`/`platform` によって選択されたテンプレートは、このリクエストの `format` と一致する必要があります。一致しないテンプレートは無視されます。これにより、Mihomo ルールが sing-box/Xray JSON に挿入されなくなります。
- **`providers=1` は、Mihomo YAML にのみ影響します**: sing-box / Xray / Base64 / raw 無視します (ノードはインラインのままです)。
- **レート制限**: サブスクリプション エンドポイントには独自の制限があります (`SubscribeRateLimit`)。虐待的なクライアントは拒否されます。
- **アクセス ログ**: すべての取得で IP / UA / CDN 情報が記録され、管理 UI の「サブスクリプション アクセス ログ」に表示されます。
- 専用のサブスクリプション ポートにドメインが構成されていない場合、リクエストの `X-Forwarded-Proto` / `X-Forwarded-Host` が使用されます。リバース プロキシが両方のヘッダーを転送することを確認してください。
- マルチドメイン展開では、エンドポイント リストをクライアントが実際に使用するドメインと一致させてください。 §6.2 を参照してください。