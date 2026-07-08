package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// AgentConfig defines the YAML configuration for the Agent daemon.
type AgentConfig struct {
	Panel struct {
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`      // Panel HTTP port (for bootstrap REST API)
		GRPCPort int    `yaml:"grpc_port"` // Panel gRPC port (for mTLS heartbeat)
		ServerID int64  `yaml:"server_id"`
		TLS      bool   `yaml:"tls"` // Use HTTPS for bootstrap API (production behind Caddy/Nginx)
	} `yaml:"panel"`
	Bootstrap struct {
		Token string `yaml:"token"`
	} `yaml:"bootstrap"`
	Agent struct {
		ListenPort int    `yaml:"listen_port"` // Deprecated: no longer used. All communication via heartbeat stream.
		LogLevel   string `yaml:"log_level"`
		DataDir    string `yaml:"data_dir"`
	} `yaml:"agent"`
}

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
	if cfg.Agent.ListenPort == 0 {
		cfg.Agent.ListenPort = 18444
	}
	if cfg.Panel.GRPCPort == 0 {
		cfg.Panel.GRPCPort = 18443
	}
	if cfg.Panel.Port == 0 {
		cfg.Panel.Port = 8080
	}
	return &cfg, nil
}
