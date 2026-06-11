package tuic

import (
	"encoding/json"
	"fmt"

	"corefusion/internal/configgen/common"
	"corefusion/internal/domain"
)

// Generator generates TUIC configuration.
type Generator struct{}

func NewGenerator() *Generator { return &Generator{} }
func (g *Generator) CoreType() domain.CoreType { return domain.CoreTypeTUIC }

// ServerConfig is the TUIC server configuration.
type ServerConfig struct {
	Server   string            `json:"server"`
	Users    map[string]string `json:"users"`
	Certificate string         `json:"certificate"`
	PrivateKey  string         `json:"private_key"`
	CongestionControl string   `json:"congestion_control,omitempty"`
	MaxIdleTime int            `json:"max_idle_time,omitempty"`
	AuthTimeout int            `json:"auth_timeout,omitempty"`
	ALPN        []string       `json:"alpn,omitempty"`
}

// GenerateServiceConfig generates a TUIC server configuration.
func (g *Generator) GenerateServiceConfig(spec common.ServiceConfigSpec) ([]byte, error) {
	users := make(map[string]string)
	for _, u := range spec.Users {
		users[u.UUID] = u.Password
	}

	config := ServerConfig{
		Server:        fmt.Sprintf(":%d", spec.ListenPort),
		Users:         users,
		Certificate:   spec.CertPath,
		PrivateKey:    spec.KeyPath,
		CongestionControl: "bbr",
	}

	if cc, ok := spec.Extra["congestion_control"].(string); ok {
		config.CongestionControl = cc
	}

	return json.MarshalIndent(config, "", "  ")
}

// GenerateClientConfig generates a TUIC client configuration.
func (g *Generator) GenerateClientConfig(spec common.ClientConfigSpec) ([]byte, error) {
	if len(spec.Nodes) == 0 {
		return nil, fmt.Errorf("no nodes specified")
	}

	node := spec.Nodes[0]
	config := map[string]interface{}{
		"server":      fmt.Sprintf("%s:%d", node.Host, node.Port),
		"uuid":        node.Auth.UUID,
		"password":    node.Auth.Password,
		"congestion_control": "bbr",
		"tls": map[string]interface{}{
			"sni": node.TLS.ServerName,
		},
	}

	return json.MarshalIndent(config, "", "  ")
}

// ValidateConfig validates a TUIC configuration.
func (g *Generator) ValidateConfig(content []byte) error {
	var config ServerConfig
	if err := json.Unmarshal(content, &config); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if config.Server == "" {
		return fmt.Errorf("server address required")
	}
	if config.Certificate == "" || config.PrivateKey == "" {
		return fmt.Errorf("certificate and private_key required")
	}
	return nil
}
