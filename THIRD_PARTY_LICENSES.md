# 第三方组件与许可证

CoreFusion 使用了以下开源组件。本文件列出主要组件及许可证，**完整清单以 `go.mod` / `go.sum` 及各模块自带的 LICENSE 文件为准**。

分发本项目的二进制时，请同时保留本文件、项目 `LICENSE` 以及各第三方组件的版权与许可声明。

## 1. 编译进二进制的 Go 依赖（主要）

| 组件 | 许可证 | 用途 |
|---|---|---|
| `github.com/xtls/xray-core` | MPL-2.0 | Xray 配置解析、gRPC 命令、协议定义 |
| `github.com/xtls/reality` | MPL-2.0 | REALITY 协议支持 |
| `github.com/sagernet/sing` | **GPL-3.0-or-later** | 经 xray-core 间接链接 |
| `github.com/sagernet/sing-shadowsocks` | **GPL-3.0-or-later** | 经 xray-core 间接链接（Shadowsocks-2022） |
| `github.com/go-sql-driver/mysql` | MPL-2.0 | MySQL 驱动 |
| `github.com/hashicorp/golang-lru/v2` | MPL-2.0 | LRU 缓存 |
| `github.com/go-acme/lego/v4` | MIT | ACME 证书签发 |
| `github.com/gin-gonic/gin` | MIT | HTTP 框架 |
| `gorm.io/gorm` | MIT | ORM |
| `github.com/aws/aws-sdk-go-v2` | Apache-2.0 | S3 备份、Route53 DNS-01 |
| `google.golang.org/grpc` | Apache-2.0 | Agent gRPC 通信 |
| `google.golang.org/protobuf` | BSD-3-Clause | Protobuf 运行时 |
| `golang.org/x/crypto` | BSD-3-Clause | 加密原语 |
| `golang.org/x/oauth2` | BSD-3-Clause | OAuth2 / Google EAB |
| `github.com/golang-jwt/jwt/v5` | MIT | JWT |
| `github.com/google/uuid` | BSD-3-Clause | UUID |

> ⚠ **GPL-3.0 组件提示**：`sing` 与 `sing-shadowsocks` 采用 GPL-3.0-or-later。它们被静态链接进面板与 Agent 二进制，因此**分发二进制时必须提供 Corresponding Source**（见 [LEGAL.md](./LEGAL.md) 第 3 节）。

## 2. 运行时调用的独立程序（不在本仓库内分发）

面板 / Agent 运行时可下载、安装或调用以下独立程序。它们与本项目通过进程/接口交互，但如果你在交付物中一并分发，仍需遵守各自的许可证：

| 程序 | 许可证 | 说明 |
|---|---|---|
| Xray-core | MPL-2.0 | 代理核心，独立进程 |
| sing-box | **GPL-3.0-or-later** | 代理核心，独立进程；分发时需提供对应源码 |
| Caddy | Apache-2.0 | 反向代理 / TLS |
| Nginx | BSD-2-Clause | 反向代理（系统安装，如使用） |

> Hysteria2、TUIC 是**由 sing-box 承载的协议**，不是独立核心，也没有独立二进制，因此不作为单独的第三方程序列出。

## 3. 数据文件

| 数据 | 来源/许可证 |
|---|---|
| `geoip.dat` / `geosite.dat` | 以其数据源仓库声明为准（常见为 GPL-3.0 或 CC-BY-SA） |
| GeoLite2 (`GeoLite2-ASN.mmdb` 等) | MaxMind GeoLite2 EULA / CC BY-SA 4.0，商用前请确认授权 |
| `country.mmdb` | 以其数据源声明为准 |

## 4. 合规建议

- 交付二进制时附带：`LICENSE`、本文件、各 GPL/MPL 组件的许可证文本或获取地址。
- 若修改了 `xray-core` 等 MPL-2.0 组件的源文件，需公开被修改文件的源码。
- 若随产品分发 sing-box 等 GPL 程序，需向接收者提供对应版本的完整源码（或有效的书面获取途径）。
- 商标与名称：不得使用各上游项目的名称、Logo 暗示官方背书。
