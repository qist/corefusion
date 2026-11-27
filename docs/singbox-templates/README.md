# sing-box 订阅格式模板
<sub>简体中文 · [English](README.en.md) · [Русский](README.ru.md) · [فارسی](README.fa.md)</sub>

[`config.json`](./config.json) 是面板「订阅格式」里 **sing-box** 那一份的 `config_json`。

模板只包含**客户端侧固定配置**：

```text
log / dns / inbounds / route / experimental
```

`outbounds` 由面板按订阅节点**动态生成**，并自动补一个 `tag: proxy` 的
`selector`，所以模板里不写 `outbounds`、也不写任何节点。

## 一、上传

```text
面板 → 订阅格式 → 新建
  client_type: singbox
  format:      singbox_json
  配置内容:     粘贴 config.json
```

绑定套餐后，客户端订阅地址：

```text
https://<面板>/sub/<token>?format=singbox_json&config=<格式名>
```

输出 = 模板里的 log/dns/inbounds/route/experimental + 动态生成的 outbounds
（含 `proxy` selector、`direct`、`block`）。

> `inbounds` 以前不会被合并（只合并 log/dns/route/experimental），
> 现在已支持（`mergeFormatSections` 增加 `inbounds`），否则客户端拿到的是
> 没有监听端口的“半份配置”。

## 二、订阅者与独立使用（不依赖面板）

- **订阅者（终端用户）只需要订阅地址**，不需要登录/拥有面板：面板服务端在渲染时
  会把节点写成 `outbounds`，并补 `tag: proxy` 的 selector；客户端拿到的就是完整配置。
- **不接面板**（或想直接给一份能跑的示例）：用同目录的
  [`standalone.json`](./standalone.json)。它是完整配置，已用 `sing-box check` 校验通过，
  只需要把 `outbounds` 里的 `node-example` 换成你自己的节点
  （示例占位是 `127.0.0.1:1080` 的 socks5，换成 ss/vmess/vless 等按 sing-box 语法写）。
- `config.json` 是**服务端**订阅格式模板（不含 outbounds）；`standalone.json` 是
  **客户端**可直接运行的完整版，两者其余部分完全一致。


## 三、结构说明

| 段 | 作用 |
| --- | --- |
| `inbounds` | `mixed` 监听 `127.0.0.1:7890`（同时支持 HTTP/SOCKS）。要 TUN 自己加，见下 |
| `dns` | 新版 DNS 格式（sing-box 1.12+）：国内 DoH → `direct`，国际 DoH → `proxy` |
| `route.rule_set` | 远端 `.srs`（MetaCubeX 公共库）：`cn`、`category-ads-all`、`geolocation-!cn`、`geoip/cn` |
| `route.rules` | 广告拦截 → reject；STUN(3478/19302) → reject；国内域名/IP → direct；国外 → proxy |
| `route.final` | `proxy`（面板生成的 selector） |
| `experimental.cache_file` | 缓存选中的节点 |

关键点：**每次订阅都会注入一个 `tag: proxy` 的 selector**（成员是全部节点），
规则里的 `outbound: "proxy"` 直接指向它，订阅者不需要手动改 `final` / 规则。
节点如果刚好叫 `proxy` / `direct` / `block`，面板会自动改名（`proxy-2` 等）避免 tag 冲突。

## 四、TUN（可选）

模板默认只开 mixed 端口。需要接管全局流量时，在 `inbounds` 里加：

```json
{
  "type": "tun",
  "tag": "tun-in",
  "address": ["172.19.0.1/30"],
  "auto_route": true,
  "strict_route": false,
  "stack": "system"
}
```

sing-box 1.15 起 `stack` 已标记弃用（改用 `route.default_domain_resolver` /
出站策略相关的新字段），目前仍可用，只是会打一条 WARN。

## 五、DNS 说明

sing-box 1.12 起旧的 `{"address": "...", "detour": "..."}` 写法被移除，
必须用新格式：

```json
{ "type": "https", "tag": "dns-cn", "server": "223.5.5.5", "path": "/dns-query", "detour": "direct" }
```

模板里用的是：

- 国内：`223.5.5.5`（阿里 DoH，走 `direct`）
- 国际：`8.8.8.8`（Google DoH，走 `proxy`）
- 广告：DNS 规则里 `action: "reject"`，直接返回空

`route.default_domain_resolver: "dns-cn"` 用于解析远端 rule_set 的域名。

## 六、规则库替换

模板用的是 MetaCubeX 的公共 `.srs`：

```text
https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/sing/geo/geosite/<name>.srs
https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/sing/geo/geoip/<name>.srs
```

换成自建规则时，保持 `type: remote` / `format: binary`，把 `url` 指到自己的 `.srs`
即可；sing-box 不支持直接吃 Clash 的 `.txt`/`.yaml` 规则集，
需要先转成 `.srs`（`sing-box rule-set convert`）。

## 七、验证

```bash
# 1. 拉订阅
curl -s "https://<面板>/sub/<token>?format=singbox_json&config=<格式名>" -o sb.json

# 2. 语法/结构校验
sing-box check -c sb.json
# exit 0 = 通过
```

本模板已在面板里实测：拉出来的订阅包含 `inbounds` + 动态 outbounds +
`proxy` selector，`sing-box check` 通过（1.15.0-alpha）。

## 外部链接

- [仓库目录](https://github.com/qist/corefusion/tree/main/docs/singbox-templates)
- 原始文件（可直接下载）：
  - [config.json](https://raw.githubusercontent.com/qist/corefusion/main/docs/singbox-templates/config.json)
  - [standalone.json](https://raw.githubusercontent.com/qist/corefusion/main/docs/singbox-templates/standalone.json)
- 上游文档：
  - sing-box：<https://sing-box.sagernet.org/>
  - 规则集（srs）：<https://github.com/MetaCubeX/meta-rules-dat>

- 项目仓库：<https://github.com/qist/corefusion>
- 规则仓库（本模板配套的 clash 规则库）：<https://github.com/qist/xray>
