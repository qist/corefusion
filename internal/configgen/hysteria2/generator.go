package hysteria2

import (
	"encoding/json"
	"fmt"

	"corefusion/internal/configgen/common"
	"corefusion/internal/domain"
)

// Generator generates Hysteria 2 configuration.
type Generator struct{}

func NewGenerator() *Generator { return &Generator{} }
func (g *Generator) CoreType() domain.CoreType { return domain.CoreTypeHysteria2 }

// ServerConfig is the Hysteria 2 server configuration.
type ServerConfig struct {
	Listen   string                 `json:"listen"`
	TLS      TLSConfig              `json:"tls"`
	Auth     AuthConfig             `json:"auth"`
	Bandwidth *BandwidthConfig      `json:"bandwidth,omitempty"`
	QUIC     *QUICConfig            `json:"quic,omitempty"`
	OutOut   []OutboundConfig       `json:"outOut,omitempty"` // Hysteria 2 uses "outOut"
	TrafficStats *TrafficStatsConfig `json:"trafficStats,omitempty"`
}

type TLSConfig struct {
	Cert string   `json:"cert"`
	Key  string   `json:"key"`
	ALPN []string `json:"alpn,omitempty"`
}

type AuthConfig struct {
	Type     string            `json:"type"`
	Password string            `json:"password,omitempty"`
	UserPass map[string]string `json:"userpass,omitempty"`
}

type BandwidthConfig struct {
	Up   string `json:"up"`
	Down string `json:"down"`
}

type QUICConfig struct {
	InitialStreamReceiveWindow     int `json:"initialStreamReceiveWindow,omitempty"`
	MaxStreamReceiveWindow         int `json:"maxStreamReceiveWindow,omitempty"`
	InitialConnectionReceiveWindow int `json:"initialConnectionReceiveWindow,omitempty"`
	MaxConnectionReceiveWindow     int `json:"maxConnectionReceiveWindow,omitempty"`
}

type TrafficStatsConfig struct {
	Listen string `json:"listen"`
	Secret string `json:"secret,omitempty"`
}

type OutboundConfig struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// GenerateServiceConfig generates a Hysteria 2 server configuration.
func (g *Generator) GenerateServiceConfig(spec common.ServiceConfigSpec) ([]byte, error) {
	config := ServerConfig{
		Listen: fmt.Sprintf(":%d", spec.ListenPort),
		TLS: TLSConfig{
			Cert: spec.CertPath,
			Key:  spec.KeyPath,
		},
	}

	// Auth
	if len(spec.Users) == 1 {
		config.Auth = AuthConfig{
			Type:     "password",
			Password: spec.Users[0].Password,
		}
	} else if len(spec.Users) > 1 {
		userPass := make(map[string]string)
		for _, u := range spec.Users {
			userPass[u.Email] = u.Password
		}
		config.Auth = AuthConfig{
			Type:     "userpass",
			UserPass: userPass,
		}
	}

	// Traffic stats
	config.TrafficStats = &TrafficStatsConfig{
		Listen: "127.0.0.1:25601",
	}

	// QUIC settings
	if v, ok := spec.Extra["max_stream_receive_window"].(int); ok {
		config.QUIC = &QUICConfig{
			MaxStreamReceiveWindow: v,
		}
	}

	return json.MarshalIndent(config, "", "  ")
}

// GenerateClientConfig generates a Hysteria 2 client configuration.
func (g *Generator) GenerateClientConfig(spec common.ClientConfigSpec) ([]byte, error) {
	if len(spec.Nodes) == 0 {
		return nil, fmt.Errorf("no nodes specified")
	}

	node := spec.Nodes[0]
	config := map[string]interface{}{
		"server":   fmt.Sprintf("%s:%d", node.Host, node.Port),
		"auth":     node.Auth.Password,
		"tls": map[string]interface{}{
			"sni": node.TLS.ServerName,
		},
	}

	if len(node.TLS.ALPN) > 0 {
		config["tls"].(map[string]interface{})["alpn"] = node.TLS.ALPN
	}

	return json.MarshalIndent(config, "", "  ")
}

// ValidateConfig validates a Hysteria 2 configuration.
func (g *Generator) ValidateConfig(content []byte) error {
	var config ServerConfig
	if err := json.Unmarshal(content, &config); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if config.Listen == "" {
		return fmt.Errorf("listen address required")
	}
	if config.TLS.Cert == "" || config.TLS.Key == "" {
		return fmt.Errorf("TLS cert and key required")
	}
	return nil
}
