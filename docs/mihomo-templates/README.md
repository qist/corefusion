# mihomo 订阅格式模板

三份模板：**Windows / Android / OpenClash（路由器）**。

它们就是**面板「订阅格式」里存的那份 YAML**：不含 `proxies`、不含
`proxy-providers`，节点由面板按套餐动态生成。导入面板后，订阅链接拉到的
配置才会带节点；加 `?providers=1` 时面板自动输出 `proxy-providers` 并在组里补 `use`。

| 文件 | 适用客户端 | 接管方式 |
| --- | --- | --- |
| [`mihomo-windows.yaml`](./mihomo-windows.yaml) | Clash Verge Rev / Mihomo Party / FlClash PC | `mixed-port` + 系统代理 |
| [`mihomo-android.yaml`](./mihomo-android.yaml) | Clash Meta for Android / FlClash | App 侧创建 VPN/TUN |
| [`mihomo-openclash.yaml`](./mihomo-openclash.yaml) | OpenClash（OpenWrt） | redirect/tproxy + 可选 TUN |

## 一、面板里的动态生成机制

模板里看不到节点，是因为节点是**渲染时注入**的：

| 位置 | 说明 |
| --- | --- |
| `_hasExternalNodes: true` | 标记「这个组要放外部节点」。原始模式下注入节点名；provider 模式下注入 `use` |
| 顶层 `proxies:` | 只允许特殊出站（如 `dns-out`）；普通节点一律由面板生成 |
| 顶层 `proxy-providers:` | **模板里不要写**：面板会忽略它（一份 YAML 多套餐复用，写了会串套餐） |
| `rules:` | 导入后由面板按规则表输出；模板里的 `rules` 只作初始参考 |

所以流程是：

```text
粘贴模板 → 面板保存为订阅格式 → 客户拉 /sub/<token>
                                   ├─ 默认：节点内联进 _hasExternalNodes 的组
                                   └─ ?providers=1：输出 proxy-providers + 组内 use
```

如果你在模板里写死 `proxy-providers`，面板会打日志忽略，且套餐之间的
provider 会互相串——这也是本目录不写 provider 的原因。

## 二、导入面板

1. 面板 → **订阅格式**（mihomo 配置）→ 新建或编辑；
2. 把对应 YAML 粘进配置内容 → 保存；
3. 套餐里绑定这份格式，并给它配订阅来源（provider）；
4. 客户订阅地址：

   ```text
   https://sub.example.com/<token>                    # 内联节点
   https://sub.example.com/<token>?format=mihomo_yaml&config=<格式名>&providers=1
                                                      # provider 模式
   ```

`providers=1` 输出的 `proxy-providers` 里，`url` 指向面板自己的数据源
（带短码或 token），不需要你手工填。

### 短链

面板「订阅短链」生成的是**整份配置**地址（服务端把 `?format=&config=` 收进短码），
可以直接当订阅地址导入客户端：

```text
https://sub.example.com/<20位短码>        # 整份配置，含节点
```

但不要把它喂给 `proxy-providers`：provider 需要的是纯 `proxies:` 列表，
短链返回的配置里有 `proxy-groups` / `rules`，会解析失败。provider 场景用：

```text
https://sub.example.com/<token>?format=mihomo_proxies_yaml
```

## 三、脱离面板单独使用

模板本身不能直接跑（没有节点）。自己补一段即可，二选一：

**A. 内联节点**

```yaml
proxies:
  - name: "node-a"
    type: ss
    server: 192.0.2.10          # 示例地址（RFC 5737）
    port: 8388
    cipher: aes-128-gcm
    password: "example"
```

**B. 订阅 provider**（推荐）

```yaml
proxy-providers:
  main:
    type: http
    url: "https://sub.example.com/<token>?format=mihomo_proxies_yaml"
    interval: 3600
    path: ./providers/main.yaml
    health-check:
      enable: true
      url: https://www.gstatic.com/generate_204
      interval: 300
```

然后给需要节点的组加 `use`（并把 `_hasExternalNodes` 删掉，那是面板内部字段）：

```yaml
- name: ✈️ 墙外网站
  type: select
  use: [main]
  proxies:
    - DIRECT
```

> `proxy-providers` 需要**纯 proxies 列表**，所以 URL 带
> `?format=mihomo_proxies_yaml`；整份配置喂给 provider 会解析失败。

## 四、规则库地址

模板统一写 `https://rules.example.com/clash/Ruleset/<name>.txt`，换成自己的规则库。
没有自建规则库时，可按下表替换成公共源：

| 模板里的名字 | 公共来源（示例） |
| --- | --- |
| `direct` / `proxy` / `reject` / `private` / `gfw` / `tld-not-cn` / `telegramcidr` / `lancidr` / `cncidr` / `apple` / `icloud` / `google` | `https://raw.githubusercontent.com/Loyalsoldier/clash-rules/release/<name>.txt` |
| `allcidr` / `allipv6cidr` | `https://ispip.clang.cn/all_cn.txt`、`https://ispip.clang.cn/all_cn_ipv6.txt` |
| `win-spy` / `win-update` / `win-extra` | `https://raw.githubusercontent.com/Loyalsoldier/v2ray-rules-dat/release/<name>.txt` |

其余自定义规则（`Foreignmedia`、`Netflix`、`tvb`、`domain-proxy-cidr`、
`ip-proxy-cidr`、`proxy-ip-cidr`、`privatecn`、`applications`）没有通用公共版本，
按自己的分流习惯维护，或者删掉对应 `RULE-SET` 与 `rule-providers` 条目。

geodata 默认用 MetaCubeX 官方发布（公共）：

```yaml
geox-url:
  geoip: "https://fastly.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/geoip.dat"
  geosite: "https://fastly.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/geosite.dat"
  mmdb: "https://fastly.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/geoip.metadb"
```

## 五、DNS：国内外都用公共 DoH

```yaml
default-nameserver:            # bootstrap，必须是 IP
  - 223.5.5.5
  - 119.29.29.29
  - 1.2.4.8

nameserver:                    # 国内
  - https://dns.alidns.com/dns-query
  - https://doh.pub/dns-query

fallback:                      # 国际
  - https://dns.google/dns-query
  - https://cloudflare-dns.com/dns-query
```

`nameserver-policy` 按规则集分流：

- 国内域名 / 国内规则集（`direct`、`apple`、`icloud`、`win-*`…）→ 阿里 / 腾讯；
- 国外域名 / 代理规则集（`proxy`、`gfw`、`tld-not-cn`…）→ Google / Cloudflare；
- 广告规则集 → `rcode://success`（返回空结果，比 REJECT 快）。

`hosts` 只保留公共 DNS 的固定 IP（`dns.alidns.com`、`doh.pub` 等），
用于 DoH 域名自身的 bootstrap，避免死循环。**没有任何私有 DNS 或内网地址。**

想让 DoH 走代理，在地址后加 `#✈️ 墙外网站`；但这要求代理可用，否则解析会挂。

## 六、分流逻辑

```text
private / privatecn / lancidr / allcidr / allipv6cidr / cncidr / direct
                                              → DIRECT（IP 类 no-resolve）
GEOSITE cn / GEOIP CN / GEOIP private         → DIRECT
reject / GEOSITE category-ads-all             → 🛑 广告拦截（REJECT）
UDP 443 且非中国 IP                            → REJECT（挡 QUIC 绕过规则）
UDP 3478 / 19302（STUN）                       → REJECT（防 WebRTC 泄漏）
DST-PORT 22（SSH）                             → 🖥️ SSH（套餐里映射到指定节点组，默认全部节点）
icloud / apple                                → 🍏 苹果
win-spy / win-update / win-extra              → Ⓜ️ 微软
google                                        → 🐓 谷歌
Foreignmedia / Netflix                        → 对应媒体组
proxy / gfw / tld-not-cn / telegramcidr       → ✈️ 墙外网站
domain-proxy-cidr / ip-proxy-cidr             → ✈️ 墙外网站
proxy-ip-cidr                                 → ✈️ 墙外网站
MATCH                                         → 🐟 其他网站
```

## 七、策略组

`✈️ 墙外网站`、`🐟 其他网站`、`🎥 NETFLIX`、`🖥️ SSH`、`🚫 以下不用动`、
`🛑 广告拦截`、`🍏 苹果`、`Ⓜ️ 微软`、`🐓 谷歌`、`🌍 国外媒体`，
外加 `♻️ 自动选择`（`url-test`）。

带 `_hasExternalNodes: true` 的组（`✈️ 墙外网站`、`♻️ 自动选择`）由面板填节点；
`🖥️ SSH` **没有**这个标记，默认只有 `DIRECT`，节点必须由套餐显式映射。

### 22 端口只走指定节点（不会带入其它节点）

`🖥️ SSH` 组刻意**不写死节点**、**不加 `_hasExternalNodes`**、也不引用 `✈️ 墙外网站`：

```yaml
- name: 🖥️ SSH
  type: select
  proxies:
    - DIRECT
```

默认它只有 `DIRECT`（22 也会直连/被客户端拒绝），**不会自动带上任何节点**。
要放某个节点组进来，只能显式映射：

```text
面板 → 套餐 → 订阅来源 → 组 → 来源映射
   组名: 🖥️ SSH
   来源: hysteria        （只选你要开放 22 的节点组）
```

结果：

```text
DST-PORT,22 → 🖥️ SSH → use: [hysteria] → 只有 hysteria 组的节点
```

- 其它组（`✈️ 墙外网站`、`ACCESS-*` 之类）**不会**被带进来，仍按各自的映射走；
- 不配映射 = 22 只有 `DIRECT`，不会误走全部节点（这是刻意设计的默认值）；
- `_hasExternalNodes` 的组（`✈️ 墙外网站`、`♻️ 自动选择`）才是「不配映射就注入全部节点」，
  两者不要混用；
- 配好后可以用订阅验证：

  ```bash
  curl -s "https://<面板>/sub/<token>?format=mihomo_yaml&config=<格式>&providers=1" | grep -A4 'name: 🖥️ SSH'
  # 期望：use: [hysteria]
  ```

> **映射不在模板里**：模板只定义组结构和 `DST-PORT,22` 规则；「哪个组用哪些节点」
> 属于**套餐**配置（`plan_templates.subscription_provider_config` 的 `group_use`），
> 同一个模板被多个套餐复用时各配各的，互不影响。
>
> **升级/换模板时必须同步组名**：映射按**组名精确匹配**，改名等于断开关联。
> 旧模板用的组名是 `ACCESS-DENIED`，如果套餐里已经配过 `ACCESS-DENIED → hysteria`，
> 换成新模板后要在「订阅来源」里把组名改成 `🖥️ SSH`（或者继续用旧组名），
> 否则 22 会退回只走 `DIRECT`，看起来像"节点没关联上"。

如果 22 本来就不该走代理，删掉 `DST-PORT,22,...` 这条规则和 `🖥️ SSH` 组即可。

## 八、平台差异

| 项 | Windows | Android | OpenClash |
| --- | --- | --- | --- |
| `allow-lan` | false | false | **true**（给局域网设备转发） |
| `ipv6` | false | true | true |
| `find-process-mode` | strict | strict | **off**（路由器无进程信息） |
| `tun` | 无（系统代理） | 无（App 自己接管） | **enable: true**，`stack: system`，mtu 1400 |
| `dns.listen` | `0.0.0.0:53` | `0.0.0.0:53` | `0.0.0.0:53` |
| `external-controller` | `127.0.0.1:9090` | `127.0.0.1:9090` | `0.0.0.0:9090` |

桌面端要用 TUN 接管游戏 / UWP 程序，在 Windows 模板补一段：

```yaml
tun:
  enable: true
  stack: system
  auto-route: true
  auto-detect-interface: true
  dns-hijack:
    - any:53
```

Android 由客户端创建 VPN，不需要写 `tun`。

> OpenClash 模板的 `external-controller` 监听 `0.0.0.0:9090`，
> 记得设 `secret` 或改回 `127.0.0.1`，别让 Web UI 裸奔。

## 九、自检

面板里保存后，直接拉一次订阅验证（推荐）：

```bash
curl -s "https://<你的面板>/sub/<token>?format=mihomo_yaml" -o test.yaml
mihomo -t -d <临时目录> -f test.yaml
```

单独使用时：

```bash
mihomo -t -d <临时目录> -f mihomo-windows.yaml
```

本目录三份模板已用「模拟面板输出」的方式校验通过：把某个
`_hasExternalNodes` 组注入一个假节点（原始模式）或 `use`（provider 模式）后，
`mihomo -t` 均返回 `test is successful`。

## 十、为什么模板里没有节点/私有信息

模板随仓库公开分发，所以刻意不包含：

- 真实服务器地址、节点密码、UUID；
- 私有 DNS（自建 DoH、内网解析器）与对应 IP；
- 私有规则库域名、内网网段、WireGuard 出站；
- 静态 `proxy-providers`（会绕过面板的套餐级 provider，导致串套餐）。

节点由面板按套餐动态生成；需要完整可用配置时，直接复制客户的订阅地址即可。
