package main

import (
	"fmt"
	"os"
	"strings"

	"corefusion/internal/constants"
	"gopkg.in/yaml.v3"
)

// AgentConfig defines the YAML configuration for the Agent daemon.
type AgentConfig struct {
	Panel struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"` // Panel HTTP port (for bootstrap REST API)
		// GRPCHost is the address used for the mTLS heartbeat connection.
		// It defaults to Host. When the panel is fronted by a normal CDN
		// (which only proxies HTTP/HTTPS), Host is the CDN domain and cannot
		// serve the gRPC port; the panel fills this with its origin IP so no
		// manual edit is needed.
		GRPCHost string `yaml:"grpc_host"`
		GRPCPort int    `yaml:"grpc_port"` // Panel gRPC port (for mTLS heartbeat)
		ServerID int64  `yaml:"server_id"`
		TLS      bool   `yaml:"tls"` // Use HTTPS for bootstrap API (production behind Caddy/Nginx)

		// PublicHost/PublicPort/PublicTLS describe the panel's CDN/reverse-proxy
		// HTTP endpoint. Host/Port/TLS point at the panel's direct listener
		// (used first); when that listener is not exposed to the node, bootstrap
		// falls back to this public endpoint. The returned Agent certificate is
		// still verified against the panel CA, so a TLS-terminating CDN cannot
		// substitute its own certificate.
		PublicHost string `yaml:"public_host"`
		PublicPort int    `yaml:"public_port"`
		PublicTLS  bool   `yaml:"public_tls"`
	} `yaml:"panel"`
	Bootstrap struct {
		Token string `yaml:"token"`
	} `yaml:"bootstrap"`
	Agent struct {
		LogLevel string `yaml:"log_level"`
		DataDir  string `yaml:"data_dir"`
	} `yaml:"agent"`
}

// 说明 ✓：Agent **没有** acme 配置段。
// ACME 账号邮箱、CA、挑战方式这些统统由**面板**在 `IssuePublicCertCommand` 里下发 ✓
// （面板本来就有 ACME 账号 ✓：`acme_account_configs`，见证书页的"邮箱"下拉 ✓），
// 挑战路径也由面板自己掌握 ✓（面板生成节点 web 配置时就带 `/.well-known/acme-challenge/` ✓）。
// 让每个节点各填一份 yaml 属于重复配置 ✗，节点侧应当零配置 ✓。

func loadConfig(path string) (*AgentConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg AgentConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.Agent.DataDir == "" {
		cfg.Agent.DataDir = "/opt/corefusion-agent/data"
	}
	if cfg.Panel.GRPCPort == 0 {
		cfg.Panel.GRPCPort = constants.DefaultAgentGRPCPort
	}
	if cfg.Panel.Port == 0 {
		cfg.Panel.Port = 8080
	}
	cfg.Panel.PublicHost = strings.TrimSpace(cfg.Panel.PublicHost)
	if cfg.Panel.PublicHost != "" && (cfg.Panel.PublicPort <= 0 || cfg.Panel.PublicPort > 65535) {
		cfg.Panel.PublicPort = cfg.Panel.Port
	}
	// Backward compatible: older configs only have panel.host. Keep using it
	// for gRPC unless the panel explicitly installed a separate grpc_host.
	if strings.TrimSpace(cfg.Panel.GRPCHost) == "" {
		cfg.Panel.GRPCHost = cfg.Panel.Host
	} else {
		cfg.Panel.GRPCHost = strings.TrimSpace(cfg.Panel.GRPCHost)
	}
	return &cfg, nil
}
