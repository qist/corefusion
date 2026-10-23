<!-- source: subscription.md | locale: ko | updated: 2026-10-23 -->
# 구독 API(공개)

> 언어: [简体中文](subscription.md) · [English](subscription.en.md) · [繁體中文](subscription.zh-TW.md) · [Русский](subscription.ru.md) · [日本語](subscription.ja.md) · 한국어 · [Deutsch](subscription.de.md) · [Français](subscription.fr.md) · [Español](subscription.es.md) · [Português](subscription.pt.md) · [Türkçe](subscription.tr.md) · [Bahasa Indonesia](subscription.id.md) · [ไทย](subscription.th.md) · [Tiếng Việt](subscription.vi.md) · [العربية](subscription.ar.md) · [فارسی](subscription.fa.md)

사용자의 구독 노드 목록을 가져옵니다. **최종 사용자가 가장 많이 사용하는 엔드포인트입니다**; 헤더가 필요하지 않습니다. 액세스는 URL의 토큰에 의해 부여됩니다.

## 1. 엔드포인트

| 배포 | URL |
|---|---|
| 메인 패널 | `GET https://<panel-domain>/sub/<token>` |
| 전용 구독 포트 | `GET https://<sub-domain>:<port>/<token>`(`/sub` 접두사 없음) |

`<token>`는 관리자 측에서 생성된 구독의 고유 액세스 토큰입니다(포털의 "내 구독" 아래에 표시됨).

## 2. 쿼리 매개변수

| 매개변수 | 설명 |
|---|---|
| `format` | 출력 형식; 구독 기본값을 재정의합니다. 아래를 참조하세요 |
| `config` | 구성 템플릿 이름(관리 UI의 "구독 형식"에서 정의됨) 이 요청의 `format`와 동일한 형식을 사용해야 합니다. 그렇지 않으면 무시됩니다. |
| `platform` | 플랫폼 이름으로 구성 템플릿을 일치시킵니다(`config`가 생략된 경우 사용됨). 형식도 일치해야 합니다. |

### 지원되는 `format` 값

| 가치 | 출력 | 클라이언트 |
|---|---|---|
| `base64_uri`(기본값) | 전체적으로 base64로 인코딩된 뉴라인 조인 공유 링크 | V2RayN, Shadowrocket, Surge 및 기타 일반 클라이언트 |
| `raw_uri` | 줄바꿈으로 연결된 공유 링크, 일반 텍스트 | 일반 링크/디버깅이 필요한 클라이언트 |
| `mihomo_yaml` | 전체 Clash/Mihomo YAML 구성(프록시 그룹, 규칙 포함) | Clash.Meta / mihomo / Clash Verge Rev |
| `mihomo_proxies_yaml` | `proxies:` 노드 목록만 있는 YAML | mihomo 프록시 공급자 소스(예약된 새로 고침 포함) |
| `singbox_json` | 전체 sing-box JSON 구성 | sing-box/SFI/FLClash(sing-box 코어) |
| `xray_json` | 전체 Xray JSON 구성 | Xray 코어 |

> 프로토콜/형식 호환성이 적용됩니다. 예를 들어 `anytls`는 sing-box/Mihomo 형식에만 나타납니다. sing-box 서버의 REALITY 노드는 Xray/sing-box JSON 구독에 표시되지 않습니다. 클라이언트가 인식할 수 없는 노드는 전체 구독이 실패하는 대신 자동으로 삭제됩니다.

## 3. 응답 헤더

| 헤더 | 예 | 설명 |
|---|---|---|
| `Subscription-Userinfo` | `upload=1024; download=2048; total=1073741824; expire=1760000000` | 사용량 및 만료(바이트/Unix 초). **403으로도 전송됨** 고객이 남은 허용량을 표시할 수 있음 |
| `Profile-Update-Interval` | `6` | 제안된 새로 고침 간격(시간) |
| `Profile-Title` | `CoreFusion` | 프로필 이름(Clash Verge Rev/V2RayN 등으로 표시) |
| `Profile-Web-Page-Url` | `https://…/portal` | 포털 페이지 링크(클라이언트에서 "웹 페이지 열기") |
| `Content-Type` | 형식별: `text/plain` / `text/yaml` / `application/json` |  |
| `Content-Disposition` | `attachment; filename="subscription.txt"` |  |

## 4. 상태 코드

| HTTP | 코드 | 의미 |
|---|---|---|
| 200 | — | 일반(본문은 `format`에서 렌더링된 구독입니다) |
| 400 | `MISSING_TOKEN` | URL에 토큰이 없습니다. |
| 403 | (일반 텍스트) | 구독이 일시 중지/만료/비활성화되었습니다. 메시지는 `Accept-Language`를 통해 현지화(zh/en/ja/ko)되었으며 한 줄 이유입니다. |
| 404 | `SUBSCRIPTION_NOT_FOUND` | 토큰이 존재하지 않습니다 |

> 취소된 구독은 존재하지 않는 구독으로 처리됩니다(404).

## 5. 예

```bash
# 1) Fetch with the default format
curl -L "https://panel.example.com/sub/abcd1234efgh5678"

# 2) Request Mihomo YAML with the "clash-global" config template
curl -L "https://panel.example.com/sub/abcd1234efgh5678?format=mihomo_yaml&config=clash-global"

# 3) Node list only (for a mihomo proxy-provider)
curl -L "https://panel.example.com/sub/abcd1234efgh5678?format=mihomo_proxies_yaml"

# 4) Check usage (headers only)
curl -sI "https://panel.example.com/sub/abcd1234efgh5678" | grep -i subscription-userinfo
```

Clash/Mihomo 동적 구독(정기적 새로 고침):

```yaml
proxy-providers:
  subscription-nodes:
    type: http
    url: "https://panel.example.com/sub/abcd1234efgh5678?format=mihomo_proxies_yaml"
    interval: 3600
    health-check:
      enable: true
      url: https://www.gstatic.com/generate_204
      interval: 300

proxy-groups:
  - name: PROXY
    type: select
    use:
      - subscription-nodes
```

## 6. 메모

- **형식과 구성은 밀접하게 결합되어 있습니다**: `config`/`platform`에서 선택한 템플릿은 이 요청의 `format`와 일치해야 합니다. 일치하지 않는 템플릿은 무시됩니다. 이렇게 하면 sing-box/Xray JSON에 Mihomo 규칙을 삽입하는 것을 방지할 수 있습니다.
- **속도 제한**: 구독 엔드포인트에는 자체 제한(`SubscribeRateLimit`)이 있습니다. 악의적인 클라이언트는 거부됩니다.
- **액세스 로그**: ​​가져올 때마다 관리 UI의 '구독 액세스 로그' 아래에 표시되는 IP / UA / CDN 정보를 기록합니다.
- 전용 구독 포트에 도메인이 구성되어 있지 않으면 요청의 `X-Forwarded-Proto` / `X-Forwarded-Host`가 사용됩니다. 역방향 프록시가 두 헤더를 모두 전달하는지 확인하세요.
