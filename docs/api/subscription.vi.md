<!-- source: subscription.md | locale: vi | updated: 2026-11-12 -->
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
| `providers` | Chỉ `mihomo_yaml`: `1` / `true` bật **Chế độ nhà cung cấp** (phát ra `proxy-providers` và tham chiếu chúng qua `use`); mặc định hoặc bất kỳ giá trị nào khác giữ cho các nút nằm trong `proxies`. Xem §6 |

### Các giá trị `format` được hỗ trợ

| Giá trị | đầu ra | Khách hàng |
|---|---|---|
| `base64_uri` (mặc định) | Liên kết chia sẻ được nối dòng mới, được mã hóa base64 nói chung | V2RayN, Shadowrocket, Surge và các ứng dụng khách chung khác |
| `raw_uri` | Liên kết chia sẻ được nối dòng mới, văn bản thuần túy | Khách hàng cần liên kết/gỡ lỗi đơn giản |
| `mihomo_yaml` | Cấu hình Clash/Mihomo YAML đầy đủ (với các nhóm proxy, quy tắc) | Clash.Meta / mihomo / Clash Verge Rev |
| `mihomo_proxies_yaml` | YAML chỉ có danh sách nút `proxies:` | nguồn cung cấp proxy mihomo (có làm mới theo lịch trình) |
| `singbox_json` | Cấu hình sing-box JSON đầy đủ | sing-box / SFI / FLClash (lõi sing-box) |
| `xray_json` | Cấu hình Xray JSON đầy đủ | Lõi Xray |

> Áp dụng khả năng tương thích giao thức/định dạng: ví dụ: `anytls` chỉ xuất hiện ở định dạng sing-box/Mihomo; **Các nút REALITY do xray phân phối được loại trừ khỏi đăng ký sing-box** (sing-box không hỗ trợ triển khai REALITY của xray, xem sing-box#4520) nhưng vẫn xuất hiện trong đăng ký Xray, trong khi các nút **REALITY được phân phối của sing-box hoạt động ở cả đăng ký Xray và sing-box**. Các nút mà khách hàng không thể nhận ra sẽ tự động bị loại bỏ thay vì toàn bộ đăng ký bị lỗi.

## 3. Tiêu đề phản hồi

| tiêu đề | Ví dụ | Mô tả |
|---|---|---|
| `Subscription-Userinfo` | `upload=1024; download=2048; total=1073741824; expire=1760000000` | Cách sử dụng và hết hạn (byte/giây Unix). **Cũng được gửi vào 403**, để khách hàng có thể hiển thị khoản trợ cấp còn lại |
| `Profile-Update-Interval` | `6` | Khoảng thời gian làm mới được đề xuất (giờ) |
| `Profile-Title` | `CoreFusion` | Tên hồ sơ (được hiển thị bởi Clash Verge Rev / V2RayN, v.v.) |
| `Profile-Web-Page-Url` | `https://portal.example.com` | Cổng thông tin URL (“trang web mở” trong máy khách; đường dẫn gốc khi cổng có cổng riêng, nếu không thì /p) |
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

# 5) Clash dynamic subscription: config template + Provider mode
curl -L "https://sub.example.com/abcd1234efgh5678?format=clash&config=out&providers=1"

# 6) sing-box / Xray with their own config templates (rules come from the template)
curl -L "https://sub.example.com/abcd1234efgh5678?format=singbox_json&config=sing-box-home"
curl -L "https://sub.example.com/abcd1234efgh5678?format=xray_json&config=xray-home"
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

## 6. Chế độ nhà cung cấp và nhiều miền đăng ký

### 6.1 `providers=1` phát ra những gì

Với `format=mihomo_yaml&providers=1` (hoặc `format=clash&providers=1`), YAML:

- không còn các nút nội tuyến ở cấp cao nhất nữa — nó phát ra `proxy-providers:`;
- nguồn nút tham chiếu nhóm proxy thông qua `use: [<provider-key>...]`;
- mọi `proxy-providers` được viết ở định dạng đăng ký dùng chung (`mihomo_configs.config_json`) đều bị **bỏ qua** (cấu hình dùng chung phục vụ một số gói; nhà cung cấp mã hóa cứng sẽ rò rỉ trên các gói). Nhóm → ánh xạ nguồn nằm trong phần “Quản lý kế hoạch → nhóm → ánh xạ nguồn”.

Các loại nhà cung cấp:

| loại | Chìa khóa | Ghi chú |
|---|---|---|
| nội bộ | Tên nhóm nút được vệ sinh | Được tạo cho mỗi nhóm nút kế hoạch; chỉ chứa các nút của nhóm đó |
| Đã đặt trước | `subscription-nodes` | Tất cả các nút của kế hoạch; nguồn URL của nó **bỏ qua** tham số `provider` |
| Bên ngoài | Tên được cấu hình trên gói | Nguồn bên ngoài (`http` YAML hoặc `file`) |

Các trường phổ biến cho mỗi nhà cung cấp:

- `url` - quay lại đăng ký này với `format=mihomo_proxies_yaml`, kế thừa `config` / `platform` từ yêu cầu chính; nhà cung cấp nội bộ bổ sung `provider=<key>`;
- `path` — **luôn bắt nguồn từ “mặc định của nhà cung cấp nội bộ → mẫu đường dẫn bộ đệm”** (`./providers/{key}.yaml` mặc định, `{key}` được thay thế). Các nhà cung cấp bên ngoài cũng không thể thiết lập con đường riêng của họ;
- `health-check` — `enable / url / interval`; nhà cung cấp nội bộ kế thừa các giá trị mặc định, nhà cung cấp bên ngoài định cấu hình nó trên gói;
- `interval` - khoảng thời gian làm mới tính bằng giây.

### 6.2 Domain nào vào nhà cung cấp URL (multi-domain)

“Trung tâm đăng ký → Điểm cuối đăng ký” có thể chứa **một số** bản ghi (miền + đường dẫn + đã bật). Nhà cung cấp `url` chọn tên miền theo thứ tự sau:

1. **Miền mà yêu cầu này thực sự sử dụng**: giá trị đầu tiên của `X-Forwarded-Host` (đã loại bỏ cổng), nếu không thì là `Host`; nó phải khớp với điểm cuối **được bật**;
2. Nếu không có gì khớp, hãy quay lại điểm cuối đã bật với **ID thấp nhất**;
3. Đường dẫn đến từ điểm cuối phù hợp (`path`, `/` mặc định), mang lại `https://<domain><path>/<token>?format=mihomo_proxies_yaml...`.

Hậu quả:

- Một khách hàng đang tìm nạp thông qua một miền mới sẽ nhận được miền đó bên trong nhà cung cấp — **không cần thay đổi bảng điều khiển**;
- Tên miền cũ vẫn được bật trong danh sách không còn được mã hóa cứng nữa;
- Nếu một khách hàng đã lưu vào bộ đệm của nhà cung cấp cũ URL, việc cập nhật đăng ký (hoặc xóa bộ nhớ đệm của nhà cung cấp/khởi động lại khách hàng) sẽ chuyển đổi đăng ký đó.

Proxy ngược phải chuyển tiếp `X-Forwarded-Proto` và `X-Forwarded-Host`; nếu không thì điểm cuối dự phòng sẽ được sử dụng. Các mẫu trang web Caddy / Nginx do bảng điều khiển tạo đã bao gồm cả hai tiêu đề (`header_up X-Forwarded-Host {host}` / `proxy_set_header X-Forwarded-Host $host;`); thêm chúng theo cách thủ công nếu bạn duy trì cấu hình proxy bằng tay.

## 7. Ghi chú

- **Định dạng và cấu hình được kết hợp chặt chẽ**: mẫu được `config`/`platform` chọn phải khớp với `format` của yêu cầu này; các mẫu không khớp sẽ bị bỏ qua - điều này ngăn việc đưa các quy tắc Mihomo vào sing-box/Xray JSON.
- **`providers=1` chỉ ảnh hưởng đến Mihomo YAML**: sing-box / Xray / base64 / raw bỏ qua nó (các nút vẫn nằm trong dòng).
- **Giới hạn tỷ lệ**: điểm cuối đăng ký có giới hạn riêng (`SubscribeRateLimit`); khách hàng lạm dụng bị từ chối.
- **Nhật ký truy cập**: mọi lần tìm nạp đều ghi lại thông tin IP / UA / CDN, hiển thị trong phần “Nhật ký truy cập đăng ký” trong giao diện người dùng quản trị viên.
- Nếu không có miền được định cấu hình trên cổng đăng ký chuyên dụng, `X-Forwarded-Proto` / `X-Forwarded-Host` của yêu cầu sẽ được sử dụng; đảm bảo proxy ngược của bạn chuyển tiếp cả hai tiêu đề.
- Trong triển khai đa miền, hãy giữ danh sách điểm cuối được căn chỉnh với các miền mà khách hàng thực sự sử dụng; xem §6.2.
