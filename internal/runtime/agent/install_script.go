package agent

import (
	"fmt"
	"strings"
	"text/template"
)

// AgentInstallScript generates a bash install script for the Agent binary.
// The script downloads the Agent (panel-first, GitHub fallback), configures it
// with the panel address and bootstrap token, then starts the service.
type AgentInstallScript struct {
	panelHost       string
	panelPort       int
	panelScheme     string // "http" or "https"
	panelGRPCPort   int
	panelTLS        bool
	bootstrapToken  string
	serverID        int64
	agentVersion    string
	githubURL       string // GitHub Releases base URL (fallback)
}

// NewAgentInstallScript creates a new AgentInstallScript generator.
func NewAgentInstallScript(panelHost string, panelPort int) *AgentInstallScript {
	return &AgentInstallScript{
		panelHost:       panelHost,
		panelPort:       panelPort,
		panelScheme:     "http",
		panelGRPCPort:   18443,
		panelTLS:        false,
		agentVersion:    "latest",
		githubURL:       "https://github.com/qist/corefusion/releases/download",
	}
}

// WithBootstrapToken sets the bootstrap token.
func (s *AgentInstallScript) WithBootstrapToken(token string) *AgentInstallScript {
	s.bootstrapToken = token
	return s
}

// WithServerID sets the server ID.
func (s *AgentInstallScript) WithServerID(id int64) *AgentInstallScript {
	s.serverID = id
	return s
}

// WithVersion sets the Agent version.
func (s *AgentInstallScript) WithVersion(version string) *AgentInstallScript {
	s.agentVersion = version
	return s
}

// WithScheme sets the panel URL scheme ("http" or "https").
func (s *AgentInstallScript) WithScheme(scheme string) *AgentInstallScript {
	s.panelScheme = scheme
	return s
}

// WithGRPCPort sets the panel gRPC port.
func (s *AgentInstallScript) WithGRPCPort(port int) *AgentInstallScript {
	s.panelGRPCPort = port
	return s
}

// WithTLS sets whether the panel uses TLS.
func (s *AgentInstallScript) WithTLS(tls bool) *AgentInstallScript {
	s.panelTLS = tls
	return s
}

// WithGitHubURL sets the GitHub Releases download base URL.
func (s *AgentInstallScript) WithGitHubURL(url string) *AgentInstallScript {
	s.githubURL = url
	return s
}

// WithDownloadURL sets the download base URL (legacy compat, maps to githubURL).
func (s *AgentInstallScript) WithDownloadURL(url string) *AgentInstallScript {
	s.githubURL = url
	return s
}

// Generate generates the install script content.
func (s *AgentInstallScript) Generate() (string, error) {
	var sb strings.Builder
	err := installScriptTmpl.Execute(&sb, map[string]interface{}{
	"PanelHost":       s.panelHost,
	"PanelPort":       s.panelPort,
	"PanelScheme":     s.panelScheme,
	"PanelGRPCPort":   s.panelGRPCPort,
	"PanelTLS":        s.panelTLS,
	"BootstrapToken":  s.bootstrapToken,
	"ServerID":        s.serverID,
		"AgentVersion":    s.agentVersion,
		"GitHubURL":       s.githubURL,
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
# Panel: {{.PanelScheme}}://{{.PanelHost}}:{{.PanelPort}}

PANEL_SCHEME="{{.PanelScheme}}"
PANEL_HOST="{{.PanelHost}}"
PANEL_PORT="{{.PanelPort}}"
PANEL_GRPC_PORT="{{.PanelGRPCPort}}"
PANEL_TLS={{.PanelTLS}}
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
echo "Panel: $PANEL_SCHEME://$PANEL_HOST:$PANEL_PORT"
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
# Strategy: try panel first (dev/test), fallback to GitHub Releases (production)
PANEL_DOWNLOAD_URL="${PANEL_SCHEME}://${PANEL_HOST}:${PANEL_PORT}/api/v1/agent/download?os=${OS}&arch=${AGENT_ARCH}"
GITHUB_DOWNLOAD_URL="${GITHUB_URL}/${AGENT_VERSION}/corefusion-agent-${OS}-${AGENT_ARCH}"

echo -e "${YELLOW}Downloading Agent...${NC}"
if curl -fsSL -o "$INSTALL_DIR/corefusion-agent" "$PANEL_DOWNLOAD_URL" 2>/dev/null; then
  echo "Downloaded from panel ($PANEL_DOWNLOAD_URL)"
elif curl -fsSL -o "$INSTALL_DIR/corefusion-agent" "$GITHUB_DOWNLOAD_URL"; then
  echo "Downloaded from GitHub ($GITHUB_DOWNLOAD_URL)"
else
  echo -e "${RED}Failed to download Agent from both panel and GitHub${NC}"
  echo "  Panel: $PANEL_DOWNLOAD_URL"
  echo "  GitHub: $GITHUB_DOWNLOAD_URL"
  exit 1
fi
chmod +x "$INSTALL_DIR/corefusion-agent"

# Generate config
echo -e "${YELLOW}Writing config...${NC}"
cat > "$CONFIG_FILE" << EOF
panel:
  host: "$PANEL_HOST"
  port: $PANEL_PORT
  grpc_port: $PANEL_GRPC_PORT
  tls: $PANEL_TLS
  server_id: $SERVER_ID

bootstrap:
  token: "$BOOTSTRAP_TOKEN"

agent:
  log_level: info
  data_dir: "$INSTALL_DIR/data"
EOF

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
