<!-- source: admin-api.md | locale: ja | updated: 2026-10-23 -->
# 管理者 API

> 言語： [简体中文](admin-api.md) · [English](admin-api.en.md) · [繁體中文](admin-api.zh-TW.md) · [Русский](admin-api.ru.md) · 日本語 · [한국어](admin-api.ko.md) · [Deutsch](admin-api.de.md) · [Français](admin-api.fr.md) · [Español](admin-api.es.md) · [Português](admin-api.pt.md) · [Türkçe](admin-api.tr.md) · [Bahasa Indonesia](admin-api.id.md) · [ไทย](admin-api.th.md) · [Tiếng Việt](admin-api.vi.md) · [العربية](admin-api.ar.md) · [فارسی](admin-api.fa.md)

**パネル開発/自動化/インテグレータ**用の完全な管理者 API ルート テーブル。

- ベース URL: `https://<panel-domain>/api/v1`
- 認証: `Authorization: Bearer <admin_token>` (`POST /api/v1/auth/login` から)
- 一般的な応答/エラー形式とレート制限規則: [README.md](./README.ja.md) を参照してください。
- 真実の情報源: `internal/api/router/router.go` (739 管理ルート)。この表が一致しない場合、**コードが優先されます**

## 許可マーカー

| マーカー | 意味 |
|---|---|
| 公開 | サインインは必要ありません (メンテナンス ガード/レート制限が引き続き適用される場合があります) |
| パブリック・レート制限あり | サインイン不要、専用のレート制限あり |
| 要サインイン | サインインした管理者トークン (`AuthRequired`) が必要です |
| 管理者 | `admin` (`RequireRole(RoleAdmin)`) 以上のロールが必要です |
| スーパー管理者 | 役割 = `super_admin` が必要です |

役割レベル: `viewer(10) < operator(20) < admin(30) < super_admin(40)`;役割が不十分な場合は、403 `INSUFFICIENT_ROLE` が返されます。

---

## クイックスタート (実際の例)

これらの例は、実際の地域の反応から得られたものです。トークンとデータは編集されます。

### 1. サインインしてトークンを取得する

```bash
curl -X POST https://<panel-domain>/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"your-password"}'
```

応答:

```json
{
  "data": {
    "access_token": "<JWT, valid 24h>",
    "refresh_token": "<JWT, valid 7 days>",
    "user": { "id": 1, "role": "super_admin", "username": "admin" }
  }
}
```

### 2. トークンを使用してビジネス エンドポイントを呼び出す

```bash
curl https://<panel-domain>/api/v1/clients?limit=1 \
  -H 'Authorization: Bearer <access_token>'
```

応答 (リストは `items` + `total` 構造を使用します):

```json
{
  "data": {
    "items": [
      {
        "id": 27,
        "email": "user@example.com",
        "status": "active",
        "quota_bytes": 10737418240,
        "used_bytes": 0,
        "plan_id": 1,
        "expires_at": null,
        "created_at": "2026-09-30T15:54:32+08:00"
      }
    ],
    "total": 22,
    "limit": 1,
    "offset": 0
  }
}
```

### 3. エラー応答の例

**401 未認証:**

```json
{
  "code": "MISSING_TOKEN",
  "message": "missing authorization header",
  "request_id": "a02cfdf9-6be0-46be-8584-824caab34dc7",
  "can_retry": false,
  "requires_action": false
}
```

**404 リソースが見つかりません:**

```json
{
  "code": "CLIENT_NOT_FOUND",
  "message": "client not found",
  "request_id": "099949bd-4ded-468d-a6f8-f8e798af2273",
  "can_retry": false,
  "requires_action": false
}
```

### 4. ページネーション

すべてのリスト エンドポイントは、`limit` (ページ サイズ、デフォルト 20) および `offset` (0 ベース) を使用し、`{items, total, limit, offset}` を返します。一部の従来のエンドポイントは、ベア配列 (テーブルごとに記載) を返します。

---

## 内容

- [管理者の認証とセキュリティ](#管理者の認証とセキュリティ)
- [管理者アカウント](#管理者アカウント)
- [顧客管理](#顧客管理)
- [計画テンプレート](#計画テンプレート)
- [プランのバリエーション](#プランのバリエーション)
- [トラフィックパック](#トラフィックパック)
- [クーポン](#クーポン)
- [ノードグループ](#ノードグループ)
- [ノードサーバー](#ノードサーバー)
- [ノード監査ログ](#ノード監査ログ)
- [ノード (レガシー モデル、廃止)](#ノード-レガシー-モデル廃止)
- [地域概要](#地域概要)
- [定期購入](#定期購入)
- [購読形式](#購読形式)
- [サブスクリプションエンドポイント](#サブスクリプションエンドポイント)
- [ドメインを共有する](#ドメインを共有する)
- [Cloudflare IP](#cloudflare-ip)
- [サブスクリプション構成 (Mihomo/Clash)](#サブスクリプション構成-mihomoclash)
- [リチャージサイト](#リチャージサイト)
- [リチャージ注文](#リチャージ注文)
- [カードをリチャージする](#カードをリチャージする)
- [チャージコールバックイベント](#チャージコールバックイベント)
- [チャージコールバックの再試行](#チャージコールバックの再試行)
- [顧客残高](#顧客残高)
- [ギフト](#ギフト)
- [紹介構成](#紹介構成)
- [紹介の引き出し](#紹介の引き出し)
- [IP許可リスト(概要)](#ip許可リスト概要)
- [顧客の OAuth バインディング](#顧客の-oauth-バインディング)
- [チケット（管理者）](#チケット管理者)
- [記事](#記事)
- [共有アカウント](#共有アカウント)
- [証明書](#証明書)
- [証明書チャネル](#証明書チャネル)
- [EAB構成](#eab構成)
- [ACME アカウント](#acme-アカウント)
- [内蔵 CA](#内蔵-ca)
- [地理的ルール](#地理的ルール)
- [WAF ルール ライブラリ](#waf-ルール-ライブラリ)
- [ログ管理](#ログ管理)
- [バックアップ](#バックアップ)
- [システム設定](#システム設定)
- [セキュリティ監査](#セキュリティ監査)
- [メトリクス](#メトリクス)
- [ポータルテンプレート](#ポータルテンプレート)
- [エージェントのインストール](#エージェントのインストール)
- [チャージコールバックエントリ](#チャージコールバックエントリ)
- [インバウンド (ノードあり)](#インバウンド-ノードあり)
- [パブリック構成](#パブリック構成)
- [クライアント認証とビジネス (管理エンジンで同じ設定)](#クライアント認証とビジネス-管理エンジンで同じ設定)
- [定期購入の配信](#定期購入の配信)
- [ランタイム構成ワークフロー](#ランタイム構成ワークフロー)
- [サービス アクセス ポリシー (廃止)](#サービス-アクセス-ポリシー-廃止)
- [トラフィッククォータの監視](#トラフィッククォータの監視)
- [健康診断](#健康診断)
- [ポータルページ](#ポータルページ)
- [ACME の課題](#acme-の課題)

---

### 管理者の認証とセキュリティ

管理パネルのサインイン、サインアウト、パスワード変更、2FA (TOTP)、Passkey (WebAuthn)、およびワンタイム チケット。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| POST | `/api/v1/auth/2fa/disable` | 要サインイン | 無効にする |
| POST | `/api/v1/auth/2fa/enable` | 要サインイン | 有効にする |
| POST | `/api/v1/auth/2fa/recovery-codes` | 要サインイン | リカバリーコードを再生成する |
| POST | `/api/v1/auth/2fa/setup` | 要サインイン | 設定 |
| GET | `/api/v1/auth/2fa/status` | 要サインイン | ステータス |
| POST | `/api/v1/auth/change-password` | 要サインイン | パスワードを変更する |
| POST | `/api/v1/auth/login` | 公開 | サインイン |
| POST | `/api/v1/auth/login/2fa` | 公開 | 動的コードサインイン (2FA) |
| GET | `/api/v1/auth/login/2fa-status` | 公開 | アカウントで 2FA が有効になっているかどうかを確認する |
| POST | `/api/v1/auth/logout` | 要サインイン | サインアウト |
| GET | `/api/v1/auth/me` | 要サインイン | 現在サインインしているユーザー |
| GET | `/api/v1/auth/passkey/credentials` | 要サインイン | Passkey 資格情報リスト |
| DELETE | `/api/v1/auth/passkey/credentials/:id` | 要サインイン | Passkey 資格情報を削除する |
| POST | `/api/v1/auth/passkey/login/begin` | 公開 | サインインを開始する |
| POST | `/api/v1/auth/passkey/login/finish` | 公開 | サインインを完了する |
| POST | `/api/v1/auth/passkey/register/begin` | 要サインイン | 登録を開始する |
| POST | `/api/v1/auth/passkey/register/finish` | 要サインイン | 登録を完了する |
| GET | `/api/v1/auth/passkey/rpids` | 要サインイン | RP ID 許可リスト |
| POST | `/api/v1/auth/passkey/rpids` | 要サインイン | RP ID 許可リストを更新する |
| GET | `/api/v1/auth/passkey/status` | 公開 | ステータス |
| POST | `/api/v1/auth/refresh` | 公開 | リフレッシュ |
| POST | `/api/v1/auth/ticket` | 要サインイン | 1 回限りの有効期間の短いチケットを発行する |

### 管理者アカウント

管理パネルのユーザーアカウント。リストには管理者が必要です。作成/更新/削除にはスーパー管理者が必要です (権限昇格を防ぎます)。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| GET | `/api/v1/users` | 管理者 | リスト |
| POST | `/api/v1/users` | スーパー管理者 | 作成 |
| DELETE | `/api/v1/users/:id` | スーパー管理者 | 削除 |
| PUT | `/api/v1/users/:id` | スーパー管理者 | アップデート |

### 顧客管理

CRUD、一時停止/再開、アップグレード/ダウングレード、更新、サブスクライバー (ポータル アカウント) の使用量のリセット。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| GET | `/api/v1/clients` | 要サインイン | 顧客をリストアップする |
| POST | `/api/v1/clients` | 要サインイン | 顧客を作成します。本体：`{"email":"user@example.com","password":"plaintext-password","plan_id":1,"quota_bytes":10737418240,"expires_at":"2026-12-31T00:00:00Z","remark":""}`。 `plan_id` は必須です。 `password` は空の場合があります (顧客は後でポータルで設定できます) |
| DELETE | `/api/v1/clients/:id` | 要サインイン | 顧客の削除 |
| GET | `/api/v1/clients/:id` | 要サインイン | 顧客を獲得する |
| PUT | `/api/v1/clients/:id` | 要サインイン | 顧客を更新する |
| POST | `/api/v1/clients/:id/downgrade` | 要サインイン | 顧客をダウングレードする |
| GET | `/api/v1/clients/:id/orders` | 要サインイン | 顧客注文リスト |
| POST | `/api/v1/clients/:id/renew` | 要サインイン | 顧客を更新する |
| POST | `/api/v1/clients/:id/reset-usage` | 要サインイン | 使用量をリセットする |
| POST | `/api/v1/clients/:id/restore` | 要サインイン | 顧客の履歴書 |
| POST | `/api/v1/clients/:id/suspend` | 要サインイン | 顧客の一時停止 |
| POST | `/api/v1/clients/:id/upgrade` | 要サインイン | 顧客をアップグレードする |

### 計画テンプレート

プラン (バリアント、請求サイクル、価格、トラフィック クォータ) およびテンプレート レベルのノード認可。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| GET | `/api/v1/plan-templates` | 要サインイン | テンプレート一覧 |
| POST | `/api/v1/plan-templates` | 要サインイン | テンプレートの作成 |
| DELETE | `/api/v1/plan-templates/:id` | 要サインイン | テンプレートの削除 |
| GET | `/api/v1/plan-templates/:id` | 要サインイン | テンプレートの詳細 |
| PUT | `/api/v1/plan-templates/:id` | 要サインイン | テンプレートを更新する |
| GET | `/api/v1/plan-templates/:id/accessible-nodes` | 要サインイン | 利用可能なノードを取得する |
| GET | `/api/v1/plan-templates/:id/billing-node-options` | 要サインイン | 課金対象ノードのオプションを取得する |
| PUT | `/api/v1/plan-templates/:id/node-groups` | 要サインイン | テンプレートノードグループを設定する |
| POST | `/api/v1/plan-templates/:id/sync-accounts` | 要サインイン | アカウントをテンプレートに同期する |
| GET | `/api/v1/plan-templates/:id/variants` | 要サインイン | バリアントリスト |
| POST | `/api/v1/plan-templates/:id/variants` | 要サインイン | バリアントの作成 |
| POST | `/api/v1/plan-templates/sync-all-accounts` | 要サインイン | すべてのアカウントを同期する |

### プランのバリエーション

プラン バリアントを更新および削除します (作成はプラン テンプレートの下にネストされます)。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| DELETE | `/api/v1/plan-variants/:id` | 要サインイン | バリアントの削除 |
| PUT | `/api/v1/plan-variants/:id` | 要サインイン | バリアントを更新する |

### トラフィックパック

個別に購入できるトラフィック パック。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| GET | `/api/v1/traffic-packs` | 要サインイン | トラフィックパックリスト |
| POST | `/api/v1/traffic-packs` | 要サインイン | トラフィックパックを作成する |
| DELETE | `/api/v1/traffic-packs/:id` | 要サインイン | トラフィックパックを削除する |
| PUT | `/api/v1/traffic-packs/:id` | 要サインイン | トラフィックパックを更新する |

### クーポン

クーポンを作成、検証、引き換えます。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| GET | `/api/v1/promotions` | 要サインイン | クーポン一覧 |
| POST | `/api/v1/promotions` | 要サインイン | クーポンの作成 |
| DELETE | `/api/v1/promotions/:id` | 要サインイン | クーポンを削除する |
| PUT | `/api/v1/promotions/:id` | 要サインイン | クーポンを更新する |
| POST | `/api/v1/promotions/validate` | 要サインイン | クーポンの検証 |

### ノードグループ

ノードをプランによって参照されるグループに編成します。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| GET | `/api/v1/node-groups` | 要サインイン | ノードグループのリストを表示する |
| POST | `/api/v1/node-groups` | 要サインイン | ノードグループの作成 |
| DELETE | `/api/v1/node-groups/:id` | 要サインイン | ノードグループの削除 |
| PUT | `/api/v1/node-groups/:id` | 要サインイン | ノードグループの更新 |
| GET | `/api/v1/node-groups/:id/members` | 要サインイン | グループメンバーをリストする |
| POST | `/api/v1/node-groups/:id/members` | 要サインイン | グループメンバーを追加 |
| DELETE | `/api/v1/node-groups/:id/members/:mid` | 要サインイン | グループメンバーを削除する |

### ノードサーバー

コア リソース: ノード CRUD、コア (xray/sing-box) 構成配信、インバウンド/アウトバウンド/エンドポイント構成、プロセス制御、証明書、WAF、レート制限、ポート ホッピング、チェーン プロキシ、ミラー/プライマリ バックアップ、エージェントのインストール/アップグレード、xray ランタイム制御。フィーチャ ファミリごとにグループ化された多数のサブパス。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| GET | `/api/v1/node-servers` | 管理者 | リスト |
| POST | `/api/v1/node-servers` | 管理者 | 作成 |
| DELETE | `/api/v1/node-servers/:id` | 管理者 | 削除 |
| GET | `/api/v1/node-servers/:id` | 管理者 | 詳細 |
| PUT | `/api/v1/node-servers/:id` | 管理者 | アップデート |
| GET | `/api/v1/node-servers/:id/binaries` | 管理者 | インストールされているコアのリスト |
| POST | `/api/v1/node-servers/:id/binaries/ech-generate` | 管理者 | ECH キーペアの生成 |
| POST | `/api/v1/node-servers/:id/binaries/install` | 管理者 | コアのインストール/アップグレード |
| GET | `/api/v1/node-servers/:id/binaries/interfaces` | 管理者 | ネットワークインターフェース一覧 |
| POST | `/api/v1/node-servers/:id/binaries/mldsa65-key` | 管理者 | MLDSA65キーを生成する |
| POST | `/api/v1/node-servers/:id/binaries/reality-key` | 管理者 | REALITY キーペアの生成 |
| POST | `/api/v1/node-servers/:id/binaries/reality-public-key` | 管理者 | REALITY秘密鍵から公開鍵を導出 |
| GET | `/api/v1/node-servers/:id/binaries/releases` | 管理者 | 利用可能なバージョン |
| POST | `/api/v1/node-servers/:id/binaries/vlessenc` | 管理者 | VLESS 暗号化パラメータを生成する |
| POST | `/api/v1/node-servers/:id/binaries/wg-key` | 管理者 | WireGuardキーを生成する |
| POST | `/api/v1/node-servers/:id/binaries/x25519-key` | 管理者 | X25519 キーペアの生成 |
| GET | `/api/v1/node-servers/:id/bootstrap-cmd` | 管理者 | エージェントブートストラップコマンドの取得 |
| POST | `/api/v1/node-servers/:id/bootstrap-token` | 管理者 | ブートストラップトークンを生成する |
| POST | `/api/v1/node-servers/:id/caddy/batch-update-log-enabled` | 管理者 | ログスイッチの一括更新 |
| POST | `/api/v1/node-servers/:id/caddy/batch-update-ports` | 管理者 | ポートの一括更新 |
| POST | `/api/v1/node-servers/:id/caddy/generate-config` | 管理者 | 構成の生成 |
| DELETE | `/api/v1/node-servers/:id/caddy/l4-rules` | 管理者 | すべての L4 ルールを削除します |
| POST | `/api/v1/node-servers/:id/caddy/save-and-apply` | 管理者 | 保存して適用する |
| GET | `/api/v1/node-servers/:id/caddy/sites` | 管理者 | サイトリスト |
| POST | `/api/v1/node-servers/:id/caddy/sites` | 管理者 | サイトの作成 |
| DELETE | `/api/v1/node-servers/:id/caddy/sites/:siteId` | 管理者 | サイトの削除 |
| PUT | `/api/v1/node-servers/:id/caddy/sites/:siteId` | 管理者 | 更新サイト |
| DELETE | `/api/v1/node-servers/:id/caddy/sni-rules` | 管理者 | すべての SNI ルールを削除します |
| POST | `/api/v1/node-servers/:id/caddy/sni-rules/toggle` | 管理者 | SNI ルールの切り替え |
| GET | `/api/v1/node-servers/:id/caddy/sni-settings` | 管理者 | SNIの設定 |
| POST | `/api/v1/node-servers/:id/caddy/sni-settings` | 管理者 | SNI設定を保存する |
| GET | `/api/v1/node-servers/:id/certificates` | 管理者 | ノード証明書リスト |
| DELETE | `/api/v1/node-servers/:id/certificates/:cid` | 管理者 | 証明書のバインドを解除する |
| POST | `/api/v1/node-servers/:id/certificates/:cid` | 管理者 | 証明書をバインドする |
| GET | `/api/v1/node-servers/:id/chain-config` | 管理者 | 連鎖構成を生成する |
| GET | `/api/v1/node-servers/:id/chain-health` | 管理者 | チェーンの健全性ステータス |
| POST | `/api/v1/node-servers/:id/control` | スーパー管理者 | コントロールノードコア。本体: `{"core_type":"xray","action":"start"}` (アクション = 開始/停止/再起動)。 `core_type` は `xray`/`singbox` にすることができます |
| GET | `/api/v1/node-servers/:id/degradation-status` | 管理者 | 劣化ステータス |
| POST | `/api/v1/node-servers/:id/distribute-cert` | 管理者 | 証明書の配布 |
| GET | `/api/v1/node-servers/:id/endpoint-configs` | 管理者 | リスト |
| POST | `/api/v1/node-servers/:id/endpoint-configs` | 管理者 | 作成 |
| DELETE | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | 管理者 | 削除 |
| GET | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | 管理者 | 詳細 |
| PUT | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | 管理者 | アップデート |
| POST | `/api/v1/node-servers/:id/failover` | 管理者 | このノードのフェイルオーバーをトリガーします |
| GET | `/api/v1/node-servers/:id/failover-status` | 管理者 | フェイルオーバーステータス |
| POST | `/api/v1/node-servers/:id/force-reconnect` | 管理者 | 強制再接続 |
| GET | `/api/v1/node-servers/:id/free-port` | 管理者 | 空きポートをリクエストする |
| POST | `/api/v1/node-servers/:id/geo-rules/push` | 管理者 | このノードにプッシュする |
| GET | `/api/v1/node-servers/:id/inbound-configs` | 管理者 | リスト |
| POST | `/api/v1/node-servers/:id/inbound-configs` | 管理者 | 作成 |
| DELETE | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | 管理者 | 削除 |
| GET | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | 管理者 | 詳細 |
| PUT | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | 管理者 | アップデート |
| GET | `/api/v1/node-servers/:id/inbound-configs/:cfgId/share-links` | 管理者 | リンクとスニペットを共有する |
| POST | `/api/v1/node-servers/:id/inbound-configs/:cfgId/users/:email/pause` | 管理者 | このユーザーを一時停止します |
| POST | `/api/v1/node-servers/:id/inbound-configs/:cfgId/users/:email/resume` | 管理者 | このユーザーを復元する |
| POST | `/api/v1/node-servers/:id/inbound-configs/reset-all-traffic` | 管理者 | すべての受信トラフィックをリセットします |
| GET | `/api/v1/node-servers/:id/install-script` | 管理者 | エージェントのインストール スクリプトを取得する |
| GET | `/api/v1/node-servers/:id/metrics-history` | 管理者 | メトリクス履歴を取得する |
| GET | `/api/v1/node-servers/:id/mirror-status` | 管理者 | ミラーステータス |
| POST | `/api/v1/node-servers/:id/nginx/batch-update-ports` | 管理者 | ポートの一括更新 |
| POST | `/api/v1/node-servers/:id/nginx/generate-config` | 管理者 | 構成の生成 |
| DELETE | `/api/v1/node-servers/:id/nginx/l4-rules` | 管理者 | すべての L4 ルールを削除します |
| POST | `/api/v1/node-servers/:id/nginx/save-and-apply` | 管理者 | 保存して適用する |
| GET | `/api/v1/node-servers/:id/nginx/settings` | 管理者 | 設定の読み取り |
| POST | `/api/v1/node-servers/:id/nginx/settings` | 管理者 | 設定を保存する |
| GET | `/api/v1/node-servers/:id/nginx/sites` | 管理者 | サイトリスト |
| POST | `/api/v1/node-servers/:id/nginx/sites` | 管理者 | サイトの作成 |
| DELETE | `/api/v1/node-servers/:id/nginx/sites/:siteId` | 管理者 | サイトの削除 |
| PUT | `/api/v1/node-servers/:id/nginx/sites/:siteId` | 管理者 | 更新サイト |
| DELETE | `/api/v1/node-servers/:id/nginx/sni-rules` | 管理者 | すべての SNI ルールを削除します |
| POST | `/api/v1/node-servers/:id/nginx/sni-rules/toggle` | 管理者 | SNI ルールの切り替え |
| PUT | `/api/v1/node-servers/:id/node-type` | 管理者 | ノードタイプの更新 |
| GET | `/api/v1/node-servers/:id/online-users` | 管理者 | オンラインユーザー |
| GET | `/api/v1/node-servers/:id/outbound-configs` | 管理者 | リスト |
| POST | `/api/v1/node-servers/:id/outbound-configs` | 管理者 | 作成 |
| DELETE | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | 管理者 | 削除 |
| GET | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | 管理者 | 詳細 |
| PUT | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | 管理者 | アップデート |
| PUT | `/api/v1/node-servers/:id/outbound-configs/reorder` | 管理者 | 並べ替え |
| GET | `/api/v1/node-servers/:id/port-hop/rules` | 管理者 | ポートホッピングルールのプレビュー |
| GET | `/api/v1/node-servers/:id/protocols` | 管理者 | プロトコルリスト |
| GET | `/api/v1/node-servers/:id/protocols/:coreType` | 管理者 | プロトコル構成 |
| PUT | `/api/v1/node-servers/:id/protocols/:coreType` | 管理者 | プロトコル設定を保存する |
| GET | `/api/v1/node-servers/:id/protocols/:coreType/config-file` | 管理者 | コア構成ファイルを読み取る |
| GET | `/api/v1/node-servers/:id/protocols/:coreType/section/:section` | 管理者 | プロトコルセクションを読む |
| PUT | `/api/v1/node-servers/:id/protocols/:coreType/section/:section` | 管理者 | プロトコルセクションの保存 |
| POST | `/api/v1/node-servers/:id/protocols/:coreType/validate` | 管理者 | 構成を検証します (実際のコアの予行演習) |
| GET | `/api/v1/node-servers/:id/proxy-status` | 管理者 | リモートプロキシステータス |
| POST | `/api/v1/node-servers/:id/proxy/batch-update-ports` | 管理者 | ポートの一括更新 |
| GET | `/api/v1/node-servers/:id/proxy/config` | 管理者 | 構成の読み取り |
| PUT | `/api/v1/node-servers/:id/proxy/config` | 管理者 | 構成を更新する |
| POST | `/api/v1/node-servers/:id/proxy/control` | 管理者 | 連鎖的なプロキシ登録制御 |
| POST | `/api/v1/node-servers/:id/proxy/download` | 管理者 | プロキシコアバージョンのダウンロード |
| POST | `/api/v1/node-servers/:id/proxy/generate-config` | 管理者 | 構成の生成 |
| POST | `/api/v1/node-servers/:id/proxy/guardian` | 管理者 | スーパーバイザ設定を更新する |
| POST | `/api/v1/node-servers/:id/proxy/install` | 管理者 | プロキシのインストール |
| DELETE | `/api/v1/node-servers/:id/proxy/l4-rules` | 管理者 | すべての L4 ルールを削除します |
| GET | `/api/v1/node-servers/:id/proxy/l4-settings` | 管理者 | L4設定 |
| POST | `/api/v1/node-servers/:id/proxy/l4-settings` | 管理者 | L4設定の保存 |
| GET | `/api/v1/node-servers/:id/proxy/log-settings` | 管理者 | ログ設定 |
| POST | `/api/v1/node-servers/:id/proxy/log-settings` | 管理者 | ログ設定の保存 |
| GET | `/api/v1/node-servers/:id/proxy/nginx-settings` | 管理者 | nginxの設定 |
| POST | `/api/v1/node-servers/:id/proxy/nginx-settings` | 管理者 | nginx設定を保存する |
| GET | `/api/v1/node-servers/:id/proxy/releases` | 管理者 | 利用可能なバージョン |
| POST | `/api/v1/node-servers/:id/proxy/reload` | 管理者 | 設定をリロードする |
| POST | `/api/v1/node-servers/:id/proxy/restart` | 管理者 | 再起動 |
| GET | `/api/v1/node-servers/:id/proxy/sites` | 管理者 | サイトリスト |
| POST | `/api/v1/node-servers/:id/proxy/sites` | 管理者 | サイトの作成 |
| DELETE | `/api/v1/node-servers/:id/proxy/sites/:siteId` | 管理者 | サイトの削除 |
| PUT | `/api/v1/node-servers/:id/proxy/sites/:siteId` | 管理者 | 更新サイト |
| DELETE | `/api/v1/node-servers/:id/proxy/sni-rules` | 管理者 | すべての SNI ルールを削除します |
| POST | `/api/v1/node-servers/:id/proxy/sni-rules/toggle` | 管理者 | SNI ルールの切り替え |
| GET | `/api/v1/node-servers/:id/proxy/sni-settings` | 管理者 | SNIの設定 |
| POST | `/api/v1/node-servers/:id/proxy/sni-settings` | 管理者 | SNI設定を保存する |
| POST | `/api/v1/node-servers/:id/proxy/start` | 管理者 | スタート |
| GET | `/api/v1/node-servers/:id/proxy/status` | 管理者 | プロキシ処理ステータス |
| POST | `/api/v1/node-servers/:id/proxy/stop` | 管理者 | 停止 |
| POST | `/api/v1/node-servers/:id/proxy/sync` | 管理者 | プロキシ構成を同期する |
| GET | `/api/v1/node-servers/:id/rate-limit` | 管理者 | 構成の読み取り |
| PUT | `/api/v1/node-servers/:id/rate-limit` | 管理者 | 構成を更新する |
| GET | `/api/v1/node-servers/:id/rate-limit/status` | 管理者 | ステータス |
| GET | `/api/v1/node-servers/:id/recover-mode` | 管理者 | 読み取りノード回復モード |
| PUT | `/api/v1/node-servers/:id/recover-mode` | 管理者 | ノード回復モードを設定する |
| POST | `/api/v1/node-servers/:id/revoke-cert` | 管理者 | エージェント証明書を取り消す |
| GET | `/api/v1/node-servers/:id/services` | 管理者 | リスト |
| POST | `/api/v1/node-servers/:id/services` | 管理者 | 作成 |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId` | 管理者 | 削除 |
| GET | `/api/v1/node-servers/:id/services/:serviceId` | 管理者 | 詳細 |
| PUT | `/api/v1/node-servers/:id/services/:serviceId` | 管理者 | アップデート |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId/crash-logs` | 管理者 | クラッシュログをクリアする |
| GET | `/api/v1/node-servers/:id/services/:serviceId/crash-logs` | 管理者 | クラッシュログリスト |
| GET | `/api/v1/node-servers/:id/services/:serviceId/crash-logs/:logId` | 管理者 | クラッシュログの詳細 |
| GET | `/api/v1/node-servers/:id/services/:serviceId/endpoints` | 管理者 | エンドポイントリスト |
| POST | `/api/v1/node-servers/:id/services/:serviceId/endpoints` | 管理者 | エンドポイントの作成 |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId/endpoints/:endpointId` | 管理者 | エンドポイントの削除 |
| PUT | `/api/v1/node-servers/:id/services/:serviceId/endpoints/:endpointId` | 管理者 | エンドポイントの更新 |
| PUT | `/api/v1/node-servers/:id/services/:serviceId/guardian` | 管理者 | スーパーバイザ設定を更新する |
| GET | `/api/v1/node-servers/:id/services/:serviceId/process-status` | 管理者 | 処理ステータスの取得 |
| POST | `/api/v1/node-servers/:id/services/:serviceId/restart` | 管理者 | 再起動 |
| POST | `/api/v1/node-servers/:id/services/:serviceId/start` | 管理者 | スタート |
| POST | `/api/v1/node-servers/:id/services/:serviceId/stop` | 管理者 | 停止 |
| GET | `/api/v1/node-servers/:id/services/process-status` | 管理者 | すべてのプロセスのステータス |
| GET | `/api/v1/node-servers/:id/share-domains` | 管理者 | このノードのドメインを共有する |
| POST | `/api/v1/node-servers/:id/share-domains` | 管理者 | 作成 |
| DELETE | `/api/v1/node-servers/:id/share-domains/:domainId` | 管理者 | 削除 |
| PUT | `/api/v1/node-servers/:id/share-domains/:domainId` | 管理者 | アップデート |
| DELETE | `/api/v1/node-servers/:id/share-domains/all` | 管理者 | すべてクリア |
| POST | `/api/v1/node-servers/:id/singbox/control/firewall/apply` | 管理者 | ポートホッピングファイアウォールルールを適用する |
| POST | `/api/v1/node-servers/:id/singbox/control/firewall/remove` | 管理者 | ポートホッピングファイアウォールルールを削除する |
| POST | `/api/v1/node-servers/:id/speed-test` | 管理者 | ノード速度テスト |
| GET | `/api/v1/node-servers/:id/status` | 管理者 | ノードのステータス |
| POST | `/api/v1/node-servers/:id/sync` | 管理者 | ノード構成の同期 (生成およびプッシュ) |
| POST | `/api/v1/node-servers/:id/sync-data` | スーパー管理者 | パネル データをノードにプッシュします (ユーザー/サブスクリプション/プランの完全同期) |
| POST | `/api/v1/node-servers/:id/sync-mirror` | 管理者 | 同期ミラーノード |
| GET | `/api/v1/node-servers/:id/tags` | 管理者 | タグ一覧 |
| POST | `/api/v1/node-servers/:id/tags` | 管理者 | タグを追加 |
| DELETE | `/api/v1/node-servers/:id/tags/:tagKey` | 管理者 | タグの削除 |
| GET | `/api/v1/node-servers/:id/traffic-history` | 管理者 | 交通履歴 |
| POST | `/api/v1/node-servers/:id/traffic-resume` | 管理者 | 超過のために一時停止されたトラフィックを復元する |
| GET | `/api/v1/node-servers/:id/traffic-usage` | 管理者 | トラフィック使用量 |
| POST | `/api/v1/node-servers/:id/update-agent` | 管理者 | アップグレードエージェント |
| GET | `/api/v1/node-servers/:id/used-server-names` | 管理者 | 使用中 REALITY SNI リスト |
| GET | `/api/v1/node-servers/:id/waf/config` | 管理者 | 構成の読み取り |
| PUT | `/api/v1/node-servers/:id/waf/config` | 管理者 | 設定を保存する |
| POST | `/api/v1/node-servers/:id/waf/disable-domain` | 管理者 | ドメイン保護を無効にする |
| GET | `/api/v1/node-servers/:id/waf/domain-config` | 管理者 | ドメイン構成リスト |
| PUT | `/api/v1/node-servers/:id/waf/domain-config` | 管理者 | ドメイン構成を一括保存する |
| DELETE | `/api/v1/node-servers/:id/waf/domain-config/:domain` | 管理者 | ドメイン構成を削除する |
| GET | `/api/v1/node-servers/:id/waf/domain-config/:domain` | 管理者 | ドメイン構成の詳細 |
| PUT | `/api/v1/node-servers/:id/waf/domain-config/:domain` | 管理者 | ドメイン構成を保存する |
| GET | `/api/v1/node-servers/:id/waf/domain-rules` | 管理者 | ドメインルールディレクトリ |
| GET | `/api/v1/node-servers/:id/waf/domain-rules/:domain/:name` | 管理者 | ドメインルールファイルの読み取り |
| PUT | `/api/v1/node-servers/:id/waf/domain-rules/:domain/:name` | 管理者 | ドメインルールファイルを保存する |
| GET | `/api/v1/node-servers/:id/waf/domain-status` | 管理者 | ドメイン保護ステータス |
| POST | `/api/v1/node-servers/:id/waf/enable-domain` | 管理者 | ドメイン保護を有効にする |
| POST | `/api/v1/node-servers/:id/waf/push` | 管理者 | このノードにプッシュする |
| GET | `/api/v1/node-servers/:id/waf/rules` | 管理者 | ノードWAFルールリスト |
| DELETE | `/api/v1/node-servers/:id/waf/rules/:name` | 管理者 | ノード WAF ルールの削除 |
| GET | `/api/v1/node-servers/:id/waf/rules/:name` | 管理者 | ノード WAF ルールの詳細 |
| PUT | `/api/v1/node-servers/:id/waf/rules/:name` | 管理者 | ノード WAF ルールの保存 |
| GET | `/api/v1/node-servers/:id/waf/status` | 管理者 | ノードWAFのステータス |
| PUT | `/api/v1/node-servers/:id/waf/status` | 管理者 | トグルノード WAF |
| POST | `/api/v1/node-servers/:id/waf/sync` | 管理者 | このノードに同期します |
| GET | `/api/v1/node-servers/:id/warp` | 管理者 | リスト |
| DELETE | `/api/v1/node-servers/:id/warp/:warpId` | 管理者 | 削除 |
| GET | `/api/v1/node-servers/:id/warp/config` | 管理者 | 構成の読み取り |
| POST | `/api/v1/node-servers/:id/warp/generate-config` | 管理者 | 構成の生成 |
| POST | `/api/v1/node-servers/:id/warp/register` | 管理者 | 登録する |
| GET | `/api/v1/node-servers/:id/warp/status` | 管理者 | ステータス |
| GET | `/api/v1/node-servers/:id/xray/control/balancer/:tag` | 管理者 | バランサー情報の取得 |
| POST | `/api/v1/node-servers/:id/xray/control/balancer/:tag` | 管理者 | バランサーのターゲットを設定する |
| POST | `/api/v1/node-servers/:id/xray/control/firewall/apply` | 管理者 | ポートホッピングファイアウォールルールを適用する |
| POST | `/api/v1/node-servers/:id/xray/control/firewall/remove` | 管理者 | ポートホッピングファイアウォールルールを削除する |
| POST | `/api/v1/node-servers/:id/xray/control/inbound` | 管理者 | インバウンドを追加 |
| DELETE | `/api/v1/node-servers/:id/xray/control/inbound/:tag` | 管理者 | インバウンドを削除する |
| POST | `/api/v1/node-servers/:id/xray/control/inbound/:tag/user` | 管理者 | ユーザーの追加 |
| DELETE | `/api/v1/node-servers/:id/xray/control/inbound/:tag/user/:email` | 管理者 | ユーザーを削除する |
| POST | `/api/v1/node-servers/:id/xray/control/outbound` | 管理者 | アウトバウンドを追加 |
| DELETE | `/api/v1/node-servers/:id/xray/control/outbound/:tag` | 管理者 | アウトバウンドを削除する |
| POST | `/api/v1/node-servers/:id/xray/control/routing` | 管理者 | リクエストルート |
| POST | `/api/v1/node-servers/:id/xray/control/routing/rule` | 管理者 | ルーティングルールを追加する |
| DELETE | `/api/v1/node-servers/:id/xray/control/routing/rule/:tag` | 管理者 | ルーティング ルールを削除する |
| GET | `/api/v1/node-servers/:id/xray/control/routing/rules` | 管理者 | ルーティングルールをリストする |
| POST | `/api/v1/node-servers/:id/xray/control/sync` | 管理者 | xray 構成同期を手動でトリガーする |
| GET | `/api/v1/node-servers/:id/xray/stats` | 管理者 | 統計の取得 |
| POST | `/api/v1/node-servers/:id/xray/stats/reset/:email` | 管理者 | ユーザー統計をリセットする |
| GET | `/api/v1/node-servers/:id/xray/stats/users` | 管理者 | ユーザー統計を取得する |
| GET | `/api/v1/node-servers/billable` | 管理者 | 課金対象ノードリスト |
| GET | `/api/v1/node-servers/ca-cert` | 管理者 | CA証明書 |
| GET | `/api/v1/node-servers/ca-status` | 管理者 | CA ステータス |
| GET | `/api/v1/node-servers/dashboard` | 管理者 | ダッシュボードのステータス |
| POST | `/api/v1/node-servers/distribute-cert` | 管理者 | 証明書をすべてのノードに配布します |
| POST | `/api/v1/node-servers/failover-check` | 管理者 | フェイルオーバーチェックのトリガー |
| GET | `/api/v1/node-servers/metrics/aggregated` | 管理者 | 集約されたメトリクス |
| POST | `/api/v1/node-servers/migrate-cert` | 管理者 | 証明書の移行 |
| GET | `/api/v1/node-servers/online-summary` | 管理者 | オンライン概要 |
| GET | `/api/v1/node-servers/online-users` | 管理者 | すべてのオンライン ユーザー |
| GET | `/api/v1/node-servers/pending-agent-updates` | 管理者 | アップグレードが保留中のエージェント |
| POST | `/api/v1/node-servers/recommend-region` | 管理者 | おすすめ地域 |
| POST | `/api/v1/node-servers/select-nodes` | 管理者 | サブスクリプション用のノードを選択してください |
| POST | `/api/v1/node-servers/speed-test-all` | 管理者 | すべてのノードの速度テスト |
| GET | `/api/v1/node-servers/speed-test-results` | 管理者 | スピードテストの結果 |
| GET | `/api/v1/node-servers/traffic/stats` | 管理者 | トラフィック統計 |
| POST | `/api/v1/node-servers/update-all-agents` | 管理者 | すべてのエージェントをアップグレードする |
| DELETE | `/api/v1/node-servers/users/:client_id` | 管理者 | 顧客の同期削除 |
| GET | `/api/v1/node-servers/users/retry-queue` | 管理者 | 再試行キューのステータス |
| POST | `/api/v1/node-servers/users/retry-queue/process` | 管理者 | プロセスリトライキュー |
| POST | `/api/v1/node-servers/users/sync` | 管理者 | 単一の顧客を同期する |
| POST | `/api/v1/node-servers/validate-chain-config` | 管理者 | 連鎖構成を検証する |

### ノード監査ログ

ノード側の操作の監査とエクスポート。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| GET | `/api/v1/node-audit-logs` | 要サインイン | ノード監査ログ |
| GET | `/api/v1/node-audit-logs/export` | 要サインイン | ノード監査ログのエクスポート |

### ノード (レガシー モデル、廃止)

> `/api/v1/nodes` CRUD は 2026 年 10 月 20 日に廃止されました。ノードは `/api/v1/node-servers` を通じて均一に管理されます。
> オンライン ユーザーのクエリは `/api/v1/node-servers/{online-users,online-summary,:id/online-users}` のままです。

### 地域概要

地域ごとに集約されたノードビュー。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| GET | `/api/v1/regions/:region/servers` | 要サインイン | このリージョンのノードリスト |
| GET | `/api/v1/regions/overview` | 要サインイン | 地域概要 |

### 定期購入

サブスクリプション記録の管理: 取り消し、トークンのローテーション、アクセス ログ、使用量のリセット。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| GET | `/api/v1/subscriptions` | 要サインイン | リスト |
| POST | `/api/v1/subscriptions` | 要サインイン | 作成 |
| DELETE | `/api/v1/subscriptions/:id` | 要サインイン | 削除 |
| GET | `/api/v1/subscriptions/:id` | 要サインイン | 詳細 |
| PUT | `/api/v1/subscriptions/:id` | 要サインイン | アップデート |
| GET | `/api/v1/subscriptions/:id/access-logs` | 要サインイン | アクセスログ |
| POST | `/api/v1/subscriptions/:id/enable` | 要サインイン | 有効にする |
| POST | `/api/v1/subscriptions/:id/reset-usage` | 要サインイン | 使用量をリセットする |
| POST | `/api/v1/subscriptions/:id/restore` | 要サインイン | 再開 |
| POST | `/api/v1/subscriptions/:id/revoke` | 要サインイン | 取り消し |
| POST | `/api/v1/subscriptions/:id/rotate` | 要サインイン | 回転 |
| GET | `/api/v1/subscriptions/:id/rotations` | 要サインイン | 回転履歴 |

### 購読形式

> 管理側 `/api/v1/subscription-formats` CRUD は 2026 年 10 月 20 日に廃止されました。実際のエントリ ポイントは次のとおりです。
> `/api/v1/mihomo/configs`（サブスクリプションフォーマットセンター）。 `subscription_formats` テーブルは残りますが、
> パブリック `/api/v1/public/client-configs` エンドポイントのシード データとしてのみ使用されます。

### サブスクリプションエンドポイント

専用のサブスクリプション ドメイン (パネルの `/sub` パスにリバース プロキシされます)。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| GET | `/api/v1/subscription-endpoints` | 要サインイン | リスト |
| POST | `/api/v1/subscription-endpoints` | 要サインイン | 作成 |
| DELETE | `/api/v1/subscription-endpoints/:id` | 要サインイン | 削除 |
| GET | `/api/v1/subscription-endpoints/:id` | 要サインイン | 詳細 |
| PUT | `/api/v1/subscription-endpoints/:id` | 要サインイン | アップデート |

### ドメインを共有する

クライアント リンクで使用されるアクセス ドメイン/IP プール。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| GET | `/api/v1/share-domains` | 要サインイン | リスト |
| POST | `/api/v1/share-domains` | 要サインイン | 作成 |
| DELETE | `/api/v1/share-domains/:id` | 要サインイン | 削除 |
| PUT | `/api/v1/share-domains/:id` | 要サインイン | アップデート |
| DELETE | `/api/v1/share-domains/all` | 要サインイン | すべてクリア |
| POST | `/api/v1/share-domains/seed` | 要サインイン | デフォルトデータの初期化 |

### Cloudflare IP

Cloudflare CDN IP プールとソースの管理。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| DELETE | `/api/v1/cfip/entries` | 要サインイン | エントリをクリアする |
| GET | `/api/v1/cfip/entries` | 要サインイン | エントリーリスト |
| POST | `/api/v1/cfip/entries` | 要サインイン | エントリを追加 |
| DELETE | `/api/v1/cfip/entries/:id` | 要サインイン | エントリの削除 |
| POST | `/api/v1/cfip/fetch-all` | 要サインイン | すべてのソースを取得する |
| POST | `/api/v1/cfip/seed` | 要サインイン | デフォルトデータの初期化 |
| GET | `/api/v1/cfip/sources` | 要サインイン | データソースリスト |
| POST | `/api/v1/cfip/sources` | 要サインイン | データソースの作成 |
| DELETE | `/api/v1/cfip/sources/:id` | 要サインイン | データソースの削除 |
| PUT | `/api/v1/cfip/sources/:id` | 要サインイン | データソースを更新する |
| POST | `/api/v1/cfip/sources/:id/fetch` | 要サインイン | このソースを取得します |

### サブスクリプション構成 (Mihomo/Clash)

サブスクリプション テンプレートの構成: グループ、ルール、ルール プロバイダー、YAML プレビュー。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| GET | `/api/v1/mihomo/configs` | 要サインイン | リスト |
| POST | `/api/v1/mihomo/configs` | 要サインイン | 作成 |
| DELETE | `/api/v1/mihomo/configs/:id` | 要サインイン | 削除 |
| GET | `/api/v1/mihomo/configs/:id` | 要サインイン | 詳細 |
| PUT | `/api/v1/mihomo/configs/:id` | 要サインイン | アップデート |
| GET | `/api/v1/mihomo/configs/:id/proxy-groups` | 要サインイン | グループリスト |
| POST | `/api/v1/mihomo/configs/:id/proxy-groups` | 要サインイン | グループの作成 |
| GET | `/api/v1/mihomo/configs/:id/rule-providers` | 要サインイン | ルールプロバイダーのリスト |
| POST | `/api/v1/mihomo/configs/:id/rule-providers` | 要サインイン | ルールプロバイダーの作成 |
| GET | `/api/v1/mihomo/configs/:id/rules` | 要サインイン | ルール一覧 |
| POST | `/api/v1/mihomo/configs/:id/rules` | 要サインイン | ルールの作成 |
| GET | `/api/v1/mihomo/configs/:id/yaml` | 要サインイン | 生成されたYAMLをプレビューする |
| DELETE | `/api/v1/mihomo/proxy-groups/:id` | 要サインイン | グループの削除 |
| DELETE | `/api/v1/mihomo/rule-providers/:id` | 要サインイン | ルールプロバイダーの削除 |
| DELETE | `/api/v1/mihomo/rules/:id` | 要サインイン | ルールの削除 |

### リチャージサイト

サードパーティの支払いサイトの統合: 署名キー、検証ファイル、ドメイン検証、統合ドキュメント。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| GET | `/api/v1/charge-sites` | 要サインイン | サイトリスト |
| POST | `/api/v1/charge-sites` | 要サインイン | サイトの作成 |
| DELETE | `/api/v1/charge-sites/:id` | 要サインイン | サイトの削除 |
| GET | `/api/v1/charge-sites/:id` | 要サインイン | サイト詳細 |
| PUT | `/api/v1/charge-sites/:id` | 要サインイン | 更新サイト |
| POST | `/api/v1/charge-sites/:id/clear-prev-secret` | 要サインイン | 古いキーをクリアする |
| POST | `/api/v1/charge-sites/:id/generate-verify` | 要サインイン | 検証ファイルの生成 |
| POST | `/api/v1/charge-sites/:id/generate-verify-file` | 要サインイン | 検証ファイルの生成 |
| GET | `/api/v1/charge-sites/:id/integration-doc` | 要サインイン | 統合ドキュメントを入手する |
| POST | `/api/v1/charge-sites/:id/rotate-secret` | 要サインイン | 署名キーをローテーションする |
| POST | `/api/v1/charge-sites/:id/verify-dns` | 要サインイン | ドメイン DNS を確認する |
| POST | `/api/v1/charge-sites/:id/verify-domain` | 要サインイン | ドメイン HTTP チャレンジを検証する |
| POST | `/api/v1/charge-sites/:id/verify-domain-dns` | 要サインイン | ドメイン DNS を確認する |
| POST | `/api/v1/charge-sites/:id/verify-domain-http` | 要サインイン | ドメイン HTTP チャレンジを検証する |

### リチャージ注文

リチャージ注文のクエリ、返金、CSV のエクスポート。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| GET | `/api/v1/charge-orders` | 要サインイン | 注文リスト |
| GET | `/api/v1/charge-orders/:id` | 要サインイン | 注文の詳細 |
| POST | `/api/v1/charge-orders/:id/refund` | 要サインイン | 払い戻し |
| GET | `/api/v1/charge-orders/export-csv` | 要サインイン | リチャージ注文のエクスポート CSV |

### カードをリチャージする

カードの一括生成、使用記録、CSV のエクスポート。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| GET | `/api/v1/charge-cards` | 要サインイン | カードリスト |
| POST | `/api/v1/charge-cards` | 要サインイン | カードの作成 |
| DELETE | `/api/v1/charge-cards/:id` | 要サインイン | カードの削除 |
| GET | `/api/v1/charge-cards/:id/usages` | 要サインイン | カード利用記録 |
| POST | `/api/v1/charge-cards/batch` | 要サインイン | カードを一括生成する |
| GET | `/api/v1/charge-cards/export-csv` | 要サインイン | エクスポートカード CSV |

### チャージコールバックイベント

コールバック イベントの監査。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| GET | `/api/v1/charge-callback-events` | 要サインイン | コールバックイベントリスト |

### チャージコールバックの再試行

失敗したコールバックを手動で再試行します。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| GET | `/api/v1/charge-callback-retries` | 要サインイン | 失敗したコールバックのリスト |
| POST | `/api/v1/charge-callback-retries/:id/retry` | 管理者 | コールバックを再試行する |

### 顧客残高

残高と取引の管理者ビュー。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| GET | `/api/v1/client-balances` | 要サインイン | お客様残高一覧表 |
| GET | `/api/v1/client-balances/transactions` | 要サインイン | 残高取引 (管理者) |
| GET | `/api/v1/client-balances/transactions/export-csv` | 要サインイン | 輸出残高取引 CSV |

### ギフト

受け取り可能なギフト (日数/トラフィック) を発行します。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| GET | `/api/v1/client-gifts` | 要サインイン | ギフトリスト |
| POST | `/api/v1/client-gifts` | 要サインイン | ギフトを作成する |
| POST | `/api/v1/client-gifts/batch` | 要サインイン | ギフトを一括作成 |
| GET | `/api/v1/client-gifts/search-clients` | 要サインイン | 顧客を検索する |

### 紹介構成

紹介手数料の構成と統計。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| GET | `/api/v1/referral/config` | 要サインイン | 紹介構成 |
| PUT | `/api/v1/referral/config` | 管理者 | 紹介構成を保存する |
| GET | `/api/v1/referral/records` | 要サインイン | 紹介実績一覧 |
| GET | `/api/v1/referral/stats` | 要サインイン | 紹介統計 |

### 紹介の引き出し

出金リクエストを確認します。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| GET | `/api/v1/referral-withdrawals` | 要サインイン | 出金依頼リスト |
| POST | `/api/v1/referral-withdrawals/:id/approve` | 要サインイン | 出金の承認 |
| POST | `/api/v1/referral-withdrawals/:id/reject` | 要サインイン | 出金を拒否する |

### IP許可リスト(概要)

すべての顧客にわたる IP 許可リストの表示。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| GET | `/api/v1/ip-whitelists` | 要サインイン | すべての顧客の IP 許可リスト |

### 顧客の OAuth バインディング

顧客のサードパーティアカウントバインディングをクエリします。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| GET | `/api/v1/client-oauth-bindings` | 要サインイン | 顧客の OAuth バインディング リスト |

### チケット（管理者）

チケットの返信、割り当て、および終了。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| GET | `/api/v1/tickets` | 要サインイン | チケット一覧 |
| GET | `/api/v1/tickets/:id` | 要サインイン | チケット詳細 |
| POST | `/api/v1/tickets/:id/assign` | 要サインイン | チケットの割り当て |
| POST | `/api/v1/tickets/:id/close` | 要サインイン | チケットを閉じる |
| POST | `/api/v1/tickets/:id/reply` | 要サインイン | チケットへの返信 |

### 記事

ポータル記事の CRUD。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| GET | `/api/v1/articles` | 要サインイン | 記事一覧 |
| POST | `/api/v1/articles` | 要サインイン | 記事を作成する |
| DELETE | `/api/v1/articles/:id` | 要サインイン | 記事の削除 |
| GET | `/api/v1/articles/:id` | 要サインイン | 記事詳細 |
| PUT | `/api/v1/articles/:id` | 要サインイン | 記事を更新 |

### 共有アカウント

共有アカウント（ストリーミング等）の発行・修理記録。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| GET | `/api/v1/shared-accounts` | 要サインイン | 共有アカウントリスト |
| POST | `/api/v1/shared-accounts` | 要サインイン | 共有アカウントを作成する |
| DELETE | `/api/v1/shared-accounts/:id` | 要サインイン | 共有アカウントを削除する |
| GET | `/api/v1/shared-accounts/:id` | 要サインイン | 共有アカウントの詳細 |
| PUT | `/api/v1/shared-accounts/:id` | 要サインイン | 共有アカウントを更新する |
| GET | `/api/v1/shared-accounts/:id/repair-logs` | 要サインイン | 修理記録 |

### 証明書

公開証明書 (ACME) の発行、アップロード、更新、エクスポート。特定のノードでの発行がサポートされています。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| GET | `/api/v1/certificates` | 要サインイン | リスト |
| POST | `/api/v1/certificates` | 要サインイン | 作成 |
| DELETE | `/api/v1/certificates/:id` | 要サインイン | 削除 |
| GET | `/api/v1/certificates/:id` | 要サインイン | 詳細 |
| PUT | `/api/v1/certificates/:id` | 要サインイン | アップデート |
| GET | `/api/v1/certificates/:id/download` | 要サインイン | ダウンロード |
| POST | `/api/v1/certificates/:id/renew` | 要サインイン | 顧客を更新する |
| GET | `/api/v1/certificates/check-port` | 要サインイン | ポートを確認してください |
| GET | `/api/v1/certificates/detect-ip` | 要サインイン | プローブIP |
| GET | `/api/v1/certificates/export` | 要サインイン | エクスポート |
| POST | `/api/v1/certificates/issue` | 要サインイン | 問題。本体：`{"domains":["example.com","*.example.com"],"challenge_type":"dns-01","dns_channel_id":2,"key_type":"rsa","key_bits":4096}`。 `challenge_type` のデフォルトは http-01 です。 `dns_channel_id` は、DNS チャネル (構成された DNS プロバイダー + 資格情報) を選択します。 |
| POST | `/api/v1/certificates/issue-on-node` | 要サインイン | ノードに発行する |
| POST | `/api/v1/certificates/upload` | 要サインイン | アップロード |

### 証明書チャネル

ACME チャネル (CA) 構成。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| GET | `/api/v1/cert-channels` | 要サインイン | リスト |
| POST | `/api/v1/cert-channels` | 要サインイン | 作成 |
| DELETE | `/api/v1/cert-channels/:id` | 要サインイン | 削除 |
| PUT | `/api/v1/cert-channels/:id` | 要サインイン | アップデート |
| GET | `/api/v1/cert-channels/providers` | 要サインイン | プロバイダー |

### EAB構成

ZeroSSL / Google などの外部アカウントをバインドする認証情報。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| GET | `/api/v1/eab-configs` | 要サインイン | リスト |
| POST | `/api/v1/eab-configs` | 要サインイン | 作成 |
| DELETE | `/api/v1/eab-configs/:id` | 要サインイン | 削除 |
| PUT | `/api/v1/eab-configs/:id` | 要サインイン | アップデート |
| POST | `/api/v1/eab-configs/fetch-google` | 要サインイン | GCP から Google EAB を取得する |
| POST | `/api/v1/eab-configs/fetch-zerossl` | 要サインイン | ワンクリックでメールで ZeroSSL EAB を取得します |

### ACME アカウント

パネル側 ACME アカウント (連絡先メールアドレス) の管理。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| GET | `/api/v1/acme-accounts` | 要サインイン | リスト |
| POST | `/api/v1/acme-accounts` | 要サインイン | 作成 |
| DELETE | `/api/v1/acme-accounts/:id` | 要サインイン | 削除 |
| PUT | `/api/v1/acme-accounts/:id` | 要サインイン | アップデート |
| GET | `/api/v1/acme-accounts/default` | 要サインイン | デフォルトを取得する |

### 内蔵 CA

内部パネル CA: エージェント証明書の発行/更新/失効、CRL エクスポート、配布キュー。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| GET | `/api/v1/ca/agent-certs` | 要サインイン | エージェント証明書リスト |
| POST | `/api/v1/ca/agent-certs/:server_id/renew` | 要サインイン | エージェント証明書を更新する |
| GET | `/api/v1/ca/distribution-queue` | 要サインイン | 証明書配布キュー |
| DELETE | `/api/v1/ca/distribution-queue/:id` | 要サインイン | 配布キューエントリの削除 |
| POST | `/api/v1/ca/distribution-queue/:id/retry` | 要サインイン | 証明書の配布を再試行します |
| GET | `/api/v1/ca/revocations` | 要サインイン | 失効リスト |
| GET | `/api/v1/ca/revocations/export` | 要サインイン | CRL のエクスポート |
| GET | `/api/v1/ca/status` | 要サインイン | CA ステータス |

### 地理的ルール

geoip/geosite データ ソースとファイルの同期と配信。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| GET | `/api/v1/geo-rules/files` | 要サインイン | ファイルリスト |
| DELETE | `/api/v1/geo-rules/files/:fileId` | 要サインイン | ファイルの削除 |
| PUT | `/api/v1/geo-rules/files/:fileId/auto-sync` | 要サインイン | 自動同期の切り替え |
| POST | `/api/v1/geo-rules/files/:fileId/download` | 要サインイン | ダウンロード |
| GET | `/api/v1/geo-rules/files/:fileId/download-file` | 要サインイン | ファイルをダウンロードする |
| GET | `/api/v1/geo-rules/github-assets` | 要サインイン | GitHub リポジトリ アセットを取得する |
| POST | `/api/v1/geo-rules/push-all` | 要サインイン | すべてのノードにプッシュする |
| POST | `/api/v1/geo-rules/seed` | 要サインイン | デフォルトデータの初期化 |
| GET | `/api/v1/geo-rules/sources` | 要サインイン | データソースリスト |
| POST | `/api/v1/geo-rules/sources` | 要サインイン | データソースの作成 |
| DELETE | `/api/v1/geo-rules/sources/:sourceId` | 要サインイン | データソースの削除 |
| PUT | `/api/v1/geo-rules/sources/:sourceId` | 要サインイン | データソースを更新する |
| GET | `/api/v1/geo-rules/sources/:sourceId/assets` | 要サインイン | アセットのフェッチ・リリース |
| POST | `/api/v1/geo-rules/sources/:sourceId/files` | 要サインイン | ファイルを追加 |
| PUT | `/api/v1/geo-rules/sources/:sourceId/schedule` | 要サインイン | スケジュールされた同期を切り替えます |
| POST | `/api/v1/geo-rules/sources/:sourceId/select` | 要サインイン | ファイルを選択 |
| POST | `/api/v1/geo-rules/sources/:sourceId/sync` | 要サインイン | このデータソースを同期します |
| GET | `/api/v1/geo-rules/status` | 要サインイン | ステータス |
| POST | `/api/v1/geo-rules/sync-all` | 要サインイン | すべてを同期する |
| GET | `/api/v1/geo-rules/sync-status/:key` | 要サインイン | 同期の進行状況 |

### WAF ルール ライブラリ

グローバル WAF ルール: GitHub ソース、同期、すべてのノードへの配信。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| GET | `/api/v1/waf/github-source` | 要サインイン | GitHub データソース |
| PUT | `/api/v1/waf/github-source` | 要サインイン | GitHub データ ソースの保存 |
| POST | `/api/v1/waf/push-all` | 要サインイン | すべてのノードにプッシュする |
| GET | `/api/v1/waf/rules` | 要サインイン | ルール一覧 |
| DELETE | `/api/v1/waf/rules/:name` | 要サインイン | ルールの削除 |
| GET | `/api/v1/waf/rules/:name` | 要サインイン | ルールの取得 |
| PUT | `/api/v1/waf/rules/:name` | 要サインイン | ルールの保存 |
| POST | `/api/v1/waf/sync-all` | 要サインイン | すべてを同期する |
| POST | `/api/v1/waf/sync-github` | 要サインイン | GitHubから同期 |

### ログ管理

ノード/コアのログの読み取りとストリーミング、クリーンアップ ポリシー、ログ ライターの制御。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| POST | `/api/v1/log-manage/cleanup-all` | 要サインイン | 今すぐすべてをクリーンアップしてください |
| GET | `/api/v1/log-manage/cleanup-rules` | 要サインイン | クリーンアップ ルールのリスト |
| PUT | `/api/v1/log-manage/cleanup-rules/:id` | 要サインイン | クリーンアップ ルールを更新する |
| POST | `/api/v1/log-manage/cleanup-table/:table` | 要サインイン | 指定されたテーブルを今すぐクリーンアップします |
| GET | `/api/v1/log-manage/core-log-output` | 要サインイン | コアログ出力構成 |
| PUT | `/api/v1/log-manage/core-log-output` | 要サインイン | コアログ出力構成を保存する |
| GET | `/api/v1/log-manage/core/:type/stdout-log` | 要サインイン | コア標準出力ログを読み取る |
| GET | `/api/v1/log-manage/core/:type/stdout-stream` | 要サインイン | ストリームコア標準出力 |
| POST | `/api/v1/log-manage/discover` | 要サインイン | ノードログファイルの検出 |
| GET | `/api/v1/log-manage/log-output` | 要サインイン | ログ出力設定 |
| PUT | `/api/v1/log-manage/log-output` | 要サインイン | ログ出力設定を保存する |
| POST | `/api/v1/log-manage/log-output/release/:type` | 要サインイン | リリースログの標準出力引き継ぎ |
| POST | `/api/v1/log-manage/log-output/takeover/:type` | 要サインイン | ログの標準出力を引き継ぐ |
| GET | `/api/v1/log-manage/nodes/:serverId/logs` | 要サインイン | ノードログファイルリスト |
| GET | `/api/v1/log-manage/nodes/:serverId/logs/:type` | 要サインイン | ノードログの読み取り |
| POST | `/api/v1/log-manage/nodes/:serverId/logs/:type/rotate` | 要サインイン | ノードログをローテーションする |
| GET | `/api/v1/log-manage/nodes/:serverId/logs/:type/stream` | 要サインイン | ストリームノードログ |
| GET | `/api/v1/log-manage/policies` | 要サインイン | ポリシー一覧 |
| POST | `/api/v1/log-manage/policies` | 要サインイン | ポリシーの作成 |
| DELETE | `/api/v1/log-manage/policies/:id` | 要サインイン | ポリシーの削除 |
| GET | `/api/v1/log-manage/policies/:id` | 要サインイン | ポリシーの詳細 |
| PUT | `/api/v1/log-manage/policies/:id` | 要サインイン | ポリシーの更新 |
| GET | `/api/v1/log-manage/policies/:id/jobs` | 要サインイン | クリーンアップジョブリスト |
| GET | `/api/v1/log-manage/policies/:id/log` | 要サインイン | ログの読み取り |
| GET | `/api/v1/log-manage/policies/:id/log-stream` | 要サインイン | ストリームログ |
| POST | `/api/v1/log-manage/policies/:id/rotate` | 要サインイン | 回転 |
| GET | `/api/v1/log-manage/proxy/:type/stdout-log` | 要サインイン | コア標準出力ログを読み取る |
| GET | `/api/v1/log-manage/proxy/:type/stdout-stream` | 要サインイン | ストリームコア標準出力 |
| GET | `/api/v1/log-manage/scheduler-config` | 要サインイン | スケジュール設定 |
| PUT | `/api/v1/log-manage/scheduler-config` | 要サインイン | スケジュール設定を更新する |
| GET | `/api/v1/log-manage/status` | 要サインイン | ステータス |
| POST | `/api/v1/log-manage/sync` | 要サインイン | 同期クリーンアップポリシー |
| GET | `/api/v1/log-manage/writers` | 要サインイン | ライターステータス |
| PUT | `/api/v1/log-manage/writers/:type/config` | 要サインイン | ライター構成を更新する |
| GET | `/api/v1/log-manage/writers/:type/files` | 要サインイン | ライターログファイルリスト |
| POST | `/api/v1/log-manage/writers/:type/start` | 要サインイン | ライターを開始する |
| POST | `/api/v1/log-manage/writers/:type/stop` | 要サインイン | ライターを停止する |

### バックアップ

データベース/完全バックアップ、復元、スケジュールされたジョブ、FTP/S3/WebDAV リモート ストレージ。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| DELETE | `/api/v1/backups/:id` | 管理者 | バックアップの削除 |
| GET | `/api/v1/backups/:id/download` | 管理者 | バックアップをダウンロードする |
| POST | `/api/v1/backups/:id/restore` | 管理者 | バックアップから復元する |
| POST | `/api/v1/backups/:id/upload-ftp` | 管理者 | FTPにアップロード |
| POST | `/api/v1/backups/:id/upload-s3` | 管理者 | S3にアップロード |
| POST | `/api/v1/backups/:id/upload-webdav` | 管理者 | WebDAVにアップロード |
| POST | `/api/v1/backups/create` | 管理者 | 完全バックアップの作成 (DB ダンプ付き) |
| GET | `/api/v1/backups/list` | 管理者 | 完全なバックアップリスト |
| POST | `/api/v1/backups/remote-restore` | 管理者 | リモートから復元する |
| GET | `/api/v1/backups/restore-status` | 管理者 | 復元の進行状況 |
| GET | `/api/v1/backups/schedules` | 管理者 | スケジュールされたジョブのリスト |
| POST | `/api/v1/backups/schedules` | 管理者 | スケジュールされたジョブを作成する |
| DELETE | `/api/v1/backups/schedules/:id` | 管理者 | スケジュールされたジョブを削除する |
| PUT | `/api/v1/backups/schedules/:id` | 管理者 | スケジュールされたジョブを更新する |
| GET | `/api/v1/backups/storages` | 管理者 | ストレージ構成リスト |
| POST | `/api/v1/backups/storages` | 管理者 | ストレージ構成の作成 |
| DELETE | `/api/v1/backups/storages/:id` | 管理者 | ストレージ構成の削除 |
| PUT | `/api/v1/backups/storages/:id` | 管理者 | ストレージ構成を更新する |
| GET | `/api/v1/backups/storages/:id/files` | 管理者 | リモートバックアップファイルリスト |
| POST | `/api/v1/backups/upload-restore` | 管理者 | バックアップと復元をアップロードする |

### システム設定

データベースの構成と移行、パネル設定、JWT キー、SMTP/Telegram テスト、通知テンプレート、オンライン アップグレード (主に管理者)。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| PUT | `/api/v1/system/bootstrap-finish` | 管理者 | 初回起動セットアップを完了する |
| GET | `/api/v1/system/bootstrap-state` | 管理者 | 初回起動ステータス |
| GET | `/api/v1/system/config-file/:coreType` | 管理者 | コア構成ファイルを読み取る |
| GET | `/api/v1/system/db-config` | 管理者 | データベース構成 |
| PUT | `/api/v1/system/db-config` | 管理者 | データベース構成を更新する |
| POST | `/api/v1/system/db-config/migrate` | 管理者 | データベースデータを移行する |
| POST | `/api/v1/system/db-config/switch` | 管理者 | データベースの切り替え (ワンステップ移行あり) |
| POST | `/api/v1/system/db-config/test` | 管理者 | データベース接続のテスト |
| GET | `/api/v1/system/db-pool-stats` | 管理者 | 接続プールの統計 |
| GET | `/api/v1/system/db-stats` | 管理者 | データベース統計 |
| GET | `/api/v1/system/db-stats/ping` | 管理者 | データベースプローブ |
| POST | `/api/v1/system/db-stats/reset` | 管理者 | データベース統計をリセットする |
| GET | `/api/v1/system/encryption-key-status` | 管理者 | 暗号化キーのステータス |
| GET | `/api/v1/system/env-config` | 管理者 | .env構成 |
| PUT | `/api/v1/system/env-config` | 管理者 | .env 構成を更新する |
| POST | `/api/v1/system/generate-encryption-key` | 管理者 | 暗号化キーを生成する |
| GET | `/api/v1/system/github-config` | 管理者 | GitHub構成 |
| PUT | `/api/v1/system/github-config` | 管理者 | GitHub 構成を更新する |
| GET | `/api/v1/system/jwt-secret-status` | 管理者 | JWT キーのステータス |
| GET | `/api/v1/system/maintenance` | 公開 | メンテナンスモードのステータス（公開） |
| PUT | `/api/v1/system/maintenance` | 管理者 | メンテナンスモードの切り替え |
| POST | `/api/v1/system/migrate-data` | 管理者 | データの移行 |
| GET | `/api/v1/system/notification-config` | 管理者 | 通知設定 |
| PUT | `/api/v1/system/notification-config` | 管理者 | 通知構成を更新する |
| GET | `/api/v1/system/notification-templates` | 管理者 | テンプレート一覧 |
| POST | `/api/v1/system/notification-templates` | 管理者 | テンプレートの作成 |
| DELETE | `/api/v1/system/notification-templates/:id` | 管理者 | テンプレートの削除 |
| PUT | `/api/v1/system/notification-templates/:id` | 管理者 | テンプレートを更新する |
| POST | `/api/v1/system/notification-templates/:id/broadcast` | 管理者 | テンプレートによるブロードキャスト通知 |
| POST | `/api/v1/system/notification-templates/:id/send` | 管理者 | テスト通知を送信する |
| GET | `/api/v1/system/panel-config` | 管理者 | パネル構成 |
| PUT | `/api/v1/system/panel-config` | 管理者 | パネル構成を更新する |
| GET | `/api/v1/system/reality-targets` | 管理者 | REALITY ターゲットリスト |
| PUT | `/api/v1/system/reality-targets` | 管理者 | REALITY ターゲットを更新 |
| POST | `/api/v1/system/reality-targets/reset` | 管理者 | REALITY ターゲットをデフォルトにリセット |
| POST | `/api/v1/system/regenerate-jwt-secret` | 管理者 | JWTキーを再生成します |
| GET | `/api/v1/system/releases` | 管理者 | 利用可能なバージョン |
| POST | `/api/v1/system/restart` | 管理者 | 再起動 |
| POST | `/api/v1/system/rollback` | 管理者 | システムのロールバック |
| GET | `/api/v1/system/settings` | 管理者 | 設定の読み取り |
| PUT | `/api/v1/system/settings/batch` | 管理者 | 一括更新設定 |
| GET | `/api/v1/system/stats` | 管理者 | システムステータス |
| GET | `/api/v1/system/subscription-billing-templates` | 管理者 | サブスクリプションの請求テンプレート |
| PUT | `/api/v1/system/subscription-billing-templates` | 管理者 | サブスクリプションの請求テンプレートを更新する |
| POST | `/api/v1/system/telegram-chat-id` | 管理者 | Telegram チャット ID を取得する |
| POST | `/api/v1/system/test-smtp` | 管理者 | SMTPをテストする |
| POST | `/api/v1/system/test-telegram` | 管理者 | Telegramをテストする |
| POST | `/api/v1/system/update` | 管理者 | システムアップグレード |
| GET | `/api/v1/system/update-jobs` | 管理者 | アップグレードジョブリスト |
| POST | `/api/v1/system/upload-mtls-ca` | 管理者 | mTLS CA 証明書をアップロードする |

### セキュリティ監査

イベントクエリを監査してエクスポートします。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| GET | `/api/v1/security/audit-events` | 要サインイン | 監査イベントのリスト |
| GET | `/api/v1/security/audit-events/export` | 要サインイン | 監査イベントのエクスポート |

### メトリクス

パネルの集約メトリクス (ノード/証明書/サービス)。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| GET | `/api/v1/metrics/certificates` | 要サインイン | 証明書のメトリクス |
| GET | `/api/v1/metrics/nodes` | 要サインイン | ノードメトリクス |
| GET | `/api/v1/metrics/overview` | 要サインイン | 概要 |

### ポータルテンプレート

ポータル テンプレート、アクティベーション構成、およびキャッシュ管理。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| GET | `/api/v1/portal/active` | 要サインイン | アクティブなテンプレート |
| PUT | `/api/v1/portal/active` | 要サインイン | アクティブなテンプレートを更新する |
| DELETE | `/api/v1/portal/cache` | 要サインイン | ポータルのキャッシュをクリアする |
| GET | `/api/v1/portal/cache` | 要サインイン | キャッシュステータス |
| PUT | `/api/v1/portal/cache/dev-mode` | 要サインイン | キャッシュ開発モードの切り替え |
| POST | `/api/v1/portal/cache/rebuild` | 要サインイン | ポータルキャッシュを再構築する |
| POST | `/api/v1/portal/reset` | 要サインイン | デフォルトのテンプレートを復元する |
| GET | `/api/v1/portal/templates` | 要サインイン | リスト |
| POST | `/api/v1/portal/templates` | 要サインイン | 作成 |
| DELETE | `/api/v1/portal/templates/:id` | 要サインイン | 削除 |
| GET | `/api/v1/portal/templates/:id` | 要サインイン | 詳細 |
| PUT | `/api/v1/portal/templates/:id` | 要サインイン | アップデート |
| POST | `/api/v1/portal/templates/:id/activate` | 要サインイン | テンプレートをアクティブ化する |

> `/portal/templates` CRUD は **高度な API** です。ポータル UI は主に `PUT /api/v1/portal/active` を使用します
> (ワンショットのフルサイト構成);このグループで複数のテンプレートを管理/切り替えます。どちらもサポートされており、競合しません。

### エージェントのインストール

ノード エージェントのインストール スクリプトとダウンロード (パブリック)。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| POST | `/api/v1/agent/bootstrap` | 公開 | エージェントの初回起動登録 (ブートストラップ トークンを使用したエージェント レポート、パブリック) |
| GET | `/api/v1/agent/releases` | 管理者 | 利用可能なエージェントのバージョン (アップグレード ページの場合) |
| GET | `/api/v1/agent/version` | 管理者 | パネル上の現在のエージェントのバージョン |
| GET | `/api/v1/agent/download` | 公開 | ダウンロードエージェント |
| GET | `/api/v1/agent/install` | 公開 | インストールスクリプトを取得する |

### チャージコールバックエントリ

サードパーティの支払いプラットフォームのコールバック (パブリック、署名が検証済み)。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| POST | `/api/v1/charge/callback/:site_id` | 公開 | チャージコールバックを処理する |

### インバウンド (ノードあり)

プランノードの選択に使用されるインバウンドリスト。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| GET | `/api/v1/inbounds-with-server` | 要サインイン | すべてのインバウンド (ノードを含む) |

### パブリック構成

サインインなしのポータル ランタイム構成 (メンテナンス ガード + パブリック レート制限)。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| GET | `/api/v1/public/articles` | 公開 | 公開記事一覧 |
| GET | `/api/v1/public/articles/:slug` | 公開 | 公開記事詳細 |
| GET | `/api/v1/public/client-configs` | 公開 | クライアントダウンロード構成 (パブリック) |
| GET | `/api/v1/public/portal-config` | 公開 | ポータル ランタイム構成 (サイト名、テーマ、言語、コンプライアンス ドキュメント、パブリック) |
| GET | `/api/v1/public/settings` | 公開 | パブリック設定 (例: register_enabled; public) |
| GET | `/api/v1/public/subscription-endpoints` | 公開 | 有効なサブスクリプション エンドポイント リスト (パブリック) |

### クライアント認証とビジネス (管理エンジンで同じ設定)

クライアント エンドポイントはポータルと同一です。詳細については、`portal-api.en.md`を参照してください。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| GET | `/api/v1/client-auth/articles` | 公開 | 記事一覧 |
| GET | `/api/v1/client-auth/articles/:slug` | 公開 | 記事を取得 |
| GET | `/api/v1/client-auth/balance` | 公開 | バランスを取る |
| GET | `/api/v1/client-auth/balance/transactions` | 公開 | 残高取引 |
| GET | `/api/v1/client-auth/bills` | 公開 | 注文リスト |
| POST | `/api/v1/client-auth/change-password` | 公開 | パスワードを変更する |
| POST | `/api/v1/client-auth/charge/create` | 公開 | リチャージ注文を作成する |
| GET | `/api/v1/client-auth/charge/orders` | 公開 | リチャージ注文をリストする |
| GET | `/api/v1/client-auth/charge/orders/:id` | 公開 | リチャージ注文の詳細を取得する |
| POST | `/api/v1/client-auth/charge/redeem-card` | 公開 | カードを引き換える |
| GET | `/api/v1/client-auth/charge/sites` | 公開 | リチャージサイトをリストアップする |
| GET | `/api/v1/client-auth/charge/status/:trade_no` | 公開 | 充電ステータスのクエリ |
| GET | `/api/v1/client-auth/checktime` | 公開 | サーバー時刻の取得 (クロック同期) |
| GET | `/api/v1/client-auth/connection-info` | 公開 | プランの接続パラメータを取得する |
| GET | `/api/v1/client-auth/gifts` | 公開 | ギフトをリストアップする |
| POST | `/api/v1/client-auth/gifts/:id/claim` | 公開 | ギフトを受け取る |
| GET | `/api/v1/client-auth/invoices` | 公開 | 注文リスト |
| GET | `/api/v1/client-auth/invoices/:id` | 公開 | 注文の詳細 |
| GET | `/api/v1/client-auth/ip-whitelist` | 公開 | IP許可リストをリストする |
| DELETE | `/api/v1/client-auth/ip-whitelist/:id` | 公開 | IP許可リストから削除 |
| POST | `/api/v1/client-auth/ip-whitelist/auto-add` | 公開 | IPの自動追加 |
| POST | `/api/v1/client-auth/login` | 公開 | サインイン |
| POST | `/api/v1/client-auth/logout` | 公開 | サインアウト |
| POST | `/api/v1/client-auth/logs/package` | 公開 | ログバンドルの作成 |
| GET | `/api/v1/client-auth/logs/packages` | 公開 | ログバンドルをリストする |
| DELETE | `/api/v1/client-auth/logs/packages/:id` | 公開 | ログバンドルを削除する |
| GET | `/api/v1/client-auth/logs/packages/:id` | 公開 | ログバンドルを取得する |
| GET | `/api/v1/client-auth/logs/packages/:id/download` | 公開 | ログバンドルをダウンロードする |
| GET | `/api/v1/client-auth/logs/types` | 公開 | ログバンドルのヘルスチェック |
| GET | `/api/v1/client-auth/me` | 公開 | 現在サインインしているユーザーを取得する |
| GET | `/api/v1/client-auth/nodes` | 公開 | 私のノードリスト |
| GET | `/api/v1/client-auth/nodes/:id/config` | 公開 | ノード構成 |
| GET | `/api/v1/client-auth/nodes/:id/link` | 公開 | ノード共有リンク |
| GET | `/api/v1/client-auth/nodes/:id/qrcode` | 公開 | ノードQRコード |
| GET | `/api/v1/client-auth/oauth/:provider` | 公開 | 承認する |
| GET | `/api/v1/client-auth/oauth/:provider/callback` | 公開 | コールバック |
| GET | `/api/v1/client-auth/oauth/providers` | 公開 | プロバイダーのステータス |
| GET | `/api/v1/client-auth/orders` | 公開 | 注文リスト |
| POST | `/api/v1/client-auth/orders` | 公開 | 注文の作成 |
| GET | `/api/v1/client-auth/orders/:id` | 公開 | 注文の詳細 |
| GET | `/api/v1/client-auth/plans` | 公開 | プラン一覧 |
| GET | `/api/v1/client-auth/plans/:id` | 公開 | プラン詳細 |
| GET | `/api/v1/client-auth/profile` | 公開 | プロフィールを取得 |
| PUT | `/api/v1/client-auth/profile` | 公開 | プロフィールを更新する |
| POST | `/api/v1/client-auth/profile/bind-oauth` | 公開 | サードパーティアカウントをバインドする |
| DELETE | `/api/v1/client-auth/profile/unbind-oauth/:provider` | 公開 | サードパーティアカウントのバインドを解除する |
| GET | `/api/v1/client-auth/referral/code` | 公開 | 紹介確認コードを取得する |
| GET | `/api/v1/client-auth/referral/records` | 公開 | 紹介記録の一覧表示 |
| GET | `/api/v1/client-auth/referral/stats` | 公開 | 紹介統計を取得する |
| POST | `/api/v1/client-auth/referral/withdraw` | 公開 | 撤回リクエスト |
| GET | `/api/v1/client-auth/referral/withdrawals` | 公開 | 紹介の引き出しをリストする |
| POST | `/api/v1/client-auth/refresh` | 公開 | リフレッシュ |
| POST | `/api/v1/client-auth/register` | 公開 | 登録する |
| POST | `/api/v1/client-auth/reset-subscription` | 公開 | サブスクリプションをリセットする |
| POST | `/api/v1/client-auth/rotate-token` | 公開 | サブスクリプション トークンをローテーションする |
| POST | `/api/v1/client-auth/self-unban` | 公開 | セルフサービスの禁止解除 |
| PUT | `/api/v1/client-auth/settings` | 公開 | 設定を更新する |
| POST | `/api/v1/client-auth/settings/auto-add-ip` | 公開 | IPの自動追加 |
| GET | `/api/v1/client-auth/settings/whitelist` | 公開 | IP許可リストをリストする |
| DELETE | `/api/v1/client-auth/settings/whitelist/:id` | 公開 | IP許可リストから削除 |
| GET | `/api/v1/client-auth/shared-accounts` | 公開 | 共有アカウントをリストする |
| GET | `/api/v1/client-auth/shared-accounts/:id` | 公開 | 共有アカウントを取得する |
| POST | `/api/v1/client-auth/shared-accounts/:id/apply` | 公開 | 共有アカウントをリクエストする |
| POST | `/api/v1/client-auth/shared-accounts/:id/repair` | 公開 | 修理依頼 |
| POST | `/api/v1/client-auth/ticket` | 公開 | 1 回限りの有効期限の短いチケットを発行する |
| GET | `/api/v1/client-auth/tickets` | 公開 | チケットをリストする |
| POST | `/api/v1/client-auth/tickets` | 公開 | チケットの作成 |
| GET | `/api/v1/client-auth/tickets/:id` | 公開 | チケットを取得する |
| POST | `/api/v1/client-auth/tickets/:id/close` | 公開 | チケットを閉じる |
| POST | `/api/v1/client-auth/tickets/:id/reply` | 公開 | チケットへの返信 |
| GET | `/api/v1/client-auth/traffic-history` | 公開 | 交通履歴 |
| GET | `/api/v1/client-auth/traffic-packs` | 公開 | トラフィックパックリスト |
| POST | `/api/v1/client-auth/traffic-packs/buy` | 公開 | トラフィックパックを購入する |

### 定期購入の配信

クライアントがサブスクリプションを取得するためのパブリック エントリ ポイント。 `subscription.en.md`を参照してください。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| GET | `/sub/:token` | 公開 | サブスクリプションを提供する |

### ランタイム構成ワークフロー

構成スナップショット / ドライラン / 適用 / ロールバック (`apply`、`rollback`、および `disable-proxy` には管理者が必要)。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| POST | `/runtime/apply` | 公開 | 申し込む |
| GET | `/runtime/apply-jobs` | 要サインイン | ジョブのリストを表示する |
| GET | `/runtime/apply-jobs/:id` | 要サインイン | 仕事を得る |
| POST | `/runtime/apply-jobs/:id/cancel` | 公開 | ジョブをキャンセルする |
| POST | `/runtime/apply-jobs/:id/retry` | 公開 | ジョブを再試行します |
| POST | `/runtime/disable-proxy` | 公開 | プロキシを無効にする |
| POST | `/runtime/dry-run` | 要サインイン | ドライラン実行 |
| GET | `/runtime/nodes` | 要サインイン | リストノード |
| GET | `/runtime/overview` | 要サインイン | 概要 |
| POST | `/runtime/rollback` | 公開 | ロールバック |
| GET | `/runtime/snapshots` | 要サインイン | スナップショットの一覧表示 |
| POST | `/runtime/snapshots` | 要サインイン | スナップショットの作成 |
| DELETE | `/runtime/snapshots/:id` | 公開 | スナップショットの削除 |
| GET | `/runtime/snapshots/:id` | 要サインイン | スナップショットの取得 |
| POST | `/runtime/verify` | 要サインイン | 検証する |

### サービス アクセス ポリシー (廃止)

> `/services/:id/access-policies` と `/services/:id/ip-bans` は 2026 年 10 月 20 日に廃止されました。
> 管理者呼び出しや接続パスがこれらのポリシーを消費することはありません。

### トラフィッククォータの監視

クォータと使用量の読み取り専用監視ビュー。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| GET | `/traffic/quotas` | 要サインイン | クォータリスト |
| GET | `/traffic/quotas/:id/usage` | 要サインイン | クォータの使用量 |
| GET | `/traffic/reset-logs` | 要サインイン | クォータリセットレコード |
| GET | `/traffic/usage-history` | 要サインイン | 利用履歴 |

### 健康診断

活性プローブ。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| GET | `/health` | 公開 | 活性プローブ。 200は健康を意味します |

### ポータルページ

ポータル SPA ページ。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| GET | `/p` | 公開 | ポータル HTML (サーバーレンダリング + Vue ランタイム) |

### ACME の課題

HTTP-01 チャレンジ コールバック。

| 方法 | パス | 許可 | 説明 |
|---|---|---|---|
| GET | `/.well-known/acme-challenge/:token` | 公開 | ACME HTTP-01 チャレンジ コールバック (発行時にドメインを検証) |
