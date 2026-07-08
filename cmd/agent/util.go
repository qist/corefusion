package main

import (
	"os"
	"path/filepath"
)

// initDataDir creates the standard directory layout under the Agent's data directory.
func initDataDir(dataDir string) {
	dirs := []string{
		filepath.Join(dataDir, "sqlite"),
		filepath.Join(dataDir, "certs"),
		filepath.Join(dataDir, "logs"),
		filepath.Join(dataDir, "runtime"),
		filepath.Join(dataDir, "proxy"),
		filepath.Join(dataDir, "binaries", "active"),
		filepath.Join(dataDir, "binaries", "proxy"),
	}
	for _, d := range dirs {
		os.MkdirAll(d, 0750)
	}
}

// generateDefaultConfig generates a minimal valid config for a core type.
// The Agent uses this when no config has been synced from the panel yet.
// Once the panel syncs a full config via SyncConfig, it overwrites these defaults.
func generateDefaultConfig(coreType string, dataDir string) map[string]interface{} {
	rulesDir := filepath.Join(dataDir, "rules")
	switch coreType {
	case "xray":
		return map[string]interface{}{
			"log": map[string]interface{}{
				"loglevel": "warning",
			},
			"api": map[string]interface{}{
				"tag":      "api",
				"services": []string{"HandlerService", "LoggerService", "StatsService", "RoutingService"},
			},
			"stats": map[string]interface{}{},
			"inbounds": []interface{}{
				map[string]interface{}{
					"tag":      "api",
					"listen":   "127.0.0.1",
					"port":     10085,
					"protocol": "dokodemo-door",
					"settings": map[string]interface{}{"address": "127.0.0.1"},
				},
			},
			"outbounds": []interface{}{
				map[string]interface{}{"protocol": "freedom", "tag": "direct"},
				map[string]interface{}{"protocol": "blackhole", "tag": "blocked"},
			},
			"routing": map[string]interface{}{
				"domainStrategy": "IPIfNonMatch",
				"rules": []interface{}{
					map[string]interface{}{
						"type":        "field",
						"inboundTag":  []string{"api"},
						"outboundTag": "api",
					},
				},
			},
			"policy": map[string]interface{}{
				"levels": map[string]interface{}{
					"0": map[string]interface{}{
						"statsUserUplink":   true,
						"statsUserDownlink": true,
					},
				},
				"system": map[string]interface{}{
					"statsInboundUplink":   true,
					"statsInboundDownlink": true,
				},
			},
		}
	case "singbox":
		return map[string]interface{}{
			"log": map[string]interface{}{
				"level": "info",
			},
			"inbounds":  []interface{}{},
			"outbounds": []interface{}{},
		}
	case "hysteria2":
		return map[string]interface{}{
			"log": map[string]interface{}{
				"level": "info",
			},
			"listen":  ":0",
			"masquerade": map[string]interface{}{
				"type": "404",
			},
		}
	case "tuic":
		return map[string]interface{}{
			"server": "0.0.0.0:0",
			"users":  []interface{}{},
		}
	}
	_ = rulesDir // rulesDir used for future geo file path injection
	return nil
}
