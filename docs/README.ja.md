<!-- source: README.md | locale: ja | updated: 2026-10-23 -->
# CoreFusion ドキュメント

> 言語： [简体中文](README.md) · [English](README.en.md) · [繁體中文](README.zh-TW.md) · [Русский](README.ru.md) · 日本語 · [한국어](README.ko.md) · [Deutsch](README.de.md) · [Français](README.fr.md) · [Español](README.es.md) · [Português](README.pt.md) · [Türkçe](README.tr.md) · [Bahasa Indonesia](README.id.md) · [ไทย](README.th.md) · [Tiếng Việt](README.vi.md) · [العربية](README.ar.md) · [فارسی](README.fa.md)

## [USER-GUIDE.ja.md](./USER-GUIDE.ja.md) — ユーザーガイド

**ユーザー** (運用 / 管理者 / オペレーター) 向けの完全な操作ガイド: インストール、初期構成、機能モジュール、FAQ、付録。

## [api/](./api/) — パブリック API リファレンス

インテグレーター、二次開発、サードパーティのクライアントの場合:

- [api/README.ja.md](./api/README.ja.md) — 概要: 基本 URL、認証、応答形式、レート制限
- [api/subscription.ja.md](./api/subscription.ja.md) — サブスクリプション エンドポイント (`/sub/:token`)、形式、応答ヘッダー、例
- [api/portal-api.ja.md](./api/portal-api.ja.md) — クライアント ポータル API (サインアップ/サインイン、プラン、注文、残高、チケット)
- [api/charge-callback.ja.md](./api/charge-callback.ja.md) — 支払いコールバックの署名と検証
- [api/admin-api.ja.md](./api/admin-api.ja.md) — モジュールごとにグループ化された管理者 API ルート テーブル

## 設計書

設計ドキュメントは内部資料 (中核的な知的財産) であり、このリポジトリでは **配布されません**。必要な場合はメンテナに連絡してください。

> **範囲**: クライアント アプリ (ローカル UI、TUN テイクオーバー、トレイ、マルチサブスクリプションのマージ) は別の製品であり、このリポジトリの範囲外です。ここのドキュメントでは、サーバー側 (パネル/ノード/サブスクリプション配信) のみを説明します。

## その他の操作ガイド

- `cert-google-eab.md` — Google EAB 証明書の発行 *(中国語)*
- `oauth-google-verification.md` — Google OAuth 検証 *(中国語)*
- `notification-templates.md` — 通知テンプレート変数 *(中国語)*

## 翻訳範囲

| 文書 | zh-CN | en | zh-TW | 他のロケール |
|----------|-------|----|-------|---------------|
| README.md | ✅ | ✅ | ✅ | ru ✅ |
| USER-GUIDE.md | ✅ | ✅ | ✅ | ru ✅ |
| I18N.md | ✅ | — | — | — |
| api/README.md | ✅ | ✅ | ✅ | ru ✅ |
| api/subscription.md | ✅ | ✅ | ✅ | ru ✅ |
| api/portal-api.md | ✅ | ✅ | ✅ | ru ✅ |
| api/charge-callback.md | ✅ | ✅ | ✅ | ru ✅ |
| api/admin-api.md | ✅ | ✅ | ✅ | ru ✅ |
| cert-google-eab.md | ✅ | ✅ | ✅ | ru ✅ |
| oauth-google-verification.md | ✅ | ✅ | ✅ | ru ✅ |
| notification-templates.md | ✅ | ✅ | ✅ | ru ✅ |
| LEGAL.md | ✅ | ✅ | ✅ | ru ✅ |
| THIRD_PARTY_LICENSES.md | ✅ | ✅ | ✅ | ru ✅ |

命名、言語ナビゲーション、レビュー ルール *(中国語)* については、[I18N.md](README.ja.md) を参照してください。
