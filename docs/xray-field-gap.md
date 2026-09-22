# Xray 字段级差异清单（基准：Xray-core v26.9.9 / commit `52a412d9`）

> 目的：找出"xray 支持、但面板配不出来 / 配了会丢"的字段，作为补齐 UI 与生成器的依据。

## 0. 方法与口径

| 项 | 说明 |
|---|---|
| 基准源码 | `git clone https://github.com/XTLS/Xray-core` → `git checkout 52a412d9`（**与面板正在跑的二进制同 commit**：`Xray 26.9.9 … 52a412d`） |
| 字段抽取 | `infra/conf/*.go` 全部配置结构体：**34 个文件 / 746 个字段**（含 json tag） |
| 面板侧来源 | 前端编辑器 `web/admin/src/**`（`views/protocols/sections/XrayInbounds.vue` 4850 行 + `XrayOutbounds.vue` 2237 行 + `xray/protocol/*.vue`）、后端 `internal/**` |
| 比对方式 | 字段名矩阵（全量 blob 检索）+ 人工抽查 17 个可疑项 |

### 两条生成路径（**关键区别**，决定了字段会不会丢）

- **P1 面板自建入站**：`internal/service/coreconfig/prepare.go` 把 DB 里的 `config_json` 反序列化成
  `map[string]interface{}` 后**原样透传**；配套的 `CleanInboundFields` 只做三件事：注入
  `port`/`protocol`（xray）/ `listen_port`/`type`（sing-box）、删除**以 `_` 开头的编辑器辅助字段**
  （`_realityPublicKey`、`_utlsFingerprint`、`_accessDomain*`、`_portHop*` …）、去掉 clients 里的 `name`。
  → **不会丢真配置字段** ✓
- **P2 强类型生成器**：`internal/configgen/xray/generator.go` 用 **Go struct** 构造配置
  （`InboundSettings{Clients,UDP,Auth,Password,Email,Encryption,Decryption,Method,Network}`、
  `StreamSettings` 只建模 `Network/Security/TLS/Reality/WS/GRPC/HTTP` 的基础字段、
  `Sniffing{Enabled,DestOverride}`）→ **没建模的字段静默消失** ✗。服务配置、客户端配置走这条。

---

## 1. A 类：P2 路径会丢的字段（P1 不受影响）

> 判定 = 前端编辑器**能产生**、但后端强类型 struct / 生成逻辑里**完全没有**。

| 字段 | 所在 | 影响 |
|---|---|---|
| `xPaddingKey` `xPaddingHeader` `xPaddingMethod` `xPaddingPlacement` | xhttp | XHTTP 的**流量填充**参数（抗特征），面板生成的服务/客户端配置发不出 |
| `seqKey` `seqPlacement` `sessionIDKey` `sessionIDPlacement` | xhttp | XHTTP 会话/序号字段名定制 |
| `uplinkDataKey` `uplinkDataPlacement` `uplinkChunkSize` | xhttp | XHTTP 上行分块参数 |
| `statsUserOnline` | policy | 在线用户统计（面板用自己的在线统计，但策略里发不出） |
| `profiles` | finalMask | 掩码配置档 |
| `proxySettings` | 顶层 Config | 全局代理设置（下发/更新用的代理） |
| `idKey` | JSONConfigLoader | 多配置加载标识 |

**修法（二选一）**：
1. 扩展 `configgen/xray` 的 struct（覆盖到 httpupgrade/xhttp/sockopt/mux 等），或
2. 把 P2 路径改成 **map 透传 + 只做必要归一化**（与 P1 一致），一次性消灭这类缺口。

---

## 2. B 类：面板完全配不了（前端与后端都没有，共 39 项）

### P0 —— 真实用户会用到，建议优先补（15 项）

| 字段 | 所在 | 说明 / 为什么重要 |
|---|---|---|
| `targetStrategy` `applyTo` `ipsBlocked` | freedom 出站 | 直连出站的**域名解析策略**与作用范围、按 IP 段阻断；`targetStrategy` 还出现在 `xray.go` 的 outbound |
| `sendThrough` | 顶层 / 出站 | **指定出口源地址**（多线/多 IP 机器常用，链式代理也需要） |
| `xudpConcurrency` `xudpProxyUDP443` | 出站 | XUDP 的 UDP 并发与 443 代理开关（**UDP over TCP 的常用调优**） |
| `expectIPs` | dns | 域名期望 IP（防 DNS 污染的关键手段） |
| `blockTypes` `nonIPQuery` | dns_proxy | 查询类型黑名单 / 非 IP 查询处理 |
| `fakeDNSInUse` | fakedns | 与 FakeDNS 组合使用时的标记 |
| `customResponseData` | blackhole | 黑洞出站返回自定义数据（探测对抗） |
| `penetrate` | sockopt | 透明代理穿透相关 |
| `remoteDNS` | wireguard | 对端 DNS |
| `localOS` | router | 路由规则里按**本地操作系统**分流 |
| `costs` | router_strategy | leastLoad 策略的权重 |
| `echSockopt` | tls 安全 | ECH 的 sockopt |

### P1 —— 进阶/少用，可后置（11 项）

`cwndMultiplier` `maxSendingWindow` `sessionIDLength` `sessionIDTable`（xhttp/brutal 调优）、
`IsClient`（wireguard）、`experiments`（vmess）、`testpre`（vless）、`rawFieldRule`（router）、
`ipMode` `remoteIPs` `remotePorts`（finalMask）、`configKey`（loader）

### P2 —— finalMask 内部实现细节，一般不建议暴露（13 项）

`custom_table` `custom_tables` `u64` `texturesSignature` `texturesValue` `padding_min` `padding_max`
`brutalDisableLossCompensation` `disableChromeParrot` `disableStatelessReset`

---

## 3. 结论

- **面板自建入站（P1）不会丢字段** ✓ —— 编辑器里能填的 xray 字段都会原样下发；`_` 前缀字段被有意剔除。
- **真正的缺口有两处**：
  1. **P2 强类型生成器**只建模了 xray 配置的一小部分 → 服务/客户端配置里的进阶字段（xhttp 填充、
     sockopt、mux 等）发不出去；建议按上面的「修法 2」改成 map 透传。
  2. **编辑器 UI 缺 39 个字段**（P0 15 项）→ 用户在界面上根本填不了；优先补 P0。
- **校验方式**：任何补齐后都跑 `xray run -test -c <生成配置>` + 真机连一次，避免"能加载、连不通"。

## 4. 建议顺序

1. 先补 **P0 15 项** 的 UI（`XrayInbounds.vue` / `XrayOutbounds.vue`：freedom 出站、dns、sockopt、blackhole、router）
2. 把 `configgen/xray/generator.go` 改成 **map 透传 + 白名单归一化**，消灭 A 类
3. 再做 **sing-box 侧** 的同类清单（`/opt/sing-box` 是 fork 源码，按结构体字段比更准）

---

## 5. 状态更新（2026-09-22 实施后）

### 5.1 P0 已全部补齐（12 项，全部经 `xray run -test` 校验为 `Configuration OK`）

| 字段 | 位置 | 备注 |
|---|---|---|
| `sendThrough` / `targetStrategy` / `xudpConcurrency` / `xudpProxyUDP443` | 出站顶层（`XrayOutbounds.vue` 基础 tab） | 面板此前**完全没有**这四项 |
| `targetStrategy` / `ipsBlocked` | freedom 出站 | `ipsBlocked` 见 5.2 |
| `customResponseData` | blackhole 出站 | type=http 时的响应体 |
| `penetrate` | sockopt（出站） | 用入站原目标地址直连发送 |
| `expectIPs` | DNS nameserver | 与已有 `expectedIPs` 是**两个**独立字段 |
| `localOS` | 路由规则 | 匹配运行核心机器的 `runtime.GOOS`（大小写不敏感） |
| `costs` | leastLoad 策略 | 元素 `{match, value, regexp}`，JSON 录入 |
| `echSockopt` | TLS（入站） | JSON，ECH 连接套用的 socket 选项 |
| `remoteDNS` | wireguard 出站 | 隧道内 DNS |

### 5.2 实施时发现：清单本身有 4 处判断需要修正

- ✗ **`fakedns.fakeDNSInUse` 不是配置字段** —— 是 `fakedns.go` 里的局部变量，删掉。
- ⚠ **`freedom.ipsBlocked` 已被 Xray 废弃** —— `xray run -test` 告警：`The feature "ipsBlocked" has
  been removed and migrated to "finalRules"`。而面板**已有 Final Rules 编辑器** → UI 标注"已废弃"并
  指向 finalRules，保留读写仅为旧配置来回编辑（否则编辑一次就把用户的配置抹掉）。
- `freedom.applyTo` 属于 **`noise`**（只接受 ip/ipv4/ipv6）→ 降级 P1。
- `dns_proxy.blockTypes` / `nonIPQuery` 源码标注 `todo: remove legacy` → 降级 P2。
- 附带确认：outbound 级 `targetStrategy` 会被 xray 自动迁移到 `sockopt.domainStrategy`（告警说明）。

### 5.3 顺带发现的**真 bug**（已修）：leastLoad / leastPing 缺根 observatory → 核心启动失败

- 实测：只写 `strategy.type = leastLoad` 的 balancer、不写顶层 observatory →
  `failed to create server > core: not all dependencies are resolved`（补上 observatory 后 `Configuration OK`）。
- 面板路由编辑器**允许**选这两种策略，却从不生成 observatory（它是 xray **顶层**键、不在 routing 里，
  编辑器产出不到）→ 用户一选 leastLoad/leastPing，**节点直接起不来**。
- 原有 observatory 注入只存在于**链式负载均衡**分支（`prepare.go` 的 `if len(chainRules) > 0` 内）。
- 修法：后端兜底 `ensureObservatoryForLeastLoad`（`json.Marshal` 之前调用）——把这些 balancer 的
  selector 并进 `observatory.subjectSelector`；已存在 observatory/burstObservatory 时**只合并不覆盖**，
  避免冲掉链式负载均衡注入的探测配置。单测 `TestEnsureObservatoryForLeastLoad` 覆盖三种情况。

### 5.4 ⚠ A 类的结论需要更正：那条路径**不可达**（死代码）

进一步核查发现：**`internal/configgen/*`（xray / singbox / mihomo / tuic / hysteria2 五个生成器 +
`common/interfaces.go` + `validate/`）在整个仓库里没有任何调用方** ——
`grep -rn "configgen" --include=*.go .`（排除该目录自身）返回空，Go 编译器也不会放过未使用的 import，
所以它确实是**死代码**。

结论修正：§1 的 **A 类（12 项）在现网路径上不可达** ✗ —— 真正跑的只有
`prepare.go` 的 map 透传（面板自建入站/出站/路由）与前端编辑器的显式构造。
另外实测确认：**真配置里不会漏出 `_` 前缀键**（面板本机与节点两侧检查均为 0 个，
`_alpn` 这类有默认值的字段每个 TLS 入站都在走，所以这条检查是有效的）。

对这份清单的影响：**A 类不必"修"**，需要决策的是这套死代码本身（删除 / 或将来真的有"从统一模型
生成配置"的需求时再接线）。B 类（编辑器 UI 缺字段）才是真实缺口，P0 12 项已补完。

### 5.5 待办

- **决策项**：`internal/configgen/*` 这套死代码 —— 删除，还是保留待接线？（约 2~3k 行）
- B 类 P1/P2（11 + 13 项）：按需补充；finalMask 内部字段建议**不暴露**
- sing-box 侧同类清单已单独产出：`docs/singbox-field-gap.md`
