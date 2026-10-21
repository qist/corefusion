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

## 设计文档

设计文档属于内部资料（核心知识产权），**不随本仓库分发**。需要查阅请联系项目维护者。

> **范围说明**：客户端 App（本机运行界面、TUN 接管、托盘、多订阅合并等）为独立产品，不在本仓库文档范围，需另行设计。本仓库文档只描述服务端（面板 / 节点 / 订阅下发）。

## 其他（运维手册）

- `cert-google-eab.md` — Google EAB 证书签发操作手册
- `oauth-google-verification.md` — Google OAuth 验证
- `notification-templates.md` — 通知模板说明
- 开发与架构文档（构建、部署、数据库设计）为内部资料，**不随本仓库分发**

接口	路由	Handler	说明
首次启动初始化	独立初始化向导服务（随机端口 18000-18999 + init_token）/ /init 前端向导 / corefusion init|reset-init	InitWizard	已实现（Web 向导 + CLI；步骤：数据库[SQLite/MySQL/Pg]、数据目录、env 加密密钥、核心安装、代理[Nginx/Caddy 二选一]、面板/订阅/门户端口+域名+默认货币+一键添加到代理、HTTPS[自签名·上传·ACME]、配置清单/下载 .env；监听地址按是否配置证书自动决定；核心/代理重启后自动安装）
面板版本列表	GET /api/v1/system/releases	ListReleases	返回 501
面板自更新	POST /api/v1/system/update	SystemUpdate	返回 501
面板自回滚	POST /api/v1/system/rollback	SystemRollback	返回 501
面板更新任务列表	GET /api/v1/system/update-jobs	ListUpdateJobs	返回 501