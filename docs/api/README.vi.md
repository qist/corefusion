<!-- source: README.md | locale: vi | updated: 2026-10-23 -->
# Tài liệu CoreFusion API

> Ngôn ngữ: [简体中文](README.md) · [English](README.en.md) · [繁體中文](README.zh-TW.md) · [Русский](README.ru.md) · [日本語](README.ja.md) · [한국어](README.ko.md) · [Deutsch](README.de.md) · [Français](README.fr.md) · [Español](README.es.md) · [Português](README.pt.md) · [Türkçe](README.tr.md) · [Bahasa Indonesia](README.id.md) · [ไทย](README.th.md) · Tiếng Việt · [العربية](README.ar.md) · [فارسی](README.fa.md)

Tài liệu API dành cho **các nhà tích hợp bên ngoài, nhà phát triển thứ cấp và khách hàng bên thứ ba**. Tất cả các đường dẫn đều liên quan đến miền triển khai.

| tài liệu | Khán giả | Nội dung |
|---|---|---|
| [subscription.md](./subscription.vi.md) | Người đăng ký / nhà phát triển khách hàng | Điểm cuối đăng ký (`/sub/:token`): định dạng, tiêu đề, mã trạng thái, ví dụ |
| [portal-api.md](README.vi.md) | Giao diện cổng thông tin / khách hàng của bên thứ ba | Cổng thông tin khách hàng API (`/api/v1/client-auth/*`, `/api/v1/public/*`): đăng ký, đăng nhập, gói, đơn đặt hàng, số dư, vé |
| [charge-callback.md](./charge-callback.vi.md) | Nhà cung cấp thanh toán | Ký và xác minh cuộc gọi lại thanh toán |
| [admin-api.vi.md](./admin-api.vi.md) | Phát triển/tự động hóa bảng điều khiển | Bảng lộ trình API quản trị đầy đủ (`/api/v1/*`, được nhóm theo mô-đun) |

---

## 1. Cổng và URL cơ sở

Bảng điều khiển có thể hiển thị ba công cụ cùng lúc, chia sẻ một cơ sở dữ liệu:

| Động cơ | Quyền truy cập mặc định | Mục đích |
|---|---|---|
| Bảng điều khiển chính | `https://<domain>/api/v1` | Phần phụ trợ quản trị + cổng thông tin + đăng ký |
| Công cụ đăng ký (cổng chuyên dụng) | `https://<domain>:<port>/:token` | Tìm nạp đăng ký đồng thời cao, không có tiền tố `/sub` |
| Công cụ cổng thông tin (cổng chuyên dụng) | `https://<portal-domain>[:port]/api/v1` | Cổng thông tin chỉ dành cho khách hàng, các trang tại `/` |

> Proxy ngược phải chuyển tiếp cả đường dẫn trang và `/api/v1`; `X-Forwarded-Proto` / `X-Forwarded-Host` rất vinh dự được khôi phục sơ đồ và miền thực.

## 2. Xác thực (ba loại, không thể thay thế cho nhau)

| Xác thực | Thông tin xác thực | Áp dụng cho |
|---|---|---|
| **Quyền quản trị** | `Authorization: Bearer <admin_token>` (từ `/api/v1/auth/login`) | Quản trị viên API |
| **Xác thực ứng dụng khách** | `Authorization: Bearer <client_token>` (từ `/api/v1/client-auth/login`) | Cổng thông tin API |
| **Mã thông báo đăng ký** | Mã thông báo trong URL: `/sub/<token>` | Tìm nạp đăng ký, không có tiêu đề |

- Mã thông báo truy cập của khách hàng kéo dài 24 giờ, mã thông báo làm mới 7 ngày; JWT chứa `tv` (token_version), do đó việc đăng xuất/đăng xuất bắt buộc sẽ làm mất hiệu lực tất cả các mã thông báo cũ.
- Vai trò quản trị viên: `admin` / `operator` / `super_admin`; một số thao tác ghi yêu cầu `RequireRole(RoleAdmin)` hoặc cao hơn.

## 3. Định dạng phản hồi

Thành công (hầu hết các điểm cuối):

```json
{ "code": 0, "data": { ... }, "message": "ok" }
```

Liệt kê các điểm cuối (một số) trả về `{ "items": [...], "total": N }` hoặc một mảng trống (được ghi chú cho mỗi điểm cuối bên dưới).

Lỗi (HTTP 4xx/5xx):

```json
{ "code": "ERROR_CODE", "message": "human-readable description", "request_id": "…", "can_retry": false }
```

**Số tiền là số nguyên tính bằng xu**: `990` = ¥9,90 (`currency: "CNY"`).
**Lưu lượng tính bằng byte**: cả yêu cầu và phản hồi đều sử dụng byte.

## 4. Giới hạn tỷ lệ và chế độ bảo trì

- Điểm cuối công cộng (`/api/v1/public/*`, `/sub/*`, đăng ký/đăng nhập) bị giới hạn tốc độ;
- Đăng nhập cũng sử dụng `LoginRateLimit`, tìm nạp đăng ký sử dụng `SubscribeRateLimit`;
- Trong chế độ bảo trì, các điểm cuối và đăng ký công khai sẽ trả về thông báo bảo trì; quản trị viên API không bị ảnh hưởng.

## 5. Các tuyến trang giao diện người dùng (không phải API)

Các tuyến này trả về nội dung HTML/tĩnh và nằm ngoài tài liệu API; được liệt kê để chúng có thể được loại trừ trong quá trình tích hợp:

| phương pháp | Đường dẫn | Mô tả |
|---|---|---|
| GET | `/` | Bảng quản trị SPA (sau khi đăng nhập) |
| GET | `/p` | Cổng thông tin HTML (`/` trên công cụ cổng thông tin chuyên dụng) |
| GET | `/:token` | Tìm nạp đăng ký trên một cổng chuyên dụng (không có tiền tố `/sub`, ngữ nghĩa giống như `/sub/:token`) |
| GET | `/static/:file` | Nội dung tĩnh giao diện người dùng |

## 6. Nguồn sự thật

`internal/api/router/router.go` (746 tuyến đường, 743 tài liệu). Bảng lộ trình trong [admin-api.md](README.vi.md) được tạo từ mã; nếu nó không đồng ý với mã, **mã sẽ thắng**.
