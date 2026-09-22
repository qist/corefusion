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
| 7 | **外部出口整条路径不可用**：向导能选、API 能过，但生成的是**另一个节点**的配置（或直接查不到） | 出口引用的是 `external_node_servers.id`，生成器却把它当 `external_nodes.id` 用 → **少了一跳**。之所以长期没被发现：既有的那条注册记录 `id=1 → external_node_id=1` **两个 id 恰好相等**，按旧逻辑查 `external_nodes.id=1` 也"碰巧对" | 真机：注册 `external_node_id=3`（hy2）→ 生成的出站 `server` 是 hy2 节点的地址（修复前会取到 id=2 的 vless 节点）| 本次 |
| 8 | 外部 hy2 出站**不带 `tls`** → 节点起不来 | mihomo 的 hy2 映射**没有 `tls` 键**（hy2 天生 TLS，mihomo 不写），而 `buildStreamFromProxyMap` 只看 `pm["tls"]` → `security` 为空 | 面板产物 `sing-box check` 通过；把它去掉 `tls` 后 → `FATAL initialize outbound[6]: TLS required` | 本次 |
| 9 | 外部 hy2 的 **obfs 被静默丢弃** | mihomo 写的是**扁平**的 `obfs: "salamander"` + `obfs-password`，旧实现只找嵌套 map | 真机产物含 `"obfs":{"type":"salamander","password":…}` | 本次 |
| 10 | 外部 hy2 的**端口跳跃**失效 | mihomo 用 `ports: "40000-50000"` 且 `port=0`，旧实现只读 `port` → 生成 `server_port: 0`（`check` 不报错 ✗、运行期连不上 ✗） | 真机产物为 `"server_ports":["40000:50000"]` 且**无** `server_port` | 本次 |
| 11 | 外部 **reality** 节点退化成普通 TLS → 连不上 | `reality-opts` 被完全忽略 | 单测断言 `security=reality` + `realitySettings{publicKey,shortId,serverName,fingerprint}` | 本次 |
| 12 | 入口 xray + 外部 hy2 **被 API 接受并落库**（只有生成器报错） | 外部出口分支**只有存在性校验**，没有协议校验 | 修复后 API 返回 `EXIT_PROTOCOL_UNSUPPORTED`（带外部节点协议与支持名单） | 本次 |
| 13 | sing-box 入口链到 **gRPC / ws** 出口时，**服务名 / 路径被静默丢掉** | xray 把它们放在**嵌套**位置（`grpcSettings.serviceName` / `wsSettings.path`），转换器只读顶层 ✗（两个字段都可选 → `check` 照样通过 ✗） | gRPC 出口产物含 `"service_name"`、ws 出口含 `"path"`+`"headers":{"Host":…}` | 本次 |
| 14 | 修 #13 时先写成 `transport.host` → **核心起不来** | sing-box 的 ws transport **没有 `host` 字段** | `FATAL decode config: outbounds[6].transport.host: json: unknown field "host"` → 改 `headers.Host` 后通过 ✓ | 本次 |
| 15 | **节点起不来**：一台机器同时跑两个核心时，sing-box 专用协议（hy2）的出站被塞进 **xray** 配置 | 链式出站**按节点**生成，没有按"入口入站的核心"过滤 ✗ | 修复前 xray：`failed to build outbound config with tag chain-118-1 > … unknown config id: hysteria2` ✗；修复后 xray 只含 `chain-4-2(vless)`、sing-box 只含 `chain-118-1(hysteria2)`，`xray run -test` → `Configuration OK.` ✓ | 本次 |
| 16 | 同机多入口时，给入口 A 建链会**静默清掉入口 B 的链**（B 的流量回落 `route.final` 直连 ✗） | 建链前的清理按 `entry_server_id` 整台服务器清 ✗，而设计是"**按入口隔离**"（§1183），读取侧 `ListByEntry` 也是按 server+inbound | 修复后同机 xray 入站 4 与 sing-box 入站 118 的链**共存**（DB 里两条 ✓） | 本次 |

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

**外部 reality / gRPC 出口**：本机 xray 入口 4 + 外部 `vless-reality-grpc` 节点（`external_nodes.id=5`）→
xray 产物含 `"security":"reality"` + `"realitySettings":{"publicKey","shortId","serverName":"nuvexo.com",
"fingerprint"}` + `"grpcSettings":{"serviceName":…}` → **`xray run -test` → `Configuration OK.`** ✓；
同一条出口给 sing-box 入口用时 → `"tls":{"reality":{"public_key","short_id"},"server_name":"nuvexo.com",
"utls":{…}}` + `"transport":{"type":"grpc","service_name":…}` → `sing-box check` 通过 ✓。

**按核心隔离**：本机同时跑 xray 与 sing-box，链分别指向 vless（入口 xray 入站 4）与 **hy2**（入口 sing-box
入站 118）→ `xray.jsonc` 只含 `chain-4-2(vless)`、`singbox.jsonc` 只含 `chain-118-1(hysteria2)` ✓，
两个核心都能通过真实核心校验 ✓（修复前 xray 会因 hy2 出站直接起不来 ✗）。

**外部出口（经 `external_node_servers` 注册）**：把外部 hy2 节点（`external_nodes.id=3`）注册后选为
sing-box 入口的出口 →

```json
{"tag":"chain-118-ext2","type":"hysteria2","server":"rem5ogoe5m.swsixtwo.top",
 "server_ports":["40000:50000"],
 "password":"fdcfb24d-…",
 "obfs":{"type":"salamander","password":"mrMdT3L9AH2VVwPI"},
 "tls":{"enabled":true,"server_name":"rem5ogoe5m.swsixtwo.top","alpn":["h3"],
        "utls":{"enabled":true,"fingerprint":"chrome"}}}
```
→ `sing-box check` **通过** ✓；反例（xray 入口 + 同一个外部 hy2 出口）→
`EXIT_PROTOCOL_UNSUPPORTED` ✓

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

# 2) 配置体检：把 data/runtime/ 下的配置交给**真实核心** dry-run（一键，推荐先跑这个）
python3 scripts/check_configs.py
#    对每个 <core>.jsonc 跑对应核心的 dry-run，并列出该配置里的链式出站清单
#    （便于一眼看出"某条链的出站跑到了另一个核心的配置里"）
#    退出码：有 FAIL → 1；hysteria2/tuic 无 dry-run 能力 → SKIP（带原因）
python3 scripts/check_configs.py --runtime-dir data/xxx --data-dir data   # 校验副本/临时目录

# 3) 手动逐核心校验（与体检脚本等价的原始命令）
cd data && ../data/binaries/active/singbox/singbox-linux-amd64 check -c runtime/singbox.jsonc
cd /opt/corefusion && ./data/binaries/active/xray/xray-linux-amd64 run -test -c data/runtime/xray.jsonc
#   ⚠ xray 的资源目录要用 -c 同目录或设 XRAY_LOCATION_ASSET=…/rules，否则会因 geosite 路径报错

# 4) 判断"流量真的走了出口" —— 只看出口节点的入站计数（0 值会被省略）
ssh <exit-node> 'X=/opt/corefusion-agent/data/binaries/active/xray/xray-linux-amd64; \
  $X api statsquery --server=127.0.0.1:10085 -pattern "inbound-vless-<port>"'
#   ✗ 不要用出口 IP 归属地判断：同机房/同 NAT 的节点出口 IP 可能完全相同

# 5) i18n 守卫（有缺失或两语言包不对称会以退出码 1 失败）
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
| 外部节点出口的支持面 | **已真机验证**：hy2（含端口跳跃 / obfs / tls）✓、reality 参数派生 ✓；vless/vmess/trojan 本来就走通 | 已闭环 |
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
