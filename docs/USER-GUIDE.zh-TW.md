<!-- source: USER-GUIDE.md | locale: zh-TW | updated: 2026-10-23 -->
# CoreFusion 操作使用手冊

> 語言： [简体中文](USER-GUIDE.md) · [English](USER-GUIDE.en.md) · 繁體中文 · [Русский](USER-GUIDE.ru.md) · [日本語](USER-GUIDE.ja.md) · [한국어](USER-GUIDE.ko.md) · [Deutsch](USER-GUIDE.de.md) · [Français](USER-GUIDE.fr.md) · [Español](USER-GUIDE.es.md) · [Português](USER-GUIDE.pt.md) · [Türkçe](USER-GUIDE.tr.md) · [Bahasa Indonesia](USER-GUIDE.id.md) · [ไทย](USER-GUIDE.th.md) · [Tiếng Việt](USER-GUIDE.vi.md) · [العربية](USER-GUIDE.ar.md) · [فارسی](USER-GUIDE.fa.md)

> 本手冊面向 CoreFusion 使用者，涵蓋從安裝部署到日常運維的完整操作流程。

---

## 目錄

1. [系統概述](#1-系統概述)
2. [安裝部署](#2-安裝部署)
3. [初始配置](#3-初始配置)
4. [儀表盤](#4-儀表盤)
5. [節點管理](#5-節點管理)
6. [協議中心](#6-協議中心)
7. [代理管理](#7-代理管理)
8. [分流資源](#8-分流資源)
9. [訂閱中心](#9-訂閱中心)
10. [客戶管理](#10-客戶管理)
11. [套餐管理](#11-套餐管理)
12. [充值管理](#12-充值管理)
13. [工單管理](#13-工單管理)
14. [證書管理](#14-證書管理)
15. [流量配額](#15-流量配額)
16. [鏈式代理](#16-鏈式代理)
17. [線上使用者](#17-線上使用者)
18. [使用者管理](#18-使用者管理)
19. [門戶站點](#19-門戶站點)
20. [系統設定](#20-系統設定)
21. [執行控制檯](#21-執行控制檯)
22. [資料庫監控](#22-資料庫監控)
23. [審計日誌](#23-審計日誌)
24. [日誌管理](#24-日誌管理)
25. [運營擴充套件](#25-運營擴充套件)
26. [常見問題](#26-常見問題)

---

## 1. 系統概述

CoreFusion 是一款多協議代理管理面板，支援 VLESS、VMess、Trojan、Shadowsocks、Hysteria2、TUIC 等多種協議，提供節點管理、訂閱分發、客戶管理、計費充值、證書管理、鏈式代理等完整功能。

### 1.1 核心功能

| 功能模組 | 說明 |
|---------|------|
| 多協議支援 | VLESS、VMess、Trojan、Shadowsocks、Hysteria2、TUIC、WireGuard 等 |
| 多核心支援 | Xray-core、sing-box 兩個核心程序（Hysteria2/TUIC 等 QUIC 協議由 sing-box 承載） |
| 多節點管理 | 支援獨立、映象、主備、鏈式入口、鏈式出口五種節點型別 |
| 訂閱分發 | 支援多格式訂閱連結、訂閱端點、訂閱格式管理 |
| 計費系統 | 套餐模板、充值卡、餘額管理、訂單管理、代理員佣金 |
| 證書管理 | Let's Encrypt 自動簽發、DNS-01/HTTP-01 驗證、證書分發 |
| 鏈式代理 | 多出口負載均衡、外部節點註冊、鏈式路由 |
| 流量配額 | 節點級月流量監控、自動暫停、重置日誌 |
| 門戶系統 | 視覺化門戶編輯器、多語言、自定義模板 |
| 通知系統 | 郵件、Telegram 通知，支援模板和群發 |

### 1.2 技術架構

- **後端**：Go 語言，gRPC 通訊
- **前端**：Vue 3 + Element Plus
- **資料庫**：SQLite（預設）/ MySQL / PostgreSQL
- **代理**：Caddy / Nginx 反向代理
- **面板埠**：由初始化嚮導指定（預設隨機空閒埠，可填 8080）
- **Agent gRPC 埠**：預設 18443

---

## 2. 安裝部署

### 2.1 支援的環境

| 專案 | 支援範圍 |
|------|----------|
| 發行版 | Debian / Ubuntu、RHEL / CentOS / Rocky / AlmaLinux / Fedora、Alpine、Arch、openSUSE |
| CPU 架構 | 官方二進位制：`amd64`、`arm64`；原始碼編譯另支援 `armv7`、`armv6`、`386`、`ppc64le`、`s390x`、`riscv64`、`loong64` |
| 服務管理 | systemd（推薦）；容器等無 systemd 環境自動回落 `nohup` |
| 記憶體 / 磁碟 | ≥ 1 GB / ≥ 10 GB（資料庫、證書、日誌與核心二進位制都放在資料目錄） |
| 面板埠 | 由初始化嚮導指定（預設隨機空閒埠，可填 `8080` 等）；初始化嚮導自身使用 18000–18999 的隨機埠 |
| Agent gRPC | 預設 `18443` |

面板與 Agent 的**執行時外部命令**由安裝指令碼自動補齊（`scripts/install.sh` 與面板生成的
Agent 安裝指令碼都會做，`--no-deps` / `CF_SKIP_DEPS=1` 可跳過）：

| 用途 | 依賴 |
|------|------|
| 防火牆 / 限速 / 併發標記 | `nftables`（`nft`） |
| 限速佇列、網絡卡與連線檢視 | `iproute2`（`tc` / `ip` / `ss`） |
| 無 nft 時回退 | `iptables`（EL8+ 為 `iptables-nft`，EL7 為 `iptables`） |
| 併發連線統計 | `conntrack-tools`（`conntrack`） |
| 載入 `nf_conntrack` / `ifb` | `kmod`（`modprobe` / `lsmod`） |
| 程序檢測與清理 | `procps`（`pgrep` / `pkill`） |
| 埠占用檢測 / DH 引數 | `lsof` / `openssl` |
| 代理使用者建立 | `shadow-utils`（Debian `passwd`，Alpine/Arch `shadow`） |
| 下載與解包 | `curl` / `ca-certificates` / `tar` / `gzip` / `xz` |

> 缺失 `nft` / `tc` 隻影響限速與防火牆能力，不影響面板啟動。
> 安裝前可先用 `--cpu-info` 檢視當前環境識別結果與依賴缺口。

### 2.2 一鍵線上安裝（推薦）

```bash
curl -fsSL https://raw.githubusercontent.com/qist/corefusion/main/scripts/install.sh -o install.sh
sudo bash install.sh
```

也可以直接管道執行（無互動、無額外引數）：

```bash
curl -fsSL https://raw.githubusercontent.com/qist/corefusion/main/scripts/install.sh | sudo bash
```

指令碼依次完成：

1. 識別發行版、CPU 架構與 libc，安裝基礎包與執行時依賴；
2. 從 GitHub Release 下載 `corefusion-linux-<arch>`，並用同名 `.sha256` 校驗；
3. 安裝到 `/opt/corefusion`，建立 `data/{sqlite,certs,logs,runtime}`；
4. 不寫資料庫配置，只預置 `SAFE_PATHS=data` 的佔位 `.env`，保證首次啟動進入初始化嚮導（完整 `.env` 由嚮導生成）；
5. 寫入 systemd 服務並啟動（無 systemd 時用 `nohup` 後臺執行）；
6. 等待就緒並列印初始化地址、令牌與各目錄位置。

安裝完成後輸出類似：

```text
==================================================================
 CoreFusion 安裝完成
------------------------------------------------------------------
 安裝目錄 : /opt/corefusion
 資料目錄 : /opt/corefusion/data
 二進位制   : /opt/corefusion/corefusion
 日誌     : journalctl -u corefusion -f
------------------------------------------------------------------
 初始化嚮導 : http://<伺服器IP>:<隨機埠>/init?init_token=<令牌>
 初始化令牌 : <令牌>
 有效期     : 2 小時（見啟動日誌）
 提示       : 令牌只顯示一次；完成後嚮導埠關閉並自動切換到正式面板。
==================================================================
```

如果面板只監聽 `127.0.0.1`（例如已配置反向代理），指令碼會改為提示 SSH 隧道命令，而不是給一個訪問不到的地址。

> 令牌丟失可重新檢視啟動日誌：`journalctl -u corefusion -n 50 --no-pager`
> （無 systemd 時為 `tail -n 50 /opt/corefusion/data/logs/panel.log`）。

### 2.3 指定版本 / 國內加速 / 離線安裝

```bash
# 指定版本
sudo bash install.sh --version v0.13.0-alpha.2

# GitHub 下載加速（作為 URL 字首）
sudo bash install.sh --mirror https://ghproxy.net --version latest

# 使用本地二進位制（離線 / 內網 / 已交叉編譯）
sudo bash install.sh --local ./dist/corefusion-linux-arm64

# 明確指定目標架構
sudo bash install.sh --arch arm64 --local ./corefusion-linux-arm64
```

> 不指定 `--version` 時取 GitHub Release 列表中最新的一條。本專案釋出多為 pre-release，
> 因此指令碼不會使用 `/releases/latest` 介面。
>
> 官方 Release 只提供 `amd64` / `arm64`；其他架構請用原始碼模式，或自行交叉編譯後配合 `--local`。

### 2.4 原始碼編譯安裝

```bash
sudo bash install.sh --source --version v0.13.0-alpha.2
```

`--source` 會克隆原始碼（指令碼位於原始碼目錄內時直接使用現有原始碼）並執行 `make panel`。
依賴版本按系統環境自動選擇：

| 依賴 | 選擇策略 |
|------|----------|
| Go | 讀取 `go.mod` 的 `toolchain` / `go` 指令（當前要求 **Go 1.27+**）；系統版本不足時下載對應工具鏈到 `<安裝目錄>/.toolchain` |
| Node | 預設 **Node 22 LTS**（Vite 構建要求 ≥ 18）；系統版本不足時下載官方構建 |
| Node（glibc < 2.28） | 自動改用 `unofficial-builds` 的 `glibc-217` 構建（如 CentOS 7） |
| Node（Alpine / musl） | 使用發行版 `nodejs` / `npm` |

可覆蓋的變數：

```bash
# 指定 Go / Node 版本
sudo CF_GO_VERSION=1.27.5 CF_NODE_MAJOR=22 bash install.sh --source

# Go 工具鏈映象（預設 https://go.dev/dl）
sudo CF_GO_MIRROR=https://golang.google.cn/dl bash install.sh --source
```

手動編譯（開發 / 除錯）：

```bash
git clone https://github.com/qist/corefusion.git
cd corefusion

# 前端 + 本機架構面板；前端透過 go:embed 打進二進位制
make panel

# 交叉編譯
make panel-linux-amd64
make panel-linux-arm64
```

> `go build ./...` 不會產出可執行檔案。面板二進位制必須用
> `go build -o corefusion ./cmd/corefusion/` 或 `make panel`。

### 2.5 安裝引數速查

| 引數 | 說明 |
|------|------|
| `-d, --dir <PATH>` | 安裝目錄，預設 `/opt/corefusion` |
| `-v, --version <TAG>` | 面板版本，如 `v0.13.0-alpha.2`，預設 `latest` |
| `--binary` | 使用 GitHub Release 二進位制（預設） |
| `--source` | 原始碼編譯安裝，自動準備匹配的 Go / Node |
| `--local <FILE>` | 使用本地二進位制檔案（離線安裝） |
| `--arch <ARCH>` | 指定 CPU 架構：`amd64` / `arm64` / `armv7` / `armv6` / `386` / `ppc64le` / `s390x` / `riscv64` / `loong64` |
| `--mirror <URL>` | GitHub 下載加速字首，如 `https://ghproxy.net` |
| `--repo <URL>` | 原始碼倉庫地址，預設 `https://github.com/qist/corefusion.git` |
| `--ref <REF>` | 原始碼分支 / 標籤，預設取 `--version`，否則 `main` |
| `--no-deps` | 跳過系統依賴安裝（離線或已預裝環境） |
| `--no-start` | 只安裝並寫入服務檔案，不啟動 |
| `--cpu-info` | 只輸出環境識別與依賴缺口，不執行安裝 |
| `--uninstall` | 停止服務並刪除程式（保留資料與 `.env`） |
| `--purge` | 與 `--uninstall` 連用，同時刪除資料目錄與 `.env` |

相關環境變數：

| 變數 | 作用 |
|------|------|
| `CF_GO_VERSION` | 覆蓋原始碼模式要求的 Go 版本 |
| `CF_NODE_MAJOR` | 覆蓋原始碼模式的 Node 主版本（預設 22） |
| `CF_GO_MIRROR` | Go 工具鏈下載映象 |
| `CF_SKIP_DEPS` | Agent 安裝指令碼中設為 `1` 可跳過依賴安裝 |

### 2.6 安裝目錄結構

```text
/opt/corefusion/
├── corefusion                 # 面板二進位制
├── corefusion.bak             # 升級前的上一版（存在時）
├── .env                       # 環境配置（0600，初始化嚮導生成）
├── .toolchain/                # 原始碼模式下載的 Go / Node（僅原始碼安裝）
└── data/                      # 資料目錄（SAFE_PATHS）
    ├── sqlite/app.db          # 預設 SQLite 資料庫
    ├── certs/                 # 證書與私鑰
    ├── logs/                  # 無 systemd 時的執行日誌
    ├── runtime/               # 核心執行配置與狀態
    ├── binaries/              # 下載的 xray / sing-box 等核心
    └── proxy/                 # 代理（Caddy/Nginx）站點與配置
```

> 安裝指令碼只建立 `sqlite/`、`certs/`、`logs/`、`runtime/`；`binaries/`、`proxy/`、`rules/` 等
> 在面板啟動或安裝核心後自動建立。
> 資料目錄位置由 `.env` 的 `SAFE_PATHS` 決定，可以是絕對路徑。更換安裝目錄不會自動搬遷資料。

### 2.7 服務管理

systemd（大多數發行版）：

```bash
systemctl status corefusion --no-pager
systemctl restart corefusion
systemctl stop corefusion
journalctl -u corefusion -f          # 實時日誌
journalctl -u corefusion -n 100      # 最近 100 行
```

無 systemd（容器等）：

```bash
cd /opt/corefusion && ./corefusion            # 前臺執行（除錯）
cd /opt/corefusion && setsid nohup ./corefusion >> data/logs/panel.log 2>&1 &
tail -f data/logs/panel.log
```

> 無 systemd 時程序不會開機自啟，伺服器重啟後需要重新拉起（或重跑安裝指令碼）。

### 2.8 升級與解除安裝

升級（保留資料與配置）：

```bash
sudo bash install.sh --version <新版本>
```

指令碼會停止舊服務、備份為 `corefusion.bak`、替換二進位制並重啟；已有 `.env` 與 `data/` 不會被動。

解除安裝：

```bash
# 只刪程式，保留資料與 .env
sudo bash install.sh --uninstall

# 連資料目錄與 .env 一起刪除（不可恢復）
sudo bash install.sh --uninstall --purge
```

### 2.9 環境配置（.env）

完整的 `.env` 由**初始化嚮導在儲存配置時生成**（資料庫 DSN、資料目錄、金鑰、埠、域名等）。

安裝指令碼只在全新安裝時預置一行佔位，避免面板把已經存在的 `data/` 誤判為“已初始化”而跳過嚮導：

```env
SAFE_PATHS=data
```

> 已有完整配置（`.env` 中含 `CF_DB_DSN`）時，安裝 / 升級指令碼不會覆蓋，也不會再進入初始化嚮導。
> 之後手工編輯 `.env` 同樣生效，改完重啟面板即可。

#### 資料目錄

```env
SAFE_PATHS=data                       # 相對安裝目錄
# SAFE_PATHS=/srv/corefusion-data     # 或絕對路徑
```

#### 資料庫配置

`CF_DB_DSN` 根據字首自動識別資料庫型別：

| 字首 | 資料庫 |
|------|--------|
| 不設定或裸路徑 | SQLite（預設） |
| `sqlite://` | SQLite（顯式） |
| `mysql://` | MySQL |
| `postgres://` / `postgresql://` | PostgreSQL |

`CF_DB_TYPE` 可選，用於覆蓋自動檢測（`sqlite` / `mysql` / `postgres`）。

> **密碼加密儲存**：透過面板寫入（初始化嚮導 / 資料庫切換）的 MySQL/PostgreSQL
> DSN 含資料庫口令，會以 `enc:...` 形式（AES-256-GCM，金鑰為 `CF_SECRET_ENCRYPTION_KEY`）
> 加密存進 `.env`；讀取時自動解密，手工寫明文 DSN 同樣相容。

**SQLite（預設）**

```env
# 裸路徑（自動識別為 SQLite）
CF_DB_DSN=data/sqlite/app.db

# 或顯式 sqlite:// 字首
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

#### 連線池（可選，MySQL/PostgreSQL）

```env
CF_DB_MAX_OPEN_CONNS=100
CF_DB_MAX_IDLE_CONNS=10
```

#### 其他常用項

| 變數 | 說明 |
|------|------|
| `JWT_SECRET` | 登入令牌簽名金鑰；缺失時首次啟動自動生成 |
| `CF_SECRET_ENCRYPTION_KEY` | 敏感配置加密金鑰（AES-256-GCM）；缺失時自動生成 |
| `CF_INITIAL_ADMIN_PASSWORD` | 首次初始化時的管理員口令（可不設，預設隨機生成並只列印一次） |
| `CF_NETWORK_TRAFFIC_ACCOUNTING` | 設為 `0` 關閉基於 iptables 的流量統計（預設開啟） |
| `CF_NETWORK_TRAFFIC_TABLE` | 流量統計使用的 iptables 表名，預設 `corefusion` |

> 可信反向代理地址（用於取真實客戶端 IP）不再透過環境變數配置，而是在
> 「系統設定 → 網路」中以 `panel.trusted_proxies` 儲存。

> `.env` 含金鑰，面板啟動時會自動把許可權收緊到 `0600`。不要提交到版本庫。

### 2.10 使用 Docker（可選，需自行構建映象）

倉庫**未內建** Dockerfile，如需容器化請自行編寫映象。參考執行引數：

```bash
docker run -d \
  --name corefusion \
  -p 8080:8080 \
  -p 18443:18443 \
  -v /opt/corefusion/data:/app/data \
  -v /opt/corefusion/.env:/app/.env \
  <your-image>
```

> 容器內通常沒有 systemd 與 `nft` 許可權，限速 / 防火牆相關功能不可用；
> 工作目錄需與 `SAFE_PATHS` 的相對路徑基準一致。

### 2.11 安裝後自檢

```bash
# 環境與依賴
sudo bash install.sh --cpu-info

# 服務狀態
systemctl is-active corefusion

# 啟動日誌（含初始化地址與令牌）
journalctl -u corefusion -n 50 --no-pager

# 監聽埠
ss -lntp | grep corefusion
```

---

## 3. 初始配置

首次啟動時，若 `.env` 中沒有有效的 `CF_DB_DSN`，面板**不會**直接提供管理介面，而是進入
**初始化嚮導**：只用 18000–18999 的隨機埠和一次性令牌提供服務，配置儲存後自動重啟為正式面板。

### 3.1 開啟初始化嚮導

1. 從安裝指令碼輸出中複製「初始化嚮導」地址；
2. 或在伺服器上檢視日誌：

```bash
journalctl -u corefusion -n 50 --no-pager | grep -A2 '初始化嚮導'
```

```text
CoreFusion 首次啟動 - 初始化嚮導
  訪問地址: http://<IP>:<埠>/init?init_token=<令牌>
  初始化令牌: <令牌>
  有效期: 2h0m0s
```

3. 瀏覽器開啟該地址（含 `init_token`），令牌無需手工輸入。

> 嚮導會話預設 **2 小時**過期。過期或面板重啟後需重新獲取；令牌不要分享給他人。
> 初始化完成後該埠關閉，僅正式面板埠對外服務。

### 3.2 嚮導步驟

嚮導共 8 步，可隨時「上一步」修改，全部完成後才落庫：

| 步驟 | 內容 |
|------|------|
| 1. 資料庫 | SQLite / MySQL / PostgreSQL |
| 2. 資料目錄 | 寫入 `.env` 的 `SAFE_PATHS` |
| 3. 加密金鑰 | `JWT_SECRET` 與 `CF_SECRET_ENCRYPTION_KEY` |
| 4. 核心安裝 | 選擇要安裝的代理核心 |
| 5. 代理 | 不使用 / Nginx / Caddy |
| 6. 站點與域名 | 埠、訪問路徑、管理員賬號、貨幣、時區、域名 |
| 7. HTTPS 證書 | 不啟用 / 上傳證書 / ACME 自動申請 |
| 8. 配置清單 | 複核、下載 `.env`、儲存並重啟 |

#### 步驟 1：資料庫

- **SQLite（本地檔案，推薦）**：檔案路徑留空即使用「資料目錄/sqlite/app.db」。
- **MySQL / PostgreSQL**：填寫主機、埠、使用者名稱、密碼、庫名；PostgreSQL 可選 `disable` / `require` / `verify-full`。
- 點「測試連線」可先驗證；儲存下一步時也會測試。庫不存在會自動嘗試建立（賬號無 `CREATE` 許可權時請先手工建庫）。
- 資料庫連線串（含密碼）會加密寫入 `.env`。

#### 步驟 2：資料目錄

存放資料庫、證書、日誌、核心二進位制等，即 `.env` 的 `SAFE_PATHS`。預設 `data`（相對安裝目錄），
也可以填絕對路徑。**建立後不要隨意改動**，否則會找不到已有資料庫與證書。

#### 步驟 3：加密金鑰

- `JWT_SECRET`：登入令牌簽名金鑰。
- `CF_SECRET_ENCRYPTION_KEY`：敏感配置（資料庫口令、證書 DNS 憑據、充值卡等）的加密金鑰。
- 兩者預設隨機生成，可用「隨機生成」重新取值。**更換 `CF_SECRET_ENCRYPTION_KEY` 會導致已加密資料無法解密**，務必備份。

#### 步驟 4：核心安裝

勾選需要安裝的代理核心：

- **Xray**：VLESS / VMess / Trojan / Shadowsocks 等；
- **sing-box**：Hysteria2 / TUIC 等 QUIC 協議由其承載。

儲存並重啟後由面板自動下載安裝，可在「協議中心」檢視版本與執行狀態。也可以稍後再裝。

#### 步驟 5：代理

- **不使用**：面板直接對外提供服務（需要自己處理 TLS）。
- **Nginx / Caddy**：由面板管理反向代理與證書託管，二者選一。

選擇代理後，面板通常只監聽 `127.0.0.1`，由代理對外服務。

#### 步驟 6：站點與域名

| 欄位 | 說明 |
|------|------|
| 監聽地址 | 自動推導：走代理 → `127.0.0.1`；不走代理且配置了證書 → `0.0.0.0`；其餘 → `127.0.0.1` |
| 管理面板埠 | 預設隨機分配空閒埠，可改（如 8080） |
| 訂閱服務埠 | 可選，`0` 表示與主面板共用埠 |
| 門戶站點埠 | 可選，`0` 表示與主面板共用埠 |
| 訪問路徑 | 管理面板前端路徑，預設 `/`；API 固定為 `/api/v1` |
| 管理員使用者名稱 / 密碼 | 預設隨機生成；**口令只在配置清單顯示一次**，務必儲存 |
| 訪問方式 | 「域名」或「IP」 |
| 預設貨幣 / 時區 | 充值卡與餘額的預設貨幣；面板時間與代理日誌時區（如 `Asia/Shanghai`） |
| 一鍵新增到反向代理 | 選了代理且填了域名時，可把面板 / 訂閱 / 門戶域名一起反代 |

填了域名會寫入對應配置（回撥域名、訂閱端點、分享域名）。

#### 步驟 7：HTTPS 證書

| 證書來源 | 說明 |
|----------|------|
| 暫不啟用 HTTPS | 面板/門戶/訂閱以 HTTP 提供服務（不安全）；反向代理場景可稍後在代理側補證書 |
| 上傳自定義證書 | 貼上證書 PEM 與私鑰 PEM（僅域名訪問方式） |
| ACME 自動申請 | 提交這一步即完成簽發，支援域名與 IP 證書 |

ACME 選項：

- **CA 服務商**：Let's Encrypt、Let's Encrypt (Staging)、ZeroSSL、Google Public CA；
- **證書目標**：「配置域名」或「使用 IP」（僅 Let's Encrypt 支援 IP 證書）；
- **驗證方式**：`HTTP-01`（需本機 80 埠）或 `DNS-01`（支援泛域名、無需 80 埠，需選擇 DNS 服務商並填憑據）；
- **金鑰型別 / 長度**：ECC 256/384 或 RSA 2048/3072/4096；
- **Google Public CA**：需要 EAB，可手工填 Key ID / HMAC，或用 GCP 服務賬號自動申請。

> IP 證書會自動檢測本機公網 IPv4/IPv6。若面板位於代理後面，簽發 IP 證書時應注意取的是本機地址，
> 而不是代理出口地址。域名證書也可以在完成初始化後到「證書管理」中申請。

#### 步驟 8：配置清單

展示資料庫、資料目錄、核心、代理、埠、管理員賬號、域名與 `.env` 鍵值（敏感項脫敏），並提供：

- **下載 .env**：儲存一份配置副本；
- **訪問地址**：面板 / 訂閱 / 門戶的最終 URL，HTTP 會標記「不安全」。

點「儲存並重啟」後寫入資料庫與 `.env`，面板自動重啟切換到正式服務。

### 3.3 初始化完成

- 服務自動重啟，啟動日誌會列印正式監聽地址；
- 用配置清單中的管理員賬號 + 訪問地址 + 訪問路徑登入；
- 若口令丟失，可用 `./corefusion admin reset-password` 離線重置。

### 3.4 CLI 非互動初始化

適合自動化部署 / 無瀏覽器環境，直接落庫並標記完成：

```bash
cd /opt/corefusion
./corefusion init \
  --db-type sqlite \
  --data-dir /opt/corefusion/data \
  --admin-username admin \
  --admin-password '改成強口令' \
  --listen-port 8080 \
  --proxy-type caddy \
  --cores xray,singbox \
  --panel-domain panel.example.com \
  --tls-mode acme --tls-domain panel.example.com --tls-email you@example.com
```

常用引數：

| 引數 | 說明 |
|------|------|
| `--db-type` / `--db-host` / `--db-port` / `--db-user` / `--db-password` / `--db-name` | 資料庫配置 |
| `--data-dir` | 資料目錄，預設取 `SAFE_PATHS` 或 `./data` |
| `--listen-port` / `--subscription-port` / `--portal-port` / `--web-path` | 埠與訪問路徑（預設隨機空閒埠） |
| `--admin-username` / `--admin-password` | 管理員賬號（口令留空則隨機生成並只列印一次） |
| `--proxy-type` | `none` / `nginx` / `caddy` |
| `--cores` | 要安裝的核心，逗號分隔：`xray,singbox` |
| `--panel-domain` / `--subscription-domain` / `--portal-domain` | 域名（會寫入配置並加入反向代理） |
| `--tls-mode` | `none` / `self_signed` / `acme` |
| `--tls-domain` / `--tls-email` / `--tls-challenge` / `--tls-key-type` / `--tls-key-bits` | 證書引數（`--tls-challenge` 預設 `http-01`；用 `dns-01` 時需配合 `--tls-dns-channel` 或 `--tls-dns-provider`） |
| `--tls-ip-cert` | 為自動檢測到的公網 IP 簽發證書（僅 Let's Encrypt） |
| `--acme-provider` | `lets_encrypt` / `lets_encrypt_staging` / `zerossl` / `google` |
| `--currency` / `--timezone` | 預設貨幣 / 時區 |

完整引數：`./corefusion init --help`。初始化後啟動：

```bash
systemctl restart corefusion
```

### 3.5 重新初始化

需要重走向導時（會刪除 `.env` 與初始化標記，**保留資料**）：

```bash
cd /opt/corefusion
./corefusion reset-init      # 需輸入 RESET 確認
systemctl restart corefusion
```

重啟後按啟動日誌中的新地址與令牌重新進入嚮導。

> 重新初始化會重置管理員與訪問路徑；舊資料庫中的業務資料仍在，但登入賬號可能需要重建。
>
> `reset-init` 會刪除整個 `.env`（包括 `SAFE_PATHS`）。**如果資料目錄是自定義絕對路徑**，
> 重啟前先補回 `SAFE_PATHS`，否則面板會按預設的 `./data` 找庫：
>
> ```bash
> printf 'SAFE_PATHS=/srv/corefusion-data\n' > /opt/corefusion/.env
> ```

### 3.6 首次登入後的安全設定

#### 修改初始密碼

1. 點選右上角使用者名稱；
2. 選擇「修改密碼」；
3. 輸入原密碼與新密碼；
4. 點選「儲存」。

#### 啟用動態碼登入（2FA / TOTP）

> 啟用後賬號改為**動態碼登入**：登入時不再接受靜態密碼，只認認證器生成的 6 位動態碼
> （或一次性恢復碼）。這是“用動態碼替代密碼登入”，不是“密碼 + 驗證碼”兩步驗證。

1. 點選右上角使用者名稱；
2. 選擇「開啟 2FA」；
3. 輸入當前密碼（僅用於本次啟用確認）；
4. 用認證器 App（Google Authenticator / 1Password 等）掃描二維碼，或手動輸入下方金鑰；
5. 輸入認證器生成的 6 位動態碼完成啟用；
6. **立即儲存恢復碼**（可下載檔案），每張只能使用一次。

啟用後的登入方式：

- 填使用者名稱後，密碼框會變為「6 位驗證碼 / 恢復碼」，按鈕變為「動態碼登入」；
- 輸入認證器當前動態碼即可，**無需也不能再用靜態密碼**；
- 恢復碼用一張少一張，建議用完後在「動態碼登入（2FA）」中重新生成（重新生成會使舊碼全部作廢）。

認證器丟失且恢復碼用完時，在伺服器上離線關閉：

```bash
cd /opt/corefusion
./corefusion admin 2fa reset -u admin    # 關閉 2FA、清空金鑰與恢復碼、吊銷全部會話
```

> 唯一的超級管理員如果既沒有認證器也沒有恢復碼，網頁端無法自助恢復，只能走上面的命令。

#### 配置 Passkey（可選）

1. 點選右上角使用者名稱；
2. 選擇「Passkey 管理」；
3. 點選「註冊新 Passkey」；
4. 按提示完成裝置認證。

> **注意**：Passkey 需要 HTTPS 或 localhost 環境，且域名需在白名單中。

---

## 4. 儀表盤

儀表盤展示多節點叢集總覽與節點健康監控。

### 4.1 檢視內容

- **系統資訊**：主機名、系統版本、執行時間、CPU、記憶體
- **流量統計**：實時頻寬、總流量、月度趨勢
- **節點健康**：線上/離線/降級節點數、高負載節點
- **異常告警**：離線節點、降級節點、即將過期證書、配額預警
- **叢集資源**：平均 CPU、記憶體、磁碟、吞吐

### 4.2 操作

- 點選「檢視詳情」進入節點詳情頁
- 點選節點卡片檢視實時狀態
- 使用篩選器按狀態、型別、區域過濾

---

## 5. 節點管理

節點管理是 CoreFusion 的核心功能，支援多種節點型別。

### 5.1 節點型別

| 型別 | 說明 |
|------|------|
| 獨立節點 (standalone) | 獨立執行的核心節點 |
| 映象節點 (mirror) | 從目標節點同步配置 |
| 主備節點 (primary_backup) | 互為故障切換的夥伴節點 |
| 鏈式入口 (chain_entry) | 鏈式代理的入口節點 |
| 鏈式出口 (chain_exit) | 鏈式代理的出口節點 |

### 5.2 新增節點

1. 進入「節點管理」頁面
2. 點選「新增伺服器」
3. 填寫基本資訊：
   - 名稱：如 `香港-HK01`
   - 主機名：IP 或域名
   - 區域：如 `HK`、`JP`、`US`
   - 節點型別：選擇型別
4. 配置 Agent 埠（預設 18443）
5. 選擇支援的核心
6. 點選「儲存」

### 5.3 一鍵部署

1. 新增節點後，點選「Bootstrap 命令」
2. 複製安裝命令
3. 在遠端節點上以 root 執行：
   ```bash
   curl -fsSL https://your-panel/install.sh | bash
   ```
4. Agent 將自動下載、註冊並啟動

### 5.4 節點詳情

點選節點進入詳情頁，可檢視：
- **概覽**：基本資訊、Agent 狀態、證書狀態
- **協議中心**：核心程序配置
- **代理**：Caddy/Nginx 配置
- **核心程序**：啟動/停止/重啟
- **WARP**：WARP 配置
- **直連域名**：域名管理
- **限速**：流量限速配置
- **證書**：證書繫結
- **線上使用者**：實時線上使用者

### 5.5 節點操作

- **同步配置**：將面板配置推送到節點
- **強制重連**：斷開 Agent 連線並自動重連
- **更新 Agent**：推送 Agent 更新
- **吊銷證書**：吊銷節點 Agent 證書

---

## 6. 協議中心

協議中心用於配置核心程序的協議引數。

### 6.1 支援的核心

| 核心 | 說明 |
|------|------|
| Xray-core | VLESS/VMess/Trojan/Shadowsocks 等多協議 |
| sing-box | 模組化通用代理平臺，支援 TUN，並承載 Hysteria2/TUIC 等 QUIC 協議 |

### 6.2 配置模組

- **日誌**：訪問日誌、錯誤日誌、輸出方式
- **API**：gRPC API 監聽地址、服務列表
- **DNS**：DNS 伺服器、查詢策略、靜態 Hosts
- **路由**：路由規則、負載均衡、VLESS 路由
- **入站**：入站列表、協議配置、傳輸、TLS
- **出站**：出站列表、協議配置、傳輸
- **傳輸**：WebSocket、gRPC、XHTTP、TUN 等
- **TLS**：證書、ALPN、uTLS 指紋、ECH
- **Reality**：Target 管理、金鑰對、Short ID
- **健康檢測**：探測 URL、間隔、併發
- **FakeDNS**：IP 池、池大小
- **指標**：Prometheus 指標監聽
- **策略**：連線超時、緩衝區、統計
- **Geo 資料**：自動更新、資原始檔
- **版本限制**：最低/最高版本

### 6.3 入站配置

1. 選擇核心和協議
2. 配置基礎資訊：
   - 標籤（tag）
   - 監聽地址和埠
   - 協議型別
3. 配置協議引數（UUID、密碼等）
4. 配置傳輸方式（TCP、WebSocket、gRPC 等）
5. 配置 TLS（證書、ALPN）
6. 儲存並應用到核心

### 6.4 出站配置

1. 新增出站
2. 配置標籤和協議
3. 配置伺服器地址和埠
4. 配置傳輸和 TLS
5. 儲存並應用

### 6.5 分享連結

1. 在入站列表點選「分享連結」
2. 選擇格式：
   - 分享連結（URI）
   - Mihomo YAML
   - sing-box JSON
   - Xray JSON
3. 複製連結或生成二維碼

---

## 7. 代理管理

代理管理用於配置 Caddy 或 Nginx 反向代理。

### 7.1 代理型別

- **Caddy**：自動 HTTPS、HTTP/2、HTTP/3
- **Nginx**：高效能、靈活配置

### 7.2 站點配置

1. 進入「代理管理」頁面
2. 選擇代理型別（Caddy/Nginx）
3. 新增站點：
   - 域名
   - 上游地址（upstream）
   - 本地埠（local_port）
4. 儲存配置

### 7.3 配置說明

- **upstream**：後端服務地址，如 `127.0.0.1:8082`
- **local_port**：本地監聽埠，如 `8443`
- **portal_forward**：門戶轉發規則
- **subscription_forward**：訂閱轉發規則

---

## 8. 分流資源

分流資源用於管理訂閱分流規則。

### 8.1 功能

- 管理分流規則
- 配置分流策略
- 支援多種分流格式

### 8.2 操作

1. 進入「分流資源」頁面
2. 新增分流規則
3. 配置規則名稱、型別、條件
4. 儲存並應用

---

## 9. 訂閱中心

訂閱中心用於管理訂閱連結和端點。

### 9.1 訂閱格式管理

1. 進入「訂閱中心」→「訂閱格式管理」
2. 新增格式：
   - 格式名稱
   - 輸出格式（Base64、YAML、JSON 等）
   - 包含的協議
3. 儲存格式

### 9.2 訂閱端點管理

1. 進入「訂閱中心」→「訂閱端點管理」
2. 新增端點：
   - 端點名稱
   - 監聽地址
   - 路徑
   - 關聯格式
3. 儲存端點

### 9.3 訂閱連結

- 訪問 `http://your-domain/sub/:token` 獲取訂閱
- 支援 `?format=yaml` 等引數指定格式

---

## 10. 客戶管理

客戶管理用於管理終端使用者。

### 10.1 新增客戶

1. 進入「客戶管理」頁面
2. 點選「新增客戶」
3. 填寫資訊：
   - 郵箱
   - 密碼
   - 套餐
   - 流量配額
   - 到期時間
4. 儲存

### 10.2 客戶操作

- **編輯**：修改客戶資訊
- **暫停**：暫停客戶服務
- **恢復**：恢復已暫停客戶
- **重置流量**：重置已用流量
- **重置密碼**：重置客戶密碼
- **續費**：延長到期時間
- **升級/降級**：更換套餐
- **刪除**：刪除客戶

### 10.3 訂閱詳情

點選客戶進入詳情頁，可檢視：
- 訂閱連結
- 流量使用
- 訂單歷史
- 交易記錄

---

## 11. 套餐管理

套餐管理用於配置銷售套餐。

### 11.1 套餐模板

1. 進入「套餐管理」→「套餐模板」
2. 新增模板：
   - 模板名稱
   - 分類（標準/高階/企業/試用）
   - 自動續費
   - 試用設定
3. 儲存模板

### 11.2 套餐規格

1. 在模板下新增規格：
   - 規格名稱（如月付、年付）
   - 價格（分）
   - 時長（天）
   - 流量（GB，0=不限）
   - 併發限制
   - 限速（Mbps）
2. 儲存規格

### 11.3 流量疊加包

1. 進入「流量疊加包」
2. 新增疊加包：
   - 名稱
   - 價格
   - 流量
   - 過期模式（延長天數/與套餐同期）
3. 儲存

### 11.4 優惠碼

1. 進入「優惠碼」
2. 新增優惠碼：
   - 優惠碼
   - 型別（百分比/固定金額）
   - 折扣值
   - 使用次數限制
   - 有效期
   - 適用套餐
3. 儲存

---

## 12. 充值管理

充值管理用於管理充值站點、訂單和充值卡。

### 12.1 充值站點

1. 進入「充值管理」→「充值站點」
2. 新增站點：
   - 站點名稱
   - 支援的支付方式（支付寶、微信、USDT、Stripe 等）
   - 貨幣
   - 匯率
3. 儲存站點
4. 驗證站點（HTTP 檔案驗證或 DNS 驗證）

### 12.2 充值訂單

- 檢視所有充值訂單
- 按狀態篩選（待支付、已支付、已失敗、已過期、已退款）
- 檢視訂單詳情
- 管理員退款

### 12.3 充值卡

1. 進入「充值卡」
2. 新增卡：
   - 型別（金額型、套餐型、流量疊加包）
   - 面值/套餐
   - 過期時間
3. 或批次生成：
   - 生成數量（最多 1000 張）
   - 卡號字首
   - 卡號格式
4. 匯出 CSV

### 12.4 餘額管理

- 檢視客戶餘額
- 調整餘額（增加/減少/設定）
- 調整凍結金額
- 檢視交易記錄

### 12.5 代理員

- 新增代理員
- 設定佣金比例
- 檢視累計佣金
- 提現佣金

---

## 13. 工單管理

工單管理用於處理客戶支援請求。

### 13.1 工單列表

- 檢視所有工單
- 按狀態篩選（待處理、已回覆、已關閉）
- 按分類篩選（技術、賬務、賬號、其他）
- 按優先順序篩選（低、普通、高、緊急）

### 13.2 工單操作

- **檢視/回覆**：檢視工單詳情並回復
- **分配**：將工單分配給管理員
- **關閉**：關閉已解決的工單

---

## 14. 證書管理

證書管理用於管理 TLS 證書。

### 14.1 簽發證書

1. 進入「證書管理」頁面
2. 點選「簽發證書」
3. 填寫資訊：
   - 域名列表（每行一個）
   - 簽發型別（域名證書/IP 證書）
   - 金鑰型別（ECC/RSA）
   - 驗證方式（DNS-01/HTTP-01）
   - DNS 提供商
   - 簽發位置（面板本機/節點）
4. 點選「簽發」

### 14.2 證書操作

- **續期**：手動續期證書
- **部署**：將證書部署到節點
- **分發**：分發到所有關聯節點
- **遷移**：將舊證書引用遷移到新證書
- **下載**：下載證書檔案（cert/fullchain/privkey）

### 14.3 上傳證書

1. 點選「上傳證書」
2. 選擇上傳方式：
   - 貼上內容
   - 上傳檔案
3. 上傳證書鏈和私鑰
4. 儲存

### 14.4 EAB 配置

- 管理 Google Public CA / ZeroSSL 的 EAB 配置
- 自動領取 EAB
- 簽發時自動帶上

---

## 15. 流量配額

流量配額用於監控節點月流量。

### 15.1 配額列表

- 檢視所有節點的配額配置
- 顯示配額大小、已用流量、剩餘額度
- 顯示用量佔比

### 15.2 週期用量

- 檢視每個節點的週期用量詳情
- 顯示入站/出站流量
- 顯示週期開始/結束時間

### 15.3 重置日誌

- 檢視流量重置記錄
- 顯示重置週期、觸發來源、操作人

---

## 16. 鏈式代理

鏈式代理用於配置多出口鏈路。

### 16.1 建立鏈路

1. 進入「鏈式代理」頁面
2. 點選「新建鏈路」
3. 選擇入口節點
4. 選擇入口入站
5. 選擇出口節點（可多選）
6. 為每個出口選擇入站
7. 配置優先順序/權重
8. 確認建立

### 16.2 鏈路管理

- 檢視拓撲總覽
- 編輯鏈路（優先順序、權重、出站 tag）
- 刪除鏈路
- 檢視健康狀態

### 16.3 外部節點

- 匯入外部節點（v2ray URI、Base64 URI、訂閱 URL）
- 註冊外部入口
- 管理外部節點

---

## 17. 線上使用者

線上使用者用於檢視實時線上狀態。

### 17.1 檢視內容

- 線上訂閱數
- 線上裝置/IP 數
- 線上節點數
- 實時速率
- 累計流量

### 17.2 篩選

- 按節點篩選
- 搜尋郵箱/客戶/節點/IP

---

## 18. 使用者管理

使用者管理用於管理面板使用者。

### 18.1 角色

| 角色 | 說明 |
|------|------|
| 超級管理員 (superAdmin) | 最高許可權 |
| 管理員 (admin) | 管理許可權 |
| 操作員 (operator) | 操作許可權 |
| 檢視者 (viewer) | 只讀許可權 |

### 18.2 新增使用者

1. 進入「使用者管理」頁面
2. 點選「新增使用者」
3. 填寫資訊：
   - 使用者名稱
   - 郵箱
   - 密碼
   - 角色
4. 儲存

### 18.3 使用者操作

- **編輯**：修改使用者資訊
- **刪除**：刪除使用者

---

## 19. 門戶站點

門戶站點用於配置客戶端門戶頁面。

### 19.1 基本設定

- 站點名稱
- 站點描述
- 開放註冊
- 門戶訪問路徑
- 門戶埠

### 19.2 外觀配置

- 站點標題
- Logo URL
- 主色調、背景色、文字色、強調色
- 字型

### 19.3 導航管理

- 新增導航頁
- 配置頁面名稱、圖示、外部連結
- 管理導航項

### 19.4 模組管理

- 新增/編輯模組
- 配置模組標題、描述、圖示
- 支援 Markdown、富文字、自定義 HTML

### 19.5 多語言

- 新增語言
- 管理文案
- 匯入/匯出 JSON

### 19.6 模板管理

- 切換模板
- 自定義 HTML/CSS/JS
- 重置為預設模板

---

## 20. 系統設定

系統設定用於配置面板引數。

### 20.1 面板配置

- 監聽地址和埠
- 資料目錄
- 時區
- 訂閱埠
- 匯出上限
- 開放註冊
- 面板模式（完整模式/純管理模式）

### 20.2 安全設定

- 加密金鑰管理
- JWT 金鑰管理
- IP 黑名單
- Passkey 白名單域名

### 20.3 OAuth 配置

- GitHub OAuth
- Google OAuth
- 回撥地址配置

### 20.4 通知設定

- 郵件通知（SMTP）
- Telegram 通知
- 通知模板
- 監控設定

### 20.5 備份還原

- 建立備份
- 備份列表
- 下載備份
- 還原備份
- 定時備份
- 遠端儲存（FTP/S3/WebDAV）

### 20.6 自解封配置

- 啟用自解封
- 冷卻時間
- 適用狀態

---

## 21. 執行控制檯

執行控制檯用於管理配置快照和應用任務。

### 21.1 配置快照

- 建立快照
- 檢視快照列表
- 回滾到指定快照
- 刪除快照

### 21.2 應用任務

- 檢視最近任務
- 立即應用配置（DB→執行時）
- 預檢（校驗埠衝突與配置 JSON）
- 取消任務
- 重試任務

### 21.3 節點驗證

- 選擇節點並驗證
- 檢視驗證結果
- 驗證全部節點

### 21.4 緊急停用

- 緊急停用代理（寫入空入站配置）
- 立即停止所有入站流量

---

## 22. 資料庫監控

資料庫監控用於檢視資料庫執行狀態。

### 22.1 監控內容

- 資料庫引擎
- 資料庫大小
- 連通性
- 延遲
- 連線池狀態
- 快取統計
- 查詢統計

### 22.2 支援引擎

- SQLite
- MySQL
- PostgreSQL

---

## 23. 審計日誌

審計日誌用於檢視操作記錄。

### 23.1 檢視內容

- 類別
- 事件型別
- 結果
- 來源 IP
- 使用者
- 詳情

### 23.2 節點操作日誌

- 檢視節點操作記錄
- 按節點篩選

---

## 24. 日誌管理

日誌管理用於檢視系統日誌。

### 24.1 日誌型別

- 面板日誌
- 節點日誌
- 核心日誌
- 代理日誌

### 24.2 操作

- 檢視日誌
- 下載日誌
- 清理日誌

---

## 25. 運營擴充套件

運營擴充套件用於配置擴充套件功能。

### 25.1 功能

- 自定義擴充套件
- 擴充套件配置
- 擴充套件管理

---

## 26. 常見問題

### 26.1 面板無法啟動

- 檢查埠是否被佔用
- 檢查 `.env` 配置是否正確
- 檢視日誌：`journalctl -u corefusion -f`；無 systemd 時 `tail -f <安裝目錄>/data/logs/panel.log`

### 26.2 節點離線

- 檢查 Agent 是否執行：`systemctl status corefusion-agent`
- 檢查網路連線
- 檢查防火牆規則（18443 埠）

### 26.3 證書籤發失敗

- 檢查域名解析
- 檢查 80 埠是否開放
- 檢查 DNS 提供商配置
- 檢視證書日誌

### 26.4 訂閱無法使用

- 檢查訂閱連結是否正確
- 檢查節點是否線上
- 檢查核心程序是否執行
- 檢查防火牆規則

### 26.5 充值回撥失敗

- 檢查回撥地址是否正確
- 檢查站點金鑰是否匹配
- 檢查 IP 白名單
- 檢視訂單日誌

### 26.6 門戶頁面無法訪問

- 檢查門戶埠是否開放
- 檢查門戶路徑配置
- **重點**：確認反向代理把門戶域名轉發到**門戶引擎埠**，而不是面板主埠——轉發到錯誤埠會表現為白屏或開啟面板而非門戶。檢查代理站點記錄裡的 `upstream` / `local_port`。
- 直接訪問 `http://<門戶域名>:<門戶埠>` 驗證門戶引擎本身是否正常
- 檢查反向代理配置
- 檢視門戶日誌

---

## 附錄

### A. 預設埠

| 埠 | 用途 |
|------|------|
| 由初始化嚮導指定（預設隨機空閒埠） | 管理面板（可改） |
| 18443 | Agent gRPC（可改） |
| 0（預設未啟用） | 訂閱引擎獨立埠（留空或 0 表示共用主埠） |
| 0（預設未啟用） | 門戶引擎獨立埠（留空或 0 表示共用主埠） |
| 80 / 443 | 反向代理（Caddy/Nginx）HTTP/HTTPS |

### B. 預設路徑

| 路徑 | 用途 |
|------|------|
| `/` | 管理面板（配置了獨立門戶引擎時，門戶也掛在 `/`） |
| `/sub/:token` | 訂閱拉取（未配置獨立訂閱埠時） |
| `/p` | 門戶頁面（未配置獨立門戶埠時） |
| `/:token` | 訂閱拉取（配置獨立訂閱埠後，無 `/sub` 字首） |

### C. 命令列工具

面板二進位制實際支援以下子命令（無引數時預設啟動 server）：

```bash
# 啟動面板（等價於 ./corefusion server）
./corefusion

# 執行資料庫遷移
./corefusion migrate

# 離線管理命令（無法登入時使用）
./corefusion admin reset-password -u admin -p '新口令'
./corefusion admin 2fa reset -u admin
./corefusion admin list

# 檢視版本 / 幫助
./corefusion version
./corefusion help
```

> 本機無 systemd 單元，重啟方式為結束舊程序後重新 `nohup` 啟動；日誌見 `/var/log/corefusion.log`。

---

> **文件版本**：v1.0  
> **最後更新**：2026-10-10  
> **適用版本**：CoreFusion v1.0+
