<!-- source: admin-api.md | locale: vi | updated: 2026-10-23 -->
# Quản trị viên API

> Ngôn ngữ: [简体中文](admin-api.md) · [English](admin-api.en.md) · [繁體中文](admin-api.zh-TW.md) · [Русский](admin-api.ru.md) · [日本語](admin-api.ja.md) · [한국어](admin-api.ko.md) · [Deutsch](admin-api.de.md) · [Français](admin-api.fr.md) · [Español](admin-api.es.md) · [Português](admin-api.pt.md) · [Türkçe](admin-api.tr.md) · [Bahasa Indonesia](admin-api.id.md) · [ไทย](admin-api.th.md) · Tiếng Việt · [العربية](admin-api.ar.md) · [فارسی](admin-api.fa.md)

Bảng lộ trình API dành cho quản trị viên hoàn chỉnh dành cho **phát triển / tự động hóa / tích hợp bảng điều khiển**.

- Đế URL: `https://<panel-domain>/api/v1`
- Xác thực: `Authorization: Bearer <admin_token>` (từ `POST /api/v1/auth/login`)
- Các định dạng phản hồi/lỗi phổ biến và quy ước giới hạn tỷ lệ: xem [README.md](./README.vi.md)
- Nguồn sự thật: `internal/api/router/router.go` (739 tuyến quản trị viên); nếu bảng này không đồng ý, **mã sẽ thắng**

## Đánh dấu quyền

| điểm đánh dấu | Ý nghĩa |
|---|---|
| công khai | Không cần đăng nhập (bảo vệ bảo trì / giới hạn tốc độ vẫn có thể được áp dụng) |
| Công cộng · tỷ lệ giới hạn | Không cần đăng nhập, với giới hạn tỷ lệ chuyên dụng |
| Đã đăng nhập | Yêu cầu mã thông báo quản trị viên đã đăng nhập (`AuthRequired`) |
| Quản trị viên | Yêu cầu vai trò ≥ `admin` (`RequireRole(RoleAdmin)`) |
| Siêu quản trị viên | Yêu cầu vai trò = `super_admin` |

Cấp độ vai trò: `viewer(10) < operator(20) < admin(30) < super_admin(40)`; không đủ vai trò trả về 403 `INSUFFICIENT_ROLE`.

---

## Bắt đầu nhanh (ví dụ thực tế)

Những ví dụ này đến từ phản ứng thực tế của địa phương; mã thông báo và dữ liệu được sắp xếp lại.

### 1. Đăng nhập và nhận token

```bash
curl -X POST https://<panel-domain>/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"your-password"}'
```

Phản hồi:

```json
{
  "data": {
    "access_token": "<JWT, valid 24h>",
    "refresh_token": "<JWT, valid 7 days>",
    "user": { "id": 1, "role": "super_admin", "username": "admin" }
  }
}
```

### 2. Gọi điểm cuối doanh nghiệp bằng mã thông báo

```bash
curl https://<panel-domain>/api/v1/clients?limit=1 \
  -H 'Authorization: Bearer <access_token>'
```

Phản hồi (danh sách sử dụng cấu trúc `items` + `total`):

```json
{
  "data": {
    "items": [
      {
        "id": 27,
        "email": "user@example.com",
        "status": "active",
        "quota_bytes": 10737418240,
        "used_bytes": 0,
        "plan_id": 1,
        "expires_at": null,
        "created_at": "2026-09-30T15:54:32+08:00"
      }
    ],
    "total": 22,
    "limit": 1,
    "offset": 0
  }
}
```

### 3. Ví dụ về phản hồi lỗi

**401 chưa được xác thực:**

```json
{
  "code": "MISSING_TOKEN",
  "message": "missing authorization header",
  "request_id": "a02cfdf9-6be0-46be-8584-824caab34dc7",
  "can_retry": false,
  "requires_action": false
}
```

**Không tìm thấy tài nguyên 404:**

```json
{
  "code": "CLIENT_NOT_FOUND",
  "message": "client not found",
  "request_id": "099949bd-4ded-468d-a6f8-f8e798af2273",
  "can_retry": false,
  "requires_action": false
}
```

### 4. Phân trang

Tất cả các điểm cuối của danh sách đều sử dụng `limit` (kích thước trang, mặc định là 20) và `offset` (dựa trên 0) và trả về `{items, total, limit, offset}`. Một số điểm cuối cũ trả về một mảng trống (được ghi chú trên mỗi bảng).

---

## Nội dung

- [Xác thực và bảo mật của quản trị viên](#xác-thực-và-bảo-mật-của-quản-trị-viên)
- [Tài khoản quản trị viên](#tài-khoản-quản-trị-viên)
- [Quản lý khách hàng](#quản-lý-khách-hàng)
- [Mẫu kế hoạch](#mẫu-kế-hoạch)
- [Các biến thể của kế hoạch](#các-biến-thể-của-kế-hoạch)
- [Gói giao thông](#gói-giao-thông)
- [Phiếu giảm giá](#phiếu-giảm-giá)
- [Nhóm nút](#nhóm-nút)
- [Máy chủ nút](#máy-chủ-nút)
- [Nhật ký kiểm tra nút](#nhật-ký-kiểm-tra-nút)
- [Nút (mô hình cũ, đã ngừng hoạt động)](#nút-mô-hình-cũ-đã-ngừng-hoạt-động)
- [Tổng quan khu vực](#tổng-quan-khu-vực)
- [Đăng ký](#đăng-ký)
- [Định dạng đăng ký](#định-dạng-đăng-ký)
- [Điểm cuối đăng ký](#điểm-cuối-đăng-ký)
- [Chia sẻ tên miền](#chia-sẻ-tên-miền)
- [IP Cloudflare](#ip-cloudflare)
- [Cấu hình đăng ký (Mihomo/Clash)](#cấu-hình-đăng-ký-mihomoclash)
- [Trang web nạp tiền](#trang-web-nạp-tiền)
- [Nạp tiền đơn hàng](#nạp-tiền-đơn-hàng)
- [Nạp thẻ](#nạp-thẻ)
- [Tính phí sự kiện gọi lại](#tính-phí-sự-kiện-gọi-lại)
- [Tính phí cuộc gọi lại lần thử](#tính-phí-cuộc-gọi-lại-lần-thử)
- [Số dư của khách hàng](#số-dư-của-khách-hàng)
- [Quà tặng](#quà-tặng)
- [Cấu hình giới thiệu](#cấu-hình-giới-thiệu)
- [Rút tiền giới thiệu](#rút-tiền-giới-thiệu)
- [Danh sách cho phép IP (tóm tắt)](#danh-sách-cho-phép-ip-tóm-tắt)
- [Ràng buộc OAuth của khách hàng](#ràng-buộc-oauth-của-khách-hàng)
- [Vé (quản trị viên)](#vé-quản-trị-viên)
- [bài viết](#bài-viết)
- [Tài khoản được chia sẻ](#tài-khoản-được-chia-sẻ)
- [Chứng chỉ](#chứng-chỉ)
- [Kênh chứng chỉ](#kênh-chứng-chỉ)
- [Cấu hình EAB](#cấu-hình-eab)
- [Tài khoản ACME](#tài-khoản-acme)
- [CA](#built-in-ca) tích hợp
- [CA tích hợp](#ca-tích-hợp)
- [Quy tắc địa lý](#quy-tắc-địa-lý)
- [Thư viện quy tắc WAF](#thư-viện-quy-tắc-waf)
- [Quản lý nhật ký](#quản-lý-nhật-ký)
- [Sao lưu](#sao-lưu)
- [Cài đặt hệ thống](#cài-đặt-hệ-thống)
- [Kiểm tra an ninh](#kiểm-tra-an-ninh)
- [Số liệu](#số-liệu)
- [Mẫu cổng thông tin](#mẫu-cổng-thông-tin)
- [Cài đặt đại lý](#cài-đặt-đại-lý)
- [Tính phí mục gọi lại](#tính-phí-mục-gọi-lại)
- [Đường vào (có nút)](#đường-vào-có-nút)
- [Cấu hình công khai](#cấu-hình-công-khai)
- [Xác thực khách hàng và doanh nghiệp (cùng được đặt trên công cụ quản trị)](#xác-thực-khách-hàng-và-doanh-nghiệp-cùng-được-đặt-trên-công-cụ-quản-trị)
- [Phân phối đăng ký](#phân-phối-đăng-ký)
- [Quy trình cấu hình thời gian chạy](#quy-trình-cấu-hình-thời-gian-chạy)
- [Chính sách truy cập dịch vụ (đã ngừng hoạt động)](#chính-sách-truy-cập-dịch-vụ-đã-ngừng-hoạt-động)
- [Giám sát hạn ngạch giao thông](#giám-sát-hạn-ngạch-giao-thông)
- [Kiểm tra sức khỏe](#kiểm-tra-sức-khỏe)
- [Trang cổng thông tin](#trang-cổng-thông-tin)

---

### Xác thực và bảo mật của quản trị viên

Đăng nhập, đăng xuất, thay đổi mật khẩu, 2FA (TOTP), Passkey (WebAuthn) và vé một lần cho bảng quản trị.

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| POST | `/api/v1/auth/2fa/disable` | Đã đăng nhập | Vô hiệu hóa |
| POST | `/api/v1/auth/2fa/enable` | Đã đăng nhập | Kích hoạt |
| POST | `/api/v1/auth/2fa/recovery-codes` | Đã đăng nhập | Tạo lại mã khôi phục |
| POST | `/api/v1/auth/2fa/setup` | Đã đăng nhập | Cài đặt |
| GET | `/api/v1/auth/2fa/status` | Đã đăng nhập | Trạng thái |
| POST | `/api/v1/auth/change-password` | Đã đăng nhập | Thay đổi mật khẩu |
| POST | `/api/v1/auth/login` | công khai | Đăng nhập |
| POST | `/api/v1/auth/login/2fa` | công khai | Đăng nhập bằng mã động (2FA) |
| GET | `/api/v1/auth/login/2fa-status` | công khai | Kiểm tra xem tài khoản đã bật 2FA chưa |
| POST | `/api/v1/auth/logout` | Đã đăng nhập | Đăng xuất |
| GET | `/api/v1/auth/me` | Đã đăng nhập | Người dùng đã đăng nhập hiện tại |
| GET | `/api/v1/auth/passkey/credentials` | Đã đăng nhập | Danh sách thông tin xác thực Passkey |
| DELETE | `/api/v1/auth/passkey/credentials/:id` | Đã đăng nhập | Xóa thông tin xác thực Passkey |
| POST | `/api/v1/auth/passkey/login/begin` | công khai | Bắt đầu đăng nhập |
| POST | `/api/v1/auth/passkey/login/finish` | công khai | Đăng nhập hoàn tất |
| POST | `/api/v1/auth/passkey/register/begin` | Đã đăng nhập | Bắt đầu đăng ký |
| POST | `/api/v1/auth/passkey/register/finish` | Đã đăng nhập | Hoàn tất đăng ký |
| GET | `/api/v1/auth/passkey/rpids` | Đã đăng nhập | Danh sách cho phép ID RP |
| POST | `/api/v1/auth/passkey/rpids` | Đã đăng nhập | Cập nhật danh sách cho phép ID RP |
| GET | `/api/v1/auth/passkey/status` | công khai | Trạng thái |
| POST | `/api/v1/auth/refresh` | công khai | Làm mới |
| POST | `/api/v1/auth/ticket` | Đã đăng nhập | Xuất vé ngắn hạn một lần |

### Tài khoản quản trị viên

Tài khoản người dùng bảng quản trị. Danh sách yêu cầu quản trị viên; tạo/cập nhật/xóa yêu cầu quản trị viên cấp cao (ngăn chặn leo thang đặc quyền).

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| GET | `/api/v1/users` | Quản trị viên | Danh sách |
| POST | `/api/v1/users` | Siêu quản trị viên | Tạo |
| DELETE | `/api/v1/users/:id` | Siêu quản trị viên | Xóa |
| PUT | `/api/v1/users/:id` | Siêu quản trị viên | cập nhật |

### Quản lý khách hàng

CRUD, tạm dừng/tiếp tục, nâng cấp/hạ cấp, gia hạn và đặt lại mức sử dụng cho người đăng ký (tài khoản cổng thông tin).

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| GET | `/api/v1/clients` | Đã đăng nhập | Liệt kê khách hàng |
| POST | `/api/v1/clients` | Đã đăng nhập | Tạo khách hàng. Thân máy: `{"email":"user@example.com","password":"plaintext-password","plan_id":1,"quota_bytes":10737418240,"expires_at":"2026-12-31T00:00:00Z","remark":""}`. `plan_id` là bắt buộc; `password` có thể trống (khách hàng có thể đặt sau trong cổng) |
| DELETE | `/api/v1/clients/:id` | Đã đăng nhập | Xóa khách hàng |
| GET | `/api/v1/clients/:id` | Đã đăng nhập | Nhận khách hàng |
| PUT | `/api/v1/clients/:id` | Đã đăng nhập | Cập nhật khách hàng |
| POST | `/api/v1/clients/:id/downgrade` | Đã đăng nhập | Hạ cấp khách hàng |
| GET | `/api/v1/clients/:id/orders` | Đã đăng nhập | Danh sách đơn hàng của khách hàng |
| POST | `/api/v1/clients/:id/renew` | Đã đăng nhập | Gia hạn khách hàng |
| POST | `/api/v1/clients/:id/reset-usage` | Đã đăng nhập | Đặt lại mức sử dụng |
| POST | `/api/v1/clients/:id/restore` | Đã đăng nhập | Tiếp tục khách hàng |
| POST | `/api/v1/clients/:id/suspend` | Đã đăng nhập | Đình chỉ khách hàng |
| POST | `/api/v1/clients/:id/upgrade` | Đã đăng nhập | Nâng cấp khách hàng |

### Mẫu kế hoạch

Các gói (biến thể, chu kỳ thanh toán, giá, hạn ngạch lưu lượng truy cập) và ủy quyền nút cấp mẫu.

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| GET | `/api/v1/plan-templates` | Đã đăng nhập | Danh sách mẫu |
| POST | `/api/v1/plan-templates` | Đã đăng nhập | Tạo mẫu |
| DELETE | `/api/v1/plan-templates/:id` | Đã đăng nhập | Xóa mẫu |
| GET | `/api/v1/plan-templates/:id` | Đã đăng nhập | Chi tiết mẫu |
| PUT | `/api/v1/plan-templates/:id` | Đã đăng nhập | Cập nhật mẫu |
| GET | `/api/v1/plan-templates/:id/accessible-nodes` | Đã đăng nhập | Nhận các nút có sẵn |
| GET | `/api/v1/plan-templates/:id/billing-node-options` | Đã đăng nhập | Nhận các tùy chọn nút có thể thanh toán |
| PUT | `/api/v1/plan-templates/:id/node-groups` | Đã đăng nhập | Đặt nhóm nút mẫu |
| POST | `/api/v1/plan-templates/:id/sync-accounts` | Đã đăng nhập | Đồng bộ hóa tài khoản với mẫu |
| GET | `/api/v1/plan-templates/:id/variants` | Đã đăng nhập | Danh sách biến thể |
| POST | `/api/v1/plan-templates/:id/variants` | Đã đăng nhập | Tạo biến thể |
| POST | `/api/v1/plan-templates/sync-all-accounts` | Đã đăng nhập | Đồng bộ hóa tất cả tài khoản |

### Các biến thể của kế hoạch

Cập nhật và xóa các biến thể kế hoạch (việc tạo được lồng trong các mẫu kế hoạch).

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| DELETE | `/api/v1/plan-variants/:id` | Đã đăng nhập | Xóa biến thể |
| PUT | `/api/v1/plan-variants/:id` | Đã đăng nhập | Cập nhật biến thể |

### Gói giao thông

Gói giao thông có thể được mua riêng.

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| GET | `/api/v1/traffic-packs` | Đã đăng nhập | Danh sách gói lưu lượng truy cập |
| POST | `/api/v1/traffic-packs` | Đã đăng nhập | Tạo gói lưu lượng truy cập |
| DELETE | `/api/v1/traffic-packs/:id` | Đã đăng nhập | Xóa gói lưu lượng truy cập |
| PUT | `/api/v1/traffic-packs/:id` | Đã đăng nhập | Cập nhật gói lưu lượng truy cập |

### Phiếu giảm giá

Tạo, xác nhận và đổi phiếu giảm giá.

| Phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| GET | `/api/v1/promotions` | Đã đăng nhập | Liệt kê phiếu giảm giá |
| POST | `/api/v1/promotions` | Đã đăng nhập | Tạo phiếu giảm giá |
| DELETE | `/api/v1/promotions/:id` | Đã đăng nhập | Xóa phiếu giảm giá |
| PUT | `/api/v1/promotions/:id` | Đã đăng nhập | Cập nhật phiếu giảm giá |
| POST | `/api/v1/promotions/validate` | Đã đăng nhập | Xác thực phiếu giảm giá |

### Nhóm nút

Tổ chức các nút thành các nhóm được tham chiếu bởi các kế hoạch.

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| GET | `/api/v1/node-groups` | Đã đăng nhập | Liệt kê các nhóm nút |
| POST | `/api/v1/node-groups` | Đã đăng nhập | Tạo nhóm nút |
| DELETE | `/api/v1/node-groups/:id` | Đã đăng nhập | Xóa nhóm nút |
| PUT | `/api/v1/node-groups/:id` | Đã đăng nhập | Cập nhật nhóm nút |
| GET | `/api/v1/node-groups/:id/members` | Đã đăng nhập | Liệt kê thành viên nhóm |
| POST | `/api/v1/node-groups/:id/members` | Đã đăng nhập | Thêm thành viên nhóm |
| DELETE | `/api/v1/node-groups/:id/members/:mid` | Đã đăng nhập | Xóa thành viên nhóm |

### Máy chủ nút

Tài nguyên cốt lõi: nút CRUD, phân phối cấu hình lõi (xray/sing-box), cấu hình gửi đến/ra/điểm cuối, kiểm soát quy trình, chứng chỉ, WAF, giới hạn tốc độ, nhảy cổng, proxy chuỗi, sao lưu nhân bản/sao lưu chính, cài đặt/nâng cấp tác nhân, kiểm soát thời gian chạy xray. Nhiều đường dẫn phụ, được nhóm theo họ đối tượng.

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| GET | `/api/v1/node-servers` | Quản trị viên | Danh sách |
| POST | `/api/v1/node-servers` | Quản trị viên | Tạo |
| DELETE | `/api/v1/node-servers/:id` | Quản trị viên | Xóa |
| GET | `/api/v1/node-servers/:id` | Quản trị viên | Chi tiết |
| PUT | `/api/v1/node-servers/:id` | Quản trị viên | cập nhật |
| GET | `/api/v1/node-servers/:id/binaries` | Quản trị viên | Danh sách lõi đã cài đặt |
| POST | `/api/v1/node-servers/:id/binaries/ech-generate` | Quản trị viên | Tạo cặp khóa ECH |
| POST | `/api/v1/node-servers/:id/binaries/install` | Quản trị viên | Cài đặt/nâng cấp lõi |
| GET | `/api/v1/node-servers/:id/binaries/interfaces` | Quản trị viên | Danh sách giao diện mạng |
| POST | `/api/v1/node-servers/:id/binaries/mldsa65-key` | Quản trị viên | Tạo khóa MLDSA65 |
| POST | `/api/v1/node-servers/:id/binaries/reality-key` | Quản trị viên | Tạo cặp khóa REALITY |
| POST | `/api/v1/node-servers/:id/binaries/reality-public-key` | Quản trị viên | Lấy khóa chung từ khóa riêng REALITY |
| GET | `/api/v1/node-servers/:id/binaries/releases` | Quản trị viên | Phiên bản có sẵn |
| POST | `/api/v1/node-servers/:id/binaries/vlessenc` | Quản trị viên | Tạo tham số mã hóa VLESS |
| POST | `/api/v1/node-servers/:id/binaries/wg-key` | Quản trị viên | Tạo khóa WireGuard |
| POST | `/api/v1/node-servers/:id/binaries/x25519-key` | Quản trị viên | Tạo cặp khóa X25519 |
| GET | `/api/v1/node-servers/:id/bootstrap-cmd` | Quản trị viên | Nhận lệnh khởi động tác nhân |
| POST | `/api/v1/node-servers/:id/bootstrap-token` | Quản trị viên | Tạo mã thông báo bootstrap |
| POST | `/api/v1/node-servers/:id/caddy/batch-update-log-enabled` | Quản trị viên | Công tắc nhật ký cập nhật hàng loạt |
| POST | `/api/v1/node-servers/:id/caddy/batch-update-ports` | Quản trị viên | Cổng cập nhật hàng loạt |
| POST | `/api/v1/node-servers/:id/caddy/generate-config` | Quản trị viên | Tạo cấu hình |
| DELETE | `/api/v1/node-servers/:id/caddy/l4-rules` | Quản trị viên | Xóa tất cả các quy tắc L4 |
| POST | `/api/v1/node-servers/:id/caddy/save-and-apply` | Quản trị viên | Lưu và áp dụng |
| GET | `/api/v1/node-servers/:id/caddy/sites` | Quản trị viên | Danh sách trang web |
| POST | `/api/v1/node-servers/:id/caddy/sites` | Quản trị viên | Tạo trang web |
| DELETE | `/api/v1/node-servers/:id/caddy/sites/:siteId` | Quản trị viên | Xóa trang web |
| PUT | `/api/v1/node-servers/:id/caddy/sites/:siteId` | Quản trị viên | Cập nhật trang web |
| DELETE | `/api/v1/node-servers/:id/caddy/sni-rules` | Quản trị viên | Xóa tất cả quy tắc SNI |
| POST | `/api/v1/node-servers/:id/caddy/sni-rules/toggle` | Quản trị viên | Chuyển đổi quy tắc SNI |
| GET | `/api/v1/node-servers/:id/caddy/sni-settings` | Quản trị viên | Cài đặt SNI |
| POST | `/api/v1/node-servers/:id/caddy/sni-settings` | Quản trị viên | Lưu cài đặt SNI |
| GET | `/api/v1/node-servers/:id/certificates` | Quản trị viên | Danh sách chứng chỉ nút |
| DELETE | `/api/v1/node-servers/:id/certificates/:cid` | Quản trị viên | Hủy liên kết chứng chỉ |
| POST | `/api/v1/node-servers/:id/certificates/:cid` | Quản trị viên | Chứng chỉ ràng buộc |
| GET | `/api/v1/node-servers/:id/chain-config` | Quản trị viên | Tạo cấu hình chuỗi |
| GET | `/api/v1/node-servers/:id/chain-health` | Quản trị viên | Tình trạng sức khỏe của chuỗi |
| POST | `/api/v1/node-servers/:id/control` | Siêu quản trị viên | Lõi nút điều khiển. phần thân: `{"core_type":"xray","action":"start"}` (hành động = bắt đầu/dừng/khởi động lại). `core_type` có thể là `xray`/`singbox` |
| GET | `/api/v1/node-servers/:id/degradation-status` | Quản trị viên | Tình trạng xuống cấp |
| POST | `/api/v1/node-servers/:id/distribute-cert` | Quản trị viên | Phân phối chứng chỉ |
| GET | `/api/v1/node-servers/:id/endpoint-configs` | Quản trị viên | Danh sách |
| POST | `/api/v1/node-servers/:id/endpoint-configs` | Quản trị viên | Tạo |
| DELETE | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | Quản trị viên | Xóa |
| GET | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | Quản trị viên | Chi tiết |
| PUT | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | Quản trị viên | cập nhật |
| POST | `/api/v1/node-servers/:id/failover` | Quản trị viên | Chuyển đổi dự phòng kích hoạt cho nút này |
| GET | `/api/v1/node-servers/:id/failover-status` | Quản trị viên | Trạng thái chuyển đổi dự phòng |
| POST | `/api/v1/node-servers/:id/force-reconnect` | Quản trị viên | Buộc kết nối lại |
| GET | `/api/v1/node-servers/:id/free-port` | Quản trị viên | Yêu cầu một cổng miễn phí |
| POST | `/api/v1/node-servers/:id/geo-rules/push` | Quản trị viên | Đẩy tới nút này |
| GET | `/api/v1/node-servers/:id/inbound-configs` | Quản trị viên | Danh sách |
| POST | `/api/v1/node-servers/:id/inbound-configs` | Quản trị viên | Tạo |
| DELETE | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | Quản trị viên | Xóa |
| GET | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | Quản trị viên | Chi tiết |
| PUT | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | Quản trị viên | cập nhật |
| GET | `/api/v1/node-servers/:id/inbound-configs/:cfgId/share-links` | Quản trị viên | Chia sẻ liên kết và đoạn trích |
| POST | `/api/v1/node-servers/:id/inbound-configs/:cfgId/users/:email/pause` | Quản trị viên | Đình chỉ người dùng này |
| POST | `/api/v1/node-servers/:id/inbound-configs/:cfgId/users/:email/resume` | Quản trị viên | Khôi phục người dùng này |
| POST | `/api/v1/node-servers/:id/inbound-configs/reset-all-traffic` | Quản trị viên | Đặt lại tất cả lưu lượng truy cập vào |
| GET | `/api/v1/node-servers/:id/install-script` | Quản trị viên | Nhận tập lệnh cài đặt đại lý |
| GET | `/api/v1/node-servers/:id/metrics-history` | Quản trị viên | Nhận lịch sử số liệu |
| GET | `/api/v1/node-servers/:id/mirror-status` | Quản trị viên | Trạng thái phản chiếu |
| POST | `/api/v1/node-servers/:id/nginx/batch-update-ports` | Quản trị viên | Cổng cập nhật hàng loạt |
| POST | `/api/v1/node-servers/:id/nginx/generate-config` | Quản trị viên | Tạo cấu hình |
| DELETE | `/api/v1/node-servers/:id/nginx/l4-rules` | Quản trị viên | Xóa tất cả các quy tắc L4 |
| POST | `/api/v1/node-servers/:id/nginx/save-and-apply` | Quản trị viên | Lưu và áp dụng |
| GET | `/api/v1/node-servers/:id/nginx/settings` | Quản trị viên | Đọc cài đặt |
| POST | `/api/v1/node-servers/:id/nginx/settings` | Quản trị viên | Lưu cài đặt |
| GET | `/api/v1/node-servers/:id/nginx/sites` | Quản trị viên | Danh sách trang web |
| POST | `/api/v1/node-servers/:id/nginx/sites` | Quản trị viên | Tạo trang web |
| DELETE | `/api/v1/node-servers/:id/nginx/sites/:siteId` | Quản trị viên | Xóa trang web |
| PUT | `/api/v1/node-servers/:id/nginx/sites/:siteId` | Quản trị viên | Cập nhật trang web |
| DELETE | `/api/v1/node-servers/:id/nginx/sni-rules` | Quản trị viên | Xóa tất cả quy tắc SNI |
| POST | `/api/v1/node-servers/:id/nginx/sni-rules/toggle` | Quản trị viên | Chuyển đổi quy tắc SNI |
| PUT | `/api/v1/node-servers/:id/node-type` | Quản trị viên | Cập nhật loại nút |
| GET | `/api/v1/node-servers/:id/online-users` | Quản trị viên | Người dùng trực tuyến |
| GET | `/api/v1/node-servers/:id/outbound-configs` | Quản trị viên | Danh sách |
| POST | `/api/v1/node-servers/:id/outbound-configs` | Quản trị viên | Tạo |
| DELETE | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | Quản trị viên | Xóa |
| GET | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | Quản trị viên | Chi tiết |
| PUT | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | Quản trị viên | cập nhật |
| PUT | `/api/v1/node-servers/:id/outbound-configs/reorder` | Quản trị viên | Sắp xếp |
| GET | `/api/v1/node-servers/:id/port-hop/rules` | Quản trị viên | Xem trước quy tắc nhảy cổng |
| GET | `/api/v1/node-servers/:id/protocols` | Quản trị viên | Danh sách giao thức |
| GET | `/api/v1/node-servers/:id/protocols/:coreType` | Quản trị viên | Cấu hình giao thức |
| PUT | `/api/v1/node-servers/:id/protocols/:coreType` | Quản trị viên | Lưu cấu hình giao thức |
| GET | `/api/v1/node-servers/:id/protocols/:coreType/config-file` | Quản trị viên | Đọc tập tin cấu hình lõi |
| GET | `/api/v1/node-servers/:id/protocols/:coreType/section/:section` | Quản trị viên | Đọc phần giao thức |
| PUT | `/api/v1/node-servers/:id/protocols/:coreType/section/:section` | Quản trị viên | Lưu phần giao thức |
| POST | `/api/v1/node-servers/:id/protocols/:coreType/validate` | Quản trị viên | Xác thực cấu hình (chạy khô lõi thực) |
| GET | `/api/v1/node-servers/:id/proxy-status` | Quản trị viên | Trạng thái proxy từ xa |
| POST | `/api/v1/node-servers/:id/proxy/batch-update-ports` | Quản trị viên | Cổng cập nhật hàng loạt |
| GET | `/api/v1/node-servers/:id/proxy/config` | Quản trị viên | Đọc cấu hình |
| PUT | `/api/v1/node-servers/:id/proxy/config` | Quản trị viên | Cập nhật cấu hình |
| POST | `/api/v1/node-servers/:id/proxy/control` | Quản trị viên | Kiểm soát đăng ký proxy theo chuỗi |
| POST | `/api/v1/node-servers/:id/proxy/download` | Quản trị viên | Tải xuống phiên bản lõi proxy |
| POST | `/api/v1/node-servers/:id/proxy/generate-config` | Quản trị viên | Tạo cấu hình |
| POST | `/api/v1/node-servers/:id/proxy/guardian` | Quản trị viên | Cập nhật cấu hình giám sát |
| POST | `/api/v1/node-servers/:id/proxy/install` | Quản trị viên | Cài đặt proxy |
| DELETE | `/api/v1/node-servers/:id/proxy/l4-rules` | Quản trị viên | Xóa tất cả các quy tắc L4 |
| GET | `/api/v1/node-servers/:id/proxy/l4-settings` | Quản trị viên | Cài đặt L4 |
| POST | `/api/v1/node-servers/:id/proxy/l4-settings` | Quản trị viên | Lưu cài đặt L4 |
| GET | `/api/v1/node-servers/:id/proxy/log-settings` | Quản trị viên | Cài đặt nhật ký |
| POST | `/api/v1/node-servers/:id/proxy/log-settings` | Quản trị viên | Lưu cài đặt nhật ký |
| GET | `/api/v1/node-servers/:id/proxy/nginx-settings` | Quản trị viên | Cài đặt nginx |
| POST | `/api/v1/node-servers/:id/proxy/nginx-settings` | Quản trị viên | Lưu cài đặt nginx |
| GET | `/api/v1/node-servers/:id/proxy/releases` | Quản trị viên | Phiên bản có sẵn |
| POST | `/api/v1/node-servers/:id/proxy/reload` | Quản trị viên | Tải lại cấu hình |
| POST | `/api/v1/node-servers/:id/proxy/restart` | Quản trị viên | Khởi động lại |
| GET | `/api/v1/node-servers/:id/proxy/sites` | Quản trị viên | Danh sách trang web |
| POST | `/api/v1/node-servers/:id/proxy/sites` | Quản trị viên | Tạo trang web |
| DELETE | `/api/v1/node-servers/:id/proxy/sites/:siteId` | Quản trị viên | Xóa trang web |
| PUT | `/api/v1/node-servers/:id/proxy/sites/:siteId` | Quản trị viên | Cập nhật trang web |
| DELETE | `/api/v1/node-servers/:id/proxy/sni-rules` | Quản trị viên | Xóa tất cả quy tắc SNI |
| POST | `/api/v1/node-servers/:id/proxy/sni-rules/toggle` | Quản trị viên | Chuyển đổi quy tắc SNI |
| GET | `/api/v1/node-servers/:id/proxy/sni-settings` | Quản trị viên | Cài đặt SNI |
| POST | `/api/v1/node-servers/:id/proxy/sni-settings` | Quản trị viên | Lưu cài đặt SNI |
| POST | `/api/v1/node-servers/:id/proxy/start` | Quản trị viên | Bắt đầu |
| GET | `/api/v1/node-servers/:id/proxy/status` | Quản trị viên | Trạng thái quá trình proxy |
| POST | `/api/v1/node-servers/:id/proxy/stop` | Quản trị viên | Dừng lại |
| POST | `/api/v1/node-servers/:id/proxy/sync` | Quản trị viên | Đồng bộ hóa cấu hình proxy |
| GET | `/api/v1/node-servers/:id/rate-limit` | Quản trị viên | Đọc cấu hình |
| PUT | `/api/v1/node-servers/:id/rate-limit` | Quản trị viên | Cập nhật cấu hình |
| GET | `/api/v1/node-servers/:id/rate-limit/status` | Quản trị viên | Trạng thái |
| GET | `/api/v1/node-servers/:id/recover-mode` | Quản trị viên | Đọc chế độ phục hồi nút |
| PUT | `/api/v1/node-servers/:id/recover-mode` | Quản trị viên | Đặt chế độ khôi phục nút |
| POST | `/api/v1/node-servers/:id/revoke-cert` | Quản trị viên | Thu hồi chứng chỉ đại lý |
| GET | `/api/v1/node-servers/:id/services` | Quản trị viên | Danh sách |
| POST | `/api/v1/node-servers/:id/services` | Quản trị viên | Tạo |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId` | Quản trị viên | Xóa |
| GET | `/api/v1/node-servers/:id/services/:serviceId` | Quản trị viên | Chi tiết |
| PUT | `/api/v1/node-servers/:id/services/:serviceId` | Quản trị viên | cập nhật |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId/crash-logs` | Quản trị viên | Xóa nhật ký sự cố |
| GET | `/api/v1/node-servers/:id/services/:serviceId/crash-logs` | Quản trị viên | Danh sách nhật ký sự cố |
| GET | `/api/v1/node-servers/:id/services/:serviceId/crash-logs/:logId` | Quản trị viên | Chi tiết nhật ký sự cố |
| GET | `/api/v1/node-servers/:id/services/:serviceId/endpoints` | Quản trị viên | Danh sách điểm cuối |
| POST | `/api/v1/node-servers/:id/services/:serviceId/endpoints` | Quản trị viên | Tạo điểm cuối |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId/endpoints/:endpointId` | Quản trị viên | Xóa điểm cuối |
| PUT | `/api/v1/node-servers/:id/services/:serviceId/endpoints/:endpointId` | Quản trị viên | Cập nhật điểm cuối |
| PUT | `/api/v1/node-servers/:id/services/:serviceId/guardian` | Quản trị viên | Cập nhật cấu hình giám sát |
| GET | `/api/v1/node-servers/:id/services/:serviceId/process-status` | Quản trị viên | Nhận trạng thái xử lý |
| POST | `/api/v1/node-servers/:id/services/:serviceId/restart` | Quản trị viên | Khởi động lại |
| POST | `/api/v1/node-servers/:id/services/:serviceId/start` | Quản trị viên | Bắt đầu |
| POST | `/api/v1/node-servers/:id/services/:serviceId/stop` | Quản trị viên | Dừng lại |
| GET | `/api/v1/node-servers/:id/services/process-status` | Quản trị viên | Tất cả các trạng thái quá trình |
| GET | `/api/v1/node-servers/:id/share-domains` | Quản trị viên | Chia sẻ tên miền cho nút này |
| POST | `/api/v1/node-servers/:id/share-domains` | Quản trị viên | Tạo |
| DELETE | `/api/v1/node-servers/:id/share-domains/:domainId` | Quản trị viên | Xóa |
| PUT | `/api/v1/node-servers/:id/share-domains/:domainId` | Quản trị viên | cập nhật |
| DELETE | `/api/v1/node-servers/:id/share-domains/all` | Quản trị viên | Xóa tất cả |
| POST | `/api/v1/node-servers/:id/singbox/control/firewall/apply` | Quản trị viên | Áp dụng quy tắc tường lửa nhảy cổng |
| POST | `/api/v1/node-servers/:id/singbox/control/firewall/remove` | Quản trị viên | Loại bỏ các quy tắc tường lửa nhảy cổng |
| POST | `/api/v1/node-servers/:id/speed-test` | Quản trị viên | Kiểm tra tốc độ nút |
| GET | `/api/v1/node-servers/:id/status` | Quản trị viên | Trạng thái nút |
| POST | `/api/v1/node-servers/:id/sync` | Quản trị viên | Cấu hình nút đồng bộ hóa (tạo và đẩy) |
| POST | `/api/v1/node-servers/:id/sync-data` | Siêu quản trị viên | Đẩy dữ liệu bảng đến các nút (đồng bộ hóa đầy đủ người dùng/đăng ký/gói) |
| POST | `/api/v1/node-servers/:id/sync-mirror` | Quản trị viên | Nút nhân bản đồng bộ hóa |
| GET | `/api/v1/node-servers/:id/tags` | Quản trị viên | Danh sách thẻ |
| POST | `/api/v1/node-servers/:id/tags` | Quản trị viên | Thêm thẻ |
| DELETE | `/api/v1/node-servers/:id/tags/:tagKey` | Quản trị viên | Xóa thẻ |
| GET | `/api/v1/node-servers/:id/traffic-history` | Quản trị viên | lịch sử giao thông |
| POST | `/api/v1/node-servers/:id/traffic-resume` | Quản trị viên | Khôi phục lưu lượng truy cập bị đình chỉ do quá tải |
| GET | `/api/v1/node-servers/:id/traffic-usage` | Quản trị viên | Sử dụng lưu lượng truy cập |
| POST | `/api/v1/node-servers/:id/update-agent` | Quản trị viên | Đại lý nâng cấp |
| GET | `/api/v1/node-servers/:id/used-server-names` | Quản trị viên | Danh sách REALITY SNI đang sử dụng |
| GET | `/api/v1/node-servers/:id/waf/config` | Quản trị viên | Đọc cấu hình |
| PUT | `/api/v1/node-servers/:id/waf/config` | Quản trị viên | Lưu cấu hình |
| POST | `/api/v1/node-servers/:id/waf/disable-domain` | Quản trị viên | Vô hiệu hóa bảo vệ tên miền |
| GET | `/api/v1/node-servers/:id/waf/domain-config` | Quản trị viên | Danh sách cấu hình tên miền |
| PUT | `/api/v1/node-servers/:id/waf/domain-config` | Quản trị viên | Lưu cấu hình tên miền hàng loạt |
| DELETE | `/api/v1/node-servers/:id/waf/domain-config/:domain` | Quản trị viên | Xóa cấu hình miền |
| GET | `/api/v1/node-servers/:id/waf/domain-config/:domain` | Quản trị viên | Chi tiết cấu hình tên miền |
| PUT | `/api/v1/node-servers/:id/waf/domain-config/:domain` | Quản trị viên | Lưu cấu hình tên miền |
| GET | `/api/v1/node-servers/:id/waf/domain-rules` | Quản trị viên | Thư mục quy tắc tên miền |
| GET | `/api/v1/node-servers/:id/waf/domain-rules/:domain/:name` | Quản trị viên | Đọc tệp quy tắc tên miền |
| PUT | `/api/v1/node-servers/:id/waf/domain-rules/:domain/:name` | Quản trị viên | Lưu tệp quy tắc tên miền |
| GET | `/api/v1/node-servers/:id/waf/domain-status` | Quản trị viên | Tình trạng bảo vệ tên miền |
| POST | `/api/v1/node-servers/:id/waf/enable-domain` | Quản trị viên | Bật bảo vệ miền |
| POST | `/api/v1/node-servers/:id/waf/push` | Quản trị viên | Đẩy tới nút này |
| GET | `/api/v1/node-servers/:id/waf/rules` | Quản trị viên | Danh sách quy tắc nút WAF |
| DELETE | `/api/v1/node-servers/:id/waf/rules/:name` | Quản trị viên | Xóa quy tắc nút WAF |
| GET | `/api/v1/node-servers/:id/waf/rules/:name` | Quản trị viên | Chi tiết quy tắc nút WAF |
| PUT | `/api/v1/node-servers/:id/waf/rules/:name` | Quản trị viên | Lưu quy tắc nút WAF |
| GET | `/api/v1/node-servers/:id/waf/status` | Quản trị viên | Trạng thái nút WAF |
| PUT | `/api/v1/node-servers/:id/waf/status` | Quản trị viên | Chuyển đổi nút WAF |
| POST | `/api/v1/node-servers/:id/waf/sync` | Quản trị viên | Đồng bộ hóa với nút này |
| GET | `/api/v1/node-servers/:id/warp` | Quản trị viên | Danh sách |
| DELETE | `/api/v1/node-servers/:id/warp/:warpId` | Quản trị viên | Xóa |
| GET | `/api/v1/node-servers/:id/warp/config` | Quản trị viên | Đọc cấu hình |
| POST | `/api/v1/node-servers/:id/warp/generate-config` | Quản trị viên | Tạo cấu hình |
| POST | `/api/v1/node-servers/:id/warp/register` | Quản trị viên | Đăng ký |
| GET | `/api/v1/node-servers/:id/warp/status` | Quản trị viên | Trạng thái |
| GET | `/api/v1/node-servers/:id/xray/control/balancer/:tag` | Quản trị viên | Nhận thông tin cân bằng |
| POST | `/api/v1/node-servers/:id/xray/control/balancer/:tag` | Quản trị viên | Đặt mục tiêu cân bằng |
| POST | `/api/v1/node-servers/:id/xray/control/firewall/apply` | Quản trị viên | Áp dụng quy tắc tường lửa nhảy cổng |
| POST | `/api/v1/node-servers/:id/xray/control/firewall/remove` | Quản trị viên | Loại bỏ các quy tắc tường lửa nhảy cổng |
| POST | `/api/v1/node-servers/:id/xray/control/inbound` | Quản trị viên | Thêm vào |
| DELETE | `/api/v1/node-servers/:id/xray/control/inbound/:tag` | Quản trị viên | Xóa thư đến |
| POST | `/api/v1/node-servers/:id/xray/control/inbound/:tag/user` | Quản trị viên | Thêm người dùng |
| DELETE | `/api/v1/node-servers/:id/xray/control/inbound/:tag/user/:email` | Quản trị viên | Xóa người dùng |
| POST | `/api/v1/node-servers/:id/xray/control/outbound` | Quản trị viên | Thêm đi |
| DELETE | `/api/v1/node-servers/:id/xray/control/outbound/:tag` | Quản trị viên | Xóa đi |
| POST | `/api/v1/node-servers/:id/xray/control/routing` | Quản trị viên | Yêu cầu tuyến đường |
| POST | `/api/v1/node-servers/:id/xray/control/routing/rule` | Quản trị viên | Thêm quy tắc định tuyến |
| DELETE | `/api/v1/node-servers/:id/xray/control/routing/rule/:tag` | Quản trị viên | Xóa quy tắc định tuyến |
| GET | `/api/v1/node-servers/:id/xray/control/routing/rules` | Quản trị viên | Liệt kê các quy tắc định tuyến |
| POST | `/api/v1/node-servers/:id/xray/control/sync` | Quản trị viên | Kích hoạt đồng bộ hóa cấu hình xray theo cách thủ công |
| GET | `/api/v1/node-servers/:id/xray/stats` | Quản trị viên | Nhận số liệu thống kê |
| POST | `/api/v1/node-servers/:id/xray/stats/reset/:email` | Quản trị viên | Đặt lại số liệu thống kê người dùng |
| GET | `/api/v1/node-servers/:id/xray/stats/users` | Quản trị viên | Nhận số liệu thống kê người dùng |
| GET | `/api/v1/node-servers/billable` | Quản trị viên | Danh sách nút có thể thanh toán |
| GET | `/api/v1/node-servers/ca-cert` | Quản trị viên | chứng chỉ CA |
| GET | `/api/v1/node-servers/ca-status` | Quản trị viên | trạng thái CA |
| GET | `/api/v1/node-servers/dashboard` | Quản trị viên | Trạng thái trang tổng quan |
| POST | `/api/v1/node-servers/distribute-cert` | Quản trị viên | Phân phối chứng chỉ cho tất cả các nút |
| POST | `/api/v1/node-servers/failover-check` | Quản trị viên | Kiểm tra chuyển đổi dự phòng kích hoạt |
| GET | `/api/v1/node-servers/metrics/aggregated` | Quản trị viên | Số liệu tổng hợp |
| POST | `/api/v1/node-servers/migrate-cert` | Quản trị viên | Di chuyển chứng chỉ |
| GET | `/api/v1/node-servers/online-summary` | Quản trị viên | Tóm tắt trực tuyến |
| GET | `/api/v1/node-servers/online-users` | Quản trị viên | Tất cả người dùng trực tuyến |
| GET | `/api/v1/node-servers/pending-agent-updates` | Quản trị viên | Đại lý đang chờ nâng cấp |
| POST | `/api/v1/node-servers/recommend-region` | Quản trị viên | Khu vực được đề xuất |
| POST | `/api/v1/node-servers/select-nodes` | Quản trị viên | Chọn các nút để đăng ký |
| POST | `/api/v1/node-servers/speed-test-all` | Quản trị viên | Kiểm tra tốc độ tất cả các nút |
| GET | `/api/v1/node-servers/speed-test-results` | Quản trị viên | Kết quả kiểm tra tốc độ |
| GET | `/api/v1/node-servers/traffic/stats` | Quản trị viên | Thống kê lưu lượng truy cập |
| POST | `/api/v1/node-servers/update-all-agents` | Quản trị viên | Nâng cấp tất cả Đại lý |
| DELETE | `/api/v1/node-servers/users/:client_id` | Quản trị viên | Đồng bộ-xóa khách hàng |
| GET | `/api/v1/node-servers/users/retry-queue` | Quản trị viên | Thử lại trạng thái hàng đợi |
| POST | `/api/v1/node-servers/users/retry-queue/process` | Quản trị viên | Hàng đợi thử lại quy trình |
| POST | `/api/v1/node-servers/users/sync` | Quản trị viên | Đồng bộ hóa một khách hàng |
| POST | `/api/v1/node-servers/validate-chain-config` | Quản trị viên | Xác thực cấu hình chuỗi |

### Nhật ký kiểm tra nút

Kiểm tra và xuất hoạt động phía nút.

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| GET | `/api/v1/node-audit-logs` | Đã đăng nhập | Nhật ký kiểm tra nút |
| GET | `/api/v1/node-audit-logs/export` | Đã đăng nhập | Xuất nhật ký kiểm tra nút |

### Nút (mô hình cũ, đã ngừng hoạt động)

> `/api/v1/nodes` CRUD đã ngừng hoạt động vào ngày 20-10-2026: các nút được quản lý thống nhất thông qua `/api/v1/node-servers`.
> Truy vấn của người dùng trực tuyến vẫn ở `/api/v1/node-servers/{online-users,online-summary,:id/online-users}`.

### Tổng quan khu vực

Chế độ xem nút được tổng hợp theo khu vực.

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| GET | `/api/v1/regions/:region/servers` | Đã đăng nhập | Danh sách nút cho khu vực này |
| GET | `/api/v1/regions/overview` | Đã đăng nhập | Tổng quan khu vực |

### Đăng ký

Quản lý hồ sơ đăng ký: thu hồi, xoay mã thông báo, nhật ký truy cập, đặt lại mức sử dụng.

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| GET | `/api/v1/subscriptions` | Đã đăng nhập | Danh sách |
| POST | `/api/v1/subscriptions` | Đã đăng nhập | Tạo |
| DELETE | `/api/v1/subscriptions/:id` | Đã đăng nhập | Xóa |
| GET | `/api/v1/subscriptions/:id` | Đã đăng nhập | Chi tiết |
| PUT | `/api/v1/subscriptions/:id` | Đã đăng nhập | cập nhật |
| GET | `/api/v1/subscriptions/:id/access-logs` | Đã đăng nhập | Nhật ký truy cập |
| POST | `/api/v1/subscriptions/:id/enable` | Đã đăng nhập | Kích hoạt |
| POST | `/api/v1/subscriptions/:id/reset-usage` | Đã đăng nhập | Đặt lại mức sử dụng |
| POST | `/api/v1/subscriptions/:id/restore` | Đã đăng nhập | Tiếp tục |
| POST | `/api/v1/subscriptions/:id/revoke` | Đã đăng nhập | Thu hồi |
| POST | `/api/v1/subscriptions/:id/rotate` | Đã đăng nhập | Xoay |
| GET | `/api/v1/subscriptions/:id/rotations` | Đã đăng nhập | Lịch sử xoay vòng |

### Định dạng đăng ký

> `/api/v1/subscription-formats` CRUD phía quản trị viên đã ngừng hoạt động vào ngày 20-10-2026: điểm vào thực tế là
> `/api/v1/mihomo/configs` (trung tâm định dạng đăng ký). Bảng `subscription_formats` vẫn còn,
> chỉ được sử dụng làm dữ liệu gốc cho điểm cuối `/api/v1/public/client-configs` công khai.

### Điểm cuối đăng ký

Miền đăng ký chuyên dụng (được ủy quyền ngược với đường dẫn `/sub` của bảng điều khiển).

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| GET | `/api/v1/subscription-endpoints` | Đã đăng nhập | Danh sách |
| POST | `/api/v1/subscription-endpoints` | Đã đăng nhập | Tạo |
| DELETE | `/api/v1/subscription-endpoints/:id` | Đã đăng nhập | Xóa |
| GET | `/api/v1/subscription-endpoints/:id` | Đã đăng nhập | Chi tiết |
| PUT | `/api/v1/subscription-endpoints/:id` | Đã đăng nhập | cập nhật |

### Chia sẻ tên miền

Truy cập nhóm miền/IP được sử dụng trong các liên kết máy khách.

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| GET | `/api/v1/share-domains` | Đã đăng nhập | Danh sách |
| POST | `/api/v1/share-domains` | Đã đăng nhập | Tạo |
| DELETE | `/api/v1/share-domains/:id` | Đã đăng nhập | Xóa |
| PUT | `/api/v1/share-domains/:id` | Đã đăng nhập | cập nhật |
| DELETE | `/api/v1/share-domains/all` | Đã đăng nhập | Xóa tất cả |
| POST | `/api/v1/share-domains/seed` | Đã đăng nhập | Khởi tạo dữ liệu mặc định |

### IP Cloudflare

Quản lý nguồn và nhóm IP Cloudflare CDN.

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| DELETE | `/api/v1/cfip/entries` | Đã đăng nhập | Xóa mục nhập |
| GET | `/api/v1/cfip/entries` | Đã đăng nhập | Danh sách dự thi |
| POST | `/api/v1/cfip/entries` | Đã đăng nhập | Thêm mục |
| DELETE | `/api/v1/cfip/entries/:id` | Đã đăng nhập | Xóa mục nhập |
| POST | `/api/v1/cfip/fetch-all` | Đã đăng nhập | Tìm nạp tất cả các nguồn |
| POST | `/api/v1/cfip/seed` | Đã đăng nhập | Khởi tạo dữ liệu mặc định |
| GET | `/api/v1/cfip/sources` | Đã đăng nhập | Danh sách nguồn dữ liệu |
| POST | `/api/v1/cfip/sources` | Đã đăng nhập | Tạo nguồn dữ liệu |
| DELETE | `/api/v1/cfip/sources/:id` | Đã đăng nhập | Xóa nguồn dữ liệu |
| PUT | `/api/v1/cfip/sources/:id` | Đã đăng nhập | Cập nhật nguồn dữ liệu |
| POST | `/api/v1/cfip/sources/:id/fetch` | Đã đăng nhập | Lấy nguồn này |

### Cấu hình đăng ký (Mihomo/Clash)

Cấu hình mẫu đăng ký: nhóm, quy tắc, nhà cung cấp quy tắc, bản xem trước YAML.

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| GET | `/api/v1/mihomo/configs` | Đã đăng nhập | Danh sách |
| POST | `/api/v1/mihomo/configs` | Đã đăng nhập | Tạo |
| DELETE | `/api/v1/mihomo/configs/:id` | Đã đăng nhập | Xóa |
| GET | `/api/v1/mihomo/configs/:id` | Đã đăng nhập | Chi tiết |
| PUT | `/api/v1/mihomo/configs/:id` | Đã đăng nhập | cập nhật |
| GET | `/api/v1/mihomo/configs/:id/proxy-groups` | Đã đăng nhập | Danh sách nhóm |
| POST | `/api/v1/mihomo/configs/:id/proxy-groups` | Đã đăng nhập | Tạo nhóm |
| GET | `/api/v1/mihomo/configs/:id/rule-providers` | Đã đăng nhập | Danh sách nhà cung cấp quy tắc |
| POST | `/api/v1/mihomo/configs/:id/rule-providers` | Đã đăng nhập | Tạo nhà cung cấp quy tắc |
| GET | `/api/v1/mihomo/configs/:id/rules` | Đã đăng nhập | Danh sách quy tắc |
| POST | `/api/v1/mihomo/configs/:id/rules` | Đã đăng nhập | Tạo quy tắc |
| GET | `/api/v1/mihomo/configs/:id/yaml` | Đã đăng nhập | Xem trước được tạo YAML |
| DELETE | `/api/v1/mihomo/proxy-groups/:id` | Đã đăng nhập | Xóa nhóm |
| DELETE | `/api/v1/mihomo/rule-providers/:id` | Đã đăng nhập | Xóa nhà cung cấp quy tắc |
| DELETE | `/api/v1/mihomo/rules/:id` | Đã đăng nhập | Xóa quy tắc |

### Trang web nạp tiền

Tích hợp trang thanh toán của bên thứ ba: khóa ký, tệp xác minh, xác minh tên miền, tài liệu tích hợp.

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| GET | `/api/v1/charge-sites` | Đã đăng nhập | Danh sách trang web |
| POST | `/api/v1/charge-sites` | Đã đăng nhập | Tạo trang web |
| DELETE | `/api/v1/charge-sites/:id` | Đã đăng nhập | Xóa trang web |
| GET | `/api/v1/charge-sites/:id` | Đã đăng nhập | Chi tiết trang web |
| PUT | `/api/v1/charge-sites/:id` | Đã đăng nhập | Cập nhật trang web |
| POST | `/api/v1/charge-sites/:id/clear-prev-secret` | Đã đăng nhập | Xóa chìa khóa cũ |
| POST | `/api/v1/charge-sites/:id/generate-verify` | Đã đăng nhập | Tạo tập tin xác minh |
| POST | `/api/v1/charge-sites/:id/generate-verify-file` | Đã đăng nhập | Tạo tập tin xác minh |
| GET | `/api/v1/charge-sites/:id/integration-doc` | Đã đăng nhập | Nhận tài liệu tích hợp |
| POST | `/api/v1/charge-sites/:id/rotate-secret` | Đã đăng nhập | Xoay phím ký |
| POST | `/api/v1/charge-sites/:id/verify-dns` | Đã đăng nhập | Xác minh tên miền DNS |
| POST | `/api/v1/charge-sites/:id/verify-domain` | Đã đăng nhập | Xác minh thử thách tên miền HTTP |
| POST | `/api/v1/charge-sites/:id/verify-domain-dns` | Đã đăng nhập | Xác minh tên miền DNS |
| POST | `/api/v1/charge-sites/:id/verify-domain-http` | Đã đăng nhập | Xác minh thử thách tên miền HTTP |

### Nạp tiền đơn hàng

Truy vấn nạp tiền đơn hàng, hoàn tiền, xuất CSV.

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| GET | `/api/v1/charge-orders` | Đã đăng nhập | Danh sách đặt hàng |
| GET | `/api/v1/charge-orders/:id` | Đã đăng nhập | Chi tiết đặt hàng |
| POST | `/api/v1/charge-orders/:id/refund` | Đã đăng nhập | Hoàn tiền |
| GET | `/api/v1/charge-orders/export-csv` | Đã đăng nhập | Xuất lệnh nạp tiền CSV |

### Nạp thẻ

Tạo thẻ số lượng lớn, hồ sơ sử dụng, xuất CSV.

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| GET | `/api/v1/charge-cards` | Đã đăng nhập | Danh sách thẻ |
| POST | `/api/v1/charge-cards` | Đã đăng nhập | Tạo thẻ |
| DELETE | `/api/v1/charge-cards/:id` | Đã đăng nhập | Xóa thẻ |
| GET | `/api/v1/charge-cards/:id/usages` | Đã đăng nhập | Hồ sơ sử dụng thẻ |
| POST | `/api/v1/charge-cards/batch` | Đã đăng nhập | Tạo thẻ hàng loạt |
| GET | `/api/v1/charge-cards/export-csv` | Đã đăng nhập | Xuất thẻ CSV |

### Tính phí sự kiện gọi lại

Kiểm tra sự kiện gọi lại.

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| GET | `/api/v1/charge-callback-events` | Đã đăng nhập | Danh sách sự kiện gọi lại |

### Tính phí cuộc gọi lại lần thử

Thử lại thủ công các cuộc gọi lại không thành công.

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| GET | `/api/v1/charge-callback-retries` | Đã đăng nhập | Danh sách gọi lại không thành công |
| POST | `/api/v1/charge-callback-retries/:id/retry` | Quản trị viên | Thử gọi lại |

### Số dư của khách hàng

Chế độ xem của quản trị viên về số dư và giao dịch.

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| GET | `/api/v1/client-balances` | Đã đăng nhập | Danh sách số dư của khách hàng |
| GET | `/api/v1/client-balances/transactions` | Đã đăng nhập | Giao dịch số dư (quản trị viên) |
| GET | `/api/v1/client-balances/transactions/export-csv` | Đã đăng nhập | Giao dịch số dư xuất khẩu CSV |

### Quà tặng

Phát hành quà tặng có thể yêu cầu (ngày/lượng truy cập).

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| GET | `/api/v1/client-gifts` | Đã đăng nhập | Danh sách quà tặng |
| POST | `/api/v1/client-gifts` | Đã đăng nhập | Tạo quà tặng |
| POST | `/api/v1/client-gifts/batch` | Đã đăng nhập | Tạo quà tặng số lượng lớn |
| GET | `/api/v1/client-gifts/search-clients` | Đã đăng nhập | Tìm kiếm khách hàng |

### Cấu hình giới thiệu

Cấu hình và thống kê hoa hồng giới thiệu.

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| GET | `/api/v1/referral/config` | Đã đăng nhập | Cấu hình giới thiệu |
| PUT | `/api/v1/referral/config` | Quản trị viên | Lưu cấu hình giới thiệu |
| GET | `/api/v1/referral/records` | Đã đăng nhập | Danh sách hồ sơ giới thiệu |
| GET | `/api/v1/referral/stats` | Đã đăng nhập | Thống kê giới thiệu |

### Rút tiền giới thiệu

Xem xét các yêu cầu rút tiền.

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| GET | `/api/v1/referral-withdrawals` | Đã đăng nhập | Danh sách yêu cầu rút tiền |
| POST | `/api/v1/referral-withdrawals/:id/approve` | Đã đăng nhập | Phê duyệt rút tiền |
| POST | `/api/v1/referral-withdrawals/:id/reject` | Đã đăng nhập | Từ chối rút tiền |

### Danh sách cho phép IP (tóm tắt)

Chế độ xem danh sách cho phép IP trên tất cả khách hàng.

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| GET | `/api/v1/ip-whitelists` | Đã đăng nhập | Tất cả danh sách cho phép IP của khách hàng |

### Ràng buộc OAuth của khách hàng

Truy vấn các ràng buộc tài khoản bên thứ ba của khách hàng.

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| GET | `/api/v1/client-oauth-bindings` | Đã đăng nhập | Danh sách ràng buộc OAuth của khách hàng |

### Vé (quản trị viên)

Trả lời vé, phân công và đóng cửa.

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| GET | `/api/v1/tickets` | Đã đăng nhập | Danh sách vé |
| GET | `/api/v1/tickets/:id` | Đã đăng nhập | Chi tiết vé |
| POST | `/api/v1/tickets/:id/assign` | Đã đăng nhập | Chỉ định vé |
| POST | `/api/v1/tickets/:id/close` | Đã đăng nhập | Đóng vé |
| POST | `/api/v1/tickets/:id/reply` | Đã đăng nhập | Trả lời vé |

### bài viết

CRUD cho các bài viết trên cổng thông tin.

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| GET | `/api/v1/articles` | Đã đăng nhập | Danh sách bài viết |
| POST | `/api/v1/articles` | Đã đăng nhập | Tạo bài viết |
| DELETE | `/api/v1/articles/:id` | Đã đăng nhập | Xóa bài viết |
| GET | `/api/v1/articles/:id` | Đã đăng nhập | Chi tiết bài viết |
| PUT | `/api/v1/articles/:id` | Đã đăng nhập | Cập nhật bài viết |

### Tài khoản được chia sẻ

Hồ sơ phát hành và sửa chữa cho các tài khoản dùng chung (phát trực tuyến, v.v.).

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| GET | `/api/v1/shared-accounts` | Đã đăng nhập | Danh sách tài khoản được chia sẻ |
| POST | `/api/v1/shared-accounts` | Đã đăng nhập | Tạo tài khoản chia sẻ |
| DELETE | `/api/v1/shared-accounts/:id` | Đã đăng nhập | Xóa tài khoản dùng chung |
| GET | `/api/v1/shared-accounts/:id` | Đã đăng nhập | Chi tiết tài khoản được chia sẻ |
| PUT | `/api/v1/shared-accounts/:id` | Đã đăng nhập | Cập nhật tài khoản chia sẻ |
| GET | `/api/v1/shared-accounts/:id/repair-logs` | Đã đăng nhập | Sửa chữa hồ sơ |

### Chứng chỉ

Phát hành, tải lên, gia hạn và xuất chứng chỉ công cộng (ACME); hỗ trợ phát hành trên một nút cụ thể.

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| GET | `/api/v1/certificates` | Đã đăng nhập | Danh sách |
| POST | `/api/v1/certificates` | Đã đăng nhập | Tạo |
| DELETE | `/api/v1/certificates/:id` | Đã đăng nhập | Xóa |
| GET | `/api/v1/certificates/:id` | Đã đăng nhập | Chi tiết |
| PUT | `/api/v1/certificates/:id` | Đã đăng nhập | cập nhật |
| GET | `/api/v1/certificates/:id/download` | Đã đăng nhập | Tải xuống |
| POST | `/api/v1/certificates/:id/renew` | Đã đăng nhập | Gia hạn khách hàng |
| GET | `/api/v1/certificates/check-port` | Đã đăng nhập | Kiểm tra cổng |
| GET | `/api/v1/certificates/detect-ip` | Đã đăng nhập | Thăm dò IP |
| GET | `/api/v1/certificates/export` | Đã đăng nhập | Xuất khẩu |
| POST | `/api/v1/certificates/issue` | Đã đăng nhập | Vấn đề. Thân máy: `{"domains":["example.com","*.example.com"],"challenge_type":"dns-01","dns_channel_id":2,"key_type":"rsa","key_bits":4096}`. `challenge_type` mặc định là http-01; `dns_channel_id` chọn kênh DNS (nhà cung cấp DNS đã định cấu hình + thông tin xác thực) |
| POST | `/api/v1/certificates/issue-on-node` | Đã đăng nhập | Vấn đề với nút |
| POST | `/api/v1/certificates/upload` | Đã đăng nhập | Tải lên |

### Kênh chứng chỉ

Cấu hình kênh (CA) ACME.

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| GET | `/api/v1/cert-channels` | Đã đăng nhập | Danh sách |
| POST | `/api/v1/cert-channels` | Đã đăng nhập | Tạo |
| DELETE | `/api/v1/cert-channels/:id` | Đã đăng nhập | Xóa |
| PUT | `/api/v1/cert-channels/:id` | Đã đăng nhập | cập nhật |
| GET | `/api/v1/cert-channels/providers` | Đã đăng nhập | nhà cung cấp |

### Cấu hình EAB

Thông tin xác thực ràng buộc các tài khoản bên ngoài như ZeroSSL / Google.

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| GET | `/api/v1/eab-configs` | Đã đăng nhập | Danh sách |
| POST | `/api/v1/eab-configs` | Đã đăng nhập | Tạo |
| DELETE | `/api/v1/eab-configs/:id` | Đã đăng nhập | Xóa |
| PUT | `/api/v1/eab-configs/:id` | Đã đăng nhập | cập nhật |
| POST | `/api/v1/eab-configs/fetch-google` | Đã đăng nhập | Tìm nạp Google EAB từ GCP |
| POST | `/api/v1/eab-configs/fetch-zerossl` | Đã đăng nhập | Tìm nạp ZeroSSL EAB bằng email chỉ bằng một cú nhấp chuột |

### Tài khoản ACME

Quản lý tài khoản ACME (email liên hệ) phía bảng điều khiển.

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| GET | `/api/v1/acme-accounts` | Đã đăng nhập | Danh sách |
| POST | `/api/v1/acme-accounts` | Đã đăng nhập | Tạo |
| DELETE | `/api/v1/acme-accounts/:id` | Đã đăng nhập | Xóa |
| PUT | `/api/v1/acme-accounts/:id` | Đã đăng nhập | cập nhật |
| GET | `/api/v1/acme-accounts/default` | Đã đăng nhập | Nhận mặc định |

### CA tích hợp

Bảng điều khiển CA nội bộ: Cấp/gia hạn/thu hồi chứng chỉ đại lý, xuất CRL, hàng đợi phân phối.

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| GET | `/api/v1/ca/agent-certs` | Đã đăng nhập | Danh sách chứng chỉ đại lý |
| POST | `/api/v1/ca/agent-certs/:server_id/renew` | Đã đăng nhập | Gia hạn chứng chỉ đại lý |
| GET | `/api/v1/ca/distribution-queue` | Đã đăng nhập | Hàng đợi phân phối chứng chỉ |
| DELETE | `/api/v1/ca/distribution-queue/:id` | Đã đăng nhập | Xóa mục nhập hàng đợi phân phối |
| POST | `/api/v1/ca/distribution-queue/:id/retry` | Đã đăng nhập | Thử lại việc phân phối chứng chỉ |
| GET | `/api/v1/ca/revocations` | Đã đăng nhập | Danh sách thu hồi |
| GET | `/api/v1/ca/revocations/export` | Đã đăng nhập | Xuất CRL |
| GET | `/api/v1/ca/status` | Đã đăng nhập | trạng thái CA |

### Quy tắc địa lý

Đồng bộ hóa và phân phối các nguồn và tệp dữ liệu geoip/geosite.

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| GET | `/api/v1/geo-rules/files` | Đã đăng nhập | Danh sách tập tin |
| DELETE | `/api/v1/geo-rules/files/:fileId` | Đã đăng nhập | Xóa tập tin |
| PUT | `/api/v1/geo-rules/files/:fileId/auto-sync` | Đã đăng nhập | Chuyển đổi tự động đồng bộ hóa |
| POST | `/api/v1/geo-rules/files/:fileId/download` | Đã đăng nhập | Tải xuống |
| GET | `/api/v1/geo-rules/files/:fileId/download-file` | Đã đăng nhập | Tải tập tin xuống |
| GET | `/api/v1/geo-rules/github-assets` | Đã đăng nhập | Tìm nạp nội dung kho lưu trữ GitHub |
| POST | `/api/v1/geo-rules/push-all` | Đã đăng nhập | Đẩy tới tất cả các nút |
| POST | `/api/v1/geo-rules/seed` | Đã đăng nhập | Khởi tạo dữ liệu mặc định |
| GET | `/api/v1/geo-rules/sources` | Đã đăng nhập | Danh sách nguồn dữ liệu |
| POST | `/api/v1/geo-rules/sources` | Đã đăng nhập | Tạo nguồn dữ liệu |
| DELETE | `/api/v1/geo-rules/sources/:sourceId` | Đã đăng nhập | Xóa nguồn dữ liệu |
| PUT | `/api/v1/geo-rules/sources/:sourceId` | Đã đăng nhập | Cập nhật nguồn dữ liệu |
| GET | `/api/v1/geo-rules/sources/:sourceId/assets` | Đã đăng nhập | Tìm nạp nội dung phát hành |
| POST | `/api/v1/geo-rules/sources/:sourceId/files` | Đã đăng nhập | Thêm tập tin |
| PUT | `/api/v1/geo-rules/sources/:sourceId/schedule` | Đã đăng nhập | Chuyển đổi đồng bộ hóa theo lịch trình |
| POST | `/api/v1/geo-rules/sources/:sourceId/select` | Đã đăng nhập | Chọn tập tin |
| POST | `/api/v1/geo-rules/sources/:sourceId/sync` | Đã đăng nhập | Đồng bộ hóa nguồn dữ liệu này |
| GET | `/api/v1/geo-rules/status` | Đã đăng nhập | Trạng thái |
| POST | `/api/v1/geo-rules/sync-all` | Đã đăng nhập | Đồng bộ hóa tất cả |
| GET | `/api/v1/geo-rules/sync-status/:key` | Đã đăng nhập | Tiến trình đồng bộ hóa |

### Thư viện quy tắc WAF

Quy tắc WAF toàn cầu: Nguồn GitHub, đồng bộ hóa, phân phối đến tất cả các nút.

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| GET | `/api/v1/waf/github-source` | Đã đăng nhập | Nguồn dữ liệu GitHub |
| PUT | `/api/v1/waf/github-source` | Đã đăng nhập | Lưu nguồn dữ liệu GitHub |
| POST | `/api/v1/waf/push-all` | Đã đăng nhập | Đẩy tới tất cả các nút |
| GET | `/api/v1/waf/rules` | Đã đăng nhập | Danh sách quy tắc |
| DELETE | `/api/v1/waf/rules/:name` | Đã đăng nhập | Xóa quy tắc |
| GET | `/api/v1/waf/rules/:name` | Đã đăng nhập | Nhận quy tắc |
| PUT | `/api/v1/waf/rules/:name` | Đã đăng nhập | Lưu quy tắc |
| POST | `/api/v1/waf/sync-all` | Đã đăng nhập | Đồng bộ hóa tất cả |
| POST | `/api/v1/waf/sync-github` | Đã đăng nhập | Đồng bộ hóa từ GitHub |

### Quản lý nhật ký

Đọc và phát trực tiếp nhật ký nút/lõi, chính sách dọn dẹp, kiểm soát trình ghi nhật ký.

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| POST | `/api/v1/log-manage/cleanup-all` | Đã đăng nhập | Dọn dẹp mọi thứ ngay bây giờ |
| GET | `/api/v1/log-manage/cleanup-rules` | Đã đăng nhập | Danh sách quy tắc dọn dẹp |
| PUT | `/api/v1/log-manage/cleanup-rules/:id` | Đã đăng nhập | Cập nhật quy tắc dọn dẹp |
| POST | `/api/v1/log-manage/cleanup-table/:table` | Đã đăng nhập | Dọn dẹp bảng được chỉ định ngay bây giờ |
| GET | `/api/v1/log-manage/core-log-output` | Đã đăng nhập | Cấu hình đầu ra nhật ký lõi |
| PUT | `/api/v1/log-manage/core-log-output` | Đã đăng nhập | Lưu cấu hình đầu ra nhật ký lõi |
| GET | `/api/v1/log-manage/core/:type/stdout-log` | Đã đăng nhập | Đọc nhật ký thiết bị xuất chuẩn cốt lõi |
| GET | `/api/v1/log-manage/core/:type/stdout-stream` | Đã đăng nhập | Truyền phát lõi thiết bị xuất chuẩn |
| POST | `/api/v1/log-manage/discover` | Đã đăng nhập | Khám phá các tệp nhật ký nút |
| GET | `/api/v1/log-manage/log-output` | Đã đăng nhập | Cấu hình đầu ra nhật ký |
| PUT | `/api/v1/log-manage/log-output` | Đã đăng nhập | Lưu cấu hình đầu ra nhật ký |
| POST | `/api/v1/log-manage/log-output/release/:type` | Đã đăng nhập | Tiếp quản thiết bị xuất chuẩn nhật ký phát hành |
| POST | `/api/v1/log-manage/log-output/takeover/:type` | Đã đăng nhập | Tiếp quản thiết bị xuất chuẩn nhật ký |
| GET | `/api/v1/log-manage/nodes/:serverId/logs` | Đã đăng nhập | Danh sách tệp nhật ký nút |
| GET | `/api/v1/log-manage/nodes/:serverId/logs/:type` | Đã đăng nhập | Đọc nhật ký nút |
| POST | `/api/v1/log-manage/nodes/:serverId/logs/:type/rotate` | Đã đăng nhập | Xoay nhật ký nút |
| GET | `/api/v1/log-manage/nodes/:serverId/logs/:type/stream` | Đã đăng nhập | Nhật ký nút truyền phát |
| GET | `/api/v1/log-manage/policies` | Đã đăng nhập | Danh sách chính sách |
| POST | `/api/v1/log-manage/policies` | Đã đăng nhập | Tạo chính sách |
| DELETE | `/api/v1/log-manage/policies/:id` | Đã đăng nhập | Xóa chính sách |
| GET | `/api/v1/log-manage/policies/:id` | Đã đăng nhập | Chi tiết chính sách |
| PUT | `/api/v1/log-manage/policies/:id` | Đã đăng nhập | Cập nhật chính sách |
| GET | `/api/v1/log-manage/policies/:id/jobs` | Đã đăng nhập | Danh sách công việc dọn dẹp |
| GET | `/api/v1/log-manage/policies/:id/log` | Đã đăng nhập | Đọc nhật ký |
| GET | `/api/v1/log-manage/policies/:id/log-stream` | Đã đăng nhập | Nhật ký phát trực tuyến |
| POST | `/api/v1/log-manage/policies/:id/rotate` | Đã đăng nhập | Xoay |
| GET | `/api/v1/log-manage/proxy/:type/stdout-log` | Đã đăng nhập | Đọc nhật ký thiết bị xuất chuẩn cốt lõi |
| GET | `/api/v1/log-manage/proxy/:type/stdout-stream` | Đã đăng nhập | Truyền phát lõi thiết bị xuất chuẩn |
| GET | `/api/v1/log-manage/scheduler-config` | Đã đăng nhập | Cấu hình lịch trình |
| PUT | `/api/v1/log-manage/scheduler-config` | Đã đăng nhập | Cập nhật cấu hình lập lịch |
| GET | `/api/v1/log-manage/status` | Đã đăng nhập | Trạng thái |
| POST | `/api/v1/log-manage/sync` | Đã đăng nhập | Đồng bộ hóa chính sách dọn dẹp |
| GET | `/api/v1/log-manage/writers` | Đã đăng nhập | trạng thái nhà văn |
| PUT | `/api/v1/log-manage/writers/:type/config` | Đã đăng nhập | Cập nhật cấu hình người viết |
| GET | `/api/v1/log-manage/writers/:type/files` | Đã đăng nhập | Danh sách tệp nhật ký của người viết |
| POST | `/api/v1/log-manage/writers/:type/start` | Đã đăng nhập | Bắt đầu viết |
| POST | `/api/v1/log-manage/writers/:type/stop` | Đã đăng nhập | Dừng người viết |

### Sao lưu

Cơ sở dữ liệu/sao lưu đầy đủ, khôi phục, công việc theo lịch trình, lưu trữ từ xa FTP/S3/WebDAV.

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| DELETE | `/api/v1/backups/:id` | Quản trị viên | Xóa bản sao lưu |
| GET | `/api/v1/backups/:id/download` | Quản trị viên | Tải xuống bản sao lưu |
| POST | `/api/v1/backups/:id/restore` | Quản trị viên | Khôi phục từ bản sao lưu |
| POST | `/api/v1/backups/:id/upload-ftp` | Quản trị viên | Tải lên FTP |
| POST | `/api/v1/backups/:id/upload-s3` | Quản trị viên | Tải lên S3 |
| POST | `/api/v1/backups/:id/upload-webdav` | Quản trị viên | Tải lên WebDAV |
| POST | `/api/v1/backups/create` | Quản trị viên | Tạo bản sao lưu đầy đủ (với kết xuất DB) |
| GET | `/api/v1/backups/list` | Quản trị viên | Danh sách sao lưu đầy đủ |
| POST | `/api/v1/backups/remote-restore` | Quản trị viên | Khôi phục từ xa |
| GET | `/api/v1/backups/restore-status` | Quản trị viên | Khôi phục tiến trình |
| GET | `/api/v1/backups/schedules` | Quản trị viên | Danh sách công việc theo lịch trình |
| POST | `/api/v1/backups/schedules` | Quản trị viên | Tạo công việc theo lịch trình |
| DELETE | `/api/v1/backups/schedules/:id` | Quản trị viên | Xóa công việc đã lên lịch |
| PUT | `/api/v1/backups/schedules/:id` | Quản trị viên | Cập nhật công việc đã lên lịch |
| GET | `/api/v1/backups/storages` | Quản trị viên | Danh sách cấu hình lưu trữ |
| POST | `/api/v1/backups/storages` | Quản trị viên | Tạo cấu hình lưu trữ |
| DELETE | `/api/v1/backups/storages/:id` | Quản trị viên | Xóa cấu hình lưu trữ |
| PUT | `/api/v1/backups/storages/:id` | Quản trị viên | Cập nhật cấu hình lưu trữ |
| GET | `/api/v1/backups/storages/:id/files` | Quản trị viên | Danh sách tập tin sao lưu từ xa |
| POST | `/api/v1/backups/upload-restore` | Quản trị viên | Tải lên bản sao lưu và khôi phục |

### Cài đặt hệ thống

Cấu hình và di chuyển cơ sở dữ liệu, cài đặt bảng điều khiển, khóa JWT, kiểm tra SMTP/Telegram, mẫu thông báo, nâng cấp trực tuyến (chủ yếu là quản trị viên).

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| PUT | `/api/v1/system/bootstrap-finish` | Quản trị viên | Hoàn tất thiết lập khởi động lần đầu |
| GET | `/api/v1/system/bootstrap-state` | Quản trị viên | Trạng thái khởi động lần đầu |
| GET | `/api/v1/system/config-file/:coreType` | Quản trị viên | Đọc tập tin cấu hình lõi |
| GET | `/api/v1/system/db-config` | Quản trị viên | Cấu hình cơ sở dữ liệu |
| PUT | `/api/v1/system/db-config` | Quản trị viên | Cập nhật cấu hình cơ sở dữ liệu |
| POST | `/api/v1/system/db-config/migrate` | Quản trị viên | Di chuyển dữ liệu cơ sở dữ liệu |
| POST | `/api/v1/system/db-config/switch` | Quản trị viên | Chuyển đổi cơ sở dữ liệu (với di chuyển một bước) |
| POST | `/api/v1/system/db-config/test` | Quản trị viên | Kiểm tra kết nối cơ sở dữ liệu |
| GET | `/api/v1/system/db-pool-stats` | Quản trị viên | Thống kê nhóm kết nối |
| GET | `/api/v1/system/db-stats` | Quản trị viên | Thống kê cơ sở dữ liệu |
| GET | `/api/v1/system/db-stats/ping` | Quản trị viên | Thăm dò cơ sở dữ liệu |
| POST | `/api/v1/system/db-stats/reset` | Quản trị viên | Đặt lại số liệu thống kê cơ sở dữ liệu |
| GET | `/api/v1/system/encryption-key-status` | Quản trị viên | Trạng thái khóa mã hóa |
| GET | `/api/v1/system/env-config` | Quản trị viên | cấu hình .env |
| PUT | `/api/v1/system/env-config` | Quản trị viên | Cập nhật cấu hình .env |
| POST | `/api/v1/system/generate-encryption-key` | Quản trị viên | Tạo khóa mã hóa |
| GET | `/api/v1/system/github-config` | Quản trị viên | Cấu hình GitHub |
| PUT | `/api/v1/system/github-config` | Quản trị viên | Cập nhật cấu hình GitHub |
| GET | `/api/v1/system/jwt-secret-status` | Quản trị viên | Trạng thái khóa JWT |
| GET | `/api/v1/system/maintenance` | công khai | Trạng thái chế độ bảo trì (công khai) |
| PUT | `/api/v1/system/maintenance` | Quản trị viên | Chuyển đổi chế độ bảo trì |
| POST | `/api/v1/system/migrate-data` | Quản trị viên | Di chuyển dữ liệu |
| GET | `/api/v1/system/notification-config` | Quản trị viên | Cấu hình thông báo |
| PUT | `/api/v1/system/notification-config` | Quản trị viên | Cập nhật cấu hình thông báo |
| GET | `/api/v1/system/notification-templates` | Quản trị viên | Danh sách mẫu |
| POST | `/api/v1/system/notification-templates` | Quản trị viên | Tạo mẫu |
| DELETE | `/api/v1/system/notification-templates/:id` | Quản trị viên | Xóa mẫu |
| PUT | `/api/v1/system/notification-templates/:id` | Quản trị viên | Cập nhật mẫu |
| POST | `/api/v1/system/notification-templates/:id/broadcast` | Quản trị viên | Thông báo phát sóng theo mẫu |
| POST | `/api/v1/system/notification-templates/:id/send` | Quản trị viên | Gửi thông báo kiểm tra |
| GET | `/api/v1/system/panel-config` | Quản trị viên | Cấu hình bảng điều khiển |
| PUT | `/api/v1/system/panel-config` | Quản trị viên | Cập nhật cấu hình bảng điều khiển |
| GET | `/api/v1/system/reality-targets` | Quản trị viên | Danh sách mục tiêu REALITY |
| PUT | `/api/v1/system/reality-targets` | Quản trị viên | Cập nhật mục tiêu REALITY |
| POST | `/api/v1/system/reality-targets/reset` | Quản trị viên | Đặt lại mục tiêu REALITY về mặc định |
| POST | `/api/v1/system/regenerate-jwt-secret` | Quản trị viên | Tạo lại khóa JWT |
| GET | `/api/v1/system/releases` | Quản trị viên | Phiên bản có sẵn |
| POST | `/api/v1/system/restart` | Quản trị viên | Khởi động lại |
| POST | `/api/v1/system/rollback` | Quản trị viên | Khôi phục hệ thống |
| GET | `/api/v1/system/settings` | Quản trị viên | Đọc cài đặt |
| PUT | `/api/v1/system/settings/batch` | Quản trị viên | Cài đặt cập nhật hàng loạt |
| GET | `/api/v1/system/stats` | Quản trị viên | Trạng thái hệ thống |
| GET | `/api/v1/system/subscription-billing-templates` | Quản trị viên | Mẫu thanh toán đăng ký |
| PUT | `/api/v1/system/subscription-billing-templates` | Quản trị viên | Cập nhật mẫu thanh toán thuê bao |
| POST | `/api/v1/system/telegram-chat-id` | Quản trị viên | Nhận ID trò chuyện Telegram |
| POST | `/api/v1/system/test-smtp` | Quản trị viên | Kiểm tra SMTP |
| POST | `/api/v1/system/test-telegram` | Quản trị viên | Kiểm tra Telegram |
| POST | `/api/v1/system/update` | Quản trị viên | Nâng cấp hệ thống |
| GET | `/api/v1/system/update-jobs` | Quản trị viên | Nâng cấp danh sách công việc |
| POST | `/api/v1/system/upload-mtls-ca` | Quản trị viên | Tải lên chứng chỉ mTLS CA |

### Kiểm tra an ninh

Kiểm tra truy vấn sự kiện và xuất.

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| GET | `/api/v1/security/audit-events` | Đã đăng nhập | Danh sách sự kiện kiểm tra |
| GET | `/api/v1/security/audit-events/export` | Đã đăng nhập | Xuất sự kiện kiểm tra |

### Số liệu

Số liệu tổng hợp của bảng điều khiển (nút/chứng chỉ/dịch vụ).

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| GET | `/api/v1/metrics/certificates` | Đã đăng nhập | Số liệu chứng chỉ |
| GET | `/api/v1/metrics/nodes` | Đã đăng nhập | Số liệu nút |
| GET | `/api/v1/metrics/overview` | Đã đăng nhập | Tổng quan |

### Mẫu cổng thông tin

Mẫu cổng thông tin, cấu hình kích hoạt và quản lý bộ đệm.

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| GET | `/api/v1/portal/active` | Đã đăng nhập | Mẫu đang hoạt động |
| PUT | `/api/v1/portal/active` | Đã đăng nhập | Cập nhật mẫu đang hoạt động |
| DELETE | `/api/v1/portal/cache` | Đã đăng nhập | Xóa bộ đệm cổng thông tin |
| GET | `/api/v1/portal/cache` | Đã đăng nhập | Trạng thái bộ đệm |
| PUT | `/api/v1/portal/cache/dev-mode` | Đã đăng nhập | Chuyển đổi chế độ phát triển bộ đệm |
| POST | `/api/v1/portal/cache/rebuild` | Đã đăng nhập | Xây dựng lại bộ đệm cổng thông tin |
| POST | `/api/v1/portal/reset` | Đã đăng nhập | Khôi phục mẫu mặc định |
| GET | `/api/v1/portal/templates` | Đã đăng nhập | Danh sách |
| POST | `/api/v1/portal/templates` | Đã đăng nhập | Tạo |
| DELETE | `/api/v1/portal/templates/:id` | Đã đăng nhập | Xóa |
| GET | `/api/v1/portal/templates/:id` | Đã đăng nhập | Chi tiết |
| PUT | `/api/v1/portal/templates/:id` | Đã đăng nhập | cập nhật |
| POST | `/api/v1/portal/templates/:id/activate` | Đã đăng nhập | Kích hoạt mẫu |

> `/portal/templates` CRUD là **API nâng cao**: giao diện người dùng cổng thông tin chủ yếu sử dụng `PUT /api/v1/portal/active`
> (cấu hình toàn bộ trang web một lần); quản lý/chuyển đổi nhiều mẫu với nhóm này. Cả hai đều được hỗ trợ và không xung đột.

### Cài đặt đại lý

Tập lệnh cài đặt và tải xuống Node Agent (công khai).

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| POST | `/api/v1/agent/bootstrap` | công khai | Đăng ký lần khởi động đầu tiên của đại lý (Báo cáo đại lý bằng mã thông báo khởi động; công khai) |
| GET | `/api/v1/agent/releases` | Quản trị viên | Các phiên bản Đại lý có sẵn (dành cho trang nâng cấp) |
| GET | `/api/v1/agent/version` | Quản trị viên | Phiên bản Đại lý hiện tại trên bảng điều khiển |
| GET | `/api/v1/agent/download` | công khai | Tải đại lý |
| GET | `/api/v1/agent/install` | công khai | Nhận tập lệnh cài đặt |

### Tính phí mục gọi lại

Gọi lại nền tảng thanh toán của bên thứ ba (công khai, xác minh chữ ký).

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| POST | `/api/v1/charge/callback/:site_id` | công khai | Xử lý cuộc gọi lại tính phí |

### Đường vào (có nút)

Danh sách gửi đến được sử dụng để lựa chọn nút kế hoạch.

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| GET | `/api/v1/inbounds-with-server` | Đã đăng nhập | Tất cả các đường vào (bao gồm cả các nút của chúng) |

### Cấu hình công khai

Cấu hình thời gian chạy cổng thông tin mà không cần đăng nhập (bảo vệ bảo trì + giới hạn tỷ lệ công khai).

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| GET | `/api/v1/public/articles` | công khai | Danh sách bài viết công khai |
| GET | `/api/v1/public/articles/:slug` | công khai | Chi tiết bài viết công khai |
| GET | `/api/v1/public/client-configs` | công khai | Cấu hình tải xuống ứng dụng khách (công khai) |
| GET | `/api/v1/public/portal-config` | công khai | Cấu hình thời gian chạy cổng thông tin (tên trang web, chủ đề, ngôn ngữ, tài liệu tuân thủ; công khai) |
| GET | `/api/v1/public/settings` | công khai | Cài đặt công khai (ví dụ: register_enabled; public) |
| GET | `/api/v1/public/subscription-endpoints` | công khai | Danh sách điểm cuối đăng ký đã bật (công khai) |

### Xác thực khách hàng và doanh nghiệp (cùng được đặt trên công cụ quản trị)

Điểm cuối của khách hàng giống hệt với cổng thông tin; xem `portal-api.en.md` để biết chi tiết.

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| GET | `/api/v1/client-auth/articles` | công khai | Liệt kê các bài viết |
| GET | `/api/v1/client-auth/articles/:slug` | công khai | Nhận bài viết |
| GET | `/api/v1/client-auth/balance` | công khai | Nhận số dư |
| GET | `/api/v1/client-auth/balance/transactions` | công khai | Giao dịch số dư |
| GET | `/api/v1/client-auth/bills` | công khai | Danh sách đặt hàng |
| POST | `/api/v1/client-auth/change-password` | công khai | Thay đổi mật khẩu |
| POST | `/api/v1/client-auth/charge/create` | công khai | Tạo lệnh nạp tiền |
| GET | `/api/v1/client-auth/charge/orders` | công khai | Liệt kê các lệnh nạp tiền |
| GET | `/api/v1/client-auth/charge/orders/:id` | công khai | Nhận chi tiết đơn hàng nạp tiền |
| POST | `/api/v1/client-auth/charge/redeem-card` | công khai | Đổi thẻ |
| GET | `/api/v1/client-auth/charge/sites` | công khai | Liệt kê các trang web nạp tiền |
| GET | `/api/v1/client-auth/charge/status/:trade_no` | công khai | Truy vấn trạng thái nạp tiền |
| GET | `/api/v1/client-auth/checktime` | công khai | Nhận thời gian máy chủ (đồng bộ hóa đồng hồ) |
| GET | `/api/v1/client-auth/connection-info` | công khai | Nhận thông số kết nối kế hoạch |
| GET | `/api/v1/client-auth/gifts` | công khai | Danh sách quà tặng |
| POST | `/api/v1/client-auth/gifts/:id/claim` | công khai | Yêu cầu quà tặng |
| GET | `/api/v1/client-auth/invoices` | công khai | Danh sách đặt hàng |
| GET | `/api/v1/client-auth/invoices/:id` | công khai | Chi tiết đặt hàng |
| GET | `/api/v1/client-auth/ip-whitelist` | công khai | Liệt kê danh sách cho phép IP |
| DELETE | `/api/v1/client-auth/ip-whitelist/:id` | công khai | Xóa khỏi danh sách cho phép IP |
| POST | `/api/v1/client-auth/ip-whitelist/auto-add` | công khai | Tự động thêm IP |
| POST | `/api/v1/client-auth/login` | công khai | Đăng nhập |
| POST | `/api/v1/client-auth/logout` | công khai | Đăng xuất |
| POST | `/api/v1/client-auth/logs/package` | công khai | Tạo gói nhật ký |
| GET | `/api/v1/client-auth/logs/packages` | công khai | Liệt kê các gói nhật ký |
| DELETE | `/api/v1/client-auth/logs/packages/:id` | công khai | Xóa gói nhật ký |
| GET | `/api/v1/client-auth/logs/packages/:id` | công khai | Nhận gói nhật ký |
| GET | `/api/v1/client-auth/logs/packages/:id/download` | công khai | Tải xuống gói nhật ký |
| GET | `/api/v1/client-auth/logs/types` | công khai | Đăng nhập kiểm tra tình trạng gói |
| GET | `/api/v1/client-auth/me` | công khai | Nhận người dùng đã đăng nhập hiện tại |
| GET | `/api/v1/client-auth/nodes` | công khai | Danh sách nút của tôi |
| GET | `/api/v1/client-auth/nodes/:id/config` | công khai | Cấu hình nút |
| GET | `/api/v1/client-auth/nodes/:id/link` | công khai | Liên kết chia sẻ nút |
| GET | `/api/v1/client-auth/nodes/:id/qrcode` | công khai | Mã QR nút |
| GET | `/api/v1/client-auth/oauth/:provider` | công khai | Ủy quyền |
| GET | `/api/v1/client-auth/oauth/:provider/callback` | công khai | Gọi lại |
| GET | `/api/v1/client-auth/oauth/providers` | công khai | Trạng thái nhà cung cấp |
| GET | `/api/v1/client-auth/orders` | công khai | Danh sách đặt hàng |
| POST | `/api/v1/client-auth/orders` | công khai | Tạo đơn hàng |
| GET | `/api/v1/client-auth/orders/:id` | công khai | Chi tiết đặt hàng |
| GET | `/api/v1/client-auth/plans` | công khai | Danh sách kế hoạch |
| GET | `/api/v1/client-auth/plans/:id` | công khai | Chi tiết kế hoạch |
| GET | `/api/v1/client-auth/profile` | công khai | Nhận hồ sơ |
| PUT | `/api/v1/client-auth/profile` | công khai | Cập nhật hồ sơ |
| POST | `/api/v1/client-auth/profile/bind-oauth` | công khai | Liên kết tài khoản của bên thứ ba |
| DELETE | `/api/v1/client-auth/profile/unbind-oauth/:provider` | công khai | Hủy liên kết tài khoản của bên thứ ba |
| GET | `/api/v1/client-auth/referral/code` | công khai | Nhận mã xác minh giới thiệu |
| GET | `/api/v1/client-auth/referral/records` | công khai | Liệt kê hồ sơ giới thiệu |
| GET | `/api/v1/client-auth/referral/stats` | công khai | Nhận số liệu thống kê giới thiệu |
| POST | `/api/v1/client-auth/referral/withdraw` | công khai | Yêu cầu rút tiền |
| GET | `/api/v1/client-auth/referral/withdrawals` | công khai | Liệt kê các lần rút tiền giới thiệu |
| POST | `/api/v1/client-auth/refresh` | công khai | Làm mới |
| POST | `/api/v1/client-auth/register` | công khai | Đăng ký |
| POST | `/api/v1/client-auth/reset-subscription` | công khai | Đặt lại đăng ký |
| POST | `/api/v1/client-auth/rotate-token` | công khai | Xoay mã thông báo đăng ký |
| POST | `/api/v1/client-auth/self-unban` | công khai | Bỏ cấm tự phục vụ |
| PUT | `/api/v1/client-auth/settings` | công khai | Cập nhật cài đặt |
| POST | `/api/v1/client-auth/settings/auto-add-ip` | công khai | Tự động thêm IP |
| GET | `/api/v1/client-auth/settings/whitelist` | công khai | Liệt kê danh sách cho phép IP |
| DELETE | `/api/v1/client-auth/settings/whitelist/:id` | công khai | Xóa khỏi danh sách cho phép IP |
| GET | `/api/v1/client-auth/shared-accounts` | công khai | Liệt kê các tài khoản được chia sẻ |
| GET | `/api/v1/client-auth/shared-accounts/:id` | công khai | Nhận tài khoản chia sẻ |
| POST | `/api/v1/client-auth/shared-accounts/:id/apply` | công khai | Yêu cầu chia sẻ tài khoản |
| POST | `/api/v1/client-auth/shared-accounts/:id/repair` | công khai | Yêu cầu sửa chữa |
| POST | `/api/v1/client-auth/ticket` | công khai | Xuất vé ngắn hạn một lần |
| GET | `/api/v1/client-auth/tickets` | công khai | Liệt kê vé |
| POST | `/api/v1/client-auth/tickets` | công khai | Tạo vé |
| GET | `/api/v1/client-auth/tickets/:id` | công khai | Nhận vé |
| POST | `/api/v1/client-auth/tickets/:id/close` | công khai | Đóng vé |
| POST | `/api/v1/client-auth/tickets/:id/reply` | công khai | Trả lời vé |
| GET | `/api/v1/client-auth/traffic-history` | công khai | lịch sử giao thông |
| GET | `/api/v1/client-auth/traffic-packs` | công khai | Danh sách gói lưu lượng truy cập |
| POST | `/api/v1/client-auth/traffic-packs/buy` | công khai | Mua gói lưu lượng truy cập |

### Phân phối đăng ký

Điểm vào công khai để khách hàng tìm nạp đăng ký; xem `subscription.en.md`.

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| GET | `/sub/:token` | công khai | Phục vụ đăng ký |

### Quy trình cấu hình thời gian chạy

Cấu hình ảnh chụp nhanh / chạy khô / áp dụng / khôi phục (`apply`, `rollback` và `disable-proxy` yêu cầu quản trị viên).

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| POST | `/runtime/apply` | công khai | Áp dụng |
| GET | `/runtime/apply-jobs` | Đã đăng nhập | Liệt kê công việc |
| GET | `/runtime/apply-jobs/:id` | Đã đăng nhập | Nhận việc làm |
| POST | `/runtime/apply-jobs/:id/cancel` | công khai | Hủy công việc |
| POST | `/runtime/apply-jobs/:id/retry` | công khai | Thử lại công việc |
| POST | `/runtime/disable-proxy` | công khai | Tắt proxy |
| POST | `/runtime/dry-run` | Đã đăng nhập | Thực hiện chạy khô |
| GET | `/runtime/nodes` | Đã đăng nhập | Liệt kê các nút |
| GET | `/runtime/overview` | Đã đăng nhập | Tổng quan |
| POST | `/runtime/rollback` | công khai | Quay lại |
| GET | `/runtime/snapshots` | Đã đăng nhập | Liệt kê ảnh chụp nhanh |
| POST | `/runtime/snapshots` | Đã đăng nhập | Tạo ảnh chụp nhanh |
| DELETE | `/runtime/snapshots/:id` | công khai | Xóa ảnh chụp nhanh |
| GET | `/runtime/snapshots/:id` | Đã đăng nhập | Nhận ảnh chụp nhanh |
| POST | `/runtime/verify` | Đã đăng nhập | Xác minh |

### Chính sách truy cập dịch vụ (đã ngừng hoạt động)

> `/services/:id/access-policies` và `/services/:id/ip-bans` đã ngừng hoạt động vào ngày 20-10-2026:
> không có cuộc gọi quản trị viên nào và không có đường dẫn kết nối nào sử dụng các chính sách này.

### Giám sát hạn ngạch giao thông

Chế độ xem giám sát chỉ đọc về hạn ngạch và mức sử dụng.

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| GET | `/traffic/quotas` | Đã đăng nhập | Danh sách hạn ngạch |
| GET | `/traffic/quotas/:id/usage` | Đã đăng nhập | Hạn mức sử dụng |
| GET | `/traffic/reset-logs` | Đã đăng nhập | Bản ghi đặt lại hạn ngạch |
| GET | `/traffic/usage-history` | Đã đăng nhập | Lịch sử sử dụng |

### Kiểm tra sức khỏe

Đầu dò sự sống.

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| GET | `/health` | công khai | Đầu dò sinh động; 200 có nghĩa là khỏe mạnh |

### Trang cổng thông tin

Trang cổng thông tin SPA.

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| GET | `/p` | công khai | Cổng thông tin HTML (được máy chủ kết xuất + thời gian chạy Vue) |

### Thử thách ACME

Gọi lại thử thách HTTP-01.

| phương pháp | Đường dẫn | Quyền | Mô tả |
|---|---|---|---|
| GET | `/.well-known/acme-challenge/:token` | công khai | Gọi lại thử thách ACME HTTP-01 (xác minh tên miền trong quá trình phát hành) |
