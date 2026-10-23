<!-- source: portal-api.md | locale: vi | updated: 2026-10-23 -->
# Tham khảo Cổng thông tin API (Khách hàng / Cổng thông tin API)

> Ngôn ngữ: [简体中文](portal-api.md) · [English](portal-api.en.md) · [繁體中文](portal-api.zh-TW.md) · [Русский](portal-api.ru.md) · [日本語](portal-api.ja.md) · [한국어](portal-api.ko.md) · [Deutsch](portal-api.de.md) · [Français](portal-api.fr.md) · [Español](portal-api.es.md) · [Português](portal-api.pt.md) · [Türkçe](portal-api.tr.md) · [Bahasa Indonesia](portal-api.id.md) · [ไทย](portal-api.th.md) · Tiếng Việt · [العربية](portal-api.ar.md) · [فارسی](portal-api.fa.md)

> Tham chiếu giao diện **hoàn chỉnh** cho giao diện người dùng cổng thông tin (`/p` hoặc cổng cổng thông tin chuyên dụng) và ứng dụng khách bên thứ ba.
> Định nghĩa tuyến đường: `internal/api/router/router.go` (`clientAuthGroup` / `clientAuthProtected` / `/api/v1/public/*`).

## 1. Công ước chung

### 1.1 Đế URL

| Triển khai | Đế URL |
|---|---|
| Động cơ bảng điều khiển chính | `https://<panel-domain>/api/v1` (trang cổng thông tin tại `/p`) |
| Cổng cổng chuyên dụng | `https://<portal-domain>[:port]/api/v1` (trang cổng thông tin tại `/`) |
| Miền cổng thông tin chuyên dụng + proxy ngược | Tương tự như trên; proxy phải chuyển tiếp cả đường dẫn trang và `/api/v1` |

### 1.2 Xác thực

- Sau khi đăng nhập, hãy gửi `Authorization: Bearer <access_token>`;
- Mã thông báo truy cập kéo dài 24 giờ, mã thông báo làm mới 7 ngày;
- JWT chứa `tv` (token_version): đăng xuất/đăng xuất bắt buộc sẽ tăng nó và vô hiệu hóa tất cả các mã thông báo cũ
 (`clients.token_version`, xem `internal/service/clientauth/service.go`);
- Mọi điểm cuối được bảo vệ đều đi qua `ClientAuthMiddleware` (kiểm tra phiên bản + bảo vệ chế độ bảo trì).

### 1.3 Phong bì phản hồi

Hầu hết các điểm cuối đều trả về `{ "code": 0, "data": ..., "message": "..." }`;
một số điểm cuối danh sách trả về `{ "items": [...], "total": N }` hoặc một mảng trống (được ghi chú cho mỗi điểm cuối bên dưới).
Lỗi trả về `{ "code": "ERROR_CODE", "message": "...", "request_id": "...", "can_retry": ... }`
(HTTP 4xx/5xx).

### 1.4 Đơn vị số lượng

Các trường đơn hàng/số dư là **xu nguyên**: `990` = ¥9,90 (`currency: "CNY"`).

### 1.5 Ví dụ thực tế (đăng nhập → tìm nạp hồ sơ)

```bash
# Register (when registration_enabled=true)
curl -X POST https://<portal-domain>/api/v1/client-auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","password":"plaintext-password","referral_code":""}'
# → 201 {"data":{"access_token":"<JWT>","refresh_token":"<JWT>","client":{...}}}

# Sign in
curl -X POST https://<portal-domain>/api/v1/client-auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","password":"plaintext-password"}'
# → 200 same as above

# Fetch the current client profile (with token)
curl https://<portal-domain>/api/v1/client-auth/me \
  -H 'Authorization: Bearer <access_token>'
# → 200 {"data":{"id":27,"email":"user@example.com","nickname":"","status":"active","quota_bytes":10737418240,"used_bytes":0,...}}

# Fetch a subscription (no header, token in the path)
curl 'https://<portal-domain>/sub/<subscription_token>?format=mihomo_yaml'
# → 200 Mihomo YAML (or base64_uri / singbox_json / xray_json)

# Error example: expired token
# → 401 {"code":"errClientTokenRevoked","message":"...","request_id":"...","can_retry":false}
```

Trên 401, hãy gọi `POST /api/v1/client-auth/refresh` một lần và thử lại; nếu vẫn không thành công, hãy xóa phiên cục bộ (mẫu cổng thông tin đã thực hiện việc này).

---

## 2. Điểm cuối công cộng (không cần đăng nhập)

| phương pháp | Đường dẫn | Mô tả |
|---|---|---|
| GET | `/p` (`/` trên công cụ cổng thông tin chuyên dụng) | Cổng thông tin HTML (được máy chủ kết xuất + thời gian chạy Vue) |
| GET | `/api/v1/public/portal-config` | Cấu hình thời gian chạy cổng thông tin: tên trang web, màu chủ đề, điều hướng, mô-đun, gói ngôn ngữ, `registration_enabled`, `regions` (nguồn vùng đăng nhập), `privacy_policy_url` / `terms_of_service_url` / `privacy_policy_content` / `terms_of_service_content` (tài liệu tuân thủ; nội dung được ánh xạ theo ngôn ngữ; các trang độc lập `/p/legal/*` ✓) |
| GET | `/api/v1/public/settings` | Cài đặt công khai (hiện tại là `registration_enabled`) |
| GET | `/api/v1/public/subscription-endpoints` | Danh sách điểm cuối đăng ký (mảng trần) |
| GET | `/api/v1/public/articles` | Bài viết công khai (chỉ được xuất bản) |
| GET | `/api/v1/public/articles/:slug` | Chi tiết bài viết |
| GET | `/api/v1/public/client-configs` | Cấu hình tải xuống của khách hàng |
| GET | `/sub/:token` | Phân phối đăng ký (`format` trả về base64_uri / raw_uri / mihomo_yaml / singbox_json / xray_json, v.v.; bị thu hồi/thiếu → 404, bị treo/hết hạn/không hoạt động → 403. Xem [subscription.md](portal-api.vi.md)) |
| GET | `/api/v1/client-auth/oauth/providers` | Khả năng hiển thị của nhà cung cấp OAuth: `{"data":{"providers":[{"provider":"github","enabled":true},...]}}` |
| GET | `/api/v1/system/maintenance` | Trạng thái bảo trì (công khai). Cổng thông tin hiển thị trang bảo trì khi được bật |

> Tất cả những điều này bao gồm bộ bảo vệ bảo trì và giới hạn tốc độ API công khai.

## 3. Xác thực/OAuth

| phương pháp | Đường dẫn | Xác thực | Mô tả |
|---|---|---|---|
| POST | `/api/v1/client-auth/register` | — | Đăng ký. Thân máy: `{email, password, referral_code?}`. Trả về `{access_token, refresh_token, client}`; tỷ lệ đăng ký giới hạn |
| POST | `/api/v1/client-auth/login` | — | Đăng nhập. Nội dung: `{email, password}`. Phản ứng tương tự; phát hành JWT với `token_version` |
| POST | `/api/v1/client-auth/refresh` | — | Làm mới. Thân máy: `{refresh_token}`. Trả về mã thông báo truy cập/làm mới mới (phiên bản đã được kiểm tra; mã thông báo làm mới cũ sẽ chết ngay sau khi đăng xuất) |
| POST | `/api/v1/client-auth/logout` | ✓ | Đăng xuất: thu hồi phía máy chủ (`token_version +1`) → tất cả các mã thông báo cũ cho khách hàng đều không hợp lệ |
| POST | `/api/v1/client-auth/rotate-token` | ✓ | Xoay mã thông báo đăng ký |
| POST | `/api/v1/client-auth/ticket` | ✓ | Phát hành **vé ngắn hạn một lần** (đối với các luồng `window.open`, chẳng hạn như “gói nhật ký tải xuống” nơi không thể đặt tiêu đề) |
| GET | `/api/v1/client-auth/oauth/:provider` | — | Bắt đầu OAuth: 307 → GitHub/Google đồng ý (`state` anti-CSRF, 10 phút); nhà cung cấp khuyết tật trả lại 400 |
| GET | `/api/v1/client-auth/oauth/:provider/callback` | — | Gọi lại OAuth: trao đổi mã → tạo/liên kết ứng dụng khách → phát hành phiên bản JWT → 302 quay lại cổng thông tin (`/<portalPath>?access_token=...&refresh_token=...`) |

> Lệnh gọi lại OAuth là **chuyển hướng trình duyệt** (không phải lệnh gọi API): sau khi đăng nhập thành công, cổng sẽ đọc mã thông báo
> từ URL trong `onMounted` (hoặc thông qua `postMessage` có cùng nguồn gốc, xem `handleOAuthCallback` trong mẫu cổng thông tin `web`).

## 4. Tài khoản

| phương pháp | Đường dẫn | Mô tả |
|---|---|---|
| GET | `/api/v1/client-auth/me` | Khách hàng hiện tại: `{id, email, plan_id, plan_name, status, quota_bytes, used_bytes, upload_bytes, download_bytes, expires_at, ip_whitelist, device_limit, sub_token, sub_status, created_at, ...}` (phong bì `data`) |
| POST | `/api/v1/client-auth/change-password` | Thân máy: `{old_password, new_password}` |
| GET | `/api/v1/client-auth/checktime` | Thời gian máy chủ (đồng bộ hóa đồng hồ máy khách) |
| GET | `/api/v1/client-auth/connection-info` | Thông số kết nối gói hiện tại: `{concurrent, device_limit, protocol, speed_limit, tls, transport}` |

## 5. Đăng ký

| phương pháp | Đường dẫn | Mô tả |
|---|---|---|
| POST | `/api/v1/client-auth/reset-subscription` | Đặt lại đăng ký; cái này quay `sub_token` |

> Liên kết đăng ký được xây dựng trên giao diện người dùng từ `sub_token` được trả về bởi `/me` (`/sub/:token`); không có
> tách biệt các điểm cuối `subscription-link`/`subscription-formats` nữa.

## 6. Nút

| phương pháp | Đường dẫn | Mô tả |
|---|---|---|
| GET | `/api/v1/client-auth/nodes` | Các nút hiển thị với gói hiện tại: `{items:[...]}` (hiển thị các trường như tên/giao thức/vận chuyển/vùng) |
| GET | `/api/v1/client-auth/nodes/:id/config` | Cấu hình nút đầy đủ |
| GET | `/api/v1/client-auth/nodes/:id/link` | Liên kết chia sẻ (vless://, v.v.) |
| GET | `/api/v1/client-auth/nodes/:id/qrcode` | Hình ảnh mã QR |

## 7. Cửa hàng/kế hoạch/đơn hàng

| phương pháp | Đường dẫn | Mô tả |
|---|---|---|
| GET | `/api/v1/client-auth/plans` | Danh sách kế hoạch (mảng trần): `{id, name, description, category, sort_order, variants:[{id, name, billing_cycle, duration_days, price(cents), original_price, currency, quota_bytes, ...}]}` |
| GET | `/api/v1/client-auth/plans/:id` | Kế hoạch chi tiết |
| GET | `/api/v1/client-auth/traffic-packs` | Danh sách gói lưu lượng truy cập |
| POST | `/api/v1/client-auth/traffic-packs/buy` | Mua gói giao thông |
| GET | `/api/v1/client-auth/orders` | Đơn đặt hàng kế hoạch của tôi (phân trang) |
| POST | `/api/v1/client-auth/orders` | Tạo đơn hàng (mua/gia hạn một biến thể gói) |
| GET | `/api/v1/client-auth/orders/:id` | Chi tiết đơn hàng |
| GET | `/api/v1/client-auth/invoices` | Hóa đơn |
| GET | `/api/v1/client-auth/invoices/:id` | Chi tiết hóa đơn |

## 8. Cân bằng/nạp tiền

| phương pháp | Đường dẫn | Mô tả |
|---|---|---|
| GET | `/api/v1/client-auth/balance` | `{data:{client_id, balance(cents), frozen}}` |
| GET | `/api/v1/client-auth/balance/transactions` | Giao dịch số dư (`type`: tính phí/tiêu dùng/hoàn tiền; phân trang) |
| GET | `/api/v1/client-auth/charge/sites` | Các trang web nạp tiền có sẵn (`ListChargeSites`: chỉ được bật) |
| POST | `/api/v1/client-auth/charge/create` | Tạo lệnh nạp tiền (chuyển hướng đến thanh toán của bên thứ ba) |
| GET | `/api/v1/client-auth/charge/orders` | Danh sách đơn hàng nạp tiền |
| GET | `/api/v1/client-auth/charge/orders/:id` | Chi tiết đơn hàng nạp tiền |
| GET | `/api/v1/client-auth/charge/status/:trade_no` | Thăm dò trạng thái thanh toán theo Trade_no |
| POST | `/api/v1/client-auth/charge/redeem-card` | Đổi thẻ nạp tiền |

> Xóa khách hàng **xếp tầng** đối với số dư và giao dịch của khách hàng đó (xem `DeleteClient` trong `internal/service/client/service.go`).

## 9. Quà tặng/lịch sử giao thông

| phương pháp | Đường dẫn | Mô tả |
|---|---|---|
| GET | `/api/v1/client-auth/gifts` | Quà tặng có thể yêu cầu (mảng trần; `type`: ngày/byte) |
| POST | `/api/v1/client-auth/gifts/:id/claim` | Yêu cầu |
| GET | `/api/v1/client-auth/traffic-history` | Lịch sử lưu lượng truy cập hàng tháng: `{data:{history:[{period_start,total_bytes,...}], months}}` |

## 10. Vé

| phương pháp | Đường dẫn | Mô tả |
|---|---|---|
| GET | `/api/v1/client-auth/tickets` | Danh sách vé: `{items,total}` |
| POST | `/api/v1/client-auth/tickets` | Tạo một vé |
| GET | `/api/v1/client-auth/tickets/:id` | Chi tiết vé (có câu trả lời) |
| POST | `/api/v1/client-auth/tickets/:id/reply` | trả lời |
| POST | `/api/v1/client-auth/tickets/:id/close` | Đóng |

## 11. Bài viết/hướng dẫn

| phương pháp | Đường dẫn | Mô tả |
|---|---|---|
| GET | `/api/v1/client-auth/articles` | Danh sách bài viết (có bộ lọc danh mục) |
| GET | `/api/v1/client-auth/articles/:slug` | Chi tiết bài viết |

## 12. Giới thiệu (bị tắt theo mặc định; có thể được bật trong trình chỉnh sửa cổng thông tin)

| phương pháp | Đường dẫn | Mô tả |
|---|---|---|
| GET | `/api/v1/client-auth/referral/code` | Mã giới thiệu và liên kết của tôi |
| GET | `/api/v1/client-auth/referral/stats` | Số liệu thống kê hoa hồng (`commission_rate`, `available_commission`, `min_withdraw_amount`) |
| GET | `/api/v1/client-auth/referral/records` | Hồ sơ giới thiệu |
| GET | `/api/v1/client-auth/referral/withdrawals` | Hồ sơ rút tiền |
| POST | `/api/v1/client-auth/referral/withdraw` | Yêu cầu rút tiền |

## 13. Tài khoản được chia sẻ

| phương pháp | Đường dẫn | Mô tả |
|---|---|---|
| GET | `/api/v1/client-auth/shared-accounts` | Các tài khoản chia sẻ có sẵn (phát trực tuyến, v.v.) |
| GET | `/api/v1/client-auth/shared-accounts/:id` | Chi tiết |
| POST | `/api/v1/client-auth/shared-accounts/:id/apply` | Yêu cầu quyền truy cập |
| POST | `/api/v1/client-auth/shared-accounts/:id/repair` | Sửa chữa bằng một cú nhấp chuột |

## 14. Gói nhật ký (chẩn đoán người đăng ký)

| phương pháp | Đường dẫn | Mô tả |
|---|---|---|
| GET | `/api/v1/client-auth/logs/types` | Các loại nhật ký có thể được nhóm |
| GET | `/api/v1/client-auth/logs/packages` | Gói nhật ký của tôi |
| POST | `/api/v1/client-auth/logs/package` | Tạo một công việc đóng gói nhật ký (chỉ các nút mục nhập) |
| GET | `/api/v1/client-auth/logs/packages/:id` | Trạng thái công việc (sẵn sàng/đang xử lý/không thành công) |
| GET | `/api/v1/client-auth/logs/packages/:id/download` | Tải xuống (thích `POST /ticket` và nối thêm `?ticket=`) |
| DELETE | `/api/v1/client-auth/logs/packages/:id` | Xóa gói nhật ký |

## 15. Cài đặt bảo mật/danh sách IP cho phép/tự bỏ cấm

| phương pháp | Đường dẫn | Mô tả |
|---|---|---|
| GET/PUT | `/api/v1/client-auth/settings` | Cài đặt máy khách: `{ip_whitelist, device_limit}` (PUT) |
| GET | `/api/v1/client-auth/settings/whitelist` | Danh sách cho phép (mảng trần) |
| DELETE | `/api/v1/client-auth/settings/whitelist/:id` | Xóa mục nhập danh sách cho phép |
| POST | `/api/v1/client-auth/settings/auto-add-ip` | Thêm IP hiện tại vào danh sách cho phép |
| POST | `/api/v1/client-auth/self-unban` | Bỏ cấm tự phục vụ (các nỗ lực bị giới hạn bởi chính sách) |

> Danh sách cho phép IP **chỉ áp dụng cho các URL đăng ký** (`/sub/:token`); nó không chặn đăng nhập vào cổng thông tin/OAuth hoặc
> các điểm cuối xác thực khách hàng khác. Nếu người dùng không có danh sách cho phép hoặc không đăng ký URL, quyền truy cập luôn được phép. các
> danh sách cho phép là cài đặt cho mỗi người dùng; không có chuyển đổi toàn cầu.

**Luồng tự bỏ cấm**: chỉ khả dụng cho `auto_quota` (tự động cấm khi quá tải) khi phía quản trị viên
`self_unban.enabled` đang bật. Đăng nhập (mật khẩu hoặc gọi lại OAuth) trả về `require_self_unban=true` + `self_unban_token`
(typ=self_unban, hợp lệ trong 15 phút, **chỉ** sử dụng được trên `POST /self-unban`; các điểm cuối khác trả về 403/`SELF_UNBAN_TOKEN_SCOPE`).
Sau đó, cổng sẽ hiển thị thẻ bỏ cấm; sau khi bỏ cấm thành công, người dùng đăng nhập lại. Lệnh gọi lại OAuth cung cấp điều này thông qua
đoạn URL `#require_self_unban=1&self_unban_token=..&self_unban_hint=..`.

## 16. Mã lỗi (đoạn trích)

| mã | Kịch bản |
|---|---|
| `INVALID_TOKEN` | Làm mới mã thông báo không hợp lệ/hết hạn/phiên bản không khớp |
| `INVALID_REQUEST` | Xác thực tham số không thành công |
| `NOT_FOUND` | Thiếu tài nguyên (vé/đơn hàng/mục nhập danh sách cho phép, v.v.) |
| `RELOAD_FAILED` / `SYNC_FAILED` | (Phía quản trị viên) đồng bộ hóa cấu hình proxy không thành công |
| 401 + `errClientTokenRevoked` | token_version không khớp (đăng xuất/bị loại bỏ) → khách hàng nên làm mới; nếu thất bại, hãy xóa mã thông báo cục bộ |

## 17. Tài liệu tuân thủ (chính sách bảo mật / điều khoản dịch vụ)

Liên kết “Chính sách quyền riêng tư · Điều khoản dịch vụ” ở cuối thẻ đăng nhập (giống như các trang web thông thường; nhấp vào sẽ mở ra nội dung):

- **Cấu hình**: trình chỉnh sửa cổng thông tin → Cài đặt → Chung → Tài liệu tuân thủ (**chỉnh sửa nội dung theo ngôn ngữ**);
 bạn cũng có thể `PUT /api/v1/portal/active` với các trường có cấu trúc `privacy_policy_url` / `privacy_policy_content` /
 `terms_of_service_url` / `terms_of_service_content` (nil = không thay đổi; nội dung là bản đồ `{"zh-CN":"...","en":"..."}`).
- **Các trang có thể truy cập độc lập** (được máy chủ hiển thị, dễ dàng cho trình thu thập thông tin xác minh Google; hỗ trợ `?lang=xx`):
  - Miền bảng điều khiển chính: `GET /p/legal/privacy`, `GET /p/legal/terms`
  - Miền cổng thông tin chuyên dụng (triển khai gốc): `GET /legal/privacy`, `GET /legal/terms`
- **Logic hiển thị**: URL hoặc nội dung, **URL thắng** (mở bên ngoài trong tab mới); nếu nội dung được cung cấp, hãy nhấp vào
 thẻ đăng nhập hiển thị Markdown trong hộp thoại (`renderMd`) bằng ngôn ngữ cổng thông tin hiện tại.
- Được phân phối trong thời gian chạy qua `/api/v1/public/portal-config`; thành phần đăng nhập tiêu thụ
 `ctx.privacyUrl / termsUrl / privacyContent / termsContent / openLegal / hasLegalLinks`.
- Xác minh OAuth Google yêu cầu các liên kết này phải có thể truy cập công khai → sử dụng các liên kết độc lập ở trên trong
 màn hình đồng ý; xem `docs/oauth-google-verification.md` (chứa các mẫu chính sách / điều khoản về quyền riêng tư).

## 18. Ví dụ tích hợp

```js
// Sign in
const { data } = await fetch('/api/v1/client-auth/login', {
  method: 'POST', headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ email, password })
}).then(r => r.json());
localStorage.setItem('client_token', data.access_token);
localStorage.setItem('client_refresh_token', data.refresh_token);

// Authenticated request
const me = await fetch('/api/v1/client-auth/me', {
  headers: { Authorization: 'Bearer ' + data.access_token }
}).then(r => r.json());
```

Trên 401, hãy gọi `POST /refresh` một lần và thử lại; nếu vẫn không thành công, hãy xóa phiên cục bộ
(`api()` của mẫu cổng thông tin đã triển khai tính năng này).
