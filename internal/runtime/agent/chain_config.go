package agent

import (
	"encoding/json"
	"fmt"
	"time"
)

// ChainConfig is the JSON structure stored in node_servers.chain_config.
// It defines the chain proxy topology for chain_entry and chain_exit nodes.
//
// chain_entry chain_config example (multi-exit with failover):
//
//	{
//	  "exits": [
//	    {"exit_server_id": 5, "outbound_tag": "to-jp", "protocol": "vless", "transport": "tcp", "weight": 100, "priority": 1},
//	    {"exit_server_id": 8, "outbound_tag": "to-sg", "protocol": "vless", "transport": "tcp", "weight": 50, "priority": 2}
//	  ],
//	  "health_check": {"interval_sec": 30, "timeout_sec": 5, "fail_threshold": 3},
//	  "failover_strategy": "priority"
//	}
//
// chain_exit chain_config example:
//
//	{"entry_server_ids": [3, 4], "listen_only": true, "relay_port": 443, "relay_protocol": "vless"}
type ChainConfig struct {
	// ── chain_entry fields ──
	Exits            []ChainExitSpec   `json:"exits,omitempty"`
	HealthCheck      *ChainHealthCheck `json:"health_check,omitempty"`
	FailoverStrategy string            `json:"failover_strategy,omitempty"` // priority | weighted | round_robin

	// ── chain_exit fields ──
	EntryServerIDs []int64 `json:"entry_server_ids,omitempty"`
	ListenOnly     bool    `json:"listen_only,omitempty"`
	RelayPort      int     `json:"relay_port,omitempty"`
	RelayProtocol  string  `json:"relay_protocol,omitempty"`
}

// ChainExitSpec defines a single chain_exit target from a chain_entry's perspective.
type ChainExitSpec struct {
	ExitServerID int64  `json:"exit_server_id"`
	OutboundTag  string `json:"outbound_tag"`
	Protocol     string `json:"protocol"`  // vless, vmess, trojan, shadowsocks
	Transport    string `json:"transport"` // tcp, ws, grpc, http
	Security     string `json:"security"`  // tls, reality, none
	Weight       int    `json:"weight"`    // for weighted strategy
	Priority     int    `json:"priority"`  // 1 = highest
}

// ChainHealthCheck defines health check parameters for chain_exit monitoring.
type ChainHealthCheck struct {
	IntervalSec   int `json:"interval_sec"`
	TimeoutSec    int `json:"timeout_sec"`
	FailThreshold int `json:"fail_threshold"`
}

// FailoverStrategy constants.
const (
	FailoverStrategyPriority   = "priority"
	FailoverStrategyWeighted   = "weighted"
	FailoverStrategyRoundRobin = "round_robin"
)

// ParseChainConfig deserializes the chain_config JSON from a NodeServer.
func ParseChainConfig(raw string) (*ChainConfig, error) {
	if raw == "" {
		return nil, nil
	}
	var cfg ChainConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, fmt.Errorf("parse chain_config: %w", err)
	}
	return &cfg, nil
}

// Serialize serializes ChainConfig to JSON string.
func (c *ChainConfig) Serialize() (string, error) {
	data, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Validate validates the chain config for a given node type.
func (c *ChainConfig) Validate(nodeType string) error {
	switch nodeType {
	case "chain_entry":
		if len(c.Exits) == 0 {
			return fmt.Errorf("chain_entry must have at least one exit")
		}
		for _, exit := range c.Exits {
			if exit.ExitServerID == 0 {
				return fmt.Errorf("exit exit_server_id cannot be 0")
			}
			if exit.OutboundTag == "" {
				return fmt.Errorf("exit outbound_tag cannot be empty")
			}
		}
	case "chain_exit":
		if c.RelayPort == 0 {
			return fmt.Errorf("chain_exit relay_port cannot be 0")
		}
	default:
		return fmt.Errorf("chain_config not applicable for node_type %s", nodeType)
	}
	return nil
}

// HealthCheckInterval returns the health check interval as a duration, defaulting to 30s.
func (c *ChainConfig) HealthCheckInterval() time.Duration {
	if c.HealthCheck != nil && c.HealthCheck.IntervalSec > 0 {
		return time.Duration(c.HealthCheck.IntervalSec) * time.Second
	}
	return 30 * time.Second
}

// HealthCheckTimeout returns the health check timeout, defaulting to 5s.
func (c *ChainConfig) HealthCheckTimeout() time.Duration {
	if c.HealthCheck != nil && c.HealthCheck.TimeoutSec > 0 {
		return time.Duration(c.HealthCheck.TimeoutSec) * time.Second
	}
	return 5 * time.Second
}

// HealthCheckFailThreshold returns the fail threshold, defaulting to 3.
func (c *ChainConfig) HealthCheckFailThreshold() int {
	if c.HealthCheck != nil && c.HealthCheck.FailThreshold > 0 {
		return c.HealthCheck.FailThreshold
	}
	return 3
}

// PrimaryExit returns the highest-priority exit (lowest priority number).
func (c *ChainConfig) PrimaryExit() *ChainExitSpec {
	if len(c.Exits) == 0 {
		return nil
	}
	best := &c.Exits[0]
	for i := range c.Exits {
		if c.Exits[i].Priority < best.Priority {
			best = &c.Exits[i]
		}
	}
	return best
}
