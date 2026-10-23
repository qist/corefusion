<!-- source: oauth-google-verification.md | locale: ko | updated: 2026-10-23 -->
# Google OAuth 검증 가이드

> 언어: [简体中文](oauth-google-verification.md) · [English](oauth-google-verification.en.md) · [繁體中文](oauth-google-verification.zh-TW.md) · [Русский](oauth-google-verification.ru.md) · [日本語](oauth-google-verification.ja.md) · 한국어 · [Deutsch](oauth-google-verification.de.md) · [Français](oauth-google-verification.fr.md) · [Español](oauth-google-verification.es.md) · [Português](oauth-google-verification.pt.md) · [Türkçe](oauth-google-verification.tr.md) · [Bahasa Indonesia](oauth-google-verification.id.md) · [ไทย](oauth-google-verification.th.md) · [Tiếng Việt](oauth-google-verification.vi.md) · [العربية](oauth-google-verification.ar.md) · [فارسی](oauth-google-verification.fa.md)

> 범위: CoreFusion 포털에 대한 Google 로그인(`internal/service/oauth`, 범위: `openid`, `email`, `profile`).
> 마지막에 GitHub 비교(GitHub에는 확인 흐름이 없습니다).

## 1. 테스트 모드와 프로덕션 모드

Google OAuth 동의 화면에는 두 가지 게시 상태가 있습니다.

| 상태 | 로그인할 수 있는 사람 | 확인 필요 |
|---|---|---|
| **테스트** | "테스트 사용자" 목록에는 Google 계정만 있습니다. | 아니요 |
| **생산 중** | 모든 Google 계정 | **예**(아래 참조) |

"시스템 설정 → OAuth"가 활성화되면 포털 로그인 페이지에 모든 방문자에게 Google 버튼이 표시됩니다.
앱이 아직 테스트 중인 경우 테스트 사용자 목록에 없는 방문자는
`Error 403: access_denied`(앱이 확인되지 않음/테스트 사용자가 아닙니다).
Google 로그인을 공개적으로 제공하려면 **앱을 프로덕션으로 전환하고 인증을 통과**하세요.

## 2. 검증을 위한 엄격한 요구 사항

Google의 검토(제한되지 않은 범위에 대한 기본 검증)에는 다음이 필요합니다.

1. **홈 페이지**: 승인된 도메인(동일 도메인 또는 하위 도메인)에서 공개적으로 액세스 가능한 URL입니다.
2. **개인정보 보호정책 링크**: **공개적으로 액세스 가능**(로그인 없음)해야 하며 수집하는 데이터와 이를 사용/저장/삭제하는 방법을 설명해야 합니다. Google 로그인의 경우 최소한 다음을 포함해야 합니다.
 Google 계정 데이터 수집(이메일, 이름, 아바타), 목적(포털 계정 생성/로그인), 제3자에게 판매 금지,
 사용자가 자신의 계정과 데이터를 삭제하는 방법.
3. **서비스 약관 링크**: 공개적으로 액세스할 수 있으며 서비스 규칙을 설명합니다.
4. **승인된 도메인**: 동의 화면에서 앱의 도메인을 등록합니다(예: `example.com`). home/privacy/terms 링크는 해당 도메인이나 하위 도메인에 있어야 합니다.
5. **앱 로고(선택 사항이지만 권장됨)**: 120×120px.
6. **범위 정당화**: 이 프로젝트는 `openid / email / profile`(비제한, 비민감)만 사용합니다.
 이는 기본 검증에 해당합니다. 범위별 정당성이 없고 **제3자 보안 평가가 없습니다**.
 (전체 Gmail 액세스와 같은 제한된 범위에는 이러한 기능이 필요하며 이 프로젝트에서는 이를 사용하지 않습니다.)

확인 항목: [Google Cloud Console](https://console.cloud.google.com/) →
**API 및 서비스 → OAuth 동의 화면 → 앱 게시 → 확인을 위해 제출**.
기본 확인은 일반적으로 며칠 내에 완료됩니다. 그 동안에는 테스트 모드로 작동할 수 있습니다.

## 3. 이 프로젝트에 대해 구성할 항목

### 3.1 승인된 리디렉션 URI

콜백은 로그인을 시작하는 도메인에서 동적으로 생성됩니다(`internal/api/handlers/oauth/handler.go`의 `Authorize` 참조).

```
https://<portal-domain>/api/v1/client-auth/oauth/google/callback
```

- 포털 도메인(예: `https://p.example.com`)에서 로그인 시작 → `https://p.example.com/...` 등록;
- 패널 도메인 `/p`에서 시작 → `https://panel.example.com/...`를 등록합니다.
- Google는 **다중** 리디렉션 URI를 허용합니다. 두 도메인을 모두 등록하세요.
- ⚠ Google에서는 리디렉션 URI가 **https**(`http://localhost` 제외)여야 합니다.
 역방향 프록시는 `X-Forwarded-Host` / `X-Forwarded-Proto`를 전달해야 합니다. 그렇지 않으면 패널이
 http 또는 내부 리디렉션_uri → Google가 이를 거부합니다(`redirect_uri_mismatch`).

“시스템 설정 → OAuth → 포털 콜백 도메인”을 설정한 후 페이지에 표시된 콜백 URL가 Google에 붙여넣는 콜백입니다.

### 3.2 개인정보 보호정책/포털의 약관 링크

로그인 카드에는 '개인정보 보호정책 · 서비스 약관' 링크(구성된 후에 표시됨)가 포함되어 있으며 두 문서 모두 **독립형 페이지**를 가지고 있습니다.
(대화상자와 독립형 페이지는 동일한 콘텐츠를 공유합니다):

- 메인 패널 도메인: `https://<panel-domain>/p/legal/privacy`, `/p/legal/terms`
- 전용 포털 도메인(루트 배포): `https://<portal-domain>/legal/privacy`, `/legal/terms`
- `?lang=xx`를 지원합니다. 그렇지 않으면 브라우저의 `Accept-Language`가 사용되고 포털 기본값이 사용됩니다.
- 페이지는 **서버 렌더링**(JS 없음)이므로 Google의 확인 크롤러에 도움이 됩니다.

**구성(어느 쪽이든, 링크 승리)**:

1. **콘텐츠 모드(권장, 즉시 사용 가능)**: 포털 편집기 → 설정 → 일반 → 규정 준수 문서 →
 "언어 편집"을 선택하고 §4 템플릿을 "개인정보 보호 정책 콘텐츠" / "서비스 약관 콘텐츠"에 붙여넣습니다.
 (**마크다운, 언어별로 편집됨**) → 저장 후 로그인 카드에 **대화상자**를 여는 링크가 표시됩니다.
 편집자의 “독립형 링크”는 Google 동의 화면에서 직접 입력할 수 있습니다.
2. **외부 링크 모드**: "개인정보 보호정책 URL" / "서비스 약관 URL"(귀하가 호스팅하는 페이지를 가리킴) 입력 → 새 탭에서 열립니다.

콘텐츠와 링크는 `/api/v1/public/portal-config`(`privacy_policy_content` /
`terms_of_service_content`는 **언어별 마크다운 맵** `{"zh-CN":"...","en":"..."}`입니다.
`privacy_policy_url` / `terms_of_service_url`는 선택적 외부 링크입니다. **로그인하지 않고도 액세스할 수 있어야 합니다** ✓.

### 3.3 패널측 스위치

`System settings → OAuth`: Google를 활성화하고 클라이언트 ID/클라이언트 비밀번호를 입력합니다(비밀은 저장 시 AES-256-GCM으로 암호화됩니다).
Google 버튼은 모든 것이 구성된 경우에만 포털 로그인 페이지에 나타납니다(`/api/v1/client-auth/oauth/providers` 반환 활성화).

## 4. 개인정보 보호정책/약관 템플릿

> 조정할 수 있는 최소한의 규정 준수 템플릿. 사용 전: `【】` 자리 표시자를 교체하고, 비즈니스에 대한 조항을 추가/제거하고,
> 운영회사가 이를 검토하게 합니다. 이 템플릿은 법적 조언이 아닙니다.

### 4.1 개인정보 보호정책(템플릿)

```markdown
# Privacy Policy

Last updated: [DATE]

[OPERATOR NAME] ("we") operates [PORTAL NAME] ("the Service"). We respect your privacy;
this policy explains what information we collect and how we use it.

## 1. Information we collect
- **Account information**: the email address and password you provide at registration (stored only as a one-way hash);
- **Google sign-in information**: when you choose "Sign in with Google", we receive, with your authorization,
  your Google account email, display name and avatar URL (scopes: openid, email, profile);
- **Service data**: subscription configuration, traffic usage, device IP allow-list, tickets and order records.

## 2. How we use information
Only to: create and sign in to your account, provide the proxy subscription service, bill you and provide support,
and for security controls (sign-in protection, IP allow-list). We **do not** sell your personal information.

## 3. Third-party services
- **Google**: receives account information within the authorized scope only when you actively use "Sign in with Google";
- Payment providers: receive order information only when you initiate a payment.

## 4. Storage and security
Data is stored on our servers; credentials are encrypted at rest and all traffic uses TLS.

## 5. Your rights
You can update your profile/password and end sessions in the portal at any time; to delete your account and all data,
contact [CONTACT EMAIL] and we will process the request within [N] business days.

## 6. Policy changes
Material changes will be announced in the portal notice board.
```

### 4.2 서비스 약관(템플릿)

```markdown
# Terms of Service

Last updated: [DATE]

## 1. Service
The Service provides [PROXY / NETWORK ACCESS] to registered users; specific plans are described on the store page.

## 2. Accounts
- You are responsible for the security of your account and password; for accounts created with Google sign-in, credentials are managed by Google;
- You may not lend or transfer your account, or use the Service for any unlawful purpose (including but not limited to:
  unauthorized access to others' systems, distributing illegal content, or circumventing mandatory legal requirements).

## 3. Payment and refunds
Plans are billed [monthly / by usage]; traffic and device limits follow the plan description;
[REFUND POLICY: e.g. full refund within 7 days if unused].

## 4. Changes and termination
We may suspend or terminate the Service due to force majeure or your breach of these terms; deleting an account deletes the related data.

## 5. Disclaimer
The Service is provided "as is"; to the extent permitted by law, we are not liable for indirect losses caused by use of the Service.

## 6. Miscellaneous
These terms are governed by the laws of [JURISDICTION]. Contact: [CONTACT EMAIL].
```

## 5. 출시 전 체크리스트

- [ ] OAuth 동의 화면: 앱 이름/로고/홈 페이지/개인정보 보호정책/약관 모두 존재합니다. 도메인 등록됨
- [ ] 개인정보 보호정책 및 약관 페이지 **로그인 없이 액세스 가능**(포털 사용자 정의 페이지 또는 독립형 정적 페이지)
- [ ] 포털 로그인 카드/바닥글에 두 링크가 모두 표시됩니다.
- [ ] 리디렉션 URI는 실제 도메인과 일치하며 https입니다.
- [ ] 앱 게시 → 확인을 위해 제출; 승인 후 테스트 사용자 제한이 해제됩니다.
- [ ] 패널 OAuth 설정: 클라이언트 ID/비밀번호가 채워진 상태에서 활성화=true(제공자 엔드포인트 반환 활성화:true)

## 6. GitHub 비교

GitHub OAuth 앱에는 게시 상태 또는 확인 흐름이 **없음** 있습니다. 앱을 만들고 콜백 URL를 설정하면 공개적으로 작동합니다.
GitHub OAuth 앱은 **하나** 인증 콜백 URL(접두사 일치)만 허용합니다. 여러 도메인의 경우
Google의 다중 URI 접근 방식을 사용하거나 포털 도메인별로 별도의 앱을 만듭니다.
