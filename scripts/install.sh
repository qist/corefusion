#!/usr/bin/env bash
#
# CoreFusion 面板多环境安装脚本
#
# 覆盖场景：
#   - 发行版：Debian/Ubuntu、RHEL/CentOS/Rocky/Alma/Fedora、Alpine、Arch、openSUSE
#   - 服务管理：systemd；无 systemd（容器 / OpenRC）自动回落 nohup
#   - CPU 架构：amd64(x86_64)、arm64(aarch64)；源码模式另支持 armv7/armv6/386/ppc64le/s390x/riscv64/loong64
#   - 安装方式：GitHub Release 二进制（默认）或源码编译（--source，按环境自动选择 Go/Node 版本）
#   - 安装完成后自动启动，并打印初始化向导地址、初始化令牌与数据目录
#
# 用法：
#   curl -fsSL https://raw.githubusercontent.com/qist/corefusion/main/scripts/install.sh | sudo bash
#   sudo bash scripts/install.sh --version v0.13.0-alpha.2
#   sudo bash scripts/install.sh --source --version v0.13.0-alpha.2
#   sudo bash scripts/install.sh --arch arm64 --local ./dist/corefusion-linux-arm64
#   sudo bash scripts/install.sh --uninstall
#
# 常用参数见 --help。
# 依赖版本可通过环境变量覆盖：CF_GO_VERSION、CF_NODE_MAJOR。

set -Eeuo pipefail

APP_NAME="corefusion"
REPO="qist/corefusion"
DEFAULT_DIR="/opt/corefusion"
DEFAULT_NODE_MAJOR="22"

# ── 参数默认值 ────────────────────────────────────────────────
INSTALL_DIR="$DEFAULT_DIR"
VERSION="latest"
ARCH=""
MODE="binary"          # binary | source
EXPLICIT_SOURCE=0
LOCAL_BINARY=""
MIRROR=""              # GitHub 加速前缀，如 https://ghproxy.net
GIT_REPO="https://github.com/${REPO}.git"
GIT_REF=""
SKIP_DEPS=0
START_SERVICE=1
UNINSTALL=0
PURGE=0
CPU_QUERY=0

# ── 颜色与日志 ───────────────────────────────────────────────
if [ -t 1 ] && [ "${NO_COLOR:-}" = "" ]; then
	C_RESET=$'\033[0m'; C_RED=$'\033[31m'; C_GREEN=$'\033[32m'
	C_YELLOW=$'\033[33m'; C_BLUE=$'\033[36m'; C_BOLD=$'\033[1m'
else
	C_RESET=""; C_RED=""; C_GREEN=""; C_YELLOW=""; C_BLUE=""; C_BOLD=""
fi

log()  { printf '%s[corefusion]%s %s\n' "$C_BLUE" "$C_RESET" "$*"; }
ok()   { printf '%s[  ok  ]%s %s\n' "$C_GREEN" "$C_RESET" "$*"; }
warn() { printf '%s[warn]%s %s\n' "$C_YELLOW" "$C_RESET" "$*" >&2; }
die()  { printf '%s[error]%s %s\n' "$C_RED" "$C_RESET" "$*" >&2; exit 1; }
step() { printf '\n%s==> %s%s\n' "$C_BOLD" "$*" "$C_RESET"; }

on_error() {
	local code=$? line=$1
	printf '%s[error]%s 第 %s 行执行失败（退出码 %s）\n' "$C_RED" "$C_RESET" "$line" "$code" >&2
	exit "$code"
}
trap 'on_error $LINENO' ERR

usage() {
	cat <<'EOF'
CoreFusion 面板多环境安装脚本

用法：
  install.sh [选项]

安装选项：
  -d, --dir <PATH>        安装目录（默认 /opt/corefusion）
  -v, --version <TAG>     面板版本，如 v0.13.0-alpha.2（默认 latest）
      --source            源码编译安装（自动安装匹配的 Go/Node 版本）
      --binary            使用 GitHub Release 二进制（默认）
      --local <FILE>      使用本地二进制文件（离线安装）
      --arch <ARCH>       指定 CPU 架构：amd64/arm64/armv7/armv6/386/ppc64le/s390x/riscv64/loong64
      --mirror <URL>      GitHub 下载加速前缀，例如 https://ghproxy.net
      --repo <URL>        源码仓库地址（默认 https://github.com/qist/corefusion.git）
      --ref <REF>         源码分支/标签（默认取 --version，否则 main）
      --no-deps           跳过系统依赖安装（离线/已预装环境）
      --no-start          只安装并写入服务文件，不启动服务
      --cpu-info          仅输出检测到的 CPU 架构与依赖版本，不执行安装
  -h, --help              显示本帮助

卸载选项：
      --uninstall         停止服务并删除程序（保留数据与 .env）
      --purge             与 --uninstall 连用，同时删除数据目录与 .env

示例：
  curl -fsSL https://raw.githubusercontent.com/qist/corefusion/main/scripts/install.sh | sudo bash
  sudo bash scripts/install.sh --version v0.13.0-alpha.2
  sudo bash scripts/install.sh --source --version v0.13.0-alpha.2
  sudo bash scripts/install.sh --mirror https://ghproxy.net --version latest
  sudo bash scripts/install.sh --local ./dist/corefusion-linux-arm64
  sudo bash scripts/install.sh --uninstall

说明：
  1. 首次启动会进入初始化向导，脚本结束时会打印「初始化地址 + 初始化令牌」。
  2. 源码模式按系统环境选择依赖版本：
       - Go：读取 go.mod 的 go 指令，已安装版本不足时下载对应版本工具链；
       - Node：优先官方 Node 22；glibc < 2.28 时尝试 glibc-217 非官方构建；
         Alpine(musl) 使用发行版 nodejs/npm。
  3. 首次安装写入的 .env 只包含 SAFE_PATHS，以确保进入初始化向导；
     数据库等配置由向导写入，已有 .env 不会被覆盖。
EOF
}

# ── 参数解析 ─────────────────────────────────────────────────
need_arg() { [ $# -ge 2 ] || die "参数 $1 缺少取值"; }

while [ $# -gt 0 ]; do
	case "$1" in
		-d|--dir)       need_arg "$@"; INSTALL_DIR="$2"; shift 2 ;;
		-v|--version)   need_arg "$@"; VERSION="$2"; shift 2 ;;
		--source)       MODE="source"; EXPLICIT_SOURCE=1; shift ;;
		--binary)       MODE="binary"; shift ;;
		--local)        need_arg "$@"; LOCAL_BINARY="$2"; MODE="binary"; shift 2 ;;
		--arch)         need_arg "$@"; ARCH="$2"; shift 2 ;;
		--mirror)       need_arg "$@"; MIRROR="${2%/}"; shift 2 ;;
		--repo)         need_arg "$@"; GIT_REPO="$2"; shift 2 ;;
		--ref)          need_arg "$@"; GIT_REF="$2"; shift 2 ;;
		--no-deps)      SKIP_DEPS=1; shift ;;
		--no-start)     START_SERVICE=0; shift ;;
		--cpu-info)     CPU_QUERY=1; shift ;;
		--uninstall)    UNINSTALL=1; shift ;;
		--purge)        PURGE=1; shift ;;
		-h|--help)      usage; exit 0 ;;
		*) die "未知参数：$1（使用 --help 查看用法）" ;;
	esac
done

INSTALL_DIR="${INSTALL_DIR%/}"
[ -n "$INSTALL_DIR" ] || die "安装目录不能为空"
[ "$PURGE" = "1" ] && [ "$UNINSTALL" != "1" ] && die "--purge 必须与 --uninstall 一起使用"
if [ -n "$LOCAL_BINARY" ] && [ "$EXPLICIT_SOURCE" = "1" ]; then
	die "--local 与 --source 不能同时使用"
fi

# 脚本所在目录（用于 --source 时复用本地源码；curl | bash 时为空）
SCRIPT_DIR=""
if [ -n "${BASH_SOURCE[0]:-}" ] && [ -f "${BASH_SOURCE[0]}" ]; then
	SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
fi

# ── 环境探测 ─────────────────────────────────────────────────
detect_os() {
	local ID="" ID_LIKE="" VERSION_ID="" VERSION="" NAME="" PRETTY_NAME=""
	OS_ID="unknown"; OS_LIKE=""; OS_VER=""; OS_PRETTY="unknown"; FAMILY="unknown"; LIBC="unknown"

	if [ -r /etc/os-release ]; then
		# shellcheck disable=SC1091
		. /etc/os-release
		OS_ID="${ID:-unknown}"
		OS_LIKE="${ID_LIKE:-}"
		OS_VER="${VERSION_ID:-}"
		OS_PRETTY="${PRETTY_NAME:-$OS_ID}"
	fi

	case " ${OS_ID} ${OS_LIKE} " in
		*" debian "*|*" ubuntu "*|*" linuxmint "*|*" raspbian "*) FAMILY="debian" ;;
		*" rhel "*|*" centos "*|*" fedora "*|*" rocky "*|*" almalinux "*|*" ol "*|*" amzn "*) FAMILY="rhel" ;;
		*" alpine "*) FAMILY="alpine" ;;
		*" arch "*|*" manjaro "*|*" endeavouros "*) FAMILY="arch" ;;
		*" suse "*|*" opensuse "*|*" opensuse-leap "*|*" opensuse-tumbleweed "*) FAMILY="suse" ;;
	esac

	if [ "$FAMILY" = "unknown" ] && [ "$OS_ID" = "unknown" ]; then
		warn "无法识别发行版，将只做依赖存在性检查"
	fi

	if command -v getconf >/dev/null 2>&1 && getconf GNU_LIBC_VERSION >/dev/null 2>&1; then
		LIBC="glibc"
	elif ldd --version 2>&1 | grep -qi musl; then
		LIBC="musl"
	fi
}

detect_arch() {
	local machine
	machine="$(uname -m)"
	if [ -n "$ARCH" ]; then
		case "$ARCH" in
			x86_64|amd64)   ARCH="amd64" ;;
			aarch64|arm64)  ARCH="arm64" ;;
			armv7l|armv7|armhf) ARCH="armv7" ;;
			armv6l|armv6|armel) ARCH="armv6" ;;
			i386|i686|386|x86) ARCH="386" ;;
			ppc64le|s390x|riscv64|loong64) ;;
			*) die "不支持的 CPU 架构：$ARCH" ;;
		esac
	else
		case "$machine" in
			x86_64|amd64)   ARCH="amd64" ;;
			aarch64|arm64)  ARCH="arm64" ;;
			armv7l|armv7|armhf) ARCH="armv7" ;;
			armv6l|armv6|armel) ARCH="armv6" ;;
			i386|i686)      ARCH="386" ;;
			ppc64le)        ARCH="ppc64le" ;;
			s390x)          ARCH="s390x" ;;
			riscv64)        ARCH="riscv64" ;;
			loongarch64|loong64) ARCH="loong64" ;;
			*) die "未识别的 CPU 架构：$machine" ;;
		esac
	fi
}

# Go 的 armv7 工具链使用 armv6l 包
go_arch() {
	case "$ARCH" in
		armv7|armv6) echo "armv6l" ;;
		*) echo "$ARCH" ;;
	esac
}

node_arch() {
	case "$ARCH" in
		amd64) echo "x64" ;;
		arm64) echo "arm64" ;;
		armv7) echo "armv7l" ;;
		*) echo "" ;;
	esac
}

release_arch_supported() { [ "$ARCH" = "amd64" ] || [ "$ARCH" = "arm64" ]; }

detect_os
detect_arch
MACHINE="$(uname -m)"
LIBC_VER=""
if [ "$LIBC" = "glibc" ]; then
	LIBC_VER="$(getconf GNU_LIBC_VERSION 2>/dev/null | awk '{print $2}' || true)"
fi

if [ "$CPU_QUERY" = "1" ]; then
	echo "系统        : $OS_PRETTY ($OS_ID $OS_VER)"
	echo "包管理家族  : $FAMILY"
	echo "CPU 架构    : $MACHINE -> $ARCH"
	echo "目标版本    : $VERSION"
	echo "libc        : ${LIBC}${LIBC_VER:+ $LIBC_VER}"
	echo "Go 目标架构 : $(go_arch)"
	echo "Node 架构   : $(node_arch || true)"
	echo "Release 支持: $(release_arch_supported && echo 是 || echo 否（仅支持 amd64/arm64，可改用 --source）)"
	exit 0
fi

[ "$(id -u)" -eq 0 ] || die "请使用 root 运行（例如 sudo bash $0）"
[ -n "$INSTALL_DIR" ] && [ "$INSTALL_DIR" != "/" ] || die "安装目录非法：$INSTALL_DIR"

log "系统：$OS_PRETTY｜CPU：$MACHINE ($ARCH)｜libc：$LIBC ${LIBC_VER:+$LIBC_VER}｜版本：$VERSION"

# ── 包管理 ───────────────────────────────────────────────────
pkg_refresh() {
	case "$FAMILY" in
		debian) export DEBIAN_FRONTEND=noninteractive; apt-get update -y -qq ;;
		rhel)   if command -v dnf >/dev/null 2>&1; then dnf -q makecache -y; else yum -q makecache -y; fi ;;
		alpine) apk update -q ;;
		arch)   pacman -Sy --noconfirm --quiet ;;
		suse)   zypper -q refresh ;;
	esac
}

pkg_install() {
	case "$FAMILY" in
		debian) DEBIAN_FRONTEND=noninteractive apt-get install -y -qq --no-install-recommends "$@" ;;
		rhel)   if command -v dnf >/dev/null 2>&1; then dnf install -y -q "$@"; else yum install -y -q "$@"; fi ;;
		alpine) apk add --no-cache -q "$@" ;;
		arch)   pacman -S --needed --noconfirm --quiet "$@" ;;
		suse)   zypper -q install -y --no-recommends "$@" ;;
		*)      return 1 ;;
	esac
}

install_deps() {
	[ "$SKIP_DEPS" = "1" ] && { warn "已跳过系统依赖安装（--no-deps）"; return 0; }

	local base_pkgs=()
	case "$FAMILY" in
		debian) base_pkgs=(curl ca-certificates tar gzip xz-utils tzdata) ;;
		rhel)   base_pkgs=(curl ca-certificates tar gzip xz tzdata) ;;
		alpine) base_pkgs=(curl ca-certificates tar gzip xz tzdata) ;;
		arch)   base_pkgs=(curl ca-certificates tar gzip xz tzdata) ;;
		suse)   base_pkgs=(curl ca-certificates tar gzip xz tzdata) ;;
		*)      base_pkgs=(curl ca-certificates tar gzip xz) ;;
	esac

	local build_pkgs=()
	if [ "$MODE" = "source" ]; then
		case "$FAMILY" in
			debian) build_pkgs=(git build-essential) ;;
			rhel)   build_pkgs=(git gcc gcc-c++ make) ;;
			alpine) build_pkgs=(git build-base) ;;
			arch)   build_pkgs=(git base-devel) ;;
			suse)   build_pkgs=(git gcc gcc-c++ make) ;;
		esac
	fi

	step "安装系统依赖（$FAMILY）"
	if ! pkg_refresh; then
		warn "包索引刷新失败，尝试直接安装"
	fi
	if ! pkg_install "${base_pkgs[@]}" ${build_pkgs[@]+"${build_pkgs[@]}"}; then
		warn "系统包安装失败，检查必要命令是否已存在"
	fi

	local missing=()
	for c in curl tar gzip; do command -v "$c" >/dev/null 2>&1 || missing+=("$c"); done
	if [ "$MODE" = "source" ]; then
		for c in git make; do command -v "$c" >/dev/null 2>&1 || missing+=("$c"); done
	fi
	[ "${#missing[@]}" -eq 0 ] || die "缺少必要命令：${missing[*]}（请先手动安装，或去掉 --no-deps）"
	ok "系统依赖就绪"
}

# ── 源码模式依赖版本选择 ─────────────────────────────────────
# 纯 shell 版本比较：ver_ge A B 表示 A >= B（支持 1.27 / 1.27.3 / 22.14.0）
ver_ge() {
	local IFS=.
	local -a A=($1) B=($2)
	local n=${#A[@]} i x y
	[ "${#B[@]}" -gt "$n" ] && n=${#B[@]}
	for ((i = 0; i < n; i++)); do
		x="${A[i]:-0}"; y="${B[i]:-0}"
		x="${x%%[!0-9]*}"; y="${y%%[!0-9]*}"
		x="${x:-0}"; y="${y:-0}"
		if ((10#$x > 10#$y)); then return 0; fi
		if ((10#$x < 10#$y)); then return 1; fi
	done
	return 0
}

req_go_version() {
	local v="" tc=""
	if [ -f "$PWD/go.mod" ]; then
		v="$(awk '/^go [0-9]/{print $2; exit}' go.mod)"
		tc="$(awk '/^toolchain go/{sub(/^toolchain go/, ""); print; exit}' go.mod)"
	fi
	echo "${CF_GO_VERSION:-${tc:-${v:-1.27}}}"
}

go_dl_base() { printf '%s' "${CF_GO_MIRROR:-https://go.dev/dl}" | sed 's#/*$##'; }

system_go_ok() {
	command -v go >/dev/null 2>&1 || return 1
	local cur
	cur="$(go env GOVERSION 2>/dev/null | sed 's/^go//')"
	[ -n "$cur" ] || return 1
	ver_ge "$cur" "$1"
}

resolve_go_release() {
	local req="$1" meta ver
	# toolchain 指令可能已是完整版本（x.y.z），直接用；否则查该 minor 的最新补丁
	if printf '%s' "$req" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$'; then
		echo "go${req}"
		return 0
	fi
	meta="$(curl -fsSL --max-time 20 "$(go_dl_base)/?mode=json" 2>/dev/null || true)"
	ver="$(printf '%s' "$meta" | grep -o "\"go${req}\.[0-9]*\"" | head -n1 | tr -d '"' || true)"
	[ -n "$ver" ] || ver="go${req}.0"
	echo "$ver"
}

ensure_go() {
	local req ver tarball
	req="$(req_go_version)"
	if system_go_ok "$req"; then
		ok "使用系统 Go $(go env GOVERSION)（要求 >= $req）"
		return 0
	fi

	ver="$(resolve_go_release "$req")"
	tarball="${ver}.linux-$(go_arch).tar.gz"
	step "安装 Go ${ver#go}（系统版本不满足 >= $req）"
	mkdir -p "$TC_DIR"
	local tmp
	tmp="$(mktemp -d)"
	curl -fL --retry 3 --connect-timeout 15 -o "$tmp/go.tgz" "$(go_dl_base)/$tarball" \
		|| die "下载 Go 工具链失败：$tarball"
	rm -rf "$TC_DIR/go"
	tar -C "$TC_DIR" -xzf "$tmp/go.tgz"
	rm -rf "$tmp"
	export PATH="$TC_DIR/go/bin:$PATH"
	export GOROOT="$TC_DIR/go"
	ok "Go $(go env GOVERSION) 已就绪"
}

node_glibc_ok() {
	[ "$LIBC" = "glibc" ] && [ -n "$LIBC_VER" ] && ver_ge "$LIBC_VER" "2.28"
}

ensure_node() {
	local major="${CF_NODE_MAJOR:-$DEFAULT_NODE_MAJOR}"
	local cur=""

	if [ "$FAMILY" = "alpine" ]; then
		# musl 环境只能用发行版构建，官方 linux-x64 是 glibc，装上去跑不起来
		if ! command -v node >/dev/null 2>&1; then
			step "安装发行版 Node.js（Alpine/musl）"
			pkg_install nodejs npm || die "安装 nodejs/npm 失败"
		fi
		cur="$(node -p 'process.versions.node.split(".")[0]' 2>/dev/null || echo 0)"
		ver_ge "$cur" "18" || die "Alpine 仓库 Node $(node -v) 过低（Vite 6 需要 >= 18）"
		ok "使用发行版 Node $(node -v)（musl）"
		return 0
	fi

	if command -v node >/dev/null 2>&1; then
		cur="$(node -p 'process.versions.node.split(".")[0]' 2>/dev/null || echo 0)"
		if ver_ge "$cur" "$major"; then
			ok "使用系统 Node $(node -v)（要求 >= $major）"
			return 0
		fi
	fi

	local narch base sums name tmp
	narch="$(node_arch)"
	[ -n "$narch" ] || die "Node.js 没有适配 $ARCH 的官方构建，请改用 --binary 或手动提供 Node"

	if node_glibc_ok; then
		base="https://nodejs.org/dist"
		sums="$(curl -fsSL --max-time 20 "$base/latest-v${major}.x/SHASUMS256.txt" 2>/dev/null || true)"
		name="$(printf '%s' "$sums" | awk '{print $2}' | grep -E "^node-v[0-9.]+-linux-${narch}\.tar\.xz$" | head -n1 || true)"
		[ -n "$name" ] || die "无法解析 Node ${major}.x 下载文件名（网络或架构不支持）"
	else
		# glibc < 2.28（如 CentOS 7）：官方 Node 18+ 无法运行，改用 glibc-217 非官方构建
		base="https://unofficial-builds.nodejs.org/download/release"
		sums="$(curl -fsSL --max-time 20 "$base/latest-v${major}.x/SHASUMS256.txt" 2>/dev/null || true)"
		name="$(printf '%s' "$sums" | awk '{print $2}' | grep -E "^node-v[0-9.]+-linux-${narch}-glibc-217\.tar\.(xz|gz)$" | head -n1 || true)"
		[ -n "$name" ] || die "glibc ${LIBC_VER:-<2.28} 无法运行官方 Node $major，且无 glibc-217 构建；请升级系统或改用 --binary"
	fi

	step "安装 Node.js ${major}.x（$name）"
	mkdir -p "$TC_DIR"
	tmp="$(mktemp -d)"
	curl -fL --retry 3 --connect-timeout 15 -o "$tmp/node.tar" "$base/latest-v${major}.x/$name" \
		|| die "下载 Node.js 失败：$name"
	rm -rf "$TC_DIR/node"
	mkdir -p "$TC_DIR/node"
	if [ "${name##*.}" = "xz" ]; then
		tar -C "$TC_DIR/node" --strip-components=1 -xJf "$tmp/node.tar"
	else
		tar -C "$TC_DIR/node" --strip-components=1 -xzf "$tmp/node.tar"
	fi
	rm -rf "$tmp"
	export PATH="$TC_DIR/node/bin:$PATH"
	export NODE_OPTIONS="--max-old-space-size=4096"
	ok "Node $(node -v) 已就绪"
}

# ── 下载与校验 ───────────────────────────────────────────────
gh_url() {
	local url="$1"
	if [ -n "$MIRROR" ]; then printf '%s/%s' "$MIRROR" "$url"; else printf '%s' "$url"; fi
}

latest_version() {
	local json tag
	# 直接用 releases 列表：本项目发布均为 prerelease，/releases/latest 会 404。
	# 列表按创建时间倒序，第一条即最新（草稿对匿名 API 不可见）。
	json="$(curl -fsSL --max-time 20 "https://api.github.com/repos/${REPO}/releases?per_page=5" 2>/dev/null || true)"
	tag="$(printf '%s' "$json" | grep -o '"tag_name"[[:space:]]*:[[:space:]]*"[^"]*"' | head -n1 | sed 's/.*"\([^"]*\)"$/\1/' || true)"
	if [ -z "$tag" ]; then
		tag="$(curl -fsSI --max-time 20 "https://github.com/${REPO}/releases/latest" 2>/dev/null \
			| tr -d '\r' | sed -n 's#^[Ll]ocation: .*/tag/##p' | head -n1 || true)"
	fi
	[ -n "$tag" ] || die "无法获取最新版本号，请用 --version 指定（例如 --version v0.13.0-alpha.2）"
	echo "$tag"
}

download_release() {
	local asset="corefusion-linux-${ARCH}" tmp base
	[ "$VERSION" = "latest" ] && VERSION="$(latest_version)"
	base="https://github.com/${REPO}/releases/download/${VERSION}"
	tmp="$(mktemp -d)"

	step "下载 CoreFusion ${VERSION}（linux-${ARCH}）"
	curl -fL --retry 3 --connect-timeout 15 -o "$tmp/$asset" "$(gh_url "$base/$asset")" \
		|| die "下载失败：$base/$asset"
	if curl -fsSL --retry 2 --connect-timeout 15 -o "$tmp/$asset.sha256" "$(gh_url "$base/$asset.sha256")"; then
		( cd "$tmp" && sha256sum -c "$asset.sha256" >/dev/null ) || die "sha256 校验失败：$asset"
		ok "sha256 校验通过"
	else
		warn "未获取到 sha256 文件，跳过校验"
	fi
	DOWNLOADED_BINARY="$tmp/$asset"
	chmod 0755 "$DOWNLOADED_BINARY"
}

# ── 安装 ─────────────────────────────────────────────────────
stop_existing() {
	if [ "$HAS_SYSTEMD" = "1" ] && [ -f /etc/systemd/system/corefusion.service ]; then
		systemctl stop corefusion >/dev/null 2>&1 || true
	fi
	if [ -f "$PIDFILE" ]; then
		local pid
		pid="$(cat "$PIDFILE" 2>/dev/null || true)"
		if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
			kill "$pid" 2>/dev/null || true
			for _ in $(seq 1 20); do kill -0 "$pid" 2>/dev/null || break; sleep 0.2; done
			kill -9 "$pid" 2>/dev/null || true
		fi
		rm -f "$PIDFILE"
	fi
}

install_files() {
	local src="$1"
	step "安装文件到 $INSTALL_DIR"
	mkdir -p "$INSTALL_DIR"
	# 避免 --local 指向安装目录自身时 cp/install 自覆盖
	if [ -x "$INSTALL_DIR/corefusion" ] && [ "$(readlink -f "$src")" != "$(readlink -f "$INSTALL_DIR/corefusion")" ]; then
		cp -a "$INSTALL_DIR/corefusion" "$INSTALL_DIR/corefusion.bak"
	fi
	install -m 0755 "$src" "$INSTALL_DIR/corefusion"
	mkdir -p "$DATA_DIR/sqlite" "$DATA_DIR/certs" "$DATA_DIR/logs" "$DATA_DIR/runtime"
	chmod 0750 "$DATA_DIR" 2>/dev/null || true

	if [ ! -f "$INSTALL_DIR/.env" ]; then
		# 只写 SAFE_PATHS：CF_DB_DSN 留空 → bootstrap.IsFirstBoot 判定为首启，进入初始化向导
		( umask 077; printf 'SAFE_PATHS=%s\n' "$SAFE_PATHS_VALUE" > "$INSTALL_DIR/.env" )
		ok "已生成 $INSTALL_DIR/.env（仅 SAFE_PATHS，数据库配置由初始化向导写入）"
	elif grep -q '^CF_DB_DSN=' "$INSTALL_DIR/.env" && [ -n "$(grep '^CF_DB_DSN=' "$INSTALL_DIR/.env" | cut -d= -f2- | tr -d '[:space:]')" ]; then
		ALREADY_CONFIGURED=1
		ok "检测到已有 .env，保留原配置（不会进入初始化向导）"
	else
		ok "检测到已有 .env，保留原配置"
	fi
}

install_systemd_unit() {
	local unit="/etc/systemd/system/corefusion.service"
	step "写入 systemd 服务"
	cat > "$unit" <<EOF
[Unit]
Description=CoreFusion Panel
Documentation=https://github.com/${REPO}
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
# 数据库 DSN 为相对路径时依赖此工作目录，不要修改
WorkingDirectory=${INSTALL_DIR}
ExecStart=${INSTALL_DIR}/corefusion
Restart=on-failure
RestartSec=3
LimitNOFILE=1048576
KillSignal=SIGTERM
TimeoutStopSec=30

[Install]
WantedBy=multi-user.target
EOF
	chmod 0644 "$unit"
	systemctl daemon-reload
	systemctl enable corefusion >/dev/null 2>&1
	ok "服务已安装：$unit"
}

start_systemd() {
	START_TS="$(date +%s)"
	systemctl restart corefusion
	ok "已启动 corefusion.service"
}

start_nohup() {
	step "无 systemd，使用 nohup 启动"
	mkdir -p "$(dirname "$LOG_FILE")"
	: > "$LOG_FILE"
	( cd "$INSTALL_DIR" && exec nohup ./corefusion >>"$LOG_FILE" 2>&1 ) &
	echo $! > "$PIDFILE"
	sleep 1
	if ! kill -0 "$(cat "$PIDFILE" 2>/dev/null)" 2>/dev/null; then
		warn "进程未存活，最近日志："
		tail -n 30 "$LOG_FILE" >&2 || true
		die "启动失败"
	fi
	START_TS="$(date +%s)"
	ok "已启动（PID $(cat "$PIDFILE")）"
}

service_log() {
	if [ "$HAS_SYSTEMD" = "1" ] && [ "$START_MODE" = "systemd" ]; then
		journalctl -u corefusion -o cat --since "@${START_TS}" --no-pager 2>/dev/null || true
	else
		tail -n 200 "$LOG_FILE" 2>/dev/null || true
	fi
}

print_init_info() {
	local log url token ip scheme port listen_addr loopback_only
	log="$(service_log)"

	url="$(printf '%s\n' "$log" | sed -n 's/^.*访问地址:[[:space:]]*//p' | tail -n1)"
	token="$(printf '%s\n' "$log" | sed -n 's/^.*初始化令牌:[[:space:]]*//p' | tail -n1)"

	# 面板可能监听 127.0.0.1:PORT / 0.0.0.0:PORT / :PORT，三种都要取到端口
	scheme="http"
	listen_addr="$(printf '%s\n' "$log" | sed -n 's/.* starting on \([^ ]*\) (HTTP).*/\1/p' | tail -n1)"
	if [ -z "$listen_addr" ]; then
		listen_addr="$(printf '%s\n' "$log" | sed -n 's/.* starting on \([^ ]*\) (HTTPS).*/\1/p' | tail -n1)"
		[ -n "$listen_addr" ] && scheme="https"
	fi
	port="${listen_addr##*:}"
	case "$port" in
		''|*[!0-9]*) port="8080" ;;
	esac
	# 仅回环监听时，用本机 IP 拼出来的地址是访问不到的，单独提示
	loopback_only=0
	case "$listen_addr" in
		127.0.0.1:*|localhost:*|\[::1\]:*) loopback_only=1 ;;
	esac

	ip="$(hostname -I 2>/dev/null | awk '{print $1}')"
	[ -n "$ip" ] || ip="127.0.0.1"

	printf '\n%s' "$C_BOLD"
	cat <<EOF
==================================================================
 CoreFusion 安装完成
------------------------------------------------------------------
 安装目录 : ${INSTALL_DIR}
 数据目录 : ${DATA_DIR}
 二进制   : ${INSTALL_DIR}/corefusion
 日志     : $(if [ "$START_MODE" = "systemd" ]; then echo "journalctl -u corefusion -f"; else echo "$LOG_FILE"; fi)
EOF
	if [ -n "$url" ]; then
		cat <<EOF
------------------------------------------------------------------
 初始化向导 : ${url}
 初始化令牌 : ${token}
 有效期     : 2 小时（见启动日志）
 提示       : 令牌只显示一次；完成后向导端口关闭并自动切换到正式面板。
EOF
	else
		if [ "${ALREADY_CONFIGURED:-0}" = "1" ]; then
			if [ "$loopback_only" = "1" ]; then
				cat <<EOF
------------------------------------------------------------------
 面板已初始化，监听地址 : ${listen_addr}
 仅本机监听，远程请用 SSH 隧道：
   ssh -L ${port}:127.0.0.1:${port} root@${ip}
 或在系统设置中配置域名 + TLS / 反向代理后对外开放
EOF
			else
				cat <<EOF
------------------------------------------------------------------
 面板已初始化，直接访问 : ${scheme}://${ip}:${port}
 （如修改过端口，请以 .env 配置为准）
EOF
			fi
		else
			cat <<EOF
------------------------------------------------------------------
 未在启动日志中捕获到初始化地址，请执行：
   $(if [ "$START_MODE" = "systemd" ]; then echo "journalctl -u corefusion -n 50 --no-pager"; else echo "tail -n 50 $LOG_FILE"; fi)
 若需重新进入初始化向导（会要求输入 RESET 确认）：
   cd ${INSTALL_DIR} && ./corefusion reset-init && systemctl restart corefusion
EOF
		fi
	fi
	printf '%s' "$C_RESET"

	if [ "$START_MODE" = "nohup" ] && [ -n "$url" ]; then
		printf ' 提示：当前环境无 systemd，进程由 nohup 托管，重启服务器后需重新执行本脚本。\n'
	fi
	echo "=================================================================="
}

uninstall() {
	step "卸载 CoreFusion"
	if [ "$HAS_SYSTEMD" = "1" ] && [ -f /etc/systemd/system/corefusion.service ]; then
		systemctl disable --now corefusion >/dev/null 2>&1 || true
	fi
	stop_existing
	rm -f /etc/systemd/system/corefusion.service
	if [ "$HAS_SYSTEMD" = "1" ]; then systemctl daemon-reload; fi
	rm -f "$INSTALL_DIR/corefusion" "$INSTALL_DIR/corefusion.bak" "$PIDFILE"
	rm -rf "$TC_DIR"
	if [ "$PURGE" = "1" ]; then
		[ "$INSTALL_DIR" != "/" ] && [ -n "$INSTALL_DIR" ] || die "拒绝删除 $INSTALL_DIR"
		case "$DATA_DIR" in
			"$INSTALL_DIR"/*) rm -rf "$DATA_DIR" ;;
			*) warn "数据目录不在安装目录下，已跳过删除：$DATA_DIR" ;;
		esac
		rm -f "$INSTALL_DIR/.env"
		rmdir "$INSTALL_DIR" 2>/dev/null || true
		ok "已删除程序与数据"
	else
		ok "已删除程序，数据与 .env 保留在 $INSTALL_DIR"
	fi
	exit 0
}

# ── 主流程 ───────────────────────────────────────────────────
HAS_SYSTEMD=0
if command -v systemctl >/dev/null 2>&1 && [ -d /run/systemd/system ]; then
	HAS_SYSTEMD=1
fi

DATA_DIR="$INSTALL_DIR/data"
PIDFILE="$INSTALL_DIR/data/corefusion.pid"
LOG_FILE="$INSTALL_DIR/data/logs/panel.log"
TC_DIR="$INSTALL_DIR/.toolchain"
SAFE_PATHS_VALUE="data"
ALREADY_CONFIGURED=0
START_MODE="systemd"
START_TS="$(date +%s)"
DOWNLOADED_BINARY=""

if [ -f "$INSTALL_DIR/.env" ]; then
	EXISTING_SAFE_PATHS="$(sed -n 's/^SAFE_PATHS=//p' "$INSTALL_DIR/.env" | tail -n1 | tr -d '"' || true)"
	if [ -n "$EXISTING_SAFE_PATHS" ]; then
		case "$EXISTING_SAFE_PATHS" in
			/*) DATA_DIR="$EXISTING_SAFE_PATHS" ;;
			*)  DATA_DIR="$INSTALL_DIR/$EXISTING_SAFE_PATHS" ;;
		esac
		SAFE_PATHS_VALUE="$EXISTING_SAFE_PATHS"
	fi
fi
PIDFILE="$DATA_DIR/corefusion.pid"
LOG_FILE="$DATA_DIR/logs/panel.log"

if [ "$UNINSTALL" = "1" ]; then
	uninstall
fi

install_deps

if [ "$MODE" = "binary" ]; then
	if [ -n "$LOCAL_BINARY" ]; then
		[ -f "$LOCAL_BINARY" ] || die "本地二进制不存在：$LOCAL_BINARY"
		DOWNLOADED_BINARY="$LOCAL_BINARY"
		chmod 0755 "$DOWNLOADED_BINARY"
		step "使用本地二进制：$LOCAL_BINARY"
	elif ! release_arch_supported; then
		die "官方 Release 仅提供 amd64/arm64，当前为 $ARCH；请改用 --source（需可编译环境）"
	else
		download_release
	fi
else
	[ "$SKIP_DEPS" = "0" ] || warn "源码模式建议保留系统依赖安装（去掉 --no-deps）"
	SRC_DIR=""
	if [ -n "$SCRIPT_DIR" ] && [ -f "$SCRIPT_DIR/../go.mod" ]; then
		SRC_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
	fi
	if [ -n "$SRC_DIR" ] && [ -f "$SRC_DIR/go.mod" ]; then
		step "使用本地源码：$SRC_DIR"
	else
		[ -n "$GIT_REF" ] || GIT_REF="$([ "$VERSION" = "latest" ] && echo main || echo "$VERSION")"
		SRC_DIR="$(mktemp -d)/corefusion"
		step "克隆源码：$GIT_REPO ($GIT_REF)"
		git clone --depth 1 --branch "$GIT_REF" "$GIT_REPO" "$SRC_DIR" \
			|| die "克隆源码失败：$GIT_REPO ($GIT_REF)"
	fi

	(
		cd "$SRC_DIR"
		ensure_go
		ensure_node
		step "编译面板（make panel）"
		make panel
		mkdir -p "$TC_DIR"
		install -m 0755 corefusion "$TC_DIR/corefusion-built"
	)
	DOWNLOADED_BINARY="$TC_DIR/corefusion-built"
fi

[ -n "$DOWNLOADED_BINARY" ] && [ -f "$DOWNLOADED_BINARY" ] || die "未获得可安装的二进制"

stop_existing
install_files "$DOWNLOADED_BINARY"

if [ "$START_SERVICE" = "0" ]; then
	step "已按要求跳过启动"
	if [ "$HAS_SYSTEMD" = "1" ]; then
		install_systemd_unit
		printf '手动启动：systemctl start corefusion\n'
	else
		printf '手动启动：cd %s && ./corefusion\n' "$INSTALL_DIR"
	fi
	printf '首启初始化地址会在启动日志中打印。\n'
	exit 0
fi

if [ "$HAS_SYSTEMD" = "1" ]; then
	install_systemd_unit
	start_systemd
else
	START_MODE="nohup"
	start_nohup
fi

# 等待初始化向导横幅（首启）或正常启动日志
step "等待服务就绪"
READY=0
for _ in $(seq 1 40); do
	LOG_NOW="$(service_log)"
	if printf '%s' "$LOG_NOW" | grep -Eq '初始化令牌|Initialize token|init_token='; then
		READY=1; break
	fi
	if printf '%s' "$LOG_NOW" | grep -Eq 'starting on .* \((HTTP|HTTPS)\)'; then
		READY=1; break
	fi
	if printf '%s' "$LOG_NOW" | grep -Eq 'failed to init database|panic:'; then
		printf '%s\n' "$LOG_NOW" | tail -n 20 >&2
		die "服务启动失败，请检查日志"
	fi
	sleep 1
done
if [ "$READY" != "1" ]; then
	if [ "$HAS_SYSTEMD" = "1" ] && ! systemctl is-active --quiet corefusion; then
		printf '%s\n' "$LOG_NOW" | tail -n 30 >&2
		die "corefusion.service 未能保持运行，请检查上面的日志"
	fi
	warn "40 秒内未捕获到就绪日志，仍继续输出已知信息"
fi

print_init_info
