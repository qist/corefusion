package agent

import (
	"fmt"
	"strings"
	"text/template"
)

// AgentInstallScript generates a bash install script for the Agent binary.
// The script downloads the Agent, configures it with the panel address and
// bootstrap token, then starts the service.
type AgentInstallScript struct {
	panelHost      string
	panelPort      int
	bootstrapToken string
	serverID       int64
	agentVersion   string
	downloadURL    string
}

// NewAgentInstallScript creates a new AgentInstallScript generator.
func NewAgentInstallScript(panelHost string, panelPort int) *AgentInstallScript {
	return &AgentInstallScript{
		panelHost:    panelHost,
		panelPort:    panelPort,
		agentVersion: "latest",
		downloadURL:  "https://github.com/corefusion/agent/releases/download",
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

// WithDownloadURL sets the download base URL.
func (s *AgentInstallScript) WithDownloadURL(url string) *AgentInstallScript {
	s.downloadURL = url
	return s
}

// Generate generates the install script content.
func (s *AgentInstallScript) Generate() (string, error) {
	var sb strings.Builder
	err := installScriptTmpl.Execute(&sb, map[string]interface{}{
		"PanelHost":      s.panelHost,
		"PanelPort":      s.panelPort,
		"BootstrapToken": s.bootstrapToken,
		"ServerID":       s.serverID,
		"AgentVersion":   s.agentVersion,
		"DownloadURL":    s.downloadURL,
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
# Panel: {{.PanelHost}}:{{.PanelPort}}

PANEL_HOST="{{.PanelHost}}"
PANEL_PORT="{{.PanelPort}}"
BOOTSTRAP_TOKEN="{{.BootstrapToken}}"
SERVER_ID="{{.ServerID}}"
AGENT_VERSION="{{.AgentVersion}}"
INSTALL_DIR="/opt/corefusion-agent"
CONFIG_FILE="$INSTALL_DIR/config.yaml"
SERVICE_FILE="/etc/systemd/system/corefusion-agent.service"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${GREEN}=== CoreFusion Agent Installer ===${NC}"
echo "Panel: $PANEL_HOST:$PANEL_PORT"
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

# Create install directory
mkdir -p "$INSTALL_DIR"

# Download Agent binary
DOWNLOAD_URL="{{.DownloadURL}}/${AGENT_VERSION}/corefusion-agent-${OS}-${AGENT_ARCH}"
echo -e "${YELLOW}Downloading Agent from $DOWNLOAD_URL...${NC}"
curl -fSL -o "$INSTALL_DIR/corefusion-agent" "$DOWNLOAD_URL"
chmod +x "$INSTALL_DIR/corefusion-agent"

# Generate config
echo -e "${YELLOW}Writing config...${NC}"
cat > "$CONFIG_FILE" << EOF
panel:
  host: "$PANEL_HOST"
  port: $PANEL_PORT
  server_id: $SERVER_ID

bootstrap:
  token: "$BOOTSTRAP_TOKEN"

agent:
  listen_port: 18443
  log_level: info
  data_dir: "$INSTALL_DIR/data"

cores:
  xray:
    binary_path: "/usr/local/bin/xray"
    config_path: "$INSTALL_DIR/configs/xray"
  singbox:
    binary_path: "/usr/local/bin/sing-box"
    config_path: "$INSTALL_DIR/configs/singbox"
EOF

mkdir -p "$INSTALL_DIR/data" "$INSTALL_DIR/configs/xray" "$INSTALL_DIR/configs/singbox"

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
`))
