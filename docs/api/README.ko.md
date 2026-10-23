<!-- source: README.md | locale: ko | updated: 2026-10-23 -->
# CoreFusion API 문서

> 언어: [简体中文](README.md) · [English](README.en.md) · [繁體中文](README.zh-TW.md) · [Русский](README.ru.md) · [日本語](README.ja.md) · 한국어 · [Deutsch](README.de.md) · [Français](README.fr.md) · [Español](README.es.md) · [Português](README.pt.md) · [Türkçe](README.tr.md) · [Bahasa Indonesia](README.id.md) · [ไทย](README.th.md) · [Tiếng Việt](README.vi.md) · [العربية](README.ar.md) · [فارسی](README.fa.md)

**외부 통합자, 보조 개발 및 타사 클라이언트**를 위한 API 문서입니다. 모든 경로는 배포 도메인을 기준으로 합니다.

| 문서 | 청중 | 내용 |
|---|---|---|
| [subscription.md](./subscription.ko.md) | 구독자/클라이언트 개발자 | 구독 엔드포인트(`/sub/:token`): 형식, 헤더, 상태 코드, 예 |
| [portal-api.md](README.ko.md) | 포털 프런트엔드/타사 클라이언트 | 클라이언트 포털 API (`/api/v1/client-auth/*`, `/api/v1/public/*`): 가입, 로그인, 요금제, 주문, 잔액, 티켓 |
| [충전-콜백.md](./charge-callback.ko.md) | 결제 제공업체 | 결제 콜백 서명 및 확인 |
| [admin-api.ko.md](./admin-api.ko.md) | 패널 개발/자동화 | 전체 관리자 API 경로 테이블(`/api/v1/*`, 모듈별로 그룹화됨) |

---

## 1. 포트 및 기본 URL

패널은 동시에 세 개의 엔진을 노출하여 하나의 데이터베이스를 공유할 수 있습니다.

| 엔진 | 기본 액세스 | 목적 |
|---|---|---|
| 메인 패널 | `https://<domain>/api/v1` | 관리 백엔드 + 포털 + 구독 |
| 구독 엔진(전용 포트) | `https://<domain>:<port>/:token` | 동시성 구독 가져오기, `/sub` 접두사 없음 |
| 포털 엔진(전용 포트) | `https://<portal-domain>[:port]/api/v1` | 클라이언트 전용 포털, `/` 페이지 |

> 역방향 프록시는 페이지 경로와 `/api/v1`를 모두 전달해야 합니다. `X-Forwarded-Proto` / `X-Forwarded-Host`는 실제 구성표와 도메인을 복원하게 된 것을 영광으로 생각합니다.

## 2. 인증 (3가지 종류, 호환 불가)

| 인증 | 자격 증명 | 적용대상 |
|---|---|---|
| **관리자 인증** | `Authorization: Bearer <admin_token>`(`/api/v1/auth/login`에서) | 관리자 API |
| **클라이언트 인증** | `Authorization: Bearer <client_token>`(`/api/v1/client-auth/login`에서) | 포털 API |
| **구독 토큰** | URL의 토큰: `/sub/<token>` | 구독 가져오기, 헤더 없음 |

- 클라이언트 액세스 토큰은 24시간 동안 지속되고 새로 고침 토큰은 7일 동안 지속됩니다. JWT에는 `tv`(token_version)가 포함되어 있으므로 로그아웃/강제 로그아웃은 모든 이전 토큰을 무효화합니다.
- 관리자 역할: `admin` / `operator` / `super_admin`; 일부 쓰기 작업에는 `RequireRole(RoleAdmin)` 이상이 필요합니다.

## 3. 응답 형식

성공(대부분의 엔드포인트):

```json
{ "code": 0, "data": { ... }, "message": "ok" }
```

목록 끝점(일부)은 `{ "items": [...], "total": N }` 또는 기본 배열(아래 끝점별로 표시)을 반환합니다.

오류(HTTP 4xx/5xx):

```json
{ "code": "ERROR_CODE", "message": "human-readable description", "request_id": "…", "can_retry": false }
```

**금액은 센트 단위의 정수입니다**: `990` = 9.90엔(`currency: "CNY"`).
**트래픽은 바이트 단위입니다**: 요청과 응답 모두 바이트를 사용합니다.

## 4. 속도 제한 및 유지 관리 모드

- 공용 엔드포인트(`/api/v1/public/*`, `/sub/*`, 가입/로그인)는 속도가 제한됩니다.
- 로그인도 `LoginRateLimit`를 사용하고 구독 가져오기는 `SubscribeRateLimit`를 사용합니다.
- 유지 관리 모드에서는 공용 엔드포인트 및 구독이 유지 관리 메시지를 반환합니다. 관리자 API는 영향을 받지 않습니다.

## 5. 프런트엔드 페이지 경로(API 아님)

이러한 경로는 HTML/정적 자산을 반환하며 API 문서 외부에 있습니다. 통합 중에 제외될 수 있도록 나열됩니다.

| 방법 | 경로 | 설명 |
|---|---|---|
| GET | `/` | 관리자 패널 SPA(로그인 후) |
| GET | `/p` | 포털 HTML(전용 포털 엔진의 `/`) |
| GET | `/:token` | 전용 포트에서 구독 가져오기(`/sub` 접두사 없음, `/sub/:token`와 동일한 의미) |
| GET | `/static/:file` | 프런트엔드 정적 자산 |

## 6. 진실의 근원

`internal/api/router/router.go`(746개 경로, 743개 문서화). [admin-api.md](README.ko.md)의 경로 테이블은 코드에서 생성됩니다. 코드와 일치하지 않으면 **코드가 승리합니다**.
