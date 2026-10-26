# CoreFusion 操作使用手册

> 语言： 简体中文 · [English](USER-GUIDE.en.md) · [繁體中文](USER-GUIDE.zh-TW.md) · [Русский](USER-GUIDE.ru.md) · [日本語](USER-GUIDE.ja.md) · [한국어](USER-GUIDE.ko.md) · [Deutsch](USER-GUIDE.de.md) · [Français](USER-GUIDE.fr.md) · [Español](USER-GUIDE.es.md) · [Português](USER-GUIDE.pt.md) · [Türkçe](USER-GUIDE.tr.md) · [Bahasa Indonesia](USER-GUIDE.id.md) · [ไทย](USER-GUIDE.th.md) · [Tiếng Việt](USER-GUIDE.vi.md) · [العربية](USER-GUIDE.ar.md) · [فارسی](USER-GUIDE.fa.md)

> 本手册面向 CoreFusion 使用者，涵盖从安装部署到日常运维的完整操作流程。

---

## 目录

1. [系统概述](#1-系统概述)
2. [安装部署](#2-安装部署)
3. [初始配置](#3-初始配置)
4. [仪表盘](#4-仪表盘)
5. [节点管理](#5-节点管理)
6. [协议中心](#6-协议中心)
7. [代理管理](#7-代理管理)
8. [分流资源](#8-分流资源)
9. [订阅中心](#9-订阅中心)
10. [客户管理](#10-客户管理)
11. [套餐管理](#11-套餐管理)
12. [充值管理](#12-充值管理)
13. [工单管理](#13-工单管理)
14. [证书管理](#14-证书管理)
15. [流量配额](#15-流量配额)
16. [链式代理](#16-链式代理)
17. [在线用户](#17-在线用户)
18. [用户管理](#18-用户管理)
19. [门户站点](#19-门户站点)
20. [系统设置](#20-系统设置)
21. [运行控制台](#21-运行控制台)
22. [数据库监控](#22-数据库监控)
23. [审计日志](#23-审计日志)
24. [日志管理](#24-日志管理)
25. [运营扩展](#25-运营扩展)
26. [常见问题](#26-常见问题)

---

## 1. 系统概述

CoreFusion 是一款多协议代理管理面板，支持 VLESS、VMess、Trojan、Shadowsocks、Hysteria2、TUIC 等多种协议，提供节点管理、订阅分发、客户管理、计费充值、证书管理、链式代理等完整功能。

### 1.1 核心功能

| 功能模块 | 说明 |
|---------|------|
| 多协议支持 | VLESS、VMess、Trojan、Shadowsocks、Hysteria2、TUIC、WireGuard 等 |
| 多核心支持 | Xray-core、sing-box 两个核心进程（Hysteria2/TUIC 等 QUIC 协议由 sing-box 承载） |
| 多节点管理 | 支持独立、镜像、主备、链式入口、链式出口五种节点类型 |
| 订阅分发 | 支持多格式订阅链接、订阅端点、订阅格式管理 |
| 计费系统 | 套餐模板、充值卡、余额管理、订单管理、代理员佣金 |
| 证书管理 | Let's Encrypt 自动签发、DNS-01/HTTP-01 验证、证书分发 |
| 链式代理 | 多出口负载均衡、外部节点注册、链式路由 |
| 流量配额 | 节点级月流量监控、自动暂停、重置日志 |
| 门户系统 | 可视化门户编辑器、多语言、自定义模板 |
| 通知系统 | 邮件、Telegram 通知，支持模板和群发 |

### 1.2 技术架构

- **后端**：Go 语言，gRPC 通信
- **前端**：Vue 3 + Element Plus
- **数据库**：SQLite（默认）/ MySQL / PostgreSQL
- **代理**：Caddy / Nginx 反向代理
- **面板端口**：由初始化向导指定（默认随机空闲端口，可填 8080）
- **Agent gRPC 端口**：默认 18443

---

## 2. 安装部署

### 2.1 支持的环境

| 项目 | 支持范围 |
|------|----------|
| 发行版 | Debian / Ubuntu、RHEL / CentOS / Rocky / AlmaLinux / Fedora、Alpine、Arch、openSUSE |
| CPU 架构 | 官方二进制：`amd64`、`arm64`；源码编译另支持 `armv7`、`armv6`、`386`、`ppc64le`、`s390x`、`riscv64`、`loong64` |
| 服务管理 | systemd（推荐）；容器等无 systemd 环境自动回落 `nohup` |
| 内存 / 磁盘 | ≥ 1 GB / ≥ 10 GB（数据库、证书、日志与核心二进制都放在数据目录） |
| 面板端口 | 由初始化向导指定（默认随机空闲端口，可填 `8080` 等）；初始化向导自身使用 18000–18999 的随机端口 |
| Agent gRPC | 默认 `18443` |

面板与 Agent 的**运行时外部命令**由安装脚本自动补齐（`scripts/install.sh` 与面板生成的
Agent 安装脚本都会做，`--no-deps` / `CF_SKIP_DEPS=1` 可跳过）：

| 用途 | 依赖 |
|------|------|
| 防火墙 / 限速 / 并发标记 | `nftables`（`nft`） |
| 限速队列、网卡与连接查看 | `iproute2`（`tc` / `ip` / `ss`） |
| 无 nft 时回退 | `iptables`（EL8+ 为 `iptables-nft`，EL7 为 `iptables`） |
| 并发连接统计 | `conntrack-tools`（`conntrack`） |
| 加载 `nf_conntrack` / `ifb` | `kmod`（`modprobe` / `lsmod`） |
| 进程检测与清理 | `procps`（`pgrep` / `pkill`） |
| 端口占用检测 / DH 参数 | `lsof` / `openssl` |
| 代理用户创建 | `shadow-utils`（Debian `passwd`，Alpine/Arch `shadow`） |
| 下载与解包 | `curl` / `ca-certificates` / `tar` / `gzip` / `xz` |

> 缺失 `nft` / `tc` 只影响限速与防火墙能力，不影响面板启动。
> 安装前可先用 `--cpu-info` 查看当前环境识别结果与依赖缺口。

### 2.2 一键在线安装（推荐）

```bash
curl -fsSL https://raw.githubusercontent.com/qist/corefusion/main/scripts/install.sh -o install.sh
sudo bash install.sh
```

也可以直接管道执行（无交互、无额外参数）：

```bash
curl -fsSL https://raw.githubusercontent.com/qist/corefusion/main/scripts/install.sh | sudo bash
```

脚本依次完成：

1. 识别发行版、CPU 架构与 libc，安装基础包与运行时依赖；
2. 从 GitHub Release 下载 `corefusion-linux-<arch>`，并用同名 `.sha256` 校验；
3. 安装到 `/opt/corefusion`，创建 `data/{sqlite,certs,logs,runtime}`；
4. 不写数据库配置，只预置 `SAFE_PATHS=data` 的占位 `.env`，保证首次启动进入初始化向导（完整 `.env` 由向导生成）；
5. 写入 systemd 服务并启动（无 systemd 时用 `nohup` 后台运行）；
6. 等待就绪并打印初始化地址、令牌与各目录位置。

安装完成后输出类似：

```text
==================================================================
 CoreFusion 安装完成
------------------------------------------------------------------
 安装目录 : /opt/corefusion
 数据目录 : /opt/corefusion/data
 二进制   : /opt/corefusion/corefusion
 日志     : journalctl -u corefusion -f
------------------------------------------------------------------
 初始化向导 : http://<服务器IP>:<随机端口>/init?init_token=<令牌>
 初始化令牌 : <令牌>
 有效期     : 2 小时（见启动日志）
 提示       : 令牌只显示一次；完成后向导端口关闭并自动切换到正式面板。
==================================================================
```

如果面板只监听 `127.0.0.1`（例如已配置反向代理），脚本会改为提示 SSH 隧道命令，而不是给一个访问不到的地址。

> 令牌丢失可重新查看启动日志：`journalctl -u corefusion -n 50 --no-pager`
> （无 systemd 时为 `tail -n 50 /opt/corefusion/data/logs/panel.log`）。

### 2.2.1 多语言输出

安装脚本内置简体中文、English、فارسی、Русский 四种界面语言，默认按系统 `LANG` 自动选择；也可显式指定：

```bash
# 指定英文
sudo bash install.sh --lang en

# 指定波斯语（环境变量方式）
sudo CF_LANG=fa bash install.sh

# 管道安装时传参
curl -fsSL https://raw.githubusercontent.com/qist/corefusion/main/scripts/install.sh | sudo bash -s -- --lang ru
```

支持 `zh` / `en` / `fa` / `ru`，也接受 `--lang=ru`。显式传入不支持的语言会报错并列出可选值。

### 2.3 指定版本 / 国内加速 / 离线安装

```bash
# 指定版本
sudo bash install.sh --version v0.13.0-alpha.2

# GitHub 下载加速（作为 URL 前缀）
sudo bash install.sh --mirror https://ghproxy.net --version latest

# 使用本地二进制（离线 / 内网 / 已交叉编译）
sudo bash install.sh --local ./dist/corefusion-linux-arm64

# 明确指定目标架构
sudo bash install.sh --arch arm64 --local ./corefusion-linux-arm64
```

> 不指定 `--version` 时取 GitHub Release 列表中最新的一条。本项目发布多为 pre-release，
> 因此脚本不会使用 `/releases/latest` 接口。
>
> 官方 Release 只提供 `amd64` / `arm64`；其他架构请用源码模式，或自行交叉编译后配合 `--local`。

### 2.4 源码编译安装

```bash
sudo bash install.sh --source --version v0.13.0-alpha.2
```

`--source` 会克隆源码（脚本位于源码目录内时直接使用现有源码）并执行 `make panel`。
依赖版本按系统环境自动选择：

| 依赖 | 选择策略 |
|------|----------|
| Go | 读取 `go.mod` 的 `toolchain` / `go` 指令（当前要求 **Go 1.27+**）；系统版本不足时下载对应工具链到 `<安装目录>/.toolchain` |
| Node | 默认 **Node 22 LTS**（Vite 构建要求 ≥ 18）；系统版本不足时下载官方构建 |
| Node（glibc < 2.28） | 自动改用 `unofficial-builds` 的 `glibc-217` 构建（如 CentOS 7） |
| Node（Alpine / musl） | 使用发行版 `nodejs` / `npm` |

可覆盖的变量：

```bash
# 指定 Go / Node 版本
sudo CF_GO_VERSION=1.27.5 CF_NODE_MAJOR=22 bash install.sh --source

# Go 工具链镜像（默认 https://go.dev/dl）
sudo CF_GO_MIRROR=https://golang.google.cn/dl bash install.sh --source
```

手动编译（开发 / 调试）：

```bash
git clone https://github.com/qist/corefusion.git
cd corefusion

# 前端 + 本机架构面板；前端通过 go:embed 打进二进制
make panel

# 交叉编译
make panel-linux-amd64
make panel-linux-arm64
```

> `go build ./...` 不会产出可执行文件。面板二进制必须用
> `go build -o corefusion ./cmd/corefusion/` 或 `make panel`。

### 2.5 安装参数速查

| 参数 | 说明 |
|------|------|
| `-d, --dir <PATH>` | 安装目录，默认 `/opt/corefusion` |
| `-v, --version <TAG>` | 面板版本，如 `v0.13.0-alpha.2`，默认 `latest` |
| `--binary` | 使用 GitHub Release 二进制（默认） |
| `--source` | 源码编译安装，自动准备匹配的 Go / Node |
| `--local <FILE>` | 使用本地二进制文件（离线安装） |
| `--arch <ARCH>` | 指定 CPU 架构：`amd64` / `arm64` / `armv7` / `armv6` / `386` / `ppc64le` / `s390x` / `riscv64` / `loong64` |
| `--mirror <URL>` | GitHub 下载加速前缀，如 `https://ghproxy.net` |
| `--repo <URL>` | 源码仓库地址，默认 `https://github.com/qist/corefusion.git` |
| `--ref <REF>` | 源码分支 / 标签，默认取 `--version`，否则 `main` |
| `--no-deps` | 跳过系统依赖安装（离线或已预装环境） |
| `--no-start` | 只安装并写入服务文件，不启动 |
| `--cpu-info` | 只输出环境识别与依赖缺口，不执行安装 |
| `--lang <LANG>` | 输出语言：`zh` / `en` / `fa` / `ru`（默认跟随系统 `LANG`） |
| `--uninstall` | 停止服务并删除程序（保留数据与 `.env`） |
| `--purge` | 与 `--uninstall` 连用，同时删除数据目录与 `.env` |

相关环境变量：

| 变量 | 作用 |
|------|------|
| `CF_GO_VERSION` | 覆盖源码模式要求的 Go 版本 |
| `CF_NODE_MAJOR` | 覆盖源码模式的 Node 主版本（默认 22） |
| `CF_GO_MIRROR` | Go 工具链下载镜像 |
| `CF_SKIP_DEPS` | Agent 安装脚本中设为 `1` 可跳过依赖安装 |
| `CF_LANG` | 覆盖安装脚本输出语言（等同 `--lang`） |

### 2.6 安装目录结构

```text
/opt/corefusion/
├── corefusion                 # 面板二进制
├── corefusion.bak             # 升级前的上一版（存在时）
├── .env                       # 环境配置（0600，初始化向导生成）
├── .toolchain/                # 源码模式下载的 Go / Node（仅源码安装）
└── data/                      # 数据目录（SAFE_PATHS）
    ├── sqlite/app.db          # 默认 SQLite 数据库
    ├── certs/                 # 证书与私钥
    ├── logs/                  # 无 systemd 时的运行日志
    ├── runtime/               # 核心运行配置与状态
    ├── binaries/              # 下载的 xray / sing-box 等核心
    └── proxy/                 # 代理（Caddy/Nginx）站点与配置
```

> 安装脚本只创建 `sqlite/`、`certs/`、`logs/`、`runtime/`；`binaries/`、`proxy/`、`rules/` 等
> 在面板启动或安装核心后自动创建。
> 数据目录位置由 `.env` 的 `SAFE_PATHS` 决定，可以是绝对路径。更换安装目录不会自动搬迁数据。

### 2.7 服务管理

systemd（大多数发行版）：

```bash
systemctl status corefusion --no-pager
systemctl restart corefusion
systemctl stop corefusion
journalctl -u corefusion -f          # 实时日志
journalctl -u corefusion -n 100      # 最近 100 行
```

无 systemd（容器等）：

```bash
cd /opt/corefusion && ./corefusion            # 前台运行（调试）
cd /opt/corefusion && setsid nohup ./corefusion >> data/logs/panel.log 2>&1 &
tail -f data/logs/panel.log
```

> 无 systemd 时进程不会开机自启，服务器重启后需要重新拉起（或重跑安装脚本）。

### 2.8 升级与卸载

升级（保留数据与配置）：

```bash
sudo bash install.sh --version <新版本>
```

脚本会停止旧服务、备份为 `corefusion.bak`、替换二进制并重启；已有 `.env` 与 `data/` 不会被动。

卸载：

```bash
# 只删程序，保留数据与 .env
sudo bash install.sh --uninstall

# 连数据目录与 .env 一起删除（不可恢复）
sudo bash install.sh --uninstall --purge
```

### 2.9 环境配置（.env）

完整的 `.env` 由**初始化向导在保存配置时生成**（数据库 DSN、数据目录、密钥、端口、域名等）。

安装脚本只在全新安装时预置一行占位，避免面板把已经存在的 `data/` 误判为“已初始化”而跳过向导：

```env
SAFE_PATHS=data
```

> 已有完整配置（`.env` 中含 `CF_DB_DSN`）时，安装 / 升级脚本不会覆盖，也不会再进入初始化向导。
> 之后手工编辑 `.env` 同样生效，改完重启面板即可。

#### 数据目录

```env
SAFE_PATHS=data                       # 相对安装目录
# SAFE_PATHS=/srv/corefusion-data     # 或绝对路径
```

#### 数据库配置

`CF_DB_DSN` 根据前缀自动识别数据库类型：

| 前缀 | 数据库 |
|------|--------|
| 不设置或裸路径 | SQLite（默认） |
| `sqlite://` | SQLite（显式） |
| `mysql://` | MySQL |
| `postgres://` / `postgresql://` | PostgreSQL |

`CF_DB_TYPE` 可选，用于覆盖自动检测（`sqlite` / `mysql` / `postgres`）。

> **密码加密存储**：通过面板写入（初始化向导 / 数据库切换）的 MySQL/PostgreSQL
> DSN 含数据库口令，会以 `enc:...` 形式（AES-256-GCM，密钥为 `CF_SECRET_ENCRYPTION_KEY`）
> 加密存进 `.env`；读取时自动解密，手工写明文 DSN 同样兼容。

**SQLite（默认）**

```env
# 裸路径（自动识别为 SQLite）
CF_DB_DSN=data/sqlite/app.db

# 或显式 sqlite:// 前缀
CF_DB_DSN=sqlite:///opt/corefusion/data/sqlite/app.db
```

**MySQL**

```env
# 格式：mysql://user:password@tcp(host:port)/dbname?params
CF_DB_DSN=mysql://corefusion:password@tcp(127.0.0.1:3306)/corefusion?charset=utf8mb4&parseTime=True&loc=Local
```

**PostgreSQL**

```env
# 格式：postgres://user:password@host:port/dbname?params
CF_DB_DSN=postgres://corefusion:password@127.0.0.1:5432/corefusion?sslmode=disable
```

#### 连接池（可选，MySQL/PostgreSQL）

```env
CF_DB_MAX_OPEN_CONNS=100
CF_DB_MAX_IDLE_CONNS=10
```

#### 其他常用项

| 变量 | 说明 |
|------|------|
| `JWT_SECRET` | 登录令牌签名密钥；缺失时首次启动自动生成 |
| `CF_SECRET_ENCRYPTION_KEY` | 敏感配置加密密钥（AES-256-GCM）；缺失时自动生成 |
| `CF_INITIAL_ADMIN_PASSWORD` | 首次初始化时的管理员口令（可不设，默认随机生成并只打印一次） |
| `CF_NETWORK_TRAFFIC_ACCOUNTING` | 设为 `0` 关闭基于 iptables 的流量统计（默认开启） |
| `CF_NETWORK_TRAFFIC_TABLE` | 流量统计使用的 iptables 表名，默认 `corefusion` |

> 可信反向代理地址（用于取真实客户端 IP）不再通过环境变量配置，而是在
> 「系统设置 → 网络」中以 `panel.trusted_proxies` 保存。

> `.env` 含密钥，面板启动时会自动把权限收紧到 `0600`。不要提交到版本库。

### 2.10 使用 Docker（可选，需自行构建镜像）

仓库**未内置** Dockerfile，如需容器化请自行编写镜像。参考运行参数：

```bash
docker run -d \
  --name corefusion \
  -p 8080:8080 \
  -p 18443:18443 \
  -v /opt/corefusion/data:/app/data \
  -v /opt/corefusion/.env:/app/.env \
  <your-image>
```

> 容器内通常没有 systemd 与 `nft` 权限，限速 / 防火墙相关功能不可用；
> 工作目录需与 `SAFE_PATHS` 的相对路径基准一致。

### 2.11 安装后自检

```bash
# 环境与依赖
sudo bash install.sh --cpu-info

# 服务状态
systemctl is-active corefusion

# 启动日志（含初始化地址与令牌）
journalctl -u corefusion -n 50 --no-pager

# 监听端口
ss -lntp | grep corefusion
```

---

## 3. 初始配置

首次启动时，若 `.env` 中没有有效的 `CF_DB_DSN`，面板**不会**直接提供管理界面，而是进入
**初始化向导**：只用 18000–18999 的随机端口和一次性令牌提供服务，配置保存后自动重启为正式面板。

### 3.1 打开初始化向导

1. 从安装脚本输出中复制「初始化向导」地址；
2. 或在服务器上查看日志：

```bash
journalctl -u corefusion -n 50 --no-pager | grep -A2 '初始化向导'
```

```text
CoreFusion 首次启动 - 初始化向导
  访问地址: http://<IP>:<端口>/init?init_token=<令牌>
  初始化令牌: <令牌>
  有效期: 2h0m0s
```

3. 浏览器打开该地址（含 `init_token`），令牌无需手工输入。

> 向导会话默认 **2 小时**过期。过期或面板重启后需重新获取；令牌不要分享给他人。
> 初始化完成后该端口关闭，仅正式面板端口对外服务。

### 3.2 向导步骤

向导共 8 步，可随时「上一步」修改，全部完成后才落库：

| 步骤 | 内容 |
|------|------|
| 1. 数据库 | SQLite / MySQL / PostgreSQL |
| 2. 数据目录 | 写入 `.env` 的 `SAFE_PATHS` |
| 3. 加密密钥 | `JWT_SECRET` 与 `CF_SECRET_ENCRYPTION_KEY` |
| 4. 核心安装 | 选择要安装的代理核心 |
| 5. 代理 | 不使用 / Nginx / Caddy |
| 6. 站点与域名 | 端口、访问路径、管理员账号、货币、时区、域名 |
| 7. HTTPS 证书 | 不启用 / 上传证书 / ACME 自动申请 |
| 8. 配置清单 | 复核、下载 `.env`、保存并重启 |

#### 步骤 1：数据库

- **SQLite（本地文件，推荐）**：文件路径留空即使用「数据目录/sqlite/app.db」。
- **MySQL / PostgreSQL**：填写主机、端口、用户名、密码、库名；PostgreSQL 可选 `disable` / `require` / `verify-full`。
- 点「测试连接」可先验证；保存下一步时也会测试。库不存在会自动尝试创建（账号无 `CREATE` 权限时请先手工建库）。
- 数据库连接串（含密码）会加密写入 `.env`。

#### 步骤 2：数据目录

存放数据库、证书、日志、核心二进制等，即 `.env` 的 `SAFE_PATHS`。默认 `data`（相对安装目录），
也可以填绝对路径。**创建后不要随意改动**，否则会找不到已有数据库与证书。

#### 步骤 3：加密密钥

- `JWT_SECRET`：登录令牌签名密钥。
- `CF_SECRET_ENCRYPTION_KEY`：敏感配置（数据库口令、证书 DNS 凭据、充值卡等）的加密密钥。
- 两者默认随机生成，可用「随机生成」重新取值。**更换 `CF_SECRET_ENCRYPTION_KEY` 会导致已加密数据无法解密**，务必备份。

#### 步骤 4：核心安装

勾选需要安装的代理核心：

- **Xray**：VLESS / VMess / Trojan / Shadowsocks 等；
- **sing-box**：Hysteria2 / TUIC 等 QUIC 协议由其承载。

保存并重启后由面板自动下载安装，可在「协议中心」查看版本与运行状态。也可以稍后再装。

#### 步骤 5：代理

- **不使用**：面板直接对外提供服务（需要自己处理 TLS）。
- **Nginx / Caddy**：由面板管理反向代理与证书托管，二者选一。

选择代理后，面板通常只监听 `127.0.0.1`，由代理对外服务。

#### 步骤 6：站点与域名

| 字段 | 说明 |
|------|------|
| 监听地址 | 自动推导：走代理 → `127.0.0.1`；不走代理且配置了证书 → `0.0.0.0`；其余 → `127.0.0.1` |
| 管理面板端口 | 默认随机分配空闲端口，可改（如 8080） |
| 订阅服务端口 | 可选，`0` 表示与主面板共用端口 |
| 门户站点端口 | 可选，`0` 表示与主面板共用端口 |
| 访问路径 | 管理面板前端路径，默认 `/`；API 固定为 `/api/v1` |
| 管理员用户名 / 密码 | 默认随机生成；**口令只在配置清单显示一次**，务必保存 |
| 访问方式 | 「域名」或「IP」 |
| 默认货币 / 时区 | 充值卡与余额的默认货币；面板时间与代理日志时区（如 `Asia/Shanghai`） |
| 一键添加到反向代理 | 选了代理且填了域名时，可把面板 / 订阅 / 门户域名一起反代 |

填了域名会写入对应配置（回调域名、订阅端点、分享域名）。

#### 步骤 7：HTTPS 证书

| 证书来源 | 说明 |
|----------|------|
| 暂不启用 HTTPS | 面板/门户/订阅以 HTTP 提供服务（不安全）；反向代理场景可稍后在代理侧补证书 |
| 上传自定义证书 | 粘贴证书 PEM 与私钥 PEM（仅域名访问方式） |
| ACME 自动申请 | 提交这一步即完成签发，支持域名与 IP 证书 |

ACME 选项：

- **CA 服务商**：Let's Encrypt、Let's Encrypt (Staging)、ZeroSSL、Google Public CA；
- **证书目标**：「配置域名」或「使用 IP」（仅 Let's Encrypt 支持 IP 证书）；
- **验证方式**：`HTTP-01`（需本机 80 端口）或 `DNS-01`（支持泛域名、无需 80 端口，需选择 DNS 服务商并填凭据）；
- **密钥类型 / 长度**：ECC 256/384 或 RSA 2048/3072/4096；
- **Google Public CA**：需要 EAB，可手工填 Key ID / HMAC，或用 GCP 服务账号自动申请。

> IP 证书会自动检测本机公网 IPv4/IPv6。若面板位于代理后面，签发 IP 证书时应注意取的是本机地址，
> 而不是代理出口地址。域名证书也可以在完成初始化后到「证书管理」中申请。

#### 步骤 8：配置清单

展示数据库、数据目录、核心、代理、端口、管理员账号、域名与 `.env` 键值（敏感项脱敏），并提供：

- **下载 .env**：保存一份配置副本；
- **访问地址**：面板 / 订阅 / 门户的最终 URL，HTTP 会标记「不安全」。

点「保存并重启」后写入数据库与 `.env`，面板自动重启切换到正式服务。

### 3.3 初始化完成

- 服务自动重启，启动日志会打印正式监听地址；
- 用配置清单中的管理员账号 + 访问地址 + 访问路径登录；
- 若口令丢失，可用 `./corefusion admin reset-password` 离线重置。

### 3.4 CLI 非交互初始化

适合自动化部署 / 无浏览器环境，直接落库并标记完成：

```bash
cd /opt/corefusion
./corefusion init \
  --db-type sqlite \
  --data-dir /opt/corefusion/data \
  --admin-username admin \
  --admin-password '改成强口令' \
  --listen-port 8080 \
  --proxy-type caddy \
  --cores xray,singbox \
  --panel-domain panel.example.com \
  --tls-mode acme --tls-domain panel.example.com --tls-email you@example.com
```

常用参数：

| 参数 | 说明 |
|------|------|
| `--db-type` / `--db-host` / `--db-port` / `--db-user` / `--db-password` / `--db-name` | 数据库配置 |
| `--data-dir` | 数据目录，默认取 `SAFE_PATHS` 或 `./data` |
| `--listen-port` / `--subscription-port` / `--portal-port` / `--web-path` | 端口与访问路径（默认随机空闲端口） |
| `--admin-username` / `--admin-password` | 管理员账号（口令留空则随机生成并只打印一次） |
| `--proxy-type` | `none` / `nginx` / `caddy` |
| `--cores` | 要安装的核心，逗号分隔：`xray,singbox` |
| `--panel-domain` / `--subscription-domain` / `--portal-domain` | 域名（会写入配置并加入反向代理） |
| `--tls-mode` | `none` / `self_signed` / `acme` |
| `--tls-domain` / `--tls-email` / `--tls-challenge` / `--tls-key-type` / `--tls-key-bits` | 证书参数（`--tls-challenge` 默认 `http-01`；用 `dns-01` 时需配合 `--tls-dns-channel` 或 `--tls-dns-provider`） |
| `--tls-ip-cert` | 为自动检测到的公网 IP 签发证书（仅 Let's Encrypt） |
| `--acme-provider` | `lets_encrypt` / `lets_encrypt_staging` / `zerossl` / `google` |
| `--currency` / `--timezone` | 默认货币 / 时区 |

完整参数：`./corefusion init --help`。初始化后启动：

```bash
systemctl restart corefusion
```

### 3.5 重新初始化

需要重走向导时（会删除 `.env` 与初始化标记，**保留数据**）：

```bash
cd /opt/corefusion
./corefusion reset-init      # 需输入 RESET 确认
systemctl restart corefusion
```

重启后按启动日志中的新地址与令牌重新进入向导。

> 重新初始化会重置管理员与访问路径；旧数据库中的业务数据仍在，但登录账号可能需要重建。
>
> `reset-init` 会删除整个 `.env`（包括 `SAFE_PATHS`）。**如果数据目录是自定义绝对路径**，
> 重启前先补回 `SAFE_PATHS`，否则面板会按默认的 `./data` 找库：
>
> ```bash
> printf 'SAFE_PATHS=/srv/corefusion-data\n' > /opt/corefusion/.env
> ```

### 3.6 首次登录后的安全设置

#### 修改初始密码

1. 点击右上角用户名；
2. 选择「修改密码」；
3. 输入原密码与新密码；
4. 点击「保存」。

#### 启用动态码登录（2FA / TOTP）

> 启用后账号改为**动态码登录**：登录时不再接受静态密码，只认认证器生成的 6 位动态码
> （或一次性恢复码）。这是“用动态码替代密码登录”，不是“密码 + 验证码”两步验证。

1. 点击右上角用户名；
2. 选择「开启 2FA」；
3. 输入当前密码（仅用于本次启用确认）；
4. 用认证器 App（Google Authenticator / 1Password 等）扫描二维码，或手动输入下方密钥；
5. 输入认证器生成的 6 位动态码完成启用；
6. **立即保存恢复码**（可下载文件），每张只能使用一次。

启用后的登录方式：

- 填用户名后，密码框会变为「6 位验证码 / 恢复码」，按钮变为「动态码登录」；
- 输入认证器当前动态码即可，**无需也不能再用静态密码**；
- 恢复码用一张少一张，建议用完后在「动态码登录（2FA）」中重新生成（重新生成会使旧码全部作废）。

认证器丢失且恢复码用完时，在服务器上离线关闭：

```bash
cd /opt/corefusion
./corefusion admin 2fa reset -u admin    # 关闭 2FA、清空密钥与恢复码、吊销全部会话
```

> 唯一的超级管理员如果既没有认证器也没有恢复码，网页端无法自助恢复，只能走上面的命令。

#### 配置 Passkey（可选）

1. 点击右上角用户名；
2. 选择「Passkey 管理」；
3. 点击「注册新 Passkey」；
4. 按提示完成设备认证。

> **注意**：Passkey 需要 HTTPS 或 localhost 环境，且域名需在白名单中。

---

## 4. 仪表盘

仪表盘展示多节点集群总览与节点健康监控。

### 4.1 查看内容

- **系统信息**：主机名、系统版本、运行时间、CPU、内存
- **流量统计**：实时带宽、总流量、月度趋势
- **节点健康**：在线/离线/降级节点数、高负载节点
- **异常告警**：离线节点、降级节点、即将过期证书、配额预警
- **集群资源**：平均 CPU、内存、磁盘、吞吐

### 4.2 操作

- 点击「查看详情」进入节点详情页
- 点击节点卡片查看实时状态
- 使用筛选器按状态、类型、区域过滤

---

## 5. 节点管理

节点管理是 CoreFusion 的核心功能，支持多种节点类型。

### 5.1 节点类型

| 类型 | 说明 |
|------|------|
| 独立节点 (standalone) | 独立运行的核心节点 |
| 镜像节点 (mirror) | 从目标节点同步配置 |
| 主备节点 (primary_backup) | 互为故障切换的伙伴节点 |
| 链式入口 (chain_entry) | 链式代理的入口节点 |
| 链式出口 (chain_exit) | 链式代理的出口节点 |

### 5.2 添加节点

1. 进入「节点管理」页面
2. 点击「新增服务器」
3. 填写基本信息：
   - 名称：如 `香港-HK01`
   - 主机名：IP 或域名
   - 区域：如 `HK`、`JP`、`US`
   - 节点类型：选择类型
4. 配置 Agent 端口（默认 18443）
5. 选择支持的核心
6. 点击「保存」

### 5.3 一键部署

1. 添加节点后，点击「Bootstrap 命令」
2. 复制安装命令
3. 在远程节点上以 root 执行：
   ```bash
   curl -fsSL https://your-panel/install.sh | bash
   ```
4. Agent 将自动下载、注册并启动

### 5.4 节点详情

点击节点进入详情页，可查看：
- **概览**：基本信息、Agent 状态、证书状态
- **协议中心**：核心进程配置
- **代理**：Caddy/Nginx 配置
- **核心进程**：启动/停止/重启
- **WARP**：WARP 配置
- **直连域名**：域名管理
- **限速**：流量限速配置
- **证书**：证书绑定
- **在线用户**：实时在线用户

### 5.5 节点操作

- **同步配置**：将面板配置推送到节点
- **强制重连**：断开 Agent 连接并自动重连
- **更新 Agent**：推送 Agent 更新
- **吊销证书**：吊销节点 Agent 证书

---

## 6. 协议中心

协议中心用于配置核心进程的协议参数。

### 6.1 支持的核心

| 核心 | 说明 |
|------|------|
| Xray-core | VLESS/VMess/Trojan/Shadowsocks 等多协议 |
| sing-box | 模块化通用代理平台，支持 TUN，并承载 Hysteria2/TUIC 等 QUIC 协议 |

### 6.2 配置模块

- **日志**：访问日志、错误日志、输出方式
- **API**：gRPC API 监听地址、服务列表
- **DNS**：DNS 服务器、查询策略、静态 Hosts
- **路由**：路由规则、负载均衡、VLESS 路由
- **入站**：入站列表、协议配置、传输、TLS
- **出站**：出站列表、协议配置、传输
- **传输**：WebSocket、gRPC、XHTTP、TUN 等
- **TLS**：证书、ALPN、uTLS 指纹、ECH
- **Reality**：Target 管理、密钥对、Short ID
- **健康检测**：探测 URL、间隔、并发
- **FakeDNS**：IP 池、池大小
- **指标**：Prometheus 指标监听
- **策略**：连接超时、缓冲区、统计
- **Geo 数据**：自动更新、资源文件
- **版本限制**：最低/最高版本

### 6.3 入站配置

1. 选择核心和协议
2. 配置基础信息：
   - 标签（tag）
   - 监听地址和端口
   - 协议类型
3. 配置协议参数（UUID、密码等）
4. 配置传输方式（TCP、WebSocket、gRPC 等）
5. 配置 TLS（证书、ALPN）
6. 保存并应用到核心

### 6.4 出站配置

1. 添加出站
2. 配置标签和协议
3. 配置服务器地址和端口
4. 配置传输和 TLS
5. 保存并应用

### 6.5 分享链接

1. 在入站列表点击「分享链接」
2. 选择格式：
   - 分享链接（URI）
   - Mihomo YAML
   - sing-box JSON
   - Xray JSON
3. 复制链接或生成二维码

---

## 7. 代理管理

代理管理用于配置 Caddy 或 Nginx 反向代理。

### 7.1 代理类型

- **Caddy**：自动 HTTPS、HTTP/2、HTTP/3
- **Nginx**：高性能、灵活配置

### 7.2 站点配置

1. 进入「代理管理」页面
2. 选择代理类型（Caddy/Nginx）
3. 新增站点：
   - 域名
   - 上游地址（upstream）
   - 本地端口（local_port）
4. 保存配置

### 7.3 配置说明

- **upstream**：后端服务地址，如 `127.0.0.1:8082`
- **local_port**：本地监听端口，如 `8443`
- **portal_forward**：门户转发规则
- **subscription_forward**：订阅转发规则

---

## 8. 分流资源

分流资源用于管理订阅分流规则。

### 8.1 功能

- 管理分流规则
- 配置分流策略
- 支持多种分流格式

### 8.2 操作

1. 进入「分流资源」页面
2. 新增分流规则
3. 配置规则名称、类型、条件
4. 保存并应用

---

## 9. 订阅中心

订阅中心用于管理订阅链接和端点。

### 9.1 订阅格式管理

1. 进入「订阅中心」→「订阅格式管理」
2. 新增格式：
   - 格式名称
   - 输出格式（Base64、YAML、JSON 等）
   - 包含的协议
3. 保存格式

### 9.2 订阅端点管理

1. 进入「订阅中心」→「订阅端点管理」
2. 新增端点：
   - 端点名称
   - 监听地址
   - 路径
   - 关联格式
3. 保存端点

### 9.3 订阅链接

- 访问 `http://your-domain/sub/:token` 获取订阅
- 支持 `?format=yaml` 等参数指定格式

---

## 10. 客户管理

客户管理用于管理终端用户。

### 10.1 新增客户

1. 进入「客户管理」页面
2. 点击「新增客户」
3. 填写信息：
   - 邮箱
   - 密码
   - 套餐
   - 流量配额
   - 到期时间
4. 保存

### 10.2 客户操作

- **编辑**：修改客户信息
- **暂停**：暂停客户服务
- **恢复**：恢复已暂停客户
- **重置流量**：重置已用流量
- **重置密码**：重置客户密码
- **续费**：延长到期时间
- **升级/降级**：更换套餐
- **删除**：删除客户

### 10.3 订阅详情

点击客户进入详情页，可查看：
- 订阅链接
- 流量使用
- 订单历史
- 交易记录

---

## 11. 套餐管理

套餐管理用于配置销售套餐。

### 11.1 套餐模板

1. 进入「套餐管理」→「套餐模板」
2. 新增模板：
   - 模板名称
   - 分类（标准/高级/企业/试用）
   - 自动续费
   - 试用设置
3. 保存模板

### 11.2 套餐规格

1. 在模板下新增规格：
   - 规格名称（如月付、年付）
   - 价格（分）
   - 时长（天）
   - 流量（GB，0=不限）
   - 并发限制
   - 限速（Mbps）
2. 保存规格

### 11.3 流量叠加包

1. 进入「流量叠加包」
2. 新增叠加包：
   - 名称
   - 价格
   - 流量
   - 过期模式（延长天数/与套餐同期）
3. 保存

### 11.4 优惠码

1. 进入「优惠码」
2. 新增优惠码：
   - 优惠码
   - 类型（百分比/固定金额）
   - 折扣值
   - 使用次数限制
   - 有效期
   - 适用套餐
3. 保存

---

## 12. 充值管理

充值管理用于管理充值站点、订单和充值卡。

> **支付接入说明**
>
> 面板**不直接接入支付机构**（支付宝、微信支付、Stripe、银行等），只接入**中间支付平台**；由中间支付平台接入对应的支付机构，完成签约、收单与结算，再按[支付回调 API](api/charge-callback.md) 把支付结果回调给面板。
>
> 中间支付平台**不在本项目范围内，需运营方自行开发（或使用自有系统）**。部署完成后在「充值站点」中登记该平台并完成域名验证，即可作为充值入口；这里的充值站点指的就是该中间支付平台，不是支付机构本身。支付资质、费率、结算与合规由运营方与支付机构、中间支付平台自行约定并承担。

### 12.1 充值站点

1. 进入「充值管理」→「充值站点」
2. 新增站点：
   - 站点名称
   - 该中间支付平台支持的支付方式（支付宝、微信、USDT、Stripe 等，由平台对接支付机构提供）
   - 货币
   - 汇率
3. 保存站点
4. 验证站点（HTTP 文件验证或 DNS 验证）

### 12.2 充值订单

- 查看所有充值订单
- 按状态筛选（待支付、已支付、已失败、已过期、已退款）
- 查看订单详情
- 管理员退款

### 12.3 充值卡

1. 进入「充值卡」
2. 新增卡：
   - 类型（金额型、套餐型、流量叠加包）
   - 面值/套餐
   - 过期时间
3. 或批量生成：
   - 生成数量（最多 1000 张）
   - 卡号前缀
   - 卡号格式
4. 导出 CSV

### 12.4 余额管理

- 查看客户余额
- 调整余额（增加/减少/设置）
- 调整冻结金额
- 查看交易记录

### 12.5 代理员

- 新增代理员
- 设置佣金比例
- 查看累计佣金
- 提现佣金

---

## 13. 工单管理

工单管理用于处理客户支持请求。

### 13.1 工单列表

- 查看所有工单
- 按状态筛选（待处理、已回复、已关闭）
- 按分类筛选（技术、账务、账号、其他）
- 按优先级筛选（低、普通、高、紧急）

### 13.2 工单操作

- **查看/回复**：查看工单详情并回复
- **分配**：将工单分配给管理员
- **关闭**：关闭已解决的工单

---

## 14. 证书管理

证书管理用于管理 TLS 证书。

### 14.1 签发证书

1. 进入「证书管理」页面
2. 点击「签发证书」
3. 填写信息：
   - 域名列表（每行一个）
   - 签发类型（域名证书/IP 证书）
   - 密钥类型（ECC/RSA）
   - 验证方式（DNS-01/HTTP-01）
   - DNS 提供商
   - 签发位置（面板本机/节点）
4. 点击「签发」

### 14.2 证书操作

- **续期**：手动续期证书
- **部署**：将证书部署到节点
- **分发**：分发到所有关联节点
- **迁移**：将旧证书引用迁移到新证书
- **下载**：下载证书文件（cert/fullchain/privkey）

### 14.3 上传证书

1. 点击「上传证书」
2. 选择上传方式：
   - 粘贴内容
   - 上传文件
3. 上传证书链和私钥
4. 保存

### 14.4 EAB 配置

- 管理 Google Public CA / ZeroSSL 的 EAB 配置
- 自动领取 EAB
- 签发时自动带上

---

## 15. 流量配额

流量配额用于监控节点月流量。

### 15.1 配额列表

- 查看所有节点的配额配置
- 显示配额大小、已用流量、剩余额度
- 显示用量占比

### 15.2 周期用量

- 查看每个节点的周期用量详情
- 显示入站/出站流量
- 显示周期开始/结束时间

### 15.3 重置日志

- 查看流量重置记录
- 显示重置周期、触发来源、操作人

---

## 16. 链式代理

链式代理用于配置多出口链路。

### 16.1 创建链路

1. 进入「链式代理」页面
2. 点击「新建链路」
3. 选择入口节点
4. 选择入口入站
5. 选择出口节点（可多选）
6. 为每个出口选择入站
7. 配置优先级/权重
8. 确认创建

### 16.2 链路管理

- 查看拓扑总览
- 编辑链路（优先级、权重、出站 tag）
- 删除链路
- 查看健康状态

### 16.3 外部节点

- 导入外部节点（v2ray URI、Base64 URI、订阅 URL）
- 注册外部入口
- 管理外部节点

---

## 17. 在线用户

在线用户用于查看实时在线状态。

### 17.1 查看内容

- 在线订阅数
- 在线设备/IP 数
- 在线节点数
- 实时速率
- 累计流量

### 17.2 筛选

- 按节点筛选
- 搜索邮箱/客户/节点/IP

---

## 18. 用户管理

用户管理用于管理面板用户。

### 18.1 角色

| 角色 | 说明 |
|------|------|
| 超级管理员 (superAdmin) | 最高权限 |
| 管理员 (admin) | 管理权限 |
| 操作员 (operator) | 操作权限 |
| 查看者 (viewer) | 只读权限 |

### 18.2 新增用户

1. 进入「用户管理」页面
2. 点击「新增用户」
3. 填写信息：
   - 用户名
   - 邮箱
   - 密码
   - 角色
4. 保存

### 18.3 用户操作

- **编辑**：修改用户信息
- **删除**：删除用户

---

## 19. 门户站点

门户站点用于配置客户端门户页面。

### 19.1 基本设置

- 站点名称
- 站点描述
- 开放注册
- 门户访问路径
- 门户端口

### 19.2 外观配置

- 站点标题
- Logo URL
- 主色调、背景色、文字色、强调色
- 字体

### 19.3 导航管理

- 添加导航页
- 配置页面名称、图标、外部链接
- 管理导航项

### 19.4 模块管理

- 添加/编辑模块
- 配置模块标题、描述、图标
- 支持 Markdown、富文本、自定义 HTML

### 19.5 多语言

- 添加语言
- 管理文案
- 导入/导出 JSON

### 19.6 模板管理

- 切换模板
- 自定义 HTML/CSS/JS
- 重置为默认模板

---

## 20. 系统设置

系统设置用于配置面板参数。

### 20.1 面板配置

- 监听地址和端口
- 数据目录
- 时区
- 订阅端口
- 导出上限
- 开放注册
- 面板模式（完整模式/纯管理模式）

### 20.2 安全设置

- 加密密钥管理
- JWT 密钥管理
- IP 黑名单
- Passkey 白名单域名

### 20.3 OAuth 配置

- GitHub OAuth
- Google OAuth
- 回调地址配置

### 20.4 通知设置

- 邮件通知（SMTP）
- Telegram 通知
- 通知模板
- 监控设置

### 20.5 备份还原

- 创建备份
- 备份列表
- 下载备份
- 还原备份
- 定时备份
- 远程存储（FTP/S3/WebDAV）

### 20.6 自解封配置

- 启用自解封
- 冷却时间
- 适用状态

---

## 21. 运行控制台

运行控制台用于管理配置快照和应用任务。

### 21.1 配置快照

- 创建快照
- 查看快照列表
- 回滚到指定快照
- 删除快照

### 21.2 应用任务

- 查看最近任务
- 立即应用配置（DB→运行时）
- 预检（校验端口冲突与配置 JSON）
- 取消任务
- 重试任务

### 21.3 节点验证

- 选择节点并验证
- 查看验证结果
- 验证全部节点

### 21.4 紧急停用

- 紧急停用代理（写入空入站配置）
- 立即停止所有入站流量

---

## 22. 数据库监控

数据库监控用于查看数据库运行状态。

### 22.1 监控内容

- 数据库引擎
- 数据库大小
- 连通性
- 延迟
- 连接池状态
- 缓存统计
- 查询统计

### 22.2 支持引擎

- SQLite
- MySQL
- PostgreSQL

---

## 23. 审计日志

审计日志用于查看操作记录。

### 23.1 查看内容

- 类别
- 事件类型
- 结果
- 来源 IP
- 用户
- 详情

### 23.2 节点操作日志

- 查看节点操作记录
- 按节点筛选

---

## 24. 日志管理

日志管理用于查看系统日志。

### 24.1 日志类型

- 面板日志
- 节点日志
- 核心日志
- 代理日志

### 24.2 操作

- 查看日志
- 下载日志
- 清理日志

---

## 25. 运营扩展

运营扩展用于配置扩展功能。

### 25.1 功能

- 自定义扩展
- 扩展配置
- 扩展管理

---

## 26. 常见问题

### 26.1 面板无法启动

- 检查端口是否被占用
- 检查 `.env` 配置是否正确
- 查看日志：`journalctl -u corefusion -f`；无 systemd 时 `tail -f <安装目录>/data/logs/panel.log`

### 26.2 节点离线

- 检查 Agent 是否运行：`systemctl status corefusion-agent`
- 检查网络连接
- 检查防火墙规则（18443 端口）

### 26.3 证书签发失败

- 检查域名解析
- 检查 80 端口是否开放
- 检查 DNS 提供商配置
- 查看证书日志

### 26.4 订阅无法使用

- 检查订阅链接是否正确
- 检查节点是否在线
- 检查核心进程是否运行
- 检查防火墙规则

### 26.5 充值回调失败

- 检查回调地址是否正确
- 检查站点密钥是否匹配
- 检查 IP 白名单
- 查看订单日志

### 26.6 门户页面无法访问

- 检查门户端口是否开放
- 检查门户路径配置
- **重点**：确认反向代理把门户域名转发到**门户引擎端口**，而不是面板主端口——转发到错误端口会表现为白屏或打开面板而非门户。检查代理站点记录里的 `upstream` / `local_port`。
- 直接访问 `http://<门户域名>:<门户端口>` 验证门户引擎本身是否正常
- 检查反向代理配置
- 查看门户日志

---

## 附录

### A. 默认端口

| 端口 | 用途 |
|------|------|
| 由初始化向导指定（默认随机空闲端口） | 管理面板（可改） |
| 18443 | Agent gRPC（可改） |
| 0（默认未启用） | 订阅引擎独立端口（留空或 0 表示共用主端口） |
| 0（默认未启用） | 门户引擎独立端口（留空或 0 表示共用主端口） |
| 80 / 443 | 反向代理（Caddy/Nginx）HTTP/HTTPS |

### B. 默认路径

| 路径 | 用途 |
|------|------|
| `/` | 管理面板（配置了独立门户引擎时，门户也挂在 `/`） |
| `/sub/:token` | 订阅拉取（未配置独立订阅端口时） |
| `/p` | 门户页面（未配置独立门户端口时） |
| `/:token` | 订阅拉取（配置独立订阅端口后，无 `/sub` 前缀） |

### C. 命令行工具

面板二进制实际支持以下子命令（无参数时默认启动 server）：

```bash
# 启动面板（等价于 ./corefusion server）
./corefusion

# 运行数据库迁移
./corefusion migrate

# 离线管理命令（无法登录时使用）
./corefusion admin reset-password -u admin -p '新口令'
./corefusion admin 2fa reset -u admin
./corefusion admin list

# 查看版本 / 帮助
./corefusion version
./corefusion help
```

> 本机无 systemd 单元，重启方式为结束旧进程后重新 `nohup` 启动；日志见 `/var/log/corefusion.log`。

---

> **文档版本**：v1.0  
> **最后更新**：2026-10-10  
> **适用版本**：CoreFusion v1.0+
