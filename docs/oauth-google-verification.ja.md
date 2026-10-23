<!-- source: oauth-google-verification.md | locale: ja | updated: 2026-10-23 -->
# Google OAuth 検証ガイド

> 言語： [简体中文](oauth-google-verification.md) · [English](oauth-google-verification.en.md) · [繁體中文](oauth-google-verification.zh-TW.md) · [Русский](oauth-google-verification.ru.md) · 日本語 · [한국어](oauth-google-verification.ko.md) · [Deutsch](oauth-google-verification.de.md) · [Français](oauth-google-verification.fr.md) · [Español](oauth-google-verification.es.md) · [Português](oauth-google-verification.pt.md) · [Türkçe](oauth-google-verification.tr.md) · [Bahasa Indonesia](oauth-google-verification.id.md) · [ไทย](oauth-google-verification.th.md) · [Tiếng Việt](oauth-google-verification.vi.md) · [العربية](oauth-google-verification.ar.md) · [فارسی](oauth-google-verification.fa.md)

> スコープ: Google CoreFusion ポータルへのサインイン (`internal/service/oauth`、スコープ: `openid`、`email`、`profile`)。
> 最後に GitHub を比較します (GitHub には検証フローがありません)。

## 1. テストモードと本番モード

Google OAuth 同意画面には 2 つの公開状態があります。

| 州 | サインインできる人 | 要検証 |
|---|---|---|
| **テスト中** | 「テスト ユーザー」リストには Google アカウントのみが含まれます | いいえ |
| **生産中** | 任意の Google アカウント | **はい** (下記を参照) |

「システム設定 → OAuth」を有効にすると、ポータルのサインイン ページにすべての訪問者に Google ボタンが表示されます。
アプリがまだテスト中の場合、テスト ユーザー リストに載っていない訪問者は「
`Error 403: access_denied` (アプリが検証されていません / あなたはテスト ユーザーではありません)。
Google サインインを公開するには、**アプリを運用環境に切り替えて検証に合格します**。

## 2. 検証のための厳しい要件

Google のレビュー (制限のないスコープの基本検証) では、次のことが必要です。

1. **ホームページ**: 承認されたドメイン (同じドメインまたはサブドメイン) 上で公的にアクセス可能な URL。
2. **プライバシー ポリシーのリンク**: **一般にアクセス可能** (サインインなし) にし、収集するデータとその使用/保存/削除方法を説明する必要があります。 Google サインインの場合、少なくとも以下をカバーする必要があります。
 収集されたGoogleアカウントデータ（電子メール、名前、アバター）、目的（ポータルアカウントの作成/サインイン）、第三者への販売禁止、
 ユーザーが自分のアカウントとデータを削除する方法についても説明します。
3. **利用規約リンク**: 一般にアクセス可能で、サービス ルールが説明されています。
4. **承認されたドメイン**: 同意画面でアプリのドメインを登録します (例: `example.com`)。 home/privacy/terms リンクは、そのドメインまたはサブドメイン上にある必要があります。
5. **アプリのロゴ (オプションですが推奨)**: 120×120px。
6. **スコープの正当性**: このプロジェクトは `openid / email / profile` (制限なし、機密ではない) のみを使用します。
 これは基本的な検証に該当します。つまり、スコープごとの正当性がなく、**サードパーティのセキュリティ評価もありません**。
 (完全な Gmail アクセスなどの制限されたスコープではそれらが必要になります。このプロジェクトでは何も使用しません。)

検証エントリ: [Google Cloud Console](https://console.cloud.google.com/) →
**API とサービス → OAuth 同意画面 → アプリを公開 → 検証のために送信します**。
基本的な検証は通常、数日以内に完了します。その間はテストモードで操作できます。

## 3. このプロジェクトで設定する内容

### 3.1 承認されたリダイレクト URI

コールバックは、サインインを開始するドメインから動的に生成されます (`internal/api/handlers/oauth/handler.go` の `Authorize` を参照)。

```
https://<portal-domain>/api/v1/client-auth/oauth/google/callback
```

- ポータル ドメイン (例: `https://p.example.com`) からサインインを開始 → `https://p.example.com/...` を登録;
- パネルドメイン`/p`から起動→`https://panel.example.com/...`を登録。
- Google では **複数** リダイレクト URI が許可されます。両方のドメインを登録します。
- ⚠ Google では、リダイレクト URI が **https** である必要があります (`http://localhost` を除く)。
 リバース プロキシは `X-Forwarded-Host` / `X-Forwarded-Proto` を転送する必要があります。そうでない場合、パネルは
 http または内部 redirect_uri → Google が拒否 (`redirect_uri_mismatch`)。

「システム設定 → OAuth → ポータル コールバック ドメイン」を設定した後、ページに表示されているコールバック URL が Google に貼り付けられるものです。

### 3.2 ポータル上のプライバシー ポリシー/規約リンク

サインイン カードには「プライバシー ポリシー · サービス利用規約」リンク (構成後に表示されます) が含まれており、どちらのドキュメントにも **スタンドアロン ページ**があります。
(ダイアログとスタンドアロン ページは同じコンテンツを共有します):

- メインパネルドメイン: `https://<panel-domain>/p/legal/privacy`、`/p/legal/terms`
- 専用ポータル ドメイン (ルート展開): `https://<portal-domain>/legal/privacy`、`/legal/terms`
- `?lang=xx`をサポートします。これがない場合は、ブラウザの `Accept-Language` が使用され、ポータルのデフォルトが使用されます。
- ページは **サーバー レンダリング** (JS なし) であり、Google の検証クローラーに役立ちます。

**構成 (どちらの方法でも、リンクが優先)**:

1. **コンテンツ モード (推奨、すぐに使用できます)**: ポータル エディター → 設定 → 一般 → コンプライアンス ドキュメント →
 「言語の編集」を選択し、§4 テンプレートを「プライバシー ポリシーの内容」/「サービス利用規約の内容」に貼り付けます。
 (**Markdown、言語ごとに編集**) → 保存後、サインイン カードには **ダイアログ**を開くリンクが表示されます。
 編集者の「スタンドアロン リンク」は、Google 同意画面に直接入力できます。
2. **外部リンク モード**: 「プライバシー ポリシー URL」/「利用規約 URL」 (ホストしているページを指す) を入力 → 新しいタブで開きます。

コンテンツとリンクは `/api/v1/public/portal-config` (`privacy_policy_content` /
`terms_of_service_content` は **言語ごとのマークダウン マップ** `{"zh-CN":"...","en":"..."}`。
`privacy_policy_url` / `terms_of_service_url` はオプションの外部リンクです)。 **サインインせずにアクセスできる必要があります** ✓。

### 3.3 パネル側スイッチ

`System settings → OAuth`: Google を有効にし、クライアント ID / クライアント シークレットを入力します (シークレットは保存時に AES-256-GCM で暗号化されます)。
Google ボタンは、すべてが構成されている場合にのみ、ポータルのサインイン ページに表示されます (`/api/v1/client-auth/oauth/providers` が有効に戻ります)。

## 4. プライバシーポリシー/規約テンプレート

> 適応できる最小限のコンプライアンス テンプレート。使用前: `【】` プレースホルダーを置き換え、ビジネスに合わせて条項を追加/削除します。
> 運営主体に見直してもらう。これらのテンプレートは法的なアドバイスではありません。

### 4.1 プライバシーポリシー（テンプレート）

```markdown
# Privacy Policy

Last updated: [DATE]

[OPERATOR NAME] ("we") operates [PORTAL NAME] ("the Service"). We respect your privacy;
this policy explains what information we collect and how we use it.

## 1. Information we collect
- **Account information**: the email address and password you provide at registration (stored only as a one-way hash);
- **Google sign-in information**: when you choose "Sign in with Google", we receive, with your authorization,
  your Google account email, display name and avatar URL (scopes: openid, email, profile);
- **Service data**: subscription configuration, traffic usage, device IP allow-list, tickets and order records.

## 2. How we use information
Only to: create and sign in to your account, provide the proxy subscription service, bill you and provide support,
and for security controls (sign-in protection, IP allow-list). We **do not** sell your personal information.

## 3. Third-party services
- **Google**: receives account information within the authorized scope only when you actively use "Sign in with Google";
- Payment providers: receive order information only when you initiate a payment.

## 4. Storage and security
Data is stored on our servers; credentials are encrypted at rest and all traffic uses TLS.

## 5. Your rights
You can update your profile/password and end sessions in the portal at any time; to delete your account and all data,
contact [CONTACT EMAIL] and we will process the request within [N] business days.

## 6. Policy changes
Material changes will be announced in the portal notice board.
```

### 4.2 利用規約（テンプレート）

```markdown
# Terms of Service

Last updated: [DATE]

## 1. Service
The Service provides [PROXY / NETWORK ACCESS] to registered users; specific plans are described on the store page.

## 2. Accounts
- You are responsible for the security of your account and password; for accounts created with Google sign-in, credentials are managed by Google;
- You may not lend or transfer your account, or use the Service for any unlawful purpose (including but not limited to:
  unauthorized access to others' systems, distributing illegal content, or circumventing mandatory legal requirements).

## 3. Payment and refunds
Plans are billed [monthly / by usage]; traffic and device limits follow the plan description;
[REFUND POLICY: e.g. full refund within 7 days if unused].

## 4. Changes and termination
We may suspend or terminate the Service due to force majeure or your breach of these terms; deleting an account deletes the related data.

## 5. Disclaimer
The Service is provided "as is"; to the extent permitted by law, we are not liable for indirect losses caused by use of the Service.

## 6. Miscellaneous
These terms are governed by the laws of [JURISDICTION]. Contact: [CONTACT EMAIL].
```

## 5. 発売前チェックリスト

- [ ] OAuth 同意画面: アプリ名 / ロゴ / ホーム ページ / プライバシー ポリシー / 規約がすべて表示されます。ドメイン登録済み
- [ ] プライバシー ポリシーと規約のページ **サインインせずにアクセス可能** (ポータルのカスタム ページまたはスタンドアロンの静的ページ)
- [ ] 両方のリンクがポータルのサインイン カード/フッターに表示されます
- [ ] リダイレクト URI は実際のドメインと一致し、https になります
- [ ] アプリを公開 → 検証のために送信します。テストユーザーの制限は承認後に解除されます
- [ ] パネル OAuth 設定: クライアント ID/シークレットを入力して、enabled=true (プロバイダー エンドポイントは、enabled:true を返します)

## 6. GitHubの比較

GitHub OAuth アプリには公開状態または検証フローが**ありません**。アプリを作成し、コールバック URL を設定すると、公開で機能します。
GitHub OAuth アプリは **1 つ** の承認コールバック URL (プレフィックス マッチング) のみを受け入れることに注意してください。複数のドメインの場合、
Google のマルチ URI アプローチを使用するか、ポータル ドメインごとに別のアプリを作成します。
