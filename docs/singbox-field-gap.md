# sing-box 字段级差异清单（基准：官方仓库 `SagerNet/sing-box`，`/opt/sing-box` branch `testing`）

> 目的：找出"sing-box 支持、但面板配不出来"的字段，作为补齐 UI 的依据。
> 与 `docs/xray-field-gap.md` 同方法，结论也共享一个关键前提（见 §1）。

## 0. 方法与口径

| 项 | 说明 |
|---|---|
| 基准源码 | `/opt/sing-box`（**官方** `github.com/SagerNet/sing-box`，branch `testing`，`4566ef089 Bump version`）|
| 字段抽取 | `option/*.go` 全部带 json tag 的字段：**约 1015 个**（含上游的 TLS spoof / finalMask 等） |
| 面板侧来源 | **只统计 sing-box 编辑器**（`views/protocols/sections/Singbox*.vue` + `sections/singbox/{common,inbounds,outbounds}/**`），避免被 Xray 编辑器误判为"已有" |
| 后端 | `internal/**` 全部 Go 代码 |
| 比对方式 | 字段名矩阵 + 关键项人工核对 |

## 1. 先说结论：不要被"后端没有"误导

矩阵里有一类结果叫"**编辑器有、后端（Go 代码里）没有**"（例如 `client_subnet`、`override_address`、
`rule_set_ip_cidr_match_source`、`hysteria2.realm_id`、`snell.obfs_mode` …）：

**这些不是缺口。** 与 xray 侧完全同因 —— 面板自建入站/出站/路由走的是
`internal/service/coreconfig/prepare.go` 的 **map 原样透传**（编辑器写什么就下发什么），
后端不需要逐个字段"点名"，所以字段名不出现在 Go 代码里 ≠ 会丢。

**真正的缺口 = 编辑器与后端都没有的字段**（下面 §2），即**用户在界面上根本填不了**。

## 2. 面板完全配不了的字段（按优先级）

### P0 —— 真实用户会用到，建议优先补

| 字段 | 位置 | 为什么重要 |
|---|---|---|
| `spoof` `spoof_method` | `tls`（inbound / outbound） | **上游自带**的 TLS spoof（见 §3 注：**不是 fork 特性，早先写错了**），编辑器里没有入口 ✗ |
| `record_fragment` `fragment_fallback_delay` | `tls` | TLS 记录分片，抗特征常用 |
| `tls_fragment` `tls_record_fragment` `tls_fragment_fallback_delay` | 路由规则动作 | 1.12+ 把分片移到 route action，与上面两个是两条实现路径 |
| `tls_spoof` `tls_spoof_method` | 路由规则动作 | 同一特性的 action 形式（上游自带）|
| `package_name` `package_name_regex` | 路由规则 / DNS 规则 / rule_set | **Android 客户端分流的核心手段** |
| `source_mac_address` `wifi_ssid` `wifi_bssid` | 路由规则 / DNS 规则 | 局域网按设备/SSID 分流（家用路由场景常用） |
| `network_type` `network_is_expensive` `network_is_constrained` | 路由规则 / DNS 规则 / outbound | 按网络类型（wifi/蜂窝/计费网）分流 |
| `source_hostname` `clash_mode` | 路由规则 / DNS 规则 | 兼容 Clash 配置与按主机名分流 |
| `query_dnssec` `query_client_subnet` `ip_accept_any` | DNS 规则 | DNS 查询控制（DNSSEC、ECS） |
| `match_response` `response_rcode` `response_answer` `response_ns` `response_extra` | DNS 规则 | **按 DNS 应答内容分流**（fakeip/去广告场景） |
| `optimistic` `cache_capacity` | `dns` | DNS 缓存优化/容量，延迟敏感场景常用 |
| `store_dns` `store_mode` `store_rdrc` `rdrc_timeout` `flush_interval` | `experimental.cache_file` | 缓存持久化（重启不掉缓存） |
| `udp_disable_domain_unmapping` | inbound / route action | UDP 域名的反查行为（很多客户端要关） |
| `network_type` `fallback_delay` `rewrite_ttl` `disable_optimistic_cache` | outbound | 出站网络类型与 TTL 改写 |
| `udp_over_tcp` | naive / shadowsocks / simple | UDP over TCP 回退 |
| `global_padding` `authenticated_length` | vmess | VMess 抗特征与认证长度 |
| `udp_over_stream` | tuic | TUIC 的 UDP 传输方式 |
| `extra_headers` `insecure_concurrency` `quic_session_receive_window` | naive | Naive 的进阶参数 |
| `padding_scheme` `client_metadata` | anytls | AnyTLS 填充方案 |
| `hop_interval_max` `max_packet_size` `min_packet_size` `disable_chrome_parrot` `rewrite_host` | hysteria2 | 端口跳跃与包大小调优 |
| `interface_address` `default_interface_address` `network_interface_address` `preferred_by` `rule_set_ipcidr_match_source` | 路由规则 / rule_set | 按接口地址匹配、规则集匹配源地址 |
| `proxy_protocol_accept_no_header` | inbound | PROXY protocol 的宽松模式 |
| `memory_limit` `oom_killer` `gc_percent` `panic_on_fault` `trace_back` `max_stack` `max_threads` | `debug` | 内存/崩溃行为；`oom_killer.go` 的 4 项（`max_interval`/`min_interval`/`safety_margin`/`memory_limit`）同属 |
| `write_to_system` | `ntp` | NTP 同步后写回系统时间 |

### P1 —— 进阶/少用，可后置

`client_authentication` `query_server_name` `pq_signature_schemes_enabled` `kernel_rx` `kernel_tx`
`dynamic_record_sizing_disabled` `certificate_public_key_sha256` `client_certificate_public_key_sha256`（tls 相关）、
`plugin_opts` `destinations`（ss）、`no_drop` `speculative` `remove_client_subnet`（rule action）、
`disable_version_fallback`（http）、`pid_file`（netns）、`set_system_proxy`（simple）、
`openvpn.*` 13 项 / `openconnect.*` 9 项 / `tailscale.*` 8 项（这些协议族编辑器已覆盖大部分，剩的是冷门项）

### P2 —— 不建议暴露

ACME 自动化（`acme.go` 10 项 + `tls_acme.go` 13 项，面板有独立的证书管理）、
`origin_ca.go` / `ccm.go` / `ocm.go`（Cloudflare Origin CA 与 OCM，属于专用场景）、
`bridge_name`、`$schema`（仅编辑器用）

## 2.5 ⚠ 优先级修正：区分"服务端可用"与"客户端专属"

编写 §2 时按"用户会不会用到"排的序，但实施前发现一个更重要的口径问题：

**面板的路由/DNS 规则编辑器编辑的是"节点（服务端）"的 sing-box 配置**，
因此下面这批 **客户端专属匹配** 在服务端**没有意义**，应从 P0 降级（只有将来做"客户端配置生成"才需要）：

`package_name` `package_name_regex`（Android）、`wifi_ssid` `wifi_bssid`、`network_is_expensive`
`network_is_constrained`、`source_hostname`（服务端反解极少用）、`preferred_by`、
`process_path` 类（服务端进程匹配意义有限）

**服务端真正有价值的**（重排后的 P0）：

| 字段 | 位置 | 价值 |
|---|---|---|
| `optimistic` `cache_capacity` | `dns` | **服务端 DNS 缓存**优化与容量（延迟敏感场景常用） |
| `store_dns` `store_mode` `store_rdrc` `rdrc_timeout` `flush_interval` | `experimental.cache_file` | 缓存持久化（重启不掉缓存） |
| `udp_disable_domain_unmapping` | inbound / route action | UDP 域名反查行为（很多场景要关） |
| `interface_address` `default_interface_address` `network_interface_address` `default_network_strategy` `default_network_type` `default_fallback_delay` `default_fallback_network_type` | `route` / rule | 多网卡/多出口的服务端分流 |
| `rule_set_ipcidr_match_source` | rule / rule_set | 规则集按**源地址**匹配（服务端分流常用） |
| `tls_fragment` `tls_record_fragment` `tls_fragment_fallback_delay` `tls_spoof` `tls_spoof_method` | 路由**动作** | 与 `tls.*` 是两条实现路径，服务端分流时也能用 |
| `no_drop` `speculative` `rewrite_ttl` `fallback_delay` | 路由动作 / outbound | 服务端行为微调 |
| `memory_limit` `oom_killer` 系列 `gc_percent` `max_stack` `max_threads` `panic_on_fault` `trace_back` | `debug` | 服务端资源与崩溃行为 |
| `clash_mode` | rule / rule_dns | 与外部控制器（clash API）配合 |

客户端专属的那批（`package_name` / `wifi_*` / `network_is_*` / `source_hostname`）建议**先不做**，
除非要做"给最终用户生成客户端配置"的功能。

---

## 2.6 实施结果与本轮实测到的**硬约束**（2026-09-22）

### 已完成（全部经 `sing-box check` 实证）

| 区域 | 字段 | 实测约束（配错会让核心**拒绝启动**） |
|---|---|---|
| route 顶层 | `default_network_strategy` | 合法值**只有 `hybrid` / `fallback`**（写别的报 `unknown network strategy`）；**必须同时开 `auto_detect_interface`**（否则 `auto_detect_interface is required by default_network_strategy`） |
| route 顶层 | `default_network_type` / `default_fallback_network_type` | 取值 `wifi` / `cellular` / `ethernet` / `other` |
| route 顶层 | `default_fallback_delay` | duration，非法值 FATAL |
| route 顶层 | `default_http_client` | 出站 tag |
| 规则 matcher | `clash_mode` / `interface_address` / `network_interface_address` / `default_interface_address` | `interface_address` 是 **map**（接口名 → 地址段）；这几个是**规则级 matcher**，放进 `route-options` 动作会报 `empty route option action` |
| route-options | `udp_disable_domain_unmapping` / `udp_connect` / `udp_timeout` / `network_strategy` / `fallback_delay` | 同 `network_strategy` 取值限制；`udp_timeout` 是 duration |
| reject 动作 | `method`（default/drop/reply）+ `no_drop` | **`method=drop` 时不能带 `no_drop`**（`no_drop is not available in current context`） |
| DNS 规则 | `query_dnssec` / `query_client_subnet` / 动作 `speculative` | 均可用 ✓ |

### 实测后判定**不适用**的（从缺口里移除）

| 项 | 实测结论 |
|---|---|
| `debug.*`（`memory_limit` / `oom_killer` / `gc_percent` / `max_stack` / `max_threads` / `panic_on_fault` / `trace_back`） | **sing-box 根配置里没有 `debug` 字段**（`FATAL decode config: debug: json: unknown field "debug"`）→ 这个结构体不在配置面上，界面**不需要**暴露 |
| `inbound.udp_disable_domain_unmapping` / `inbound.proxy_protocol_accept_no_header` | `FATAL …: legacy inbound fields are deprecated in 1.11.0 and removed in 1.13.0` → **已移到规则动作**（本轮的 route-options 里已提供） |
| `oom_killer` 的 4 个子项 | 它属于 **`services`**（`OOMKillerServiceOptions`，根配置的 `services[]`），不是 `debug` 的子项 → 属"服务"这一块能力，另立需求 |
| DNS 应答匹配（`match_response` + `response_rcode` / `response_answer` / `response_ns` / `response_extra` / `ip_accept_any`） | `FATAL validate dns rule[1]: response-based matching requires a preceding evaluate action` → 需要**先有一条 `evaluate` 动作规则**才能用，是个独立特性（本轮未做，已在清单里保留） |

### 顺带修掉的一个 bug（上一轮引入）
`SingboxRoute.vue` 的第二个构建站点（由**已存规则对象**构造，字段是原始 JSON 名）里
`applyRouteOptions()` 只认 camelCase → 那条路径上 `tls_fragment` / `tls_spoof` 等**根本没写出去**。
本轮把 `applyRouteOptions` / `applyRejectOptions` 都改成**双命名兼容**（`src[a] !== undefined ? src[a] : src[b]`），
并新增 `applyExtraMatchersRaw()` 处理同一问题。

---

## 2.7 DNS 应答匹配（已实现，2026-09-22）

sing-box 的 DNS **应答匹配**（按解析结果而不是按查询分流）需要两个东西配合，现已全部做进 `SingboxDns.vue`：

1. **`action: "evaluate"` 动作规则**（新增到动作下拉）：`server` + 可选 `tag` + `speculative` + `timeout`
2. **应答匹配块**：`match_response` + `response_rcode` + `response_answer` / `response_ns` / `response_extra` + `ip_accept_any`
   （开启后 `ip_cidr` / `ip_private` / `ip_accept_any` / `response_*` 都变成**按应答**匹配）

### 实测约束（三条都会让核心**拒绝启动**）

| 约束 | 报错 |
|---|---|
| `match_response` 与 `evaluate` 的 **tag 配对**：evaluate **带 tag** → `match_response` 必须写**同一 tag 字符串**；evaluate **不带 tag** → 只能写 `true` | `FATAL …: response-based matching requires a preceding evaluate action without `tag`; use `match_response` with an evaluate tag to reference a tagged result` |
| `response_answer` / `response_ns` / `response_extra` 是**字符串列表**，每项必须是 **zone 格式的完整 RR**（只写 IP 会报错） | `FATAL …: response_answer: … dns: not a TTL: "1.2.3.4"` |
| `response_rcode` 是**枚举**（NOERROR / NXDOMAIN / SERVFAIL / REFUSED / FORMERR / NOTIMP） | `FATAL …: unknown rcode: BOGUS` |

UI 据此做了：tag 输入框（留空即写 `true`）、rcode 用**下拉**（避免自由输入写出非法值）、
RR 文本框带 zone 格式示例与提示。

### 顺带发现（**不是 bug**，面板已覆盖）
sing-box 1.12+ 起，`dial` 字段缺 `domain_resolver`（且没有 `route.default_domain_resolver`）是
**FATAL**（`missing route.default_domain_resolver or domain_resolver in dial fields …`，
需 `ENABLE_DEPRECATED_MISSING_DOMAIN_RESOLVER=true` 才能继续）。
面板侧**已有兜底**：`internal/service/coreconfig/prepare.go` 会按需自动补 `route.default_domain_resolver`
（源码注释里就写了这条 FATAL 文案），所以用户不受影响 ✓。

### 仍未做
- `services`（含 `oom-killer`：`OOMKillerServiceOptions{memory_limit, safety_margin, min_interval, max_interval}`）
- `rule_set.initial_path`、naive `udp_over_tcp` 等协议细节

---

## 2.8 按序推进的收尾结果（2026-09-22）

### ② `services`（含 `oom-killer`）——**面板早已支持，无需新 UI**
`SingboxJsonSection.vue` 的通用 JSON 段落编辑器已支持 `services`（列表模式 + 11 个模板：
api / ccm / derp / dynamic / hysteria-realm / ocm / **oom-killer** / resolved / ssm-api / usbip-client / usbip-server）。
本轮只把 `desc` 补上 `oom-killer` 的字段说明（`memory_limit` / `safety_margin`（如 256M）/
`min_interval` / `max_interval`）—— 这符合项目对低频段落的既有做法（通用 JSON 编辑器 + 模板）。

### ③ `route.rule_set` 定义编辑器（**本轮新增**，这是真正缺的一块）
此前面板只有"规则的 rule_set **引用**下拉"，**没有任何地方能定义 rule_set**（只能靠 API 手写）。
现在 `SingboxRoute.vue` 增加了 **Rule Sets** 区（三个面板都已在运行的版本里）：
- 三种形态：`remote`（url / update_interval / download_detour / http_client / initial_path）、
  `local`（path）、`inline`（rules JSON）
- 多 tag 用逗号分隔（**inline 只能单 tag** ✗）

实测约束：
| 约束 | 报错 |
|---|---|
| inline 规则集带多个 tag | `FATAL route.rule_set[2]: inline rule-set does not support multiple tags` |
| `initial_path` 不含 `<tag>` 占位符 | 源码：`E.New("missing <tag> placeholder in initial_path")` |
| 写 `exclude_sequence` | `FATAL …: unknown field "exclude_sequence"`（**不是合法字段**） |
| local 规则集的 `path` 必须**真实存在** | `FATAL initialize router: parse rule-set[1]: open …: no such file or directory`（启动时会读文件） |

另核对了"会不会静默丢定义"：`SingboxRoute.vue` 的 `build()` **已经**会把未托管字段原样带回
（源码注释：*保留未在本编辑器管理的 route 字段（如 rule_set 定义、geo 资源等）*）✓
→ 之前**没有**静默丢失 ✓；新编辑器把 `rule_set` 也纳入托管（在保留逻辑之后覆盖）✓

### ③ 剩余协议细节 —— 判定**属客户端侧，暂不做**（记档）
`vmess.global_padding` / `vmess.authenticated_length`、`naive.udp_over_tcp`、`shadowsocks.udp_over_tcp`、
`tuic.udp_over_stream`、`hysteria2.hop_interval_max / max_packet_size / min_packet_size / disable_chrome_parrot`
—— 实测这些字段都在**出站（Outbound）**结构体上，**不在服务端入站**上；面板的 `singbox/outbounds/SingboxOb*.vue`
是"直绑 `model.<json名>`"的薄片段 ✓，但 `singboxBuildOutbound.js` 是**逐字段白名单** ✗
→ 要加必须同时改"片段 + build"两处。考虑价值（很少用面板编辑客户端出站）与风险，**本轮不做**，记录做法备查。

### ④ xray 侧 B 类 P1/P2 —— 判定**不建议做**（记档）
P1（xhttp 的 `cwndMultiplier` / `maxSendingWindow` / `sessionIDLength` / `sessionIDTable`、`vmess.experiments`、
`vless.testpre`、`loader.configKey`、`wireguard.IsClient`）与 P2（finalMask 的 `custom_table(s)` / `u64` /
`textures*` / `padding_*`、`brutalDisableLossCompensation` 等）都是**客户端调优或核心内部实现细节** ✗，
暴露出来只会增加误配风险（这类字段配错往往直接让核心起不来），**建议不暴露** ✓。

## 3. 建议顺序

0. **按 §2.5 修正后的口径做**（服务端可用优先；客户端专属暂缓）
1. ~~**补 P0 里"高价值"的前 6 行**~~：`tls.spoof/spoof_method`、`tls.record_fragment`、
   路由动作的 `tls_fragment`/`tls_record_fragment`/`tls_spoof` ——
   **更正（2026-09-22 核实）**：这些**不是 fork 特性，而是上游 SagerNet/sing-box 自带**的字段
   （`/opt/sing-box` 的 remote 就是 `github.com/SagerNet/sing-box`，`option/tls.go`、
   `option/rule_action.go` 里都在）——之前写成"本 fork 相对上游的差异"是**过时说法**，已更正；
   面板不给入口 = 上游特性用不起来（结论不变）。
2. 补**路由/DNS 规则**那一批（`package_name`、`source_mac_address`、`wifi_*`、`network_type`、
   `source_hostname`、`clash_mode`）+ **DNS 应答匹配**（`match_response`/`response_*`）
3. 补 `dns.optimistic`/`cache_capacity` 与 `experimental.cache_file` 的 store 系列
4. 每项都按 xray 侧同样的做法：UI + 默认值 + parse + build 四处，并**用 `sing-box check -c` 校验**产物

## 4. 与 xray 侧清单的关系

- 两边都遵循同一条结论：**面板自建配置走 map 透传，不会丢字段**；真正的缺口只在**编辑器 UI**
- 两边都发现过"清单误判"，实施前务必用**核心自身校验命令**（xray `run -test` / sing-box `check`）验证
  字段是否仍被支持（例如 xray 的 `ipsBlocked` 已废弃、sing-box 的字段随版本改名）
- 已知死代码：`internal/configgen/*`（5 个生成器 + `interfaces.go` + `validate/`）**全仓库无调用方**，
  其"A 类丢字段"在现网路径上不可达（详见 xray 清单 §5）
