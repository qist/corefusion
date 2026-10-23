<!-- source: oauth-google-verification.md | locale: vi | updated: 2026-10-23 -->
# Hướng dẫn xác minh OAuth Google

> Ngôn ngữ: [简体中文](oauth-google-verification.md) · [English](oauth-google-verification.en.md) · [繁體中文](oauth-google-verification.zh-TW.md) · [Русский](oauth-google-verification.ru.md) · [日本語](oauth-google-verification.ja.md) · [한국어](oauth-google-verification.ko.md) · [Deutsch](oauth-google-verification.de.md) · [Français](oauth-google-verification.fr.md) · [Español](oauth-google-verification.es.md) · [Português](oauth-google-verification.pt.md) · [Türkçe](oauth-google-verification.tr.md) · [Bahasa Indonesia](oauth-google-verification.id.md) · [ไทย](oauth-google-verification.th.md) · Tiếng Việt · [العربية](oauth-google-verification.ar.md) · [فارسی](oauth-google-verification.fa.md)

> Phạm vi: Google đăng nhập vào cổng CoreFusion (`internal/service/oauth`, phạm vi: `openid`, `email`, `profile`).
> So sánh GitHub ở cuối (GitHub không có quy trình xác minh).

## 1. Chế độ thử nghiệm và sản xuất

Màn hình đồng ý OAuth của Google có hai trạng thái xuất bản:

| tiểu bang | Ai có thể đăng nhập | Yêu cầu xác minh |
|---|---|---|
| **Thử nghiệm** | Chỉ các tài khoản Google trong danh sách “người dùng thử nghiệm” | Không |
| **Đang sản xuất** | Bất kỳ tài khoản Google nào | **Có** (xem bên dưới) |

Khi “Cài đặt hệ thống → OAuth” được bật, trang đăng nhập cổng thông tin sẽ hiển thị nút Google cho mọi khách truy cập;
nếu ứng dụng vẫn đang trong quá trình thử nghiệm, khách truy cập không có trong danh sách người dùng thử nghiệm sẽ thấy
`Error 403: access_denied` (ứng dụng chưa được xác minh/bạn không phải là người dùng thử nghiệm).
Để cung cấp đăng nhập Google một cách công khai, **chuyển ứng dụng sang phiên bản chính thức và vượt qua xác minh**.

## 2. Yêu cầu khắt khe để xác minh

Đánh giá của Google (xác minh cơ bản cho phạm vi không bị hạn chế) yêu cầu:

1. **Trang chủ**: URL có thể truy cập công khai trên miền được ủy quyền (cùng miền hoặc miền phụ).
2. **Liên kết Chính sách quyền riêng tư**: phải **có thể truy cập công khai** (không cần đăng nhập) và giải thích dữ liệu nào bạn thu thập cũng như cách bạn sử dụng/lưu trữ/xóa dữ liệu đó. Để đăng nhập Google, ít nhất nó phải bao gồm:
 dữ liệu tài khoản Google được thu thập (email, tên, hình đại diện), mục đích (tạo/đăng nhập vào tài khoản cổng thông tin), không bán cho bên thứ ba,
 và cách người dùng có thể xóa tài khoản và dữ liệu của họ.
3. **Liên kết Điều khoản dịch vụ**: có thể truy cập công khai, mô tả các quy tắc dịch vụ.
4. **Miền được ủy quyền**: đăng ký miền của ứng dụng trong màn hình đồng ý (ví dụ: `example.com`); liên kết trang chủ/quyền riêng tư/điều khoản phải có trên đó hoặc tên miền phụ.
5. **Logo ứng dụng (tùy chọn nhưng được khuyến nghị)**: 120×120px.
6. **Giải thích phạm vi**: dự án này chỉ sử dụng `openid / email / profile` (không hạn chế, không nhạy cảm),
 thuộc phạm vi xác minh cơ bản — không có biện minh theo phạm vi và **không có đánh giá bảo mật của bên thứ ba**.
 (Các phạm vi bị hạn chế như quyền truy cập đầy đủ vào Gmail sẽ yêu cầu chúng; dự án này không sử dụng bất kỳ phạm vi nào.)

Mục xác minh: [Google Cloud Console](https://console.cloud.google.com/) →
**API & Dịch vụ → Màn hình đồng ý OAuth → Xuất bản ứng dụng → Gửi để xác minh**.
Quá trình xác minh cơ bản thường hoàn tất trong vòng vài ngày; trong thời gian chờ đợi, bạn có thể hoạt động ở chế độ thử nghiệm.

## 3. Cấu hình gì cho dự án này

### 3.1 Chuyển hướng được ủy quyền URI

Lệnh gọi lại được tạo động từ miền bắt đầu đăng nhập (xem `Authorize` trong `internal/api/handlers/oauth/handler.go`):

```
https://<portal-domain>/api/v1/client-auth/oauth/google/callback
```

- Đăng nhập bắt đầu từ miền cổng thông tin (ví dụ: `https://p.example.com`) → đăng ký `https://p.example.com/...`;
- Bắt đầu từ miền bảng điều khiển `/p` → đăng ký `https://panel.example.com/...`.
- Google cho phép **nhiều** URI chuyển hướng; đăng ký cả hai tên miền.
- ⚠ Google yêu cầu URI chuyển hướng phải là **https** (ngoại trừ `http://localhost`);
 proxy ngược phải chuyển tiếp `X-Forwarded-Host` / `X-Forwarded-Proto`, nếu không bảng sẽ tạo một
 http hoặc redirect_uri nội bộ → Google từ chối nó (`redirect_uri_mismatch`).

Sau khi cài đặt “Cài đặt hệ thống → OAuth → Miền gọi lại cổng thông tin”, lệnh gọi lại URL hiển thị trên trang sẽ được dán vào Google.

### 3.2 Liên kết Chính sách quyền riêng tư/Điều khoản trên cổng thông tin

Thẻ đăng nhập bao gồm các liên kết “Chính sách quyền riêng tư · Điều khoản dịch vụ” (hiển thị sau khi được định cấu hình) và cả hai tài liệu đều có **trang độc lập**
(hộp thoại và trang độc lập có cùng nội dung):

- Miền bảng điều khiển chính: `https://<panel-domain>/p/legal/privacy`, `/p/legal/terms`
- Miền cổng thông tin chuyên dụng (triển khai gốc): `https://<portal-domain>/legal/privacy`, `/legal/terms`
- Hỗ trợ `?lang=xx`; không có nó, `Accept-Language` của trình duyệt sẽ được sử dụng, sau đó là cổng mặc định.
- Các trang được **máy chủ hiển thị** (không có JS), điều này giúp trình thu thập thông tin xác minh của Google.

**Cấu hình (một trong hai cách; liên kết thắng)**:

1. **Chế độ nội dung (được khuyến nghị, hoạt động ngay lập tức)**: trình chỉnh sửa cổng thông tin → Cài đặt → Chung → Tài liệu tuân thủ →
 chọn “Chỉnh sửa ngôn ngữ” và dán mẫu §4 vào “Nội dung chính sách quyền riêng tư” / “Nội dung điều khoản dịch vụ”
 (**Đánh dấu, chỉnh sửa theo ngôn ngữ**) → sau khi lưu, thẻ đăng nhập hiển thị các liên kết mở **hộp thoại**;
 “Liên kết độc lập” của người chỉnh sửa có thể được nhập trực tiếp vào màn hình đồng ý Google.
2. **Chế độ liên kết ngoài**: nhập “Chính sách quyền riêng tư URL” / “Điều khoản dịch vụ URL” (trỏ đến các trang bạn lưu trữ) → mở trong tab mới.

Nội dung và liên kết được gửi qua `/api/v1/public/portal-config` (`privacy_policy_content` /
`terms_of_service_content` là **bản đồ Markdown theo ngôn ngữ** `{"zh-CN":"...","en":"..."}`;
`privacy_policy_url` / `terms_of_service_url` là các liên kết bên ngoài tùy chọn). **Họ phải có thể truy cập được mà không cần đăng nhập** ✓.

### 3.3 Công tắc phía bảng điều khiển

`System settings → OAuth`: bật Google và nhập Client ID/Client Secret (bí mật được mã hóa bằng AES-256-GCM khi lưu trữ).
Nút Google chỉ xuất hiện trên trang đăng nhập cổng thông tin khi mọi thứ đã được định cấu hình (`/api/v1/client-auth/oauth/providers` đã bật trả về).

## 4. Mẫu Chính sách quyền riêng tư / Điều khoản

> Các mẫu tuân thủ tối thiểu mà bạn có thể điều chỉnh. Trước khi sử dụng: thay thế phần giữ chỗ `【】`, thêm/xóa các mệnh đề cho doanh nghiệp của bạn,
> và yêu cầu đơn vị điều hành xem xét chúng. Những mẫu này không phải là lời khuyên pháp lý.

### 4.1 Chính sách quyền riêng tư (mẫu)

```markdown
# Privacy Policy

Last updated: [DATE]

[OPERATOR NAME] ("we") operates [PORTAL NAME] ("the Service"). We respect your privacy;
this policy explains what information we collect and how we use it.

## 1. Information we collect
- **Account information**: the email address and password you provide at registration (stored only as a one-way hash);
- **Google sign-in information**: when you choose "Sign in with Google", we receive, with your authorization,
  your Google account email, display name and avatar URL (scopes: openid, email, profile);
- **Service data**: subscription configuration, traffic usage, device IP allow-list, tickets and order records.

## 2. How we use information
Only to: create and sign in to your account, provide the proxy subscription service, bill you and provide support,
and for security controls (sign-in protection, IP allow-list). We **do not** sell your personal information.

## 3. Third-party services
- **Google**: receives account information within the authorized scope only when you actively use "Sign in with Google";
- Payment providers: receive order information only when you initiate a payment.

## 4. Storage and security
Data is stored on our servers; credentials are encrypted at rest and all traffic uses TLS.

## 5. Your rights
You can update your profile/password and end sessions in the portal at any time; to delete your account and all data,
contact [CONTACT EMAIL] and we will process the request within [N] business days.

## 6. Policy changes
Material changes will be announced in the portal notice board.
```

### 4.2 Điều khoản dịch vụ (mẫu)

```markdown
# Terms of Service

Last updated: [DATE]

## 1. Service
The Service provides [PROXY / NETWORK ACCESS] to registered users; specific plans are described on the store page.

## 2. Accounts
- You are responsible for the security of your account and password; for accounts created with Google sign-in, credentials are managed by Google;
- You may not lend or transfer your account, or use the Service for any unlawful purpose (including but not limited to:
  unauthorized access to others' systems, distributing illegal content, or circumventing mandatory legal requirements).

## 3. Payment and refunds
Plans are billed [monthly / by usage]; traffic and device limits follow the plan description;
[REFUND POLICY: e.g. full refund within 7 days if unused].

## 4. Changes and termination
We may suspend or terminate the Service due to force majeure or your breach of these terms; deleting an account deletes the related data.

## 5. Disclaimer
The Service is provided "as is"; to the extent permitted by law, we are not liable for indirect losses caused by use of the Service.

## 6. Miscellaneous
These terms are governed by the laws of [JURISDICTION]. Contact: [CONTACT EMAIL].
```

## 5. Danh sách kiểm tra trước khi ra mắt

- [ ] Màn hình đồng ý OAuth: tên ứng dụng / logo / trang chủ / chính sách quyền riêng tư / điều khoản đều có; tên miền đã đăng ký
- [ ] Các trang Chính sách quyền riêng tư và Điều khoản **có thể truy cập mà không cần đăng nhập** (trang tùy chỉnh cổng thông tin hoặc trang tĩnh độc lập)
- [ ] Cả hai liên kết đều hiển thị trên thẻ/chân trang đăng nhập vào cổng thông tin
- [ ] chuyển hướng URI khớp với tên miền thực tế và là https
- [ ] Xuất bản ứng dụng → gửi để xác minh; hạn chế người dùng thử nghiệm được dỡ bỏ sau khi được phê duyệt
- [ ] Cài đặt OAuth của bảng điều khiển: đã bật=true với ID khách hàng/Bí mật được điền (điểm cuối của nhà cung cấp trả về đã bật:true)

## 6. So sánh GitHub

Ứng dụng OAuth GitHub **không** có trạng thái xuất bản hoặc quy trình xác minh: tạo ứng dụng, đặt lệnh gọi lại URL và nó hoạt động công khai.
Lưu ý rằng Ứng dụng OAuth GitHub chỉ chấp nhận **một** lệnh gọi lại ủy quyền URL (khớp tiền tố); cho nhiều tên miền,
sử dụng phương pháp tiếp cận đa URI từ Google hoặc tạo một ứng dụng riêng cho mỗi miền cổng thông tin.
