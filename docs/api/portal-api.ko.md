<!-- source: portal-api.md | locale: ko | updated: 2026-10-23 -->
# Portal API 참조(클라이언트/포털 API)

> 언어: [简体中文](portal-api.md) · [English](portal-api.en.md) · [繁體中文](portal-api.zh-TW.md) · [Русский](portal-api.ru.md) · [日本語](portal-api.ja.md) · 한국어 · [Deutsch](portal-api.de.md) · [Français](portal-api.fr.md) · [Español](portal-api.es.md) · [Português](portal-api.pt.md) · [Türkçe](portal-api.tr.md) · [Bahasa Indonesia](portal-api.id.md) · [ไทย](portal-api.th.md) · [Tiếng Việt](portal-api.vi.md) · [العربية](portal-api.ar.md) · [فارسی](portal-api.fa.md)

> 포털 프런트엔드(`/p` 또는 전용 포털 포트) 및 타사 클라이언트에 대한 **전체** 인터페이스 참조입니다.
> 경로 정의: `internal/api/router/router.go`(`clientAuthGroup` / `clientAuthProtected` / `/api/v1/public/*`).

## 1. 일반 규칙

### 1.1 베이스 URL

| 배포 | 베이스 URL |
|---|---|
| 메인 패널 엔진 | `https://<panel-domain>/api/v1`(`/p`의 포털 페이지) |
| 전용 포털 포트 | `https://<portal-domain>[:port]/api/v1`(`/`의 포털 페이지) |
| 전용포털도메인 + 역방향프록시 | 위와 동일합니다. 프록시는 페이지 경로와 `/api/v1`를 모두 전달해야 합니다. |

### 1.2 인증

- 로그인 후 `Authorization: Bearer <access_token>`를 보내세요.
- 액세스 토큰은 24시간 동안 지속되고, 새로 고침 토큰은 7일 동안 지속됩니다.
- JWT에는 `tv`(token_version)가 포함되어 있습니다. 로그아웃/강제 로그아웃은 이를 증가시키고 모든 이전 토큰을 무효화합니다.
 (`clients.token_version`, `internal/service/clientauth/service.go` 참조);
- 모든 보호된 엔드포인트는 `ClientAuthMiddleware`(버전 확인 + 유지 관리 모드 가드)를 통과합니다.

### 1.3 응답 봉투

대부분의 엔드포인트는 `{ "code": 0, "data": ..., "message": "..." }`를 반환합니다.
일부 목록 끝점은 `{ "items": [...], "total": N }` 또는 기본 배열(아래 끝점별로 표시)을 반환합니다.
오류가 `{ "code": "ERROR_CODE", "message": "...", "request_id": "...", "can_retry": ... }`를 반환합니다.
(HTTP 4xx/5xx).

### 1.4 금액 단위

주문/잔고 필드는 **정수 센트**입니다: `990` = 9.90엔(`currency: "CNY"`).

### 1.5 실제 예시 (로그인 → 프로필 가져오기)

```bash
# Register (when registration_enabled=true)
curl -X POST https://<portal-domain>/api/v1/client-auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","password":"plaintext-password","referral_code":""}'
# → 201 {"data":{"access_token":"<JWT>","refresh_token":"<JWT>","client":{...}}}

# Sign in
curl -X POST https://<portal-domain>/api/v1/client-auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","password":"plaintext-password"}'
# → 200 same as above

# Fetch the current client profile (with token)
curl https://<portal-domain>/api/v1/client-auth/me \
  -H 'Authorization: Bearer <access_token>'
# → 200 {"data":{"id":27,"email":"user@example.com","nickname":"","status":"active","quota_bytes":10737418240,"used_bytes":0,...}}

# Fetch a subscription (no header, token in the path)
curl 'https://<portal-domain>/sub/<subscription_token>?format=mihomo_yaml'
# → 200 Mihomo YAML (or base64_uri / singbox_json / xray_json)

# Error example: expired token
# → 401 {"code":"errClientTokenRevoked","message":"...","request_id":"...","can_retry":false}
```

401에서 `POST /api/v1/client-auth/refresh`를 한 번 호출하고 다시 시도하세요. 그래도 실패하면 로컬 세션을 지우십시오(포털 템플릿에서 이미 이 작업을 수행함).

---

## 2. 공개 엔드포인트(로그인 없음)

| 방법 | 경로 | 설명 |
|---|---|---|
| GET | `/p`(전용 포털 엔진의 `/`) | 포털 HTML(서버 렌더링 + Vue 런타임) |
| GET | `/api/v1/public/portal-config` | 포털 런타임 구성: 사이트 이름, 테마 색상, 탐색, 모듈, 언어 팩, `registration_enabled`, `regions`(로그인 지역 소스), `privacy_policy_url` / `terms_of_service_url` / `privacy_policy_content` / `terms_of_service_content`(규정 준수 문서, 언어별로 매핑된 콘텐츠, 독립 실행형 페이지 `/p/legal/*` ✓) |
| GET | `/api/v1/public/settings` | 공개 설정(현재 `registration_enabled`) |
| GET | `/api/v1/public/subscription-endpoints` | 구독 끝점 목록(베어 배열) |
| GET | `/api/v1/public/articles` | 공개 기사(게시된 경우에만) |
| GET | `/api/v1/public/articles/:slug` | 기사 세부정보 |
| GET | `/api/v1/public/client-configs` | 클라이언트 다운로드 구성 |
| GET | `/sub/:token` | 구독 전달(`format`는 base64_uri / raw_uri / mihomo_yaml / singbox_json / xray_json 등을 반환합니다. 취소됨/누락 → 404, 일시 중지/만료/비활성 → 403. [subscription.md](portal-api.ko.md) 참조) |
| GET | `/api/v1/client-auth/oauth/providers` | OAuth 제공자 가시성: `{"data":{"providers":[{"provider":"github","enabled":true},...]}}` |
| GET | `/api/v1/system/maintenance` | 유지 관리 상태(공개) 활성화되면 포털에 유지 관리 페이지가 표시됩니다. |

> 이들 모두에는 유지 관리 가드와 공개 API 속도 제한이 포함됩니다.

## 3. 인증/OAuth

| 방법 | 경로 | 인증 | 설명 |
|---|---|---|---|
| POST | `/api/v1/client-auth/register` | — | 등록하세요. 본체: `{email, password, referral_code?}`. `{access_token, refresh_token, client}`를 반환합니다. 등록률 제한 |
| POST | `/api/v1/client-auth/login` | — | 로그인하세요. 본문: `{email, password}`. 같은 반응; `token_version`로 JWT를 발행합니다. |
| POST | `/api/v1/client-auth/refresh` | — | 새로 고침. 본체: `{refresh_token}`. 새 액세스/새로 고침 토큰을 반환합니다(버전 확인, 이전 새로 고침 토큰은 로그아웃 후 즉시 사라짐) |
| POST | `/api/v1/client-auth/logout` | ✓ | 로그아웃: 서버 측 취소(`token_version +1`) → 클라이언트의 모든 이전 토큰이 유효하지 않습니다. |
| POST | `/api/v1/client-auth/rotate-token` | ✓ | 구독 토큰 순환 |
| POST | `/api/v1/client-auth/ticket` | ✓ | **일회성 단기 티켓** 발행(헤더를 설정할 수 없는 "로그 번들 다운로드"와 같은 `window.open` 흐름의 경우) |
| GET | `/api/v1/client-auth/oauth/:provider` | — | OAuth 시작: 307 → GitHub/Google 동의(`state` anti-CSRF, 10분) 장애인 공급자는 400을 반환합니다. |
| GET | `/api/v1/client-auth/oauth/:provider/callback` | — | OAuth 콜백: 코드 교환 → 클라이언트 생성/연결 → JWT 버전 발행 → 302를 다시 포털(`/<portalPath>?access_token=...&refresh_token=...`)로 보냅니다. |

> OAuth 콜백은 **브라우저 리디렉션**(API 호출 아님)입니다. 로그인에 성공하면 포털에서 토큰을 읽습니다.
> `onMounted`의 URL에서(또는 동일한 출처 `postMessage`를 통해, `web` 포털 템플릿의 `handleOAuthCallback` 참조)

## 4. 계정

| 방법 | 경로 | 설명 |
|---|---|---|
| GET | `/api/v1/client-auth/me` | 현재 클라이언트: `{id, email, plan_id, plan_name, status, quota_bytes, used_bytes, upload_bytes, download_bytes, expires_at, ip_whitelist, device_limit, sub_token, sub_status, created_at, ...}` (`data` 봉투) |
| POST | `/api/v1/client-auth/change-password` | 본체: `{old_password, new_password}` |
| GET | `/api/v1/client-auth/checktime` | 서버 시간(클라이언트 시계 동기화) |
| GET | `/api/v1/client-auth/connection-info` | 현재 계획 연결 매개변수: `{concurrent, device_limit, protocol, speed_limit, tls, transport}` |

## 5. 구독

| 방법 | 경로 | 설명 |
|---|---|---|
| POST | `/api/v1/client-auth/reset-subscription` | 구독을 재설정하세요. 이것은 `sub_token`를 회전시킵니다. |

> 구독 링크는 `/me`(`/sub/:token`)에서 반환된 `sub_token`의 프런트엔드에 구축됩니다. 없다
> 더 이상 `subscription-link` / `subscription-formats` 엔드포인트를 분리합니다.

## 6. 노드

| 방법 | 경로 | 설명 |
|---|---|---|
| GET | `/api/v1/client-auth/nodes` | 현재 계획에 표시되는 노드: `{items:[...]}`(이름/프로토콜/전송/지역과 같은 필드 표시) |
| GET | `/api/v1/client-auth/nodes/:id/config` | 전체 노드 구성 |
| GET | `/api/v1/client-auth/nodes/:id/link` | 링크 공유(vless:// 등) |
| GET | `/api/v1/client-auth/nodes/:id/qrcode` | QR 코드 이미지 |

## 7. 매장/계획/주문

| 방법 | 경로 | 설명 |
|---|---|---|
| GET | `/api/v1/client-auth/plans` | 계획 목록(베어 어레이): `{id, name, description, category, sort_order, variants:[{id, name, billing_cycle, duration_days, price(cents), original_price, currency, quota_bytes, ...}]}` |
| GET | `/api/v1/client-auth/plans/:id` | 계획 세부 사항 |
| GET | `/api/v1/client-auth/traffic-packs` | 트래픽 팩 목록 |
| POST | `/api/v1/client-auth/traffic-packs/buy` | 트래픽 팩 구매 |
| GET | `/api/v1/client-auth/orders` | 내 계획 주문(페이지가 매겨져 있음) |
| POST | `/api/v1/client-auth/orders` | 주문 생성(계획 변형 구매/갱신) |
| GET | `/api/v1/client-auth/orders/:id` | 주문 세부정보 |
| GET | `/api/v1/client-auth/invoices` | 송장 |
| GET | `/api/v1/client-auth/invoices/:id` | 송장 세부정보 |

## 8. 잔액/충전

| 방법 | 경로 | 설명 |
|---|---|---|
| GET | `/api/v1/client-auth/balance` | `{data:{client_id, balance(cents), frozen}}` |
| GET | `/api/v1/client-auth/balance/transactions` | 잔액 거래(`type`: 청구/소비/환불, 페이지 매김) |
| GET | `/api/v1/client-auth/charge/sites` | 사용 가능한 충전소(`ListChargeSites`: 활성화된 경우에만 해당) |
| POST | `/api/v1/client-auth/charge/create` | 재충전 주문 생성(제3자 결제로 리디렉션) |
| GET | `/api/v1/client-auth/charge/orders` | 충전 주문 목록 |
| GET | `/api/v1/client-auth/charge/orders/:id` | 충전 주문 세부정보 |
| GET | `/api/v1/client-auth/charge/status/:trade_no` | trade_no별 결제 현황 폴링 |
| POST | `/api/v1/client-auth/charge/redeem-card` | 충전카드 교환하기 |

> 클라이언트를 삭제하면 잔액과 거래가 **계단적으로** 진행됩니다(`internal/service/client/service.go`의 `DeleteClient` 참조).

## 9. 선물/교통 이력

| 방법 | 경로 | 설명 |
|---|---|---|
| GET | `/api/v1/client-auth/gifts` | 청구 가능한 선물(기본 배열; `type`: 일/바이트) |
| POST | `/api/v1/client-auth/gifts/:id/claim` | 청구 |
| GET | `/api/v1/client-auth/traffic-history` | 월별 트래픽 내역: `{data:{history:[{period_start,total_bytes,...}], months}}` |

## 10. 티켓

| 방법 | 경로 | 설명 |
|---|---|---|
| GET | `/api/v1/client-auth/tickets` | 티켓 목록: `{items,total}` |
| POST | `/api/v1/client-auth/tickets` | 티켓 만들기 |
| GET | `/api/v1/client-auth/tickets/:id` | 티켓 세부정보(답글 포함) |
| POST | `/api/v1/client-auth/tickets/:id/reply` | 답장하다 |
| POST | `/api/v1/client-auth/tickets/:id/close` | 닫기 |

## 11. 기사/가이드

| 방법 | 경로 | 설명 |
|---|---|---|
| GET | `/api/v1/client-auth/articles` | 기사 목록(카테고리 필터 포함) |
| GET | `/api/v1/client-auth/articles/:slug` | 기사 세부정보 |

## 12. 추천(기본적으로 비활성화되어 있으며 포털 편집기에서 활성화할 수 있음)

| 방법 | 경로 | 설명 |
|---|---|---|
| GET | `/api/v1/client-auth/referral/code` | 내 추천 코드 및 링크 |
| GET | `/api/v1/client-auth/referral/stats` | 커미션 통계(`commission_rate`, `available_commission`, `min_withdraw_amount`) |
| GET | `/api/v1/client-auth/referral/records` | 추천 기록 |
| GET | `/api/v1/client-auth/referral/withdrawals` | 출금기록 |
| POST | `/api/v1/client-auth/referral/withdraw` | 출금 요청 |

## 13. 공유 계정

| 방법 | 경로 | 설명 |
|---|---|---|
| GET | `/api/v1/client-auth/shared-accounts` | 사용 가능한 공유 계정(스트리밍 등) |
| GET | `/api/v1/client-auth/shared-accounts/:id` | 세부정보 |
| POST | `/api/v1/client-auth/shared-accounts/:id/apply` | 액세스 요청 |
| POST | `/api/v1/client-auth/shared-accounts/:id/repair` | 원클릭 수리 |

## 14. 로그 번들(구독자 진단)

| 방법 | 경로 | 설명 |
|---|---|---|
| GET | `/api/v1/client-auth/logs/types` | 번들로 묶을 수 있는 로그 유형 |
| GET | `/api/v1/client-auth/logs/packages` | 내 로그 번들 |
| POST | `/api/v1/client-auth/logs/package` | 로그 번들링 작업 생성(입력 노드만 해당) |
| GET | `/api/v1/client-auth/logs/packages/:id` | 작업 상태(준비/처리 중/실패) |
| GET | `/api/v1/client-auth/logs/packages/:id/download` | 다운로드(`POST /ticket` 선호 및 `?ticket=` 추가) |
| DELETE | `/api/v1/client-auth/logs/packages/:id` | 로그 번들 삭제 |

## 15. 보안 설정 / IP 허용 목록 / 자체 차단 해제

| 방법 | 경로 | 설명 |
|---|---|---|
| GET/PUT | `/api/v1/client-auth/settings` | 클라이언트 설정: `{ip_whitelist, device_limit}`(PUT) |
| GET | `/api/v1/client-auth/settings/whitelist` | 허용 목록(베어 배열) |
| DELETE | `/api/v1/client-auth/settings/whitelist/:id` | 허용 목록 항목 삭제 |
| POST | `/api/v1/client-auth/settings/auto-add-ip` | 현재 IP를 허용 목록에 추가 |
| POST | `/api/v1/client-auth/self-unban` | 셀프 서비스 금지 해제(정책에 따라 시도가 제한됨) |

> IP 허용 목록은 **구독 URL에만 적용됩니다**(`/sub/:token`). 포털 로그인/OAuth를 차단하지 않습니다.
> 다른 클라이언트 인증 엔드포인트. 사용자에게 허용 목록이 없거나 구독 URL가 없으면 액세스가 항상 허용됩니다. 는
> 허용 목록은 사용자별 설정입니다. 글로벌 스위치가 없습니다.

**자체 차단 해제 흐름**: 관리자 측에서 `auto_quota`(트래픽 초과 자동 차단)에만 사용 가능
`self_unban.enabled`가 켜져 있습니다. 로그인(비밀번호 또는 OAuth 콜백)은 `require_self_unban=true` + `self_unban_token`를 반환합니다.
(일반=self_unban, 유효 기간은 15분, **`POST /self-unban`에서만** 사용 가능, 다른 엔드포인트는 403/`SELF_UNBAN_TOKEN_SCOPE`를 반환)
그러면 포털에 금지 해제 카드가 표시됩니다. 차단 해제에 성공한 후 사용자는 다시 로그인합니다. OAuth 콜백은 이를 다음을 통해 전달합니다.
URL 단편 `#require_self_unban=1&self_unban_token=..&self_unban_hint=..`.

## 16. 오류 코드(발췌)

| 코드 | 시나리오 |
|---|---|
| `INVALID_TOKEN` | 새로 고침 토큰이 잘못됨/만료됨/버전 불일치 |
| `INVALID_REQUEST` | 매개변수 검증 실패 |
| `NOT_FOUND` | 리소스 누락(티켓/주문/허용 목록 항목 등) |
| `RELOAD_FAILED` / `SYNC_FAILED` | (관리자 측) 프록시 구성 동기화 실패 |
| 401 + `errClientTokenRevoked` | token_version 불일치(로그아웃/쫓겨남) → 클라이언트는 새로 고쳐야 합니다. 실패하면 로컬 토큰을 지웁니다. |

## 17. 규정 준수 문서(개인정보 보호정책/서비스 약관)

로그인 카드 하단의 "개인정보 보호정책 · 서비스 약관" 링크(일반 사이트와 동일, 클릭하면 콘텐츠가 열립니다):

- **구성**: 포털 편집기 → 설정 → 일반 → 규정 준수 문서(**언어별 콘텐츠 편집**);
 구조화된 필드 `privacy_policy_url` / `privacy_policy_content` /를 사용하여 `PUT /api/v1/portal/active`를 사용할 수도 있습니다.
 `terms_of_service_url` / `terms_of_service_content` (nil = 변경되지 않음, 콘텐츠는 `{"zh-CN":"...","en":"..."}` 맵임)
- **독립형 액세스 가능 페이지**(서버 렌더링, Google 확인 크롤러에 용이, `?lang=xx` 지원):
  - 메인 패널 도메인: `GET /p/legal/privacy`, `GET /p/legal/terms`
  - 전용 포털 도메인(루트 배포): `GET /legal/privacy`, `GET /legal/terms`
- **디스플레이 로직**: URL 또는 콘텐츠, **URL 승리**(새 탭에서 외부에서 열림) 콘텐츠가 제공되면
 로그인 카드는 현재 포털 언어를 사용하여 대화 상자(`renderMd`)에서 Markdown을 렌더링합니다.
- `/api/v1/public/portal-config`를 통해 런타임에 제공됩니다. 로그인 구성 요소가 소비합니다.
 `ctx.privacyUrl / termsUrl / privacyContent / termsContent / openLegal / hasLegalLinks`.
- Google OAuth 확인을 위해서는 이러한 링크가 공개적으로 액세스 가능해야 합니다. → 위의 독립 실행형 링크를 사용하세요.
 동의 화면; `docs/oauth-google-verification.md`(개인정보 보호정책/약관 템플릿 포함)를 참조하세요.

## 18. 통합 예시

```js
// Sign in
const { data } = await fetch('/api/v1/client-auth/login', {
  method: 'POST', headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ email, password })
}).then(r => r.json());
localStorage.setItem('client_token', data.access_token);
localStorage.setItem('client_refresh_token', data.refresh_token);

// Authenticated request
const me = await fetch('/api/v1/client-auth/me', {
  headers: { Authorization: 'Bearer ' + data.access_token }
}).then(r => r.json());
```

401에서 `POST /refresh`를 한 번 호출하고 다시 시도하세요. 그래도 실패하면 로컬 세션을 지우십시오.
(포털 템플릿의 `api()`는 이미 이를 구현하고 있습니다).
