<!-- source: README.md | locale: ko | updated: 2026-10-23 -->
# CoreFusion 문서

> 언어: [简体中文](README.md) · [English](README.en.md) · [繁體中文](README.zh-TW.md) · [Русский](README.ru.md) · [日本語](README.ja.md) · 한국어 · [Deutsch](README.de.md) · [Français](README.fr.md) · [Español](README.es.md) · [Português](README.pt.md) · [Türkçe](README.tr.md) · [Bahasa Indonesia](README.id.md) · [ไทย](README.th.md) · [Tiếng Việt](README.vi.md) · [العربية](README.ar.md) · [فارسی](README.fa.md)

## [USER-GUIDE.ko.md](./USER-GUIDE.ko.md) — 사용자 가이드

**사용자**(운영/관리자/운영자)를 위한 전체 운영 가이드: 설치, 초기 구성, 기능 모듈, FAQ 및 부록.

## [api/](./api/) — 공개 API 참조

통합업체, 2차 개발 및 제3자 클라이언트의 경우:

- [api/README.ko.md](./api/README.ko.md) — 개요: 기본 URL, 인증, 응답 형식, 속도 제한
- [api/subscription.ko.md](./api/subscription.ko.md) — 구독 엔드포인트(`/sub/:token`), 형식, 응답 헤더, 예
- [api/portal-api.ko.md](./api/portal-api.ko.md) — 클라이언트 포털 API (가입/로그인, 요금제, 주문, 잔액, 티켓)
- [api/charge-callback.ko.md](./api/charge-callback.ko.md) — 결제 콜백 서명 및 확인
- [api/admin-api.ko.md](./api/admin-api.ko.md) — 모듈별로 그룹화된 관리자 API 경로 테이블

## 디자인 문서

디자인 문서는 내부 자료(핵심 지적 재산)이며 이 저장소와 함께 **배포되지 않습니다**. 필요한 경우 관리자에게 문의하세요.

> **범위**: 클라이언트 앱(로컬 UI, TUN 인계, 트레이, 다중 구독 병합)은 별도의 제품이며 이 저장소의 범위를 벗어납니다. 여기 문서는 서버 측(패널/노드/구독 전달)만 다룹니다.

## 기타 운영 가이드

- `cert-google-eab.md` — Google EAB 인증서 발급 *(중국어)*
- `oauth-google-verification.md` — Google OAuth 확인 *(중국어)*
- `notification-templates.md` — 알림 템플릿 변수 *(중국어)*

## 번역 범위

| 문서 | zh-CN | en | zh-TW | 기타 지역 |
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

이름 지정, 언어 탐색 및 검토 규칙 *(중국어)*는 [I18N.md](README.ko.md)를 참조하세요.
