<!-- source: cert-google-eab.md | locale: ko | updated: 2026-10-23 -->
# Google Public CA 가이드 (EAB 자동 발급)

> 언어: [简体中文](cert-google-eab.md) · [English](cert-google-eab.en.md) · [繁體中文](cert-google-eab.zh-TW.md) · [Русский](cert-google-eab.ru.md) · [日本語](cert-google-eab.ja.md) · 한국어 · [Deutsch](cert-google-eab.de.md) · [Français](cert-google-eab.fr.md) · [Español](cert-google-eab.es.md) · [Português](cert-google-eab.pt.md) · [Türkçe](cert-google-eab.tr.md) · [Bahasa Indonesia](cert-google-eab.id.md) · [ไทย](cert-google-eab.th.md) · [Tiếng Việt](cert-google-eab.vi.md) · [العربية](cert-google-eab.ar.md) · [فارسی](cert-google-eab.fa.md)

> 적용 대상: 패널 v0.12.101+ | 페이지: **인증서 → EAB 구성**
> 코드: `internal/service/certificate/eab_google.go`, `internal/api/handlers/certificates/eab_config.go`

## 1. Google에 EAB가 필요한 이유

| 캘리포니아 | ACME에는 EAB가 필수입니까? | EAB를 얻는 방법 |
|---|---|---|
| 암호화하자 | ❌ 아니요 | — |
| ZeroSSL | ✅ 예 | **이메일을 1개로 교환하세요** (`api.zerossl.com/acme/eab-credentials-email`, acme.sh와 동일); 패널이 자동으로 가져올 수 있습니다. |
| **Google 공용 CA** | ✅ **예** | **GCP 승인으로 생성되어야 함**(이메일로는 충분하지 않음) → 이 페이지의 초점 |

즉, Google **"인증"을 피할 수 없습니다**: 서비스 계정 JSON 파일 또는 임시 액세스 토큰.

## 2. Google의 엄격한 규칙(먼저 읽어보세요)

| 규칙 | 설명 |
|---|---|
| **7일 이내에 사용하세요** | EAB 키는 7일 이내에 사용하지 않으면 **만료**되며 다시 생성해야 합니다. |
| **하나의 계정만** | 하나의 EAB 키는 정확히 하나의 ACME 계정을 등록합니다. 등록 후에는 키가 **무효화**되어 재사용할 수 없습니다. |
| **ACME 계정은 만료되지 않습니다** | 이를 통해 생성된 ACME 계정은 **장수** → **갱신에는 EAB가 필요하지 않습니다** |
| ⚠ **GCP 프로젝트를 삭제하면 모든 계정이 종료됩니다** | 프로젝트를 삭제하면 해당 프로젝트에 속한 **모든 ACME 계정**이 즉시 무효화되고 **갱신이 불가능해집니다** |

> 결론: **EAB는 일회성 "온보딩 자격 증명"입니다. ACME 계정이 지속됩니다.**
> 패널은 `data/acme/accounts/<ca>.json`에 계정을 저장합니다 — **이 디렉토리를 백업합니다**.

## 3. 세 가지 경로(경로 A로 시작 - 메뉴 찾기 없음)

> Cloud Console 진입점이 여러 번 변경되었으므로 직접 링크와 Cloud Shell이 ​​먼저 표시됩니다. 메뉴 경로는 대체 경로입니다.

### 경로 A(가장 쉬움, 먼저 권장): GCP Cloud Shell의 명령어 1개

Cloud Shell은 **gcloud가 사전 설치되어 있고 이미 로그인되어 있는** 브라우저 터미널입니다. 로컬 설정도 없고 메뉴도 없습니다.

1. <https://console.cloud.google.com/cloudshell>(또는 콘솔 오른쪽 상단의 `>_` 아이콘)를 엽니다.
2. 프로젝트를 선택합니다(프로젝트 ID는 상단의 프로젝트 선택기에 표시됩니다).
   ```bash
   gcloud config set project <your-project-id>
   ```
3. 공용 CA API를 활성화합니다(API 라이브러리에서 활성화하는 것과 **동등**).
   ```bash
   gcloud services enable publicca.googleapis.com
   ```
4. EAB를 만듭니다(**필요한 두 문자열입니다**).
   ```bash
   gcloud publicca external-account-keys create
   ```
 출력에서: `keyId` → 패널의 **EAB-KID**; `b64MacKey` → 패널의 **EAB-HMAC-Key**
5. 패널로 돌아가기: **인증서 → EAB 구성 → EAB 구성 추가 → CA: Google 공용 CA** →
 `EAB-KID` / `EAB-HMAC-Key`에 두 값을 입력 → 저장 ✓

> ⚠ 비용: Google EAB 키는 **7일, 일회성** ✗ → 7일 후(또는 계정 전환 시) **4단계를 반복하세요**.
> 이를 방지하려면 경로 B를 사용하세요(서비스 계정을 한 번 저장하고 패널에서 자동으로 다시 요청하도록 함).

### 경로 B(일회성 설정 후 자동): 서비스 계정 키 JSON

**① 공개 CA API를 활성화합니다**(직접 링크가 가장 쉽습니다)
<https://console.cloud.google.com/apis/library/publicca.googleapis.com?project=YOUR_PROJECT_ID> → '활성화'를 클릭하세요.
대체: ☰ → **API 및 서비스 → 라이브러리** → `Public CA` 검색 → 열기 → '활성화'

**② 서비스 계정 만들기**
<https://console.cloud.google.com/iam-admin/serviceaccounts?project=YOUR_PROJECT_ID>
→ '**+ 서비스 계정 생성**' → 원하는 이름(예: `corefusion-eab`) → '만들고 계속하기' → 역할 단계를 **건너뛰**할 수 있음 → '완료'
대체: ☰ → **IAM 및 관리자 → 서비스 계정**

** ③ 최소한의 역할 부여 ** (동일 페이지에서 가장 쉬움)
새로 생성된 계정 → **“권한” 탭** → “접근 권한 부여” → 새 주체: **서비스 계정 이메일** 클릭
→ `publicca`에 대한 역할 검색 → **“Public CA 외부 계정 키 생성자”** 선택 → 저장
대체: ☰ → **IAM 및 관리자 → IAM** → “액세스 권한 부여” → 동일한 이메일 + 역할
⚠ **소유자/편집자를 부여하지 않음** ✗

** ④ JSON 키 다운로드 **
서비스 계정 → 계정 열기 → **“키” 탭** → “**키 추가**” → “**새 키 생성**” → **JSON** → 생성
(브라우저는 `xxx.json`를 다운로드합니다. 또는 `⋮` 행 → "키 관리" → 동일한 흐름)

**⑤ 패널로 돌아가기**: **프로젝트 ID**를 입력하고 JSON의 **전체 내용**을 "서비스 계정 키 파일"에 붙여넣기 → "자동으로 EAB 요청"을 클릭합니다.
→ 이후 패널은 만료되면 **EAB를 자동으로 다시 요청** ✓ 더 이상 GCP 방문이 없습니다 ✓

### 경로 C(일회성 테스트 전용): 액세스 토큰 붙여넣기

Cloud Shell에서:
```bash
gcloud auth print-access-token
```
패널의 "임시 액세스 토큰"과 프로젝트 ID에 출력을 붙여넣고 → "자동으로 EAB 요청"을 클릭합니다.
⚠ 약 1시간 동안 유효하며 **저장되지 않음** ✓ → 확인만 가능, 자동 재요청 불가 ✗

### 진입점을 찾을 수 없는 경우의 치트 시트(콘솔 메뉴 변경)

| 작업 | 직접 링크(가장 안정적임) | 대체: 메뉴 |
|---|---|---|
| 공용 CA API를 활성화합니다. | `https://console.cloud.google.com/apis/library/publicca.googleapis.com?project=<PROJECT_ID>` | ☰ → API 및 서비스 → 라이브러리 → `Public CA` 검색 |
| 서비스 계정 | `https://console.cloud.google.com/iam-admin/serviceaccounts?project=<PROJECT_ID>` | ☰ → IAM 및 관리자 → 서비스 계정 |
| 역할 부여 | `https://console.cloud.google.com/iam-admin/iam?project=<PROJECT_ID>` | ☰ → IAM 및 관리자 → IAM → 액세스 권한 부여 |
| 클라우드 셸 | `https://console.cloud.google.com/cloudshell` | 오른쪽 상단의 `>_` 아이콘 |

> 팁: 페이지로 바로 이동하려면 `<PROJECT_ID>` / `YOUR_PROJECT_ID`를 실제 프로젝트 ID(예: `my-project-123456`)로 바꾸세요.

## 4. 패널 구성(두 가지 방법)

**인증서 → EAB 구성 → EAB 구성 추가**로 이동하여 **Google 공용 CA**를 선택합니다.

### 옵션 A(권장): 서비스 계정 JSON 붙여넣기

| 필드 | 가치 |
|---|---|
| GCP 프로젝트 ID | 섹션 3의 프로젝트 ID입니다. 예: `my-project-123456` |
| 서비스 계정 JSON | `corefusion-eab.json`의 **전체 내용**을 붙여넣으세요. |
| 임시 액세스 토큰 | 비워두세요 |
| 이름 | `Google Public CA - <project-id>`로 자동 생성됩니다. 편집 가능 |

**자동으로 EAB 요청**을 클릭하고 → 성공하면 KID/HMAC 키가 채워지고 저장됩니다. 나중에 발급할 때 `CA = Google Public CA`를 선택하면 됩니다(EAB는 CA별로 연결되어 있으며 수동으로 선택할 수 없음).

> 서비스 계정 JSON는 **패널 데이터베이스의 EAB 구성으로 암호화됩니다**(`json:"-"`, **프런트엔드로 반환되지 않음**).
> 수동으로 GCP를 방문하는 대신 **7일 만료 후 자동으로 키를 다시 요청**하는 것이 목적입니다.

### 옵션 B: 임시 액세스 토큰(일회성)

```bash
gcloud auth print-access-token        # valid ~1 hour
```

“임시 액세스 토큰”에 붙여넣고 프로젝트 ID를 입력한 후 **자동으로 EAB 요청**을 클릭하세요.
⚠ 이 토큰은 **저장되지 않으며** 만료 후 다시 획득해야 합니다. → 장기간 사용이 아닌 일회성 테스트용으로 괜찮습니다.

### 해당 명령줄(선택 사항, 비교용)

```bash
gcloud publicca external-account-keys create
# returns keyId / b64MacKey, corresponding to ACME's --eab-kid / --eab-hmac-key
```

## 4.5 왜 "단일 키" 대신 JSON를 사용합니까?

이것은 가장 일반적인 질문입니다. 두 종류의 "키"를 구분합니다.

| 당신이 기대할 수있는 것 | 실제로 필요한 것 |
|---|---|
| **API 키** (`AIza…` 문자열) | ❌ **쓸모 없음** — API 키는 공개 식별자일 뿐이며 **신원을 전달하지 않습니다**. EAB를 생성하려면 **IAM 권한**이 필요하므로 ID 확인 자격 증명이 필요합니다. |
| — | ✅ **서비스 계정 키**: GCP에서 다운로드한 **파일** "IAM → 서비스 계정 → 키 → 새 키 생성 → **JSON**" |

요점: **JSON 파일 자체는 '키'입니다** — GCP는 계정 ID(`client_email`)와 개인 키(`private_key`)를 하나의 파일로 패키지합니다. 패널은 JWT에 서명하고 → OAuth2 액세스 토큰을 교환하고 → 공용 CA API를 호출합니다.
따라서 "JSON 붙여넣기" ≠ 많은 필드를 채우십시오. **전체 파일을 붙여넣습니다**(패널은 `type` / `token_uri`와 같은 선택적 필드도 채웁니다).

**실제 EAB는 요청의 **결과**인 두 개의 문자열**(`EAB-KID` + `EAB-HMAC-Key`)입니다. 패널에서 자동으로 채워집니다.

### 파일을 넘겨주고 싶지 않으신가요? 두 가지 대안

| 방법 | 어떻게 | 비용 |
|---|---|---|
| **임시 액세스 토큰** | `gcloud auth print-access-token` → '임시 액세스 토큰'에 붙여넣기 | ~1시간 유효, **저장되지 않음** → 일회성만 가능, 자동 재요청 없음 |
| **수동 KID/HMAC** | 로컬로 `gcloud publicca external-account-keys create` 실행 → “EAB-KID / EAB-HMAC-Key”에 `keyId` / `b64MacKey` 입력 | 자격 증명이 전달되지 않았습니다 ✓; 그러나 Google EAB는 **7일, 일회성** → 만료 후 반복 |

> 패널은 한 가지 이유로 서비스 계정 JSON를 권장합니다. **EAB는 7일 후에 만료되며** 저장된 서비스 계정만 수동 작업 없이 **자동 재요청**을 허용합니다.

## 4.6 FAQ: API 키를 사용할 수 있나요?

**아니요.** 이는 구현 선택이 아닙니다. **Google는 이를 명시적으로 거부합니다**(두 가지 방법으로 확인됨).

```bash
# 1) Query parameter
curl -X POST "https://publicca.googleapis.com/v1/projects/<ID>/locations/global/externalAccountKeys?key=AIza..." -d '{}'
# 2) x-goog-api-key header recommended by the official docs
curl -X POST "https://publicca.googleapis.com/v1/projects/<ID>/locations/global/externalAccountKeys" \
     -H "x-goog-api-key: AIza..." -d '{}'
```

둘 다 다음을 반환합니다.

```json
{"error":{"code":401,"status":"UNAUTHENTICATED",
  "message":"API keys are not supported by this API. Expected OAuth2 access token
             or other authentication credentials that assert a principal."}}
```

비교를 위해 **(가짜) OAuth Bearer**는 다음을 반환합니다.
`Request had invalid authentication credentials. Expected OAuth 2 access token...` —
따라서 OAuth는 인식되지만 **API 키는 완전히 거부됩니다** ✗.

**이유**: EAB를 만드는 것은 "당신이 누구인지, `roles/publicca.externalAccountKeyCreator`가 있는지 여부"를 알아야 하는 **관리 작업**입니다. ✗
— API 키는 프로젝트/애플리케이션만 식별하며 ✓ **주요 ID는 표현하지 않습니다** ✓.
(Google의 "API 키 관리 모범 사례"에서는 관리 시나리오에 **IAM + 단기 서비스 계정 자격 증명**을 권장합니다 ✓ — 여기에서 사용된 것과 동일한 접근 방식 ✓.)

**따라서 세 가지 경로만 있습니다** ✓ (§3 참조): 서비스 계정 JSON ✓ / 임시 액세스 토큰 ✓ /
또는 **자격 증명이 전혀 없음** — Cloud Shell에서 `gcloud publicca external-account-keys create`를 실행하세요 ✓
패널에 `keyId` / `b64MacKey`를 입력하세요 ✓ (**GCP IAM이 사용자를 차단하는 경우 사용** ✓).

## 5. 자동 등록 및 갱신(중요)

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

따라서:

- **첫 번째 발급은 자동으로 등록됩니다**; 수동 GCP 단계가 필요하지 않습니다.
- 갱신 시에는 EAB를 사용하지 **않습니다**(계정 파일이 존재하고 데이터 디렉터리가 안정적으로 유지되는 한).
- ⚠ 에이전트 측 ACME 데이터 디렉터리는 고정된 절대 경로(기본값 `/var/lib/corefusion-agent/acme`)입니다.
 **함부로 변경하지 마세요** 그렇지 않으면 새 계정으로 처리됩니다 → 다른 EAB 키가 필요합니다.

## 6. 문제 해결

| 증상/오류 | 원인 | 수정 |
|---|---|---|
| `PERMISSION_DENIED` (403) | 서비스 계정에 `roles/publicca.externalAccountKeyCreator`가 없습니다. | 섹션 3으로 돌아가기 ④ |
| `SERVICE_DISABLED` (403) | 프로젝트에 `publicca.googleapis.com`가 활성화되지 않았습니다. | 제3항 ② |
| `NOT_FOUND` (404) | 잘못된 프로젝트 ID/프로젝트가 삭제되었습니다. | 프로젝트 ID 확인 |
| 발행 보고서 `externalAccountRequired` | 이 CA에는 EAB가 없습니다. | 먼저 패널에서 EAB를 요청/구성하세요. |
| 발행 보고서 `Accounts must have at least one contact.` | 등록 시 이메일이 제공되지 않았습니다. | 인증서 페이지에서 ACME 계정 이메일을 설정하세요(패널은 기본 계정을 사용함) |
| 갱신 시 EAB 다시 요청/등록 실패 | ACME 계정 파일 손실(데이터 디렉터리 변경/청소) | **백업을 복원**; 하나도 없으면 서비스 계정을 사용하여 새 EAB를 요청하세요. |
| 발행자가 Google에서 Let's Encrypt로 변경되었습니다. | CA가 누락된 갱신 명령(이전 버그, v0.12.101에서 수정됨) | 업그레이드 및 재발행 |

## 7. 보안 및 운영 권장 사항

1. **최소 권한**: 서비스 계정에는 소유자/편집자가 아닌 `roles/publicca.externalAccountKeyCreator`만 부여합니다.
2. **프로젝트를 삭제하지 마세요**: GCP 프로젝트를 삭제하면 모든 ACME 계정이 무효화되고 → 인증서를 갱신할 수 없습니다.
3. **항상 백업** `data/acme/accounts/`(계정 ID 자체) 및 패널 데이터베이스의 EAB 구성을 백업하세요.
4. **EAB 만료에 대해 당황하지 마십시오**: ACME 계정 파일이 존재하는 한 갱신은 영향을 받지 않습니다. 새로운 EAB는 새 계정을 등록하는 데만 필요합니다.
5. 서비스 계정 키가 유출된 경우: GCP에서 해당 키를 삭제하고 새 JSON를 패널에 붙여넣습니다.

## 8. 관련 엔드포인트(스크립팅용)

| 엔드포인트 | 설명 |
|---|---|
| `POST /api/v1/eab-configs/fetch-google` | GCP 인증을 사용하여 EAB를 만듭니다. 본체: `{project_id, service_account_json?, access_token?}` |
| `POST /api/v1/eab-configs/fetch-zerossl` | 이메일로 ZeroSSL EAB를 받으세요. 본체: `{email}` |
| `GET  /api/v1/eab-configs` | EAB 구성 나열(HMAC 키 또는 서비스 계정 JSON가 **없음**, `has_mac_key` / `has_google_sa`만 해당) |
