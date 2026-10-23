<!-- source: admin-api.md | locale: ko | updated: 2026-10-23 -->
# 관리자 API

> 언어: [简体中文](admin-api.md) · [English](admin-api.en.md) · [繁體中文](admin-api.zh-TW.md) · [Русский](admin-api.ru.md) · [日本語](admin-api.ja.md) · 한국어 · [Deutsch](admin-api.de.md) · [Français](admin-api.fr.md) · [Español](admin-api.es.md) · [Português](admin-api.pt.md) · [Türkçe](admin-api.tr.md) · [Bahasa Indonesia](admin-api.id.md) · [ไทย](admin-api.th.md) · [Tiếng Việt](admin-api.vi.md) · [العربية](admin-api.ar.md) · [فارسی](admin-api.fa.md)

**패널 개발/자동화/통합업체**를 위한 완전한 관리자 API 경로 테이블입니다.

- 베이스 URL: `https://<panel-domain>/api/v1`
- 저자: `Authorization: Bearer <admin_token>`(`POST /api/v1/auth/login`에서)
- 일반적인 응답/오류 형식 및 속도 제한 규칙: [README.md](./README.ko.md)를 참조하세요.
- 정보 출처: `internal/api/router/router.go`(관리 경로 739개); 이 테이블이 동의하지 않으면 **코드가 승리합니다**

## 권한 표시

| 마커 | 의미 |
|---|---|
| 공개 | 로그인 필요 없음(유지 관리 가드/속도 제한이 여전히 적용될 수 있음) |
| 공개 · 요금 제한 | 로그인 없음, 전용 비율 제한 있음 |
| 로그인됨 | 로그인된 관리자 토큰이 필요합니다(`AuthRequired`) |
| 관리자 | 역할 ≥ `admin`(`RequireRole(RoleAdmin)`) 필요 |
| 최고 관리자 | 역할 필요 = `super_admin` |

역할 수준: `viewer(10) < operator(20) < admin(30) < super_admin(40)`; 역할이 부족하면 403 `INSUFFICIENT_ROLE`가 반환됩니다.

---

## 빠른 시작(실제 예)

이러한 예는 실제 지역 응답에서 나온 것입니다. 토큰과 데이터가 수정됩니다.

### 1. 로그인하고 토큰 받기

```bash
curl -X POST https://<panel-domain>/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"your-password"}'
```

응답:

```json
{
  "data": {
    "access_token": "<JWT, valid 24h>",
    "refresh_token": "<JWT, valid 7 days>",
    "user": { "id": 1, "role": "super_admin", "username": "admin" }
  }
}
```

### 2. 토큰을 사용하여 비즈니스 엔드포인트 호출

```bash
curl https://<panel-domain>/api/v1/clients?limit=1 \
  -H 'Authorization: Bearer <access_token>'
```

응답(목록은 `items` + `total` 구조를 사용함):

```json
{
  "data": {
    "items": [
      {
        "id": 27,
        "email": "user@example.com",
        "status": "active",
        "quota_bytes": 10737418240,
        "used_bytes": 0,
        "plan_id": 1,
        "expires_at": null,
        "created_at": "2026-09-30T15:54:32+08:00"
      }
    ],
    "total": 22,
    "limit": 1,
    "offset": 0
  }
}
```

### 3. 오류 응답 예시

**401 인증되지 않음:**

```json
{
  "code": "MISSING_TOKEN",
  "message": "missing authorization header",
  "request_id": "a02cfdf9-6be0-46be-8584-824caab34dc7",
  "can_retry": false,
  "requires_action": false
}
```

**404 리소스를 찾을 수 없습니다:**

```json
{
  "code": "CLIENT_NOT_FOUND",
  "message": "client not found",
  "request_id": "099949bd-4ded-468d-a6f8-f8e798af2273",
  "can_retry": false,
  "requires_action": false
}
```

### 4. 페이지 매김

모든 목록 엔드포인트는 `limit`(페이지 크기, 기본값 20) 및 `offset`(0 기반)를 사용하고 `{items, total, limit, offset}`를 반환합니다. 일부 레거시 엔드포인트는 기본 배열을 반환합니다(테이블별로 표시됨).

---

## 목차

- [관리자 인증 및 보안](#관리자-인증-및-보안)
- [관리자 계정](#관리자-계정)
- [고객관리](#고객관리)
- [계획 템플릿](#계획-템플릿)
- [계획 변형](#계획-변형)
- [트래픽 팩](#트래픽-팩)
- [쿠폰](#쿠폰)
- [노드 그룹](#노드-그룹)
- [노드 서버](#노드-서버)
- [노드 감사 로그](#노드-감사-로그)
- [노드(레거시 모델, 폐기됨)](#노드레거시-모델-폐기됨)
- [지역 개요](#지역-개요)
- [구독](#구독)
- [구독 형식](#구독-형식)
- [구독 엔드포인트](#구독-엔드포인트)
- [도메인 공유](#도메인-공유)
- [Cloudflare IP](#cloudflare-ip)
- [구독 구성(Mihomo/Clash)](#구독-구성mihomoclash)
- [충전소](#충전소)
- [주문 재충전](#주문-재충전)
- [충전카드](#충전카드)
- [청구 콜백 이벤트](#청구-콜백-이벤트)
- [콜백 재시도 청구](#콜백-재시도-청구)
- [고객 잔액](#고객-잔액)
- [선물](#선물)
- [추천 구성](#추천-구성)
- [추천 철회](#추천-철회)
- [IP 허용 목록(요약)](#ip-허용-목록요약)
- [고객 OAuth 바인딩](#고객-oauth-바인딩)
- [티켓(관리자)](#티켓관리자)
- [기사](#기사)
- [공유 계정](#공유-계정)
- [인증서](#인증서)
- [인증서 채널](#인증서-채널)
- [EAB 구성](#eab-구성)
- [ACME 계정](#acme-계정)
- [내장 CA](#내장-ca)
- [지역 규칙](#지역-규칙)
- [WAF 규칙 라이브러리](#waf-규칙-라이브러리)
- [로그 관리](#로그-관리)
- [백업](#백업)
- [시스템 설정](#시스템-설정)
- [보안 감사](#보안-감사)
- [측정항목](#측정항목)
- [포털 템플릿](#포털-템플릿)
- [에이전트 설치](#에이전트-설치)
- [청구 콜백 항목](#청구-콜백-항목)
- [인바운드(노드 포함)](#인바운드노드-포함)
- [공개 구성](#공개-구성)
- [클라이언트 인증 및 비즈니스(관리 엔진에서도 동일 설정)](#클라이언트-인증-및-비즈니스관리-엔진에서도-동일-설정)
- [구독 전달](#구독-전달)
- [런타임 구성 워크플로](#런타임-구성-워크플로)
- [서비스 액세스 정책(폐기됨)](#서비스-액세스-정책폐기됨)
- [트래픽 할당량 모니터링](#트래픽-할당량-모니터링)
- [건강검진](#건강검진)
- [포털 페이지](#포털-페이지)
- [ACME 과제](#acme-과제)

---

### 관리자 인증 및 보안

로그인, 로그아웃, 비밀번호 변경, 2FA(TOTP), Passkey(WebAuthn) 및 관리자 패널용 일회성 티켓.

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| POST | `/api/v1/auth/2fa/disable` | 로그인됨 | 비활성화 |
| POST | `/api/v1/auth/2fa/enable` | 로그인됨 | 활성화 |
| POST | `/api/v1/auth/2fa/recovery-codes` | 로그인됨 | 복구 코드 재생성 |
| POST | `/api/v1/auth/2fa/setup` | 로그인됨 | 설정 |
| GET | `/api/v1/auth/2fa/status` | 로그인됨 | 상태 |
| POST | `/api/v1/auth/change-password` | 로그인됨 | 비밀번호 변경 |
| POST | `/api/v1/auth/login` | 공개 | 로그인 |
| POST | `/api/v1/auth/login/2fa` | 공개 | 동적 코드 로그인(2FA) |
| GET | `/api/v1/auth/login/2fa-status` | 공개 | 계정에 2FA가 활성화되어 있는지 확인하세요. |
| POST | `/api/v1/auth/logout` | 로그인됨 | 로그아웃 |
| GET | `/api/v1/auth/me` | 로그인됨 | 현재 로그인한 사용자 |
| GET | `/api/v1/auth/passkey/credentials` | 로그인됨 | Passkey 자격 증명 목록 |
| DELETE | `/api/v1/auth/passkey/credentials/:id` | 로그인됨 | Passkey 자격 증명 삭제 |
| POST | `/api/v1/auth/passkey/login/begin` | 공개 | 로그인 시작 |
| POST | `/api/v1/auth/passkey/login/finish` | 공개 | 로그인 완료 |
| POST | `/api/v1/auth/passkey/register/begin` | 로그인됨 | 등록 시작 |
| POST | `/api/v1/auth/passkey/register/finish` | 로그인됨 | 등록 완료 |
| GET | `/api/v1/auth/passkey/rpids` | 로그인됨 | RP ID 허용 목록 |
| POST | `/api/v1/auth/passkey/rpids` | 로그인됨 | RP ID 허용 목록 업데이트 |
| GET | `/api/v1/auth/passkey/status` | 공개 | 상태 |
| POST | `/api/v1/auth/refresh` | 공개 | 새로고침 |
| POST | `/api/v1/auth/ticket` | 로그인됨 | 일회성 단기 티켓 발행 |

### 관리자 계정

관리자 패널 사용자 계정. 목록에는 관리자가 필요합니다. 생성/업데이트/삭제에는 최고 관리자가 필요합니다(권한 상승 방지).

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| GET | `/api/v1/users` | 관리자 | 목록 |
| POST | `/api/v1/users` | 최고 관리자 | 만들기 |
| DELETE | `/api/v1/users/:id` | 최고 관리자 | 삭제 |
| PUT | `/api/v1/users/:id` | 최고 관리자 | 업데이트 |

### 고객관리

구독자(포털 계정)에 대한 CRUD, 일시 중지/재개, 업그레이드/다운그레이드, 갱신 및 사용량 재설정.

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| GET | `/api/v1/clients` | 로그인됨 | 고객 나열 |
| POST | `/api/v1/clients` | 로그인됨 | 고객을 생성합니다. 본체: `{"email":"user@example.com","password":"plaintext-password","plan_id":1,"quota_bytes":10737418240,"expires_at":"2026-12-31T00:00:00Z","remark":""}`. `plan_id`가 필요합니다. `password`는 비어 있을 수 있습니다(고객이 나중에 포털에서 설정할 수 있음). |
| DELETE | `/api/v1/clients/:id` | 로그인됨 | 고객 삭제 |
| GET | `/api/v1/clients/:id` | 로그인됨 | 고객 확보 |
| PUT | `/api/v1/clients/:id` | 로그인됨 | 고객 업데이트 |
| POST | `/api/v1/clients/:id/downgrade` | 로그인됨 | 고객 다운그레이드 |
| GET | `/api/v1/clients/:id/orders` | 로그인됨 | 고객 주문 목록 |
| POST | `/api/v1/clients/:id/renew` | 로그인됨 | 고객 갱신 |
| POST | `/api/v1/clients/:id/reset-usage` | 로그인됨 | 사용량 재설정 |
| POST | `/api/v1/clients/:id/restore` | 로그인됨 | 고객 이력서 |
| POST | `/api/v1/clients/:id/suspend` | 로그인됨 | 고객 정지 |
| POST | `/api/v1/clients/:id/upgrade` | 로그인됨 | 고객 업그레이드 |

### 계획 템플릿

계획(변형, 청구 주기, 가격, 트래픽 할당량) 및 템플릿 수준 노드 승인.

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| GET | `/api/v1/plan-templates` | 로그인됨 | 템플릿 목록 |
| POST | `/api/v1/plan-templates` | 로그인됨 | 템플릿 만들기 |
| DELETE | `/api/v1/plan-templates/:id` | 로그인됨 | 템플릿 삭제 |
| GET | `/api/v1/plan-templates/:id` | 로그인됨 | 템플릿 세부정보 |
| PUT | `/api/v1/plan-templates/:id` | 로그인됨 | 템플릿 업데이트 |
| GET | `/api/v1/plan-templates/:id/accessible-nodes` | 로그인됨 | 사용 가능한 노드 가져오기 |
| GET | `/api/v1/plan-templates/:id/billing-node-options` | 로그인됨 | 청구 가능한 노드 옵션 가져오기 |
| PUT | `/api/v1/plan-templates/:id/node-groups` | 로그인됨 | 템플릿 노드 그룹 설정 |
| POST | `/api/v1/plan-templates/:id/sync-accounts` | 로그인됨 | 템플릿에 계정 동기화 |
| GET | `/api/v1/plan-templates/:id/variants` | 로그인됨 | 변형 목록 |
| POST | `/api/v1/plan-templates/:id/variants` | 로그인됨 | 변형 만들기 |
| POST | `/api/v1/plan-templates/sync-all-accounts` | 로그인됨 | 모든 계정 동기화 |

### 계획 변형

계획 변형을 업데이트하고 삭제합니다(생성은 계획 템플릿 아래에 중첩됩니다).

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| DELETE | `/api/v1/plan-variants/:id` | 로그인됨 | 변형 삭제 |
| PUT | `/api/v1/plan-variants/:id` | 로그인됨 | 변형 업데이트 |

### 트래픽 팩

별도로 구매할 수 있는 트래픽 팩입니다.

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| GET | `/api/v1/traffic-packs` | 로그인됨 | 트래픽 팩 목록 |
| POST | `/api/v1/traffic-packs` | 로그인됨 | 트래픽 팩 생성 |
| DELETE | `/api/v1/traffic-packs/:id` | 로그인됨 | 트래픽 팩 삭제 |
| PUT | `/api/v1/traffic-packs/:id` | 로그인됨 | 트래픽 팩 업데이트 |

### 쿠폰

쿠폰을 생성, 검증 및 교환하세요.

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| GET | `/api/v1/promotions` | 로그인됨 | 쿠폰 나열 |
| POST | `/api/v1/promotions` | 로그인됨 | 쿠폰 생성 |
| DELETE | `/api/v1/promotions/:id` | 로그인됨 | 쿠폰 삭제 |
| PUT | `/api/v1/promotions/:id` | 로그인됨 | 쿠폰 업데이트 |
| POST | `/api/v1/promotions/validate` | 로그인됨 | 쿠폰 확인 |

### 노드 그룹

계획에서 참조하는 그룹으로 노드를 구성합니다.

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| GET | `/api/v1/node-groups` | 로그인됨 | 노드 그룹 나열 |
| POST | `/api/v1/node-groups` | 로그인됨 | 노드 그룹 생성 |
| DELETE | `/api/v1/node-groups/:id` | 로그인됨 | 노드 그룹 삭제 |
| PUT | `/api/v1/node-groups/:id` | 로그인됨 | 노드 그룹 업데이트 |
| GET | `/api/v1/node-groups/:id/members` | 로그인됨 | 그룹 구성원 나열 |
| POST | `/api/v1/node-groups/:id/members` | 로그인됨 | 그룹 회원 추가 |
| DELETE | `/api/v1/node-groups/:id/members/:mid` | 로그인됨 | 그룹 회원 삭제 |

### 노드 서버

핵심 리소스: 노드 CRUD, 코어(xray/sing-box) 구성 전달, 인바운드/아웃바운드/엔드포인트 구성, 프로세스 제어, 인증서, WAF, 속도 제한, 포트 호핑, 연결된 프록시, 미러/기본 백업, 에이전트 설치/업그레이드, xray 런타임 제어. 기능 계열별로 그룹화된 많은 하위 경로.

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| GET | `/api/v1/node-servers` | 관리자 | 목록 |
| POST | `/api/v1/node-servers` | 관리자 | 만들기 |
| DELETE | `/api/v1/node-servers/:id` | 관리자 | 삭제 |
| GET | `/api/v1/node-servers/:id` | 관리자 | 세부정보 |
| PUT | `/api/v1/node-servers/:id` | 관리자 | 업데이트 |
| GET | `/api/v1/node-servers/:id/binaries` | 관리자 | 설치된 코어 목록 |
| POST | `/api/v1/node-servers/:id/binaries/ech-generate` | 관리자 | ECH 키 쌍 생성 |
| POST | `/api/v1/node-servers/:id/binaries/install` | 관리자 | 코어 설치/업그레이드 |
| GET | `/api/v1/node-servers/:id/binaries/interfaces` | 관리자 | 네트워크 인터페이스 목록 |
| POST | `/api/v1/node-servers/:id/binaries/mldsa65-key` | 관리자 | MLDSA65 키 생성 |
| POST | `/api/v1/node-servers/:id/binaries/reality-key` | 관리자 | REALITY 키 쌍 생성 |
| POST | `/api/v1/node-servers/:id/binaries/reality-public-key` | 관리자 | REALITY 개인 키에서 공개 키 파생 |
| GET | `/api/v1/node-servers/:id/binaries/releases` | 관리자 | 사용 가능한 버전 |
| POST | `/api/v1/node-servers/:id/binaries/vlessenc` | 관리자 | VLESS 암호화 매개변수 생성 |
| POST | `/api/v1/node-servers/:id/binaries/wg-key` | 관리자 | WireGuard 키 생성 |
| POST | `/api/v1/node-servers/:id/binaries/x25519-key` | 관리자 | X25519 키 쌍 생성 |
| GET | `/api/v1/node-servers/:id/bootstrap-cmd` | 관리자 | 에이전트 부트스트랩 명령 가져오기 |
| POST | `/api/v1/node-servers/:id/bootstrap-token` | 관리자 | 부트스트랩 토큰 생성 |
| POST | `/api/v1/node-servers/:id/caddy/batch-update-log-enabled` | 관리자 | 대량 업데이트 로그 스위치 |
| POST | `/api/v1/node-servers/:id/caddy/batch-update-ports` | 관리자 | 대량 업데이트 포트 |
| POST | `/api/v1/node-servers/:id/caddy/generate-config` | 관리자 | 구성 생성 |
| DELETE | `/api/v1/node-servers/:id/caddy/l4-rules` | 관리자 | 모든 L4 규칙 삭제 |
| POST | `/api/v1/node-servers/:id/caddy/save-and-apply` | 관리자 | 저장 및 적용 |
| GET | `/api/v1/node-servers/:id/caddy/sites` | 관리자 | 사이트 목록 |
| POST | `/api/v1/node-servers/:id/caddy/sites` | 관리자 | 사이트 만들기 |
| DELETE | `/api/v1/node-servers/:id/caddy/sites/:siteId` | 관리자 | 사이트 삭제 |
| PUT | `/api/v1/node-servers/:id/caddy/sites/:siteId` | 관리자 | 사이트 업데이트 |
| DELETE | `/api/v1/node-servers/:id/caddy/sni-rules` | 관리자 | 모든 SNI 규칙 삭제 |
| POST | `/api/v1/node-servers/:id/caddy/sni-rules/toggle` | 관리자 | SNI 규칙 전환 |
| GET | `/api/v1/node-servers/:id/caddy/sni-settings` | 관리자 | SNI 설정 |
| POST | `/api/v1/node-servers/:id/caddy/sni-settings` | 관리자 | SNI 설정 저장 |
| GET | `/api/v1/node-servers/:id/certificates` | 관리자 | 노드 인증서 목록 |
| DELETE | `/api/v1/node-servers/:id/certificates/:cid` | 관리자 | 인증서 바인딩 해제 |
| POST | `/api/v1/node-servers/:id/certificates/:cid` | 관리자 | 인증서 바인딩 |
| GET | `/api/v1/node-servers/:id/chain-config` | 관리자 | 연결된 구성 생성 |
| GET | `/api/v1/node-servers/:id/chain-health` | 관리자 | 체인 상태 |
| POST | `/api/v1/node-servers/:id/control` | 최고 관리자 | 제어 노드 코어. 본문: `{"core_type":"xray","action":"start"}`(작업 = 시작/중지/다시 시작). `core_type`는 `xray`/`singbox`일 수 있습니다. |
| GET | `/api/v1/node-servers/:id/degradation-status` | 관리자 | 저하된 상태 |
| POST | `/api/v1/node-servers/:id/distribute-cert` | 관리자 | 인증서 배포 |
| GET | `/api/v1/node-servers/:id/endpoint-configs` | 관리자 | 목록 |
| POST | `/api/v1/node-servers/:id/endpoint-configs` | 관리자 | 만들기 |
| DELETE | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | 관리자 | 삭제 |
| GET | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | 관리자 | 세부정보 |
| PUT | `/api/v1/node-servers/:id/endpoint-configs/:cfgId` | 관리자 | 업데이트 |
| POST | `/api/v1/node-servers/:id/failover` | 관리자 | 이 노드에 대한 장애 조치 트리거 |
| GET | `/api/v1/node-servers/:id/failover-status` | 관리자 | 장애 조치 상태 |
| POST | `/api/v1/node-servers/:id/force-reconnect` | 관리자 | 강제로 다시 연결 |
| GET | `/api/v1/node-servers/:id/free-port` | 관리자 | 무료 포트 요청 |
| POST | `/api/v1/node-servers/:id/geo-rules/push` | 관리자 | 이 노드로 푸시 |
| GET | `/api/v1/node-servers/:id/inbound-configs` | 관리자 | 목록 |
| POST | `/api/v1/node-servers/:id/inbound-configs` | 관리자 | 만들기 |
| DELETE | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | 관리자 | 삭제 |
| GET | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | 관리자 | 세부정보 |
| PUT | `/api/v1/node-servers/:id/inbound-configs/:cfgId` | 관리자 | 업데이트 |
| GET | `/api/v1/node-servers/:id/inbound-configs/:cfgId/share-links` | 관리자 | 링크 및 스니펫 공유 |
| POST | `/api/v1/node-servers/:id/inbound-configs/:cfgId/users/:email/pause` | 관리자 | 이 사용자를 일시중지합니다. |
| POST | `/api/v1/node-servers/:id/inbound-configs/:cfgId/users/:email/resume` | 관리자 | 이 사용자 복원 |
| POST | `/api/v1/node-servers/:id/inbound-configs/reset-all-traffic` | 관리자 | 모든 인바운드 트래픽 재설정 |
| GET | `/api/v1/node-servers/:id/install-script` | 관리자 | 에이전트 설치 스크립트 가져오기 |
| GET | `/api/v1/node-servers/:id/metrics-history` | 관리자 | 측정항목 기록 가져오기 |
| GET | `/api/v1/node-servers/:id/mirror-status` | 관리자 | 미러 상태 |
| POST | `/api/v1/node-servers/:id/nginx/batch-update-ports` | 관리자 | 대량 업데이트 포트 |
| POST | `/api/v1/node-servers/:id/nginx/generate-config` | 관리자 | 구성 생성 |
| DELETE | `/api/v1/node-servers/:id/nginx/l4-rules` | 관리자 | 모든 L4 규칙 삭제 |
| POST | `/api/v1/node-servers/:id/nginx/save-and-apply` | 관리자 | 저장 및 적용 |
| GET | `/api/v1/node-servers/:id/nginx/settings` | 관리자 | 설정 읽기 |
| POST | `/api/v1/node-servers/:id/nginx/settings` | 관리자 | 설정 저장 |
| GET | `/api/v1/node-servers/:id/nginx/sites` | 관리자 | 사이트 목록 |
| POST | `/api/v1/node-servers/:id/nginx/sites` | 관리자 | 사이트 만들기 |
| DELETE | `/api/v1/node-servers/:id/nginx/sites/:siteId` | 관리자 | 사이트 삭제 |
| PUT | `/api/v1/node-servers/:id/nginx/sites/:siteId` | 관리자 | 사이트 업데이트 |
| DELETE | `/api/v1/node-servers/:id/nginx/sni-rules` | 관리자 | 모든 SNI 규칙 삭제 |
| POST | `/api/v1/node-servers/:id/nginx/sni-rules/toggle` | 관리자 | SNI 규칙 전환 |
| PUT | `/api/v1/node-servers/:id/node-type` | 관리자 | 노드 유형 업데이트 |
| GET | `/api/v1/node-servers/:id/online-users` | 관리자 | 온라인 사용자 |
| GET | `/api/v1/node-servers/:id/outbound-configs` | 관리자 | 목록 |
| POST | `/api/v1/node-servers/:id/outbound-configs` | 관리자 | 만들기 |
| DELETE | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | 관리자 | 삭제 |
| GET | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | 관리자 | 세부정보 |
| PUT | `/api/v1/node-servers/:id/outbound-configs/:cfgId` | 관리자 | 업데이트 |
| PUT | `/api/v1/node-servers/:id/outbound-configs/reorder` | 관리자 | 정렬 |
| GET | `/api/v1/node-servers/:id/port-hop/rules` | 관리자 | 포트 호핑 규칙 미리보기 |
| GET | `/api/v1/node-servers/:id/protocols` | 관리자 | 프로토콜 목록 |
| GET | `/api/v1/node-servers/:id/protocols/:coreType` | 관리자 | 프로토콜 구성 |
| PUT | `/api/v1/node-servers/:id/protocols/:coreType` | 관리자 | 프로토콜 구성 저장 |
| GET | `/api/v1/node-servers/:id/protocols/:coreType/config-file` | 관리자 | 핵심 구성 파일 읽기 |
| GET | `/api/v1/node-servers/:id/protocols/:coreType/section/:section` | 관리자 | 프로토콜 섹션 읽기 |
| PUT | `/api/v1/node-servers/:id/protocols/:coreType/section/:section` | 관리자 | 프로토콜 섹션 저장 |
| POST | `/api/v1/node-servers/:id/protocols/:coreType/validate` | 관리자 | 구성 검증(실제 코어 테스트 실행) |
| GET | `/api/v1/node-servers/:id/proxy-status` | 관리자 | 원격 프록시 상태 |
| POST | `/api/v1/node-servers/:id/proxy/batch-update-ports` | 관리자 | 대량 업데이트 포트 |
| GET | `/api/v1/node-servers/:id/proxy/config` | 관리자 | 구성 읽기 |
| PUT | `/api/v1/node-servers/:id/proxy/config` | 관리자 | 구성 업데이트 |
| POST | `/api/v1/node-servers/:id/proxy/control` | 관리자 | 연결된 프록시 등록 제어 |
| POST | `/api/v1/node-servers/:id/proxy/download` | 관리자 | 프록시 코어 버전 다운로드 |
| POST | `/api/v1/node-servers/:id/proxy/generate-config` | 관리자 | 구성 생성 |
| POST | `/api/v1/node-servers/:id/proxy/guardian` | 관리자 | 감독자 구성 업데이트 |
| POST | `/api/v1/node-servers/:id/proxy/install` | 관리자 | 프록시 설치 |
| DELETE | `/api/v1/node-servers/:id/proxy/l4-rules` | 관리자 | 모든 L4 규칙 삭제 |
| GET | `/api/v1/node-servers/:id/proxy/l4-settings` | 관리자 | L4 설정 |
| POST | `/api/v1/node-servers/:id/proxy/l4-settings` | 관리자 | L4 설정 저장 |
| GET | `/api/v1/node-servers/:id/proxy/log-settings` | 관리자 | 로그 설정 |
| POST | `/api/v1/node-servers/:id/proxy/log-settings` | 관리자 | 로그 설정 저장 |
| GET | `/api/v1/node-servers/:id/proxy/nginx-settings` | 관리자 | nginx 설정 |
| POST | `/api/v1/node-servers/:id/proxy/nginx-settings` | 관리자 | nginx 설정 저장 |
| GET | `/api/v1/node-servers/:id/proxy/releases` | 관리자 | 사용 가능한 버전 |
| POST | `/api/v1/node-servers/:id/proxy/reload` | 관리자 | 구성 다시 로드 |
| POST | `/api/v1/node-servers/:id/proxy/restart` | 관리자 | 다시 시작 |
| GET | `/api/v1/node-servers/:id/proxy/sites` | 관리자 | 사이트 목록 |
| POST | `/api/v1/node-servers/:id/proxy/sites` | 관리자 | 사이트 만들기 |
| DELETE | `/api/v1/node-servers/:id/proxy/sites/:siteId` | 관리자 | 사이트 삭제 |
| PUT | `/api/v1/node-servers/:id/proxy/sites/:siteId` | 관리자 | 사이트 업데이트 |
| DELETE | `/api/v1/node-servers/:id/proxy/sni-rules` | 관리자 | 모든 SNI 규칙 삭제 |
| POST | `/api/v1/node-servers/:id/proxy/sni-rules/toggle` | 관리자 | SNI 규칙 전환 |
| GET | `/api/v1/node-servers/:id/proxy/sni-settings` | 관리자 | SNI 설정 |
| POST | `/api/v1/node-servers/:id/proxy/sni-settings` | 관리자 | SNI 설정 저장 |
| POST | `/api/v1/node-servers/:id/proxy/start` | 관리자 | 시작 |
| GET | `/api/v1/node-servers/:id/proxy/status` | 관리자 | 프록시 프로세스 상태 |
| POST | `/api/v1/node-servers/:id/proxy/stop` | 관리자 | 중지 |
| POST | `/api/v1/node-servers/:id/proxy/sync` | 관리자 | 동기화 프록시 구성 |
| GET | `/api/v1/node-servers/:id/rate-limit` | 관리자 | 구성 읽기 |
| PUT | `/api/v1/node-servers/:id/rate-limit` | 관리자 | 구성 업데이트 |
| GET | `/api/v1/node-servers/:id/rate-limit/status` | 관리자 | 상태 |
| GET | `/api/v1/node-servers/:id/recover-mode` | 관리자 | 노드 복구 모드 읽기 |
| PUT | `/api/v1/node-servers/:id/recover-mode` | 관리자 | 노드 복구 모드 설정 |
| POST | `/api/v1/node-servers/:id/revoke-cert` | 관리자 | 에이전트 인증서 취소 |
| GET | `/api/v1/node-servers/:id/services` | 관리자 | 목록 |
| POST | `/api/v1/node-servers/:id/services` | 관리자 | 만들기 |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId` | 관리자 | 삭제 |
| GET | `/api/v1/node-servers/:id/services/:serviceId` | 관리자 | 세부정보 |
| PUT | `/api/v1/node-servers/:id/services/:serviceId` | 관리자 | 업데이트 |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId/crash-logs` | 관리자 | 충돌 로그 지우기 |
| GET | `/api/v1/node-servers/:id/services/:serviceId/crash-logs` | 관리자 | 충돌 로그 목록 |
| GET | `/api/v1/node-servers/:id/services/:serviceId/crash-logs/:logId` | 관리자 | 충돌 로그 세부정보 |
| GET | `/api/v1/node-servers/:id/services/:serviceId/endpoints` | 관리자 | 엔드포인트 목록 |
| POST | `/api/v1/node-servers/:id/services/:serviceId/endpoints` | 관리자 | 엔드포인트 생성 |
| DELETE | `/api/v1/node-servers/:id/services/:serviceId/endpoints/:endpointId` | 관리자 | 엔드포인트 삭제 |
| PUT | `/api/v1/node-servers/:id/services/:serviceId/endpoints/:endpointId` | 관리자 | 엔드포인트 업데이트 |
| PUT | `/api/v1/node-servers/:id/services/:serviceId/guardian` | 관리자 | 감독자 구성 업데이트 |
| GET | `/api/v1/node-servers/:id/services/:serviceId/process-status` | 관리자 | 처리 상태 가져오기 |
| POST | `/api/v1/node-servers/:id/services/:serviceId/restart` | 관리자 | 다시 시작 |
| POST | `/api/v1/node-servers/:id/services/:serviceId/start` | 관리자 | 시작 |
| POST | `/api/v1/node-servers/:id/services/:serviceId/stop` | 관리자 | 중지 |
| GET | `/api/v1/node-servers/:id/services/process-status` | 관리자 | 모든 프로세스 상태 |
| GET | `/api/v1/node-servers/:id/share-domains` | 관리자 | 이 노드에 대한 도메인 공유 |
| POST | `/api/v1/node-servers/:id/share-domains` | 관리자 | 만들기 |
| DELETE | `/api/v1/node-servers/:id/share-domains/:domainId` | 관리자 | 삭제 |
| PUT | `/api/v1/node-servers/:id/share-domains/:domainId` | 관리자 | 업데이트 |
| DELETE | `/api/v1/node-servers/:id/share-domains/all` | 관리자 | 모두 지우기 |
| POST | `/api/v1/node-servers/:id/singbox/control/firewall/apply` | 관리자 | 포트 호핑 방화벽 규칙 적용 |
| POST | `/api/v1/node-servers/:id/singbox/control/firewall/remove` | 관리자 | 포트 호핑 방화벽 규칙 제거 |
| POST | `/api/v1/node-servers/:id/speed-test` | 관리자 | 노드 속도 테스트 |
| GET | `/api/v1/node-servers/:id/status` | 관리자 | 노드 상태 |
| POST | `/api/v1/node-servers/:id/sync` | 관리자 | 동기화 노드 구성(생성 및 푸시) |
| POST | `/api/v1/node-servers/:id/sync-data` | 최고 관리자 | 패널 데이터를 노드에 푸시(사용자/구독/계획의 전체 동기화) |
| POST | `/api/v1/node-servers/:id/sync-mirror` | 관리자 | 동기화 미러 노드 |
| GET | `/api/v1/node-servers/:id/tags` | 관리자 | 태그 목록 |
| POST | `/api/v1/node-servers/:id/tags` | 관리자 | 태그 추가 |
| DELETE | `/api/v1/node-servers/:id/tags/:tagKey` | 관리자 | 태그 삭제 |
| GET | `/api/v1/node-servers/:id/traffic-history` | 관리자 | 교통 이력 |
| POST | `/api/v1/node-servers/:id/traffic-resume` | 관리자 | 초과분으로 인해 일시 중단된 트래픽 복원 |
| GET | `/api/v1/node-servers/:id/traffic-usage` | 관리자 | 교통 사용량 |
| POST | `/api/v1/node-servers/:id/update-agent` | 관리자 | 업그레이드 에이전트 |
| GET | `/api/v1/node-servers/:id/used-server-names` | 관리자 | 사용중인 REALITY SNI 목록 |
| GET | `/api/v1/node-servers/:id/waf/config` | 관리자 | 구성 읽기 |
| PUT | `/api/v1/node-servers/:id/waf/config` | 관리자 | 구성 저장 |
| POST | `/api/v1/node-servers/:id/waf/disable-domain` | 관리자 | 도메인 보호 비활성화 |
| GET | `/api/v1/node-servers/:id/waf/domain-config` | 관리자 | 도메인 구성 목록 |
| PUT | `/api/v1/node-servers/:id/waf/domain-config` | 관리자 | 대량 저장 도메인 구성 |
| DELETE | `/api/v1/node-servers/:id/waf/domain-config/:domain` | 관리자 | 도메인 구성 삭제 |
| GET | `/api/v1/node-servers/:id/waf/domain-config/:domain` | 관리자 | 도메인 구성 세부정보 |
| PUT | `/api/v1/node-servers/:id/waf/domain-config/:domain` | 관리자 | 도메인 구성 저장 |
| GET | `/api/v1/node-servers/:id/waf/domain-rules` | 관리자 | 도메인 규칙 디렉터리 |
| GET | `/api/v1/node-servers/:id/waf/domain-rules/:domain/:name` | 관리자 | 도메인 규칙 파일 읽기 |
| PUT | `/api/v1/node-servers/:id/waf/domain-rules/:domain/:name` | 관리자 | 도메인 규칙 파일 저장 |
| GET | `/api/v1/node-servers/:id/waf/domain-status` | 관리자 | 도메인 보호 상태 |
| POST | `/api/v1/node-servers/:id/waf/enable-domain` | 관리자 | 도메인 보호 활성화 |
| POST | `/api/v1/node-servers/:id/waf/push` | 관리자 | 이 노드로 푸시 |
| GET | `/api/v1/node-servers/:id/waf/rules` | 관리자 | 노드 WAF 규칙 목록 |
| DELETE | `/api/v1/node-servers/:id/waf/rules/:name` | 관리자 | 노드 WAF 규칙 삭제 |
| GET | `/api/v1/node-servers/:id/waf/rules/:name` | 관리자 | 노드 WAF 규칙 세부정보 |
| PUT | `/api/v1/node-servers/:id/waf/rules/:name` | 관리자 | 노드 WAF 규칙 저장 |
| GET | `/api/v1/node-servers/:id/waf/status` | 관리자 | 노드 WAF 상태 |
| PUT | `/api/v1/node-servers/:id/waf/status` | 관리자 | 토글 노드 WAF |
| POST | `/api/v1/node-servers/:id/waf/sync` | 관리자 | 이 노드에 동기화 |
| GET | `/api/v1/node-servers/:id/warp` | 관리자 | 목록 |
| DELETE | `/api/v1/node-servers/:id/warp/:warpId` | 관리자 | 삭제 |
| GET | `/api/v1/node-servers/:id/warp/config` | 관리자 | 구성 읽기 |
| POST | `/api/v1/node-servers/:id/warp/generate-config` | 관리자 | 구성 생성 |
| POST | `/api/v1/node-servers/:id/warp/register` | 관리자 | 등록 |
| GET | `/api/v1/node-servers/:id/warp/status` | 관리자 | 상태 |
| GET | `/api/v1/node-servers/:id/xray/control/balancer/:tag` | 관리자 | 밸런서 정보 얻기 |
| POST | `/api/v1/node-servers/:id/xray/control/balancer/:tag` | 관리자 | 밸런서 타겟 설정 |
| POST | `/api/v1/node-servers/:id/xray/control/firewall/apply` | 관리자 | 포트 호핑 방화벽 규칙 적용 |
| POST | `/api/v1/node-servers/:id/xray/control/firewall/remove` | 관리자 | 포트 호핑 방화벽 규칙 제거 |
| POST | `/api/v1/node-servers/:id/xray/control/inbound` | 관리자 | 인바운드 추가 |
| DELETE | `/api/v1/node-servers/:id/xray/control/inbound/:tag` | 관리자 | 인바운드 삭제 |
| POST | `/api/v1/node-servers/:id/xray/control/inbound/:tag/user` | 관리자 | 사용자 추가 |
| DELETE | `/api/v1/node-servers/:id/xray/control/inbound/:tag/user/:email` | 관리자 | 사용자 삭제 |
| POST | `/api/v1/node-servers/:id/xray/control/outbound` | 관리자 | 아웃바운드 추가 |
| DELETE | `/api/v1/node-servers/:id/xray/control/outbound/:tag` | 관리자 | 아웃바운드 삭제 |
| POST | `/api/v1/node-servers/:id/xray/control/routing` | 관리자 | 경로 요청 |
| POST | `/api/v1/node-servers/:id/xray/control/routing/rule` | 관리자 | 라우팅 규칙 추가 |
| DELETE | `/api/v1/node-servers/:id/xray/control/routing/rule/:tag` | 관리자 | 라우팅 규칙 삭제 |
| GET | `/api/v1/node-servers/:id/xray/control/routing/rules` | 관리자 | 라우팅 규칙 나열 |
| POST | `/api/v1/node-servers/:id/xray/control/sync` | 관리자 | xray 구성 동기화를 수동으로 트리거 |
| GET | `/api/v1/node-servers/:id/xray/stats` | 관리자 | 통계 얻기 |
| POST | `/api/v1/node-servers/:id/xray/stats/reset/:email` | 관리자 | 사용자 통계 재설정 |
| GET | `/api/v1/node-servers/:id/xray/stats/users` | 관리자 | 사용자 통계 가져오기 |
| GET | `/api/v1/node-servers/billable` | 관리자 | 청구 가능한 노드 목록 |
| GET | `/api/v1/node-servers/ca-cert` | 관리자 | CA 인증서 |
| GET | `/api/v1/node-servers/ca-status` | 관리자 | CA 상태 |
| GET | `/api/v1/node-servers/dashboard` | 관리자 | 대시보드 상태 |
| POST | `/api/v1/node-servers/distribute-cert` | 관리자 | 모든 노드에 인증서 배포 |
| POST | `/api/v1/node-servers/failover-check` | 관리자 | 장애 조치 확인 트리거 |
| GET | `/api/v1/node-servers/metrics/aggregated` | 관리자 | 집계된 측정항목 |
| POST | `/api/v1/node-servers/migrate-cert` | 관리자 | 인증서 마이그레이션 |
| GET | `/api/v1/node-servers/online-summary` | 관리자 | 온라인 요약 |
| GET | `/api/v1/node-servers/online-users` | 관리자 | 모든 온라인 사용자 |
| GET | `/api/v1/node-servers/pending-agent-updates` | 관리자 | 업그레이드 보류 중인 에이전트 |
| POST | `/api/v1/node-servers/recommend-region` | 관리자 | 추천 지역 |
| POST | `/api/v1/node-servers/select-nodes` | 관리자 | 구독할 노드 선택 |
| POST | `/api/v1/node-servers/speed-test-all` | 관리자 | 모든 노드의 속도 테스트 |
| GET | `/api/v1/node-servers/speed-test-results` | 관리자 | 속도 테스트 결과 |
| GET | `/api/v1/node-servers/traffic/stats` | 관리자 | 교통통계 |
| POST | `/api/v1/node-servers/update-all-agents` | 관리자 | 모든 에이전트 업그레이드 |
| DELETE | `/api/v1/node-servers/users/:client_id` | 관리자 | 고객 동기화 삭제 |
| GET | `/api/v1/node-servers/users/retry-queue` | 관리자 | 재시도 대기열 상태 |
| POST | `/api/v1/node-servers/users/retry-queue/process` | 관리자 | 프로세스 재시도 큐 |
| POST | `/api/v1/node-servers/users/sync` | 관리자 | 단일 고객 동기화 |
| POST | `/api/v1/node-servers/validate-chain-config` | 관리자 | 연결된 구성 검증 |

### 노드 감사 로그

노드 측 작업 감사 및 내보내기.

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| GET | `/api/v1/node-audit-logs` | 로그인됨 | 노드 감사 로그 |
| GET | `/api/v1/node-audit-logs/export` | 로그인됨 | 노드 감사 로그 내보내기 |

### 노드(레거시 모델, 폐기됨)

> `/api/v1/nodes` CRUD는 2026년 10월 20일에 폐기되었습니다. 노드는 `/api/v1/node-servers`를 통해 균일하게 관리됩니다.
> 온라인 사용자 쿼리는 `/api/v1/node-servers/{online-users,online-summary,:id/online-users}`에 남아 있습니다.

### 지역 개요

지역별로 집계된 노드 보기입니다.

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| GET | `/api/v1/regions/:region/servers` | 로그인됨 | 이 지역의 노드 목록 |
| GET | `/api/v1/regions/overview` | 로그인됨 | 지역 개요 |

### 구독

구독 기록 관리: 취소, 토큰 순환, 액세스 로그, 사용량 재설정.

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| GET | `/api/v1/subscriptions` | 로그인됨 | 목록 |
| POST | `/api/v1/subscriptions` | 로그인됨 | 만들기 |
| DELETE | `/api/v1/subscriptions/:id` | 로그인됨 | 삭제 |
| GET | `/api/v1/subscriptions/:id` | 로그인됨 | 세부정보 |
| PUT | `/api/v1/subscriptions/:id` | 로그인됨 | 업데이트 |
| GET | `/api/v1/subscriptions/:id/access-logs` | 로그인됨 | 액세스 로그 |
| POST | `/api/v1/subscriptions/:id/enable` | 로그인됨 | 활성화 |
| POST | `/api/v1/subscriptions/:id/reset-usage` | 로그인됨 | 사용량 재설정 |
| POST | `/api/v1/subscriptions/:id/restore` | 로그인됨 | 이력서 |
| POST | `/api/v1/subscriptions/:id/revoke` | 로그인됨 | 취소 |
| POST | `/api/v1/subscriptions/:id/rotate` | 로그인됨 | 회전 |
| GET | `/api/v1/subscriptions/:id/rotations` | 로그인됨 | 순환 기록 |

### 구독 형식

> 관리자 측 `/api/v1/subscription-formats` CRUD는 2026년 10월 20일에 폐기되었습니다. 실제 진입점은 다음과 같습니다.
> `/api/v1/mihomo/configs` (구독 형식 센터). `subscription_formats` 테이블은 남아 있으며,
> 공개 `/api/v1/public/client-configs` 엔드포인트의 시드 데이터로만 사용됩니다.

### 구독 엔드포인트

전용 구독 도메인(패널의 `/sub` 경로로 역방향 프록시됨).

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| GET | `/api/v1/subscription-endpoints` | 로그인됨 | 목록 |
| POST | `/api/v1/subscription-endpoints` | 로그인됨 | 만들기 |
| DELETE | `/api/v1/subscription-endpoints/:id` | 로그인됨 | 삭제 |
| GET | `/api/v1/subscription-endpoints/:id` | 로그인됨 | 세부정보 |
| PUT | `/api/v1/subscription-endpoints/:id` | 로그인됨 | 업데이트 |

### 도메인 공유

클라이언트 링크에 사용되는 액세스 도메인/IP 풀입니다.

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| GET | `/api/v1/share-domains` | 로그인됨 | 목록 |
| POST | `/api/v1/share-domains` | 로그인됨 | 만들기 |
| DELETE | `/api/v1/share-domains/:id` | 로그인됨 | 삭제 |
| PUT | `/api/v1/share-domains/:id` | 로그인됨 | 업데이트 |
| DELETE | `/api/v1/share-domains/all` | 로그인됨 | 모두 지우기 |
| POST | `/api/v1/share-domains/seed` | 로그인됨 | 기본 데이터 초기화 |

### Cloudflare IP

Cloudflare CDN IP 풀 및 소스 관리.

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| DELETE | `/api/v1/cfip/entries` | 로그인됨 | 항목 지우기 |
| GET | `/api/v1/cfip/entries` | 로그인됨 | 참가목록 |
| POST | `/api/v1/cfip/entries` | 로그인됨 | 항목 추가 |
| DELETE | `/api/v1/cfip/entries/:id` | 로그인됨 | 항목 삭제 |
| POST | `/api/v1/cfip/fetch-all` | 로그인됨 | 모든 소스 가져오기 |
| POST | `/api/v1/cfip/seed` | 로그인됨 | 기본 데이터 초기화 |
| GET | `/api/v1/cfip/sources` | 로그인됨 | 데이터 소스 목록 |
| POST | `/api/v1/cfip/sources` | 로그인됨 | 데이터 소스 만들기 |
| DELETE | `/api/v1/cfip/sources/:id` | 로그인됨 | 데이터 소스 삭제 |
| PUT | `/api/v1/cfip/sources/:id` | 로그인됨 | 데이터 소스 업데이트 |
| POST | `/api/v1/cfip/sources/:id/fetch` | 로그인됨 | 이 소스 가져오기 |

### 구독 구성(Mihomo/Clash)

구독 템플릿 구성: 그룹, 규칙, 규칙 제공자, YAML 미리보기.

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| GET | `/api/v1/mihomo/configs` | 로그인됨 | 목록 |
| POST | `/api/v1/mihomo/configs` | 로그인됨 | 만들기 |
| DELETE | `/api/v1/mihomo/configs/:id` | 로그인됨 | 삭제 |
| GET | `/api/v1/mihomo/configs/:id` | 로그인됨 | 세부정보 |
| PUT | `/api/v1/mihomo/configs/:id` | 로그인됨 | 업데이트 |
| GET | `/api/v1/mihomo/configs/:id/proxy-groups` | 로그인됨 | 그룹 목록 |
| POST | `/api/v1/mihomo/configs/:id/proxy-groups` | 로그인됨 | 그룹 만들기 |
| GET | `/api/v1/mihomo/configs/:id/rule-providers` | 로그인됨 | 규칙 공급자 목록 |
| POST | `/api/v1/mihomo/configs/:id/rule-providers` | 로그인됨 | 규칙 공급자 생성 |
| GET | `/api/v1/mihomo/configs/:id/rules` | 로그인됨 | 규칙 목록 |
| POST | `/api/v1/mihomo/configs/:id/rules` | 로그인됨 | 규칙 만들기 |
| GET | `/api/v1/mihomo/configs/:id/yaml` | 로그인됨 | 미리보기 생성 YAML |
| DELETE | `/api/v1/mihomo/proxy-groups/:id` | 로그인됨 | 그룹 삭제 |
| DELETE | `/api/v1/mihomo/rule-providers/:id` | 로그인됨 | 규칙 공급자 삭제 |
| DELETE | `/api/v1/mihomo/rules/:id` | 로그인됨 | 규칙 삭제 |

### 충전소

제3자 결제 사이트 통합: 서명 키, 확인 파일, 도메인 확인, 통합 문서.

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| GET | `/api/v1/charge-sites` | 로그인됨 | 사이트 목록 |
| POST | `/api/v1/charge-sites` | 로그인됨 | 사이트 만들기 |
| DELETE | `/api/v1/charge-sites/:id` | 로그인됨 | 사이트 삭제 |
| GET | `/api/v1/charge-sites/:id` | 로그인됨 | 사이트 세부정보 |
| PUT | `/api/v1/charge-sites/:id` | 로그인됨 | 사이트 업데이트 |
| POST | `/api/v1/charge-sites/:id/clear-prev-secret` | 로그인됨 | 이전 키 지우기 |
| POST | `/api/v1/charge-sites/:id/generate-verify` | 로그인됨 | 확인 파일 생성 |
| POST | `/api/v1/charge-sites/:id/generate-verify-file` | 로그인됨 | 확인 파일 생성 |
| GET | `/api/v1/charge-sites/:id/integration-doc` | 로그인됨 | 통합 문서 받기 |
| POST | `/api/v1/charge-sites/:id/rotate-secret` | 로그인됨 | 서명 키 순환 |
| POST | `/api/v1/charge-sites/:id/verify-dns` | 로그인됨 | 도메인 DNS 확인 |
| POST | `/api/v1/charge-sites/:id/verify-domain` | 로그인됨 | 도메인 HTTP 챌린지 확인 |
| POST | `/api/v1/charge-sites/:id/verify-domain-dns` | 로그인됨 | 도메인 DNS 확인 |
| POST | `/api/v1/charge-sites/:id/verify-domain-http` | 로그인됨 | 도메인 HTTP 챌린지 확인 |

### 주문 재충전

충전 주문 문의, 환불, CSV 내보내기.

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| GET | `/api/v1/charge-orders` | 로그인됨 | 주문 목록 |
| GET | `/api/v1/charge-orders/:id` | 로그인됨 | 주문 세부정보 |
| POST | `/api/v1/charge-orders/:id/refund` | 로그인됨 | 환불 |
| GET | `/api/v1/charge-orders/export-csv` | 로그인됨 | 재충전 주문 내보내기 CSV |

### 충전카드

대량 카드 생성, 사용 기록, CSV 내보내기.

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| GET | `/api/v1/charge-cards` | 로그인됨 | 카드 목록 |
| POST | `/api/v1/charge-cards` | 로그인됨 | 카드 만들기 |
| DELETE | `/api/v1/charge-cards/:id` | 로그인됨 | 카드 삭제 |
| GET | `/api/v1/charge-cards/:id/usages` | 로그인됨 | 카드사용기록 |
| POST | `/api/v1/charge-cards/batch` | 로그인됨 | 대량 생성 카드 |
| GET | `/api/v1/charge-cards/export-csv` | 로그인됨 | 수출 카드 CSV |

### 청구 콜백 이벤트

콜백 이벤트 감사.

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| GET | `/api/v1/charge-callback-events` | 로그인됨 | 콜백 이벤트 목록 |

### 콜백 재시도 청구

실패한 콜백을 수동으로 재시도합니다.

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| GET | `/api/v1/charge-callback-retries` | 로그인됨 | 실패한 콜백 목록 |
| POST | `/api/v1/charge-callback-retries/:id/retry` | 관리자 | 콜백 재시도 |

### 고객 잔액

잔액 및 거래에 대한 관리 보기입니다.

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| GET | `/api/v1/client-balances` | 로그인됨 | 고객 잔액 목록 |
| GET | `/api/v1/client-balances/transactions` | 로그인됨 | 잔액 거래(관리자) |
| GET | `/api/v1/client-balances/transactions/export-csv` | 로그인됨 | 잔액 거래 내보내기 CSV |

### 선물

청구 가능한 선물을 발행합니다(일수/교통량).

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| GET | `/api/v1/client-gifts` | 로그인됨 | 선물 목록 |
| POST | `/api/v1/client-gifts` | 로그인됨 | 선물 만들기 |
| POST | `/api/v1/client-gifts/batch` | 로그인됨 | 선물 대량 생성 |
| GET | `/api/v1/client-gifts/search-clients` | 로그인됨 | 고객 검색 |

### 추천 구성

추천 커미션 구성 및 통계.

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| GET | `/api/v1/referral/config` | 로그인됨 | 추천 구성 |
| PUT | `/api/v1/referral/config` | 관리자 | 추천 구성 저장 |
| GET | `/api/v1/referral/records` | 로그인됨 | 추천 기록 목록 |
| GET | `/api/v1/referral/stats` | 로그인됨 | 추천 통계 |

### 추천 철회

철회 요청을 검토하세요.

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| GET | `/api/v1/referral-withdrawals` | 로그인됨 | 출금 요청 목록 |
| POST | `/api/v1/referral-withdrawals/:id/approve` | 로그인됨 | 출금 승인 |
| POST | `/api/v1/referral-withdrawals/:id/reject` | 로그인됨 | 출금 거부 |

### IP 허용 목록(요약)

모든 고객에 대한 IP 허용 목록 보기.

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| GET | `/api/v1/ip-whitelists` | 로그인됨 | 모든 고객 IP 허용 목록 |

### 고객 OAuth 바인딩

고객의 제3자 계정 바인딩을 쿼리합니다.

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| GET | `/api/v1/client-oauth-bindings` | 로그인됨 | 고객 OAuth 바인딩 목록 |

### 티켓(관리자)

티켓 회신, 할당 및 종료.

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| GET | `/api/v1/tickets` | 로그인됨 | 티켓리스트 |
| GET | `/api/v1/tickets/:id` | 로그인됨 | 티켓 세부정보 |
| POST | `/api/v1/tickets/:id/assign` | 로그인됨 | 티켓 할당 |
| POST | `/api/v1/tickets/:id/close` | 로그인됨 | 티켓 닫기 |
| POST | `/api/v1/tickets/:id/reply` | 로그인됨 | 티켓에 답장하기 |

### 기사

포털 기사용 CRUD.

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| GET | `/api/v1/articles` | 로그인됨 | 기사 목록 |
| POST | `/api/v1/articles` | 로그인됨 | 기사 작성 |
| DELETE | `/api/v1/articles/:id` | 로그인됨 | 기사 삭제 |
| GET | `/api/v1/articles/:id` | 로그인됨 | 기사 세부정보 |
| PUT | `/api/v1/articles/:id` | 로그인됨 | 기사 업데이트 |

### 공유 계정

공유계정(스트리밍 등)에 대한 발급 및 수리기록.

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| GET | `/api/v1/shared-accounts` | 로그인됨 | 공유 계정 목록 |
| POST | `/api/v1/shared-accounts` | 로그인됨 | 공유 계정 만들기 |
| DELETE | `/api/v1/shared-accounts/:id` | 로그인됨 | 공유 계정 삭제 |
| GET | `/api/v1/shared-accounts/:id` | 로그인됨 | 공유 계정 세부정보 |
| PUT | `/api/v1/shared-accounts/:id` | 로그인됨 | 공유 계정 업데이트 |
| GET | `/api/v1/shared-accounts/:id/repair-logs` | 로그인됨 | 수리 기록 |

### 인증서

공인인증서(ACME) 발급, 업로드, 갱신 및 내보내기 특정 노드에서의 발행이 지원됩니다.

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| GET | `/api/v1/certificates` | 로그인됨 | 목록 |
| POST | `/api/v1/certificates` | 로그인됨 | 만들기 |
| DELETE | `/api/v1/certificates/:id` | 로그인됨 | 삭제 |
| GET | `/api/v1/certificates/:id` | 로그인됨 | 세부정보 |
| PUT | `/api/v1/certificates/:id` | 로그인됨 | 업데이트 |
| GET | `/api/v1/certificates/:id/download` | 로그인됨 | 다운로드 |
| POST | `/api/v1/certificates/:id/renew` | 로그인됨 | 고객 갱신 |
| GET | `/api/v1/certificates/check-port` | 로그인됨 | 포트 확인 |
| GET | `/api/v1/certificates/detect-ip` | 로그인됨 | 프로브 IP |
| GET | `/api/v1/certificates/export` | 로그인됨 | 수출 |
| POST | `/api/v1/certificates/issue` | 로그인됨 | 이슈. 본체: `{"domains":["example.com","*.example.com"],"challenge_type":"dns-01","dns_channel_id":2,"key_type":"rsa","key_bits":4096}`. `challenge_type`의 기본값은 http-01입니다. `dns_channel_id`는 DNS 채널을 선택합니다(구성된 DNS 공급자 + 자격 증명) |
| POST | `/api/v1/certificates/issue-on-node` | 로그인됨 | 노드에 발행 |
| POST | `/api/v1/certificates/upload` | 로그인됨 | 업로드 |

### 인증서 채널

ACME 채널(CA) 구성.

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| GET | `/api/v1/cert-channels` | 로그인됨 | 목록 |
| POST | `/api/v1/cert-channels` | 로그인됨 | 만들기 |
| DELETE | `/api/v1/cert-channels/:id` | 로그인됨 | 삭제 |
| PUT | `/api/v1/cert-channels/:id` | 로그인됨 | 업데이트 |
| GET | `/api/v1/cert-channels/providers` | 로그인됨 | 공급자 |

### EAB 구성

ZeroSSL / Google와 같은 외부 계정을 바인딩하는 자격 증명입니다.

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| GET | `/api/v1/eab-configs` | 로그인됨 | 목록 |
| POST | `/api/v1/eab-configs` | 로그인됨 | 만들기 |
| DELETE | `/api/v1/eab-configs/:id` | 로그인됨 | 삭제 |
| PUT | `/api/v1/eab-configs/:id` | 로그인됨 | 업데이트 |
| POST | `/api/v1/eab-configs/fetch-google` | 로그인됨 | GCP에서 Google EAB 가져오기 |
| POST | `/api/v1/eab-configs/fetch-zerossl` | 로그인됨 | 한 번의 클릭으로 이메일로 ZeroSSL EAB를 가져옵니다. |

### ACME 계정

패널측 ACME 계정(연락처 이메일) 관리.

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| GET | `/api/v1/acme-accounts` | 로그인됨 | 목록 |
| POST | `/api/v1/acme-accounts` | 로그인됨 | 만들기 |
| DELETE | `/api/v1/acme-accounts/:id` | 로그인됨 | 삭제 |
| PUT | `/api/v1/acme-accounts/:id` | 로그인됨 | 업데이트 |
| GET | `/api/v1/acme-accounts/default` | 로그인됨 | 기본값 가져오기 |

### 내장 CA

내부 패널 CA: 에이전트 인증서 발급/갱신/해지, CRL 내보내기, 배포 대기열.

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| GET | `/api/v1/ca/agent-certs` | 로그인됨 | 에이전트 인증서 목록 |
| POST | `/api/v1/ca/agent-certs/:server_id/renew` | 로그인됨 | 에이전트 인증서 갱신 |
| GET | `/api/v1/ca/distribution-queue` | 로그인됨 | 인증서 배포 대기열 |
| DELETE | `/api/v1/ca/distribution-queue/:id` | 로그인됨 | 분배 대기열 항목 삭제 |
| POST | `/api/v1/ca/distribution-queue/:id/retry` | 로그인됨 | 인증서 배포 재시도 |
| GET | `/api/v1/ca/revocations` | 로그인됨 | 철회 목록 |
| GET | `/api/v1/ca/revocations/export` | 로그인됨 | CRL 내보내기 |
| GET | `/api/v1/ca/status` | 로그인됨 | CA 상태 |

### 지역 규칙

geoip/geosite 데이터 소스 및 파일의 동기화 및 전달.

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| GET | `/api/v1/geo-rules/files` | 로그인됨 | 파일 목록 |
| DELETE | `/api/v1/geo-rules/files/:fileId` | 로그인됨 | 파일 삭제 |
| PUT | `/api/v1/geo-rules/files/:fileId/auto-sync` | 로그인됨 | 자동 동기화 전환 |
| POST | `/api/v1/geo-rules/files/:fileId/download` | 로그인됨 | 다운로드 |
| GET | `/api/v1/geo-rules/files/:fileId/download-file` | 로그인됨 | 파일 다운로드 |
| GET | `/api/v1/geo-rules/github-assets` | 로그인됨 | GitHub 저장소 자산 가져오기 |
| POST | `/api/v1/geo-rules/push-all` | 로그인됨 | 모든 노드에 푸시 |
| POST | `/api/v1/geo-rules/seed` | 로그인됨 | 기본 데이터 초기화 |
| GET | `/api/v1/geo-rules/sources` | 로그인됨 | 데이터 소스 목록 |
| POST | `/api/v1/geo-rules/sources` | 로그인됨 | 데이터 소스 만들기 |
| DELETE | `/api/v1/geo-rules/sources/:sourceId` | 로그인됨 | 데이터 소스 삭제 |
| PUT | `/api/v1/geo-rules/sources/:sourceId` | 로그인됨 | 데이터 소스 업데이트 |
| GET | `/api/v1/geo-rules/sources/:sourceId/assets` | 로그인됨 | 릴리스 자산 가져오기 |
| POST | `/api/v1/geo-rules/sources/:sourceId/files` | 로그인됨 | 파일 추가 |
| PUT | `/api/v1/geo-rules/sources/:sourceId/schedule` | 로그인됨 | 예약된 동기화 전환 |
| POST | `/api/v1/geo-rules/sources/:sourceId/select` | 로그인됨 | 파일 선택 |
| POST | `/api/v1/geo-rules/sources/:sourceId/sync` | 로그인됨 | 이 데이터 소스 동기화 |
| GET | `/api/v1/geo-rules/status` | 로그인됨 | 상태 |
| POST | `/api/v1/geo-rules/sync-all` | 로그인됨 | 모두 동기화 |
| GET | `/api/v1/geo-rules/sync-status/:key` | 로그인됨 | 동기화 진행 |

### WAF 규칙 라이브러리

글로벌 WAF 규칙: GitHub 소스, 동기화, 모든 노드에 전달.

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| GET | `/api/v1/waf/github-source` | 로그인됨 | GitHub 데이터 소스 |
| PUT | `/api/v1/waf/github-source` | 로그인됨 | GitHub 데이터 소스 저장 |
| POST | `/api/v1/waf/push-all` | 로그인됨 | 모든 노드에 푸시 |
| GET | `/api/v1/waf/rules` | 로그인됨 | 규칙 목록 |
| DELETE | `/api/v1/waf/rules/:name` | 로그인됨 | 규칙 삭제 |
| GET | `/api/v1/waf/rules/:name` | 로그인됨 | 규칙 가져오기 |
| PUT | `/api/v1/waf/rules/:name` | 로그인됨 | 규칙 저장 |
| POST | `/api/v1/waf/sync-all` | 로그인됨 | 모두 동기화 |
| POST | `/api/v1/waf/sync-github` | 로그인됨 | GitHub에서 동기화 |

### 로그 관리

노드/코어 로그 읽기 및 스트리밍, 정리 정책, 로그 작성자 제어.

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| POST | `/api/v1/log-manage/cleanup-all` | 로그인됨 | 이제 다 정리하세요 |
| GET | `/api/v1/log-manage/cleanup-rules` | 로그인됨 | 정리 규칙 목록 |
| PUT | `/api/v1/log-manage/cleanup-rules/:id` | 로그인됨 | 정리 규칙 업데이트 |
| POST | `/api/v1/log-manage/cleanup-table/:table` | 로그인됨 | 지금 지정된 테이블을 정리하세요. |
| GET | `/api/v1/log-manage/core-log-output` | 로그인됨 | 코어 로그 출력 구성 |
| PUT | `/api/v1/log-manage/core-log-output` | 로그인됨 | 코어 로그 출력 구성 저장 |
| GET | `/api/v1/log-manage/core/:type/stdout-log` | 로그인됨 | 핵심 stdout 로그 읽기 |
| GET | `/api/v1/log-manage/core/:type/stdout-stream` | 로그인됨 | 스트림 코어 표준 출력 |
| POST | `/api/v1/log-manage/discover` | 로그인됨 | 노드 로그 파일 검색 |
| GET | `/api/v1/log-manage/log-output` | 로그인됨 | 로그 출력 구성 |
| PUT | `/api/v1/log-manage/log-output` | 로그인됨 | 로그 출력 구성 저장 |
| POST | `/api/v1/log-manage/log-output/release/:type` | 로그인됨 | 릴리스 로그 stdout 인계 |
| POST | `/api/v1/log-manage/log-output/takeover/:type` | 로그인됨 | 로그 표준 출력 인계 |
| GET | `/api/v1/log-manage/nodes/:serverId/logs` | 로그인됨 | 노드 로그 파일 목록 |
| GET | `/api/v1/log-manage/nodes/:serverId/logs/:type` | 로그인됨 | 노드 로그 읽기 |
| POST | `/api/v1/log-manage/nodes/:serverId/logs/:type/rotate` | 로그인됨 | 노드 로그 순환 |
| GET | `/api/v1/log-manage/nodes/:serverId/logs/:type/stream` | 로그인됨 | 스트림 노드 로그 |
| GET | `/api/v1/log-manage/policies` | 로그인됨 | 정책 목록 |
| POST | `/api/v1/log-manage/policies` | 로그인됨 | 정책 만들기 |
| DELETE | `/api/v1/log-manage/policies/:id` | 로그인됨 | 정책 삭제 |
| GET | `/api/v1/log-manage/policies/:id` | 로그인됨 | 정책 세부정보 |
| PUT | `/api/v1/log-manage/policies/:id` | 로그인됨 | 정책 업데이트 |
| GET | `/api/v1/log-manage/policies/:id/jobs` | 로그인됨 | 정리 작업 목록 |
| GET | `/api/v1/log-manage/policies/:id/log` | 로그인됨 | 로그 읽기 |
| GET | `/api/v1/log-manage/policies/:id/log-stream` | 로그인됨 | 스트림 로그 |
| POST | `/api/v1/log-manage/policies/:id/rotate` | 로그인됨 | 회전 |
| GET | `/api/v1/log-manage/proxy/:type/stdout-log` | 로그인됨 | 핵심 stdout 로그 읽기 |
| GET | `/api/v1/log-manage/proxy/:type/stdout-stream` | 로그인됨 | 스트림 코어 표준 출력 |
| GET | `/api/v1/log-manage/scheduler-config` | 로그인됨 | 예약 구성 |
| PUT | `/api/v1/log-manage/scheduler-config` | 로그인됨 | 일정 구성 업데이트 |
| GET | `/api/v1/log-manage/status` | 로그인됨 | 상태 |
| POST | `/api/v1/log-manage/sync` | 로그인됨 | 동기화 정리 정책 |
| GET | `/api/v1/log-manage/writers` | 로그인됨 | 작가 상태 |
| PUT | `/api/v1/log-manage/writers/:type/config` | 로그인됨 | 작성자 구성 업데이트 |
| GET | `/api/v1/log-manage/writers/:type/files` | 로그인됨 | 작성자 로그 파일 목록 |
| POST | `/api/v1/log-manage/writers/:type/start` | 로그인됨 | 작가 시작 |
| POST | `/api/v1/log-manage/writers/:type/stop` | 로그인됨 | 작가 중지 |

### 백업

데이터베이스/전체 백업, 복원, 예약된 작업, FTP/S3/WebDAV 원격 스토리지.

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| DELETE | `/api/v1/backups/:id` | 관리자 | 백업 삭제 |
| GET | `/api/v1/backups/:id/download` | 관리자 | 백업 다운로드 |
| POST | `/api/v1/backups/:id/restore` | 관리자 | 백업에서 복원 |
| POST | `/api/v1/backups/:id/upload-ftp` | 관리자 | FTP에 업로드 |
| POST | `/api/v1/backups/:id/upload-s3` | 관리자 | S3에 업로드 |
| POST | `/api/v1/backups/:id/upload-webdav` | 관리자 | WebDAV에 업로드 |
| POST | `/api/v1/backups/create` | 관리자 | 전체 백업 생성(DB 덤프 포함) |
| GET | `/api/v1/backups/list` | 관리자 | 전체 백업 목록 |
| POST | `/api/v1/backups/remote-restore` | 관리자 | 원격에서 복원 |
| GET | `/api/v1/backups/restore-status` | 관리자 | 복원 진행 상황 |
| GET | `/api/v1/backups/schedules` | 관리자 | 예정된 작업 목록 |
| POST | `/api/v1/backups/schedules` | 관리자 | 예약된 작업 만들기 |
| DELETE | `/api/v1/backups/schedules/:id` | 관리자 | 예약된 작업 삭제 |
| PUT | `/api/v1/backups/schedules/:id` | 관리자 | 예약된 작업 업데이트 |
| GET | `/api/v1/backups/storages` | 관리자 | 스토리지 구성 목록 |
| POST | `/api/v1/backups/storages` | 관리자 | 스토리지 구성 만들기 |
| DELETE | `/api/v1/backups/storages/:id` | 관리자 | 스토리지 구성 삭제 |
| PUT | `/api/v1/backups/storages/:id` | 관리자 | 스토리지 구성 업데이트 |
| GET | `/api/v1/backups/storages/:id/files` | 관리자 | 원격 백업 파일 목록 |
| POST | `/api/v1/backups/upload-restore` | 관리자 | 백업 업로드 및 복원 |

### 시스템 설정

데이터베이스 구성 및 마이그레이션, 패널 설정, JWT 키, SMTP/Telegram 테스트, 알림 템플릿, 온라인 업그레이드(주로 관리자).

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| PUT | `/api/v1/system/bootstrap-finish` | 관리자 | 첫 번째 부팅 설정 완료 |
| GET | `/api/v1/system/bootstrap-state` | 관리자 | 최초 부팅 상태 |
| GET | `/api/v1/system/config-file/:coreType` | 관리자 | 핵심 구성 파일 읽기 |
| GET | `/api/v1/system/db-config` | 관리자 | 데이터베이스 구성 |
| PUT | `/api/v1/system/db-config` | 관리자 | 데이터베이스 구성 업데이트 |
| POST | `/api/v1/system/db-config/migrate` | 관리자 | 데이터베이스 데이터 마이그레이션 |
| POST | `/api/v1/system/db-config/switch` | 관리자 | 데이터베이스 전환(원스텝 마이그레이션 포함) |
| POST | `/api/v1/system/db-config/test` | 관리자 | 데이터베이스 연결 테스트 |
| GET | `/api/v1/system/db-pool-stats` | 관리자 | 연결 풀 통계 |
| GET | `/api/v1/system/db-stats` | 관리자 | 데이터베이스 통계 |
| GET | `/api/v1/system/db-stats/ping` | 관리자 | 데이터베이스 프로브 |
| POST | `/api/v1/system/db-stats/reset` | 관리자 | 데이터베이스 통계 재설정 |
| GET | `/api/v1/system/encryption-key-status` | 관리자 | 암호화 키 상태 |
| GET | `/api/v1/system/env-config` | 관리자 | .env 구성 |
| PUT | `/api/v1/system/env-config` | 관리자 | .env 구성 업데이트 |
| POST | `/api/v1/system/generate-encryption-key` | 관리자 | 암호화 키 생성 |
| GET | `/api/v1/system/github-config` | 관리자 | GitHub 구성 |
| PUT | `/api/v1/system/github-config` | 관리자 | GitHub 구성 업데이트 |
| GET | `/api/v1/system/jwt-secret-status` | 관리자 | JWT 키 상태 |
| GET | `/api/v1/system/maintenance` | 공개 | 유지 관리 모드 상태(공개) |
| PUT | `/api/v1/system/maintenance` | 관리자 | 유지 관리 모드 전환 |
| POST | `/api/v1/system/migrate-data` | 관리자 | 데이터 마이그레이션 |
| GET | `/api/v1/system/notification-config` | 관리자 | 알림 구성 |
| PUT | `/api/v1/system/notification-config` | 관리자 | 알림 구성 업데이트 |
| GET | `/api/v1/system/notification-templates` | 관리자 | 템플릿 목록 |
| POST | `/api/v1/system/notification-templates` | 관리자 | 템플릿 만들기 |
| DELETE | `/api/v1/system/notification-templates/:id` | 관리자 | 템플릿 삭제 |
| PUT | `/api/v1/system/notification-templates/:id` | 관리자 | 템플릿 업데이트 |
| POST | `/api/v1/system/notification-templates/:id/broadcast` | 관리자 | 템플릿에 의한 방송 알림 |
| POST | `/api/v1/system/notification-templates/:id/send` | 관리자 | 테스트 알림 보내기 |
| GET | `/api/v1/system/panel-config` | 관리자 | 패널 구성 |
| PUT | `/api/v1/system/panel-config` | 관리자 | 패널 구성 업데이트 |
| GET | `/api/v1/system/reality-targets` | 관리자 | REALITY 대상 목록 |
| PUT | `/api/v1/system/reality-targets` | 관리자 | REALITY 대상 업데이트 |
| POST | `/api/v1/system/reality-targets/reset` | 관리자 | REALITY 대상을 기본값으로 재설정 |
| POST | `/api/v1/system/regenerate-jwt-secret` | 관리자 | JWT 키 재생성 |
| GET | `/api/v1/system/releases` | 관리자 | 사용 가능한 버전 |
| POST | `/api/v1/system/restart` | 관리자 | 다시 시작 |
| POST | `/api/v1/system/rollback` | 관리자 | 시스템 롤백 |
| GET | `/api/v1/system/settings` | 관리자 | 설정 읽기 |
| PUT | `/api/v1/system/settings/batch` | 관리자 | 대량 업데이트 설정 |
| GET | `/api/v1/system/stats` | 관리자 | 시스템 상태 |
| GET | `/api/v1/system/subscription-billing-templates` | 관리자 | 구독 청구 템플릿 |
| PUT | `/api/v1/system/subscription-billing-templates` | 관리자 | 구독 청구 템플릿 업데이트 |
| POST | `/api/v1/system/telegram-chat-id` | 관리자 | Telegram 채팅 ID 받기 |
| POST | `/api/v1/system/test-smtp` | 관리자 | SMTP 테스트 |
| POST | `/api/v1/system/test-telegram` | 관리자 | Telegram 테스트 |
| POST | `/api/v1/system/update` | 관리자 | 시스템 업그레이드 |
| GET | `/api/v1/system/update-jobs` | 관리자 | 업그레이드 작업 목록 |
| POST | `/api/v1/system/upload-mtls-ca` | 관리자 | mTLS CA 인증서 업로드 |

### 보안 감사

이벤트 쿼리를 감사하고 내보냅니다.

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| GET | `/api/v1/security/audit-events` | 로그인됨 | 감사 이벤트 목록 |
| GET | `/api/v1/security/audit-events/export` | 로그인됨 | 감사 이벤트 내보내기 |

### 측정항목

패널 집계 지표(노드/인증서/서비스).

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| GET | `/api/v1/metrics/certificates` | 로그인됨 | 인증서 지표 |
| GET | `/api/v1/metrics/nodes` | 로그인됨 | 노드 측정항목 |
| GET | `/api/v1/metrics/overview` | 로그인됨 | 개요 |

### 포털 템플릿

포털 템플릿, 활성화 구성 및 캐시 관리.

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| GET | `/api/v1/portal/active` | 로그인됨 | 활성 템플릿 |
| PUT | `/api/v1/portal/active` | 로그인됨 | 활성 템플릿 업데이트 |
| DELETE | `/api/v1/portal/cache` | 로그인됨 | 포털 캐시 지우기 |
| GET | `/api/v1/portal/cache` | 로그인됨 | 캐시 상태 |
| PUT | `/api/v1/portal/cache/dev-mode` | 로그인됨 | 캐시 개발 모드 전환 |
| POST | `/api/v1/portal/cache/rebuild` | 로그인됨 | 포털 캐시 재구축 |
| POST | `/api/v1/portal/reset` | 로그인됨 | 기본 템플릿 복원 |
| GET | `/api/v1/portal/templates` | 로그인됨 | 목록 |
| POST | `/api/v1/portal/templates` | 로그인됨 | 만들기 |
| DELETE | `/api/v1/portal/templates/:id` | 로그인됨 | 삭제 |
| GET | `/api/v1/portal/templates/:id` | 로그인됨 | 세부정보 |
| PUT | `/api/v1/portal/templates/:id` | 로그인됨 | 업데이트 |
| POST | `/api/v1/portal/templates/:id/activate` | 로그인됨 | 템플릿 활성화 |

> `/portal/templates` CRUD는 **고급 API**입니다. 포털 UI는 주로 `PUT /api/v1/portal/active`를 사용합니다.
> (원샷 전체 사이트 구성) 이 그룹으로 여러 템플릿을 관리/전환합니다. 둘 다 지원되며 충돌하지 않습니다.

### 에이전트 설치

노드 에이전트 설치 스크립트 및 다운로드(공개)

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| POST | `/api/v1/agent/bootstrap` | 공개 | 에이전트 최초 부팅 등록(에이전트가 부트스트랩 토큰으로 보고, 공개) |
| GET | `/api/v1/agent/releases` | 관리자 | 사용 가능한 에이전트 버전(업그레이드 페이지용) |
| GET | `/api/v1/agent/version` | 관리자 | 패널의 현재 에이전트 버전 |
| GET | `/api/v1/agent/download` | 공개 | 에이전트 다운로드 |
| GET | `/api/v1/agent/install` | 공개 | 설치 스크립트 받기 |

### 청구 콜백 항목

제3자 결제 플랫폼 콜백(공개, 서명 확인)

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| POST | `/api/v1/charge/callback/:site_id` | 공개 | 청구 콜백 처리 |

### 인바운드(노드 포함)

계획 노드 선택에 사용되는 인바운드 목록입니다.

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| GET | `/api/v1/inbounds-with-server` | 로그인됨 | 모든 인바운드(노드 포함) |

### 공개 구성

로그인 없는 포털 런타임 구성(유지 관리 가드 + 공개 속도 제한)

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| GET | `/api/v1/public/articles` | 공개 | 공개 기사 목록 |
| GET | `/api/v1/public/articles/:slug` | 공개 | 공개 기사 세부정보 |
| GET | `/api/v1/public/client-configs` | 공개 | 클라이언트 다운로드 구성(공개) |
| GET | `/api/v1/public/portal-config` | 공개 | 포털 런타임 구성(사이트 이름, 테마, 언어, 규정 준수 문서, 공개) |
| GET | `/api/v1/public/settings` | 공개 | 공개 설정(예: Registration_enabled, 공개) |
| GET | `/api/v1/public/subscription-endpoints` | 공개 | 활성화된 구독 엔드포인트 목록(공개) |

### 클라이언트 인증 및 비즈니스(관리 엔진에서도 동일 설정)

포털과 동일한 클라이언트 엔드포인트 자세한 내용은 `portal-api.en.md`를 참조하세요.

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| GET | `/api/v1/client-auth/articles` | 공개 | 기사 나열 |
| GET | `/api/v1/client-auth/articles/:slug` | 공개 | 기사 받기 |
| GET | `/api/v1/client-auth/balance` | 공개 | 잔액 확보 |
| GET | `/api/v1/client-auth/balance/transactions` | 공개 | 잔액 거래 |
| GET | `/api/v1/client-auth/bills` | 공개 | 주문 목록 |
| POST | `/api/v1/client-auth/change-password` | 공개 | 비밀번호 변경 |
| POST | `/api/v1/client-auth/charge/create` | 공개 | 재충전 주문 생성 |
| GET | `/api/v1/client-auth/charge/orders` | 공개 | 재충전 주문 나열 |
| GET | `/api/v1/client-auth/charge/orders/:id` | 공개 | 재충전 주문 세부정보 받기 |
| POST | `/api/v1/client-auth/charge/redeem-card` | 공개 | 카드 사용 |
| GET | `/api/v1/client-auth/charge/sites` | 공개 | 충전소 나열 |
| GET | `/api/v1/client-auth/charge/status/:trade_no` | 공개 | 충전상태 조회 |
| GET | `/api/v1/client-auth/checktime` | 공개 | 서버 시간 가져오기(시계 동기화) |
| GET | `/api/v1/client-auth/connection-info` | 공개 | 계획 연결 매개변수 가져오기 |
| GET | `/api/v1/client-auth/gifts` | 공개 | 선물 나열 |
| POST | `/api/v1/client-auth/gifts/:id/claim` | 공개 | 선물 받기 |
| GET | `/api/v1/client-auth/invoices` | 공개 | 주문 목록 |
| GET | `/api/v1/client-auth/invoices/:id` | 공개 | 주문 세부정보 |
| GET | `/api/v1/client-auth/ip-whitelist` | 공개 | IP 허용 목록 나열 |
| DELETE | `/api/v1/client-auth/ip-whitelist/:id` | 공개 | IP 허용 목록에서 제거 |
| POST | `/api/v1/client-auth/ip-whitelist/auto-add` | 공개 | IP 자동 추가 |
| POST | `/api/v1/client-auth/login` | 공개 | 로그인 |
| POST | `/api/v1/client-auth/logout` | 공개 | 로그아웃 |
| POST | `/api/v1/client-auth/logs/package` | 공개 | 로그 번들 생성 |
| GET | `/api/v1/client-auth/logs/packages` | 공개 | 로그 번들 나열 |
| DELETE | `/api/v1/client-auth/logs/packages/:id` | 공개 | 로그 번들 삭제 |
| GET | `/api/v1/client-auth/logs/packages/:id` | 공개 | 로그 번들 가져오기 |
| GET | `/api/v1/client-auth/logs/packages/:id/download` | 공개 | 로그 번들 다운로드 |
| GET | `/api/v1/client-auth/logs/types` | 공개 | 로그 번들 상태 확인 |
| GET | `/api/v1/client-auth/me` | 공개 | 현재 로그인한 사용자 가져오기 |
| GET | `/api/v1/client-auth/nodes` | 공개 | 내 노드 목록 |
| GET | `/api/v1/client-auth/nodes/:id/config` | 공개 | 노드 구성 |
| GET | `/api/v1/client-auth/nodes/:id/link` | 공개 | 노드 공유 링크 |
| GET | `/api/v1/client-auth/nodes/:id/qrcode` | 공개 | 노드 QR 코드 |
| GET | `/api/v1/client-auth/oauth/:provider` | 공개 | 승인하다 |
| GET | `/api/v1/client-auth/oauth/:provider/callback` | 공개 | 콜백 |
| GET | `/api/v1/client-auth/oauth/providers` | 공개 | 제공자 상태 |
| GET | `/api/v1/client-auth/orders` | 공개 | 주문 목록 |
| POST | `/api/v1/client-auth/orders` | 공개 | 주문 생성 |
| GET | `/api/v1/client-auth/orders/:id` | 공개 | 주문 세부정보 |
| GET | `/api/v1/client-auth/plans` | 공개 | 플랜 목록 |
| GET | `/api/v1/client-auth/plans/:id` | 공개 | 요금제 세부정보 |
| GET | `/api/v1/client-auth/profile` | 공개 | 프로필 받기 |
| PUT | `/api/v1/client-auth/profile` | 공개 | 프로필 업데이트 |
| POST | `/api/v1/client-auth/profile/bind-oauth` | 공개 | 타사 계정 바인딩 |
| DELETE | `/api/v1/client-auth/profile/unbind-oauth/:provider` | 공개 | 타사 계정 바인딩 해제 |
| GET | `/api/v1/client-auth/referral/code` | 공개 | 추천 확인 코드 받기 |
| GET | `/api/v1/client-auth/referral/records` | 공개 | 추천 기록 나열 |
| GET | `/api/v1/client-auth/referral/stats` | 공개 | 추천 통계 받기 |
| POST | `/api/v1/client-auth/referral/withdraw` | 공개 | 출금요청 |
| GET | `/api/v1/client-auth/referral/withdrawals` | 공개 | 추천 철회 나열 |
| POST | `/api/v1/client-auth/refresh` | 공개 | 새로고침 |
| POST | `/api/v1/client-auth/register` | 공개 | 등록 |
| POST | `/api/v1/client-auth/reset-subscription` | 공개 | 구독 재설정 |
| POST | `/api/v1/client-auth/rotate-token` | 공개 | 구독 토큰 순환 |
| POST | `/api/v1/client-auth/self-unban` | 공개 | 셀프 서비스 금지 해제 |
| PUT | `/api/v1/client-auth/settings` | 공개 | 업데이트 설정 |
| POST | `/api/v1/client-auth/settings/auto-add-ip` | 공개 | IP 자동 추가 |
| GET | `/api/v1/client-auth/settings/whitelist` | 공개 | IP 허용 목록 나열 |
| DELETE | `/api/v1/client-auth/settings/whitelist/:id` | 공개 | IP 허용 목록에서 제거 |
| GET | `/api/v1/client-auth/shared-accounts` | 공개 | 공유 계정 나열 |
| GET | `/api/v1/client-auth/shared-accounts/:id` | 공개 | 공유 계정 받기 |
| POST | `/api/v1/client-auth/shared-accounts/:id/apply` | 공개 | 공유 계정 요청 |
| POST | `/api/v1/client-auth/shared-accounts/:id/repair` | 공개 | 수리요청 |
| POST | `/api/v1/client-auth/ticket` | 공개 | 일회성 단기 티켓 발행 |
| GET | `/api/v1/client-auth/tickets` | 공개 | 티켓 나열 |
| POST | `/api/v1/client-auth/tickets` | 공개 | 티켓 만들기 |
| GET | `/api/v1/client-auth/tickets/:id` | 공개 | 티켓 받기 |
| POST | `/api/v1/client-auth/tickets/:id/close` | 공개 | 티켓 닫기 |
| POST | `/api/v1/client-auth/tickets/:id/reply` | 공개 | 티켓에 답장하기 |
| GET | `/api/v1/client-auth/traffic-history` | 공개 | 교통 이력 |
| GET | `/api/v1/client-auth/traffic-packs` | 공개 | 트래픽 팩 목록 |
| POST | `/api/v1/client-auth/traffic-packs/buy` | 공개 | 트래픽 팩 구매 |

### 구독 전달

클라이언트가 구독을 가져오는 공개 진입점입니다. `subscription.en.md`를 참조하세요.

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| GET | `/sub/:token` | 공개 | 구독 제공 |

### 런타임 구성 워크플로

스냅샷 구성/드라이런/적용/롤백(`apply`, `rollback` 및 `disable-proxy`에는 관리자 필요)

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| POST | `/runtime/apply` | 공개 | 적용 |
| GET | `/runtime/apply-jobs` | 로그인됨 | 채용정보 나열 |
| GET | `/runtime/apply-jobs/:id` | 로그인됨 | 취업 |
| POST | `/runtime/apply-jobs/:id/cancel` | 공개 | 작업 취소 |
| POST | `/runtime/apply-jobs/:id/retry` | 공개 | 작업 재시도 |
| POST | `/runtime/disable-proxy` | 공개 | 프록시 비활성화 |
| POST | `/runtime/dry-run` | 로그인됨 | 드라이런 실행 |
| GET | `/runtime/nodes` | 로그인됨 | 노드 나열 |
| GET | `/runtime/overview` | 로그인됨 | 개요 |
| POST | `/runtime/rollback` | 공개 | 롤백 |
| GET | `/runtime/snapshots` | 로그인됨 | 스냅샷 나열 |
| POST | `/runtime/snapshots` | 로그인됨 | 스냅샷 생성 |
| DELETE | `/runtime/snapshots/:id` | 공개 | 스냅샷 삭제 |
| GET | `/runtime/snapshots/:id` | 로그인됨 | 스냅샷 받기 |
| POST | `/runtime/verify` | 로그인됨 | 확인 |

### 서비스 액세스 정책(폐기됨)

> `/services/:id/access-policies` 및 `/services/:id/ip-bans`는 2026년 10월 20일에 중단되었습니다.
> 관리자 호출이 없으며 연결 경로가 이러한 정책을 사용하지 않습니다.

### 트래픽 할당량 모니터링

할당량 및 사용량에 대한 읽기 전용 모니터링 보기입니다.

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| GET | `/traffic/quotas` | 로그인됨 | 할당량 목록 |
| GET | `/traffic/quotas/:id/usage` | 로그인됨 | 할당량 사용량 |
| GET | `/traffic/reset-logs` | 로그인됨 | 할당량 재설정 기록 |
| GET | `/traffic/usage-history` | 로그인됨 | 이용내역 |

### 건강검진

활성 프로브.

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| GET | `/health` | 공개 | 활동성 프로브; 200은 건강하다는 뜻 |

### 포털 페이지

포털 SPA 페이지.

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| GET | `/p` | 공개 | 포털 HTML(서버 렌더링 + Vue 런타임) |

### ACME 과제

HTTP-01 챌린지 콜백.

| 방법 | 경로 | 허가 | 설명 |
|---|---|---|---|
| GET | `/.well-known/acme-challenge/:token` | 공개 | ACME HTTP-01 챌린지 콜백(발급 중 도메인 확인) |
