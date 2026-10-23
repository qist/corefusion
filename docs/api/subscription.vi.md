<!-- source: subscription.md | locale: vi | updated: 2026-10-23 -->
# Đăng ký API (công khai)

> Ngôn ngữ: [简体中文](subscription.md) · [English](subscription.en.md) · [繁體中文](subscription.zh-TW.md) · [Русский](subscription.ru.md) · [日本語](subscription.ja.md) · [한국어](subscription.ko.md) · [Deutsch](subscription.de.md) · [Français](subscription.fr.md) · [Español](subscription.es.md) · [Português](subscription.pt.md) · [Türkçe](subscription.tr.md) · [Bahasa Indonesia](subscription.id.md) · [ไทย](subscription.th.md) · Tiếng Việt · [العربية](subscription.ar.md) · [فارسی](subscription.fa.md)

Tìm nạp danh sách nút đăng ký của người dùng. **Đây là điểm cuối được người dùng cuối sử dụng nhiều nhất**; không yêu cầu tiêu đề - quyền truy cập được cấp bằng mã thông báo trong URL.

## 1. Điểm cuối

| Triển khai | URL |
|---|---|
| Bảng điều khiển chính | `GET https://<panel-domain>/sub/<token>` |
| Cổng thuê bao chuyên dụng | `GET https://<sub-domain>:<port>/<token>` (không có tiền tố `/sub`) |

`<token>` là mã thông báo truy cập duy nhất của một gói đăng ký, do phía quản trị viên tạo ra (hiển thị trong phần “Đăng ký của tôi” trong cổng thông tin).

## 2. Tham số truy vấn

| tham số | Mô tả |
|---|---|
| `format` | Định dạng đầu ra; ghi đè mặc định đăng ký. Xem bên dưới |
| `config` | Tên mẫu cấu hình (được xác định trong “Định dạng đăng ký” trong giao diện người dùng quản trị viên). Phải sử dụng cùng định dạng với `format` của yêu cầu này, nếu không nó sẽ bị bỏ qua |
| `platform` | Khớp mẫu cấu hình theo tên nền tảng (được sử dụng khi bỏ qua `config`); định dạng cũng phải phù hợp |

### Các giá trị `format` được hỗ trợ

| Giá trị | đầu ra | Khách hàng |
|---|---|---|
| `base64_uri` (mặc định) | Liên kết chia sẻ được nối dòng mới, được mã hóa base64 nói chung | V2RayN, Shadowrocket, Surge và các ứng dụng khách chung khác |
| `raw_uri` | Liên kết chia sẻ được nối dòng mới, văn bản thuần túy | Khách hàng cần liên kết/gỡ lỗi đơn giản |
| `mihomo_yaml` | Cấu hình Clash/Mihomo YAML đầy đủ (với các nhóm proxy, quy tắc) | Clash.Meta / mihomo / Clash Verge Rev |
| `mihomo_proxies_yaml` | YAML chỉ có danh sách nút `proxies:` | nguồn cung cấp proxy mihomo (có làm mới theo lịch trình) |
| `singbox_json` | Cấu hình sing-box JSON đầy đủ | sing-box / SFI / FLClash (lõi sing-box) |
| `xray_json` | Cấu hình Xray JSON đầy đủ | Lõi Xray |

> Áp dụng khả năng tương thích giao thức/định dạng: ví dụ: `anytls` chỉ xuất hiện ở định dạng sing-box/Mihomo; Các nút REALITY trên máy chủ sing-box không xuất hiện trong đăng ký Xray/sing-box JSON. Các nút mà khách hàng không thể nhận ra sẽ tự động bị loại bỏ thay vì toàn bộ đăng ký bị lỗi.

## 3. Tiêu đề phản hồi

| tiêu đề | Ví dụ | Mô tả |
|---|---|---|
| `Subscription-Userinfo` | `upload=1024; download=2048; total=1073741824; expire=1760000000` | Cách sử dụng và hết hạn (byte/giây Unix). **Cũng được gửi vào 403**, để khách hàng có thể hiển thị khoản trợ cấp còn lại |
| `Profile-Update-Interval` | `6` | Khoảng thời gian làm mới được đề xuất (giờ) |
| `Profile-Title` | `CoreFusion` | Tên hồ sơ (được hiển thị bởi Clash Verge Rev / V2RayN, v.v.) |
| `Profile-Web-Page-Url` | `https://…/portal` | Liên kết trang cổng thông tin (“trang web mở” trong ứng dụng khách) |
| `Content-Type` | Mỗi định dạng: `text/plain` / `text/yaml` / `application/json` |  |
| `Content-Disposition` | `attachment; filename="subscription.txt"` |  |

## 4. Mã trạng thái

| HTTP | mã | Ý nghĩa |
|---|---|---|
| 200 | — | Bình thường (nội dung là đăng ký được hiển thị trong `format`) |
| 400 | `MISSING_TOKEN` | Không có mã thông báo trong URL |
| 403 | (văn bản thuần túy) | Đăng ký bị đình chỉ/hết hạn/không hoạt động. Thông báo được bản địa hóa (zh/en/ja/ko) qua `Accept-Language` và là lý do một dòng |
| 404 | `SUBSCRIPTION_NOT_FOUND` | Mã thông báo không tồn tại |

> Đăng ký bị thu hồi được coi là không tồn tại (404).

## 5. Ví dụ

```bash
# 1) Fetch with the default format
curl -L "https://panel.example.com/sub/abcd1234efgh5678"

# 2) Request Mihomo YAML with the "clash-global" config template
curl -L "https://panel.example.com/sub/abcd1234efgh5678?format=mihomo_yaml&config=clash-global"

# 3) Node list only (for a mihomo proxy-provider)
curl -L "https://panel.example.com/sub/abcd1234efgh5678?format=mihomo_proxies_yaml"

# 4) Check usage (headers only)
curl -sI "https://panel.example.com/sub/abcd1234efgh5678" | grep -i subscription-userinfo
```

Đăng ký động Clash/Mihomo (làm mới định kỳ):

```yaml
proxy-providers:
  subscription-nodes:
    type: http
    url: "https://panel.example.com/sub/abcd1234efgh5678?format=mihomo_proxies_yaml"
    interval: 3600
    health-check:
      enable: true
      url: https://www.gstatic.com/generate_204
      interval: 300

proxy-groups:
  - name: PROXY
    type: select
    use:
      - subscription-nodes
```

## 6. Ghi chú

- **Định dạng và cấu hình được kết hợp chặt chẽ**: mẫu được `config`/`platform` chọn phải khớp với `format` của yêu cầu này; các mẫu không khớp sẽ bị bỏ qua - điều này ngăn việc đưa các quy tắc Mihomo vào sing-box/Xray JSON.
- **Giới hạn tỷ lệ**: điểm cuối đăng ký có giới hạn riêng (`SubscribeRateLimit`); khách hàng lạm dụng bị từ chối.
- **Nhật ký truy cập**: mọi lần tìm nạp đều ghi lại thông tin IP / UA / CDN, hiển thị trong phần “Nhật ký truy cập đăng ký” trong giao diện người dùng quản trị viên.
- Nếu không có miền được định cấu hình trên cổng đăng ký chuyên dụng, `X-Forwarded-Proto` / `X-Forwarded-Host` của yêu cầu sẽ được sử dụng; đảm bảo proxy ngược của bạn chuyển tiếp cả hai tiêu đề.
