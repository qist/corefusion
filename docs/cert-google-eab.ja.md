<!-- source: cert-google-eab.md | locale: ja | updated: 2026-10-23 -->
# Google パブリック CA ガイド (EAB 自動発行)

> 言語： [简体中文](cert-google-eab.md) · [English](cert-google-eab.en.md) · [繁體中文](cert-google-eab.zh-TW.md) · [Русский](cert-google-eab.ru.md) · 日本語 · [한국어](cert-google-eab.ko.md) · [Deutsch](cert-google-eab.de.md) · [Français](cert-google-eab.fr.md) · [Español](cert-google-eab.es.md) · [Português](cert-google-eab.pt.md) · [Türkçe](cert-google-eab.tr.md) · [Bahasa Indonesia](cert-google-eab.id.md) · [ไทย](cert-google-eab.th.md) · [Tiếng Việt](cert-google-eab.vi.md) · [العربية](cert-google-eab.ar.md) · [فارسی](cert-google-eab.fa.md)

> 適用対象: パネル v0.12.101+ |ページ: **証明書 → EAB 構成**
> コード: `internal/service/certificate/eab_google.go`、`internal/api/handlers/certificates/eab_config.go`

## 1. Google に EAB が必要な理由

| カリフォルニア州 | EAB は ACME に必須ですか? | EABの入手方法 |
|---|---|---|
| 暗号化しましょう | ❌ いいえ | — |
| ZeroSSL | ✅ はい | **電子メールを 1 つと交換します** (`api.zerossl.com/acme/eab-credentials-email`、acme.sh と同じ)。パネルはそれを自動的に取得できます |
| **Google パブリック CA** | ✅ **はい** | **GCP 認証を使用して作成する必要があります** (メールだけでは十分ではありません) → このページの焦点 |

言い換えれば、Google **「承認」**: サービス アカウント JSON ファイルまたは一時アクセス トークンのいずれかを回避することはできません。

## 2. Google の厳しいルール (最初にお読みください)

| ルール | 説明 |
|---|---|
| **7日以内にご使用ください** | EAB キーは 7 日以内に使用されないと **期限切れ**になるため、再作成する必要があります |
| **1 つのアカウントのみ** | 1 つの EAB キーは 1 つの ACME アカウントを登録します。登録後、キーは **無効化**され、再利用できなくなります |
| **ACME アカウントには有効期限がありません** | これで作成された ACME アカウントは **長期間有効** → **更新は必要ありません EAB** |
| ⚠ **GCP プロジェクトを削除するとすべてのアカウントが削除されます** | プロジェクトを削除すると、その下の **すべての ACME アカウント**が直ちに無効になり、**更新が不可能になります** |

> 結論: **EAB は 1 回限りの「オンボーディング資格情報」です。 ACME アカウントが存続します。**
> パネルは `data/acme/accounts/<ca>.json` にアカウントを保存します — **このディレクトリをバックアップします**。

## 3. 3 つのパス (パス A から開始 - メニュー検索なし)

> Cloud Console のエントリ ポイントは何度か変更されているため、直接リンクと Cloud Shell が最初に表示されます。メニュー パスはフォールバックです。

### パス A (最も簡単、最初に推奨): GCP Cloud Shell の 1 つのコマンド

Cloud Shell は、**gcloud がプリインストールされ、すでにサインインされている** ブラウザ ターミナルです。ローカル設定やメニューはありません。

1. <https://console.cloud.google.com/cloudshell> (またはコンソールの右上にある `>_` アイコン) を開きます。
2. プロジェクトを選択します (プロジェクト ID は上部のプロジェクト ピッカーに表示されます)。
   ```bash
   gcloud config set project <your-project-id>
   ```
3. パブリック CA API を有効にします (API ライブラリで有効にすることと同等**)。
   ```bash
   gcloud services enable publicca.googleapis.com
   ```
4. EAB を作成します (**これらは必要な 2 つの文字列です**)。
   ```bash
   gcloud publicca external-account-keys create
   ```
 出力: `keyId` → パネルの **EAB-KID**; `b64MacKey` → パネルの **EAB-HMAC-Key**
5. パネルに戻ります: **証明書 → EAB 構成 → EAB 構成の追加 → CA: Google パブリック CA** →
 `EAB-KID` / `EAB-HMAC-Key` に 2 つの値を入力 → 保存 ✓

> ⚠ コスト: Google EAB キーは **7 日間、1 回限り** ✗ → 7 日後 (またはアカウントの切り替え時) **ステップ 4 を繰り返します**。
> これを回避するには、パス B (サービス アカウントを一度保存​​し、パネルが自動的に再要求するようにする) を使用します。

### パス B (1 回限りのセットアップ、その後は自動): サービス アカウント キー JSON

**① パブリック CA API** を有効にする (直接リンクが最も簡単です)
<https://console.cloud.google.com/apis/library/publicca.googleapis.com?project=YOUR_PROJECT_ID> → 「有効にする」をクリック
フォールバック: ☰ → **API とサービス → ライブラリ** → `Public CA` を検索 → 開く → 「有効にする」

**② サービスアカウントを作成します**
<https://console.cloud.google.com/iam-admin/serviceaccounts?project=YOUR_PROJECT_ID>
→ 「**+ サービス アカウントの作成**」 → 任意の名前 (例: `corefusion-eab`) → 「作成して続行」 → ロールのステップを**スキップ**できます → 「完了」
フォールバック: ☰ → **IAM と管理者 → サービス アカウント**

**③ 最小限の役割を付与** (同じページで最も簡単)
新しく作成したアカウントをクリックします → **「権限」タブ** → 「アクセスの許可」 → 新しいプリンシパル: **サービス アカウントの電子メール**
→ `publicca` のロールを検索 → **「パブリック CA 外部アカウント キー作成者」** を選択 → 保存
フォールバック: ☰ → **IAM と管理者 → IAM** → 「アクセスの許可」 → 同じメールアドレスとロール
⚠ **所有者/編集者を許可しない** ✗

**④ キー JSON** をダウンロードします。
サービスアカウント → アカウントを開く → **「キー」タブ** → 「**キーを追加**」 → 「**新しいキーを作成**」 → **JSON** → 作成
(ブラウザは `xxx.json` をダウンロードします。または `⋮` 行 → 「キーの管理」 → 同じフロー)

**⑤ パネルに戻ります**: **プロジェクト ID** を入力し、JSON の **コンテンツ全体**を「サービス アカウント キー ファイル」に貼り付けます → 「EAB を自動的にリクエスト」をクリックします
→ その後、パネルは有効期限が切れると **EAB を自動的に再リクエスト**します ✓、GCP へのアクセスはなくなります ✓

### パス C (1 回限りのテストのみ): アクセス トークンを貼り付けます

Cloud Shell の場合:
```bash
gcloud auth print-access-token
```
出力をパネルの「一時アクセストークン」とプロジェクトIDに貼り付け、「EABを自動的にリクエスト」をクリックします。
⚠ 約 1 時間有効で **保存されません** ✓ → 検証のみ、自動再リクエストなし ✗

### エントリポイントが見つからない場合のチートシート (コンソールメニューの変更)

| タスク | 直接リンク (最も信頼性の高い) | フォールバック: メニュー |
|---|---|---|
| パブリック CA API を有効にする | `https://console.cloud.google.com/apis/library/publicca.googleapis.com?project=<PROJECT_ID>` | ☰ → API とサービス → ライブラリ → `Public CA` を検索 |
| サービスアカウント | `https://console.cloud.google.com/iam-admin/serviceaccounts?project=<PROJECT_ID>` | ☰ → IAM と管理 → サービスアカウント |
| 役割の付与 | `https://console.cloud.google.com/iam-admin/iam?project=<PROJECT_ID>` | ☰ → IAM と管理 → IAM → アクセスの許可 |
| クラウドシェル | `https://console.cloud.google.com/cloudshell` | 右上の`>_`アイコン |

> ヒント: ページに直接ジャンプするには、`<PROJECT_ID>` / `YOUR_PROJECT_ID` を実際のプロジェクト ID (`my-project-123456` など) に置き換えます。

## 4. パネルの構成 (2 つの方法)

**証明書 → EAB 構成 → EAB 構成の追加** に移動し、**Google パブリック CA** を選択します。

### オプション A (推奨): サービス アカウント JSON を貼り付けます。

| フィールド | 値 |
|---|---|
| GCP プロジェクト ID | セクション 3 のプロジェクト ID (例: `my-project-123456` |
| サービスアカウント JSON | `corefusion-eab.json` の **コンテンツ全体** を貼り付けます |
| 一時的なアクセストークン | 空のままにしておきます |
| 名前 | `Google Public CA - <project-id>` として自動生成されます。編集可能 |

**[EAB を自動的にリクエスト]** をクリックすると、成功すると KID/HMAC キーが入力されて保存されます。後で発行するときに `CA = Google Public CA` を選択するだけです (EAB は CA ごとに関連付けられており、手動で選択することはできません)。

> サービス アカウント JSON は、**パネル データベースの EAB 構成に暗号化されます** (`json:"-"`、**フロントエンドに返されることはありません**)。
> その目的は、**7 日間の有効期限が切れた後に手動で GCP にアクセスする代わりにキーを自動的に再リクエスト**することです。

### オプション B: 一時的なアクセス トークン (1 回限り)

```bash
gcloud auth print-access-token        # valid ~1 hour
```

それを「一時アクセス トークン」に貼り付け、プロジェクト ID を入力し、**EAB を自動的にリクエスト** をクリックします。
⚠ このトークンは**保存されない**ため、期限切れ後に再取得する必要があります。→ 長期使用ではなく、1回限りのテストには問題ありません。

### 同等のコマンドライン (オプション、比較用)

```bash
gcloud publicca external-account-keys create
# returns keyId / b64MacKey, corresponding to ACME's --eab-kid / --eab-hmac-key
```

## 4.5 なぜ「単一キー」ではなく JSON なのでしょうか?

これは最も一般的な質問です。 2 種類の「キー」を分離します。

| 期待されること | 実際に必要なものは何なのか |
|---|---|
| **API キー** (`AIza…` 文字列) | ❌ **役に立たない** — API キーは単なる公開識別子であり、**識別性を持ちません**。 EAB の作成には **IAM 権限**が必要なので、ID を保証する認証情報が必要です |
| — | ✅ **サービス アカウント キー**: GCP「IAM → サービス アカウント → キー → 新しいキーの作成 → **JSON**」からダウンロードした **ファイル** |

重要なポイント: **JSON ファイル自体は「キー」** — GCP は、アカウント ID (`client_email`) と秘密キー (`private_key`) を 1 つのファイルにパッケージ化します。パネルは、JWT に署名し、OAuth2 アクセス トークンを交換し、パブリック CA API を呼び出します。
したがって、「JSON を貼り付ける」≠ 多くのフィールドを埋めることになります。 **ファイル全体を貼り付けます** (パネルには、`type` / `token_uri` などのオプションのフィールドも入力されます)。

**実際の EAB は 2 つの文字列** (`EAB-KID` + `EAB-HMAC-Key`) であり、これらはリクエストの**結果**です。パネルが自動的にそれらを入力します。

### ファイルを渡したくありませんか? 2 つの選択肢

| 方法 | どうやって | コスト |
|---|---|---|
| **一時的なアクセス トークン** | `gcloud auth print-access-token` → 「一時アクセストークン」に貼り付けます | ~1 時間有効、**保存されません** → 1 回限り、自動再リクエストなし |
| **マニュアル KID/HMAC** | ローカルで`gcloud publicca external-account-keys create`を実行 → 「EAB-KID / EAB-HMAC-Key」に`keyId` / `b64MacKey`を入力 | 認証情報は渡されません ✓;ただし、Google EAB は **7 日間、1 回限り** → 有効期限が切れた後に繰り返します |

> 委員会がサービス アカウント JSON を推奨する理由は 1 つです。**EAB は 7 日後に期限切れになる**、保存されたサービス アカウントのみが手動作業なしで**自動再リクエスト**を許可します。

## 4.6 FAQ: API キーは使用できますか?

**いいえ。** これは実装上の選択ではありません。**Google は明示的に拒否します** (両方の方法で検証済み):

```bash
# 1) Query parameter
curl -X POST "https://publicca.googleapis.com/v1/projects/<ID>/locations/global/externalAccountKeys?key=AIza..." -d '{}'
# 2) x-goog-api-key header recommended by the official docs
curl -X POST "https://publicca.googleapis.com/v1/projects/<ID>/locations/global/externalAccountKeys" \
     -H "x-goog-api-key: AIza..." -d '{}'
```

どちらも次を返します:

```json
{"error":{"code":401,"status":"UNAUTHENTICATED",
  "message":"API keys are not supported by this API. Expected OAuth2 access token
             or other authentication credentials that assert a principal."}}
```

比較のために、**(偽) OAuth Bearer** が返します。
`Request had invalid authentication credentials. Expected OAuth 2 access token...` —
したがって、OAuth は認識されますが、**API キーは完全に拒否されます** ✗。

**理由**: EAB の作成は **管理操作**であり、「自分が誰なのか、`roles/publicca.externalAccountKeyCreator` を所有しているかどうか」を知る必要があります ✗
— API キーはプロジェクト/アプリケーション ✓ を識別するだけであり、**プリンシパル ID を表すものではありません** ✓。
(Google の「API キーを管理するためのベスト プラクティス」では、管理シナリオに **IAM + 有効期間の短いサービス アカウント資格情報** を推奨しています ✓ — ここで使用されているのと同じアプローチ ✓。)

**したがって、パスは 3 つだけです** ✓ (§3 を参照): サービス アカウント JSON ✓ / 一時アクセス トークン ✓ /
または **認証情報がまったくありません** — Cloud Shell で `gcloud publicca external-account-keys create` を実行します ✓
パネルに `keyId` / `b64MacKey` と入力します ✓ (**GCP IAM がブロックする場合はこれを使用してください** ✓)。

## 5. 自動登録と自動更新について（重要）

```
Panel: CA = Google Public CA
  ├─ usable EAB present in the config → use it
  └─ missing/expired → request a new one on the spot with the stored service-account JSON (auto-registration)
        │
First issuance on the node/panel ── registers the ACME account with EAB (one-time!)
        │
Account persisted: data/acme/accounts/google.json (panel) / <Agent data_dir>/acme/accounts/google.json (node)
        │
Every later renewal ── uses the account key only; **EAB is never needed again**
```

したがって:

- **最初の発行は自動的に登録されます**。手動の GCP ステップは必要ありません。
- 更新では EAB は **消費されません** (アカウント ファイルが存在し、データ ディレクトリが安定している限り)。
- ⚠ エージェント側の ACME データ ディレクトリは固定の絶対パス (デフォルトは `/var/lib/corefusion-agent/acme`) です。
 **むやみに変更しないでください**。そうしないと、新しいアカウントとして扱われるため、別の EAB キーが必要になります。

## 6. トラブルシューティング

| 症状・エラー | 原因 | 修正 |
|---|---|---|
| `PERMISSION_DENIED` (403) | サービス アカウントに `roles/publicca.externalAccountKeyCreator` がありません | セクション 3 ④に戻る |
| `SERVICE_DISABLED` (403) | `publicca.googleapis.com` はプロジェクトに対して有効になっていません | 第3節② |
| `NOT_FOUND` (404) | 間違ったプロジェクト ID / プロジェクトが削除されました | プロジェクトIDを確認する |
| 発行レポート `externalAccountRequired` | この CA には EAB がありません | 最初にパネルで EAB をリクエスト/構成します |
| 発行レポート `Accounts must have at least one contact.` | 登録時に電子メールが提供されませんでした | 証明書ページで ACME アカウントの電子メールを設定します (パネルはデフォルトのアカウントを使用します) |
| 更新時に再度 EAB が要求される / 登録が失敗する | ACME アカウント ファイルが失われました (データ ディレクトリが変更/消去されました) | **バックアップを復元**;ない場合は、サービス アカウントを使用して新しい EAB をリクエストします |
| 発行者が Google から Let's Encrypt に変更 | 更新コマンドに CA がありません (古いバグ、v0.12.101 で修正) | アップグレードと再発行 |

## 7. セキュリティと運用に関する推奨事項

1. **最低権限**: サービス アカウントには `roles/publicca.externalAccountKeyCreator` のみを付与し、所有者/編集者は付与しないでください。
2. **プロジェクトを削除しないでください**: GCP プロジェクトを削除すると、そのすべての ACME アカウントが無効になり、証明書を更新できなくなります。
3. **常にバックアップ** `data/acme/accounts/` (アカウント ID 自体) とパネル データベースの EAB 構成。
4. **EAB の有効期限が切れても慌てないでください**: ACME アカウント ファイルが存在する限り、更新は影響を受けません。新しい EAB は、新しいアカウントを登録する場合にのみ必要です。
5. サービス アカウント キーが漏洩した可能性がある場合: GCP でそのキーを削除し、新しい JSON をパネルに貼り付けます。

## 8. 関連エンドポイント (スクリプト用)

| エンドポイント | 説明 |
|---|---|
| `POST /api/v1/eab-configs/fetch-google` | GCP 認証を使用して EAB を作成します。本体：`{project_id, service_account_json?, access_token?}` |
| `POST /api/v1/eab-configs/fetch-zerossl` | 電子メールで ZeroSSL EAB を取得します。本体：`{email}` |
| `GET  /api/v1/eab-configs` | EAB 構成のリスト (HMAC キーまたはサービス アカウント JSON なし**、`has_mac_key` / `has_google_sa` のみ) |
