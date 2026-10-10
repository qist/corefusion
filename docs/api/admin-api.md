# 管理端 API（Admin API）

面向**面板二开 / 自动化运维 / 对接方**的管理端接口全量路由表。

- Base URL：`https://<面板域名>/api/v1`
- 认证：`Authorization: Bearer <admin_token>`（`POST /api/v1/auth/login` 获取）
- 通用响应/错误格式、限流约定见 [README.md](./README.md)
- 路由权威来源：`internal/api/router/router.go`（共 739 条管理端路由）；与本表不一致时**以代码为准**

## 权限标记

| 标记 | 含义 |
|---|---|
| 公开 | 无需登录（仍可能有维护模式守卫 / 限流） |
| 公开·限流 | 无需登录，且带专用限流 |
| 登录 | 需已登录的管理端 token（`AuthRequired`） |
| 管理员 | 需角色 ≥ `admin`（`RequireRole(RoleAdmin)`） |
| 超级管理员 | 需角色 = `super_admin` |

角色等级：`viewer(10) < operator(20) < admin(30) < super_admin(40)`；不满足返回 403 `INSUFFICIENT_ROLE`。

---

## 快速开始（真实示例）

以下示例均取自本机真实响应，token 与数据已脱敏。

### 1. 登录获取 token

```bash
curl -X POST https://<面板域名>/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"你的密码"}'
```

响应：

```json
{
  "data": {
    "access_token": "<JWT，有效期 24h>",
    "refresh_token": "<JWT，有效期 7 天>",
    "user": { "id": 1, "role": "super_admin", "username": "admin" }
  }
}
```

### 2. 带 token 调用业务接口

```bash
curl https://<面板域名>/api/v1/clients?limit=1 \
  -H 'Authorization: Bearer <access_token>'
```

响应（列表统一 `items` + `total` 结构）：

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

### 3. 错误响应样例

**401 未认证：**

```json
{
  "code": "MISSING_TOKEN",
  "message": "missing authorization header",
  "request_id": "a02cfdf9-6be0-46be-8584-824caab34dc7",
  "can_retry": false,
  "requires_action": false
}
```

**404 资源不存在：**

```json
{
  "code": "CLIENT_NOT_FOUND",
  "message": "client not found",
  "request_id": "099949bd-4ded-468d-a6f8-f8e798af2273",
  "can_retry": false,
  "requires_action": false
}
```

### 4. 分页约定

所有列表接口统一使用 `limit`（每页条数，默认 20）与 `偏移量 offset`（从 0 开始），返回 `{items, total, limit, offset}`。部分旧接口返回裸数组（见各表标注）。

---

## 目录

- [管理员认证与安全](#管理员认证与安全)
- [管理员账号](#管理员账号)
- [客户管理](#客户管理)
- [套餐模板](#套餐模板)
- [套餐变体](#套餐变体)
- [流量包](#流量包)
- [优惠码](#优惠码)
- [节点分组](#节点分组)
- [节点服务器](#节点服务器)
- [节点审计日志](#节点审计日志)
- [节点（旧版模型）](#节点旧版模型)
- [地区总览](#地区总览)
- [订阅](#订阅)
- [订阅格式](#订阅格式)
- [订阅接入点](#订阅接入点)
- [分享域名](#分享域名)
- [Cloudflare IP](#cloudflare-ip)
- [订阅配置（Mihomo/Clash）](#订阅配置mihomoclash)
- [充值站点](#充值站点)
- [充值订单](#充值订单)
- [充值卡密](#充值卡密)
- [支付回调事件](#支付回调事件)
- [支付回调重试](#支付回调重试)
- [客户余额](#客户余额)
- [礼品](#礼品)
- [推广配置](#推广配置)
- [推广提现](#推广提现)
- [IP 白名单（汇总）](#ip-白名单汇总)
- [客户 OAuth 绑定](#客户-oauth-绑定)
- [工单（管理端）](#工单管理端)
- [文章](#文章)
- [共享账号](#共享账号)
- [证书](#证书)
- [证书通道](#证书通道)
- [EAB 配置](#eab-配置)
- [ACME 账号](#acme-账号)
- [内置 CA](#内置-ca)
- [Geo 规则](#geo-规则)
- [WAF 规则库](#waf-规则库)
- [日志管理](#日志管理)
- [备份](#备份)
- [系统设置](#系统设置)
- [安全审计](#安全审计)
- [指标](#指标)
- [门户模板](#门户模板)
- [Agent 安装](#agent-安装)
- [支付回调入口](#支付回调入口)
- [入站（含节点）](#入站含节点)
- [公开配置](#公开配置)
- [客户认证与业务（管理端引擎上的同一套）](#客户认证与业务管理端引擎上的同一套)
- [订阅下发](#订阅下发)
- [运行时配置工作流](#运行时配置工作流)
- [服务访问策略](#服务访问策略)
- [流量配额监控](#流量配额监控)
- [健康检查](#健康检查)
- [门户页](#门户页)
- [ACME 挑战](#acme-挑战)

---

### 管理员认证与安全

面板管理端的登录、登出、改密、2FA（TOTP）、Passkey（WebAuthn）与一次性票据。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| POST | `/api/v1/auth/2fa/disable` | 登录 | 禁用 |
| POST | `/api/v1/auth/2fa/enable` | 登录 | 启用 |
| POST | `/api/v1/auth/2fa/recovery-codes` | 登录 | 重新生成恢复码 |
| POST | `/api/v1/auth/2fa/setup` | 登录 | 设置 |
| GET | `/api/v1/auth/2fa/status` | 登录 | 状态 |
| POST | `/api/v1/auth/change-password` | 登录 | 修改密码 |
| POST | `/api/v1/auth/login` | 公开 | 登录 |
| POST | `/api/v1/auth/login/2fa` | 公开 | 第二步验证（2FA）登录 |
| GET | `/api/v1/auth/login/2fa-status` | 公开 | 查询账号是否启用 2FA |
| POST | `/api/v1/auth/logout` | 登录 | 登出 |
| GET | `/api/v1/auth/me` | 登录 | 当前登录用户 |
| GET | `/api/v1/auth/passkey/credentials` | 登录 | Passkey 凭据列表 |
| DELETE | `/api/v1/auth/passkey/credentials/:id` | 登录 | 删除 Passkey 凭据 |
| POST | `/api/v1/auth/passkey/login/begin` | 公开 | 登录发起 |
| POST | `/api/v1/auth/passkey/login/finish` | 公开 | 登录完成 |
| POST | `/api/v1/auth/passkey/register/begin` | 登录 | 注册发起 |
| POST | `/api/v1/auth/passkey/register/finish` | 登录 | 注册完成 |
| GET | `/api/v1/auth/passkey/rpids` | 登录 | RP ID 白名单列表 |
| POST | `/api/v1/auth/passkey/rpids` | 登录 | 更新 RP ID 白名单 |
| GET | `/api/v1/auth/passkey/status` | 公开 | 状态 |
| POST | `/api/v1/auth/refresh` | 公开 | 刷新 |
| POST | `/api/v1/auth/ticket` | 登录 | 签发一次性短时票据 |

### 管理员账号

管理面板使用者账号。列表需管理员，增删改仅超级管理员（防止普通管理员自我提权）。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/v1/users` | 管理员 | 列表 |
| POST | `/api/v1/users` | 超级管理员 | 创建 |
| DELETE | `/api/v1/users/:id` | 超级管理员 | 删除 |
| PUT | `/api/v1/users/:id` | 超级管理员 | 更新 |

### 客户管理

订阅客户（Portal 账号）的增删改查、暂停/恢复、升降级、续费、用量重置。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/v1/clients` | 登录 | 列表客户 |
| POST | `/api/v1/clients` | 登录 | 创建客户。body：`{"email":"user@example.com","password":"明文密码","plan_id":1,"quota_bytes":10737418240,"expires_at":"2026-12-31T00:00:00Z","remark":""}`。`plan_id` 必填，`password` 可空（客户后续在门户自设） |
| DELETE | `/api/v1/clients/:id` | 登录 | 删除客户 |
| GET | `/api/v1/clients/:id` | 登录 | 获取客户 |
| PUT | `/api/v1/clients/:id` | 登录 | 更新客户 |
| POST | `/api/v1/clients/:id/downgrade` | 登录 | 客户降级 |
| GET | `/api/v1/clients/:id/orders` | 登录 | 客户的订单列表 |
| POST | `/api/v1/clients/:id/renew` | 登录 | 客户续费 |
| POST | `/api/v1/clients/:id/reset-usage` | 登录 | 重置用量 |
| POST | `/api/v1/clients/:id/restore` | 登录 | 恢复客户 |
| POST | `/api/v1/clients/:id/suspend` | 登录 | 暂停客户 |
| POST | `/api/v1/clients/:id/upgrade` | 登录 | 客户升级 |

### 套餐模板

套餐（变体、计费周期、价格、流量额度）与模板级节点授权。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/v1/plan-templates` | 登录 | 模板列表 |
| POST | `/api/v1/plan-templates` | 登录 | 创建模板 |
| DELETE | `/api/v1/plan-templates/:id` | 登录 | 删除模板 |
| GET | `/api/v1/plan-templates/:id` | 登录 | 模板详情 |
| PUT | `/api/v1/plan-templates/:id` | 登录 | 更新模板 |
| GET | `/api/v1/plan-templates/:id/accessible-nodes` | 登录 | 获取可用节点 |
| GET | `/api/v1/plan-templates/:id/billing-node-options` | 登录 | 获取计费节点选项 |
| PUT | `/api/v1/plan-templates/:id/node-groups` | 登录 | 设置模板节点分组 |
| POST | `/api/v1/plan-templates/:id/sync-accounts` | 登录 | 同步账号给模板 |
| GET | `/api/v1/plan-templates/:id/variants` | 登录 | 变体列表 |
| POST | `/api/v1/plan-templates/:id/variants` | 登录 | 创建变体 |
| POST | `/api/v1/plan-templates/sync-all-accounts` | 登录 | 同步全部账号 |

### 套餐变体

套餐变体的更新与删除（创建挂在套餐模板下）。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| DELETE | `/api/v1/plan-variants/:id` | 登录 | 删除变体 |
| PUT | `/api/v1/plan-variants/:id` | 登录 | 更新变体 |

### 流量包

可单独购买的流量包。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/v1/traffic-packs` | 登录 | 流量包列表 |
| POST | `/api/v1/traffic-packs` | 登录 | 创建流量包 |
| DELETE | `/api/v1/traffic-packs/:id` | 登录 | 删除流量包 |
| PUT | `/api/v1/traffic-packs/:id` | 登录 | 更新流量包 |

### 优惠码

促销/优惠码的创建、校验与核销配置。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/v1/promotions` | 登录 | 列表优惠码 |
| POST | `/api/v1/promotions` | 登录 | 创建优惠码 |
| DELETE | `/api/v1/promotions/:id` | 登录 | 删除优惠码 |
| PUT | `/api/v1/promotions/:id` | 登录 | 更新优惠码 |
| POST | `/api/v1/promotions/validate` | 登录 | 校验优惠码 |

### 节点分组

把节点组织成分组，供套餐引用。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/v1/node-groups` | 登录 | 列表节点分组 |
| POST | `/api/v1/node-groups` | 登录 | 创建节点分组 |
| DELETE | `/api/v1/node-groups/:id` | 登录 | 删除节点分组 |
| PUT | `/api/v1/node-groups/:id` | 登录 | 更新节点分组 |
| GET | `/api/v1/node-groups/:id/members` | 登录 | 列表分组成员 |
| POST | `/api/v1/node-groups/:id/members` | 登录 | 新增分组成员 |
| DELETE | `/api/v1/node-groups/:id/members/:mid` | 登录 | 移除分组成员 |

### 节点服务器

核心资源：节点增删改查、核心（xray/sing-box）配置下发、入站/出站/端点配置、进程控制、证书、WAF、限速、端口跳跃、链式代理、镜像/主备、Agent 安装升级、xray 运行时控制等。子路径极多，按功能族列出。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/v1/node-servers` | 管理员 | 列表 |
| POST | `/api/v1/node-servers` | 管理员 | 创建 |
| DELETE | `/api/v1/node-servers/:id` | 管理员 | 删除 |
| GET | `/api/v1/node-servers/:id` | 管理员 | 详情 |
| PUT | `/api/v1/node-servers/:id` | 管理员 | 更新 |
| GET | `/api/v1/node-servers/:id/binaries` | 管理员 | 已装核心列表 |
| POST | `/api/v1/node-servers/:id/binaries/ech-generate` | 管理员 | 生成 ECH 密钥对 |
| POST | `/api/v1/node-servers/:id/binaries/install` | 管理员 | 安装/升级核心 |
| GET | `/api/v1/node-servers/:id/binaries/interfaces` | 管理员 | 网卡列表 |
| POST | `/api/v1/node-servers/:id/binaries/mldsa65-key` | 管理员 | 生成 MLDSA65 密钥 |
| POST | `/api/v1/node-servers/:id/binaries/reality-key` | 管理员 | 生成 REALITY 密钥对 |
| POST | `/api/v1/node-servers/:id/binaries/reality-public-key` | 管理员 | 由 REALITY 私钥推导公钥 |
| GET | `/api/v1/node-servers/:id/binaries/releases` | 管理员 | 可用版本列表 |
| POST | `/api/v1/node-servers/:id/binaries/vlessenc` | 管理员 | 生成 VLESS 加密参数 |
| POST | `/api/v1/node-servers/:id/binaries/wg-key` | 管理员 | 生成 WireGuard 密钥 |
| POST | `/api/v1/node-servers/:id/binaries/x25519-key` | 管理员 | 生成 X25519 密钥对 |
| GET | `/api/v1/node-servers/:id/bootstrap-cmd` | 管理员 | 获取 Agent 引导命令 |
| POST | `/api/v1/node-servers/:id/bootstrap-token` | 管理员 | 生成引导令牌 |
| POST | `/api/v1/node-servers/:id/caddy/batch-update-log-enabled` | 管理员 | 批量更新日志开关 |
| POST | `/api/v1/node-servers/:id/caddy/batch-update-ports` | 管理员 | 批量更新端口 |
| POST | `/api/v1/node-servers/:id/caddy/generate-config` | 管理员 | 生成配置 |
| DELETE | `/api/v1/node-servers/:id/caddy/l4-rules` | 管理员 | 删除全部 L4 规则 |
| POST | `/api/v1/node-servers/:id/caddy/save-and-apply` | 管理员 | 保存并应用 |
| GET | `/api/v1/node-servers/:id/caddy/sites` | 管理员 | 站点列表 |
| POST | `/api/v1/node-servers/:id/caddy/sites` | 管理员 | 创建站点 |
| DELETE | `/api/v1/node-servers/:id/caddy/sites/:siteId` | 管理员 | 删除站点 |
| PUT | `/api/v1/node-servers/:id/caddy/sites/:siteId` | 管理员 | 更新站点 |
| DELETE | `/api/v1/node-servers/:id/caddy/sni-rules` | 管理员 | 删除全部 SNI 规则 |
| POST | `/api/v1/node-servers/:id/caddy/sni-rules/toggle` | 管理员 | 开关 SNI 规则 |
| GET | `/api/v1/node-servers/:id/caddy/sni-settings` | 管理员 | SNI 设置 |
| POST | `/api/v1/node-servers/:id/caddy/sni-settings` | 管理员 | 保存 SNI 设置 |
| GET | `/api/v1/node-servers/:id/certificates` | 管理员 | 节点证书列表 |
| DELETE | `/api/v1/node-servers/:id/certificates/:cid` | 管理员 | 解绑证书 |
| POST | `/api/v1/node-servers/:id/certificates/:cid` | 管理员 | 绑定证书 |
| GET | `/api/v1/node-servers/:id/chain-config` | 管理员 | 生成链式配置 |
| GET | `/api/v1/node-servers/:id/chain-health` | 管理员 | 链式健康状态 |
| POST | `/api/v1/node-servers/:id/control` | 超级管理员 | 控制节点核心。body：`{"core_type":"xray","action":"start"}`（action = start/stop/restart）。`core_type` 可选 `xray`/`singbox` |
| GET | `/api/v1/node-servers/:id/degradation-status` | 管理员 | 降级状态 |
| POST | `/api/v1/node-servers/:id/distribute-cert` | 管理员 | 分发证书 |
| GET | `/api/v1/node-servers/:id/endpoint-configs` | 管理员 | 列表 |
| POST | `/api/v1/node-servers/:id/endpoint-configs` | 管理员 | 创建 |
| DELETE | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | 管理员 | 删除 |
| GET | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | 管理员 | 详情 |
| PUT | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | 管理员 | 更新 |
| POST | `/api/v1/node-servers/:id/failover` | 管理员 | 触发该节点故障切换 |
| GET | `/api/v1/node-servers/:id/failover-status` | 管理员 | 故障切换状态 |
| POST | `/api/v1/node-servers/:id/force-reconnect` | 管理员 | 强制重连 |
| GET | `/api/v1/node-servers/:id/free-port` | 管理员 | 申请空闲端口 |
| POST | `/api/v1/node-servers/:id/geo-rules/push` | 管理员 | 下发到该节点 |
| GET | `/api/v1/node-servers/:id/inbound-configs` | 管理员 | 列表 |
| POST | `/api/v1/node-servers/:id/inbound-configs` | 管理员 | 创建 |
| DELETE | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | 管理员 | 删除 |
| GET | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | 管理员 | 详情 |
| PUT | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | 管理员 | 更新 |
| GET | `/api/v1/node-servers/:id/inbound-configs/:cfgId/share-links` | 管理员 | 分享链接与片段 |
| POST | `/api/v1/node-servers/:id/inbound-configs/:cfgId/users/:email/pause` | 管理员 | 暂停该用户 |
| POST | `/api/v1/node-servers/:id/inbound-configs/:cfgId/users/:email/resume` | 管理员 | 恢复该用户 |
| POST | `/api/v1/node-servers/:id/inbound-configs/reset-all-traffic` | 管理员 | 重置全部入站流量 |
| GET | `/api/v1/node-servers/:id/install-script` | 管理员 | 获取 Agent 安装脚本 |
| GET | `/api/v1/node-servers/:id/metrics-history` | 管理员 | 获取指标历史 |
| GET | `/api/v1/node-servers/:id/mirror-status` | 管理员 | 镜像状态 |
| POST | `/api/v1/node-servers/:id/nginx/batch-update-ports` | 管理员 | 批量更新端口 |
| POST | `/api/v1/node-servers/:id/nginx/generate-config` | 管理员 | 生成配置 |
| DELETE | `/api/v1/node-servers/:id/nginx/l4-rules` | 管理员 | 删除全部 L4 规则 |
| POST | `/api/v1/node-servers/:id/nginx/save-and-apply` | 管理员 | 保存并应用 |
| GET | `/api/v1/node-servers/:id/nginx/settings` | 管理员 | 读取设置 |
| POST | `/api/v1/node-servers/:id/nginx/settings` | 管理员 | 保存设置 |
| GET | `/api/v1/node-servers/:id/nginx/sites` | 管理员 | 站点列表 |
| POST | `/api/v1/node-servers/:id/nginx/sites` | 管理员 | 创建站点 |
| DELETE | `/api/v1/node-servers/:id/nginx/sites/:siteId` | 管理员 | 删除站点 |
| PUT | `/api/v1/node-servers/:id/nginx/sites/:siteId` | 管理员 | 更新站点 |
| DELETE | `/api/v1/node-servers/:id/nginx/sni-rules` | 管理员 | 删除全部 SNI 规则 |
| POST | `/api/v1/node-servers/:id/nginx/sni-rules/toggle` | 管理员 | 开关 SNI 规则 |
| PUT | `/api/v1/node-servers/:id/node-type` | 管理员 | 更新节点类型 |
| GET | `/api/v1/node-servers/:id/online-users` | 管理员 | 在线用户 |
| GET | `/api/v1/node-servers/:id/outbound-configs` | 管理员 | 列表 |
| POST | `/api/v1/node-servers/:id/outbound-configs` | 管理员 | 创建 |
| DELETE | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | 管理员 | 删除 |
| GET | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | 管理员 | 详情 |
| PUT | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | 管理员 | 更新 |
| PUT | `/api/v1/node-servers/:id/outbound-configs/reorder` | 管理员 | 排序 |
| GET | `/api/v1/node-servers/:id/port-hop/rules` | 管理员 | 端口跳跃规则预览 |
| GET | `/api/v1/node-servers/:id/protocols` | 管理员 | 协议列表 |
| GET | `/api/v1/node-servers/:id/protocols/:coreType` | 管理员 | 协议配置 |
| PUT | `/api/v1/node-servers/:id/protocols/:coreType` | 管理员 | 保存协议配置 |
| GET | `/api/v1/node-servers/:id/protocols/:coreType/config-file` | 管理员 | 读取核心配置文件 |
| GET | `/api/v1/node-servers/:id/protocols/:coreType/section/:section` | 管理员 | 读取协议段落 |
| PUT | `/api/v1/node-servers/:id/protocols/:coreType/section/:section` | 管理员 | 保存协议段落 |
| POST | `/api/v1/node-servers/:id/protocols/:coreType/validate` | 管理员 | 校验配置（真实核心 dry-run） |
| GET | `/api/v1/node-servers/:id/proxy-status` | 管理员 | 远端代理状态 |
| POST | `/api/v1/node-servers/:id/proxy/batch-update-ports` | 管理员 | 批量更新端口 |
| GET | `/api/v1/node-servers/:id/proxy/config` | 管理员 | 读取配置 |
| PUT | `/api/v1/node-servers/:id/proxy/config` | 管理员 | 更新配置 |
| POST | `/api/v1/node-servers/:id/proxy/control` | 管理员 | 链式代理注册控制 |
| POST | `/api/v1/node-servers/:id/proxy/download` | 管理员 | 下载代理核心版本 |
| POST | `/api/v1/node-servers/:id/proxy/generate-config` | 管理员 | 生成配置 |
| POST | `/api/v1/node-servers/:id/proxy/guardian` | 管理员 | 更新守护配置 |
| POST | `/api/v1/node-servers/:id/proxy/install` | 管理员 | 安装代理 |
| DELETE | `/api/v1/node-servers/:id/proxy/l4-rules` | 管理员 | 删除全部 L4 规则 |
| GET | `/api/v1/node-servers/:id/proxy/l4-settings` | 管理员 | L4 设置 |
| POST | `/api/v1/node-servers/:id/proxy/l4-settings` | 管理员 | 保存 L4 设置 |
| GET | `/api/v1/node-servers/:id/proxy/log-settings` | 管理员 | 日志设置 |
| POST | `/api/v1/node-servers/:id/proxy/log-settings` | 管理员 | 保存日志设置 |
| GET | `/api/v1/node-servers/:id/proxy/nginx-settings` | 管理员 | nginx 设置 |
| POST | `/api/v1/node-servers/:id/proxy/nginx-settings` | 管理员 | 保存 nginx 设置 |
| GET | `/api/v1/node-servers/:id/proxy/releases` | 管理员 | 可用版本列表 |
| POST | `/api/v1/node-servers/:id/proxy/reload` | 管理员 | 重载配置 |
| POST | `/api/v1/node-servers/:id/proxy/restart` | 管理员 | 重启 |
| GET | `/api/v1/node-servers/:id/proxy/sites` | 管理员 | 站点列表 |
| POST | `/api/v1/node-servers/:id/proxy/sites` | 管理员 | 创建站点 |
| DELETE | `/api/v1/node-servers/:id/proxy/sites/:siteId` | 管理员 | 删除站点 |
| PUT | `/api/v1/node-servers/:id/proxy/sites/:siteId` | 管理员 | 更新站点 |
| DELETE | `/api/v1/node-servers/:id/proxy/sni-rules` | 管理员 | 删除全部 SNI 规则 |
| POST | `/api/v1/node-servers/:id/proxy/sni-rules/toggle` | 管理员 | 开关 SNI 规则 |
| GET | `/api/v1/node-servers/:id/proxy/sni-settings` | 管理员 | SNI 设置 |
| POST | `/api/v1/node-servers/:id/proxy/sni-settings` | 管理员 | 保存 SNI 设置 |
| POST | `/api/v1/node-servers/:id/proxy/start` | 管理员 | 启动 |
| GET | `/api/v1/node-servers/:id/proxy/status` | 管理员 | 代理进程状态 |
| POST | `/api/v1/node-servers/:id/proxy/stop` | 管理员 | 停止 |
| POST | `/api/v1/node-servers/:id/proxy/sync` | 管理员 | 同步代理配置 |
| GET | `/api/v1/node-servers/:id/rate-limit` | 管理员 | 读取配置 |
| PUT | `/api/v1/node-servers/:id/rate-limit` | 管理员 | 更新配置 |
| GET | `/api/v1/node-servers/:id/rate-limit/status` | 管理员 | 状态 |
| GET | `/api/v1/node-servers/:id/recover-mode` | 管理员 | 读取节点恢复模式 |
| PUT | `/api/v1/node-servers/:id/recover-mode` | 管理员 | 设置节点恢复模式 |
| POST | `/api/v1/node-servers/:id/revoke-cert` | 管理员 | 吊销 Agent 证书 |
| GET | `/api/v1/node-servers/:id/services` | 管理员 | 列表 |
| POST | `/api/v1/node-servers/:id/services` | 管理员 | 创建 |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId` | 管理员 | 删除 |
| GET | `/api/v1/node-servers/:id/services/:serviceId` | 管理员 | 详情 |
| PUT | `/api/v1/node-servers/:id/services/:serviceId` | 管理员 | 更新 |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId/crash-logs` | 管理员 | 清空崩溃日志 |
| GET | `/api/v1/node-servers/:id/services/:serviceId/crash-logs` | 管理员 | 崩溃日志列表 |
| GET | `/api/v1/node-servers/:id/services/:serviceId/crash-logs/:logId` | 管理员 | 崩溃日志详情 |
| GET | `/api/v1/node-servers/:id/services/:serviceId/endpoints` | 管理员 | 端点列表 |
| POST | `/api/v1/node-servers/:id/services/:serviceId/endpoints` | 管理员 | 创建端点 |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId/endpoints/:endpointId` | 管理员 | 删除端点 |
| PUT | `/api/v1/node-servers/:id/services/:serviceId/endpoints/:endpointId` | 管理员 | 更新端点 |
| PUT | `/api/v1/node-servers/:id/services/:serviceId/guardian` | 管理员 | 更新守护配置 |
| GET | `/api/v1/node-servers/:id/services/:serviceId/process-status` | 管理员 | 获取处理状态 |
| POST | `/api/v1/node-servers/:id/services/:serviceId/restart` | 管理员 | 重启 |
| POST | `/api/v1/node-servers/:id/services/:serviceId/start` | 管理员 | 启动 |
| POST | `/api/v1/node-servers/:id/services/:serviceId/stop` | 管理员 | 停止 |
| GET | `/api/v1/node-servers/:id/services/process-status` | 管理员 | 全部进程状态 |
| GET | `/api/v1/node-servers/:id/share-domains` | 管理员 | 该节点的分享域名 |
| POST | `/api/v1/node-servers/:id/share-domains` | 管理员 | 创建 |
| DELETE | `/api/v1/node-servers/:id/share-domains/:domainId` | 管理员 | 删除 |
| PUT | `/api/v1/node-servers/:id/share-domains/:domainId` | 管理员 | 更新 |
| DELETE | `/api/v1/node-servers/:id/share-domains/all` | 管理员 | 清空全部 |
| POST | `/api/v1/node-servers/:id/singbox/control/firewall/apply` | 管理员 | 应用端口跳跃防火墙规则 |
| POST | `/api/v1/node-servers/:id/singbox/control/firewall/remove` | 管理员 | 移除端口跳跃防火墙规则 |
| POST | `/api/v1/node-servers/:id/speed-test` | 管理员 | 节点测速 |
| GET | `/api/v1/node-servers/:id/status` | 管理员 | 节点状态 |
| POST | `/api/v1/node-servers/:id/sync` | 管理员 | 同步节点配置（生成并下发） |
| POST | `/api/v1/node-servers/:id/sync-data` | 超级管理员 | 向节点推送面板数据（用户/订阅/套餐等全量同步） |
| POST | `/api/v1/node-servers/:id/sync-mirror` | 管理员 | 同步镜像节点 |
| GET | `/api/v1/node-servers/:id/tags` | 管理员 | 标签列表 |
| POST | `/api/v1/node-servers/:id/tags` | 管理员 | 添加标签 |
| DELETE | `/api/v1/node-servers/:id/tags/:tagKey` | 管理员 | 删除标签 |
| GET | `/api/v1/node-servers/:id/traffic-history` | 管理员 | 流量历史 |
| POST | `/api/v1/node-servers/:id/traffic-resume` | 管理员 | 恢复因超额暂停的流量 |
| GET | `/api/v1/node-servers/:id/traffic-usage` | 管理员 | 流量用量 |
| POST | `/api/v1/node-servers/:id/update-agent` | 管理员 | 升级 Agent |
| GET | `/api/v1/node-servers/:id/used-server-names` | 管理员 | 已占用的 REALITY SNI 列表 |
| GET | `/api/v1/node-servers/:id/waf/config` | 管理员 | 读取配置 |
| PUT | `/api/v1/node-servers/:id/waf/config` | 管理员 | 保存配置 |
| POST | `/api/v1/node-servers/:id/waf/disable-domain` | 管理员 | 停用域名防护 |
| GET | `/api/v1/node-servers/:id/waf/domain-config` | 管理员 | 域名配置列表 |
| PUT | `/api/v1/node-servers/:id/waf/domain-config` | 管理员 | 批量保存域名配置 |
| DELETE | `/api/v1/node-servers/:id/waf/domain-config/:domain` | 管理员 | 删除域名配置 |
| GET | `/api/v1/node-servers/:id/waf/domain-config/:domain` | 管理员 | 域名配置详情 |
| PUT | `/api/v1/node-servers/:id/waf/domain-config/:domain` | 管理员 | 保存域名配置 |
| GET | `/api/v1/node-servers/:id/waf/domain-rules` | 管理员 | 域名规则目录 |
| GET | `/api/v1/node-servers/:id/waf/domain-rules/:domain/:name` | 管理员 | 读取域名规则文件 |
| PUT | `/api/v1/node-servers/:id/waf/domain-rules/:domain/:name` | 管理员 | 保存域名规则文件 |
| GET | `/api/v1/node-servers/:id/waf/domain-status` | 管理员 | 域名防护状态 |
| POST | `/api/v1/node-servers/:id/waf/enable-domain` | 管理员 | 启用域名防护 |
| POST | `/api/v1/node-servers/:id/waf/push` | 管理员 | 下发到该节点 |
| GET | `/api/v1/node-servers/:id/waf/rules` | 管理员 | 节点 WAF 规则列表 |
| DELETE | `/api/v1/node-servers/:id/waf/rules/:name` | 管理员 | 删除节点 WAF 规则 |
| GET | `/api/v1/node-servers/:id/waf/rules/:name` | 管理员 | 节点 WAF 规则详情 |
| PUT | `/api/v1/node-servers/:id/waf/rules/:name` | 管理员 | 保存节点 WAF 规则 |
| GET | `/api/v1/node-servers/:id/waf/status` | 管理员 | 节点 WAF 状态 |
| PUT | `/api/v1/node-servers/:id/waf/status` | 管理员 | 开关节点 WAF |
| POST | `/api/v1/node-servers/:id/waf/sync` | 管理员 | 同步到该节点 |
| GET | `/api/v1/node-servers/:id/warp` | 管理员 | 列表 |
| DELETE | `/api/v1/node-servers/:id/warp/:warpId` | 管理员 | 删除 |
| GET | `/api/v1/node-servers/:id/warp/config` | 管理员 | 读取配置 |
| POST | `/api/v1/node-servers/:id/warp/generate-config` | 管理员 | 生成配置 |
| POST | `/api/v1/node-servers/:id/warp/register` | 管理员 | 注册 |
| GET | `/api/v1/node-servers/:id/warp/status` | 管理员 | 状态 |
| GET | `/api/v1/node-servers/:id/xray/control/balancer/:tag` | 管理员 | 获取均衡器信息 |
| POST | `/api/v1/node-servers/:id/xray/control/balancer/:tag` | 管理员 | 设置均衡器目标 |
| POST | `/api/v1/node-servers/:id/xray/control/firewall/apply` | 管理员 | 应用端口跳跃防火墙规则 |
| POST | `/api/v1/node-servers/:id/xray/control/firewall/remove` | 管理员 | 移除端口跳跃防火墙规则 |
| POST | `/api/v1/node-servers/:id/xray/control/inbound` | 管理员 | 新增入站 |
| DELETE | `/api/v1/node-servers/:id/xray/control/inbound/:tag` | 管理员 | 移除入站 |
| POST | `/api/v1/node-servers/:id/xray/control/inbound/:tag/user` | 管理员 | 新增用户 |
| DELETE | `/api/v1/node-servers/:id/xray/control/inbound/:tag/user/:email` | 管理员 | 移除用户 |
| POST | `/api/v1/node-servers/:id/xray/control/outbound` | 管理员 | 新增出站 |
| DELETE | `/api/v1/node-servers/:id/xray/control/outbound/:tag` | 管理员 | 移除出站 |
| POST | `/api/v1/node-servers/:id/xray/control/routing` | 管理员 | 申请路由 |
| POST | `/api/v1/node-servers/:id/xray/control/routing/rule` | 管理员 | 新增路由规则 |
| DELETE | `/api/v1/node-servers/:id/xray/control/routing/rule/:tag` | 管理员 | 移除路由规则 |
| GET | `/api/v1/node-servers/:id/xray/control/routing/rules` | 管理员 | 列表路由规则 |
| POST | `/api/v1/node-servers/:id/xray/control/sync` | 管理员 | 手动触发 xray 配置同步下发 |
| GET | `/api/v1/node-servers/:id/xray/stats` | 管理员 | 获取统计 |
| POST | `/api/v1/node-servers/:id/xray/stats/reset/:email` | 管理员 | 重置用户统计 |
| GET | `/api/v1/node-servers/:id/xray/stats/users` | 管理员 | 获取用户统计 |
| GET | `/api/v1/node-servers/billable` | 管理员 | 可计费节点列表 |
| GET | `/api/v1/node-servers/ca-cert` | 管理员 | CA 证书 |
| GET | `/api/v1/node-servers/ca-status` | 管理员 | CA 状态 |
| GET | `/api/v1/node-servers/dashboard` | 管理员 | 仪表盘状态 |
| POST | `/api/v1/node-servers/distribute-cert` | 管理员 | 向全部节点分发证书 |
| POST | `/api/v1/node-servers/failover-check` | 管理员 | 触发故障切换检查 |
| GET | `/api/v1/node-servers/metrics/aggregated` | 管理员 | 聚合指标 |
| POST | `/api/v1/node-servers/migrate-cert` | 管理员 | 迁移证书 |
| GET | `/api/v1/node-servers/online-summary` | 管理员 | 在线汇总 |
| GET | `/api/v1/node-servers/online-users` | 管理员 | 全部在线用户 |
| GET | `/api/v1/node-servers/pending-agent-updates` | 管理员 | 待升级的 Agent |
| POST | `/api/v1/node-servers/recommend-region` | 管理员 | 推荐地区 |
| POST | `/api/v1/node-servers/select-nodes` | 管理员 | 为订阅挑选节点 |
| POST | `/api/v1/node-servers/speed-test-all` | 管理员 | 全部节点测速 |
| GET | `/api/v1/node-servers/speed-test-results` | 管理员 | 测速结果 |
| GET | `/api/v1/node-servers/traffic/stats` | 管理员 | 流量统计 |
| POST | `/api/v1/node-servers/update-all-agents` | 管理员 | 升级全部 Agent |
| DELETE | `/api/v1/node-servers/users/:client_id` | 管理员 | 同步删除某客户 |
| GET | `/api/v1/node-servers/users/retry-queue` | 管理员 | 重试队列状态 |
| POST | `/api/v1/node-servers/users/retry-queue/process` | 管理员 | 处理重试队列 |
| POST | `/api/v1/node-servers/users/sync` | 管理员 | 同步单个客户 |
| POST | `/api/v1/node-servers/validate-chain-config` | 管理员 | 校验链式配置 |

### 节点审计日志

节点侧操作审计与导出。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/v1/node-audit-logs` | 登录 | 节点审计日志 |
| GET | `/api/v1/node-audit-logs/export` | 登录 | 导出节点审计日志 |

### 节点（旧版模型）

旧版 node 概型（保留兼容）。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/v1/nodes` | 登录 | 列表 |
| POST | `/api/v1/nodes` | 登录 | 创建 |
| DELETE | `/api/v1/nodes/:id` | 登录 | 删除 |
| GET | `/api/v1/nodes/:id` | 登录 | 详情 |
| PUT | `/api/v1/nodes/:id` | 登录 | 更新 |
| GET | `/api/v1/nodes/:id/online-users` | 登录 | 在线用户 |

### 地区总览

按地区聚合的节点视图。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/v1/regions/:region/servers` | 登录 | 该地区节点列表 |
| GET | `/api/v1/regions/overview` | 登录 | 地区总览 |

### 订阅

订阅记录管理：吊销、轮换 token、访问日志、用量重置。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/v1/subscriptions` | 登录 | 列表 |
| POST | `/api/v1/subscriptions` | 登录 | 创建 |
| DELETE | `/api/v1/subscriptions/:id` | 登录 | 删除 |
| GET | `/api/v1/subscriptions/:id` | 登录 | 详情 |
| PUT | `/api/v1/subscriptions/:id` | 登录 | 更新 |
| GET | `/api/v1/subscriptions/:id/access-logs` | 登录 | 访问日志 |
| POST | `/api/v1/subscriptions/:id/enable` | 登录 | 启用 |
| POST | `/api/v1/subscriptions/:id/reset-usage` | 登录 | 重置用量 |
| POST | `/api/v1/subscriptions/:id/restore` | 登录 | 恢复 |
| POST | `/api/v1/subscriptions/:id/revoke` | 登录 | 吊销 |
| POST | `/api/v1/subscriptions/:id/rotate` | 登录 | 轮换 |
| GET | `/api/v1/subscriptions/:id/rotations` | 登录 | 轮换历史 |

### 订阅格式

订阅输出格式（mihomo_yaml / singbox_json / xray_json / raw_uri…）及其规则。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/v1/subscription-formats` | 登录 | 列表 |
| POST | `/api/v1/subscription-formats` | 登录 | 创建 |
| DELETE | `/api/v1/subscription-formats/:id` | 登录 | 删除 |
| GET | `/api/v1/subscription-formats/:id` | 登录 | 详情 |
| PUT | `/api/v1/subscription-formats/:id` | 登录 | 更新 |
| GET | `/api/v1/subscription-formats/:id/rules` | 登录 | 规则列表 |
| POST | `/api/v1/subscription-formats/:id/rules` | 登录 | 创建规则 |
| DELETE | `/api/v1/subscription-formats/:id/rules/:rid` | 登录 | 删除规则 |
| PUT | `/api/v1/subscription-formats/:id/rules/:rid` | 登录 | 更新规则 |
| POST | `/api/v1/subscription-formats/:id/rules/sort` | 登录 | 规则排序 |

### 订阅接入点

独立订阅域名（反代回面板的 /sub 路径）。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/v1/subscription-endpoints` | 登录 | 列表 |
| POST | `/api/v1/subscription-endpoints` | 登录 | 创建 |
| DELETE | `/api/v1/subscription-endpoints/:id` | 登录 | 删除 |
| GET | `/api/v1/subscription-endpoints/:id` | 登录 | 详情 |
| PUT | `/api/v1/subscription-endpoints/:id` | 登录 | 更新 |

### 分享域名

客户端链接使用的接入域名/IP 池。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/v1/share-domains` | 登录 | 列表 |
| POST | `/api/v1/share-domains` | 登录 | 创建 |
| DELETE | `/api/v1/share-domains/:id` | 登录 | 删除 |
| PUT | `/api/v1/share-domains/:id` | 登录 | 更新 |
| DELETE | `/api/v1/share-domains/all` | 登录 | 清空全部 |
| POST | `/api/v1/share-domains/seed` | 登录 | 初始化默认数据 |

### Cloudflare IP

CF CDN IP 池与来源管理。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| DELETE | `/api/v1/cfip/entries` | 登录 | 清空条目 |
| GET | `/api/v1/cfip/entries` | 登录 | 条目列表 |
| POST | `/api/v1/cfip/entries` | 登录 | 新增条目 |
| DELETE | `/api/v1/cfip/entries/:id` | 登录 | 删除条目 |
| POST | `/api/v1/cfip/fetch-all` | 登录 | 拉取全部来源 |
| POST | `/api/v1/cfip/seed` | 登录 | 初始化默认数据 |
| GET | `/api/v1/cfip/sources` | 登录 | 数据源列表 |
| POST | `/api/v1/cfip/sources` | 登录 | 创建数据源 |
| DELETE | `/api/v1/cfip/sources/:id` | 登录 | 删除数据源 |
| PUT | `/api/v1/cfip/sources/:id` | 登录 | 更新数据源 |
| POST | `/api/v1/cfip/sources/:id/fetch` | 登录 | 拉取该来源 |

### 订阅配置（Mihomo/Clash）

订阅模板配置：分组、规则、rule-providers、YAML 预览。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/v1/mihomo/configs` | 登录 | 列表 |
| POST | `/api/v1/mihomo/configs` | 登录 | 创建 |
| DELETE | `/api/v1/mihomo/configs/:id` | 登录 | 删除 |
| GET | `/api/v1/mihomo/configs/:id` | 登录 | 详情 |
| PUT | `/api/v1/mihomo/configs/:id` | 登录 | 更新 |
| GET | `/api/v1/mihomo/configs/:id/proxy-groups` | 登录 | 分组列表 |
| POST | `/api/v1/mihomo/configs/:id/proxy-groups` | 登录 | 创建分组 |
| GET | `/api/v1/mihomo/configs/:id/rule-providers` | 登录 | 规则提供者列表 |
| POST | `/api/v1/mihomo/configs/:id/rule-providers` | 登录 | 创建规则提供者 |
| GET | `/api/v1/mihomo/configs/:id/rules` | 登录 | 规则列表 |
| POST | `/api/v1/mihomo/configs/:id/rules` | 登录 | 创建规则 |
| GET | `/api/v1/mihomo/configs/:id/yaml` | 登录 | 预览生成的 YAML |
| DELETE | `/api/v1/mihomo/proxy-groups/:id` | 登录 | 删除分组 |
| DELETE | `/api/v1/mihomo/rule-providers/:id` | 登录 | 删除规则提供者 |
| DELETE | `/api/v1/mihomo/rules/:id` | 登录 | 删除规则 |

### 充值站点

第三方支付站点接入：签名密钥、验签文件、域名验证、对接文档。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/v1/charge-sites` | 登录 | 站点列表 |
| POST | `/api/v1/charge-sites` | 登录 | 创建站点 |
| DELETE | `/api/v1/charge-sites/:id` | 登录 | 删除站点 |
| GET | `/api/v1/charge-sites/:id` | 登录 | 站点详情 |
| PUT | `/api/v1/charge-sites/:id` | 登录 | 更新站点 |
| POST | `/api/v1/charge-sites/:id/clear-prev-secret` | 登录 | 清除旧密钥 |
| POST | `/api/v1/charge-sites/:id/generate-verify` | 登录 | 生成验签文件 |
| POST | `/api/v1/charge-sites/:id/generate-verify-file` | 登录 | 生成验签文件 |
| GET | `/api/v1/charge-sites/:id/integration-doc` | 登录 | 获取对接文档 |
| POST | `/api/v1/charge-sites/:id/rotate-secret` | 登录 | 轮换签名密钥 |
| POST | `/api/v1/charge-sites/:id/verify-dns` | 登录 | 验证域名 DNS |
| POST | `/api/v1/charge-sites/:id/verify-domain` | 登录 | 验证域名 HTTP 挑战 |
| POST | `/api/v1/charge-sites/:id/verify-domain-dns` | 登录 | 验证域名 DNS |
| POST | `/api/v1/charge-sites/:id/verify-domain-http` | 登录 | 验证域名 HTTP 挑战 |

### 充值订单

充值订单查询、退款、CSV 导出。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/v1/charge-orders` | 登录 | 订单列表 |
| GET | `/api/v1/charge-orders/:id` | 登录 | 订单详情 |
| POST | `/api/v1/charge-orders/:id/refund` | 登录 | 退款 |
| GET | `/api/v1/charge-orders/export-csv` | 登录 | 导出充值订单 CSV |

### 充值卡密

卡密批量生成、使用记录、CSV 导出。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/v1/charge-cards` | 登录 | 卡密列表 |
| POST | `/api/v1/charge-cards` | 登录 | 创建卡密 |
| DELETE | `/api/v1/charge-cards/:id` | 登录 | 删除卡密 |
| GET | `/api/v1/charge-cards/:id/usages` | 登录 | 卡密使用记录 |
| POST | `/api/v1/charge-cards/batch` | 登录 | 批量生成卡密 |
| GET | `/api/v1/charge-cards/export-csv` | 登录 | 导出卡密 CSV |

### 支付回调事件

回调事件审计。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/v1/charge-callback-events` | 登录 | 回调事件列表 |

### 支付回调重试

失败回调的手动重试。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/v1/charge-callback-retries` | 登录 | 失败回调列表 |
| POST | `/api/v1/charge-callback-retries/:id/retry` | 管理员 | 重试回调 |

### 客户余额

余额与流水的管理端视图。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/v1/client-balances` | 登录 | 客户余额列表 |
| GET | `/api/v1/client-balances/transactions` | 登录 | 余额流水（管理端） |
| GET | `/api/v1/client-balances/transactions/export-csv` | 登录 | 导出余额流水 CSV |

### 礼品

可领取礼品（天数/流量）的发放。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/v1/client-gifts` | 登录 | 礼品列表 |
| POST | `/api/v1/client-gifts` | 登录 | 创建礼品 |
| POST | `/api/v1/client-gifts/batch` | 登录 | 批量创建礼品 |
| GET | `/api/v1/client-gifts/search-clients` | 登录 | 搜索客户 |

### 推广配置

推广返佣配置与统计。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/v1/referral/config` | 登录 | 推广配置 |
| PUT | `/api/v1/referral/config` | 管理员 | 保存推广配置 |
| GET | `/api/v1/referral/records` | 登录 | 推广记录列表 |
| GET | `/api/v1/referral/stats` | 登录 | 推广统计 |

### 推广提现

提现申请的审核。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/v1/referral-withdrawals` | 登录 | 提现申请列表 |
| POST | `/api/v1/referral-withdrawals/:id/approve` | 登录 | 批准提现 |
| POST | `/api/v1/referral-withdrawals/:id/reject` | 登录 | 拒绝提现 |

### IP 白名单（汇总）

全部客户的 IP 白名单视图。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/v1/ip-whitelists` | 登录 | 全部客户 IP 白名单 |

### 客户 OAuth 绑定

客户第三方账号绑定查询。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/v1/client-oauth-bindings` | 登录 | 客户 OAuth 绑定列表 |

### 工单（管理端）

工单回复、指派、关闭。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/v1/tickets` | 登录 | 工单列表 |
| GET | `/api/v1/tickets/:id` | 登录 | 工单详情 |
| POST | `/api/v1/tickets/:id/assign` | 登录 | 指派工单 |
| POST | `/api/v1/tickets/:id/close` | 登录 | 关闭工单 |
| POST | `/api/v1/tickets/:id/reply` | 登录 | 回复工单 |

### 文章

门户文章的增删改查。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/v1/articles` | 登录 | 文章列表 |
| POST | `/api/v1/articles` | 登录 | 创建文章 |
| DELETE | `/api/v1/articles/:id` | 登录 | 删除文章 |
| GET | `/api/v1/articles/:id` | 登录 | 文章详情 |
| PUT | `/api/v1/articles/:id` | 登录 | 更新文章 |

### 共享账号

流媒体等共享账号的发放与修复记录。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/v1/shared-accounts` | 登录 | 共享账号列表 |
| POST | `/api/v1/shared-accounts` | 登录 | 创建共享账号 |
| DELETE | `/api/v1/shared-accounts/:id` | 登录 | 删除共享账号 |
| GET | `/api/v1/shared-accounts/:id` | 登录 | 共享账号详情 |
| PUT | `/api/v1/shared-accounts/:id` | 登录 | 更新共享账号 |
| GET | `/api/v1/shared-accounts/:id/repair-logs` | 登录 | 修复记录 |

### 证书

公信证书（ACME）签发、上传、续期、导出；支持指定节点签发。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/v1/certificates` | 登录 | 列表 |
| POST | `/api/v1/certificates` | 登录 | 创建 |
| DELETE | `/api/v1/certificates/:id` | 登录 | 删除 |
| GET | `/api/v1/certificates/:id` | 登录 | 详情 |
| PUT | `/api/v1/certificates/:id` | 登录 | 更新 |
| GET | `/api/v1/certificates/:id/download` | 登录 | 下载 |
| POST | `/api/v1/certificates/:id/renew` | 登录 | 客户续费 |
| GET | `/api/v1/certificates/check-port` | 登录 | 检查端口 |
| GET | `/api/v1/certificates/detect-ip` | 登录 | 探测IP |
| GET | `/api/v1/certificates/export` | 登录 | 导出 |
| POST | `/api/v1/certificates/issue` | 登录 | 签发。body：`{"domains":["example.com","*.example.com"],"challenge_type":"dns-01","dns_channel_id":2,"key_type":"rsa","key_bits":4096}`。`challenge_type` 缺省 http-01；`dns_channel_id` 指定 DNS 通道（面板配置好的 DNS 提供者+凭据） |
| POST | `/api/v1/certificates/issue-on-node` | 登录 | 签发到节点 |
| POST | `/api/v1/certificates/upload` | 登录 | 上传 |

### 证书通道

ACME 通道（CA）配置。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/v1/cert-channels` | 登录 | 列表 |
| POST | `/api/v1/cert-channels` | 登录 | 创建 |
| DELETE | `/api/v1/cert-channels/:id` | 登录 | 删除 |
| PUT | `/api/v1/cert-channels/:id` | 登录 | 更新 |
| GET | `/api/v1/cert-channels/providers` | 登录 | 提供商 |

### EAB 配置

ZeroSSL / Google 等外部账号绑定凭据。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/v1/eab-configs` | 登录 | 列表 |
| POST | `/api/v1/eab-configs` | 登录 | 创建 |
| DELETE | `/api/v1/eab-configs/:id` | 登录 | 删除 |
| PUT | `/api/v1/eab-configs/:id` | 登录 | 更新 |
| POST | `/api/v1/eab-configs/fetch-google` | 登录 | 从 GCP 获取 Google EAB |
| POST | `/api/v1/eab-configs/fetch-zerossl` | 登录 | 用邮箱一键获取 ZeroSSL EAB |

### ACME 账号

面板侧 ACME 账号（联系邮箱）管理。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/v1/acme-accounts` | 登录 | 列表 |
| POST | `/api/v1/acme-accounts` | 登录 | 创建 |
| DELETE | `/api/v1/acme-accounts/:id` | 登录 | 删除 |
| PUT | `/api/v1/acme-accounts/:id` | 登录 | 更新 |
| GET | `/api/v1/acme-accounts/default` | 登录 | 获取默认 |

### 内置 CA

面板内部 CA：Agent 证书签发/续期/吊销、CRL 导出、分发队列。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/v1/ca/agent-certs` | 登录 | Agent 证书列表 |
| POST | `/api/v1/ca/agent-certs/:server_id/renew` | 登录 | 续期 Agent 证书 |
| GET | `/api/v1/ca/distribution-queue` | 登录 | 证书分发队列 |
| DELETE | `/api/v1/ca/distribution-queue/:id` | 登录 | 删除分发队列条目 |
| POST | `/api/v1/ca/distribution-queue/:id/retry` | 登录 | 重试证书分发 |
| GET | `/api/v1/ca/revocations` | 登录 | 吊销列表 |
| GET | `/api/v1/ca/revocations/export` | 登录 | 导出 CRL |
| GET | `/api/v1/ca/status` | 登录 | CA 状态 |

### Geo 规则

geoip/geosite 数据源与文件的同步、下发。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/v1/geo-rules/files` | 登录 | 文件列表 |
| DELETE | `/api/v1/geo-rules/files/:fileId` | 登录 | 删除文件 |
| PUT | `/api/v1/geo-rules/files/:fileId/auto-sync` | 登录 | 开关自动同步 |
| POST | `/api/v1/geo-rules/files/:fileId/download` | 登录 | 下载 |
| GET | `/api/v1/geo-rules/files/:fileId/download-file` | 登录 | 下载文件 |
| GET | `/api/v1/geo-rules/github-assets` | 登录 | 拉取 GitHub 仓库资产 |
| POST | `/api/v1/geo-rules/push-all` | 登录 | 下发到全部节点 |
| POST | `/api/v1/geo-rules/seed` | 登录 | 初始化默认数据 |
| GET | `/api/v1/geo-rules/sources` | 登录 | 数据源列表 |
| POST | `/api/v1/geo-rules/sources` | 登录 | 创建数据源 |
| DELETE | `/api/v1/geo-rules/sources/:sourceId` | 登录 | 删除数据源 |
| PUT | `/api/v1/geo-rules/sources/:sourceId` | 登录 | 更新数据源 |
| GET | `/api/v1/geo-rules/sources/:sourceId/assets` | 登录 | 拉取 Release 资产 |
| POST | `/api/v1/geo-rules/sources/:sourceId/files` | 登录 | 添加文件 |
| PUT | `/api/v1/geo-rules/sources/:sourceId/schedule` | 登录 | 开关定时同步 |
| POST | `/api/v1/geo-rules/sources/:sourceId/select` | 登录 | 选择文件 |
| POST | `/api/v1/geo-rules/sources/:sourceId/sync` | 登录 | 同步该数据源 |
| GET | `/api/v1/geo-rules/status` | 登录 | 状态 |
| POST | `/api/v1/geo-rules/sync-all` | 登录 | 同步全部 |
| GET | `/api/v1/geo-rules/sync-status/:key` | 登录 | 同步进度 |

### WAF 规则库

全局 WAF 规则：GitHub 源、同步、下发到全部节点。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/v1/waf/github-source` | 登录 | GitHub 数据源 |
| PUT | `/api/v1/waf/github-source` | 登录 | 保存 GitHub 数据源 |
| POST | `/api/v1/waf/push-all` | 登录 | 下发到全部节点 |
| GET | `/api/v1/waf/rules` | 登录 | 规则列表 |
| DELETE | `/api/v1/waf/rules/:name` | 登录 | 删除规则 |
| GET | `/api/v1/waf/rules/:name` | 登录 | 获取规则 |
| PUT | `/api/v1/waf/rules/:name` | 登录 | 保存规则 |
| POST | `/api/v1/waf/sync-all` | 登录 | 同步全部 |
| POST | `/api/v1/waf/sync-github` | 登录 | 从 GitHub 同步 |

### 日志管理

节点/核心日志读取与流式跟随、清理策略、日志写入器控制。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| POST | `/api/v1/log-manage/cleanup-all` | 登录 | 立即全量清理 |
| GET | `/api/v1/log-manage/cleanup-rules` | 登录 | 清理规则列表 |
| PUT | `/api/v1/log-manage/cleanup-rules/:id` | 登录 | 更新清理规则 |
| POST | `/api/v1/log-manage/cleanup-table/:table` | 登录 | 立即清理指定表 |
| GET | `/api/v1/log-manage/core-log-output` | 登录 | 核心日志输出配置 |
| PUT | `/api/v1/log-manage/core-log-output` | 登录 | 保存核心日志输出配置 |
| GET | `/api/v1/log-manage/core/:type/stdout-log` | 登录 | 读取核心 stdout 日志 |
| GET | `/api/v1/log-manage/core/:type/stdout-stream` | 登录 | 流式跟随核心 stdout |
| POST | `/api/v1/log-manage/discover` | 登录 | 发现节点日志文件 |
| GET | `/api/v1/log-manage/log-output` | 登录 | 日志输出配置 |
| PUT | `/api/v1/log-manage/log-output` | 登录 | 保存日志输出配置 |
| POST | `/api/v1/log-manage/log-output/release/:type` | 登录 | 释放日志 stdout 接管 |
| POST | `/api/v1/log-manage/log-output/takeover/:type` | 登录 | 接管日志 stdout |
| GET | `/api/v1/log-manage/nodes/:serverId/logs` | 登录 | 节点日志文件列表 |
| GET | `/api/v1/log-manage/nodes/:serverId/logs/:type` | 登录 | 读取节点日志 |
| POST | `/api/v1/log-manage/nodes/:serverId/logs/:type/rotate` | 登录 | 轮转节点日志 |
| GET | `/api/v1/log-manage/nodes/:serverId/logs/:type/stream` | 登录 | 流式跟随节点日志 |
| GET | `/api/v1/log-manage/policies` | 登录 | 策略列表 |
| POST | `/api/v1/log-manage/policies` | 登录 | 创建策略 |
| DELETE | `/api/v1/log-manage/policies/:id` | 登录 | 删除策略 |
| GET | `/api/v1/log-manage/policies/:id` | 登录 | 策略详情 |
| PUT | `/api/v1/log-manage/policies/:id` | 登录 | 更新策略 |
| GET | `/api/v1/log-manage/policies/:id/jobs` | 登录 | 清理任务列表 |
| GET | `/api/v1/log-manage/policies/:id/log` | 登录 | 读取日志 |
| GET | `/api/v1/log-manage/policies/:id/log-stream` | 登录 | 流式跟随日志 |
| POST | `/api/v1/log-manage/policies/:id/rotate` | 登录 | 轮换 |
| GET | `/api/v1/log-manage/proxy/:type/stdout-log` | 登录 | 读取核心 stdout 日志 |
| GET | `/api/v1/log-manage/proxy/:type/stdout-stream` | 登录 | 流式跟随核心 stdout |
| GET | `/api/v1/log-manage/scheduler-config` | 登录 | 调度配置 |
| PUT | `/api/v1/log-manage/scheduler-config` | 登录 | 更新调度配置 |
| GET | `/api/v1/log-manage/status` | 登录 | 状态 |
| POST | `/api/v1/log-manage/sync` | 登录 | 同步清理策略 |
| GET | `/api/v1/log-manage/writers` | 登录 | 写入器状态 |
| PUT | `/api/v1/log-manage/writers/:type/config` | 登录 | 更新写入器配置 |
| GET | `/api/v1/log-manage/writers/:type/files` | 登录 | 写入器日志文件列表 |
| POST | `/api/v1/log-manage/writers/:type/start` | 登录 | 启动写入器 |
| POST | `/api/v1/log-manage/writers/:type/stop` | 登录 | 停止写入器 |

### 备份

数据库/全量备份、恢复、定时任务、FTP/S3/WebDAV 远端存储。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| DELETE | `/api/v1/backups/:id` | 管理员 | 删除备份 |
| GET | `/api/v1/backups/:id/download` | 管理员 | 下载备份 |
| POST | `/api/v1/backups/:id/restore` | 管理员 | 从备份恢复 |
| POST | `/api/v1/backups/:id/upload-ftp` | 管理员 | 上传到 FTP |
| POST | `/api/v1/backups/:id/upload-s3` | 管理员 | 上传到 S3 |
| POST | `/api/v1/backups/:id/upload-webdav` | 管理员 | 上传到 WebDAV |
| POST | `/api/v1/backups/cleanup-jobs` | 管理员 | 创建清理任务 |
| POST | `/api/v1/backups/create` | 管理员 | 创建全量备份 |
| GET | `/api/v1/backups/database` | 管理员 | 数据库备份列表 |
| POST | `/api/v1/backups/database` | 管理员 | 创建数据库备份 |
| POST | `/api/v1/backups/database/restore` | 管理员 | 从数据库备份恢复 |
| GET | `/api/v1/backups/list` | 管理员 | 全量备份列表 |
| POST | `/api/v1/backups/remote-restore` | 管理员 | 从远端恢复 |
| GET | `/api/v1/backups/restore-status` | 管理员 | 恢复进度 |
| GET | `/api/v1/backups/schedules` | 管理员 | 定时任务列表 |
| POST | `/api/v1/backups/schedules` | 管理员 | 创建定时任务 |
| DELETE | `/api/v1/backups/schedules/:id` | 管理员 | 删除定时任务 |
| PUT | `/api/v1/backups/schedules/:id` | 管理员 | 更新定时任务 |
| GET | `/api/v1/backups/storages` | 管理员 | 存储配置列表 |
| POST | `/api/v1/backups/storages` | 管理员 | 创建存储配置 |
| DELETE | `/api/v1/backups/storages/:id` | 管理员 | 删除存储配置 |
| PUT | `/api/v1/backups/storages/:id` | 管理员 | 更新存储配置 |
| GET | `/api/v1/backups/storages/:id/files` | 管理员 | 远端备份文件列表 |
| POST | `/api/v1/backups/upload-restore` | 管理员 | 上传备份并恢复 |

### 系统设置

数据库配置与迁移、面板设置、JWT 密钥、SMTP/Telegram 测试、通知模板、在线升级等（多数需管理员）。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| PUT | `/api/v1/system/bootstrap-finish` | 管理员 | 完成首次引导 |
| GET | `/api/v1/system/bootstrap-state` | 管理员 | 首次引导状态 |
| GET | `/api/v1/system/config-file/:coreType` | 管理员 | 读取核心配置文件 |
| GET | `/api/v1/system/db-config` | 管理员 | 数据库配置 |
| PUT | `/api/v1/system/db-config` | 管理员 | 更新数据库配置 |
| POST | `/api/v1/system/db-config/migrate` | 管理员 | 迁移数据库数据 |
| POST | `/api/v1/system/db-config/switch` | 管理员 | 切换数据库（含一步迁移） |
| POST | `/api/v1/system/db-config/test` | 管理员 | 测试数据库连接 |
| GET | `/api/v1/system/db-pool-stats` | 管理员 | 连接池统计 |
| GET | `/api/v1/system/db-stats` | 管理员 | 数据库统计 |
| GET | `/api/v1/system/db-stats/ping` | 管理员 | 数据库探测 |
| POST | `/api/v1/system/db-stats/reset` | 管理员 | 重置数据库统计 |
| GET | `/api/v1/system/encryption-key-status` | 管理员 | 加密密钥状态 |
| GET | `/api/v1/system/env-config` | 管理员 | .env 配置 |
| PUT | `/api/v1/system/env-config` | 管理员 | 更新 .env 配置 |
| POST | `/api/v1/system/generate-encryption-key` | 管理员 | 生成加密密钥 |
| GET | `/api/v1/system/github-config` | 管理员 | GitHub 配置 |
| PUT | `/api/v1/system/github-config` | 管理员 | 更新 GitHub 配置 |
| GET | `/api/v1/system/jwt-secret-status` | 管理员 | JWT 密钥状态 |
| GET | `/api/v1/system/maintenance` | 公开 | 维护模式状态（公开） |
| PUT | `/api/v1/system/maintenance` | 管理员 | 切换维护模式 |
| POST | `/api/v1/system/migrate-data` | 管理员 | 迁移数据 |
| GET | `/api/v1/system/notification-config` | 管理员 | 通知配置 |
| PUT | `/api/v1/system/notification-config` | 管理员 | 更新通知配置 |
| GET | `/api/v1/system/notification-templates` | 管理员 | 模板列表 |
| POST | `/api/v1/system/notification-templates` | 管理员 | 创建模板 |
| DELETE | `/api/v1/system/notification-templates/:id` | 管理员 | 删除模板 |
| PUT | `/api/v1/system/notification-templates/:id` | 管理员 | 更新模板 |
| POST | `/api/v1/system/notification-templates/:id/broadcast` | 管理员 | 按模板群发通知 |
| POST | `/api/v1/system/notification-templates/:id/send` | 管理员 | 发送测试通知 |
| GET | `/api/v1/system/panel-config` | 管理员 | 面板配置 |
| PUT | `/api/v1/system/panel-config` | 管理员 | 更新面板配置 |
| GET | `/api/v1/system/reality-targets` | 管理员 | REALITY 目标列表 |
| PUT | `/api/v1/system/reality-targets` | 管理员 | 更新 REALITY 目标 |
| POST | `/api/v1/system/reality-targets/reset` | 管理员 | 重置 REALITY 目标为默认 |
| POST | `/api/v1/system/regenerate-jwt-secret` | 管理员 | 重新生成 JWT 密钥 |
| GET | `/api/v1/system/releases` | 管理员 | 可用版本列表 |
| POST | `/api/v1/system/restart` | 管理员 | 重启 |
| POST | `/api/v1/system/rollback` | 管理员 | 系统回滚 |
| GET | `/api/v1/system/settings` | 管理员 | 读取设置 |
| PUT | `/api/v1/system/settings/batch` | 管理员 | 批量更新设置 |
| GET | `/api/v1/system/stats` | 管理员 | 系统状态 |
| GET | `/api/v1/system/subscription-billing-templates` | 管理员 | 订阅账单模板 |
| PUT | `/api/v1/system/subscription-billing-templates` | 管理员 | 更新订阅账单模板 |
| POST | `/api/v1/system/telegram-chat-id` | 管理员 | 获取 Telegram Chat ID |
| POST | `/api/v1/system/test-smtp` | 管理员 | 测试 SMTP |
| POST | `/api/v1/system/test-telegram` | 管理员 | 测试 Telegram |
| POST | `/api/v1/system/update` | 管理员 | 系统升级 |
| GET | `/api/v1/system/update-jobs` | 管理员 | 升级任务列表 |
| POST | `/api/v1/system/upload-mtls-ca` | 管理员 | 上传 mTLS CA 证书 |

### 安全审计

审计事件查询与导出。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/v1/security/audit-events` | 登录 | 审计事件列表 |
| GET | `/api/v1/security/audit-events/export` | 登录 | 导出审计事件 |

### 指标

面板聚合指标（节点/证书/服务）。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/v1/metrics/certificates` | 登录 | 证书指标 |
| GET | `/api/v1/metrics/nodes` | 登录 | 节点指标 |
| GET | `/api/v1/metrics/overview` | 登录 | 总览 |
| GET | `/api/v1/metrics/services` | 登录 | 服务指标 |

### 门户模板

门户（Portal）模板与激活配置、缓存管理。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/v1/portal/active` | 登录 | 当前激活模板 |
| PUT | `/api/v1/portal/active` | 登录 | 更新激活模板 |
| DELETE | `/api/v1/portal/cache` | 登录 | 清空门户缓存 |
| GET | `/api/v1/portal/cache` | 登录 | 缓存状态 |
| PUT | `/api/v1/portal/cache/dev-mode` | 登录 | 切换缓存开发模式 |
| POST | `/api/v1/portal/cache/rebuild` | 登录 | 重建门户缓存 |
| POST | `/api/v1/portal/reset` | 登录 | 恢复默认模板 |
| GET | `/api/v1/portal/templates` | 登录 | 列表 |
| POST | `/api/v1/portal/templates` | 登录 | 创建 |
| DELETE | `/api/v1/portal/templates/:id` | 登录 | 删除 |
| GET | `/api/v1/portal/templates/:id` | 登录 | 详情 |
| PUT | `/api/v1/portal/templates/:id` | 登录 | 更新 |
| POST | `/api/v1/portal/templates/:id/activate` | 登录 | 激活模板 |

### Agent 安装

节点 Agent 安装脚本与下载（公开）。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| POST | `/api/v1/agent/bootstrap` | 公开 | Agent 首次引导注册（Agent 凭 bootstrap token 上报，公开） |
| GET | `/api/v1/agent/releases` | 管理员 | Agent 可用版本列表（供升级页选择） |
| GET | `/api/v1/agent/version` | 管理员 | 面板侧 Agent 当前版本 |
| GET | `/api/v1/agent/download` | 公开 | 下载Agent |
| GET | `/api/v1/agent/install` | 公开 | 获取安装脚本 |

### 支付回调入口

第三方支付平台回调（公开、验签）。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| POST | `/api/v1/charge/callback/:site_id` | 公开 | Handle充值回调 |

### 入站（含节点）

套餐选节点用的入站列表。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/v1/inbounds-with-server` | 登录 | 全部入站（含所属节点） |

### 公开配置

无需登录的门户运行时配置（带维护模式守卫 + 公网限流）。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/v1/public/articles` | 公开 | 公开文章列表 |
| GET | `/api/v1/public/articles/:slug` | 公开 | 公开文章详情 |
| GET | `/api/v1/public/client-configs` | 公开 | 客户端下载配置（公开） |
| GET | `/api/v1/public/portal-config` | 公开 | 门户运行时配置（站点名、主题、多语言、合规文档；公开） |
| GET | `/api/v1/public/settings` | 公开 | 公开设置（如 registration_enabled；公开） |
| GET | `/api/v1/public/subscription-endpoints` | 公开 | 启用的订阅接入点列表（公开） |

### 客户认证与业务（管理端引擎上的同一套）

与门户一致的客户接口，完整说明见 portal-api.md。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/v1/client-auth/articles` | 公开 | 列表文章 |
| GET | `/api/v1/client-auth/articles/:slug` | 公开 | 获取文章 |
| GET | `/api/v1/client-auth/balance` | 公开 | 获取余额 |
| GET | `/api/v1/client-auth/balance/transactions` | 公开 | 余额流水 |
| GET | `/api/v1/client-auth/bills` | 公开 | 订单列表 |
| POST | `/api/v1/client-auth/change-password` | 公开 | 修改密码 |
| POST | `/api/v1/client-auth/charge/create` | 公开 | 创建充值订单 |
| GET | `/api/v1/client-auth/charge/orders` | 公开 | 列表充值订单 |
| GET | `/api/v1/client-auth/charge/orders/:id` | 公开 | 获取充值订单详情 |
| POST | `/api/v1/client-auth/charge/redeem-card` | 公开 | 卡密兑换 |
| GET | `/api/v1/client-auth/charge/sites` | 公开 | 列表充值站点 |
| GET | `/api/v1/client-auth/charge/status/:trade_no` | 公开 | 查询充值状态 |
| GET | `/api/v1/client-auth/checktime` | 公开 | 获取服务器时间（校时） |
| GET | `/api/v1/client-auth/connection-info` | 公开 | 获取套餐连接参数 |
| GET | `/api/v1/client-auth/gifts` | 公开 | 列表礼品 |
| POST | `/api/v1/client-auth/gifts/:id/claim` | 公开 | 领取礼品 |
| GET | `/api/v1/client-auth/invoices` | 公开 | 订单列表 |
| GET | `/api/v1/client-auth/invoices/:id` | 公开 | 订单详情 |
| GET | `/api/v1/client-auth/ip-whitelist` | 公开 | 列表IP白名单 |
| DELETE | `/api/v1/client-auth/ip-whitelist/:id` | 公开 | 移除IP白名单 |
| POST | `/api/v1/client-auth/ip-whitelist/auto-add` | 公开 | 自动新增IP |
| POST | `/api/v1/client-auth/login` | 公开 | 登录 |
| POST | `/api/v1/client-auth/logout` | 公开 | 登出 |
| POST | `/api/v1/client-auth/logs/package` | 公开 | 创建日志日志包 |
| GET | `/api/v1/client-auth/logs/packages` | 公开 | 列表日志日志包 |
| DELETE | `/api/v1/client-auth/logs/packages/:id` | 公开 | 删除日志日志包 |
| GET | `/api/v1/client-auth/logs/packages/:id` | 公开 | 获取日志日志包 |
| GET | `/api/v1/client-auth/logs/packages/:id/download` | 公开 | 下载日志日志包 |
| GET | `/api/v1/client-auth/logs/types` | 公开 | 日志日志包健康检查 |
| GET | `/api/v1/client-auth/me` | 公开 | 获取当前登录用户 |
| GET | `/api/v1/client-auth/nodes` | 公开 | 我的节点列表 |
| GET | `/api/v1/client-auth/nodes/:id/config` | 公开 | 节点配置 |
| GET | `/api/v1/client-auth/nodes/:id/link` | 公开 | 节点分享链接 |
| GET | `/api/v1/client-auth/nodes/:id/qrcode` | 公开 | 节点二维码 |
| GET | `/api/v1/client-auth/oauth/:provider` | 公开 | 授权 |
| GET | `/api/v1/client-auth/oauth/:provider/callback` | 公开 | 回调 |
| GET | `/api/v1/client-auth/oauth/providers` | 公开 | 提供商状态 |
| GET | `/api/v1/client-auth/orders` | 公开 | 订单列表 |
| POST | `/api/v1/client-auth/orders` | 公开 | 创建订单 |
| GET | `/api/v1/client-auth/orders/:id` | 公开 | 订单详情 |
| GET | `/api/v1/client-auth/plans` | 公开 | 套餐列表 |
| GET | `/api/v1/client-auth/plans/:id` | 公开 | 套餐详情 |
| GET | `/api/v1/client-auth/profile` | 公开 | 获取资料 |
| PUT | `/api/v1/client-auth/profile` | 公开 | 更新资料 |
| POST | `/api/v1/client-auth/profile/bind-oauth` | 公开 | 绑定第三方账号 |
| DELETE | `/api/v1/client-auth/profile/unbind-oauth/:provider` | 公开 | 解绑第三方账号 |
| GET | `/api/v1/client-auth/referral/code` | 公开 | 获取推广验证码 |
| GET | `/api/v1/client-auth/referral/records` | 公开 | 列表推广记录 |
| GET | `/api/v1/client-auth/referral/stats` | 公开 | 获取推广统计 |
| POST | `/api/v1/client-auth/referral/withdraw` | 公开 | Request提现 |
| GET | `/api/v1/client-auth/referral/withdrawals` | 公开 | 列表推广提现 |
| POST | `/api/v1/client-auth/refresh` | 公开 | 刷新 |
| POST | `/api/v1/client-auth/register` | 公开 | 注册 |
| POST | `/api/v1/client-auth/reset-subscription` | 公开 | 重置订阅 |
| POST | `/api/v1/client-auth/rotate-token` | 公开 | 轮换订阅 token |
| POST | `/api/v1/client-auth/self-unban` | 公开 | 自助解封 |
| PUT | `/api/v1/client-auth/settings` | 公开 | 更新设置 |
| POST | `/api/v1/client-auth/settings/auto-add-ip` | 公开 | 自动新增IP |
| GET | `/api/v1/client-auth/settings/whitelist` | 公开 | 列表IP白名单 |
| DELETE | `/api/v1/client-auth/settings/whitelist/:id` | 公开 | 移除IP白名单 |
| GET | `/api/v1/client-auth/shared-accounts` | 公开 | 列表共享账号 |
| GET | `/api/v1/client-auth/shared-accounts/:id` | 公开 | 获取共享账号 |
| POST | `/api/v1/client-auth/shared-accounts/:id/apply` | 公开 | 申请共享账号 |
| POST | `/api/v1/client-auth/shared-accounts/:id/repair` | 公开 | Request修复 |
| GET | `/api/v1/client-auth/subscription-formats` | 公开 | 支持的订阅格式 |
| GET | `/api/v1/client-auth/subscription-link` | 公开 | 我的订阅链接 |
| GET | `/api/v1/client-auth/subscription-link/backup` | 公开 | 备用订阅链接 |
| POST | `/api/v1/client-auth/subscription/reset` | 公开 | 重置订阅 |
| POST | `/api/v1/client-auth/ticket` | 公开 | 签发一次性短时票据 |
| GET | `/api/v1/client-auth/tickets` | 公开 | 列表工单 |
| POST | `/api/v1/client-auth/tickets` | 公开 | 创建工单 |
| GET | `/api/v1/client-auth/tickets/:id` | 公开 | 获取工单 |
| POST | `/api/v1/client-auth/tickets/:id/close` | 公开 | 关闭工单 |
| POST | `/api/v1/client-auth/tickets/:id/reply` | 公开 | 回复工单 |
| GET | `/api/v1/client-auth/traffic-history` | 公开 | 流量历史 |
| GET | `/api/v1/client-auth/traffic-packs` | 公开 | 流量包列表 |
| POST | `/api/v1/client-auth/traffic-packs/buy` | 公开 | 购买流量包 |

### 订阅下发

客户端拉取订阅的公开入口，详见 subscription-api.md。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/sub/:token` | 公开 | Serve订阅 |

### 运行时配置工作流

配置快照 / 干跑 / 应用 / 回滚（apply、rollback、disable-proxy 需管理员）。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| POST | `/runtime/apply` | 公开 | 申请 |
| GET | `/runtime/apply-jobs` | 登录 | 列表任务 |
| GET | `/runtime/apply-jobs/:id` | 登录 | 获取任务 |
| POST | `/runtime/apply-jobs/:id/cancel` | 公开 | 取消任务 |
| POST | `/runtime/apply-jobs/:id/retry` | 公开 | 重试任务 |
| POST | `/runtime/disable-proxy` | 公开 | 禁用代理 |
| POST | `/runtime/dry-run` | 登录 | 试算执行 |
| GET | `/runtime/nodes` | 登录 | 列表节点 |
| GET | `/runtime/overview` | 登录 | 总览 |
| POST | `/runtime/rollback` | 公开 | 回滚 |
| GET | `/runtime/snapshots` | 登录 | 列表快照 |
| POST | `/runtime/snapshots` | 登录 | 创建快照 |
| DELETE | `/runtime/snapshots/:id` | 公开 | 删除快照 |
| GET | `/runtime/snapshots/:id` | 登录 | 获取快照 |
| POST | `/runtime/verify` | 登录 | 验证 |

### 服务访问策略

服务级访问策略与 IP 封禁。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/services/:id/access-policies` | 登录 | 策略列表 |
| POST | `/services/:id/access-policies` | 登录 | 创建策略 |
| DELETE | `/services/:id/access-policies/:policyId` | 登录 | 删除策略 |
| PUT | `/services/:id/access-policies/:policyId` | 登录 | 更新策略 |
| GET | `/services/:id/ip-bans` | 登录 | IP 封禁列表 |
| POST | `/services/:id/ip-bans` | 登录 | 新增 IP 封禁 |
| DELETE | `/services/:id/ip-bans/:banId` | 登录 | 删除 IP 封禁 |

### 流量配额监控

配额与用量的只读监控视图。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/traffic/quotas` | 登录 | 配额列表 |
| GET | `/traffic/quotas/:id/usage` | 登录 | 配额用量 |
| GET | `/traffic/reset-logs` | 登录 | 配额重置记录 |
| GET | `/traffic/usage-history` | 登录 | 用量历史 |

### 健康检查

存活探针。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/health` | 公开 | 存活探针，返回 200 即健康 |

### 门户页

门户 SPA 页面。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/p` | 公开 | 门户页 HTML（服务端渲染 + Vue 运行时） |

### ACME 挑战

HTTP-01 挑战回调。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/.well-known/acme-challenge/:token` | 公开 | ACME HTTP-01 挑战回调（证书签发时验证域名） |
