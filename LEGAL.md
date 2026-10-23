# 法律与合规说明

> 语言： 简体中文 · [English](LEGAL.en.md) · [繁體中文](LEGAL.zh-TW.md) · [Русский](LEGAL.ru.md) · [日本語](LEGAL.ja.md) · [한국어](LEGAL.ko.md) · [Deutsch](LEGAL.de.md) · [Français](LEGAL.fr.md) · [Español](LEGAL.es.md) · [Português](LEGAL.pt.md) · [Türkçe](LEGAL.tr.md) · [Bahasa Indonesia](LEGAL.id.md) · [ไทย](LEGAL.th.md) · [Tiếng Việt](LEGAL.vi.md) · [العربية](LEGAL.ar.md) · [فارسی](LEGAL.fa.md)

> 本文档说明 CoreFusion 的许可证、商业使用政策、第三方组件义务以及使用者在各地区应自行关注的合规事项。
>
> **本文档不构成法律意见。** 具体业务是否合规取决于部署地区、服务对象和运营模式，请咨询当地执业律师。

## 1. 项目许可证

CoreFusion 以 **GNU Affero General Public License v3.0（AGPL-3.0）** 发布，全文见 [LICENSE](./LICENSE)。

```
Copyright (C) 2026 qist

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published
by the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
```

## 2. 商业使用与商业托管：明确允许

**任何个人或组织都可以将 CoreFusion 用于商业目的，包括但不限于：**

- 自建自用、内部部署；
- 对外提供收费的代理/节点运营服务；
- 以 SaaS、托管、代运维、系统集成等方式向第三方提供服务并收费；
- 二次开发、定制、分发（包括随硬件或整体方案交付）。

**上述商业使用和商业托管无需向本项目另行申请授权，也无需支付任何许可费或分成。**

唯一需要遵守的是 AGPL-3.0 本身的义务，核心有两条：

1. **分发二进制时**（§4–§6）：向接收者提供 Corresponding Source（完整对应源码），或按 §6 提供有效的书面获取途径；保留版权声明和许可证文本。
2. **修改后通过网络提供服务时**（§13）：如果发布了修改版，并让第三方通过网络与修改版交互，必须向这些用户**免费提供修改版的 Corresponding Source**（例如在面板显著位置放置源码下载/仓库链接）。

补充说明：

- **未修改**地原样部署并对外提供服务，不因"托管"这一行为触发 §13 的源码提供义务；但分发二进制仍适用第 1 条。
- 收费本身不被禁止；AGPL 约束的是**源码提供**，而不是商业模式。修改版对外提供服务时不能以"商业/闭源运营"为由拒绝履行 §13。
- 如果你希望获得**不受 AGPL 约束的闭源商业授权**，需要单独联系版权人；注意本项目链接了第三方 GPL-3.0 组件（见第 3 节），任何闭源版本都必须先移除这些组件。

## 3. 第三方组件与再分发

主要第三方组件及其许可证见 [THIRD_PARTY_LICENSES.md](./THIRD_PARTY_LICENSES.md)。

特别提示：

- 本项目通过 `github.com/xtls/xray-core` 间接链接了 **GPL-3.0-or-later** 的 `github.com/sagernet/sing` 和 `sing-shadowsocks`。因此**分发面板或 Agent 二进制时，必须同时满足 GPL-3.0 的对应源码提供义务**。
- 面板运行时可下载/调用 Xray-core、sing-box、Caddy 等独立程序。这些程序**不在本仓库内分发**，但如果你在交付物中一并分发，必须遵守各自的许可证（尤其是 sing-box 的 GPL-3.0：需提供其对应源码）。Hysteria2、TUIC 是 sing-box 承载的协议，不是独立二进制。
- 项目附带的 GeoIP/GeoSite 数据（如 GeoLite2、`geoip.dat`、`geosite.dat`）适用各自的数据许可；商用前请确认数据源的授权条款。

## 4. 地区合规与使用者责任

CoreFusion 是**通用软件**：它不提供任何代理或网络服务，不内置节点、带宽或凭据，也不代收任何费用。实际运营由部署者负责。

部署者/运营者应自行确认其业务在所在司法辖区是否需要许可、备案或安全评估，包括但不限于：

- **中国大陆**：增值电信业务经营许可（ICP/IDC/ISP 等）、ICP 备案、网络安全等级保护、跨境数据传输安全评估，以及《网络安全法》《数据安全法》《个人信息保护法》下的义务。不得将本软件用于违反当地法律法规的用途。
- **欧盟/英国**：GDPR / UK GDPR、ePrivacy、数字服务法（DSA）下的数据处理者/控制者义务、日志与 Cookie 合规。
- **美国**：各州隐私法、加密软件出口管制（EAR）与制裁名单筛查。
- **其他地区**：当地电信、加密通信、内容管理、数据本地化和消费者保护法规。

使用者承诺不将本软件用于：

- 违反适用法律或出口管制/制裁规定的活动；
- 侵犯他人知识产权、隐私权或其他合法权益；
- 传播违法内容、实施网络攻击、欺诈或其他犯罪活动。

**因使用或误用本软件产生的任何法律责任，由使用者自行承担。**

## 5. 数据保护与隐私

面板在运行过程中可能存储账号、联系方式、订单、支付回调、订阅、流量统计、登录与操作日志等数据。运营者作为数据处理者/控制者，应自行：

- 向用户提供隐私政策并取得必要授权；
- 遵循最小必要原则，设置合理的日志与统计数据保留期限（项目提供日志清理策略，需运营者配置）；
- 支持用户查询、更正、删除其个人信息的请求；
- 对数据库、备份、证书私钥和支付凭据采取访问控制与加密措施；
- 在跨境部署时完成相应的数据传输合规评估。

## 6. 免责声明

本软件按 AGPL-3.0 第 15–17 条"按原样"提供，不附带任何明示或默示的担保，包括但不限于适销性、特定用途适用性和不侵权的担保。版权人不对因使用本软件产生的直接、间接、偶然或后果性损失承担责任。

## 7. 联系

- 项目地址：<https://github.com/qist/corefusion>
- 商业授权 / 合规问题：请通过项目 Issue 或仓库主页公布的联系方式沟通。
