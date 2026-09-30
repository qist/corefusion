# CoreFusion 全量安全审计报告

- **审计日期**：2026-09-30
- **审计版本**：v0.12.257（commit 84157356，main）
- **审计方式**：4 个审计域并行（认证授权 / 注入与不可信输入 / 密码学与敏感信息 / 前端与门户），每条发现均要求给出"攻击者可控入口 → 危险落点"的完整证据链（source→sink，含当前文件:行号），置信度 < 0.80 的一律丢弃
- **结论**：发现 **HIGH × 2、MEDIUM × 4、LOW × 6**；无 SQL 注入、无命令注入、无伪造 Agent 接入面

---

## 一、发现清单

### HIGH

#### H-1 SSRF：geo 规则自定义源可探测内网 + 公开端点回读响应

| 项 | 内容 |
|---|---|
| 入口 | `POST /api/v1/geo-rules/sources/:id/files`（仅登录校验，未限 Admin）；`GET /api/v1/geo-rules/files/:id/download-file`（**完全公开**） |
| 落点 | `AddFile` 仅校验 http(s) 前缀（service.go:447）→ `downloadLocal` 直接请求任意 URL（service.go:588-597），该 HTTP client 无内网地址拦截、默认跟随重定向 → 响应落盘后可经公开 download-file 端点回读 |
| 危害 | 登录用户（含 viewer）可借面板网络位置探测内网服务（192.168.x.x / 169.254.169.254 元数据等）并回读内容 |
| 修复建议 | 下载改用项目现成的 `netutil.SafeFetchGET`（协议白名单 + 禁内网 + 禁重定向 + 限长）；`/geo-rules/files/:id/download-file` 增加鉴权 |
| 位置 | `internal/service/georule/service.go#L447`、`#L588`；`internal/api/router/router.go#L2932` |

#### H-2 越权（漏挂 RequireRole）：大量写/管理路由 viewer/operator 全可达

| 项 | 内容 |
|---|---|
| 根因 | `AuthRequired` 对 RankOf>0 一律放行（middleware/auth.go:183），viewer(10)/operator(20) 均通过；以下写/管理路由只挂了 AuthRequired 未挂 `RequireRole(RoleAdmin)` |
| 受影响端点 | `POST /runtime/apply`、`/runtime/rollback`、`/runtime/disable-proxy`（全节点下发/停机配置）；`/nodes` 增删改；`/clients` 续费/停用；工单 AdminReply/Assign；`PUT /referral/config`（调返佣比例）；`POST /charge-callback-retries/:id/retry`；礼品卡发放；门户模板激活 |
| 佐证 | 同仓 WAF/charge/node-servers 等敏感路由均显式要求 Admin（router.go:1408/1609/2299），证明策略意图是管理操作限 Admin |
| 修复建议 | 上述路由统一补挂 `RequireRole(RoleAdmin)`；建议顺带全量梳理一遍"写操作路由的角色矩阵" |
| 位置 | `internal/middleware/auth.go#L183`；`internal/api/router/router.go#L3398` 等 |

### MEDIUM

#### M-1 敏感信息暴露：/clients 接口回传订阅客户 bcrypt 密码哈希

- `entity.Client.Password` 的 json tag 为 `json:"password"`（internal/model/entity/client.go:42），列表/详情接口直接序列化实体返回；`/clients` 路由未限角色。
- 对照先例：面板管理员列表（handlers/users/handler.go:42-54）手动剔除哈希字段。
- **修复**：改为 `json:"-"` 或引入 VO 白名单。低成本高收益。

#### M-2 路径穿越：面板本地日志读取 log_path 任意文件读取

- 入口 `GET /log-manage/nodes/:serverId/logs/:type?log_path=`（Admin）→ handler.go:1058 → `TailLogWithType` 直接 `os.Open`（service/logmanage/tailer.go:27-31）；`resolveLogPath` 绝对路径直通、相对路径 Join 不拒 `..`（service.go:1155-1161）；`StreamNodeLog` 同。
- **对照**：Agent 端同类问题已修复（cmd/agent/log_handler.go:1133 `RejectTraversal`），面板侧漏修。
- **修复**：`resolveLogPath` 接入 `safepath.RejectTraversal` 或限制在 dataDir/logs 白名单内。

#### M-3 会话生命周期：API 改密不吊销 token_version

- `POST /auth/change-password` 仅更新密码哈希（service/auth/auth.go:151-168），未调用 `BumpTokenVersion` —— 改密前签发的旧 access/refresh token 在 TTL 内继续有效；CLI 管理路径已有"改口令同时 BumpTokenVersion"的意图注释（admin.go:177），API 路径违背该意图；订阅者（clientauth/service.go:372-389）同病。
- **修复**：改密成功后对相应用户/客户执行 BumpTokenVersion。

#### M-4 密钥派生：AES 加密密钥缺省回退主机名派生，可离线穷举

- `CF_SECRET_ENCRYPTION_KEY` 缺省时 key = SHA-256(主机名 + 固定盐)，无 KDF 工作因子、无每安装随机量（internal/security/crypto/secret_crypto.go:65-74）。
- 受保护数据：门户 TOTP 密钥、CA 私钥（runtime/agent/ca.go:193/239）、Agent 侧证书 DNS 凭据 meta.json（cmd/agent/acme_meta.go:40-43）。
- 可利用链：取得 DB/备份文件 → 按常见命名规律穷举主机名 → 解出 CA 私钥 → 伪造 Agent mTLS 证书接入 gRPC。前提依赖密文先行泄漏，故定级 MEDIUM(低)。
- **修复**：首启生成随机 key 写 .env（0600），复用 JWT_SECRET 的现成模式（cmd/corefusion/main.go:398-409）；主机名派生仅作历史数据迁移兜底并打印告警。

### LOW

#### L-1 路径穿越：备份恢复 filename 未做 Base/白名单

`POST /backups/database/restore`（super_admin）→ `req.Filename` 直接 `filepath.Join(dataDir,"backup","database",filename)` 不防 `../`（handlers/backups/handler.go:94）→ 作为恢复源读入执行。建议 `filepath.Base` + 扩展名白名单。

#### L-2 XSS（模板补丁覆盖缺口）：门户 M7 补丁可被绕过

`EnsureSecurityPatches` 仅补当前激活模板（template_security.go:139）；`Activate`（service.go:350-359）与 `UpdateActiveConfig`（service.go:411-419）落库前不补丁 → 激活旧模板可让工单消息恢复 `v-html` 原始 HTML 执行；且补丁为精确串匹配，`renderMd( msg.content )` 等写法变体即失配。影响边界：管理端为纯文本插值不受影响，受害者为门户工单查看者（偏自 XSS）。建议：Activate/UpdateActiveConfig 落库前统一调用 PatchTemplateSecurity，或渲染时对工单内容强制走转义渲染器。

#### L-3 输入校验：/system/config-file/:coreType 未校验核心类型

`filepath.Join(dataDir,"runtime",coreType+".jsonc")` 直读（handlers/system/handler.go:657-662），错误信息回显完整路径；受 `.jsonc` 后缀拼接限制危害有限。nodeserver 侧已有 `isValidCoreType` 先例（protocol_proxy.go:308），建议复用 `safepath.CheckName`。

#### L-4 会话生命周期：refresh 轮换无复用检测

`/auth/refresh` 换发新 token 对但不作废所提交的旧 refresh（无 jti/一次性存储，auth.go:87-118）→ 被窃取的旧 refresh 在 7 天 TTL 内可与新一轮并行使用且不可检测。建议：refresh 记录 jti 黑名单或缩短 TTL + 绑定设备指纹。

#### L-5 敏感信息暴露：OAuth 回调经 query 串下发长效 token

`GET /client-auth/oauth/:provider/callback` 302 至 `/p?access_token=..&refresh_token=..`（handlers/oauth/handler.go:200-201）；token 会进浏览器历史/访问日志/Referer。同仓 M10 安全基线已认定"长效 JWT 进 URL"不合格（clientauth/handler.go:391-393）。建议改 fragment 或一次性 code 交换。

#### L-6 敏感信息暴露：ACME 账号实体未打 json:"-"

`entity.RuntimeACMEAccount` 的 `EABMACKey/AccountKeyPEM`（internal/model/entity/runtime.go:197/200）无 `json:"-"`；当前未发现 API 直接回传该实体，属防回归加固。

---

## 二、已核实干净的面（抽查无发现）

| 面 | 结论 |
|---|---|
| JWT | HS256 + 首启随机 secret；typ 类型区分防 access/refresh 混用；alg/exp 由 golang-jwt v5 默认强校验；token_version 登出/踢下线真实生效 |
| SSE/下载票据 | 32B CSPRNG、60s 有效、一次性（先删后验防重放）、路径白名单失败关闭、绑定令牌版本；客户端票据强制 scope=client_download 并校验归属 |
| C 端鉴权 | client token role="client" 硬校验防管理端冒充；工单/订单/余额/充值/日志打包均按 client_id 归属校验 |
| 支付 | 充值回调 HMAC-SHA256 验签 |
| 口令 | 管理员/订阅者均 bcrypt cost 12 |
| 面板↔Agent | gRPC mTLS TLS1.3 + RequireAndVerifyClientCert + CRL；证书 OU 身份与 server_id 绑定、失败关闭，无法伪造接入 |
| SQL | 全参数化/GORM；清理表名先查白名单表非拼接 |
| YAML | 订阅导入用 SafeFetchGET（协议白名单+禁内网+禁跳转+限长）+ yaml.v3 解码到 map，无任意类型解码 |
| 命令执行 | 全部 exec.Command 无 shell 拼接；唯一 `sh -c` 参数为自身可执行路径 |
| 前端 XSS | 11 处 v-html 均为管理员自撰内容或经 DOMPurify；门户工单走转义渲染器（仅 L-2 所述补丁覆盖缺口） |
| 其它 | 无 postMessage/动态 script 注入；订阅端点纯文本输出且邮箱脱敏；TOTPSecret/bootstrap_token 均 json:"-"；日志未打印明文 token/密码 |

**排除项说明**：`registerPlaceholderRoutes` 已更名 `registerAuxiliaryRoutes`（84157356）——其内三组路由全部真实现，非占位；subscription security_mode "未实现"为故意 fail-close 拒写入（安全设计）；agent 侧 gRPC Unimplemented 为生成代码，前端走 HTTP 真实现。

---

## 三、修复优先级

| 优先级 | 项 | 说明 |
|---|---|---|
| P0 立即 | H-1、H-2 | SSRF 改 SafeFetchGET + 回读端点鉴权；写/管理路由批量补挂 RequireRole |
| P1 本周 | M-1、M-2、M-3 | 密码哈希 json tag（一行）；日志路径复用 RejectTraversal；改密吊销 token_version |
| P2 计划内 | M-4、L-1 ~ L-6 | 随机加密密钥迁移、门户补丁覆盖 Activate/UpdateActiveConfig、其余加固项 |

---

*审计方法：四域并行 + 统一证据链标准（入口→落点、当前行号、置信度 ≥0.80、排除 DoS/依赖/测试代码/日志伪造）。发现基于审计时点代码，后续提交可能改变相关行为。*
