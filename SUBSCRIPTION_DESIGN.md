# 订阅设计文档

## 1. 文档目标

本文件用于定义以下内容：

- 订阅地址数据模型设计
- 订阅签名与轮换流程
- 订阅 API 请求响应结构
- 多种客户端的订阅兼容策略
- 订阅安全与可用性的平衡方案

边界说明：

- 本文档中的“服务端订阅”指服务端把自己管理的节点、入口和规则模板生成给外部客户端导入
- 本文档中的“客户端订阅”指客户端使用外部分享的订阅、购买的订阅或其它第三方订阅源
- 服务端生成订阅与客户端消费订阅必须分开设计，不能混用数据模型与页面入口

本设计重点解决一个实际问题：

- 很多客户端只会保存一个固定订阅地址
- 一旦把订阅安全做成“每次访问都必须动态变签名”，会导致很多客户端无法稳定更新

因此，订阅设计必须优先兼容真实客户端行为，而不是只追求理论上的最强安全。

## 2. 核心结论

先给出最终建议：

- 主订阅地址必须是长期稳定的固定 URL
- 主安全机制应采用高强度随机 `token`
- 轮换应作为手动操作或泄露后的恢复操作
- 不应把“每次请求动态签名”作为通用客户端的默认方案
- 签名机制可以作为增强模式，但不能破坏基础兼容性

一句话总结：

- 订阅地址默认“稳定”
- 安全依赖“高强度 token + 可吊销 + 可轮换 + 限速 + 审计”

## 3. 为什么不能把动态签名当默认方案

很多客户端的订阅更新方式是：

1. 用户导入一个固定订阅 URL
2. 客户端定期用这个 URL 拉取订阅
3. 客户端不会在每次请求前重新生成签名

这意味着如果服务端要求：

- 每次请求都带新的时间签名
- 每次请求都带一次性随机值
- URL 在短时间内自动失效

那么大多数客户端会直接失效。

因此以下方案不适合作为默认订阅模式：

- 一次性订阅 URL
- 每次请求不同签名
- 超短时效签名 URL
- 必须前端先换票再访问订阅

这些更适合：

- 浏览器临时下载
- 管理端手动导出
- 高安全内部场景

不适合：

- 通用订阅客户端

## 4. 推荐的订阅安全模型

推荐采用三层安全模型：

### 4.1 第一层：稳定主订阅地址

格式示例：

```text
https://example.com/sub/<token>
```

要求：

- `token` 长度足够长
- 使用高强度随机数生成
- 不包含用户 ID、递增 ID 或可推断信息
- 客户端长期保存同一个地址

这是兼容所有客户端的基础模式。

### 4.2 第二层：服务端风控与控制

在不改变 URL 的前提下做安全增强：

- 有效期控制
- 吊销控制
- 轮换控制
- 访问频率限制
- 异常访问检测
- 访问日志记录

这样不会破坏客户端兼容性。

### 4.3 第三层：可选增强签名

只在客户端支持的情况下使用：

- 固定附加签名参数
- 固定设备标识参数
- 固定客户端标识参数

例如：

```text
https://example.com/sub/<token>?sig=<static-sign>
```

注意：

- 这里的签名应是稳定的，不应要求每次请求变化
- 如果客户端不能保留这些参数，则不应强制启用

## 5. 数据模型设计

### 5.1 subscriptions 表建议

在现有设计基础上，建议扩展为：

```sql
CREATE TABLE subscriptions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  client_id INTEGER NOT NULL,
  format TEXT NOT NULL,
  token TEXT NOT NULL UNIQUE,
  token_version INTEGER NOT NULL DEFAULT 1,
  status TEXT NOT NULL DEFAULT 'active',
  security_mode TEXT NOT NULL DEFAULT 'token_only',
  signature TEXT,
  bind_mode TEXT NOT NULL DEFAULT 'none',
  bind_value TEXT,
  activated_at DATETIME,
  max_active_days INTEGER,
  expires_at DATETIME,
  quota_bytes INTEGER NOT NULL DEFAULT 0,
  used_bytes_cache INTEGER NOT NULL DEFAULT 0,
  quota_reset_cycle TEXT NOT NULL DEFAULT 'none',
  quota_reset_day INTEGER,
  last_reset_at DATETIME,
  next_reset_at DATETIME,
  suspend_on_quota_exhausted INTEGER NOT NULL DEFAULT 1,
  quota_state TEXT NOT NULL DEFAULT 'normal',
  suspended_at DATETIME,
  revoked_at DATETIME,
  last_access_at DATETIME,
  access_count INTEGER NOT NULL DEFAULT 0,
  last_ip TEXT,
  last_user_agent TEXT,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
);
```

字段说明：

- `token`：订阅主凭据
- `token_version`：轮换次数或版本号
- `status`：`active`、`revoked`、`expired`
- `security_mode`：安全模式
- `signature`：固定签名，可选
- `bind_mode`：绑定方式
- `bind_value`：绑定值，例如设备标识或来源信息
- `activated_at`：~~首次成功代理认证时间~~ **暂缓（2026-09-23 决议不做）**
- `max_active_days`：~~激活后有效天数~~ **暂缓（不实现）** —— 订阅有效期的**唯一来源是套餐**：
  `plan_variant.duration_days`（购买/续费/升级/试用/卡密兑换）或客户端表单里的到期日 → `expires_at`。
  "首次使用起算"这条时间轴没有入口，保留只会造成两个到期来源的困惑；因此字段的写入会被 API 拒绝
  （fail-closed，与 `security_mode` 同样处理），库列保留仅为兼容。将来若要在**卡密兑换**上支持
  "兑换后先不起算、首次使用才起算"，应在那条流程里新设计入口，而不是靠这个孤立字段。
- `revoked_at`：吊销时间
- `quota_bytes`：当前周期总配额，`0` 表示不限量
- `used_bytes_cache`：当前周期已用流量缓存值，真实来源仍以流量账本聚合为准
- `quota_reset_cycle`：`none` 或 `monthly`
- `quota_reset_day`：每月重置日，未设置时按首次激活日对齐
- `quota_state`：`normal`、`near_limit`、`over_limit_soft`、`over_limit_hard`、`suspended`、`restored`、`expired_days`
- `suspend_on_quota_exhausted`：达到配额后是否暂停新代理连接

### 5.1.1 订阅配额重置日志

订阅配额重置日志统一存储在 `traffic_quota_reset_logs` 表中（定义见 DEVELOPMENT.md §7.8.2），通过 `subject_type = 'subscription'` 区分。

不再使用独立的 `subscription_usage_reset_logs` 表，避免两张表语义重复。

### 5.2 subscription_access_logs 表建议

```sql
CREATE TABLE subscription_access_logs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  subscription_id INTEGER NOT NULL,
  ip TEXT,
  user_agent TEXT,
  status_code INTEGER NOT NULL,
  hit_result TEXT NOT NULL,
  risk_level TEXT,
  created_at DATETIME NOT NULL
);
```

用途：

- 记录访问情况
- 排查泄露
- 做风控
- 做限速统计

### 5.3 subscription_rotations 表建议

```sql
CREATE TABLE subscription_rotations (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  subscription_id INTEGER NOT NULL,
  old_token TEXT NOT NULL,
  new_token TEXT NOT NULL,
  rotated_by INTEGER NOT NULL,
  reason TEXT,
  created_at DATETIME NOT NULL
);
```

用途：

- 记录轮换历史
- 便于审计
- 便于回溯泄露事件

### 5.4 server_share_profiles 表建议

服务端节点分享需要独立于客户端“使用订阅”的模型。

```sql
CREATE TABLE server_share_profiles (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  client_id INTEGER,
  target_client_type TEXT NOT NULL,
  output_format TEXT NOT NULL,
  delivery_mode TEXT NOT NULL,
  template_id INTEGER,
  route_template_id INTEGER,
  default_outbound_mode TEXT NOT NULL DEFAULT 'rule',
  expires_at DATETIME,
  security_mode TEXT NOT NULL DEFAULT 'token_only',
  bind_mode TEXT NOT NULL DEFAULT 'none',
  include_rule_urls INTEGER NOT NULL DEFAULT 1,
  include_custom_rules INTEGER NOT NULL DEFAULT 1,
  https_required INTEGER NOT NULL DEFAULT 1,
  status TEXT NOT NULL DEFAULT 'draft',
  last_preview_at DATETIME,
  last_published_at DATETIME,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
);
```

字段说明：

- `target_client_type`：目标导入客户端，例如 `sing-box`、`xray-core`、`mihomo`、`universal_uri`
- `output_format`：实际输出格式，例如 `singbox_json`、`xray_json`、`mihomo_yaml`、`base64_uri_bundle`
- `delivery_mode`：交付方式，例如 `single_uri`、`single_uri_qr`、`https_subscription`、`https_subscription_qr`
- `template_id`：核心模板或分享模板
- `route_template_id`：分流模板
- `include_rule_urls`：是否把远程规则 URL 写入输出
- `include_custom_rules`：是否把自定义规则写入输出
- `https_required`：强制要求分享链接只能以 `HTTPS` 发布
- `status`：`draft`、`preview_ready`、`published`、`revoked`

### 5.5 server_share_profile_nodes 表建议

```sql
CREATE TABLE server_share_profile_nodes (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  profile_id INTEGER NOT NULL,
  node_id INTEGER NOT NULL,
  node_order INTEGER NOT NULL DEFAULT 0,
  enabled INTEGER NOT NULL DEFAULT 1,
  is_primary INTEGER NOT NULL DEFAULT 0,
  remark TEXT,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
);
```

用途：

- 保存“按节点勾选生成分享内容”的结果
- 保持订阅节点顺序稳定
- 支持主节点、备用节点和禁用节点

### 5.6 server_share_rule_sources 表建议

```sql
CREATE TABLE server_share_rule_sources (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  profile_id INTEGER NOT NULL,
  source_type TEXT NOT NULL,
  name TEXT NOT NULL,
  source_url TEXT,
  custom_content TEXT,
  rule_format TEXT,
  priority INTEGER NOT NULL DEFAULT 100,
  enabled INTEGER NOT NULL DEFAULT 1,
  sync_interval_seconds INTEGER,
  last_sync_at DATETIME,
  last_sync_status TEXT,
  last_error_message TEXT,
  last_etag TEXT,
  last_modified TEXT,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
);
```

字段说明：

- `source_type`：`remote_url`、`custom_inline`
- `source_url`：远程规则 URL
- `custom_content`：自定义规则文本或结构化规则 JSON
- `rule_format`：原始规则规格，例如 `mihomo_rule_provider`、`singbox_rule_set`、`cidr_text`
- `priority`：规则合并顺序

### 5.7 server_share_render_revisions 表建议

```sql
CREATE TABLE server_share_render_revisions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  profile_id INTEGER NOT NULL,
  subscription_id INTEGER,
  render_version TEXT NOT NULL,
  target_client_type TEXT NOT NULL,
  output_format TEXT NOT NULL,
  delivery_mode TEXT NOT NULL,
  mime_type TEXT NOT NULL,
  file_name TEXT,
  node_count INTEGER NOT NULL DEFAULT 0,
  included_rule_url_count INTEGER NOT NULL DEFAULT 0,
  included_custom_rule_count INTEGER NOT NULL DEFAULT 0,
  degrade_summary TEXT,
  render_hash TEXT NOT NULL,
  qr_payload TEXT,
  content_preview TEXT,
  created_at DATETIME NOT NULL
);
```

用途：

- 保存每次预览或发布生成的订阅渲染结果
- 记录规则注入后的降级说明
- 为二维码导出和文件下载提供稳定版本

### 5.8 server_share_publish_jobs 表建议

```sql
CREATE TABLE server_share_publish_jobs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  profile_id INTEGER NOT NULL,
  render_revision_id INTEGER,
  subscription_id INTEGER,
  job_type TEXT NOT NULL,
  status TEXT NOT NULL,
  phase TEXT,
  error_code TEXT,
  error_message TEXT,
  can_retry INTEGER NOT NULL DEFAULT 0,
  can_reuse_last_render INTEGER NOT NULL DEFAULT 0,
  requires_action INTEGER NOT NULL DEFAULT 0,
  requires_action_reason TEXT,
  started_at DATETIME,
  finished_at DATETIME,
  created_at DATETIME NOT NULL
);
```

用途：

- 保存预览、发布、二维码导出、文件导出的任务记录
- 驱动 UI 的按钮可用性和失败重试
- 为审计和结构化事件提供稳定主键

### 5.9 server_share_audit_events 表建议

```sql
CREATE TABLE server_share_audit_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  profile_id INTEGER NOT NULL,
  publish_job_id INTEGER,
  subscription_id INTEGER,
  event_type TEXT NOT NULL,
  operator_id INTEGER,
  level TEXT NOT NULL,
  message TEXT NOT NULL,
  detail_json TEXT,
  created_at DATETIME NOT NULL
);
```

建议事件类型：

- `share_profile_created`
- `share_profile_preview_started`
- `share_profile_preview_succeeded`
- `share_profile_preview_failed`
- `share_profile_published`
- `share_profile_publish_failed`
- `share_qrcode_exported`
- `share_file_exported`
- `share_subscription_revoked`

### 5.10 subscriptions 与分享档案的关系

`subscriptions` 仍作为最终对外订阅凭据表使用，但需要补充与分享档案的关联语义：

- 一个 `server_share_profiles` 可以对应多个历史订阅版本
- 一个 `subscriptions` 记录只绑定一个目标分享档案
- 访问 `GET /sub/:token` 时，先找到 `subscriptions`，再装配关联的 `server_share_profiles`
- 节点勾选结果、分流模板和规则 URL 不直接保存在 token 表中，而是保存在分享档案与渲染版本中

### 5.11 服务端节点分享索引建议

建议索引：

```sql
CREATE UNIQUE INDEX idx_subscriptions_token ON subscriptions(token);
CREATE INDEX idx_subscriptions_status_expires ON subscriptions(status, expires_at);
CREATE INDEX idx_subscriptions_last_access ON subscriptions(last_access_at);

CREATE INDEX idx_server_share_profiles_status_updated ON server_share_profiles(status, updated_at);
CREATE INDEX idx_server_share_profiles_client_type ON server_share_profiles(target_client_type, output_format);

CREATE UNIQUE INDEX idx_server_share_profile_nodes_profile_node ON server_share_profile_nodes(profile_id, node_id);
CREATE INDEX idx_server_share_profile_nodes_profile_order ON server_share_profile_nodes(profile_id, node_order);

CREATE INDEX idx_server_share_rule_sources_profile_priority ON server_share_rule_sources(profile_id, priority);
CREATE INDEX idx_server_share_rule_sources_profile_type ON server_share_rule_sources(profile_id, source_type);
CREATE INDEX idx_server_share_rule_sources_sync_status ON server_share_rule_sources(last_sync_status, last_sync_at);

CREATE UNIQUE INDEX idx_server_share_render_revisions_version ON server_share_render_revisions(render_version);
CREATE INDEX idx_server_share_render_revisions_profile_created ON server_share_render_revisions(profile_id, created_at);
CREATE INDEX idx_server_share_render_revisions_subscription ON server_share_render_revisions(subscription_id);

CREATE INDEX idx_server_share_publish_jobs_profile_created ON server_share_publish_jobs(profile_id, created_at);
CREATE INDEX idx_server_share_publish_jobs_status_created ON server_share_publish_jobs(status, created_at);
CREATE INDEX idx_server_share_publish_jobs_type_status ON server_share_publish_jobs(job_type, status);

CREATE INDEX idx_server_share_audit_events_profile_created ON server_share_audit_events(profile_id, created_at);
CREATE INDEX idx_server_share_audit_events_job_created ON server_share_audit_events(publish_job_id, created_at);
CREATE INDEX idx_server_share_audit_events_type_created ON server_share_audit_events(event_type, created_at);
```

原则：

- 列表页优先优化 `status + updated_at`
- 详情页优先优化 `profile_id + created_at`
- 审计和任务时间线优先优化“按档案倒序查”
- 避免对大文本字段如 `detail_json`、`content_preview` 建普通索引

### 5.12 服务端节点分享迁移顺序建议

建议迁移顺序：

1. 创建 `server_share_profiles`
2. 创建 `server_share_profile_nodes`
3. 创建 `server_share_rule_sources`
4. 创建 `server_share_render_revisions`
5. 创建 `server_share_publish_jobs`
6. 创建 `server_share_audit_events`
7. 为 `subscriptions` 增加与分享档案关联字段或关系约束
8. 最后补充所有索引

迁移规则：

- 表结构迁移与索引迁移分文件管理
- 先表后索引，避免升级过程中锁表时间过长
- 若涉及给 `subscriptions` 补关联字段，必须先为旧数据填充默认兼容值
- 不可逆迁移前必须执行数据库快照

## 6. 安全模式设计

建议支持以下模式：

### 6.1 `token_only`

最兼容模式：

- 仅使用高强度 token
- 推荐作为默认模式

适用：

- 通用客户端
- 大多数导入场景

### 6.2 `token_with_expiry`

在 `token_only` 基础上增加过期时间：

- URL 不变
- 到期后失效

适用：

- 临时订阅
- 试用用户

### 6.3 `token_with_signature`

在固定 URL 上附带固定签名：

- 签名不应频繁变化
- 只适合支持保留参数的客户端

适用：

- 已知兼容的客户端
- 需要额外校验的场景

### 6.4 `token_with_binding`

绑定来源信息：

- 可绑定设备标识
- 可绑定固定来源 IP 段
- 可绑定客户端类型

注意：

- 不要默认强制绑定 IP
- 因为很多客户端网络环境变化大

## 7. 轮换策略设计

### 7.1 什么是轮换

订阅轮换不是“每次请求都变”。

订阅轮换应定义为：

- 管理员或用户主动触发生成新订阅地址
- 新地址生效
- 旧地址立即失效或短暂宽限后失效

### 7.2 什么时候轮换

建议在以下场景支持轮换：

- 订阅地址泄露
- 用户更换设备
- 风控检测到异常访问
- 用户主动重置订阅

### 7.3 轮换流程

推荐流程：

1. 用户或管理员触发轮换
2. 服务端生成新 `token`
3. 记录轮换历史
4. 旧地址立刻失效或进入极短宽限期
5. 新地址返回给用户
6. 写入审计日志

### 7.4 不推荐的做法

不推荐：

- 定时自动轮换但不通知客户端
- 每次访问自动生成新地址
- 要求客户端自己先调用换票接口

原因：

- 大多数订阅客户端不支持
- 会导致用户以为订阅坏了

## 8. 吊销策略设计

必须支持：

- 手动吊销
- 风控自动吊销
- 到期失效

吊销后的行为：

- 返回明确状态码
- 返回简短错误说明
- 写入访问日志

建议：

- 吊销和轮换分开设计
- 吊销不等于自动签发新地址

## 9. 多客户端支持策略

### 9.1 通用策略

所有客户端都应优先支持：

- 固定 URL 订阅
- HTTPS 访问
- 无需动态签名

### 9.2 `sing-box`

建议支持：

- JSON 订阅输出
- 通用 URL 订阅
- 可选固定签名参数

### 9.3 `Xray-core`

建议支持：

- JSON 订阅输出
- Base64 聚合订阅
- 固定 URL

### 9.4 `Mihomo`

建议支持：

- YAML 订阅
- 分组规则输出
- 固定 URL

### 9.5 兼容格式矩阵

| 目标客户端 | 推荐输出 | 支持交付方式 | 远程规则 URL | 自定义规则 | 降级策略 |
| --- | --- | --- | --- | --- | --- |
| `sing-box` | `singbox_json` | `HTTPS` 订阅、订阅二维码、文件下载 | 原生支持，优先写规则集引用 | 支持写入规则和出站选择 | 若规则源格式不兼容，先归一化为内部模型再输出 |
| `Xray-core` | `xray_json`、`base64_uri_bundle` | `HTTPS` 订阅、文件下载 | 不建议直接透传通用规则 URL | 仅支持可映射到路由规则的子集 | 无法表达的规则必须在预览中标记为忽略 |
| `Mihomo` | `mihomo_yaml` | `HTTPS` 订阅、订阅二维码、文件下载 | 支持 `rule-providers` 或远程规则集 URL | 支持规则、策略组和直连策略 | 若规则不支持映射为 `Mihomo` 规则，必须降级为注释说明 |
| `universal_uri` | `raw_uri`、`base64_uri_bundle` | 单节点 URI、二维码、文本下载 | 不支持 | 不支持 | 只输出节点，不输出分流信息 |

要求：

- 同一份分享档案一次只面向一个目标客户端类型渲染
- 二维码只承载“单节点 URI”或“HTTPS 订阅链接”，不直接承载大体积完整配置
- 若目标客户端不支持远程规则 URL 或自定义规则，页面和 API 都必须返回结构化降级说明

### 9.6 规则模板注入策略

统一原则：

- 服务端订阅先生成统一内部节点与规则模型，再按目标客户端格式渲染
- 远程规则 URL 与自定义规则需要分开建模、分开统计、分开降级
- 节点分享与规则模板输出必须绑定同一个渲染版本，避免链接内容与预览不一致

建议流程：

1. 读取已选节点列表
2. 读取分享模板与分流模板
3. 拉取并校验远程规则 URL
4. 读取本地自定义规则
5. 统一归一化为内部规则模型
6. 按目标客户端能力做裁剪和降级
7. 生成预览内容、文件名、二维码载荷与订阅 URL

规则注入要求：

- 远程规则 URL 支持 `ETag`、`Last-Modified` 和 `304`
- 自定义规则必须保留原始文本和归一化结果
- 不能表达的规则要进入 `degrade_summary`
- 若所有规则都无法表达，节点分享仍可继续，但必须明确提示“仅输出节点，不包含分流规则”

### 9.7 浏览器下载模式

浏览器下载可以使用更严格策略：

- 短时签名 URL
- 一次性下载链接
- 二次确认

但这不应影响客户端订阅模式。

## 10. API 设计

服务端订阅 API 说明：

- 以下 API 默认指服务端向外部客户端分发节点内容与订阅内容
- 不用于客户端多订阅合并场景
- 客户端多订阅合并应在客户端配置与运行模块中单独设计

### 10.1 服务端节点分享档案 API

#### 10.1.1 创建分享档案

`POST /api/v1/server-share/profiles`

请求示例：

```json
{
  "name": "ios-singbox-share",
  "client_id": 1001,
  "target_client_type": "sing-box",
  "output_format": "singbox_json",
  "delivery_mode": "https_subscription",
  "node_ids": [101, 103, 108],
  "route_template_id": 12,
  "rule_sources": [
    {
      "source_type": "remote_url",
      "name": "cn-rule",
      "source_url": "https://example.com/rules/cn.srs",
      "rule_format": "singbox_rule_set",
      "priority": 100
    },
    {
      "source_type": "custom_inline",
      "name": "local-direct",
      "custom_content": "DOMAIN-SUFFIX,example.com,DIRECT",
      "rule_format": "mihomo_rule_provider",
      "priority": 10
    }
  ],
  "security_mode": "token_only",
  "expires_at": "2026-12-31T23:59:59Z"
}
```

响应示例：

```json
{
  "id": 11,
  "status": "draft",
  "target_client_type": "sing-box",
  "output_format": "singbox_json",
  "delivery_mode": "https_subscription",
  "selected_node_count": 3,
  "https_required": true
}
```

#### 10.1.2 预览分享内容

`POST /api/v1/server-share/profiles/:id/preview`

请求示例：

```json
{
  "regenerate_rules": true,
  "include_qrcode": true
}
```

响应示例：

```json
{
  "profile_id": 11,
  "render_version": "shr_20260525_000001",
  "status": "preview_ready",
  "target_client_type": "sing-box",
  "delivery_mode": "https_subscription",
  "selected_node_count": 3,
  "share_url": "https://example.com/sub/sub_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
  "preview_excerpt": "{...}",
  "degrade_items": [],
  "rule_summary": {
    "remote_rule_url_count": 1,
    "custom_rule_count": 1,
    "compiled_rule_count": 24
  },
  "qr_ready": true
}
```

#### 10.1.3 发布分享订阅

`POST /api/v1/server-share/profiles/:id/publish`

请求示例：

```json
{
  "publish_mode": "issue_new_token",
  "invalidate_previous_token": true
}
```

响应示例：

```json
{
  "profile_id": 11,
  "subscription_id": 88,
  "status": "published",
  "subscription_url": "https://example.com/sub/sub_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
  "expires_at": "2026-12-31T23:59:59Z",
  "render_version": "shr_20260525_000001"
}
```

#### 10.1.4 获取二维码

`GET /api/v1/server-share/profiles/:id/qrcode?render_version=shr_20260525_000001&format=png`

返回要求：

- `delivery_mode=single_uri_qr` 时返回节点 URI 的二维码
- `delivery_mode=https_subscription_qr` 时返回 `HTTPS` 订阅链接二维码
- 非 `HTTPS` 或无可用渲染版本时返回错误，不允许回退到 `HTTP`

#### 10.1.5 下载分享文件

`GET /api/v1/server-share/profiles/:id/export?render_version=shr_20260525_000001`

返回要求：

- 返回本次渲染版本对应的文件内容
- 文件名根据目标客户端自动生成，如 `share-singbox-20260525.json`
- 若存在降级项，响应头和响应体摘要中都要体现

#### 10.1.6 查询分享档案详情

`GET /api/v1/server-share/profiles/:id`

响应示例：

```json
{
  "id": 11,
  "name": "ios-singbox-share",
  "status": "published",
  "target_client_type": "sing-box",
  "output_format": "singbox_json",
  "delivery_mode": "https_subscription_qr",
  "selected_node_count": 3,
  "last_render_version": "shr_20260525_000001",
  "last_publish_job_status": "succeeded",
  "subscription_id": 88,
  "subscription_url_masked": "https://example.com/sub/sub_xxx...xxx",
  "degrade_items": [],
  "last_access_at": "2026-05-25T10:10:00Z",
  "access_count": 23
}
```

#### 10.1.7 查询分享历史版本

`GET /api/v1/server-share/profiles/:id/revisions`

返回要求：

- 按时间倒序返回渲染版本
- 显示每次版本对应的节点数、规则数、降级摘要和发布结果
- 支持标记“当前正在对外使用的版本”

#### 10.1.8 查询分享审计事件

`GET /api/v1/server-share/profiles/:id/events`

返回要求：

- 展示预览、发布、导出、吊销相关事件
- 每条事件返回 `event_type`、`level`、`message`、`detail_json`
- 支持按 `publish_job_id` 过滤

#### 10.1.9 吊销分享订阅

`POST /api/v1/server-share/profiles/:id/revoke`

请求示例：

```json
{
  "reason": "manual_revoke",
  "revoke_active_subscription": true
}
```

响应示例：

```json
{
  "profile_id": 11,
  "status": "revoked",
  "subscription_id": 88,
  "publish_job_id": 91,
  "revoked_at": "2026-05-25T10:20:00Z"
}
```

### 10.2 服务端节点分享错误响应样例

#### 10.2.1 通用错误响应结构

```json
{
  "error": {
    "code": "SHARE_PROFILE_HTTPS_REQUIRED",
    "message": "当前站点未完成 HTTPS 接入，不能生成分享链接",
    "request_id": "req_20260525_xxxxx",
    "can_retry": false,
    "can_reuse_last_render": false,
    "requires_action": true,
    "requires_action_reason": "https_not_ready",
    "detail": {
      "profile_id": 11,
      "delivery_mode": "https_subscription_qr"
    }
  }
}
```

字段要求：

- `code`：稳定错误码，供 UI 分支和审计使用
- `message`：用户可读错误文案
- `request_id`：用于日志追踪
- `can_retry`：是否允许页面直接展示“重试”
- `can_reuse_last_render`：是否允许复用最近成功渲染版本
- `requires_action`：是否必须人工处理
- `requires_action_reason`：固定枚举，驱动主按钮和跳转
- `detail`：结构化补充信息，不返回大段未脱敏敏感数据

#### 10.2.2 典型错误样例

1. 未选择节点

```json
{
  "error": {
    "code": "SHARE_PROFILE_EMPTY_NODES",
    "message": "至少选择一个节点后才能预览或发布",
    "request_id": "req_20260525_empty_nodes",
    "can_retry": false,
    "can_reuse_last_render": false,
    "requires_action": true,
    "requires_action_reason": "select_nodes_first",
    "detail": {
      "profile_id": 11
    }
  }
}
```

2. 远程规则拉取失败

```json
{
  "error": {
    "code": "SHARE_RULE_SOURCE_FETCH_FAILED",
    "message": "远程规则源同步失败",
    "request_id": "req_20260525_rule_fetch",
    "can_retry": true,
    "can_reuse_last_render": true,
    "requires_action": false,
    "requires_action_reason": "",
    "detail": {
      "profile_id": 11,
      "source_name": "cn-rule",
      "source_url": "https://example.com/rules/cn.srs",
      "last_success_snapshot_at": "2026-05-24T08:00:00Z"
    }
  }
}
```

3. 进入人工介入

```json
{
  "error": {
    "code": "SHARE_NEEDS_INTERVENTION",
    "message": "订阅地址已轮换，但版本关联未完整写入，需要人工处理",
    "request_id": "req_20260525_intervention",
    "can_retry": false,
    "can_reuse_last_render": false,
    "requires_action": true,
    "requires_action_reason": "publish_state_inconsistent",
    "detail": {
      "profile_id": 11,
      "publish_job_id": 81,
      "subscription_id": 88
    }
  }
}
```

### 10.3 创建订阅

`POST /api/v1/subscriptions`

请求示例：

```json
{
  "client_id": 1001,
  "format": "singbox_json",
  "security_mode": "token_only",
  "expires_at": "2026-12-31T23:59:59Z",
  "bind_mode": "none",
  "max_active_days": 30,
  "quota_bytes": 214748364800,
  "quota_reset_cycle": "monthly",
  "quota_reset_day": 1,
  "suspend_on_quota_exhausted": true
}
```

响应示例：

```json
{
  "id": 1,
  "status": "active",
  "token": "sub_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
  "subscription_url": "https://example.com/sub/sub_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
  "security_mode": "token_only",
  "expires_at": "2026-12-31T23:59:59Z",
  "max_active_days": 30,
  "quota_bytes": 214748364800,
  "used_bytes": 53687091200,
  "quota_reset_cycle": "monthly",
  "quota_reset_day": 1,
  "quota_state": "normal",
  "next_reset_at": "2026-06-01T00:00:00Z"
}
```

### 10.4 查询订阅列表

`GET /api/v1/subscriptions`

响应示例：

```json
{
  "items": [
    {
      "id": 1,
      "client_id": 1001,
      "format": "mihomo_yaml",
      "status": "active",
      "security_mode": "token_only",
      "expires_at": "2026-12-31T23:59:59Z",
      "max_active_days": 30,
      "quota_bytes": 214748364800,
      "used_bytes": 53687091200,
      "quota_reset_cycle": "monthly",
      "quota_state": "near_limit",
      "last_access_at": "2026-05-22T10:00:00Z",
      "access_count": 28
    }
  ]
}
```

### 10.5 查询订阅详情

`GET /api/v1/subscriptions/:id`

响应示例：

```json
{
  "id": 1,
  "client_id": 1001,
  "format": "singbox_json",
  "status": "active",
  "security_mode": "token_only",
  "token_version": 1,
  "bind_mode": "none",
  "subscription_url_masked": "https://example.com/sub/sub_xxx...xxx",
  "activated_at": "2026-05-02T08:00:00Z",
  "max_active_days": 30,
  "expires_at": "2026-12-31T23:59:59Z",
  "quota_bytes": 214748364800,
  "used_bytes": 53687091200,
  "remaining_bytes": 161061273600,
  "quota_reset_cycle": "monthly",
  "quota_reset_day": 1,
  "last_reset_at": "2026-05-01T00:00:00Z",
  "next_reset_at": "2026-06-01T00:00:00Z",
  "quota_state": "normal",
  "remaining_active_days": 10,
  "effective_expire_at": "2026-06-01T08:00:00Z",
  "suspend_on_quota_exhausted": true,
  "suspended_at": null,
  "last_access_at": "2026-05-22T10:00:00Z",
  "access_count": 28,
  "can_reset_usage": true,
  "can_restore": false,
  "can_rotate": true,
  "can_recalculate_next_reset": true
}
```

### 10.6 重置订阅用量

`POST /api/v1/subscriptions/:id/reset-usage`

请求示例：

```json
{
  "reason": "manual_monthly_reset",
  "reset_used_bytes": true,
  "recalculate_next_reset_at": true
}
```

响应示例：

```json
{
  "id": 1,
  "status": "active",
  "quota_state": "normal",
  "quota_bytes": 214748364800,
  "used_bytes": 0,
  "remaining_bytes": 214748364800,
  "last_reset_at": "2026-05-25T12:00:00Z",
  "next_reset_at": "2026-06-01T00:00:00Z",
  "reset_result": "succeeded",
  "reset_reason": "manual_monthly_reset",
  "can_reset_usage": true,
  "can_restore": false
}
```

### 10.7 恢复订阅

`POST /api/v1/subscriptions/:id/restore`

请求示例：

```json
{
  "reason": "operator_restore_after_payment",
  "clear_suspended_state": true,
  "recalculate_next_reset_at": true
}
```

响应示例：

```json
{
  "id": 1,
  "status": "active",
  "quota_state": "normal",
  "suspended_at": null,
  "effective_expire_at": "2026-06-15T08:00:00Z",
  "next_reset_at": "2026-06-01T00:00:00Z",
  "restore_result": "succeeded",
  "restore_reason": "operator_restore_after_payment",
  "can_reset_usage": true,
  "can_restore": false,
  "can_rotate": true
}
```

### 10.8 轮换订阅

`POST /api/v1/subscriptions/:id/rotate`

请求示例：

```json
{
  "reason": "suspected_leak",
  "invalidate_old_token_immediately": true
}
```

响应示例：

```json
{
  "id": 1,
  "old_status": "revoked",
  "new_status": "active",
  "new_token_version": 2,
  "subscription_url": "https://example.com/sub/sub_new_xxxxxxxxxxxxxxxxxxxx"
}
```

### 10.9 吊销订阅

`POST /api/v1/subscriptions/:id/revoke`

请求示例：

```json
{
  "reason": "manual_revoke"
}
```

响应示例：

```json
{
  "id": 1,
  "status": "revoked",
  "revoked_at": "2026-05-22T10:05:00Z"
}
```

### 10.10 获取订阅内容

`GET /sub/:token`

处理流程：

1. 查找 token
2. 校验状态
3. 校验是否过期
4. 校验风控策略
5. 记录访问日志
6. 返回目标格式内容

失败响应建议：

- `401`：无效 token
- `403`：已吊销或不允许访问
- `410`：已过期
- `429`：访问过快

## 11. 请求响应结构建议

### 11.1 ShareProfileItem

```json
{
  "id": 11,
  "name": "ios-singbox-share",
  "target_client_type": "sing-box",
  "output_format": "singbox_json",
  "delivery_mode": "https_subscription",
  "selected_node_count": 3,
  "remote_rule_url_count": 1,
  "custom_rule_count": 1,
  "status": "published",
  "https_required": true,
  "last_preview_at": "2026-05-25T10:00:00Z",
  "last_published_at": "2026-05-25T10:05:00Z"
}
```

### 11.2 SharePreviewResult

```json
{
  "profile_id": 11,
  "render_version": "shr_20260525_000001",
  "target_client_type": "mihomo",
  "delivery_mode": "https_subscription_qr",
  "share_url": "https://example.com/sub/sub_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
  "qr_ready": true,
  "degrade_items": [
    {
      "code": "RULE_URL_DOWNGRADED",
      "message": "目标客户端不支持原始规则 URL，已转换为内联规则"
    }
  ]
}
```

### 11.3 ShareQRCodeResult

```json
{
  "profile_id": 11,
  "render_version": "shr_20260525_000001",
  "payload_type": "https_subscription_url",
  "image_format": "png",
  "download_url": "/api/v1/server-share/profiles/11/qrcode?render_version=shr_20260525_000001&format=png"
}
```

### 11.4 ShareRevisionItem

```json
{
  "render_version": "shr_20260525_000001",
  "created_at": "2026-05-25T10:00:00Z",
  "delivery_mode": "https_subscription",
  "node_count": 3,
  "included_rule_url_count": 1,
  "included_custom_rule_count": 1,
  "degrade_summary": "",
  "is_active_published_revision": true
}
```

### 11.5 ShareAuditEvent

```json
{
  "id": 101,
  "event_type": "share_profile_publish_failed",
  "level": "error",
  "message": "目标客户端规则编译失败",
  "detail_json": {
    "publish_job_id": 81,
    "error_code": "SHARE_RENDER_FAILED",
    "render_version": "shr_20260525_000001"
  },
  "created_at": "2026-05-25T10:03:00Z"
}
```

### 11.6 ShareErrorResponse

```json
{
  "error": {
    "code": "SHARE_RULE_SOURCE_FETCH_FAILED",
    "message": "远程规则源同步失败",
    "request_id": "req_20260525_rule_fetch",
    "can_retry": true,
    "can_reuse_last_render": true,
    "requires_action": false,
    "requires_action_reason": "",
    "detail": {
      "profile_id": 11,
      "source_name": "cn-rule"
    }
  }
}
```

### 11.7 ShareDetailJSONSchema

```json
{
  "profile_id": 11,
  "publish_job_id": 81,
  "subscription_id": 88,
  "render_version": "shr_20260525_000001",
  "delivery_mode": "https_subscription",
  "target_client_type": "sing-box",
  "export_kind": "file",
  "mime_type": "application/json",
  "file_name": "share-singbox-20260525.json",
  "degrade_count": 1,
  "degrade_items": [],
  "error_code": "",
  "error_message": "",
  "requires_action": false
}
```

### 11.8 SubscriptionItem

```json
{
  "id": 1,
  "client_id": 1001,
  "format": "singbox_json",
  "status": "active",
  "security_mode": "token_only",
  "token_version": 1,
  "expires_at": "2026-12-31T23:59:59Z",
  "activated_at": "2026-05-02T08:00:00Z",
  "max_active_days": 30,
  "quota_bytes": 214748364800,
  "used_bytes": 53687091200,
  "quota_reset_cycle": "monthly",
  "quota_reset_day": 1,
  "quota_state": "normal",
  "next_reset_at": "2026-06-01T00:00:00Z",
  "last_access_at": "2026-05-22T10:00:00Z",
  "access_count": 28,
  "subscription_url_masked": "https://example.com/sub/sub_xxx...xxx"
}
```

### 11.9 SubscriptionAccessLog

```json
{
  "id": 1,
  "ip": "203.0.113.10",
  "user_agent": "Mihomo/1.0",
  "status_code": 200,
  "hit_result": "success",
  "risk_level": "low",
  "created_at": "2026-05-22T10:00:00Z"
}
```

### 11.10 SubscriptionQuotaResetResult

```json
{
  "id": 1,
  "status": "active",
  "quota_state": "normal",
  "used_bytes": 0,
  "remaining_bytes": 214748364800,
  "last_reset_at": "2026-05-25T12:00:00Z",
  "next_reset_at": "2026-06-01T00:00:00Z",
  "reset_result": "succeeded",
  "reset_reason": "manual_monthly_reset",
  "can_reset_usage": true,
  "can_restore": false
}
```

### 11.11 SubscriptionRestoreResult

```json
{
  "id": 1,
  "status": "active",
  "quota_state": "normal",
  "suspended_at": null,
  "effective_expire_at": "2026-06-15T08:00:00Z",
  "next_reset_at": "2026-06-01T00:00:00Z",
  "restore_result": "succeeded",
  "restore_reason": "operator_restore_after_payment",
  "can_reset_usage": true,
  "can_restore": false,
  "can_rotate": true
}
```

## 12. 订阅安全的最终建议

最终建议非常明确：

- 不要把“动态签名 URL”作为通用订阅主方案
- 主方案应该是稳定 URL + 高强度 token
- 轮换是人工触发或风险触发的恢复机制
- 吊销是独立能力
- 限速、日志、签名、风控作为增强层

最推荐默认方案：

- `token_only`
- 长随机 token
- 支持过期
- 支持吊销
- 支持轮换
- 支持访问日志
- 支持限速

这样既兼容绝大多数客户端，也能把订阅地址安全做扎实。

## 13. 订阅状态机设计

建议将订阅状态定义为明确状态机，而不是只靠布尔值。

推荐状态：

- `draft`
- `active`
- `expired`
- `revoked`
- `disabled`

状态说明：

- `draft`：已创建但尚未正式启用
- `active`：可正常访问
- `expired`：因时间或策略过期
- `revoked`：已主动吊销
- `disabled`：因风控或系统策略临时停用

推荐状态流转：

```text
draft -> active
active -> expired
active -> revoked
active -> disabled
disabled -> active
expired -> active   # 允许续期后重新启用
revoked -> active   # 仅在重新签发新 token 后表现为新订阅版本
```

注意：

- `revoked` 不建议直接恢复原 token
- 恢复应通过“重新签发”或“轮换”实现
- `expired` 是否可恢复由业务策略决定

## 14. 服务端节点分享状态机设计

建议将服务端节点分享档案状态与发布任务状态拆开设计。

### 14.1 分享档案状态

推荐状态：

- `draft`
- `preview_ready`
- `published`
- `revoked`
- `disabled`

推荐状态流转：

```text
draft -> preview_ready
preview_ready -> published
published -> preview_ready
published -> revoked
revoked -> preview_ready
preview_ready -> disabled
disabled -> preview_ready
```

说明：

- `preview_ready` 表示当前至少存在一个可发布渲染版本
- `published` 表示当前已有对外可访问订阅地址
- `revoked` 表示当前对外分享已失效，但档案仍保留

### 14.2 发布任务状态

推荐状态：

- `created`
- `preparing_sources`
- `rendering`
- `issuing_token`
- `persisting_revision`
- `succeeded`
- `failed`
- `needs_intervention`

推荐状态流转：

```text
created
  -> preparing_sources
  -> rendering
  -> persisting_revision
  -> succeeded

created
  -> preparing_sources
  -> rendering
  -> issuing_token
  -> persisting_revision
  -> succeeded

created
  -> preparing_sources
  -> failed

created
  -> preparing_sources
  -> rendering
  -> failed

created
  -> preparing_sources
  -> rendering
  -> issuing_token
  -> needs_intervention
```

规则：

- 预览任务不进入 `issuing_token`
- 二维码导出和文件导出复用同一状态机，但 `job_type` 不同
- `needs_intervention` 只用于 token 已轮换但版本落库或审计写入失败等不一致场景

## 15. 服务端节点分享错误码建议

建议统一定义：

- `SHARE_PROFILE_NOT_FOUND`
- `SHARE_PROFILE_EMPTY_NODES`
- `SHARE_PROFILE_HTTPS_REQUIRED`
- `SHARE_PROFILE_NOT_PREVIEWED`
- `SHARE_RULE_SOURCE_FETCH_FAILED`
- `SHARE_RULE_SOURCE_NOT_MODIFIED`
- `SHARE_RULE_COMPILE_FAILED`
- `SHARE_RENDER_FAILED`
- `SHARE_QRCODE_UNSUPPORTED`
- `SHARE_QRCODE_GENERATE_FAILED`
- `SHARE_EXPORT_VERSION_NOT_FOUND`
- `SHARE_SUBSCRIPTION_ROTATE_FAILED`
- `SHARE_SUBSCRIPTION_REVOKE_FAILED`
- `SHARE_AUDIT_WRITE_FAILED`
- `SHARE_NEEDS_INTERVENTION`

建议 UI 映射：

- `SHARE_PROFILE_HTTPS_REQUIRED`：阻断发布和二维码按钮，并提示先完成 `HTTPS`
- `SHARE_PROFILE_EMPTY_NODES`：高亮节点选择区
- `SHARE_RULE_SOURCE_FETCH_FAILED`：允许重试并显示最近成功规则快照是否可复用
- `SHARE_QRCODE_UNSUPPORTED`：仅禁用二维码导出，不影响文件导出
- `SHARE_NEEDS_INTERVENTION`：冻结自动重试，要求管理员处理

### 15.1 `requires_action_reason` 枚举建议

建议固定枚举：

- `https_not_ready`
- `select_nodes_first`
- `publish_state_inconsistent`
- `manual_rule_fix_required`
- `token_rotate_partially_failed`
- `audit_write_failed`
- `render_version_missing`

建议 UI 映射：

- `https_not_ready`：主按钮显示“完成 HTTPS 配置”
- `select_nodes_first`：主按钮显示“去选择节点”
- `publish_state_inconsistent`：主按钮显示“查看任务详情”
- `manual_rule_fix_required`：主按钮显示“修复规则源”
- `token_rotate_partially_failed`：主按钮显示“检查订阅状态”
- `audit_write_failed`：主按钮显示“查看审计写入失败”
- `render_version_missing`：主按钮显示“重新生成预览”

### 15.2 事件类型枚举建议

建议统一枚举：

- `share_profile_created`
- `share_profile_updated`
- `share_profile_preview_started`
- `share_profile_preview_succeeded`
- `share_profile_preview_failed`
- `share_profile_publish_started`
- `share_profile_published`
- `share_profile_publish_failed`
- `share_profile_export_file_started`
- `share_profile_export_file_succeeded`
- `share_profile_export_file_failed`
- `share_profile_export_qrcode_started`
- `share_profile_export_qrcode_succeeded`
- `share_profile_export_qrcode_failed`
- `share_profile_revoked`
- `share_profile_revoke_failed`

### 15.3 服务端订阅额度与周期控制错误码

- `SUBSCRIPTION_QUOTA_EXCEEDED`：当前周期流量已用尽，拒绝新的代理连接
- `SUBSCRIPTION_DAYS_EXPIRED`：激活后有效天数已到期
- `SUBSCRIPTION_RESET_FAILED`：月度重置任务执行失败
- `SUBSCRIPTION_ALREADY_SUSPENDED`：订阅已处于暂停状态
- `SUBSCRIPTION_RESET_NOT_DUE`：当前未到重置时间，不允许执行自动重置

## 16. 服务端处理流程设计

### 16.1 `GET /sub/:token` 处理链

建议服务端按固定顺序处理：

1. 解析 token 与查询参数
2. 查询订阅记录
3. 校验订阅状态
4. 校验过期时间
5. 校验安全模式
6. 执行限速与风控
7. 记录访问日志
8. 生成目标客户端格式
9. 返回订阅内容

### 16.2 详细判定流程

推荐判定顺序：

```text
receive request
  -> token exists?
  -> subscription active?
  -> expired?
  -> revoked?
  -> binding valid?
  -> rate limit passed?
  -> risk score acceptable?
  -> render content
  -> write access log
  -> return response
```

设计原因：

- 先做便宜校验，减少资源浪费
- 先拦截非法请求，再做格式渲染
- 日志应记录命中结果和失败原因

### 16.3 返回策略

建议：

- 对无效 token 返回统一错误，不暴露是否存在真实订阅
- 对 revoked / expired / rate-limited 记录内部原因
- 外部错误文案保持简单

## 17. 风控与限速设计

### 17.1 限速层级

建议至少支持三层限速：

- 按订阅 token 限速
- 按来源 IP 限速
- 按全局系统限速

示例策略：

- 单 token 每分钟 30 次
- 单 IP 每分钟 120 次
- 系统总订阅出口限流保护

### 17.2 风控信号

可用于打分的信号：

- 单位时间访问次数异常
- 来源 IP 高频变化
- User-Agent 异常频繁变化
- 已知高风险地区访问
- 已吊销 token 继续访问

### 17.3 风控动作

建议支持：

- 仅记录
- 增加限速
- 临时禁用订阅
- 自动通知管理员
- 建议轮换订阅地址

### 17.4 设备或来源绑定策略

推荐只做“宽松绑定”，不要默认强绑定。

建议模式：

- `none`
- `soft_ip`
- `soft_user_agent`
- `device_id`

说明：

- `soft_ip`：IP 变化只告警，不立即封禁
- `soft_user_agent`：UA 变化只增加风险分
- `device_id`：适合已知可传固定标识的客户端

## 18. 缓存与性能设计

### 18.1 为什么需要缓存

订阅请求通常重复频繁，而配置内容变化不一定频繁。

建议缓存：

- 订阅基础信息
- 渲染后的订阅内容
- 风控临时状态

### 18.2 缓存键建议

可使用：

- `sub:meta:<token>`
- `sub:content:<subscription_id>:<format>:<version>`
- `sub:rate:<token>`

### 18.3 缓存失效时机

以下操作后必须清缓存：

- 订阅轮换
- 订阅吊销
- 订阅过期时间变更
- 节点或模板变更
- 目标核心格式变更

### 18.4 内容缓存策略

建议：

- 渲染内容可短时缓存
- 不缓存带明显用户敏感信息的调试输出
- 被吊销或过期时立即失效

## 19. 多格式渲染设计

### 19.1 渲染输入模型

建议统一从同一个订阅上下文生成：

```go
type SubscriptionRenderContext struct {
    SubscriptionID int64
    ShareProfileID int64
    ClientID       int64
    OutputFormat   string
    OutputCore     string
    DeliveryMode   string
    Nodes          []ProxyNode
    Policy         SubscriptionPolicy
    Template       TemplateConfig
    RuleSummary    SubscriptionRuleSummary
}
```

补充建议：

- `ShareProfileID` 用于回溯服务端节点分享档案
- `DeliveryMode` 用于决定返回文件、订阅链接还是二维码载荷
- `RuleSummary` 用于记录远程规则 URL、自定义规则和降级统计

### 19.2 渲染器接口建议

```go
type SubscriptionFormatter interface {
    Format(ctx SubscriptionRenderContext) ([]byte, string, error)
}
```

建议实现：

- `SingBoxJSONFormatter`
- `XrayJSONFormatter`
- `MihomoYAMLFormatter`
- `RawURIFormatter`
- `Base64AggregateFormatter`

### 19.3 多客户端兼容输出

输出时要考虑：

- 内容编码
- 文件名建议
- `Content-Type`
- 换行格式

建议：

- JSON 输出使用 `application/json`
- YAML 输出使用 `text/yaml` 或 `application/yaml`
- URI 聚合可用 `text/plain`

## 20. 模块分层设计

建议后端按以下模块拆分：

### 20.1 handler 层

负责：

- 参数解析
- 鉴权
- 响应包装

建议文件：

- `internal/api/handlers/server_share/create_profile.go`
- `internal/api/handlers/server_share/update_profile.go`
- `internal/api/handlers/server_share/get_profile.go`
- `internal/api/handlers/server_share/list_profiles.go`
- `internal/api/handlers/server_share/list_revisions.go`
- `internal/api/handlers/server_share/list_events.go`
- `internal/api/handlers/server_share/preview_profile.go`
- `internal/api/handlers/server_share/publish_profile.go`
- `internal/api/handlers/server_share/export_profile.go`
- `internal/api/handlers/server_share/export_qrcode.go`
- `internal/api/handlers/server_share/revoke_profile.go`
- `internal/api/handlers/subscriptions/create.go`
- `internal/api/handlers/subscriptions/list.go`
- `internal/api/handlers/subscriptions/rotate.go`
- `internal/api/handlers/subscriptions/revoke.go`
- `internal/api/handlers/subscriptions/fetch.go`

挂载建议：

- `/api/v1/server-share/*` 统一走管理鉴权中间件
- `GET /sub/:token` 不走管理鉴权，但走订阅风控、限速和访问日志中间件
- 导出二维码和导出文件接口继续走管理鉴权，不直接暴露匿名下载地址

### 20.2 service 层

负责：

- 创建订阅
- 签发 token
- 轮换
- 吊销
- 风控判断
- 格式渲染编排

建议接口：

```go
type SubscriptionService interface {
    CreateShareProfile(ctx context.Context, in CreateShareProfileInput) (*ServerShareProfile, error)
    PreviewShareProfile(ctx context.Context, id int64, in PreviewShareProfileInput) (*SharePreviewResult, error)
    PublishShareProfile(ctx context.Context, id int64, in PublishShareProfileInput) (*PublishShareProfileResult, error)
    GetShareQRCode(ctx context.Context, id int64, in GetShareQRCodeInput) (*ShareQRCodeResult, error)
    ExportShareProfile(ctx context.Context, id int64, renderVersion string) (*ShareExportResult, error)
    Create(ctx context.Context, in CreateSubscriptionInput) (*Subscription, error)
    Rotate(ctx context.Context, id int64, in RotateSubscriptionInput) (*SubscriptionRotateResult, error)
    Revoke(ctx context.Context, id int64, in RevokeSubscriptionInput) error
    Fetch(ctx context.Context, token string, req FetchSubscriptionRequest) (*FetchSubscriptionResult, error)
}
```

建议文件：

- `internal/service/server_share/profile_service.go`
- `internal/service/server_share/render_service.go`
- `internal/service/server_share/publish_service.go`
- `internal/service/server_share/export_service.go`
- `internal/service/server_share/audit_service.go`
- `internal/service/subscriptions/subscription_service.go`

职责拆分建议：

- `profile_service`：档案增删改查、节点勾选、规则源维护
- `render_service`：规则归一化、目标客户端编译、生成 `render_version`
- `publish_service`：任务状态推进、token 签发/轮换、版本绑定、失败处理
- `export_service`：文件导出、二维码导出、下载名和 `mime_type` 计算
- `audit_service`：审计事件写入、脱敏、事件查询装配

### 20.3 repository 层

负责：

- 服务端分享档案
- 渲染版本
- 发布任务
- 审计事件
- 订阅主表
- 访问日志
- 轮换历史
- 风控状态存储

建议文件：

- `internal/repository/server_share/profile_repository.go`
- `internal/repository/server_share/node_repository.go`
- `internal/repository/server_share/rule_source_repository.go`
- `internal/repository/server_share/render_revision_repository.go`
- `internal/repository/server_share/publish_job_repository.go`
- `internal/repository/server_share/audit_event_repository.go`
- `internal/repository/subscriptions/subscription_repository.go`
- `internal/repository/subscriptions/access_log_repository.go`

### 20.4 formatter 层

负责：

- 输出不同客户端订阅格式

建议文件：

- `internal/subscription/formatter/singbox_formatter.go`
- `internal/subscription/formatter/xray_formatter.go`
- `internal/subscription/formatter/mihomo_formatter.go`
- `internal/subscription/formatter/raw_uri_formatter.go`
- `internal/subscription/formatter/qrcode_payload_builder.go`

### 20.5 security 层

负责：

- token 生成
- 签名生成
- 风控判断
- 限速

建议文件：

- `internal/security/subscription/token_issuer.go`
- `internal/security/subscription/signature_verifier.go`
- `internal/security/subscription/rate_limiter.go`
- `internal/security/subscription/access_risk_evaluator.go`
- `internal/security/audit/server_share_audit_masker.go`

权限与脱敏建议：

- `server_share:*` 接口默认要求管理员权限
- “查看完整订阅地址”与“复制当前链接”应要求额外敏感操作权限
- `subscription_url_masked` 默认只返回脱敏值
- 审计日志中的 `token`、完整订阅 URL、二维码载荷必须默认脱敏存储

## 21. 输入输出结构建议

### 21.1 CreateSubscriptionInput

```json
{
  "client_id": 1001,
  "format": "mihomo_yaml",
  "security_mode": "token_only",
  "expires_at": "2026-12-31T23:59:59Z",
  "bind_mode": "none",
  "template_id": 10
}
```

### 21.2 CreateShareProfileInput

```json
{
  "name": "android-mihomo-share",
  "client_id": 1001,
  "target_client_type": "mihomo",
  "output_format": "mihomo_yaml",
  "delivery_mode": "https_subscription_qr",
  "node_ids": [1, 2, 9],
  "route_template_id": 12,
  "rule_sources": [
    {
      "source_type": "remote_url",
      "source_url": "https://example.com/rules/mihomo.yaml"
    },
    {
      "source_type": "custom_inline",
      "custom_content": "DOMAIN-SUFFIX,internal.example,DIRECT"
    }
  ]
}
```

### 21.3 PreviewShareProfileInput

```json
{
  "regenerate_rules": true,
  "include_qrcode": true
}
```

### 21.4 PublishShareProfileInput

```json
{
  "publish_mode": "issue_new_token",
  "invalidate_previous_token": true
}
```

### 21.5 RotateSubscriptionInput

```json
{
  "reason": "suspected_leak",
  "invalidate_old_token_immediately": true
}
```

### 21.6 FetchSubscriptionRequest

```json
{
  "ip": "203.0.113.10",
  "user_agent": "sing-box/1.0",
  "query_signature": "",
  "device_hint": ""
}
```

## 22. 错误码建议

建议统一定义：

- `SUBSCRIPTION_NOT_FOUND`
- `SUBSCRIPTION_EXPIRED`
- `SUBSCRIPTION_REVOKED`
- `SUBSCRIPTION_DISABLED`
- `SUBSCRIPTION_RATE_LIMITED`
- `SUBSCRIPTION_BINDING_MISMATCH`
- `SUBSCRIPTION_RENDER_FAILED`
- `SHARE_PROFILE_NOT_FOUND`
- `SHARE_PROFILE_EMPTY_NODES`
- `SHARE_PROFILE_HTTPS_REQUIRED`
- `SHARE_PROFILE_QRCODE_UNSUPPORTED`
- `SHARE_RULE_DOWNGRADED`
- `SHARE_EXPORT_VERSION_NOT_FOUND`

好处：

- 前后端联动清晰
- 审计和告警可直接识别

## 23. 审计建议

必须审计：

- 创建服务端分享档案
- 更新节点勾选结果
- 预览分享内容
- 发布分享订阅
- 导出二维码
- 导出分享文件
- 吊销服务端分享
- 创建订阅
- 修改安全模式
- 轮换订阅
- 吊销订阅
- 导出或查看完整订阅地址
- 高风险访问命中

审计内容建议：

- 操作人
- 订阅 ID
- 客户端 ID
- 旧状态
- 新状态
- 原因
- 时间

服务端节点分享审计补充：

- 必须额外记录 `profile_id`、`publish_job_id`、`render_version`
- 必须记录 `delivery_mode`、`target_client_type`
- 若存在降级，必须记录 `degrade_items` 与 `degrade_count`
- 若失败，必须记录 `error_code`、`error_message`、`requires_action`
- 二维码与文件导出需要记录 `export_kind`、`mime_type`、`file_name`

### 23.1 `detail_json` 固定字段表

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `profile_id` | integer | 是 | 服务端分享档案 ID |
| `publish_job_id` | integer | 否 | 关联任务 ID |
| `subscription_id` | integer | 否 | 关联订阅 ID |
| `render_version` | string | 否 | 渲染版本 |
| `delivery_mode` | string | 是 | 当前交付方式 |
| `target_client_type` | string | 是 | 目标客户端类型 |
| `selected_node_count` | integer | 否 | 已选节点数 |
| `degrade_count` | integer | 否 | 降级条目数量 |
| `degrade_items` | array | 否 | 降级条目数组 |
| `error_code` | string | 否 | 错误码 |
| `error_message` | string | 否 | 错误摘要 |
| `requires_action` | boolean | 否 | 是否需要人工处理 |
| `export_kind` | string | 否 | `file` 或 `qrcode` |
| `mime_type` | string | 否 | 导出内容类型 |
| `file_name` | string | 否 | 导出文件名 |

要求：

- `detail_json` 必须保持结构化对象，禁止只写一段自由文本
- 不得写入完整 token、完整订阅 URL、二维码原始载荷
- 若需要保留敏感字段，必须先做脱敏后再入库

### 23.2 订阅额度与周期控制审计补充

- 月度重置成功必须记录 `subscription_quota_reset_succeeded`
- 月度重置失败必须记录 `subscription_quota_reset_failed`
- 达到配额上限必须记录 `subscription_quota_exhausted`
- 激活后有效天数到期必须记录 `subscription_active_days_expired`
- 手动恢复或手动重置必须记录 `subscription_quota_restored`

额度控制审计字段补充：

- `quota_bytes`
- `used_bytes`
- `reset_cycle`
- `reset_day`
- `last_reset_at`
- `next_reset_at`
- `max_active_days`
- `activated_at`
- `quota_state`

## 24. 默认实现建议

如果现在就要落地开发，最推荐的第一版订阅实现是：

- 服务端分享档案与客户端订阅使用分开建模
- 服务端分享先支持 `sing-box`、`Xray-core`、`Mihomo`、通用 URI 四类目标
- 先支持“已选节点 + 远程规则 URL + 自定义规则”的统一渲染链
- 二维码第一版只支持单节点 URI 和 `HTTPS` 订阅链接
- 安全模式默认 `token_only`
- 默认固定 URL
- 支持过期时间
- 支持手动吊销
- 支持手动轮换
- 支持访问日志
- 支持请求限速
- 支持 `sing-box`、`Xray-core`、`Mihomo` 三种输出

第二版再扩展：

- 固定签名参数
- 宽松绑定策略
- 风控自动化
- 高风险告警

这样既能快速落地，也不会把客户端兼容性做坏。

## 25. 服务端订阅有效期、使用量与月度重置设计

### 25.1 生效时间规则

服务端订阅支持两套时间约束：

- 固定到期时间：使用 `expires_at`
- 激活后有效天数：使用 `activated_at + max_active_days`

规则：

- `activated_at` 以首次成功代理认证时间为准，不以首次访问订阅 URL 为准
- 同时配置 `expires_at` 与 `max_active_days` 时，取先到者失效
- 到期后不自动删除订阅记录，只更新状态并拒绝新代理连接

### 25.2 流量配额规则

规则：

- `quota_bytes = 0` 表示不限量
- `used_bytes_cache` 只做快速展示，真实口径以流量账本聚合为准
- 订阅下发流量、面板流量、证书流量不计入订阅代理配额
- 命中绕过规则的直连流量不计入订阅代理配额

### 25.3 月度重置规则

建议支持：

- `quota_reset_cycle = none`
- `quota_reset_cycle = monthly`

`monthly` 规则：

- 若显式设置 `quota_reset_day`，按每月固定日期重置
- 若未设置 `quota_reset_day`，默认按首次激活日对齐
- 若某月没有对应日期，如 31 号，默认取当月最后一天
- 每次重置后写 `last_reset_at` 和 `next_reset_at`
- 自动重置失败时保持旧用量不清零，并进入告警

### 25.4 超量停用规则

建议默认：

- 达到配额后拒绝新的代理连接
- 是否断开已建立连接由配额策略控制
- 默认不吊销订阅 URL，仍允许客户端继续拉取订阅内容
- 若管理员选择“超量即彻底停用”，才额外把订阅状态切到 `suspended`

### 25.5 手动重置与恢复接口建议

建议补充：

- `POST /api/v1/subscriptions/:id/reset-usage`
- `POST /api/v1/subscriptions/:id/restore`

用途：

- 手动清零当前周期用量
- 从 `suspended` 或 `expired_days` 恢复订阅
- 重新计算 `next_reset_at` 并写审计事件
