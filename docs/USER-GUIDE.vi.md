<!-- source: USER-GUIDE.md | locale: vi | updated: 2026-11-20 -->
# Hướng dẫn vận hành CoreFusion

> Ngôn ngữ: [简体中文](USER-GUIDE.md) · [English](USER-GUIDE.en.md) · [繁體中文](USER-GUIDE.zh-TW.md) · [Русский](USER-GUIDE.ru.md) · [日本語](USER-GUIDE.ja.md) · [한국어](USER-GUIDE.ko.md) · [Deutsch](USER-GUIDE.de.md) · [Français](USER-GUIDE.fr.md) · [Español](USER-GUIDE.es.md) · [Português](USER-GUIDE.pt.md) · [Türkçe](USER-GUIDE.tr.md) · [Bahasa Indonesia](USER-GUIDE.id.md) · [ไทย](USER-GUIDE.th.md) · Tiếng Việt · [العربية](USER-GUIDE.ar.md) · [فارسی](USER-GUIDE.fa.md)

> Hướng dẫn này dành cho người dùng CoreFusion và bao gồm toàn bộ quy trình làm việc từ cài đặt đến vận hành hàng ngày.

---

## Nội dung

1. [Tổng quan về hệ thống](#1-tổng-quan-về-hệ-thống)
2. [Cài đặt](#2-cài-đặt)
3. [Cấu hình ban đầu](#3-cấu-hình-ban-đầu)
4. [Trang tổng quan](#4-trang-tổng-quan)
5. [Quản lý nút](#5-quản-lý-nút)
6. [Trung tâm giao thức](#6-trung-tâm-giao-thức)
7. [Quản lý proxy](#7-quản-lý-proxy)
8. [Tài nguyên định tuyến](#8-tài-nguyên-định-tuyến)
9. [Trung tâm đăng ký](#9-trung-tâm-đăng-ký)
10. [Quản lý khách hàng](#10-quản-lý-khách-hàng)
11. [Quản lý kế hoạch](#11-quản-lý-kế-hoạch)
12. [Quản lý nạp tiền](#12-quản-lý-nạp-tiền)
13. [Quản lý vé](#13-quản-lý-vé)
14. [Quản lý chứng chỉ](#14-quản-lý-chứng-chỉ)
15. [Hạn ngạch giao thông](#15-hạn-ngạch-giao-thông)
16. [Proxy chuỗi](#16-proxy-chuỗi)
17. [Người dùng trực tuyến](#17-người-dùng-trực-tuyến)
18. [Quản lý người dùng](#18-quản-lý-người-dùng)
19. [Trang web cổng thông tin](#19-trang-web-cổng-thông-tin)
20. [Cài đặt hệ thống](#20-cài-đặt-hệ-thống)
21. [Bảng điều khiển hoạt động](#21-bảng-điều-khiển-hoạt-động)
22. [Giám sát cơ sở dữ liệu](#22-giám-sát-cơ-sở-dữ-liệu)
23. [Nhật ký kiểm tra](#23-nhật-ký-kiểm-tra)
24. [Quản lý nhật ký](#24-quản-lý-nhật-ký)
25. [Tiện ích mở rộng hoạt động](#25-tiện-ích-mở-rộng-hoạt-động)
26. [Câu hỏi thường gặp](#26-câu-hỏi-thường-gặp)
- [Phụ lục](#phụ-lục)

---

## 1. Tổng quan về hệ thống

CoreFusion là bảng quản lý proxy đa giao thức hỗ trợ VLESS, VMess, Trojan, Shadowsocks, Hysteria2, TUIC, v.v., với quản lý nút, phân phối đăng ký, quản lý khách hàng, thanh toán, chứng chỉ và proxy chuỗi.

### 1.1 Tính năng cốt lõi

| mô-đun | Mô tả |
|--------|-------------|
| Đa giao thức | VLESS, VMess, Trojan, Shadowsocks, Hysteria2, TUIC, WireGuard, v.v. |
| Đa lõi | Xray-core và sing-box (Hysteria2/TUIC và các giao thức QUIC khác chạy trên sing-box) |
| Đa nút | Năm loại nút: độc lập, nhân bản, hoạt động/dự phòng, mục nhập theo chuỗi, thoát theo chuỗi |
| Phân phối đăng ký | Nhiều định dạng đăng ký, điểm cuối và quản lý định dạng |
| Thanh toán | Mẫu kế hoạch, thẻ nạp tiền, số dư, đơn đặt hàng, hoa hồng đại lý |
| Chứng chỉ | Let's Encrypt phát hành, xác thực DNS-01/HTTP-01, phân phối chứng chỉ |
| Proxy bị xích | Cân bằng tải nhiều lối ra, đăng ký nút bên ngoài, định tuyến theo chuỗi |
| Hạn ngạch giao thông | Giám sát lưu lượng truy cập hàng tháng trên mỗi nút, tự động tạm dừng, đặt lại nhật ký |
| Cổng thông tin | Trình chỉnh sửa cổng thông tin trực quan, đa ngôn ngữ, mẫu tùy chỉnh |
| Thông báo | Email và Telegram, các mẫu và chương trình phát sóng |

### 1.2 Kiến trúc

- **Phần cuối**: Đi, gRPC
- **Giao diện người dùng**: Vue 3 + Element Plus
- **Cơ sở dữ liệu**: SQLite (mặc định) / MySQL / PostgreSQL
- **Proxy**: Proxy ngược Caddy / Nginx
- **Cổng bảng điều khiển**: được chỉ định bởi trình hướng dẫn thiết lập (cổng trống ngẫu nhiên theo mặc định; 8080 là lựa chọn phổ biến)
- **Cổng gRPC của tác nhân**: 18443 theo mặc định

---

## 2. Cài đặt

### 2.1 Môi trường được hỗ trợ

| Mục | Hỗ trợ |
|------|---------|
| Phân phối | Debian / Ubuntu, RHEL / CentOS / Rocky / AlmaLinux / Fedora, Alpine, Arch, openSUSE |
| Kiến trúc CPU | Các nhị phân chính thức: `amd64`, `arm64`; bản dựng nguồn cũng hỗ trợ `armv7`, `armv6`, `386`, `ppc64le`, `s390x`, `riscv64`, `loong64` |
| Người quản lý dịch vụ | systemd (được khuyến nghị); quay trở lại `nohup` trong các thùng chứa hoặc các môi trường khác không có systemd |
| Bộ nhớ/đĩa | ≥ 1 GB / ≥ 10 GB (cơ sở dữ liệu, chứng chỉ, nhật ký và tệp nhị phân lõi nằm trong thư mục dữ liệu) |
| Cổng bảng điều khiển | được chọn trong trình hướng dẫn thiết lập (cổng trống ngẫu nhiên theo mặc định; `8080` và các cổng tương tự đều ổn) |
| Đại lý gRPC | `18443` theo mặc định |

Các lệnh bên ngoài trong thời gian chạy được trình cài đặt tự động cài đặt (`scripts/install.sh` và tập lệnh cài đặt Tác nhân do bảng điều khiển tạo ra; bỏ qua với `--no-deps` / `CF_SKIP_DEPS=1`):

| Mục đích | Gói |
|---------|---------|
| Tường lửa/giới hạn tốc độ/đánh dấu đồng thời | `nftables` (`nft`) |
| Định hình giao thông, liên kết và ổ cắm | `iproute2` (`tc` / `ip` / `ss`) |
| Dự phòng khi nft không có sẵn | `iptables` (`iptables-nft` trên EL8+, `iptables` trên EL7) |
| Theo dõi kết nối | `conntrack-tools` (`conntrack`) |
| Tải `nf_conntrack` / `ifb` | `kmod` (`modprobe` / `lsmod`) |
| Phát hiện và dọn dẹp quy trình | `procps` (`pgrep` / `pkill`) |
| Kiểm tra cổng/thông số DH | `lsof` / `openssl` |
| Tạo người dùng proxy | `shadow-utils` (Debian `passwd`, Alpine/Arch `shadow`) |
| Tải xuống và lưu trữ | `curl` / `ca-certificates` / `tar` / `gzip` / `xz` |

> Thiếu `nft`/`tc` chỉ ảnh hưởng đến tính năng giới hạn tốc độ và tường lửa; bảng điều khiển vẫn bắt đầu.
> Chạy `--cpu-info` trước để xem cách phát hiện môi trường và gói nào bị thiếu.

### 2.2 Cài đặt một dòng (khuyến nghị)

```bash
curl -fsSL https://raw.githubusercontent.com/qist/corefusion/main/scripts/install.sh -o install.sh
sudo bash install.sh
```

Hoặc đặt trực tiếp (không tương tác, không có cờ bổ sung):

```bash
curl -fsSL https://raw.githubusercontent.com/qist/corefusion/main/scripts/install.sh | sudo bash
```

Kịch bản sẽ:

1. Phát hiện bản phân phối, kiến trúc CPU và libc, sau đó cài đặt các gói cơ sở và các phần phụ thuộc thời gian chạy;
2. Tải xuống `corefusion-linux-<arch>` từ Bản phát hành GitHub và xác minh nó bằng `.sha256` phù hợp;
3. Cài đặt vào `/opt/corefusion` và tạo `data/{sqlite,certs,logs,runtime}`;
4. Chỉ viết trình giữ chỗ `.env` với `SAFE_PATHS=data` — không có cấu hình cơ sở dữ liệu — vì vậy lần khởi động đầu tiên sẽ đi vào trình hướng dẫn thiết lập (`.env` đầy đủ được trình hướng dẫn tạo ra);
5. Cài đặt và khởi động dịch vụ systemd (hoặc chạy trong `nohup` khi systemd không khả dụng);
6. Đợi cho đến khi sẵn sàng, sau đó in đường dẫn thiết lập URL, mã thông báo và thư mục.

Đầu ra ví dụ:

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

Nếu bảng điều khiển chỉ nghe trên `127.0.0.1` (ví dụ: sau proxy ngược), tập lệnh sẽ in lệnh đường hầm SSH thay vì URL không thể truy cập được.

> Mất mã thông báo? Đọc lại nhật ký khởi động: `journalctl -u corefusion -n 50 --no-pager`
> (không có systemd: `tail -n 50 /opt/corefusion/data/logs/panel.log`).

### 2.2.1 Đầu ra đa ngôn ngữ

Trình cài đặt bao gồm bốn ngôn ngữ giao diện người dùng: Tiếng Trung giản thể, tiếng Anh, tiếng Ba Tư (فارسی) và tiếng Nga (Русский). Nó tự động phát hiện hệ thống `LANG` hoặc bạn có thể chọn rõ ràng:

```bash
# English
sudo bash install.sh --lang en

# Persian via environment variable
sudo CF_LANG=fa bash install.sh

# Pass options through a pipe
curl -fsSL https://raw.githubusercontent.com/qist/corefusion/main/scripts/install.sh | sudo bash -s -- --lang ru
```

Các giá trị được hỗ trợ: `zh` / `en` / `fa` / `ru`; `--lang=ru` cũng được chấp nhận. Giá trị rõ ràng không được hỗ trợ không thành công với danh sách các ngôn ngữ có sẵn.

### 2.3 Phiên bản ghim/gương/cài đặt offline

```bash
# Pin a version
sudo bash install.sh --version v0.13.1

# GitHub download mirror (used as a URL prefix)
sudo bash install.sh --mirror https://ghproxy.net --version latest

# Use a local binary (offline / intranet / pre-built)
sudo bash install.sh --local ./dist/corefusion-linux-arm64

# Explicit target architecture
sudo bash install.sh --arch arm64 --local ./corefusion-linux-arm64
```

> Nếu không có `--version`, tập lệnh sẽ sử dụng mục mới nhất trong danh sách Bản phát hành GitHub.
> Hầu hết các bản phát hành đều là bản phát hành trước nên điểm cuối `/releases/latest` không được sử dụng.
>
> Bản phát hành chính thức chỉ cung cấp `amd64` / `arm64`; sử dụng `--source` hoặc tệp nhị phân được biên dịch chéo với `--local` cho các kiến ​​trúc khác.

### 2.4 Xây dựng từ nguồn

```bash
sudo bash install.sh --source --version v0.13.1
```

`--source` sao chép kho lưu trữ (hoặc sử dụng thanh toán hiện có khi tập lệnh nằm trong đó) và chạy `make panel`. Các phiên bản chuỗi công cụ được chọn cho mỗi môi trường:

| phụ thuộc | Lựa chọn |
|------------|-----------|
| Đi | Đọc từ `toolchain` / `go` trong `go.mod` (hiện **Go 1.27+**); nếu phiên bản hệ thống quá cũ, chuỗi công cụ phù hợp sẽ được tải xuống `<install-dir>/.toolchain` |
| nút | **Node 22 LTS** theo mặc định (Vite yêu cầu ≥ 18); bản dựng chính thức phù hợp sẽ được tải xuống khi cần |
| Nút (glibc < 2,28) | Sử dụng bản dựng `unofficial-builds` glibc-217 (ví dụ: CentOS 7) |
| Nút (Alpine / Musl) | Sử dụng bản phân phối `nodejs` / `npm` |

Ghi đè:

```bash
# Force Go / Node versions
sudo CF_GO_VERSION=1.27.5 CF_NODE_MAJOR=22 bash install.sh --source

# Go toolchain mirror (default https://go.dev/dl)
sudo CF_GO_MIRROR=https://golang.google.cn/dl bash install.sh --source
```

Xây dựng thủ công (phát triển/gỡ lỗi):

```bash
git clone https://github.com/qist/corefusion.git
cd corefusion

# Frontend + panel for the current architecture; the frontend is embedded via go:embed
make panel

# Cross-compile
make panel-linux-amd64
make panel-linux-arm64
```

> `go build ./...` không tạo ra nhị phân. Xây dựng bảng điều khiển với
> `go build -o corefusion ./cmd/corefusion/` hoặc `make panel`.

### 2.5 Cờ cài đặt

| Cờ | Mô tả |
|------|-------------|
| `-d, --dir <PATH>` | Thư mục cài đặt, mặc định `/opt/corefusion` |
| `-v, --version <TAG>` | Phiên bản bảng điều khiển, ví dụ: `v0.13.1`; mặc định `latest` |
| `--binary` | Sử dụng mã nhị phân phát hành GitHub (mặc định) |
| `--source` | Xây dựng từ nguồn và chuẩn bị Go/Node phù hợp |
| `--local <FILE>` | Sử dụng tệp nhị phân cục bộ (cài đặt ngoại tuyến) |
| `--arch <ARCH>` | Kiến trúc mục tiêu: `amd64` / `arm64` / `armv7` / `armv6` / `386` / `ppc64le` / `s390x` / `riscv64` / `loong64` |
| `--mirror <URL>` | Tiền tố tải xuống GitHub, ví dụ: `https://ghproxy.net` |
| `--repo <URL>` | Kho nguồn, mặc định `https://github.com/qist/corefusion.git` |
| `--ref <REF>` | Nhánh/thẻ nguồn; mặc định là `--version`, nếu không thì `main` |
| `--no-deps` | Bỏ qua cài đặt phụ thuộc hệ thống |
| `--no-start` | Cài đặt và ghi tệp dịch vụ mà không cần khởi động |
| `--cpu-info` | In phát hiện môi trường và thiếu phần phụ thuộc, sau đó thoát |
| `--lang <LANG>` | Ngôn ngữ đầu ra: `zh` / `en` / `fa` / `ru` (mặc định: hệ thống `LANG`) |
| `--uninstall` | Dừng dịch vụ và xóa chương trình (giữ dữ liệu và `.env`) |
| `--purge` | Với `--uninstall` cũng xóa luôn thư mục data và `.env` |

> Các bản phát hành chính thức chỉ xuất xưởng các tệp nhị phân `amd64` / `arm64`. Các kiến trúc khác yêu cầu `--source`,
> nếu không thì quá trình tải xuống sẽ trả về 404.

Các biến môi trường liên quan:

| Biến | Mục đích |
|----------|---------|
| `CF_GO_VERSION` | Ghi đè phiên bản Go cần thiết cho các bản dựng nguồn |
| `CF_NODE_MAJOR` | Ghi đè phiên bản chính của Node (mặc định 22) |
| `CF_GO_MIRROR` | Đi nhân bản tải xuống chuỗi công cụ |
| `CF_SKIP_DEPS` | Đặt thành `1` trong tập lệnh cài đặt Tác nhân để bỏ qua cài đặt phụ thuộc |
| `CF_LANG` | Ghi đè ngôn ngữ đầu ra của trình cài đặt (giống như `--lang`) |

### 2.6 Bố cục thư mục

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

> Trình cài đặt chỉ tạo `sqlite/`, `certs/`, `logs/` và `runtime/`; `binaries/`, `proxy/`, `rules/` và các loại khác được tạo bởi bảng điều khiển hoặc khi cài đặt lõi.
> Thư mục dữ liệu đến từ `SAFE_PATHS` trong `.env` và có thể là đường dẫn tuyệt đối. Thay đổi thư mục cài đặt không di chuyển dữ liệu hiện có.

### 2.7 Quản lý dịch vụ

systemd (hầu hết các bản phân phối):

```bash
systemctl status corefusion --no-pager
systemctl restart corefusion
systemctl stop corefusion
journalctl -u corefusion -f          # follow logs
journalctl -u corefusion -n 100      # last 100 lines
```

Không có systemd (thùng chứa, v.v.):

```bash
cd /opt/corefusion && ./corefusion            # foreground (debugging)
cd /opt/corefusion && setsid nohup ./corefusion >> data/logs/panel.log 2>&1 &
tail -f data/logs/panel.log
```

> Nếu không có systemd, quá trình sẽ không bắt đầu khi khởi động; khởi động lại thủ công sau khi khởi động lại (hoặc chạy lại trình cài đặt).

### 2.8 Nâng cấp và gỡ cài đặt

Nâng cấp (giữ dữ liệu và cấu hình):

```bash
sudo bash install.sh --version <new-version>
```

Tập lệnh dừng dịch vụ, sao lưu tệp nhị phân cũ dưới dạng `corefusion.bak`, thay thế nó và khởi động lại. `.env` và `data/` hiện tại vẫn được giữ nguyên.

Gỡ cài đặt:

```bash
# Remove the program only; keep data and .env
sudo bash install.sh --uninstall

# Also remove the data directory and .env (irreversible)
sudo bash install.sh --uninstall --purge
```

### 2.9 Tệp môi trường (.env)

`.env` hoàn chỉnh được **tạo bởi trình hướng dẫn thiết lập** khi cấu hình được lưu (DSN cơ sở dữ liệu, thư mục dữ liệu, bí mật, cổng, miền).

Khi cài đặt mới, trình cài đặt chỉ ghi trình giữ chỗ này, do đó bảng điều khiển không nhầm `data/` hiện có với cài đặt đã khởi tạo và bỏ qua trình hướng dẫn:

```env
SAFE_PATHS=data
```

> Khi đã tồn tại một cấu hình hoàn chỉnh (`.env` với `CF_DB_DSN`), trình cài đặt sẽ không bao giờ ghi đè lên cấu hình đó và trình hướng dẫn sẽ không xuất hiện lại.
> Các chỉnh sửa thủ công sau này đối với `.env` sẽ có hiệu lực sau khi khởi động lại.

#### Thư mục dữ liệu

```env
SAFE_PATHS=data                       # relative to the install directory
# SAFE_PATHS=/srv/corefusion-data     # or an absolute path
```

#### Cơ sở dữ liệu

`CF_DB_DSN` được phát hiện theo tiền tố:

| Tiền tố | Cơ sở dữ liệu |
|--------|----------|
| không được đặt hoặc một con đường trống | SQLite (mặc định) |
| `sqlite://` | SQLite (rõ ràng) |
| `mysql://` | MySQL |
| `postgres://` / `postgresql://` | PostgreSQL |

`CF_DB_TYPE` là tính năng phát hiện tùy chọn và ghi đè (`sqlite` / `mysql` / `postgres`).

> **Mật khẩu được mã hóa**: DSN MySQL/PostgreSQL được bảng điều khiển ghi (trình hướng dẫn thiết lập/chuyển đổi cơ sở dữ liệu)
> được lưu trữ trong `.env` dưới dạng `enc:...` (AES-256-GCM, khóa `CF_SECRET_ENCRYPTION_KEY`).
> Chúng được giải mã khi đọc; DSN văn bản gốc được viết bằng tay vẫn hoạt động.

**SQLite (mặc định)**

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

#### Nhóm kết nối (tùy chọn, MySQL/PostgreSQL)

```env
CF_DB_MAX_OPEN_CONNS=100
CF_DB_MAX_IDLE_CONNS=10
```

#### Các biến phổ biến khác

| Biến | Mô tả |
|----------|-------------|
| `JWT_SECRET` | Khóa ký cho mã thông báo đăng nhập; được tạo trong lần khởi động đầu tiên khi bị thiếu |
| `CF_SECRET_ENCRYPTION_KEY` | Khóa mã hóa cài đặt nhạy cảm (AES-256-GCM); được tạo ra khi thiếu |
| `CF_INITIAL_ADMIN_PASSWORD` | Mật khẩu quản trị viên ban đầu (tùy chọn; ngẫu nhiên và được in một lần theo mặc định) |
| `CF_NETWORK_TRAFFIC_ACCOUNTING` | Đặt thành `0` để tắt tính năng tính toán lưu lượng truy cập dựa trên iptables (được bật theo mặc định) |
| `CF_NETWORK_TRAFFIC_TABLE` | bảng iptables được sử dụng để tính toán lưu lượng, `corefusion` mặc định |

> **Khóa mã hóa tồn tại trong `.env.secret`**: khi khởi động lần đầu, `CF_SECRET_ENCRYPTION_KEY` được ghi vào `.env.secret` (0600, cùng thư mục với `.env`) thay vì `.env`; khóa kế thừa trong `.env` được di chuyển tự động. Sao lưu tệp này một cách riêng biệt — việc mất nó sẽ khiến mọi giá trị `enc:` không thể giải mã được.

> Proxy ngược đáng tin cậy để phân giải IP máy khách thực không còn được định cấu hình thông qua các biến môi trường;
> chúng được lưu trữ dưới dạng `panel.trusted_proxies` trong **Cài đặt → Mạng**.

> Cả `.env` và `.env.secret` đều chứa đựng bí mật; bảng điều khiển thắt chặt quyền của họ đối với `0600` khi bắt đầu. Không bao giờ cam kết chúng.

### 2.10 Docker (tùy chọn, xây dựng hình ảnh của riêng bạn)

Kho lưu trữ **không** gửi Dockerfile. Nếu bạn chứa nó, hãy viết hình ảnh của riêng bạn. Lời gọi tham khảo:

```bash
docker run -d \
  --name corefusion \
  -p 8080:8080 \
  -p 18443:18443 \
  -v /opt/corefusion/data:/app/data \
  -v /opt/corefusion/.env:/app/.env \
  <your-image>
```

> Các container thường không có đặc quyền systemd và `nft`, do đó các tính năng giới hạn tốc độ/tường lửa sẽ không hoạt động.
> Thư mục làm việc phải khớp với cơ sở cho `SAFE_PATHS` tương đối.

### 2.11 Kiểm tra sau cài đặt

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

## 3. Cấu hình ban đầu

Trong lần khởi động đầu tiên, nếu `.env` không có `CF_DB_DSN` hợp lệ thì bảng điều khiển **chưa** phục vụ giao diện người dùng quản trị viên. Nó bắt đầu
**trình hướng dẫn thiết lập** trên một cổng ngẫu nhiên trong khoảng thời gian 18000–18999 bằng mã thông báo một lần, sau đó khởi động lại như bảng thông thường sau khi được lưu.

### 3.1 Mở trình hướng dẫn thiết lập

1. Sao chép “trình hướng dẫn thiết lập” URL từ đầu ra của trình cài đặt; hoặc
2. Đọc nhật ký trên máy chủ:

```bash
journalctl -u corefusion -n 50 --no-pager | grep -A2 'setup wizard'
```

```text
CoreFusion first start - setup wizard
  URL   : http://<ip>:<port>/init?init_token=<token>
  Token : <token>
  Valid : 2h0m0s
```

3. Mở URL trong trình duyệt (nó đã chứa `init_token` nên không cần nhập thủ công).

> Theo mặc định, phiên hướng dẫn sẽ hết hạn sau **2 giờ**. Nếu nó hết hạn hoặc bảng điều khiển khởi động lại, hãy lấy một cái mới.
> Không chia sẻ mã thông báo. Sau khi thiết lập hoàn tất, cổng này sẽ đóng lại và chỉ còn lại cổng bảng điều khiển thông thường.

### 3.2 Các bước hướng dẫn

Trình hướng dẫn có 8 bước. Bạn có thể quay lại bất cứ lúc nào; không có gì được cam kết cho đến lần lưu cuối cùng.

| Bước | Nội dung |
|------|---------|
| 1. Cơ sở dữ liệu | SQLite / MySQL / PostgreSQL |
| 2. Thư mục dữ liệu | Được viết cho `SAFE_PATHS` trong `.env` |
| 3. Khóa mã hóa | `JWT_SECRET` và `CF_SECRET_ENCRYPTION_KEY` |
| 4. Lõi | Những lõi proxy nào cần cài đặt |
| 5. Ủy nhiệm | Không có / Nginx / Caddy |
| 6. Trang web và tên miền | Cổng, đường dẫn web, tài khoản quản trị viên, tiền tệ, múi giờ, tên miền |
| 7. Chứng chỉ HTTPS | Đã tắt / tải lên / ACME |
| 8. Tóm tắt | Xem lại, tải xuống `.env`, lưu và khởi động lại |

#### Bước 1: Cơ sở dữ liệu

- **SQLite (tệp cục bộ, được khuyến nghị)**: để trống đường dẫn để sử dụng `<data-dir>/sqlite/app.db`.
- **MySQL / PostgreSQL**: máy chủ, cổng, người dùng, mật khẩu, tên cơ sở dữ liệu; PostgreSQL cũng hỗ trợ `disable` / `require` / `verify-full`.
- Nhấp vào “Kiểm tra kết nối” để xác thực. Tiết kiệm cũng kiểm tra nó. Cơ sở dữ liệu bị thiếu sẽ được tạo tự động (tạo thủ công nếu tài khoản thiếu `CREATE`).
- DSN (bao gồm cả mật khẩu) được mã hóa trước khi được ghi vào `.env`.

#### Bước 2: Thư mục dữ liệu

Lưu trữ cơ sở dữ liệu, chứng chỉ, nhật ký, tệp nhị phân lõi và hơn thế nữa; đây là `SAFE_PATHS` trong `.env`. Mặc định là `data`
(liên quan đến thư mục cài đặt); một đường dẫn tuyệt đối được cho phép. **Đừng tùy tiện thay đổi nó sau này**, hoặc bảng điều khiển
sẽ không tìm thấy cơ sở dữ liệu và chứng chỉ hiện có.

#### Bước 3: Khóa mã hóa

- `JWT_SECRET`: ký mã thông báo đăng nhập.
- `CF_SECRET_ENCRYPTION_KEY`: mã hóa các cài đặt nhạy cảm (mật khẩu cơ sở dữ liệu, thông tin xác thực DNS, thẻ nạp tiền, v.v.).
- Cả hai đều được tạo ngẫu nhiên theo mặc định; sử dụng “Tái tạo” để nhận các giá trị mới. **Thay đổi `CF_SECRET_ENCRYPTION_KEY` khiến dữ liệu được mã hóa trước đó không thể đọc được** — hãy sao lưu dữ liệu đó.

#### Bước 4: Lõi

Chọn lõi proxy để cài đặt:

- **Xray**: VLESS / VMess / Trojan / Shadowsocks, v.v.;
- **sing-box**: mang Hysteria2 / TUIC và các giao thức QUIC khác.

Sau khi lưu và khởi động lại, bảng điều khiển sẽ tải xuống và cài đặt chúng tự động. Kiểm tra phiên bản và trạng thái trong “Giao thức”. Bạn cũng có thể cài đặt chúng sau.

#### Bước 5: Ủy quyền

- **Không**: bảng điều khiển phục vụ khách hàng trực tiếp (tự xử lý TLS).
- **Nginx / Caddy**: bảng quản lý proxy ngược và lưu trữ chứng chỉ; chọn một.

Với proxy được chọn, bảng điều khiển thường chỉ lắng nghe trên `127.0.0.1` và proxy phục vụ thế giới bên ngoài.

#### Bước 6: Trang web và tên miền

| trường | Mô tả |
|-------|-------------|
| Nghe địa chỉ | Xuất phát tự động: proxy → `127.0.0.1`; không có proxy có chứng chỉ → `0.0.0.0`; mặt khác → `127.0.0.1` |
| Cổng bảng điều khiển | Cổng miễn phí ngẫu nhiên theo mặc định; thay đổi nó nếu bạn thích (ví dụ: 8080) |
| Cổng đăng ký | Tùy chọn; `0` chia sẻ cổng bảng điều khiển |
| Cổng cổng thông tin | Tùy chọn; `0` chia sẻ cổng bảng điều khiển |
| Đường dẫn web | Đường dẫn giao diện người dùng quản trị viên, `/` mặc định; API luôn là `/api/v1` |
| Tên người dùng/mật khẩu quản trị viên | Ngẫu nhiên theo mặc định; **mật khẩu chỉ được hiển thị một lần trong phần tóm tắt** — hãy lưu nó |
| Chế độ truy cập | “Tên miền” hoặc “IP” |
| Tiền tệ/múi giờ mặc định | Loại tiền mặc định cho thẻ nạp tiền và số dư; múi giờ của bảng điều khiển và nhật ký proxy (ví dụ: `Asia/Shanghai`) |
| Thêm vào proxy ngược | Khi proxy và miền được đặt, hãy thêm miền bảng điều khiển / đăng ký / cổng thông tin vào proxy chỉ trong một lần |

Nhập tên miền ghi các cài đặt tương ứng (tên miền gọi lại, điểm cuối đăng ký, tên miền chia sẻ).

#### Bước 7: Chứng chỉ HTTPS

| Nguồn | Mô tả |
|--------|-------------|
| Vô hiệu hóa HTTPS | Bảng điều khiển/cổng thông tin/đăng ký được cung cấp qua HTTP (không an toàn); với proxy ngược, bạn có thể thêm TLS vào phía proxy sau |
| Tải chứng chỉ lên | Dán chứng chỉ PEM và khóa riêng PEM (chỉ chế độ truy cập tên miền) |
| ACME | Cấp chứng chỉ khi bạn gửi bước này; hỗ trợ chứng chỉ tên miền và IP |

Tùy chọn ACME:

- **CA**: Let's Encrypt, Let's Encrypt (Staging), ZeroSSL, Google Public CA;
- **Mục tiêu**: miền được định cấu hình hoặc IP công cộng (chứng chỉ IP chỉ được Let's Encrypt hỗ trợ);
- **Thử thách**: `HTTP-01` (yêu cầu cổng 80) hoặc `DNS-01` (ký tự đại diện, không có cổng 80; chọn nhà cung cấp DNS và nhập thông tin xác thực);
- **Loại / kích thước khóa**: ECC 256/384 hoặc RSA 2048/3072/4096;
- **Google CA công khai**: yêu cầu EAB; nhập Key ID/HMAC theo cách thủ công hoặc sử dụng tài khoản dịch vụ GCP.

> Chứng chỉ IP tự động phát hiện IPv4/IPv6 công khai của máy chủ. Đằng sau proxy, địa chỉ được phát hiện là của máy chủ chứ không phải proxy đầu ra.
> Chứng chỉ tên miền cũng có thể được cấp sau trong phần “Chứng chỉ”.

#### Bước 8: Tóm tắt

Hiển thị cơ sở dữ liệu, thư mục dữ liệu, lõi, proxy, cổng, tài khoản quản trị viên, tên miền và khóa `.env` (bí mật được che giấu) và cung cấp:

- **Tải xuống .env**: giữ một bản sao của cấu hình;
- **URL truy cập**: URL cuối cùng cho bảng điều khiển / đăng ký / cổng thông tin; HTTP được đánh dấu là không an toàn.

Nhấp vào “Lưu và khởi động lại” để ghi cơ sở dữ liệu và `.env`; bảng điều khiển khởi động lại vào dịch vụ bình thường.

### 3.3 Sau khi thiết lập

- Dịch vụ khởi động lại và nhật ký khởi động in địa chỉ nghe cuối cùng;
- Đăng nhập bằng tài khoản quản trị viên, truy cập URL và đường dẫn web từ bản tóm tắt;
- Nếu mất mật khẩu, hãy đặt lại ngoại tuyến bằng `./corefusion admin reset-password`.

### 3.4 Thiết lập CLI không tương tác

Đối với máy chủ tự động hóa hoặc không có đầu, hãy viết trực tiếp cấu hình và đánh dấu thiết lập hoàn tất:

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

Cờ phổ biến:

| Cờ | Mô tả |
|------|-------------|
| `--db-type` / `--db-host` / `--db-port` / `--db-user` / `--db-password` / `--db-name` | Cài đặt cơ sở dữ liệu |
| `--data-dir` | Thư mục dữ liệu; mặc định là `SAFE_PATHS` hoặc `./data` |
| `--listen-port` / `--subscription-port` / `--portal-port` / `--web-path` | Cổng và đường dẫn web (cổng trống ngẫu nhiên theo mặc định) |
| `--admin-username` / `--admin-password` | Tài khoản quản trị viên (mật khẩu ngẫu nhiên được in một lần khi bỏ qua) |
| `--proxy-type` | `none` / `nginx` / `caddy` |
| `--cores` | Các lõi cần cài đặt, được phân tách bằng dấu phẩy: `xray,singbox` |
| `--panel-domain` / `--subscription-domain` / `--portal-domain` | Tên miền (được ghi vào cài đặt và được thêm vào proxy ngược) |
| `--tls-mode` | `none` / `self_signed` / `acme` |
| `--tls-domain` / `--tls-email` / `--tls-challenge` / `--tls-key-type` / `--tls-key-bits` | Tùy chọn chứng chỉ (`--tls-challenge` mặc định là `http-01`; `dns-01` cần `--tls-dns-channel` hoặc `--tls-dns-provider`) |
| `--tls-ip-cert` | Cấp chứng chỉ cho IP công cộng được phát hiện (Chỉ Let's Encrypt) |
| `--acme-provider` | `lets_encrypt` / `lets_encrypt_staging` / `zerossl` / `google` |
| `--currency` / `--timezone` | Tiền tệ/múi giờ mặc định |

Danh sách đầy đủ: `./corefusion init --help`. Sau đó bắt đầu dịch vụ:

```bash
systemctl restart corefusion
```

### 3.5 Chạy lại trình hướng dẫn thiết lập

Để bắt đầu lại (xóa `.env` và các dấu thiết lập, **giữ lại dữ liệu**):

```bash
cd /opt/corefusion
./corefusion reset-init      # type RESET to confirm
systemctl restart corefusion
```

Sau đó sử dụng URL mới và mã thông báo từ nhật ký khởi động.

> Việc khởi tạo lại sẽ đặt lại tài khoản quản trị viên và đường dẫn web. Dữ liệu doanh nghiệp trong cơ sở dữ liệu cũ vẫn còn nhưng bạn có thể cần phải tạo lại tài khoản đăng nhập.
>
> `reset-init` xóa toàn bộ `.env` (bao gồm `SAFE_PATHS`). **Nếu thư mục dữ liệu của bạn là đường dẫn tuyệt đối tùy chỉnh**, hãy khôi phục `SAFE_PATHS` trước khi khởi động lại, nếu không, bảng điều khiển sẽ trông như `./data`:
>
> ```bash
> printf 'SAFE_PATHS=/srv/corefusion-data\n' > /opt/corefusion/.env
> ```

### 3.6 Bảo mật sau lần đăng nhập đầu tiên

#### Thay đổi mật khẩu ban đầu

1. Nhấp vào tên người dùng ở góc trên bên phải;
2. Chọn “Đổi mật khẩu”;
3. Nhập mật khẩu cũ và mới;
4. Nhấp vào “Lưu”.

#### Kích hoạt đăng nhập mã động (2FA / TOTP)

> Sau khi được bật, tài khoản sẽ chuyển sang **đăng nhập mã động**: mật khẩu tĩnh không còn được chấp nhận; chỉ có
> Mã gồm 6 chữ số từ ứng dụng xác thực của bạn (hoặc mã khôi phục một lần) hoạt động. Điều này **thay thế** đăng nhập bằng mật khẩu - đó là
> không phải “mật khẩu + mã” xác minh hai bước.

1. Nhấp vào tên người dùng ở góc trên bên phải;
2. Chọn “Bật 2FA”;
3. Nhập mật khẩu hiện tại (chỉ để xác nhận thay đổi này);
4. Quét mã QR bằng ứng dụng xác thực (Google Authenticator / 1Password, v.v.) hoặc nhập khóa theo cách thủ công;
5. Nhập mã gồm 6 chữ số để hoàn tất kích hoạt;
6. **Lưu mã khôi phục ngay lập tức** (có thể tải xuống); mỗi mã hoạt động một lần.

Đăng nhập sau đó:

- Sau khi nhập tên người dùng, trường mật khẩu sẽ trở thành “Mã 6 chữ số / mã khôi phục” và nút trở thành “Đăng nhập bằng Mã”;
- Nhập mã hiện tại; **mật khẩu tĩnh không thể sử dụng được nữa**;
- Mã khôi phục chỉ dùng một lần; tạo lại chúng trong phần “Đăng nhập mã động (2FA)” khi chúng hết (điều này làm mất hiệu lực tất cả các mã cũ).

Nếu trình xác thực bị mất và mã khôi phục đã hết, hãy tắt 2FA ngoại tuyến trên máy chủ:

```bash
cd /opt/corefusion
./corefusion admin 2fa reset -u admin    # disables 2FA, clears the secret and recovery codes, revokes all sessions
```

> Quản trị viên cấp cao duy nhất không có trình xác thực cũng như mã khôi phục không thể khôi phục thông qua giao diện người dùng web và phải sử dụng lệnh trên.

#### Đăng ký Passkey (tùy chọn)

1. Nhấp vào tên người dùng ở góc trên bên phải;
2. Chọn “Quản lý Passkey”;
3. Nhấp vào “Đăng ký Passkey mới”;
4. Hoàn thành lời nhắc thiết bị.

> **Lưu ý**: Mật mã yêu cầu HTTPS hoặc localhost và miền phải nằm trong danh sách cho phép.

---

## 4. Trang tổng quan

Trang tổng quan hiển thị tổng quan về cụm nhiều nút và tình trạng nút.

### 4.1 Nó thể hiện điều gì

- **Thông tin hệ thống**: tên máy chủ, phiên bản hệ điều hành, thời gian hoạt động, CPU, bộ nhớ
- **Lưu lượng truy cập**: băng thông thời gian thực, tổng lưu lượng truy cập, xu hướng hàng tháng
- **Tình trạng nút**: số lượng trực tuyến / ngoại tuyến / xuống cấp, các nút tải cao
- **Cảnh báo**: nút ngoại tuyến, nút xuống cấp, chứng chỉ sắp hết hạn, cảnh báo hạn ngạch
- **Tài nguyên cụm**: CPU, bộ nhớ, ổ đĩa và thông lượng trung bình

### 4.2 Hành động

- Nhấp vào “Xem chi tiết” để mở trang chi tiết của nút
- Nhấp vào thẻ nút để xem trạng thái trực tiếp của nó
- Sử dụng bộ lọc để thu hẹp theo trạng thái, loại hoặc khu vực

---

## 5. Quản lý nút

Quản lý nút là cốt lõi của CoreFusion và hỗ trợ một số loại nút.

### 5.1 Các loại nút

| loại | Mô tả |
|------|-------------|
| Độc lập (`standalone`) | Một nút lõi độc lập |
| Gương (`mirror`) | Đồng bộ hóa cấu hình từ nút mục tiêu |
| Chính/dự phòng (`primary_backup`) | Các nút đối tác không kết nối được với nhau |
| Mục nhập chuỗi (`chain_entry`) | Nút đầu vào của proxy chuỗi |
| Thoát chuỗi (`chain_exit`) | Nút thoát của proxy chuỗi |

### 5.2 Thêm nút

1. Mở “Quản lý nút”;
2. Nhấp vào “Thêm máy chủ”;
3. Điền thông tin cơ bản:
   - Tên, ví dụ: `HongKong-HK01`
   - Tên máy chủ: IP hoặc tên miền
   - Khu vực, ví dụ: `HK`, `JP`, `US`
   - Loại nút;
4. Đặt cổng Tác nhân (`18443` mặc định);
5. Chọn các lõi được hỗ trợ;
6. Nhấp vào “Lưu”.

### 5.3 Triển khai bằng một cú nhấp chuột

1. Sau khi thêm nút, chọn “Triển khai lệnh” trong danh sách nút (trang chi tiết cũng cung cấp
 “Lệnh cài đặt Bootstrap”);
2. Sao chép lệnh - nó chứa mã thông báo đăng ký một lần (có giá trị trong 24 giờ, không sử dụng lại);
3. Chạy nó với quyền root trên nút từ xa; nó trông giống như:

   ```bash
   bash <(curl -fsSL "http://<panel-host>:<panel-port>/api/v1/agent/install?id=<node-id>&token=<one-time-token>&port=<panel-port>")
   ```

4. Đại lý tải gói xuống, đăng ký với bảng điều khiển và bắt đầu.

### 5.4 Chi tiết nút

Nhấp vào một nút để mở trang chi tiết của nó:

- **Tổng quan**: thông tin cơ bản, trạng thái Đại lý, trạng thái chứng chỉ; thẻ tài nguyên hiển thị mức sử dụng CPU/bộ nhớ/đĩa, tốc độ tăng/giảm NIC, tốc độ lưu lượng người dùng cốt lõi, lưu lượng sử dụng hàng tháng, kết nối TCP, tải hệ thống và lưu lượng tích lũy NIC đầu ra (làm mới im lặng cứ sau 10 giây)
- **Trung tâm giao thức**: cấu hình quy trình cốt lõi
- **Proxy**: Cấu hình Caddy / Nginx
- **Quy trình cốt lõi**: bắt đầu / dừng / khởi động lại
- **Lưu lượng truy cập cốt lõi**: lưu lượng truy cập người dùng được chuyển tiếp theo lõi và tổng lưu lượng hàng tháng của nó
- **WARP**: Cấu hình WARP
- **Miền trực tiếp**: quản lý miền
- **Giới hạn tỷ lệ**: định hình lưu lượng truy cập
- **Chứng chỉ**: ràng buộc chứng chỉ
- **Người dùng trực tuyến**: người dùng trực tuyến theo thời gian thực

### 5.5 Hành động của nút

- **Cấu hình đồng bộ**: đẩy cấu hình bảng điều khiển vào nút
- **Buộc kết nối lại**: hủy kết nối Tác nhân và kết nối lại
- **Cập nhật tác nhân**: đẩy bản cập nhật Tác nhân
- **Thu hồi chứng chỉ**: thu hồi chứng chỉ Đại lý của nút

---

## 6. Trung tâm giao thức

Trung tâm giao thức cấu hình các tham số giao thức cho các quy trình cốt lõi.

### 6.1 Các lõi được hỗ trợ

| Cốt lõi | Mô tả |
|------|-------------|
| Lõi Xray | VLESS / VMess / Trojan / Shadowsocks và hơn thế nữa |
| sing-box | Nền tảng proxy phổ quát mô-đun có hỗ trợ TUN; mang Hysteria2 / TUIC và các giao thức QUIC khác |

### 6.2 Mô-đun cấu hình

Các mô-đun khác nhau giữa hai lõi; bảng điều khiển hiển thị tập hợp phù hợp với lõi đã chọn:

**Lõi Xray**

- **Nhật ký**: nhật ký truy cập, nhật ký lỗi, chế độ đầu ra
- **API**: gRPC API địa chỉ nghe, danh sách dịch vụ
- **DNS**: Máy chủ DNS, chiến lược truy vấn, máy chủ tĩnh
- **Định tuyến**: quy tắc định tuyến, cân bằng tải, định tuyến VLESS
- **Inbounds / Outbounds**: giao thức, vận chuyển, TLS
- **Truyền tải**: WebSocket, gRPC, XHTTP, v.v. (TUN là giao thức gửi đến, không phải giao thức truyền tải)
- **TLS**: chứng chỉ, ALPN, dấu vân tay uTLS, ECH
- **Reality**: quản lý mục tiêu, cặp khóa, ID ngắn
- **Kiểm tra tình trạng / FakeDNS / Số liệu / Chính sách / Dữ liệu địa lý / Giới hạn phiên bản**: đầu dò, nhóm IP, Prometheus, thời gian chờ và bộ đệm, tệp tài nguyên, phạm vi phiên bản cốt lõi

**sing-box**

- **Nhật ký**: cấp độ, dấu thời gian, đầu ra
- **NTP**: đồng bộ hóa thời gian
- **DNS**: Máy chủ và quy tắc DNS
- **Định tuyến**: `final` gửi đi mặc định, **Bộ quy tắc**, **Quy tắc**, trình phân giải tên miền mặc định, chiến lược mạng và dự phòng (xem 6.6)
- **Inbounds / Outbounds**: giao thức, vận chuyển, TLS
- **Điểm cuối**: WireGuard (bao gồm cấu hình WARP), máy khách MASQUE và tương tự; một số có thể cùng tồn tại và được phân biệt bởi `tag`

> Khi bật “sử dụng tích hợp mạng hệ thống”, **name** của điểm cuối là tên giao diện kernel:
> tối đa 15 ký tự, chỉ chữ cái/chữ số/`._-`. Tên quá dài hoặc không hợp lệ sẽ tự động bị lùi lại
> đến điểm cuối `tag`. Điểm cuối mới được đặt trước cho mỗi giao thức (`wg-` / `ts-` / `oc-` / `ovpn-c-` /
> `ovpn-s-` / `mq-c-` / `mq-s-`) và các bản ghi kế thừa như `WireGuard-xxxxxx` được hiển thị chuẩn hóa
> ngay khi trình soạn thảo được mở.
- **Dịch vụ**: mảng sing-box `services` cấp cao nhất, chẳng hạn như `hysteria-realm`, DERP hoặc `resolved`
- **Chứng chỉ / Máy khách HTTP / Không gian tên mạng**: Các phần JSON bạn chỉ mở rộng khi cần
- **Thử nghiệm**: `clash_api` và tương tự; **không có gì được ghi vào cấu hình được tạo trừ khi được bật**

> sing-box 1.12 đã loại bỏ các trường định tuyến `geosite` / `geoip`; Định tuyến dựa trên địa lý hiện đã được thực hiện
> `rule_set`. Việc giữ lại các trường cũ khiến lõi không giải mã được cấu hình (`unknown field`).

### 6.3 Cấu hình gửi đến

1. Chọn lõi và giao thức;
2. Điền thông tin cơ bản:
   - Gắn thẻ
   - Nghe địa chỉ và cổng
   - Loại giao thức
3. Đặt tham số giao thức (UUID, mật khẩu, v.v.);
4. Chọn phương tiện vận chuyển (TCP, WebSocket, gRPC, v.v.);
5. Định cấu hình TLS (chứng chỉ, ALPN);
6. Lưu và áp dụng cho lõi.

> Theo mặc định, các luồng gửi đến mới nghe trên `::` (ngăn xếp kép trên Linux, xử lý cả IPv4 và IPv6).
> Chỉ thay đổi nó thành `0.0.0.0` hoặc một địa chỉ cụ thể khi bạn cần hạn chế giao diện.

Chế độ cổng “Sử dụng cổng vào” cũng cung cấp **tự động tạo các quy tắc cho phép tường lửa**: mở
trình soạn thảo gửi đến sẽ thăm dò tường lửa của nút trước tiên và khi phát hiện thấy tường lửa / ufw / nftables / iptables
bạn có thể cho phép lưu cổng theo giao thức nghe (TCP/UDP).

- nhảy cổng hy2 / hysteria cho phép cả cổng nghe và mọi phạm vi cổng động (ví dụ: `20000-30000`); nhiều phạm vi được phân tách bằng dấu phẩy;
- Phụ cấp không có giá trị bình thường: quy tắc bao trùm hiện có (phạm vi cổng / đa cổng) không được tạo lại;
- Việc thay đổi cổng sẽ thu hồi quy tắc cũ trước khi cho phép quy tắc mới; xóa thư đến, vô hiệu hóa cổng nhảy hoặc chuyển sang Unix Socket/tun (không có cổng nghe) sẽ dọn sạch nó;
- Khi không phát hiện thấy tường lửa nào được kích hoạt, chỉ có một gợi ý được hiển thị và không có quy tắc nào được viết.

### 6.3.1 Đường dẫn proxy ngược (Miền truy cập → TLS phía trước → Đường dẫn proxy ngược)

Khi đường truyền tải riêng của đường vào là `/` nhưng nginx / caddy bên ngoài gắn nó dưới một đường dẫn phụ
(ví dụ: `location /apps { proxy_pass http://127.0.0.1:8080/; }`), máy khách phải kết nối với `/apps` hoặc
yêu cầu không bao giờ đến được vị trí đó — đăng ký vẫn tải xuống nhưng nút không kết nối.
Điền **Đường dẫn proxy ngược** trong “Miền truy cập → Mặt trước TLS”:

- Nhập `/apps` hoặc `/apps?ed=2560` (có dữ liệu ban đầu); đăng ký và chia sẻ liên kết **thay thế** đường dẫn vận chuyển đến bằng nó;
- Áp dụng cho ws, httpupgrade, xhttp và h2/http; truyền tải không có đường dẫn (raw/tcp, kcp) bị bỏ qua;
- Nó độc lập với nơi TLS kết thúc: đường dẫn vẫn được áp dụng khi Front TLS tắt (đi theo lõi);
- gRPC không có khái niệm tiền tố URL (đường dẫn máy khách luôn là `/{serviceName}/…`), vì vậy đường dẫn được **sáp nhập vào serviceName**
(`/apps` + `svc` → `apps/svc`); proxy ngược phải loại bỏ tiền tố đó trong `grpc_pass`.

### 6.4 Cấu hình gửi đi

1. Thêm một chuyến đi;
2. Đặt thẻ và giao thức;
3. Đặt địa chỉ máy chủ và cổng;
4. Định cấu hình vận chuyển và TLS;
5. Lưu và áp dụng.

### 6.5 Chia sẻ liên kết

1. Nhấp vào “Chia sẻ liên kết” trong danh sách gửi đến;
2. Chọn một định dạng:
   - Chia sẻ URI
   - Mihomo YAML
   - sing-box JSON
   - Xray JSON
3. Sao chép liên kết hoặc tạo mã QR.

### 6.6 Bộ quy tắc và định tuyến sing-box

Trong “Trung tâm giao thức” → sing-box → “Định tuyến” có hai phần: **Bộ quy tắc** khai báo có thể sử dụng lại
các tệp quy tắc và **Quy tắc** quyết định các điều kiện đối sánh và dữ liệu gửi đi. Các quy tắc được đánh giá từ
từ trên xuống dưới và trận đầu tiên thắng.

#### 6.6.1 Bộ quy tắc

| trường | Mô tả |
|-------|-------------|
| gắn thẻ | Mã định danh bộ quy tắc được tham chiếu bởi Quy tắc; phải là duy nhất |
| loại | `local` đọc tệp trên nút; `remote` cho phép lõi tải xuống trên `update_interval` |
| định dạng | `binary` cho `.srs` (mặc định); `source` cho định dạng nguồn `.json` |
| Đường dẫn | Đường dẫn `local` có liên quan đến thư mục dữ liệu nút; các tệp trong `data/rules/` sử dụng `rules/xxx.srs` |
| URL | Địa chỉ tải xuống cho `remote`; `download_detour` chọn đường ra được sử dụng để tải xuống (thường là `direct`) |

Bộ quy tắc cục bộ (được khuyến nghị; trước tiên hãy tải xuống các tệp trong phần “Tài nguyên dịch vụ → Tệp quy tắc”):

```jsonc
"route": {
  "rule_set": [
    { "type": "local", "tag": "geosite-cn", "format": "binary", "path": "rules/geosite-cn.srs" },
    { "type": "local", "tag": "geoip-cn", "format": "binary", "path": "rules/geoip-cn.srs" }
  ]
}
```

> Tải xuống tệp trên bảng điều khiển là chưa đủ: trước tiên, bộ quy tắc `local` phải được đẩy lên
> các nút (“Đẩy tới tất cả các nút”) để tệp tồn tại trong `data/rules/` của nút; nếu không thì cốt lõi
> không khởi động được vì không tìm thấy tập tin.

Bộ quy tắc từ xa (nút phải có khả năng tiếp cận URL):

```jsonc
{ "type": "remote", "tag": "geosite-openai", "format": "binary",
  "url": "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-openai.srs",
  "download_detour": "direct", "update_interval": "1d" }
```

Khi bạn chọn một tệp cục bộ, thẻ được lấy từ tên tệp (`rules/geosite-cn.srs` →
`geosite-cn`); đối với bộ điều khiển từ xa, nó nằm ở cuối URL. Định dạng cũng được điền từ
phần mở rộng (`.srs` → `binary`, `.json` → `source`). Các giá trị dẫn xuất có thể được chỉnh sửa tự do và
các giá trị bạn đã gõ sẽ không bao giờ bị ghi đè.

> `download_detour` không được dùng nữa kể từ sing-box 1.14 và sẽ bị xóa trong 1.16; nó vẫn còn
> hoạt động trong 1.15. Các cấu hình mới nên sử dụng `http_clients` cấp cao nhất với
> Thay vào đó là `route.default_http_client`.

#### 6.6.2 Quy tắc

| trường | Trận đấu |
|-------|---------|
| `domain` | Tên miền chính xác |
| `domain_suffix` | Hậu tố tên miền, bao gồm cả tên miền phụ |
| `domain_keyword` | So khớp chuỗi con |
| `domain_regex` | Biểu thức chính quy |
| `ip_cidr` / `ip_is_private` | IP đích |
| `source_ip_cidr` | IP nguồn |
| `port` / `port_range` | Cảng đích |
| `network` | `tcp` / `udp` |
| `rule_set` | Thẻ của Bộ quy tắc sẽ sử dụng |
| `outbound` | Thẻ gửi đi để sử dụng (phải tồn tại trong “Outbounds” hoặc “Endpoints”) |
| `type: "logical"` | Các điều kiện kết hợp với `mode` được đặt thành `and` / `or` |

Nhiều trường trong một quy tắc được kết hợp với AND (ví dụ `network` + `port`), do đó không
Cần có trình bao bọc `logical`. Chỉ sử dụng `type: "logical"` cho các kết hợp OR / NOT. `rule_set`
phải được đặt bên trong các quy tắc phụ `rules`, không được đặt bên cạnh `rules`:

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

#### 6.6.3 Di chuyển quy tắc xray

| xray | sing-box |
|------|----------|
| Chuỗi trần (bao gồm các giá trị trần trong `domain`; khớp chuỗi con) | `"domain_keyword": ["ip.sb"]` |
| `"domain": ["a.com"]` | `"domain_keyword": ["a.com"]`; sử dụng `"domain_suffix": ["a.com"]` để khớp hậu tố thực hoặc `"domain": ["a.com"]` để khớp chính xác |
| `"domain:clstorage.net"` | `"domain_suffix": ["clstorage.net"]` |
| `"full:a.com"` | `"domain": ["a.com"]` |
| `"regexp:..."` | `"domain_regex": ["..."]` |
| `"ip": ["geoip:cn"]` | `"rule_set": ["geoip-cn"]` |
| `"domain": ["geosite:cn"]` | `"rule_set": ["geosite-cn"]` |
| `"outboundTag": "direct"` | `"outbound": "direct"` |

Ghi chú:

- **Thẻ không được chia sẻ giữa các lõi**: không thể sử dụng thẻ gửi đi xray trong quy tắc sing-box;
 sử dụng thẻ thực sự tồn tại trong sing-box “Outbounds” hoặc “Endpoint”.
- Thẻ điểm cuối hoặc thẻ gửi đi sai báo cáo `outbound not found`; báo cáo thẻ đặt quy tắc sai
 `rule-set not found`.
- Đặt quy tắc trực tiếp/danh sách cho phép lên hàng đầu; khi một quy tắc phù hợp, các quy tắc sau này sẽ không còn áp dụng nữa.

---

## 7. Quản lý proxy

Quản lý proxy định cấu hình proxy ngược Caddy hoặc Nginx.

### 7.1 Các loại proxy

- **Caddy**: HTTPS, HTTP/1.1 và HTTP/2 tự động (HTTP/3 / QUIC bị tắt để tránh xung đột UDP 443 với định tuyến lớp4)
- **Nginx**: hiệu suất cao, cấu hình linh hoạt

### 7.2 Cấu hình trang web

1. Mở “Quản lý proxy”;
2. Chọn loại proxy (Caddy / Nginx);
3. Thêm một trang web:
   - Tên miền
   - Thượng nguồn
   - Cảng địa phương
4. Lưu.

### 7.3 Trường

- **ngược dòng**: địa chỉ phụ trợ, ví dụ: `127.0.0.1:8082`
- **local_port**: cổng nghe cục bộ, ví dụ: `8443`
- **portal_forward**: quy tắc chuyển tiếp cổng thông tin
- **subscription_forward**: quy tắc chuyển tiếp đăng ký
- **Định tuyến SNI**: khi một người đến sử dụng TLS / REALITY và các máy khách luôn kết nối trên 443, bảng điều khiển sẽ xuất phát
 443 quy tắc định tuyến TCP từ ClientHello SNI (cả xray `streamSettings` và sing-box `tls.reality`);
 proxy phía trước chỉ chuyển tiếp TCP và TLS vẫn bị chấm dứt bên trong lõi

---

## 8. Tài nguyên định tuyến

Tài nguyên định tuyến thu thập tài sản được chia sẻ được sử dụng bởi đăng ký và định tuyến. Trang này có năm tab:

| Thẻ | Mục đích |
|-----|---------|
| Tên miền ưa thích Direct & CF | Nhập miền trực tiếp / IP và miền ưu tiên Cloudflare, được sử dụng khi viết lại địa chỉ truy cập trong đăng ký và các nút bên ngoài |
| Nhóm IP CF | Các nguồn IP, cổng và cài đặt thăm dò ưa thích, được sử dụng bởi “Miền truy cập” |
| Quy tắc toàn cầu WAF | Nguồn quy tắc và tệp quy tắc cho proxy ngược WAF, được đẩy tới các nút |
| Tệp quy tắc | Tải xuống, đồng bộ hóa theo lịch và đẩy các tệp quy tắc (`.srs` / `.json` / `.dat`) |
| Các nút bên ngoài | Nhập các nút bên ngoài từ v2ray URI / Base64 / đăng ký URL (xem 16.3) |

### 8.1 Tên miền ưa thích Direct & CF

- **Truy cập trực tiếp**: nhập tên miền hoặc IP, cổng 443 theo mặc định; liên kết các nút hoặc để trống cho toàn cầu;
- **Miền ưu tiên CF**: Miền ưu tiên Cloudflare cộng với cổng, có kiểm tra kết nối;
- Cả hai đều cung cấp các ứng cử viên “Miền truy cập” được sử dụng bởi các đăng ký và viết lại nút bên ngoài (xem 16.3).

### Nhóm IP 8.2 CF

- Duy trì các nguồn IP ưa thích (bao gồm các nguồn kiểu `bestproxy`), khoảng thời gian tìm nạp và cổng;
- “Cài đặt đầu dò” kiểm soát khoảng thời gian thăm dò, ngưỡng độ trễ và nút đầu vào thăm dò; thăm dò địa phương
 chỉ lọc các IP rõ ràng là không thể truy cập được, trong khi việc chọn một nút đầu vào sẽ mô phỏng đường dây của nút đó;
- “Trình quay số” có thể tự động (lõi đã cài đặt), `singbox`, `xray` hoặc `mihomo`; lõi không phải
 cài đặt không được liệt kê. Việc thăm dò chạy như một quá trình tạm thời và sẽ thoát khi hoàn tất và không bao giờ đọc hoặc
 ghi cấu hình riêng của lõi;
- Khi “Miền truy cập” sử dụng nhóm IP CF mà không có giá trị cụ thể, một mục nhập (với cổng của nó) sẽ được
 được chọn ngẫu nhiên.

### 8.3 Quy tắc chung WAF

- Định cấu hình nguồn quy tắc WAF URL và các tệp quy tắc để đồng bộ hóa;
- Sau khi đồng bộ hóa, chúng được đẩy đến các nút và được tải bởi proxy nút (Nginx / Caddy);
- Một tệp có URL đầy đủ được tải xuống từ URL đó; nếu không thì sử dụng “nguồn URL + /tên tệp”;
- “Bộ nhớ đệm trùng khớp” (`match_cache` / `config_match_cache`) được bật theo mặc định: giới hạn toàn quy trình
 bộ nhớ đệm của các kết quả so khớp quy tắc nóng giúp giảm việc truyền tải quy tắc theo yêu cầu. Nó chỉ có trên toàn cầu và không có
 ghi đè theo từng miền; tạm thời vô hiệu hóa nó để so sánh A/B khi điều tra các kết quả dương tính giả
 (vô hiệu hóa có nghĩa là đánh giá quy tắc đầy đủ cho mọi yêu cầu và mức sử dụng CPU cao hơn).

### 8.4 Tệp quy tắc (tệp quy tắc định tuyến)

Tab này chỉ tải xuống các tệp quy tắc trên bảng điều khiển và đẩy chúng đến các nút; không có gì có hiệu lực
tự động. Để sử dụng chúng trong định tuyến, hãy tham khảo chúng trong “Trung tâm giao thức → sing-box → Định tuyến”
trong Bộ quy tắc và quy tắc (xem 6.6).

- **Kho lưu trữ quy tắc**: nhập `owner/repo` (GitHub, có thể duyệt) hoặc tải xuống đầy đủ URL; theo lịch trình
 đồng bộ hóa có thể chạy 6/12 giờ một lần, hàng ngày lúc 03:00, hàng ngày lúc nửa đêm hoặc hàng tuần vào Chủ nhật;
- **Thêm tệp**: đối với các tệp đơn lẻ bên ngoài bản phát hành kho lưu trữ (ví dụ: nhánh SagerNet `.srs`
 tập tin); nhập URL đầy đủ và nó sẽ được tải xuống tự động;
- **Danh sách tệp**: hiển thị tên, kích thước, bản cập nhật gần đây nhất và nút chuyển “Tự động đồng bộ hóa”, với “Tải xuống” cho mỗi tệp;
 nút “Đẩy tới tất cả các nút” ở trên cùng sẽ đẩy mọi tệp quy tắc đến tất cả các nút;
- Các tệp được tải xuống `data/rules/` của bảng điều khiển và sau khi được đẩy, sẽ nằm dưới dữ liệu nút
 `rules/` của thư mục; tham chiếu chúng trong sing-box là `rules/<file name>`;
- Định dạng:
  - `.srs`: Bộ quy tắc nhị phân sing-box, được khuyến nghị và nhỏ nhất;
  - `.json`: bộ quy tắc định dạng nguồn; `rule_set` phải đặt `format: "source"`;
  - `.dat`: xray `geosite.dat` / `geoip.dat`, sing-box không thể sử dụng trực tiếp.

> Khối “Cách sử dụng” có thể thu gọn ở đầu trang chứa các ví dụ tham khảo, ánh xạ xray
> bảng và các lỗi thường gặp.

### 8.5 Nút bên ngoài

Nhập và quản lý các nút bên ngoài được đề cập trong [16.3 Nút bên ngoài](#163-external-nodes).

---

## 9. Trung tâm đăng ký

Trung tâm đăng ký đóng gói các nút thành các đăng ký sẵn sàng cho khách hàng: xác định **định dạng** đăng ký (nội dung sẽ phát ra, quy tắc nào, gói nào có thể nhìn thấy chúng), sau đó đăng ký **điểm cuối** (miền công cộng / đường dẫn), sau đó phân phối các liên kết.

### 9.1 Định dạng đăng ký

“Trung tâm đăng ký” → “Các hình thức đăng ký”; một hàng cho mỗi đầu ra của khách hàng:

| trường | Mô tả |
|---|---|
| Tên | Độc đáo; `?config=<name>` khớp với nó và hộp thoại khách hàng hiển thị nó |
| Loại khách hàng | clash / shadowrocket / singbox / v2ray — chỉ hiển thị nhóm |
| định dạng đầu ra | `mihomo_yaml` (Clash/Mihomo), `singbox_json`, `xray_json`, `base64_uri`, `raw_uri` |
| Loại nội dung/phần mở rộng | Tiêu đề phản hồi và tên tệp |
| Nền tảng | giành chiến thắng/mac/android/ios/phổ quát; khớp với `?platform=`, trống = phổ quát |
| kế hoạch ràng buộc | Trống = có sẵn cho mọi gói; nếu không thì chỉ những kế hoạch đó mới có thể sử dụng nó |
| Quy tắc | Quy tắc định tuyến theo định dạng (trình soạn thảo có cấu trúc cho Mihomo, JSON thô cho mỗi quy tắc cho sing-box / Xray). **Quy tắc không được chuyển đổi giữa các định dạng** |

Ghi chú:

- Một số cấu hình có thể tồn tại cho cùng một định dạng đầu ra (ví dụ: `out`, `home`); chọn một cái có `config=` trong liên kết;
- Nếu không có `config`, thứ tự là “cấu hình được liên kết với đăng ký → trận đấu `platform` → mặc định”;
- Mẫu sing-box có thể mang `route.rule_set` riêng cho máy khách, thông thường là `remote` nên
 khách hàng tải nó xuống; các tệp cục bộ được đẩy tới `rules/` của nút trong “Tệp quy tắc” chỉ khả dụng
 tới lõi trên nút đó, không phải đăng ký của khách hàng;
- Khi tạo các mẫu sing-box / Xray, bảng điều khiển sẽ loại bỏ các tham chiếu lơ lửng (`final`, `dns.final`,
 `default_domain_resolver`, `download_detour`, `rule_set` cục bộ, định nghĩa `rule_set` không hợp lệ, trống
 `balancer`, v.v.) để máy khách không bị lỗi khi khởi động với lỗi NGHIÊM TRỌNG;
- Bí danh gửi đi trong quy tắc được giải quyết dựa trên thẻ thực (`REJECT` / `blocked` → `block`, `DIRECT` →
 `direct`, không phân biệt chữ hoa chữ thường); các lệnh gửi đi không thể giải quyết được sẽ quay trở lại lệnh gửi đi mặc định và quy tắc
 giữ vị trí của nó;
- Một nhóm có `tag = proxy` luôn được tạo: `selector` cho sing-box và bộ cân bằng `random` cho
 Xray và `outbound: proxy` trong các quy tắc được viết lại thành `balancerTag` phù hợp. Trình chỉnh sửa quy tắc tải
 các ứng viên gửi đi một cách linh hoạt từ “nhóm proxy + các lượt gửi đi tùy chỉnh” và các thẻ được nhập theo cách thủ công là
 vẫn được xác nhận;
- Cấu hình sing-box / Xray phải được tạo riêng; nếu chúng không tồn tại thì chúng sẽ không xuất hiện trong liên kết khách hàng (xem 10.3).

### 9.2 Điểm cuối đăng ký

“Trung tâm đăng ký” → “Điểm cuối đăng ký”; một hàng cho mỗi địa chỉ công cộng:

| trường | Mô tả |
|---|---|
| Tên miền | Miền đăng ký công khai |
| Đường dẫn | Tiền tố trong miền, mặc định `/` |
| Đã bật | Điểm cuối bị vô hiệu hóa sẽ bị bỏ qua khi xây dựng liên kết |
| Mô tả | Ghi chú văn bản tự do |

- **Nhiều hàng (đa miền)** được hỗ trợ; khi một số miền được bật, miền của nhà cung cấp sẽ tuân theo miền được yêu cầu hiện tại sử dụng (xem 9.4);
- Cổng đăng ký chuyên dụng phục vụ ở thư mục gốc (`/<token>`, không có `/sub`); khi chia sẻ cổng bảng điều khiển thì đó là `/sub/<token>`;
- Proxy ngược phải chuyển tiếp `X-Forwarded-Proto` / `X-Forwarded-Host`, nếu không thì điểm cuối được kích hoạt đầu tiên sẽ được sử dụng. Các mẫu trang web Caddy / Nginx do bảng điều khiển tạo ra bao gồm cả hai tiêu đề; cấu hình proxy được duy trì bằng tay cần được thêm chúng theo cách thủ công.

### 9.3 Liên kết đăng ký

Dạng cơ sở: `https://<sub-domain>/<token>` (cổng bảng điều khiển: `https://<panel-domain>/sub/<token>`)

| tham số | Mô tả |
|---|---|
| `format` | Định dạng đầu ra; bí danh: `clash` → mihomo_yaml, `singbox`/`sing-box` → singbox_json, `xray` → xray_json, `raw` → raw_uri, `base64`/`v2ray` → base64_uri |
| `config` | Áp dụng định dạng đăng ký theo tên; phải khớp với `format`, nếu không thì bỏ qua |
| `platform` | Khớp cấu hình theo nền tảng (khi bỏ qua `config`) |
| `providers` | `1` / `true`: Chế độ nhà cung cấp Clash (`proxy-providers` + `use`); nút nội tuyến mặc định |

Ví dụ:

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

### 9.4 Chế độ đa miền và nhà cung cấp

Với `providers=1`, mọi mục nhập `proxy-providers` đều quay trở lại điểm cuối đăng ký. Lựa chọn tên miền:

1. **Miền mà yêu cầu này đã sử dụng** (giá trị đầu tiên `X-Forwarded-Host`, cổng bị loại bỏ, nếu không thì là `Host`) nếu nó khớp với điểm cuối **được bật**;
2. Nếu không thì điểm cuối được bật có **ID thấp nhất**;
3. Đường dẫn xuất phát từ điểm cuối đó: `https://<domain><path>/<token>?format=mihomo_proxies_yaml...`.

Vì vậy, việc thay đổi miền không cần thay đổi bảng điều khiển: các máy khách tìm nạp qua miền mới sẽ nhận được miền mới bên trong nhà cung cấp và miền cũ được kích hoạt không còn được mã hóa cứng nữa. Nếu máy khách đã lưu vào bộ đệm của nhà cung cấp cũ URL, hãy cập nhật đăng ký (hoặc xóa bộ đệm của nhà cung cấp/khởi động lại máy khách).

Ghi chú bổ sung:

- `proxy-providers` bên trong định dạng dùng chung (`config_json`) bị bỏ qua (tránh rò rỉ nhiều kế hoạch); các nhà cung cấp đến từ các nhóm nút kế hoạch và các nguồn bên ngoài;
- Đường dẫn bộ đệm của nhà cung cấp nội bộ và bên ngoài luôn được lấy từ “mặc định của nhà cung cấp nội bộ → mẫu đường dẫn bộ đệm” (`./providers/{key}.yaml` mặc định); các nguồn bên ngoài không thể tự thiết lập;
- Nhà cung cấp dành riêng cũ `subscription-nodes` không còn được phát hành nữa: các nhóm sử dụng "tất cả các nút" sẽ mở rộng sang các nhà cung cấp cụ thể hoặc các nút nội tuyến khi không có gói nào.

---

## 10. Quản lý khách hàng

Quản lý khách hàng xử lý người dùng cuối.

### 10.1 Thêm khách hàng

1. Mở “Quản lý khách hàng”;
2. Nhấp vào “Thêm khách hàng”;
3. Điền vào:
   - Email
   - Mật khẩu
   - kế hoạch
   - Hạn ngạch giao thông
   - Thời gian hết hạn
4. Cứu.

### 10.2 Hành động của khách hàng

- **Chỉnh sửa**: thay đổi thông tin khách hàng
- **Tạm dừng**: tạm dừng dịch vụ
- **Tiếp tục**: khôi phục khách hàng bị đình chỉ
- **Đặt lại lưu lượng truy cập**: đặt lại lưu lượng truy cập đã sử dụng
- **Reset mật khẩu**: đặt lại mật khẩu của khách hàng
- **Gia hạn**: gia hạn thời hạn sử dụng
- **Nâng cấp/hạ cấp**: thay đổi gói
- **Xóa**: xóa khách hàng

### 10.3 Chi tiết đăng ký

Quản lý khách hàng → cột “Đăng ký” → “Chi tiết”. Hộp thoại liệt kê các liên kết cho mọi định dạng có sẵn cho gói của khách hàng:

- **Clash / Mihomo**: hai hàng — các nút nội tuyến và `providers=1` (nhà cung cấp proxy), cả hai đều mang `config=<name>`;
- **Sing-box / Xray**: chỉ hiển thị khi cấu hình có định dạng đó tồn tại và hiển thị với gói; liên kết mang `config=<name>`;
- **Base64 URI / Raw URI**: luôn hiển thị (không có `config`);
- Mỗi hàng có nút sao chép riêng; nhấp vào một hàng sẽ chuyển mã QR bên dưới sang định dạng đó.

Vì vậy, chỉ với cấu hình Clash, hộp thoại sẽ hiển thị Clash + Base64 + Raw; Các liên kết sing-box / Xray chỉ xuất hiện sau khi cấu hình của chúng được tạo.

Hộp thoại tương tự cũng hiển thị mức sử dụng lưu lượng truy cập, lịch sử đặt hàng và lịch sử giao dịch.

---

## 11. Quản lý kế hoạch

Quản lý kế hoạch định cấu hình các kế hoạch bạn bán.

### 11.1 Mẫu kế hoạch

1. Mở “Quản lý kế hoạch” → “Mẫu kế hoạch”;
2. Thêm một mẫu:
   - Tên mẫu
   - Danh mục (tiêu chuẩn/cao cấp/doanh nghiệp/dùng thử)
   - Tự động gia hạn
   - Cài đặt dùng thử
3. Lưu.

### 11.2 Các phương án kế hoạch

1. Thêm một biến thể theo mẫu:
   - Tên biến thể (ví dụ: hàng tháng, hàng năm)
   - Giá (tính bằng xu)
   - Thời lượng (ngày)
   - Lưu lượng truy cập (GB, `0` = không giới hạn)
   - Giới hạn đồng thời
   - Giới hạn tốc độ (Mbps)
2. Lưu.

### 11.3 Tiện ích giao thông

1. Mở “Tiện ích bổ sung giao thông”;
2. Thêm một tiện ích bổ sung:
   - Tên
   - Giá
   - Giao thông
   - Chế độ hết hạn (gia hạn ngày / giống như kế hoạch)
3. Lưu.

### 11.4 Phiếu giảm giá

1. Mở “Phiếu giảm giá”;
2. Thêm phiếu giảm giá:
   - Mã
   - Loại (tỷ lệ phần trăm / số tiền cố định)
   - Giá trị chiết khấu
   - Giới hạn sử dụng
   - Thời hạn hiệu lực
   - Kế hoạch áp dụng
3. Lưu.

---

## 12. Quản lý nạp tiền

Quản lý nạp tiền xử lý các trang nạp tiền, đơn đặt hàng và thẻ nạp tiền.

> **Tích hợp thanh toán**
>
> Bảng điều khiển **không tích hợp trực tiếp với các tổ chức thanh toán** (Alipay, WeChat Pay, Stripe, ngân hàng, v.v.). Nó chỉ kết nối với **nền tảng thanh toán trung gian**, sau đó tích hợp với tổ chức thanh toán tương ứng, xử lý hợp đồng, mua và thanh toán, đồng thời báo cáo kết quả thanh toán qua API](api/charge-callback.md) gọi lại [charge.
>
> Nền tảng thanh toán trung gian **không phải là một phần của dự án này; các nhà khai thác phải tự phát triển nó (hoặc sử dụng hệ thống của riêng họ)**. Sau khi triển khai, hãy đăng ký nó trong “Trang web nạp tiền” và hoàn tất xác minh tên miền để sử dụng nó làm điểm nạp tiền; trang nạp tiền ở đây là nền tảng trung gian đó, không phải bản thân tổ chức thanh toán. Điều kiện thanh toán, phí, quyết toán và tuân thủ được nhà điều hành, tổ chức thanh toán và nền tảng trung gian đồng ý và chịu trách nhiệm.

### 12.1 Địa điểm nạp tiền

1. Mở “Quản lý nạp tiền” → “Trang web nạp tiền”;
2. Thêm một trang web:
   - Tên trang web
   - Các phương thức thanh toán được hỗ trợ bởi nền tảng trung gian đó (Alipay, WeChat, USDT, Stripe, v.v., được cung cấp thông qua việc tích hợp với các tổ chức thanh toán)
   - Tiền tệ
   - Tỷ giá hối đoái
3. Lưu;
4. Xác minh trang web (tệp HTTP hoặc xác minh DNS).

### 12.2 Lệnh nạp tiền

- Xem tất cả các đơn đặt hàng nạp tiền
- Lọc theo trạng thái (đang chờ xử lý, đã thanh toán, không thành công, đã hết hạn, đã hoàn tiền)
- Xem chi tiết đơn hàng
- Vấn đề hoàn tiền của quản trị viên

### 12.3 Thẻ nạp tiền

1. Mở “Thẻ nạp tiền”;
2. Thêm thẻ:
   - Loại (số lượng, kế hoạch, bổ sung lưu lượng truy cập)
   - Mệnh giá/kế hoạch
   - Thời gian hết hạn
3. Hoặc tạo hàng loạt:
   - Số lượng (lên tới 1000)
   - Tiền tố số thẻ
   - Định dạng số thẻ
4. Xuất dưới dạng CSV.

### 12.4 Quản lý số dư

- Xem số dư của khách hàng
- Điều chỉnh số dư (tăng/giảm/đặt)
- Điều chỉnh số lượng đông lạnh
- Xem lịch sử giao dịch

### 12.5 Đại lý

- Thêm đại lý
- Đặt tỷ lệ hoa hồng
- Xem hoa hồng tích lũy
- Rút tiền hoa hồng

---

## 13. Quản lý vé

Quản lý vé xử lý các yêu cầu hỗ trợ của khách hàng.

### 13.1 Danh sách vé

- Xem tất cả vé
- Lọc theo trạng thái (mở, trả lời, đóng)
- Lọc theo danh mục (kỹ thuật, thanh toán, tài khoản, khác)
- Lọc theo mức độ ưu tiên (thấp, bình thường, cao, khẩn cấp)

### 13.2 Hành động về vé

- **Xem / trả lời**: mở ticket và trả lời
- **Chỉ định**: chỉ định một vé cho quản trị viên
- **Đóng**: đóng một phiếu đã được giải quyết

---

## 14. Quản lý chứng chỉ

Quản lý chứng chỉ xử lý các chứng chỉ TLS.

### 14.1 Cấp giấy chứng nhận

1. Mở “Quản lý chứng chỉ”;
2. Nhấp vào “Cấp chứng chỉ”;
3. Điền vào:
   - Danh sách tên miền (mỗi dòng một danh sách)
   - Loại chứng chỉ (tên miền/IP)
   - Loại khóa (ECC / RSA)
   - Phương thức xác thực (DNS-01 / HTTP-01)
   - Nhà cung cấp DNS
   - Vị trí phát hành (máy chủ/nút của bảng điều khiển)
4. Nhấp vào “Vấn đề”.

### 14.2 Hành động chứng nhận

- **Gia hạn**: gia hạn thủ công
- **Triển khai**: triển khai tới một nút
- **Phân phối**: đẩy tới tất cả các nút được liên kết
- **Di chuyển**: di chuyển các tham chiếu từ chứng chỉ cũ sang chứng chỉ mới
- **Tải xuống**: tải xuống `cert` / `fullchain` / `privkey`

### 14.3 Tải lên chứng chỉ

1. Nhấp vào “Tải lên chứng chỉ”;
2. Chọn phương pháp:
   - Dán nội dung
   - Tải tập tin lên
3. Tải lên chuỗi chứng chỉ và khóa riêng;
4. Cứu.

### Cấu hình 14.4 EAB

- Quản lý thông tin xác thực EAB cho Google CA công khai / ZeroSSL
- Tự động lấy EAB
- Đính kèm nó tự động khi phát hành

---

## 15. Hạn ngạch giao thông

Hạn ngạch lưu lượng truy cập giám sát lưu lượng truy cập hàng tháng của mỗi nút.

### 15.1 Danh sách hạn ngạch

- Xem cài đặt hạn ngạch cho tất cả các nút
- Hiển thị kích thước hạn ngạch, lưu lượng đã sử dụng và phụ cấp còn lại
- Hiển thị phần trăm sử dụng

### 15.2 Chu trình sử dụng

- Xem chi tiết sử dụng chu kỳ trên mỗi nút
- Hiển thị lưu lượng truy cập vào / ra
- Hiển thị thời gian bắt đầu/kết thúc chu kỳ

### 15.3 Nhật ký đặt lại

- Xem hồ sơ thiết lập lại giao thông
- Hiển thị chu kỳ thiết lập lại, nguồn kích hoạt và toán tử

---

## 16. Proxy chuỗi

Proxy chuỗi cấu hình liên kết nhiều lối thoát.

### 16.1 Tạo liên kết

1. Mở “Proxy được xâu chuỗi”;
2. Nhấp vào “Liên kết mới”;
3. Chọn nút nhập;
4. Chọn mục nhập vào;
5. Chọn nút thoát (cho phép nhiều nút);
6. Chọn một lối vào cho mỗi lối ra;
7. Đặt mức độ ưu tiên/trọng lượng;
8. Xác nhận.

### 16.2 Quản lý liên kết

- Xem tổng quan về cấu trúc liên kết
- Chỉnh sửa liên kết (mức độ ưu tiên, trọng lượng, thẻ gửi đi)
- Xóa một liên kết
- Kiểm tra tình trạng sức khỏe

### 16.3 Các nút bên ngoài

- Nhập các nút bên ngoài (v2ray URI, Base64 URI, đăng ký URL)
- Đăng ký các mục bên ngoài
- Quản lý các nút bên ngoài (chỉnh sửa/xóa riêng lẻ)
- Miền truy cập: định cấu hình IP ưa thích / tăng tốc CDN cho mỗi nút (miền trực tiếp, ưu tiên CF
 tên miền, nhóm IP CF, tùy chỉnh). Chỉ máy chủ/cổng được phân phối trong đăng ký mới được viết lại;
 SNI/Máy chủ vẫn giữ nguyên như đã nhập. Khi bị tắt, địa chỉ/cổng đã nhập sẽ được sử dụng
- Khi "Miền ưa thích CF / Nhóm IP CF" được chọn mà không có giá trị cụ thể, một mục nhập sẽ được chọn
 ngẫu nhiên từ nhóm (với cổng của nó); "trực tiếp / tùy chỉnh" yêu cầu giá trị rõ ràng, nếu không
 nó quay trở lại địa chỉ của nút đó

---

## 17. Người dùng trực tuyến

Người dùng trực tuyến hiển thị trạng thái trực tuyến theo thời gian thực.

### 17.1 Nó thể hiện điều gì

- Đăng ký trực tuyến
- Thiết bị / IP trực tuyến
- Nút, IP và thời gian kết nối cho mỗi thuê bao
- Tốc độ đường lên / đường xuống thời gian thực
- Tổng số đường lên / tổng số đường xuống / tổng lưu lượng truy cập

> Tổng số đường lên, tổng số đường xuống và tổng lưu lượng truy cập tích lũy qua **phiên trực tuyến liên tục**:
> kết nối lại trong thời gian ngắn, khởi động lại lõi và đặt lại bộ đếm không xóa chúng. Chỉ sau 60
> giây mà không có báo cáo mới thì phiên mới sẽ bắt đầu từ số 0. Đây là một biện pháp khác
> từ bộ đếm "lưu lượng truy cập kể từ lần bắt đầu này" của chính lõi và phản ánh tốt hơn những gì người dùng thực sự
> được sử dụng trong phiên hiện tại.

### 17.2 Lọc

- Lọc theo nút
- Tìm kiếm theo email/khách hàng/node/IP

---

## 18. Quản lý người dùng

Quản lý người dùng xử lý người dùng bảng điều khiển.

### 18.1 Vai trò

| Vai trò | Mô tả |
|------|-------------|
| Quản trị viên cấp cao (`superAdmin`) | Đặc quyền cao nhất |
| Quản trị viên (`admin`) | Đặc quyền quản lý |
| Toán tử (`operator`) | Đặc quyền hoạt động |
| Trình xem (`viewer`) | Chỉ đọc |

### 18.2 Thêm người dùng

1. Mở “Quản lý người dùng”;
2. Nhấp vào “Thêm người dùng”;
3. Điền vào:
   - Tên người dùng
   - Email
   - Mật khẩu
   - Vai trò
4. Lưu.

### 18.3 Hành động của người dùng

- **Chỉnh sửa**: thay đổi chi tiết người dùng
- **Xóa**: xóa người dùng

---

## 19. Trang web cổng thông tin

Trang cổng thông tin cấu hình cổng thông tin khách hàng.

### 19.1 Tổng quát

- Tên trang web
- Mô tả trang web
- Mở đăng ký
- Đường dẫn cổng thông tin
- Cổng cổng thông tin

### 19.2 Ngoại hình

- Tiêu đề trang web
- Logo URL
- Màu chính, màu nền, văn bản và màu nhấn
- Phông chữ

### 19.3 Điều hướng

- Thêm trang điều hướng
- Định cấu hình tên trang, biểu tượng và liên kết bên ngoài
- Quản lý các mục điều hướng

### 19.4 Mô-đun

- Thêm/chỉnh sửa mô-đun
- Định cấu hình tiêu đề, mô tả và biểu tượng mô-đun
- Hỗ trợ Markdown, văn bản đa dạng thức và HTML tùy chỉnh

### 19.5 Ngôn ngữ

- Thêm ngôn ngữ
- Quản lý bản sao
- Xuất nhập khẩu JSON

### 19.6 Mẫu

- Chuyển đổi mẫu
- Tùy chỉnh HTML/CSS/JS
- Đặt lại về mẫu mặc định

---

## 20. Cài đặt hệ thống

Cài đặt hệ thống cấu hình các thông số bảng điều khiển.

### Bảng 20.1

- Nghe địa chỉ và cổng
- Thư mục dữ liệu
- Múi giờ
- Cổng đăng ký
- Giới hạn xuất khẩu
- Mở đăng ký
- Chế độ bảng điều khiển (đầy đủ / chỉ quản lý)

### 20.2 Bảo mật

- Quản lý khóa mã hóa
- Quản lý khóa JWT
- danh sách đen IP
- Các miền được liệt kê cho phép Passkey

### 20.3 OAuth

- GitHub OAuth
- Google OAuth
- Gọi lại cấu hình URL

### 20.4 Thông báo

- Thông báo qua email (SMTP)
- Thông báo Telegram
- Mẫu thông báo
- Cài đặt giám sát

### 20.5 Sao lưu và khôi phục

- Tạo bản sao lưu
- Danh sách dự phòng
- Tải xuống bản sao lưu
- Khôi phục bản sao lưu
- Sao lưu theo lịch trình
- Lưu trữ từ xa (FTP / S3 / WebDAV)

### 20.6 Tự động bỏ chặn

- Bật tự động bỏ chặn
- Thời gian hồi chiêu
- Trạng thái áp dụng

---

## 21. Bảng điều khiển hoạt động

Bảng điều khiển hoạt động quản lý ảnh chụp nhanh cấu hình và áp dụng công việc.

### 21.1 Ảnh chụp nhanh cấu hình

- Tạo ảnh chụp nhanh
- Liệt kê ảnh chụp nhanh
- Quay lại ảnh chụp nhanh
- Xóa ảnh chụp nhanh

### 21.2 Nộp đơn xin việc

- Xem công việc gần đây
- Áp dụng cấu hình ngay bây giờ (DB → thời gian chạy)
- Kiểm tra trước (xác thực xung đột cổng và cấu hình JSON)
- Hủy công việc
- Thử lại công việc

### 21.3 Xác thực nút

- Chọn một nút và xác nhận
- Xem kết quả xác thực
- Xác thực tất cả các nút

### 21.4 Dừng khẩn cấp

- Dừng khẩn cấp proxy (ghi cấu hình gửi đến trống)
- Dừng ngay lập tức tất cả lưu lượng truy cập vào

---

## 22. Giám sát cơ sở dữ liệu

Giám sát cơ sở dữ liệu cho thấy tình trạng cơ sở dữ liệu.

### 22.1 Nó thể hiện điều gì

- Công cụ cơ sở dữ liệu
- Kích thước cơ sở dữ liệu
- Kết nối
- Độ trễ
- Trạng thái nhóm kết nối
- Thống kê bộ đệm
- Thống kê truy vấn

### 22.2 Động cơ được hỗ trợ

- SQLite
- MySQL
- PostgreSQL

---

## 23. Nhật ký kiểm tra

Nhật ký kiểm tra ghi lại các hoạt động.

### 23.1 Nó thể hiện điều gì

- Danh mục
- Loại sự kiện
- kết quả
- IP nguồn
- người dùng
- Chi tiết

### 23.2 Nhật ký hoạt động của nút

- Xem hồ sơ hoạt động của nút
- Lọc theo nút

---

## 24. Quản lý nhật ký

Quản lý nhật ký hiển thị nhật ký hệ thống.

### 24.1 Các loại nhật ký

- Nhật ký bảng điều khiển
- Nhật ký nút
- Nhật ký cốt lõi
- Nhật ký proxy

### 24.2 Hành động

- Xem nhật ký
- Tải xuống nhật ký
- Nhật ký sạch

---

## 25. Tiện ích mở rộng hoạt động

“Tiện ích mở rộng hoạt động” (`/client-ext`) định cấu hình tiếp thị giới thiệu, hoa hồng và rút tiền;
“Tài liệu pháp lý” (`/legal-documents`) duy trì các thỏa thuận phía cổng thông tin.

### 25.1 Giới thiệu và hoa hồng

- **Cấu hình giảm giá**: tỷ lệ giảm giá (phần trăm của khoản thanh toán đầu tiên của người được mời, `0` = bị vô hiệu hóa), chế độ hoa hồng (chỉ mua lần đầu / tất cả các giao dịch mua), số tiền rút tối thiểu (số dư hoa hồng phải đạt đến mức đó trước khi có thể yêu cầu rút tiền)
- **Mã mời**: khi được bật, việc đăng ký sẽ yêu cầu mã mời; đặt độ dài mã, ngày hết hạn và số lần sử dụng tối đa (`0` = không giới hạn) và tạo/xem danh sách mã
- **Xem xét rút tiền**: lọc theo trạng thái chờ xử lý/được phê duyệt/bị từ chối; thêm ghi chú xử lý và ghi lại phương thức thanh toán và địa chỉ ví
- **Hồ sơ giới thiệu**: người mời, người được mời, số tiền hoa hồng và trạng thái hoa hồng

### 25.2 Văn bản pháp luật

- Duy trì riêng “Chính sách quyền riêng tư” và “Điều khoản dịch vụ”, một tài liệu Markdown cho mỗi ngôn ngữ, có bản xem trước trực tiếp;
- Mỗi tài liệu có thể sử dụng URL bên ngoài thay vì trang tích hợp sẵn; khi trống, trang tích hợp URL được sử dụng (có thể sao chép trên trang);
- Cổng thông tin hiển thị phiên bản cho ngôn ngữ hiện tại và quay lại ngôn ngữ mặc định khi bị thiếu.

---

## 26. Câu hỏi thường gặp

### 26.1 Bảng điều khiển không khởi động

- Kiểm tra xem cổng đã được sử dụng chưa
- Kiểm tra xem `.env` có hợp lệ không
- Kiểm tra nhật ký: `journalctl -u corefusion -f`; không có systemd, `tail -f <install-dir>/data/logs/panel.log`

### 26.2 Nút ngoại tuyến

- Kiểm tra xem Agent có đang chạy không: `systemctl status corefusion-agent`
- Kiểm tra kết nối mạng
- Kiểm tra quy tắc tường lửa (cổng `18443`)

### 26.3 Cấp chứng chỉ không thành công

- Kiểm tra độ phân giải DNS
- Kiểm tra xem cổng 80 có thể truy cập được không
- Kiểm tra cấu hình nhà cung cấp DNS
- Kiểm tra nhật ký chứng chỉ

### 26.4 Đăng ký không hoạt động

- Kiểm tra liên kết đăng ký
- Kiểm tra xem nút có trực tuyến không
- Kiểm tra xem quy trình cốt lõi có đang chạy không
- Kiểm tra quy tắc tường lửa

### 26.5 Gọi lại nạp tiền không thành công

- Kiểm tra cuộc gọi lại URL
- Kiểm tra xem khóa trang web có khớp không
- Kiểm tra danh sách IP cho phép
- Kiểm tra nhật ký đơn hàng

### 26.6 Trang cổng thông tin không thể truy cập được

- Kiểm tra xem cổng cổng có mở không
- Kiểm tra cấu hình đường dẫn cổng thông tin
- **Quan trọng**: đảm bảo proxy ngược chuyển tiếp miền cổng thông tin tới **cổng công cụ cổng thông tin**, không phải cổng bảng điều khiển chính — việc chuyển tiếp đến cổng sai sẽ hiển thị một trang trống hoặc bảng điều khiển thay vì cổng thông tin. Kiểm tra `upstream` / `local_port` trong bản ghi trang proxy.
- Mở trực tiếp `http://<portal-domain>:<portal-port>` để xác minh chính công cụ cổng thông tin
- Kiểm tra cấu hình proxy ngược
- Kiểm tra nhật ký cổng thông tin

---

## Phụ lục

### A. Cổng mặc định

| Cảng | Mục đích |
|------|---------|
| Được chỉ định bởi trình hướng dẫn thiết lập (cổng trống ngẫu nhiên theo mặc định) | Bảng quản trị (có thể thay đổi) |
| 18443 | Đại lý gRPC (có thể thay đổi) |
| 0 (bị tắt theo mặc định) | Cổng công cụ đăng ký chuyên dụng (trống hoặc `0` chia sẻ cổng chính) |
| 0 (bị tắt theo mặc định) | Cổng công cụ cổng thông tin chuyên dụng (trống hoặc `0` chia sẻ cổng chính) |
| 80 / 443 | Proxy ngược (Caddy / Nginx) HTTP / HTTPS |

### B. Đường dẫn mặc định

| Đường dẫn | Mục đích |
|------|---------|
| `/` | Bảng quản trị (cổng thông tin cũng tồn tại ở `/` khi không có công cụ cổng thông tin chuyên dụng nào được định cấu hình) |
| `/sub/:token` | Tìm nạp đăng ký (khi không có cổng đăng ký chuyên dụng nào được định cấu hình) |
| `/p` | Trang cổng thông tin (khi không có cổng cổng chuyên dụng nào được định cấu hình) |
| `/:token` | Tìm nạp đăng ký (với cổng đăng ký chuyên dụng, không có tiền tố `/sub`) |

### C. Công cụ dòng lệnh

Bảng nhị phân hỗ trợ các lệnh con này (không có đối số nào khởi động máy chủ):

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

> Với systemd, hãy sử dụng `systemctl restart corefusion` và `journalctl -u corefusion -f`; không có
> systemd (thùng chứa và tương tự), hãy dừng quy trình cũ và bắt đầu lại với `nohup`, nhật ký sẽ được lưu
> trong `data/logs/panel.log`.

---

## Thay đổi v0.13.0-rc.1

- **Nâng cấp bảng điều khiển trực tuyến**: «Cài đặt hệ thống → Cập nhật bảng điều khiển» —
  chọn phiên bản (`pre` = bản phát hành trước), cập nhật, xem lịch sử và quay lại tệp nhị phân
  trước đó.
- **Nâng cấp Agent trực tuyến**: trong chi tiết nút, «Cập nhật Agent» từ tệp nhị phân cục bộ
  của bảng điều khiển hoặc GitHub Releases; nút được nâng cấp từ phiên bản cũ sẽ tự nhận
  `update.public_key` (0600) khi kết nối lần đầu và kiểm tra chữ ký ed25519.
- **Đồng bộ WAF**: thời điểm đồng bộ cuối được lưu trong `waf.github_source.last_sync`; khởi
  động lại không đặt lại bộ đếm; `0 3 * * *` chạy lúc 03:00 theo `panel.timezone` (mặc định
  Asia/Shanghai); các lần bị lỡ được bù khoảng 15 giây sau khi khởi động.
- **Danh sách trắng WAF**: tên miền chuyển tiếp của bảng điều khiển tự duy trì
  `whiteurl.rule`; kiểm tra URL/tham số/POST chỉ được bỏ qua cho API quản trị, nên lưu trang
  thanh toán với `http://127.0.0.1:18081` không còn bị chặn bởi quy tắc SSRF.
- **Chính sách hạn mức**: cộng dồn khi gia hạn / xóa khi hết hạn / đặt lại khi gia hạn (biến
  thể có thể kế thừa gói); chế độ «hàng tháng» kiểm tra mỗi giờ và đưa mức dùng về 0.
- **Trang thanh toán**: bật/tắt không thay đổi khóa hay phương thức thanh toán; đơn nạp tiền
  lọc theo loại và xuất CSV.
- **Cổng thông tin**: «Nạp tiền» hỗ trợ thanh toán trực tuyến qua trang đang bật và thẻ cào;
  nếu không có trang nào thì chỉ hiện đổi thẻ.

---

> **Phiên bản tài liệu**: v1.2
> **Cập nhật lần cuối**: 2026-11-20
> **Áp dụng cho**: CoreFusion v0.13.1
