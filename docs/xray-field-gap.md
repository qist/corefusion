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
