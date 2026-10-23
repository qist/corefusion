# CoreFusion 文档

> 语言： 简体中文 · [English](README.en.md) · [繁體中文](README.zh-TW.md) · [Русский](README.ru.md) · [日本語](README.ja.md) · [한국어](README.ko.md) · [Deutsch](README.de.md) · [Français](README.fr.md) · [Español](README.es.md) · [Português](README.pt.md) · [Türkçe](README.tr.md) · [Bahasa Indonesia](README.id.md) · [ไทย](README.th.md) · [Tiếng Việt](README.vi.md) · [العربية](README.ar.md) · [فارسی](README.fa.md)

## [USER-GUIDE.md](./USER-GUIDE.md) — 使用者操作手册

面向**使用者**（运维 / 管理员 / 操作员）的完整操作手册：安装部署、初始配置、各功能模块操作、常见问题与附录。

## [api/](./api/) — 对外 API 文档

给集成者 / 二开 / 第三方客户端看的接口说明：

- [api/README.md](./api/README.md) — 总览：Base URL、认证方式、响应格式、限流
- [api/subscription.md](./api/subscription.md) — 订阅拉取接口（`/sub/:token`），含格式参数、响应头、示例
- [api/portal-api.md](./api/portal-api.md) — 客户端门户 API（注册登录、套餐、订单、余额、工单等）
- [api/charge-callback.md](./api/charge-callback.md) — 支付回调签名与验签（对接支付平台用）
- [api/admin-api.md](./api/admin-api.md) — 管理端 API 路由表（按模块分组，自代码提取）

## 设计文档

设计文档属于内部资料（核心知识产权），**不随本仓库分发**。需要查阅请联系项目维护者。

> **范围说明**：客户端 App（本机运行界面、TUN 接管、托盘、多订阅合并等）为独立产品，不在本仓库文档范围，需另行设计。本仓库文档只描述服务端（面板 / 节点 / 订阅下发）。

## 其他（运维手册）

- `cert-google-eab.md` — Google EAB 证书签发操作手册
- `oauth-google-verification.md` — Google OAuth 验证
- `notification-templates.md` — 通知模板说明

## 多语言覆盖

| Document | zh-CN | en | zh-TW | Other languages |
|---|---|---|---|---|
| README.md | ✅ | ✅ | ✅ | ru, ja, ko, de, fr, es, pt, tr, id, th, vi, ar, fa ✅ |
| USER-GUIDE.md | ✅ | ✅ | ✅ | ru, ja, ko, de, fr, es, pt, tr, id, th, vi, ar, fa ✅ |
| I18N.md | ✅ | — | — | — |
| api/README.md | ✅ | ✅ | ✅ | ru, ja, ko, de, fr, es, pt, tr, id, th, vi, ar, fa ✅ |
| api/subscription.md | ✅ | ✅ | ✅ | ru, ja, ko, de, fr, es, pt, tr, id, th, vi, ar, fa ✅ |
| api/portal-api.md | ✅ | ✅ | ✅ | ru, ja, ko, de, fr, es, pt, tr, id, th, vi, ar, fa ✅ |
| api/charge-callback.md | ✅ | ✅ | ✅ | ru, ja, ko, de, fr, es, pt, tr, id, th, vi, ar, fa ✅ |
| api/admin-api.md | ✅ | ✅ | ✅ | ru, ja, ko, de, fr, es, pt, tr, id, th, vi, ar, fa ✅ |
| cert-google-eab.md | ✅ | ✅ | ✅ | ru, ja, ko, de, fr, es, pt, tr, id, th, vi, ar, fa ✅ |
| oauth-google-verification.md | ✅ | ✅ | ✅ | ru, ja, ko, de, fr, es, pt, tr, id, th, vi, ar, fa ✅ |
| notification-templates.md | ✅ | ✅ | ✅ | ru, ja, ko, de, fr, es, pt, tr, id, th, vi, ar, fa ✅ |
| LEGAL.md | ✅ | ✅ | ✅ | ru, ja, ko, de, fr, es, pt, tr, id, th, vi, ar, fa ✅ |
| THIRD_PARTY_LICENSES.md | ✅ | ✅ | ✅ | ru, ja, ko, de, fr, es, pt, tr, id, th, vi, ar, fa ✅ |

命名规则、语言导航与校验流程见 [I18N.md](./I18N.md)。
