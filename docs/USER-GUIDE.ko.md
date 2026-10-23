<!-- source: USER-GUIDE.md | locale: ko | updated: 2026-10-23 -->
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
- **프록시**: 캐디 / Nginx 리버스 프록시
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
4. `SAFE_PATHS=data`가 포함된 자리 표시자 `.env`만 작성합니다(데이터베이스 구성 없음). 따라서 첫 번째 시작은 설정 마법사로 들어갑니다(전체 `.env`는 마법사에 의해 생성됩니다).
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

### 2.3 버전 고정/미러/오프라인 설치

```bash
# Pin a version
sudo bash install.sh --version v0.13.0-alpha.2

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
sudo bash install.sh --source --version v0.13.0-alpha.2
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
| `-v, --version <TAG>` | 패널 버전(예: `v0.13.0-alpha.2`; 기본 `latest` |
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
| `--uninstall` | 서비스를 중지하고 프로그램을 제거합니다(데이터 및 `.env` 유지). |
| `--purge` | `--uninstall`를 사용하여 데이터 디렉터리와 `.env`도 제거합니다. |

관련 환경 변수:

| 변수 | 목적 |
|----------|---------|
| `CF_GO_VERSION` | 소스 빌드에 필요한 Go 버전 재정의 |
| `CF_NODE_MAJOR` | Node 주 버전 재정의(기본값 22) |
| `CF_GO_MIRROR` | 툴체인 다운로드 미러로 이동 |
| `CF_SKIP_DEPS` | 종속성 설치를 건너뛰려면 에이전트 설치 스크립트에서 `1`로 설정하세요. |

### 2.6 디렉토리 레이아웃

```text
/opt/corefusion/
├── corefusion                 # panel binary
├── corefusion.bak             # previous version (when upgrading)
├── .env                       # environment file (0600, generated by the setup wizard)
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

> 전체 구성(`CF_DB_DSN`가 포함된 `.env`)이 이미 존재하는 경우 설치 프로그램은 이를 덮어쓰지 않으며 마법사가 다시 나타나지 않습니다.
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

#### 기타 공통변수

| 변수 | 설명 |
|----------|-------------|
| `JWT_SECRET` | 로그인 토큰의 서명 키 누락된 경우 처음 시작 시 생성됨 |
| `CF_SECRET_ENCRYPTION_KEY` | 민감한 설정을 암호화하기 위한 키(AES-256-GCM) 누락 시 생성됨 |
| `CF_INITIAL_ADMIN_PASSWORD` | 초기 관리자 비밀번호(선택 사항, 무작위이며 기본적으로 한 번 인쇄됨) |
| `CF_NETWORK_TRAFFIC_ACCOUNTING` | iptables 기반 트래픽 계정을 비활성화하려면 `0`로 설정합니다(기본적으로 활성화됨). |
| `CF_NETWORK_TRAFFIC_TABLE` | 트래픽 계산에 사용되는 iptables 테이블, 기본값 `corefusion` |

> 실제 클라이언트 IP를 확인하기 위한 신뢰할 수 있는 역방향 프록시는 더 이상 환경 변수를 통해 구성되지 않습니다.
> **설정 → 네트워크**에 `panel.trusted_proxies`로 저장됩니다.

> `.env`에는 비밀이 포함되어 있습니다. 패널은 시작 시 `0600`에 대한 권한을 강화합니다. 절대 커밋하지 마세요.

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
| 5. 프록시 | 없음 / Nginx / 캐디 |
| 6. 사이트 및 도메인 | 포트, 웹 경로, 관리자 계정, 통화, 시간대, 도메인 |
| 7. HTTPS 인증서 | 비활성화됨 / 업로드 / ACME |
| 8. 요약 | `.env`를 검토하고 다운로드한 후 저장하고 다시 시작하세요. |

#### 1단계: 데이터베이스

- **SQLite(로컬 파일, 권장)**: `<data-dir>/sqlite/app.db`를 사용하려면 경로를 비워 두세요.
- **MySQL / PostgreSQL**: 호스트, 포트, 사용자, 비밀번호, 데이터베이스 이름; PostgreSQL는 `disable` / `require` / `verify-full`도 지원합니다.
- "연결 테스트"를 클릭하여 확인하세요. 저장도 테스트합니다. 누락된 데이터베이스가 자동으로 생성됩니다(계정에 `CREATE`가 없는 경우 수동으로 생성).
- DSN(비밀번호 포함)은 `.env`에 기록되기 전에 암호화됩니다.

#### 2단계: 데이터 디렉터리

데이터베이스, 인증서, 로그, 핵심 바이너리 등을 저장합니다. 이것은 `.env`의 `SAFE_PATHS`입니다. 기본값은 `data`입니다.
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
- **Nginx / 캐디**: 패널은 역방향 프록시 및 인증서 호스팅을 관리합니다. 하나를 선택하세요.

프록시를 선택하면 패널은 일반적으로 `127.0.0.1`에서만 수신하고 프록시는 외부 세계에 서비스를 제공합니다.

#### 6단계: 사이트 및 도메인

| 필드 | 설명 |
|-------|-------------|
| 청취 주소 | 자동 파생: 프록시 → `127.0.0.1`; 인증서가 있는 프록시 없음 → `0.0.0.0`; 그렇지 않으면 → `127.0.0.1` |
| 패널 포트 | 기본적으로 임의의 무료 포트; 원하는 경우 변경하세요(예: 8080). |
| 구독 포트 | 선택사항; `0`는 패널 포트를 공유합니다. |
| 포털 포트 | 선택 과목; `0`는 패널 포트를 공유합니다. |
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
- **챌린지**: `HTTP-01`(포트 80 필요) 또는 `DNS-01`(와일드카드, 포트 80 없음, DNS 공급자를 선택하고 자격 증명 입력)
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
| `--data-dir` | 데이터 디렉토리 기본값은 `SAFE_PATHS` 또는 `./data`입니다. |
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

1. 노드를 추가한 후 "Bootstrap 명령"을 클릭하십시오.
2. 설치 명령을 복사하십시오.
3. 원격 노드에서 루트로 실행하십시오.

   ```bash
   curl -fsSL https://your-panel/install.sh | bash
   ```

4. 에이전트는 자동으로 다운로드, 등록 및 시작됩니다.

### 5.4 노드 세부정보

노드를 클릭하여 세부정보 페이지를 엽니다.

- **개요**: 기본 정보, 에이전트 상태, 인증서 상태
- **프로토콜 센터**: 핵심 프로세스 구성
- **프록시**: 캐디 / Nginx 구성
- **핵심 프로세스**: 시작/중지/다시 시작
- **워프**: 워프 구성
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

- **로그**: 접속 로그, 오류 로그, 출력 모드
- **API**: gRPC API 수신 주소, 서비스 목록
- **DNS**: DNS 서버, 쿼리 전략, 정적 호스트
- **라우팅**: 라우팅 규칙, 로드 밸런싱, VLESS 라우팅
- **인바운드**: 인바운드 목록, 프로토콜 설정, 전송, TLS
- **아웃바운드**: 아웃바운드 목록, 프로토콜 설정, 전송
- **전송**: WebSocket, gRPC, XHTTP, TUN 등
- **TLS**: 인증서, ALPN, uTLS 지문, ECH
- **Reality**: 대상 관리, 키 쌍, 짧은 ID
- **상태 점검**: URL 프로브, 간격, 동시성
- **FakeDNS**: IP 풀, 풀 크기
- **측정항목**: Prometheus 측정항목 리스너
- **정책**: 연결 시간 초과, 버퍼, 통계
- **지역 데이터**: 자동 업데이트, 리소스 파일
- **버전 제한**: 최소/최대 버전

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
   - 미호모 YAML
   - sing-box JSON
   - Xray JSON
3. 링크를 복사하거나 QR 코드를 생성하세요.

---

## 7. 프록시 관리

프록시 관리는 Caddy 또는 Nginx 역방향 프록시를 구성합니다.

### 7.1 프록시 유형

- **캐디**: 자동 HTTPS, HTTP/2, HTTP/3
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

---

## 8. 라우팅 리소스

라우팅 리소스는 구독 라우팅 규칙을 관리합니다.

### 8.1 특징

- 라우팅 규칙 관리
- 라우팅 정책 구성
- 다양한 라우팅 형식 지원

### 8.2 사용법

1. "라우팅 리소스"를 엽니다.
2. 라우팅 규칙을 추가합니다.
3. 규칙 이름, 유형 및 조건을 설정합니다.
4. 저장하고 적용하세요.

---

## 9. 가입센터

신청 센터는 구독 링크와 엔드포인트를 관리합니다.

### 9.1 구독 형식

1. "구독 센터" → "구독 형식"을 엽니다.
2. 형식 추가:
   - 형식 이름
   - 출력 형식(Base64, YAML, JSON 등)
   - 포함된 프로토콜
3. 저장합니다.

### 9.2 구독 엔드포인트

1. "구독 센터" → "구독 엔드포인트"를 엽니다.
2. 엔드포인트를 추가합니다.
   - 엔드포인트 이름
   - 청취 주소
   - 경로
   - 관련 형식
3. 저장합니다.

### 9.3 구독 링크

- `http://your-domain/sub/:token`에서 구독 가져오기
- 형식을 선택하려면 `?format=yaml`와 같은 매개변수를 사용하세요.

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
4. 저장합니다.

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

고객을 클릭하면 다음 내용을 볼 수 있습니다.

- 구독 링크
- 교통 사용량
- 주문 내역
- 거래 내역

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
2. 구하다.

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

### 12.1 충전소

1. "충전 관리" → "충전 사이트"를 엽니다.
2. 사이트 추가:
   - 사이트 이름
   - 지원되는 결제 수단(Alipay, WeChat, USDT, Stripe 등)
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
- 외부 노드 관리

---

## 17. 온라인 사용자

온라인 사용자는 실시간 온라인 상태를 보여줍니다.

### 17.1 그것이 보여주는 것

- 온라인 구독
- 온라인 장치/IP
- 온라인 노드
- 실시간 환율
- 총 트래픽

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
| 관리자 (`admin`) | 관리 권한 |
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
- 공개등록
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
- 데이터 디렉토리
- 시간대
- 구독 포트
- 수출 한도
- 공개등록
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

### 24.2 조치

- 로그 보기
- 로그 다운로드
- 로그 정리

---

## 25. 운영 확장

작업 확장은 선택적 확장을 구성합니다.

### 25.1 특징

- 맞춤 확장
- 확장 구성
- 확장 관리

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

- 콜백 URL 확인
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
| 80 / 443 | 리버스 프록시(캐디/Nginx) HTTP/HTTPS |

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

# Offline administration (when you cannot sign in)
./corefusion admin reset-password -u admin -p 'new-password'
./corefusion admin 2fa reset -u admin
./corefusion admin list

# Version / help
./corefusion version
./corefusion help
```

> systemd 장치가 없으면 이전 프로세스를 중지하고 `nohup`로 다시 시작하여 다시 시작합니다. 로그는 `<install-dir>/data/logs/panel.log`에 있습니다.

---

> **문서 버전**: v1.0
> **최종 업데이트**: 2026-10-23
> **적용 대상**: CoreFusion v1.0+
