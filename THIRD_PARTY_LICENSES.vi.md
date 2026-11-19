<!-- source: THIRD_PARTY_LICENSES.md | locale: vi | updated: 2026-11-19 -->
# Thành phần và giấy phép của bên thứ ba

> Ngôn ngữ: [简体中文](THIRD_PARTY_LICENSES.md) · [English](THIRD_PARTY_LICENSES.en.md) · [繁體中文](THIRD_PARTY_LICENSES.zh-TW.md) · [Русский](THIRD_PARTY_LICENSES.ru.md) · [日本語](THIRD_PARTY_LICENSES.ja.md) · [한국어](THIRD_PARTY_LICENSES.ko.md) · [Deutsch](THIRD_PARTY_LICENSES.de.md) · [Français](THIRD_PARTY_LICENSES.fr.md) · [Español](THIRD_PARTY_LICENSES.es.md) · [Português](THIRD_PARTY_LICENSES.pt.md) · [Türkçe](THIRD_PARTY_LICENSES.tr.md) · [Bahasa Indonesia](THIRD_PARTY_LICENSES.id.md) · [ไทย](THIRD_PARTY_LICENSES.th.md) · Tiếng Việt · [العربية](THIRD_PARTY_LICENSES.ar.md) · [فارسی](THIRD_PARTY_LICENSES.fa.md)

CoreFusion sử dụng các thành phần nguồn mở sau. Tệp này liệt kê các thành phần và giấy phép chính; **danh sách đầy đủ được xác định bởi `go.mod` / `go.sum`, `web/admin/package.json` và tệp LICENSE riêng của mỗi mô-đun**.

Khi phân phối các tệp nhị phân của dự án này, hãy giữ tệp này, dự án `LICENSE` và thông báo bản quyền/giấy phép của tất cả các thành phần bên thứ ba.

## 1. Đi phụ thuộc được biên dịch thành nhị phân (chính)

| thành phần | Giấy phép | Mục đích |
|---|---|---|
| `github.com/xtls/xray-core` | MPL-2.0 | Phân tích cú pháp cấu hình Xray, lệnh gRPC, định nghĩa giao thức |
| `github.com/xtls/reality` | MPL-2.0 | Hỗ trợ giao thức REALITY |
| `github.com/sagernet/sing` | **GPL-3.0-or-later** | Liên kết gián tiếp qua lõi xray |
| `github.com/sagernet/sing-shadowsocks` | **GPL-3.0-or-later** | Liên kết gián tiếp qua lõi xray (Shadowsocks-2022) |
| `github.com/go-sql-driver/mysql` | MPL-2.0 | Trình điều khiển MySQL |
| `github.com/hashicorp/golang-lru/v2` | MPL-2.0 | Bộ đệm LRU |
| `github.com/go-acme/lego/v5` | MIT | Cấp chứng chỉ ACME |
| `github.com/gin-gonic/gin` | MIT | Khung HTTP |
| `gorm.io/gorm` | MIT | ORM |
| `gorm.io/driver/mysql` | MIT | Trình điều khiển MySQL cho GORM |
| `gorm.io/driver/postgres` | MIT | Trình điều khiển PostgreSQL cho GORM |
| `github.com/jackc/pgx/v5` | MIT | Trình điều khiển PostgreSQL |
| `github.com/qist/sqlite` | MIT | Trình điều khiển SQLite (cơ sở dữ liệu mặc định cho bảng điều khiển/Tác nhân) |
| `github.com/aws/aws-sdk-go-v2` | Apache-2.0 | Bản sao lưu S3, Route53 DNS-01 |
| `google.golang.org/grpc` | Apache-2.0 | Giao tiếp gRPC của đại lý |
| `google.golang.org/protobuf` | BSD-3-Clause | Thời gian chạy Protobuf |
| `golang.org/x/crypto` | BSD-3-Clause | Mật mã nguyên thủy |
| `golang.org/x/oauth2` | BSD-3-Clause | OAuth2 / Google EAB |
| `golang.org/x/sys` | BSD-3-Clause | Cuộc gọi hệ thống (giới hạn tốc độ nft/tc, quản lý quy trình) |
| `github.com/golang-jwt/jwt/v5` | MIT | JWT |
| `github.com/google/uuid` | BSD-3-Clause | UUID |
| `github.com/go-webauthn/webauthn` | BSD-3-Clause | Đăng nhập Passkey / WebAuthn |
| `github.com/pquerna/otp` | Apache-2.0 | Xác thực hai yếu tố TOTP |
| `github.com/jlaffaye/ftp` | ISC | Tải lên bản sao lưu FTP |
| `gopkg.in/yaml.v3` | MIT / Apache-2.0 | Phân tích cấu hình YAML |

> ⚠ **Thành phần GPL-3.0**: `sing` và `sing-shadowsocks` là GPL-3.0 trở lên. Chúng được liên kết tĩnh vào bảng điều khiển và các tệp nhị phân Tác nhân, vì vậy **việc phân phối các tệp nhị phân yêu cầu cung cấp Nguồn tương ứng** (xem phần 3 của [LEGAL.md](./LEGAL.md)).

## 2. Phần phụ thuộc của giao diện người dùng (được biên dịch thành tệp nhị phân của bảng điều khiển)

Giao diện người dùng quản trị được xây dựng bằng Vite và được nhúng vào bảng nhị phân thông qua `go:embed`, do đó, các thành phần sau cũng được phân phối cùng với tệp nhị phân:

| thành phần | Giấy phép | Mục đích |
|---|---|---|
| Vue 3 / Vue Router / Pinia / Element Plus / axios / vue-i18n / marked / js-yaml / qrcode / Monaco Editor | MIT | Giao diện người dùng quản trị, định tuyến và trạng thái, i18n, hiển thị Markdown, mã QR và trình chỉnh sửa mã |
| DOMPurify | Apache-2.0 / MPL-2.0 | Khử trùng XSS trước khi hiển thị Markdown/HTML |

## 3. Các chương trình độc lập được bảng điều khiển tải xuống/gọi khi chạy

Khi cài đặt hoặc nâng cấp lõi, bảng điều khiển/Tác nhân tải xuống các tệp nhị phân sau từ GitHub và chạy chúng dưới dạng các quy trình riêng biệt. **Các sản phẩm được đánh dấu là "do dự án này xây dựng" được tổ chức này biên soạn và xuất bản từ nguồn thượng nguồn** (`qist/*-binaries`) và được phân phối cùng với sản phẩm, do đó, nghĩa vụ cấp phép tương ứng (bao gồm cả nguồn sẵn có cho GPL) thuộc về dự án này; những cái khác là URL tải xuống của các bản phát hành ngược dòng chính thức.

| chương trình | Giấy phép | Ghi chú |
|---|---|---|
| Xray-core | MPL-2.0 | Lõi proxy, quy trình riêng biệt; bản phát hành ngược dòng chính thức (`XTLS/Xray-core`) |
| sing-box | **GPL-3.0-or-later** | Lõi proxy, quy trình riêng biệt; **được xây dựng bởi dự án này** (`qist/singbox-binaries`; bản phát hành chính thức thiếu `with_v2ray_api`, cần thiết cho việc tính toán lưu lượng truy cập cho mỗi người dùng); cung cấp nguồn tương ứng khi phân phối |
| Caddy | Apache-2.0 | Proxy ngược / TLS; **được xây dựng bởi dự án này** (`qist/caddy-binaries`) |
| Nginx | BSD-2-Clause | Proxy ngược; **được xây dựng bởi dự án này** (`qist/nginx-binaries`, được chia cho glibc / musl) |

> Hysteria2 và TUIC là **các giao thức do sing-box cung cấp**, không phải lõi hoặc nhị phân độc lập nên chúng không được liệt kê riêng.

## 4. Tệp dữ liệu

| dữ liệu | Nguồn/giấy phép |
|---|---|
| `geoip.dat` / `geosite.dat` | Như được khai báo bởi kho lưu trữ nguồn dữ liệu (thường là GPL-3.0 hoặc CC-BY-SA) |
| GeoLite2 (`GeoLite2-ASN.mmdb`, v.v.) | MaxMind GeoLite2 EULA / CC BY-SA 4.0; xác minh giấy phép trước khi sử dụng thương mại |
| `country.mmdb` | Như được khai báo bởi nguồn dữ liệu của nó |

## 5. Khuyến nghị tuân thủ

- Khi phân phối các tệp nhị phân, hãy bao gồm: `LICENSE`, tệp này và văn bản giấy phép hoặc việc mua lại URL của từng thành phần GPL/MPL.
- Các tệp nhị phân do dự án này xây dựng (sing-box / Caddy / Nginx) đi kèm với bảng điều khiển: vì chúng được biên dịch từ nguồn ngược dòng, hãy xuất bản cách chúng được tạo (phiên bản, thẻ xây dựng, cờ định cấu hình); Các thành phần GPL (sing-box) cũng yêu cầu nguồn tương ứng hoàn chỉnh của phiên bản đó hoặc lộ trình thu nhận hiệu quả.
- Nếu bạn sửa đổi tệp nguồn của các thành phần MPL-2.0 như `xray-core`, hãy xuất bản nguồn của tệp đã sửa đổi.
- Nếu bạn phân phối các chương trình GPL như sing-box cùng với sản phẩm của mình, hãy cung cấp cho người nhận nguồn tương ứng đầy đủ của phiên bản đó (hoặc một đề nghị bằng văn bản có hiệu lực).
- Thương hiệu và tên: không sử dụng tên hoặc logo của dự án ngược dòng theo cách ngụ ý sự chứng thực chính thức.
