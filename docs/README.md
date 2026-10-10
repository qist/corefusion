# CoreFusion 文档

## [USER-GUIDE.md](./USER-GUIDE.md) — 使用者操作手册

面向**使用者**（运维 / 管理员 / 操作员）的完整操作手册：安装部署、初始配置、各功能模块操作、常见问题与附录。

## [api/](./api/) — 对外 API 文档

给集成者 / 二开 / 第三方客户端看的接口说明：

- [api/README.md](./api/README.md) — 总览：Base URL、认证方式、响应格式、限流
- [api/subscription.md](./api/subscription.md) — 订阅拉取接口（`/sub/:token`），含格式参数、响应头、示例
- [api/portal-api.md](./api/portal-api.md) — 客户端门户 API（注册登录、套餐、订单、余额、工单等）
- [api/charge-callback.md](./api/charge-callback.md) — 支付回调签名与验签（对接支付平台用）
- [api/admin-api.md](./api/admin-api.md) — 管理端 API 路由表（按模块分组，自代码提取）

## [design/](./design/) — 设计文档

架构与功能设计、审计报告（内部参考）：

| 类别 | 文档 |
|---|---|
| 总体架构 | `CODE_LEVEL_DESIGN.md`（代码级设计）、`CODE_STRUCTURE_SECURITY.md`（结构与安全）、`core-data-layer-design.md` |
| 订阅 | `SUBSCRIPTION_DESIGN.md`、`singbox-configgen-design.md` |
| 商务 | `plan-design.md`、`plan-integration.md`、`plan-node-account-sync.md`、`CHARGE_SITE_INTEGRATION.md` |
| 节点/Agent | `node-server-architecture.md`、`chain-proxy-redesign.md`、`mirror-sync-redesign.md`、`one-line-agent-deploy.md`、`agent-push-update.md`、`eliminate-reverse-dial.md`、`remote-traffic-sync.md` |
| 门户 | `portal-redesign.md`、`PORTAL_EDITOR_DESIGN.md`、`PORTAL_REGION_EDITOR_DESIGN.md`、`PORTAL_TEMPLATE_FRONTEND_DESIGN.md`、`PORTAL_FREEFORM_DESIGN.md` |
| 安全审计 | `security-audit-v2.md` ~ `v5.md`、`SECURITY_AUDIT_2026-09-30.md`、`code-audit-report.md`、`WAF_TEST_ISSUES.md` |
| 流量/配额 | `TRAFFIC_QUOTA_DESIGN.md`、`rate-limit-design.md`、`remote-traffic-sync.md` |
| UI | `UI_DESIGN.md`、`NETWORK_CONTROL_UI.md`、`ONLINE_USERS_DESIGN.md`、`DASHBOARD_MULTINODE_DESIGN.md` |
| 其他 | `first-boot-initialization.md`、`log-management.md`、`waf-rule-store.md`、`MISSING_CLIENT_API_DESIGN.md`、`inbound-design.md`、`share-e2e-connectivity-*`、`NODE_PLANNING.md`、`UNIMPLEMENTED_FEATURES.md`（历史审计）、`PAGINATION_PLAN.md` |

> 设计文档为某一时刻的快照；**实现状态以代码与提交历史为准**，多数文档顶部有"实现状态"修订说明。

## 其他（运维手册）

- `cert-google-eab.md` — Google EAB 证书签发操作手册
- `oauth-google-verification.md` — Google OAuth 验证
- `notification-templates.md` — 通知模板说明
- 仓库根目录 `DEVELOPMENT.md` — 开发指南（构建、部署、数据库）
