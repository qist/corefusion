<!-- source: cert-google-eab.md | locale: vi | updated: 2026-10-23 -->
# Hướng dẫn CA công khai Google (phát hành EAB tự động)

> Ngôn ngữ: [简体中文](cert-google-eab.md) · [English](cert-google-eab.en.md) · [繁體中文](cert-google-eab.zh-TW.md) · [Русский](cert-google-eab.ru.md) · [日本語](cert-google-eab.ja.md) · [한국어](cert-google-eab.ko.md) · [Deutsch](cert-google-eab.de.md) · [Français](cert-google-eab.fr.md) · [Español](cert-google-eab.es.md) · [Português](cert-google-eab.pt.md) · [Türkçe](cert-google-eab.tr.md) · [Bahasa Indonesia](cert-google-eab.id.md) · [ไทย](cert-google-eab.th.md) · Tiếng Việt · [العربية](cert-google-eab.ar.md) · [فارسی](cert-google-eab.fa.md)

> Áp dụng cho: bảng v0.12.101+ | Trang: **Chứng chỉ → Cấu hình EAB**
> Mã số: `internal/service/certificate/eab_google.go`, `internal/api/handlers/certificates/eab_config.go`

## 1. Tại sao Google yêu cầu EAB

| CA | EAB có bắt buộc đối với ACME không? | Làm thế nào để có được EAB |
|---|---|---|
| Hãy mã hóa | ❌ Không | — |
| ZeroSSL | ✅ Có | **Đổi một email lấy một email** (`api.zerossl.com/acme/eab-credentials-email`, giống như acme.sh); bảng điều khiển có thể tự động tìm nạp nó |
| **Google CA công cộng** | ✅ **Có** | **Phải được tạo bằng ủy quyền của GCP** (email là không đủ) → trọng tâm của trang này |

Nói cách khác, Google **không thể tránh khỏi "ủy quyền"**: tệp JSON của tài khoản dịch vụ hoặc mã thông báo truy cập tạm thời.

## 2. Quy tắc cứng của Google (đọc trước)

| quy tắc | Mô tả |
|---|---|
| **Sử dụng trong vòng 7 ngày** | Khóa EAB **hết hạn** nếu không được sử dụng trong vòng 7 ngày và phải được tạo lại |
| **Chỉ một tài khoản** | Một khóa EAB đăng ký chính xác một tài khoản ACME; sau khi đăng ký, khóa **không hợp lệ** và không thể sử dụng lại |
| **Tài khoản ACME không hết hạn** | Tài khoản ACME được tạo bằng tài khoản này **tồn tại lâu** → **không cần gia hạn EAB** |
| ⚠ **Xóa dự án GCP sẽ giết chết tất cả tài khoản** | Việc xóa dự án sẽ ngay lập tức làm mất hiệu lực **tất cả các tài khoản ACME** trong dự án đó và **không thể gia hạn** |

> Kết luận: **EAB là "chứng chỉ đăng nhập" một lần; tài khoản ACME sẽ tồn tại lâu dài.**
> Bảng điều khiển lưu trữ các tài khoản tại `data/acme/accounts/<ca>.json` — **sao lưu thư mục này**.

## 3. Ba đường dẫn (bắt đầu bằng đường dẫn A - không tìm kiếm menu)

> Điểm truy cập Cloud Console đã thay đổi nhiều lần, do đó, các liên kết trực tiếp và Cloud Shell được ưu tiên hàng đầu; đường dẫn menu là dự phòng.

### Đường dẫn A (dễ nhất, được đề xuất trước): một lệnh trong GCP Cloud Shell

Cloud Shell là một thiết bị đầu cuối trình duyệt có **gcloud được cài đặt sẵn và đã đăng nhập**; không có thiết lập cục bộ và không có menu.

1. Mở <https://console.cloud.google.com/cloudshell> (hoặc biểu tượng `>_` ở phía trên bên phải của bảng điều khiển)
2. Chọn dự án (ID dự án hiển thị trong bộ chọn dự án ở trên cùng):
   ```bash
   gcloud config set project <your-project-id>
   ```
3. Kích hoạt CA công khai API (**tương đương** để kích hoạt nó trong Thư viện API):
   ```bash
   gcloud services enable publicca.googleapis.com
   ```
4. Tạo EAB (**đây là hai chuỗi bạn cần**):
   ```bash
   gcloud publicca external-account-keys create
   ```
 Ở đầu ra: `keyId` → **EAB-KID** của bảng điều khiển; `b64MacKey` → **EAB-HMAC-Key** của bảng điều khiển
5. Quay lại bảng điều khiển: **Chứng chỉ → Cấu hình EAB → Thêm cấu hình EAB → CA: Google CA công cộng** →
 nhập hai giá trị vào `EAB-KID` / `EAB-HMAC-Key` → lưu ✓

> ⚠ Chi phí: Google EAB key là **7 ngày, một lần** ✗ → sau 7 ngày (hoặc khi chuyển đổi tài khoản) **lặp lại bước 4**.
> Để tránh điều đó, hãy sử dụng đường dẫn B (lưu trữ tài khoản dịch vụ một lần và để bảng điều khiển tự động yêu cầu lại).

### Đường dẫn B (thiết lập một lần, sau đó tự động): khóa tài khoản dịch vụ JSON

**① Kích hoạt CA công khai API** (liên kết trực tiếp là dễ nhất)
<https://console.cloud.google.com/apis/library/publicca.googleapis.com?project=YOUR_PROJECT_ID> → nhấp vào “Bật”
Dự phòng: ☰ → **API & Dịch vụ → Thư viện** → tìm kiếm `Public CA` → mở → “Bật”

**② Tạo tài khoản dịch vụ**
<https://console.cloud.google.com/iam-admin/serviceaccounts?project=YOUR_PROJECT_ID>
→ “**+ Tạo tài khoản dịch vụ**” → bất kỳ tên nào (ví dụ: `corefusion-eab`) → “Tạo và tiếp tục” → bạn có thể **bỏ qua** bước vai trò → “Xong”
Dự phòng: ☰ → **IAM & Quản trị viên → Tài khoản dịch vụ**

**③ Cấp vai trò tối thiểu** (dễ nhất trên cùng một trang)
Nhấp vào tài khoản mới tạo → **“tab Quyền”** → “Cấp quyền truy cập” → hiệu trưởng mới: **email tài khoản dịch vụ**
→ tìm kiếm vai trò cho `publicca` → chọn **“Trình tạo khóa tài khoản bên ngoài CA công khai”** → lưu
Dự phòng: ☰ → **IAM & Quản trị viên → IAM** → “Cấp quyền truy cập” → cùng một email + vai trò
⚠ **Không cấp Chủ sở hữu/Người chỉnh sửa** ✗

**④ Tải khóa JSON**
Tài khoản dịch vụ → mở tài khoản → **“Keys” tab** → “**Thêm khóa**” → “**Tạo khóa mới**” → **JSON** → tạo
(trình duyệt tải xuống `xxx.json`; hoặc hàng `⋮` → “Quản lý khóa” → cùng một luồng)

**⑤ Quay lại bảng**: nhập **ID dự án** và dán **toàn bộ nội dung** của JSON vào “Tệp khóa tài khoản dịch vụ” → nhấp vào “Yêu cầu EAB tự động”
→ sau đó bảng điều khiển **tự động yêu cầu lại EAB** khi nó hết hạn ✓, không còn lượt truy cập GCP nữa ✓

### Đường dẫn C (chỉ kiểm tra một lần): dán mã thông báo truy cập

Trong Cloud Shell:
```bash
gcloud auth print-access-token
```
Dán đầu ra vào “Mã thông báo truy cập tạm thời” của bảng điều khiển cùng với ID dự án → nhấp vào “Tự động yêu cầu EAB”
⚠ Có hiệu lực trong khoảng 1 giờ và **không được lưu trữ** ✓ → chỉ xác minh, không tự động yêu cầu lại ✗

### Bảng cheat khi bạn không thể tìm thấy điểm vào (menu bảng điều khiển thay đổi)

| Nhiệm vụ | Link trực tiếp (đáng tin cậy nhất) | Dự phòng: menu |
|---|---|---|
| Kích hoạt CA công khai API | `https://console.cloud.google.com/apis/library/publicca.googleapis.com?project=<PROJECT_ID>` | ☰ → API & Dịch vụ → Thư viện → tìm kiếm `Public CA` |
| Tài khoản dịch vụ | `https://console.cloud.google.com/iam-admin/serviceaccounts?project=<PROJECT_ID>` | ☰ → IAM & Quản trị viên → Tài khoản dịch vụ |
| Vai trò cấp | `https://console.cloud.google.com/iam-admin/iam?project=<PROJECT_ID>` | ☰ → IAM & Quản trị viên → IAM → Cấp quyền truy cập |
| Vỏ đám mây | `https://console.cloud.google.com/cloudshell` | Biểu tượng `>_` ở phía trên bên phải |

> Mẹo: thay thế `<PROJECT_ID>` / `YOUR_PROJECT_ID` bằng ID dự án thực (như `my-project-123456`) để chuyển thẳng đến trang.

## 4. Cấu hình bảng điều khiển (hai cách)

Đi tới **Chứng chỉ → Cấu hình EAB → Thêm cấu hình EAB** và chọn **Google CA công cộng**:

### Tùy chọn A (được khuyến nghị): dán tài khoản dịch vụ JSON

| trường | Giá trị |
|---|---|
| Mã dự án GCP | ID dự án từ phần 3, ví dụ: `my-project-123456` |
| Tài khoản dịch vụ JSON | Dán **toàn bộ nội dung** của `corefusion-eab.json` |
| Mã thông báo truy cập tạm thời | Để trống |
| Tên | Được tạo tự động dưới dạng `Google Public CA - <project-id>`; có thể chỉnh sửa được |

Nhấp vào **Yêu cầu EAB tự động** → nếu thành công, khóa KID/HMAC sẽ được điền và lưu; sau này chỉ cần chọn `CA = Google Public CA` khi phát hành (EAB được liên kết với mỗi CA; không có lựa chọn thủ công).

> Tài khoản dịch vụ JSON được **mã hóa vào cấu hình EAB của cơ sở dữ liệu bảng** (`json:"-"`, **không bao giờ quay trở lại giao diện người dùng**).
> Mục đích là **tự động yêu cầu lại khóa sau khi hết hạn 7 ngày** thay vì truy cập GCP thủ công.

### Tùy chọn B: mã thông báo truy cập tạm thời (một lần)

```bash
gcloud auth print-access-token        # valid ~1 hour
```

Dán nó vào “Mã thông báo truy cập tạm thời”, điền ID dự án, nhấp vào **Tự động yêu cầu EAB**.
⚠ Mã thông báo này **không được lưu trữ** và phải được lấy lại sau khi hết hạn → bị phạt vì thử nghiệm một lần, không được sử dụng lâu dài.

### Dòng lệnh tương đương (tùy chọn, để so sánh)

```bash
gcloud publicca external-account-keys create
# returns keyId / b64MacKey, corresponding to ACME's --eab-kid / --eab-hmac-key
```

## 4.5 Tại sao JSON thay vì "một phím duy nhất"?

Đây là câu hỏi phổ biến nhất. Tách biệt hai loại “chìa khóa”:

| Những gì bạn có thể mong đợi | Điều thực sự cần thiết |
|---|---|
| **Khóa API** (chuỗi `AIza…`) | ❌ **Vô dụng** — khóa API chỉ là mã định danh công khai và **không mang danh tính**; việc tạo EAB cần **quyền IAM**, vì vậy cần có thông tin xác thực danh tính |
| — | ✅ **Khóa tài khoản dịch vụ**: **tệp** được tải xuống từ GCP “IAM → Tài khoản dịch vụ → Khóa → Tạo khóa mới → **JSON**” |

Điểm mấu chốt: **bản thân tệp JSON là "một khóa"** — GCP gói danh tính tài khoản (`client_email`) cộng với khóa riêng tư (`private_key`) vào một tệp. Bảng điều khiển ký JWT với nó → trao đổi lấy mã thông báo truy cập OAuth2 → gọi CA công khai API.
Vì vậy, hãy “dán JSON” ≠ điền vào nhiều trường; **dán toàn bộ tệp** (bảng điều khiển cũng điền vào các trường tùy chọn như `type` / `token_uri`).

**EAB thực là hai chuỗi** (`EAB-KID` + `EAB-HMAC-Key`), là **kết quả** của yêu cầu; bảng điều khiển sẽ tự động điền chúng vào.

### Bạn không muốn bàn giao hồ sơ? Hai lựa chọn thay thế

| phương pháp | Làm thế nào | Chi phí |
|---|---|---|
| **Mã thông báo truy cập tạm thời** | `gcloud auth print-access-token` → dán vào “Mã thông báo truy cập tạm thời” | ~1 giờ có hiệu lực, **không được lưu trữ** → chỉ một lần, không tự động yêu cầu lại |
| **KID/HMAC thủ công** | Chạy cục bộ `gcloud publicca external-account-keys create` → nhập `keyId` / `b64MacKey` vào “EAB-KID / EAB-HMAC-Key” | Không có thông tin xác thực được bàn giao ✓; nhưng Google EAB là **7 ngày, một lần** → lặp lại sau khi hết hạn |

> Hội đồng khuyến nghị tài khoản dịch vụ JSON vì một lý do: **EAB hết hạn sau 7 ngày** và chỉ tài khoản dịch vụ được lưu trữ mới cho phép **tự động yêu cầu lại** mà không cần thao tác thủ công.

## 4.6 Câu hỏi thường gặp: Tôi có thể sử dụng khóa API không?

**Không.** Đây không phải là lựa chọn triển khai — **Google từ chối nó một cách rõ ràng** (đã xác minh cả hai cách):

```bash
# 1) Query parameter
curl -X POST "https://publicca.googleapis.com/v1/projects/<ID>/locations/global/externalAccountKeys?key=AIza..." -d '{}'
# 2) x-goog-api-key header recommended by the official docs
curl -X POST "https://publicca.googleapis.com/v1/projects/<ID>/locations/global/externalAccountKeys" \
     -H "x-goog-api-key: AIza..." -d '{}'
```

Cả hai đều trở lại:

```json
{"error":{"code":401,"status":"UNAUTHENTICATED",
  "message":"API keys are not supported by this API. Expected OAuth2 access token
             or other authentication credentials that assert a principal."}}
```

Để so sánh, **(giả) OAuth Bearer** trả về
`Request had invalid authentication credentials. Expected OAuth 2 access token...` —
vì vậy OAuth được nhận dạng trong khi **khóa API bị từ chối hoàn toàn** ✗.

**Tại sao**: tạo EAB là một **hoạt động hành chính** đòi hỏi phải biết “bạn là ai và liệu bạn có `roles/publicca.externalAccountKeyCreator`” ✗
— khóa API chỉ xác định một dự án/ứng dụng ✓ và **không thể hiện danh tính chính** ✓.
("Các phương pháp hay nhất để quản lý khóa API" của Google khuyến nghị **IAM + thông tin xác thực tài khoản dịch vụ tồn tại trong thời gian ngắn** cho các tình huống quản trị ✓ — cách tiếp cận tương tự được sử dụng ở đây ✓.)

**Vì vậy chỉ có ba đường dẫn** ✓ (xem §3): tài khoản dịch vụ JSON ✓ / mã thông báo truy cập tạm thời ✓ /
hoặc **không có thông tin xác thực nào** — chạy `gcloud publicca external-account-keys create` trong Cloud Shell ✓
và nhập `keyId` / `b64MacKey` vào bảng ✓ (**sử dụng thông tin này nếu GCP IAM chặn bạn** ✓).

## 5. Tự động đăng ký và gia hạn (quan trọng)

```
Panel: CA = Google Public CA
  ├─ usable EAB present in the config → use it
  └─ missing/expired → request a new one on the spot with the stored service-account JSON (auto-registration)
        │
First issuance on the node/panel ── registers the ACME account with EAB (one-time!)
        │
Account persisted: data/acme/accounts/google.json (panel) / <Agent data_dir>/acme/accounts/google.json (node)
        │
Every later renewal ── uses the account key only; **EAB is never needed again**
```

Vì vậy:

- **Lần phát hành đầu tiên được đăng ký tự động**; không cần bước GCP thủ công;
- Việc gia hạn **không** tiêu tốn EAB (miễn là tệp tài khoản tồn tại và thư mục dữ liệu ổn định);
- ⚠ Thư mục dữ liệu ACME phía Tác nhân là đường dẫn tuyệt đối cố định (`/var/lib/corefusion-agent/acme` mặc định);
 **không tự ý thay đổi**, nếu không nó sẽ được coi là tài khoản mới → cần có khóa EAB khác.

## 6. Khắc phục sự cố

| Triệu chứng/lỗi | nguyên nhân | sửa chữa |
|---|---|---|
| `PERMISSION_DENIED` (403) | Tài khoản dịch vụ thiếu `roles/publicca.externalAccountKeyCreator` | Quay lại phần 3 ④ |
| `SERVICE_DISABLED` (403) | `publicca.googleapis.com` chưa được kích hoạt cho dự án | Mục 3 ② |
| `NOT_FOUND` (404) | ID dự án sai/dự án đã bị xóa | Xác minh ID dự án |
| Báo cáo phát hành `externalAccountRequired` | CA này không có EAB | Yêu cầu/cấu hình EAB trong bảng điều khiển trước |
| Báo cáo phát hành `Accounts must have at least one contact.` | Không có email được cung cấp trong quá trình đăng ký | Đặt email tài khoản ACME trên trang chứng chỉ (bảng sử dụng tài khoản mặc định) |
| Gia hạn yêu cầu EAB lại / đăng ký không thành công | Tệp tài khoản ACME bị mất (thư mục dữ liệu đã thay đổi/làm sạch) | **Khôi phục bản sao lưu**; không có, hãy yêu cầu EAB mới bằng tài khoản dịch vụ |
| Nhà phát hành thay đổi từ Google thành Let's Encrypt | Lệnh gia hạn thiếu CA (lỗi cũ, đã sửa trong v0.12.101) | Nâng cấp và phát hành lại |

## 7. Khuyến nghị về an ninh và vận hành

1. **Đặc quyền tối thiểu**: chỉ cấp tài khoản dịch vụ `roles/publicca.externalAccountKeyCreator`, không bao giờ cấp Chủ sở hữu/Người chỉnh sửa.
2. **Không xóa dự án**: xóa dự án GCP sẽ làm mất hiệu lực tất cả các tài khoản ACME của nó → chứng chỉ không thể gia hạn.
3. **Luôn sao lưu** `data/acme/accounts/` (chính danh tính tài khoản) và cấu hình EAB của cơ sở dữ liệu bảng điều khiển.
4. **Đừng lo lắng về việc EAB hết hạn**: miễn là tệp tài khoản ACME tồn tại, việc gia hạn sẽ không bị ảnh hưởng; chỉ cần một EAB mới để đăng ký tài khoản mới.
5. Nếu khóa tài khoản dịch vụ có thể đã bị rò rỉ: hãy xóa khóa đó trong GCP và dán JSON mới vào bảng điều khiển.

## 8. Điểm cuối liên quan (đối với tập lệnh)

| Điểm cuối | Mô tả |
|---|---|
| `POST /api/v1/eab-configs/fetch-google` | Tạo EAB bằng ủy quyền GCP. Thân máy: `{project_id, service_account_json?, access_token?}` |
| `POST /api/v1/eab-configs/fetch-zerossl` | Nhận ZeroSSL EAB bằng email. Thân máy: `{email}` |
| `GET  /api/v1/eab-configs` | Liệt kê các cấu hình EAB (**không có** khóa HMAC hoặc tài khoản dịch vụ JSON; chỉ `has_mac_key` / `has_google_sa`) |
