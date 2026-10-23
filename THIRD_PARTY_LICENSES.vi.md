<!-- source: THIRD_PARTY_LICENSES.md | locale: vi | updated: 2026-10-23 -->
# Thành phần và giấy phép của bên thứ ba

> Ngôn ngữ: [简体中文](THIRD_PARTY_LICENSES.md) · [English](THIRD_PARTY_LICENSES.en.md) · [繁體中文](THIRD_PARTY_LICENSES.zh-TW.md) · [Русский](THIRD_PARTY_LICENSES.ru.md) · [日本語](THIRD_PARTY_LICENSES.ja.md) · [한국어](THIRD_PARTY_LICENSES.ko.md) · [Deutsch](THIRD_PARTY_LICENSES.de.md) · [Français](THIRD_PARTY_LICENSES.fr.md) · [Español](THIRD_PARTY_LICENSES.es.md) · [Português](THIRD_PARTY_LICENSES.pt.md) · [Türkçe](THIRD_PARTY_LICENSES.tr.md) · [Bahasa Indonesia](THIRD_PARTY_LICENSES.id.md) · [ไทย](THIRD_PARTY_LICENSES.th.md) · Tiếng Việt · [العربية](THIRD_PARTY_LICENSES.ar.md) · [فارسی](THIRD_PARTY_LICENSES.fa.md)

CoreFusion sử dụng các thành phần nguồn mở sau. Tệp này liệt kê các thành phần và giấy phép chính; **danh sách đầy đủ được xác định bởi `go.mod` / `go.sum` và tệp GIẤY PHÉP riêng của mỗi mô-đun**.

Khi phân phối các tệp nhị phân của dự án này, hãy giữ tệp này, dự án `LICENSE` và thông báo bản quyền/giấy phép của tất cả các thành phần bên thứ ba.

## 1. Đi phụ thuộc được biên dịch thành nhị phân (chính)

| thành phần | Giấy phép | Mục đích |
|---|---|---|
| `github.com/xtls/xray-core` | MPL-2.0 | Phân tích cú pháp cấu hình Xray, lệnh gRPC, định nghĩa giao thức |
| `github.com/xtls/reality` | MPL-2.0 | Hỗ trợ giao thức REALITY |
| `github.com/sagernet/sing` | **GPL-3.0 trở lên** | Liên kết gián tiếp qua lõi xray |
| `github.com/sagernet/sing-shadowsocks` | **GPL-3.0 trở lên** | Liên kết gián tiếp qua lõi xray (Shadowsocks-2022) |
| `github.com/go-sql-driver/mysql` | MPL-2.0 | Trình điều khiển MySQL |
| `github.com/hashicorp/golang-lru/v2` | MPL-2.0 | Bộ đệm LRU |
| `github.com/go-acme/lego/v4` | MIT | Cấp chứng chỉ ACME |
| `github.com/gin-gonic/gin` | MIT | Khung HTTP |
| `gorm.io/gorm` | MIT | ORM |
| `github.com/aws/aws-sdk-go-v2` | Apache-2.0 | Bản sao lưu S3, Route53 DNS-01 |
| `google.golang.org/grpc` | Apache-2.0 | Giao tiếp gRPC của đại lý |
| `google.golang.org/protobuf` | Điều khoản BSD-3 | Thời gian chạy Protobuf |
| `golang.org/x/crypto` | Điều khoản BSD-3 | Mật mã nguyên thủy |
| `golang.org/x/oauth2` | Điều khoản BSD-3 | OAuth2 / Google EAB |
| `github.com/golang-jwt/jwt/v5` | MIT | JWT |
| `github.com/google/uuid` | Điều khoản BSD-3 | UUID |

> ⚠ **Thành phần GPL-3.0**: `sing` và `sing-shadowsocks` là GPL-3.0 trở lên. Chúng được liên kết tĩnh vào bảng điều khiển và các tệp nhị phân Tác nhân, vì vậy **việc phân phối các tệp nhị phân yêu cầu cung cấp Nguồn tương ứng** (xem phần 3 của [LEGAL.md](THIRD_PARTY_LICENSES.vi.md)).

## 2. Các chương trình độc lập được gọi trong thời gian chạy (không được phân phối trong kho lưu trữ này)

Bảng điều khiển / Tác nhân có thể tải xuống, cài đặt hoặc gọi các chương trình độc lập sau khi chạy. Họ tương tác với dự án này thông qua các quy trình/giao diện, nhưng nếu bạn gửi chúng cùng với sản phẩm bàn giao của mình thì bạn vẫn phải tuân thủ giấy phép của họ:

| chương trình | Giấy phép | Ghi chú |
|---|---|---|
| Lõi Xray | MPL-2.0 | Lõi proxy, quy trình riêng biệt |
| sing-box | **GPL-3.0 trở lên** | Lõi proxy, quy trình riêng biệt; cung cấp nguồn tương ứng khi phân phối |
| caddie | Apache-2.0 | Proxy ngược / TLS |
| Nginx | Điều khoản BSD-2 | Proxy ngược (được cài đặt trong hệ thống, nếu được sử dụng) |

> Hysteria2 và TUIC là **các giao thức do sing-box cung cấp**, không phải lõi hoặc nhị phân độc lập nên chúng không được liệt kê riêng.

## 3. Tệp dữ liệu

| dữ liệu | Nguồn/giấy phép |
|---|---|
| `geoip.dat` / `geosite.dat` | Như được khai báo bởi kho lưu trữ nguồn dữ liệu (thường là GPL-3.0 hoặc CC-BY-SA) |
| GeoLite2 (`GeoLite2-ASN.mmdb`, v.v.) | MaxMind GeoLite2 EULA / CC BY-SA 4.0; xác minh giấy phép trước khi sử dụng thương mại |
| `country.mmdb` | Như được khai báo bởi nguồn dữ liệu của nó |

## 4. Khuyến nghị tuân thủ

- Khi phân phối các tệp nhị phân, hãy bao gồm: `LICENSE`, tệp này và văn bản giấy phép hoặc việc mua lại URL của từng thành phần GPL/MPL.
- Nếu bạn sửa đổi tệp nguồn của các thành phần MPL-2.0 như `xray-core`, hãy xuất bản nguồn của tệp đã sửa đổi.
- Nếu bạn phân phối các chương trình GPL như sing-box cùng với sản phẩm của mình, hãy cung cấp cho người nhận nguồn tương ứng đầy đủ của phiên bản đó (hoặc một đề nghị bằng văn bản có hiệu lực).
- Thương hiệu và tên: không sử dụng tên hoặc logo của dự án ngược dòng theo cách ngụ ý sự chứng thực chính thức.
