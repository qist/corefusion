<!-- source: USER-GUIDE.md | locale: zh-TW | updated: 2026-11-20 -->
# CoreFusion操作指南

> 語言： [简体中文](USER-GUIDE.md) · [English](USER-GUIDE.en.md) · 繁體中文 · [Русский](USER-GUIDE.ru.md) · [日本語](USER-GUIDE.ja.md) · [한국어](USER-GUIDE.ko.md) · [Deutsch](USER-GUIDE.de.md) · [Français](USER-GUIDE.fr.md) · [Español](USER-GUIDE.es.md) · [Português](USER-GUIDE.pt.md) · [Türkçe](USER-GUIDE.tr.md) · [Bahasa Indonesia](USER-GUIDE.id.md) · [ไทย](USER-GUIDE.th.md) · [Tiếng Việt](USER-GUIDE.vi.md) · [العربية](USER-GUIDE.ar.md) · [فارسی](USER-GUIDE.fa.md)

> 本指南適用於 CoreFusion 用戶，涵蓋從安裝到日常操作的完整工作流程。

---

## 內容

1. [系統概述](#1-系統概述)
2. [安裝](#2-安裝)
3. [初始配置](#3-初始配置)
4. [儀表板](#4-儀表板)
5. [節點管理](#5-節點管理)
6. [禮賓中心](#6-禮賓中心)
7. [代理管理](#7-代理管理)
8. [路由資源](#8-路由資源)
9. [訂閱中心](#9-訂閱中心)
10. [客戶管理](#10-客戶管理)
11. [計劃管理](#11-計劃管理)
12. [儲值管理](#12-儲值管理)
13. [票務管理](#13-票務管理)
14. [證書管理](#14-證書管理)
15. [流量配額](#15-流量配額)
16. [鍊式代理](#16-鍊式代理)
17. [線上用戶](#17-線上用戶)
18. [使用者管理](#18-使用者管理)
19. [入口網站](#19-入口網站)
20. [系統設定](#20-系統設定)
21. [操作控制台](#21-操作控制台)
22. [資料庫監控](#22-資料庫監控)
23. [審核日誌](#23-審核日誌)
24. [日誌管理](#24-日誌管理)
25. [營運擴展](#25-營運擴展)
26. [常見問題解答](#26-常見問題解答)
- [附錄](#附錄)

---

## 1. 系統概述

CoreFusion是一個多協定代理管理面板，支援VLESS、VMess、Trojan、Shadowsocks、Hysteria2、TUIC等，具備節點管理、訂閱、Hysteria2、TUIC等，具有節點管理、訂閱、客戶管理、憑證、憑證交付。

### 1.1 核心特點

| 模組 | 描述 |
|--------|-------------|
| 多協議 | VLESS、VMess、Trojan、Shadowsocks、Hysteria2、TUIC、WireGuard等 |
| 多核心 | Xray-core和sing-box（Hysteria2/TUIC等QUIC協定運行在sing-box上） |
| 多節點 | 五種節點類型：獨立、鏡像、主備、鏈入、鏈出 |
| 訂閱交付 | 多種訂閱格式、端點和格式管理 |
| 計費 | 計劃範本、儲值卡、餘額、訂單、經銷商佣金 |
| 證書 | Let's Encrypt發行、DNS-01/HTTP-01驗證、憑證分發 |
| 鍊式代理 | 多出口負載平衡、外部節點註冊、鍊式路由 |
| 流量配額 | 每個節點每月流量監控、自動暫停、重置日誌 |
| 入口網站 | 視覺化入口網站編輯器、多語言、自訂模板 |
| 通知 | 電子郵件和 Telegram、範本和廣播 |

### 1.2 架構

- **後端**：Go、gRPC
- **前端**：Vue 3 + Element Plus
- **資料庫**：SQLite（預設）/MySQL/PostgreSQL
- **代理**：Caddy / Nginx 反向代理
- **面板連接埠**：由設定精靈指派（預設隨機空閒連接埠；8080是常見選擇）
- **代理 gRPC 連接埠**：預設為 18443

---

## 2. 安裝

### 2.1 支持的環境

| 專案 | 支援 |
|------|---------|
| 發行版 | Debian / Ubuntu、RHEL / CentOS / Rocky / AlmaLinux / Fedora、Alpine、Arch、openSUSE |
| CPU架構 | 官方二進位檔案：`amd64`、`arm64`；原始版本還支援 `armv7`、`armv6`、`386`、`ppc64le`、`s390x`、`riscv64`、`loong64` |
| 服務經理 | systemd（建議）；在沒有 systemd 的容器或其他環境中回退到 `nohup` |
| 記憶體/磁碟 | ≥ 1 GB / ≥ 10 GB（資料庫、憑證、日誌和核心二進位檔案位於資料目錄中） |
| 面板連接埠 | 在設定精靈中選擇（預設隨機空閒連接埠；`8080` 和類似的都可以） |
| 代理 gRPC | 預設為`18443` |

執行時間外部命令由安裝程式自動安裝（`scripts/install.sh` 和麵板產生的代理安裝腳本；使用 `--no-deps` / `CF_SKIP_DEPS=1` 跳過）：

| 目的 | 套餐 |
|---------|---------|
| 防火牆/速率限制/併發標記 | `nftables` (`nft`) |
| 流量整形、連結和套接字 | `iproute2` (`tc` / `ip` / `ss`) |
| nft 不可用時的回退 | `iptables`（EL8+ 上為 `iptables-nft`，EL7 上為 `iptables`） |
| 連接追蹤 | `conntrack-tools` (`conntrack`) |
| 負載`nf_conntrack` / `ifb` | `kmod` (`modprobe` / `lsmod`) |
| 過程檢測和清理 | `procps` (`pgrep` / `pkill`) |
| 連接埠檢查/DH參數 | `lsof` / `openssl` |
| 代理用戶創建 | `shadow-utils`（Debian `passwd`、Alpine/Arch `shadow`） |
| 下載和存檔 | `curl` / `ca-certificates` / `tar` / `gzip` / `xz` |

> 缺少`nft` / `tc`僅影響速率限制和防火牆功能；面板仍然啟動。
> 先運行`--cpu-info`，看看環境偵測狀況如何，缺少哪些套件。

### 2.2 一線式安裝（建議）

```bash
curl -fsSL https://raw.githubusercontent.com/qist/corefusion/main/scripts/install.sh -o install.sh
sudo bash install.sh
```

或直接透過管道傳輸（沒有交互，沒有額外的標誌）：

```bash
curl -fsSL https://raw.githubusercontent.com/qist/corefusion/main/scripts/install.sh | sudo bash
```

該腳本將：

1. 偵測發行版、CPU架構和libc，然後安裝基礎套件和執行時間相依性；
2. 從 GitHub Releases 下載 `corefusion-linux-<arch>` 並與匹配的 `.sha256` 進行驗證；
3. 安裝到`/opt/corefusion`並建立`data/{sqlite,certs,logs,runtime}`；
4. 只寫一個佔位符`.env`和`SAFE_PATHS=data`－沒有資料庫設定－這樣第一次啟動就進入設定精靈（完整的`.env`是由精靈產生的）；
5. 安裝並啟動systemd服務（或當systemd不可用時在`nohup`下運行）；
6. 等待準備就緒，然後列印設定 URL、令牌和目錄路徑。

輸出範例：

```text
==================================================================
 CoreFusion installation complete
------------------------------------------------------------------
 Install dir : /opt/corefusion
 Data dir    : /opt/corefusion/data
 Binary      : /opt/corefusion/corefusion
 Logs        : journalctl -u corefusion -f
------------------------------------------------------------------
 Setup wizard : http://<server-ip>:<random-port>/init?init_token=<token>
 Setup token  : <token>
 Valid for    : 2 hours (see startup log)
 Note         : the token is shown once; after setup this port closes and the panel switches to its normal port.
==================================================================
```

如果主機僅偵聽 `127.0.0.1`（例如在反向代理後面），則腳本將列印 SSH 隧道命令，而不是無法存取的 URL。

> 丟失了令牌？再次讀取啟動日誌：`journalctl -u corefusion -n 50 --no-pager`
> （不含 systemd：`tail -n 50 /opt/corefusion/data/logs/panel.log`）。

### 2.2.1 多語言輸出

安裝程式包括四種 UI 語言：簡體中文、英語、波斯語 (فרсо) 和俄語 (Русский)。它會自動偵測系統 `LANG`，或者您可以明確選擇：

```bash
# English
sudo bash install.sh --lang en

# Persian via environment variable
sudo CF_LANG=fa bash install.sh

# Pass options through a pipe
curl -fsSL https://raw.githubusercontent.com/qist/corefusion/main/scripts/install.sh | sudo bash -s -- --lang ru
```

支援值：`zh` / `en` / `fa` / `ru`；`--lang=ru` 也被接受。不支援的顯式值會因可用語言清單而失敗。

### 2.3 版本固定/鏡像/離線安裝

```bash
# Pin a version
sudo bash install.sh --version v0.13.0-rc.1

# GitHub download mirror (used as a URL prefix)
sudo bash install.sh --mirror https://ghproxy.net --version latest

# Use a local binary (offline / intranet / pre-built)
sudo bash install.sh --local ./dist/corefusion-linux-arm64

# Explicit target architecture
sudo bash install.sh --arch arm64 --local ./corefusion-linux-arm64
```

> 如果沒有 `--version`，腳本將使用 GitHub 版本清單中的最新條目。
> 大多數版本都是預版本，因此不使用 `/releases/latest` 端點。
>
> 官方發布僅提供`amd64` / `arm64`；對於其他架構，使用 `--source` 或與 `--local` 交叉編譯的二進位。

### 2.4 從原始碼構建

```bash
sudo bash install.sh --source --version v0.13.0-rc.1
```

`--source` 複製儲存庫（或當腳本位於其中時使用現有的簽出）並執行 `make panel`。根據環境選擇工具鏈版本：

| 依賴性 | 選擇 |
|------------|-----------|
| 去 | 從 `go.mod` 中的 `toolchain` / `go` 讀取（目前 **Go 1.27+**）；如果系統版本太舊，則下載相符的工具鏈到`<install-dir>/.toolchain` |
| 節點 | **預設節點22 LTS**（Vite要求≥18）；需要時下載符合的官方版本 |
| 節點（glibc < 2.28） | 使用 `unofficial-builds` glibc-217 版本（例如 CentOS 7） |
| 節點（高山/穆斯林） | 使用分佈 `nodejs` / `npm` |

覆蓋：

```bash
# Force Go / Node versions
sudo CF_GO_VERSION=1.27.5 CF_NODE_MAJOR=22 bash install.sh --source

# Go toolchain mirror (default https://go.dev/dl)
sudo CF_GO_MIRROR=https://golang.google.cn/dl bash install.sh --source
```

手動建置（開發/調試）：

```bash
git clone https://github.com/qist/corefusion.git
cd corefusion

# Frontend + panel for the current architecture; the frontend is embedded via go:embed
make panel

# Cross-compile
make panel-linux-amd64
make panel-linux-arm64
```

> `go build ./...` 不產生二進位。建構面板
> `go build -o corefusion ./cmd/corefusion/` 或 `make panel`。

### 2.5 安裝程式標誌

| 旗幟 | 描述 |
|------|-------------|
| `-d, --dir <PATH>` | 安裝目錄，預設`/opt/corefusion` |
| `-v, --version <TAG>` | 面板版本，例如`v0.13.0-rc.1`；預設`latest` |
| `--binary` | 使用 GitHub 發布二進位（預設） |
| `--source` | 從原始碼建置並準備匹配的 Go / Node |
| `--local <FILE>` | 使用本地二進位（離線安裝） |
| `--arch <ARCH>` | 目標架構：`amd64` / `arm64` / `armv7` / `armv6` / `386` / `ppc64le` / `s390x` / `riscv64` / `loong64` |
| `--mirror <URL>` | GitHub 下載前綴，例如`https://ghproxy.net` |
| `--repo <URL>` | 源碼倉庫，預設`https://github.com/qist/corefusion.git` |
| `--ref <REF>` | 來源分支/標籤；預設為`--version`，否則為`main` |
| `--no-deps` | 跳過系統依賴安裝 |
| `--no-start` | 不啟動安裝並寫入服務文件 |
| `--cpu-info` | 列印環境檢測和缺少依賴項，然後退出 |
| `--lang <LANG>` | 輸出語言：`zh` / `en` / `fa` / `ru`（預設：系統`LANG`） |
| `--uninstall` | 停止服務並刪除程式（保留資料和`.env`） |
| `--purge` | 對於`--uninstall`，也要刪除資料目錄和`.env` |

> 官方版本僅提供 `amd64` / `arm64` 二進位。其他架構需要`--source`，
> 否則下載返回 404。

相關環境變數：

| 變數 | 目的 |
|----------|---------|
| `CF_GO_VERSION` | 覆蓋來源建置所需的 Go 版本 |
| `CF_NODE_MAJOR` | 覆蓋 Node 主要版本（預設 22） |
| `CF_GO_MIRROR` | Go工具鏈下載鏡像 |
| `CF_SKIP_DEPS` | 在代理安裝腳本中設定為 `1` 以跳過相依性安裝 |
| `CF_LANG` | 覆蓋安裝程式輸出語言（與 `--lang` 相同） |

### 2.6 目錄佈局

```text
/opt/corefusion/
├── corefusion                 # panel binary
├── corefusion.bak             # previous version (when upgrading)
├── .env                       # environment file (0600, generated by the setup wizard)
├── .env.secret                # encryption key file (0600, generated on first start)
├── .toolchain/                # Go / Node downloaded for source builds
└── data/                      # data directory (SAFE_PATHS)
    ├── sqlite/app.db          # default SQLite database
    ├── certs/                 # certificates and keys
    ├── logs/                  # runtime logs (without systemd)
    ├── runtime/               # core runtime config and state
    ├── binaries/              # downloaded xray / sing-box cores
    └── proxy/                 # proxy (Caddy/Nginx) sites and config
```

> 安裝程式僅創建`sqlite/`、`certs/`、`logs/`和`runtime/`；`binaries/`、`proxy/`、`rules/`等由面板或安裝核心時創建。
> 資料目錄來自`.env`中的`SAFE_PATHS`，可能是絕對路徑。更改安裝目錄不會移動現有資料。

### 2.7 服務管理

systemd（多數發行版）：

```bash
systemctl status corefusion --no-pager
systemctl restart corefusion
systemctl stop corefusion
journalctl -u corefusion -f          # follow logs
journalctl -u corefusion -n 100      # last 100 lines
```

不含systemd（貨櫃等）：

```bash
cd /opt/corefusion && ./corefusion            # foreground (debugging)
cd /opt/corefusion && setsid nohup ./corefusion >> data/logs/panel.log 2>&1 &
tail -f data/logs/panel.log
```

> 如果沒有 systemd，則該進程不會在啟動時啟動；重新啟動後手動重新啟動它（或重新執行安裝程式）。

### 2.8 升級與卸載

升級（保留資料和配置）：

```bash
sudo bash install.sh --version <new-version>
```

腳本停止服務，將舊二進位檔案備份為 `corefusion.bak`，取代它並重新啟動。現有的 `.env` 和 `data/` 保持不變。

卸載：

```bash
# Remove the program only; keep data and .env
sudo bash install.sh --uninstall

# Also remove the data directory and .env (irreversible)
sudo bash install.sh --uninstall --purge
```

#### 面板線上升級（建議）

「系統設定 → 面板更新」（僅超級管理員可見）：

1. 頂部顯示**目前版本**與**最新版本**；
2. 在版本下拉選單選擇目標版本（標 `pre` 的是預發布版，例如 `v0.13.0-rc.1`；
   目前架構沒有二進位檔的版本會反灰並提示「無資產」）；
3. 點「更新」：面板自行下載 → 校驗 SHA-256 → 替換二進位檔 → 重新啟動；
   頁面會短暫中斷，約 10~30 秒後重新整理即可；
4. 下方「歷史」列出每次升級/回滾的狀態與錯誤訊息；
5. 「回滾」可回到上一次升級前自動備份的版本（按鈕旁會提示備份版本與時間）。

注意事項：

- 面板本機沒有對應架構的二進位檔時會從 GitHub Releases 下載；純內網環境請改用
  `install.sh --version <版本>` 離線升級；
- 只替換程式，`data/`、`.env`、資料庫不會變動，但升級前仍建議在
  「系統設定 → 備份還原」做一次備份；
- 預發布版本（tag 帶 `-rc`）用於灰度驗證，建議先在單台面板觀察再全量。

### 2.9 環境文件（.env）

儲存設定時，完整的 `.env` 由設定精靈產生（資料庫 DSN、資料目錄、機密、連接埠、網域）。

在全新安裝時，安裝程式僅寫入此佔位符，因此面板不會將現有的 `data/` 誤認為是初始化安裝並跳過精靈：

```env
SAFE_PATHS=data
```

> 當完整配置（`.env` 和 `CF_DB_DSN`）已存在時，安裝程式絕不會覆寫它，且精靈也不會再次出現。
> 稍後 `.env` 的手動編輯將在重新啟動後生效。

#### 數據目錄

```env
SAFE_PATHS=data                       # relative to the install directory
# SAFE_PATHS=/srv/corefusion-data     # or an absolute path
```

#### 資料庫

`CF_DB_DSN`透過前綴檢測：

| 前綴 | 資料庫 |
|--------|----------|
| 未設定或裸路徑 | SQLite（預設） |
| `sqlite://` | SQLite（顯） |
| `mysql://` | MySQL |
| `postgres://` / `postgresql://` | PostgreSQL |

`CF_DB_TYPE` 是可選的，並覆蓋檢測 (`sqlite` / `mysql` / `postgres`)。

> **加密密碼**：面板寫入的MySQL/PostgreSQL DSN（設定精靈/資料庫開關）
> 儲存在 `.env` 中為 `enc:...`（AES-256-GCM，金鑰 `CF_SECRET_ENCRYPTION_KEY`）。
> 它們在讀取時被解密；手寫的純文字 DSN 仍然有效。

**SQLite（預設）**

```env
# Bare path (detected as SQLite)
CF_DB_DSN=data/sqlite/app.db

# Or explicit sqlite:// prefix
CF_DB_DSN=sqlite:///opt/corefusion/data/sqlite/app.db
```

**MySQL**

```env
# Format: mysql://user:password@tcp(host:port)/dbname?params
CF_DB_DSN=mysql://corefusion:password@tcp(127.0.0.1:3306)/corefusion?charset=utf8mb4&parseTime=True&loc=Local
```

**PostgreSQL**

```env
# Format: postgres://user:password@host:port/dbname?params
CF_DB_DSN=postgres://corefusion:password@127.0.0.1:5432/corefusion?sslmode=disable
```

#### 連接池（可選，MySQL/PostgreSQL）

```env
CF_DB_MAX_OPEN_CONNS=100
CF_DB_MAX_IDLE_CONNS=10
```

#### 其他常見變數

| 變數 | 描述 |
|----------|-------------|
| `JWT_SECRET` | 登入令牌的簽名金鑰；遺失時首次啟動時生成 |
| `CF_SECRET_ENCRYPTION_KEY` | 用於加密敏感設定的金鑰 (AES-256-GCM)；缺失時產生 |
| `CF_INITIAL_ADMIN_PASSWORD` | 初始管理員密碼（可選；隨機且預設列印一次） |
| `CF_NETWORK_TRAFFIC_ACCOUNTING` | 設定為 `0` 以停用基於 iptables 的流量計費（預設為啟用） |
| `CF_NETWORK_TRAFFIC_TABLE` | 用於流量統計的iptables表，預設`corefusion` |

> **加密金鑰位於 `.env.secret`**：首次啟動時，`CF_SECRET_ENCRYPTION_KEY` 被寫入 `.env.secret`（0600，與 `.env` 相同的目錄）而不是 `.env`；`.env` 中的舊金鑰會自動遷移。單獨備份此檔案 - 遺失它會使每個 `enc:` 值無法解密。

> 用於解析真實客戶端 IP 的可信任反向代理程式不再透過環境變數進行設定；
> 它們在**設定 → 網路**中儲存為 `panel.trusted_proxies`。

> `.env`和`.env.secret`都包含秘密；面板在啟動時將其權限收緊至 `0600`。永遠不要犯下這些錯誤。

### 2.10 Docker（可選，建造自己的鏡像）

該存儲庫**不**提供 Dockerfile。如果您將其容器化，請編寫您自己的映像。參考調用：

```bash
docker run -d \
  --name corefusion \
  -p 8080:8080 \
  -p 18443:18443 \
  -v /opt/corefusion/data:/app/data \
  -v /opt/corefusion/.env:/app/.env \
  <your-image>
```

> 容器通常沒有 systemd 和 `nft` 權限，因此速率限制/防火牆功能將無法運作。
> 工作目錄必須與相對 `SAFE_PATHS` 的基址相符。

### 2.11 安裝後檢查

```bash
# Environment and dependencies
sudo bash install.sh --cpu-info

# Service state
systemctl is-active corefusion

# Startup log (contains the setup URL and token)
journalctl -u corefusion -n 50 --no-pager

# Listening ports
ss -lntp | grep corefusion
```

---

## 3. 初始配置

首次啟動時，如果 `.env` 沒有有效的 `CF_DB_DSN`，則面板**不**為管理 UI 提供服務。它開始
**設定精靈**在 18000–18999 的隨機連接埠上使用一次性令牌，然後在儲存後作為正常面板重新啟動。

### 3.1 開啟設定嚮導

1. 從安裝程式輸出複製「設定精靈」URL；或
2. 讀取伺服器上的日誌：

```bash
journalctl -u corefusion -n 50 --no-pager | grep -A2 'setup wizard'
```

```text
CoreFusion first start - setup wizard
  URL   : http://<ip>:<port>/init?init_token=<token>
  Token : <token>
  Valid : 2h0m0s
```

3. 在瀏覽器中開啟URL（它已經包含`init_token`，因此不需要手動輸入）。

> 預設情況下，嚮導會話將在 **2 小時**後過期。如果它過期或面板重新啟動，請取得一個新的。
> 不要共享令牌。設定完成後，此連接埠將關閉，僅保留普通面板連接埠。

### 3.2 嚮導步驟

此嚮導有 8 個步驟。您可以隨時返回；在最終保存之前不會提交任何內容。

| 步驟 | 內容 |
|------|---------|
| 1. 資料庫 | SQLite / MySQL / PostgreSQL |
| 2. 數據目錄 | 寫入`.env`中的`SAFE_PATHS` |
| 3. 加密金鑰 | `JWT_SECRET` 和 `CF_SECRET_ENCRYPTION_KEY` |
| 4. 核心 | 要安裝哪些代理核心 |
| 5. 代理 | 無 / Nginx / Caddy |
| 6. 站點和網域 | 連接埠、Web 路徑、管理員帳戶、貨幣、時區、網域 |
| 7、HTTPS證書 | 已停用/上傳/ACME |
| 八、總結 | 查看，下載`.env`，儲存並重啟 |

#### 第一步：資料庫

- **SQLite（本地文件，建議）**：將路徑留空以使用`<data-dir>/sqlite/app.db`。
- **MySQL / PostgreSQL**：主機、連接埠、使用者、密碼、資料庫名稱；PostgreSQL 也支援 `disable` / `require` / `verify-full`。
- 按一下“測試連線”進行驗證。保存也可以測試它。缺少的資料庫會自動建立（如果帳戶缺少 `CREATE`，請手動建立）。
- DSN（包括密碼）在寫入 `.env` 之前經過加密。

#### 第二步：資料目錄

儲存資料庫、憑證、日誌、核心二進位等；這是`.env`中的`SAFE_PATHS`。預設為 `data`
（相對於安裝目錄）；允許使用絕對路徑。 **以後不要隨便改**，或是面板
將找不到現有的資料庫和憑證。

#### 第 3 步：加密金鑰

- `JWT_SECRET`：簽署登入令牌。
- `CF_SECRET_ENCRYPTION_KEY`：加密敏感設定（資料庫密碼、DNS憑證、儲值卡等）。
- 兩者預設都是隨機產生的；使用“重新生成”來獲取新值。 **更改 `CF_SECRET_ENCRYPTION_KEY` 會使先前加密的資料無法讀取** - 備份它。

#### 第 4 步：核心

選擇要安裝的代理核心：

- **Xray**：VLESS / VMess / Trojan / Shadowsocks 等；
- **sing-box**：承載Hysteria2 / TUIC等QUIC協定。

儲存並重新啟動後，面板將自動下載並安裝。檢查「協定」下的版本和狀態。您也可以稍後安裝它們。

#### 第5步：代理

- **無**：面板直接為客戶服務（TLS自行處理）。
- **Nginx / Caddy**：面板管理反向代理程式和憑證託管；選擇一個。

選擇代理後，主機通常僅偵聽 `127.0.0.1`，而代理則為外界提供服務。

#### 第 6 步：站點和網域

| 領域 | 描述 |
|-------|-------------|
| 收聽地址 | 自動衍生：代理→`127.0.0.1`；沒有附證書的代理程式 → `0.0.0.0`；否則 → `127.0.0.1` |
| 面板連接埠 | 預設隨機空閒連接埠；如果您願意，可以更改它（例如 8080） |
| 訂閱埠 | 選購; `0` 共用面板口 |
| 門戶連接埠 | 選配； `0` 共用面板口 |
| 網頁路徑 | 管理UI路徑，預設`/`；API 始終是 `/api/v1` |
| 管理者使用者名稱/密碼 | 預設隨機； **密碼在摘要中僅顯示一次** — 儲存 |
| 存取方式 | “網域名稱”或“IP” |
| 預設貨幣/時區 | 儲值卡和餘額的預設貨幣；面板和代理日誌時區（例如 `Asia/Shanghai`） |
| 新增到反向代理 | 設定好代理程式和網域後，一次將面板/訂閱/門戶網域新增至代理程式中 |

輸入域會寫入對應的設定（回呼域、訂閱端點、共用域）。

#### 第7步：HTTPS證書

| 來源 | 描述 |
|--------|-------------|
| 停用HTTPS | 面板/入口網站/訂閱透過 HTTP 提供服務（不安全）；使用反向代理，您可以稍後在代理端新增 TLS |
| 上傳證書 | 貼上憑證PEM和私鑰PEM（僅限網域存取模式） |
| ACME | 當您提交此步驟時頒發證書；支援網域名稱和IP證書 |

ACME選項：

- **CA**：Let's Encrypt、Let's Encrypt（暫存）、ZeroSSL、Google 公共 CA；
- **目標**：配置的網域，或公用IP（僅Let's Encrypt支援IP憑證）；
- **挑戰**：`HTTP-01`（需要連接埠 80）或 `DNS-01`（通配符，無連接埠 80；選擇 DNS 提供者並輸入憑證）；
- **金鑰類型/大小**：ECC 256/384 或 RSA 2048/3072/4096；
- **Google 公用 CA**：需要 EAB；手動輸入金鑰 ID / HMAC 或使用 GCP 服務帳戶。

> IP 憑證會自動偵測主機的公共 IPv4/IPv6。在代理後面，偵測到的位址是主機的位址，而不是代理出口的位址。
> 網域證書也可以稍後在「證書」下頒發。

#### 第 8 步：總結

顯示資料庫、資料目錄、核心、代理程式、連接埠、管理員帳戶、網域和 `.env` 金鑰（秘密封鎖），並提供：

- **下載.env**：保留配置的副本；
- **存取 URL**：面板/訂閱/入口網站的最終 URL；HTTP 被標記為不安全。

點選「儲存並重新啟動」寫入資料庫和`.env`；面板重新啟動並進入正常服務狀態。

### 3.3 設定後

- 服務重啟，啟動日誌列印最終監聽位址；
- 使用管理員帳號登入，從摘要中存取URL和網路路徑；
- 若密碼遺失，可使用`./corefusion admin reset-password`離線重設。

### 3.4 非互動式 CLI 設定

對於自動化或無頭伺服器，直接編寫配置並標記設定完成：

```bash
cd /opt/corefusion
./corefusion init \
  --db-type sqlite \
  --data-dir /opt/corefusion/data \
  --admin-username admin \
  --admin-password 'change-me-strong-password' \
  --listen-port 8080 \
  --proxy-type caddy \
  --cores xray,singbox \
  --panel-domain panel.example.com \
  --tls-mode acme --tls-domain panel.example.com --tls-email you@example.com
```

常見標誌：

| 旗幟 | 描述 |
|------|-------------|
| `--db-type` / `--db-host` / `--db-port` / `--db-user` / `--db-password` / `--db-name` | 資料庫設定 |
| `--data-dir` | 資料目錄；預設為 `SAFE_PATHS` 或 `./data` |
| `--listen-port` / `--subscription-port` / `--portal-port` / `--web-path` | 連接埠和Web路徑（預設隨機空閒連接埠） |
| `--admin-username` / `--admin-password` | 管理者帳號（省略時隨機列印一次密碼） |
| `--proxy-type` | `none` / `nginx` / `caddy` |
| `--cores` | 要安裝的核心，逗號分隔：`xray,singbox` |
| `--panel-domain` / `--subscription-domain` / `--portal-domain` | 域（寫入設定並新增到反向代理） |
| `--tls-mode` | `none` / `self_signed` / `acme` |
| `--tls-domain` / `--tls-email` / `--tls-challenge` / `--tls-key-type` / `--tls-key-bits` | 证书选项（`--tls-challenge` 默认为 `http-01`；`dns-01` 需要 `--tls-dns-channel` 或 `--tls-dns-provider`） |
| `--tls-ip-cert` | 為偵測到的公用IP核發憑證（僅限Let's Encrypt） |
| `--acme-provider` | `lets_encrypt` / `lets_encrypt_staging` / `zerossl` / `google` |
| `--currency` / `--timezone` | 預設貨幣/時區 |

完整列表：`./corefusion init --help`。然後啟動服務：

```bash
systemctl restart corefusion
```

### 3.5 重新運行設定嚮導

重新開始（刪除 `.env` 和設定標記，**保留資料**）：

```bash
cd /opt/corefusion
./corefusion reset-init      # type RESET to confirm
systemctl restart corefusion
```

然後使用啟動日誌中的新 URL 和令牌。

> 重新初始化會重設管理員帳戶和 Web 路徑。舊資料庫中的業務資料仍然存在，但您可能需要重新建立登入帳戶。
>
> `reset-init`删除整个`.env`（包括`SAFE_PATHS`）。 **如果您的資料目錄是自訂絕對路徑**，請在重新啟動之前恢復 `SAFE_PATHS`，否則面板將在 `./data` 中找到：
>
> ```bash
> printf 'SAFE_PATHS=/srv/corefusion-data\n' > /opt/corefusion/.env
> ```

### 3.6 首次登入後的安全性

#### 修改初始密碼

1. 點選右上角的使用者名稱；
2. 選擇“更改密碼”；
3. 輸入舊密碼和新密碼；
4. 點選“儲存”。

#### 啟用動態程式碼登入（2FA / TOTP）

> 啟用後，帳號切換為**動態密碼登入**：不再接受靜態密碼；只有
> 來自身份驗證器應用程式的 6 位元代碼（或一次性恢復代碼）有效。這**取代**密碼登入 - 它是
> 不是「密碼+驗證碼」的兩步驟驗證。

1. 點選右上角的使用者名稱；
2. 選擇“啟用2FA”；
3. 輸入目前密碼（僅用於確認此變更）；
4. 使用認證器應用程式（Google Authenticator / 1Password等）掃描二維碼，或手動輸入金鑰；
5. 輸入6位密碼完成啟用；
6. **立即儲存恢復代碼**（可下載）；每個代碼只能運行一次。

之後登入：

- 輸入使用者名稱後，密碼欄位變為“6位元代碼/恢復代碼”，按鈕變為“使用代碼登入”；
- 輸入目前代碼； **靜態密碼不能再使用**；
- 恢復代碼是一次性的；當它們用完時，在「動態代碼登入（2FA）」下重新產生它們（這會使所有舊代碼失效）。

如果驗證器遺失且恢復碼已用盡，請在伺服器上離線停用 2FA：

```bash
cd /opt/corefusion
./corefusion admin 2fa reset -u admin    # disables 2FA, clears the secret and recovery codes, revokes all sessions
```

> 既沒有身份驗證器也沒有恢復程式碼的唯一超級管理員無法透過 Web UI 進行恢復，必須使用上面的命令。

#### 註冊Passkey（可選）

1. 點選右上角的使用者名稱；
2. 選擇“Passkey管理”；
3. 點選「註冊新的Passkey」；
4. 完成設備提示。

> **注意**：金鑰需要 HTTPS 或 localhost，且該網域必須列入允許清單。

---

## 4. 儀表板

儀表板顯示多節點叢集概覽和節點運作狀況。

### 4.1 顯示內容

- **系統資訊**：主機名稱、作業系統版本、正常運作時間、CPU、記憶體
- **流量**：即時頻寬、總流量、每月趨勢
- **節點健康狀況**：線上/離線/降級計數，高負載節點
- **警報**：節點離線、節點降級、憑證即將過期、配額警告
- **叢集資源**：平均CPU、記憶體、磁碟和吞吐量

### 4.2 行動

- 點擊“查看詳情”，開啟節點詳情頁面
- 點選節點卡可查看其實時狀態
- 使用篩選器按狀態、類型或區域縮小範圍

---

## 5. 節點管理

節點管理是CoreFusion的核心，支援多種節點類型。

### 5.1 節點類型

| 類型 | 描述 |
|------|-------------|
| 獨立式（`standalone`） | 獨立核心節點 |
| 鏡子(`mirror`) | 從目標節點同步配置 |
| 主/備（`primary_backup`） | 相互故障轉移的合作夥伴節點 |
| 鏈條入口（`chain_entry`） | 鍊式代理的入口節點 |
| 鏈條出口（`chain_exit`） | 鍊式代理程式的退出節點 |

### 5.2 新增節點

1. 開啟“節點管理”；
2. 點選「新增伺服器」；
3. 填寫基本資料：
   - 名稱，例如`HongKong-HK01`
   - 主機名稱：IP 或域名
   - 區，如`HK`、`JP`、`US`
   - 節點類型；
4. 設定Agent埠（預設`18443`）；
5. 選擇支持的核心；
6. 點選“儲存”。

### 5.3 一鍵部署

1. 新增節點後，在節點清單中選擇「部署指令」（詳情頁面也提供
 “引導安裝命令”）；
2. 複製命令－它包含一次性註冊令牌（24小時內有效，請勿重複使用）；
3. 在遠端節點上以 root 身份運行它；它看起來像：

   ```bash
   bash <(curl -fsSL "http://<panel-host>:<panel-port>/api/v1/agent/install?id=<node-id>&token=<one-time-token>&port=<panel-port>")
   ```

4. 代理下載包，向面板註冊並啟動。

### 5.4 節點詳情

點擊節點可開啟其詳細資訊頁面：

- **概覽**：基本資料、代理狀態、憑證狀態；資源卡顯示CPU/記憶體/磁碟使用量、網路卡上離線速率、核心使用者流量速率、每月使用流量、TCP連線數、系統負載及出口網路卡累計流量（每10秒靜默刷新）
- **協議中心**：核心流程配置
- **代理**：Caddy / Nginx 配置
- **核心進程**：啟動/停止/重啟
- **核心流量**：核心轉送的使用者流量及其每月總量
- **WARP**：WARP 配置
- **直接域名**：域名管理
- **速率限制**：流量整形
- **憑證**：憑證綁定
- **線上用戶**：即時線上用戶

### 5.5 節點動作

- **同步配置**：將面板配置推送到節點
- **強制重新連接**：斷開代理連接並重新連接
- **更新 Agent（線上升級）**：節點詳情 →「Agent 更新」：
  - 面板本機有同架構 Agent 二進位檔時，可直接「從面板更新」（內網/離線環境）；
  - 否則從 GitHub Releases 選擇版本更新（營運環境通常走這條）；
  - 升級時 Agent 程序會短暫重啟並自動重連，已在執行的節點核心/代理不會被一併回收；
  - 從舊版本升級上來的節點，Agent 首次連上面板會把面板更新簽章公鑰寫入
    `config.yaml` 的 `update.public_key`（權限 `0600`，已有值不覆蓋），之後自更新
    必須帶 ed25519 簽章，未簽章或簽章不符會被拒絕。
- **撤銷憑證**：撤銷節點的Agent憑證

---

## 6. 禮賓中心

協定中心為核心進程配置協定參數。

### 6.1 支援的內核

| 核心 | 描述 |
|------|-------------|
| Xray-核心 | VLESS / VMess / Trojan / Shadowsocks 等 |
| sing-box | 支援 TUN 的模組化通用代理平台；搭載Hysteria2 / TUIC等QUIC協議 |

### 6.2 配置模組

兩個內核之間的模組不同；此面板顯示與所選核心相符的集：

**Xray-核心**

- **日誌**：訪問日誌、錯誤日誌、輸出方式
- **API**：gRPC API監聽位址，服務列表
- **DNS**: DNS servers, query strategy, static hosts
- **路由**：路由規則、負載平衡、VLESS路由
- **入站/出站**：協定、傳輸、TLS
- **傳輸**：WebSocket、gRPC、XHTTP 等（TUN 是入站協議，而不是傳輸）
- **TLS**：憑證、ALPN、uTLS 指紋、ECH
- **Reality**：目標管理、金鑰對、短ID
- **健康檢查/FakeDNS/指標/策略/地理資料/版本限制**：探針、IP池、Prometheus、逾時和緩衝區、資源檔案、核心版本範圍

**sing-box**

- **日誌**：等級、時間戳記、輸出
- **NTP**：時間同步
- **DNS**：DNS 伺服器與規則
- **路由**：`final`預設出站、**規則集**、**規則**、預設域解析器、網路策略和回退（請參閱6.6）
- **入站/出站**：協定、傳輸、TLS
- **端點**：WireGuard（包括WARP配置）、MASQUE客戶端等；幾種可以共存並以`tag`區分

> 啟用「使用系統網路整合」後，端點的**名稱**是核心介面名稱：
> 最多 15 個字符，僅限字母/數字/`._-`。太長或無效的名稱會自動回退
> 到端點 `tag`。新端點依協定加入前綴（`wg-` / `ts-` / `oc-` / `ovpn-c-` /
> `ovpn-s-` / `mq-c-` / `mq-s-`），以及舊記錄（例如 `WireGuard-xxxxxx`）顯示標準化
> 編輯器一打開。
- **Services**: the top-level sing-box `services` array, such as `hysteria-realm`, DERP or `resolved`
- **證書/HTTP 用戶端/網路命名空間**：僅在需要時展開的 JSON 部分
- **實驗**：`clash_api` 及類似； **除非啟用，否則不會將任何內容寫入產生的配置**

> sing-box 1.12 刪除了 `geosite` / `geoip` 路由欄位；現在可以進行基於地理的路由
> `rule_set`。保留舊欄位會使核心無法解碼配置（`unknown field`）。

### 6.3 入站配置

1. 選擇核心和協議；
2. 填寫基本資料：
   - 標籤
   - 監聽位址和連接埠
   - 協定類型
3. 設定協議參數（UUID、密碼等）；
4. 選擇傳輸方式（TCP、WebSocket、gRPC 等）；
5. 配置TLS（證書、ALPN）；
6. 保存並應用到核心。

> 新的入站預設偵聽 `::`（Linux 上的雙堆疊，同時處理 IPv4 和 IPv6）。
> 僅當需要限制介面時才將其變更為`0.0.0.0`或特定位址。

「使用入站連接埠」連接埠模式也提供**自動建立防火牆允許規則**：開啟
入站編輯器首先探測節點防火牆，偵測到firewalld/ufw/nftables/iptables時
您可以根據監聽協定（TCP/UDP）允許連接埠儲存。

- hy2 / hysteria 連接埠跳變允許偵聽連接埠和每個動態連接埠範圍（例如 `20000-30000`）；多個範圍以逗號分隔；
- 允許是冪等的：不會再次建立現有的覆蓋規則（連接埠範圍/多連接埠）；
- 更改連接埠會在允許新規則之前撤銷舊規則；刪除入站、停用連接埠跳躍或切換到 Unix Socket/tun（無監聽連接埠）可將其清除；
- 當未偵測到啟用的防火牆時，僅顯示提示且不寫入規則。

### 6.3.1 反向代理路徑（訪問域→前置TLS→反向代理路徑）

當入站自身傳輸路徑為 `/` 但外部 nginx / caddy 將其掛載在子路徑下時
（例如 `location /apps { proxy_pass http://127.0.0.1:8080/; }`），客戶端必須連接到 `/apps` 或
請求永遠不會到達該位置 - 訂閱仍然會下載，但節點不會連接。
在「訪問域→前端TLS」下填入**反向代理路徑**：

- 輸入`/apps`或`/apps?ed=2560`（有早期資料）；訂閱和共享連結**用它取代**入站傳輸路徑；
- 適用於 ws、httpupgrade、xhttp 和 h2/http；沒有路徑的傳輸（raw/tcp、kcp）將被忽略；
- 它與 TLS 終止位置無關：當 Front TLS 關閉時，路徑仍然適用（跟隨核心）；
- gRPC 沒有 URL 前綴概念（客戶端路徑始終為 `/{serviceName}/…`），因此該路徑 **合併到 serviceName**
（`/apps` + `svc` → `apps/svc`）；反向代理必須移除 `grpc_pass` 中的該前綴。

### 6.4 出站配置

1. 添加出站；
2. 設定標籤和協議；
3. 設定伺服器位址和連接埠；
4. 配置传输和TLS；
5. 保存並應用。

### 6.5 分享鏈接

1. 在入站清單中點選「分享連結」；
2. 選擇格式：
   - 分享 URI
   - Mihomo YAML
   - sing-box JSON
   - Xray JSON
3. 複製連結或產生二維碼。

### 6.6 sing-box 路由和规则集

在「協定中心」→sing-box→「路由」中有兩個部分： **規則集**聲明可重複使用
規則文件，**規則**決定符合條件和出站。規則的評估來自
從上到下，第一場比賽獲勝。

#### 6.6.1 規則集

| 領域 | 描述 |
|-------|-------------|
| 標籤 | 規則引用的規則集標識符；必須是唯一的 |
| 類型 | `local`讀取節點上的檔案；`remote`讓核心在`update_interval`上下載它 |
| 格式 | `binary` 為 `.srs`（預設）； `source` 用於來源格式 `.json` |
| 路徑 | `local`路徑是相對於節點資料目錄的；`data/rules/`中的檔案使用`rules/xxx.srs` |
| URL | `remote`的下載位址；`download_detour` 選擇用於下載的出站（通常為 `direct`） |

本機規則集（建議；先在「服務資源→規則檔」下下載檔案）：

```jsonc
"route": {
  "rule_set": [
    { "type": "local", "tag": "geosite-cn", "format": "binary", "path": "rules/geosite-cn.srs" },
    { "type": "local", "tag": "geoip-cn", "format": "binary", "path": "rules/geoip-cn.srs" }
  ]
}
```

> 在面板上下載檔案是不夠的：必須先將 `local` 規則集推送到
> 節點（「推送到所有節點」），因此檔案存在於節點的 `data/rules/` 中；否則核心
> 無法啟動，因為找不到該檔案。

遠端規則集（節點必須能夠到達URL）：

```jsonc
{ "type": "remote", "tag": "geosite-openai", "format": "binary",
  "url": "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-openai.srs",
  "download_detour": "direct", "update_interval": "1d" }
```

當您選擇本機檔案時，標籤是從檔案名稱衍生的（`rules/geosite-cn.srs` →
`geosite-cn`）；對於遠端裝置，它來自 URL 的末端。格式也是填寫自
分機（`.srs` → `binary`、`.json` → `source`）。導出的值可以自由編輯，並且
您已經輸入的值永遠不會被覆蓋。

> `download_detour`自sing-box 1.14起已棄用，將在1.16中移除；它仍然
> 適用於 1.15。新配置應使用頂級 `http_clients`
> 改為 `route.default_http_client`。

#### 6.6.2 規則

| 領域 | 火柴 |
|-------|---------|
| `domain` | 精確域 |
| `domain_suffix` | 域名後綴，包括子域名 |
| `domain_keyword` | 子字串匹配 |
| `domain_regex` | 正規表示式 |
| `ip_cidr` / `ip_is_private` | 目的IP |
| `source_ip_cidr` | 來源IP |
| `port` / `port_range` | 目的港 |
| `network` | `tcp` / `udp` |
| `rule_set` | 要使用的規則集的標籤 |
| `outbound` | 要使用的出站標籤（必須存在於「出站」或「端點」下） |
| `type: "logical"` | `mode` 設定為 `and` / `or` 的組合條件 |

一條規則中的多個欄位透過 AND 組合（例如 `network` + `port`），因此不會
需要 `logical` 包裝紙。 `type: "logical"` 僅用於 OR / NOT 組合。 `rule_set`
必須放置在 `rules` 子規則內，而不是放在 `rules` 旁邊：

```jsonc
"rules": [
  { "domain_suffix": ["gstatic.com", "googleapis.com"], "outbound": "direct" },
  { "domain_keyword": ["ip.sb", "yandex.com"], "outbound": "warp" },
  { "rule_set": ["geosite-openai", "geosite-google"], "outbound": "warp" },
  { "rule_set": ["geoip-cn"], "outbound": "warp" },
  { "network": "tcp", "port": 443,
    "rule_set": ["geosite-google"], "outbound": "warp" },
  { "type": "logical", "mode": "or",
    "rules": [
      { "rule_set": ["geosite-google"] },
      { "rule_set": ["geosite-openai"] }
    ],
    "outbound": "warp" }
]
```

#### 6.6.3 遷移xray規則

| xray | sing-box |
|------|----------|
| 裸字串（包括`domain`中的裸值；子字串匹配） | `"domain_keyword": ["ip.sb"]` |
| `"domain": ["a.com"]` | `"domain_keyword": ["a.com"]`；使用 `"domain_suffix": ["a.com"]` 進行真正的後綴匹配或使用 `"domain": ["a.com"]` 進行精確匹配 |
| `"domain:clstorage.net"` | `"domain_suffix": ["clstorage.net"]` |
| `"full:a.com"` | `"domain": ["a.com"]` |
| `"regexp:..."` | `"domain_regex": ["..."]` |
| `"ip": ["geoip:cn"]` | `"rule_set": ["geoip-cn"]` |
| `"domain": ["geosite:cn"]` | `"rule_set": ["geosite-cn"]` |
| `"outboundTag": "direct"` | `"outbound": "direct"` |

注意事項：

- **標籤不在核心之間共用**：xray 出站標籤不能在 sing-box 規則中使用；
 使用sing-box「Outbounds」或「Endpoints」下真實存在的標籤。
- 錯誤的出站或端點標籤報告`outbound not found`；錯誤的規則集標籤報告
 `rule-set not found`。
- 將直接/允許清單規則放在首位；一旦規則匹配，後面的規則就不再適用。

---

## 7. 代理管理

代理管理配置Caddy或Nginx反向代理。

### 7.1 代理類型

- **Caddy**：自動 HTTPS、HTTP/1.1 與 HTTP/2（停用 HTTP/3 / QUIC 以避免 UDP 443 與第 4 層路由衝突）
- **Nginx**：高效能，配置靈活

### 7.2 站點配置

1. 開啟“代理管理”；
2. 選擇代理類型（Caddy / Nginx）；
3. 新增站點：
   - 網域
   - 上游
   - 本地埠
4. 保存。

### 7.3 字段

- **上游**：後端位址，例如`127.0.0.1:8082`
- **local_port**：本地監聽端口，例如`8443`
- **portal_forward**：門戶轉送規則
- **subscription_forward**：訂閱轉送規則
- **SNI 路由**：當入站使用 TLS / REALITY 並且客戶端始終在 443 上連接時，面板派生
 443 TCP來自ClientHello SNI的路由規則（xray `streamSettings`和sing-box `tls.reality`）；
 前端代理僅轉發TCP，TLS仍在核心內部終止

---

## 8. 路由資源

路由資源收集訂閱和路由使用的共享資產。該頁面有五個選項卡：

| 選項卡 | 目的 |
|-----|---------|
| Direct 和 CF 首選網域 | 入口直接域/IP和Cloudflare首選域，在重寫訂閱和外部節點中的存取位址時使用 |
| CF IP池 | 「訪問域」使用的首選 IP 來源、連接埠和探測設置 |
| WAF 全局规则 | 反向代理WAF的規則來源和規則文件，推送到節點 |
| 規則文件 | 規則檔案的下載、定時同步與推播（`.srs` / `.json` / `.dat`） |
| 外部節點 | 從v2ray URI / Base64 /訂閱URL匯入外部節點（見16.3） |

### 8.1 Direct 和 CF 首選域

- **直接存取**：入口網域名稱或IP，連接埠預設443；綁定節點或留空全域；
- **CF首選域**：Cloudflare首選域加端口，帶連通性檢查；
- 兩者都提供訂閱和外部節點重寫所使用的「存取域」候選者（請參閱 16.3）。

### 8.2 CF IP池

- 維護首選IP來源（包括`bestproxy`樣式來源）、取得間隔和連接埠；
- 「探測設定」控制探測間隔、延遲閾值和探測入口節點；本地探測
 只過濾明顯不可達的IP，同時選擇入口節點模擬該節點的線路；
- 「撥號器」可以是自動的（已安裝核心）、`singbox`、`xray` 或 `mihomo`；不屬於的核心
 已安裝的未列出。探測作為臨時進程運行，完成後退出並且從不讀取或
 編寫核心自己的配置；
- 當「存取域」使用CF IP池而沒有特定值時，一個條目（及其連接埠）是
 隨機挑選的。

### 8.3 WAF 全域規則

- 設定WAF規則來源URL和要同步的規則檔；
- 同步後，它們被推送到節點並由節點代理程式載入（Nginx / Caddy）；
- 從該 URL 下載具有完整 URL 的檔案；否則使用「來源 URL + /檔案名稱」；
- 「匹配快取」（`match_cache` / `config_match_cache`）預設為啟用：進程範圍內有界
 熱規則匹配結果的快取減少了每個請求的規則遍歷。它是全球唯一的，沒有
 每個域覆蓋；在調查誤報時暫時停用 A/B 比較
 （停用意味著對每個請求進行完整的規則評估以及更高的 CPU 使用率）。

#### 同步週期與時區

- 支援簡化 cron：`@every 1h`、`0 * * * *`（每小時）、`0 */6 * * *`（每 6 小時整點）、
  `0 3 * * *`（每天 03:00）、`0 0 * * *`（每天 00:00）、`0 0 * * 0`（每週）；
- **時區**跟隨「系統設定 → 面板 → 時區」（`panel.timezone`，預設 `Asia/Shanghai`）；
  未設定或名稱非法時回落到伺服器時區；
- **重啟不重置**：上次同步時間保存在 `waf.github_source.last_sync`，面板重啟後按
  「上次同步 + 間隔」繼續；`@every` 同樣以上次同步為基準；
- **錯過補跑**：上次同步已超過一個週期（停機或頻繁重啟）時，啟動後約 15 秒先補一次；
- 調度結果可在日誌確認：
  `[WAFScheduler] next sync at 2026-11-20 18:34:12 CST+0800 (... tz=Asia/Shanghai)`；
- 手動「立即同步」不受週期限制，可隨時拉取。

> 舊版本（≤ `v0.13.0-beta.100`）的計時器是「程序啟動 + 間隔」，面板頻繁重啟會導致
> 24 小時同步永遠不執行；升級到 `v0.13.0-rc.1` 及以上即修復。

#### 面板管理介面白名單

面板轉發網域會自動維護一份 URL 白名單（`whiteurl.rule`）：只跳過**管理端介面**的
URL / 參數 / POST 內容檢查，IP、UA、Cookie、CC、檔案上傳仍照常生效。
這樣儲存儲值站點（位址可能是 `http://127.0.0.1:18081` 等內網位址）、客戶備註、
設定文字時不會被 SSRF / SQL 注入規則誤攔。

系統區塊以「# 面板管理接口」開頭，面板啟動/升級時會冪等刷新；自訂規則請寫在該區塊之外。

### 8.4 規則檔（路由規則檔）

此選項卡僅下載面板上的規則檔案並推送到節點；沒有任何效果
自動。若要在路由中使用它們，請在「協定中心→sing-box→路由」下引用它們
規則集和規則（見 6.6）。

- **規則儲存庫**：輸入`owner/repo`（GitHub，可瀏覽）或完整下載URL；預定的
 同步可以每 6 / 12 小時運行一次，每天 03:00、每天午夜或每週日運行；
- **新增檔案**：對於儲存庫版本以外的單一檔案（例如 SagerNet `.srs` 分支
 文件）；輸入完整的URL，它會自動下載；
- **檔案清單**：顯示名稱、大小、上次更新和「自動同步」開關，以及每個檔案的「下載」；
 頂部的“推送到所有節點”按鈕將每個規則檔案推送到所有節點；
- 檔案被下載到面板的 `data/rules/` 中，一旦推送，就存在於節點資料下
 目錄的`rules/`；在 sing-box 中將它們引用為 `rules/<file name>`；
- 格式：
  - `.srs`：sing-box二進位規則集，建議且最小；
  - `.json`：來源格式規則集；`rule_set`必須設定`format: "source"`；
  - `.dat`：xray `geosite.dat` / `geoip.dat`，其中sing-box不能直接使用。

> 頁面頂部的可折疊「用法」區塊包含參考範例，xray 映射
> 表和常見錯誤。

### 8.5 外部節點

[16.3 外部節點](#163-external-nodes) 中介紹了匯入和管理外部節點。

---

## 9. 訂閱中心

訂閱中心將節點打包成客戶端就緒的訂閱：定義訂閱**格式**（發送什麼，哪些規則，哪些計劃可以看到它們），然後訂閱**端點**（公共域/路徑），然後分發連結。

### 9.1 訂閱格式

「訂閱中心」→「訂閱格式」；每個客戶端輸出一行：

| 領域 | 描述 |
|---|---|
| 名稱 | 獨特； `?config=<name>` 與其匹配並且客戶對話框顯示它 |
| 客戶類型 | clash / shadowrocket / singbox / v2ray — 僅顯示分組 |
| 輸出格式 | `mihomo_yaml` (Clash/Mihomo)、`singbox_json`、`xray_json`、`base64_uri`、`raw_uri` |
| 內容類型/擴展名 | 回應頭和檔案名 |
| 平台 | win/mac/android/ios/通用；與`?platform=`匹配，空=通用 |
| 綁定計劃 | 空=每個計劃都可用；否則只有那些計劃可以使用它 |
| 規則 | 每個格式的路由規則（Mihomo 的結構化編輯器，sing-box / Xray 每個規則的原始 JSON）。 **規則不會在格式之間轉換** |

注意事項：

- 相同的輸出格式可能存在多個配置（例如 `out`、`home`）；選擇連結中帶有 `config=` 的一個；
- 不帶`config`，順序為「訂閱綁定配置→`platform`匹配→預設」；
- sing-box 範本可以為客戶端攜帶自己的 `route.rule_set`，通常為 `remote`，因此
 客戶端下載它；推送到節點`rules/`「規則檔案」下的本機檔案僅可用
 到該節點上的核心，而不是客戶端訂閱；
- 產生 sing-box / Xray 範本時，面板會刪除懸空引用（`final`、`dns.final`、
 `default_domain_resolver`、`download_detour`、本地 `rule_set`、無效的 `rule_set` 定義、空
 `balancer` 等），因此客戶端在啟動時不會因致命錯誤而失敗；
- 規則中的出站別名根據實際標籤解析（`REJECT` / `blocked` → `block`、`DIRECT` →
 `direct`，不區分大小寫）；無法解析的出站回退到預設出站和規則
 保持其立場；
- 始終產生包含 `tag = proxy` 的群組：sing-box 的 `selector` 和 `random` 的平衡器
 規則中的Xray、`outbound: proxy`被改寫為符合的`balancerTag`。規則編輯器載入
 從「代理群組+自訂出站」動態篩選出站候選項，並手動輸入標籤
 仍在驗證中；
- sing-box / Xray 配置必須單獨建立；如果它們不存在，則不會出現在客戶連結中（請參閱 10.3）。

### 9.2 訂閱端點

「訂閱中心」→「訂閱端點」；每個公用位址一行：

| 領域 | 描述 |
|---|---|
| 網域 | 公開訂閱域 |
| 路徑 | 域下前綴，預設`/` |
| 啟用 | 建立連結時會忽略停用的端點 |
| 描述 | 自由文本註釋 |

- **支援多行（多域）**；啟用多個時，提供者網域遵循目前請求使用的網域（請參閱 9.4）；
- 專用訂閱埠在根服務（`/<token>`，無`/sub`）；共用面板口時為`/sub/<token>`；
- 反向代理必須轉送 `X-Forwarded-Proto` / `X-Forwarded-Host`，否則將使用第一個啟用的端點。此面板產生的 Caddy / Nginx 站點範本包括兩個標題；手動維護的代理配置需要手動新增。

### 9.3 訂閱鏈接

底座形式：`https://<sub-domain>/<token>`（面板連接埠：`https://<panel-domain>/sub/<token>`）

| 參數 | 描述 |
|---|---|
| `format` | 輸出格式；別名：`clash`→mihomo_yaml、`singbox`/`sing-box`→singbox_json、`xray`→xray_json、`raw`→raw_uri、`base64`/Z/ZV |
| `config` | 按名稱套用訂閱格式；必須符合`format`，否則忽略 |
| `platform` | 依平台匹配配置（當省略 `config` 時） |
| `providers` | `1` / `true`：Clash 提供者模式（`proxy-providers` + `use`）；預設內聯節點 |

範例：

```bash
# Clash dynamic subscription (config template + providers)
https://sub.example.com/<token>?format=clash&config=out&providers=1

# sing-box / Xray (only meaningful when such a config exists)
https://sub.example.com/<token>?format=singbox_json&config=sing-box-home
https://sub.example.com/<token>?format=xray_json&config=xray-home

# generic clients / plain text
https://sub.example.com/<token>?format=base64_uri
https://sub.example.com/<token>?format=raw_uri
```

### 9.4 多域和Provider模式

對於 `providers=1`，每個 `proxy-providers` 條目都指向訂閱端點。域名選擇：

1. **此請求使用的網域**（`X-Forwarded-Host` 第一個值，連接埠剝離，否則 `Host`）如果它與 **啟用** 端點相符；
2. 否則，具有**最低 ID** 的已啟用端點；
3. 該路徑來自該端點：`https://<domain><path>/<token>?format=mihomo_proxies_yaml...`。

因此，更改網域不需要更改面板：透過新網域取得的用戶端會在提供者內取得新網域，並且啟用的舊網域不再是硬編碼的。如果用戶端快取了舊的提供者 URL，請更新訂閱（或清除提供者快取/重新啟動用戶端）。

附加說明：

- 共享格式（`config_json`）內的`proxy-providers`被忽略（避免跨計畫洩漏）；提供者來自計畫節點群組和外部來源；
- 內部和外部提供者快取路徑始終源自「內部提供者預設值 → 快取路徑範本」（預設 `./providers/{key}.yaml`）；外部來源無法自行設定；
- 不再發出舊的保留提供者 `subscription-nodes`：使用「所有節點」的群組擴展到特定提供程序，或當計劃沒有時，擴展到內聯節點。

---

## 10. 客戶管理

客戶管理處理最終使用者。

### 10.1 新增客戶

1. 開啟“客戶管理”；
2. 點擊“新增客戶”；
3. 填寫：
   - 電子郵件
   - 密碼
   - 計劃
   - 流量配額
   - 到期時間
4. 節省。

### 10.2 客戶行為

- **編輯**：更改客戶詳細信息
- **暫停**：暫停服務
- **恢復**：恢復暫停的客戶
- **重置流量**：重置已使用流量
- **重設密碼**：重設客戶密碼
- **續訂**：延長有效期限
- **升級/降級**：更改計劃
- **刪除**：刪除客戶

### 10.3 訂閱詳情

客戶管理→「訂閱」欄位→「詳情」。該對話方塊列出了客戶計劃可用的每種格式的連結：

- **Clash / Mihomo**：兩行 - 內聯節點和 `providers=1`（代理提供者），均攜帶 `config=<name>`；
- **Sing-box / Xray**：僅當該格式的配置存在並且對計劃可見時才顯示；連結帶有`config=<name>`；
- **Base64 URI / Raw URI**：永遠顯示（無 `config`）；
- 每行都有自己的複製按鈕；點擊一行會將下面的 QR 碼切換為該格式。

因此，只有 Clash 設定時，對話方塊顯示 Clash + Base64 + Raw；sing-box / Xray 連結僅在建立其設定後才會出現。

同一對話方塊也會顯示流量使用情況、訂單歷史記錄和交易記錄。

---

## 11. 計劃管理

計劃管理配置您銷售的計劃。

### 11.1 計劃模板

1. 開啟「計劃管理」→「計劃範本」；
2. 新增模板：
   - 模板名稱
   - 類別（標準/高級/企業/試用）
   - 自動續訂
   - 試用設定
3. 保存。

### 11.2 計劃變體

1. 在模板下方新增一個變體：
   - 變體名稱（例如每月、每年）
   - 價格（以美分為單位）
   - 持續時間（天）
   - 流量（GB、`0` = 無限制）
   - 並發限制
   - 速率限制（Mbps）
2. 保存。

### 11.3 流量插件

1. 打開“流量插件”；
2. 新增一個附加元件：
   - 名稱
   - 價格
   - 交通
   - 到期模式（延長天數/與計畫相同）
3. 保存。

### 11.4 優惠券

1. 打開「優惠券」；
2. 新增優惠券：
   - 程式碼
   - 類型（百分比/固定金額）
   - 折扣值
   - 使用限制
   - 有效期限
   - 適用計劃
3. 保存。

### 11.5 配額策略與自動重置

計劃模板與變體都可以設定「配額策略」，變體留空表示繼承模板：

| 策略 | 到期時 | 續訂時 |
|---|---|---|
| 續訂累加（預設） | 保留未用完額度 | 原配額 + 本次配額，已用流量不清零 |
| 到期清空 | 未用完額度作廢（清零） | 按本次計劃配額發放 |
| 續訂重置 | 保留 | 按本次計劃配額重置，已用流量清零 |

- 變體視窗另有「配額重置週期」：繼承 / 不重置 / 月付；
- 選「月付」後，面板每小時巡檢一次，到期把訂閱已用流量清零（同時同步節點計數），
  並推進下一次重置時間；
- 續訂是否疊加、到期是否作廢，完全由這裡的設定決定，不再寫死。

---

## 12. 儲值管理

儲值管理管理儲值站點、訂單和儲值卡。

> **支付整合**
>
> 此面板**不直接與支付機構整合**（支付寶、微信支付、Stripe、銀行等）。它只連接**中間支付平台**，中間支付平台又與相應的支付機構集成，處理簽約、收單和結算，並通過[charge回調API](api/charge-callback.md)返回支付結果。
>
> 中間支付平台**不屬於本項目；運營商必須自己開發（或使用自己的系統）**。部署完成後，在「儲值站點」下註冊並完成網域驗證，即可作為儲值入口點；這裡的儲值站點是中間平台，而不是支付機構本身。支付資格、費用、結算及合規性由業者、支付機構、中介平台約定並承擔。

### 12.1 儲值站點

1. 開啟「儲值管理」→「儲值站點」；
2. 新增站點：
   - 網站名稱
   - 此中間平台支援的支付方式（支付寶、微信、USDT、Stripe等，透過與支付機構整合提供）
   - 貨幣
   - 匯率
3. 保存；
4. 驗證站點（HTTP 檔案或 DNS 驗證）。
5. 列表中的「停用/啟用」只切換站點狀態，不會改動站點位址、API Key、支付方式或回調金鑰；
6. 站點位址可以是內網/本機位址（如 `http://127.0.0.1:18081`）；面板轉發網域的 WAF
   白名單已放行管理端儲存操作，不會再被 SSRF 規則誤攔。

### 12.2 儲值訂單

- 查看所有儲值訂單
- 按狀態過濾（待付款、已付款、失敗、過期、退款）
- 按類型過濾：儲值 / 計劃（含流量包購買），可與狀態過濾組合
- 匯出 CSV 跟隨目前過濾條件，表頭跟隨介面語言
- 查看訂單詳情
- 發出管理退款

### 12.3 儲值卡

1. 打開「儲值卡」；
2. 新增卡：
   - 類型（金額、方案、流量附加）
   - 面額/計劃
   - 到期時間
3. 或批量生成：
   - 數量（最多 1000 個）
   - 卡號前綴
   - 卡號格式
4. 出口為 CSV。

### 12.4 餘額管理

- 查看客戶餘額
- 調整餘額（增加/減少/設定）
- 調整凍結金額
- 看交易歷史

### 12.5 經銷商

- 新增經銷商
- 設定佣金率
- 查看累計佣金
- 提取佣金

---

## 13. 票務管理

票證管理處理客戶支援請求。

### 13.1 門票清單

- 查看所有門票
- 依狀態過濾（開啟、回覆、關閉）
- 按類別篩選（技術、計費、帳戶、其他）
- 按優先順序過濾（低、正常、高、緊急）

### 13.2 工單操作

- **查看/回覆**：開票並回复
- **分配**：將票證分配給管理員
- **關閉**：關閉已解決的工單

---

## 14. 證書管理

證書管理處理TLS證書。

### 14.1 頒發證書

1. 開啟“證書管理”；
2. 點選「頒發證書」；
3. 填寫：
   - 域列表（每行一個）
   - 證書類型（網域名稱/IP）
   - 金鑰類型（ECC/RSA）
   - 驗證方法（DNS-01 / HTTP-01）
   - DNS供應商
   - 發行地點（面板主機/節點）
4. 按一下“問題”。

### 14.2 證書操作

- **續訂**：手動續訂
- **部署**：部署到節點
- **分發**：推送到所有關聯節點
- **遷移**：將引用從舊證書移至新證書
- **下载**：下载`cert` / `fullchain` / `privkey`

### 14.3 上傳證書

1. 點選「上傳憑證」；
2. 選擇方法：
   - 貼上內容
   - 上傳文件
3. 上傳憑證鍊和私鑰；
4. 節省。

### 14.4 EAB配置

- 管理 Google 公共 CA / ZeroSSL 的 EAB 凭证
- 自動取得EAB
- 發卡時自動附上

---

## 15. 流量配額

流量配額監控每個節點每月的流量。

### 15.1 配額清單

- 查看所有節點的配額設置
- 顯示配額大小、已使用流量和剩餘配額
- 顯示使用百分比

### 15.2 循環使用

- 查看每個節點的周期使用詳細信息
- 顯示入站/出站流量
- 顯示循環開始/結束時間

### 15.3 重置日誌

- 查看流量重置記錄
- 顯示重設週期、觸發來源和運算符

---

## 16. 鍊式代理

鍊式代理配置多出口連結。

### 16.1 建立連結

1. 打開“連結代理”；
2. 點擊「新連結」；
3. 選擇入口節點；
4. 選擇入站條目；
5. 選擇退出節點（允許多個）；
6. 為每個出口選擇一個入站；
7. 設定優先權/權重；
8. 確認。

### 16.2 連結管理

- 查看拓撲概覽
- 編輯連結（優先順序、重量、出站標籤）
- 刪除連結
- 檢查健康狀況

### 16.3 外部節點

- 匯入外部節點（v2ray URI、Base64 URI、訂閱URL）
- 註冊外部條目
- 管理外部節點（單獨編輯/刪除）
- 存取域：設定CDN加速/每節點首選IP（直接域，CF首選
 域、CF IP 池、自訂）。僅重寫訂閱中傳遞的伺服器/連接埠；
 SNI/主机保持原装进口。停用時，使用導入的位址/端口
- 當選擇“CF 首選域/CF IP 池”且未指定特定值時，將選擇一個條目
 從池中隨機（及其端口）； “direct / custom”需要明確的值，否則
 它回落到節點自己的位址

---

## 17. 線上用戶

線上用戶顯示即時線上狀態。

### 17.1 它顯示了什麼

- 線上訂閱
- 線上設備/IP
- 每個訂閱的節點、IP 和連線時間
- 即時上行/下行速率
- 總上行鏈路/總下行鏈路/總流量

> 總上行鏈路、總下行鏈路和總流量在**連續線上會話**中累積：
> 短暫的重新連接、核心重新啟動和計數器重置不會清除它們。限60歲以後
> 沒有新報告的秒數是否會從零開始新會話。這是一個不同的措施
> 來自核心自己的“自此開始以來的流量”計數器，更好地反映了用戶的實際情況
> 在目前會話期間使用。

### 17.2 過濾

- 按節點過濾
- 按電子郵件/客戶/節點/IP搜尋

---

## 18. 使用者管理

使用者管理處理面板使用者。

### 18.1 角色

| 角色 | 描述 |
|------|-------------|
| 超级管理员（`superAdmin`） | 最高特權 |
| 管理員 (`admin`) | 管理權限 |
| 操作員（`operator`） | 操作權限 |
| 檢視器 (`viewer`) | 只讀 |

### 18.2 新增用戶

1. 開啟“用戶管理”；
2. 點選「新增用戶」；
3. 填寫：
   - 使用者名稱
   - 電子郵件
   - 密碼
   - 角色
4. 保存。

### 18.3 使用者操作

- **編輯**：更改用戶詳細信息
- **刪除**：刪除用戶

---

## 19. 入口網站

門戶網站配置客戶端入口網站。

### 19.1 概述

- 網站名稱
- 站點描述
- 開放報名
- 傳送門路徑
- 門戶連接埠

### 19.2 外觀

- 網站標題
- 標誌URL
- 原色、背景色、文字色和強調色
- 字體

### 19.3 導航

- 新增導航頁面
- 配置頁面名稱、圖標和外部鏈接
- 管理導航項目

### 19.4 模組

- 新增/編輯模組
- 配置模組標題、描述和圖標
- 支援 Markdown、富文本和自訂 HTML

### 19.5 語言

- 新增語言
- 管理副本
- 進出口 JSON

### 19.6 模板

- 切換模板
- 自訂 HTML/CSS/JS
- 重設為預設模板

### 19.7 儲值入口

- 入口網站首頁「去儲值」支援兩種方式：**線上支付**（選擇已啟用的儲值站點）與**儲值卡兌換**；
- 開啟視窗會自動載入可用站點並預設選取第一個線上站點；沒有可用站點時只顯示卡密兌換，
  不會出現空選項；
- 購買計劃/流量包時的「線上支付」使用同一套站點設定。

---

## 20. 系統設定

系統設定配置面板參數。

### 20.1 面板

- 監聽位址和連接埠
- 數據目錄
- 時區
- 訂閱埠
- 出口限額
- 開放報名
- 面板模式（完整/僅管理）

### 20.2 安全

- 加密金鑰管理
- JWT金鑰管理
- IP黑名單
- Passkey 允許列出的域

### 20.3 OAuth

- GitHub OAuth
- Google OAuth
- 回調URL配置

### 20.4 通知

- 電子郵件通知 (SMTP)
- Telegram 通知
- 通知模板
- 監控設定

### 20.5 備份與恢復

- 建立備份
- 備份列表
- 下載備份
- 恢復備份
- 預定備份
- 遠端儲存（FTP / S3 / WebDAV）

### 20.6 自動解鎖

- 啟用自動解鎖
- 冷卻時間
- 適用狀態

---

## 21. 操作控制台

操作控制台管理設定快照並套用作業。

### 21.1 配置快照

- 建立快照
- 列出快照
- 回滾到快照
- 刪除快照

### 21.2 申請職位

- 查看最近的職位
- 立即套用配置（資料庫→執行時間）
- 預檢查（驗證連接埠衝突和配置JSON）
- 取消工作
- 重試作業

### 21.3 節點驗證

- 選擇節點並驗證
- 查看驗證結果
- 驗證所有節點

### 21.4 緊急停止

- 緊急停止代理（寫入空的入站配置）
- 立即停止所有入境流量

---

## 22. 資料庫監控

資料庫監控顯示資料庫健康狀況。

### 22.1 它顯示了什麼

- 資料庫引擎
- 資料庫大小
- 連接性
- 延遲
- 連線池狀態
- 快取統計
- 查詢統計

### 22.2 支援的引擎

- SQLite
- MySQL
- PostgreSQL

---

## 23. 審核日誌

審計日誌記錄操作。

### 23.1 它顯示了什麼

- 類別
- 事件類型
- 結果
- 來源IP
- 使用者
- 詳情

### 23.2 節點操作日誌

- 查看節點操作記錄
- 按節點過濾

---

## 24. 日誌管理

日誌管理顯示系統日誌。

### 24.1 日誌類型

- 面板日誌
- 節點日誌
- 核心日誌
- 代理日誌

### 24.2 行動

- 查看日誌
- 下載日誌
- 清潔日誌

---

## 25. 營運擴展

「營運擴展」（`/client-ext`）配置推薦行銷、佣金和提款；
《法律文件》（`/legal-documents`）維護門戶端協定。

### 25.1 推薦和佣金

- **返傭配置**：返傭比例（受邀者首次付款的百分比，`0` = 停用）、佣金模式（僅限首筆購買/全部購買）、最低提款金額（佣金餘額必須達到該金額才能請求提款）
- **邀請碼**：啟用後，註冊需要邀請碼；設定代碼長度、有效期和最大使用次數（`0` = 無限制），並產生/查看代碼列表
- **提款審核**：依待定/批准/拒絕過濾；新增處理備註並記錄付款方式和錢包地址
- **推薦記錄**：邀請人、被邀請人、佣金金額、佣金狀態

### 25.2 法律文件

- 單獨維護“隱私權政策”和“服務條款”，每個區域一份 Markdown 文檔，並提供即時預覽；
- 每個文件可以使用外部URL代替內建頁面；當為空時，使用內建頁面URL（可在頁面上複製）；
- 入口網站顯示目前區域設定的版本，並在遺失時回退到預設區域設定。

---

## 26. 常見問題解答

### 26.1 面板不啟動

- 檢查連接埠是否已被使用
- 检查`.env`是否有效
- 檢視日誌：`journalctl -u corefusion -f`；不含 systemd、`tail -f <install-dir>/data/logs/panel.log`

### 26.2 節點下線

- 检查代理是否正在运行：`systemctl status corefusion-agent`
- 檢查網路連接
- 检查防火墙规则（端口`18443`）

### 26.3 證書頒發失敗

- 檢查DNS分辨率
- 檢查80埠是否可達
- 檢查 DNS 提供者配置
- 檢查證書日誌

### 26.4 訂閱不起作用

- 檢查訂閱鏈接
- 檢查節點是否在線
- 檢查核心進程是否正在執行
- 檢查防火牆規則

### 26.5 儲值回調失敗

- 查看回調URL
- 檢查站點密鑰是否匹配
- 檢查 IP 允許列表
- 檢查訂單日誌

### 26.6 Portal頁面無法訪問

- 檢查portal埠是否開放
- 檢查Portal路徑配置
- **重要**：確保反向代理將門戶網域轉送到**入口網站引擎連接埠**，而不是主面板連接埠 - 轉送至錯誤的連接埠會顯示空白頁面或面板而不是入口網站。檢查代理站點記錄中的`upstream` / `local_port`。
- 直接開啟`http://<portal-domain>:<portal-port>`驗證入口網站引擎本身
- 檢查反向代理配置
- 檢查門戶日誌

---

## 附錄

### A. 預設連接埠

| 港口 | 目的 |
|------|---------|
| 由設定精靈分配（預設隨機空閒連接埠） | 管理面板（可變更） |
| 18443 | 代理 gRPC（可更改） |
| 0（預設為停用） | 專用訂閱引擎連接埠（空或`0`共享主連接埠） |
| 0（預設為停用） | 專用入口網站引擎連接埠（空或`0`共用主連接埠） |
| 80 / 443 | 反向代理 (Caddy / Nginx) HTTP / HTTPS |

### B.預設路徑

| 路徑 | 目的 |
|------|---------|
| `/` | 管理面板（當未配置專用入口網站引擎時，入口網站也位於 `/`） |
| `/sub/:token` | 訂閱獲取（未配置專用訂閱連接埠時） |
| `/p` | Portal頁面（未配置專用Portal埠時） |
| `/:token` | 訂閱獲取（使用專用訂閱端口，無 `/sub` 前綴） |

### C. 命令列工具

面板二進位檔案支援這些子命令（無參數啟動伺服器）：

```bash
# Start the panel (same as ./corefusion server)
./corefusion

# Run database migrations
./corefusion migrate

# Non-interactive init / reset init state (see 3.4, 3.5)
./corefusion init --help
./corefusion reset-init

# Offline administration (when you cannot sign in)
./corefusion admin reset-password -u admin -p 'new-password'
./corefusion admin 2fa reset -u admin
./corefusion admin list

# Version / help
./corefusion version
./corefusion help
```

> 與systemd一起使用`systemctl restart corefusion`和`journalctl -u corefusion -f`；沒有
> systemd（容器和類似），停止舊進程並使用 `nohup` 重新啟動，日誌為
> 在`data/logs/panel.log`。

---

> **文檔版本**：v1.2
> **最後更新**：2026-11-20
> **適用於**：CoreFusion v0.13.0-rc.1
