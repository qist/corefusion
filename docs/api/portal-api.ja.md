<!-- source: portal-api.md | locale: ja | updated: 2026-10-23 -->
# ポータル API リファレンス (クライアント / ポータル API)

> 言語： [简体中文](portal-api.md) · [English](portal-api.en.md) · [繁體中文](portal-api.zh-TW.md) · [Русский](portal-api.ru.md) · 日本語 · [한국어](portal-api.ko.md) · [Deutsch](portal-api.de.md) · [Français](portal-api.fr.md) · [Español](portal-api.es.md) · [Português](portal-api.pt.md) · [Türkçe](portal-api.tr.md) · [Bahasa Indonesia](portal-api.id.md) · [ไทย](portal-api.th.md) · [Tiếng Việt](portal-api.vi.md) · [العربية](portal-api.ar.md) · [فارسی](portal-api.fa.md)

> ポータル フロントエンド (`/p` または専用ポータル ポート) とサードパーティ クライアントの **完全な** インターフェイス リファレンス。
> ルート定義: `internal/api/router/router.go` (`clientAuthGroup` / `clientAuthProtected` / `/api/v1/public/*`)。

## 1. 一般的な規約

### 1.1 ベース URL

| 導入 | ベース URL |
|---|---|
| メインパネルエンジン | `https://<panel-domain>/api/v1` (`/p` のポータル ページ) |
| 専用ポータルポート | `https://<portal-domain>[:port]/api/v1` (`/` のポータル ページ) |
| 専用ポータルドメイン + リバースプロキシ | 上記と同じ。プロキシはページ パスと `/api/v1` の両方を転送する必要があります |

### 1.2 認証

- サインイン後、`Authorization: Bearer <access_token>` を送信します。
- アクセス トークンは 24 時間持続し、リフレッシュ トークンは 7 日間持続します。
- JWT には `tv` (token_version) が含まれています。ログアウト/強制サインアウトにより値が増加し、古いトークンがすべて無効になります。
 (`clients.token_version`、`internal/service/clientauth/service.go` を参照);
- すべての保護されたエンドポイントは、`ClientAuthMiddleware` (バージョン チェック + メンテナンス モード ガード) を通過します。

### 1.3 応答エンベロープ

ほとんどのエンドポイントは `{ "code": 0, "data": ..., "message": "..." }` を返します。
一部のリスト エンドポイントは、`{ "items": [...], "total": N }` または裸の配列を返します (以下のエンドポイントごとに記載)。
エラーは`{ "code": "ERROR_CODE", "message": "...", "request_id": "...", "can_retry": ... }`を返します
(HTTP 4xx/5xx)。

### 1.4 金額単位

注文/残高フィールドは **整数セント**: `990` = ¥9.90 (`currency: "CNY"`)。

### 1.5 実際の例（サインイン→プロファイルの取得）

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

401 で、`POST /api/v1/client-auth/refresh` を 1 回呼び出して再試行します。それでも失敗する場合は、ローカル セッションをクリアします (これはポータル テンプレートですでに行われています)。

---

## 2. パブリック エンドポイント (サインインなし)

| 方法 | パス | 説明 |
|---|---|---|
| GET | `/p` (専用ポータル エンジン上の `/`) | ポータル HTML (サーバーレンダリング + Vue ランタイム) |
| GET | `/api/v1/public/portal-config` | ポータル ランタイム構成: サイト名、テーマの色、ナビゲーション、モジュール、言語パック、`registration_enabled`、`regions` (サインイン領域ソース)、`privacy_policy_url` / `terms_of_service_url` / `privacy_policy_content` / `terms_of_service_content` (コンプライアンス ドキュメント、言語ごとにマップされたコンテンツ、スタンドアロン ページ `/p/legal/*` ✓) |
| GET | `/api/v1/public/settings` | パブリック設定 (現在 `registration_enabled`) |
| GET | `/api/v1/public/subscription-endpoints` | サブスクリプションエンドポイントリスト (ベア配列) |
| GET | `/api/v1/public/articles` | 公開記事（公開のみ） |
| GET | `/api/v1/public/articles/:slug` | 記事詳細 |
| GET | `/api/v1/public/client-configs` | クライアントのダウンロード構成 |
| GET | `/sub/:token` | サブスクリプション配信 (`format` は、base64_uri / raw_uri / mihomo_yaml / singbox_json / xray_json などを返します。取り消し/欠落 → 404、一時停止/期限切れ/非アクティブ → 403 を参照。[subscription.md](portal-api.ja.md) を参照) |
| GET | `/api/v1/client-auth/oauth/providers` | OAuth プロバイダーの可視性: `{"data":{"providers":[{"provider":"github","enabled":true},...]}}` |
| GET | `/api/v1/system/maintenance` | メンテナンス状況（公開）。有効にすると、ポータルにメンテナンス ページが表示されます |

> これらにはすべて、メンテナンス ガードとパブリック API レート制限が含まれます。

## 3. 認証 / OAuth

| 方法 | パス | 認証 | 説明 |
|---|---|---|---|
| POST | `/api/v1/client-auth/register` | — | 登録します。本体：`{email, password, referral_code?}`。 `{access_token, refresh_token, client}`を返します。登録率が制限されている |
| POST | `/api/v1/client-auth/login` | — | サインイン。本体: `{email, password}`。同じ反応です。 `token_version` で JWT を発行します |
| POST | `/api/v1/client-auth/refresh` | — | リフレッシュしてください。本体：`{refresh_token}`。新しいアクセス/リフレッシュ トークンを返します (バージョンがチェックされます。古いリフレッシュ トークンはログアウト後すぐに失われます)。 |
| POST | `/api/v1/client-auth/logout` | ✓ | ログアウト: サーバー側の取り消し (`token_version +1`) → クライアントの古いトークンはすべて無効です |
| POST | `/api/v1/client-auth/rotate-token` | ✓ | サブスクリプション トークンをローテーションする |
| POST | `/api/v1/client-auth/ticket` | ✓ | **1 回限りの有効期限の短いチケット** を発行します (ヘッダーを設定できない「ダウンロード ログ バンドル」などの `window.open` フローの場合) |
| GET | `/api/v1/client-auth/oauth/:provider` | — | OAuth の開始: 307 → GitHub/Google 同意 (`state` アンチ CSRF、10 分);無効なプロバイダーは 400 を返します |
| GET | `/api/v1/client-auth/oauth/:provider/callback` | — | OAuth コールバック: コード交換 → クライアントの作成/リンク → バージョン管理された JWT の発行 → 302 をポータルに戻します (`/<portalPath>?access_token=...&refresh_token=...`) |

> OAuth コールバックは **ブラウザ リダイレクト** (API 呼び出しではありません) です。サインインが成功した後、ポータルはトークンを読み取ります。
> `onMounted` の URL から (または同じオリジンの `postMessage` 経由。`web` ポータル テンプレートの `handleOAuthCallback` を参照)。

## 4. アカウント

| 方法 | パス | 説明 |
|---|---|---|
| GET | `/api/v1/client-auth/me` | 現在のクライアント: `{id, email, plan_id, plan_name, status, quota_bytes, used_bytes, upload_bytes, download_bytes, expires_at, ip_whitelist, device_limit, sub_token, sub_status, created_at, ...}` (`data` エンベロープ) |
| POST | `/api/v1/client-auth/change-password` | 本体：`{old_password, new_password}` |
| GET | `/api/v1/client-auth/checktime` | サーバー時刻 (クライアント時計の同期) |
| GET | `/api/v1/client-auth/connection-info` | 最新プランの接続パラメータ: `{concurrent, device_limit, protocol, speed_limit, tls, transport}` |

## 5. 定期購入

| 方法 | パス | 説明 |
|---|---|---|
| POST | `/api/v1/client-auth/reset-subscription` | サブスクリプションをリセットします。これは `sub_token` を回転させます |

> サブスクリプション リンクは、`/me` (`/sub/:token`) によって返される `sub_token` からフロントエンド上に構築されます。ありません
> `subscription-link` / `subscription-formats` エンドポイントはもう必要ありません。

## 6. ノード

| 方法 | パス | 説明 |
|---|---|---|
| GET | `/api/v1/client-auth/nodes` | 現在のプランに表示されるノード: `{items:[...]}` (名前/プロトコル/トランスポート/リージョンなどのフィールドを表示) |
| GET | `/api/v1/client-auth/nodes/:id/config` | 完全なノード構成 |
| GET | `/api/v1/client-auth/nodes/:id/link` | リンクを共有 (vless:// など) |
| GET | `/api/v1/client-auth/nodes/:id/qrcode` | QRコード画像 |

## 7. ストア/プラン/注文

| 方法 | パス | 説明 |
|---|---|---|
| GET | `/api/v1/client-auth/plans` | プランリスト（ベアアレイ）：`{id, name, description, category, sort_order, variants:[{id, name, billing_cycle, duration_days, price(cents), original_price, currency, quota_bytes, ...}]}` |
| GET | `/api/v1/client-auth/plans/:id` | プラン詳細 |
| GET | `/api/v1/client-auth/traffic-packs` | トラフィックパックリスト |
| POST | `/api/v1/client-auth/traffic-packs/buy` | トラフィックパックを購入する |
| GET | `/api/v1/client-auth/orders` | 私のプランの注文 (ページ分割) |
| POST | `/api/v1/client-auth/orders` | 注文の作成 (プラン バリアントの購入/更新) |
| GET | `/api/v1/client-auth/orders/:id` | 注文詳細 |
| GET | `/api/v1/client-auth/invoices` | 請求書 |
| GET | `/api/v1/client-auth/invoices/:id` | 請求書の詳細 |

## 8. バランス/リチャージ

| 方法 | パス | 説明 |
|---|---|---|
| GET | `/api/v1/client-auth/balance` | `{data:{client_id, balance(cents), frozen}}` |
| GET | `/api/v1/client-auth/balance/transactions` | 残高トランザクション (`type`: チャージ/消費/返金、ページ分割) |
| GET | `/api/v1/client-auth/charge/sites` | 利用可能な充電サイト (`ListChargeSites`: 有効のみ) |
| POST | `/api/v1/client-auth/charge/create` | リチャージ注文の作成 (サードパーティの支払いにリダイレクト) |
| GET | `/api/v1/client-auth/charge/orders` | リチャージ注文リスト |
| GET | `/api/v1/client-auth/charge/orders/:id` | リチャージ注文の詳細 |
| GET | `/api/v1/client-auth/charge/status/:trade_no` | trade_no ごとに支払いステータスをポーリングします |
| POST | `/api/v1/client-auth/charge/redeem-card` | リチャージカードを引き換える |

> クライアントを削除すると、残高とトランザクションに**カスケード**されます (`internal/service/client/service.go` の `DeleteClient` を参照)。

## 9. ギフト/トラフィック履歴

| 方法 | パス | 説明 |
|---|---|---|
| GET | `/api/v1/client-auth/gifts` | 請求可能なギフト (ベア配列; `type`: 日/バイト) |
| POST | `/api/v1/client-auth/gifts/:id/claim` | クレーム |
| GET | `/api/v1/client-auth/traffic-history` | 月間トラフィック履歴: `{data:{history:[{period_start,total_bytes,...}], months}}` |

## 10. チケット

| 方法 | パス | 説明 |
|---|---|---|
| GET | `/api/v1/client-auth/tickets` | チケットリスト：`{items,total}` |
| POST | `/api/v1/client-auth/tickets` | チケットを作成する |
| GET | `/api/v1/client-auth/tickets/:id` | チケット詳細（返信あり） |
| POST | `/api/v1/client-auth/tickets/:id/reply` | 返信 |
| POST | `/api/v1/client-auth/tickets/:id/close` | 閉じる |

## 11. 記事/ガイド

| 方法 | パス | 説明 |
|---|---|---|
| GET | `/api/v1/client-auth/articles` | 記事一覧（カテゴリフィルターあり） |
| GET | `/api/v1/client-auth/articles/:slug` | 記事詳細 |

## 12. 紹介 (デフォルトでは無効になっています。ポータル エディターで有効にできます)

| 方法 | パス | 説明 |
|---|---|---|
| GET | `/api/v1/client-auth/referral/code` | 私の紹介コードとリンク |
| GET | `/api/v1/client-auth/referral/stats` | コミッション統計 (`commission_rate`、`available_commission`、`min_withdraw_amount`) |
| GET | `/api/v1/client-auth/referral/records` | 紹介記録 |
| GET | `/api/v1/client-auth/referral/withdrawals` | 出金記録 |
| POST | `/api/v1/client-auth/referral/withdraw` | 引き出しをリクエストする |

## 13. 共有アカウント

| 方法 | パス | 説明 |
|---|---|---|
| GET | `/api/v1/client-auth/shared-accounts` | 利用可能な共有アカウント（ストリーミングなど） |
| GET | `/api/v1/client-auth/shared-accounts/:id` | 詳細 |
| POST | `/api/v1/client-auth/shared-accounts/:id/apply` | アクセスをリクエストする |
| POST | `/api/v1/client-auth/shared-accounts/:id/repair` | ワンクリック修復 |

## 14. ログバンドル (加入者診断)

| 方法 | パス | 説明 |
|---|---|---|
| GET | `/api/v1/client-auth/logs/types` | バンドルできるログの種類 |
| GET | `/api/v1/client-auth/logs/packages` | 私のログバンドル |
| POST | `/api/v1/client-auth/logs/package` | ログ バンドル ジョブの作成 (エントリ ノードのみ) |
| GET | `/api/v1/client-auth/logs/packages/:id` | ジョブのステータス (準備完了/処理中/失敗) |
| GET | `/api/v1/client-auth/logs/packages/:id/download` | ダウンロード (`POST /ticket` を優先し、`?ticket=` を追加) |
| DELETE | `/api/v1/client-auth/logs/packages/:id` | ログバンドルを削除する |

## 15. セキュリティ設定 / IP許可リスト / 自己禁止解除

| 方法 | パス | 説明 |
|---|---|---|
| GET/PUT | `/api/v1/client-auth/settings` | クライアント設定: `{ip_whitelist, device_limit}` (PUT) |
| GET | `/api/v1/client-auth/settings/whitelist` | 許可リスト (ベア配列) |
| DELETE | `/api/v1/client-auth/settings/whitelist/:id` | 許可リストのエントリを削除する |
| POST | `/api/v1/client-auth/settings/auto-add-ip` | 現在の IP を許可リストに追加します |
| POST | `/api/v1/client-auth/self-unban` | セルフサービスの禁止解除 (ポリシーにより試行が制限されています) |

> IP 許可リストは **サブスクリプション URL にのみ適用されます** (`/sub/:token`)。ポータルのサインイン/OAuth はブロックされません。
> 他のクライアント認証エンドポイント。ユーザーに許可リストがない場合、またはサブスクリプション URL がない場合、アクセスは常に許可されます。の
> ホワイトリストはユーザーごとの設定です。グローバルスイッチはありません。

**自己禁止フロー**: 管理者側の場合、`auto_quota` (トラフィック超過の自動禁止) でのみ利用可能
`self_unban.enabled`がオンです。サインイン (パスワードまたは OAuth コールバック) は `require_self_unban=true` + `self_unban_token` を返します
(typ=self_unban、有効な 15 分、`POST /self-unban` でのみ使用可能、他のエンドポイントは 403/`SELF_UNBAN_TOKEN_SCOPE` を返します)。
次に、ポータルに禁止解除カードが表示されます。禁止解除に成功すると、ユーザーは再度サインインします。 OAuth コールバックはこれを経由して配信します
URL フラグメント `#require_self_unban=1&self_unban_token=..&self_unban_hint=..`。

## 16. エラーコード（抜粋）

| コード | シナリオ |
|---|---|
| `INVALID_TOKEN` | リフレッシュトークンが無効/期限切れ/バージョンが一致しません |
| `INVALID_REQUEST` | パラメータの検証に失敗しました |
| `NOT_FOUND` | リソースが不足しています (チケット/オーダー/許可リストのエントリなど) |
| `RELOAD_FAILED` / `SYNC_FAILED` | (管理者側) プロキシ構成の同期に失敗しました |
| 401+`errClientTokenRevoked` | token_version の不一致 (ログアウト/キック) → クライアントは更新する必要があります。それが失敗した場合は、ローカル トークンをクリアします |

## 17. コンプライアンス文書（プライバシーポリシー/利用規約）

サインイン カードの下部にある「プライバシー ポリシー · 利用規約」のリンク (通常のサイトと同じです。クリックするとコンテンツが開きます)。

- **設定**: ポータル エディター → 設定 → 一般 → コンプライアンス ドキュメント (**言語ごとにコンテンツを編集**);
 構造化フィールド `PUT /api/v1/portal/active` を使用することもできます `privacy_policy_url` / `privacy_policy_content` /
 `terms_of_service_url` / `terms_of_service_content` (nil = 変更なし、コンテンツは `{"zh-CN":"...","en":"..."}` マップ)。
- **スタンドアロンのアクセス可能なページ** (サーバーレンダリング、Google 検証クローラーにとって簡単、`?lang=xx` をサポート):
  - メインパネルドメイン: `GET /p/legal/privacy`、`GET /p/legal/terms`
  - 専用ポータル ドメイン (ルート展開): `GET /legal/privacy`、`GET /legal/terms`
- **表示ロジック**: URL またはコンテンツ、**URL が優先** (新しいタブで外部に開きます)。コンテンツが提供されている場合は、
 サインイン カードは、現在のポータル言語を使用してダイアログ (`renderMd`) に Markdown をレンダリングします。
- 実行時に `/api/v1/public/portal-config` 経由で配信されます。サインインコンポーネントは消費します
 `ctx.privacyUrl / termsUrl / privacyContent / termsContent / openLegal / hasLegalLinks`。
- Google OAuth 検証では、これらのリンクがパブリックにアクセスできる必要があります → 上記のスタンドアロン リンクを使用してください
 同意画面。 `docs/oauth-google-verification.md` (プライバシー ポリシー/規約テンプレートを含む) を参照してください。

## 18. 統合例

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

401 で、`POST /refresh` を 1 回呼び出して再試行します。それでも失敗する場合は、ローカル セッションをクリアします
(ポータル テンプレートの `api()` はすでにこれを実装しています)。
