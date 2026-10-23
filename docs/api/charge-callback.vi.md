<!-- source: charge-callback.md | locale: vi | updated: 2026-10-23 -->
# Tính phí gọi lại API

> Ngôn ngữ: [简体中文](charge-callback.md) · [English](charge-callback.en.md) · [繁體中文](charge-callback.zh-TW.md) · [Русский](charge-callback.ru.md) · [日本語](charge-callback.ja.md) · [한국어](charge-callback.ko.md) · [Deutsch](charge-callback.de.md) · [Français](charge-callback.fr.md) · [Español](charge-callback.es.md) · [Português](charge-callback.pt.md) · [Türkçe](charge-callback.tr.md) · [Bahasa Indonesia](charge-callback.id.md) · [ไทย](charge-callback.th.md) · Tiếng Việt · [العربية](charge-callback.ar.md) · [فارسی](charge-callback.fa.md)

Đối với **nền tảng thanh toán / nhà tích hợp trang web**: sau khi người dùng thanh toán trên một trang web được gắn kết, máy chủ của nền tảng thanh toán sẽ thông báo cho bảng điều khiển.

## 1. Điểm cuối

```
POST /api/v1/charge/callback/{site_id}
Content-Type: application/json
```

`site_id` là ID (số nguyên) “trang web được gắn” phía quản trị viên. Lệnh gọi lại được thực hiện từ máy chủ đến máy chủ và sử dụng **ký HMAC** cùng với danh sách cho phép IP tùy chọn thay vì xác thực Bearer.

## 2. Nội dung yêu cầu

```json
{
  "trade_no":       "CF20261010120000abcd1234",
  "site_trade_no":  "payment-platform-order-no",
  "status":         "success",
  "amount":         990,
  "currency":       "CNY",
  "timestamp":      1791640000,
  "nonce":          "random-unique-string",
  "paid_currency":  "CNY",
  "paid_amount":    990,
  "method":         "alipay",
  "exchange_rate":  1.0,
  "paid_at":        "2026-10-10 12:00:01",
  "sign":           "hex(signature)",
  "extra":          {}
}
```

| trường | Mô tả |
|---|---|
| `trade_no` | Số giao dịch phía bảng điều khiển (được phát hành khi tạo đơn hàng, tiền tố `CF`) |
| `site_trade_no` | Số thứ tự nền tảng thanh toán (đã ký) |
| `status` | Tình trạng doanh nghiệp, **chưa ký** |
| `amount` / `currency` | Số tiền đặt hàng (xu nguyên) và tiền tệ (cả hai đều đã ký) |
| `timestamp` | giây Unix; bị từ chối bên ngoài cửa sổ (đã ký) |
| `nonce` | Bảo vệ phát lại: INSERT nguyên tử, mỗi lần xử lý không được xử lý một lần (đã ký) |
| `paid_*` / `method` / `exchange_rate` / `paid_at` | Thông tin thanh toán thực tế, **không được ký** (tính toàn vẹn được đảm bảo bởi `body_hash`) |
| `sign` | Chữ ký HMAC-SHA256, được mã hóa hex (được tính trên phần thân mà không có `sign`) |
| `extra` | Trường mở rộng |

## 3. Thuật toán ký (v3.2 / `version: "v1"`)

1. Xây dựng **phần thân thô không có `sign`** và tính toán `body_hash = SHA256(raw body bytes)` (hex chữ thường).
   > Hội đồng cũng chấp nhận sơ đồ kế thừa băm toàn bộ nội dung bao gồm `sign`; tích hợp mới nên tuân theo chương trình tài liệu.
2. Xây dựng tải trọng ký (**các khóa theo thứ tự từ điển**, ngữ nghĩa Go `json.Marshal`):

```json
{
  "amount": 990,
  "body_hash": "<hash from step 1>",
  "currency": "CNY",
  "nonce": "<nonce>",
  "site_trade_no": "<site_trade_no>",
  "timestamp": 1791640000,
  "trade_no": "<trade_no>",
  "version": "v1"
}
```

3. `sign = hex( HMAC_SHA256( key=callback secret, msg=JSON bytes from step 2) )`.
   > Bí mật là bí mật gọi lại của trang được gắn trong giao diện người dùng quản trị viên (sau khi xoay, bí mật trước đó được giữ cho một cửa sổ chuyển tiếp).

```python
import hashlib, hmac, json

def sign_callback(body: dict, secret: str) -> str:
    # 1) body_hash first: body must not contain sign here
    body_hash = hashlib.sha256(json.dumps(body, separators=(",", ":"), ensure_ascii=False).encode()).hexdigest()
    # 2) signing payload (key order irrelevant: json.dumps(sort_keys=True))
    payload = {
        "amount": body["amount"], "body_hash": body_hash,
        "currency": body["currency"], "nonce": body["nonce"],
        "site_trade_no": body["site_trade_no"], "timestamp": body["timestamp"],
        "trade_no": body["trade_no"], "version": "v1",
    }
    msg = json.dumps(payload, separators=(",", ":"), sort_keys=True).encode()
    return hmac.new(secret.encode(), msg, hashlib.sha256).hexdigest()
```

## 4. Chuỗi xác thực phía bảng điều khiển (xác định lỗi bạn gặp phải)

1. Trang web tồn tại và tên miền của nó được xác minh → 404 / 403
2. Danh sách cho phép IP (nếu `callback_whitelist` được định cấu hình, các IP không được liệt kê sẽ **bị chặn cứng**; nếu không thì bị bỏ qua và chỉ kiểm tra chữ ký)
3. Cửa sổ `timestamp` + Bảo vệ phát lại `nonce` (CHÈN nguyên tử)
4. Xác minh HMAC (chấp nhận cả bí mật đang hoạt động và bí mật kế thừa)
5. Đơn hàng tồn tại và thuộc về trang web → idempotency (đã thanh toán trả lại thành công) → **xác thực khóa số tiền và tiền tệ** → đăng giao dịch (sổ cái số dư + trạng thái đơn hàng)

Quá trình xử lý **không có hiệu lực**: các lệnh gọi lại lặp lại với cùng một `trade_no` sẽ không được đăng hai lần. Những điểm bất thường về chữ ký/tỷ lệ được ghi lại dưới dạng "sự kiện gọi lại" mà quản trị viên có thể kiểm tra và thử lại theo cách thủ công.

## 5. Phản hồi

- Thành công: 2xx (sau khi bảng đăng thanh toán)
- Lỗi: 4xx/5xx + `{ "code": "...", "message": "..." }`; các mã điển hình bao gồm `INVALID_SIGNATURE`, `IP_BLOCKED`, `ORDER_NOT_FOUND`, số lượng không khớp, v.v.

> Mọi cuộc gọi lại đều mang theo `X-Request-ID` (do bảng điều khiển tạo ra nếu không có); nhật ký và sự kiện có thể theo dõi được từ đầu đến cuối. Cung cấp ID này cho quản trị viên của bạn khi khắc phục sự cố.

## 6. Kiểm tra và khắc phục sự cố

- Quản trị viên có thể tìm nạp **tài liệu tích hợp** của mỗi trang web: `GET /api/v1/charge-sites/{id}/integration-doc` (bao gồm các ví dụ gọi lại URL thực tế của trang web).
- Sự kiện gọi lại: `GET /api/v1/charge-callback-events`; thử lại: `GET /api/v1/charge-callback-retries`, `POST /api/v1/charge-callback-retries/{id}/retry`.
- Xoay bí mật: `POST /api/v1/charge-sites/{id}/rotate-secret` (bí mật trước đó được giữ cho một cửa sổ chuyển tiếp).
