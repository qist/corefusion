# 链式代理 v2.0 交付报告（2026-09-22）

> 本文是**交付/验收**文档：说明 v2.0 的语义变更、按核心的协议矩阵、本轮修掉的真问题、
> 真机验收证据、以及**怎么复现验证**与遗留项。
> 设计细节见 `docs/design/chain-proxy-redesign.md`（已同步到 v2.0）。

---

## 1. 背景与目标

旧链式设计让面板在**出口节点上自动创建 relay-in 入站**（`chain_exit_configs`），存在两个问题：

1. 会与出口节点**已有**的配置/端口产生干扰（"链式一开就把出口弄乱"）；
2. 无法复用出口上已经配好、带 TLS/REALITY 的入站 —— 而绝大多数场景下，用户想要的正是
   "**入口收流量 → 交给某节点上那个已经能用的入站出去**"。

v2.0 的目标：**入口与出口都从"节点上已有的入站"里选，链式这边不创建任何入站**。

---

## 2. 语义变更（v2.0 核心）

| 项 | 旧 | 新（v2.0） |
|---|---|---|
| 出口来源 | 面板在出口节点**自动创建** relay-in 入站 | 选出口节点上**已有的入站**（`chain_links.exit_inbound_id`） |
| 出口配置表 | 必需 `chain_exit_configs` | **已彻底移除**（表也 drop 了，见 §6） |
| 出站参数 | 由 relay 配置推导 | 由**真实入站**派生：协议 / 端口 / 传输 / 安全层 / 凭据（xray 与 sing-box 两种入站形状都支持） |
| 入口侧 | 已经是"选现有入站" | 不变（两侧语义对齐） |

---

## 3. 协议矩阵：必须按「入口核心」分开

实测（**不是照文档抄的**）：

```
$ xray run -test -c hy2_outbound.json
Failed to start: … failed to load outbound detour config for protocol hysteria2
              > infra/conf: unknown config id: hysteria2        ✗   xray 没有 hy2 出站

$ sing-box check -c hy2_outbound.json
（无输出 = 通过）                                              ✓
```

| 入口核心 | 可用作链式出口的协议 |
|---|---|
| **xray** | `vless` / `vmess` / `trojan` / `shadowsocks`（**无 hy2** ✗） |
| **sing-box** | 上述 4 种 **+ `hysteria2`** ✓（hy2 入站侧还支持 reality） |

sing-box 侧 hy2 的两条硬约束（缺任一 → 核心 `FATAL` 起不来）：

- **必须带 `tls`**：缺 → `FATAL initialize outbound: TLS required`
- **配 reality 时必须带 `uTLS`**：缺 → `FATAL uTLS is required by reality client`
- `up_mbps` / `down_mbps` **可省略**（链式拿不到带宽值）；`obfs`（salamander）原样带过

**双层拦截**：后端 `agent.ChainSupportedProtocols(coreType)` 是唯一权威；前端 `CHAIN_PROTOCOLS`
镜像它 → 入口是 xray 时 **hy2 不出现在候选里**，即使绕过 UI 也会被 API 拒绝：

```
EXIT_PROTOCOL_UNSUPPORTED
exit inbound 120 protocol "hysteria2" is not chain-capable for entry core "xray"
  (supported: vless/vmess/trojan/shadowsocks)
```

---

## 4. 多出口策略

- 每个出口可设 `priority` / `weight` / `sort_order`（向导里逐出口可配）
- **同优先级** → `balancer`（`leastPing`）+ `burstObservatory`（负载均衡）
- **不同优先级** → 多条 `field` 规则按优先级排序（failover）
- 出站 tag 可用 `exit_tag` 自定义（留空则自动 `chain-<入口入站ID>-<出口ID>`）
- 拓扑图 / 链路 / 健康状态**每 30 秒自动刷新**（弹窗打开或页面在后台时跳过）

---

## 5. 本轮修掉的真问题（都经真机或真实核心验证）

| # | 现象 | 根因 | 证据 | 提交 |
|---|---|---|---|---|
| 1 | 外部出口若是不支持的协议，会生成**空 settings** 的出站 → 入口节点核心起不来且无提示 | `buildExternalOutbound` **完全没有协议校验** | 补校验 + 显式报错（列出支持名单） | 上游提交链 |
| 2 | hy2 出站**不带 tls** → `FATAL TLS required` | sing-box 形状的 TLS/REALITY 探测被写在 `else if cfg["transport"]` 里，而 **hy2 没有 transport** → 整段跳过 | 修复前 `FATAL`，修复后 `sing-box check` 通过 | `02382020` |
| 3 | REALITY 链路**时通时不通**（靠服务端容忍空 shortId） | 派生时只查 `shortId`，而服务端是**数组** `shortIds` | 真机出站补齐 `"shortId":"28da759cb18a710e"` | `cb4d652b` |
| 4 | 自定义 `exit_tag` 后，**流量静默绕过出口直连** ✗ | balancer `selector` 用自动前缀 `chain-<入站ID>-`，自定义 tag 时**圈不到任何出站** → 运行期落到 `fallbackTag: direct` | 真机 `selector` 变为 `["lb-exit-A","lb-exit-B"]`；回归测试 `chain_selector_test.go` | `db192ef4` |
| 5 | sing-box 入口节点**核心起不来** | 传输名 `raw`（xray 对 TCP 的新叫法）被**直接透传**给 sing-box → `unknown transport type: raw` | 修复后本机配置 `sing-box check` 通过 | `db192ef4` |
| 6 | 改了链式配置**不生效**（节点核心从不重载） | `reload_strategy=None` 的语义是"只写文件、不碰进程"，而所有节点都是 None → 配置变更静默无效 | Agent 日志 `config changed → restarted core to apply it`；核心启动时间随变更刷新 | `3664ccd0` |

> 其中 #4 与 #5 是**上一轮"端到端通过"结论的错误来源**：当时看到的出口 IP 其实是入口节点自己的
> 公网 IP（同机房节点的出口 IP 完全相同 ✗），流量实际是**直连**出去的。
> 教训：判断"流量真的走了出口"**只能看出口节点的入站计数**（见 §8）。

---

## 6. 清理项（旧模式彻底移除，`17bb1493` / `ed20abd1` / `f30f1528`）

- **实体/仓库**：`entity/chain_exit_config.go`、`repository/chain_exit_config.go`（删除）
- **生成与配置**：`GenerateExitInbound` / `IsRelayOnlyServer` / `GenerateChainExitInbound`；
  `prepare.go` 的 8c/8d（注入 relay-in 入站 + relay_only 过滤用户入站）与两个辅助函数
- **API**：5 个 exit-config handler + 5 条路由；`GET /node-servers/:id/chain-exit-config`；
  集群出口未指定 `exit_inbound_id` → `EXIT_INBOUND_REQUIRED`
- **同步/迁移**：面板与 Agent 的 chain_exit_configs upsert/剪枝；旧迁移文件
- **proto**：`SyncDataRequest.chain_exit_configs`(26)、`ChainExitConfigEntry`、
  `SyncDataResponse.chain_exit_configs_applied`(12) —— 删除并按惯例 **`reserved` 字段号与名字**，
  用**同版本** protoc 插件重新生成
- **数据库**：`chain_exit_configs` 表 **drop**（幂等迁移，删前 0 行）
- **前端**：出口配置卡片、出口弹窗、5 个 CRUD 函数、i18n 键；拓扑的 relay_only 列

**行为变化（需知悉）**：`BillingFilter` 与订阅侧的 `relay_only` 判断一并移除 →
"纯出口节点"概念消失，节点是否收用户/进订阅只看它自己的 `node_type` + `billing_enabled`。

---

## 7. 真机验收证据

**拓扑**：入口 = 节点2（192.168.2.50）**已有**的 vless/REALITY 入站 :31164；
出口 = 节点13（192.168.2.176）**已有**的 vless 入站 :10087。

```
客户端 → 节点2 入口 → 链式 → 节点13 出口 → 出网

基线 → 打流后：
  节点2  入口入站计数:   0 → 14827     ✓
  节点13 出口入站计数:   0 →  9383     ✓
出口 IP: 58.63.60.98 (CN)    vs    面板宿主直连 23.254.233.185 (US)   ✓
```

其它已验证：

- **配置生成**：loadbalance（balancer + burstObservatory）/ failover（两条 `pri` 规则）均核对
- **hy2 链路**：sing-box 入口 + hy2 出口 → 出站含 `obfs` + `tls{server_name,alpn}` → `sing-box check` 通过
- **旧 agent 兼容**：入口节点曾是 v0.12.17，面板下发 `config_content` 的方式向后兼容
- **删链自清理**：删除链路后，入口节点的运行时配置里链式出站**残留 0**

---

## 8. 怎么复现/回归验证

```bash
# 1) 单元测试（含真机踩过的坑的回归防线）
go test ./internal/runtime/agent/ ./internal/service/coreconfig/ ./internal/subscription/sharelink/

# 2) 用**真实核心**校验生成的配置（关键：比单测更硬的证据）
cd data && ../data/binaries/active/singbox/singbox-linux-amd64 check -c runtime/singbox.jsonc
cd /opt/corefusion && ./data/binaries/active/xray/xray-linux-amd64 run -test -c data/runtime/xray.jsonc
#   ⚠ xray 的资源目录要用 -c 同目录或设 XRAY_LOCATION_ASSET=…/rules，否则会因 geosite 路径报错

# 3) 判断"流量真的走了出口" —— 只看出口节点的入站计数（0 值会被省略）
ssh <exit-node> 'X=/opt/corefusion-agent/data/binaries/active/xray/xray-linux-amd64; \
  $X api statsquery --server=127.0.0.1:10085 -pattern "inbound-vless-<port>"'
#   ✗ 不要用出口 IP 归属地判断：同机房/同 NAT 的节点出口 IP 可能完全相同

# 4) i18n 守卫（有缺失或两语言包不对称会以退出码 1 失败）
cd web/admin && npm run i18n:audit
```

**改完 Go 代码必须重建面板二进制再验证** ✗ —— 否则会拿旧二进制的产物当成新代码的结果
（本项目踩过一次）。

---

## 9. 遗留与建议

| 项 | 说明 | 建议 |
|---|---|---|
| xray 侧 P1（11 项）/ P2（13 项） | 进阶调优 / finalMask 内部实现细节 | **不建议暴露** —— 配错往往让核心起不来 |
| sing-box 侧剩余项 | 见 `docs/singbox-field-gap.md` §3，多为客户端专属 | 按清单按需补 |
| 外部节点出口的支持面 | 已支持 hy2（代码 + 单测就位）；缺一台 hy2 外部节点做真机验证 | 有环境时补验 |
| 测试环境 | 面板宿主机上有历史遗留进程（用户自己的 `server3.json` 等），且核心为 file mode（访问日志不更新） | 数据面验证建议用"核心由面板管理、且节点已升级新 Agent"的组合 |

---

## 10. 相关提交（按时间）

```
30fb9893  后端：选出口已有入站 + 按核心协议矩阵 + hy2 支持
02382020  前端向导 + 修 hy2 出站缺 tls
95b8f8f8  设计文档更新到 v2.0
64b487fe  leastLoad/leastPing 缺 observatory 兜底
6233ac45  xray 清单" A 类丢字段"结论作废 + 白名单回归测试
40c462ab  清单措辞修正（configgen 已作为死代码删除）
6e3b4381  i18n 全量审计与修复（26 键）+ 审计守卫
37535d54  链式缺口第一批（外部出口协议校验 + 编辑走 updateLink）
3fb021bf  链式缺口第二批（每出口参数 + 外部 hy2）
17bb1493  彻底移除旧模式（−1181 行）
ed20abd1  proto 残留清理并重新生成
9d02729e / d6e9ac30  /opt/sing-box 来源更正（官方仓库，非 fork）
a0bd35cd  exit_tag / 自动刷新 / 编辑态说明
cb4d652b  派生 REALITY 参数时补 shortId
db192ef4  balancer selector + sing-box 传输名（两个真 bug）
3664ccd0  reload 语义修正：配置变了就重启
f30f1528  删除 chain_exit_configs 旧表
```
