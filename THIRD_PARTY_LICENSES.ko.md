<!-- source: THIRD_PARTY_LICENSES.md | locale: ko | updated: 2026-11-19 -->
# 제3자 구성요소 및 라이센스

> 언어: [简体中文](THIRD_PARTY_LICENSES.md) · [English](THIRD_PARTY_LICENSES.en.md) · [繁體中文](THIRD_PARTY_LICENSES.zh-TW.md) · [Русский](THIRD_PARTY_LICENSES.ru.md) · [日本語](THIRD_PARTY_LICENSES.ja.md) · 한국어 · [Deutsch](THIRD_PARTY_LICENSES.de.md) · [Français](THIRD_PARTY_LICENSES.fr.md) · [Español](THIRD_PARTY_LICENSES.es.md) · [Português](THIRD_PARTY_LICENSES.pt.md) · [Türkçe](THIRD_PARTY_LICENSES.tr.md) · [Bahasa Indonesia](THIRD_PARTY_LICENSES.id.md) · [ไทย](THIRD_PARTY_LICENSES.th.md) · [Tiếng Việt](THIRD_PARTY_LICENSES.vi.md) · [العربية](THIRD_PARTY_LICENSES.ar.md) · [فارسی](THIRD_PARTY_LICENSES.fa.md)

CoreFusion는 다음 오픈 소스 구성 요소를 사용합니다. 이 파일에는 주요 구성요소와 라이센스가 나열되어 있습니다. **전체 목록은 `go.mod` / `go.sum`, `web/admin/package.json` 및 각 모듈의 자체 라이센스 파일에 의해 정의됩니다**.

이 프로젝트의 바이너리를 배포할 때 이 파일, 프로젝트 `LICENSE` 및 모든 타사 구성 요소의 저작권/라이센스 고지를 보관하세요.

## 1. 바이너리로 컴파일된 Go 종속성(기본)

| 구성 요소 | 라이센스 | 목적 |
|---|---|---|
| `github.com/xtls/xray-core` | MPL-2.0 | Xray 구성 구문 분석, gRPC 명령, 프로토콜 정의 |
| `github.com/xtls/reality` | MPL-2.0 | REALITY 프로토콜 지원 |
| `github.com/sagernet/sing` | **GPL-3.0-or-later** | xray-core를 통해 간접적으로 연결됨 |
| `github.com/sagernet/sing-shadowsocks` | **GPL-3.0-or-later** | xray-core(Shadowsocks-2022)를 통해 간접적으로 연결됨 |
| `github.com/go-sql-driver/mysql` | MPL-2.0 | MySQL 드라이버 |
| `github.com/hashicorp/golang-lru/v2` | MPL-2.0 | LRU 캐시 |
| `github.com/go-acme/lego/v5` | MIT | ACME 인증서 발급 |
| `github.com/gin-gonic/gin` | MIT | HTTP 프레임워크 |
| `gorm.io/gorm` | MIT | ORM |
| `gorm.io/driver/mysql` | MIT | GORM용 MySQL 드라이버 |
| `gorm.io/driver/postgres` | MIT | GORM용 PostgreSQL 드라이버 |
| `github.com/jackc/pgx/v5` | MIT | PostgreSQL 드라이버 |
| `github.com/qist/sqlite` | MIT | SQLite 드라이버(패널/에이전트용 기본 데이터베이스) |
| `github.com/aws/aws-sdk-go-v2` | Apache-2.0 | S3 백업, Route53 DNS-01 |
| `google.golang.org/grpc` | Apache-2.0 | 에이전트 gRPC 통신 |
| `google.golang.org/protobuf` | BSD-3-Clause | 프로토부프 런타임 |
| `golang.org/x/crypto` | BSD-3-Clause | 암호화 기본 요소 |
| `golang.org/x/oauth2` | BSD-3-Clause | OAuth2 / Google EAB |
| `golang.org/x/sys` | BSD-3-Clause | 시스템 호출(nft/tc 속도 제한, 프로세스 관리) |
| `github.com/golang-jwt/jwt/v5` | MIT | JWT |
| `github.com/google/uuid` | BSD-3-Clause | UUID |
| `github.com/go-webauthn/webauthn` | BSD-3-Clause | Passkey / WebAuthn 로그인 |
| `github.com/pquerna/otp` | Apache-2.0 | TOTP 이중 인증 |
| `github.com/jlaffaye/ftp` | ISC | FTP 백업 업로드 |
| `gopkg.in/yaml.v3` | MIT / Apache-2.0 | YAML 구성 구문 분석 |

> ⚠ **GPL-3.0 구성 요소**: `sing` 및 `sing-shadowsocks`는 GPL-3.0 이상입니다. 이는 패널 및 에이전트 바이너리에 정적으로 연결되므로 **바이너리를 배포하려면 해당 소스를 제공해야 합니다**([LEGAL.md](./LEGAL.md)의 섹션 3 참조).

## 2. 프런트엔드 종속성(패널 바이너리로 컴파일됨)

관리 UI는 Vite로 구축되고 `go:embed`를 통해 패널 바이너리에 내장되므로 다음 구성 요소도 바이너리와 함께 배포됩니다.

| 구성 요소 | 라이센스 | 목적 |
|---|---|---|
| Vue 3 / Vue Router / Pinia / Element Plus / axios / vue-i18n / marked / js-yaml / qrcode / Monaco Editor | MIT | 관리 UI, 라우팅 및 상태, i18n, 마크다운 렌더링, QR 코드 및 코드 편집기 |
| DOMPurify | Apache-2.0 / MPL-2.0 | Markdown/HTML을 렌더링하기 전 XSS 삭제 |

## 3. 런타임 시 패널에서 다운로드/호출되는 독립 실행형 프로그램

코어를 설치하거나 업그레이드할 때 패널/에이전트는 GitHub에서 다음 바이너리를 다운로드하여 별도의 프로세스로 실행합니다. **"이 프로젝트에 의해 구축됨"이라고 표시된 아티팩트는 이 조직이 업스트림 소스**(`qist/*-binaries`)에서 컴파일 및 게시하고 제품과 함께 배포되므로 해당 라이센스 의무(GPL에 대한 소스 가용성 포함)는 이 프로젝트에 있습니다. 나머지는 공식 업스트림 릴리스의 다운로드 URL입니다.

| 프로그램 | 라이센스 | 메모 |
|---|---|---|
| Xray-core | MPL-2.0 | 프록시 코어, 별도의 프로세스; 공식 업스트림 릴리스(`XTLS/Xray-core`) |
| sing-box | **GPL-3.0-or-later** | 프록시 코어, 별도의 프로세스; **이 프로젝트에 의해 구축됨** (`qist/singbox-binaries`; 공식 릴리스에는 사용자별 트래픽 계산에 필요한 `with_v2ray_api`가 없음) 배포시 해당 소스 제공 |
| Caddy | Apache-2.0 | 역방향 프록시 / TLS; **이 프로젝트로 제작됨** (`qist/caddy-binaries`) |
| Nginx | BSD-2-Clause | 역방향 프록시; **이 프로젝트에 의해 구축됨** (`qist/nginx-binaries`, glibc / musl로 분할됨) |

> Hysteria2 및 TUIC는 독립형 코어나 바이너리가 아닌 **sing-box에서 실행되는 프로토콜**이므로 별도로 나열되지 않습니다.

## 4. 데이터 파일

| 데이터 | 소스/라이센스 |
|---|---|
| `geoip.dat` / `geosite.dat` | 데이터 소스 저장소(일반적으로 GPL-3.0 또는 CC-BY-SA)에서 선언한 대로 |
| GeoLite2(`GeoLite2-ASN.mmdb` 등) | MaxMind GeoLite2 EULA / CC BY-SA 4.0; 상업적으로 사용하기 전에 라이센스 확인 |
| `country.mmdb` | 데이터 소스에서 선언한 대로 |

## 5. 규정 준수 권장 사항

- 바이너리를 제공할 때 다음을 포함합니다: `LICENSE`, 이 파일, 그리고 각 GPL/MPL 구성 요소의 라이센스 텍스트 또는 취득 URL.
- 이 프로젝트에서 빌드한 바이너리(sing-box / Caddy / Nginx)는 패널과 함께 제공됩니다. 업스트림 소스에서 컴파일되므로 빌드 방법(버전, 빌드 태그, 구성 플래그)을 게시합니다. GPL 구성 요소(sing-box)에는 해당 버전의 완전한 해당 소스 또는 효과적인 획득 경로가 추가로 필요합니다.
- `xray-core`와 같은 MPL-2.0 구성 요소의 소스 파일을 수정하는 경우 수정된 파일의 소스를 게시하세요.
- sing-box와 같은 GPL 프로그램을 제품과 함께 배포하는 경우 수신자에게 해당 버전의 전체 소스(또는 효과적인 서면 제안)를 제공하십시오.
- 상표 및 이름: 공식적인 보증을 암시하는 방식으로 업스트림 프로젝트 이름이나 로고를 사용하지 마십시오.
