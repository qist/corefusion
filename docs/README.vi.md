<!-- source: README.md | locale: vi | updated: 2026-10-23 -->
# Tài liệu CoreFusion

> Ngôn ngữ: [简体中文](README.md) · [English](README.en.md) · [繁體中文](README.zh-TW.md) · [Русский](README.ru.md) · [日本語](README.ja.md) · [한국어](README.ko.md) · [Deutsch](README.de.md) · [Français](README.fr.md) · [Español](README.es.md) · [Português](README.pt.md) · [Türkçe](README.tr.md) · [Bahasa Indonesia](README.id.md) · [ไทย](README.th.md) · Tiếng Việt · [العربية](README.ar.md) · [فارسی](README.fa.md)

## [USER-GUIDE.vi.md](./USER-GUIDE.vi.md) — Hướng dẫn sử dụng

Hướng dẫn vận hành đầy đủ dành cho **người dùng** (vận hành / quản trị viên / người vận hành): cài đặt, cấu hình ban đầu, mô-đun tính năng, Câu hỏi thường gặp và phụ lục.

## [api/](./api/) - Tham chiếu API công khai

Đối với các nhà tích hợp, phát triển thứ cấp và khách hàng bên thứ ba:

- [api/README.vi.md](./api/README.vi.md) - tổng quan: cơ sở URL, xác thực, định dạng phản hồi, giới hạn tốc độ
- [api/subscription.vi.md](./api/subscription.vi.md) — điểm cuối đăng ký (`/sub/:token`), định dạng, tiêu đề phản hồi, ví dụ
- [api/portal-api.vi.md](./api/portal-api.vi.md) — cổng thông tin khách hàng API (đăng ký / đăng nhập, gói, đơn đặt hàng, số dư, vé)
- [api/charge-callback.vi.md](./api/charge-callback.vi.md) - xác minh và chữ ký gọi lại thanh toán
- [api/admin-api.vi.md](./api/admin-api.vi.md) — bảng lộ trình API của quản trị viên được nhóm theo mô-đun

## Tài liệu thiết kế

Tài liệu thiết kế là tài liệu nội bộ (sở hữu trí tuệ cốt lõi) và **không được phân phối** cùng với kho lưu trữ này. Liên hệ với người bảo trì nếu bạn cần chúng.

> **Phạm vi**: ứng dụng khách (giao diện người dùng cục bộ, tiếp quản TUN, khay, hợp nhất nhiều đăng ký) là một sản phẩm riêng biệt và nằm ngoài phạm vi của kho lưu trữ này. Tài liệu ở đây chỉ bao gồm phía máy chủ (bảng điều khiển/nút/phân phối đăng ký).

## Hướng dẫn hoạt động khác

- `cert-google-eab.md` — Phát hành chứng chỉ Google EAB *(tiếng Trung)*
- `oauth-google-verification.md` — Xác minh OAuth Google *(tiếng Trung)*
- `notification-templates.md` — biến mẫu thông báo *(tiếng Trung)*

## Phạm vi dịch thuật

| tài liệu | zh-CN | en | zh-TW | Các địa phương khác |
|----------|-------|----|-------|---------------|
| README.md | ✅ | ✅ | ✅ | ru ✅ |
| USER-GUIDE.md | ✅ | ✅ | ✅ | ru ✅ |
| I18N.md | ✅ | — | — | — |
| api/README.md | ✅ | ✅ | ✅ | ru ✅ |
| api/subscription.md | ✅ | ✅ | ✅ | ru ✅ |
| api/portal-api.md | ✅ | ✅ | ✅ | ru ✅ |
| api/charge-callback.md | ✅ | ✅ | ✅ | ru ✅ |
| api/admin-api.md | ✅ | ✅ | ✅ | ru ✅ |
| cert-google-eab.md | ✅ | ✅ | ✅ | ru ✅ |
| oauth-google-verification.md | ✅ | ✅ | ✅ | ru ✅ |
| notification-templates.md | ✅ | ✅ | ✅ | ru ✅ |
| LEGAL.md | ✅ | ✅ | ✅ | ru ✅ |
| THIRD_PARTY_LICENSES.md | ✅ | ✅ | ✅ | ru ✅ |

Xem [I18N.md](README.vi.md) để biết các quy tắc đặt tên, điều hướng ngôn ngữ và đánh giá *(tiếng Trung)*.
