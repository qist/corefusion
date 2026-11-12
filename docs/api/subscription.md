# 订阅 API（对外）

> 语言： 简体中文 · [English](subscription.en.md) · [繁體中文](subscription.zh-TW.md) · [Русский](subscription.ru.md) · [日本語](subscription.ja.md) · [한국어](subscription.ko.md) · [Deutsch](subscription.de.md) · [Français](subscription.fr.md) · [Español](subscription.es.md) · [Português](subscription.pt.md) · [Türkçe](subscription.tr.md) · [Bahasa Indonesia](subscription.id.md) · [ไทย](subscription.th.md) · [Tiếng Việt](subscription.vi.md) · [العربية](subscription.ar.md) · [فارسی](subscription.fa.md)

拉取用户订阅节点列表。**这是面向最终用户最常用的接口**，无需任何 Header，凭 URL 中的 token 访问。

## 1. 端点

| 部署形态 | URL |
|---|---|
| 主面板 | `GET https://<面板域名>/sub/<token>` |
| 订阅独立端口 | `GET https://<订阅域名>:<端口>/<token>`（无 `/sub` 前缀） |

`<token>` 是每个订阅唯一的访问令牌，由管理端生成（可在门户"我的订阅"里查看）。

## 2. 查询参数

| 参数 | 说明 |
|---|---|
| `format` | 输出格式，覆盖订阅默认格式。见下表 |
| `config` | 套用的配置模板名称（管理端\"订阅格式\"里定义的 config），须与本次 `format` 同格式，否则忽略 |
| `platform` | 按平台名匹配配置模板（`config` 未指定时生效），同样要求格式一致 |
| `providers` | 仅对 `mihomo_yaml` 生效：`1` / `true` 进入 **Provider 模式**，输出 `proxy-providers`、组内用 `use` 引用；缺省或其它值为原始模式（节点内联在 `proxies`）。详见 §6 |

### 支持的 format 值

| 值 | 输出 | 适用客户端 |
|---|---|---|
| `base64_uri`（默认） | 换行拼接的分享链接，整体 base64 | V2RayN、Shadowrocket、Surge 等通用客户端 |
| `raw_uri` | 换行拼接的分享链接，明文 | 需要明文链接的客户端 / 调试 |
| `mihomo_yaml` | 完整 Clash/Mihomo YAML 配置（含 proxy-groups、rules） | Clash.Meta / mihomo / Clash Verge Rev |
| `mihomo_proxies_yaml` | 仅 `proxies:` 节点列表的 YAML | mihomo proxy-provider 回源文件（配合动态 provider 定时刷新） |
| `singbox_json` | sing-box 完整 JSON 配置 | sing-box / SFI / FLClash(sing-box 内核) |
| `xray_json` | Xray 完整 JSON 配置 | Xray-core |

`format` 支持别名（大小写不敏感，便于用户手写/客户端默认）：`clash` / `clash_yaml` / `mihomo` / `yaml` → `mihomo_yaml`；`singbox` / `sing-box` → `singbox_json`；`xray` → `xray_json`；`base64` / `v2ray` / `v2rayn` / `uri` → `base64_uri`；`raw` → `raw_uri`。无法识别的值按 `base64_uri` 处理。

> 协议与格式存在兼容矩阵：例如 `anytls` 只进 sing-box/Mihomo 格式；**xray 服务端的 REALITY 节点不会出现在 sing-box 订阅里**（sing-box 不支持 xray 的 REALITY 实现，见 sing-box#4520），但仍会正常出现在 Xray 订阅中；**sing-box 服务端的 REALITY 节点在 Xray / sing-box 两种订阅里都能用**。客户端不识别的节点会被自动剔除，不会整份失败。

## 3. 响应头

| 响应头 | 示例 | 说明 |
|---|---|---|
| `Subscription-Userinfo` | `upload=1024; download=2048; total=1073741824; expire=1760000000` | 用量与到期时间（字节 / Unix 秒）。**错误响应（403）时也带**，供客户端展示余量 |
| `Profile-Update-Interval` | `6` | 建议刷新间隔（小时） |
| `Profile-Title` | `CoreFusion` | 订阅配置名（Clash Verge Rev / V2RayN 等显示用） |
| `Profile-Web-Page-Url` | `https://portal.example.com` | 门户对外地址（客户端"打开网页"入口；独立门户端口为域名根路径，否则 /p） |
| `Content-Type` | 按格式：`text/plain` / `text/yaml` / `application/json` | |
| `Content-Disposition` | `attachment; filename="subscription.txt"` | |

## 4. 状态码

| HTTP | code | 含义 |
|---|---|---|
| 200 | — | 正常下发（body 为按 format 渲染的订阅内容） |
| 400 | `MISSING_TOKEN` | URL 缺 token |
| 403 | （纯文本消息） | 订阅被暂停 / 已到期 / 未激活。消息按 `Accept-Language` 出 zh/en/ja/ko，body 为一行原因说明 |
| 404 | `SUBSCRIPTION_NOT_FOUND` | token 不存在 |

> 吊销（revoke）的订阅按不存在处理（404）。

## 5. 示例

```bash
# 1) 用默认格式拉取
curl -L "https://panel.example.com/sub/abcd1234efgh5678"

# 2) 指定 Mihomo YAML，并套用名为 "clash-global" 的配置模板
curl -L "https://panel.example.com/sub/abcd1234efgh5678?format=mihomo_yaml&config=clash-global"

# 3) 只拿节点列表（喂给 mihomo proxy-provider）
curl -L "https://panel.example.com/sub/abcd1234efgh5678?format=mihomo_proxies_yaml"

# 4) 查看用量（不取 body）
curl -sI "https://panel.example.com/sub/abcd1234efgh5678" | grep -i subscription-userinfo

# 5) Clash 动态订阅：套用配置模板 + Provider 模式（节点由 proxy-providers 定时回源）
curl -L "https://sub.example.com/abcd1234efgh5678?format=clash&config=out&providers=1"

# 6) sing-box / Xray 同样可套用各自格式的配置模板（规则按模板注入）
curl -L "https://sub.example.com/abcd1234efgh5678?format=singbox_json&config=sing-box-home"
curl -L "https://sub.example.com/abcd1234efgh5678?format=xray_json&config=xray-home"
```

Clash/Mihomo 动态订阅（节点定时回源刷新）写法：

```yaml
proxy-providers:
  subscription-nodes:
    type: http
    url: "https://panel.example.com/sub/abcd1234efgh5678?format=mihomo_proxies_yaml"
    interval: 3600
    health-check:
      enable: true
      url: https://www.gstatic.com/generate_204
      interval: 300

proxy-groups:
  - name: PROXY
    type: select
    use:
      - subscription-nodes
```

## 6. Provider 模式与多订阅域名

### 6.1 `providers=1` 输出什么

`format=mihomo_yaml&providers=1`（或 `format=clash&providers=1`）时，返回的 YAML：

- 顶层**不再内联节点**，改为 `proxy-providers:`；
- 组的节点来源用 `use: [<provider-key>...]` 引用；
- 共享订阅格式配置（`mihomo_configs.config_json`）里如果写了 `proxy-providers`，会被**忽略**（一份共享配置被多个套餐复用，写死 provider 会串套餐）；组 → 来源的映射在「套餐管理 → 组 → 来源映射」里配。

Provider 分三类：

| 类型 | key | 说明 |
|---|---|---|
| 内部 provider | 套餐节点组名（清洗后） | 按套餐的节点组自动生成，只包含该组的节点 |
| 保留 provider | `subscription-nodes` | 本套餐全部节点；回源 URL **不带** `provider` 参数 |
| 外部 provider | 套餐里配置的名称 | 外部来源（`http` 拉 YAML / `file` 本地文件） |

每个 provider 的公共字段：
- `url`：指回本订阅的 `format=mihomo_proxies_yaml` 端点，并继承主订阅的 `config` / `platform` 等参数，内部 provider 额外带 `provider=<key>`；
- `path`：**统一由「内部 Provider 默认参数 → 缓存路径模板」生成**（默认 `./providers/{key}.yaml`，`{key}` 自动替换）。外部 provider 也不能单独指定 path；
- `health-check`：`enable / url / interval`，内部 provider 可继承默认值，外部 provider 在套餐里单独配置；
- `interval`：刷新间隔（秒）。

### 6.2 provider 里的域名怎么选（多域名）

「订阅中心 → 订阅端点管理」可以配置**多条**端点记录（域名 + 路径 + 启用状态）。生成 provider 的 `url` 时按以下优先级选域名：

1. **本次请求实际使用的域名**：优先 `X-Forwarded-Host` 的第一个值（去掉端口），没有则用 `Host`；该域名必须命中一条**已启用**的端点；
2. 没有命中时，回落到 **ID 最小**的启用端点；
3. 路径取命中端点的 `path`（默认 `/`），最终形如 `https://<domain><path>/<token>?format=mihomo_proxies_yaml...`。

因此：

- 客户端用新域名拉订阅 → provider 里自动写新域名，**不需要改面板配置**；
- 旧域名如果还留在端点列表里且处于启用状态也没关系，不会再被写死；
- 客户端此前缓存了旧 provider 地址时，更新一次订阅（或清 providers 缓存 / 重启客户端）即可切换。

反代务必透传 `X-Forwarded-Proto` 与 `X-Forwarded-Host`，否则只能回落到第一个启用端点。面板生成 Caddy / Nginx 站点模板时已自动写入这两个头（`header_up X-Forwarded-Host {host}` / `proxy_set_header X-Forwarded-Host $host;`），手工维护的反代配置请自行补齐。

## 7. 注意事项

- **格式与配置强绑定**：`config`/`platform` 指定的模板格式必须与本次 `format` 一致，面板会自动忽略不匹配的模板——防止把 Mihomo 行格式规则注入 sing-box/Xray 的 JSON。
- **`providers=1` 只影响 Mihomo YAML**：sing-box / Xray / base64 / raw 会忽略该参数（节点始终内联）。
- **限流**：订阅端点有独立限流（`SubscribeRateLimit`），恶意高频拉取会被拒。
- **访问记录**：每次拉取记录 IP / UA / CDN 信息，管理端"订阅访问日志"可查。
- 独立订阅端口未配置域名时回退到本请求的 `X-Forwarded-Proto` / `X-Forwarded-Host`；反代请正确透传这两个头。
- 多域名场景请保持「订阅端点管理」里的域名与实际访问域名一致，详见 §6.2。
