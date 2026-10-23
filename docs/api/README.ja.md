<!-- source: README.md | locale: ja | updated: 2026-10-23 -->
# CoreFusion API ドキュメント

> 言語： [简体中文](README.md) · [English](README.en.md) · [繁體中文](README.zh-TW.md) · [Русский](README.ru.md) · 日本語 · [한국어](README.ko.md) · [Deutsch](README.de.md) · [Français](README.fr.md) · [Español](README.es.md) · [Português](README.pt.md) · [Türkçe](README.tr.md) · [Bahasa Indonesia](README.id.md) · [ไทย](README.th.md) · [Tiếng Việt](README.vi.md) · [العربية](README.ar.md) · [فارسی](README.fa.md)

**外部インテグレータ、二次開発およびサードパーティ クライアント**向けの API ドキュメント。すべてのパスは展開ドメインに対する相対パスです。

| 文書 | 観客 | 内容 |
|---|---|---|
| [subscription.md](./subscription.ja.md) | 加入者 / クライアント開発者 | サブスクリプションエンドポイント (`/sub/:token`): フォーマット、ヘッダー、ステータスコード、例 |
| [ポータル-api.md](README.ja.md) | ポータルフロントエンド/サードパーティクライアント | クライアント ポータル API (`/api/v1/client-auth/*`、`/api/v1/public/*`): サインアップ、サインイン、プラン、注文、残高、チケット |
| [チャージコールバック.md](./charge-callback.ja.md) | 決済プロバイダー | 支払いコールバックの署名と検証 |
| [admin-api.ja.md](./admin-api.ja.md) | パネル開発・自動化 | 完全な管理 API ルート テーブル (`/api/v1/*`、モジュールごとにグループ化) |

---

## 1. ポートとベース URL

パネルは 3 つのエンジンを同時に公開し、1 つのデータベースを共有できます。

| エンジン | デフォルトのアクセス | 目的 |
|---|---|---|
| メインパネル | `https://<domain>/api/v1` | 管理バックエンド + ポータル + サブスクリプション |
| サブスクリプション エンジン (専用ポート) | `https://<domain>:<port>/:token` | 同時実行性の高いサブスクリプション取得、`/sub` プレフィックスなし |
| ポータルエンジン（専用ポート） | `https://<portal-domain>[:port]/api/v1` | クライアント専用ポータル、`/` のページ |

> リバース プロキシは、ページ パスと `/api/v1` の両方を転送する必要があります。 `X-Forwarded-Proto` / `X-Forwarded-Host` は、実際のスキームとドメインを復元できることを光栄に思います。

## 2. 認証（3種類、互換性なし）

| 認証 | 資格情報 | に適用されます |
|---|---|---|
| **管理者認証** | `Authorization: Bearer <admin_token>` (`/api/v1/auth/login` から) | 管理者 API |
| **クライアント認証** | `Authorization: Bearer <client_token>` (`/api/v1/client-auth/login` から) | ポータル API |
| **サブスクリプション トークン** | URL のトークン: `/sub/<token>` | サブスクリプションフェッチ、ヘッダーなし |

- クライアント アクセス トークンは 24 時間持続し、リフレッシュ トークンは 7 日間持続します。 JWT には `tv` (token_version) が含まれているため、ログアウト/強制サインアウトすると古いトークンがすべて無効になります。
- 管理者の役割: `admin` / `operator` / `super_admin`;一部の書き込み操作には、`RequireRole(RoleAdmin)` 以降が必要です。

## 3. 応答フォーマット

成功 (ほとんどのエンドポイント):

```json
{ "code": 0, "data": { ... }, "message": "ok" }
```

リスト エンドポイント (一部) は、`{ "items": [...], "total": N }` またはベア配列 (以下のエンドポイントごとに記載) を返します。

エラー (HTTP 4xx/5xx):

```json
{ "code": "ERROR_CODE", "message": "human-readable description", "request_id": "…", "can_retry": false }
```

**金額はセント単位の整数です**: `990` = ¥9.90 (`currency: "CNY"`)。
**トラフィックはバイト単位です**: リクエストと応答は両方ともバイトを使用します。

## 4. レート制限とメンテナンス モード

- パブリック エンドポイント (`/api/v1/public/*`、`/sub/*`、サインアップ/サインイン) にはレート制限があります。
- サインインには `LoginRateLimit` も使用され、サブスクリプションの取得には `SubscribeRateLimit` が使用されます。
- メンテナンス モードでは、パブリック エンドポイントとサブスクリプションはメンテナンス メッセージを返します。管理者 API は影響を受けません。

## 5. フロントエンド ページ ルート (API ではない)

これらのルートは HTML/静的アセットを返しますが、API ドキュメントの範囲外です。統合中に除外できるようにリストされています。

| 方法 | パス | 説明 |
|---|---|---|
| GET | `/` | 管理パネル SPA (サインイン後) |
| GET | `/p` | ポータル HTML (専用ポータル エンジン上の `/`) |
| GET | `/:token` | 専用ポートでのサブスクリプションフェッチ (`/sub` プレフィックスなし、`/sub/:token` と同じセマンティクス) |
| GET | `/static/:file` | フロントエンドの静的アセット |

## 6. 真実の情報源

`internal/api/router/router.go` (746 ルート、743 文書化)。 [admin-api.md](README.ja.md) のルート テーブルはコードから生成されます。コードと一致しない場合は、**コードが優先されます**。
