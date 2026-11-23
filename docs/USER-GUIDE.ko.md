<!-- source: USER-GUIDE.md | locale: ko | updated: 2026-11-20 -->
# CoreFusion 운영 가이드

> 언어: [简体中文](USER-GUIDE.md) · [English](USER-GUIDE.en.md) · [繁體中文](USER-GUIDE.zh-TW.md) · [Русский](USER-GUIDE.ru.md) · [日本語](USER-GUIDE.ja.md) · 한국어 · [Deutsch](USER-GUIDE.de.md) · [Français](USER-GUIDE.fr.md) · [Español](USER-GUIDE.es.md) · [Português](USER-GUIDE.pt.md) · [Türkçe](USER-GUIDE.tr.md) · [Bahasa Indonesia](USER-GUIDE.id.md) · [ไทย](USER-GUIDE.th.md) · [Tiếng Việt](USER-GUIDE.vi.md) · [العربية](USER-GUIDE.ar.md) · [فارسی](USER-GUIDE.fa.md)

> 이 가이드는 CoreFusion 사용자를 위한 것이며 설치부터 일상 작업까지 전체 작업 흐름을 다루고 있습니다.

---

## 목차

1. [시스템 개요](#1-시스템-개요)
2. [설치](#2-설치)
3. [초기 구성](#3-초기-구성)
4. [대시보드](#4-대시보드)
5. [노드 관리](#5-노드-관리)
6. [프로토콜 센터](#6-프로토콜-센터)
7. [프록시 관리](#7-프록시-관리)
8. [라우팅 리소스](#8-라우팅-리소스)
9. [가입센터](#9-가입센터)
10. [고객관리](#10-고객관리)
11. [계획 관리](#11-계획-관리)
12. [충전관리](#12-충전관리)
13. [티켓 관리](#13-티켓-관리)
14. [인증서 관리](#14-인증서-관리)
15. [트래픽 할당량](#15-트래픽-할당량)
16. [체인으로 연결된 프록시](#16-체인으로-연결된-프록시)
17. [온라인 사용자](#17-온라인-사용자)
18. [사용자 관리](#18-사용자-관리)
19. [포털 사이트](#19-포털-사이트)
20. [시스템 설정](#20-시스템-설정)
21. [운영 콘솔](#21-운영-콘솔)
22. [데이터베이스 모니터링](#22-데이터베이스-모니터링)
23. [감사 로그](#23-감사-로그)
24. [로그 관리](#24-로그-관리)
25. [운영 확장](#25-운영-확장)
26. [FAQ](#26-faq)
- [부록](#부록)

---

## 1. 시스템 개요

CoreFusion는 노드 관리, 구독 전달, 고객 관리, 청구, 인증서 및 체인 프록시를 통해 VLESS, VMess, Trojan, Shadowsocks, Hysteria2, TUIC 등을 지원하는 다중 프로토콜 프록시 관리 패널입니다.

### 1.1 핵심 기능

| 모듈 | 설명 |
|--------|-------------|
| 다중 프로토콜 | VLESS, VMess, Trojan, Shadowsocks, Hysteria2, TUIC, WireGuard 등 |
| 멀티코어 | Xray-core 및 sing-box(Hysteria2/TUIC 및 기타 QUIC 프로토콜은 sing-box에서 실행됨) |
| 다중 노드 | 5가지 노드 유형: 독립형, 미러, 활성/대기, 체인 입구, 체인 출구 |
| 구독 전달 | 다양한 구독 형식, 엔드포인트 및 형식 관리 |
| 청구 | 계획 템플릿, 충전 카드, 잔액, 주문, 리셀러 수수료 |
| 인증서 | Let's Encrypt 발급, DNS-01/HTTP-01 검증, 인증서 배포 |
| 연결된 프록시 | 다중 출구 로드 밸런싱, 외부 노드 등록, 체인 라우팅 |
| 트래픽 할당량 | 노드별 월별 트래픽 모니터링, 자동 일시 중지, 로그 재설정 |
| 포털 | 시각적 포털 편집기, 다국어, 사용자 정의 템플릿 |
| 알림 | 이메일 및 Telegram, 템플릿 및 방송 |

### 1.2 아키텍처

- **백엔드**: Go, gRPC
- **프런트엔드**: Vue 3 + Element Plus
- **데이터베이스**: SQLite(기본값) / MySQL / PostgreSQL
- **프록시**: Caddy / Nginx 역방향 프록시
- **패널 포트**: 설정 마법사에서 할당(기본적으로 임의의 무료 포트, 8080이 일반적으로 선택됨)
- **에이전트 gRPC 포트**: 기본적으로 18443

---

## 2. 설치

### 2.1 지원되는 환경

| 품목 | 지원 |
|------|---------|
| 배포판 | Debian / Ubuntu, RHEL / CentOS / Rocky / AlmaLinux / Fedora, Alpine, Arch, openSUSE |
| CPU 아키텍처 | 공식 바이너리: `amd64`, `arm64`; 소스 빌드는 `armv7`, `armv6`, `386`, `ppc64le`, `s390x`, `riscv64`, `loong64`도 지원합니다. |
| 서비스 관리자 | systemd(권장); systemd가 없는 컨테이너 또는 기타 환경에서는 `nohup`로 대체됩니다. |
| 메모리/디스크 | ≥ 1GB / ≥ 10GB(데이터베이스, 인증서, 로그 및 코어 바이너리는 데이터 디렉터리에 있음) |
| 패널 포트 | 설정 마법사에서 선택됨(기본적으로 임의의 빈 포트, `8080` 및 이와 유사한 포트도 괜찮음) |
| 에이전트 gRPC | 기본적으로 `18443` |

런타임 외부 명령은 설치 프로그램에 의해 자동으로 설치됩니다(`scripts/install.sh` 및 패널에서 생성된 에이전트 설치 스크립트, `--no-deps` / `CF_SKIP_DEPS=1`로 건너뛰기).

| 목적 | 패키지 |
|---------|---------|
| 방화벽 / 속도 제한 / 동시성 표시 | `nftables` (`nft`) |
| 트래픽 형성, 링크 및 소켓 | `iproute2` (`tc` / `ip` / `ss`) |
| nft를 사용할 수 없는 경우 대체 | `iptables`(EL8+에서는 `iptables-nft`, EL7에서는 `iptables`) |
| 연결 추적 | `conntrack-tools` (`conntrack`) |
| `nf_conntrack` / `ifb` 로드 | `kmod` (`modprobe` / `lsmod`) |
| 프로세스 감지 및 정리 | `procps` (`pgrep` / `pkill`) |
| 포트 확인/DH 매개변수 | `lsof` / `openssl` |
| 프록시 사용자 생성 | `shadow-utils`(데비안 `passwd`, 알파인/아치 `shadow`) |
| 다운로드 및 아카이브 | `curl` / `ca-certificates` / `tar` / `gzip` / `xz` |

> `nft` / `tc` 누락은 속도 제한 및 방화벽 기능에만 영향을 미칩니다. 패널이 여전히 시작됩니다.
> 먼저 `--cpu-info`를 실행하여 환경이 어떻게 감지되고 어떤 패키지가 누락되었는지 확인하세요.

### 2.2 단선 설치(권장)

```bash
curl -fsSL https://raw.githubusercontent.com/qist/corefusion/main/scripts/install.sh -o install.sh
sudo bash install.sh
```

또는 직접 파이프하십시오(상호작용 없음, 추가 플래그 없음).

```bash
curl -fsSL https://raw.githubusercontent.com/qist/corefusion/main/scripts/install.sh | sudo bash
```

스크립트는 다음을 수행합니다.

1. 배포판, CPU 아키텍처 및 libc를 감지한 다음 기본 패키지와 런타임 종속성을 설치합니다.
2. GitHub 릴리스에서 `corefusion-linux-<arch>`를 다운로드하고 일치하는 `.sha256`로 확인하세요.
3. `/opt/corefusion`에 설치하고 `data/{sqlite,certs,logs,runtime}`를 생성합니다.
4. 데이터베이스 구성 없이 `SAFE_PATHS=data`가 포함된 자리 표시자 `.env`만 작성하므로 첫 번째 시작은 설정 마법사로 들어갑니다(전체 `.env`는 마법사에 의해 생성됩니다).
5. systemd 서비스를 설치하고 시작합니다(또는 systemd를 사용할 수 없는 경우 `nohup`에서 실행).
6. 준비될 때까지 기다린 다음 설정 URL, 토큰 및 디렉터리 경로를 인쇄합니다.

출력 예:

```text
==================================================================
 CoreFusion installation complete
------------------------------------------------------------------
 Install dir : /opt/corefusion
 Data dir    : /opt/corefusion/data
 Binary      : /opt/corefusion/corefusion
 Logs        : journalctl -u corefusion -f
------------------------------------------------------------------
 Setup wizard : http://<server-ip>:<random-port>/init?init_token=<token>
 Setup token  : <token>
 Valid for    : 2 hours (see startup log)
 Note         : the token is shown once; after setup this port closes and the panel switches to its normal port.
==================================================================
```

패널이 `127.0.0.1`(예: 역방향 프록시 뒤)에서만 수신 대기하는 경우 스크립트는 연결할 수 없는 URL 대신 SSH 터널 명령을 인쇄합니다.

> 토큰을 분실하셨나요? 시작 로그를 다시 읽으십시오: `journalctl -u corefusion -n 50 --no-pager`
> (systemd 제외: `tail -n 50 /opt/corefusion/data/logs/panel.log`).

### 2.2.1 다국어 출력

설치 프로그램에는 중국어 간체, 영어, 페르시아어(Пارسی) 및 러시아어(Русский)의 네 가지 UI 언어가 포함되어 있습니다. `LANG` 시스템을 자동 감지하거나 명시적으로 선택할 수 있습니다.

```bash
# English
sudo bash install.sh --lang en

# Persian via environment variable
sudo CF_LANG=fa bash install.sh

# Pass options through a pipe
curl -fsSL https://raw.githubusercontent.com/qist/corefusion/main/scripts/install.sh | sudo bash -s -- --lang ru
```

지원되는 값: `zh` / `en` / `fa` / `ru`; `--lang=ru`도 허용됩니다. 지원되지 않는 명시적 값은 사용 가능한 언어 목록에서 실패합니다.

### 2.3 버전 고정/미러/오프라인 설치

```bash
# Pin a version
sudo bash install.sh --version v0.13.0

# GitHub download mirror (used as a URL prefix)
sudo bash install.sh --mirror https://ghproxy.net --version latest

# Use a local binary (offline / intranet / pre-built)
sudo bash install.sh --local ./dist/corefusion-linux-arm64

# Explicit target architecture
sudo bash install.sh --arch arm64 --local ./corefusion-linux-arm64
```

> `--version`가 없으면 스크립트는 GitHub 릴리스 목록의 최신 항목을 사용합니다.
> 대부분의 릴리스는 시험판이므로 `/releases/latest` 엔드포인트는 사용되지 않습니다.
>
> 공식 릴리스는 `amd64` / `arm64`만 제공합니다. 다른 아키텍처의 경우 `--source` 또는 `--local`와 함께 크로스 컴파일된 바이너리를 사용하세요.

### 2.4 소스에서 빌드하기

```bash
sudo bash install.sh --source --version v0.13.0
```

`--source`는 저장소를 복제하고(또는 스크립트가 내부에 있을 때 기존 체크아웃을 사용) `make panel`를 실행합니다. 툴체인 버전은 환경에 따라 선택됩니다.

| 의존성 | 선택 |
|------------|-----------|
| 이동 | `go.mod`(현재 **Go 1.27+**)의 `toolchain` / `go`에서 읽습니다. 시스템 버전이 너무 오래된 경우 일치하는 툴체인이 `<install-dir>/.toolchain`에 다운로드됩니다. |
| 노드 | **노드 22 LTS** 기본적으로(Vite에는 ≥ 18이 필요함) 필요할 때 일치하는 공식 빌드가 다운로드됩니다. |
| 노드(glibc < 2.28) | `unofficial-builds` glibc-217 빌드를 사용합니다(예: CentOS 7) |
| 노드(알파인/머슬) | 분포 `nodejs` / `npm`를 사용합니다. |

재정의:

```bash
# Force Go / Node versions
sudo CF_GO_VERSION=1.27.5 CF_NODE_MAJOR=22 bash install.sh --source

# Go toolchain mirror (default https://go.dev/dl)
sudo CF_GO_MIRROR=https://golang.google.cn/dl bash install.sh --source
```

수동 빌드(개발/디버깅):

```bash
git clone https://github.com/qist/corefusion.git
cd corefusion

# Frontend + panel for the current architecture; the frontend is embedded via go:embed
make panel

# Cross-compile
make panel-linux-amd64
make panel-linux-arm64
```

> `go build ./...`는 바이너리를 생성하지 않습니다. 다음을 사용하여 패널을 구축하세요.
> `go build -o corefusion ./cmd/corefusion/` 또는 `make panel`.

### 2.5 설치 프로그램 플래그

| 깃발 | 설명 |
|------|-------------|
| `-d, --dir <PATH>` | 설치 디렉토리, 기본값 `/opt/corefusion` |
| `-v, --version <TAG>` | 패널 버전(예: `v0.13.0`; 기본 `latest` |
| `--binary` | GitHub 릴리스 바이너리 사용(기본값) |
| `--source` | 소스에서 빌드하고 일치하는 Go/Node 준비 |
| `--local <FILE>` | 로컬 바이너리 사용(오프라인 설치) |
| `--arch <ARCH>` | 대상 아키텍처: `amd64` / `arm64` / `armv7` / `armv6` / `386` / `ppc64le` / `s390x` / `riscv64` / `loong64` |
| `--mirror <URL>` | GitHub 다운로드 접두사, 예: `https://ghproxy.net` |
| `--repo <URL>` | 소스 저장소, 기본값 `https://github.com/qist/corefusion.git` |
| `--ref <REF>` | 소스 브랜치/태그; 기본값은 `--version`이고, 그렇지 않으면 `main`입니다. |
| `--no-deps` | 시스템 종속성 설치 건너뛰기 |
| `--no-start` | 시작하지 않고 서비스 파일 설치 및 쓰기 |
| `--cpu-info` | 환경 감지 및 종속성 누락 인쇄 후 종료 |
| `--lang <LANG>` | 출력 언어: `zh` / `en` / `fa` / `ru` (기본값: 시스템 `LANG`) |
| `--uninstall` | 서비스를 중지하고 프로그램을 제거합니다(데이터 및 `.env` 유지). |
| `--purge` | `--uninstall`를 사용하여 데이터 디렉터리와 `.env`도 제거합니다. |

> 공식 릴리스에서는 `amd64` / `arm64` 바이너리만 제공됩니다. 다른 아키텍처에는 `--source`가 필요합니다.
> 그렇지 않으면 다운로드가 404를 반환합니다.

관련 환경 변수:

| 변수 | 목적 |
|----------|---------|
| `CF_GO_VERSION` | 소스 빌드에 필요한 Go 버전 재정의 |
| `CF_NODE_MAJOR` | Node 주 버전 재정의(기본값 22) |
| `CF_GO_MIRROR` | 툴체인 다운로드 미러로 이동 |
| `CF_SKIP_DEPS` | 종속성 설치를 건너뛰려면 에이전트 설치 스크립트에서 `1`로 설정하세요. |
| `CF_LANG` | 설치 프로그램 출력 언어 재정의(`--lang`와 동일) |

### 2.6 디렉토리 레이아웃

```text
/opt/corefusion/
├── corefusion                 # panel binary
├── corefusion.bak             # previous version (when upgrading)
├── .env                       # environment file (0600, generated by the setup wizard)
├── .env.secret                # encryption key file (0600, generated on first start)
├── .toolchain/                # Go / Node downloaded for source builds
└── data/                      # data directory (SAFE_PATHS)
    ├── sqlite/app.db          # default SQLite database
    ├── certs/                 # certificates and keys
    ├── logs/                  # runtime logs (without systemd)
    ├── runtime/               # core runtime config and state
    ├── binaries/              # downloaded xray / sing-box cores
    └── proxy/                 # proxy (Caddy/Nginx) sites and config
```

> 설치 프로그램은 `sqlite/`, `certs/`, `logs/` 및 `runtime/`만 생성합니다. `binaries/`, `proxy/`, `rules/` 등은 패널에 의해 생성되거나 코어가 설치될 때 생성됩니다.
> 데이터 디렉터리는 `.env`의 `SAFE_PATHS`에서 나오며 절대 경로일 수 있습니다. 설치 디렉터리를 변경해도 기존 데이터는 이동되지 않습니다.

### 2.7 서비스 관리

systemd(대부분의 배포판):

```bash
systemctl status corefusion --no-pager
systemctl restart corefusion
systemctl stop corefusion
journalctl -u corefusion -f          # follow logs
journalctl -u corefusion -n 100      # last 100 lines
```

systemd 제외(컨테이너 등):

```bash
cd /opt/corefusion && ./corefusion            # foreground (debugging)
cd /opt/corefusion && setsid nohup ./corefusion >> data/logs/panel.log 2>&1 &
tail -f data/logs/panel.log
```

> systemd가 없으면 부팅 시 프로세스가 시작되지 않습니다. 재부팅 후 수동으로 다시 시작하십시오(또는 설치 프로그램을 다시 실행하십시오).

### 2.8 업그레이드 및 제거

업그레이드(데이터 및 구성 유지):

```bash
sudo bash install.sh --version <new-version>
```

스크립트는 서비스를 중지하고 이전 바이너리를 `corefusion.bak`로 백업한 후 교체하고 다시 시작합니다. 기존 `.env` 및 `data/`는 그대로 유지됩니다.

제거:

```bash
# Remove the program only; keep data and .env
sudo bash install.sh --uninstall

# Also remove the data directory and .env (irreversible)
sudo bash install.sh --uninstall --purge
```

### 2.9 환경 파일(.env)

전체 `.env`는 구성(데이터베이스 DSN, 데이터 디렉터리, 비밀, 포트, 도메인)이 저장될 때 **설정 마법사에 의해 생성**됩니다.

새로 설치하는 경우 설치 프로그램은 이 자리 표시자만 작성하므로 패널은 기존 `data/`를 초기화된 설치로 착각하지 않고 마법사를 건너뜁니다.

```env
SAFE_PATHS=data
```

> 전체 구성(`.env` 및 `CF_DB_DSN`)이 이미 존재하는 경우 설치 프로그램은 이를 덮어쓰지 않으며 마법사가 다시 나타나지 않습니다.
> 나중에 `.env`에 대한 수동 편집 내용은 다시 시작한 후에 적용됩니다.

#### 데이터 디렉터리

```env
SAFE_PATHS=data                       # relative to the install directory
# SAFE_PATHS=/srv/corefusion-data     # or an absolute path
```

#### 데이터베이스

`CF_DB_DSN`는 접두사로 감지됩니다.

| 접두사 | 데이터베이스 |
|--------|----------|
| 설정되지 않았거나 경로가 비어 있음 | SQLite(기본값) |
| `sqlite://` | SQLite(명시적) |
| `mysql://` | MySQL |
| `postgres://` / `postgresql://` | PostgreSQL |

`CF_DB_TYPE`는 선택 사항이며 감지(`sqlite` / `mysql` / `postgres`)를 재정의합니다.

> **암호화된 비밀번호**: MySQL/PostgreSQL 패널에서 작성된 DSN(설정 마법사/데이터베이스 스위치)
> `.env`에 `enc:...`(AES-256-GCM, 키 `CF_SECRET_ENCRYPTION_KEY`)로 저장됩니다.
> 읽기 시 해독됩니다. 직접 작성한 일반 텍스트 DSN은 여전히 ​​작동합니다.

**SQLite(기본값)**

```env
# Bare path (detected as SQLite)
CF_DB_DSN=data/sqlite/app.db

# Or explicit sqlite:// prefix
CF_DB_DSN=sqlite:///opt/corefusion/data/sqlite/app.db
```

**MySQL**

```env
# Format: mysql://user:password@tcp(host:port)/dbname?params
CF_DB_DSN=mysql://corefusion:password@tcp(127.0.0.1:3306)/corefusion?charset=utf8mb4&parseTime=True&loc=Local
```

**PostgreSQL**

```env
# Format: postgres://user:password@host:port/dbname?params
CF_DB_DSN=postgres://corefusion:password@127.0.0.1:5432/corefusion?sslmode=disable
```

#### 연결 풀(선택 사항, MySQL/PostgreSQL)

```env
CF_DB_MAX_OPEN_CONNS=100
CF_DB_MAX_IDLE_CONNS=10
```

#### 기타 공통 변수

| 변수 | 설명 |
|----------|-------------|
| `JWT_SECRET` | 로그인 토큰의 서명 키 누락된 경우 처음 시작 시 생성됨 |
| `CF_SECRET_ENCRYPTION_KEY` | 민감한 설정을 암호화하기 위한 키(AES-256-GCM) 누락 시 생성됨 |
| `CF_INITIAL_ADMIN_PASSWORD` | 초기 관리자 비밀번호(선택 사항, 무작위이며 기본적으로 한 번 인쇄됨) |
| `CF_NETWORK_TRAFFIC_ACCOUNTING` | iptables 기반 트래픽 계정을 비활성화하려면 `0`로 설정합니다(기본적으로 활성화됨). |
| `CF_NETWORK_TRAFFIC_TABLE` | 트래픽 계산에 사용되는 iptables 테이블, 기본값 `corefusion` |

> **암호화 키는 `.env.secret`에 있습니다**: 처음 시작 시 `CF_SECRET_ENCRYPTION_KEY`는 `.env` 대신 `.env.secret`(0600, `.env`와 동일한 디렉터리)에 기록됩니다. `.env`의 레거시 키는 자동으로 마이그레이션됩니다. 이 파일을 별도로 백업하십시오. 파일을 분실하면 모든 `enc:` 값을 해독할 수 없게 됩니다.

> 실제 클라이언트 IP를 확인하기 위한 신뢰할 수 있는 역방향 프록시는 더 이상 환경 변수를 통해 구성되지 않습니다.
> **설정 → 네트워크**에 `panel.trusted_proxies`로 저장됩니다.

> `.env` 및 `.env.secret`에는 모두 비밀이 포함되어 있습니다. 패널은 시작 시 `0600`에 대한 권한을 강화합니다. 절대 커밋하지 마세요.

### 2.10 Docker(선택 사항, 자체 이미지 구축)

저장소는 Dockerfile을 제공하지 **않습니다**. 컨테이너화하는 경우 고유한 이미지를 작성하세요. 참조 호출:

```bash
docker run -d \
  --name corefusion \
  -p 8080:8080 \
  -p 18443:18443 \
  -v /opt/corefusion/data:/app/data \
  -v /opt/corefusion/.env:/app/.env \
  <your-image>
```

> 컨테이너에는 일반적으로 systemd 및 `nft` 권한이 없으므로 속도 제한/방화벽 기능이 작동하지 않습니다.
> 작업 디렉토리는 상대 `SAFE_PATHS`의 기본과 일치해야 합니다.

### 2.11 설치 후 확인

```bash
# Environment and dependencies
sudo bash install.sh --cpu-info

# Service state
systemctl is-active corefusion

# Startup log (contains the setup URL and token)
journalctl -u corefusion -n 50 --no-pager

# Listening ports
ss -lntp | grep corefusion
```

---

## 3. 초기 구성

처음 시작할 때 `.env`에 유효한 `CF_DB_DSN`가 없으면 패널은 아직 관리 UI를 제공하지 **않습니다**. 그것은 시작한다
일회성 토큰을 사용하여 18000~18999의 임의 포트에서 **설정 마법사**를 수행한 다음 저장된 후 일반 패널로 다시 시작합니다.

### 3.1 설정 마법사 열기

1. 설치 프로그램 출력에서 "설정 마법사" URL를 복사합니다. 또는
2. 서버의 로그를 읽으십시오.

```bash
journalctl -u corefusion -n 50 --no-pager | grep -A2 'setup wizard'
```

```text
CoreFusion first start - setup wizard
  URL   : http://<ip>:<port>/init?init_token=<token>
  Token : <token>
  Valid : 2h0m0s
```

3. 브라우저에서 URL를 엽니다(이미 `init_token`가 포함되어 있으므로 수동 입력이 필요하지 않습니다).

> 마법사 세션은 기본적으로 **2시간** 후에 만료됩니다. 만료되거나 패널이 다시 시작되면 새 패널을 얻으십시오.
> 토큰을 공유하지 마세요. 설정이 완료되면 이 포트는 닫히고 일반 패널 포트만 남습니다.

### 3.2 마법사 단계

마법사에는 8단계가 있습니다. 언제든지 돌아갈 수 있습니다. 최종 저장 전까지는 아무 것도 커밋되지 않습니다.

| 단계 | 내용 |
|------|---------|
| 1. 데이터베이스 | SQLite / MySQL / PostgreSQL |
| 2. 데이터 디렉토리 | `.env`의 `SAFE_PATHS`에 작성됨 |
| 3. 암호화 키 | `JWT_SECRET` 및 `CF_SECRET_ENCRYPTION_KEY` |
| 4. 코어 | 설치할 프록시 코어 |
| 5. 프록시 | 없음 / Nginx / Caddy |
| 6. 사이트 및 도메인 | 포트, 웹 경로, 관리자 계정, 통화, 시간대, 도메인 |
| 7. HTTPS 인증서 | 비활성화됨 / 업로드 / ACME |
| 8. 요약 | `.env`를 검토하고 다운로드한 후 저장하고 다시 시작하세요. |

#### 1단계: 데이터베이스

- **SQLite(로컬 파일, 권장)**: `<data-dir>/sqlite/app.db`를 사용하려면 경로를 비워 두세요.
- **MySQL / PostgreSQL**: 호스트, 포트, 사용자, 비밀번호, 데이터베이스 이름; PostgreSQL는 `disable` / `require` / `verify-full`도 지원합니다.
- "연결 테스트"를 클릭하여 확인하세요. 저장도 테스트합니다. 누락된 데이터베이스가 자동으로 생성됩니다(계정에 `CREATE`가 없는 경우 수동으로 생성).
- DSN(비밀번호 포함)은 `.env`에 기록되기 전에 암호화됩니다.

#### 2단계: 데이터 디렉터리

데이터베이스, 인증서, 로그, 핵심 바이너리 등을 저장합니다. `.env`의 `SAFE_PATHS`입니다. 기본값은 `data`입니다.
(설치 디렉토리에 상대적); 절대 경로가 허용됩니다. **나중에 함부로 바꾸지 마세요**, 또는 패널
기존 데이터베이스와 인증서를 찾을 수 없습니다.

#### 3단계: 암호화 키

- `JWT_SECRET`: 로그인 토큰에 서명합니다.
- `CF_SECRET_ENCRYPTION_KEY`: 민감한 설정(데이터베이스 비밀번호, DNS 자격 증명, 충전 카드 등)을 암호화합니다.
- 둘 다 기본적으로 무작위로 생성됩니다. 새로운 값을 얻으려면 "재생성"을 사용하십시오. **`CF_SECRET_ENCRYPTION_KEY`를 변경하면 이전에 암호화된 데이터를 읽을 수 없게 됩니다** — 백업하세요.

#### 4단계: 코어

설치할 프록시 코어를 선택하세요.

- **Xray**: VLESS / VMess / Trojan / Shadowsocks 등;
- **sing-box**: Hysteria2 / TUIC 및 기타 QUIC 프로토콜을 전달합니다.

저장하고 다시 시작하면 패널이 자동으로 다운로드되어 설치됩니다. "프로토콜"에서 버전과 상태를 확인하세요. 나중에 설치할 수도 있습니다.

#### 5단계: 프록시

- **없음**: 패널이 클라이언트에게 직접 서비스를 제공합니다(TLS를 직접 처리).
- **Nginx / Caddy**: 패널은 역방향 프록시 및 인증서 호스팅을 관리합니다. 하나를 선택하세요.

프록시를 선택하면 패널은 일반적으로 `127.0.0.1`에서만 수신하고 프록시는 외부 세계에 서비스를 제공합니다.

#### 6단계: 사이트 및 도메인

| 필드 | 설명 |
|-------|-------------|
| 청취 주소 | 자동 파생: 프록시 → `127.0.0.1`; 인증서가 있는 프록시 없음 → `0.0.0.0`; 그렇지 않으면 → `127.0.0.1` |
| 패널 포트 | 기본적으로 임의의 무료 포트; 원하는 경우 변경하세요(예: 8080). |
| 구독 포트 | 선택사항; `0`는 패널 포트를 공유합니다. |
| 포털 포트 | 선택사항; `0`는 패널 포트를 공유합니다. |
| 웹 경로 | 관리 UI 경로, 기본값 `/`; API는 항상 `/api/v1`입니다. |
| 관리자 사용자 이름 / 비밀번호 | 기본적으로 무작위입니다. **비밀번호는 요약에 한 번만 표시됩니다** — 저장하세요 |
| 액세스 모드 | “도메인” 또는 “IP” |
| 기본 통화/시간대 | 충전카드 및 잔액의 기본 통화 패널 및 프록시 로그 시간대(예: `Asia/Shanghai`) |
| 역방향 프록시에 추가 | 프록시와 도메인이 설정되면 프록시에 패널/구독/포털 도메인을 한 번에 추가 |

도메인을 입력하면 해당 설정(콜백 도메인, 구독 엔드포인트, 공유 도메인)이 기록됩니다.

#### 7단계: HTTPS 인증서

| 소스 | 설명 |
|--------|-------------|
| HTTPS 비활성화 | 패널/포털/구독은 HTTP(안전하지 않음)를 통해 제공됩니다. 역방향 프록시를 사용하면 나중에 프록시 측에 TLS를 추가할 수 있습니다. |
| 인증서 업로드 | 인증서 PEM 및 개인 키 PEM 붙여넣기(도메인 액세스 모드만 해당) |
| ACME | 이 단계를 제출하면 인증서를 발급합니다. 도메인 및 IP 인증서 지원 |

ACME 옵션:

- **CA**: Let's Encrypt, Let's Encrypt(스테이징), ZeroSSL, Google 공용 CA;
- **대상**: 구성된 도메인 또는 공용 IP(IP 인증서는 Let's Encrypt에서만 지원됨)
- **도전과제**: `HTTP-01`(포트 80 필요) 또는 `DNS-01`(와일드카드, 포트 80 없음, DNS 공급자를 선택하고 자격 증명 입력)
- **키 유형/크기**: ECC 256/384 또는 RSA 2048/3072/4096;
- **Google 공용 CA**: EAB가 필요합니다. 키 ID/HMAC를 수동으로 입력하거나 GCP 서비스 계정을 사용하세요.

> IP 인증서는 호스트의 공개 IPv4/IPv6을 자동 감지합니다. 프록시 뒤에서 감지된 주소는 프록시 송신이 아닌 호스트의 주소입니다.
> 도메인 인증서는 나중에 "인증서"에서 발급될 수도 있습니다.

#### 8단계: 요약

데이터베이스, 데이터 디렉터리, 코어, 프록시, 포트, 관리자 계정, 도메인 및 `.env` 키(비밀 마스크)를 표시하고 다음을 제공합니다.

- **.env 다운로드**: 구성 사본을 보관하세요.
- **액세스 URL**: 패널/구독/포털의 최종 URL; HTTP는 안전하지 않은 것으로 표시됩니다.

데이터베이스와 `.env`를 작성하려면 "저장 및 다시 시작"을 클릭하십시오. 패널이 정상 서비스로 다시 시작됩니다.

### 3.3 설정 후

- 서비스가 다시 시작되고 시작 로그에 최종 수신 주소가 인쇄됩니다.
- 관리자 계정으로 로그인하고 요약에서 URL 및 웹 경로에 액세스하세요.
- 비밀번호를 분실한 경우 `./corefusion admin reset-password`를 사용하여 오프라인으로 재설정하세요.

### 3.4 비대화형 CLI 설정

자동화 또는 헤드리스 서버의 경우 구성을 작성하고 설정 완료를 직접 표시합니다.

```bash
cd /opt/corefusion
./corefusion init \
  --db-type sqlite \
  --data-dir /opt/corefusion/data \
  --admin-username admin \
  --admin-password 'change-me-strong-password' \
  --listen-port 8080 \
  --proxy-type caddy \
  --cores xray,singbox \
  --panel-domain panel.example.com \
  --tls-mode acme --tls-domain panel.example.com --tls-email you@example.com
```

일반적인 플래그:

| 깃발 | 설명 |
|------|-------------|
| `--db-type` / `--db-host` / `--db-port` / `--db-user` / `--db-password` / `--db-name` | 데이터베이스 설정 |
| `--data-dir` | 데이터 디렉토리; 기본값은 `SAFE_PATHS` 또는 `./data`입니다. |
| `--listen-port` / `--subscription-port` / `--portal-port` / `--web-path` | 포트 및 웹 경로(기본적으로 임의의 무료 포트) |
| `--admin-username` / `--admin-password` | 관리자 계정(생략시 임의의 비밀번호가 1회 출력됩니다) |
| `--proxy-type` | `none` / `nginx` / `caddy` |
| `--cores` | 설치할 코어(쉼표로 구분): `xray,singbox` |
| `--panel-domain` / `--subscription-domain` / `--portal-domain` | 도메인(설정에 기록되고 역방향 프록시에 추가됨) |
| `--tls-mode` | `none` / `self_signed` / `acme` |
| `--tls-domain` / `--tls-email` / `--tls-challenge` / `--tls-key-type` / `--tls-key-bits` | 인증서 옵션(`--tls-challenge`의 기본값은 `http-01`이며, `dns-01`에는 `--tls-dns-channel` 또는 `--tls-dns-provider`가 필요함) |
| `--tls-ip-cert` | 탐지된 공인 IP에 대한 인증서 발급(Let's Encrypt만 해당) |
| `--acme-provider` | `lets_encrypt` / `lets_encrypt_staging` / `zerossl` / `google` |
| `--currency` / `--timezone` | 기본 통화/시간대 |

전체 목록: `./corefusion init --help`. 그런 다음 서비스를 시작합니다.

```bash
systemctl restart corefusion
```

### 3.5 설정 마법사 다시 실행

다시 시작하려면(`.env` 및 설정 표시 제거, **데이터 유지**):

```bash
cd /opt/corefusion
./corefusion reset-init      # type RESET to confirm
systemctl restart corefusion
```

그런 다음 시작 로그의 새 URL 및 토큰을 사용하십시오.

> 다시 초기화하면 관리자 계정과 웹 경로가 재설정됩니다. 이전 데이터베이스의 비즈니스 데이터는 그대로 유지되지만 로그인 계정을 다시 생성해야 할 수도 있습니다.
>
> `reset-init`는 전체 `.env`(`SAFE_PATHS` 포함)를 삭제합니다. **데이터 디렉터리가 사용자 정의 절대 경로**인 경우 다시 시작하기 전에 `SAFE_PATHS`를 복원하십시오. 그렇지 않으면 패널이 `./data`로 표시됩니다.
>
> ```배쉬
> printf 'SAFE_PATHS=/srv/corefusion-data\n' > /opt/corefusion/.env
> ```

### 3.6 처음 로그인 후 보안

#### 초기 비밀번호 변경

1. 오른쪽 상단에 있는 사용자 이름을 클릭하세요.
2. "비밀번호 변경"을 선택하십시오.
3. 이전 비밀번호와 새 비밀번호를 입력하세요.
4. “저장”을 클릭하세요.

#### 동적 코드 로그인 활성화(2FA / TOTP)

> 활성화되면 계정이 **동적 코드 로그인**으로 전환됩니다. 즉, 정적 비밀번호는 더 이상 허용되지 않습니다. 오직
> 인증 앱의 6자리 코드(또는 일회성 복구 코드)가 작동합니다. 이것은 비밀번호 로그인을 **대체**합니다.
> '비밀번호+코드' 2단계 인증이 아닙니다.

1. 오른쪽 상단에 있는 사용자 이름을 클릭하세요.
2. "2FA 활성화"를 선택하십시오.
3. 현재 비밀번호를 입력하세요(이 변경 사항을 확인하는 경우에만).
4. 인증 앱(Google Authenticator / 1Password 등)으로 QR 코드를 스캔하거나 수동으로 키를 입력하세요.
5. 활성화를 완료하려면 6자리 코드를 입력하세요.
6. **복구 코드를 즉시 저장하세요**(다운로드 가능) 각 코드는 한 번만 작동합니다.

나중에 로그인:

- 사용자 이름을 입력한 후 비밀번호 필드는 "6자리 코드/복구 코드"가 되고 버튼은 "코드로 로그인"이 됩니다.
- 현재 코드를 입력하세요. **정적 비밀번호는 더 이상 사용할 수 없습니다**;
- 복구 코드는 일회용입니다. 코드가 소진되면 “동적 코드 로그인(2FA)”에서 다시 생성하십시오(이렇게 하면 이전 코드가 모두 무효화됩니다).

인증자가 분실되고 복구 코드가 소진된 경우 서버에서 2FA 오프라인을 비활성화하세요.

```bash
cd /opt/corefusion
./corefusion admin 2fa reset -u admin    # disables 2FA, clears the secret and recovery codes, revokes all sessions
```

> 인증자와 복구 코드가 없는 단독 최고 관리자는 웹 UI를 통해 복구할 수 없으며 위 명령을 사용해야 합니다.

#### Passkey 등록(선택 사항)

1. 오른쪽 상단에 있는 사용자 이름을 클릭하세요.
2. "Passkey 관리"를 선택하십시오.
3. “새 Passkey 등록”을 클릭하세요.
4. 장치 프롬프트를 완료합니다.

> **참고**: 암호 키에는 HTTPS 또는 localhost가 필요하며 도메인이 허용 목록에 있어야 합니다.

---

## 4. 대시보드

대시보드에는 다중 노드 클러스터 개요 및 노드 상태가 표시됩니다.

### 4.1 그것이 보여주는 것

- **시스템 정보**: 호스트 이름, OS 버전, 가동 시간, CPU, 메모리
- **트래픽**: 실시간 대역폭, 총 트래픽, 월별 추세
- **노드 상태**: 온라인/오프라인/수 저하, 고부하 노드
- **경고**: 오프라인 노드, 저하된 노드, 곧 만료되는 인증서, 할당량 경고
- **클러스터 리소스**: 평균 CPU, 메모리, 디스크 및 처리량

### 4.2 조치

- 노드의 세부정보 페이지를 열려면 '세부정보 보기'를 클릭하세요.
- 실시간 상태를 보려면 노드 카드를 클릭하세요.
- 필터를 사용하여 상태, 유형 또는 지역별로 범위를 좁힙니다.

---

## 5. 노드 관리

노드 관리는 CoreFusion의 핵심이며 여러 노드 유형을 지원합니다.

### 5.1 노드 유형

| 유형 | 설명 |
|------|-------------|
| 독립형(`standalone`) | 독립 코어 노드 |
| 거울 (`mirror`) | 대상 노드의 구성을 동기화합니다. |
| 기본/백업(`primary_backup`) | 서로 장애 조치되는 파트너 노드 |
| 체인 엔트리(`chain_entry`) | 체인 프록시의 진입 노드 |
| 체인 엑시트(`chain_exit`) | 연결된 프록시의 종료 노드 |

### 5.2 노드 추가

1. "노드 관리"를 엽니다.
2. “서버 추가”를 클릭하세요.
3. 기본사항을 입력하세요.
   - 이름(예: `HongKong-HK01`
   - 호스트 이름: IP 또는 도메인
   - 지역(예: `HK`, `JP`, `US`
   - 노드 유형;
4. 에이전트 포트(기본값 `18443`)를 설정합니다.
5. 지원되는 코어를 선택하세요.
6. “저장”을 클릭하세요.

### 5.3 원클릭 배포

1. 노드를 추가한 후 노드 목록에서 "배포 명령"을 선택합니다(세부 정보 페이지에서도
 "부트스트랩 설치 명령");
2. 명령을 복사하세요. 여기에는 일회성 등록 토큰이 포함되어 있습니다(24시간 동안 유효하며 재사용하지 마십시오).
3. 원격 노드에서 루트로 실행하십시오. 그것은 다음과 같습니다:

   ```bash
   bash <(curl -fsSL "http://<panel-host>:<panel-port>/api/v1/agent/install?id=<node-id>&token=<one-time-token>&port=<panel-port>")
   ```

4. 에이전트는 패키지를 다운로드하고 패널에 등록한 후 시작합니다.

### 5.4 노드 세부정보

노드를 클릭하여 세부정보 페이지를 엽니다.

- **개요**: 기본 정보, 에이전트 상태, 인증서 상태; 리소스 카드에는 CPU/메모리/디스크 사용량, NIC 업/다운 비율, 코어 사용자 트래픽 비율, 월간 사용 트래픽, TCP 연결, 시스템 로드 및 송신 NIC 누적 트래픽(10초마다 자동 새로 고침)이 표시됩니다.
- **프로토콜 센터**: 핵심 프로세스 구성
- **프록시**: Caddy / Nginx 구성
- **핵심 프로세스**: 시작/중지/다시 시작
- **코어 트래픽**: 코어에서 전달된 사용자 트래픽 및 월별 총계
- **WARP**: WARP 구성
- **직접 도메인**: 도메인 관리
- **비율 제한**: 트래픽 형성
- **인증서**: 인증서 바인딩
- **온라인 사용자**: 실시간 온라인 사용자

### 5.5 노드 작업

- **동기화 구성**: 패널 구성을 노드에 푸시합니다.
- **강제 재연결**: 에이전트 연결을 끊고 다시 연결
- **업데이트 에이전트**: 에이전트 업데이트 푸시
- **인증서 취소**: 노드의 에이전트 인증서를 취소합니다.

---

## 6. 프로토콜 센터

프로토콜 센터는 핵심 프로세스에 대한 프로토콜 매개변수를 구성합니다.

### 6.1 지원되는 코어

| 코어 | 설명 |
|------|-------------|
| Xray 코어 | VLESS / VMess / Trojan / Shadowsocks 등 |
| sing-box | TUN을 지원하는 모듈형 범용 프록시 플랫폼 Hysteria2 / TUIC 및 기타 QUIC 프로토콜을 전달합니다. |

### 6.2 구성 모듈

모듈은 두 코어 간에 다릅니다. 패널에는 선택한 코어와 일치하는 세트가 표시됩니다.

**Xray-코어**

- **로그**: 접속 로그, 오류 로그, 출력 모드
- **API**: gRPC API 수신 주소, 서비스 목록
- **DNS**: DNS 서버, 쿼리 전략, 정적 호스트
- **라우팅**: 라우팅 규칙, 로드 밸런싱, VLESS 라우팅
- **인바운드/아웃바운드**: 프로토콜, 전송, TLS
- **전송**: WebSocket, gRPC, XHTTP 등(TUN은 전송이 아닌 인바운드 프로토콜입니다)
- **TLS**: 인증서, ALPN, uTLS 지문, ECH
- **Reality**: 대상 관리, 키 쌍, 짧은 ID
- **상태 확인/FakeDNS/지표/정책/지역 데이터/버전 제한**: 프로브, IP 풀, Prometheus, 시간 초과 및 버퍼, 리소스 파일, 코어 버전 범위

**sing-box**

- **로그**: 레벨, 타임스탬프, 출력
- **NTP**: 시간 동기화
- **DNS**: DNS 서버 및 규칙
- **라우팅**: `final` 기본 아웃바운드, **규칙 세트**, **규칙**, 기본 도메인 확인자, 네트워크 전략 및 폴백(6.6 참조)
- **인바운드/아웃바운드**: 프로토콜, 전송, TLS
- **엔드포인트**: WireGuard(WARP 구성 포함), MASQUE 클라이언트 및 유사 여러 개가 공존할 수 있으며 `tag`로 구별됩니다.

> "시스템 네트워크 통합 사용"이 활성화되면 엔드포인트의 **이름**은 커널 인터페이스 이름입니다.
> 최대 15자, 문자/숫자/`._-`만 가능합니다. 너무 길거나 유효하지 않은 이름은 자동으로 대체됩니다.
> 엔드포인트 `tag`로. 새 엔드포인트는 프로토콜별로 접두사가 붙습니다(`wg-` / `ts-` / `oc-` / `ovpn-c-` /
> `ovpn-s-` / `mq-c-` / `mq-s-`) 및 `WireGuard-xxxxxx`와 같은 레거시 레코드는 정규화된 것으로 표시됩니다.
> 편집기가 열리자마자.
- **서비스**: `hysteria-realm`, DERP 또는 `resolved`와 같은 최상위 sing-box `services` 어레이
- **인증서 / HTTP 클라이언트 / 네트워크 네임스페이스**: 필요할 때만 확장하는 JSON 섹션
- **실험적**: `clash_api` 및 유사; **활성화하지 않으면 생성된 구성에 아무 것도 기록되지 않습니다**

> sing-box 1.12에서는 `geosite` / `geoip` 라우팅 필드를 제거했습니다. 지역 기반 라우팅이 이제 통과됩니다.
> `rule_set`. 이전 필드를 유지하면 코어가 구성을 디코딩하지 못합니다(`unknown field`).

### 6.3 인바운드 구성

1. 코어와 프로토콜을 선택합니다.
2. 기본사항을 입력하세요.
   - 태그
   - 수신 주소 및 포트
   - 프로토콜 유형
3. 프로토콜 매개변수(UUID, 비밀번호 등)를 설정합니다.
4. 전송(TCP, WebSocket, gRPC 등)을 선택합니다.
5. TLS(인증서, ALPN)를 구성합니다.
6. 저장하고 코어에 적용합니다.

> 새로운 인바운드는 기본적으로 `::`에서 수신 대기합니다(Linux의 이중 스택, IPv4 및 IPv6 모두 처리).
> 인터페이스를 제한해야 하는 경우에만 `0.0.0.0` 또는 특정 주소로 변경하세요.

"인바운드 포트 사용" 포트 모드는 **자동으로 방화벽 허용 규칙 생성**도 제공합니다.
인바운드 편집기는 먼저 노드 방화벽을 조사하고, Firewalld / ufw / nftables / iptables가 감지되면
청취 프로토콜(TCP/UDP)에 따라 저장 시 포트를 허용할 수 있습니다.

- hy2 / 히스테리 포트 호핑은 청취 포트와 모든 동적 포트 범위(예: `20000-30000`)를 모두 허용합니다. 여러 범위는 쉼표로 구분됩니다.
- 허용량은 멱등적입니다. 기존 적용 규칙(포트 범위/다중 포트)은 다시 생성되지 않습니다.
- 포트를 변경하면 새 규칙을 허용하기 전에 이전 규칙이 취소됩니다. 인바운드 삭제, 포트 호핑 비활성화 또는 Unix 소켓/tun(수신 포트 없음)으로 전환하면 정리됩니다.
- 활성화된 방화벽이 감지되지 않으면 힌트만 표시되고 규칙은 작성되지 않습니다.

### 6.3.1 역방향 프록시 경로(액세스 도메인 → 전면 TLS → 역방향 프록시 경로)

인바운드의 자체 전송 경로가 `/`인데 외부 nginx / caddy가 하위 경로 아래에 마운트하는 경우
(예: `location /apps { proxy_pass http://127.0.0.1:8080/; }`), 클라이언트는 `/apps` 또는
요청은 해당 위치에 도달하지 않습니다. 구독은 계속 다운로드되지만 노드는 연결되지 않습니다.
"액세스 도메인 → 전면 TLS" 아래에 **역방향 프록시 경로**를 입력합니다.

- `/apps` 또는 `/apps?ed=2560`(초기 데이터 포함)를 입력하세요. 구독 및 공유 링크는 인바운드 전송 경로를 **대체**합니다.
- ws, httpupgrade, xhttp 및 h2/http에 적용됩니다. 경로가 없는 전송(raw/tcp, kcp)은 무시됩니다.
- 이는 TLS가 종료되는 위치와 무관합니다. 전면 TLS가 꺼진 경우(코어를 따라감) 경로는 계속 적용됩니다.
- gRPC에는 URL 접두사 개념이 없으므로(클라이언트 경로는 항상 `/{serviceName}/…`임) 경로가 **serviceName에 병합됩니다**
(`/apps` + `svc` → `apps/svc`); 역방향 프록시는 `grpc_pass`에서 해당 접두사를 제거해야 합니다.

### 6.4 아웃바운드 구성

1. 아웃바운드를 추가하세요.
2. 태그와 프로토콜을 설정합니다.
3. 서버 주소와 포트를 설정합니다.
4. 전송 및 TLS를 구성합니다.
5. 저장하고 적용하세요.

### 6.5 링크 공유

1. 인바운드 목록에서 “링크 공유”를 클릭하세요.
2. 형식을 선택하세요:
   - URI 공유
   - Mihomo YAML
   - sing-box JSON
   - Xray JSON
3. 링크를 복사하거나 QR 코드를 생성하세요.

### 6.6 sing-box 라우팅 및 규칙 세트

"프로토콜 센터" → sing-box → "라우팅"에는 두 부분이 있습니다: **규칙 세트** 재사용 가능 선언
rule 파일과 **Rules**가 일치 조건과 아웃바운드를 결정합니다. 규칙은 다음에서 평가됩니다.
위에서 아래로 첫 번째 경기가 승리합니다.

#### 6.6.1 규칙 세트

| 필드 | 설명 |
|-------|-------------|
| 태그 | 규칙에서 참조하는 규칙 세트 식별자입니다. 고유해야 합니다 |
| 유형 | `local`는 노드에서 파일을 읽습니다. `remote`를 사용하면 코어가 `update_interval`에 다운로드할 수 있습니다. |
| 형식 | `.srs`의 경우 `binary`(기본값); 소스 형식 `.json`용 `source` |
| 경로 | `local` 경로는 노드 데이터 디렉터리에 상대적입니다. `data/rules/`의 파일은 `rules/xxx.srs`를 사용합니다. |
| URL | `remote`의 다운로드 주소; `download_detour`는 다운로드에 사용되는 아웃바운드를 선택합니다(보통 `direct`). |

로컬 규칙 세트(권장, 먼저 “서비스 리소스 → 규칙 파일”에서 파일을 다운로드):

```jsonc
"route": {
  "rule_set": [
    { "type": "local", "tag": "geosite-cn", "format": "binary", "path": "rules/geosite-cn.srs" },
    { "type": "local", "tag": "geoip-cn", "format": "binary", "path": "rules/geoip-cn.srs" }
  ]
}
```

> 패널에서 파일을 다운로드하는 것만으로는 충분하지 않습니다. 먼저 `local` 규칙 세트를 패널에 푸시해야 합니다.
> 노드(“모든 노드에 푸시”)를 실행하여 파일이 노드의 `data/rules/`에 존재하도록 합니다. 그렇지 않으면 핵심
> 파일을 찾을 수 없기 때문에 시작하지 못했습니다.

원격 규칙 세트(노드가 URL에 도달할 수 있어야 함):

```jsonc
{ "type": "remote", "tag": "geosite-openai", "format": "binary",
  "url": "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-openai.srs",
  "download_detour": "direct", "update_interval": "1d" }
```

로컬 파일을 선택하면 태그는 파일 이름(`rules/geosite-cn.srs` →
`geosite-cn`); 원격 세트의 경우 URL 끝에서 나옵니다. 형식은 다음에서 채워집니다.
확장자(`.srs` → `binary`, `.json` → `source`). 파생된 값은 자유롭게 편집할 수 있으며,
이미 입력한 값은 덮어쓰이지 않습니다.

> `download_detour`는 sing-box 1.14부터 더 이상 사용되지 않으며 1.16에서 제거될 예정입니다. 아직도
> 1.15에서 작동합니다. 새로운 구성에서는 다음과 함께 최상위 `http_clients`를 사용해야 합니다.
> 대신 `route.default_http_client`.

#### 6.6.2 규칙

| 필드 | 경기 |
|-------|---------|
| `domain` | 정확한 도메인 |
| `domain_suffix` | 하위 도메인을 포함한 도메인 접미사 |
| `domain_keyword` | 하위 문자열 일치 |
| `domain_regex` | 정규식 |
| `ip_cidr` / `ip_is_private` | 대상 IP |
| `source_ip_cidr` | 소스 IP |
| `port` / `port_range` | 대상 포트 |
| `network` | `tcp` / `udp` |
| `rule_set` | 사용할 규칙 세트의 태그 |
| `outbound` | 사용할 아웃바운드 태그("아웃바운드" 또는 "엔드포인트" 아래에 있어야 함) |
| `type: "logical"` | `mode`가 `and` / `or`로 설정된 결합 조건 |

하나의 규칙에 있는 여러 필드는 AND(예: `network` + `port`)로 결합됩니다.
`logical` 래퍼가 필요합니다. OR / NOT 조합에만 `type: "logical"`를 사용하세요. `rule_set`
`rules` 옆이 아닌 `rules` 하위 규칙 내부에 배치해야 합니다.

```jsonc
"rules": [
  { "domain_suffix": ["gstatic.com", "googleapis.com"], "outbound": "direct" },
  { "domain_keyword": ["ip.sb", "yandex.com"], "outbound": "warp" },
  { "rule_set": ["geosite-openai", "geosite-google"], "outbound": "warp" },
  { "rule_set": ["geoip-cn"], "outbound": "warp" },
  { "network": "tcp", "port": 443,
    "rule_set": ["geosite-google"], "outbound": "warp" },
  { "type": "logical", "mode": "or",
    "rules": [
      { "rule_set": ["geosite-google"] },
      { "rule_set": ["geosite-openai"] }
    ],
    "outbound": "warp" }
]
```

#### 6.6.3 xray 규칙 마이그레이션

| xray | sing-box |
|------|----------|
| 베어 문자열(`domain`의 베어 값 포함, 하위 문자열 일치) | `"domain_keyword": ["ip.sb"]` |
| `"domain": ["a.com"]` | `"domain_keyword": ["a.com"]`; 실제 접미사 일치에는 `"domain_suffix": ["a.com"]`를 사용하고 정확한 일치에는 `"domain": ["a.com"]`를 사용하세요. |
| `"domain:clstorage.net"` | `"domain_suffix": ["clstorage.net"]` |
| `"full:a.com"` | `"domain": ["a.com"]` |
| `"regexp:..."` | `"domain_regex": ["..."]` |
| `"ip": ["geoip:cn"]` | `"rule_set": ["geoip-cn"]` |
| `"domain": ["geosite:cn"]` | `"rule_set": ["geosite-cn"]` |
| `"outboundTag": "direct"` | `"outbound": "direct"` |

참고:

- **태그는 코어 간에 공유되지 않습니다**: xray 아웃바운드 태그는 sing-box 규칙에서 사용할 수 없습니다.
 sing-box "아웃바운드" 또는 "엔드포인트" 아래에 실제로 존재하는 태그를 사용하세요.
- 잘못된 아웃바운드 또는 엔드포인트 태그가 `outbound not found`를 보고합니다. 잘못된 규칙 세트 태그 보고
 `rule-set not found`.
- 직접/허용 목록 규칙을 먼저 입력하세요. 규칙이 일치하면 이후 규칙은 더 이상 적용되지 않습니다.

---

## 7. 프록시 관리

프록시 관리는 Caddy 또는 Nginx 역방향 프록시를 구성합니다.

### 7.1 프록시 유형

- **Caddy**: 자동 HTTPS, HTTP/1.1 및 HTTP/2(HTTP/3 / QUIC는 UDP 443이 layer4 라우팅과 충돌하는 것을 피하기 위해 비활성화됩니다)
- **Nginx**: 고성능, 유연한 구성

### 7.2 사이트 구성

1. "프록시 관리"를 엽니다.
2. 프록시 유형(Caddy / Nginx)을 선택합니다.
3. 사이트 추가:
   - 도메인
   - 업스트림
   - 로컬 포트
4. 저장합니다.

### 7.3 분야

- **업스트림**: 백엔드 주소(예: `127.0.0.1:8082`
- **local_port**: 로컬 수신 포트, 예: `8443`
- **portal_forward**: 포털 전달 규칙
- **subscription_forward**: 구독 전달 규칙
- **SNI 라우팅**: 인바운드가 TLS / REALITY를 사용하고 클라이언트가 항상 443에 연결하는 경우 패널은 다음을 파생합니다.
 ClientHello SNI(xray `streamSettings` 및 sing-box `tls.reality` 모두)의 443 TCP 라우팅 규칙;
 전면 프록시는 TCP만 전달하고 TLS는 여전히 코어 내부에서 종료됩니다.

---

## 8. 라우팅 리소스

라우팅 리소스는 구독 및 라우팅에 사용되는 공유 자산을 수집합니다. 페이지에는 5개의 탭이 있습니다.

| 탭 | 목적 |
|-----|---------|
| 다이렉트 및 CF 선호 도메인 | 구독 및 외부 노드에서 액세스 주소를 다시 쓸 때 사용되는 수신 직접 도메인/IP 및 Cloudflare 기본 도메인 |
| CF IP 풀 | "액세스 도메인"에서 사용되는 기본 IP 소스, 포트 및 프로브 설정 |
| WAF 전역 규칙 | 노드에 푸시된 역방향 프록시 WAF에 대한 규칙 소스 및 규칙 파일 |
| 규칙 파일 | 규칙 파일 다운로드, 예약된 동기화 및 푸시(`.srs` / `.json` / `.dat`) |
| 외부 노드 | v2ray URI / Base64 / 구독 URL에서 외부 노드 가져오기(16.3 참조) |

### 8.1 Direct & CF 선호 도메인

- **직접 액세스**: 수신 도메인 또는 IP, 기본적으로 포트 443; 노드를 바인딩하거나 전역적으로 사용하려면 비워 두세요.
- **CF 선호 도메인**: Cloudflare 선호 도메인과 포트, 연결 확인 포함;
- 둘 다 구독 및 외부 노드 재작성에 사용되는 "액세스 도메인" 후보를 제공합니다(16.3 참조).

### 8.2 CF IP 풀

- 기본 IP 소스(`bestproxy` 스타일 소스 포함), 가져오기 간격 및 포트를 유지합니다.
- "프로브 설정"은 프로브 간격, 대기 시간 임계값 및 프로브 진입 노드를 제어합니다. 로컬 프로빙
 명확하게 연결할 수 없는 IP만 필터링하고, 엔트리 노드를 선택하면 해당 노드의 회선을 시뮬레이션합니다.
- "다이얼러"는 자동(코어 설치), `singbox`, `xray` 또는 `mihomo`일 수 있습니다. 그렇지 않은 코어
 설치된 항목은 목록에 없습니다. 프로빙은 완료되면 종료되고 읽거나 읽지 않는 임시 프로세스로 실행됩니다.
 코어의 자체 구성을 작성합니다.
- "액세스 도메인"이 구체적인 값 없이 CF IP 풀을 사용하는 경우 하나의 항목(해당 포트 포함)은
 무작위로 골랐습니다.

### 8.3 WAF 전역 규칙

- WAF 규칙 소스 URL 및 동기화할 규칙 파일을 구성합니다.
- 동기화 후에는 노드로 푸시되고 노드 프록시(Nginx / Caddy)에 의해 로드됩니다.
- 전체 URL가 포함된 파일은 해당 URL에서 다운로드됩니다. 그렇지 않으면 "소스 URL + /파일 이름"이 사용됩니다.
- "일치 캐시"(`match_cache` / `config_match_cache`)는 기본적으로 활성화되어 있습니다.
 핫 규칙 일치 결과 캐시는 요청별 규칙 순회를 줄입니다. 전역 전용이며
 도메인별 재정의; 오탐지 조사 시 A/B 비교를 위해 일시적으로 비활성화
 (비활성화한다는 것은 모든 요청에 대한 전체 규칙 평가와 더 높은 CPU 사용량을 의미합니다).

### 8.4 규칙 파일(라우팅 규칙 파일)

이 탭은 패널의 규칙 파일만 다운로드하여 노드에 푸시합니다. 아무것도 효과가 없습니다
자동으로. 라우팅에 사용하려면 “Protocol center → sing-box → Routing” 항목을 참고하시기 바랍니다.
규칙 세트 및 규칙(6.6 참조).

- **규칙 저장소**: `owner/repo`(GitHub, 탐색 가능)를 입력하거나 전체 다운로드 URL를 입력하세요. 예정된
 동기화는 6/12시간마다, 매일 03:00에, 매일 자정에 또는 매주 일요일에 실행될 수 있습니다.
- **파일 추가**: 저장소 릴리스 외부의 단일 파일용(예: SagerNet `.srs` 지점)
 파일); 전체 URL를 입력하면 자동으로 다운로드됩니다.
- **파일 목록**: 파일별 '다운로드'와 함께 이름, 크기, 마지막 업데이트 및 '자동 동기화' 스위치를 표시합니다.
 상단의 "모든 노드에 푸시" 버튼은 모든 규칙 파일을 모든 노드에 푸시합니다.
- 파일은 패널의 `data/rules/`로 다운로드되고 일단 푸시되면 노드 데이터 아래에 라이브로 저장됩니다.
 디렉토리의 `rules/`; sing-box에서 `rules/<file name>`로 참조하세요.
- 형식:
  - `.srs`: sing-box 바이너리 규칙 세트, 권장되며 가장 작습니다.
  - `.json`: 소스 형식 규칙 세트; `rule_set`는 `format: "source"`를 설정해야 합니다.
  - `.dat`: xray `geosite.dat` / `geoip.dat`, sing-box는 직접 사용할 수 없습니다.

> 페이지 상단의 축소 가능한 "사용" 블록에는 참조 예제인 xray 매핑이 포함되어 있습니다.
> 테이블과 일반적인 오류.

### 8.5 외부 노드

외부 노드 가져오기 및 관리는 [16.3 외부 노드](#163-external-nodes)에서 다룹니다.

---

## 9. 가입센터

신청 센터는 노드를 클라이언트 지원 구독으로 패키징합니다. 구독 **형식**(내보낼 대상, 규칙, 볼 수 있는 계획)을 정의한 다음 구독 **엔드포인트**(공개 도메인/경로)를 정의한 다음 링크를 전달합니다.

### 9.1 구독 형식

“구독 센터” → “구독 형식”; 클라이언트 출력당 한 행:

| 필드 | 설명 |
|---|---|
| 이름 | 고유함; `?config=<name>`가 일치하고 고객 대화 상자에 표시됩니다. |
| 클라이언트 유형 | clash / shadowrocket / singbox / v2ray — 디스플레이 그룹화 전용 |
| 출력 형식 | `mihomo_yaml` (Clash/Mihomo), `singbox_json`, `xray_json`, `base64_uri`, `raw_uri` |
| 콘텐츠 유형/확장자 | 응답 헤더 및 파일 이름 |
| 플랫폼 | 승리 / 맥 / android / ios / 유니버셜; `?platform=`와 일치, 비어 있음 = 범용 |
| 제한된 계획 | 비어 있음 = 모든 계획에 사용 가능 그렇지 않으면 해당 계획만 사용할 수 있습니다. |
| 규칙 | 형식별 라우팅 규칙(Mihomo의 경우 구조화된 편집기, sing-box/Xray의 경우 규칙당 원시 JSON). **규칙은 형식 간에 변환되지 않습니다** |

참고:

- 동일한 출력 형식에 대해 여러 구성이 존재할 수 있습니다(예: `out`, `home`). 링크에서 `config=`가 있는 것을 선택하세요.
- `config`가 없으면 순서는 "구독에 바인딩된 구성 → `platform` 일치 → 기본값"입니다.
- sing-box 템플릿은 클라이언트를 위해 일반적으로 `remote`와 같은 자체 `route.rule_set`를 가질 수 있습니다.
 클라이언트가 그것을 다운로드합니다. "규칙 파일" 아래 노드의 `rules/`에 푸시된 로컬 파일만 사용할 수 있습니다.
 클라이언트 구독이 아닌 해당 노드의 코어에 연결됩니다.
- sing-box / Xray 템플릿을 생성할 때 패널은 매달린 참조(`final`, `dns.final`,
 `default_domain_resolver`, `download_detour`, 로컬 `rule_set`, 잘못된 `rule_set` 정의, 비어 있음
 `balancer` 등) 클라이언트가 시작 시 치명적인 오류로 인해 실패하지 않도록 합니다.
- 규칙의 아웃바운드 별칭은 실제 태그(`REJECT` / `blocked` → `block`, `DIRECT` →
 `direct`, 대소문자 구분); 확인할 수 없는 아웃바운드는 기본 아웃바운드 및 규칙으로 대체됩니다.
 그 위치를 유지합니다.
- `tag = proxy`가 있는 그룹은 항상 생성됩니다. sing-box에는 `selector`, `random` 밸런서는 다음과 같습니다.
 규칙의 Xray 및 `outbound: proxy`는 일치하는 `balancerTag`로 다시 작성됩니다. 규칙 편집기가 로드됩니다.
 "프록시 그룹 + 사용자 정의 아웃바운드"에서 동적으로 아웃바운드 후보 및 수동으로 입력된 태그는
 아직 검증된 상태입니다.
- sing-box / Xray 구성은 별도로 생성해야 합니다. 존재하지 않는 경우 고객 링크에 표시되지 않습니다(10.3 참조).

### 9.2 구독 엔드포인트

“구독 센터” → “구독 엔드포인트”; 공개 주소당 한 행:

| 필드 | 설명 |
|---|---|
| 도메인 | 공개 구독 도메인 |
| 경로 | 도메인 아래 접두사, 기본값 `/` |
| 활성화됨 | 링크를 구축할 때 비활성화된 끝점은 무시됩니다. |
| 설명 | 자유 텍스트 메모 |

- **여러 행(다중 도메인)**이 지원됩니다. 여러 개가 활성화되면 공급자 도메인은 현재 요청에서 사용되는 도메인을 따릅니다(9.4 참조).
- 전용 구독 포트는 루트(`/<token>`, `/sub` 없음)에서 서비스를 제공합니다. 패널 포트를 공유하는 경우 `/sub/<token>`입니다.
- 역방향 프록시는 `X-Forwarded-Proto` / `X-Forwarded-Host`를 전달해야 합니다. 그렇지 않으면 활성화된 첫 번째 엔드포인트가 사용됩니다. 패널에서 생성된 Caddy / Nginx 사이트 템플릿에는 두 헤더가 모두 포함되어 있습니다. 직접 관리하는 프록시 구성에는 수동으로 추가해야 합니다.

### 9.3 구독 링크

기본 형식: `https://<sub-domain>/<token>`(패널 포트: `https://<panel-domain>/sub/<token>`)

| 매개변수 | 설명 |
|---|---|
| `format` | 출력 형식; 별칭: `clash` → mihomo_yaml, `singbox`/`sing-box` → singbox_json, `xray` → xray_json, `raw` → raw_uri, `base64`/`v2ray` → base64_uri |
| `config` | 이름별로 구독 형식을 적용합니다. `format`와 일치해야 하며 그렇지 않으면 무시됩니다. |
| `platform` | 플랫폼별 구성 일치 (`config`가 생략된 경우) |
| `providers` | `1` / `true`: Clash 공급자 모드(`proxy-providers` + `use`); 기본 인라인 노드 |

예:

```bash
# Clash dynamic subscription (config template + providers)
https://sub.example.com/<token>?format=clash&config=out&providers=1

# sing-box / Xray (only meaningful when such a config exists)
https://sub.example.com/<token>?format=singbox_json&config=sing-box-home
https://sub.example.com/<token>?format=xray_json&config=xray-home

# generic clients / plain text
https://sub.example.com/<token>?format=base64_uri
https://sub.example.com/<token>?format=raw_uri
```

### 9.4 다중 도메인 및 공급자 모드

`providers=1`를 사용하면 모든 `proxy-providers` 항목이 구독 끝점을 다시 가리킵니다. 도메인 선택:

1. **활성화된** 엔드포인트와 일치하는 경우 **이 요청이 사용한 도메인**(`X-Forwarded-Host` 첫 번째 값, 포트 제거, 그렇지 않으면 `Host`).
2. 그렇지 않으면 **가장 낮은 ID**를 가진 활성화된 엔드포인트입니다.
3. 경로는 해당 끝점인 `https://<domain><path>/<token>?format=mihomo_proxies_yaml...`에서 옵니다.

따라서 도메인을 변경하는 경우 패널 변경이 필요하지 않습니다. 새 도메인을 통해 가져오는 클라이언트는 공급자 내에서 새 도메인을 가져오고 활성화된 상태로 남아 있는 이전 도메인은 더 이상 하드 코딩되지 않습니다. 클라이언트가 이전 공급자 URL를 캐시한 경우 구독을 업데이트합니다(또는 공급자 캐시를 지우고 클라이언트를 다시 시작).

추가 참고 사항:

- 공유 형식(`config_json`) 내부의 `proxy-providers`는 무시됩니다(교차 계획 누출 방지). 공급자는 계획 노드 그룹 및 외부 소스에서 제공됩니다.
- 내부 및 외부 공급자 캐시 경로는 항상 "내부 공급자 기본값 → 캐시 경로 템플릿"(기본값 `./providers/{key}.yaml`)에서 파생됩니다. 외부 소스는 자체적으로 설정할 수 없습니다.
- 레거시 예약 공급자 `subscription-nodes`는 더 이상 내보내지지 않습니다. "모든 노드"를 사용하는 그룹은 구체적인 공급자로 확장되거나 계획에 없는 경우 인라인 노드로 확장됩니다.

---

## 10. 고객관리

고객 관리는 최종 사용자를 관리합니다.

### 10.1 고객 추가

1. "고객 관리"를 엽니다.
2. “고객 추가”를 클릭하세요.
3. 작성:
   - 이메일
   - 비밀번호
   - 계획
   - 트래픽 할당량
   - 만료 시간
4. 구하다.

### 10.2 고객 조치

- **수정**: 고객 세부정보 변경
- **일시 중지**: 서비스를 일시 중지합니다.
- **재개**: 정지된 고객을 복원합니다.
- **트래픽 재설정**: 사용된 트래픽 재설정
- **비밀번호 재설정**: 고객의 비밀번호를 재설정합니다.
- **갱신**: 만료일 연장
- **업그레이드/다운그레이드**: 요금제 변경
- **삭제**: 고객 삭제

### 10.3 구독 세부정보

고객 관리 → “구독” 열 → “상세정보”. 대화 상자에는 고객의 계획에 사용 가능한 모든 형식에 대한 링크가 나열됩니다.

- **Clash / Mihomo**: 2개 행 — 인라인 노드 및 `providers=1`(프록시 제공자), 둘 다 `config=<name>`를 전달합니다.
- **Sing-box / Xray**: 해당 형식의 구성이 존재하고 계획에 표시되는 경우에만 표시됩니다. 링크에는 `config=<name>`가 포함되어 있습니다.
- **Base64 URI / 원시 URI**: 항상 표시됩니다(`config` 없음).
- 각 행에는 자체 복사 버튼이 있습니다. 행을 클릭하면 아래 QR 코드가 해당 형식으로 전환됩니다.

따라서 Clash 구성만 사용하면 대화 상자에 Clash + Base64 + Raw가 표시됩니다. sing-box / Xray 링크는 해당 구성이 생성된 후에만 나타납니다.

동일한 대화 상자에는 트래픽 사용량, 주문 내역 및 거래 내역도 표시됩니다.

---

## 11. 계획 관리

계획 관리는 귀하가 판매하는 계획을 구성합니다.

### 11.1 계획 템플릿

1. "계획 관리" → "계획 템플릿"을 엽니다.
2. 템플릿을 추가합니다.
   - 템플릿 이름
   - 카테고리(표준/프리미엄/기업/평가판)
   - 자동 갱신
   - 평가판 설정
3. 저장합니다.

### 11.2 계획 변형

1. 템플릿 아래에 변형을 추가합니다.
   - 변형 이름(예: 월별, 연간)
   - 가격(센트)
   - 기간(일)
   - 트래픽(GB, `0` = 무제한)
   - 동시성 제한
   - 속도 제한(Mbps)
2. 저장합니다.

### 11.3 트래픽 추가 기능

1. "트래픽 추가 기능"을 엽니다.
2. 추가 기능을 추가하세요:
   - 이름
   - 가격
   - 교통
   - 만료 모드(일수 연장/계획과 동일)
3. 저장합니다.

### 11.4 쿠폰

1. "쿠폰"을 엽니다.
2. 쿠폰 추가:
   - 코드
   - 종류(비율/정액)
   - 할인 가치
   - 사용량 한도
   - 유효기간
   - 적용 가능한 계획
3. 저장합니다.

---

## 12. 충전관리

충전 관리는 충전 사이트, 주문 및 충전 카드를 처리합니다.

> **결제 통합**
>
> 패널은 **결제 기관**(Alipay, WeChat Pay, Stripe, 은행 등)과 직접 통합되지 않습니다. **중간 결제 플랫폼**에만 연결되며 해당 결제 기관과 통합되어 계약, 인수 및 정산을 처리하고 [charge 콜백 API](api/charge-callback.md)를 통해 결제 결과를 다시 보고합니다.
>
> 중간 결제 플랫폼은 **본 프로젝트의 일부가 아닙니다. 운영자는 직접 개발해야 합니다(또는 자체 시스템을 사용해야 함)**. 배포가 완료되면 "충전 사이트"에 등록하고 도메인 확인을 완료하여 재충전 진입점으로 사용하세요. 여기서 충전 사이트는 결제 기관 자체가 아닌 중간 플랫폼입니다. 결제 자격, 수수료, 정산 및 규정 준수는 운영자, 결제 기관 및 중개 플랫폼이 합의하고 부담합니다.

### 12.1 충전소

1. "충전 관리" → "충전 사이트"를 엽니다.
2. 사이트 추가:
   - 사이트 이름
   - 해당 중개 플랫폼에서 지원하는 결제수단(Alipay, WeChat, USDT, Stripe 등 결제기관과의 통합을 통해 제공)
   - 통화
   - 환율
3. 저장;
4. 사이트를 확인하세요(HTTP 파일 또는 DNS 확인).

### 12.2 주문 재충전

- 모든 재충전 주문 보기
- 상태별 필터링(대기 중, 지불됨, 실패함, 만료됨, 환불됨)
- 주문 세부정보 보기
- 관리자 환불 처리

### 12.3 충전카드

1. "충전 카드"를 엽니다.
2. 카드 추가:
   - 유형(금액, 계획, 교통 부가 기능)
   - 액면가 / 계획
   - 만료 시간
3. 또는 대량으로 생성:
   - 수량(최대 1000개)
   - 카드번호 앞자리
   - 카드번호 형식
4. CSV로 내보냅니다.

### 12.4 잔액 관리

- 고객 잔액 보기
- 잔액 조정(증가/감소/설정)
- 동결금액 조정
- 거래 내역 보기

### 12.5 리셀러

- 리셀러 추가
- 수수료율 설정
- 누적수수료 보기
- 수수료 인출

---

## 13. 티켓 관리

티켓 관리는 고객 지원 요청을 처리합니다.

### 13.1 티켓 목록

- 모든 티켓 보기
- 상태별 필터링(열림, 응답함, 닫힘)
- 카테고리별로 필터링(기술, 청구, 계정, 기타)
- 우선순위별로 필터링(낮음, 보통, 높음, 긴급)

### 13.2 티켓 조치

- **보기/답장**: 티켓을 열고 답장을 보내세요.
- **할당**: 관리자에게 티켓을 할당합니다
- **닫기**: 해결된 티켓을 닫습니다.

---

## 14. 인증서 관리

인증서 관리는 TLS 인증서를 처리합니다.

### 14.1 인증서 발급

1. "인증서 관리"를 엽니다.
2. "인증서 발급"을 클릭하세요.
3. 작성:
   - 도메인 목록(한 줄에 하나씩)
   - 인증서 유형(도메인/IP)
   - 키 유형(ECC/RSA)
   - 검증 방법(DNS-01 / HTTP-01)
   - DNS 공급자
   - 발급 위치(패널 호스트/노드)
4. "발행"을 클릭하세요.

### 14.2 인증서 작업

- **갱신**: 수동으로 갱신
- **배포**: 노드에 배포
- **배포**: 연결된 모든 노드에 푸시
- **이전**: 이전 인증서에서 새 인증서로 참조 이동
- **다운로드**: `cert` / `fullchain` / `privkey` 다운로드

### 14.3 인증서 업로드

1. “인증서 업로드”를 클릭하세요.
2. 방법을 선택하세요:
   - 내용을 붙여넣으세요
   - 파일 업로드
3. 인증서 체인과 개인 키를 업로드합니다.
4. 구하다.

### 14.4 EAB 구성

- Google 공용 CA / ZeroSSL에 대한 EAB 자격 증명 관리
- 자동으로 EAB 획득
- 발급시 자동으로 첨부

---

## 15. 트래픽 할당량

트래픽 할당량은 각 노드의 월별 트래픽을 모니터링합니다.

### 15.1 할당량 목록

- 모든 노드의 할당량 설정 보기
- 할당량 크기, 사용된 트래픽 및 남은 허용량 표시
- 사용량 비율 표시

### 15.2 사이클 사용법

- 노드당 주기 사용량 세부정보 보기
- 인바운드/아웃바운드 트래픽 표시
- 주기 시작/종료 시간 표시

### 15.3 로그 재설정

- 트래픽 재설정 기록 보기
- 재설정 주기, 트리거 소스 및 연산자 표시

---

## 16. 체인으로 연결된 프록시

연결된 프록시는 다중 종료 링크를 구성합니다.

### 16.1 링크 생성

1. "체인으로 연결된 프록시"를 엽니다.
2. “새 링크”를 클릭하세요.
3. 진입 노드를 선택하세요.
4. 인바운드 항목을 선택하십시오.
5. 종료 노드 선택(여러 개 허용)
6. 각 출구에 대한 인바운드를 선택하십시오.
7. 우선순위/가중치 설정
8. 확인하다.

### 16.2 링크 관리

- 토폴로지 개요 보기
- 링크 편집(우선순위, 가중치, 아웃바운드 태그)
- 링크 삭제
- 건강상태를 확인하세요

### 16.3 외부 노드

- 외부 노드 가져오기(v2ray URI, Base64 URI, 구독 URL)
- 외부 항목 등록
- 외부 노드 관리(개별 수정/삭제)
- 액세스 도메인: CDN 가속 구성 / 노드당 기본 IP(직접 도메인, CF 선호)
 도메인, CF IP 풀, 사용자 정의). 구독으로 전달된 서버/포트만 다시 작성됩니다.
 SNI/호스트는 가져온 상태로 유지됩니다. 비활성화되면 가져온 주소/포트가 사용됩니다.
- 특정 값 없이 "CF 선호 도메인/CF IP 풀"을 선택한 경우 한 항목이 선택됩니다.
 풀에서 무작위로(해당 포트 포함) "직접/맞춤형"에는 명시적인 값이 필요합니다. 그렇지 않으면
 노드의 자체 주소로 돌아갑니다.

---

## 17. 온라인 사용자

온라인 사용자는 실시간 온라인 상태를 보여줍니다.

### 17.1 그것이 보여주는 것

- 온라인 구독
- 온라인 장치/IP
- 각 구독에 대한 노드, IP 및 연결 시간
- 실시간 업링크/다운링크 속도
- 총 업링크 / 총 다운링크 / 총 트래픽

> 총 업링크, 총 다운링크 및 총 트래픽은 **지속적인 온라인 세션**을 통해 누적됩니다.
> 단기 재연결, 코어 재시작 및 카운터 재설정으로는 해당 내용이 지워지지 않습니다. 60 이후에만
> 새 보고서가 없으면 새 세션이 0부터 시작됩니다. 이건 다른 척도야
> 코어 자체의 "이번 시작 이후의 트래픽" 카운터에서 사용자가 실제로 무엇을 하는지 더 잘 반영합니다.
> 현재 세션 중에 사용됩니다.

### 17.2 필터링

- 노드별로 필터링
- 이메일/고객/노드/IP로 검색

---

## 18. 사용자 관리

사용자 관리는 패널 사용자를 처리합니다.

### 18.1 역할

| 역할 | 설명 |
|------|-------------|
| 최고 관리자(`superAdmin`) | 최고 권한 |
| 관리자(`admin`) | 관리 권한 |
| 운영자(`operator`) | 운영 권한 |
| 뷰어 (`viewer`) | 읽기 전용 |

### 18.2 사용자 추가

1. "사용자 관리"를 엽니다.
2. “사용자 추가”를 클릭하세요.
3. 작성:
   - 사용자 이름
   - 이메일
   - 비밀번호
   - 역할
4. 저장합니다.

### 18.3 사용자 조치

- **수정**: 사용자 세부정보 변경
- **삭제**: 사용자 제거

---

## 19. 포털 사이트

포털 사이트는 클라이언트 포털을 구성합니다.

### 19.1 일반사항

- 사이트 이름
- 사이트 설명
- 공개 등록
- 포털 경로
- 포털 포트

### 19.2 외관

- 사이트 제목
- 로고 URL
- 기본, 배경, 텍스트 및 강조 색상
- 글꼴

### 19.3 탐색

- 탐색 페이지 추가
- 페이지 이름, 아이콘, 외부 링크 구성
- 탐색 항목 관리

### 19.4 모듈

- 모듈 추가/수정
- 모듈 제목, 설명 및 아이콘 구성
- 마크다운, 서식 있는 텍스트 및 사용자 정의 HTML 지원

### 19.5 언어

- 언어 추가
- 사본 관리
- JSON 가져오기/내보내기

### 19.6 템플릿

- 템플릿 전환
- HTML/CSS/JS 사용자 정의
- 기본 템플릿으로 재설정

---

## 20. 시스템 설정

시스템 설정은 패널 매개변수를 구성합니다.

### 20.1 패널

- 수신 주소 및 포트
- 데이터 디렉터리
- 시간대
- 구독 포트
- 수출 한도
- 공개 등록
- 패널 모드(전체/관리 전용)

### 20.2 보안

- 암호화 키 관리
- JWT 키 관리
- IP 블랙리스트
- Passkey 허용 목록에 있는 도메인

### 20.3 OAuth

- GitHub OAuth
- Google OAuth
- 콜백 URL 구성

### 20.4 알림

- 이메일 알림(SMTP)
- Telegram 알림
- 알림 템플릿
- 모니터링 설정

### 20.5 백업 및 복원

- 백업 만들기
- 백업 목록
- 백업 다운로드
- 백업 복원
- 예약된 백업
- 원격 스토리지(FTP / S3 / WebDAV)

### 20.6 자동 차단 해제

- 자동 차단 활성화
- 쿨다운
- 적용 가능한 상태

---

## 21. 운영 콘솔

운영 콘솔은 구성 스냅샷을 관리하고 작업을 적용합니다.

### 21.1 구성 스냅샷

- 스냅샷 만들기
- 스냅샷 나열
- 스냅샷으로 롤백
- 스냅샷 삭제

### 21.2 작업 적용

- 최근 채용정보 보기
- 지금 구성 적용(DB → 런타임)
- 사전 확인(포트 충돌 및 구성 JSON 확인)
- 작업 취소
- 작업 재시도

### 21.3 노드 검증

- 노드를 선택하고 유효성을 검사합니다.
- 검증 결과 보기
- 모든 노드 검증

### 21.4 비상정지

- 프록시 긴급 중지(빈 인바운드 구성 작성)
- 모든 인바운드 트래픽을 즉시 중지

---

## 22. 데이터베이스 모니터링

데이터베이스 모니터링은 데이터베이스 상태를 보여줍니다.

### 22.1 보여주는 내용

- 데이터베이스 엔진
- 데이터베이스 크기
- 연결성
- 대기 시간
- 연결 풀 상태
- 캐시 통계
- 쿼리 통계

### 22.2 지원되는 엔진

- SQLite
- MySQL
- PostgreSQL

---

## 23. 감사 로그

감사 로그는 작업을 기록합니다.

### 23.1 보여주는 내용

- 카테고리
- 이벤트 유형
- 결과
- 소스 IP
- 사용자
- 세부정보

### 23.2 노드 운영 로그

- 노드 운영 기록 보기
- 노드별로 필터링

---

## 24. 로그 관리

로그 관리에는 시스템 로그가 표시됩니다.

### 24.1 로그 유형

- 패널 로그
- 노드 로그
- 핵심 로그
- 프록시 로그

### 24.2 액션

- 로그 보기
- 로그 다운로드
- 로그 정리

---

## 25. 운영 확장

"운영 확장"(`/client-ext`)은 추천 마케팅, 커미션 및 인출을 구성합니다.
"법률 문서"(`/legal-documents`)는 포털 측 계약을 유지합니다.

### 25.1 추천 및 커미션

- **리베이트 구성**: 리베이트 비율(초대 대상자의 첫 번째 결제 비율, `0` = 비활성화됨), 커미션 모드(첫 번째 구매만/모든 구매), 최소 출금 금액(커미션 잔액이 도달해야 출금을 요청할 수 있음)
- **초대 코드**: 활성화된 경우 등록하려면 초대 코드가 필요합니다. 코드 길이, 만료일 및 최대 사용 설정(`0` = 무제한) 및 코드 목록 생성/보기
- **철회 검토**: 보류/승인/거부별로 필터링합니다. 처리 메모를 추가하고 지급 방법과 지갑 주소를 기록하세요.
- **추천 기록**: 초대자, 초대 대상자, 커미션 금액 및 커미션 상태

### 25.2 법률 문서

- 실시간 미리보기를 통해 "개인정보 보호정책" 및 "서비스 약관"을 별도로 유지 관리합니다. 로케일별로 하나의 Markdown 문서를 제공합니다.
- 각 문서는 내장 페이지 대신 외부 URL를 사용할 수 있습니다. 비어 있으면 내장 페이지 URL가 사용됩니다(페이지에서 복사 가능).
- 포털에는 현재 로케일의 버전이 표시되며 누락된 경우 기본 로케일로 대체됩니다.

---

## 26. FAQ

### 26.1 패널이 시작되지 않습니다

- 해당 포트가 이미 사용 중인지 확인
- `.env`가 유효한지 확인하세요.
- 로그 확인: `journalctl -u corefusion -f`; systemd, `tail -f <install-dir>/data/logs/panel.log` 없이

### 26.2 노드 오프라인

- 에이전트가 실행 중인지 확인하세요: `systemctl status corefusion-agent`
- 네트워크 연결 확인
- 방화벽 규칙 확인(포트 `18443`)

### 26.3 인증서 발급 실패

- DNS 해상도 확인
- 포트 80에 연결할 수 있는지 확인하세요.
- DNS 공급자 구성을 확인하세요.
- 인증서 로그 확인

### 26.4 구독이 작동하지 않습니다

- 구독 링크를 확인하세요
- 노드가 온라인 상태인지 확인하세요.
- 핵심 프로세스가 실행 중인지 확인
- 방화벽 규칙 확인

### 26.5 재충전 콜백 실패

- 콜백 URL를 확인하세요.
- 사이트 키가 일치하는지 확인하세요.
- IP 허용 목록 확인
- 주문 로그를 확인하세요

### 26.6 포털 페이지에 접근할 수 없습니다

- 포털 포트가 열려 있는지 확인하세요.
- 포털 경로 구성을 확인하세요.
- **중요**: 역방향 프록시가 포털 도메인을 기본 패널 포트가 아닌 **포털 엔진 포트**로 전달하는지 확인하세요. 잘못된 포트로 전달하면 포털 대신 빈 페이지나 패널이 표시됩니다. 프록시 사이트 기록에서 `upstream` / `local_port`를 확인하세요.
- `http://<portal-domain>:<portal-port>`를 직접 열어 포털 엔진 자체를 확인하세요.
- 역방향 프록시 구성 확인
- 포털 로그 확인

---

## 부록

### A. 기본 포트

| 항구 | 목적 |
|------|---------|
| 설정 마법사에 의해 할당됨(기본적으로 임의의 여유 포트) | 관리자 패널(변경 가능) |
| 18443 | 에이전트 gRPC(변경 가능) |
| 0(기본적으로 비활성화됨) | 전용 구독 엔진 포트(비어 있거나 `0`가 기본 포트를 공유함) |
| 0(기본적으로 비활성화됨) | 전용 포털 엔진 포트(비어 있거나 `0`가 기본 포트를 공유함) |
| 80 / 443 | 리버스 프록시(Caddy / Nginx) HTTP / HTTPS |

### B. 기본 경로

| 경로 | 목적 |
|------|---------|
| `/` | 관리자 패널(전용 포털 엔진이 구성되지 않은 경우 포털은 `/`에도 있음) |
| `/sub/:token` | 구독 가져오기(전용 구독 포트가 구성되지 않은 경우) |
| `/p` | 포털 페이지(전용 포털 포트가 구성되지 않은 경우) |
| `/:token` | 구독 가져오기(전용 구독 포트 사용, `/sub` 접두사 없음) |

### C. 명령줄 도구

패널 바이너리는 다음 하위 명령을 지원합니다(인수 없이 서버를 시작함).

```bash
# Start the panel (same as ./corefusion server)
./corefusion

# Run database migrations
./corefusion migrate

# Non-interactive init / reset init state (see 3.4, 3.5)
./corefusion init --help
./corefusion reset-init

# Offline administration (when you cannot sign in)
./corefusion admin reset-password -u admin -p 'new-password'
./corefusion admin 2fa reset -u admin
./corefusion admin list

# Version / help
./corefusion version
./corefusion help
```

> systemd의 경우 `systemctl restart corefusion` 및 `journalctl -u corefusion -f`를 사용하십시오. 없이
> systemd(컨테이너 및 유사), 이전 프로세스를 중지하고 `nohup`로 다시 시작하면 로그는 다음과 같습니다.
> `data/logs/panel.log`에서.

---

## v0.13.0-rc.1 변경 사항

- **패널 온라인 업그레이드**: 「시스템 설정 → 패널 업데이트」에서 버전 선택(`pre`는
  사전 릴리스), 업데이트, 작업 기록, 이전 바이너리로 롤백을 지원.
- **Agent 온라인 업그레이드**: 노드 상세의 「Agent 업데이트」에서 패널 로컬 바이너리 또는
  GitHub Releases 사용. 구버전에서 업그레이드한 노드는 첫 연결 시
  `update.public_key`(0600)를 자동으로 받고 이후 ed25519 서명을 검증.
- **WAF 동기화**: 마지막 동기화 시각을 `waf.github_source.last_sync`에 저장하며 재시작해도
  타이머가 초기화되지 않음. `0 3 * * *`은 `panel.timezone`(기본 Asia/Shanghai) 기준 03:00에
  실행되고, 놓친 실행은 시작 약 15초 후 보충.
- **WAF 화이트리스트**: 패널 포워드 도메인은 `whiteurl.rule`을 자동 유지. URL/인자/POST
  검사는 관리 API에만 건너뛰므로 `http://127.0.0.1:18081` 같은 결제 사이트 저장이 SSRF
  규칙에 막히지 않음.
- **할당량 정책**: 갱신 시 누적 / 만료 시 초기화 / 갱신 시 재설정(변형은 플랜 상속 가능).
  「월간」은 매시간 확인 후 사용량을 0으로.
- **결제 사이트**: 활성/비활성 전환이 키나 결제 수단을 바꾸지 않음. 충전 주문은 유형별
  필터와 CSV 내보내기 지원.
- **포털**: 「충전」은 활성 결제 사이트를 통한 온라인 결제와 카드 사용을 지원하며, 사이트가
  없으면 카드만 표시.

---

> **문서 버전**: v1.2
> **최종 업데이트**: 2026-11-20
> **적용 대상**: CoreFusion v0.13.0-rc.1
