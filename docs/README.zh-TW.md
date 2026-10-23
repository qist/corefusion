<!-- source: README.md | locale: zh-TW | updated: 2026-10-23 -->
# CoreFusion 文件

> 語言： [简体中文](README.md) · [English](README.en.md) · 繁體中文 · [Русский](README.ru.md) · [日本語](README.ja.md) · [한국어](README.ko.md) · [Deutsch](README.de.md) · [Français](README.fr.md) · [Español](README.es.md) · [Português](README.pt.md) · [Türkçe](README.tr.md) · [Bahasa Indonesia](README.id.md) · [ไทย](README.th.md) · [Tiếng Việt](README.vi.md) · [العربية](README.ar.md) · [فارسی](README.fa.md)

## [USER-GUIDE.zh-TW.md](./USER-GUIDE.zh-TW.md) — 使用者操作手冊

面向**使用者**（維運 / 管理員 / 操作員）的完整操作手冊：安裝部署、初始設定、各功能模組操作、常見問題與附錄。

## [api/](./api/) — 對外 API 文件

給整合者 / 二次開發 / 第三方用戶端看的介面說明：

- [api/README.zh-TW.md](./api/README.zh-TW.md) — 總覽：Base URL、認證方式、回應格式、限流
- [api/subscription.zh-TW.md](./api/subscription.zh-TW.md) — 訂閱拉取介面（`/sub/:token`），含格式參數、回應標頭、範例
- [api/portal-api.zh-TW.md](./api/portal-api.zh-TW.md) — 用戶端入口網站 API（註冊登入、套餐、訂單、餘額、工單等）
- [api/charge-callback.zh-TW.md](./api/charge-callback.zh-TW.md) — 支付回呼簽名與驗簽（對接支付平台用）
- [api/admin-api.zh-TW.md](./api/admin-api.zh-TW.md) — 管理端 API 路由表（按模組分組，自程式碼提取）

## 設計文件

設計文件屬於內部資料（核心智慧財產權），**不隨本儲存庫發佈**。需要查閱請聯絡專案維護者。

> **範圍說明**：用戶端 App（本機執行介面、TUN 接管、托盤、多訂閱合併等）為獨立產品，不在本儲存庫文件範圍，需另行設計。本儲存庫文件只描述伺服器端（面板 / 節點 / 訂閱下發）。

## 其他（維運手冊）

- [cert-google-eab.zh-TW.md](./cert-google-eab.zh-TW.md) — Google EAB 憑證簽發操作手冊
- [oauth-google-verification.zh-TW.md](./oauth-google-verification.zh-TW.md) — Google OAuth 驗證
- [notification-templates.zh-TW.md](./notification-templates.zh-TW.md) — 通知模板說明

## 翻譯覆蓋

| 文件 | zh-CN | en | zh-TW | ru ✅ |
|------|-------|----|-------|---------|
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

命名、語言導覽與校驗規則見 [I18N.md](./I18N.md)。
