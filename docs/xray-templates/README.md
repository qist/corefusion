# Xray 订阅格式模板
<sub>简体中文 · [English](README.en.md) · [Русский](README.ru.md) · [فارسی](README.fa.md)</sub>

[`config.json`](./config.json) 是面板「订阅格式」里 **xray_json** 那一份的 `config_json`。

模板只包含客户端固定配置：

```text
log / dns / inbounds / routing
```

`outbounds` 由面板按订阅节点动态生成，并自动补：

- `direct`（freedom）、`block`（blackhole）；
- 一个 `tag: proxy` 的 **balancer**（xray 没有 selector 概念，用 random 策略的
  balancer 承载“代理组”），规则里写 `"outboundTag": "proxy"` 会被面板改成
  `"balancerTag": "proxy"`。

## 一、上传

```text
面板 → 订阅格式 → 新建
  client_type: v2ray
  format:      xray_json
  配置内容:     粘贴 config.json
```

订阅地址：

```text
https://<面板>/sub/<token>?format=xray_json&config=<格式名>
```

## 二、订阅者与独立使用（不依赖面板）

- **订阅者（终端用户）只需要订阅地址**，不需要登录/拥有面板：面板服务端渲染时会生成
  `outbounds` 和 `tag: proxy` 的 balancer，客户端拿到的就是完整配置。
- **不接面板**：用同目录的 [`standalone.json`](./standalone.json)。它是完整配置，
  已用 `xray run -test` 校验通过；把 `outbounds` 里的 `node-example`
  （示例占位是 `127.0.0.1:1080` 的 socks）换成你自己的节点即可。
- `config.json` 是**服务端**订阅格式模板（不含 outbounds），`standalone.json` 是
  **客户端**可直接运行的完整版。


## 三、结构说明

| 段 | 作用 |
| --- | --- |
| `inbounds` | SOCKS `127.0.0.1:10808` + HTTP `127.0.0.1:10809` |
| `dns` | 国内域名 → 阿里 DoH，国外域名 → Google DoH，其余走 `1.1.1.1` |
| `routing.rules` | 广告 → `block`；STUN(3478/19302) → `block`；国内 → `direct`；国外/默认 → `proxy` |
| `routing.domainStrategy` | `IPIfNonMatch`（先域名匹配，再解析 IP 匹配 geoip） |

`routing.rules` 里的 `outboundTag: "proxy"` 由面板改写为 balancer，
所以模板里不需要写 `balancers`。**每次订阅都会注入 `tag: proxy` 的 balancer**，
订阅者不用手动改 `routing`；节点名与 `proxy` / `direct` / `block` 冲突时会自动加后缀。

## 四、规则依赖 geodata

模板用的是 xray 自带的 `geosite:` / `geoip:` 标签：

```text
geosite:category-ads-all
geosite:cn
geosite:geolocation-!cn
geoip:cn
```

客户端需要能找到 `geosite.dat` / `geoip.dat`（面板会在 `data/rules/` 下同步这些文件）。
自建/精简数据时把 `geoip.dat`、`geosite.dat` 放到 `XRAY_LOCATION_ASSET` 指向的目录。

想换成自己的规则，直接把 `routing.rules` 写成 xray 原生 JSON 即可
（面板只做“塞进数组 + 清理悬空引用”，不做格式转换）。

## 五、验证

```bash
# 1. 拉订阅
curl -s "https://<面板>/sub/<token>?format=xray_json&config=<格式名>" -o xr.json

# 2. 校验（需要 geodata）
XRAY_LOCATION_ASSET=/opt/corefusion/data/rules xray run -test -c xr.json
# Configuration OK. = 通过
```

本模板已在面板里实测：输出包含 `inbounds` + 动态 outbounds + `proxy` balancer，
`xray run -test` 返回 `Configuration OK.`（Xray 26.11.9）。

## 六、已知告警

订阅里的节点如果带 WebSocket `headers.Host`、或 Trojan 无 flow，xray 26.x 会打
弃用告警（`WebSocket transport ... deprecated`、`Trojan ... deprecated`）。
这些来自节点协议本身，不影响加载与运行。

## 外部链接

- [仓库目录](https://github.com/qist/corefusion/tree/main/docs/xray-templates)
- 原始文件（可直接下载）：
  - [config.json](https://raw.githubusercontent.com/qist/corefusion/main/docs/xray-templates/config.json)
  - [standalone.json](https://raw.githubusercontent.com/qist/corefusion/main/docs/xray-templates/standalone.json)
- 上游文档：
  - Xray-core：<https://xtls.github.io/>
  - geosite / geoip 数据：<https://github.com/v2fly/domain-list-community>

- 项目仓库：<https://github.com/qist/corefusion>
- 规则仓库（本模板配套的 clash 规则库）：<https://github.com/qist/xray>
