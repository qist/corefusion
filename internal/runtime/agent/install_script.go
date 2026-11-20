package agent

import (
	"fmt"
	"regexp"
	"strings"
	"text/template"

	"corefusion/internal/constants"
)

// installHostPattern 限制安装脚本里出现的 host：脚本用双引号包住变量，
// host 里出现 `"`/`$(`/空白会变成命令注入（安全审计 v7 L-9）。
var installHostPattern = regexp.MustCompile(`^[A-Za-z0-9._:\[\]-]+$`)

func validateInstallHost(what, host string) error {
	if host == "" {
		return fmt.Errorf("安装脚本缺少 %s", what)
	}
	if len(host) > 255 || !installHostPattern.MatchString(host) {
		return fmt.Errorf("非法 %s %q：只允许主机名/IP（字母、数字、. _ : [ ] -）", what, host)
	}
	return nil
}

// AgentInstallScript generates a bash install script for the Agent binary.
// The script downloads the Agent (GitHub Releases first, panel fallback),
// configures it with the panel address and bootstrap token, then starts the
// service.
type AgentInstallScript struct {
	// panelHost/panelPort/panelScheme 是**下载脚本和二进制**用的公开地址
	// （通常是 CDN/反代域名），不用于 Agent 配置。
	panelHost   string
	panelPort   int
	panelScheme string // "http" or "https"

	// bootstrapHost/bootstrapPort/bootstrapTLS 是 Agent 引导阶段直连的
	// 面板自身监听地址。反代/CDN 会终止 TLS，其证书不是面板 CA 签发的，
	// Agent 无法用 panel CA 校验；因此引导必须走源站，而不是公开域名。
	bootstrapHost   string
	bootstrapPort   int
	bootstrapTLS    bool
	bootstrapTLSSet bool

	panelGRPCHost  string // gRPC 连接地址：面板未走 CDN 时与 panelHost 相同，走 CDN 时为源站 IP
	panelGRPCPort  int
	panelTLS       bool // 兼容旧调用：未单独设置 bootstrapTLS 时作为引导 TLS
	bootstrapToken string
	caCertPEM      []byte // 面板 CA 证书（公开信息）—— 让 Agent 引导阶段就能校验面板身份
	// updatePublicKeyPEM 是面板更新签名公钥（公开信息）—— 写入 config.yaml 的
	// update.public_key，Agent 自更新时用它校验控制面下发的 ed25519 签名（v7 L-6）。
	updatePublicKeyPEM []byte
	serverID           int64
	agentVersion       string
	githubURL          string // GitHub Releases base URL (fallback)
}

// NewAgentInstallScript creates a new AgentInstallScript generator.
func NewAgentInstallScript(panelHost string, panelPort int) *AgentInstallScript {
	return &AgentInstallScript{
		panelHost:     panelHost,
		panelPort:     panelPort,
		panelScheme:   "http",
		panelGRPCPort: constants.DefaultAgentGRPCPort,
		panelTLS:      false,
		agentVersion:  "latest",
		githubURL:     "https://github.com/qist/corefusion/releases",
	}
}

// WithBootstrapToken sets the bootstrap token.
func (s *AgentInstallScript) WithBootstrapToken(token string) *AgentInstallScript {
	s.bootstrapToken = token
	return s
}

// WithCACertPEM 设置随脚本一起下发的面板 CA 证书（PEM）。
// 有了它，Agent 在**引导阶段**（还没有自己的证书时）就能对面板做正常的 TLS 校验，
// 而不是像以前那样 `InsecureSkipVerify: true` ✗（见安全审计 v5 M6）。
func (s *AgentInstallScript) WithCACertPEM(pemBytes []byte) *AgentInstallScript {
	s.caCertPEM = pemBytes
	return s
}

// WithUpdatePublicKey 设置随脚本写入节点 config.yaml 的面板更新签名公钥（PEM）。
func (s *AgentInstallScript) WithUpdatePublicKey(pemBytes []byte) *AgentInstallScript {
	s.updatePublicKeyPEM = pemBytes
	return s
}

// WithServerID sets the server ID.
func (s *AgentInstallScript) WithServerID(id int64) *AgentInstallScript {
	s.serverID = id
	return s
}

// WithScheme sets the panel URL scheme ("http" or "https").
func (s *AgentInstallScript) WithScheme(scheme string) *AgentInstallScript {
	s.panelScheme = scheme
	return s
}

// WithGRPCHost sets the address the Agent uses for the gRPC heartbeat.
// When the panel is fronted by a normal CDN, panelHost is the CDN domain but
// only the origin IP can serve the gRPC port, so the panel passes that here.
func (s *AgentInstallScript) WithGRPCHost(host string) *AgentInstallScript {
	s.panelGRPCHost = strings.TrimSpace(host)
	return s
}

// WithGRPCPort sets the panel gRPC port.
func (s *AgentInstallScript) WithGRPCPort(port int) *AgentInstallScript {
	s.panelGRPCPort = port
	return s
}

// WithTLS sets whether the panel uses TLS for bootstrap. Kept for compatibility;
// WithBootstrapEndpoint takes precedence when the panel's own listen endpoint
// differs from the public CDN/reverse-proxy endpoint.
func (s *AgentInstallScript) WithTLS(tls bool) *AgentInstallScript {
	s.panelTLS = tls
	if !s.bootstrapTLSSet {
		s.bootstrapTLS = tls
	}
	return s
}

// WithBootstrapEndpoint sets the panel's direct HTTP(S) endpoint that the Agent
// uses for bootstrap. This must be the panel's own listener (origin IP + local
// listen port), because a CDN/reverse proxy terminates TLS with a certificate
// that is not signed by the panel CA.
func (s *AgentInstallScript) WithBootstrapEndpoint(host string, port int, tls bool) *AgentInstallScript {
	s.bootstrapHost = strings.TrimSpace(host)
	s.bootstrapPort = port
	s.bootstrapTLS = tls
	s.bootstrapTLSSet = true
	return s
}

// WithGitHubURL sets the GitHub Releases base URL (i.e. without
// /download/<tag> or /latest/download).
func (s *AgentInstallScript) WithGitHubURL(url string) *AgentInstallScript {
	s.githubURL = url
	return s
}

// WithAgentVersion sets the release tag used for the GitHub download.
// "latest" (the default) resolves to GitHub's newest stable Release; any other
// value is used as an explicit tag under /releases/download/<tag>/.
func (s *AgentInstallScript) WithAgentVersion(version string) *AgentInstallScript {
	if v := strings.TrimSpace(version); v != "" {
		s.agentVersion = v
	}
	return s
}

// Generate generates the install script content.
func (s *AgentInstallScript) Generate() (string, error) {
	grpcHost := strings.TrimSpace(s.panelGRPCHost)
	if grpcHost == "" {
		grpcHost = s.panelHost
	}
	bootstrapHost := strings.TrimSpace(s.bootstrapHost)
	if bootstrapHost == "" {
		bootstrapHost = s.panelHost
	}
	bootstrapPort := s.bootstrapPort
	if bootstrapPort <= 0 {
		bootstrapPort = s.panelPort
	}
	bootstrapTLS := s.bootstrapTLS
	if !s.bootstrapTLSSet {
		bootstrapTLS = s.panelTLS
	}
	bootstrapScheme := "http"
	if bootstrapTLS {
		bootstrapScheme = "https"
	}
	// 安全检查（安全审计 v7 L-9）：host 会原样拼进 bash 脚本的双引号变量。
	for what, host := range map[string]string{
		"面板地址":    s.panelHost,
		"gRPC 地址": grpcHost,
		"引导地址":    bootstrapHost,
	} {
		if err := validateInstallHost(what, host); err != nil {
			return "", err
		}
	}
	var sb strings.Builder
	err := installScriptTmpl.Execute(&sb, map[string]interface{}{
		"PanelHost":          s.panelHost,
		"PanelPort":          s.panelPort,
		"PanelScheme":        s.panelScheme,
		"PanelTLS":           s.panelTLS,
		"BootstrapHost":      bootstrapHost,
		"BootstrapPort":      bootstrapPort,
		"BootstrapScheme":    bootstrapScheme,
		"BootstrapTLS":       bootstrapTLS,
		"PanelGRPCHost":      grpcHost,
		"PanelGRPCPort":      s.panelGRPCPort,
		"BootstrapToken":     s.bootstrapToken,
		"CACertPEM":          string(s.caCertPEM),
		"UpdatePublicKeyPEM": string(s.updatePublicKeyPEM),
		"ServerID":           s.serverID,
		"AgentVersion":       s.agentVersion,
		"GitHubURL":          s.githubURL,
	})
	if err != nil {
		return "", fmt.Errorf("generate install script: %w", err)
	}
	return sb.String(), nil
}

// installScriptTmpl is the bash template for the Agent install script.
var installScriptTmpl = template.Must(template.New("install").Parse(`#!/bin/bash
set -e

# CoreFusion Agent Install Script
# Generated for server_id={{.ServerID}}
# Panel download: {{.PanelScheme}}://{{.PanelHost}}:{{.PanelPort}}
# Agent bootstrap: {{.PanelScheme}}://{{.PanelHost}}:{{.PanelPort}}
# Agent bootstrap fallback (direct): {{.BootstrapScheme}}://{{.BootstrapHost}}:{{.BootstrapPort}}
# Agent gRPC: {{.PanelGRPCHost}}:{{.PanelGRPCPort}}

PANEL_SCHEME="{{.PanelScheme}}"
PANEL_HOST="{{.PanelHost}}"
PANEL_PORT="{{.PanelPort}}"
PANEL_TLS={{.PanelTLS}}
BOOTSTRAP_SCHEME="{{.BootstrapScheme}}"
BOOTSTRAP_HOST="{{.BootstrapHost}}"
BOOTSTRAP_PORT="{{.BootstrapPort}}"
BOOTSTRAP_TLS={{.BootstrapTLS}}
PANEL_GRPC_HOST="{{.PanelGRPCHost}}"
PANEL_GRPC_PORT="{{.PanelGRPCPort}}"
# Bracket IPv6 literals for URL interpolation; the raw host stays in config.yaml.
PANEL_URL_HOST="$PANEL_HOST"
case "$PANEL_HOST" in
  *:*) PANEL_URL_HOST="[$PANEL_HOST]" ;;
esac
BOOTSTRAP_TOKEN="{{.BootstrapToken}}"
SERVER_ID="{{.ServerID}}"
AGENT_VERSION="{{.AgentVersion}}"
GITHUB_URL="{{.GitHubURL}}"
INSTALL_DIR="/opt/corefusion-agent"
CONFIG_FILE="$INSTALL_DIR/config.yaml"
SERVICE_FILE="/etc/systemd/system/corefusion-agent.service"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${GREEN}=== CoreFusion Agent Installer ===${NC}"
echo "Panel download: $PANEL_SCHEME://$PANEL_URL_HOST:$PANEL_PORT"
echo "Agent bootstrap: $PANEL_SCHEME://$PANEL_URL_HOST:$PANEL_PORT"
if [ "$BOOTSTRAP_TLS" = "true" ] && { [ "$BOOTSTRAP_HOST" != "$PANEL_HOST" ] || [ "$BOOTSTRAP_PORT" != "$PANEL_PORT" ]; }; then
  echo "Agent bootstrap fallback (direct): $BOOTSTRAP_SCHEME://$BOOTSTRAP_HOST:$BOOTSTRAP_PORT"
fi
echo "Agent gRPC: $PANEL_GRPC_HOST:$PANEL_GRPC_PORT"
echo "Server ID: $SERVER_ID"
echo ""

# Check root
if [ "$EUID" -ne 0 ]; then
  echo -e "${RED}Error: Please run as root${NC}"
  exit 1
fi

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  AGENT_ARCH="amd64" ;;
  aarch64) AGENT_ARCH="arm64" ;;
  *)       echo -e "${RED}Unsupported architecture: $ARCH${NC}"; exit 1 ;;
esac

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
echo "OS: $OS, Arch: $AGENT_ARCH"

# ── 运行时依赖 ──────────────────────────────────────────────
# Agent 在本机接管限速/防火墙/流量统计，需要这些系统命令：
#   nftables       nft（防火墙/限速/并发标记）
#   iproute2       tc / ip / ss（限速队列、网卡与连接查看）
#   iptables       无 nft 时的回退
#   conntrack      conntrack-tools（并发连接统计）
#   kmod           modprobe / lsmod（加载 nf_conntrack、ifb）
#   procps         pgrep / pkill（进程检测与清理）
#   lsof/openssl   端口占用检测 / DH 参数生成
# 装不上不阻断安装：缺这些只会让限速/防火墙能力受限。
install_runtime_deps() {
  if [ "${CF_SKIP_DEPS:-0}" = "1" ]; then
    echo "已跳过系统依赖安装（CF_SKIP_DEPS=1）"
    return 0
  fi

  local pkgs=""
  local mgr=""
  # EL8+ 的 iptables 命令由 iptables-nft 提供（没有独立的 iptables 包）；EL7 只有 iptables。
  local ipt_pkg="iptables"
  if [ -r /etc/os-release ]; then
    local os_id="" os_major=""
    os_id="$(. /etc/os-release 2>/dev/null; echo "$ID")"
    os_major="$(. /etc/os-release 2>/dev/null; echo "${VERSION_ID%%.*}")"
    case "$os_id" in
      rhel|centos|rocky|almalinux|fedora|ol|amzn)
        if [ "${os_major:-0}" -ge 8 ] 2>/dev/null; then ipt_pkg="iptables-nft"; fi
        ;;
    esac
  fi
  if command -v apt-get >/dev/null 2>&1; then
    mgr="apt"
    pkgs="nftables iproute2 iptables conntrack kmod procps lsof openssl passwd ca-certificates curl tar gzip xz-utils"
  elif command -v dnf >/dev/null 2>&1; then
    mgr="dnf"
    pkgs="nftables iproute $ipt_pkg conntrack-tools kmod procps-ng lsof openssl shadow-utils ca-certificates curl tar gzip xz"
  elif command -v yum >/dev/null 2>&1; then
    mgr="yum"
    pkgs="nftables iproute $ipt_pkg conntrack-tools kmod procps-ng lsof openssl shadow-utils ca-certificates curl tar gzip xz"
  elif command -v apk >/dev/null 2>&1; then
    mgr="apk"
    pkgs="nftables iproute2 iptables conntrack-tools kmod procps lsof openssl shadow ca-certificates curl tar gzip xz"
  elif command -v pacman >/dev/null 2>&1; then
    mgr="pacman"
    pkgs="nftables iproute2 iptables conntrack-tools kmod procps-ng lsof openssl shadow ca-certificates curl tar gzip xz"
  elif command -v zypper >/dev/null 2>&1; then
    mgr="zypper"
    pkgs="nftables iproute2 iptables conntrack-tools kmod procps lsof openssl shadow ca-certificates curl tar gzip xz"
  fi

  if [ -z "$mgr" ]; then
    echo "警告：未识别的包管理器，跳过依赖安装（可设置 CF_SKIP_DEPS=1 消除本提示）"
    return 0
  fi

  echo "安装运行时依赖（$mgr）..."
  case "$mgr" in
    apt)
      export DEBIAN_FRONTEND=noninteractive
      apt-get update -y -qq >/dev/null 2>&1 || true
      apt-get install -y -qq --no-install-recommends $pkgs >/dev/null 2>&1 \
        || for p in $pkgs; do apt-get install -y -qq --no-install-recommends "$p" >/dev/null 2>&1 || true; done
      ;;
    dnf|yum)
      $mgr install -y -q $pkgs >/dev/null 2>&1 \
        || for p in $pkgs; do $mgr install -y -q "$p" >/dev/null 2>&1 || true; done
      ;;
    apk)
      apk add --no-cache -q $pkgs >/dev/null 2>&1 \
        || for p in $pkgs; do apk add --no-cache -q "$p" >/dev/null 2>&1 || true; done
      ;;
    pacman)
      pacman -Sy --needed --noconfirm --quiet $pkgs >/dev/null 2>&1 \
        || for p in $pkgs; do pacman -S --needed --noconfirm --quiet "$p" >/dev/null 2>&1 || true; done
      ;;
    zypper)
      zypper -q install -y --no-recommends $pkgs >/dev/null 2>&1 \
        || for p in $pkgs; do zypper -q install -y --no-recommends "$p" >/dev/null 2>&1 || true; done
      ;;
  esac

  local miss=""
  for c in nft tc ip ss pgrep pkill lsof openssl modprobe conntrack; do
    command -v "$c" >/dev/null 2>&1 || miss="$miss $c"
  done
  if [ -n "$miss" ]; then
    echo "警告：以下命令仍缺失，限速/防火墙能力会受限：$miss"
  else
    echo "运行时依赖就绪（nft/tc/ip/ss/procps/lsof/openssl）"
  fi
}
install_runtime_deps

# Create install directory structure (must match initDataDir in agent main.go)
mkdir -p "$INSTALL_DIR/data/sqlite" \
         "$INSTALL_DIR/data/certs" \
         "$INSTALL_DIR/data/logs" \
         "$INSTALL_DIR/data/runtime" \
         "$INSTALL_DIR/data/proxy" \
         "$INSTALL_DIR/data/binaries/active" \
         "$INSTALL_DIR/data/binaries/proxy" \
         "$INSTALL_DIR/data/rules" \
         "$INSTALL_DIR/data/rules_tmp"

# Download Agent binary
# Strategy: **panel first** —— 面板分发目录里的二进制才是这台面板实际管理、会通过
# 「更新 Agent」下发的版本（可能带尚未发版的修复）。GitHub Release 只作兜底
# （面板下载端点异常时）。
# 面板优先：本地分发目录有**同版本**二进制时走面板（内网快，且能分发未发版的修复）；
# 面板没有该版本 / 端点异常时回退 GitHub。版本由文件 services/agent 的 VERSION 决定。
PANEL_DOWNLOAD_URL="${PANEL_SCHEME}://${PANEL_HOST}:${PANEL_PORT}/api/v1/agent/download?os=${OS}&arch=${AGENT_ARCH}&version=${AGENT_VERSION:-latest}"
GITHUB_LATEST_URL="${GITHUB_URL}/latest/download/corefusion-agent-${OS}-${AGENT_ARCH}"
if [ "$AGENT_VERSION" = "latest" ] || [ -z "$AGENT_VERSION" ]; then
  GITHUB_DOWNLOAD_URL="$GITHUB_LATEST_URL"
else
  GITHUB_DOWNLOAD_URL="${GITHUB_URL}/download/${AGENT_VERSION}/corefusion-agent-${OS}-${AGENT_ARCH}"
fi

# 先下到临时文件再 mv 覆盖：重装/升级时目标二进制正在运行，直接 curl -o
# 会命中 ETXTBSY（Text file busy）→ 下载失败。mv 换 inode 不受影响。
AGENT_TMP="$INSTALL_DIR/corefusion-agent.new"

# 只接受真正的 ELF 文件：面板挂在 CDN/反代后面时，/api/v1/agent/download
# 可能被拦成 200 的 HTML/JSON —— curl -f 判定为成功，节点执行时才会报
# Exec format error。这里当场识别并换下一个源。
agent_download_ok() {
  url="$1"
  [ -n "$url" ] || return 1
  rm -f "$AGENT_TMP"
  curl -fsSL --connect-timeout 10 --max-time 300 -o "$AGENT_TMP" "$url" 2>/dev/null || return 1
  [ -s "$AGENT_TMP" ] || { rm -f "$AGENT_TMP"; return 1; }
  magic="$(head -c 4 "$AGENT_TMP" 2>/dev/null | od -An -tx1 | tr -d ' \n')"
  if [ "$magic" != "7f454c46" ]; then
    echo "  ignored non-ELF response from $url"
    rm -f "$AGENT_TMP"
    return 1
  fi
  return 0
}

echo -e "${YELLOW}Downloading Agent (version=${AGENT_VERSION:-latest})...${NC}"
if agent_download_ok "$PANEL_DOWNLOAD_URL"; then
  echo "Downloaded from panel ($PANEL_DOWNLOAD_URL)"
elif agent_download_ok "$GITHUB_DOWNLOAD_URL"; then
  echo "Downloaded from GitHub ($GITHUB_DOWNLOAD_URL)"
elif [ "$GITHUB_DOWNLOAD_URL" != "$GITHUB_LATEST_URL" ] && agent_download_ok "$GITHUB_LATEST_URL"; then
  echo "Downloaded from GitHub latest ($GITHUB_LATEST_URL)"
else
  rm -f "$AGENT_TMP"
  echo -e "${RED}Failed to download Agent from panel or GitHub${NC}"
  echo "  Panel: $PANEL_DOWNLOAD_URL"
  echo "  GitHub: $GITHUB_DOWNLOAD_URL"
  exit 1
fi
chmod +x "$AGENT_TMP"
mv -f "$AGENT_TMP" "$INSTALL_DIR/corefusion-agent"

# Generate config
echo -e "${YELLOW}Writing config...${NC}"
PANEL_CA_PEM='{{.CACertPEM}}'
if [ -n "$PANEL_CA_PEM" ] && [ "$PANEL_CA_PEM" != "''" ]; then
  # Agent 的 a.caPath 是 $dataDir/certs/ca-cert.pem，而 data_dir 是
  # "$INSTALL_DIR/data"；CA 写错到 "$INSTALL_DIR/certs" 会导致 gRPC mTLS
  # 读不到 CA，心跳永远连不上。
  CA_DIR="$INSTALL_DIR/data/certs"
  mkdir -p "$CA_DIR"
  printf '%s\n' "$PANEL_CA_PEM" > "$CA_DIR/ca-cert.pem"
  chmod 640 "$CA_DIR/ca-cert.pem"
  echo -e "${GREEN}面板 CA 证书已就位（Agent gRPC 心跳将用它校验面板）${NC}"
fi

cat > "$CONFIG_FILE" << EOF
panel:
  host: "$BOOTSTRAP_HOST"
  port: $BOOTSTRAP_PORT
  grpc_host: "$PANEL_GRPC_HOST"
  grpc_port: $PANEL_GRPC_PORT
  tls: $BOOTSTRAP_TLS
  public_host: "$PANEL_HOST"
  public_port: $PANEL_PORT
  public_tls: $PANEL_TLS
  server_id: $SERVER_ID

bootstrap:
  token: "$BOOTSTRAP_TOKEN"

agent:
  log_level: info
  data_dir: "$INSTALL_DIR/data"
EOF
# 更新签名公钥（v7 L-6）：写入 config.yaml 后，Agent 拒绝未签名/签名不符的自更新。
UPDATE_PUBKEY_PEM='{{.UpdatePublicKeyPEM}}'
if [ -n "$UPDATE_PUBKEY_PEM" ]; then
  printf '\nupdate:\n  public_key: |\n' >> "$CONFIG_FILE"
  printf '%s\n' "$UPDATE_PUBKEY_PEM" | sed 's/^/    /' >> "$CONFIG_FILE"
fi
# config.yaml 里有 bootstrap token → 收紧到 0600（默认 umask 会给出 0644，其他本地用户可读 ✗）
chmod 600 "$CONFIG_FILE"


# Create systemd service
echo -e "${YELLOW}Creating systemd service...${NC}"
cat > "$SERVICE_FILE" << EOF
[Unit]
Description=CoreFusion Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=$INSTALL_DIR/corefusion-agent -config $CONFIG_FILE
Restart=on-failure
RestartSec=5
LimitNOFILE=65535
WorkingDirectory=$INSTALL_DIR
# 只杀主进程（Agent），不回收整个 cgroup：节点上的 caddy/nginx 代理子进程
# 在 Agent 重启/自更新期间保持运行，新 Agent 按 PID 收编，代理不中断。
KillMode=process

[Install]
WantedBy=multi-user.target
EOF

# Reload and start
systemctl daemon-reload
systemctl enable corefusion-agent
systemctl restart corefusion-agent

# Wait and check
sleep 3
if systemctl is-active --quiet corefusion-agent; then
  echo ""
  echo -e "${GREEN}=== Installation Complete ===${NC}"
  echo "Agent installed at: $INSTALL_DIR/corefusion-agent"
  echo "Config: $CONFIG_FILE"
  echo "Service: corefusion-agent"
  echo ""
  echo "Check status: systemctl status corefusion-agent"
  echo "View logs: journalctl -u corefusion-agent -f"
  echo ""
  echo -e "${YELLOW}The Agent will bootstrap with the panel and begin mTLS communication.${NC}"
else
  echo ""
  echo -e "${RED}Agent failed to start. Recent logs:${NC}"
  journalctl -u corefusion-agent --no-pager -n 20
  exit 1
fi
`))
