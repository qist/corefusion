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
| `config` | 套用的配置模板名称（管理端"订阅格式"里定义的 config），须与本次 `format` 同格式，否则忽略 |
| `platform` | 按平台名匹配配置模板（`config` 未指定时生效），同样要求格式一致 |

### 支持的 format 值

| 值 | 输出 | 适用客户端 |
|---|---|---|
| `base64_uri`（默认） | 换行拼接的分享链接，整体 base64 | V2RayN、Shadowrocket、Surge 等通用客户端 |
| `raw_uri` | 换行拼接的分享链接，明文 | 需要明文链接的客户端 / 调试 |
| `mihomo_yaml` | 完整 Clash/Mihomo YAML 配置（含 proxy-groups、rules） | Clash.Meta / mihomo / Clash Verge Rev |
| `mihomo_proxies_yaml` | 仅 `proxies:` 节点列表的 YAML | mihomo proxy-provider 回源文件（配合动态 provider 定时刷新） |
| `singbox_json` | sing-box 完整 JSON 配置 | sing-box / SFI / FLClash(sing-box 内核) |
| `xray_json` | Xray 完整 JSON 配置 | Xray-core |

> 协议与格式存在兼容矩阵：例如 `anytls` 只进 sing-box/Mihomo 格式；sing-box 服务端的 REALITY 节点不会出现在 Xray/sing-box JSON 订阅中。客户端不识别的节点会被自动剔除，不会整份失败。

## 3. 响应头

| 响应头 | 示例 | 说明 |
|---|---|---|
| `Subscription-Userinfo` | `upload=1024; download=2048; total=1073741824; expire=1760000000` | 用量与到期时间（字节 / Unix 秒）。**错误响应（403）时也带**，供客户端展示余量 |
| `Profile-Update-Interval` | `6` | 建议刷新间隔（小时） |
| `Profile-Title` | `CoreFusion` | 订阅配置名（Clash Verge Rev / V2RayN 等显示用） |
| `Profile-Web-Page-Url` | `https://…/portal` | 门户页链接（客户端"打开网页"入口） |
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

## 6. 注意事项

- **格式与配置强绑定**：`config`/`platform` 指定的模板格式必须与本次 `format` 一致，面板会自动忽略不匹配的模板——防止把 Mihomo 行格式规则注入 sing-box/Xray 的 JSON。
- **限流**：订阅端点有独立限流（`SubscribeRateLimit`），恶意高频拉取会被拒。
- **访问记录**：每次拉取记录 IP / UA / CDN 信息，管理端"订阅访问日志"可查。
- 独立订阅端口未配置域名时回退到本请求的 `X-Forwarded-Proto` / `X-Forwarded-Host`；反代请正确透传这两个头。
