# sing-box 字段级差异清单（基准：本项目的 cf fork，`/opt/sing-box` branch `testing`）

> 目的：找出"sing-box 支持、但面板配不出来"的字段，作为补齐 UI 的依据。
> 与 `docs/xray-field-gap.md` 同方法，结论也共享一个关键前提（见 §1）。

## 0. 方法与口径

| 项 | 说明 |
|---|---|
| 基准源码 | `/opt/sing-box`（本项目的 cf fork，branch `testing`，`4566ef089 Bump version`） |
| 字段抽取 | `option/*.go` 全部带 json tag 的字段：**约 1015 个**（含 fork 自己新增的 TLS spoof / finalMask 等） |
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
| `spoof` `spoof_method` | `tls`（inbound / outbound） | **本 fork 的招牌特性（TLS spoof）**，编辑器里居然没有入口 ✗ |
| `record_fragment` `fragment_fallback_delay` | `tls` | TLS 记录分片，抗特征常用 |
| `tls_fragment` `tls_record_fragment` `tls_fragment_fallback_delay` | 路由规则动作 | 1.12+ 把分片移到 route action，与上面两个是两条实现路径 |
| `tls_spoof` `tls_spoof_method` | 路由规则动作 | fork 特性的 action 形式 |
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

## 3. 建议顺序

0. **按 §2.5 修正后的口径做**（服务端可用优先；客户端专属暂缓）
1. ~~**补 P0 里"fork 招牌 + 高价值"的前 6 行**~~：`tls.spoof/spoof_method`、`tls.record_fragment`、
   路由动作的 `tls_fragment`/`tls_record_fragment`/`tls_spoof` —— 这是本 fork 相对上游的核心差异，
   面板不给入口等于这个特性用不起来
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
