<!-- source: LEGAL.md | locale: vi | updated: 2026-10-23 -->
# Lưu ý về pháp lý và tuân thủ

> Bản dịch này chỉ nhằm mục đích tham khảo. Nếu có sai khác, bản gốc tiếng Trung sẽ được ưu tiên.

> Ngôn ngữ: [简体中文](LEGAL.md) · [English](LEGAL.en.md) · [繁體中文](LEGAL.zh-TW.md) · [Русский](LEGAL.ru.md) · [日本語](LEGAL.ja.md) · [한국어](LEGAL.ko.md) · [Deutsch](LEGAL.de.md) · [Français](LEGAL.fr.md) · [Español](LEGAL.es.md) · [Português](LEGAL.pt.md) · [Türkçe](LEGAL.tr.md) · [Bahasa Indonesia](LEGAL.id.md) · [ไทย](LEGAL.th.md) · Tiếng Việt · [العربية](LEGAL.ar.md) · [فارسی](LEGAL.fa.md)

> Tài liệu này giải thích giấy phép của CoreFusion, chính sách sử dụng thương mại, nghĩa vụ của bên thứ ba và các điểm tuân thủ mà người dùng nên xem trong khu vực pháp lý của riêng họ.
>
> **Tài liệu này không phải là tư vấn pháp lý.** Việc tuân thủ tùy thuộc vào khu vực triển khai, khách hàng và mô hình hoạt động của bạn; tham khảo ý kiến ​​​​của một luật sư địa phương có trình độ.
>
> *Bản dịch này được cung cấp để thuận tiện. Nếu nó khác với bản gốc tiếng Trung thì phiên bản tiếng Trung sẽ chiếm ưu thế.*

## 1. Giấy phép dự án

CoreFusion được phát hành theo **Giấy phép Công cộng GNU Affero v3.0 (AGPL-3.0)**; xem [LICENSE](./LICENSE) để biết toàn văn.

```
Copyright (C) 2026 qist

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published
by the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
```

## 2. Sử dụng thương mại và lưu trữ thương mại: được cho phép rõ ràng

**Bất kỳ cá nhân hoặc tổ chức nào cũng có thể sử dụng CoreFusion cho mục đích thương mại, bao gồm nhưng không giới hạn:**

- Tự lưu trữ và triển khai nội bộ;
- Chạy dịch vụ proxy/nút trả phí cho bên thứ ba;
- Cung cấp SaaS, dịch vụ lưu trữ, hoạt động được quản lý hoặc tích hợp hệ thống cho bên thứ ba có tính phí;
- Phát triển, tùy chỉnh và phân phối thứ cấp (bao gồm phân phối bằng phần cứng hoặc như một phần của giải pháp).

**Việc sử dụng thương mại và lưu trữ thương mại ở trên không yêu cầu ủy quyền riêng và không có phí giấy phép hoặc chia sẻ doanh thu.**

Các nghĩa vụ duy nhất là của chính AGPL-3.0, chủ yếu là hai:

1. **Khi phân phối các tệp nhị phân** (§4–§6): cung cấp cho người nhận Nguồn Tương ứng (nguồn tương ứng hoàn chỉnh) hoặc một đề nghị bằng văn bản có hiệu lực theo §6; giữ lại các thông báo bản quyền và văn bản giấy phép.
2. **Khi cung cấp phiên bản đã sửa đổi qua mạng** (§13): nếu bạn xuất bản một phiên bản đã sửa đổi và cho phép các bên thứ ba tương tác với phiên bản đó qua mạng, thì bạn phải **cung cấp miễn phí cho những người dùng đó Nguồn tương ứng của phiên bản đã sửa đổi** (ví dụ: liên kết kho lưu trữ/tải xuống nguồn nổi bật trong bảng điều khiển).

Ghi chú bổ sung:

- Việc triển khai một bản sao **chưa sửa đổi** và phục vụ người dùng tự nó không kích hoạt nghĩa vụ nguồn §13; phân phối nhị phân vẫn thuộc mục 1.
- Việc thu tiền không bị cấm; AGPL chi phối **nguồn sẵn có** chứ không phải mô hình kinh doanh. Bạn không thể từ chối §13 đối với phiên bản sửa đổi chỉ vì bạn hoạt động thương mại hoặc nguồn đóng.
- Nếu bạn muốn có **giấy phép thương mại nguồn đóng không tuân theo AGPL**, hãy liên hệ riêng với chủ sở hữu bản quyền. Lưu ý rằng dự án này liên kết các thành phần GPL-3.0 của bên thứ ba (xem phần 3); bất kỳ phiên bản nguồn đóng nào cũng phải loại bỏ chúng trước.

## 3. Các thành phần và phân phối lại của bên thứ ba

Các thành phần chính của bên thứ ba và giấy phép của chúng được liệt kê trong [THIRD_PARTY_LICENSES.md](LEGAL.vi.md).

Ghi chú đặc biệt:

- Thông qua `github.com/xtls/xray-core`, dự án này liên kết gián tiếp **GPL-3.0 trở lên** `github.com/sagernet/sing` và `sing-shadowsocks`. Do đó **các tệp nhị phân của bảng phân phối hoặc Tác nhân cũng yêu cầu đáp ứng các nghĩa vụ nguồn tương ứng của GPL-3.0**.
- Khi chạy, bảng điều khiển có thể tải xuống hoặc gọi các chương trình độc lập như Xray-core, sing-box và Caddy. Chúng **không được phân phối trong kho lưu trữ này**, nhưng nếu bạn gửi chúng cùng với sản phẩm phân phối của mình, bạn phải tuân thủ giấy phép của chúng (đặc biệt là GPL-3.0 của sing-box: cung cấp nguồn tương ứng). Hysteria2 và TUIC là các giao thức được sing-box thực hiện, không phải các giao thức nhị phân độc lập.
- Dữ liệu GeoIP/GeoSite đi kèm (GeoLite2, `geoip.dat`, `geosite.dat`) phải tuân theo giấy phép dữ liệu riêng; xác minh các điều khoản trước khi sử dụng thương mại.

## 4. Tuân thủ khu vực và trách nhiệm của người dùng

CoreFusion là **phần mềm đa năng**: nó không cung cấp bất kỳ proxy hoặc dịch vụ mạng nào, không cung cấp nút, băng thông hoặc thông tin xác thực và không thu phí. Hoạt động là trách nhiệm của người triển khai.

Người triển khai/điều hành phải xác định xem hoạt động kinh doanh của họ có yêu cầu giấy phép, hồ sơ hoặc đánh giá bảo mật trong phạm vi quyền hạn của họ hay không, bao gồm nhưng không giới hạn ở:

- **Trung Quốc đại lục**: giấy phép kinh doanh viễn thông giá trị gia tăng (ICP/IDC/ISP, v.v.), nộp ICP, MLPS, đánh giá bảo mật truyền dữ liệu xuyên biên giới và các nghĩa vụ theo Luật An ninh mạng, Luật bảo mật dữ liệu và Luật bảo vệ thông tin cá nhân. Không sử dụng phần mềm này cho các mục đích vi phạm luật pháp địa phương.
- **EU/Anh**: GDPR / GDPR của Vương quốc Anh, quyền riêng tư điện tử, nghĩa vụ của bộ xử lý/bộ kiểm soát theo Đạo luật dịch vụ kỹ thuật số (DSA), tuân thủ ghi nhật ký và cookie.
- **Hoa Kỳ**: luật về quyền riêng tư của tiểu bang, kiểm soát xuất khẩu mã hóa (EAR) và sàng lọc các biện pháp trừng phạt.
- **Các khu vực khác**: viễn thông địa phương, thông tin liên lạc được mã hóa, quản lý nội dung, bản địa hóa dữ liệu và các quy tắc bảo vệ người tiêu dùng.

Người dùng đồng ý không sử dụng phần mềm này cho:

- Các hoạt động vi phạm luật hiện hành hoặc các quy tắc kiểm soát/trừng phạt xuất khẩu;
- Xâm phạm quyền sở hữu trí tuệ, quyền riêng tư hoặc các quyền hợp pháp khác;
- Phân phối nội dung bất hợp pháp, tấn công mạng, lừa đảo hoặc hoạt động tội phạm khác.

**Mọi trách nhiệm pháp lý phát sinh từ việc sử dụng hoặc sử dụng sai mục đích phần mềm này thuộc về người dùng.**

## 5. Bảo vệ dữ liệu và quyền riêng tư

Bảng điều khiển có thể lưu trữ tài khoản, chi tiết liên hệ, đơn đặt hàng, gọi lại thanh toán, đăng ký, thống kê lưu lượng truy cập và nhật ký đăng nhập/hoạt động. Với tư cách là bộ xử lý/bộ điều khiển, người vận hành phải:

- Cung cấp cho người dùng chính sách quyền riêng tư và nhận được sự đồng ý cần thiết;
- Tuân theo việc giảm thiểu dữ liệu và đặt khoảng thời gian lưu giữ hợp lý cho nhật ký và số liệu thống kê (dự án cung cấp các chính sách dọn sạch nhật ký mà người vận hành phải định cấu hình);
- Hỗ trợ các yêu cầu truy cập, chỉnh sửa và xóa thông tin cá nhân;
- Áp dụng kiểm soát quyền truy cập và mã hóa vào cơ sở dữ liệu, bản sao lưu, khóa riêng của chứng chỉ và thông tin xác thực thanh toán;
- Hoàn thành đánh giá truyền dữ liệu xuyên biên giới khi triển khai xuyên biên giới.

## 6. Tuyên bố miễn trừ trách nhiệm

Phần mềm này được cung cấp “nguyên trạng” theo AGPL-3.0 §§15–17 mà không có bất kỳ bảo đảm rõ ràng hay ngụ ý nào, bao gồm khả năng bán được, sự phù hợp cho một mục đích cụ thể và không vi phạm. Người giữ bản quyền không chịu trách nhiệm về những thiệt hại trực tiếp, gián tiếp, ngẫu nhiên hoặc do hậu quả phát sinh từ việc sử dụng phần mềm này.

## 7. Liên hệ

- Dự án: <https://github.com/qist/corefusion>
- Các câu hỏi về tuân thủ/cấp phép thương mại: sử dụng Vấn đề của dự án hoặc chi tiết liên hệ được công bố trên trang kho lưu trữ.
