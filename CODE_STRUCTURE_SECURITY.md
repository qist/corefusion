# 开发代码目录结构与安全建议

> **实现状态（2026-08-01 修订）**：**RBAC 接口级权限校验、登录失败限流、Access/Refresh Token 分离、高危操作权限点（后端）、页面级/按钮级权限（前端）、管理端 mTLS 均已落地**（详见第 8 章、第 15 章）。仍未实现：2FA/TOTP、CLI 恢复命令。其中 **Token 撤销与强制下线经评估不作为本阶段必要项**——当前 Refresh Token 有效期较短（7d）且已与 Access Token 分离、登录失败已限流，Token 撤销（Redis 黑名单 / 用户级 Token 版本号）的边际收益有限；如需支持管理员强制下线可后续追加，`POST /auth/logout` 当前仍为空实现（前端丢弃 Token 即可）。**CSRF 防护明确不实现**——系统使用 Bearer Token 鉴权、无 Cookie 会话，CSRF 无攻击面，不适用。敏感字段加密（如充值卡号 AES-256-GCM）已落地。详见 `UNIMPLEMENTED_FEATURES.md` P3 安全能力项。

## 1. 文档目标

本文件用于细化项目的实际开发目录结构，并给出可直接落地的安全建议，覆盖以下范围：

- Go 后端代码目录
- 前端代码目录
- 多核心适配器目录
- 运行时目录
- 配置、证书、日志、数据库目录
- 认证与权限安全
- 配置与密钥安全
- 单节点运行时安全
- 证书与订阅安全
- 运行与发布安全

目标不是只给一个大概目录，而是让后续开发时每个模块都知道应该放在哪里。

## 2. 总体结构原则

目录设计必须遵循以下原则：

- 业务层和基础设施层分离
- 通用能力和核心适配器分离
- 读写模型和运行时文件分离
- 前端页面、组件、状态逻辑分离
- 安全相关代码集中管理，不分散到各业务模块
- 客户端与服务端可以独立运行，但仍由同一个可执行文件提供不同运行模式

建议采用：

- `cmd` 放程序入口
- `internal` 放后端主业务代码
- `pkg` 只放明确需要复用的公共库
- `web` 放前端源码与构建产物
- `deploy` 放部署模板和安装脚本
- `data` 放运行期数据

## 3. 推荐目录结构

```text
project/
  cmd/
    corefusion/
      main.go

  internal/
    app/
      bootstrap/
      container/
      lifecycle/

    api/
      middleware/
      response/
      request/
      router/
      handlers/
        auth/
        users/
        nodes/
        services/
        configs/
        subscriptions/
        runtime/
        binaries/
        certificates/
        metrics/
        networkpolicy/

    auth/
      jwt/
      password/
      otp/
      session/
      rbac/

    clientapp/
      bootstrap/
      profile/
      runtime/
      updater/
      tray/
      networkpolicy/

    domain/
      user/
      node/
      service/
      configprofile/
      subscription/
      certificate/
      runtime/
      binary/
      metric/
      networkpolicy/
      clientruntime/

    service/
      auth/
      user/
      node/
      service/
      config/
      subscription/
      runtime/
      binary/
      certificate/
      metric/
      networkpolicy/
      audit/
      clientruntime/

    store/
      db/
      query/
      repository/
      tx/

    model/
      entity/
      dto/
      vo/

    configgen/
      common/
      xray/
      singbox/
      mihomo/
      hysteria2/
      tuic/
      convert/

    runtime/
      process/
      caddy/
      nginx/
      binary/
      cert/
      logs/
      network/
      firewall/

    subscription/
      formatter/
      signer/
      template/

    security/
      crypto/
      secret/
      sanitize/
      rate_limit/
      audit/
      permission/

    platform/
      linux/
      windows/
      darwin/
      common/

    migration/
    scheduler/
    observer/
    region/
    constants/

  pkg/
    fileutil/
    hashutil/
    netutil/

  web/
    admin/
      src/
        api/
        router/
        store/
        views/
          dashboard/
          auth/
          users/
          nodes/
          services/
          subscriptions/
          certificates/
          runtime/
          binaries/
          network-policy/
          settings/
          audit/
        components/
          common/
          forms/
          network/
          certificate/
        composables/
        utils/
        styles/
        types/
        locales/
      public/
      dist/
    client/
      src/
        api/
        router/
        store/
        views/
          dashboard/
          profiles/
          runtime/
          network-policy/
          subscriptions/
          settings/
          security/
        components/
          common/
          network/
          profile/
        composables/
        utils/
        styles/
        types/
        locales/
      public/
      dist/

  deploy/
    caddy/
    nginx/
    systemd/
    init/
    docker/
    scripts/

  configs/
    examples/
    templates/
    cores/
    reverse-proxy/

  data/
    sqlite/
    certs/
    logs/
    cache/
    backup/
    runtime/
    binaries/
      active/
      staging/
      backup/
    update/
      staging/
      backup/

  docs/
  scripts/
  Makefile
  go.mod
```

## 4. 目录职责说明

### 4.1 `cmd`

用途：

- 放程序入口
- 不写业务逻辑
- 只负责启动、装配、参数解析

建议：

- `corefusion server` 启动服务端管理面板模式
- `corefusion client` 启动客户端模式
- `corefusion migrate` 执行数据库迁移模式

补充要求：

- 同一个可执行文件内支持多运行模式
- 服务端模式不依赖客户端托盘与本地运行逻辑
- 客户端模式不依赖服务端管理能力
- 各模式共享统一配置模型，但运行时模块分开装配

### 4.2 `internal/app`

用途：

- 应用生命周期
- 依赖注入
- 配置装配
- 程序启动顺序管理

不要放：

- 具体业务逻辑
- 协议适配逻辑

### 4.3 `internal/api`

用途：

- 路由定义
- 中间件
- 请求解析
- 响应封装
- Handler 控制层

要求：

- Handler 只做参数校验和调用 service
- 不直接操作数据库
- 不直接拼核心配置

### 4.4 `internal/domain`

用途：

- 领域对象
- 领域规则
- 领域接口定义

适合放：

- `ServiceProfile`
- `ClientProfile`
- `RoutePolicy`
- `CertificateBundle`

### 4.5 `internal/service`

用途：

- 业务编排层
- 调用 repository、runtime、configgen
- 实现事务和流程控制

示例：

- 创建服务
- 生成配置
- 签发证书
- 应用单节点运行时变更
- 应用网络策略

### 4.6 `internal/store`

用途：

- 数据库访问
- Repository 实现
- 查询封装
- 事务封装

要求：

- 所有 SQL 或 ORM 操作集中在此处
- 不允许在 handler 和 runtime 中直接操作数据库

### 4.7 `internal/configgen`

用途：

- 统一模型到不同核心配置的渲染
- 多核心配置导入与转换

子目录建议：

- `common` 放共享结构
- `xray`、`singbox`、`mihomo`、`hysteria2`、`tuic` 放各核心适配器
- `convert` 放跨核心转换逻辑
- `validate` 放协议、传输层、TLS 与入口组合校验器

要求：

- 每个核心只实现自己的适配逻辑
- 公共路由和网络策略在 `common` 统一表达

### 4.8 `internal/runtime`

用途：

- 进程托管
- 核心二进制管理
- 反向代理控制
- 证书部署
- 日志采集
- 防火墙和网络策略落地

要求：

- 所有运行时副作用集中在这里
- 不允许业务层直接执行系统命令

### 4.9 `internal/security`

用途：

- 加密
- 密钥管理
- 审计
- 权限校验
- 限流
- 输入清洗

这是必须单独存在的目录，不能零散分布。

### 4.10 `internal/platform`

用途：

- Linux、Windows、macOS 差异适配
- 服务托管差异
- 防火墙差异
- 路由与网卡差异

要求：

- 系统差异不要散落在业务代码中
- 所有平台分支逻辑放在这里

### 4.11 `web/src`

用途：

- 前端页面
- 组件
- API 调用
- 状态管理
- 样式和类型

建议：

- 前端拆成 `web/admin` 和 `web/client`
- `web/admin` 面向服务端管理面板
- `web/client` 面向客户端本地控制界面
- 网络控制组件单独一组
- 证书与安全组件单独一组

### 4.12 客户端与服务端独立运行建议

系统应明确拆为三类运行模式：

- 管理端模式：负责服务端管理、证书、订阅、监控与单节点运行时控制
- 客户端模式：负责本机核心运行、网络策略执行、订阅导入、配置切换
- 迁移模式：负责数据库迁移、校验与恢复辅助

要求：

- 共享统一领域模型和配置适配器
- 运行时依赖最小化，不让客户端强依赖服务端模块
- 服务端不强依赖客户端本地 UI 与托盘逻辑
- 数据目录和配置目录允许按模式分别部署
- 前端管理端界面与客户端界面可同时内嵌在同一程序中

## 5. 推荐开发落地结构

### 5.1 后端层级

推荐调用链：

`handler -> service -> repository/runtime/configgen -> platform/security`

禁止调用链：

- `handler -> db`
- `handler -> os/exec`
- `service -> gin.Context`
- `configgen -> database`

### 5.2 前端层级

推荐调用链：

`view -> business component -> composable/store -> api`

要求：

- 页面不直接写复杂业务判断
- 页面不直接拼装后端请求结构
- 网络策略编辑器与核心配置编辑器分开

## 6. 配置与运行目录建议

### 6.1 `configs`

建议存放：

- 配置模板
- 示例配置
- 生成后的临时配置
- 反向代理模板

不建议存放：

- 明文长期密钥
- 真正使用中的私钥长期副本

### 6.2 `data`

建议存放：

- SQLite 数据库
- 证书文件
- 运行日志
- 缓存
- 备份
- PID、锁文件、运行态文件

建议目录：

```text
data/
  sqlite/app.db
  certs/<domain>/
  logs/<service>.log
  runtime/
  backup/
```

### 6.3 文件权限建议

- `data/sqlite` 仅运行用户可读写
- `data/certs` 仅运行用户可读
- 私钥文件权限必须最小化
- 日志文件避免写入敏感明文

## 7. 安全建议总则

安全建议按“默认安全”原则设计：

- 默认拒绝
- 最小权限
- 最少暴露
- 敏感操作必须审计
- 所有高危变更可回滚

## 8. 认证与权限安全

### 8.1 认证

必须实现：

- ✅ 密码哈希存储，使用 `bcrypt` 或等效安全算法 —— 已实现，bcrypt cost=12（`internal/auth/password`）
- ✅ 支持账号密码修改 —— 已实现（`POST /api/v1/auth/change-password`）
- ❌ `2FA` 支持，优先 `TOTP` —— **未实现**
- ✅ Refresh Token 与 Access Token 分离 —— **已实现 2026-08-01**，见 8.2
- ✅ 登录失败限流 —— **已实现 2026-08-01**（`internal/api/middleware/loginlimit.go`）
- ✅ 登录日志记录 —— 已实现，登录成功/失败均写审计（`audit.Record`，category=`auth`）

登录失败限流实现说明：

- 按客户端 IP 计数，滑动窗口 15 分钟内累计 5 次失败即锁定 15 分钟
- 登录成功立即清空该 IP 的失败历史，避免误伤偶发输错密码的用户
- 超窗口的失败记录自动衰减，不会长期累积成封禁
- 状态为**进程内内存态**：管理面板（master/控制面）标准部署为**单副本**，多副本会引入状态一致性与任务重复执行问题，故进程内限流已足够拖慢撞库，阈值不会被放大；只有当"多面板实例 + 无 leader 选举 + 共享入口"时才需改为 Redis 后端，**评估后不作为本阶段必要项**

建议（暂未实现）：

- 新设备登录提醒
- 风险登录二次验证
- 可选设备信任

### 8.2 会话与 Token

要求：

- ✅ Token 必须设置过期时间 —— 已实现（access 24h / refresh 7d）
- ⚠️ Refresh Token 必须可撤销 —— **评估后不作为本阶段必要项**。当前 Refresh Token 有效期较短（7d）且已与 Access Token 分离、登录失败已限流，撤销（Redis 黑名单 / Token 版本号）边际收益有限；`POST /auth/logout` 目前仍为空实现（前端丢弃 Token 即可），后续如需管理员强制下线可追加
- ✅ 不要把长期凭据明文存在前端
- ⚠️ 管理端必须支持强制下线 —— **评估后不作为本阶段必要项**，理由同上；`POST /auth/logout` 当前为空实现，前端丢弃 Token 后即失效于客户端，服务端暂不维护黑名单

Access / Refresh Token 分离（2026-08-01）：

JWT Claims 新增 `typ` 字段区分令牌用途（`internal/auth/jwt/jwt.go`）：

- 此前两类 Token 结构完全相同，仅 TTL 不同 —— 意味着**泄露的 access token 可当作 refresh token 无限换取新凭据**
- 现在鉴权中间件强制要求 `typ=access`，刷新接口强制要求 `typ=refresh`，交叉使用返回 `ErrWrongTokenUse`
- 管理端（`service/auth`）与客户端（`service/clientauth`）两套 Token 体系均已加固
- **向后兼容**：升级前签发的 Token 无 `typ` 字段，仅在"要求 access"处被接受，避免升级即踢掉所有在线会话；而真正能换取新凭据的 refresh 一侧强制要求新字段

### 8.3 权限控制

要求：

- ✅ 使用 RBAC —— **已实现 2026-08-01**（`internal/api/middleware/rbac.go`）
- ✅ 接口级权限校验 —— 已实现，`middleware.RequireRole(minRole)`
- ✅ 页面级 / 按钮级权限控制 —— 已实现：前端 `Layout.vue` 按角色过滤菜单，`can(role)` 全局助手按角色收敛高危按钮（含非 super_admin 不可创建 super_admin 账户）；后端已拦截，UI 不再展示越权入口
- ✅ 高危操作单独权限点 —— 已实现：节点删除/核心控制/证书下发/数据同步、证书签发部署、CA 中心、备份恢复/删除/上传（super_admin）、系统升级/回滚/重启/迁移/生成加密密钥/重置 reality（super_admin）、日志接管、Portal 缓存、充值密钥轮转/CSV 导出、订单退款、返利提现审批、Agent 版本/发布查询等均已接入 `RequireRole` 分级校验

RBAC 模型说明：

采用**角色分级（rank）**而非完整权限矩阵，高等级隐含低等级的全部能力：

| 角色 | rank |
|---|---|
| `viewer` | 10 |
| `operator` | 20 |
| `admin` | 30 |
| `super_admin` | 40 |

- 未知角色 rank 为 0，**失败关闭（fail closed）**，不会被误判为高权限
- 上下文中缺少 `role` 视为 `AuthRequired` 未挂载的接线错误，返回 401 而非放行
- 选择分级而非 casbin：管理面接口数量有限，完整策略引擎引入的复杂度大于收益。若将来需要按资源粒度授权，应替换为策略表，而不是在此 map 上继续堆特例

已收敛的接口：

| 接口 | 最低角色 |
|---|---|
| `GET /api/v1/users` | `admin` |
| `POST /api/v1/users` | `super_admin` |
| `PUT /api/v1/users/:id` | `super_admin` |
| `DELETE /api/v1/users/:id` | `super_admin` |

高危操作示例（**以下多数尚未接入权限点，仅校验登录态**）：

- 删除服务
- 查看密钥
- 导出配置
- 停止核心进程
- 下发证书
- 修改网络策略

## 9. 配置与密钥安全

### 9.1 敏感数据存储

必须保护的数据：

- 用户密码哈希
- 节点密钥
- 私钥
- 订阅签名密钥
- API 密钥

建议：

- 敏感字段入库前加密
- 主密钥从环境变量或外部安全存储注入
- 不要把密钥硬编码进仓库

### 9.2 配置导入导出安全

要求：

- 导入文件前先校验格式和大小
- 导出配置时默认隐藏敏感信息
- 原始配置编辑器要做输入校验
- 配置变更前自动备份

### 9.3 日志脱敏

禁止直接写入日志：

- 私钥
- Token
- 完整密码
- 完整订阅地址

建议：

- 只记录掩码值
- 敏感字段统一通过脱敏工具处理

## 10. 单节点运行时安全

### 10.1 本机执行边界

要求：

- 所有系统命令都经 `runtime` 层统一下发
- 只允许执行预定义二进制与参数模板
- 不允许直接拼接用户输入执行 shell
- 运行时变更必须写入审计日志

### 10.2 最小权限

建议：

- 控制端使用独立系统用户运行
- 非必要不使用 root
- 涉及防火墙、网卡、端口绑定时单独定义提权边界
- 证书目录、配置目录、日志目录分别设置最小文件权限

### 10.3 单节点回滚保护

要求：

- 配置应用前先创建快照
- 证书部署失败时恢复旧版本文件与引用
- 反向代理 reload 失败时禁止保留半更新状态
- 核心重启失败时回退到最近一次成功配置

## 11. 核心二进制与供应链安全

### 11.1 二进制管理

要求：

- 下载后校验哈希
- 记录来源地址和版本
- 只允许安装白名单版本
- 支持回滚
- 支持在线版本索引与指定版本安装
- 支持多版本共存

### 11.2 供应链风险控制

建议：

- 固定依赖版本
- 定期扫描漏洞依赖
- 第三方二进制与库分开管理
- 发布前做完整校验

### 11.3 控制端自更新安全

要求：

- 控制端更新前必须创建数据库与关键配置快照
- 不直接覆盖正在运行的控制端二进制
- 更新包必须校验哈希与签名
- 升级任务必须记录完整阶段状态
- 升级失败必须自动回滚

建议：

- 使用版本目录与当前版本指针管理控制端
- 使用 `stable`、`beta`、`lts` 通道
- 升级前做版本兼容性检查
- 升级过程进入维护模式或短暂停机窗口
- 数据库迁移前做完整快照并检查可逆性
- 当前阶段控制端与核心升级遵守兼容矩阵
- 多主机与集群升级策略后续再设计

## 12. 证书安全

### 12.1 证书存储

要求：

- 私钥单独存储
- 文件权限最小化
- 证书目录不可开放浏览
- 所有下发操作必须审计

### 12.2 证书自动化

要求：

- 自动续期前先备份旧证书
- 更新失败时自动回滚
- 证书到期提前告警
- 禁止未校验域名就下发证书

补充要求：

- 默认通过 Go 原生 ACME 客户端（基于 `go-acme/lego`）执行 ACME 申请与续期，跨平台统一无外部依赖
- ACME 账号、DNS 凭据由面板集中管理并加密存储
- 不允许 `Caddy`、`nginx`、核心各自重复申请同域名证书
- 证书变更优先走动态加载或热重载
- 无法动态生效时才允许受控重启

## 13. 订阅与客户端配置安全

### 13.1 订阅链接

要求：

- 使用随机高强度 token
- 支持过期时间
- 支持手动吊销
- 支持访问日志
- 支持订阅地址轮换
- 支持签名校验或附加安全参数
- 支持限速与防爆破

建议：

- 订阅 token 长度足够且不可预测
- 不在日志中记录完整订阅地址
- 支持按用户、设备或来源策略限制订阅使用
- 订阅地址泄露后可立即轮换并失效旧地址

### 13.2 配置下发

建议：

- 用户默认只看到自己的配置
- 敏感导出操作需要二次验证
- 支持按用户、设备、核心分别下发

### 13.3 订阅地址安全边界

必须实现：

- 订阅接口请求限流
- 异常访问频率检测
- 订阅 token 可单独吊销
- 订阅地址轮换后旧地址立即失效
- 访问日志保留必要审计信息但避免泄露完整 token

建议：

- 支持订阅签名校验
- 支持来源 IP 或设备指纹辅助风控
- 对高风险订阅访问触发告警

## 14. 网络控制安全

### 14.1 策略变更

要求：

- 路由与防火墙变更要有预览
- 应用前显示差异
- 应用失败可回滚
- 高危网络操作必须二次确认

### 14.2 黑白名单模式

要求：

- 清晰显示当前是白名单还是黑名单模式
- 明确说明流量去向
- 防止用户误操作导致全局断网

## 15. Web 安全建议

必须实现：

- 输入校验
- 输出转义
- 统一错误响应
- 接口限流
- 文件上传限制
- CORS 严格控制
- 管理面板访问路径可自定义
- 首次安装完成后强制使用 `HTTPS`
- 管理端证书配置能力
- 管理端 `mTLS` 支持 ✅（已实现 2026-08-01 核查）：`cmd/corefusion/main.go` 读取 `mtls_enabled` + `mtls_ca_path`，开启时 `tls.Config.ClientAuth = tls.RequireAndVerifyClientCert` 并加载 `ClientCAs`，访问面板需客户端证书

补充说明：

- 自定义管理路径只能降低被简单扫描的概率
- 不能替代认证、`2FA`、RBAC、IP 白名单和 HTTPS
- 路径变更必须审计并支持回滚

### 15.1 首次安装访问策略

要求：

- 仅在未初始化完成时允许通过 `HTTP` 访问初始化向导
- 初始化完成后必须拒绝普通 `HTTP` 管理访问
- 首次初始化流程中必须完成管理员密码设置
- 若未配置管理端证书，则服务只允许监听内网地址或 `127.0.0.1`
- 仅在证书就绪后才允许正式对外提供 `HTTPS` 管理访问
- 是否启用 `mTLS` 可在初始化阶段或后续系统设置中配置

安全边界：

- 初始化接口数量必须最小化
- 初始化完成状态必须可靠持久化
- 不允许通过伪造请求重新进入未初始化模式
- 初始化完成后访问 `HTTP` 必须跳转或直接拒绝
- 无证书状态不得监听公网管理入口

### 15.2 CLI 恢复能力

必须提供本机 `CLI` 恢复命令，用于防止管理端被错误配置后无法进入。

建议支持：

- 修改管理端证书路径或重新导入证书
- 修改管理端登录账号
- 重置管理端登录密码
- 查看当前 Web 管理路径
- 重置管理路径为安全默认值

安全要求：

- `CLI` 恢复命令仅允许本机执行
- 高危恢复操作必须写入审计日志
- 密码重置后应提示失效旧会话

如果使用 Cookie 会话，还应实现：

- CSRF 防护
- `HttpOnly`
- `Secure`
- `SameSite`

## 16. 数据库与备份安全

### 16.1 SQLite

要求：

- 数据库文件放到受限目录
- 开启定期备份
- 备份文件加密或受限存储
- 不允许通过 Web 直接下载数据库文件

### 16.2 备份策略

建议：

- 每日自动备份
- 配置变更前增量备份
- 保留最近多个版本
- 备份恢复流程可演练

### 16.3 备份还原安全边界

要求：

- 核心配置文件支持版本化备份与单版本还原
- 数据库还原前必须二次确认
- 还原前自动创建当前快照
- 备份文件必须校验完整性
- 不允许未授权用户执行备份下载或还原

## 17. 运行与部署安全

### 17.1 服务运行

建议：

- 使用独立系统用户运行
- 不以 root 长期运行主进程
- 设置资源限制
- 关键目录做只读或受限挂载

### 17.2 部署环境

要求：

- 默认只监听必要端口
- 管理端口尽量不暴露公网
- 对管理面板增加 IP 白名单或反向代理访问控制
- 启用 HTTPS

### 17.3 服务端入口访问控制

除了管理面板访问控制，还必须对代理服务入口本身做访问控制。

必须实现：

- 服务入口 IP 新建连接限速
- 服务入口单 IP 并发连接限制
- 服务入口 IP 黑名单与白名单
- 服务入口自动封禁与定时解封
- 封禁与限速日志审计

建议：

- 支持按服务、入口、用户、订阅多维度叠加策略
- 支持 CIDR 级规则
- 支持短期封禁和永久封禁
- 将高风险 IP 命中情况接入告警
- 支持 GeoIP、ASN、CIDR 风控规则
- 访问控制采用状态机而不是简单 allow/deny
- 限速、封禁、解封都写入安全审计日志
- 高频安全事件接入统一告警中心
- `UDP` 与 `QUIC` 使用独立会话和限速策略
- 连接失败原因必须标准化分类

### 17.4 独立部署安全边界

当客户端和服务端独立运行时，必须遵守：

- 客户端默认不开放不必要的管理接口
- 服务端默认不包含本地 `TUN` 与托盘运行能力
- 客户端与服务端通信必须认证
- 本地客户端即使脱离服务端也应可安全运行
- 服务端即使没有客户端也应可单独完成服务管理与证书管理
- 虽然是同一可执行文件，但不同运行模式的监听端口、权限和数据目录必须隔离

## 18. 审计建议

必须审计的操作：

- 登录与登出
- 权限变更
- 节点新增删除
- 服务启停
- 配置导入导出
- 证书签发和续期
- 运行时配置应用与回滚
- 网络策略变更

审计日志至少包含：

- 用户
- 时间
- IP
- 操作对象
- 操作结果
- 变更摘要

告警建议：

- 支持通知渠道配置
- 支持去重窗口与静默期
- 支持恢复通知
- 防止同类高频事件造成告警风暴

## 19. 代码规范建议

### 19.1 后端

建议：

- 一个文件只放一个主要职责
- 一个目录只放同一类能力
- 错误统一包装
- 上下文透传 `context.Context`
- 不在业务代码里直接写平台分支

### 19.2 前端

建议：

- 页面只负责布局和组合
- 复杂逻辑放 `composables` 或 `store`
- 表单 schema 独立
- 常量和类型独立文件维护

## 20. 最终建议

本项目建议采用：

- 以 `internal` 为主的分层后端结构
- 以业务域划分的前端页面结构
- 以统一模型为核心的多核心配置适配结构
- 以 `security`、`runtime`、`platform` 三个独立目录管理安全、运行时和跨平台问题
- 以 `corefusion` 单一可执行文件支持 `server`、`client`、`migrate` 多模式运行

安全上必须坚持：

- 默认最小权限
- 敏感字段加密
- 高危操作审计
- 配置与证书可回滚
- 单节点运行时执行严格受控

如果后续开始搭代码，这份目录结构可以直接作为项目初始化蓝图。
