<!-- source: notification-templates.md | locale: vi | updated: 2026-10-23 -->
# Biến mẫu thông báo

> Ngôn ngữ: [简体中文](notification-templates.md) · [English](notification-templates.en.md) · [繁體中文](notification-templates.zh-TW.md) · [Русский](notification-templates.ru.md) · [日本語](notification-templates.ja.md) · [한국어](notification-templates.ko.md) · [Deutsch](notification-templates.de.md) · [Français](notification-templates.fr.md) · [Español](notification-templates.es.md) · [Português](notification-templates.pt.md) · [Türkçe](notification-templates.tr.md) · [Bahasa Indonesia](notification-templates.id.md) · [ไทย](notification-templates.th.md) · Tiếng Việt · [العربية](notification-templates.ar.md) · [فارسی](notification-templates.fa.md)

Mẫu thông báo sử dụng phần giữ chỗ `{variable}` được thay thế bằng giá trị thực tế khi gửi. Tài liệu này liệt kê tất cả các biến được hỗ trợ, các biến có sẵn cho mỗi sự kiện và quy tắc kích hoạt tự động.

## 1. Các loại mẫu và đối tượng

| Khán giả | Kênh | Người nhận | sử dụng điển hình |
|---|---|---|---|
| **Người dùng bảng** (quản trị viên) | Telegram / Email | Các kênh và người nhận được quản trị viên định cấu hình | Cảnh báo nút, báo cáo dự phòng, thông báo hệ thống |
| **Người đăng ký** | Email | Email riêng của khách hàng (điền tự động) | Số dư thấp, nhắc nhở hết hạn đăng ký |

## 2. Biến chung

Có sẵn trong tất cả các mẫu:

| Biến | Mô tả | Ví dụ |
|---|---|---|
| `{time}` | Gửi thời gian | `2026-09-28 14:30:00` |
| `{user}` | Email khách hàng (mẫu người đăng ký) / mã định danh người dùng | `user@example.com` |
| `{message}` | Nội dung thông báo | Văn bản thông báo bảo trì |

## 3. Biến sự kiện người đăng ký

### Lời nhắc hết hạn đăng ký (event_key = `subscription_expire`)

Được quét hàng ngày và gửi đến những người đăng ký đang hoạt động, hết hạn trong vòng 7 ngày:

| Biến | Mô tả | Ví dụ |
|---|---|---|
| `{user}` | Email khách hàng | `user@example.com` |
| `{expire_date}` | Hết hạn đăng ký | `2026-10-05 09:00` |
| `{days}` | Số ngày còn lại | `7` |
| `{time}` | Gửi thời gian | `2026-09-28 09:05:00` |

### Nhắc nhở số dư thấp (event_key = `balance_low`)

Quét hàng ngày và gửi cho khách hàng có số dư dưới ngưỡng:

| Biến | Mô tả | Ví dụ |
|---|---|---|
| `{user}` | Email khách hàng | `user@example.com` |
| `{balance}` | Số dư hiện tại (đơn vị tiền tệ, hai số thập phân) | `0.85` |
| `{time}` | Gửi thời gian | `2026-09-28 09:05:00` |

## 4. Các biến ví dụ về mẫu hoạt động

Khi bạn nhấp vào “Gửi” để kiểm tra khả năng kết nối, các mẫu của người dùng bảng điều khiển sẽ điền vào các phần giữ chỗ bằng các ví dụ sau:

| Biến | Mô tả | Ví dụ |
|---|---|---|
| `{node}` | Tên nút | `node-01` |
| `{backup_file}` | Tên tập tin sao lưu | `corefusion-20260101-0900.tar.gz` |
| `{size}` | Kích thước tệp | `128MB` |
| `{domain}` | Tên miền | `example.com` |
| `{expire_date}` | Ngày hết hạn | `2026-10-12` |
| `{days}` | Số ngày còn lại | `14` |
| `{count}` | Đếm | `3` |

## 5. Quy tắc kích hoạt tự động

- **Lịch trình**: lần quét đầu tiên diễn ra 30 giây sau khi bảng điều khiển bắt đầu, sau đó hàng ngày lúc **09:05**
- **Hết hạn đăng ký**: quét các đăng ký đang hoạt động (`status=active`) hết hạn trong vòng 7 ngày; khách hàng cũng phải chủ động
- **Số dư thấp**: quét các khách hàng đang hoạt động có số dư ≤ ngưỡng; ngưỡng là `notify.balance_threshold`, **tính bằng xu** (`100` mặc định = 1,00)
- **Loại bỏ trùng lặp**: tối đa một tin nhắn cho mỗi khách hàng cho mỗi sự kiện **mỗi ngày** (`notification_send_logs` loại bỏ trùng lặp theo ngày)

## 6. Theo dõi sức khỏe nút

Nhịp tim của nút được kiểm tra liên tục; **thông báo chỉ được gửi khi trạng thái thay đổi - các vòng không có thay đổi sẽ im lặng**:

| Phím cài đặt | Mô tả | Mặc định |
|---|---|---|
| `monitor.node_interval_seconds` | Khoảng thời gian kiểm tra (giây) | `60` (tối thiểu 30) |
| `monitor.node_offline_timeout_seconds` | Thời gian chờ nhịp tim (giây) | `120` |

- **Cảnh báo ngoại tuyến**: khi một nút chuyển từ trực tuyến sang ngoại tuyến, hãy gửi mẫu quản trị viên “**Cảnh báo ngoại tuyến của nút**” với `{node}` (tên) và `{time}`
- **Thông báo khôi phục**: khi một nút khôi phục, hãy gửi “**Thông báo khôi phục nút**” với cùng các biến
- **Vòng im lặng**: không có thông báo khi trạng thái không thay đổi; sau khi bảng khởi động lại, vòng đầu tiên chỉ thiết lập đường cơ sở
- **Giám sát không gửi**: tắt mẫu tương ứng (Đã bật = TẮT)

## 7. Nút cảnh báo hạn ngạch lưu lượng truy cập hàng tháng (tự động)

- **Kích hoạt**: khi lưu lượng truy cập hàng tháng của nút đạt đến ngưỡng hạn ngạch, hãy gửi “**Cảnh báo hạn ngạch lưu lượng truy cập của nút**”; khi vượt quá 100% và bị treo, hãy gửi cảnh báo vượt quá giới hạn (khóa sự kiện `traffic_quota_threshold` / `traffic_quota_exceeded`)
- **Ngưỡng**: `monitor.traffic_quota_threshold_percent` (phần trăm, `80` mặc định; đặt `0` để tắt cảnh báo ngưỡng)
- **Loại bỏ trùng lặp**: tối đa một lần cho mỗi nút trong mỗi chu kỳ thanh toán (chu kỳ tuân theo `traffic_quota_reset_day` của nút)
- Các biến có sẵn: `{node}` (tên), `{used}` (GB đã sử dụng), `{quota}` (hạn mức GB), `{percent}` (phần trăm), `{time}`

## 8. Thông báo kết quả sao lưu (tự động)

- **Kích hoạt**: được gửi ngay sau khi chạy bản sao lưu theo lịch trình (`BackupScheduler`) — thành công gửi “**Báo cáo sao lưu hàng ngày**”, lỗi (tạo bản sao lưu / bộ lưu trữ đọc / tải lên điều khiển từ xa) gửi “**Cảnh báo lỗi sao lưu**”
- Các biến có sẵn: `{backup_file}` (tên), `{size}` (kích thước, chỉ báo cáo thành công), `{message}` (lý do lỗi, chỉ cảnh báo lỗi), `{time}`
- **Giám sát mà không gửi**: bỏ qua âm thầm khi mẫu bị tắt (Đã bật = TẮT)

## 9. Lời nhắc hết hạn chứng chỉ (tự động)

- **Kích hoạt**: hàng ngày lúc 09:05, quét các chứng chỉ `active` sắp hết hạn (**Chứng chỉ IP còn 3 ngày**, **chứng chỉ miền còn 7 ngày**) và gửi mẫu quản trị viên “**Lời nhắc hết hạn chứng chỉ**” cho mỗi chứng chỉ
- **Loại bỏ trùng lặp**: tối đa một lời nhắc cho mỗi chứng chỉ mỗi ngày; sau khi gia hạn, thời hạn sử dụng sẽ di chuyển ra ngoài cửa sổ (IP 3 ngày / tên miền 7 ngày) và dừng nhắc nhở
- Các biến có sẵn: `{domain}` (miền chứng chỉ), `{expire_date}` (ngày hết hạn), `{days}` (số ngày còn lại), `{time}`

## 10. Cảnh báo sự cố quy trình cốt lõi (tự động)

- **Kích hoạt**: khi một quy trình cốt lõi (xray/singbox) thoát ra bất thường hoặc quy trình proxy (caddy/nginx) gặp sự cố, hãy gửi mẫu quản trị viên “**Cảnh báo sự cố quy trình cốt lõi**” ngay lập tức; người giám sát sau đó khởi động lại nó với thời gian chờ theo cấp số nhân
- Các biến có sẵn: `{core}` (loại quy trình), `{restarts}` / `{max}` (số lần khởi động lại / giới hạn), `{message}` (thông báo thoát, chỉ các quy trình cốt lõi), `{time}`
- **Giám sát mà không gửi**: bỏ qua âm thầm khi mẫu bị tắt (Đã bật = TẮT)

## 11. Thông báo đăng ký người dùng mới (tự động)

- **Kích hoạt**: gửi ngay sau khi khách hàng đăng ký thành công, sử dụng mẫu quản trị viên “**Thông báo đăng ký người dùng mới**”
- Các biến có sẵn: `{user}` (email khách hàng), `{time}`

## 12. Cách sử dụng

1. **Thêm mẫu**: Cài đặt thông báo → Mẫu thông báo → Thêm mẫu → “Điền từ cài sẵn” và điều chỉnh nếu cần
2. **Bật**: đánh dấu vào Đã bật và lưu; trình kích hoạt tự động chỉ xử lý các mẫu đã bật
3. **Kiểm tra**: nhấp vào “Gửi” trong danh sách để gửi tin nhắn thực với các biến mẫu (email kiểm tra mẫu người đăng ký sẽ đến địa chỉ người gửi của quản trị viên)
4. **Thông báo ngay lập tức** (mã doanh nghiệp): gọi `NotifyClientEvent(ctx, db, settingSvc, eventKey, clientEmail, vars)` tại địa điểm sự kiện (nạp tiền, khấu trừ, gia hạn) để thông báo ngay cho một khách hàng, ngoài giới hạn quét hàng ngày
